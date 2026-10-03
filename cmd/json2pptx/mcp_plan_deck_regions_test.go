package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestPlanDeckRegionsDraftsValidateOnEveryTemplate is the go-slide-creator-
// umev3 regression: plan_deck drafts a regions slide, an author fills only
// the drafted fields (keeping the arrangement and shares), and the slide
// validates with no blocking finding on every shipped template — and on the
// local p-style when present. A stat drafted under a timeline used to get
// 40% of a modern stack and shrink below the 12pt floor.
func TestPlanDeckRegionsDraftsValidateOnEveryTemplate(t *testing.T) {
	tpls, err := filepath.Glob("../../templates/*.pptx")
	if err != nil || len(tpls) < 9 {
		t.Fatalf("shipped templates: %v %v", tpls, err)
	}
	briefs := map[string]string{
		"chart-left":    "left two-thirds a line chart of quarterly revenue Q1=12, Q2=14, Q3=17, Q4=21 million; upper right a 32% gross margin KPI; lower right a three-step launch timeline (Design October, Pilot November, Rollout December)",
		"chart-top":     "top a line chart of quarterly revenue; bottom left a 32% gross margin KPI; bottom right a three-step launch timeline (Design October, Pilot November, Rollout December)",
		"stat-timeline": "top a 32% gross margin KPI; bottom a three-step launch timeline (Design October, Pilot November, Rollout December)",
		"stat-table":    "left two-thirds a line chart of quarterly revenue; upper right a 32% gross margin KPI; lower right a table of revenue by region",
	}
	mc := refusalTestConfig(t)
	for name, layout := range briefs {
		brief := "Create a two-slide executive review. Slide 1 is the title. Slide 2 must divide the canvas into regions: " + layout + "."
		for _, format := range []string{"deckspec", "raw"} {
			res := mustCall(t, mc.handlePlanDeck, map[string]any{"brief": brief, "slide_budget": float64(2), "format": format, "template": "modern"})
			if res.IsError {
				t.Fatalf("%s/%s: plan_deck: %s", name, format, textContent(res))
			}
			slide := filledRegionsDraft(t, format, textContent(res))
			for _, tpl := range tpls {
				tpl = strings.TrimSuffix(filepath.Base(tpl), ".pptx")
				t.Run(name+"/"+format+"/"+tpl, func(t *testing.T) {
					spec, _ := json.Marshal(map[string]any{
						"meta":   map[string]any{"template": tpl, "title": "Executive review"},
						"slides": []any{map[string]any{"kind": "title", "title": "Executive review", "subtitle": "Board, October"}, slide},
					})
					env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": string(spec)}))
					onSlide := env.FindingEnvelope
					onSlide.Findings = nil
					for _, f := range env.Findings {
						if f.Where == nil || f.Where.Slide == nil || *f.Where.Slide == 1 {
							onSlide.Findings = append(onSlide.Findings, f)
						}
					}
					if bad := blockingFindings(onSlide); !env.OK || len(bad) > 0 {
						t.Errorf("drafted regions slide does not validate (ok=%t):\n  %s", env.OK, strings.Join(bad, "\n  "))
					}
				})
			}
		}
	}
}

// filledRegionsDraft returns plan_deck's drafted regions slide as a DeckSpec
// slide with every placeholder filled the way an author would — the drafted
// fields plus a stat's context line, arrangement and shares kept. A raw draft
// is wrapped as a raw_json2pptx slide.
func filledRegionsDraft(t *testing.T, format, body string) map[string]any {
	t.Helper()
	var plan struct {
		DeckSpec struct {
			Slides []map[string]any `json:"slides"`
		} `json:"deck_spec"`
		Slides []struct {
			NarrativeRole string         `json:"narrative_role"`
			Skeleton      map[string]any `json:"skeleton"`
		} `json:"slides"`
	}
	if err := json.Unmarshal([]byte(body), &plan); err != nil {
		t.Fatal(err)
	}
	var slide map[string]any
	if format == "raw" {
		for _, s := range plan.Slides {
			if s.NarrativeRole == "composition" {
				slide = map[string]any{"kind": "raw_json2pptx", "slide": s.Skeleton}
			}
		}
	} else {
		for _, s := range plan.DeckSpec.Slides {
			if s["kind"] == "regions" {
				slide = s
			}
		}
	}
	if slide == nil {
		t.Fatalf("no regions slide drafted: %s", body)
	}
	milestones := []string{"Design", "Pilot", "Rollout", "Scale", "Review", "Close", "Exit"}
	months := []string{"Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr"}
	stop := 0
	var fill func(v any, key string) any
	fill = func(v any, key string) any {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["categories"]; ok && key == "data" {
				return map[string]any{"categories": []any{"Q1", "Q2", "Q3", "Q4"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17, 21}}}}
			}
			_, isStop := x["date"]
			_, isStat := x["value"]
			for k, e := range x {
				switch s, _ := e.(string); {
				case s != patterns.FillPlaceholder:
					x[k] = fill(e, k)
				case isStop && k == "label":
					x[k] = milestones[stop%len(milestones)]
				case isStop && k == "date":
					x[k] = months[stop%len(months)]
				case isStat && k == "label":
					x[k] = "Gross margin, Q4"
				case k == "value":
					x[k] = "32%"
				case k == "takeaway":
					x[k] = "Revenue nearly doubled in a year; the margin pays for the rollout."
				case k == "title" || k == "text_value":
					x[k] = "Revenue growth funds the launch at a 32% gross margin"
				default:
					x[k] = "Revenue"
				}
			}
			if isStop {
				stop++
			}
			if isStat {
				// The stat's optional context line: the tallest stat an
				// author writes from the draft.
				x["context"] = "Up 4 points on plan"
			}
			return x
		case []any:
			for i := range x {
				x[i] = fill(x[i], key)
			}
			return x
		case string:
			if x == patterns.FillPlaceholder {
				return []string{"Region", "Revenue"}[stop%2]
			}
		}
		return v
	}
	fill(slide, "")
	if format == "raw" {
		slide["slide"].(map[string]any)["source"] = "Finance ledger, FY26"
	} else {
		slide["source"] = "Finance ledger, FY26"
	}
	return slide
}

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
