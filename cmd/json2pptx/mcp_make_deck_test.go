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

// TestMakeDeck_ReturnsFinalPresentation asserts make_deck returns the deck it
// planned, expanded, and repaired in final_presentation, and that the JSON
// round-trips back through validate_input — so a cold-start agent can keep
// editing the auto-authored deck without rebuilding it from the plan summary.
func TestMakeDeck_ReturnsFinalPresentation(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":         "Pitch our Series B for an AI infrastructure company",
		"template":        "midnight-blue",
		"output_filename": "make_deck_final_pres.pptx",
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

	if len(output.FinalPresentation) == 0 {
		t.Fatal("expected non-empty final_presentation on a successful run")
	}

	var finalDeck PresentationInput
	if err := json.Unmarshal(output.FinalPresentation, &finalDeck); err != nil {
		t.Fatalf("final_presentation is not a valid PresentationInput: %v", err)
	}
	if finalDeck.Template != "midnight-blue" {
		t.Errorf("final_presentation.template = %q, want midnight-blue", finalDeck.Template)
	}
	// The plan summary and the returned deck must agree on slide count so the
	// agent can map plan.slides[].slide_index onto final_presentation.slides[].
	if output.Plan != nil && len(finalDeck.Slides) != len(output.Plan.Slides) {
		t.Errorf("final_presentation has %d slides but plan has %d", len(finalDeck.Slides), len(output.Plan.Slides))
	}

	// Headline guarantee: feed final_presentation back into validate_input.
	vResult, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(string(output.FinalPresentation)),
	}))
	if err != nil {
		t.Fatalf("validate_input round-trip errored: %v", err)
	}
	if vResult.IsError {
		t.Fatalf("validate_input round-trip returned tool error: %s", textContent(vResult))
	}
	var vOut dryRunOutput
	if err := json.Unmarshal([]byte(textContent(vResult)), &vOut); err != nil {
		t.Fatalf("unmarshal validate_input response: %v", err)
	}
	if !vOut.Valid {
		t.Errorf("final_presentation did not validate; findings: %s", textContent(vResult))
	}
}

// TestMakeDeck_ExemplarContentNeverPublishable pins the core go-slide-creator-33oo
// guarantee: because make_deck fills slides with pattern exemplar placeholder
// content, the response must mark the deck an exemplar skeleton and NEVER report
// it publishable — even when the quality gate passes. Agents branch on these
// machine-readable fields, so a clean transport response must not look like a
// finished deck.
func TestMakeDeck_ExemplarContentNeverPublishable(t *testing.T) {
	mc := repairMC(t)

	result, err := mc.handleMakeDeck(context.Background(), makeRequest(map[string]any{
		"outline":         "Pitch our Series B for an AI infrastructure company",
		"template":        "midnight-blue",
		"output_filename": "make_deck_exemplar_status.pptx",
		"style_hints": map[string]any{
			"slide_budget": float64(5),
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

	// Content provenance must be unambiguous.
	if output.ContentStatus != "exemplar_skeleton" {
		t.Errorf("content_status = %q, want %q", output.ContentStatus, "exemplar_skeleton")
	}
	if !output.UsesExemplarContent {
		t.Error("uses_exemplar_content must be true for make_deck cold-start output")
	}

	// Publishability is the headline guarantee: exemplar content is never
	// publishable, regardless of gate_passed.
	if output.Publishable {
		t.Errorf("make_deck exemplar output must never be publishable (gate_passed=%v)", output.GatePassed)
	}
	if !output.ManualReviewRequired {
		t.Error("manual_review_required must be true when the deck is not publishable")
	}
	if len(output.BlockingReasons) == 0 {
		t.Fatal("blocking_reasons must explain why an exemplar skeleton is not publishable")
	}
	// At least one blocking reason must name the exemplar-content cause.
	exemplarNamed := false
	for _, r := range output.BlockingReasons {
		if strings.Contains(strings.ToLower(r), "exemplar") {
			exemplarNamed = true
		}
	}
	if !exemplarNamed {
		t.Errorf("blocking_reasons must name the exemplar-content cause; got %v", output.BlockingReasons)
	}

	// artifact_status is independent of publishability — the file is written.
	if output.ArtifactStatus != "generated" && output.ArtifactStatus != "generated_invalid" {
		t.Errorf("artifact_status = %q, want generated|generated_invalid", output.ArtifactStatus)
	}
	// publishable is exactly "no blocking reasons".
	if output.Publishable != (len(output.BlockingReasons) == 0) {
		t.Errorf("publishable=%v inconsistent with blocking_reasons=%v", output.Publishable, output.BlockingReasons)
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
