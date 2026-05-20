package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
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
			mcp.Description("Scoring mode. Only 'deterministic' is implemented (default, zero false positives). 'with_heuristics' is reserved and currently rejected with UNSUPPORTED_MODE — for vision-based visual QA call inspect_slide_images directly on rendered thumbnails."),
			mcp.Enum("deterministic", "with_heuristics"),
		),
		mcp.WithArray("slide_indices",
			mcp.Description("Optional array of 0-based slide indices to score. When provided, only those slides are rendered + scored — skipping the rest is significantly faster for iterative single-slide refinement. Findings on other slides are omitted; per_slide contains only the requested indices. summary.slide_count still reflects the full deck size. composition is omitted because it only meaningfully scores the full deck."),
			mcp.Items(map[string]any{"type": "integer", "minimum": 0}),
		),
	)
}

// handleScoreDeck renders the deck and returns visual-quality findings. All
// error responses (missing params, invalid JSON, template lookup, marshal
// failure) carry a next_tool_call suggestion so the agent can chain to the
// recovery tool (get_input_schema, list_templates) without inferring it.
func (mc *mcpConfig) handleScoreDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("score_deck", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	mode := "deterministic"
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		mode = m
	}

	// Reject unimplemented modes up front rather than silently downgrading.
	// 'with_heuristics' is reserved for a future render+inspect pass; until
	// that ships, agents that want vision-based QA should call
	// inspect_slide_images directly on rendered thumbnails.
	switch mode {
	case "deterministic":
		// supported
	case "with_heuristics":
		return mcpErrorWithNext(
			"UNSUPPORTED_MODE",
			"score_deck mode 'with_heuristics' is not implemented; call inspect_slide_images on rendered thumbnails for vision-based visual QA, or omit the mode parameter to use 'deterministic'",
			nextCallInspectSlideImages(),
		), nil
	default:
		return mcpErrorWithNext(
			"UNSUPPORTED_MODE",
			fmt.Sprintf("score_deck mode %q is not recognized; supported modes: 'deterministic'", mode),
			nextCallGetInputSchema(),
		), nil
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nextCallGetInputSchema()), nil
	}

	// Apply deck-level defaults before checks.
	applyDefaults(&input)

	// Resolve named style references from template settings.
	mc.resolveInputNamedSettings(&input)

	// Expand structure block into flat slides (mutually exclusive with
	// top-level slides). Mirrors the CLI path so MCP and CLI agree on the
	// effective slide list before slide-count checks.
	if structDiags := applyStructureExpansion(&input); len(structDiags) > 0 {
		return api.MCPDiagnosticsError(structDiags), nil
	}

	// Resolve template name.
	templateName := input.Template
	if override, err := request.RequireString("template"); err == nil && override != "" {
		templateName = override
	}
	if templateName == "" {
		return argMissing("score_deck", "template", "string", "midnight-blue", nextCallListTemplates()), nil
	}
	if len(input.Slides) == 0 {
		return argMissing("score_deck", "presentation.slides", "array", []any{map[string]any{"layout_id": "title"}}, nextCallGetInputSchema()), nil
	}

	// Optional slide_indices: when provided, only those slides are rendered +
	// scored, which is significantly faster than rerunning the whole deck.
	slideIndices, idxErr := extractSlideIndices(request, len(input.Slides))
	if idxErr != nil {
		return argInvalidValue("score_deck", "INVALID_PARAMETER", "slide_indices", idxErr.Error(), "array", []any{0, 1}, nil), nil
	}

	// Resolve and analyze template.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return mcpErrorWithNext("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir), nextCallListTemplates()), nil
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates()), nil
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates()), nil
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
	findings := collectFitFindings(&input, layouts, slideWidth, slideHeight, &analysis.Theme)

	// 2. Run actual generation to a temp directory to capture render-time findings
	//    (contrast swaps, autofit shrink, pagination, clamping). When
	//    slide_indices is set, render only that subset (significantly faster
	//    for iterative per-slide refinement) and remap finding paths from the
	//    subset index space back to the original deck index space.
	dataPalette := resolveDataPalette(templateMetadata, analysis.Theme.Colors)
	var renderFindings []patterns.FitFinding
	if len(slideIndices) > 0 {
		subset, subsetToOrig := buildSlideSubset(&input, slideIndices)
		renderFindings = mc.collectRenderFindings(ctx, subset, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette)
		renderFindings = remapFindingsSlideIndex(renderFindings, subsetToOrig)
	} else {
		renderFindings = mc.collectRenderFindings(ctx, &input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette)
	}
	findings = append(findings, renderFindings...)

	// 3. Append synthesis findings (template-level).
	findings = append(findings, synthesisFindings...)

	// Score the combined findings (correctness axis). When slide_indices is
	// set, PerSlide only contains entries for those indices; composition is
	// omitted because it only meaningfully scores the full deck.
	var ds *deterministic.DeckScore
	if len(slideIndices) > 0 {
		ds = deterministic.ScoreFromFindingsForIndices(findings, len(input.Slides), slideIndices)
	} else {
		ds = deterministic.ScoreFromFindings(findings, len(input.Slides))
		// 4. Composition axis — deck-level rhythm analysis.
		ds.Composition = compositionAxis(input.Slides)
	}

	mcpResult, err := api.MCPSuccessResult(ctx, ds)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("score_deck", "presentation")), nil
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
	dataPalette []string,
) []patterns.FitFinding {
	// Resolve deck-level rhythm grid when configured.
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		rhythmGrid = resolveGrid(input.Grid, layouts, slideWidth, slideHeight)
	}

	// Convert slides to generator specs.
	slideSpecs, _, _, err := convertPresentationSlides(input.Slides, layouts, slideWidth, slideHeight, templateMetadata, rhythmGrid, patterns.AccentStrategy(input.AccentStrategy), nil, false)
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
		DataPalette:           dataPalette,
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

// compositionAxis runs analyze_deck_rhythm on the input slides and converts
// the result into a CompositionResult for the score_deck response.
func compositionAxis(slides []SlideInput) *deterministic.CompositionResult {
	if len(slides) == 0 {
		return nil
	}

	rhythm := analyzeDeckRhythm(slides)
	compositionScore := rhythm.CompositionScore

	// Always non-nil so JSON marshals as [] (not null) when no diagnostics.
	diags := []deterministic.CompositionDiagnostic{}

	// Flag duplicate titles on content slides (case-insensitive). Title and
	// section-divider slides are exempt. Each duplicate group adds one
	// diagnostic and lowers the composition score by 5 per occurrence beyond
	// the first, capped at 30 so a single offending group can't sink the
	// composition axis alone.
	dupBeyondFirst, dupGroups, dupExamples := duplicateTitleSummary(slides)
	if dupGroups > 0 {
		penalty := 5 * dupBeyondFirst
		if penalty > 30 {
			penalty = 30
		}
		compositionScore -= penalty
		if compositionScore < 0 {
			compositionScore = 0
		}
		msg := fmt.Sprintf("%d content slide(s) repeat a title used earlier in the deck — distinct headlines improve audience comprehension", dupBeyondFirst)
		if len(dupExamples) > 0 {
			preview := dupExamples
			if len(preview) > 3 {
				preview = preview[:3]
			}
			quoted := make([]string, len(preview))
			for i, t := range preview {
				quoted[i] = fmt.Sprintf("%q", t)
			}
			msg += " (e.g., " + strings.Join(quoted, ", ") + ")"
		}
		diags = append(diags, deterministic.CompositionDiagnostic{
			Code:     "duplicate_title",
			Severity: "warning",
			Message:  msg,
		})
	}

	// Flag long pattern runs (3+).
	for _, run := range rhythm.Aggregates.PatternRuns {
		if run.Len >= 3 {
			diags = append(diags, deterministic.CompositionDiagnostic{
				Code:     "pattern_run",
				Severity: "warning",
				Message:  fmt.Sprintf("pattern %q repeats %d consecutive slides (index %d–%d); vary with a different layout", run.Name, run.Len, run.Start, run.Start+run.Len-1),
			})
		}
	}

	// Flag accent dominance (>80%).
	for accent, frac := range rhythm.Aggregates.AccentBalance {
		if frac > 0.8 && len(rhythm.Aggregates.AccentBalance) > 1 {
			diags = append(diags, deterministic.CompositionDiagnostic{
				Code:     "accent_dominance",
				Severity: "warning",
				Message:  fmt.Sprintf("accent %q dominates %.0f%% of accented slides; consider using more accent colors", accent, frac*100),
			})
		}
	}

	// Flag low density variation.
	if rhythm.Aggregates.DensityCV < 0.15 && len(slides) > 3 {
		diags = append(diags, deterministic.CompositionDiagnostic{
			Code:     "density_monotony",
			Severity: "info",
			Message:  fmt.Sprintf("density coefficient of variation is %.2f (low); mix content-heavy and content-light slides", rhythm.Aggregates.DensityCV),
		})
	}

	// Flag missing narrative roles — no emphasis slide in 10+ slide decks.
	if len(slides) >= 10 {
		hasEmphasis := false
		for _, si := range rhythm.PerSlide {
			switch si.Pattern {
			case "stat-hero", "pull-quote":
				hasEmphasis = true
			}
		}
		if !hasEmphasis {
			diags = append(diags, deterministic.CompositionDiagnostic{
				Code:     "missing_emphasis",
				Severity: "info",
				Message:  "deck has 10+ slides but no emphasis slide (stat-hero, pull-quote); consider adding one to break monotony",
			})
		}
	}

	return &deterministic.CompositionResult{
		Score:       compositionScore,
		Diagnostics: diags,
	}
}

// extractSlideIndices parses an optional slide_indices array parameter into a
// validated, sorted, deduplicated slice of 0-based indices. Returns (nil, nil)
// when the parameter is absent or empty (full-deck scoring). Returns an error
// when any value is non-numeric or out of range.
func extractSlideIndices(request mcp.CallToolRequest, slideCount int) ([]int, error) {
	args := request.GetArguments()
	raw, ok := args["slide_indices"]
	if !ok || raw == nil {
		return nil, nil
	}

	// Re-marshal/unmarshal so we accept either []float64, []int, or json.Number.
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("slide_indices: %w", err)
	}
	if string(data) == "null" {
		return nil, nil
	}
	var arr []json.Number
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&arr); err != nil {
		return nil, fmt.Errorf("slide_indices must be an array of integers: %v", err)
	}
	if len(arr) == 0 {
		return nil, nil
	}

	seen := make(map[int]bool, len(arr))
	out := make([]int, 0, len(arr))
	for _, n := range arr {
		i64, err := n.Int64()
		if err != nil {
			return nil, fmt.Errorf("slide_indices must contain integers, got %s", n.String())
		}
		idx := int(i64)
		if idx < 0 || idx >= slideCount {
			return nil, fmt.Errorf("slide_indices: index %d out of range (deck has %d slides, valid range 0-%d)", idx, slideCount, slideCount-1)
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, idx)
	}
	sort.Ints(out)
	return out, nil
}

// buildSlideSubset returns a shallow copy of input containing only the slides
// at indices (in ascending order) and a mapping from subset position back to
// the original index, so finding paths emitted by the renderer can be remapped
// to the original index space.
func buildSlideSubset(input *PresentationInput, indices []int) (*PresentationInput, map[int]int) {
	clone := *input
	clone.Slides = make([]SlideInput, 0, len(indices))
	subsetToOrig := make(map[int]int, len(indices))
	for subsetIdx, origIdx := range indices {
		clone.Slides = append(clone.Slides, input.Slides[origIdx])
		subsetToOrig[subsetIdx] = origIdx
	}
	return &clone, subsetToOrig
}

// remapFindingsSlideIndex rewrites the "/slides/{idx}/..." prefix in each
// finding's Path from the subset (rendered) index back to the original deck
// index, using subsetToOrig. Findings whose path doesn't begin with "/slides/"
// or whose subset index isn't in the mapping are returned unchanged.
func remapFindingsSlideIndex(findings []patterns.FitFinding, subsetToOrig map[int]int) []patterns.FitFinding {
	if len(findings) == 0 || len(subsetToOrig) == 0 {
		return findings
	}
	out := make([]patterns.FitFinding, len(findings))
	for i, f := range findings {
		out[i] = f
		subsetIdx := slidepath.SlideIndex(f.Path)
		if subsetIdx < 0 {
			continue
		}
		origIdx, ok := subsetToOrig[subsetIdx]
		if !ok || origIdx == subsetIdx {
			continue
		}
		oldPrefix := slidepath.Slide(subsetIdx)
		newPrefix := slidepath.Slide(origIdx)
		out[i].Path = newPrefix + f.Path[len(oldPrefix):]
	}
	return out
}


