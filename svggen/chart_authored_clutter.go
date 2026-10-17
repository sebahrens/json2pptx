package svggen

import (
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
)

func reportAuthoredChartClutter(b *SVGBuilder, req *RequestEnvelope) {
	chart := DefaultRegistry().Get(req.Type)
	if chart == nil {
		return
	}
	typ := chart.Type()
	barLine := typ == "bar_chart" || typ == "grouped_bar_chart" || typ == "stacked_bar_chart" || typ == "line_chart" || typ == "area_chart"
	if !barLine {
		return
	}
	series, _ := toAnySlice(req.Data["series"])
	forced := req.Style.ShowLegend
	field, key := "style.show_legend", "show_legend"
	if req.Style.ChartStyle != nil && req.Style.ChartStyle.ShowSingleSeriesLegend != nil {
		forced = *req.Style.ChartStyle.ShowSingleSeriesLegend
		field, key = "style.chart_style.show_single_series_legend", "show_single_series_legend"
	}
	if len(series) == 1 && forced {
		b.AddFinding(Finding{Code: core.FindingRedundantLegend, Field: field, Message: "Authored style forces a one-series legend; use the chart title or a direct series label when the legend adds no information.", Severity: "warning", Fix: &FixSuggestion{Kind: FixKindReplaceValue, Params: map[string]any{key: false}}})
	}
	if req.Style.ChartStyle != nil && req.Style.ChartStyle.ShowVerticalGridlines != nil && *req.Style.ChartStyle.ShowVerticalGridlines {
		b.AddFinding(Finding{Code: core.FindingChartVerticalGridlines, Field: "style.chart_style.show_vertical_gridlines", Message: "Authored style requests vertical gridlines on a " + strings.TrimSuffix(typ, "_chart") + " chart; keep them only when they help read values.", Severity: "warning", Fix: &FixSuggestion{Kind: FixKindReplaceValue, Params: map[string]any{"show_vertical_gridlines": false}}})
	}
}
