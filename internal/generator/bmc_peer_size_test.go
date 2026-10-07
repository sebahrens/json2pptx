package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

func bmcPeerSpec() *types.DiagramSpec {
	list := func(items ...string) []any {
		out := make([]any, len(items))
		for i, s := range items {
			out[i] = s
		}
		return out
	}
	return &types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
		"key_partners":       list("Model labs", "SaaS vendors", "Cloud provider", "Systems integrators"),
		"key_activities":     list("Run the runtime", "Certify tools", "Train teams", "Review incidents"),
		"key_resources":      list("Platform team", "Tool catalogue", "Evaluation data", "Compute budget"),
		"value_propositions": list("Weeks to launch", "Built-in controls", "One bill", "Shared evaluation"),
		"customer_relations": list("Product partner", "Office hours", "Quarterly review"),
		"channels":           list("Developer portal", "Solution architects", "Community of practice"),
		"customer_segments":  list("Service", "IT", "Finance", "Legal"),
		"cost_structure":     list("Inference", "Engineering", "Oversight"),
		"revenue_streams":    list("Chargeback per task", "Platform fee"),
	}}
}

var (
	bmcBodyShapeRE = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	bmcRunSizeRE   = regexp.MustCompile(`<a:rPr[^>]* sz="(\d+)"`)
)

// bmcBodySizes returns, for every section body of the rendered canvas, the
// set of run sizes it is written in and its normAutofit element.
func bmcBodySizes(t *testing.T, xml string) (sizes []string, autofit []string) {
	t.Helper()
	for _, sp := range bmcBodyShapeRE.FindAllString(xml, -1) {
		if !strings.Contains(sp, `name="BMC Body"`) && !strings.Contains(sp, `name="BMC Bullets"`) {
			continue
		}
		seen := map[string]bool{}
		for _, m := range bmcRunSizeRE.FindAllStringSubmatch(sp, -1) {
			seen[m[1]] = true
		}
		if len(seen) != 1 {
			t.Fatalf("a section body mixes run sizes %v", seen)
		}
		for s := range seen {
			sizes = append(sizes, s)
		}
		autofit = append(autofit, regexp.MustCompile(`<a:normAutofit[^>]*/>`).FindString(sp))
	}
	return sizes, autofit
}

// The nine sections are peers: in a region too short for them at 12pt every
// body is set at one size, the largest at which all of them fit. Each body
// used to store its own autofit scale (88%, 94%, 96% and six at 100% in a
// 70% compose segment), so the stacked half-height cells were set smaller
// than the sections beside them (go-slide-creator-7ee4f).
func TestBMCSectionsShareOneBodySize(t *testing.T) {
	env := nativeDiagramEnv{fontName: "Arial"}
	render := func(w, h int64) string {
		layout, err := layoutNativeDiagram(bmcPeerSpec(), ptBounds(w, h), env, nativeDiagramSite{})
		if err != nil {
			t.Fatal(err)
		}
		return renderNativeInsert(&layout.insert, 100, env)
	}

	// A 70%-wide segment of a short content area.
	sizes, autofit := bmcBodySizes(t, render(560, 250))
	if len(sizes) != 9 {
		t.Fatalf("found %d section bodies, want 9", len(sizes))
	}
	for i, s := range sizes {
		if s != sizes[0] {
			t.Errorf("section %d is set at %s, section 0 at %s: peers must share one size", i, s, sizes[0])
		}
		// The size is declared and marked with a 100% scale: a per-shape
		// stored scale is what LibreOffice ignores, and the mark keeps the
		// shape in the readability scan.
		if autofit[i] != `<a:normAutofit fontScale="100000"/>` {
			t.Errorf("section %d autofit = %s, want the 100%% mark", i, autofit[i])
		}
	}
	if len(sizes[0]) == 4 && sizes[0] >= "1200" {
		t.Errorf("shared size = %s, want below 1200 in this region", sizes[0])
	}
	// The shrink is still reported against the 12pt floor.
	findings := unreadableAutofitFindings(render(560, 250), tokens.ViewingModePresentation, func(id string) string { return "/rendered_shapes/" + id }, nil)
	if len(findings) == 0 {
		t.Error("a canvas set below 12pt raises no readability finding")
	}

	// With room, nothing changes: 12pt and the bare autofit element.
	sizes, autofit = bmcBodySizes(t, render(900, 420))
	for i, s := range sizes {
		if s != "1200" || autofit[i] != `<a:normAutofit/>` {
			t.Errorf("roomy canvas: section %d at %s with %s, want 1200 and no scale", i, s, autofit[i])
		}
	}
}
