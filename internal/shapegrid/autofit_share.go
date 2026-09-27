package shapegrid

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Sibling cells share one autofit shrink (go-slide-creator-csclk.99).
//
// Every shape_grid text body is normAutofit and each shape measured its own
// fontScale, so in a row of timeline labels, waterfall labels or option-matrix
// cells the one long label shrank alone and the row rendered at uneven sizes.
// Cells in the same row that declare the same font size are siblings: they take
// the smallest shrink any of them needs.

// shareRowAutofitScale sets AutofitScale on same-size sibling shape cells of
// each row to the smallest shrink the group needs. Groups where every cell fits
// are left alone (AutofitScale 0), so a row that needs no shrink is unchanged.
func shareRowAutofitScale(cells []ResolvedCell) {
	type key struct {
		row    int
		sizeHP int
	}
	type member struct {
		idx   int
		scale float64
	}
	groups := map[key][]member{}
	for i := range cells {
		c := &cells[i]
		if c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil || tb.AutoFit != "normAutofit" {
			continue
		}
		size := firstRunSizeHP(tb)
		if size <= 0 {
			continue
		}
		for j := 0; j < 4; j++ {
			tb.Insets[j] += c.TextInsets[j]
		}
		k := key{row: c.RowIdx, sizeHP: size}
		groups[k] = append(groups[k], member{idx: i, scale: pptx.AutofitScaleFor(tb, c.Bounds)})
	}
	for _, ms := range groups {
		if len(ms) < 2 {
			continue
		}
		minScale := 1.0
		for _, m := range ms {
			if m.scale < minScale {
				minScale = m.scale
			}
		}
		if minScale >= 1 {
			continue
		}
		for _, m := range ms {
			cells[m.idx].AutofitScale = minScale
		}
	}
}

// firstRunSizeHP returns the font size (hundredths of a point) of the body's
// first sized run, or 0 when none declares one.
func firstRunSizeHP(tb *pptx.TextBody) int {
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if r.Text != "" && r.FontSize > 0 {
				return r.FontSize
			}
		}
	}
	return 0
}

// GenerateCellShapeXML renders a resolved shape cell, applying the row-shared
// autofit shrink when one was assigned.
func GenerateCellShapeXML(cell ResolvedCell) ([]byte, error) {
	return generateShapeXML(cell.ShapeSpec, cell.ID, cell.Bounds, cell.AutofitScale, cell.TextInsets)
}
