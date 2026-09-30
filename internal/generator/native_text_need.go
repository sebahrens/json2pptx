package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Sizing native diagram cells to their text (go-slide-creator-zbo58).
//
// The native diagram builders used to cut their cells by fixed ratios (a BMC
// top row of 60%, SWOT headers of 18%, Porter's peripheral boxes of 25%) and
// let the stored autofit shrink whatever did not fit. On a short content area
// (abstract, modern) that shrink reached 20-60%: four BMC bullets written at
// 10pt and stored at 2pt, which generation then correctly refused. The layouts
// below measure each cell's text with the writer's own estimator and hand the
// space to the cells that need it, before any shrink is stored.

// nativeCardSeamInsetEMU is the text inset on the side of a card body that
// meets its own header. A BMC / SWOT / nine-box card is drawn as a header shape
// stacked directly on a body shape of the same fill, so that seam is not an
// edge a reader sees: the uniform 0.5 cm margin still holds on every visible
// edge of the card, but repeating it at the seam put 1 cm of dead space between
// a header and its first bullet. The seam keeps the OOXML default top inset
// (0.05in).
const nativeCardSeamInsetEMU int64 = 45720

// nativeCardBodyInsets are the insets of a card body under a same-fill header:
// the uniform margin on the three visible edges, the seam inset on top.
func nativeCardBodyInsets() [4]int64 {
	return [4]int64{pptx.ShapeTextInsetEMU, nativeCardSeamInsetEMU, pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU}
}

// nativeCardHeaderInsets are the insets of a card header above a same-fill
// body: the uniform margin on the three visible edges, the seam inset below.
func nativeCardHeaderInsets() [4]int64 {
	return [4]int64{pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU, nativeCardSeamInsetEMU}
}

// nativeTextNeedEMU returns the smallest shape height at which tb, set in a
// shape of the given width, needs no autofit shrink. It measures with
// pptx.AutofitScaleFor after the EffectiveTextInsets clamp GenerateShape
// applies, so the height it reports is the one the writer will not shrink.
// It returns limit when the text does not fit even there, and 0 for a body
// with no text.
func nativeTextNeedEMU(tb pptx.TextBody, width, limit int64) int64 {
	if width <= 0 || limit <= 0 || len(tb.Paragraphs) == 0 {
		return 0
	}
	fits := func(h int64) bool {
		probe := tb
		bounds := pptx.RectEmu{CX: width, CY: h}
		probe.Insets = pptx.EffectiveTextInsets(&probe, bounds)
		return pptx.AutofitFitsFor(&probe, bounds)
	}
	if !fits(limit) {
		return limit
	}
	lo, hi := int64(0), limit
	for hi-lo > int64(types.EMUPerPoint)/2 {
		mid := lo + (hi-lo)/2
		if fits(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// splitByNeed divides total between two stacked parts in proportion to what
// each needs, keeping each part's share within [minShare, 1-minShare] so a
// sparse part never collapses. With no measured need it splits evenly.
func splitByNeed(total, needA, needB int64, minShare float64) (int64, int64) {
	share := 0.5
	if needA+needB > 0 {
		share = float64(needA) / float64(needA+needB)
	}
	share = min(max(share, minShare), 1-minShare)
	a := int64(float64(total) * share)
	return a, total - a
}
