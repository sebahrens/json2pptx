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

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
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

	s := server.NewMCPServer(
		"json2pptx",
		Version,
		server.WithToolCapabilities(false),
	)

	// Register tools
	s.AddTool(mcpGenerateTool(), mc.handleGenerate)
	s.AddTool(mcpListTemplatesTool(), mc.handleListTemplates)
	s.AddTool(mcpValidateTool(), mc.handleValidate)
	s.AddTool(mcpListPatternsTool(), handleListPatterns)
	s.AddTool(mcpShowPatternTool(), handleShowPattern)
	s.AddTool(mcpValidatePatternTool(), handleValidatePattern)
	s.AddTool(mcpExpandPatternTool(), mc.handleExpandPattern)

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

Shape grid (optional per-slide): "shape_grid" places preset geometry shapes in a grid layout.
Example: {"shape_grid":{"columns":3,"rows":[{"cells":[{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 1"}},{"shape":{"geometry":"rightArrow","fill":"#70AD47"}},{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 2"}}]}]}}
Cell types: "shape" (preset geometry with fill/line/text) or "table" (same as table content type).
Common geometries: rect, roundRect, ellipse, diamond, chevron, rightArrow, hexagon, plus, star5, donut, flowChartProcess, flowChartDecision, flowChartTerminator.
Grid options: "columns" (number or width array), "gap"/"col_gap"/"row_gap" (points), "bounds" (percentage {x,y,width,height}).
Cell options: "col_span", "row_span" for merged cells. Shape options: "geometry", "fill" (color string or {color,alpha}), "line" ({color,width,dash}), "text" (string or {content,size,bold,italic,align,vertical_align,color,font,inset_left,inset_right,inset_top,inset_bottom}), "rotation", "adjustments".

Optional top-level fields: "output_filename", "footer":{"enabled":true,"left_text":"..."}, "theme_override":{"colors":{},"title_font":"...","body_font":"..."}.
Optional slide fields: "slide_type", "speaker_notes", "source", "transition", "build".`),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: output.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("strict_fit",
			mcp.Description("Text-fit checking mode: off (skip fit checks), warn (default; report overflow warnings), or strict (refuse generation if any cell overflows)."),
			mcp.Enum("off", "warn", "strict"),
		),
	)
}

func mcpListTemplatesTool() mcp.Tool {
	return mcp.NewTool("list_templates",
		mcp.WithDescription("List available presentation templates with their layouts, theme colors, and capabilities."),
		mcp.WithString("template",
			mcp.Description("Analyze a single template by name (optional, omit to list all)."),
		),
		mcp.WithString("mode",
			mcp.Description("Detail level: list (names only), compact (names + theme), or full (all placeholders)."),
			mcp.Enum("list", "compact", "full"),
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
	)
}

// --- Tool handlers ---

func (mc *mcpConfig) handleGenerate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, err := request.RequireString("json_input")
	if err != nil {
		return mcp.NewToolResultError("json_input is required"), nil
	}

	// Parse JSON input
	var input PresentationInput
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Validate
	if input.Template == "" {
		return mcp.NewToolResultError("template is required in JSON input"), nil
	}
	if len(input.Slides) == 0 {
		return mcp.NewToolResultError("at least one slide is required"), nil
	}

	// Text-fit checking via strict_fit parameter (default: warn).
	strictFit := "warn"
	if sf, err := request.RequireString("strict_fit"); err == nil && sf != "" {
		strictFit = sf
	}
	if strictFit != "off" {
		if err := checkStrictFit(&input, strictFit); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("strict-fit check failed: %v", err)), nil
		}
	}

	// Create output directory
	if err := os.MkdirAll(mc.outputDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create output directory: %v", err)), nil
	}

	// Resolve template
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return mcp.NewToolResultError(templateNotFoundError(input.Template, mc.templatesDir)), nil
	}
	defer templateCleanup()

	// Analyze template
	var templateLayouts []types.LayoutMetadata
	var syntheticFiles map[string][]byte
	var slideWidth, slideHeight int64
	var templateMetadata *types.TemplateMetadata
	if reader, err := template.OpenTemplate(templatePath); err == nil {
		defer func() { _ = reader.Close() }()
		if layouts, err := template.ParseLayouts(reader); err == nil {
			theme := template.ParseTheme(reader)
			slideWidth, slideHeight = template.ParseSlideDimensions(reader)
			analysis := &types.TemplateAnalysis{
				TemplatePath: templatePath,
				SlideWidth:   slideWidth,
				SlideHeight:  slideHeight,
				Layouts:      layouts,
				Theme:        theme,
			}
			template.SynthesizeIfNeeded(reader, analysis)
			templateLayouts = analysis.Layouts
			if analysis.Synthesis != nil {
				syntheticFiles = analysis.Synthesis.SyntheticFiles
			}
			templateMetadata, _ = template.ParseMetadata(reader)
		}
	}

	// Resolve relative icon paths against CWD (MCP receives inline JSON, not a file path)
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if iconErr := resolveIconPaths(input.Slides, cwd); iconErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("icon path error: %v", iconErr)), nil
		}
	}

	// Convert slides
	slideSpecs, err := convertPresentationSlides(input.Slides, templateLayouts, slideWidth, slideHeight, templateMetadata)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid slide specification: %v", err)), nil
	}

	// Pre-validate chart/diagram data structures via svggen Validate().
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
		return mcp.NewToolResultError(fmt.Sprintf("generation failed: %v", err)), nil
	}

	duration := time.Since(startTime)

	// Merge input-layer warnings with generation warnings
	allWarnings := append(inputWarnings, result.Warnings...)

	// Build response
	output := JSONOutput{
		Success:    true,
		OutputPath: outputPath,
		SlideCount: result.SlideCount,
		DurationMs: duration.Milliseconds(),
		Warnings:   allWarnings,
		Quality:    computeQualityScore(input.Slides, allWarnings),
	}

	responseJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

func (mc *mcpConfig) handleListTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mode := "compact"
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		switch m {
		case "list", "compact", "full":
			mode = m
		default:
			return mcp.NewToolResultError(fmt.Sprintf("invalid mode %q: must be list, compact, or full", m)), nil
		}
	}

	templateName, _ := request.RequireString("template")

	// Discover templates
	var templatePaths []string
	if templateName != "" {
		path := filepath.Join(mc.templatesDir, templateName+".pptx")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return mcp.NewToolResultError(templateNotFoundError(templateName, mc.templatesDir)), nil
		}
		templatePaths = []string{path}
	} else {
		entries, err := os.ReadDir(mc.templatesDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read templates directory: %v", err)), nil
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

	output := skillInfo{
		Tool: skillToolInfo{
			Name:    "json2pptx",
			Version: Version,
		},
		Templates:      templates,
		SupportedTypes: buildSupportedTypes(),
		InputFormats:   []string{"json"},
		OutputFormats:  []string{"pptx"},
	}

	responseJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

func (mc *mcpConfig) handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, err := request.RequireString("json_input")
	if err != nil {
		return mcp.NewToolResultError("json_input is required"), nil
	}

	// Parse JSON input
	var input PresentationInput
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	output := dryRunOutput{
		Valid:    true,
		Warnings: []string{},
		Slides:   []dryRunSlide{},
	}

	// Validate required fields
	if input.Template == "" {
		output.Valid = false
		output.Errors = append(output.Errors, "template is required")
	}
	if len(input.Slides) == 0 {
		output.Valid = false
		output.Errors = append(output.Errors, "at least one slide is required")
	}
	if !output.Valid {
		return marshalValidateResult(output)
	}

	// Resolve and analyze template
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, templateNotFoundError(input.Template, mc.templatesDir))
		return marshalValidateResult(output)
	}
	defer templateCleanup()

	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, mc.cache)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, fmt.Sprintf("template analysis failed: %v", err))
		return marshalValidateResult(output)
	}

	// Validate slides against template (layout IDs, placeholder IDs,
	// character limits, content types, chart/diagram data)
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)

	// Fit report: measure per-cell text overflow when requested.
	if fitReport, ok := request.GetArguments()["fit_report"].(bool); ok && fitReport {
		output.FitFindings = generateFitReport(&input)
	}

	return marshalValidateResult(output)
}

// marshalValidateResult serializes a dryRunOutput as a CallToolResult.
func marshalValidateResult(output dryRunOutput) (*mcp.CallToolResult, error) {
	responseJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(responseJSON)), nil
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

func handleListPatterns(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	responseJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(responseJSON)), nil
}

func handleShowPattern(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
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
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s; available: %v", msg, names)), nil
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

	responseJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(responseJSON)), nil
}

func handleValidatePattern(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	valuesStr, err := request.RequireString("values")
	if err != nil {
		return mcp.NewToolResultError("values is required"), nil
	}

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", name)
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return mcp.NewToolResultError(msg), nil
	}

	// Unmarshal values
	values := pat.NewValues()
	if err := json.Unmarshal([]byte(valuesStr), values); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid values JSON: %v", err)), nil
	}

	// Unmarshal overrides
	var overrides any
	if overridesStr, err := request.RequireString("overrides"); err == nil && overridesStr != "" {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal([]byte(overridesStr), overrides); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid overrides JSON: %v", err)), nil
			}
		}
	}

	// Unmarshal cell_overrides
	var cellOverrides map[int]any
	if coStr, err := request.RequireString("cell_overrides"); err == nil && coStr != "" {
		var rawCO map[string]json.RawMessage
		if err := json.Unmarshal([]byte(coStr), &rawCO); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid cell_overrides JSON: %v", err)), nil
		}
		cellOverrides = make(map[int]any, len(rawCO))
		for key, raw := range rawCO {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("cell_overrides key %q is not an integer", key)), nil
			}
			co := pat.NewCellOverride()
			if co == nil {
				return mcp.NewToolResultError(fmt.Sprintf("pattern %q does not support cell_overrides", name)), nil
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid cell_overrides[%d]: %v", idx, err)), nil
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

		responseJSON, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(responseJSON)), nil
	}

	result := struct {
		OK bool `json:"ok"`
	}{OK: true}
	responseJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(responseJSON)), nil
}

func (mc *mcpConfig) handleExpandPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	valuesStr, err := request.RequireString("values")
	if err != nil {
		return mcp.NewToolResultError("values is required"), nil
	}

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", name)
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return mcp.NewToolResultError(msg), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("invalid cell_overrides JSON: %v", err)), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("template %q not found", templateName)), nil
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
		return mcp.NewToolResultError(err.Error()), nil
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

	responseJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(responseJSON)), nil
}
