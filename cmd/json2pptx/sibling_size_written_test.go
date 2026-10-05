package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/svggen/safeyaml"
)

// SIBLING_SIZE_MISMATCH and the slide XML (go-slide-creator-bhoo3).
//
// The finding said a comparison cell "is shrunk to about 16.2pt" while the
// slide carried it at 18pt with no stored scale and LibreOffice drew the row
// at one size: the 16.2pt was what the check expects of a renderer whose font
// runs wider, stated as what the file holds. Each cell of the finding now
// names the size that is written (written_pt) beside the one expected of a
// renderer (rendered_pt), and these tests read the generated deck to hold
// written_pt to the XML.

// siblingSizeTemplates are the templates the example decks are swept on:
// two shipped ones whose faces differ in width, and the local p-style when it
// is present.
var siblingSizeTemplates = []string{"midnight-blue", "modern-template", "p-style"}

// exampleDeckForFindings loads an example deck the way the CLI does for a
// -json file, set on tpl. A semantic example (.yaml) is compiled for tpl
// first, as `semantic compile --template` does.
func exampleDeckForFindings(t *testing.T, path, tpl string) PresentationInput {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if strings.HasSuffix(path, ".yaml") {
		var doc map[string]any
		if err := safeyaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		asJSON, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		spec, diags := semantic.ParseJSON(asJSON)
		if spec == nil {
			t.Fatalf("%s: does not parse: %v", path, diags)
		}
		spec.Meta.Template = tpl
		compiled, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		input = *compiled
	} else if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	input.Template = tpl
	input.OutputFilename = "deck.pptx"
	resolveLocalAssetPaths(input.Slides, filepath.Dir(path))
	applyDefaults(&input)
	return input
}

// siblingSizeFindingsOf returns the deck's SIBLING_SIZE_MISMATCH findings on
// its template.
func siblingSizeFindingsOf(t *testing.T, input *PresentationInput) []patterns.FitFinding {
	t.Helper()
	layouts, theme, w, h := fitReportGeometry(input.Template, testutil.TemplatesDir())
	if layouts == nil || theme == nil {
		t.Fatalf("cannot analyze template %q", input.Template)
	}
	resolveCanonicalLayoutIDs(input.Slides, layouts)
	effective := *theme
	if input.ThemeOverride != nil {
		effective, _ = effective.ApplyOverride(input.ThemeOverride.ToThemeOverride())
	}
	var out []patterns.FitFinding
	for _, f := range collectFitFindings(input, layouts, w, h, &effective) {
		if f.Code == patterns.ErrCodeSiblingSizeMismatch {
			out = append(out, f)
		}
	}
	return out
}

var (
	siblingSlidePathRe = regexp.MustCompile(`^/slides/(\d+)/`)
	siblingShapeRe     = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	siblingScaleRe     = regexp.MustCompile(`<a:normAutofit[^>]*fontScale="(\d+)"`)
)

// writtenShapeSizePt reads the generated deck: the size (points) slide
// slideNo carries for the shape whose text starts with prefix — the size of
// its first sized run times the autofit scale stored on the shape.
func writtenShapeSizePt(t *testing.T, pptxPath string, slideNo int, prefix string) (float64, bool) {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.Open(fmt.Sprintf("ppt/slides/slide%d.xml", slideNo))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range siblingShapeRe.FindAllString(string(data), -1) {
		var text []string
		for _, tm := range shapeRunTextRe.FindAllStringSubmatch(sp, -1) {
			text = append(text, tm[1])
		}
		if !strings.HasPrefix(strings.Join(text, ""), prefix) {
			continue
		}
		at := strings.Index(sp, "<a:r>")
		if at < 0 {
			continue
		}
		m := runSizeRe.FindStringSubmatch(sp[at:])
		if m == nil {
			continue
		}
		sz, _ := strconv.Atoi(m[1])
		size := float64(sz) / 100
		if s := siblingScaleRe.FindStringSubmatch(sp); s != nil {
			scale, _ := strconv.Atoi(s[1])
			size *= float64(scale) / 100000
		}
		return size, true
	}
	return 0, false
}

// assertSiblingFindingMatchesXML generates the deck and holds every cell of
// every finding to the slide XML: written_pt is the size the shape carries,
// and a message that states a shrink as done ("is shrunk to") needs a cell
// the file really holds smaller than its authored size.
func assertSiblingFindingMatchesXML(t *testing.T, input PresentationInput, findings []patterns.FitFinding) {
	t.Helper()
	result, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
		AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, f := range findings {
		m := siblingSlidePathRe.FindStringSubmatch(f.Path)
		if m == nil {
			t.Fatalf("finding path %q names no slide", f.Path)
		}
		idx, _ := strconv.Atoi(m[1])
		if f.Fix == nil {
			t.Fatalf("finding carries no fix: %s", f.Message)
		}
		cells, _ := f.Fix.Params["cells"].([]any)
		if len(cells) < 2 {
			t.Fatalf("finding names %d cells: %s", len(cells), f.Message)
		}
		authored, _ := f.Fix.Params["authored_pt"].(float64)
		storedShrink := false
		for _, c := range cells {
			cell, _ := c.(map[string]any)
			label, _ := cell["text"].(string)
			written, ok := cell["written_pt"].(float64)
			if !ok {
				t.Fatalf("cell %q carries no written_pt: %v", label, cell)
			}
			inXML, found := writtenShapeSizePt(t, result.OutputPath, idx+1, strings.TrimSuffix(label, "…"))
			if !found {
				t.Fatalf("slide %d has no shape starting %q", idx+1, label)
			}
			if math.Abs(written-inXML) > 0.06 {
				t.Errorf("cell %q: the finding says it is written at %.1fpt, the slide XML carries %.2fpt", label, written, inXML)
			}
			rendered, _ := cell["rendered_pt"].(float64)
			if rendered > written+0.06 {
				t.Errorf("cell %q: expected to render at %.1fpt, above the %.1fpt it is written at", label, rendered, written)
			}
			if inXML < authored-0.06 {
				storedShrink = true
			}
		}
		if strings.Contains(f.Message, "is shrunk to") && !storedShrink {
			t.Errorf("the message states a shrink the slide XML does not hold (every cell is written at %.1fpt): %s", authored, f.Message)
		}
		if !storedShrink && !strings.Contains(f.Message, "is written at") {
			t.Errorf("a shrink expected of the renderer must name the size that is written: %s", f.Message)
		}
	}
}

// siblingRefitProbe is a raw grid of two labels authored alike at 18pt in a
// row that holds one line: a short one and one that fills its box.
func siblingRefitProbe(long string) PresentationInput {
	title := "Sibling size probe"
	cell := func(text string) string {
		raw, _ := json.Marshal(map[string]any{"content": text, "size": 18, "align": "l"})
		return fmt.Sprintf(`{"shape":{"geometry":"rect","fill":"none","text":%s}}`, raw)
	}
	grid := fmt.Sprintf(`{"columns":2,"vertical_align":"top","rows":[{"min_height":66,"max_height":66,"cells":[%s,%s]}]}`,
		cell("Short label"), cell(long))
	return PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID:  "content",
			Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
			ShapeGrid: mustGridInput(grid),
		}},
	}
}

func mustGridInput(raw string) *ShapeGridInput {
	var g ShapeGridInput
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		panic(err)
	}
	return &g
}

// The finding's written_pt is the size in the slide XML. The probe grows a
// label a letter at a time until it sits on one line within the render slack of
// its box: the file carries it at the authored 18pt, so the finding must say
// "written at 18.0pt" and keep its 16-odd points for what a renderer may do.
func TestSiblingSizeFindingReportsTheWrittenSize(t *testing.T) {
	for n := 0; n <= 80; n++ {
		input := siblingRefitProbe("Always-on client portal with live " + strings.Repeat("i", n))
		applyDefaults(&input)
		findings := siblingSizeFindingsOf(t, &input)
		if len(findings) == 0 {
			continue
		}
		assertSiblingFindingMatchesXML(t, input, findings)
		cells, _ := findings[0].Fix.Params["cells"].([]any)
		first, _ := cells[0].(map[string]any)
		if first["written_pt"] != 18.0 || first["shrunk"] != true {
			t.Errorf("the label that fills its box is written at 18pt and expected to be re-fitted; got %v", first)
		}
		return
	}
	t.Fatal("no label length put the probe within the render slack: the probe no longer exercises the check")
}

// No shipped example — raw deck, diagram deck or semantic spec — carries a
// row at two sizes on these templates, and where a finding is raised its
// sizes are the file's. Four slides reported one:
//
//   - examples/comparison-2col-connectors slide 2 (midnight-blue) and the
//     deals playbook's option cards (modern-template): the sparse block's
//     type step set a label at 18pt on one line of a row one line tall,
//     within 12% of the box, where a renderer's wider face wraps and shrinks
//     it. A stepped label that close to its box now gets a row with room for
//     its second line (shapegrid stepFit), or the block keeps its type.
//   - the risk playbook's six-row comparison (modern-template): the rows did
//     not fit, the default 14pt was kept and the writer shrank one row to
//     13.4pt. The comparison takes the first step at which every row is
//     written whole (12pt here).
//   - the risk playbook's next steps (modern-template): "Executive
//     committee" filled the 22% owner column; the column now holds its
//     longest label with the render slack to spare.
func TestExamplesRowsReadAtOneSizeAcrossTemplates(t *testing.T) {
	decks, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "examples", "*.json"))
	if err != nil || len(decks) == 0 {
		t.Fatalf("find example decks: %v (%d files)", err, len(decks))
	}
	for _, glob := range []string{
		filepath.Join("diagrams", "*.json"),
		filepath.Join("semantic", "*.yaml"),
		filepath.Join("semantic", "playbooks", "*.yaml"),
	} {
		more, _ := filepath.Glob(filepath.Join(testutil.RepoRoot(), "examples", glob))
		for _, deck := range more {
			// examples/semantic/invalid.yaml is the spec that must not parse.
			if filepath.Base(deck) != "invalid.yaml" {
				decks = append(decks, deck)
			}
		}
	}
	templates := siblingSizeTemplates
	if testing.Short() {
		decks = []string{filepath.Join(testutil.RepoRoot(), "examples", "comparison-2col-connectors.json")}
		templates = templates[:1]
	}
	for _, tpl := range templates {
		if _, err := os.Stat(filepath.Join(testutil.TemplatesDir(), tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		for _, deck := range decks {
			t.Run(strings.TrimSuffix(filepath.Base(deck), filepath.Ext(deck))+"/"+tpl, func(t *testing.T) {
				input := exampleDeckForFindings(t, deck, tpl)
				findings := siblingSizeFindingsOf(t, &input)
				for _, f := range findings {
					t.Errorf("%s: %s", f.Path, f.Message)
				}
				if len(findings) > 0 {
					assertSiblingFindingMatchesXML(t, exampleDeckForFindings(t, deck, tpl), findings)
				}
			})
		}
	}
}

// comparison-2col sizes a row for the wrap a renderer may add
// (go-slide-creator-bhoo3). A dense comparison is not stepped and its rows
// are content-sized: a cell that fills its box on one line sat in a row one
// line tall. The sweep grows one cell a letter at a time through the window
// in which it is one line here and two in a wider face; the row holds the
// second line throughout, so the comparison reads at one size.
func TestComparisonRowsHoldARendererWrap(t *testing.T) {
	for _, connectors := range []bool{false, true} {
		reported := 0
		for n := 0; n <= 90; n += 2 {
			rows := []map[string]string{}
			for i := 0; i < 6; i++ {
				rows = append(rows, map[string]string{"left": "Quarterly reporting packs", "right": "Reactive issue handling"})
			}
			rows[2]["right"] = "Always-on client portal with live " + strings.Repeat("i", n)
			values, _ := json.Marshal(map[string]any{"headers": []string{"Today", "Tomorrow"}, "rows": rows})
			slide := titledPatternSlide("comparison-2col", values)
			if connectors {
				slide.Pattern.Overrides = json.RawMessage(`{"connectors":true}`)
			}
			input := PresentationInput{Template: "midnight-blue", Slides: []SlideInput{slide}}
			applyDefaults(&input)
			for _, f := range siblingSizeFindingsOf(t, &input) {
				reported++
				if reported <= 2 {
					t.Errorf("connectors=%t n=%d: %s", connectors, n, f.Message)
				}
			}
		}
		if reported > 2 {
			t.Errorf("connectors=%t: %d label lengths in all leave a row at two sizes", connectors, reported)
		}
	}
}

// A comparison whose rows fit at no step is written at one size
// (go-slide-creator-bhoo3). Six rows under headers do not fit
// modern-template's content area at full margins; the default 14pt was kept
// and the writer shrank the one crowded row to 13.4pt beside five at 14pt.
// The comparison now takes the first step every cell is written whole at.
func TestOverflowingComparisonIsWrittenAtOneSize(t *testing.T) {
	rows := []map[string]string{
		{"left": "1st line: 1,200 control owners self-assess their controls", "right": "1st line: 1,200 owners trained, controls tested quarterly"},
		{"left": "2nd line: Risk & Compliance, 28 FTE", "right": "2nd line: Risk & Compliance, 44 FTE (+16)"},
		{"left": "3rd line: Internal Audit, 12 FTE", "right": "3rd line: Internal Audit, 12 FTE, assures the new framework"},
		{"left": "Weakness: risk appetite not cascaded below the board", "right": "Appetite cascaded to business-unit limits and KRIs"},
		{"left": "Weakness: 2nd line too thin to challenge the business", "right": "2nd line resourced to 44 FTE by March 2027"},
		{"left": "Weakness: no integrated risk reporting", "right": "One integrated risk report to the board monthly"},
	}
	values, _ := json.Marshal(map[string]any{"headers": []string{"Today", "Target (June 2027)"}, "rows": rows})
	slide := titledPatternSlide("comparison-2col", values)
	takeaway := "Same three lines; the 2nd line grows and the reporting becomes one view."
	slide.Takeaway = takeaway
	deck := generateForSweep(t, "modern-template", PresentationInput{Slides: []SlideInput{slide}})
	if scales, err := storedFontScales(deck); err != nil || len(scales) > 0 {
		t.Errorf("stored autofit shrinks: %v (%v)", scales, err)
	}
	var texts []string
	for _, r := range rows {
		texts = append(texts, strings.ReplaceAll(r["left"], "&", "&amp;"), strings.ReplaceAll(r["right"], "&", "&amp;"))
	}
	sizes := slideRunSizes(t, deck, texts...)
	for i, sz := range sizes {
		if sz == 0 {
			t.Fatalf("no run carries %q", texts[i])
		}
		if sz != sizes[0] {
			t.Errorf("%q is written at %.2fpt, %q at %.2fpt: the comparison reads at two sizes", texts[i], float64(sz)/100, texts[0], float64(sizes[0])/100)
		}
	}
}

// `score` resolves a deck's relative assets beside the deck, as generate and
// validate do. It resolved them against the working directory, so
// examples/exhibit-callouts.json (images/ops-console.png) scored only from
// inside examples/ and was refused with INPUT.IMAGE_PATH everywhere else —
// the sweep of the examples could not score it.
func TestCLIScoreResolvesAssetsBesideTheDeck(t *testing.T) {
	deck := filepath.Join(testutil.RepoRoot(), "examples", "exhibit-callouts.json")
	stdout, stderr, code := cliRun(t, nil, "score", "--json", deck, "--templates-dir", testutil.TemplatesDir())
	var scored struct {
		OverallScore *float64 `json:"overall_score"`
	}
	decodeOneJSON(t, stdout, &scored)
	if code != 0 || scored.OverallScore == nil {
		t.Fatalf("score %s: exit=%d stderr=%q stdout=%.300s", deck, code, stderr, stdout)
	}
}
