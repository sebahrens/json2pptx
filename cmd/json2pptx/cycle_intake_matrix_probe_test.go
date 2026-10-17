//go:build probe

package main

import (
	"os"

	"testing"
)

// Run with JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE=1 to measure, per combination,
// the readable loop description budget with every other field at its pinned
// length, through the fit-report collector on every template.
func TestCycleIntakeBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CYCLE_INTAKE_BUDGET_PROBE=1")
	}
	for nI := 1; nI <= 3; nI++ {
		for nL := 3; nL <= 8; nL++ {
			label := cycleIntakeBudgets[[2]int{nI, nL}].label
			budget := probeReadableBudget(t, "cycle-intake", 70, func(length int) any {
				return cycleIntakeBudgetValues(nI, nL, label, length)
			})
			t.Logf("intake=%d loop=%d label=%d description budget=%d (pinned %d)", nI, nL, label, budget, cycleIntakeBudgets[[2]int{nI, nL}].description)
		}
	}
}
