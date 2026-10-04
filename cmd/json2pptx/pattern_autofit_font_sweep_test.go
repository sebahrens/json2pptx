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

var runSizeRe = regexp.MustCompile(` sz="(\d+)"`)

var shapeRunTextRe = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)

// generateForSweep generates a one-deck input on tpl and returns the deck
// path.
func generateForSweep(t *testing.T, tpl string, input PresentationInput) string {
	t.Helper()
	input.Template = tpl
	input.OutputFilename = "deck.pptx"
	dir := t.TempDir()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(inputPath, resultPath, testutil.TemplatesDir(), dir, "", false, false, tpl, "off", false, "off", "free", false); err != nil {
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
	return out.OutputPath
}

// slideRunSizes returns the size (hundredths of a point) of the run that
// carries each of texts on the deck's first slide; 0 for a text not found.
func slideRunSizes(t *testing.T, pptxPath string, texts ...string) []int {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.Open("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	sizes := make([]int, len(texts))
	for i, text := range texts {
		at := strings.Index(string(data), "<a:t>"+text+"</a:t>")
		if at < 0 {
			continue
		}
		if all := runSizeRe.FindAllStringSubmatch(string(data[:at]), -1); len(all) > 0 {
			sizes[i], _ = strconv.Atoi(all[len(all)-1][1])
		}
	}
	return sizes
}

// A KPI value fitted down to the 16pt floor is written at 16pt over its 12pt
// caption — the grid's type scale used to settle it onto 14pt, the size of a
// subhead. A row with a value that does not fit one line even at the floor
// is written at 14pt, and the finding quotes both the 16pt it was held to
// and the size on the slide (go-slide-creator-a5ogo). A value with no digit is
// written at its siblings' size: the scale used to keep only digit-bearing
// figures as measured, so "Unlimited" sat at 18pt beside 21pt values
// (go-slide-creator-3hxu6).
func TestKPIValuesAreWrittenAtTheirFittedSize(t *testing.T) {
	t.Parallel()
	kpis := func(values ...string) PresentationInput {
		cells := make([]map[string]string, len(values))
		for i, v := range values {
			cells[i] = map[string]string{"big": v, "small": "Net revenue retention"}
		}
		raw, err := json.Marshal(cells)
		if err != nil {
			t.Fatal(err)
		}
		return PresentationInput{Slides: []SlideInput{titledPatternSlide(fmt.Sprintf("kpi-%dup", len(values)), raw)}}
	}

	// modern-yellow's face is the widest shipped. Six values of nine or ten
	// characters fit one line at the 16pt floor and no larger.
	a := loadTemplateAnalysis(t, "modern-yellow")
	tooLong := func(deck *PresentationInput) []string {
		var out []string
		for _, f := range collectFitFindings(deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
			if f.Code == patterns.ErrCodeBodyTooLong && strings.Contains(f.Message, "cannot fit on one line") {
				out = append(out, f.Message)
			}
		}
		return out
	}
	fitted := kpis("EUR 48.2M", "$1,234.5k", "98.7% YoY", "12,345", "USD 9.8bn", "1,234 bps")
	sizes := slideRunSizes(t, generateForSweep(t, "modern-yellow", fitted), "EUR 48.2M", "Net revenue retention")
	if sizes[0] != 1600 || sizes[1] != 1200 {
		t.Errorf("six values fitted at the floor on modern-yellow: value %d over caption %d (hundredths of a point), want 1600 over 1200", sizes[0], sizes[1])
	}
	if msgs := tooLong(&fitted); len(msgs) > 0 {
		t.Errorf("values that fit at 16pt are reported: %v", msgs)
	}

	// Six 12-character values do not fit one line at the floor: the row is
	// written at 14pt and the finding says so.
	long := kpis("EUR 1,234.5M", "$12,345,678M", "98.765% YoYs", "123,456 FTEs", "USD 98.76bns", "12,345.6 bps")
	sizes = slideRunSizes(t, generateForSweep(t, "modern-yellow", long), "EUR 1,234.5M", "Net revenue retention")
	if sizes[0] != 1400 || sizes[1] != 1200 {
		t.Errorf("six values too long for the floor on modern-yellow: value %d over caption %d (hundredths of a point), want 1400 over 1200", sizes[0], sizes[1])
	}
	msgs := tooLong(&long)
	if len(msgs) == 0 {
		t.Error("fixture: six 12-character values on modern-yellow no longer report a value that cannot fit; pick longer values")
	}
	for _, msg := range msgs {
		if !strings.Contains(msg, "at the 16pt minimum") || !strings.Contains(msg, fmt.Sprintf("written at %dpt", sizes[0]/100)) {
			t.Errorf("the finding must quote the 16pt minimum and the %dpt the value is written at: %s", sizes[0]/100, msg)
		}
	}

	mixed := kpis("Unlimited", "EUR 48.2M", "98.7% YoY", "12,345", "Not rated", "1,234 bps")
	sizes = slideRunSizes(t, generateForSweep(t, "midnight-blue", mixed), "Unlimited", "EUR 48.2M", "Not rated")
	if sizes[0] == 0 || sizes[0] != sizes[1] || sizes[2] != sizes[1] {
		t.Errorf("values with and without a digit are written at %v hundredths of a point, want one size", sizes)
	}
}

// An authored cell with no same-size sibling whose text needs a shrink is
// written at the fitted size, not with a stored fontScale LibreOffice ignores
// (go-slide-creator-217cd): the "4" of examples/sovereign-ai-strategy.json's
// axis rows.
func TestAuthoredLoneCellStoresNoAutofitShrink(t *testing.T) {
	t.Parallel()
	grid := `{"bounds":{"x":3,"y":18,"width":94,"height":76},"gap":8,"columns":[8,44,44],"rows":[
		{"height":86,"cells":[
			{"shape":{"geometry":"rect","fill":"none","text":{"content":"Low","size":13,"bold":true,"align":"ctr"}}},
			{"shape":{"geometry":"roundRect","fill":"accent1","text":{"content":"Managed risk","size":13,"align":"ctr"}}},
			{"shape":{"geometry":"roundRect","fill":"accent1","text":{"content":"Current position","size":13,"align":"ctr"}}}]},
		{"height":7,"cells":[
			{"shape":{"geometry":"rect","fill":"none","text":{"content":"4","size":24,"bold":true,"align":"ctr","vertical_align":"ctr"}}},
			{"col_span":2,"shape":{"geometry":"rect","fill":"none","text":{"content":"Data location","size":13,"bold":true,"align":"ctr"}}}]}]}`
	var sg ShapeGridInput
	if err := json.Unmarshal([]byte(grid), &sg); err != nil {
		t.Fatal(err)
	}
	title := "Who controls the data"
	input := PresentationInput{Slides: []SlideInput{{
		LayoutID:  "blank",
		Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		ShapeGrid: &sg,
	}}}
	deck := generateForSweep(t, "warm-coral", input)
	if scales, err := storedFontScales(deck); err != nil || len(scales) > 0 {
		t.Errorf("lone cell stores autofit shrink(s) %v (err %v), want its fitted size written", scales, err)
	}
	if sizes := slideRunSizes(t, deck, "4"); sizes[0] >= 2400 || sizes[0] < 1200 {
		t.Errorf("the lone 24pt figure is written at %d hundredths of a point, want it fitted under 2400", sizes[0])
	}
}
