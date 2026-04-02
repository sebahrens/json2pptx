package svggen

import (
	"fmt"
	"testing"
)

// diagramFunc is a function adapter for the Diagram interface.
// It allows registering simple render functions without full interface implementation.
// Used only in tests.
type diagramFunc struct {
	typeID     string
	renderFunc func(*RequestEnvelope) (*SVGDocument, error)
	validFunc  func(*RequestEnvelope) error
}

func (f *diagramFunc) Type() string {
	return f.typeID
}

func (f *diagramFunc) Render(req *RequestEnvelope) (*SVGDocument, error) {
	if f.renderFunc == nil {
		return nil, fmt.Errorf("svggen: render not implemented for %s", f.typeID)
	}
	return f.renderFunc(req)
}

func (f *diagramFunc) Validate(req *RequestEnvelope) error {
	if f.validFunc != nil {
		return f.validFunc(req)
	}
	return nil
}

func TestRequestEnvelope_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid request",
			req: RequestEnvelope{
				Type: "bar_chart",
				Data: map[string]any{"a": 1, "b": 2},
			},
			wantErr: false,
		},
		{
			name: "missing type",
			req: RequestEnvelope{
				Data: map[string]any{"a": 1},
			},
			wantErr: true,
		},
		{
			name: "missing data",
			req: RequestEnvelope{
				Type: "bar_chart",
			},
			wantErr: true,
		},
		{
			name: "empty type",
			req: RequestEnvelope{
				Type: "",
				Data: map[string]any{"a": 1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid JSON",
			json:    `{"type": "bar_chart", "data": {"a": 1, "b": 2}}`,
			wantErr: false,
		},
		{
			name:    "with optional fields",
			json:    `{"type": "bar_chart", "title": "Sales", "data": {"a": 1}, "output": {"width": 800}}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "missing type",
			json:    `{"data": {"a": 1}}`,
			wantErr: true,
		},
		{
			name:    "missing data",
			json:    `{"type": "bar_chart"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && req == nil {
				t.Error("ParseRequest() returned nil request without error")
			}
		})
	}
}

func TestSVGDocument_String(t *testing.T) {
	doc := &SVGDocument{
		Content: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		Width:   100,
		Height:  100,
	}

	got := doc.String()
	want := `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
	if got != want {
		t.Errorf("String() = %v, want %v", got, want)
	}
}

func TestSVGDocument_Bytes(t *testing.T) {
	content := []byte(`<svg></svg>`)
	doc := &SVGDocument{Content: content}

	got := doc.Bytes()
	if string(got) != string(content) {
		t.Errorf("Bytes() = %v, want %v", got, content)
	}
}

func TestDiagramFunc(t *testing.T) {
	// Create a simple diagram function
	df := &diagramFunc{
		typeID: "test_diagram",
		renderFunc: func(req *RequestEnvelope) (*SVGDocument, error) {
			return &SVGDocument{Content: []byte("<svg></svg>")}, nil
		},
		validFunc: func(req *RequestEnvelope) error {
			return nil
		},
	}

	// Test Type()
	if got := df.Type(); got != "test_diagram" {
		t.Errorf("Type() = %v, want test_diagram", got)
	}

	// Test Render()
	req := &RequestEnvelope{Type: "test_diagram", Data: map[string]any{}}
	doc, err := df.Render(req)
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}
	if doc == nil {
		t.Error("Render() returned nil document")
	}

	// Test Validate()
	if err := df.Validate(req); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestDiagramFunc_NilFuncs(t *testing.T) {
	df := &diagramFunc{
		typeID: "nil_funcs",
	}

	req := &RequestEnvelope{Type: "nil_funcs", Data: map[string]any{}}

	// Render with nil func should error
	_, err := df.Render(req)
	if err == nil {
		t.Error("Render() with nil func should error")
	}

	// Validate with nil func should pass
	if err := df.Validate(req); err != nil {
		t.Errorf("Validate() with nil func should pass, got %v", err)
	}
}
