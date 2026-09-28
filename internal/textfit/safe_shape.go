package textfit

import (
	"unicode/utf8"

	"github.com/tdewolff/canvas"
)

// fallbackGlyphEm is the average advance of one glyph, as a fraction of the
// font size, used when shaping fails. 0.55em sits between the average Latin
// lowercase advance (~0.5em) and the wider CJK / emoji advance (~1em), so a
// fallback measurement errs slightly wide: text that falls back wraps a
// little early rather than overflowing.
const fallbackGlyphEm = 0.55

// LineWidthMM returns the rendered width, in millimetres, of s set on one
// line in face. It is canvas.NewTextLine(face, s, canvas.Left).Bounds().W()
// with a panic guard: the HarfBuzz shaper in tdewolff/canvas panics (slice
// bounds out of range in text/harfbuzz.go) on some adversarial rune
// sequences, e.g. a bidi override followed by an emoji and a combining mark
// (go-slide-creator-7oz3c). A text measurement must never take the process
// down, so on a panic the width falls back to an average-glyph-width
// estimate of the rune count.
//
// Every canvas.NewTextLine call that measures author-supplied text should go
// through this function.
func LineWidthMM(face *canvas.FontFace, s string) float64 {
	if face == nil {
		return 0
	}
	if w, ok := tryLineWidthMM(face, s); ok {
		return w
	}
	return EstimateLineWidthMM(face, s)
}

func tryLineWidthMM(face *canvas.FontFace, s string) (width float64, ok bool) {
	defer func() {
		if recover() != nil {
			width, ok = 0, false
		}
	}()
	return canvas.NewTextLine(face, s, canvas.Left).Bounds().W(), true
}

// EstimateLineWidthMM is the shaping-free width estimate LineWidthMM falls
// back to: rune count times an average glyph advance of fallbackGlyphEm.
func EstimateLineWidthMM(face *canvas.FontFace, s string) float64 {
	if face == nil {
		return 0
	}
	return float64(utf8.RuneCountInString(s)) * fallbackGlyphEm * face.Size
}
