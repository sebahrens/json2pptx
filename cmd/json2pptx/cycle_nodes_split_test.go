package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// cycleNodesSplitSteps is the copy a ring carries beside a second zone: short
// labels with a one-line detail.
var cycleNodesSplitSteps = []patterns.CycleNodesStep{
	{Label: "Plan", Description: "Pick one loss"},
	{Label: "Do", Description: "Trial one line"},
	{Label: "Check", Description: "Measure the gap"},
	{Label: "Act", Description: "Set the standard"},
	{Label: "Share", Description: "Brief the plants"},
	{Label: "Audit", Description: "Verify the hold"},
	{Label: "Review", Description: "Rank next losses"},
	{Label: "Reset", Description: "Open the next loop"},
}

func cycleNodesSplitPattern(t *testing.T, n int, descriptions bool) PatternInput {
	t.Helper()
	v := &patterns.CycleNodesValues{Highlight: 2}
	for _, s := range cycleNodesSplitSteps[:n] {
		if !descriptions {
			s.Description = ""
		}
		v.Steps = append(v.Steps, s)
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return PatternInput{Name: "cycle-nodes", Values: encoded}
}

var (
	cycleNodesEllipseRE = regexp.MustCompile(`(?s)<a:ext cx="(\d+)" cy="(\d+)"/>.*?<a:prstGeom prst="ellipse">`)
	cycleNodesOffRE     = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/>`)
)

// cycleNodesExt is a written shape's size in EMU.
type cycleNodesExt struct{ cx, cy int64 }

// cycleNodesWrittenEllipses returns the size of every ellipse the slide
// writes, in EMU.
func cycleNodesWrittenEllipses(t *testing.T, input *PresentationInput, geom schemaMaximaGeometry) []cycleNodesExt {
	t.Helper()
	specs, _, _, err := convertPresentationSlides(input.Slides, geom.layouts, geom.width, geom.height, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var out []cycleNodesExt
	for _, spec := range specs {
		for _, fragment := range spec.RawShapeXML {
			for _, raw := range writtenShapeRE.FindAllString(string(fragment), -1) {
				m := cycleNodesEllipseRE.FindStringSubmatch(raw)
				if m == nil || !cycleNodesOffRE.MatchString(raw) {
					continue
				}
				cx, _ := strconv.ParseInt(m[1], 10, 64)
				cy, _ := strconv.ParseInt(m[2], 10, 64)
				out = append(out, cycleNodesExt{cx: cx, cy: cy})
			}
		}
	}
	return out
}

// TestCycleNodesSplitLayoutsAcrossTemplates is the split wall: beside a second pattern in a
// horizontal compose half (50%) and a 60% share, in a vertical segment and in
// a nested shape-grid cell the ring's nodes are still circles, every count
// renders on every template without a readability or refuse finding, and no
// run is written below its role floor.
func TestCycleNodesSplitLayoutsAcrossTemplates(t *testing.T) {
	kpi := PatternInput{Name: "kpi-2up", Values: json.RawMessage(`["38 | Loops closed","6 wk | Median loop"]`)}
	type layout struct {
		name         string
		descriptions func(n int) bool
		slide        func(cn PatternInput) SlideInput
	}
	compose := func(direction string, share float64) func(PatternInput) SlideInput {
		return func(cn PatternInput) SlideInput {
			return SlideInput{SlideType: "content", LayoutID: "blank-title", Compose: &ComposeInput{Direction: direction, Segments: []SegmentInput{
				{SizePct: share, Pattern: cn}, {SizePct: 100 - share, Pattern: kpi},
			}}}
		}
	}
	layouts := []layout{
		// A half holds a description per step up to seven steps (the legend
		// budget); eight carry labels only.
		{"horizontal-50", func(n int) bool { return n <= 5 }, compose("horizontal", 50)},
		{"horizontal-60", func(n int) bool { return n <= 5 }, compose("horizontal", 60)},
		{"vertical-65", func(n int) bool { return n <= 4 }, compose("vertical", 65)},
		{"nested-cell", func(n int) bool { return n <= 5 }, func(cn PatternInput) SlideInput {
			nested, _ := json.Marshal(cn)
			return SlideInput{SlideType: "content", LayoutID: "blank-title", ShapeGrid: &ShapeGridInput{
				Columns: json.RawMessage(`[50,50]`),
				Rows: []GridRowInput{{Cells: []*GridCellInput{
					{Pattern: nested},
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: json.RawMessage(`{"content":"The loop closes every six weeks","size":18}`)}},
				}}},
			}}
		}},
	}
	for _, templateName := range schemaMaximaRunTemplateNames(t) {
		geom := loadSchemaMaximaGeometry(t, templateName)
		for _, l := range layouts {
			for n := 3; n <= 8; n++ {
				t.Run(fmt.Sprintf("%s/%s/%d", templateName, l.name, n), func(t *testing.T) {
					input := &PresentationInput{Template: geom.name, Slides: []SlideInput{l.slide(cycleNodesSplitPattern(t, n, l.descriptions(n)))}}
					for _, f := range collectReadabilityFindings(input, geom.layouts, geom.width, geom.height) {
						t.Errorf("readability finding: %s %s", f.Code, f.Message)
					}
					if v := writtenRoleViolations(t, input, geom); v > 0 {
						t.Errorf("%d runs written below their role floor", v)
					}
					for _, f := range collectFitFindings(input, geom.layouts, geom.width, geom.height, nil) {
						if f.Action == "refuse" || (f.Code == patterns.ErrCodeBodyTooLong && strings.Contains(f.Message, "cycle-nodes")) {
							t.Errorf("fit finding: %s (%s) %s", f.Code, f.Action, f.Message)
						}
					}
					ellipses := cycleNodesWrittenEllipses(t, input, geom)
					if len(ellipses) != n {
						t.Fatalf("%d ellipses written, want the %d nodes", len(ellipses), n)
					}
					first := ellipses[0]
					for i, e := range ellipses {
						// Frame edges are rounded to whole EMU independently.
						if d := e.cx - e.cy; d < -2 || d > 2 {
							t.Errorf("node %d is %d × %d EMU: not a circle", i+1, e.cx, e.cy)
						}
						if d := e.cx - first.cx; d < -2 || d > 2 {
							t.Errorf("node %d is %d EMU across, node 1 %d", i+1, e.cx, first.cx)
						}
						// The smallest ring still gives a node room for a
						// 12pt numeral.
						if float64(e.cx)/12700 < 18 {
							t.Errorf("node %d is only %.1fpt across", i+1, float64(e.cx)/12700)
						}
					}
				})
			}
		}
	}
}

// TestCycleNodesLegendEngagesInAComposeHalf: in a 50% horizontal segment the
// labels become one legend column to the right of the ring, and the ring cell
// (a row-spanning cell of the nested lattice) keeps its square.
func TestCycleNodesLegendEngagesInAComposeHalf(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	for n := 3; n <= 8; n++ {
		compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: cycleNodesSplitPattern(t, n, n <= 5)}, {SizePct: 50, Pattern: hero}}}
		merged, _, err := expandCompose(compose, ctx, patterns.Default())
		if err != nil {
			t.Fatal(err)
		}
		result, err := resolveShapeGrid(merged, pptx.NewShapeIDAllocator(nil), &pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}, nil, ctx.SlideWidth, ctx.SlideHeight, nil)
		if err != nil {
			t.Fatal(err)
		}
		var ringRight, heroLeft int64
		var nodes, labels []shapegrid.ResolvedCell
		for _, c := range result.Cells {
			if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil {
				continue
			}
			switch {
			case strings.Contains(string(c.ShapeSpec.Text), "99%"):
				heroLeft = c.Bounds.X
			case c.Layer && strings.HasPrefix(c.LayerName, "node-"):
				nodes = append(nodes, c)
				ringRight = max(ringRight, c.Bounds.X+c.Bounds.CX)
			case !c.Layer:
				labels = append(labels, c)
			}
		}
		if len(nodes) != n || len(labels) != 2*n || heroLeft == 0 {
			t.Fatalf("n=%d: %d nodes and %d label cells beside the hero at %d, want %d and %d", n, len(nodes), len(labels), heroLeft, n, 2*n)
		}
		for i, c := range nodes {
			// The ring is much taller than one lattice row: a node of a
			// collapsed ring would be a few points across.
			if d := c.Bounds.CX - c.Bounds.CY; d < -2 || d > 2 || float64(c.Bounds.CX)/12700 < 20 {
				t.Errorf("n=%d node %d is %.1f × %.1fpt", n, i+1, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700)
			}
		}
		for _, c := range labels {
			if c.Bounds.X < ringRight {
				t.Errorf("n=%d: a label at x=%.0fpt is not right of the ring (%.0fpt): the legend did not engage", n, float64(c.Bounds.X)/12700, float64(ringRight)/12700)
			}
			if c.Bounds.X+c.Bounds.CX > heroLeft {
				t.Errorf("n=%d: a label ends at %.0fpt, inside the second segment (from %.0fpt)", n, float64(c.Bounds.X+c.Bounds.CX)/12700, float64(heroLeft)/12700)
			}
		}
	}
}

func TestCycleNodesDenseDescriptionWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.CycleNodesValues{}
	for _, s := range cycleNodesSplitSteps {
		values.Steps = append(values.Steps, patterns.CycleNodesStep{Label: s.Label, Description: s.Description})
	}
	// One legend column cannot hold eight three-line descriptions on any
	// template: the pattern names the steps that outgrow their row.
	for i := range values.Steps {
		values.Steps[i].Description = strings.TrimSpace(strings.Repeat("word ", 14))
	}
	assertBudgetFindingAcrossTemplates(t, "cycle-nodes", values, "steps[0].description", "label row with 8 steps",
		map[string]any{"labels": "legend", "body_size": 16})
}

// ---------------------------------------------------------------------------
// Render check (LibreOffice): the link arrows are drawn
// ---------------------------------------------------------------------------

func cycleNodesRenderSpec(templateName string, n int, arrows string) map[string]any {
	steps := make([]any, n)
	for i := range steps {
		steps[i] = map[string]any{"label": cycleNodesSplitSteps[i].Label}
	}
	slide := map[string]any{
		"slide_type": "content",
		"layout_id":  composeRecipeLayoutID,
		"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "The loop closes every six weeks"}},
		"pattern": map[string]any{"name": "cycle-nodes", "values": map[string]any{"steps": steps},
			"overrides": map[string]any{"arrows": arrows}},
	}
	return recipeSpec("cycle nodes", templateName, map[string]any{"kind": "raw_json2pptx", "slide": slide})
}

func cycleNodesRenderImage(t *testing.T, mc *mcpConfig, spec map[string]any, density int) image.Image {
	t.Helper()
	rendered, err := render.RenderSlideOpts(renderDeckSpecPath(t, mc, spec), 0, density, true)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	data, err := readSlideImageBytes(rendered)
	if err != nil {
		t.Fatal(err)
	}
	return decodePreviewPNG(t, data)
}

// TestCycleNodesArrowsRenderAcrossTemplates guards the choice of circularArrow
// for the links: a ring of six rendered with and without arrows differs in
// exactly the six gaps between the nodes, by about the area of six shafts and
// heads — a renderer that dropped the arrows, or drew the shafts without
// their heads, falls outside the band. Skipped without LibreOffice (CI).
func TestCycleNodesArrowsRenderAcrossTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	const (
		n       = 6
		density = 100
		// The geometry ring_nodes_draw.go draws, as shares of the ring square.
		nodeDia, shaft, headHalf, headLen, clearDeg = 0.22, 0.010, 0.026, 0.050, 4.0
	)
	templates := []string{"midnight-blue"}
	for _, name := range previewTestTemplates() {
		if name == "p-style" {
			templates = append(templates, name)
		}
	}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			mc := semanticTestConfig(t)
			with := cycleNodesRenderImage(t, mc, cycleNodesRenderSpec(tpl, n, "curved"), density)
			without := cycleNodesRenderImage(t, mc, cycleNodesRenderSpec(tpl, n, "none"), density)
			if with.Bounds() != without.Bounds() {
				t.Fatalf("renders differ in size: %v vs %v", with.Bounds(), without.Bounds())
			}
			type pt struct{ x, y int }
			var diff []pt
			b := with.Bounds()
			minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					r1, g1, b1, _ := with.At(x, y).RGBA()
					r2, g2, b2, _ := without.At(x, y).RGBA()
					if absDiff16(r1, r2) > 0x3000 || absDiff16(g1, g2) > 0x3000 || absDiff16(b1, b2) > 0x3000 {
						diff = append(diff, pt{x, y})
						minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
					}
				}
			}
			if len(diff) == 0 {
				t.Fatal("the render with arrows is the render without them: no link arrow was drawn")
			}
			// The arrows lie on one circle. Two of the six cross 3 and 9
			// o'clock, so the box is as wide as the circle; none crosses 12 or
			// 6 o'clock (nodes sit there), so it is a little less tall.
			w, h := float64(maxX-minX+1), float64(maxY-minY+1)
			if h > w || h < 0.8*w {
				t.Errorf("the arrows span %.0f × %.0f px: not six arrows on a circle", w, h)
			}
			cx, cy := float64(minX+maxX)/2, float64(minY+maxY)/2
			radius := 0.5 - nodeDia/2
			side := w / (2 * (radius + headHalf)) // ring square in pixels
			sectors := make([]int, n)
			for _, p := range diff {
				deg := math.Atan2(float64(p.y)-cy, float64(p.x)-cx)*180/math.Pi + 90 // 0 = 12 o'clock, clockwise
				if deg < 0 {
					deg += 360
				}
				sectors[int(deg/(360.0/n))%n]++
			}
			// One arrow: its shaft plus its triangular head.
			half := 2*math.Asin(nodeDia/(4*radius))*180/math.Pi + clearDeg
			arc := (360.0/n - 2*half) * math.Pi / 180 * radius
			want := ((arc-headLen)*shaft + headLen*headHalf) * side * side
			for i, got := range sectors {
				if float64(got) < 0.7*want || float64(got) > 1.5*want {
					t.Errorf("gap between node %d and node %d: %d differing pixels, want about %.0f (shaft and head) — the arrow is missing or was drawn without its head",
						i+1, i%n+2, got, want)
				}
			}
		})
	}
}

func absDiff16(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
