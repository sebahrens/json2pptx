package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/template"
)

func TestComposePreflightMatchesGeneratedGeometry(t *testing.T) {
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	sw, sh := template.ParseSlideDimensions(reader)
	slide := SlideInput{LayoutID: "content", Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{
		{SizePct: 40, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}},
		{SizePct: 60, Pattern: PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$4M | ARR","98% | NRR","1K | Customers"]`)}},
	}}}
	preflight := expandComposeForPreflight(&PresentationInput{Slides: []SlideInput{slide}}, sw, sh, layouts...)
	grid := preflight.Slides[0].ShapeGrid
	if grid == nil {
		t.Fatal("preflight did not expand compose")
	}
	geom := resolveGridGeometry(preflight.Slides[0], layouts, sw, sh)
	alloc := pptx.NewShapeIDAllocator(nil)
	alloc.SetMinID(200)
	preflightShapes, err := resolveShapeGrid(grid, alloc, geom.OverrideBounds, geom.Zone, sw, sh, nil)
	if err != nil {
		t.Fatal(err)
	}
	specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, layouts, sw, sh, nil, nil, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(preflightShapes.Shapes) != len(specs[0].RawShapeXML) {
		t.Fatalf("preflight %d shapes, generation %d", len(preflightShapes.Shapes), len(specs[0].RawShapeXML))
	}
	for i := range preflightShapes.Shapes {
		if !bytes.Equal(preflightShapes.Shapes[i], specs[0].RawShapeXML[i]) {
			t.Fatalf("compose shape %d differs between preflight and generation", i)
		}
	}
}

// The generate path must size KPI rows against the template's actual content
// frame. On midnight-blue's content layout this is shorter than the generic
// default used before go-slide-creator-byr2b.
func TestGeneratePatternUsesTemplateContentHeight(t *testing.T) {
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	sw, sh := template.ParseSlideDimensions(reader)
	slide := SlideInput{LayoutID: "content", Pattern: &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$4.2M | ARR", "127% | NRR", "12 days | Cycle"]`)}}
	_, content := patternExpansionGeometry(slide, layouts, sw, sh, nil)
	if content.CY <= 0 || content.CY >= shapegrid.DefaultBounds(sw, sh).CY {
		t.Fatalf("unexpected midnight-blue content height: %d EMU", content.CY)
	}
	specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, layouts, sw, sh, nil, nil, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || len(specs[0].RawShapeXML) == 0 {
		t.Fatal("no pattern shapes generated")
	}
	re := regexp.MustCompile(`(?:a:ext|p:ext) cx="\d+" cy="(\d+)"`)
	var maxHeight int64
	for _, shape := range specs[0].RawShapeXML {
		for _, m := range re.FindAllSubmatch(shape, -1) {
			h, err := strconv.ParseInt(string(m[1]), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if h > maxHeight {
				maxHeight = h
			}
		}
	}
	if maxHeight == 0 {
		t.Fatal("no rendered shape extent found")
	}
	preflight, _ := expandPatternsForFit(&PresentationInput{Slides: []SlideInput{slide}}, sw, sh, nil, layouts...)
	if preflight.Slides[0].ShapeGrid == nil {
		t.Fatal("preflight did not expand KPI pattern")
	}
	preflightHeight := preflight.Slides[0].ShapeGrid.Rows[0].MaxHeight
	if diff := float64(maxHeight)/12700 - preflightHeight; diff < -1 || diff > 1 {
		t.Errorf("preflight KPI height %.1fpt disagrees with generated shape %.1fpt", preflightHeight, float64(maxHeight)/12700)
	}
	// Content-sized (go-slide-creator-wntyw): a value + caption card hugs
	// its text instead of taking 70% of the zone.
	if float64(maxHeight) > 0.45*float64(content.CY) {
		t.Errorf("KPI row height %.1fpt stretches past 45%% of template content height %.1fpt", float64(maxHeight)/12700, float64(content.CY)/12700)
	}

	// A deck rhythm grid can make the real render frame shorter still; the
	// content-sized card keeps its height on the tighter frame.
	rhythm := &resolvedGrid{TitleBaselineY: 1600000, ContentBottomY: 5741940, LeftMarginX: 838200, RightEdgeX: 11353800, SlideWidth: sw, SlideHeight: sh}
	_, tight := patternExpansionGeometry(slide, layouts, sw, sh, rhythm)
	if tight.CY != 3913340 {
		t.Fatalf("rhythm content height = %d, want 3913340", tight.CY)
	}
	tightSpecs, _, _, err := convertPresentationSlides([]SlideInput{slide}, layouts, sw, sh, nil, rhythm, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var tightMax int64
	for _, shape := range tightSpecs[0].RawShapeXML {
		for _, m := range re.FindAllSubmatch(shape, -1) {
			h, err := strconv.ParseInt(string(m[1]), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if h > tightMax {
				tightMax = h
			}
		}
	}
	if got, want := float64(tightMax)/12700, float64(maxHeight)/12700; got < want-1 || got > want+1 {
		t.Errorf("KPI card height = %.1fpt with 308.1pt render frame, want the content-sized %.1fpt", got, want)
	}
}

// contentRect is a 16:9 content area below a title (EMU).
var contentRect = pptx.RectEmu{X: 457200, Y: 1400000, CX: 11277600, CY: 4700000}

func resolvePatternForTest(t *testing.T, name, values string) (*ShapeGridResult, *ShapeGridInput) {
	t.Helper()
	ctx := patterns.ExpandContext{
		SlideWidth:   12192000,
		SlideHeight:  6858000,
		LayoutBounds: patterns.LayoutBounds{X: contentRect.X, Y: contentRect.Y, Width: contentRect.CX, Height: contentRect.CY},
	}
	grid, _, err := expandPattern(&PatternInput{Name: name, Values: json.RawMessage(values)}, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expand %s: %v", name, err)
	}
	b := contentRect
	res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("resolve %s: %v", name, err)
	}
	return res, grid
}

func blockExtent(cells []shapegrid.ResolvedCell) (top, bottom int64) {
	top, bottom = 1<<62, 0
	for _, c := range cells {
		if c.CellBounds.Y < top {
			top = c.CellBounds.Y
		}
		if e := c.CellBounds.Y + c.CellBounds.CY; e > bottom {
			bottom = e
		}
	}
	return top, bottom
}

// assertCentred checks the default "auto" placement of a content-sized
// block (go-slide-creator-e17xy): one that fills under 60% of the content
// zone hangs from its top, where native body text starts; a fuller one is
// centred (go-slide-creator-7km8).
func assertCentred(t *testing.T, name string, cells []shapegrid.ResolvedCell) {
	t.Helper()
	top, bottom := blockExtent(cells)
	above := top - contentRect.Y
	below := contentRect.Y + contentRect.CY - bottom
	topAnchored := above <= 12700 && above >= -12700
	d := above - below
	centred := d <= 12700 && d >= -12700
	fill := float64(bottom-top) / float64(contentRect.CY)
	switch {
	case fill < shapegrid.AutoCenterMinFill-0.02 && !topAnchored:
		t.Errorf("%s: short block (%.0f%%) not top-anchored (above=%d below=%d)", name, fill*100, above, below)
	case fill > shapegrid.AutoCenterMinFill+0.02 && !centred:
		t.Errorf("%s: full block (%.0f%%) not vertically centred (above=%d below=%d)", name, fill*100, above, below)
	case !topAnchored && !centred:
		t.Errorf("%s: block (%.0f%%) neither top-anchored nor centred (above=%d below=%d)", name, fill*100, above, below)
	}
}

// Full-size patterns take the auto placement in the content zone. Row lists
// (agenda, exec-summary) keep a minimum presence; box patterns (before-after,
// card-grid, and process-flow since go-slide-creator-xb06p) are content-sized
// and never stretched to fill (go-slide-creator-wntyw), so they carry a
// ceiling instead of a floor.
func TestFullPatternsOccupyContentZone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		min, max float64
	}{
		// kpi-4up figures sit on the 40pt KPI step (was 36pt, go-slide-creator-vmdfm);
		// the content-sized row is a little taller for it, not stretched.
		{"kpi-4up", 0, 0.65},
		{"process-flow", 0, 0.30},
		{"before-after", 0, 0.50},
		{"agenda", 0.59, 1},
		{"exec-summary", 0.40, 1},
		{"card-grid", 0, 0.55},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pat, ok := patterns.Default().Get(tc.name)
			if !ok {
				t.Fatal("pattern not registered")
			}
			ex, ok := pat.(patterns.Exemplar)
			if !ok {
				t.Fatal("pattern has no exemplar")
			}
			values, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				t.Fatal(err)
			}
			res, _ := resolvePatternForTest(t, tc.name, string(values))
			top, bottom := blockExtent(res.Cells)
			got := float64(bottom-top) / float64(contentRect.CY)
			if got < tc.min {
				t.Errorf("block fills %.1f%% of content zone, want at least %.1f%%", got*100, tc.min*100)
			}
			if got > tc.max {
				t.Errorf("block fills %.1f%% of content zone, want at most %.1f%% (content-sized, no stretch)", got*100, tc.max*100)
			}
			assertCentred(t, tc.name, res.Cells)
		})
	}
}

// TestKPI3up_CardFillsZoneAndCentred covers the full-size occupancy policy.
func TestKPI3up_CardFillsZoneAndCentred(t *testing.T) {
	res, grid := resolvePatternForTest(t, "kpi-3up", `[{"big":"$4.2M","small":"ARR added in H1","icon":"currency-dollar"},{"big":"127%","small":"Net revenue retention"},{"big":"12 days","small":"Median sales cycle"}]`)
	if grid.VerticalAlign != patterns.GridVerticalAlignDefault {
		t.Errorf("pattern grid vertical_align = %q, want %q", grid.VerticalAlign, patterns.GridVerticalAlignDefault)
	}
	for _, c := range res.Cells {
		if float64(c.CellBounds.CY) < 0.59*float64(contentRect.CY) {
			t.Errorf("KPI card height %d < 60%% of content height %d", c.CellBounds.CY, contentRect.CY)
		}
	}
	assertCentred(t, "kpi-3up", res.Cells)
}

// Process steps are content-sized boxes (go-slide-creator-xb06p): no taller
// than 30% of the content area, never shorter than a box, centred.
func TestProcessFlow_StepsContentSizedAndCentred(t *testing.T) {
	res, _ := resolvePatternForTest(t, "process-flow", `{"steps":[{"label":"Intake"},{"label":"Triage"},{"label":"Approve?","type":"decision"},{"label":"Close"}]}`)
	for _, c := range res.Cells {
		if h := float64(c.CellBounds.CY); h > 0.30*float64(contentRect.CY)+12700 || h < 0.35*float64(c.CellBounds.CX) {
			t.Errorf("process step %dx%d EMU, want content-sized within 30%% of the %d content height", c.CellBounds.CX, c.CellBounds.CY, contentRect.CY)
		}
	}
	assertCentred(t, "process-flow", res.Cells)
}

func TestProcessFlowCompactChevronResolvedGeometry(t *testing.T) {
	res, _ := resolvePatternForTest(t, "process-flow-compact", `{"steps":[{"label":"Baseline","type":"chevron"},{"label":"Review","type":"chevron"},{"label":"Approve","type":"chevron"},{"label":"Launch","type":"chevron"}]}`)
	if len(res.Cells) != 4 {
		t.Fatalf("resolved %d cells, want 4", len(res.Cells))
	}
	for i, cell := range res.Cells {
		if cell.Bounds.CY*2 > cell.Bounds.CX+12700 {
			t.Errorf("chevron %d is too tall: %dx%d EMU", i, cell.Bounds.CX, cell.Bounds.CY)
		}
		if cell.ShapeSpec == nil || cell.ShapeSpec.Geometry != "chevron" {
			t.Errorf("cell %d lost chevron geometry", i)
		}
	}
	// The compact band is top-anchored under the title (go-slide-creator-xb06p).
	if top, _ := blockExtent(res.Cells); top-contentRect.Y > 12700 {
		t.Errorf("process-flow-compact band starts %d EMU below the content top, want top-anchored", top-contentRect.Y)
	}
	shapes := 0
	for _, shape := range res.Shapes {
		if !bytes.Contains(shape, []byte(`prst="chevron"`)) {
			continue
		}
		shapes++
		// The preset's text rectangle clears the point and notch; the label
		// keeps the uniform 0.5 cm side margin inside it.
		if !bytes.Contains(shape, []byte(`lIns="180000"`)) || !bytes.Contains(shape, []byte(`rIns="180000"`)) {
			t.Errorf("chevron XML has wrong text insets: %s", shape)
		}
	}
	if shapes != 4 {
		t.Errorf("generated %d chevron shapes, want 4", shapes)
	}
}

// TestTimelineDots_AxisAndDots: the default dots style draws real dots joined
// by connector lines (the axis), not full-height filled pillars.
func TestTimelineDots_AxisAndDots(t *testing.T) {
	res, _ := resolvePatternForTest(t, "timeline-horizontal", `[{"label":"Kickoff","date":"Jan"},{"label":"Pilot","date":"Mar"},{"label":"Scale","date":"Jun"}]`)
	dots := 0
	for _, c := range res.Cells {
		if c.ShapeSpec != nil && c.ShapeSpec.Geometry == "ellipse" {
			dots++
			if c.Bounds.CX != c.Bounds.CY {
				t.Errorf("dot must be round, got %dx%d", c.Bounds.CX, c.Bounds.CY)
			}
		}
		if float64(c.CellBounds.CY) > 0.4*float64(contentRect.CY)+12700 {
			t.Errorf("timeline row height %d too tall", c.CellBounds.CY)
		}
	}
	if dots != 3 {
		t.Errorf("want 3 dots, got %d", dots)
	}
	cxn := 0
	for _, sh := range res.Shapes {
		if strings.Contains(string(sh), "<p:cxnSp") {
			cxn++
		}
	}
	if cxn != 2 {
		t.Errorf("want 2 axis connectors between 3 dots, got %d", cxn)
	}
	assertCentred(t, "timeline-horizontal", res.Cells)
}

// TestHeightCappedPatternBoundsCentred: pattern-authored top-anchored bounds
// (before-after-compact caps at 60%) are centred in the content area.
func TestHeightCappedPatternBoundsCentred(t *testing.T) {
	res, _ := resolvePatternForTest(t, "before-after-compact", `{"before":{"header":"Today","items":["Slow"]},"after":{"header":"Target","items":["Fast"]}}`)
	assertCentred(t, "before-after-compact", res.Cells)
}

// TestMaxHeightPctStaysTopAnchored: a user bounds override keeps its
// documented top-left anchoring.
func TestMaxHeightPctStaysTopAnchored(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, _, err := expandPattern(&PatternInput{Name: "kpi-3up", MaxHeightPct: 40, Values: json.RawMessage(`["1 | a","2 | b","3 | c"]`)}, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	b := contentRect
	res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if top, _ := blockExtent(res.Cells); top != contentRect.Y {
		t.Errorf("max_height_pct block must stay at the top: y=%d want %d", top, contentRect.Y)
	}
}

func TestResolveShapeGrid_InvalidVerticalAlign(t *testing.T) {
	in := &ShapeGridInput{VerticalAlign: "sideways", Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	if _, err := resolveShapeGrid(in, pptx.NewShapeIDAllocator(nil), nil, nil, 0, 0, nil); err == nil {
		t.Error("invalid vertical_align must error")
	}
	_, _, errs, _ := validateShapeGrid(in, 1)
	if len(errs) == 0 {
		t.Error("validate must report invalid vertical_align")
	}
}
