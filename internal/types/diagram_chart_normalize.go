package types

import "encoding/json"

// svggenTypeToChartType is the reverse of chartTypeToSvggenType so a
// DiagramSpec that names either form ("bar" or "bar_chart") resolves to the
// ChartType buildChartData understands.
var svggenTypeToChartType = func() map[string]ChartType {
	m := make(map[string]ChartType, len(chartTypeToSvggenType))
	for ct, sv := range chartTypeToSvggenType {
		m[sv] = ct
	}
	return m
}()

// chartTypeForDiagramType maps a DiagramSpec type (short ChartType name or
// svggen type ID) to its ChartType. ok is false for non-chart diagram types.
func chartTypeForDiagramType(t string) (ChartType, bool) {
	if _, ok := chartTypeToSvggenType[ChartType(t)]; ok {
		return ChartType(t), true
	}
	ct, ok := svggenTypeToChartType[t]
	return ct, ok
}

// isFlatNumericMap reports whether every value in data is a JSON number —
// the {label: value} shorthand chart_value accepts (examples/charts.json).
func isFlatNumericMap(data map[string]any) bool {
	if len(data) == 0 {
		return false
	}
	for _, v := range data {
		switch v.(type) {
		case float64, int, int64, int32, json.Number:
		default:
			return false
		}
	}
	return true
}

// NormalizeFlatChartData converts the {label: value} shorthand on a chart
// DiagramSpec (e.g. {"Q1": 12, "Q2": 14}) into svggen's native
// categories/series payload, using the same conversion chart_value content
// items get via ChartSpec.ToDiagramSpec. keyOrder preserves the author's
// category order (pass nil to fall back to sorted keys).
//
// Only flat all-numeric maps on chart types are rewritten; native svggen
// payloads (categories+series, value+min+max, ...) and non-chart diagrams are
// left untouched. Returns true when the spec was rewritten.
func (ds *DiagramSpec) NormalizeFlatChartData(keyOrder []string) bool {
	if ds == nil || !isFlatNumericMap(ds.Data) {
		return false
	}
	ct, ok := chartTypeForDiagramType(ds.Type)
	if !ok || isAlreadySvggenFormat(ds.Data, ct) {
		return false
	}
	order := make([]string, 0, len(keyOrder))
	for _, k := range keyOrder {
		if _, present := ds.Data[k]; present {
			order = append(order, k)
		}
	}
	if len(order) != len(ds.Data) {
		order = nil // incomplete order: let buildChartData sort deterministically
	}
	data, warnings, diags := buildChartData(&ChartSpec{Type: ct, Data: ds.Data, DataOrder: order}) //nolint:staticcheck // ChartSpec conversion is the shared flat-map path
	ds.Data = data
	ds.Type = chartTypeToSvggenType[ct]
	ds.Warnings = append(ds.Warnings, warnings...)
	ds.ChartDiagnostics = append(ds.ChartDiagnostics, diags...)
	return true
}

// JSONObjectKeyOrder returns the top-level key order of a JSON object, or nil
// when raw is not an object. Go maps drop insertion order, so callers that
// decode chart data into map[string]any use this to keep category order.
func JSONObjectKeyOrder(raw json.RawMessage) []string {
	return extractJSONKeyOrder(raw)
}
