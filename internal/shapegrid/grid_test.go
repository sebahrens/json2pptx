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
	// The auto row gets its estimated content height (not the full remaining space).
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
	// Auto row gets its estimated content height (clamped between 8-80%).
	// With 3 lines of 11pt text + 12pt inset, it should be a small fraction of 5M EMU.
	if heights[1] < 8 || heights[1] > 80 {
		t.Errorf("auto row: expected 8-80%%, got %f", heights[1])
	}
	// Total should equal header + auto allocation
	total := heights[0] + heights[1]
	if total < 20 || total > 100 {
		t.Errorf("total should be between 20-100%%, got %f", total)
	}
}

func TestResolveRowHeights_AutoHeightWithUnspecified(t *testing.T) {
	// Header at 20%, auto-height row, and an unspecified (flex=1) row.
	// Auto row gets its content estimate; flex row gets the remainder.
	rows := []Row{
		{Height: 20},
		{
			AutoHeight: true,
			Cells: []Cell{{Shape: &ShapeSpec{
				Geometry: "rect",
				Text:     json.RawMessage(`{"content":"Short","size":11}`),
			}}},
		},
		{}, // unspecified → flex=1
	}
	gridH := int64(5000000)
	heights := resolveRowHeights(rows, gridH)

	if heights[0] != 20 {
		t.Errorf("header row: expected 20, got %f", heights[0])
	}
	// Auto row gets its estimated content height (small for "Short" text)
	if heights[1] < 8 || heights[1] > 80 {
		t.Errorf("auto row: expected 8-80%%, got %f", heights[1])
	}
	// Flex row gets the remaining space
	remaining := 100.0 - heights[0] - heights[1]
	if heights[2] < remaining-0.01 || heights[2] > remaining+0.01 {
		t.Errorf("flex row: expected ~%.1f%%, got %f", remaining, heights[2])
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

func TestResolve_AllZeroHeights_BoundsAuthoritative(t *testing.T) {
	// Bounds are authoritative — they must never shrink even when all rows
	// have zero height. Rows get flex-distributed shares of the available space.
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

	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Bounds must remain unchanged (authoritative, never shrink).
	if grid.Bounds.CY != 5000000 {
		t.Errorf("expected grid CY to remain 5000000, got %d", grid.Bounds.CY)
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

func TestResolve_FlexRows_EvenDistribution(t *testing.T) {
	// All rows with zero height (default flex=1) should distribute evenly.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000},
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short"`)}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"A"`)}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short"`)}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"B"`)}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"Short"`)}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`"C"`)}},
			}},
		},
	}

	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Bounds should remain unchanged (authoritative).
	if grid.Bounds.CY != 5000000 {
		t.Errorf("expected grid CY to remain 5000000, got %d", grid.Bounds.CY)
	}

	// All 3 cells per row should have approximately equal height.
	if len(result.Cells) >= 3 {
		h0 := result.Cells[0].Bounds.CY
		h2 := result.Cells[2].Bounds.CY
		// Heights should be within 1 EMU of each other (rounding).
		diff := h0 - h2
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("expected equal row heights, got %d and %d (diff %d)", h0, h2, diff)
		}
	}
}

func TestResolveRowHeights_FlexWeights(t *testing.T) {
	// Two flex rows with weights 2:1 should get 2/3 and 1/3 of available space.
	rows := []Row{
		{Flex: 2}, // 2x weight
		{Flex: 1}, // 1x weight
	}
	gridH := int64(3000000) // 3M EMU
	heights := resolveRowHeights(rows, gridH)

	// Row 0 should get ~66.67%, row 1 ~33.33%
	if heights[0] < 66 || heights[0] > 67 {
		t.Errorf("flex=2 row: expected ~66.67%%, got %f", heights[0])
	}
	if heights[1] < 33 || heights[1] > 34 {
		t.Errorf("flex=1 row: expected ~33.33%%, got %f", heights[1])
	}
}

func TestResolveRowHeights_FixedPlusFlexPlusFlex(t *testing.T) {
	// 30% fixed + two flex rows (default flex=1) should split remaining 70%.
	rows := []Row{
		{Height: 30},
		{}, // flex=1 (default)
		{}, // flex=1 (default)
	}
	gridH := int64(5000000)
	heights := resolveRowHeights(rows, gridH)

	if heights[0] != 30 {
		t.Errorf("fixed row: expected 30, got %f", heights[0])
	}
	// Each flex row gets 35% of total
	if heights[1] < 34.9 || heights[1] > 35.1 {
		t.Errorf("flex row 1: expected ~35%%, got %f", heights[1])
	}
	if heights[2] < 34.9 || heights[2] > 35.1 {
		t.Errorf("flex row 2: expected ~35%%, got %f", heights[2])
	}
}

func TestResolveRowHeights_MinHeight(t *testing.T) {
	// Two flex rows, first with min_height that's larger than its flex share.
	// 100pt in 2000000 EMU total = 100*12700/2000000 * 100 ≈ 63.5%
	rows := []Row{
		{MinHeight: 100}, // min 100pt ≈ 63.5% of 2M EMU
		{},               // flex=1
	}
	gridH := int64(2000000)
	heights := resolveRowHeights(rows, gridH)

	minPct := (100.0 * 12700) / float64(gridH) * 100.0
	if heights[0] < minPct-0.1 {
		t.Errorf("row 0: expected >= %.1f%% (min_height), got %f", minPct, heights[0])
	}
}

func TestResolveRowHeights_MaxHeight(t *testing.T) {
	// Single flex row with max_height constraint.
	// 50pt in 5000000 EMU = 50*12700/5000000 * 100 ≈ 12.7%
	rows := []Row{
		{MaxHeight: 50}, // max 50pt
		{},              // flex=1 gets the rest
	}
	gridH := int64(5000000)
	heights := resolveRowHeights(rows, gridH)

	maxPct := (50.0 * 12700) / float64(gridH) * 100.0
	if heights[0] > maxPct+0.1 {
		t.Errorf("row 0: expected <= %.1f%% (max_height), got %f", maxPct, heights[0])
	}
	// Row 1 should get the remaining space
	if heights[1] < 100-maxPct-0.1 {
		t.Errorf("row 1: expected ~%.1f%%, got %f", 100-maxPct, heights[1])
	}
}

func TestResolve_RowOverflow(t *testing.T) {
	// A row with max_height = 20pt but content that exceeds it should
	// report a RowOverflow in the result.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000},
		Columns: []float64{100},
		Rows: []Row{
			{
				MaxHeight: 20, // 20pt max
				Cells: []Cell{{Shape: &ShapeSpec{
					Geometry: "rect",
					// Many lines will exceed 20pt
					Text: json.RawMessage(`{"content":"Line1\nLine2\nLine3\nLine4\nLine5\nLine6","size":14}`),
				}}},
			},
		},
	}

	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.RowOverflows) == 0 {
		t.Error("expected at least one RowOverflow")
	} else {
		ov := result.RowOverflows[0]
		if ov.RowIndex != 0 {
			t.Errorf("expected RowIndex 0, got %d", ov.RowIndex)
		}
		if ov.MaxHeightPt != 20 {
			t.Errorf("expected MaxHeightPt 20, got %f", ov.MaxHeightPt)
		}
		if ov.ContentPt <= 20 {
			t.Errorf("expected ContentPt > 20, got %f", ov.ContentPt)
		}
	}
}

func TestResolve_NoRowOverflowWhenFits(t *testing.T) {
	// Content that fits within max_height should not produce an overflow.
	grid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 8229600, CY: 5000000},
		Columns: []float64{100},
		Rows: []Row{
			{
				MaxHeight: 200, // generous max
				Cells: []Cell{{Shape: &ShapeSpec{
					Geometry: "rect",
					Text:     json.RawMessage(`"Short"`),
				}}},
			},
		},
	}

	alloc := newAlloc(100)
	result, err := Resolve(grid, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RowOverflows) != 0 {
		t.Errorf("expected no RowOverflows, got %d", len(result.RowOverflows))
	}
}

func TestPtToPct(t *testing.T) {
	// 100pt in 5000000 EMU = (100*12700)/5000000 * 100 = 25.4%
	got := ptToPct(100, 5000000)
	if got < 25.3 || got > 25.5 {
		t.Errorf("ptToPct(100, 5000000) = %f, want ~25.4", got)
	}

	// Zero/negative inputs
	if ptToPct(0, 5000000) != 0 {
		t.Error("ptToPct(0, ...) should be 0")
	}
	if ptToPct(100, 0) != 0 {
		t.Error("ptToPct(..., 0) should be 0")
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

// TestResolve_DiagramRespectsExplicitFit verifies that diagram cells round-trip
// user-specified cell.fit values through ApplyFitMode, just like icons and
// images. Unlike icons/images, diagrams do NOT auto-switch to FitContain — the
// default remains FitStretch so the chart fills the full cell.
func TestResolve_DiagramRespectsExplicitFit(t *testing.T) {
	tests := []struct {
		name        string
		fit         FitMode
		wantCX      int64
		wantCY      int64
		wantX       int64
		wantY       int64
	}{
		{name: "default stretch", fit: FitStretch, wantCX: 4000, wantCY: 2000, wantX: 0, wantY: 0},
		{name: "explicit contain", fit: FitContain, wantCX: 2000, wantCY: 2000, wantX: 1000, wantY: 0},
		{name: "explicit fit-width", fit: FitWidth, wantCX: 4000, wantCY: 4000, wantX: 0, wantY: -1000},
		{name: "explicit fit-height", fit: FitHeight, wantCX: 2000, wantCY: 2000, wantX: 1000, wantY: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			grid := &Grid{
				Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 2000},
				Columns: []float64{100},
				Rows: []Row{{
					Cells: []Cell{
						{Fit: tc.fit, DiagramSpec: &types.DiagramSpec{Type: "bar_chart"}},
					},
				}},
			}

			result, err := Resolve(grid, newAlloc(1))
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			if len(result.Cells) != 1 {
				t.Fatalf("expected 1 cell, got %d", len(result.Cells))
			}
			c := result.Cells[0]
			if c.Kind != CellKindDiagram {
				t.Errorf("kind = %q, want %q", c.Kind, CellKindDiagram)
			}
			// CellBounds is always the original cell rect; Bounds is post-fit.
			if c.CellBounds.CX != 4000 || c.CellBounds.CY != 2000 {
				t.Errorf("CellBounds = %dx%d, want 4000x2000", c.CellBounds.CX, c.CellBounds.CY)
			}
			if c.Bounds.CX != tc.wantCX || c.Bounds.CY != tc.wantCY {
				t.Errorf("Bounds size = %dx%d, want %dx%d", c.Bounds.CX, c.Bounds.CY, tc.wantCX, tc.wantCY)
			}
			if c.Bounds.X != tc.wantX || c.Bounds.Y != tc.wantY {
				t.Errorf("Bounds origin = (%d,%d), want (%d,%d)", c.Bounds.X, c.Bounds.Y, tc.wantX, tc.wantY)
			}
		})
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

func TestResolve_ShapeWithIconOverlay_ParagraphsText(t *testing.T) {
	// A cell whose text is authored as a paragraphs array with non-empty content
	// must be treated as text-present, so the icon auto-positions to top/left with
	// text insets rather than centering with no inset.
	wideGrid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{
					Fill: json.RawMessage(`"accent1"`),
					Text: json.RawMessage(`{"paragraphs":[{"content":"hello"},{"content":"world"}]}`),
				},
				Icon: &IconSpec{Name: "shield"}, // no explicit position
			}},
		}},
	}
	result, err := Resolve(wideGrid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	// Wide cell with paragraph text → left position → left text inset set.
	if result.Cells[0].TextInsets[0] == 0 {
		t.Error("wide cell with paragraphs text should auto-detect 'left' position with left text inset")
	}

	// Empty paragraphs (and empty content) remain text-absent → center, no insets.
	emptyGrid := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 0, CX: 4000, CY: 1000},
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{{
				Shape: &ShapeSpec{
					Fill: json.RawMessage(`"accent1"`),
					Text: json.RawMessage(`{"paragraphs":[{"content":""},{"content":"   "}]}`),
				},
				Icon: &IconSpec{Name: "shield"},
			}},
		}},
	}
	result2, err := Resolve(emptyGrid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if result2.Cells[0].TextInsets[0] != 0 || result2.Cells[0].TextInsets[1] != 0 {
		t.Error("cell with empty paragraphs should auto-detect 'center' position with no text insets")
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

func TestResolve_CompositeSplitTopDefault(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
					},
				},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != 2 {
		t.Fatalf("expected composite to expand into 2 resolved cells, got %d", len(result.Cells))
	}
	text, diag := result.Cells[0], result.Cells[1]
	if text.Kind != CellKindShape || text.ShapeSpec == nil {
		t.Errorf("expected first sub-cell to be a shape carrying text, got kind=%s spec=%v", text.Kind, text.ShapeSpec)
	}
	if diag.Kind != CellKindDiagram || diag.DiagramSpec == nil {
		t.Errorf("expected second sub-cell to be a diagram, got kind=%s spec=%v", diag.Kind, diag.DiagramSpec)
	}
	// Text on top: text.Y should be <= diag.Y, and the two halves should not overlap.
	if text.Bounds.Y > diag.Bounds.Y {
		t.Errorf("expected text on top (text.Y=%d, diag.Y=%d)", text.Bounds.Y, diag.Bounds.Y)
	}
	if text.Bounds.Y+text.Bounds.CY > diag.Bounds.Y {
		t.Errorf("text and diagram halves overlap: text bottom=%d, diag top=%d", text.Bounds.Y+text.Bounds.CY, diag.Bounds.Y)
	}
	// Both halves share the cell's RowIdx/ColIdx so downstream code can group them.
	if text.RowIdx != diag.RowIdx || text.ColIdx != diag.ColIdx {
		t.Errorf("composite halves should share (row,col): text=(%d,%d) diag=(%d,%d)", text.RowIdx, text.ColIdx, diag.RowIdx, diag.ColIdx)
	}
}

func TestResolve_CompositeSplitBottom(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
						Split:      CompositeSplitBottom,
					},
				},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != 2 {
		t.Fatalf("expected 2 resolved cells, got %d", len(result.Cells))
	}
	text, diag := result.Cells[0], result.Cells[1]
	if text.Kind != CellKindShape || diag.Kind != CellKindDiagram {
		t.Fatalf("unexpected kinds: text=%s diag=%s", text.Kind, diag.Kind)
	}
	// Text on bottom: diag.Y should be less than text.Y.
	if diag.Bounds.Y >= text.Bounds.Y {
		t.Errorf("expected diagram above text (diag.Y=%d, text.Y=%d)", diag.Bounds.Y, text.Bounds.Y)
	}
}

func TestResolve_CompositeRatio(t *testing.T) {
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
						Ratio:      0.25, // text gets 25%, diagram 75%
					},
				},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != 2 {
		t.Fatalf("expected 2 resolved cells, got %d", len(result.Cells))
	}
	text, diag := result.Cells[0], result.Cells[1]
	if text.Bounds.CY >= diag.Bounds.CY {
		t.Errorf("expected diagram half (CY=%d) larger than text half (CY=%d) at ratio=0.25", diag.Bounds.CY, text.Bounds.CY)
	}
}

func TestResolve_CompositeWithAccentBar(t *testing.T) {
	// A composite cell with an accent_bar should produce exactly one accent bar
	// spanning the whole outer cell rectangle (not one per sub-half).
	_ = json.RawMessage("") // silence unused-import warning on encoding/json in narrow builds
	grid := &Grid{
		Bounds:  DefaultBounds(0, 0),
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
					},
					AccentBar: &AccentBarSpec{Position: "left", Color: "accent1", Width: 4},
				},
			},
		}},
	}
	result, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AccentBars) != 1 {
		t.Fatalf("expected exactly 1 accent bar for composite cell, got %d", len(result.AccentBars))
	}
	// Bar height should span both halves (≈ full row height, modulo the inter-inset).
	text, diag := result.Cells[0], result.Cells[1]
	bar := result.AccentBars[0]
	combinedSpan := (diag.Bounds.Y + diag.Bounds.CY) - text.Bounds.Y
	if text.Bounds.Y > diag.Bounds.Y {
		combinedSpan = (text.Bounds.Y + text.Bounds.CY) - diag.Bounds.Y
	}
	if bar.Bounds.CY < combinedSpan-1 {
		t.Errorf("accent bar should span both halves (CY=%d, combined span=%d)", bar.Bounds.CY, combinedSpan)
	}
}

func TestSplitCompositeBounds_Default(t *testing.T) {
	cell := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 1000000}
	spec := &CompositeSpec{}
	textRect, diagRect := splitCompositeBounds(cell, spec)
	if textRect.Y != 0 {
		t.Errorf("default split: text should start at Y=0, got %d", textRect.Y)
	}
	if diagRect.Y <= textRect.Y+textRect.CY-1 {
		t.Errorf("default split: diagram should start after text, got text bottom=%d diag top=%d", textRect.Y+textRect.CY, diagRect.Y)
	}
	if textRect.X != cell.X || diagRect.X != cell.X {
		t.Errorf("composite halves should share X with parent cell")
	}
	if textRect.CX != cell.CX || diagRect.CX != cell.CX {
		t.Errorf("composite halves should span the full cell width")
	}
}

// TestResolve_CardRowGeometry locks in equal card widths, uniform gutters,
// and horizontal centering for kpi-Nup / card-grid layouts. Regression test
// for go-slide-creator-sx2u: visible mis-alignment in kpi-3up, kpi-4up, and
// card-grid rows.
//
// Acceptance criteria:
//   - Cells in the same row have widths within 1 EMU.
//   - Inter-cell gutters are uniform within 1 EMU.
//   - The first cell starts at the grid's left edge and the last cell ends at
//     the grid's right edge (the row spans the full content area, so the
//     surrounding slide margins are symmetric whenever the grid bounds are).
func TestResolve_CardRowGeometry(t *testing.T) {
	cases := []struct {
		name    string
		columns int
		rows    int
		gapPt   float64
	}{
		{name: "kpi-2up", columns: 2, rows: 1, gapPt: 12},
		{name: "kpi-3up", columns: 3, rows: 1, gapPt: 12},
		{name: "kpi-4up", columns: 4, rows: 1, gapPt: 12},
		{name: "kpi-5up", columns: 5, rows: 1, gapPt: 12},
		{name: "kpi-6up", columns: 6, rows: 1, gapPt: 12},
		{name: "card-grid 3x2", columns: 3, rows: 2, gapPt: 10},
		{name: "card-grid 2x3", columns: 2, rows: 3, gapPt: 10},
		{name: "card-grid 3x1", columns: 3, rows: 1, gapPt: 10},
	}

	bounds := DefaultBounds(0, 0)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rowSpecs := make([]Row, tc.rows)
			for r := 0; r < tc.rows; r++ {
				cells := make([]Cell, tc.columns)
				for c := 0; c < tc.columns; c++ {
					cells[c] = Cell{Shape: &ShapeSpec{Geometry: "roundRect"}}
				}
				rowSpecs[r] = Row{Cells: cells}
			}
			cols := make([]float64, tc.columns)
			each := 100.0 / float64(tc.columns)
			for i := range cols {
				cols[i] = each
			}
			grid := &Grid{
				Bounds:  bounds,
				Columns: cols,
				Rows:    rowSpecs,
				ColGap:  tc.gapPt,
				RowGap:  tc.gapPt,
			}

			result, err := Resolve(grid, newAlloc(100))
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if result == nil || len(result.Cells) != tc.columns*tc.rows {
				t.Fatalf("expected %d cells, got %d", tc.columns*tc.rows, len(result.Cells))
			}

			// Group cells by row to validate per-row geometry.
			byRow := make(map[int][]ResolvedCell)
			for _, cell := range result.Cells {
				byRow[cell.RowIdx] = append(byRow[cell.RowIdx], cell)
			}

			expectedGapEMU := PtToEMU(tc.gapPt)

			for r := 0; r < tc.rows; r++ {
				rowCells := byRow[r]
				if len(rowCells) != tc.columns {
					t.Fatalf("row %d: expected %d cells, got %d", r, tc.columns, len(rowCells))
				}

				// Widths within 1 EMU of each other.
				minW, maxW := rowCells[0].Bounds.CX, rowCells[0].Bounds.CX
				for _, c := range rowCells[1:] {
					if c.Bounds.CX < minW {
						minW = c.Bounds.CX
					}
					if c.Bounds.CX > maxW {
						maxW = c.Bounds.CX
					}
				}
				if maxW-minW > 1 {
					t.Errorf("row %d: card widths span %d EMU (min=%d max=%d); want ≤ 1", r, maxW-minW, minW, maxW)
				}

				// Uniform gutters (within 1 EMU).
				for i := 0; i < len(rowCells)-1; i++ {
					left := rowCells[i]
					right := rowCells[i+1]
					gap := right.Bounds.X - (left.Bounds.X + left.Bounds.CX)
					diff := gap - expectedGapEMU
					if diff < -1 || diff > 1 {
						t.Errorf("row %d gutter %d→%d: got %d EMU, want %d (±1)", r, i, i+1, gap, expectedGapEMU)
					}
				}

				// Row spans the grid bounds: first cell starts at grid.X and
				// last cell ends at grid.X+grid.CX. Combined with symmetric
				// bounds (DefaultBounds is symmetric), this guarantees the
				// row is centered against the slide content area.
				first := rowCells[0]
				last := rowCells[len(rowCells)-1]
				if first.Bounds.X != bounds.X {
					t.Errorf("row %d: first cell X=%d, want grid X=%d", r, first.Bounds.X, bounds.X)
				}
				lastRight := last.Bounds.X + last.Bounds.CX
				gridRight := bounds.X + bounds.CX
				if lastRight != gridRight {
					t.Errorf("row %d: last cell right edge=%d, want grid right=%d (diff %d)", r, lastRight, gridRight, gridRight-lastRight)
				}
			}
		})
	}
}

// TestResolve_BannerRowOuterEdgeAlignment locks in that a row-0 full-width
// banner (col_span = numCols) and any subsequent multi-cell rows share the
// same outer edges. Regression test for go-slide-creator-rebg: bottom row's
// total width must never exceed (or fall short of) the banner's, regardless
// of column-width distribution or gap size.
//
// Acceptance criteria:
//   - First cell in every row starts at the same X (grid.X).
//   - Last cell in every row ends at the same X (grid.X + grid.CX).
//   - Outer-edge difference between any two rows is 0 EMU.
func TestResolve_BannerRowOuterEdgeAlignment(t *testing.T) {
	cases := []struct {
		name    string
		columns []float64
		gapPt   float64
		extraRows int // additional equal-column rows after the banner
	}{
		{name: "5-col equal banner+1 row, gap 16", columns: []float64{20, 20, 20, 20, 20}, gapPt: 16, extraRows: 1},
		{name: "5-col equal banner+2 rows, gap 4", columns: []float64{20, 20, 20, 20, 20}, gapPt: 4, extraRows: 2},
		{name: "3-col equal banner+1 row, gap 12", columns: []float64{100.0 / 3, 100.0 / 3, 100.0 / 3}, gapPt: 12, extraRows: 1},
		{name: "5-col weighted banner+1 row, gap 8", columns: []float64{17, 18, 23, 19, 23}, gapPt: 8, extraRows: 1},
		{name: "4-col banner+3 rows, gap 10", columns: []float64{25, 25, 25, 25}, gapPt: 10, extraRows: 3},
		{name: "2-col asymmetric banner+1 row, gap 6", columns: []float64{8, 92}, gapPt: 6, extraRows: 1},
	}

	bounds := DefaultBounds(0, 0)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			numCols := len(tc.columns)
			rows := []Row{{
				Height: 12,
				Cells: []Cell{{
					ColSpan: numCols,
					Shape:   &ShapeSpec{Geometry: "rect"},
				}},
			}}
			for r := 0; r < tc.extraRows; r++ {
				cells := make([]Cell, numCols)
				for c := 0; c < numCols; c++ {
					cells[c] = Cell{Shape: &ShapeSpec{Geometry: "rect"}}
				}
				rows = append(rows, Row{Cells: cells})
			}

			grid := &Grid{
				Bounds:  bounds,
				Columns: tc.columns,
				Rows:    rows,
				ColGap:  tc.gapPt,
				RowGap:  tc.gapPt,
			}

			result, err := Resolve(grid, newAlloc(100))
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}

			// Group cells by row index.
			byRow := make(map[int][]ResolvedCell)
			for _, c := range result.Cells {
				byRow[c.RowIdx] = append(byRow[c.RowIdx], c)
			}

			banner := byRow[0]
			if len(banner) != 1 {
				t.Fatalf("row 0 expected 1 banner cell, got %d", len(banner))
			}
			bannerLeft := banner[0].Bounds.X
			bannerRight := banner[0].Bounds.X + banner[0].Bounds.CX

			if bannerLeft != bounds.X {
				t.Errorf("banner left=%d, want grid X=%d (diff %d)", bannerLeft, bounds.X, bounds.X-bannerLeft)
			}
			gridRight := bounds.X + bounds.CX
			if bannerRight != gridRight {
				t.Errorf("banner right=%d, want grid right=%d (diff %d)", bannerRight, gridRight, gridRight-bannerRight)
			}

			for r := 1; r <= tc.extraRows; r++ {
				rowCells := byRow[r]
				if len(rowCells) != numCols {
					t.Fatalf("row %d expected %d cells, got %d", r, numCols, len(rowCells))
				}
				first := rowCells[0]
				last := rowCells[len(rowCells)-1]
				rowLeft := first.Bounds.X
				rowRight := last.Bounds.X + last.Bounds.CX

				if rowLeft != bannerLeft {
					t.Errorf("row %d left=%d, banner left=%d (diff %d)", r, rowLeft, bannerLeft, bannerLeft-rowLeft)
				}
				if rowRight != bannerRight {
					t.Errorf("row %d right=%d, banner right=%d (diff %d)", r, rowRight, bannerRight, bannerRight-rowRight)
				}
			}
		})
	}
}

// TestDefaultBounds_HorizontallyCentered locks in the invariant that
// shapegrid.DefaultBounds places content with symmetric horizontal margins,
// so any centered row span fills the bounds and ends up centered on the
// slide.
func TestDefaultBounds_HorizontallyCentered(t *testing.T) {
	cases := []struct {
		name   string
		width  int64
		height int64
	}{
		{"16:9 default", 0, 0},
		{"4:3 explicit", 9144000, 6858000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := DefaultBounds(tc.width, tc.height)
			sw := tc.width
			if sw <= 0 {
				sw = DefaultSlideWidthEMU
			}
			leftMargin := b.X
			rightMargin := sw - (b.X + b.CX)
			if leftMargin != rightMargin {
				t.Errorf("asymmetric horizontal margins: left=%d right=%d (diff %d)", leftMargin, rightMargin, leftMargin-rightMargin)
			}
		})
	}
}

// TestDefaultBoundsFromZone_HorizontallyCentered locks in that a
// ContentZone with symmetric LeftMargin/RightEdge produces grid bounds
// whose horizontal margins are symmetric against the slide width.
func TestDefaultBoundsFromZone_HorizontallyCentered(t *testing.T) {
	zone := ContentZone{
		TitleBottom: 1690688,
		FooterTop:   6356350,
		LeftMargin:  838200,
		RightEdge:   DefaultSlideWidthEMU - 838200,
		SlideWidth:  DefaultSlideWidthEMU,
		SlideHeight: DefaultSlideHeightEMU,
	}
	b := DefaultBoundsFromZone(zone, 9.0)
	leftMargin := b.X
	rightMargin := zone.SlideWidth - (b.X + b.CX)
	if leftMargin != rightMargin {
		t.Errorf("DefaultBoundsFromZone: asymmetric margins left=%d right=%d", leftMargin, rightMargin)
	}
}
