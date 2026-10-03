package placeholder

import "testing"

func TestDetectRegisteredPlaceholders(t *testing.T) {
	hits := map[string]string{
		"__FILL__":                               NameFillToken,
		"Q3 __FILL__ results":                    NameFillToken,
		RecipeActionTitle("bar chart"):           NameRecipeTitle,
		"REPLACE WITH THE ACTION TITLE this one": NameRecipeTitle,
		RecipeAltText("swot diagram"):            NameRecipeAltText,
		RecipeSampleSource:                       NameRecipeSampleSource,
		"<rewrite this field to resolve INPUT.SEMANTIC_DENSITY while preserving meaning; at most 40 characters>": NameArgumentHint,
		"<the point this slide makes>": NameArgumentHint,
		" <DeckSpec> ":                 NameArgumentHint,
	}
	for text, want := range hits {
		m, ok := Detect(text)
		if !ok || m.Name != want {
			t.Errorf("Detect(%q) = %q, %v; want %s", text, m.Name, ok, want)
		}
	}
	for _, text := range []string{
		"",
		"Replace legacy billing with usage-based pricing by 2027",
		"Margins stay <5% until the second plant opens",
		"Churn < 2% and NRR > 110%",
		"<b>bold</b> claims need a source",
		"Fill rate reached 96% in the north region",
		"Illustrative sample of 40 accounts",
	} {
		if m, ok := Detect(text); ok {
			t.Errorf("Detect(%q) reports %s; authored copy is not a placeholder", text, m.Name)
		}
	}
}

// Every marker is named, sourced and detected by its own phrase, so a producer
// that registers one gets detection with it.
func TestRegisteredMarkersAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Registered() {
		if m.Name == "" || m.Phrase == "" || m.Source == "" {
			t.Errorf("incomplete marker: %+v", m)
		}
		if seen[m.Name] {
			t.Errorf("marker %s is registered twice", m.Name)
		}
		seen[m.Name] = true
		if m.Name == NameArgumentHint {
			continue // a shape, not a phrase
		}
		if got, ok := Detect("x " + m.Phrase + " y"); !ok || got.Name != m.Name {
			t.Errorf("marker %s is not detected by its phrase %q", m.Name, m.Phrase)
		}
	}
	if len(seen) < 5 {
		t.Errorf("registry lists %d markers", len(seen))
	}
}
