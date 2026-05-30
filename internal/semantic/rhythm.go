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

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

const (
	// maxConsecutiveSameFamily is the longest run of one visual family that does
	// not trip the monotony rule; a run strictly longer than this is flagged.
	maxConsecutiveSameFamily = 2
	// maxConsecutiveDense is the longest run of heavy-density slides that does not
	// trip the density rule.
	maxConsecutiveDense = 2
	// sectioningSlideThreshold is the slide count above which a deck with no
	// section divider is flagged for missing chapter structure.
	sectioningSlideThreshold = 8
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
	if w, ok := ir.sectioningWarning(); ok {
		out = append(out, w)
	}
	if w, ok := ir.synthesisWarning(); ok {
		out = append(out, w)
	}
	return out
}

// monotonyWarnings flags every run of more than maxConsecutiveSameFamily
// adjacent slides sharing one visual family. Raw/passthrough slides have no
// modeled family and never trip the rule.
func (ir *DeckIR) monotonyWarnings() []RhythmWarning {
	var out []RhythmWarning
	for _, run := range familyRuns(ir.Slides) {
		if run.family == FamilyRaw || run.length <= maxConsecutiveSameFamily {
			continue
		}
		out = append(out, RhythmWarning{
			Code: string(diagnostics.CodeSemanticRhythmMonotony),
			Message: fmt.Sprintf("%d consecutive %s slides read as monotonous; vary the slide kinds or insert a section break",
				run.length, run.family),
			Path: fmt.Sprintf("slides[%d]", run.start),
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
			out = append(out, RhythmWarning{
				Code: string(diagnostics.CodeSemanticRhythmDensity),
				Message: fmt.Sprintf("%d consecutive dense slides fatigue the audience; break the run with a lighter slide",
					runLen),
				Path: fmt.Sprintf("slides[%d]", ir.Slides[runStart].SourceIndex),
			})
		}
	}
	for i := range ir.Slides {
		if ir.Slides[i].Visual.Density == DensityHeavy {
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

// sectioningWarning flags a long deck that carries no section divider.
func (ir *DeckIR) sectioningWarning() (RhythmWarning, bool) {
	if len(ir.Slides) <= sectioningSlideThreshold {
		return RhythmWarning{}, false
	}
	for i := range ir.Slides {
		if ir.Slides[i].Kind == KindSection || ir.Slides[i].Role == RoleTransition {
			return RhythmWarning{}, false
		}
	}
	return RhythmWarning{
		Code: string(diagnostics.CodeSemanticRhythmSectioning),
		Message: fmt.Sprintf("a %d-slide deck has no section dividers; add `section` slides to group it into chapters",
			len(ir.Slides)),
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
	start  int // SourceIndex of the run's first slide
	length int
}

// familyRuns groups the slides into maximal runs of identical visual family, in
// source order.
func familyRuns(slides []SlideIR) []familyRun {
	var runs []familyRun
	for i := range slides {
		fam := slides[i].Visual.Family
		if n := len(runs); n > 0 && runs[n-1].family == fam {
			runs[n-1].length++
			continue
		}
		runs = append(runs, familyRun{family: fam, start: slides[i].SourceIndex, length: 1})
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
		out = append(out, diagnostics.Diagnostic{
			Code:     w.Code,
			Message:  w.Message,
			Path:     w.Path,
			Severity: sev,
		})
	}
	return out
}
