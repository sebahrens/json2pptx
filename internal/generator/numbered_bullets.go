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
// numbers run 1, 2, 3 …. That is deliberate: it is the shape an author writing
// an ordered list produces, and it cannot be reached by accident. A single line
// opening "2024. A big year" is prose, and stays prose; a list numbered 1, 2, 2
// is a mistake worth reporting rather than silently renumbering.

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
	if len(bullets) < 2 {
		// A one-item "ordered list" is a line that happens to start with a
		// number; numbering it would be a guess.
		return nil, false
	}
	stripped := make([]string, len(bullets))
	for i, bullet := range bullets {
		n, rest, ok := pptx.ParseNumberedPrefix(strings.TrimSpace(bullet))
		if !ok || n != i+1 || strings.TrimSpace(rest) == "" {
			return nil, false
		}
		stripped[i] = rest
	}
	return stripped, true
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
	if pProps == nil {
		return
	}
	inner := buMarkerRe.ReplaceAllString(pProps.Inner, "")
	const marker = `<a:buAutoNum type="arabicPeriod"/>`
	if loc := pPrTailRe.FindStringIndex(inner); loc != nil {
		pProps.Inner = inner[:loc[0]] + marker + inner[loc[0]:]
		return
	}
	pProps.Inner = inner + marker
}
