package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompileNextStepsPattern(t *testing.T) {
	body := map[string]any{
		"title": "Three actions start the pilot in October",
		"actions": []any{
			map[string]any{"action": "Confirm pilot scope", "owner": "COO", "date": "15 Oct"},
			map[string]any{"title": "Hire the pod", "who": "VP Customer", "due": "31 Oct"},
			"Report the first read-out",
		},
		"decisions": []any{"Approve the €1.2M budget"},
	}
	slide, links, err := CompileNextSteps(Input{Title: "Three actions start the pilot in October", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "next-steps" || slide.LayoutID != "blank-title" {
		t.Fatalf("slide = %+v, want the next-steps pattern", slide)
	}
	var v nextStepsValues
	if err := json.Unmarshal(slide.Pattern.Values, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Actions) != 3 || v.Actions[1] != (nextStepsAction{Action: "Hire the pod", Owner: "VP Customer", Date: "31 Oct"}) {
		t.Errorf("actions = %+v", v.Actions)
	}
	if len(v.Decisions) != 1 || v.Decisions[0] != "Approve the €1.2M budget" {
		t.Errorf("decisions = %+v", v.Decisions)
	}
	if len(links) < 3 {
		t.Errorf("links = %+v, want title, actions and decisions", links)
	}
}

// Outside the pattern's bounds nothing is lost: actions keep owner and date
// and decisions stay labelled in the bullet fallback.
func TestCompileNextStepsFallback(t *testing.T) {
	body := map[string]any{
		"actions":   []any{map[string]any{"action": "Only one", "owner": "COO", "date": "Oct"}},
		"decisions": []any{"Approve"},
	}
	if NextStepsPattern(body) != "" || NextStepsDegradeReason(body) == "" {
		t.Fatal("a single action must degrade with a reason")
	}
	slide, _, err := CompileNextSteps(Input{Title: "Next steps", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil {
		t.Fatalf("pattern = %+v, want a bullet fallback", slide.Pattern)
	}
	var all []string
	for _, c := range slide.Content {
		if c.BulletsValue != nil {
			all = append(all, *c.BulletsValue...)
		}
	}
	joined := strings.Join(all, "|")
	if !strings.Contains(joined, "Only one — COO, Oct") || !strings.Contains(joined, "Decision requested: Approve") {
		t.Errorf("fallback bullets = %q", joined)
	}
}

func TestKPISnapshotCarriesComparator(t *testing.T) {
	body := map[string]any{"kpis": []any{
		map[string]any{"value": "38%", "label": "Gross margin", "comparator": "vs plan +4 pts"},
		map[string]any{"value": "$12M", "label": "Free cash flow", "vs": "vs PY -2%"},
	}}
	slide, _, err := CompileKPISnapshot(Input{Title: "Margin beat plan", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || !strings.Contains(string(slide.Pattern.Values), `"comparator":"vs plan +4 pts"`) || !strings.Contains(string(slide.Pattern.Values), `"comparator":"vs PY -2%"`) {
		t.Fatalf("pattern values = %s", slide.Pattern.Values)
	}
}
