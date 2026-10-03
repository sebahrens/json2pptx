package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func strategyHouseFindings(t *testing.T, templateName string, values any) []patterns.FitFinding {
	t.Helper()
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	layouts, width, height := schemaMaximaLayouts(t, templateName)
	deck := &PresentationInput{Template: templateName, Slides: []SlideInput{{
		SlideType: "content", LayoutID: "blank-title",
		Pattern: &PatternInput{Name: "strategy-house", Values: encoded},
	}}}
	return collectFitFindings(deck, layouts, width, height, nil)
}

// TestStrategyHouseShapeAdvisoryReachesFitReport pins go-slide-creator-qad87:
// a foundation that joins separate enablers, and a pillar left empty beside
// full ones, reach the fit report as an info finding on every template — and
// a house whose shape follows its content carries none.
func TestStrategyHouseShapeAdvisoryReachesFitReport(t *testing.T) {
	forced := &patterns.StrategyHouseValues{
		Objective: "Become the trusted settlement platform", Foundation: "People · Data · Controls",
		Pillars: []patterns.StrategyHousePillar{
			{Title: "Customer trust", Body: []string{"Transparent pricing", "Operational resilience", "Same-day disputes"}},
			{Title: "Product velocity", Body: []string{"Weekly releases"}},
			{Title: "Partner reach"},
		},
	}
	shaped := patterns.Default()
	pat, _ := shaped.Get("strategy-house")
	exemplar := pat.(patterns.Exemplar).ExemplarValues()
	for _, templateName := range schemaMaximaTemplateNames(t) {
		t.Run(templateName, func(t *testing.T) {
			t.Parallel()
			var joined, empty bool
			for _, f := range strategyHouseFindings(t, templateName, forced) {
				if f.Code != patterns.ErrCodeHouseShapeForced {
					continue
				}
				if f.Action != "info" {
					t.Errorf("HOUSE_SHAPE_FORCED action = %q, want info", f.Action)
				}
				joined = joined || strings.Contains(f.Message, `foundation joins 3 separate items`)
				empty = empty || strings.Contains(f.Message, `pillars[2] ("Partner reach") has no body`)
			}
			if !joined || !empty {
				t.Errorf("advisory missing: joined foundation %t, empty pillar %t", joined, empty)
			}
			for _, f := range strategyHouseFindings(t, templateName, exemplar) {
				if f.Code == patterns.ErrCodeHouseShapeForced {
					t.Errorf("the exemplar is reported as forced: %s", f.Message)
				}
			}
		})
	}
}

// TestStrategyHouseRoofIsNotASparseBlock pins the roof against SPARSE_FILL:
// the gable is the roof's form, so the roof is measured by the eaves band
// that holds its text, with and without roof badges, for 3-5 pillars and one
// or two foundation levels.
func TestStrategyHouseRoofIsNotASparseBlock(t *testing.T) {
	pillars := []patterns.StrategyHousePillar{
		{Title: "Workflow redesign", Body: []string{"Map work end to end", "Set human checkpoints", "Measure cycle time"}},
		{Title: "Agent platform", Body: []string{"Shared tools and data access", "Evaluation and guardrails"}},
		{Title: "People and skills", Body: []string{"Train process owners", "Reward adoption"}},
		{Title: "Risk and controls", Body: []string{"Audit every production agent"}},
		{Title: "Partner reach", Body: []string{"Marketplace live in 12 markets"}},
	}
	for _, templateName := range schemaMaximaTemplateNames(t) {
		t.Run(templateName, func(t *testing.T) {
			t.Parallel()
			for n := 3; n <= 5; n++ {
				for _, badges := range []bool{false, true} {
					for _, layered := range []bool{false, true} {
						v := &patterns.StrategyHouseValues{
							Objective: "Turn AI from pilots into a repeatable operating capability",
							Pillars:   pillars[:n], Foundation: "Governance, security and cost tracking for every production agent",
						}
						if badges {
							v.RoofBadges = []string{"Vision 2027", "Mission"}
						}
						if layered {
							v.Foundation = ""
							v.FoundationLayers = []patterns.StrategyHouseLayer{{"Governance and cost tracking for every agent"}, {"People", "Data platform", "Controls"}}
						}
						for _, f := range strategyHouseFindings(t, templateName, v) {
							if f.Code == "SPARSE_FILL" || f.Code == patterns.ErrCodeBodyTooLong {
								t.Errorf("%d pillars badges=%t layered=%t: %s at %s: %s", n, badges, layered, f.Code, f.Path, f.Message)
							}
						}
					}
				}
			}
		})
	}
}
