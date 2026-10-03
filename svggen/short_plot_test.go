package svggen

import (
	"regexp"
	"strconv"
	"testing"
)

// shortChartRequest is a chart in a short, wide shape-grid cell: the 844x61pt
// region a line chart got above a stat-and-timeline band in main_top on modern
// (go-slide-creator-qpd9c). svggen draws it on a 844x100 canvas and scales the
// type to the 12pt floor at the placement, so every label is ~1.6x larger in
// canvas units than the cell's height suggests.
func shortChartRequest(chartType string, height int, series []any) *RequestEnvelope {
	req := &RequestEnvelope{
		Type: chartType,
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3", "Q4"},
			"series":     series,
		},
		Output: OutputSpec{Width: 844, Height: height},
	}
	req.Style.PlacementWidthPt = 844
	req.Style.PlacementHeightPt = float64(height) * 0.61
	req.Style.MinReadablePt = 12
	return req
}

var (
	revenueSeries = []any{map[string]any{"name": "Revenue", "values": []any{12.0, 14.0, 17.0, 21.0}}}
	mixSeries     = []any{
		map[string]any{"name": "Digital", "values": []any{20.0, 24.0, 27.0, 30.0}},
		map[string]any{"name": "Phone", "values": []any{15.0, 12.0, 11.0, 10.0}},
		map[string]any{"name": "Branch", "values": []any{10.0, 9.0, 8.0, 7.0}},
		map[string]any{"name": "Other", "values": []any{5.0, 5.0, 4.0, 4.0}},
	}
	svgTextRE = regexp.MustCompile(`<text[^>]* y="([-0-9.]+)"[^>]*font-size:([0-9.]+)px`)
)

func hasCollapsed(findings []Finding) *Finding {
	for i := range findings {
		if findings[i].Code == FindingPlotAreaCollapsed {
			return &findings[i]
		}
	}
	return nil
}

// A short chart's value labels stay on the canvas, and the plot squeezed
// under its fixed decoration is reported with the height ratio and the
// side-by-side alternative (go-slide-creator-qpd9c, go-slide-creator-9re9p).
func TestShortChartKeepsLabelsAndReportsCollapse(t *testing.T) {
	for _, tc := range []struct {
		chartType string
		series    []any
	}{
		{"line_chart", revenueSeries},
		{"bar_chart", revenueSeries},
		{"stacked_bar_chart", mixSeries},
	} {
		t.Run(tc.chartType, func(t *testing.T) {
			out, err := RenderMultiFormatWithFindings(shortChartRequest(tc.chartType, 100, tc.series), "svg")
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range svgTextRE.FindAllStringSubmatch(string(out.SVG.Content), -1) {
				y, _ := strconv.ParseFloat(m[1], 64)
				size, _ := strconv.ParseFloat(m[2], 64)
				// A baseline at y draws glyphs up to ~0.75em above it.
				if top := y - 0.75*size; top < -0.5 {
					t.Errorf("text at y=%.1f (font %.1f) reaches %.1f above the canvas: clipped", y, size, -top)
				}
			}
			f := hasCollapsed(out.Findings)
			if f == nil {
				t.Fatalf("a 61pt-tall %s must report %s, got %+v", tc.chartType, FindingPlotAreaCollapsed, out.Findings)
			}
			if f.Severity != "shrink_or_split" || f.Fix == nil || f.Fix.Kind != FixKindIncreaseCanvas {
				t.Errorf("finding = %+v, want shrink_or_split with fix %s", f, FixKindIncreaseCanvas)
			}
			if r, _ := f.Fix.Params["height_ratio"].(float64); r <= 1 {
				t.Errorf("height_ratio = %v, want > 1", f.Fix.Params["height_ratio"])
			}
		})
	}
}

// The same charts given the height the finding asks for read and stay quiet.
func TestTallerChartDoesNotReportCollapse(t *testing.T) {
	for _, tc := range []struct {
		chartType string
		series    []any
	}{
		{"line_chart", revenueSeries},
		{"bar_chart", revenueSeries},
		{"stacked_bar_chart", mixSeries},
	} {
		t.Run(tc.chartType, func(t *testing.T) {
			first, err := DryRender(shortChartRequest(tc.chartType, 100, tc.series))
			if err != nil {
				t.Fatal(err)
			}
			f := hasCollapsed(first)
			if f == nil {
				t.Fatal("fixture no longer collapses")
			}
			ratio, _ := f.Fix.Params["height_ratio"].(float64)
			// The ratio is a lower bound: the scaled margins grow with the
			// canvas, so give the chart half as much again.
			findings, err := DryRender(shortChartRequest(tc.chartType, int(100*ratio*1.5), tc.series))
			if err != nil {
				t.Fatal(err)
			}
			if f := hasCollapsed(findings); f != nil {
				t.Errorf("at %.1fx the height the plot must read: %+v", ratio*1.5, f)
			}
		})
	}
}
