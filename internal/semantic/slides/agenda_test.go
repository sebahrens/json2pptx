package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func agendaBody(extra map[string]any, sections ...any) map[string]any {
	body := map[string]any{"title": "What we will cover", "sections": sections}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func subtitled(title, subtitle string) map[string]any {
	return map[string]any{"title": title, "subtitle": subtitle}
}

// Every deck opens with an agenda and the DeckSpec had no kind for it, so an
// agent on the recommended path dropped to raw_json2pptx or shipped a bullet
// list (go-slide-creator-3rvk).
func TestCompileAgendaPicksThePatternFromTheContent(t *testing.T) {
	cases := []struct {
		name        string
		body        map[string]any
		wantPattern string
	}{
		{
			name: "subtitled sections become rows",
			body: agendaBody(nil,
				subtitled("Where we are", "Q3 against the plan"),
				subtitled("What we found", "Three findings"),
				subtitled("What we recommend", "The decision")),
			wantPattern: "agenda-with-images",
		},
		{
			name:        "plain sections become the numbered list",
			body:        agendaBody(nil, "Performance", "Risks", "Investment"),
			wantPattern: "agenda",
		},
		{
			name:        "two plain sections still fit the list",
			body:        agendaBody(nil, "Performance", "Risks"),
			wantPattern: "agenda",
		},
		{
			name: "two subtitled sections fall back to the list",
			body: agendaBody(nil,
				subtitled("Where we are", "Q3 against the plan"),
				subtitled("What we found", "Three findings")),
			wantPattern: "agenda",
		},
		{
			name:        "one section is not an agenda",
			body:        agendaBody(nil, "Performance"),
			wantPattern: "",
		},
		{
			name:        "eleven sections exceed the list",
			body:        agendaBody(nil, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"),
			wantPattern: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AgendaPattern(c.body); got != c.wantPattern {
				t.Fatalf("AgendaPattern = %q, want %q", got, c.wantPattern)
			}
			slide, _, err := CompileAgenda(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			switch {
			case c.wantPattern == "":
				if slide.Pattern != nil {
					t.Errorf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
				}
				if len(slide.Content) == 0 {
					t.Error("the fallback lost the sections entirely")
				}
			default:
				if slide.Pattern == nil || slide.Pattern.Name != c.wantPattern {
					t.Errorf("pattern = %+v, want %s", slide.Pattern, c.wantPattern)
				}
			}
		})
	}
}

// The current section is the whole reason an agenda is repeated between
// sections; it must survive whichever way the slide is rendered.
func TestAgendaCurrentSectionSurvivesEveryPath(t *testing.T) {
	t.Run("numbered list highlights the row", func(t *testing.T) {
		body := agendaBody(map[string]any{"current": 2.0}, "Performance", "Risks", "Investment")
		slide, _, err := CompileAgenda(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		var ovr struct {
			Highlight int `json:"highlight"`
		}
		if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
			t.Fatalf("overrides %s: %v", slide.Pattern.Overrides, err)
		}
		if ovr.Highlight != 2 {
			t.Errorf("highlight = %d, want 2", ovr.Highlight)
		}
	})

	t.Run("a section title selects the row", func(t *testing.T) {
		body := agendaBody(map[string]any{"current": "risks"}, "Performance", "Risks", "Investment")
		sections := AgendaSections(body)
		if got := AgendaCurrentIndex(body, sections); got != 2 {
			t.Errorf("AgendaCurrentIndex = %d, want 2", got)
		}
	})

	t.Run("rows carry the marker in the subtitle", func(t *testing.T) {
		body := agendaBody(map[string]any{"current": 2.0},
			subtitled("Where we are", "Q3 against the plan"),
			subtitled("What we found", "Three findings"),
			subtitled("What we recommend", "The decision"))
		slide, _, err := CompileAgenda(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if !strings.Contains(string(slide.Pattern.Values), "we are here") {
			t.Errorf("the current row is unmarked: %s", slide.Pattern.Values)
		}
	})

	t.Run("the fallback marks it too", func(t *testing.T) {
		body := agendaBody(map[string]any{"current": 3.0}, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k")
		slide, _, err := CompileAgenda(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		raw, mErr := json.Marshal(slide.Content)
		if mErr != nil {
			t.Fatalf("marshal content: %v", mErr)
		}
		if !strings.Contains(string(raw), "we are here") {
			t.Errorf("the fallback lost the current-section marker: %s", raw)
		}
	})

	t.Run("an out-of-range index is ignored rather than guessed", func(t *testing.T) {
		body := agendaBody(map[string]any{"current": 9.0}, "Performance", "Risks")
		if got := AgendaCurrentIndex(body, AgendaSections(body)); got != 0 {
			t.Errorf("AgendaCurrentIndex = %d, want 0", got)
		}
	})
}

// The aliases an author reaches for must all resolve.
func TestAgendaSectionAliases(t *testing.T) {
	for _, field := range []string{"sections", "items", "agenda"} {
		body := map[string]any{field: []any{"Performance", "Risks"}}
		if n := len(AgendaSections(body)); n != 2 {
			t.Errorf("%s: resolved %d sections, want 2", field, n)
		}
	}
	// Object entries accept the usual spellings of "the section's name".
	for _, key := range []string{"title", "label", "name", "section"} {
		body := map[string]any{"sections": []any{
			map[string]any{key: "Performance"}, map[string]any{key: "Risks"},
		}}
		got := AgendaSections(body)
		if len(got) != 2 || got[0].Title != "Performance" {
			t.Errorf("%s: resolved %+v", key, got)
		}
	}
	// An entry with no usable name is dropped rather than rendering blank.
	body := map[string]any{"sections": []any{"Performance", map[string]any{"subtitle": "orphan"}, "Risks"}}
	if n := len(AgendaSections(body)); n != 2 {
		t.Errorf("resolved %d sections, want the 2 with names", n)
	}
}
