package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// bmcAdvertisedKeys is the nine-key vocabulary agents are told to send: the
// RequiredFields get_diagram_capabilities advertises for business_model_canvas.
func bmcAdvertisedKeys(t *testing.T) []string {
	t.Helper()
	for _, c := range svggen.DiagramCapabilities() {
		if c.Type == "business_model_canvas" {
			if len(c.RequiredFields) != 9 {
				t.Fatalf("business_model_canvas advertises %d required fields, want 9", len(c.RequiredFields))
			}
			return c.RequiredFields
		}
	}
	t.Fatal("business_model_canvas missing from svggen capabilities")
	return nil
}

func bmcMarker(key string) string { return "Marker " + strings.ReplaceAll(key, "_", " ") }

// TestBMCAdvertisedKeysRenderOnEveryPath is the go-slide-creator-b7qqg.17
// acceptance test: every advertised section key — customer_relations
// included — reaches the PPTX through the native placeholder diagram, the
// semantic framework kind and the raw bmc-canvas pattern. customer_relations
// used to render an empty Customer Relationships cell on the native path.
func TestBMCAdvertisedKeysRenderOnEveryPath(t *testing.T) {
	keys := bmcAdvertisedKeys(t)
	const tmpl = "midnight-blue"

	t.Run("native placeholder diagram", func(t *testing.T) {
		data := map[string]any{}
		for _, k := range keys {
			data[k] = []string{bmcMarker(k)}
		}
		slide := map[string]any{
			"slide_type": "diagram",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Canvas"},
				map[string]any{"placeholder_id": "body", "type": "diagram", "diagram_value": map[string]any{
					"type": "business_model_canvas", "data": data,
				}},
			},
		}
		xml := readZipText(t, generateDeckForChromeTest(t, testTemplatesDir, tmpl, slide), "ppt/slides/slide")
		assertBMCMarkers(t, xml, keys)
	})

	t.Run("raw bmc-canvas pattern", func(t *testing.T) {
		values := map[string]any{}
		for _, k := range keys {
			values[k] = map[string]any{"header": k, "bullets": []string{bmcMarker(k)}}
		}
		slide := map[string]any{
			"slide_type": "content",
			"layout_id":  "blank-title",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Canvas"},
			},
			"pattern": map[string]any{"name": "bmc-canvas", "values": values},
		}
		xml := readZipText(t, generateDeckForChromeTest(t, testTemplatesDir, tmpl, slide), "ppt/slides/slide")
		assertBMCMarkers(t, xml, keys)
	})

	t.Run("semantic framework", func(t *testing.T) {
		var b strings.Builder
		fmt.Fprintf(&b, "meta:\n  title: Canvas\n  template: %s\nslides:\n", tmpl)
		b.WriteString("  - kind: framework\n    title: Our business model\n    framework: bmc\n    sections:\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "      %s: [\"%s\"]\n", k, bmcMarker(k))
		}
		spec := writeSpec(t, "bmc.yaml", b.String())
		out := filepath.Join(t.TempDir(), "bmc.pptx")
		stdout, err := runSemanticArgs(t, "render", "--spec", spec, "--output", out, "--templates-dir", testTemplatesDir)
		if err != nil {
			t.Fatalf("semantic render: %v\n%s", err, stdout)
		}
		assertBMCMarkers(t, readZipText(t, out, "ppt/slides/slide"), keys)
	})
}

func assertBMCMarkers(t *testing.T, xml string, keys []string) {
	t.Helper()
	for _, k := range keys {
		if !strings.Contains(xml, bmcMarker(k)) {
			t.Errorf("section %q: authored text %q is missing from the slide", k, bmcMarker(k))
		}
	}
}

// TestBMCIgnoredDataKeyIsReported pins the loss warning: a data key the native
// canvas does not read is reported by validate instead of vanishing.
func TestBMCIgnoredDataKeyIsReported(t *testing.T) {
	deck := `{"template":"midnight-blue","slides":[{"slide_type":"diagram","content":[
	{"placeholder_id":"title","type":"text","text_value":"Canvas"},
	{"placeholder_id":"body","type":"diagram","diagram_value":{"type":"business_model_canvas","data":{
	"key_partners":["a"],"key_activities":["b"],"key_resources":["c"],"value_propositions":["d"],
	"customer_relatons":["typo"],"channels":["f"],"customer_segments":["g"],"cost_structure":["h"],"revenue_streams":["i"]}}}]}]}`
	var input PresentationInput
	if err := json.Unmarshal([]byte(deck), &input); err != nil {
		t.Fatal(err)
	}
	applyDefaults(&input)
	var found bool
	for _, f := range fitFindingsForInput(&input, "midnight-blue", testTemplatesDir, false) {
		if f.Code != "diagram.data_key_ignored" {
			continue
		}
		found = true
		if !strings.HasSuffix(f.Path, "data/customer_relatons") {
			t.Errorf("path = %q, want it to end at data/customer_relatons", f.Path)
		}
		if f.Fix == nil || f.Fix.Params["did_you_mean"] == nil {
			t.Errorf("fix = %+v, want a did_you_mean suggestion", f.Fix)
		}
	}
	if !found {
		t.Fatal("validate did not report the unread customer_relatons key")
	}
}
