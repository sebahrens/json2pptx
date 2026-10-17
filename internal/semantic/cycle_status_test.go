package semantic

import (
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"strings"
	"testing"
)

func TestCycleInvalidStatusIsHardForSlideAndRegion(t *testing.T) {
	for _, region := range []bool{false, true} {
		body := map[string]any{"title": "Cycle", "phases": []any{map[string]any{"label": "Plan", "status": "unknown"}, "Build", "Run", "Learn"}}
		spec := oneSlideSpec(KindCycle, body)
		if region {
			body["kind"] = "cycle"
			spec = regionsSpec(map[string]any{"arrangement": "columns", "regions": []any{body, map[string]any{"kind": "text", "body": "Notes"}}})
		}
		got := findingsWithCode(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticFieldType)
		found := false
		for _, d := range got {
			if strings.HasSuffix(d.Path, "phases[0].status") && d.Severity == diagnostics.SeverityError {
				found = true
			}
		}
		if !found {
			t.Fatalf("region=%v: invalid status not rejected: %+v", region, got)
		}
	}
}
