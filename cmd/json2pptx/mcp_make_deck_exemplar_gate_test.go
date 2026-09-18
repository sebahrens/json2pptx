package main

import (
	"context"
	"encoding/json"
	"testing"
)

// TestMakeDeck_ExemplarContentFailsGate asserts go-slide-creator-htwq: a
// make_deck deck filled with pattern exemplar placeholder copy must never
// report gate_passed=true or a high final_score, whatever the outline. The
// structural loop score survives as structural_score.
func TestMakeDeck_ExemplarContentFailsGate(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":         "Quarterly business review for a SaaS company: revenue growth, churn, H2 plan",
		"template":        "midnight-blue",
		"output_filename": "make_deck_exemplar_gate.pptx",
		"style_hints":     map[string]any{"slide_budget": float64(4)},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}
	var output makeDeckOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.GatePassed {
		t.Error("gate_passed must be false for exemplar-skeleton output")
	}
	if !containsString(output.BlockingReasons, exemplarContentReason) {
		t.Errorf("blocking_reasons must contain %q, got %v", exemplarContentReason, output.BlockingReasons)
	}
	if len(output.GateReasons) == 0 || output.GateReasons[0] != exemplarContentReason {
		t.Errorf("gate_reasons must lead with %q, got %v", exemplarContentReason, output.GateReasons)
	}
	if output.FinalScore != 0 {
		t.Errorf("final_score = %d, want 0 for exemplar content", output.FinalScore)
	}
	if output.ContentScore == nil || *output.ContentScore != 0 {
		t.Errorf("content_score = %v, want 0", output.ContentScore)
	}
	if output.StructuralScore <= 0 {
		t.Errorf("structural_score = %d, want the loop's positive structural score", output.StructuralScore)
	}
}

func TestApplyExemplarContentGate_Idempotent(t *testing.T) {
	out := &makeDeckOutput{FinalScore: 98, StructuralScore: 98, GatePassed: true, UsesExemplarContent: true, Publishable: true}
	applyExemplarContentGate(out)
	applyExemplarContentGate(out)
	if out.GatePassed || out.Publishable || !out.ManualReviewRequired {
		t.Fatalf("gate not failed: %+v", out)
	}
	if len(out.GateReasons) != 1 || len(out.BlockingReasons) != 1 {
		t.Fatalf("reason token duplicated: gate=%v blocking=%v", out.GateReasons, out.BlockingReasons)
	}
	if out.StructuralScore != 98 || out.FinalScore != 0 {
		t.Fatalf("scores = final %d structural %d", out.FinalScore, out.StructuralScore)
	}
}
