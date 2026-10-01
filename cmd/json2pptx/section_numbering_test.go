package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

func sectionSlide(title string, override *SectionNumberInput) SlideInput {
	s := trackerTitled("section", "section", title)
	s.SectionNumber = override
	return s
}

// TestDeckSectionNumbersBackMatterAndOverrides pins go-slide-creator-7ldh9:
// Appendix / Backup / Q&A dividers are neither numbered nor counted, and
// section_number: false | "<label>" overrides the automatic number.
func TestDeckSectionNumbersBackMatterAndOverrides(t *testing.T) {
	slides := []SlideInput{
		trackerTitled("title", "title", "Cover"),
		sectionSlide("Market context", nil),
		trackerTitled("content", "content", "Demand is shifting"),
		sectionSlide("Interlude", &SectionNumberInput{Suppress: true}),
		sectionSlide("Where we win", nil),
		sectionSlide("Deep dive", &SectionNumberInput{Label: "05"}),
		sectionSlide("Plan", nil),
		sectionSlide("Appendix", nil),
		sectionSlide("Detailed financials", &SectionNumberInput{Label: "A"}),
		sectionSlide("Q&A", nil),
		sectionSlide("Thank you", &SectionNumberInput{}), // true: default behaviour
	}
	labels, ordinals := deckSectionNumbers(slides, nil)
	wantLabels := []string{"", "01", "", "", "02", "05", "06", "", "A", "", ""}
	wantOrdinals := []int{0, 1, 0, 0, 2, 5, 6, 0, 0, 0, 0}
	for i := range slides {
		if labels[i] != wantLabels[i] || ordinals[i] != wantOrdinals[i] {
			t.Errorf("slide %d: label %q ordinal %d, want %q / %d", i, labels[i], ordinals[i], wantLabels[i], wantOrdinals[i])
		}
	}
}

func TestSectionNumberInputJSON(t *testing.T) {
	cases := []struct {
		in      string
		want    SectionNumberInput
		wantErr bool
	}{
		{`false`, SectionNumberInput{Suppress: true}, false},
		{`true`, SectionNumberInput{}, false},
		{`"A"`, SectionNumberInput{Label: "A"}, false},
		{`"02"`, SectionNumberInput{Label: "02"}, false},
		{`""`, SectionNumberInput{}, true},
		{`"Appendix"`, SectionNumberInput{}, true},
		{`3`, SectionNumberInput{}, true},
	}
	for _, c := range cases {
		var slide SlideInput
		err := json.Unmarshal([]byte(`{"slide_type":"section","content":[],"section_number":`+c.in+`}`), &slide)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if slide.SectionNumber == nil || *slide.SectionNumber != c.want {
			t.Errorf("%s: got %+v, want %+v", c.in, slide.SectionNumber, c.want)
		}
		out, _ := json.Marshal(slide.SectionNumber)
		if c.in != "true" && string(out) != c.in {
			t.Errorf("%s: round-trips to %s", c.in, out)
		}
	}
}

// TestSectionNumberSequenceSkipsUnnumbered: an authored "02" on the divider
// after an Appendix is right, because the appendix is not counted.
func TestSectionNumberSequenceSkipsUnnumbered(t *testing.T) {
	two := "02"
	second := sectionSlide("Plan", nil)
	second.Content = append(second.Content, ContentInput{PlaceholderID: "body", Type: "text", TextValue: &two})
	input := &PresentationInput{Slides: []SlideInput{
		sectionSlide("Market", nil),
		sectionSlide("Appendix", nil),
		second,
	}}
	if f := collectSectionNumberSequenceFindings(input, nil); len(f) != 0 {
		t.Errorf("unexpected findings: %+v", f)
	}
}

// TestExpandStructureAppendixSection pins the structure-level appendix flag
// (go-slide-creator-deb2h): unnumbered divider, left out of the agenda, and
// its slides' running section reads "Appendix".
func TestExpandStructureAppendixSection(t *testing.T) {
	s := crumbStructure()
	extra := "Detail"
	s.Sections = append(s.Sections, SectionInput{
		Title: "Detailed financials", Appendix: true,
		Slides: []SlideInput{{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &extra}}}},
	})
	slides, err := expandStructure(s)
	if err != nil {
		t.Fatal(err)
	}
	// cover, agenda, div, Size, Growth, div, Edge, appendix div, Detail, closing
	if len(slides) != 10 {
		t.Fatalf("expanded %d slides, want 10", len(slides))
	}
	var agenda AgendaPatternValues
	if err := json.Unmarshal(slides[1].Pattern.Values, &agenda); err != nil {
		t.Fatal(err)
	}
	if strings.Join(agenda.Items, "|") != "Market context|Where we win" {
		t.Errorf("agenda items = %v, want the two chapters only", agenda.Items)
	}
	div := slides[7]
	if div.SectionNumber == nil || !div.SectionNumber.Suppress {
		t.Errorf("appendix divider SectionNumber = %+v, want suppressed", div.SectionNumber)
	}
	if slides[8].SectionTitle != "Appendix: Detailed financials" {
		t.Errorf("appendix slide section = %q", slides[8].SectionTitle)
	}
	labels, _ := deckSectionNumbers(slides, nil)
	if labels[2] != "01" || labels[5] != "02" || labels[7] != "" {
		t.Errorf("labels = %q", labels)
	}
}

// TestApplyChromeSectionCrumbFlatDeck pins go-slide-creator-gl5bh at the unit
// level: a flat deck's crumb follows the preceding divider, like the tracker.
func TestApplyChromeSectionCrumbFlatDeck(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "title", CanonicalType: types.CanonicalLayoutTitleSlide},
		{ID: "section", CanonicalType: types.CanonicalLayoutSectionDivider},
		{ID: "content", CanonicalType: types.CanonicalLayoutOneContent},
	}
	slides := []SlideInput{
		trackerTitled("title", "title", "Cover"),
		trackerTitled("content", "content", "Before"),
		trackerTitled("section", "section", "Financial performance"),
		trackerTitled("content", "content", "Margins"),
	}
	specs := make([]generator.SlideSpec, len(slides))
	for i, s := range slides {
		specs[i].LayoutID = s.LayoutID
	}
	chrome := &ChromeInput{Confidentiality: "Confidential", SectionCrumb: true}
	cfg := chromeToFooterConfig(chrome, len(slides), slides)
	applyChromeSectionCrumb(cfg, specs, chrome, slides, layouts)
	if got := cfg.LeftTextFor(3); got != "Confidential | Financial performance" {
		t.Errorf("crumb = %q", got)
	}
	if got := cfg.LeftTextFor(1); got != "Confidential" {
		t.Errorf("pre-section footer = %q", got)
	}
}

// TestSectionCrumbFlatDeckRenders is the end-to-end check the bead asks for:
// a flat deck with a section divider and chrome.section_crumb renders the
// section name into the content slide's footer.
func TestSectionCrumbFlatDeckRenders(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	title := func(s string) []any {
		return []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": s}}
	}
	input := map[string]any{
		"template":        "midnight-blue",
		"output_filename": "crumb.pptx",
		"chrome":          map[string]any{"confidentiality": "Confidential", "section_crumb": true},
		"slides": []any{
			map[string]any{"layout_id": "section", "content": title("Financial performance")},
			map[string]any{"slide_type": "content", "content": append(title("Margins expanded 3 points"),
				map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []string{"Gross margin up", "Opex flat"}})},
		},
	}
	data, _ := json.Marshal(input)
	inPath := filepath.Join(dir, "in.json")
	if err := os.WriteFile(inPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runJSONMode(inPath, filepath.Join(dir, "result.json"), filepath.Join(root, "templates"), dir,
		"", false, false, "midnight-blue", "off", false, "off", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "crumb.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	var names []string
	xmlByName := map[string]string{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			names = append(names, f.Name)
			xmlByName[f.Name] = string(b)
		}
	}
	sort.Strings(names)
	if len(names) != 2 {
		t.Fatalf("got %d slides", len(names))
	}
	if !strings.Contains(xmlByName[names[1]], "Confidential | Financial performance") {
		t.Errorf("content slide footer lacks the section crumb")
	}
}
