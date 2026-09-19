package main

import (
	"context"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-h8o7: the MCP input schema for spec must carry the real
// DeckSpec schema — slides[].items.oneOf with one closed variant per kind —
// instead of an empty object.
func TestSemanticMCP_SpecInputSchemaHasPerKindOneOf(t *testing.T) {
	kinds := semantic.AllSlideKinds()
	for _, tool := range []struct {
		name string
		spec map[string]any
	}{
		{"render_deck_spec", toolSpecProperty(t, mcpRenderDeckSpecTool().InputSchema.Properties)},
		{"validate_deck_spec", toolSpecProperty(t, mcpValidateDeckSpecTool().InputSchema.Properties)},
		{"compile_deck_spec", toolSpecProperty(t, mcpCompileDeckSpecTool().InputSchema.Properties)},
		{"explain_deck_spec", toolSpecProperty(t, mcpExplainDeckSpecTool().InputSchema.Properties)},
	} {
		if tool.spec["type"] != "object" {
			t.Errorf("%s: spec.type = %v, want object", tool.name, tool.spec["type"])
		}
		if tool.spec["additionalProperties"] != false {
			t.Errorf("%s: spec must be closed (additionalProperties:false)", tool.name)
		}
		props, _ := tool.spec["properties"].(map[string]any)
		slides, _ := props["slides"].(map[string]any)
		items, _ := slides["items"].(map[string]any)
		oneOf, _ := items["oneOf"].([]any)
		if len(oneOf) != len(kinds) {
			t.Fatalf("%s: slides.items.oneOf has %d variants, want %d (one per kind)", tool.name, len(oneOf), len(kinds))
		}
		seen := map[string]bool{}
		for _, v := range oneOf {
			variant, _ := v.(map[string]any)
			if _, isRef := variant["$ref"]; isRef {
				t.Fatalf("%s: variant is an unresolved $ref: %v", tool.name, variant)
			}
			if variant["additionalProperties"] != false {
				t.Errorf("%s: variant %v must set additionalProperties:false", tool.name, variant["title"])
			}
			vp, _ := variant["properties"].(map[string]any)
			kind, _ := vp["kind"].(map[string]any)
			if c, ok := kind["const"].(string); ok {
				seen[c] = true
			}
		}
		for _, k := range kinds {
			if !seen[string(k)] {
				t.Errorf("%s: no oneOf variant pins kind %q", tool.name, k)
			}
		}
	}
}

func toolSpecProperty(t *testing.T, props map[string]any) map[string]any {
	t.Helper()
	spec, ok := props["spec"].(map[string]any)
	if !ok {
		t.Fatalf("tool has no spec property: %v", props)
	}
	return spec
}

// Unknown fields in a kpi payload surface over MCP as SEMANTIC_UNKNOWN_FIELD
// naming the dropped path.
func TestSemanticMCP_UnknownKPIFieldDiagnostic(t *testing.T) {
	spec := map[string]any{
		"meta": map[string]any{"title": "Deck"},
		"slides": []any{map[string]any{
			"kind": "kpi_snapshot", "title": "KPIs", "takeaway": "Up.",
			"kpis": []any{
				map[string]any{"value": "$4M", "label": "ARR"},
				map[string]any{"value": "118%", "lable": "NRR"},
			},
		}},
	}
	res, err := testValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil || res.IsError {
		t.Fatalf("validate_deck_spec failed: %v %v", err, res)
	}
	var env diagnostics.FindingEnvelope
	structuredInto(t, res.StructuredContent, &env)
	found := false
	for _, f := range env.Findings {
		if f.Evidence["path"] == "slides[0].kpis[1].lable" && strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownField) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a finding at slides[0].kpis[1].lable, got %+v", env.Findings)
	}
}

// Every list_slide_kinds example must validate clean through validate_deck_spec.
func TestSemanticMCP_ListSlideKindsExamplesValidate(t *testing.T) {
	ctx := context.Background()
	res, err := handleListSlideKinds(ctx, makeRequest(map[string]any{}))
	if err != nil || res.IsError {
		t.Fatalf("list_slide_kinds failed: %v", err)
	}
	var out struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, res.StructuredContent, &out)
	if len(out.SlideKinds) != len(semantic.AllSlideKinds()) {
		t.Fatalf("got %d kinds, want %d", len(out.SlideKinds), len(semantic.AllSlideKinds()))
	}
	for _, k := range out.SlideKinds {
		if k.Example == nil || k.ItemSchema == nil {
			t.Errorf("%s: missing example or item_schema", k.Kind)
			continue
		}
		if k.ItemSchema["additionalProperties"] != false {
			t.Errorf("%s: item_schema must be closed", k.Kind)
		}
		spec := map[string]any{
			"meta":   map[string]any{"title": "Example deck"},
			"slides": []any{k.Example},
		}
		vres, err := testValidateDeckSpec(ctx, makeRequest(map[string]any{"spec": spec, "strict": "strict"}))
		if err != nil || vres.IsError {
			t.Fatalf("%s: validate_deck_spec failed: %v", k.Kind, err)
		}
		var env diagnostics.FindingEnvelope
		structuredInto(t, vres.StructuredContent, &env)
		if !env.OK || len(env.Findings) != 0 {
			t.Errorf("%s: example does not validate clean: %+v", k.Kind, env.Findings)
		}
	}
}
