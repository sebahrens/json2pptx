package semantic

// This file maps raw json2pptx findings — validation errors, fit findings, and
// output-validation findings emitted against the generated PresentationInput —
// back to the compact semantic source the author wrote. The SourceMap supplies
// the raw->semantic path correspondence (exact match first, then nearest
// ancestor); this layer wraps a finding with that resolved location and, for the
// common density/overflow failures, a recommended semantic edit so an agent can
// repair the YAML/JSON it authored rather than the generated shape_grid JSON.
//
// It is deliberately decoupled from internal/patterns: callers adapt their
// finding type (patterns.FitFinding, diagnostics.Diagnostic, ...) into the
// transport-neutral RawFinding so this package never imports the renderer.

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// RawFinding is a transport-neutral view of one raw finding to be mapped back to
// a semantic source path. Callers populate it from whatever finding type they
// hold (a fit finding, a validation error, an output-validation finding).
type RawFinding struct {
	// Code is the finding's stable machine-readable code (e.g. "BODY_TOO_LONG").
	Code string
	// Message is the human-readable description.
	Message string
	// Severity classifies the finding.
	Severity diagnostics.Severity
	// Action is the fit-finding remediation action ("refuse", "shrink_or_split",
	// "review", "info"), or "" for non-fit findings.
	Action string
	// RawPath is the JSON pointer into the generated PresentationInput.
	RawPath string
}

// MappedFinding is a RawFinding traced back to the semantic source the author
// wrote. SemanticPath is the resolved authoring path (exact or nearest
// ancestor); RawPath is always retained as fallback evidence so the precise
// generated location is never lost. SlideIndex is the semantic slide index, or
// -1 when the finding is not slide-scoped. Edit carries a recommended semantic
// edit for the common density/overflow failures, when one is known.
type MappedFinding struct {
	Code         string               `json:"code"`
	Message      string               `json:"message"`
	Severity     diagnostics.Severity `json:"severity,omitempty"`
	Action       string               `json:"action,omitempty"`
	SemanticPath string               `json:"semantic_path,omitempty"`
	RawPath      string               `json:"raw_path,omitempty"`
	SlideIndex   int                  `json:"slide_index"`
	Mapped       bool                 `json:"mapped"`
	Edit         *SemanticEdit        `json:"recommended_edit,omitempty"`
}

// SemanticEdit is a recommended edit to the semantic source that should resolve
// a finding. Kind is a stable machine-readable category; Hint is a short
// human-readable instruction the agent (or author) can act on.
type SemanticEdit struct {
	Kind string `json:"kind"`
	Hint string `json:"hint"`
}

// Semantic edit kinds. Stable strings suitable for programmatic matching.
const (
	// EditShortenText recommends shortening a single text field.
	EditShortenText = "shorten_text"
	// EditReduceItems recommends removing or merging list items.
	EditReduceItems = "reduce_items"
	// EditSplitSlide recommends splitting one dense slide into two.
	EditSplitSlide = "split_slide"
	// EditSimplifySide recommends trimming one side of a comparison.
	EditSimplifySide = "simplify_side"
	// EditSplitPhases recommends spreading roadmap phases across slides.
	EditSplitPhases = "split_phases"
)

// MapFinding traces a raw finding back to its semantic source using sm and
// attaches a recommended semantic edit. SemanticPath is set to the exact match
// or nearest registered ancestor; on a full miss it is empty but SlideIndex is
// still recovered from the raw "slides[N]" prefix. RawPath is always retained.
func MapFinding(sm *SourceMap, in RawFinding) MappedFinding {
	semPath, slideIdx, mapped := sm.ResolveSemantic(in.RawPath)
	return MappedFinding{
		Code:         in.Code,
		Message:      in.Message,
		Severity:     in.Severity,
		Action:       in.Action,
		SemanticPath: semPath,
		RawPath:      in.RawPath,
		SlideIndex:   slideIdx,
		Mapped:       mapped,
		Edit:         suggestSemanticEdit(in.Code, in.Action, semPath),
	}
}

// MapFindings maps a batch of raw findings, preserving order.
func MapFindings(sm *SourceMap, in []RawFinding) []MappedFinding {
	if len(in) == 0 {
		return nil
	}
	out := make([]MappedFinding, len(in))
	for i := range in {
		out[i] = MapFinding(sm, in[i])
	}
	return out
}

// suggestSemanticEdit returns a recommended semantic edit for a density/overflow
// finding, keyed by the semantic field the finding lands on. It returns nil when
// the finding is not a length/density failure or the semantic path is unknown
// (a full source-map miss leaves nothing reliable to recommend).
func suggestSemanticEdit(code, action, semanticPath string) *SemanticEdit {
	if semanticPath == "" || !isLengthFinding(code, action) {
		return nil
	}
	switch {
	case pathHasField(semanticPath, "takeaway", "insight"):
		return &SemanticEdit{Kind: EditShortenText, Hint: "Shorten the slide takeaway to a single short line."}
	case pathHasField(semanticPath, "metrics", "kpis", "big", "small", "value", "label", "caption"):
		if isCountFinding(code) {
			return &SemanticEdit{Kind: EditSplitSlide, Hint: "Split this KPI snapshot into two slides with fewer metrics each."}
		}
		return &SemanticEdit{Kind: EditShortenText, Hint: "Shorten the metric label or value so the card fits."}
	case pathHasField(semanticPath, "phases", "roadmap", "milestones"):
		return &SemanticEdit{Kind: EditSplitPhases, Hint: "Split the roadmap into fewer phases per slide."}
	case pathHasField(semanticPath, "left", "right", "before", "after"):
		return &SemanticEdit{Kind: EditSimplifySide, Hint: "Simplify this comparison side; use fewer or shorter points."}
	case pathHasField(semanticPath, "points", "options", "bullets", "items"):
		return &SemanticEdit{Kind: EditReduceItems, Hint: "Remove or merge list items to reduce slide density."}
	case pathHasField(semanticPath, "title", "subtitle", "recommendation"):
		return &SemanticEdit{Kind: EditShortenText, Hint: "Shorten this text so it fits its placeholder."}
	default:
		return nil
	}
}

// isLengthFinding reports whether a finding describes content that is too long
// or too dense for its slot — the failures a semantic edit can address. It keys
// off the finding code and the fit action (refuse / shrink_or_split both mean
// the content does not fit), and also recognizes the raw pattern-validation
// codes the post-compile preflight surfaces (a value over its max length, or a
// list with the wrong item count, both mean the authored content does not fit
// the chosen pattern).
func isLengthFinding(code, action string) bool {
	switch action {
	case "refuse", "shrink_or_split":
		return true
	}
	switch code {
	case "max_length", "count_mismatch", "max_items", "min_items":
		return true
	}
	c := strings.ToUpper(code)
	for _, marker := range []string{"TOO_LONG", "TOO_MANY", "TOO_DENSE", "OVERFLOW"} {
		if strings.Contains(c, marker) {
			return true
		}
	}
	return false
}

// isCountFinding reports whether a finding is about having the wrong number of
// items (as opposed to items being individually too long), which calls for a
// split rather than a shorten. It covers both the fit "TOO_MANY" codes and the
// raw pattern count-validation codes surfaced by the preflight.
func isCountFinding(code string) bool {
	switch code {
	case "count_mismatch", "max_items", "min_items":
		return true
	}
	return strings.Contains(strings.ToUpper(code), "TOO_MANY")
}

// pathHasField reports whether any dotted/indexed segment of a semantic path
// equals one of the given field names. It splits on "." and "[" so that
// "slides[1].metrics[2].label" matches "metrics" or "label".
func pathHasField(path string, fields ...string) bool {
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '.' || r == '[' || r == ']'
	}) {
		for _, f := range fields {
			if seg == f {
				return true
			}
		}
	}
	return false
}
