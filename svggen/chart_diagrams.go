// Package svggen provides SVG diagram generation.
// This file implements Diagram interfaces for standard chart types (bar, line, pie, donut)
// to enable rendering via the svggen registry.
package svggen

import (
	"fmt"
)

// =============================================================================
// Bar Chart Diagram
// =============================================================================

// BarChartDiagram implements the Diagram interface for bar charts.
type BarChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a bar chart.
func (d *BarChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "bar_chart", true, 1)
}

// Render generates an SVG document for the bar chart.
func (d *BarChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *BarChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultBarChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend defaults to true in DefaultChartConfig; the Draw method
		// only renders legends for multi-series data (len(series) > 1).
		// Do not override with req.Style.ShowLegend because Go's bool
		// zero-value (false) is indistinguishable from "not set", which
		// would suppress legends on multi-series charts when callers omit
		// the show_legend field from JSON.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		// Only disable if the request explicitly sets show_grid (Go zero-value means unset).

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewBarChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("bar_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Line Chart Diagram
// =============================================================================

// LineChartDiagram implements the Diagram interface for line charts.
type LineChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a line chart.
func (d *LineChartDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data

	// Check for series (required for all line charts)
	series, hasSeries := data["series"]
	if !hasSeries {
		return &ValidationError{Field: "data.series", Code: ErrCodeRequired, Message: "line_chart requires 'series' field. Expected format: {\"categories\": [\"Jan\", \"Feb\"], \"series\": [{\"name\": \"Sales\", \"values\": [10, 20]}]}"}
	}

	seriesSlice, ok := toSeriesSlice(series)
	if !ok || len(seriesSlice) == 0 {
		return &ValidationError{Field: "data.series", Code: ErrCodeInvalidType, Message: "line_chart 'series' must be a non-empty array of objects, e.g. [{\"name\": \"Sales\", \"values\": [10, 20]}]", Value: series}
	}

	// Check if this is a time-series chart (has time_strings or time_values in series)
	isTimeSeries := false
	for _, s := range seriesSlice {
		if _, hasTimeStrings := s["time_strings"]; hasTimeStrings {
			isTimeSeries = true
			break
		}
		if _, hasTimeValues := s["time_values"]; hasTimeValues {
			isTimeSeries = true
			break
		}
	}

	// Categories are required for non-time-series charts
	normalizeCategoryAliases(data)
	if !isTimeSeries {
		categories, hasCats := data["categories"]
		if !hasCats {
			return &ValidationError{Field: "data.categories", Code: ErrCodeRequired, Message: "line_chart requires 'categories' field (or time_strings/time_values in series for time-series). Expected: {\"categories\": [\"Jan\", \"Feb\"], ...} or {\"series\": [{\"time_strings\": [\"2024-01\", \"2024-02\"], \"values\": [10, 20]}]}"}
		}

		catSlice, ok := toStringSlice(categories)
		if !ok || len(catSlice) == 0 {
			return &ValidationError{Field: "data.categories", Code: ErrCodeInvalidType, Message: "line_chart 'categories' must be a non-empty array of strings, e.g. [\"Jan\", \"Feb\", \"Mar\"]", Value: categories}
		}
	}

	return nil
}

// Render generates an SVG document for the line chart.
func (d *LineChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *LineChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultLineChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewLineChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("line_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Pie Chart Diagram
// =============================================================================

// PieChartDiagram implements the Diagram interface for pie charts.
type PieChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a pie chart.
func (d *PieChartDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data

	// Check for values
	values, hasValues := data["values"]
	if !hasValues {
		return &ValidationError{Field: "data.values", Code: ErrCodeRequired, Message: "pie_chart requires 'values' field. Expected format: {\"categories\": [\"A\", \"B\", \"C\"], \"values\": [40, 30, 30]}"}
	}

	valSlice, ok := toFloat64Slice(values)
	if !ok || len(valSlice) == 0 {
		return &ValidationError{Field: "data.values", Code: ErrCodeInvalidType, Message: "pie_chart 'values' must be a non-empty array of numbers, e.g. [40, 30, 30]", Value: values}
	}

	return nil
}

// Render generates an SVG document for the pie chart.
func (d *PieChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *PieChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractPieChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultPieChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// Pie charts ALWAYS show legend by default - it's essential to know what each segment represents
		// The percentage labels on slices show values (40.0%) but not category names (Development, Marketing)
		// Without the legend, viewers cannot understand what the chart is showing
		// Note: DefaultPieChartConfig sets ShowLegend=true, so we keep that default

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		chart := NewPieChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("pie_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Donut Chart Diagram
// =============================================================================

// DonutChartDiagram implements the Diagram interface for donut charts.
type DonutChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a donut chart.
func (d *DonutChartDiagram) Validate(req *RequestEnvelope) error {
	// Same validation as pie chart
	data := req.Data

	values, hasValues := data["values"]
	if !hasValues {
		return &ValidationError{Field: "data.values", Code: ErrCodeRequired, Message: "donut_chart requires 'values' field. Expected format: {\"categories\": [\"A\", \"B\", \"C\"], \"values\": [40, 30, 30]}"}
	}

	valSlice, ok := toFloat64Slice(values)
	if !ok || len(valSlice) == 0 {
		return &ValidationError{Field: "data.values", Code: ErrCodeInvalidType, Message: "donut_chart 'values' must be a non-empty array of numbers, e.g. [40, 30, 30]", Value: values}
	}

	return nil
}

// Render generates an SVG document for the donut chart.
func (d *DonutChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *DonutChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractPieChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultDonutChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// Donut charts ALWAYS show legend by default - it's essential to know what each segment represents
		// The percentage labels on slices show values (40.0%) but not category names (Development, Marketing)
		// Without the legend, viewers cannot understand what the chart is showing
		// Note: DefaultDonutChartConfig sets ShowLegend=true, so we keep that default

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		chart := NewPieChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("donut_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Area Chart Diagram
// =============================================================================

// AreaChartDiagram implements the Diagram interface for area charts.
type AreaChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for an area chart.
func (d *AreaChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "area_chart", true, 1)
}

// Render generates an SVG document for the area chart.
func (d *AreaChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *AreaChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultAreaChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewAreaChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("area_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Radar Chart Diagram
// =============================================================================

// RadarChartDiagram implements the Diagram interface for radar charts.
type RadarChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a radar chart.
func (d *RadarChartDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data

	// Accept "labels" or "axes" as aliases for "categories"
	if _, hasCats := data["categories"]; !hasCats {
		if labels, ok := data["labels"]; ok {
			data["categories"] = labels
		} else if axes, ok := data["axes"]; ok {
			data["categories"] = axes
		}
	}

	// Check for categories (axes)
	categories, hasCats := data["categories"]
	if !hasCats {
		return &ValidationError{Field: "data.categories", Code: ErrCodeRequired, Message: "radar_chart requires 'categories' (or 'labels'/'axes') field. Expected format: {\"categories\": [\"Speed\", \"Power\", \"Range\"], \"series\": [{\"name\": \"Model A\", \"values\": [8, 6, 7]}]}"}
	}

	catSlice, ok := toStringSlice(categories)
	if !ok || len(catSlice) < 3 {
		return &ValidationError{Field: "data.categories", Code: ErrCodeConstraint, Message: "radar_chart 'categories' must have at least 3 items (axes), e.g. [\"Speed\", \"Power\", \"Range\"]", Value: categories}
	}

	// Check for series
	series, hasSeries := data["series"]
	if !hasSeries {
		return &ValidationError{Field: "data.series", Code: ErrCodeRequired, Message: "radar_chart requires 'series' field. Expected: {\"series\": [{\"name\": \"Model A\", \"values\": [8, 6, 7]}]}"}
	}

	seriesSlice, ok := toSeriesSlice(series)
	if !ok || len(seriesSlice) == 0 {
		return &ValidationError{Field: "data.series", Code: ErrCodeInvalidType, Message: "radar_chart 'series' must be a non-empty array of objects, e.g. [{\"name\": \"Model A\", \"values\": [8, 6, 7]}]", Value: series}
	}

	return nil
}

// Render generates an SVG document for the radar chart.
func (d *RadarChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *RadarChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultRadarChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		// Only disable if the request explicitly sets show_grid (Go zero-value means unset).

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		chart := NewRadarChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("radar_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Scatter Chart Diagram
// =============================================================================

// ScatterChartDiagram implements the Diagram interface for scatter charts.
type ScatterChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a scatter chart.
func (d *ScatterChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "scatter_chart", false, 1)
}

// Render generates an SVG document for the scatter chart.
func (d *ScatterChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *ScatterChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractScatterChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultScatterChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// Map ShowValues → ShowLabels so that enabling show_values in the
		// request style causes scatter point labels to be rendered.
		config.ShowLabels = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		// Supports "x_label"/"y_label" (concise) and "x_axis_title"/"y_axis_title" (explicit).
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")

		// When axis titles are present, increase bottom/left margins to
		// accommodate the title text so it does not overlap the tick labels.
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewScatterChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("scatter_chart render failed: %w", err)
		}
		return nil
	})
}

// extractScatterChartData extracts ChartData for scatter charts with X values.
//
// Supports two series formats:
//  1. Parallel arrays: {"values": [y1,y2,...], "x_values": [x1,x2,...]}
//  2. Point objects:   {"points": [{"x": x1, "y": y1}, {"x": x2, "y": y2}, ...]}
//
// Format 2 is produced by ChartSpec.ToDiagramSpec() when converting
// map[string]float64 data into scatter chart format.
//nolint:gocognit // complex chart rendering logic
func extractScatterChartData(req *RequestEnvelope) (ChartData, error) {
	data := req.Data
	chartData := ChartData{
		Title:    req.Title,
		Footnote: req.Subtitle,
	}

	// Extract series with x_values support
	if series, ok := data["series"]; ok {
		seriesSlice, ok := toSeriesSlice(series)
		if !ok {
			return chartData, fmt.Errorf("invalid series format")
		}

		chartData.Series = make([]ChartSeries, len(seriesSlice))
		for i, s := range seriesSlice {
			cs := ChartSeries{}

			if name, ok := s["name"].(string); ok {
				cs.Name = name
			}

			// Format 1: parallel arrays (values + x_values)
			if values, ok := s["values"]; ok {
				if valSlice, ok := toFloat64Slice(values); ok {
					cs.Values = valSlice
				}
			}

			if xValues, ok := s["x_values"]; ok {
				if xSlice, ok := toFloat64Slice(xValues); ok {
					cs.XValues = xSlice
				}
			}

			// Extract labels for Format 1 (parallel arrays).
			// Labels are string arrays that identify each data point.
			if labels, ok := s["labels"]; ok {
				if lblSlice, ok := toStringSlice(labels); ok {
					cs.Labels = lblSlice
				}
			}

			// Format 2: point objects (points: [{x, y}, ...])
			// Used by ChartSpec.ToDiagramSpec() for scatter charts.
			if len(cs.Values) == 0 {
				if points, ok := s["points"]; ok {
					if pointSlice, ok := toPointSlice(points); ok {
						cs.XValues = make([]float64, len(pointSlice))
						cs.Values = make([]float64, len(pointSlice))
						cs.Labels = make([]string, len(pointSlice))
						for j, pt := range pointSlice {
							cs.XValues[j] = pt.x
							cs.Values[j] = pt.y
							cs.Labels[j] = pt.label
						}
					}
				}
			}

			chartData.Series[i] = cs
		}
	}

	return chartData, nil
}

// scatterPoint holds x, y, and optional label from a point object.
type scatterPoint struct {
	x, y  float64
	label string
}

// toPointSlice converts an interface{} to a slice of scatterPoint.
// Expects []map[string]any with "x" and "y" keys (and optional "label").
func toPointSlice(v any) ([]scatterPoint, bool) {
	var maps []map[string]any

	switch val := v.(type) {
	case []map[string]any:
		maps = val
	case []any:
		maps = make([]map[string]any, len(val))
		for i, item := range val {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			maps[i] = m
		}
	default:
		return nil, false
	}

	if len(maps) == 0 {
		return nil, false
	}

	points := make([]scatterPoint, len(maps))
	for i, m := range maps {
		xVal, okX := toFloat64(m["x"])
		yVal, okY := toFloat64(m["y"])
		if !okX || !okY {
			return nil, false
		}
		points[i] = scatterPoint{x: xVal, y: yVal}
		if label, ok := m["label"].(string); ok {
			points[i].label = label
		}
	}
	return points, true
}

// toFloat64 converts a numeric interface{} to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// =============================================================================
// Stacked Bar Chart Diagram
// =============================================================================

// StackedBarChartDiagram implements the Diagram interface for stacked bar charts.
type StackedBarChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a stacked bar chart.
func (d *StackedBarChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "stacked_bar_chart", true, 1)
}

// Render generates an SVG document for the stacked bar chart.
func (d *StackedBarChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *StackedBarChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultBarChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		config.Stacked = true // Enable stacking
		// Stacked bar charts always show legend - it's essential to identify segments
		config.ShowLegend = true

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewBarChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("stacked_bar_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// getOutputDimensions extracts width and height from the request.
// Priority: explicit Width/Height > Preset > defaults (800x600).
func getOutputDimensions(req *RequestEnvelope) (width, height float64) {
	width = 800
	height = 600

	// Check preset first (can be overridden by explicit dimensions)
	if req.Output.Preset != "" {
		if w, h, ok := getPresetDimensions(req.Output.Preset); ok {
			width = float64(w)
			height = float64(h)
		}
	}

	// Explicit dimensions override preset
	if req.Output.Width > 0 {
		width = float64(req.Output.Width)
	}
	if req.Output.Height > 0 {
		height = float64(req.Output.Height)
	}

	return width, height
}

// applyStyleToBuilder applies style settings to the SVGBuilder.
func applyStyleToBuilder(builder *SVGBuilder, style StyleSpec) {
	// Apply the full StyleSpec → StyleGuide conversion which handles
	// ThemeColors (template palette), named palettes, custom color arrays,
	// font family, background, and surface colors.
	// Previously this only handled explicit Palette arrays and missed ThemeColors
	// entirely, causing all charts to render with the default Tableau 10 palette
	// regardless of the template's theme.
	guide := StyleGuideFromSpec(style)
	builder.SetStyleGuide(guide)

	// Draw background fill if explicitly specified.
	// StyleGuideFromSpec stores the background color in the palette, but
	// doesn't draw it — that's the builder's job.
	if style.Background != "" {
		if bgColor, err := ParseColor(style.Background); err == nil {
			builder.SetFillColor(bgColor)
			builder.FillRect(builder.Bounds())
		}
	}
}

// extractPalette extracts colors from the palette field.
func extractPalette(palette any) []Color {
	if palette == nil {
		return nil
	}

	// Handle string array (custom hex colors)
	if strSlice, ok := toStringSlice(palette); ok {
		colors := make([]Color, 0, len(strSlice))
		for _, s := range strSlice {
			if c, err := ParseColor(s); err == nil {
				colors = append(colors, c)
			}
		}
		return colors
	}

	// Handle named palette (string)
	if name, ok := palette.(string); ok {
		p := GetPaletteByName(name)
		return p.AccentColors()
	}

	return nil
}

// extractChartColors extracts an optional color override array from req.Data["colors"].
// The field should be an array of hex color strings (e.g. ["#FF0000", "#00FF00"]).
// Returns nil when no colors are specified, causing charts to fall back to the
// style guide palette via resolveColors.
func extractChartColors(data map[string]any) []Color {
	raw, ok := data["colors"]
	if !ok {
		return nil
	}
	strSlice, ok := toStringSlice(raw)
	if !ok || len(strSlice) == 0 {
		return nil
	}
	colors := make([]Color, 0, len(strSlice))
	for _, s := range strSlice {
		if c, err := ParseColor(s); err == nil {
			colors = append(colors, c)
		}
	}
	if len(colors) == 0 {
		return nil
	}
	return colors
}

// extractChartData extracts ChartData from a request envelope.
func extractChartData(req *RequestEnvelope) (ChartData, error) {
	data := req.Data
	chartData := ChartData{
		Title:    req.Title,
		Footnote: req.Subtitle,
	}

	// Extract categories
	if cats, ok := data["categories"]; ok {
		if catSlice, ok := toStringSlice(cats); ok {
			chartData.Categories = catSlice
		}
	}

	// Extract series
	if series, ok := data["series"]; ok {
		seriesSlice, ok := toSeriesSlice(series)
		if !ok {
			return chartData, fmt.Errorf("invalid series format")
		}

		chartData.Series = make([]ChartSeries, len(seriesSlice))
		for i, s := range seriesSlice {
			cs := ChartSeries{}

			if name, ok := s["name"].(string); ok {
				cs.Name = name
			}

			if values, ok := s["values"]; ok {
				if valSlice, ok := toFloat64Slice(values); ok {
					cs.Values = valSlice
				}
			}

			// Extract time_strings for time-series support
			if timeStrings, ok := s["time_strings"]; ok {
				if tsSlice, ok := toStringSlice(timeStrings); ok {
					cs.TimeStrings = tsSlice
				}
			}

			// Extract time_values (Unix timestamps) for time-series support
			if timeValues, ok := s["time_values"]; ok {
				if tvSlice, ok := toInt64Slice(timeValues); ok {
					cs.TimeValues = tvSlice
				}
			}

			chartData.Series[i] = cs
		}
	}

	return chartData, nil
}

// toInt64Slice attempts to convert an interface{} to []int64.
func toInt64Slice(v any) ([]int64, bool) {
	switch val := v.(type) {
	case []int64:
		return val, true
	case []any:
		result := make([]int64, len(val))
		for i, item := range val {
			switch n := item.(type) {
			case float64:
				result[i] = int64(n)
			case int:
				result[i] = int64(n)
			case int64:
				result[i] = n
			default:
				return nil, false
			}
		}
		return result, true
	case []int:
		result := make([]int64, len(val))
		for i, n := range val {
			result[i] = int64(n)
		}
		return result, true
	case []float64:
		result := make([]int64, len(val))
		for i, n := range val {
			result[i] = int64(n)
		}
		return result, true
	}
	return nil, false
}

// extractPieChartData extracts ChartData for pie/donut charts.
func extractPieChartData(req *RequestEnvelope) (ChartData, error) {
	data := req.Data
	chartData := ChartData{
		Title:    req.Title,
		Footnote: req.Subtitle,
	}

	// Normalize aliases so "labels" maps to "categories".
	normalizeCategoryAliases(data)

	// Extract categories (labels)
	if cats, ok := data["categories"]; ok {
		if catSlice, ok := toStringSlice(cats); ok {
			chartData.Categories = catSlice
		}
	}

	// Extract values
	if values, ok := data["values"]; ok {
		if valSlice, ok := toFloat64Slice(values); ok {
			// Create a single series with the values
			chartData.Series = []ChartSeries{
				{
					Name:   "Values",
					Values: valSlice,
				},
			}
		}
	}

	return chartData, nil
}

// toStringSlice attempts to convert an interface{} to []string.
func toStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []any:
		result := make([]string, len(val))
		for i, item := range val {
			if s, ok := item.(string); ok {
				result[i] = s
			} else {
				return nil, false
			}
		}
		return result, true
	}
	return nil, false
}

// toFloat64Slice attempts to convert an interface{} to []float64.
func toFloat64Slice(v any) ([]float64, bool) {
	switch val := v.(type) {
	case []float64:
		return val, true
	case []any:
		result := make([]float64, len(val))
		for i, item := range val {
			switch n := item.(type) {
			case float64:
				result[i] = n
			case int:
				result[i] = float64(n)
			case int64:
				result[i] = float64(n)
			default:
				return nil, false
			}
		}
		return result, true
	case []int:
		result := make([]float64, len(val))
		for i, n := range val {
			result[i] = float64(n)
		}
		return result, true
	}
	return nil, false
}

// toAnySlice converts various typed slices to []any.
// This is needed because Go's type system treats []map[string]any and []any
// as distinct types that cannot be directly type-asserted.
func toAnySlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []map[string]any:
		result := make([]any, len(s))
		for i, val := range s {
			result[i] = val
		}
		return result, true
	default:
		return nil, false
	}
}

// extractAxisTitle returns the first non-empty string value found in data
// for any of the given keys. This supports multiple naming conventions
// (e.g., "x_label" vs "x_axis_title") for axis titles.
func extractAxisTitle(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// toSeriesSlice attempts to convert an interface{} to []map[string]any.
func toSeriesSlice(v any) ([]map[string]any, bool) {
	switch val := v.(type) {
	case []map[string]any:
		return val, true
	case []any:
		result := make([]map[string]any, len(val))
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				result[i] = m
			} else {
				return nil, false
			}
		}
		return result, true
	}
	return nil, false
}

// =============================================================================
// Render Helper
// =============================================================================

// DrawFunc is a function that draws diagram content onto an SVGBuilder.
// It receives the pre-configured builder and request, and returns an error if drawing fails.
type DrawFunc func(builder *SVGBuilder, req *RequestEnvelope) error

// RenderWithHelper executes the common RenderWithBuilder boilerplate:
// 1. Auto-applies FitMode "contain" for all charts if not set
// 2. Extracts dimensions from request (with defaults)
// 3. Applies FitMode to calculate actual content dimensions
// 4. Creates and configures an SVGBuilder
// 5. Applies style settings from request
// 6. Scales typography for output dimensions
// 7. Calls the provided draw function
// 8. Renders and returns the builder and document
//
// When FitMode is "contain", the content is rendered at dimensions that preserve
// aspect ratio. The returned SVGDocument includes metadata about the content
// dimensions and offset for proper embedding.
//
// This reduces duplication across 20+ diagram types that all follow the same pattern.
func RenderWithHelper(req *RequestEnvelope, draw DrawFunc) (*SVGBuilder, *SVGDocument, error) {
	// Auto-apply FitMode "contain" for all charts to prevent stretching.
	// Every diagram has a natural aspect ratio defined by its render dimensions;
	// stretching to fill an arbitrarily-shaped placeholder distorts the content.
	// "contain" fits the diagram within the placeholder while preserving aspect ratio.
	if req.Output.FitMode == "" {
		req.Output.FitMode = "contain"
	}

	containerW, containerH := getOutputDimensions(req)

	// Apply fit mode to get actual content dimensions
	contentW, contentH, offsetX, offsetY := applyFitMode(req, containerW, containerH)

	// Create builder with content dimensions
	// The content will be properly sized, and the offset is used for positioning
	builder := NewSVGBuilder(contentW, contentH)
	applyStyleToBuilder(builder, req.Style)
	scaleTypographyForBuilder(builder, contentW, contentH, req.Output.Preset)

	if err := draw(builder, req); err != nil {
		return nil, nil, err
	}

	doc, err := builder.Render()
	if err != nil {
		return nil, nil, err
	}

	// Store fit mode metadata in the document for downstream use
	doc.FitMode = req.Output.FitMode
	doc.ContainerWidth = containerW
	doc.ContainerHeight = containerH
	doc.OffsetX = offsetX
	doc.OffsetY = offsetY

	return builder, doc, nil
}

// applyFitMode calculates content dimensions and offset based on fit mode.
// Returns the content dimensions to draw at and the offset to center it within the container.
func applyFitMode(req *RequestEnvelope, containerW, containerH float64) (contentW, contentH, offsetX, offsetY float64) {
	fitMode := req.Output.FitMode
	if fitMode == "" || fitMode == "stretch" {
		// Default: use container dimensions directly
		return containerW, containerH, 0, 0
	}

	// Determine source aspect ratio
	srcW, srcH := getSourceAspectRatio(req)
	if srcW <= 0 || srcH <= 0 {
		// Fallback to container dimensions
		return containerW, containerH, 0, 0
	}

	return FitDimensions(fitMode, srcW, srcH, containerW, containerH)
}

// getSourceAspectRatio determines the natural aspect ratio for the diagram type.
// Uses AspectRatio from output spec if set, otherwise container ratio.
//
// All chart types — including circular ones (pie, donut, radar, gauge) — use the
// container ratio. This means "contain" fit mode is effectively a no-op: the SVG
// canvas fills the entire placeholder, and each chart type handles its circular
// shape internally via min(W,H) inscribed rendering. This avoids the problem where
// forcing a 1:1 canvas shrinks the chart to a small square in wide placeholders,
// wasting significant horizontal space that labels and legends could use.
func getSourceAspectRatio(req *RequestEnvelope) (srcW, srcH float64) {
	// If explicit aspect ratio is set, use it
	if req.Output.AspectRatio > 0 {
		return req.Output.AspectRatio, 1.0
	}

	// All charts use container ratio (effectively no constraint).
	// Circular charts (pie, donut, radar, gauge) inscribe their shapes in
	// min(W,H) internally, and benefit from the extra width for labels/legends.
	return float64(req.Output.Width), float64(req.Output.Height)
}

// RenderWithHelperDimensions is like RenderWithHelper but with custom default dimensions.
// Use this for diagrams that need non-standard defaults (e.g., timeline uses 400 height).
//
// FitMode is applied consistently with RenderWithHelper: the custom default dimensions
// define the diagram's natural aspect ratio, and FitMode "contain" (the default) ensures
// the content is scaled to fit within the container without stretching.
func RenderWithHelperDimensions(req *RequestEnvelope, defaultWidth, defaultHeight float64, draw DrawFunc) (*SVGBuilder, *SVGDocument, error) {
	// Auto-apply FitMode "contain" to prevent stretching in non-matching placeholders,
	// consistent with RenderWithHelper.
	if req.Output.FitMode == "" {
		req.Output.FitMode = "contain"
	}

	// Get the container dimensions (what the placeholder wants).
	// If explicit dimensions are set, those become the container; otherwise use custom defaults.
	containerW, containerH := getOutputDimensionsWithDefaults(req, defaultWidth, defaultHeight)

	// Apply fit mode: use the custom default dimensions as the natural aspect ratio
	// so the diagram preserves its intended proportions within the container.
	contentW, contentH, offsetX, offsetY := applyFitModeWithSource(
		req.Output.FitMode, defaultWidth, defaultHeight, containerW, containerH, req,
	)

	builder := NewSVGBuilder(contentW, contentH)
	applyStyleToBuilder(builder, req.Style)
	scaleTypographyForBuilder(builder, contentW, contentH, req.Output.Preset)

	if err := draw(builder, req); err != nil {
		return nil, nil, err
	}

	doc, err := builder.Render()
	if err != nil {
		return nil, nil, err
	}

	// Store fit mode metadata in the document for downstream use
	doc.FitMode = req.Output.FitMode
	doc.ContainerWidth = containerW
	doc.ContainerHeight = containerH
	doc.OffsetX = offsetX
	doc.OffsetY = offsetY

	return builder, doc, nil
}

// applyFitModeWithSource calculates content dimensions using explicit source dimensions
// as the natural aspect ratio. This is used by RenderWithHelperDimensions where the
// diagram's natural dimensions are known (the custom defaults), rather than being
// inferred from the request type.
func applyFitModeWithSource(fitMode string, srcW, srcH, containerW, containerH float64, req *RequestEnvelope) (contentW, contentH, offsetX, offsetY float64) {
	if fitMode == "" || fitMode == "stretch" {
		return containerW, containerH, 0, 0
	}

	// If explicit aspect ratio is set on the request, use it instead of defaults.
	if req.Output.AspectRatio > 0 {
		srcW = req.Output.AspectRatio
		srcH = 1.0
	}

	if srcW <= 0 || srcH <= 0 {
		return containerW, containerH, 0, 0
	}

	return FitDimensions(fitMode, srcW, srcH, containerW, containerH)
}

// scaleTypographyForBuilder scales the builder's typography for the given output.
// If a known layout preset is provided, uses hand-tuned PresetTypography values
// for deterministic, professionally calibrated font sizes.
// Falls back to ScaleForDimensions (geometric mean with caps) for unknown presets.
func scaleTypographyForBuilder(builder *SVGBuilder, width, height float64, preset string) {
	style := builder.StyleGuide()
	if style == nil || style.Typography == nil {
		return
	}

	// Try preset lookup first — deterministic, hand-tuned values.
	if preset != "" {
		if t := TypographyForPreset(preset); t != nil {
			// Preserve font family and fallbacks from the current style
			// (which may have been customized via StyleSpec).
			t.FontFamily = style.Typography.FontFamily
			t.FallbackFonts = style.Typography.FallbackFonts
			style.Typography = t
			return
		}
	}

	// Fallback: geometric mean scaling with min/max caps.
	style.Typography = style.Typography.ScaleForDimensions(width, height)
}

// getOutputDimensionsWithDefaults extracts dimensions with custom defaults.
func getOutputDimensionsWithDefaults(req *RequestEnvelope, defaultWidth, defaultHeight float64) (width, height float64) {
	width = defaultWidth
	height = defaultHeight

	if req.Output.Width > 0 {
		width = float64(req.Output.Width)
	}
	if req.Output.Height > 0 {
		height = float64(req.Output.Height)
	}

	return width, height
}

// RenderFromBuilder generates an SVGDocument from a RenderWithBuilder implementation.
// This is a simple wrapper that calls RenderWithBuilder and discards the builder.
func RenderFromBuilder(rwb func(*RequestEnvelope) (*SVGBuilder, *SVGDocument, error), req *RequestEnvelope) (*SVGDocument, error) {
	_, doc, err := rwb(req)
	return doc, err
}

// =============================================================================
// Bubble Chart Diagram
// =============================================================================

// BubbleChartDiagram implements the Diagram interface for bubble charts.
type BubbleChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a bubble chart.
func (d *BubbleChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "bubble_chart", false, 1)
}

// Render generates an SVG document for the bubble chart.
func (d *BubbleChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *BubbleChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractBubbleChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultScatterChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		config.ShowLabels = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		config.VariableSize = true

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewScatterChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("bubble_chart render failed: %w", err)
		}
		return nil
	})
}

// extractBubbleChartData extracts ChartData for bubble charts with bubble_values.
//nolint:gocognit // complex chart rendering logic
func extractBubbleChartData(req *RequestEnvelope) (ChartData, error) {
	data := req.Data
	chartData := ChartData{
		Title:    req.Title,
		Footnote: req.Subtitle,
	}

	if series, ok := data["series"]; ok {
		seriesSlice, ok := toSeriesSlice(series)
		if !ok {
			return chartData, fmt.Errorf("invalid series format")
		}

		chartData.Series = make([]ChartSeries, len(seriesSlice))
		for i, s := range seriesSlice {
			cs := ChartSeries{}

			if name, ok := s["name"].(string); ok {
				cs.Name = name
			}

			if values, ok := s["values"]; ok {
				if valSlice, ok := toFloat64Slice(values); ok {
					cs.Values = valSlice
				}
			}

			if xValues, ok := s["x_values"]; ok {
				if xSlice, ok := toFloat64Slice(xValues); ok {
					cs.XValues = xSlice
				}
			}

			if bubbleValues, ok := s["bubble_values"]; ok {
				if bSlice, ok := toFloat64Slice(bubbleValues); ok {
					cs.BubbleValues = bSlice
				}
			}

			// Fallback: series with "points" array of {x, y, size, label} objects
			// (produced by buildStructuredChartData in chart.go for bubble charts).
			// Handle both []any (JSON unmarshal) and []map[string]any (direct construction).
			if len(cs.Values) == 0 {
				parseBubblePoint := func(m map[string]any) {
					x, _ := toFloat64(m["x"])
					y, _ := toFloat64(m["y"])
					size, _ := toFloat64(m["size"])
					cs.XValues = append(cs.XValues, x)
					cs.Values = append(cs.Values, y)
					cs.BubbleValues = append(cs.BubbleValues, size)
					if label, ok := m["label"].(string); ok {
						cs.Labels = append(cs.Labels, label)
					}
				}
				switch pts := s["points"].(type) {
				case []any:
					for _, item := range pts {
						if m, ok := item.(map[string]any); ok {
							parseBubblePoint(m)
						}
					}
				case []map[string]any:
					for _, m := range pts {
						parseBubblePoint(m)
					}
				}
			}

			chartData.Series[i] = cs
		}
		return chartData, nil
	}

	// Fallback: per-category format where each key maps to an array of
	// {x, y, size, label} objects, e.g.:
	//   {"Indonesia": [{"x":48, "y":280, "size":3.2, "label":"..."}], ...}
	// Convert to series format grouped by category name.
	chartData.Series = parseBubblePerCategoryData(data)
	return chartData, nil
}

// parseBubblePerCategoryData converts per-category bubble data to ChartSeries.
// Input format: {"CategoryName": [{"x": float, "y": float, "size": float, "label": string}], ...}
func parseBubblePerCategoryData(data map[string]any) []ChartSeries {
	// Skip known metadata keys
	skip := map[string]bool{
		"title": true, "subtitle": true, "series": true,
		"colors": true, "x_label": true, "y_label": true,
		"x_axis_title": true, "y_axis_title": true,
		"data_order": true,
	}

	var series []ChartSeries
	for key, val := range data {
		if skip[key] {
			continue
		}
		arr, ok := val.([]any)
		if !ok || len(arr) == 0 {
			continue
		}

		cs := ChartSeries{Name: key}
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			x, _ := toFloat64(m["x"])
			y, _ := toFloat64(m["y"])
			size, _ := toFloat64(m["size"])
			cs.XValues = append(cs.XValues, x)
			cs.Values = append(cs.Values, y)
			cs.BubbleValues = append(cs.BubbleValues, size)
			if label, ok := m["label"].(string); ok {
				cs.Labels = append(cs.Labels, label)
			}
		}
		if len(cs.Values) > 0 {
			series = append(series, cs)
		}
	}
	return series
}

// =============================================================================
// Stacked Area Chart Diagram
// =============================================================================

// StackedAreaChartDiagram implements the Diagram interface for stacked area charts.
type StackedAreaChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a stacked area chart.
func (d *StackedAreaChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "stacked_area_chart", true, 1)
}

// Render generates an SVG document for the stacked area chart.
func (d *StackedAreaChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *StackedAreaChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultAreaChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		config.Stacked = true

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewStackedAreaChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("stacked_area_chart render failed: %w", err)
		}
		return nil
	})
}

// =============================================================================
// Grouped Bar Chart Diagram
// =============================================================================

// GroupedBarChartDiagram implements the Diagram interface for grouped bar charts.
type GroupedBarChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a grouped bar chart.
func (d *GroupedBarChartDiagram) Validate(req *RequestEnvelope) error {
	return validateCategoriesAndSeries(req.Data, "grouped_bar_chart", true, 2)
}

// Render generates an SVG document for the grouped bar chart.
func (d *GroupedBarChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *GroupedBarChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultBarChartConfig(width, height)
		config.ShowTitle = req.Title != ""
		// ShowLegend kept at default true; Draw only renders for multi-series.
		config.ShowValues = req.Style.ShowValues
		// ShowGrid defaults to true in DefaultChartConfig for professional dashboards.
		config.Stacked = false // Explicit: side-by-side bars

		// Apply color overrides from data.colors (array of hex strings).
		config.Colors = extractChartColors(req.Data)

		// Extract axis titles from request data.
		config.XAxisTitle = extractAxisTitle(req.Data, "x_label", "x_axis_title")
		config.YAxisTitle = extractAxisTitle(req.Data, "y_label", "y_axis_title")
		if config.XAxisTitle != "" {
			config.MarginBottom += 20
		}
		if config.YAxisTitle != "" {
			config.MarginLeft += 20
		}

		chart := NewBarChart(builder, config)
		if err := chart.Draw(chartData); err != nil {
			return fmt.Errorf("grouped_bar_chart render failed: %w", err)
		}
		return nil
	})
}


// layoutPresetDimensions maps preset names to pixel dimensions.
var layoutPresetDimensions = map[string][2]int{
	"slide_16x9":   {1920, 1080},
	"content_16x9": {1600, 900},
	"half_16x9":    {760, 720},
	"third_16x9":   {500, 720},
	"slide_4x3":    {1024, 768},
	"half_4x3":     {420, 540},
	"square":       {600, 600},
	"thumbnail":    {400, 300},
}

// getPresetDimensions returns the width and height for a named layout preset.
func getPresetDimensions(preset string) (width, height int, ok bool) {
	dims, found := layoutPresetDimensions[preset]
	if !found {
		return 0, 0, false
	}
	return dims[0], dims[1], true
}
