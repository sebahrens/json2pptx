package svggen

import (
	"regexp"
	"strconv"
	"testing"
)

// Filled bar segments render as closed axis-aligned paths: M x y0 H x1 V y1 H x z.
var stackedBarRectRe = regexp.MustCompile(`<path d="M(-?[0-9.]+) (-?[0-9.]+)H-?[0-9.]+V(-?[0-9.]+)H-?[0-9.]+z" fill=`)
var svgViewBoxHeightRe = regexp.MustCompile(`viewBox="0 0 [0-9.]+ ([0-9.]+)"`)

// go-slide-creator-s1uvj.29: stacked bar charts computed their domain as
// [0, max net stack sum] and stacked every segment on one running total, so a
// negative segment was drawn off the bottom of the canvas (Refunds of -30 on a
// Revenue of 10 reached y=738.9 in a 667-high viewBox), and a stack whose net
// sum was <= 0 collapsed the domain to [0,0] and raised all_zero_series.
// A diverging stack is required: positives stack up from zero, negatives stack
// down from zero, and the domain spans min(sum neg)..max(sum pos).
func TestStackedBar_NegativeSegmentsDivergeFromZero(t *testing.T) {
	cases := []struct {
		name             string
		series           []any
		wantNeg, wantPos float64
	}{
		{
			name: "mixed revenue and refunds",
			series: []any{
				map[string]any{"name": "Revenue", "values": []any{50.0, 10.0, 20.0}},
				map[string]any{"name": "Refunds", "values": []any{-5.0, -30.0, -5.0}},
			},
			wantNeg: -30, wantPos: 50,
		},
		{
			name: "net-negative stacks",
			series: []any{
				map[string]any{"name": "In", "values": []any{10.0, 10.0, 10.0}},
				map[string]any{"name": "Out", "values": []any{-30.0, -30.0, -30.0}},
			},
			wantNeg: -30, wantPos: 10,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type: "stacked_bar_chart",
				Data: map[string]any{
					"categories": []any{"A", "B", "C"},
					"series":     tc.series,
				},
				Output: OutputSpec{Width: 800, Height: 600},
			}
			data, err := extractChartData(req)
			if err != nil {
				t.Fatal(err)
			}
			cfg := DefaultBarChartConfig(800, 600)
			cfg.Stacked = true
			bc := NewBarChart(NewSVGBuilder(800, 600), cfg)
			min, max := bc.calculateDomain(data)
			if min > tc.wantNeg || max < tc.wantPos {
				t.Errorf("domain = [%v, %v], want to cover [%v, %v]", min, max, tc.wantNeg, tc.wantPos)
			}

			output, err := RenderMultiFormatWithFindings(req, "svg")
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if f := findFindingByCode(output.Findings, FindingAllZeroSeries); f != nil {
				t.Errorf("unexpected %s finding for non-zero data", FindingAllZeroSeries)
			}
			svg := string(output.SVG.Content)
			vb := svgViewBoxHeightRe.FindStringSubmatch(svg)
			if vb == nil {
				t.Fatal("no viewBox in SVG")
			}
			canvasH, _ := strconv.ParseFloat(vb[1], 64)
			rects := stackedBarRectRe.FindAllStringSubmatch(svg, -1)
			if len(rects) == 0 {
				t.Fatal("no filled bar segments found in SVG")
			}
			for _, m := range rects {
				for _, ys := range []string{m[2], m[3]} {
					y, _ := strconv.ParseFloat(ys, 64)
					if y < 0 || y > canvasH {
						t.Errorf("bar segment %q reaches y=%v outside the %v-high canvas", m[0], y, canvasH)
					}
				}
			}
		})
	}
}
