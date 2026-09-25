package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/placeholderrole"
	"github.com/sebahrens/json2pptx/internal/policy/emoji"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
	"github.com/sebahrens/json2pptx/svggen"
)

// ConversionResult holds the result of a single file conversion.
type ConversionResult struct {
	InputPath    string
	OutputPath   string
	Success      bool
	Error        string
	SlideCount   int
	FailedSlides int
	Duration     time.Duration
}

// JSONInput represents the JSON input format for headless conversions.
// This allows direct specification of slides without markdown parsing.
type JSONInput struct {
	// Template is the template name (without .pptx extension)
	Template string `json:"template"`

	// OutputFilename is the desired output filename (optional, defaults to output.pptx)
	OutputFilename string `json:"output_filename,omitempty"`

	// Footer controls slide footer injection (optional, disabled by default)
	Footer *JSONFooter `json:"footer,omitempty"`

	// ThemeOverride allows per-deck theme color/font overrides
	ThemeOverride *ThemeInput `json:"theme_override,omitempty"`

	// Slides contains the slide specifications
	Slides []JSONSlide `json:"slides"`
}

// JSONFooter configures slide footer injection.
// Defined in internal/deckinput; aliased here so package-main call sites are unchanged.
type JSONFooter = deckinput.JSONFooter

// JSONSlide represents a single slide in JSON input format.
type JSONSlide struct {
	// LayoutID is the layout identifier (e.g., "slideLayout1", "Title Slide")
	LayoutID string `json:"layout_id"`

	// Content contains the content items for placeholders
	Content []JSONContentItem `json:"content"`

	// SpeakerNotes is optional speaker notes text
	SpeakerNotes string `json:"speaker_notes,omitempty"`

	// Source is optional source attribution text
	Source     string     `json:"source,omitempty"`
	SourceLink *LinkInput `json:"source_link,omitempty"`

	// Takeaway is the slide's headline answer / "so what" line. Renders
	// as bold text above the source note row. Strongly recommended on
	// chart and matrix slides.
	Takeaway string `json:"takeaway,omitempty"`

	// Transition is the slide transition type: "fade", "push", "wipe", etc.
	Transition string `json:"transition,omitempty"`

	// TransitionSpeed is the transition speed: "slow", "med", "fast"
	TransitionSpeed string `json:"transition_speed,omitempty"`

	// Build is the build animation: "bullets" for one-by-one bullet reveal
	Build string `json:"build,omitempty"`

	// ContrastCheck controls WCAG text contrast enforcement. Default true.
	// Set to false to preserve user-specified text colors even when contrast is low.
	ContrastCheck *bool `json:"contrast_check,omitempty"`
}

// JSONContentItem represents content to place in a placeholder.
type JSONContentItem struct {
	// PlaceholderID identifies the target placeholder
	PlaceholderID string `json:"placeholder_id"`

	// Type is the content type: "text", "bullets", "image", or "chart"
	Type string `json:"type"`

	// Value is the content value (type depends on Type field)
	// - text: string
	// - bullets: []string
	// - image: {path: string, alt: string}
	// - chart: {type: string, title: string, data: {label: value}}
	Value json.RawMessage `json:"value"`
	Link  *LinkInput      `json:"link,omitempty"`

	// FontSize overrides the template's default font size (in points, e.g., 72).
	FontSize *float64 `json:"font_size,omitempty"`
}

func toGeneratorLink(link *LinkInput) *generator.LinkSpec {
	if link == nil {
		return nil
	}
	return &generator.LinkSpec{URL: link.URL, Slide: link.Slide}
}

// SlideResolution describes how a single slide was resolved during generation.
// It tells agents which layout was actually used (after auto-selection / fallback),
// which placeholders received content, and whether synthesis or pagination applied.
type SlideResolution struct {
	Index            int      `json:"index"`
	ResolvedLayoutID string   `json:"resolved_layout_id"`
	PlaceholdersUsed []string `json:"placeholders_used"`
	// PlaceholdersDropped lists the placeholder IDs the slide targeted that the
	// resolved layout does not declare. Their content was NOT rendered, so they
	// are excluded from PlaceholdersUsed (go-slide-creator-lhq6).
	PlaceholdersDropped []string `json:"placeholders_dropped,omitempty"`
	WasSynthesized      bool     `json:"was_synthesized_layout,omitempty"`
	WasAutoSelected     bool     `json:"was_auto_selected,omitempty"`
	OccupancyPct        int      `json:"occupancy_pct,omitempty"`
}

// JSONOutput represents the JSON output for headless mode.
type JSONOutput struct {
	Success                  bool                        `json:"success"`
	OutputPath               string                      `json:"output_path,omitempty"`
	SlideCount               int                         `json:"slide_count,omitempty"`
	ContentHash              string                      `json:"content_hash,omitempty"`
	DurationMs               int64                       `json:"duration_ms,omitempty"`
	Error                    string                      `json:"error,omitempty"`
	Warnings                 []string                    `json:"warnings,omitempty"`
	SlideErrors              []SlideError                `json:"slide_errors,omitempty"`
	Quality                  *QualityScore               `json:"quality,omitempty"`
	ValidationErrors         []*patterns.ValidationError `json:"validation_errors,omitempty"`
	FitFindings              []patterns.FitFinding       `json:"fit_findings,omitempty"`
	Slides                   []SlideResolution           `json:"slides,omitempty"`
	OutputValidationFindings []pptx.Finding              `json:"output_validation_findings,omitempty"`
	// IdempotentReplay is true when this response was served from the
	// idempotency cache instead of regenerated. Only set on MCP responses
	// when an idempotency_key was supplied and matched a prior call.
	IdempotentReplay bool `json:"idempotent_replay,omitempty"`
}

// SlideError describes a render-time failure for a specific slide.
// These are populated from generator.MediaFailure records when charts,
// diagrams, images, or tables fail to render during PPTX generation.
type SlideError struct {
	SlideNumber int    `json:"slide_number"`
	ContentType string `json:"content_type"`           // "diagram", "image", "table"
	DiagramType string `json:"diagram_type,omitempty"` // e.g., "pie_chart", "timeline"
	Error       string `json:"error"`
	Fallback    string `json:"fallback,omitempty"` // what was done instead: "placeholder_image", "skipped"
}

// QualityScore provides an overall quality estimate for the generated deck.
type QualityScore struct {
	Score       float64                   `json:"score"`                  // 0-100 input-heuristic quality estimate
	Basis       string                    `json:"basis"`                  // always scoreBasisInput
	SlideScores []SlideQuality            `json:"slide_scores,omitempty"` // per-slide breakdown
	Issues      []string                  `json:"issues,omitempty"`       // quality concerns
	Scope       string                    `json:"scope"`                  // input_heuristic; never a visual verdict
	Evidence    *pipeline.QualityEvidence `json:"evidence,omitempty"`
	// StructuralScore and QualityGate carry the deterministic verdict on the
	// paths that generate a deck (render_deck_spec). quality_summary alone is an
	// input heuristic that cannot fail, so the recommended path had no gate at
	// all (go-slide-creator-05wn). Score is capped at StructuralScore when both
	// are present: the headline number an agent reads first must not claim more
	// than the structural verdict.
	StructuralScore int                        `json:"structural_score,omitempty"`
	QualityGate     *deterministic.QualityGate `json:"quality_gate,omitempty"`
}

// SlideQuality provides quality metrics for a single slide.
type SlideQuality struct {
	SlideNumber int      `json:"slide_number"`
	Score       float64  `json:"score"` // 0-100
	Issues      []string `json:"issues,omitempty"`
}

// parseJSONInput reads JSON from a file or stdin, applies patch operations if present,
// applies the template and design_mode overrides, and validates required fields.
// The returned warnings include unknown-key detection (additionalProperties:false
// enforcement). A non-empty designModeOverride replaces input.DesignMode after
// parsing so the CLI flag wins over the JSON field. When strictUnknownKeys is
// true, unknown JSON keys are returned as an error instead of warnings — this
// mirrors the MCP strict_unknown_keys=true semantics.
func parseJSONInput(jsonPath, templateOverride, designModeOverride string, strictUnknownKeys bool) (*PresentationInput, []string, error) {
	var inputData []byte
	var err error

	if jsonPath == "-" {
		inputData, err = io.ReadAll(os.Stdin)
	} else {
		inputData, err = os.ReadFile(jsonPath)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read JSON input: %w", err)
	}

	var input PresentationInput
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(inputData, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			return nil, nil, fmt.Errorf("failed to apply patch: %w", patchErr)
		}
		input = *patched
	} else {
		if err := json.Unmarshal(inputData, &input); err != nil {
			return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	if templateOverride != "" {
		input.Template = strings.TrimSuffix(templateOverride, ".pptx")
	}

	if designModeOverride != "" {
		input.DesignMode = designModeOverride
	}

	if input.Template == "" && input.TemplatePath == "" {
		return nil, nil, fmt.Errorf("template is required: use --template flag, or set \"template\" (a registered name) or \"template_path\" (a local .pptx) in JSON input")
	}
	if input.Template != "" && input.TemplatePath != "" {
		return nil, nil, fmt.Errorf("set only one of \"template\" (a registered name) or \"template_path\" (a local .pptx), not both")
	}
	// A structure block IS the slide list — it expands into one later in
	// runJSONMode. Rejecting an empty slides[] here made the documented
	// structure form unusable from the CLI, though validate accepted it and
	// generate_presentation rendered it: the same deck, two verdicts
	// (go-slide-creator-m1kg). The expansion checks its own result below.
	if len(input.Slides) == 0 && input.Structure == nil {
		return nil, nil, fmt.Errorf("at least one slide is required: provide \"slides\" or a top-level \"structure\" block")
	}

	// Check for unknown keys (additionalProperties:false). Warn by default; when
	// --strict-unknown-keys is set, unknown keys become a hard error so a typo
	// like "tmplate" or "slidez" fails generation instead of being silently
	// dropped during JSON unmarshal.
	var warnings []string
	unknownKeyErrs := checkInputUnknownKeys(inputData)
	if strictUnknownKeys && len(unknownKeyErrs) > 0 {
		msgs := make([]string, len(unknownKeyErrs))
		for i, ve := range unknownKeyErrs {
			msgs[i] = ve.Error()
		}
		return nil, nil, fmt.Errorf("unknown JSON keys (strict mode): %s", strings.Join(msgs, "; "))
	}
	for _, ve := range unknownKeyErrs {
		warnings = append(warnings, ve.Error())
	}

	// Enum validation — unknown values are errors (they silently become no-ops).
	if enumErrs := checkInputEnumValues(&input); len(enumErrs) > 0 {
		msgs := make([]string, len(enumErrs))
		for i, ve := range enumErrs {
			msgs[i] = ve.Error()
		}
		return nil, nil, fmt.Errorf("enum validation failed: %s", strings.Join(msgs, "; "))
	}

	return &input, warnings, nil
}

// loadRunConfig loads configuration from configPath (always, so environment
// overrides apply even when no --config is supplied) and then applies explicit
// CLI overrides. An empty templatesDir/outputDir means the flag was not
// explicitly set, so the config-file/env/default value is preserved.
func loadRunConfig(configPath, templatesDir, outputDir string, chartPNG bool) (config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to load config: %w", err)
	}
	if templatesDir != "" {
		cfg.Templates.Dir = templatesDir
	}
	if outputDir != "" {
		cfg.Storage.OutputDir = outputDir
	}
	if chartPNG {
		slog.Warn("--chart-png is deprecated and will be removed in a future release; native SVG is the default strategy")
		cfg.SVG.Strategy = types.SVGStrategyPNG
	}
	return cfg, nil
}

// analyzeTemplateLayouts opens a template, parses layouts, synthesizes missing layouts,
// normalizes placeholder names, and returns the metadata needed for slide conversion.
func analyzeTemplateLayouts(templatePath string) ([]types.LayoutMetadata, map[string][]byte, int64, int64, *types.TemplateMetadata, types.ThemeInfo, []patterns.FitFinding) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, nil, 0, 0, nil, types.ThemeInfo{}, nil
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, nil, 0, 0, nil, types.ThemeInfo{}, nil
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

	// Parse optional template metadata (for semantic accents, layout hints, etc.)
	metadata, _ := template.ParseMetadata(reader)
	if metadata != nil {
		theme.SemanticAccents = metadata.SemanticAccents
	}

	// Normalize placeholder names to canonical form (body, body_2, body_3, etc.)
	normalizedFiles, normErr := template.NormalizeLayoutFiles(reader, analysis.Layouts)
	if normErr == nil && len(normalizedFiles) > 0 {
		if analysis.Synthesis == nil {
			analysis.Synthesis = &types.SynthesisManifest{
				SyntheticFiles: normalizedFiles,
			}
		} else {
			for path, data := range normalizedFiles {
				analysis.Synthesis.SyntheticFiles[path] = data
			}
		}
	}

	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	return analysis.Layouts, syntheticFiles, slideWidth, slideHeight, metadata, theme, synthesisFindings
}

// runJSONMode processes JSON input and generates PPTX.
func runJSONMode(jsonPath, jsonOutputPath, templatesDir, outputDir, configPath string, verbose bool, chartPNG bool, templateOverride string, strictFit string, partial bool, outputValidation string, designModeOverride string, strictUnknownKeys bool) error { //nolint:gocognit,gocyclo
	startTime := time.Now()

	// If --output looks like a .pptx file path (e.g. "/tmp/deck.pptx"), split
	// it into the parent directory + filename so users can intuitively pass a
	// file destination instead of being forced to use a directory. The filename
	// portion overrides input.OutputFilename below.
	outputFilenameOverride := ""
	if strings.HasSuffix(strings.ToLower(outputDir), ".pptx") {
		outputFilenameOverride = filepath.Base(outputDir)
		outputDir = filepath.Dir(outputDir)
	}

	// Parse and validate JSON input
	input, inputWarnings, err := parseJSONInput(jsonPath, templateOverride, designModeOverride, strictUnknownKeys)
	if err != nil {
		return writeJSONError(jsonOutputPath, err)
	}
	if outputFilenameOverride != "" {
		input.OutputFilename = outputFilenameOverride
	}
	// inputWarnings consumed below via result.Warnings merge.

	// Apply deck-level defaults before any validation or conversion.
	applyDefaults(input)

	// Load configuration (config file + env overrides) with explicit CLI
	// overrides before resolving template-dependent settings, so named-style
	// resolution and the rest of the pipeline share one effective templates dir.
	cfg, err := loadRunConfig(configPath, templatesDir, outputDir, chartPNG)
	if err != nil {
		return writeJSONError(jsonOutputPath, err)
	}

	// Resolve named style references from template settings (shared with MCP).
	resolveInputNamedSettingsForDir(cfg.Templates.Dir, input)

	// Expand structure block into flat slides (mutually exclusive with top-level slides).
	if input.Structure != nil {
		if len(input.Slides) > 0 {
			return writeJSONError(jsonOutputPath, fmt.Errorf("structure and slides are mutually exclusive — use one or the other"))
		}
		expanded, err := expandStructure(input.Structure)
		if err != nil {
			return writeJSONError(jsonOutputPath, fmt.Errorf("invalid structure: %w", err))
		}
		if len(expanded) == 0 {
			return writeJSONError(jsonOutputPath, fmt.Errorf("structure expanded to no slides: add a section with slides, or a cover / closing"))
		}
		input.Slides = expanded
	}

	// Check structural grammar (e.g. missing closing slide) and collect warnings.
	structureWarnings := validateStructure(input)
	inputWarnings = append(inputWarnings, structureWarnings...)

	// Enforce design mode constraints — reject raw hex colors and absolute font sizes.
	// In partial mode, drop slides that have violations and emit per-slide warnings
	// instead of aborting the entire run.
	if effectiveDesignMode(input) == designModeConstrained {
		if partial {
			kept := make([]SlideInput, 0, len(input.Slides))
			for i := range input.Slides {
				slideNum := i + 1
				violations := validateSlideDesignMode(&input.Slides[i], slideNum)
				if len(violations) == 0 {
					kept = append(kept, input.Slides[i])
					continue
				}
				msgs := make([]string, 0, len(violations))
				for _, v := range violations {
					msgs = append(msgs, v.Message)
				}
				inputWarnings = append(inputWarnings, fmt.Sprintf(
					"slide %d: skipped (partial mode): design_mode violation(s): %s",
					slideNum, strings.Join(msgs, "; ")))
			}
			input.Slides = kept
		} else if violations := validateDesignMode(input); len(violations) > 0 {
			msgs := make([]string, 0, len(violations))
			for _, v := range violations {
				msg := v.Message
				if v.Fix != nil {
					if text, ok := v.Fix.Params["message"].(string); ok {
						msg += " — fix: " + text
					}
				}
				msgs = append(msgs, msg)
			}
			return writeJSONError(jsonOutputPath, fmt.Errorf(
				"design_mode %q violation(s):\n  %s\n\n"+
					"To allow raw hex colors and absolute font sizes, rerun with --design-mode=free "+
					"or set \"design_mode\": \"free\" in the JSON input",
				effectiveDesignMode(input), strings.Join(msgs, "\n  ")))
		}
	}

	// No-emoji policy — applies regardless of design mode. Emoji codepoints
	// in any user-supplied text are a hard error so authors switch to the
	// bundled SVG icon set or user-provided icons.
	if emojiViolations := emoji.ValidateNoEmojiInText(input); len(emojiViolations) > 0 {
		msgs := make([]string, 0, len(emojiViolations))
		for _, v := range emojiViolations {
			msgs = append(msgs, v.Message)
		}
		return writeJSONError(jsonOutputPath, fmt.Errorf(
			"no_emoji policy violation(s):\n  %s",
			strings.Join(msgs, "\n  ")))
	}

	// urlResolverCleanup releases any URL-download cache opened by the PreConvert
	// hook. Held here so it survives until after generation (the resolved local
	// paths are embedded in the slides). Defaulted to a no-op.
	urlResolverCleanup := func() {}
	defer func() { urlResolverCleanup() }()

	// preConvertErr captures a fatal error raised inside the PreConvert hook so
	// it can be surfaced with the original CLI error shape (the hook itself only
	// returns a sentinel; the real error is stored here).
	var preConvertErr error

	// Run the shared raw generation pipeline. RunPresentation performs, in the
	// same order as before: output-dir creation, template resolution, template
	// analysis, canonical layout resolution, strict_fit, then (via PreConvert)
	// URL + relative-asset resolution, then rhythm-grid resolution, slide
	// conversion, generation, and output validation.
	// A deck that names its own .pptx resolves it against the deck file's own
	// directory, the same frame relative asset paths use (go-slide-creator-ydbk).
	resolvedTemplatePath, tplPathErr := resolveDeckTemplatePath(input.TemplatePath, jsonPath)
	if tplPathErr != nil {
		return tplPathErr
	}

	runRes, renderCleanup, renderErr := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir:            cfg.Storage.OutputDir,
		TemplatesDir:         cfg.Templates.Dir,
		ResolvedTemplatePath: resolvedTemplatePath,
		StrictFit:            strictFit,
		OutputValidation:     outputValidation,
		AccentStrategy:       patterns.AccentStrategy(input.AccentStrategy),
		Partial:              partial,
		SVGStrategy:          string(cfg.SVG.Strategy),
		SVGScale:             cfg.SVG.Scale,
		SVGNativeCompat:      string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:          cfg.SVG.MaxPNGWidth,
		PreConvert: func() error {
			// Resolve any URL references (icon.url, image.url, background.url) by
			// downloading them to a session-scoped cache with SSRF protection.
			if hasURLReferences(input.Slides) {
				resolver, resolverErr := resource.NewResolver(resource.ResolverOptions{})
				if resolverErr != nil {
					preConvertErr = fmt.Errorf("resource resolver: %w", resolverErr)
					return preConvertErr
				}
				urlResolverCleanup = func() { resolver.Close() }
				if urlFindings := resolveURLs(input.Slides, resolver); len(urlFindings) > 0 {
					preConvertErr = iconFindingsToError(urlFindings)
					return preConvertErr
				}
			}

			// Resolve relative asset paths (icon.path, content image_value.path,
			// shape_grid cell image.path, slide background.image) against the JSON
			// input directory. This must happen before convertPresentationSlides so
			// that downstream specs see absolute paths.
			//
			// Errors abort with an aggregated message; non-blocking warnings (e.g.
			// ICON_FILL_IGNORED_ON_INLINE) flow into inputWarnings so the user sees
			// them in the success output instead of having generation refused.
			if jsonPath != "-" {
				// Resolve relative assets against the deck's own directory, absolutized so
				// the base dir satisfies the absolute-path contract that the MCP base_dir
				// resolver enforces and that resolveIconInputPath's symlink-escape check
				// assumes. A raw filepath.Dir(jsonPath) leaves the base dir relative (".")
				// when -json is itself relative (e.g. `generate -json deck.json`), which
				// makes a valid relative icon *file path* falsely trip
				// ICON_PATH_SYMLINK_ESCAPE. validate already absolutizes via
				// validateBaseDir; sharing it keeps the two CLI surfaces in lockstep.
				inputDir := validateBaseDir(jsonPath, "")
				assetFindings := resolveLocalAssetPaths(input.Slides, inputDir)
				if assetErr := iconFindingsToError(assetFindings); assetErr != nil {
					preConvertErr = assetErr
					return preConvertErr
				}
				for _, d := range assetFindings {
					if d.Severity != diagnostics.SeverityError {
						inputWarnings = append(inputWarnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
					}
				}
			}
			return nil
		},
	})
	defer renderCleanup()
	if renderErr != nil {
		// Preserve each historical error shape:
		//   - PreConvert (URL/asset) failures surface verbatim, as before.
		//   - strict_fit refusal dumps NDJSON to stderr then returns the error.
		//   - output-validation strict failure aggregates blocking findings.
		if preConvertErr != nil {
			return writeJSONError(jsonOutputPath, preConvertErr)
		}
		switch e := renderErr.(type) {
		case *StrictFitRefusal:
			enc := json.NewEncoder(os.Stderr)
			for _, f := range e.Findings {
				_ = enc.Encode(f)
			}
			return writeJSONError(jsonOutputPath, e.Err)
		case *OutputValidationFailure:
			blocking := e.Report.Blocking()
			msgs := make([]string, 0, len(blocking))
			for _, f := range blocking {
				msgs = append(msgs, f.Error())
			}
			return writeJSONError(jsonOutputPath, fmt.Errorf("output validation failed (strict): %s", strings.Join(msgs, "; ")))
		default:
			return writeJSONError(jsonOutputPath, renderErr)
		}
	}

	// Unpack render results into the local names the rest of runJSONMode uses.
	templateLayouts := runRes.TemplateLayouts
	syntheticFiles := runRes.SyntheticFiles
	synthesisFindings := runRes.SynthesisFindings
	strictFitFindings := runRes.StrictFitFindings
	slideSpecs := runRes.SlideSpecs
	gridVisualFindings := runRes.GridVisualFindings
	outputPath := runRes.OutputPath
	outputValidationFindings := runRes.OutputValidationFindings
	result := runRes.GenResult

	inputWarnings = append(inputWarnings, runRes.GridDiagWarnings...)
	inputWarnings = append(inputWarnings, runRes.ThemeOverrideWarnings...)
	// Pre-validate chart/diagram data structures via svggen Validate().
	// Issues are collected as warnings so generation still proceeds.
	inputWarnings = append(inputWarnings, validateSlidesChartData(input.Slides)...)
	chartDiagFindings := validateSlidesChartDiagnostics(input.Slides)

	// Constrained-mode diagram data colors (e.g. pyramid levels[].color) are
	// rendered with the template scheme and otherwise silently ignored. Surface
	// an advisory finding/warning so the author knows the custom colors were
	// dropped and that design_mode "free" is required to honor them.
	droppedColorFindings := collectDroppedDiagramColorWarnings(input)
	for _, f := range droppedColorFindings {
		inputWarnings = append(inputWarnings, f.Message)
	}

	// Merge input-layer warnings with generation warnings
	allWarnings := append(inputWarnings, result.Warnings...)

	// Convert structured media failures to per-slide error details
	slideErrors := convertMediaFailures(result.MediaFailures)

	// Build success output
	// Collect fit findings: synthesis + render-time + contrast.
	var allFitFindings []patterns.FitFinding
	if jsonOutputPath != "" {
		// Headless consumers need the same review-class preflight findings MCP
		// exposes with fit_report=true (substance, contrast predictions, chrome,
		// accessibility, and geometry), not only render-time warnings.
		allFitFindings = append(allFitFindings, collectFitFindings(input, templateLayouts,
			runRes.SlideWidth, runRes.SlideHeight, &runRes.TemplateTheme)...)
	} else {
		allFitFindings = append(allFitFindings, unresolvedGradientContrastFindings(input, templateLayouts, runRes.TemplateTheme.Colors)...)
	}
	allFitFindings = append(allFitFindings, synthesisFindings...)
	allFitFindings = append(allFitFindings, result.FitFindings...)
	allFitFindings = append(allFitFindings, contrastSwapsToFindings(result.ContrastSwaps, input, runRes.TemplateTheme.Colors)...)
	allFitFindings = append(allFitFindings, chartDiagFindings...)
	allFitFindings = append(allFitFindings, gridVisualFindings...)
	allFitFindings = append(allFitFindings, droppedColorFindings...)
	// Append strict_fit findings collected before generation so CLI JSON
	// consumers see warn-mode overflow diagnostics in the structured response.
	allFitFindings = append(allFitFindings, strictFitFindings...)
	// A pattern's own post-expand warnings (BODY_TOO_LONG on an over-budget
	// bio, CHART_PLACEHOLDER_EMPTY on a chart panel with no chart) are part of
	// what generate saw; without them the JSON report called a degraded deck
	// clean (go-slide-creator-wn4v).
	if jsonOutputPath == "" {
		// The full headless preflight above already includes these warnings.
		allFitFindings = append(allFitFindings, collectPatternPostExpandFindings(input, runRes.SlideWidth, runRes.SlideHeight, &runRes.TemplateTheme)...)
	}
	// An icon name that does not resolve drops the icon from its panel while
	// the siblings keep theirs; generation only logged it (go-slide-creator-puki).
	if jsonOutputPath == "" {
		allFitFindings = append(allFitFindings, collectDiagramIconFindings(input)...)
	}
	allFitFindings = dedupFitFindings(allFitFindings)

	// Build per-slide resolution summary
	slideResolutions := buildSlideResolutions(input.Slides, slideSpecs, templateLayouts, syntheticFiles,
		droppedPlaceholdersBySlide(allFitFindings))

	quality := computeQualityScoreWithLayouts(input.Slides, allWarnings, templateLayouts, allFitFindings...)
	evidence := &pipeline.QualityEvidence{ArtifactSHA256: result.ContentHash, SchemaValid: true, Generated: true, FitChecked: true, StructuralValid: !hasBlockingOutputFinding(outputValidationFindings), TotalSlides: result.SlideCount}
	evidence.Finalize()
	quality.Evidence = evidence
	output := JSONOutput{
		Success:                  renderSucceeded(allFitFindings, outputValidation),
		OutputPath:               outputPath,
		SlideCount:               result.SlideCount,
		ContentHash:              result.ContentHash,
		DurationMs:               time.Since(startTime).Milliseconds(),
		Warnings:                 allWarnings,
		SlideErrors:              slideErrors,
		Quality:                  quality,
		ValidationErrors:         result.ValidationErrors,
		FitFindings:              allFitFindings,
		Slides:                   slideResolutions,
		OutputValidationFindings: outputValidationFindings,
	}

	// Write JSON output if requested
	if jsonOutputPath != "" {
		return writeJSONOutput(jsonOutputPath, output)
	}

	// Otherwise print summary to stdout. Surface every collected warning so the
	// human-readable CLI path is not silent about partial-mode dropped slides and
	// other non-fatal issues that the JSON path already reports via the
	// "warnings" field. Without this, --partial could drop slides with no message
	// (e.g. 44 inputs -> 41 outputs and nothing explaining the 3 that vanished).
	for _, w := range allWarnings {
		slog.Warn(w)
	}

	slog.Info("JSON conversion complete",
		"output", outputPath,
		"slides", result.SlideCount,
		"duration_ms", output.DurationMs,
	)

	return nil
}

// convertJSONSlides converts JSON slide definitions to generator specs.
// Deprecated: Use convertPresentationSlides for typed field support.
func convertJSONSlides(jsonSlides []JSONSlide) ([]generator.SlideSpec, error) {
	specs := make([]generator.SlideSpec, 0, len(jsonSlides))

	for i, jsonSlide := range jsonSlides {
		if jsonSlide.LayoutID == "" {
			return nil, fmt.Errorf("slide %d: layout_id is required", i+1)
		}

		slideType := inferJSONSlideType(jsonSlide)
		contentItems, err := convertJSONContent(jsonSlide.Content, i+1, slideType)
		if err != nil {
			return nil, err
		}

		specs = append(specs, generator.SlideSpec{
			LayoutID:        jsonSlide.LayoutID,
			Content:         contentItems,
			SpeakerNotes:    jsonSlide.SpeakerNotes,
			SourceNote:      jsonSlide.Source,
			SourceLink:      toGeneratorLink(jsonSlide.SourceLink),
			Takeaway:        jsonSlide.Takeaway,
			Transition:      jsonSlide.Transition,
			TransitionSpeed: jsonSlide.TransitionSpeed,
			Build:           jsonSlide.Build,
			ContrastCheck:   jsonSlide.ContrastCheck,
		})
	}

	return specs, nil
}

// convertPresentationSlides converts typed SlideInput definitions to generator specs.
// This is the primary conversion path that supports both typed fields (text_value,
// bullets_value, etc.) and legacy json.RawMessage Value field.
// When layouts is non-empty and a slide omits layout_id, auto-layout selection is used.
func convertPresentationSlides(slides []SlideInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, metadata *types.TemplateMetadata, rhythmGrid *resolvedGrid, accentStrategy patterns.AccentStrategy, diagCtx *GridDiagramContext, partial bool) ([]generator.SlideSpec, []string, []patterns.FitFinding, error) { //nolint:gocognit,gocyclo
	specs := make([]generator.SlideSpec, 0, len(slides))
	var gridWarnings []string
	var gridFitFindings []patterns.FitFinding

	// Track layout usage for variety scoring during auto-selection
	var usedLayouts map[string]int
	if len(layouts) > 0 {
		usedLayouts = make(map[string]int)
	}

	// Compute the formatted section number for each section-divider slide.
	// The number is injected into the correct placeholder per-slide, after
	// layout resolution, so it lands in the decorative section_number frame when
	// the layout has one and falls back to the body/tagline slot otherwise
	// (see convertSinglePresentationSlide / injectSectionNumber).
	sectionNumbers := make([]string, len(slides))
	sectionNum := 0
	for i := range slides {
		if isSectionSlideInput(slides[i], layouts) {
			sectionNum++
			sectionNumbers[i] = fmt.Sprintf("%02d", sectionNum)
		}
	}

	// Build per-slide section index for accent-strategy=section-keyed.
	// Section indices start at 0 and increment each time a "section" slide is seen.
	// Slides before the first section divider get index 0.
	sectionIndices := slideSectionIndices(slides, layouts)

	for i, slide := range slides {
		spec, slideWarnings, slideFindings, err := convertSinglePresentationSlide(
			slide, i, slides, specs, layouts, metadata,
			slideWidth, slideHeight, rhythmGrid, accentStrategy,
			sectionIndices, usedLayouts, diagCtx, sectionNumbers[i],
		)
		gridWarnings = append(gridWarnings, slideWarnings...)
		gridFitFindings = append(gridFitFindings, slideFindings...)
		if err != nil {
			if !partial {
				return nil, nil, nil, err
			}
			// Partial mode: skip the failing slide and record a warning plus a
			// machine-actionable CONTENT_DROPPED finding so the dropped slide is
			// a repairable signal, not just a human-readable string.
			gridWarnings = append(gridWarnings, fmt.Sprintf("slide %d: skipped (partial mode): %v", i+1, err))
			gridFitFindings = append(gridFitFindings, patterns.ContentDropped(
				slidepath.Slide(i),
				fmt.Sprintf("slide %d", i+1),
				fmt.Sprintf("skipped in partial mode: %v", err),
			))
			continue
		}
		specs = append(specs, spec)
	}

	return specs, gridWarnings, gridFitFindings, nil
}

// slideSectionIndices is shared by rendering and preflight pattern expansion;
// section-keyed accents must not depend on which path asks for the slide.
func slideSectionIndices(slides []SlideInput, layouts []types.LayoutMetadata) []int {
	indices := make([]int, len(slides))
	section := 0
	for i := range slides {
		if i > 0 && isSectionSlideInput(slides[i], layouts) {
			section++
		}
		indices[i] = section
	}
	return indices
}

// convertSinglePresentationSlide converts a single slide input into a generator SlideSpec.
// Returns the spec, any warnings, any structured fit findings, and an error if the slide cannot be converted.
func convertSinglePresentationSlide( //nolint:gocognit,gocyclo
	slide SlideInput,
	i int,
	allSlides []SlideInput,
	existingSpecs []generator.SlideSpec,
	layouts []types.LayoutMetadata,
	metadata *types.TemplateMetadata,
	slideWidth, slideHeight int64,
	rhythmGrid *resolvedGrid,
	accentStrategy patterns.AccentStrategy,
	sectionIndices []int,
	usedLayouts map[string]int,
	diagCtx *GridDiagramContext,
	sectionNumber string,
) (generator.SlideSpec, []string, []patterns.FitFinding, error) {
	var warnings []string
	var slideFitFindings []patterns.FitFinding
	hasComposition := hasPatternContent(slide)
	explicitLayout := slide.LayoutID != ""
	slide.ColumnLeftPercent = derivedColumnLeftPercent(slide)

	if slide.LayoutID != "" && len(layouts) > 0 {
		if resolved, ok := layout.ResolveCanonicalLayoutID(slide.LayoutID, layouts); ok {
			slide.LayoutID = resolved
		}
	}

	// Visual compositions need a title-bearing canvas with the remainder of the
	// slide available for shapes. Bind that role before heuristic scoring so
	// variety and misleading layout names cannot select a section or closing
	// layout. Templates without a canonical blank-title layout fall back to the
	// regular compatibility-scored diagram path.
	if hasComposition && slide.LayoutID == "" && len(layouts) > 0 {
		if resolved, ok := layout.ResolveCanonicalLayoutID("blank-title", layouts); ok {
			slide.LayoutID = resolved
		}
	}

	// An explicit override remains authoritative when it is compatible. Reject
	// unsafe overrides using the same structural gate as auto-selection; shape
	// grids also accept the canonical blank-title canvas, which deliberately has
	// no body placeholder because its free area is derived from title/footer
	// geometry.
	if hasComposition && explicitLayout && len(layouts) > 0 {
		if selected := findLayoutMetadataByID(layouts, slide.LayoutID); selected != nil &&
			!isCompositionLayoutCompatible(*selected) {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: layout %q is incompatible with pattern/compose/shape_grid content", i+1, slide.LayoutID)
		}
	}

	if slide.LayoutID == "" {
		if len(layouts) == 0 {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: layout_id is required (no template layouts available for auto-selection)", i+1)
		}

		// Auto-select layout using heuristic engine
		slideDef := jsonSlideToDefinition(slide)
		req := layout.SelectionRequest{
			Slide:   slideDef,
			Layouts: layouts,
			Context: layout.SelectionContext{
				Position:    i,
				TotalSlides: len(allSlides),
				UsedLayouts: usedLayouts,
			},
		}
		if i > 0 && len(existingSpecs) > 0 {
			req.Context.PreviousType = existingSpecs[len(existingSpecs)-1].LayoutID
		}

		resolvedID, confidence, fallbackWarning, selErr := resolveAutoLayout(req, slide, hasComposition, layouts, i)
		if selErr != nil {
			return generator.SlideSpec{}, nil, nil, selErr
		}
		if fallbackWarning != "" {
			warnings = append(warnings, fallbackWarning)
		}

		slide.LayoutID = resolvedID
		usedLayouts[resolvedID]++

		slog.Info("auto-layout selected",
			slog.Int("slide", i+1),
			slog.String("layout_id", resolvedID),
			slog.String("slide_type", string(slideDef.Type)),
			slog.Float64("confidence", confidence),
		)

		// Auto-map placeholder IDs for items that don't have one
		var selectedLayout *types.LayoutMetadata
		for j := range layouts {
			if layouts[j].ID == resolvedID {
				selectedLayout = &layouts[j]
				break
			}
		}
		// Route the auto-assigned section number into the resolved layout's
		// section_number frame (or body fallback) before virtual IDs are mapped.
		slide.Content = injectSectionNumber(slide.Content, selectedLayout, sectionNumber)
		if selectedLayout != nil {
			slide.Content = autoMapPlaceholders(slide.Content, *selectedLayout)
		}
	} else {
		if usedLayouts != nil {
			// Track explicitly-specified layouts too, for variety scoring
			usedLayouts[slide.LayoutID]++
		}
		var selectedLayout *types.LayoutMetadata
		for j := range layouts {
			if layouts[j].ID == slide.LayoutID {
				selectedLayout = &layouts[j]
				break
			}
		}
		// Route the auto-assigned section number into the resolved layout's
		// section_number frame (or body fallback) before virtual IDs are mapped.
		slide.Content = injectSectionNumber(slide.Content, selectedLayout, sectionNumber)
		// Resolve virtual placeholder IDs even for explicit layout IDs
		if selectedLayout != nil && hasVirtualPlaceholders(slide.Content) {
			slide.Content = autoMapPlaceholders(slide.Content, *selectedLayout)
		}
	}

	contentItems, err := convertPresentationContent(slide.Content, i+1, inferSlideType(slide, layouts...))
	if err != nil {
		return generator.SlideSpec{}, nil, nil, err
	}

	spec := generator.SlideSpec{
		LayoutID:          slide.LayoutID,
		ColumnLeftPercent: slide.ColumnLeftPercent,
		Content:           contentItems,
		Eyebrow:           slide.Eyebrow,
		SpeakerNotes:      slide.SpeakerNotes,
		SourceNote:        slide.Source,
		SourceLink:        toGeneratorLink(slide.SourceLink),
		Takeaway:          slide.Takeaway,
		Transition:        slide.Transition,
		TransitionSpeed:   slide.TransitionSpeed,
		Build:             slide.Build,
		ContrastCheck:     slide.ContrastCheck,
	}
	if finding, ok := derivedColumnLayoutFinding(slide, i); ok {
		slideFitFindings = append(slideFitFindings, finding)
	}
	if strings.TrimSpace(slide.Headline) != "" && isBlankCanvasLayout(slide.LayoutID, layouts) {
		headlineXML, headlineErr := generateCanvasHeadline(slide.Headline, canvasHeadlineBounds(slideWidth, slideHeight), diagCtx)
		if headlineErr != nil {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: headline: %w", i+1, headlineErr)
		}
		spec.OverlayShapeXML = append(spec.OverlayShapeXML, headlineXML)
	}

	// Convert the background spec: an image, a solid colour, or both
	// (go-slide-creator-uy5s).
	if bg := slide.Background; bg != nil && (bg.Image != "" || bg.Color != "") {
		spec.Background = &generator.BackgroundImage{
			Path:  bg.Image,
			Fit:   bg.Fit,
			Color: bg.Color,
		}
		if bg.Overlay != nil {
			spec.Background.Overlay = &generator.BackgroundOverlay{
				Color: bg.Overlay.Color,
				Alpha: bg.Overlay.Alpha,
			}
		}
	}

	// XOR enforcement: pattern, compose, and shape_grid are mutually exclusive (D1)
	{
		setCount := 0
		if slide.Pattern != nil {
			setCount++
		}
		if slide.Compose != nil {
			setCount++
		}
		if slide.ShapeGrid != nil {
			setCount++
		}
		if setCount > 1 {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: only one of pattern, compose, or shape_grid may be set", i+1)
		}
	}

	// Resolve the content rectangle before expanding patterns. Content-sized
	// patterns must size themselves against the same rectangle the grid renderer
	// will use, including the rhythm grid and reserved takeaway/source band.
	geom, contentBounds := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythmGrid)
	patternBounds := patterns.LayoutBounds{X: contentBounds.X, Y: contentBounds.Y, Width: contentBounds.CX, Height: contentBounds.CY}

	// Expand compose envelope into shape_grid before downstream processing
	if slide.Compose != nil {
		ctx := patterns.ExpandContext{
			Metadata:       metadata,
			ContentZone:    geom.Zone,
			SlideWidth:     slideWidth,
			SlideHeight:    slideHeight,
			LayoutBounds:   patternBounds,
			AccentStrategy: accentStrategy,
			SlideIndex:     i,
			SectionIndex:   sectionIndices[i],
			Theme:          patternThemeFromDiag(diagCtx),
		}
		expanded, composeWarnings, err := expandCompose(slide.Compose, ctx, patterns.Default())
		if err != nil {
			return generator.SlideSpec{}, nil, nil, newSlidePatternError(i, "compose", "compose", err)
		}
		for _, w := range composeWarnings {
			slog.Warn("compose warning",
				slog.Int("slide", i+1),
				slog.String("message", w))
			if f := composeWarningAsFinding(i, w); f != nil {
				slideFitFindings = append(slideFitFindings, *f)
			}
		}
		slide.ShapeGrid = expanded
	}

	// Expand pattern into shape_grid before downstream processing
	if slide.Pattern != nil {
		ctx := patterns.ExpandContext{
			Metadata:       metadata,
			ContentZone:    geom.Zone,
			SlideWidth:     slideWidth,
			SlideHeight:    slideHeight,
			LayoutBounds:   patternBounds,
			AccentStrategy: accentStrategy,
			SlideIndex:     i,
			SectionIndex:   sectionIndices[i],
			Theme:          patternThemeFromDiag(diagCtx),
		}
		expanded, patternWarnings, err := expandPattern(slide.Pattern, ctx, patterns.Default())
		if err != nil {
			return generator.SlideSpec{}, nil, nil, newSlidePatternError(i, "pattern", "pattern", err)
		}
		for _, warning := range patternWarnings {
			if f := patternWarningAsFinding(i, slide.Pattern.Name, warning); f != nil {
				slideFitFindings = append(slideFitFindings, *f)
			}
		}
		slide.ShapeGrid = expanded
	}

	// Expand any cell-level nested patterns into nested grids. This runs
	// after slide-level Pattern/Compose expansion so it covers every grid
	// path. The same ExpandContext (accent strategy, slide/section indices)
	// is reused, so nested patterns inherit accent rotation from the deck.
	if slide.ShapeGrid != nil {
		nestedCtx := patterns.ExpandContext{
			Metadata:       metadata,
			ContentZone:    geom.Zone,
			SlideWidth:     slideWidth,
			SlideHeight:    slideHeight,
			LayoutBounds:   patternBounds,
			AccentStrategy: accentStrategy,
			SlideIndex:     i,
			SectionIndex:   sectionIndices[i],
			Theme:          patternThemeFromDiag(diagCtx),
		}
		if err := expandNestedCellPatternsInBounds(slide.ShapeGrid, nestedCtx, contentBounds, patterns.Default()); err != nil {
			return generator.SlideSpec{}, nil, nil, newSlidePatternError(i, "shape_grid", "nested pattern", err)
		}
	}

	// Resolve shape_grid into raw p:sp XML fragments
	if slide.ShapeGrid != nil {
		// Virtual layout resolution: derive layout and bounds from template
		overrideBounds := geom.OverrideBounds
		contentZone := geom.Zone

		// Derive the ContentZone so shape_grid bounds respect the real title
		// height and footer clearance. resolveGridGeometry is the shared
		// contract used by preflight and preview, so the geometry validated
		// before render matches what generation lays down: a slide with a
		// concrete (explicit or auto-selected) layout takes its zone from THAT
		// layout's placeholders, while blank/virtual slides go through virtual
		// resolution, which also drives the layout pick and override bounds
		// (go-slide-creator-j15r, go-slide-creator-s1rd).
		if geom.VirtualUsed {
			spec.LayoutID = geom.LayoutID
			slog.Info("virtual layout resolved",
				slog.Int("slide", i+1),
				slog.String("layout_id", geom.LayoutID),
			)
		}

		// Use a high-start allocator to avoid colliding with template shape IDs.
		alloc := &pptx.ShapeIDAllocator{}
		alloc.SetMinID(200)
		// Build per-slide diagram context with the correct slide number.
		var slideDiagCtx *GridDiagramContext
		if diagCtx != nil {
			slideDiagCtx = &GridDiagramContext{
				ThemeColors: diagCtx.ThemeColors,
				DataPalette: diagCtx.DataPalette,
				FontFamily:  diagCtx.FontFamily,
				TitleFont:   diagCtx.TitleFont,
				SlideNum:    i + 1,
			}
		}
		gridResult, err := resolveShapeGrid(slide.ShapeGrid, alloc, overrideBounds, contentZone, slideWidth, slideHeight, slideDiagCtx)
		if err != nil {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: shape_grid: %w", i+1, err)
		}
		if gridResult != nil {
			spec.RawShapeXML = gridResult.Shapes
			for _, cell := range gridResult.Cells {
				if cell.ShapeSpec != nil && cell.ShapeSpec.Link != nil {
					spec.ShapeLinks = append(spec.ShapeLinks, generator.ShapeLink{
						Marker: fmt.Sprintf("json2pptx_shape_link_%d", cell.ID),
						Link:   generator.LinkSpec{URL: cell.ShapeSpec.Link.URL, Slide: cell.ShapeSpec.Link.Slide},
					})
				}
			}
			spec.IconInserts = gridResult.IconInserts
			spec.ImageInserts = gridResult.ImageInserts
			warnings = append(warnings, gridResult.Warnings...)
			slideFitFindings = append(slideFitFindings, gridResult.FitFindings...)
		}

		// Slide-level overlays (arrows, lines, badges) render on top of the
		// grid. They may reference grid cells by (row, col) via anchor_cell.
		if len(slide.Overlays) > 0 {
			overlayAlloc := &pptx.ShapeIDAllocator{}
			overlayAlloc.SetMinID(400)
			var cells []shapegrid.ResolvedCell
			if gridResult != nil {
				cells = gridResult.Cells
			}
			overlayShapes, err := resolveOverlays(slide.Overlays, cells, overlayAlloc, slideWidth, slideHeight, overlayThemeColors(slideDiagCtx))
			if err != nil {
				return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: overlays: %w", i+1, err)
			}
			spec.OverlayShapeXML = append(spec.OverlayShapeXML, overlayShapes...)
		}
	} else if len(slide.Overlays) > 0 {
		// Allow overlays on slides without a shape_grid (purely floating shapes).
		overlayAlloc := &pptx.ShapeIDAllocator{}
		overlayAlloc.SetMinID(400)
		overlayShapes, err := resolveOverlays(slide.Overlays, nil, overlayAlloc, slideWidth, slideHeight, overlayThemeColors(diagCtx))
		if err != nil {
			return generator.SlideSpec{}, nil, nil, fmt.Errorf("slide %d: overlays: %w", i+1, err)
		}
		spec.OverlayShapeXML = append(spec.OverlayShapeXML, overlayShapes...)
	}
	for idx, overlay := range slide.Overlays {
		if overlay != nil && overlay.Link != nil {
			spec.ShapeLinks = append(spec.ShapeLinks, generator.ShapeLink{
				Marker: fmt.Sprintf("json2pptx_overlay_link_%d", idx),
				Link:   generator.LinkSpec{URL: overlay.Link.URL, Slide: overlay.Link.Slide},
			})
		}
	}

	return spec, warnings, slideFitFindings, nil
}

// buildSlideResolutions constructs per-slide resolution metadata from the original
// input slides and the resolved generator specs. It reports which layout was chosen,
// which placeholders received content, whether auto-selection or synthesis was involved,
// and a rough occupancy estimate.
func buildSlideResolutions(
	inputSlides []SlideInput,
	specs []generator.SlideSpec,
	layouts []types.LayoutMetadata,
	syntheticFiles map[string][]byte,
	droppedPlaceholders map[int]map[string]bool,
) []SlideResolution {
	// Build layout lookup
	layoutByID := make(map[string]types.LayoutMetadata, len(layouts))
	for _, l := range layouts {
		layoutByID[l.ID] = l
	}

	// Build set of synthetic layout IDs from the file paths in the manifest.
	// Synthetic files have paths like "ppt/slideLayouts/slideLayout99.xml".
	syntheticIDs := make(map[string]bool, len(syntheticFiles))
	for path := range syntheticFiles {
		// Extract ID from path: "ppt/slideLayouts/slideLayout99.xml" -> "slideLayout99"
		base := filepath.Base(path)
		id := strings.TrimSuffix(base, filepath.Ext(base))
		syntheticIDs[id] = true
	}

	resolutions := make([]SlideResolution, 0, len(specs))
	for i, spec := range specs {
		sr := SlideResolution{
			Index:            i,
			ResolvedLayoutID: spec.LayoutID,
		}

		// Determine if layout was auto-selected (input had empty layout_id)
		if i < len(inputSlides) && inputSlides[i].LayoutID == "" {
			sr.WasAutoSelected = true
		}

		// Check if synthesized layout
		if syntheticIDs[spec.LayoutID] {
			sr.WasSynthesized = true
		}

		// Collect placeholders that received content. A placeholder the
		// resolved layout does not declare never received anything — its
		// content was dropped — so it is reported under placeholders_dropped
		// instead of being claimed as used (go-slide-creator-lhq6).
		dropped := droppedPlaceholders[i]
		seen := make(map[string]bool)
		for _, ci := range spec.Content {
			if seen[ci.PlaceholderID] {
				continue
			}
			seen[ci.PlaceholderID] = true
			if dropped[ci.PlaceholderID] {
				sr.PlaceholdersDropped = append(sr.PlaceholdersDropped, ci.PlaceholderID)
				continue
			}
			sr.PlaceholdersUsed = append(sr.PlaceholdersUsed, ci.PlaceholderID)
		}

		// Compute occupancy: placeholders used / total placeholders in layout
		if lm, ok := layoutByID[spec.LayoutID]; ok && len(lm.Placeholders) > 0 {
			sr.OccupancyPct = len(sr.PlaceholdersUsed) * 100 / len(lm.Placeholders)
			if sr.OccupancyPct > 100 {
				sr.OccupancyPct = 100
			}
		}

		resolutions = append(resolutions, sr)
	}

	return resolutions
}

// convertPresentationContent converts typed ContentInput items to generator content items.
// Uses ContentInput.ResolveValue() to support both typed fields and legacy Value.
func convertPresentationContent(content []ContentInput, slideNum int, slideType types.SlideType) ([]generator.ContentItem, error) { //nolint:gocognit,gocyclo
	items := make([]generator.ContentItem, 0, len(content))

	for j, ci := range content {
		if ci.PlaceholderID == "" {
			return nil, fmt.Errorf("slide %d, content %d: placeholder_id is required", slideNum, j+1)
		}
		if ci.Type == "" {
			return nil, fmt.Errorf("slide %d, content %d: type is required", slideNum, j+1)
		}

		item := generator.ContentItem{
			PlaceholderID: ci.PlaceholderID,
			Link:          toGeneratorLink(ci.Link),
		}

		// Apply font size override (convert points to hundredths of a point).
		if ci.FontSize != nil && *ci.FontSize > 0 {
			item.FontSize = int(*ci.FontSize * 100)
		}

		resolved, err := ci.ResolveValue()
		if err != nil {
			return nil, fmt.Errorf("slide %d, content %d: %w", slideNum, j+1, err)
		}

		switch ci.Type {
		case "text":
			// Section divider titles use ContentSectionTitle so the generator
			// preserves the template's large font size instead of capping it.
			// Check both the original virtual ID "title" and section slide type:
			// autoMapPlaceholders may have already resolved "title" to the actual
			// placeholder index (e.g., "13"), so we also accept any text item on
			// a section slide — section dividers only contain title text.
			//
			// Title slide titles use ContentTitleSlideTitle to preserve the
			// template's ctrTitle font size (typically 40-60pt) and centered
			// alignment instead of capping to 24pt body-text size.
			//
			// Section number aliases (section_number, section_no, large_number)
			// use ContentSectionTitle to preserve the template's large decorative
			// font size regardless of slide type.
			if slideType == types.SlideTypeSection {
				item.Type = generator.ContentSectionTitle
			} else if placeholderrole.IsSectionNumberAlias(ci.PlaceholderID) {
				item.Type = generator.ContentSectionTitle
			} else if slideType == types.SlideTypeTitle && (isTitlePlaceholderID(ci.PlaceholderID) || isLikelySubtitle(ci.PlaceholderID)) {
				item.Type = generator.ContentTitleSlideTitle
			} else {
				item.Type = generator.ContentText
			}
			text, ok := resolved.(string)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: text resolved to %T, want string", slideNum, j+1, resolved)
			}
			item.Value = text

		case "bullets":
			item.Type = generator.ContentBullets
			bullets, ok := resolved.([]string)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: bullets resolved to %T, want []string", slideNum, j+1, resolved)
			}
			item.Value = bullets

		case "body_and_bullets":
			item.Type = generator.ContentBodyAndBullets
			input, ok := resolved.(*BodyAndBulletsInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: body_and_bullets resolved to %T, want *BodyAndBulletsInput", slideNum, j+1, resolved)
			}
			item.Value = generator.BodyAndBulletsContent{
				Body:         input.Body,
				Bullets:      input.Bullets,
				TrailingBody: input.TrailingBody,
			}

		case "body_and_lead":
			item.Type = generator.ContentBodyAndLead
			input, ok := resolved.(*BodyAndLeadInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: body_and_lead resolved to %T, want *BodyAndLeadInput", slideNum, j+1, resolved)
			}
			item.Value = generator.BodyAndLeadContent{
				Lead:    input.Lead,
				Bullets: input.Bullets,
			}

		case "bullet_groups":
			item.Type = generator.ContentBulletGroups
			input, ok := resolved.(*BulletGroupsInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: bullet_groups resolved to %T, want *BulletGroupsInput", slideNum, j+1, resolved)
			}
			item.Value = convertBulletGroupsInput(input)

		case "table":
			item.Type = generator.ContentTable
			input, ok := resolved.(*TableInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: table resolved to %T, want *TableInput", slideNum, j+1, resolved)
			}
			item.Value = input.ToTableSpec()

		case "chart":
			item.Type = generator.ContentDiagram
			if resolved != nil {
				// Typed field path: ChartValue was set
				chart, ok := resolved.(*types.ChartSpec) //nolint:staticcheck // backward compatibility
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: chart resolved to %T, want *types.ChartSpec", slideNum, j+1, resolved)
				}
				item.Value = chart.ToDiagramSpec()
			} else {
				// Legacy path: parse from Value json.RawMessage
				var chart types.ChartSpec                                //nolint:staticcheck // backward compatibility
				if err := json.Unmarshal(ci.Value, &chart); err != nil { //nolint:staticcheck // backward compatibility
					return nil, fmt.Errorf("slide %d, content %d: invalid chart value: %w", slideNum, j+1, err)
				}
				if chart.Type == "" {
					return nil, fmt.Errorf("slide %d, content %d: chart type is required", slideNum, j+1)
				}
				item.Value = chart.ToDiagramSpec()
			}

		case "diagram":
			item.Type = generator.ContentDiagram
			if resolved != nil {
				// Typed field path: DiagramValue was set
				diagram, ok := resolved.(*types.DiagramSpec)
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: diagram resolved to %T, want *types.DiagramSpec", slideNum, j+1, resolved)
				}
				item.Value = diagram
			} else {
				// Legacy path: parse from Value json.RawMessage
				var diagram types.DiagramSpec
				if err := json.Unmarshal(ci.Value, &diagram); err != nil {
					return nil, fmt.Errorf("slide %d, content %d: invalid diagram value: %w", slideNum, j+1, err)
				}
				if diagram.Type == "" {
					return nil, fmt.Errorf("slide %d, content %d: diagram type is required", slideNum, j+1)
				}
				item.Value = &diagram
			}

		case "image":
			item.Type = generator.ContentImage
			if resolved != nil {
				// Typed field path: ImageValue was set
				img, ok := resolved.(*ImageInput)
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: image resolved to %T, want *ImageInput", slideNum, j+1, resolved)
				}
				if img.Path == "" {
					return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
				}
				item.Value = generator.ImageContent{
					Path: img.Path,
					Alt:  img.Alt,
				}
			} else {
				// Legacy path: parse from Value json.RawMessage
				var img struct {
					Path string `json:"path"`
					Alt  string `json:"alt"`
				}
				if err := json.Unmarshal(ci.Value, &img); err != nil {
					return nil, fmt.Errorf("slide %d, content %d: invalid image value: %w", slideNum, j+1, err)
				}
				if img.Path == "" {
					return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
				}
				item.Value = generator.ImageContent{
					Path: img.Path,
					Alt:  img.Alt,
				}
			}

		default:
			return nil, fmt.Errorf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)", slideNum, j+1, ci.Type)
		}

		items = append(items, item)
	}

	// Merge text items targeting the same placeholder (e.g., template_2 section
	// dividers where both "title" and "body" resolve to the same body placeholder).
	// Without merging, the second populateShapeText call overwrites the first.
	items = mergeTextItemsSamePlaceholder(items)

	return items, nil
}

// validateSlidesChartData runs svggen Validate() on chart/diagram content items
// across all slides, returning any validation issues as warning strings.
// This catches structural data problems (e.g., flat map for waterfall charts)
// before render time, surfacing them in the JSON output warnings array.
func validateSlidesChartData(slides []SlideInput) []string {
	var warnings []string
	for i, slide := range slides {
		for j, ci := range slide.Content {
			if ci.Type != "chart" && ci.Type != "diagram" {
				continue
			}
			warnings = append(warnings, validateContentDiagramData(ci, i+1, j+1)...)
		}
	}
	return warnings
}

// validateSlidesChartDiagnostics collects structured chart diagnostics from
// all chart/diagram content items across all slides. These are returned as
// FitFinding values suitable for inclusion in the fit_findings response array.
func validateSlidesChartDiagnostics(slides []SlideInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for i, slide := range slides {
		for j, ci := range slide.Content {
			if ci.Type != "chart" && ci.Type != "diagram" {
				continue
			}
			findings = append(findings, collectChartDiagnostics(ci, i, j)...)
		}
	}
	return findings
}

// collectChartDiagnostics extracts structured ChartDiagnostic values from a
// content item's resolved DiagramSpec and converts them to FitFindings.
func collectChartDiagnostics(ci ContentInput, slideIdx, contentIdx int) []patterns.FitFinding {
	resolved, err := ci.ResolveValue()
	if err != nil {
		return nil
	}

	var spec *types.DiagramSpec

	switch ci.Type {
	case "chart":
		if resolved != nil {
			chart, ok := resolved.(*types.ChartSpec) //nolint:staticcheck // backward compat
			if !ok {
				return nil
			}
			spec = chart.ToDiagramSpec()
		} else if len(ci.Value) > 0 {
			var chart types.ChartSpec //nolint:staticcheck // backward compat
			if err := json.Unmarshal(ci.Value, &chart); err != nil {
				return nil
			}
			spec = chart.ToDiagramSpec()
		}
	case "diagram":
		if resolved != nil {
			diagram, ok := resolved.(*types.DiagramSpec)
			if !ok {
				return nil
			}
			spec = diagram
		} else if len(ci.Value) > 0 {
			var diagram types.DiagramSpec
			if err := json.Unmarshal(ci.Value, &diagram); err != nil {
				return nil
			}
			spec = &diagram
		}
	}

	if spec == nil {
		return nil
	}

	var findings []patterns.FitFinding
	path := fmt.Sprintf("/slides/%d/content/%d", slideIdx, contentIdx)

	// Empty data → chart_data_empty finding.
	if len(spec.Data) == 0 {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    patterns.ErrCodeChartDataEmpty,
				Message: fmt.Sprintf("slide %d, content %d: %s data is empty; output will be blank", slideIdx+1, contentIdx+1, spec.Type),
				Fix: &patterns.FixSuggestion{
					Kind: "provide_data",
					Params: map[string]any{
						"chart_type": spec.Type,
					},
				},
			},
			Action: "refuse",
		})
	}

	// Convert ChartDiagnostics to FitFindings.
	for _, cd := range spec.ChartDiagnostics {
		f := patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    cd.Code,
				Message: fmt.Sprintf("slide %d, content %d: %s", slideIdx+1, contentIdx+1, cd.Message),
			},
			Action: "review",
		}
		// Attach fix suggestion based on diagnostic code.
		switch cd.Code {
		case patterns.ErrCodeChartValueCoerced:
			f.Fix = &patterns.FixSuggestion{
				Kind:   "provide_numeric_value",
				Params: cd.Details,
			}
		case patterns.ErrCodeChartShapeInferred:
			f.Fix = &patterns.FixSuggestion{
				Kind:   "provide_native_format",
				Params: cd.Details,
			}
		}
		if len(cd.Details) > 0 {
			f.ValidationError.Pattern = spec.Type
		}
		findings = append(findings, f)
	}

	return findings
}

// validateContentDiagramData resolves a chart/diagram ContentInput to a DiagramSpec
// and validates its data structure via the svggen registry. Returns warning
// strings for validation failures and flat-map auto-conversions.
func validateContentDiagramData(ci ContentInput, slideNum, contentNum int) []string {
	resolved, err := ci.ResolveValue()
	if err != nil {
		return nil // parse errors are caught elsewhere
	}

	var spec *types.DiagramSpec

	switch ci.Type {
	case "chart":
		if resolved != nil {
			chart, ok := resolved.(*types.ChartSpec) //nolint:staticcheck // backward compat
			if !ok {
				return nil
			}
			spec = chart.ToDiagramSpec()
		} else if len(ci.Value) > 0 {
			var chart types.ChartSpec //nolint:staticcheck // backward compat
			if err := json.Unmarshal(ci.Value, &chart); err != nil {
				return nil
			}
			spec = chart.ToDiagramSpec()
		}
	case "diagram":
		if resolved != nil {
			diagram, ok := resolved.(*types.DiagramSpec)
			if !ok {
				return nil
			}
			spec = diagram
		} else if len(ci.Value) > 0 {
			var diagram types.DiagramSpec
			if err := json.Unmarshal(ci.Value, &diagram); err != nil {
				return nil
			}
			spec = &diagram
		}
	}

	return validateDiagramSpecAll(spec, slideNum, contentNum)
}

// validateDiagramSpecAll checks a DiagramSpec for both flat-map conversion warnings
// and svggen data validation issues. Returns all warnings found.
func validateDiagramSpecAll(spec *types.DiagramSpec, slideNum, contentNum int) []string {
	if spec == nil || spec.Type == "" {
		return nil
	}

	var warnings []string

	// Collect flat-map auto-conversion warnings from buildChartData.
	for _, w := range spec.Warnings {
		warnings = append(warnings, fmt.Sprintf("slide %d, content %d: %s", slideNum, contentNum, w))
	}

	// Run svggen structural validation.
	if w := validateDiagramSpec(spec, slideNum, contentNum); w != "" {
		warnings = append(warnings, w)
	}

	return warnings
}

// validateDiagramSpec checks a DiagramSpec's data against the svggen registry's
// diagram-specific Validate() method. Returns a warning string or "".
func validateDiagramSpec(spec *types.DiagramSpec, slideNum, contentNum int) string {
	if spec == nil || spec.Type == "" {
		return ""
	}
	if spec.Type == "heatmap" {
		if scale, present := spec.Data["color_scale"]; present {
			if err := generator.ValidateHeatmapColorScale(scale); err != nil {
				return fmt.Sprintf("slide %d, content %d: heatmap data validation: %v", slideNum, contentNum, err)
			}
		}
	}

	req := &svggen.RequestEnvelope{
		Type: spec.Type,
		Data: spec.Data,
	}

	d := svggen.DefaultRegistry().Get(req.Type)
	if d == nil {
		return "" // unknown types are caught elsewhere
	}

	if err := d.Validate(req); err != nil {
		return fmt.Sprintf("slide %d, content %d: %s data validation: %v",
			slideNum, contentNum, spec.Type, err)
	}
	return ""
}

// mergeTextItemsSamePlaceholder combines text/section-title items that target
// the same placeholder ID. The first item's text becomes "first\nsecond".
// Non-text items and items with unique placeholder IDs are left unchanged.
func mergeTextItemsSamePlaceholder(items []generator.ContentItem) []generator.ContentItem {
	if len(items) <= 1 {
		return items
	}

	// Find placeholder IDs that appear more than once for text items
	seen := make(map[string]int) // placeholder_id -> index of first text item
	for i, item := range items {
		if item.Type != generator.ContentText && item.Type != generator.ContentSectionTitle {
			continue
		}
		if _, ok := seen[item.PlaceholderID]; !ok {
			seen[item.PlaceholderID] = i
		}
	}

	// Merge duplicates
	merged := make([]generator.ContentItem, 0, len(items))
	skip := make(map[int]bool)
	for i, item := range items {
		if skip[i] {
			continue
		}
		if item.Type != generator.ContentText && item.Type != generator.ContentSectionTitle {
			merged = append(merged, item)
			continue
		}
		firstIdx := seen[item.PlaceholderID]
		if firstIdx != i {
			// This is a duplicate — merge into the first occurrence
			continue
		}
		// Collect all text for this placeholder
		text, _ := item.Value.(string)
		for j := i + 1; j < len(items); j++ {
			if items[j].PlaceholderID == item.PlaceholderID &&
				(items[j].Type == generator.ContentText || items[j].Type == generator.ContentSectionTitle) {
				if addText, ok := items[j].Value.(string); ok && addText != "" {
					text += "\n" + addText
				}
				skip[j] = true
			}
		}
		item.Value = text
		merged = append(merged, item)
	}
	return merged
}

// convertJSONContent converts JSON content items to generator content items.
func convertJSONContent(jsonContent []JSONContentItem, slideNum int, slideType types.SlideType) ([]generator.ContentItem, error) { //nolint:gocognit,gocyclo
	items := make([]generator.ContentItem, 0, len(jsonContent))

	for j, jsonItem := range jsonContent {
		if jsonItem.PlaceholderID == "" {
			return nil, fmt.Errorf("slide %d, content %d: placeholder_id is required", slideNum, j+1)
		}
		if jsonItem.Type == "" {
			return nil, fmt.Errorf("slide %d, content %d: type is required", slideNum, j+1)
		}

		item := generator.ContentItem{
			PlaceholderID: jsonItem.PlaceholderID,
			Link:          toGeneratorLink(jsonItem.Link),
		}

		// Apply font size override (convert points to hundredths of a point).
		if jsonItem.FontSize != nil && *jsonItem.FontSize > 0 {
			item.FontSize = int(*jsonItem.FontSize * 100)
		}

		switch jsonItem.Type {
		case "text":
			// Title slide titles use ContentTitleSlideTitle to preserve the
			// template's ctrTitle font size (typically 40-60pt) and centered
			// alignment instead of capping to 24pt body-text size.
			// Section number aliases preserve the template's large decorative font.
			if placeholderrole.IsSectionNumberAlias(jsonItem.PlaceholderID) {
				item.Type = generator.ContentSectionTitle
			} else if slideType == types.SlideTypeTitle && (isTitlePlaceholderID(jsonItem.PlaceholderID) || isLikelySubtitle(jsonItem.PlaceholderID)) {
				item.Type = generator.ContentTitleSlideTitle
			} else {
				item.Type = generator.ContentText
			}
			var text string
			if err := json.Unmarshal(jsonItem.Value, &text); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid text value: %w", slideNum, j+1, err)
			}
			item.Value = text

		case "bullets":
			item.Type = generator.ContentBullets
			var bullets []string
			if err := json.Unmarshal(jsonItem.Value, &bullets); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid bullets value (expected array of strings): %w", slideNum, j+1, err)
			}
			item.Value = bullets

		case "image":
			item.Type = generator.ContentImage
			var img struct {
				Path string `json:"path"`
				Alt  string `json:"alt"`
			}
			if err := json.Unmarshal(jsonItem.Value, &img); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid image value: %w", slideNum, j+1, err)
			}
			if img.Path == "" {
				return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
			}
			item.Value = generator.ImageContent{
				Path: img.Path,
				Alt:  img.Alt,
			}

		case "chart":
			item.Type = generator.ContentDiagram
			var chart types.ChartSpec //nolint:staticcheck // backward compat
			if err := json.Unmarshal(jsonItem.Value, &chart); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid chart value: %w", slideNum, j+1, err)
			}
			if chart.Type == "" {
				return nil, fmt.Errorf("slide %d, content %d: chart type is required", slideNum, j+1)
			}
			// Convert ChartSpec to DiagramSpec at the API boundary
			item.Value = chart.ToDiagramSpec()

		case "diagram":
			// Diagram content type accepts DiagramSpec directly with map[string]any data.
			// Use this for complex diagram types (swot, org_chart, timeline, etc.)
			// that need structured data passed directly as map[string]any.
			item.Type = generator.ContentDiagram
			var diagram types.DiagramSpec
			if err := json.Unmarshal(jsonItem.Value, &diagram); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid diagram value: %w", slideNum, j+1, err)
			}
			if diagram.Type == "" {
				return nil, fmt.Errorf("slide %d, content %d: diagram type is required", slideNum, j+1)
			}
			item.Value = &diagram

		case "table":
			item.Type = generator.ContentTable
			var tableInput TableInput
			if err := json.Unmarshal(jsonItem.Value, &tableInput); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid table value: %w", slideNum, j+1, err)
			}
			item.Value = tableInput.ToTableSpec()

		case "body_and_bullets":
			item.Type = generator.ContentBodyAndBullets
			var input BodyAndBulletsInput
			if err := json.Unmarshal(jsonItem.Value, &input); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid body_and_bullets value: %w", slideNum, j+1, err)
			}
			item.Value = generator.BodyAndBulletsContent{
				Body:         input.Body,
				Bullets:      input.Bullets,
				TrailingBody: input.TrailingBody,
			}

		case "bullet_groups":
			item.Type = generator.ContentBulletGroups
			var input BulletGroupsInput
			if err := json.Unmarshal(jsonItem.Value, &input); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid bullet_groups value: %w", slideNum, j+1, err)
			}
			item.Value = convertBulletGroupsInput(&input)

		default:
			return nil, fmt.Errorf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)", slideNum, j+1, jsonItem.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

// convertBulletGroupsInput converts a BulletGroupsInput to generator.BulletGroupsContent.
func convertBulletGroupsInput(input *BulletGroupsInput) generator.BulletGroupsContent {
	groups := make([]generator.BulletGroup, len(input.Groups))
	for i, g := range input.Groups {
		groups[i] = generator.BulletGroup{
			Header:     g.Header,
			Body:       g.Body,
			Bullets:    g.Bullets,
			GroupLabel: g.GroupLabel,
		}
	}
	return generator.BulletGroupsContent{
		Body:         input.Body,
		Groups:       groups,
		TrailingBody: input.TrailingBody,
	}
}

// writeJSONError writes an error response to JSON output or returns the error.
func writeJSONError(jsonOutputPath string, err error) error {
	if jsonOutputPath == "" {
		return err
	}

	output := JSONOutput{
		Success: false,
		Error:   err.Error(),
	}

	if writeErr := writeJSONOutput(jsonOutputPath, output); writeErr != nil {
		return fmt.Errorf("%v (also failed to write JSON output: %v)", err, writeErr)
	}

	return err
}

// computeQualityScore evaluates the quality of a set of slides and
// returns a QualityScore. It can be used by both JSON mode (json_mode.go)
// and markdown mode (generate.go) when --json-output is specified.
// Accepts []SlideInput (typed schema) for full typed + legacy field support.
func computeQualityScore(slides []SlideInput, warnings []string) *QualityScore {
	return computeQualityScoreWithLayouts(slides, warnings, nil)
}

// computeQualityScoreWithLayouts is computeQualityScore with the template
// layouts available: titles are then judged by measured fit against their
// resolved title placeholder (go-slide-creator-vjwn) instead of the 60-char
// heuristic, which remains the fallback when a title cannot be measured.
//
// The layout is the one the deck will ACTUALLY use — the author's layout_id
// when set, otherwise the heuristic selector's choice. Measuring only explicit
// layout_ids meant the semantic path, which never sets one, always fell back to
// the character heuristic (go-slide-creator-t64e).
func computeQualityScoreWithLayouts(slides []SlideInput, warnings []string, layouts []types.LayoutMetadata, findings ...patterns.FitFinding) *QualityScore { //nolint:gocognit,gocyclo
	if len(slides) == 0 {
		return &QualityScore{
			Score:  0.0,
			Basis:  scoreBasisInput,
			Scope:  "input_heuristic",
			Issues: []string{"no slides in presentation"},
		}
	}

	const (
		maxBullets     = 8
		maxSubtitleLen = 120
		maxContent     = 6
	)

	var slideScores []SlideQuality
	var globalIssues []string
	totalScore := 0.0

	predictedLayouts := predictSlideLayouts(&PresentationInput{Slides: slides}, layouts)

	for i, slide := range slides {
		slideScore := 1.0
		var issues []string
		if slideCarriesArgument(slide, layouts...) && !hasNonTextContent(slide) {
			if words := slideBodyWordCount(slide); words < minSlideWords {
				slideScore -= 0.2
				issues = append(issues, fmt.Sprintf("content slide has %d body words (minimum %d)", words, minSlideWords))
			}
		}

		// Check for empty slide (no content items AND no shape_grid/pattern)
		if len(slide.Content) == 0 && slide.ShapeGrid == nil && slide.Pattern == nil && slide.Compose == nil {
			slideScore -= 0.5
			issues = append(issues, "empty slide with no content")
		}

		// Analyze each content item using ResolveValue for typed + legacy support.
		// Bullets are counted per placeholder: a two-column slide with 5+5
		// bullets is two readable lists, not one list of 10.
		bulletsByPlaceholder := map[string]int{}
		contentCount := len(slide.Content)
		for _, item := range slide.Content {
			resolved, _ := item.ResolveValue()
			switch item.Type {
			case "text":
				// Check title/subtitle length — inspect the placeholder ID to classify
				if isLikelySubtitle(item.PlaceholderID) {
					if text, ok := resolved.(string); ok && len(text) > maxSubtitleLen {
						penalty := float64(len(text)-maxSubtitleLen) / 100.0
						if penalty > 0.3 {
							penalty = 0.3
						}
						slideScore -= penalty
						issues = append(issues, fmt.Sprintf("subtitle too long (%d chars, max %d)", len(text), maxSubtitleLen))
					}
				} else if isLikelyTitle(item.PlaceholderID) {
					// Measured fit is the only title rule. The 60-character
					// fallback that used to sit here fired whenever measurement
					// was unavailable, so one deck reported "title too long
					// (106 chars, max 60)" here, "headline is 13 words; trim to
					// 12" from the content lint, and a max_chars of 29 in the
					// placeholder metadata — three numbers for one question, and
					// it still scored a visibly two-line 54-char title at 100.
					// An unmeasurable title now scores clean rather than against
					// a number nothing renders (go-slide-creator-jcph).
					if text, isText := resolved.(string); isText {
						m := measureTitleInPlaceholder(text, titlePlaceholderIn(predictedLayouts[i], item.PlaceholderID))
						if m.Flagged() {
							penalty := 0.15
							if m.Overflow {
								penalty = 0.3
							}
							slideScore -= penalty
							issues = append(issues, m.describe())
						}
					}
				}
			case "bullets":
				if bullets, ok := resolved.([]string); ok {
					bulletsByPlaceholder[item.PlaceholderID] += len(bullets)
				}
			case "table":
				if table, ok := resolved.(*TableInput); ok {
					if len(table.Rows) == 0 {
						slideScore -= 0.2
						issues = append(issues, "table has no data rows")
					}
					if len(table.Headers) == 0 {
						slideScore -= 0.1
						issues = append(issues, "table has no headers")
					}
				}
			case "body_and_bullets":
				if bab, ok := resolved.(*BodyAndBulletsInput); ok {
					bulletsByPlaceholder[item.PlaceholderID] += len(bab.Bullets)
				}
			case "bullet_groups":
				if bg, ok := resolved.(*BulletGroupsInput); ok {
					for _, g := range bg.Groups {
						bulletsByPlaceholder[item.PlaceholderID] += len(g.Bullets)
					}
				}
			case "chart":
				chartIssues := scoreChartData(item)
				for _, ci := range chartIssues {
					slideScore -= 0.3
					issues = append(issues, ci)
				}
			case "diagram":
				diagIssues := scoreDiagramData(item)
				for _, di := range diagIssues {
					slideScore -= 0.3
					issues = append(issues, di)
				}
			}
		}

		// Section divider body misuse: agents sometimes put long sentences into
		// the body placeholder which is designed for a decorative section number
		// (e.g. "01"). Warn when the body text is >4 chars and non-numeric.
		if inferSlideType(slide) == types.SlideTypeSection {
			for _, item := range slide.Content {
				if item.Type == "text" && strings.EqualFold(item.PlaceholderID, "body") {
					resolved, _ := item.ResolveValue()
					if text, ok := resolved.(string); ok {
						trimmed := strings.TrimSpace(text)
						if len(trimmed) > 4 {
							if _, err := strconv.Atoi(trimmed); err != nil {
								slideScore -= 0.3
								issues = append(issues, fmt.Sprintf("section divider body misused as text (%d chars; use short number like '01')", len(trimmed)))
							}
						}
					}
				}
			}
		}

		// Bullet count penalty — applied to the densest single placeholder.
		bulletCount := 0
		for _, n := range bulletsByPlaceholder {
			if n > bulletCount {
				bulletCount = n
			}
		}
		if bulletCount > maxBullets {
			penalty := float64(bulletCount-maxBullets) * 0.05
			if penalty > 0.4 {
				penalty = 0.4
			}
			slideScore -= penalty
			issues = append(issues, fmt.Sprintf("too many bullets (%d, max %d)", bulletCount, maxBullets))
		}

		// Too many content items = crowded
		if contentCount > maxContent {
			penalty := float64(contentCount-maxContent) * 0.05
			if penalty > 0.3 {
				penalty = 0.3
			}
			slideScore -= penalty
			issues = append(issues, fmt.Sprintf("slide is crowded (%d content items)", contentCount))
		}

		// Clamp slide score to [0, 1]
		if slideScore < 0 {
			slideScore = 0
		}

		slideScores = append(slideScores, SlideQuality{
			SlideNumber: i + 1,
			Score:       toScore100(slideScore),
			Issues:      issues,
		})
		totalScore += slideScore
	}

	// Average slide score
	overallScore := totalScore / float64(len(slides))

	// Penalty for pipeline warnings
	if len(warnings) > 0 {
		warningPenalty := float64(len(warnings)) * 0.02
		if warningPenalty > 0.2 {
			warningPenalty = 0.2
		}
		overallScore -= warningPenalty
		globalIssues = append(globalIssues, fmt.Sprintf("%d pipeline warning(s)", len(warnings)))
	}

	// Refuse-class findings are content loss or an unrenderable slide, not a
	// styling nit. A deck that dropped three rows of financial data must not
	// score 100 (go-slide-creator-oaif), so these carry a hard, uncapped
	// penalty — unlike the generic warning penalty, which caps at 0.2.
	if blocking := blockingFindingCodes(findings); len(blocking) > 0 {
		overallScore -= float64(len(blocking)) * refuseFindingPenalty
		globalIssues = append(globalIssues, fmt.Sprintf(
			"%d blocking finding(s): %s", len(blocking), strings.Join(blocking, ", ")))
	}

	// Clamp overall score
	if overallScore < 0 {
		overallScore = 0
	}
	if overallScore > 1 {
		overallScore = 1
	}

	return &QualityScore{
		Score:       toScore100(overallScore),
		Basis:       scoreBasisInput,
		Scope:       "input_heuristic",
		SlideScores: slideScores,
		Issues:      globalIssues,
	}
}

func hasBlockingOutputFinding(findings []pptx.Finding) bool {
	for _, finding := range findings {
		if finding.Severity == pptx.SeverityBlocking {
			return true
		}
	}
	return false
}

// sanitizeOutputFilename strips directory components from a user-supplied
// filename to prevent path-traversal attacks (e.g. "../../evil.pptx").
// It returns a bare filename with a .pptx suffix, defaulting to "output.pptx"
// when the input is empty or resolves to nothing useful.
func sanitizeOutputFilename(raw string) string {
	// filepath.Base strips all directory components:
	//   "../../evil.pptx"  → "evil.pptx"
	//   "/tmp/secret.pptx" → "secret.pptx"
	//   ""                 → "."
	name := filepath.Base(raw)
	if name == "" || name == "." || name == ".." {
		name = "output.pptx"
	}
	if !strings.HasSuffix(name, ".pptx") {
		name += ".pptx"
	}
	return name
}

// isLikelyTitle returns true if a placeholder ID looks like a title placeholder
// (but not a subtitle).
func isLikelyTitle(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	if isLikelySubtitle(placeholderID) {
		return false
	}
	return strings.Contains(lower, "title") || strings.Contains(lower, "heading")
}

// isLikelySubtitle returns true if a placeholder ID looks like a subtitle placeholder.
func isLikelySubtitle(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	return strings.Contains(lower, "subtitle") || strings.Contains(lower, "subheading")
}

// scoreChartData checks a chart content item for basic data structure problems.
// Returns a list of issue strings (empty if no problems detected).
func scoreChartData(item ContentInput) []string {
	spec := item.ChartValue
	if spec == nil && len(item.Value) > 0 {
		var parsed types.ChartSpec //nolint:staticcheck // backward compat
		if err := json.Unmarshal(item.Value, &parsed); err == nil {
			spec = &parsed
		}
	}
	if spec == nil {
		return nil // no data to inspect
	}
	return checkDiagramDataStructure(string(spec.Type), spec.Data)
}

// scoreDiagramData checks a diagram content item for basic data structure problems.
// Returns a list of issue strings (empty if no problems detected).
func scoreDiagramData(item ContentInput) []string {
	spec := item.DiagramValue
	if spec == nil && len(item.Value) > 0 {
		var parsed types.DiagramSpec
		if err := json.Unmarshal(item.Value, &parsed); err == nil {
			spec = &parsed
		}
	}
	if spec == nil {
		return nil
	}
	return checkDiagramDataStructure(spec.Type, spec.Data)
}

// checkDiagramDataStructure validates that a diagram's data map contains
// the required keys for its type. Returns issue descriptions for any
// structural problems found.
func checkDiagramDataStructure(diagramType string, data map[string]any) []string {
	if data == nil {
		return []string{fmt.Sprintf("%s has no data", diagramType)}
	}

	// Normalize type name: handle both canonical and alias forms.
	normalized := strings.ToLower(diagramType)

	var issues []string
	switch normalized {
	case "waterfall":
		_, hasPoints := data["points"]
		_, hasLabels := data["labels"]
		_, hasValues := data["values"]
		if !hasPoints && !(hasLabels && hasValues) {
			issues = append(issues, "waterfall chart missing 'points' array (or 'labels'+'values')")
		}
	case "funnel", "funnel_chart":
		_, hasStages := data["stages"]
		_, hasValues := data["values"]
		_, hasPoints := data["points"]
		if !hasStages && !hasValues && !hasPoints {
			issues = append(issues, "funnel chart missing 'stages' (or 'values'/'points') array")
		}
	case "gauge", "gauge_chart":
		if _, ok := data["value"]; !ok {
			issues = append(issues, "gauge chart missing 'value' field")
		}
		_, hasMin := data["min"]
		_, hasMax := data["max"]
		if !hasMin && !hasMax {
			issues = append(issues, "gauge chart missing 'min'/'max' range")
		}
	case "porters_five_forces", "porter", "porters":
		if _, ok := data["forces"]; !ok {
			// Check if force keys are embedded directly (e.g. data.rivalry)
			directForceKeys := []string{"rivalry", "new_entrants", "substitutes", "suppliers", "buyers"}
			hasAny := false
			for _, k := range directForceKeys {
				if _, ok := data[k]; ok {
					hasAny = true
					break
				}
			}
			if !hasAny {
				issues = append(issues, "porter's five forces missing 'forces' array")
			}
		}
	}
	return issues
}

// convertMediaFailures converts generator.MediaFailure records into
// SlideError structs for JSON output. Each MediaFailure represents a
// chart, diagram, image, or table that failed to render on a specific slide.
func convertMediaFailures(failures []generator.MediaFailure) []SlideError {
	if len(failures) == 0 {
		return nil
	}
	errors := make([]SlideError, len(failures))
	for i, f := range failures {
		errors[i] = SlideError{
			SlideNumber: f.SlideNum,
			ContentType: f.ContentType,
			DiagramType: f.DiagramType,
			Error:       f.Reason,
			Fallback:    f.Fallback,
		}
	}
	return errors
}

// writeJSONOutput writes a JSON output to file or stdout.
func writeJSONOutput(path string, output JSONOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}

	if path == "-" {
		_, err = os.Stdout.Write(data)
		_, _ = os.Stdout.WriteString("\n")
	} else {
		err = os.WriteFile(path, append(data, '\n'), 0644)
	}

	return err
}

// inferSlideType determines the SlideType from an explicit hint or content analysis.
func inferSlideType(slide SlideInput, layouts ...types.LayoutMetadata) types.SlideType {
	if slide.SlideType != "" {
		return types.SlideType(slide.SlideType)
	}
	if isSectionSlideInput(slide, layouts) {
		return types.SlideTypeSection
	}

	hasChart := false
	hasDiagram := false
	hasImage := false
	hasTable := false
	textCount := 0
	bodyTextCount := 0 // text items that are NOT title/subtitle
	hasBullets := false

	for _, item := range slide.Content {
		switch item.Type {
		case "chart":
			hasChart = true
		case "diagram":
			hasDiagram = true
		case "image":
			hasImage = true
		case "table":
			hasTable = true
		case "text":
			textCount++
			if item.PlaceholderID != "" && !isTitlePlaceholderID(item.PlaceholderID) && !isLikelySubtitle(item.PlaceholderID) {
				bodyTextCount++
			}
		case "bullets", "body_and_bullets", "bullet_groups":
			hasBullets = true
		}
	}

	if hasChart || hasDiagram {
		if hasChart {
			return types.SlideTypeChart
		}
		return types.SlideTypeDiagram
	}
	if hasImage && (textCount > 0 || hasBullets) {
		return types.SlideTypeTwoColumn
	}
	if hasImage {
		return types.SlideTypeImage
	}
	if hasTable {
		return types.SlideTypeContent
	}
	// A slide with only title/subtitle text (no body text, no bullets) is a
	// title-type slide — this covers both opening and closing slides (e.g.,
	// "Thank You" + "Questions?" on slideLayout5). An explicitly chosen
	// content layout is different: a lone title there is a nearly empty
	// argument slide, not an opener or closer.
	if bodyTextCount == 0 && !hasBullets {
		if explicitContentLayout(slide.LayoutID, layouts) {
			return types.SlideTypeContent
		}
		return types.SlideTypeTitle
	}
	return types.SlideTypeContent
}

func explicitContentLayout(layoutID string, layouts []types.LayoutMetadata) bool {
	if strings.EqualFold(layoutID, "content") {
		return true
	}
	for i := range layouts {
		if layouts[i].ID != layoutID {
			continue
		}
		if template.EffectiveCanonicalType(&layouts[i]).Family() == types.LayoutFamilyOneContent {
			return true
		}
		return hasLayoutTag(layouts[i].Tags, "content") || hasLayoutTag(layouts[i].Tags, "two-column")
	}
	return false
}

func isSectionSlideInput(slide SlideInput, layouts []types.LayoutMetadata) bool {
	if types.SlideType(slide.SlideType) == types.SlideTypeSection {
		return true
	}
	if strings.EqualFold(slide.LayoutID, "section") {
		return true
	}
	for _, l := range layouts {
		if l.ID != slide.LayoutID {
			continue
		}
		if l.CanonicalType == types.CanonicalLayoutSectionDivider {
			return true
		}
		for _, tag := range l.Tags {
			if tag == "section-header" {
				return true
			}
		}
	}
	return false
}

// isTitlePlaceholderID returns true if the placeholder ID targets the title area
// (not subtitle, body, or other content areas).
func isTitlePlaceholderID(id string) bool {
	lower := strings.ToLower(id)
	return lower == "title" || strings.HasPrefix(lower, "title_")
}

// inferJSONSlideType guesses the slide type from legacy JSON slide content.
// Title slides typically have only "title" and optionally "subtitle" text — no
// body, bullets, charts, or images.
func inferJSONSlideType(slide JSONSlide) types.SlideType {
	for _, item := range slide.Content {
		switch item.Type {
		case "bullets", "chart", "diagram", "image", "table",
			"body_and_bullets", "bullet_groups":
			return types.SlideTypeContent
		}
		// Text in non-title, non-subtitle placeholder → content slide
		if item.Type == "text" {
			lower := strings.ToLower(item.PlaceholderID)
			if lower != "title" && lower != "subtitle" && !strings.HasPrefix(lower, "title_") {
				return types.SlideTypeContent
			}
		}
	}
	return types.SlideTypeTitle
}

// jsonSlideToDefinition builds a types.SlideDefinition from JSON input
// for the layout heuristic engine. It translates ContentInput items into the
// SlideContent fields that the heuristic scorer expects.
func jsonSlideToDefinition(slide SlideInput) types.SlideDefinition { //nolint:gocognit,gocyclo
	def := types.SlideDefinition{
		Type: inferSlideType(slide),
	}

	for _, item := range slide.Content {
		resolved, _ := item.ResolveValue()

		// Populate Slots map for content items with slot markers so that
		// HasSlots() returns true and layout selection requires enough
		// content placeholders (prevents silent slot2 content loss).
		if isSlotMarker(item.PlaceholderID) {
			slotNum, _ := strconv.Atoi(item.PlaceholderID[4:])
			if def.Slots == nil {
				def.Slots = make(map[int]*types.SlotContent)
			}
			sc := &types.SlotContent{SlotNumber: slotNum}
			switch item.Type {
			case "bullets":
				sc.Type = types.SlotContentBullets
			case "body_and_bullets":
				sc.Type = types.SlotContentBodyAndBullets
			case "bullet_groups":
				sc.Type = types.SlotContentBulletGroups
			case "chart":
				sc.Type = types.SlotContentChart
			case "diagram":
				sc.Type = types.SlotContentInfographic
			case "table":
				sc.Type = types.SlotContentTable
			case "image":
				sc.Type = types.SlotContentImage
			case "text":
				sc.Type = types.SlotContentText
			}
			def.Slots[slotNum] = sc
		}

		switch item.Type {
		case "text":
			if def.Title == "" {
				if text, ok := resolved.(string); ok {
					def.Title = text
				}
			} else {
				if text, ok := resolved.(string); ok {
					def.Content.Body = text
				}
			}
		case "bullets":
			if bullets, ok := resolved.([]string); ok {
				def.Content.Bullets = bullets
			}
		case "body_and_bullets":
			if bab, ok := resolved.(*BodyAndBulletsInput); ok {
				def.Content.Body = bab.Body
				def.Content.Bullets = bab.Bullets
				def.Content.BodyAfterBullets = bab.TrailingBody
			}
		case "bullet_groups":
			if bg, ok := resolved.(*BulletGroupsInput); ok {
				var groups []types.BulletGroup
				for _, g := range bg.Groups {
					groups = append(groups, types.BulletGroup{
						Header:  g.Header,
						Body:    g.Body,
						Bullets: g.Bullets,
					})
					// Populate flat Bullets for backward compatibility with
					// layout scoring (mirrors markdown parser behavior).
					def.Content.Bullets = append(def.Content.Bullets, g.Bullets...)
				}
				def.Content.BulletGroups = groups
				if bg.Body != "" {
					def.Content.Body = bg.Body
				}
			}
		case "chart":
			if chart, ok := resolved.(*types.ChartSpec); ok { //nolint:staticcheck // backward compat
				def.Content.DiagramSpec = chart.ToDiagramSpec()
			} else if len(item.Value) > 0 {
				var chart types.ChartSpec //nolint:staticcheck // backward compat
				if json.Unmarshal(item.Value, &chart) == nil {
					def.Content.DiagramSpec = chart.ToDiagramSpec()
				}
			}
		case "diagram":
			if diagram, ok := resolved.(*types.DiagramSpec); ok {
				def.Content.DiagramSpec = diagram
			} else if len(item.Value) > 0 {
				var diagram types.DiagramSpec
				if json.Unmarshal(item.Value, &diagram) == nil {
					def.Content.DiagramSpec = &diagram
				}
			}
		case "table":
			def.Content.TableRaw = "table" // Signal presence for heuristic
		case "image":
			if img, ok := resolved.(*ImageInput); ok {
				def.Content.ImagePath = img.Path
			} else if len(item.Value) > 0 {
				var img struct {
					Path string `json:"path"`
				}
				if json.Unmarshal(item.Value, &img) == nil {
					def.Content.ImagePath = img.Path
				}
			}
		}
	}

	// When the content array (no explicit ::slotN:: markers) targets two or more
	// distinct content placeholders (e.g., body + body_2, or left + right), model
	// the slide as multi-slot so layout selection requires a layout with enough
	// content placeholders. Without this, a two-column slide whose per-column
	// bullet counts exceed a multi-content layout's small MaxBullets is scored
	// down on capacity and loses to a single-content layout — collapsing both
	// columns into one placeholder (the second silently overwrites the first).
	if len(def.Slots) == 0 {
		if slots := synthesizeContentSlots(slide.Content); len(slots) >= 2 {
			def.Slots = slots
		}
	}

	// Pattern, compose, and shape_grid slides expand into a full-area shape grid
	// rendered into the body content zone. Their `content` array usually carries
	// only a title, so inferSlideType classifies them as title-type slides — and
	// the heuristic then happily assigns a Section Divider / title / closing
	// layout whose tiny content area (~0.58in tall) the expanded grid overflows
	// (silently dropped under --partial, hard error otherwise). Model them as
	// full-area visual slides so layout selection reuses isLayoutSuitable's
	// rejection of section-header/closing/two-column layouts and its minimum
	// body-placeholder size guard, steering to a real content layout instead.
	// Composition requirements take precedence over a generic slide_type hint:
	// semantic compilation intentionally emits slide_type=content, but the
	// pattern still requires a full visual canvas.
	if hasPatternContent(slide) {
		def.Type = types.SlideTypeDiagram
	}

	return def
}

func findLayoutMetadataByID(layouts []types.LayoutMetadata, id string) *types.LayoutMetadata {
	for i := range layouts {
		if layouts[i].ID == id {
			return &layouts[i]
		}
	}
	return nil
}

func hasLayoutTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

func isCompositionLayoutCompatible(candidate types.LayoutMetadata) bool {
	if hasLayoutTag(candidate.Tags, "blank-title") || hasLayoutTag(candidate.Tags, "content") {
		return true
	}
	for _, incompatible := range []string{"section-header", "closing", "two-column", "title-slide"} {
		if hasLayoutTag(candidate.Tags, incompatible) {
			return false
		}
	}
	// Unclassified explicit layouts remain backward compatible. Their actual
	// geometry is still clamped by resolveGridGeometry.
	return true
}

// hasPatternContent reports whether a slide carries pattern, compose, or
// shape_grid content that expands into a full-area shape grid. These three
// envelopes are mutually exclusive (XOR-enforced in convertSinglePresentationSlide).
func hasPatternContent(slide SlideInput) bool {
	return slide.Pattern != nil || slide.Compose != nil || slide.ShapeGrid != nil
}

// synthesizeContentSlots derives a slot map from the distinct content-bearing
// placeholder IDs referenced by a slide's content array. Title, subtitle, and
// section-number placeholders are excluded because they are chrome, not content
// columns. Slot numbers are assigned 1..N in order of first appearance so that
// downstream position-sensitive checks (narrow-diagram penalty, per-slot height
// guards) map slots to content placeholders left-to-right.
func synthesizeContentSlots(content []ContentInput) map[int]*types.SlotContent {
	slots := make(map[int]*types.SlotContent)
	order := make(map[string]int) // normalized placeholder ID -> slot number
	next := 1

	for _, item := range content {
		id := item.PlaceholderID
		if id == "" {
			continue
		}
		if isTitlePlaceholderID(id) || isLikelySubtitle(id) || placeholderrole.IsSectionNumberAlias(id) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(id))
		if _, seen := order[key]; seen {
			continue
		}
		slotNum := next
		next++
		order[key] = slotNum

		sc := &types.SlotContent{SlotNumber: slotNum}
		switch item.Type {
		case "bullets":
			sc.Type = types.SlotContentBullets
		case "body_and_bullets":
			sc.Type = types.SlotContentBodyAndBullets
		case "bullet_groups":
			sc.Type = types.SlotContentBulletGroups
		case "chart":
			sc.Type = types.SlotContentChart
		case "diagram":
			sc.Type = types.SlotContentInfographic
		case "table":
			sc.Type = types.SlotContentTable
		case "image":
			sc.Type = types.SlotContentImage
		case "text":
			sc.Type = types.SlotContentText
		}
		// Attach the diagram spec so width-sensitive scoring (narrow-diagram
		// penalty) can evaluate this slot's placeholder.
		if sc.Type == types.SlotContentChart || sc.Type == types.SlotContentInfographic {
			if resolved, err := item.ResolveValue(); err == nil {
				switch v := resolved.(type) {
				case *types.DiagramSpec:
					sc.DiagramSpec = v
				case *types.ChartSpec: //nolint:staticcheck // backward compat
					sc.DiagramSpec = v.ToDiagramSpec()
				}
			}
		}
		slots[slotNum] = sc
	}

	return slots
}

// autoMapPlaceholders assigns placeholder IDs to content items that lack them,
// using the selected layout's placeholder metadata. It also resolves virtual
// slot markers ("slot1", "slot2", ...) to actual content placeholder IDs.
func autoMapPlaceholders(items []ContentInput, selectedLayout types.LayoutMetadata) []ContentInput {
	result := make([]ContentInput, len(items))
	copy(result, items)

	// Build slot-to-placeholder mapping for "slot1", "slot2", etc.
	// This mirrors the logic in generator.BuildSlotContentItems.
	contentPlaceholders := generator.FilterContentPlaceholders(selectedLayout.Placeholders)
	slotMap := make(map[string]string, len(contentPlaceholders))
	for i, ph := range contentPlaceholders {
		slotMap[fmt.Sprintf("slot%d", i+1)] = placeholderIDStr(ph)
	}

	titlePH := findFirstPlaceholder(selectedLayout, types.PlaceholderTitle)
	bodyPH := findFirstPlaceholder(selectedLayout, types.PlaceholderBody)
	if bodyPH == nil {
		bodyPH = findFirstPlaceholder(selectedLayout, types.PlaceholderContent)
	}
	subtitlePH := findFirstPlaceholder(selectedLayout, types.PlaceholderSubtitle)
	imagePH := findFirstPlaceholder(selectedLayout, types.PlaceholderImage)
	chartPH := findFirstPlaceholder(selectedLayout, types.PlaceholderChart)

	// Build a map of well-known virtual IDs to actual placeholder IDs.
	// JSON producers use "title", "subtitle", "body" as logical names,
	// but the actual OOXML placeholder types may differ (e.g., "ctrTitle", "subTitle").
	virtualMap := make(map[string]string)
	if titlePH != nil {
		virtualMap["title"] = placeholderIDStr(*titlePH)
	} else if bodyPH != nil {
		// Section divider layouts may lack a title placeholder entirely,
		// using only a body placeholder for the section title text.
		// Fall back to the body placeholder so "title" content items render.
		virtualMap["title"] = placeholderIDStr(*bodyPH)
	}
	if subtitlePH != nil {
		virtualMap["subtitle"] = placeholderIDStr(*subtitlePH)
	}
	if bodyPH != nil {
		virtualMap["body"] = placeholderIDStr(*bodyPH)
	}
	// Map "body_2", "body_3", etc. to the 2nd, 3rd, ... content placeholders.
	// This ensures two-column (and multi-column) layouts populated via "body"/"body_2"
	// target the correct physical placeholder instead of relying on semantic fallback
	// which sorts by area and may map both to the same shape.
	for i := 1; i < len(contentPlaceholders); i++ {
		key := fmt.Sprintf("body_%d", i+1) // body_2, body_3, ...
		virtualMap[key] = placeholderIDStr(contentPlaceholders[i])
	}

	titleAssigned := false
	for i := range result {
		// Resolve virtual placeholder IDs (slots and well-known names).
		if phID, ok := slotMap[result[i].PlaceholderID]; ok {
			result[i].PlaceholderID = phID
			continue
		}
		if phID, ok := virtualMap[result[i].PlaceholderID]; ok {
			if result[i].PlaceholderID == "title" {
				titleAssigned = true
			}
			result[i].PlaceholderID = phID
			continue
		}

		if result[i].PlaceholderID != "" {
			continue // Already has explicit placeholder
		}

		switch result[i].Type {
		case "text":
			if !titleAssigned && titlePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*titlePH)
				titleAssigned = true
			} else if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			} else if subtitlePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*subtitlePH)
			}
		case "bullets", "body_and_bullets", "bullet_groups", "table":
			if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			}
		case "chart", "diagram":
			if chartPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*chartPH)
			} else if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			}
		case "image":
			if imagePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*imagePH)
			}
		}
	}

	return result
}

// injectSectionNumber appends an auto-assigned section number to a
// section-divider slide's content. When the resolved layout exposes a
// decorative section_number placeholder, the number is routed there via the
// virtual ID "section_number" (resolved downstream by the generator's
// section-number alias chain). Otherwise it falls back to the body/tagline slot
// so templates without a dedicated number frame keep their prior behavior.
//
// It is a no-op when num is empty (non-section slide) or when the chosen target
// is already populated by user-supplied content.
func injectSectionNumber(content []ContentInput, layout *types.LayoutMetadata, num string) []ContentInput {
	if num == "" {
		return content
	}
	target := "body"
	if layout != nil && layoutHasSectionNumberPlaceholder(*layout) {
		target = "section_number"
	}
	if contentTargetsSectionSlot(content, target) {
		return content
	}
	n := num
	return append(content, ContentInput{
		PlaceholderID: target,
		Type:          "text",
		TextValue:     &n,
	})
}

// layoutHasSectionNumberPlaceholder reports whether a layout exposes a
// decorative section_number placeholder. It trusts the canonical Role
// classification first, then falls back to alias IDs (section_number, section_no,
// large_number) and the conventional "Section Number" name.
func layoutHasSectionNumberPlaceholder(layout types.LayoutMetadata) bool {
	for _, ph := range layout.Placeholders {
		if ph.Role == types.PlaceholderRoleSectionNumber {
			return true
		}
		if placeholderrole.IsSectionNumberAlias(ph.ID) {
			return true
		}
		lid := strings.ToLower(ph.ID)
		if strings.Contains(lid, "section") && strings.Contains(lid, "number") {
			return true
		}
	}
	return false
}

// contentTargetsSectionSlot reports whether the content already populates the
// chosen section-number target, so the auto-injection can be skipped. For the
// "section_number" target it matches any section-number alias or "Section
// Number" name; for the "body" fallback it matches an exact "body" ID, mirroring
// the original guard.
func contentTargetsSectionSlot(content []ContentInput, target string) bool {
	for _, ci := range content {
		if target == "section_number" {
			if placeholderrole.IsSectionNumberAlias(ci.PlaceholderID) {
				return true
			}
			lid := strings.ToLower(ci.PlaceholderID)
			if strings.Contains(lid, "section") && strings.Contains(lid, "number") {
				return true
			}
		} else if ci.PlaceholderID == target {
			return true
		}
	}
	return false
}

// hasVirtualPlaceholders returns true if any content item uses a virtual
// placeholder ID that needs resolution (e.g., "slot1", "title", "subtitle", "body").
func hasVirtualPlaceholders(items []ContentInput) bool {
	for _, item := range items {
		switch item.PlaceholderID {
		case "title", "subtitle", "body":
			return true
		}
		if isSlotMarker(item.PlaceholderID) {
			return true
		}
		if isBodyNMarker(item.PlaceholderID) {
			return true
		}
	}
	return false
}

// isBodyNMarker returns true if the placeholder ID is a virtual body_N marker (e.g., "body_2", "body_3").
func isBodyNMarker(id string) bool {
	if !strings.HasPrefix(id, "body_") {
		return false
	}
	_, err := strconv.Atoi(id[5:])
	return err == nil
}

// isSlotMarker returns true if the placeholder ID is a virtual slot marker (e.g., "slot1", "slot2").
func isSlotMarker(id string) bool {
	if !strings.HasPrefix(id, "slot") {
		return false
	}
	_, err := strconv.Atoi(id[4:])
	return err == nil
}

// findFirstPlaceholder returns the first placeholder of a given type in a layout.
func findFirstPlaceholder(layout types.LayoutMetadata, phType types.PlaceholderType) *types.PlaceholderInfo {
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == phType {
			return &layout.Placeholders[i]
		}
	}
	return nil
}

// placeholderIDStr returns the placeholder's canonical ID.
// After normalization, placeholder IDs are unique within a layout
// (e.g., "title", "body", "body_2", "image").
func placeholderIDStr(ph types.PlaceholderInfo) string {
	return ph.ID
}

// chromeToFooterConfig converts a ChromeInput into a generator.FooterConfig.
// It composes the left footer text from the chrome fields and sets up page
// number formatting.
//
// slides are the expanded deck slides, needed only for chrome.section_crumb:
// each slide carries the section it came from, and the crumb is appended to
// that slide's own footer line (go-slide-creator-ynfv).
func chromeToFooterConfig(chrome *ChromeInput, totalSlides int, slides []SlideInput) *generator.FooterConfig {
	base := composeChromeLine(chrome)
	cfg := &generator.FooterConfig{
		Enabled:     true,
		LeftText:    base,
		TotalSlides: totalSlides,
	}
	if chrome.SectionCrumb {
		cfg.LeftTextBySlide = sectionCrumbFooterLines(base, slides)
	}
	if chrome.PageNumbers != nil {
		if chrome.PageNumbers.Enabled != nil && !*chrome.PageNumbers.Enabled {
			// Page numbers explicitly disabled — keep footer but no slide number.
			cfg.PageNumberFormat = ""
		} else if chrome.PageNumbers.Format != "" {
			cfg.PageNumberFormat = chrome.PageNumbers.Format
		}
	}
	return cfg
}

// footerConfigForInput merges the legacy footer line with chrome instead of
// discarding it whenever chrome configures an unrelated feature (such as page
// numbering). Chrome's structured line is retained, with distinct legacy text
// appended; section crumbs are then derived from the final merged line.
func footerConfigForInput(input *PresentationInput, totalSlides int) *generator.FooterConfig {
	if input == nil {
		return nil
	}
	legacyText := ""
	legacyEnabled := input.Footer != nil && input.Footer.Enabled
	if legacyEnabled {
		legacyText = input.Footer.LeftText
	}
	if input.Chrome == nil {
		if !legacyEnabled {
			return nil
		}
		return &generator.FooterConfig{Enabled: true, LeftText: legacyText}
	}
	cfg := chromeToFooterConfig(input.Chrome, totalSlides, input.Slides)
	if legacyText != "" && legacyText != cfg.LeftText && !chromeContainsFooterText(input.Chrome, legacyText) {
		if cfg.LeftText == "" {
			cfg.LeftText = legacyText
		} else {
			cfg.LeftText += " | " + legacyText
		}
		if input.Chrome.SectionCrumb {
			cfg.LeftTextBySlide = sectionCrumbFooterLines(cfg.LeftText, input.Slides)
		}
	}
	return cfg
}

func chromeContainsFooterText(chrome *ChromeInput, text string) bool {
	return text == chrome.Confidentiality || text == chrome.ClientName ||
		text == chrome.FooterDate || (chrome.ProjectCode != "" && text == "Project "+chrome.ProjectCode)
}

// composeChromeLine builds the left footer text from chrome fields.
// Non-empty fields are joined with " | " separators.
// Example: "Strictly confidential — Project Aurora | Acme Corp | May 2026"
func composeChromeLine(chrome *ChromeInput) string {
	var parts []string
	if chrome.Confidentiality != "" {
		parts = append(parts, chrome.Confidentiality)
	}
	if chrome.ProjectCode != "" {
		parts = append(parts, "Project "+chrome.ProjectCode)
	}
	if chrome.ClientName != "" {
		parts = append(parts, chrome.ClientName)
	}
	if chrome.FooterDate != "" {
		parts = append(parts, chrome.FooterDate)
	}
	if len(parts) == 0 {
		return ""
	}
	// Use " — " between confidentiality and the rest, " | " within.
	if chrome.Confidentiality != "" && len(parts) > 1 {
		return parts[0] + " — " + strings.Join(parts[1:], " | ")
	}
	return strings.Join(parts, " | ")
}

// sectionCrumbFooterLines builds the per-slide footer text for
// chrome.section_crumb: the deck-wide chrome line with the running section
// title appended. Slides outside a section (cover, agenda, dividers, closing)
// get an empty entry and fall back to the deck-wide line.
//
// Returns nil when no slide carries a section, so a deck that sets
// section_crumb without a structure block costs nothing — and, importantly,
// changes nothing.
func sectionCrumbFooterLines(base string, slides []SlideInput) []string {
	lines := make([]string, len(slides))
	any := false
	for i := range slides {
		sec := strings.TrimSpace(slides[i].SectionTitle)
		if sec == "" {
			continue
		}
		any = true
		if base == "" {
			lines[i] = sec
			continue
		}
		lines[i] = base + " | " + sec
	}
	if !any {
		return nil
	}
	return lines
}

// applyChromeSkip sets SkipFooter=true on slides whose layout should not carry
// page-number / footer chrome. The default skip set is ["title", "closing"]
// when page_numbers is unset or has no explicit skip list.
//
// The two well-known skip names route through the canonical layout taxonomy
// (template.EffectiveCanonicalType) rather than the raw structural tags:
// "title" matches the canonical Title Slide and "closing" the canonical
// Closing. This is the single source of truth shared with generation and
// preflight, and it deliberately fixes a long-standing divergence — a
// title-only Section Divider carries the structural "title-slide" tag and used
// to be skipped, but it classifies canonically as Section Divider and is now
// kept in the numbered body flow, matching the documented "title and closing"
// default. Any other (caller-supplied, arbitrary) skip value still matches the
// layout's structural tags, so e.g. ["section-header"] can opt section dividers
// back out.
func applyChromeSkip(specs []generator.SlideSpec, chrome *ChromeInput, slides []SlideInput, layouts []types.LayoutMetadata) {
	// Canonical layout types to skip (well-known names), and arbitrary tags to
	// skip (caller-supplied values that aren't well-known names).
	skipCanonical := map[types.CanonicalLayoutType]bool{
		types.CanonicalLayoutTitleSlide: true,
		types.CanonicalLayoutClosing:    true,
	}
	skipTags := map[string]bool{}
	if chrome.PageNumbers != nil && chrome.PageNumbers.Skip != nil {
		skipCanonical = map[types.CanonicalLayoutType]bool{}
		for _, name := range chrome.PageNumbers.Skip {
			switch name {
			case "title":
				skipCanonical[types.CanonicalLayoutTitleSlide] = true
			case "closing":
				skipCanonical[types.CanonicalLayoutClosing] = true
			default:
				// Arbitrary structural tag (e.g. "section-header", "blank").
				skipTags[name] = true
			}
		}
	}

	// Build layout ID → layout lookup so we can read both canonical type and tags.
	layoutByID := make(map[string]*types.LayoutMetadata, len(layouts))
	for i := range layouts {
		layoutByID[layouts[i].ID] = &layouts[i]
	}

	for i := range specs {
		l := layoutByID[specs[i].LayoutID]
		if l == nil {
			continue
		}
		if len(skipCanonical) > 0 && skipCanonical[template.EffectiveCanonicalType(l)] {
			specs[i].SkipFooter = true
			continue
		}
		for _, tag := range l.Tags {
			if skipTags[tag] {
				specs[i].SkipFooter = true
				break
			}
		}
	}
}

// patternThemeFromDiag exposes template colors and the body font to pattern
// expanders for contrast and content-sized text measurement. Returns the zero
// ThemeInfo when no diagram context is available.
func patternThemeFromDiag(diagCtx *GridDiagramContext) types.ThemeInfo {
	if diagCtx == nil {
		return types.ThemeInfo{}
	}
	return types.ThemeInfo{Colors: diagCtx.ThemeColors, BodyFont: diagCtx.FontFamily}
}

// droppedPlaceholdersBySlide indexes hard content drops — content targeting a
// placeholder the resolved layout does not declare — by 0-based slide index.
// The result feeds buildSlideResolutions so placeholders_used reports only
// placeholders that actually received content (go-slide-creator-lhq6).
func droppedPlaceholdersBySlide(findings []patterns.FitFinding) map[int]map[string]bool {
	var bySlide map[int]map[string]bool
	for _, f := range findings {
		if !patterns.IsHardContentDrop(f) {
			continue
		}
		phID, _ := f.Fix.Params["placeholder_id"].(string)
		if phID == "" {
			continue
		}
		slideIdx := slidepath.SlideIndex(f.Path)
		if slideIdx < 0 {
			continue
		}
		if bySlide == nil {
			bySlide = make(map[int]map[string]bool)
		}
		if bySlide[slideIdx] == nil {
			bySlide[slideIdx] = make(map[string]bool)
		}
		bySlide[slideIdx][phID] = true
	}
	return bySlide
}

// hasHardContentDrop reports whether the run dropped author-provided content
// because a targeted placeholder does not exist in the resolved layout. Under
// strict output_validation (the default) this fails the render: the artifact is
// missing content the author asked for, so answering success:true would tell
// the agent nothing is wrong (go-slide-creator-lhq6).
func hasHardContentDrop(findings []patterns.FitFinding) bool {
	for _, f := range findings {
		if patterns.IsHardContentDrop(f) {
			return true
		}
	}
	return false
}

// renderSucceeded reports the success flag for a completed render given the
// findings collected and the effective output_validation mode.
func renderSucceeded(findings []patterns.FitFinding, outputValidation string) bool {
	if outputValidation == "" {
		outputValidation = "strict"
	}
	if outputValidation == "strict" && hasHardContentDrop(findings) {
		return false
	}
	return true
}

// refuseFindingPenalty is the score deducted per refuse-class finding. At 0.25
// a single one takes a perfect deck out of the 90s, and three take it below the
// usual gate — which is the point: the deck is missing content.
const refuseFindingPenalty = 0.25

// blockingFindingCodes returns the distinct codes of the refuse-class findings
// in a set, in first-seen order.
func blockingFindingCodes(findings []patterns.FitFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, f := range findings {
		if f.Action != "refuse" || seen[f.Code] {
			continue
		}
		seen[f.Code] = true
		codes = append(codes, f.Code)
	}
	return codes
}

// resolveAutoLayout picks a layout for a slide that declared none.
//
// A template that lacks the role a slide needs used to fail the WHOLE deck at
// the first such slide, reporting the INTERNAL coerced slide type ("diagram")
// rather than what the author wrote, and offering no remediation. A pattern /
// shape_grid / compose slide needs only a canvas, and any template's Blank or
// Blank+Title layout provides one, so such a slide falls back to that canvas
// with a warning instead of refusing the deck (go-slide-creator-9svyz).
//
// It returns the resolved layout ID, the selection confidence, a warning when
// the fallback was taken, and an error only when nothing can host the slide.
func resolveAutoLayout(req layout.SelectionRequest, slide SlideInput, hasComposition bool, layouts []types.LayoutMetadata, slideIdx int) (string, float64, string, error) {
	result, err := layout.SelectLayout(req)
	if err == nil {
		return result.LayoutID, result.Confidence, "", nil
	}

	if hasComposition {
		if fallbackID, ok := compositionFallbackLayoutID(layouts); ok {
			warning := fmt.Sprintf(
				"slide %d: no layout matches slide_type %q, so its %s content was placed on the %q canvas layout instead — this template declares no better-suited layout",
				slideIdx+1, authoredSlideType(slide), compositionKind(slide), fallbackID)
			return fallbackID, 0, warning, nil
		}
	}

	return "", 0, "", fmt.Errorf(
		"slide %d: no layout in this template can host slide_type %q: %w",
		slideIdx+1, authoredSlideType(slide), err)
}

// compositionFallbackLayoutID returns the layout a pattern / shape_grid /
// compose slide can always be placed on: the title canvas when the template has
// one (so the slide keeps its heading), else the bare Blank layout.
func compositionFallbackLayoutID(layouts []types.LayoutMetadata) (string, bool) {
	for _, canonical := range []string{"blank-title", "blank-canvas", "blank"} {
		if id, ok := layout.ResolveCanonicalLayoutID(canonical, layouts); ok && id != canonical {
			return id, true
		}
	}
	return "", false
}

// authoredSlideType returns the slide_type the AUTHOR wrote, so an error names
// their input rather than the type the engine coerced it to internally (a
// pattern slide authored as "content" was reported as "diagram").
func authoredSlideType(slide SlideInput) string {
	if slide.SlideType != "" {
		return slide.SlideType
	}
	return "(unset)"
}

// compositionKind names which composition surface a slide carries, for warning
// messages.
func compositionKind(slide SlideInput) string {
	switch {
	case slide.Pattern != nil:
		return "pattern \"" + slide.Pattern.Name + "\""
	case slide.Compose != nil:
		return "compose"
	case slide.ShapeGrid != nil:
		return "shape_grid"
	default:
		return "composition"
	}
}
