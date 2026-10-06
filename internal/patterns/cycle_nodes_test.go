package patterns

import (
	"encoding/json"
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

var cycleNodesTestSteps = []CycleNodesStep{
	{Label: "Cut-off", Description: "Freeze postings on day one"},
	{Label: "Accruals", Description: "Book open orders and receipts"},
	{Label: "Reconcile", Description: "Clear bank and intercompany items"},
	{Label: "Consolidate", Description: "Roll up the 14 legal entities"},
	{Label: "Review", Description: "Controllers challenge variances"},
	{Label: "Report", Description: "Flash figures reach the board"},
	{Label: "Forecast", Description: "Update the rolling outlook"},
	{Label: "Reopen", Description: "Release the next period"},
}

func cycleNodesTestValues(n int) *CycleNodesValues {
	return &CycleNodesValues{Steps: append([]CycleNodesStep(nil), cycleNodesTestSteps[:n]...)}
}

// cycleNodesAreaCtx is an expand context whose content area is w × h points.
func cycleNodesAreaCtx(w, h float64) ExpandContext {
	return ExpandContext{LayoutBounds: LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}}
}

// The content areas the pattern must hold in: the shortest shipped body
// (abstract), the widest (the local p-style), and a compose half.
var cycleNodesAreas = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"p-style", 899, 360},
	{"compose-half", 330, 290},
}

func cycleNodesExpand(t *testing.T, ctx ExpandContext, v *CycleNodesValues, ovr *CycleNodesOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	grid, err := (&cycleNodes{}).Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	ApplyGridDefaults(grid)
	return grid
}

// cycleNodesRing returns the ring cell of an expanded grid: the one that
// carries layers.
func cycleNodesRing(t *testing.T, grid *jsonschema.ShapeGridInput) *jsonschema.GridCellInput {
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
	if ring.Fit != "contain" {
		t.Errorf("ring cell fit = %q, want contain (the ring must stay round)", ring.Fit)
	}
	return ring
}

func cycleNodesLayersNamed(ring *jsonschema.GridCellInput, prefix string) []jsonschema.LayerInput {
	var out []jsonschema.LayerInput
	for _, l := range ring.Layers {
		if strings.HasPrefix(l.Name, prefix) {
			out = append(out, l)
		}
	}
	return out
}

// cycleNodesTextCells returns the lattice's text cells (number cues and
// labels) with their first paragraph.
func cycleNodesTextCells(t *testing.T, grid *jsonschema.ShapeGridInput) []cycleNodesTextCellInfo {
	t.Helper()
	var out []cycleNodesTextCellInfo
	for _, r := range grid.Rows {
		for _, c := range r.Cells {
			if c == nil || c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			var obj struct {
				Align      string         `json:"align"`
				Paragraphs []ringNodePara `json:"paragraphs"`
			}
			if err := json.Unmarshal(c.Shape.Text, &obj); err != nil {
				t.Fatal(err)
			}
			out = append(out, cycleNodesTextCellInfo{cell: c, align: obj.Align, paras: obj.Paragraphs})
		}
	}
	return out
}

type cycleNodesTextCellInfo struct {
	cell  *jsonschema.GridCellInput
	align string
	paras []ringNodePara
}

// cycleNodesResolveAt resolves an expanded grid, layers included, in a
// w × h point area at the origin.
func cycleNodesResolveAt(t *testing.T, in *jsonschema.ShapeGridInput, w, h float64) *shapegrid.ResolveResult {
	t.Helper()
	var cols []float64
	if err := json.Unmarshal(in.Columns, &cols); err != nil {
		t.Fatalf("columns: %v", err)
	}
	spec := func(s *jsonschema.ShapeSpecInput) *shapegrid.ShapeSpec {
		return &shapegrid.ShapeSpec{Geometry: s.Geometry, Fill: s.Fill, Line: s.Line, Text: s.Text, Adjustments: s.Adjustments, FlipH: s.FlipH}
	}
	rows := make([]shapegrid.Row, len(in.Rows))
	for i, r := range in.Rows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil {
				continue
			}
			cells[j] = shapegrid.Cell{ColSpan: c.ColSpan, RowSpan: c.RowSpan, Fit: shapegrid.FitMode(c.Fit)}
			if c.Shape != nil {
				cells[j].Shape = spec(c.Shape)
			}
			for _, l := range c.Layers {
				cells[j].Layers = append(cells[j].Layers, shapegrid.Layer{Name: l.Name, Shape: spec(l.Shape),
					Frame: shapegrid.LayerFrame{X: l.Frame.X, Y: l.Frame.Y, W: l.Frame.W, H: l.Frame.H}})
			}
		}
		rows[i] = shapegrid.Row{Cells: cells, MinHeight: r.MinHeight, MaxHeight: r.MaxHeight}
	}
	vAlign, _ := shapegrid.ParseVerticalAlign(in.VerticalAlign)
	g := &shapegrid.Grid{
		Bounds:  pptx.RectEmu{CX: int64(w * 12700), CY: int64(h * 12700)},
		Columns: cols, Rows: rows, ColGap: in.ColGap, RowGap: in.RowGap, VAlign: vAlign,
	}
	if err := shapegrid.Validate(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := shapegrid.Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}

func cycleNodesRectsOverlap(a, b pptx.RectEmu) bool {
	const slack = 2 // EMU of rounding
	return a.X+slack < b.X+b.CX && b.X+slack < a.X+a.CX && a.Y+slack < b.Y+b.CY && b.Y+slack < a.Y+a.CY
}

// ---------------------------------------------------------------------------

func TestCycleNodes_Metadata(t *testing.T) {
	p, ok := Default().Get("cycle-nodes")
	if !ok {
		t.Fatal("cycle-nodes is not registered")
	}
	if p.Name() != "cycle-nodes" || p.Version() != 1 || p.Description() == "" || p.CellsHint() == "" {
		t.Errorf("metadata: %q v%d %q %q", p.Name(), p.Version(), p.Description(), p.CellsHint())
	}
	for _, sibling := range []string{"cycle-ring", "cycle-intake", "cycle-figure-eight", "radial-hub", "concentric-rings", "numbered-step-strip", "process-flow"} {
		if !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("NotWhen does not name %s", sibling)
		}
	}
	if !strings.Contains(p.UseWhen(), "prefer") {
		t.Error("UseWhen is not contrastive")
	}
	tax := p.Taxonomy()
	if tax.Category == "" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 || tax.DataVisual {
		t.Errorf("taxonomy: %+v", tax)
	}
	if p.NewCellOverride() != nil {
		t.Error("cycle-nodes takes no cell overrides")
	}
	if _, ok := p.NewValues().(*CycleNodesValues); !ok {
		t.Errorf("NewValues: %T", p.NewValues())
	}
	if _, ok := p.NewOverrides().(*CycleNodesOverrides); !ok {
		t.Errorf("NewOverrides: %T", p.NewOverrides())
	}
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Errorf("exemplar does not validate: %v", err)
	}
}

func TestCycleNodes_SchemaIsValidJSON(t *testing.T) {
	data := SchemaJSON(&cycleNodes{})
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	if len(data) > 6000 {
		t.Errorf("schema is %d bytes (max 6000)", len(data))
	}
	if _, ok := m["$defs"]; !ok {
		t.Error("schema has no $defs")
	}
	s := string(data)
	for _, key := range []string{`"steps"`, `"center"`, `"highlight"`, `"direction"`, `"labels"`, `"arrows"`, `"cell_accent_mode"`, `"header_size"`, `"body_size"`, `"counter_clockwise"`, `"legend"`} {
		if !strings.Contains(s, key) {
			t.Errorf("schema does not publish %s", key)
		}
	}
}

func TestCycleNodes_Validate(t *testing.T) {
	p := &cycleNodes{}
	for n := ringMinItems; n <= ringMaxItems; n++ {
		v := cycleNodesTestValues(n)
		v.Highlight = n
		v.Center = &CycleNodesCenter{Label: "Monthly close"}
		if err := p.Validate(v, &CycleNodesOverrides{Direction: "counter_clockwise", Labels: "legend", Arrows: "none"}, nil); err != nil {
			t.Errorf("%d steps: %v", n, err)
		}
	}
	// A multi-byte value inside the budget is counted in runes, not bytes.
	umlauts := cycleNodesTestValues(3)
	umlauts.Steps[0].Label = strings.Repeat("ä", cycleNodesLabelMax)
	umlauts.Steps[0].Description = strings.Repeat("ü", cycleNodesDescMax)
	umlauts.Center = &CycleNodesCenter{Label: strings.Repeat("ö", cycleNodesCenterMax)}
	if err := p.Validate(umlauts, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budget: %v", err)
	}

	mutate := func(n int, f func(*CycleNodesValues)) *CycleNodesValues {
		v := cycleNodesTestValues(n)
		f(v)
		return v
	}
	cases := []struct {
		name      string
		values    any
		overrides any
		cells     map[int]any
		want      string
	}{
		{"wrong values type", &StateShiftHubValues{}, nil, nil, "values must be"},
		{"wrong overrides type", cycleNodesTestValues(3), &TextOverrides{}, nil, "overrides must be"},
		{"two steps", &CycleNodesValues{Steps: cycleNodesTestSteps[:2]}, nil, nil, "before-after"},
		{"nine steps", &CycleNodesValues{Steps: append(append([]CycleNodesStep(nil), cycleNodesTestSteps...), CycleNodesStep{Label: "Ninth"})}, nil, nil, "numbered-step-strip"},
		{"missing label", mutate(3, func(v *CycleNodesValues) { v.Steps[1].Label = "  " }), nil, nil, "steps[1].label"},
		{"label too long", mutate(3, func(v *CycleNodesValues) { v.Steps[2].Label = strings.Repeat("x", cycleNodesLabelMax+1) }), nil, nil, "steps[2].label"},
		{"description too long", mutate(3, func(v *CycleNodesValues) { v.Steps[0].Description = strings.Repeat("x", cycleNodesDescMax+1) }), nil, nil, "steps[0].description"},
		{"empty centre", mutate(3, func(v *CycleNodesValues) { v.Center = &CycleNodesCenter{} }), nil, nil, "center.label"},
		{"centre too long", mutate(3, func(v *CycleNodesValues) {
			v.Center = &CycleNodesCenter{Label: strings.Repeat("x", cycleNodesCenterMax+1)}
		}), nil, nil, "center.label"},
		{"highlight past the ring", mutate(4, func(v *CycleNodesValues) { v.Highlight = 5 }), nil, nil, "highlight"},
		{"negative highlight", mutate(4, func(v *CycleNodesValues) { v.Highlight = -1 }), nil, nil, "highlight"},
		{"bad direction", cycleNodesTestValues(3), &CycleNodesOverrides{Direction: "anticlockwise"}, nil, "overrides.direction"},
		{"bad labels", cycleNodesTestValues(3), &CycleNodesOverrides{Labels: "below"}, nil, "overrides.labels"},
		{"bad arrows", cycleNodesTestValues(3), &CycleNodesOverrides{Arrows: "straight"}, nil, "overrides.arrows"},
		{"bad accent mode", cycleNodesTestValues(3), &CycleNodesOverrides{TextOverrides: TextOverrides{CellAccentMode: "rainbow"}}, nil, "cell_accent_mode"},
		{"cell overrides", cycleNodesTestValues(3), nil, map[int]any{0: &CellOverride{}}, "cell_overrides are not supported"},
		{"inside with a description", cycleNodesTestValues(3), &CycleNodesOverrides{Labels: "inside"}, nil, "draws no descriptions"},
	}
	for _, tc := range cases {
		err := p.Validate(tc.values, tc.overrides, tc.cells)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", tc.name, err, tc.want)
		}
	}
}

func TestCycleNodesInsideLabelsRejectedAboveFive(t *testing.T) {
	p := &cycleNodes{}
	short := func(n int) *CycleNodesValues {
		v := &CycleNodesValues{}
		for i := 0; i < n; i++ {
			v.Steps = append(v.Steps, CycleNodesStep{Label: cycleNodesTestSteps[i].Label})
		}
		return v
	}
	inside := &CycleNodesOverrides{Labels: "inside"}
	for n := 3; n <= 5; n++ {
		if err := p.Validate(short(n), inside, nil); err != nil {
			t.Errorf("%d steps with inside labels: %v", n, err)
		}
	}
	for n := 6; n <= 8; n++ {
		err := p.Validate(short(n), inside, nil)
		if err == nil || !strings.Contains(err.Error(), "at most 5 steps") || !strings.Contains(err.Error(), `"outside"`) {
			t.Errorf("%d steps with inside labels: %v, want a refusal that names labels \"outside\"", n, err)
		}
	}
	long := short(4)
	long.Steps[1].Label = strings.Repeat("ä", cycleNodesInsideLabelMax)
	if err := p.Validate(long, inside, nil); err != nil {
		t.Errorf("a %d-rune inside label: %v", cycleNodesInsideLabelMax, err)
	}
	long.Steps[1].Label += "x"
	if err := p.Validate(long, inside, nil); err == nil || !strings.Contains(err.Error(), "steps[1].label") {
		t.Errorf("a %d-character inside label: %v", cycleNodesInsideLabelMax+1, err)
	}
	if err := p.Validate(long, nil, nil); err != nil {
		t.Errorf("the same label outside: %v", err)
	}
}

func TestCycleNodesInsideLabelsMeasuredWarning(t *testing.T) {
	p := &cycleNodes{}
	inside := &CycleNodesOverrides{Labels: "inside"}
	ctx := cycleNodesAreaCtx(687, 294)
	fits := &CycleNodesValues{Steps: []CycleNodesStep{{Label: "Sense"}, {Label: "Decide"}, {Label: "Act"}, {Label: "Learn"}}}
	if w := p.PostExpandWarnings(ctx, fits, inside); len(w) != 0 {
		t.Errorf("short inside labels warn: %v", w)
	}
	// Inside the character limit, but one unbroken word wider than the node.
	wide := &CycleNodesValues{Steps: []CycleNodesStep{{Label: "Sense"}, {Label: "WWWWWWWWWWWWWW"}, {Label: "Act"}, {Label: "Learn"}, {Label: "Share"}}}
	if err := p.Validate(wide, inside, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	w := p.PostExpandWarnings(ctx, wide, inside)
	if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong+":") || !strings.Contains(w[0], "steps[1].label") || !strings.Contains(w[0], `"outside"`) {
		t.Errorf("warnings = %v, want one BODY_TOO_LONG naming steps[1].label and labels \"outside\"", w)
	}
	// The same label fits outside the ring.
	if w := p.PostExpandWarnings(ctx, wide, nil); len(w) != 0 {
		t.Errorf("outside labels warn: %v", w)
	}
}

func TestCycleNodes_ExpandLayout(t *testing.T) {
	for n := ringMinItems; n <= ringMaxItems; n++ {
		for _, direction := range []string{"clockwise", "counter_clockwise"} {
			t.Run(fmt.Sprintf("%d/%s", n, direction), func(t *testing.T) {
				v := cycleNodesTestValues(n)
				v.Center = &CycleNodesCenter{Label: "Close"}
				grid := cycleNodesExpand(t, cycleNodesAreaCtx(899, 360), v, &CycleNodesOverrides{Direction: direction})
				if grid.VerticalAlign != "center" {
					t.Errorf("vertical_align = %q", grid.VerticalAlign)
				}
				ring := cycleNodesRing(t, grid)
				nodes := cycleNodesLayersNamed(ring, "node-")
				links := cycleNodesLayersNamed(ring, "link-")
				if len(nodes) != n || len(links) != n || len(cycleNodesLayersNamed(ring, "centre")) != 1 || len(ring.Layers) != 2*n+1 {
					t.Fatalf("%d node, %d link layers of %d, want %d + %d + centre", len(nodes), len(links), len(ring.Layers), n, n)
				}
				// Links are drawn first, so a node always covers an arrow end.
				if !strings.HasPrefix(ring.Layers[0].Name, "link-") || ring.Layers[len(ring.Layers)-1].Name != "centre" {
					t.Errorf("layer order: first %q, last %q", ring.Layers[0].Name, ring.Layers[len(ring.Layers)-1].Name)
				}
				dia := ringNodeDiameter(n)
				sign := 1.0
				if direction == "counter_clockwise" {
					sign = -1
				}
				for i, l := range nodes {
					f := l.Frame
					if l.Shape.Geometry != "ellipse" || !ringNear(f.W, dia) || !ringNear(f.H, dia) {
						t.Errorf("node %d: %s %+v, want an ellipse %.2f across", i+1, l.Shape.Geometry, f, dia)
					}
					if f.X < -ringTol || f.Y < -ringTol || f.X+f.W > 1+ringTol || f.Y+f.H > 1+ringTol {
						t.Errorf("node %d leaves the square: %+v", i+1, f)
					}
					// Node 1 at 12 o'clock, the rest at equal steps in the
					// direction of travel, all on one centreline.
					wantX, wantY := pointOnCircle(0.5, 0.5, 0.5-dia/2, -90+sign*float64(i)*360/float64(n))
					if !ringNear(f.X+f.W/2, wantX) || !ringNear(f.Y+f.H/2, wantY) {
						t.Errorf("node %d centre (%.4f, %.4f), want (%.4f, %.4f)", i+1, f.X+f.W/2, f.Y+f.H/2, wantX, wantY)
					}
					if !strings.Contains(string(l.Shape.Text), fmt.Sprintf(`"content":"%d"`, i+1)) {
						t.Errorf("node %d does not carry its number: %s", i+1, l.Shape.Text)
					}
				}
				// Labels: a number cue and a text frame per step, turned to
				// the ring (right-aligned on the left).
				cells := cycleNodesTextCells(t, grid)
				if len(cells) != 2*n {
					t.Fatalf("%d text cells, want %d cues + %d labels", len(cells), n, n)
				}
				left, right := 0, 0
				for _, c := range cells {
					if c.align == "r" {
						left++
					} else {
						right++
					}
				}
				if left == 0 || right == 0 || left+right != 2*n || absInt(left-right) > 2 {
					t.Errorf("label cells: %d left, %d right", left/2, right/2)
				}
			})
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestCycleNodesArrowsClearNodes: for every count and direction each link
// arrow starts and ends at least 4° outside the node circles it joins, sits
// centred in the ring square and keeps the preset inside the limits both
// renderers draw alike (shaft no wider than the head, head overhang <= 12.5%).
func TestCycleNodesArrowsClearNodes(t *testing.T) {
	for n := ringMinItems; n <= ringMaxItems; n++ {
		for _, clockwise := range []bool{true, false} {
			direction := "clockwise"
			if !clockwise {
				direction = "counter_clockwise"
			}
			grid := cycleNodesExpand(t, cycleNodesAreaCtx(687, 294), cycleNodesTestValues(n), &CycleNodesOverrides{Direction: direction})
			ring := cycleNodesRing(t, grid)
			dia := ringNodeDiameter(n)
			radius := 0.5 - dia/2
			nodeHalf := 2 * radToDeg(math.Asin(dia/(4*radius)))
			step := 360 / float64(n)
			for i, l := range cycleNodesLayersNamed(ring, "link-") {
				s := l.Shape
				if s.Geometry != "circularArrow" || s.FlipH == clockwise {
					t.Fatalf("n=%d %s link %d: %s flip_h=%v", n, direction, i+1, s.Geometry, s.FlipH)
				}
				if string(s.Line) != `"none"` {
					t.Errorf("n=%d link %d carries an outline", n, i+1)
				}
				f := l.Frame
				if !ringNear(f.W, f.H) || !ringNear(f.X+f.W/2, 0.5) || !ringNear(f.Y+f.H/2, 0.5) || f.X < 0 || f.W > 1 {
					t.Errorf("n=%d link %d frame %+v is not a centred square inside the ring square", n, i+1, f)
				}
				a := s.Adjustments
				start := float64(a["adj4"]) / ooxmlAngleUnits
				tip := float64(a["adj3"]+a["adj2"]) / ooxmlAngleUnits
				// The preset always runs clockwise; a counter-clockwise ring
				// is its mirror image, so in the mirrored frame node i sits
				// at -90 + i·step as well.
				from := -90 + float64(i)*step
				gapStart := normDeg(start - from)
				gapEnd := normDeg(from + step - tip)
				if gapStart < nodeHalf+ringNodeClearDeg-1e-3 || gapEnd < nodeHalf+ringNodeClearDeg-1e-3 {
					t.Errorf("n=%d %s link %d: starts %.2f° and ends %.2f° from the node centres, want >= %.2f° (node half-width %.2f° + %.0f° clearance)",
						n, direction, i+1, gapStart, gapEnd, nodeHalf+ringNodeClearDeg, nodeHalf, ringNodeClearDeg)
				}
				if a["adj2"] <= 0 || normDeg(tip-start) <= float64(a["adj2"])/ooxmlAngleUnits {
					t.Errorf("n=%d link %d has no shaft: start %.2f° tip %.2f° head %.2f°", n, i+1, start, tip, float64(a["adj2"])/ooxmlAngleUnits)
				}
				if a["adj1"] > a["adj5"] || a["adj5"] > 12500 || a["adj1"] <= 0 {
					t.Errorf("n=%d link %d: shaft adj1=%d head adj5=%d outside the verified range", n, i+1, a["adj1"], a["adj5"])
				}
				// The shaft runs on the node centreline.
				if centreline := f.W * (0.5 - float64(a["adj5"])/ooxmlFracUnits); math.Abs(centreline-radius) > 1e-4 {
					t.Errorf("n=%d link %d centreline radius %.4f, want %.4f", n, i+1, centreline, radius)
				}
			}
		}
	}
}

// TestCycleNodesNoOverlapForEveryCount resolves every count in every label
// mode in the areas the pattern must hold in: the ring stays a square, no two
// lattice cells overlap, no label touches the ring square, node circles keep
// clear of each other and nothing leaves the area.
func TestCycleNodesNoOverlapForEveryCount(t *testing.T) {
	for _, area := range cycleNodesAreas {
		for n := ringMinItems; n <= ringMaxItems; n++ {
			for _, labels := range []string{"", "legend", "inside"} {
				if labels == "inside" && n > cycleNodesInsideMaxSteps {
					continue
				}
				t.Run(fmt.Sprintf("%s/%d/%s", area.name, n, labels), func(t *testing.T) {
					v := cycleNodesTestValues(n)
					if labels == "inside" {
						for i := range v.Steps {
							v.Steps[i].Description = ""
						}
					}
					grid := cycleNodesExpand(t, cycleNodesAreaCtx(area.w, area.h), v, &CycleNodesOverrides{Labels: labels})
					res := cycleNodesResolveAt(t, grid, area.w, area.h)
					var cells, nodes []shapegrid.ResolvedCell
					var ringRect pptx.RectEmu
					for _, c := range res.Cells {
						if c.ShapeSpec == nil {
							continue
						}
						if c.Bounds.X < -2 || c.Bounds.Y < -2 || c.Bounds.X+c.Bounds.CX > int64(area.w*12700)+2 || c.Bounds.Y+c.Bounds.CY > int64(area.h*12700)+2 {
							t.Errorf("%s leaves the %.0f × %.0fpt area: %+v", c.ShapeSpec.Geometry, area.w, area.h, c.Bounds)
						}
						switch {
						case !c.Layer:
							cells = append(cells, c)
						case strings.HasPrefix(c.LayerName, "node-"):
							nodes = append(nodes, c)
							ringRect = c.CellBounds
						}
					}
					if len(nodes) != n {
						t.Fatalf("%d resolved nodes, want %d", len(nodes), n)
					}
					for i, a := range nodes {
						if d := a.Bounds.CX - a.Bounds.CY; d < -2 || d > 2 {
							t.Errorf("node %d is not a circle: %d × %d EMU", i+1, a.Bounds.CX, a.Bounds.CY)
						}
						for j := i + 1; j < len(nodes); j++ {
							b := nodes[j]
							dist := math.Hypot(float64(a.Bounds.X+a.Bounds.CX/2-b.Bounds.X-b.Bounds.CX/2), float64(a.Bounds.Y+a.Bounds.CY/2-b.Bounds.Y-b.Bounds.CY/2))
							if dist < float64(a.Bounds.CX)*1.05 {
								t.Errorf("nodes %d and %d touch: centres %.1fpt apart, diameter %.1fpt", i+1, j+1, dist/12700, float64(a.Bounds.CX)/12700)
							}
						}
					}
					wantCells := 2 * n
					if labels == "inside" {
						wantCells = 0
					}
					if len(cells) != wantCells {
						t.Fatalf("%d lattice text cells, want %d", len(cells), wantCells)
					}
					for i, a := range cells {
						if cycleNodesRectsOverlap(a.Bounds, ringRect) {
							t.Errorf("text cell %+v overlaps the ring cell %+v", a.Bounds, ringRect)
						}
						for _, b := range cells[i+1:] {
							if cycleNodesRectsOverlap(a.Bounds, b.Bounds) {
								t.Errorf("text cells overlap: %+v and %+v", a.Bounds, b.Bounds)
							}
						}
					}
				})
			}
		}
	}
}

// TestCycleNodes_LegendEngagesInANarrowArea: in a compose half two label
// columns would be under the minimum width, so the default layout becomes one
// legend column to the right of the ring, in step order.
func TestCycleNodes_LegendEngagesInANarrowArea(t *testing.T) {
	v := cycleNodesTestValues(6)
	ctx := cycleNodesAreaCtx(330, 290)
	lay, err := cycleNodesMeasure(ctx, v, &CycleNodesOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if lay.mode != cycleNodesLabelsLegend {
		t.Fatalf("mode = %q in a 330pt area, want legend", lay.mode)
	}
	if wide, _ := cycleNodesMeasure(cycleNodesAreaCtx(687, 294), v, &CycleNodesOverrides{}); wide.mode != cycleNodesLabelsOutside {
		t.Errorf("mode = %q in a 687pt area, want outside", wide.mode)
	}
	if forced, _ := cycleNodesMeasure(ctx, v, &CycleNodesOverrides{Labels: "outside"}); forced.mode != cycleNodesLabelsOutside {
		t.Errorf("an authored labels mode must stand, got %q", forced.mode)
	}
	prevY := -1.0
	for i, r := range lay.rows {
		if r.side != ringSideRight || r.cueX0 < lay.ringX+lay.side || r.textX0 < r.cueX1-1e-6 || r.textX1 > lay.w+1e-6 {
			t.Errorf("legend row %d is not right of the ring: %+v (ring %.0f..%.0f)", i, r, lay.ringX, lay.ringX+lay.side)
		}
		if r.y0 <= prevY {
			t.Errorf("legend row %d is not below row %d", i, i-1)
		}
		prevY = r.y0
	}
	for _, c := range cycleNodesTextCells(t, cycleNodesExpand(t, ctx, v, nil)) {
		if c.align != "l" {
			t.Errorf("legend text aligned %q, want l", c.align)
		}
	}
}

func cycleNodesNodeFills(t *testing.T, ring *jsonschema.GridCellInput) []string {
	t.Helper()
	var fills []string
	for _, l := range cycleNodesLayersNamed(ring, "node-") {
		fills = append(fills, string(l.Shape.Fill))
	}
	return fills
}

func TestCycleNodes_ExpandStyling(t *testing.T) {
	// Default: every node the neutral tint, numerals in the default accent,
	// no solid accent without a highlight.
	v := cycleNodesTestValues(5)
	ring := cycleNodesRing(t, cycleNodesExpand(t, ExpandContext{}, v, nil))
	neutral := string(neutralFillJSON(NeutralTint8))
	for i, fill := range cycleNodesNodeFills(t, ring) {
		if fill != neutral {
			t.Errorf("node %d fill %s, want the neutral tint %s", i+1, fill, neutral)
		}
	}
	accent := ExpandContext{}.DefaultAccent()
	for _, l := range cycleNodesLayersNamed(ring, "node-") {
		if !strings.Contains(string(l.Shape.Text), fmt.Sprintf(`"color":%q`, accent)) {
			t.Errorf("%s numeral is not in %s: %s", l.Name, accent, l.Shape.Text)
		}
	}
	for _, l := range ring.Layers {
		if string(l.Shape.Line) != `"none"` {
			t.Errorf("layer %s carries an outline", l.Name)
		}
		if l.Name != "centre" && string(l.Shape.Fill) == `"none"` {
			t.Errorf("layer %s has no fill", l.Name)
		}
	}
	for _, l := range cycleNodesLayersNamed(ring, "link-") {
		if string(l.Shape.Fill) != string(ringLinkFillJSON()) || strings.Contains(string(l.Shape.Fill), "accent") {
			t.Errorf("%s fill %s, want the neutral link grey", l.Name, l.Shape.Fill)
		}
	}

	// Highlight: exactly one solid accent circle, in the authored accent.
	for _, base := range []string{"accent1", "accent3"} {
		v.Highlight = 4
		ring = cycleNodesRing(t, cycleNodesExpand(t, ExpandContext{}, v, &CycleNodesOverrides{TextOverrides: TextOverrides{Accent: base}}))
		solid := 0
		for i, fill := range cycleNodesNodeFills(t, ring) {
			switch {
			case fill == fmt.Sprintf("%q", base) && i == 3:
				solid++
			case fill != neutral:
				t.Errorf("accent %s: node %d fill %s", base, i+1, fill)
			}
		}
		if solid != 1 {
			t.Errorf("accent %s: %d solid accent nodes, want exactly the highlighted one", base, solid)
		}
		// The number cues beside the labels take the accent too.
		cues := 0
		for _, c := range cycleNodesTextCells(t, cycleNodesExpand(t, ExpandContext{}, v, &CycleNodesOverrides{TextOverrides: TextOverrides{Accent: base}})) {
			if len(c.paras) == 1 && c.paras[0].Color == base {
				cues++
			}
		}
		if cues != 5 {
			t.Errorf("accent %s: %d number cues in the accent, want 5", base, cues)
		}
	}

	// cell_accent_mode: uniform keeps the neutral tint; alternate and
	// progressive tint each node in its own accent.
	v.Highlight = 0
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			ring = cycleNodesRing(t, cycleNodesExpand(t, ExpandContext{}, v, &CycleNodesOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}))
			for i, fill := range cycleNodesNodeFills(t, ring) {
				want := neutral
				if mode != "uniform" {
					want = string(inactiveTintTone(ResolveCellAccent(base, i, mode)).fillJSON())
				}
				if fill != want {
					t.Errorf("%s %s: node %d fill %s, want %s", base, mode, i+1, fill, want)
				}
			}
		}
	}

	// arrows "none" draws a nondirectional cycle.
	ring = cycleNodesRing(t, cycleNodesExpand(t, ExpandContext{}, v, &CycleNodesOverrides{Arrows: "none"}))
	if n := len(cycleNodesLayersNamed(ring, "link-")); n != 0 || len(ring.Layers) != 5 {
		t.Errorf("arrows none: %d link layers of %d", n, len(ring.Layers))
	}

	// Sizes: header_size sets the labels, body_size the descriptions.
	for _, c := range cycleNodesTextCells(t, cycleNodesExpand(t, cycleNodesAreaCtx(899, 360), v, &CycleNodesOverrides{TextOverrides: TextOverrides{HeaderSize: 16, BodySize: 13}})) {
		if c.paras[0].Size != 16 || (len(c.paras) == 2 && c.paras[1].Size != 13) {
			t.Errorf("sizes %+v, want 16 / 13", c.paras)
		}
	}
}

// TestCycleNodes_MeasuredInk: on a theme whose accent does not read on the
// neutral tint the numerals take a theme ink that does, and the highlighted
// node's numeral is measured against the solid accent.
func TestCycleNodes_MeasuredInk(t *testing.T) {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1F2A44"}, {Name: "lt2", RGB: "#EEEEEE"},
		{Name: "accent1", RGB: "#FFD54F"}, {Name: "accent2", RGB: "#1565C0"},
	}}}
	v := cycleNodesTestValues(4)
	v.Highlight = 2
	ring := cycleNodesRing(t, cycleNodesExpand(t, ctx, v, &CycleNodesOverrides{TextOverrides: TextOverrides{Accent: "accent1"}}))
	nodes := cycleNodesLayersNamed(ring, "node-")
	if strings.Contains(string(nodes[0].Shape.Text), `"color":"accent1"`) {
		t.Errorf("a pale accent numeral on the neutral tint: %s", nodes[0].Shape.Text)
	}
	if !strings.Contains(string(nodes[1].Shape.Text), `"color":"dk`) {
		t.Errorf("numeral on a pale solid accent is not dark: %s", nodes[1].Shape.Text)
	}
	ring = cycleNodesRing(t, cycleNodesExpand(t, ctx, v, &CycleNodesOverrides{TextOverrides: TextOverrides{Accent: "accent2"}}))
	nodes = cycleNodesLayersNamed(ring, "node-")
	if !strings.Contains(string(nodes[0].Shape.Text), `"color":"accent2"`) || !strings.Contains(string(nodes[1].Shape.Text), `"color":"lt1"`) {
		t.Errorf("a dark accent: numeral %s, highlighted %s", nodes[0].Shape.Text, nodes[1].Shape.Text)
	}
}

func TestCycleNodes_InsideLabels(t *testing.T) {
	v := &CycleNodesValues{Steps: []CycleNodesStep{{Label: "Sense"}, {Label: "Decide"}, {Label: "Act"}, {Label: "Learn"}}, Highlight: 2}
	grid := cycleNodesExpand(t, cycleNodesAreaCtx(687, 294), v, &CycleNodesOverrides{Labels: "inside"})
	ring := cycleNodesRing(t, grid)
	if cells := cycleNodesTextCells(t, grid); len(cells) != 0 {
		t.Errorf("inside labels still draw %d label cells", len(cells))
	}
	nodes, badges := cycleNodesLayersNamed(ring, "node-"), cycleNodesLayersNamed(ring, "badge-")
	if len(nodes) != 4 || len(badges) != 4 {
		t.Fatalf("%d nodes, %d badges", len(nodes), len(badges))
	}
	for i, l := range nodes {
		if !strings.Contains(string(l.Shape.Text), fmt.Sprintf(`"content":%q`, v.Steps[i].Label)) {
			t.Errorf("node %d does not carry its label: %s", i+1, l.Shape.Text)
		}
		if l.Frame.W <= ringNodeDiameter(4) {
			t.Errorf("node %d is %.2f across, want larger than the numbered node (%.2f)", i+1, l.Frame.W, ringNodeDiameter(4))
		}
		b := badges[i]
		if !strings.Contains(string(b.Shape.Text), fmt.Sprintf(`"content":"%d"`, i+1)) || strings.Contains(string(b.Shape.Fill), "accent") {
			t.Errorf("badge %d: fill %s text %s", i+1, b.Shape.Fill, b.Shape.Text)
		}
		// The badge sits on the node's inner edge: nearer the centre.
		nodeR := math.Hypot(l.Frame.X+l.Frame.W/2-0.5, l.Frame.Y+l.Frame.H/2-0.5)
		badgeR := math.Hypot(b.Frame.X+b.Frame.W/2-0.5, b.Frame.Y+b.Frame.H/2-0.5)
		if !ringNear(nodeR-badgeR, l.Frame.W/2) {
			t.Errorf("badge %d is %.4f inside the node centre, want the node radius %.4f", i+1, nodeR-badgeR, l.Frame.W/2)
		}
	}
}

func TestCycleNodes_TextAtOrAboveTheFloor(t *testing.T) {
	for _, area := range cycleNodesAreas {
		for n := ringMinItems; n <= ringMaxItems; n++ {
			ctx := cycleNodesAreaCtx(area.w, area.h)
			v := cycleNodesTestValues(n)
			v.Center = &CycleNodesCenter{Label: "Close"}
			if area.name == "compose-half" && n == 8 {
				// Eight rows of label + description do not fit a compose
				// half (the pattern says so); eight labels do.
				for i := range v.Steps {
					v.Steps[i].Description = ""
				}
			}
			if w := (&cycleNodes{}).PostExpandWarnings(ctx, v, nil); len(w) != 0 {
				t.Errorf("%s n=%d: %v", area.name, n, w)
			}
			grid := cycleNodesExpand(t, ctx, v, nil)
			res := cycleNodesResolveAt(t, grid, area.w, area.h)
			for _, c := range res.Cells {
				if c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
					continue
				}
				tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
				if err != nil || tb == nil {
					t.Fatalf("text: %v", err)
				}
				smallest := smallestRunPt(tb)
				if smallest < 12 {
					t.Errorf("%s n=%d: %q declared at %.0fpt", area.name, n, firstText(tb), smallest)
				}
				if scale := writtenScaleIn(ctx, tb, c.Bounds); smallest*scale < 12 {
					t.Errorf("%s n=%d: %q is written at %.1fpt in a %.0f × %.0fpt shape", area.name, n, firstText(tb), smallest*scale, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700)
				}
			}
		}
	}
}

func TestCycleNodes_PostExpandWarnings(t *testing.T) {
	p := &cycleNodes{}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Errorf("nil values: %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, &CycleNodesValues{}, nil); got != nil {
		t.Errorf("no steps: %v", got)
	}
	for _, area := range cycleNodesAreas[:2] {
		if got := p.PostExpandWarnings(cycleNodesAreaCtx(area.w, area.h), p.ExemplarValues(), nil); len(got) != 0 {
			t.Errorf("exemplar warns on %s: %v", area.name, got)
		}
	}
	// A short, narrow area cannot hold eight full-length rows.
	dense := cycleNodesTestValues(8)
	for i := range dense.Steps {
		dense.Steps[i].Description = strings.Repeat("word ", 13) + "words"
	}
	got := p.PostExpandWarnings(cycleNodesAreaCtx(420, 200), dense, &CycleNodesOverrides{Labels: "outside"})
	if len(got) == 0 {
		t.Fatal("eight 70-character descriptions in a 420 × 200pt area raise no warning")
	}
	for _, w := range got {
		if !strings.HasPrefix(w, ErrCodeBodyTooLong+": cycle-nodes steps[") || !strings.Contains(w, "].description needs") || !strings.Contains(w, "label row with 8 steps") || !strings.Contains(w, "shorten it or use fewer steps") {
			t.Errorf("warning shape: %s", w)
		}
	}
	// One long row among short ones is the one named.
	one := cycleNodesTestValues(8)
	for i := range one.Steps {
		one.Steps[i].Description = ""
	}
	one.Steps[2].Description = strings.Repeat("word ", 13) + "words"
	one.Steps[1].Description = strings.Repeat("word ", 13) + "words"
	one.Steps[0].Description = strings.Repeat("word ", 13) + "words"
	got = p.PostExpandWarnings(cycleNodesAreaCtx(330, 120), one, &CycleNodesOverrides{Labels: "outside"})
	named := strings.Join(got, "\n")
	if !strings.Contains(named, "steps[2].description") || strings.Contains(named, "steps[5]") {
		t.Errorf("warnings name the wrong rows: %v", got)
	}
	// A centre label of one long word does not fit the ring of a small area.
	centre := cycleNodesTestValues(3)
	centre.Center = &CycleNodesCenter{Label: "WWWWWWWWWWWWWWWWWWWWWWWW"}
	got = p.PostExpandWarnings(cycleNodesAreaCtx(330, 200), centre, nil)
	if len(got) != 1 || !strings.Contains(got[0], "center.label") || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") {
		t.Errorf("centre label warnings: %v", got)
	}
}

func TestCycleNodes_ExpandRejects(t *testing.T) {
	p := &cycleNodes{}
	if _, err := p.Expand(ExpandContext{}, &StateShiftHubValues{}, nil, nil); err == nil {
		t.Error("wrong values type")
	}
	if _, err := p.Expand(ExpandContext{}, cycleNodesTestValues(3), &TextOverrides{}, nil); err == nil {
		t.Error("wrong overrides type")
	}
	if _, err := p.Expand(ExpandContext{}, &CycleNodesValues{Steps: cycleNodesTestSteps[:2]}, nil, nil); err == nil {
		t.Error("two steps")
	}
}

func TestCycleNodes_Deterministic(t *testing.T) {
	a, _ := json.Marshal(cycleNodesExpand(t, cycleNodesAreaCtx(687, 294), cycleNodesTestValues(7), nil))
	b, _ := json.Marshal(cycleNodesExpand(t, cycleNodesAreaCtx(687, 294), cycleNodesTestValues(7), nil))
	if string(a) != string(b) {
		t.Error("two expansions of the same input differ")
	}
}

func TestCycleNodes_Golden(t *testing.T) {
	p := &cycleNodes{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "cycle-nodes", "default.golden.json"))
}

func TestCycleNodes_RecommendIntents(t *testing.T) {
	// The loop intents rank cycle-nodes (or its ring sibling) in the top two.
	for _, intent := range []string{
		"plan do check act",
		"sense decide act learn",
		"innovation loop",
		"feedback loop",
		"PDCA cycle for the plant",
		"closed loop from customer feedback to the roadmap",
	} {
		res := Recommend(Default(), intent, &ContentHints{ItemCount: 4}, 5)
		found := false
		var names []string
		for i, c := range res.Candidates {
			names = append(names, c.PatternName)
			if i < 2 && (c.PatternName == "cycle-nodes" || c.PatternName == "cycle-ring") {
				found = true
			}
		}
		if !found {
			t.Errorf("intent %q: top candidates %v, want cycle-nodes or cycle-ring in the top two", intent, names)
		}
	}
	// A sequence that does not return to its start is not a cycle.
	for intent, want := range map[string]string{
		"process flow with a decision point for approvals": "process-flow",
		"today vs future state across the workflow":        "state-shift-hub",
	} {
		res := Recommend(Default(), intent, &ContentHints{ItemCount: 4}, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != want {
			t.Errorf("intent %q: top = %v, want %s", intent, res.Candidates, want)
		}
	}
}
