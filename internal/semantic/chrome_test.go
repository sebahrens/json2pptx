package semantic

import (
	"testing"
)

// go-slide-creator-zmjs: get_started("brief") and SKILL.md send agents to
// render_deck_spec as THE path for a new deck, but DeckSpec could not express a
// confidentiality line, page numbers, speaker notes, or a source on any kind but
// chart_insight. meta.chrome failed with "unknown meta field" and the deck did
// not compile; a slide-level source was accepted and reported as DROPPED. Real
// board decks carry all of it, so the recommended path produced unshippable
// decks.
func TestDeckSpecCarriesChromeAndPerSlideFields(t *testing.T) {
	enabled := true
	spec := &DeckSpec{
		Meta: DeckMeta{
			Title:          "Board update",
			Template:       "midnight-blue",
			Date:           "September 2026",
			ViewingMode:    "present",
			AccentStrategy: "rotate",
			Chrome: &ChromeSpec{
				Confidentiality: "Strictly confidential",
				ClientName:      "Acme Corp",
				ProjectCode:     "Aurora",
				PageNumbers:     &PageNumbersSpec{Enabled: &enabled, Format: "{current} / {total}", Skip: []string{"title"}},
			},
		},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Board update", "notes": "Set the agenda."}},
			{Kind: KindExecutiveSummary, Body: map[string]any{
				"title":    "Where we stand",
				"points":   []any{"Revenue grew 12%", "Churn below 2%", "Runway 26 months"},
				"takeaway": "On track.",
				"notes":    "Pause for questions.",
				"source":   "Finance close pack",
			}},
		},
	}

	input, result, err := Compile(spec, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}

	if input.Chrome == nil {
		t.Fatal("meta.chrome did not reach the compiled deck")
	}
	if input.Chrome.Confidentiality != "Strictly confidential" || input.Chrome.ClientName != "Acme Corp" || input.Chrome.ProjectCode != "Aurora" {
		t.Errorf("chrome = %+v", input.Chrome)
	}
	// footer_date is not set on the block, so meta.date fills it.
	if input.Chrome.FooterDate != "September 2026" {
		t.Errorf("footer_date = %q, want meta.date", input.Chrome.FooterDate)
	}
	if input.Chrome.PageNumbers == nil || input.Chrome.PageNumbers.Format != "{current} / {total}" ||
		len(input.Chrome.PageNumbers.Skip) != 1 {
		t.Errorf("page_numbers = %+v", input.Chrome.PageNumbers)
	}
	if input.ViewingMode != "present" {
		t.Errorf("viewing_mode = %q", input.ViewingMode)
	}
	if input.AccentStrategy != "rotate" {
		t.Errorf("accent_strategy = %q", input.AccentStrategy)
	}

	if got := input.Slides[0].SpeakerNotes; got != "Set the agenda." {
		t.Errorf("slide 0 speaker notes = %q", got)
	}
	if got := input.Slides[1].SpeakerNotes; got != "Pause for questions." {
		t.Errorf("slide 1 speaker notes = %q", got)
	}
	if got := input.Slides[1].Source; got != "Finance close pack" {
		t.Errorf("slide 1 source = %q — before this, only chart_insight could cite anything", got)
	}
	// No diagnostics about dropped content.
	for _, d := range result.Diagnostics {
		switch d.Code {
		case "SEMANTIC_UNKNOWN_FIELD":
			t.Errorf("unexpected unknown-field diagnostic: %s %s", d.Path, d.Message)
		}
	}
}

// The spec's own accent strategy wins over the caller default.
func TestDeckSpecAccentStrategyOverridesOption(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Board update", Template: "midnight-blue", AccentStrategy: "section-keyed"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Board update"}},
			{Kind: KindExecutiveSummary, Body: map[string]any{
				"title":    "Where we stand",
				"points":   []any{"Revenue grew 12%", "Churn below 2%", "Runway 26 months"},
				"takeaway": "On track.",
			}},
		},
	}
	input, result, err := Compile(spec, CompileOptions{Strict: StrictnessWarn, AccentStrategy: "rotate"})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}
	if input.AccentStrategy != "section-keyed" {
		t.Errorf("accent_strategy = %q, want the spec's own choice", input.AccentStrategy)
	}
}

// Every kind accepts notes and source, and the schema says so.
func TestUniversalFieldsOnEveryKind(t *testing.T) {
	for _, k := range AllSlideKinds() {
		if k == KindRawJSON2pptx {
			continue // raw escape hatch: the slide object carries its own fields
		}
		names := PayloadFieldNames(k)
		for _, want := range []string{"notes", "source"} {
			found := false
			for _, n := range names {
				if n == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("kind %s does not accept %q; fields: %v", k, want, names)
			}
		}
	}
}

// meta.chrome must be a known field — it used to fail the compile outright.
func TestChromeIsAKnownMetaField(t *testing.T) {
	for _, want := range []string{"chrome", "viewing_mode", "accent_strategy"} {
		found := false
		for _, k := range knownMetaKeys {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("meta.%s is not in knownMetaKeys: %v", want, knownMetaKeys)
		}
	}
}
