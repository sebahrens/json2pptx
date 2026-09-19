package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func statBody(overlay map[string]any) map[string]any {
	body := map[string]any{
		"title": "The prize",
		"value": "$2.4B",
		"label": "Addressable clearing-services market by FY27",
	}
	for k, v := range overlay {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return body
}

// A single number carrying the slide is the oldest move in a pitch deck, and
// DeckSpec had no kind for it (go-slide-creator-2hkc).
func TestCompileStatUsesTheHeroWhenItFits(t *testing.T) {
	body := statBody(map[string]any{
		"unit":    "TAM",
		"context": "Up from $1.6B in FY24, driven by the T+1 mandate.",
		"source":  "Oliver Wyman market model, 2026",
	})
	if got := StatPattern(body); got != "stat-hero" {
		t.Fatalf("StatPattern = %q, want stat-hero", got)
	}
	slide, _, err := CompileStat(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "stat-hero" {
		t.Fatalf("pattern = %+v, want stat-hero", slide.Pattern)
	}
	var values statHeroValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatalf("values %s: %v", slide.Pattern.Values, err)
	}
	want := statHeroValues{
		Value:   "$2.4B",
		Unit:    "TAM",
		Label:   "Addressable clearing-services market by FY27",
		Context: "Up from $1.6B in FY24, driven by the T+1 mandate.",
		Source:  "Oliver Wyman market model, 2026",
	}
	if values != want {
		t.Errorf("values = %+v, want %+v", values, want)
	}
}

// The label is the words beneath the number. An author who wrote only a title
// meant it as those words, rather than leaving the number bare.
func TestStatLabelFallsBackToTheTitle(t *testing.T) {
	body := statBody(map[string]any{"label": nil})
	if over := StatOverBudget(body); over != "" {
		t.Errorf("a titled stat reported %q", over)
	}
	if got := statValues(body).Label; got != "The prize" {
		t.Errorf("label = %q, want the title", got)
	}
	// With neither, there are no words to put beneath the number, so the hero is
	// not the right treatment.
	bare := map[string]any{"value": "$2.4B"}
	if over := StatOverBudget(bare); !strings.Contains(over, "no label") {
		t.Errorf("StatOverBudget = %q, want it to name the missing label", over)
	}
	if got := StatPattern(bare); got != "" {
		t.Errorf("StatPattern = %q, want the content fallback", got)
	}
}

// Past the pattern's budgets the slide degrades rather than handing the renderer
// a payload it will reject, and the finding names the budget that broke.
func TestStatDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "a value the display size cannot hold",
			body:   statBody(map[string]any{"value": "$2,412,880,000.00 (FY27)"}),
			reason: "value is 24 characters",
		},
		{
			name:   "a unit written as a sentence",
			body:   statBody(map[string]any{"unit": "addressable market"}),
			reason: "unit is 18 characters",
		},
		{
			name:   "a label written as a paragraph",
			body:   statBody(map[string]any{"label": strings.Repeat("a", 90)}),
			reason: "label is 90 characters",
		},
		{
			name:   "an over-long context line",
			body:   statBody(map[string]any{"context": strings.Repeat("a", 130)}),
			reason: "context is 130 characters",
		},
		{
			name:   "an over-long source",
			body:   statBody(map[string]any{"source": strings.Repeat("a", 90)}),
			reason: "source is 90 characters",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatPattern(c.body); got != "" {
				t.Errorf("StatPattern = %q, want the content fallback", got)
			}
			over := StatOverBudget(c.body)
			if !strings.Contains(over, c.reason) {
				t.Errorf("StatOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileStat(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the content fallback, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Fatal("the fallback lost the number entirely")
			}
		})
	}
}

// The degrade must not cost the author a word: everything they wrote is still on
// the slide, just as text. The source is the one exception — the compiler
// promotes it to the slide's attribution band, so repeating it as a bullet
// would print it twice.
func TestStatFallbackKeepsEveryField(t *testing.T) {
	body := statBody(map[string]any{
		"value":   "$2,412,880,000.00 (FY27)",
		"unit":    "TAM",
		"context": "Up from $1.6B in FY24.",
		"source":  "Oliver Wyman, 2026",
	})
	slide, _, err := CompileStat(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{
		"$2,412,880,000.00 (FY27)", "TAM",
		"Addressable clearing-services market by FY27",
		"Up from $1.6B in FY24.",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the fallback dropped %q: %s", want, encoded)
		}
	}
	if strings.Contains(string(encoded), "Oliver Wyman") {
		t.Errorf("the source is in the content as well as the attribution band: %s", encoded)
	}
}

// An empty value is the required-field gate's business, so the budget rule stays
// quiet rather than reporting the same problem twice.
func TestStatWithNoValueReportsNoBudgetProblem(t *testing.T) {
	if over := StatOverBudget(map[string]any{"title": "The prize"}); over != "" {
		t.Errorf("a valueless stat reported a budget problem: %q", over)
	}
	if n := UsableStatValue(map[string]any{"title": "The prize"}); n != 0 {
		t.Errorf("UsableStatValue = %d, want 0", n)
	}
	if n := UsableStatValue(map[string]any{"number": "118%"}); n != 1 {
		t.Errorf("UsableStatValue = %d, want 1", n)
	}
}

// The spellings an author reaches for all resolve to the same field.
func TestStatFieldAliases(t *testing.T) {
	for _, key := range []string{"value", "stat", "number", "metric"} {
		body := map[string]any{key: "118%", "label": "Net revenue retention"}
		if got := statValues(body).Value; got != "118%" {
			t.Errorf("%s: value = %q", key, got)
		}
		if got := statValueField(body); got != key {
			t.Errorf("%s: statValueField = %q", key, got)
		}
	}
	for _, key := range []string{"label", "caption", "subtitle"} {
		body := map[string]any{"value": "118%", key: "Net revenue retention"}
		if got := statValues(body).Label; got != "Net revenue retention" {
			t.Errorf("%s: label = %q", key, got)
		}
	}
	for _, key := range []string{"unit", "suffix"} {
		body := map[string]any{"value": "118%", "label": "NRR", key: "MRR"}
		if got := statValues(body).Unit; got != "MRR" {
			t.Errorf("%s: unit = %q", key, got)
		}
	}
	for _, key := range []string{"context", "detail", "description"} {
		body := map[string]any{"value": "118%", "label": "NRR", key: "Up nine points."}
		if got := statValues(body).Context; got != "Up nine points." {
			t.Errorf("%s: context = %q", key, got)
		}
	}
}
