package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
)

// ChartSpec defines a chart to be rendered.
//
// Deprecated: Use DiagramSpec instead. ChartSpec requires explicit type mapping.
// DiagramSpec is more flexible and passes data directly to svggen.
type ChartSpec struct {
	Type         ChartType          `json:"type"`                    // Chart type (bar, line, pie, donut)
	Title        string             `json:"title,omitempty"`         // Chart title
	Data         map[string]any     `json:"data"`                    // Label to value mapping (flexible: float64 for simple charts, arrays/objects for structured charts)
	DataOrder    []string           `json:"data_order,omitempty"`    // Preserved input order of data keys
	Width        int                `json:"width,omitempty"`         // Width in pixels (default: 800)
	Height       int                `json:"height,omitempty"`        // Height in pixels (default: 600)
	Scale        float64            `json:"scale,omitempty"`         // Resolution scale (default: calculated dynamically, min 2.0)
	Style        *ChartStyle        `json:"style,omitempty"`         // Optional styling overrides
	OutputFormat ChartOutputFormat  `json:"output_format,omitempty"` // Output format: png (default) or svg

	// SeriesLabels provides series names for multi-series chart types
	// (stacked_bar, grouped_bar, stacked_area). When Data values are arrays,
	// SeriesLabels[i] names the i-th element in each array.
	SeriesLabels []string `json:"series_labels,omitempty"`

	// TimeData provides time-series data support for line charts.
	// Keys are ISO8601 date/time strings or Unix timestamps.
	// Takes precedence over Data when set.
	TimeData map[string]any `json:"time_data,omitempty"`

	// TimeOrder preserves the input order of time keys (optional).
	TimeOrder []string `json:"time_order,omitempty"`
}

// UnmarshalJSON handles the case where "data" is either a map (normal) or an array
// (scatter/bubble point format like [{"label":"A","x":50,"y":200}]).
// When data is a map and data_order is not provided, JSON key order is preserved
// so that line/area charts render categories in the author's intended order.
func (cs *ChartSpec) UnmarshalJSON(b []byte) error {
	// Always use RawMessage for data so we can extract key order.
	type chartSpecRawData struct {
		Type         ChartType         `json:"type"`
		Title        string            `json:"title,omitempty"`
		Data         json.RawMessage   `json:"data"`
		DataOrder    []string          `json:"data_order,omitempty"`
		Width        int               `json:"width,omitempty"`
		Height       int               `json:"height,omitempty"`
		Scale        float64           `json:"scale,omitempty"`
		Style        *ChartStyle       `json:"style,omitempty"`
		OutputFormat ChartOutputFormat `json:"output_format,omitempty"`
		SeriesLabels []string          `json:"series_labels,omitempty"`
		TimeData     map[string]any    `json:"time_data,omitempty"`
		TimeOrder    []string          `json:"time_order,omitempty"`
	}

	var raw chartSpecRawData
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	cs.Type = raw.Type
	cs.Title = raw.Title
	cs.DataOrder = raw.DataOrder
	cs.Width = raw.Width
	cs.Height = raw.Height
	cs.Scale = raw.Scale
	cs.Style = raw.Style
	cs.OutputFormat = raw.OutputFormat
	cs.SeriesLabels = raw.SeriesLabels
	cs.TimeData = raw.TimeData
	cs.TimeOrder = raw.TimeOrder

	if len(raw.Data) == 0 {
		return nil
	}

	// Try parsing data as array of point objects.
	var dataArr []map[string]any
	if err := json.Unmarshal(raw.Data, &dataArr); err == nil && len(dataArr) > 0 {
		// Convert array of {label, x, y} to series format.
		cs.Data = convertPointArrayToSeriesData(dataArr)
		return nil
	}

	// Parse data as map.
	var dataMap map[string]any
	if err := json.Unmarshal(raw.Data, &dataMap); err != nil {
		// Data is neither map nor array — leave it nil.
		return nil
	}
	cs.Data = dataMap

	// Preserve JSON key order when data_order is not explicitly provided.
	// Go maps lose insertion order, so we extract it from the raw JSON tokens.
	if len(cs.DataOrder) == 0 {
		cs.DataOrder = extractJSONKeyOrder(raw.Data)
	}

	return nil
}

// convertPointArrayToSeriesData converts [{label, x, y}, ...] to {"series": [{name, points}]}.
func convertPointArrayToSeriesData(arr []map[string]any) map[string]any {
	points := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		pt := make(map[string]any)
		if x, ok := item["x"]; ok {
			pt["x"] = x
		}
		if y, ok := item["y"]; ok {
			pt["y"] = y
		}
		if label, ok := item["label"]; ok {
			pt["label"] = label
		}
		if size, ok := item["size"]; ok {
			pt["size"] = size
		}
		points = append(points, pt)
	}
	return map[string]any{
		"series": []map[string]any{{"name": "Data", "points": points}},
	}
}

// IsTimeSeries returns true if this chart spec contains time-series data.
func (cs ChartSpec) IsTimeSeries() bool {
	return len(cs.TimeData) > 0
}

// ChartOutputFormat specifies the output format for chart rendering.
type ChartOutputFormat string

const (
	// ChartFormatPNG renders the chart as a PNG image (default).
	ChartFormatPNG ChartOutputFormat = "png"

	// ChartFormatSVG renders the chart as an SVG vector graphic.
	// SVG provides better scaling fidelity and smaller file size for simple charts.
	// When embedded in PPTX, SVG is converted based on the configured SVG strategy.
	ChartFormatSVG ChartOutputFormat = "svg"
)

// ChartType represents the type of chart to render.
type ChartType string

const (
	ChartBar        ChartType = "bar"         // Vertical bar chart
	ChartLine       ChartType = "line"        // Line chart with points
	ChartPie        ChartType = "pie"         // Pie chart
	ChartDonut      ChartType = "donut"       // Donut chart (pie with hole)
	ChartFunnel     ChartType = "funnel"      // Funnel chart
	ChartGauge      ChartType = "gauge"       // Gauge/speedometer chart
	ChartTreemap    ChartType = "treemap"     // Treemap chart
	ChartWaterfall  ChartType = "waterfall"   // Waterfall/bridge chart for financial flows
	ChartArea       ChartType = "area"        // Area chart (filled line chart)
	ChartRadar      ChartType = "radar"       // Radar/spider chart for multi-dimensional comparison
	ChartScatter     ChartType = "scatter"      // Scatter plot for X-Y data
	ChartStackedBar  ChartType = "stacked_bar"  // Stacked bar chart for part-to-whole comparisons
	ChartBubble      ChartType = "bubble"       // Bubble chart (scatter with size dimension)
	ChartStackedArea ChartType = "stacked_area" // Stacked area chart for cumulative trends
	ChartGroupedBar  ChartType = "grouped_bar"  // Grouped bar chart for side-by-side comparisons
)

// ChartStyle provides styling options for chart rendering.
type ChartStyle struct {
	Colors      []string     `json:"colors,omitempty"`      // Hex colors for data series (overrides ThemeColors)
	ThemeColors []ThemeColor `json:"-"`                     // Theme colors from template (internal use)
	FontFamily  string       `json:"font_family,omitempty"` // Font for labels and text
	FontSize    int          `json:"font_size,omitempty"`   // Label font size in points
	Background  string       `json:"background,omitempty"`  // Background color (default: transparent)
	ShowLegend  bool         `json:"show_legend,omitempty"` // Display legend
	ShowValues  bool         `json:"show_values,omitempty"` // Display values on chart elements
}

// DefaultChartDimensions provides default chart dimensions.
const (
	DefaultChartWidth  = 800
	DefaultChartHeight = 600
)

// DefaultMinScale is the minimum scale factor for raster output.
const DefaultMinScale = 2.0

// TargetDPI is the target DPI for high-quality rendering.
const TargetDPI = 150.0

// CalculateDynamicScale computes the optimal resolution scale factor based on
// placeholder dimensions to ensure high-quality rendering at any size.
//
// The scale is calculated to ensure the final rendered image has at least
// TargetDPI (150 DPI) when displayed at the placeholder's physical size.
// The formula is:
//
//	requiredPixels = placeholderWidthInches × TargetDPI
//	scale = requiredPixels / outputWidth
//
// The returned scale is always at least DefaultMinScale (2.0) to ensure
// basic quality for smaller placeholders.
//
// Parameters:
//   - placeholderWidthEMU: The placeholder width in EMUs (914400 per inch)
//   - outputWidth: The base output width in pixels before scaling
//
// Example:
//
//	// For a 5-inch wide placeholder with 800px base output:
//	// requiredPixels = 5.0 × 150 = 750px
//	// scale = 750 / 800 = 0.9375 → clamped to 2.0 (minimum)
//
//	// For a 12-inch wide placeholder with 800px base output:
//	// requiredPixels = 12.0 × 150 = 1800px
//	// scale = 1800 / 800 = 2.25
func CalculateDynamicScale(placeholderWidthEMU EMU, outputWidth int) float64 {
	if outputWidth <= 0 || placeholderWidthEMU <= 0 {
		return DefaultMinScale
	}

	// Convert placeholder width from EMU to inches
	placeholderWidthInches := placeholderWidthEMU.Inches()

	// Calculate required pixels for target DPI
	requiredPixels := placeholderWidthInches * TargetDPI

	// Calculate scale factor
	scale := requiredPixels / float64(outputWidth)

	// Ensure minimum scale for quality
	if scale < DefaultMinScale {
		scale = DefaultMinScale
	}

	return scale
}

// FitMode controls how content is scaled to fit within the output dimensions.
type FitMode string

const (
	// FitStretch fills the container, possibly distorting aspect ratio.
	// This is the default behavior for backwards compatibility.
	FitStretch FitMode = "stretch"

	// FitContain fits the content inside the container while preserving aspect ratio.
	// May result in letterboxing (empty space on sides or top/bottom).
	FitContain FitMode = "contain"

	// FitCover fills the container while preserving aspect ratio.
	// Content may be cropped if aspect ratios don't match.
	FitCover FitMode = "cover"
)

// LayoutPreset is a named preset for chart/diagram sizing.
// These presets provide convenient sizing for common slide layouts.
type LayoutPreset string

const (
	// PresetSlide16x9 is the full slide area for 16:9 presentations.
	PresetSlide16x9 LayoutPreset = "slide_16x9"
	// PresetContent16x9 is the content area (minus margins) for 16:9 slides.
	PresetContent16x9 LayoutPreset = "content_16x9"
	// PresetHalf16x9 is half the content area width for 16:9 slides.
	PresetHalf16x9 LayoutPreset = "half_16x9"
	// PresetThird16x9 is one-third the content area width for 16:9 slides.
	PresetThird16x9 LayoutPreset = "third_16x9"
	// PresetSlide4x3 is the full slide area for 4:3 presentations.
	PresetSlide4x3 LayoutPreset = "slide_4x3"
	// PresetHalf4x3 is half the content area width for 4:3 slides.
	PresetHalf4x3 LayoutPreset = "half_4x3"
	// PresetSquare is a square aspect ratio suitable for centered diagrams.
	PresetSquare LayoutPreset = "square"
	// PresetThumbnail is a small preview size.
	PresetThumbnail LayoutPreset = "thumbnail"
)

// LayoutPresetDimensions defines the dimensions for each layout preset.
type LayoutPresetDimensions struct {
	Width  int
	Height int
}

// LayoutPresets maps preset names to their dimensions in pixels.
var LayoutPresets = map[LayoutPreset]LayoutPresetDimensions{
	PresetSlide16x9:   {Width: 1920, Height: 1080},
	PresetContent16x9: {Width: 1600, Height: 900},
	PresetHalf16x9:    {Width: 760, Height: 720},
	PresetThird16x9:   {Width: 500, Height: 720},
	PresetSlide4x3:    {Width: 1024, Height: 768},
	PresetHalf4x3:     {Width: 420, Height: 540},
	PresetSquare:      {Width: 600, Height: 600},
	PresetThumbnail:   {Width: 400, Height: 300},
}

// GetPresetDimensions returns the width and height for a named preset.
// Returns (0, 0, false) if the preset is not found.
func GetPresetDimensions(preset string) (width, height int, ok bool) {
	dims, found := LayoutPresets[LayoutPreset(preset)]
	if !found {
		return 0, 0, false
	}
	return dims.Width, dims.Height, true
}

// Standard slide dimensions in EMUs.
const (
	// StandardSlideWidth16x9 is a 16:9 slide width (10 inches in EMUs).
	StandardSlideWidth16x9 EMU = 9144000
	// StandardSlideWidth4x3 is a 4:3 slide width (10 inches in EMUs).
	StandardSlideWidth4x3 EMU = 9144000
)

// DetectLayoutPreset determines the appropriate layout preset based on
// placeholder width relative to standard slide dimensions.
//
// This function enables automatic preset selection without manual specification,
// analyzing the placeholder's width as a ratio of the slide width:
//   - ~100% (>80%) → PresetContent16x9 (1600x900 - full content area)
//   - ~50% (35%-80%) → PresetHalf16x9 (760x720 - half width)
//   - ~33% (<35%) → PresetThird16x9 (500x720 - third width)
//
// For 4:3 slides, corresponding 4:3 presets are returned.
//
// Example usage:
//
//	preset := DetectLayoutPreset(placeholderWidth, slideWidth)
//	dims := LayoutPresets[preset]
func DetectLayoutPreset(placeholderWidth, slideWidth EMU) LayoutPreset {
	if slideWidth <= 0 {
		return PresetContent16x9 // Default fallback
	}

	ratio := float64(placeholderWidth) / float64(slideWidth)

	// Without slide height, we cannot distinguish 4:3 from 16:9
	// (both standard formats use 10" = 9144000 EMU width).
	// Use DetectLayoutPresetWithDimensions for 4:3 support.
	const (
		fullWidthThreshold = 0.80 // >80% is full content area
		halfWidthThreshold = 0.35 // 35%-80% is half width
	)

	switch {
	case ratio >= fullWidthThreshold:
		return PresetContent16x9
	case ratio >= halfWidthThreshold:
		return PresetHalf16x9
	default:
		return PresetThird16x9
	}
}

// isNear4x3Ratio checks if slide dimensions indicate a 4:3 aspect ratio.
// It computes width/height and checks if it's close to 4/3 (1.333...) within tolerance.
func isNear4x3Ratio(slideWidth, slideHeight EMU) bool {
	if slideWidth <= 0 || slideHeight <= 0 {
		return false
	}
	ratio := float64(slideWidth) / float64(slideHeight)
	// 4:3 = 1.333..., tolerance of 5%
	const target = 4.0 / 3.0
	const tolerance = 0.05
	return ratio >= target*(1-tolerance) && ratio <= target*(1+tolerance)
}

// DetectLayoutPresetWithDimensions determines the appropriate layout preset
// based on placeholder dimensions (width and height) and slide dimensions.
// This variant considers aspect ratio for better circular chart handling
// and can detect 4:3 slides (which DetectLayoutPreset cannot).
func DetectLayoutPresetWithDimensions(placeholderWidth, placeholderHeight, slideWidth, slideHeight EMU) LayoutPreset {
	if slideWidth <= 0 {
		return PresetContent16x9
	}

	// Check for square placeholder (good for circular charts like pie/donut)
	if placeholderWidth > 0 && placeholderHeight > 0 {
		aspectRatio := float64(placeholderWidth) / float64(placeholderHeight)
		if aspectRatio >= 0.9 && aspectRatio <= 1.1 {
			return PresetSquare
		}
	}

	ratio := float64(placeholderWidth) / float64(slideWidth)
	is4x3 := isNear4x3Ratio(slideWidth, slideHeight)

	const (
		fullWidthThreshold = 0.80
		halfWidthThreshold = 0.35
	)

	switch {
	case ratio >= fullWidthThreshold:
		if is4x3 {
			return PresetSlide4x3
		}
		return PresetContent16x9
	case ratio >= halfWidthThreshold:
		if is4x3 {
			return PresetHalf4x3
		}
		return PresetHalf16x9
	default:
		return PresetThird16x9
	}
}

// chartTypeToSvggenType maps ChartType to svggen diagram type strings.
var chartTypeToSvggenType = map[ChartType]string{
	ChartBar:        "bar_chart",
	ChartLine:       "line_chart",
	ChartPie:        "pie_chart",
	ChartDonut:      "donut_chart",
	ChartArea:       "area_chart",
	ChartRadar:      "radar_chart",
	ChartScatter:    "scatter_chart",
	ChartStackedBar:  "stacked_bar_chart",
	ChartWaterfall:   "waterfall",
	ChartFunnel:      "funnel_chart",
	ChartGauge:       "gauge_chart",
	ChartTreemap:     "treemap_chart",
	ChartBubble:      "bubble_chart",
	ChartStackedArea: "stacked_area_chart",
	ChartGroupedBar:  "grouped_bar_chart",
}

// ToDiagramSpec converts a ChartSpec to a DiagramSpec.
// This enables unified rendering through the DiagramSpec code path.
func (cs *ChartSpec) ToDiagramSpec() *DiagramSpec {
	if cs == nil {
		return nil
	}

	// Map chart type to svggen type.
	// All standard chart types (bar→bar_chart, line→line_chart, etc.) are in the map.
	// Diagram types (pyramid, swot, timeline, etc.) use their raw name as the svggen
	// type ID, so the fallback passes through the type string unchanged.
	svggenType, ok := chartTypeToSvggenType[cs.Type]
	if !ok {
		svggenType = string(cs.Type)
	}

	// Build data payload based on chart type
	data, warnings := buildChartData(cs)

	// Convert style
	var style *DiagramStyle
	if cs.Style != nil {
		style = &DiagramStyle{
			Colors:      cs.Style.Colors,
			ThemeColors: cs.Style.ThemeColors,
			FontFamily:  cs.Style.FontFamily,
			ShowLegend:  cs.Style.ShowLegend,
			ShowValues:  cs.Style.ShowValues,
			Background:  cs.Style.Background,
		}
	}

	// Width/Height: pass through as-is (0 means "auto-detect from placeholder").
	return &DiagramSpec{
		Type:     svggenType,
		Title:    cs.Title,
		Data:     data,
		Width:    cs.Width,
		Height:   cs.Height,
		Scale:    cs.Scale,
		Style:    style,
		Warnings: warnings,
	}
}

// toFloat64 safely extracts a float64 from an any value.
// JSON numbers unmarshal as float64 by default; this also handles int types.
// toFloat64 converts any value to float64.
// Non-numeric values (strings, bools, nil) are coerced to 0 with a warning.
func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		if v != nil {
			slog.Warn("non-numeric chart value coerced to zero", "value", v, "type", fmt.Sprintf("%T", v))
		}
		return 0
	}
}

// hasStructuredValues returns true if any value in the data map is an actual
// structured data container (map or slice), indicating multi-series chart data.
// Non-numeric scalars (string, bool, nil) are NOT considered structured — they
// are invalid scalars that toFloat64() will coerce to 0.
func hasStructuredValues(data map[string]any) bool {
	for _, v := range data {
		switch v.(type) {
		case map[string]any, []any:
			return true
		}
	}
	return false
}

// isAlreadySvggenFormat returns true if the data map already contains keys that
// indicate svggen-native format. When true, the data should be passed through
// to svggen without transformation by buildChartData.
//
// Detection is type-aware to avoid false positives (e.g., a flat data map with
// a "value" label key should NOT be treated as gauge format).
func isAlreadySvggenFormat(data map[string]any, chartType ChartType) bool {
	if len(data) == 0 {
		return false
	}

	// categories+series (or labels+series) is the native format for bar, line, area, stacked, grouped charts.
	_, hasCats := data["categories"]
	_, hasLabels := data["labels"]
	_, hasSeries := data["series"]
	if (hasCats || hasLabels) && hasSeries {
		return true
	}

	// series-only format is native for bubble and scatter charts.
	if hasSeries && !hasCats {
		switch chartType {
		case ChartBubble, ChartScatter:
			return true
		}
	}

	// gauge: value + min or max
	if _, hasVal := data["value"]; hasVal {
		_, hasMin := data["min"]
		_, hasMax := data["max"]
		if hasMin || hasMax {
			return true
		}
	}

	// waterfall: points array
	if _, ok := data["points"]; ok {
		if chartType == ChartWaterfall {
			return true
		}
	}

	// funnel: stages array
	if _, ok := data["stages"]; ok {
		if chartType == ChartFunnel {
			return true
		}
	}

	// treemap: items array
	if _, ok := data["items"]; ok {
		if chartType == ChartTreemap {
			return true
		}
	}

	// pyramid: levels array
	if _, ok := data["levels"]; ok {
		return true
	}

	// org_chart: root tree or flat nodes array
	if chartType == "org_chart" {
		if _, ok := data["root"]; ok {
			return true
		}
		if _, ok := data["nodes"]; ok {
			return true
		}
	}

	return false
}

// buildChartData constructs the data payload for svggen based on chart type.
// It returns the data map and any warnings generated during conversion
// (e.g., when flat-map data is auto-converted to a structured format).
func buildChartData(spec *ChartSpec) (map[string]any, []string) {
	// TimeData takes precedence over Data when set (for time-series charts).
	data := spec.Data
	order := spec.DataOrder
	if len(spec.TimeData) > 0 && len(data) == 0 {
		data = spec.TimeData
		if len(order) == 0 {
			order = spec.TimeOrder
		}
	}

	// If data is already in svggen-native format (e.g., has categories+series,
	// value+min+max, points, stages, levels, etc.), pass it through directly.
	// This avoids mangling data that LLMs produce in the correct output format.
	if isAlreadySvggenFormat(data, spec.Type) {
		return data, nil
	}

	// Use order if available, otherwise sort keys for determinism
	var keys []string
	if len(order) > 0 {
		keys = order
	} else {
		keys = make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		// Sort for deterministic output
		sortStrings(keys)
	}

	// Check if we have structured (array/object) values in data.
	// Multi-series chart types (stacked_bar, grouped_bar, stacked_area, scatter, bubble)
	// may receive data where each key maps to an array of values rather than a single float64.
	if hasStructuredValues(data) {
		return buildStructuredChartData(spec, data, keys), nil
	}

	// Build values array from flat numeric data
	values := make([]float64, len(keys))
	for i, k := range keys {
		values[i] = toFloat64(data[k])
	}

	// Diagram types that need special data formats when coming through ChartSpec.
	// These convert flat numeric data into the structured format each diagram expects.
	switch spec.Type {
	case "pyramid":
		levels := make([]map[string]any, len(keys))
		for i, k := range keys {
			levels[i] = map[string]any{"label": k}
		}
		return map[string]any{
			"levels": levels,
		}, []string{fmt.Sprintf("%s chart received flat data; expected {levels: [{label: ...}]} format", spec.Type)}

	case ChartPie, ChartDonut:
		return map[string]any{
			"categories": keys,
			"values":     values,
		}, nil

	case ChartGauge:
		gaugeValue := 0.0
		if len(values) > 0 {
			gaugeValue = values[0]
		}
		return map[string]any{
			"value": gaugeValue,
			"min":   0.0,
			"max":   100.0,
		}, []string{fmt.Sprintf("%s chart received flat data; expected {value: N, min: N, max: N} format", spec.Type)}

	case ChartFunnel:
		points := make([]map[string]any, len(keys))
		for i, k := range keys {
			points[i] = map[string]any{
				"label": k,
				"value": values[i],
			}
		}
		return map[string]any{
			"values": points,
		}, []string{fmt.Sprintf("%s chart received flat data; expected {values: [{label: ..., value: N}]} format", spec.Type)}

	case ChartWaterfall:
		points := make([]map[string]any, len(keys))
		for i, k := range keys {
			pointType := "increase"
			if values[i] < 0 {
				pointType = "decrease"
			}
			points[i] = map[string]any{
				"label": k,
				"value": values[i],
				"type":  pointType,
			}
		}
		return map[string]any{
			"points": points,
		}, []string{fmt.Sprintf("%s chart received flat data; expected {points: [{label: ..., value: N, type: \"increase\"|\"decrease\"|\"total\"}]} format", spec.Type)}

	case ChartTreemap:
		nodes := make([]map[string]any, len(keys))
		for i, k := range keys {
			nodes[i] = map[string]any{
				"label": k,
				"value": values[i],
			}
		}
		return map[string]any{
			"values": nodes,
		}, []string{fmt.Sprintf("%s chart received flat data; expected {values: [{label: ..., value: N}]} format", spec.Type)}

	case ChartScatter:
		points := make([]map[string]any, len(values))
		for i, v := range values {
			points[i] = map[string]any{
				"x": float64(i + 1),
				"y": v,
			}
		}
		result := map[string]any{
			"series": []map[string]any{
				{
					"name":   "Data",
					"points": points,
				},
			},
		}
		copyAxisTitles(result, data)
		return result, []string{fmt.Sprintf("%s chart received flat data; expected {series: [{name: ..., points: [{x: N, y: N}]}]} format", spec.Type)}

	default:
		// Bar/line/area/radar/stacked_bar expect categories + series format
		result := map[string]any{
			"categories": keys,
			"series": []map[string]any{
				{
					"name":   "Data",
					"values": values,
				},
			},
		}
		copyAxisTitles(result, data)
		return result, nil
	}
}

// buildStructuredChartData handles chart data where values are arrays or objects
// (e.g., stacked_bar with {"Q1": [10, 20, 30], "Q2": [15, 25, 35]}).
func buildStructuredChartData(spec *ChartSpec, data map[string]any, keys []string) map[string]any { //nolint:gocognit,gocyclo
	switch spec.Type {
	case ChartStackedBar, ChartGroupedBar, ChartStackedArea:
		// Data format: {"Q1": [10, 20, 30], "Q2": [15, 25, 35]}
		// with series_labels: ["A", "B", "C"]
		// Convert to: { categories: ["Q1", "Q2"], series: [{name: "A", values: [10, 15]}, ...] }

		// Determine the number of series from the first array value
		numSeries := 0
		for _, k := range keys {
			if arr, ok := data[k].([]any); ok {
				numSeries = len(arr)
				break
			}
		}

		// Build series arrays
		seriesValues := make([][]float64, numSeries)
		for i := range seriesValues {
			seriesValues[i] = make([]float64, len(keys))
		}

		for catIdx, k := range keys {
			switch v := data[k].(type) {
			case []any:
				for serIdx := 0; serIdx < numSeries && serIdx < len(v); serIdx++ {
					seriesValues[serIdx][catIdx] = toFloat64(v[serIdx])
				}
			case float64:
				// Mixed: some categories have arrays, some have scalars
				if numSeries > 0 {
					seriesValues[0][catIdx] = v
				}
			}
		}

		// Build series with names from SeriesLabels
		series := make([]map[string]any, numSeries)
		for i := 0; i < numSeries; i++ {
			name := ""
			if i < len(spec.SeriesLabels) {
				name = spec.SeriesLabels[i]
			} else {
				name = fmt.Sprintf("Series %d", i+1)
			}
			series[i] = map[string]any{
				"name":   name,
				"values": seriesValues[i],
			}
		}

		result := map[string]any{
			"categories": keys,
			"series":     series,
		}
		copyAxisTitles(result, data)
		return result

	case ChartScatter:
		// Data format: {"Series A": [{"x": 1, "y": 2}, ...], "Series B": [...]}
		// Convert to: { series: [{name: "Series A", points: [...]}, ...] }
		seriesList := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			if isAxisTitleKey(k) {
				continue
			}
			points := make([]map[string]any, 0)
			if arr, ok := data[k].([]any); ok {
				for _, item := range arr {
					if pt, ok := item.(map[string]any); ok {
						points = append(points, pt)
					}
				}
			}
			seriesList = append(seriesList, map[string]any{
				"name":   k,
				"points": points,
			})
		}
		result := map[string]any{
			"series": seriesList,
		}
		copyAxisTitles(result, data)
		return result

	case ChartBubble:
		// Data format: {"Series A": [{"x": 1, "y": 2, "size": 10}, ...]}
		// Convert to: { series: [{name: "Series A", points: [...]}, ...] }
		seriesList := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			if isAxisTitleKey(k) {
				continue
			}
			points := make([]map[string]any, 0)
			if arr, ok := data[k].([]any); ok {
				for _, item := range arr {
					if pt, ok := item.(map[string]any); ok {
						points = append(points, pt)
					}
				}
			}
			seriesList = append(seriesList, map[string]any{
				"name":   k,
				"points": points,
			})
		}
		result := map[string]any{
			"series": seriesList,
		}
		copyAxisTitles(result, data)
		return result

	default:
		// For other chart types receiving structured data, pass through the data
		// directly. This provides a fallback so unknown structured data doesn't crash.
		return data
	}
}

// axisTitleKeys are the data-map keys that svggen uses for axis labels.
// These must survive any data-format transformation so that scatter/bubble
// charts can display axis titles (e.g., "Effort" on X, "Impact" on Y).
var axisTitleKeys = []string{"x_label", "y_label", "x_axis_title", "y_axis_title"}

// copyAxisTitles copies axis title keys from src into dst (if present).
func copyAxisTitles(dst, src map[string]any) {
	for _, k := range axisTitleKeys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

// isAxisTitleKey returns true if key is an axis title metadata key.
func isAxisTitleKey(key string) bool {
	for _, k := range axisTitleKeys {
		if key == k {
			return true
		}
	}
	return false
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// extractJSONKeyOrder extracts the top-level key order from a JSON object.
// Returns nil if the input is not a valid JSON object.
func extractJSONKeyOrder(raw json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Read opening brace
	t, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil
	}

	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil
		}
		key, ok := t.(string)
		if !ok {
			return nil
		}
		keys = append(keys, key)
		// Skip the value (could be any JSON value)
		var discard json.RawMessage
		if err := dec.Decode(&discard); err != nil {
			return nil
		}
	}
	return keys
}
