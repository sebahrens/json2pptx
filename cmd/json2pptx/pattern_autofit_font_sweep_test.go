package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// patternAutofitSweepTemplates are the templates whose theme body font is
// Calibri — measured by pattern sizing as its metric clone Carlito — plus the
// local p-style (Arial) when present. -short sweeps the first two: modern
// (Calibri Light headings) and midnight-blue (Calibri throughout).
var patternAutofitSweepTemplates = []string{"modern", "midnight-blue", "business-template", "forest-green", "p-style"}

var storedFontScaleRe = regexp.MustCompile(`<a:normAutofit fontScale="(\d+)"`)

// TestPatternExemplarsStoreNoAutofitShrink generates every pattern exemplar
// on the Calibri templates (and p-style) and asserts the written slide stores
// no normAutofit shrink. Pattern sizing measures the theme body font
// (Calibri as Carlito); the writer measured every body in Liberation Sans,
// which is wider, so text the pattern sized to fit was written with a stored
// shrink — or refused below the 12pt floor (go-slide-creator-ohhb2).
// Exemplars are in-budget content: none of their text should shrink.
//
// SWEEP_KEEP=<dir> keeps each generated deck under <dir> for inspection.
func TestPatternExemplarsStoreNoAutofitShrink(t *testing.T) {
	t.Parallel()
	templatesDir := testutil.TemplatesDir()
	templates := patternAutofitSweepTemplates
	if testing.Short() {
		templates = templates[:2]
	}

	var pats []patterns.Pattern
	for _, p := range patterns.Default().List() {
		if _, ok := p.(patterns.Exemplar); ok {
			pats = append(pats, p)
		}
	}
	sort.Slice(pats, func(i, j int) bool { return pats[i].Name() < pats[j].Name() })

	for _, tpl := range templates {
		if _, err := os.Stat(filepath.Join(templatesDir, tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		for _, p := range pats {
			tpl, p := tpl, p
			t.Run(tpl+"/"+p.Name(), func(t *testing.T) {
				t.Parallel()
				values, err := json.Marshal(p.(patterns.Exemplar).ExemplarValues())
				if err != nil {
					t.Fatal(err)
				}
				title := "Exemplar"
				input := PresentationInput{
					Template:       tpl,
					OutputFilename: "deck.pptx",
					Slides: []SlideInput{{
						LayoutID: "content",
						Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
						Pattern:  &PatternInput{Name: p.Name(), Values: json.RawMessage(values)},
					}},
				}
				dir := t.TempDir()
				if k := os.Getenv("SWEEP_KEEP"); k != "" {
					dir = filepath.Join(k, tpl+"_"+p.Name())
					_ = os.MkdirAll(dir, 0o755)
				}
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				inputPath := filepath.Join(dir, "input.json")
				if err := os.WriteFile(inputPath, raw, 0o644); err != nil {
					t.Fatal(err)
				}
				resultPath := filepath.Join(dir, "result.json")
				if err := runJSONMode(inputPath, resultPath, templatesDir, dir, "", false, false, tpl, "off", false, "off", "free", false); err != nil {
					t.Fatalf("generate: %v", err)
				}
				resBytes, err := os.ReadFile(resultPath)
				if err != nil {
					t.Fatal(err)
				}
				var out JSONOutput
				if err := json.Unmarshal(resBytes, &out); err != nil {
					t.Fatal(err)
				}
				if !out.Success {
					t.Fatalf("generation failed: %s", out.Error)
				}
				scales, err := storedFontScales(out.OutputPath)
				if err != nil {
					t.Fatal(err)
				}
				if len(scales) > 0 {
					t.Errorf("%s on %s stores autofit shrink(s) %s on in-budget exemplar text", p.Name(), tpl, strings.Join(scales, ", "))
				}
			})
		}
	}
}

// storedFontScales returns every stored normAutofit fontScale on the deck's
// slides, as percentages.
func storedFontScales(pptxPath string) ([]string, error) {
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var out []string
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		for _, sp := range strings.Split(string(data), "<p:sp>")[1:] {
			m := storedFontScaleRe.FindStringSubmatch(sp)
			if m == nil {
				continue
			}
			v, _ := strconv.Atoi(m[1])
			var text []string
			for _, tm := range shapeRunTextRe.FindAllStringSubmatch(sp, -1) {
				text = append(text, tm[1])
			}
			out = append(out, fmt.Sprintf("%.1f%% %q", float64(v)/1000, strings.Join(text, " ")))
		}
	}
	return out, nil
}

var shapeRunTextRe = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
