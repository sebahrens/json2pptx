package patterns

import (
	"strings"
	"testing"
)

func budgetRoadmap(phases, workstreams int) *RoadmapPhasedValues {
	v := &RoadmapPhasedValues{}
	for i := 0; i < phases; i++ {
		v.Phases = append(v.Phases, "Q")
	}
	for i := 0; i < workstreams; i++ {
		ws := RoadmapWorkstream{Name: "Team"}
		for j := 0; j < phases; j++ {
			ws.Items = append(ws.Items, "Task")
		}
		v.Workstreams = append(v.Workstreams, ws)
	}
	return v
}

func TestRoadmapPhasedItemBudgetsMatchProbe(t *testing.T) {
	for p := 2; p <= 8; p++ {
		for w := 2; w <= 6; w++ {
			want := roadmapPhasedItemBudgets[p-2][w-2]
			if got := roadmapPhasedItemBudget(p, w); got != want {
				t.Errorf("budget(%d,%d)=%d, want %d", p, w, got, want)
			}
		}
	}
	if got := roadmapPhasedItemBudget(0, 0); got != 80 {
		t.Errorf("invalid shape budget=%d", got)
	}
	pat := &roadmapPhased{}
	schema := pat.Schema()
	items := schema.raw.Properties["values"].raw.Properties["workstreams"].raw.Items.raw.Properties["items"]
	if !strings.Contains(items.raw.Description, "8: 80/61/41/32/32") || items.raw.Items.raw.MaxLength == nil || *items.raw.Items.raw.MaxLength != 80 {
		t.Errorf("schema omits budget table or reduces sparse-cell maximum: %+v", items.raw)
	}
}

func TestRoadmapPhasedWarnsOnlyOverBudgetActivityPills(t *testing.T) {
	pat := &roadmapPhased{}
	v := budgetRoadmap(8, 6)
	v.Workstreams[1].Items[3] = strings.Repeat("A", 32)
	atBudget := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	v.Workstreams[1].Items[3] += "x"
	overBudget := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	v = budgetRoadmap(2, 2)
	v.Workstreams[0].Items[0] = strings.Repeat("A", 80)
	assertDenseGridWarnings(t, atBudget, overBudget,
		pat.PostExpandWarnings(ExpandContext{}, v, nil),
		pat.PostExpandWarnings(ExpandContext{}, nil, nil),
		"workstreams[1].items[3]", "8-phase x 6-workstream", "about 32")
}
