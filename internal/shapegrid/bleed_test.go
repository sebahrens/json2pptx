package shapegrid

import (
	"encoding/json"
	"math"
	"testing"
)

// A cell's bleed_left extends its shape over the column gap into the cell
// before it — an interlocking chevron — without moving any cell rectangle
// (go-slide-creator-gm4q9).
func TestResolveBleedLeftExtendsTheShapeOnly(t *testing.T) {
	build := func(bleed float64) *Grid {
		return &Grid{
			Bounds:  DefaultBounds(0, 0),
			Columns: []float64{50, 50},
			ColGap:  4,
			Rows: []Row{{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "homePlate"}},
				{Shape: &ShapeSpec{Geometry: "chevron"}, BleedLeft: bleed},
			}}},
		}
	}
	plain, err := Resolve(build(0), newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	bled, err := Resolve(build(12), newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	want := PtToEMU(12)
	p, b := plain.Cells[1], bled.Cells[1]
	if b.CellBounds != p.CellBounds || bled.Cells[0].Bounds != plain.Cells[0].Bounds {
		t.Errorf("bleed_left moved a cell: %+v vs %+v", b.CellBounds, p.CellBounds)
	}
	if b.Bounds.X != p.Bounds.X-want || b.Bounds.CX != p.Bounds.CX+want || b.Bounds.Y != p.Bounds.Y || b.Bounds.CY != p.Bounds.CY {
		t.Errorf("shape bounds = %+v, want %+v extended %d EMU to the left", b.Bounds, p.Bounds, want)
	}
	// The tail now reaches past the gap into the first cell.
	if first := bled.Cells[0].Bounds; b.Bounds.X >= first.X+first.CX {
		t.Errorf("the bled shape starts at %d, not inside the previous cell ending at %d", b.Bounds.X, first.X+first.CX)
	}
}

func TestValidateTrackWeightsRejectsBadBleed(t *testing.T) {
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		grid := &Grid{Columns: []float64{100}, Rows: []Row{{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}, BleedLeft: bad}}}}}
		if errs := ValidateTrackWeights(grid); len(errs) == 0 {
			t.Errorf("bleed_left %v was accepted", bad)
		}
	}
}

// A cell that spans rows is not bounded by the max_height of the row it
// starts in, so it must not be reported as overflowing it
// (go-slide-creator-j8t7o: the maturity staircase's headers span bands).
func TestRowOverflowIgnoresSpanningCells(t *testing.T) {
	text := json.RawMessage(`{"paragraphs":[{"content":"A header that needs one full line","size":12}]}`)
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{
			{MinHeight: 6, MaxHeight: 6, Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect", Text: text}},
				{},
			}},
			{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
		},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RowOverflows) != 0 {
		t.Errorf("a row-spanning cell was reported against its first row's max_height: %+v", result.RowOverflows)
	}
}

// bleed_top and inset_top / inset_bottom move a shape's edges inside (or
// above) its cell without moving the cell; a row rule is a full-width
// hairline in the row gap that takes no row.
func TestResolveVerticalOffsetsAndRowRules(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		RowGap:  10,
		Rows: []Row{
			{MinHeight: 100, MaxHeight: 100, Rule: "both", Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}, InsetTop: 30, InsetBottom: 20},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Rule: "below", Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}, BleedTop: 25},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	inset, plain := result.Cells[0], result.Cells[1]
	if inset.CellBounds.Y != plain.CellBounds.Y || inset.CellBounds.CY != plain.CellBounds.CY {
		t.Errorf("insets moved the cell: %+v vs %+v", inset.CellBounds, plain.CellBounds)
	}
	if inset.Bounds.Y != plain.Bounds.Y+PtToEMU(30) || inset.Bounds.CY != plain.Bounds.CY-PtToEMU(50) {
		t.Errorf("inset shape = %+v, want %+v pulled in 30pt at the top and 20pt at the bottom", inset.Bounds, plain.Bounds)
	}
	bled, below := result.Cells[2], result.Cells[3]
	if bled.Bounds.Y != below.Bounds.Y-PtToEMU(25) || bled.Bounds.CY != below.Bounds.CY+PtToEMU(25) {
		t.Errorf("bled shape = %+v, want %+v extended 25pt upward", bled.Bounds, below.Bounds)
	}
	if bled.RowIdx != 1 || inset.RowIdx != 0 {
		t.Errorf("rules or offsets changed the row indices: %d, %d", inset.RowIdx, bled.RowIdx)
	}

	if len(result.AccentBars) != 3 {
		t.Fatalf("%d rules, want 3 (above and below row 0, below row 1)", len(result.AccentBars))
	}
	thick := PtToEMU(RowRulePt)
	for i, bar := range result.AccentBars {
		if bar.Bounds.X != grid.Bounds.X || bar.Bounds.CX != grid.Bounds.CX || bar.Bounds.CY != thick {
			t.Errorf("rule %d = %+v, want a %.2fpt hairline across the grid", i, bar.Bounds, RowRulePt)
		}
		if _, err := GenerateAccentBarXML(&result.AccentBars[i]); err != nil {
			t.Errorf("rule %d does not render: %v", i, err)
		}
	}
	// The rule between the rows sits in the middle of their gap.
	mid := result.AccentBars[1].Bounds.Y + thick/2
	if want := plain.CellBounds.Y + plain.CellBounds.CY + PtToEMU(5); mid != want {
		t.Errorf("the rule between the rows is centred at %d, want %d", mid, want)
	}

	bad := &Grid{Columns: []float64{100}, Rows: []Row{{Rule: "through", Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}}}}
	if errs := ValidateTrackWeights(bad); len(errs) == 0 {
		t.Error(`rule "through" was accepted`)
	}
}
