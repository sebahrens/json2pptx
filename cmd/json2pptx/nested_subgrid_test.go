package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// TestResolveShapeGrid_NestedSubGrid verifies that a cell with a nested
// ShapeGridInput (set directly via the Grid field) is rendered using the
// outer cell's bounds, and that the nested cells fit inside the outer cell
// without overlapping siblings.
func TestResolveShapeGrid_NestedSubGrid(t *testing.T) {
	innerGrid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"$4.2M"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"98%"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`), Text: json.RawMessage(`"1.2K"`)}},
			},
		}},
	}

	grid := &ShapeGridInput{
		Bounds:  &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 100},
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{
			{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
			}},
			{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`)}},
				{Grid: innerGrid}, // Nested sub-grid in bottom-right cell
			}},
		},
	}

	alloc := newAllocFrom(200)
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("resolveShapeGrid failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Parent: 3 outer shapes (top-left, top-right, bottom-left). The bottom-right
	// cell is a sub-grid placeholder (no shape itself). Inner kpi-3up contributes
	// 3 more shapes. Total: 6.
	if len(result.Shapes) != 6 {
		t.Fatalf("expected 6 shapes (3 outer + 3 nested), got %d", len(result.Shapes))
	}

	// Locate the sub-grid placeholder cell.
	var subgridCell *shapegrid.ResolvedCell
	for i := range result.Cells {
		if result.Cells[i].Kind == shapegrid.CellKindSubGrid {
			subgridCell = &result.Cells[i]
			break
		}
	}
	if subgridCell == nil {
		t.Fatal("expected a resolved cell with Kind=CellKindSubGrid")
	}
	if subgridCell.RowIdx != 1 || subgridCell.ColIdx != 1 {
		t.Errorf("subgrid placeholder at unexpected position: row=%d col=%d (want row=1 col=1)", subgridCell.RowIdx, subgridCell.ColIdx)
	}

	// Collect non-placeholder cells. The 3 nested cells must fit strictly inside
	// the sub-grid placeholder's bounds.
	parentBounds := subgridCell.Bounds
	nestedCount := 0
	for _, rc := range result.Cells {
		if rc.Kind == shapegrid.CellKindSubGrid {
			continue
		}
		// Skip the three outer-grid shape cells; identify nested ones by being
		// strictly contained within the sub-grid placeholder rect.
		if rectContains(parentBounds, rc.Bounds) {
			nestedCount++
		}
	}
	if nestedCount != 3 {
		t.Fatalf("expected 3 nested cells inside subgrid bounds, got %d", nestedCount)
	}

	// No two non-placeholder resolved cells overlap (excluding the placeholder
	// itself, which deliberately contains the nested cells).
	var renderCells []pptx.RectEmu
	for _, rc := range result.Cells {
		if rc.Kind == shapegrid.CellKindSubGrid {
			continue
		}
		renderCells = append(renderCells, rc.Bounds)
	}
	for i := 0; i < len(renderCells); i++ {
		for j := i + 1; j < len(renderCells); j++ {
			if rectsOverlap(renderCells[i], renderCells[j]) {
				t.Errorf("rendered cells overlap: %+v and %+v", renderCells[i], renderCells[j])
			}
		}
	}
}

// TestExpandNestedCellPatterns_KPI3UpInMatrixCell verifies that a cell-level
// Pattern (kpi-3up) is expanded into the cell's Grid by expandNestedCellPatterns,
// and that the resulting nested grid renders alongside the outer 2x2 cells.
func TestExpandNestedCellPatterns_KPI3UpInMatrixCell(t *testing.T) {
	grid := &ShapeGridInput{
		Bounds:  &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 100},
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{
			{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
			}},
			{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`)}},
				{Pattern: json.RawMessage(`{
					"name": "kpi-3up",
					"values": [
						{"big": "$4.2M", "small": "ARR"},
						{"big": "98%", "small": "Uptime"},
						{"big": "1.2K", "small": "Users"}
					]
				}`)},
			}},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	if err := expandNestedCellPatterns(grid, ctx, patterns.Default()); err != nil {
		t.Fatalf("expandNestedCellPatterns failed: %v", err)
	}

	// After expansion, the bottom-right cell's Pattern must be cleared and its
	// Grid populated with the expanded ShapeGridInput.
	cell := grid.Rows[1].Cells[1]
	if len(cell.Pattern) != 0 {
		t.Error("cell.Pattern should be cleared after expansion")
	}
	if cell.Grid == nil {
		t.Fatal("cell.Grid should be populated with the expanded sub-grid")
	}
	if len(cell.Grid.Rows) == 0 {
		t.Fatal("expanded sub-grid has no rows")
	}

	// Now resolve and ensure the nested kpi-3up renders without overlap.
	alloc := newAllocFrom(200)
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("resolveShapeGrid failed: %v", err)
	}
	if result == nil || len(result.Cells) == 0 {
		t.Fatal("expected resolved cells")
	}

	// The placeholder cell must exist and be at (row=1, col=1).
	found := false
	var parentBounds pptx.RectEmu
	for _, rc := range result.Cells {
		if rc.Kind == shapegrid.CellKindSubGrid {
			if rc.RowIdx == 1 && rc.ColIdx == 1 {
				found = true
				parentBounds = rc.Bounds
			}
		}
	}
	if !found {
		t.Fatal("expected a CellKindSubGrid placeholder at row=1 col=1")
	}

	// At least one rendered cell must fall inside the placeholder bounds.
	inside := 0
	for _, rc := range result.Cells {
		if rc.Kind == shapegrid.CellKindSubGrid {
			continue
		}
		if rectContains(parentBounds, rc.Bounds) {
			inside++
		}
	}
	if inside == 0 {
		t.Error("expected at least one rendered cell inside the placeholder bounds")
	}
}

// TestExpandNestedCellPatterns_PatternAndGridConflict verifies the mutual
// exclusion: a cell cannot set both Pattern and Grid.
func TestExpandNestedCellPatterns_PatternAndGridConflict(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{{
				Pattern: json.RawMessage(`{"name":"kpi-3up","values":[]}`),
				Grid:    &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{}}}}},
			}},
		}},
	}

	err := expandNestedCellPatterns(grid, patterns.ExpandContext{}, patterns.Default())
	if err == nil {
		t.Fatal("expected error for cell with both Pattern and Grid")
	}
}

// rectContains returns true if outer strictly contains inner (inclusive of
// touching edges).
func rectContains(outer, inner pptx.RectEmu) bool {
	return inner.X >= outer.X &&
		inner.Y >= outer.Y &&
		inner.X+inner.CX <= outer.X+outer.CX &&
		inner.Y+inner.CY <= outer.Y+outer.CY
}

// rectsOverlap returns true if a and b share interior area (touching edges
// do not count as overlap).
func rectsOverlap(a, b pptx.RectEmu) bool {
	if a.X+a.CX <= b.X || b.X+b.CX <= a.X {
		return false
	}
	if a.Y+a.CY <= b.Y || b.Y+b.CY <= a.Y {
		return false
	}
	return true
}
