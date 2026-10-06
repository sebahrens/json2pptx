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

// chevronBleedTolEMU is how far a chevron's bleed may exceed its notch before
// chevronBleedEMU holds it to the notch: a quarter point, the rounding of an
// "adj" computed from a notch in points.
const chevronBleedTolEMU = 12700 / 4

// chevronBleedEMU is the left bleed of a cell whose shape is spec, w wide
// and h tall before the bleed. An interlocking chevron bleeds by the depth of
// its own notch, so the notch sits one column gap off the point before it;
// the notch is the chevron's "adj" share of its shorter side, so it deepens
// when the composition policy grows the row while the bleed is a fixed
// length. Scaled with the row (scaledGrid) the bleed would overshoot once the
// chevron is taller than wide and its notch stops growing, so a bleed deeper
// than the notch drawn is held to that notch (go-slide-creator-cuq95). Any
// other shape, and a chevron with no authored "adj", keeps its bleed.
func chevronBleedEMU(spec *ShapeSpec, bleed, w, h int64) int64 {
	if bleed <= 0 || spec == nil || spec.Geometry != string(pptx.GeomChevron) || w <= 0 || h <= 0 {
		return bleed
	}
	adj := spec.Adjustments["adj"]
	if adj <= 0 || adj >= 100000 {
		return bleed
	}
	a := float64(adj) / 100000
	notch := a * float64(h)
	if float64(h) > float64(w)+notch {
		// The bled width is the shorter side: notch = a·(w + notch).
		notch = a * float64(w) / (1 - a)
	}
	if float64(bleed) > notch+chevronBleedTolEMU {
		return int64(notch)
	}
	return bleed
}
