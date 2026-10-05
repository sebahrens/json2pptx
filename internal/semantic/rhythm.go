package semantic

// This file implements the semantic compiler's deck-rhythm rules: the quality
// layer that, given a normalized DeckIR, flags monotony and missing narrative
// structure before a deck is rendered. The rules read the planning decisions the
// IR already carries (per-slide visual family and density, the deck's executive
// flag) rather than re-deriving them, so explain and compile agree on every run.
//
// The run-length threshold matches internal/deckplan's enforceRhythm convention:
// a run of 3+ adjacent slides of the same family (i.e. more than two in a row)
// is the monotony trip point. The rules here only diagnose; they never rewrite
// the deck — that remains the author's choice.

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/rhythm"
)

const (
	// maxConsecutiveSameFamily is the longest run of one visual family that does
	// not trip the monotony rule; a run strictly longer than this is flagged.
	maxConsecutiveSameFamily = 2
	// maxConsecutiveDense is the longest run of heavy-density slides that does not
	// trip the density rule.
	maxConsecutiveDense = 2
	// sectioningSlideThreshold is the count of body slides (cover, closing and
	// other structural chrome excluded) above which a deck with no section
	// divider is flagged for missing chapter structure. It was 10, plan_deck's
	// default slide_budget (go-slide-creator-n83ml), which still nagged every
	// 12-slide proposal and steering-committee case on each validate and
	// render: a brief that dictates 11-13 slides has no room for dividers, and
	// a deck that short is one chapter (go-slide-creator-th6o9). The advice
	// now starts at 13 body slides.
	sectioningSlideThreshold      = 12
	visualBreadthSlideThreshold   = 7
	minimumDistinctVisualFamilies = 3
)

// RhythmWarning is a single deck-rhythm advisory: a monotony or
// missing-structure finding anchored to the semantic source path it concerns.
type RhythmWarning struct {
	// Code is one of the SEMANTIC_RHYTHM_* diagnostic codes.
	Code string `json:"code"`
	// Message is the human-facing explanation.
	Message string `json:"message"`
	// Path is the semantic source path the warning anchors to (e.g. "slides[3]"
	// for the slide that starts a run), or "" for a deck-level warning.
	Path string `json:"path,omitempty"`
	// Run lists the source paths of the slides a run warning is about, in deck
	// order, so the reader knows which slides to vary or break up without
	// counting from Path (go-slide-creator-c2j5b).
	Run []string `json:"run,omitempty"`
	// InsertBefore is the source path of the slide a density run should be
	// broken in front of: where a divider or a lighter slide goes
	// (go-slide-creator-th6o9).
	InsertBefore string `json:"insert_before,omitempty"`
}

// RhythmWarnings analyzes the deck's rhythm and returns the advisory findings in
// a deterministic order: per-slide run warnings first (ascending by slide
// index — monotony then density within a tie), then the deck-level sectioning
// and synthesis warnings. A nil or empty IR yields no warnings.
func (ir *DeckIR) RhythmWarnings() []RhythmWarning {
	if ir == nil || len(ir.Slides) == 0 {
		return nil
	}
	var out []RhythmWarning
	out = append(out, ir.monotonyWarnings()...)
	out = append(out, ir.densityWarnings()...)
	out = append(out, ir.continuationWarnings()...)
	if w, ok := ir.sectioningWarning(); ok {
		out = append(out, w)
	}
	if w, ok := ir.synthesisWarning(); ok {
		out = append(out, w)
	}
	if w, ok := ir.evidenceVisualWarning(); ok {
		out = append(out, w)
	}
	if w, ok := ir.visualFamilyWarning(); ok {
		out = append(out, w)
	}
	for _, missing := range ir.LayoutCoverage.Missing {
		out = append(out, RhythmWarning{
			Code:    string(diagnostics.CodeSemanticRequiredLayoutMissing),
			Message: fmt.Sprintf("required layout %q has no compatible authored slide; add a suitable semantic slide instead of forcing an incompatible layout", missing),
			Path:    "meta.required_layouts",
		})
	}
	return out
}

func (ir *DeckIR) evidenceVisualWarning() (RhythmWarning, bool) {
	if ir.Archetype != ArchetypeMarketAnalysis || len(ir.Slides) < 5 || ir.Rhythm.EvidenceFamilyCount > 0 {
		return RhythmWarning{}, false
	}
	return RhythmWarning{
		Code:    string(diagnostics.CodeSemanticEvidenceVisualMissing),
		Message: "market_analysis decks need at least one data-bearing evidence visual; add a chart_insight, table, kpi_snapshot, bridge, or sourced stat slide",
		Path:    "meta.archetype",
	}, true
}

func (ir *DeckIR) visualFamilyWarning() (RhythmWarning, bool) {
	if len(ir.Slides) < visualBreadthSlideThreshold || ir.Rhythm.DistinctFamilyCount >= minimumDistinctVisualFamilies {
		return RhythmWarning{}, false
	}
	return RhythmWarning{
		Code: string(diagnostics.CodeSemanticVisualFamilyNarrow),
		Message: fmt.Sprintf("a %d-slide deck uses only %d non-structural visual families; use at least %d compatible families so evidence, analysis, and structure do not all read the same",
			len(ir.Slides), ir.Rhythm.DistinctFamilyCount, minimumDistinctVisualFamilies),
		Path: ir.slideListPath(),
	}, true
}

// monotonyWarnings flags every run of more than maxConsecutiveSameFamily
// adjacent slides sharing one visual family. Raw/passthrough slides have no
// modeled family and structural slides are deliberate navigation/chrome; neither
// trips the rule. This keeps the normal cover → agenda → section sequence from
// being diagnosed as monotony.
func (ir *DeckIR) monotonyWarnings() []RhythmWarning {
	var out []RhythmWarning
	for _, run := range familyRuns(ir.Slides) {
		if run.family == FamilyRaw || run.family == FamilyStructural || run.units <= maxConsecutiveSameFamily {
			continue
		}
		paths := make([]string, 0, run.length)
		for i := run.start; i < run.start+run.length; i++ {
			paths = append(paths, ir.slidePath(i))
		}
		out = append(out, RhythmWarning{
			Code: string(diagnostics.CodeSemanticRhythmMonotony),
			Message: fmt.Sprintf("%d consecutive %s slides (%s to %s) read as monotonous; vary the slide kinds or insert a section break inside the run",
				run.units, run.family, paths[0], paths[len(paths)-1]),
			Path: ir.slidePath(run.start),
			Run:  paths,
		})
	}
	return out
}

// continuationWarnings flags a continued exhibit whose parts do not follow
// each other: "Savings by lever (1/2)", a section divider, "(2/2)". The parts
// are one unit of the argument, and the divider's section then labels the
// wrong slides (go-slide-creator-xy51l).
func (ir *DeckIR) continuationWarnings() []RhythmWarning {
	titles := make([]string, len(ir.Slides))
	for i := range ir.Slides {
		titles[i] = ir.Slides[i].Title
	}
	var out []RhythmWarning
	for _, gap := range rhythm.ContinuationGaps(titles) {
		between := ir.Slides[gap.Between[0]]
		what := "a slide"
		if between.Visual.Family == FamilyStructural {
			what = fmt.Sprintf("a %s slide", between.Kind)
		}
		out = append(out, RhythmWarning{
			Code: string(diagnostics.CodeSemanticRhythmContinuationSplit),
			Message: fmt.Sprintf("%s sits between %q and its continuation %q; a continued exhibit is one unit — move %s before %s or after %s",
				what, ir.Slides[gap.Before].Title, ir.Slides[gap.After].Title,
				ir.slidePath(gap.Between[0]), ir.slidePath(gap.Before), ir.slidePath(gap.After)),
			Path: ir.slidePath(gap.Between[0]),
		})
	}
	return out
}

// densityWarnings flags every run of more than maxConsecutiveDense adjacent
// heavy-density slides.
func (ir *DeckIR) densityWarnings() []RhythmWarning {
	var out []RhythmWarning
	runStart := 0
	runLen := 0
	flush := func() {
		if runLen > maxConsecutiveDense {
			paths := make([]string, 0, runLen)
			for i := runStart; i < runStart+runLen; i++ {
				paths = append(paths, ir.slidePath(i))
			}
			// The break goes where the run first exceeds what an audience
			// takes in one stretch: after its second slide. The advice used
			// to name neither the run nor a place, and an author whose dense
			// slides were all required content had nothing to act on
			// (go-slide-creator-th6o9).
			at := paths[maxConsecutiveDense]
			out = append(out, RhythmWarning{
				Code: string(diagnostics.CodeSemanticRhythmDensity),
				Message: fmt.Sprintf("%d consecutive dense slides (%s to %s) fatigue the audience; insert a section divider or a lighter slide before %s, or move a lighter slide there",
					runLen, paths[0], paths[len(paths)-1], at),
				Path:         ir.slidePath(runStart),
				Run:          paths,
				InsertBefore: at,
			})
		}
	}
	for i := range ir.Slides {
		if ir.Slides[i].Visual.Density == DensityHeavy && !ir.Slides[i].Appendix {
			if runLen == 0 {
				runStart = i
			}
			runLen++
			continue
		}
		flush()
		runLen = 0
	}
	flush()
	return out
}

// sectioningWarning flags a long deck that carries no section divider. An
// explicit meta.chrome.tracker: false declines chapters at any length.
func (ir *DeckIR) sectioningWarning() (RhythmWarning, bool) {
	if ir.Chrome != nil && ir.Chrome.TrackerDeclined {
		return RhythmWarning{}, false
	}
	body := 0
	for i := range ir.Slides {
		if ir.Slides[i].Kind == KindSection || ir.Slides[i].Role == RoleTransition {
			return RhythmWarning{}, false
		}
		if ir.Slides[i].Visual.Family != FamilyStructural {
			body++
		}
	}
	if body <= sectioningSlideThreshold {
		return RhythmWarning{}, false
	}
	return RhythmWarning{
		Code: string(diagnostics.CodeSemanticRhythmSectioning),
		Message: fmt.Sprintf("a %d-slide deck carries %d body slides and no section dividers; add `section` slides to group it into chapters",
			len(ir.Slides), body),
	}, true
}

// synthesisWarning flags an executive-archetype deck with no synthesis
// (executive_summary) or decision slide.
func (ir *DeckIR) synthesisWarning() (RhythmWarning, bool) {
	if !ir.Executive {
		return RhythmWarning{}, false
	}
	for i := range ir.Slides {
		if ir.Slides[i].Kind == KindExecutiveSummary || ir.Slides[i].Kind == KindDecision {
			return RhythmWarning{}, false
		}
	}
	return RhythmWarning{
		Code: string(diagnostics.CodeSemanticRhythmSynthesis),
		Message: fmt.Sprintf("%s decks should land the message with a synthesis or decision slide; add an executive_summary or decision slide",
			ir.Archetype),
	}, true
}

// familyRun describes a maximal run of adjacent slides sharing one visual family.
type familyRun struct {
	family VisualFamily
	start  int // index into the slice of the run's first slide
	length int
	// units is the run's length in exhibits: the parts of a continued slide
	// ("(1/2)", "(2/2)", "(cont.)") count once (go-slide-creator-xy51l).
	units int
}

// slidePath is the DeckSpec locator of ir.Slides[i]: its authored source path
// (a structure-mode deck has no slides[] and its expanded stream includes the
// cover, generated agenda and dividers), falling back to slides[SourceIndex]
// (go-slide-creator-csclk.45).
func (ir *DeckIR) slidePath(i int) string {
	if p := ir.Slides[i].SourcePath; p != "" {
		return p
	}
	return fmt.Sprintf("slides[%d]", ir.Slides[i].SourceIndex)
}

// slideListPath is the deck-level slide container: "structure" for a
// structure-mode deck, "slides" otherwise.
func (ir *DeckIR) slideListPath() string {
	for i := range ir.Slides {
		if strings.HasPrefix(ir.Slides[i].SourcePath, "structure") {
			return "structure"
		}
	}
	return "slides"
}

// familyRuns groups the slides into maximal runs of identical visual family, in
// source order. Appendix back matter is reference material, not part of the
// argument's rhythm: it counts as structural, so it neither forms nor extends
// a run (go-slide-creator-khzni).
func familyRuns(slides []SlideIR) []familyRun {
	var runs []familyRun
	for i := range slides {
		fam := slides[i].Visual.Family
		if slides[i].Appendix {
			fam = FamilyStructural
		}
		if n := len(runs); n > 0 && runs[n-1].family == fam {
			runs[n-1].length++
			if !rhythm.Continues(slides[i-1].Title, slides[i].Title) {
				runs[n-1].units++
			}
			continue
		}
		runs = append(runs, familyRun{family: fam, start: i, length: 1, units: 1})
	}
	return runs
}

// rhythmDiagnostics converts the deck's rhythm warnings into transport-neutral
// diagnostics under the given strictness: suppressed entirely under off, emitted
// as warnings under warn, promoted to errors under strict — mirroring the
// advisory policy the per-slide validation gates use.
func rhythmDiagnostics(ir *DeckIR, strict Strictness) []diagnostics.Diagnostic {
	if strict == StrictnessOff {
		return nil
	}
	sev := diagnostics.SeverityWarning
	if strict == StrictnessStrict {
		sev = diagnostics.SeverityError
	}
	warnings := ir.RhythmWarnings()
	if len(warnings) == 0 {
		return nil
	}
	out := make([]diagnostics.Diagnostic, 0, len(warnings))
	for _, w := range warnings {
		findingSeverity := sev
		if w.Code == string(diagnostics.CodeSemanticRequiredLayoutMissing) {
			findingSeverity = diagnostics.SeverityError
		}
		d := diagnostics.Diagnostic{
			Code:     w.Code,
			Message:  w.Message,
			Path:     w.Path,
			Severity: findingSeverity,
		}
		if len(w.Run) > 0 {
			d.Details = map[string]any{"run": w.Run}
		}
		if w.InsertBefore != "" {
			d.Details["insert_before"] = w.InsertBefore
		}
		out = append(out, d)
	}
	return out
}
