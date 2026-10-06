package deckplan

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// cycleSlide returns the drafted cycle slide of a brief, or nil.
func cycleSlide(t *testing.T, brief string) map[string]any {
	t.Helper()
	plan := BuildDeckSpecPlan(Params{Brief: brief})
	for _, s := range plan.DeckSpec.Slides {
		if s["kind"] == "cycle" {
			return s
		}
	}
	if plan.DeckSpec.Structure != nil {
		for _, sec := range plan.DeckSpec.Structure.Sections {
			for _, s := range sec.Slides {
				if s["kind"] == "cycle" {
					return s
				}
			}
		}
	}
	return nil
}

// A brief that names a loop, a hub or nested rings gets a cycle slide in the
// style its words point at (go-slide-creator-53v5u, go-slide-creator-9d1mp).
func TestPlanDeckDraftsCycleSlides(t *testing.T) {
	cases := []struct {
		name, brief string
		style       any // nil: the default ring
		phases      int // 0: not drafted
	}{
		{"lifecycle", "Customer success review for the COO. The customer lifecycle: acquire, onboard, adopt, renew, expand.", nil, 5},
		{"pdca", "Quality programme update. We run PDCA: plan, do, check, act.", nil, 4},
		{"flywheel", "Growth strategy for the board. The flywheel: more sellers, wider selection, lower prices, more buyers.", nil, 4},
		{"devops loop", "Engineering operating model. Our DevOps loop: plan, code, build, test, release, deploy, operate, monitor.", "figure_eight", 8},
		{"hub and spoke", "Data strategy for the CIO. A hub and spoke model around the customer record: sales, service, finance, marketing, product.", "radial", 5},
		{"onion", "Security posture for the audit committee. The onion model: data, applications, network, perimeter.", "concentric", 4},
		{"onboarding then recurring", "Service model for the client. Two onboarding steps then a recurring four-phase cycle.", "intake", 4},
		{"topic sentence only", "our operating rhythm is a six-phase continuous improvement loop fed by a two-step onboarding intake", "intake", 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := cycleSlide(t, tc.brief)
			if s == nil {
				t.Fatalf("no cycle slide drafted for %q", tc.brief)
			}
			if s["style"] != tc.style {
				t.Errorf("style = %v, want %v", s["style"], tc.style)
			}
			phases, _ := s["phases"].([]any)
			if len(phases) != tc.phases {
				t.Errorf("phases = %v, want %d", s["phases"], tc.phases)
			}
		})
	}
}

// The single-sentence brief of the journey: six placeholders for the loop,
// two for the intake, and the slot's guidance names the styles.
func TestPlanDeckCycleFromTopicSentence(t *testing.T) {
	brief := "our operating rhythm is a six-phase continuous improvement loop fed by a two-step onboarding intake"
	plan := BuildDeckSpecPlan(Params{Brief: brief})
	s := cycleSlide(t, brief)
	if s == nil {
		t.Fatal("no cycle slide")
	}
	intake, _ := s["intake"].([]any)
	if len(intake) != 2 || intake[0] != patterns.FillPlaceholder {
		t.Errorf("intake = %v, want two placeholders", s["intake"])
	}
	found := false
	for _, slot := range plan.Slots {
		if slot.Kind != "cycle" {
			continue
		}
		found = true
		if slot.Guidance != cycleGuidance || len(slot.Facts) != 1 {
			t.Errorf("cycle slot = %+v, want the cycle guidance and the brief's sentence as its fact", slot)
		}
	}
	if !found {
		t.Error("no slot explains the cycle slide")
	}
}

// opsRhythmBrief is the j-ops-operating-rhythm persona's brief
// (tests/quality/journey/personas): the loop is named in one sentence, its
// intake steps listed in a second, its phases in a third, and a fourth is
// about one of the phases.
const opsRhythmBrief = "Build the operations review for \"Harbourline Logistics\" for the COO. 6–7 slides on `midnight-blue`. Our operating rhythm is a six-phase continuous improvement loop fed by a two-step onboarding intake. Intake: sign the service contract, onboard the site. The monthly loop: plan the month, run the service, measure against the SLA, review with the client, fix root causes, reset the targets. KPIs: on-time delivery 96.5%, rework −34%, sites onboarded 14, reviews held on time 4 of 9. The review phase is the weak one: it slipped past month end in 5 of the last 9 months. Ask: approve a dedicated review lead from November."

// Everything a brief says about its loop lands on one cycle slide: the brief's
// own phases and intake steps instead of placeholders, the phase it singles
// out as the highlight, and none of it left unplaced or drafted as a roadmap.
func TestPlanDeckMergesTheLoopIntoOneCycleSlide(t *testing.T) {
	plan := BuildDeckSpecPlan(Params{Brief: opsRhythmBrief})
	cycles, kinds := 0, map[any]int{}
	for _, s := range plan.DeckSpec.Slides {
		kinds[s["kind"]]++
		if s["kind"] == "cycle" {
			cycles++
		}
	}
	if cycles != 1 || kinds["roadmap"] != 0 {
		t.Fatalf("kinds = %v, want one cycle slide and no roadmap", kinds)
	}
	s := cycleSlide(t, opsRhythmBrief)
	phases, _ := s["phases"].([]any)
	intake, _ := s["intake"].([]any)
	if s["style"] != "intake" || len(phases) != 6 || len(intake) != 2 {
		t.Fatalf("cycle slide = %v", s)
	}
	if phases[0] != "Plan the month" || intake[1] != "Onboard the site" || s["highlight"] != "Review with the client" {
		t.Errorf("cycle slide = %v, want the brief's own phases, intake steps and the review phase highlighted", s)
	}
	if len(plan.UnplacedFacts) != 0 {
		t.Errorf("unplaced facts: %v", plan.UnplacedFacts)
	}
	if kinds["kpi_snapshot"] != 1 || kinds["next_steps"] != 1 {
		t.Errorf("kinds = %v, want the KPIs and the closer beside the loop", kinds)
	}
}

// An intake list with no loop beside it is not a cycle slide.
func TestPlanDeckIntakeListAloneIsNotACycle(t *testing.T) {
	brief := "Client onboarding review for the COO. Onboarding: sign the contract, migrate the data, train the users. Time to first value fell from 9 weeks to 4."
	if s := cycleSlide(t, brief); s != nil {
		t.Errorf("drafted a cycle slide from an intake list alone: %v", s)
	}
}

// Loop words in ordinary prose do not draft a cycle slide.
func TestPlanDeckCycleCuesIgnoreProse(t *testing.T) {
	for _, brief := range []string{
		"Q3 board update. The sales cycle shortened from 90 to 60 days. Annual recurring revenue grew 41% to $48M.",
		"Board update on the ERP migration. Keep the steering committee in the loop on cutover risk. Cycle time fell 12%.",
		"Platform review for the CTO. Our target architecture has five tiers and two rails. Process: discover, design, build, test, launch.",
		"Investor pitch: problem, solution, market size chart, traction KPIs, team, the ask.",
	} {
		if s := cycleSlide(t, brief); s != nil {
			t.Errorf("brief %q drafted a cycle slide: %v", brief, s)
		}
	}
}

// An outline item that names a loop is a cycle slide.
func TestOutlineItemNamesCycle(t *testing.T) {
	for _, item := range []string{"the customer lifecycle", "growth flywheel", "hub-and-spoke operating model"} {
		if got := outlineKindFor(item); got.kind != "cycle" {
			t.Errorf("outlineKindFor(%q) = %s, want cycle", item, got.kind)
		}
	}
	if got := outlineKindFor("the sales process"); got.kind == "cycle" {
		t.Errorf("a process item drafted a cycle")
	}
}
