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
