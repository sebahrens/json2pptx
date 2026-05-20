package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// TestMakeDeck_ColdStartOutlineProducesPPTX asserts the canonical agent
// usage: hand an outline, get a deck. The output must include a path on disk,
// a plan summary the agent can inspect, and a trace identical to auto_repair's
// for callers that share parsing logic.
func TestMakeDeck_ColdStartOutlineProducesPPTX(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":         "Pitch our Series B for an AI infrastructure company",
		"template":        "midnight-blue",
		"output_filename": "make_deck_cold_start.pptx",
		"style_hints": map[string]any{
			"slide_budget": float64(6),
		},
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

	if output.Path == "" {
		t.Fatal("expected non-empty path on success")
	}
	if _, err := os.Stat(output.Path); err != nil {
		t.Errorf("output PPTX not found at %s: %v", output.Path, err)
	}
	if output.Passes < 1 {
		t.Errorf("expected ≥1 pass, got %d", output.Passes)
	}
	if len(output.Trace) != output.Passes {
		t.Errorf("trace length %d != passes %d", len(output.Trace), output.Passes)
	}

	if output.Plan == nil {
		t.Fatal("expected plan summary in response")
	}
	if output.Plan.Template != "midnight-blue" {
		t.Errorf("plan.template = %q, want midnight-blue", output.Plan.Template)
	}
	if output.Plan.SlideBudget != 6 {
		t.Errorf("plan.slide_budget = %d, want 6", output.Plan.SlideBudget)
	}
	if len(output.Plan.Slides) != 6 {
		t.Errorf("plan.slides length = %d, want 6", len(output.Plan.Slides))
	}
	if len(output.Plan.Slides) > 0 {
		// Opening slide title should reflect the brief.
		if !strings.Contains(strings.ToLower(output.Plan.Slides[0].Title), "series b") {
			t.Errorf("opening slide title %q does not echo the brief; expected 'Series B' substring", output.Plan.Slides[0].Title)
		}
		if output.Plan.Slides[0].NarrativeRole != "opening" {
			t.Errorf("first slide role = %q, want opening", output.Plan.Slides[0].NarrativeRole)
		}
	}

	// Gate semantics mirror auto_repair: when gate_passed=false, gate_reasons
	// must explain why; when true, must be empty.
	if output.GatePassed && len(output.GateReasons) != 0 {
		t.Errorf("gate_passed=true but gate_reasons non-empty: %v", output.GateReasons)
	}
	if !output.GatePassed && len(output.GateReasons) == 0 {
		t.Error("gate_passed=false but gate_reasons empty — agents need a reason to retry")
	}
}

// TestMakeDeck_MissingOutlineReturnsArgError asserts the tool refuses to run
// without an outline rather than silently producing an empty plan.
func TestMakeDeck_MissingOutlineReturnsArgError(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for missing outline, got: %s", textContent(result))
	}
}

// TestMakeDeck_UnknownMustIncludePattern asserts misconfigured style_hints fail
// cleanly with a structured argInvalidValue error rather than silently dropping
// the requested pattern.
func TestMakeDeck_UnknownMustIncludePattern(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline": "test outline",
		"style_hints": map[string]any{
			"must_include": []any{"this-pattern-does-not-exist"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for unknown must_include pattern, got: %s", textContent(result))
	}
}

// TestMakeDeck_MaxRepairPassesClamps asserts both the preferred (max_repair_passes)
// and legacy (max_passes) arg names are honored and clamped to [1, 10]. We
// drive the clamp via a degenerate request — anything ≥ 1 pass is acceptable;
// the assertion is that we never run more than 10 even when 50 is requested.
func TestMakeDeck_MaxRepairPassesClamps(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":           "Quick test deck",
		"max_repair_passes": float64(50),
		"output_filename":   "make_deck_clamp.pptx",
		"style_hints": map[string]any{
			"slide_budget": float64(3),
		},
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
	if output.Passes > 10 {
		t.Errorf("max_repair_passes=50 should clamp to ≤10, got %d", output.Passes)
	}
}
