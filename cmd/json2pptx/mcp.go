package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// mcpConfig holds the resolved configuration for MCP tool handlers.
type mcpConfig struct {
	templatesDir string
	outputDir    string
	cfg          config.Config
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
		mcp.WithDescription("Validate a JSON presentation definition without generating output. Returns validation errors or success."),
		mcp.WithString("json_input",
			mcp.Required(),
			mcp.Description(`JSON string containing the presentation definition to validate. Same format as generate_presentation json_input.

Example: {"template":"my-template","slides":[{"layout_id":"slideLayout1","content":[{"placeholder_id":"title","type":"text","text_value":"Hello"}]}]}`),
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

	// Create output directory
	if err := os.MkdirAll(mc.outputDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create output directory: %v", err)), nil
	}

	// Resolve template
	templatePath, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return mcp.NewToolResultError(templateNotFoundError(input.Template, mc.templatesDir)), nil
	}

	// Analyze template
	var templateLayouts []types.LayoutMetadata
	var syntheticFiles map[string][]byte
	var slideWidth, slideHeight int64
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
		}
	}

	// Resolve relative icon paths against CWD (MCP receives inline JSON, not a file path)
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if iconErr := resolveIconPaths(input.Slides, cwd); iconErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("icon path error: %v", iconErr)), nil
		}
	}

	// Convert slides
	slideSpecs, err := convertPresentationSlides(input.Slides, templateLayouts, slideWidth, slideHeight)
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

	cache := template.NewMemoryCache(24 * time.Hour)

	var templates []skillTemplateInfo
	for _, path := range templatePaths {
		info, err := analyzeTemplateForSkillInfo(path, cache, mode)
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
		OutputFormats:  []string{"pptx", "pdf"},
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
	templatePath, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, templateNotFoundError(input.Template, mc.templatesDir))
		return marshalValidateResult(output)
	}

	cache := template.NewMemoryCache(24 * time.Hour)
	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, fmt.Sprintf("template analysis failed: %v", err))
		return marshalValidateResult(output)
	}

	// Validate slides against template (layout IDs, placeholder IDs,
	// character limits, content types, chart/diagram data)
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)

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
