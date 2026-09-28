package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestCLIGridBodyBelowRoleFloorCannotPublish(t *testing.T) {
	dir := t.TempDir()
	input, report := filepath.Join(dir, "input.json"), filepath.Join(dir, "result.json")
	source := []byte(`{"template":"midnight-blue","design_mode":"free","viewing_mode":"present","output_filename":"deck.pptx","slides":[{"layout_id":"blank-title","title":"Readable body policy","shape_grid":{"bounds":{"x":5,"y":30,"width":90,"height":5},"columns":[100],"rows":[{"cells":[{"shape":{"geometry":"rect","text":{"content":"This complete body paragraph must remain readable when projected to the audience.\nAnother complete body paragraph needs enough room without shrinking its type size.","size":13}}}]}]}}]}`)
	if err := os.WriteFile(input, source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runJSONMode(input, report, testutil.TemplatesDir(), dir, "", false, false, "", "off", false, "strict", "", false); err == nil {
		t.Fatal("undersized panel published body text below the present-mode 12pt floor")
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var result JSONOutput
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	finding := firstFindingCode(result.FitFindings, patterns.ErrCodeTextBelowReadableMin)
	if result.Success || result.OutputPath != "" || finding == nil || finding.Action != "refuse" || finding.Fix != nil || !strings.Contains(finding.Path, "/rendered_shapes/") {
		t.Fatalf("missing generated role-floor refusal: %+v", result)
	}
	if after, err := os.ReadFile(input); err != nil || string(after) != string(source) {
		t.Fatal("source changed")
	}
}

// The lone axis numbers on examples/sovereign-ai-strategy.json slide 9 are
// marker labels, not KPIs, and must never be refused. Their 6% band is too
// short for one 24pt line plus the uniform 0.5 cm text margin, so the writer
// clamps the margin and writes the run at full size — which also keeps a real
// figure in the same band above the KPI floor.
func TestCLIGridAxisMarkerIsNotKPIButLoneFigureIs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "examples", "sovereign-ai-strategy.json"))
	if err != nil {
		t.Fatal(err)
	}
	deck := func(t *testing.T, template, marker string) []byte {
		t.Helper()
		var d map[string]any
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatal(err)
		}
		slide := d["slides"].([]any)[8].(map[string]any)
		rows := slide["shape_grid"].(map[string]any)["rows"].([]any)
		text := rows[3].(map[string]any)["cells"].([]any)[0].(map[string]any)["shape"].(map[string]any)["text"].(map[string]any)
		if text["content"] != "4" || text["size"] != float64(24) {
			t.Fatalf("sovereign axis fixture moved: %v", text)
		}
		text["content"] = marker
		d["slides"], d["template"], d["output_filename"] = []any{slide}, template, "deck.pptx"
		out, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run := func(t *testing.T, source []byte) (JSONOutput, error) {
		t.Helper()
		dir := t.TempDir()
		input, report := filepath.Join(dir, "input.json"), filepath.Join(dir, "result.json")
		if err := os.WriteFile(input, source, 0600); err != nil {
			t.Fatal(err)
		}
		runErr := runJSONMode(input, report, testutil.TemplatesDir(), dir, "", false, false, "", "off", false, "strict", "", false)
		var result JSONOutput
		out, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if after, err := os.ReadFile(input); err != nil || string(after) != string(source) {
			t.Fatal("source changed")
		}
		return result, runErr
	}
	// The marker is the slide's only 24pt run. On the deck's own template it
	// publishes; on every test template (including local p-style) no refusal
	// may name it. Other cells are judged on their own roles: on the modern
	// templates the 13pt bold column headers are independently refused.
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			result, err := run(t, deck(t, name, "4"))
			for _, f := range result.FitFindings {
				if f.Code == patterns.ErrCodeTextBelowReadableMin && strings.Contains(f.Message, "written 24.0pt") {
					t.Fatalf("axis marker refused: %s", f.Message)
				}
			}
			if name == "warm-coral" && (err != nil || !result.Success || result.OutputPath == "") {
				t.Fatalf("sovereign slide refused on its own template: %v %+v", err, result.FitFindings)
			}
		})
	}
	// The band is shorter than one 24pt line plus the uniform 0.5 cm margin,
	// so the writer clamps its vertical margin (a degenerate shape) and the
	// figure keeps its full written size: a real figure in the same band is
	// now readable at the KPI floor rather than shrunk below it.
	for _, figure := range []string{"$4.2M", "78%"} {
		result, err := run(t, deck(t, "warm-coral", figure))
		for _, f := range result.FitFindings {
			if f.Code == patterns.ErrCodeTextBelowReadableMin && strings.Contains(f.Message, "written 24.0pt") {
				t.Fatalf("lone figure %q shrunk below its floor despite the clamped margin: %v %s", figure, err, f.Message)
			}
		}
		if err != nil || !result.Success {
			t.Fatalf("lone figure %q refused: %v %+v", figure, err, result.FitFindings)
		}
	}
}
