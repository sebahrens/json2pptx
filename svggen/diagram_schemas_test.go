package svggen

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// TestEveryRegisteredTypeHasDataSchema: a registered type without a data
// contract silently ignores misspelled keys (go-slide-creator-x9s5i). The
// contract must also marshal (no recursive cycles) for get_schema.
func TestEveryRegisteredTypeHasDataSchema(t *testing.T) {
	for _, typ := range DefaultRegistry().Types() {
		d := DefaultRegistry().Get(typ)
		ds, ok := d.(DiagramWithSchema)
		if !ok {
			t.Errorf("%s: registered without a DataSchema", typ)
			continue
		}
		schema := ds.DataSchema()
		if schema == nil || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
			t.Errorf("%s: DataSchema must be a closed object", typ)
			continue
		}
		if _, err := json.Marshal(schema); err != nil {
			t.Errorf("%s: DataSchema does not marshal: %v", typ, err)
		}
	}
}

// TestEveryCapabilityFieldIsAccepted: get_diagram_capabilities tells agents
// which fields an svggen type takes; the data contract must never refuse one.
func TestEveryCapabilityFieldIsAccepted(t *testing.T) {
	for _, c := range DiagramCapabilities() {
		d := DefaultRegistry().Get(c.Type)
		if d == nil {
			continue // native type
		}
		schema := d.(DiagramWithSchema).DataSchema()
		for _, f := range append(append([]string{}, c.RequiredFields...), c.OptionalFields...) {
			if !schemaAcceptsKeyAnywhere(schema, f, 0) {
				t.Errorf("%s: capability field %q is refused by the data contract", c.Type, f)
			}
		}
	}
}

func schemaAcceptsKeyAnywhere(s *DataSchema, key string, depth int) bool {
	if s == nil || depth > 4 {
		return false
	}
	if _, ok := s.Properties[key]; ok {
		return true
	}
	if schemaAcceptsKeyAnywhere(s.Items, key, depth+1) {
		return true
	}
	for _, p := range s.Properties {
		if schemaAcceptsKeyAnywhere(p, key, depth+1) {
			return true
		}
	}
	return false
}

// unknownFieldErrors renders req dry and returns its UNKNOWN_FIELD errors.
func unknownFieldErrors(t *testing.T, req *RequestEnvelope) []core.ValidationError {
	t.Helper()
	_, err := DryRender(req)
	if err == nil {
		t.Fatalf("%s: payload with an unread key rendered", req.Type)
	}
	var out []core.ValidationError
	for _, ve := range core.GetValidationErrors(err) {
		if ve.Code == core.ErrCodeUnknownField {
			out = append(out, ve)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s: no UNKNOWN_FIELD error in %v", req.Type, err)
	}
	return out
}

// TestDataContract_RefusesUnreadKeys pins the bead's cases and one per type:
// the unread key is reported at its path with the keys that would have drawn
// and a did-you-mean.
func TestDataContract_RefusesUnreadKeys(t *testing.T) {
	cases := []struct {
		name       string
		req        *RequestEnvelope
		field      string
		didYouMean string
		occurrence int
	}{
		{"timeline items {title, when}", &RequestEnvelope{Type: "timeline", Data: map[string]any{
			"items": []any{
				map[string]any{"title": "Kickoff", "when": "2026-01"},
				map[string]any{"title": "Launch", "when": "2026-06"},
			},
		}}, "data.items[0].when", "date", 2},
		{"venn circles {title}", &RequestEnvelope{Type: "venn", Data: map[string]any{
			"circles": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}},
		}}, "data.circles[0].title", "label", 2},
		{"bar series misspelled", &RequestEnvelope{Type: "bar_chart", Data: map[string]any{
			"categories": []any{"Q1", "Q2"},
			"series":     []any{map[string]any{"name": "Revenue", "valeus": []any{1.0, 2.0}}},
		}}, "data.series[0].valeus", "values", 1},
		{"org chart nested label", &RequestEnvelope{Type: "org_chart", Data: map[string]any{
			"root": map[string]any{"name": "Ada", "children": []any{map[string]any{"name": "Bo", "role": "CTO"}}},
		}}, "data.root.children[0].role", "title", 1},
		{"gantt task owner", &RequestEnvelope{Type: "gantt", Data: map[string]any{
			"tasks": []any{map[string]any{"name": "Build", "start": "2026-01-01", "end": "2026-02-01", "owner": "Eng"}},
		}}, "data.tasks[0].owner", "swimlane", 1},
		{"matrix point name", &RequestEnvelope{Type: "matrix_2x2", Data: map[string]any{
			"points": []any{map[string]any{"name": "A", "x": 10.0, "y": 20.0}},
		}}, "data.points[0].name", "label", 1},
		{"fishbone top-level causes", &RequestEnvelope{Type: "fishbone", Data: map[string]any{
			"effect": "Late", "causes": []any{map[string]any{"category": "People"}},
		}}, "data.causes", "categories", 1},
		{"waterfall point name", &RequestEnvelope{Type: "waterfall", Data: map[string]any{
			"points": []any{map[string]any{"label": "Start", "value": 10.0, "note": "x"}},
		}}, "data.points[0].note", "", 1},
		{"funnel stage name", &RequestEnvelope{Type: "funnel", Data: map[string]any{
			"values": []any{map[string]any{"name": "Leads", "value": 100.0}},
		}}, "data.values[0].name", "label", 1},
		{"gauge threshold", &RequestEnvelope{Type: "gauge", Data: map[string]any{
			"value": 50.0, "thresholds": []any{map[string]any{"value": 30.0, "colour": "#ff0000"}},
		}}, "data.thresholds[0].colour", "color", 1},
		{"treemap nested child", &RequestEnvelope{Type: "treemap", Data: map[string]any{
			"nodes": []any{map[string]any{"label": "A", "children": []any{map[string]any{"name": "A1", "value": 3.0}}}},
		}}, "data.nodes[0].children[0].name", "label", 1},
		{"venn intersection reversed", &RequestEnvelope{Type: "venn", Data: map[string]any{
			"circles":       []any{map[string]any{"label": "A"}, map[string]any{"label": "B"}},
			"intersections": map[string]any{"ba": "Both"},
		}}, "data.intersections.ba", "ab", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := unknownFieldErrors(t, tc.req)
			got := errs[0]
			if got.Field != tc.field {
				t.Errorf("field = %q, want %q (%v)", got.Field, tc.field, got.Message)
			}
			if got.DidYouMean != tc.didYouMean {
				t.Errorf("did_you_mean = %q, want %q (%v)", got.DidYouMean, tc.didYouMean, got.Message)
			}
			if got.Occurrences != tc.occurrence {
				t.Errorf("occurrences = %d, want %d", got.Occurrences, tc.occurrence)
			}
			if len(got.Expected) == 0 {
				t.Error("no expected keys listed")
			}
			// CheckDataContract reports the same problem without rendering.
			if err := CheckDataContract(tc.req.Type, tc.req.Data); err == nil {
				t.Error("CheckDataContract accepted the payload")
			}
		})
	}
}

// TestDataContract_TolerantOfRenderNormalization: Validate rewrites some
// payloads in place (flat org nodes -> root plus an issues key, events ->
// activities). Rendering the same map again must still pass the contract.
func TestDataContract_TolerantOfRenderNormalization(t *testing.T) {
	reqs := []*RequestEnvelope{
		{Type: "org_chart", Data: map[string]any{"nodes": []any{
			map[string]any{"id": "a", "name": "Ada"},
			map[string]any{"id": "b", "name": "Bo", "parent": "zz"},
		}}},
		{Type: "timeline", Data: map[string]any{"events": []any{map[string]any{"label": "Go", "date": "2026-01"}}}},
		{Type: "fishbone", Data: map[string]any{"problem": "Late", "categories": []any{map[string]any{"name": "People", "causes": []any{"x"}}}}},
		{Type: "waterfall", Data: map[string]any{"labels": []any{"A", "B"}, "values": []any{1.0, -0.5}}},
		{Type: "treemap", Data: map[string]any{"items": []any{map[string]any{"label": "A", "value": 1.0}}}},
		{Type: "funnel", Data: map[string]any{"stages": []any{map[string]any{"label": "A", "value": 1.0}}}},
	}
	for _, req := range reqs {
		for pass := 1; pass <= 2; pass++ {
			if _, err := DryRender(req); err != nil {
				t.Errorf("%s pass %d: %v", req.Type, pass, err)
			}
		}
	}
}

// TestDataContract_RefusesBlankPayload: keys that pass but labels that are
// all empty would draw blank shapes.
func TestDataContract_RefusesBlankPayload(t *testing.T) {
	data := map[string]any{"circles": []any{map[string]any{"label": ""}, map[string]any{"label": " "}}}
	err := CheckDataContract("venn", data)
	var ve *core.ValidationError
	if !errors.As(err, &ve) || ve.Code != ErrCodeBlankPayload {
		t.Fatalf("CheckDataContract = %v, want a %s error", err, ErrCodeBlankPayload)
	}
	if _, err := DryRender(&RequestEnvelope{Type: "venn", Data: data}); err == nil {
		t.Error("blank venn rendered")
	}
}

// TestDataContract_AcceptsDocumentedExamples: every diagram or chart payload
// in a JSON example of the repo's docs and skills passes its type's data
// contract — the docs are what agents copy. Skipped when the svggen module is
// checked out on its own.
func TestDataContract_AcceptsDocumentedExamples(t *testing.T) {
	var files []string
	for _, dir := range []string{"../docs", "../skills"} {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			// The changelog quotes historical and chart-content (flat label ->
			// value map) payloads, which are not raw svggen data.
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") && filepath.Base(path) != "SCHEMA_CHANGELOG.md" {
				files = append(files, path)
			}
			return nil
		})
	}
	if len(files) == 0 {
		t.Skip("repo docs not present")
	}
	block := regexp.MustCompile("(?s)```json\\s*\\n(.*?)```")
	checked := 0
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range block.FindAllSubmatch(body, -1) {
			var doc any
			if json.Unmarshal(m[1], &doc) != nil {
				continue
			}
			walkDiagramPayloads(doc, func(typ string, data map[string]any) {
				checked++
				if err := CheckDataContract(typ, data); err != nil {
					t.Errorf("%s: documented %s example refused: %v", path, typ, err)
				}
			})
		}
	}
	if checked < 20 {
		t.Errorf("only %d documented svggen payloads checked; the doc scan is not finding them", checked)
	}
}

// walkDiagramPayloads calls fn for every {"type": <svggen type>, "data": {...}}
// object nested anywhere in v.
func walkDiagramPayloads(v any, fn func(string, map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		if typ, ok := x["type"].(string); ok {
			if data, ok := x["data"].(map[string]any); ok && DefaultRegistry().Get(typ) != nil {
				fn(typ, data)
			}
		}
		for _, child := range x {
			walkDiagramPayloads(child, fn)
		}
	case []any:
		for _, child := range x {
			walkDiagramPayloads(child, fn)
		}
	}
}
