package core_test

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
)

func TestValidateUnknownFields(t *testing.T) {
	schema := core.ObjectDataSchema("test", map[string]*core.DataSchema{
		"name":  core.StringDataSchema("Name"),
		"value": core.NumberDataSchema("Value"),
	}, []string{"name"})

	t.Run("accepts known fields", func(t *testing.T) {
		data := map[string]any{"name": "foo", "value": 42}
		err := core.ValidateUnknownFields(data, schema, "test_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		data := map[string]any{"name": "foo", "bogus": "bad"}
		err := core.ValidateUnknownFields(data, schema, "test_type")
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
		ves := core.GetValidationErrors(err)
		if len(ves) != 1 {
			t.Fatalf("expected 1 error, got %d", len(ves))
		}
		if ves[0].Code != core.ErrCodeUnknownField {
			t.Errorf("expected code %s, got %s", core.ErrCodeUnknownField, ves[0].Code)
		}
		if ves[0].Field != "data.bogus" {
			t.Errorf("expected field data.bogus, got %s", ves[0].Field)
		}
	})

	t.Run("multiple unknown fields", func(t *testing.T) {
		data := map[string]any{"name": "foo", "bad1": 1, "bad2": 2}
		err := core.ValidateUnknownFields(data, schema, "test_type")
		if err == nil {
			t.Fatal("expected error")
		}
		ves := core.GetValidationErrors(err)
		if len(ves) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(ves))
		}
	})

	t.Run("nil schema allows everything", func(t *testing.T) {
		data := map[string]any{"anything": "goes"}
		err := core.ValidateUnknownFields(data, nil, "test_type")
		if err != nil {
			t.Fatalf("nil schema should allow everything: %v", err)
		}
	})
}

func TestValidateUnknownFieldsNested(t *testing.T) {
	item := core.ObjectDataSchema("item", map[string]*core.DataSchema{
		"label":  core.StringDataSchema("Label"),
		"legacy": core.ToleratedDataSchema("Accepted, not drawn"),
	}, nil)
	schema := core.ObjectDataSchema("test", map[string]*core.DataSchema{
		"items": core.ArrayDataSchema("Items", item, 0),
	}, nil)

	data := map[string]any{"items": []any{
		map[string]any{"title": "a", "legacy": 1},
		"plain string",
		map[string]any{"title": "b"},
		map[string]any{"lable": "c"},
	}}
	ves := core.GetValidationErrors(core.ValidateUnknownFields(data, schema, "t"))
	if len(ves) != 2 {
		t.Fatalf("want 2 errors (title deduped, lable), got %+v", ves)
	}
	if ves[0].Field != "data.items[0].title" || ves[0].Occurrences != 2 || ves[0].DidYouMean != "label" {
		t.Errorf("title error = %+v", ves[0])
	}
	if len(ves[0].Expected) != 1 || ves[0].Expected[0] != "label" {
		t.Errorf("expected keys must omit tolerated ones: %v", ves[0].Expected)
	}
	if ves[1].Field != "data.items[3].lable" || ves[1].DidYouMean != "label" {
		t.Errorf("lable error = %+v", ves[1])
	}
}

func TestSuggestField(t *testing.T) {
	expected := []string{"date", "label", "start", "values"}
	for key, want := range map[string]string{
		"when":   "date",
		"title":  "label",
		"vaules": "values",
		"Label":  "label",
		"zzzzzz": "",
	} {
		if got := core.SuggestField(key, expected); got != want {
			t.Errorf("SuggestField(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestValidateUnknownFieldsInRegistry(t *testing.T) {
	// mockWithSchema implements DiagramWithSchema.
	schema := core.ObjectDataSchema("test", map[string]*core.DataSchema{
		"allowed": core.StringDataSchema("An allowed field"),
	}, nil)

	m := &mockDiagramWithSchema{
		BaseDiagram: core.NewBaseDiagram("schema_test"),
		schema:      schema,
	}

	r := core.NewRegistry()
	r.Register(m)

	t.Run("rejects via Render", func(t *testing.T) {
		req := &core.RequestEnvelope{
			Type: "schema_test",
			Data: map[string]any{"allowed": "ok", "forbidden": "nope"},
		}
		_, err := r.Render(req)
		if err == nil {
			t.Fatal("expected error for unknown field in Render")
		}
	})

	t.Run("accepts via Render", func(t *testing.T) {
		req := &core.RequestEnvelope{
			Type: "schema_test",
			Data: map[string]any{"allowed": "ok"},
		}
		_, err := r.Render(req)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestDataSchemaJSON(t *testing.T) {
	schema := core.ObjectDataSchema("test object", map[string]*core.DataSchema{
		"name": core.StringDataSchema("The name"),
		"size": core.NumberDataSchema("The size"),
	}, []string{"name"})

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("expected type=object, got %v", parsed["type"])
	}
	if parsed["additionalProperties"] != false {
		t.Errorf("expected additionalProperties=false, got %v", parsed["additionalProperties"])
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be an object")
	}
	if _, ok := props["name"]; !ok {
		t.Error("expected name property")
	}
	if _, ok := props["size"]; !ok {
		t.Error("expected size property")
	}
}

// mockDiagramWithSchema implements DiagramWithSchema for testing.
type mockDiagramWithSchema struct {
	core.BaseDiagram
	schema *core.DataSchema
}

func (m *mockDiagramWithSchema) Render(req *core.RequestEnvelope) (*core.SVGDocument, error) {
	return &core.SVGDocument{Content: []byte("<svg></svg>"), Width: 100, Height: 100}, nil
}

func (m *mockDiagramWithSchema) Validate(req *core.RequestEnvelope) error {
	return nil
}

func (m *mockDiagramWithSchema) DataSchema() *core.DataSchema {
	return m.schema
}
