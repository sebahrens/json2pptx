package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func publishedInputSchema(t *testing.T, tool mcp.Tool) map[string]any {
	t.Helper()
	wire, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal %s: %v", tool.Name, err)
	}
	var envelope struct {
		InputSchema map[string]any `json:"inputSchema"`
	}
	if err := json.Unmarshal(wire, &envelope); err != nil {
		t.Fatalf("decode %s: %v", tool.Name, err)
	}
	return envelope.InputSchema
}

func schemaProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	props, _ := schema["properties"].(map[string]any)
	prop, _ := props[name].(map[string]any)
	if prop == nil {
		t.Fatalf("missing %s property in schema", name)
	}
	return prop
}

func TestSemanticToolInputSchemaChoiceAndEnums(t *testing.T) {
	for _, tool := range []mcp.Tool{mcpValidateDeckSpecTool(), mcpRenderDeckSpecTool(), mcpExplainDeckSpecTool()} {
		t.Run(tool.Name, func(t *testing.T) {
			schema := publishedInputSchema(t, tool)
			if tool.Name == "validate_deck_spec" {
				spec := schemaProperty(t, schema, "spec")
				defs, _ := spec["$defs"].(map[string]any)
				if len(defs) == 0 {
					t.Fatal("published full DeckSpec lost its definitions")
				}
			}
			oneOf, _ := schema["oneOf"].([]any)
			if len(oneOf) != 2 {
				t.Fatalf("spec/deck_id oneOf = %v", schema["oneOf"])
			}
			for i, name := range []string{"spec", "deck_id"} {
				branch, _ := oneOf[i].(map[string]any)
				if !reflect.DeepEqual(branch["required"], []any{name}) {
					t.Errorf("oneOf[%d].required = %v, want %s", i, branch["required"], name)
				}
			}
			dependent, _ := schema["dependentRequired"].(map[string]any)
			if !reflect.DeepEqual(dependent["patch"], []any{"deck_id"}) {
				t.Errorf("patch dependency = %v", dependent["patch"])
			}
			patch := schemaProperty(t, schema, "patch")
			items, _ := patch["items"].(map[string]any)
			if !reflect.DeepEqual(items["required"], []any{"op", "path"}) {
				t.Errorf("patch item required = %v", items["required"])
			}
			op := schemaProperty(t, items, "op")
			if !reflect.DeepEqual(op["enum"], []any{"replace", "add", "remove"}) {
				t.Errorf("patch op enum = %v", op["enum"])
			}
			if tool.Name != "explain_deck_spec" {
				strict := schemaProperty(t, schema, "strict")
				if !reflect.DeepEqual(strict["enum"], []any{"off", "warn", "strict"}) {
					t.Errorf("strict enum = %v", strict["enum"])
				}
			}
		})
	}
	compile := publishedInputSchema(t, mcpCompileDeckSpecTool())
	if !reflect.DeepEqual(schemaProperty(t, compile, "strict")["enum"], []any{"off", "warn", "strict"}) {
		t.Error("compile_deck_spec strict enum missing")
	}
	render := publishedInputSchema(t, mcpRenderDeckSpecTool())
	if !reflect.DeepEqual(schemaProperty(t, render, "output_validation")["enum"], []any{"off", "warn", "strict"}) {
		t.Error("render_deck_spec output_validation enum missing")
	}
}

func TestPublishedListAndHintItemSchemas(t *testing.T) {
	for _, tt := range []struct {
		tool mcp.Tool
		arg  string
		want string
	}{
		{mcpRepairSlideTool(), "fixes", "object"},
		{mcpRepairSlidesBatchTool(), "fixes", "object"},
		{mcpScoreDeckTool(), "slide_indices", "integer"},
		{mcpRenderDeckThumbnailsTool(), "slide_indices", "integer"},
	} {
		t.Run(tt.tool.Name+"/"+tt.arg, func(t *testing.T) {
			prop := schemaProperty(t, publishedInputSchema(t, tt.tool), tt.arg)
			items, _ := prop["items"].(map[string]any)
			if items["type"] != tt.want {
				t.Errorf("%s.items.type = %v, want %s", tt.arg, items["type"], tt.want)
			}
		})
	}
	for _, tool := range []mcp.Tool{mcpRecommendPatternTool(), mcpRecommendVisualTool()} {
		hints := schemaProperty(t, publishedInputSchema(t, tool), "content_hints")
		for _, name := range []string{"item_count", "has_chart", "has_metrics", "columns", "density_hint"} {
			_ = schemaProperty(t, hints, name)
		}
		if tool.Name == "recommend_visual" {
			for _, name := range []string{"data_points", "series_count", "audience"} {
				_ = schemaProperty(t, hints, name)
			}
		}
	}
}
