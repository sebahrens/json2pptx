// Package slides holds the per-kind compilers that turn a planned semantic
// slide into a raw internal/deckinput.SlideInput. It imports only deckinput (and
// the shared types it references), never the parent internal/semantic package,
// so the dependency runs one way: semantic -> slides -> deckinput. The parent
// package builds an Input from each planned SlideIR, dispatches to the matching
// CompileX function here, and registers the returned SourceLinks in its
// SourceMap.
//
// Every compiler is a pure function: given an Input it returns a SlideInput, the
// raw->semantic source links it emitted, and an error only for genuinely
// malformed payloads. The compilers favour safe, always-valid output (a content
// slide with bullets) over failing, so a deck that passed semantic validation
// always compiles.
package slides

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Input is the per-slide compile input: the planned slide plus the bookkeeping
// the compilers need to emit source-map links. Title and Takeaway are lifted
// from the payload by the planner; Body carries the full kind-specific payload.
type Input struct {
	// SourceIndex is the slide's index in the semantic DeckSpec.Slides.
	SourceIndex int
	// OutputIndex is the slide's index in the emitted PresentationInput.Slides.
	OutputIndex int
	// Title is the slide title extracted from the payload (may be empty).
	Title string
	// Takeaway is the slide's one-line takeaway/insight, if present.
	Takeaway string
	// Pattern is the named pattern the planner selected, or "" for none.
	Pattern string
	// Layout is the planner's slide_type/layout selection.
	Layout string
	// Body is the kind-specific semantic payload.
	Body map[string]any
}

// SourceLink records one raw->semantic correspondence a compiler emitted. The
// parent package feeds these into its SourceMap so a raw fit-report path can be
// traced back to the semantic field the author wrote.
type SourceLink struct {
	RawPath      string
	SemanticPath string
}

// rawSlide returns the raw JSON path prefix for the emitted slide.
func (in Input) rawSlide() string {
	return fmt.Sprintf("slides[%d]", in.OutputIndex)
}

// semSlide returns the semantic JSON path prefix for the source slide.
func (in Input) semSlide() string {
	return fmt.Sprintf("slides[%d]", in.SourceIndex)
}

// appendContent appends a content item to the slide and returns the index it
// landed at, so callers can build the raw source path for the item without
// tracking a separate counter.
func appendContent(slide *deckinput.SlideInput, c deckinput.ContentInput) int {
	idx := len(slide.Content)
	slide.Content = append(slide.Content, c)
	return idx
}

// textContent builds a text content item bound to a placeholder.
func textContent(placeholderID, text string) deckinput.ContentInput {
	v := text
	return deckinput.ContentInput{
		PlaceholderID: placeholderID,
		Type:          "text",
		TextValue:     &v,
	}
}

// bulletsContent builds a bullets content item bound to a placeholder.
func bulletsContent(placeholderID string, bullets []string) deckinput.ContentInput {
	b := append([]string(nil), bullets...)
	return deckinput.ContentInput{
		PlaceholderID: placeholderID,
		Type:          "bullets",
		BulletsValue:  &b,
	}
}

// bodyAndBulletsContent builds a body_and_bullets content item: a lead-in body
// paragraph followed by supporting bullets.
func bodyAndBulletsContent(placeholderID, body string, bullets []string) deckinput.ContentInput {
	return deckinput.ContentInput{
		PlaceholderID: placeholderID,
		Type:          "body_and_bullets",
		BodyAndBulletsValue: &deckinput.BodyAndBulletsInput{
			Body:    body,
			Bullets: append([]string(nil), bullets...),
		},
	}
}

// strField returns the trimmed string value of a payload field, or "" when it
// is absent or not a string.
func strField(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	if v, ok := body[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// stringList returns the string entries of a list-valued payload field. Non-
// string entries are coerced via fmt when they are scalars and skipped when
// blank. The bool reports whether the field was a list at all.
func stringList(body map[string]any, key string) ([]string, bool) {
	if body == nil {
		return nil, false
	}
	raw, ok := body[key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, s)
			}
		case nil:
			// skip
		default:
			out = append(out, fmt.Sprintf("%v", t))
		}
	}
	return out, true
}

// mapList returns the map entries of a list-valued payload field, preserving
// order and skipping non-map entries.
func mapList(body map[string]any, key string) []map[string]any {
	raw, ok := body[key].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// firstNonEmpty returns the first non-empty trimmed string among its arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// sortedKeys returns the keys of a payload map in sorted order, for
// deterministic iteration.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
