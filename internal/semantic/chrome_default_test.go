package semantic

import "testing"

// go-slide-creator-1iy0x: a DeckSpec without meta.chrome rendered with no page
// number, date or tracker on any slide. Consulting chrome is now the default:
// page numbers (title and closing skipped by the default skip list), the footer
// date from meta.date, and the tracker when the deck has sections.
func TestCompileDefaultsConsultingChrome(t *testing.T) {
	summary := SlideSpec{Kind: KindExecutiveSummary, Body: map[string]any{
		"title":  "Where we stand",
		"points": []any{"Revenue grew 12%", "Churn below 2%", "Runway 26 months"},
	}}
	flat := &DeckSpec{
		Meta:   DeckMeta{Title: "Board update", Template: "midnight-blue", Date: "October 2026"},
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Board update"}}, summary},
	}
	input, result, err := Compile(flat, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}
	c := input.Chrome
	if c == nil || c.PageNumbers == nil || c.PageNumbers.Enabled == nil || !*c.PageNumbers.Enabled {
		t.Fatalf("no meta.chrome must default page numbers on, got %+v", c)
	}
	if c.PageNumbers.Skip != nil {
		t.Errorf("default chrome must keep the default title/closing skip list, got %v", c.PageNumbers.Skip)
	}
	if c.FooterDate != "October 2026" {
		t.Errorf("footer_date = %q, want meta.date", c.FooterDate)
	}
	if c.Tracker {
		t.Error("a flat deck without section slides has nothing to track")
	}

	structured := &DeckSpec{
		Meta: DeckMeta{Title: "Board update", Template: "midnight-blue"},
		Structure: &DeckStructure{
			Cover:      &SlideSpec{Kind: KindTitle, Body: map[string]any{"title": "Board update"}},
			AutoAgenda: true,
			Sections: []DeckSection{
				{Title: "Where we stand", Slides: []SlideSpec{summary}},
				{Title: "What we do next", Slides: []SlideSpec{summary}},
			},
		},
	}
	input, result, err = Compile(structured, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile structured: %v (%+v)", err, result.Diagnostics)
	}
	if input.Chrome == nil || !input.Chrome.Tracker {
		t.Errorf("a sectioned deck must default the tracker on, got %+v", input.Chrome)
	}

	off := false
	optOut := &DeckSpec{
		Meta:   DeckMeta{Title: "Board update", Template: "midnight-blue", Chrome: &ChromeSpec{PageNumbers: &PageNumbersSpec{Enabled: &off}}},
		Slides: flat.Slides,
	}
	input, result, err = Compile(optOut, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile opt-out: %v (%+v)", err, result.Diagnostics)
	}
	if input.Chrome == nil || input.Chrome.PageNumbers == nil || input.Chrome.PageNumbers.Enabled == nil || *input.Chrome.PageNumbers.Enabled {
		t.Errorf("explicit page_numbers.enabled:false must survive compile, got %+v", input.Chrome)
	}
}
