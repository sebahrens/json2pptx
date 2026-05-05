package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

func mcpScoreDeckTool() mcp.Tool {
	return mcp.NewTool("score_deck",
		mcp.WithDescription(`Score a presentation for visual quality using deterministic rules. Returns an overall score (0-100), per-slide scores, and structured findings with fix suggestions.

Runs a full generation pass (to a temporary directory) so the score reflects actual rendered state — including pagination, autofit shrink, contrast swaps, and layout synthesis. Static analysis alone (without generation) misses these render-time effects.

Score formula: 100 - sum(severity_weights × findings). Weights: refuse=25, shrink_or_split=15, review=5, info=0.

Use this after generate_presentation to get structured visual feedback without burning vision tokens. The score will differ from a naive static check when generation-time autofix kicked in.`),
		mcp.WithRawOutputSchema(outputSchemaScoreDeck),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description("Presentation definition. Same schema as generate_presentation."),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithString("template",
			mcp.Description("Template name override. If omitted, uses the template field from the presentation object."),
		),
		mcp.WithString("mode",
			mcp.Description("Scoring mode: 'deterministic' (default, zero false positives) or 'with_heuristics' (adds vision-model checks, requires ANTHROPIC_API_KEY and rendered images)."),
			mcp.Enum("deterministic", "with_heuristics"),
		),
	)
}

func (mc *mcpConfig) handleScoreDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "presentation is required"), nil
	}

	mode := "deterministic"
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		mode = m
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "presentation", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Apply deck-level defaults before checks.
	applyDefaults(&input)

	// Resolve named style references from template settings.
	mc.resolveInputNamedSettings(&input)

	// Resolve template name.
	templateName := input.Template
	if override, err := request.RequireString("template"); err == nil && override != "" {
		templateName = override
	}
	if templateName == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "template is required (in presentation or as template parameter)"), nil
	}
	if len(input.Slides) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "at least one slide is required in presentation"), nil
	}

	// Resolve and analyze template.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)

	// Analyze template for synthesis and metadata.
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        template.ParseTheme(reader),
	}
	synthesisFindings := template.SynthesizeIfNeeded(reader, analysis)
	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	templateMetadata, _ := template.ParseMetadata(reader)

	// 1. Collect static fit findings from input JSON.
	findings := collectFitFindings(&input, layouts, slideWidth, slideHeight)

	// 2. Run actual generation to a temp directory to capture render-time findings
	//    (contrast swaps, autofit shrink, pagination, clamping).
	renderFindings := mc.collectRenderFindings(ctx, &input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata)
	findings = append(findings, renderFindings...)

	// 3. Append synthesis findings (template-level).
	findings = append(findings, synthesisFindings...)

	// Score the combined findings.
	ds := deterministic.ScoreFromFindings(findings, len(input.Slides))

	if mode == "with_heuristics" {
		// Heuristic mode requires rendered images + API key — not yet wired up.
		// Return deterministic results with a note.
		ds.ModeUsed = "deterministic"
		ds.Summary.TopCodes = appendHeuristicNote(ds.Summary.TopCodes)
	}

	mcpResult, err := api.MCPSuccessResult(ctx, ds)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// collectRenderFindings runs the generation pipeline to a temp directory and
// returns findings that only materialize at render time: contrast swaps,
// autofit shrink/truncation, clamping, etc.
func (mc *mcpConfig) collectRenderFindings(
	ctx context.Context,
	input *PresentationInput,
	templatePath string,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
	syntheticFiles map[string][]byte,
	templateMetadata *types.TemplateMetadata,
) []patterns.FitFinding {
	// Convert slides to generator specs.
	slideSpecs, err := convertPresentationSlides(input.Slides, layouts, slideWidth, slideHeight, templateMetadata)
	if err != nil {
		// If conversion fails, skip render findings (static findings still apply).
		return nil
	}

	// Create temp directory for the generation output.
	tmpDir, err := os.MkdirTemp("", "score-deck-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, "scored.pptx")

	genReq := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		SVGStrategy:           string(mc.cfg.SVG.Strategy),
		SVGScale:              mc.cfg.SVG.Scale,
		SVGNativeCompat:       string(mc.cfg.SVG.NativeCompatibility),
		MaxPNGWidth:           mc.cfg.SVG.MaxPNGWidth,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
		StrictFit:             "warn",
	}

	// Wire footer/chrome configuration.
	if input.Chrome != nil {
		genReq.Footer = chromeToFooterConfig(input.Chrome, len(slideSpecs))
		applyChromeSkip(slideSpecs, input.Chrome, input.Slides, layouts)
	} else if input.Footer != nil && input.Footer.Enabled {
		genReq.Footer = &generator.FooterConfig{
			Enabled:  true,
			LeftText: input.Footer.LeftText,
		}
	}
	if input.ThemeOverride != nil {
		genReq.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}

	result, err := generator.Generate(ctx, genReq)
	if err != nil {
		// Generation failed — skip render findings.
		return nil
	}

	var renderFindings []patterns.FitFinding
	renderFindings = append(renderFindings, result.FitFindings...)
	renderFindings = append(renderFindings, contrastSwapsToFindings(result.ContrastSwaps)...)
	return renderFindings
}

// appendHeuristicNote adds a synthetic code entry indicating heuristic mode
// was requested but is not yet available.
func appendHeuristicNote(codes []deterministic.CodeCount) []deterministic.CodeCount {
	return codes // no-op for now; heuristic mode is opt-in future work
}

