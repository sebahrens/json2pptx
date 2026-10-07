package deckplan

import (
	"fmt"
	"strings"
	"testing"
)

// Regressions from the 2026-10-07 MCP audit. Each reads the public plan
// response — slots, deck_spec and unplaced_facts — not the private outline.

// planAccountsFor reports whether the text is in a slot's facts or in
// unplaced_facts.
func planAccountsFor(p *DeckSpecPlan, text string) bool {
	for _, s := range p.Slots {
		for _, f := range s.Facts {
			if strings.Contains(f, text) {
				return true
			}
		}
	}
	for _, f := range p.UnplacedFacts {
		if strings.Contains(f, text) {
			return true
		}
	}
	return false
}

// draftSlideAt resolves a slot path to its slide in the draft.
func draftSlideAt(t *testing.T, p *DeckSpecPlan, path string) map[string]any {
	t.Helper()
	var a, b int
	if n, _ := fmt.Sscanf(path, "slides[%d]", &a); n == 1 {
		return p.DeckSpec.Slides[a]
	}
	if n, _ := fmt.Sscanf(path, "structure.sections[%d].slides[%d]", &a, &b); n == 2 {
		return p.DeckSpec.Structure.Sections[a].Slides[b]
	}
	t.Fatalf("unresolvable slot path %q", path)
	return nil
}

// go-slide-creator-l3e28: the facts routed to an outline's items are in the
// returned slots.
func TestOutlineRoutedFactsReachTheSlots(t *testing.T) {
	figures := []string{"EUR 212m", "EUR 31m", "14.6%"}
	for _, tc := range []struct {
		name, brief  string
		wantUnplaced bool
	}{
		{"body items", "Create 5 slides. Slides: title, executive summary, market chart, recommendation, next steps. Revenue is EUR 212m. EBITDA is EUR 31m. Margin is 14.6%.", false},
		{"appendix item", "Create 7 slides. Slides: title, executive summary, market chart, recommendation, next steps, appendix market assumptions. Revenue is EUR 212m. EBITDA is EUR 31m. Margin is 14.6%.", false},
		{"no room", "Create 4 slides. Slides: title, agenda, team, next steps. Revenue is EUR 212m. EBITDA is EUR 31m. Margin is 14.6%.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildDeckSpecPlan(Params{Brief: tc.brief})
			for _, fig := range figures {
				if !planAccountsFor(p, fig) {
					t.Errorf("%s is in neither slots[].facts nor unplaced_facts\nslots: %+v\nunplaced: %q", fig, p.Slots, p.UnplacedFacts)
				}
			}
			if got := len(p.UnplacedFacts) > 0; got != tc.wantUnplaced {
				t.Errorf("unplaced_facts = %q, want any = %v", p.UnplacedFacts, tc.wantUnplaced)
			}
		})
	}

	// Several facts appended to one item all survive the appendix split.
	p := BuildDeckSpecPlan(Params{Brief: "Create 5 slides. Slides: title, executive summary, market chart, recommendation, next steps. Revenue is EUR 212m. EBITDA is EUR 31m. Margin is 14.6%."})
	routed := 0
	for _, s := range p.Slots {
		if s.Kind == "executive_summary" {
			routed = len(s.Facts) - 1
		}
	}
	if routed < 2 {
		t.Errorf("executive summary carries %d routed facts, want at least 2", routed)
	}
}

// go-slide-creator-zjyee: slots[].regions describes the slot's own slide.
func TestRegionSlotsMatchTheirOwnSlide(t *testing.T) {
	for _, tc := range []struct {
		name, brief string
		want        [][]string // region kinds per regions slot, in deck order
	}{
		{"chart and stat, then table and text", "Create 5 slides. Slides: title; executive summary; revenue chart beside a margin KPI; cost table beside explanatory text; next steps.", [][]string{{"chart", "stat"}, {"table", "text"}}},
		{"chart and stat, then chart and timeline", "Create 5 slides. Slides: title; executive summary; revenue chart beside a margin KPI; cost chart beside a timeline; next steps.", [][]string{{"chart", "stat"}, {"chart", "timeline"}}},
		{"appendix path", "Create 7 slides. Slides: title; executive summary; revenue chart beside a margin KPI; next steps; appendix: cost table beside a timeline.", [][]string{{"chart", "stat"}, {"table", "timeline"}}},
		{"section path", "Board update on churn for the CFO with chapters, 12 slides. Churn rose to 4.2%. Churn by year: 2023 3.1%, 2024 3.6%, 2025 4.2% on the left and the 61 lost accounts on the right. Appendix with backup detail.", [][]string{{"chart", "stat"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildDeckSpecPlan(Params{Brief: tc.brief})
			var got [][]string
			for _, s := range p.Slots {
				if s.Kind != "regions" {
					continue
				}
				slide := draftSlideAt(t, p, s.Path)
				drafted, _ := slide["regions"].([]any)
				if len(drafted) != len(s.Regions) {
					t.Fatalf("%s: %d region slots for %d drafted regions", s.Path, len(s.Regions), len(drafted))
				}
				var kinds []string
				for k, r := range s.Regions {
					region := drafted[k].(map[string]any)
					if r.Kind != region["kind"] {
						t.Errorf("%s: slot says %s, the draft has %v", r.Path, r.Kind, region["kind"])
					}
					if want := fmt.Sprintf("%s.regions[%d]", s.Path, k); r.Path != want {
						t.Errorf("region path = %s, want %s", r.Path, want)
					}
					if wantPos := []string{"left", "right"}[k]; slide["arrangement"] == "columns" && r.Position != wantPos {
						t.Errorf("%s: position = %s, want %s", r.Path, r.Position, wantPos)
					}
					kinds = append(kinds, r.Kind)
				}
				got = append(got, kinds)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("region kinds = %v, want %v", got, tc.want)
			}
			if tc.name == "section path" && p.DeckSpec.Structure == nil {
				t.Error("want a chaptered draft")
			}
		})
	}
}

// go-slide-creator-hxcum: the data of one slide is not the deck's outline.
func TestSlideDataListIsNotAnOutline(t *testing.T) {
	for _, brief := range []string{
		"Board deck, 8 slides. A chart slide shows revenue: 2023 EUR 6.1m, 2024 EUR 8.4m, 2025 EUR 11.2m.",
		"Board deck, 8 slides. A chart slide shows revenue: 2023 EUR 6.1m; 2024 EUR 8.4m; 2025 EUR 11.2m.",
		"Board deck, 8 slides. The bridge slide walks EBITDA: 2023 EBITDA 24.0, price +6.5, volume +3.2, opex -2.7.",
		"Board deck, 8 slides. The bridge slide walks EBITDA: 2023 EBITDA 24.0; price +6.5; volume +3.2; opex -2.7.",
		"Board deck, 8 slides. One KPI slide carries the baseline: revenue EUR 212m, EBITDA EUR 31m, margin 14.6%.",
		"Board deck, 8 slides. One KPI slide carries the baseline: revenue EUR 212m; EBITDA EUR 31m; margin 14.6%.",
		"Board deck, 8 slides. The chart slide: 2023 EUR 6.1m, 2024 EUR 8.4m, 2025 EUR 11.2m.",
		"Board deck, 8 slides. A pillars slide covers the model: governance, risk identification, control framework, reporting.",
		"Board deck, 8 slides. The deck must cover: revenue EUR 212m, EBITDA EUR 31m, margin 14.6%.",
	} {
		if o := parseOutline(brief, 8); o != nil {
			t.Errorf("%q\n  read as an outline of %d items", brief, len(o.items))
		}
	}
	for brief, want := range map[string]int{
		"Board deck. Slides: problem, solution, market size chart, the ask.":                                4,
		"Board deck. Slides: revenue EUR 212m, EBITDA EUR 31m, margin 14.6%.":                               3,
		"Board deck. Outline: problem; solution; market size chart; the ask.":                               4,
		"Board deck. Slide order: problem, solution, market size chart, the ask.":                           4,
		"Board deck.\nHere is the slide order:\n- problem\n- solution\n- market size chart\n- the ask":      4,
		"Board deck.\nSlide 1: title\nSlide 2: revenue chart\nSlide 3: risks\nSlide 4: next steps":          3,
		"7-slide investor pitch: problem, solution, market size chart, traction KPIs, team of 4, the ask":   6,
		"Board deck covering: problem, solution, market size chart, the ask. Revenue is EUR 212m in total.": 4,
	} {
		got := 0
		if o := parseOutline(brief, 0); o != nil {
			got = len(o.items)
		}
		if got != want {
			t.Errorf("%q\n  outline items = %d, want %d", brief, got, want)
		}
	}
}

// go-slide-creator-hxcum: the audit brief keeps its three years on one chart
// and the layout it asks for.
func TestSingularSlideLabelKeepsItsChartTogether(t *testing.T) {
	p := BuildDeckSpecPlan(Params{Brief: "Create an 8-slide board proposal named Nordbolt diligence. Revenue EUR 212m and EBITDA EUR 31m imply a 14.6% margin. A regions slide shows a bar chart of the market: 2021 EUR 8.1bn, 2025 EUR 9.4bn, 2028 forecast EUR 10.6bn on the left and our 2.3% share on the right. Ask: CFO approves scope on Friday."})
	var regions []map[string]any
	for _, s := range p.DeckSpec.Slides {
		switch s["kind"] {
		case "regions":
			regions = append(regions, s)
		case "stat":
			if f := fmt.Sprint(s); strings.Contains(f, "8.1") || strings.Contains(f, "9.4") {
				t.Errorf("a market year is drafted as its own stat slide: %v", s)
			}
		}
	}
	if len(regions) != 1 {
		t.Fatalf("regions slides = %d, want 1\n%v", len(regions), p.DeckSpec.Slides)
	}
	if regions[0]["arrangement"] != "columns" {
		t.Errorf("arrangement = %v, want columns", regions[0]["arrangement"])
	}
	pair := regions[0]["regions"].([]any)
	left, right := pair[0].(map[string]any), pair[1].(map[string]any)
	if left["kind"] != "chart" || right["kind"] != "stat" {
		t.Fatalf("regions = %v beside %v, want a chart beside a stat", left["kind"], right["kind"])
	}
	data := left["chart"].(map[string]any)["data"].(map[string]any)
	if got := fmt.Sprint(data["categories"]); got != "[2021 2025 2028]" {
		t.Errorf("chart categories = %s, want [2021 2025 2028]", got)
	}
	if got := fmt.Sprint(data["series"].([]any)[0].(map[string]any)["values"]); got != "[8.1 9.4 10.6]" {
		t.Errorf("chart values = %s, want [8.1 9.4 10.6]", got)
	}
	if right["value"] != "2.3%" {
		t.Errorf("stat value = %v, want 2.3%%", right["value"])
	}
	for _, fig := range []string{"EUR 212m", "EUR 31m", "14.6%", "10.6bn", "2.3%", "CFO approves"} {
		if !planAccountsFor(p, fig) {
			t.Errorf("%s is in neither slots[].facts nor unplaced_facts", fig)
		}
	}
	if strings.Contains(p.BudgetNote, "outline") {
		t.Errorf("budget note still claims an outline: %s", p.BudgetNote)
	}
}
