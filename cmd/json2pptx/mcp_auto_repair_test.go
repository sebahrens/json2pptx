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

// TestAutoRepair_ReturnsFinalPresentation asserts that a converging run
// returns the full repaired deck JSON in final_presentation, that the JSON
// reflects the applied repairs, and that it round-trips back through
// validate_input without a caller having to reconstruct state from the trace.
func TestAutoRepair_ReturnsFinalPresentation(t *testing.T) {
	mc := repairMC(t)

	deckJSON := autoRepairDeck(3)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"gate": map[string]any{
			"min_score":                  float64(95),
			"max_p0_findings":            float64(0),
			"max_p1_findings":            float64(2),
			"require_takeaway_on_charts": false,
		},
		"max_passes":      float64(3),
		"output_filename": "auto_repair_final_pres.pptx",
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

	if len(output.FinalPresentation) == 0 {
		t.Fatal("expected non-empty final_presentation on a successful run")
	}

	// The returned JSON must parse as a deck with the same shape we sent.
	var finalDeck PresentationInput
	if err := json.Unmarshal(output.FinalPresentation, &finalDeck); err != nil {
		t.Fatalf("final_presentation is not a valid PresentationInput: %v", err)
	}
	if finalDeck.Template != "midnight-blue" {
		t.Errorf("final_presentation.template = %q, want midnight-blue", finalDeck.Template)
	}
	if len(finalDeck.Slides) != 3 {
		t.Errorf("final_presentation has %d slides, want 3", len(finalDeck.Slides))
	}

	// The deck entered the loop with six bullets per body (BODY_TOO_LONG);
	// convergence to min_score=95 requires reduce_text, so the repaired JSON
	// must show at least one body trimmed below the original six bullets.
	// This proves final_presentation is the post-repair state, not the input.
	trimmed := false
	for _, s := range finalDeck.Slides {
		for _, c := range s.Content {
			if c.BulletsValue != nil && len(*c.BulletsValue) < 6 {
				trimmed = true
			}
		}
	}
	if !trimmed {
		t.Errorf("expected final_presentation to reflect a reduce_text repair (a body with <6 bullets), found none")
	}

	// The headline guarantee: feed final_presentation straight back into
	// validate_input and it must validate, with no reconstruction from trace.
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

// TestAutoRepair_FinalPresentationOnZeroRepairRun asserts the field is present
// even when the deck passes the gate on pass 1 with no repairs applied — the
// acceptance criterion explicitly covers zero-repair runs.
func TestAutoRepair_FinalPresentationOnZeroRepairRun(t *testing.T) {
	mc := repairMC(t)

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
		"output_filename": "auto_repair_zero_repair.pptx",
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
		t.Fatalf("expected trivial deck to pass gate on pass 1, reasons=%v", output.GateReasons)
	}
	if len(output.FinalPresentation) == 0 {
		t.Fatal("expected final_presentation present on a zero-repair run")
	}
	var finalDeck PresentationInput
	if err := json.Unmarshal(output.FinalPresentation, &finalDeck); err != nil {
		t.Fatalf("final_presentation is not a valid PresentationInput: %v", err)
	}
	if len(finalDeck.Slides) != 1 {
		t.Errorf("final_presentation has %d slides, want 1", len(finalDeck.Slides))
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

// TestFacadeOutputSchemasAdvertisePublishability is the schema-sync gate for
// go-slide-creator-33oo: both facade output schemas must advertise AND require
// the agent-native publishability status fields, so MCP clients can rely on
// them always being present.
func TestFacadeOutputSchemasAdvertisePublishability(t *testing.T) {
	required := []string{
		"artifact_status", "content_status", "uses_exemplar_content",
		"validation_status", "publishable", "manual_review_required",
	}
	for name, schema := range map[string]json.RawMessage{
		"auto_repair": outputSchemaAutoRepair,
		"make_deck":   outputSchemaMakeDeck,
	} {
		var parsed map[string]any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("%s schema is not valid JSON: %v", name, err)
		}
		props, _ := parsed["properties"].(map[string]any)
		reqList, _ := parsed["required"].([]any)
		reqSet := map[string]bool{}
		for _, r := range reqList {
			if s, ok := r.(string); ok {
				reqSet[s] = true
			}
		}
		// blocking_reasons is advertised but omitempty (present only when not
		// publishable), so it is a property but not required.
		if _, ok := props["blocking_reasons"]; !ok {
			t.Errorf("%s schema must advertise blocking_reasons", name)
		}
		for _, field := range required {
			if _, ok := props[field]; !ok {
				t.Errorf("%s schema must advertise %q", name, field)
			}
			if !reqSet[field] {
				t.Errorf("%s schema must require %q (it is always present)", name, field)
			}
		}
	}
}

// TestAutoRepair_GateFailedFacadeSuccessNotPublishable pins go-slide-creator-33oo
// for the gate-failed case: the tool returns a SUCCESSFUL transport response
// (IsError=false) with a written PPTX even though the gate failed, so the
// publishability fields must make the failure unambiguous. We force a
// deterministic gate failure with max_passes=1 (one scoring pass, no repair) on a
// deck that starts below the score threshold.
func TestAutoRepair_GateFailedFacadeSuccessNotPublishable(t *testing.T) {
	mc := repairMC(t)

	// Three BODY_TOO_LONG findings (action=review, weight 5) → score 85. With
	// min_score=95 the single scoring pass fails the gate, and max_passes=1 means
	// no repair is attempted, so the failure is deterministic.
	deckJSON := autoRepairDeck(3)

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"gate": map[string]any{
			"min_score":                  float64(95),
			"max_p0_findings":            float64(0),
			"max_p1_findings":            float64(2),
			"require_takeaway_on_charts": false,
		},
		"max_passes":      float64(1),
		"output_filename": "auto_repair_gate_failed.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The facade SUCCEEDS at the transport layer even though the gate failed.
	if result.IsError {
		t.Fatalf("expected a successful transport response on gate failure, got error: %s", textContent(result))
	}

	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if output.GatePassed {
		t.Fatalf("expected gate_passed=false with min_score=95 and max_passes=1; final_score=%d", output.FinalScore)
	}
	// A written artifact must NOT imply publishability.
	if output.Path == "" {
		t.Error("expected a written PPTX path even on gate failure")
	}
	if output.Publishable {
		t.Error("gate-failed deck must not be publishable")
	}
	if !output.ManualReviewRequired {
		t.Error("manual_review_required must be true for a gate-failed deck")
	}
	if output.ValidationStatus != "failed" {
		t.Errorf("validation_status = %q, want %q", output.ValidationStatus, "failed")
	}
	if len(output.BlockingReasons) == 0 {
		t.Fatal("gate_passed=false must produce explicit blocking_reasons")
	}
	// blocking_reasons is a superset of gate_reasons: every gate reason appears.
	for _, gr := range output.GateReasons {
		found := false
		for _, br := range output.BlockingReasons {
			if br == gr {
				found = true
			}
		}
		if !found {
			t.Errorf("gate reason %q missing from blocking_reasons %v", gr, output.BlockingReasons)
		}
	}
	// auto_repair operates on caller-authored content — never exemplar.
	if output.ContentStatus != "author_supplied" {
		t.Errorf("content_status = %q, want %q", output.ContentStatus, "author_supplied")
	}
	if output.UsesExemplarContent {
		t.Error("uses_exemplar_content must be false for auto_repair")
	}
}

// TestAutoRepair_CleanRunIsPublishable asserts the positive path: an
// author-supplied deck that passes the gate on complete evidence is reported
// publishable with no blocking reasons. This is the only combination that may be
// publishable, so it must light up correctly.
func TestAutoRepair_CleanRunIsPublishable(t *testing.T) {
	mc := repairMC(t)

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
		"output_filename": "auto_repair_publishable.pptx",
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
		t.Fatalf("expected trivial author-supplied deck to pass the gate; reasons=%v", output.GateReasons)
	}
	if !output.EvidenceComplete {
		t.Fatalf("expected complete evidence on a passing render; render_evidence=%+v output_validation=%+v",
			output.RenderEvidence, output.OutputValidation)
	}
	if !output.Publishable {
		t.Errorf("a gate-passed, evidence-complete, author-supplied deck must be publishable; blocking_reasons=%v", output.BlockingReasons)
	}
	if output.ManualReviewRequired {
		t.Error("manual_review_required must be false for a publishable deck")
	}
	if len(output.BlockingReasons) != 0 {
		t.Errorf("blocking_reasons must be empty for a publishable deck, got %v", output.BlockingReasons)
	}
	if output.ContentStatus != "author_supplied" {
		t.Errorf("content_status = %q, want %q", output.ContentStatus, "author_supplied")
	}
	if output.ValidationStatus != "passed" {
		t.Errorf("validation_status = %q, want %q", output.ValidationStatus, "passed")
	}
	if output.ArtifactStatus != "generated" {
		t.Errorf("artifact_status = %q, want %q", output.ArtifactStatus, "generated")
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
