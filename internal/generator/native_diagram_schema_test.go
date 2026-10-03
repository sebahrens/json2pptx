package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func hdx2lSpec(t *testing.T, raw string) *types.DiagramSpec {
	t.Helper()
	var spec types.DiagramSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return &spec
}

// TestNativeDiagramDataErrors_EvidenceFixtures pins the go-slide-creator-hdx2l
// fixtures: the wrong keys are refused at their exact field, once per list,
// with the key the builder reads as the suggestion.
func TestNativeDiagramDataErrors_EvidenceFixtures(t *testing.T) {
	errs := NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "pyramid", "data": {"levels": [{"title": "Strategy"}, {"title": "Delivery"}, {"title": "Operations"}]}}`))
	if len(errs) != 1 {
		t.Fatalf("pyramid: got %d errors, want 1 (deduplicated per list): %v", len(errs), errs)
	}
	e := errs[0]
	if e.Field != "data.levels[0].title" || e.Key != "title" || e.DidYouMean != "label" || e.Occurrences != 3 {
		t.Errorf("pyramid error = %+v", e)
	}
	if strings.Join(e.Expected, ",") != "description,label" {
		t.Errorf("pyramid expected keys = %v", e.Expected)
	}

	errs = NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "house_diagram", "data": {"roof": "Vision", "sections": [{"title": "Growth", "bullets": ["Scale"]}], "foundation": "People"}}`))
	got := map[string]string{}
	for _, e := range errs {
		got[e.Field] = e.DidYouMean
	}
	want := map[string]string{"data.sections[0].title": "label", "data.sections[0].bullets": "items"}
	if len(got) != len(want) {
		t.Fatalf("house_diagram errors = %v, want %v", errs, want)
	}
	for f, s := range want {
		if got[f] != s {
			t.Errorf("house_diagram %s: did-you-mean %q, want %q", f, got[f], s)
		}
	}
}

// TestNativeDiagramDataErrors_TopLevelAndEmptyLabels covers a misspelled
// top-level key and a payload that passes the key check but draws no text.
func TestNativeDiagramDataErrors_TopLevelAndEmptyLabels(t *testing.T) {
	errs := NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "pyramid", "data": {"tiers": ["a", "b"]}}`))
	if len(errs) != 1 || errs[0].Key != "tiers" || errs[0].DidYouMean != "levels" {
		t.Errorf("tiers: %v", errs)
	}
	errs = NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "pyramid", "data": {"levels": [{}, {"description": ""}]}}`))
	if len(errs) != 1 || errs[0].Key != "" || !strings.Contains(errs[0].Error(), "every label and body") {
		t.Errorf("empty labels: %v", errs)
	}
	errs = NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "house_diagram", "data": {}}`))
	if len(errs) != 1 || errs[0].Key != "" {
		t.Errorf("empty house: %v", errs)
	}
	// Built-in titles always draw: an empty SWOT is not "all labels empty".
	if errs := NativeDiagramDataErrors(hdx2lSpec(t, `{"type": "swot", "data": {}}`)); len(errs) != 0 {
		t.Errorf("empty swot: %v", errs)
	}
	// Non-native types and business_model_canvas are out of scope.
	for _, raw := range []string{
		`{"type": "timeline", "data": {"whatever": 1}}`,
		`{"type": "business_model_canvas", "data": {"not_a_section": ["x"]}}`,
	} {
		if errs := NativeDiagramDataErrors(hdx2lSpec(t, raw)); len(errs) != 0 {
			t.Errorf("%s: %v", raw, errs)
		}
	}
}

// TestNativeDataSchemas_CoverEveryNativeType: every native type except
// business_model_canvas (whose unread keys are the review finding
// diagram.data_key_ignored) has a data contract, so a new native type cannot
// silently skip this check.
func TestNativeDataSchemas_CoverEveryNativeType(t *testing.T) {
	for _, typ := range NativeDiagramTypeNames() {
		if typ == "business_model_canvas" {
			continue
		}
		if _, ok := nativeDataSchemas[typ]; !ok {
			t.Errorf("native type %q has no data schema in nativeDataSchemas", typ)
		}
	}
	for typ := range nativeDataSchemas {
		if !IsNativeDiagramType(&types.DiagramSpec{Type: typ}) {
			t.Errorf("nativeDataSchemas has %q, which is not a native type", typ)
		}
	}
}

// acceptsKeyAnywhere reports whether key is a field or tolerated key at any
// level of shape.
func acceptsKeyAnywhere(shape *nativeDataShape, key string, seen map[*nativeDataShape]bool) bool {
	if shape == nil || seen[shape] {
		return false
	}
	seen[shape] = true
	if _, ok := shape.fields[key]; ok || shape.tolerated[key] {
		return true
	}
	for _, child := range shape.fields {
		if acceptsKeyAnywhere(child, key, seen) {
			return true
		}
	}
	return false
}

// TestNativeDataSchemas_AcceptCapabilityFields: get_diagram_capabilities
// tells agents which fields a type takes. The data contract must never refuse
// one of them, or an agent following the capability metadata is rejected.
func TestNativeDataSchemas_AcceptCapabilityFields(t *testing.T) {
	for _, c := range svggen.DiagramCapabilities() {
		shape, ok := nativeDataSchemas[c.Type]
		if !ok {
			continue
		}
		for _, f := range append(append([]string{}, c.RequiredFields...), c.OptionalFields...) {
			if !acceptsKeyAnywhere(shape, f, map[*nativeDataShape]bool{}) {
				t.Errorf("%s: capability field %q is refused by the native data schema", c.Type, f)
			}
		}
	}
}

var docJSONBlockRE = regexp.MustCompile("(?s)```json\\s*\\n(.*?)```")

// TestNativeDataSchemas_AcceptDocumentedExamples: every JSON example in
// docs/diagrams/ for a native type passes the data contract — the docs are
// what agents copy.
func TestNativeDataSchemas_AcceptDocumentedExamples(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "docs", "diagrams", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no diagram docs found: %v", err)
	}
	checked := 0
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range docJSONBlockRE.FindAllSubmatch(body, -1) {
			var spec types.DiagramSpec
			if json.Unmarshal(m[1], &spec) != nil || spec.Data == nil {
				continue
			}
			if _, ok := nativeDataSchemas[spec.Type]; !ok {
				continue
			}
			checked++
			for _, e := range NativeDiagramDataErrors(&spec) {
				t.Errorf("%s: documented %s example refused: %v", filepath.Base(path), spec.Type, e)
			}
		}
	}
	if checked < 10 {
		t.Errorf("only %d native examples checked; the doc scan is not finding them", checked)
	}
}

// TestGenerateGridNativeDiagram_RefusesWrongDataKeys: the region placement
// path refuses before laying anything out.
func TestGenerateGridNativeDiagram_RefusesWrongDataKeys(t *testing.T) {
	spec := hdx2lSpec(t, `{"type": "pyramid", "data": {"levels": [{"title": "Strategy"}, {"title": "Delivery"}]}}`)
	region := NativeDiagramRegion{Bounds: types.BoundingBox{Width: 6 * 914400, Height: 4 * 914400}}
	_, err := GenerateGridNativeDiagram(spec, region, "", func(int) uint32 { return 100 })
	if err == nil || !strings.Contains(err.Error(), `did you mean "label"`) {
		t.Fatalf("err = %v, want unknown-key refusal", err)
	}
}
