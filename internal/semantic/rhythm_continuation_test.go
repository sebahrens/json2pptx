package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// titledKPI is a KPI slide with a given title.
func titledKPI(title string) SlideSpec {
	s := kpiSlide(3)
	s.Body["title"] = title
	return s
}

// xy51l: the parts of a continued exhibit count once, so the split that
// SEMANTIC_DENSITY advises does not create a monotony run.
func TestRhythmMonotonyCountsContinuationOnce(t *testing.T) {
	monotony := string(diagnostics.CodeSemanticRhythmMonotony)
	split := &DeckSpec{Slides: []SlideSpec{titledKPI("Savings by lever (1/2)"), titledKPI("Savings by lever (2/2)"), titledKPI("Run rate")}}
	if ws := Normalize(split).RhythmWarnings(); rhythmCodes(ws)[monotony] {
		t.Errorf("a split exhibit plus one slide is two units, not a run of three: %+v", ws)
	}
	separate := &DeckSpec{Slides: []SlideSpec{titledKPI("Savings by lever"), titledKPI("Costs by region"), titledKPI("Run rate")}}
	ws := Normalize(separate).RhythmWarnings()
	if !rhythmCodes(ws)[monotony] {
		t.Fatalf("three separate KPI slides are still a run: %+v", ws)
	}
	for _, w := range ws {
		if w.Code == monotony && !strings.Contains(w.Message, "slides[0] to slides[2]") {
			t.Errorf("monotony message does not name the run: %q", w.Message)
		}
	}
}

// xy51l: a section divider between the halves of a continued exhibit is
// reported at the divider.
func TestRhythmContinuationSplitByDivider(t *testing.T) {
	code := string(diagnostics.CodeSemanticRhythmContinuationSplit)
	divider := SlideSpec{Kind: KindSection, Body: map[string]any{"title": "Savings levers"}}
	spec := &DeckSpec{Slides: []SlideSpec{titledKPI("Savings by lever (1/2)"), divider, titledKPI("Savings by lever (2/2)")}}
	ws := Normalize(spec).RhythmWarnings()
	var got *RhythmWarning
	for i := range ws {
		if ws[i].Code == code {
			got = &ws[i]
		}
	}
	if got == nil {
		t.Fatalf("expected %s, got %+v", code, ws)
	}
	if got.Path != "slides[1]" {
		t.Errorf("path = %q, want slides[1] (the divider)", got.Path)
	}
	if !strings.Contains(got.Message, "section slide") || !strings.Contains(got.Message, "(2/2)") {
		t.Errorf("message = %q", got.Message)
	}
	adjacent := &DeckSpec{Slides: []SlideSpec{divider, titledKPI("Savings by lever (1/2)"), titledKPI("Savings by lever (2/2)")}}
	if ws := Normalize(adjacent).RhythmWarnings(); rhythmCodes(ws)[code] {
		t.Errorf("adjacent halves reported: %+v", ws)
	}
}
