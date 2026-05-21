package tokens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestChartStyleDefaults_Parity_Gridlines asserts that svggen's chart-grid
// default still matches the executive token contract. If svggen flips
// DefaultGridConfig.ShowVertical back to true, this test catches the drift
// before a deck ever ships with PowerPoint-looking vertical gridlines.
//
// The constant is the canonical source; svggen is required to match.
func TestChartStyleDefaults_Parity_Gridlines(t *testing.T) {
	cfg := svggen.DefaultGridConfig()
	wantVertical := !ChartHideVerticalGridlines
	if cfg.ShowVertical != wantVertical {
		t.Errorf("svggen.DefaultGridConfig().ShowVertical = %t, want %t (per tokens.ChartHideVerticalGridlines = %t)",
			cfg.ShowVertical, wantVertical, ChartHideVerticalGridlines)
	}
	// Horizontal gridlines must stay on — the y-axis carries the comparison.
	if !cfg.ShowHorizontal {
		t.Error("svggen.DefaultGridConfig().ShowHorizontal = false, want true (horizontal gridlines remain on by executive default)")
	}
}

// TestNumericColumnType locks the set of column-type strings that are
// considered numeric for header / cell right-alignment. Drift here would mean
// a "delta" column suddenly stops right-aligning, or a "date" column starts.
func TestNumericColumnType(t *testing.T) {
	numeric := []string{"number", "currency", "percent", "delta"}
	for _, c := range numeric {
		if !NumericColumnType(c) {
			t.Errorf("NumericColumnType(%q) = false, want true", c)
		}
	}
	nonNumeric := []string{"", "text", "date", "string", "Number" /* case-sensitive */}
	for _, c := range nonNumeric {
		if NumericColumnType(c) {
			t.Errorf("NumericColumnType(%q) = true, want false", c)
		}
	}
}

// TestChartStyleDefaults_Parity_TabularNums asserts that svggen emits the
// tabular-nums CSS rule on every chart SVG when ChartTickLabelTabularNums is
// true. Without this, executive decks would render columns of numbers with
// proportional digits and the tens-column drift would be visible to readers.
func TestChartStyleDefaults_Parity_TabularNums(t *testing.T) {
	if !ChartTickLabelTabularNums {
		t.Skip("ChartTickLabelTabularNums is false; renderer is not expected to emit the rule")
	}
	req := &svggen.RequestEnvelope{
		Type:  "bar_chart",
		Title: "parity",
		Data: map[string]any{
			"categories": []string{"A", "B"},
			"series": []map[string]any{
				{"name": "S1", "values": []float64{1, 2}},
			},
		},
		Output: svggen.OutputSpec{Format: "svg", Width: 400, Height: 300},
	}
	out, err := svggen.RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render svg: %v", err)
	}
	if out == nil || out.SVG == nil {
		t.Fatal("svggen returned no SVG document")
	}
	got := string(out.SVG.Content)
	const want = "font-variant-numeric:tabular-nums"
	if !strings.Contains(got, want) {
		t.Errorf("chart SVG missing %q rule (per tokens.ChartTickLabelTabularNums)", want)
	}
}

// TestChartStyleDefaults_Parity_DirectLabelThreshold asserts svggen exposes
// the same direct-label threshold as the token. Drift here would mean charts
// silently switch back to legends at the wrong series count, breaking the
// executive default.
func TestChartStyleDefaults_Parity_DirectLabelThreshold(t *testing.T) {
	if svggen.MinLegendSeriesCount != ChartLegendMinSeries {
		t.Errorf("svggen.MinLegendSeriesCount = %d, want %d (per tokens.ChartLegendMinSeries)",
			svggen.MinLegendSeriesCount, ChartLegendMinSeries)
	}
	if svggen.MaxDirectLabelSeriesCount != ChartDirectLabelMaxSeries {
		t.Errorf("svggen.MaxDirectLabelSeriesCount = %d, want %d (per tokens.ChartDirectLabelMaxSeries)",
			svggen.MaxDirectLabelSeriesCount, ChartDirectLabelMaxSeries)
	}
}

// TestChartStyleDefaults_Parity_DirectLabelsRendered asserts that bar and
// line charts emitted by svggen suppress the legend and draw inline series
// labels when the series count falls in the executive direct-label window
// (2-4 series by default) and that the legend reappears above the window
// (5+ series). This is the end-to-end contract for the token threshold.
func TestChartStyleDefaults_Parity_DirectLabelsRendered(t *testing.T) {
	t.Helper()

	cases := []struct {
		name        string
		chartType   string
		seriesCount int
		wantLegend  bool
	}{
		{"bar_2series_direct", "bar_chart", 2, false},
		{"bar_4series_direct", "bar_chart", 4, false},
		{"bar_5series_legend", "bar_chart", 5, true},
		{"line_2series_direct", "line_chart", 2, false},
		{"line_4series_direct", "line_chart", 4, false},
		{"line_5series_legend", "line_chart", 5, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &svggen.RequestEnvelope{
				Type:   tc.chartType,
				Title:  "Series threshold parity",
				Data:   makeSeriesData(tc.seriesCount),
				Output: svggen.OutputSpec{Format: "svg", Width: 1600, Height: 900},
			}
			out, err := svggen.RenderMultiFormatWithFindings(req, "svg")
			if err != nil {
				t.Fatalf("render svg: %v", err)
			}
			if out == nil || out.SVG == nil {
				t.Fatal("svggen returned no SVG document")
			}
			svg := string(out.SVG.Content)
			labelHits := 0
			for i := 0; i < tc.seriesCount; i++ {
				if strings.Contains(svg, seriesNameFor(i)) {
					labelHits++
				}
			}
			if tc.wantLegend {
				// At/above the threshold every series name must appear in the
				// legend.
				if labelHits != tc.seriesCount {
					t.Errorf("%s: expected legend with %d series names, found %d", tc.name, tc.seriesCount, labelHits)
				}
			} else {
				// Below the threshold the legend is suppressed but the inline
				// labels still write each series name to the SVG.
				if labelHits != tc.seriesCount {
					t.Errorf("%s: expected inline labels for all %d series, found %d in SVG", tc.name, tc.seriesCount, labelHits)
				}
			}
		})
	}
}

// makeSeriesData builds a chart data payload with N named series sharing the
// same 4 categories. Numeric values stay small enough to keep the y-axis
// linear (no auto-log scale) so the rendered SVG is predictable across runs.
func makeSeriesData(n int) map[string]any {
	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	series := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		values := make([]float64, len(categories))
		for j := range values {
			values[j] = float64(10 + i*2 + j)
		}
		series[i] = map[string]any{
			"name":   seriesNameFor(i),
			"values": values,
		}
	}
	return map[string]any{
		"categories": categories,
		"series":     series,
	}
}

// seriesNameFor returns a stable, unique-by-index series label so tests can
// search the rendered SVG for series identifiers without false positives
// against shared substrings ("Series 1" vs "Series 10").
func seriesNameFor(i int) string {
	return fmt.Sprintf("DirectLabelSeries%c", 'A'+i)
}

// TestChartStyleDefaults_Sanity guards the contract values themselves so a
// future "drop legend when series >= 3" or "direct-label up to 8 series"
// change has to update both the constant and this test (and, by the
// commit-discipline policy, RULES.md if it surfaces these to agents).
func TestChartStyleDefaults_Sanity(t *testing.T) {
	if !ChartHideAreaBorder {
		t.Error("ChartHideAreaBorder = false; executive defaults drop the outer chart-area rectangle")
	}
	if !ChartHideVerticalGridlines {
		t.Error("ChartHideVerticalGridlines = false; executive defaults drop vertical gridlines")
	}
	if ChartLegendMinSeries < 2 {
		t.Errorf("ChartLegendMinSeries = %d; single-series charts must not show a legend by default", ChartLegendMinSeries)
	}
	if ChartDirectLabelMaxSeries < ChartLegendMinSeries {
		t.Errorf("ChartDirectLabelMaxSeries (%d) must be ≥ ChartLegendMinSeries (%d); otherwise the direct-label window is empty",
			ChartDirectLabelMaxSeries, ChartLegendMinSeries)
	}
	if !ChartTickLabelTabularNums {
		t.Error("ChartTickLabelTabularNums = false; numeric tick labels should request tabular figures")
	}
	if !TableNumericHeaderAlignRight {
		t.Error("TableNumericHeaderAlignRight = false; numeric headers should inherit the right-alignment of their data column")
	}
}
