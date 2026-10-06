package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// The family's label rule (go-slide-creator-70q6f): every label block stands
// the same horizontal gap from its own circle — its node, its satellite, or
// the ring's outer edge within the row's height — whatever the item count and
// the content area. The checks read the RESOLVED grid, so they hold for what
// is drawn, not for what the layout meant to place.
// ---------------------------------------------------------------------------

// ringGapBodies are the content areas the rule is checked in: the smallest
// shipped body, a mid-sized one and the local p-style.
var ringGapBodies = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"midnight-blue", 828, 349},
	{"p-style", 899, 360},
}

const (
	ringGapTolPt = 1.0
	// ringGapClearPt is the least distance, in any direction, between a label
	// block and the ring it follows: the gap is horizontal, so beside a steep
	// part of the circle the block's corner is nearer than the gap.
	ringGapClearPt = 4.0
)

// ringTestRect is a rectangle in points.
type ringTestRect struct{ x0, x1, y0, y1 float64 }

func ringTestRectOf(b pptx.RectEmu) ringTestRect {
	return ringTestRect{float64(b.X) / 12700, float64(b.X+b.CX) / 12700, float64(b.Y) / 12700, float64(b.Y+b.CY) / 12700}
}

func (r ringTestRect) union(o ringTestRect) ringTestRect {
	return ringTestRect{math.Min(r.x0, o.x0), math.Max(r.x1, o.x1), math.Min(r.y0, o.y0), math.Max(r.y1, o.y1)}
}

func (r ringTestRect) contains(x, y float64) bool {
	return x > r.x0 && x < r.x1 && y > r.y0 && y < r.y1
}

// ringTestCircle is a circle in points.
type ringTestCircle struct{ cx, cy, r float64 }

// ringTestCircleOf is the circle inscribed in a resolved square.
func ringTestCircleOf(b pptx.RectEmu) ringTestCircle {
	r := ringTestRectOf(b)
	return ringTestCircle{(r.x0 + r.x1) / 2, (r.y0 + r.y1) / 2, (r.x1 - r.x0) / 2}
}

// clear is the distance between the circle's edge and the rectangle; negative
// when they overlap.
func (c ringTestCircle) clear(r ringTestRect) float64 {
	dx := math.Max(math.Max(r.x0-c.cx, c.cx-r.x1), 0)
	dy := math.Max(math.Max(r.y0-c.cy, c.cy-r.y1), 0)
	return math.Hypot(dx, dy) - c.r
}

// hGap is the horizontal gap between the circle and a label block beside it:
// from the furthest the circle reaches within the block's height to the
// block's near edge.
func (c ringTestCircle) hGap(r ringTestRect) float64 {
	if (r.x0+r.x1)/2 >= c.cx {
		return r.x0 - ringBandEdgeX(c.cx, c.cy, c.r, r.y0, r.y1, ringSideRight)
	}
	return ringBandEdgeX(c.cx, c.cy, c.r, r.y0, r.y1, ringSideLeft) - r.x1
}

// ringGapText is a resolved lattice text cell: its rectangle and the content
// of its first paragraph.
type ringGapText struct {
	rect  ringTestRect
	first string
	paras int
}

func ringGapTexts(t *testing.T, res *shapegrid.ResolveResult) []ringGapText {
	t.Helper()
	var out []ringGapText
	for _, c := range res.Cells {
		if c.Layer || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		var obj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(c.ShapeSpec.Text, &obj); err != nil || len(obj.Paragraphs) == 0 {
			t.Fatalf("text cell %s: %v", c.ShapeSpec.Text, err)
		}
		out = append(out, ringGapText{rect: ringTestRectOf(c.Bounds), first: obj.Paragraphs[0].Content, paras: len(obj.Paragraphs)})
	}
	return out
}

// ringGapNumberedBlocks returns, per item number, the label block of a
// pattern that leads its labels with a number: the numeral cell joined with
// the label cell beside it on the same row.
func ringGapNumberedBlocks(t *testing.T, res *shapegrid.ResolveResult, n int) map[int]ringTestRect {
	t.Helper()
	texts := ringGapTexts(t, res)
	blocks := map[int]ringTestRect{}
	for _, cue := range texts {
		k, err := strconv.Atoi(cue.first)
		if err != nil || cue.paras != 1 {
			continue
		}
		joined := false
		for _, label := range texts {
			if _, err := strconv.Atoi(label.first); err == nil && label.paras == 1 {
				continue
			}
			sameRow := math.Abs(label.rect.y0-cue.rect.y0) < 0.5 && math.Abs(label.rect.y1-cue.rect.y1) < 0.5
			touches := math.Abs(label.rect.x0-cue.rect.x1) < 1.5 || math.Abs(cue.rect.x0-label.rect.x1) < 1.5
			if sameRow && touches {
				blocks[k], joined = cue.rect.union(label.rect), true
			}
		}
		if !joined {
			t.Fatalf("numeral %d at %+v has no label beside it", k, cue.rect)
		}
	}
	if len(blocks) != n {
		t.Fatalf("%d label blocks, want %d", len(blocks), n)
	}
	return blocks
}

// ringGapLayerCircle is the circle of the resolved layer whose name is name,
// or ends in it when suffix is set (a figure eight prefixes its lobes).
func ringGapLayerCircle(t *testing.T, res *shapegrid.ResolveResult, name string, suffix bool) ringTestCircle {
	t.Helper()
	for _, c := range res.Cells {
		if c.Layer && (c.LayerName == name || (suffix && strings.HasSuffix(c.LayerName, "-"+name))) {
			if d := c.Bounds.CX - c.Bounds.CY; d < -2 || d > 2 {
				t.Fatalf("layer %s is not round: %d x %d EMU", c.LayerName, c.Bounds.CX, c.Bounds.CY)
			}
			return ringTestCircleOf(c.Bounds)
		}
	}
	t.Fatalf("no layer %q", name)
	return ringTestCircle{}
}

// ringGapCheck asserts that every gap equals want and that all of them agree,
// within a point.
func ringGapCheck(t *testing.T, name string, gaps map[int]float64, want float64) {
	t.Helper()
	lo, hi := math.Inf(1), math.Inf(-1)
	for k, g := range gaps {
		lo, hi = math.Min(lo, g), math.Max(hi, g)
		if math.Abs(g-want) > ringGapTolPt {
			t.Errorf("%s: label %d stands %.2fpt from its circle, want %.0fpt", name, k, g, want)
		}
	}
	if len(gaps) > 0 && hi-lo > ringGapTolPt {
		t.Errorf("%s: gaps run from %.2f to %.2fpt, want one gap for every label", name, lo, hi)
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func TestRingEdgeXAt(t *testing.T) {
	const cx, cy, r = 100.0, 50.0, 30.0
	cases := []struct {
		y     float64
		side  string
		want  float64
		label string
	}{
		{50, ringSideRight, 130, "right edge at the centre's height"},
		{50, ringSideLeft, 70, "left edge at the centre's height"},
		{50 + 18, ringSideRight, 124, "3-4-5 triangle below the centre"},
		{50 - 18, ringSideLeft, 76, "3-4-5 triangle above the centre"},
		{50 - 30, ringSideRight, 100, "the top of the circle"},
		{50 + 45, ringSideRight, 100, "below the circle: clamped to the centre line"},
		{50 - 45, ringSideLeft, 100, "above the circle: clamped to the centre line"},
		{50, "", 130, "an unknown side is the right one"},
	}
	for _, c := range cases {
		if got := ringEdgeXAt(cx, cy, r, c.y, c.side); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: ringEdgeXAt(y=%.0f, %q) = %.3f, want %.3f", c.label, c.y, c.side, got, c.want)
		}
	}
	// A band reaches as far as the circle does at the band's height nearest
	// the centre; a label block keeps the gap from there.
	bands := []struct {
		y0, y1, want float64
	}{
		{40, 60, 130}, // straddles the centre
		{68, 90, 124}, // below it: the band's top is nearest
		{0, 32, 124},  // above it: the band's bottom is nearest
		{90, 99, 100}, // clear of the circle
	}
	for _, b := range bands {
		if got := ringBandEdgeX(cx, cy, r, b.y0, b.y1, ringSideRight); math.Abs(got-b.want) > 1e-9 {
			t.Errorf("ringBandEdgeX(%.0f..%.0f) = %.3f, want %.3f", b.y0, b.y1, got, b.want)
		}
		if got := ringLabelEdgeX(cx, cy, r, b.y0, b.y1, 12, ringSideRight); math.Abs(got-(b.want+12)) > 1e-9 {
			t.Errorf("ringLabelEdgeX(%.0f..%.0f, right) = %.3f, want %.3f", b.y0, b.y1, got, b.want+12)
		}
		if got := ringLabelEdgeX(cx, cy, r, b.y0, b.y1, 12, ringSideLeft); math.Abs(got-(2*cx-b.want-12)) > 1e-9 {
			t.Errorf("ringLabelEdgeX(%.0f..%.0f, left) = %.3f, want %.3f", b.y0, b.y1, got, 2*cx-b.want-12)
		}
	}
}

// The spine cell resolves to the ring square: a square of the cell's height
// centred on the spine, whatever the lattice column's width.
func TestRingSpinePlacementResolvesToTheRingSquare(t *testing.T) {
	const w, h, cx, y0, side = 600.0, 300.0, 250.0, 20.0, 240.0
	ring := &jsonschema.GridCellInput{Layers: []jsonschema.LayerInput{{
		Name:  "disc",
		Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1},
		Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"accent1"`), Line: noLine},
	}}}
	place := ringSpinePlacement(cx, y0, side, ring)
	if ring.Fit != ringSpineFit || place.X1-place.X0 != ringSpinePt || place.X1-place.X0 >= 2*ringLabelGapPt || ringSpinePt <= ringEdgeMergePt {
		t.Fatalf("spine: fit %q, %.1fpt wide; want %q and a column between the merge distance and twice the label gap", ring.Fit, place.X1-place.X0, ringSpineFit)
	}
	// A label cell inside the ring's bounding square, where the old square
	// cell stood: the lattice takes both.
	label := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine}}
	grid, err := ringLattice([]ringPlacement{place, {X0: cx + 40, X1: w, Y0: y0, Y1: y0 + 30, Cell: label}}, w, h)
	if err != nil {
		t.Fatalf("lattice: %v", err)
	}
	res := cycleNodesResolveAt(t, grid, w, h)
	disc := ringGapLayerCircle(t, res, "disc", false)
	if math.Abs(disc.cx-cx) > 0.5 || math.Abs(disc.cy-(y0+side/2)) > 0.5 || math.Abs(disc.r-side/2) > 0.5 {
		t.Errorf("ring resolves to a circle of radius %.1f about (%.1f, %.1f), want %.1f about (%.1f, %.1f)", disc.r, disc.cx, disc.cy, side/2, cx, y0+side/2)
	}
}

// ringSettleRows keeps the heights that always hold when nothing better
// settles, and gives a row the smaller height of the wider place it stands in.
func TestRingSettleRows(t *testing.T) {
	spec := newRingSpec(4)
	items, err := spec.items()
	if err != nil {
		t.Fatal(err)
	}
	items = ringSidesLR(items, true)
	rowsSpec := ringRowsSpec{CentreY: 150, RadiusPt: 100, GapPt: 4, Top: 0, Bottom: 300, Heights: []float64{60, 60, 60, 60}}

	// No width function: plain ringLabelRows.
	plain, need, fits := ringSettleRows(items, rowsSpec, nil, nil)
	want, wantFits := ringLabelRows(items, rowsSpec)
	if fits != wantFits || fmt.Sprint(plain) != fmt.Sprint(want) || fmt.Sprint(need) != fmt.Sprint(rowsSpec.Heights) {
		t.Errorf("without a width function: rows %v need %v, want %v", plain, need, want)
	}

	// Every row is wide where it stands: all of them take the lower height.
	rows, need, fits := ringSettleRows(items, rowsSpec, func(ringRow) float64 { return 400 },
		func(_ int, widthPt float64) float64 {
			if widthPt >= 400*ringSettleWidthShare-1e-9 {
				return 30
			}
			return 60
		})
	if !fits || len(rows) != 4 {
		t.Fatalf("settled rows: %v fits=%v", rows, fits)
	}
	for _, r := range rows {
		if r.H != 30 || need[r.Index] != 30 {
			t.Errorf("row %d: %.0fpt tall (need %.0f), want 30", r.Index, r.H, need[r.Index])
		}
	}

	// A measure that asks for more than the narrowest width needs is capped
	// there: the base heights always hold.
	_, need, _ = ringSettleRows(items, rowsSpec, func(ringRow) float64 { return 10 }, func(int, float64) float64 { return 500 })
	for i, h := range need {
		if h != 60 {
			t.Errorf("item %d: need %.0f, want the base height 60", i, h)
		}
	}
}

// Outward moves the row of an item at 12 o'clock above its anchor and one at
// 6 o'clock below it, and leaves rows at 3 and 9 o'clock centred.
func TestRingLabelRowsOutward(t *testing.T) {
	nodes, err := newRingNodeSpec(4, 0.2, true).nodes(0.2, 0)
	if err != nil {
		t.Fatal(err)
	}
	items := ringSidesLR(ringNodeItems(nodes), true)
	spec := ringRowsSpec{CentreY: 200, RadiusPt: 100, RowPt: 40, GapPt: 4, Top: 0, Bottom: 400}
	centred, _ := ringLabelRows(items, spec)
	spec.Outward = 1
	moved, _ := ringLabelRows(items, spec)
	for i, r := range moved {
		want := centred[i].Y
		switch i {
		case 0: // 12 o'clock
			want -= 20
		case 2: // 6 o'clock
			want += 20
		}
		if math.Abs(r.Y-want) > 1e-6 || r.AnchorY != centred[i].AnchorY {
			t.Errorf("item %d: row centre %.2f (anchor %.2f), want %.2f with the anchor unchanged", i, r.Y, r.AnchorY, want)
		}
	}
}

// ringProtectEdges moves the other edges onto a gap edge they would have
// merged with, so the lattice keeps the gap edge where the layout put it.
func TestRingProtectEdges(t *testing.T) {
	cell := func() *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine}}
	}
	// Row A's number cue ends at 200.35; row B's block ends (its gap edge) at
	// 201.31: under a point apart, so the lattice alone keeps 200.35 for both.
	rows := [5]ringPlacement{
		{X0: 100, X1: 178.35, Y0: 0, Y1: 30, Cell: cell()},
		{X0: 178.35, X1: 200.35, Y0: 0, Y1: 30, Cell: cell()},
		{X0: 100, X1: 201.31, Y0: 40, Y1: 70, Cell: cell()},
		{X0: 300, X1: 400, Y0: 0, Y1: 30, Cell: cell()},
		{X0: 300.6, X1: 400, Y0: 40, Y1: 70, Cell: cell()},
	}
	places := rows[:]
	ringProtectEdges(places, []float64{201.31, 300, 300.6})
	if rows[1].X1 != 201.31 || rows[2].X1 != 201.31 {
		t.Errorf("edges beside the gap edge 201.31: %.2f and %.2f, want both on it", rows[1].X1, rows[2].X1)
	}
	// Two gap edges under a point apart meet at their mean.
	if math.Abs(rows[3].X0-300.3) > 1e-9 || math.Abs(rows[4].X0-300.3) > 1e-9 {
		t.Errorf("gap edges 300 and 300.6 become %.2f and %.2f, want 300.30 for both", rows[3].X0, rows[4].X0)
	}
	if first, fourth := rows[0], rows[3]; first.X0 != 100 || first.X1 != 178.35 || fourth.X1 != 400 {
		t.Errorf("edges away from every gap edge moved: %+v", rows)
	}
	if _, err := ringLattice(places, 500, 100); err != nil {
		t.Errorf("lattice: %v", err)
	}
	// No gap edges: nothing moves.
	before := rows[0]
	ringProtectEdges(places, nil)
	if rows[0].X0 != before.X0 || rows[0].X1 != before.X1 {
		t.Errorf("no gap edges: %+v moved", rows[0])
	}
}
