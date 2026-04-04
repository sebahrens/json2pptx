// Clamp utilities have been moved to core/clamp.go.
// This file provides unexported aliases used by existing code in the package.
package svggen

import (
	"math"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// maxSafeValue mirrors core's unexported constant for package-internal use.
const maxSafeValue = 1e15

// clampDataValues is a package-local alias for core.ClampDataValues.
func clampDataValues(data map[string]any) {
	core.ClampDataValues(data)
}

// clampFloat64 clamps a float64 to [-maxSafeValue, maxSafeValue].
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
