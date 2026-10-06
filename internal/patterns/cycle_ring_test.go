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
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func cycleRingValues(n int) *CycleRingValues {
	pool := []CycleRingPhase{
		{Label: "Awareness", Description: "Referrals bring 4 in 10 leads"},
		{Label: "Consideration", Description: "Buyers compare three vendors"},
		{Label: "Purchase", Description: "Contract signed in 21 days"},
		{Label: "Onboarding", Description: "First value inside 30 days"},
		{Label: "Adoption", Description: "Seven in ten seats active weekly"},
		{Label: "Service", Description: "Churn risk peaks at first ticket"},
		{Label: "Renewal", Description: "Decision made 90 days ahead"},
		{Label: "Advocacy", Description: "Promoters refer the next buyer"},
		{Label: "Extra", Description: "One too many"},
	}
	return &CycleRingValues{Phases: append([]CycleRingPhase(nil), pool[:n]...)}
}

// cycleRingBodies are the content areas of the smallest shipped template, a
// typical one and the local p-style, in points.
var cycleRingBodies = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"midnight-blue", 828, 349},
	{"p-style", 899, 360},
}

func cycleRingCtx(w, h float64) ExpandContext {
	return ExpandContext{LayoutBounds: LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}}
}

func cycleRingExpand(t *testing.T, ctx ExpandContext, v *CycleRingValues, ovr *CycleRingOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	p := &cycleRing{}
	if err := p.Validate(v, overrides, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

// cycleRingCell returns the ring cell (the one that carries layers).
func cycleRingCell(t *testing.T, grid *jsonschema.ShapeGridInput) *jsonschema.GridCellInput {
	t.Helper()
	var ring *jsonschema.GridCellInput
	for _, r := range grid.Rows {
		for _, c := range r.Cells {
			if c != nil && len(c.Layers) > 0 {
				if ring != nil {
					t.Fatal("more than one cell carries layers")
				}
				ring = c
			}
		}
	}
	if ring == nil {
		t.Fatal("no ring cell")
	}
	return ring
}

func cycleRingLayers(ring *jsonschema.GridCellInput, prefix string) []jsonschema.LayerInput {
	var out []jsonschema.LayerInput
	for _, l := range ring.Layers {
		if strings.HasPrefix(l.Name, prefix) {
			out = append(out, l)
		}
	}
	return out
}

func TestCycleRing_Metadata(t *testing.T) {
	p, ok := Default().Get("cycle-ring")
	if !ok {
		t.Fatal("cycle-ring not registered")
	}
	if p.Name() != "cycle-ring" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" || p.Description() == "" || p.CellsHint() == "" {
		t.Error("UseWhen/NotWhen/Description/CellsHint must be non-empty")
	}
	for _, sibling := range []string{"cycle-nodes", "cycle-intake", "cycle-figure-eight", "radial-hub", "process-flow"} {
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
	if PatternMotif("cycle-ring") != MotifDiagram {
		t.Errorf("motif = %q, want diagram", PatternMotif("cycle-ring"))
	}
}

func TestCycleRing_SchemaIsValidJSON(t *testing.T) {
	data, err := json.Marshal((&cycleRing{}).Schema())
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
	for _, key := range []string{"phases", "label", "description", "highlight", "center", "sublabel", "style", "direction", "thickness", "labels", "cell_accent_mode", "header_size", "body_size", "semantic_accent"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("schema missing %q", key)
		}
	}
	// The schema states the per-count budgets the pattern warns at.
	for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
		if !strings.Contains(string(data), fmt.Sprintf("%d", cycleRingDescBudget(n))) || !strings.Contains(string(data), fmt.Sprintf("%d", cycleRingLabelBudget(n))) {
			t.Errorf("schema does not state the %d-phase budgets (%d / %d)", n, cycleRingLabelBudget(n), cycleRingDescBudget(n))
		}
	}
}

func TestCycleRing_Validate(t *testing.T) {
	p := &cycleRing{}
	for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
		if err := p.Validate(cycleRingValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}
	var ve *ValidationError
	if err := p.Validate(cycleRingValues(3), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMinItems || !strings.Contains(err.Error(), "cycle-nodes") {
		t.Errorf("3 phases: want min_items with the cycle-nodes hint, got %v", err)
	}
	if err := p.Validate(cycleRingValues(9), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxItems || !strings.Contains(err.Error(), "cycle-figure-eight") {
		t.Errorf("9 phases: want max_items with the split / cycle-figure-eight hint, got %v", err)
	}

	v := cycleRingValues(4)
	v.Phases[1].Label = " "
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "phases[1].label") {
		t.Errorf("want phases[1].label required, got %v", err)
	}

	// Budgets count characters, not bytes.
	v = cycleRingValues(4)
	v.Phases[0].Label = strings.Repeat("ü", cycleRingLabelMax)
	v.Phases[0].Description = strings.Repeat("€", cycleRingDescMax)
	v.Center = &CycleRingCenter{Label: strings.Repeat("é", cycleRingCenterMax), Sublabel: strings.Repeat("ö", cycleRingSublabelMax)}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("multi-byte text inside the budget rejected: %v", err)
	}
	for _, mutate := range []func(*CycleRingValues) string{
		func(v *CycleRingValues) string { v.Phases[0].Label += "ü"; return "phases[0].label" },
		func(v *CycleRingValues) string {
			v.Phases[2].Description = strings.Repeat("€", cycleRingDescMax+1)
			return "phases[2].description"
		},
		func(v *CycleRingValues) string { v.Center.Label += "é"; return "center.label" },
		func(v *CycleRingValues) string { v.Center.Sublabel += "ö"; return "center.sublabel" },
	} {
		c := *v
		c.Phases = append([]CycleRingPhase(nil), v.Phases...)
		centre := *v.Center
		c.Center = &centre
		path := mutate(&c)
		if err := p.Validate(&c, nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxLength || !strings.Contains(err.Error(), path) {
			t.Errorf("%s one over: want max_length, got %v", path, err)
		}
	}

	v = cycleRingValues(5)
	v.Phases[0].Highlight, v.Phases[3].Highlight = true, true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Errorf("two highlights: got %v", err)
	}

	for _, ovr := range []*CycleRingOverrides{
		{Style: "chevrons"}, {Direction: "anticlockwise"}, {Thickness: "huge"}, {Labels: "inside"},
		{TextOverrides: TextOverrides{CellAccentMode: "rainbow"}},
	} {
		if err := p.Validate(cycleRingValues(4), ovr, nil); err == nil {
			t.Errorf("overrides %+v accepted", ovr)
		}
	}
	if err := p.Validate(cycleRingValues(4), &CycleRingOverrides{Style: "arrows", Direction: cycleRingCCW, Thickness: "thick", Labels: "legend", TextOverrides: TextOverrides{CellAccentMode: "progressive"}}, nil); err != nil {
		t.Errorf("valid overrides rejected: %v", err)
	}
	if err := p.Validate(cycleRingValues(4), nil, map[int]any{0: &CellOverride{}}); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeUnknownKey {
		t.Errorf("cell_overrides must be rejected with unknown_key, got %v", err)
	}
	if err := p.Validate(cycleRingValues(4), &TextOverrides{}, nil); err == nil {
		t.Error("wrong overrides type must be rejected")
	}
	if err := p.Validate(&StateShiftHubValues{}, nil, nil); err == nil {
		t.Error("wrong values type must be rejected")
	}
}

// Geometry per layer for every count: one segment and one badge per phase,
// segments tiling the ring clockwise from 12 o'clock, each badge on its
// segment's centreline, and labels split left / right in badge order.
func TestCycleRing_ExpandLayout(t *testing.T) {
	for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
		grid := cycleRingExpand(t, cycleRingCtx(828, 349), cycleRingValues(n), nil)
		ring := cycleRingCell(t, grid)
		if ring.Fit != "contain" || ring.Shape != nil {
			t.Errorf("n=%d: the ring cell is a fit-contain canvas of layers only, got fit %q shape %v", n, ring.Fit, ring.Shape)
		}
		segments, badges := cycleRingLayers(ring, "segment-"), cycleRingLayers(ring, "badge-")
		if len(segments) != n || len(badges) != n || len(ring.Layers) != 2*n {
			t.Fatalf("n=%d: %d segments, %d badges, %d layers", n, len(segments), len(badges), len(ring.Layers))
		}
		step := int64(360 * 60000 / n)
		for i, seg := range segments {
			if seg.Shape.Geometry != "blockArc" || seg.Frame != (jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1}) {
				t.Errorf("n=%d segment %d: %s in frame %+v, want a blockArc in the whole square", n, i, seg.Shape.Geometry, seg.Frame)
			}
			a1, a2 := seg.Shape.Adjustments["adj1"], seg.Shape.Adjustments["adj2"]
			wantStart := (16200000 + 180000 + int64(i)*step) % 21600000 // 12 o'clock + half the 6° gap
			if d := a1 - wantStart; d < -8 || d > 8 {                   // n = 7 does not divide the circle
				t.Errorf("n=%d segment %d: adj1 = %d, want %d", n, i, a1, wantStart)
			}
			if sweep := (a2 - a1 + 21600000) % 21600000; math.Abs(float64(sweep-(step-360000))) > 8 {
				t.Errorf("n=%d segment %d: sweep %d, want %d", n, i, sweep, step-360000)
			}
			if seg.Shape.Adjustments["adj3"] != 20000 {
				t.Errorf("n=%d segment %d: adj3 = %d, want 20000", n, i, seg.Shape.Adjustments["adj3"])
			}
			if string(seg.Shape.Line) != `"none"` {
				t.Errorf("n=%d segment %d: filled segment carries an outline %s", n, i, seg.Shape.Line)
			}
			// The badge sits on the centreline (radius 0.4) at the segment's middle.
			b := badges[i]
			mid := float64(a1)/60000 + float64((a2-a1+21600000)%21600000)/120000
			wx, wy := pointOnCircle(0.5, 0.5, 0.4, mid)
			if cx, cy := b.Frame.X+b.Frame.W/2, b.Frame.Y+b.Frame.H/2; math.Abs(cx-wx) > 1e-4 || math.Abs(cy-wy) > 1e-4 {
				t.Errorf("n=%d badge %d: centre (%.4f, %.4f), want (%.4f, %.4f)", n, i, cx, cy, wx, wy)
			}
			if b.Shape.Geometry != "ellipse" || b.Frame.W != b.Frame.H || !strings.Contains(string(b.Shape.Text), fmt.Sprintf(`"content":"%d"`, i+1)) {
				t.Errorf("n=%d badge %d: %s %vx%v text %s", n, i, b.Shape.Geometry, b.Frame.W, b.Frame.H, b.Shape.Text)
			}
		}

		// Labels: phases 1..ceil(n/2) right of the ring (left-aligned), the
		// rest left of it (right-aligned); 2n text cells beside the ring.
		lay, err := cycleRingMeasure(cycleRingCtx(828, 349), cycleRingValues(n), &CycleRingOverrides{})
		if err != nil {
			t.Fatal(err)
		}
		if lay.legend {
			t.Fatalf("n=%d: a full slide must use the outside layout", n)
		}
		right := n / 2
		for i, row := range lay.rows {
			want := ringSideRight
			if i >= right {
				want = ringSideLeft
			}
			if row.side != want {
				t.Errorf("n=%d phase %d: label on the %s, want %s", n, i+1, row.side, want)
			}
		}
		raw, _ := json.Marshal(grid)
		if got := strings.Count(string(raw), `"align":"l","vertical_align":"t"`) + strings.Count(string(raw), `"align":"r","vertical_align":"t"`); got != 2*n {
			t.Errorf("n=%d: %d label / numeral cells, want %d", n, got, 2*n)
		}
	}
}

// For every count and every shipped body size, no two label cells overlap,
// none intersects the ring's square, the ring resolves to a square, and every
// label row is inside the content area.
func TestCycleRingNoOverlapForEveryCount(t *testing.T) {
	for _, body := range cycleRingBodies {
		for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
			for _, style := range []string{"", cycleRingLabelsLegend} {
				t.Run(fmt.Sprintf("%s/%d/%s", body.name, n, style), func(t *testing.T) {
					ctx := cycleRingCtx(body.w, body.h)
					grid := cycleRingExpand(t, ctx, cycleRingValues(n), &CycleRingOverrides{Labels: style})
					ring := cycleRingCell(t, grid)
					// A marker shape makes the layers-only ring cell resolve as a box.
					ring.Shape = &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"lt2"`)}
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
							if d := c.Bounds.CX - c.Bounds.CY; d < -1 || d > 1 {
								t.Errorf("ring resolves to %d x %d EMU, want a square", c.Bounds.CX, c.Bounds.CY)
							}
							if float64(c.Bounds.CX)/12700 < ringMinSidePt-1 {
								t.Errorf("ring side %.0fpt is under the %0.fpt minimum", float64(c.Bounds.CX)/12700, ringMinSidePt)
							}
						}
					}
					if rings != 1 || boxes != 1+2*n {
						t.Errorf("%d rings and %d boxes, want 1 and %d", rings, boxes, 1+2*n)
					}
				})
			}
		}
	}
}

// N = 4, default: the four quarter segments, 12 o'clock first, with the 6°
// gap centred on the axes. The golden carries the same values.
func TestCycleRingBlockArcAdjValuesPinned(t *testing.T) {
	grid := cycleRingExpand(t, ExpandContext{}, cycleRingValues(4), nil)
	want := [][3]int64{
		{16380000, 21420000, 20000}, // 273° → 357°
		{180000, 5220000, 20000},    // 3° → 87°
		{5580000, 10620000, 20000},  // 93° → 177°
		{10980000, 16020000, 20000}, // 183° → 267°
	}
	for i, seg := range cycleRingLayers(cycleRingCell(t, grid), "segment-") {
		got := [3]int64{seg.Shape.Adjustments["adj1"], seg.Shape.Adjustments["adj2"], seg.Shape.Adjustments["adj3"]}
		if got != want[i] {
			t.Errorf("segment %d adjustments = %v, want %v", i+1, got, want[i])
		}
	}

	// Counter-clockwise: phase 1 is the top-LEFT quarter, drawn clockwise
	// from its far end back to 12 o'clock.
	grid = cycleRingExpand(t, ExpandContext{}, cycleRingValues(4), &CycleRingOverrides{Direction: cycleRingCCW})
	first := cycleRingLayers(cycleRingCell(t, grid), "segment-")[0]
	if first.Shape.Adjustments["adj1"] != 10980000 || first.Shape.Adjustments["adj2"] != 16020000 {
		t.Errorf("counter-clockwise segment 1 = %v, want 183° → 267°", first.Shape.Adjustments)
	}
	lay, _ := cycleRingMeasure(ExpandContext{}, cycleRingValues(4), &CycleRingOverrides{Direction: cycleRingCCW})
	if lay.rows[0].side != ringSideLeft || lay.rows[3].side != ringSideRight {
		t.Errorf("counter-clockwise label sides = %s … %s, want left … right", lay.rows[0].side, lay.rows[3].side)
	}

	// Thickness: thin 14%, thick 28% of the diameter.
	for name, adj := range map[string]int64{"thin": 14000, "regular": 20000, "thick": 28000} {
		grid = cycleRingExpand(t, ExpandContext{}, cycleRingValues(4), &CycleRingOverrides{Thickness: name})
		if got := cycleRingLayers(cycleRingCell(t, grid), "segment-")[0].Shape.Adjustments["adj3"]; got != adj {
			t.Errorf("thickness %s: adj3 = %d, want %d", name, got, adj)
		}
	}
}

func TestCycleRing_ArrowsStyle(t *testing.T) {
	for _, ccw := range []bool{false, true} {
		ovr := &CycleRingOverrides{Style: cycleRingStyleArrows}
		if ccw {
			ovr.Direction = cycleRingCCW
		}
		grid := cycleRingExpand(t, ExpandContext{}, cycleRingValues(6), ovr)
		segs := cycleRingLayers(cycleRingCell(t, grid), "segment-")
		if len(segs) != 6 {
			t.Fatalf("%d arrows, want 6", len(segs))
		}
		for i, s := range segs {
			if s.Shape.Geometry != "circularArrow" || s.Shape.FlipH != ccw || len(s.Shape.Adjustments) != 5 {
				t.Errorf("arrow %d: %s flip=%v adj=%v", i+1, s.Shape.Geometry, s.Shape.FlipH, s.Shape.Adjustments)
			}
			if s.Frame.X < 0 || s.Frame.Y < 0 || s.Frame.X+s.Frame.W > 1 || s.Frame.Y+s.Frame.H > 1 {
				t.Errorf("arrow %d frame %+v leaves the cell", i+1, s.Frame)
			}
			// The head is as wide as the band, the shaft narrower.
			if s.Shape.Adjustments["adj1"] >= 2*s.Shape.Adjustments["adj5"] {
				t.Errorf("arrow %d: shaft %d is not narrower than the head (2 x %d)", i+1, s.Shape.Adjustments["adj1"], s.Shape.Adjustments["adj5"])
			}
		}
	}
}

// cycleRingSolidAccents lists the layers / cells whose fill is a plain scheme
// accent (no tint): the solid accent blocks.
func cycleRingSolidAccents(ring *jsonschema.GridCellInput, prefix string) []string {
	var out []string
	for _, l := range cycleRingLayers(ring, prefix) {
		var name string
		if json.Unmarshal(l.Shape.Fill, &name) == nil && strings.HasPrefix(name, "accent") {
			out = append(out, l.Name+"="+name)
		}
	}
	return out
}

// By default no segment is a solid accent; a highlighted phase is the only
// one, on every style, and its badge flips to the page colour.
func TestCycleRingHighlightIsTheOnlySolidAccent(t *testing.T) {
	for _, style := range cycleRingStyles {
		for _, mode := range []string{"", "alternate", "progressive"} {
			ovr := &CycleRingOverrides{Style: style, TextOverrides: TextOverrides{CellAccentMode: mode}}
			ring := cycleRingCell(t, cycleRingExpand(t, ExpandContext{}, cycleRingValues(8), ovr))
			if got := cycleRingSolidAccents(ring, "segment-"); len(got) != 0 {
				t.Errorf("style %s mode %q: solid accent segments without a highlight: %v", style, mode, got)
			}
			v := cycleRingValues(8)
			v.Phases[5].Highlight = true
			ring = cycleRingCell(t, cycleRingExpand(t, ExpandContext{}, v, ovr))
			got := cycleRingSolidAccents(ring, "segment-")
			if len(got) != 1 || !strings.HasPrefix(got[0], "segment-6=") {
				t.Errorf("style %s mode %q: solid accent segments = %v, want only segment-6", style, mode, got)
			}
			badges := cycleRingLayers(ring, "badge-")
			if string(badges[5].Shape.Fill) != `"lt1"` {
				t.Errorf("style %s mode %q: the highlight's badge fill = %s, want lt1", style, mode, badges[5].Shape.Fill)
			}
			if solid := cycleRingSolidAccents(ring, "badge-"); len(solid) != 7 {
				t.Errorf("style %s mode %q: %d accent badges, want 7", style, mode, len(solid))
			}
		}
	}
}

func TestCycleRing_ExpandStyling(t *testing.T) {
	// DefaultAccent: the template's primary fill, not a hard-coded accent1.
	// A theme whose accent1 cannot carry white text hands over its next slot.
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1F2937"}, {Name: "lt2", RGB: "#EEEEEE"},
		{Name: "accent1", RGB: "#FFD54F"}, {Name: "accent2", RGB: "#0B3D91"},
	}}}
	raw, _ := json.Marshal(cycleRingExpand(t, ctx, cycleRingValues(4), nil))
	if want := ctx.DefaultAccent(); want != "accent2" || !strings.Contains(string(raw), `"accent2"`) || strings.Contains(string(raw), `"accent1"`) {
		t.Errorf("default accent %s not used throughout", want)
	}

	// An accent override reaches badges, numerals and the highlight.
	v := cycleRingValues(4)
	v.Phases[2].Highlight = true
	raw, _ = json.Marshal(cycleRingExpand(t, ExpandContext{}, v, &CycleRingOverrides{TextOverrides: TextOverrides{Accent: "accent3", HeaderSize: 16, BodySize: 13}}))
	out := string(raw)
	for _, want := range []string{`"accent3"`, `"size":16`, `"size":13`, `"Awareness"`, `"Referrals bring 4 in 10 leads"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expansion missing %s", want)
		}
	}
	if strings.Contains(out, `"accent1"`) {
		t.Error("accent override not applied everywhere")
	}

	// cell_accent_mode x base accent: uniform keeps neutral segments and one
	// badge colour; alternate uses two accents; progressive walks them. The
	// non-uniform modes tint each segment with its own accent.
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			ovr := &CycleRingOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}
			ring := cycleRingCell(t, cycleRingExpand(t, ExpandContext{}, cycleRingValues(6), ovr))
			badgeFills := map[string]bool{}
			for i, b := range cycleRingLayers(ring, "badge-") {
				var fill string
				_ = json.Unmarshal(b.Shape.Fill, &fill)
				badgeFills[fill] = true
				if want := ResolveCellAccent(base, i, mode); fill != want {
					t.Errorf("%s/%s badge %d fill = %s, want %s", base, mode, i+1, fill, want)
				}
			}
			wantDistinct := map[string]int{"uniform": 1, "alternate": 2, "progressive": 6}[mode]
			if len(badgeFills) != wantDistinct {
				t.Errorf("%s/%s: %d badge accents, want %d", base, mode, len(badgeFills), wantDistinct)
			}
			for i, s := range cycleRingLayers(ring, "segment-") {
				fill := string(s.Shape.Fill)
				if mode == "uniform" {
					if !strings.Contains(fill, `"dk1"`) || !strings.Contains(fill, `"lumMod":16000`) {
						t.Errorf("%s/uniform segment %d fill = %s, want the dk1 16%% neutral", base, i+1, fill)
					}
					continue
				}
				if want := ResolveCellAccent(base, i, mode); !strings.Contains(fill, `"`+want+`"`) || !strings.Contains(fill, `"lumMod"`) {
					t.Errorf("%s/%s segment %d fill = %s, want a tint of %s", base, mode, i+1, fill, want)
				}
			}
		}
	}

	// semantic_accent resolves through the theme.
	ctx = ExpandContext{}
	ctx.Theme.SemanticAccents = map[string]string{"positive": "accent6"}
	raw, _ = json.Marshal(cycleRingExpand(t, ctx, cycleRingValues(4), &CycleRingOverrides{TextOverrides: TextOverrides{SemanticAccent: "positive"}}))
	if !strings.Contains(string(raw), `"accent6"`) {
		t.Error("semantic_accent not resolved")
	}
}

// Ink is measured against the fill it sits on: a light accent gets dark
// numerals in its badges, and a numeral beside a label that the accent cannot
// carry on the page takes a theme ink.
func TestCycleRing_InkIsMeasured(t *testing.T) {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1F2937"}, {Name: "lt2", RGB: "#EEEEEE"},
		{Name: "accent1", RGB: "#FFD54F"}, {Name: "accent2", RGB: "#0B3D91"},
	}}}
	for accent, wantBadgeInk := range map[string]string{"accent1": "dk2", "accent2": "lt1"} {
		v := cycleRingValues(4)
		v.Phases[1].Highlight = true
		grid := cycleRingExpand(t, ctx, v, &CycleRingOverrides{TextOverrides: TextOverrides{Accent: accent}})
		badges := cycleRingLayers(cycleRingCell(t, grid), "badge-")
		if !strings.Contains(string(badges[0].Shape.Text), `"color":"`+wantBadgeInk+`"`) {
			t.Errorf("%s badge ink: %s, want %s", accent, badges[0].Shape.Text, wantBadgeInk)
		}
		// The highlight's badge is the page colour: its numeral is the accent
		// only where the accent reads on white.
		wantOnPage := accent
		if accent == "accent1" {
			wantOnPage = "dk2"
		}
		if !strings.Contains(string(badges[1].Shape.Text), `"color":"`+wantOnPage+`"`) {
			t.Errorf("%s highlight badge ink: %s, want %s", accent, badges[1].Shape.Text, wantOnPage)
		}
		raw, _ := json.Marshal(grid)
		if !strings.Contains(string(raw), `"content":"1","size":14,"bold":true,"color":"`+wantOnPage+`"`) {
			t.Errorf("%s: the numeral beside label 1 is not in %s", accent, wantOnPage)
		}
	}
}

func TestCycleRing_Center(t *testing.T) {
	v := cycleRingValues(4)
	v.Center = &CycleRingCenter{Label: "Customer lifecycle", Sublabel: "B2B software"}
	ring := cycleRingCell(t, cycleRingExpand(t, cycleRingCtx(828, 349), v, nil))
	centre := ring.Layers[len(ring.Layers)-1]
	if centre.Name != "centre" || centre.Shape.Geometry != "rect" || string(centre.Shape.Fill) != `"none"` {
		t.Fatalf("last layer = %s %s fill %s, want the unfilled centre text", centre.Name, centre.Shape.Geometry, centre.Shape.Fill)
	}
	// Inside the hole (diameter 0.6 of the square), centred.
	if f := centre.Frame; math.Abs(f.X+f.W/2-0.5) > 1e-6 || math.Abs(f.Y+f.H/2-0.5) > 1e-6 || f.W > 0.6 || f.W < 0.4 {
		t.Errorf("centre frame %+v is not centred in the hole", f)
	}
	for _, want := range []string{"Customer lifecycle", "B2B software", `"bold":true`, `"alpha":70`} {
		if !strings.Contains(string(centre.Shape.Text), want) {
			t.Errorf("centre text missing %s: %s", want, centre.Shape.Text)
		}
	}
	// The label shrinks towards 12pt before it warns, and never under it.
	lay, _ := cycleRingMeasure(cycleRingCtx(828, 349), v, &CycleRingOverrides{})
	if lay.centrePt < shapegrid.MinTextSizePt || lay.centrePt > ringCentreMaxPt || !lay.centreFits {
		t.Errorf("centre label size %.0f fits=%v", lay.centrePt, lay.centreFits)
	}
	// A sublabel alone is drawn too.
	v.Center = &CycleRingCenter{Sublabel: "Every quarter"}
	ring = cycleRingCell(t, cycleRingExpand(t, ExpandContext{}, v, nil))
	if ring.Layers[len(ring.Layers)-1].Name != "centre" {
		t.Error("a sublabel without a label is dropped")
	}
	// No centre: no layer.
	ring = cycleRingCell(t, cycleRingExpand(t, ExpandContext{}, cycleRingValues(4), nil))
	if ring.Layers[len(ring.Layers)-1].Name == "centre" {
		t.Error("centre layer drawn without center values")
	}
}

// A narrow content area (a 50% compose segment) takes the legend layout: the
// ring on the left, one numbered list beside it. Descriptions are left off,
// with a warning, when the list cannot hold them.
func TestCycleRing_LegendFallback(t *testing.T) {
	for _, tc := range []struct {
		w, h   float64
		legend bool
	}{{828, 349, false}, {687, 294, false}, {490, 341, false}, {449, 341, true}, {408, 341, true}, {300, 200, true}} {
		lay, err := cycleRingMeasure(cycleRingCtx(tc.w, tc.h), cycleRingValues(5), &CycleRingOverrides{})
		if err != nil {
			t.Fatal(err)
		}
		if lay.legend != tc.legend {
			t.Errorf("%.0f x %.0f: legend = %v, want %v", tc.w, tc.h, lay.legend, tc.legend)
		}
		if lay.side < math.Min(ringMinSidePt, tc.h)-0.5 {
			t.Errorf("%.0f x %.0f: ring side %.0fpt", tc.w, tc.h, lay.side)
		}
	}
	// overrides.labels forces either layout.
	lay, _ := cycleRingMeasure(cycleRingCtx(828, 349), cycleRingValues(5), &CycleRingOverrides{Labels: cycleRingLabelsLegend})
	if !lay.legend || lay.x0 != 0 {
		t.Errorf("labels legend on a full slide: legend=%v x0=%.0f", lay.legend, lay.x0)
	}
	for i := 1; i < len(lay.rows); i++ {
		if lay.rows[i].y0 < lay.rows[i-1].y1 || lay.rows[i].side != ringSideRight {
			t.Errorf("legend row %d overlaps the one above or is not right of the ring: %+v", i, lay.rows[i])
		}
	}

	p := &cycleRing{}
	narrow := cycleRingCtx(408, 250)
	v := cycleRingValues(8)
	lay, _ = cycleRingMeasure(narrow, v, &CycleRingOverrides{})
	if lay.showDesc {
		t.Fatal("eight described phases cannot fit a 250pt legend: descriptions must be left off")
	}
	warnings := p.PostExpandWarnings(narrow, v, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], ErrCodeBodyTooLong) || !strings.Contains(warnings[0], "phases[].description is left off") {
		t.Errorf("legend warning = %v", warnings)
	}
	raw, _ := json.Marshal(cycleRingExpand(t, narrow, v, nil))
	if strings.Contains(string(raw), "Referrals bring") || !strings.Contains(string(raw), `"Awareness"`) {
		t.Error("the legend must keep the labels and drop the descriptions")
	}
	// Without descriptions the same legend is clean.
	for i := range v.Phases {
		v.Phases[i].Description = ""
	}
	if w := p.PostExpandWarnings(narrow, v, nil); len(w) != 0 {
		t.Errorf("label-only legend warned: %v", w)
	}
}

func TestCycleRing_PostExpandWarnings(t *testing.T) {
	p := &cycleRing{}
	for _, body := range cycleRingBodies {
		for n := cycleRingMinPhases; n <= cycleRingMaxPhases; n++ {
			if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), cycleRingValues(n), nil); len(w) != 0 {
				t.Errorf("%s n=%d: realistic content warned: %v", body.name, n, w)
			}
		}
	}
	// Outside its range, or with the wrong type, it reports nothing (Validate does).
	if w := p.PostExpandWarnings(ExpandContext{}, cycleRingValues(3), nil); w != nil {
		t.Errorf("3 phases: %v", w)
	}
	if w := p.PostExpandWarnings(ExpandContext{}, &StateShiftHubValues{}, nil); w != nil {
		t.Errorf("wrong type: %v", w)
	}

	// A centre label that cannot fit the hole of a small ring.
	v := cycleRingValues(4)
	v.Center = &CycleRingCenter{Label: "Institutionalisation", Sublabel: "of continuous improvement"}
	w := p.PostExpandWarnings(cycleRingCtx(408, 200), v, nil)
	found := false
	for _, msg := range w {
		found = found || (strings.HasPrefix(msg, ErrCodeBodyTooLong) && strings.Contains(msg, "center"))
	}
	if !found {
		t.Errorf("centre warning missing: %v", w)
	}

	// A row that cannot hold its text in a short area names the phase.
	v = cycleRingValues(8)
	for i := range v.Phases {
		v.Phases[i].Description = "Buyers compare three vendors and two partners"
	}
	w = p.PostExpandWarnings(cycleRingCtx(700, 150), v, nil)
	if len(w) == 0 || !strings.Contains(strings.Join(w, "\n"), "phases[0].description needs") || !strings.Contains(w[0], "label row") {
		t.Errorf("short-area warnings = %v", w)
	}
}

func TestCycleRing_Golden(t *testing.T) {
	p := &cycleRing{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "cycle-ring", "default.golden.json"))
}

func TestCycleRing_ExemplarIsClean(t *testing.T) {
	p := &cycleRing{}
	if err := p.Validate(p.ExemplarValues(), nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, body := range cycleRingBodies {
		if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), p.ExemplarValues(), nil); len(w) != 0 {
			t.Errorf("%s: exemplar warns: %v", body.name, w)
		}
	}
}

func TestCycleRing_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"continuous improvement cycle", &ContentHints{ItemCount: 4}, "cycle-ring"},
		{"PDCA", nil, "cycle-ring"},
		{"customer lifecycle", &ContentHints{ItemCount: 6}, "cycle-ring"},
		{"operating rhythm", &ContentHints{ItemCount: 5}, "cycle-ring"},
		{"flywheel", nil, "cycle-ring"},
		// A sequence that does not loop back still belongs to the flow family.
		{"approval workflow with a decision point", nil, "process-flow"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}
