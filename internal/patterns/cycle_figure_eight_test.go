package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func cfeValues(n, left int) *CycleFigureEightValues {
	pool := []CycleRingPhase{
		{Label: "Sense demand", Description: "Daily point-of-sale feeds"},
		{Label: "Forecast", Description: "Weekly consensus by region"},
		{Label: "Shape demand", Description: "Promotions steer the mix"},
		{Label: "Commit volumes", Description: "Orders frozen on Thursday"},
		{Label: "Plan supply", Description: "Capacity matched by line"},
		{Label: "Source", Description: "Call-offs to 40 suppliers"},
		{Label: "Make", Description: "Schedules fixed for five days"},
		{Label: "Deliver", Description: "Service level fed back"},
		{Label: "Extra", Description: "One too many"},
	}
	return &CycleFigureEightValues{Phases: append([]CycleRingPhase(nil), pool[:n]...), LeftCount: left}
}

// cfeSplits lists every legal left_count for n phases, 0 (the default) first.
func cfeSplits(n int) []int {
	out := []int{0}
	lo, hi := cfeLeftRange(n)
	for k := lo; k <= hi; k++ {
		out = append(out, k)
	}
	return out
}

func cfeExpand(t *testing.T, ctx ExpandContext, v *CycleFigureEightValues, ovr *CycleFigureEightOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	p := &cycleFigureEight{}
	if err := p.Validate(v, overrides, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

// cfeLobes returns the two lobe cells (the ones that carry layers), left first.
func cfeLobes(t *testing.T, grid *jsonschema.ShapeGridInput) [2]*jsonschema.GridCellInput {
	t.Helper()
	var lobes []*jsonschema.GridCellInput
	for _, r := range grid.Rows {
		for _, c := range r.Cells {
			if c != nil && len(c.Layers) > 0 {
				lobes = append(lobes, c)
			}
		}
	}
	if len(lobes) != 2 {
		t.Fatalf("%d cells carry layers, want the two lobes", len(lobes))
	}
	if !strings.HasPrefix(lobes[0].Layers[0].Name, "left-") || !strings.HasPrefix(lobes[1].Layers[0].Name, "right-") {
		t.Fatalf("lobes out of order: %s, %s", lobes[0].Layers[0].Name, lobes[1].Layers[0].Name)
	}
	return [2]*jsonschema.GridCellInput{lobes[0], lobes[1]}
}

func cfeLayerCentre(l jsonschema.LayerInput) (x, y float64) {
	return l.Frame.X + l.Frame.W/2, l.Frame.Y + l.Frame.H/2
}

func TestCycleFigureEight_Metadata(t *testing.T) {
	p, ok := Default().Get("cycle-figure-eight")
	if !ok {
		t.Fatal("cycle-figure-eight not registered")
	}
	if p.Name() != "cycle-figure-eight" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" || p.Description() == "" || p.CellsHint() == "" {
		t.Error("UseWhen/NotWhen/Description/CellsHint must be non-empty")
	}
	for _, sibling := range []string{"cycle-ring", "cycle-intake", "before-after", "swimlane"} {
		if !strings.Contains(p.UseWhen(), sibling) || !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("UseWhen/NotWhen should contrast with %s", sibling)
		}
	}
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" || len(tx.NarrativeRole) == 0 || len(tx.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tx)
	}
	for _, sib := range tx.PairsWith {
		if _, ok := Default().Get(sib); !ok {
			t.Errorf("PairsWith names unregistered pattern %q", sib)
		}
	}
	if PatternMotif("cycle-figure-eight") != MotifDiagram {
		t.Errorf("motif = %q, want diagram", PatternMotif("cycle-figure-eight"))
	}
}

func TestCycleFigureEight_SchemaIsValidJSON(t *testing.T) {
	data, err := json.Marshal((&cycleFigureEight{}).Schema())
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if m["$schema"] == nil || m["$defs"] == nil {
		t.Error("schema must be a root schema with $defs")
	}
	if len(data) > 6000 {
		t.Errorf("schema is %d bytes, over the 6000 byte budget", len(data))
	}
	for _, key := range []string{"phases", "label", "description", "highlight", "left_count", "left_label", "right_label", "thickness", "cell_accent_mode", "header_size", "body_size", "semantic_accent"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("schema does not publish %q", key)
		}
	}
	// The schema states the budget the warnings enforce and the width it needs.
	for _, want := range []string{"60 characters", "40 once a lobe holds 4", "580pt wide"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema does not state %q", want)
		}
	}
}

func TestCycleFigureEight_Validate(t *testing.T) {
	p := &cycleFigureEight{}
	for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
		for _, k := range cfeSplits(n) {
			if err := p.Validate(cfeValues(n, k), nil, nil); err != nil {
				t.Errorf("n=%d left_count=%d: unexpected error: %v", n, k, err)
			}
		}
	}
	var ve *ValidationError
	if err := p.Validate(cfeValues(3, 0), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMinItems || !strings.Contains(err.Error(), "cycle-ring") {
		t.Errorf("3 phases: want min_items with the cycle-ring hint, got %v", err)
	}
	if err := p.Validate(cfeValues(9, 0), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxItems || !strings.Contains(err.Error(), "cycle-ring") {
		t.Errorf("9 phases: want max_items with the cycle-ring hint, got %v", err)
	}
	// Each lobe holds 2-4: the legal left_count depends on the phase count.
	for _, tc := range []struct{ n, k int }{{4, 1}, {4, 3}, {5, 4}, {6, 5}, {7, 2}, {8, 3}, {8, 5}, {6, -1}} {
		if err := p.Validate(cfeValues(tc.n, tc.k), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeOutOfRange || ve.Path != "left_count" {
			t.Errorf("n=%d left_count=%d: want out_of_range at left_count, got %v", tc.n, tc.k, err)
		}
	}

	v := cfeValues(4, 0)
	v.Phases[1].Label = " "
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "phases[1].label") {
		t.Errorf("want phases[1].label required, got %v", err)
	}

	// Budgets count characters, not bytes.
	v = cfeValues(4, 0)
	v.Phases[0].Label = strings.Repeat("ü", cfeLabelMax)
	v.Phases[0].Description = strings.Repeat("€", cfeDescMax)
	v.LeftLabel, v.RightLabel = strings.Repeat("é", cfeLobeLabelMax), strings.Repeat("ö", cfeLobeLabelMax)
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("multi-byte text inside the budget rejected: %v", err)
	}
	for _, mutate := range []func(*CycleFigureEightValues) string{
		func(v *CycleFigureEightValues) string { v.Phases[0].Label += "ü"; return "phases[0].label" },
		func(v *CycleFigureEightValues) string {
			v.Phases[2].Description = strings.Repeat("€", cfeDescMax+1)
			return "phases[2].description"
		},
		func(v *CycleFigureEightValues) string { v.LeftLabel += "é"; return "left_label" },
		func(v *CycleFigureEightValues) string { v.RightLabel += "ö"; return "right_label" },
	} {
		c := *v
		c.Phases = append([]CycleRingPhase(nil), v.Phases...)
		path := mutate(&c)
		if err := p.Validate(&c, nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxLength || !strings.Contains(err.Error(), path) {
			t.Errorf("%s one over: want max_length, got %v", path, err)
		}
	}

	v = cfeValues(5, 0)
	v.Phases[0].Highlight, v.Phases[3].Highlight = true, true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Errorf("two highlights: got %v", err)
	}

	for _, ovr := range []*CycleFigureEightOverrides{
		{Thickness: "huge"},
		{TextOverrides: TextOverrides{CellAccentMode: "rainbow"}},
	} {
		if err := p.Validate(cfeValues(4, 0), ovr, nil); err == nil {
			t.Errorf("overrides %+v accepted", ovr)
		}
	}
	if err := p.Validate(cfeValues(4, 0), &CycleFigureEightOverrides{Thickness: "thick", TextOverrides: TextOverrides{CellAccentMode: "progressive"}}, nil); err != nil {
		t.Errorf("valid overrides rejected: %v", err)
	}
	if err := p.Validate(cfeValues(4, 0), nil, map[int]any{0: &CellOverride{}}); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeUnknownKey {
		t.Errorf("cell_overrides must be rejected with unknown_key, got %v", err)
	}
	if err := p.Validate(cfeValues(4, 0), &CycleRingOverrides{}, nil); err == nil {
		t.Error("wrong overrides type must be rejected")
	}
	if err := p.Validate(&CycleRingValues{}, nil, nil); err == nil {
		t.Error("wrong values type must be rejected")
	}
	// Expand refuses what Validate refuses.
	if _, err := p.Expand(ExpandContext{}, cfeValues(8, 3), nil, nil); err == nil {
		t.Error("Expand accepted a left_count Validate rejects")
	}
	if _, err := p.Expand(ExpandContext{}, cfeValues(3, 0), nil, nil); err == nil {
		t.Error("Expand accepted 3 phases")
	}
}

// Every count and split: two fit-contain lobe cells of layers only, one
// segment and one badge per phase, badges numbered along the path (the right
// lobe continues where the left stops), each badge on its segment's
// centreline, the left lobe's labels right-aligned on the left and the right
// lobe's left-aligned on the right.
func TestCycleFigureEight_ExpandLayout(t *testing.T) {
	for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
		for _, k := range cfeSplits(n) {
			ctx := cycleRingCtx(828, 349)
			v := cfeValues(n, k)
			grid := cfeExpand(t, ctx, v, nil)
			left := cfeLeftCount(n, k)
			if k == 0 && left != (n+1)/2 {
				t.Errorf("n=%d: default left count %d, want %d", n, left, (n+1)/2)
			}
			number := 0
			lay, err := cfeMeasure(ctx, v, &CycleFigureEightOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			for li, lobe := range cfeLobes(t, grid) {
				prefix := []string{"left-", "right-"}[li]
				want := []int{left, n - left}[li]
				spec := lay.lobes[li].spec
				if lobe.Fit != ringSpineFit || lobe.Shape != nil {
					t.Errorf("n=%d k=%d: lobe %d is the spine canvas (%s) of layers only, got fit %q shape %v", n, k, li, ringSpineFit, lobe.Fit, lobe.Shape)
				}
				segments, badges := cycleRingLayers(lobe, prefix+"segment-"), cycleRingLayers(lobe, prefix+"badge-")
				if len(segments) != want || len(badges) != want {
					t.Fatalf("n=%d k=%d lobe %d: %d segments and %d badges, want %d", n, k, li, len(segments), len(badges), want)
				}
				if got := len(lobe.Layers); got != 2*want+3 {
					t.Errorf("n=%d k=%d lobe %d: %d layers, want %d (segments, badges, two arms, one arrowhead)", n, k, li, got, 2*want+3)
				}
				for i, seg := range segments {
					number++
					if seg.Name != fmt.Sprintf("%ssegment-%d", prefix, number) || badges[i].Name != fmt.Sprintf("%sbadge-%d", prefix, number) {
						t.Errorf("n=%d k=%d: layers %s / %s, want number %d", n, k, seg.Name, badges[i].Name, number)
					}
					if seg.Shape.Geometry != "blockArc" || seg.Frame != spec.bandFrame().layer() {
						t.Errorf("n=%d k=%d %s: %s in frame %+v, want a blockArc in the lobe's band frame %+v", n, k, seg.Name, seg.Shape.Geometry, seg.Frame, spec.bandFrame())
					}
					if seg.Shape.Adjustments["adj3"] != 20000 || string(seg.Shape.Line) != `"none"` {
						t.Errorf("n=%d k=%d %s: adj3 %d line %s", n, k, seg.Name, seg.Shape.Adjustments["adj3"], seg.Shape.Line)
					}
					// blockArc runs clockwise adj1 → adj2; the badge is at its middle
					// on the centreline.
					a1, a2 := seg.Shape.Adjustments["adj1"], seg.Shape.Adjustments["adj2"]
					mid := float64(a1)/60000 + float64((a2-a1+21600000)%21600000)/120000
					wx, wy := pointOnCircle(0.5, 0.5, spec.Radius, mid)
					b := badges[i]
					if cx, cy := cfeLayerCentre(b); math.Abs(cx-wx) > 1e-4 || math.Abs(cy-wy) > 1e-4 {
						t.Errorf("n=%d k=%d %s: centre (%.4f, %.4f), want (%.4f, %.4f)", n, k, b.Name, cx, cy, wx, wy)
					}
					if b.Shape.Geometry != "ellipse" || b.Frame.W != b.Frame.H || !strings.Contains(string(b.Shape.Text), fmt.Sprintf(`"content":"%d"`, number)) {
						t.Errorf("n=%d k=%d %s: %s %vx%v text %s", n, k, b.Name, b.Shape.Geometry, b.Frame.W, b.Frame.H, b.Shape.Text)
					}
				}
			}

			for i, row := range lay.rows {
				want := ringSideLeft
				if i >= left {
					want = ringSideRight
				}
				if row.side != want {
					t.Errorf("n=%d k=%d phase %d: label on the %s, want %s", n, k, i+1, row.side, want)
				}
				// Rows run top to bottom in path order within a lobe.
				if i > 0 && i != left && row.y0 < lay.rows[i-1].y1-0.01 {
					t.Errorf("n=%d k=%d phase %d: row %.1f-%.1f starts above the end of phase %d's (%.1f)", n, k, i+1, row.y0, row.y1, i, lay.rows[i-1].y1)
				}
			}
			raw, _ := json.Marshal(grid)
			if l, r := strings.Count(string(raw), `"align":"r","vertical_align":"t"`), strings.Count(string(raw), `"align":"l","vertical_align":"t"`); l+r != 2*n {
				t.Errorf("n=%d k=%d: %d label / numeral cells, want %d", n, k, l+r, 2*n)
			}
		}
	}
}

// The path is one continuous line: up from the crossing and counter-clockwise
// round the left lobe, through the crossing, clockwise round the right lobe.
// Each lobe's first segment starts on the upper side of the crossing and its
// last ends on the lower side, both the same angle from the crossing point.
func TestFigureEightPathOrderIsContinuous(t *testing.T) {
	for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
		for _, k := range cfeSplits(n) {
			lay, err := cfeMeasure(cycleRingCtx(828, 349), cfeValues(n, k), &CycleFigureEightOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			seen := map[int]bool{}
			for _, it := range lay.items {
				if seen[it.Index] {
					t.Errorf("n=%d k=%d: phase %d appears twice", n, k, it.Index)
				}
				seen[it.Index] = true
			}
			if len(seen) != n {
				t.Errorf("n=%d k=%d: %d phases on the path, want %d", n, k, len(seen), n)
			}
			left, right := lay.lobes[0], lay.lobes[1]
			if left.spec.Clockwise || !right.spec.Clockwise {
				t.Errorf("n=%d k=%d: left lobe clockwise=%v right lobe clockwise=%v, want false / true", n, k, left.spec.Clockwise, right.spec.Clockwise)
			}
			if right.from != len(left.items) || left.from != 0 {
				t.Errorf("n=%d k=%d: right lobe starts at phase %d after %d on the left", n, k, right.from, len(left.items))
			}
			open := cfeOpenDeg(left.spec)
			// Left lobe: from −open (above 3 o'clock) round to +open (below it).
			if got := left.items[0].StartDeg; math.Abs(got-normDeg(-open)) > 1e-6 {
				t.Errorf("n=%d k=%d: left lobe starts at %.2f°, want %.2f°", n, k, got, normDeg(-open))
			}
			if got := left.items[len(left.items)-1].EndDeg; math.Abs(got-open) > 1e-6 {
				t.Errorf("n=%d k=%d: left lobe ends at %.2f°, want %.2f°", n, k, got, open)
			}
			// Right lobe: from 180 + open (above 9 o'clock) round to 180 − open.
			if got := right.items[0].StartDeg; math.Abs(got-(180+open)) > 1e-6 {
				t.Errorf("n=%d k=%d: right lobe starts at %.2f°, want %.2f°", n, k, got, 180+open)
			}
			if got := right.items[len(right.items)-1].EndDeg; math.Abs(got-(180-open)) > 1e-6 {
				t.Errorf("n=%d k=%d: right lobe ends at %.2f°, want %.2f°", n, k, got, 180-open)
			}
			// Travel along a lobe only ever advances.
			for _, lobe := range lay.lobes {
				travelled := 0.0
				for _, it := range lobe.items {
					at := ringTravelDeg(lobe.items[0].StartDeg, it.MidDeg, lobe.spec.Clockwise)
					if at <= travelled {
						t.Errorf("n=%d k=%d %s: phase %d sits at %.1f° of travel, not past %.1f°", n, k, lobe.prefix, it.Index, at, travelled)
					}
					travelled = at
				}
			}
		}
	}
}

// The two lobes are equal squares that share an edge, whose middle is the
// crossing point; they sit centred in the area with equal label columns
// either side. Each ring is centred in its square and a little smaller than
// it — as large as a full-width arm to the crossing point allows — so the two
// rings stop a few percent of a side short of each other, and the line from a
// lobe's end to the crossing point is tangent to its centreline.
func TestFigureEightLobesMeetAtTheCrossing(t *testing.T) {
	for _, body := range cycleRingBodies {
		for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
			ctx := cycleRingCtx(body.w, body.h)
			lay, err := cfeMeasure(ctx, cfeValues(n, 0), &CycleFigureEightOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(lay.x0+lay.side-body.w/2) > 0.01 {
				t.Errorf("%s n=%d: the lobes meet at x=%.1f, want the middle %.1f", body.name, n, lay.x0+lay.side, body.w/2)
			}
			for _, lobe := range lay.lobes {
				f, spec := lobe.spec.bandFrame(), lobe.spec
				if math.Abs(f.X-f.Y) > 1e-9 || math.Abs(f.W-f.H) > 1e-9 || math.Abs(f.X+f.W/2-0.5) > 1e-9 {
					t.Errorf("%s n=%d %s: band frame %+v is not a square centred in the lobe's cell", body.name, n, lobe.prefix, f)
				}
				if f.W > 1 || f.W < 0.94 {
					t.Errorf("%s n=%d %s: the ring is %.1f%% of its square, want 94-100%%", body.name, n, lobe.prefix, f.W*100)
				}
				if want := cfeBandRadius(spec.Thickness); math.Abs(spec.Radius-want) > 1e-9 {
					t.Errorf("%s n=%d %s: centreline radius %.4f, want %.4f (the largest whose arm fits the cell)", body.name, n, lobe.prefix, spec.Radius, want)
				}
				// Tangency: the triangle centre – lobe's end – crossing point has
				// its right angle at the lobe's end.
				open := degToRad(cfeOpenDeg(spec))
				if got := spec.Radius / math.Cos(open); math.Abs(got-cfeCrossDist) > 1e-9 {
					t.Errorf("%s n=%d %s: the tangent at the lobe's end meets the axis %.4f from the centre, want the crossing point at %.1f", body.name, n, lobe.prefix, got, cfeCrossDist)
				}
			}

			grid := cfeExpand(t, ctx, cfeValues(n, 0), nil)
			for _, lobe := range cfeLobes(t, grid) {
				lobe.Shape = &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`)}
			}
			ApplyGridDefaults(grid)
			res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
			var rings []pptx.RectEmu
			for _, c := range res.Cells {
				if c.ShapeSpec != nil && c.ShapeSpec.Geometry == "ellipse" {
					rings = append(rings, c.Bounds)
				}
			}
			if len(rings) != 2 {
				t.Fatalf("%s n=%d: %d lobes resolved", body.name, n, len(rings))
			}
			l, r := rings[0], rings[1]
			if l.X > r.X {
				l, r = r, l
			}
			const ptEMU = 12700
			if d := l.CX - l.CY; d < -1 || d > 1 || l.CX != r.CX || l.CY != r.CY || l.Y != r.Y {
				t.Errorf("%s n=%d: lobes resolve to %dx%d and %dx%d EMU, want equal squares on one line", body.name, n, l.CX, l.CY, r.CX, r.CY)
			}
			// Each lobe is the square of its spine cell's height about the
			// spine, so the two meet within the lattice's edge rounding.
			if gap := r.X - (l.X + l.CX); gap < -ptEMU || gap > ptEMU {
				t.Errorf("%s n=%d: %.2fpt between the lobes, want them touching (under 1pt)", body.name, n, float64(gap)/ptEMU)
			}
			if float64(l.CX)/ptEMU < cfeMinSidePt-1 {
				t.Errorf("%s n=%d: lobe side %.0fpt is under the %.0fpt minimum", body.name, n, float64(l.CX)/ptEMU, cfeMinSidePt)
			}
		}
	}
}

// cfeAxisPoint is the point d along a rotated layer's long axis from its
// centre (positive towards the frame's top, the way a triangle points).
func cfeAxisPoint(l jsonschema.LayerInput, d float64) (x, y float64) {
	cx, cy := cfeLayerCentre(l)
	sin, cos := math.Sincos(degToRad(l.Shape.Rotation))
	return cx + sin*d, cy - cos*d
}

// The crossing: per lobe two rect arms and one arrowhead. Each arm is exactly
// as wide as the band, in the band's own tone without an outline, lies on
// the line from the lobe's end to the crossing point (tangent to the lobe's
// centreline there), starts inside the lobe's end segment and ends just past
// the crossing point, where the other cell's arm overlaps it — so the ribbon
// has no gap at the crossing — and its frame stays inside its cell.
func TestFigureEightCrossingArms(t *testing.T) {
	for _, thickness := range []string{"thin", "", "thick"} {
		grid := cfeExpand(t, cycleRingCtx(828, 349), cfeValues(6, 0), &CycleFigureEightOverrides{Thickness: thickness})
		lay, err := cfeMeasure(cycleRingCtx(828, 349), cfeValues(6, 0), &CycleFigureEightOverrides{Thickness: thickness})
		if err != nil {
			t.Fatal(err)
		}
		for li, lobe := range cfeLobes(t, grid) {
			prefix := []string{"left-", "right-"}[li]
			touchX := []float64{1, 0}[li]
			spec := lay.lobes[li].spec
			ends := []float64{lay.lobes[li].items[0].StartDeg, lay.lobes[li].items[len(lay.lobes[li].items)-1].EndDeg}
			for ai, name := range []string{"arm-in", "arm-out"} {
				arms := cycleRingLayers(lobe, prefix+name)
				if len(arms) != 1 {
					t.Fatalf("%q %s%s: %d layers", thickness, prefix, name, len(arms))
				}
				arm := arms[0]
				if arm.Shape.Geometry != "rect" || string(arm.Shape.Line) != `"none"` || string(arm.Shape.Fill) != string(ringBandTone(ExpandContext{}, "accent1").fillJSON()) {
					t.Errorf("%q %s: %s line %s fill %s, want a rect in the band's tone without outline", thickness, arm.Name, arm.Shape.Geometry, arm.Shape.Line, arm.Shape.Fill)
				}
				if arm.Frame.X < 0 || arm.Frame.Y < 0 || arm.Frame.X+arm.Frame.W > 1+1e-9 || arm.Frame.Y+arm.Frame.H > 1+1e-9 {
					t.Errorf("%q %s: frame %+v leaves the cell", thickness, arm.Name, arm.Frame)
				}
				if math.Abs(arm.Frame.W-spec.Thickness) > 1e-6 {
					t.Errorf("%q %s: %.4f wide on a %.4f band, want the band's full width", thickness, arm.Name, arm.Frame.W, spec.Thickness)
				}
				// The far end is cfeArmPastFrac past the crossing point, on the axis.
				tx, ty := cfeAxisPoint(arm, arm.Frame.H/2-cfeArmPastFrac)
				if math.Abs(tx-touchX) > 1e-4 || math.Abs(ty-0.5) > 1e-4 {
					t.Errorf("%q %s: its axis is at (%.4f, %.4f) %.3f before its end, want the crossing point (%.0f, 0.5)", thickness, arm.Name, tx, ty, cfeArmPastFrac, touchX)
				}
				// The near end is cfeArmLapFrac behind the lobe's end, inside the
				// segment (which is drawn over it in the same fill).
				nx, ny := cfeAxisPoint(arm, -(arm.Frame.H/2 - cfeArmLapFrac))
				ex, ey := pointOnCircle(0.5, 0.5, spec.Radius, ends[ai])
				if math.Abs(nx-ex) > 1e-4 || math.Abs(ny-ey) > 1e-4 {
					t.Errorf("%q %s: its axis is at (%.4f, %.4f) %.3f past its start, want the lobe's end (%.4f, %.4f)", thickness, arm.Name, nx, ny, cfeArmLapFrac, ex, ey)
				}
				// Tangent: the axis is perpendicular to the radius at the lobe's end.
				sin, cos := math.Sincos(degToRad(arm.Shape.Rotation))
				rx, ry := pointOnCircle(0, 0, 1, ends[ai])
				if dot := rx*sin - ry*cos; math.Abs(dot) > 1e-6 {
					t.Errorf("%q %s: axis is %.2f° off the tangent at the lobe's end", thickness, arm.Name, radToDeg(math.Asin(dot)))
				}
			}
			// The arms are drawn first: segments, arrowhead and badges sit on top.
			if !strings.HasSuffix(lobe.Layers[0].Name, "arm-in") || !strings.HasSuffix(lobe.Layers[1].Name, "arm-out") || !strings.Contains(lobe.Layers[2].Name, "segment-") {
				t.Errorf("%q lobe %d: layers start %s, %s, %s", thickness, li, lobe.Layers[0].Name, lobe.Layers[1].Name, lobe.Layers[2].Name)
			}
		}
		// The two arms of one band are collinear across the cells' shared edge:
		// the left lobe's leaving arm and the right lobe's entering arm (and the
		// other pair) have the same slope.
		lobes := cfeLobes(t, grid)
		for _, pair := range [][2]string{{"left-arm-out", "right-arm-in"}, {"left-arm-in", "right-arm-out"}} {
			a, b := cycleRingLayers(lobes[0], pair[0])[0], cycleRingLayers(lobes[1], pair[1])[0]
			if d := math.Mod(math.Abs(a.Shape.Rotation-b.Shape.Rotation), 360); math.Abs(d-180) > 1e-6 {
				t.Errorf("%q: %s (%.2f°) and %s (%.2f°) are not one straight band", thickness, pair[0], a.Shape.Rotation, pair[1], b.Shape.Rotation)
			}
		}
	}
}

// A segment with a colour of its own (the highlight, or any segment in a
// tinted accent mode) keeps it to its end: the band-toned arm beside it starts a
// regular segment gap away instead of running under it.
func TestFigureEightArmLeavesAGapAtAColouredSegment(t *testing.T) {
	gapOf := func(v *CycleFigureEightValues, ovr *CycleFigureEightOverrides, li, ai int) float64 {
		t.Helper()
		ctx := cycleRingCtx(828, 349)
		lay, err := cfeMeasure(ctx, v, ovr)
		if err != nil {
			t.Fatal(err)
		}
		lobe := lay.lobes[li]
		deg := lobe.items[0].StartDeg
		if ai == 1 {
			deg = lobe.items[len(lobe.items)-1].EndDeg
		}
		arm := cycleRingLayers(cfeLobes(t, cfeExpand(t, ctx, v, ovr))[li], lobe.prefix+[]string{"arm-in", "arm-out"}[ai])[0]
		nx, ny := cfeAxisPoint(arm, -arm.Frame.H/2)
		ex, ey := pointOnCircle(0.5, 0.5, lobe.spec.Radius, deg)
		tx := []float64{1, 0}[li]
		// Signed distance from the lobe's end to the arm's start, towards the
		// crossing point.
		d := math.Hypot(nx-ex, ny-ey)
		if math.Hypot(nx-tx, ny-0.5) > math.Hypot(ex-tx, ey-0.5) {
			d = -d
		}
		return d
	}
	want := func(v *CycleFigureEightValues, ovr *CycleFigureEightOverrides) float64 {
		lay, err := cfeMeasure(cycleRingCtx(828, 349), v, ovr)
		if err != nil {
			t.Fatal(err)
		}
		return lay.lobes[0].spec.Radius * degToRad(cfeArmGapDeg)
	}

	plain := &CycleFigureEightOverrides{}
	hi := cfeValues(6, 0)
	hi.Phases[3].Highlight = true // the right lobe's first segment
	for _, tc := range []struct {
		name   string
		li, ai int
		gap    bool
	}{{"left-arm-in", 0, 0, false}, {"left-arm-out", 0, 1, false}, {"right-arm-in", 1, 0, true}, {"right-arm-out", 1, 1, false}} {
		got := gapOf(hi, plain, tc.li, tc.ai)
		switch {
		case tc.gap && math.Abs(got-want(hi, plain)) > 1e-4:
			t.Errorf("highlight on phase 4: %s starts %.4f past the lobe's end, want the segment gap %.4f", tc.name, got, want(hi, plain))
		case !tc.gap && math.Abs(got+cfeArmLapFrac) > 1e-4:
			t.Errorf("highlight on phase 4: %s starts %.4f from the lobe's end, want %.3f inside the segment", tc.name, got, cfeArmLapFrac)
		}
	}
	for _, mode := range []string{"alternate", "progressive"} {
		tinted := &CycleFigureEightOverrides{TextOverrides: TextOverrides{CellAccentMode: mode}}
		for li := 0; li < 2; li++ {
			for ai := 0; ai < 2; ai++ {
				if got := gapOf(cfeValues(6, 0), tinted, li, ai); math.Abs(got-want(cfeValues(6, 0), tinted)) > 1e-4 {
					t.Errorf("%s: arm %d of lobe %d starts %.4f past the lobe's end, want the segment gap", mode, ai, li, got)
				}
			}
		}
	}
}

// The direction cue: one arrowhead per lobe on the arm that ENTERS it (the
// two upper arms), a triangle longer than wide in the muted text colour — not
// a page-coloured cut-out and not an accent — centred on the arm's axis,
// pointing away from the crossing point, on the stretch of the arm the other
// band does not cross. The two are mirror images.
func TestFigureEightArrowheads(t *testing.T) {
	for _, thickness := range []string{"thin", "", "thick"} {
		ctx := cycleRingCtx(828, 349)
		ovr := &CycleFigureEightOverrides{Thickness: thickness}
		lobes := cfeLobes(t, cfeExpand(t, ctx, cfeValues(6, 0), ovr))
		lay, err := cfeMeasure(ctx, cfeValues(6, 0), ovr)
		if err != nil {
			t.Fatal(err)
		}
		var heads [2]jsonschema.LayerInput
		for li, lobe := range lobes {
			prefix := []string{"left-", "right-"}[li]
			touchX := []float64{1, 0}[li]
			spec := lay.lobes[li].spec
			found := cycleRingLayers(lobe, prefix+"arrowhead")
			in := cycleRingLayers(lobe, prefix+"arm-in")[0]
			if len(found) != 1 {
				t.Fatalf("%q %sarrowhead: %d layers, want one", thickness, prefix, len(found))
			}
			head := found[0]
			heads[li] = head
			if head.Shape.Geometry != "triangle" || string(head.Shape.Line) != `"none"` || string(head.Shape.Fill) != string(cfeHeadTone(ExpandContext{}, "accent1").fillJSON()) {
				t.Errorf("%q %s: %s line %s fill %s, want a triangle in the deeper rung of the band's ladder without outline", thickness, head.Name, head.Shape.Geometry, head.Shape.Line, head.Shape.Fill)
			}
			if d := math.Mod(math.Abs(head.Shape.Rotation-in.Shape.Rotation), 360); math.Abs(d-180) > 1e-6 {
				t.Errorf("%q %s: rotation %.2f° on an arm at %.2f°, want it pointing away from the crossing point", thickness, head.Name, head.Shape.Rotation, in.Shape.Rotation)
			}
			if head.Frame.H < 1.29*head.Frame.W || head.Frame.W > 0.65*spec.Thickness || head.Frame.W < 0.45*spec.Thickness {
				t.Errorf("%q %s: %.4f x %.4f on a %.4f band, want it longer than wide and 45-65%% of the band across", thickness, head.Name, head.Frame.W, head.Frame.H, spec.Thickness)
			}
			// On the arm's axis: its centre is collinear with the crossing point
			// along the arm's direction.
			hx, hy := cfeLayerCentre(head)
			sin, cos := math.Sincos(degToRad(in.Shape.Rotation))
			if off := (hx-touchX)*cos + (hy-0.5)*sin; math.Abs(off) > 1e-6 {
				t.Errorf("%q %s: centre is %.5f off the arm's axis", thickness, head.Name, off)
			}
			// Clear of the other band (tip to tail beyond where it crosses) and
			// short of the lobe's end.
			ex, ey := pointOnCircle(0.5, 0.5, spec.Radius, lay.lobes[li].items[0].StartDeg)
			dist, at := math.Hypot(ex-touchX, ey-0.5), math.Hypot(hx-touchX, hy-0.5)
			ux, uy := (touchX-ex)/dist, (0.5-ey)/dist
			free := spec.Thickness / 2 * (1 + math.Abs(ux*ux-uy*uy)) / math.Abs(2*ux*uy)
			if at-head.Frame.H/2 < free || at+head.Frame.H/2 > dist {
				t.Errorf("%q %s: spans %.4f-%.4f from the crossing point, want it between the other band's edge (%.4f) and the lobe's end (%.4f)", thickness, head.Name, at-head.Frame.H/2, at+head.Frame.H/2, free, dist)
			}
			// Drawn over the segments, under the badges.
			order := map[string]int{}
			for i, l := range lobe.Layers {
				switch {
				case strings.Contains(l.Name, "segment-"):
					order["segment"] = i
				case strings.Contains(l.Name, "arrowhead"):
					order["head"] = i
				case strings.Contains(l.Name, "badge-") && order["badge"] == 0:
					order["badge"] = i
				}
			}
			if order["head"] < order["segment"] || order["head"] > order["badge"] {
				t.Errorf("%q %s: layer %d, want it after the segments (%d) and before the badges (%d)", thickness, head.Name, order["head"], order["segment"], order["badge"])
			}
		}
		// Mirror images about the cells' shared edge; the left one points up
		// and to the left, the right one up and to the right.
		lx, ly := cfeLayerCentre(heads[0])
		rx, ry := cfeLayerCentre(heads[1])
		if math.Abs(lx-(1-rx)) > 1e-5 || math.Abs(ly-ry) > 1e-5 || heads[0].Frame.W != heads[1].Frame.W || heads[0].Frame.H != heads[1].Frame.H ||
			math.Abs(heads[0].Shape.Rotation+heads[1].Shape.Rotation-360) > 1e-6 {
			t.Errorf("%q: arrowheads %+v @%.2f° and %+v @%.2f° are not mirror images", thickness, heads[0].Frame, heads[0].Shape.Rotation, heads[1].Frame, heads[1].Shape.Rotation)
		}
		if l, r := heads[0].Shape.Rotation, heads[1].Shape.Rotation; l <= 270 || l >= 360 || r <= 0 || r >= 90 {
			t.Errorf("%q: arrowheads point %.1f° and %.1f° clockwise from up, want up-left and up-right", thickness, l, r)
		}
		if ly >= 0.5 {
			t.Errorf("%q: arrowheads at y=%.3f, want them on the upper arms", thickness, ly)
		}
	}
}

// N = 6 (3 + 3), default thickness: the band is 20% of a lobe 97.6% of its
// square, its centreline radius 0.3904, so the opening is acos(0.3904 / 0.5)
// = 38.67° either side of the crossing point. The golden carries the same
// values.
func TestFigureEightBlockArcAdjValuesPinned(t *testing.T) {
	lobes := cfeLobes(t, cfeExpand(t, ExpandContext{}, cfeValues(6, 0), nil))
	// Left lobe, counter-clockwise from 321.33°: the preset draws clockwise,
	// so each segment's adj1 is its END in path order.
	wantLeft := [][2]int64{{13866634, 19279903}, {8093366, 13506634}}
	for i, seg := range cycleRingLayers(lobes[0], "left-segment-") {
		if i >= len(wantLeft) {
			break
		}
		a1, a2 := seg.Shape.Adjustments["adj1"], seg.Shape.Adjustments["adj2"]
		if a1 != wantLeft[i][0] || a2 != wantLeft[i][1] {
			t.Errorf("%s: adj1/adj2 = %d/%d, want %d/%d", seg.Name, a1, a2, wantLeft[i][0], wantLeft[i][1])
		}
	}
	first := cycleRingLayers(lobes[1], "right-segment-")[0]
	if a1 := first.Shape.Adjustments["adj1"]; a1 != 13120097 { // 180° + 38.67°
		t.Errorf("right-segment-4 starts at %d, want 13120097", a1)
	}
}

// For every count, split and shipped body size: no two cells overlap, no
// label cell intersects a lobe's square, everything stays inside the content
// area.
func TestFigureEightNoOverlapForEveryCount(t *testing.T) {
	for _, body := range cycleRingBodies {
		for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
			for _, k := range cfeSplits(n) {
				t.Run(fmt.Sprintf("%s/%d/%d", body.name, n, k), func(t *testing.T) {
					ctx := cycleRingCtx(body.w, body.h)
					grid := cfeExpand(t, ctx, cfeValues(n, k), nil)
					for _, lobe := range cfeLobes(t, grid) {
						lobe.Shape = &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`)}
					}
					ApplyGridDefaults(grid)
					res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
					assertNoOverlap(t, res)
					boxes, rings := 0, 0
					for _, c := range res.Cells {
						if c.ShapeSpec == nil {
							continue
						}
						boxes++
						if c.Bounds.Y < 0 || c.Bounds.Y+c.Bounds.CY > ctx.LayoutBounds.Height+12700 || c.Bounds.X < 0 || c.Bounds.X+c.Bounds.CX > ctx.LayoutBounds.Width+12700 {
							t.Errorf("cell %+v leaves the %v x %vpt content area", c.Bounds, body.w, body.h)
						}
						if c.ShapeSpec.Geometry == "ellipse" {
							rings++
						}
					}
					if rings != 2 || boxes != 2+2*n {
						t.Errorf("%d lobes and %d boxes, want 2 and %d", rings, boxes, 2+2*n)
					}
				})
			}
		}
	}
}

// An area too narrow (or too short) for two lobes and their label columns is
// refused at Expand with a located fit_overflow whose fix swaps to
// cycle-ring; PostExpandWarnings stays silent there (the refusal says it).
func TestFigureEightRefusesNarrowArea(t *testing.T) {
	p := &cycleFigureEight{}
	for _, tc := range []struct {
		name   string
		w, h   float64
		refuse bool
	}{
		{"abstract 50% segment", 337, 294, true},
		{"abstract 60% segment", 405, 294, true},
		{"p-style 50% segment", 443, 360, true},
		{"just under the minimum width", 578, 300, true},
		{"too short", 700, 130, true},
		{"the minimum", 580, 140, false},
		{"p-style 60% segment", 532, 360, true},
		{"p-style 70% segment", 623, 360, false},
		{"abstract full width, 70% height", 687, 205, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := cycleRingCtx(tc.w, tc.h)
			grid, err := p.Expand(ctx, cfeValues(6, 0), nil, nil)
			if !tc.refuse {
				if err != nil || grid == nil {
					t.Fatalf("%.0f x %.0fpt refused: %v", tc.w, tc.h, err)
				}
				return
			}
			var ve *ValidationError
			if grid != nil || !errors.As(err, &ve) {
				t.Fatalf("%.0f x %.0fpt: want a ValidationError and no grid, got %v", tc.w, tc.h, err)
			}
			if ve.Code != ErrCodeFitOverflow || ve.Path != "values" || ve.Pattern != "cycle-figure-eight" {
				t.Errorf("refusal = %s at %q for %q, want fit_overflow at values", ve.Code, ve.Path, ve.Pattern)
			}
			for _, want := range []string{"580pt wide", "140pt tall", fmt.Sprintf("%.0f x %.0fpt", tc.w, tc.h), "cycle-ring"} {
				if !strings.Contains(ve.Message, want) {
					t.Errorf("message does not say %q: %s", want, ve.Message)
				}
			}
			if ve.Fix == nil || ve.Fix.Kind != "swap_pattern" {
				t.Fatalf("fix = %+v, want swap_pattern", ve.Fix)
			}
			suggested, _ := ve.Fix.Params["suggested"].([]any)
			if len(suggested) != 1 {
				t.Fatalf("suggested = %v", ve.Fix.Params)
			}
			if s, _ := suggested[0].(map[string]any); s["from"] != "cycle-figure-eight" || s["to"] != "cycle-ring" || s["rationale"] == "" {
				t.Errorf("suggestion = %v, want cycle-figure-eight -> cycle-ring with a rationale", suggested[0])
			}
			if w := p.PostExpandWarnings(ctx, cfeValues(6, 0), nil); w != nil {
				t.Errorf("warnings on a refused area: %v", w)
			}
		})
	}
	if cfeMinWidthPt != 580 || cfeMinHeightPt != 140 {
		t.Errorf("minimum area = %.0f x %.0fpt; the schema, the docs and the refusal message say 580 x 140", cfeMinWidthPt, cfeMinHeightPt)
	}
}

// By default no segment is a solid accent; a highlighted phase is the only
// one — on either lobe and in every accent mode — and its badge flips to the
// page colour. The other badges are the neutral dark, so none of them is a
// second solid accent; only a cell accent mode gives each badge its accent.
func TestFigureEightHighlightIsTheOnlySolidAccent(t *testing.T) {
	for _, mode := range []string{"", "alternate", "progressive"} {
		ovr := &CycleFigureEightOverrides{TextOverrides: TextOverrides{CellAccentMode: mode}}
		for _, lobe := range cfeLobes(t, cfeExpand(t, ExpandContext{}, cfeValues(8, 0), ovr)) {
			for _, l := range lobe.Layers {
				var name string
				if !strings.Contains(l.Name, "badge-") && json.Unmarshal(l.Shape.Fill, &name) == nil && strings.HasPrefix(name, "accent") {
					t.Errorf("mode %q: %s is a solid %s without a highlight", mode, l.Name, name)
				}
			}
		}
		for _, hi := range []int{1, 5} {
			v := cfeValues(8, 0)
			v.Phases[hi].Highlight = true
			lobes := cfeLobes(t, cfeExpand(t, ExpandContext{}, v, ovr))
			var solid []string
			accentBadges, darkBadges := 0, 0
			for li, lobe := range lobes {
				prefix := []string{"left-", "right-"}[li]
				solid = append(solid, cycleRingSolidAccents(lobe, prefix+"segment-")...)
				solid = append(solid, cycleRingSolidAccents(lobe, prefix+"arm-")...)
				solid = append(solid, cycleRingSolidAccents(lobe, prefix+"arrowhead")...)
				accentBadges += len(cycleRingSolidAccents(lobe, prefix+"badge-"))
				for _, b := range cycleRingLayers(lobe, prefix+"badge-") {
					if string(b.Shape.Fill) == `"dk2"` {
						darkBadges++
					}
				}
				for _, b := range cycleRingLayers(lobe, fmt.Sprintf("%sbadge-%d", prefix, hi+1)) {
					if string(b.Shape.Fill) != `"lt1"` {
						t.Errorf("mode %q: the highlight's badge fill = %s, want lt1", mode, b.Shape.Fill)
					}
				}
			}
			if len(solid) != 1 || !strings.Contains(solid[0], fmt.Sprintf("segment-%d=", hi+1)) {
				t.Errorf("mode %q highlight %d: solid accent blocks = %v, want only segment-%d", mode, hi+1, solid, hi+1)
			}
			wantAccent, wantDark := 7, 0
			if mode == "" {
				wantAccent, wantDark = 0, 7
			}
			if accentBadges != wantAccent || darkBadges != wantDark {
				t.Errorf("mode %q highlight %d: %d accent and %d neutral-dark badges, want %d and %d", mode, hi+1, accentBadges, darkBadges, wantAccent, wantDark)
			}
		}
	}
}

func TestCycleFigureEight_ExpandStyling(t *testing.T) {
	// DefaultAccent: the template's primary fill, not a hard-coded accent1.
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1F2937"}, {Name: "lt2", RGB: "#EEEEEE"},
		{Name: "accent1", RGB: "#FFD54F"}, {Name: "accent2", RGB: "#0B3D91"},
	}}}
	raw, _ := json.Marshal(cfeExpand(t, ctx, cfeValues(4, 0), nil))
	if want := ctx.DefaultAccent(); want != "accent2" || !strings.Contains(string(raw), `"accent2"`) || strings.Contains(string(raw), `"accent1"`) {
		t.Errorf("default accent %s not used throughout", want)
	}
	// Ink is measured: the default badge is the neutral dark with the page
	// colour as ink whatever the accent, and under a cell accent mode a light
	// accent takes dark numerals in its badges.
	lobes := cfeLobes(t, cfeExpand(t, ctx, cfeValues(4, 0), &CycleFigureEightOverrides{TextOverrides: TextOverrides{Accent: "accent1"}}))
	if badge := cycleRingLayers(lobes[1], "right-badge-3")[0]; string(badge.Shape.Fill) != `"dk2"` || !strings.Contains(string(badge.Shape.Text), `"color":"lt1"`) {
		t.Errorf("default badge: fill %s text %s, want lt1 ink on dk2", badge.Shape.Fill, badge.Shape.Text)
	}
	lobes = cfeLobes(t, cfeExpand(t, ctx, cfeValues(4, 0), &CycleFigureEightOverrides{TextOverrides: TextOverrides{Accent: "accent1", CellAccentMode: "alternate"}}))
	if badge := cycleRingLayers(lobes[0], "left-badge-1")[0]; string(badge.Shape.Fill) != `"accent1"` || !strings.Contains(string(badge.Shape.Text), `"color":"dk2"`) {
		t.Errorf("badge on a light accent: fill %s text %s, want dk2 ink on accent1", badge.Shape.Fill, badge.Shape.Text)
	}

	// An accent override reaches the segments, the highlight and its
	// numerals; sizes and lobe titles reach the text.
	v := cfeValues(4, 0)
	v.Phases[2].Highlight = true
	v.LeftLabel, v.RightLabel = "Demand", "Supply"
	raw, _ = json.Marshal(cfeExpand(t, ExpandContext{}, v, &CycleFigureEightOverrides{TextOverrides: TextOverrides{Accent: "accent3", HeaderSize: 16, BodySize: 13}}))
	out := string(raw)
	for _, want := range []string{`"accent3"`, `"size":16`, `"size":13`, `"Sense demand"`, `"Daily point-of-sale feeds"`, `"Demand"`, `"Supply"`, `"left-centre"`, `"right-centre"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expansion missing %s", want)
		}
	}
	if strings.Contains(out, `"accent1"`) {
		t.Error("accent override not applied everywhere")
	}

	// cell_accent_mode x base accent, running over the whole path (the right
	// lobe continues the walk): uniform sets every segment in the base
	// accent's Lighter 80% swatch under neutral-dark badges; the others tint
	// each segment with its own accent and give each badge that accent.
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			ovr := &CycleFigureEightOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}
			lobes := cfeLobes(t, cfeExpand(t, ExpandContext{}, cfeValues(6, 2), ovr))
			badgeFills := map[string]bool{}
			i := 0
			for li, lobe := range lobes {
				prefix := []string{"left-", "right-"}[li]
				segments := cycleRingLayers(lobe, prefix+"segment-")
				for bi, b := range cycleRingLayers(lobe, prefix+"badge-") {
					var fill string
					_ = json.Unmarshal(b.Shape.Fill, &fill)
					badgeFills[fill] = true
					want := ResolveCellAccent(base, i, mode)
					wantBadge := want
					if mode == "uniform" {
						wantBadge = "dk2"
					}
					if fill != wantBadge {
						t.Errorf("%s/%s badge %d fill = %s, want %s", base, mode, i+1, fill, wantBadge)
					}
					// Uniform or not, a segment is the Lighter 80% swatch of
					// its own accent (the base accent under uniform).
					segFill := string(segments[bi].Shape.Fill)
					if wantSeg := `{"color":"` + want + `","lumMod":20000,"lumOff":80000}`; segFill != wantSeg {
						t.Errorf("%s/%s segment %d fill = %s, want %s", base, mode, i+1, segFill, wantSeg)
					}
					i++
				}
				// The arms that cross between the lobes take the base accent's
				// swatch in every mode.
				for _, arm := range cycleRingLayers(lobe, prefix+"arm-") {
					if got, wantArm := string(arm.Shape.Fill), `{"color":"`+base+`","lumMod":20000,"lumOff":80000}`; got != wantArm {
						t.Errorf("%s/%s %s fill = %s, want %s", base, mode, arm.Name, got, wantArm)
					}
				}
			}
			if want := map[string]int{"uniform": 1, "alternate": 2, "progressive": 6}[mode]; len(badgeFills) != want {
				t.Errorf("%s/%s: %d badge fills, want %d", base, mode, len(badgeFills), want)
			}
		}
	}

	// semantic_accent resolves through the theme.
	ctx = ExpandContext{}
	ctx.Theme.SemanticAccents = map[string]string{"positive": "accent6"}
	raw, _ = json.Marshal(cfeExpand(t, ctx, cfeValues(4, 0), &CycleFigureEightOverrides{TextOverrides: TextOverrides{SemanticAccent: "positive"}}))
	if !strings.Contains(string(raw), `"accent6"`) {
		t.Error("semantic_accent not resolved")
	}
}

// Thickness sets the band of both lobes and with it the opening at the
// crossing: a thicker band opens wider.
func TestFigureEightThickness(t *testing.T) {
	prev := 0.0
	for _, tc := range []struct {
		name string
		adj3 int64
	}{{"thin", 14000}, {"regular", 20000}, {"thick", 28000}} {
		lobes := cfeLobes(t, cfeExpand(t, cycleRingCtx(899, 360), cfeValues(4, 0), &CycleFigureEightOverrides{Thickness: tc.name}))
		for li, lobe := range lobes {
			prefix := []string{"left-", "right-"}[li]
			if got := cycleRingLayers(lobe, prefix+"segment-")[0].Shape.Adjustments["adj3"]; got != tc.adj3 {
				t.Errorf("%s lobe %d: adj3 = %d, want %d", tc.name, li, got, tc.adj3)
			}
		}
		lay, err := cfeMeasure(cycleRingCtx(899, 360), cfeValues(4, 0), &CycleFigureEightOverrides{Thickness: tc.name})
		if err != nil {
			t.Fatal(err)
		}
		open := cfeOpenDeg(lay.lobes[0].spec)
		if open <= prev || open < 25 || open > 50 {
			t.Errorf("%s: opening %.1f° (previous %.1f°), want it to grow with the band inside 25-50°", tc.name, open, prev)
		}
		prev = open
	}
}

func TestCycleFigureEight_PostExpandWarnings(t *testing.T) {
	p := &cycleFigureEight{}
	for _, body := range cycleRingBodies {
		for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
			for _, k := range cfeSplits(n) {
				if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), cfeValues(n, k), nil); len(w) != 0 {
					t.Errorf("%s n=%d k=%d: realistic content warned: %v", body.name, n, k, w)
				}
			}
		}
	}
	// Outside its range, or with the wrong type, it reports nothing (Validate does).
	if w := p.PostExpandWarnings(ExpandContext{}, cfeValues(3, 0), nil); w != nil {
		t.Errorf("3 phases: %v", w)
	}
	if w := p.PostExpandWarnings(ExpandContext{}, &CycleRingValues{}, nil); w != nil {
		t.Errorf("wrong type: %v", w)
	}

	// A lobe title whose word cannot fit the hole of a small lobe.
	v := cfeValues(4, 0)
	v.LeftLabel = "Standardisations"
	w := p.PostExpandWarnings(cycleRingCtx(580, 150), v, nil)
	if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong) || !strings.Contains(w[0], "left_label") {
		t.Errorf("lobe title warning = %v", w)
	}

	// A row that cannot hold its text in a short area names the phase.
	v = cfeValues(8, 0)
	for i := range v.Phases {
		v.Phases[i].Description = "Buyers compare three vendors first"
	}
	w = p.PostExpandWarnings(cycleRingCtx(700, 142), v, nil)
	if len(w) == 0 || !strings.Contains(strings.Join(w, "\n"), "phases[0].description needs") || !strings.Contains(w[0], "label row") {
		t.Errorf("short-area warnings = %v", w)
	}
}

func TestCycleFigureEight_Golden(t *testing.T) {
	p := &cycleFigureEight{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "cycle-figure-eight", "default.golden.json"))
}

func TestCycleFigureEight_ExemplarIsClean(t *testing.T) {
	p := &cycleFigureEight{}
	if err := p.Validate(p.ExemplarValues(), nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, body := range cycleRingBodies {
		if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), p.ExemplarValues(), nil); len(w) != 0 {
			t.Errorf("%s: exemplar warns: %v", body.name, w)
		}
	}
}

func TestCycleFigureEight_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"devops loop", &ContentHints{ItemCount: 6}, "cycle-figure-eight"},
		{"infinity loop", nil, "cycle-figure-eight"},
		{"figure eight", &ContentHints{ItemCount: 4}, "cycle-figure-eight"},
		{"build and run loop", &ContentHints{ItemCount: 8}, "cycle-figure-eight"},
		{"two coupled cycles", nil, "cycle-figure-eight"},
		// One loop, or a loop in a narrow panel, stays a ring.
		{"cycle in a half-width panel", &ContentHints{ItemCount: 5}, "cycle-ring"},
		{"continuous improvement cycle", &ContentHints{ItemCount: 4}, "cycle-ring"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}

// TestCycleFigureEightLabelsKeepOneGapFromTheirLobe: every label block
// (numeral + label) stands the same horizontal gap from the outer edge of its
// own lobe within its row's height, for every count and split in every body
// size, read from the resolved grid.
func TestCycleFigureEightLabelsKeepOneGapFromTheirLobe(t *testing.T) {
	for _, body := range ringGapBodies {
		for n := cfeMinPhases; n <= cfeMaxPhases; n++ {
			lo, hi := cfeLeftRange(n)
			for _, k := range []int{0, lo, hi} {
				name := fmt.Sprintf("%s/%d/left=%d", body.name, n, k)
				ctx, v := cycleRingCtx(body.w, body.h), cfeValues(n, k)
				res := cycleNodesResolveAt(t, cfeExpand(t, ctx, v, nil), body.w, body.h)
				left := cfeLeftCount(n, k)
				gaps := map[int]float64{}
				for num, block := range ringGapNumberedBlocks(t, res, n) {
					lobe := ringGapLayerCircle(t, res, fmt.Sprintf("segment-%d", num), true)
					gaps[num] = lobe.hGap(block)
					if clear := lobe.clear(block); clear < ringGapClearPt {
						t.Errorf("%s: label %d %+v stands %.1fpt from its lobe", name, num, block, clear)
					}
					// Left lobe's labels to the left of it, right lobe's to the right.
					if onLeft := (block.x0+block.x1)/2 < lobe.cx; onLeft != (num <= left) {
						t.Errorf("%s: label %d %+v is on the wrong side of its lobe (centre %.1f)", name, num, block, lobe.cx)
					}
					if block.x0 < -0.5 || block.x1 > body.w+0.5 || block.y0 < -0.5 || block.y1 > body.h+0.5 {
						t.Errorf("%s: label %d %+v leaves the %.0f x %.0fpt area", name, num, block, body.w, body.h)
					}
				}
				ringGapCheck(t, name, gaps, ringLabelGapPt)
			}
		}
	}
}
