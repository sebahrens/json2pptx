package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestStructuredDeckExpandsInDeterministicOrder(t *testing.T) {
	cover := SlideSpec{Kind: KindTitle, Body: map[string]any{"title": "Agentic AI"}}
	closing := SlideSpec{Kind: KindClosing, Body: map[string]any{"title": "Questions"}}
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Agentic AI", Chrome: &ChromeSpec{SectionCrumb: true}},
		Structure: &DeckStructure{
			Cover: &cover, AutoAgenda: true,
			Sections: []DeckSection{
				{Title: "Landscape", Slides: []SlideSpec{kpiSlide(3)}},
				{Title: "Outlook", Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{"title": "Scenarios", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}, "takeaway": "Two paths"}}}},
			},
			Closing: &closing,
		},
	}
	ir := Normalize(spec)
	want := []SlideKind{KindTitle, KindAgenda, KindSection, KindKPISnapshot, KindSection, KindComparison, KindClosing}
	if len(ir.Slides) != len(want) {
		t.Fatalf("expanded slides = %d, want %d: %+v", len(ir.Slides), len(want), ir.Slides)
	}
	for i, kind := range want {
		if ir.Slides[i].Kind != kind {
			t.Errorf("slide %d kind = %q, want %q", i, ir.Slides[i].Kind, kind)
		}
	}
	if ir.Slides[3].SectionTitle != "Landscape" || ir.Slides[5].SectionTitle != "Outlook" {
		t.Errorf("section titles not carried into content: %+v", ir.Slides)
	}
	if ir.Slides[2].SourcePath != "structure.sections[0]" || ir.Slides[4].SourcePath != "structure.sections[1]" {
		t.Errorf("divider source paths = %q, %q", ir.Slides[2].SourcePath, ir.Slides[4].SourcePath)
	}

	input, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile structured deck: %v; diagnostics=%+v", err, result.Diagnostics)
	}
	if input.Slides[3].SectionTitle != "Landscape" || input.Slides[5].SectionTitle != "Outlook" {
		t.Errorf("compiled section crumbs = %q, %q", input.Slides[3].SectionTitle, input.Slides[5].SectionTitle)
	}
	if input.Slides[2].SectionTitle != "" || input.Slides[4].SectionTitle != "" {
		t.Error("divider slides must not repeat their title in section crumb chrome")
	}
	if input.Slides[2].LayoutID != "section" || input.Slides[4].LayoutID != "section" {
		t.Errorf("generated divider layouts = %q, %q; want section for both", input.Slides[2].LayoutID, input.Slides[4].LayoutID)
	}
	entry, ok := result.SourceMap.Lookup("slides[4].content[0].text_value")
	if !ok || !strings.HasPrefix(entry.SemanticPath, "structure.sections[1]") {
		t.Errorf("divider source mapping = %+v, %v; entries=%+v", entry, ok, result.SourceMap.Entries())
	}
}

func TestStructuredDeckValidationRejectsMixedAndEmptySections(t *testing.T) {
	spec := &DeckSpec{
		Meta:      DeckMeta{Title: "Deck"},
		Slides:    []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}},
		Structure: &DeckStructure{Sections: []DeckSection{{Title: "Empty"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	var mixed, empty bool
	for _, d := range ds {
		mixed = mixed || d.Code == "STRUCTURE_AND_SLIDES"
		empty = empty || (d.Code == diagnostics.CodeSemanticRequired && d.Path == "structure.sections[0].slides")
	}
	if !mixed || !empty {
		t.Fatalf("mixed=%v empty=%v diagnostics=%+v", mixed, empty, ds)
	}
}

func TestParseStructuredDeck(t *testing.T) {
	spec, ds := ParseYAML([]byte(`
meta:
  title: Structured
structure:
  auto_agenda: true
  sections:
    - title: One
      slides:
        - kind: kpi_snapshot
          title: Metrics
          kpis:
            - {label: Revenue, value: 10}
          takeaway: Revenue grew
`))
	if ds.HasErrors() {
		t.Fatalf("parse diagnostics: %+v", ds)
	}
	if spec.Structure == nil || len(spec.Structure.Sections) != 1 || len(spec.Structure.Sections[0].Slides) != 1 {
		t.Fatalf("parsed structure = %+v", spec.Structure)
	}
}

func TestParsedStructuredDeckRejectsExplicitEmptySlides(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		data string
	}{
		{"yaml", "deck.yaml", "meta: {title: Deck}\nslides: []\nstructure:\n  sections:\n    - title: One\n      slides:\n        - {kind: stat, title: Growth, value: 10}\n"},
		{"json", "deck.json", `{"meta":{"title":"Deck"},"slides":[],"structure":{"sections":[{"title":"One","slides":[{"kind":"stat","title":"Growth","value":10}]}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := Check(tc.file, []byte(tc.data), StrictnessOff)
			count := 0
			for _, diagnostic := range diagnostics {
				if diagnostic.Code == "STRUCTURE_AND_SLIDES" && diagnostic.Path == "structure" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("mutual-exclusion diagnostics = %d, want 1: %+v", count, diagnostics)
			}
		})
	}
}

func TestStructuredContainerShapeDiagnosticsArePathScoped(t *testing.T) {
	cases := []struct{ body, path string }{
		{"structure: {sections: wrong}\n", "structure.sections"},
		{"structure: {sections: [wrong]}\n", "structure.sections[0]"},
		{"structure: {sections: [{title: One, slides: wrong}]}\n", "structure.sections[0].slides"},
		{"structure: {cover: wrong, sections: []}\n", "structure.cover"},
		{"structure: {cover: null, sections: []}\n", "structure.cover"},
		{"structure: {closing: wrong, sections: []}\n", "structure.closing"},
	}
	for _, tc := range cases {
		_, diagnostics := ParseYAML([]byte("meta: {title: Deck}\n" + tc.body))
		if len(diagnostics) == 0 || diagnostics[0].Path != tc.path || strings.Contains(diagnostics[0].Message, "rawSection") {
			t.Errorf("body %q diagnostics = %+v, want first path %q", tc.body, diagnostics, tc.path)
		}
	}
}

func TestStructuredUnknownFieldsAreDiagnosed(t *testing.T) {
	_, diagnostics := ParseYAML([]byte("meta: {title: Deck}\nstructure:\n  typo_field: true\n  sections:\n    - title: One\n      slidez: []\n      slides: []\n"))
	want := map[string]bool{"structure.typo_field": false, "structure.sections[0].slidez": false}
	for _, diagnostic := range diagnostics {
		if _, ok := want[diagnostic.Path]; ok {
			want[diagnostic.Path] = true
		}
		if diagnostic.Path == "structure.sections[0].slidez" && !strings.Contains(diagnostic.Message, `did you mean "slides"`) {
			t.Errorf("slidez diagnostic lacks suggestion: %+v", diagnostic)
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("missing unknown-field diagnostic at %s: %+v", path, diagnostics)
		}
	}
}

func TestExpandedSlidePayloadsIncludeGeneratedStructureSlides(t *testing.T) {
	cover := SlideSpec{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}
	closing := SlideSpec{Kind: KindClosing, Body: map[string]any{"title": "Close"}}
	spec := &DeckSpec{Structure: &DeckStructure{Cover: &cover, AutoAgenda: true, Closing: &closing, Sections: []DeckSection{
		{Title: "One", Slides: []SlideSpec{{Kind: KindStat, Body: map[string]any{"title": "One", "value": 1}}}},
		{Title: "Two", Slides: []SlideSpec{{Kind: KindStat, Body: map[string]any{"title": "Two", "value": 2}}}},
	}}}
	payloads, err := ExpandedSlidePayloads(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 7 || ExpandedSlideCount(spec) != 7 {
		t.Fatalf("expanded payload/count = %d/%d, want 7", len(payloads), ExpandedSlideCount(spec))
	}
}
