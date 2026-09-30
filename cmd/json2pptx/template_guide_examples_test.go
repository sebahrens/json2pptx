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
	"strings"
	"testing"
)

// TestTemplateGuideExamplesUseTypedFieldsAndRender guards the copy-ready JSON
// examples in skills/template-deck/TEMPLATE_GUIDE.md (go-slide-creator-b7qqg.7).
// The guide once taught the untyped `"value"` field, which generates a content
// slide with deprecation warnings and an empty body. Every parseable ```json
// block must therefore:
//
//   - carry no content item with a `value` key (typed *_value fields only);
//   - for complete decks and single-slide examples: parse without deprecation
//     warnings, generate, and keep every authored text_value / bullets_value
//     string on the rendered slide.
func TestTemplateGuideExamplesUseTypedFieldsAndRender(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join(projectRoot, "skills", "template-deck", "TEMPLATE_GUIDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")

	blocks := strings.Split(string(guide), "```json\n")[1:]
	parsed, rendered := 0, 0
	for i, block := range blocks {
		end := strings.Index(block, "\n```")
		if end < 0 {
			continue
		}
		snippet := block[:end]
		var doc any
		if json.Unmarshal([]byte(snippet), &doc) != nil {
			// Illustrative fragments with `...` placeholders are not JSON.
			continue
		}
		parsed++
		name := fmt.Sprintf("block%02d", i)

		for _, path := range legacyValueItems(doc, "$") {
			t.Errorf("%s: content item %s uses the deprecated untyped \"value\" field; use text_value / bullets_value / *_value", name, path)
		}

		deck := asGuideDeck(doc)
		if deck == nil {
			continue
		}
		// Examples that reference local assets (images) cannot render here.
		if strings.Contains(snippet, "assets/") {
			continue
		}
		rendered++
		t.Run(name, func(t *testing.T) {
			data, mErr := json.Marshal(deck)
			if mErr != nil {
				t.Fatal(mErr)
			}
			var input PresentationInput
			if uErr := json.Unmarshal(data, &input); uErr != nil {
				t.Fatalf("example does not decode as PresentationInput: %v", uErr)
			}
			if w := deprecationWarnings(&input); len(w) > 0 {
				t.Errorf("example raises deprecation warnings: %v", w)
			}

			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			if wErr := os.WriteFile(inputPath, data, 0o600); wErr != nil {
				t.Fatal(wErr)
			}
			tpl, _ := deck["template"].(string)
			if rErr := runJSONMode(inputPath, filepath.Join(dir, "result.json"), templatesDir, dir,
				"", false, false, tpl, "off", false, "off", "", false); rErr != nil {
				t.Fatalf("generate: %v", rErr)
			}
			slideText := guideDeckSlideText(t, filepath.Join(dir, "guide.pptx"))
			slides, _ := deck["slides"].([]any)
			if len(slideText) != len(slides) {
				t.Fatalf("rendered %d slides, example has %d", len(slideText), len(slides))
			}
			for si, s := range slides {
				for _, want := range authoredStrings(s) {
					if !strings.Contains(slideText[si], want) {
						t.Errorf("slide %d lost authored text %q (rendered: %q)", si+1, want, slideText[si])
					}
				}
			}
		})
	}
	if parsed == 0 || rendered == 0 {
		t.Fatalf("found %d parseable / %d renderable json examples in TEMPLATE_GUIDE.md; extraction broke", parsed, rendered)
	}
}

// legacyValueItems returns the JSON paths of content items (objects carrying a
// placeholder_id) that still use the untyped `value` key.
func legacyValueItems(v any, path string) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		if _, isItem := x["placeholder_id"]; isItem {
			if _, legacy := x["value"]; legacy {
				out = append(out, path)
			}
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, legacyValueItems(x[k], path+"."+k)...)
		}
	case []any:
		for i, e := range x {
			out = append(out, legacyValueItems(e, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return out
}

// asGuideDeck turns a full-deck example or a single-slide example into a
// renderable deck; content-item fragments and patch envelopes return nil.
func asGuideDeck(doc any) map[string]any {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	var deck map[string]any
	switch {
	case m["slides"] != nil:
		if _, ok := m["slides"].([]any); !ok {
			return nil
		}
		deck = map[string]any{}
		for k, v := range m {
			deck[k] = v
		}
	case m["content"] != nil && (m["layout_id"] != nil || m["slide_type"] != nil):
		deck = map[string]any{"template": "midnight-blue", "slides": []any{m}}
	default:
		return nil
	}
	if _, ok := deck["template"].(string); !ok {
		deck["template"] = "midnight-blue"
	}
	deck["output_filename"] = "guide.pptx"
	return deck
}

// authoredStrings collects text_value / bullets_value strings from a slide's
// content items.
func authoredStrings(slide any) []string {
	m, _ := slide.(map[string]any)
	items, _ := m["content"].([]any)
	var out []string
	for _, it := range items {
		ci, _ := it.(map[string]any)
		if s, ok := ci["text_value"].(string); ok {
			out = append(out, s)
		}
		if bs, ok := ci["bullets_value"].([]any); ok {
			for _, b := range bs {
				if s, ok := b.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

var guideTextRun = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)

// guideDeckSlideText returns the concatenated run text of each slide in order.
func guideDeckSlideText(t *testing.T, pptxPath string) []string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()
	slideName := regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	texts := map[int]string{}
	for _, f := range zr.File {
		sm := slideName.FindStringSubmatch(f.Name)
		if sm == nil {
			continue
		}
		var n int
		_, _ = fmt.Sscanf(sm[1], "%d", &n)
		rc, oErr := f.Open()
		if oErr != nil {
			t.Fatal(oErr)
		}
		data, rErr := io.ReadAll(rc)
		_ = rc.Close()
		if rErr != nil {
			t.Fatal(rErr)
		}
		var b strings.Builder
		for _, m := range guideTextRun.FindAllStringSubmatch(string(data), -1) {
			b.WriteString(m[1])
			b.WriteString("\n")
		}
		texts[n] = b.String()
	}
	out := make([]string, len(texts))
	for n, s := range texts {
		if n >= 1 && n <= len(out) {
			out[n-1] = s
		}
	}
	return out
}
