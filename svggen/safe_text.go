package svggen

import (
	"strings"
	"unicode"

	"github.com/tdewolff/canvas"
)

// safeTextLine is canvas.NewTextLine with a panic guard. The HarfBuzz shaper
// in tdewolff/canvas panics (slice bounds out of range in text/harfbuzz.go)
// on some adversarial rune sequences, e.g. a bidi control (U+202E, U+2067,
// U+200F) followed by an emoji and a combining mark
// (go-slide-creator-7oz3c). On a panic the text is retried with Unicode
// format characters (category Cf: bidi controls, BOM, zero-width joiners)
// removed, which is what triggers the bad cluster mapping. It returns nil
// when shaping still fails, so the caller can skip the draw instead of taking
// the process down.
func safeTextLine(face *canvas.FontFace, text string, align canvas.TextAlign) *canvas.Text {
	if t := tryTextLine(face, text, align); t != nil {
		return t
	}
	stripped := stripFormatRunes(text)
	if stripped == text {
		return nil
	}
	return tryTextLine(face, stripped, align)
}

func tryTextLine(face *canvas.FontFace, text string, align canvas.TextAlign) (t *canvas.Text) {
	defer func() {
		if recover() != nil {
			t = nil
		}
	}()
	return canvas.NewTextLine(face, text, align)
}

// stripFormatRunes removes Unicode format characters (category Cf).
func stripFormatRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}
