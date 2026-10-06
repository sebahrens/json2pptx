package patterns

import (
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

const ringTol = 1e-6

func ringNear(a, b float64) bool { return math.Abs(a-b) <= ringTol }

func ringMustItems(t *testing.T, s ringSpec) []ringItem {
	t.Helper()
	items, err := ringItems(s)
	if err != nil {
		t.Fatalf("ringItems(%+v): %v", s, err)
	}
	return items
}

func TestRingItems_EqualSegmentsAndGaps(t *testing.T) {
	for n := ringMinItems; n <= ringMaxItems; n++ {
		for _, clockwise := range []bool{true, false} {
			s := newRingSpec(n)
			s.Clockwise = clockwise
			items := ringMustItems(t, s)
			if len(items) != n {
				t.Fatalf("n=%d: %d items", n, len(items))
			}
			wantSeg := (360 - float64(n)*s.GapDeg) / float64(n)
			total := 0.0
			for i, it := range items {
				if it.Index != i {
					t.Errorf("n=%d item %d: index %d", n, i, it.Index)
				}
				for _, d := range []float64{it.StartDeg, it.EndDeg, it.MidDeg} {
					if d < 0 || d >= 360 {
						t.Errorf("n=%d item %d: angle %v not normalised", n, i, d)
					}
				}
				seg := ringTravelDeg(it.StartDeg, it.EndDeg, clockwise)
				if !ringNear(seg, wantSeg) {
					t.Errorf("n=%d cw=%v item %d: sweep %v, want %v", n, clockwise, i, seg, wantSeg)
				}
				if got := ringTravelDeg(it.StartDeg, it.MidDeg, clockwise); !ringNear(got, wantSeg/2) {
					t.Errorf("n=%d item %d: mid is %v into the segment, want %v", n, i, got, wantSeg/2)
				}
				next := items[(i+1)%n]
				if gap := ringTravelDeg(it.EndDeg, next.StartDeg, clockwise); !ringNear(gap, s.GapDeg) {
					t.Errorf("n=%d cw=%v gap after item %d: %v, want %v", n, clockwise, i, gap, s.GapDeg)
				}
				// No segment crosses the seam: its distance from the seam
				// only grows along it.
				if a, b := ringTravelDeg(s.StartDeg, it.StartDeg, clockwise), ringTravelDeg(s.StartDeg, it.EndDeg, clockwise); b <= a {
					t.Errorf("n=%d item %d crosses the seam (%v .. %v from it)", n, i, a, b)
				}
				if it.Side != labelSide(it.MidDeg) {
					t.Errorf("n=%d item %d: side %q", n, i, it.Side)
				}
				total += seg
			}
			if !ringNear(total, 360-float64(n)*s.GapDeg) {
				t.Errorf("n=%d: total sweep %v", n, total)
			}
			// The seam gap is centred on StartDeg.
			if got := ringTravelDeg(s.StartDeg, items[0].StartDeg, clockwise); !ringNear(got, s.GapDeg/2) {
				t.Errorf("n=%d cw=%v: item 0 starts %v past the seam, want %v", n, clockwise, got, s.GapDeg/2)
			}
		}
	}
}

func TestRingItems_DirectionReversesOrder(t *testing.T) {
	cw := newRingSpec(4)
	cw.GapDeg = 0
	ccw := cw
	ccw.Clockwise = false
	a, b := ringMustItems(t, cw), ringMustItems(t, ccw)
	wantCW := []float64{315, 45, 135, 225}
	wantCCW := []float64{225, 135, 45, 315}
	for i := range a {
		if !ringNear(a[i].MidDeg, wantCW[i]) || !ringNear(b[i].MidDeg, wantCCW[i]) {
			t.Errorf("item %d: mids %v / %v, want %v / %v", i, a[i].MidDeg, b[i].MidDeg, wantCW[i], wantCCW[i])
		}
	}
}

func TestRingItems_Rejects(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 2, 9, 20} {
		if _, err := ringItems(newRingSpec(n)); err == nil {
			t.Errorf("full ring of %d items accepted", n)
		}
	}
	bad := map[string]func(*ringSpec){
		"zero radius":        func(s *ringSpec) { s.Radius = 0 },
		"zero thickness":     func(s *ringSpec) { s.Thickness = 0 },
		"band past centre":   func(s *ringSpec) { s.Radius, s.Thickness = 0.1, 0.3 },
		"band leaves square": func(s *ringSpec) { s.Radius = 0.45 },
		"negative gap":       func(s *ringSpec) { s.GapDeg = -1 },
		"gap eats segments":  func(s *ringSpec) { s.GapDeg = 45 },
	}
	for name, mut := range bad {
		s := newRingSpec(8)
		mut(&s)
		if _, err := ringItems(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if name != "gap eats segments" {
			if _, err := s.nodes(0.1, 0); err == nil {
				t.Errorf("%s: nodes accepted", name)
			}
		}
	}
	if _, err := ringItems(newRingArcSpec(9, 0, 180, true)); err == nil {
		t.Error("open arc of 9 items accepted")
	}
	if _, err := ringItems(newRingArcSpec(0, 0, 180, true)); err == nil {
		t.Error("open arc of 0 items accepted")
	}
}

func TestRingItems_OpenArc(t *testing.T) {
	// A ring with the wedge at 12 o'clock left out: four segments from -45°
	// clockwise round to 225°.
	s := newRingArcSpec(4, -45, 225, true)
	if s.full() || !ringNear(s.SweepDeg, 270) {
		t.Fatalf("sweep %v full=%v", s.SweepDeg, s.full())
	}
	items := ringMustItems(t, s)
	wantSeg := (270 - 3*s.GapDeg) / 4
	if !ringNear(items[0].StartDeg, 315) || !ringNear(items[3].EndDeg, 225) {
		t.Errorf("arc ends: %v .. %v, want 315 .. 225", items[0].StartDeg, items[3].EndDeg)
	}
	for i, it := range items {
		if seg := ringTravelDeg(it.StartDeg, it.EndDeg, true); !ringNear(seg, wantSeg) {
			t.Errorf("item %d sweep %v, want %v", i, seg, wantSeg)
		}
		if i > 0 {
			if gap := ringTravelDeg(items[i-1].EndDeg, it.StartDeg, true); !ringNear(gap, s.GapDeg) {
				t.Errorf("gap before item %d: %v", i, gap)
			}
		}
	}

	// The same arc walked the other way.
	back := ringMustItems(t, newRingArcSpec(4, 225, -45, false))
	for i := range back {
		j := len(items) - 1 - i
		if !ringNear(back[i].StartDeg, items[j].EndDeg) || !ringNear(back[i].EndDeg, items[j].StartDeg) {
			t.Errorf("reverse item %d: %v..%v, want %v..%v", i, back[i].StartDeg, back[i].EndDeg, items[j].EndDeg, items[j].StartDeg)
		}
	}

	// One or two items are fine on an open arc; one item takes all of it.
	one := ringMustItems(t, newRingArcSpec(1, 10, 100, true))
	if len(one) != 1 || !ringNear(one[0].StartDeg, 10) || !ringNear(one[0].EndDeg, 100) || !ringNear(one[0].MidDeg, 55) {
		t.Errorf("single item: %+v", one)
	}
	if got := ringMustItems(t, newRingArcSpec(2, 0, 180, false)); len(got) != 2 || !ringNear(got[1].EndDeg, 180) {
		t.Errorf("two items counter-clockwise: %+v", got)
	}

	// from == to is the whole circle.
	if s := newRingArcSpec(4, 30, 30, true); !s.full() || s.sweep() != 360 {
		t.Errorf("from == to: sweep %v", s.sweep())
	}
	var zero ringSpec
	if !zero.full() || zero.sweep() != 360 || zero.dir() != -1 {
		t.Errorf("zero spec: full=%v sweep=%v dir=%v", zero.full(), zero.sweep(), zero.dir())
	}
}

// The OOXML convention (0 = 3 o'clock, clockwise, 60000ths of a degree; the
// band thickness in 100000ths of the short side) is pinned as exact integers.
func TestRingBlockArcAdj_PinsOOXMLConvention(t *testing.T) {
	s := newRingSpec(4)
	s.GapDeg = 0
	items := ringMustItems(t, s)
	want := [][2]int64{{16200000, 0}, {0, 5400000}, {5400000, 10800000}, {10800000, 16200000}}
	for i, it := range items {
		adj := s.segmentAdj(it)
		if adj["adj1"] != want[i][0] || adj["adj2"] != want[i][1] || adj["adj3"] != 20000 || len(adj) != 3 {
			t.Errorf("item %d: %v, want adj1=%d adj2=%d adj3=20000", i, adj, want[i][0], want[i][1])
		}
	}

	// Counter-clockwise: the preset still draws clockwise, so the ends swap.
	s.Clockwise = false
	for i, it := range ringMustItems(t, s) {
		w := want[len(want)-1-i]
		if adj := s.segmentAdj(it); adj["adj1"] != w[0] || adj["adj2"] != w[1] {
			t.Errorf("ccw item %d: %v, want adj1=%d adj2=%d", i, adj, w[0], w[1])
		}
	}

	// With the default 6° gap the seam gap straddles 12 o'clock.
	six := newRingSpec(6)
	adj := six.segmentAdj(ringMustItems(t, six)[0])
	if adj["adj1"] != 273*60000 || adj["adj2"] != 327*60000 {
		t.Errorf("six, item 0: %v, want 273° .. 327°", adj)
	}

	// A ring inset in the square: adj3 is a share of the blockArc's own
	// frame, not of the square.
	inset := newRingSpec(6).withBand(0.8, 0.1)
	if !ringNear(inset.Radius, 0.35) || !ringNear(inset.outerRadius(), 0.4) {
		t.Fatalf("withBand: radius %v outer %v", inset.Radius, inset.outerRadius())
	}
	if got := inset.segmentAdj(ringMustItems(t, inset)[0])["adj3"]; got != 12500 {
		t.Errorf("inset adj3 = %d, want 12500 (0.1 of a 0.8 frame)", got)
	}
	if f := inset.bandFrame(); !ringNear(f.X, 0.1) || !ringNear(f.Y, 0.1) || !ringNear(f.W, 0.8) || !ringNear(f.H, 0.8) {
		t.Errorf("inset band frame %+v", f)
	}
	if f := newRingSpec(6).bandFrame(); !ringNear(f.X, 0) || !ringNear(f.Y, 0) || !ringNear(f.W, 1) || !ringNear(f.H, 1) {
		t.Errorf("default band frame %+v, want the whole square", f)
	}

	// Raw conversion: negative angles normalise, 360 wraps to 0, the
	// thickness is clamped to the preset's range.
	if got := blockArcAdj(-90, 360, 0.9); got["adj1"] != 16200000 || got["adj2"] != 0 || got["adj3"] != 50000 {
		t.Errorf("blockArcAdj(-90, 360, 0.9) = %v", got)
	}
	if got := blockArcAdj(359.9999999999, 0, -1); got["adj1"] != 0 || got["adj3"] != 0 {
		t.Errorf("blockArcAdj rounding / clamp: %v", got)
	}
}

func TestRingPointOnCircle_Cardinals(t *testing.T) {
	cases := []struct{ deg, x, y float64 }{
		{0, 15, 20}, {90, 10, 25}, {180, 5, 20}, {270, 10, 15}, {-90, 10, 15}, {360, 15, 20},
	}
	for _, c := range cases {
		x, y := pointOnCircle(10, 20, 5, c.deg)
		if !ringNear(x, c.x) || !ringNear(y, c.y) {
			t.Errorf("pointOnCircle(%v) = %v, %v; want %v, %v", c.deg, x, y, c.x, c.y)
		}
	}
}

func TestRingFrames_BadgeAndFrameAt(t *testing.T) {
	s := newRingSpec(4)
	s.GapDeg = 0
	items := ringMustItems(t, s)
	// Item 0's middle is at -45°: the badge centre is on the centreline.
	f := s.badgeFrame(items[0], 0.12)
	cx, cy := f.X+f.W/2, f.Y+f.H/2
	wx, wy := 0.5+0.4*math.Sqrt2/2, 0.5-0.4*math.Sqrt2/2
	if !ringNear(cx, wx) || !ringNear(cy, wy) || !ringNear(f.W, 0.12) || !ringNear(f.H, 0.12) {
		t.Errorf("badge frame %+v centre %v,%v; want %v,%v", f, cx, cy, wx, wy)
	}
	if got := s.frameAt(180, 0.25, 0.2, 0.1); !ringNear(got.X, 0.15) || !ringNear(got.Y, 0.45) || !ringNear(got.W, 0.2) || !ringNear(got.H, 0.1) {
		t.Errorf("frameAt = %+v", got)
	}
}

func TestRingLabelSide(t *testing.T) {
	cases := map[float64]string{
		0: "right", 90: "bottom", 180: "left", 270: "top", -90: "top",
		60: "right", 100: "bottom", 120: "left", 75: "bottom", 105: "bottom", 74: "right", 106: "left",
		255: "top", 285: "top", 254: "left", 286: "right", 359: "right", 450: "bottom",
	}
	for deg, want := range cases {
		if got := labelSide(deg); got != want {
			t.Errorf("labelSide(%v) = %q, want %q", deg, got, want)
		}
	}
	// A ring has at most one label above and one below it, whatever N.
	for n := ringMinItems; n <= ringMaxItems; n++ {
		for _, start := range []float64{-90, -90 - 180/float64(n), 0, 90} {
			s := newRingSpec(n)
			s.StartDeg = start
			count := map[string]int{}
			for _, it := range ringMustItems(t, s) {
				count[it.Side]++
			}
			if count["top"] > 1 || count["bottom"] > 1 {
				t.Errorf("n=%d start=%v: %v", n, start, count)
			}
		}
	}
	// An odd ring whose first segment is centred on 12 o'clock has its label
	// above the ring.
	five := newRingSpec(5)
	five.StartDeg = -90 - 36
	if items := ringMustItems(t, five); items[0].Side != "top" {
		t.Errorf("five, item 0 side %q", items[0].Side)
	}
	if got := ringMustItems(t, newRingSpec(5))[2].Side; got != "bottom" {
		t.Errorf("five from the seam: item 2 side %q, want bottom", got)
	}
}

func TestRingLabelSideLR(t *testing.T) {
	cases := []struct {
		deg       float64
		clockwise bool
		want      string
	}{
		{0, true, "right"}, {180, true, "left"}, {80, true, "right"}, {100, true, "left"}, {260, false, "left"}, {280, false, "right"},
		{270, true, "right"}, {90, true, "left"}, {270, false, "left"}, {90, false, "right"},
	}
	for _, c := range cases {
		if got := labelSideLR(c.deg, c.clockwise); got != c.want {
			t.Errorf("labelSideLR(%v, %v) = %q, want %q", c.deg, c.clockwise, got, c.want)
		}
	}
	// Four nodes on the cardinal points: two labels a side.
	nodes, err := newRingSpec(4).nodes(0.1, 0)
	if err != nil {
		t.Fatal(err)
	}
	items := ringNodeItems(nodes)
	if got := []string{items[0].Side, items[1].Side, items[2].Side, items[3].Side}; !reflect.DeepEqual(got, []string{"top", "right", "bottom", "left"}) {
		t.Errorf("node sides %v", got)
	}
	lr := ringSidesLR(items, true)
	if got := []string{lr[0].Side, lr[1].Side, lr[2].Side, lr[3].Side}; !reflect.DeepEqual(got, []string{"right", "right", "left", "left"}) {
		t.Errorf("label-column sides %v", got)
	}
	if items[0].Side != "top" {
		t.Error("ringSidesLR changed its input")
	}
}

func TestRingNodes(t *testing.T) {
	s := newRingSpec(4)
	nodes, err := s.nodes(0.2, 5)
	if err != nil {
		t.Fatal(err)
	}
	half := 2*radToDeg(math.Asin(0.2/(4*0.4))) + 5
	for i, n := range nodes {
		wantDeg := normDeg(270 + 90*float64(i))
		if n.Index != i || !ringNear(n.Deg, wantDeg) {
			t.Errorf("node %d at %v, want %v", i, n.Deg, wantDeg)
		}
		cx, cy := pointOnCircle(0.5, 0.5, 0.4, wantDeg)
		if !ringNear(n.Frame.X+n.Frame.W/2, cx) || !ringNear(n.Frame.Y+n.Frame.H/2, cy) || !ringNear(n.Frame.W, 0.2) || !ringNear(n.Frame.H, 0.2) {
			t.Errorf("node %d frame %+v", i, n.Frame)
		}
		if !n.HasLink || !ringNear(n.LinkFromDeg, normDeg(wantDeg+half)) || !ringNear(n.LinkToDeg, normDeg(wantDeg+90-half)) {
			t.Errorf("node %d link %v..%v (has=%v), want %v..%v", i, n.LinkFromDeg, n.LinkToDeg, n.HasLink, normDeg(wantDeg+half), normDeg(wantDeg+90-half))
		}
		// The link's ends are clear of both circles.
		for _, end := range []struct{ deg, centre float64 }{{n.LinkFromDeg, wantDeg}, {n.LinkToDeg, wantDeg + 90}} {
			px, py := pointOnCircle(0.5, 0.5, 0.4, end.deg)
			qx, qy := pointOnCircle(0.5, 0.5, 0.4, end.centre)
			if d := math.Hypot(px-qx, py-qy); d <= 0.1 {
				t.Errorf("node %d: link end at %v is %v from a node centre (radius 0.1)", i, end.deg, d)
			}
		}
	}

	// Counter-clockwise: the same positions in the other order.
	s.Clockwise = false
	ccw, err := s.nodes(0.2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !ringNear(ccw[1].Deg, 180) || !ringNear(ccw[0].LinkFromDeg, normDeg(270-half)) || !ringNear(ccw[0].LinkToDeg, normDeg(180+half)) {
		t.Errorf("ccw nodes: %+v", ccw[:2])
	}

	// An open arc: nodes on both ends, no link after the last.
	arc, err := newRingArcSpec(3, 200, -20, false).nodes(0.1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !ringNear(arc[0].Deg, 200) || !ringNear(arc[1].Deg, 90) || !ringNear(arc[2].Deg, 340) {
		t.Errorf("arc nodes at %v, %v, %v", arc[0].Deg, arc[1].Deg, arc[2].Deg)
	}
	if !arc[0].HasLink || !arc[1].HasLink || arc[2].HasLink {
		t.Errorf("arc links: %v %v %v", arc[0].HasLink, arc[1].HasLink, arc[2].HasLink)
	}
	one, err := newRingArcSpec(1, 0, 90, true).nodes(0.1, 0)
	if err != nil || len(one) != 1 || !ringNear(one[0].Deg, 45) || one[0].HasLink {
		t.Errorf("single node: %+v, %v", one, err)
	}

	for name, call := range map[string]func() ([]ringNode, error){
		"nodes overlap":      func() ([]ringNode, error) { return newRingSpec(8).nodes(0.5, 0) },
		"clearance too wide": func() ([]ringNode, error) { return newRingSpec(8).nodes(0.1, 20) },
		"zero diameter":      func() ([]ringNode, error) { return newRingSpec(4).nodes(0, 0) },
		"diameter over 4R":   func() ([]ringNode, error) { return newRingSpec(4).nodes(2, 0) },
	} {
		if _, err := call(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRingArrow_Adjustments(t *testing.T) {
	s := newRingSpec(4)
	a := s.arrow(-90, 0, 0.1, 0.1, 20)
	want := map[string]int64{"adj1": 10000, "adj2": 1200000, "adj3": 20400000, "adj4": 16200000, "adj5": 10000}
	if !reflect.DeepEqual(a.Adjustments, want) || a.FlipH {
		t.Errorf("clockwise arrow: %v flip=%v, want %v", a.Adjustments, a.FlipH, want)
	}
	// Centreline 0.4 plus half a head of 0.1: the frame is the whole square.
	if !ringNear(a.Frame.X, 0) || !ringNear(a.Frame.Y, 0) || !ringNear(a.Frame.W, 1) || !ringNear(a.Frame.H, 1) {
		t.Errorf("frame %+v", a.Frame)
	}

	// Counter-clockwise from 12 to 9 o'clock is the mirror image of the
	// clockwise arrow from 12 to 3 o'clock.
	s.Clockwise = false
	m := s.arrow(-90, 180, 0.1, 0.1, 20)
	if !reflect.DeepEqual(m.Adjustments, want) || !m.FlipH {
		t.Errorf("counter-clockwise arrow: %v flip=%v, want %v flipped", m.Adjustments, m.FlipH, want)
	}
	s.Clockwise = true

	// A thinner head than the shaft is raised to the shaft; the frame follows.
	b := s.withBand(0.6, 0.1).arrow(0, 90, 0.2, 0.05, 10)
	if !ringNear(b.Frame.W, 0.7) || !ringNear(b.Frame.X, 0.15) || b.Adjustments["adj5"] != 14286 || b.Adjustments["adj1"] != 28571 {
		t.Errorf("raised head: frame %+v adj %v", b.Frame, b.Adjustments)
	}
	// The shaft end is never 0 (the preset pins adj3 to at least 1).
	if got := s.arrow(270, 20, 0.1, 0.1, 20).Adjustments["adj3"]; got != 1 {
		t.Errorf("adj3 = %d, want 1", got)
	}
	// A head longer than half the arc is cut to half of it.
	short := s.arrow(0, 10, 0.1, 0.1, 20)
	if short.Adjustments["adj2"] != 300000 || short.Adjustments["adj3"] != 300000 {
		t.Errorf("short arc: %v", short.Adjustments)
	}
	// from == to is a full turn.
	if full := s.arrow(30, 30, 0.1, 0.1, 20); full.Adjustments["adj2"] != 1200000 || full.Adjustments["adj3"] != 600000 || full.Adjustments["adj4"] != 1800000 {
		t.Errorf("full turn: %v", full.Adjustments)
	}
	// The head never exceeds the preset's 25000.
	if got := s.withBand(0.1, 0.02).arrow(0, 90, 0.02, 0.3, 10).Adjustments["adj5"]; got != 25000 {
		t.Errorf("adj5 = %d, want 25000", got)
	}
}

func TestRingArrowhead_PointsAlongTravel(t *testing.T) {
	s := newRingSpec(4)
	// A triangle points up at rotation 0. Clockwise travel heads right at
	// 12 o'clock, down at 3, left at 6, up at 9.
	for deg, want := range map[float64]float64{-90: 90, 0: 180, 90: 270, 180: 0} {
		f, rot := s.arrowhead(deg, 0.06, 0.1)
		if !ringNear(rot, want) {
			t.Errorf("clockwise at %v: rotation %v, want %v", deg, rot, want)
		}
		cx, cy := pointOnCircle(0.5, 0.5, s.Radius, deg)
		if !ringNear(f.X+f.W/2, cx) || !ringNear(f.Y+f.H/2, cy) || !ringNear(f.W, 0.06) || !ringNear(f.H, 0.1) {
			t.Errorf("arrowhead frame %+v", f)
		}
	}
	s.Clockwise = false
	for deg, want := range map[float64]float64{-90: 270, 0: 0, 90: 90, 180: 180} {
		if _, rot := s.arrowhead(deg, 0.06, 0.1); !ringNear(rot, want) {
			t.Errorf("counter-clockwise at %v: rotation %v, want %v", deg, rot, want)
		}
	}
}

func TestRingSquareSide(t *testing.T) {
	cases := []struct {
		name              string
		w, h, left, right float64
		want              float64
	}{
		{"abstract body, 150pt label columns", 687, 294, 150, 150, 294},
		{"p-style body", 899, 360, 200, 200, 360},
		{"width-limited", 600, 400, 150, 150, 300},
		{"50% compose segment: the reserves give way", 340, 294, 150, 150, ringMinSidePt},
		{"area under the minimum", 100, 80, 20, 20, 80},
		{"no area", 0, 0, 10, 10, 0},
	}
	for _, c := range cases {
		if got := ringSquareSide(c.w, c.h, c.left, c.right); !ringNear(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}

	x, y, side := ringSquareBox(687, 294, ringReserve{Left: 150, Right: 190})
	if !ringNear(side, 294) || !ringNear(x, 150+(687-340-294)/2.0) || !ringNear(y, 0) {
		t.Errorf("box: x=%v y=%v side=%v", x, y, side)
	}
	x, y, side = ringSquareBox(687, 294, ringReserve{Left: 150, Right: 150, Top: 30, Bottom: 40})
	if !ringNear(side, 224) || !ringNear(y, 30) || !ringNear(x, 150+(387-224)/2.0) {
		t.Errorf("box with top / bottom reserves: x=%v y=%v side=%v", x, y, side)
	}
	// The reserves gave way: the square is centred in the whole area.
	x, y, side = ringSquareBox(340, 294, ringReserve{Left: 200, Right: 100})
	if !ringNear(side, ringMinSidePt) || !ringNear(x, 110) || !ringNear(y, 87) {
		t.Errorf("narrow box: x=%v y=%v side=%v", x, y, side)
	}
	x, y, side = ringSquareBox(400, 150, ringReserve{Top: 40, Bottom: 40})
	if !ringNear(side, 70) || !ringNear(y, 40) || !ringNear(x, 165) {
		t.Errorf("short box: x=%v y=%v side=%v", x, y, side)
	}
	if x, y, side = ringSquareBox(400, 100, ringReserve{Top: 80, Bottom: 80}); side != 0 || !ringNear(x, 200) || !ringNear(y, 50) {
		t.Errorf("reserves taller than the area: x=%v y=%v side=%v", x, y, side)
	}
}

// ringRowsApart reports the rows of one side top to bottom and fails when two
// overlap or sit closer than gap.
func ringRowsApart(t *testing.T, rows []ringRow, side string, gap float64) []ringRow {
	t.Helper()
	var out []ringRow
	for _, r := range rows {
		if r.Side == side {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Y < out[j].Y })
	for i := 1; i < len(out); i++ {
		if free := (out[i].Y - out[i].H/2) - (out[i-1].Y + out[i-1].H/2); free < gap-ringTol {
			t.Errorf("%s rows %d and %d are %v apart, want at least %v", side, out[i-1].Index, out[i].Index, free, gap)
		}
	}
	return out
}

func TestRingLabelRows_EightItems(t *testing.T) {
	const side = 280.0
	s := newRingSpec(8)
	items := ringMustItems(t, s)
	for _, rowPt := range []float64{34, 60, 70} {
		rows, fits := ringLabelRows(items, ringRowsSpec{CentreY: side / 2, RadiusPt: s.Radius * side, RowPt: rowPt, Top: 0, Bottom: side})
		if !fits || len(rows) != 8 {
			t.Fatalf("rowPt=%v: fits=%v rows=%d", rowPt, fits, len(rows))
		}
		for i, r := range rows {
			if r.Index != i || r.Side != items[i].Side || r.H != rowPt {
				t.Errorf("rowPt=%v row %d: %+v", rowPt, i, r)
			}
			if r.Y-r.H/2 < -ringTol || r.Y+r.H/2 > side+ringTol {
				t.Errorf("rowPt=%v row %d leaves the content height: %v ± %v", rowPt, i, r.Y, r.H/2)
			}
		}
		for _, sd := range []string{"left", "right"} {
			got := ringRowsApart(t, rows, sd, 0)
			if len(got) != 4 {
				t.Fatalf("rowPt=%v: %d rows on the %s", rowPt, len(got), sd)
			}
			// A symmetric ring keeps symmetric rows.
			for i := range got {
				if j := len(got) - 1 - i; !ringNear(got[i].Y-side/2, side/2-got[j].Y) {
					t.Errorf("rowPt=%v %s: rows %v and %v are not mirrored about the centre", rowPt, sd, got[i].Y, got[j].Y)
				}
			}
			if rowPt == 34 {
				for _, r := range got {
					if !ringNear(r.Y, r.AnchorY) {
						t.Errorf("34pt row %d moved from its badge: %v vs %v", r.Index, r.Y, r.AnchorY)
					}
				}
			}
			if rowPt == 70 {
				for i, r := range got {
					if want := 35 + 70*float64(i); !ringNear(r.Y, want) {
						t.Errorf("70pt %s row %d at %v, want %v", sd, i, r.Y, want)
					}
				}
			}
		}
	}

	// Four 71pt rows do not fit 280pt: stacked from the top, reported.
	rows, fits := ringLabelRows(items, ringRowsSpec{CentreY: side / 2, RadiusPt: s.Radius * side, RowPt: 71, Top: 0, Bottom: side})
	if fits {
		t.Error("4 × 71pt reported as fitting 280pt")
	}
	if got := ringRowsApart(t, rows, "right", 0); !ringNear(got[0].Y, 35.5) || !ringNear(got[3].Y, 35.5+3*71) {
		t.Errorf("overflowing rows at %v .. %v", got[0].Y, got[3].Y)
	}
}

func TestRingLabelRows_FourItemsSitOnTheirBadges(t *testing.T) {
	const side = 280.0
	s := newRingSpec(4)
	items := ringMustItems(t, s)
	rows, fits := ringLabelRows(items, ringRowsSpec{CentreY: side / 2, RadiusPt: s.Radius * side, RowPt: 34, GapPt: 6, Top: 0, Bottom: side})
	if !fits {
		t.Fatal("does not fit")
	}
	for i, r := range rows {
		f := s.badgeFrame(items[i], 0.1)
		badgeY := (f.Y + f.H/2) * side
		if !ringNear(r.AnchorY, badgeY) || !ringNear(r.Y, badgeY) {
			t.Errorf("row %d: anchor %v y %v, badge at %v", i, r.AnchorY, r.Y, badgeY)
		}
	}
}

func TestRingLabelRows_HeightsGapAndPoles(t *testing.T) {
	// Five nodes: one at 12 o'clock (label above the ring), two a side.
	s := newRingSpec(5)
	nodes, err := s.nodes(0.1, 0)
	if err != nil {
		t.Fatal(err)
	}
	items := ringNodeItems(nodes)
	spec := ringRowsSpec{CentreY: 100, RadiusPt: 80, RowPt: 30, Heights: []float64{0, 100, 70}, GapPt: 10, Top: 0, Bottom: 200}
	rows, fits := ringLabelRows(items, spec)
	if !fits {
		t.Fatal("does not fit")
	}
	if rows[0].Side != "top" || !ringNear(rows[0].Y, 20) || rows[0].H != 30 {
		t.Errorf("12 o'clock row: %+v, want it left on its anchor at 20", rows[0])
	}
	if rows[1].H != 100 || rows[2].H != 70 || rows[3].H != 30 {
		t.Errorf("heights: %v %v %v", rows[1].H, rows[2].H, rows[3].H)
	}
	right := ringRowsApart(t, rows, "right", 10)
	if len(right) != 2 || right[0].Index != 1 || right[1].Index != 2 {
		t.Fatalf("right rows: %+v", right)
	}
	// The anchors are 89.4pt apart and the rows need 50 + 10 + 35: they are
	// pushed exactly that far apart, and up until the lower one ends at 200.
	if !ringNear(right[1].Y-right[0].Y, 95) || !ringNear(right[1].Y+right[1].H/2, 200) {
		t.Errorf("right rows at %v and %v, want centres 95 apart ending at 200", right[0].Y, right[1].Y)
	}
	ringRowsApart(t, rows, "left", 10)

	// With label columns only the 12 o'clock label joins the right column.
	spec.Heights = nil
	lr, fits := ringLabelRows(ringSidesLR(items, true), spec)
	if !fits {
		t.Fatal("label columns do not fit")
	}
	if got := ringRowsApart(t, lr, "right", 10); len(got) != 3 || got[0].Index != 0 {
		t.Errorf("right column: %+v", got)
	}
}

func TestRingLabelRows_Exclusion(t *testing.T) {
	const side = 220.0
	s := newRingSpec(8).withBand(0.8, 0.03)
	nodes, err := s.nodes(0.17, 3)
	if err != nil {
		t.Fatal(err)
	}
	items := ringSidesLR(ringNodeItems(nodes), true)
	ex := ringExclusion{Side: "right", Y0: 90, Y1: 130}
	spec := ringRowsSpec{CentreY: side / 2, RadiusPt: s.Radius * side, RowPt: 40, GapPt: 4, Top: -20, Bottom: side + 20, Exclude: []ringExclusion{ex, {Side: "left", Y0: 5, Y1: 5}}}
	rows, fits := ringLabelRows(items, spec)
	if !fits {
		t.Fatal("does not fit")
	}
	right := ringRowsApart(t, rows, "right", 4)
	ringRowsApart(t, rows, "left", 4)
	if len(right) != 4 {
		t.Fatalf("%d rows on the right", len(right))
	}
	above := 0
	for _, r := range right {
		top, bottom := r.Y-r.H/2, r.Y+r.H/2
		if top < ex.Y1-ringTol && bottom > ex.Y0+ringTol {
			t.Errorf("row %d (%v..%v) enters the excluded band %v..%v", r.Index, top, bottom, ex.Y0, ex.Y1)
		}
		if top < spec.Top-ringTol || bottom > spec.Bottom+ringTol {
			t.Errorf("row %d (%v..%v) leaves %v..%v", r.Index, top, bottom, spec.Top, spec.Bottom)
		}
		if bottom <= ex.Y0+ringTol {
			above++
		}
	}
	// Node 3 sits level with the band; the band above holds only two 40pt
	// rows, so it goes below with node 4.
	if above != 2 {
		t.Errorf("%d rows above the band, want 2", above)
	}
	// The left side has no real exclusion: its rows only spread.
	for _, r := range rows {
		if r.Side == "left" && math.Abs(r.Y-r.AnchorY) > 20 {
			t.Errorf("left row %d moved %v from its anchor", r.Index, r.Y-r.AnchorY)
		}
	}

	// Rows above an over-full band below the exclusion move up across it.
	ys, ok := ringSpreadBands([]float64{150, 160, 170}, []float64{30, 30, 30}, 0, []ringBand{{0, 100}, {140, 200}})
	if !ok || ys[0] > 100 || ys[1] < 140 || ys[2] < 140 {
		t.Errorf("move up: %v ok=%v", ys, ok)
	}
	// Nothing fits anywhere: reported, rows still ordered.
	ys, ok = ringSpreadBands([]float64{10, 20, 150, 160}, []float64{80, 80, 80, 80}, 0, []ringBand{{0, 100}, {140, 200}})
	if ok || ys[0] >= ys[1] || ys[2] >= ys[3] {
		t.Errorf("over-full bands: %v ok=%v", ys, ok)
	}
}

func TestRingFreeBands(t *testing.T) {
	cases := []struct {
		name    string
		exclude []ringExclusion
		want    []ringBand
	}{
		{"none", nil, []ringBand{{0, 100}}},
		{"other side", []ringExclusion{{"left", 10, 20}}, []ringBand{{0, 100}}},
		{"middle", []ringExclusion{{"right", 40, 60}}, []ringBand{{0, 40}, {60, 100}}},
		{"at the top", []ringExclusion{{"right", -5, 20}}, []ringBand{{20, 100}}},
		{"at the bottom", []ringExclusion{{"right", 80, 120}}, []ringBand{{0, 80}}},
		{"two, unsorted, overlapping", []ringExclusion{{"right", 50, 70}, {"right", 20, 30}, {"right", 60, 80}}, []ringBand{{0, 20}, {30, 50}, {80, 100}}},
		{"everything", []ringExclusion{{"right", -10, 110}}, []ringBand{{0, 100}}},
		{"empty band ignored", []ringExclusion{{"right", 50, 50}}, []ringBand{{0, 100}}},
	}
	for _, c := range cases {
		if got := ringFreeBands(0, 100, "right", c.exclude); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRingSpreadRows(t *testing.T) {
	if ys, ok := ringSpreadRows(nil, nil, 0, 0, 10); ys != nil || !ok {
		t.Errorf("no rows: %v %v", ys, ok)
	}
	// Far apart: untouched.
	ys, ok := ringSpreadRows([]float64{20, 80}, []float64{10, 10}, 4, 0, 100)
	if !ok || !ringNear(ys[0], 20) || !ringNear(ys[1], 80) {
		t.Errorf("apart: %v", ys)
	}
	// Colliding pair: split evenly about its mean.
	ys, _ = ringSpreadRows([]float64{50, 52}, []float64{10, 10}, 4, 0, 100)
	if !ringNear(ys[0], 44) || !ringNear(ys[1], 58) {
		t.Errorf("pair: %v, want 44 and 58", ys)
	}
	// A cluster near the bottom edge is pushed up into the band.
	ys, ok = ringSpreadRows([]float64{90, 95, 99}, []float64{10, 10, 10}, 0, 0, 100)
	if !ok || !ringNear(ys[0], 75) || !ringNear(ys[1], 85) || !ringNear(ys[2], 95) {
		t.Errorf("bottom cluster: %v ok=%v", ys, ok)
	}
	// And one near the top edge is pushed down.
	ys, _ = ringSpreadRows([]float64{1, 2}, []float64{10, 10}, 0, 0, 100)
	if !ringNear(ys[0], 5) || !ringNear(ys[1], 15) {
		t.Errorf("top cluster: %v", ys)
	}
	// A third row far away is left alone by a collision elsewhere.
	ys, _ = ringSpreadRows([]float64{10, 12, 80}, []float64{10, 10, 10}, 0, 0, 100)
	if !ringNear(ys[2], 80) || !ringNear(ys[1]-ys[0], 10) {
		t.Errorf("independent rows: %v", ys)
	}
	if got := ringIsotonic([]float64{3, 1, 2, 5, 4}); !reflect.DeepEqual(got, []float64{2, 2, 2, 4.5, 4.5}) {
		t.Errorf("isotonic: %v", got)
	}
}

func ringLabelCell(text string) *jsonschema.GridCellInput {
	data, _ := json.Marshal(map[string]any{"content": text, "size": 12})
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"none"`), Text: data}}
}

func TestRingLattice_ResolvesToThePlacedRectangles(t *testing.T) {
	bounds := shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
	w := float64(bounds.CX) / 12700
	const h = 240.0
	sqX := (w - h) / 2
	ring := &jsonschema.GridCellInput{Fit: "contain", ColSpan: 9, Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"accent1"`), Line: noLine}}
	places := []ringPlacement{
		{X0: sqX + h + 10, X1: w, Y0: 150, Y1: 190, Cell: ringLabelCell("right 2")},
		{X0: sqX, X1: sqX + h, Y0: 0, Y1: h, Cell: ring},
		{X0: 0, X1: sqX - 10, Y0: 20, Y1: 60, Cell: ringLabelCell("left 1")},
		{X0: sqX + h + 10, X1: w, Y0: 40, Y1: 80, Cell: ringLabelCell("right 1")},
		{X0: 0, X1: sqX - 10, Y0: 100, Y1: 140, Cell: ringLabelCell("left 2")},
	}
	grid, err := ringLattice(places, w, h)
	if err != nil {
		t.Fatal(err)
	}
	if grid.ColGap != ringColGapPt || grid.RowGap != ringColGapPt {
		t.Errorf("gaps %v / %v", grid.ColGap, grid.RowGap)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatal(err)
	}
	// Label column, gap, ring, gap, label column.
	if len(cols) != 5 {
		t.Fatalf("%d columns: %v", len(cols), cols)
	}
	sum := 0.0
	for _, c := range cols {
		sum += c
	}
	if math.Abs(sum-100) > 0.01 {
		t.Errorf("columns sum to %v", sum)
	}
	// Rows cut at 0, 20, 40, 60, 80, 100, 140, 150, 190, 240.
	wantRows := []float64{20, 20, 20, 20, 20, 40, 10, 40, 50}
	if len(grid.Rows) != len(wantRows) {
		t.Fatalf("%d rows, want %d", len(grid.Rows), len(wantRows))
	}
	for r, row := range grid.Rows {
		if !ringNear(row.MinHeight, wantRows[r]) || row.MinHeight != row.MaxHeight {
			t.Errorf("row %d: min %v max %v, want %v", r, row.MinHeight, row.MaxHeight, wantRows[r])
		}
		if len(row.Cells) == 0 {
			t.Errorf("row %d has no cells", r)
		}
	}
	// The ring spans every row and one column; its stale col_span is reset.
	if ring.RowSpan != len(wantRows) || ring.ColSpan != 0 {
		t.Errorf("ring spans: rows %d cols %d", ring.RowSpan, ring.ColSpan)
	}
	// Row 0: the two free columns left of the ring, then the ring.
	if got := grid.Rows[0].Cells; len(got) != 3 || got[0].Shape != nil || got[1].Shape != nil || got[2] != ring {
		t.Errorf("row 0 cells: %d", len(got))
	}
	// Row 2 (y 40): column 0 is taken by "left 1" from the row above and the
	// ring column by the ring, which the grid's cursor skips by itself; one
	// empty cell per FREE column (the two gap columns) comes before the label.
	if got := grid.Rows[2].Cells; len(got) != 3 || got[0].Shape != nil || got[1].Shape != nil || got[2] != places[3].Cell {
		t.Errorf("row 2 cells: %d", len(got))
	}
	// Rows no cell starts in carry one empty cell.
	if got := grid.Rows[3].Cells; len(got) != 1 || got[0].Shape != nil {
		t.Errorf("row 3 cells: %d", len(got))
	}

	grid.VerticalAlign = "top"
	res := resolvePatternGrid(t, grid)
	if len(res.Cells) != len(places) {
		t.Fatalf("%d resolved cells, want %d", len(res.Cells), len(places))
	}
	assertNoOverlap(t, res)
	const tolEMU = 12700 // 1pt: the lattice's merge distance
	for i, p := range places {
		found := false
		for _, c := range res.Cells {
			b := c.CellBounds
			if ringAbs64(b.X-bounds.X-shapegrid.PtToEMU(p.X0)) <= tolEMU && ringAbs64(b.Y-bounds.Y-shapegrid.PtToEMU(p.Y0)) <= tolEMU &&
				ringAbs64(b.CX-shapegrid.PtToEMU(p.X1-p.X0)) <= tolEMU && ringAbs64(b.CY-shapegrid.PtToEMU(p.Y1-p.Y0)) <= tolEMU {
				found = true
			}
		}
		if !found {
			t.Errorf("placement %d (%v-%v × %v-%v) has no resolved cell at its rectangle", i, p.X0, p.X1, p.Y0, p.Y1)
		}
	}
	// The ring cell is square, so the contained ellipse is a circle.
	for _, c := range res.Cells {
		if c.ShapeSpec.Geometry == "ellipse" && ringAbs64(c.Bounds.CX-c.Bounds.CY) > 2 {
			t.Errorf("ring is %d × %d EMU", c.Bounds.CX, c.Bounds.CY)
		}
	}
}

func ringAbs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestRingLattice_MergesCloseEdges(t *testing.T) {
	a, b := ringLabelCell("a"), ringLabelCell("b")
	grid, err := ringLattice([]ringPlacement{
		{X0: 0.3, X1: 100, Y0: 0, Y1: 50, Cell: a},
		{X0: 100.4, X1: 199.6, Y0: 0.2, Y1: 49.7, Cell: b},
	}, 200, 50)
	if err != nil {
		t.Fatal(err)
	}
	if string(grid.Columns) != "[50,50]" {
		t.Errorf("columns %s, want [50,50]", grid.Columns)
	}
	if len(grid.Rows) != 1 || grid.Rows[0].MaxHeight != 50 || len(grid.Rows[0].Cells) != 2 {
		t.Errorf("rows: %+v", grid.Rows)
	}
	if a.ColSpan != 0 || a.RowSpan != 0 || b.ColSpan != 0 {
		t.Errorf("spans: %d %d %d", a.ColSpan, a.RowSpan, b.ColSpan)
	}

	// A cell over several tracks gets both spans.
	wide := ringLabelCell("wide")
	grid, err = ringLattice([]ringPlacement{
		{X0: 0, X1: 200, Y0: 0, Y1: 20, Cell: wide},
		{X0: 0, X1: 50, Y0: 20, Y1: 40, Cell: ringLabelCell("c")},
		{X0: 150, X1: 200, Y0: 30, Y1: 60, Cell: ringLabelCell("d")},
	}, 200, 60)
	if err != nil {
		t.Fatal(err)
	}
	if wide.ColSpan != 3 || wide.RowSpan != 0 || len(grid.Rows) != 4 {
		t.Errorf("wide: col_span %d row_span %d, %d rows", wide.ColSpan, wide.RowSpan, len(grid.Rows))
	}
}

func TestRingLattice_Errors(t *testing.T) {
	cell := func() *jsonschema.GridCellInput { return ringLabelCell("x") }
	var many []ringPlacement
	for i := 0; i < 13; i++ {
		x := float64(i) * 20
		many = append(many, ringPlacement{X0: x, X1: x + 10, Y0: 0, Y1: 10, Cell: cell()})
	}
	cases := []struct {
		name   string
		places []ringPlacement
		w, h   float64
		want   string
	}{
		{"no area", nil, 0, 10, "not an area"},
		{"nil cell", []ringPlacement{{X0: 0, X1: 10, Y0: 0, Y1: 10}}, 100, 100, "no cell"},
		{"outside", []ringPlacement{{X0: 50, X1: 120, Y0: 0, Y1: 10, Cell: cell()}}, 100, 100, "leaves"},
		{"overlap", []ringPlacement{{X0: 0, X1: 60, Y0: 0, Y1: 50, Cell: cell()}, {X0: 40, X1: 100, Y0: 20, Y1: 80, Cell: cell()}}, 100, 100, "overlaps"},
		{"collapsed", []ringPlacement{{X0: 30, X1: 30.5, Y0: 0, Y1: 50, Cell: cell()}}, 100, 100, "collapsed"},
		{"inverted", []ringPlacement{{X0: 0, X1: 50, Y0: 60, Y1: 20, Cell: cell()}}, 100, 100, "collapsed"},
		{"too many columns", many, 260, 10, "columns"},
	}
	for _, c := range cases {
		_, err := ringLattice(c.places, c.w, c.h)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one mentioning %q", c.name, err, c.want)
		}
	}
	// No placements at all is one empty track.
	grid, err := ringLattice(nil, 100, 0.5)
	if err != nil || string(grid.Columns) != "[100]" || len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 1 {
		t.Errorf("empty lattice: %+v, %v", grid, err)
	}
}

func TestRingAngleHelpers(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, 360: 0, -90: 270, 450: 90, 720.5: 0.5, 359.99999999999: 0} {
		if got := normDeg(in); got != want {
			t.Errorf("normDeg(%v) = %v, want %v", in, got, want)
		}
	}
	if got := ringTravelDeg(350, 10, true); got != 20 {
		t.Errorf("clockwise travel %v", got)
	}
	if got := ringTravelDeg(350, 10, false); got != 340 {
		t.Errorf("counter-clockwise travel %v", got)
	}
	if got := ooxmlAngle(-90); got != 16200000 {
		t.Errorf("ooxmlAngle(-90) = %d", got)
	}
	for _, deg := range []float64{359.9999999996, 359.999999999, 360, 720} {
		if got := ooxmlAngle(deg); got != 0 {
			t.Errorf("ooxmlAngle(%v) = %d, want 0", deg, got)
		}
	}
	if !ringNear(degToRad(180), math.Pi) || !ringNear(radToDeg(math.Pi/2), 90) {
		t.Error("degree / radian conversion")
	}
}

// Everything a pattern derives from the helper is identical across calls: it
// feeds the golden size metrics.
func TestRingGeometry_Deterministic(t *testing.T) {
	build := func() []byte {
		type layer struct {
			Frame ringFrame
			Adj   map[string]int64
			Rot   float64
			Flip  bool
		}
		var out struct {
			Layers []layer
			Rows   []ringRow
			Fits   bool
			Grid   *jsonschema.ShapeGridInput
		}
		const w, h = 687.0, 294.0
		for n := ringMinItems; n <= ringMaxItems; n++ {
			s := newRingSpec(n)
			items, _ := s.items()
			for _, it := range items {
				out.Layers = append(out.Layers, layer{Frame: s.bandFrame(), Adj: s.segmentAdj(it)}, layer{Frame: s.badgeFrame(it, 0.12)})
				f, rot := s.arrowhead(it.EndDeg, 0.05, 0.08)
				a := s.arrow(it.StartDeg, it.EndDeg, 0.1, 0.08, 12)
				out.Layers = append(out.Layers, layer{Frame: f, Rot: rot}, layer{Frame: a.Frame, Adj: a.Adjustments, Flip: a.FlipH})
			}
			x, y, side := ringSquareBox(w, h, ringReserve{Left: 150, Right: 150})
			rows, fits := ringLabelRows(items, ringRowsSpec{CentreY: y + side/2, RadiusPt: s.Radius * side, RowPt: 44, GapPt: 4, Top: 0, Bottom: h})
			out.Rows, out.Fits = append(out.Rows, rows...), out.Fits || fits
			places := []ringPlacement{{X0: x, X1: x + side, Y0: y, Y1: y + side, Cell: &jsonschema.GridCellInput{Fit: "contain"}}}
			for _, r := range rows {
				p := ringPlacement{X0: 0, X1: x - 8, Y0: r.Y - r.H/2, Y1: r.Y + r.H/2, Cell: ringLabelCell("label")}
				if r.Side == ringSideRight {
					p.X0, p.X1 = x+side+8, w
				}
				if r.Side == ringSideLeft || r.Side == ringSideRight {
					places = append(places, p)
				}
			}
			grid, err := ringLattice(places, w, h)
			if err != nil {
				t.Fatalf("n=%d: %v", n, err)
			}
			out.Grid = grid
		}
		data, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if a, b := build(), build(); string(a) != string(b) {
		t.Error("ring geometry differs between two calls")
	}
}
