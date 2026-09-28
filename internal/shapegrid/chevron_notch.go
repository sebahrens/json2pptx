package shapegrid

import "github.com/sebahrens/json2pptx/internal/pptx"

// chevronMaxNotchFraction caps the depth of each chevron notch as a fraction
// of the shape width. The preset default (adj 50000) sets the notch to half
// the shorter side, so a chevron less than twice as wide as it is tall loses
// its whole text rectangle to the two notches and wraps labels mid-word
// ("Dis/cov/er", go-slide-creator-5wm83). A 25% cap keeps at least half the
// width for text while leaving wide chevrons at the preset default.
const chevronMaxNotchFraction = 0.25

// DefaultChevronAdj returns the "adj" value a chevron of the given size gets
// when the author set none: the preset 50000, reduced for stubby chevrons so
// each notch is at most chevronMaxNotchFraction of the width. ok is false for
// other geometries and for degenerate bounds.
func DefaultChevronAdj(geometry string, cx, cy int64) (adj int64, ok bool) {
	if geometry != string(pptx.GeomChevron) || cx <= 0 || cy <= 0 {
		return 0, false
	}
	ss := min(cx, cy)
	capped := int64(chevronMaxNotchFraction * 100000 * float64(cx) / float64(ss))
	return min(int64(50000), capped), true
}
