package patterns

import (
	"strings"
	"testing"
)

func TestRecommendVisual_ChartForTrend(t *testing.T) {
	reg := newTestRegistry("kpi-3up", "card-grid", "timeline-horizontal")
	result := RecommendVisual(reg, "show our Q3 revenue trend", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	// First result should be a chart (line).
	first := result.Candidates[0]
	if first.Category != VisualCategoryChart {
		t.Errorf("expected first candidate to be chart, got %s (%s)", first.Category, first.Name)
	}
	if first.Name != "line" {
		t.Errorf("expected line chart for trend intent, got %s", first.Name)
	}
}

func TestRecommendVisual_MatrixForVendorComparison(t *testing.T) {
	reg := newTestRegistry("matrix-2x2", "comparison-2col", "card-grid")
	result := RecommendVisual(reg, "compare 3 vendors on 5 dimensions", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	// Should have matrix or comparison in candidates.
	found := false
	for _, c := range result.Candidates {
		if c.Name == "matrix-2x2" || c.Name == "comparison-2col" || c.Name == "radar" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected matrix-2x2, comparison-2col, or radar in candidates; got %+v", result.Candidates)
	}
}

func TestRecommendVisual_PlaceholderForTitleSlide(t *testing.T) {
	reg := newTestRegistry()
	result := RecommendVisual(reg, "create a title slide for the presentation", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	first := result.Candidates[0]
	if first.Category != VisualCategoryPlaceholder {
		t.Errorf("expected placeholder_layout for title slide, got %s (%s)", first.Category, first.Name)
	}
	if first.Name != "title" {
		t.Errorf("expected title slide type, got %s", first.Name)
	}
}

func TestRecommendVisual_FallbackToShapeGrid(t *testing.T) {
	reg := newTestRegistry()
	result := RecommendVisual(reg, "wjklmn plugh qrstfv", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected fallback candidate")
	}
	if result.Candidates[0].Category != VisualCategoryShapeGrid {
		t.Errorf("expected raw_shape_grid fallback, got %s (%s)", result.Candidates[0].Category, result.Candidates[0].Name)
	}
}

func TestRecommendVisual_DiagramForSWOT(t *testing.T) {
	reg := newTestRegistry()
	result := RecommendVisual(reg, "SWOT analysis of our competitive position", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	found := false
	for _, c := range result.Candidates {
		if c.Category == VisualCategoryDiagram && c.Name == "swot" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected swot diagram in candidates; got %+v", result.Candidates)
	}
}

func TestRecommendVisual_AgendaReachable(t *testing.T) {
	reg := newTestRegistry("agenda")
	result := RecommendVisual(reg, "agenda slide showing the deck sections", nil, 5)

	if len(result.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	found := false
	for _, c := range result.Candidates {
		if c.Name == "agenda" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected agenda pattern in candidates; got %+v", result.Candidates)
	}
}

func TestRecommendVisual_AllRegisteredPatternsReachable(t *testing.T) {
	reg := Default()
	allPatterns := reg.List()

	// Collect all pattern names from the recommendation rules.
	reachable := make(map[string]bool)
	for _, r := range rules {
		reachable[r.pattern] = true
	}

	for _, p := range allPatterns {
		name := p.Name()
		if !reachable[name] {
			t.Errorf("registered pattern %q has no recommendation rules — it is unreachable via recommend_pattern/recommend_visual", name)
		}
	}
}

func TestRecommendVisual_QueryUnderstood(t *testing.T) {
	reg := newTestRegistry()
	result := RecommendVisual(reg, "show revenue trend", &VisualHints{
		ContentHints: ContentHints{ItemCount: 5},
		DataPoints:   12,
		SeriesCount:  2,
		Audience:     "executive",
	}, 5)

	qu := result.QueryUnderstood
	if qu == "" {
		t.Error("QueryUnderstood should not be empty")
	}
	if !strings.Contains(qu, "data_points=12") {
		t.Errorf("QueryUnderstood should contain data_points=12, got %q", qu)
	}
	if !strings.Contains(qu, "series_count=2") {
		t.Errorf("QueryUnderstood should contain series_count=2, got %q", qu)
	}
}

// newTestRegistry creates a registry with optional stub patterns.
func newTestRegistry(names ...string) *Registry {
	reg := NewRegistry()
	for _, name := range names {
		reg.Register(&stubPattern{name: name, desc: name, useWhen: name, version: 1})
	}
	return reg
}
