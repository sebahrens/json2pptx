package svggen

import (
	"fmt"
	"testing"
)

// TestUseDirectLabels_Window asserts the decision function picks the direct-
// label path only when the series count is inside the executive window and
// PreferDirectLabels is set. The window boundaries are the load-bearing
// contract: if either changes silently, charts will render with the wrong
// labeling strategy and the parity test in internal/tokens will fail.
func TestUseDirectLabels_Window(t *testing.T) {
	cases := []struct {
		name        string
		prefer      bool
		seriesCount int
		want        bool
	}{
		{"prefer_off_in_window", false, 3, false},
		{"prefer_on_below_window", true, 1, false},
		{"prefer_on_min_window", true, MinLegendSeriesCount, true},
		{"prefer_on_max_window", true, MaxDirectLabelSeriesCount, true},
		{"prefer_on_above_window", true, MaxDirectLabelSeriesCount + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultChartConfig(800, 600)
			cfg.PreferDirectLabels = tc.prefer
			if got := useDirectLabels(cfg, tc.seriesCount); got != tc.want {
				t.Errorf("useDirectLabels(prefer=%t, n=%d) = %t, want %t",
					tc.prefer, tc.seriesCount, got, tc.want)
			}
		})
	}
}

// TestCartesianLayout_SkipsLegendReservation asserts that the shared layout
// helper does not reserve legend height when direct labels will be drawn.
// Without this, the plot area would shrink unnecessarily and direct labels
// would have less vertical room than expected.
func TestCartesianLayout_SkipsLegendReservation(t *testing.T) {
	style := DefaultStyleGuide()

	baseCfg := DefaultChartConfig(800, 600)
	baseCfg.PreferDirectLabels = false

	directCfg := DefaultChartConfig(800, 600)
	directCfg.PreferDirectLabels = true

	withLegend := ComputeCartesianLayout(baseCfg, style, "Title", "", "", 3)
	withDirect := ComputeCartesianLayout(directCfg, style, "Title", "", "", 3)

	if withLegend.LegendHeight == 0 {
		t.Fatal("baseline expectation broken: legend height should be reserved when PreferDirectLabels is false")
	}
	if withDirect.LegendHeight != 0 {
		t.Errorf("LegendHeight = %.2f, want 0 (direct-label path should not reserve legend space)", withDirect.LegendHeight)
	}
	if withDirect.PlotArea.H <= withLegend.PlotArea.H {
		t.Errorf("PlotArea.H = %.2f, want > %.2f (direct labels should free up the legend strip)", withDirect.PlotArea.H, withLegend.PlotArea.H)
	}

	// At 5 series we're above the direct-label window, so the legend must
	// reappear even when PreferDirectLabels is true. This guards the upper
	// boundary of the window: silent off-by-one drift would suppress the
	// legend for many-series charts where inline labels start to collide.
	directBeyond := ComputeCartesianLayout(directCfg, style, "Title", "", "", MaxDirectLabelSeriesCount+1)
	if directBeyond.LegendHeight == 0 {
		t.Errorf("LegendHeight = 0 for %d series; legend must reappear above the direct-label window", MaxDirectLabelSeriesCount+1)
	}
}

// TestDirectLabels_LegendDiffersAcrossWindow renders the same chart twice —
// once with PreferDirectLabels and once with an explicit ShowLegend=true
// override — and asserts the two SVGs differ in size. When the explicit
// override forces the legend back on, the rendered SVG must include extra
// legend markup that direct-label output omits. This is the integration
// guard: if a future refactor silently flips the default behaviour, both
// renders would collapse to the same output and this test would fail.
func TestDirectLabels_LegendDiffersAcrossWindow(t *testing.T) {
	chartTypes := []string{"bar_chart", "line_chart", "area_chart"}
	for _, ct := range chartTypes {
		t.Run(ct+"_in_window", func(t *testing.T) {
			direct := renderChartSVG(t, ct, 3, false)
			forced := renderChartSVG(t, ct, 3, true)
			if len(forced) <= len(direct) {
				t.Errorf("%s: explicit ShowLegend=true rendered len=%d, direct-label rendered len=%d (forced should be larger because legend markup is added)",
					ct, len(forced), len(direct))
			}
		})
	}
}

// renderChartSVG is a test helper that renders a chart with N series and
// returns the SVG body so the caller can measure relative sizes. The
// series names are unique-by-letter so a substring match is unambiguous.
func renderChartSVG(t *testing.T, chartType string, seriesCount int, forceLegend bool) string {
	t.Helper()

	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	series := make([]map[string]any, seriesCount)
	for i := 0; i < seriesCount; i++ {
		values := make([]float64, len(categories))
		for j := range values {
			values[j] = float64(10 + i*2 + j)
		}
		series[i] = map[string]any{
			"name":   directLabelTestSeriesName(i),
			"values": values,
		}
	}
	req := &RequestEnvelope{
		Type:   chartType,
		Title:  "direct-label parity",
		Data:   map[string]any{"categories": categories, "series": series},
		Output: OutputSpec{Format: "svg", Width: 1600, Height: 900},
	}
	if forceLegend {
		req.Style.ShowLegend = true
	}
	out, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render %s svg: %v", chartType, err)
	}
	if out == nil || out.SVG == nil {
		t.Fatalf("render %s svg: no SVG document", chartType)
	}
	return string(out.SVG.Content)
}

// directLabelTestSeriesName produces a stable, substring-unique series label
// indexed by an ASCII letter so tests can grep the SVG without colliding on
// repeated digits ("Series 1" vs "Series 10").
func directLabelTestSeriesName(i int) string {
	return fmt.Sprintf("DLSeries%c", 'A'+i)
}
