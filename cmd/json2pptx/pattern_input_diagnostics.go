package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Pattern input diagnostics (go-slide-creator-20jm).
//
// expandPattern used to report every pattern failure as one string, and both
// callers turned that string into one finding: generate as
// INPUT.INVALID_SLIDE("invalid slide specification: slide 1: pattern: pattern
// \"process-flow\": invalid values: json: cannot unmarshal string into Go
// struct field ProcessFlowValues.steps of type patterns.ProcessFlowStep"),
// validate as GRID.PATTERN_ERROR with the path /slides/0/pattern. An agent got
// Go type names, no per-field path, and nothing to act on — and when several
// fields were wrong it got them newline-joined inside a single message.
//
// patternInputError carries the per-field findings instead, so both surfaces
// report one finding per problem, each addressed at /slides/i/pattern/values/…
// with a fix and a next_tool_call.

// patternInputError is a pattern-input failure decomposed into per-field
// findings. Its Unwrap() []error makes it transparent to
// diagnostics.FromJoinedError and to errors.Is on the code sentinels, so a
// caller that only has the plain error interface loses nothing.
type patternInputError struct {
	pattern string
	errs    []*patterns.ValidationError
}

// newPatternInputError returns nil when there is nothing to report, so callers
// can assign it straight to an error variable.
func newPatternInputError(pattern string, errs []*patterns.ValidationError) error {
	if len(errs) == 0 {
		return nil
	}
	return &patternInputError{pattern: pattern, errs: errs}
}

// Error joins the findings' messages, preserving the single-string behaviour any
// path that just prints the error (the CLI, logs) still depends on.
func (e *patternInputError) Error() string {
	msgs := make([]string, len(e.errs))
	for i, ve := range e.errs {
		msgs[i] = ve.Message
	}
	return strings.Join(msgs, "\n")
}

// Unwrap exposes the findings for errors.As / errors.Is and for
// diagnostics.FromJoinedError.
func (e *patternInputError) Unwrap() []error {
	out := make([]error, len(e.errs))
	for i, ve := range e.errs {
		out[i] = ve
	}
	return out
}

// patternValidationFindings returns the per-field findings carried by err, or
// nil when err is not a pattern-input failure.
func patternValidationFindings(err error) []*patterns.ValidationError {
	var pie *patternInputError
	if errors.As(err, &pie) {
		return pie.errs
	}
	// A pattern's own Validate returns errors.Join of *ValidationError; harvest
	// those too so a required-field failure is path-addressed even on the paths
	// that do not route through patternInputError.
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		var out []*patterns.ValidationError
		for _, e := range joined.Unwrap() {
			var ve *patterns.ValidationError
			if errors.As(e, &ve) {
				out = append(out, ve)
				continue
			}
			// One non-structured member means the set is not fully addressable;
			// fall back rather than report a partial picture.
			return nil
		}
		return out
	}
	var ve *patterns.ValidationError
	if errors.As(err, &ve) {
		return []*patterns.ValidationError{ve}
	}
	return nil
}

// patternInputDiagnostics converts a pattern-input failure into one diagnostic
// per field, anchored under basePath (the JSON Pointer of the pattern object,
// e.g. "/slides/0/pattern"). It returns nil when err carries no per-field
// findings, leaving the caller's existing single-finding path in charge.
func patternInputDiagnostics(err error, basePath, slideLabel string) []diagnostics.Diagnostic {
	findings := patternValidationFindings(err)
	if len(findings) == 0 {
		return nil
	}
	out := make([]diagnostics.Diagnostic, 0, len(findings))
	for _, ve := range findings {
		d := diagnostics.FromValidationError(ve)
		d.Path = patternFieldPointer(basePath, ve.Path)
		if slideLabel != "" {
			d.Message = slideLabel + ": " + d.Message
		}
		d.NextToolCall = patternNextToolCall(ve)
		// A fix's path has to address the same place evidence.path does; a
		// pattern-relative "values.members[0].title" is not something an agent
		// holding the whole deck can apply.
		if d.Fix != nil {
			if _, ok := d.Fix.Params["path"]; ok {
				params := make(map[string]any, len(d.Fix.Params))
				for k, v := range d.Fix.Params {
					params[k] = v
				}
				params["path"] = d.Path
				d.Fix = &diagnostics.Fix{Kind: d.Fix.Kind, Params: params}
			}
		}
		out = append(out, d)
	}
	return out
}

// patternNextToolCall names the call that answers the finding: the pattern's own
// schema and example values for a field-level problem, the catalogue when the
// pattern name itself is wrong.
func patternNextToolCall(ve *patterns.ValidationError) *patterns.ToolCallSuggestion {
	if ve.Code == diagnostics.CodeUnknownPattern {
		args := map[string]any{"fields": "compact"}
		if ve.Fix != nil {
			if sug, ok := ve.Fix.Params["did_you_mean"].(string); ok && sug != "" {
				return &patterns.ToolCallSuggestion{Tool: "show_pattern", ArgsTemplate: map[string]any{"name": sug}}
			}
		}
		return &patterns.ToolCallSuggestion{Tool: "list_patterns", ArgsTemplate: args}
	}
	if ve.Pattern == "" {
		return nil
	}
	return &patterns.ToolCallSuggestion{Tool: "show_pattern", ArgsTemplate: map[string]any{"name": ve.Pattern}}
}

// patternFieldPointer turns a pattern-relative dotted path into a deck-absolute
// JSON Pointer: ("/slides/0/pattern", "values.members[0].role") becomes
// "/slides/0/pattern/values/members/0/role". Paths are already section-rooted
// by rootPatternFindingPaths when the findings are harvested.
func patternFieldPointer(basePath, dotted string) string {
	if dotted == "" {
		return basePath
	}
	return slidepath.Join(basePath, dottedPathToPointer(dotted))
}

// patternInputSections are PatternInput's own JSON keys. A finding path that
// starts with one of them is already rooted at the pattern object.
var patternInputSections = map[string]bool{
	"name": true, "values": true, "overrides": true, "cell_overrides": true,
	"callout": true, "bounds": true, "max_height_pct": true,
}

// firstPathSegment returns the leading field name of a dotted path, without any
// index suffix ("values[0].big" → "values").
func firstPathSegment(dotted string) string {
	seg := dotted
	if i := strings.IndexAny(seg, ".["); i >= 0 {
		seg = seg[:i]
	}
	return seg
}

// rootPatternFindingPaths returns copies of ves whose paths are rooted at the
// pattern object. Pattern.Validate reports values-relative paths
// ("members[0].role", "values[0].small"), and only the code that harvests them
// knows that; rooting happens once, here, so every consumer downstream can treat
// a finding path as pattern-relative without special cases.
func rootPatternFindingPaths(ves []*patterns.ValidationError) []*patterns.ValidationError {
	out := make([]*patterns.ValidationError, len(ves))
	for i, ve := range ves {
		copied := *ve
		if copied.Path == "" {
			copied.Path = "values"
		} else if !patternInputSections[firstPathSegment(copied.Path)] {
			copied.Path = "values." + copied.Path
		}
		out[i] = &copied
	}
	return out
}

// prefixPatternFindingPaths re-roots a pattern-input failure under prefix, used
// when the pattern sits inside a shape_grid cell: the findings then address
// /slides/i/shape_grid/rows/R/cells/C/pattern/values/… instead of losing the
// cell coordinates. Non-pattern errors pass through untouched.
func prefixPatternFindingPaths(err error, prefix string) error {
	var pie *patternInputError
	if !errors.As(err, &pie) {
		return err
	}
	moved := make([]*patterns.ValidationError, len(pie.errs))
	for i, ve := range pie.errs {
		copied := *ve
		copied.Path = prefix + copied.Path
		moved[i] = &copied
	}
	return &patternInputError{pattern: pie.pattern, errs: moved}
}

// dottedPathToPointer rewrites "values.members[0].role" as
// "values/members/0/role". Index brackets become their own segments; a
// non-numeric bracket key (cell_overrides[3]) is treated the same way.
func dottedPathToPointer(dotted string) string {
	var segs []string
	for _, part := range strings.Split(dotted, ".") {
		for part != "" {
			open := strings.IndexByte(part, '[')
			if open < 0 {
				segs = append(segs, part)
				break
			}
			if open > 0 {
				segs = append(segs, part[:open])
			}
			close := strings.IndexByte(part[open:], ']')
			if close < 0 {
				segs = append(segs, part[open+1:])
				break
			}
			segs = append(segs, part[open+1:open+close])
			part = part[open+close+1:]
		}
	}
	return strings.Join(segs, "/")
}

// unknownPatternInputError reports a pattern name the registry does not have,
// with the closest registered name when there is one. It is a ValidationError so
// it travels the same per-field path as the rest of the pattern input findings.
// (The CLI's unknownPatternError stays as it is: a terminal reader wants the
// whole catalogue printed, an agent wants one suggestion and a tool call.)
func unknownPatternInputError(reg *patterns.Registry, name string) error {
	ve := &patterns.ValidationError{
		Path: "name",
		Code: diagnostics.CodeUnknownPattern,
	}
	if suggestion, ok := reg.Suggest(name); ok {
		ve.Message = fmt.Sprintf("unknown pattern %q; did you mean %q?", name, suggestion)
		ve.Fix = &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{
			"from": name, "to": suggestion, "did_you_mean": suggestion,
		}}
	} else {
		ve.Message = fmt.Sprintf("unknown pattern %q; call list_patterns for the registered names", name)
		ve.Fix = &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{"from": name}}
	}
	return &patternInputError{pattern: name, errs: []*patterns.ValidationError{ve}}
}

// ---------------------------------------------------------------------------
// Slide attribution
// ---------------------------------------------------------------------------

// slidePatternError records which slide and which field a pattern failure came
// from. The generate path used to encode that in the message
// ("slide 3: pattern: …") and the MCP handler could only pass the whole string
// through as one INVALID_SLIDE finding; carrying the index lets the handler
// address each finding at /slides/3/pattern/values/… instead.
type slidePatternError struct {
	slideIdx int
	// field is the JSON key the failure belongs to ("pattern", "compose",
	// "shape_grid"), used to build the JSON Pointer.
	field string
	// label is how the field reads in a message ("nested pattern" for a pattern
	// inside a shape_grid cell).
	label string
	err   error
}

func newSlidePatternError(slideIdx int, field, label string, err error) error {
	if err == nil {
		return nil
	}
	return &slidePatternError{slideIdx: slideIdx, field: field, label: label, err: err}
}

// Error keeps the historical wording so log lines and CLI output are unchanged.
func (e *slidePatternError) Error() string {
	return fmt.Sprintf("slide %d: %s: %v", e.slideIdx+1, e.label, e.err)
}

func (e *slidePatternError) Unwrap() error { return e.err }

// slidePatternInputDiagnostics converts a slide-attributed pattern failure into
// per-field diagnostics. It returns nil when err is not one, or carries no
// per-field findings, so the caller falls back to its existing single finding.
func slidePatternInputDiagnostics(err error) []diagnostics.Diagnostic {
	var spe *slidePatternError
	if !errors.As(err, &spe) {
		return nil
	}
	return patternInputDiagnostics(spe.err,
		slidepath.SlideField(spe.slideIdx, spe.field),
		fmt.Sprintf("slide %d", spe.slideIdx+1))
}
