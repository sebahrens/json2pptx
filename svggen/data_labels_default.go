package svggen

// Value labels are the default wherever they read cleanly, on every path into
// the renderer (go-slide-creator-oocqj). A bar chart whose bars carry their
// values drops the value axis and gridlines for a thin baseline (the labelled
// layout), so a raw slide_type chart looks like the same data rendered inside
// chart-insights-split instead of an Excel default.
const (
	// LabelledDefaultMaxBarPoints is the largest bar count (categories x
	// series) that gets value labels by default. Mirrors the
	// chart-insights-split cisDataLabelMaxPts.
	LabelledDefaultMaxBarPoints = 16
	// LabelledDefaultMaxLinePoints is the largest point count of a
	// single-series line / area chart that gets value labels by default;
	// labels of crossing series collide, so multi-series lines never do.
	LabelledDefaultMaxLinePoints = 12
)

// DefaultDataLabels reports whether a chart of chartType with seriesCount
// series over categoryCount categories shows its values when the author said
// nothing: bar-family charts up to LabelledDefaultMaxBarPoints bars, and a
// single-series line or area chart up to LabelledDefaultMaxLinePoints points.
func DefaultDataLabels(chartType string, seriesCount, categoryCount int) bool {
	if seriesCount <= 0 || categoryCount <= 0 {
		return false
	}
	switch chartType {
	case "bar_chart", "bar", "column_chart", "grouped_bar_chart", "grouped_bar",
		"stacked_bar_chart", "stacked_bar", "horizontal_bar_chart":
		return seriesCount*categoryCount <= LabelledDefaultMaxBarPoints
	case "line_chart", "line", "area_chart", "area":
		return seriesCount == 1 && categoryCount <= LabelledDefaultMaxLinePoints
	}
	return false
}

// dataLabelsNotOff reports false only for an explicit data.data_labels: false.
func dataLabelsNotOff(data map[string]any) bool {
	on, isBool := data["data_labels"].(bool)
	return !isBool || on
}

// resolveDefaultShowValues applies the labelled default to a chart config:
// explicit style.show_values wins, then data.data_labels: false, then the
// chart's DefaultDataLabels. data.data_labels as true or an object is applied
// afterwards by applyDataLabelsToConfig.
func resolveDefaultShowValues(req *RequestEnvelope, chartType string, data ChartData) bool {
	def := DefaultDataLabels(chartType, len(data.Series), len(data.Categories)) && dataLabelsNotOff(req.Data)
	return req.Style.ValuesShown(def)
}
