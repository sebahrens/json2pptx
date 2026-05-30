package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestResolve_ExcessiveColGap verifies that a grid whose total column gap meets
// or exceeds the grid width is rejected rather than producing negative extents.
func TestResolve_ExcessiveColGap(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 200000, CY: 5000000},
		Columns: []float64{25, 25, 25, 25}, // 4 columns → 3 gaps
		ColGap:  100,                       // 100pt = 1,270,000 EMU/gap; 3 gaps ≫ 200000 EMU bounds
		Rows: []Row{{Cells: []Cell{
			{Shape: &ShapeSpec{Geometry: "rect"}},
			{Shape: &ShapeSpec{Geometry: "rect"}},
			{Shape: &ShapeSpec{Geometry: "rect"}},
			{Shape: &ShapeSpec{Geometry: "rect"}},
		}}},
	}

	if _, err := Resolve(grid, newAlloc(1)); err == nil {
		t.Fatal("expected error for excessive col_gap, got nil")
	}
}

// TestResolve_ExcessiveRowGap verifies that a grid whose total row gap meets or
// exceeds the grid height is rejected rather than producing negative extents.
func TestResolve_ExcessiveRowGap(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 5000000, CY: 200000},
		Columns: []float64{100},
		RowGap:  100, // 100pt = 1,270,000 EMU/gap; 2 gaps ≫ 200000 EMU bounds
		Rows: []Row{
			{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
			{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
			{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
		},
	}

	if _, err := Resolve(grid, newAlloc(1)); err == nil {
		t.Fatal("expected error for excessive row_gap, got nil")
	}
}

// TestResolve_GapWithinBounds verifies that a grid with reasonable gaps relative
// to its bounds still resolves successfully (no false positives from the guard).
func TestResolve_GapWithinBounds(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000},
		Columns: []float64{50, 50},
		ColGap:  8,
		RowGap:  8,
		Rows: []Row{{Cells: []Cell{
			{Shape: &ShapeSpec{Geometry: "rect"}},
			{Shape: &ShapeSpec{Geometry: "ellipse"}},
		}}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatalf("unexpected error for valid gaps: %v", err)
	}
	if result == nil || len(result.Cells) != 2 {
		t.Fatalf("expected 2 resolved cells, got %v", result)
	}
}

// TestResolve_ZeroBoundsZeroGapSingleRow verifies that the gap guard does not
// false-positive on the height-deferred case: a single-row grid whose bounds
// height is 0 has a total row gap of 0, which equals (does not exceed) the
// bounds and must still resolve (it produces zero-height cells, as before).
func TestResolve_ZeroBoundsZeroGapSingleRow(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 0}, // height deferred to 0
		Columns: []float64{50, 50},
		Rows: []Row{{Cells: []Cell{
			{Shape: &ShapeSpec{Geometry: "rect"}},
			{Shape: &ShapeSpec{Geometry: "rect"}},
		}}},
	}

	if _, err := Resolve(grid, newAlloc(1)); err != nil {
		t.Fatalf("unexpected error for zero-height single-row grid: %v", err)
	}
}

// TestDistributeEMU_NonPositiveTotal verifies the defensive clamp: a zero or
// negative total yields zero-width entries (never negative DrawingML extents),
// even if upstream gap/bounds validation is bypassed.
func TestDistributeEMU_NonPositiveTotal(t *testing.T) {
	for _, total := range []int64{0, -100, -5000000} {
		got := distributeEMU([]float64{50, 50}, total)
		if len(got) != 2 {
			t.Fatalf("total=%d: expected 2 entries, got %d", total, len(got))
		}
		for i, v := range got {
			if v != 0 {
				t.Errorf("total=%d: entry %d = %d, want 0", total, i, v)
			}
		}
	}
}
