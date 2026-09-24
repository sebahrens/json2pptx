package semantic

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// go-slide-creator-xg48. chart_insight puts the source into the
// chart-insights-split pattern's own values, which draws it under the chart.
// applyUniversalSlideFields set the slide-level source as well, so the
// attribution printed twice — once under the chart and once in the chrome
// source band.
func TestPatternRendersSource(t *testing.T) {
	withValues := func(v map[string]any) *deckinput.SlideInput {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return &deckinput.SlideInput{Pattern: &deckinput.PatternInput{Name: "chart-insights-split", Values: b}}
	}

	if !patternRendersSource(withValues(map[string]any{"source": "Company filings"})) {
		t.Error("a pattern carrying a source should be detected")
	}
	if patternRendersSource(withValues(map[string]any{"source": "   "})) {
		t.Error("a blank source is not a rendered source")
	}
	if patternRendersSource(withValues(map[string]any{"insights": []string{"a"}})) {
		t.Error("a pattern with no source key should not be detected")
	}
	if patternRendersSource(&deckinput.SlideInput{}) {
		t.Error("a slide with no pattern should not be detected")
	}
	if patternRendersSource(nil) {
		t.Error("nil must not panic or report true")
	}
}

// TestChartInsightSourceRenderedOnce compiles both shapes of a chart_insight
// slide — the pattern path and the >6-insight fallback — and asserts the source
// survives exactly once on each.
func TestChartInsightSourceRenderedOnce(t *testing.T) {
	const src = "Source: Company filings FY2026"
	chart := map[string]any{
		"type": "bar",
		"data": map[string]any{
			"categories": []any{"Q1", "Q2"},
			"series":     []any{map[string]any{"name": "Rev", "values": []any{1, 2}}},
		},
	}
	mk := func(n int) *DeckSpec {
		insights := make([]any, n)
		for i := range insights {
			insights[i] = "Insight about the quarter"
		}
		raw := map[string]any{
			"meta": map[string]any{"title": "T", "template": "midnight-blue"},
			"slides": []any{
				map[string]any{"kind": "title", "title": "T"},
				map[string]any{"kind": "chart_insight", "title": "Chart", "chart": chart, "insights": insights, "source": src},
			},
		}
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		spec, diags := Parse("spec.json", b)
		if diags.HasErrors() {
			t.Fatalf("%d insights: spec did not parse: %+v", n, diags)
		}
		return spec
	}

	for _, n := range []int{6, 8} {
		input, result, err := Compile(mk(n), CompileOptions{Strict: StrictnessOff})
		if err != nil {
			t.Fatalf("%d insights: compile: %v; diagnostics: %+v", n, err, result.Diagnostics)
		}
		slide := input.Slides[1]

		patternHas := patternRendersSource(&slide)
		slideHas := slide.Source != ""
		switch {
		case patternHas && slideHas:
			t.Errorf("%d insights: source is set on BOTH the pattern and the slide — it renders twice", n)
		case !patternHas && !slideHas:
			t.Errorf("%d insights: source was dropped entirely — an author who supplied an attribution believes they shipped one", n)
		}
	}
}
