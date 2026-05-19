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

	t.Run("split_slide variants are exposed in slides.items", func(t *testing.T) {
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)

		// SplitSlideInput and SplitConfig must be registered as $defs.
		if _, ok := defs["SplitSlideInput"]; !ok {
			t.Fatal("missing $defs/SplitSlideInput")
		}
		if _, ok := defs["SplitConfig"]; !ok {
			t.Fatal("missing $defs/SplitConfig")
		}

		// PresentationInput.slides.items must be a oneOf of SplitSlideInput
		// and SlideInput so agents can author either variant from schema alone.
		pi := defs["PresentationInput"].(map[string]any)
		props := pi["properties"].(map[string]any)
		slides := props["slides"].(map[string]any)
		if slides["type"] != "array" {
			t.Errorf("expected slides.type=array, got %v", slides["type"])
		}
		items, ok := slides["items"].(map[string]any)
		if !ok {
			t.Fatal("expected slides.items to be an object")
		}
		oneOf, ok := items["oneOf"].([]any)
		if !ok {
			t.Fatalf("expected slides.items.oneOf to be a list, got %T", items["oneOf"])
		}
		if len(oneOf) != 2 {
			t.Fatalf("expected exactly 2 oneOf branches (split_slide, regular_slide), got %d", len(oneOf))
		}
		refs := map[string]bool{}
		for _, branch := range oneOf {
			bm := branch.(map[string]any)
			if r, ok := bm["$ref"].(string); ok {
				refs[r] = true
			}
		}
		if !refs["#/$defs/SplitSlideInput"] {
			t.Error("expected slides.items.oneOf to include $ref to SplitSlideInput")
		}
		if !refs["#/$defs/SlideInput"] {
			t.Error("expected slides.items.oneOf to include $ref to SlideInput")
		}
	})

	t.Run("SplitSlideInput pins type discriminator", func(t *testing.T) {
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)
		ss, ok := defs["SplitSlideInput"].(map[string]any)
		if !ok {
			t.Fatal("missing $defs/SplitSlideInput")
		}
		ssProps := ss["properties"].(map[string]any)
		typeProp, ok := ssProps["type"].(map[string]any)
		if !ok {
			t.Fatal("SplitSlideInput.properties.type missing")
		}
		if got := enumStrings(typeProp["enum"]); len(got) != 1 || got[0] != "split_slide" {
			t.Errorf("expected SplitSlideInput.type enum=[\"split_slide\"], got %v", typeProp["enum"])
		}

		// SplitConfig.by must be pinned to "table.rows".
		sc := defs["SplitConfig"].(map[string]any)
		scProps := sc["properties"].(map[string]any)
		byProp := scProps["by"].(map[string]any)
		if got := enumStrings(byProp["enum"]); len(got) != 1 || got[0] != "table.rows" {
			t.Errorf("expected SplitConfig.by enum=[\"table.rows\"], got %v", byProp["enum"])
		}
	})

	t.Run("SlideInput expresses slide-level alternatives", func(t *testing.T) {
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)
		si := defs["SlideInput"].(map[string]any)

		// anyOf must capture the "layout_id OR slide_type required" rule.
		anyOf, ok := si["anyOf"].([]any)
		if !ok {
			t.Fatal("expected SlideInput.anyOf to express layout_id/slide_type alternative")
		}
		seen := map[string]bool{}
		for _, branch := range anyOf {
			bm := branch.(map[string]any)
			req, ok := bm["required"].([]any)
			if !ok || len(req) != 1 {
				continue
			}
			if name, ok := req[0].(string); ok {
				seen[name] = true
			}
		}
		if !seen["layout_id"] || !seen["slide_type"] {
			t.Errorf("expected SlideInput.anyOf to require layout_id or slide_type, got %v", anyOf)
		}

		// allOf must express mutual exclusion among pattern/shape_grid/compose.
		allOf, ok := si["allOf"].([]any)
		if !ok {
			t.Fatal("expected SlideInput.allOf to express pattern/shape_grid/compose mutual exclusion")
		}
		excluded := map[string]bool{}
		for _, branch := range allOf {
			bm := branch.(map[string]any)
			not, ok := bm["not"].(map[string]any)
			if !ok {
				continue
			}
			req, ok := not["required"].([]any)
			if !ok {
				continue
			}
			if len(req) == 2 {
				if a, ok := req[0].(string); ok {
					if b, ok := req[1].(string); ok {
						pair := a + "|" + b
						if b < a {
							pair = b + "|" + a
						}
						excluded[pair] = true
					}
				}
			}
		}
		wantedPairs := []string{"pattern|shape_grid", "compose|pattern", "compose|shape_grid"}
		for _, p := range wantedPairs {
			if !excluded[p] {
				t.Errorf("expected allOf to exclude pair %q, missing", p)
			}
		}
	})

	t.Run("ContentInput encodes typed-value discriminator", func(t *testing.T) {
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)
		ci, ok := defs["ContentInput"].(map[string]any)
		if !ok {
			t.Fatal("missing $defs/ContentInput")
		}

		// type field still has the full enum so agents can list valid discriminators.
		ciProps := ci["properties"].(map[string]any)
		typeProp, ok := ciProps["type"].(map[string]any)
		if !ok {
			t.Fatal("ContentInput.properties.type missing")
		}
		typeEnum := enumStrings(typeProp["enum"])
		wantTypes := []string{
			"text", "bullets", "body_and_bullets", "body_and_lead",
			"bullet_groups", "table", "chart", "diagram", "image",
		}
		got := map[string]bool{}
		for _, v := range typeEnum {
			got[v] = true
		}
		for _, want := range wantTypes {
			if !got[want] {
				t.Errorf("ContentInput.type enum missing %q (got %v)", want, typeEnum)
			}
		}

		// allOf must contain one if/then per content type, each requiring its
		// matching *_value field and forbidding the others.
		allOf, ok := ci["allOf"].([]any)
		if !ok {
			t.Fatal("expected ContentInput.allOf to encode discriminator rules")
		}
		if len(allOf) != len(wantTypes) {
			t.Errorf("expected %d if/then branches, got %d", len(wantTypes), len(allOf))
		}

		expectedPairs := map[string]string{
			"text":             "text_value",
			"bullets":          "bullets_value",
			"body_and_bullets": "body_and_bullets_value",
			"body_and_lead":    "body_and_lead_value",
			"bullet_groups":    "bullet_groups_value",
			"table":            "table_value",
			"chart":            "chart_value",
			"diagram":          "diagram_value",
			"image":            "image_value",
		}
		allTypedKeys := []string{
			"text_value", "bullets_value", "body_and_bullets_value",
			"body_and_lead_value", "bullet_groups_value", "table_value",
			"chart_value", "diagram_value", "image_value",
		}

		seen := map[string]bool{}
		for _, branch := range allOf {
			bm, ok := branch.(map[string]any)
			if !ok {
				t.Fatalf("expected allOf entry to be an object, got %T", branch)
			}
			ifClause, ok := bm["if"].(map[string]any)
			if !ok {
				t.Fatalf("missing if clause in branch: %v", bm)
			}
			ifProps, ok := ifClause["properties"].(map[string]any)
			if !ok {
				t.Fatalf("missing if.properties: %v", ifClause)
			}
			typeSpec, ok := ifProps["type"].(map[string]any)
			if !ok {
				t.Fatalf("missing if.properties.type: %v", ifProps)
			}
			constVal, ok := typeSpec["const"].(string)
			if !ok {
				t.Fatalf("expected if.properties.type.const to be a string, got %T", typeSpec["const"])
			}
			wantValue, ok := expectedPairs[constVal]
			if !ok {
				t.Errorf("unexpected discriminator type %q", constVal)
				continue
			}
			seen[constVal] = true

			thenClause, ok := bm["then"].(map[string]any)
			if !ok {
				t.Fatalf("missing then clause for %q", constVal)
			}

			// (2) Required typed value field for this branch.
			thenReq := enumStrings(thenClause["required"])
			if len(thenReq) != 1 || thenReq[0] != wantValue {
				t.Errorf("type=%q: expected then.required=[%q], got %v", constVal, wantValue, thenClause["required"])
			}

			// (3) All OTHER typed value fields are forbidden (schema = false).
			thenProps, ok := thenClause["properties"].(map[string]any)
			if !ok {
				t.Fatalf("type=%q: missing then.properties", constVal)
			}
			for _, k := range allTypedKeys {
				if k == wantValue {
					if _, present := thenProps[k]; present {
						t.Errorf("type=%q: required field %q must not appear in then.properties (would forbid it)", constVal, k)
					}
					continue
				}
				v, present := thenProps[k]
				if !present {
					t.Errorf("type=%q: expected then.properties.%s = false, key missing", constVal, k)
					continue
				}
				if b, isBool := v.(bool); !isBool || b {
					t.Errorf("type=%q: expected then.properties.%s = false, got %v", constVal, k, v)
				}
			}
		}
		for _, want := range wantTypes {
			if !seen[want] {
				t.Errorf("missing if/then branch for type=%q", want)
			}
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

	t.Run("struct $defs forbid unknown keys", func(t *testing.T) {
		// All non-map object $defs must emit additionalProperties:false so
		// that schema-driven agents catch typos before calling validate_input.
		schema := buildInputSchema()
		defs := schema["$defs"].(map[string]any)
		for typeName, def := range defs {
			defMap, ok := def.(map[string]any)
			if !ok {
				continue
			}
			if defMap["type"] != "object" {
				continue
			}
			// Map-like objects intentionally use additionalProperties:<schema>;
			// non-map struct objects must use additionalProperties:false.
			if _, hasProps := defMap["properties"]; !hasProps {
				continue
			}
			ap, ok := defMap["additionalProperties"]
			if !ok {
				t.Errorf("%s: missing additionalProperties", typeName)
				continue
			}
			b, isBool := ap.(bool)
			if !isBool || b {
				t.Errorf("%s: expected additionalProperties=false, got %v", typeName, ap)
			}
		}
	})
}

// enumStrings normalizes an enum value (which may be []string from the builder
// or []any after JSON roundtrip) into []string for assertion convenience.
func enumStrings(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, x := range s {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
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
