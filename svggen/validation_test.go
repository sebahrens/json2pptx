package svggen

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		contains []string
	}{
		{
			name: "with field",
			err: ValidationError{
				Field:   "data.values",
				Code:    ErrCodeRequired,
				Message: "field is required",
			},
			contains: []string{"data.values", "REQUIRED", "field is required"},
		},
		{
			name: "without field",
			err: ValidationError{
				Code:    ErrCodeParseFailed,
				Message: "invalid JSON",
			},
			contains: []string{"PARSE_FAILED", "invalid JSON"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, s := range tt.contains {
				if !strings.Contains(errStr, s) {
					t.Errorf("Error() = %q, want to contain %q", errStr, s)
				}
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	errs := &ValidationErrors{}

	// Initially no errors
	if errs.HasErrors() {
		t.Error("HasErrors() should be false initially")
	}
	if errs.AsError() != nil {
		t.Error("AsError() should be nil initially")
	}

	// Add one error
	errs.Add(ValidationError{
		Field:   "type",
		Code:    ErrCodeRequired,
		Message: "type is required",
	})

	if !errs.HasErrors() {
		t.Error("HasErrors() should be true after adding error")
	}
	if errs.AsError() == nil {
		t.Error("AsError() should not be nil after adding error")
	}

	// Error message for single error
	errStr := errs.Error()
	if !strings.Contains(errStr, "type") {
		t.Errorf("Error() = %q, want to contain 'type'", errStr)
	}

	// Add second error
	errs.Add(ValidationError{
		Field:   "data",
		Code:    ErrCodeRequired,
		Message: "data is required",
	})

	errStr = errs.Error()
	if !strings.Contains(errStr, "2 validation errors") {
		t.Errorf("Error() = %q, want to contain '2 validation errors'", errStr)
	}
}

func TestDecoder_DecodeJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		errCode  string
		checkReq func(*testing.T, *RequestEnvelope)
	}{
		{
			name:    "valid minimal request",
			json:    `{"type": "bar_chart", "data": {"a": 1}}`,
			wantErr: false,
			checkReq: func(t *testing.T, req *RequestEnvelope) {
				if req.Type != "bar_chart" {
					t.Errorf("Type = %q, want bar_chart", req.Type)
				}
			},
		},
		{
			name: "valid full request",
			json: `{
				"type": "matrix_2x2",
				"title": "BCG Matrix",
				"subtitle": "Portfolio Analysis",
				"data": {"items": [{"name": "Product A", "x": 0.8, "y": 0.6}]},
				"output": {"format": "png", "width": 1200, "height": 900, "scale": 2.0},
				"style": {"palette": "corporate", "font_family": "Helvetica", "show_legend": true}
			}`,
			wantErr: false,
			checkReq: func(t *testing.T, req *RequestEnvelope) {
				if req.Title != "BCG Matrix" {
					t.Errorf("Title = %q, want BCG Matrix", req.Title)
				}
				if req.Output.Format != "png" {
					t.Errorf("Output.Format = %q, want png", req.Output.Format)
				}
				if req.Output.Width != 1200 {
					t.Errorf("Output.Width = %d, want 1200", req.Output.Width)
				}
			},
		},
		{
			name:    "invalid JSON syntax",
			json:    `{invalid`,
			wantErr: true,
			errCode: ErrCodeParseFailed,
		},
		{
			name:    "missing type",
			json:    `{"data": {"a": 1}}`,
			wantErr: true,
			errCode: ErrCodeRequired,
		},
		{
			name:    "missing data",
			json:    `{"type": "bar_chart"}`,
			wantErr: true,
			errCode: ErrCodeRequired,
		},
		{
			name:    "invalid output format",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"format": "gif"}}`,
			wantErr: true,
			errCode: ErrCodeInvalidValue,
		},
		{
			name:    "type mismatch",
			json:    `{"type": 123, "data": {"a": 1}}`,
			wantErr: true,
			errCode: ErrCodeInvalidType,
		},
	}

	dec := NewDecoder(DefaultDecodeOptions())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := dec.Decode([]byte(tt.json))

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCode != "" {
				verrs := GetValidationErrors(err)
				if len(verrs) == 0 {
					t.Errorf("expected ValidationError with code %s", tt.errCode)
					return
				}
				found := false
				for _, ve := range verrs {
					if ve.Code == tt.errCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error code %s, got %v", tt.errCode, verrs)
				}
			}

			if !tt.wantErr && tt.checkReq != nil {
				tt.checkReq(t, req)
			}
		})
	}
}

func TestDecoder_DecodeYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		checkReq func(*testing.T, *RequestEnvelope)
	}{
		{
			name: "valid minimal request",
			yaml: `
type: bar_chart
data:
  a: 1
`,
			wantErr: false,
			checkReq: func(t *testing.T, req *RequestEnvelope) {
				if req.Type != "bar_chart" {
					t.Errorf("Type = %q, want bar_chart", req.Type)
				}
			},
		},
		{
			name: "valid full request",
			yaml: `
type: matrix_2x2
title: BCG Matrix
subtitle: Portfolio Analysis
data:
  items:
    - name: Product A
      x: 0.8
      y: 0.6
output:
  format: png
  width: 1200
  height: 900
style:
  palette: corporate
  show_legend: true
`,
			wantErr: false,
			checkReq: func(t *testing.T, req *RequestEnvelope) {
				if req.Title != "BCG Matrix" {
					t.Errorf("Title = %q, want BCG Matrix", req.Title)
				}
				if req.Output.Width != 1200 {
					t.Errorf("Output.Width = %d, want 1200", req.Output.Width)
				}
			},
		},
		{
			name: "missing required field",
			yaml: `
data:
  a: 1
`,
			wantErr: true,
		},
	}

	opts := DefaultDecodeOptions()
	opts.Format = "yaml"
	dec := NewDecoder(opts)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := dec.Decode([]byte(tt.yaml))

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkReq != nil {
				tt.checkReq(t, req)
			}
		})
	}
}

func TestDecoder_StrictMode(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		strict  bool
		wantErr bool
		errCode string
	}{
		{
			name:    "unknown field in strict mode",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "unknown_field": true}`,
			strict:  true,
			wantErr: true,
			errCode: ErrCodeUnknownField,
		},
		{
			name:    "unknown field in lenient mode",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "unknown_field": true}`,
			strict:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultDecodeOptions()
			opts.Strict = tt.strict
			dec := NewDecoder(opts)

			_, err := dec.Decode([]byte(tt.json))

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCode != "" {
				verrs := GetValidationErrors(err)
				found := false
				for _, ve := range verrs {
					if ve.Code == tt.errCode {
						found = true
						break
					}
				}
				if !found && len(verrs) > 0 {
					t.Errorf("expected error code %s, got %v", tt.errCode, verrs)
				}
			}
		})
	}
}

func TestDecoder_FormatDetection(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantFormat string
	}{
		{
			name:       "JSON object",
			data:       `{"type": "bar"}`,
			wantFormat: "json",
		},
		{
			name:       "JSON array",
			data:       `[1, 2, 3]`,
			wantFormat: "json",
		},
		{
			name:       "JSON with whitespace",
			data:       `   {"type": "bar"}`,
			wantFormat: "json",
		},
		{
			name:       "YAML",
			data:       "type: bar\ndata:\n  a: 1",
			wantFormat: "yaml",
		},
		{
			name:       "empty input defaults to JSON",
			data:       "",
			wantFormat: "json",
		},
		{
			name:       "whitespace only defaults to JSON",
			data:       "   ",
			wantFormat: "json",
		},
	}

	dec := NewDecoder(DefaultDecodeOptions())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dec.detectFormat([]byte(tt.data))
			if got != tt.wantFormat {
				t.Errorf("detectFormat() = %q, want %q", got, tt.wantFormat)
			}
		})
	}
}

func TestDecoder_Defaults(t *testing.T) {
	json := `{"type": "bar_chart", "data": {"a": 1}}`

	// With defaults enabled
	opts := DefaultDecodeOptions()
	opts.Defaults = true
	dec := NewDecoder(opts)

	req, err := dec.Decode([]byte(json))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if req.Output.Format != "svg" {
		t.Errorf("Output.Format = %q, want svg", req.Output.Format)
	}
	if req.Output.Width != 800 {
		t.Errorf("Output.Width = %d, want 800", req.Output.Width)
	}
	if req.Output.Height != 600 {
		t.Errorf("Output.Height = %d, want 600", req.Output.Height)
	}
	if req.Output.Scale != 2.0 {
		t.Errorf("Output.Scale = %f, want 2.0", req.Output.Scale)
	}
	if req.Style.FontFamily != "Arial" {
		t.Errorf("Style.FontFamily = %q, want Arial", req.Style.FontFamily)
	}
	if req.Style.Background != "transparent" {
		t.Errorf("Style.Background = %q, want transparent", req.Style.Background)
	}

	// Without defaults
	opts.Defaults = false
	dec = NewDecoder(opts)

	req, err = dec.Decode([]byte(json))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if req.Output.Format != "" {
		t.Errorf("Output.Format = %q, want empty", req.Output.Format)
	}
	if req.Style.FontFamily != "" {
		t.Errorf("Style.FontFamily = %q, want empty", req.Style.FontFamily)
	}
}

func TestDecoder_DecodeReader(t *testing.T) {
	json := `{"type": "bar_chart", "data": {"a": 1}}`

	dec := NewDecoder(DefaultDecodeOptions())
	req, err := dec.DecodeReader(bytes.NewReader([]byte(json)))

	if err != nil {
		t.Fatalf("DecodeReader() error = %v", err)
	}
	if req.Type != "bar_chart" {
		t.Errorf("Type = %q, want bar_chart", req.Type)
	}
}

func TestDecodeStrict(t *testing.T) {
	// Valid request
	json := `{"type": "bar_chart", "data": {"a": 1}}`
	req, err := DecodeStrict([]byte(json))
	if err != nil {
		t.Fatalf("DecodeStrict() error = %v", err)
	}
	if req.Type != "bar_chart" {
		t.Errorf("Type = %q, want bar_chart", req.Type)
	}

	// Unknown field should fail
	json = `{"type": "bar_chart", "data": {"a": 1}, "extra": true}`
	_, err = DecodeStrict([]byte(json))
	if err == nil {
		t.Error("DecodeStrict() should fail with unknown field")
	}
}

func TestDecodeJSON(t *testing.T) {
	json := `{"type": "bar_chart", "data": {"a": 1}}`
	req, err := DecodeJSON([]byte(json))
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if req.Type != "bar_chart" {
		t.Errorf("Type = %q, want bar_chart", req.Type)
	}
}

func TestDecodeYAML(t *testing.T) {
	yaml := `
type: bar_chart
data:
  a: 1
`
	req, err := DecodeYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("DecodeYAML() error = %v", err)
	}
	if req.Type != "bar_chart" {
		t.Errorf("Type = %q, want bar_chart", req.Type)
	}
}

func TestIsValidationError(t *testing.T) {
	ve := &ValidationError{Code: ErrCodeRequired, Message: "test"}
	ves := &ValidationErrors{Errors: []ValidationError{*ve}}

	if !IsValidationError(ve) {
		t.Error("IsValidationError() should return true for ValidationError")
	}
	if !IsValidationError(ves) {
		t.Error("IsValidationError() should return true for ValidationErrors")
	}

	otherErr := strings.NewReader("not a validation error")
	_ = otherErr // We just need a different error type
}

func TestGetValidationErrors(t *testing.T) {
	// Single error
	ve := &ValidationError{Code: ErrCodeRequired, Message: "test"}
	errs := GetValidationErrors(ve)
	if len(errs) != 1 {
		t.Errorf("GetValidationErrors() returned %d errors, want 1", len(errs))
	}

	// Multiple errors
	ves := &ValidationErrors{Errors: []ValidationError{
		{Code: ErrCodeRequired, Message: "test1"},
		{Code: ErrCodeInvalidType, Message: "test2"},
	}}
	errs = GetValidationErrors(ves)
	if len(errs) != 2 {
		t.Errorf("GetValidationErrors() returned %d errors, want 2", len(errs))
	}

	// Non-validation error
	errs = GetValidationErrors(nil)
	if errs != nil {
		t.Errorf("GetValidationErrors(nil) returned %v, want nil", errs)
	}
}

func TestValidationConstraints(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		errCode string
	}{
		{
			name:    "negative width",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"width": -100}}`,
			wantErr: true,
			errCode: ErrCodeConstraint,
		},
		{
			name:    "negative height",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"height": -100}}`,
			wantErr: true,
			errCode: ErrCodeConstraint,
		},
		{
			name:    "negative scale",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": -1.0}}`,
			wantErr: true,
			errCode: ErrCodeConstraint,
		},
		{
			name:    "scale below minimum",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": 0.3}}`,
			wantErr: true,
			errCode: ErrCodeConstraint,
		},
		{
			name:    "scale above maximum",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": 15.0}}`,
			wantErr: true,
			errCode: ErrCodeConstraint,
		},
		{
			name:    "scale at minimum is valid",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": 0.5}}`,
			wantErr: false,
		},
		{
			name:    "scale at maximum is valid",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": 10.0}}`,
			wantErr: false,
		},
		{
			name:    "zero scale is valid (uses default)",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"scale": 0}}`,
			wantErr: false,
		},
		{
			name:    "zero dimensions are valid",
			json:    `{"type": "bar_chart", "data": {"a": 1}, "output": {"width": 0, "height": 0}}`,
			wantErr: false,
		},
	}

	dec := NewDecoder(DefaultDecodeOptions())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dec.Decode([]byte(tt.json))

			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCode != "" {
				verrs := GetValidationErrors(err)
				found := false
				for _, ve := range verrs {
					if ve.Code == tt.errCode {
						found = true
						break
					}
				}
				if !found && len(verrs) > 0 {
					t.Errorf("expected error code %s, got %v", tt.errCode, verrs)
				}
			}
		})
	}
}

func TestYAMLStrictMode(t *testing.T) {
	// Unknown field in YAML with strict mode
	yaml := `
type: bar_chart
data:
  a: 1
unknown_field: true
`
	opts := DefaultDecodeOptions()
	opts.Format = "yaml"
	opts.Strict = true
	dec := NewDecoder(opts)

	_, err := dec.Decode([]byte(yaml))
	if err == nil {
		t.Error("expected error for unknown field in strict YAML mode")
	}
}

func TestYAMLSafeParsingLimits(t *testing.T) {
	t.Run("rejects oversized YAML", func(t *testing.T) {
		// Create YAML larger than safeyaml.DefaultMaxSize (64KB)
		// We'll use a large string value to exceed the limit
		largeValue := strings.Repeat("x", 70*1024) // 70KB
		yaml := "type: bar_chart\ndata:\n  a: \"" + largeValue + "\""

		opts := DefaultDecodeOptions()
		opts.Format = "yaml"
		dec := NewDecoder(opts)

		_, err := dec.Decode([]byte(yaml))
		if err == nil {
			t.Error("expected error for oversized YAML")
		}
		if !strings.Contains(err.Error(), "exceeds") && !strings.Contains(err.Error(), "size") {
			t.Errorf("expected size limit error, got: %v", err)
		}
	})

	t.Run("rejects deeply nested YAML", func(t *testing.T) {
		// Create deeply nested YAML structure exceeding DefaultMaxDepth (20)
		// We need about 25 levels of nesting
		var sb strings.Builder
		sb.WriteString("type: bar_chart\ndata:\n")
		for i := 0; i < 25; i++ {
			for j := 0; j < i+2; j++ {
				sb.WriteString("  ")
			}
			sb.WriteString("level")
			sb.WriteString(strings.Repeat("_", i))
			sb.WriteString(":\n")
		}
		// Add final value
		for j := 0; j < 27; j++ {
			sb.WriteString("  ")
		}
		sb.WriteString("value: 1")

		opts := DefaultDecodeOptions()
		opts.Format = "yaml"
		dec := NewDecoder(opts)

		_, err := dec.Decode([]byte(sb.String()))
		if err == nil {
			t.Error("expected error for deeply nested YAML")
		}
		if !strings.Contains(err.Error(), "depth") {
			t.Errorf("expected depth limit error, got: %v", err)
		}
	})

	t.Run("accepts valid YAML within limits", func(t *testing.T) {
		yaml := `
type: bar_chart
data:
  a: 1
  b: 2
`
		opts := DefaultDecodeOptions()
		opts.Format = "yaml"
		dec := NewDecoder(opts)

		_, err := dec.Decode([]byte(yaml))
		if err != nil {
			t.Errorf("unexpected error for valid YAML: %v", err)
		}
	})
}

func TestDefaultBackgroundByFormat(t *testing.T) {
	// PNG format should default to white background
	json := `{"type": "bar_chart", "data": {"a": 1}, "output": {"format": "png"}}`

	opts := DefaultDecodeOptions()
	opts.Defaults = true
	dec := NewDecoder(opts)

	req, err := dec.Decode([]byte(json))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if req.Style.Background != "white" {
		t.Errorf("Style.Background = %q, want 'white' for PNG format", req.Style.Background)
	}

	// SVG format should default to transparent
	json = `{"type": "bar_chart", "data": {"a": 1}, "output": {"format": "svg"}}`
	req, err = dec.Decode([]byte(json))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if req.Style.Background != "transparent" {
		t.Errorf("Style.Background = %q, want 'transparent' for SVG format", req.Style.Background)
	}
}
