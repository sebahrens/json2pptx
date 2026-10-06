package patterns

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// cycleNodesCopy is copy of exactly length characters made of short words,
// the way real text wraps.
func cycleNodesCopy(length int) string {
	return strings.TrimSpace(strings.Repeat("word ", length/5) + strings.Repeat("w", length%5))
}

func cycleNodesBudgetValues(n, labelLen, descLen int) *CycleNodesValues {
	v := &CycleNodesValues{}
	for i := 0; i < n; i++ {
		v.Steps = append(v.Steps, CycleNodesStep{Label: cycleNodesCopy(labelLen), Description: cycleNodesCopy(descLen)})
	}
	return v
}

// cycleNodesFullSlideBudget is the published copy budget per step count on a
// full content area: label and description characters every step carries at
// once. Three to six steps hold the schema maxima; seven and eight put four
// rows in one column, which holds a shorter label and a two-line description.
func cycleNodesFullSlideBudget(n int) (label, description int) {
	if n >= 7 {
		return 22, 40
	}
	return cycleNodesLabelMax, cycleNodesDescMax
}

// TestCycleNodesBudgets pins the copy budgets: on a full content area — the
// shortest shipped one (abstract, 687 × 294pt) and the local p-style — every
// step of every count carries a label and a description at its budget
// without a warning, in the default measuring face (wider than most theme
// faces). The cross-template wall with the real theme fonts is cmd/json2pptx
// TestCycleNodesMatrixAcrossTemplates.
func TestCycleNodesBudgets(t *testing.T) {
	p := &cycleNodes{}
	for _, area := range cycleNodesAreas[:2] {
		for n := ringMinItems; n <= ringMaxItems; n++ {
			for _, direction := range []string{"clockwise", "counter_clockwise"} {
				labelLen, descLen := cycleNodesFullSlideBudget(n)
				v := cycleNodesBudgetValues(n, labelLen, descLen)
				ctx := cycleNodesAreaCtx(area.w, area.h)
				if w := p.PostExpandWarnings(ctx, v, &CycleNodesOverrides{Direction: direction}); len(w) != 0 {
					t.Errorf("%s n=%d %s at %d / %d characters: %v", area.name, n, direction, labelLen, descLen, w)
				}
				lay, err := cycleNodesMeasure(ctx, v, &CycleNodesOverrides{Direction: direction})
				if err != nil {
					t.Fatal(err)
				}
				if lay.mode != cycleNodesLabelsOutside || lay.labelSize < cycleNodesFloorPt || lay.bodySize < cycleNodesFloorPt {
					t.Errorf("%s n=%d: mode %s, label %.0fpt, description %.0fpt", area.name, n, lay.mode, lay.labelSize, lay.bodySize)
				}
				for i, r := range lay.rows {
					if r.y1-r.y0 < r.need-1e-6 {
						t.Errorf("%s n=%d row %d is %.1fpt tall, its text needs %.1fpt", area.name, n, i, r.y1-r.y0, r.need)
					}
				}
			}
		}
	}
}

// TestCycleNodesLegendBudgets pins the legend of a compose half (330 × 290pt):
// what one column beside the ring holds per count.
func TestCycleNodesLegendBudgets(t *testing.T) {
	p := &cycleNodes{}
	ctx := cycleNodesAreaCtx(330, 290)
	budgets := map[int]struct{ label, description int }{
		3: {28, 70},
		4: {28, 50},
		5: {22, 50},
		6: {22, 20},
		7: {22, 20},
		8: {22, 0},
	}
	for n := ringMinItems; n <= ringMaxItems; n++ {
		b := budgets[n]
		v := cycleNodesBudgetValues(n, b.label, b.description)
		lay, err := cycleNodesMeasure(ctx, v, &CycleNodesOverrides{})
		if err != nil {
			t.Fatal(err)
		}
		if lay.mode != cycleNodesLabelsLegend {
			t.Fatalf("n=%d: mode %s, want legend", n, lay.mode)
		}
		if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
			t.Errorf("n=%d at %d / %d characters: %v", n, b.label, b.description, w)
		}
		// The ring gives width to the legend but never drops under the
		// smallest square that holds a 12pt numeral in its nodes.
		if lay.side < ringMinSidePt-1e-6 {
			t.Errorf("n=%d: ring side %.0fpt under the %.0fpt minimum", n, lay.side, ringMinSidePt)
		}
	}
	// Past the budget the pattern says which steps do not fit.
	over := cycleNodesBudgetValues(8, 22, 70)
	w := p.PostExpandWarnings(ctx, over, nil)
	if len(w) == 0 || !strings.Contains(w[0], ErrCodeBodyTooLong) || !strings.Contains(w[0], "steps[0].description") {
		t.Errorf("eight 70-character descriptions in a compose half: %v", w)
	}
}

// TestCycleNodesLabelSizeStepsDownBeforeTextShrinks: the label keeps 14pt
// while the rows fit and steps to the 12pt floor when they do not; an authored
// header_size is not stepped.
func TestCycleNodesLabelSizeStepsDownBeforeTextShrinks(t *testing.T) {
	roomy, err := cycleNodesMeasure(cycleNodesAreaCtx(899, 360), cycleNodesTestValues(4), &CycleNodesOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if roomy.labelSize != cycleNodesLabelPt || !roomy.rowsFit {
		t.Errorf("roomy layout: label %.0fpt, fits %v", roomy.labelSize, roomy.rowsFit)
	}
	stepped := false
	for h := 290.0; h >= 150; h -= 10 {
		v := cycleNodesBudgetValues(8, 22, 40)
		lay, err := cycleNodesMeasure(cycleNodesAreaCtx(687, h), v, &CycleNodesOverrides{})
		if err != nil {
			t.Fatal(err)
		}
		if lay.rowsFit && lay.labelSize == cycleNodesFloorPt {
			stepped = true
			authored, _ := cycleNodesMeasure(cycleNodesAreaCtx(687, h), v, &CycleNodesOverrides{TextOverrides: TextOverrides{HeaderSize: 14}})
			if authored.labelSize != 14 {
				t.Errorf("authored header_size stepped to %.0fpt", authored.labelSize)
			}
			break
		}
	}
	if !stepped {
		t.Error("no area height made the label step from 14pt to 12pt with the rows still fitting")
	}
}

// TestCycleNodesNumeralSize: the numeral follows the node's size and never
// drops under the 12pt floor.
func TestCycleNodesNumeralSize(t *testing.T) {
	for _, tc := range []struct{ diaPt, want float64 }{{94, 24}, {60, 24}, {53, 18}, {45, 18}, {40, 14}, {31, 12}, {20, 12}} {
		if got := cycleNodesNumeralPt(tc.diaPt); got != tc.want {
			t.Errorf("numeral in a %.0fpt node = %.0fpt, want %.0fpt", tc.diaPt, got, tc.want)
		}
	}
	for n := ringMinItems; n <= ringMaxItems; n++ {
		for _, area := range cycleNodesAreas {
			lay, err := cycleNodesMeasure(cycleNodesAreaCtx(area.w, area.h), cycleNodesTestValues(n), &CycleNodesOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			if lay.numeralPt < cycleNodesFloorPt || lay.numeralPt > lay.dia*lay.side*cycleNodesNumeralShare+1e-6 && lay.numeralPt > cycleNodesFloorPt {
				t.Errorf("%s n=%d: %.0fpt numeral in a %.0fpt node", area.name, n, lay.numeralPt, lay.dia*lay.side)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ring_nodes_draw.go — the builders cycle-intake reuses
// ---------------------------------------------------------------------------

func TestRingNodeDraw_DiametersAndSpec(t *testing.T) {
	for n, want := range map[int]float64{3: 0.26, 4: 0.26, 5: 0.26, 6: 0.22, 7: 0.18, 8: 0.18} {
		if got := ringNodeDiameter(n); got != want {
			t.Errorf("ringNodeDiameter(%d) = %v, want %v", n, got, want)
		}
		s := newRingNodeSpec(n, want, true)
		if err := s.validate(); err != nil {
			t.Errorf("n=%d: %v", n, err)
		}
		if !ringNear(s.outerRadius(), 0.5) || !ringNear(s.Radius, 0.5-want/2) {
			t.Errorf("n=%d: nodes do not touch the square: radius %v outer %v", n, s.Radius, s.outerRadius())
		}
		if _, err := s.nodes(want, ringNodeClearDeg); err != nil {
			t.Errorf("n=%d: %v", n, err)
		}
	}
	if s := newRingNodeSpec(4, 0.26, false); s.Clockwise {
		t.Error("counter-clockwise spec is clockwise")
	}
}

func TestRingNodeDraw_Layers(t *testing.T) {
	for _, clockwise := range []bool{true, false} {
		s := newRingNodeSpec(5, 0.26, clockwise)
		nodes, err := s.nodes(0.26, ringNodeClearDeg)
		if err != nil {
			t.Fatal(err)
		}
		layers := ringNodeLayers(nodes, func(i int) ringNodePaint {
			return ringNodePaint{Fill: neutralFillJSON(NeutralTint8), Text: ringNodeTextJSON("ctr", "ctr", -1, ringNodePara{Content: fmt.Sprint(i + 1), Size: 18, Bold: true})}
		})
		if len(layers) != 5 {
			t.Fatalf("%d node layers", len(layers))
		}
		for i, l := range layers {
			if l.Name != fmt.Sprintf("node-%d", i+1) || l.Shape.Geometry != "ellipse" || string(l.Shape.Line) != `"none"` {
				t.Errorf("node layer %d: %q %s line %s", i, l.Name, l.Shape.Geometry, l.Shape.Line)
			}
			if l.Frame.X < 0 || l.Frame.Y < 0 || l.Frame.X+l.Frame.W > 1 || l.Frame.Y+l.Frame.H > 1 {
				t.Errorf("node layer %d frame leaves the cell: %+v", i, l.Frame)
			}
			if strings.Contains(string(l.Shape.Text), "inset_") {
				t.Errorf("a numeral keeps the writer's margin: %s", l.Shape.Text)
			}
		}
		links := ringLinkArrowLayers(s, nodes, ringLinkFillJSON())
		if len(links) != 5 {
			t.Fatalf("%d link layers", len(links))
		}
		for i, l := range links {
			if l.Name != fmt.Sprintf("link-%d", i+1) || l.Shape.Geometry != "circularArrow" || l.Shape.FlipH == clockwise || len(l.Shape.Adjustments) != 5 {
				t.Errorf("link layer %d: %q %s flip=%v adj=%v", i, l.Name, l.Shape.Geometry, l.Shape.FlipH, l.Shape.Adjustments)
			}
		}
	}
	// An open arc has no link after its last node.
	open := newRingArcSpec(3, 180, 0, true)
	open.Radius, open.Thickness = 0.37, 0.26
	nodes, err := open.nodes(0.26, ringNodeClearDeg)
	if err != nil {
		t.Fatal(err)
	}
	if links := ringLinkArrowLayers(open, nodes, ringLinkFillJSON()); len(links) != 2 {
		t.Errorf("open arc of 3 nodes: %d links, want 2", len(links))
	}
	if _, ok := ringLinkArrow(open, nodes[2]); ok {
		t.Error("the last node of an open arc has a link")
	}
}

// TestRingNodeDraw_LinkHeadGivesWay: on a dense ring the arrowhead shrinks in
// proportion so the link keeps a shaft; on a sparse ring it keeps its size.
func TestRingNodeDraw_LinkHeadGivesWay(t *testing.T) {
	head := func(n int) (headDeg, half float64) {
		dia := ringNodeDiameter(n)
		s := newRingNodeSpec(n, dia, true)
		nodes, err := s.nodes(dia, ringNodeClearDeg)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := ringLinkArrow(s, nodes[0])
		if !ok {
			t.Fatal("no link")
		}
		travel := ringTravelDeg(nodes[0].LinkFromDeg, nodes[0].LinkToDeg, true)
		headDeg = float64(a.Adjustments["adj2"]) / ooxmlAngleUnits
		if headDeg > travel*ringLinkHeadShare+1e-3 {
			t.Errorf("n=%d: head %.2f° of a %.2f° link", n, headDeg, travel)
		}
		return headDeg, float64(a.Adjustments["adj5"]) / ooxmlFracUnits * a.Frame.W
	}
	_, sparse := head(3)
	_, dense := head(8)
	if math.Abs(sparse-ringLinkHeadHalf) > 1e-4 {
		t.Errorf("sparse head half width %.4f, want %.4f", sparse, ringLinkHeadHalf)
	}
	if dense >= sparse {
		t.Errorf("dense head half width %.4f does not give way (sparse %.4f)", dense, sparse)
	}
}

func TestRingNodeDraw_CentreAndBadge(t *testing.T) {
	s := newRingNodeSpec(4, 0.26, true)
	nodes, _ := s.nodes(0.26, ringNodeClearDeg)
	centre := ringNodeCentreFrame(s, 0.13, 0.03)
	if !ringNear(centre.X+centre.W/2, 0.5) || !ringNear(centre.W, 2*(0.37-0.13-0.03)) || !ringNear(centre.W, centre.H) {
		t.Errorf("centre frame %+v", centre)
	}
	l := ringCentreLabelLayer(centre, ringNodeTextJSON("ctr", "ctr", 2, ringNodePara{Content: "Loop", Size: 14, Bold: true}))
	if l.Name != "centre" || l.Shape.Geometry != "ellipse" || string(l.Shape.Fill) != `"none"` || !strings.Contains(string(l.Shape.Text), `"inset_left":2`) {
		t.Errorf("centre layer: %+v %s", l, l.Shape.Text)
	}
	// The badge of the node at 12 o'clock sits straight below the node's centre.
	b := ringInnerBadgeFrame(s, nodes[0], 0.26, 0.08)
	if !ringNear(b.X+b.W/2, 0.5) || !ringNear(b.Y+b.H/2, 0.26) {
		t.Errorf("badge frame %+v, want centred on (0.5, 0.26)", b)
	}
	if f := ringLayerFrame(ringFrame{X: -1e-12, Y: 0.5, W: 0.5, H: 0.5 + 1e-12}); f.X != 0 || f.Y+f.H > 1 {
		t.Errorf("ringLayerFrame does not pull the frame inside the cell: %+v", f)
	}
}

func TestRingNodeDraw_FitCircleLabel(t *testing.T) {
	ctx := ExpandContext{}
	if size, ok := ringFitCircleLabel(ctx, "Agent loop", []float64{14, 12}, 120, 2, 3); !ok || size != 14 {
		t.Errorf("a short label in a 120pt circle: %.0fpt fits=%v", size, ok)
	}
	if size, ok := ringFitCircleLabel(ctx, "WWWWWWWWWWWWWWWWWWWW", []float64{14, 12}, 60, 2, 3); ok || size != 12 {
		t.Errorf("an unbreakable word in a 60pt circle: %.0fpt fits=%v, want the last size and no fit", size, ok)
	}
	if _, ok := ringFitCircleLabel(ctx, "one two three four five six seven eight", []float64{12}, 70, 2, 2); ok {
		t.Error("eight words fit two lines of a 70pt circle")
	}
	if !ringBreaksWord(ctx, "extraordinarily", 12, 30) || ringBreaksWord(ctx, "a b c", 12, 30) {
		t.Error("ringBreaksWord")
	}
}
