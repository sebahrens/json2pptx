package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Run with JSON2PPTX_BUDGET_PROBE=1 go test ./cmd/json2pptx
// -run TestPatternBudgetProbe -v. It measures the actual fit collector against
// every bundled template, without adding a slow combinatorial sweep to normal
// tests. A clean result (no below-floor finding) has budget >= the probe size.
func TestPatternBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_BUDGET_PROBE") != "1" {
		t.Skip("manual budget calibration probe")
	}
	for count := 1; count <= 10; count++ {
		bindingName, bindingBudget := "", 201
		for _, name := range bmcProbeCellNames() {
			budget := bmcReadableBudget(t, name, count)
			t.Logf("bmc-canvas %-21s %2d bullets: %3d chars per bullet", name, count, budget)
			if budget < bindingBudget {
				bindingName, bindingBudget = name, budget
			}
		}
		t.Logf("bmc-canvas %d bullets: binding cell %s at %d chars", count, bindingName, bindingBudget)
	}

	pat, _ := patterns.Default().Get("pull-quote")
	values, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		input := &PresentationInput{Template: name, Slides: []SlideInput{{
			SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: pat.Name(), Values: encoded},
		}}}
		for _, finding := range collectReadabilityFindings(input, layouts, width, height) {
			t.Logf("pull-quote %s: %s", name, finding.Message)
		}
	}
}

func bmcProbeCellNames() []string {
	return []string{"key_partners", "key_activities", "key_resources", "value_propositions", "customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"}
}

func bmcReadableBudget(t *testing.T, cellName string, bulletCount int) int {
	t.Helper()
	type templateGeometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	geometries := make([]templateGeometry, 0, len(schemaMaximaTemplates))
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, templateGeometry{name, layouts, width, height})
	}
	clean := func(length int) bool {
		values := bmcProbeValues()
		cell := bmcProbeCell(values, cellName)
		cell.Bullets = make([]string, bulletCount)
		for i := range cell.Bullets {
			cell.Bullets[i] = strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "bmc-canvas", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, 200
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func bmcProbeValues() *patterns.BMCCanvasValues {
	cell := func(header string) patterns.BMCCell {
		return patterns.BMCCell{Header: header, Bullets: []string{"base"}}
	}
	return &patterns.BMCCanvasValues{
		KeyPartners: cell("Key Partners"), KeyActivities: cell("Key Activities"),
		KeyResources: cell("Key Resources"), ValuePropositions: cell("Value Propositions"),
		CustomerRelations: cell("Customer Relations"), Channels: cell("Channels"),
		CustomerSegments: cell("Customer Segments"), CostStructure: cell("Cost Structure"),
		RevenueStreams: cell("Revenue Streams"),
	}
}

func bmcProbeCell(values *patterns.BMCCanvasValues, name string) *patterns.BMCCell {
	switch name {
	case "key_partners":
		return &values.KeyPartners
	case "key_activities":
		return &values.KeyActivities
	case "key_resources":
		return &values.KeyResources
	case "value_propositions":
		return &values.ValuePropositions
	case "customer_relations":
		return &values.CustomerRelations
	case "channels":
		return &values.Channels
	case "customer_segments":
		return &values.CustomerSegments
	case "cost_structure":
		return &values.CostStructure
	case "revenue_streams":
		return &values.RevenueStreams
	default:
		return nil
	}
}
