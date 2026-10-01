package patterns

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

// One source convention for the whole engine (go-slide-creator-7eib).
//
// slide.source used to render as "Source: <text>" at 8pt in a raw #888888,
// right-aligned in the chrome band, while chart-insights-split drew its own
// values.source verbatim at 9pt dk1 italic on the left, and stat-hero at 10pt
// centred. One deck therefore showed sources in three sizes, two alignments
// and two corners depending on which pattern happened to own the slide — and
// an agent that wrote "Source: …" into the slide-level field got
// "Source: Source: …" on a pattern slide.

const (
	// SourceNoteSizePt is the one size a source line is set in: 9pt, the
	// footnote step of the type scale (go-slide-creator-cuszt).
	//
	// On a generated slide every source — slide.source and the values.source
	// of chart-insights-split and stat-hero, which LiftPatternSource moves to
	// the slide — renders once, in the chrome frame's source zone just above
	// the footer, at this size. A pattern expanded on its own (expand_pattern)
	// has no chrome and still draws its source inside the grid, where
	// shape_grid's 12pt readability floor (shapegrid.MinTextSizePt) applies.
	SourceNoteSizePt = tokens.SourceLinePt
	// SourceNoteScheme is the source line's base colour: the template's text
	// colour, which the chrome zone mutes to about 60% (SourceNoteLumMod /
	// SourceNoteLumOff) so the attribution reads as a footnote without
	// leaving the theme. tx1 rather than dk1 so a layout whose colour map
	// puts light text on a dark background keeps a readable note.
	SourceNoteScheme = "tx1"
	// SourceNoteLumMod and SourceNoteLumOff mute the source colour to ~60%
	// strength (PowerPoint's "Lighter 40%"): dark text becomes a mid grey
	// that still clears WCAG AA on a light background; light text stays light.
	SourceNoteLumMod = 60000
	SourceNoteLumOff = 40000
	// SourceNoteAlign is the alignment of a source line: left, on the content
	// grid's left edge.
	SourceNoteAlign = "l"
)

// SourceNoteText labels a source line without stuttering when the author
// already wrote the label. It is the one place that decides whether a source
// carries the "Source: " prefix, so the two surfaces cannot disagree
// (go-slide-creator-xg48 fixed the stutter for the slide-level note;
// go-slide-creator-7eib made the pattern surfaces use the same rule).
func SourceNoteText(sourceText string) string {
	trimmed := strings.TrimSpace(sourceText)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "source:") {
		return trimmed
	}
	return "Source: " + trimmed
}

// sourceLiftPatterns are the patterns whose values carry a "source" string
// that, on a generated slide, belongs in the chrome source zone rather than
// inside the pattern's grid.
var sourceLiftPatterns = map[string]bool{
	"chart-insights-split": true,
	"stat-hero":            true,
}

// LiftPatternSource moves a pattern's values.source out of its values so the
// slide's chrome source zone can render it (go-slide-creator-cuszt). It
// returns the trimmed source and the values without it; ok is false when the
// pattern carries no liftable source (or its values are not an object), in
// which case values is returned unchanged.
//
// Reserving the source as chrome is what lets the pattern centre in the space
// that remains: the zone sits in the chrome frame's band stack, which the
// content zone already stops above, so a lifted source no longer eats a row of
// the chart panel and every source on the deck lands in the same place, at
// the same size, on the same left edge.
func LiftPatternSource(name string, values json.RawMessage) (source string, stripped json.RawMessage, ok bool) {
	if !sourceLiftPatterns[name] || len(values) == 0 {
		return "", values, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(values, &fields); err != nil {
		return "", values, false
	}
	raw, present := fields["source"]
	if !present {
		return "", values, false
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return "", values, false
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return "", values, false
	}
	delete(fields, "source")
	out, err := json.Marshal(fields)
	if err != nil {
		return "", values, false
	}
	return source, out, true
}

// MergeSourceNotes joins a slide-level source with a lifted pattern source,
// dropping a duplicate (compared case-insensitively, label included) so one
// attribution never prints twice.
func MergeSourceNotes(slideSource, patternSource string) string {
	a, b := strings.TrimSpace(slideSource), strings.TrimSpace(patternSource)
	switch {
	case a == "":
		return b
	case b == "" || strings.EqualFold(SourceNoteText(a), SourceNoteText(b)):
		return a
	}
	return a + "; " + strings.TrimSpace(strings.TrimPrefix(SourceNoteText(b), "Source: "))
}
