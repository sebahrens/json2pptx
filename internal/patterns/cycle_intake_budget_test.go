package patterns

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// cycleIntakeStepCopy is n characters of short words (an intake arrow holds a
// word of about nine letters on one line with three steps).
func cycleIntakeStepCopy(n int) string {
	const words = "Screen the deals and sign the term sheet with our partner banks today "
	return strings.TrimRight(strings.Repeat(words, 2)[:n-1], " ") + "s"
}

// cycleIntakeBudgetValues is nIntake steps and nLoop phases whose labels and
// descriptions are ordinary words of the given lengths (0 = no description).
func cycleIntakeBudgetValues(nIntake, nLoop, stepLabel, stepDesc, label, desc int) *CycleIntakeValues {
	v := &CycleIntakeValues{}
	for i := 0; i < nIntake; i++ {
		s := CycleIntakeStep{Label: cycleIntakeStepCopy(stepLabel)}
		if stepDesc > 0 {
			s.Description = cycleRingCopy(stepDesc)
		}
		v.Intake = append(v.Intake, s)
	}
	for i := 0; i < nLoop; i++ {
		ph := CycleIntakePhase{Label: cycleRingCopy(label)}
		if desc > 0 {
			ph.Description = cycleRingCopy(desc)
		}
		v.Loop = append(v.Loop, ph)
	}
	return v
}

// cycleIntakeClean reports whether the layout holds every text at 12pt or
// above in a row as tall as it needs, with nothing left off.
func cycleIntakeClean(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides) (bool, string) {
	lay, err := cycleIntakeMeasure(ctx, v, ovr)
	if err != nil {
		return false, err.Error()
	}
	switch {
	case lay.stacked:
		return false, "stacked"
	case len(lay.unfit) > 0:
		return false, fmt.Sprintf("intake label breaks a word: %v", lay.unfit)
	case !lay.list.showDesc:
		return false, "loop descriptions left off"
	case lay.list.labelPt < 12 || lay.list.descPt < 12 || lay.fit.labelPt < 12 || lay.descPt < 12:
		return false, "text under 12pt"
	}
	for i, need := range lay.descNeed {
		if need > lay.descRoom+0.01 {
			return false, fmt.Sprintf("intake[%d].description needs %.0fpt of %.0fpt", i, need, lay.descRoom)
		}
	}
	for i, row := range lay.list.rows {
		if row.capped || row.y1-row.y0 < row.need-0.01 {
			return false, fmt.Sprintf("loop[%d] row is %.1fpt but needs %.1fpt", i, row.y1-row.y0, row.need)
		}
	}
	return true, ""
}

// The documented budgets per (intake, loop) combination. Text inside them is
// laid out side by side at 12pt or above with every row as tall as its text
// needs, on the smallest shipped content area (abstract, 687 x 294pt) and the
// larger ones, in both loop styles.
func TestCycleIntakeMeasuredBudgets(t *testing.T) {
	p := &cycleIntake{}
	for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
		for nL := ringMinItems; nL <= ringMaxItems; nL++ {
			stepLabel, stepDesc := cycleIntakeStepLabelMax, cycleIntakeStepDescMax
			label, desc := cycleIntakeLabelBudget(nI, nL), cycleIntakeDescBudget(nI, nL)
			for _, body := range cycleRingBodies {
				for _, style := range cycleIntakeStyles {
					t.Run(fmt.Sprintf("%d+%d/%s/%s", nI, nL, body.name, style), func(t *testing.T) {
						ctx := cycleRingCtx(body.w, body.h)
						ovr := &CycleIntakeOverrides{LoopStyle: style}
						v := cycleIntakeBudgetValues(nI, nL, stepLabel, stepDesc, label, desc)
						if ok, why := cycleIntakeClean(ctx, v, ovr); !ok {
							t.Errorf("at budget (intake %d/%d, loop %d/%d): %s", stepLabel, stepDesc, label, desc, why)
						}
						if w := p.PostExpandWarnings(ctx, v, ovr); len(w) != 0 {
							t.Errorf("at budget: %v", w)
						}
					})
				}
			}
		}
	}
}

// The budgets pinned for the docs (docs/PATTERNS.md): loop label /
// description by (intake, loop). The intake's own budgets are its schema
// maxima at every count.
func TestCycleIntakeBudgetTable(t *testing.T) {
	want := map[int][cycleIntakeMaxSteps][2]int{ // loop count -> by intake count 1, 2, 3
		3: {{26, 70}, {26, 70}, {26, 70}},
		4: {{26, 70}, {26, 70}, {22, 65}},
		5: {{26, 70}, {26, 60}, {22, 45}},
		6: {{26, 40}, {26, 30}, {22, 22}},
		7: {{26, 40}, {26, 30}, {22, 22}},
		8: {{26, 0}, {26, 0}, {22, 0}},
	}
	for nL, row := range want {
		for i, w := range row {
			if got := [2]int{cycleIntakeLabelBudget(i+1, nL), cycleIntakeDescBudget(i+1, nL)}; got != w {
				t.Errorf("loop budgets(%d intake, %d loop) = %v, want %v", i+1, nL, got, w)
			}
		}
	}
}

// Eight phases have no static description budget: the list holds one line
// under each label only where the area is tall enough, and says so where it is
// not.
func TestCycleIntakeEightPhaseDescriptionsNeedHeight(t *testing.T) {
	p := &cycleIntake{}
	v := cycleIntakeBudgetValues(3, 8, 12, 0, 16, 20)
	lay, err := cycleIntakeMeasure(cycleRingCtx(687, 294), v, &CycleIntakeOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if lay.list.showDesc {
		t.Fatal("abstract: eight list rows with descriptions should not fit 294pt")
	}
	for i, row := range lay.list.rows {
		if row.capped {
			t.Errorf("abstract: loop[%d] label row is capped once the descriptions are left off", i)
		}
	}
	w := p.PostExpandWarnings(cycleRingCtx(687, 294), v, nil)
	if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong) || !strings.Contains(w[0], "loop[].description is left off") {
		t.Errorf("abstract warnings = %v", w)
	}
	for _, body := range cycleRingBodies[1:] {
		if ok, why := cycleIntakeClean(cycleRingCtx(body.w, body.h), v, &CycleIntakeOverrides{}); !ok {
			t.Errorf("%s: one-line descriptions with 8 phases: %s", body.name, why)
		}
	}
}

// One character over a budget reports BODY_TOO_LONG naming the item and the
// budget of its combination.
func TestCycleIntakeWarningsNameTheBindingContent(t *testing.T) {
	p := &cycleIntake{}
	ctx := cycleRingCtx(828, 349)

	v := cycleIntakeBudgetValues(3, 5, 12, 0, 12, 45)
	if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
		t.Fatalf("at the description budget warned: %v", w)
	}
	v.Loop[4].Description = cycleRingCopy(46)
	w := p.PostExpandWarnings(ctx, v, nil)
	if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong) ||
		!strings.Contains(w[0], "loop[4].description is 46 characters; with 3 intake steps and 5 phases use about 45") {
		t.Errorf("loop description warning = %v", w)
	}

	v = cycleIntakeBudgetValues(3, 7, 12, 0, 23, 0)
	w = p.PostExpandWarnings(ctx, v, nil)
	if len(w) != 7 || !strings.Contains(w[0], "loop[0].label is 23 characters; with 3 intake steps and 7 phases use about 22") {
		t.Errorf("loop label warnings = %v", w)
	}

	// An intake word its arrow cannot hold on one line is named, with the
	// letters that do fit.
	v = cycleIntakeBudgetValues(3, 4, 12, 0, 12, 0)
	v.Intake[1].Label = "Prequalification"
	w = p.PostExpandWarnings(cycleRingCtx(687, 294), v, nil)
	if len(w) != 1 || !strings.Contains(w[0], "intake[1].label has a word wider than its") || !strings.Contains(w[0], "letters") {
		t.Errorf("intake word warning = %v", w)
	}
}

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
