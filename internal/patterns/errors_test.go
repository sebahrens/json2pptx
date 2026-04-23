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
	if ve.Fix == "" {
		t.Error("Fix is empty")
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
				if ve.Fix == "" {
					t.Error("ValidationError.Fix is empty")
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
