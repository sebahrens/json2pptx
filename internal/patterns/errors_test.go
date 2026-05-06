package patterns

import (
	"errors"
	"strings"
	"testing"
)

func TestValidationErrorImplementsError(t *testing.T) {
	ve := errRequired("card-grid", "cells[0].header")
	if ve.Error() == "" {
		t.Fatal("Error() returned empty string")
	}
	if ve.Code != ErrCodeRequired {
		t.Errorf("Code = %q, want %q", ve.Code, ErrCodeRequired)
	}
	if ve.Path != "cells[0].header" {
		t.Errorf("Path = %q, want %q", ve.Path, "cells[0].header")
	}
	if ve.Fix == nil || ve.Fix.Kind == "" {
		t.Error("Fix is nil or has empty Kind")
	}
}

func TestValidationErrorUnwrap(t *testing.T) {
	ve := errMaxLength("card-grid", "cells[0].body", 300, 450)
	joined := errors.Join(ve, errRequired("card-grid", "cells[1].header"))

	// errors.As should find ValidationError in joined errors.
	var target *ValidationError
	if !errors.As(joined, &target) {
		t.Fatal("errors.As failed to find *ValidationError in joined error")
	}
	if target.Code != ErrCodeMaxLength {
		t.Errorf("Code = %q, want %q", target.Code, ErrCodeMaxLength)
	}
}

func TestValidationErrorSentinelMatching(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		sentinel error
	}{
		{"required", errRequired("p", "f"), ErrRequired},
		{"max_length", errMaxLength("p", "f", 10, 20), ErrMaxLength},
		{"out_of_range", errOutOfRange("p", "f", 1, 5, 10), ErrOutOfRange},
		{"count_mismatch", errCountMismatch("p", "f", 4, 3, ""), ErrCountMismatch},
		{"unknown_key", errUnknownKey("p", "f", "bad", "a, b"), ErrUnknownKey},
		{"min_items", errMinItems("p", "f", 2, 1, ""), ErrMinItems},
		{"max_items", errMaxItems("p", "f", 5, 10, ""), ErrMaxItems},
		{"empty_value", errEmptyValue("p", "f"), ErrEmptyValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.sentinel) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.sentinel)
			}
			// Original error text must be preserved.
			if tt.err.Error() == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

func TestValidationErrorSentinelInJoinedErrors(t *testing.T) {
	joined := errors.Join(
		errRequired("card-grid", "cells[0].header"),
		errMaxLength("card-grid", "cells[0].body", 300, 450),
	)

	if !errors.Is(joined, ErrRequired) {
		t.Error("errors.Is(joined, ErrRequired) = false, want true")
	}
	if !errors.Is(joined, ErrMaxLength) {
		t.Error("errors.Is(joined, ErrMaxLength) = false, want true")
	}
	if errors.Is(joined, ErrUnknownKey) {
		t.Error("errors.Is(joined, ErrUnknownKey) = true, want false")
	}
}

func TestValidationErrorUnwrapUnknownCode(t *testing.T) {
	ve := newValidationError("p", "f", "custom_code", "msg", TextFix("fix"))
	if ve.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil for unknown code", ve.Unwrap())
	}
}

func TestValidateProducesStructuredErrors(t *testing.T) {
	p, ok := Default().Get("card-grid")
	if !ok {
		t.Fatal("card-grid pattern not found in registry")
	}

	vals := &CardGridValues{
		Columns: 2,
		Rows:    2,
		Cells: []CardGridCell{
			{Header: "", Body: "b1"},
			{Header: "h2", Body: "b2"},
			{Header: "h3", Body: "b3"},
			{Header: "h4", Body: "b4"},
		},
	}

	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	// Unwrap joined errors and check that at least one is a *ValidationError.
	type unwrapper interface {
		Unwrap() []error
	}
	var found bool
	if uw, ok := err.(unwrapper); ok {
		for _, e := range uw.Unwrap() {
			var ve *ValidationError
			if errors.As(e, &ve) {
				found = true
				if ve.Code == "" {
					t.Error("ValidationError.Code is empty")
				}
				if ve.Path == "" {
					t.Error("ValidationError.Path is empty")
				}
				if ve.Fix == nil || ve.Fix.Kind == "" {
					t.Error("ValidationError.Fix is nil or has empty Kind")
				}
				if !strings.Contains(ve.Message, "card-grid") {
					t.Errorf("Message %q does not contain pattern name", ve.Message)
				}
			}
		}
	} else {
		var ve *ValidationError
		if errors.As(err, &ve) {
			found = true
		}
	}
	if !found {
		t.Error("no *ValidationError found in validation output")
	}
}

func TestStructuredFixKinds(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		wantKind string
		wantKey  string // a key expected in params
	}{
		{"required_provides_value", errRequired("p", "title"), "provide_value", "path"},
		{"max_length_reduces_text", errMaxLength("p", "body", 100, 200), "reduce_text", "max_length"},
		{"out_of_range_replaces_value", errOutOfRange("p", "cols", 1, 5, 10), "replace_value", "min"},
		{"unknown_key_removes", errUnknownKey("p", "overrides", "bad", "a, b"), "remove_key", "key"},
		{"empty_value_provides", errEmptyValue("p", "title"), "provide_value", "path"},
		{"count_mismatch_resizes", errCountMismatch("p", "cells", 4, 3, ""), "resize_list", "count"},
		{"min_items_adds", errMinItems("p", "items", 2, 1, ""), "add_items", "min_items"},
		{"max_items_reduces", errMaxItems("p", "items", 5, 10, ""), "reduce_items", "max_items"},
		{"cell_override_replaces", errCellOverrideOutOfRange("p", 99, 3, ""), "replace_value", "min"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Fix == nil {
				t.Fatal("Fix is nil")
			}
			if tt.err.Fix.Kind != tt.wantKind {
				t.Errorf("Fix.Kind = %q, want %q", tt.err.Fix.Kind, tt.wantKind)
			}
			if tt.err.Fix.Kind == "text" {
				t.Error("Fix.Kind is still 'text' — must use a structured kind")
			}
			if _, ok := tt.err.Fix.Params[tt.wantKey]; !ok {
				t.Errorf("Fix.Params missing key %q; got %v", tt.wantKey, tt.err.Fix.Params)
			}
		})
	}
}

func TestRepairFixKindsIncludesNewKinds(t *testing.T) {
	requiredKinds := []string{
		"provide_value", "replace_value", "reduce_items",
		"add_items", "resize_list", "remove_key", "remove_field",
	}
	for _, kind := range requiredKinds {
		if !repairFixKinds[kind] {
			t.Errorf("repairFixKinds missing %q", kind)
		}
	}
}

func TestRepairToolCallForNewKinds(t *testing.T) {
	tests := []struct {
		kind string
	}{
		{"provide_value"},
		{"replace_value"},
		{"reduce_items"},
		{"add_items"},
		{"resize_list"},
		{"remove_key"},
		{"remove_field"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			fix := &FixSuggestion{Kind: tt.kind, Params: map[string]any{"path": "test"}}
			tc := RepairToolCall(0, fix)
			if tc == nil {
				t.Fatalf("RepairToolCall returned nil for kind %q", tt.kind)
			}
			if tc.Tool != "repair_slide" {
				t.Errorf("Tool = %q, want repair_slide", tc.Tool)
			}
		})
	}
}

func TestValidateCellOverrideKeysStructuredErrors(t *testing.T) {
	overrides := map[int]any{
		99: map[string]any{"accent_bar": true},
	}

	err := validateCellOverrideKeys("test-pattern", overrides, 3, "")
	if err == nil {
		t.Fatal("expected error for out-of-range key")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("expected *ValidationError, got plain error")
	}
	if ve.Code != ErrCodeOutOfRange {
		t.Errorf("Code = %q, want %q", ve.Code, ErrCodeOutOfRange)
	}
	if !strings.Contains(ve.Path, "cell_overrides") {
		t.Errorf("Path %q does not contain cell_overrides", ve.Path)
	}
}
