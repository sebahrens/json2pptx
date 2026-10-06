package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func budgetCFE(n, left, labelLen, descLen int) *CycleFigureEightValues {
	v := &CycleFigureEightValues{LeftCount: left}
	for i := 0; i < n; i++ {
		ph := CycleRingPhase{Label: cycleRingCopy(labelLen)}
		if descLen > 0 {
			ph.Description = cycleRingCopy(descLen)
		}
		v.Phases = append(v.Phases, ph)
	}
	return v
}

// cfeBusiest is the phase count of the fuller lobe.
func cfeBusiest(n, left int) int {
	k := cfeLeftCount(n, left)
	return max(k, n-k)
}

// The documented budgets: labels at the schema maximum at every count,
// descriptions 60 while each lobe holds at most three phases and 40 once a
// lobe holds four. Text inside them is laid out at 12pt or above with every
// label row as tall as its text needs, on the smallest shipped content area
// (abstract, 687 x 294pt) and the larger ones, for every split.
func TestCycleFigureEightMeasuredBudgets(t *testing.T) {
	wantDesc := map[int]int{2: 60, 3: 60, 4: 40}
	p := &cycleFigureEight{}
	for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
		for _, k := range cfeSplits(n) {
			busiest := cfeBusiest(n, k)
			if got := cfeDescBudget(busiest); got != wantDesc[busiest] {
				t.Errorf("description budget with %d phases on a lobe = %d, want %d", busiest, got, wantDesc[busiest])
			}
			for _, body := range cycleRingBodies {
				t.Run(fmt.Sprintf("%d/%d/%s", n, k, body.name), func(t *testing.T) {
					ctx := cycleRingCtx(body.w, body.h)
					v := budgetCFE(n, k, cfeLabelMax, wantDesc[busiest])
					if l, d := runeLen(v.Phases[0].Label), runeLen(v.Phases[0].Description); l != cfeLabelMax || d != wantDesc[busiest] {
						t.Fatalf("probe copy is %d / %d characters, want %d / %d", l, d, cfeLabelMax, wantDesc[busiest])
					}
					lay, err := cfeMeasure(ctx, v, &CycleFigureEightOverrides{})
					if err != nil {
						t.Fatal(err)
					}
					if lay.labelPt < 12 || lay.descPt < 12 || !lay.showDesc {
						t.Errorf("at budget: label %.0fpt description %.0fpt showDesc=%v", lay.labelPt, lay.descPt, lay.showDesc)
					}
					for i, row := range lay.rows {
						if row.capped || row.y1-row.y0 < row.need-0.01 {
							t.Errorf("at budget: phases[%d] row is %.1fpt tall but needs %.1fpt", i, row.y1-row.y0, row.need)
						}
					}
					if lay.side < cfeMinSidePt {
						t.Errorf("at budget: lobes of %.0fpt, under the %.0fpt minimum", lay.side, cfeMinSidePt)
					}
					if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
						t.Errorf("at budget: %v", w)
					}
				})
			}
		}
	}
}

// A lobe of up to three phases keeps its full size at the budget on every
// shipped body: the lobes only give width to the label columns past it.
func TestCycleFigureEightLobesKeepTheirSizeAtBudget(t *testing.T) {
	for _, body := range cycleRingBodies {
		for n := 4; n <= 6; n++ {
			ctx := cycleRingCtx(body.w, body.h)
			empty, err := cfeMeasure(ctx, budgetCFE(n, 0, 4, 0), &CycleFigureEightOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			full, err := cfeMeasure(ctx, budgetCFE(n, 0, cfeLabelMax, cfeDescMax), &CycleFigureEightOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			if full.side < empty.side-0.01 {
				t.Errorf("%s n=%d: lobes shrink from %.0fpt to %.0fpt at the budget", body.name, n, empty.side, full.side)
			}
		}
	}
}

// One character over the description budget reports BODY_TOO_LONG naming the
// phase, the lobe's phase count and the budget. Only a lobe of four can be
// over it: for smaller lobes the schema maximum is the budget.
func TestCycleFigureEightWarningsNameTheBindingContent(t *testing.T) {
	p := &cycleFigureEight{}
	for _, tc := range []struct{ n, k int }{{6, 2}, {6, 4}, {7, 0}, {7, 3}, {8, 0}} {
		v := budgetCFE(tc.n, tc.k, 12, cfeDescFourMax)
		if w := p.PostExpandWarnings(ExpandContext{}, v, nil); len(w) != 0 {
			t.Fatalf("n=%d k=%d at the description budget warned: %v", tc.n, tc.k, w)
		}
		v.Phases[tc.n-2].Description = cycleRingCopy(cfeDescFourMax + 1)
		w := p.PostExpandWarnings(ExpandContext{}, v, nil)
		want := fmt.Sprintf("phases[%d].description is %d characters; with 4 phases on a lobe use about %d", tc.n-2, cfeDescFourMax+1, cfeDescFourMax)
		if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong) || !strings.Contains(w[0], want) {
			t.Errorf("n=%d k=%d description warning = %v, want %q", tc.n, tc.k, w, want)
		}
	}
	for _, tc := range []struct{ n, k int }{{4, 0}, {5, 0}, {5, 2}, {6, 0}} {
		if w := p.PostExpandWarnings(cycleRingCtx(687, 294), budgetCFE(tc.n, tc.k, cfeLabelMax, cfeDescMax), nil); len(w) != 0 {
			t.Errorf("n=%d k=%d at the schema maxima on the smallest body warned: %v", tc.n, tc.k, w)
		}
	}
}
