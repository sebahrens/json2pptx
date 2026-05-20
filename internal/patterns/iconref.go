package patterns

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

// IconRef is a polymorphic icon reference used by pattern slots that accept
// either a string shorthand (treated as a bundled icon name, URL, inline SVG,
// data URI, or local file path — classified at unmarshal time) or a full
// IconInput object with explicit name/path/url/svg_data/fill/alt/position
// fields.
//
// Both JSON forms are accepted:
//
//	"rocket"
//	{"name": "rocket"}
//	{"path": "logo.svg", "fill": "#FF0000"}
//	{"url": "https://example.com/icon.svg"}
//	{"svg_data": "<svg>...</svg>"}
//
// Go callers (tests, exemplars) use a struct literal:
//
//	IconRef{Name: "rocket"}
//	IconRef{Path: "logo.svg", Fill: "#FF0000"}
//
// Pattern Expand code calls Resolve(defaultFill, defaultPosition) to produce
// the *jsonschema.IconInput suitable for embedding in a ShapeSpecInput.
type IconRef jsonschema.IconInput

// UnmarshalJSON accepts either a string shorthand or a full IconInput object.
// String shorthand is classified via svggen.ClassifyIcon and routed to the
// matching field so a URL string lands in URL, inline SVG in SVGData, a file
// path in Path, and everything else (bundled names, unknown strings) in Name.
// Unknown-string Name values produce a friendly "not a bundled icon" error at
// Validate time rather than a shape-mismatch error here.
func (r *IconRef) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*r = parseIconString(s)
		return nil
	}
	var raw jsonschema.IconInput
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("icon must be a string shorthand or {name|path|url|svg_data} object: %w", err)
	}
	*r = IconRef(raw)
	return nil
}

// MarshalJSON emits the most compact representation. When only Name is set the
// shorthand string form is used (preserves byte-for-byte equivalence with the
// legacy schema in exemplars and golden files). Otherwise the full object form
// is emitted so deck round-trips capture path/url/svg_data/fill/alt/position.
//
// Pattern cell types use *IconRef with json:"icon,omitempty" so an absent icon
// is omitted from the wire form entirely; MarshalJSON only runs on non-nil
// refs, so empty IconRef values never reach this code via the pointer path.
func (r IconRef) MarshalJSON() ([]byte, error) {
	// Compact shorthand path: bundled-name-only refs marshal as a JSON string
	// to keep size-budgeted goldens and on-the-wire payloads tight.
	if r.Name != "" && r.Path == "" && r.URL == "" && r.SVGData == "" &&
		r.Fill == "" && r.Alt == "" && r.Position == "" {
		return json.Marshal(r.Name)
	}
	return json.Marshal(jsonschema.IconInput(r))
}

// IconRefOrNil returns nil when the ref is empty, and a non-nil pointer copy
// otherwise. Useful for tests that construct cells with value-type literals and
// want omitempty to apply downstream.
func IconRefOrNil(r IconRef) *IconRef {
	if r.IsEmpty() {
		return nil
	}
	return &r
}

// IsEmpty returns true when no source field (Name/Path/URL/SVGData) is set.
// Decorative-only fields (Fill, Position, Alt) without a source are still
// considered empty — there is nothing to render.
func (r IconRef) IsEmpty() bool {
	return strings.TrimSpace(r.Name) == "" &&
		strings.TrimSpace(r.Path) == "" &&
		strings.TrimSpace(r.URL) == "" &&
		strings.TrimSpace(r.SVGData) == ""
}

// Resolve produces a *jsonschema.IconInput suitable for embedding in a
// ShapeSpecInput. defaultFill and defaultPosition are applied only when the
// caller left those fields blank, so author-supplied overrides win. Returns nil
// when the ref is empty (so the caller can omit the icon overlay).
func (r IconRef) Resolve(defaultFill, defaultPosition string) *jsonschema.IconInput {
	if r.IsEmpty() {
		return nil
	}
	out := jsonschema.IconInput(r)
	if out.Fill == "" {
		out.Fill = defaultFill
	}
	if out.Position == "" {
		out.Position = defaultPosition
	}
	return &out
}

// parseIconString classifies a bare string and routes it to the appropriate
// IconRef field. This is shared by IconRef.UnmarshalJSON and the per-cell
// shorthand parsers ("icon | caption", "header | body | icon") so JSON-level
// behavior matches Go-literal construction.
func parseIconString(s string) IconRef {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return IconRef{}
	}
	switch svggen.ClassifyIcon(s) {
	case svggen.IconKindURL, svggen.IconKindDataURI:
		return IconRef{URL: s}
	case svggen.IconKindInlineSVG:
		return IconRef{SVGData: s}
	case svggen.IconKindFilePath:
		return IconRef{Path: s}
	default:
		// IconKindName and IconKindEmpty both fall through to Name so unknown
		// short strings produce a "not a bundled icon" Validate error rather
		// than a shape-mismatch unmarshal error.
		return IconRef{Name: s}
	}
}

// validateIconRef checks an IconRef for arity (at most one source field set),
// validates bundled names against the registry, and rejects non-SVG paths.
// URL and inline SVG sources are accepted as-is; full URL resolution and SVG
// size enforcement happen later in the cmd/json2pptx asset pipeline.
//
// Returns a nil slice when the ref is empty or fully valid. pattern and path
// identify the location for error messages (e.g. "card-grid", "cells[2].icon").
func validateIconRef(pattern, path string, ref IconRef) []error {
	if ref.IsEmpty() {
		return nil
	}
	var errs []error
	sources := 0
	if ref.Name != "" {
		sources++
	}
	if ref.Path != "" {
		sources++
	}
	if ref.URL != "" {
		sources++
	}
	if ref.SVGData != "" {
		sources++
	}
	if sources > 1 {
		errs = append(errs, &ValidationError{
			Pattern: pattern,
			Path:    path,
			Code:    ErrCodeInvalidShape,
			Message: fmt.Sprintf("%s: %s must set exactly one of name/path/url/svg_data, got %d", pattern, path, sources),
		})
		return errs
	}
	switch {
	case ref.Name != "":
		if !icons.Exists(ref.Name) {
			errs = append(errs, &ValidationError{
				Pattern: pattern,
				Path:    path + ".name",
				Code:    ErrCodeInvalidShape,
				Message: fmt.Sprintf("%s: %s must be a bundled icon name (e.g. \"rocket\"); emoji glyphs and unknown names are rejected — call list_icons or supply path/url/svg_data instead, got %q", pattern, path, ref.Name),
			})
		}
	case ref.Path != "":
		if ext := strings.ToLower(filepath.Ext(ref.Path)); ext != ".svg" {
			errs = append(errs, &ValidationError{
				Pattern: pattern,
				Path:    path + ".path",
				Code:    ErrCodeInvalidShape,
				Message: fmt.Sprintf("%s: %s.path must be a .svg file (got %q); raster formats are not supported for icons", pattern, path, ext),
			})
		}
	}
	return errs
}

// IconRefSchema returns a OneOf schema accepting either a string shorthand
// (bundled name, URL, inline SVG, or file path — classified at unmarshal time)
// or a full IconInput object with explicit name/path/url/svg_data/fill/alt/
// position fields. description overrides the default OneOf description; pass
// "" for the default.
//
// Descriptions are kept terse so per-pattern schemas stay under the 6 KB
// compression budget when an icon slot is repeated across multiple cells.
func IconRefSchema(description string) *Schema {
	if description == "" {
		description = "Icon: name string or {name|path|url|svg_data, fill?, alt?, position?} object"
	}
	objectSchema := ObjectSchema(
		map[string]*Schema{
			"name":     StringSchema(60),
			"path":     StringSchema(0),
			"url":      StringSchema(0),
			"svg_data": StringSchema(0),
			"alt":      StringSchema(120),
			"fill":     StringSchema(0),
			"position": EnumSchema("left", "top", "center"),
		},
		nil,
	).WithAdditionalProperties(false)
	return OneOfSchema(StringSchema(0), objectSchema).WithDescription(description)
}
