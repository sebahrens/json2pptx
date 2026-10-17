package slides

import (
	"strings"
	"testing"
)

func TestCycleStatusesReachSlideAndRegion(t *testing.T) {
	for _, style := range []string{"ring", "figure_eight"} {
		body := map[string]any{"style": style, "phases": []any{map[string]any{"label": "Plan", "highlight": true, "status": "done"}, "Build", map[string]any{"label": "Run", "status": "planned"}, map[string]any{"label": "Learn", "status": "planned"}}}
		if issues := CycleIssues(body); len(issues) > 0 {
			t.Fatalf("%s: %+v", style, issues)
		}
		p := resolveCycle(body)
		block, err := p.pattern()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(block.Values), `"status":"planned"`) != 2 || !strings.Contains(string(block.Values), `"highlight":true`) {
			t.Fatal(string(block.Values))
		}
		region, err := regionCycle(body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(region.cell.Pattern), `"status":"planned"`) {
			t.Fatal(string(region.cell.Pattern))
		}
	}
	for _, status := range []any{"invalid", true, ""} {
		issues := CycleIssues(map[string]any{"phases": []any{map[string]any{"label": "Plan", "status": status}, "Build", "Run", "Learn"}})
		if len(issues) == 0 || !issues[0].Invalid || issues[0].Field != "phases[0].status" {
			t.Fatalf("invalid status %v: %+v", status, issues)
		}
	}
	issues := CycleIssues(map[string]any{"style": "nodes", "phases": []any{map[string]any{"label": "Plan", "status": "planned"}, "Build", "Run", "Learn"}})
	if len(issues) == 0 || !issues[0].Invalid {
		t.Fatal("unsupported style silently drops status")
	}
}
