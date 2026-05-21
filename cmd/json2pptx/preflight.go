package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Preflight stage names. Every finding preflight emits is tagged with the
// stage that produced it (carried in evidence.stage), and the envelope's
// findings are ordered by stage then severity. The stages run in a fixed
// fail-fast order documented in docs/AGENT_DIAGNOSTICS.md section 3: a stage
// whose failure makes later stages impossible — an unparseable deck, a missing
// required field, or a template that will not load — short-circuits the
// remaining stages. Content-policy findings do NOT short-circuit; preflight is
// the "run every static check" surface and reports the full picture in one pass.
const (
	preflightStageInput       = "INPUT"
	preflightStagePolicy      = "POLICY"
	preflightStageTemplate    = "TEMPLATE"
	preflightStageLayout      = "LAYOUT"
	preflightStagePlaceholder = "PLACEHOLDER"
	preflightStageGrid        = "GRID"
	preflightStagePattern     = "PATTERN"
	preflightStageRender      = "RENDER_PROJECTION"
)

// preflightStageRank gives the catalog order of the stages, used to sort the
// envelope's findings by stage (then by severity within a stage).
var preflightStageRank = map[string]int{
	preflightStageInput:       0,
	preflightStagePolicy:      1,
	preflightStageTemplate:    2,
	preflightStageLayout:      3,
	preflightStagePlaceholder: 4,
	preflightStageGrid:        5,
	preflightStagePattern:     6,
	preflightStageRender:      7,
}

// preflightExitInternal is the exit code preflight uses when it fails
// internally (e.g. it computed findings but cannot serialize them). It is
// distinct from the validation-failed code (2) so an agent can tell "the deck
// has errors" apart from "preflight itself broke".
const preflightExitInternal = 3

// preflightOptions configures a preflight run.
type preflightOptions struct {
	templatesDir string
	strict       bool
}

// runPreflight implements the "preflight" subcommand: a single static checker
// that runs every stage-based check on a deck JSON without writing a PPTX. It
// emits the unified FindingEnvelope (subcommand "preflight") to stdout and
// exits 0 when clean, 2 when any error finding is present (or, under --strict,
// any warning), and a distinct non-zero code on an internal failure.
func runPreflight() error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	jsonInput := fs.String("json", "", "Path to deck JSON input (use - for stdin)")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	strict := fs.Bool("strict", false, "Treat warnings as failures (exit 2 when any warning is present)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preflight --json <file.json> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Run every static check on a deck JSON without generating a PPTX.\n")
		fmt.Fprintf(os.Stderr, "Stages run in order: INPUT, POLICY, TEMPLATE, LAYOUT, PLACEHOLDER,\n")
		fmt.Fprintf(os.Stderr, "GRID, PATTERN, RENDER_PROJECTION. Each finding carries evidence.stage.\n\n")
		fmt.Fprintf(os.Stderr, "Output: the unified FindingEnvelope (JSON) on stdout. Exit codes:\n")
		fmt.Fprintf(os.Stderr, "  0  no error findings (and, under --strict, no warnings)\n")
		fmt.Fprintf(os.Stderr, "  2  at least one error finding (or any warning under --strict)\n")
		fmt.Fprintf(os.Stderr, "  3  internal failure\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preflight --json deck.json --templates-dir ./templates\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preflight --json deck.json --templates-dir ./templates --strict\n")
		fmt.Fprintf(os.Stderr, "  cat deck.json | json2pptx preflight --json -\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Accept the deck path either via --json or as a positional argument.
	path := *jsonInput
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		fs.Usage()
		return fmt.Errorf("preflight: deck JSON is required: use --json <file.json> or --json - for stdin")
	}

	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		// A read failure is reported through the same envelope shape as every
		// other failure path so an agent never has to special-case it.
		env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "preflight"}, []diagnostics.Diagnostic{{
			Code:     diagnostics.CodeReadFailed,
			Message:  fmt.Sprintf("failed to read JSON input: %v", err),
			Severity: diagnostics.SeverityError,
			Details:  map[string]any{"stage": preflightStageInput},
		}})
		_ = printPreflightEnvelope(env)
		os.Exit(2)
	}

	env := runPreflightCore(data, preflightOptions{templatesDir: *templatesDir, strict: *strict})
	if perr := printPreflightEnvelope(env); perr != nil {
		fmt.Fprintf(os.Stderr, "preflight: failed to write output: %v\n", perr)
		os.Exit(preflightExitInternal)
	}
	os.Exit(preflightExitCode(env, *strict))
	return nil // unreachable; os.Exit does not return.
}

// runPreflightCore runs every static-check stage on a deck JSON and returns the
// unified FindingEnvelope. It is the shared, side-effect-free core (no stdout,
// no os.Exit) so the CLI subcommand and unit tests exercise the same code. The
// only I/O it performs is reading the resolved template.
func runPreflightCore(inputData []byte, opts preflightOptions) diagnostics.FindingEnvelope {
	acc := &preflightAccumulator{}
	sha := diagnostics.ComputeInputSHA256(inputData)

	// STAGE 1: INPUT — parse the deck (patch envelopes resolved up front).
	input, parseErr := parsePreflightInput(inputData)
	if parseErr != nil {
		acc.add(preflightStageInput, *parseErr)
		return acc.envelope(sha, "", opts)
	}

	applyDefaults(input)
	resolveInputNamedSettingsForDir(opts.templatesDir, input)

	// Structure block → flat slides; unknown keys (warn); enum values (error).
	acc.add(preflightStageInput, applyStructureExpansion(input)...)
	for _, ve := range checkInputUnknownKeys(inputData) {
		acc.add(preflightStageInput, diagnostics.FromValidationWarning(ve))
	}
	for _, ve := range checkInputEnumValues(input) {
		acc.add(preflightStageInput, diagnostics.FromValidationError(ve))
	}
	acc.add(preflightStageInput, requiredTopLevelDiags(input)...)

	// STAGE 2: POLICY — design-mode constraints and the no-emoji content policy.
	acc.add(preflightStagePolicy, designModeDiagnostics(validateDesignMode(input))...)
	acc.add(preflightStagePolicy, noEmojiDiagnostics(ValidateNoEmojiInText(input))...)

	// Fail-fast: a deck missing a template (or slides) cannot resolve a
	// template or validate layouts, so stop after INPUT/POLICY. Policy findings
	// alone do not short-circuit — they ride alongside the rest of the report.
	if acc.hasInputErrors() {
		return acc.envelope(sha, input.Template, opts)
	}

	// STAGE 3: TEMPLATE — resolve + analyze (loads layouts, canonical roles).
	templatePath, cleanup, err := resolveTemplatePath(input.Template, opts.templatesDir)
	if err != nil {
		acc.add(preflightStageTemplate, diagnostics.Diagnostic{
			Code:     diagnostics.CodeTemplateNotFound,
			Path:     "template",
			Message:  templateNotFoundError(input.Template, opts.templatesDir),
			Severity: diagnostics.SeverityError,
		})
		return acc.envelope(sha, input.Template, opts)
	}
	defer cleanup()

	cache := template.NewMemoryCache(24 * time.Hour)
	analysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		acc.add(preflightStageTemplate, diagnostics.Diagnostic{
			Code:     diagnostics.CodeTemplateError,
			Path:     "template",
			Message:  fmt.Sprintf("template analysis failed: %v", err),
			Severity: diagnostics.SeverityError,
		})
		return acc.envelope(sha, input.Template, opts)
	}

	// STAGE 4-6: LAYOUT / PLACEHOLDER / GRID — slide-vs-template validation.
	// One producer emits all three; classifySlideValidationStage routes each
	// finding to its stage by code/path.
	resolveCanonicalLayoutIDs(input.Slides, analysis.Layouts)
	var slideOut dryRunOutput
	validateSlidesAgainstTemplate(&slideOut, input.Slides, analysis)
	acc.addClassified(slideOut.Diagnostics, classifySlideValidationStage)

	// STAGE 7: PATTERN — expand each pattern/compose slide; capture resolution
	// errors (unknown pattern, invalid values, unpopulated required slots) that
	// the slide-vs-template pass cannot see.
	acc.add(preflightStagePattern, collectPatternResolutionDiags(input, analysis)...)

	// STAGE 5/6/8: per-cell fit, grid geometry, and render-projection findings.
	// collectFitFindings shares the layout-aware ContentZone resolver with
	// generation; classifyFitStage routes each finding to PLACEHOLDER, GRID, or
	// RENDER_PROJECTION.
	fit := collectFitFindings(input, analysis.Layouts, analysis.SlideWidth, analysis.SlideHeight, &analysis.Theme)
	fit = BudgetFitFindings(fit, DefaultFindingBudget, false)
	acc.addClassified(diagnostics.FromFitFindings(fit), classifyFitStage)

	return acc.envelope(sha, input.Template, opts)
}

// parsePreflightInput unmarshals deck JSON, resolving a patch envelope to its
// effective presentation first. It returns either a *PresentationInput or a
// single INPUT-stage error diagnostic describing the parse failure.
func parsePreflightInput(inputData []byte) (*PresentationInput, *diagnostics.Diagnostic) {
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(inputData, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, perr := applyPresentationPatch(patchInput)
		if perr != nil {
			return nil, &diagnostics.Diagnostic{
				Code:     diagnostics.CodeInvalidParameter,
				Message:  fmt.Sprintf("failed to apply patch: %v", perr),
				Severity: diagnostics.SeverityError,
			}
		}
		return patched, nil
	}

	var input PresentationInput
	if err := json.Unmarshal(inputData, &input); err != nil {
		return nil, &diagnostics.Diagnostic{
			Code:     diagnostics.CodeInvalidJSON,
			Message:  fmt.Sprintf("failed to parse JSON: %v", err),
			Severity: diagnostics.SeverityError,
		}
	}
	return &input, nil
}

// requiredTopLevelDiags reports the missing required top-level fields (template,
// at least one slide) as INPUT-stage errors with a provide_value fix.
func requiredTopLevelDiags(input *PresentationInput) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	if input.Template == "" {
		out = append(out, diagnostics.Diagnostic{
			Code:     patterns.ErrCodeRequired,
			Path:     "template",
			Message:  "template is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
		})
	}
	if len(input.Slides) == 0 {
		out = append(out, diagnostics.Diagnostic{
			Code:     patterns.ErrCodeRequired,
			Path:     "slides",
			Message:  "at least one slide is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
		})
	}
	return out
}

// collectPatternResolutionDiags expands every slide-level pattern and compose
// envelope and reports expansion failures as PATTERN-stage errors. It uses a
// minimal ExpandContext (the structural resolvers only need slide geometry and
// the template theme/metadata), mirroring expandComposeForPreflight.
func collectPatternResolutionDiags(input *PresentationInput, analysis *types.TemplateAnalysis) []diagnostics.Diagnostic {
	reg := patterns.Default()
	var out []diagnostics.Diagnostic
	for i := range input.Slides {
		s := &input.Slides[i]
		if s.Pattern == nil && s.Compose == nil {
			continue
		}
		ctx := patterns.ExpandContext{
			Theme:       analysis.Theme,
			Metadata:    analysis.Metadata,
			SlideWidth:  analysis.SlideWidth,
			SlideHeight: analysis.SlideHeight,
			SlideIndex:  i,
		}
		switch {
		case s.Pattern != nil:
			if _, _, err := expandPattern(s.Pattern, ctx, reg); err != nil {
				out = append(out, diagnostics.Diagnostic{
					Code:     diagnostics.CodePatternError,
					Path:     slidepath.SlideField(i, "pattern"),
					Message:  fmt.Sprintf("slide %d: %v", i+1, err),
					Severity: diagnostics.SeverityError,
				})
			}
		case s.Compose != nil:
			if _, _, err := expandCompose(s.Compose, ctx, reg); err != nil {
				out = append(out, diagnostics.Diagnostic{
					Code:     diagnostics.CodePatternError,
					Path:     slidepath.SlideField(i, "compose"),
					Message:  fmt.Sprintf("slide %d: %v", i+1, err),
					Severity: diagnostics.SeverityError,
				})
			}
		}
	}
	return out
}

// classifySlideValidationStage routes a diagnostic produced by
// validateSlidesAgainstTemplate to its preflight stage. Layout-resolution
// findings map to LAYOUT, shape-grid structural findings to GRID, and every
// content/placeholder finding to PLACEHOLDER.
func classifySlideValidationStage(d diagnostics.Diagnostic) string {
	switch d.Code {
	case patterns.ErrCodeUnknownLayoutID:
		return preflightStageLayout
	case diagnostics.CodeInvalidGrid:
		return preflightStageGrid
	case patterns.ErrCodeRequired:
		if strings.Contains(d.Path, "layout_id") {
			return preflightStageLayout
		}
		return preflightStagePlaceholder
	default:
		return preflightStagePlaceholder
	}
}

// classifyFitStage routes a fit finding (already converted to a Diagnostic) to
// its preflight stage. Rendered-geometry collisions map to RENDER_PROJECTION,
// shape-grid layout findings to GRID, pattern-fill findings to PATTERN, and
// every remaining content-fit finding to PLACEHOLDER.
func classifyFitStage(d diagnostics.Diagnostic) string {
	switch d.Code {
	case patterns.ErrCodeFooterCollision, patterns.ErrCodeTitleCollision, patterns.ErrCodeTitleWraps:
		return preflightStageRender
	case patterns.ErrCodeSlideBoundsOverflow, patterns.ErrCodeSparseLayout,
		patterns.ErrCodeCellUnderfilled, patterns.ErrCodeGridDiagramNarrow,
		patterns.ErrCodeMixedFillScheme, patterns.ErrCodeDividerTooThin,
		patterns.ErrCodeStackedTables, patterns.ErrCodeAccentOverload,
		patterns.ErrCodeContrastPredicted, patterns.ErrCodeColumnWidthDeficit,
		patterns.ErrCodeDensityExceeded:
		return preflightStageGrid
	case patterns.ErrCodePatternUnderfilled, patterns.ErrCodePatternOvercrowded,
		patterns.ErrCodeWrongPattern:
		return preflightStagePattern
	default:
		return preflightStagePlaceholder
	}
}

// preflightAccumulator collects stage-tagged diagnostics across the preflight
// stages and folds them into a single ordered FindingEnvelope.
type preflightAccumulator struct {
	diags []diagnostics.Diagnostic
}

// add appends diagnostics tagged with a fixed stage.
func (a *preflightAccumulator) add(stage string, ds ...diagnostics.Diagnostic) {
	for i := range ds {
		tagPreflightStage(&ds[i], stage)
		a.diags = append(a.diags, ds[i])
	}
}

// addClassified appends diagnostics whose stage is derived per-finding by
// classify (used for producers that emit findings spanning multiple stages).
func (a *preflightAccumulator) addClassified(ds []diagnostics.Diagnostic, classify func(diagnostics.Diagnostic) string) {
	for i := range ds {
		tagPreflightStage(&ds[i], classify(ds[i]))
		a.diags = append(a.diags, ds[i])
	}
}

// hasInputErrors reports whether any INPUT-stage error has been accumulated.
func (a *preflightAccumulator) hasInputErrors() bool {
	for i := range a.diags {
		if a.diags[i].Severity != diagnostics.SeverityError {
			continue
		}
		if stageOf(a.diags[i]) == preflightStageInput {
			return true
		}
	}
	return false
}

// envelope sorts the accumulated diagnostics by stage then severity and builds
// the unified FindingEnvelope stamped with the preflight subcommand.
func (a *preflightAccumulator) envelope(sha, templateName string, _ preflightOptions) diagnostics.FindingEnvelope {
	sort.SliceStable(a.diags, func(i, j int) bool {
		si, sj := stageRankOf(a.diags[i]), stageRankOf(a.diags[j])
		if si != sj {
			return si < sj
		}
		return severityRank(a.diags[i].Severity) < severityRank(a.diags[j].Severity)
	})
	return diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "preflight",
		Template:    templateName,
		InputSHA256: sha,
	}, a.diags)
}

// tagPreflightStage records the producing stage on a diagnostic. The stage
// rides in Details so it surfaces as evidence.stage in the finding (Details
// scalars are carried into Finding.Evidence). An already-set stage is kept.
func tagPreflightStage(d *diagnostics.Diagnostic, stage string) {
	if d.Details == nil {
		d.Details = map[string]any{}
	}
	if _, ok := d.Details["stage"]; !ok {
		d.Details["stage"] = stage
	}
}

// stageOf returns the stage tag of a diagnostic, or "" when unset.
func stageOf(d diagnostics.Diagnostic) string {
	if d.Details == nil {
		return ""
	}
	if s, ok := d.Details["stage"].(string); ok {
		return s
	}
	return ""
}

// stageRankOf returns the catalog rank of a diagnostic's stage. Unknown or
// unset stages sort last so they never displace classified findings.
func stageRankOf(d diagnostics.Diagnostic) int {
	if r, ok := preflightStageRank[stageOf(d)]; ok {
		return r
	}
	return len(preflightStageRank)
}

// preflightExitCode maps an envelope to a process exit code: 0 when clean, 2
// when any error finding is present, and (under strict) 2 when any warning is
// present.
func preflightExitCode(env diagnostics.FindingEnvelope, strict bool) int {
	if !env.OK {
		return 2
	}
	if strict {
		for i := range env.Findings {
			if env.Findings[i].Severity == diagnostics.SeverityWarning {
				return 2
			}
		}
	}
	return 0
}

// printPreflightEnvelope writes the envelope as indented JSON to stdout.
func printPreflightEnvelope(env diagnostics.FindingEnvelope) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = os.Stdout.Write(data)
	return err
}
