package core

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError represents a structured validation failure.
type ValidationError struct {
	// Field is the JSON/YAML path to the invalid field (e.g., "data.values[0]").
	Field string `json:"field,omitempty"`

	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`

	// Value is the invalid value (if safe to include).
	Value any `json:"value,omitempty"`
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (code: %s)", e.Field, e.Message, e.Code)
	}
	return fmt.Sprintf("%s (code: %s)", e.Message, e.Code)
}

// Validation error codes.
const (
	ErrCodeRequired       = "REQUIRED"
	ErrCodeInvalidType    = "INVALID_TYPE"
	ErrCodeInvalidFormat  = "INVALID_FORMAT"
	ErrCodeInvalidValue   = "INVALID_VALUE"
	ErrCodeUnknownField   = "UNKNOWN_FIELD"
	ErrCodeParseFailed    = "PARSE_FAILED"
	ErrCodeConstraint     = "CONSTRAINT"
	ErrCodeUnknownDiagram = "UNKNOWN_DIAGRAM"
)

// SVG scale limits for request validation.
// Matches limits in internal/api/validation.go for consistency.
const (
	MinSVGScale = 0.5  // Minimum allowed SVG scale factor
	MaxSVGScale = 10.0 // Maximum allowed SVG scale factor (memory/performance bound)
)

// ValidationErrors is a collection of validation errors.
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error implements the error interface.
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("%d validation errors: %s", len(e.Errors), strings.Join(msgs, "; "))
}

// Add appends a validation error.
func (e *ValidationErrors) Add(err ValidationError) {
	e.Errors = append(e.Errors, err)
}

// HasErrors returns true if there are any errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// AsError returns nil if no errors, otherwise returns self.
func (e *ValidationErrors) AsError() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	var ve *ValidationError
	var ves *ValidationErrors
	return errors.As(err, &ve) || errors.As(err, &ves)
}

// GetValidationErrors extracts validation errors from an error.
func GetValidationErrors(err error) []ValidationError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return []ValidationError{*ve}
	}

	var ves *ValidationErrors
	if errors.As(err, &ves) {
		return ves.Errors
	}

	return nil
}
