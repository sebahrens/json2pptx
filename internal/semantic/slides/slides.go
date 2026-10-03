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
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Input is the per-slide compile input: the planned slide plus the bookkeeping
// the compilers need to emit source-map links. Title and Takeaway are lifted
// from the payload by the planner; Body carries the full kind-specific payload.
type Input struct {
	// SourceIndex is the slide's index in the semantic DeckSpec.Slides.
	SourceIndex int
	SourcePath  string
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
	// Override is the composition the author asked for with the slide's
	// pattern / layout field, already matched against the kind's advertised
	// candidates; zero when there is none. A compiler that offers the
	// composition must emit it when the payload fits, rather than re-deriving
	// its own choice from the body (go-slide-creator-vj549).
	Override Composition
}

// Composition is a pattern / layout pair a slide can be asked to take.
type Composition struct {
	Pattern string
	Layout  string
}

// wantsContent reports whether the author asked for the native content-slide
// composition: source-complete bullets, no pattern.
func (in Input) wantsContent() bool {
	return in.Override.Pattern == "" && in.Override.Layout == "content"
}

func diagramContent(placeholderID string, diagram *types.DiagramSpec) deckinput.ContentInput {
	return deckinput.ContentInput{PlaceholderID: placeholderID, Type: "diagram", DiagramValue: diagram}
}

// blankTitleCellPath is the raw path of the single cell blankTitleCellSlide
// hosts its visual in.
const blankTitleCellPath = ".shape_grid.rows[0].cells[0]"

// blankTitleCellSlide hosts one native visual (a table or a diagram) in a
// one-cell shape grid on the blank-title layout. Every pattern kind renders on
// blank-title; a native table or diagram used to need the template's body
// placeholder instead, so the layout matcher picked a different layout for it
// (on abstract, "One Content" with its own smaller, lower title) and the title
// jumped position mid-deck (go-slide-creator-ngbnf). Hosting the visual in the
// grid keeps one title treatment across every DeckSpec content kind.
func blankTitleCellSlide(cell *deckinput.GridCellInput) *deckinput.SlideInput {
	return &deckinput.SlideInput{
		SlideType: "content",
		LayoutID:  "blank-title",
		ShapeGrid: &deckinput.ShapeGridInput{
			Columns: json.RawMessage("1"),
			Rows:    []deckinput.GridRowInput{{Cells: []*deckinput.GridCellInput{cell}}},
		},
	}
}

// visualAltText is the alt text a compiled chart, diagram or table carries: the
// slide's takeaway, else its title. A semantic slide states its point in one
// sentence, and that sentence is exactly what a screen reader should hear about
// the visual making it — better than any description the engine can derive from
// the data, and free for every semantic deck (go-slide-creator-6e8h). An alt the
// author wrote on the payload itself always wins; callers only apply this when
// the field is empty.
func visualAltText(in Input) string {
	if in.Takeaway != "" {
		return in.Takeaway
	}
	return in.Title
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
	if in.SourcePath != "" {
		return in.SourcePath
	}
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
// is absent or not a string. A numeric value is rendered as its plain decimal
// text (as stringList does) rather than silently dropped: a nested kpis[].value
// of 48 or milestones[].date of 2024 is content, not an absent field
// (go-slide-creator-csclk.43).
func strField(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	s, _ := scalarText(body[key])
	return s
}

// scalarText renders a string or numeric payload value as trimmed text. The
// bool reports whether v was such a scalar.
func scalarText(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case json.Number:
		return t.String(), true
	case int:
		return strconv.Itoa(t), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case uint64:
		return strconv.FormatUint(t, 10), true
	}
	return "", false
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
		case map[string]any:
			// An object entry (e.g. a {title, description} step) must never be
			// stringified with fmt's default %v, which leaks Go "map[...]" syntax
			// into slide body content. Render it as a readable label line instead;
			// drop it when it carries no usable text.
			if s := objectText(t); s != "" {
				out = append(out, s)
			}
		case []any:
			// Nested lists have no sensible single-line rendering here; skip them
			// rather than emit a Go "[...]" dump.
		case nil:
			// skip
		case float64:
			// JSON numbers decode to float64; the default %v uses %g, which
			// switches to exponent form for large/small magnitudes (1e+06,
			// 1e-07) and leaks scientific notation into slide bullets. Format as
			// a plain decimal with the minimal digits needed to round-trip.
			out = append(out, strconv.FormatFloat(t, 'f', -1, 64))
		case json.Number:
			// Rendered verbatim if the decoder is ever switched to UseNumber, so
			// the author's exact numeric literal survives.
			out = append(out, t.String())
		case bool:
			// Stringify booleans deliberately rather than letting %v emit a Go
			// literal incidentally.
			out = append(out, strconv.FormatBool(t))
		default:
			// Integer and other scalar types render without exponent risk.
			out = append(out, fmt.Sprintf("%v", t))
		}
	}
	return out, true
}

// objectText renders a structured list entry (a map) as a single readable line:
// its primary label, optionally followed by " — <detail>". It is the guard that
// keeps Go "map[...]" syntax out of generated slide content when a payload list
// holds objects rather than strings. It returns "" when the entry carries no
// usable text.
func objectText(m map[string]any) string {
	label := firstNonEmpty(
		strField(m, "title"), strField(m, "label"), strField(m, "name"),
		strField(m, "heading"), strField(m, "step"), strField(m, "phase"),
		strField(m, "text"),
	)
	detail := firstNonEmpty(
		strField(m, "description"), strField(m, "detail"),
		strField(m, "summary"), strField(m, "caption"), strField(m, "body"),
	)
	switch {
	case label != "" && detail != "":
		return label + " — " + detail
	case label != "":
		return label
	default:
		return detail
	}
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
