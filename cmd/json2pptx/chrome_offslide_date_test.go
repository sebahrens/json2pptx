package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-tvy9q: a template whose date placeholder is parked outside
// the slide (hidden) beside a wide visible footer placeholder drew the footer
// line in the hidden date's 3in box, starting mid-slide, and reported
// CHROME_TRUNCATED for a line the visible footer slot holds in full.

// writeTemplateWithHiddenDate copies midnight-blue and rewrites its master's
// footer placeholders to that geometry: dt below the slide edge at mid-slide
// x, ftr spanning what stock midnight-blue's dt + ftr span together. A deck
// rendered on the copy must therefore carry stock midnight-blue's footer.
func writeTemplateWithHiddenDate(t *testing.T, dir, name string) string {
	t.Helper()
	const (
		master   = "ppt/slideMasters/slideMaster1.xml"
		stockDt  = `<a:off x="838200" y="6356350"/><a:ext cx="2743200" cy="365125"/>`
		hiddenDt = `<a:off x="5265850" y="7056786"/><a:ext cx="2743200" cy="365125"/>`
		stockFtr = `<a:off x="4038600" y="6356350"/><a:ext cx="4114800" cy="365125"/>`
		wideFtr  = `<a:off x="838200" y="6356350"/><a:ext cx="7315200" cy="365125"/>`
	)
	src, err := zip.OpenReader(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	path := filepath.Join(dir, name+".pptx")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	patched := false
	for _, f := range src.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == master {
			if bytes.Count(data, []byte(stockDt)) != 1 || bytes.Count(data, []byte(stockFtr)) != 1 {
				t.Fatalf("%s no longer holds the footer placeholders this fixture rewrites", master)
			}
			data = bytes.Replace(data, []byte(stockDt), []byte(hiddenDt), 1)
			data = bytes.Replace(data, []byte(stockFtr), []byte(wideFtr), 1)
			patched = true
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if !patched {
		t.Fatalf("midnight-blue has no %s", master)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

var footerLeftXfrm = regexp.MustCompile(`(?s)name="Footer Left".*?<a:off x="(\d+)" y="(\d+)"/>\s*<a:ext cx="(\d+)" cy="(\d+)"/>`)

// footerLeftShapes returns "slideN: x y cx cy | text" for every slide that
// carries a left footer.
func footerLeftShapes(t *testing.T, pptxPath string) []string {
	t.Helper()
	parts := pptxParts(t, pptxPath)
	var out []string
	for n := 1; ; n++ {
		xml, ok := parts[fmt.Sprintf("ppt/slides/slide%d.xml", n)]
		if !ok {
			break
		}
		m := footerLeftXfrm.FindStringSubmatch(string(xml))
		if m == nil {
			continue
		}
		out = append(out, fmt.Sprintf("slide%d: %s %s %s %s | %s", n, m[1], m[2], m[3], m[4], footerLeftLine(string(xml))))
	}
	return out
}

// renderChromeDeck validates and renders a deck on a template file and returns
// the CHROME_TRUNCATED messages of each, plus the left footer of every slide.
func renderChromeDeck(t *testing.T, templatesDir, tpl string, deck map[string]any) (predicted, rendered, footers []string) {
	t.Helper()
	data, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	tctx, err := loadPreviewTemplate(filepath.Join(templatesDir, tpl+".pptx"))
	if err != nil {
		t.Fatalf("load template: %v", err)
	}
	defer tctx.reader.Close()
	for _, f := range chromeTruncatedFindings(collectFitFindings(&input, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.theme)) {
		predicted = append(predicted, f.Message)
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(inputPath, resultPath, templatesDir, dir,
		"", false, false, tpl, "off", false, "off", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	raw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		FitFindings []patterns.FitFinding `json:"fit_findings"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	for _, f := range chromeTruncatedFindings(result.FitFindings) {
		rendered = append(rendered, f.Message)
	}
	return predicted, rendered, footerLeftShapes(t, filepath.Join(dir, "chrome.pptx"))
}

func TestChromeFooterAnchorsOnFooterSlotWhenDateIsOffSlide(t *testing.T) {
	const tpl = "hidden-date"
	templatesDir := t.TempDir()
	writeTemplateWithHiddenDate(t, templatesDir, tpl)
	stockDir := filepath.Join("..", "..", "templates")

	// A 106-character line: more than the hidden date's 3in box holds, well
	// within the 8in visible footer slot.
	fits := func(tpl string) map[string]any {
		deck := chromeTruncatedDeck(tpl)
		deck["chrome"].(map[string]any)["project_code"] = "Falcon Commercial Due Diligence"
		return deck
	}
	predicted, rendered, footers := renderChromeDeck(t, templatesDir, tpl, fits(tpl))
	if len(predicted) != 0 || len(rendered) != 0 {
		t.Errorf("a line the visible footer slot holds was reported:\nvalidate %q\nrender   %q", predicted, rendered)
	}
	if len(footers) == 0 {
		t.Fatal("no slide carries a left footer")
	}
	for _, f := range footers {
		if strings.Contains(f, ": 5265850 ") {
			t.Errorf("left footer starts at the hidden date placeholder's x: %s", f)
		}
		if !strings.Contains(f, "Project Falcon Commercial Due Diligence | Meridian Capital Partners | October 2026") || strings.Contains(f, "…") {
			t.Errorf("left footer is not the full line: %s", f)
		}
	}
	// The fixture's ftr spans stock midnight-blue's dt + ftr, so the footers
	// must be the stock template's, box for box.
	_, _, stock := renderChromeDeck(t, stockDir, "midnight-blue", fits("midnight-blue"))
	if strings.Join(footers, "\n") != strings.Join(stock, "\n") {
		t.Errorf("footers on the hidden-date template\n%s\nwant stock midnight-blue's\n%s", strings.Join(footers, "\n"), strings.Join(stock, "\n"))
	}

	// A line no slot holds: validation and render still report one identical
	// finding, measured on the footer slot.
	predicted, rendered, footers = renderChromeDeck(t, templatesDir, tpl, chromeTruncatedDeck(tpl))
	if len(predicted) == 0 || strings.Join(predicted, "\n") != strings.Join(rendered, "\n") {
		t.Fatalf("render reported\n%s\nwant what validation predicted\n%s", strings.Join(rendered, "\n"), strings.Join(predicted, "\n"))
	}
	stockPredicted, _, stock := renderChromeDeck(t, stockDir, "midnight-blue", chromeTruncatedDeck("midnight-blue"))
	if strings.Join(predicted, "\n") != strings.Join(stockPredicted, "\n") || strings.Join(footers, "\n") != strings.Join(stock, "\n") {
		t.Errorf("over-long line on the hidden-date template\n%s\n%s\nwant stock midnight-blue's\n%s\n%s",
			strings.Join(predicted, "\n"), strings.Join(footers, "\n"), strings.Join(stockPredicted, "\n"), strings.Join(stock, "\n"))
	}
}
