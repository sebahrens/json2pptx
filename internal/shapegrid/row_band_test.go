package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"
)

// A row band is one rectangle as wide as the grid and as tall as its row. It
// takes no height and no cell, so the row's cells and their addresses are the
// same with and without it (go-slide-creator-vx7wk).
func TestResolveRowBands(t *testing.T) {
	build := func(band json.RawMessage) *Grid {
		return &Grid{
			Bounds:  DefaultBounds(0, 0),
			Columns: []float64{30, 70},
			RowGap:  6,
			Rows: []Row{
				{Band: band, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}, {Shape: &ShapeSpec{Geometry: "rect"}}}},
				{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}, {Shape: &ShapeSpec{Geometry: "rect"}}}},
			},
		}
	}
	fill := json.RawMessage(`{"color":"dk1","lumMod":4000,"lumOff":96000}`)
	grid := build(fill)
	banded, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Resolve(build(nil), newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Bands) != 0 {
		t.Errorf("a grid without bands resolved %d", len(plain.Bands))
	}
	if len(banded.Cells) != len(plain.Cells) {
		t.Fatalf("the band changed the cell count: %d vs %d", len(banded.Cells), len(plain.Cells))
	}
	for i := range banded.Cells {
		if banded.Cells[i].Bounds != plain.Cells[i].Bounds || banded.Cells[i].RowIdx != plain.Cells[i].RowIdx || banded.Cells[i].ColIdx != plain.Cells[i].ColIdx {
			t.Errorf("cell %d moved: %+v vs %+v", i, banded.Cells[i], plain.Cells[i])
		}
	}
	if len(banded.Bands) != 1 {
		t.Fatalf("%d bands, want one (row 0 only)", len(banded.Bands))
	}
	band, first := banded.Bands[0], banded.Cells[0]
	if band.Bounds.X != grid.Bounds.X || band.Bounds.CX != grid.Bounds.CX {
		t.Errorf("band = %+v, want the grid's full width %+v", band.Bounds, grid.Bounds)
	}
	if band.Bounds.Y != first.CellBounds.Y || band.Bounds.CY != first.CellBounds.CY {
		t.Errorf("band = %+v, want the row's own rectangle (y %d, cy %d)", band.Bounds, first.CellBounds.Y, first.CellBounds.CY)
	}
	xml, err := GenerateAccentBarXML(&banded.Bands[0])
	if err != nil {
		t.Fatalf("the band does not render: %v", err)
	}
	if !strings.Contains(string(xml), `val="dk1"`) || !strings.Contains(string(xml), "lumMod") {
		t.Errorf("band XML does not carry its scheme fill: %s", xml)
	}

	// An unpainted band draws nothing; a band that is not a fill is refused.
	none, err := Resolve(build(json.RawMessage(`"none"`)), newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(none.Bands) != 0 {
		t.Errorf(`band "none" resolved %d rectangles`, len(none.Bands))
	}
	if errs := ValidateTrackWeights(build(json.RawMessage(`[1,2]`))); len(errs) == 0 {
		t.Error("a band that is not a fill was accepted")
	}
	if errs := ValidateTrackWeights(build(fill)); len(errs) != 0 {
		t.Errorf("a fill band was refused: %v", errs)
	}
}
