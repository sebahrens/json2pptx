package shapegrid

import (
	"encoding/json"
	"strings"
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

// A shared shrink is written as text sizes, which every renderer draws alike;
// LibreOffice ignores a stored fontScale and would shrink only the long label
// (go-slide-creator-5x4w4). A row with text the shrink would take under the
// size floor keeps the stored scale, every cell of it.
func TestWriteSharedShrink(t *testing.T) {
	cell := func(row int, text json.RawMessage, scale float64) ResolvedCell {
		return ResolvedCell{
			Kind:         CellKindShape,
			RowIdx:       row,
			ID:           2,
			Bounds:       pptx.RectEmu{CX: 200 * 12700, CY: 80 * 12700},
			ShapeSpec:    &ShapeSpec{Geometry: "rect", Text: text},
			AutofitScale: scale,
		}
	}
	value := json.RawMessage(`{"paragraphs":[{"content":"$4.2","size":40,"suffix":"M","suffix_size":20},{"content":"ARR","size":14}]}`)
	small := json.RawMessage(`{"content":"Label","size":12}`)
	cells := []ResolvedCell{
		cell(0, value, 0.9), cell(0, value, 0.9),
		cell(1, small, 0.9), cell(1, value, 0.9),
		cell(2, json.RawMessage(`"Plain"`), 0),
	}
	authored := cells[0].ShapeSpec
	writeSharedShrink(cells)

	if cells[0].AutofitScale != 0 || cells[1].AutofitScale != 0 {
		t.Errorf("shrunk row keeps stored scales %v, %v, want none", cells[0].AutofitScale, cells[1].AutofitScale)
	}
	if string(authored.Text) != string(value) {
		t.Error("the shrink mutated the spec it was resolved from")
	}
	xml, err := GenerateCellShapeXML(cells[0])
	if err != nil || strings.Contains(string(xml), "fontScale") {
		t.Errorf("shrunk cell XML (err %v) must carry no fontScale: %s", err, xml)
	}
	for _, sz := range []string{`sz="3600"`, `sz="1800"`, `sz="1260"`} {
		if !strings.Contains(string(xml), sz) {
			t.Errorf("shrunk cell XML lacks %s (value 40, suffix 20 and caption 14 at 90%%): %s", sz, xml)
		}
	}
	if cells[2].AutofitScale != 0.9 || string(cells[2].ShapeSpec.Text) != string(small) {
		t.Errorf("12pt text cannot shrink by size: want the stored scale kept, got scale %v text %s", cells[2].AutofitScale, cells[2].ShapeSpec.Text)
	}
	if cells[3].AutofitScale != 0.9 || string(cells[3].ShapeSpec.Text) != string(value) {
		t.Errorf("the sibling of a cell that keeps its stored scale must keep it too, got scale %v text %s", cells[3].AutofitScale, cells[3].ShapeSpec.Text)
	}
	if cells[4].AutofitScale != 0 || string(cells[4].ShapeSpec.Text) != `"Plain"` {
		t.Error("a cell with no shared shrink must be untouched")
	}
}
