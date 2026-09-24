package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func toolParameterDescription(t *testing.T, tool mcp.Tool, param string) string {
	t.Helper()
	property, ok := tool.InputSchema.Properties[param]
	if !ok {
		t.Fatalf("%s has no %q parameter", tool.Name, param)
	}
	raw, err := json.Marshal(property)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	return schema.Description
}

func TestMCPDescriptionsMatchRuntimeContracts(t *testing.T) {
	for _, tool := range []mcp.Tool{mcpListTemplatesTool(), mcpListPatternsTool(), mcpListIconsTool()} {
		desc := toolParameterDescription(t, tool, "fields")
		if !strings.Contains(desc, listFieldsOmitted) || strings.Contains(desc, "deprecation hint") {
			t.Errorf("%s.fields misstates the compact default: %q", tool.Name, desc)
		}
	}
	if desc := getStartedToolDescription(); !strings.Contains(desc, `"revise": modifying an existing PPTX — fast_path render_deck_spec`) || strings.Contains(desc, `"revise": modifying an existing PPTX — fast_path repair_slide`) {
		t.Errorf("get_started description misstates revise fast path: %q", desc)
	}
	if desc := mcpGetCapabilitiesTool().Description; !strings.Contains(desc, "runtime.schema_fingerprint") || strings.Contains(desc, "schema_fingerprint and changelog_url are in every projection") {
		t.Errorf("get_capabilities description misstates fingerprint projection: %q", desc)
	}
	for _, tool := range []mcp.Tool{mcpRenderSlideImageTool(), mcpRenderDeckThumbnailsTool()} {
		desc := toolParameterDescription(t, tool, "pptx_path")
		if !strings.Contains(desc, "generate_presentation returns output_path") || !strings.Contains(desc, "render_deck_spec returns pptx_path") {
			t.Errorf("%s.pptx_path omits an artifact field name: %q", tool.Name, desc)
		}
	}
	if desc := mcpPlanDeckTool().Description; !strings.Contains(desc, "slides[i].skeleton") || !strings.Contains(desc, "NOT SlideInput objects") || strings.Contains(desc, "directly consumable as the slides array") {
		t.Errorf("plan_deck description misstates the consumable slide field: %q", desc)
	}
	if schema := string(outputSchemaPlanDeck); !strings.Contains(schema, "Copy this field (not the enclosing plan record)") || strings.Contains(schema, "Validates as-is with validate_input") {
		t.Error("plan_deck output schema still promises the advisory record or unfilled skeleton is directly valid")
	}
}
