package patterns

import (
	"sort"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestDiagramRulesMatchSvggenRegistry locks the keyword->diagram-type scoring
// table (diagramRules) to the svggen "ready" diagram registry. The two are kept
// in sync by hand: adding a diagram type to svggen.DiagramCapabilitiesReady()
// without a matching recommend_visual rule (or vice versa) is otherwise silent —
// the new type becomes un-recommendable, or a rule points at a non-ready type.
//
// This guard catches that drift. If it fails, reconcile diagramRules in
// recommend_visual.go with svggen.DiagramCapabilitiesReady() (svggen/capabilities.go).
func TestDiagramRulesMatchSvggenRegistry(t *testing.T) {
	ruleTypes := make(map[string]bool, len(diagramRules))
	for _, r := range diagramRules {
		if ruleTypes[r.diagramType] {
			t.Errorf("duplicate diagramType in diagramRules: %q", r.diagramType)
		}
		ruleTypes[r.diagramType] = true
	}

	readyTypes := make(map[string]bool)
	for _, d := range svggen.DiagramCapabilitiesReady() {
		readyTypes[d.Type] = true
	}

	for typ := range ruleTypes {
		if !readyTypes[typ] {
			t.Errorf("diagramRules has type %q with no matching svggen ready diagram (rule points at a non-ready or unknown type)", typ)
		}
	}
	for typ := range readyTypes {
		if !ruleTypes[typ] {
			t.Errorf("svggen ready diagram %q has no matching diagramRules entry (type is un-recommendable)", typ)
		}
	}

	if len(ruleTypes) != len(readyTypes) {
		t.Errorf("diagramRules type count (%d) != svggen ready diagram count (%d): %v vs %v",
			len(ruleTypes), len(readyTypes), sortedKeys(ruleTypes), sortedKeys(readyTypes))
	}
}

// TestChartRulesMatchSvggenRegistry mirrors TestDiagramRulesMatchSvggenRegistry
// for the chart scoring table (chartRules) against the svggen "ready" chart
// registry. Same silent-drift risk: a ready chart type with no rule is
// un-recommendable, and a rule for a non-ready type recommends a stub.
//
// If it fails, reconcile chartRules in recommend_visual.go with the "ready"
// entries of svggen.ChartCapabilities() (svggen/capabilities.go).
func TestChartRulesMatchSvggenRegistry(t *testing.T) {
	ruleTypes := make(map[string]bool, len(chartRules))
	for _, r := range chartRules {
		if ruleTypes[r.chartType] {
			t.Errorf("duplicate chartType in chartRules: %q", r.chartType)
		}
		ruleTypes[r.chartType] = true
	}

	readyTypes := make(map[string]bool)
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			readyTypes[c.Type] = true
		}
	}

	for typ := range ruleTypes {
		if !readyTypes[typ] {
			t.Errorf("chartRules has type %q with no matching svggen ready chart (rule points at a non-ready or unknown type)", typ)
		}
	}
	for typ := range readyTypes {
		if !ruleTypes[typ] {
			t.Errorf("svggen ready chart %q has no matching chartRules entry (type is un-recommendable)", typ)
		}
	}

	if len(ruleTypes) != len(readyTypes) {
		t.Errorf("chartRules type count (%d) != svggen ready chart count (%d): %v vs %v",
			len(ruleTypes), len(readyTypes), sortedKeys(ruleTypes), sortedKeys(readyTypes))
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
