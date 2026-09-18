package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chartOverCapSpec is the go-slide-creator-j02x repro: a chart_insight with a
// renderable chart and 8 insights (over the chart-insights-split cap of 6).
// Before the fix the density fallback compiled to a one-content layout with the
// insights in body_2, which resolved onto body and was buried by the chart —
// 0 of 8 insights rendered while the render still exited 0.
func chartOverCapSpec(template string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "meta:\n  title: Stress\n  template: %s\nslides:\n", template)
	b.WriteString("  - kind: chart_insight\n    title: Revenue Trajectory\n    takeaway: Revenue grew every quarter.\n")
	b.WriteString("    chart:\n      type: bar_chart\n      title: Quarterly revenue\n      data:\n")
	b.WriteString("        categories: [\"Q1\", \"Q2\", \"Q3\", \"Q4\"]\n        series:\n          - name: Revenue\n            values: [34, 40, 44, 48]\n")
	b.WriteString("    insights:\n")
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&b, "      - Insight marker %02d holds\n", i)
	}
	return b.String()
}

// readZipText returns the concatenated text of every zip entry whose name has
// the given prefix.
func readZipText(t *testing.T, path, prefix string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	var sb strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		sb.Write(data)
	}
	return sb.String()
}

func TestSemanticRender_ChartInsightOverCapKeepsAllInsights(t *testing.T) {
	for _, tmpl := range []string{"midnight-blue", "warm-coral"} {
		t.Run(tmpl, func(t *testing.T) {
			spec := writeSpec(t, "stress.yaml", chartOverCapSpec(tmpl))
			out := filepath.Join(t.TempDir(), "stress.pptx")
			stdout, err := runSemanticArgs(t, "render", "--spec", spec, "--output", out, "--templates-dir", testTemplatesDir)
			if err != nil {
				t.Fatalf("semantic render returned error: %v\noutput=%s", err, stdout)
			}
			var res semanticRenderResult
			if jerr := json.Unmarshal([]byte(stdout), &res); jerr != nil {
				t.Fatalf("render output is not a semanticRenderResult: %v", jerr)
			}
			if !res.OK {
				t.Fatalf("render OK=false: %s", res.Error)
			}
			slideXML := readZipText(t, out, "ppt/slides/slide")
			for i := 1; i <= 8; i++ {
				want := fmt.Sprintf("Insight marker %02d holds", i)
				if !strings.Contains(slideXML, want) {
					t.Errorf("rendered slide XML is missing insight %q", want)
				}
			}
			// The chart must survive alongside the insights.
			media := readZipText(t, out, "ppt/slides/_rels/")
			if !strings.Contains(media, "media/") {
				t.Error("rendered slide carries no chart media relationship")
			}
		})
	}
}

// TestGenerate_DuplicatePlaceholderResolutionErrors pins the generator-side
// guard: two content items that fall back onto one physical placeholder must
// fail generation instead of silently burying one of them.
func TestGenerate_DuplicatePlaceholderResolutionErrors(t *testing.T) {
	templatesDir, err := filepath.Abs(testTemplatesDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	deck := `{
  "template": "midnight-blue",
  "slides": [{
    "slide_type": "content",
    "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Collision"},
      {"placeholder_id": "body_2", "type": "bullets", "bullets_value": ["alpha", "bravo"]},
      {"placeholder_id": "body", "type": "diagram", "diagram_value": {"type": "bar_chart", "data": {"categories": ["A", "B"], "series": [{"name": "S", "values": [1, 2]}]}}}
    ]
  }]
}`
	inputPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(inputPath, []byte(deck), 0o600); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "result.json")
	err = runJSONMode(inputPath, resultPath, templatesDir, dir, "", false, false, "midnight-blue", "off", false, "off", "free", false)
	if err == nil {
		t.Fatal("expected generation to fail when body_2 and body resolve to the same placeholder")
	}
	if !strings.Contains(err.Error(), "both resolve to placeholder") {
		t.Errorf("error does not describe the placeholder collision: %v", err)
	}
}
