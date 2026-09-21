package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func TestPhaseRoadmapDenseCopyWarnings(t *testing.T) {
	pat := &phaseRoadmap{}
	for _, tc := range []struct {
		phases, dateBudget, milestoneBudget int
	}{
		{3, 30, 60}, {4, 30, 60}, {5, 27, 52}, {6, 22, 42},
	} {
		t.Run(fmt.Sprintf("%d phases", tc.phases), func(t *testing.T) {
			v := &PhaseRoadmapValues{}
			for i := 0; i < tc.phases; i++ {
				v.Phases = append(v.Phases, PhaseRoadmapPhase{Name: "Plan", DateLabel: "Q1", Description: "Brief", Milestone: "Gate"})
			}
			v.Phases[1].DateLabel = strings.Repeat("D", tc.dateBudget)
			v.Phases[1].Milestone = strings.Repeat("M", tc.milestoneBudget)
			if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
				t.Fatalf("at readable budgets: %v", got)
			}
			if tc.phases <= 4 {
				return
			}
			v.Phases[1].DateLabel += "D"
			v.Phases[1].Milestone += "M"
			got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
			if len(got) != 2 || !strings.Contains(got[0], "phases[1].date_label") ||
				!strings.Contains(got[0], fmt.Sprintf("about %d", tc.dateBudget)) ||
				!strings.Contains(got[1], "phases[1].milestone") ||
				!strings.Contains(got[1], fmt.Sprintf("about %d", tc.milestoneBudget)) {
				t.Fatalf("dense copy warnings: %v", got)
			}
		})
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	fields := pat.Schema().raw.Properties["values"].raw.Properties["phases"].raw.Items.raw.Properties
	for _, field := range []struct {
		name string
		max  int
		hint string
	}{
		{"date_label", 30, "22 with 6"}, {"milestone", 60, "42 with 6"},
	} {
		schema := fields[field.name]
		if schema.raw.MaxLength == nil || *schema.raw.MaxLength != field.max || !strings.Contains(schema.raw.Description, field.hint) {
			t.Errorf("%s loses sparse maximum or dense guidance: %+v", field.name, schema.raw)
		}
	}
}
