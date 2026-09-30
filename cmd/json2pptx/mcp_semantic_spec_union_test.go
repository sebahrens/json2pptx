package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// go-slide-creator-b7qqg.5: every DeckSpec tool documents spec as "a JSON
// object … or a raw YAML/JSON string", and the handler accepts both. The
// advertised input schema must too, or a host that validates arguments against
// tools/list rejects the documented YAML before the server sees it. The root
// oneOf (slides XOR structure) used to reject every string: "required" holds
// vacuously for a non-object, so both "not required" branches failed.
func TestDeckSpecToolSchemasAcceptDocumentedSpecForms(t *testing.T) {
	yamlSpec, err := os.ReadFile("../../examples/semantic/qbr.yaml")
	if err != nil {
		t.Fatal(err)
	}
	objectSpec := decodeSpecObject(t, archOnModernSpec)
	specTools := map[string]bool{"validate_deck_spec": true, "render_deck_spec": true, "compile_deck_spec": true, "explain_deck_spec": true}

	for _, profile := range []string{toolProfileDeckSpec, toolProfileCore, toolProfileAll} {
		raw, _ := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), profile))
		var wire struct {
			Result struct {
				Tools []struct {
					Name        string          `json:"name"`
					InputSchema json.RawMessage `json:"inputSchema"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("%s: decode tools/list: %v", profile, err)
		}
		seen := 0
		for _, tool := range wire.Result.Tools {
			if !specTools[tool.Name] {
				continue
			}
			seen++
			schema := compileToolInputSchema(t, profile+"/"+tool.Name, tool.InputSchema)
			valid := func(name string, args map[string]any) {
				t.Helper()
				if err := schema.Validate(roundTripJSON(t, args)); err != nil {
					t.Errorf("%s/%s: %s rejected by the advertised schema: %v", profile, tool.Name, name, err)
				}
			}
			invalid := func(name string, args map[string]any) {
				t.Helper()
				if err := schema.Validate(roundTripJSON(t, args)); err == nil {
					t.Errorf("%s/%s: %s accepted by the advertised schema", profile, tool.Name, name)
				}
			}
			valid("YAML string spec (examples/semantic/qbr.yaml)", map[string]any{"spec": string(yamlSpec)})
			valid("JSON text spec", map[string]any{"spec": archOnModernSpec})
			valid("object spec", map[string]any{"spec": objectSpec})
			invalid("numeric spec", map[string]any{"spec": 42})
			invalid("empty string spec", map[string]any{"spec": ""})
			invalid("object with both slides and structure", map[string]any{"spec": map[string]any{
				"slides": []any{map[string]any{"kind": "title", "title": "T"}}, "structure": map[string]any{"sections": []any{map[string]any{}}},
			}})
		}
		if seen == 0 {
			t.Errorf("%s: advertises no DeckSpec tool", profile)
		}
	}
}

func compileToolInputSchema(t *testing.T, name string, raw json.RawMessage) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s: decode input schema: %v", name, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	const url = "mem://input.json"
	if err := c.AddResource(url, doc); err != nil {
		t.Fatalf("%s: add schema: %v", name, err)
	}
	schema, err := c.Compile(url)
	if err != nil {
		t.Fatalf("%s: compile input schema: %v", name, err)
	}
	return schema
}

func roundTripJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
