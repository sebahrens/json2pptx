package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestProcessFlowReadableVerticalBoxes(t *testing.T) {
	steps := []processFlowStep{
		{id: "s", label: "Start", stepType: pfStartType},
		{id: "d", label: "Approved?", stepType: pfDecisionType},
		{id: "y", label: "Release", stepType: pfStepType},
		{id: "n", label: "Revise", stepType: pfStepType},
		{id: "e", label: "Complete", stepType: pfEndType},
	}
	bounds := types.BoundingBox{Width: nativePreflightWidthEMU, Height: nativePreflightHeightEMU}
	got := computeProcessFlowLayout(steps, nil, bounds, "vertical", "Arial")
	for i, box := range got.steps {
		w, h := pfTextArea(steps[i].stepType, box.cx, box.cy)
		need := pfRequiredTextHeight(steps[i], "Arial", w)
		if need > h {
			t.Errorf("%s: box=%+v text width=%d need=%d available=%d", steps[i].label, box, w, need, h)
		}
	}
}

func TestProcessFlowLongTextStaysInFrameAndReportsCapacity(t *testing.T) {
	step := processFlowStep{id: "review", label: "Review", description: strings.Repeat("Detail\n", 30), stepType: pfStepType}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 120 * 12700}
	box := computeProcessFlowLayout([]processFlowStep{step}, nil, bounds, "horizontal", "Arial").steps[0]
	if box.y < bounds.Y || box.y+box.cy > bounds.Y+bounds.Height {
		t.Fatalf("text growth escaped frame: box=%+v frame=%+v", box, bounds)
	}
	spec := &types.DiagramSpec{Type: "process_flow", Height: 120, Data: map[string]any{
		"steps": []any{map[string]any{"id": step.id, "label": step.label, "description": step.description}},
	}}
	findings := NativeDiagramPreflight(spec, "Arial", "p")
	if len(findings) == 0 || findings[0].Code != "diagram.text_overlap" {
		t.Fatalf("undersized frame must report text-capacity deficit: %+v", findings)
	}
}

func TestProcessFlowKeepsTextRolesAndAuthoredWords(t *testing.T) {
	for _, count := range []int{1, 7, 12} {
		step := processFlowStep{label: "Partner auswählen・販売協力先を選定", description: "First line\nSecond line", stepType: pfStepType}
		shape := pfGenerateStepShape(step, pfStepLayout{cx: 2000000, cy: 2000000}, 2, count)
		label := shape.Text.Paragraphs[0].Runs[0]
		body := shape.Text.Paragraphs[1].Runs[0]
		if label.Text != step.label || body.Text != step.description {
			t.Fatalf("count %d: authored text changed: %+v", count, shape.Text)
		}
		if label.FontSize != 1800 || !label.Bold || body.FontSize != 1400 || body.Bold {
			t.Fatalf("count %d: unreadable or inconsistent text roles: label=%+v body=%+v", count, label, body)
		}
	}
}

func TestProcessFlowReadableHeightIncludesDescription(t *testing.T) {
	step := processFlowStep{label: "Review", stepType: pfSubprocessType}
	bounds := types.BoundingBox{Width: 10000000, Height: 5000000}
	plain := computeProcessFlowLayout([]processFlowStep{step}, nil, bounds, "horizontal", "Arial").steps[0]
	step.description = "First line\nSecond line\nThird line\nFourth line\nFifth line"
	withBody := computeProcessFlowLayout([]processFlowStep{step}, nil, bounds, "horizontal", "Arial").steps[0]
	if withBody.cy <= plain.cy {
		t.Fatalf("description must reserve space: plain=%+v with body=%+v", plain, withBody)
	}
	w, h := pfTextArea(step.stepType, withBody.cx, withBody.cy)
	if need := pfRequiredTextHeight(step, "Arial", w); need > h {
		t.Fatalf("description clipped: need %d available %d", need, h)
	}
}
