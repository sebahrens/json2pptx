//go:build probe

package patterns

import (
	"fmt"
	"os"

	"testing"
)

// TestCycleIntakeBudgetProbe prints, for every (intake, loop) combination,
// the longest copy each field holds on the smallest content area. Run it with
// CYCLE_INTAKE_PROBE=1 after a layout change and re-pin the budget functions.
func TestCycleIntakeBudgetProbe(t *testing.T) {
	if os.Getenv("CYCLE_INTAKE_PROBE") == "" {
		t.Skip("set CYCLE_INTAKE_PROBE=1 to print the measured budgets")
	}
	worst := func(nI, nL int, mk func(n int) *CycleIntakeValues, max int) int {
		best := 0
		for n := 4; n <= max; n++ {
			ok := true
			for _, body := range cycleRingBodies {
				for _, style := range cycleIntakeStyles {
					if clean, _ := cycleIntakeClean(cycleRingCtx(body.w, body.h), mk(n), &CycleIntakeOverrides{LoopStyle: style}); !clean {
						ok = false
					}
				}
			}
			if !ok {
				break
			}
			best = n
		}
		return best
	}
	for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
		for nL := ringMinItems; nL <= ringMaxItems; nL++ {
			stepLabel := worst(nI, nL, func(n int) *CycleIntakeValues { return cycleIntakeBudgetValues(nI, nL, n, 0, 12, 0) }, cycleIntakeStepLabelMax)
			stepDesc := worst(nI, nL, func(n int) *CycleIntakeValues { return cycleIntakeBudgetValues(nI, nL, stepLabel, n, 12, 0) }, cycleIntakeStepDescMax)
			label := worst(nI, nL, func(n int) *CycleIntakeValues { return cycleIntakeBudgetValues(nI, nL, 12, 0, n, 0) }, cycleIntakeLabelMax)
			line := fmt.Sprintf("intake %d loop %d: intake label %d desc %d | loop label %d, desc", nI, nL, stepLabel, stepDesc, label)
			for _, l := range []int{26, 22, 20, 18} {
				desc := worst(nI, nL, func(n int) *CycleIntakeValues { return cycleIntakeBudgetValues(nI, nL, stepLabel, stepDesc, l, n) }, cycleIntakeDescMax)
				line += fmt.Sprintf(" %d@label%d", desc, l)
			}
			t.Log(line)
		}
	}
}
