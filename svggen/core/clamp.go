package core

import "math"

// maxSafeValue is the maximum absolute value allowed for float64 data values.
// This is far beyond any real-world use case (1 quadrillion) but safely within
// the range where float64 arithmetic won't overflow to Inf or produce NaN.
const maxSafeValue = 1e15

// ClampDataValues recursively walks a data map and clamps any float64 values
// to the range [-maxSafeValue, maxSafeValue]. This prevents downstream
// arithmetic (scaling, coordinate computation) from producing NaN or Inf,
// which would crash the canvas rasterizer or produce corrupt SVG paths.
//
// The function modifies the map in place for efficiency.
func ClampDataValues(data map[string]any) {
	for k, v := range data {
		data[k] = clampValue(v)
	}
}

// clampValue clamps a single value, recursing into maps and slices.
func clampValue(v any) any {
	switch val := v.(type) {
	case float64:
		return clampFloat64(val)
	case map[string]any:
		for k, inner := range val {
			val[k] = clampValue(inner)
		}
		return val
	case []any:
		for i, inner := range val {
			val[i] = clampValue(inner)
		}
		return val
	default:
		return v
	}
}

// clampFloat64 clamps a float64 to [-maxSafeValue, maxSafeValue].
// NaN and Inf values are clamped to 0 and the boundary respectively.
func clampFloat64(f float64) float64 {
	if math.IsNaN(f) {
		return 0
	}
	if math.IsInf(f, 1) {
		return maxSafeValue
	}
	if math.IsInf(f, -1) {
		return -maxSafeValue
	}
	if f > maxSafeValue {
		return maxSafeValue
	}
	if f < -maxSafeValue {
		return -maxSafeValue
	}
	return f
}
