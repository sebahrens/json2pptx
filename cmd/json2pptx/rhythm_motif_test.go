package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// Motif and continuation checks through the JSON adapter
// (go-slide-creator-rd7oj, go-slide-creator-xy51l).

func motifPatternSlide(name, title, overrides string) SlideInput {
	s := SlideInput{SlideType: "content", Pattern: &PatternInput{Name: name}}
	if overrides != "" {
		s.Pattern.Overrides = json.RawMessage(overrides)
	}
	s.Content = []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}
	return s
}

func motifChartSlide(kind, title string) SlideInput {
	return SlideInput{SlideType: "content", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: &title},
		{PlaceholderID: "body", Type: "chart", ChartValue: &types.ChartSpec{Type: types.ChartType(kind), Data: map[string]any{"a": 1.0, "b": 2.0}}},
	}}
}

func motifTableSlide(title string) SlideInput {
	return SlideInput{SlideType: "content", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: &title},
		{PlaceholderID: "body", Type: "table"},
	}}
}

func compositionCodesOf(c *deterministic.CompositionResult) map[string]string {
	out := map[string]string{}
	for _, d := range c.Diagnostics {
		out[d.Code] = d.Message
	}
	return out
}

// A deck alternating kpi-4up / stylish-panels / card-grid is flagged by
// analyze_deck_rhythm and by score_deck's composition axis.
func TestRhythmMotif_LookAlikeDeckIsFlagged(t *testing.T) {
	var slides []SlideInput
	for i := 0; i < 2; i++ {
		slides = append(slides,
			motifPatternSlide("kpi-4up", "Four numbers moved this year", ""),
			motifPatternSlide("stylish-panels", "Three pillars carry the plan", ""),
			motifPatternSlide("card-grid", "Six levers close the gap", ""),
		)
	}
	result := analyzeDeckRhythm(slides)
	if result.Aggregates.LongestRun >= 3 {
		t.Fatalf("no pattern run expected: %+v", result.Aggregates.PatternRuns)
	}
	if result.Aggregates.DominantMotif != string(patterns.MotifOpenColumns) {
		t.Fatalf("dominant_motif = %q, share %v", result.Aggregates.DominantMotif, result.Aggregates.MotifShare)
	}
	if got := result.PerSlide[0].Motif; got != string(patterns.MotifOpenColumns) {
		t.Errorf("kpi-4up motif = %q", got)
	}
	if got := result.PerSlide[2].Motif; got != string(patterns.MotifTiles) {
		t.Errorf("card-grid motif = %q", got)
	}

	comp := compositionAxis(slides)
	codes := compositionCodesOf(comp)
	msg, ok := codes["motif_dominance"]
	if !ok {
		t.Fatalf("score_deck composition has no motif_dominance diagnostic: %+v", comp.Diagnostics)
	}
	if !strings.Contains(msg, "open-columns") || !strings.Contains(msg, "try:") {
		t.Errorf("motif_dominance message lacks the motif or alternatives: %q", msg)
	}
	if comp.Score >= 100 {
		t.Errorf("composition score %d: the look-alike deck must lose points", comp.Score)
	}
}

// A run of three different patterns in one motif is reported with break
// suggestions; the same slides with an explicit tile style are not a run.
func TestRhythmMotif_RunAndExplicitStyle(t *testing.T) {
	run := []SlideInput{
		motifChartSlide("bar", "Revenue grew 12% in 2025"),
		motifPatternSlide("kpi-3up", "Three numbers moved", ""),
		motifPatternSlide("icon-row", "Three capabilities matter", ""),
		motifPatternSlide("stylish-panels", "Three pillars carry the plan", ""),
		motifTableSlide("Costs fell in every region"),
	}
	result := analyzeDeckRhythm(run)
	if len(result.Aggregates.MotifRuns) != 1 || result.Aggregates.MotifRuns[0].Start != 1 || result.Aggregates.MotifRuns[0].End != 3 {
		t.Fatalf("motif_runs = %+v, want one run over slides 1..3", result.Aggregates.MotifRuns)
	}
	if _, ok := compositionCodesOf(compositionAxis(run))["motif_run"]; !ok {
		t.Errorf("score_deck composition has no motif_run diagnostic")
	}

	styled := append([]SlideInput(nil), run...)
	styled[3] = motifPatternSlide("stylish-panels", "Three pillars carry the plan", `{"style":"ribbon"}`)
	result = analyzeDeckRhythm(styled)
	if len(result.Aggregates.MotifRuns) != 0 {
		t.Errorf("ribbon panels are tiles; the run should end: %+v", result.Aggregates.MotifRuns)
	}
	if got := result.PerSlide[3].Motif; got != string(patterns.MotifTiles) {
		t.Errorf("stylish-panels style ribbon motif = %q, want tiles", got)
	}
}

// A deck alternating chart / table / tile row is not flagged.
func TestRhythmMotif_VariedDeckIsClean(t *testing.T) {
	var slides []SlideInput
	for _, kind := range []string{"bar", "line", "pie"} {
		slides = append(slides,
			motifChartSlide(kind, "Revenue grew 12% in "+kind),
			motifTableSlide("Costs fell in every region "+kind),
			motifPatternSlide("card-grid", "Six levers close the gap "+kind, ""),
		)
	}
	result := analyzeDeckRhythm(slides)
	if len(result.Aggregates.MotifRuns) != 0 || result.Aggregates.DominantMotif != "" {
		t.Fatalf("varied deck flagged: %+v dominant %q", result.Aggregates.MotifRuns, result.Aggregates.DominantMotif)
	}
	codes := compositionCodesOf(compositionAxis(slides))
	for _, code := range []string{"motif_run", "motif_dominance", "pattern_run"} {
		if msg, ok := codes[code]; ok {
			t.Errorf("unexpected %s: %s", code, msg)
		}
	}
}

// The two halves of a split table are one exhibit for every monotony check,
// and a divider between them is reported.
func TestRhythmContinuation_SplitTable(t *testing.T) {
	deck := []SlideInput{
		motifTableSlide("Savings by lever (1/2)"),
		motifTableSlide("Savings by lever (2/2)"),
		motifTableSlide("Options compared on four criteria"),
	}
	if codes := compositionCodesOf(compositionAxis(deck)); codes["pattern_run"] != "" || codes["motif_run"] != "" {
		t.Errorf("split halves drew a run diagnostic: %v", codes)
	}

	// DECK_MONOTONY: five table slides of which two are one split exhibit are
	// four exhibits (review), not five; with the split they would be a run of
	// four exhibits only when counted separately.
	four := &PresentationInput{Slides: []SlideInput{
		motifTableSlide("Savings by lever (1/2)"),
		motifTableSlide("Savings by lever (2/2)"),
		motifTableSlide("Costs by region"),
		motifTableSlide("Options compared"),
	}}
	if got := collectMonotonyFindings(four); len(got) != 0 {
		t.Errorf("three exhibits (one split in two) reported as DECK_MONOTONY: %+v", got)
	}
	four.Slides[1] = motifTableSlide("Headcount by team")
	if got := collectMonotonyFindings(four); len(got) != 1 {
		t.Errorf("four separate table slides must still report DECK_MONOTONY, got %+v", got)
	}

	divider := "Savings levers"
	interrupted := []SlideInput{
		motifTableSlide("Savings by lever (1/2)"),
		{SlideType: "section", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &divider}}},
		motifTableSlide("Savings by lever (2/2)"),
	}
	codes := compositionCodesOf(compositionAxis(interrupted))
	msg, ok := codes["continuation_interrupted"]
	if !ok {
		t.Fatalf("no continuation_interrupted diagnostic: %v", codes)
	}
	if !strings.Contains(msg, "section divider") {
		t.Errorf("message does not name the divider: %q", msg)
	}
	var rec bool
	for _, r := range analyzeDeckRhythm(interrupted).Recommendations {
		rec = rec || (r.Code == "continuation_interrupted" && r.SlideIndex == 1)
	}
	if !rec {
		t.Errorf("analyze_deck_rhythm did not report continuation_interrupted at slide 1")
	}
}
