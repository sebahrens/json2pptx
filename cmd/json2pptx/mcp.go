package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/policy/emoji"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

// --- Tool definitions ---

func mcpGenerateTool() mcp.Tool {
	return mcp.NewTool("generate_presentation",
		mcp.WithDescription("Generate a PowerPoint presentation from JSON slide definitions. Returns the output file path on success."),
		mcp.WithRawOutputSchema(outputSchemaGenerate),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Presentation definition. Use list_templates to discover available template names, layout_ids, and placeholder_ids.

Minimal example:
{"template":"my-template","slides":[{"layout_id":"slideLayout1","content":[{"placeholder_id":"title","type":"text","text_value":"Hello World"}]}]}

Content types and their value fields:
- "text": "text_value":"string"
- "bullets": "bullets_value":["item1","item2"]
- "body_and_bullets": "body_and_bullets_value":{"body":"...","bullets":["..."],"trailing_body":"..."}
- "bullet_groups": "bullet_groups_value":{"body":"...","groups":[{"header":"...","bullets":["..."]}],"trailing_body":"..."}
- "table": "table_value":{"headers":["H1","H2"],"rows":[["a","b"],["c","d"]]}
- "chart": "chart_value":{"type":"bar|grouped_bar|stacked_bar|line|area|stacked_area|pie|donut|scatter|bubble|radar|waterfall|funnel|gauge|treemap","title":"...","data":{...}}
- "diagram": "diagram_value":{"type":"timeline|process_flow|pyramid|venn|swot|org_chart|gantt|matrix_2x2|porters_five_forces|house_diagram|business_model_canvas|value_chain|nine_box_talent|kpi_dashboard|heatmap|fishbone|pestel|panel_layout|icon_columns|icon_rows|stat_cards","title":"...","data":{...}}
- "image": "image_value":{"path":"/path/to/image.png","alt":"description"}

Named patterns (optional per-slide, XOR with shape_grid): "pattern" expands a named pattern into a shape_grid. Use list_patterns/show_pattern to discover names and schemas.
Example: {"pattern":{"name":"kpi-3up","values":{"items":[{"label":"Revenue","value":"$1.2M"},{"label":"Growth","value":"+15%"},{"label":"Users","value":"4.3K"}]}}}

Shape grid (optional per-slide, XOR with pattern): "shape_grid" places preset geometry shapes in a grid layout.
Example: {"shape_grid":{"columns":3,"rows":[{"cells":[{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 1"}},{"shape":{"geometry":"rightArrow","fill":"#70AD47"}},{"shape":{"geometry":"roundRect","fill":"#4472C4","text":"Step 2"}}]}]}}
Cell types: "shape" (preset geometry with fill/line/text) or "table" (same as table content type).
Common geometries: rect, roundRect, ellipse, diamond, chevron, rightArrow, hexagon, plus, star5, donut, flowChartProcess, flowChartDecision, flowChartTerminator.
Grid options: "columns" (number or width array), "gap"/"col_gap"/"row_gap" (points), "bounds" (percentage {x,y,width,height}).
Cell options: "col_span", "row_span" for merged cells. Shape options: "geometry", "fill" (color string or {color,alpha}), "line" ({color,width,dash}), "text" (string or {content,size,bold,italic,align,vertical_align,color,font,inset_left,inset_right,inset_top,inset_bottom}), "rotation", "adjustments".

Optional top-level fields: "output_filename", "accent_strategy" ("primary"|"rotate"|"section-keyed" — controls default accent color rotation across slides), "defaults":{"table_style":{...},"cell_style":{...}} (swap-only deck-level defaults applied before validation), "footer":{"enabled":true,"left_text":"..."}, "theme_override":{"colors":{},"title_font":"...","body_font":"..."}.

Advanced deck-level fields (all opt-in; see get_capabilities.features for support + usage hints):
- "chrome":{"confidentiality":"...","client_name":"...","project_code":"...","footer_date":"...","page_numbers":{"enabled":true,"format":"{current} / {total}","skip":["title","closing"]},"section_crumb":true} — deck-wide footer chrome with page numbers and an optional section title crumb; auto-suppressed on title/closing slides.
- "structure":{"cover":{...},"closing":{...},"auto_agenda":true,"sections":[{"title":"...","slides":[...]}]} — expands into a flat slide sequence with auto section dividers + optional agenda slide. Mutually exclusive with top-level "slides".

Optional slide fields: "slide_type", "speaker_notes", "source", "transition", "build".

Split slide (optional, replaces a slide entry): {"type":"split_slide","by":"table.rows","layout_id":"...","content":[...]} auto-paginates overflowing table rows across multiple slides.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name (use list_templates to discover available names)"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: output.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("strict_fit",
			mcp.Description("Text-fit checking mode: off (skip fit checks), warn (default; report overflow warnings), or strict (refuse generation if any cell overflows)."),
			mcp.Enum("off", "warn", "strict"),
			mcp.DefaultString("warn"),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, include fit_findings in the response with text overflow, placeholder overflow, footer collision, and bounds-check findings. Default: false."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit (default: 5 per slide). Default: false."),
		),
		mcp.WithBoolean("strict_unknown_keys",
			mcp.Description("When true, unknown JSON keys are errors that block generation. When false (default), unknown keys are reported as warnings and generation proceeds."),
		),
		mcp.WithString("output_validation",
			mcp.Description("Post-generation PPTX validation policy: off (skip), warn (run validation, include findings in response), or strict (default; fail generation with diagnostics envelope when blocking findings exist). On strict failure, the response carries next_tool_call=repair_slide so agents can chain a fix without re-deriving the protocol."),
			mcp.Enum("off", "warn", "strict"),
			mcp.DefaultString("strict"),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used as the root for resolving relative local-asset paths (image_value.path, background.image, shape_grid image/icon paths). Required when any slide references a relative path and the agent cannot guarantee the server CWD matches the JSON's authoring directory. When omitted, the server falls back to its process CWD (not portable). Must be an absolute path to an existing directory."),
		),
		idempotencyKeyToolParam(),
	)
}

func mcpListTemplatesTool() mcp.Tool {
	return mcp.NewTool("list_templates",
		mcp.WithDescription(`List available presentation templates with their layouts, theme colors, and capabilities.

Response shape per template (compact/full modes): name, aspect_ratio, layout_count, sha256 (stable content hash), metadata_version, theme_colors (scheme→hex map), color_roles (primary_fill, secondary_fill, body_fill, body_text, white_text_safe), title_font, body_font, semantic_accents (positive/negative/neutral→accent), surface_tints, data_palette, accent_usage_guide (when authored), canonical_layout_ids (canonical name→layout ID), canonical_coverage (per content-bearing family: present + covering layouts), derivable_layouts ([{name, ready, missing}]), layout_names, layout_summaries ([{id, name, canonical_type, placeholders[{id, type, role, max_chars}]}]), table_styles [{id,name}]. Full mode adds layouts with per-layout canonical_type/canonical_family/canonical_confidence and placeholders carrying role, role_confidence, font_size_pt (font-aware max_chars evidence), exact bounds, and capacity.
Response also includes: supported_types (slide/chart/diagram/grid types, shape_geometries, chart_capabilities, diagram_capabilities), data_format_hints_digest (use get_data_format_hints to fetch full hints when digest changes).

Pagination: full-mode payloads can be large. Use cursor + page_size to iterate. The response always includes total_count and page_size; next_cursor is present only when more templates remain.

Projection (token-economy): pass fields="compact" to suppress per-template detail (theme_colors / color_roles / layouts) and keep only names + aspect_ratio + layout_count + table_styles. Pass fields="full" for the legacy full payload. Omitting fields emits a deprecation hint in warnings[] — future releases will switch the default to compact.

Filtering: pass filter="<substring>" to limit the response to templates whose name contains the substring (case-insensitive). Composes with pagination — filter applies before cursor/page_size.`),
		mcp.WithRawOutputSchema(outputSchemaListTemplates),
		mcp.WithString("template",
			mcp.Description("Analyze a single template by name (optional, omit to list all)."),
		),
		mcp.WithString("mode",
			mcp.Description("Legacy detail level: list (names only), compact (names + theme), or full (all placeholders). Prefer fields=compact|full; mode is honored when fields is unset."),
			mcp.Enum("list", "compact", "full"),
		),
		mcp.WithString("fields",
			mcp.Description("Field projection: compact (slim — name, aspect_ratio, layout_count, table_styles) or full (current full payload). When omitted, behavior matches mode (or full when neither is set) and a deprecation hint is returned in warnings[]."),
			mcp.Enum(listFieldsCompact, listFieldsFull),
		),
		mcp.WithString("filter",
			mcp.Description("Case-insensitive substring filter on template name. Applied before pagination."),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque continuation token from a previous response's next_cursor. Omit or pass empty for the first page."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Maximum number of template entries to return. Default: 50. Clamped to [1, 200]."),
		),
		mcp.WithBoolean("read_only",
			mcp.Description("Read-only discovery: skip layout-preview PNG generation so the call writes no cache files (preview_png_path is then omitted from layout summaries / layouts). Set this when gathering template context in a read-only planning context. The response's side_effects block reports whether preview cache writes occurred and the cache directory. Default: false (default mode may write preview PNGs to the cache dir when LibreOffice + ImageMagick are present)."),
			mcp.DefaultBool(false),
		),
	)
}

func mcpGetDataFormatHintsTool() mcp.Tool {
	return mcp.NewTool("get_data_format_hints",
		mcp.WithDescription("Fetch the full data_format_hints map for all chart and diagram types. Use the digest from list_templates to avoid refetching when hints haven't changed. Note: list_templates is the canonical bundled discovery tool — it returns templates, supported types, chart/diagram capabilities, and a data_format_hints digest in a single call. Use this tool only when you need to fetch the full hints after a digest change."),
		mcp.WithRawOutputSchema(outputSchemaGetDataFormatHints),
		mcp.WithString("digest",
			mcp.Description("Digest from a previous list_templates response. If it matches the current hints, a not_modified response is returned instead of the full map."),
		),
	)
}

func mcpValidateTool() mcp.Tool {
	return mcp.NewTool("validate_input",
		mcp.WithDescription("Validate a JSON presentation definition without generating output. Returns validation errors or success. When fit_report is true, also runs per-cell text overflow measurement and includes findings in the result."),
		mcp.WithRawOutputSchema(outputSchemaValidate),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Presentation definition to validate. Same schema as generate_presentation.

Example: {"template":"my-template","slides":[{"layout_id":"slideLayout1","content":[{"placeholder_id":"title","type":"text","text_value":"Hello"}]}]}`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, run per-cell text overflow measurement and include NDJSON-style fit findings in the result. Default: true."),
			mcp.DefaultBool(true),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit (default: 5 per slide). Default: false."),
		),
		mcp.WithBoolean("strict_unknown_keys",
			mcp.Description("When true, unknown JSON keys are errors that block validation. When false (default), unknown keys are reported as warnings."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used as the root for resolving relative local-asset paths during pre-flight (image_value.path, background.image, shape_grid image/icon paths). Same contract as generate_presentation: must be absolute and exist. Omit only if every asset reference is absolute or a URL."),
		),
	)
}

// --- Tool handlers ---

func (mc *mcpConfig) handleGenerate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:gocyclo,gocognit
	// Idempotency: if a matching key is already in the cache, replay the
	// cached response instead of regenerating. The cache is per-process and
	// only holds successful responses, so retries on transient transport
	// failures dedupe but bad-input retries still surface diagnostics.
	idemKey := idempotencyKey(request)
	if cached, ok := mc.idempotency.Get("generate_presentation", idemKey); ok {
		if out, ok := cached.(JSONOutput); ok {
			out.IdempotentReplay = true
			return api.MCPSuccessResult(ctx, out)
		}
	}

	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("generate_presentation", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// Parse JSON input — reject trailing data.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}

	// Apply deck-level defaults before any validation or conversion.
	applyDefaults(&input)

	// Resolve named style references from template settings (priority 3).
	mc.resolveInputNamedSettings(&input)

	// Expand structure block into flat slides (mutually exclusive with
	// top-level slides). Mirrors the CLI path so MCP and CLI agree on the
	// effective slide list before any slide-count checks.
	if structDiags := applyStructureExpansion(&input); len(structDiags) > 0 {
		return api.MCPDiagnosticsError(structDiags), nil
	}

	// Collect all boundary diagnostics before proceeding.
	var boundaryDiags []diagnostics.Diagnostic

	// Required fields.
	if input.Template == "" {
		boundaryDiags = append(boundaryDiags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required in presentation",
			Severity:     diagnostics.SeverityError,
			ExpectedType: "string",
			ExampleValue: "midnight-blue",
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
			NextToolCall: nextCallListTemplates(),
		})
	}
	if len(input.Slides) == 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity:     diagnostics.SeverityError,
			ExpectedType: "array",
			ExampleValue: []any{map[string]any{"layout_id": "title", "content": []any{}}},
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
			NextToolCall: nextCallGetInputSchema(),
		})
	}

	// Unknown keys — warnings by default, errors when strict_unknown_keys=true.
	strictUnknownKeys, _ := request.GetArguments()["strict_unknown_keys"].(bool)
	boundaryDiags = append(boundaryDiags, unknownKeyDiags([]byte(jsonStr), strictUnknownKeys)...)

	// Enum validation — reject unknown values for transition, transition_speed, build, background.fit.
	if enumErrs := checkInputEnumValues(&input); len(enumErrs) > 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationErrors(enumErrs)...)
	}

	// Design mode constraints — reject raw hex colors and absolute sizes in constrained mode.
	if violations := validateDesignMode(&input); len(violations) > 0 {
		boundaryDiags = append(boundaryDiags, designModeDiagnostics(violations)...)
	}

	// No-emoji policy — reject emoji codepoints anywhere in user-supplied text.
	// Authors must use bundled SVG icons (see svggen/icons) or user-provided icons.
	if emojiViolations := emoji.ValidateNoEmojiInText(&input); len(emojiViolations) > 0 {
		boundaryDiags = append(boundaryDiags, noEmojiDiagnostics(emojiViolations)...)
	}

	// Fail fast if any boundary diagnostic is an error.
	if diagnostics.HasErrors(boundaryDiags) {
		return api.MCPDiagnosticsError(boundaryDiags), nil
	}

	// Text-fit checking via strict_fit parameter (default: warn).
	// In warn mode, findings are collected here and merged into the structured
	// fit_findings response below so MCP clients see them without having to
	// separately pass fit_report=true. In strict mode, refuse-class findings
	// abort generation with a diagnostics error.
	strictFit := "warn"
	if sf, err := request.RequireString("strict_fit"); err == nil && sf != "" {
		strictFit = sf
	}
	var strictFitFindings []patterns.FitFinding
	if strictFit != "off" {
		rawFindings, refuseErr := evaluateStrictFit(&input, strictFit)
		if refuseErr != nil {
			// Preserve historical stderr NDJSON dump on refuse for CLI/log parity.
			enc := json.NewEncoder(os.Stderr)
			for _, f := range rawFindings {
				_ = enc.Encode(f)
			}
			return api.MCPDiagnosticsError(diagnostics.FromJoinedError(refuseErr, "STRICT_FIT")), nil
		}
		for _, f := range rawFindings {
			strictFitFindings = append(strictFitFindings, convertTextFitFinding(f))
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
	synthesisFindings := template.SynthesizeIfNeeded(reader, analysis)
	templateLayouts := analysis.Layouts
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	templateMetadata, _ = template.ParseMetadata(reader)

	// Resolve canonical layout names (e.g. "title", "content", "blank") to
	// concrete layout IDs using tag-based matching against the target template.
	resolveCanonicalLayoutIDs(input.Slides, templateLayouts)

	// Resolve URL references (background.url, image_value.url, grid image.url,
	// icon.url, nested shape.icon.url). Mirrors the CLI generate path so MCP
	// and CLI agree on which URL fields are supported. Failures become
	// per-field URL_FETCH_FAILED diagnostics. The cache cleanup is deferred
	// for the rest of this handler — closing it earlier would invalidate the
	// local paths now embedded in the slides.
	urlFindings, urlCleanup, urlErr := mc.resolvePresentationURLs(input.Slides)
	defer urlCleanup()
	if urlErr != nil {
		return api.MCPSimpleError("URL_RESOLVER_INIT", fmt.Sprintf("resource resolver: %v", urlErr)), nil
	}
	if len(urlFindings) > 0 {
		return api.MCPDiagnosticsError(urlFindings), nil
	}

	// Resolve relative asset paths (icons, content images, grid images,
	// background images) against base_dir. MCP receives inline JSON, not a
	// file path, so callers pass base_dir to specify the directory relative
	// paths should resolve against. When omitted we fall back to the server
	// process CWD — historical behaviour that is not portable across MCP
	// launch configurations.
	//
	// All per-asset failures are collected into structured diagnostics so
	// the caller can fix each broken reference independently instead of
	// seeing one bag-of-strings error.
	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return baseDirErr, nil
	}
	assetFindings := resolveLocalAssetPaths(input.Slides, baseDir)
	if assetErrors := diagnostics.FilterBySeverity(assetFindings, diagnostics.SeverityError); len(assetErrors) > 0 {
		return api.MCPDiagnosticsError(assetErrors), nil
	}

	// Resolve deck-level rhythm grid when configured.
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		if err := validateGridConfig(input.Grid); err != nil {
			return api.MCPSimpleError("INVALID_GRID", fmt.Sprintf("grid: %v", err)), nil
		}
		rhythmGrid = resolveGrid(input.Grid, templateLayouts, slideWidth, slideHeight)
	}

	// Convert slides
	mcpDiagCtx := &GridDiagramContext{
		ThemeColors: theme.Colors,
		DataPalette: resolveDataPalette(templateMetadata, theme.Colors),
	}
	slideSpecs, gridDiagWarnings, gridVisualFindings, err := convertPresentationSlides(input.Slides, templateLayouts, slideWidth, slideHeight, templateMetadata, rhythmGrid, patterns.AccentStrategy(input.AccentStrategy), mcpDiagCtx, false)
	if err != nil {
		return api.MCPSimpleError("INVALID_SLIDE", fmt.Sprintf("invalid slide specification: %v", err)), nil
	}

	// Pre-validate chart/diagram data (unknown keys already caught at boundary).
	inputWarnings := validateSlidesChartData(input.Slides)
	inputWarnings = append(inputWarnings, gridDiagWarnings...)
	// Surface non-blocking asset findings (e.g. ICON_FILL_IGNORED_ON_INLINE)
	// as warnings so generation proceeds while the agent still sees them.
	for _, d := range assetFindings {
		if d.Severity != diagnostics.SeverityError {
			inputWarnings = append(inputWarnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
		}
	}
	chartDiagFindings := validateSlidesChartDiagnostics(input.Slides)

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
		DataPalette:           resolveDataPalette(templateMetadata, theme.Colors),
	}

	// Wire footer/chrome configuration.
	if input.Chrome != nil {
		genReq.Footer = chromeToFooterConfig(input.Chrome, len(slideSpecs))
		applyChromeSkip(slideSpecs, input.Chrome, input.Slides, templateLayouts)
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
		return api.MCPSimpleError("GENERATION_FAILED", fmt.Sprintf("generation failed: %v", err)), nil
	}

	duration := time.Since(startTime)

	// Post-generation output validation via output_validation parameter (default: strict).
	// Strict mode is the standing guarantee for the 'zero needs repair' contract
	// (see go-slide-creator-0myv): every successful generate_presentation response
	// implies a clean output-validation pass.
	outputValidation := "strict"
	if ov, ovErr := request.RequireString("output_validation"); ovErr == nil && ov != "" {
		outputValidation = ov
	}
	var outputValidationFindings []pptx.Finding
	if outputValidation != "off" {
		report, valErr := pptx.ValidateOutputFile(outputPath)
		if valErr != nil {
			return api.MCPSimpleError("OUTPUT_VALIDATION_ERROR", fmt.Sprintf("output validation failed: %v", valErr)), nil
		}
		outputValidationFindings = report.Findings

		// In strict mode, fail if any blocking findings exist.
		if outputValidation == "strict" && !report.IsValid() {
			return mcpOutputValidationError(report), nil
		}
	}

	// Merge input-layer warnings with generation warnings.
	allWarnings := append(inputWarnings, result.Warnings...)
	// Surface deprecation warnings for legacy field usage.
	allWarnings = append(allWarnings, deprecationWarnings(&input)...)
	// Surface boundary warnings (e.g. unknown keys) in the response.
	for _, d := range boundaryDiags {
		if d.Severity == diagnostics.SeverityWarning {
			allWarnings = append(allWarnings, d.Message)
		}
	}

	// Collect fit findings when requested.
	var fitFindings []patterns.FitFinding
	if fitReport, _ := request.GetArguments()["fit_report"].(bool); fitReport {
		fitFindings = collectFitFindings(&input, templateLayouts, slideWidth, slideHeight, &analysis.Theme)
	}

	// Append render-time fit findings from the generator (overflow, truncation, clamping).
	fitFindings = append(fitFindings, result.FitFindings...)

	// Append chart data diagnostics (coerced values, inferred shapes, empty data).
	fitFindings = append(fitFindings, chartDiagFindings...)

	// Append contrast auto-fix findings (always emitted, not gated by fit_report).
	fitFindings = append(fitFindings, contrastSwapsToFindings(result.ContrastSwaps)...)

	// Append grid visual findings (diagram narrow-cell, etc.) — always emitted.
	fitFindings = append(fitFindings, gridVisualFindings...)

	// Append strict_fit findings unconditionally so MCP clients see warn-mode
	// overflow diagnostics without needing to separately pass fit_report=true.
	// When fit_report=true also collected text-fit findings via
	// collectFitFindings, dedupFitFindings removes the duplicates by
	// (Code, Path, Action, Message).
	fitFindings = append(fitFindings, strictFitFindings...)
	fitFindings = dedupFitFindings(fitFindings)

	// Apply per-slide finding budget.
	verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
	fitFindings = BudgetFitFindings(fitFindings, DefaultFindingBudget, verboseFit)

	// Append synthesis findings (template-level, always emitted, not subject to
	// per-slide budget).
	fitFindings = append(fitFindings, synthesisFindings...)

	// Build per-slide resolution summary
	slideResolutions := buildSlideResolutions(input.Slides, slideSpecs, templateLayouts, syntheticFiles)

	// Build response
	output := JSONOutput{
		Success:                  true,
		OutputPath:               outputPath,
		SlideCount:               result.SlideCount,
		ContentHash:              result.ContentHash,
		DurationMs:               duration.Milliseconds(),
		Warnings:                 allWarnings,
		Quality:                  computeQualityScore(input.Slides, allWarnings),
		ValidationErrors:         result.ValidationErrors,
		FitFindings:              fitFindings,
		Slides:                   slideResolutions,
		OutputValidationFindings: outputValidationFindings,
	}

	mc.idempotency.Set("generate_presentation", idemKey, output)

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcpResult, nil
}

func (mc *mcpConfig) handleListTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Legacy `mode` parameter — accepted for backward compatibility.
	mode := "compact"
	modeExplicit := false
	if m, err := request.RequireString("mode"); err == nil && m != "" {
		switch m {
		case "list", "compact", "full":
			mode = m
			modeExplicit = true
		default:
			return mcpParseErrorWithFix(diagnostics.CodeUnknownEnum, "mode",
				fmt.Sprintf("invalid mode %q: must be list, compact, or full", m),
				&diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"allowed": []string{"list", "compact", "full"}}},
			), nil
		}
	}

	// New `fields` parameter — preferred over `mode`. When both are provided,
	// fields wins. The two surfaces use different vocabularies on purpose:
	//
	//   fields=compact -> mode="list" (slim: name + aspect_ratio + layout_count + table_styles)
	//   fields=full    -> mode="full" (every per-template detail incl. layouts)
	//
	// The legacy mode flag still reaches the intermediate "compact" detail level
	// (theme + layout_summaries, no placeholder geometry); agents that want it
	// must pin mode=compact explicitly.
	fieldsMode, fieldsExplicit, fErrField, fErrMsg := listFieldsParam(request)
	if fErrMsg != "" {
		return argInvalidValue("list_templates", "INVALID_PARAMETER", fErrField, fErrMsg, "string", "compact", nil), nil
	}
	if fieldsExplicit {
		switch fieldsMode {
		case listFieldsCompact:
			mode = "list"
		case listFieldsFull:
			mode = "full"
		}
	}

	templateName, _ := request.RequireString("template")
	filterStr := listFilterParam(request)

	// Read-only discovery: when set, skip layout-preview PNG generation so the
	// call writes no cache files. The side_effects block in the response reports
	// what happened either way.
	readOnly := false
	if v, err := request.RequireBool("read_only"); err == nil {
		readOnly = v
	}

	// Parse pagination parameters (cursor, page_size). Invalid input is
	// surfaced as a structured INVALID_PARAMETER error.
	offset, pageSize, errField, errMsg := paginationParams(request)
	if errMsg != "" {
		return argInvalidValue("list_templates", "INVALID_PARAMETER", errField, errMsg, "", nil, nil), nil
	}

	// Discover templates using the shared resolution path (flag → env → user
	// home → ./templates → embedded), so discovery succeeds in the same
	// environments where generation does — including embedded-only mode.
	var templateNames []string
	if templateName != "" {
		templateNames = []string{templateName}
	} else {
		templateNames = listAvailableTemplates(mc.templatesDir)
		sort.Strings(templateNames)
	}

	// Apply name filter before resolution so we don't pay for analyses we'll
	// discard. The single-template path (templateName != "") is exempt — the
	// caller already named the target explicitly.
	if filterStr != "" && templateName == "" {
		filtered := make([]string, 0, len(templateNames))
		for _, n := range templateNames {
			if strings.Contains(strings.ToLower(n), filterStr) {
				filtered = append(filtered, n)
			}
		}
		templateNames = filtered
	}

	type resolvedTemplate struct {
		name string
		path string
	}
	var resolved []resolvedTemplate
	for _, name := range templateNames {
		path, cleanup, err := resolveTemplatePath(name, mc.templatesDir)
		if err != nil {
			if templateName != "" {
				return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(name, mc.templatesDir)), nil
			}
			slog.Warn("failed to resolve template", "template", name, "error", err)
			continue
		}
		defer cleanup()
		resolved = append(resolved, resolvedTemplate{name: name, path: path})
	}

	totalCount := len(resolved)
	start, end, nextCursor := paginationSlice(totalCount, offset, pageSize)
	pagedTemplates := resolved[start:end]

	skillOpts := skillInfoOptions{NoPreview: readOnly}
	var templates []skillTemplateInfo
	for _, rt := range pagedTemplates {
		info, err := analyzeTemplateForSkillInfoOpts(rt.path, mc.cache, mode, skillOpts)
		if err != nil {
			slog.Error("failed to analyze template", "template", rt.name, "error", err)
			templates = append(templates, skillTemplateInfo{
				Name:  rt.name,
				Error: fmt.Sprintf("failed to analyze template: %v", err),
			})
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
		TotalCount:     totalCount,
		PageSize:       pageSize,
		NextCursor:     nextCursor,
		SideEffects:    buildSkillSideEffects(readOnly, "read_only=true"),
	}

	// Deprecation hint: nudge callers that did not pick a projection so a
	// future release can flip the default to compact without surprising them.
	// `mode` is the historical surface — callers who pinned it already made an
	// explicit choice and don't need the hint.
	if !fieldsExplicit && !modeExplicit {
		output.Warnings = append(output.Warnings, defaultFieldsDeprecation)
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
		mcp.WithDescription("Fetch capability metadata for all chart types: limits, density behavior, label strategy, and supported options per chart type. Note: list_templates already includes chart_capabilities in its supported_types response — prefer that single call for initial discovery."),
		mcp.WithRawOutputSchema(outputSchemaGetChartCapabilities),
	)
}

func mcpGetDiagramCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_diagram_capabilities",
		mcp.WithDescription("Fetch capability metadata for all diagram types: node limits, overflow behavior, required/optional fields per diagram type. Note: list_templates already includes diagram_capabilities in its supported_types response — prefer that single call for initial discovery."),
		mcp.WithRawOutputSchema(outputSchemaGetDiagramCapabilities),
		mcp.WithBoolean("include_experimental",
			mcp.Description("Include experimental/stub diagram types that are not yet fully functional. Default false."),
		),
	)
}

// chartCapabilitiesResponse is the JSON envelope for get_chart_capabilities.
type chartCapabilitiesResponse struct {
	ChartCapabilities []svggen.ChartCapability `json:"chart_capabilities"`
}

// diagramCapabilitiesResponse is the JSON envelope for get_diagram_capabilities.
type diagramCapabilitiesResponse struct {
	DiagramCapabilities []svggen.DiagramCapability `json:"diagram_capabilities"`
}

func handleGetChartCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := chartCapabilitiesResponse{
		ChartCapabilities: svggen.ChartCapabilities(),
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func handleGetDiagramCapabilities(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	caps := svggen.DiagramCapabilitiesReady()
	if v, ok := request.GetArguments()["include_experimental"]; ok {
		if inc, isBool := v.(bool); isBool && inc {
			caps = svggen.DiagramCapabilities()
		}
	}
	// Merge placement-aware metadata from the canonical registry.
	caps = generator.ApplyPlacementMetadata(caps)
	resp := diagramCapabilitiesResponse{
		DiagramCapabilities: caps,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func (mc *mcpConfig) handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("validate_input", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// Parse JSON input — reject trailing data.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nextCallGetInputSchema()), nil
	}

	// Apply deck-level defaults before validation.
	applyDefaults(&input)

	// Resolve named style references from template settings (priority 3).
	mc.resolveInputNamedSettings(&input)

	// Expand structure block into flat slides (mutually exclusive with
	// top-level slides). Mirrors the CLI path so MCP and CLI agree on the
	// effective slide list before slide-level diagnostics run.
	if structDiags := applyStructureExpansion(&input); len(structDiags) > 0 {
		return api.MCPDiagnosticsError(structDiags), nil
	}

	// Unknown keys — warnings by default, errors when strict_unknown_keys=true.
	strictUnknownKeys := false
	if v, err := request.RequireBool("strict_unknown_keys"); err == nil {
		strictUnknownKeys = v
	}
	var boundaryDiags []diagnostics.Diagnostic
	if ukWarns := checkInputUnknownKeys([]byte(jsonStr)); len(ukWarns) > 0 {
		if strictUnknownKeys {
			boundaryDiags = append(boundaryDiags, diagnostics.FromValidationErrors(ukWarns)...)
		} else {
			boundaryDiags = append(boundaryDiags, diagnostics.FromValidationWarnings(ukWarns)...)
		}
	}
	if enumErrs := checkInputEnumValues(&input); len(enumErrs) > 0 {
		boundaryDiags = append(boundaryDiags, diagnostics.FromValidationErrors(enumErrs)...)
	}

	// Design mode constraints.
	if violations := validateDesignMode(&input); len(violations) > 0 {
		boundaryDiags = append(boundaryDiags, designModeDiagnostics(violations)...)
	}

	// No-emoji policy — reject emoji codepoints anywhere in user-supplied text.
	if emojiViolations := emoji.ValidateNoEmojiInText(&input); len(emojiViolations) > 0 {
		boundaryDiags = append(boundaryDiags, noEmojiDiagnostics(emojiViolations)...)
	}

	// URL preflight: download URL-based assets so agents see broken URLs
	// alongside other asset findings in validate_input. Same surfaces as
	// handleGenerate (background.url, image_value.url, grid image.url,
	// icon.url, nested shape.icon.url). The downloaded cache is cleaned up
	// before this handler returns — validate does not need the bytes, only
	// confirmation that the URL is reachable.
	urlFindings, urlCleanup, urlErr := mc.resolvePresentationURLs(input.Slides)
	defer urlCleanup()
	if urlErr != nil {
		return mcpErrorWithNext("URL_RESOLVER_INIT", fmt.Sprintf("resource resolver: %v", urlErr), nextCallRetry("validate_input", "presentation")), nil
	}
	if len(urlFindings) > 0 {
		boundaryDiags = append(boundaryDiags, urlFindings...)
	}

	// Asset preflight: validate bundled icon names (catches typos and
	// missing "filled:" prefix), resolve icon.path fields, and resolve
	// content image_value, shape_grid cell image, and slide background
	// image paths against base_dir. Mirrors the handleGenerate preflight so
	// agents catch broken references in validate_input instead of burning a
	// generate call. base_dir resolution failures short-circuit before
	// per-asset findings — the agent can't fix individual asset paths until
	// the base directory is correct.
	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return baseDirErr, nil
	}
	if assetFindings := resolveLocalAssetPaths(input.Slides, baseDir); len(assetFindings) > 0 {
		boundaryDiags = append(boundaryDiags, assetFindings...)
	}

	output := dryRunOutput{
		Valid:       !diagnostics.HasErrors(boundaryDiags),
		Diagnostics: boundaryDiags,
		Slides:      []dryRunSlide{},
		subcommand:  "validate_input",
		template:    input.Template,
		inputSHA256: diagnostics.ComputeInputSHA256([]byte(jsonStr)),
	}

	// Validate required fields
	if input.Template == "" {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required",
			Severity:     diagnostics.SeverityError,
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
			NextToolCall: nextCallListTemplates(),
		})
	}
	if len(input.Slides) == 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity:     diagnostics.SeverityError,
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
			NextToolCall: nextCallGetInputSchema(),
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
			Severity:     diagnostics.SeverityError,
			NextToolCall: nextCallListTemplates(),
		})
		return marshalValidateResult(ctx, output)
	}
	defer templateCleanup()

	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, mc.cache)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "TEMPLATE_ERROR", Path: "template", Message: fmt.Sprintf("template analysis failed: %v", err),
			Severity:     diagnostics.SeverityError,
			NextToolCall: nextCallListTemplates(),
		})
		return marshalValidateResult(ctx, output)
	}

	// Resolve canonical layout names before validation so agents can use
	// stable aliases like "title", "content", "blank".
	resolveCanonicalLayoutIDs(input.Slides, templateAnalysis.Layouts)

	// Validate slides against template (layout IDs, placeholder IDs,
	// character limits, content types, chart/diagram data)
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)

	// Fit report: run all fit detectors (default true for validate).
	fitReport := true
	if v, ok := request.GetArguments()["fit_report"].(bool); ok {
		fitReport = v
	}
	if fitReport {
		findings := collectFitFindings(&input, templateAnalysis.Layouts, templateAnalysis.SlideWidth, templateAnalysis.SlideHeight, &templateAnalysis.Theme)
		verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
		output.FitFindings = BudgetFitFindings(findings, DefaultFindingBudget, verboseFit)
	}

	return marshalValidateResult(ctx, output)
}

// marshalValidateResult serializes a dryRunOutput as a CallToolResult.
// When validation fails (any error-severity diagnostic), it returns IsError=true
// with the same FindingEnvelope error shape that generate_presentation uses. When
// validation passes, it returns a success envelope whose warnings/info findings
// travel in the single Findings envelope (see docs/AGENT_DIAGNOSTICS.md).
func marshalValidateResult(ctx context.Context, output dryRunOutput) (*mcp.CallToolResult, error) {
	if !output.Valid {
		// Return the same error envelope shape as generate_presentation.
		return api.MCPDiagnosticsError(output.Diagnostics), nil
	}
	// Success path: fold the accumulated diagnostics and fit findings into the
	// single Findings envelope just before serialization.
	output.buildFindingsEnvelope()
	if err := api.ComputeResponseFingerprint(&output); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("validate_input", "presentation")), nil
	}
	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("validate_input", "presentation")), nil
	}
	return mcpResult, nil
}

// --- Pattern tool definitions ---

func mcpRecommendPatternTool() mcp.Tool {
	return mcp.NewTool("recommend_pattern",
		mcp.WithDescription("Recommend named patterns for a content intent. Returns up to 3 ranked candidates with scores, rationales, confidence bands, and expansion previews. When prefer_variety is true and recent_patterns is provided, previously-used patterns are penalized and a diversity bonus candidate may be injected. When candidates is supplied, scores ONLY those pattern names against intent/hints and returns all of them ranked (no threshold cutoff, no truncation, no near-misses)."),
		mcp.WithRawOutputSchema(outputSchemaRecommendPattern),
		mcp.WithString("intent",
			mcp.Required(),
			mcp.Description("Natural-language description of what the slide should show (e.g., \"show 3 KPIs\", \"compare two options\", \"business model canvas\", \"project roadmap\")."),
		),
		mcp.WithObject("content_hints",
			mcp.Description("Optional structured hints to refine ranking. Properties: item_count (int), has_chart (bool), has_metrics (bool), columns (int)."),
		),
		mcp.WithArray("recent_patterns",
			mcp.Description("Pattern names used on preceding slides in this deck, in order. Used with prefer_variety to penalize repeated patterns."),
		),
		mcp.WithBoolean("prefer_variety",
			mcp.Description("When true, apply recency decay penalty to patterns in recent_patterns and inject a diversity bonus candidate."),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("0-based index of the slide being built. Provides context for diversity scoring."),
		),
		mcp.WithArray("candidates",
			mcp.Description("Explicit shortlist of pattern names to rank against intent/hints. When supplied, ALL listed names are scored and returned ranked (no threshold cutoff, no truncation, no near-misses, no diversity bonus). Names not in the catalog still appear with score 0 and a rationale noting the miss."),
		),
	)
}

func mcpListPatternsTool() mcp.Tool {
	return mcp.NewTool("list_patterns",
		mcp.WithDescription(`List all available named patterns. Patterns are high-level primitives that expand to shape_grid definitions, replacing ~600 tokens of boilerplate with ~100 tokens.

Pagination: response is an object {groups, total_count, page_size, next_cursor?}. Patterns are flattened across categories for paging; categories are rebuilt for each page in the canonical order. Use cursor + page_size to iterate.

Projection (token-economy): pass fields="compact" to receive only {name, category, cells, use_when, supports_callout} per pattern. Pass fields="full" for the legacy taxonomy payload (narrative_role, pairs_with, composes_with, role_on_slide, density_class, accent_weight, estimated_prompt_size_bytes). Omitting fields emits a deprecation hint in warnings[] — future releases will switch the default to compact.

Filtering: pass filter="<substring>" to limit the response to patterns whose name contains the substring (case-insensitive). Applied before pagination.`),
		mcp.WithRawOutputSchema(outputSchemaListPatterns),
		mcp.WithString("fields",
			mcp.Description("Field projection: compact (slim — name, category, cells, use_when, supports_callout) or full (legacy taxonomy detail). When omitted, behavior matches full and a deprecation hint is returned in warnings[]."),
			mcp.Enum(listFieldsCompact, listFieldsFull),
		),
		mcp.WithString("filter",
			mcp.Description("Case-insensitive substring filter on pattern name. Applied before pagination."),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque continuation token from a previous response's next_cursor. Omit or pass empty for the first page."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Maximum number of pattern entries to return (across all categories). Default: 50. Clamped to [1, 200]."),
		),
	)
}

func mcpShowPatternTool() mcp.Tool {
	return mcp.NewTool("show_pattern",
		mcp.WithDescription("Show full details for a named pattern, including its authoritative JSON Schema for values, overrides, and cell_overrides."),
		mcp.WithRawOutputSchema(outputSchemaShowPattern),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name (e.g., kpi-3up, bmc-canvas, card-grid)."),
		),
	)
}

func mcpValidatePatternTool() mcp.Tool {
	return mcp.NewTool("validate_pattern",
		mcp.WithDescription("Validate pattern inputs without expanding. Returns structured errors on failure."),
		mcp.WithRawOutputSchema(outputSchemaValidatePattern),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name to validate against."),
		),
		mcp.WithObject("values",
			mcp.Required(),
			mcp.Description("Pattern values. Use show_pattern to see the schema for each pattern."),
		),
		mcp.WithObject("overrides",
			mcp.Description("Pattern-level overrides (optional). Use show_pattern to see supported override fields."),
		),
		mcp.WithObject("cell_overrides",
			mcp.Description("Per-cell overrides keyed by cell index (optional). Example: {\"0\":{\"fill\":\"#FF0000\"}}"),
		),
		mcp.WithObject("callout",
			mcp.Description("Callout band (optional). Only supported by some patterns (card-grid, comparison-2col). Example: {\"text\":\"Key takeaway\",\"emphasis\":\"bold\"}"),
		),
	)
}

func mcpExpandPatternTool() mcp.Tool {
	return mcp.NewTool("expand_pattern",
		mcp.WithDescription("Expand a named pattern into its full shape_grid definition. Useful for debugging and previewing what a pattern call produces. Returns density_warnings if any embedded tables exceed density thresholds."),
		mcp.WithRawOutputSchema(outputSchemaExpandPattern),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Pattern name to expand."),
		),
		mcp.WithObject("values",
			mcp.Required(),
			mcp.Description("Pattern values. Use show_pattern to see the schema for each pattern."),
		),
		mcp.WithObject("overrides",
			mcp.Description("Pattern-level overrides (optional). Use show_pattern to see supported override fields."),
		),
		mcp.WithObject("cell_overrides",
			mcp.Description("Per-cell overrides keyed by cell index (optional)."),
		),
		mcp.WithString("theme_template",
			mcp.Description("Template name to use for theme context during expansion. If omitted, a minimal synthesized theme is used."),
		),
		mcp.WithObject("bounds",
			mcp.Description("Explicit bounding rectangle (percentages of slide dimensions: x, y, width, height). Constrains the grid to a sub-region, fixing density math for patterns that don't fill the full content area."),
		),
		mcp.WithNumber("max_height_pct",
			mcp.Description("Convenience alias: constrains grid height to this percentage of the content area (1-99). Equivalent to bounds:{x:0,y:0,width:100,height:<value>}."),
		),
	)
}

// --- Pattern tool handlers ---

// patternValidationError is a D10 structured error for pattern validation.
type patternValidationError struct {
	Field        string                       `json:"field"`
	Code         string                       `json:"code,omitempty"`
	Message      string                       `json:"message"`
	Fix          *patterns.FixSuggestion      `json:"fix,omitempty"`
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
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

// unmarshalValidationErrorResult checks if a json.Unmarshal error contains
// structured *ValidationError(s) from custom UnmarshalJSON methods. If so,
// it returns a structured validation failure result; otherwise returns nil.
func unmarshalValidationErrorResult(ctx context.Context, err error, patternName string) *mcp.CallToolResult {
	errs := splitValidationErrors(err)
	hasStructured := false
	for _, e := range errs {
		if e.Fix != nil {
			hasStructured = true
			break
		}
	}
	if !hasStructured {
		return nil
	}
	attachNextToolCallsToValidationErrors(errs, patternName)
	result := struct {
		OK     bool                     `json:"ok"`
		Errors []patternValidationError `json:"errors"`
	}{OK: false, Errors: errs}
	mcpResult, _ := api.MCPSuccessResult(ctx, result)
	return mcpResult
}

// attachNextToolCallsToValidationErrors populates NextToolCall on each
// patternValidationError that has a Fix with a recognized kind. Unlike
// AttachNextToolCalls (which operates on FitFindings with a known slide index),
// validation errors occur before slide placement, so repair_slide suggestions
// use a placeholder slide_index of -1 that the agent must replace.
func attachNextToolCallsToValidationErrors(errs []patternValidationError, patternName string) {
	for i := range errs {
		e := &errs[i]
		if e.Fix == nil || e.NextToolCall != nil {
			continue
		}
		switch e.Fix.Kind {
		case "swap_pattern":
			itemCount := 0
			e.NextToolCall = patterns.RecommendToolCall(itemCount)
		case "adopt_pattern":
			itemCount := 0
			if n, ok := e.Fix.Params["filled_slots"].(int); ok {
				itemCount = n
			}
			e.NextToolCall = patterns.RecommendToolCall(itemCount)
		default:
			tc := patterns.RepairToolCall(-1, e.Fix)
			if tc != nil {
				// Add pattern name for context — agent needs it for repair_slide.
				tc.ArgsTemplate["pattern"] = patternName
			}
			e.NextToolCall = tc
		}
	}
}

// attachBoundsHintToCapacityWarnings adds next_tool_call to underfilled capacity
// warnings, suggesting re-expansion with a recommended max_height_pct. This
// eliminates false-positive underfill warnings for patterns that genuinely have
// short content by guiding agents to constrain grid height.
func attachBoundsHintToCapacityWarnings(warnings []cellDensityWarning, patternName string, pi *PatternInput) {
	// Only suggest bounds when no explicit bounds were already provided
	if pi.Bounds != nil || pi.MaxHeightPct > 0 {
		return
	}
	for i := range warnings {
		if warnings[i].Status != "underfilled" {
			continue
		}
		// Recommend max_height_pct based on actual/budget ratio
		ratio := float64(warnings[i].Actual) / float64(warnings[i].Budget)
		if ratio <= 0 || ratio >= 0.6 {
			continue
		}
		// Suggest a height that would make content fill ~70% of the cell
		suggestedPct := int(ratio / 0.7 * 100)
		if suggestedPct < 20 {
			suggestedPct = 20
		}
		if suggestedPct > 90 {
			suggestedPct = 90
		}
		warnings[i].NextToolCall = &patterns.ToolCallSuggestion{
			Tool: "expand_pattern",
			ArgsTemplate: map[string]any{
				"name":           patternName,
				"max_height_pct": suggestedPct,
			},
		}
	}
}

func (mc *mcpConfig) handleRecommendPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	intent, err := request.RequireString("intent")
	if err != nil {
		return argMissing("recommend_pattern", "intent", "string", "compare two options side-by-side", nil), nil
	}

	// Parse optional content_hints.
	var hints patterns.ContentHints
	if hintsRaw, ok := request.GetArguments()["content_hints"]; ok && hintsRaw != nil {
		hintsJSON, err := json.Marshal(hintsRaw)
		if err == nil {
			_ = json.Unmarshal(hintsJSON, &hints)
		}
	}

	// Parse optional variety/diversity parameters.
	var opts patterns.RecommendOptions
	if rpRaw, ok := request.GetArguments()["recent_patterns"]; ok && rpRaw != nil {
		rpJSON, err := json.Marshal(rpRaw)
		if err == nil {
			_ = json.Unmarshal(rpJSON, &opts.RecentPatterns)
		}
	}
	if pv, ok := request.GetArguments()["prefer_variety"]; ok {
		if b, ok := pv.(bool); ok {
			opts.PreferVariety = b
		}
	}
	if si, ok := request.GetArguments()["slide_index"]; ok {
		if f, ok := si.(float64); ok {
			opts.SlideIndex = int(f)
		}
	}
	if candsRaw, ok := request.GetArguments()["candidates"]; ok && candsRaw != nil {
		candsJSON, err := json.Marshal(candsRaw)
		if err == nil {
			_ = json.Unmarshal(candsJSON, &opts.Candidates)
		}
	}

	reg := patterns.Default()
	// In candidates mode, rank every supplied name regardless of how many — the
	// agent asked for these specifically, so we don't truncate.
	maxCands := 3
	if len(opts.Candidates) > 0 {
		maxCands = len(opts.Candidates)
	}
	rec := patterns.Recommend(reg, intent, &hints, maxCands, &opts)

	// Build expansion previews for each candidate using exemplar values.
	expandCtx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}

	type nearMissResult struct {
		PatternName string  `json:"pattern_name"`
		Score       float64 `json:"score"`
		WouldTipIf  string  `json:"would_tip_if"`
	}

	type candidateResult struct {
		PatternName      string                     `json:"pattern_name"`
		Score            float64                    `json:"score"`
		Rationale        string                     `json:"rationale"`
		ConfidenceBand   string                     `json:"confidence_band"`
		DiversityBonus   bool                       `json:"diversity_bonus,omitempty"`
		ExpansionPreview *jsonschema.ShapeGridInput `json:"expansion_preview,omitempty"`
		PreviewPNGPaths  []string                   `json:"preview_png_paths,omitempty"`
	}

	candidates := make([]candidateResult, len(rec.Candidates))
	for i, c := range rec.Candidates {
		candidates[i] = candidateResult{
			PatternName:    c.PatternName,
			Score:          c.Score,
			Rationale:      c.Rationale,
			ConfidenceBand: c.ConfidenceBand,
			DiversityBonus: c.DiversityBonus,
		}

		// Try expanding with exemplar values for a preview.
		pat, ok := reg.Get(c.PatternName)
		if !ok {
			continue
		}
		exemplar, ok := pat.(patterns.Exemplar)
		if !ok {
			continue
		}
		grid, err := pat.Expand(expandCtx, exemplar.ExemplarValues(), nil, nil)
		if err == nil {
			candidates[i].ExpansionPreview = grid
		}

		// Look up pre-generated preview PNGs from assets directory
		candidates[i].PreviewPNGPaths = findPatternPreviewPNGs(mc.templatesDir, c.PatternName)
	}

	nearMisses := make([]nearMissResult, len(rec.NearMisses))
	for i, nm := range rec.NearMisses {
		nearMisses[i] = nearMissResult{
			PatternName: nm.PatternName,
			Score:       nm.Score,
			WouldTipIf:  nm.WouldTipIf,
		}
	}

	type resultType struct {
		Candidates              []candidateResult `json:"candidates"`
		QueryUnderstood         string            `json:"query_understood_as"`
		Suggestion              string            `json:"suggestion,omitempty"`
		NearMisses              []nearMissResult  `json:"near_misses,omitempty"`
		DisambiguatingQuestions []string          `json:"disambiguating_questions,omitempty"`
	}

	result := resultType{
		Candidates:              candidates,
		QueryUnderstood:         rec.QueryUnderstood,
		NearMisses:              nearMisses,
		DisambiguatingQuestions: rec.DisambiguatingQuestions,
	}

	// When no candidates match, add a suggestion.
	if len(candidates) == 0 {
		result.Suggestion = "No patterns matched this intent. Consider using shape_grid directly to build a custom layout, or try rephrasing with keywords like: kpi, compare, timeline, matrix, bmc, icon, card."
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("recommend_pattern", "intent")), nil
	}
	return mcpResult, nil
}

func mcpRecommendVisualTool() mcp.Tool {
	return mcp.NewTool("recommend_visual",
		mcp.WithDescription("Unified visual recommender: ranks candidates across placeholder layouts, named patterns, charts, diagrams, and raw shape_grid. Replaces guesswork — ask this tool first, then use the winning category's tool to build the slide. When candidates is supplied, scores ONLY those names against intent/hints and returns all of them ranked (no threshold cutoff, no truncation); category is auto-resolved from the catalog and unknown names appear with score 0."),
		mcp.WithRawOutputSchema(outputSchemaRecommendVisual),
		mcp.WithString("intent",
			mcp.Required(),
			mcp.Description("Natural-language description of what the slide should show (e.g., \"show Q3 revenue trend\", \"compare 3 vendors on 5 dimensions\", \"agenda slide with 4 sections\")."),
		),
		mcp.WithObject("content_hints",
			mcp.Description("Optional structured hints to refine ranking. Properties: item_count (int), has_chart (bool), has_metrics (bool), columns (int), data_points (int), series_count (int), audience (string)."),
		),
		mcp.WithArray("recent_patterns",
			mcp.Description("Pattern names used on preceding slides in this deck, in order. Used with prefer_variety to penalize repeated patterns."),
		),
		mcp.WithBoolean("prefer_variety",
			mcp.Description("When true, apply recency decay penalty to patterns in recent_patterns."),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("0-based index of the slide being built."),
		),
		mcp.WithArray("candidates",
			mcp.Description("Explicit shortlist of candidate names to rank across all visual categories (placeholder layouts, named patterns, chart types, diagram types, or raw_shape_grid). When supplied, ALL listed names are scored and returned ranked (no threshold cutoff, no truncation). Category is auto-resolved from the catalog; unknown names appear with score 0 and a rationale noting the miss."),
		),
		mcp.WithString("template",
			mcp.Description("Optional template name (e.g., midnight-blue) to make recommendations template-aware. When supplied, each candidate carries a template_support object {status: supported|risky|unsupported, reasons[], required_layout} grounded in the template's canonical layouts, derivable layouts, font-aware placeholder capacities, and palette; candidates needing absent layouts or violating capacity are demoted so they no longer rank first. Use list_templates to discover names."),
		),
	)
}

// parseRecommendVisualArgs extracts the optional content_hints and
// variety/candidate options from a recommend_visual request. Malformed optional
// fields are silently ignored (best-effort parse), matching the tool contract.
func parseRecommendVisualArgs(request mcp.CallToolRequest) (patterns.VisualHints, patterns.RecommendOptions) {
	var hints patterns.VisualHints
	if hintsRaw, ok := request.GetArguments()["content_hints"]; ok && hintsRaw != nil {
		if hintsJSON, err := json.Marshal(hintsRaw); err == nil {
			_ = json.Unmarshal(hintsJSON, &hints)
		}
	}

	var opts patterns.RecommendOptions
	if rpRaw, ok := request.GetArguments()["recent_patterns"]; ok && rpRaw != nil {
		if rpJSON, err := json.Marshal(rpRaw); err == nil {
			_ = json.Unmarshal(rpJSON, &opts.RecentPatterns)
		}
	}
	if pv, ok := request.GetArguments()["prefer_variety"]; ok {
		if b, ok := pv.(bool); ok {
			opts.PreferVariety = b
		}
	}
	if si, ok := request.GetArguments()["slide_index"]; ok {
		if f, ok := si.(float64); ok {
			opts.SlideIndex = int(f)
		}
	}
	if candsRaw, ok := request.GetArguments()["candidates"]; ok && candsRaw != nil {
		if candsJSON, err := json.Marshal(candsRaw); err == nil {
			_ = json.Unmarshal(candsJSON, &opts.Candidates)
		}
	}
	return hints, opts
}

// resolveTemplateAnalysis loads the optional `template` argument into a parsed
// TemplateAnalysis. It returns (nil, nil) when no template was supplied, or
// (nil, errResult) when the named template cannot be resolved or analysed. The
// returned cleanup must be called by the caller when non-nil.
func (mc *mcpConfig) resolveTemplateAnalysis(request mcp.CallToolRequest) (*types.TemplateAnalysis, func(), *mcp.CallToolResult) {
	templateName := ""
	if t, ok := request.GetArguments()["template"]; ok {
		if s, ok := t.(string); ok {
			templateName = s
		}
	}
	if templateName == "" {
		return nil, nil, nil
	}
	templatePath, cleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return nil, nil, api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir))
	}
	analysis, err := getOrAnalyzeTemplate(templatePath, mc.cache)
	if err != nil {
		cleanup()
		return nil, nil, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("failed to analyze template %q: %v", templateName, err), nextCallListTemplates())
	}
	return analysis, cleanup, nil
}

func (mc *mcpConfig) handleRecommendVisual(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	intent, err := request.RequireString("intent")
	if err != nil {
		return argMissing("recommend_visual", "intent", "string", "show revenue growth over four quarters", nil), nil
	}

	hints, opts := parseRecommendVisualArgs(request)

	// Optional template context — when supplied, recommendations become
	// template-aware (per-candidate support + demotion of unsupported visuals).
	analysis, cleanup, errResult := mc.resolveTemplateAnalysis(request)
	if errResult != nil {
		return errResult, nil
	}
	if cleanup != nil {
		defer cleanup()
	}

	reg := patterns.Default()
	// In candidates mode, rank every supplied name — never truncate.
	maxCands := 5
	if len(opts.Candidates) > 0 {
		maxCands = len(opts.Candidates)
	}
	// With template context (non-candidates mode), fetch a wider set so that
	// support-aware demotion can promote a template-supported candidate into the
	// visible top-N instead of leaving an unsupported one at the top.
	fetchN := maxCands
	if analysis != nil && len(opts.Candidates) == 0 {
		fetchN = maxCands * 3
	}
	rec := patterns.RecommendVisual(reg, intent, &hints, fetchN, &opts)

	// Enrich candidates with placement guidance from capability truth.
	generator.EnrichVisualPlacement(&rec)

	// Annotate template support and demote candidates the template cannot host.
	if analysis != nil {
		generator.AnnotateTemplateSupport(&rec, analysis, &hints, reg)
		generator.ReorderByTemplateSupport(&rec)
		if len(opts.Candidates) == 0 && len(rec.Candidates) > maxCands {
			rec.Candidates = rec.Candidates[:maxCands]
		}
	}

	if err := api.ComputeResponseFingerprint(&rec); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("recommend_visual", "intent")), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, rec)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("recommend_visual", "intent")), nil
	}
	return mcpResult, nil
}

// patternCategoryGroup is a category-keyed group of patterns for list_patterns.
type patternCategoryGroup struct {
	Category string                `json:"category"`
	Patterns []skillPatternCompact `json:"patterns"`
}

// listPatternsResponse is the paginated envelope for list_patterns. Groups are
// rebuilt per page in the canonical category order; only categories with at
// least one pattern in the current page appear.
type listPatternsResponse struct {
	Groups     []patternCategoryGroup `json:"groups"`
	TotalCount int                    `json:"total_count"`
	PageSize   int                    `json:"page_size"`
	NextCursor string                 `json:"next_cursor,omitempty"`
	// Warnings carries advisory hints for the call — currently used to surface
	// the deprecation notice when the caller did not pass `fields`.
	Warnings []string `json:"warnings,omitempty"`
}

func handleListPatterns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	offset, pageSize, errField, errMsg := paginationParams(request)
	if errMsg != "" {
		return mcpParseError("INVALID_PARAMETER", errField, errMsg), nil
	}

	fieldsMode, fieldsExplicit, fErrField, fErrMsg := listFieldsParam(request)
	if fErrMsg != "" {
		return mcpParseError("INVALID_PARAMETER", fErrField, fErrMsg), nil
	}
	if !fieldsExplicit {
		// Legacy behavior — full payload. A deprecation hint is emitted below.
		fieldsMode = listFieldsFull
	}

	filterStr := listFilterParam(request)

	reg := patterns.Default()
	all := reg.List()

	// Build entries with taxonomy. In compact mode, leave optional fields
	// zero/empty so they drop from the wire via omitempty.
	entries := make([]skillPatternCompact, 0, len(all))
	for _, p := range all {
		name := p.Name()
		if filterStr != "" && !strings.Contains(strings.ToLower(name), filterStr) {
			continue
		}
		tax := p.Taxonomy()
		supportsCallout := false
		if cs, ok := p.(patterns.CalloutSupport); ok {
			supportsCallout = cs.SupportsCallout()
		}
		entry := skillPatternCompact{
			Name:            name,
			Category:        tax.Category,
			Cells:           p.CellsHint(),
			UseWhen:         p.UseWhen(),
			SupportsCallout: supportsCallout,
		}
		if fieldsMode == listFieldsFull {
			entry.NotWhen = p.NotWhen()
			entry.NarrativeRole = tax.NarrativeRole
			entry.PairsWith = tax.PairsWith
			entry.ComposesWith = tax.ComposesWith
			entry.RoleOnSlide = tax.RoleOnSlide
			entry.DensityClass = tax.DensityClass
			entry.AccentWeight = tax.AccentWeight
		}
		entries = append(entries, entry)
	}

	// Reorder entries so that category-grouped pagination produces stable,
	// canonical-ordered pages: flatten by category in canonical order.
	categoryOrder := []string{"data-display", "narrative", "structural", "hero"}
	grouped := make(map[string][]skillPatternCompact, len(categoryOrder))
	for _, e := range entries {
		grouped[e.Category] = append(grouped[e.Category], e)
	}
	ordered := make([]skillPatternCompact, 0, len(entries))
	for _, cat := range categoryOrder {
		ordered = append(ordered, grouped[cat]...)
	}

	totalCount := len(ordered)
	start, end, nextCursor := paginationSlice(totalCount, offset, pageSize)
	page := ordered[start:end]

	// Re-group the page slice into categories (preserving canonical order).
	pageGrouped := make(map[string][]skillPatternCompact, len(categoryOrder))
	for _, e := range page {
		pageGrouped[e.Category] = append(pageGrouped[e.Category], e)
	}
	groups := make([]patternCategoryGroup, 0, len(categoryOrder))
	for _, cat := range categoryOrder {
		if pats, ok := pageGrouped[cat]; ok {
			groups = append(groups, patternCategoryGroup{Category: cat, Patterns: pats})
		}
	}

	resp := listPatternsResponse{
		Groups:     groups,
		TotalCount: totalCount,
		PageSize:   pageSize,
		NextCursor: nextCursor,
	}
	if !fieldsExplicit {
		resp.Warnings = append(resp.Warnings, defaultFieldsDeprecation)
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func handleShowPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return argMissing("show_pattern", "name", "string", "kpi-3up", nextCallListPatterns()), nil
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

	tax := pat.Taxonomy()
	result := skillPatternFull{
		Name:            pat.Name(),
		Description:     pat.Description(),
		Cells:           "",
		UseWhen:         pat.UseWhen(),
		NotWhen:         pat.NotWhen(),
		Version:         pat.Version(),
		Schema:          schemaJSON,
		TextBudgetGuide: computeTextBudgetGuide(pat),
		ComposesWith:    tax.ComposesWith,
		RoleOnSlide:     tax.RoleOnSlide,
	}
	result.Cells = pat.CellsHint()

	if cs, ok := pat.(patterns.CalloutSupport); ok {
		result.SupportsCallout = cs.SupportsCallout()
		if cs.SupportsCallout() {
			result.CalloutSchema = patternCalloutSchemaJSON()
		}
	}

	if ex, ok := pat.(patterns.Exemplar); ok {
		result.ExampleValues = ex.ExemplarValues()
	}

	result.RenderingCapabilities = patternRenderingCapabilities(pat.Name())

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// patternRenderingCapabilities returns rendering capability metadata for a pattern.
func patternRenderingCapabilities(name string) *renderingCapabilities {
	switch name {
	case "icon-row":
		return &renderingCapabilities{
			IconSupport: "svg_only",
		}
	case "kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up", "kpi-inline":
		return &renderingCapabilities{IconSupport: "svg_only"}
	case "card-grid":
		return &renderingCapabilities{IconSupport: "svg_and_text"}
	case "matrix-2x2":
		return &renderingCapabilities{IconSupport: "svg_only"}
	case "hero-detail":
		return &renderingCapabilities{IconSupport: "svg_and_text"}
	default:
		return &renderingCapabilities{IconSupport: "none"}
	}
}

func handleValidatePattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return argMissing("validate_pattern", "name", "string", "kpi-3up", nextCallListPatterns()), nil
	}
	valuesStr, paramErr := objectParamAsJSON(request, "values")
	if paramErr != nil {
		return paramErr, nil
	}
	if valuesStr == "" {
		return argMissing("validate_pattern", "values", "object", map[string]any{
			"items": []any{map[string]any{"label": "Revenue", "value": "$1.2M"}},
		}, nil), nil
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
		if result := unmarshalValidationErrorResult(ctx, err, name); result != nil {
			return result, nil
		}
		return argInvalidJSON("values", fmt.Sprintf("invalid values JSON: %v", err), "object", nil, nil), nil
	}

	// Unmarshal overrides
	var overrides any
	overridesStr, paramErr2 := objectParamAsJSON(request, "overrides")
	if paramErr2 != nil {
		return paramErr2, nil
	}
	if overridesStr != "" {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal([]byte(overridesStr), overrides); err != nil {
				return argInvalidJSON("overrides", fmt.Sprintf("invalid overrides JSON: %v", err), "object", nil, nil), nil
			}
		}
	}

	// Unmarshal cell_overrides
	coStr, paramErr3 := objectParamAsJSON(request, "cell_overrides")
	if paramErr3 != nil {
		return paramErr3, nil
	}
	var cellOverrides map[int]any
	if coStr != "" {
		var rawCO map[string]json.RawMessage
		if err := json.Unmarshal([]byte(coStr), &rawCO); err != nil {
			return argInvalidJSON("cell_overrides", fmt.Sprintf("invalid cell_overrides JSON: %v", err), "object", nil, nil), nil
		}
		cellOverrides = make(map[int]any, len(rawCO))
		for key, raw := range rawCO {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return argInvalidValue("validate_pattern", "INVALID_KEY", fmt.Sprintf("cell_overrides.%s", key), fmt.Sprintf("cell_overrides key %q is not an integer", key), "integer", 0, nil), nil
			}
			co := pat.NewCellOverride()
			if co == nil {
				return api.MCPSimpleError("UNSUPPORTED", fmt.Sprintf("pattern %q does not support cell_overrides", name)), nil
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return argInvalidJSON(fmt.Sprintf("cell_overrides[%d]", idx), fmt.Sprintf("invalid cell_overrides[%d]: %v", idx, err), "object", nil, nil), nil
			}
			cellOverrides[idx] = co
		}
	}

	// Validate
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		// Return D10 structured errors — split joined errors into individual entries.
		errs := splitValidationErrors(err)
		attachNextToolCallsToValidationErrors(errs, name)
		result := struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}{OK: false, Errors: errs}

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
	calloutStr, paramErr := objectParamAsJSON(request, "callout")
	if paramErr != nil {
		return paramErr
	}
	if calloutStr == "" {
		return nil
	}
	var callout patterns.PatternCallout
	if err := json.Unmarshal([]byte(calloutStr), &callout); err != nil {
		return argInvalidJSON("callout", fmt.Sprintf("invalid callout JSON: %v", err), "object", nil, nil)
	}
	cs, ok := pat.(patterns.CalloutSupport)
	if ok && cs.SupportsCallout() {
		return nil
	}
	reg := patterns.Default()
	veErr := patterns.ErrCalloutUnsupportedFor(name, reg.CalloutSupportedPatterns())
	errs := splitValidationErrors(veErr)
	attachNextToolCallsToValidationErrors(errs, name)
	result := struct {
		OK     bool                     `json:"ok"`
		Errors []patternValidationError `json:"errors"`
	}{OK: false, Errors: errs}
	mcpResult, _ := api.MCPSuccessResult(ctx, result)
	return mcpResult
}

func (mc *mcpConfig) handleExpandPattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return argMissing("expand_pattern", "name", "string", "kpi-3up", nextCallListPatterns()), nil
	}
	valuesStr, paramErr := objectParamAsJSON(request, "values")
	if paramErr != nil {
		return paramErr, nil
	}
	if valuesStr == "" {
		return argMissing("expand_pattern", "values", "object", map[string]any{
			"items": []any{map[string]any{"label": "Revenue", "value": "$1.2M"}},
		}, nil), nil
	}

	reg := patterns.Default()
	if _, ok := reg.Get(name); !ok {
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
	overridesStr, paramErr2 := objectParamAsJSON(request, "overrides")
	if paramErr2 != nil {
		return paramErr2, nil
	}
	if overridesStr != "" {
		pi.Overrides = json.RawMessage(overridesStr)
	}
	coStr, paramErr3 := objectParamAsJSON(request, "cell_overrides")
	if paramErr3 != nil {
		return paramErr3, nil
	}
	if coStr != "" {
		var rawCO map[string]json.RawMessage
		if err := json.Unmarshal([]byte(coStr), &rawCO); err != nil {
			return argInvalidJSON("cell_overrides", fmt.Sprintf("invalid cell_overrides JSON: %v", err), "object", nil, nil), nil
		}
		pi.CellOverrides = rawCO
	}

	// Parse optional bounds override
	boundsStr, paramErrBounds := objectParamAsJSON(request, "bounds")
	if paramErrBounds != nil {
		return paramErrBounds, nil
	}
	if boundsStr != "" {
		var b jsonschema.GridBoundsInput
		if err := json.Unmarshal([]byte(boundsStr), &b); err != nil {
			return argInvalidJSON("bounds", fmt.Sprintf("invalid bounds JSON: %v", err), "object", nil, nil), nil
		}
		pi.Bounds = &b
	}

	// Parse optional max_height_pct convenience alias
	if mhpRaw, ok := request.GetArguments()["max_height_pct"]; ok && mhpRaw != nil {
		if mhp, ok := mhpRaw.(float64); ok && mhp > 0 {
			pi.MaxHeightPct = mhp
		}
	}

	// Build ExpandContext — use template layout bounds if provided, else defaults
	templateName, _ := request.RequireString("theme_template")
	expandCtx, boundsSource, err := resolveExpandContext(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", fmt.Sprintf("template %q: %v", templateName, err)), nil
	}

	// Use the shared helper so single-pattern and batch (expand_patterns) tools
	// emit identical per-pattern shapes.
	result, err := buildPatternExpansionResult(pi, expandCtx, boundsSource, reg)
	if err != nil {
		return api.MCPDiagnosticsError(diagnostics.FromJoinedError(err, "PATTERN_ERROR")), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// gridOccupancy reports how much of the layout a pattern fills.
type gridOccupancy struct {
	FilledPct       float64 `json:"filled_pct"`
	RowsUsed        int     `json:"rows_used"`
	RowsEmpty       int     `json:"rows_empty"`
	BoundsHeightPct float64 `json:"bounds_height_pct"`
}

// computeGridOccupancy calculates occupancy metrics for an expanded shape grid.
func computeGridOccupancy(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext) gridOccupancy {
	if grid == nil || len(grid.Rows) == 0 {
		return gridOccupancy{}
	}

	// Determine column count from the grid's Columns field or infer from max cells per row
	numCols := 0
	if len(grid.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(grid.Columns, &n); err == nil {
			numCols = int(n)
		} else {
			var arr []float64
			if err := json.Unmarshal(grid.Columns, &arr); err == nil {
				numCols = len(arr)
			}
		}
	}
	if numCols == 0 {
		for _, row := range grid.Rows {
			if len(row.Cells) > numCols {
				numCols = len(row.Cells)
			}
		}
	}

	totalSlots := len(grid.Rows) * numCols
	filledSlots := 0
	rowsUsed := 0
	rowsEmpty := 0

	for _, row := range grid.Rows {
		rowHasContent := false
		for _, cell := range row.Cells {
			if cell != nil {
				filledSlots++
				rowHasContent = true
			}
		}
		if rowHasContent {
			rowsUsed++
		} else {
			rowsEmpty++
		}
	}

	filledPct := 0.0
	if totalSlots > 0 {
		filledPct = math.Round(float64(filledSlots)/float64(totalSlots)*1000) / 10
	}

	// bounds_height_pct: percentage of the layout area height the grid occupies
	boundsHeightPct := 100.0
	if grid.Bounds != nil && grid.Bounds.Height > 0 {
		boundsHeightPct = grid.Bounds.Height
	}

	return gridOccupancy{
		FilledPct:       filledPct,
		RowsUsed:        rowsUsed,
		RowsEmpty:       rowsEmpty,
		BoundsHeightPct: boundsHeightPct,
	}
}

// collectGridDensityWarnings checks tables in the grid for density issues.
func collectGridDensityWarnings(grid *jsonschema.ShapeGridInput) []patternValidationError {
	var warnings []patternValidationError
	for rowIdx, row := range grid.Rows {
		for cellIdx, cell := range row.Cells {
			if cell != nil && cell.Table != nil {
				tablePath := fmt.Sprintf("shape_grid.rows[%d].cells[%d].table", rowIdx, cellIdx)
				for _, ve := range pipeline.DetectTableDensity(cell.Table, tablePath) {
					warnings = append(warnings, patternValidationError{
						Field:   ve.Path,
						Code:    ve.Code,
						Message: ve.Message,
						Fix:     ve.Fix,
					})
				}
			}
		}
	}
	return warnings
}

// --- Icon tool ---

func mcpListIconsTool() mcp.Tool {
	return mcp.NewTool("list_icons",
		mcp.WithDescription(`List available icon names for use in shape_grid cells via {"icon":{"name":"icon-name"}}. Icons are bundled SVGs in two sets: outline (default, stroke-based) and filled (solid). Use set:name syntax (e.g. "filled:chart-pie") to select a set; plain names default to outline.

Canonical identifier: each set entry returns both a legacy bare-name array (sets[].names) and a structured sets[].icons array. Each entry in sets[].icons has {name, qualified_name}; qualified_name is always "<set>:<name>" (e.g. "filled:chart-pie", "outline:chart-pie") and is the canonical token to drop into icon.name — required for filled icons, since a bare name alone resolves to the outline set.

Pagination: response is an object {sets, total_count, page_size, next_cursor?}. Names are flattened across the requested set(s) and paged; for each page, sets are rebuilt containing only the icons that fall within the slice. count on each set entry reflects icons in that slice, not the full set size; use total_count for the corpus total.

Projection (token-economy): pass fields="compact" to drop the redundant sets[].icons[] dual array (qualified_name is always "<set>:<name>", easy to synthesize). Pass fields="full" for the legacy payload. Omitting fields emits a deprecation hint in warnings[] — future releases will switch the default to compact.

Filtering: filter (preferred) and search (legacy alias) both apply a case-insensitive substring filter on the icon name. Applied before pagination.`),
		mcp.WithRawOutputSchema(outputSchemaListIcons),
		mcp.WithString("set",
			mcp.Description("Icon set to list: outline, filled, or omit for all sets."),
			mcp.Enum("outline", "filled"),
		),
		mcp.WithString("search",
			mcp.Description("Legacy alias for filter. Substring filter applied to icon names. Case-insensitive. Example: \"chart\" returns chart-pie, chart-bar, etc."),
		),
		mcp.WithString("filter",
			mcp.Description("Case-insensitive substring filter on icon name. Equivalent to search; when both are set, filter wins."),
		),
		mcp.WithString("fields",
			mcp.Description("Field projection: compact (slim — drop redundant icons[] dual array) or full (legacy payload). When omitted, behavior matches full and a deprecation hint is returned in warnings[]."),
			mcp.Enum(listFieldsCompact, listFieldsFull),
		),
		mcp.WithString("cursor",
			mcp.Description("Opaque continuation token from a previous response's next_cursor. Omit or pass empty for the first page."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Maximum number of icon names to return (across the requested set(s)). Default: 50. Clamped to [1, 200]."),
		),
	)
}

// iconEntry is the per-icon record returned by list_icons. qualified_name is
// the canonical authoring identifier (always "<set>:<name>") that agents can
// drop directly into an `icon.name` field — including for filled icons where
// the bare name alone would resolve to the outline set.
type iconEntry struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
}

// iconSetResult is the JSON shape for each icon set in the list_icons response.
// When paginated, count reflects the icons included on the current page, not
// the full set size; the envelope's total_count covers the entire filtered
// corpus. `names` is preserved for backward compatibility; new consumers
// should use `icons[].qualified_name`.
//
// The structured `icons[]` array carries omitempty so the compact projection
// (fields="compact") can drop it from the wire; callers can still synthesize
// qualified_name as `set + ":" + name` on demand.
type iconSetResult struct {
	Set   string      `json:"set"`
	Count int         `json:"count"`
	Names []string    `json:"names"`
	Icons []iconEntry `json:"icons,omitempty"`
}

// listIconsResponse is the paginated envelope for list_icons.
type listIconsResponse struct {
	Sets       []iconSetResult `json:"sets"`
	TotalCount int             `json:"total_count"`
	PageSize   int             `json:"page_size"`
	NextCursor string          `json:"next_cursor,omitempty"`
	// Warnings carries advisory hints (currently: deprecation notice when
	// `fields` is omitted).
	Warnings []string `json:"warnings,omitempty"`
}

func handleListIcons(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	offset, pageSize, errField, errMsg := paginationParams(request)
	if errMsg != "" {
		return argInvalidValue("list_icons", "INVALID_PARAMETER", errField, errMsg, "", nil, nil), nil
	}

	fieldsMode, fieldsExplicit, fErrField, fErrMsg := listFieldsParam(request)
	if fErrMsg != "" {
		return argInvalidValue("list_icons", "INVALID_PARAMETER", fErrField, fErrMsg, "string", "compact", nil), nil
	}
	if !fieldsExplicit {
		fieldsMode = listFieldsFull
	}

	sets := []string{"outline", "filled"}
	if s, err := request.RequireString("set"); err == nil && s != "" {
		sets = []string{s}
	}

	// `filter` is the canonical name; `search` is the legacy alias. When both
	// are supplied, filter wins. Trim+lower once for the inner loop.
	filterStr := listFilterParam(request)
	if filterStr == "" {
		s, _ := request.RequireString("search")
		filterStr = strings.ToLower(strings.TrimSpace(s))
	}

	// Flatten (set, name) pairs across all requested sets, applying the
	// search filter as we go. Preserves intra-set ordering and overall
	// set order.
	type setName struct {
		set, name string
	}
	flat := make([]setName, 0, 256)
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return api.MCPSimpleError("ICON_LIST", fmt.Sprintf("listing %s icons: %v", s, err)), nil
		}
		for _, n := range names {
			if filterStr == "" || strings.Contains(strings.ToLower(n), filterStr) {
				flat = append(flat, setName{set: s, name: n})
			}
		}
	}

	totalCount := len(flat)
	start, end, nextCursor := paginationSlice(totalCount, offset, pageSize)
	page := flat[start:end]

	// Re-group page entries by set, preserving the original set order.
	pageBySet := make(map[string][]string, len(sets))
	for _, e := range page {
		pageBySet[e.set] = append(pageBySet[e.set], e.name)
	}
	pageSets := make([]iconSetResult, 0, len(sets))
	for _, s := range sets {
		if names, ok := pageBySet[s]; ok {
			entry := iconSetResult{
				Set:   s,
				Count: len(names),
				Names: names,
			}
			if fieldsMode == listFieldsFull {
				entries := make([]iconEntry, len(names))
				for i, n := range names {
					entries[i] = iconEntry{Name: n, QualifiedName: s + ":" + n}
				}
				entry.Icons = entries
			}
			pageSets = append(pageSets, entry)
		}
	}

	resp := listIconsResponse{
		Sets:       pageSets,
		TotalCount: totalCount,
		PageSize:   pageSize,
		NextCursor: nextCursor,
	}
	if !fieldsExplicit {
		resp.Warnings = append(resp.Warnings, defaultFieldsDeprecation)
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
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

Results are cached by file content hash — repeated calls with unchanged PPTX return instantly. Pass force=true to re-render even if cached.

Cost note: a density=100 slide is typically 20-80KB base64. Higher densities produce larger payloads.`),
		mcp.WithRawOutputSchema(outputSchemaRenderSlideImage),
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
		mcp.WithBoolean("force",
			mcp.Description("If true, bypass the render cache and re-convert even if a cached result exists. Default: false."),
		),
	)
}

func mcpRenderDeckThumbnailsTool() mcp.Tool {
	return mcp.NewTool("render_deck_thumbnails",
		mcp.WithDescription(`Render all slides in a PPTX as low-resolution PNG thumbnails. Returns an array of base64-encoded PNGs.

Requires LibreOffice and ImageMagick (magick) on PATH. Use this for a quick visual overview of the entire deck.

Results are cached by file content hash — repeated calls with unchanged PPTX return instantly. Pass force=true to re-render even if cached.

Cost note: at density=50, each thumbnail is typically 5-20KB base64. A 10-slide deck is ~100-200KB total.`),
		mcp.WithRawOutputSchema(outputSchemaRenderDeckThumbnails),
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
		mcp.WithBoolean("force",
			mcp.Description("If true, bypass the render cache and re-convert even if a cached result exists. Default: false."),
		),
	)
}

func (mc *mcpConfig) handleRenderSlideImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil {
		return argMissing("render_slide_image", "pptx_path", "string", "/tmp/out/deck.pptx", nil), nil
	}

	if err := api.ValidatePptxPath(pptxPath); err != nil {
		return argInvalidValue("render_slide_image", "INVALID_PATH", "pptx_path", err.Error(), "string", "/tmp/out/deck.pptx", nil), nil
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

	force := false
	if v, ok := request.GetArguments()["force"].(bool); ok {
		force = v
	}

	img, err := render.RenderSlideOpts(pptxPath, slideIndex, density, force)
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
		return argMissing("render_deck_thumbnails", "pptx_path", "string", "/tmp/out/deck.pptx", nil), nil
	}

	if err := api.ValidatePptxPath(pptxPath); err != nil {
		return argInvalidValue("render_deck_thumbnails", "INVALID_PATH", "pptx_path", err.Error(), "string", "/tmp/out/deck.pptx", nil), nil
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

	force := false
	if v, ok := request.GetArguments()["force"].(bool); ok {
		force = v
	}

	deckResult, err := render.RenderDeckOpts(pptxPath, density, maxSlides, force)
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
