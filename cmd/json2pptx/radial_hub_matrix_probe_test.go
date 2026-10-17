//go:build probe

package main

import (
	_ "image/png"

	"os"

	"testing"
)

// Run with JSON2PPTX_RADIAL_HUB_BUDGET_PROBE=1 to measure the readable
// description budget per count (every label at its documented maximum).
func TestRadialHubBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_RADIAL_HUB_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_RADIAL_HUB_BUDGET_PROBE=1")
	}
	for n := 4; n <= 8; n++ {
		label := 26
		if n > 6 {
			label = 22
		}
		budget := probeReadableBudget(t, "radial-hub", 60, func(length int) any { return radialHubValues(n, label, length) })
		t.Logf("spokes=%d label=%d description budget=%d", n, label, budget)
	}
}
