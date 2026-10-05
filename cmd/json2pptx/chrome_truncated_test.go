package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-m2tlt: a footer line wider than the template's footer slot
// used to be ellipsized from the right — "Confidential — Project Falcon |
// Meridian Capital…" on every slide of a proposal — with no finding and a
// score of 100. The line now gives up project_code, then footer_date, for the
// whole deck, and CHROME_TRUNCATED says so at validate and at render.

func TestChromeLineFallbacksDropProjectThenDate(t *testing.T) {
	chrome := &ChromeInput{Confidentiality: "Confidential", ProjectCode: "Falcon", ClientName: "Meridian Capital Partners", FooterDate: "October 2026"}
	cfg := chromeToFooterConfig(chrome, 3, nil)
	if cfg.LeftText != "Confidential — Project Falcon | Meridian Capital Partners | October 2026" || cfg.LeftTextPath != "/chrome" {
		t.Fatalf("full line = %q at %q", cfg.LeftText, cfg.LeftTextPath)
	}
	want := []struct {
		dropped string
		line    string
	}{
		{"project_code", "Confidential — Meridian Capital Partners | October 2026"},
		{"project_code,footer_date", "Confidential — Meridian Capital Partners"},
	}
	if len(cfg.LeftFallbacks) != len(want) {
		t.Fatalf("fallbacks = %+v, want %d", cfg.LeftFallbacks, len(want))
	}
	for i, w := range want {
		if got := strings.Join(cfg.LeftFallbacks[i].Dropped, ","); got != w.dropped || cfg.LeftFallbacks[i].LeftText != w.line {
			t.Errorf("fallback %d = %q %q, want %q %q", i, got, cfg.LeftFallbacks[i].LeftText, w.dropped, w.line)
		}
	}

	// A field the chrome does not set yields no fallback, and the line is
	// never dropped to nothing.
	if fb := chromeToFooterConfig(&ChromeInput{ClientName: "Acme"}, 3, nil).LeftFallbacks; len(fb) != 0 {
		t.Errorf("client-only chrome has nothing to drop, got %+v", fb)
	}
	if fb := chromeToFooterConfig(&ChromeInput{ProjectCode: "Falcon"}, 3, nil).LeftFallbacks; len(fb) != 0 {
		t.Errorf("a project-only line must not be dropped to nothing, got %+v", fb)
	}
	if fb := chromeToFooterConfig(&ChromeInput{ProjectCode: "Falcon", FooterDate: "May"}, 3, nil).LeftFallbacks; len(fb) != 1 || fb[0].LeftText != "May" {
		t.Errorf("project + date drops the project only, got %+v", fb)
	}
}

// The fallbacks carry what the full line carries: the legacy footer text and
// the section crumb.
func TestChromeLineFallbacksKeepLegacyTextAndCrumb(t *testing.T) {
	input := &PresentationInput{
		Footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Draft"},
		Chrome: &ChromeInput{ClientName: "Acme", ProjectCode: "Falcon", SectionCrumb: true},
		Slides: []SlideInput{{}, {SectionTitle: "Market"}},
	}
	cfg := footerConfigForInput(input, 2)
	if got := cfg.LeftTextFor(1); got != "Project Falcon | Acme | Draft | Market" {
		t.Fatalf("full crumb line = %q", got)
	}
	if len(cfg.LeftFallbacks) != 1 {
		t.Fatalf("fallbacks = %+v, want one", cfg.LeftFallbacks)
	}
	fb := cfg.LeftFallbacks[0]
	if fb.LeftText != "Acme | Draft" || len(fb.LeftTextBySlide) != 2 || fb.LeftTextBySlide[1] != "Acme | Draft | Market" {
		t.Errorf("fallback = %+v, want the legacy text and the crumb kept", fb)
	}
}

// chromeTruncatedDeck is a flat deck with a section divider (so the crumb is
// exercised) whose project code is too long for any shipped footer slot.
func chromeTruncatedDeck(tpl string) map[string]any {
	titled := func(slideType, title string) map[string]any {
		s := map[string]any{"slide_type": slideType, "content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": title},
		}}
		if slideType == "content" {
			s["content"] = append(s["content"].([]any), map[string]any{
				"placeholder_id": "body", "type": "bullets",
				"bullets_value": []any{"First supporting point of the argument", "Second supporting point of the argument", "Third supporting point of the argument"},
			})
		}
		return s
	}
	return map[string]any{
		"template":        tpl,
		"output_filename": "chrome.pptx",
		"chrome": map[string]any{
			"confidentiality": "Strictly confidential",
			"project_code":    "Falcon" + strings.Repeat(" Commercial Due Diligence", 8),
			"client_name":     "Meridian Capital Partners",
			"footer_date":     "October 2026",
			"section_crumb":   true,
			"page_numbers":    map[string]any{"enabled": true, "format": "{current} / {total}"},
		},
		"slides": []any{
			titled("title", "Project Falcon"),
			titled("content", "Nordbolt holds a defensible number six position in Europe"),
			titled("section", "Market"),
			titled("content", "The European fastener market grows four percent a year"),
		},
	}
}

func chromeTruncatedFindings(findings []patterns.FitFinding) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range findings {
		if f.Code == patterns.ErrCodeChromeTruncated {
			out = append(out, f)
		}
	}
	return out
}

// TestChromeTruncatedPreflightMatchesRender renders the deck on every template
// in templates/ (the local p-style included when present) and holds the three
// views of the footer to one another: what validation predicts, what the
// render reports, and what the slide XML shows.
func TestChromeTruncatedPreflightMatchesRender(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "templates", "*.pptx"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no templates: %v", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		tpl := strings.TrimSuffix(filepath.Base(path), ".pptx")
		t.Run(tpl, func(t *testing.T) {
			deck := chromeTruncatedDeck(tpl)
			data, err := json.Marshal(deck)
			if err != nil {
				t.Fatal(err)
			}

			// Validation: deck-level review findings, before any render — one
			// per footer slot that fell short (the section divider's slot is
			// narrower than the content slides' on some templates).
			var input PresentationInput
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			tctx, err := loadPreviewTemplate(path)
			if err != nil {
				t.Fatalf("load template: %v", err)
			}
			defer tctx.reader.Close()
			predicted := chromeTruncatedFindings(collectFitFindings(&input, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.theme))
			if len(predicted) == 0 {
				t.Fatal("validation reported no CHROME_TRUNCATED finding")
			}
			truncated := map[int]bool{} // 1-based slide -> still ellipsized
			var want []string
			for _, p := range predicted {
				if p.Path != "/chrome" || p.Action != "review" || p.Fix == nil || p.Fix.Kind != "rewrite_field" {
					t.Errorf("finding = %+v, want a review rewrite_field at /chrome", p)
				}
				dropped, _ := p.Fix.Params["dropped"].([]string)
				if len(dropped) == 0 || dropped[0] != "project_code" {
					t.Fatalf("dropped = %v, want project_code first: %s", dropped, p.Message)
				}
				slides, _ := p.Fix.Params["slides"].([]int)
				for _, n := range slides {
					if _, twice := truncated[n]; twice {
						t.Errorf("slide %d is reported by two findings", n)
					}
					truncated[n] = p.Fix.Params["truncated"] == true
				}
				want = append(want, p.Message)
			}

			// Render: the same findings, once each (the preflight copy and
			// the render copy are one finding only when they agree).
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			if err := os.WriteFile(inputPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(dir, "result.json")
			if err := runJSONMode(inputPath, resultPath, filepath.Join("..", "..", "templates"), dir,
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
			var got []string
			for _, f := range chromeTruncatedFindings(result.FitFindings) {
				got = append(got, f.Message)
			}
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("render reported\n%s\nwant what validation predicted\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}

			// Slide XML: no project code anywhere, an ellipsis only where a
			// finding says one remains, and the client name whole elsewhere.
			parts := pptxParts(t, filepath.Join(dir, "chrome.pptx"))
			footers := 0
			for n := 1; ; n++ {
				xml, ok := parts[fmt.Sprintf("ppt/slides/slide%d.xml", n)]
				if !ok {
					break
				}
				line := footerLeftLine(string(xml))
				if line == "" {
					continue // title slide: chrome skipped
				}
				footers++
				if _, reported := truncated[n]; !reported {
					t.Errorf("slide %d carries a footer no finding covers: %q", n, line)
				}
				if strings.Contains(line, "Falcon") {
					t.Errorf("slide %d footer still carries the project code: %q", n, line)
				}
				if got := strings.Contains(line, "…"); got != truncated[n] {
					t.Errorf("slide %d footer %q ellipsized = %t, finding says %t", n, line, got, truncated[n])
				}
				if !truncated[n] && !strings.Contains(line, "Meridian Capital Partners") {
					t.Errorf("slide %d footer lost the client name: %q", n, line)
				}
			}
			if footers == 0 {
				t.Fatal("no slide carries a left footer")
			}
		})
	}
}

// footerLeftLine returns the text of a slide's "Footer Left" shape, "" when it
// has none.
func footerLeftLine(xml string) string {
	for _, chunk := range strings.Split(xml, "<p:sp>") {
		if !strings.Contains(chunk, `name="Footer Left"`) {
			continue
		}
		var text strings.Builder
		for _, m := range footerRunText.FindAllStringSubmatch(chunk, -1) {
			text.WriteString(m[1])
		}
		return text.String()
	}
	return ""
}

// A footer line that fits is not reported.
func TestChromeTruncatedSilentWhenLineFits(t *testing.T) {
	path := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	deck := chromeTruncatedDeck("midnight-blue")
	deck["chrome"].(map[string]any)["project_code"] = "Falcon"
	data, _ := json.Marshal(deck)
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	tctx, err := loadPreviewTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer tctx.reader.Close()
	if got := chromeTruncatedFindings(collectFitFindings(&input, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.theme)); len(got) != 0 {
		t.Errorf("a line that fits was reported: %+v", got)
	}
}
