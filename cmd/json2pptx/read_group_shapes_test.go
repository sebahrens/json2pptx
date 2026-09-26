package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptxread"
)

// go-slide-creator-s1uvj.27: pptxread decoded only p:sp and p:graphicFrame
// directly under spTree, so the text of native diagrams (swot, pestel, bmc),
// which render as p:grpSp groups, was missing from `read` / read_presentation.
func TestReadPresentation_IncludesGroupedDiagramText(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "diagrams", "swot.json"))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "swot.json")
	if err := os.WriteFile(inputPath, src, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runJSONMode(inputPath, filepath.Join(dir, "result.json"), testTemplatesDir, dir,
		"", false, false, "midnight-blue", "off", false, "off", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	pres, err := pptxread.ReadFile(filepath.Join(dir, "swot.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var all strings.Builder
	for _, s := range pres.Slides {
		for _, sh := range s.Shapes {
			all.WriteString(sh.Text + "\n")
			if sh.Bounds != nil && (sh.Bounds.Width <= 0 || sh.Bounds.Height <= 0) {
				t.Errorf("shape %q has degenerate bounds %+v", sh.Name, *sh.Bounds)
			}
		}
	}
	for _, want := range []string{"Strong brand recognition", "Limited market share in Asia", "Emerging markets expansion"} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("read output is missing SWOT text %q", want)
		}
	}
}
