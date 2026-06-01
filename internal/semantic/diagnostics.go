// Package semantic holds the compact, semantic deck authoring model — a small
// human-friendly vocabulary (DeckSpec / SlideSpec, kind-discriminated) that
// later phases of the semantic compiler validate and compile down into the raw
// internal/deckinput.PresentationInput model consumed by the generator.
//
// This file defines the diagnostic envelope. Parsing and validation surfaces
// emit Diagnostic values with a JSON-ish path so that a malformed authoring
// document yields a precise, machine-readable complaint ("slides[2].kind:
// unknown slide kind") rather than an opaque decode error.
package semantic

import (
	"fmt"
	"strings"
)

// Severity classifies a diagnostic.
type Severity string

const (
	// SeverityError marks a diagnostic that makes the spec unusable.
	SeverityError Severity = "error"
	// SeverityWarning marks a recoverable problem worth surfacing.
	SeverityWarning Severity = "warning"
	// SeverityInfo marks an advisory note.
	SeverityInfo Severity = "info"
)

// Diagnostic codes emitted by the semantic parser. The validation gates added
// in a later phase extend this set; keep codes stable and machine-readable.
const (
	// CodeParseError indicates the document could not be decoded as YAML/JSON.
	CodeParseError = "parse_error"
	// CodeInvalidRoot indicates the document root is not a mapping/object.
	CodeInvalidRoot = "invalid_root"
	// CodeInvalidMeta indicates the meta field is present but not a mapping/object.
	CodeInvalidMeta = "invalid_meta"
	// CodeInvalidSlides indicates the slides field is present but not a list.
	CodeInvalidSlides = "invalid_slides"
	// CodeInvalidSlide indicates a slide entry is not a mapping/object.
	CodeInvalidSlide = "invalid_slide"
	// CodeMissingKind indicates a slide has no kind discriminator.
	CodeMissingKind = "missing_kind"
	// CodeInvalidKindType indicates a slide's kind is present but not a string.
	CodeInvalidKindType = "invalid_kind_type"
	// CodeUnknownKind indicates a slide's kind is not a registered slide kind.
	CodeUnknownKind = "unknown_kind"
	// CodeUnknownField indicates an unrecognized top-level key in the document
	// (a key other than "meta" or "slides", e.g. a stale "deck" object).
	CodeUnknownField = "unknown_field"
	// CodeUnknownArchetype indicates meta.archetype is not a registered archetype.
	CodeUnknownArchetype = "unknown_archetype"
)

// Diagnostic is a structured complaint about a semantic deck document. It
// carries a path into the source document, a machine-readable code, a
// human-readable message, and a severity. It implements the error interface.
type Diagnostic struct {
	// Path is a JSON-ish location, e.g. "slides[2].kind" or "meta.archetype".
	// The empty string refers to the document root.
	Path string `json:"path"`
	// Code is a stable machine-readable identifier (see the Code* constants).
	Code string `json:"code"`
	// Message is a human-readable description.
	Message string `json:"message"`
	// Severity classifies the diagnostic.
	Severity Severity `json:"severity"`
}

// Error implements the error interface.
func (d Diagnostic) Error() string {
	if d.Path == "" {
		return fmt.Sprintf("%s: %s", d.Code, d.Message)
	}
	return fmt.Sprintf("%s: %s: %s", d.Path, d.Code, d.Message)
}

// Diagnostics is an ordered collection of diagnostics.
type Diagnostics []Diagnostic

// add appends an error-severity diagnostic.
func (ds *Diagnostics) add(path, code, message string) {
	*ds = append(*ds, Diagnostic{Path: path, Code: code, Message: message, Severity: SeverityError})
}

// HasErrors reports whether any diagnostic has SeverityError.
func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns only the error-severity diagnostics.
func (ds Diagnostics) Errors() Diagnostics {
	var out Diagnostics
	for _, d := range ds {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// AsError returns a single error joining all error-severity diagnostics, or nil
// when there are none. Warnings and info diagnostics are not included.
func (ds Diagnostics) AsError() error {
	errs := ds.Errors()
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, d := range errs {
		msgs = append(msgs, d.Error())
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
