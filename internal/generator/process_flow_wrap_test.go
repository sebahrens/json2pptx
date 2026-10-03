package generator

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-6shxx: the native process_flow diagram drew seven small
// outlined boxes in one strip across the middle of the body placeholder, with
// the text shrunk below body size and a look unrelated to the process-flow
// pattern's.

// orderFulfilmentSteps is the bundled examples/diagrams/process_flow.json flow.
func orderFulfilmentSteps(n int, descriptions bool) []processFlowStep {
	all := [][2]string{
		{"Order Received", "Customer places order"},
		{"Validate Order", "Check order details"},
		{"Process Payment", "Authorize payment"},
		{"Check Inventory", "Verify stock levels"},
		{"Fulfill Order", "Pick and pack items"},
		{"Ship Package", "Hand off to carrier"},
		{"Notify Customer", "Send tracking info"},
		{"Close Order", "Archive the record"},
		{"Review", "Measure cycle time"},
		{"Improve", "Fix the bottleneck"},
	}
	steps := make([]processFlowStep, n)
	for i := range steps {
		steps[i] = processFlowStep{id: fmt.Sprintf("s%d", i), label: all[i][0], stepType: pfStepType}
		if descriptions {
			steps[i].description = all[i][1]
		}
	}
	return steps
}

// bodyPlaceholder is a 16:9 body placeholder: about 11" x 4.5".
var bodyPlaceholder = types.BoundingBox{X: 600000, Y: 1600000, Width: 10972800, Height: 4114800}

func TestProcessFlowWrapsToReadableRows(t *testing.T) {
	for _, descriptions := range []bool{true, false} {
		steps := orderFulfilmentSteps(7, descriptions)
		layout := computeProcessFlowLayout(steps, generateSequentialFlowConnections(steps), bodyPlaceholder, "horizontal", "Arial")
		if layout.direction != "horizontal" {
			t.Fatalf("descriptions=%t: a seven-step flow went %s", descriptions, layout.direction)
		}

		// Rows by y: 4 + 3, never 7 in a strip.
		rowOf := map[int64][]int{}
		var ys []int64
		for i, box := range layout.steps {
			cy := box.y + box.cy/2
			if _, seen := rowOf[cy]; !seen {
				ys = append(ys, cy)
			}
			rowOf[cy] = append(rowOf[cy], i)
		}
		if len(ys) != 2 || len(rowOf[ys[0]]) != 4 || len(rowOf[ys[1]]) != 3 {
			t.Fatalf("descriptions=%t: rows = %v, want 4 + 3", descriptions, rowOf)
		}

		// The text is stored at its declared size: every box holds its label
		// at 18pt and its description at 14pt without an autofit shrink.
		minY, maxY := layout.steps[0].y, layout.steps[0].y
		for i, box := range layout.steps {
			w, h := pfTextArea(steps[i], "Arial", box.cx, box.cy)
			if need := pfRequiredTextHeight(steps[i], "Arial", w); need > h {
				t.Errorf("descriptions=%t step %d: text needs %d EMU of %d — it would be shrunk below its role size", descriptions, i, need, h)
			}
			if box.x < bodyPlaceholder.X || box.x+box.cx > bodyPlaceholder.X+bodyPlaceholder.Width {
				t.Errorf("descriptions=%t step %d leaves the placeholder: %+v", descriptions, i, box)
			}
			minY, maxY = min(minY, box.y), max(maxY, box.y+box.cy)
		}
		// Two rows of steps use the placeholder: at least a third of its
		// height with descriptions (a one-row strip used a fifth).
		if used := maxY - minY; descriptions && used*3 < bodyPlaceholder.Height {
			t.Errorf("the flow uses %d of %d EMU of height", used, bodyPlaceholder.Height)
		}

		// Clear return path: the second row runs right to left and its first
		// step sits directly under the first row's last, so the turn is a
		// straight drop.
		last, first := layout.steps[3], layout.steps[4]
		if first.x != last.x || first.y <= last.y+last.cy {
			t.Errorf("descriptions=%t: row 2 starts at %+v, want directly under %+v", descriptions, first, last)
		}
		for i := 5; i < 7; i++ {
			if layout.steps[i].x >= layout.steps[i-1].x {
				t.Errorf("descriptions=%t: row 2 must run right to left, step %d is at x=%d after x=%d", descriptions, i, layout.steps[i].x, layout.steps[i-1].x)
			}
		}
	}
}

func TestProcessFlowStepsPerRow(t *testing.T) {
	tests := []struct {
		n            int
		descriptions bool
		want         int
	}{
		{3, true, 3},  // a short flow is one row
		{4, true, 4},  // four or fewer always are
		{5, true, 5},  // five readable steps still fit
		{7, true, 4},  // seven wrap 4 + 3, balanced
		{10, true, 5}, // ten wrap 5 + 5
	}
	for _, tt := range tests {
		steps := orderFulfilmentSteps(tt.n, tt.descriptions)
		if got := pfStepsPerRow(steps, bodyPlaceholder, "Arial"); got != tt.want {
			t.Errorf("%d steps: %d per row, want %d", tt.n, got, tt.want)
		}
	}
	// Short labels with no descriptions need less width: seven fit one row.
	short := make([]processFlowStep, 7)
	for i := range short {
		short[i] = processFlowStep{id: fmt.Sprintf("s%d", i), label: "Plan", stepType: pfStepType}
	}
	if got := pfStepsPerRow(short, bodyPlaceholder, "Arial"); got != 7 {
		t.Errorf("seven one-word steps: %d per row, want 7", got)
	}
}

// The diagram shares the process-flow pattern's tinted look: neutral steps
// with no outline, decisions outlined in the accent, accent connectors on the
// pattern's line with the pattern's arrowhead.
func TestProcessFlowSharesThePatternStyle(t *testing.T) {
	tint := patterns.ProcessFlowStepTintPct * 1000
	wantFill := fmt.Sprintf(`<a:schemeClr val="dk1"><a:lumMod val="%d"/><a:lumOff val="%d"/></a:schemeClr>`, tint, 100000-tint)
	for _, kind := range []processFlowStepType{pfStepType, pfDecisionType, pfStartType, pfEndType, pfSubprocessType} {
		fill, line := pfColorsForStepType(kind)
		var body, outline bytes.Buffer
		fill.WriteTo(&body)
		line.Fill.WriteTo(&outline)
		if !strings.Contains(body.String(), wantFill) {
			t.Errorf("%s fill = %s, want the pattern's neutral step tint", kind, body.String())
		}
		switch kind {
		case pfDecisionType:
			if !strings.Contains(outline.String(), `<a:schemeClr val="accent1"/>`) || line.Width != int64(patterns.ProcessFlowDecisionLinePt*12700) {
				t.Errorf("decision outline = %s at %d EMU, want the accent at the pattern's %.1fpt", outline.String(), line.Width, patterns.ProcessFlowDecisionLinePt)
			}
		case pfSubprocessType:
			if line.Width <= 0 {
				t.Error("a subprocess needs an outline for its side bars to draw")
			}
		default:
			if !strings.Contains(outline.String(), "noFill") {
				t.Errorf("%s outline = %s, want none", kind, outline.String())
			}
		}
	}

	src := pptx.ShapeOptions{Bounds: pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}, Geometry: pptx.GeomFlowChartProcess}
	tgt := pptx.ShapeOptions{Bounds: pptx.RectEmu{X: 1000000 + pfGap, Y: 0, CX: 1000000, CY: 500000}, Geometry: pptx.GeomFlowChartProcess}
	xml := string(pfGenerateConnector(9, src, tgt, 2, 3, processFlowConnection{style: "solid"}, "horizontal"))
	for _, want := range []string{
		fmt.Sprintf(`w="%d"`, int64(patterns.ProcessFlowConnectorLinePt*12700)),
		`<a:schemeClr val="accent1"/>`,
		fmt.Sprintf(`w="%s" len="%s"`, patterns.ProcessFlowConnectorHead, patterns.ProcessFlowConnectorHead),
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("connector lacks %s: %s", want, xml)
		}
	}
	if pfGap < int64(patterns.ProcessFlowConnectorMinPt*12700) {
		t.Errorf("step gap %d EMU is shorter than the pattern's %.0fpt connector minimum", pfGap, patterns.ProcessFlowConnectorMinPt)
	}
}

// A flow too long for three rows stays a vertical flow, as before.
func TestProcessFlowLongFlowGoesVertical(t *testing.T) {
	steps := make([]processFlowStep, 20)
	for i := range steps {
		steps[i] = processFlowStep{id: fmt.Sprintf("s%d", i), label: "Order Received", description: "Customer places order", stepType: pfStepType}
	}
	if got := computeProcessFlowLayout(steps, nil, bodyPlaceholder, "horizontal", "Arial").direction; got != "vertical" {
		t.Errorf("a 20-step flow went %s, want vertical", got)
	}
}
