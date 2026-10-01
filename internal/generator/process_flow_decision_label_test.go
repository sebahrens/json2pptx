package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// orderToCashSteps is the go-slide-creator-acydi review flow: six steps in one
// row with a single "Approved?" decision and a "Yes" edge label.
func orderToCashSteps() ([]processFlowStep, []processFlowConnection) {
	steps := []processFlowStep{
		{id: "a", label: "Receive order", stepType: pfStartType},
		{id: "b", label: "Credit check", stepType: pfStepType},
		{id: "c", label: "Approved?", stepType: pfDecisionType},
		{id: "d", label: "Fulfil and ship", stepType: pfStepType},
		{id: "e", label: "Invoice customer", stepType: pfStepType},
		{id: "f", label: "Collect payment", stepType: pfEndType},
	}
	conns := []processFlowConnection{
		{from: "a", to: "b"}, {from: "b", to: "c"}, {from: "c", to: "d", label: "Yes"},
		{from: "d", to: "e"}, {from: "e", to: "f"},
	}
	return steps, conns
}

// TestProcessFlowDecisionLabelNeverBreaksInsideAWord pins
// go-slide-creator-acydi: on the review's 6-step row, across the body sizes of
// the shipped templates, the decision label is drawn whole on one unwrapped
// line at 14-18pt, so no renderer can split "Approved?" mid-word.
func TestProcessFlowDecisionLabelNeverBreaksInsideAWord(t *testing.T) {
	steps, conns := orderToCashSteps()
	for _, bounds := range []types.BoundingBox{
		{Width: 10972800, Height: 4525963}, // 16:9 one-content body
		{Width: 8229600, Height: 4114800},  // 4:3 body
		{Width: 7315200, Height: 3200400},  // narrow frame
	} {
		layout := computeProcessFlowLayout(steps, conns, bounds, "horizontal", "Arial")
		shape := pfGenerateStepShape(steps[2], layout.steps[2], 3, len(steps), "Arial")
		if shape.Text.Wrap != "none" {
			t.Errorf("%v: decision wrap = %q, want none", bounds, shape.Text.Wrap)
		}
		var texts []string
		for _, p := range shape.Text.Paragraphs {
			for _, r := range p.Runs {
				texts = append(texts, r.Text)
				if r.FontSize < pfDecisionMinLabelSize || r.FontSize > pfLabelFontSize {
					t.Errorf("%v: decision label size %d outside [1400, 1800]", bounds, r.FontSize)
				}
			}
		}
		if len(texts) != 1 || texts[0] != "Approved?" {
			t.Errorf("%v: decision label lines = %q, want the single word whole", bounds, texts)
		}
		if !pfDecisionLabelFits("Approved?", "Arial", layout.steps[2].cx, layout.steps[2].cy) {
			t.Errorf("%v: 'Approved?' does not fit its %dx%d diamond at 14pt or more", bounds, layout.steps[2].cx, layout.steps[2].cy)
		}
	}
}

// TestProcessFlowDecisionLabelBreaksOnlyAtSpaces covers a multi-word
// decision: lines are whole words.
func TestProcessFlowDecisionLabelBreaksOnlyAtSpaces(t *testing.T) {
	label := "Credit limit exceeded for this account?"
	size, lines := pfDecisionLabelLines(label, "Arial", 1600000, 1400000)
	if size < pfDecisionMinLabelSize {
		t.Fatalf("size %d below 14pt", size)
	}
	if got := strings.Join(lines, " "); got != label {
		t.Fatalf("lines %q do not rejoin to the label", lines)
	}
	words := map[string]bool{}
	for _, w := range strings.Fields(label) {
		words[w] = true
	}
	for _, line := range lines {
		for _, w := range strings.Fields(line) {
			if !words[w] {
				t.Errorf("line %q splits a word", line)
			}
		}
	}
}

// TestProcessFlowEdgeLabelSitsAboveTheConnector pins the edge-label half of
// go-slide-creator-acydi: the "Yes" label sits wholly above the connector
// line with clearance, clear of the arrowhead, of the target box and of the
// source diamond's outline, on a knock-out, at the 12pt floor.
func TestProcessFlowEdgeLabelSitsAboveTheConnector(t *testing.T) {
	steps, conns := orderToCashSteps()
	bounds := types.BoundingBox{Width: 10972800, Height: 4525963}
	layout := computeProcessFlowLayout(steps, conns, bounds, "horizontal", "Arial")
	src, tgt := layout.steps[2], layout.steps[3]
	srcRect := pptx.RectEmu{X: src.x, Y: src.y, CX: src.cx, CY: src.cy}
	tgtRect := pptx.RectEmu{X: tgt.x, Y: tgt.y, CX: tgt.cx, CY: tgt.cy}
	w, h := pfConnLabelSize("Yes", "Arial")
	label := pfConnLabelBounds(srcRect, tgtRect, "horizontal", w, h, true)

	lineY := src.y + src.cy/2
	if label.Y+label.CY > lineY-pfConnLabelClearance {
		t.Errorf("label bottom %d is not clear above the connector at %d", label.Y+label.CY, lineY)
	}
	// A medium triangle arrowhead on a 1pt line is ~3pt tall, centred on the line.
	arrow := pptx.RectEmu{X: tgt.x - 3*12700, Y: lineY - 2*12700, CX: 3 * 12700, CY: 4 * 12700}
	if nativeRectsOverlap(label, arrow) {
		t.Errorf("label %+v intersects the arrowhead %+v", label, arrow)
	}
	if pfRectHitsStep(label, tgtRect, pfStepType) || pfRectHitsStep(label, srcRect, pfDecisionType) {
		t.Errorf("label %+v touches a step (src %+v tgt %+v)", label, srcRect, tgtRect)
	}

	shape := pfGenerateStepShape(steps[2], src, 3, len(steps), "Arial")
	xml := string(pfGenerateConnLabel(9, shape, pptx.ShapeOptions{Bounds: tgtRect}, "Yes", "horizontal", "Arial"))
	if !strings.Contains(xml, `<a:schemeClr val="bg1"/>`) {
		t.Error("edge label has no background knock-out fill")
	}
	if !strings.Contains(xml, `sz="1200"`) {
		t.Error("edge label is not at the 12pt floor")
	}
}
