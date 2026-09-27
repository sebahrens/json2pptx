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
