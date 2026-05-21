package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// TestCollectResolvedGridCells_Basic verifies that a 2x2 shape grid resolves to
// four wireframe cells with correct row/col indices, kind, and EMU bounds that
// stay inside the slide and do not overlap.
func TestCollectResolvedGridCells_Basic(t *testing.T) {
	cols, _ := json.Marshal([]float64{50, 50})
	grid := &ShapeGridInput{
		Columns: cols,
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
					{Icon: &IconInput{Name: "chart-pie"}},
				},
			},
			{
				Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
				},
			},
		},
	}

	cells := collectResolvedGridCells(grid, GridGeometry{}, shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
	if len(cells) != 4 {
		t.Fatalf("expected 4 resolved cells, got %d", len(cells))
	}

	// Cells are emitted in row-major order. Verify (row, col) sequence.
	wantRC := [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}}
	for i, rc := range wantRC {
		if cells[i].Row != rc[0] || cells[i].Col != rc[1] {
			t.Errorf("cell[%d]: want (row=%d,col=%d), got (row=%d,col=%d)",
				i, rc[0], rc[1], cells[i].Row, cells[i].Col)
		}
	}

	// Kinds should be shape, icon, shape, shape.
	wantKinds := []string{"shape", "icon", "shape", "shape"}
	for i, k := range wantKinds {
		if cells[i].Kind != k {
			t.Errorf("cell[%d].kind: want %q, got %q", i, k, cells[i].Kind)
		}
	}

	// All cells must have positive width/height and fit inside slide bounds.
	for i, c := range cells {
		if c.W <= 0 || c.H <= 0 {
			t.Errorf("cell[%d]: non-positive size w=%d h=%d", i, c.W, c.H)
		}
		if c.X < 0 || c.Y < 0 {
			t.Errorf("cell[%d]: negative origin x=%d y=%d", i, c.X, c.Y)
		}
		if c.X+c.W > shapegrid.DefaultSlideWidthEMU {
			t.Errorf("cell[%d]: extends past slide width: x=%d w=%d slide=%d",
				i, c.X, c.W, shapegrid.DefaultSlideWidthEMU)
		}
		if c.Y+c.H > shapegrid.DefaultSlideHeightEMU {
			t.Errorf("cell[%d]: extends past slide height: y=%d h=%d slide=%d",
				i, c.Y, c.H, shapegrid.DefaultSlideHeightEMU)
		}
	}

	// Row-0 cells share a Y; row-1 cells share a different (greater) Y.
	if cells[0].Y != cells[1].Y {
		t.Errorf("row 0 cells should share Y: got %d vs %d", cells[0].Y, cells[1].Y)
	}
	if cells[2].Y != cells[3].Y {
		t.Errorf("row 1 cells should share Y: got %d vs %d", cells[2].Y, cells[3].Y)
	}
	if cells[2].Y <= cells[0].Y {
		t.Errorf("row 1 Y (%d) should be greater than row 0 Y (%d)", cells[2].Y, cells[0].Y)
	}

	// Column 0 cells share an X; column 1 cells share a different (greater) X.
	if cells[0].X != cells[2].X {
		t.Errorf("col 0 cells should share X: got %d vs %d", cells[0].X, cells[2].X)
	}
	if cells[1].X != cells[3].X {
		t.Errorf("col 1 cells should share X: got %d vs %d", cells[1].X, cells[3].X)
	}
	if cells[1].X <= cells[0].X {
		t.Errorf("col 1 X (%d) should be greater than col 0 X (%d)", cells[1].X, cells[0].X)
	}
}

// TestCollectResolvedGridCells_NilOrEmpty asserts that nil and empty grids
// return no cells (used for non-shape_grid slides).
func TestCollectResolvedGridCells_NilOrEmpty(t *testing.T) {
	if cells := collectResolvedGridCells(nil, GridGeometry{}, 0, 0); cells != nil {
		t.Errorf("nil grid: want nil cells, got %d", len(cells))
	}
	if cells := collectResolvedGridCells(&ShapeGridInput{}, GridGeometry{}, 0, 0); cells != nil {
		t.Errorf("empty grid: want nil cells, got %d", len(cells))
	}
}

// TestCollectResolvedGridCells_SkipsEmptyCells confirms cells with no shape,
// table, icon, image, or diagram content are excluded from the wireframe.
func TestCollectResolvedGridCells_SkipsEmptyCells(t *testing.T) {
	cols, _ := json.Marshal([]float64{50, 50})
	grid := &ShapeGridInput{
		Columns: cols,
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
					nil, // empty cell — resolver omits it
				},
			},
		},
	}

	cells := collectResolvedGridCells(grid, GridGeometry{}, shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
	if len(cells) != 1 {
		t.Fatalf("expected 1 resolved cell (empty cell omitted), got %d", len(cells))
	}
	if cells[0].Row != 0 || cells[0].Col != 0 || cells[0].Kind != "shape" {
		t.Errorf("unexpected cell: %+v", cells[0])
	}
}
