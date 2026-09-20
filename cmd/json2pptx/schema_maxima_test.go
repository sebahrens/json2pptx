package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// A pattern's JSON schema is the contract an agent sizes its copy against. Where
// a field's maxLength is far above what the cell can hold, content that respects
// the schema in every particular still renders as a wall of 4-6pt text — and the
// agent has no way to know before it looks (go-slide-creator-0g6p).
//
// These tests build, for every registered pattern, the largest payload its own
// schema permits, expand it at the default geometry and measure what the cells
// would have to hold.

// schemaMaximaTemplates are the bundled templates the measurement runs on: a
// pattern tuned on one palette's geometry can still overflow on another.
var schemaMaximaTemplates = []string{"midnight-blue", "warm-coral", "forest-green", "modern-template"}

// TestSchemaMaximaStayReadable measures, for every registered pattern, the
// smallest text its OWN schema maximum would render at — the same prediction
// the fit report gives an agent — and pins it. A pattern whose schema permits
// content that renders below the readable floor is one an agent cannot size its
// copy against by reading the contract.
func TestSchemaMaximaStayReadable(t *testing.T) {
	measured := map[string]float64{}
	for _, pat := range patterns.Default().List() {
		// The score is the SMALLEST size any cell would render at across the
		// bundled templates — the worst case an author can hit — so a lower
		// number is a worse pattern, and 0 ("no cell drops below the floor")
		// is the best of all.
		worst := 0.0
		for _, tpl := range schemaMaximaTemplates {
			pt, note := measureSchemaMaximumPt(t, pat, tpl)
			if note != "" {
				t.Logf("%s on %s: %s", pat.Name(), tpl, note)
				continue
			}
			if pt > 0 && (worst == 0 || pt < worst) {
				worst = pt
			}
		}
		measured[pat.Name()] = worst
	}

	names := make([]string, 0, len(measured))
	for name := range measured {
		names = append(names, name)
	}
	// Worst first: the smallest surviving size is the most broken pattern.
	sort.Slice(names, func(i, j int) bool {
		a, b := measured[names[i]], measured[names[j]]
		if a == 0 || b == 0 {
			return b == 0 && a != 0 // 0 (nothing below the floor) sorts last
		}
		return a < b
	})

	var report strings.Builder
	for _, name := range names {
		fmt.Fprintf(&report, "%-30s %.1fpt\n", name, measured[name])
	}
	t.Logf("smallest size a schema-maximum payload renders at, worst first (0 = no cell drops below the floor):\n%s", report.String())

	for name, shrink := range measured {
		pinned, ok := schemaMaximaShrinkPt[name]
		if !ok {
			t.Errorf("pattern %q has no entry in schemaMaximaShrinkPt (measured %.1fpt) — add one, or bring its schema maxima down to what its cells hold", name, shrink)
			continue
		}
		switch {
		case pinned == 0 && shrink != 0:
			t.Errorf("%s: schema maxima now render text at %.1fpt; the pin says nothing should drop below the floor", name, shrink)
		case pinned != 0 && shrink != 0 && shrink < pinned-0.05:
			t.Errorf("%s: schema maxima now render at %.1fpt, SMALLER than the pinned %.1fpt — a field's maxLength grew past what its cell holds", name, shrink, pinned)
		case pinned != 0 && (shrink == 0 || shrink > pinned+1):
			t.Errorf("%s: schema maxima now render at %.1fpt, better than the pinned %.1fpt — raise the pin to hold the ground", name, shrink, pinned)
		}
	}
}

// measureSchemaMaximumPt builds the largest payload a pattern's schema permits,
// runs it through the same readability prediction the fit report uses, and
// returns the SMALLEST size any of its cells would render at — 0 when every
// cell stays above the floor. A note explains a payload that could not be built.
func measureSchemaMaximumPt(t *testing.T, pat patterns.Pattern, templateName string) (float64, string) {
	t.Helper()
	values, note := schemaMaximumValues(pat)
	if note != "" {
		return 0, note
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return 0, "values do not marshal: " + err.Error()
	}

	input := &PresentationInput{
		Template: templateName,
		Slides: []SlideInput{{
			SlideType: "content",
			LayoutID:  "blank-title",
			Pattern:   &PatternInput{Name: pat.Name(), Values: encoded},
		}},
	}
	layouts, w, h := schemaMaximaLayouts(t, templateName)

	worst := 0.0
	for _, f := range collectReadabilityFindings(input, layouts, w, h) {
		if pt := readabilityRenderedPt(f.Message); pt > 0 && (worst == 0 || pt < worst) {
			worst = pt
		}
	}
	return worst, ""
}

// readabilityRenderedPt reads the predicted size out of a
// TEXT_BELOW_READABLE_MIN message ("... renders at 4.6pt, below ...").
func readabilityRenderedPt(message string) float64 {
	const marker = "renders at "
	i := strings.Index(message, marker)
	if i < 0 {
		return 0
	}
	rest := message[i+len(marker):]
	j := strings.Index(rest, "pt")
	if j < 0 {
		return 0
	}
	var pt float64
	if _, err := fmt.Sscanf(rest[:j], "%g", &pt); err != nil {
		return 0
	}
	return math.Round(pt*10) / 10
}

// schemaMaximaLayouts loads a bundled template's layouts and slide size.
func schemaMaximaLayouts(t *testing.T, name string) ([]types.LayoutMetadata, int64, int64) {
	t.Helper()
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse %s layouts: %v", name, err)
	}
	w, h := template.ParseSlideDimensions(reader)
	return layouts, w, h
}

// schemaMaximumValues builds the largest values payload a pattern's schema
// permits and confirms the pattern itself accepts it.
func schemaMaximumValues(pat patterns.Pattern) (any, string) {
	schema := pat.Schema()
	if schema == nil {
		return nil, "no schema"
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, "schema does not marshal: " + err.Error()
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "schema does not decode: " + err.Error()
	}
	defs, _ := doc["$defs"].(map[string]any)
	props, _ := doc["properties"].(map[string]any)
	valuesSchema, ok := props["values"].(map[string]any)
	if !ok {
		return nil, "schema has no values property"
	}

	generated := coherentMaximum(pat.Name(), maximalValue(valuesSchema, defs, 0))
	encoded, err := json.Marshal(generated)
	if err != nil {
		return nil, "generated values do not marshal: " + err.Error()
	}
	values := pat.NewValues()
	if err := json.Unmarshal(encoded, values); err != nil {
		return nil, "generated values do not decode into the pattern: " + err.Error()
	}
	if err := pat.Validate(values, nil, nil); err != nil {
		return nil, "pattern rejects its own schema maximum: " + firstLine(err.Error())
	}
	return values, ""
}

// schemaMaximaShrinkPt pins, per pattern, the smallest size its schema-maximum
// payload renders at, across ALL four bundled templates — 0 meaning every cell
// stays above the readable floor. The pins are measured, not chosen:
// TestSchemaMaximaStayReadable fails both when a number gets worse and when it
// improves without the pin following, so the ground a fix wins cannot be given
// back (go-slide-creator-0g6p).
var schemaMaximaShrinkPt = map[string]float64{
	"agenda":                       7.8,
	"agenda-with-images":           6.0,
	"arch-stack":                   0,
	"before-after":                 0.0,
	"before-after-compact":         6.0,
	"bmc-canvas":                   3.8,
	"card-grid":                    2.4,
	"chart-insights-split":         9.4,
	"comparison-2col":              4.2,
	"driver-tree":                  4.1,
	"dual-org-ladder":              7.0,
	"exec-summary":                 7.2,
	"hero-detail":                  6.7,
	"horizontal-bar-with-callouts": 6.0,
	"icon-row":                     0.0,
	"image-text-split":             0.0,
	"journey-maturity-model":       9.1,
	"kpi-2up":                      0.0,
	"kpi-3up":                      0.0,
	"kpi-4up":                      0.0,
	"kpi-5up":                      0.0,
	"kpi-6up":                      0.0,
	"kpi-inline":                   5.8,
	"matrix-2x2":                   10.1,
	"numbered-step-strip":          7.0,
	"phase-roadmap":                6.2,
	"process-flow":                 0.0,
	"process-flow-compact":         9.1,
	"process-grid-2row":            0.0,
	"pull-quote":                   17.3,
	"pyramid":                      9.2,
	"quote-cluster":                6.7,
	"roadmap-phased":               5.0,
	"scqa-summary":                 7.4,
	"stat-hero":                    7.4,
	"strategy-house":               8.2,
	"stylish-panels":               6.4,
	"swimlane":                     5.5,
	"table-highlight":              6.7,
	"team-bios":                    5.5,
	"timeline-horizontal":          6.2,
	"value-chain":                  8.2,
	"waterfall-bridge":             7.2,
}

// coherentMaximum applies the cross-field rules a pattern enforces but its
// per-field schema cannot express, so the payload is the largest LEGAL one
// rather than the largest the field bounds alone allow.
func coherentMaximum(pattern string, v any) any {
	dropIconAlternatives(v, 0)
	switch pattern {
	case "card-grid":
		// cells must number exactly columns x rows.
		if m, ok := v.(map[string]any); ok {
			cells, _ := m["cells"].([]any)
			if n := len(cells); n > 0 {
				m["columns"] = 5
				m["rows"] = n / 5
				if m["rows"].(int) < 1 {
					m["columns"], m["rows"] = n, 1
				}
				m["cells"] = cells[:m["columns"].(int)*m["rows"].(int)]
			}
		}
	case "timeline-horizontal":
		// end_date is only legal in gantt style, which lives in overrides.
		if steps, ok := v.([]any); ok {
			for _, st := range steps {
				if m, isMap := st.(map[string]any); isMap {
					delete(m, "end_date")
				}
			}
		}
	case "chart-insights-split":
		// The chart needs data the schema types only as an object.
		if m, ok := v.(map[string]any); ok {
			if chart, isMap := m["chart"].(map[string]any); isMap {
				chart["type"] = "bar_chart"
				chart["data"] = map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series":     []any{map[string]any{"name": "Revenue", "values": []any{1, 2, 3, 4}}},
				}
			}
		}
	}
	return v
}

// dropIconAlternatives keeps one source on every icon object: the patterns
// require exactly one of name / path / url / svg_data and the schema states the
// alternatives side by side.
func dropIconAlternatives(v any, depth int) {
	if depth > 8 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for key, child := range t {
			if m, ok := child.(map[string]any); ok && key == "icon" {
				if _, hasName := m["name"]; hasName {
					m["name"] = "check"
					delete(m, "path")
					delete(m, "url")
					delete(m, "svg_data")
				}
			}
			dropIconAlternatives(child, depth+1)
		}
	case []any:
		for _, child := range t {
			dropIconAlternatives(child, depth+1)
		}
	}
}

// maximalValue builds the largest value a schema node permits: strings at
// maxLength, arrays at maxItems, every property present.
func maximalValue(node map[string]any, defs map[string]any, depth int) any {
	if depth > 6 || node == nil {
		return nil
	}
	if ref, ok := node["$ref"].(string); ok {
		if target := resolveSchemaRef(ref, defs); target != nil {
			return maximalValue(target, defs, depth+1)
		}
		return nil
	}
	if variants, ok := node["oneOf"].([]any); ok && len(variants) > 0 {
		// Take the branch that can hold the most: an object variant carries more
		// than the string shorthand beside it.
		for _, v := range variants {
			if m, isMap := v.(map[string]any); isMap && m["type"] == "object" {
				return maximalValue(m, defs, depth+1)
			}
		}
		if m, isMap := variants[0].(map[string]any); isMap {
			return maximalValue(m, defs, depth+1)
		}
		return nil
	}
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}

	switch node["type"] {
	case "string":
		return strings.Repeat("W", schemaMaxLength(node))
	case "number", "integer":
		return schemaNumber(node)
	case "boolean":
		return false
	case "array":
		items, _ := node["items"].(map[string]any)
		n := schemaMaxItems(node)
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			if v := maximalValue(items, defs, depth+1); v != nil {
				out = append(out, v)
			}
		}
		return out
	case "object":
		props, _ := node["properties"].(map[string]any)
		out := map[string]any{}
		for name, p := range props {
			pm, isMap := p.(map[string]any)
			if !isMap {
				continue
			}
			if v := maximalValue(pm, defs, depth+1); v != nil {
				out[name] = v
			}
		}
		return out
	}
	return nil
}

// resolveSchemaRef follows a local "#/$defs/name" reference.
func resolveSchemaRef(ref string, defs map[string]any) map[string]any {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) || defs == nil {
		return nil
	}
	target, _ := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	return target
}

// schemaMaxLength is the string length a schema permits. A field with no
// maxLength has no stated bound; 400 characters is a long paragraph and is
// enough to show whether the cell can hold one.
func schemaMaxLength(node map[string]any) int {
	if v, ok := node["maxLength"].(float64); ok && v > 0 {
		return int(v)
	}
	return 400
}

// schemaMaxItems is the array length a schema permits, defaulting to a small
// number when the schema states none.
func schemaMaxItems(node map[string]any) int {
	if v, ok := node["maxItems"].(float64); ok && v > 0 {
		return int(v)
	}
	if v, ok := node["minItems"].(float64); ok && v > 0 {
		return int(v)
	}
	return 3
}

// schemaNumber returns a number inside the schema's range.
func schemaNumber(node map[string]any) float64 {
	if v, ok := node["minimum"].(float64); ok {
		return v
	}
	if v, ok := node["maximum"].(float64); ok {
		return v
	}
	return 1
}

// firstLine trims a multi-line error to its first line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
