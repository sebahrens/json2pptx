package slides

import (
	"strings"
	"testing"
)

// In main_left / main_right led by a cycle the two stacked regions meet at
// the stack's seam — the upper one set on it, the lower one hung from it —
// and the stack is not re-split by the engine (go-slide-creator-tgfdn). A
// stack beside any other main region keeps its top anchors and its stamp.
func TestStackBesideACycleMeetsAtItsSeam(t *testing.T) {
	loop := map[string]any{"kind": "cycle", "phases": []any{"Plan", "Do", "Check", "Act"}}
	text := map[string]any{"kind": "text", "heading": "What changed", "bullets": []any{"Rework is down", "One owner per phase"}}
	for _, arrangement := range []string{ArrangeMainLeft, ArrangeMainRight} {
		sideCol := 1
		if arrangement == ArrangeMainRight {
			sideCol = 0
		}
		grid := compileBesideCycle(t, arrangement, loop, besideCycleRegion(RegionKPIs, false), text)
		stack := grid.Rows[0].Cells[sideCol].Grid
		if stack == nil || len(stack.Rows) != 2 {
			t.Fatalf("%s: no two-row stack: %+v", arrangement, grid.Rows[0].Cells[sideCol])
		}
		if stack.Source != regionsGridSource {
			t.Errorf("%s: the stack beside a cycle carries the stamp %q, want the plain %q (no re-split)", arrangement, stack.Source, regionsGridSource)
		}
		if p := nestedPattern(t, stack.Rows[0].Cells[0]); p.VerticalAlign != regionAnchorBottom {
			t.Errorf("%s: upper region vertical_align = %q, want bottom", arrangement, p.VerticalAlign)
		}
		lower := stack.Rows[1].Cells[0]
		if lower.Shape == nil || !strings.Contains(string(lower.Shape.Text), `"vertical_align":"t"`) ||
			!strings.Contains(string(lower.Shape.Text), `{"content":"What changed","bold":true}`) {
			t.Errorf("%s: lower text region = %+v, want one top-anchored block led by its heading", arrangement, lower)
		}

		// Text above, a headed table below: bottom-anchored text, and the
		// table's heading and band as one block hung from the seam.
		grid = compileBesideCycle(t, arrangement, loop, text, besideCycleRegion(RegionTable, true))
		stack = grid.Rows[0].Cells[sideCol].Grid
		if upper := stack.Rows[0].Cells[0]; upper.Shape == nil || !strings.Contains(string(upper.Shape.Text), `"vertical_align":"b"`) {
			t.Errorf("%s: upper text region = %+v, want bottom-anchored", arrangement, upper)
		}
		if block := stack.Rows[1].Cells[0].Grid; block == nil || block.VerticalAlign != regionAnchorTop || block.Rows[1].MaxHeight != 3*regionTableRowPt {
			t.Errorf("%s: lower table region = %+v, want a top-anchored heading + band block", arrangement, block)
		}
	}

	chart := besideCycleRegion(RegionChart, false)
	grid := compileBesideCycle(t, ArrangeMainLeft, chart, besideCycleRegion(RegionKPIs, false), text)
	stack := grid.Rows[0].Cells[1].Grid
	if stack.Source == regionsGridSource || stack.Source == "" {
		t.Errorf("a stack beside a chart lost its re-split stamp: %q", stack.Source)
	}
	if p := nestedPattern(t, stack.Rows[0].Cells[0]); p.VerticalAlign != "" {
		t.Errorf("kpis stacked beside a chart carry vertical_align %q", p.VerticalAlign)
	}
	if lower := stack.Rows[1].Cells[0]; lower.Grid == nil {
		t.Errorf("a headed text region stacked beside a chart is no longer a heading row over its text: %+v", lower)
	}
}
