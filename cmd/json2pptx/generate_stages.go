package main

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"os"
	"time"
)

type parsedGenerateRequest struct {
	input                            *PresentationInput
	jsonStr, sourceDeckID, strictFit string
	authored                         *authoredPaths
	boundaryDiags                    []diagnostics.Diagnostic
	structureWarnings                []string
}

func (mc *mcpConfig) parseGenerateRequest(request mcp.CallToolRequest) (*parsedGenerateRequest, *mcp.CallToolResult) {
	jsonStr, sourceDeckID, paramErr := mc.presentationForTool("generate_presentation", request)
	if paramErr != nil {
		return nil, paramErr
	}
	if jsonStr == "" {
		return nil, argRequired(request, "generate_presentation", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema())
	}

	// Parse JSON input — reject trailing data.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return nil, argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil)
	}

	applyRequestTemplateOverride(&input, request)

	// Apply deck-level defaults before any validation or conversion.
	applyDefaults(&input)

	// Resolve named style references from template settings (priority 3).
	mc.resolveInputNamedSettings(&input)

	// Expand structure block into flat slides (mutually exclusive with
	// top-level slides). Mirrors the CLI path so MCP and CLI agree on the
	// effective slide list before any slide-count checks.
	if structDiags := applyStructureExpansion(&input); len(structDiags) > 0 {
		return nil, api.MCPDiagnosticsError(structDiags)
	}
	// Every finding this call returns is addressed to the deck the caller
	// sent, not to the expanded slide list the engine works on.
	authored := newAuthoredPaths(&input)

	strictUnknownKeys, _ := request.GetArguments()["strict_unknown_keys"].(bool)
	boundaryDiags, structureWarnings := deckBoundaryDiagnostics(&input, placeholderPolicyFromRequest(request), false)
	boundaryDiags = append(boundaryDiags, unknownKeyDiags([]byte(jsonStr), strictUnknownKeys)...)

	// Fail fast if any boundary diagnostic is an error.
	if diagnostics.HasErrors(boundaryDiags) {
		return nil, authored.mcpError(boundaryDiags)
	}

	// strict_fit parameter (default: warn) — parsed up front so it can be
	// forwarded to generation (StrictFit below) even though the text-fit gate
	// itself now runs after template analysis (see below).
	strictFit := "warn"
	if sf, err := request.RequireString("strict_fit"); err == nil && sf != "" {
		strictFit = sf
	}

	return &parsedGenerateRequest{input: &input, jsonStr: jsonStr, sourceDeckID: sourceDeckID, authored: authored, boundaryDiags: boundaryDiags, structureWarnings: structureWarnings, strictFit: strictFit}, nil
}

type generateRender struct {
	run                            RenderResult
	urls                           *urlMaterialization
	assetFindings                  []diagnostics.Diagnostic
	outputValidation               string
	duration                       time.Duration
	templateCleanup, renderCleanup func()
}

func (r *generateRender) close() {
	if r.renderCleanup != nil {
		r.renderCleanup()
	}
	if r.urls != nil {
		r.urls.Cleanup()
	}
	if r.templateCleanup != nil {
		r.templateCleanup()
	}
}

func (mc *mcpConfig) runGenerateRequest(ctx context.Context, request mcp.CallToolRequest, parsed *parsedGenerateRequest) (*generateRender, *mcp.CallToolResult) {
	rendered := &generateRender{}
	input, authored, jsonStr, strictFit := parsed.input, parsed.authored, parsed.jsonStr, parsed.strictFit
	// Create output directory
	if err := os.MkdirAll(mc.outputDir, 0755); err != nil {
		return rendered, api.MCPSimpleError("OUTPUT_DIR", fmt.Sprintf("failed to create output directory: %v", err))
	}

	// Resolve template: a registered name, or a guarded local .pptx.
	templatePath, templateCleanup, tplDiag := mc.resolveTemplateSource(request, "generate_presentation",
		"presentation.template", "presentation.template_path", input.Template, input.TemplatePath)
	if tplDiag != nil {
		if override, _ := request.RequireString("template"); override != "" {
			tplDiag.Path = "template"
			tplDiag = withTemplateRetry(tplDiag, request, "generate_presentation", "template")
		}
		return rendered, authored.mcpError([]diagnostics.Diagnostic{*tplDiag})
	}
	rendered.templateCleanup = templateCleanup

	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return rendered, baseDirErr
	}
	urls, resolveURLsNow, urlErr := mc.prepareURLs(input.Slides)
	rendered.urls = urls
	if urlErr != nil {
		return rendered, api.MCPSimpleError("URL_RESOLVER_INIT", fmt.Sprintf("resource resolver: %v", urlErr))
	}
	var assetFindings []diagnostics.Diagnostic
	var assetFailure *mcp.CallToolResult
	outputFilename := input.OutputFilename
	if outputFilename == "" {
		outputFilename = deckSpecOutputFilename("presentation", []byte(jsonStr), firstNonEmpty(input.Template, templatePath))
	}
	if reqFilename, err := request.RequireString("output_filename"); err == nil && reqFilename != "" {
		outputFilename = reqFilename
	}
	outputValidation := "strict"
	if ov, ovErr := request.RequireString("output_validation"); ovErr == nil && ov != "" {
		outputValidation = ov
	}
	startTime := time.Now()
	run, cleanup, runErr := RunPresentation(ctx, input, mc.withServerSVGConfig(RenderOptions{
		OutputDir: mc.outputDir, OutputFilename: outputFilename, TemplatesDir: mc.templatesDir,
		TemplateCache: mc.cache, ResolvedTemplatePath: templatePath, StrictFit: strictFit,
		OutputValidation: outputValidation, AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
		AllowedImagePaths: imageAllowList(mc.cfg.Images.AllowedBasePaths, urls.CacheDir),
		PreConvert: func() error {
			resolveURLsNow()
			if len(urls.Findings) > 0 {
				assetFailure = authored.mcpError(urls.Findings)
				return iconFindingsToError(urls.Findings)
			}
			assetFindings = resolveLocalAssetPaths(input.Slides, baseDir, imageAllowList(mc.cfg.Images.AllowedBasePaths, urls.CacheDir)...)
			if ds := diagnostics.FilterBySeverity(assetFindings, diagnostics.SeverityError); len(ds) > 0 {
				assetFailure = authored.mcpError(ds)
				return iconFindingsToError(ds)
			}
			return nil
		},
	}))
	rendered.renderCleanup = cleanup
	rendered.run, rendered.assetFindings = run, assetFindings
	rendered.duration = time.Since(startTime)
	rendered.outputValidation = outputValidation
	if runErr != nil {
		if assetFailure != nil {
			return rendered, assetFailure
		}
		return rendered, generateRunError(input, authored, run, runErr)
	}
	return rendered, nil
}

func assembleGenerateOutput(parsed *parsedGenerateRequest, rendered *generateRender, request mcp.CallToolRequest) (JSONOutput, []patterns.FitFinding) {
	input, authored := parsed.input, parsed.authored
	boundaryDiags, structureWarnings := parsed.boundaryDiags, parsed.structureWarnings
	run := rendered.run
	assetFindings, outputValidation := rendered.assetFindings, rendered.outputValidation
	duration := rendered.duration
	result := run.GenResult
	templateLayouts, syntheticFiles := run.TemplateLayouts, run.SyntheticFiles
	slideSpecs, outputPath := run.SlideSpecs, run.OutputPath
	outputValidationFindings := run.OutputValidationFindings
	theme := run.TemplateTheme
	themeOverrideWarnings := run.ThemeOverrideWarnings
	inputWarnings := append(validateSlidesChartData(input.Slides), structureWarnings...)
	inputWarnings = append(inputWarnings, run.GridDiagWarnings...)
	for _, d := range authored.diagnostics(assetFindings) {
		if d.Severity != diagnostics.SeverityError {
			inputWarnings = append(inputWarnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
		}
	}

	// Merge input-layer warnings with generation warnings.
	allWarnings := append(inputWarnings, result.Warnings...)
	allWarnings = append(allWarnings, themeOverrideWarnings...)
	allWarnings = append(allWarnings, themeFontSubstitutionWarnings(theme)...)
	// Surface deprecation warnings for legacy field usage.
	allWarnings = append(allWarnings, deprecationWarnings(input)...)
	// Surface boundary warnings (e.g. unknown keys) in the response.
	for _, d := range boundaryDiags {
		if d.Severity == diagnostics.SeverityWarning {
			allWarnings = append(allWarnings, d.Message)
		}
	}

	fitReport, _ := request.GetArguments()["fit_report"].(bool)
	verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
	fitFindings := assembleGenerationFindings(input, run, fitReport, verboseFit)

	// Build per-slide resolution summary
	slideResolutions := buildSlideResolutions(input.Slides, slideSpecs, templateLayouts, syntheticFiles,
		droppedPlaceholdersBySlide(fitFindings))

	// Build response
	output := JSONOutput{
		Success:                  renderSucceeded(fitFindings, outputValidation),
		OutputPath:               outputPath,
		SlideCount:               result.SlideCount,
		ContentHash:              result.ContentHash,
		DurationMs:               duration.Milliseconds(),
		Warnings:                 allWarnings,
		Quality:                  computeQualityScoreWithLayouts(input.Slides, allWarnings, templateLayouts, fitFindings...),
		ValidationErrors:         result.ValidationErrors,
		FitFindings:              authored.fitFindings(fitFindings),
		Slides:                   slideResolutions,
		OutputValidationFindings: outputValidationFindings,
	}
	return output, fitFindings
}
