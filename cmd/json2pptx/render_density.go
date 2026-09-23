package main

// clampedRenderDensity parses the MCP numeric density once for every render
// tool. A missing or non-numeric value keeps the tool's documented default.
func clampedRenderDensity(args map[string]any, fallback, minDPI, maxDPI int) int {
	v, ok := args["density"].(float64)
	if !ok {
		return fallback
	}
	density := int(v)
	if density < minDPI {
		return minDPI
	}
	if density > maxDPI {
		return maxDPI
	}
	return density
}
