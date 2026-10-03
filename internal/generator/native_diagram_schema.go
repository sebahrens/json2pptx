package generator

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Native diagram data validation (go-slide-creator-hdx2l).
//
// The native builders read their data with type assertions on known keys and
// skip anything else. A pyramid whose levels say {"title": ...} instead of
// {"label": ...}, or a house whose sections say {"title", "bullets"} instead of
// {"label", "items"}, therefore parsed cleanly and rendered every shape with
// an empty label — in a placeholder, a shape_grid cell and a compose segment
// alike — while validate called the deck clean.
//
// nativeDataSchemas is the per-type data contract those builders implement:
// for every native type, the keys each level of the payload may carry. It is
// the single source validate, validate_input and generate all check against,
// so an unknown or misspelled key is refused with the keys that would have
// rendered (and a did-you-mean) instead of silently drawing nothing.
//
// Two key sets per level:
//   - fields are the keys the builder actually reads; they are listed in the
//     error as the expected keys.
//   - tolerated are keys documented for the type (docs/diagrams/*.md, the
//     diagram capability metadata) that the native builder does not draw —
//     legacy svggen styling knobs such as gap or corner_radius, a footnote.
//     Refusing them would break payloads the docs told agents to write, so
//     they pass without being advertised.
//
// business_model_canvas is deliberately absent: its unread keys are reported
// as the review finding diagram.data_key_ignored (go-slide-creator-b7qqg.17).

// nativeDataShape describes the keys one object level of a diagram payload
// may carry. A child shape applies to the key's value when that value is an
// object, or to each object element when it is an array; a nil child is a
// scalar, a list of strings, or a value this check does not descend into.
type nativeDataShape struct {
	fields    map[string]*nativeDataShape
	tolerated map[string]bool
}

// shapeOf builds a nativeDataShape from field and tolerated-key lists.
func shapeOf(fields map[string]*nativeDataShape, tolerated ...string) *nativeDataShape {
	s := &nativeDataShape{fields: fields, tolerated: make(map[string]bool, len(tolerated))}
	for _, k := range tolerated {
		s.tolerated[k] = true
	}
	return s
}

// leaves maps keys to nil child shapes.
func leaves(keys ...string) map[string]*nativeDataShape {
	m := make(map[string]*nativeDataShape, len(keys))
	for _, k := range keys {
		m[k] = nil
	}
	return m
}

// with adds keys sharing one child shape to m and returns it.
func with(m map[string]*nativeDataShape, child *nativeDataShape, keys ...string) map[string]*nativeDataShape {
	for _, k := range keys {
		m[k] = child
	}
	return m
}

// nativeTopTolerated are keys any native payload may carry undrawn: the
// diagram-level title/subtitle/footnote the per-type docs list as optional.
var nativeTopTolerated = []string{"title", "subtitle", "footnote"}

func topShape(fields map[string]*nativeDataShape, tolerated ...string) *nativeDataShape {
	return shapeOf(fields, append(append([]string{}, nativeTopTolerated...), tolerated...)...)
}

var nativeDataSchemas = buildNativeDataSchemas()

func buildNativeDataSchemas() map[string]*nativeDataShape {
	// house_diagram: a labelled box (roof, foundation, center) and a section.
	houseBox := shapeOf(leaves("label"), "id", "description", "icon", "color", "size")
	houseSection := shapeOf(leaves("label", "items"), "id", "description", "icon", "color", "size")
	houseFloor := shapeOf(with(leaves("type", "label", "items"), houseSection, "sections", "pillars", "columns"),
		"id", "description", "icon", "color", "size")

	pfStep := shapeOf(leaves("id", "label", "title", "name", "description", "type"), "icon", "color")
	pfConn := shapeOf(leaves("from", "to", "label", "style"), "color")

	vcActivity := shapeOf(leaves("label", "name", "title", "items", "description"), "id", "icon", "color")

	porterForce := shapeOf(leaves("type", "label", "intensity", "factors", "description"), "color")
	porterTop := with(leaves(), porterForce, "forces")
	for alias := range porterForceKeyAliases {
		porterTop[alias] = porterForce
	}

	nineBoxItem := shapeOf(leaves("name"), "subtitle", "title", "size", "color")
	nineBoxCell := shapeOf(map[string]*nativeDataShape{
		"position": shapeOf(leaves("row", "col")),
		"row":      nil, "col": nil, "label": nil,
		"items": nineBoxItem,
	}, "description", "color")
	nineBoxEmployee := shapeOf(leaves("name", "performance", "potential"), "title", "subtitle")

	panel := shapeOf(leaves("title", "value", "body", "icon"), "color")
	panelTop := topShape(with(leaves("layout"), panel, "panels"),
		"gap", "corner_radius", "icon_size", "separator_width", "callout")

	return map[string]*nativeDataShape{
		"pyramid": topShape(with(leaves(), shapeOf(leaves("label", "description"), "color"), "levels"),
			"description", "gap", "top_width_ratio", "label_position"),
		"house_diagram": topShape(
			with(with(with(leaves(), houseBox, "roof", "foundation", "center_element"),
				houseSection, "sections", "pillars", "columns", "outer_elements"),
				houseFloor, "floors"),
			"show_connectors"),
		"swot": topShape(leaves("strengths", "weaknesses", "opportunities", "threats"),
			"gap", "quadrant_opacity", "corner_radius"),
		"pestel": topShape(with(leaves("political", "economic", "social", "technological", "environmental", "legal"),
			shapeOf(leaves("name", "category", "items"), "color"), "segments", "factors")),
		"nine_box_talent": topShape(
			with(with(leaves("x_axis_label", "y_axis_label", "x_axis_labels", "y_axis_labels"),
				nineBoxCell, "cells"), nineBoxEmployee, "employees"),
			"x_label", "y_label", "color_scheme"),
		"value_chain": topShape(
			with(leaves("margin_label", "margin", "show_margin"), vcActivity,
				"primary", "primary_activities", "support", "support_activities"),
			"show_arrows"),
		"kpi_dashboard": topShape(
			with(leaves(), shapeOf(leaves("label", "value", "change", "delta", "unit", "trend"), "color"), "metrics", "kpis"),
			"gap", "max_columns", "corner_radius"),
		"porters_five_forces": topShape(porterTop, "industry_name"),
		"process_flow": topShape(with(with(leaves("direction"), pfStep, "steps"), pfConn, "connections"),
			"description"),
		"heatmap": topShape(leaves("values", "rows", "row_labels", "y_labels", "col_labels", "x_labels",
			"column_labels", "color_scale")),
		"panel_layout": panelTop,
		"icon_columns": panelTop,
		"icon_rows":    panelTop,
		"stat_cards":   panelTop,
	}
}

// NativeDiagramDataError is one problem with a native diagram's data payload
// that would leave text off the slide: a key the builder does not read, or a
// payload whose every label parses empty.
type NativeDiagramDataError struct {
	// DiagramType is the diagram's type.
	DiagramType string
	// Field locates the problem inside the diagram, dotted from "data"
	// ("data.levels[0].title"); slidepath.Field turns it into a pointer.
	Field string
	// Key is the unknown key, or "" for the empty-labels case.
	Key string
	// Expected are the keys the builder reads at that level.
	Expected []string
	// DidYouMean is the expected key the unknown one most likely meant.
	DidYouMean string
	// Occurrences counts the elements of the same list that carry Key.
	Occurrences int
	// Reason is set for a problem that is not an unknown key.
	Reason string
}

func (e *NativeDiagramDataError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("%s: %s: %s", e.DiagramType, e.Field, e.Reason)
	}
	msg := fmt.Sprintf("%s: %s: unknown key %q — it is not drawn, so its text would never reach the slide; expected keys: %s",
		e.DiagramType, e.Field, e.Key, strings.Join(e.Expected, ", "))
	if e.Occurrences > 1 {
		msg += fmt.Sprintf(" (%d list elements carry it)", e.Occurrences)
	}
	if e.DidYouMean != "" {
		msg += fmt.Sprintf("; did you mean %q?", e.DidYouMean)
	}
	return msg
}

// NativeDiagramDataErrors checks a native diagram's data against the type's
// data contract and returns every problem found, in a stable order. It returns
// nil for a non-native type, for business_model_canvas (whose unread keys are
// a review finding), and for a payload the builders draw in full.
func NativeDiagramDataErrors(spec *types.DiagramSpec) []*NativeDiagramDataError {
	if spec == nil || !IsNativeDiagramType(spec) {
		return nil
	}
	schema, ok := nativeDataSchemas[spec.Type]
	if !ok {
		return nil
	}
	var out []*NativeDiagramDataError
	seen := map[string]*NativeDiagramDataError{}
	checkNativeDataShape(spec.Type, "data", spec.Data, schema, &out, seen)
	if len(out) == 0 {
		if reason := nativeDiagramAllLabelsEmpty(spec); reason != "" {
			out = append(out, &NativeDiagramDataError{DiagramType: spec.Type, Field: "data", Reason: reason})
		}
	}
	return out
}

// ValidateNativeDiagramData is NativeDiagramDataErrors as one error, or nil.
func ValidateNativeDiagramData(spec *types.DiagramSpec) error {
	errs := NativeDiagramDataErrors(spec)
	if len(errs) == 0 {
		return nil
	}
	joined := make([]error, len(errs))
	for i, e := range errs {
		joined[i] = e
	}
	return errors.Join(joined...)
}

var listIndexRE = regexp.MustCompile(`\[\d+\]`)

func checkNativeDataShape(diagramType, path string, value any, shape *nativeDataShape, out *[]*NativeDiagramDataError, seen map[string]*NativeDiagramDataError) {
	if shape == nil {
		return
	}
	switch v := value.(type) {
	case []any:
		for i, el := range v {
			checkNativeDataShape(diagramType, fmt.Sprintf("%s[%d]", path, i), el, shape, out, seen)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child, known := shape.fields[k]
			if known {
				checkNativeDataShape(diagramType, path+"."+k, v[k], child, out, seen)
				continue
			}
			if shape.tolerated[k] {
				continue
			}
			// One error per key per list: levels[0..n].title is one mistake.
			dedupe := listIndexRE.ReplaceAllString(path, "[]") + "." + k
			if prev, dup := seen[dedupe]; dup {
				prev.Occurrences++
				continue
			}
			e := &NativeDiagramDataError{
				DiagramType: diagramType,
				Field:       path + "." + k,
				Key:         k,
				Expected:    shape.expectedKeys(),
				Occurrences: 1,
			}
			e.DidYouMean = nativeKeySuggestion(k, e.Expected)
			seen[dedupe] = e
			*out = append(*out, e)
		}
	}
}

func (s *nativeDataShape) expectedKeys() []string {
	keys := make([]string, 0, len(s.fields))
	for k := range s.fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// nativeKeySynonyms are the spellings agents reach for that edit distance
// cannot connect to the key the builder reads ("title" is four edits from
// "label"). The first synonym the level actually accepts is suggested.
var nativeKeySynonyms = map[string][]string{
	"title":       {"label", "name"},
	"name":        {"label", "title"},
	"text":        {"label", "title", "body"},
	"heading":     {"label", "title"},
	"header":      {"label", "title"},
	"caption":     {"label", "title"},
	"bullets":     {"items", "factors"},
	"points":      {"items", "factors"},
	"children":    {"items"},
	"details":     {"description", "items", "body"},
	"desc":        {"description", "body"},
	"subtitle":    {"description", "body"},
	"description": {"body", "items"},
	"content":     {"body", "items", "description"},
	"tiers":       {"levels"},
	"layers":      {"levels"},
	"stages":      {"steps", "levels"},
	"pillars":     {"sections"},
	"activities":  {"items", "primary"},
}

func nativeKeySuggestion(key string, expected []string) string {
	accepted := make(map[string]bool, len(expected))
	for _, k := range expected {
		accepted[k] = true
	}
	for _, s := range nativeKeySynonyms[strings.ToLower(key)] {
		if accepted[s] {
			return s
		}
	}
	if match, dist := ClosestMatch(key, expected, 3); dist >= 0 && dist < len([]rune(key)) {
		return match
	}
	return ""
}

// nativeDiagramAllLabelsEmpty reports why a payload whose keys all pass would
// still draw no text: every label and body the builder parsed is empty.
func nativeDiagramAllLabelsEmpty(spec *types.DiagramSpec) string {
	texts := nativeDiagramTexts(spec)
	if len(texts) == 0 {
		return "" // nothing parsed (the builder's own error says so) or not checked
	}
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			return ""
		}
	}
	return "every label and body in the payload is empty, so the diagram would draw only blank shapes — check the per-type data keys (get_diagram_capabilities / docs/diagrams)"
}

// nativeDiagramTexts returns the label and body text the builder would draw
// for each parsed item, or nil when the payload parses to no items. Types
// whose boxes carry built-in titles (SWOT, nine-box, five forces) and the
// numeric heatmap always draw something and return nil.
func nativeDiagramTexts(spec *types.DiagramSpec) []string {
	switch {
	case isPanelNativeLayout(spec):
		var texts []string
		raw, _ := spec.Data["panels"].([]any)
		for _, item := range raw {
			m, _ := item.(map[string]any)
			title, _ := m["title"].(string)
			value, _ := m["value"].(string)
			body, _ := m["body"].(string)
			texts = append(texts, title, value, body)
		}
		return texts
	case isPyramidDiagram(spec):
		levels, _ := parsePyramidDiagramData(spec.Data)
		texts := make([]string, 0, 2*len(levels))
		for _, l := range levels {
			texts = append(texts, l.label, l.description)
		}
		return texts
	case isHouseDiagram(spec):
		panels, _, _ := parseHouseDiagramNativeData(spec.Data)
		return nativePanelTexts(panels)
	case isValueChainDiagram(spec):
		panels, _ := parseValueChainData(spec.Data)
		return nativePanelTexts(panels)
	case isProcessFlowDiagram(spec):
		steps, _, _ := parseProcessFlowDiagramData(spec.Data)
		texts := make([]string, 0, 2*len(steps))
		for _, st := range steps {
			texts = append(texts, st.label, st.description)
		}
		return texts
	case isPESTELDiagram(spec):
		return nativePanelTexts(parsePESTELSegments(spec.Data))
	case isKPIDashboardDiagram(spec):
		metrics := parseKPIMetrics(spec.Data)
		texts := make([]string, 0, 2*len(metrics))
		for _, m := range metrics {
			texts = append(texts, m.label, m.value)
		}
		return texts
	}
	return nil
}

func nativePanelTexts(panels []nativePanelData) []string {
	texts := make([]string, 0, 2*len(panels))
	for _, p := range panels {
		texts = append(texts, p.title, p.body)
	}
	return texts
}
