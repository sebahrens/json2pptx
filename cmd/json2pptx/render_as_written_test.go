package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// renderAsWrittenDeck exercises, on one deck, the text a renderer must not
// rewrite: leading "N. " numbers on a cover, in a shape, in body text, in
// bullets and in table cells (go-slide-creator-zdzk2), number-unit tokens in a
// title (go-slide-creator-3rg5f), and a table's highlight column
// (go-slide-creator-290o9).
const renderAsWrittenDeck = `{"template":"TEMPLATE","output_filename":"written.pptx","slides":[
 {"slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"2. Savings reach €2.2m in 6 mo"},{"placeholder_id":"subtitle","type":"text","text_value":"2. Second half outlook"}]},
 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Margin rises 3pp to €12bn"}],
  "shape_grid":{"columns":2,"rows":[{"cells":[
    {"shape":{"geometry":"rect","fill":"lt2","text":"2. Enabler bar: funded first"}},
    {"shape":{"geometry":"rect","fill":"lt2","text":"3. Third thing\n4. Fourth thing"}}]},
   {"cells":[
    {"shape":{"geometry":"rect","fill":"lt2","text":"1. One\n2. Two\n3. Three"}},
    {"shape":{"geometry":"rect","fill":"lt2","text":"Intro\n7. Seven"}}]}]}},
 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Steps continued"},{"placeholder_id":"body","type":"bullets","bullets_value":["4. Fourth step","5. Fifth step","6. Sixth step"]}]},
 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"One numbered bullet"},{"placeholder_id":"body","type":"bullets","bullets_value":["2. Enabler bar: funded first","Then the rest of the plan follows"]}]},
 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Body text"},{"placeholder_id":"body","type":"text","text_value":"2. Enabler bar: funded first"}]},
 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Table cells"},{"placeholder_id":"body","type":"table","table_value":{"headers":["Step","Owner"],"rows":[["2. Enabler bar","Ops"],["3. Next","Fin"]],"style":{"highlight_column":2}}}]}
]}`

var (
	paragraphRe  = regexp.MustCompile(`(?s)<a:p>.*?</a:p>`)
	paraTextRe   = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
	keepCaseRe   = regexp.MustCompile(`(?s)<a:rPr[^>]*\bcap="none"[^>]*>.*?<a:t>([^<]*)</a:t>|<a:rPr[^>]*\bcap="none"[^>]*/><a:t>([^<]*)</a:t>`)
	highlightRe  = regexp.MustCompile(`<a:schemeClr val="([a-z0-9]+)"><a:lumMod val="20000"/><a:lumOff val="80000"/>`)
	slideEntryRe = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
)

// slideParagraphs returns, per slide number, each paragraph's text and XML.
func slideParagraphs(t *testing.T, pptxPath string) map[int][][2]string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open %s: %v", pptxPath, err)
	}
	defer func() { _ = zr.Close() }()
	out := map[int][][2]string{}
	for _, f := range zr.File {
		m := slideEntryRe.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paragraphRe.FindAllString(string(data), -1) {
			var text strings.Builder
			for _, r := range paraTextRe.FindAllStringSubmatch(p, -1) {
				text.WriteString(r[1])
			}
			out[n] = append(out[n], [2]string{text.String(), p})
		}
	}
	return out
}

// TestNumbersUnitsAndHighlightRenderAsWritten generates the deck on every
// shipped template (and the local p-style template when present).
func TestNumbersUnitsAndHighlightRenderAsWritten(t *testing.T) {
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			if err := os.WriteFile(inputPath, []byte(strings.Replace(renderAsWrittenDeck, "TEMPLATE", name, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := runJSONMode(inputPath, filepath.Join(dir, "result.json"), testutil.TemplatesDir(), dir,
				"", false, false, name, "off", false, "off", "", false); err != nil {
				t.Fatalf("generate: %v", err)
			}
			slides := slideParagraphs(t, filepath.Join(dir, "written.pptx"))

			// find returns the XML of the one paragraph on a slide with this text.
			find := func(slide int, text string) string {
				t.Helper()
				for _, p := range slides[slide] {
					if p[0] == text {
						return p[1]
					}
				}
				var have []string
				for _, p := range slides[slide] {
					have = append(have, p[0])
				}
				t.Errorf("slide %d: no paragraph %q (have %q)", slide, text, have)
				return ""
			}
			literal := func(slide int, text string) {
				t.Helper()
				if p := find(slide, text); strings.Contains(p, "buAutoNum") {
					t.Errorf("slide %d: %q is auto-numbered, so it renders from 1: %s", slide, text, p)
				}
			}

			// zdzk2 — a number the author typed is the number that renders.
			literal(1, "2. Savings reach €2.2m in 6 mo")
			literal(1, "2. Second half outlook")
			literal(2, "2. Enabler bar: funded first")
			literal(2, "3. Third thing")
			literal(2, "4. Fourth thing")
			literal(2, "7. Seven")
			for _, text := range []string{"One", "Two", "Three"} {
				if p := find(2, text); !strings.Contains(p, `<a:buAutoNum type="arabicPeriod"/>`) {
					t.Errorf("slide 2: list item %q from a 1, 2, 3 list is not auto-numbered: %s", text, p)
				}
			}
			for _, text := range []string{"4. Fourth step", "5. Fifth step", "6. Sixth step"} {
				p := find(3, text)
				if strings.Contains(p, "buAutoNum") || strings.Contains(p, "buChar") || !strings.Contains(p, "<a:buNone/>") {
					t.Errorf("slide 3: %q should carry its typed number as the only marker: %s", text, p)
				}
			}
			literal(4, "2. Enabler bar: funded first")
			literal(5, "2. Enabler bar: funded first")
			literal(6, "2. Enabler bar")
			literal(6, "3. Next")

			// 3rg5f — number-unit tokens keep their case on all-caps titles.
			for slide, units := range map[int][]string{1: {"€2.2m", "6 mo"}, 2: {"3pp", "€12bn"}} {
				var title string
				for _, p := range slides[slide] {
					if strings.Contains(p[0], units[0]) {
						title = p[1]
					}
				}
				kept := map[string]bool{}
				for _, m := range keepCaseRe.FindAllStringSubmatch(title, -1) {
					kept[m[1]+m[2]] = true
				}
				for _, u := range units {
					if !kept[u] {
						t.Errorf("slide %d title: %q is not in a cap=\"none\" run: %s", slide, u, title)
					}
				}
			}

			// 290o9 — the highlight column tints the template's primary fill.
			reader, err := template.OpenTemplate(filepath.Join(testutil.TemplatesDir(), name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			want := patterns.PrimaryFill(template.ParseTheme(reader).Colors)
			raw := readZipText(t, filepath.Join(dir, "written.pptx"), "ppt/slides/slide6.xml")
			slots := map[string]bool{}
			for _, m := range highlightRe.FindAllStringSubmatch(raw, -1) {
				slots[m[1]] = true
			}
			if len(slots) != 1 || !slots[want] {
				t.Errorf("highlight column tints %v, want the template's primary fill %q", slots, want)
			}
		})
	}
}
