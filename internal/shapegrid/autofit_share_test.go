package shapegrid

import (
	"encoding/json"
	"fmt"
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
	// The cell of the other row stands apart: another size, no shared edge.
	other := cell(1, "Other row")
	other.Bounds = pptx.RectEmu{X: 400 * 12700, Y: 300 * 12700, CX: 240 * 12700, CY: 70 * 12700}
	cells := []ResolvedCell{cell(0, "Short"), cell(0, long), other}
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

// Peers beyond the row share the shrink too (go-slide-creator-riyh7): the
// cells of one column of a heatmap are one set, and a shrink taken row by row
// set the row with the long activity smaller than the rows around it.
func TestSharePeerAutofitScaleAcrossRows(t *testing.T) {
	cell := func(row int, text string) ResolvedCell {
		raw, _ := json.Marshal(map[string]any{"content": text, "size": 14})
		return ResolvedCell{
			Kind:      CellKindShape,
			RowIdx:    row,
			Bounds:    pptx.RectEmu{X: 0, Y: int64(row) * 50 * 12700, CX: 90 * 12700, CY: 40 * 12700},
			ShapeSpec: &ShapeSpec{Geometry: "rect", Text: raw},
		}
	}
	long := "A much longer label that wraps onto several lines in this narrow box"
	cells := []ResolvedCell{cell(0, "Short"), cell(1, long), cell(2, "Brief")}
	// A cell of another geometry in the long cell's column is no peer of it.
	pill := cell(3, "Pill")
	pill.ShapeSpec.Geometry = "roundRect"
	cells = append(cells, pill)
	shareRowAutofitScale(cells)
	if s := cells[1].AutofitScale; s <= 0 || s >= 1 {
		t.Fatalf("long label scale = %v, want a shrink", s)
	}
	if cells[0].AutofitScale != cells[1].AutofitScale || cells[2].AutofitScale != cells[1].AutofitScale {
		t.Errorf("column peers differ: %v, %v, %v", cells[0].AutofitScale, cells[1].AutofitScale, cells[2].AutofitScale)
	}
	if cells[3].AutofitScale != 0 {
		t.Errorf("a shape of another geometry is not a peer, got scale %v", cells[3].AutofitScale)
	}
	writeSharedShrink(cells)
	sizes := map[string]bool{}
	for _, c := range cells[:3] {
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil {
			t.Fatal(err)
		}
		sizes[fmt.Sprint(firstRunSizeHP(tb), c.AutofitScale)] = true
	}
	if len(sizes) != 1 {
		t.Errorf("column peers are written at several sizes: %v", sizes)
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

// A cell with no same-size sibling in its row shares no scale, so the writer
// used to store its shrink on the shape: an authored axis label "4" at 24pt in
// a 27pt-tall box was 92% in PowerPoint and re-fitted by LibreOffice. Its
// shrink is written into its sizes too; text that would fall under the 12pt
// floor keeps the stored scale (go-slide-creator-217cd).
func TestWriteSharedShrinkWritesALoneCell(t *testing.T) {
	lone := func(text string, hPt float64) ResolvedCell {
		return ResolvedCell{
			Kind:      CellKindShape,
			ID:        2,
			Bounds:    pptx.RectEmu{CX: 74 * 12700, CY: int64(hPt * 12700)},
			ShapeSpec: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)},
		}
	}
	const figure = `{"content":"4","size":24,"bold":true,"align":"ctr"}`
	const floor = `{"content":"A label that needs three lines here","size":12}`
	cells := []ResolvedCell{lone(figure, 27), lone(floor, 30), lone(figure, 60)}
	shrunk, kept, roomy := &cells[0], &cells[1], &cells[2]
	before, err := GenerateCellShapeXML(*shrunk)
	if err != nil || !strings.Contains(string(before), "fontScale") {
		t.Fatalf("fixture: the 24pt figure must need a shrink in a 27pt box (err %v): %s", err, before)
	}
	writeSharedShrink(cells)

	xml, err := GenerateCellShapeXML(*shrunk)
	if err != nil || strings.Contains(string(xml), "fontScale") {
		t.Errorf("lone cell XML (err %v) must carry no fontScale: %s", err, xml)
	}
	tb, err := ResolveTextInput(shrunk.ShapeSpec.Text)
	if err != nil {
		t.Fatal(err)
	}
	if got := tb.Paragraphs[0].Runs[0].FontSize; got >= 2400 || got < 1200 {
		t.Errorf("lone figure written at %d hundredths of a point, want its shrink in the size (under 2400, at least 1200)", got)
	}
	if string(kept.ShapeSpec.Text) != floor {
		t.Errorf("12pt text cannot shrink by size and must keep its text, got %s", kept.ShapeSpec.Text)
	}
	if xml, err := GenerateCellShapeXML(*kept); err != nil || !strings.Contains(string(xml), "fontScale") {
		t.Errorf("12pt text that does not fit keeps the stored scale (err %v): %s", err, xml)
	}
	if string(roomy.ShapeSpec.Text) != figure {
		t.Errorf("a cell that fits must be untouched, got %s", roomy.ShapeSpec.Text)
	}
}
