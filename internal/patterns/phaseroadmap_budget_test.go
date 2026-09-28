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
		{3, 30, 40}, {4, 28, 30}, {5, 21, 21}, {6, 16, 16},
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
			// A 30-character date label is the schema maximum: only the
			// milestone can go over budget at three phases.
			var want []string
			if tc.dateBudget < 30 {
				v.Phases[1].DateLabel += "D"
				want = append(want, "phases[1].date_label", fmt.Sprintf("about %d", tc.dateBudget))
			}
			v.Phases[1].Milestone += "M"
			want = append(want, "phases[1].milestone", fmt.Sprintf("about %d", tc.milestoneBudget))
			got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
			if len(got) != len(want)/2 {
				t.Fatalf("dense copy warnings: %v", got)
			}
			for i := 0; i < len(want); i += 2 {
				if !strings.Contains(got[i/2], want[i]) || !strings.Contains(got[i/2], want[i+1]) {
					t.Fatalf("dense copy warnings: %v, want %q / %q", got, want[i], want[i+1])
				}
			}
		})
	}
	// Six phases also bound the description (107) and name (31).
	v := &PhaseRoadmapValues{}
	for i := 0; i < 6; i++ {
		v.Phases = append(v.Phases, PhaseRoadmapPhase{Name: "Plan", Description: "Brief"})
	}
	v.Phases[2].Name = strings.Repeat("N", 31)
	v.Phases[2].Description = strings.Repeat("d", 107)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("six-phase name/description at budget: %v", got)
	}
	v.Phases[2].Name += "N"
	v.Phases[2].Description += "d"
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 2 ||
		!strings.Contains(got[0], "phases[2].description") || !strings.Contains(got[0], "about 107") ||
		!strings.Contains(got[1], "phases[2].name") || !strings.Contains(got[1], "about 31") {
		t.Fatalf("six-phase name/description warnings: %v", got)
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
		{"date_label", 30, "16 with 6"}, {"milestone", 60, "16 with 6"},
	} {
		schema := fields[field.name]
		if schema.raw.MaxLength == nil || *schema.raw.MaxLength != field.max || !strings.Contains(schema.raw.Description, field.hint) {
			t.Errorf("%s loses sparse maximum or dense guidance: %+v", field.name, schema.raw)
		}
	}
}
