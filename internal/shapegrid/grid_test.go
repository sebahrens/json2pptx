package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func newAlloc(startID uint32) *pptx.ShapeIDAllocator {
	alloc := &pptx.ShapeIDAllocator{}
	alloc.SetMinID(startID)
	return alloc
}

func TestResolve_SingleRow(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	if cells[0].Kind != CellKindShape || cells[1].Kind != CellKindShape {
		t.Error("expected all cells to be CellKindShape")
	}
	if cells[0].ID != 100 || cells[1].ID != 101 {
		t.Errorf("expected IDs 100,101 got %d,%d", cells[0].ID, cells[1].ID)
	}
}

func TestResolve_NilGrid(t *testing.T) {
	result, err := Resolve(nil, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil grid, got %v", result)
	}
}

func TestResolve_EmptyCellSkipped(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{}, // no shape
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell (empty skipped), got %d", len(cells))
	}
}

func TestResolve_ColSpan(t *testing.T) {
	grid := &Grid{
		Bounds:  BoundsFromPercentages(0, 0, 100, 100, 0, 0),
		Columns: []float64{25, 25, 25, 25},
		Rows: []Row{
			{Height: 50, Cells: []Cell{
				{ColSpan: 4, Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Height: 50, Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 5 {
		t.Fatalf("expected 5 cells, got %d", len(cells))
	}
	// First cell should span full width (wider than individual cols)
	if cells[0].Bounds.CX <= cells[1].Bounds.CX {
		t.Error("col_span=4 cell should be wider than single column cell")
	}
}

func TestResolve_RowSpan(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				// col 0 occupied by row_span
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(cells))
	}
	// First cell (row_span=2) should be taller than others
	if cells[0].Bounds.CY <= cells[1].Bounds.CY {
		t.Error("row_span=2 cell should be taller than single row cell")
	}
}

func TestResolveColumns_EqualSplit(t *testing.T) {
	cols, err := ResolveColumns(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
}

func TestResolveColumns_Array(t *testing.T) {
	cols, err := ResolveColumns([]float64{30, 40, 30}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cols[0] != 30 || cols[1] != 40 || cols[2] != 30 {
		t.Errorf("unexpected columns: %v", cols)
	}
}

func TestResolveColumns_InferFromRows(t *testing.T) {
	cols, err := ResolveColumns(nil, []int{4, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 4 {
		t.Fatalf("expected 4 (max), got %d", len(cols))
	}
}

func TestResolveColumns_InvalidCount(t *testing.T) {
	_, err := ResolveColumns(0, nil)
	if err == nil {
		t.Error("expected error for count=0")
	}
}

func TestResolveRowHeights_EqualSplit(t *testing.T) {
	heights := resolveRowHeights([]Row{{}, {}, {}, {}}, 5000000)
	for i, h := range heights {
		if h != 25 {
			t.Errorf("row %d: expected 25, got %f", i, h)
		}
	}
}

func TestResolveRowHeights_Mixed(t *testing.T) {
	heights := resolveRowHeights([]Row{{Height: 20}, {}, {}}, 5000000)
	if heights[0] != 20 {
		t.Errorf("expected 20, got %f", heights[0])
	}
	if heights[1] != 40 || heights[2] != 40 {
		t.Errorf("expected 40 each, got %f and %f", heights[1], heights[2])
	}
}

func TestResolveRowHeights_AutoHeight(t *testing.T) {
	// A header row at 20% + an auto-height content row with short text.
	// The auto row should fill the remaining 80% since it's the only flexible row.
	rows := []Row{
		{Height: 20},
		{
			AutoHeight: true,
			Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`{"content":"Line 1\nLine 2\nLine 3","size":11,"inset_top":12}`),
			}}},
		},
	}
	gridH := int64(5000000) // ~5M EMU
	heights := resolveRowHeights(rows, gridH)

	if heights[0] != 20 {
		t.Errorf("header row: expected 20, got %f", heights[0])
	}
	// Auto row gets the full remaining 80% (equal share of remaining space)
	if heights[1] != 80 {
		t.Errorf("auto row: expected 80, got %f", heights[1])
	}
}

func TestResolveRowHeights_AutoHeightWithUnspecified(t *testing.T) {
	// Header at 20%, auto-height row, and an unspecified row.
	// Both flexible rows should share the remaining 80% equally.
	rows := []Row{
		{Height: 20},
		{
			AutoHeight: true,
			Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`{"content":"Short","size":11}`),
			}}},
		},
		{}, // unspecified
	}
	gridH := int64(5000000)
	heights := resolveRowHeights(rows, gridH)

	if heights[0] != 20 {
		t.Errorf("header row: expected 20, got %f", heights[0])
	}
	// Auto row gets at least equal share = 40%
	if heights[1] != 40 {
		t.Errorf("auto row: expected 40, got %f", heights[1])
	}
	// Unspecified row gets the remaining 40%
	if heights[2] != 40 {
		t.Errorf("unspecified row: expected 40, got %f", heights[2])
	}
}

func TestEstimateCellTextHeightEMU_StringShorthand(t *testing.T) {
	cell := Cell{Shape: &ShapeSpec{
		Text: json.RawMessage(`"Hello\nWorld"`),
	}}
	h := estimateCellTextHeightEMU(cell)
	if h <= 0 {
		t.Error("expected positive height for text cell")
	}
}

func TestEstimateCellTextHeightEMU_ObjectForm(t *testing.T) {
	cell := Cell{Shape: &ShapeSpec{
		Text: json.RawMessage(`{"content":"A\nB\nC","size":14,"inset_top":10,"inset_bottom":5}`),
	}}
	h := estimateCellTextHeightEMU(cell)
	if h <= 0 {
		t.Error("expected positive height for text cell")
	}
}

func TestEstimateCellTextHeightEMU_EmptyShape(t *testing.T) {
	cell := Cell{Shape: nil}
	h := estimateCellTextHeightEMU(cell)
	if h != 0 {
		t.Errorf("expected 0 for nil shape, got %d", h)
	}
}

func TestPctToEMU_Basic(t *testing.T) {
	got := PctToEMU(50, 12192000)
	if got != 6096000 {
		t.Errorf("expected 6096000, got %d", got)
	}
}

func TestResolve_TableCell(t *testing.T) {
	tableSpec := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{TableSpec: tableSpec},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	if cells[0].Kind != CellKindShape {
		t.Errorf("cell 0: expected CellKindShape, got %s", cells[0].Kind)
	}
	if cells[1].Kind != CellKindTable {
		t.Errorf("cell 1: expected CellKindTable, got %s", cells[1].Kind)
	}
	if cells[1].TableSpec == nil {
		t.Error("cell 1: expected TableSpec to be set")
	}
	if cells[1].ShapeSpec != nil {
		t.Error("cell 1: expected ShapeSpec to be nil for table cell")
	}
	// Verify bounds are computed (non-zero width and height)
	if cells[1].Bounds.CX == 0 || cells[1].Bounds.CY == 0 {
		t.Error("cell 1: expected non-zero bounds for table cell")
	}
}

func TestResolveColumns_SingleColumn(t *testing.T) {
	cols, err := ResolveColumns(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0] != 100 {
		t.Errorf("expected [100], got %v", cols)
	}
}

func TestResolveColumns_LargeCount(t *testing.T) {
	cols, err := ResolveColumns(50, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 50 {
		t.Fatalf("expected 50 columns, got %d", len(cols))
	}
	for _, c := range cols {
		if c != 2 {
			t.Errorf("expected 2%% each, got %f", c)
		}
	}
}

func TestResolveColumns_NegativeCount(t *testing.T) {
	_, err := ResolveColumns(-1, nil)
	if err == nil {
		t.Error("expected error for negative count")
	}
}

func TestResolveColumns_EmptyArray(t *testing.T) {
	_, err := ResolveColumns([]float64{}, nil)
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestResolveColumns_InferFromEmptyRows(t *testing.T) {
	_, err := ResolveColumns(nil, []int{})
	if err == nil {
		t.Error("expected error when inferring from empty rows")
	}
}

func TestResolveColumns_InferFromZeroCells(t *testing.T) {
	_, err := ResolveColumns(nil, []int{0, 0})
	if err == nil {
		t.Error("expected error when all rows have 0 cells")
	}
}

func TestResolveRowHeights_SingleRow(t *testing.T) {
	heights := resolveRowHeights([]Row{{}}, 5000000)
	if heights[0] != 100 {
		t.Errorf("expected 100, got %f", heights[0])
	}
}

func TestResolveRowHeights_AllSpecified(t *testing.T) {
	heights := resolveRowHeights([]Row{{Height: 30}, {Height: 40}, {Height: 30}}, 5000000)
	if heights[0] != 30 || heights[1] != 40 || heights[2] != 30 {
		t.Errorf("expected [30,40,30], got %v", heights)
	}
}

func TestResolveRowHeights_ExceedingTotal(t *testing.T) {
	heights := resolveRowHeights([]Row{{Height: 60}, {Height: 60}}, 5000000)
	// Both specified, no unspecified to adjust
	if heights[0] != 60 || heights[1] != 60 {
		t.Errorf("expected [60,60], got %v", heights)
	}
}

func TestResolveRowHeights_ExceedingWithUnspecified(t *testing.T) {
	heights := resolveRowHeights([]Row{{Height: 80}, {Height: 40}, {}}, 5000000)
	// Remaining = 100 - 120 = 0 (clamped), so unspecified gets 0
	if heights[2] != 0 {
		t.Errorf("expected 0 for unspecified row when total exceeds 100, got %f", heights[2])
	}
}

func TestResolveRowHeights_TenEqualRows(t *testing.T) {
	rows := make([]Row, 10)
	heights := resolveRowHeights(rows, 5000000)
	for i, h := range heights {
		if h != 10 {
			t.Errorf("row %d: expected 10, got %f", i, h)
		}
	}
}

func TestResolve_AllZeroHeights_ShrinksBounds(t *testing.T) {
	// When no row specifies explicit height, the grid should auto-calculate
	// content-based heights and shrink bounds to fit instead of stretching
	// to fill the full content zone.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000}, // large grid
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`{"content":"Line 1\nLine 2","size":11}`),
			}}, {Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short`)}}}},
			{Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`{"content":"A\nB\nC","size":11}`),
			}}, {Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Hi`)}}}},
			{Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`"One line"`),
			}}, {Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Two`)}}}},
		},
	}

	origCY := grid.Bounds.CY
	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Grid should have shrunk because content is short relative to the full 5M EMU.
	if grid.Bounds.CY >= origCY {
		t.Errorf("expected grid CY to shrink below %d, got %d", origCY, grid.Bounds.CY)
	}
}

func TestResolve_ExplicitHeights_NoBoundsShrink(t *testing.T) {
	// When rows have explicit heights, bounds should NOT be modified.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000},
		Columns: []float64{100},
		Rows: []Row{
			{Height: 50, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short`)}}}},
			{Height: 50, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short`)}}}},
		},
	}

	alloc := newAlloc(100)
	_, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}

	if grid.Bounds.CY != 5000000 {
		t.Errorf("expected grid CY to remain 5000000, got %d", grid.Bounds.CY)
	}
}

func TestDistributeEMU_ExactSum(t *testing.T) {
	// 3 equal rows in a space that isn't evenly divisible by 3
	pcts := []float64{33.333333, 33.333333, 33.333334}
	totalEMU := int64(5000001) // not divisible by 3
	result := distributeEMU(pcts, totalEMU)

	var sum int64
	for _, v := range result {
		sum += v
	}
	if sum != totalEMU {
		t.Errorf("distributeEMU sum = %d, want %d", sum, totalEMU)
	}
}

func TestDistributeEMU_AllZero(t *testing.T) {
	pcts := []float64{0, 0, 0}
	result := distributeEMU(pcts, 9000000)
	var sum int64
	for _, v := range result {
		sum += v
	}
	if sum != 9000000 {
		t.Errorf("distributeEMU all-zero sum = %d, want 9000000", sum)
	}
}

func TestResolve_SingleCell(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	if cells[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", cells[0].ID)
	}
}

func TestResolve_ExplicitGap(t *testing.T) {
	grid := &Grid{
		Bounds:  BoundsFromPercentages(0, 0, 100, 100, 0, 0),
		Columns: []float64{50, 50},
		ColGap:  5,
		RowGap:  5,
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	// Second cell X should be offset by first cell width + gap
	if cells[1].Bounds.X <= cells[0].Bounds.X+cells[0].Bounds.CX {
		t.Error("expected gap between cells")
	}
}

func TestResolve_ZeroGap(t *testing.T) {
	// ColGap/RowGap = 0 means use default (8pt), need to explicitly test that
	// The system defaults to 8pt when 0 is set
	grid := &Grid{
		Bounds:  BoundsFromPercentages(0, 0, 100, 100, 0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	gapResult, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	gapCells := gapResult.Cells
	// Default gap is 2%, so cells should not be adjacent
	if gapCells[1].Bounds.X == gapCells[0].Bounds.X+gapCells[0].Bounds.CX {
		t.Error("expected default gap between cells when gap=0")
	}
}

func TestResolve_AsymmetricColumnsAbsoluteGap(t *testing.T) {
	// With absolute point gaps, a 10/90 column split with gap:4 produces
	// a consistent 4pt gap regardless of column proportions.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 11_430_000, CY: 5_000_000},
		Columns: []float64{10, 90},
		ColGap:  4,
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}

	gap := cells[1].Bounds.X - (cells[0].Bounds.X + cells[0].Bounds.CX)
	expectedGap := int64(4 * 12700) // 4pt = 50,800 EMU

	if gap != expectedGap {
		t.Errorf("asymmetric gap should be exactly 4pt: got %d EMU, want %d", gap, expectedGap)
	}
}

func TestResolve_EqualColumnsAbsoluteGap(t *testing.T) {
	// Gap values are absolute points, independent of grid width.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 11_430_000, CY: 5_000_000},
		Columns: []float64{50, 50},
		ColGap:  4,
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells

	gap := cells[1].Bounds.X - (cells[0].Bounds.X + cells[0].Bounds.CX)
	expectedGap := int64(4 * 12700) // 4pt = 50,800 EMU

	if gap != expectedGap {
		t.Errorf("gap should be exactly 4pt (50800 EMU): got %d, want %d", gap, expectedGap)
	}
}

func TestResolve_ColSpanAndRowSpanCombined(t *testing.T) {
	grid := &Grid{
		Bounds:  BoundsFromPercentages(0, 0, 100, 100, 0, 0),
		Columns: []float64{25, 25, 25, 25},
		Rows: []Row{
			{Height: 50, Cells: []Cell{
				{ColSpan: 2, RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
			}},
			{Height: 50, Cells: []Cell{
				// cols 0-1 occupied by span
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
			}},
		},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 5 {
		t.Fatalf("expected 5 cells, got %d", len(cells))
	}
	// First cell should be wider AND taller than others
	if cells[0].Bounds.CX <= cells[1].Bounds.CX {
		t.Error("col_span=2 cell should be wider")
	}
	if cells[0].Bounds.CY <= cells[1].Bounds.CY {
		t.Error("row_span=2 cell should be taller")
	}
}

func TestResolve_MoreCellsThanColumns(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}}, // extra cell - should be ignored
			},
		}},
	}
	extraResult, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Only 2 columns, so third cell should be dropped
	if len(extraResult.Cells) != 2 {
		t.Fatalf("expected 2 cells (extra ignored), got %d", len(extraResult.Cells))
	}
}

func TestResolve_FewerCellsThanColumns(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{25, 25, 25, 25},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
}

func TestResolve_AllEmptyCells(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{{}, {}},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 0 {
		t.Fatalf("expected 0 cells (all empty), got %d", len(cells))
	}
}

func TestResolve_10x10StressGrid(t *testing.T) {
	cols := make([]float64, 10)
	for i := range cols {
		cols[i] = 10
	}
	rows := make([]Row, 10)
	for r := range rows {
		cells := make([]Cell, 10)
		for c := range cells {
			cells[c] = Cell{Shape: &ShapeSpec{Geometry: "rect"}}
		}
		rows[r] = Row{Cells: cells}
	}
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: cols,
		Rows:    rows,
	}
	stressResult, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	resolved := stressResult.Cells
	if len(resolved) != 100 {
		t.Fatalf("expected 100 cells, got %d", len(resolved))
	}
	// Verify IDs are sequential
	for i, c := range resolved {
		if c.ID != uint32(i+1) {
			t.Errorf("cell %d: expected ID %d, got %d", i, i+1, c.ID)
		}
	}
	// Verify no bounds overlap
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			a, b := resolved[i].Bounds, resolved[j].Bounds
			if a.X < b.X+b.CX && a.X+a.CX > b.X && a.Y < b.Y+b.CY && a.Y+a.CY > b.Y {
				t.Errorf("overlap between cell %d and cell %d", i, j)
			}
		}
	}
}

func TestResolve_MixedShapeAndTable(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{33.33, 33.33, 33.34},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{TableSpec: &types.TableSpec{Headers: []string{"A"}, Rows: [][]types.TableCell{{{Content: "1", ColSpan: 1, RowSpan: 1}}}, Style: types.DefaultTableStyle}},
				{Shape: &ShapeSpec{Geometry: "ellipse"}},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(cells))
	}
	if cells[0].Kind != CellKindShape || cells[1].Kind != CellKindTable || cells[2].Kind != CellKindShape {
		t.Error("expected shape, table, shape kinds")
	}
}

func TestApplyFitMode_Stretch(t *testing.T) {
	bounds := pptx.RectEmu{X: 100, Y: 200, CX: 400, CY: 300}
	result := ApplyFitMode(FitStretch, bounds)
	if result != bounds {
		t.Errorf("stretch should return original bounds, got %+v", result)
	}
}

func TestApplyFitMode_Contain_WideCell(t *testing.T) {
	// Cell is wider than tall: 400x200, contain should produce 200x200 centered
	bounds := pptx.RectEmu{X: 100, Y: 100, CX: 400, CY: 200}
	result := ApplyFitMode(FitContain, bounds)
	if result.CX != 200 || result.CY != 200 {
		t.Errorf("contain: expected 200x200, got %dx%d", result.CX, result.CY)
	}
	if result.X != 200 { // 100 + (400-200)/2
		t.Errorf("contain: expected X=200, got %d", result.X)
	}
	if result.Y != 100 { // no vertical offset
		t.Errorf("contain: expected Y=100, got %d", result.Y)
	}
}

func TestApplyFitMode_Contain_TallCell(t *testing.T) {
	// Cell is taller than wide: 200x400, contain should produce 200x200 centered
	bounds := pptx.RectEmu{X: 100, Y: 100, CX: 200, CY: 400}
	result := ApplyFitMode(FitContain, bounds)
	if result.CX != 200 || result.CY != 200 {
		t.Errorf("contain: expected 200x200, got %dx%d", result.CX, result.CY)
	}
	if result.X != 100 { // no horizontal offset
		t.Errorf("contain: expected X=100, got %d", result.X)
	}
	if result.Y != 200 { // 100 + (400-200)/2
		t.Errorf("contain: expected Y=200, got %d", result.Y)
	}
}

func TestApplyFitMode_Contain_SquareCell(t *testing.T) {
	bounds := pptx.RectEmu{X: 100, Y: 100, CX: 300, CY: 300}
	result := ApplyFitMode(FitContain, bounds)
	if result != bounds {
		t.Errorf("contain on square cell should return original bounds, got %+v", result)
	}
}

func TestApplyFitMode_FitWidth(t *testing.T) {
	// Cell 400x200, fit-width: width stays 400, height becomes 400, centered vertically
	bounds := pptx.RectEmu{X: 100, Y: 100, CX: 400, CY: 200}
	result := ApplyFitMode(FitWidth, bounds)
	if result.CX != 400 || result.CY != 400 {
		t.Errorf("fit-width: expected 400x400, got %dx%d", result.CX, result.CY)
	}
	if result.X != 100 {
		t.Errorf("fit-width: expected X=100, got %d", result.X)
	}
	if result.Y != 0 { // 100 + (200-400)/2 = 0
		t.Errorf("fit-width: expected Y=0, got %d", result.Y)
	}
}

func TestApplyFitMode_FitHeight(t *testing.T) {
	// Cell 200x400, fit-height: height stays 400, width becomes 400, centered horizontally
	bounds := pptx.RectEmu{X: 100, Y: 100, CX: 200, CY: 400}
	result := ApplyFitMode(FitHeight, bounds)
	if result.CX != 400 || result.CY != 400 {
		t.Errorf("fit-height: expected 400x400, got %dx%d", result.CX, result.CY)
	}
	if result.X != 0 { // 100 + (200-400)/2 = 0
		t.Errorf("fit-height: expected X=0, got %d", result.X)
	}
	if result.Y != 100 {
		t.Errorf("fit-height: expected Y=100, got %d", result.Y)
	}
}

func TestResolve_FitContain(t *testing.T) {
	// One row, one wide cell with fit=contain
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 2000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{Fit: FitContain, Shape: &ShapeSpec{Geometry: "ellipse"}},
			},
		}},
		ColGap: 0,
		RowGap: 0,
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	c := cells[0]
	// Cell bounds: 4000x2000, contain -> 2000x2000 centered at X=1000
	if c.CellBounds.CX != 4000 || c.CellBounds.CY != 2000 {
		t.Errorf("cell bounds should be 4000x2000, got %dx%d", c.CellBounds.CX, c.CellBounds.CY)
	}
	if c.Bounds.CX != 2000 || c.Bounds.CY != 2000 {
		t.Errorf("shape bounds should be 2000x2000, got %dx%d", c.Bounds.CX, c.Bounds.CY)
	}
	if c.Bounds.X != 1000 {
		t.Errorf("shape X should be 1000, got %d", c.Bounds.X)
	}
}

func TestResolve_IconDefaultContain(t *testing.T) {
	// Icons should default to contain mode even without explicit fit setting.
	// Wide cell (4000x2000) with icon should produce 2000x2000 centered.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 2000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{Icon: &IconSpec{Name: "chart-pie"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	if c.CellBounds.CX != 4000 || c.CellBounds.CY != 2000 {
		t.Errorf("cell bounds should be 4000x2000, got %dx%d", c.CellBounds.CX, c.CellBounds.CY)
	}
	// Contain on 4000x2000 → 2000x2000, centered at X=1000
	if c.Bounds.CX != 2000 || c.Bounds.CY != 2000 {
		t.Errorf("icon bounds should be 2000x2000 (contain), got %dx%d", c.Bounds.CX, c.Bounds.CY)
	}
	if c.Bounds.X != 1000 {
		t.Errorf("icon X should be 1000 (centered), got %d", c.Bounds.X)
	}
}

func TestResolve_ShapeWithIconOverlay(t *testing.T) {
	// A cell with both shape and icon should NOT be forced to square.
	// The shape should stretch to fill the full cell, and the icon should
	// be contained (square) within the shape bounds.
	// Wide cell (4000x1000): shape = 4000x1000, icon = 1000x1000 centered.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`)},
					Icon:  &IconSpec{Name: "shield", Position: "center"},
				},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	// Shape bounds should be full cell (stretch)
	if c.Bounds.CX != 4000 || c.Bounds.CY != 1000 {
		t.Errorf("shape bounds should be 4000x1000 (stretch), got %dx%d", c.Bounds.CX, c.Bounds.CY)
	}
	// Icon bounds should be scaled to 60% of min(cx,cy)=1000 → 600x600
	// centered within shape: X = 0 + (4000-600)/2 = 1700, Y = 0 + (1000-600)/2 = 200
	if c.IconBounds.CX != 600 || c.IconBounds.CY != 600 {
		t.Errorf("icon bounds should be 600x600 (60%% of min dim), got %dx%d", c.IconBounds.CX, c.IconBounds.CY)
	}
	if c.IconBounds.X != 1700 {
		t.Errorf("icon X should be 1700 (centered), got %d", c.IconBounds.X)
	}
	if c.IconBounds.Y != 200 {
		t.Errorf("icon Y should be 200 (centered), got %d", c.IconBounds.Y)
	}
}

func TestResolve_ShapeWithIconOverlay_CustomScale(t *testing.T) {
	// Custom scale of 0.8 on a 4000x1000 cell:
	// minDim=1000, size=800, pad=(1000-800)/2=100
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`)},
					Icon:  &IconSpec{Name: "shield", Scale: 0.8, Position: "center"},
				},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	if c.IconBounds.CX != 800 || c.IconBounds.CY != 800 {
		t.Errorf("icon bounds should be 800x800 (80%% of min dim), got %dx%d", c.IconBounds.CX, c.IconBounds.CY)
	}
	// centered: X = 0 + (4000-800)/2 = 1600, Y = 0 + (1000-800)/2 = 100
	if c.IconBounds.X != 1600 {
		t.Errorf("icon X should be 1600 (centered), got %d", c.IconBounds.X)
	}
	if c.IconBounds.Y != 100 {
		t.Errorf("icon Y should be 100 (centered), got %d", c.IconBounds.Y)
	}
}

func TestResolve_ShapeWithIconOverlay_LeftPosition(t *testing.T) {
	// Wide cell (4000x1000): icon on the left, text shifted right.
	// scale=0.6, minDim=1000, size=600, iconH=min(h*0.6,size)=600
	// Icon X = gap(38100), Y = (1000-600)/2=200
	// TextInsets.L = 600 + 2*38100 = 676200... wait, let me recalc
	// iconH = 600, gap = 38100
	// TextInsets[0] = iconH + 2*gap = 600 + 76200 = 76800
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`)},
				Icon:  &IconSpec{Name: "shield", Position: "left"},
			}},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	// Icon should be left-aligned with a gap
	if c.IconBounds.X != 38100 {
		t.Errorf("icon X should be 38100 (gap), got %d", c.IconBounds.X)
	}
	if c.IconBounds.CX != 600 || c.IconBounds.CY != 600 {
		t.Errorf("icon bounds should be 600x600, got %dx%d", c.IconBounds.CX, c.IconBounds.CY)
	}
	// TextInsets should push text right past the icon
	if c.TextInsets[0] == 0 {
		t.Error("TextInsets[0] (left) should be > 0 for left-positioned icon")
	}
	expectedInsetL := int64(600 + 2*38100)
	if c.TextInsets[0] != expectedInsetL {
		t.Errorf("TextInsets[0] should be %d, got %d", expectedInsetL, c.TextInsets[0])
	}
}

func TestResolve_ShapeWithIconOverlay_TopPosition(t *testing.T) {
	// Square cell (1000x1000): icon on top, text shifted down.
	// scale=0.6, minDim=1000, size=600
	// Icon X = (1000-600)/2=200, Y = gap(38100)
	// TextInsets[1] = 600 + 2*38100 = 76800... wait let me recalc
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 1000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`)},
				Icon:  &IconSpec{Name: "shield", Position: "top"},
			}},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	// Icon should be centered horizontally at top
	if c.IconBounds.X != 200 {
		t.Errorf("icon X should be 200 (centered), got %d", c.IconBounds.X)
	}
	if c.IconBounds.Y != 38100 {
		t.Errorf("icon Y should be 38100 (gap), got %d", c.IconBounds.Y)
	}
	// TextInsets should push text below the icon
	if c.TextInsets[1] == 0 {
		t.Error("TextInsets[1] (top) should be > 0 for top-positioned icon")
	}
	expectedInsetT := int64(600 + 2*38100)
	if c.TextInsets[1] != expectedInsetT {
		t.Errorf("TextInsets[1] should be %d, got %d", expectedInsetT, c.TextInsets[1])
	}
}

func TestResolve_ShapeWithIconOverlay_AutoDetect(t *testing.T) {
	// Wide cell with text auto-detects to "left", square cell with text auto-detects to "top".
	// Wide: 4000x1000, CX > 1.2*CY → "left"
	wideGrid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`[{"text":"hello"}]`)},
				Icon:  &IconSpec{Name: "shield"}, // no explicit position
			}},
		}},
	}
	result, err := Resolve(wideGrid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Wide cell with text → left position → TextInsets[0] (left) should be set
	if result.Cells[0].TextInsets[0] == 0 {
		t.Error("wide cell should auto-detect 'left' position with left text inset")
	}

	// Square: 1000x1000, CX <= 1.2*CY → "top"
	squareGrid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 1000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`[{"text":"hello"}]`)},
				Icon:  &IconSpec{Name: "shield"}, // no explicit position
			}},
		}},
	}
	result2, err := Resolve(squareGrid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Square cell with text → top position → TextInsets[1] (top) should be set
	if result2.Cells[0].TextInsets[1] == 0 {
		t.Error("square cell should auto-detect 'top' position with top text inset")
	}

	// No-text cell auto-detects to "center" regardless of aspect ratio.
	noTextGrid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{Fill: json.RawMessage(`"accent1"`)},
				Icon:  &IconSpec{Name: "shield"}, // no explicit position, no text
			}},
		}},
	}
	result3, err := Resolve(noTextGrid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// No text → center position → no text insets
	if result3.Cells[0].TextInsets[0] != 0 || result3.Cells[0].TextInsets[1] != 0 {
		t.Error("no-text cell should auto-detect 'center' position with no text insets")
	}
}

func TestResolve_ImageDefaultContain(t *testing.T) {
	// Images should also default to contain mode.
	// Tall cell (2000x4000) with image should produce 2000x2000 centered.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 2000, CY: 4000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{Image: &ImageSpec{Path: "/tmp/photo.jpg"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	// Contain on 2000x4000 → 2000x2000, centered at Y=1000
	if c.Bounds.CX != 2000 || c.Bounds.CY != 2000 {
		t.Errorf("image bounds should be 2000x2000 (contain), got %dx%d", c.Bounds.CX, c.Bounds.CY)
	}
	if c.Bounds.Y != 1000 {
		t.Errorf("image Y should be 1000 (centered), got %d", c.Bounds.Y)
	}
}

func TestResolve_ShapeDefaultStretch(t *testing.T) {
	// Shapes should still default to stretch (no fit mode change).
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 2000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	c := result.Cells[0]
	// Stretch: bounds should equal cell bounds
	if c.Bounds.CX != 4000 || c.Bounds.CY != 2000 {
		t.Errorf("shape bounds should be 4000x2000 (stretch), got %dx%d", c.Bounds.CX, c.Bounds.CY)
	}
}

func TestResolve_ImageCell(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Image: &ImageSpec{Path: "/tmp/photo.jpg", Alt: "Test photo"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	if cells[0].Kind != CellKindShape {
		t.Errorf("cell 0: expected CellKindShape, got %s", cells[0].Kind)
	}
	if cells[1].Kind != CellKindImage {
		t.Errorf("cell 1: expected CellKindImage, got %s", cells[1].Kind)
	}
	if cells[1].ImageSpec == nil {
		t.Fatal("cell 1: ImageSpec should not be nil")
	}
	if cells[1].ImageSpec.Path != "/tmp/photo.jpg" {
		t.Errorf("cell 1: expected path /tmp/photo.jpg, got %s", cells[1].ImageSpec.Path)
	}
	if cells[1].ImageSpec.Alt != "Test photo" {
		t.Errorf("cell 1: expected alt 'Test photo', got %s", cells[1].ImageSpec.Alt)
	}
}

func TestResolve_ImageCellPriority(t *testing.T) {
	// Image takes priority over shape when both are set
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape: &ShapeSpec{Geometry: "rect"},
					Image: &ImageSpec{Path: "/tmp/photo.jpg"},
				},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	if cells[0].Kind != CellKindImage {
		t.Errorf("expected CellKindImage (image takes priority), got %s", cells[0].Kind)
	}
}

func TestResolve_ImageWithRowSpan(t *testing.T) {
	// Split-column asymmetric layout: left column has 2 stacked text shapes,
	// right column has a single full-height image spanning both rows.
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		ColGap:  2, // 2pt gap
		RowGap:  2,
		Rows: []Row{
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{RowSpan: 2, Image: &ImageSpec{
					Path: "/tmp/hero.jpg",
					Alt:  "Full-height dramatic photo",
					Overlay: &OverlaySpec{Color: "000000", Alpha: 0.3},
					Text:    &ImageText{Content: "Caption", Size: 16, Bold: true, Color: "FFFFFF"},
				}},
			}},
			{Cells: []Cell{
				// col 1 occupied by row_span image
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}

	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	cells := result.Cells
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(cells))
	}

	// Cell 0: top-left shape (single row)
	if cells[0].Kind != CellKindShape {
		t.Errorf("cell 0: expected CellKindShape, got %s", cells[0].Kind)
	}

	// Cell 1: right-column image spanning 2 rows
	if cells[1].Kind != CellKindImage {
		t.Errorf("cell 1: expected CellKindImage, got %s", cells[1].Kind)
	}
	if cells[1].ImageSpec == nil {
		t.Fatal("cell 1: ImageSpec should not be nil")
	}
	if cells[1].ImageSpec.Path != "/tmp/hero.jpg" {
		t.Errorf("cell 1: expected path /tmp/hero.jpg, got %s", cells[1].ImageSpec.Path)
	}
	if cells[1].ImageSpec.Overlay == nil {
		t.Error("cell 1: Overlay should not be nil")
	}
	if cells[1].ImageSpec.Text == nil {
		t.Error("cell 1: Text should not be nil")
	}

	// The row-spanning image should be taller than the single-row shapes
	if cells[1].Bounds.CY <= cells[0].Bounds.CY {
		t.Errorf("row_span=2 image should be taller than single row shape: %d <= %d",
			cells[1].Bounds.CY, cells[0].Bounds.CY)
	}

	// Cell 2: bottom-left shape (single row)
	if cells[2].Kind != CellKindShape {
		t.Errorf("cell 2: expected CellKindShape, got %s", cells[2].Kind)
	}

	// Both single-row shapes should have same height (within 1 EMU rounding tolerance)
	heightDiff := cells[0].Bounds.CY - cells[2].Bounds.CY
	if heightDiff < -1 || heightDiff > 1 {
		t.Errorf("single-row shapes should have same height (±1 EMU): %d != %d",
			cells[0].Bounds.CY, cells[2].Bounds.CY)
	}
}

func TestResolve_Connectors(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 9144000, CY: 4572000},
		Columns: []float64{30, 30, 30},
		ColGap:  3,
		RowGap:  3,
		Rows: []Row{{
			Connector: &ConnectorSpec{Style: "arrow", Color: "FF0000", Width: 1.5, Dash: "dot"},
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "homePlate"}},
				{Shape: &ShapeSpec{Geometry: "homePlate"}},
				{Shape: &ShapeSpec{Geometry: "homePlate"}},
			},
		}},
	}

	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(result.Cells))
	}
	if len(result.Connectors) != 2 {
		t.Fatalf("expected 2 connectors, got %d", len(result.Connectors))
	}

	// Verify connector properties
	conn0 := result.Connectors[0]
	if conn0.Spec.Style != "arrow" {
		t.Errorf("connector 0: expected style 'arrow', got %q", conn0.Spec.Style)
	}
	if conn0.Spec.Color != "FF0000" {
		t.Errorf("connector 0: expected color 'FF0000', got %q", conn0.Spec.Color)
	}
	if conn0.SourceID != result.Cells[0].ID {
		t.Errorf("connector 0: source ID mismatch: got %d, want %d", conn0.SourceID, result.Cells[0].ID)
	}
	if conn0.TargetID != result.Cells[1].ID {
		t.Errorf("connector 0: target ID mismatch: got %d, want %d", conn0.TargetID, result.Cells[1].ID)
	}

	conn1 := result.Connectors[1]
	if conn1.SourceID != result.Cells[1].ID {
		t.Errorf("connector 1: source ID mismatch: got %d, want %d", conn1.SourceID, result.Cells[1].ID)
	}
	if conn1.TargetID != result.Cells[2].ID {
		t.Errorf("connector 1: target ID mismatch: got %d, want %d", conn1.TargetID, result.Cells[2].ID)
	}

	// Connectors should have valid bounds (positive width)
	if conn0.Bounds.CX <= 0 {
		t.Errorf("connector 0: expected positive width, got %d", conn0.Bounds.CX)
	}

	// Connector IDs should be unique and after cell IDs
	if conn0.ID <= result.Cells[2].ID {
		t.Errorf("connector ID %d should be after last cell ID %d", conn0.ID, result.Cells[2].ID)
	}
}

func TestResolve_NoConnectorWithSingleCell(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Connector: &ConnectorSpec{Style: "line"},
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Connectors) != 0 {
		t.Errorf("expected 0 connectors for single cell, got %d", len(result.Connectors))
	}
}

func TestResolve_ConnectorOnlyOnSpecifiedRows(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{
			{
				Connector: &ConnectorSpec{Style: "arrow"},
				Cells: []Cell{
					{Shape: &ShapeSpec{Geometry: "rect"}},
					{Shape: &ShapeSpec{Geometry: "rect"}},
				},
			},
			{
				// No connector on this row
				Cells: []Cell{
					{Shape: &ShapeSpec{Geometry: "rect"}},
					{Shape: &ShapeSpec{Geometry: "rect"}},
				},
			},
		},
	}

	result, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Only row 0 has connector, so 1 connector total
	if len(result.Connectors) != 1 {
		t.Errorf("expected 1 connector, got %d", len(result.Connectors))
	}
}

func TestAccentBarBounds_Left(t *testing.T) {
	cell := pptx.RectEmu{X: 100000, Y: 200000, CX: 500000, CY: 300000}
	spec := &AccentBarSpec{Position: "left", Width: 4.0}
	b := accentBarBounds(cell, spec)

	widthEMU := int64(4.0 * 12700) // 50800
	gapEMU := int64(2 * 12700)     // 25400

	if b.X != cell.X-widthEMU-gapEMU {
		t.Errorf("expected X=%d, got %d", cell.X-widthEMU-gapEMU, b.X)
	}
	if b.Y != cell.Y {
		t.Errorf("expected Y=%d, got %d", cell.Y, b.Y)
	}
	if b.CX != widthEMU {
		t.Errorf("expected CX=%d, got %d", widthEMU, b.CX)
	}
	if b.CY != cell.CY {
		t.Errorf("expected CY=%d, got %d", cell.CY, b.CY)
	}
}

func TestAccentBarBounds_Bottom(t *testing.T) {
	cell := pptx.RectEmu{X: 100000, Y: 200000, CX: 500000, CY: 300000}
	spec := &AccentBarSpec{Position: "bottom", Width: 6.0}
	b := accentBarBounds(cell, spec)

	widthEMU := int64(6.0 * 12700)
	gapEMU := int64(2 * 12700)

	if b.X != cell.X {
		t.Errorf("expected X=%d, got %d", cell.X, b.X)
	}
	if b.Y != cell.Y+cell.CY+gapEMU {
		t.Errorf("expected Y=%d, got %d", cell.Y+cell.CY+gapEMU, b.Y)
	}
	if b.CX != cell.CX {
		t.Errorf("expected CX=%d, got %d", cell.CX, b.CX)
	}
	if b.CY != widthEMU {
		t.Errorf("expected CY=%d, got %d", widthEMU, b.CY)
	}
}

func TestAccentBarBounds_DefaultPositionAndWidth(t *testing.T) {
	cell := pptx.RectEmu{X: 100000, Y: 200000, CX: 500000, CY: 300000}
	spec := &AccentBarSpec{} // defaults: left, 4pt
	b := accentBarBounds(cell, spec)

	widthEMU := int64(4.0 * 12700)
	gapEMU := int64(2 * 12700)

	if b.X != cell.X-widthEMU-gapEMU {
		t.Errorf("expected left-positioned bar, got X=%d", b.X)
	}
	if b.CX != widthEMU {
		t.Errorf("expected default 4pt width (%d EMU), got %d", widthEMU, b.CX)
	}
}

func TestResolve_AccentBars(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape:     &ShapeSpec{Geometry: "rect"},
					AccentBar: &AccentBarSpec{Position: "left", Color: "accent1", Width: 4},
				},
				{
					Shape: &ShapeSpec{Geometry: "rect"},
					// No accent bar on this cell
				},
			},
		}},
	}

	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(result.Cells))
	}
	if len(result.AccentBars) != 1 {
		t.Fatalf("expected 1 accent bar, got %d", len(result.AccentBars))
	}

	bar := result.AccentBars[0]
	if bar.Spec.Color != "accent1" {
		t.Errorf("expected color accent1, got %s", bar.Spec.Color)
	}
	if bar.Spec.Position != "left" {
		t.Errorf("expected position left, got %s", bar.Spec.Position)
	}
	// Bar should be positioned to the left of the first cell
	if bar.Bounds.X >= result.Cells[0].CellBounds.X {
		t.Errorf("bar X (%d) should be less than cell X (%d)", bar.Bounds.X, result.Cells[0].CellBounds.X)
	}
}
