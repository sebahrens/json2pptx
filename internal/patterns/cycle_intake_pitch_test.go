package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// cycleIntakePitchTolPt is how far a list row may start from the common pitch:
// the lattice merges breakpoints that fall within half a point of another
// cell's edge (the lane, the entry arrow), which no reader sees. The defect
// this guards against is a row a whole text line (14pt or more) out of step.
const cycleIntakePitchTolPt = 1.0

// cycleIntakeListTops resolves the grid in a w × h area and returns the top
// of every numbered list row (the numeral cells, in number order) in points.
func cycleIntakeListTops(t *testing.T, grid *jsonschema.ShapeGridInput, w, h float64) []float64 {
	t.Helper()
	tops := map[int]float64{}
	for _, c := range cycleNodesResolveAt(t, grid, w, h).Cells {
		if c.ShapeSpec == nil || c.Layer || c.ShapeSpec.Geometry != "rect" || string(c.ShapeSpec.Fill) != `"none"` {
			continue
		}
		var obj ringTextObj
		if err := json.Unmarshal(c.ShapeSpec.Text, &obj); err != nil || len(obj.Paragraphs) != 1 {
			continue
		}
		if n, err := strconv.Atoi(obj.Paragraphs[0].Content); err == nil {
			tops[n] = float64(c.Bounds.Y) / 12700
		}
	}
	out := make([]float64, len(tops))
	for n, y := range tops {
		if n < 1 || n > len(out) {
			t.Fatalf("list numeral %d of %d", n, len(out))
		}
		out[n-1] = y
	}
	return out
}

// The numbered list beside the loop keeps one pitch (go-slide-creator-by2e3):
// a row whose text is measured a line taller than the others must not push
// the rows under it further apart than the rest. Checked from the resolved
// grid for every phase count, with and without descriptions, the longest text
// in the first row, on the three template bodies.
func TestCycleIntakeListKeepsOnePitch(t *testing.T) {
	// Eight phases have no static description budget: the bead's copy, whose
	// first description (31 characters) is the longest.
	eight := []string{"Every company files its monthly", "Against the plan", "Top five per company", "Quartiles by return",
		"Follow-on or hold", "Operating partners", "Buyer long list", "Quarterly letter"}
	for _, body := range cycleRingBodies {
		for nI := cycleIntakeMinSteps; nI <= cycleIntakeMaxSteps; nI++ {
			for nL := ringMinItems; nL <= ringMaxItems; nL++ {
				for _, withDesc := range []bool{false, true} {
					for _, style := range cycleIntakeStyles {
						t.Run(fmt.Sprintf("%s/%d+%d/desc=%v/%s", body.name, nI, nL, withDesc, style), func(t *testing.T) {
							v := cycleIntakeValues(nI, nL)
							// The first row carries the longest copy its budget allows.
							v.Loop[0].Label = cycleRingCopy(cycleIntakeLabelBudget(nI, nL))
							if withDesc {
								for i := range v.Loop {
									v.Loop[i].Description = eight[i]
								}
								if budget := cycleIntakeDescBudget(nI, nL); budget > 0 {
									v.Loop[0].Description = cycleRingCopy(budget)
								}
							}
							ctx := cycleRingCtx(body.w, body.h)
							grid := cycleIntakeExpand(t, ctx, v, &CycleIntakeOverrides{LoopStyle: style})
							tops := cycleIntakeListTops(t, grid, body.w, body.h)
							if len(tops) != nL {
								t.Fatalf("found %d list numerals, want %d", len(tops), nL)
							}
							pitch := tops[1] - tops[0]
							if pitch <= 0 {
								t.Fatalf("row 2 starts %.2fpt under row 1", pitch)
							}
							for i := 2; i < nL; i++ {
								if d := tops[i] - tops[i-1]; math.Abs(d-pitch) > cycleIntakePitchTolPt {
									t.Errorf("row %d starts %.2fpt under row %d, but row 2 starts %.2fpt under row 1: tops %.1f", i+1, d, i, pitch, tops)
								}
							}
						})
					}
				}
			}
		}
	}
}

// The bead's case: three intake steps and eight phases whose first
// description is measured a line longer than the others. The ring gives the
// list the width that sets it on one line, so every row keeps the pitch and
// the descriptions stay.
func TestCycleIntakeRingGivesWidthForOnePitch(t *testing.T) {
	v := cycleIntakeValues(3, 8)
	for i, d := range []string{"Every company files its monthly", "Against the plan", "Top five per company", "Quartiles by return",
		"Follow-on or hold", "Operating partners", "Buyer long list", "Quarterly letter"} {
		v.Loop[i].Description = d
	}
	const w, h = 828.0, 349.0 // midnight-blue
	ctx := cycleRingCtx(w, h)
	lay, err := cycleIntakeMeasure(ctx, v, &CycleIntakeOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	full := cycleNodesRingSide(w, h, cycleIntakeRingFrac[3])
	if !lay.list.showDesc {
		t.Fatal("the loop descriptions are left off")
	}
	if lay.side >= full || lay.side < full*(1-cycleIntakeMaxShrink)-0.01 {
		t.Errorf("ring side %.1fpt: want under the full %.1fpt and at least %.1fpt", lay.side, full, full*(1-cycleIntakeMaxShrink))
	}
	for i, r := range lay.list.rows {
		if r.capped || math.Abs((r.y1-r.y0)-(lay.list.rows[0].y1-lay.list.rows[0].y0)) > 0.01 {
			t.Errorf("row %d is %.1fpt tall (capped %v), row 1 is %.1fpt", i+1, r.y1-r.y0, r.capped, lay.list.rows[0].y1-lay.list.rows[0].y0)
		}
	}
	// Short copy leaves the ring its full side.
	short, err := cycleIntakeMeasure(ctx, cycleIntakeValues(3, 8), &CycleIntakeOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(short.side-full) > 0.01 {
		t.Errorf("ring side %.1fpt with short copy, want the full %.1fpt", short.side, full)
	}
}
