package patterns

import (
	"encoding/json"
	"math"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// writtenNeedOrOverflowPt is writtenFitHeightPt for row sizing: the written
// fit of text in a shape widthPt wide. writtenFitHeightPt returns its minPt
// when the text still shrinks after its 400pt search window, so a row sized
// from it would read an overflowing cell as needing nothing; here such text
// needs at least that window — or the theme-font estimate when larger — so
// it can never pass a fit check (go-slide-creator-n1muf). font is the theme
// body font, which the written fit measures in as the writer does
// (go-slide-creator-ohhb2); heading-font text falls back to Liberation Sans.
func writtenNeedOrOverflowPt(font string, text json.RawMessage, widthPt float64) float64 {
	if h := writtenFitHeightPt(pptx.ThemeFonts{Minor: font}, text, widthPt, 0); h > 0 || len(text) == 0 {
		return h
	}
	return math.Max(writtenFitOverflowPt, shapeTextHeightPt(font, text, widthPt-2*defaultShapeInsetLRPt)+2*defaultShapeInsetTBPt)
}

// writtenFitOverflowPt is the height past writtenFitHeightPt's search window.
const writtenFitOverflowPt = 401.0
