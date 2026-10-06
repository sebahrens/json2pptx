package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// cycleNodesProbeLabel is a step label of exactly length characters made of
// short words, the way a real label wraps.
func cycleNodesProbeLabel(length int) string {
	return strings.TrimSpace(budgetProbeCopy(length))
}

// cycleNodesBudgetValues is a ring of n steps whose labels and descriptions
// are labelLen and descLen characters long.
func cycleNodesBudgetValues(n, labelLen, descLen int) *patterns.CycleNodesValues {
	v := &patterns.CycleNodesValues{}
	for i := 0; i < n; i++ {
		v.Steps = append(v.Steps, patterns.CycleNodesStep{Label: cycleNodesProbeLabel(labelLen), Description: cycleNodesProbeLabel(descLen)})
	}
	return v
}

// cycleNodesBudgets are the copy budgets pinned per step count: label and
// description characters every step can carry at once, written unshrunk at
// 12pt or above on every shipped template and on the local p-style
// (TestCycleNodesBudgetProbe measures them; abstract's 687 × 294pt body is
// the binding one).
var cycleNodesBudgets = map[int]struct{ label, description int }{
	3: {28, 70},
	4: {28, 70},
	5: {28, 70},
	6: {28, 70},
	7: {22, 40},
	8: {22, 40},
}

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

// TestCycleNodesMatrixAcrossTemplates is the count × template wall: every step count at its
// pinned copy budget renders on every template (the local p-style included)
// with no readability finding, no run written below its role floor and no
// pattern warning.
func TestCycleNodesMatrixAcrossTemplates(t *testing.T) {
	for _, templateName := range schemaMaximaRunTemplateNames(t) {
		geom := loadSchemaMaximaGeometry(t, templateName)
		for n := 3; n <= 8; n++ {
			for _, direction := range []string{"clockwise", "counter_clockwise"} {
				t.Run(fmt.Sprintf("%s/%d/%s", templateName, n, direction), func(t *testing.T) {
					b := cycleNodesBudgets[n]
					values := cycleNodesBudgetValues(n, b.label, b.description)
					values.Highlight = n
					values.Center = &patterns.CycleNodesCenter{Label: "Close loop"}
					encoded, err := json.Marshal(values)
					if err != nil {
						t.Fatal(err)
					}
					overrides := json.RawMessage(fmt.Sprintf(`{"direction":%q}`, direction))
					input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
						SlideType: "content", LayoutID: "blank-title",
						Pattern: &PatternInput{Name: "cycle-nodes", Values: encoded, Overrides: overrides},
					}}}
					for _, f := range collectReadabilityFindings(input, geom.layouts, geom.width, geom.height) {
						t.Errorf("readability finding: %s %s", f.Code, f.Message)
					}
					if n := writtenRoleViolations(t, input, geom); n > 0 {
						t.Errorf("%d runs written below their role floor", n)
					}
					for _, f := range collectFitFindings(input, geom.layouts, geom.width, geom.height, nil) {
						if f.Code == patterns.ErrCodeBodyTooLong || f.Action == "refuse" {
							t.Errorf("fit finding: %s (%s) %s", f.Code, f.Action, f.Message)
						}
					}
				})
			}
		}
	}
}
