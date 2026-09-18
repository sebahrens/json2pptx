package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/portabilitycheck"
)

// portabilityFixtures maps each portability fixture template (built by
// `go run ./cmd/mktemplate -portability-fixtures`) to the representative deck
// generated on it.
var portabilityFixtures = map[string]string{
	"portability-4x3":        "deck.json",
	"portability-21x9":       "deck.json",
	"portability-two-master": "deck-two-master.json",
	"portability-side-logo":  "deck.json",
}

// TestPortabilityFixtureGeometry generates a representative deck (title,
// bullets, kpi pattern, chart, two-column; footer, takeaway and source bands)
// on every portability fixture template — 4:3, 21:9, two slide masters, and a
// master logo in the right margin — and asserts the rendered geometry: every
// shape inside the canvas, the takeaway/source band on the layout's body column
// and clear of the title, footer placeholders, rendered footers and logo, and
// content inside the safe area (go-slide-creator-94sk).
func TestPortabilityFixtureGeometry(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := filepath.Join(root, "tests", "quality", "fixtures", "portability")
	templatesDir := filepath.Join(fixtures, "templates")

	for tpl, deck := range portabilityFixtures {
		t.Run(tpl, func(t *testing.T) {
			templatePath := filepath.Join(templatesDir, tpl+".pptx")
			if _, err := os.Stat(templatePath); err != nil {
				t.Fatalf("fixture template missing (run `make portability-fixtures`): %v", err)
			}
			outDir := t.TempDir()
			resultPath := filepath.Join(outDir, "result.json")
			if err := runJSONMode(filepath.Join(fixtures, deck), resultPath, templatesDir, outDir, "", false, false, tpl, "off", false, "warn", "free", false); err != nil {
				t.Fatalf("generate: %v", err)
			}
			data, err := os.ReadFile(resultPath) //nolint:gosec // test output
			if err != nil {
				t.Fatal(err)
			}
			var out JSONOutput
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatal(err)
			}
			if !out.Success {
				t.Fatalf("generation failed: %s", out.Error)
			}
			violations, err := portabilitycheck.CheckDeckPortability(out.OutputPath, templatePath)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range violations {
				t.Error(v)
			}
			slides, err := portabilitycheck.ReadDeckSlides(out.OutputPath)
			if err != nil {
				t.Fatal(err)
			}
			bands := 0
			for _, s := range slides {
				for _, sh := range s.Shapes {
					if sh.Name == "Takeaway" || sh.Name == "Source Note" {
						bands++
					}
				}
			}
			if bands == 0 {
				t.Error("no takeaway/source band rendered — the fixture deck no longer exercises the band")
			}
		})
	}
}
