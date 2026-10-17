//go:build probe

package main

import (
	"os"

	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Budget probe: the longest description (under a 24-character label) that
// stays readable on every template, per layer count. Run with
// JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE=1 when the layout changes and
// update patterns' crDescBudgetByCount and the schema description.
func TestConcentricRingsBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE=1")
	}
	for n := 3; n <= 5; n++ {
		budget := probeReadableBudget(t, "concentric-rings", 70, func(length int) any {
			v := &patterns.ConcentricRingsValues{}
			for i := 0; i < n; i++ {
				v.Layers = append(v.Layers, patterns.ConcentricRingsLayer{Label: budgetProbeCopy(24), Description: budgetProbeCopy(length)})
			}
			return v
		})
		t.Logf("layers=%d description budget=%d", n, budget)
	}
}
