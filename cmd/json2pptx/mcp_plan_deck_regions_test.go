package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// plan_deck turns a brief's same-slide layout into one mixed slide in both
// formats, and the response matches the advertised output schema
// (go-slide-creator-vae7f).
func TestPlanDeckRegions(t *testing.T) {
	const brief = "Create a three-slide executive review. Slide 1 is the title. Slide 2 must divide the canvas into three regions: left two-thirds a line chart of quarterly revenue Q1=12, Q2=14, Q3=17, Q4=21 million; upper right a 32% gross margin KPI; lower right a three-step launch timeline (Design October, Pilot November, Rollout December). Slide 3 asks the COO to approve the rollout on 9 October."
	schema := compileToolInputSchema(t, "plan_deck output", outputSchemaPlanDeck)
	mc := testMCPConfig(t)
	for _, format := range []string{"deckspec", "raw"} {
		t.Run(format, func(t *testing.T) {
			res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{
				"brief": brief, "slide_budget": float64(3), "format": format, "template": "midnight-blue",
			}))
			if err != nil || res == nil || res.IsError {
				t.Fatalf("plan_deck failed: %v %s", err, textContent(res))
			}
			body := textContent(res)
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(body)))
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Fatalf("response does not match outputSchemaPlanDeck: %v", err)
			}
			var out struct {
				Slides []struct {
					NarrativeRole string            `json:"narrative_role"`
					Regions       []json.RawMessage `json:"regions"`
				} `json:"slides"`
				Slots []struct {
					Slot    string            `json:"slot"`
					Regions []json.RawMessage `json:"regions"`
				} `json:"slots"`
			}
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				t.Fatal(err)
			}
			regions := 0
			for _, s := range out.Slides {
				if s.NarrativeRole == "composition" {
					regions = len(s.Regions)
				}
			}
			for _, s := range out.Slots {
				if s.Slot == "regions" {
					regions = len(s.Regions)
				}
			}
			if regions != 3 {
				t.Fatalf("want one mixed slide with 3 regions, got %d: %s", regions, body)
			}
			if !strings.Contains(body, "upper right a 32% gross margin KPI") {
				t.Error("the KPI clause is not carried on its region")
			}
		})
	}
}
