// Package diagnostics provides a transport-neutral diagnostic type for
// machine-readable, caller-fixable issues. It unifies the various structured
// error shapes in the codebase (patterns.ValidationError, generator warnings,
// media failures) into a single envelope that MCP, HTTP, and CLI surfaces can
// consume without coupling to transport-specific details.
package diagnostics

import (
	"errors"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Severity indicates how important a diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Diagnostic is a single machine-readable issue that a caller can act on.
// It carries enough context for an agent or human to understand what went
// wrong and how to fix it, without embedding transport-specific fields like
// HTTP status codes or MCP IsError flags.
type Diagnostic struct {
	// Code is a machine-readable identifier, e.g. "required", "fit_overflow",
	// "INVALID_REQUEST". Codes are stable strings suitable for programmatic
	// matching.
	Code string `json:"code"`

	// Message is a human-readable description of the issue.
	Message string `json:"message"`

	// Path identifies where in the input the issue occurs, typically a JSON
	// path like "slides[0].content.body" or a field name. Empty when the
	// issue is global.
	Path string `json:"path,omitempty"`

	// Severity indicates importance: error, warning, or info.
	Severity Severity `json:"severity"`

	// Fix is an optional structured remediation suggestion with a
	// machine-readable kind and parameters.
	Fix *Fix `json:"fix,omitempty"`

	// Details carries additional context that doesn't fit the other fields.
	// Preserves raw cause text, overflow ratios, etc.
	Details map[string]any `json:"details,omitempty"`
}

// Fix is a structured remediation suggestion.
type Fix struct {
	Kind   string         `json:"kind"`            // e.g. "split_at_row", "shrink_text", "provide_value"
	Params map[string]any `json:"params,omitempty"` // kind-specific parameters
}

// FromValidationError converts a patterns.ValidationError into a Diagnostic.
func FromValidationError(ve *patterns.ValidationError) Diagnostic {
	d := Diagnostic{
		Code:     ve.Code,
		Message:  ve.Message,
		Path:     ve.Path,
		Severity: SeverityError,
	}
	if ve.Fix != nil {
		d.Fix = &Fix{
			Kind:   ve.Fix.Kind,
			Params: ve.Fix.Params,
		}
	}
	if ve.Pattern != "" {
		d.Details = map[string]any{"pattern": ve.Pattern}
	}
	return d
}

// FromValidationErrors converts a slice of patterns.ValidationError pointers
// into a slice of Diagnostics.
func FromValidationErrors(ves []*patterns.ValidationError) []Diagnostic {
	if len(ves) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(ves))
	for i, ve := range ves {
		out[i] = FromValidationError(ve)
	}
	return out
}

// FromFitFinding converts a patterns.FitFinding into a Diagnostic.
// The action field maps to severity: "refuse" → error, "shrink_or_split" →
// warning, "review"/"info" → info.
func FromFitFinding(f patterns.FitFinding) Diagnostic {
	d := FromValidationError(&f.ValidationError)
	d.Severity = fitActionToSeverity(f.Action)
	if d.Details == nil {
		d.Details = make(map[string]any)
	}
	d.Details["action"] = f.Action
	if f.OverflowRatio != 0 {
		d.Details["overflow_ratio"] = f.OverflowRatio
	}
	if f.Measured != nil {
		d.Details["measured"] = f.Measured
	}
	if f.Allowed != nil {
		d.Details["allowed"] = f.Allowed
	}
	return d
}

// FromFitFindings converts a slice of FitFinding into Diagnostics.
func FromFitFindings(findings []patterns.FitFinding) []Diagnostic {
	if len(findings) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(findings))
	for i, f := range findings {
		out[i] = FromFitFinding(f)
	}
	return out
}

// FromWarning creates a warning-severity Diagnostic from a plain string.
func FromWarning(code, message string) Diagnostic {
	return Diagnostic{
		Code:     code,
		Message:  message,
		Severity: SeverityWarning,
	}
}

// FromJoinedError splits an errors.Join-ed error into individual Diagnostics.
// Each unwrapped error is inspected: *patterns.ValidationError gets structured
// conversion; plain errors become generic diagnostics with the given code.
func FromJoinedError(err error, fallbackCode string) []Diagnostic {
	if err == nil {
		return nil
	}
	// Try to unwrap a joined error.
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		errs := joined.Unwrap()
		out := make([]Diagnostic, 0, len(errs))
		for _, e := range errs {
			out = append(out, fromSingleError(e, fallbackCode))
		}
		return out
	}
	// Single error.
	return []Diagnostic{fromSingleError(err, fallbackCode)}
}

// fromSingleError converts one error into a Diagnostic, using structured
// conversion when the error is a known type.
func fromSingleError(err error, fallbackCode string) Diagnostic {
	var ve *patterns.ValidationError
	if errors.As(err, &ve) {
		return FromValidationError(ve)
	}
	return Diagnostic{
		Code:     fallbackCode,
		Message:  err.Error(),
		Severity: SeverityError,
	}
}

// fitActionToSeverity maps fit-finding action strings to diagnostic severities.
func fitActionToSeverity(action string) Severity {
	switch action {
	case "refuse":
		return SeverityError
	case "shrink_or_split":
		return SeverityWarning
	default: // "review", "info", unknown
		return SeverityInfo
	}
}

// HasErrors returns true if any diagnostic has error severity.
func HasErrors(ds []Diagnostic) bool {
	for i := range ds {
		if ds[i].Severity == SeverityError {
			return true
		}
	}
	return false
}

// FilterBySeverity returns only diagnostics matching the given severity.
func FilterBySeverity(ds []Diagnostic, sev Severity) []Diagnostic {
	var out []Diagnostic
	for i := range ds {
		if ds[i].Severity == sev {
			out = append(out, ds[i])
		}
	}
	return out
}

// Summary returns a short human-readable summary of the diagnostics,
// e.g. "2 errors, 1 warning".
func Summary(ds []Diagnostic) string {
	var errs, warns, infos int
	for i := range ds {
		switch ds[i].Severity {
		case SeverityError:
			errs++
		case SeverityWarning:
			warns++
		case SeverityInfo:
			infos++
		}
	}
	var parts []string
	if errs > 0 {
		parts = append(parts, pluralize(errs, "error"))
	}
	if warns > 0 {
		parts = append(parts, pluralize(warns, "warning"))
	}
	if infos > 0 {
		parts = append(parts, pluralize(infos, "info"))
	}
	if len(parts) == 0 {
		return "no issues"
	}
	return strings.Join(parts, ", ")
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
