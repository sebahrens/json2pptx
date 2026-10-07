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
	steps := orderFulfilmentSteps(7, true)
	layout := computeProcessFlowLayout(steps, generateSequentialFlowConnections(steps), bodyPlaceholder, "horizontal", "Arial")
	if layout.direction != "horizontal" || layout.band == nil {
		t.Fatalf("a seven-step sequence went %s (band %v), want a horizontal band", layout.direction, layout.band != nil)
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
		t.Fatalf("rows = %v, want 4 + 3", rowOf)
	}

	// The text is stored at its declared size: every arrow holds its
	// numeral, label and description without an autofit shrink.
	minY, maxY := layout.steps[0].y, layout.steps[0].y
	for i, box := range layout.steps {
		if need, have := pfBandTextDeficit(steps, i, layout, "Arial"); need > have {
			t.Errorf("step %d: text needs %d EMU of %d — it would be shrunk below its role size", i, need, have)
		}
		if box.x < bodyPlaceholder.X || box.x+box.cx > bodyPlaceholder.X+bodyPlaceholder.Width {
			t.Errorf("step %d leaves the placeholder: %+v", i, box)
		}
		minY, maxY = min(minY, box.y), max(maxY, box.y+box.cy)
	}
	// Two rows of steps use the placeholder: at least a third of its height
	// (a one-row strip used a fifth).
	if used := maxY - minY; used*3 < bodyPlaceholder.Height {
		t.Errorf("the flow uses %d of %d EMU of height", used, bodyPlaceholder.Height)
	}

	// Clear return path: the second row runs right to left, mirrored, and
	// its first step sits directly under the first row's last.
	last, first := layout.steps[3], layout.steps[4]
	if first.x != last.x || first.y <= last.y+last.cy {
		t.Errorf("row 2 starts at %+v, want directly under %+v", first, last)
	}
	for i := 5; i < 7; i++ {
		if layout.steps[i].x >= layout.steps[i-1].x {
			t.Errorf("row 2 must run right to left, step %d is at x=%d after x=%d", i, layout.steps[i].x, layout.steps[i-1].x)
		}
	}
	for i := range steps {
		if layout.band.flip[i] != (i >= 4) {
			t.Errorf("step %d: mirrored = %t, want %t", i, layout.band.flip[i], i >= 4)
		}
	}

	// Short labels with no descriptions keep one row: seven arrows.
	bare := orderFulfilmentSteps(7, false)
	one := computeProcessFlowLayout(bare, generateSequentialFlowConnections(bare), bodyPlaceholder, "horizontal", "Arial")
	if one.band == nil || one.band.perRow != 7 {
		t.Fatalf("seven steps without descriptions: band %+v, want one row of seven", one.band)
	}
}

// A plain sequence is a band of interlocking arrows: a pentagon, then
// chevrons that reach over the gap, numbered, in the accent's content tint,
// with no connector between them (go-slide-creator-av25u).
func TestProcessFlowSequenceIsABandOfArrows(t *testing.T) {
	steps := orderFulfilmentSteps(4, true)
	steps[2].stepType = pfDecisionType
	steps[2].description = ""
	steps[2].label = "In stock?"
	var panels []nativePanelData
	for _, s := range steps {
		panels = append(panels, nativePanelData{title: s.label, body: s.description, value: fmt.Sprintf("%s:%s", s.stepType, s.id)})
	}
	for _, c := range generateSequentialFlowConnections(steps) {
		panels = append(panels, nativePanelData{title: c.label, value: fmt.Sprintf("conn:%s:%s:%s", c.from, c.to, c.style)})
	}
	xml := generateProcessFlowGroupXML(panels, bodyPlaceholder, 100, processFlowMeta{fontName: "Arial", direction: "horizontal"})
	for _, want := range []string{
		`prst="homePlate"`, `prst="chevron"`, `prst="flowChartDecision"`,
		`<a:t>01</a:t>`, `<a:t>02</a:t>`, `<a:t>03</a:t>`,
		`<a:schemeClr val="accent1"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("band lacks %s", want)
		}
	}
	for _, not := range []string{"<p:cxnSp>", "flowChartProcess", "<a:t>04</a:t>", "<a:t>Yes</a:t>"} {
		if strings.Contains(xml, not) {
			t.Errorf("band must not contain %s", not)
		}
	}

	layout := computeProcessFlowLayout(steps, generateSequentialFlowConnections(steps), bodyPlaceholder, "horizontal", "Arial")
	colW := layout.steps[0].cx
	// The chevron after the pentagon reaches left over the gap by its notch.
	if got, want := layout.steps[1].x, layout.steps[0].x+colW+pfBandGap-layout.band.notch; got != want {
		t.Errorf("second step starts at %d, want %d (tucked round the first step's point)", got, want)
	}

	// Authored connections that branch or carry a label are a flowchart.
	branch := []processFlowConnection{{from: "s0", to: "s1", style: "solid"}, {from: "s1", to: "s3", label: "Skip", style: "solid"}, {from: "s1", to: "s2", style: "solid"}}
	if pfBandEligible(steps, branch, "horizontal") {
		t.Error("a branching flow must stay a flowchart")
	}
	if pfBandEligible(steps, generateSequentialFlowConnections(steps), "vertical") {
		t.Error("a vertical flow must stay a flowchart")
	}
	terminators := orderFulfilmentSteps(3, false)
	terminators[0].stepType = pfStartType
	if pfBandEligible(terminators, generateSequentialFlowConnections(terminators), "horizontal") {
		t.Error("a flow with a terminator must stay a flowchart")
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

// The flowchart look takes the tonal system's roles: steps in the accent's
// content tint with no outline, decisions outlined in the accent, and neutral
// connectors — a connector is never the accent.
func TestProcessFlowFlowchartTones(t *testing.T) {
	wantFill := `<a:schemeClr val="accent1"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr>`
	for _, kind := range []processFlowStepType{pfStepType, pfDecisionType, pfStartType, pfEndType, pfSubprocessType} {
		fill, line := pfColorsForStepType(kind)
		var body, outline bytes.Buffer
		fill.WriteTo(&body)
		line.Fill.WriteTo(&outline)
		if !strings.Contains(body.String(), wantFill) {
			t.Errorf("%s fill = %s, want the accent's content tint", kind, body.String())
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
		`w="19050"`,
		`<a:schemeClr val="dk1"><a:lumMod val="50000"/><a:lumOff val="50000"/></a:schemeClr>`,
		fmt.Sprintf(`w="%s" len="%s"`, patterns.ProcessFlowConnectorHead, patterns.ProcessFlowConnectorHead),
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("connector lacks %s: %s", want, xml)
		}
	}
	if strings.Contains(xml, `val="accent1"`) {
		t.Errorf("a connector must not take the accent: %s", xml)
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
