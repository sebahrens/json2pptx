package rhythm_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestAnalyze_BasicRun(t *testing.T) {
	// 5 slides all using card-grid pattern should trigger a run detection.
	slides := make([]rhythm.Slide, 5)
	for i := range slides {
		slides[i] = rhythm.Slide{HasPattern: true, PatternName: "card-grid"}
	}

	result := rhythm.Analyze(slides)

	if len(result.PerSlide) != 5 {
		t.Fatalf("expected 5 per_slide entries, got %d", len(result.PerSlide))
	}

	for i, s := range result.PerSlide {
		if s.Pattern != "card-grid" {
			t.Errorf("slide %d: expected pattern card-grid, got %q", i, s.Pattern)
		}
		if s.DominantVisual != "pattern" {
			t.Errorf("slide %d: expected dominant_visual pattern, got %q", i, s.DominantVisual)
		}
	}

	if result.Aggregates.LongestRun != 5 {
		t.Errorf("expected longest_run=5, got %d", result.Aggregates.LongestRun)
	}

	if len(result.Aggregates.PatternRuns) != 1 {
		t.Fatalf("expected 1 pattern run, got %d", len(result.Aggregates.PatternRuns))
	}
	run := result.Aggregates.PatternRuns[0]
	if run.Name != "card-grid" || run.Start != 0 || run.Len != 5 {
		t.Errorf("unexpected run: %+v", run)
	}

	// Repetition index: 1 - (unique/total) = 1 - 1/5 = 0.8
	if result.Aggregates.RepetitionIndex != 0.8 {
		t.Errorf("expected repetition_index=0.8, got %f", result.Aggregates.RepetitionIndex)
	}

	if len(result.Recommendations) == 0 {
		t.Error("expected at least one recommendation for a 5-slide run")
	}

	if result.CompositionScore >= 100 {
		t.Errorf("expected composition_score < 100 for a monotonous deck, got %d", result.CompositionScore)
	}
}

// khzni: appendix back matter neither forms nor extends a pattern run.
func TestAnalyze_AppendixIsOutsideRuns(t *testing.T) {
	slides := make([]rhythm.Slide, 6)
	for i := range slides {
		slides[i] = rhythm.Slide{HasPattern: true, PatternName: "card-grid", Appendix: i >= 2}
	}
	result := rhythm.Analyze(slides)
	if result.Aggregates.LongestRun != 2 {
		t.Errorf("longest_run = %d, want 2 (the four backup pages are exempt): %+v", result.Aggregates.LongestRun, result.Aggregates.PatternRuns)
	}
	for _, r := range result.Recommendations {
		if r.Code == rhythm.CodeBreakRun {
			t.Errorf("break_run recommended inside the appendix: %+v", r)
		}
	}
}

func TestAnalyze_AlternatingKPIsAreOneVisualRun(t *testing.T) {
	slides := make([]rhythm.Slide, 8)
	for i := range slides {
		name := "kpi-3up"
		if i%2 == 1 {
			name = "kpi-4up"
		}
		slides[i] = rhythm.Slide{HasPattern: true, PatternName: name}
	}
	result := rhythm.Analyze(slides)
	if len(result.Aggregates.PatternRuns) != 1 || result.Aggregates.PatternRuns[0].Len != 8 {
		t.Fatalf("alternating KPIs should form one 8-slide run, got %+v", result.Aggregates.PatternRuns)
	}
	if result.Aggregates.RepetitionIndex != 0.88 {
		t.Errorf("family repetition index = %v, want 0.88", result.Aggregates.RepetitionIndex)
	}
	if result.CompositionScore >= 60 {
		t.Errorf("8-slide KPI rut scored %d, want below 60", result.CompositionScore)
	}
	if len(result.Recommendations) == 0 || len(result.Recommendations[0].RecommendedBreak) == 0 {
		t.Fatalf("KPI run has no break recommendations: %+v", result.Recommendations)
	}
	if result.Recommendations[0].RecommendedBreak[0] == "kpi-3up" || result.Recommendations[0].RecommendedBreak[0] == "kpi-4up" {
		t.Errorf("KPI break should use another family: %+v", result.Recommendations[0])
	}
}

func TestAnalyze_BreakSuggestionsUseContentHints(t *testing.T) {
	for _, tt := range []struct {
		name      string
		pattern   string
		kind      string
		slideType string
		want      string
	}{
		{"chart", "kpi-3up", "chart", "", "chart-insights-split"},
		{"table", "kpi-3up", "table", "", "table-highlight"},
		// No dates on the slide: no timeline (go-slide-creator-hl17m).
		{"diagram", "kpi-3up", "diagram", "", "journey-maturity-model"},
		{"image", "kpi-3up", "image", "", "image-text-split"},
		{"process", "process-flow", "", "", "journey-maturity-model"},
		{"cards", "card-grid", "", "", "comparison-2col"},
		{"comparison", "pull-quote", "", "comparison", "comparison-2col"},
		// Prose without numbers, options or dates gets text-shaped patterns,
		// never an invented hero number.
		{"fallback", "pull-quote", "", "", "labeled-rows"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			second := tt.pattern
			if tt.pattern == "kpi-3up" {
				second = "kpi-4up"
			}
			slides := []rhythm.Slide{
				{HasPattern: true, PatternName: tt.pattern},
				{HasPattern: true, PatternName: second},
				{HasPattern: true, PatternName: tt.pattern, ContentKinds: []string{tt.kind}, SlideType: tt.slideType},
			}
			result := rhythm.Analyze(slides)
			if len(result.Recommendations) == 0 || len(result.Recommendations[0].RecommendedBreak) == 0 {
				t.Fatalf("no break recommendation: %+v", result.Recommendations)
			}
			if got := result.Recommendations[0].RecommendedBreak[0]; got != tt.want {
				t.Errorf("first %s break = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestAnalyze_BreakSuggestionsIgnoreContentOrder(t *testing.T) {
	for _, kinds := range [][]string{{"table", "chart"}, {"chart", "table"}} {
		slides := []rhythm.Slide{
			{HasPattern: true, PatternName: "kpi-3up"},
			{HasPattern: true, PatternName: "kpi-4up"},
			{HasPattern: true, PatternName: "kpi-3up", ContentKinds: kinds},
		}
		result := rhythm.Analyze(slides)
		if got := result.Recommendations[0].RecommendedBreak[0]; got != "chart-insights-split" {
			t.Errorf("content kinds %v: first break = %q, want chart-insights-split", kinds, got)
		}
	}
}

func TestAnalyze_VisualFamiliesAndBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name     string
		patterns []string
		wantRun  int
	}{
		{"compact variant", []string{"process-flow", "process-flow-compact", "process-flow"}, 3},
		{"process steps", []string{"process-flow", "numbered-step-strip", "process-grid-2row"}, 3},
		{"card panels", []string{"card-grid", "stylish-panels", "icon-row"}, 3},
		{"different families", []string{"kpi-3up", "process-flow", "stat-hero"}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slides := make([]rhythm.Slide, len(tt.patterns))
			for i, name := range tt.patterns {
				slides[i] = rhythm.Slide{HasPattern: true, PatternName: name}
			}
			result := rhythm.Analyze(slides)
			if result.Aggregates.LongestRun != tt.wantRun {
				t.Errorf("longest run = %d, want %d", result.Aggregates.LongestRun, tt.wantRun)
			}
		})
	}
}

func TestAnalyze_MixedPatterns(t *testing.T) {
	slides := []rhythm.Slide{
		{HasPattern: true, PatternName: "kpi-3up"},
		{HasPattern: true, PatternName: "card-grid"},
		{HasPattern: true, PatternName: "timeline-horizontal"},
		{SlideType: "title"},
	}

	result := rhythm.Analyze(slides)

	if result.Aggregates.LongestRun != 0 {
		t.Errorf("expected no runs for all-different patterns, got longest_run=%d", result.Aggregates.LongestRun)
	}
	if result.Aggregates.RepetitionIndex != 0.0 {
		t.Errorf("expected repetition_index=0.0, got %f", result.Aggregates.RepetitionIndex)
	}
	if result.CompositionScore != 100 {
		t.Errorf("expected perfect composition_score for varied deck, got %d", result.CompositionScore)
	}
}

func TestAnalyze_AccentBalance(t *testing.T) {
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 1, CellAccents: []string{"accent1"}},
		{HasShapeGrid: true, CellCount: 1, CellAccents: []string{"accent2"}},
	}

	result := rhythm.Analyze(slides)

	if len(result.Aggregates.AccentBalance) != 2 {
		t.Errorf("expected 2 accents in balance, got %d", len(result.Aggregates.AccentBalance))
	}
	for accent, frac := range result.Aggregates.AccentBalance {
		if frac != 0.5 {
			t.Errorf("accent %s: expected 0.5 fraction, got %f", accent, frac)
		}
	}
}

func TestAnalyze_DensityClasses(t *testing.T) {
	slides := []rhythm.Slide{
		// Low density: single text content.
		{ContentKinds: []string{"text"}},
		// High density: table + chart + bullets.
		{ContentKinds: []string{"table", "chart", "bullets"}},
	}

	result := rhythm.Analyze(slides)

	if result.PerSlide[0].DensityClass != "low" {
		t.Errorf("slide 0: expected density_class=low, got %q", result.PerSlide[0].DensityClass)
	}
	if result.PerSlide[1].DensityClass != "high" {
		t.Errorf("slide 1: expected density_class=high, got %q", result.PerSlide[1].DensityClass)
	}
	if result.Aggregates.DensityCV == 0 {
		t.Error("expected non-zero density_cv for slides with different densities")
	}
}

func TestAnalyze_EmptySlides(t *testing.T) {
	slides := []rhythm.Slide{{}, {}}

	result := rhythm.Analyze(slides)

	if len(result.PerSlide) != 2 {
		t.Fatalf("expected 2 per_slide entries, got %d", len(result.PerSlide))
	}
	for i, s := range result.PerSlide {
		if s.Pattern != "content" {
			t.Errorf("slide %d: expected pattern=content, got %q", i, s.Pattern)
		}
	}
}

func TestAnalyze_RecommendationIndices(t *testing.T) {
	// 10 identical slides — should get recommendations at indices 2, 5, 8.
	slides := make([]rhythm.Slide, 10)
	for i := range slides {
		slides[i] = rhythm.Slide{HasPattern: true, PatternName: "card-grid"}
	}

	result := rhythm.Analyze(slides)

	expectedIndices := map[int]bool{2: true, 5: true, 8: true}
	for _, rec := range result.Recommendations {
		if rec.Code != rhythm.CodeBreakRun {
			continue // the deck-level motif_dominant advice is not a break point
		}
		if !expectedIndices[rec.SlideIndex] {
			t.Errorf("unexpected recommendation at slide_index=%d", rec.SlideIndex)
		}
		delete(expectedIndices, rec.SlideIndex)
	}
	for idx := range expectedIndices {
		t.Errorf("missing recommendation at slide_index=%d", idx)
	}
}

func TestAnalyze_WithinSlideAccentVariety(t *testing.T) {
	// Slide with 3 distinct accents across cells.
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 3, CellAccents: []string{"accent1", "accent2", "accent3"}},
	}

	result := rhythm.Analyze(slides)

	if result.PerSlide[0].WithinSlideAccentVariety != 3 {
		t.Errorf("expected within_slide_accent_variety=3, got %d", result.PerSlide[0].WithinSlideAccentVariety)
	}
}

// TestAnalyze_NoMoreAccentAdvice pins the removed rule
// (go-slide-creator-hl17m): a restrained slide — many cells, one accent —
// is the standard, not a defect, so nothing asks for more accents.
func TestAnalyze_NoMoreAccentAdvice(t *testing.T) {
	accents := []string{"accent1", "accent1", "accent1", "accent1", "accent1", "accent1"}
	for _, s := range []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 6, CellAccents: accents},
		{HasPattern: true, PatternName: "card-grid", CellCount: 6, CellAccents: accents},
		{HasPattern: true, PatternName: "kpi-6up", CellCount: 6},
	} {
		for _, rec := range rhythm.Analyze([]rhythm.Slide{s}).Recommendations {
			for _, b := range rec.RecommendedBreak {
				if b == "cell_accent_mode: progressive" {
					t.Errorf("%s: still recommends progressive accents: %+v", s.PatternName, rec)
				}
			}
			if rec.SlideIndex == 0 {
				t.Errorf("%s: unexpected per-slide accent advice: %+v", s.PatternName, rec)
			}
		}
	}
}

// TestAnalyze_AccentHeaviness: a slide with many solid accent cells, and a
// run of AccentWeight=strong slides, are what get flagged now.
func TestAnalyze_AccentHeaviness(t *testing.T) {
	heavy := rhythm.Slide{HasShapeGrid: true, CellCount: 6, SolidAccentCells: 5, CellAccents: []string{"accent1"}}
	recs := rhythm.Analyze([]rhythm.Slide{heavy}).Recommendations
	if len(recs) != 1 || recs[0].Code != rhythm.CodeAccentHeavySlide {
		t.Fatalf("solid-accent slide recs = %+v", recs)
	}

	strong := []rhythm.Slide{
		{HasPattern: true, PatternName: "stat-hero"},
		{HasPattern: true, PatternName: "hero-detail"},
		{HasPattern: true, PatternName: "metric-list"},
		{HasPattern: true, PatternName: "table-highlight"},
	}
	found := false
	for _, rec := range rhythm.Analyze(strong).Recommendations {
		if rec.Code == rhythm.CodeStrongAccentRun {
			found = true
			if rec.SlideIndex != 2 {
				t.Errorf("strong run flagged at %d, want 2", rec.SlideIndex)
			}
			for _, b := range rec.RecommendedBreak {
				if b == "stat-hero" || b == "metric-list" || b == "kpi-3up" {
					t.Errorf("strong-run break proposes another strong pattern %q", b)
				}
			}
		}
	}
	if !found {
		t.Error("three consecutive strong-accent slides not flagged")
	}
}

func TestAnalyze_NoAccentVarietyRecForFewCells(t *testing.T) {
	// Slide with 3 cells and 1 accent — should NOT trigger recommendation (< 5 cells).
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 3, CellAccents: []string{"accent1", "accent1", "accent1"}},
	}

	result := rhythm.Analyze(slides)

	if len(result.Recommendations) != 0 {
		t.Errorf("should not recommend accent variety for slide with < 5 cells: %+v", result.Recommendations)
	}
}

func TestAnalyze_ContentVisualBreaksFalseRun(t *testing.T) {
	slides := []rhythm.Slide{
		{SlideType: "content", ContentKinds: []string{"text", "chart", "bullets"}},
		{SlideType: "content", ContentKinds: []string{"bullets"}},
		{SlideType: "content", ContentKinds: []string{"bullets"}},
	}
	result := rhythm.Analyze(slides)
	if got := result.PerSlide[0].Pattern; got != "content" {
		t.Errorf("chart slide pattern = %q, want content", got)
	}
	if result.Aggregates.LongestRun != 2 || len(result.Recommendations) != 0 {
		t.Errorf("chart + bullets + bullets must not form a 3-slide run: %+v", result.Aggregates)
	}
	if result.Aggregates.RepetitionIndex != 0.33 {
		t.Errorf("visual repetition index = %.2f, want .33", result.Aggregates.RepetitionIndex)
	}
	charts := []rhythm.Slide{
		{SlideType: "content", ContentKinds: []string{"chart"}},
		{SlideType: "content", ContentKinds: []string{"bullets", "chart"}},
		{SlideType: "content", ContentKinds: []string{"chart", "bullets"}},
	}
	if got := rhythm.Analyze(charts).Aggregates.LongestRun; got != 3 {
		t.Errorf("three content charts should still form a run, got %d", got)
	}
}

func recCodes(r *rhythm.Result) map[string][]rhythm.Recommendation {
	out := map[string][]rhythm.Recommendation{}
	for _, rec := range r.Recommendations {
		out[rec.Code] = append(out[rec.Code], rec)
	}
	return out
}

func bulletsSlide(title, text string) rhythm.Slide {
	return rhythm.Slide{SlideType: "content", Title: title, Text: text, ContentKinds: []string{"text", "bullets"}}
}

// TestAnalyze_NarrativeStructure pins go-slide-creator-hl17m on the review's
// mixed deck shape: topic-titled bullets, no sources, a "Thank you" close.
func TestAnalyze_NarrativeStructure(t *testing.T) {
	slides := []rhythm.Slide{
		{Role: "title", Title: "FY26 plan"},
		bulletsSlide("Margin analysis", "Gross margin fell from 41% to 38%"),
		{HasPattern: true, PatternName: "kpi-4up", Title: "Key metrics"},
		bulletsSlide("Market overview", "The market is consolidating"),
		{SlideType: "content", Title: "Revenue", ContentKinds: []string{"text", "chart"}, HasTakeaway: true},
		bulletsSlide("Options", "Option A versus option B"),
		{HasPattern: true, PatternName: "kpi-4up", Title: "More metrics", HasTakeaway: true, HasSource: true},
		bulletsSlide("Risks", "Execution risk is moderate"),
		{Role: "closing", Title: "Thank you"},
	}
	codes := recCodes(rhythm.Analyze(slides))
	if r := codes[rhythm.CodeMissingExecutiveSummary]; len(r) != 1 || r[0].SlideIndex != 1 {
		t.Errorf("missing exec summary = %+v", r)
	}
	if r := codes[rhythm.CodeMissingNextSteps]; len(r) != 1 || r[0].SlideIndex != 8 {
		t.Errorf("missing next steps = %+v", r)
	}
	// Slide 2 (kpi, no takeaway/source) and 4 (chart, no source) flagged; 6 is complete.
	ev := codes[rhythm.CodeEvidenceMissingTakeawaySrc]
	if len(ev) != 2 || ev[0].SlideIndex != 2 || ev[1].SlideIndex != 4 {
		t.Errorf("evidence recs = %+v", ev)
	}
	bh := codes[rhythm.CodeBulletsHeavy]
	if len(bh) != 1 {
		t.Fatalf("bullets-heavy recs = %+v", bh)
	}
	for _, b := range bh[0].RecommendedBreak {
		if b == "timeline-horizontal" {
			t.Errorf("bullets break proposes a timeline without dates: %v", bh[0].RecommendedBreak)
		}
	}
	if bh[0].RecommendedBreak[0] != "kpi-3up" {
		t.Errorf("numeric bullets should suggest a KPI visual first: %v", bh[0].RecommendedBreak)
	}
	if _, ok := codes[rhythm.CodeMissingSections]; ok {
		t.Error("a 7-content-slide deck does not need sections")
	}

	// The complete deck: exec summary, sections, next steps — nothing narrative.
	good := []rhythm.Slide{
		{Role: "title", Title: "Plan"},
		{HasPattern: true, PatternName: "exec-summary", Title: "We should expand"},
		{Role: "section", Title: "Market"},
		{SlideType: "content", Title: "Demand grew 12%", ContentKinds: []string{"chart"}, HasTakeaway: true, HasSource: true},
		{HasPattern: true, PatternName: "comparison-2col", Title: "Two options"},
		{HasPattern: true, PatternName: "next-steps", Title: "Next steps"},
	}
	for code := range recCodes(rhythm.Analyze(good)) {
		switch code {
		case rhythm.CodeMissingExecutiveSummary, rhythm.CodeMissingNextSteps, rhythm.CodeEvidenceMissingTakeawaySrc, rhythm.CodeBulletsHeavy:
			t.Errorf("complete deck flagged %s", code)
		}
	}
}

// go-slide-creator-th6o9: a 12-slide deck is one chapter; the advice starts
// at 13 content slides, names where the first chapter starts, and is off for
// a deck whose chrome declines the section tracker.
func TestAnalyze_MissingSectionsOnLongDecks(t *testing.T) {
	deck := func(content int) []rhythm.Slide {
		slides := []rhythm.Slide{{Role: "title", Title: "Deck"}, {HasPattern: true, PatternName: "exec-summary", Title: "Executive summary"}}
		for i := 1; i < content; i++ {
			slides = append(slides, rhythm.Slide{HasPattern: true, PatternName: "card-grid", Title: "Point"})
		}
		return slides
	}
	for content := 10; content <= 12; content++ {
		if r := recCodes(rhythm.Analyze(deck(content)))[rhythm.CodeMissingSections]; len(r) != 0 {
			t.Errorf("%d content slides are one chapter, got %+v", content, r)
		}
	}
	slides := deck(13)
	r := recCodes(rhythm.Analyze(slides))[rhythm.CodeMissingSections]
	if len(r) != 1 {
		t.Fatalf("missing sections recs = %+v", r)
	}
	// The first chapter starts after the cover and the executive summary.
	if r[0].SlideIndex != 2 || !strings.Contains(r[0].Message, "before slide 3") {
		t.Errorf("missing_sections should name where the first divider goes (slide 3), got %+v", r[0])
	}
	if r := recCodes(rhythm.AnalyzeWith(slides, rhythm.Options{SectionsDeclined: true}))[rhythm.CodeMissingSections]; len(r) != 0 {
		t.Errorf("a deck that declines the section tracker still got %+v", r)
	}
	slides = append(slides[:3], append([]rhythm.Slide{{Role: "section", Title: "Part two"}}, slides[3:]...)...)
	if r := recCodes(rhythm.Analyze(slides))[rhythm.CodeMissingSections]; len(r) != 0 {
		t.Errorf("deck with a divider still flagged: %+v", r)
	}
}

// TestAnalyze_BulletRunBreaksFromContent: a run of plain bullets slides gets
// alternatives from what the slide says, and a timeline only with dates.
func TestAnalyze_BulletRunBreaksFromContent(t *testing.T) {
	for _, tc := range []struct {
		text, want string
	}{
		{"Revenue up 12% to $48M", "kpi-3up"},
		{"Option A versus option B", "comparison-2col"},
		{"Launch in Q3 2026, scale by March 2027", "timeline-horizontal"},
		{"Culture matters and teams must align", "labeled-rows"},
	} {
		slides := []rhythm.Slide{bulletsSlide("A", "x"), bulletsSlide("B", "y"), bulletsSlide("C", tc.text)}
		var brk []string
		for _, rec := range rhythm.Analyze(slides).Recommendations {
			if rec.Code == rhythm.CodeBreakRun {
				brk = rec.RecommendedBreak
			}
		}
		if len(brk) == 0 || brk[0] != tc.want {
			t.Errorf("%q: break = %v, want %s first", tc.text, brk, tc.want)
		}
		if tc.want != "timeline-horizontal" {
			for _, b := range brk {
				if b == "timeline-horizontal" || b == "phase-roadmap" {
					t.Errorf("%q: proposes %s without dates", tc.text, b)
				}
			}
		}
	}
}

// TestAnalyze_PatternAccentBalance: pattern slides contribute their resolved
// accent, so accent_balance is measured on the pattern / DeckSpec path.
func TestAnalyze_PatternAccentBalance(t *testing.T) {
	slides := []rhythm.Slide{
		{HasPattern: true, PatternName: "kpi-3up", PatternAccent: "accent1"},
		{HasPattern: true, PatternName: "card-grid", PatternAccent: "accent2"},
	}
	r := rhythm.Analyze(slides)
	if r.Aggregates.AccentBalance["accent1"] != 0.5 || r.Aggregates.AccentBalance["accent2"] != 0.5 {
		t.Errorf("accent_balance = %v", r.Aggregates.AccentBalance)
	}
	if r.PerSlide[1].AccentRole != "accent2" {
		t.Errorf("accent_role = %q", r.PerSlide[1].AccentRole)
	}
}

// go-slide-creator-th6o9 (h-A12): "100% of cells (2/2) are underfilled" came
// back at slide_index -1 with kpi-3up and comparison-2col as the fix for a
// deck that already used both. Every recommendation names a slide.
func TestAnalyze_EveryRecommendationNamesASlide(t *testing.T) {
	emptyText, _ := json.Marshal("")
	sparse := func(cells int) *shapegrid.Grid {
		row := shapegrid.Row{}
		cols := make([]float64, cells)
		for i := range cols {
			cols[i] = 100 / float64(cells)
			row.Cells = append(row.Cells, shapegrid.Cell{Shape: &shapegrid.ShapeSpec{Geometry: "rect", Text: emptyText}})
		}
		return &shapegrid.Grid{
			Bounds:  shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU),
			Columns: cols, Rows: []shapegrid.Row{row},
		}
	}
	slides := []rhythm.Slide{
		{Role: "title", Title: "FY26 plan"},
		bulletsSlide("Margin analysis", "Gross margin fell from 41% to 38%"),
		{HasShapeGrid: true, CellCount: 2, Grid: sparse(2), Title: "Two cells"},
		bulletsSlide("Market overview", "The market is consolidating"),
		{HasPattern: true, PatternName: "kpi-3up", HasShapeGrid: true, CellCount: 3, Grid: sparse(3), Title: "Three cells"},
		bulletsSlide("Risks", "Execution risk is moderate"),
	}
	result := rhythm.Analyze(slides)
	codes := recCodes(result)
	for _, rec := range result.Recommendations {
		if rec.SlideIndex < 0 || rec.SlideIndex >= len(slides) {
			t.Errorf("%s names no slide (slide_index %d): %s", rec.Code, rec.SlideIndex, rec.Message)
		}
	}
	under := codes[rhythm.CodeUnderfilledCells]
	if len(under) != 1 {
		t.Fatalf("underfilled recs = %+v", under)
	}
	// The slide with the most underfilled cells is the one to fix first, and
	// its own pattern is not offered as the way out.
	if under[0].SlideIndex != 4 || !strings.Contains(under[0].Message, "slide 5") || !strings.Contains(under[0].Message, "slides 3, 5") {
		t.Errorf("underfilled_cells should point at slide 5 and list slides 3, 5: %+v", under[0])
	}
	for _, name := range under[0].RecommendedBreak {
		if name == "kpi-3up" {
			t.Errorf("underfilled_cells recommends the pattern the slide already uses: %v", under[0].RecommendedBreak)
		}
	}
	if bh := codes[rhythm.CodeBulletsHeavy]; len(bh) != 1 || bh[0].SlideIndex != 1 {
		t.Errorf("bullets_heavy should point at the first bullets-only slide: %+v", bh)
	}
}

func TestAnalyze_DensityDistributionZero(t *testing.T) {
	// Slides with no grid — density distribution should be all zeros.
	slides := []rhythm.Slide{
		{ContentKinds: []string{"text"}},
		{HasPattern: true, PatternName: "card-grid"},
	}

	result := rhythm.Analyze(slides)

	dd := result.Aggregates.DensityDistribution
	if dd.UnderfilledCells != 0 || dd.OptimalCells != 0 || dd.OverflowCells != 0 {
		t.Errorf("expected all zeros for non-grid slides, got %+v", dd)
	}
}

func TestAnalyze_DensityDistributionWithGrid(t *testing.T) {
	// Slide with a real shape_grid of two empty-text cells — both classify as
	// underfilled, so the density distribution should report 2 underfilled cells.
	emptyText, _ := json.Marshal("")
	grid := &shapegrid.Grid{
		Bounds:  shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU),
		Columns: []float64{50, 50},
		Rows: []shapegrid.Row{
			{Cells: []shapegrid.Cell{
				{Shape: &shapegrid.ShapeSpec{Geometry: "rect", Text: emptyText}},
				{Shape: &shapegrid.ShapeSpec{Geometry: "rect", Text: emptyText}},
			}},
		},
	}
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 2, Grid: grid},
	}

	result := rhythm.Analyze(slides)

	dd := result.Aggregates.DensityDistribution
	total := dd.UnderfilledCells + dd.OptimalCells + dd.OverflowCells
	if total != 2 {
		t.Errorf("expected 2 total cells in density distribution, got %d (%+v)", total, dd)
	}
	if dd.UnderfilledCells != 2 {
		t.Errorf("expected 2 underfilled cells for empty text, got %d", dd.UnderfilledCells)
	}

	// A cell that holds a table or an image is not a text box short of text
	// (go-slide-creator-th6o9): a chart beside a table was "100% of cells
	// (2/2) are underfilled".
	visuals := &shapegrid.Grid{
		Bounds:  shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU),
		Columns: []float64{50, 50},
		Rows: []shapegrid.Row{
			{Cells: []shapegrid.Cell{
				{TableSpec: &types.TableSpec{Headers: []string{"A", "B"}, Rows: [][]types.TableCell{{{Content: "1"}, {Content: "2"}}}}},
				{Image: &shapegrid.ImageSpec{Path: "chart.png"}},
			}},
		},
	}
	result = rhythm.Analyze([]rhythm.Slide{{HasShapeGrid: true, CellCount: 2, Grid: visuals, Title: "Chart beside table"}})
	if dd := result.Aggregates.DensityDistribution; dd.UnderfilledCells != 0 {
		t.Errorf("table and image cells counted as underfilled text: %+v", dd)
	}
	if recs := recCodes(result)[rhythm.CodeUnderfilledCells]; len(recs) != 0 {
		t.Errorf("a slide of visuals got underfilled_cells advice: %+v", recs)
	}
}
