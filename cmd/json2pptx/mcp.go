package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/icons"
	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// mcpConfig holds the resolved configuration for MCP tool handlers.
type mcpConfig struct {
	templatesDir string
	outputDir    string
	cfg          config.Config
	cache        *template.MemoryCache
}

// runMCP starts an MCP server over stdio, exposing json2pptx tools.
func runMCP() error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./output", "Output directory for generated PPTX files")
	configPath := fs.String("config", "", "Path to config file (optional)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx mcp [options]\n\n")
		fmt.Fprintf(os.Stderr, "Start an MCP (Model Context Protocol) server over stdio.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Fail fast if the font subsystem is broken.
	if err := fontcache.Verify(); err != nil {
		return fmt.Errorf("font subsystem check failed: %w", err)
	}

	// Load configuration
	cfg := config.DefaultConfig()
	if *configPath != "" {
		var err error
		cfg, err = config.Load(*configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	}

	// Apply flag overrides
	if *templatesDir != "" {
		cfg.Templates.Dir = *templatesDir
	}
	if *outputDir != "" {
		cfg.Storage.OutputDir = *outputDir
	}

	// Logging goes to stderr so stdio transport stays clean
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	mc := &mcpConfig{
		templatesDir: cfg.Templates.Dir,
		outputDir:    cfg.Storage.OutputDir,
		cfg:          cfg,
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Advertise compact_responses as an experimental server capability.
	// Clients that send experimental.compact_responses: true in their
	// initialize request will receive compact (non-indented) JSON responses.
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(_ context.Context, _ any, _ *mcp.InitializeRequest, result *mcp.InitializeResult) {
		if result.Capabilities.Experimental == nil {
			result.Capabilities.Experimental = make(map[string]any)
		}
		result.Capabilities.Experimental["compact_responses"] = true
	})

	s := server.NewMCPServer(
		"json2pptx",
		Version,
		server.WithToolCapabilities(false),
		server.WithHooks(hooks),
	)

	// Register tools
	s.AddTool(mcpGenerateTool(), mc.handleGenerate)
	s.AddTool(mcpListTemplatesTool(), mc.handleListTemplates)
	s.AddTool(mcpGetDataFormatHintsTool(), handleGetDataFormatHints)
	s.AddTool(mcpGetChartCapabilitiesTool(), handleGetChartCapabilities)
	s.AddTool(mcpGetDiagramCapabilitiesTool(), handleGetDiagramCapabilities)
	s.AddTool(mcpValidateTool(), mc.handleValidate)
	s.AddTool(mcpListPatternsTool(), handleListPatterns)
	s.AddTool(mcpShowPatternTool(), handleShowPattern)
	s.AddTool(mcpValidatePatternTool(), handleValidatePattern)
	s.AddTool(mcpExpandPatternTool(), mc.handleExpandPattern)
	s.AddTool(mcpListIconsTool(), handleListIcons)
	s.AddTool(mcpTableDensityGuideTool(), mc.handleTableDensityGuide)
	s.AddTool(mcpResolveThemeTool(), mc.handleResolveTheme)
	s.AddTool(mcpRenderSlideImageTool(), mc.handleRenderSlideImage)
	s.AddTool(mcpRenderDeckThumbnailsTool(), mc.handleRenderDeckThumbnails)
	s.AddTool(mcpScoreDeckTool(), mc.handleScoreDeck)

	slog.Info("starting json2pptx MCP server",
		"version", Version,
		"templates_dir", mc.templatesDir,
		"output_dir", mc.outputDir,
	)

	return server.ServeStdio(s)
}

// --- Tool definitions ---

func mcpGenerateTool() mcp.Tool {
	return mcp.NewTool("generate_presentation",
		mcp.WithDescription("Generate a PowerPoint presentation from JSON slide definitions. Returns the output file path on success."),
		mcp.WithString("json_input",
			mcp.Required(),
			mcp.Description(`JSON string containing the presentation definition. Use list_templates to discover available template names, layout_ids, and placeholder_ids.

Minimal example:
{"template":"my-template","slides":[{"layout_id":"slideLayout1","content":[{"placeholder_id":"title","type":"text","text_value":"Hello World"}]}]}

Content types and their value fields:
- "text": "text_value":"string"
- "bullets": "bullets_value":["item1","item2"]
- "body_and_bullets": "body_and_bullets_value":{"body":"...","bullets":["..."],"trailing_body":"..."}
- "bullet_groups": "bullet_groups_value":{"body":"...","groups":[{"header":"...","bullets":["..."]}],"trailing_body":"..."}
- "table": "table_value":{"headers":["H1","H2"],"rows":[["a","b"],["c","d"]]}
- "chart": "chart_value":{"type":"bar|line|pie|radar|scatter|bubble|waterfall","title":"...","data":{...}}
- "diagram": "diagram_value":{"type":"timeline|process_flow|pyramid|venn|swot|org_chart|gantt|matrix_2x2|porters_five_forces|house_diagram|business_model_canvas|value_chain|nine_box_talent|kpi_dashboard|heatmap|fishbone|pestel|panel_layout","title":"...","data":{...}}
- "image": "image_value":{"path":"/path/to/image.png","alt":"description"}

Named patterns (optional per-slide, XOR with shape_grid): "pattern" expands a named pattern into a shape_grid. Use list_patterns/show_pattern to discover names and schemas.
Example: {"pattern":{"name":"kpi-3up","values":{"items":[{"label":"Revenue","value":"$1.2M"},{"label":"Growth","value":"+15%"},{"label":"Users","value":"4.3K"}]}}}

Shape grid (optional per-slide, XOR with pattern): "shape_grid" places preset geometry shapes in a grid layout.
Example: {"shape_grid":{"columns":3,"rows":[{"cells":[{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 1"}},{"shape":{"geometry":"rightArrow","fill":"#70AD47"}},{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 2"}}]}]}}
Cell types: "shape" (preset geometry with fill/line/text) or "table" (same as table content type).
Common geometries: rect, roundRect, ellipse, diamond, chevron, rightArrow, hexagon, plus, star5, donut, flowChartProcess, flowChartDecision, flowChartTerminator.
Grid options: "columns" (number or width array), "gap"/"col_gap"/"row_gap" (points), "bounds" (percentage {x,y,width,height}).
Cell options: "col_span", "row_span" for merged cells. Shape options: "geometry", "fill" (color string or {color,alpha}), "line" ({color,width,dash}), "text" (string or {content,size,bold,italic,align,vertical_align,color,font,inset_left,inset_right,inset_top,inset_bottom}), "rotation", "adjustments".

Optional top-level fields: "output_filename", "defaults":{"table_style":{...},"cell_style":{...}} (swap-only deck-level defaults applied before validation), "footer":{"enabled":true,"left_text":"..."}, "theme_override":{"colors":{},"title_font":"...","body_font":"..."}.
Optional slide fields: "slide_type", "speaker_notes", "source", "transition", "build".

Split slide (optional, replaces a slide entry): {"type":"split_slide","by":"table.rows","layout_id":"...","content":[...]} auto-paginates overflowing table rows across multiple slides.`),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: output.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("strict_fit",
			mcp.Description("Text-fit checking mode: off (skip fit checks), warn (default; report overflow warnings), or strict (refuse generation if any cell overflows)."),
			mcp.Enum("off", "warn", "strict"),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, include fit_findings in the response with text overflow, placeholder overflow, footer collision, and bounds-check findings. Default: false."),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit (default: 5 per slide). Default: false."),
		),
	)
}

func mcpListTemplatesTool() mcp.Tool {
	return mcp.NewTool("list_templates",
		mcp.WithDescription(`List available presentation templates with their layouts, theme colors, and capabilities.

Response shape per template (compact/full modes): name, aspect_ratio, layout_count, theme_colors (scheme→hex map), color_roles (primary_fill, secondary_fill, body_fill, body_text, white_text_safe), title_font, body_font, layout_names, table_styles [{id,name}]. Full mode adds layouts with placeholders and capacity.
Response also includes: supported_types (slide/chart/diagram/grid types, shape_geometries, chart_capabilities, diagram_capabilities), data_format_hints_digest (use get_data_format_hints to fetch full hints when digest changes).`),
		mcp.WithString("template",
			mcp.Description("Analyze a single template by name (optional, omit to list all)."),
		),
		mcp.WithString("mode",
			mcp.Description("Detail level: list (names only), compact (names + theme), or full (all placeholders)."),
			mcp.Enum("list", "compact", "full"),
		),
	)
}

func mcpGetDataFormatHintsTool() mcp.Tool {
	return mcp.NewTool("get_data_format_hints",
		mcp.WithDescription("Fetch the full data_format_hints map for all chart and diagram types. Use the digest from list_templates to avoid refetching when hints haven't changed. Note: list_templates is the canonical bundled discovery tool — it returns templates, supported types, chart/diagram capabilities, and a data_format_hints digest in a single call. Use this tool only when you need to fetch the full hints after a digest change."),
		mcp.WithString("digest",
			mcp.Description("Digest from a previous list_templates response. If it matches the current hints, a not_modified response is returned instead of the full map."),
		),
	)
}

func mcpValidateTool() mcp.Tool {
	return mcp.NewTool("validate_input",
		mcp.WithDescription("Validate a JSON presentation definition without generating output. Returns validation errors or success. When fit_report is true, also runs per-cell text overflow measurement and includes findings in the result."),
		mcp.WithString("json_input",
			mcp.Required(),
			mcp.Description(`JSON string containing the presentation definition to validate. Same format as generate_presentation json_input.

Example: {"template":"my-template","slides":[{"layout_id":"slideLayout1","content":[{"placeholder_id":"title","type":"text","text_value":"Hello"}]}]}`),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, run per-cell text overflow measurement and include NDJSON-style fit findings in the result. Default: false."),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit (default: 5 per slide). Default: false."),
		),
	)
}

// --- Tool handlers ---

func (mc *mcpConfig) handleGenerate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:gocyclo
	jsonStr, err := request.RequireString("json_input")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "json_input is required"), nil
	}

	// Parse JSON input — reject trailing data.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "json_input", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Apply deck-level defaults before any validation or conversion.
	applyDefaults(&input)

	// Collect all boundary diagnostics before proceeding.
	var boundaryDiags []diagnostics.Diagnostic

	// Required fields.
	if input.Template == "" {
		boundaryDiags = append(boundaryDiags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required in JSON input",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
		})
	}
	if len(input.Slides) == 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
		})
	}

	// Unknown keys — promoted to errors on the agent path.
	for _, ve := range checkInputUnknownKeys([]byte(jsonStr)) {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationError(ve))
	}

	// Enum validation — reject unknown values for transition, transition_speed, build, background.fit.
	if enumErrs := checkInputEnumValues(&input); len(enumErrs) > 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationErrors(enumErrs)...)
	}

	// Fail fast if any boundary diagnostic is an error.
	if diagnostics.HasErrors(boundaryDiags) {
		return api.MCPDiagnosticsError(boundaryDiags), nil
	}

	// Text-fit checking via strict_fit parameter (default: warn).
	strictFit := "warn"
	if sf, err := request.RequireString("strict_fit"); err == nil && sf != "" {
		strictFit = sf
	}
	if strictFit != "off" {
		if err := checkStrictFit(&input, strictFit); err != nil {
			return api.MCPDiagnosticsError(diagnostics.FromJoinedError(err, "STRICT_FIT")), nil
		}
	}

	// Create output directory
	if err := os.MkdirAll(mc.outputDir, 0755); err != nil {
		return api.MCPSimpleError("OUTPUT_DIR", fmt.Sprintf("failed to create output directory: %v", err)), nil
	}

	// Resolve template
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(input.Template, mc.templatesDir)), nil
	}
	defer templateCleanup()

	// Analyze template
	var syntheticFiles map[string][]byte
	var templateMetadata *types.TemplateMetadata
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	theme := template.ParseTheme(reader)
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        theme,
	}
	template.SynthesizeIfNeeded(reader, analysis)
	templateLayouts := analysis.Layouts
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	templateMetadata, _ = template.ParseMetadata(reader)

	// Resolve relative icon paths against CWD (MCP receives inline JSON, not a file path)
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if iconErr := resolveIconPaths(input.Slides, cwd); iconErr != nil {
			return api.MCPSimpleError("ICON_PATH", fmt.Sprintf("icon path error: %v", iconErr)), nil
		}
	}

	// Convert slides
	slideSpecs, err := convertPresentationSlides(input.Slides, templateLayouts, slideWidth, slideHeight, templateMetadata)
	if err != nil {
		return api.MCPSimpleError("INVALID_SLIDE", fmt.Sprintf("invalid slide specification: %v", err)), nil
	}

	// Pre-validate chart/diagram data (unknown keys already caught at boundary).
	inputWarnings := validateSlidesChartData(input.Slides)

	// Determine output filename
	outputFilename := sanitizeOutputFilename(input.OutputFilename)
	// Check for override from MCP request
	if reqFilename, err := request.RequireString("output_filename"); err == nil && reqFilename != "" {
		outputFilename = sanitizeOutputFilename(reqFilename)
	}
	outputPath := filepath.Join(mc.outputDir, outputFilename)

	// Generate
	startTime := time.Now()
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
		StrictFit:             strictFit,
	}

	if input.Footer != nil && input.Footer.Enabled {
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
		return api.MCPSimpleError("GENERATION_FAILED", fmt.Sprintf("generation failed: %v", err)), nil
	}

	duration := time.Since(startTime)

	// Merge input-layer warnings with generation warnings
	allWarnings := append(inputWarnings, result.Warnings...)

	// Collect fit findings when requested.
	var fitFindings []patterns.FitFinding
	if fitReport, _ := request.GetArguments()["fit_report"].(bool); fitReport {
		fitFindings = collectFitFindings(&input, templateLayouts, slideWidth, slideHeight)
	}

	// Append render-time fit findings from the generator (overflow, truncation, clamping).
	fitFindings = append(fitFindings, result.FitFindings...)

	// Append contrast auto-fix findings (always emitted, not gated by fit_report).
	fitFindings = append(fitFindings, contrastSwapsToFindings(result.ContrastSwaps)...)

	// Apply per-slide finding budget.
	verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
	fitFindings = BudgetFitFindings(fitFindings, DefaultFindingBudget, verboseFit)

	// Build response
	output := JSONOutput{
		Success:          true,
		OutputPath:       outputPath,
		SlideCount:       result.SlideCount,
		DurationMs:       duration.Milliseconds(),
		Warnings:         allWarnings,
		Quality:          computeQualityScore(input.Slides, allWarnings),
		ValidationErrors: result.ValidationErrors,
		FitFindings:      fitFindings,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcpResult, nil
}

func (mc *mcpConfig) handleListTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mode := "compact"
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		switch m {
		case "list", "compact", "full":
			mode = m
		default:
			return mcpParseErrorWithFix("unknown_enum", "mode",
				fmt.Sprintf("invalid mode %q: must be list, compact, or full", m),
				&diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"allowed": []string{"list", "compact", "full"}}},
			), nil
		}
	}

	templateName, _ := request.RequireString("template")

	// Discover templates
	var templatePaths []string
	if templateName != "" {
		path := filepath.Join(mc.templatesDir, templateName+".pptx")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
		}
		templatePaths = []string{path}
	} else {
		entries, err := os.ReadDir(mc.templatesDir)
		if err != nil {
			return api.MCPSimpleError("TEMPLATES_DIR", fmt.Sprintf("failed to read templates directory: %v", err)), nil
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".pptx" {
				templatePaths = append(templatePaths, filepath.Join(mc.templatesDir, e.Name()))
			}
		}
	}

	var templates []skillTemplateInfo
	for _, path := range templatePaths {
		info, err := analyzeTemplateForSkillInfo(path, mc.cache, mode)
		if err != nil {
			continue
		}
		templates = append(templates, info)
	}

	st := buildSupportedTypes()

	// Replace full data_format_hints with a digest to reduce payload size.
	// Agents fetch the full hints on demand via get_data_format_hints.
	st.DataFormatHintsDigest = computeDataFormatHintsDigest(st.DataFormatHints)
	st.DataFormatHints = nil

	output := skillInfo{
		Tool: skillToolInfo{
			Name:    "json2pptx",
			Version: Version,
		},
		Templates:      templates,
		SupportedTypes: st,
		InputFormats:   []string{"json"},
		OutputFormats:  []string{"pptx"},
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcpResult, nil
}

// dataFormatHintsResponse is the JSON envelope for get_data_format_hints.
type dataFormatHintsResponse struct {
	Digest      string                     `json:"digest"`
	NotModified bool                       `json:"not_modified,omitempty"`
	Hints       map[string]skillDataFormat `json:"data_format_hints,omitempty"`
}

func handleGetDataFormatHints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hints := buildDataFormatHints()
	digest := computeDataFormatHintsDigest(hints)

	// If the caller already has this digest, return a short not_modified response.
	if d, err := request.RequireString("digest"); err == nil && d == digest {
		resp := dataFormatHintsResponse{
			Digest:      digest,
			NotModified: true,
		}
		mcpResult, err := api.MCPSuccessResult(ctx, resp)
		if err != nil {
			return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
		}
		return mcpResult, nil
	}

	resp := dataFormatHintsResponse{
		Digest: digest,
		Hints:  hints,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func mcpGetChartCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_chart_capabilities",
		mcp.WithDescription("Fetch capability metadata for all chart types. Values are TBD (null) until populated in a future release; the struct shape is stable. Note: list_templates already includes chart_capabilities in its supported_types response — prefer that single call for initial discovery."),
	)
}

func mcpGetDiagramCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_diagram_capabilities",
		mcp.WithDescription("Fetch capability metadata for all diagram types. Values are TBD (null) until populated in a future release; the struct shape is stable. Note: list_templates already includes diagram_capabilities in its supported_types response — prefer that single call for initial discovery."),
	)
}

// chartCapabilitiesResponse is the JSON envelope for get_chart_capabilities.
type chartCapabilitiesResponse struct {
	CapabilitiesTBD   bool                      `json:"capabilities_tbd"`
	ChartCapabilities []svggen.ChartCapability   `json:"chart_capabilities"`
}

// diagramCapabilitiesResponse is the JSON envelope for get_diagram_capabilities.
type diagramCapabilitiesResponse struct {
	CapabilitiesTBD       bool                        `json:"capabilities_tbd"`
	DiagramCapabilities   []svggen.DiagramCapability   `json:"diagram_capabilities"`
}

func handleGetChartCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := chartCapabilitiesResponse{
		CapabilitiesTBD:   svggen.CapabilitiesTBD(),
		ChartCapabilities: svggen.ChartCapabilities(),
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func handleGetDiagramCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := diagramCapabilitiesResponse{
		CapabilitiesTBD:     svggen.CapabilitiesTBD(),
		DiagramCapabilities: svggen.DiagramCapabilities(),
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func (mc *mcpConfig) handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, err := request.RequireString("json_input")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "json_input is required"), nil
	}

	// Parse JSON input — reject trailing data.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "json_input", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Apply deck-level defaults before validation.
	applyDefaults(&input)

	// Collect structured diagnostics for unknown keys and enum errors.
	var boundaryDiags []diagnostics.Diagnostic
	for _, ve := range checkInputUnknownKeys([]byte(jsonStr)) {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationError(ve))
	}
	if enumErrs := checkInputEnumValues(&input); len(enumErrs) > 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationErrors(enumErrs)...)
	}

	output := dryRunOutput{
		Valid:       !diagnostics.HasErrors(boundaryDiags),
		Diagnostics: boundaryDiags,
		Slides:      []dryRunSlide{},
	}

	// Validate required fields
	if input.Template == "" {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
		})
	}
	if len(input.Slides) == 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
		})
	}
	if !output.Valid {
		return marshalValidateResult(ctx, output)
	}

	// Resolve and analyze template
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "TEMPLATE_NOT_FOUND", Path: "template", Message: templateNotFoundError(input.Template, mc.templatesDir),
			Severity: diagnostics.SeverityError,
		})
		return marshalValidateResult(ctx, output)
	}
	defer templateCleanup()

	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, mc.cache)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "TEMPLATE_ERROR", Path: "template", Message: fmt.Sprintf("template analysis failed: %v", err),
			Severity: diagnostics.SeverityError,
		})
		return marshalValidateResult(ctx, output)
	}

	// Validate slides against template (layout IDs, placeholder IDs,
	// character limits, content types, chart/diagram data)
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)

	// Fit report: measure per-cell text overflow when requested.
	if fitReport, ok := request.GetArguments()["fit_report"].(bool); ok && fitReport {
		findings := generateFitReport(&input)
		verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
		output.FitFindings = budgetLocalFindings(findings, DefaultFindingBudget, verboseFit)
	}

	return marshalValidateResult(ctx, output)
}

// marshalValidateResult serializes a dryRunOutput as a CallToolResult.
// It backfills the string Errors/Warnings fields from Diagnostics for
// backward compatibility with consumers that read those fields.
func marshalValidateResult(ctx context.Context, output dryRunOutput) (*mcp.CallToolResult, error) {
	// Backfill string fields from diagnostics so both formats are available.
	for _, d := range output.Diagnostics {
		switch d.Severity {
		case diagnostics.SeverityError:
			output.Errors = append(output.Errors, d.Message)
		case diagnostics.SeverityWarning:
			output.Warnings = append(output.Warnings, d.Message)
		}
	}
	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Pattern tool definitions ---

func mcpListPatternsTool() mcp.Tool {
	return mcp.NewTool("list_patterns",
		mcp.WithDescription("List all available named patterns. Patterns are high-level primitives that expand to shape_grid definitions, replacing ~600 tokens of boilerplate with ~100 tokens."),
	)
}

func mcpShowPatternTool() mcp.Tool {
	return mcp.NewTool("show_pattern",
		mcp.WithDescription("Show full details for a named pattern, including its authoritative JSON Schema for values, overrides, and cell_overrides."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name (e.g., kpi-3up, bmc-canvas, card-grid)."),
		),
	)
}

func mcpValidatePatternTool() mcp.Tool {
	return mcp.NewTool("validate_pattern",
		mcp.WithDescription("Validate pattern inputs without expanding. Returns structured errors on failure."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name to validate against."),
		),
		mcp.WithString("values",
			mcp.Required(),
			mcp.Description("JSON string of the pattern's values (structure varies by pattern; use show_pattern to see the schema)."),
		),
		mcp.WithString("overrides",
			mcp.Description("JSON string of pattern-level overrides (optional)."),
		),
		mcp.WithString("cell_overrides",
			mcp.Description("JSON string of per-cell overrides keyed by cell index (optional). Example: {\"0\":{\"fill\":\"#FF0000\"}}"),
		),
		mcp.WithString("callout",
			mcp.Description("JSON string of callout band (optional). Only supported by some patterns (card-grid, comparison-2col). Example: {\"text\":\"Key takeaway\",\"emphasis\":\"bold\"}"),
		),
	)
}

func mcpExpandPatternTool() mcp.Tool {
	return mcp.NewTool("expand_pattern",
		mcp.WithDescription("Expand a named pattern into its full shape_grid definition. Useful for debugging and previewing what a pattern call produces."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name to expand."),
		),
		mcp.WithString("values",
			mcp.Required(),
			mcp.Description("JSON string of the pattern's values."),
		),
		mcp.WithString("overrides",
			mcp.Description("JSON string of pattern-level overrides (optional)."),
		),
		mcp.WithString("cell_overrides",
			mcp.Description("JSON string of per-cell overrides keyed by cell index (optional)."),
		),
		mcp.WithString("theme_template",
			mcp.Description("Template name to use for theme context during expansion. If omitted, a minimal synthesized theme is used."),
		),
	)
}

// --- Pattern tool handlers ---

// patternValidationError is a D10 structured error for pattern validation.
type patternValidationError struct {
	Field   string                  `json:"field"`
	Code    string                  `json:"code,omitempty"`
	Message string                  `json:"message"`
	Fix     *patterns.FixSuggestion `json:"fix,omitempty"`
}

// splitValidationErrors converts a (possibly joined) validation error into
// individual D10 structured errors. If the error is a *patterns.ValidationError,
// the structured fields are extracted directly. Otherwise the field is parsed
// from the error message prefix "pattern-name: field rest…".
func splitValidationErrors(err error) []patternValidationError {
	individual := unwrapErrors(err)

	out := make([]patternValidationError, 0, len(individual))
	for _, e := range individual {
		// Recurse into nested errors.Join from validateCellOverrideKeys.
		if nested := unwrapErrors(e); len(nested) > 1 {
			for _, ne := range nested {
				out = append(out, toPatternValidationError(ne))
			}
			continue
		}
		out = append(out, toPatternValidationError(e))
	}
	return out
}

// unwrapErrors splits an error into individual sub-errors if it implements
// Unwrap() []error (as errors.Join does). Otherwise returns a single-element slice.
func unwrapErrors(err error) []error {
	type unwrapper interface {
		Unwrap() []error
	}
	if joined, ok := err.(unwrapper); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

// toPatternValidationError converts a single error into a patternValidationError,
// preferring structured fields from *patterns.ValidationError when available.
func toPatternValidationError(e error) patternValidationError {
	// Check for structured ValidationError.
	var ve *patterns.ValidationError
	if errors.As(e, &ve) {
		return patternValidationError{
			Field:   ve.Path,
			Code:    ve.Code,
			Message: ve.Message,
			Fix:     ve.Fix,
		}
	}

	// Fallback: parse field from message format "pattern-name: field_path rest…".
	msg := e.Error()
	field := "values"
	if colonIdx := strings.Index(msg, ": "); colonIdx >= 0 {
		rest := msg[colonIdx+2:]
		endIdx := 0
		for endIdx < len(rest) {
			ch := rest[endIdx]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' || ch == '[' || ch == ']' {
				endIdx++
			} else {
				break
			}
		}
		if endIdx > 0 {
			field = rest[:endIdx]
		}
	}

	return patternValidationError{
		Field:   field,
		Message: msg,
	}
}

func handleListPatterns(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reg := patterns.Default()
	all := reg.List()

	entries := make([]skillPatternCompact, len(all))
	for i, p := range all {
		entries[i] = skillPatternCompact{
			Name:    p.Name(),
			Cells:   p.CellsHint(),
			UseWhen: p.UseWhen(),
		}
	}

	mcpResult, err := api.MCPSuccessResult(ctx, entries)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func handleShowPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "name is required"), nil
	}

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		available := reg.List()
		names := make([]string, len(available))
		for i, p := range available {
			names[i] = p.Name()
		}
		msg := fmt.Sprintf("unknown pattern %q", name)
		fix := &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"allowed": names}}
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
			fix = &diagnostics.Fix{Kind: "replace_value", Params: map[string]any{"suggestion": suggestion, "allowed": names}}
		}
		return mcpParseErrorWithFix("UNKNOWN_PATTERN", "name", msg, fix), nil
	}

	schemaJSON := patterns.SchemaJSON(pat)

	result := skillPatternFull{
		Name:        pat.Name(),
		Description: pat.Description(),
		Cells:       "",
		UseWhen:     pat.UseWhen(),
		Version:     pat.Version(),
		Schema:      schemaJSON,
	}
	result.Cells = pat.CellsHint()

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func handleValidatePattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "name is required"), nil
	}
	valuesStr, err := request.RequireString("values")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "values is required"), nil
	}

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", name)
		fix := &diagnostics.Fix{Kind: "use_one_of"}
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
			fix = &diagnostics.Fix{Kind: "replace_value", Params: map[string]any{"suggestion": suggestion}}
		}
		return mcpParseErrorWithFix("UNKNOWN_PATTERN", "name", msg, fix), nil
	}

	// Unmarshal values
	values := pat.NewValues()
	if err := json.Unmarshal([]byte(valuesStr), values); err != nil {
		return mcpParseError("INVALID_JSON", "values", fmt.Sprintf("invalid values JSON: %v", err)), nil
	}

	// Unmarshal overrides
	var overrides any
	if overridesStr, err := request.RequireString("overrides"); err == nil && overridesStr != "" {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal([]byte(overridesStr), overrides); err != nil {
				return mcpParseError("INVALID_JSON", "overrides", fmt.Sprintf("invalid overrides JSON: %v", err)), nil
			}
		}
	}

	// Unmarshal cell_overrides
	var cellOverrides map[int]any
	if coStr, err := request.RequireString("cell_overrides"); err == nil && coStr != "" {
		var rawCO map[string]json.RawMessage
		if err := json.Unmarshal([]byte(coStr), &rawCO); err != nil {
			return mcpParseError("INVALID_JSON", "cell_overrides", fmt.Sprintf("invalid cell_overrides JSON: %v", err)), nil
		}
		cellOverrides = make(map[int]any, len(rawCO))
		for key, raw := range rawCO {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return mcpParseError("INVALID_KEY", fmt.Sprintf("cell_overrides.%s", key), fmt.Sprintf("cell_overrides key %q is not an integer", key)), nil
			}
			co := pat.NewCellOverride()
			if co == nil {
				return api.MCPSimpleError("UNSUPPORTED", fmt.Sprintf("pattern %q does not support cell_overrides", name)), nil
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return mcpParseError("INVALID_JSON", fmt.Sprintf("cell_overrides[%d]", idx), fmt.Sprintf("invalid cell_overrides[%d]: %v", idx, err)), nil
			}
			cellOverrides[idx] = co
		}
	}

	// Validate
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		// Return D10 structured errors — split joined errors into individual entries.
		result := struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}{OK: false, Errors: splitValidationErrors(err)}

		mcpResult, _ := api.MCPSuccessResult(ctx, result)
		return mcpResult, nil
	}

	// Callout support check — parity with expandPattern (0kyd)
	if calloutResult := validateCalloutParam(ctx, request, name, pat); calloutResult != nil {
		return calloutResult, nil
	}

	result := struct {
		OK bool `json:"ok"`
	}{OK: true}
	mcpResult, _ := api.MCPSuccessResult(ctx, result)
	return mcpResult, nil
}

// validateCalloutParam checks the optional "callout" parameter against the
// pattern's CalloutSupport interface. Returns a non-nil result on error,
// or nil when callout is absent or the pattern supports it.
func validateCalloutParam(ctx context.Context, request mcp.CallToolRequest, name string, pat patterns.Pattern) *mcp.CallToolResult {
	calloutStr, err := request.RequireString("callout")
	if err != nil || calloutStr == "" {
		return nil
	}
	var callout patterns.PatternCallout
	if err := json.Unmarshal([]byte(calloutStr), &callout); err != nil {
		return mcpParseError("INVALID_JSON", "callout", fmt.Sprintf("invalid callout JSON: %v", err))
	}
	cs, ok := pat.(patterns.CalloutSupport)
	if ok && cs.SupportsCallout() {
		return nil
	}
	reg := patterns.Default()
	veErr := patterns.ErrCalloutUnsupportedFor(name, reg.CalloutSupportedPatterns())
	result := struct {
		OK     bool                     `json:"ok"`
		Errors []patternValidationError `json:"errors"`
	}{OK: false, Errors: splitValidationErrors(veErr)}
	mcpResult, _ := api.MCPSuccessResult(ctx, result)
	return mcpResult
}

func (mc *mcpConfig) handleExpandPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "name is required"), nil
	}
	valuesStr, err := request.RequireString("values")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "values is required"), nil
	}

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", name)
		fix := &diagnostics.Fix{Kind: "use_one_of"}
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
			fix = &diagnostics.Fix{Kind: "replace_value", Params: map[string]any{"suggestion": suggestion}}
		}
		return mcpParseErrorWithFix("UNKNOWN_PATTERN", "name", msg, fix), nil
	}

	// Build PatternInput for reuse of existing expandPattern logic
	pi := &PatternInput{
		Name:   name,
		Values: json.RawMessage(valuesStr),
	}
	if overridesStr, err := request.RequireString("overrides"); err == nil && overridesStr != "" {
		pi.Overrides = json.RawMessage(overridesStr)
	}
	if coStr, err := request.RequireString("cell_overrides"); err == nil && coStr != "" {
		var rawCO map[string]json.RawMessage
		if err := json.Unmarshal([]byte(coStr), &rawCO); err != nil {
			return mcpParseError("INVALID_JSON", "cell_overrides", fmt.Sprintf("invalid cell_overrides JSON: %v", err)), nil
		}
		pi.CellOverrides = rawCO
	}

	// Build ExpandContext — use template theme if provided, else synthesized minimal
	expandCtx := patterns.ExpandContext{
		SlideWidth:  9144000, // 10 inches in EMU (standard 16:9)
		SlideHeight: 5143500, // 7.5 inches in EMU (standard 16:9 adjusted)
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200, // 0.5 inch margins
			Width: 8229600, Height: 4229100,
		},
	}

	if templateName, err := request.RequireString("theme_template"); err == nil && templateName != "" {
		templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
		if err != nil {
			return api.MCPSimpleError("TEMPLATE_NOT_FOUND", fmt.Sprintf("template %q not found", templateName)), nil
		}
		defer templateCleanup()

		if reader, err := template.OpenTemplate(templatePath); err == nil {
			defer func() { _ = reader.Close() }()
			theme := template.ParseTheme(reader)
			expandCtx.Theme = theme
			w, h := template.ParseSlideDimensions(reader)
			if w > 0 {
				expandCtx.SlideWidth = w
			}
			if h > 0 {
				expandCtx.SlideHeight = h
			}
		}
	}

	// Use expandPattern helper (which handles unmarshal, validate, expand)
	grid, _, err := expandPattern(pi, expandCtx, reg)
	if err != nil {
		return api.MCPDiagnosticsError(diagnostics.FromJoinedError(err, "PATTERN_ERROR")), nil
	}

	// Also provide the pattern version for traceability
	result := struct {
		Pattern   string                   `json:"pattern"`
		Version   int                      `json:"version"`
		ShapeGrid *jsonschema.ShapeGridInput `json:"shape_grid"`
	}{
		Pattern:   pat.Name(),
		Version:   pat.Version(),
		ShapeGrid: grid,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Icon tool ---

func mcpListIconsTool() mcp.Tool {
	return mcp.NewTool("list_icons",
		mcp.WithDescription("List available icon names for use in shape_grid cells via {\"icon\":{\"name\":\"icon-name\"}}. Icons are bundled SVGs in two sets: outline (default, stroke-based) and filled (solid). Use set:name syntax (e.g. \"filled:chart-pie\") to select a set; plain names default to outline."),
		mcp.WithString("set",
			mcp.Description("Icon set to list: outline, filled, or omit for all sets."),
			mcp.Enum("outline", "filled"),
		),
		mcp.WithString("search",
			mcp.Description("Substring filter applied to icon names. Case-insensitive. Example: \"chart\" returns chart-pie, chart-bar, etc."),
		),
	)
}

// iconSetResult is the JSON shape for each icon set in the list_icons response.
type iconSetResult struct {
	Set   string   `json:"set"`
	Count int      `json:"count"`
	Names []string `json:"names"`
}

func handleListIcons(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sets := []string{"outline", "filled"}
	if s, err := request.RequireString("set"); err == nil && s != "" {
		sets = []string{s}
	}

	search, _ := request.RequireString("search")
	search = strings.ToLower(strings.TrimSpace(search))

	result := make([]iconSetResult, 0, len(sets))
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return api.MCPSimpleError("ICON_LIST", fmt.Sprintf("listing %s icons: %v", s, err)), nil
		}
		if search != "" {
			filtered := make([]string, 0, len(names)/4)
			for _, n := range names {
				if strings.Contains(n, search) {
					filtered = append(filtered, n)
				}
			}
			names = filtered
		}
		result = append(result, iconSetResult{
			Set:   s,
			Count: len(names),
			Names: names,
		})
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Render tools ---

func mcpRenderSlideImageTool() mcp.Tool {
	return mcp.NewTool("render_slide_image",
		mcp.WithDescription(`Render a single slide from a generated PPTX to a PNG image. Returns base64-encoded PNG data (or a file path if the image exceeds 200KB).

Requires LibreOffice and ImageMagick (magick) on PATH. Use this for detailed visual inspection of a specific slide.

Cost note: a density=100 slide is typically 20-80KB base64. Higher densities produce larger payloads.`),
		mcp.WithString("pptx_path",
			mcp.Required(),
			mcp.Description("Path to the PPTX file to render. Use the output_path from generate_presentation."),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("0-based slide index to render. Default: 0."),
		),
		mcp.WithNumber("density",
			mcp.Description("DPI for rendering. Higher = sharper but larger. Default: 100. Range: 50-300."),
		),
	)
}

func mcpRenderDeckThumbnailsTool() mcp.Tool {
	return mcp.NewTool("render_deck_thumbnails",
		mcp.WithDescription(`Render all slides in a PPTX as low-resolution PNG thumbnails. Returns an array of base64-encoded PNGs.

Requires LibreOffice and ImageMagick (magick) on PATH. Use this for a quick visual overview of the entire deck.

Cost note: at density=50, each thumbnail is typically 5-20KB base64. A 10-slide deck is ~100-200KB total.`),
		mcp.WithString("pptx_path",
			mcp.Required(),
			mcp.Description("Path to the PPTX file to render. Use the output_path from generate_presentation."),
		),
		mcp.WithNumber("density",
			mcp.Description("DPI for thumbnails. Lower = smaller payloads. Default: 50. Range: 25-150."),
		),
		mcp.WithNumber("max_slides",
			mcp.Description("Maximum number of slides to render. Default: 50."),
		),
	)
}

func (mc *mcpConfig) handleRenderSlideImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "pptx_path is required"), nil
	}

	if _, err := os.Stat(pptxPath); os.IsNotExist(err) {
		return api.MCPSimpleError("FILE_NOT_FOUND", fmt.Sprintf("pptx file not found: %s", pptxPath)), nil
	}

	slideIndex := 0
	if v, ok := request.GetArguments()["slide_index"].(float64); ok {
		slideIndex = int(v)
	}

	density := 100
	if v, ok := request.GetArguments()["density"].(float64); ok {
		d := int(v)
		if d < 50 {
			d = 50
		} else if d > 300 {
			d = 300
		}
		density = d
	}

	img, err := render.RenderSlide(pptxPath, slideIndex, density)
	if err != nil {
		code := "RENDER_FAILED"
		if strings.Contains(err.Error(), "not found on PATH") {
			if strings.Contains(err.Error(), "libreoffice") {
				code = "LIBREOFFICE_UNAVAILABLE"
			} else {
				code = "IMAGEMAGICK_UNAVAILABLE"
			}
		}
		return api.MCPSimpleError(code, err.Error()), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, img)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func (mc *mcpConfig) handleRenderDeckThumbnails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "pptx_path is required"), nil
	}

	if _, err := os.Stat(pptxPath); os.IsNotExist(err) {
		return api.MCPSimpleError("FILE_NOT_FOUND", fmt.Sprintf("pptx file not found: %s", pptxPath)), nil
	}

	density := 50
	if v, ok := request.GetArguments()["density"].(float64); ok {
		d := int(v)
		if d < 25 {
			d = 25
		} else if d > 150 {
			d = 150
		}
		density = d
	}

	maxSlides := 50
	if v, ok := request.GetArguments()["max_slides"].(float64); ok {
		m := int(v)
		if m > 0 {
			maxSlides = m
		}
	}

	deckResult, err := render.RenderDeck(pptxPath, density, maxSlides)
	if err != nil {
		code := "RENDER_FAILED"
		if strings.Contains(err.Error(), "not found on PATH") {
			if strings.Contains(err.Error(), "libreoffice") {
				code = "LIBREOFFICE_UNAVAILABLE"
			} else {
				code = "IMAGEMAGICK_UNAVAILABLE"
			}
		}
		return api.MCPSimpleError(code, err.Error()), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, deckResult)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}
