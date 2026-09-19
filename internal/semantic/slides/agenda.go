package slides

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Agenda slides (go-slide-creator-3rvk).
//
// Every deck opens with one and the DeckSpec had no kind for it, so an agent on
// the recommended path either dropped to raw_json2pptx or shipped a bullet
// list. The payload here is what an author writes — a list of sections, and
// optionally which one the deck is at — and the compiler maps it onto the
// agenda / agenda-with-images patterns.
//
// Two patterns, one payload: sections that carry a subtitle are rows with a
// description (agenda-with-images, 3–6 of them), and plain sections are the
// numbered list (agenda, 2–10). The choice follows the content rather than an
// authoring flag, because "did you write subtitles" is the only thing that
// distinguishes the two visuals.

const (
	// agendaMinItems / agendaMaxItems mirror the agenda pattern's bounds.
	agendaMinItems = 2
	agendaMaxItems = 10
	// agendaItemMax is the pattern's per-item character budget.
	agendaItemMax = 100
	// agendaImagesMinItems / agendaImagesMaxItems mirror agenda-with-images.
	agendaImagesMinItems = 3
	agendaImagesMaxItems = 6
	// agendaTitleMax / agendaSubtitleMax mirror its string budgets.
	agendaTitleMax    = 80
	agendaSubtitleMax = 160
)

// agendaSection is one resolved agenda entry. It carries the same shape as
// agendaImagesItem — the two are separate because one is the semantic payload
// and the other is what the pattern reads — so a row converts directly.
type agendaSection struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// agendaValues is the agenda pattern's values object.
type agendaValues struct {
	Items []string `json:"items"`
}

// agendaOverrides carries the highlighted row.
type agendaOverrides struct {
	Highlight int `json:"highlight,omitempty"`
}

// agendaImagesItem is one agenda-with-images row.
type agendaImagesItem struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// agendaImagesValues is the agenda-with-images values object.
type agendaImagesValues struct {
	Items []agendaImagesItem `json:"items"`
}

// CompileAgenda compiles an agenda payload onto the agenda or
// agenda-with-images pattern, falling back to a numbered bullet list when the
// payload does not fit either.
func CompileAgenda(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	sections := AgendaSections(in.Body)
	current := AgendaCurrentIndex(in.Body, sections)

	switch {
	case agendaImagesFits(sections):
		return compileAgendaWithImages(in, sections, current)
	case agendaFits(sections):
		return compileAgendaList(in, sections, current)
	default:
		return compileAgendaFallback(in, sections, current)
	}
}

// compileAgendaList emits the numbered agenda pattern.
func compileAgendaList(in Input, sections []agendaSection, current int) (*deckinput.SlideInput, []SourceLink, error) {
	items := make([]string, 0, len(sections))
	for _, s := range sections {
		items = append(items, s.Title)
	}
	encoded, err := json.Marshal(agendaValues{Items: items})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal agenda values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "agenda", Values: encoded}
	if current > 0 {
		overrides, oErr := json.Marshal(agendaOverrides{Highlight: current})
		if oErr != nil {
			return nil, nil, fmt.Errorf("marshal agenda overrides: %w", oErr)
		}
		slide.Pattern.Overrides = overrides
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.overrides.highlight",
			SemanticPath: in.semSlide() + "." + agendaCurrentField(in.Body),
		})
	}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.items",
		SemanticPath: in.semSlide() + "." + agendaSectionsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileAgendaWithImages emits the row-per-section pattern, which carries the
// subtitles the plain list cannot.
func compileAgendaWithImages(in Input, sections []agendaSection, current int) (*deckinput.SlideInput, []SourceLink, error) {
	items := make([]agendaImagesItem, 0, len(sections))
	for _, s := range sections {
		items = append(items, agendaImagesItem(s))
	}
	encoded, err := json.Marshal(agendaImagesValues{Items: items})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal agenda-with-images values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "agenda-with-images", Values: encoded}
	// agenda-with-images has no highlight override; the current section is
	// marked in its own subtitle so the signal is not silently lost.
	if current > 0 && current <= len(items) {
		items[current-1].Subtitle = agendaCurrentMarker(items[current-1].Subtitle)
		if encoded, err = json.Marshal(agendaImagesValues{Items: items}); err != nil {
			return nil, nil, fmt.Errorf("marshal agenda-with-images values: %w", err)
		}
		slide.Pattern.Values = encoded
	}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.items",
		SemanticPath: in.semSlide() + "." + agendaSectionsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileAgendaFallback renders the sections as bullets when neither pattern
// can take them, marking the current one rather than losing it.
func compileAgendaFallback(in Input, sections []agendaSection, current int) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(sections))
	for i, s := range sections {
		line := strconv.Itoa(i+1) + ". " + s.Title
		if s.Subtitle != "" {
			line += " — " + s.Subtitle
		}
		if current == i+1 {
			line = agendaCurrentMarker(line)
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + "." + agendaSectionsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// agendaCurrentMarker marks the section the deck is at.
func agendaCurrentMarker(text string) string {
	const marker = "(we are here)"
	if strings.Contains(strings.ToLower(text), "we are here") {
		return text
	}
	if text == "" {
		return marker
	}
	return text + " " + marker
}

// AgendaSections resolves an agenda payload's sections. A section is a string,
// or an object carrying a title and an optional subtitle. Entries with no
// usable title are dropped.
func AgendaSections(body map[string]any) []agendaSection {
	raw, ok := firstList(body, "sections", "items", "agenda")
	if !ok {
		return nil
	}
	var out []agendaSection
	for _, e := range raw {
		switch s := e.(type) {
		case string:
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, agendaSection{Title: t})
			}
		case map[string]any:
			title := firstNonEmpty(strField(s, "title"), strField(s, "label"), strField(s, "name"), strField(s, "section"))
			if title == "" {
				continue
			}
			out = append(out, agendaSection{
				Title:    title,
				Subtitle: firstNonEmpty(strField(s, "subtitle"), strField(s, "description"), strField(s, "detail")),
			})
		}
	}
	return out
}

// AgendaCurrentIndex resolves which section the deck is at, as a 1-based index.
// It accepts a number (1-based) or the section's own title, and returns 0 when
// the payload names none or names one that is not in the list.
func AgendaCurrentIndex(body map[string]any, sections []agendaSection) int {
	for _, key := range []string{"current", "current_section", "highlight", "active"} {
		v, ok := body[key]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			n := int(t)
			if n >= 1 && n <= len(sections) {
				return n
			}
		case string:
			want := strings.TrimSpace(strings.ToLower(t))
			for i, s := range sections {
				if strings.ToLower(s.Title) == want {
					return i + 1
				}
			}
		}
	}
	return 0
}

// AgendaSectionsField names the payload field the sections came from, so a
// finding addresses what the author wrote.
func agendaSectionsField(body map[string]any) string {
	for _, key := range []string{"sections", "items", "agenda"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "sections"
}

// agendaCurrentField names the field the current-section marker came from.
func agendaCurrentField(body map[string]any) string {
	for _, key := range []string{"current", "current_section", "highlight", "active"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "current"
}

// agendaFits reports whether the sections fit the plain agenda pattern.
func agendaFits(sections []agendaSection) bool {
	if len(sections) < agendaMinItems || len(sections) > agendaMaxItems {
		return false
	}
	for _, s := range sections {
		if runeLen(s.Title) > agendaItemMax {
			return false
		}
	}
	return true
}

// agendaImagesFits reports whether the sections fit agenda-with-images: at
// least one subtitle to justify the row layout, and every row inside its
// budgets.
func agendaImagesFits(sections []agendaSection) bool {
	if len(sections) < agendaImagesMinItems || len(sections) > agendaImagesMaxItems {
		return false
	}
	subtitled := false
	for _, s := range sections {
		if runeLen(s.Title) > agendaTitleMax || runeLen(s.Subtitle) > agendaSubtitleMax {
			return false
		}
		if s.Subtitle != "" {
			subtitled = true
		}
	}
	return subtitled
}

// AgendaPattern returns the pattern an agenda payload compiles to, or "" when
// it degrades to bullets. The explain planner and validation read it so neither
// promises a visual compile will not emit.
func AgendaPattern(body map[string]any) string {
	sections := AgendaSections(body)
	switch {
	case agendaImagesFits(sections):
		return "agenda-with-images"
	case agendaFits(sections):
		return "agenda"
	default:
		return ""
	}
}

// UsableAgendaSectionCount returns how many sections survive extraction, so
// validation counts what compile will render.
func UsableAgendaSectionCount(body map[string]any) int { return len(AgendaSections(body)) }
