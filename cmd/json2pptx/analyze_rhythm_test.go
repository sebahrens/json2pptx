package main

import (
	"encoding/json"
	"testing"
)

func TestAnalyzeDeckRhythm_BasicRun(t *testing.T) {
	// 5 slides all using card-grid pattern should trigger a run detection.
	slides := make([]SlideInput, 5)
	for i := range slides {
		slides[i] = SlideInput{
			Pattern: &PatternInput{Name: "card-grid"},
		}
	}

	result := analyzeDeckRhythm(slides)

	if len(result.PerSlide) != 5 {
		t.Fatalf("expected 5 per_slide entries, got %d", len(result.PerSlide))
	}

	// All slides should have pattern="card-grid".
	for i, s := range result.PerSlide {
		if s.Pattern != "card-grid" {
			t.Errorf("slide %d: expected pattern card-grid, got %q", i, s.Pattern)
		}
		if s.DominantVisual != "pattern" {
			t.Errorf("slide %d: expected dominant_visual pattern, got %q", i, s.DominantVisual)
		}
	}

	// Should detect a run of length 5.
	if result.Aggregates.LongestRun != 5 {
		t.Errorf("expected longest_run=5, got %d", result.Aggregates.LongestRun)
	}

	// Should have exactly one pattern run.
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

	// Should have recommendations for breaking the run.
	if len(result.Recommendations) == 0 {
		t.Error("expected at least one recommendation for a 5-slide run")
	}

	// Composition score should be penalized.
	if result.CompositionScore >= 100 {
		t.Errorf("expected composition_score < 100 for a monotonous deck, got %d", result.CompositionScore)
	}
}

func TestAnalyzeDeckRhythm_MixedPatterns(t *testing.T) {
	slides := []SlideInput{
		{Pattern: &PatternInput{Name: "kpi-3up"}},
		{Pattern: &PatternInput{Name: "card-grid"}},
		{Pattern: &PatternInput{Name: "timeline-horizontal"}},
		{SlideType: "title"},
	}

	result := analyzeDeckRhythm(slides)

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

func TestAnalyzeDeckRhythm_AccentBalance(t *testing.T) {
	accent1Fill, _ := json.Marshal("accent1")
	accent2Fill, _ := json.Marshal("accent2")

	slides := []SlideInput{
		{ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: accent1Fill}},
			}}},
		}},
		{ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: accent2Fill}},
			}}},
		}},
	}

	result := analyzeDeckRhythm(slides)

	if len(result.Aggregates.AccentBalance) != 2 {
		t.Errorf("expected 2 accents in balance, got %d", len(result.Aggregates.AccentBalance))
	}

	for accent, frac := range result.Aggregates.AccentBalance {
		if frac != 0.5 {
			t.Errorf("accent %s: expected 0.5 fraction, got %f", accent, frac)
		}
	}
}

func TestAnalyzeDeckRhythm_DensityClasses(t *testing.T) {
	slides := []SlideInput{
		// Low density: single text content.
		{Content: []ContentInput{{Type: "text"}}},
		// High density: table + chart + bullets.
		{Content: []ContentInput{{Type: "table"}, {Type: "chart"}, {Type: "bullets"}}},
	}

	result := analyzeDeckRhythm(slides)

	if result.PerSlide[0].DensityClass != "low" {
		t.Errorf("slide 0: expected density_class=low, got %q", result.PerSlide[0].DensityClass)
	}
	if result.PerSlide[1].DensityClass != "high" {
		t.Errorf("slide 1: expected density_class=high, got %q", result.PerSlide[1].DensityClass)
	}

	// With mixed density, CV should be > 0.
	if result.Aggregates.DensityCV == 0 {
		t.Error("expected non-zero density_cv for slides with different densities")
	}
}

func TestAnalyzeDeckRhythm_EmptySlides(t *testing.T) {
	slides := []SlideInput{{}, {}}

	result := analyzeDeckRhythm(slides)

	if len(result.PerSlide) != 2 {
		t.Fatalf("expected 2 per_slide entries, got %d", len(result.PerSlide))
	}

	// Both should default to "content" pattern and "text" dominant visual.
	for i, s := range result.PerSlide {
		if s.Pattern != "content" {
			t.Errorf("slide %d: expected pattern=content, got %q", i, s.Pattern)
		}
	}
}

func TestAnalyzeDeckRhythm_RecommendationIndices(t *testing.T) {
	// 10 identical slides — should get recommendations at indices 2, 5, 8.
	slides := make([]SlideInput, 10)
	for i := range slides {
		slides[i] = SlideInput{Pattern: &PatternInput{Name: "card-grid"}}
	}

	result := analyzeDeckRhythm(slides)

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
