package deckplan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The brief from the 2026-10-02 layout-composition review: one slide of three
// placed visuals between a title and an ask (go-slide-creator-vae7f).
const regionsBrief = "Create a three-slide executive review. Slide 1 is the title. Slide 2 must divide the canvas into three regions: left two-thirds a line chart of quarterly revenue Q1=12, Q2=14, Q3=17, Q4=21 million; upper right a 32% gross margin KPI; lower right a three-step launch timeline (Design October, Pilot November, Rollout December). Keep those content types together so executives see the performance, profitability and delivery plan on one slide. Slide 3 asks the COO to approve the rollout on 9 October."

func TestParseRegionRequest(t *testing.T) {
	req := parseRegionRequest(regionsBrief)
	if !req.draftable() {
		t.Fatal("the brief places three visuals on one slide")
	}
	if req.arrangement != "main_left" || len(req.regions) != 3 || len(req.unsupported) != 0 {
		t.Fatalf("arrangement %q, %d regions, unsupported %v", req.arrangement, len(req.regions), req.unsupported)
	}
	want := []struct {
		pos, kind string
		size      float64
	}{{"L", "chart", 67}, {"TR", "stat", 0}, {"BR", "timeline", 0}}
	for i, w := range want {
		r := req.regions[i]
		if r.pos != w.pos || r.kind != w.kind || r.size != w.size {
			t.Errorf("region %d = %s/%s/%g, want %s/%s/%g", i, r.pos, r.kind, r.size, w.pos, w.kind, w.size)
		}
	}
	if req.regions[0].chartType != "line_chart" || req.regions[2].count != 3 {
		t.Errorf("chart type %q, timeline count %d", req.regions[0].chartType, req.regions[2].count)
	}
	for _, gone := range []string{"line chart", "gross margin KPI", "launch timeline", "three regions"} {
		if strings.Contains(req.remainder, gone) {
			t.Errorf("remainder still carries %q: %s", gone, req.remainder)
		}
	}
	if !strings.Contains(req.remainder, "approve the rollout") {
		t.Errorf("remainder lost the rest of the brief: %s", req.remainder)
	}
}

// Ordinary briefs are not region requests: no false positives from
// "left", "top" or a chart mentioned in passing.
func TestParseRegionRequest_OrdinaryBriefs(t *testing.T) {
	for _, brief := range []string{
		reviewBriefQBR, reviewBriefStrategy, reviewBriefProduct,
		"Left unchanged, the pricing table still drives churn; top accounts want a chart of usage.",
		"Bottom line: margin is 32%. Top priority: a timeline for the launch.",
	} {
		if req := parseRegionRequest(brief); req.draftable() {
			t.Errorf("%q drafted regions %+v", brief, req.regions)
		}
	}
}

func TestParseRegionRequest_Variants(t *testing.T) {
	cases := []struct {
		brief       string
		arrangement string
		kinds       []string
		sizes       []float64
	}{
		{"On one slide: left 60%, a bar chart of bookings by region; right a table of the top five deals.", "columns", []string{"chart", "table"}, []float64{60, 0}},
		{"Top half: a stacked bar chart of cost by quarter. Bottom: commentary on the drivers.", "rows", []string{"chart", "text"}, []float64{50, 0}},
		{"Right: a photo of the plant; upper left the KPIs for output and yield; lower left the narrative.", "main_right", []string{"image", "kpis", "text"}, []float64{0, 0, 0}},
		{"Top a donut chart of the mix; bottom left a table of segments; bottom right three insights.", "main_top", []string{"chart", "table", "text"}, []float64{0, 0, 0}},
	}
	for _, tc := range cases {
		req := parseRegionRequest(tc.brief)
		if !req.draftable() || req.arrangement != tc.arrangement {
			t.Errorf("%q: arrangement %v", tc.brief, req)
			continue
		}
		for i, k := range tc.kinds {
			if req.regions[i].kind != k || req.regions[i].size != tc.sizes[i] {
				t.Errorf("%q region %d = %s/%g, want %s/%g", tc.brief, i, req.regions[i].kind, req.regions[i].size, k, tc.sizes[i])
			}
		}
	}
}

// What the plan cannot draft as asked is said, not dropped or rerouted.
func TestParseRegionRequest_Unsupported(t *testing.T) {
	req := parseRegionRequest("Left a line chart of revenue; right a map of the 14 depots.")
	if !req.draftable() || len(req.unsupported) != 1 || req.regions[1].kind != "text" ||
		!strings.Contains(req.unsupported[0].Reason, "no region kind draws a map") {
		t.Fatalf("unknown visual: %+v / %+v", req.regions, req.unsupported)
	}

	brief := "Upper left a chart of revenue by quarter; lower right a table of the 3 largest deals."
	req = parseRegionRequest(brief)
	if req.draftable() || len(req.unsupported) != 2 || req.remainder != brief ||
		!strings.Contains(req.unsupported[0].Reason, "not a supported arrangement") {
		t.Fatalf("unsupported arrangement: %+v", req)
	}
	plan := BuildDeckSpecPlan(Params{Brief: brief, SlideBudget: 4})
	if len(plan.UnsupportedRegions) != 2 || slotByName(plan, "regions") != nil {
		t.Fatalf("plan should report the arrangement and draft no regions slot: %+v", plan.UnsupportedRegions)
	}
	if !strings.Contains(strings.Join(append(allFacts(plan), plan.UnplacedFacts...), "|"), "3 largest deals") {
		t.Errorf("the unsupported clause's fact was lost: %v / %v", allFacts(plan), plan.UnplacedFacts)
	}
}

func allFacts(p *DeckSpecPlan) []string {
	var out []string
	for _, s := range p.Slots {
		out = append(out, s.Facts...)
	}
	return out
}

// The DeckSpec plan drafts the regions slide with every placed visual mapped
// to a region, keeps the region facts off other slides, and — once filled —
// validates clean.
func TestDeckSpecPlanDraftsRegions(t *testing.T) {
	plan := BuildDeckSpecPlan(Params{Brief: regionsBrief, SlideBudget: 3, TemplateName: "midnight-blue"})
	if got := strings.Join(storylineKinds(plan), ","); got != "title,regions,next_steps" {
		t.Fatalf("storyline = %s", got)
	}
	slot := slotByName(plan, "regions")
	if slot.Path != "slides[1]" || len(slot.Regions) != 3 || len(slot.Facts) != 3 {
		t.Fatalf("regions slot: %+v", slot)
	}
	for i, want := range []string{"chart", "stat", "timeline"} {
		r := slot.Regions[i]
		if r.Kind != want || r.Path != "slides[1].regions["+string(rune('0'+i))+"]" || len(r.Facts) != 1 {
			t.Errorf("region %d: %+v", i, r)
		}
	}
	if slot.Regions[0].Role != "main" || slot.Regions[1].Role != "supporting" || slot.Regions[0].Position != "left" {
		t.Errorf("hierarchy: %+v", slot.Regions)
	}
	for _, s := range plan.Slots {
		if s.Slot == "regions" {
			continue
		}
		for _, f := range s.Facts {
			if strings.Contains(f, "Q2=14") || strings.Contains(f, "gross margin") || strings.Contains(f, "Pilot") {
				t.Errorf("region fact %q routed to %s", f, s.Slot)
			}
		}
	}
	for _, f := range plan.UnplacedFacts {
		if strings.Contains(f, "gross margin") || strings.Contains(f, "Q4=21") {
			t.Errorf("region fact %q left unplaced", f)
		}
	}

	draft := plan.DeckSpec.Slides[1]
	regions := draft["regions"].([]any)
	if draft["kind"] != "regions" || draft["arrangement"] != "main_left" || regions[0].(map[string]any)["size_pct"] != 67.0 {
		t.Fatalf("draft slide: %+v", draft)
	}
	// Fill the draft from the facts, as an agent would, and validate it.
	draft["title"] = "Growth funds the launch at a 32% margin"
	draft["takeaway"] = "Revenue rose to €21m in Q4; the margin pays for the rollout."
	chart := regions[0].(map[string]any)
	chart["heading"] = "Quarterly revenue"
	chart["chart"] = map[string]any{"type": "line_chart", "data": map[string]any{
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series":     []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17, 21}}},
	}}
	stat := regions[1].(map[string]any)
	stat["value"], stat["label"] = "32%", "Gross margin"
	for i, m := range regions[2].(map[string]any)["milestones"].([]any) {
		m.(map[string]any)["label"] = []string{"Design", "Pilot", "Rollout"}[i]
		m.(map[string]any)["date"] = []string{"Oct", "Nov", "Dec"}[i]
	}
	encoded, _ := json.Marshal(map[string]any{
		"meta":   map[string]any{"title": "Executive review", "source": "Finance ledger"},
		"slides": []any{draft},
	})
	spec, parse := semantic.Parse("plan.json", encoded)
	if parse.HasErrors() {
		t.Fatal(parse)
	}
	if ds := semantic.Validate(spec, semantic.StrictnessStrict); diagnostics.HasErrors(ds) {
		t.Fatalf("filled regions draft must validate clean: %v", ds)
	}
	if _, _, err := semantic.Compile(spec, semantic.CompileOptions{}); err != nil {
		t.Fatal(err)
	}
}

// The raw plan drafts the same slide as the shape_grid the regions kind
// compiles to, with each region's cell path, in place of a pattern slot.
func TestRawPlanDraftsCompositionSlide(t *testing.T) {
	res := BuildDeckPlan(patterns.Default(), Params{Brief: regionsBrief, SlideBudget: 3}, nil)
	if len(res.Slides) != 3 {
		t.Fatalf("budget 3 → %d slides", len(res.Slides))
	}
	comp := res.Slides[1]
	if comp.NarrativeRole != "composition" || comp.RecommendedPattern != "" || comp.Layout != "blank-title" || len(comp.Regions) != 3 {
		t.Fatalf("composition slide: %+v", comp)
	}
	var slide deckinput.SlideInput
	dec := json.NewDecoder(bytes.NewReader(comp.Skeleton))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&slide); err != nil {
		t.Fatalf("skeleton is not a raw slide: %v", err)
	}
	if slide.ShapeGrid == nil || string(slide.ShapeGrid.Columns) != "[67,33]" {
		t.Fatalf("skeleton grid: %s", comp.Skeleton)
	}
	paths := []string{"shape_grid.rows[0].cells[0]", "shape_grid.rows[0].cells[1].grid.rows[0].cells[0]", "shape_grid.rows[0].cells[1].grid.rows[1].cells[0]"}
	for i, p := range paths {
		if comp.Regions[i].Path != p {
			t.Errorf("region %d path = %q, want %q", i, comp.Regions[i].Path, p)
		}
	}
	for _, want := range []string{`"stat-hero"`, `"timeline-horizontal"`, `"line_chart"`, patterns.FillPlaceholder} {
		if !strings.Contains(string(comp.Skeleton), want) {
			t.Errorf("skeleton lacks %s", want)
		}
	}
	for _, f := range res.UnplacedFacts {
		if strings.Contains(f, "gross margin") || strings.Contains(f, "Q4=21") {
			t.Errorf("region fact %q left unplaced", f)
		}
	}
}
