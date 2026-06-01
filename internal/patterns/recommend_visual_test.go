package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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

// TestRecommendVisual_OrderedStepsRouting verifies J2P-RECO-006 at the visual
// layer: an ordered-steps intent ranks the numbered-step-strip pattern above the
// process_flow diagram, while a branching/cross-functional intent still lets the
// process_flow diagram surface strongly.
func TestRecommendVisual_OrderedStepsRouting(t *testing.T) {
	reg := Default()

	scoreOf := func(cs []VisualCandidate, name string) (float64, bool) {
		for _, c := range cs {
			if c.Name == name {
				return c.Score, true
			}
		}
		return 0, false
	}

	t.Run("ordered-steps demotes process_flow diagram", func(t *testing.T) {
		res := RecommendVisual(reg, "four step selection process for AI tool criteria", nil, 10)
		if len(res.Candidates) == 0 {
			t.Fatal("no candidates")
		}
		nss, ok := scoreOf(res.Candidates, "numbered-step-strip")
		if !ok {
			t.Fatalf("expected numbered-step-strip candidate, got %+v", res.Candidates)
		}
		if pf, ok := scoreOf(res.Candidates, "process_flow"); ok && pf >= nss {
			t.Errorf("process_flow diagram (%.2f) should rank below numbered-step-strip (%.2f)", pf, nss)
		}
	})

	t.Run("branching intent keeps process_flow diagram strong", func(t *testing.T) {
		res := RecommendVisual(reg, "cross-functional workflow with handoffs and decision branches", nil, 10)
		if pf, ok := scoreOf(res.Candidates, "process_flow"); ok && pf < 0.85 {
			t.Errorf("process_flow diagram demoted to %.2f despite explicit branching terms", pf)
		}
	})
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

func TestRecommendVisual_DensityHintBoostsMatchingPatterns(t *testing.T) {
	// Register two KPI patterns: kpi-3up (low density) and kpi-4up (medium density)
	reg := NewRegistry()
	reg.Register(&densityStubPattern{name: "kpi-3up", density: "low"})
	reg.Register(&densityStubPattern{name: "kpi-4up", density: "medium"})

	// With density_hint="low", kpi-3up should score higher than kpi-4up
	hints := &VisualHints{
		ContentHints: ContentHints{
			HasMetrics:  true,
			DensityHint: "low",
		},
	}
	result := RecommendVisual(reg, "show kpi metrics", hints, 5)

	// Find both candidates
	var kpi3Score, kpi4Score float64
	for _, c := range result.Candidates {
		switch c.Name {
		case "kpi-3up":
			kpi3Score = c.Score
		case "kpi-4up":
			kpi4Score = c.Score
		}
	}
	if kpi3Score == 0 {
		t.Fatal("expected kpi-3up in candidates")
	}
	if kpi3Score <= kpi4Score {
		t.Errorf("with density_hint=low, kpi-3up (low, score=%.2f) should outscore kpi-4up (medium, score=%.2f)", kpi3Score, kpi4Score)
	}
}

func TestRecommendVisual_DensityHintPenalizesDistantPatterns(t *testing.T) {
	// With density_hint="high", low-density patterns should be penalized more than medium
	reg := NewRegistry()
	reg.Register(&densityStubPattern{name: "stat-hero", density: "low"})
	reg.Register(&densityStubPattern{name: "card-grid", density: "medium"})
	reg.Register(&densityStubPattern{name: "bmc-canvas", density: "high"})

	// Use a "card" intent that matches both card-grid and bmc-canvas
	hints := &VisualHints{
		ContentHints: ContentHints{DensityHint: "high"},
	}
	result := RecommendVisual(reg, "show cards overview categories", hints, 5)

	var cardScore, bmcScore float64
	for _, c := range result.Candidates {
		switch c.Name {
		case "card-grid":
			cardScore = c.Score
		case "bmc-canvas":
			bmcScore = c.Score
		}
	}

	// bmc-canvas (high) should be boosted, card-grid (medium) penalized mildly
	if bmcScore > 0 && cardScore > 0 && bmcScore <= cardScore {
		t.Errorf("with density_hint=high, bmc-canvas (high, %.2f) should outscore card-grid (medium, %.2f)", bmcScore, cardScore)
	}
}

// densityStubPattern is a stub pattern with configurable DensityClass for testing.
type densityStubPattern struct {
	name    string
	density string
}

func (d *densityStubPattern) Name() string        { return d.name }
func (d *densityStubPattern) Description() string { return d.name }
func (d *densityStubPattern) UseWhen() string     { return d.name }
func (d *densityStubPattern) NotWhen() string     { return "" }
func (d *densityStubPattern) Version() int        { return 1 }
func (d *densityStubPattern) NewValues() any      { return nil }
func (d *densityStubPattern) NewOverrides() any   { return nil }
func (d *densityStubPattern) NewCellOverride() any { return nil }
func (d *densityStubPattern) Schema() *Schema     { return nil }
func (d *densityStubPattern) CellsHint() string   { return "" }
func (d *densityStubPattern) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{Category: "data-display", DensityClass: d.density}
}
func (d *densityStubPattern) Validate(_, _ any, _ map[int]any) error { return nil }
func (d *densityStubPattern) Expand(_ ExpandContext, _, _ any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	return nil, nil
}

// newTestRegistry creates a registry with optional stub patterns.
func newTestRegistry(names ...string) *Registry {
	reg := NewRegistry()
	for _, name := range names {
		reg.Register(&stubPattern{name: name, desc: name, useWhen: name, version: 1})
	}
	return reg
}

// TestRecommendVisual_Candidates_RanksMixedCategories verifies that an
// explicit candidates list spanning pattern + chart + diagram + placeholder
// categories is fully returned (every name, every category resolved), with
// rationale and confidence_band populated on each.
func TestRecommendVisual_Candidates_RanksMixedCategories(t *testing.T) {
	reg := newTestRegistry("kpi-3up", "matrix-2x2")

	opts := &RecommendOptions{
		Candidates: []string{
			"kpi-3up",            // named_pattern
			"bar",                // chart
			"pyramid",            // diagram
			"title",              // placeholder_layout
			"made-up-thing",      // unknown
		},
	}
	result := RecommendVisual(reg, "compare top KPIs", &VisualHints{
		ContentHints: ContentHints{ItemCount: 3, HasMetrics: true},
	}, 5, opts)

	if len(result.Candidates) != 5 {
		t.Fatalf("expected 5 candidates (one per shortlist name), got %d: %+v",
			len(result.Candidates), result.Candidates)
	}

	byName := make(map[string]VisualCandidate)
	for _, c := range result.Candidates {
		byName[c.Name] = c
	}

	wantCategory := map[string]VisualCategory{
		"kpi-3up":       VisualCategoryPattern,
		"bar":           VisualCategoryChart,
		"pyramid":       VisualCategoryDiagram,
		"title":         VisualCategoryPlaceholder,
		"made-up-thing": VisualCategoryShapeGrid,
	}
	for name, wantCat := range wantCategory {
		c, ok := byName[name]
		if !ok {
			t.Errorf("candidate %q missing from result", name)
			continue
		}
		if c.Category != wantCat {
			t.Errorf("candidate %q: got category %q, want %q", name, c.Category, wantCat)
		}
		if c.Rationale == "" {
			t.Errorf("candidate %q: empty rationale", name)
		}
		if c.ConfidenceBand == "" {
			t.Errorf("candidate %q: empty confidence_band", name)
		}
	}

	// Unknown should have score 0.
	if byName["made-up-thing"].Score != 0 {
		t.Errorf("unknown candidate score: got %v, want 0", byName["made-up-thing"].Score)
	}
}

// TestRecommendVisual_Candidates_NoThresholdCutoff confirms that low-scoring
// candidates are NOT filtered out in candidates mode (the threshold cutoff
// that normally drops sub-0.5 entries is bypassed).
func TestRecommendVisual_Candidates_NoThresholdCutoff(t *testing.T) {
	reg := newTestRegistry("kpi-3up")

	// "agenda" intent shouldn't match the chart keyword set well.
	opts := &RecommendOptions{Candidates: []string{"line", "bar", "scatter"}}
	result := RecommendVisual(reg, "agenda slide", nil, 5, opts)

	if len(result.Candidates) != 3 {
		t.Fatalf("expected 3 candidates (all chart names returned), got %d: %+v",
			len(result.Candidates), result.Candidates)
	}
	for _, c := range result.Candidates {
		if c.Rationale == "" {
			t.Errorf("candidate %q: empty rationale", c.Name)
		}
		if c.ConfidenceBand == "" {
			t.Errorf("candidate %q: empty confidence_band", c.Name)
		}
	}
}

// TestRecommendVisual_ComposeOnExplicitIntent verifies that an explicit
// "side by side" / "compose" intent surfaces a Category=="compose" candidate
// with placement.composable_with populated.
func TestRecommendVisual_ComposeOnExplicitIntent(t *testing.T) {
	reg := Default()
	result := RecommendVisual(reg, "panels and quote side by side on one slide", nil, 8)

	var compose *VisualCandidate
	for i := range result.Candidates {
		if result.Candidates[i].Category == VisualCategoryCompose {
			compose = &result.Candidates[i]
			break
		}
	}
	if compose == nil {
		t.Fatalf("expected a compose candidate; got %+v", result.Candidates)
	}
	if !strings.HasPrefix(compose.Name, "compose:") {
		t.Errorf("compose candidate name should be prefixed with \"compose:\"; got %q", compose.Name)
	}
	if compose.Placement == nil {
		t.Fatal("compose candidate placement is nil")
	}
	if len(compose.Placement.ComposableWith) < 2 {
		t.Errorf("compose placement.composable_with should list both sibling patterns; got %v", compose.Placement.ComposableWith)
	}
	if compose.Placement.HostStrategy != "pattern_expansion" {
		t.Errorf("compose host_strategy: got %q, want %q", compose.Placement.HostStrategy, "pattern_expansion")
	}
}

// TestRecommendVisual_ComposeNameSorted asserts that the compose candidate
// name is order-independent — the same pair always produces the same name so
// downstream dedupe works.
func TestRecommendVisual_ComposeNameSorted(t *testing.T) {
	got1 := composeNameFor("stylish-panels", "pull-quote")
	got2 := composeNameFor("pull-quote", "stylish-panels")
	if got1 != got2 {
		t.Errorf("composeNameFor should be order-independent: %q vs %q", got1, got2)
	}
	if !strings.Contains(got1, "pull-quote") || !strings.Contains(got1, "stylish-panels") {
		t.Errorf("composeNameFor result missing one of the names: %q", got1)
	}
}
