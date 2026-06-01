package semantic

// This file implements the structural validation gate for the raw_json2pptx
// escape hatch. The escape hatch carries a verbatim json2pptx slide under the
// "slide" key, bypassing the semantic abstraction. Left unchecked, an object
// that is not a valid raw slide — unknown fields, no slide_type/layout, no
// renderable content — sails through validate (which only sees "slide" is
// non-empty) and compile (which marshals/unmarshals lossily, dropping unknown
// fields), yielding a broken empty slide that only surfaces far downstream.
//
// validateRawEscapeHatch closes that gap: it decodes the payload strictly as a
// deckinput.SlideInput, rejecting unknown raw fields rather than silently
// dropping them, and requires a renderable slide shape (a slide_type/layout_id
// plus content/shape_grid/pattern/compose, with the deliberately content-free
// "blank" canvas exempted). Findings are hard structural errors so they block
// at validate and compile time, before a degraded slide is emitted.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// validateRawEscapeHatch enforces that a raw_json2pptx slide's "slide" payload
// is a valid, renderable raw json2pptx slide. The missing-"slide" case is left
// to the required-field check in validateSlide; this function only runs further
// checks when a payload is present.
func validateRawEscapeHatch(path string, body map[string]any, s *semDiags) {
	raw, ok := body["slide"]
	if !ok || raw == nil {
		return // missing/empty "slide" already reported by the required-field check
	}
	slidePath := path + ".slide"

	obj, ok := raw.(map[string]any)
	if !ok {
		s.hard(slidePath, diagnostics.CodeSemanticRequired,
			fmt.Sprintf("raw_json2pptx \"slide\" payload must be a json2pptx slide object, got %s", jsonKindOf(raw)))
		return
	}

	// Re-encode the generic map and decode it strictly into the canonical raw
	// slide shape. DisallowUnknownFields turns a field the escape hatch would
	// otherwise discard (e.g. a typo'd "slidetype") into an actionable finding
	// instead of a silently dropped value that compiles to nothing.
	encoded, err := json.Marshal(obj)
	if err != nil {
		s.hard(slidePath, diagnostics.CodeSemanticRequired,
			fmt.Sprintf("raw_json2pptx slide payload could not be encoded: %v", err))
		return
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.DisallowUnknownFields()
	var slide deckinput.SlideInput
	if err := dec.Decode(&slide); err != nil {
		s.hard(slidePath, diagnostics.CodeSemanticUnknownField,
			fmt.Sprintf("raw_json2pptx slide payload is not a valid json2pptx slide: %s", cleanRawDecodeError(err)))
		return
	}

	// A renderable slide must declare what it is. Both slide_type and layout_id
	// are optional hints individually, but a raw slide with neither has no anchor
	// for layout selection.
	hasKind := strings.TrimSpace(slide.SlideType) != "" || strings.TrimSpace(slide.LayoutID) != ""
	if !hasKind {
		s.hard(slidePath, diagnostics.CodeSemanticRequired,
			"raw_json2pptx slide must set a \"slide_type\" or \"layout_id\"")
	}

	// A content slide with no body compiles to a blank slide behind a green gate.
	// The "blank" slide_type is the one deliberate content-free canvas, so it is
	// exempt from the renderable-content requirement.
	if !strings.EqualFold(strings.TrimSpace(slide.SlideType), "blank") && !rawSlideHasRenderableContent(&slide) {
		s.hard(slidePath, diagnostics.CodeSemanticRequired,
			"raw_json2pptx slide has no renderable content; provide content, shape_grid, pattern, or compose")
	}
}

// rawSlideHasRenderableContent reports whether a decoded raw slide carries at
// least one element that renders to visible output: a non-empty content list, a
// shape grid, a pattern, a compose envelope, or a background image.
func rawSlideHasRenderableContent(slide *deckinput.SlideInput) bool {
	switch {
	case len(slide.Content) > 0:
		return true
	case slide.ShapeGrid != nil:
		return true
	case slide.Pattern != nil:
		return true
	case slide.Compose != nil:
		return true
	case slide.Background != nil:
		return true
	default:
		return false
	}
}

// jsonKindOf names the JSON kind of a decoded value for diagnostic messages.
func jsonKindOf(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// cleanRawDecodeError trims the redundant "json: " prefix the standard decoder
// prepends to unknown-field and type errors so the finding message reads
// cleanly.
func cleanRawDecodeError(err error) string {
	return strings.TrimPrefix(err.Error(), "json: ")
}
