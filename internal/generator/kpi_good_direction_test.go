package generator

import (
	"regexp"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// good_direction says which way a metric improves: a fall in a cost is good
// news and takes the positive ink, a rise in it the negative ink, while the
// arrow keeps showing which way the metric moved. Without it up is good, as
// before (go-slide-creator-atocl).
func TestKPIDeltaToneFollowsGoodDirection(t *testing.T) {
	spec := &types.DiagramSpec{Type: "kpi_dashboard", Data: map[string]any{"metrics": []any{
		map[string]any{"label": "Task success", "value": "91%", "change": "+4 pts", "trend": "up"},
		map[string]any{"label": "Cost per task", "value": "$0.90", "change": "-12%", "trend": "down", "good_direction": "down"},
		map[string]any{"label": "Churn", "value": "3.1%", "change": "+0.4 pts", "trend": "up", "good_direction": "down"},
		map[string]any{"label": "Revenue", "value": "$4M", "change": "-2%", "trend": "down"},
		map[string]any{"label": "NPS", "value": "62", "change": "0", "trend": "flat", "good_direction": "down"},
	}}}
	if err := ValidateNativeDiagramData(spec); err != nil {
		t.Fatalf("good_direction must be an accepted metric field: %v", err)
	}
	wantTone := []string{kpiDeltaGood, kpiDeltaGood, kpiDeltaBad, kpiDeltaBad, ""}
	for i, m := range parseKPIMetrics(spec.Data) {
		if got := m.deltaTone(); got != wantTone[i] {
			t.Errorf("%s: tone %q, want %q", m.label, got, wantTone[i])
		}
	}

	env := nativeDiagramEnv{fontName: "Arial"}
	layout, err := layoutNativeDiagram(spec, types.BoundingBox{Width: 11000000, Height: 2000000}, env, nativeDiagramSite{})
	if err != nil {
		t.Fatal(err)
	}
	xml := renderNativeInsert(&layout.insert, 100, env)
	// Each delta run: the scheme colour it is printed in and its text.
	re := regexp.MustCompile(`(?s)<a:schemeClr val="(accent\d|dk1)"[^<]*(?:<a:shade val="\d+"/>\s*</a:schemeClr>)?\s*</a:solidFill>.{0,200}?<a:t>([^<]*)</a:t>`)
	got := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(xml, -1) {
		got[m[2]] = m[1]
	}
	for text, want := range map[string]string{
		"▲ +4 pts":   "accent6", // up, good
		"▼ -12%":     "accent6", // down, and down is good
		"▲ +0.4 pts": "accent2", // up, and down is good
		"▼ -2%":      "accent2", // down, up is good
		"→ 0":        "dk1",     // flat has no tone
	} {
		if got[text] != want {
			t.Errorf("delta %q is printed in %q, want %q", text, got[text], want)
		}
	}
}
