// presentation_runner.go provides a reusable, in-memory render runner that
// drives the raw generation pipeline shared by the CLI `generate` path
// (runJSONMode) and the MCP `generate_presentation` path (handleGenerate).
//
// Both call sites perform their own boundary validation, error shaping, and
// response construction (which differ on purpose: the CLI returns Go errors /
// writes a JSON file, while MCP returns structured diagnostics with
// idempotency). The steps in between — template resolution, template analysis,
// canonical layout resolution, strict_fit evaluation, rhythm-grid resolution,
// slide conversion, generation, and output validation — are identical in
// ordering and semantics. RunPresentation captures exactly that shared middle
// so a compiled, in-memory *deckinput.PresentationInput can be fed through the
// same gates without round-tripping raw JSON to disk.
//
// The runner assumes the caller has already:
//   - parsed the input into a *deckinput.PresentationInput,
//   - applied deck-level defaults (applyDefaults),
//   - resolved named style settings (resolveInputNamedSettings*),
//   - expanded any structure block into flat slides,
//   - run boundary validation (design mode, emoji, enums, unknown keys),
//   - resolved URL and relative asset references on the slides.
//
// It then runs the raw generation pipeline and returns the artifact path plus
// the findings/warnings each caller folds into its own response shape.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// RenderOptions carries the render-time knobs both call sites already compute.
// Zero values are chosen so the strictest, default behaviour is the default:
// StrictFit defaults to "warn" semantics only when set; OutputValidation
// defaults to "strict" when empty (see RunPresentation).
type RenderOptions struct {
	// OutputDir is the directory the .pptx is written into. Created if missing.
	OutputDir string
	// OutputFilename overrides input.OutputFilename when non-empty. The value
	// is sanitized for path traversal.
	OutputFilename string
	// TemplatesDir is the template search directory passed to resolveTemplatePath.
	TemplatesDir string

	// StrictFit is the text-fit gate mode: "off", "warn", or "strict".
	// Empty is treated as "warn" to match the historical CLI/MCP default.
	StrictFit string
	// OutputValidation is the post-generation validation mode: "off", "warn",
	// or "strict". Empty is treated as "strict" — the standing default
	// guarantee that every successful render passes output validation.
	OutputValidation string

	// AccentStrategy is the deck accent rotation strategy.
	AccentStrategy patterns.AccentStrategy

	// Partial drops failing slides instead of aborting (CLI --partial).
	Partial bool

	// SVG strategy / scaling knobs mirrored from config.Config.SVG so the
	// runner does not depend on a particular config plumbing.
	SVGStrategy     string
	SVGScale        float64
	SVGNativeCompat string
	MaxPNGWidth     int

	// PreConvert, when non-nil, runs AFTER template analysis, canonical layout
	// resolution, and the strict_fit gate, but BEFORE rhythm-grid resolution and
	// slide conversion. This is the exact point where the CLI path historically
	// resolved URL / relative-asset references on input.Slides, so callers can
	// keep that step in its original position (preserving error-precedence
	// ordering) instead of running it before RunPresentation. Returning an error
	// aborts the run; the error is surfaced verbatim to the caller.
	PreConvert func() error
}

// RenderResult bundles everything the callers need to build their responses.
type RenderResult struct {
	// OutputPath is the absolute/relative path the .pptx was written to.
	OutputPath string
	// GenResult is the raw generator output (slide count, hash, warnings,
	// media failures, fit findings, contrast swaps, validation errors).
	GenResult *generator.GenerationResult
	// SlideSpecs are the converted generator specs (for slide-resolution summaries).
	SlideSpecs []generator.SlideSpec

	// TemplateLayouts / SyntheticFiles / dimensions / theme are returned so
	// callers can build slide-resolution summaries and fit reports without
	// re-analyzing the template.
	TemplateLayouts  []types.LayoutMetadata
	SyntheticFiles   map[string][]byte
	SlideWidth       int64
	SlideHeight      int64
	TemplateMetadata *types.TemplateMetadata
	TemplateTheme    types.ThemeInfo

	// Findings collected during the run, in the same buckets the callers merge.
	SynthesisFindings  []patterns.FitFinding // template-level synthesis
	StrictFitFindings  []patterns.FitFinding // text-fit (warn mode) findings
	GridDiagWarnings   []string              // slide-conversion warnings
	GridVisualFindings []patterns.FitFinding // diagram narrow-cell etc.

	// OutputValidationFindings is the post-generation validation report's findings.
	OutputValidationFindings []pptx.Finding
}

// RunPresentation executes the shared raw generation pipeline in memory.
//
// It does NOT mutate observable behaviour of either caller: the ordering and
// semantics of each step mirror the inline code in runJSONMode and
// handleGenerate. The first error returned aborts at the same point the
// inline code would have. Strict output validation is the default (empty
// OutputValidation => "strict").
//
// ctx is forwarded to generator.Generate. cleanup must be called by the caller
// (it releases the resolved template path).
func RunPresentation(ctx context.Context, input *PresentationInput, opts RenderOptions) (res RenderResult, cleanup func(), err error) {
	cleanup = func() {}

	strictFit := opts.StrictFit
	if strictFit == "" {
		strictFit = "warn"
	}
	outputValidation := opts.OutputValidation
	if outputValidation == "" {
		outputValidation = "strict"
	}

	// Create output directory.
	if mkErr := os.MkdirAll(opts.OutputDir, 0o755); mkErr != nil {
		return res, cleanup, fmt.Errorf("failed to create output directory: %w", mkErr)
	}

	// Resolve template path using the shared search path (flag, env, home, cwd, embedded).
	templatePath, templateCleanup, tplErr := resolveTemplatePath(input.Template, opts.TemplatesDir)
	if tplErr != nil {
		return res, cleanup, fmt.Errorf("%s", templateNotFoundError(input.Template, opts.TemplatesDir))
	}
	cleanup = templateCleanup

	// Analyze template for layout metadata, synthetic files, and dimensions.
	templateLayouts, syntheticFiles, slideWidth, slideHeight, templateMetadata, templateTheme, synthesisFindings := analyzeTemplateLayouts(templatePath)
	res.TemplateLayouts = templateLayouts
	res.SyntheticFiles = syntheticFiles
	res.SlideWidth = slideWidth
	res.SlideHeight = slideHeight
	res.TemplateMetadata = templateMetadata
	res.TemplateTheme = templateTheme
	res.SynthesisFindings = synthesisFindings

	// Resolve canonical layout names to concrete layout IDs.
	resolveCanonicalLayoutIDs(input.Slides, templateLayouts)

	// Text-fit checking via strict_fit. Runs AFTER template analysis and
	// canonical-layout resolution so shape_grid fit checks resolve against the
	// SAME layout-aware bounds generation renders. Strict mode aborts on
	// refuse-class findings; the caller is responsible for the stderr NDJSON
	// dump on refuse (it is part of each caller's error shape).
	if strictFit != "off" {
		rawFindings, refuseErr := evaluateStrictFit(input, strictFit, templateLayouts, slideWidth, slideHeight)
		if refuseErr != nil {
			return res, cleanup, &StrictFitRefusal{Findings: rawFindings, Err: refuseErr}
		}
		for _, f := range rawFindings {
			res.StrictFitFindings = append(res.StrictFitFindings, convertTextFitFinding(f))
		}
	}

	// Caller-supplied pre-conversion step (e.g. URL / relative-asset resolution
	// on input.Slides). Runs in the same position the CLI path historically used
	// so error precedence is unchanged.
	if opts.PreConvert != nil {
		if hookErr := opts.PreConvert(); hookErr != nil {
			return res, cleanup, hookErr
		}
	}

	// Resolve deck-level rhythm grid when configured.
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		if gridErr := validateGridConfig(input.Grid); gridErr != nil {
			return res, cleanup, fmt.Errorf("grid: %w", gridErr)
		}
		rhythmGrid = resolveGrid(input.Grid, templateLayouts, slideWidth, slideHeight)
	}

	// Convert typed slides to generator specs.
	dataPalette := resolveDataPalette(templateMetadata, templateTheme.Colors)
	diagCtx := &GridDiagramContext{
		ThemeColors: templateTheme.Colors,
		DataPalette: dataPalette,
		FontFamily:  templateTheme.BodyFont,
	}
	slideSpecs, gridDiagWarnings, gridVisualFindings, convErr := convertPresentationSlides(
		input.Slides, templateLayouts, slideWidth, slideHeight, templateMetadata,
		rhythmGrid, opts.AccentStrategy, diagCtx, opts.Partial,
	)
	if convErr != nil {
		return res, cleanup, fmt.Errorf("invalid slide specification: %w", convErr)
	}
	res.SlideSpecs = slideSpecs
	res.GridDiagWarnings = gridDiagWarnings
	res.GridVisualFindings = gridVisualFindings

	// Determine output filename — sanitize to prevent path traversal.
	outputFilename := sanitizeOutputFilename(input.OutputFilename)
	if opts.OutputFilename != "" {
		outputFilename = sanitizeOutputFilename(opts.OutputFilename)
	}
	outputPath := filepath.Join(opts.OutputDir, outputFilename)
	res.OutputPath = outputPath

	// Build the generation request.
	genReq := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		SVGStrategy:           opts.SVGStrategy,
		SVGScale:              opts.SVGScale,
		SVGNativeCompat:       opts.SVGNativeCompat,
		MaxPNGWidth:           opts.MaxPNGWidth,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
		StrictFit:             strictFit,
		DataPalette:           dataPalette,
	}

	// Wire footer/chrome configuration. Chrome supersedes footer.
	if input.Chrome != nil {
		genReq.Footer = chromeToFooterConfig(input.Chrome, len(slideSpecs))
		applyChromeSkip(slideSpecs, input.Chrome, input.Slides, templateLayouts)
	} else if input.Footer != nil && input.Footer.Enabled {
		genReq.Footer = &generator.FooterConfig{
			Enabled:  true,
			LeftText: input.Footer.LeftText,
		}
	}

	// Wire theme override.
	if input.ThemeOverride != nil {
		genReq.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}

	genResult, genErr := generator.Generate(ctx, genReq)
	if genErr != nil {
		return res, cleanup, fmt.Errorf("failed to generate PPTX: %w", genErr)
	}
	res.GenResult = genResult

	// Post-generation output validation (default: strict).
	if outputValidation != "off" {
		report, valErr := pptx.ValidateOutputFile(outputPath)
		if valErr != nil {
			return res, cleanup, fmt.Errorf("output validation failed: %w", valErr)
		}
		res.OutputValidationFindings = report.Findings

		if outputValidation == "strict" && !report.IsValid() {
			return res, cleanup, &OutputValidationFailure{Report: report}
		}
	}

	return res, cleanup, nil
}

// StrictFitRefusal is returned when strict_fit=strict encounters a refuse-class
// finding. Callers unwrap it to preserve their distinct error-shaping (the CLI
// dumps NDJSON to stderr + returns the error; MCP emits structured diagnostics).
type StrictFitRefusal struct {
	Findings []fitFinding
	Err      error
}

func (e *StrictFitRefusal) Error() string { return e.Err.Error() }
func (e *StrictFitRefusal) Unwrap() error { return e.Err }

// OutputValidationFailure is returned when output_validation=strict finds
// blocking issues. Callers unwrap the report to build their error response.
type OutputValidationFailure struct {
	Report *pptx.Report
}

func (e *OutputValidationFailure) Error() string {
	blocking := e.Report.Blocking()
	if len(blocking) == 0 {
		return "output validation failed (strict)"
	}
	return fmt.Sprintf("output validation failed (strict): %d blocking finding(s)", len(blocking))
}
