package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-khzni: slides inside an appendix section show appendix page
// labels (A1, A2, …) in the slide-number position instead of continuing the
// main deck's numbering, and the main deck's numbers stay contiguous.

var footerRunText = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)

// pageLabelSequence returns, per rendered slide, what its slide-number position
// shows: the physical slide number for PowerPoint's auto field, the literal
// text otherwise, and "" for a slide with no page number.
func pageLabelSequence(t *testing.T, pptxPath string) []string {
	t.Helper()
	parts := pptxParts(t, pptxPath)
	var out []string
	for n := 1; ; n++ {
		xml, ok := parts[fmt.Sprintf("ppt/slides/slide%d.xml", n)]
		if !ok {
			break
		}
		shape := firstFooterRight(string(xml))
		switch {
		case shape == "":
			out = append(out, "")
		case strings.Contains(shape, `type="slidenum"`):
			out = append(out, strconv.Itoa(n))
		default:
			var text strings.Builder
			for _, m := range footerRunText.FindAllStringSubmatch(shape, -1) {
				text.WriteString(m[1])
			}
			out = append(out, text.String())
		}
	}
	return out
}

// firstFooterRight extracts the "Footer Right" p:sp element.
func firstFooterRight(xml string) string {
	for _, chunk := range strings.Split(xml, "<p:sp>") {
		if !strings.Contains(chunk, `name="Footer Right"`) {
			continue
		}
		if end := strings.Index(chunk, "</p:sp>"); end >= 0 {
			return chunk[:end]
		}
		return chunk
	}
	return ""
}

func appendixTitledSlide(title string) map[string]any {
	return map[string]any{
		"slide_type": "content",
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": title},
			map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{"First supporting point", "Second supporting point"}},
		},
	}
}

func appendixDivider(title string, extra map[string]any) map[string]any {
	s := map[string]any{
		"slide_type": "section",
		"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": title}},
	}
	for k, v := range extra {
		s[k] = v
	}
	return s
}

func renderRawAppendixDeck(t *testing.T, tpl string, input map[string]any) []string {
	t.Helper()
	dir := t.TempDir()
	input["template"] = tpl
	input["output_filename"] = "appendix.pptx"
	input["chrome"] = map[string]any{"page_numbers": map[string]any{"enabled": true}}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runJSONMode(inputPath, filepath.Join(dir, "result.json"), "../../templates", dir,
		"", false, false, tpl, "off", false, "off", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return pageLabelSequence(t, filepath.Join(dir, "appendix.pptx"))
}

var appendixTemplates = []string{"midnight-blue", "forest-green"}

// TestAppendixPageLabelsRawStructure: structure.sections[].appendix numbers its
// slides A1, A2 while the main deck runs 1..N, and the closing slide after the
// appendix is not renumbered by its physical position.
func TestAppendixPageLabelsRawStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("renders decks")
	}
	for _, tpl := range appendixTemplates {
		t.Run(tpl, func(t *testing.T) {
			got := renderRawAppendixDeck(t, tpl, map[string]any{
				"structure": map[string]any{
					"cover": map[string]any{"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Board update"}}},
					"sections": []any{
						map[string]any{"title": "Where we stand", "slides": []any{appendixTitledSlide("Revenue grew 12%"), appendixTitledSlide("Margin held at 18%")}},
						map[string]any{"title": "What we propose", "slides": []any{appendixTitledSlide("Expand into two regions")}},
						map[string]any{"title": "Detailed financials", "appendix": true, "slides": []any{appendixTitledSlide("P&L detail"), appendixTitledSlide("Cash flow detail")}},
					},
				},
			})
			// cover, divider, 2 slides, divider, 1 slide, appendix divider, A1, A2
			want := []string{"", "2", "3", "4", "5", "6", "", "A1", "A2"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("page labels = %q, want %q", got, want)
			}
		})
	}
}

// TestAppendixPageLabelsRawFlat covers the flat deck: a divider titled
// Appendix, and a divider carrying section_number "A" after numbered chapters,
// both start A-numbering; a main slide after the back matter continues the
// main count.
func TestAppendixPageLabelsRawFlat(t *testing.T) {
	if testing.Short() {
		t.Skip("renders decks")
	}
	cases := map[string]map[string]any{
		"titled":         appendixDivider("Appendix", nil),
		"section_number": appendixDivider("Supporting analysis", map[string]any{"section_number": "A"}),
	}
	for _, tpl := range appendixTemplates {
		for name, divider := range cases {
			t.Run(tpl+"/"+name, func(t *testing.T) {
				got := renderRawAppendixDeck(t, tpl, map[string]any{
					"slides": []any{
						map[string]any{"slide_type": "title", "content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Board update"}}},
						appendixDivider("Where we stand", nil),
						appendixTitledSlide("Revenue grew 12%"),
						appendixTitledSlide("Margin held at 18%"),
						divider,
						appendixTitledSlide("P&L detail"),
						appendixTitledSlide("Cash flow detail"),
						appendixTitledSlide("Customer survey detail"),
					},
				})
				want := []string{"", "2", "3", "4", "", "A1", "A2", "A3"}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("page labels = %q, want %q", got, want)
				}
			})
		}
	}
}

// TestAppendixPageLabelsDeckSpec renders a DeckSpec with an appendix section
// through render_deck_spec.
func TestAppendixPageLabelsDeckSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("renders decks")
	}
	for _, tpl := range appendixTemplates {
		t.Run(tpl, func(t *testing.T) {
			mc := semanticTestConfig(t)
			spec := "meta:\n  title: Board update\n  template: " + tpl + "\n" +
				"structure:\n  cover:\n    kind: title\n    title: Board update\n  sections:\n" +
				"    - title: Where we stand\n      slides:\n" +
				"        - kind: stat\n          title: Revenue grew 12% on the new regions\n          value: \"12%\"\n          label: Revenue growth year on year\n" +
				"        - kind: stat\n          title: Margin held at 18% despite input costs\n          value: \"18%\"\n          label: Operating margin\n" +
				"    - title: Detailed financials\n      appendix: true\n      slides:\n" +
				"        - kind: stat\n          title: North region drove most of the growth\n          value: \"14%\"\n          label: North region growth\n" +
				"        - kind: stat\n          title: Cash conversion stayed above plan\n          value: \"92%\"\n          label: Cash conversion\n"
			out := renderDeckSpec(t, mc, map[string]any{"spec": spec})
			got := pageLabelSequence(t, out.PptxPath)
			want := []string{"", "2", "3", "4", "", "A1", "A2"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("page labels = %q, want %q", got, want)
			}
		})
	}
}

// TestDeckPageLabelsNoBackMatter: a deck without back matter keeps the auto
// field on every slide (no literal labels, byte-identical output).
func TestDeckPageLabelsNoBackMatter(t *testing.T) {
	title := "Where we stand"
	slides := []SlideInput{
		{SlideType: "section", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}},
		{SlideType: "content"},
	}
	labels, hide, total := deckPageLabels(slides, nil)
	if labels != nil || hide != nil || total != 0 {
		t.Fatalf("deck without back matter got labels %q hide %v total %d", labels, hide, total)
	}
}

// TestAppendixDividerPrefixes pins the divider rules: a lettered chapter deck
// is not back matter, "Appendix B" numbers B1, B2.
func TestAppendixDividerPrefixes(t *testing.T) {
	div := func(title string, label string) SlideInput {
		v := title
		s := SlideInput{SlideType: "section", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &v}}}
		if label != "" {
			s.SectionNumber = &SectionNumberInput{Label: label}
		}
		return s
	}
	lettered := []SlideInput{div("Situation", "A"), {SlideType: "content"}, div("Options", "B"), {SlideType: "content"}}
	if labels, _, _ := deckPageLabels(lettered, nil); labels != nil {
		t.Errorf("a deck lettering every chapter is not an appendix, got %q", labels)
	}
	deck := []SlideInput{div("Situation", ""), {SlideType: "content"}, div("Appendix B: Methodology", ""), {SlideType: "content"}, {SlideType: "content"}}
	labels, hide, total := deckPageLabels(deck, nil)
	want := []string{"", "", "", "B1", "B2"}
	if !reflect.DeepEqual(labels, want) || !hide[2] || total != 2 {
		t.Errorf("labels %q hide %v total %d, want %q with the divider hidden and total 2", labels, hide, total, want)
	}
}
