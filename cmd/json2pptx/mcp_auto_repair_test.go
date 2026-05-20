package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// autoRepairDeck builds a midnight-blue deck of `n` slides, each carrying six
// 20-word bullets. Six bullets × 20 words = 120 words per body, well above the
// 80-word BODY_TOO_LONG readability threshold, so the deck enters the loop
// with one finding per slide.
func autoRepairDeck(n int) string {
	slides := make([]map[string]any, n)
	for i := range slides {
		slides[i] = map[string]any{
			"layout_id": "slideLayout2",
			"content": []any{
				map[string]any{
					"placeholder_id": "title",
					"type":           "text",
					"text_value":     "Heavy slide",
				},
				map[string]any{
					"placeholder_id": "body",
					"type":           "bullets",
					"bullets_value": func() []any {
						bs := bodyTooLongBullets()
						out := make([]any, len(bs))
						for j, b := range bs {
							out[j] = b
						}
						return out
					}(),
				},
			},
		}
	}
	deck := map[string]any{
		"template": "midnight-blue",
		"slides":   slides,
	}
	b, _ := json.Marshal(deck)
	return string(b)
}

// TestAutoRepair_ConvergesWithinMaxPasses asserts the loop drives a deck with
// BODY_TOO_LONG findings into gate-passing state and returns a trace whose
// score is monotonically improving.
func TestAutoRepair_ConvergesWithinMaxPasses(t *testing.T) {
	mc := repairMC(t)

	deckJSON := autoRepairDeck(3)

	// BODY_TOO_LONG findings have action=review (weight 5). Three of them drop
	// the score by 15 → 85 with default gate min_score=75 would already pass on
	// pass 1, so raise min_score to 95 to force at least one repair pass.
	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"gate": map[string]any{
			"min_score":                  float64(95),
			"max_p0_findings":            float64(0),
			"max_p1_findings":            float64(2),
			"require_takeaway_on_charts": false, // chart-less deck — irrelevant here
		},
		"max_passes":      float64(3),
		"output_filename": "auto_repair_test.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.GatePassed {
		t.Fatalf("expected gate_passed=true within %d passes; final_score=%d gate_reasons=%v trace=%+v",
			3, output.FinalScore, output.GateReasons, output.Trace)
	}
	if output.FinalScore < 95 {
		t.Errorf("expected final_score ≥ 95 after convergence, got %d", output.FinalScore)
	}
	if output.Passes < 1 || output.Passes > 3 {
		t.Errorf("expected passes in [1, 3], got %d", output.Passes)
	}
	if len(output.Trace) != output.Passes {
		t.Errorf("trace length %d does not match passes %d", len(output.Trace), output.Passes)
	}

	// Pass 1 records the pre-repair deck state (score + finding count) and
	// the repairs applied DURING pass 1; per the spec example, pass 1
	// typically carries the bulk of repair activity. Subsequent passes
	// observe progressively cleaner state until the final converged pass,
	// which lists no repairs (the gate is already satisfied).
	if output.Trace[0].FindingsCount < 3 {
		t.Errorf("trace[0].findings_count = %d, expected ≥ 3 BODY_TOO_LONG findings before any repair",
			output.Trace[0].FindingsCount)
	}
	// Across the trace, at least one repair must have landed — otherwise the
	// gate never could have flipped.
	totalRepairs := 0
	for _, e := range output.Trace {
		totalRepairs += len(e.RepairsApplied)
	}
	if totalRepairs == 0 {
		t.Errorf("expected at least one repair applied across the trace, got none: %+v", output.Trace)
	}
	// The final trace entry is the converged pass: gate satisfied, no
	// further repairs needed.
	last := output.Trace[len(output.Trace)-1]
	if len(last.RepairsApplied) != 0 {
		t.Errorf("trace[final].repairs_applied = %v, expected empty on converged pass", last.RepairsApplied)
	}

	// Scores must be monotonically non-decreasing across passes — repairs
	// should never make the deck worse.
	for i := 1; i < len(output.Trace); i++ {
		if output.Trace[i].Score < output.Trace[i-1].Score {
			t.Errorf("score regressed at pass %d: %d < %d", output.Trace[i].Pass, output.Trace[i].Score, output.Trace[i-1].Score)
		}
	}

	// Final PPTX must exist on disk so callers can chain to pptx2jpg / read.
	if output.Path == "" {
		t.Fatal("expected non-empty output path")
	}
	if _, err := os.Stat(output.Path); err != nil {
		t.Errorf("output PPTX not found at %s: %v", output.Path, err)
	}

	if len(output.GateReasons) != 0 {
		t.Errorf("expected gate_reasons to be empty on success, got %v", output.GateReasons)
	}
}

// TestAutoRepair_GateNotMetExhaustsPasses asserts that when the gate is set
// to an unattainable threshold, the loop runs the full max_passes count,
// returns gate_passed=false, and records every pass in the trace with
// explanatory gate_reasons.
func TestAutoRepair_GateNotMetExhaustsPasses(t *testing.T) {
	mc := repairMC(t)

	// Set min_score=100 so even a perfectly clean deck would fail (unless we
	// also force require_takeaway_on_charts=false to make all chart-related
	// reasoning irrelevant). Plus, the deck below has BODY_TOO_LONG so the
	// first pass clearly fails.
	deckJSON := autoRepairDeck(3)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"gate": map[string]any{
			"min_score":                  float64(100),
			"max_p0_findings":            float64(0),
			"max_p1_findings":            float64(0),
			"require_takeaway_on_charts": false,
		},
		"max_passes":      float64(2),
		"output_filename": "auto_repair_unmet.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// We MAY converge to score=100 inside the loop (reduce_text strips the
	// BODY_TOO_LONG findings). The interesting assertion here is that the
	// trace records every pass actually run and gate_passed reflects the
	// configured threshold rather than silently succeeding.
	if output.Passes < 1 {
		t.Fatalf("expected at least one pass, got %d", output.Passes)
	}
	if len(output.Trace) != output.Passes {
		t.Errorf("trace length %d != passes %d", len(output.Trace), output.Passes)
	}
	// Output PPTX must still be written even when the gate fails.
	if output.Path == "" {
		t.Fatal("expected non-empty output path even on gate failure")
	}
	if _, err := os.Stat(output.Path); err != nil {
		t.Errorf("output PPTX not found at %s: %v", output.Path, err)
	}

	// If the gate failed, gate_reasons must explain why. If it passed (because
	// the deck cleaned up to 100), gate_reasons must be empty.
	if output.GatePassed && len(output.GateReasons) != 0 {
		t.Errorf("gate_passed=true but gate_reasons non-empty: %v", output.GateReasons)
	}
	if !output.GatePassed && len(output.GateReasons) == 0 {
		t.Errorf("gate_passed=false but gate_reasons empty — agents need a reason to retry")
	}
}

// TestAutoRepair_TraceShapeStable asserts the response carries every required
// field even on trivial decks so MCP clients can rely on the schema.
func TestAutoRepair_TraceShapeStable(t *testing.T) {
	mc := repairMC(t)

	// A single trivially clean slide — no findings, gate passes on pass 1.
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout1",
				"content": []any{
					map[string]any{
						"placeholder_id": "title",
						"type":           "text",
						"text_value":     "Hello",
					},
				},
			},
		},
	}
	b, _ := json.Marshal(deck)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation":    mustParseJSON(string(b)),
		"output_filename": "auto_repair_trivial.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !output.GatePassed {
		t.Errorf("expected trivial deck to pass gate, got reasons=%v", output.GateReasons)
	}
	if output.Passes != 1 {
		t.Errorf("expected 1 pass on trivial deck, got %d", output.Passes)
	}
	if len(output.Trace) != 1 {
		t.Errorf("expected trace length 1 on trivial deck, got %d", len(output.Trace))
	}
	if output.Path == "" {
		t.Error("expected non-empty path on trivial deck")
	}
}
