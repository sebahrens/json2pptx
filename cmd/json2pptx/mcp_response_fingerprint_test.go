package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

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
		result, err := handlePlanDeck(ctx, makeRequest(map[string]any{
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

// TestResponseFingerprint_DeterministicAcrossCalls verifies that calling a
// fingerprinted handler twice with identical inputs produces identical
// fingerprints, which is the cache-key contract.
func TestResponseFingerprint_DeterministicAcrossCalls(t *testing.T) {
	ctx := context.Background()

	args := map[string]any{
		"brief":        "annual planning offsite",
		"slide_budget": 8.0,
	}

	r1, err := handlePlanDeck(ctx, makeRequest(args))
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	r2, err := handlePlanDeck(ctx, makeRequest(args))
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}

	fp1 := fingerprintFromResult(t, r1)
	fp2 := fingerprintFromResult(t, r2)
	if fp1 != fp2 {
		t.Fatalf("expected identical fingerprints for identical inputs, got %s vs %s", fp1, fp2)
	}
}
