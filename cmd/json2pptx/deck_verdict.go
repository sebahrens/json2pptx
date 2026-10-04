package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/emoji"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// One verdict (go-slide-creator-9k5fh).
//
// validate, generate --dry-run, validate_input and generate each decided on
// their own whether a deck is structurally acceptable: validation ran a list
// of checks, generation ran its slide conversion, and the two lists were kept
// alike by hand. They drifted both ways — validation passed a slide carrying
// both a pattern and a shape_grid, an overlay of an unknown kind and an anchor
// to a cell that is not there, all of which generation refused; generation
// rendered a content item without a placeholder_id that validation rejected.
//
// Both sides now read the same three steps:
//
//   - deckStructuralChecks — validation's slide checks and the chart dry
//     render. Validation reports them; generation runs them before it converts
//     anything and refuses the deck on the same findings
//     (deckStructuralRefusal), so nothing validation rejects is rendered.
//   - convertPresentationSlidesEach — generation's slide conversion.
//     Generation stops at its first refusal; validation runs the same function
//     (without its media) and reports every refusal, so a slide generation
//     refuses is one validation predicted.
//   - generator.Generate — the generator itself (go-slide-creator-ni65k).
//     What it decides while it prepares and writes the slides was generation's
//     alone: a second block for a placeholder was dropped after validation had
//     called the deck valid, forty bullets were refused for the paragraphs the
//     fit would trim. Validation now runs it for a deck the first two steps
//     accept, with nothing written (generationRefusals), and reports what it
//     refuses; a render refuses the same deck before it writes a file.
//
// One function turns a conversion refusal into findings
// (conversionRefusalDiagnostics) and one a generator refusal
// (generationRefusalDiagnostics), so every surface names the same code at the
// same path.

// deckRefusal is the error generation returns for a deck the structural
// verdict rejects. It carries the findings validation reports for the same
// deck, so every surface that reports the failure names the same codes and
// paths.
type deckRefusal struct {
	Diagnostics []diagnostics.Diagnostic
	// summary, when set, is the human wording of the whole refusal (the CLI's
	// multi-line design-mode message); otherwise the findings' messages are
	// joined.
	summary string
	// cause is the error generation's own conversion refused the deck with,
	// when it refused it too. Callers that read a refusal's type (a pattern
	// that does not fit its region, text below the readable floor) find it
	// in the chain.
	cause error
}

func (e *deckRefusal) Unwrap() error { return e.cause }

func (e *deckRefusal) Error() string {
	if e.summary != "" {
		return e.summary
	}
	if e.cause != nil {
		return "invalid slide specification: " + e.cause.Error()
	}
	msgs := make([]string, 0, len(e.Diagnostics))
	for _, d := range e.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	return "invalid slide specification: " + strings.Join(msgs, "; ")
}

// newDeckRefusal returns a deckRefusal for the error-severity findings in ds,
// or nil when there are none.
func newDeckRefusal(summary string, ds []diagnostics.Diagnostic) error {
	errs := diagnostics.FilterBySeverity(ds, diagnostics.SeverityError)
	if len(errs) == 0 {
		return nil
	}
	return &deckRefusal{Diagnostics: errs, summary: summary}
}

// slideRefusedError says which slide the conversion refused. The message and
// the chain are the refusal's own.
type slideRefusedError struct {
	slideIdx int
	err      error
}

func (e *slideRefusedError) Error() string { return e.err.Error() }
func (e *slideRefusedError) Unwrap() error { return e.err }

// refusalDiagnostics returns the located findings an error carries: a deck
// refusal's own, or those of the slide the conversion refused. nil means the
// error has only its message and code.
func refusalDiagnostics(err error) (ds []diagnostics.Diagnostic) {
	// A typed-nil error in the chain panics in its own Unwrap when the chain
	// is walked; such an error carries no findings.
	defer func() {
		if recover() != nil {
			ds = nil
		}
	}()
	var refusal *deckRefusal
	if errors.As(err, &refusal) {
		return refusal.Diagnostics
	}
	var refused *slideRefusedError
	if errors.As(err, &refused) {
		return conversionRefusalDiagnostics(refused.slideIdx, refused.err)
	}
	if ds := generationRefusalDiagnostics(err); len(ds) > 0 {
		return ds
	}
	return slidePatternInputDiagnostics(err)
}

// generationRefusalDiagnostics turns the error the generator refused a
// converted deck with into findings — the one reading of it every surface
// reports (go-slide-creator-ni65k):
//
//   - author content the slide would drop (a second block for a placeholder
//     that renders one, a block whose placeholder the layout does not have) is
//     one CONTENT_DROPPED finding per dropped block, at the block;
//   - two blocks that reach one placeholder through a fallback are
//     CONTENT_DROPPED at the second;
//   - source the written text would lose or shrink below the readable floor
//     is the fit finding the refusal wraps.
//
// nil means the error is not a refusal of the deck's content.
func generationRefusalDiagnostics(err error) []diagnostics.Diagnostic {
	var drop *generator.ContentDropRefusal
	if errors.As(err, &drop) && drop != nil {
		ds := refuseFindingDiagnostics(drop.Findings)
		// The finding names the block; a reader of the message alone (the
		// CLI's human output) needs the slide too.
		for i := range ds {
			if idx := slidepath.SlideIndex(ds[i].Path); idx >= 0 {
				ds[i].Message = fmt.Sprintf("slide %d: %s", idx+1, ds[i].Message)
			}
		}
		return ds
	}
	var collision *generator.PlaceholderCollisionError
	if errors.As(err, &collision) && collision != nil {
		return []diagnostics.Diagnostic{{
			Code:     patterns.ErrCodeContentDropped,
			Path:     slidepath.ContentIndex(collision.SlideIndex, collision.SecondIndex),
			Message:  collision.Error(),
			Severity: diagnostics.SeverityError,
		}}
	}
	if loss := generationRefusal(err); loss != nil {
		return refuseFindingDiagnostics([]patterns.FitFinding{{ValidationError: *loss, Action: "refuse"}})
	}
	return nil
}

// contentDropRefusalDiagnostics are the findings of a generation refusal that
// is about dropped content (a block the slide has no free placeholder for),
// nil for any other error.
func contentDropRefusalDiagnostics(err error) []diagnostics.Diagnostic {
	var drop *generator.ContentDropRefusal
	var collision *generator.PlaceholderCollisionError
	if errors.As(err, &drop) || errors.As(err, &collision) {
		return generationRefusalDiagnostics(err)
	}
	return nil
}

// deckGenerationRequest completes the request a converted deck is generated
// with: the slides and the footer, chrome, appendix labels and theme override
// the deck asks for. base carries what the caller renders with — the template,
// the output path, the SVG and fit settings. Every render builds its request
// here, and so does validation's no-write run, so the two generate the same
// deck.
func deckGenerationRequest(input *PresentationInput, specs []generator.SlideSpec, layouts []types.LayoutMetadata, base generator.GenerationRequest) generator.GenerationRequest {
	req := base
	req.Slides = specs
	req.ExcludeTemplateSlides = true
	req.ViewingMode = input.ViewingMode
	req.Footer = footerConfigForInput(input, len(specs))
	if input.Chrome != nil {
		applyChromeSkip(specs, input.Chrome, input.Slides, layouts)
		applyChromeTracker(specs, input.Chrome, input.Slides, layouts)
		applyChromeSectionCrumb(req.Footer, specs, input.Chrome, input.Slides, layouts)
	}
	applyAppendixPageLabels(req.Footer, input.Slides, layouts)
	if input.ThemeOverride != nil {
		req.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}
	return req
}

// generationRefusals is the last part of the verdict: what the generator
// itself refuses in a deck the checks accept and the conversion converted. It
// runs generation — slide preparation, the placeholder resolver, text and
// table fitting, the write phases — with nothing written (NoWrite), so a deck
// it refuses is one a render refuses, with the same findings: a content block
// the resolver would drop, text or table rows the fit would lose.
//
// nil when generation accepts the deck, or when the analysis names no
// template file to generate from.
func generationRefusals(input *PresentationInput, analysis *types.TemplateAnalysis, specs []generator.SlideSpec) []diagnostics.Diagnostic {
	if analysis.TemplatePath == "" || len(specs) == 0 {
		return nil
	}
	theme := analysis.Theme
	if input.ThemeOverride != nil {
		theme, _ = theme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
	}
	var synthetic map[string][]byte
	if analysis.Synthesis != nil {
		synthetic = analysis.Synthesis.SyntheticFiles
	}
	req := deckGenerationRequest(input, specs, analysis.Layouts, generator.GenerationRequest{
		TemplatePath:         analysis.TemplatePath,
		SyntheticFiles:       synthetic,
		StrictFit:            "warn",
		DataPalette:          resolveDataPalette(analysis.Metadata, theme.Colors),
		NoWrite:              true,
		RefuseDroppedContent: true,
	})
	_, err := generator.Generate(context.Background(), req)
	if err == nil {
		return nil
	}
	if loss := generationRefusal(err); loss != nil {
		if _, location := locateRefusal(input, specs, analysis.Layouts, nil, loss); location != "" {
			loss.Message = location + ": " + loss.Message
		}
	}
	if ds := generationRefusalDiagnostics(err); len(ds) > 0 {
		return ds
	}
	// Any other failure is generation's own; a render reports it the same way.
	return []diagnostics.Diagnostic{{
		Code:     string(diagnostics.CodeGenerationFailed),
		Message:  "generation failed: " + err.Error(),
		Severity: diagnostics.SeverityError,
	}}
}

// conversionRefusalDiagnostics turns the error slide conversion refused slide
// slideIdx with into findings — the one reading of it that validation and
// generation both report:
//
//   - a pattern's input failure is one finding per field;
//   - a refusal that is a fit verdict (a pattern or native diagram that does
//     not fit its region, table rows that would be dropped, text below the
//     readable floor) is the fit finding it wraps, the one the fit report
//     predicts;
//   - anything else is one finding with the code and the field the refusing
//     site named.
func conversionRefusalDiagnostics(slideIdx int, err error) []diagnostics.Diagnostic {
	if fit := conversionFitRefusals(err); len(fit) > 0 {
		return refuseFindingDiagnostics(fit)
	}
	if ds := slidePatternInputDiagnostics(err); len(ds) > 0 {
		return ds
	}
	var spe *slidePatternError
	isPattern := errors.As(err, &spe)

	code := cliErrorCode(err)
	path := cliErrorPath(err)
	if isPattern {
		if code == diagnostics.CodeInternal {
			code = diagnostics.CodePatternError
		}
		if path == "" {
			path = slidepath.SlideField(spe.slideIdx, spe.field)
		}
	}
	if code == diagnostics.CodeInternal {
		code = diagnostics.CodeInvalidSlide
	}
	if path == "" {
		path = slidepath.Slide(slideIdx)
	}
	return []diagnostics.Diagnostic{{
		Code: code, Path: path, Message: err.Error(), Severity: diagnostics.SeverityError,
	}}
}

// conversionFitRefusals returns the fit findings a conversion refusal wraps,
// nil when the refusal is not a fit verdict: a pattern that does not fit the
// area it was given, a native diagram too small for its region, table rows
// the grid would drop, text below the readable floor.
func conversionFitRefusals(err error) []patterns.FitFinding {
	var spe *slidePatternError
	if errors.As(err, &spe) {
		if area := patternAreaRefusals(spe.err, slidepath.SlideField(spe.slideIdx, spe.field)); len(area) > 0 {
			return area
		}
	}
	var capacity *generator.NativeRegionCapacityError
	if errors.As(err, &capacity) && capacity != nil {
		return []patterns.FitFinding{capacity.Finding}
	}
	if loss := generationRefusal(err); loss != nil {
		return []patterns.FitFinding{{ValidationError: *loss, Action: "refuse"}}
	}
	return nil
}

// refuseFindingDiagnostics converts fit findings that refuse the deck into
// error findings.
func refuseFindingDiagnostics(fs []patterns.FitFinding) []diagnostics.Diagnostic {
	ds := diagnostics.FromFitFindings(fs)
	for i := range ds {
		ds[i].Severity = diagnostics.SeverityError
	}
	return ds
}

// slideConversionRefusals runs generation's slide conversion over the deck and
// returns what it refuses: per slide index, the findings of that slide's
// refusal; under -1, a deck-level one (the rhythm grid). The conversion is the
// function generation calls, in the geometry, theme and accent strategy it
// renders in; only the media is left out (ShapesOnly) — image-rendered
// diagrams and icons, which the asset and diagram checks cover and which need
// files validation may not have resolved.
//
// keep, when set, restricts the result to the refusals it accepts.
//
// specs are the converted slides when the conversion refused none of them
// (whatever keep reports), nil otherwise: the deck generation would go on to
// generate.
func slideConversionRefusals(input *PresentationInput, analysis *types.TemplateAnalysis, keep func(err error) bool) (out map[int][]diagnostics.Diagnostic, specs []generator.SlideSpec) {
	if input == nil || analysis == nil || len(input.Slides) == 0 {
		return nil, nil
	}
	out = map[int][]diagnostics.Diagnostic{}
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		if refusal := rhythmGridRefusal(input.Grid); refusal != nil {
			out[-1] = refusalDiagnostics(refusal)
			return out, nil
		}
		rhythmGrid = resolveGrid(input.Grid, analysis.Layouts, analysis.SlideWidth, analysis.SlideHeight)
	}
	theme := analysis.Theme
	if input.ThemeOverride != nil {
		theme, _ = theme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
	}
	diagCtx := &GridDiagramContext{
		ThemeColors: theme.Colors,
		DataPalette: resolveDataPalette(analysis.Metadata, theme.Colors),
		FontFamily:  theme.BodyFont,
		TitleFont:   theme.TitleFont,
		ViewingMode: tokens.ParseViewingMode(input.ViewingMode),
		ShapesOnly:  true,
	}
	refused := 0
	specs, _, _, _ = convertPresentationSlidesEach(input.Slides, analysis.Layouts, analysis.SlideWidth, analysis.SlideHeight,
		analysis.Metadata, rhythmGrid, patterns.AccentStrategy(input.AccentStrategy), diagCtx, false,
		func(slideIdx int, err error) bool {
			refused++
			if keep == nil || keep(err) {
				out[slideIdx] = conversionRefusalDiagnostics(slideIdx, err)
			}
			return true
		})
	if refused > 0 {
		specs = nil
	}
	return out, specs
}

// rhythmGridRefusal is the refusal for a deck-level grid block whose values
// are out of range, nil when the block is usable.
func rhythmGridRefusal(cfg *GridConfig) error {
	err := validateGridConfig(cfg)
	if err == nil {
		return nil
	}
	return &deckRefusal{Diagnostics: []diagnostics.Diagnostic{{
		Code: diagnostics.CodeInvalidGrid, Path: "/grid",
		Message: "grid: " + err.Error(), Severity: diagnostics.SeverityError,
	}}, summary: "grid: " + err.Error()}
}

// deckStructuralChecks is the half of the verdict that is a list of checks:
// validation's slide checks against the template, and the dry render of each
// chart and diagram. output is filled as validateSlidesAgainstTemplate fills
// it. The caller has resolved canonical layout names.
func deckStructuralChecks(output *dryRunOutput, input *PresentationInput, analysis *types.TemplateAnalysis) {
	validateSlidesAgainstTemplate(output, input.Slides, analysis)

	// A chart or diagram that cannot render leaves the deck without its
	// visual: no surface accepts that deck (go-slide-creator-gr64x). The CLI
	// used to find out after writing the file, generate_presentation not at
	// all.
	if failed := unrenderableDiagramFindings(input, analysis); len(failed) > 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, refuseFindingDiagnostics(failed)...)
	}
}

// deckStructuralDiagnostics is the structural verdict on a deck against its
// template, as validation reports it: deckStructuralChecks, then every slide
// generation's conversion refuses that those checks did not already reject,
// then — for a deck both accept — what the generator itself refuses
// (generationRefusals).
func deckStructuralDiagnostics(output *dryRunOutput, input *PresentationInput, analysis *types.TemplateAnalysis) {
	specs := deckStructuralDiagnosticsWhere(output, input, analysis, nil)
	if specs == nil || diagnostics.HasErrors(output.Diagnostics) {
		return
	}
	if refused := generationRefusals(input, analysis, specs); len(refused) > 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, refused...)
	}
}

// deckStructuralDiagnosticsWhere is the checks and the conversion refusals
// keep accepts (nil keeps all). It returns the converted slides when the
// conversion refused none.
func deckStructuralDiagnosticsWhere(output *dryRunOutput, input *PresentationInput, analysis *types.TemplateAnalysis, keep func(err error) bool) []generator.SlideSpec {
	deckStructuralChecks(output, input, analysis)

	refusals, specs := slideConversionRefusals(input, analysis, keep)
	if len(refusals) == 0 {
		return specs
	}
	// A slide the checks already reject keeps their findings: they name the
	// field and carry a fix, and generation refuses on them first.
	rejected := map[int]bool{}
	for _, d := range output.Diagnostics {
		if d.Severity == diagnostics.SeverityError {
			rejected[slidepath.SlideIndex(d.Path)] = true
		}
	}
	idxs := make([]int, 0, len(refusals))
	for i := range refusals {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	for _, i := range idxs {
		if i >= 0 && rejected[i] {
			continue
		}
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, refusals[i]...)
	}
	return specs
}

// deckStructuralRefusal is generation's side of the verdict: the error for a
// deck deckStructuralChecks rejects, nil for one they accept. Generation calls
// it with the layouts, theme and metadata it rendered with and the error its
// conversion returned (nil when it converted the deck). A deck the checks
// accept keeps the conversion's own refusal, the one
// deckStructuralDiagnostics predicts.
func deckStructuralRefusal(input *PresentationInput, layouts []types.LayoutMetadata, theme types.ThemeInfo, metadata *types.TemplateMetadata, slideWidth, slideHeight int64, convErr error) error {
	out := dryRunOutput{Valid: true, verdictOnly: true}
	deckStructuralChecks(&out, input, &types.TemplateAnalysis{
		Layouts: layouts, Theme: theme, Metadata: metadata,
		SlideWidth: slideWidth, SlideHeight: slideHeight,
	})
	if out.Valid {
		return nil
	}
	errs := diagnostics.FilterBySeverity(out.Diagnostics, diagnostics.SeverityError)
	if len(errs) == 0 {
		return nil
	}
	return &deckRefusal{Diagnostics: errs, cause: convErr}
}

// deckPolicyDiagnostics are the deck-content refusals that need no template:
// enum values outside their set, design-mode violations and emoji in text.
// Every surface that accepts a deck reports them from here.
func deckPolicyDiagnostics(input *PresentationInput) []diagnostics.Diagnostic {
	var ds []diagnostics.Diagnostic
	if enumErrs := checkInputEnumValues(input); len(enumErrs) > 0 {
		ds = append(ds, diagnostics.FromValidationErrors(enumErrs)...)
	}
	if violations := validateDesignMode(input); len(violations) > 0 {
		ds = append(ds, designModeDiagnostics(violations)...)
	}
	if emojiViolations := emoji.ValidateNoEmojiInText(input); len(emojiViolations) > 0 {
		ds = append(ds, noEmojiDiagnostics(emojiViolations)...)
	}
	return ds
}

// deckTemplateFieldDiagnostics reports a deck that names no template, or names
// one twice (template and template_path). requiredMessage is the wording of
// the first case on the calling surface.
func deckTemplateFieldDiagnostics(input *PresentationInput, requiredMessage string) []diagnostics.Diagnostic {
	switch {
	case input.Template == "" && input.TemplatePath == "":
		return []diagnostics.Diagnostic{{
			Code: "REQUIRED", Path: "/template", Message: requiredMessage,
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
		}}
	case input.Template != "" && input.TemplatePath != "":
		return []diagnostics.Diagnostic{{
			Code: diagnostics.CodeAmbiguousInput, Path: "/template_path",
			Message:  "set only one of template (a registered name) or template_path (a local .pptx), not both",
			Severity: diagnostics.SeverityError,
		}}
	}
	return nil
}

// deckSlidesRequiredDiagnostic is the finding for a deck with no slides.
func deckSlidesRequiredDiagnostic() diagnostics.Diagnostic {
	return diagnostics.Diagnostic{
		Code: "REQUIRED", Path: "/slides", Message: "at least one slide is required",
		Severity: diagnostics.SeverityError,
		Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
	}
}

// assetRefusal is the refusal for a deck whose icon, image or URL references
// cannot be resolved: the findings validation reports for them, under the
// aggregate message the CLI prints. nil when no finding is an error.
func assetRefusal(findings []diagnostics.Diagnostic) error {
	err := iconFindingsToError(findings)
	if err == nil {
		return nil
	}
	return newDeckRefusal(err.Error(), findings)
}
