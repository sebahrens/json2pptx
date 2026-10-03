package svggen

// This file defines DataSchema implementations for chart diagram types.
// Each schema declares the allowed fields in RequestEnvelope.Data so that
// unknown keys are rejected at validate-time with UNKNOWN_FIELD errors.
//
// Schemas are added incrementally — diagram types without a DataSchema()
// method continue to accept any keys (the old behavior).

// seriesItemSchema is the shared schema for a single series object
// used by bar_chart, line_chart, area_chart, etc.
var seriesItemSchema = ObjectDataSchema("A data series", map[string]*DataSchema{
	"name":         StringDataSchema("Series name"),
	"values":       ArrayDataSchema("Numeric values", NumberDataSchema("Value"), 1),
	"time_strings": ArrayDataSchema("Time-axis string labels (ISO dates, etc.)", StringDataSchema("Time label"), 1),
	"time_values":  ArrayDataSchema("Time-axis Unix timestamps", NumberDataSchema("Unix timestamp"), 1),
}, nil) // no required — name is optional, values or time_* is validated elsewhere

// commonChartFields returns the fields shared by most category+series charts:
// categories, labels, x_labels, series, colors, footnote, axis titles.
func commonChartFields() map[string]*DataSchema {
	return map[string]*DataSchema{
		"categories":   ArrayDataSchema("Category labels for the x-axis", StringDataSchema("Category"), 1),
		"labels":       ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
		"x_labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
		"series":       ArrayDataSchema("Data series array", seriesItemSchema, 1),
		"colors":       ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color, e.g. #FF0000"), 0),
		"footnote":     StringDataSchema("Chart footnote text"),
		"x_label":      StringDataSchema("X-axis title"),
		"x_axis_title": StringDataSchema("X-axis title (alias)"),
		"y_label":      StringDataSchema("Y-axis title"),
		"y_axis_title": StringDataSchema("Y-axis title (alias)"),
		"annotations":  annotationsFieldSchema(),
		"data_labels":  dataLabelsFieldSchema(),
	}
}

// annotationsFieldSchema describes the `annotations` array every Cartesian
// chart accepts: reference lines ("Target: $180M"), fitted trendlines, and
// free-positioned callout arrows. The renderer has always supported these
// (Annotation / DrawAnnotations, read by extractAnnotations); until they were
// declared here, strict schema validation rejected the field outright and the
// deck fell back to a grey "Data unavailable" placeholder
// (go-slide-creator-pizh).
//
// Which keys apply depends on `kind`:
//   - reference_line: axis ("x" | "y", default "y"), value, label, style, color
//   - trendline:      series, method ("linear"), label, style, color
//   - callout:        x, y, text, label, color
func annotationsFieldSchema() *DataSchema {
	kind := StringDataSchema("Annotation kind")
	kind.Enum = []string{"reference_line", "trendline", "callout"}

	axis := StringDataSchema("Axis a reference_line is drawn against (default \"y\")")
	axis.Enum = []string{"x", "y"}

	style := StringDataSchema("Line style")
	style.Enum = []string{"solid", "dashed", "dotted"}

	method := StringDataSchema("Trendline fit method")
	method.Enum = []string{"linear"}

	item := ObjectDataSchema(
		"One chart annotation",
		map[string]*DataSchema{
			"kind":   kind,
			"axis":   axis,
			"value":  NumberDataSchema("reference_line: the axis value the line is drawn at"),
			"label":  StringDataSchema("Short label rendered beside the annotation, e.g. \"Target: $180M\""),
			"style":  style,
			"color":  StringDataSchema("Hex color or scheme color name; defaults to a neutral accent"),
			"series": StringDataSchema("trendline: name of the series to fit"),
			"method": method,
			"x":      NumberDataSchema("callout: x position, in category index or axis units"),
			"y":      NumberDataSchema("callout: y position, in value-axis units"),
			"text":   StringDataSchema("callout: the callout body text"),
		},
		[]string{"kind"},
	)
	return ArrayDataSchema(
		"Chart annotations: reference lines, fitted trendlines, and positioned callouts",
		item, 0)
}

// dataLabelsFieldSchema describes the `data_labels` object that turns on
// per-point value labels and controls their number format
// (go-slide-creator-pizh).
func dataLabelsFieldSchema() *DataSchema {
	showOn := StringDataSchema("Which points carry a label (default \"all\")")
	showOn.Enum = []string{"all", "last", "peaks", "first_last"}

	return ObjectDataSchema(
		"Per-point value labels on the series. `true` is shorthand for `{}` (labels on with defaults); `false` leaves them off.",
		map[string]*DataSchema{
			"format":  StringDataSchema("Go fmt number format for the label. It carries the units too: \"%.1f%%\" for a percentage, \"€%.1fM\" for a currency, \"%.0f\" to force whole numbers. Omit it and the renderer picks the precision that keeps the labels DISTINCT — a series [4.6 … 6.5] labels as 4.6/4.9/5.2/… rather than collapsing seven bars onto three values — and groups thousands (12,400)."),
			"show_on": showOn,
		},
		nil,
	)
}

// DataSchema returns the JSON Schema for bar_chart data.
func (d *BarChartDiagram) DataSchema() *DataSchema {
	fields := commonChartFields()
	fields["highlight"] = highlightFieldSchema()
	fields["sort"] = sortFieldSchema(true)
	fields["orientation"] = orientationFieldSchema()
	return ObjectDataSchema(
		"Bar chart data with categories and series. orientation \"horizontal\" draws ranked horizontal bars; sort orders the bars (default descending for non-time, non-ordinal single-series categories).",
		fields,
		[]string{"categories", "series"},
	)
}

// barFamilyFields are the shared fields of grouped / stacked bar charts.
func barFamilyFields() map[string]*DataSchema {
	fields := commonChartFields()
	fields["sort"] = sortFieldSchema(false)
	fields["orientation"] = orientationFieldSchema()
	fields["highlight"] = seriesHighlightFieldSchema()
	return fields
}

// sortFieldSchema describes data.sort on bar-family charts
// (go-slide-creator-oocqj).
func sortFieldSchema(rankedDefault bool) *DataSchema {
	desc := "Bar order by value: \"desc\" (largest first), \"asc\", or \"none\" (the authored order). Multi-series charts sort by each category's total."
	if rankedDefault {
		desc += " Omitted, a single-series chart whose categories are not periods (years, quarters, months) or an ordinal scale (ranges, stages, low/medium/high) sorts descending; set \"none\" to keep a meaningful authored order."
	} else {
		desc += " Omitted, the authored order is kept."
	}
	return EnumDataSchema(desc, SortDesc, SortAsc, SortNone)
}

// orientationFieldSchema describes data.orientation on bar-family charts.
func orientationFieldSchema() *DataSchema {
	return EnumDataSchema(
		"\"horizontal\" draws bars left-to-right with the category names in a column on the left — the form for a ranking or long category names (over ~14 characters). Default \"vertical\".",
		OrientationVertical, OrientationHorizontal)
}

// seriesHighlightFieldSchema describes data.highlight on multi-series charts:
// the series the message is about (go-slide-creator-kbzu2).
func seriesHighlightFieldSchema() *DataSchema {
	return ArrayDataSchema(
		"Series the slide is about: 0-based series indices or series names. Highlighted series keep accent1 (a 2pt line, labelled at its end, on line / area charts); every other series turns neutral dk1 at 38% (1pt lines without markers). Omitted, every series keeps its palette colour.",
		&DataSchema{Description: "0-based series index (integer) or series name (string)"},
		0,
	)
}

// withSeriesHighlight adds the series highlight to a line / area schema.
func withSeriesHighlight(fields map[string]*DataSchema) map[string]*DataSchema {
	fields["highlight"] = seriesHighlightFieldSchema()
	return fields
}

// pieSortFieldSchema describes data.sort on pie / donut charts
// (go-slide-creator-ihlsr).
func pieSortFieldSchema() *DataSchema {
	return EnumDataSchema(
		"Slice order clockwise from 12 o'clock: \"desc\" (default — largest first), \"asc\", or \"none\" (the authored order, for slices with a natural sequence).",
		SortDesc, SortAsc, SortNone)
}

// sliceHighlightFieldSchema describes data.highlight on pie / donut charts.
func sliceHighlightFieldSchema() *DataSchema {
	return ArrayDataSchema(
		"Slices the slide is about: 0-based slice indices or slice labels. Highlighted slices are painted in accent1 and every other slice in neutral greys. Omitted, slices take the series palette.",
		&DataSchema{Description: "0-based slice index (integer) or slice label (string)"},
		0,
	)
}

// highlightFieldSchema describes data.highlight on a single-series bar chart:
// the bars painted in accent1 while every other bar stays neutral
// (go-slide-creator-sdxii).
func highlightFieldSchema() *DataSchema {
	return ArrayDataSchema(
		"Bars to paint in accent1 on a single-series bar chart; every other bar is neutral (dk1 at 38%). Each entry is a 0-based category index or a category name. Omit it and a time series (years, quarters, months) accents its last bar, any other chart its largest; [] accents none. Ignored by multi-series charts.",
		&DataSchema{Description: "0-based category index (integer) or category name (string)"},
		0,
	)
}

// DataSchema returns the JSON Schema for line_chart data.
func (d *LineChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Line chart data with categories and series. Supports time-series via time_strings/time_values in series objects.",
		withSeriesHighlight(commonChartFields()),
		nil, // categories not always required (time-series mode)
	)
}

// DataSchema returns the JSON Schema for pie_chart data.
func (d *PieChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Pie chart data with categories/labels and values.",
		map[string]*DataSchema{
			"categories":            ArrayDataSchema("Slice labels", StringDataSchema("Label"), 1),
			"labels":                ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"x_labels":              ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"values":                ArrayDataSchema("Slice values", NumberDataSchema("Value"), 1),
			"colors":                ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color, e.g. #FF0000"), 0),
			"footnote":              StringDataSchema("Chart footnote text"),
			"highlight":             sliceHighlightFieldSchema(),
			"sort":                  pieSortFieldSchema(),
			"group_small_below_pct": NumberDataSchema("Slices under this share of the total (percent, default 3) fold into one trailing \"Other\" slice when two or more qualify; 0 never folds. A highlighted slice is never folded."),
		},
		[]string{"values"},
	)
}

// DataSchema returns the JSON Schema for donut_chart data (same as pie).
func (d *DonutChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Donut chart data with categories/labels and values.",
		map[string]*DataSchema{
			"categories":            ArrayDataSchema("Slice labels", StringDataSchema("Label"), 1),
			"labels":                ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"x_labels":              ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"values":                ArrayDataSchema("Slice values", NumberDataSchema("Value"), 1),
			"colors":                ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color, e.g. #FF0000"), 0),
			"footnote":              StringDataSchema("Chart footnote text"),
			"highlight":             sliceHighlightFieldSchema(),
			"sort":                  pieSortFieldSchema(),
			"group_small_below_pct": NumberDataSchema("Slices under this share of the total (percent, default 3) fold into one trailing \"Other\" slice when two or more qualify; 0 never folds. A highlighted slice is never folded."),
		},
		[]string{"values"},
	)
}

// DataSchema returns the JSON Schema for area_chart data.
func (d *AreaChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Area chart data with categories and series.",
		withSeriesHighlight(commonChartFields()),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for stacked_bar_chart data.
func (d *StackedBarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Stacked bar chart data with categories and series.",
		barFamilyFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for grouped_bar_chart data.
func (d *GroupedBarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Grouped bar chart data with categories and series.",
		barFamilyFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for stacked_area_chart data.
func (d *StackedAreaChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Stacked area chart data with categories and series.",
		commonChartFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for radar_chart data.
func (d *RadarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Radar/spider chart data with axes and series.",
		map[string]*DataSchema{
			"categories": ArrayDataSchema("Axis labels (canonical)", StringDataSchema("Axis name"), 3),
			"labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Axis name"), 3),
			"axes":       ArrayDataSchema("Alias for categories", StringDataSchema("Axis name"), 3),
			"series":     ArrayDataSchema("Data series array", seriesItemSchema, 1),
			"colors":     ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color"), 0),
			"footnote":   StringDataSchema("Chart footnote text"),
		},
		nil, // one of categories/labels/axes is required but validated elsewhere
	)
}

// scatterPointSchema describes a single scatter/bubble point.
var scatterPointSchema = ObjectDataSchema("A data point", map[string]*DataSchema{
	"x":     NumberDataSchema("X coordinate"),
	"y":     NumberDataSchema("Y coordinate"),
	"size":  NumberDataSchema("Bubble size (bubble_chart only)"),
	"label": StringDataSchema("Point label"),
}, []string{"x", "y"})

// scatterSeriesSchema describes a scatter/bubble series with points.
// Supports two formats: point objects (points) or parallel arrays (values + x_values).
var scatterSeriesSchema = ObjectDataSchema("A point series", map[string]*DataSchema{
	"name":     StringDataSchema("Series name"),
	"points":   ArrayDataSchema("Data points (format 2)", scatterPointSchema, 1),
	"values":   ArrayDataSchema("Y values (format 1, parallel arrays)", NumberDataSchema("Y value"), 1),
	"x_values": ArrayDataSchema("X values (format 1, parallel arrays)", NumberDataSchema("X value"), 1),
	"labels":   ArrayDataSchema("Point labels (format 1)", StringDataSchema("Label"), 0),
}, nil) // one of points or values is required but validated elsewhere

// DataSchema returns the JSON Schema for scatter_chart data.
func (d *ScatterChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Scatter plot data with x/y data points.",
		map[string]*DataSchema{
			"categories":   ArrayDataSchema("Optional category labels", StringDataSchema("Category"), 0),
			"labels":       ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 0),
			"x_labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 0),
			"series":       ArrayDataSchema("Point series array", scatterSeriesSchema, 1),
			"colors":       ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color"), 0),
			"footnote":     StringDataSchema("Chart footnote text"),
			"x_label":      StringDataSchema("X-axis title"),
			"x_axis_title": StringDataSchema("X-axis title (alias)"),
			"y_label":      StringDataSchema("Y-axis title"),
			"y_axis_title": StringDataSchema("Y-axis title (alias)"),
		},
		[]string{"series"},
	)
}

// bubbleSeriesSchema is scatterSeriesSchema plus the parallel bubble sizes
// parseBubbleChartData reads.
var bubbleSeriesSchema = ObjectDataSchema("A bubble series", map[string]*DataSchema{
	"name":          StringDataSchema("Series name"),
	"points":        ArrayDataSchema("Data points {x, y, size, label} (format 2)", scatterPointSchema, 1),
	"values":        ArrayDataSchema("Y values (format 1, parallel arrays)", NumberDataSchema("Y value"), 1),
	"x_values":      ArrayDataSchema("X values (format 1, parallel arrays)", NumberDataSchema("X value"), 1),
	"bubble_values": ArrayDataSchema("Bubble sizes (format 1, parallel arrays)", NumberDataSchema("Bubble size"), 1),
	"labels":        ArrayDataSchema("Point labels (format 1)", StringDataSchema("Label"), 0),
}, nil)

// DataSchema returns the JSON Schema for bubble_chart data.
func (d *BubbleChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Bubble chart data with x/y/size data points.",
		map[string]*DataSchema{
			"categories":   ArrayDataSchema("Optional category labels", StringDataSchema("Category"), 0),
			"labels":       ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 0),
			"x_labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 0),
			"series":       ArrayDataSchema("Point series array", bubbleSeriesSchema, 1),
			"colors":       ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color"), 0),
			"footnote":     StringDataSchema("Chart footnote text"),
			"x_label":      StringDataSchema("X-axis title"),
			"x_axis_title": StringDataSchema("X-axis title (alias)"),
			"y_label":      StringDataSchema("Y-axis title"),
			"y_axis_title": StringDataSchema("Y-axis title (alias)"),
		},
		[]string{"series"},
	)
}
