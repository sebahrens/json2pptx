package svggen

import (
	"strings"
	"testing"
)

// buildOrgChartRequest makes an org_chart request with the given hierarchy:
// one CEO, len(reportsPerCLevel) C-levels, and that many reports under each.
func buildOrgChartRequest(reportsPerCLevel []int, width, height int) *RequestEnvelope {
	cLevels := make([]any, 0, len(reportsPerCLevel))
	for i, n := range reportsPerCLevel {
		cid := string(rune('A' + i))
		reports := make([]any, 0, n)
		for j := 0; j < n; j++ {
			reports = append(reports, map[string]any{
				"name":  "Report " + cid + string(rune('1'+j)),
				"title": "Director",
			})
		}
		cLevels = append(cLevels, map[string]any{
			"name":     "C-Level " + cid,
			"title":    "Chief Officer",
			"children": reports,
		})
	}
	return &RequestEnvelope{
		Type: "org_chart",
		Data: map[string]any{
			"root": map[string]any{"name": "Ada Root", "title": "CEO", "children": cLevels},
		},
		Output: OutputSpec{Width: width, Height: height},
	}
}

// go-slide-creator-pwcg: a 21-person org chart rendered as 5 boxes reading
// "+4 reports" with every named person below level 1 gone — no warning, no
// finding. collapseSiblings reported the nodes IT hid; pruneDeepestLevel
// reported nothing.
func TestOrgChart_DepthPruneIsReported(t *testing.T) {
	// 4 C-levels with 4-5 reports each = 21 people, on a canvas too small to
	// render them all: the same shape as the report.
	req := buildOrgChartRequest([]int{4, 4, 4, 5}, 700, 300)

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}

	var pruned *Finding
	for i := range findings {
		if findings[i].Code == FindingOrgChartDepthPruned {
			pruned = &findings[i]
			break
		}
	}
	if pruned == nil {
		t.Fatalf("a chart that drops a whole level must report %s; got %+v",
			FindingOrgChartDepthPruned, findings)
	}
	if pruned.Severity != "warning" {
		t.Errorf("severity = %q, want warning", pruned.Severity)
	}
	if pruned.Fix == nil || pruned.Fix.Kind != FixKindReduceItems {
		t.Fatalf("fix = %+v, want kind %q", pruned.Fix, FixKindReduceItems)
	}
	removed, ok := pruned.Fix.Params["removed_count"].(int)
	if !ok || removed <= 0 {
		t.Errorf("fix.params.removed_count = %v, want a positive count", pruned.Fix.Params["removed_count"])
	}
	if !strings.Contains(pruned.Message, "split it across slides") {
		t.Errorf("message should tell the author what to do, got: %s", pruned.Message)
	}
}

// A chart that fits must not report a prune.
func TestOrgChart_SmallChartReportsNoPrune(t *testing.T) {
	req := buildOrgChartRequest([]int{1, 1}, 1024, 768)

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for _, f := range findings {
		if f.Code == FindingOrgChartDepthPruned {
			t.Errorf("a 5-person chart on a full canvas must not report a depth prune: %+v", f)
		}
	}
}
