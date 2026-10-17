//go:build probe

package main

import (
	"os"

	"testing"
)

// Run with JSON2PPTX_CYCLE_NODES_BUDGET_PROBE=1 to measure the readable
// description budget per step count with every label at its pinned length.
func TestCycleNodesBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CYCLE_NODES_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CYCLE_NODES_BUDGET_PROBE=1")
	}
	for n := 3; n <= 8; n++ {
		label := cycleNodesBudgets[n].label
		budget := probeReadableBudget(t, "cycle-nodes", 70, func(length int) any {
			return cycleNodesBudgetValues(n, label, length)
		})
		short := probeReadableBudget(t, "cycle-nodes", 70, func(length int) any {
			return cycleNodesBudgetValues(n, 12, length)
		})
		t.Logf("steps=%d label=%d description budget=%d (with 12-character labels: %d)", n, label, budget, short)
	}
}
