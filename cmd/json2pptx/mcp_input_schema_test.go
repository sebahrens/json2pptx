package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestGetInputSchema(t *testing.T) {
	t.Run("returns valid schema", func(t *testing.T) {
		result, err := handleGetInputSchema(context.Background(), makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp inputSchemaResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.Digest == "" {
			t.Error("expected non-empty digest")
		}
		if resp.NotModified {
			t.Error("expected not_modified to be false on first call")
		}
		if resp.Schema == nil {
			t.Fatal("expected non-nil schema")
		}

		// Check root schema structure.
		if resp.Schema["$ref"] != "#/$defs/PresentationInput" {
			t.Errorf("expected $ref to PresentationInput, got %v", resp.Schema["$ref"])
		}
		defs, ok := resp.Schema["$defs"].(map[string]any)
		if !ok {
			t.Fatal("expected $defs to be a map")
		}

		// Verify key types are present.
		expectedTypes := []string{
			"PresentationInput", "SlideInput", "ContentInput",
			"ShapeGridInput", "GridCellInput", "TableInput",
			"PatternInput", "ComposeInput", "ChartSpec", "DiagramSpec",
		}
		for _, name := range expectedTypes {
			if _, ok := defs[name]; !ok {
				t.Errorf("missing $defs/%s", name)
			}
		}
	})

	t.Run("field_scope annotations present", func(t *testing.T) {
		result, _ := handleGetInputSchema(context.Background(), makeRequest(map[string]any{}))
		text := result.Content[0].(mcp.TextContent).Text
		var resp inputSchemaResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		defs := resp.Schema["$defs"].(map[string]any)

		// PresentationInput.design_mode should have field_scope = "deck"
		pi := defs["PresentationInput"].(map[string]any)
		props := pi["properties"].(map[string]any)
		dm := props["design_mode"].(map[string]any)
		if dm["x-field-scope"] != "deck" {
			t.Errorf("expected design_mode field_scope=deck, got %v", dm["x-field-scope"])
		}

		// SlideInput.contrast_check should have field_scope = "slide"
		si := defs["SlideInput"].(map[string]any)
		siProps := si["properties"].(map[string]any)
		cc := siProps["contrast_check"].(map[string]any)
		if cc["x-field-scope"] != "slide" {
			t.Errorf("expected contrast_check field_scope=slide, got %v", cc["x-field-scope"])
		}
	})

	t.Run("enum values present", func(t *testing.T) {
		result, _ := handleGetInputSchema(context.Background(), makeRequest(map[string]any{}))
		text := result.Content[0].(mcp.TextContent).Text
		var resp inputSchemaResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		defs := resp.Schema["$defs"].(map[string]any)
		pi := defs["PresentationInput"].(map[string]any)
		props := pi["properties"].(map[string]any)

		// design_mode should have enum
		dm := props["design_mode"].(map[string]any)
		enumVals, ok := dm["enum"].([]any)
		if !ok || len(enumVals) == 0 {
			t.Error("expected design_mode to have enum values")
		}

		// accent_strategy should have enum
		as := props["accent_strategy"].(map[string]any)
		asEnum, ok := as["enum"].([]any)
		if !ok || len(asEnum) == 0 {
			t.Error("expected accent_strategy to have enum values")
		}
	})

	t.Run("digest caching works", func(t *testing.T) {
		// Get the schema to learn the digest.
		result1, _ := handleGetInputSchema(context.Background(), makeRequest(map[string]any{}))
		text1 := result1.Content[0].(mcp.TextContent).Text
		var resp1 inputSchemaResponse
		_ = json.Unmarshal([]byte(text1), &resp1)

		// Request again with the same digest.
		result2, _ := handleGetInputSchema(context.Background(), makeRequest(map[string]any{
			"digest": resp1.Digest,
		}))
		text2 := result2.Content[0].(mcp.TextContent).Text
		var resp2 inputSchemaResponse
		_ = json.Unmarshal([]byte(text2), &resp2)

		if !resp2.NotModified {
			t.Error("expected not_modified=true when digest matches")
		}
		if resp2.Schema != nil {
			t.Error("expected nil schema when not_modified")
		}
	})

	t.Run("digest is stable", func(t *testing.T) {
		schema1 := buildInputSchema()
		schema2 := buildInputSchema()
		d1 := computeInputSchemaDigest(schema1)
		d2 := computeInputSchemaDigest(schema2)
		if d1 != d2 {
			t.Errorf("digest is non-deterministic: %q != %q", d1, d2)
		}
	})

	t.Run("$refs resolve to $defs entries", func(t *testing.T) {
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)

		// Walk all properties and check that $ref targets exist in $defs.
		for typeName, def := range defs {
			defMap := def.(map[string]any)
			props, ok := defMap["properties"].(map[string]any)
			if !ok {
				continue
			}
			for fieldName, prop := range props {
				propMap := prop.(map[string]any)
				checkRef(t, typeName+"."+fieldName, propMap, defs)
			}
		}
	})
}

// checkRef recursively validates that any $ref in a property schema points to an existing $defs entry.
func checkRef(t *testing.T, path string, prop map[string]any, defs map[string]any) {
	t.Helper()
	if ref, ok := prop["$ref"].(string); ok {
		// Extract def name from "#/$defs/FooBar"
		const prefix = "#/$defs/"
		if len(ref) > len(prefix) {
			defName := ref[len(prefix):]
			if _, ok := defs[defName]; !ok {
				t.Errorf("%s: $ref %q not found in $defs", path, ref)
			}
		}
	}
	// Check inside array items.
	if items, ok := prop["items"].(map[string]any); ok {
		checkRef(t, path+"[items]", items, defs)
	}
}
