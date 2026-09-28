package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-waft9: plan_deck format:"deckspec" drafts the DeckSpec the
// default path authors — kinds per narrative slot, not raw pattern skeletons.
func TestPlanDeckDeckSpecFormat(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	brief := "Board update on SMB churn. Churn rose to 4.2% in Q2, NRR fell to 108%, and the EU expansion slipped a quarter. We propose a dedicated success pod."
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{
		"brief": brief, "slide_budget": float64(9), "format": "deckspec",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("plan_deck deckspec failed: %v %s", err, textContent(res))
	}
	var plan deckplan.DeckSpecPlan
	if err := json.Unmarshal([]byte(textContent(res)), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Format != "deckspec" || len(plan.DeckSpec.Slides) != 9 || len(plan.Slots) != 9 {
		t.Fatalf("format=%q slides=%d slots=%d, want deckspec/9/9", plan.Format, len(plan.DeckSpec.Slides), len(plan.Slots))
	}
	var slots []string
	for i, slot := range plan.Slots {
		slots = append(slots, slot.Slot)
		kind, _ := plan.DeckSpec.Slides[i]["kind"].(string)
		if kind != slot.Kind {
			t.Errorf("slide %d kind %q != slot kind %q", i, kind, slot.Kind)
		}
		if !semantic.SlideKind(kind).Valid() {
			t.Errorf("slide %d uses unknown DeckSpec kind %q", i, kind)
		}
		if slot.Guidance == "" {
			t.Errorf("slot %d has no guidance", i)
		}
	}
	want := "cover answer problem cause evidence plan roadmap ask closing"
	if got := strings.Join(slots, " "); got != want {
		t.Errorf("storyline = %q, want %q", got, want)
	}
	// The brief's numbers land on the problem slide (several -> KPI cards).
	problem := plan.Slots[2]
	if problem.Kind != "kpi_snapshot" || len(problem.Facts) < 2 {
		t.Errorf("problem slot = %+v, want kpi_snapshot carrying the churn/NRR facts", problem)
	}
	if plan.UnplacedFacts == nil {
		t.Error("unplaced_facts must always be present")
	}
}

func TestPlanDeckDeckSpecSmallBudgetKeepsTheAsk(t *testing.T) {
	plan := deckplan.BuildDeckSpecPlan(deckplan.Params{Brief: "Approve the pilot budget", SlideBudget: 3})
	var kinds []string
	for _, s := range plan.Slots {
		kinds = append(kinds, s.Kind)
	}
	if got := strings.Join(kinds, " "); got != "title decision next_steps" {
		t.Errorf("3-slide storyline = %q, want the cover, the ask and the close", got)
	}
	big := deckplan.BuildDeckSpecPlan(deckplan.Params{Brief: "Strategy review", SlideBudget: 30})
	if len(big.Slots) >= 30 || big.BudgetNote == "" {
		t.Errorf("a 30-slide budget should not be padded with evidence slides: %d slots, note %q", len(big.Slots), big.BudgetNote)
	}
}

func TestPlanDeckRejectsUnknownFormat(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{"brief": "x", "format": "pptx"}))
	if err != nil || res == nil || !res.IsError || !strings.Contains(textContent(res), "format") {
		t.Fatalf("unknown format should be an INVALID_PARAMETER on format: %v %s", err, textContent(res))
	}
}
