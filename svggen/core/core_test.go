package core_test

import (
	"encoding/json"
	"testing"

	"github.com/ahrens/svggen/core"
)

// mockDiagram is a minimal Diagram for testing the registry.
type mockDiagram struct {
	core.BaseDiagram
}

func (m *mockDiagram) Render(req *core.RequestEnvelope) (*core.SVGDocument, error) {
	return &core.SVGDocument{
		Content: []byte("<svg></svg>"),
		Width:   100,
		Height:  100,
	}, nil
}

func (m *mockDiagram) Validate(req *core.RequestEnvelope) error {
	return nil
}

func TestCoreRegistry(t *testing.T) {
	r := core.NewRegistry()
	r.Register(&mockDiagram{core.NewBaseDiagram("test_type")})

	// Verify registration
	types := r.Types()
	if len(types) != 1 || types[0] != "test_type" {
		t.Errorf("Types() = %v, want [test_type]", types)
	}

	// Verify get
	d := r.Get("test_type")
	if d == nil {
		t.Fatal("Get(test_type) returned nil")
	}
	if d.Type() != "test_type" {
		t.Errorf("Type() = %q, want test_type", d.Type())
	}

	// Verify render
	req := &core.RequestEnvelope{
		Type: "test_type",
		Data: map[string]any{"key": "value"},
	}
	doc, err := r.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Error("Render() returned empty document")
	}
}

func TestCoreAlias(t *testing.T) {
	r := core.NewRegistry()
	r.Register(&mockDiagram{core.NewBaseDiagram("canonical")})
	r.Alias("short", "canonical")

	d := r.Get("short")
	if d == nil {
		t.Fatal("alias lookup returned nil")
	}
	if d.Type() != "canonical" {
		t.Errorf("Type() = %q, want canonical", d.Type())
	}
}

func TestCoreTypesRoundtrip(t *testing.T) {
	req := &core.RequestEnvelope{
		Type:  "bar_chart",
		Title: "Test Chart",
		Data:  map[string]any{"values": []any{1.0, 2.0, 3.0}},
		Output: core.OutputSpec{
			Format: "svg",
			Width:  800,
			Height: 600,
		},
		Style: core.StyleSpec{
			Palette:    core.PaletteSpec{Name: "vibrant"},
			FontFamily: "Arial",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	parsed, err := core.ParseRequest(data)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}
	if parsed.Type != "bar_chart" {
		t.Errorf("Type = %q, want bar_chart", parsed.Type)
	}
	if parsed.Style.Palette.Name != "vibrant" {
		t.Errorf("Palette.Name = %q, want vibrant", parsed.Style.Palette.Name)
	}
}

func TestCoreDefaultRegistry(t *testing.T) {
	// Default registry should be empty (no diagrams registered without importing root)
	r := core.DefaultRegistry()
	types := r.Types()
	// Note: in this test (core_test package), no diagrams are registered
	// because we don't import the root svggen/ package
	if len(types) != 0 {
		t.Errorf("DefaultRegistry().Types() = %v, want empty (no diagrams in core/)", types)
	}
}

func TestCoreValidationError(t *testing.T) {
	err := &core.ValidationError{
		Field:   "data.values",
		Code:    core.ErrCodeRequired,
		Message: "values is required",
	}
	if !core.IsValidationError(err) {
		t.Error("IsValidationError() = false, want true")
	}
	errs := core.GetValidationErrors(err)
	if len(errs) != 1 {
		t.Errorf("GetValidationErrors() returned %d errors, want 1", len(errs))
	}
}
