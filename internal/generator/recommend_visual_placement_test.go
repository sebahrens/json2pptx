package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestEnrichVisualPlacement_DiagramNativeOOXML(t *testing.T) {
	// SWOT is native_ooxml in placeholder, SVG in grid cell.
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryDiagram, Name: "swot", Score: 0.95},
		},
	}
	EnrichVisualPlacement(result)

	p := result.Candidates[0].Placement
	if p == nil {
		t.Fatal("expected placement guidance for swot diagram")
	}
	if p.PreferredPlacement != "placeholder" {
		t.Errorf("swot preferred_placement: got %q, want %q", p.PreferredPlacement, "placeholder")
	}
	if p.HostStrategy != "placeholder_content" {
		t.Errorf("swot host_strategy: got %q, want %q", p.HostStrategy, "placeholder_content")
	}
	if !p.GridEmbeddable {
		t.Error("swot should be grid-embeddable (SVG fallback)")
	}
	if p.RenderPipeline != "native_ooxml" {
		t.Errorf("swot render_pipeline: got %q, want %q", p.RenderPipeline, "native_ooxml")
	}
}

func TestEnrichVisualPlacement_DiagramSVGOnly(t *testing.T) {
	// Timeline is SVG-only in both contexts.
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryDiagram, Name: "timeline", Score: 0.90},
		},
	}
	EnrichVisualPlacement(result)

	p := result.Candidates[0].Placement
	if p == nil {
		t.Fatal("expected placement guidance for timeline diagram")
	}
	if p.PreferredPlacement != "either" {
		t.Errorf("timeline preferred_placement: got %q, want %q", p.PreferredPlacement, "either")
	}
	if !p.GridEmbeddable {
		t.Error("timeline should be grid-embeddable")
	}
	if p.RenderPipeline != "svg" {
		t.Errorf("timeline render_pipeline: got %q, want %q", p.RenderPipeline, "svg")
	}
}

func TestEnrichVisualPlacement_Chart(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryChart, Name: "line", Score: 0.90},
		},
	}
	EnrichVisualPlacement(result)

	p := result.Candidates[0].Placement
	if p == nil {
		t.Fatal("expected placement guidance for chart")
	}
	if p.PreferredPlacement != "either" {
		t.Errorf("chart preferred_placement: got %q, want %q", p.PreferredPlacement, "either")
	}
	if !p.GridEmbeddable {
		t.Error("chart should be grid-embeddable")
	}
	if p.RenderPipeline != "svg" {
		t.Errorf("chart render_pipeline: got %q, want %q", p.RenderPipeline, "svg")
	}
}

func TestEnrichVisualPlacement_Pattern(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryPattern, Name: "kpi-3up", Score: 0.88},
		},
	}
	EnrichVisualPlacement(result)

	p := result.Candidates[0].Placement
	if p == nil {
		t.Fatal("expected placement guidance for pattern")
	}
	if p.HostStrategy != "pattern_expansion" {
		t.Errorf("pattern host_strategy: got %q, want %q", p.HostStrategy, "pattern_expansion")
	}
	if p.GridEmbeddable {
		t.Error("pattern should not be grid-embeddable (it IS the grid)")
	}
	if p.RenderPipeline != "native_ooxml" {
		t.Errorf("pattern render_pipeline: got %q, want %q", p.RenderPipeline, "native_ooxml")
	}
	// Patterns compose with charts and diagrams.
	if len(p.ComposableWith) == 0 {
		t.Error("pattern should have composable_with entries")
	}
}

func TestEnrichVisualPlacement_Placeholder(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryPlaceholder, Name: "title", Score: 0.95},
		},
	}
	EnrichVisualPlacement(result)

	p := result.Candidates[0].Placement
	if p == nil {
		t.Fatal("expected placement guidance for placeholder")
	}
	if p.HostStrategy != "standalone_slide" {
		t.Errorf("placeholder host_strategy: got %q, want %q", p.HostStrategy, "standalone_slide")
	}
	if p.GridEmbeddable {
		t.Error("placeholder layout should not be grid-embeddable")
	}
	if p.RenderPipeline != "template_driven" {
		t.Errorf("placeholder render_pipeline: got %q, want %q", p.RenderPipeline, "template_driven")
	}
}

func TestEnrichVisualPlacement_HybridCase(t *testing.T) {
	// A result with both a pattern and a chart — the hybrid case where the agent
	// should understand that the pattern can host the chart in a composed layout.
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryPattern, Name: "kpi-3up", Score: 0.90},
			{Category: patterns.VisualCategoryChart, Name: "line", Score: 0.85},
		},
	}
	EnrichVisualPlacement(result)

	patternP := result.Candidates[0].Placement
	chartP := result.Candidates[1].Placement
	if patternP == nil || chartP == nil {
		t.Fatal("expected placement guidance for both candidates")
	}

	// The pattern uses pattern_expansion and the chart is grid-embeddable,
	// meaning the agent can compose them on one slide.
	if patternP.HostStrategy != "pattern_expansion" {
		t.Errorf("pattern host_strategy: got %q, want %q", patternP.HostStrategy, "pattern_expansion")
	}
	if !chartP.GridEmbeddable {
		t.Error("chart should be grid-embeddable for hybrid composition")
	}
	// Pattern declares it composes with charts.
	found := false
	for _, c := range patternP.ComposableWith {
		if c == "chart" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pattern should declare composability with 'chart'")
	}
}
