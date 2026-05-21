package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
)

// fingerprintFromResult extracts the response_fingerprint field from an MCP
// tool result's StructuredContent. It fails the test if the field is missing,
// empty, or not a 64-character hex string.
func fingerprintFromResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatalf("nil tool result")
	}
	if result.IsError {
		t.Fatalf("expected success result, got IsError=true content=%v", result.Content)
	}
	// Marshal StructuredContent through JSON so we get the same view the agent
	// receives over the wire.
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal StructuredContent: %v: %s", err, string(raw))
	}
	fp, ok := envelope["response_fingerprint"].(string)
	if !ok || fp == "" {
		t.Fatalf("expected response_fingerprint in response, got: %s", string(raw))
	}
	if len(fp) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got %d: %q", len(fp), fp)
	}
	return fp
}

// TestResponseFingerprint_PresentInAllFourTools verifies that the four target
// MCP handlers (validate_input, preview_presentation_plan, plan_deck,
// recommend_visual) embed a non-empty response_fingerprint in their success
// responses.
func TestResponseFingerprint_PresentInAllFourTools(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()

	deck := mustParseJSON(`{
		"template": "midnight-blue",
		"slides": [
			{"layout_id": "title", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hello"}]}
		]
	}`)

	t.Run("validate_input", func(t *testing.T) {
		result, err := mc.handleValidate(ctx, makeRequest(map[string]any{
			"presentation": deck,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fingerprintFromResult(t, result)
	})

	t.Run("preview_presentation_plan", func(t *testing.T) {
		result, err := mc.handlePreviewPlan(ctx, makeRequest(map[string]any{
			"presentation": deck,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fingerprintFromResult(t, result)
	})

	t.Run("plan_deck", func(t *testing.T) {
		result, err := mc.handlePlanDeck(ctx, makeRequest(map[string]any{
			"brief":        "quarterly review of north america sales",
			"slide_budget": 6.0,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fingerprintFromResult(t, result)
	})

	t.Run("recommend_visual", func(t *testing.T) {
		result, err := mc.handleRecommendVisual(ctx, makeRequest(map[string]any{
			"intent": "show three KPI cards for revenue, customers, NPS",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fingerprintFromResult(t, result)
	})
}

// TestResponseFingerprint_AdvertisedInOutputSchemas asserts that every tool
// that emits response_fingerprint also lists it as a "string" property in its
// MCP output schema. Schema-driven clients must be able to discover the field
// from the schema alone; drift between emission and schema breaks them
// silently. This test must fail the moment a tool starts emitting (or stops
// advertising) the field.
func TestResponseFingerprint_AdvertisedInOutputSchemas(t *testing.T) {
	schemas := map[string]json.RawMessage{
		"validate_input":            outputSchemaValidate,
		"preview_presentation_plan": outputSchemaPreviewPlan,
		"plan_deck":                 outputSchemaPlanDeck,
		"recommend_visual":          outputSchemaRecommendVisual,
	}

	for name, schema := range schemas {
		var parsed map[string]any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("%s: schema not valid JSON: %v", name, err)
		}
		props, ok := parsed["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: schema missing top-level properties object", name)
		}
		fpProp, ok := props["response_fingerprint"].(map[string]any)
		if !ok {
			t.Errorf("%s: output schema must declare response_fingerprint as a property (drift between emission and schema)", name)
			continue
		}
		if got, _ := fpProp["type"].(string); got != "string" {
			t.Errorf("%s: response_fingerprint must be type=string, got %q", name, got)
		}
	}
}

// assertCanonicalFindingsOrder fails the test if findings is not sorted by
// (action_rank desc, slide_index asc, code asc) — the canonical invariant
// every fit_report / findings array must satisfy before crossing a
// serialization boundary. See patterns.SortCanonical and
// docs/FIT_FINDINGS.md "Sort invariant".
func assertCanonicalFindingsOrder(t *testing.T, label string, findings []patterns.FitFinding) {
	t.Helper()
	for i := 1; i < len(findings); i++ {
		ri := patterns.ActionRank(findings[i].Action)
		rPrev := patterns.ActionRank(findings[i-1].Action)
		if ri > rPrev {
			t.Errorf("%s: findings[%d] action %q (rank %d) ranks higher than findings[%d] action %q (rank %d) — severity must be non-increasing",
				label, i, findings[i].Action, ri, i-1, findings[i-1].Action, rPrev)
			continue
		}
		if ri != rPrev {
			continue
		}
		si := slidepath.SlideIndex(findings[i].Path)
		sPrev := slidepath.SlideIndex(findings[i-1].Path)
		if si < sPrev {
			t.Errorf("%s: findings[%d] slide %d precedes findings[%d] slide %d at equal severity — slide must be non-decreasing",
				label, i-1, sPrev, i, si)
			continue
		}
		if si != sPrev {
			continue
		}
		if findings[i].Code < findings[i-1].Code {
			t.Errorf("%s: findings[%d] code %q precedes findings[%d] code %q at equal (severity, slide) — code must be non-decreasing",
				label, i-1, findings[i-1].Code, i, findings[i].Code)
		}
	}
}

// TestFindingsCanonicalOrder_AcrossTools asserts the canonical findings sort
// invariant — (action_rank desc, slide_index asc, code asc) — at every
// serialization boundary that emits patterns.FitFinding. Agents that grab
// findings[0] must reliably get the most important fix.
//
// Strategy: build synthetic findings deliberately scrambled across severity /
// slide / code; route them through the same BudgetFitFindings gate that
// validate_input, preview_presentation_plan, generate_presentation, repair_slide,
// and score_deck all funnel through (see mc.handleValidate, handlePreviewPlan,
// collectFitFindings + BudgetFitFindings call sites). One invariant check at
// the gate covers every downstream caller.
func TestFindingsCanonicalOrder_AcrossTools(t *testing.T) {
	// Scrambled input: mix actions and slide indices so the budget pass would
	// pick them up in random order without a canonical sort.
	scrambled := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: "/slides/3/content/body", Code: "zzz_low_severity"}, Action: "info"},
		{ValidationError: patterns.ValidationError{Path: "/slides/1/content/body", Code: "alpha_refuse",
			Fix: &patterns.FixSuggestion{Kind: "reduce_text"}}, Action: "refuse"},
		{ValidationError: patterns.ValidationError{Path: "/slides/0/shape_grid", Code: "beta_shrink",
			Fix: &patterns.FixSuggestion{Kind: "split_at_row"}}, Action: "shrink_or_split"},
		{ValidationError: patterns.ValidationError{Path: "/slides/2/content/body", Code: "alpha_review"}, Action: "review"},
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/body", Code: "zeta_refuse",
			Fix: &patterns.FixSuggestion{Kind: "shorten_title"}}, Action: "refuse"},
		{ValidationError: patterns.ValidationError{Path: "/slides/2/content/body", Code: "beta_review"}, Action: "review"},
	}

	// Pre-sort with the canonical helper directly (collectFitFindings path).
	pre := make([]patterns.FitFinding, len(scrambled))
	copy(pre, scrambled)
	patterns.SortCanonical(pre, slidepath.SlideIndex)
	assertCanonicalFindingsOrder(t, "patterns.SortCanonical", pre)

	// Spot-check the exact order: refuse on slide 0 before refuse on slide 1,
	// shrink_or_split on slide 0, then review (slide 2 by code asc), then info.
	wantOrder := []string{
		"zeta_refuse",   // refuse, slide 0
		"alpha_refuse",  // refuse, slide 1
		"beta_shrink",   // shrink_or_split, slide 0
		"alpha_review",  // review, slide 2, code "alpha_review" < "beta_review"
		"beta_review",   // review, slide 2
		"zzz_low_severity", // info, slide 3
	}
	for i, want := range wantOrder {
		if pre[i].Code != want {
			t.Errorf("patterns.SortCanonical: findings[%d].Code = %q, want %q", i, pre[i].Code, want)
		}
	}

	// Now run the same scrambled input through BudgetFitFindings (the final
	// gate before findings hit validate_input / preview_presentation_plan /
	// generate_presentation / repair_slide / score_deck responses). The budget
	// pass groups by slide internally; the post-budget canonical sort must
	// still leave the result in invariant order.
	budgeted := BudgetFitFindings(scrambled, DefaultFindingBudget, false /* verbose */)
	assertCanonicalFindingsOrder(t, "BudgetFitFindings (non-verbose)", budgeted)

	// Verbose mode bypasses the budget pass entirely — it should not reorder
	// findings, but collectFitFindings always pre-sorts canonically, so we
	// pre-sort our synthetic input first to mirror real usage and then verify
	// the invariant still holds end-to-end.
	verboseInput := make([]patterns.FitFinding, len(scrambled))
	copy(verboseInput, scrambled)
	patterns.SortCanonical(verboseInput, slidepath.SlideIndex)
	verbose := BudgetFitFindings(verboseInput, DefaultFindingBudget, true /* verbose */)
	assertCanonicalFindingsOrder(t, "BudgetFitFindings (verbose)", verbose)
}

// TestFindingsCanonicalOrder_OverflowBudgetAcrossSlides asserts the invariant
// holds even when the per-slide budget triggers (truncation summaries appended
// per slide). Without the post-budget canonical sort the per-slide
// truncation_summary entries land interleaved by insertion order of slide
// indices, which would scramble the (severity, slide, code) ordering for
// agents that grab findings[0].
func TestFindingsCanonicalOrder_OverflowBudgetAcrossSlides(t *testing.T) {
	// Slide 5 gets 8 info findings (exceeds budget 5 → triggers summary).
	// Slide 1 gets one refuse and one info, both under budget.
	// Slide 3 gets 7 review findings (exceeds budget → summary).
	var findings []patterns.FitFinding
	for i := 0; i < 8; i++ {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: "/slides/5/content/body",
				Code: fmt.Sprintf("slide5_info_%02d", i),
			},
			Action: "info",
		})
	}
	findings = append(findings,
		patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: "/slides/1/content/body",
				Code: "slide1_refuse",
				Fix:  &patterns.FixSuggestion{Kind: "reduce_text"},
			},
			Action: "refuse",
		},
		patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: "/slides/1/content/body",
				Code: "slide1_info",
			},
			Action: "info",
		},
	)
	for i := 0; i < 7; i++ {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: "/slides/3/shape_grid",
				Code: fmt.Sprintf("slide3_review_%02d", i),
				Fix:  &patterns.FixSuggestion{Kind: "reshape_grid"},
			},
			Action: "review",
		})
	}

	budgeted := BudgetFitFindings(findings, DefaultFindingBudget, false)
	assertCanonicalFindingsOrder(t, "BudgetFitFindings multi-slide overflow", budgeted)

	// findings[0] must be the most actionable: refuse on slide 1.
	if budgeted[0].Action != "refuse" || budgeted[0].Code != "slide1_refuse" {
		t.Errorf("findings[0] = {action:%q code:%q}, want {refuse slide1_refuse} — agents rely on findings[0] being the most important fix",
			budgeted[0].Action, budgeted[0].Code)
	}
}

// TestResponseFingerprint_DeterministicAcrossCalls verifies that calling a
// fingerprinted handler twice with identical inputs produces identical
// fingerprints, which is the cache-key contract.
func TestResponseFingerprint_DeterministicAcrossCalls(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()

	args := map[string]any{
		"brief":        "annual planning offsite",
		"slide_budget": 8.0,
	}

	r1, err := mc.handlePlanDeck(ctx, makeRequest(args))
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	r2, err := mc.handlePlanDeck(ctx, makeRequest(args))
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}

	fp1 := fingerprintFromResult(t, r1)
	fp2 := fingerprintFromResult(t, r2)
	if fp1 != fp2 {
		t.Fatalf("expected identical fingerprints for identical inputs, got %s vs %s", fp1, fp2)
	}
}
