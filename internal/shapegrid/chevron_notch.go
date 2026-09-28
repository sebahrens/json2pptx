package shapegrid

import "github.com/sebahrens/json2pptx/internal/pptx"

// DefaultChevronAdj returns the "adj" value a chevron of the given size gets
// when the author set none: the preset 50000, reduced for stubby chevrons so
// each notch is at most chevronMaxNotchFraction of the width. ok is false for
// other geometries and for degenerate bounds.
func DefaultChevronAdj(geometry string, cx, cy int64) (adj int64, ok bool) {
	if geometry != string(pptx.GeomChevron) {
		return 0, false
	}
	return pptx.DefaultChevronAdj(cx, cy)
}
