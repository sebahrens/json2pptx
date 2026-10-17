package svggen

import (
	"github.com/sebahrens/json2pptx/svggen/core"
	"testing"
)

func TestAuthoredChartClutterDefaultsAndOverrides(t *testing.T) {
	for _, typ := range []string{"bar_chart", "grouped_bar_chart", "line_chart", "area_chart"} {
		t.Run(typ, func(t *testing.T) {
			req := &RequestEnvelope{Type: typ, Data: map[string]any{"categories": []any{"Q1", "Q2", "Q3"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{2.0, 4.0, 3.0}}}}}
			b, _, err := RenderWithHelper(req, func(*SVGBuilder, *RequestEnvelope) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range b.Findings() {
				if f.Code == core.FindingRedundantLegend || f.Code == core.FindingChartVerticalGridlines {
					t.Fatalf("default clutter finding %+v", f)
				}
			}
			yes := true
			req.Style.ShowLegend = true
			req.Style.ChartStyle = &ChartStyleOverrides{ShowVerticalGridlines: &yes}
			b, _, err = RenderWithHelper(req, func(*SVGBuilder, *RequestEnvelope) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, f := range b.Findings() {
				seen[f.Code] = true
			}
			if !seen[core.FindingRedundantLegend] || !seen[core.FindingChartVerticalGridlines] {
				t.Fatalf("missing authored warnings: %+v", b.Findings())
			}
		})
	}
}

func TestChartLegendWarningUsesWinningOverride(t *testing.T) {
	for _, force := range []bool{false, true} {
		req := &RequestEnvelope{Type: "bar_chart", Data: map[string]any{"series": []any{map[string]any{"name": "Revenue", "values": []any{1.0}}}}, Style: StyleSpec{ShowLegend: true, ChartStyle: &ChartStyleOverrides{ShowSingleSeriesLegend: &force}}}
		b, _, err := RenderWithHelper(req, func(*SVGBuilder, *RequestEnvelope) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		var warning *Finding
		for _, f := range b.Findings() {
			if f.Code == core.FindingRedundantLegend {
				copy := f
				warning = &copy
			}
		}
		if !force {
			if warning != nil {
				t.Fatal("false specific override did not suppress warning")
			}
			continue
		}
		if warning == nil || warning.Field != "style.chart_style.show_single_series_legend" || warning.Fix == nil || warning.Fix.Params["show_single_series_legend"] != false {
			t.Fatalf("wrong authored override fix %+v", warning)
		}
	}
}
