package patterns

import (
	"fmt"
	"strings"
	"testing"
)

// cycleRingCopy is n characters of ordinary words (no trailing space).
func cycleRingCopy(n int) string {
	const words = "Customer onboarding reduces churn across enterprise accounts while regional teams standardise the weekly cadence "
	return strings.TrimRight(strings.Repeat(words, 2)[:n-1], " ") + "s"
}

func budgetCycleRing(n, labelLen, descLen int) *CycleRingValues {
	v := &CycleRingValues{}
	for i := 0; i < n; i++ {
		ph := CycleRingPhase{Label: cycleRingCopy(labelLen)}
		if descLen > 0 {
			ph.Description = cycleRingCopy(descLen)
		}
		v.Phases = append(v.Phases, ph)
	}
	return v
}

// The documented budgets per phase count. Text inside them is laid out at
// 12pt or above with every label row as tall as its text needs, on the
// smallest shipped content area (abstract, 687 x 294pt) and the larger ones.
func TestCycleRingMeasuredBudgets(t *testing.T) {
	want := map[int][2]int{4: {28, 90}, 5: {28, 70}, 6: {28, 70}, 7: {24, 50}, 8: {24, 50}}
	p := &cycleRing{}
	for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
		if got := [2]int{cycleRingLabelBudget(n), cycleRingDescBudget(n)}; got != want[n] {
			t.Errorf("budgets(%d) = %v, want %v", n, got, want[n])
		}
		for _, body := range cycleRingBodies {
			t.Run(fmt.Sprintf("%d/%s", n, body.name), func(t *testing.T) {
				ctx := cycleRingCtx(body.w, body.h)
				v := budgetCycleRing(n, want[n][0], want[n][1])
				if l := runeLen(v.Phases[0].Label); l != want[n][0] {
					t.Fatalf("probe label is %d characters, want %d", l, want[n][0])
				}
				lay, err := cycleRingMeasure(ctx, v, &CycleRingOverrides{})
				if err != nil {
					t.Fatal(err)
				}
				if lay.legend || !lay.showDesc || lay.labelPt < 12 || lay.descPt < 12 {
					t.Errorf("at budget: legend=%v showDesc=%v label %.0fpt description %.0fpt", lay.legend, lay.showDesc, lay.labelPt, lay.descPt)
				}
				for i, row := range lay.rows {
					if row.capped || row.y1-row.y0 < row.need-0.01 {
						t.Errorf("at budget: phases[%d] row is %.1fpt tall but needs %.1fpt", i, row.y1-row.y0, row.need)
					}
				}
				if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
					t.Errorf("at budget: %v", w)
				}
			})
		}
	}
}

// One character over a budget reports BODY_TOO_LONG naming the phase and the
// budget; the schema maximum is the 4-phase budget, so only 5-8 phases can
// be over a description budget and only 7-8 over a label budget.
func TestCycleRingWarningsNameTheBindingContent(t *testing.T) {
	p := &cycleRing{}
	for n := 5; n <= cycleRingMaxPhases; n++ {
		budget := cycleRingDescBudget(n)
		v := budgetCycleRing(n, 12, budget)
		if w := p.PostExpandWarnings(ExpandContext{}, v, nil); len(w) != 0 {
			t.Fatalf("n=%d at the description budget warned: %v", n, w)
		}
		v.Phases[n-2].Description = cycleRingCopy(budget + 1)
		w := p.PostExpandWarnings(ExpandContext{}, v, nil)
		want := fmt.Sprintf("phases[%d].description is %d characters; with %d phases use about %d", n-2, budget+1, n, budget)
		if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong) || !strings.Contains(w[0], want) {
			t.Errorf("n=%d description warning = %v, want %q", n, w, want)
		}
	}
	for n := 7; n <= cycleRingMaxPhases; n++ {
		v := budgetCycleRing(n, 24, 0)
		if w := p.PostExpandWarnings(ExpandContext{}, v, nil); len(w) != 0 {
			t.Fatalf("n=%d at the label budget warned: %v", n, w)
		}
		v.Phases[0].Label = cycleRingCopy(25)
		w := p.PostExpandWarnings(ExpandContext{}, v, nil)
		want := fmt.Sprintf("phases[0].label is 25 characters; with %d phases use about 24", n)
		if len(w) != 1 || !strings.Contains(w[0], want) {
			t.Errorf("n=%d label warning = %v, want %q", n, w, want)
		}
	}
	// 4 phases: the schema maxima are the budget.
	if w := p.PostExpandWarnings(cycleRingCtx(687, 294), budgetCycleRing(4, cycleRingLabelMax, cycleRingDescMax), nil); len(w) != 0 {
		t.Errorf("4 phases at the schema maxima on the smallest body warned: %v", w)
	}
}
