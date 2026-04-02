package shapegrid

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestValidate_NilGrid(t *testing.T) {
	if err := Validate(nil); err != nil {
		t.Errorf("expected nil error for nil grid, got %v", err)
	}
}

func TestValidate_EmptyColumns(t *testing.T) {
	grid := &Grid{
		Rows: []Row{{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for empty columns")
	}
	if !strings.Contains(err.Error(), "empty columns") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ColSpanExceedsGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{ColSpan: 3, Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for col_span exceeding grid")
	}
	if !strings.Contains(err.Error(), "col_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RowSpanExceedsGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for row_span exceeding grid")
	}
	if !strings.Contains(err.Error(), "row_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ShapeAndTableBothSet(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape:     &ShapeSpec{Geometry: "rect"},
					TableSpec: &types.TableSpec{Headers: []string{"A"}},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when both shape and table are set")
	}
	if !strings.Contains(err.Error(), "both shape and table") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_TableCellOnly(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{TableSpec: &types.TableSpec{Headers: []string{"A"}}},
			},
		}},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for valid grid with table cell, got %v", err)
	}
}

func TestValidate_ValidGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for valid grid, got %v", err)
	}
}

func TestValidate_NoRows(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for no rows")
	}
	if !strings.Contains(err.Error(), "no rows") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_CellOverlapFromRowSpan(t *testing.T) {
	// Row 0: cell at col 0 with col_span=2, row_span=2 occupies (0,0),(0,1),(1,0),(1,1)
	// Row 1: cell at col 0 starts, but skip logic moves to col 2 (out of range)
	//        so the second row's cell can't fit, producing a col_span exceeds error.
	// This validates that row_span reservations are detected as conflicts.
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}}, // pushed past occupied col 0
				{Shape: &ShapeSpec{Geometry: "rect"}}, // exceeds grid width
			}},
		},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for cell overflow from row_span")
	}
	if !strings.Contains(err.Error(), "col_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_EmptyGrid(t *testing.T) {
	grid := &Grid{}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for empty grid")
	}
}

func TestValidate_ValidLargeGrid(t *testing.T) {
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
	grid := &Grid{Columns: cols, Rows: rows}
	if err := Validate(grid); err != nil {
		t.Errorf("expected valid large grid, got %v", err)
	}
}

func TestValidate_CombinedSpansValid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{25, 25, 25, 25},
		Rows: []Row{
			{Cells: []Cell{
				{ColSpan: 2, RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				// cols 0-1 occupied
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected valid grid with combined spans, got %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	grid := &Grid{} // no columns AND no rows
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "columns") || !strings.Contains(errStr, "rows") {
		t.Errorf("expected errors about both columns and rows, got: %v", err)
	}
}
