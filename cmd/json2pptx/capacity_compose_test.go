package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// go-slide-creator-yhzxt: capacity advice describes the block generation
// renders. A sparse slide-level pattern is composed (stepped type, grown
// rows, optical centre), so cell_budgets and the cells handed to a nested
// pattern are resolved under the same policy instead of at the un-stepped
// sizes.

func capacityComposeCtx() patterns.ExpandContext {
	return patterns.ExpandContext{
		SlideWidth:   12192000,
		SlideHeight:  6858000,
		LayoutBounds: patterns.LayoutBounds{X: 457200, Y: 1371600, Width: 11277600, Height: 4572000},
	}
}

func TestCellBudgetsResolveTheComposedBlock(t *testing.T) {
	ctx := capacityComposeCtx()
	pi := &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`[{"big":"42%","small":"Margin"},{"big":"3.1x","small":"Return"},{"big":"12","small":"Markets"}]`)}
	grid, _, err := expandPattern(pi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	capacity, bounds := resolveCapacityGrid(grid, ctx)
	if capacity == nil {
		t.Fatal("capacity grid did not resolve")
	}
	if !capacity.Composed {
		t.Fatal("cell_budgets resolved a sparse slide block without the composition policy")
	}
	// What generation resolves in the same rectangle.
	rendered := resolveGridForStructuralComposed(t, grid, bounds)
	if len(rendered.Cells) != len(capacity.Cells) {
		t.Fatalf("cells: capacity %d, rendered %d", len(capacity.Cells), len(rendered.Cells))
	}
	for i := range rendered.Cells {
		if capacity.Cells[i].Bounds != rendered.Cells[i].Bounds {
			t.Errorf("cell %d bounds: capacity %+v, rendered %+v", i, capacity.Cells[i].Bounds, rendered.Cells[i].Bounds)
		}
		if got, want := string(capacity.Cells[i].ShapeSpec.Text), string(rendered.Cells[i].ShapeSpec.Text); got != want {
			t.Errorf("cell %d text: capacity %s, rendered %s", i, got, want)
		}
	}
	// The block is the stepped one: its rows are taller than the pattern's
	// own max_height.
	if h := float64(capacity.Cells[0].CellBounds.CY) / 12700; h <= grid.Rows[0].MaxHeight {
		t.Errorf("capacity row height %.1fpt, want the row grown past its %.1fpt cap", h, grid.Rows[0].MaxHeight)
	}
}

// resolveGridForStructuralComposed resolves grid as a slide's own block in
// bounds, the way generation does.
func resolveGridForStructuralComposed(t *testing.T, grid *jsonschema.ShapeGridInput, bounds pptx.RectEmu) *shapegrid.ResolveResult {
	t.Helper()
	res, err := resolveShapeGridAs(grid, pptx.NewShapeIDAllocator(nil), &bounds, nil, 12192000, 6858000, nil, true)
	if err != nil || res == nil {
		t.Fatalf("resolve: %v", err)
	}
	return &shapegrid.ResolveResult{Cells: res.Cells}
}

func TestNestedPatternCellsFollowTheComposedParent(t *testing.T) {
	ctx := capacityComposeCtx()
	parent := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(`2`),
		VerticalAlign: "auto",
		Rows: []jsonschema.GridRowInput{{
			MaxHeight: 90,
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`{"content":"Context","size":12}`)}},
				{Pattern: json.RawMessage(`{"name":"kpi-2up","values":[{"big":"42%","small":"Margin"},{"big":"3.1x","small":"Return"}]}`)},
			},
		}},
	}
	area := pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	composed, err := nestedPatternCellBounds(parent, ctx, area, true)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := nestedPatternCellBounds(parent, ctx, area, false)
	if err != nil {
		t.Fatal(err)
	}
	key := [2]int{0, 1}
	if composed[key].CY <= plain[key].CY {
		t.Errorf("slide-block cell height %d, want it grown past the un-composed %d", composed[key].CY, plain[key].CY)
	}
	if composed[key].Y <= plain[key].Y {
		t.Errorf("slide-block cell top %d, want it below the top-hung %d (optical centre)", composed[key].Y, plain[key].Y)
	}
}
