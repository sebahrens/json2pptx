package patterns

import (
	"strings"
	"testing"
)

func budgetSwimlane(steps, lanes int) *SwimlaneValues {
	v := &SwimlaneValues{}
	for i := 0; i < lanes; i++ {
		lane := SwimlaneLane{Actor: "Team"}
		for j := 0; j < steps; j++ {
			lane.Steps = append(lane.Steps, "Task")
		}
		v.Lanes = append(v.Lanes, lane)
	}
	return v
}

func TestSwimlaneStepBudgetsMatchProbe(t *testing.T) {
	for steps := 2; steps <= 8; steps++ {
		for lanes := 2; lanes <= 6; lanes++ {
			want := swimlaneStepBudgets[steps-2][lanes-2]
			if got := swimlaneStepBudget(steps, lanes); got != want {
				t.Errorf("budget(%d,%d)=%d, want %d", steps, lanes, got, want)
			}
		}
	}
	pat := &swimlane{}
	steps := pat.Schema().raw.Properties["values"].raw.Properties["lanes"].raw.Items.raw.Properties["steps"]
	if !strings.Contains(steps.raw.Description, "8: 80/80/62/42/32") || steps.raw.Items.raw.MaxLength == nil || *steps.raw.Items.raw.MaxLength != 80 {
		t.Errorf("schema loses dense guidance or sparse maximum: %+v", steps.raw)
	}
}

func TestSwimlaneWarningsNameStepAndShape(t *testing.T) {
	pat := &swimlane{}
	v := budgetSwimlane(8, 6)
	v.Lanes[2].Steps[4] = strings.Repeat("S", 32)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("at budget: %v", got)
	}
	v.Lanes[2].Steps[4] += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "lanes[2].steps[4]") || !strings.Contains(got[0], "8-step x 6-lane") || !strings.Contains(got[0], "about 32") {
		t.Fatalf("dense step warning: %v", got)
	}
	v = budgetSwimlane(2, 2)
	v.Lanes[0].Steps[0] = strings.Repeat("S", 80)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("sparse step at schema maximum warned: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}
