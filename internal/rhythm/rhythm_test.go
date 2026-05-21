package rhythm_test

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
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

func TestAnalyze_AccentVarietyRecommendation(t *testing.T) {
	// Slide with 6 cells all using the same accent — should trigger recommendation.
	accents := make([]string, 6)
	for i := range accents {
		accents[i] = "accent1"
	}
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 6, CellAccents: accents},
	}

	result := rhythm.Analyze(slides)

	found := false
	for _, rec := range result.Recommendations {
		if rec.SlideIndex == 0 && len(rec.RecommendedBreak) > 0 && rec.RecommendedBreak[0] == "cell_accent_mode: progressive" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected accent variety recommendation for 6-cell slide with 1 accent")
	}
}

func TestAnalyze_NoAccentVarietyRecForFewCells(t *testing.T) {
	// Slide with 3 cells and 1 accent — should NOT trigger recommendation (< 5 cells).
	slides := []rhythm.Slide{
		{HasShapeGrid: true, CellCount: 3, CellAccents: []string{"accent1", "accent1", "accent1"}},
	}

	result := rhythm.Analyze(slides)

	for _, rec := range result.Recommendations {
		if rec.SlideIndex == 0 && len(rec.RecommendedBreak) > 0 && rec.RecommendedBreak[0] == "cell_accent_mode: progressive" {
			t.Error("should not recommend accent variety for slide with < 5 cells")
		}
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
}
