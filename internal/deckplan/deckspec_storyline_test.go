package deckplan

import (
	"fmt"
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

// khzni: back matter is drafted from spare budget when the brief has backup
// material, after the next-steps close, and never pads a brief that has none.
func TestDeckSpecAppendix(t *testing.T) {
	appendixOf := func(p *DeckSpecPlan) *DeckSpecSectionDraft {
		if st := p.DeckSpec.Structure; st != nil && len(st.Sections) > 0 && st.Sections[len(st.Sections)-1].Appendix {
			return &st.Sections[len(st.Sections)-1]
		}
		return nil
	}

	// A QBR with six figures and room to spare gets a backup data table after
	// the close; the close moves into the last chapter so it precedes it.
	qbr := BuildDeckSpecPlan(Params{Brief: reviewBriefQBR, SlideBudget: 12})
	app := appendixOf(qbr)
	if app == nil || app.Title != "Appendix" || len(app.Slides) != 1 || app.Slides[0]["kind"] != "table" {
		t.Fatalf("QBR at 12 slides: want one backup table in an appendix section, got %+v", qbr.DeckSpec.Structure)
	}
	if qbr.DeckSpec.Structure.Closing != nil {
		t.Errorf("with an appendix the close must end the last chapter, not render after the appendix")
	}
	closing := slotByName(qbr, "closing")
	if closing == nil || !strings.HasPrefix(closing.Path, "structure.sections[1].slides[") || closing.Appendix {
		t.Errorf("closing slot = %+v, want the end of the last body chapter", closing)
	}
	if backup := slotByName(qbr, "backup"); backup == nil || !backup.Appendix || backup.Section != "Appendix" || backup.Path != "structure.sections[2].slides[0]" {
		t.Errorf("backup slot = %+v", backup)
	}

	// No room: the appendix never displaces a body slide.
	if app := appendixOf(BuildDeckSpecPlan(Params{Brief: reviewBriefQBR, SlideBudget: 10})); app != nil {
		t.Errorf("QBR at 10 slides has no spare room but drafted an appendix: %+v", app)
	}
	// No backup material: room alone does not pad the deck.
	for _, brief := range []string{reviewBriefProduct, reviewBriefStrategy} {
		if app := appendixOf(BuildDeckSpecPlan(Params{Brief: brief, SlideBudget: 16})); app != nil {
			t.Errorf("brief without backup material drafted an appendix: %+v", app)
		}
	}

	// A brief that asks for backup and names its methodology gets both.
	asked := BuildDeckSpecPlan(Params{Brief: reviewBriefStrategy + "; include backup detail on the market sizing methodology and assumptions", SlideBudget: 14})
	app = appendixOf(asked)
	if app == nil || len(app.Slides) != 2 || slotByName(asked, "methodology") == nil {
		t.Errorf("backup + methodology brief: want backup and methodology slides in the appendix, got %+v / %v", app, storylineKinds(asked))
	}

	// A flat draft carries the appendix as an appendix divider after the close.
	flat := BuildDeckSpecPlan(Params{Brief: reviewBriefProduct + "; include backup detail on adoption by segment", SlideBudget: 8})
	if flat.DeckSpec.Structure != nil {
		t.Fatalf("product brief at 8 slides should stay flat: %+v", flat.DeckSpec.Structure)
	}
	slides := flat.DeckSpec.Slides
	if len(slides) < 3 || slides[len(slides)-2]["kind"] != "section" || slides[len(slides)-2]["appendix"] != true || slides[len(slides)-3]["kind"] != "next_steps" {
		t.Errorf("flat appendix: want next_steps, then a section with appendix: true, then backup: %v", slides)
	}
	for i, s := range flat.Slots {
		if s.SlideIndex != i || s.Path != fmt.Sprintf("slides[%d]", i) {
			t.Errorf("flat slot %d = %+v: slide_index / path out of step", i, s)
		}
	}
	if d := slotByName(flat, "appendix"); d == nil || d.Kind != "section" || !d.Appendix {
		t.Errorf("flat appendix divider slot = %+v", d)
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
				body := 0
				for si, sec := range st.Sections {
					if !sec.Appendix {
						body++
					} else if si != len(st.Sections)-1 {
						t.Errorf("budget %d: the appendix section must be last: %+v", budget, st.Sections)
					}
				}
				if !st.AutoAgenda || body < 2 || body > 4 {
					t.Errorf("budget %d: structure must carry auto_agenda and 2-4 chapters, got %+v", budget, st)
				}
				if p.DeckSpec.Slides != nil {
					t.Errorf("budget %d: structure and slides are exclusive", budget)
				}
				chrome, _ := p.DeckSpec.Meta["chrome"].(map[string]any)
				if chrome["tracker"] != true {
					t.Errorf("budget %d: a chaptered draft must turn the tracker on: %v", budget, chrome)
				}
			} else if budget >= 16 && brief != reviewBriefProduct {
				t.Errorf("budget %d: a %d-slide board draft should be chaptered: %v", budget, budget, kinds)
			}
			// 58qda: no agenda or dividers under 12 slides unless asked.
			if budget < chapterBudget && p.DeckSpec.Structure != nil {
				t.Errorf("budget %d: a draft under %d slides must stay flat: %+v", budget, chapterBudget, p.DeckSpec.Structure)
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

// planFactsAndUnplaced joins every slot's facts and the unplaced facts.
func planFactsAndUnplaced(p *DeckSpecPlan) string {
	var all []string
	for _, s := range p.Slots {
		all = append(all, s.Facts...)
	}
	return strings.Join(append(all, p.UnplacedFacts...), " | ")
}

// ze5u7: every amount and date of a long comparison brief reaches a slot or
// unplaced_facts, and the comparison reaches its slot whole.
func TestDeckSpecLongComparisonKeepsEveryAmount(t *testing.T) {
	for _, budget := range []int{5, 8, 10, 12} {
		p := BuildDeckSpecPlan(Params{Brief: reviewBriefPilot, SlideBudget: budget, TemplateName: "warm-coral"})
		all := planFactsAndUnplaced(p)
		for _, want := range []string{"€10m", "2024", "€12m", "2025", "45%", "38%", "€0.8m", "6 months", "€0.5m", "2 months", "January 2027", "November 2026"} {
			if !strings.Contains(all, want) {
				t.Errorf("budget %d: %q in neither slots[].facts nor unplaced_facts: %s", budget, want, all)
			}
		}
		if strings.Contains(all, "(€ ") || strings.HasSuffix(all, "(€") || strings.Contains(all, "...") {
			t.Errorf("budget %d: a fact was truncated: %s", budget, all)
		}
		opts := factsOf(p, "options")
		if !strings.Contains(opts, "€0.8m per year, launch in 6 months) with outsourcing (€0.5m per year, launch in 2 months)") {
			t.Errorf("budget %d: option_matrix facts = %q, want the whole comparison", budget, opts)
		}
	}
}

// fu6uy: an explicit "compare A with B" request drafts an option matrix before
// the decision, routes both alternatives intact, and carries the recommended
// alternative with its reason to the decision slot.
func TestDeckSpecExplicitCompareDraftsOptionMatrix(t *testing.T) {
	const reco = "Recommend internal support to retain customer relationships"
	for _, brief := range []string{
		"Compare internal support (€0.8m per year) with outsourcing (€0.5m per year) on cost and launch speed. " + reco + ".",
		"Board pilot review. We compare internal support (€0.8m per year) to outsourcing (€0.5m per year) on cost and launch speed. " + reco + ".",
		"Board pilot review. Outsourcing (€0.5m per year) compared to internal support (€0.8m per year) on cost and launch speed. " + reco + ".",
	} {
		p := BuildDeckSpecPlan(Params{Brief: brief, SlideBudget: 12})
		kinds := storylineKinds(p)
		om, dec := indexOfKind(kinds, "option_matrix"), indexOfKind(kinds, "decision")
		if om < 0 || dec < 0 || om > dec {
			t.Errorf("%q: storyline %v, want option_matrix before decision", brief, kinds)
		}
		if opts := factsOf(p, "options"); !strings.Contains(opts, "€0.8m") || !strings.Contains(opts, "€0.5m") {
			t.Errorf("%q: option_matrix facts = %q, want both alternatives", brief, opts)
		}
		for _, slot := range []string{"context", "problem"} {
			if got := factsOf(p, slot); strings.Contains(got, "€0.5m") {
				t.Errorf("%q: the comparison landed on the %s slot: %q", brief, slot, got)
			}
		}
		if ask := factsOf(p, "ask"); !strings.Contains(ask, reco) {
			t.Errorf("%q: decision facts = %q, want the recommendation and its reason (unplaced %v)", brief, ask, p.UnplacedFacts)
		}
	}

	// A comparison against a benchmark is a metric, not a choice.
	bench := BuildDeckSpecPlan(Params{Brief: "Q3 results for the board. Revenue compared with plan rose 12% to €40m. Margin compared to last year reached 31%.", SlideBudget: 12})
	if kinds := storylineKinds(bench); indexOfKind(kinds, "option_matrix") >= 0 {
		t.Errorf("benchmark comparison drafted an option matrix: %v", kinds)
	}
	if got := factsOf(bench, "context") + factsOf(bench, "problem") + factsOf(bench, "evidence"); !strings.Contains(got, "€40m") || !strings.Contains(got, "31%") {
		t.Errorf("benchmark metrics should stay on the KPI slots, got %q", got)
	}
}
