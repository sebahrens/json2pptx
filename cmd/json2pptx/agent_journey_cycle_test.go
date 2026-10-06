package main

import (
	"os"
	"testing"
)

// opsRhythmJourneyBrief is the brief of the j-ops-operating-rhythm persona
// (tests/quality/journey/personas/j-ops-operating-rhythm.md, the blockquote).
const opsRhythmJourneyBrief = "Build the operations review for \"Harbourline Logistics\" for the COO. 6–7 slides on `midnight-blue`. Our operating rhythm is a six-phase continuous improvement loop fed by a two-step onboarding intake. Intake: sign the service contract, onboard the site. The monthly loop: plan the month, run the service, measure against the SLA, review with the client, fix root causes, reset the targets. KPIs: on-time delivery 96.5%, rework −34%, sites onboarded 14, reviews held on time 4 of 9. The review phase is the weak one: it slipped past month end in 5 of the last 9 months. Ask: approve a dedicated review lead from November."

// TestOpsLeadOperatingRhythmJourney is the deterministic half of the
// j-ops-operating-rhythm persona (go-slide-creator-9d1mp): plan_deck drafts the
// loop as a cycle slide in the intake style with the brief's own phases, and
// that slide — untouched but for its title — validates and renders ready, with
// the same findings from both tools, on the two templates the journey uses.
func TestOpsLeadOperatingRhythmJourney(t *testing.T) {
	mc := refusalTestConfig(t)
	var plan struct {
		DeckSpec struct {
			Slides []map[string]any `json:"slides"`
		} `json:"deck_spec"`
		Slots []struct {
			Kind     string   `json:"kind"`
			Guidance string   `json:"guidance"`
			Facts    []string `json:"facts"`
		} `json:"slots"`
		UnplacedFacts []string `json:"unplaced_facts"`
	}
	structuredInto(t, mustCall(t, mc.handlePlanDeck, map[string]any{"brief": opsRhythmJourneyBrief, "format": "deckspec"}).StructuredContent, &plan)

	var cycle map[string]any
	for _, s := range plan.DeckSpec.Slides {
		if s["kind"] == "cycle" {
			if cycle != nil {
				t.Fatalf("plan_deck drafted two cycle slides: %v", plan.DeckSpec.Slides)
			}
			cycle = s
		}
	}
	if cycle == nil {
		t.Fatalf("plan_deck drafted no cycle slide: %v", plan.DeckSpec.Slides)
	}
	phases, _ := cycle["phases"].([]any)
	intake, _ := cycle["intake"].([]any)
	if cycle["style"] != "intake" || len(phases) != 6 || len(intake) != 2 || cycle["highlight"] != "Review with the client" {
		t.Fatalf("drafted cycle slide = %v, want style intake, six phases, two intake steps and the review phase highlighted", cycle)
	}
	if len(plan.UnplacedFacts) != 0 {
		t.Errorf("unplaced facts: %v", plan.UnplacedFacts)
	}
	guided := false
	for _, slot := range plan.Slots {
		guided = guided || (slot.Kind == "cycle" && slot.Guidance != "" && len(slot.Facts) > 0)
	}
	if !guided {
		t.Errorf("no slot explains the cycle slide with guidance and facts: %+v", plan.Slots)
	}

	// The agent's edit: a title and a takeaway on the drafted slide, and the
	// slides around it.
	cycle["title"] = "Two onboarding steps feed a monthly loop of six phases"
	cycle["takeaway"] = "Review is the phase that slips: 5 of the last 9 months."
	spec := map[string]any{
		"meta": map[string]any{"title": "Harbourline Logistics operations review", "date": "October 2026", "source": "Harbourline service data, January–September 2026"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Harbourline Logistics operations review", "subtitle": "For the COO, October 2026"},
			map[string]any{"kind": "executive_summary", "title": "The operating rhythm works; the review phase needs an owner",
				"points": []any{
					map[string]any{"lead": "Delivery is on target.", "support": "On-time delivery reached 96.5% and rework fell 34%."},
					map[string]any{"lead": "Onboarding scales.", "support": "14 sites came through the two intake steps this year."},
					map[string]any{"lead": "Review slips.", "support": "It ran past month end in 5 of the last 9 months."},
				},
				"bottom_line": "Approve a dedicated review lead from November."},
			cycle,
			map[string]any{"kind": "kpi_snapshot", "title": "On-time delivery is at 96.5% and rework is down 34%",
				"kpis": []any{
					map[string]any{"value": "96.5%", "label": "On-time delivery"},
					map[string]any{"value": "−34%", "label": "Rework"},
					map[string]any{"value": "14", "label": "Sites onboarded"},
					map[string]any{"value": "4 of 9", "label": "Reviews held on time"},
				},
				"takeaway": "Three of four measures are on target; review timeliness is not."},
			map[string]any{"kind": "next_steps", "title": "One decision puts an owner on the review phase",
				"actions": []any{
					map[string]any{"action": "Appoint the review lead", "owner": "COO", "date": "1 Nov"},
					map[string]any{"action": "Move reviews to the first week of the month", "owner": "Ops lead", "date": "15 Nov"},
				},
				"decisions": []any{"Approve a dedicated review lead from November"}},
		},
	}

	templates := []string{"midnight-blue", "warm-coral"}
	if _, err := os.Stat("../../templates/p-style.pptx"); err == nil {
		templates[1] = "p-style" // the journey's second template, when it is installed
	}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			v := deckSpecVerdicts(t, mc, map[string]any{"spec": spec, "template": tpl})
			assertFindingParity(t, tpl, v)
			if !v.Validate.OK {
				t.Errorf("validate is not ok: %+v", v.Validate.Findings)
			}
			for _, f := range v.Validate.Findings {
				if f.Severity == "error" {
					t.Errorf("blocking finding %s: %s", f.Code, f.Message)
				}
			}
			if v.Render.DeterministicReady == nil || !*v.Render.DeterministicReady {
				t.Errorf("render is not deterministic_ready: %q %v", v.Render.Error, v.Render.DeterministicBlockingReasons)
			}
		})
	}
}
