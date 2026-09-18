package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// contentRect is a 16:9 content area below a title (EMU).
var contentRect = pptx.RectEmu{X: 457200, Y: 1400000, CX: 11277600, CY: 4700000}

func resolvePatternForTest(t *testing.T, name, values string) (*ShapeGridResult, *ShapeGridInput) {
	t.Helper()
	ctx := patterns.ExpandContext{
		SlideWidth:   12192000,
		SlideHeight:  6858000,
		LayoutBounds: patterns.LayoutBounds{X: contentRect.X, Y: contentRect.Y, Width: contentRect.CX, Height: contentRect.CY},
	}
	grid, _, err := expandPattern(&PatternInput{Name: name, Values: json.RawMessage(values)}, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expand %s: %v", name, err)
	}
	b := contentRect
	res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("resolve %s: %v", name, err)
	}
	return res, grid
}

func blockExtent(cells []shapegrid.ResolvedCell) (top, bottom int64) {
	top, bottom = 1<<62, 0
	for _, c := range cells {
		if c.CellBounds.Y < top {
			top = c.CellBounds.Y
		}
		if e := c.CellBounds.Y + c.CellBounds.CY; e > bottom {
			bottom = e
		}
	}
	return top, bottom
}

func assertCentred(t *testing.T, name string, cells []shapegrid.ResolvedCell) {
	t.Helper()
	top, bottom := blockExtent(cells)
	above := top - contentRect.Y
	below := contentRect.Y + contentRect.CY - bottom
	if d := above - below; d > 12700 || d < -12700 {
		t.Errorf("%s: block not vertically centred (above=%d below=%d)", name, above, below)
	}
}

// TestKPI3up_CardHeightCappedAndCentred covers go-slide-creator-7km8.
func TestKPI3up_CardHeightCappedAndCentred(t *testing.T) {
	res, grid := resolvePatternForTest(t, "kpi-3up", `[{"big":"$4.2M","small":"ARR added in H1","icon":"currency-dollar"},{"big":"127%","small":"Net revenue retention"},{"big":"12 days","small":"Median sales cycle"}]`)
	if grid.VerticalAlign != "center" {
		t.Errorf("pattern grid vertical_align = %q, want center", grid.VerticalAlign)
	}
	for _, c := range res.Cells {
		if float64(c.CellBounds.CY) > 0.45*float64(contentRect.CY)+12700 {
			t.Errorf("KPI card height %d > 45%% of content height %d", c.CellBounds.CY, contentRect.CY)
		}
	}
	assertCentred(t, "kpi-3up", res.Cells)
}

func TestProcessFlow_StepsCappedAndCentred(t *testing.T) {
	res, _ := resolvePatternForTest(t, "process-flow", `{"steps":[{"label":"Intake"},{"label":"Triage"},{"label":"Approve?","type":"decision"},{"label":"Close"}]}`)
	for _, c := range res.Cells {
		if float64(c.CellBounds.CY) > 0.35*float64(contentRect.CY)+12700 {
			t.Errorf("process step height %d > 35%% of content height", c.CellBounds.CY)
		}
	}
	assertCentred(t, "process-flow", res.Cells)
}

// TestTimelineDots_AxisAndDots: the default dots style draws real dots joined
// by connector lines (the axis), not full-height filled pillars.
func TestTimelineDots_AxisAndDots(t *testing.T) {
	res, _ := resolvePatternForTest(t, "timeline-horizontal", `[{"label":"Kickoff","date":"Jan"},{"label":"Pilot","date":"Mar"},{"label":"Scale","date":"Jun"}]`)
	dots := 0
	for _, c := range res.Cells {
		if c.ShapeSpec != nil && c.ShapeSpec.Geometry == "ellipse" {
			dots++
			if c.Bounds.CX != c.Bounds.CY {
				t.Errorf("dot must be round, got %dx%d", c.Bounds.CX, c.Bounds.CY)
			}
		}
		if float64(c.CellBounds.CY) > 0.4*float64(contentRect.CY)+12700 {
			t.Errorf("timeline row height %d too tall", c.CellBounds.CY)
		}
	}
	if dots != 3 {
		t.Errorf("want 3 dots, got %d", dots)
	}
	cxn := 0
	for _, sh := range res.Shapes {
		if strings.Contains(string(sh), "<p:cxnSp") {
			cxn++
		}
	}
	if cxn != 2 {
		t.Errorf("want 2 axis connectors between 3 dots, got %d", cxn)
	}
	assertCentred(t, "timeline-horizontal", res.Cells)
}

// TestHeightCappedPatternBoundsCentred: pattern-authored top-anchored bounds
// (before-after-compact caps at 60%) are centred in the content area.
func TestHeightCappedPatternBoundsCentred(t *testing.T) {
	res, _ := resolvePatternForTest(t, "before-after-compact", `{"before":{"header":"Today","items":["Slow"]},"after":{"header":"Target","items":["Fast"]}}`)
	assertCentred(t, "before-after-compact", res.Cells)
}

// TestMaxHeightPctStaysTopAnchored: a user bounds override keeps its
// documented top-left anchoring.
func TestMaxHeightPctStaysTopAnchored(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, _, err := expandPattern(&PatternInput{Name: "kpi-3up", MaxHeightPct: 40, Values: json.RawMessage(`["1 | a","2 | b","3 | c"]`)}, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	b := contentRect
	res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if top, _ := blockExtent(res.Cells); top != contentRect.Y {
		t.Errorf("max_height_pct block must stay at the top: y=%d want %d", top, contentRect.Y)
	}
}

func TestResolveShapeGrid_InvalidVerticalAlign(t *testing.T) {
	in := &ShapeGridInput{VerticalAlign: "sideways", Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	if _, err := resolveShapeGrid(in, pptx.NewShapeIDAllocator(nil), nil, nil, 0, 0, nil); err == nil {
		t.Error("invalid vertical_align must error")
	}
	_, _, errs, _ := validateShapeGrid(in, 1)
	if len(errs) == 0 {
		t.Error("validate must report invalid vertical_align")
	}
}
