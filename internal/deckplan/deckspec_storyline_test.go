package deckplan

import (
	"strings"
	"testing"
)

// The three briefs from the 2026-10-01 slide-quality review that all drafted
// the same kind sequence (go-slide-creator-khzni, go-slide-creator-gvbw8).
const (
	reviewBriefQBR      = "Q3 FY26 quarterly business review for the executive committee: revenue grew 18% YoY to $42M, gross margin expanded to 61% (+3 pts vs plan), net revenue retention 112%, EMEA pipeline up 35%, churn rose to 4.1% in SMB segment; priorities for Q4 are launching the enterprise tier, hiring 12 AEs, and fixing SMB onboarding"
	reviewBriefStrategy = "Strategy recommendation for the board: should Acme enter the mid-market segment? Market is $4.2B growing 14% a year; three options were evaluated (build, partner with Globex, acquire Initech) against cost, time-to-market, and risk; we recommend partnering with Globex, which costs $8M over 2 years versus $25M to acquire, and asks the board to approve a term sheet by 30 November"
	reviewBriefProduct  = "Product update for customers: we shipped SSO, audit logs and a new analytics dashboard in Q3; adoption of the dashboard is 38% of active accounts; roadmap for Q4 is mobile app beta in October, API v2 in November, and SOC 2 Type II in December"
)

func storylineKinds(p *DeckSpecPlan) []string {
	var kinds []string
	for _, s := range p.Slots {
		kinds = append(kinds, s.Kind)
	}
	return kinds
}

func slotByName(p *DeckSpecPlan, name string) *DeckSpecSlot {
	for i := range p.Slots {
		if p.Slots[i].Slot == name {
			return &p.Slots[i]
		}
	}
	return nil
}

func factsOf(p *DeckSpecPlan, slot string) string {
	var all []string
	for _, s := range p.Slots {
		if s.Slot == slot {
			all = append(all, s.Facts...)
		}
	}
	return strings.Join(all, " | ")
}

func indexOfKind(kinds []string, kind string) int {
	for i, k := range kinds {
		if k == kind {
			return i
		}
	}
	return -1
}

func TestDeckSpecStorylineFollowsTheBrief(t *testing.T) {
	qbr := BuildDeckSpecPlan(Params{Brief: reviewBriefQBR, SlideBudget: 10})
	strategy := BuildDeckSpecPlan(Params{Brief: reviewBriefStrategy, SlideBudget: 10})
	product := BuildDeckSpecPlan(Params{Brief: reviewBriefProduct, SlideBudget: 8})

	q, s, pr := strings.Join(storylineKinds(qbr), " "), strings.Join(storylineKinds(strategy), " "), strings.Join(storylineKinds(product), " ")
	t.Logf("qbr:      %s", q)
	t.Logf("strategy: %s", s)
	t.Logf("product:  %s", pr)
	if q == s || s == pr || q == pr {
		t.Errorf("three different briefs drafted the same storyline:\n qbr %s\n strategy %s\n product %s", q, s, pr)
	}

	// (1) An options-vs-criteria brief gets an option matrix before the decision.
	sk := storylineKinds(strategy)
	om, dec := indexOfKind(sk, "option_matrix"), indexOfKind(sk, "decision")
	if om < 0 || dec < 0 || om > dec {
		t.Errorf("strategy storyline %v: want option_matrix before decision", sk)
	}

	// (2) No series data, no chart: neither the strategy nor the product brief
	// carries a trend, so neither drafts chart_insight.
	for name, p := range map[string]*DeckSpecPlan{"strategy": strategy, "product": product} {
		if indexOfKind(storylineKinds(p), "chart_insight") >= 0 {
			t.Errorf("%s brief has no series data but drafted chart_insight: %v", name, storylineKinds(p))
		}
	}
	// The QBR's YoY revenue fact is a series: one chart, carrying it.
	if got := factsOf(qbr, "evidence"); !strings.Contains(got, "18% YoY") {
		t.Errorf("QBR chart evidence = %q, want the YoY revenue fact", got)
	}

	// (3) A customer product update drops cause and decision for highlights
	// and a dated timeline.
	pk := storylineKinds(product)
	if slotByName(product, "cause") != nil || indexOfKind(pk, "decision") >= 0 {
		t.Errorf("customer update drafted a cause/decision arc: %v", pk)
	}
	if indexOfKind(pk, "timeline") < 0 && indexOfKind(pk, "roadmap") < 0 {
		t.Errorf("customer update with dated milestones drafted no timeline: %v", pk)
	}
	if got := factsOf(product, "roadmap"); !strings.Contains(got, "October") || !strings.Contains(got, "SOC 2") {
		t.Errorf("product roadmap facts = %q, want the dated milestones", got)
	}
}

// gvbw8: facts route by what they are, not by whether they contain a digit.
func TestDeckSpecFactRouting(t *testing.T) {
	qbr := BuildDeckSpecPlan(Params{Brief: reviewBriefQBR, SlideBudget: 10})
	kpi := factsOf(qbr, "problem") + factsOf(qbr, "context")
	for _, bad := range []string{"hiring 12 AEs", "fixing SMB onboarding", "launching"} {
		if strings.Contains(kpi, bad) {
			t.Errorf("QBR KPI slot carries the to-do %q: %s", bad, kpi)
		}
	}
	for _, want := range []string{"61%", "112%", "35%", "4.1%"} {
		if !strings.Contains(kpi, want) {
			t.Errorf("QBR KPI slot lost the metric %q: %s", want, kpi)
		}
	}
	todo := factsOf(qbr, "plan") + factsOf(qbr, "closing") + factsOf(qbr, "ask")
	if !strings.Contains(todo, "hiring 12 AEs") {
		t.Errorf("QBR to-do 'hiring 12 AEs' should reach the plan/next steps, got %q (unplaced %v)", todo, qbr.UnplacedFacts)
	}

	product := BuildDeckSpecPlan(Params{Brief: reviewBriefProduct, SlideBudget: 8})
	if got := factsOf(product, "context") + factsOf(product, "problem"); strings.Contains(got, "SOC 2") || !strings.Contains(got, "38%") {
		t.Errorf("product stat slot = %q, want the 38%% adoption metric and not the SOC 2 milestone", got)
	}

	strategy := BuildDeckSpecPlan(Params{Brief: reviewBriefStrategy, SlideBudget: 10})
	for _, s := range strategy.Slots {
		for _, f := range s.Facts {
			if f == "partner with Globex" || strings.HasPrefix(f, "acquire Initech)") {
				t.Errorf("parenthesised option list was split into the fragment %q (slot %s)", f, s.Slot)
			}
		}
	}
	if got := factsOf(strategy, "options"); !strings.Contains(got, "(build, partner with Globex, acquire Initech)") {
		t.Errorf("options slot = %q, want the intact option list", got)
	}
	if got := factsOf(strategy, "ask"); !strings.Contains(got, "30 November") {
		t.Errorf("the board ask should route to the decision slot, got %q", got)
	}
	if got := factsOf(strategy, "problem") + factsOf(strategy, "context"); strings.Contains(got, "30 November") {
		t.Errorf("the ask landed on the KPI slot: %q", got)
	}
}

// khzni (4)/(5): a large budget is drafted in chapters, never as a run of
// same-kind slides.
func TestDeckSpecChaptersAndRuns(t *testing.T) {
	for _, brief := range []string{reviewBriefQBR, reviewBriefStrategy, reviewBriefProduct} {
		for _, budget := range []int{3, 5, 7, 8, 10, 12, 16, 30} {
			p := BuildDeckSpecPlan(Params{Brief: brief, SlideBudget: budget})
			kinds := storylineKinds(p)
			for i := 2; i < len(kinds); i++ {
				if kinds[i] == kinds[i-1] && kinds[i] == kinds[i-2] {
					t.Errorf("budget %d: three consecutive %s slides: %v", budget, kinds[i], kinds)
				}
			}
			rendered := len(p.Slots)
			if st := p.DeckSpec.Structure; st != nil {
				rendered += 1 + len(st.Sections)
				if !st.AutoAgenda || len(st.Sections) < 2 || len(st.Sections) > 4 {
					t.Errorf("budget %d: structure must carry auto_agenda and 2-4 sections, got %+v", budget, st)
				}
				if p.DeckSpec.Slides != nil {
					t.Errorf("budget %d: structure and slides are exclusive", budget)
				}
				chrome, _ := p.DeckSpec.Meta["chrome"].(map[string]any)
				if chrome["tracker"] != true {
					t.Errorf("budget %d: a chaptered draft must turn the tracker on: %v", budget, chrome)
				}
			} else if budget >= 10 && brief != reviewBriefProduct {
				t.Errorf("budget %d: a %d-slide board draft should be chaptered: %v", budget, budget, kinds)
			}
			if rendered > budget {
				t.Errorf("budget %d: draft renders %d slides", budget, rendered)
			}
			chrome, _ := p.DeckSpec.Meta["chrome"].(map[string]any)
			pn, _ := chrome["page_numbers"].(map[string]any)
			if pn["enabled"] != true {
				t.Errorf("budget %d: draft meta.chrome must enable page numbers: %v", budget, p.DeckSpec.Meta)
			}
			for _, s := range p.Slots {
				if s.Path == "" {
					t.Errorf("budget %d: slot %s has no path", budget, s.Slot)
				}
			}
		}
	}
}
