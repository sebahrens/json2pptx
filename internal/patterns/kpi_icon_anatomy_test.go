package patterns

import "testing"

// Every kpi-Nup card stacks its default icon above the value, so kpi-2up /
// kpi-3up share the kpi-4up+ anatomy on every template instead of pinning
// the icon at the left edge of landscape cards (go-slide-creator-ux1le). An
// authored position still wins.
func TestKPINupIconAnatomyIsTopForEveryCount(t *testing.T) {
	for _, ctx := range []ExpandContext{{}, kpiTestCtx()} {
		for _, name := range []string{"kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up"} {
			pat, _ := Default().Get(name)
			k := pat.(*kpiNup)
			// Copy: the exemplars are shared package state.
			cells := append(KPINupValues(nil), k.cfg.Exemplars...)
			for i := range cells {
				cells[i].Icon = &IconRef{Name: "trending-up"}
			}
			cells[0].Icon.Position = "left"
			grid, err := pat.Expand(ctx, &cells, nil, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for i, c := range grid.Rows[0].Cells {
				if c.Shape == nil || c.Shape.Icon == nil {
					t.Fatalf("%s card %d has no icon", name, i)
				}
				want := "top"
				if i == 0 {
					want = "left"
				}
				if got := c.Shape.Icon.Position; got != want {
					t.Errorf("%s card %d icon position = %q, want %q", name, i, got, want)
				}
			}
		}
	}
}
