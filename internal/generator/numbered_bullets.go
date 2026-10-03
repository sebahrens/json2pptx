package generator

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Numbered placeholder bullets (go-slide-creator-6or2).
//
// bullets_value ["1. First do this", "2. Then do that"] rendered as
// "• 1. First do this": the layout's own bullet glyph AND the author's typed
// number. OOXML auto-numbering was already implemented — pptx.BulletDef.AutoNum
// emits <a:buAutoNum> — but only the shape_grid text path ever asked for it, so
// the routine ordered list (steps, rankings, priorities) had no clean form in a
// placeholder at all.
//
// A list is treated as numbered only when EVERY bullet carries a prefix and the
// numbers count up by one. That is deliberate: it is the shape an author writing
// an ordered list produces, and it cannot be reached by accident. A single line
// opening "2024. A big year" is prose, and stays prose; a list numbered 1, 2, 2
// is a mistake worth reporting rather than silently renumbering.
//
// The list need not start at 1: steps 4–6 continued from the previous slide are
// an ordered list too (go-slide-creator-zdzk2). Before, they printed as
// "• 4. …". They keep their typed numbers and lose the layout's glyph instead
// of being auto-numbered with startAt, because renderers disagree about
// startAt (see pptx.NumberedRuns) and the typed number is the one form they
// all draw the same.

// numberedListPrefix matches the "N. " opening of an ordered-list line.
var numberedListPrefix = regexp.MustCompile(`^\d+\.\s+`)

// HasNumberedPrefix reports whether a line opens with an "N. " marker. It is
// the same test the renderer applies, so validation can say what the renderer
// will do.
func HasNumberedPrefix(line string) bool {
	return numberedListPrefix.MatchString(strings.TrimSpace(line))
}

// NumberedList reports whether a bullet list is an ordered list the renderer
// will auto-number: every entry carries a prefix and the numbers ascend from 1
// by one. It returns the texts with their prefixes stripped, which is what the
// renderer writes once OOXML supplies the numbers.
func NumberedList(bullets []string) ([]string, bool) {
	stripped, start, ok := NumberedListStart(bullets)
	if !ok || start != 1 {
		return nil, false
	}
	return stripped, true
}

// NumberedListStart reports whether a bullet list is an ordered list — every
// entry carries a prefix and the numbers ascend by one — and the number it
// starts at. A list from 1 is auto-numbered (NumberedList); one that starts
// higher renders with its typed numbers in place of the layout's glyph.
func NumberedListStart(bullets []string) (stripped []string, start int, ok bool) {
	if len(bullets) < 2 {
		// A one-item "ordered list" is a line that happens to start with a
		// number; numbering it would be a guess.
		return nil, 0, false
	}
	stripped = make([]string, len(bullets))
	for i, bullet := range bullets {
		n, rest, ok := pptx.ParseNumberedPrefix(strings.TrimSpace(bullet))
		if i == 0 {
			start = n
		}
		if !ok || n != start+i || strings.TrimSpace(rest) == "" {
			return nil, 0, false
		}
		stripped[i] = rest
	}
	if start < 1 {
		return nil, 0, false
	}
	return stripped, start, true
}

// buMarkerRe matches the bullet-marker elements a paragraph may inherit; the
// marker is exactly one of buNone / buChar / buAutoNum, so an auto-numbered
// paragraph must drop whatever it inherited first.
var buMarkerRe = regexp.MustCompile(`<a:buNone\s*/>|<a:buChar[^>]*/>|<a:buAutoNum[^>]*/>`)

// pPrTailRe matches the first child element that must follow the bullet marker
// in CT_TextParagraphProperties' sequence, so the marker is inserted in a valid
// position rather than appended after defRPr.
var pPrTailRe = regexp.MustCompile(`<a:tabLst|<a:defRPr|<a:extLst`)

// applyAutoNumbering rewrites a paragraph's properties to use OOXML
// auto-numbering instead of the glyph it inherited from the layout.
func applyAutoNumbering(pProps *paragraphPropertiesXML) {
	setBulletMarker(pProps, `<a:buAutoNum type="arabicPeriod"/>`)
}

// applyTypedNumbering drops the glyph a paragraph inherited from the layout,
// so the number the author typed is its only marker.
func applyTypedNumbering(pProps *paragraphPropertiesXML) {
	setBulletMarker(pProps, `<a:buNone/>`)
}

// setBulletMarker replaces a paragraph's bullet marker with the given one.
func setBulletMarker(pProps *paragraphPropertiesXML, marker string) {
	if pProps == nil {
		return
	}
	inner := buMarkerRe.ReplaceAllString(pProps.Inner, "")
	if loc := pPrTailRe.FindStringIndex(inner); loc != nil {
		pProps.Inner = inner[:loc[0]] + marker + inner[loc[0]:]
		return
	}
	pProps.Inner = inner + marker
}
