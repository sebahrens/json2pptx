package svggen

import (
	"errors"
	"slices"
	"testing"
)

// mockDiagram is a test diagram implementation
type mockDiagram struct {
	typeID      string
	renderErr   error
	validateErr error
}

func (m *mockDiagram) Type() string { return m.typeID }

func (m *mockDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	if m.renderErr != nil {
		return nil, m.renderErr
	}
	return &SVGDocument{
		Content: []byte("<svg></svg>"),
		Width:   100,
		Height:  100,
	}, nil
}

func (m *mockDiagram) Validate(req *RequestEnvelope) error {
	return m.validateErr
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	// Register should work
	d := &mockDiagram{typeID: "test_type"}
	r.Register(d)

	// Should be retrievable
	got := r.Get("test_type")
	if got == nil {
		t.Error("Get() returned nil after Register()")
	}
	if got.Type() != "test_type" {
		t.Errorf("Get() returned wrong type: %v", got.Type())
	}
}

func TestRegistry_RegisterPanic(t *testing.T) {
	r := NewRegistry()

	d1 := &mockDiagram{typeID: "duplicate"}
	d2 := &mockDiagram{typeID: "duplicate"}

	r.Register(d1)

	// Second registration should panic
	defer func() {
		if recover() == nil {
			t.Error("Register() should panic on duplicate type")
		}
	}()
	r.Register(d2)
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	// Get non-existent type
	got := r.Get("nonexistent")
	if got != nil {
		t.Error("Get() should return nil for non-existent type")
	}

	// Register and get
	r.Register(&mockDiagram{typeID: "exists"})
	got = r.Get("exists")
	if got == nil {
		t.Error("Get() should return diagram after registration")
	}
}

func TestRegistry_Alias(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{typeID: "funnel_chart"})
	r.Alias("funnel", "funnel_chart")

	// Get via alias should work
	got := r.Get("funnel")
	if got == nil {
		t.Fatal("Get() via alias returned nil")
	}
	if got.Type() != "funnel_chart" {
		t.Errorf("Get() via alias returned wrong type: %v", got.Type())
	}

	// Get via canonical name should still work
	got = r.Get("funnel_chart")
	if got == nil {
		t.Fatal("Get() via canonical name returned nil after alias registration")
	}

	// Alias to non-existent type returns nil
	r.Alias("bogus_alias", "nonexistent")
	got = r.Get("bogus_alias")
	if got != nil {
		t.Error("Get() via alias to nonexistent type should return nil")
	}
}

func TestRegistry_AliasConflictPanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{typeID: "existing_type"})

	defer func() {
		if recover() == nil {
			t.Error("Alias() should panic when alias conflicts with registered type")
		}
	}()
	r.Alias("existing_type", "other_type")
}

func TestRegistry_BuiltinFunnelAlias(t *testing.T) {
	// Verify the "funnel" alias works via the default registry
	d := DefaultRegistry().Get("funnel")
	if d == nil {
		t.Fatal("Default registry should resolve 'funnel' alias to funnel_chart")
	}
	if d.Type() != "funnel_chart" {
		t.Errorf("'funnel' alias resolved to %q, want %q", d.Type(), "funnel_chart")
	}
}

func TestRegistry_BuiltinLLMAliases(t *testing.T) {
	// Verify LLM-commonly-generated aliases resolve to canonical types.
	tests := []struct {
		alias     string
		canonical string
	}{
		{"orgchart", "org_chart"},
		{"org", "org_chart"},
		{"bar-stacked", "stacked_bar_chart"},
		{"stacked_bar", "stacked_bar_chart"},
		{"stacked_area", "stacked_area_chart"},
		{"grouped_bar", "grouped_bar_chart"},
		{"matrix", "matrix_2x2"},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			d := DefaultRegistry().Get(tt.alias)
			if d == nil {
				t.Fatalf("Default registry should resolve %q alias", tt.alias)
			}
			if d.Type() != tt.canonical {
				t.Errorf("%q alias resolved to %q, want %q", tt.alias, d.Type(), tt.canonical)
			}
		})
	}
}

func TestRegistry_RenderViaAlias(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{typeID: "funnel_chart"})
	r.Alias("funnel", "funnel_chart")

	req := &RequestEnvelope{
		Type: "funnel",
		Data: map[string]any{"key": "value"},
	}

	doc, err := r.Render(req)
	if err != nil {
		t.Fatalf("Render() via alias failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Render() via alias returned nil document")
	}
}

func TestRegistry_Types(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockDiagram{typeID: "type_a"})
	r.Register(&mockDiagram{typeID: "type_b"})
	r.Register(&mockDiagram{typeID: "type_c"})

	types := r.Types()
	slices.Sort(types)

	want := []string{"type_a", "type_b", "type_c"}
	if len(types) != len(want) {
		t.Errorf("Types() length = %d, want %d", len(types), len(want))
	}
	for i, typ := range types {
		if typ != want[i] {
			t.Errorf("Types()[%d] = %v, want %v", i, typ, want[i])
		}
	}
}

func TestRegistry_Render(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{typeID: "test"})

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid request",
			req: &RequestEnvelope{
				Type: "test",
				Data: map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "missing type",
			req: &RequestEnvelope{
				Data: map[string]any{"key": "value"},
			},
			wantErr: true,
		},
		{
			name: "unknown type",
			req: &RequestEnvelope{
				Type: "unknown",
				Data: map[string]any{"key": "value"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := r.Render(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && doc == nil {
				t.Error("Render() returned nil document without error")
			}
		})
	}
}

func TestRegistry_RenderValidationError(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{
		typeID:      "validating",
		validateErr: errors.New("validation failed"),
	})

	req := &RequestEnvelope{
		Type: "validating",
		Data: map[string]any{"key": "value"},
	}

	_, err := r.Render(req)
	if err == nil {
		t.Error("Render() should return validation error")
	}
}

func TestRegistry_RenderError(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDiagram{
		typeID:    "erroring",
		renderErr: errors.New("render failed"),
	})

	req := &RequestEnvelope{
		Type: "erroring",
		Data: map[string]any{"key": "value"},
	}

	_, err := r.Render(req)
	if err == nil {
		t.Error("Render() should return render error")
	}
}

func TestMustRegister(t *testing.T) {
	r := NewRegistry()

	// MustRegister returns the registry for chaining
	got := r.MustRegister(&mockDiagram{typeID: "chained"})
	if got != r {
		t.Error("MustRegister() should return the registry")
	}

	// Should be registered
	if r.Get("chained") == nil {
		t.Error("MustRegister() did not register the diagram")
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Use SetDefaultRegistry for test isolation
	oldDefault := SetDefaultRegistry(NewRegistry())
	defer SetDefaultRegistry(oldDefault)

	// Test package-level functions
	Register(&mockDiagram{typeID: "pkg_level"})

	types := Types()
	if len(types) != 1 || types[0] != "pkg_level" {
		t.Errorf("Types() = %v, want [pkg_level]", types)
	}

	req := &RequestEnvelope{
		Type: "pkg_level",
		Data: map[string]any{"key": "value"},
	}
	doc, err := Render(req)
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}
	if doc == nil {
		t.Error("Render() returned nil document")
	}
}

func TestSetDefaultRegistry(t *testing.T) {
	// Save original
	original := DefaultRegistry()

	// Create a custom registry
	custom := NewRegistry()
	custom.Register(&mockDiagram{typeID: "custom"})

	// Set the custom registry
	old := SetDefaultRegistry(custom)
	defer SetDefaultRegistry(old)

	// Verify the custom registry is now default
	if DefaultRegistry() != custom {
		t.Error("SetDefaultRegistry did not set the registry")
	}

	// Verify we can use it via package-level functions
	types := Types()
	if len(types) != 1 || types[0] != "custom" {
		t.Errorf("Types() = %v, want [custom]", types)
	}

	// Restore and verify
	SetDefaultRegistry(original)
	if DefaultRegistry() != original {
		t.Error("failed to restore original registry")
	}
}

func TestResetDefaultRegistry(t *testing.T) {
	// Register something in the default
	custom := NewRegistry()
	custom.Register(&mockDiagram{typeID: "test"})
	old := SetDefaultRegistry(custom)
	defer SetDefaultRegistry(old)

	// Verify something is registered
	if len(Types()) != 1 {
		t.Error("expected one registered type before reset")
	}

	// Reset
	ResetDefaultRegistry()

	// Verify registry is fresh
	if len(Types()) != 0 {
		t.Error("expected empty registry after reset")
	}
}

// mockDiagramWithBuilder implements DiagramWithBuilder for testing multi-format rendering.
type mockDiagramWithBuilder struct {
	typeID      string
	renderErr   error
	validateErr error
}

func (m *mockDiagramWithBuilder) Type() string { return m.typeID }

func (m *mockDiagramWithBuilder) Render(req *RequestEnvelope) (*SVGDocument, error) {
	_, doc, err := m.RenderWithBuilder(req)
	return doc, err
}

func (m *mockDiagramWithBuilder) Validate(req *RequestEnvelope) error {
	return m.validateErr
}

func (m *mockDiagramWithBuilder) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	if m.renderErr != nil {
		return nil, nil, m.renderErr
	}

	// Create a simple builder and draw something
	width := float64(req.Output.Width)
	if width == 0 {
		width = 100
	}
	height := float64(req.Output.Height)
	if height == 0 {
		height = 100
	}

	builder := NewSVGBuilder(width, height)
	c, _ := ParseColor("#336699")
	builder.SetFillColor(c)
	builder.FillRect(Rect{X: 0, Y: 0, W: width, H: height})

	doc, err := builder.Render()
	if err != nil {
		return nil, nil, err
	}

	return builder, doc, nil
}

func TestRegistry_RenderMultiFormat(t *testing.T) {
	t.Run("DiagramWithBuilder - SVG only", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockDiagramWithBuilder{typeID: "test_builder"})

		req := &RequestEnvelope{
			Type: "test_builder",
			Data: map[string]any{"key": "value"},
		}

		result, err := renderMultiFormat(r, req)
		if err != nil {
			t.Fatalf("RenderMultiFormat() error = %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should be present")
		}
		if result.PNG != nil {
			t.Error("PNG should not be present when not requested")
		}
		if result.Format != "svg" {
			t.Errorf("Format = %v, want svg", result.Format)
		}
	})

	t.Run("DiagramWithBuilder - with PNG", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockDiagramWithBuilder{typeID: "test_builder"})

		req := &RequestEnvelope{
			Type: "test_builder",
			Data: map[string]any{"key": "value"},
			Output: OutputSpec{
				Width:  200,
				Height: 150,
				Scale:  2.0,
			},
		}

		result, err := renderMultiFormat(r, req, "png")
		if err != nil {
			t.Fatalf("RenderMultiFormat() error = %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should be present")
		}
		if result.PNG == nil {
			t.Error("PNG should be present when requested")
		}
		// Verify PNG header
		if len(result.PNG) < 4 {
			t.Fatal("PNG output too short")
		}
		if result.PNG[0] != 0x89 || result.PNG[1] != 0x50 || result.PNG[2] != 0x4E || result.PNG[3] != 0x47 {
			t.Error("Invalid PNG header")
		}
	})

	t.Run("DiagramWithBuilder - with PDF", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockDiagramWithBuilder{typeID: "test_builder"})

		req := &RequestEnvelope{
			Type: "test_builder",
			Data: map[string]any{"key": "value"},
		}

		result, err := renderMultiFormat(r, req, "pdf")
		if err != nil {
			t.Fatalf("RenderMultiFormat() error = %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should be present")
		}
		if result.PDF == nil {
			t.Error("PDF should be present when requested")
		}
		// Verify PDF header
		if len(result.PDF) < 5 {
			t.Fatal("PDF output too short")
		}
		header := string(result.PDF[:5])
		if header != "%PDF-" {
			t.Errorf("Invalid PDF header: %q", header)
		}
	})

	t.Run("DiagramWithBuilder - format from request", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockDiagramWithBuilder{typeID: "test_builder"})

		req := &RequestEnvelope{
			Type: "test_builder",
			Data: map[string]any{"key": "value"},
			Output: OutputSpec{
				Format: "png",
			},
		}

		result, err := renderMultiFormat(r, req)
		if err != nil {
			t.Fatalf("RenderMultiFormat() error = %v", err)
		}
		if result.PNG == nil {
			t.Error("PNG should be present when format=png in request")
		}
		if result.Format != "png" {
			t.Errorf("Format = %v, want png", result.Format)
		}
	})

	t.Run("Regular Diagram - error on PNG request", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockDiagram{typeID: "regular"})

		req := &RequestEnvelope{
			Type: "regular",
			Data: map[string]any{"key": "value"},
		}

		_, err := renderMultiFormat(r, req, "png")
		if err == nil {
			t.Error("Expected error when requesting PNG from diagram without builder support")
		}
	})

	t.Run("Unknown diagram type", func(t *testing.T) {
		r := NewRegistry()

		req := &RequestEnvelope{
			Type: "unknown",
			Data: map[string]any{"key": "value"},
		}

		_, err := renderMultiFormat(r, req)
		if err == nil {
			t.Error("Expected error for unknown diagram type")
		}
	})
}

func TestRenderMultiFormat_PackageLevel(t *testing.T) {
	// Use SetDefaultRegistry for test isolation
	oldDefault := SetDefaultRegistry(NewRegistry())
	defer SetDefaultRegistry(oldDefault)

	Register(&mockDiagramWithBuilder{typeID: "test"})

	req := &RequestEnvelope{
		Type: "test",
		Data: map[string]any{"key": "value"},
	}

	result, err := RenderMultiFormat(req, "png")
	if err != nil {
		t.Fatalf("RenderMultiFormat() error = %v", err)
	}
	if result.SVG == nil || result.PNG == nil {
		t.Error("Both SVG and PNG should be present")
	}
}
