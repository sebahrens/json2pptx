package patterns

import (
	"errors"
	"fmt"
)

// Error codes for structured validation errors.
const (
	ErrCodeRequired      = "required"
	ErrCodeMaxLength     = "max_length"
	ErrCodeOutOfRange    = "out_of_range"
	ErrCodeCountMismatch = "count_mismatch"
	ErrCodeUnknownKey    = "unknown_key"
	ErrCodeMinItems      = "min_items"
	ErrCodeMaxItems      = "max_items"
	ErrCodeEmptyValue          = "empty_value"
	ErrCodeHexFillNonBrand    = "hex_fill_non_brand"
	ErrCodeUnknownLayoutID    = "unknown_layout_id"
	ErrCodeCalloutUnsupported = "callout_unsupported"
	ErrCodeUnknownEnum        = "unknown_enum"

	// Fit-report error codes.
	ErrCodeFitOverflow       = "fit_overflow"
	ErrCodeDensityExceeded   = "density_exceeded"
	ErrCodeStackedTables     = "stacked_tables"
	ErrCodeDividerTooThin    = "divider_too_thin"
	ErrCodeMixedFillScheme       = "mixed_fill_scheme"
	ErrCodePlaceholderOverflow   = "placeholder_overflow"
	ErrCodeSlideBoundsOverflow   = "slide_bounds_overflow"
	ErrCodeFooterCollision       = "footer_collision"
	ErrCodeTitleWraps            = "title_wraps"
)

// Sentinel errors for matching with errors.Is. Each ValidationError wraps the
// sentinel corresponding to its Code, so callers can write:
//
//	if errors.Is(err, patterns.ErrRequired) { ... }
var (
	ErrRequired      = errors.New("required field missing")
	ErrMaxLength     = errors.New("value exceeds maximum length")
	ErrOutOfRange    = errors.New("value out of range")
	ErrCountMismatch = errors.New("item count mismatch")
	ErrUnknownKey    = errors.New("unknown key")
	ErrMinItems      = errors.New("too few items")
	ErrMaxItems      = errors.New("too many items")
	ErrEmptyValue          = errors.New("empty value")
	ErrHexFillNonBrand    = errors.New("hex fill color is not in brand allowlist")
	ErrUnknownLayoutID        = errors.New("layout_id not found in template")
	ErrCalloutUnsupported     = errors.New("pattern does not support callout")
	ErrUnknownEnum            = errors.New("unknown enum value")

	ErrFitOverflow     = errors.New("text exceeds cell dimensions")
	ErrDensityExceeded = errors.New("table density exceeds TDR ceiling")
	ErrStackedTables   = errors.New("stacked tables with insufficient gap")
	ErrDividerTooThin  = errors.New("divider shape too thin")
	ErrMixedFillScheme     = errors.New("slide mixes hex and semantic fill colors")
	ErrPlaceholderOverflow = errors.New("placeholder text overflows frame")
	ErrSlideBoundsOverflow = errors.New("shape center falls outside slide bounds")
	ErrFooterCollision     = errors.New("shape intrudes into footer reserved area")
	ErrTitleWraps          = errors.New("title text wraps to multiple lines")
)

// codeSentinel maps error code strings to their sentinel errors.
var codeSentinel = map[string]error{
	ErrCodeRequired:      ErrRequired,
	ErrCodeMaxLength:     ErrMaxLength,
	ErrCodeOutOfRange:    ErrOutOfRange,
	ErrCodeCountMismatch: ErrCountMismatch,
	ErrCodeUnknownKey:    ErrUnknownKey,
	ErrCodeMinItems:      ErrMinItems,
	ErrCodeMaxItems:      ErrMaxItems,
	ErrCodeEmptyValue:          ErrEmptyValue,
	ErrCodeHexFillNonBrand:    ErrHexFillNonBrand,
	ErrCodeUnknownLayoutID:        ErrUnknownLayoutID,
	ErrCodeCalloutUnsupported:     ErrCalloutUnsupported,
	ErrCodeUnknownEnum:            ErrUnknownEnum,
	ErrCodeFitOverflow:       ErrFitOverflow,
	ErrCodeDensityExceeded:   ErrDensityExceeded,
	ErrCodeStackedTables:     ErrStackedTables,
	ErrCodeDividerTooThin:    ErrDividerTooThin,
	ErrCodeMixedFillScheme:       ErrMixedFillScheme,
	ErrCodePlaceholderOverflow:   ErrPlaceholderOverflow,
	ErrCodeSlideBoundsOverflow:   ErrSlideBoundsOverflow,
	ErrCodeFooterCollision:       ErrFooterCollision,
	ErrCodeTitleWraps:            ErrTitleWraps,
}

// FixSuggestion is a structured fix suggestion with a machine-readable kind
// and optional parameters. The kind identifies the category of remediation
// (e.g. "split_at_row", "shrink_text"), and params carry the specifics.
type FixSuggestion struct {
	Kind   string         `json:"kind"`             // e.g. "split_at_row", "shrink_text", "provide_value"
	Params map[string]any `json:"params,omitempty"`  // kind-specific parameters
}

// TextFix creates a FixSuggestion with kind "text" wrapping a free-form message.
// Used for existing pattern validation errors that predate structured fix kinds.
func TextFix(msg string) *FixSuggestion {
	return &FixSuggestion{Kind: "text", Params: map[string]any{"message": msg}}
}

// ValidationError is a structured validation error with a JSON path, error
// code, human-readable message, and optional fix suggestion. It implements the
// error interface so it can be used with errors.Join alongside plain errors.
type ValidationError struct {
	Pattern string         `json:"pattern"`       // e.g. "card-grid"
	Path    string         `json:"path"`          // JSON path, e.g. "cells[2].header"
	Code    string         `json:"code"`          // machine-readable code, e.g. "required"
	Message string         `json:"message"`       // human-readable, e.g. "card-grid: cells[2].header is required"
	Fix     *FixSuggestion `json:"fix,omitempty"` // optional structured fix suggestion
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Unwrap returns the sentinel error for this validation error's Code,
// enabling errors.Is matching (e.g. errors.Is(err, patterns.ErrRequired)).
func (e *ValidationError) Unwrap() error {
	return codeSentinel[e.Code]
}

// --- Constructors ---

// errRequired creates a "required" validation error.
func errRequired(pattern, path string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeRequired,
		Message: fmt.Sprintf("%s: %s is required", pattern, path),
		Fix:     TextFix(fmt.Sprintf("provide a non-empty value for %s", path)),
	}
}

// errMaxLength creates a "max_length" validation error.
func errMaxLength(pattern, path string, maxLen, actualLen int) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMaxLength,
		Message: fmt.Sprintf("%s: %s exceeds maxLength %d (%d chars)", pattern, path, maxLen, actualLen),
		Fix:     TextFix(fmt.Sprintf("shorten %s to at most %d characters", path, maxLen)),
	}
}

// errOutOfRange creates an "out_of_range" validation error for integer bounds.
func errOutOfRange(pattern, path string, min, max, actual int) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeOutOfRange,
		Message: fmt.Sprintf("%s: %s must be %d–%d, got %d", pattern, path, min, max, actual),
		Fix:     TextFix(fmt.Sprintf("set %s to a value between %d and %d", path, min, max)),
	}
}

// errUnknownKey creates an "unknown_key" validation error for cell_overrides.
func errUnknownKey(pattern, path, key, allowedList string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeUnknownKey,
		Message: fmt.Sprintf("%s: %s contains unknown key %q; allowed keys per D15: %s", pattern, path, key, allowedList),
		Fix:     TextFix(fmt.Sprintf("remove %q from %s or use one of: %s", key, path, allowedList)),
	}
}

// errEmptyValue creates an "empty_value" validation error.
func errEmptyValue(pattern, path string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeEmptyValue,
		Message: fmt.Sprintf("%s: %s must not be empty", pattern, path),
		Fix:     TextFix(fmt.Sprintf("provide a non-empty value for %s", path)),
	}
}

// errCellOverrideOutOfRange creates an "out_of_range" error for cell_overrides keys.
func errCellOverrideOutOfRange(pattern string, idx, maxIdx int, hint string) *ValidationError {
	path := fmt.Sprintf("cell_overrides[%d]", idx)
	msg := fmt.Sprintf("%s: cell_overrides key %d out of range [0,%d]", pattern, idx, maxIdx)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeOutOfRange,
		Message: msg,
		Fix:     TextFix(fmt.Sprintf("use a cell_overrides key between 0 and %d", maxIdx)),
	}
}

// errCountMismatch creates a "count_mismatch" validation error.
func errCountMismatch(pattern, path string, expected, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain exactly %d items, got %d", pattern, path, expected, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeCountMismatch,
		Message: msg,
		Fix:     TextFix(fmt.Sprintf("provide exactly %d items in %s", expected, path)),
	}
}

// errMinItems creates a "min_items" validation error.
func errMinItems(pattern, path string, minCount, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain at least %d items, got %d", pattern, path, minCount, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMinItems,
		Message: msg,
		Fix:     TextFix(fmt.Sprintf("provide at least %d items in %s", minCount, path)),
	}
}

// errMaxItems creates a "max_items" validation error.
func errMaxItems(pattern, path string, maxCount, actual int, hint string) *ValidationError {
	msg := fmt.Sprintf("%s: %s must contain at most %d items, got %d", pattern, path, maxCount, actual)
	if hint != "" {
		msg += " " + hint
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeMaxItems,
		Message: msg,
		Fix:     TextFix(fmt.Sprintf("reduce %s to at most %d items", path, maxCount)),
	}
}

// ErrCalloutUnsupportedFor creates a "callout_unsupported" validation error
// that names the pattern and suggests patterns that do support callout.
func ErrCalloutUnsupportedFor(pattern string, supportedPatterns []string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    "pattern.callout",
		Code:    ErrCodeCalloutUnsupported,
		Message: fmt.Sprintf("%s: does not support callout", pattern),
		Fix: &FixSuggestion{
			Kind: "remove_field_or_switch_pattern",
			Params: map[string]any{
				"supports_callout_patterns": supportedPatterns,
			},
		},
	}
}

// newValidationError creates a ValidationError with explicit message and fix.
// Use this when the canned constructors don't match the required message format.
func newValidationError(pattern, path, code, message, fix string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    code,
		Message: message,
		Fix:     TextFix(fix),
	}
}
