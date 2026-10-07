package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-40xtt: a horizontal flow with a branching decision set all
// its steps in one row and drew the "No" connector straight through the step
// between the decision and its target.

func claimFlow() ([]processFlowStep, []processFlowConnection) {
	steps := []processFlowStep{
		{id: "a", label: "Submit claim", stepType: pfStartType},
		{id: "b", label: "Complete?", stepType: pfDecisionType},
		{id: "c", label: "Assess claim", stepType: pfStepType},
		{id: "d", label: "Request documents", stepType: pfStepType},
		{id: "e", label: "Pay out", stepType: pfEndType},
	}
	conns := []processFlowConnection{
		{from: "a", to: "b", style: "solid"},
		{from: "b", to: "c", label: "Yes", style: "solid"},
		{from: "b", to: "d", label: "No", style: "solid"},
		{from: "d", to: "b", style: "solid"},
		{from: "c", to: "e", style: "solid"},
	}
	return steps, conns
}

func twoDecisionFlow() ([]processFlowStep, []processFlowConnection) {
	steps := []processFlowStep{
		{id: "s1", label: "Intake", stepType: pfStepType},
		{id: "s2", label: "Eligible?", stepType: pfDecisionType},
		{id: "s3", label: "Score the case", description: "Risk model", stepType: pfStepType},
		{id: "s4", label: "Reject", stepType: pfStepType},
		{id: "s5", label: "Above limit?", stepType: pfDecisionType},
		{id: "s6", label: "Committee review", stepType: pfStepType},
		{id: "s7", label: "Approve", stepType: pfStepType},
	}
	conns := []processFlowConnection{
		{from: "s1", to: "s2", style: "solid"},
		{from: "s2", to: "s3", label: "Yes", style: "solid"},
		{from: "s2", to: "s4", label: "No", style: "solid"},
		{from: "s3", to: "s5", style: "solid"},
		{from: "s5", to: "s6", label: "Yes", style: "solid"},
		{from: "s5", to: "s7", label: "No", style: "solid"},
		{from: "s6", to: "s7", style: "solid"},
	}
	return steps, conns
}

// segmentCrossesRect reports whether the axis-aligned segment passes through
// the interior of rect (touching its edge is how a connector meets a step).
func segmentCrossesRect(seg [4]int64, r pptx.RectEmu) bool {
	x1, y1, x2, y2 := min(seg[0], seg[2]), min(seg[1], seg[3]), max(seg[0], seg[2]), max(seg[1], seg[3])
	return x1 < r.X+r.CX && x2 > r.X && y1 < r.Y+r.CY && y2 > r.Y
}

// No connector of a horizontal flowchart crosses a step: neighbours in a row
// are joined across the gap between them, every other connection runs in a
// lane outside the rows and meets its steps on their top or bottom edge.
func TestProcessFlowHorizontalConnectorsNeverCrossAStep(t *testing.T) {
	generated := orderFulfilmentSteps(5, false)
	generated[0].stepType, generated[1].stepType, generated[4].stepType = pfStartType, pfDecisionType, pfEndType
	cs, cc := claimFlow()
	ts, tc := twoDecisionFlow()
	cases := map[string]struct {
		steps []processFlowStep
		conns []processFlowConnection
		rows  int
	}{
		"authored branch and loop":         {cs, cc, 1},
		"generated Yes / No with a skip":   {generated, generateSequentialFlowConnections(generated), 1},
		"two decisions wrapping to 2 rows": {ts, tc, 2},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			layout := computeProcessFlowLayout(tt.steps, tt.conns, bodyPlaceholder, "horizontal", "Arial")
			if layout.direction != "horizontal" || layout.band != nil {
				t.Fatalf("layout went %s (band %t), want a horizontal flowchart", layout.direction, layout.band != nil)
			}
			index := map[string]int{}
			rects := make([]pptx.RectEmu, len(tt.steps))
			rowYs := map[int64]bool{}
			for i, s := range tt.steps {
				index[s.id] = i
				b := layout.steps[i]
				rects[i] = pptx.RectEmu{X: b.x, Y: b.y, CX: b.cx, CY: b.cy}
				rowYs[b.y+b.cy/2] = true
				if b.x < bodyPlaceholder.X || b.x+b.cx > bodyPlaceholder.X+bodyPlaceholder.Width ||
					b.y < bodyPlaceholder.Y || b.y+b.cy > bodyPlaceholder.Y+bodyPlaceholder.Height {
					t.Errorf("step %d leaves the placeholder: %+v", i, b)
				}
			}
			if len(rowYs) != tt.rows {
				t.Errorf("steps sit on %d rows, want %d", len(rowYs), tt.rows)
			}
			detoured := 0
			for ci, c := range tt.conns {
				from, to := index[c.from], index[c.to]
				d, ok := pfDetourFor(layout.detours, ci)
				if !ok {
					// A straight connector: only between a step and its
					// right-hand neighbour on the same row.
					if to != from+1 || rects[from].Y+rects[from].CY/2 != rects[to].Y+rects[to].CY/2 {
						t.Errorf("connection %s -> %s is drawn straight but the steps are not neighbours in a row", c.from, c.to)
					}
					continue
				}
				detoured++
				for _, seg := range pfDetourSegments(d) {
					for i, r := range rects {
						if segmentCrossesRect(seg, r) {
							t.Errorf("connection %s -> %s crosses step %q: segment %v, step %+v", c.from, c.to, tt.steps[i].label, seg, r)
						}
					}
					for _, v := range []int64{seg[1], seg[3]} {
						if v < bodyPlaceholder.Y || v > bodyPlaceholder.Y+bodyPlaceholder.Height {
							t.Errorf("connection %s -> %s leaves the placeholder: %v", c.from, c.to, seg)
						}
					}
				}
				// It starts on its source's edge and ends on its target's.
				src, tgt := rects[from], rects[to]
				if d.srcX < src.X || d.srcX > src.X+src.CX || (d.srcY != src.Y && d.srcY != src.Y+src.CY) {
					t.Errorf("connection %s -> %s does not leave its source's top or bottom edge: %+v", c.from, c.to, d)
				}
				if d.tgtX < tgt.X || d.tgtX > tgt.X+tgt.CX || (d.tgtY != tgt.Y && d.tgtY != tgt.Y+tgt.CY) {
					t.Errorf("connection %s -> %s does not enter its target's top or bottom edge: %+v", c.from, c.to, d)
				}
				if c.label != "" {
					w, h := pfConnLabelSize(c.label, "Arial")
					label := pfDetourLabelBounds(d, w, h)
					for i, r := range rects {
						if pfRectHitsStep(label, r, tt.steps[i].stepType) {
							t.Errorf("label %q of %s -> %s sits on step %q", c.label, c.from, c.to, tt.steps[i].label)
						}
					}
				}
			}
			if detoured == 0 {
				t.Error("no connection took a detour")
			}
			// Two detours never share a stretch of one lane.
			for i, a := range layout.detours {
				for _, b := range layout.detours[i+1:] {
					if a.laneY == b.laneY && min(a.srcX, a.tgtX) <= max(b.srcX, b.tgtX) && min(b.srcX, b.tgtX) <= max(a.srcX, a.tgtX) {
						t.Errorf("two detours overlap on one lane: %+v and %+v", a, b)
					}
				}
			}
		})
	}
}

// The detour is written: three line segments per detoured connection, the
// last with the arrowhead, and its label beside the first.
func TestProcessFlowDetourIsDrawn(t *testing.T) {
	steps, conns := claimFlow()
	var panels []nativePanelData
	for _, s := range steps {
		panels = append(panels, nativePanelData{title: s.label, value: fmt.Sprintf("%s:%s", s.stepType, s.id)})
	}
	for _, c := range conns {
		panels = append(panels, nativePanelData{title: c.label, value: fmt.Sprintf("conn:%s:%s:%s", c.from, c.to, c.style)})
	}
	xml := generateProcessFlowGroupXML(panels, bodyPlaceholder, 100, processFlowMeta{fontName: "Arial", direction: "horizontal"})
	// 2 straight connectors (a-b, b-c) + 3 detours of 3 segments.
	if got := strings.Count(xml, "<p:cxnSp>"); got != 2+3*3 {
		t.Errorf("connectors = %d, want 11", got)
	}
	// One arrowhead per connection.
	if got := strings.Count(xml, `<a:tailEnd type="triangle"`); got != len(conns) {
		t.Errorf("arrowheads = %d, want %d", got, len(conns))
	}
	for _, label := range []string{"<a:t>Yes</a:t>", "<a:t>No</a:t>"} {
		if strings.Count(xml, label) != 1 {
			t.Errorf("label %s is not drawn exactly once", label)
		}
	}
	ids := map[string]bool{}
	for _, m := range renderedShapeIDRE.FindAllStringSubmatch(xml, -1) {
		if ids[m[1]] {
			t.Errorf("shape id %s is used twice", m[1])
		}
		ids[m[1]] = true
	}
	if uint32(len(ids)) > pfEstimateShapeCount(panels) {
		t.Errorf("%d shape ids exceed the estimate %d", len(ids), pfEstimateShapeCount(panels))
	}
}

// A branching flow that cannot be set in rows inside its region is the
// vertical flow, never a row with connectors through its steps.
func TestProcessFlowDetouredFlowFallsBackToVertical(t *testing.T) {
	steps, conns := claimFlow()
	narrow := types.BoundingBox{X: 0, Y: 0, Width: 2400000, Height: 4114800}
	layout := computeProcessFlowLayout(steps, conns, narrow, "horizontal", "Arial")
	if layout.direction != "vertical" && len(layout.detours) == 0 {
		t.Fatalf("a branching flow in a narrow region went %s with no detours", layout.direction)
	}
}
