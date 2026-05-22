package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// TestAutoRepair_NextStatePresentOnConvergedRun asserts that a normal converging
// run carries a next_state block reporting a clean, non-resumable convergence —
// the shape MCP clients can always rely on.
func TestAutoRepair_NextStatePresentOnConvergedRun(t *testing.T) {
	mc := repairMC(t)

	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout1",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
				},
			},
		},
	}
	b, _ := json.Marshal(deck)

	output := runAutoRepairForTest(t, mc, map[string]any{
		"presentation":    mustParseJSON(string(b)),
		"output_filename": "auto_repair_next_state_converged.pptx",
	})

	if output.NextState == nil {
		t.Fatal("next_state must always be present")
	}
	if output.NextState.Completion != loopCompletionConverged {
		t.Errorf("completion = %q, want %q", output.NextState.Completion, loopCompletionConverged)
	}
	if output.NextState.Resumable {
		t.Error("a clean converged run must not be resumable")
	}
	if output.NextState.NextAction == "" {
		t.Error("next_action must be populated")
	}
	if output.NextState.PassesRun != output.Passes {
		t.Errorf("next_state.passes_run %d != passes %d", output.NextState.PassesRun, output.Passes)
	}
	if len(output.NextState.RemainingFindings) != 0 {
		t.Errorf("a converged run should have no remaining findings, got %v", output.NextState.RemainingFindings)
	}
	// A resume token is still minted so a caller can refine with a stricter gate.
	if output.NextState.ResumeToken == "" {
		t.Error("expected a resume_token even on a converged run (store is wired)")
	}
}

// TestAutoRepair_ResumeAfterDeterministicPass is the headline acceptance test:
// a first call that exhausts a one-pass budget without converging returns a
// resume token; a second call with that token continues from the saved deck,
// preserves the completed pass in the trace (does NOT re-run it), and converges.
func TestAutoRepair_ResumeAfterDeterministicPass(t *testing.T) {
	mc := repairMC(t)

	// autoRepairDeck(3): three BODY_TOO_LONG slides → score 85. min_score=95 with
	// max_passes=1 records pass 1 (score 85) and stops without converging.
	gate := map[string]any{
		"min_score":                  float64(95),
		"max_p0_findings":            float64(0),
		"max_p1_findings":            float64(2),
		"require_takeaway_on_charts": false,
	}
	first := runAutoRepairForTest(t, mc, map[string]any{
		"presentation":    mustParseJSON(autoRepairDeck(3)),
		"gate":            gate,
		"max_passes":      float64(1),
		"output_filename": "auto_repair_resume_first.pptx",
	})

	if first.GatePassed {
		t.Fatalf("expected the one-pass run not to converge at min_score=95; final_score=%d", first.FinalScore)
	}
	if first.NextState == nil || !first.NextState.Resumable {
		t.Fatalf("expected a resumable next_state, got %+v", first.NextState)
	}
	if first.NextState.Completion != loopCompletionExhausted {
		t.Errorf("completion = %q, want %q", first.NextState.Completion, loopCompletionExhausted)
	}
	if first.NextState.PassesRun != 1 || first.NextState.NextPass != 2 {
		t.Errorf("expected passes_run=1, next_pass=2; got passes_run=%d next_pass=%d", first.NextState.PassesRun, first.NextState.NextPass)
	}
	token := first.NextState.ResumeToken
	if token == "" {
		t.Fatal("expected a non-empty resume_token on a resumable result")
	}
	if len(first.Trace) != 1 {
		t.Fatalf("expected first-call trace length 1, got %d", len(first.Trace))
	}
	firstPassScore := first.Trace[0].Score

	// Resume: no presentation supplied — the deck comes from the checkpoint. Grant
	// more passes so it can converge. The gate is inherited from the session.
	resumed := runAutoRepairForTest(t, mc, map[string]any{
		"resume_token":    token,
		"max_passes":      float64(3),
		"output_filename": "auto_repair_resume_second.pptx",
	})

	if resumed.NextState == nil {
		t.Fatal("resumed run must carry next_state")
	}
	// The completed pass must be preserved (continued, not restarted): the trace
	// grows, global pass numbering continues, and pass 1's record is unchanged.
	if resumed.Passes <= first.Passes {
		t.Errorf("expected resumed passes (%d) to exceed the first call's (%d)", resumed.Passes, first.Passes)
	}
	if len(resumed.Trace) != resumed.Passes {
		t.Errorf("trace length %d != passes %d", len(resumed.Trace), resumed.Passes)
	}
	if resumed.Trace[0].Pass != 1 || resumed.Trace[0].Score != firstPassScore {
		t.Errorf("resume must preserve pass 1 (pass=%d score=%d), got pass=%d score=%d — completed pass was repeated/lost",
			1, firstPassScore, resumed.Trace[0].Pass, resumed.Trace[0].Score)
	}
	for i := 1; i < len(resumed.Trace); i++ {
		if resumed.Trace[i].Pass != i+1 {
			t.Errorf("resumed trace[%d].pass = %d, want %d (global numbering must be continuous)", i, resumed.Trace[i].Pass, i+1)
		}
	}
	if !resumed.GatePassed {
		t.Errorf("expected the resumed run to converge to min_score=95; final_score=%d reasons=%v", resumed.FinalScore, resumed.GateReasons)
	}
	if !resumed.NextState.Resumable && resumed.NextState.Completion != loopCompletionConverged {
		t.Errorf("a converged resume should report completion=converged, got %q", resumed.NextState.Completion)
	}
	if resumed.Path == "" {
		t.Error("resumed run must still write a PPTX")
	}
	if _, err := os.Stat(resumed.Path); err != nil {
		t.Errorf("resumed PPTX not found at %s: %v", resumed.Path, err)
	}
}

// TestAutoRepair_ResumeAfterRenderFailureCheckpoint covers resuming after a
// render/visual-QA failure. Renders complete on dev machines, so the failed
// state is reconstructed exactly as the engine would persist it: a checkpoint
// with completion=render_incomplete and one recorded pass. Resuming must
// continue from pass 2 (not repeat pass 1) and produce a fresh, valid result.
func TestAutoRepair_ResumeAfterRenderFailureCheckpoint(t *testing.T) {
	mc := repairMC(t)

	input := renderEvidenceInput(t) // minimal, cleanly-rendering deck
	cp := &loopCheckpoint{
		Tool:     "auto_repair",
		Input:    input,
		Trace:    []autoRepairTraceEntry{{Pass: 1, Score: 0, FindingsCount: 1, RepairsApplied: []string{}}},
		NextPass: 2,
		Gate: autoRepairGate{
			MinScore:                50,
			MaxP0Findings:           0,
			MaxP1Findings:           2,
			RequireTakeawayOnCharts: false,
		},
		MaxPasses:      1,
		OutputFilename: "auto_repair_resume_render_fail.pptx",
		Provenance:     contentProvenanceAuthorSupplied,
		Completion:     loopCompletionRenderIncomplete,
	}
	token := mc.loopSessions.Save(cp)
	if token == "" {
		t.Fatal("failed to seed the render-failure checkpoint")
	}

	resumed := runAutoRepairForTest(t, mc, map[string]any{
		"resume_token": token,
		"max_passes":   float64(2),
	})

	if resumed.NextState == nil {
		t.Fatal("resumed run must carry next_state")
	}
	// Continues at pass 2: the synthetic pass 1 stays at the head of the trace.
	if len(resumed.Trace) < 2 {
		t.Fatalf("expected the resume to add at least one pass beyond the seeded one, trace=%+v", resumed.Trace)
	}
	if resumed.Trace[0].Pass != 1 {
		t.Errorf("seeded pass 1 must be preserved at trace[0], got pass=%d", resumed.Trace[0].Pass)
	}
	if resumed.Trace[1].Pass != 2 {
		t.Errorf("resume must continue at pass 2, got pass=%d", resumed.Trace[1].Pass)
	}
	if resumed.Path == "" {
		t.Error("resumed run must write a PPTX")
	}
	if _, err := os.Stat(resumed.Path); err != nil {
		t.Errorf("resumed PPTX not found at %s: %v", resumed.Path, err)
	}
	// The deck renders cleanly here, so the resume should now reach complete
	// evidence and a converged state — proving the render-failure was recoverable.
	if !resumed.GatePassed {
		t.Errorf("expected the clean deck to pass the inherited gate on resume; reasons=%v", resumed.GateReasons)
	}
}

// TestAutoRepair_ResumeTokenNotFound asserts an unknown/expired token is a clean
// structured error, not a silent fresh run.
func TestAutoRepair_ResumeTokenNotFound(t *testing.T) {
	mc := repairMC(t)
	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"resume_token": "rs_unknown",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected an error result for an unknown resume_token, got success: %s", textContent(result))
	}
}

// TestAutoRepair_ResumeTokenMismatch asserts a make_deck token cannot be resumed
// through auto_repair (their provenance and required fields differ).
func TestAutoRepair_ResumeTokenMismatch(t *testing.T) {
	mc := repairMC(t)
	token := mc.loopSessions.Save(&loopCheckpoint{Tool: "make_deck", Input: renderEvidenceInput(t), NextPass: 2})

	result, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
		"resume_token": token,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected an error resuming a make_deck token via auto_repair, got success: %s", textContent(result))
	}
}

// TestMakeDeck_ResumePreservesPlan asserts make_deck surfaces a resumable
// next_state and that resuming reproduces the plan summary (carried in the
// checkpoint) without re-planning.
func TestMakeDeck_ResumePreservesPlan(t *testing.T) {
	mc := repairMC(t)

	lenientGate := map[string]any{
		"min_score":                  float64(0),
		"max_p0_findings":            float64(99),
		"max_p1_findings":            float64(99),
		"require_takeaway_on_charts": false,
	}
	first := runMakeDeckForTest(t, mc, map[string]any{
		"outline":           "Quarterly business review for the leadership team",
		"style_hints":       map[string]any{"slide_budget": float64(3)},
		"gate":              lenientGate,
		"max_repair_passes": float64(1),
		"output_filename":   "make_deck_resume_first.pptx",
	})

	if first.NextState == nil {
		t.Fatal("make_deck must carry next_state")
	}
	if first.Plan == nil || len(first.Plan.Slides) == 0 {
		t.Fatal("make_deck must return a plan")
	}
	token := first.NextState.ResumeToken
	if token == "" {
		t.Fatal("expected a resume_token from make_deck")
	}

	resumed := runMakeDeckForTest(t, mc, map[string]any{
		"resume_token":    token,
		"output_filename": "make_deck_resume_second.pptx",
	})

	if resumed.Plan == nil {
		t.Fatal("resumed make_deck must still return a plan (carried in the checkpoint)")
	}
	if resumed.Plan.Template != first.Plan.Template {
		t.Errorf("resumed plan template %q != original %q", resumed.Plan.Template, first.Plan.Template)
	}
	if len(resumed.Plan.Slides) != len(first.Plan.Slides) {
		t.Errorf("resumed plan has %d slides, want %d", len(resumed.Plan.Slides), len(first.Plan.Slides))
	} else {
		for i := range first.Plan.Slides {
			if resumed.Plan.Slides[i].RecommendedPattern != first.Plan.Slides[i].RecommendedPattern {
				t.Errorf("resumed plan slide %d pattern %q != original %q", i,
					resumed.Plan.Slides[i].RecommendedPattern, first.Plan.Slides[i].RecommendedPattern)
			}
		}
	}
	if resumed.ContentStatus != "exemplar_skeleton" || resumed.Publishable {
		t.Errorf("resumed make_deck must stay an exemplar skeleton (publishable=false), got content_status=%q publishable=%v",
			resumed.ContentStatus, resumed.Publishable)
	}
}

// TestAutoRepair_ResumeSameTokenIsDeterministic asserts that resuming the same
// token twice starts from the same saved deck both times — the loop must not
// mutate the stored checkpoint in place. Without the deep copy, the second
// resume would begin from the first resume's already-repaired deck and observe a
// different finding count on its first new pass.
func TestAutoRepair_ResumeSameTokenIsDeterministic(t *testing.T) {
	mc := repairMC(t)

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(autoRepairDeck(3)), &input); err != nil {
		t.Fatalf("unmarshal seed deck: %v", err)
	}
	applyDefaults(&input)

	cp := &loopCheckpoint{
		Tool:     "auto_repair",
		Input:    &input,
		Trace:    []autoRepairTraceEntry{{Pass: 1, Score: 85, FindingsCount: 3, RepairsApplied: []string{}}},
		NextPass: 2,
		Gate: autoRepairGate{
			MinScore:                95,
			MaxP0Findings:           0,
			MaxP1Findings:           2,
			RequireTakeawayOnCharts: false,
		},
		MaxPasses:      1,
		OutputFilename: "auto_repair_resume_determinism.pptx",
		Provenance:     contentProvenanceAuthorSupplied,
		Completion:     loopCompletionExhausted,
	}
	token := mc.loopSessions.Save(cp)

	first := runAutoRepairForTest(t, mc, map[string]any{"resume_token": token, "max_passes": float64(3)})
	second := runAutoRepairForTest(t, mc, map[string]any{"resume_token": token, "max_passes": float64(3)})

	if len(first.Trace) < 2 || len(second.Trace) < 2 {
		t.Fatalf("expected both resumes to add passes; first=%d second=%d", len(first.Trace), len(second.Trace))
	}
	// The first NEW pass (index 1, global pass 2) must see the same finding count
	// both times — proving the second resume started from the original saved deck,
	// not the first resume's repaired one.
	if first.Trace[1].FindingsCount != second.Trace[1].FindingsCount {
		t.Errorf("resume not deterministic: pass-2 findings_count differ (first=%d second=%d) — the stored checkpoint deck was mutated in place",
			first.Trace[1].FindingsCount, second.Trace[1].FindingsCount)
	}
	if first.Passes != second.Passes {
		t.Errorf("resume not deterministic: passes differ (first=%d second=%d)", first.Passes, second.Passes)
	}
}

// --- helpers ---

func runAutoRepairForTest(t *testing.T, mc *mcpConfig, args map[string]any) autoRepairOutput {
	t.Helper()
	result, err := mc.handleAutoRepair(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handleAutoRepair error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleAutoRepair tool error: %s", textContent(result))
	}
	var output autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal auto_repair response: %v", err)
	}
	return output
}

func runMakeDeckForTest(t *testing.T, mc *mcpConfig, args map[string]any) makeDeckOutput {
	t.Helper()
	result, err := mc.handleMakeDeck(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handleMakeDeck error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleMakeDeck tool error: %s", textContent(result))
	}
	var output makeDeckOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("unmarshal make_deck response: %v", err)
	}
	return output
}
