package testrand

import "fmt"

// Keep dense fixtures inside each chart's published geometry budget. Edge cases
// exercise zero weights and escaped labels without asking for invalid negatives.
func consultingStressData(typ string, count int, edge bool) map[string]any {
	n := max(2, min(count, 12))
	categories := make([]any, n)
	values := func(base float64) []any {
		v := make([]any, n)
		for i := range v {
			v[i] = base + float64(i)*2
		}
		if edge {
			v[0] = 0.0
		}
		return v
	}
	for i := range categories {
		categories[i] = fmt.Sprintf("C%d", i+1)
	}
	if edge {
		categories[0] = "R&D <"
	}
	series := func(name string, base float64) map[string]any {
		return map[string]any{"name": name, "values": values(base)}
	}
	switch typ {
	case "butterfly_chart":
		return map[string]any{"categories": categories, "left": series("Before", 30), "right": series("After", 50)}
	case "mekko_chart":
		n = min(n, 8)
		partsCount := 2
		if count > 12 {
			partsCount = 6
		}
		segments := make([]any, n)
		for i := range segments {
			parts := make([]any, partsCount)
			for j := range parts {
				value := 20.0
				if edge && j == 0 {
					value = 0
				}
				parts[j] = map[string]any{"name": fmt.Sprintf("P%d", j+1), "value": value}
			}
			segments[i] = map[string]any{"name": fmt.Sprintf("S%d", i+1), "width": 1.0, "parts": parts}
		}
		return map[string]any{"segments": segments}
	default:
		bars := []any{series("B1", 50)}
		lines := []any{series("L1", 20)}
		if count > 12 {
			bars = append(bars, series("B2", 70), series("B3", 90))
			lines = append(lines, series("L2", 70))
		}
		return map[string]any{"categories": categories, "bars": bars, "lines": lines, "bar_unit": "USD", "line_unit": "%", "secondary_axis": true}
	}
}
