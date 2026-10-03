package generator

import (
	"regexp"
	"strings"
	"unicode"
)

// Unit case in all-caps titles (go-slide-creator-3rg5f).
//
// Several templates set their titles in capitals (cap="all" in the master's
// titleStyle or the layout's title / subtitle list style). That is a display
// transform, and for words it changes nothing but the look. For a number with
// a unit it changes the meaning: "€2.2m" becomes "€2.2M", "6 mo" becomes
// "6 MO", "3pp" becomes "3PP", "40 kWh" becomes "40 KWH" — and nothing told the
// author before the image was seen.
//
// Title and subtitle text is therefore written with its number-unit tokens in
// runs of their own that carry cap="none", so the unit keeps the case it was
// typed in on every template. On a template that does not capitalise titles
// the attribute changes nothing. Only tokens whose letters include a lower-case
// one are touched: "5G", "Q3" and "B2B" read the same either way.

// attachedUnitRe matches a number with a unit written against it: "€2.2m",
// "12bn", "40k", "3pp", "10x", "200ms", "15kWh", "1990s". The unit is one to
// four letters that end the word.
var attachedUnitRe = regexp.MustCompile(`[^\s\pL]*\d[\d.,]*(\pL{1,4})`)

// spacedUnitRe matches a number followed by a space and a unit: "6 mo",
// "2.2 bn", "40 kWh". Whether the word really is a unit is decided by
// spacedUnits, since most short words after a number are not ("3 of", "5 key").
var spacedUnitRe = regexp.MustCompile(`[^\s\pL]*\d[\d.,]*%?[ \x{00A0}](\pL{1,4})`)

// spacedUnits are the units recognised after a space, lower-cased. They are
// abbreviations that are not English words, so a title such as "6 in 10" or
// "3 min read" is not mistaken for a measurement.
var spacedUnits = map[string]bool{
	"m": true, "mn": true, "bn": true, "tn": true, "k": true,
	"mo": true, "mos": true, "yr": true, "yrs": true, "hr": true, "hrs": true, "ms": true,
	"pp": true, "ppt": true, "bp": true, "bps": true, "pt": true, "pts": true, "x": true,
	"g": true, "kg": true, "mg": true, "km": true, "cm": true, "mm": true,
	"kw": true, "kwh": true, "mwh": true, "gwh": true, "twh": true,
	"kb": true, "mb": true, "gb": true, "tb": true,
}

// ordinalSuffixes are not units: "1ST" in a capitalised title is the same word.
var ordinalSuffixes = map[string]bool{"st": true, "nd": true, "rd": true, "th": true}

// unitCaseSpans returns the byte ranges of text that hold a number-unit token
// whose unit is written with a lower-case letter, in order and not overlapping.
func unitCaseSpans(text string) [][2]int {
	var spans [][2]int
	hasLower := func(s string) bool { return strings.IndexFunc(s, unicode.IsLower) >= 0 }
	wordEnds := func(end int) bool {
		if end >= len(text) {
			return true
		}
		r := []rune(text[end:])[0]
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}
	add := func(m []int, spaced bool) {
		unit := text[m[2]:m[3]]
		if !hasLower(unit) || !wordEnds(m[3]) {
			return
		}
		lower := strings.ToLower(unit)
		if spaced && !spacedUnits[lower] {
			return
		}
		if !spaced && ordinalSuffixes[lower] {
			return
		}
		spans = append(spans, [2]int{m[0], m[1]})
	}
	for _, m := range attachedUnitRe.FindAllStringSubmatchIndex(text, -1) {
		add(m, false)
	}
	for _, m := range spacedUnitRe.FindAllStringSubmatchIndex(text, -1) {
		add(m, true)
	}
	if len(spans) < 2 {
		return spans
	}
	// Merge the two passes: sort by start and fuse ranges that touch or overlap
	// ("€2.2m" attached, then "2 mo" spaced, never share text, but "5k km" could).
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j][0] < spans[j-1][0]; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
	merged := spans[:1]
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s[0] <= last[1] {
			if s[1] > last[1] {
				last[1] = s[1]
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}

// UnitCaseTokens lists the number-unit tokens in a title whose case an
// all-caps template would otherwise change. It is what the renderer protects.
func UnitCaseTokens(text string) []string {
	var tokens []string
	for _, s := range unitCaseSpans(text) {
		tokens = append(tokens, text[s[0]:s[1]])
	}
	return tokens
}

// preserveUnitCase splits title runs so each number-unit token sits in a run
// carrying cap="none". Runs without such a token are returned untouched.
func preserveUnitCase(runs []runXML) []runXML {
	var out []runXML
	changed := false
	for _, run := range runs {
		spans := unitCaseSpans(run.Text)
		if len(spans) == 0 {
			out = append(out, run)
			continue
		}
		changed = true
		piece := func(text string, keepCase bool) {
			if text == "" {
				return
			}
			props := runPropertiesXML{Lang: "en-US"}
			if run.RunProperties != nil {
				props = *run.RunProperties
			}
			if keepCase {
				props.Caps = "none"
			}
			out = append(out, runXML{RunProperties: &props, Text: text})
		}
		pos := 0
		for _, s := range spans {
			piece(run.Text[pos:s[0]], false)
			piece(run.Text[s[0]:s[1]], true)
			pos = s[1]
		}
		piece(run.Text[pos:], false)
	}
	if !changed {
		return runs
	}
	return out
}

// isHeadlineShape reports whether a shape is a title or subtitle — the
// placeholders templates set in capitals.
func isHeadlineShape(shape *shapeXML, placeholderID string) bool {
	if isTitleShape(shape) || isTitlePlaceholder(placeholderID) {
		return true
	}
	id := strings.ToLower(strings.TrimSpace(placeholderID))
	if id == "subtitle" || strings.HasPrefix(id, "subtitle_") {
		return true
	}
	ph := shape.NonVisualProperties.NvPr.Placeholder
	return ph != nil && ph.Type == "subTitle"
}
