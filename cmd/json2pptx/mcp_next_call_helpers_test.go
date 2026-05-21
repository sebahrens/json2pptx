package main

import (
	"context"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// requireNextToolCall asserts that the diagnostic carries a next_tool_call
// suggesting the given tool. The test fails with a helpful message when the
// field is missing or wrong, since the contract is "every handler error chains
// forward to a recovery tool".
func requireNextToolCall(t *testing.T, d diagnostics.Diagnostic, wantTool string) {
	t.Helper()
	if d.NextToolCall == nil {
		t.Fatalf("expected diagnostic %q to carry next_tool_call, got nil", d.Code)
	}
	if d.NextToolCall.Tool != wantTool {
		t.Fatalf("expected next_tool_call.tool=%q, got %q (code=%q)", wantTool, d.NextToolCall.Tool, d.Code)
	}
}

// TestHandlerErrors_CarryNextToolCall asserts that the error responses from
// plan_deck, recommend_pattern, recommend_visual, validate_input,
// preview_presentation_plan, and score_deck all set next_tool_call so agents
// can chain to the recovery tool without inferring it from prose.
func TestHandlerErrors_CarryNextToolCall(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()

	t.Run("plan_deck_missing_brief", func(t *testing.T) {
		result, err := mc.handlePlanDeck(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "plan_deck")
	})

	t.Run("plan_deck_unknown_must_include", func(t *testing.T) {
		result, err := mc.handlePlanDeck(ctx, makeRequest(map[string]any{
			"brief":        "test deck",
			"must_include": []any{"no-such-pattern"},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "INVALID_PARAMETER")
		requireNextToolCall(t, d, "list_patterns")
	})

	t.Run("recommend_pattern_missing_intent", func(t *testing.T) {
		result, err := mc.handleRecommendPattern(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "recommend_pattern")
	})

	t.Run("recommend_visual_missing_intent", func(t *testing.T) {
		result, err := mc.handleRecommendVisual(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "recommend_visual")
	})

	t.Run("validate_missing_presentation", func(t *testing.T) {
		result, err := mc.handleValidate(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("validate_invalid_json", func(t *testing.T) {
		result, err := mc.handleValidate(ctx, makeRequest(map[string]any{
			"presentation": "not-an-object",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("validate_template_required", func(t *testing.T) {
		result, err := mc.handleValidate(ctx, makeRequest(map[string]any{
			"presentation": mustParseJSON(`{"slides":[{"layout_id":"title"}]}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "REQUIRED")
		requireNextToolCall(t, d, "list_templates")
	})

	t.Run("preview_missing_presentation", func(t *testing.T) {
		result, err := mc.handlePreviewPlan(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("preview_invalid_json", func(t *testing.T) {
		result, err := mc.handlePreviewPlan(ctx, makeRequest(map[string]any{
			"presentation": "not-an-object",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("preview_template_required", func(t *testing.T) {
		result, err := mc.handlePreviewPlan(ctx, makeRequest(map[string]any{
			"presentation": mustParseJSON(`{"slides":[{"layout_id":"title"}]}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "REQUIRED")
		requireNextToolCall(t, d, "list_templates")
	})

	t.Run("score_deck_missing_presentation", func(t *testing.T) {
		result, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("score_deck_invalid_json", func(t *testing.T) {
		result, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{
			"presentation": "not-an-object",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "INVALID_JSON")
		requireNextToolCall(t, d, "get_input_schema")
	})

	t.Run("score_deck_missing_template", func(t *testing.T) {
		result, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{
			"presentation": mustParseJSON(`{"slides":[{"layout_id":"title"}]}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "MISSING_PARAMETER")
		requireNextToolCall(t, d, "list_templates")
	})

	t.Run("score_deck_template_not_found", func(t *testing.T) {
		result, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{
			"presentation": mustParseJSON(`{"template":"no-such","slides":[{"layout_id":"title"}]}`),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := parseMCPError(t, result)
		d := requireDiagCode(t, env.Diagnostics, "TEMPLATE_NOT_FOUND")
		requireNextToolCall(t, d, "list_templates")
	})
}
