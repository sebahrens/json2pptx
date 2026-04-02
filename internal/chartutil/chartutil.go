// Package chartutil provides shared chart/diagram YAML parsing utilities
// used by both the old parser and the AST lowering pipeline.
package chartutil

import (
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/safeyaml"
	"github.com/sebahrens/json2pptx/internal/types"
)

// NormalizeDiagramData converts common YAML shorthand formats into the canonical
// format expected by svggen renderers.  This bridges the gap between what users
// naturally write in markdown diagram blocks and what the renderers validate.
func NormalizeDiagramData(diagramType string, data map[string]any) map[string]any {
	switch diagramType {
	case "pyramid":
		return normalizePyramid(data)
	case "heatmap":
		return normalizeHeatmap(data)
	case "venn":
		return normalizeVenn(data)
	case "org_chart":
		return normalizeOrgChart(data)
	default:
		return data
	}
}

// normalizePyramid converts string-only levels to {label: ...} objects.
//
// Shorthand: levels: ["Top", "Mid", "Base"]
// Canonical: levels: [{label: "Top"}, {label: "Mid"}, {label: "Base"}]
func normalizePyramid(data map[string]any) map[string]any {
	levelsRaw, ok := data["levels"]
	if !ok {
		return data
	}
	levels, ok := toAnySliceUtil(levelsRaw)
	if !ok || len(levels) == 0 {
		return data
	}

	// Check if first element is already an object
	if _, isMap := levels[0].(map[string]any); isMap {
		return data // already canonical
	}

	normalized := make([]any, 0, len(levels))
	for _, l := range levels {
		switch v := l.(type) {
		case string:
			normalized = append(normalized, map[string]any{"label": v})
		case map[string]any:
			normalized = append(normalized, v) // already object
		}
	}
	out := copyMap(data)
	out["levels"] = normalized
	return out
}

// normalizeHeatmap converts rows-of-objects + columns into values 2D array + labels.
//
// Shorthand:
//
//	rows:
//	  - label: "Row1"
//	    values: [1, 2, 3]
//	columns: ["A", "B", "C"]
//
// Canonical:
//
//	values: [[1,2,3]]
//	row_labels: ["Row1"]
//	col_labels: ["A","B","C"]
func normalizeHeatmap(data map[string]any) map[string]any {
	// If "values" already present as a nested array, assume canonical
	if valRaw, ok := data["values"]; ok {
		if arr, ok := toAnySliceUtil(valRaw); ok && len(arr) > 0 {
			if _, isSlice := toAnySliceUtil(arr[0]); isSlice {
				// Already a 2D array — just copy "columns" → "col_labels" alias
				out := copyMap(data)
				if cols, ok := data["columns"]; ok {
					if _, hasColLabels := out["col_labels"]; !hasColLabels {
						out["col_labels"] = cols
					}
					delete(out, "columns")
				}
				return out
			}
		}
	}

	rowsRaw, ok := data["rows"]
	if !ok {
		return data
	}
	rows, ok := toAnySliceUtil(rowsRaw)
	if !ok || len(rows) == 0 {
		return data
	}

	// Check if rows are objects with label+values (shorthand) vs plain arrays (canonical)
	firstRow, isMap := rows[0].(map[string]any)
	if !isMap {
		return data // rows are probably already number arrays
	}
	if _, hasLabel := firstRow["label"]; !hasLabel {
		return data
	}

	out := copyMap(data)
	var rowLabels []any
	var values []any
	for _, rRaw := range rows {
		r, ok := rRaw.(map[string]any)
		if !ok {
			continue
		}
		if label, ok := r["label"].(string); ok {
			rowLabels = append(rowLabels, label)
		}
		if vals, ok := r["values"]; ok {
			values = append(values, vals)
		}
	}
	out["values"] = values
	out["row_labels"] = rowLabels
	delete(out, "rows")

	// Rename "columns" to "col_labels"
	if cols, ok := data["columns"]; ok {
		out["col_labels"] = cols
		delete(out, "columns")
	}

	return out
}

// normalizeVenn renames "sets" → "circles" and converts array intersections to map.
//
// Shorthand:
//
//	sets: [{label: "A"}, {label: "B"}]
//	intersections:
//	  - sets: ["A", "B"]
//	    items: ["shared"]
//
// Canonical:
//
//	circles: [{label: "A"}, {label: "B"}]
//	intersections:
//	  ab: {items: ["shared"]}
func normalizeVenn(data map[string]any) map[string]any {
	out := copyMap(data)

	// Rename "sets" to "circles"
	if sets, ok := out["sets"]; ok {
		if _, hasCircles := out["circles"]; !hasCircles {
			out["circles"] = sets
		}
		delete(out, "sets")
	}

	// Convert array intersections to map form
	if intersRaw, ok := out["intersections"]; ok {
		if intersArr, ok := toAnySliceUtil(intersRaw); ok {
			out["intersections"] = normalizeVennIntersections(intersArr, out)
		}
		// If already a map, leave as is
	}

	return out
}

// normalizeVennIntersections converts an array of {sets, items, label} objects
// into the map[string]VennRegion format (keyed by "ab", "ac", "bc", "abc").
func normalizeVennIntersections(arr []any, data map[string]any) map[string]any { //nolint:gocognit,gocyclo
	// Build label → index map from circles
	labelToIdx := map[string]int{}
	if circlesRaw, ok := data["circles"]; ok {
		if circles, ok := toAnySliceUtil(circlesRaw); ok {
			for i, cRaw := range circles {
				if c, ok := cRaw.(map[string]any); ok {
					if label, ok := c["label"].(string); ok {
						labelToIdx[label] = i
					}
				}
			}
		}
	}

	// Map index pairs to keys: {0,1}→"ab", {0,2}→"ac", {1,2}→"bc", {0,1,2}→"abc"
	indexToKey := map[string]string{
		"01":  "ab",
		"02":  "ac",
		"12":  "bc",
		"012": "abc",
	}

	result := map[string]any{}
	for _, iRaw := range arr {
		inter, ok := iRaw.(map[string]any)
		if !ok {
			continue
		}

		// Determine the key from the "sets" array
		setsRaw, ok := inter["sets"]
		if !ok {
			continue
		}
		sets, ok := toAnySliceUtil(setsRaw)
		if !ok {
			continue
		}

		// Resolve set labels to indices
		indices := ""
		for _, sRaw := range sets {
			if s, ok := sRaw.(string); ok {
				if idx, found := labelToIdx[s]; found {
					indices += string(rune('0' + idx))
				}
			}
		}

		key, found := indexToKey[indices]
		if !found {
			continue
		}

		// Build the region object
		region := map[string]any{}
		if label, ok := inter["label"].(string); ok {
			region["label"] = label
		}
		if items, ok := inter["items"]; ok {
			region["items"] = items
		}
		result[key] = region
	}

	return result
}

// normalizeOrgChart wraps top-level {name, title, children} in a "root" key.
//
// Shorthand: data: {name: "CEO", title: "...", children: [...]}
// Canonical: data: {root: {name: "CEO", title: "...", children: [...]}}
func normalizeOrgChart(data map[string]any) map[string]any {
	if _, hasRoot := data["root"]; hasRoot {
		return data // already canonical
	}

	// Check if top-level has org-chart-like fields
	_, hasName := data["name"]
	_, hasChildren := data["children"]
	if !hasName && !hasChildren {
		return data
	}

	// Extract org node fields from top level, wrap in "root"
	root := map[string]any{}
	orgFields := []string{"name", "title", "children", "color", "style"}
	out := copyMap(data)
	for _, field := range orgFields {
		if v, ok := out[field]; ok {
			root[field] = v
			delete(out, field)
		}
	}
	out["root"] = root
	return out
}

// copyMap returns a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// toAnySliceUtil converts various slice types to []any (utility version for chartutil).
func toAnySliceUtil(v interface{}) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []map[string]any:
		r := make([]any, len(s))
		for i, m := range s {
			r[i] = m
		}
		return r, true
	case []string:
		r := make([]any, len(s))
		for i, str := range s {
			r[i] = str
		}
		return r, true
	}
	return nil, false
}

// MapChartTypeToSvggen maps user-facing chart type names to svggen diagram types.
func MapChartTypeToSvggen(chartType string) string {
	switch chartType {
	case "bar":
		return "bar_chart"
	case "line":
		return "line_chart"
	case "pie":
		return "pie_chart"
	case "donut":
		return "donut_chart"
	case "area":
		return "area_chart"
	case "radar":
		return "radar_chart"
	case "scatter":
		return "scatter_chart"
	case "stacked_bar":
		return "stacked_bar_chart"
	case "waterfall":
		return "waterfall"
	case "funnel":
		return "funnel_chart"
	case "gauge":
		return "gauge_chart"
	case "treemap":
		return "treemap_chart"
	case "bubble":
		return "bubble_chart"
	case "stacked_area":
		return "stacked_area_chart"
	case "grouped_bar":
		return "grouped_bar_chart"
	default:
		return chartType
	}
}

// BuildChartDataPayload constructs the data payload for svggen from parsed chart YAML.
// yamlContent is the raw YAML string used to preserve source key order for map-format data.
func BuildChartDataPayload(chartData map[string]interface{}, yamlContent string) map[string]any {
	chartType, _ := chartData["type"].(string)

	// Handle multi-series radar/chart format:
	//   data:
	//     - series: "Name"
	//       values:
	//         - category: "Cat1"
	//           value: 85
	if chartType == "radar" {
		if payload := tryParseMultiSeriesChart(chartData["data"]); payload != nil {
			return payload
		}
	}

	// Handle scatter/bubble array-of-points format:
	//   data:
	//     - label: "Product A"
	//       x: 50
	//       y: 200
	if chartType == "scatter" || chartType == "bubble" {
		if payload := tryParseScatterPointArray(chartData["data"]); payload != nil {
			copyAxisTitles(payload, chartData)
			return payload
		}
	}

	keys, values := extractChartKeysValues(chartData["data"], yamlContent)
	if chartType == "waterfall" {
		types := extractWaterfallTypes(chartData["data"])
		return map[string]any{"points": buildWaterfallPoints(keys, values, types)}
	}
	payload := formatChartPayload(chartType, keys, values)
	// Copy axis title keys (y_label, x_label, etc.) from the root chart YAML
	// into the payload so that bar/line/area charts can display axis titles.
	copyAxisTitles(payload, chartData)
	return payload
}

// axisTitleKeys are the chart YAML keys that specify axis labels.
var axisTitleKeys = []string{"x_label", "y_label", "x_axis_title", "y_axis_title"}

// copyAxisTitles copies axis title keys from src into dst (if present).
func copyAxisTitles(dst, src map[string]any) {
	for _, k := range axisTitleKeys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

// tryParseMultiSeriesChart handles the multi-series format where data is an array
// of {series: "name", values: [{category, value}, ...]}.
func tryParseMultiSeriesChart(rawData interface{}) map[string]any {
	arr, ok := rawData.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}

	// Check if first item has "series" and "values" keys
	firstItem, ok := arr[0].(map[string]interface{})
	if !ok {
		return nil
	}
	if _, hasSeries := firstItem["series"]; !hasSeries {
		return nil
	}
	if _, hasValues := firstItem["values"]; !hasValues {
		return nil
	}

	var categories []string
	var series []map[string]any

	for _, itemRaw := range arr {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		seriesName, _ := item["series"].(string)
		valuesRaw, ok := item["values"].([]interface{})
		if !ok {
			continue
		}

		var vals []float64
		for i, vRaw := range valuesRaw {
			vMap, ok := vRaw.(map[string]interface{})
			if !ok {
				continue
			}
			cat, _ := vMap["category"].(string)
			val := ToFloat64(vMap["value"])
			if val == nil {
				continue
			}
			vals = append(vals, *val)
			// Use first series to establish categories
			if len(series) == 0 && i == len(categories) {
				categories = append(categories, cat)
			}
		}

		series = append(series, map[string]any{
			"name":   seriesName,
			"values": vals,
		})
	}

	if len(categories) < 3 || len(series) == 0 {
		return nil
	}

	return map[string]any{
		"categories": categories,
		"series":     series,
	}
}

// extractChartKeysValues extracts keys and values from chart data in map or array format.
func extractChartKeysValues(rawData interface{}, yamlContent string) ([]string, []float64) {
	var keys []string
	var values []float64

	switch data := rawData.(type) {
	case map[string]interface{}:
		orderedKeys := safeyaml.ExtractMapKeyOrder(yamlContent, "data")
		if len(orderedKeys) > 0 {
			for _, k := range orderedKeys {
				if numVal := ToFloat64(data[k]); numVal != nil {
					keys = append(keys, k)
					values = append(values, *numVal)
				}
			}
		} else {
			for key, val := range data {
				if numVal := ToFloat64(val); numVal != nil {
					keys = append(keys, key)
					values = append(values, *numVal)
				}
			}
			slices.Sort(keys)
			values = make([]float64, len(keys))
			for i, k := range keys {
				if numVal := ToFloat64(data[k]); numVal != nil {
					values[i] = *numVal
				}
			}
		}
	case []interface{}:
		for _, item := range data {
			if itemMap, ok := item.(map[string]interface{}); ok {
				label, hasLabel := itemMap["label"].(string)
				if !hasLabel {
					continue
				}
				if numVal := ToFloat64(itemMap["value"]); numVal != nil {
					keys = append(keys, label)
					values = append(values, *numVal)
				}
			}
		}
	}

	return keys, values
}

func formatChartPayload(chartType string, keys []string, values []float64) map[string]any {
	switch chartType {
	case "pie", "donut":
		return map[string]any{"categories": keys, "values": values}
	case "gauge":
		gaugeValue := 0.0
		if len(values) > 0 {
			gaugeValue = values[0]
		}
		return map[string]any{"value": gaugeValue, "min": 0.0, "max": 100.0}
	case "funnel":
		return map[string]any{"values": buildLabelValuePoints(keys, values)}
	case "waterfall":
		return map[string]any{"points": buildWaterfallPoints(keys, values, nil)}
	case "treemap":
		return map[string]any{"values": buildLabelValuePoints(keys, values)}
	case "scatter":
		return buildScatterPayload(values)
	default:
		return map[string]any{
			"categories": keys,
			"series":     []map[string]any{{"name": "Data", "values": values}},
		}
	}
}

func buildLabelValuePoints(keys []string, values []float64) []map[string]any {
	points := make([]map[string]any, len(keys))
	for i, k := range keys {
		points[i] = map[string]any{"label": k, "value": values[i]}
	}
	return points
}

func extractWaterfallTypes(rawData interface{}) []string {
	arr, ok := rawData.([]interface{})
	if !ok {
		return nil
	}
	types := make([]string, len(arr))
	for i, item := range arr {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if t, ok := itemMap["type"].(string); ok {
				types[i] = t
			}
		}
	}
	return types
}

// InferWaterfallType infers the waterfall bar type from label name and value.
// Labels containing "subtotal" or "sub-total" → subtotal; "total" → total;
// otherwise positive → increase, negative → decrease.
func InferWaterfallType(label string, value float64) string {
	lower := strings.ToLower(label)
	if strings.Contains(lower, "subtotal") || strings.Contains(lower, "sub-total") {
		return "subtotal"
	}
	if strings.Contains(lower, "total") {
		return "total"
	}
	if value < 0 {
		return "decrease"
	}
	return "increase"
}

func buildWaterfallPoints(keys []string, values []float64, types []string) []map[string]any {
	points := make([]map[string]any, len(keys))
	for i, k := range keys {
		pointType := ""
		if i < len(types) {
			pointType = types[i]
		}
		if pointType == "" {
			pointType = InferWaterfallType(k, values[i])
		}
		points[i] = map[string]any{"label": k, "value": values[i], "type": pointType}
	}
	return points
}

// tryParseScatterPointArray handles scatter/bubble data as an array of point objects:
//
//	data:
//	  - label: "Product A"
//	    x: 50
//	    y: 200
//
// Returns nil if data is not in this format.
func tryParseScatterPointArray(rawData interface{}) map[string]any {
	arr, ok := rawData.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}

	// Verify first item has x and y keys
	firstItem, ok := arr[0].(map[string]interface{})
	if !ok {
		return nil
	}
	if _, hasX := firstItem["x"]; !hasX {
		return nil
	}
	if _, hasY := firstItem["y"]; !hasY {
		return nil
	}

	points := make([]map[string]any, 0, len(arr))
	for _, itemRaw := range arr {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		xVal := ToFloat64(item["x"])
		yVal := ToFloat64(item["y"])
		if xVal == nil || yVal == nil {
			continue
		}
		pt := map[string]any{"x": *xVal, "y": *yVal}
		if label, ok := item["label"].(string); ok {
			pt["label"] = label
		}
		if size := ToFloat64(item["size"]); size != nil {
			pt["size"] = *size
		}
		points = append(points, pt)
	}

	if len(points) == 0 {
		return nil
	}

	return map[string]any{
		"series": []map[string]any{{"name": "Data", "points": points}},
	}
}

func buildScatterPayload(values []float64) map[string]any {
	points := make([]map[string]any, len(values))
	for i, v := range values {
		points[i] = map[string]any{"x": float64(i + 1), "y": v}
	}
	return map[string]any{
		"series": []map[string]any{{"name": "Data", "points": points}},
	}
}

// ParseChartStyle extracts style settings from parsed chart YAML.
func ParseChartStyle(styleData map[string]interface{}) *types.DiagramStyle {
	style := &types.DiagramStyle{}
	if colors, ok := styleData["colors"].([]interface{}); ok {
		style.Colors = make([]string, len(colors))
		for i, c := range colors {
			if s, ok := c.(string); ok {
				style.Colors[i] = s
			}
		}
	}
	if fontFamily, ok := styleData["font_family"].(string); ok {
		style.FontFamily = fontFamily
	}
	if showLegend, ok := styleData["show_legend"].(bool); ok {
		style.ShowLegend = showLegend
	}
	if showValues, ok := styleData["show_values"].(bool); ok {
		style.ShowValues = showValues
	}
	if background, ok := styleData["background"].(string); ok {
		style.Background = background
	}
	return style
}

// ToFloat64 converts various numeric types to float64.
// Returns nil if the value is not a numeric type.
func ToFloat64(val interface{}) *float64 {
	var result float64
	switch v := val.(type) {
	case int:
		result = float64(v)
	case int64:
		result = float64(v)
	case float64:
		result = v
	case float32:
		result = float64(v)
	default:
		return nil
	}
	return &result
}

// NormalizeYAMLIndentation removes excessive leading whitespace from YAML content.
// This is necessary because YAML extracted from HTML comments may have extra
// indentation from the markdown source.
func NormalizeYAMLIndentation(yaml string) string {
	lines := strings.Split(yaml, "\n")
	if len(lines) == 0 {
		return yaml
	}

	indents := calculateLineIndents(lines)
	firstNonEmptyIdx := findFirstNonEmptyLine(indents)
	minIndent := findMinimumIndent(indents, firstNonEmptyIdx)

	if minIndent <= 0 {
		return yaml
	}

	return applyIndentNormalization(lines, indents, firstNonEmptyIdx, minIndent)
}

func calculateLineIndents(lines []string) []int {
	indents := make([]int, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			indents[i] = -1
			continue
		}
		count := 0
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				count++
			} else {
				break
			}
		}
		indents[i] = count
	}
	return indents
}

func findFirstNonEmptyLine(indents []int) int {
	for i, indent := range indents {
		if indent >= 0 {
			return i
		}
	}
	return -1
}

func findMinimumIndent(indents []int, firstNonEmptyIdx int) int {
	minIndent := -1
	for i, indent := range indents {
		if indent < 0 {
			continue
		}
		if i == firstNonEmptyIdx && indent == 0 && len(indents) > 1 {
			continue
		}
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	return minIndent
}

func applyIndentNormalization(lines []string, indents []int, firstNonEmptyIdx, minIndent int) string {
	normalized := make([]string, len(lines))
	for i, line := range lines {
		indent := indents[i]
		if indent < 0 {
			normalized[i] = ""
		} else if i == firstNonEmptyIdx && indent == 0 {
			normalized[i] = line
		} else if len(line) > minIndent {
			normalized[i] = line[minIndent:]
		} else {
			normalized[i] = strings.TrimLeft(line, " \t")
		}
	}
	return strings.Join(normalized, "\n")
}
