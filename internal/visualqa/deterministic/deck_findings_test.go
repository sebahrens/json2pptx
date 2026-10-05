package deterministic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func deckLevel(code, path, action string, slides any) patterns.FitFinding {
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{Code: code, Path: path, Message: code + " at " + path},
		Action:          action,
	}
	if slides != nil {
		f.Fix = &patterns.FixSuggestion{Kind: "rewrite_field", Params: map[string]any{"slides": slides}}
	}
	return f
}

// go-slide-creator-qhgm8: a finding whose path is not under /slides/N was
// grouped under index -1 and never read back — CHROME_TRUNCATED at /chrome was
// in validate and generate but not in score_deck, and cost nothing. It is now
// listed once in deck_findings, costs its weight once on the overall score and
// is counted in top_codes; the slides it names are given as per_slide indices.
func TestScoreFromFindings_DeckLevelFindingIsListedAndCosts(t *testing.T) {
	chrome := deckLevel(patterns.ErrCodeChromeTruncated, "/chrome", "review", []int{2, 3, 4})
	clean := ScoreFromFindings(nil, 4)
	ds := ScoreFromFindings([]patterns.FitFinding{chrome}, 4)

	if got, want := ds.OverallScore, clean.OverallScore-SeverityWeight["review"]; got != want {
		t.Errorf("overall = %d, want %d: a deck-level review finding costs its weight once", got, want)
	}
	if len(ds.DeckFindings) != 1 {
		t.Fatalf("deck_findings = %+v, want the one chrome finding", ds.DeckFindings)
	}
	df := ds.DeckFindings[0]
	if df.Code != patterns.ErrCodeChromeTruncated || df.Path != "/chrome" || df.Points != 5 || df.Severity != "info" || df.Fix == nil {
		t.Errorf("deck finding = %+v", df)
	}
	if len(df.Slides) != 3 || df.Slides[0] != 1 || df.Slides[2] != 3 {
		t.Errorf("slides = %v, want the 0-based [1 2 3]", df.Slides)
	}
	for _, ss := range ds.PerSlide {
		if ss.Score != 100 || len(ss.Findings) != 0 {
			t.Errorf("slide %d = %+v: one footer fact must not be charged to every slide", ss.Index, ss)
		}
	}
	if ds.Summary.ProblemSlidesCount != 0 {
		t.Errorf("problem slides = %d, want 0", ds.Summary.ProblemSlidesCount)
	}
	if len(ds.Summary.TopCodes) != 1 || ds.Summary.TopCodes[0] != (CodeCount{Code: patterns.ErrCodeChromeTruncated, Count: 1}) {
		t.Errorf("top_codes = %+v", ds.Summary.TopCodes)
	}
	gate := EvaluateQualityGate(ds, []patterns.FitFinding{chrome}, DefaultQualityGateCriteria())
	if !gate.Passed {
		t.Errorf("a review-level footer finding must not fail the gate: %v", gate.Reasons)
	}
}

// Preflight and render report the same footer line; the scorer lists it once.
// After a JSON round trip the slide numbers are float64.
func TestScoreFromFindings_DeckLevelFindingsAreDeduplicated(t *testing.T) {
	a := deckLevel(patterns.ErrCodeChromeTruncated, "/chrome", "review", []int{1, 2})
	b := deckLevel(patterns.ErrCodeChromeTruncated, "/chrome", "review", []any{float64(1), float64(2)})
	ds := ScoreFromFindings([]patterns.FitFinding{a, b}, 2)
	if len(ds.DeckFindings) != 1 || ds.OverallScore != 95 {
		t.Errorf("deck_findings = %d overall = %d, want one finding and 95", len(ds.DeckFindings), ds.OverallScore)
	}
	if got := ds.DeckFindings[0].Slides; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("slides = %v, want [0 1]", got)
	}
}

// A finding with a deck-level path that names exactly one slide is that
// slide's finding; one that names none stays deck-level; a refuse-level one
// (RENDER_EVIDENCE_INCOMPLETE has no path at all) costs 25.
func TestScoreFromFindings_DeckLevelAttribution(t *testing.T) {
	one := deckLevel(patterns.ErrCodeChromeTruncated, "/footer/left_text", "review", []int{3})
	none := deckLevel("TEMPLATE_NOTE", "", "review", nil)
	refuse := deckLevel("RENDER_EVIDENCE_INCOMPLETE", "", "refuse", nil)
	ds := ScoreFromFindings([]patterns.FitFinding{one, none, refuse}, 4)

	if got := ds.PerSlide[2]; got.Score != 95 || len(got.Findings) != 1 || got.Findings[0].Code != patterns.ErrCodeChromeTruncated {
		t.Errorf("slide 2 = %+v, want the finding that names slide 3 only", got)
	}
	if len(ds.DeckFindings) != 2 {
		t.Fatalf("deck_findings = %+v, want the two that name no slide", ds.DeckFindings)
	}
	for _, df := range ds.DeckFindings {
		if df.Slides != nil {
			t.Errorf("%s: slides = %v, want none", df.Code, df.Slides)
		}
	}
	// Mean of 100, 100, 95, 100 is 98; one of four slides is under the share
	// floor, so no breadth penalty; then 5 + 25 for the deck-level findings.
	if ds.OverallScore != 98-5-25 {
		t.Errorf("overall = %d, want 68", ds.OverallScore)
	}
	raw, err := json.Marshal(ds.DeckFindings[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"path"`) || strings.Contains(string(raw), `"slides"`) {
		t.Errorf("empty path / slides must be omitted: %s", raw)
	}
}

// The overall score never goes below zero, and a deck with no deck-level
// finding has no deck_findings key.
func TestScoreFromFindings_DeckLevelClampAndOmission(t *testing.T) {
	var many []patterns.FitFinding
	for i := 0; i < 6; i++ {
		many = append(many, deckLevel("RENDER_EVIDENCE_INCOMPLETE", "", "refuse", nil))
		many[i].Message += strings.Repeat("!", i) // distinct findings
	}
	if ds := ScoreFromFindings(many, 1); ds.OverallScore != 0 || len(ds.DeckFindings) != 6 {
		t.Errorf("overall = %d deck_findings = %d, want 0 and 6", ds.OverallScore, len(ds.DeckFindings))
	}
	raw, err := json.Marshal(ScoreFromFindings(nil, 2))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "deck_findings") {
		t.Errorf("deck_findings must be omitted when empty: %s", raw)
	}
}

// With slide_indices the score covers a subset: a deck-level finding that
// names slides is kept when it names a scored one and dropped otherwise, a
// finding that names exactly one slide follows that slide, and one that names
// none always applies.
func TestScoreFromFindingsForIndices_DeckLevelFindings(t *testing.T) {
	chrome := deckLevel(patterns.ErrCodeChromeTruncated, "/chrome", "review", []int{2, 3, 4})
	single := deckLevel(patterns.ErrCodeChromeTruncated, "/footer/left_text", "review", []int{1})
	note := deckLevel("TEMPLATE_NOTE", "", "review", nil)
	all := []patterns.FitFinding{chrome, single, note}

	in := ScoreFromFindingsForIndices(all, 4, []int{1, 2})
	if len(in.DeckFindings) != 2 || in.OverallScore != 90 {
		t.Errorf("slides 1,2: deck_findings = %+v overall = %d, want chrome + note and 90", in.DeckFindings, in.OverallScore)
	}
	if got := in.DeckFindings[0].Slides; len(got) != 3 || got[0] != 1 {
		t.Errorf("slides = %v, want the deck's own 0-based [1 2 3]", got)
	}

	out := ScoreFromFindingsForIndices(all, 4, []int{0})
	if len(out.DeckFindings) != 1 || out.DeckFindings[0].Code != "TEMPLATE_NOTE" {
		t.Errorf("slide 0: deck_findings = %+v, want the note only", out.DeckFindings)
	}
	if len(out.PerSlide) != 1 || len(out.PerSlide[0].Findings) != 1 || out.PerSlide[0].Score != 95 {
		t.Errorf("slide 0: per_slide = %+v, want the finding that names slide 1", out.PerSlide)
	}
	if out.OverallScore != 90 {
		t.Errorf("slide 0: overall = %d, want 95 - 5", out.OverallScore)
	}
}
