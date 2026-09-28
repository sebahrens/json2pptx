package svggen

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/svggen/core"
	"github.com/tdewolff/canvas"
)

// Missing-glyph handling (go-slide-creator-s27x0).
//
// svggen measures and draws text with a single embedded face. Characters that
// face has no glyph for (CJK, emoji, Hebrew/Arabic, ...) resolve to .notdef:
// the measured width was the .notdef advance, so wrap, overlap and truncation
// decisions for non-Latin labels were wrong, and nothing told the author the
// output depends on whatever font the viewer substitutes. MeasureText now
// estimates those characters' advances, and every draw records them so the
// render reports FindingGlyphMissing.

// Estimated advances, in ems, for characters the face cannot shape. East
// Asian wide characters and emoji occupy a full em in every common fallback
// font; other scripts average a little over half an em.
const (
	missingGlyphWideEm   = 1.0
	missingGlyphNarrowEm = 0.6
	// maxMissingGlyphSamples caps the characters quoted in the finding.
	maxMissingGlyphSamples = 12
)

// isIgnorableForGlyphCheck reports runes that legitimately have no glyph of
// their own: whitespace, controls, joiners and variation selectors.
func isIgnorableForGlyphCheck(r rune) bool {
	if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r) {
		return true
	}
	return r >= 0xFE00 && r <= 0xFE0F // variation selectors
}

// isWideRune reports East Asian wide/fullwidth characters and emoji.
func isWideRune(r rune) bool {
	switch {
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo):
		return true
	case r >= 0x3000 && r <= 0x303F, // CJK symbols and punctuation
		r >= 0xFF00 && r <= 0xFFEF,   // halfwidth and fullwidth forms
		r >= 0x1F000 && r <= 0x1FAFF, // emoji, pictographs, symbols
		r >= 0x2600 && r <= 0x27BF:   // misc symbols and dingbats
		return true
	}
	return false
}

// missingGlyphRunes returns the runes of text the face has no glyph for.
func missingGlyphRunes(face *canvas.FontFace, text string) []rune {
	if face == nil || face.Font == nil || face.Font.SFNT == nil {
		return nil
	}
	var missing []rune
	for _, r := range text {
		if r < 0x80 || isIgnorableForGlyphCheck(r) {
			continue // the embedded faces always cover ASCII
		}
		if face.Font.GlyphIndex(r) == 0 {
			missing = append(missing, r)
		}
	}
	return missing
}

// estimateMissingGlyphWidth returns the measured width of text with its
// missing glyphs removed plus an estimated advance for each missing glyph, in
// points. ok is false when text has no missing glyphs.
func estimateMissingGlyphWidth(face *canvas.FontFace, text string, fontSizePt float64) (width float64, ok bool) {
	missing := missingGlyphRunes(face, text)
	if len(missing) == 0 {
		return 0, false
	}
	missingSet := make(map[rune]bool, len(missing))
	for _, r := range missing {
		missingSet[r] = true
	}
	var kept strings.Builder
	est := 0.0
	for _, r := range text {
		if !missingSet[r] {
			kept.WriteRune(r)
			continue
		}
		if isWideRune(r) {
			est += missingGlyphWideEm * fontSizePt
		} else {
			est += missingGlyphNarrowEm * fontSizePt
		}
	}
	if s := kept.String(); s != "" {
		// Shaping can still panic on the kept runes (bidi controls + combining
		// marks); fall back to the same 0.55em estimate MeasureText uses.
		if tl := safeTextLine(face, s, canvas.Left); tl != nil {
			width = tl.Bounds().W() * mmToPt
		} else {
			width = float64(len([]rune(s))) * 0.55 * fontSizePt
		}
	}
	return width + est, true
}

// noteMissingGlyphs records the characters of a drawn string the face cannot
// render, for glyphMissingFindings.
func (b *SVGBuilder) noteMissingGlyphs(face *canvas.FontFace, text string) {
	missing := missingGlyphRunes(face, text)
	if len(missing) == 0 {
		return
	}
	if b.missingGlyphs == nil {
		b.missingGlyphs = make(map[rune]bool)
	}
	for _, r := range missing {
		b.missingGlyphs[r] = true
	}
	b.missingGlyphTexts++
}

// glyphMissingFindings reports, once per render, the characters drawn that
// the embedded chart font has no glyph for.
func (b *SVGBuilder) glyphMissingFindings() []core.Finding {
	if len(b.missingGlyphs) == 0 {
		return nil
	}
	runes := make([]rune, 0, len(b.missingGlyphs))
	for r := range b.missingGlyphs {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })

	scriptSet := map[string]bool{}
	for _, r := range runes {
		scriptSet[runeScript(r)] = true
	}
	scripts := make([]string, 0, len(scriptSet))
	for s := range scriptSet {
		scripts = append(scripts, s)
	}
	sort.Strings(scripts)

	samples := runes
	if len(samples) > maxMissingGlyphSamples {
		samples = samples[:maxMissingGlyphSamples]
	}
	chars := make([]string, len(samples))
	for i, r := range samples {
		chars[i] = string(r)
	}
	return []core.Finding{{
		Code: core.FindingGlyphMissing,
		Message: fmt.Sprintf(
			"%d text item(s) use %d character(s) the embedded chart font has no glyph for (%s; scripts: %s); their widths are estimated, so wrapping and truncation are approximate, and the viewer substitutes a system font — check the rendered output",
			b.missingGlyphTexts, len(runes), strings.Join(chars, " "), strings.Join(scripts, ", ")),
		Severity: core.SeverityWarning,
		Fix: &core.FixSuggestion{
			Kind: core.FixKindReplaceValue,
			Params: map[string]any{
				"characters":    chars,
				"missing_count": len(runes),
				"scripts":       scripts,
			},
		},
	}}
}

// runeScript names the script of r for the finding message.
func runeScript(r rune) string {
	for _, s := range []struct {
		name  string
		table *unicode.RangeTable
	}{
		{"Han", unicode.Han}, {"Hiragana", unicode.Hiragana}, {"Katakana", unicode.Katakana},
		{"Hangul", unicode.Hangul}, {"Hebrew", unicode.Hebrew}, {"Arabic", unicode.Arabic},
		{"Cyrillic", unicode.Cyrillic}, {"Greek", unicode.Greek}, {"Thai", unicode.Thai},
		{"Devanagari", unicode.Devanagari},
	} {
		if unicode.Is(s.table, r) {
			return s.name
		}
	}
	if isWideRune(r) {
		return "emoji/symbols"
	}
	return "other"
}

var (
	svgTspanRe       = regexp.MustCompile(`(?s)<tspan\b([^>]*)>(.*?)</tspan>`)
	svgTspanYAttr    = regexp.MustCompile(`\by="(-?[\d.]+)"`)
	svgDirectionDecl = regexp.MustCompile(`;?\s*direction:\s*rtl\s*`)
	svgEmptyStyle    = regexp.MustCompile(`\s*style="\s*;?\s*"`)
)

// stripRTLDirection removes the canvas library's per-run direction:rtl
// declaration. SVG ties text-anchor to direction, so on a left-aligned RTL
// label "start" became the RIGHT edge and the label hung left of its anchor.
// Without it the viewer's bidi algorithm still orders the glyphs correctly
// inside an LTR-anchored run.
func stripRTLDirection(attrs []byte) []byte {
	attrs = svgDirectionDecl.ReplaceAll(attrs, nil)
	return svgEmptyStyle.ReplaceAll(attrs, nil)
}

// mergeSplitTspans collapses a <text> element whose single line the canvas
// library split into several <tspan>s (one per script/direction run, e.g.
// Han then Katakana) into one <tspan> carrying the whole string. Each tspan
// kept its own absolute x, and once fixSVGTextAlignment gave the element
// text-anchor="middle" every tspan became its own anchored chunk: the second
// run of "日本語テキスト" was drawn centred LEFT of the first. One tspan lets
// the viewer shape and bidi-order the string as a unit. The original logical
// string recorded at draw time is used when it matches the runs' characters.
func (b *SVGBuilder) mergeSplitTspans(svgContent []byte) []byte {
	idx := 0
	return svgTextBlockRe.ReplaceAllFunc(svgContent, func(block []byte) []byte {
		i := idx
		idx++
		spans := svgTspanRe.FindAllSubmatchIndex(block, -1)
		if len(spans) == 1 {
			attrs := block[spans[0][2]:spans[0][3]]
			if !bytes.Contains(attrs, []byte("direction")) {
				return block
			}
			var out bytes.Buffer
			out.Write(block[:spans[0][2]])
			out.Write(stripRTLDirection(attrs))
			out.Write(block[spans[0][3]:])
			return out.Bytes()
		}
		if len(spans) < 2 {
			return block
		}
		var firstY []byte
		var joined strings.Builder
		for k, sp := range spans {
			attrs := block[sp[2]:sp[3]]
			y := svgTspanYAttr.FindSubmatch(attrs)
			if y == nil {
				return block
			}
			if k == 0 {
				firstY = y[1]
			} else if !bytes.Equal(y[1], firstY) {
				return block // genuinely multi-line; leave it alone
			}
			joined.WriteString(html.UnescapeString(string(block[sp[4]:sp[5]])))
		}
		text := joined.String()
		if i < len(b.textStrings) && sameRunes(b.textStrings[i], text) {
			text = b.textStrings[i]
		}
		firstAttrs := stripRTLDirection(block[spans[0][2]:spans[0][3]])
		var out bytes.Buffer
		out.Write(block[:spans[0][0]])
		out.WriteString("<tspan")
		out.Write(firstAttrs)
		out.WriteString(">")
		out.WriteString(html.EscapeString(text))
		out.WriteString("</tspan>")
		out.Write(block[spans[len(spans)-1][1]:])
		return out.Bytes()
	})
}

// sameRunes reports whether a and b contain the same multiset of runes.
func sameRunes(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) {
		return false
	}
	sort.Slice(ra, func(i, j int) bool { return ra[i] < ra[j] })
	sort.Slice(rb, func(i, j int) bool { return rb[i] < rb[j] })
	return string(ra) == string(rb)
}
