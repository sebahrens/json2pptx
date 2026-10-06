package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// cycleIntakeValues is nIntake steps feeding nLoop phases of realistic copy
// inside every combination's budget (descriptions on the intake only, so the
// same values are clean with eight phases on the shortest body).
func cycleIntakeValues(nIntake, nLoop int) *CycleIntakeValues {
	steps := []CycleIntakeStep{
		{Label: "Source deals", Description: "Partners log every lead"},
		{Label: "Screen", Description: "Fit with the fund thesis"},
		{Label: "Approve", Description: "Committee signs the ticket"},
		{Label: "Extra", Description: "One too many"},
	}
	phases := []CycleIntakePhase{
		{Label: "Collect the numbers"},
		{Label: "Score performance"},
		{Label: "Review the risks"},
		{Label: "Rank the portfolio"},
		{Label: "Rebalance capital"},
		{Label: "Support the laggards"},
		{Label: "Prepare the exits"},
		{Label: "Report to investors"},
		{Label: "Extra"},
	}
	return &CycleIntakeValues{
		Intake: append([]CycleIntakeStep(nil), steps[:nIntake]...),
		Loop:   append([]CycleIntakePhase(nil), phases[:nLoop]...),
	}
}

func cycleIntakeExpand(t *testing.T, ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	p := &cycleIntake{}
	if err := p.Validate(v, overrides, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

// cycleIntakeCells sorts an expanded grid's cells by what they draw.
type cycleIntakeCells struct {
	ring   *jsonschema.GridCellInput
	lane   []*jsonschema.GridCellInput // homePlate + chevrons, in order
	entry  *jsonschema.GridCellInput
	texts  []*jsonschema.GridCellInput // unfilled text cells: descriptions, numerals, labels
	others int
}

func cycleIntakeSort(t *testing.T, grid *jsonschema.ShapeGridInput) cycleIntakeCells {
	t.Helper()
	var out cycleIntakeCells
	for _, r := range grid.Rows {
		for _, c := range r.Cells {
			switch {
			case c == nil || (c.Shape == nil && len(c.Layers) == 0):
			case len(c.Layers) > 0:
				if out.ring != nil {
					t.Fatal("more than one cell carries layers")
				}
				out.ring = c
			case c.Shape.Geometry == "homePlate" || c.Shape.Geometry == "chevron":
				out.lane = append(out.lane, c)
			case c.Shape.Geometry == "rightArrow" || c.Shape.Geometry == "downArrow":
				if out.entry != nil {
					t.Fatal("more than one entry arrow")
				}
				out.entry = c
			case c.Shape.Geometry == "rect" && string(c.Shape.Fill) == `"none"`:
				out.texts = append(out.texts, c)
			default:
				out.others++
			}
		}
	}
	if out.ring == nil || out.entry == nil {
		t.Fatalf("ring cell %v, entry arrow %v", out.ring != nil, out.entry != nil)
	}
	return out
}

func cycleIntakeTextOf(t *testing.T, raw json.RawMessage) (align string, paras []ringPara) {
	t.Helper()
	var obj ringTextObj
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("text: %v", err)
	}
	return obj.Align, obj.Paragraphs
}

// ---------------------------------------------------------------------------

func TestCycleIntake_Metadata(t *testing.T) {
	p, ok := Default().Get("cycle-intake")
	if !ok {
		t.Fatal("cycle-intake not registered")
	}
	if p.Name() != "cycle-intake" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" || p.Description() == "" || p.CellsHint() == "" {
		t.Error("UseWhen/NotWhen/Description/CellsHint must be non-empty")
	}
	for _, sibling := range []string{"cycle-ring", "cycle-nodes", "process-flow", "swimlane", "value-chain", "cycle-figure-eight"} {
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
	if PatternMotif("cycle-intake") != MotifDiagram {
		t.Errorf("motif = %q, want diagram", PatternMotif("cycle-intake"))
	}
}

func TestCycleIntake_SchemaIsValidJSON(t *testing.T) {
	data, err := json.Marshal((&cycleIntake{}).Schema())
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
	for _, key := range []string{"intake", "loop", "label", "description", "highlight", "center", "loop_style", "cell_accent_mode", "header_size", "body_size", "semantic_accent"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("schema missing %q", key)
		}
	}
	// The schema states the per-combination budgets the pattern warns at.
	for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
		for nL := ringMinItems; nL < ringMaxItems; nL++ {
			for _, b := range []int{cycleIntakeLabelBudget(nI, nL), cycleIntakeDescBudget(nI, nL)} {
				if !strings.Contains(string(data), fmt.Sprintf("%d", b)) {
					t.Errorf("schema does not state the %d+%d budget %d", nI, nL, b)
				}
			}
		}
	}
}

func TestCycleIntake_Validate(t *testing.T) {
	p := &cycleIntake{}
	for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
		for nL := ringMinItems; nL <= ringMaxItems; nL++ {
			if err := p.Validate(cycleIntakeValues(nI, nL), nil, nil); err != nil {
				t.Errorf("%d+%d: unexpected error: %v", nI, nL, err)
			}
		}
	}
	var ve *ValidationError
	if err := p.Validate(cycleIntakeValues(0, 4), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMinItems || !strings.Contains(err.Error(), "cycle-ring") {
		t.Errorf("no intake: want min_items with the cycle-ring hint, got %v", err)
	}
	if err := p.Validate(cycleIntakeValues(4, 4), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxItems ||
		!strings.Contains(err.Error(), "intake") || !strings.Contains(err.Error(), "process-flow") {
		t.Errorf("4 intake steps: want max_items naming the 3-step limit and process-flow, got %v", err)
	}
	if err := p.Validate(cycleIntakeValues(2, 2), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMinItems || !strings.Contains(err.Error(), "process-flow") {
		t.Errorf("2 phases: want min_items with the process-flow hint, got %v", err)
	}
	if err := p.Validate(cycleIntakeValues(2, 9), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxItems || !strings.Contains(err.Error(), "cycle-figure-eight") {
		t.Errorf("9 phases: want max_items with the cycle-figure-eight hint, got %v", err)
	}

	v := cycleIntakeValues(2, 4)
	v.Intake[1].Label = " "
	v.Loop[2].Label = ""
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "intake[1].label") || !strings.Contains(err.Error(), "loop[2].label") {
		t.Errorf("want intake[1].label and loop[2].label required, got %v", err)
	}

	// Budgets count characters, not bytes.
	v = cycleIntakeValues(2, 4)
	v.Intake[0].Label = strings.Repeat("ü", cycleIntakeStepLabelMax)
	v.Intake[0].Description = strings.Repeat("é", cycleIntakeStepDescMax)
	v.Loop[0].Label = strings.Repeat("ö", cycleIntakeLabelMax)
	v.Loop[0].Description = strings.Repeat("ß", cycleIntakeDescMax)
	v.Center = &CycleIntakeCenter{Label: strings.Repeat("ä", cycleIntakeCenterMax)}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budgets: %v", err)
	}
	v.Intake[0].Label += "x"
	v.Intake[0].Description += "x"
	v.Loop[0].Label += "x"
	v.Loop[0].Description += "x"
	v.Center.Label += "x"
	err := p.Validate(v, nil, nil)
	for _, path := range []string{"intake[0].label", "intake[0].description", "loop[0].label", "loop[0].description", "center.label"} {
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("want max_length for %s, got %v", path, err)
		}
	}
	if err := p.Validate(&CycleIntakeValues{Intake: cycleIntakeValues(1, 3).Intake, Loop: cycleIntakeValues(1, 3).Loop, Center: &CycleIntakeCenter{}}, nil, nil); err == nil || !strings.Contains(err.Error(), "center.label") {
		t.Errorf("empty centre: want center.label required, got %v", err)
	}

	v = cycleIntakeValues(2, 5)
	v.Loop[1].Highlight, v.Loop[3].Highlight = true, true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one phase") {
		t.Errorf("two highlights: got %v", err)
	}

	if err := p.Validate(cycleIntakeValues(2, 4), &CycleIntakeOverrides{LoopStyle: "arrows"}, nil); err == nil || !errors.As(err, &ve) || ve.Path != "overrides.loop_style" {
		t.Errorf("bad loop_style: got %v", err)
	}
	if err := p.Validate(cycleIntakeValues(2, 4), &CycleIntakeOverrides{TextOverrides: TextOverrides{CellAccentMode: "rainbow"}}, nil); err == nil {
		t.Error("bad cell_accent_mode accepted")
	}
	if err := p.Validate(cycleIntakeValues(2, 4), nil, map[int]any{0: struct{}{}}); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeUnknownKey {
		t.Errorf("cell_overrides: want unknown_key, got %v", err)
	}
	if err := p.Validate(&CycleRingValues{}, nil, nil); err == nil {
		t.Error("wrong values type accepted")
	}
	if _, err := p.Expand(ExpandContext{}, cycleIntakeValues(4, 4), nil, nil); err == nil {
		t.Error("Expand accepted 4 intake steps")
	}
}

// Side by side: a pentagon and chevrons on the ring's centreline, the entry
// arrow, the loop, then one numbered list right of the ring in phase order.
func TestCycleIntake_ExpandLayout(t *testing.T) {
	for _, body := range cycleRingBodies {
		t.Run(body.name, func(t *testing.T) {
			ctx := cycleRingCtx(body.w, body.h)
			v := cycleIntakeValues(3, 8)
			grid := cycleIntakeExpand(t, ctx, v, nil)
			cells := cycleIntakeSort(t, grid)
			if cells.ring.Fit != "contain" {
				t.Errorf("ring cell fit = %q, want contain", cells.ring.Fit)
			}
			if len(cells.lane) != 3 || cells.lane[0].Shape.Geometry != "homePlate" || cells.lane[1].Shape.Geometry != "chevron" || cells.lane[2].Shape.Geometry != "chevron" {
				t.Fatalf("lane = %d cells, want a pentagon and two chevrons", len(cells.lane))
			}
			if cells.lane[0].BleedLeft != 0 || cells.lane[1].BleedLeft <= 0 || cells.lane[1].BleedLeft != cells.lane[2].BleedLeft {
				t.Errorf("chevron tails must tuck under the point before them: bleed %v %v %v", cells.lane[0].BleedLeft, cells.lane[1].BleedLeft, cells.lane[2].BleedLeft)
			}
			for i, c := range cells.lane {
				if c.Shape.Adjustments["adj"] <= 0 {
					t.Errorf("lane cell %d has no point depth", i)
				}
				_, paras := cycleIntakeTextOf(t, c.Shape.Text)
				if len(paras) != 1 || paras[0].Content != v.Intake[i].Label || paras[0].Size < 12 || !paras[0].Bold {
					t.Errorf("lane cell %d text = %+v", i, paras)
				}
			}
			if cells.entry.Shape.Geometry != "rightArrow" {
				t.Errorf("entry arrow = %s, want rightArrow", cells.entry.Shape.Geometry)
			}
			if got := len(cycleRingLayers(cells.ring, "segment-")); got != 8 {
				t.Errorf("%d segments, want 8", got)
			}
			if got := len(cycleRingLayers(cells.ring, "badge-")); got != 8 {
				t.Errorf("%d badges, want 8", got)
			}
			// 3 intake descriptions, then per phase a numeral and a label.
			if len(cells.texts) != 3+2*8 || cells.others != 0 {
				t.Fatalf("%d text cells (%d other), want %d", len(cells.texts), cells.others, 3+2*8)
			}
			lay, err := cycleIntakeMeasure(ctx, v, &CycleIntakeOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			if lay.stacked {
				t.Fatal("a full-width body must lay out side by side")
			}
			// The lane, the arrow and the ring share the horizontal centreline.
			for name, box := range map[string]cycleIntakeBox{"lane": lay.lane, "entry": lay.entry} {
				if mid := (box.y0 + box.y1) / 2; math.Abs(mid-(lay.ringY+lay.side/2)) > 0.01 {
					t.Errorf("%s centre %.1f is off the ring's centreline %.1f", name, mid, lay.ringY+lay.side/2)
				}
			}
			if math.Abs(lay.entry.x1-lay.ringX) > 0.01 || lay.entry.x0 < lay.lane.x1 {
				t.Errorf("entry arrow %.1f-%.1f must run from the lane (%.1f) to the ring (%.1f)", lay.entry.x0, lay.entry.x1, lay.lane.x1, lay.ringX)
			}
			// The list is right of the ring, in phase order, one pitch.
			prev := -1.0
			for i, row := range lay.list.rows {
				if lay.listX0 < lay.ringX+lay.side || row.y0 <= prev {
					t.Errorf("list row %d at x %.1f y %.1f is not right of the ring / below row %d", i, lay.listX0, row.y0, i-1)
				}
				if i > 0 && math.Abs((row.y1-row.y0)-(lay.list.rows[0].y1-lay.list.rows[0].y0)) > 0.01 {
					t.Errorf("list row %d is %.1fpt tall, row 0 %.1fpt: the list keeps one pitch", i, row.y1-row.y0, lay.list.rows[0].y1-lay.list.rows[0].y0)
				}
				prev = row.y0
			}
			if lay.listX0+ringNumberColPt+lay.list.labelW > body.w+0.01 {
				t.Errorf("list runs past the body: %.1f > %.1f", lay.listX0+ringNumberColPt+lay.list.labelW, body.w)
			}
		})
	}
}

// Phase 1 begins where the intake arrow lands: the first segment's blockArc
// starts just past 9 o'clock and the loop runs clockwise; with loop_style
// nodes the first node sits AT 9 o'clock.
func TestCycleIntakeLoopStartsAtNineOClock(t *testing.T) {
	ctx := cycleRingCtx(828, 349)
	for n := ringMinItems; n <= ringMaxItems; n++ {
		ring := cycleIntakeSort(t, cycleIntakeExpand(t, ctx, cycleIntakeValues(2, n), nil)).ring
		segs := cycleRingLayers(ring, "segment-")
		// The seam gap (6 degrees) is centred on 180: segment 1 runs clockwise
		// from 183, and the last segment ends at 177.
		if got := segs[0].Shape.Adjustments["adj1"]; got != 183*60000 {
			t.Errorf("n=%d: segment-1 adj1 = %d, want %d (183 degrees)", n, got, 183*60000)
		}
		if got := segs[n-1].Shape.Adjustments["adj2"]; got != 177*60000 {
			t.Errorf("n=%d: segment-%d adj2 = %d, want %d (177 degrees)", n, n, got, 177*60000)
		}
		// Clockwise from 9 o'clock goes over the top: segment 1 ends before 12.
		if end := segs[0].Shape.Adjustments["adj2"]; end <= 183*60000 || (n >= 4 && end > 270*60000) {
			t.Errorf("n=%d: segment-1 ends at %d: it must run clockwise from 9 o'clock towards 12", n, end)
		}

		nodes := cycleRingLayers(cycleIntakeSort(t, cycleIntakeExpand(t, ctx, cycleIntakeValues(2, n), &CycleIntakeOverrides{LoopStyle: "nodes"})).ring, "node-")
		if len(nodes) != n {
			t.Fatalf("n=%d: %d nodes", n, len(nodes))
		}
		f := nodes[0].Frame
		if f.X > 1e-6 || math.Abs(f.Y+f.H/2-0.5) > 1e-6 {
			t.Errorf("n=%d: node-1 frame %+v is not at 9 o'clock on the square's left edge", n, f)
		}
		if n >= 4 && nodes[1].Frame.Y >= f.Y {
			t.Errorf("n=%d: node-2 must sit above node-1 (clockwise from 9 o'clock)", n)
		}
	}
}

// Every combination in the three bodies resolves without overlap, inside the
// area, and the entry arrow's tip touches the ring.
func TestCycleIntakeNoOverlapForEveryCombination(t *testing.T) {
	for _, body := range cycleRingBodies {
		for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
			for nL := ringMinItems; nL <= ringMaxItems; nL++ {
				for _, style := range cycleIntakeStyles {
					t.Run(fmt.Sprintf("%s/%d+%d/%s", body.name, nI, nL, style), func(t *testing.T) {
						grid := cycleIntakeExpand(t, cycleRingCtx(body.w, body.h), cycleIntakeValues(nI, nL), &CycleIntakeOverrides{LoopStyle: style})
						cycleIntakeAssertNoOverlap(t, grid, body.w, body.h, false)
					})
				}
			}
		}
	}
}

// cycleIntakeAssertNoOverlap resolves the grid in a w × h area and checks that
// no lattice cell overlaps another or the ring, that everything stays inside
// the area, that the ring is square and that the entry arrow touches it.
func cycleIntakeAssertNoOverlap(t *testing.T, grid *jsonschema.ShapeGridInput, w, h float64, down bool) {
	t.Helper()
	res := cycleNodesResolveAt(t, grid, w, h)
	var cells []shapegrid.ResolvedCell
	var ring, entry pptx.RectEmu
	for _, c := range res.Cells {
		if c.ShapeSpec == nil {
			continue
		}
		if c.Bounds.X < -2 || c.Bounds.Y < -2 || c.Bounds.X+c.Bounds.CX > int64(w*12700)+2 || c.Bounds.Y+c.Bounds.CY > int64(h*12700)+2 {
			t.Errorf("%s leaves the %.0f x %.0fpt area: %+v", c.ShapeSpec.Geometry, w, h, c.Bounds)
		}
		if c.Layer {
			// Every layer shares the ring cell: its bounds are the square the
			// loop is drawn in (segments and node 1 reach its edge).
			ring = c.CellBounds
			continue
		}
		if g := c.ShapeSpec.Geometry; g == "rightArrow" || g == "downArrow" {
			entry = c.Bounds
		}
		cells = append(cells, c)
	}
	if d := ring.CX - ring.CY; d < -12700 || d > 12700 {
		t.Errorf("the ring is not round: %d x %d EMU", ring.CX, ring.CY)
	}
	if float64(ring.CX)/12700 < ringMinSidePt-1 {
		t.Errorf("the ring is %.0fpt across, under the %.0fpt floor", float64(ring.CX)/12700, ringMinSidePt)
	}
	gap := float64(ring.X-(entry.X+entry.CX)) / 12700
	if down {
		gap = float64(ring.Y-(entry.Y+entry.CY)) / 12700
	}
	if math.Abs(gap) > 1 {
		t.Errorf("the entry arrow ends %.2fpt from the ring; it must touch it within 1pt", gap)
	}
	for i, a := range cells {
		// A chevron's tail reaches under the point before it by design.
		if cycleNodesRectsOverlap(a.Bounds, ring) {
			t.Errorf("%s cell %+v overlaps the ring %+v", a.ShapeSpec.Geometry, a.Bounds, ring)
		}
		for _, b := range cells[i+1:] {
			if a.ShapeSpec.Geometry == "chevron" || b.ShapeSpec.Geometry == "chevron" {
				if (a.ShapeSpec.Geometry == "homePlate" || a.ShapeSpec.Geometry == "chevron") && (b.ShapeSpec.Geometry == "homePlate" || b.ShapeSpec.Geometry == "chevron") {
					continue
				}
			}
			if cycleNodesRectsOverlap(a.Bounds, b.Bounds) {
				t.Errorf("cells overlap: %s %+v and %s %+v", a.ShapeSpec.Geometry, a.Bounds, b.ShapeSpec.Geometry, b.Bounds)
			}
		}
	}
}

// A content area too narrow for lane, ring and list side by side (a compose
// half) stacks them: the lane on top, a down arrow into 12 o'clock — where
// phase 1 then begins — and the list left of the ring, turned towards it.
// (The bead's "legend fallback when the left labels do not fit" does not
// arise: every loop label is in the one list at every width.)
func TestCycleIntakeStacksInANarrowArea(t *testing.T) {
	for _, area := range []struct{ w, h float64 }{{337, 294}, {408, 349}, {443, 360}} {
		for nL := ringMinItems; nL <= ringMaxItems; nL++ {
			for _, style := range cycleIntakeStyles {
				t.Run(fmt.Sprintf("%.0fx%.0f/3+%d/%s", area.w, area.h, nL, style), func(t *testing.T) {
					ctx := cycleRingCtx(area.w, area.h)
					v := cycleIntakeValues(3, nL)
					ovr := &CycleIntakeOverrides{LoopStyle: style}
					lay, err := cycleIntakeMeasure(ctx, v, ovr)
					if err != nil {
						t.Fatal(err)
					}
					if !lay.stacked || !lay.entryDown || lay.showDesc {
						t.Fatalf("stacked=%v entryDown=%v showDesc=%v, want the stacked layout without intake descriptions", lay.stacked, lay.entryDown, lay.showDesc)
					}
					if lay.fit.colWPt < cycleIntakeMinStepPt {
						t.Errorf("intake arrows are %.0fpt wide, under the %.0fpt floor", lay.fit.colWPt, cycleIntakeMinStepPt)
					}
					// The last intake arrow stands above the ring's centre.
					last := lay.lane.x1 - lay.fit.colWPt/2
					if cx := lay.ringX + lay.side/2; math.Abs(cx-last) > 0.5 || math.Abs((lay.entry.x0+lay.entry.x1)/2-cx) > 0.01 {
						t.Errorf("ring centre %.1f, last arrow centre %.1f, entry arrow %.1f-%.1f", cx, last, lay.entry.x0, lay.entry.x1)
					}
					if lay.lane.y1 > lay.entry.y0 || math.Abs(lay.entry.y1-lay.ringY) > 0.01 {
						t.Errorf("lane %.1f, entry %.1f-%.1f, ring top %.1f: the arrow runs from the lane down to the ring", lay.lane.y1, lay.entry.y0, lay.entry.y1, lay.ringY)
					}
					if lay.listX0+lay.list.labelW+ringNumberColPt > lay.ringX+0.01 {
						t.Errorf("the list (to %.1f) must end left of the ring (%.1f)", lay.listX0+lay.list.labelW+ringNumberColPt, lay.ringX)
					}
					grid := cycleIntakeExpand(t, ctx, v, ovr)
					cells := cycleIntakeSort(t, grid)
					if cells.entry.Shape.Geometry != "downArrow" {
						t.Errorf("entry arrow = %s, want downArrow", cells.entry.Shape.Geometry)
					}
					if len(cells.texts) != 2*nL {
						t.Errorf("%d text cells, want %d (no intake descriptions)", len(cells.texts), 2*nL)
					}
					if style == cycleIntakeStyleSegments {
						if got := cycleRingLayers(cells.ring, "segment-")[0].Shape.Adjustments["adj1"]; got != 273*60000 {
							t.Errorf("segment-1 adj1 = %d, want %d (just past 12 o'clock)", got, 273*60000)
						}
					}
					cycleIntakeAssertNoOverlap(t, grid, area.w, area.h, true)
					// Labels inside the stacked budget (one line each) leave only
					// the left-off intake descriptions to report.
					w := (&cycleIntake{}).PostExpandWarnings(ctx, v, ovr)
					if len(w) != 1 || !strings.Contains(w[0], "intake[].description is left off") {
						t.Errorf("warnings = %v, want the left-off intake descriptions only", w)
					}
				})
			}
		}
	}
	// One intake step still fits side by side in a 60% segment.
	if lay, err := cycleIntakeMeasure(cycleRingCtx(497, 349), cycleIntakeValues(1, 5), &CycleIntakeOverrides{}); err != nil || lay.stacked {
		t.Errorf("1 intake step in 497pt: stacked=%v err=%v, want side by side", lay.stacked, err)
	}
}

func cycleIntakeFillOf(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	return string(raw)
}

// The intake stays neutral, the entry arrow is the accent cue, and the loop
// spends at most one solid accent on the highlighted phase.
func TestCycleIntake_ExpandStyling(t *testing.T) {
	ctx := cycleRingCtx(828, 349)
	v := cycleIntakeValues(3, 6)

	cells := cycleIntakeSort(t, cycleIntakeExpand(t, ctx, v, nil))
	neutral := string(neutralTone(ProcessFlowStepTintPct).fillJSON())
	for i, c := range cells.lane {
		if got := cycleIntakeFillOf(t, c.Shape.Fill); got != neutral {
			t.Errorf("intake step %d fill = %s, want the neutral tint %s", i, got, neutral)
		}
		if string(c.Shape.Line) != string(noLine) {
			t.Errorf("intake step %d carries an outline", i)
		}
	}
	if got := string(cells.entry.Shape.Fill); got != `"accent1"` || string(cells.entry.Shape.Line) != string(noLine) {
		t.Errorf("entry arrow fill = %s line = %s, want the solid default accent without an outline", got, cells.entry.Shape.Line)
	}
	if solid := cycleRingSolidAccents(cells.ring, "segment-"); len(solid) != 0 {
		t.Errorf("no highlight: solid accent segments %v", solid)
	}

	v.Loop[3].Highlight = true
	for _, tc := range []struct {
		name   string
		ctx    ExpandContext
		ovr    *CycleIntakeOverrides
		accent string
	}{
		{"default", ctx, nil, "accent1"},
		{"accent override", ctx, &CycleIntakeOverrides{TextOverrides: TextOverrides{Accent: "accent3"}}, "accent3"},
		// A theme whose accent1 cannot carry white text hands over its next slot.
		{"template primary fill", ExpandContext{LayoutBounds: ctx.LayoutBounds, Theme: types.ThemeInfo{Colors: []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1F2937"}, {Name: "lt2", RGB: "#EEEEEE"},
			{Name: "accent1", RGB: "#FFD54F"}, {Name: "accent2", RGB: "#0B3D91"},
		}}}, nil, "accent2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells := cycleIntakeSort(t, cycleIntakeExpand(t, tc.ctx, v, tc.ovr))
			want := `"` + tc.accent + `"`
			if got := string(cells.entry.Shape.Fill); got != want {
				t.Errorf("entry arrow fill = %s, want %s", got, want)
			}
			solid := cycleRingSolidAccents(cells.ring, "segment-")
			if len(solid) != 1 || !strings.HasSuffix(solid[0], "="+tc.accent) {
				t.Errorf("solid accent segments = %v, want only %s", solid, tc.accent)
			}
			if got := string(cycleRingLayers(cells.ring, "segment-")[3].Shape.Fill); got != want {
				t.Errorf("segment-4 fill = %s, want %s", got, want)
			}
		})
	}

	// cell_accent_mode varies the loop only: tinted segments, each badge its
	// own accent; the intake and its arrow keep the base accent.
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			t.Run(base+"/"+mode, func(t *testing.T) {
				ovr := &CycleIntakeOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}
				cells := cycleIntakeSort(t, cycleIntakeExpand(t, ctx, cycleIntakeValues(2, 6), ovr))
				if got := string(cells.entry.Shape.Fill); got != `"`+base+`"` {
					t.Errorf("entry arrow fill = %s, want %s", got, base)
				}
				for i, c := range cells.lane {
					if string(c.Shape.Fill) != neutral {
						t.Errorf("intake step %d is not neutral under %s", i, mode)
					}
				}
				badges := cycleRingLayers(cells.ring, "badge-")
				for i, b := range badges {
					want := `"` + ctx.ResolveCellAccent(base, i, mode) + `"`
					if got := string(b.Shape.Fill); got != want {
						t.Errorf("badge-%d fill = %s, want %s", i+1, got, want)
					}
				}
				segs := cycleRingLayers(cells.ring, "segment-")
				tinted := string(segs[0].Shape.Fill) != string(neutralTone(ringSegmentTint).fillJSON())
				if tinted != (mode != "uniform") {
					t.Errorf("mode %s: segment tinted = %v", mode, tinted)
				}
				if solid := cycleRingSolidAccents(cells.ring, "segment-"); len(solid) != 0 {
					t.Errorf("mode %s without a highlight: solid segments %v", mode, solid)
				}
			})
		}
	}
}

// loop_style nodes: numbered circles joined by link arrows, the highlighted
// one the only solid accent circle.
func TestCycleIntake_NodesStyle(t *testing.T) {
	ctx := cycleRingCtx(828, 349)
	v := cycleIntakeValues(2, 6)
	v.Loop[2].Highlight = true
	v.Center = &CycleIntakeCenter{Label: "Every month"}
	cells := cycleIntakeSort(t, cycleIntakeExpand(t, ctx, v, &CycleIntakeOverrides{LoopStyle: "nodes"}))
	nodes, links := cycleRingLayers(cells.ring, "node-"), cycleRingLayers(cells.ring, "link-")
	if len(nodes) != 6 || len(links) != 6 {
		t.Fatalf("%d nodes and %d links, want 6 and 6", len(nodes), len(links))
	}
	if len(cycleRingLayers(cells.ring, "segment-")) != 0 {
		t.Error("nodes style still draws segments")
	}
	solid := 0
	for i, n := range nodes {
		if n.Shape.Geometry != "ellipse" || math.Abs(n.Frame.W-n.Frame.H) > 1e-9 {
			t.Errorf("node-%d is not a circle: %+v", i+1, n.Frame)
		}
		if string(n.Shape.Fill) == `"accent1"` {
			solid++
			if i != 2 {
				t.Errorf("node-%d is solid; the highlight is node-3", i+1)
			}
		}
		var obj struct {
			Paragraphs []ringNodePara `json:"paragraphs"`
		}
		if err := json.Unmarshal(n.Shape.Text, &obj); err != nil || len(obj.Paragraphs) != 1 || obj.Paragraphs[0].Content != fmt.Sprint(i+1) || obj.Paragraphs[0].Size < 12 {
			t.Errorf("node-%d text = %s", i+1, n.Shape.Text)
		}
	}
	if solid != 1 {
		t.Errorf("%d solid accent nodes, want 1", solid)
	}
	for _, l := range links {
		if l.Shape.Geometry != "circularArrow" || l.Shape.FlipH {
			t.Errorf("%s = %s flip %v, want a clockwise circularArrow", l.Name, l.Shape.Geometry, l.Shape.FlipH)
		}
	}
	if got := cycleRingLayers(cells.ring, "centre"); len(got) != 1 || !strings.Contains(string(got[0].Shape.Text), "Every month") {
		t.Errorf("centre layer = %+v", got)
	}
}

func TestCycleIntake_Center(t *testing.T) {
	p := &cycleIntake{}
	for _, body := range cycleRingBodies {
		ctx := cycleRingCtx(body.w, body.h)
		v := cycleIntakeValues(3, 8)
		v.Center = &CycleIntakeCenter{Label: "Every quarter"}
		ring := cycleIntakeSort(t, cycleIntakeExpand(t, ctx, v, nil)).ring
		centre := cycleRingLayers(ring, "centre")
		if len(centre) != 1 {
			t.Fatalf("%s: %d centre layers", body.name, len(centre))
		}
		_, paras := cycleIntakeTextOf(t, centre[0].Shape.Text)
		if len(paras) != 1 || paras[0].Size < 12 || !paras[0].Bold {
			t.Errorf("%s: centre text = %+v", body.name, paras)
		}
		if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
			t.Errorf("%s: a two-word centre label warns: %v", body.name, w)
		}
	}
	v := cycleIntakeValues(3, 8)
	v.Center = &CycleIntakeCenter{Label: "Institutionalisation"}
	w := p.PostExpandWarnings(cycleRingCtx(687, 294), v, nil)
	if len(w) != 1 || !strings.Contains(w[0], "center.label does not fit") {
		t.Errorf("a 20-letter word in the smallest loop: warnings = %v", w)
	}
}

// Every text the pattern writes is at 12pt or above, in every combination,
// body and style, and the numerals in the list match the badges.
func TestCycleIntake_TextAtOrAboveTheFloor(t *testing.T) {
	check := func(t *testing.T, raw json.RawMessage) {
		t.Helper()
		if len(raw) == 0 {
			return
		}
		var obj struct {
			Paragraphs []struct {
				Size float64 `json:"size"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("text: %v", err)
		}
		for _, p := range obj.Paragraphs {
			if p.Size < shapegrid.MinTextSizePt {
				t.Errorf("text at %.1fpt: %s", p.Size, raw)
			}
		}
	}
	areas := append([]struct {
		name string
		w, h float64
	}{{"half", 337, 294}}, cycleRingBodies...)
	for _, body := range areas {
		for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
			for nL := ringMinItems; nL <= ringMaxItems; nL++ {
				for _, style := range cycleIntakeStyles {
					grid := cycleIntakeExpand(t, cycleRingCtx(body.w, body.h), cycleIntakeValues(nI, nL), &CycleIntakeOverrides{LoopStyle: style})
					for _, r := range grid.Rows {
						for _, c := range r.Cells {
							if c == nil {
								continue
							}
							if c.Shape != nil {
								check(t, c.Shape.Text)
							}
							for _, l := range c.Layers {
								check(t, l.Shape.Text)
							}
						}
					}
				}
			}
		}
	}
}

func TestCycleIntake_PostExpandWarnings(t *testing.T) {
	p := &cycleIntake{}
	// Clean in every combination on every body.
	for _, body := range cycleRingBodies {
		for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
			for nL := ringMinItems; nL <= ringMaxItems; nL++ {
				for _, style := range cycleIntakeStyles {
					if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), cycleIntakeValues(nI, nL), &CycleIntakeOverrides{LoopStyle: style}); len(w) != 0 {
						t.Errorf("%s %d+%d %s: %v", body.name, nI, nL, style, w)
					}
				}
			}
		}
	}
	// Out-of-range counts and foreign values report nothing (Validate does).
	if w := p.PostExpandWarnings(ExpandContext{}, cycleIntakeValues(4, 4), nil); w != nil {
		t.Errorf("4 intake steps: %v", w)
	}
	if w := p.PostExpandWarnings(ExpandContext{}, &CycleRingValues{}, nil); w != nil {
		t.Errorf("foreign values: %v", w)
	}

	// An intake description that outgrows the room under the lane is named.
	v := cycleIntakeValues(3, 4)
	v.Intake[1].Description = strings.TrimSpace(strings.Repeat("Partners log every lead ", 3))[:cycleIntakeStepDescMax]
	w := p.PostExpandWarnings(cycleRingCtx(687, 150), v, nil)
	found := false
	for _, msg := range w {
		if strings.HasPrefix(msg, ErrCodeBodyTooLong) && strings.Contains(msg, "intake[1].description needs") && strings.Contains(msg, "under the intake lane") {
			found = true
		}
	}
	if !found {
		t.Errorf("short area: warnings = %v, want intake[1].description named", w)
	}

	// List rows that cannot hold their labels are named.
	w = p.PostExpandWarnings(cycleRingCtx(687, 150), cycleIntakeValues(1, 8), nil)
	if len(w) == 0 || !strings.Contains(strings.Join(w, "\n"), "label needs") || !strings.Contains(strings.Join(w, "\n"), "list row") {
		t.Errorf("eight rows in 150pt: warnings = %v", w)
	}
}

func TestCycleIntake_Deterministic(t *testing.T) {
	p := &cycleIntake{}
	a, _ := p.Expand(cycleRingCtx(828, 349), p.ExemplarValues(), nil, nil)
	b, _ := p.Expand(cycleRingCtx(828, 349), p.ExemplarValues(), nil, nil)
	if !reflect.DeepEqual(a, b) {
		t.Error("two expansions of the same values differ")
	}
}

func TestCycleIntake_Golden(t *testing.T) {
	p := &cycleIntake{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "cycle-intake", "default.golden.json"))
}

func TestCycleIntake_ExemplarIsClean(t *testing.T) {
	p := &cycleIntake{}
	if err := p.Validate(p.ExemplarValues(), nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, body := range cycleRingBodies {
		if w := p.PostExpandWarnings(cycleRingCtx(body.w, body.h), p.ExemplarValues(), nil); len(w) != 0 {
			t.Errorf("%s: exemplar warns: %v", body.name, w)
		}
	}
}

func TestCycleIntake_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"onboarding then recurring service cycle", &ContentHints{ItemCount: 5}, "cycle-intake"},
		{"intake into the cycle", nil, "cycle-intake"},
		{"pipeline feeding the loop", &ContentHints{ItemCount: 6}, "cycle-intake"},
		{"acquire then retain loop", &ContentHints{ItemCount: 4}, "cycle-intake"},
		// A loop that nothing feeds stays with the plain cycles.
		{"PDCA", nil, "cycle-ring"},
		{"approval workflow with a decision point", nil, "process-flow"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}
