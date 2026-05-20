package svggen

import (
	"strings"
	"testing"
)

// renderSingleSeriesBar renders a 1-series bar chart with the given chart_style
// overrides applied, returning the SVG content as a string.
func renderSingleSeriesBar(t *testing.T, overrides *ChartStyleOverrides) string {
	t.Helper()
	req := &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Test",
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3", "Q4"},
			"series": []any{
				map[string]any{"name": "Revenue", "values": []any{10.0, 20.0, 15.0, 25.0}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600, Format: "svg"},
		Style:  StyleSpec{ChartStyle: overrides},
	}
	out, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out == nil || out.SVG == nil {
		t.Fatal("nil SVG")
	}
	return string(out.SVG.Content)
}

// TestChartStyleOverrides_SingleSeriesLegend_Default asserts the default
// (no chart_style override) suppresses the legend on a 1-series chart, per
// internal/tokens.ChartLegendMinSeries.
func TestChartStyleOverrides_SingleSeriesLegend_Default(t *testing.T) {
	svg := renderSingleSeriesBar(t, nil)
	// The legend draws the series name as a <tspan> element. With a
	// single series and no override the legend is suppressed entirely,
	// so the series name does not appear in the SVG at all.
	if strings.Contains(svg, "Revenue") {
		t.Errorf("series name leaked into SVG without legend override (legend should be suppressed for single series)")
	}
}

// TestChartStyleOverrides_SingleSeriesLegend_ForcedOn asserts that
// show_single_series_legend=true makes the legend render even with one series.
func TestChartStyleOverrides_SingleSeriesLegend_ForcedOn(t *testing.T) {
	svg := renderSingleSeriesBar(t, &ChartStyleOverrides{
		ShowSingleSeriesLegend: boolPtr(true),
	})
	// With the override on the legend renders, which paints the series
	// name as a <tspan>. Default-off baseline above shows zero
	// occurrences, so even one indicates the legend was drawn.
	if !strings.Contains(svg, "Revenue") {
		t.Error("show_single_series_legend=true should render the legend with the series name")
	}
}

// gridStrokeCount counts the number of subtle grid paths (stroke=#e0e0e0,
// the DefaultGridConfig color) in an SVG. Each grid path adds one stroke
// match; horizontal vs vertical aren't distinguished here, but the
// difference between two renders with/without the vertical-gridlines
// override isolates the per-category vertical lines added by
// DrawCategoricalVerticalGrid.
func gridStrokeCount(svg string) int {
	return strings.Count(svg, "stroke:#e0e0e0")
}

// TestChartStyleOverrides_VerticalGridlines_Default asserts the default
// (no chart_style override) does NOT draw vertical gridlines on a bar chart,
// per internal/tokens.ChartHideVerticalGridlines. The default renders only
// horizontal gridlines from yScale ticks; the override-on render adds one
// vertical line per category.
func TestChartStyleOverrides_VerticalGridlines_Default(t *testing.T) {
	defaultSVG := renderSingleSeriesBar(t, nil)
	overrideSVG := renderSingleSeriesBar(t, &ChartStyleOverrides{
		ShowVerticalGridlines: boolPtr(true),
	})

	defaultGridLines := gridStrokeCount(defaultSVG)
	overrideGridLines := gridStrokeCount(overrideSVG)

	// 4 categories → DrawCategoricalVerticalGrid emits 4 additional
	// vertical lines on top of the horizontal-only baseline.
	const wantAdded = 4
	if overrideGridLines-defaultGridLines != wantAdded {
		t.Errorf("expected %d additional vertical gridlines from override: default=%d override=%d (diff=%d)",
			wantAdded, defaultGridLines, overrideGridLines, overrideGridLines-defaultGridLines)
	}
}

// TestChartStyleOverrides_VerticalGridlines_ForcedOn asserts that
// show_vertical_gridlines=true causes additional grid strokes to be drawn
// at category positions on a bar chart.
func TestChartStyleOverrides_VerticalGridlines_ForcedOn(t *testing.T) {
	svg := renderSingleSeriesBar(t, &ChartStyleOverrides{
		ShowVerticalGridlines: boolPtr(true),
	})
	// With 4 categories, the override produces at least 4 additional
	// vertical lines on top of the horizontal grid. Any grid stroke count
	// above 6 (the horizontal-only baseline) indicates verticals were
	// added.
	if got := gridStrokeCount(svg); got < 7 {
		t.Errorf("expected ≥7 grid strokes when show_vertical_gridlines=true (6 horizontal + per-category verticals), got %d", got)
	}
}

// TestChartStyleOverrides_NilOverridesNoOp asserts that applyChartStyleOverrides
// with a nil overrides pointer leaves the chart-style fields untouched. This
// is the fast-path guarantee that lets every chart factory call the helper
// unconditionally.
func TestChartStyleOverrides_NilOverridesNoOp(t *testing.T) {
	cfg := DefaultChartConfig(800, 600)
	cfg.ShowVerticalGrid = true
	cfg.ForceLegendSingleSeries = true
	applyChartStyleOverrides(&cfg, nil)
	if !cfg.ShowVerticalGrid {
		t.Error("ShowVerticalGrid should remain true when overrides is nil")
	}
	if !cfg.ForceLegendSingleSeries {
		t.Error("ForceLegendSingleSeries should remain true when overrides is nil")
	}
}

// TestChartStyleOverrides_PartialOverride asserts that only the fields set on
// ChartStyleOverrides are copied — nil pointers leave the existing config
// field alone.
func TestChartStyleOverrides_PartialOverride(t *testing.T) {
	cfg := DefaultChartConfig(800, 600)
	cfg.ShowVerticalGrid = true
	cfg.ForceLegendSingleSeries = true

	// Override only ShowSingleSeriesLegend=false; ShowVerticalGrid should
	// remain true because the override is nil.
	overrides := &ChartStyleOverrides{
		ShowSingleSeriesLegend: boolPtr(false),
	}
	applyChartStyleOverrides(&cfg, overrides)

	if !cfg.ShowVerticalGrid {
		t.Error("ShowVerticalGrid should remain true (override was nil)")
	}
	if cfg.ForceLegendSingleSeries {
		t.Error("ForceLegendSingleSeries should be false (override set to false)")
	}
}
