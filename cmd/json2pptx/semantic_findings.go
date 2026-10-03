package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// One finding set and one severity model for a DeckSpec
// (go-slide-creator-x9rhq, -oh3qr, -3rn3s, -uon4b).
//
// The agent journey review found the DeckSpec loop telling three stories about
// one deck: validate_deck_spec said "no issues", render_deck_spec then blocked
// deterministic_ready on an info-level finding, and the blocking reason named
// neither the code nor the slide. The cause was structural: validate predicted
// findings from the compiled deck, render collected them from the generated
// one, and each graded them on its own scale.
//
// Everything a DeckSpec surface reports now comes from this file's pipeline:
//
//   - one collection: the diagnostics of a render (compile diagnostics, the
//     shared fit collectors, generation findings and refusals). validate runs
//     the same render into a scratch directory, so its envelope is a second
//     rendering of the same list, not a second opinion;
//   - one severity: a finding that stops deterministic_ready is an error and
//     carries blocking:true; everything else is a warning or an info and
//     never blocks. A gate criterion no single finding accounts for (the score
//     floor, the action-title share) is reported as one QUALITY_GATE finding,
//     so the gate cannot fail without an error to point at;
//   - one place for waivers: a storyline finding the deck's archetype does not
//     call for, or that meta.waivers names with a reason, is an advisory and
//     the waiver is recorded in the result.

// findingWaiver records one waived storyline finding code in a result.
type findingWaiver struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	// Source is "meta.waivers" for an authored waiver or "meta.archetype"
	// when the deck's archetype does not call for the rule.
	Source string `json:"source"`
	// Findings is how many findings this waiver turned into advisories.
	Findings int `json:"findings"`
}

// findingSymptom is one per-field finding folded under its root cause.
type findingSymptom struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
	// pointer is Path as an authored JSON Pointer, once resolved.
	pointer string
}

// findingPolicy decides, for one deck, which findings block and which are
// waived.
type findingPolicy struct {
	criteria deterministic.QualityGateCriteria
	waivers  []findingWaiver
	byCode   map[string]int
}

// newFindingPolicy builds the deck's policy from its IR: the default gate
// criteria, the authored waivers, and the archetype's implied one. A nil IR
// (a deck that did not compile) waives nothing.
func newFindingPolicy(ir *semantic.DeckIR) *findingPolicy {
	p := &findingPolicy{criteria: deterministic.DefaultQualityGateCriteria(), byCode: map[string]int{}}
	if ir == nil {
		return p
	}
	add := func(w findingWaiver) {
		if _, dup := p.byCode[w.Code]; dup {
			return
		}
		p.byCode[w.Code] = len(p.waivers)
		p.waivers = append(p.waivers, w)
	}
	for _, w := range ir.Waivers {
		code, waivable := semantic.CanonicalWaivableCode(w.Code)
		if !waivable || strings.TrimSpace(w.Reason) == "" {
			continue // reported by validateMeta; never silently honoured
		}
		add(findingWaiver{Code: code, Reason: strings.TrimSpace(w.Reason), Source: "meta.waivers"})
	}
	// An archetype that does not expect a synthesis slide (a sales pitch, a
	// roadmap, a market analysis) is not missing an executive summary. An
	// unset archetype keeps the rule: without a stated purpose the storyline
	// default applies.
	if ir.Archetype != "" && ir.Archetype.Valid() && !ir.Executive {
		add(findingWaiver{
			Code:   patterns.ErrCodeNoExecutiveSummary,
			Reason: fmt.Sprintf("archetype %q does not call for an executive summary", ir.Archetype),
			Source: "meta.archetype",
		})
	}
	return p
}

// waiverFor returns the waiver covering code, or nil.
func (p *findingPolicy) waiverFor(code string) *findingWaiver {
	if p == nil {
		return nil
	}
	if i, ok := p.byCode[code]; ok {
		return &p.waivers[i]
	}
	return nil
}

// gateFindings returns the findings the score and the quality gate judge:
// everything except what the deck waived.
func (p *findingPolicy) gateFindings(fit []patterns.FitFinding) []patterns.FitFinding {
	if p == nil || len(p.waivers) == 0 {
		return fit
	}
	out := make([]patterns.FitFinding, 0, len(fit))
	for _, f := range fit {
		if p.waiverFor(f.Code) == nil {
			out = append(out, f)
		}
	}
	return out
}

// applyWaivers turns every waived diagnostic into an advisory and counts it.
func (p *findingPolicy) applyWaivers(diags []semanticDiagnostic) {
	if p == nil {
		return
	}
	for i := range diags {
		w := p.waiverFor(diags[i].Code)
		if w == nil {
			continue
		}
		w.Findings++
		diags[i].Waived = w.Reason
		setDiagnosticSeverity(&diags[i], diagnostics.SeverityInfo)
	}
}

// recorded returns the waivers a result reports: every authored waiver, and
// the archetype's only when it actually waived something.
func (p *findingPolicy) recorded() []findingWaiver {
	if p == nil {
		return nil
	}
	var out []findingWaiver
	for _, w := range p.waivers {
		if w.Source == "meta.archetype" && w.Findings == 0 {
			continue
		}
		out = append(out, w)
	}
	return out
}

// fitFindingSeverity is the DeckSpec severity of a fit finding: error exactly
// when the default quality gate blocks on it, info otherwise.
func fitFindingSeverity(f patterns.FitFinding) (string, bool) {
	if deterministic.BlocksGate(f, deterministic.DefaultQualityGateCriteria()) {
		return string(diagnostics.SeverityError), true
	}
	return string(diagnostics.SeverityInfo), false
}

// setDiagnosticSeverity sets a diagnostic's severity and the blocking flag
// that follows from it, on both of its renderings.
func setDiagnosticSeverity(d *semanticDiagnostic, sev diagnostics.Severity) {
	d.Severity = string(sev)
	d.Blocking = sev == diagnostics.SeverityError
	if d.diag != nil {
		d.diag.Severity = sev
	}
}

// codeQualityGate is the finding reported when the quality gate fails on a
// criterion no single finding accounts for.
const codeQualityGate = patterns.ErrCodeQualityGate

// qualityGateDiagnostics reports the gate's aggregate reasons — and, as a
// safety net, any failure no blocking finding explains — as deck-level error
// findings, so a failed gate always has an error to point at.
func qualityGateDiagnostics(gate *deterministic.QualityGate, diags []semanticDiagnostic) []semanticDiagnostic {
	if gate == nil || gate.Passed {
		return nil
	}
	reasons := deterministic.AggregateGateReasons(gate)
	if len(reasons) == 0 && !hasBlockingDiagnostic(diags) {
		reasons = gate.Reasons
		if len(reasons) == 0 {
			reasons = []string{"failed"}
		}
	}
	out := make([]semanticDiagnostic, 0, len(reasons))
	for _, r := range reasons {
		msg := "quality gate: " + r
		counted := gateReasonCounted(r, diags)
		if names := gateReasonContributors(counted); names != "" {
			msg += " — " + names
		}
		// The evidence rides in Details for the validate envelope and in
		// Evidence for the render verdict: both report the same facts.
		evidence := gateReasonEvidence(r, counted)
		d := diagnostics.Diagnostic{Code: codeQualityGate, Path: "slides", Message: msg, Severity: diagnostics.SeverityError, Details: evidence}
		sd := semanticDiagFromCompile(d)
		sd.Evidence = evidence
		out = append(out, sd)
	}
	return out
}

// gateCriterion names the quality-gate criterion an aggregate reason is about.
func gateCriterion(reason string) string {
	switch {
	case strings.Contains(reason, "lack an action title"):
		return "max_topic_title_pct"
	case strings.Contains(reason, "slides carry findings"):
		return "max_problem_slides_pct"
	case strings.HasPrefix(reason, "composition "):
		return "min_composition_score"
	case strings.HasPrefix(reason, "score "):
		return "min_score"
	}
	return ""
}

// gateReasonEvidence is a QUALITY_GATE finding's evidence: the criterion that
// failed and every advisory counted against it, as {code, path} — the findings
// of this same response whose fixes clear the gate. The message names the
// first six; an agent reads them all here instead of parsing the sentence.
func gateReasonEvidence(reason string, counted []semanticDiagnostic) map[string]any {
	evidence := map[string]any{}
	if c := gateCriterion(reason); c != "" {
		evidence["criterion"] = c
	}
	if len(counted) > 0 {
		list := make([]any, 0, len(counted))
		for _, d := range counted {
			entry := map[string]any{"code": d.Code}
			if p := diagnosticPointer(d); p != "" {
				entry["path"] = p
			}
			list = append(list, entry)
		}
		evidence["counted"] = list
	}
	if len(evidence) == 0 {
		return nil
	}
	return evidence
}

// gateReasonCounted lists the advisories behind an aggregate gate reason: the
// topic titles for the action-title share, the scored fit advisories for the
// score floor and the problem-slide share.
func gateReasonCounted(reason string, diags []semanticDiagnostic) []semanticDiagnostic {
	var counted []semanticDiagnostic
	wantTitle := strings.Contains(reason, "lack an action title")
	for _, d := range diags {
		if d.Blocking || d.Waived != "" || d.Action == "" || d.Action == "info" {
			continue // not a scored fit advisory
		}
		if wantTitle != (d.Code == patterns.ErrCodeTitleNotAction) {
			continue
		}
		counted = append(counted, d)
	}
	return counted
}

// gateReasonContributors names the counted advisories in the gate's sentence.
func gateReasonContributors(counted []semanticDiagnostic) string {
	names := make([]string, 0, len(counted))
	for _, d := range counted {
		names = append(names, diagnosticName(d))
	}
	const limit = 6
	if len(names) > limit {
		names = append(names[:limit], fmt.Sprintf("+%d more", len(names)-limit))
	}
	if len(names) == 0 {
		return ""
	}
	return "advisories counted: " + strings.Join(names, ", ")
}

// diagnosticName is a finding's "CODE at path" name.
func diagnosticName(d semanticDiagnostic) string {
	if p := diagnosticPointer(d); p != "" {
		return d.Code + " at " + p
	}
	return d.Code
}

// diagnosticPointer is where a diagnostic sits in the authored spec, as a JSON
// Pointer: its DeckSpec path, or its slide when the finding could not be
// traced to a field. Reasons and gate messages name findings by it, so prose
// and the path field agree (go-slide-creator-pilpn).
func diagnosticPointer(d semanticDiagnostic) string {
	if d.address != nil {
		return d.address.Path
	}
	if d.SemanticPath != "" && !strings.HasPrefix(d.SemanticPath, "/") {
		return specPointer(d.SemanticPath)
	}
	if d.SlideIndex != nil {
		return "/slides/" + strconv.Itoa(*d.SlideIndex)
	}
	return ""
}

// isPlaceholderFinding reports whether a diagnostic is a blocking finding for
// placeholder copy the product itself emitted.
func isPlaceholderFinding(d semanticDiagnostic) bool {
	if d.Evidence == nil {
		return false
	}
	_, ok := d.Evidence[semantic.PlaceholderDetail]
	return ok && diagnosticBlocks(d)
}

func hasBlockingDiagnostic(diags []semanticDiagnostic) bool {
	for _, d := range diags {
		if diagnosticBlocks(d) {
			return true
		}
	}
	return false
}

// diagnosticBlocks reports whether a diagnostic blocks. Blocking, error
// severity and a refuse action are one fact under the severity model; a
// diagnostic built by hand may state only one of them.
func diagnosticBlocks(d semanticDiagnostic) bool {
	return d.Blocking || d.Severity == string(diagnostics.SeverityError) || d.Action == "refuse"
}

// rootCausePathRE matches the slide-level container a capacity finding is
// reported on: the slide's pattern, compose envelope or shape grid.
var rootCausePathRE = regexp.MustCompile(`^/slides/\d+/(pattern|compose|shape_grid)$`)

// groupRootCauses folds per-field readability refusals under the capacity
// finding that causes them (go-slide-creator-3rn3s). A pattern that needs
// 392pt in a 311pt content area shrinks every cell: that used to read as one
// info ("needs 392pt, holds 311pt") beside ten errors, one per shrunk field.
// The capacity finding now carries the blocking severity and lists the shrunk
// fields as its symptoms; fixing it clears them all.
func groupRootCauses(diags []semanticDiagnostic) []semanticDiagnostic {
	return collapseDiagnostics(groupCapacityCauses(diags))
}

// groupCapacityCauses is the capacity rule of groupRootCauses.
func groupCapacityCauses(diags []semanticDiagnostic) []semanticDiagnostic {
	roots := map[int]int{} // raw slide index -> index of its root in diags
	for i, d := range diags {
		if d.Code != patterns.ErrCodeBodyTooLong || !rootCausePathRE.MatchString(d.RawPath) {
			continue
		}
		if idx := slidepath.SlideIndex(d.RawPath); idx >= 0 {
			if _, seen := roots[idx]; !seen {
				roots[idx] = i
			}
		}
	}
	if len(roots) == 0 {
		return diags
	}
	drop := map[int]bool{}
	for i, d := range diags {
		if d.Code != patterns.ErrCodeTextBelowReadableMin || !d.Blocking {
			continue
		}
		rootAt, ok := roots[symptomSlideIndex(d)]
		if !ok {
			continue
		}
		root := &diags[rootAt]
		if len(root.Symptoms) == 0 {
			adoptSymptomRepair(root, d)
		}
		if d.Action == "refuse" && d.Evidence != nil && root.Evidence == nil {
			// The generation refusal carries the measured size; keep it.
			root.Evidence = d.Evidence
		}
		root.Symptoms = append(root.Symptoms, findingSymptom{Code: d.Code, Path: firstNonEmpty(d.SemanticPath, d.RawPath), Message: d.Message})
		drop[i] = true
	}
	if len(drop) == 0 {
		return diags
	}
	out := make([]semanticDiagnostic, 0, len(diags)-len(drop))
	for i := range diags {
		if drop[i] {
			continue
		}
		d := diags[i]
		if len(d.Symptoms) > 0 {
			setDiagnosticSeverity(&d, diagnostics.SeverityError)
			if d.diag != nil {
				if d.diag.Details == nil {
					d.diag.Details = map[string]any{}
				}
				d.diag.Details[symptomsDetail] = symptomsEvidence(d.Symptoms)
			}
		}
		out = append(out, d)
	}
	return out
}

// symptomsDetail is the Details / evidence key a root-cause finding lists its
// symptoms under in the validate envelope.
const symptomsDetail = "symptoms"

func symptomsEvidence(symptoms []findingSymptom) []any {
	out := make([]any, 0, len(symptoms))
	for _, s := range symptoms {
		entry := map[string]any{"code": s.Code}
		if s.Path != "" {
			entry["path"] = s.Path
		}
		if s.Message != "" {
			entry["message"] = s.Message
		}
		out = append(out, entry)
	}
	return out
}

// symptomSlideIndex is the raw slide a symptom belongs to.
func symptomSlideIndex(d semanticDiagnostic) int {
	if idx := slidepath.SlideIndex(d.RawPath); idx >= 0 {
		return idx
	}
	if d.SlideIndex != nil {
		return *d.SlideIndex
	}
	return -1
}

// adoptSymptomRepair gives a root cause the first symptom's source-preserving
// repair (the composition switch) when it has none of its own: the root is
// reported on the slide, which no scalar patch can rewrite.
func adoptSymptomRepair(root *semanticDiagnostic, symptom semanticDiagnostic) {
	if len(root.fallbackPatch) == 0 {
		root.fallbackPatch = symptom.fallbackPatch
	}
	if root.diag == nil || symptom.diag == nil || symptom.diag.Details == nil {
		return
	}
	if patch, ok := symptom.diag.Details[compositionPatchDetail]; ok {
		if root.diag.Details == nil {
			root.diag.Details = map[string]any{}
		}
		if _, has := root.diag.Details[compositionPatchDetail]; !has {
			root.diag.Details[compositionPatchDetail] = patch
		}
	}
	if len(root.fallbackPatch) == 0 {
		if patch, ok := symptom.diag.Details[compositionPatchDetail].([]any); ok {
			root.fallbackPatch = patch
		}
	}
}

// finishFitDiagnostics maps a render's fit findings onto DeckSpec diagnostics
// with the deck's one severity model applied.
func finishFitDiagnostics(sm *semantic.SourceMap, ir *semantic.DeckIR, fit []patterns.FitFinding) []semanticDiagnostic {
	out := make([]semanticDiagnostic, 0, len(fit))
	for _, f := range fit {
		d := semanticDiagFromFitWithIR(sm, ir, f)
		if f.Path == rotatedAccentDeckPath && d.SemanticPath == "" {
			d.SemanticPath = "meta.accent_strategy"
			if d.diag != nil {
				d.diag.Path = d.SemanticPath
			}
		}
		if len(d.fallbackPatch) == 0 && d.diag != nil && d.diag.Details != nil {
			if patch, ok := d.diag.Details[compositionPatchDetail].([]any); ok {
				d.fallbackPatch = patch
			}
		}
		out = append(out, d)
	}
	return out
}

// blockingFindingReasons names every blocking finding: "CODE at path", or
// "CODE at a, b, c (+N more)" when a code blocks in several places. The
// quality gate's own sentence is appended for a QUALITY_GATE finding, since
// that finding IS the reason.
func blockingFindingReasons(diags []semanticDiagnostic) []string {
	paths := map[string][]string{}
	messages := map[string][]string{}
	var codes []string
	exemplar := false
	for _, d := range diags {
		if !diagnosticBlocks(d) {
			continue
		}
		if _, seen := paths[d.Code]; !seen {
			codes = append(codes, d.Code)
			paths[d.Code] = nil
		}
		if p := diagnosticPointer(d); p != "" && !stringListHas(paths[d.Code], p) {
			paths[d.Code] = append(paths[d.Code], p)
		}
		for _, m := range d.members {
			// A folded entry blocks at every path it stands for.
			if p := specPointer(m.Path); p != "" && !stringListHas(paths[d.Code], p) {
				paths[d.Code] = append(paths[d.Code], p)
			}
		}
		if isPlaceholderFinding(d) {
			exemplar = true
		}
		if d.Code == codeQualityGate {
			messages[d.Code] = append(messages[d.Code], strings.TrimPrefix(d.Message, "quality gate: "))
		}
	}
	sort.Strings(codes)
	const limit = 4
	reasons := make([]string, 0, len(codes)+1)
	if exemplar {
		// The same token make_deck leads with: the deck still carries copy the
		// product wrote as scaffolding (go-slide-creator-327g6).
		reasons = append(reasons, exemplarContentReason)
	}
	for _, code := range codes {
		reason := code
		if ps := paths[code]; len(ps) > 0 {
			shown := ps
			if len(shown) > limit {
				shown = shown[:limit]
			}
			reason += " at " + strings.Join(shown, ", ")
			if extra := len(ps) - len(shown); extra > 0 {
				reason += fmt.Sprintf(" (+%d more)", extra)
			}
		}
		if ms := messages[code]; len(ms) > 0 {
			reason += ": " + strings.Join(ms, "; ")
		}
		reasons = append(reasons, reason)
	}
	return reasons
}

func stringListHas(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- a render that did not produce a deck -----------------------------------

var slideNumberRE = regexp.MustCompile(`\bslide (\d+)\b`)

// buildSemanticRunFailure is buildSemanticRenderFailure for a render that
// compiled but was refused or failed while running. It reports what a single
// refusal string cannot: every contract error on the compiled slides at the
// DeckSpec path that carries it (a raw_json2pptx slide's unknown pattern or
// diagram key used to surface only as "invalid slide specification: slide 1:
// …", go-slide-creator-uon4b), and the fit findings on the rest of the deck,
// so one render names every over-full slide rather than the first refused
// paragraph (go-slide-creator-3rn3s). rr is whatever the run resolved before
// it stopped.
func buildSemanticRunFailure(input *PresentationInput, cr *semantic.CompileResult, rr RenderResult, err error, known ...semanticDiagnostic) semanticRenderResult {
	res := buildSemanticRenderFailure(cr, err)
	// explained: the failure is already a finding (a refusal, an asset the
	// caller resolved, a contract or layout error found below).
	explained := generationRefusal(err) != nil || errors.As(err, new(*StrictFitRefusal)) || len(known) > 0
	for _, d := range known {
		res.Diagnostics = appendDistinctDiagnostic(res.Diagnostics, d)
	}
	if input == nil || cr == nil {
		return res
	}
	sm, ir := cr.SourceMap, cr.IR

	contract := compiledContractDiagnostics(input, cr, rr)
	if ir != nil && len(rr.TemplateLayouts) > 0 {
		// A layout the deck requires and the template lacks stops generation;
		// name the requirement rather than the generator's error.
		for _, d := range requiredLayoutTemplateDiagnostics(ir.LayoutCoverage.Requested, input.Template, rr.TemplateLayouts) {
			contract = append(contract, semanticDiagFromCompile(d))
		}
	}
	for _, d := range contract {
		if diagnosticBlocks(d) {
			explained = true
		}
		res.Diagnostics = appendDistinctDiagnostic(res.Diagnostics, d)
	}

	if rr.SlideWidth > 0 && len(contract) == 0 {
		fit := collectFitFindings(input, rr.TemplateLayouts, rr.SlideWidth, rr.SlideHeight, &rr.TemplateTheme)
		fit = collapseRotatedAccentFindings(dedupFitFindings(fit))
		patterns.SortCanonical(fit, slidepath.SlideIndex)
		static := finishFitDiagnostics(sm, ir, fit)
		res.Diagnostics = mergeStaticDiagnostics(res.Diagnostics, static)
	}

	policy := newFindingPolicy(ir)
	policy.applyWaivers(res.Diagnostics)
	res.Waivers = policy.recorded()
	res.Diagnostics = groupRootCauses(res.Diagnostics)

	// A failed render always has an error to point at. A failure no finding
	// explains (a conversion error outside the contract checks, an output
	// validation refusal) is reported as one, on the slide it names.
	if !explained {
		code := diagnostics.CodeGenerationFailed
		if errors.As(err, new(*OutputValidationFailure)) {
			code = diagnostics.CodeOutputValidationError
		}
		d := diagnostics.Diagnostic{Code: string(code), Message: err.Error(), Severity: diagnostics.SeverityError}
		if m := slideNumberRE.FindStringSubmatch(err.Error()); m != nil {
			if n, convErr := strconv.Atoi(m[1]); convErr == nil && n >= 1 {
				d.Path = sm.SlidePath(n - 1)
			}
		}
		res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
	}
	return res
}

// mergeStaticDiagnostics adds the static fit pass to a failed render's
// diagnostics. A generation refusal and the static pass can both report the
// refused field; the refusal is kept (it carries the measured size) and takes
// the static finding's place, with its envelope rendering. A fit finding the
// failed run already reported is not reported a second time.
func mergeStaticDiagnostics(have, static []semanticDiagnostic) []semanticDiagnostic {
	key := func(d semanticDiagnostic) string { return d.Code + "\x00" + firstNonEmpty(d.SemanticPath, d.RawPath) }
	isRefusal := func(d semanticDiagnostic) bool { return d.Action == "refuse" }
	fitAt := map[string]int{} // the run's own fit findings, by code and path
	out := make([]semanticDiagnostic, 0, len(have)+len(static))
	for i, d := range have {
		if d.Action != "" {
			if _, dup := fitAt[key(d)]; !dup {
				fitAt[key(d)] = i
			}
		}
		if !isRefusal(d) {
			out = append(out, d)
		}
		// A refusal is placed where its static twin sits, or appended below.
	}
	placed := map[int]bool{}
	for _, s := range static {
		i, twin := fitAt[key(s)]
		switch {
		case !twin:
			out = append(out, s)
		case isRefusal(have[i]) && !placed[i]:
			refusal := have[i]
			if s.diag != nil {
				refusal.diag = s.diag
			}
			placed[i] = true
			out = append(out, refusal)
		}
	}
	for i, d := range have {
		if isRefusal(d) && !placed[i] {
			out = append(out, d)
		}
	}
	return out
}

// appendDistinctDiagnostic appends d unless the same code is already reported
// at the same path.
func appendDistinctDiagnostic(list []semanticDiagnostic, d semanticDiagnostic) []semanticDiagnostic {
	for _, have := range list {
		if have.Code == d.Code && have.SemanticPath == d.SemanticPath {
			return list
		}
	}
	return append(list, d)
}

// --- raw slide contracts -----------------------------------------------------

// compiledContractDiagnostics runs the pattern and diagram data contracts
// generation enforces over the compiled slides and reports each failure at a
// DeckSpec path: inside the raw slide for a raw_json2pptx payload
// ("slides[2].slide.pattern.values.current"), at the mapped source field
// otherwise. These are the checks validate_input runs on a raw deck; a
// DeckSpec carrying a raw slide skipped them, so validate_deck_spec said "no
// issues" for a payload render_deck_spec then refused
// (go-slide-creator-uon4b).
func compiledContractDiagnostics(input *PresentationInput, cr *semantic.CompileResult, rr RenderResult) []semanticDiagnostic {
	if input == nil {
		return nil
	}
	var sm *semantic.SourceMap
	var ir *semantic.DeckIR
	if cr != nil {
		sm, ir = cr.SourceMap, cr.IR
	}
	reg := patterns.Default()
	var out []semanticDiagnostic
	for i := range input.Slides {
		slide := &input.Slides[i]
		var raw []diagnostics.Diagnostic
		ctx := patterns.ExpandContext{
			Theme:       rr.TemplateTheme,
			Metadata:    rr.TemplateMetadata,
			SlideWidth:  rr.SlideWidth,
			SlideHeight: rr.SlideHeight,
			SlideIndex:  i,
		}
		if slide.Pattern != nil || slide.Compose != nil || slide.ShapeGrid != nil {
			raw = append(raw, slidePatternDiagnostics(slide, i, ctx, reg)...)
		}
		for j, item := range slide.Content {
			if item.Type == "chart" || item.Type == "diagram" {
				raw = append(raw, contentDiagramValidationDiagnostics(item, i, j)...)
			}
		}
		if len(raw) == 0 {
			// Grid diagrams are checked only on a slide whose patterns resolve:
			// an unresolved pattern has no grid to inspect.
			raw = append(raw, gridDiagramValidationDiagnostics(slide.ShapeGrid, i, "shape_grid", slidepath.ShapeGrid(i))...)
			if rr.SlideWidth > 0 {
				theme := rr.TemplateTheme
				if pg := expandSlidePatternGrid(slide, i, rr.SlideWidth, rr.SlideHeight, &theme); pg != nil {
					raw = append(raw, gridDiagramValidationDiagnostics(pg, i, "pattern "+slide.Pattern.Name, slidepath.SlideField(i, "pattern"))...)
				}
			}
			raw = append(raw, composeDiagramValidationDiagnostics(slide.Compose, i, slidepath.SlideField(i, "compose"))...)
		}
		for _, d := range raw {
			if d.Severity != diagnostics.SeverityError {
				continue
			}
			rawPath := d.Path
			d.Path = contractSemanticPath(sm, ir, i, rawPath)
			d.Message = strings.TrimSuffix(d.Message, " (generate would refuse this deck)")
			d.Message = slideNumberMessage(d.Message, i, i)
			sd := semanticDiagFromCompile(d)
			sd.RawPath = rawPath
			if idx := i; idx >= 0 {
				sd.SlideIndex = &idx
			}
			out = appendDistinctDiagnostic(out, sd)
		}
	}
	return out
}

// contractSemanticPath maps a contract finding's raw pointer to the DeckSpec.
// A raw_json2pptx slide carries its payload verbatim under "slide", so the
// pointer's tail is a path inside it; any other kind goes through the source
// map and falls back to the slide.
func contractSemanticPath(sm *semantic.SourceMap, ir *semantic.DeckIR, rawIdx int, rawPath string) string {
	slidePath := sm.SlidePath(rawIdx)
	if ir != nil && rawIdx < len(ir.Slides) && ir.Slides[rawIdx].Kind == semantic.KindRawJSON2pptx {
		tail := strings.TrimPrefix(rawPath, slidepath.Slide(rawIdx))
		if tail != rawPath {
			return slidePath + ".slide" + pointerTailToDotted(tail)
		}
		return slidePath + ".slide"
	}
	if semPath, _, mapped := sm.ResolveSemantic(rawPath); mapped && semPath != "" {
		return semPath
	}
	return slidePath
}

// pointerTailToDotted renders a JSON Pointer tail ("/content/1/data/primary/2")
// in DeckSpec path notation (".content[1].data.primary[2]"). A tail that
// already uses dotted notation after its last pointer token is kept.
func pointerTailToDotted(tail string) string {
	var b strings.Builder
	for _, tok := range strings.Split(strings.TrimPrefix(tail, "/"), "/") {
		if tok == "" {
			continue
		}
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		if _, err := strconv.Atoi(tok); err == nil {
			b.WriteString("[" + tok + "]")
			continue
		}
		b.WriteString("." + tok)
	}
	return b.String()
}

// --- the validate envelope ----------------------------------------------------

// envelopeDiagnostics renders a render's diagnostics as the transport-neutral
// diagnostics validate_deck_spec's envelope is built from. Severity, path and
// code are the render's own, so the two tools cannot disagree about a finding.
func envelopeDiagnostics(diags []semanticDiagnostic) []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(diags))
	for _, d := range diags {
		var dd diagnostics.Diagnostic
		if d.diag != nil {
			dd = *d.diag
		} else {
			dd = diagnostics.Diagnostic{Code: d.Code, Message: d.Message}
		}
		dd.Severity = diagnostics.Severity(d.Severity)
		if dd.Severity == "" {
			dd.Severity = diagnostics.SeverityInfo
		}
		dd.Path = firstNonEmpty(d.SemanticPath, d.RawPath, dd.Path)
		dd.Message = d.Message
		if d.Action == "refuse" || len(d.Evidence) > 0 {
			dd.Details = mergeRefusalEvidence(dd.Details, d)
		}
		out = append(out, dd)
	}
	return out
}

// mergeRefusalEvidence carries a generation refusal's measurement into the
// envelope's details, where the static pass keeps the same facts.
func mergeRefusalEvidence(details map[string]any, d semanticDiagnostic) map[string]any {
	out := map[string]any{}
	for k, v := range details {
		out[k] = v
	}
	if d.Action != "" {
		out["action"] = d.Action
	}
	for _, k := range []string{"measured", "allowed"} {
		if v, ok := d.Evidence[k]; ok {
			if _, has := out[k]; !has {
				out[k] = v
			}
		}
	}
	if _, has := out[compositionPatchDetail]; !has && len(d.fallbackPatch) > 0 {
		out[compositionPatchDetail] = d.fallbackPatch
	}
	return out
}

// stampEnvelopeFindings writes each finding's blocking flag, waiver and
// symptoms onto the envelope built from envelopeDiagnostics(diags). The two
// slices are index-aligned; a spec that did not compile has no diags and gets
// the blocking flag alone.
func stampEnvelopeFindings(envelope *diagnostics.FindingEnvelope, diags []semanticDiagnostic) {
	for i := range envelope.Findings {
		f := &envelope.Findings[i]
		blocking := f.Severity == diagnostics.SeverityError
		f.Blocking = &blocking
		if i >= len(diags) || (diags[i].Waived == "" && len(diags[i].Symptoms) == 0) {
			continue
		}
		if f.Evidence == nil {
			f.Evidence = map[string]any{}
		}
		if diags[i].Waived != "" {
			f.Evidence["waived"] = diags[i].Waived
		}
		if len(diags[i].Symptoms) > 0 {
			f.Evidence[symptomsDetail] = symptomsEvidence(diags[i].Symptoms)
		}
	}
}
