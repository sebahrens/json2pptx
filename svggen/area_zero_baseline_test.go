package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-6wfe: a filled mark is read against the axis — the band's
// area IS the claim, and on a stacked area the bands are a part-to-whole. With
// revenue data in the 28–61 range the area axis started at 30 and the
// stacked_area axis at 40, so a 40-unit base looked like zero and the
// composition was simply wrong. stacked_bar already baselined at zero, so the
// inconsistency was inside svggen.
func TestFilledChartsBaselineAtZero(t *testing.T) {
	// Realistic revenue data far from zero — the shape that triggered the bug.
	data := ChartData{
		Categories: []string{"FY21", "FY22", "FY23", "FY24"},
		Series: []ChartSeries{
			{Name: "Licence", Values: []float64{28, 34, 41, 47}},
			{Name: "Services", Values: []float64{45, 51, 56, 61}},
		},
	}

	t.Run("area baselines at zero", func(t *testing.T) {
		cfg := DefaultAreaChartConfig(900, 500)
		lc := NewLineChart(NewSVGBuilder(900, 500), cfg.LineChartConfig)
		min, _ := lc.calculateYDomain(data)
		if min > 0 {
			t.Errorf("area chart y-domain starts at %v; a filled band must be measured from zero", min)
		}
	})

	t.Run("plain line keeps a zoomed axis", func(t *testing.T) {
		cfg := DefaultLineChartConfig(900, 500)
		if cfg.FillArea {
			t.Fatal("DefaultLineChartConfig should not fill the area")
		}
		lc := NewLineChart(NewSVGBuilder(900, 500), cfg)
		min, _ := lc.calculateYDomain(data)
		if min <= 0 {
			t.Errorf("plain line chart y-domain starts at %v; a line encodes value by position, "+
				"so the zoomed axis is intended here", min)
		}
	})

	t.Run("negative values still reach their minimum", func(t *testing.T) {
		withNegatives := ChartData{
			Categories: []string{"Q1", "Q2", "Q3"},
			Series:     []ChartSeries{{Name: "Net", Values: []float64{-15, 8, 22}}},
		}
		cfg := DefaultAreaChartConfig(900, 500)
		lc := NewLineChart(NewSVGBuilder(900, 500), cfg.LineChartConfig)
		min, max := lc.calculateYDomain(withNegatives)
		if min > -15 {
			t.Errorf("y-domain min = %v, must cover the -15 data point", min)
		}
		if max < 22 {
			t.Errorf("y-domain max = %v, must cover the 22 data point", max)
		}
	})
}

// End-to-end: the rendered area and stacked_area SVGs must carry a "0" tick.
func TestRenderedFilledCharts_HaveZeroTick(t *testing.T) {
	// stacked_bar is included as the control: it already baselined at zero,
	// which is what made the area inconsistency a bug rather than a choice.
	for _, ct := range []string{"area_chart", "stacked_area_chart", "stacked_bar_chart"} {
		t.Run(ct, func(t *testing.T) {
			doc, err := Render(&RequestEnvelope{
				Type: ct,
				Data: map[string]any{
					"categories": []any{"FY21", "FY22", "FY23", "FY24"},
					"series": []any{
						map[string]any{"name": "Licence", "values": []any{28.0, 34.0, 41.0, 47.0}},
						map[string]any{"name": "Services", "values": []any{45.0, 51.0, 56.0, 61.0}},
					},
				},
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !hasAxisTick(string(doc.Content), "0") {
				t.Errorf("%s does not render a zero tick — the filled bands are truncated", ct)
			}
		})
	}
}

// hasAxisTick reports whether the SVG contains a text node whose entire body is
// the given tick label.
func hasAxisTick(svg, label string) bool {
	return strings.Contains(svg, ">"+label+"<")
}
