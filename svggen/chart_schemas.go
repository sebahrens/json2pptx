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
	}
}

// DataSchema returns the JSON Schema for bar_chart data.
func (d *BarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Bar chart data with categories and series. Supports grouped, stacked, and horizontal variants.",
		commonChartFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for line_chart data.
func (d *LineChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Line chart data with categories and series. Supports time-series via time_strings/time_values in series objects.",
		commonChartFields(),
		nil, // categories not always required (time-series mode)
	)
}

// DataSchema returns the JSON Schema for pie_chart data.
func (d *PieChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Pie chart data with categories/labels and values.",
		map[string]*DataSchema{
			"categories": ArrayDataSchema("Slice labels", StringDataSchema("Label"), 1),
			"labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"x_labels":   ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"values":     ArrayDataSchema("Slice values", NumberDataSchema("Value"), 1),
			"colors":     ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color, e.g. #FF0000"), 0),
			"footnote":   StringDataSchema("Chart footnote text"),
		},
		[]string{"values"},
	)
}

// DataSchema returns the JSON Schema for donut_chart data (same as pie).
func (d *DonutChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Donut chart data with categories/labels and values.",
		map[string]*DataSchema{
			"categories": ArrayDataSchema("Slice labels", StringDataSchema("Label"), 1),
			"labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"x_labels":   ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"values":     ArrayDataSchema("Slice values", NumberDataSchema("Value"), 1),
			"colors":     ArrayDataSchema("Custom hex color overrides", StringDataSchema("Hex color, e.g. #FF0000"), 0),
			"footnote":   StringDataSchema("Chart footnote text"),
		},
		[]string{"values"},
	)
}

// DataSchema returns the JSON Schema for area_chart data.
func (d *AreaChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Area chart data with categories and series.",
		commonChartFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for stacked_bar_chart data.
func (d *StackedBarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Stacked bar chart data with categories and series.",
		commonChartFields(),
		[]string{"categories", "series"},
	)
}

// DataSchema returns the JSON Schema for grouped_bar_chart data.
func (d *GroupedBarChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Grouped bar chart data with categories and series.",
		commonChartFields(),
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

// DataSchema returns the JSON Schema for bubble_chart data.
func (d *BubbleChartDiagram) DataSchema() *DataSchema {
	return ObjectDataSchema(
		"Bubble chart data with x/y/size data points.",
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
