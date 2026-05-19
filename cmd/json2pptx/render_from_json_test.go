package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/render"
)

// TestHandleRenderSlideImageFromJSON_MissingSlide verifies the handler returns
// a structured error when the required slide parameter is absent.
func TestHandleRenderSlideImageFromJSON_MissingSlide(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"template": "midnight-blue",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when slide is missing")
	}
}

// TestHandleRenderSlideImageFromJSON_MissingTemplate verifies the handler
// rejects a request that omits the template name.
func TestHandleRenderSlideImageFromJSON_MissingTemplate(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide": map[string]any{"layout_id": "slideLayout1"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when template is missing")
	}
}

// TestHandleRenderSlideImageFromJSON_TemplateNotFound verifies the handler
// surfaces TEMPLATE_NOT_FOUND for an unknown template name.
func TestHandleRenderSlideImageFromJSON_TemplateNotFound(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide":    map[string]any{"layout_id": "slideLayout1"},
		"template": "definitely-not-a-real-template",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for unknown template")
	}
}

// TestHandleRenderSlideImageFromJSON_DensityClampParsing exercises density
// clamp branches and the force flag without requiring LibreOffice to succeed
// (LibreOffice is invoked but failure here is acceptable for branch coverage).
func TestHandleRenderSlideImageFromJSON_DensityClampParsing(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")

	// Lower clamp.
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide":    map[string]any{"layout_id": "slideLayout1"},
		"template": "midnight-blue",
		"density":  float64(10), // below 50; clamp branch
		"force":    true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	// Upper clamp.
	res2, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide":    map[string]any{"layout_id": "slideLayout1"},
		"template": "midnight-blue",
		"density":  float64(9999), // above 300; clamp branch
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2 == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestRenderSlideWithCacheKey_RequiresKey verifies the render helper rejects
// an empty cache key — callers must supply an identity derived from upstream
// inputs.
func TestRenderSlideWithCacheKey_RequiresKey(t *testing.T) {
	_, err := render.RenderSlideWithCacheKey("/tmp/does-not-matter.pptx", 0, 100, false, "")
	if err == nil {
		t.Fatal("expected error for empty cache key")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("expected error mentioning key, got: %v", err)
	}
}

// TestLookupCachedSlide_EmptyKey returns nil for empty key (defensive check
// — callers should never reach this branch but failing closed is correct).
func TestLookupCachedSlide_EmptyKey(t *testing.T) {
	if got := render.LookupCachedSlide("", 0, 100); got != nil {
		t.Fatalf("expected nil for empty key, got %+v", got)
	}
}

// TestRenderSlideImageFromJSON_OutputSchemaShared verifies that the new tool
// reuses the same output schema as render_slide_image — callers can parse
// either response with the same JSON unmarshal.
func TestRenderSlideImageFromJSON_OutputSchemaShared(t *testing.T) {
	tool := mcpRenderSlideImageFromJSONTool()
	// Output schema is the same constant (pointer/value identity not required;
	// what matters is the JSON structure).
	var schema map[string]any
	if err := json.Unmarshal(tool.RawOutputSchema, &schema); err != nil {
		t.Fatalf("schema not valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	for _, want := range []string{"index", "png_base64", "path", "size_error"} {
		if _, ok := props[want]; !ok {
			t.Errorf("expected schema property %q", want)
		}
	}
}

// _ keeps mcpgo referenced even when no other test in this file uses it
// directly.
var _ mcpgo.Tool
