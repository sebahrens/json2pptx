package slides

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Agenda slides (go-slide-creator-3rvk).
//
// Every deck opens with one and the DeckSpec had no kind for it, so an agent on
// the recommended path either dropped to raw_json2pptx or shipped a bullet
// list. The payload here is what an author writes — a list of sections, and
// optionally which one the deck is at — and the compiler maps it onto the
// agenda / agenda-with-images patterns.
//
// One payload, normally one pattern: the numbered agenda list (2–10 sections),
// with each section's subtitle as a smaller muted line under its title and the
// current section in bold with the others dimmed. Subtitled sections used to
// switch to agenda-with-images, whose photo column fell back to solid accent
// tiles and which had no highlight, so "current" was printed as a literal
// "(we are here)" (go-slide-creator-rv9fe). agenda-with-images is now used only
// when the slide asks for it (pattern: agenda-with-images) or a subtitle is too
// long for the list.

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
	Items     []string `json:"items"`
	Subtitles []string `json:"subtitles,omitempty"`
}

// agendaListSubtitleMax mirrors the agenda pattern's subtitle budget.
const agendaListSubtitleMax = 120

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
	case in.Pattern == "agenda-with-images" && agendaImagesFits(sections):
		return compileAgendaWithImages(in, sections)
	case agendaFits(sections):
		return compileAgendaList(in, sections, current)
	case agendaImagesFits(sections):
		return compileAgendaWithImages(in, sections)
	default:
		return compileAgendaFallback(in, sections, current)
	}
}

// compileAgendaList emits the numbered agenda pattern.
func compileAgendaList(in Input, sections []agendaSection, current int) (*deckinput.SlideInput, []SourceLink, error) {
	items := make([]string, 0, len(sections))
	var subtitles []string
	for i, s := range sections {
		items = append(items, s.Title)
		if s.Subtitle != "" {
			for len(subtitles) < i {
				subtitles = append(subtitles, "")
			}
			subtitles = append(subtitles, s.Subtitle)
		}
	}
	encoded, err := json.Marshal(agendaValues{Items: items, Subtitles: subtitles})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal agenda values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "agenda", Values: encoded}
	// The pattern's own type (28pt serif numerals, 14pt items) is the
	// design-review agenda (go-slide-creator-r3gsw); only the current-section
	// highlight is carried.
	ovr := patterns.AgendaOverrides{Highlight: current}
	var oErr error
	slide.Pattern.Overrides, oErr = json.Marshal(ovr)
	if oErr != nil {
		return nil, nil, fmt.Errorf("marshal agenda overrides: %w", oErr)
	}
	if current > 0 {
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

// compileAgendaWithImages emits the row-per-section pattern: on request, or
// for subtitles longer than the list holds. It has no highlight, and the
// current section is not printed as a marker in its text
// (go-slide-creator-rv9fe).
func compileAgendaWithImages(in Input, sections []agendaSection) (*deckinput.SlideInput, []SourceLink, error) {
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
	slide.Pattern.Overrides, err = json.Marshal(patterns.AgendaWithImagesOverrides{TitleSize: 16, SubtitleSize: 12})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal agenda-with-images design defaults: %w", err)
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
		if i, isInt := v.(int); isInt {
			v = float64(i)
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

// AgendaUnresolvedCurrent returns the current-section field when the author
// set one that names no section (an out-of-range number or an unknown title),
// so validation can say the marker is lost; "" otherwise.
func AgendaUnresolvedCurrent(body map[string]any) string {
	field := agendaCurrentField(body)
	if !referencePresent(body[field]) {
		return ""
	}
	if AgendaCurrentIndex(map[string]any{field: body[field]}, AgendaSections(body)) != 0 {
		return ""
	}
	return field
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
		if runeLen(s.Title) > agendaItemMax || runeLen(s.Subtitle) > agendaListSubtitleMax {
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
	case agendaFits(sections):
		return "agenda"
	case agendaImagesFits(sections):
		return "agenda-with-images"
	default:
		return ""
	}
}

// UsableAgendaSectionCount returns how many sections survive extraction, so
// validation counts what compile will render.
func UsableAgendaSectionCount(body map[string]any) int { return len(AgendaSections(body)) }
