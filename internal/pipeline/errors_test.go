package pipeline

import (
	"errors"
	"testing"
)

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ParseError
		want string
	}{
		{
			name: "message with line number",
			err:  ParseError{Message: "unexpected token", Line: 42},
			want: "line 42: unexpected token",
		},
		{
			name: "message without line number",
			err:  ParseError{Message: "missing title"},
			want: "missing title",
		},
		{
			name: "message with line zero",
			err:  ParseError{Message: "bad field", Line: 0},
			want: "bad field",
		},
		{
			name: "message with negative line",
			err:  ParseError{Message: "bad field", Line: -1},
			want: "bad field",
		},
		{
			name: "wrapped error without message",
			err:  ParseError{Err: errors.New("io timeout")},
			want: "parse error: io timeout",
		},
		{
			name: "wrapped nil error without message",
			err:  ParseError{},
			want: "parse error: <nil>",
		},
		{
			name: "message takes precedence over wrapped error",
			err:  ParseError{Message: "custom msg", Err: errors.New("ignored"), Line: 5},
			want: "line 5: custom msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseError_Unwrap(t *testing.T) {
	inner := errors.New("root cause")
	pe := &ParseError{Err: inner, Message: "wrapper"}

	if got := pe.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
}

func TestParseError_Unwrap_Nil(t *testing.T) {
	pe := &ParseError{Message: "no inner"}
	if got := pe.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestParseError_ErrorsIs(t *testing.T) {
	inner := errors.New("root cause")
	pe := &ParseError{Err: inner}

	if !errors.Is(pe, inner) {
		t.Error("errors.Is should find wrapped error")
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "with line number",
			err:  ValidationError{Line: 10, Field: "title", Message: "title is required"},
			want: "line 10: title is required",
		},
		{
			name: "without line number",
			err:  ValidationError{Field: "bullets", Message: "too many bullets"},
			want: "too many bullets",
		},
		{
			name: "line zero",
			err:  ValidationError{Line: 0, Message: "global validation failed"},
			want: "global validation failed",
		},
		{
			name: "negative line",
			err:  ValidationError{Line: -3, Message: "negative line"},
			want: "negative line",
		},
		{
			name: "line one",
			err:  ValidationError{Line: 1, Message: "first line error"},
			want: "line 1: first line error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLayoutError_Error(t *testing.T) {
	tests := []struct {
		name  string
		inner error
		want  string
	}{
		{
			name:  "with inner error",
			inner: errors.New("no matching layout"),
			want:  "layout selection error: no matching layout",
		},
		{
			name:  "with nil inner",
			inner: nil,
			want:  "layout selection error: <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			le := &LayoutError{Err: tt.inner}
			got := le.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLayoutError_Unwrap(t *testing.T) {
	inner := errors.New("no candidates")
	le := &LayoutError{Err: inner}

	if got := le.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
}

func TestLayoutError_Unwrap_Nil(t *testing.T) {
	le := &LayoutError{}
	if got := le.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestLayoutError_ErrorsIs(t *testing.T) {
	inner := errors.New("sentinel")
	le := &LayoutError{Err: inner}
	if !errors.Is(le, inner) {
		t.Error("errors.Is should find wrapped error")
	}
}

func TestGenerationError_Error(t *testing.T) {
	tests := []struct {
		name  string
		inner error
		want  string
	}{
		{
			name:  "with inner error",
			inner: errors.New("template not found"),
			want:  "generation error: template not found",
		},
		{
			name:  "with nil inner",
			inner: nil,
			want:  "generation error: <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ge := &GenerationError{Err: tt.inner}
			got := ge.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerationError_Unwrap(t *testing.T) {
	inner := errors.New("disk full")
	ge := &GenerationError{Err: inner}

	if got := ge.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
}

func TestGenerationError_Unwrap_Nil(t *testing.T) {
	ge := &GenerationError{}
	if got := ge.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestGenerationError_ErrorsIs(t *testing.T) {
	inner := errors.New("sentinel")
	ge := &GenerationError{Err: inner}
	if !errors.Is(ge, inner) {
		t.Error("errors.Is should find wrapped error")
	}
}

// TestErrorTypes_ImplementErrorInterface verifies all error types satisfy the error interface.
func TestErrorTypes_ImplementErrorInterface(t *testing.T) {
	var _ error = &ParseError{}
	var _ error = &ValidationError{}
	var _ error = &LayoutError{}
	var _ error = &GenerationError{}
}
