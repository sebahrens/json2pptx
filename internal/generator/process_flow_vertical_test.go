package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestProcessFlowVertical_ContentSizedAndBranchesAcrossSpine(t *testing.T) {
	steps := []processFlowStep{
		{id: "start", label: "Start", stepType: pfStartType},
		{id: "review", label: "Approved?", stepType: pfDecisionType},
		{id: "yes", label: "Release", stepType: pfStepType},
		{id: "no", label: "Revise", stepType: pfStepType},
		{id: "end", label: "Complete", stepType: pfEndType},
	}
	connections := []processFlowConnection{
		{from: "start", to: "review"},
		{from: "review", to: "yes", label: "Yes"},
		{from: "review", to: "no", label: "No", style: "dashed"},
		{from: "yes", to: "end"},
		{from: "no", to: "end"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10500000, Height: 5000000}

	got := computeProcessFlowLayout(steps, connections, bounds, "vertical")
	if got.direction != "vertical" || len(got.steps) != len(steps) {
		t.Fatalf("layout = %+v", got)
	}
	maxW := bounds.Width * pfVerticalMaxWidthPct / 100
	for i, step := range got.steps {
		if step.cx > maxW {
			t.Errorf("step %d width = %d, exceeds 40%% cap %d", i, step.cx, maxW)
		}
		if step.x < bounds.X || step.x+step.cx > bounds.X+bounds.Width {
			t.Errorf("step %d escapes horizontal bounds: %+v", i, step)
		}
		if step.y < bounds.Y || step.y+step.cy > bounds.Y+bounds.Height {
			t.Errorf("step %d escapes vertical bounds: %+v", i, step)
		}
	}
	spine := bounds.X + bounds.Width/2
	yesCenter := got.steps[2].x + got.steps[2].cx/2
	noCenter := got.steps[3].x + got.steps[3].cx/2
	if yesCenter >= spine || noCenter <= spine {
		t.Fatalf("decision branches are not split around spine %d: yes=%d no=%d", spine, yesCenter, noCenter)
	}
	decision := got.steps[1]
	if decision.cx > decision.cy*2 {
		t.Errorf("decision rendered as a lozenge: width=%d height=%d", decision.cx, decision.cy)
	}
}

func TestProcessFlowVertical_DenseLayoutKeepsGapsInsideBounds(t *testing.T) {
	steps := make([]processFlowStep, 12)
	for i := range steps {
		steps[i] = processFlowStep{id: string(rune('a' + i)), label: "Step", stepType: pfStepType}
	}
	bounds := types.BoundingBox{Width: 9000000, Height: 4200000}
	got := computeProcessFlowLayout(steps, generateSequentialFlowConnections(steps), bounds, "vertical")
	for i, step := range got.steps {
		if step.y+step.cy > bounds.Height {
			t.Fatalf("step %d bottom=%d exceeds frame=%d", i, step.y+step.cy, bounds.Height)
		}
		if i > 0 && step.y <= got.steps[i-1].y+got.steps[i-1].cy {
			t.Fatalf("steps %d/%d overlap: prev=%+v current=%+v", i-1, i, got.steps[i-1], step)
		}
	}
}

func TestProcessFlowVertical_ConnectionLabelsSitBesideBranchAndBetweenBoxes(t *testing.T) {
	src := pptx.RectEmu{X: 4000000, Y: 500000, CX: 1200000, CY: 700000}
	left := pptx.RectEmu{X: 1500000, Y: 2000000, CX: 1200000, CY: 700000}
	right := pptx.RectEmu{X: 7000000, Y: 2000000, CX: 1200000, CY: 700000}

	leftLabel := pfConnLabelBounds(src, left, "vertical")
	rightLabel := pfConnLabelBounds(src, right, "vertical")
	leftMid := (src.X + src.CX/2 + left.X + left.CX/2) / 2
	rightMid := (src.X + src.CX/2 + right.X + right.CX/2) / 2
	if leftLabel.X+leftLabel.CX >= leftMid {
		t.Errorf("left-branch label is not left of connector: %+v midpoint=%d", leftLabel, leftMid)
	}
	if rightLabel.X <= rightMid {
		t.Errorf("right-branch label is not right of connector: %+v midpoint=%d", rightLabel, rightMid)
	}
	gapTop, gapBottom := src.Y+src.CY, left.Y
	for _, label := range []pptx.RectEmu{leftLabel, rightLabel} {
		if label.Y < gapTop || label.Y+label.CY > gapBottom {
			t.Errorf("label overlaps a step instead of occupying the gap: %+v gap=%d..%d", label, gapTop, gapBottom)
		}
	}
}

func TestProcessFlowVertical_LongContentCapsAtFortyPercent(t *testing.T) {
	bounds := types.BoundingBox{Width: 10000000, Height: 5000000}
	short := pfVerticalStepWidth(processFlowStep{label: "Review"}, pfStepLayout{cx: pfMinStepWidth}, bounds)
	long := pfVerticalStepWidth(processFlowStep{label: "A very long decision label that must not become a slide-wide banner", stepType: pfDecisionType}, pfStepLayout{cx: pfMinStepWidth}, bounds)
	capW := bounds.Width * pfVerticalMaxWidthPct / 100
	if short >= capW {
		t.Errorf("short label width=%d should remain content-sized below cap=%d", short, capW)
	}
	if long != capW {
		t.Errorf("long label width=%d, want cap=%d", long, capW)
	}
}

func TestProcessFlowVertical_LongDecisionStillAvoidsLozenge(t *testing.T) {
	steps := []processFlowStep{{id: "d", label: "A very long decision label that must wrap", stepType: pfDecisionType}}
	bounds := types.BoundingBox{Width: 10000000, Height: 1600000}
	got := computeProcessFlowLayout(steps, nil, bounds, "vertical").steps[0]
	if got.cx > got.cy*2 {
		t.Fatalf("decision width=%d height=%d exceeds 2:1 aspect", got.cx, got.cy)
	}
}
