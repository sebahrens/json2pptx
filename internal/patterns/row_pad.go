package patterns

import "math"

// rowPadStepsPt are the top / bottom text margins the rows of a ruled list or
// table step down to when the list does not fit its content area at the
// uniform 0.5 cm shape margin, largest first. A six-row table is set the way
// a native table is: the type stays at a readable size and the air inside each
// row gives way first. The row cells these apply to are unfilled (or a row
// tint that runs edge to edge), so the margin is row padding between hairline
// rules rather than the distance of text from a drawn box. The house builder
// uses the same steps for its one-line bands, which become slimmer bars before
// the gable is flattened. Left and right margins are never touched, and a
// layout that fits at the uniform margin never takes a step. The last step is
// a little above PowerPoint's own table cell margin (0.05 in)
// (go-slide-creator-vg73u).
var rowPadStepsPt = []float64{10, 7, 5}

// rowPadTrimPt is how much shorter a text block is at a top / bottom margin of
// padPt than at the uniform margin; padPt 0 means the uniform margin.
func rowPadTrimPt(padPt float64) float64 {
	if padPt <= 0 || padPt >= sizingInsetTBPt {
		return 0
	}
	return 2 * (sizingInsetTBPt - padPt)
}

// rowPadInsets returns the inset_top / inset_bottom values a row cell is
// written with at padPt, the top one pushed down by nudgePt: nil (the uniform
// margin) when padPt is 0.
func rowPadInsets(padPt, nudgePt float64) (top, bottom *float64) {
	if padPt <= 0 || padPt >= sizingInsetTBPt {
		return nil, nil
	}
	t, b := padPt+nudgePt, padPt
	return &t, &b
}

// spreadRowSlack shares the height a tightened list leaves spare in areaPt
// among its rows, a whole point at a time: the padding step that fits is
// usually tighter than the area needs, and the rows take the rest back rather
// than leaving it as a band of white under the list. A point is kept in hand so
// rounding never over-fills the area.
func spreadRowSlack(rowPt []float64, usedPt, areaPt float64) {
	if len(rowPt) == 0 {
		return
	}
	extra := math.Floor((areaPt - usedPt - 1) / float64(len(rowPt)))
	if extra <= 0 {
		return
	}
	for i := range rowPt {
		rowPt[i] += extra
	}
}
