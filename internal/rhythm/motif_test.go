package rhythm_test

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/rhythm"
)

func patternSlide(name, title, text string) rhythm.Slide {
	return rhythm.Slide{HasPattern: true, PatternName: name, Title: title, Text: text}
}

func recsWithCode(r *rhythm.Result, code string) []rhythm.Recommendation {
	var out []rhythm.Recommendation
	for _, rec := range r.Recommendations {
		if rec.Code == code {
			out = append(out, rec)
		}
	}
	return out
}

// A deck that alternates kpi-4up, stylish-panels and card-grid uses three
// patterns and no pattern run, and still shows a row of look-alike columns on
// most slides (go-slide-creator-rd7oj).
func TestMotif_AlternatingLookAlikePatternsAreFlagged(t *testing.T) {
	var slides []rhythm.Slide
	for i := 0; i < 2; i++ {
		slides = append(slides,
			patternSlide("kpi-4up", "Revenue grew 12% across four regions", "$4.2M ARR 127% NRR"),
			patternSlide("stylish-panels", "Three pillars carry the plan", "Strategy Execution Measurement"),
			patternSlide("card-grid", "Six levers close the gap", "Pricing Procurement Footprint"),
		)
	}
	result := rhythm.Analyze(slides)

	if result.Aggregates.LongestRun >= 3 {
		t.Fatalf("no pattern run expected, got longest_run %d", result.Aggregates.LongestRun)
	}
	if got := result.Aggregates.DominantMotif; got != string(patterns.MotifOpenColumns) {
		t.Fatalf("dominant_motif = %q, want %q; share %v", got, patterns.MotifOpenColumns, result.Aggregates.MotifShare)
	}
	recs := recsWithCode(result, rhythm.CodeMotifDominant)
	if len(recs) != 1 {
		t.Fatalf("want one motif_dominant recommendation, got %+v", result.Recommendations)
	}
	if len(recs[0].RecommendedBreak) == 0 {
		t.Fatalf("motif_dominant carries no alternatives")
	}
	// It names the first slide that draws the motif (go-slide-creator-th6o9).
	if recs[0].SlideIndex != 0 {
		t.Errorf("motif_dominant slide_index = %d, want the first slide of the motif (0)", recs[0].SlideIndex)
	}
	for _, name := range recs[0].RecommendedBreak {
		if patterns.PatternMotif(name) == patterns.MotifOpenColumns {
			t.Errorf("alternative %q draws the motif the deck should leave", name)
		}
	}
	if result.CompositionScore >= 100 {
		t.Errorf("composition score %d: a dominant motif across patterns must cost points", result.CompositionScore)
	}
}

// Three different patterns in a row that draw one motif are a run.
func TestMotif_RunAcrossPatterns(t *testing.T) {
	slides := []rhythm.Slide{
		patternSlide("pyramid", "The strategy stacks in five layers", ""),
		patternSlide("kpi-3up", "Three numbers moved in 2025: 12%, 4.2M, 127%", "12% 4.2M 127%"),
		patternSlide("icon-row", "Three capabilities matter", "Launch Growth Revenue"),
		patternSlide("team-bios", "Option A vs option B team", "Jane Arun Lila"),
		patternSlide("table-highlight", "Buying beats building", ""),
	}
	result := rhythm.Analyze(slides)
	if len(result.Aggregates.MotifRuns) != 1 {
		t.Fatalf("want one motif run, got %+v", result.Aggregates.MotifRuns)
	}
	run := result.Aggregates.MotifRuns[0]
	if run.Motif != string(patterns.MotifOpenColumns) || run.Start != 1 || run.End != 3 || run.Len != 3 {
		t.Fatalf("run = %+v, want open-columns 1..3 len 3", run)
	}
	recs := recsWithCode(result, rhythm.CodeBreakMotifRun)
	if len(recs) != 1 || recs[0].SlideIndex != 3 {
		t.Fatalf("want one break_motif_run at slide 3, got %+v", recs)
	}
	for _, name := range recs[0].RecommendedBreak {
		if patterns.PatternMotif(name) == patterns.MotifOpenColumns {
			t.Errorf("break alternative %q is the run's own motif", name)
		}
	}
	// "Option A vs option B" is an options slide: the alternative should be a
	// comparison-shaped pattern of another motif, not an invented number.
	if !strings.Contains(strings.Join(recs[0].RecommendedBreak, ","), "table-highlight") {
		t.Errorf("alternatives %v do not follow the slide's content (options)", recs[0].RecommendedBreak)
	}
	// The run costs what a pattern run of the same length costs: the same
	// deck with its middle slide redrawn as a table scores 10 more (and loses
	// the dominant motif, another 10).
	varied := append([]rhythm.Slide(nil), slides...)
	varied[2] = patternSlide("table-highlight", "Three capabilities matter", "")
	if got, base := result.CompositionScore, rhythm.Analyze(varied).CompositionScore; got != base-20 {
		t.Errorf("composition score = %d, want %d (varied deck %d less 10 for the run and 10 for the dominant motif)", got, base-20, base)
	}
}

// A deck alternating chart, table and card grid is varied.
func TestMotif_VariedDeckIsNotFlagged(t *testing.T) {
	var slides []rhythm.Slide
	for i := 0; i < 3; i++ {
		slides = append(slides,
			rhythm.Slide{ContentKinds: []string{"chart"}, MotifVariant: []string{"bar", "line", "pie"}[i], Title: "Revenue rises"},
			rhythm.Slide{ContentKinds: []string{"table"}, Title: "Costs fall"},
			patternSlide("card-grid", "Six levers close the gap", ""),
		)
	}
	result := rhythm.Analyze(slides)
	if len(result.Aggregates.MotifRuns) != 0 || result.Aggregates.DominantMotif != "" {
		t.Fatalf("varied deck flagged: runs %+v dominant %q", result.Aggregates.MotifRuns, result.Aggregates.DominantMotif)
	}
	for _, code := range []string{rhythm.CodeBreakMotifRun, rhythm.CodeMotifDominant} {
		if recs := recsWithCode(result, code); len(recs) != 0 {
			t.Errorf("unexpected %s: %+v", code, recs)
		}
	}
	// card-grid's default is the open card: open columns, not tiles.
	for motif, want := range map[string]float64{"chart": 0.33, "table": 0.33, "open-columns": 0.33} {
		if got := result.Aggregates.MotifShare[motif]; got != want {
			t.Errorf("motif_share[%s] = %v, want %v", motif, got, want)
		}
	}
}

// Three different diagrams are not look-alikes; three bar charts are.
func TestMotif_DistinctiveMotifsNeedTheSameVariant(t *testing.T) {
	diagrams := []rhythm.Slide{
		patternSlide("pyramid", "a", ""), patternSlide("strategy-house", "b", ""), patternSlide("driver-tree", "c", ""),
	}
	if runs := rhythm.Analyze(diagrams).Aggregates.MotifRuns; len(runs) != 0 {
		t.Errorf("three different diagrams formed a motif run: %+v", runs)
	}
	bars := make([]rhythm.Slide, 3)
	for i := range bars {
		bars[i] = rhythm.Slide{ContentKinds: []string{"chart"}, Motif: "chart", MotifVariant: "bar"}
	}
	if runs := rhythm.Analyze(bars).Aggregates.MotifRuns; len(runs) != 1 {
		t.Errorf("three bar charts should form a motif run, got %+v", runs)
	}
}

// An explicit style changes the motif the caller projects.
func TestMotif_ExplicitStyleChangesTheRun(t *testing.T) {
	slides := []rhythm.Slide{
		patternSlide("kpi-3up", "a", ""),
		patternSlide("icon-row", "b", ""),
		{HasPattern: true, PatternName: "stylish-panels", Title: "c", Motif: string(patterns.MotifTiles)},
	}
	if runs := rhythm.Analyze(slides).Aggregates.MotifRuns; len(runs) != 0 {
		t.Errorf("a ribbon-style panel slide is tiles, not open columns: %+v", runs)
	}
}

// Structural slides and back matter neither form nor extend a motif run, and
// do not count as content slides.
func TestMotif_StructuralAndAppendixSlidesAreOutside(t *testing.T) {
	slides := []rhythm.Slide{
		{Role: "title", Title: "Deck"},
		patternSlide("kpi-3up", "a", ""),
		patternSlide("icon-row", "b", ""),
		{Role: "section", Title: "Part two"},
		patternSlide("team-bios", "c", ""),
		{HasPattern: true, PatternName: "kpi-4up", Title: "backup", Appendix: true},
	}
	result := rhythm.Analyze(slides)
	if len(result.Aggregates.MotifRuns) != 0 {
		t.Errorf("section divider should break the run: %+v", result.Aggregates.MotifRuns)
	}
	if result.Aggregates.DominantMotif != "" {
		t.Errorf("three content slides are too few for a dominant motif, got %q", result.Aggregates.DominantMotif)
	}
	if result.PerSlide[0].Motif != "none" || result.PerSlide[1].Motif != string(patterns.MotifOpenColumns) {
		t.Errorf("per-slide motifs: %+v", result.PerSlide)
	}
}

// --- continuation slides (go-slide-creator-xy51l) ---

func TestParseContinuation(t *testing.T) {
	cases := []struct {
		title       string
		base        string
		part, total int
		ok          bool
	}{
		{"Savings by lever (1/2)", "savings by lever", 1, 2, true},
		{"Savings by lever (2/2)", "savings by lever", 2, 2, true},
		{"Savings  by lever [2 of 3]", "savings by lever", 2, 3, true},
		{"Savings by lever – 2/3", "savings by lever", 2, 3, true},
		{"Savings by lever (cont.)", "savings by lever", 0, 0, true},
		{"Savings by lever (continued)", "savings by lever", 0, 0, true},
		{"Savings by lever", "", 0, 0, false},
		{"Margin rose to (3/2)", "", 0, 0, false},
		{"(1/2)", "", 0, 0, false},
	}
	for _, c := range cases {
		got, ok := rhythm.ParseContinuation(c.title)
		if ok != c.ok || got.Base != c.base || got.Part != c.part || got.Total != c.total {
			t.Errorf("ParseContinuation(%q) = %+v, %v; want base %q part %d/%d ok %v", c.title, got, ok, c.base, c.part, c.total, c.ok)
		}
	}
	if !rhythm.Continues("Savings by lever (1/2)", "Savings by lever (2/2)") {
		t.Error("(2/2) should continue (1/2)")
	}
	if !rhythm.Continues("Savings by lever", "Savings by lever (cont.)") {
		t.Error("(cont.) should continue the unmarked first part")
	}
	if rhythm.Continues("Savings by lever (1/2)", "Costs by lever (2/2)") {
		t.Error("a different exhibit is not a continuation")
	}
	if rhythm.Continues("Savings by lever (1/3)", "Savings by lever (3/3)") {
		t.Error("part 3 does not directly continue part 1")
	}
}

// The two halves of a split table are one exhibit: with one more table slide
// they are a pair, not a run of three.
func TestContinuation_SplitHalvesAreOneUnit(t *testing.T) {
	table := func(title string) rhythm.Slide {
		return rhythm.Slide{ContentKinds: []string{"table"}, Title: title}
	}
	split := []rhythm.Slide{
		table("Savings by lever (1/2)"), table("Savings by lever (2/2)"), table("Options compared on four criteria"),
	}
	result := rhythm.Analyze(split)
	if result.Aggregates.LongestRun != 2 {
		t.Fatalf("longest_run = %d, want 2 (the halves count once): %+v", result.Aggregates.LongestRun, result.Aggregates.PatternRuns)
	}
	if run := result.Aggregates.PatternRuns[0]; run.Start != 0 || run.End != 2 {
		t.Errorf("run span = %d..%d, want 0..2", run.Start, run.End)
	}
	if recs := recsWithCode(result, rhythm.CodeBreakRun); len(recs) != 0 {
		t.Errorf("split halves drew break_run: %+v", recs)
	}
	if len(result.Aggregates.MotifRuns) != 0 {
		t.Errorf("split halves drew a motif run: %+v", result.Aggregates.MotifRuns)
	}

	// Three separate tables are still a run, and score lower for it.
	separate := []rhythm.Slide{table("Savings by lever"), table("Costs by region"), table("Options compared")}
	sep := rhythm.Analyze(separate)
	if sep.Aggregates.LongestRun != 3 {
		t.Errorf("three separate tables: longest_run = %d, want 3", sep.Aggregates.LongestRun)
	}
	if result.CompositionScore != sep.CompositionScore+10 {
		t.Errorf("composition score = %d, want %d (no run penalty)", result.CompositionScore, sep.CompositionScore+10)
	}
}

// A section divider between the halves of a continued exhibit is reported.
func TestContinuation_StructuralSlideBetweenHalvesIsReported(t *testing.T) {
	slides := []rhythm.Slide{
		{Role: "title", Title: "Cloud cost review"},
		{ContentKinds: []string{"table"}, Title: "Savings by lever (1/2)"},
		{Role: "section", Title: "Savings levers"},
		{ContentKinds: []string{"table"}, Title: "Savings by lever (2/2)"},
		{ContentKinds: []string{"bullets"}, Title: "We recommend option B"},
	}
	result := rhythm.Analyze(slides)
	recs := recsWithCode(result, rhythm.CodeContinuationInterrupted)
	if len(recs) != 1 {
		t.Fatalf("want one continuation_interrupted, got %+v", result.Recommendations)
	}
	if recs[0].SlideIndex != 2 {
		t.Errorf("slide_index = %d, want 2 (the divider)", recs[0].SlideIndex)
	}
	for _, want := range []string{"section divider", "(1/2)", "(2/2)"} {
		if !strings.Contains(recs[0].Message, want) {
			t.Errorf("message %q does not mention %q", recs[0].Message, want)
		}
	}

	// Adjacent halves are fine, and score 10 more.
	adjacent := rhythm.Analyze([]rhythm.Slide{slides[0], slides[2], slides[1], slides[3], slides[4]})
	if recs := recsWithCode(adjacent, rhythm.CodeContinuationInterrupted); len(recs) != 0 {
		t.Errorf("adjacent halves reported: %+v", recs)
	}
	if result.CompositionScore != adjacent.CompositionScore-10 {
		t.Errorf("composition score = %d, want %d", result.CompositionScore, adjacent.CompositionScore-10)
	}
}
