package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Sibling labels in one row share the shrink the longest needs, so the row
// renders at one size (go-slide-creator-csclk.99).
func TestShareRowAutofitScale(t *testing.T) {
	cell := func(row int, text string) ResolvedCell {
		raw, _ := json.Marshal(map[string]any{"content": text, "size": 14})
		return ResolvedCell{
			Kind:      CellKindShape,
			RowIdx:    row,
			Bounds:    pptx.RectEmu{CX: 90 * 12700, CY: 40 * 12700},
			ShapeSpec: &ShapeSpec{Geometry: "rect", Text: raw},
		}
	}
	long := "A much longer label that wraps onto several lines in this narrow box"
	cells := []ResolvedCell{cell(0, "Short"), cell(0, long), cell(1, "Other row")}
	shareRowAutofitScale(cells)
	if s := cells[1].AutofitScale; s <= 0 || s >= 1 {
		t.Fatalf("long label scale = %v, want a shrink", s)
	}
	if cells[0].AutofitScale != cells[1].AutofitScale {
		t.Errorf("siblings differ: %v vs %v", cells[0].AutofitScale, cells[1].AutofitScale)
	}
	if cells[2].AutofitScale != 0 {
		t.Errorf("a lone cell in another row must keep its own measurement, got %v", cells[2].AutofitScale)
	}
}
