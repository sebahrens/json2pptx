package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestComposeContentSizedLeavesUseAllocatedSegments(t *testing.T) {
	ctx := patterns.ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340},
	}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["1 | A","2 | B","3 | C"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	full, _, err := expandPattern(&kpi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	fullHeight := full.Rows[0].MaxHeight
	for _, tc := range []struct {
		name     string
		compose  *ComposeInput
		maxShare float64
	}{
		{"explicit", &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{SizePct: 40, Pattern: kpi}, {SizePct: 60, Pattern: hero}}}, 0.40},
		{"smart", &ComposeInput{Direction: "vertical", SmartCompose: true, Segments: []SegmentInput{{Pattern: kpi}, {Pattern: hero}}}, 0.75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merged, _, err := expandCompose(tc.compose, ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			// Each vertical segment is a nested sub-grid in its own parent row.
			got := merged.Rows[0].Cells[0].Grid.Rows[0].MaxHeight
			allowed := float64(ctx.LayoutBounds.Height)/12700*tc.maxShare + 2
			// Content-sized KPI rows (go-slide-creator-wntyw) are the same
			// height on the full slide and in a segment that holds them.
			if got > fullHeight || got > allowed {
				t.Errorf("KPI row max height %.1fpt does not fit its %.0f%% vertical segment (full-slide %.1fpt)", got, tc.maxShare*100, fullHeight)
			}
		})
	}
}

func TestComposeHorizontalLeafTextFitsSegmentWidth(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Bookings","$987.65M | Revenue","$555.55M | Pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	full, _, err := expandPattern(&kpi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if firstCellParagraphSize(t, merged) >= firstCellParagraphSize(t, full) {
		t.Errorf("KPI value font did not shrink to fit the 50%% horizontal segment")
	}
}

func TestComposeHorizontalKeepsCompactCardBesideFullHeightHero(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["1 | A","2 | B","3 | C"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if merged.Rows[0].Cells[0].Grid.Rows[0].MaxHeight <= 0 {
		t.Fatal("horizontal merge lost the KPI row height cap")
	}
	if merged.Rows[0].Cells[1].Grid.Rows[0].MaxHeight != 0 {
		t.Fatal("KPI height cap leaked into the hero segment")
	}
	if occ := computeResolvedOccupancy(merged, true); occ == nil || occ.FilledSlots != occ.TotalSlots {
		t.Fatalf("horizontal segment spans should occupy all parent columns, got %+v", occ)
	}
	result, err := resolveShapeGrid(merged, pptx.NewShapeIDAllocator(nil), &pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}, nil, ctx.SlideWidth, ctx.SlideHeight, nil)
	if err != nil {
		t.Fatal(err)
	}
	var shapes []pptx.RectEmu
	for _, cell := range result.Cells {
		if cell.Kind == shapegrid.CellKindShape {
			shapes = append(shapes, cell.Bounds)
		}
	}
	if len(shapes) < 4 {
		t.Fatalf("expected three KPI cards and hero, got %d shapes", len(shapes))
	}
	card, full := shapes[0], shapes[3]
	// Content-sized (go-slide-creator-wntyw): the card hugs its value and
	// caption rather than stretching towards the segment height.
	if float64(card.CY) > 0.5*float64(full.CY) {
		t.Errorf("KPI card height %.1fpt stretches past half of the %.1fpt segment", float64(card.CY)/12700, float64(full.CY)/12700)
	}
	// A short card hangs from the segment top beside the hero, on the same
	// content line (auto placement, go-slide-creator-e17xy).
	if d := card.Y - full.Y; d < -12700 || d > 12700 || card.Y+card.CY >= full.Y+full.CY {
		t.Errorf("KPI card is not top-aligned within its segment: card=%+v hero=%+v", card, full)
	}
}

func TestComposeHorizontalSegmentsHaveIndependentRows(t *testing.T) {
	cell := func() *GridCellInput { return &GridCellInput{Shape: &ShapeSpecInput{Geometry: "rect"}} }
	left := &ShapeGridInput{Columns: json.RawMessage(`1`), VerticalAlign: "center", Rows: []GridRowInput{
		{MinHeight: 40, MaxHeight: 40, Cells: []*GridCellInput{cell()}},
		{MinHeight: 60, MaxHeight: 60, Cells: []*GridCellInput{cell()}},
	}}
	right := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{cell()}}}}
	merged, warnings, err := mergeHorizontal([]*ShapeGridInput{left, right}, []float64{50, 50}, 8)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("mergeHorizontal: %v, warnings: %v", err, warnings)
	}
	if len(merged.Rows) != 1 || len(merged.Rows[0].Cells) != 2 {
		t.Fatalf("horizontal compose should allocate one independent sub-grid per segment, got %+v", merged.Rows)
	}
	bounds := &pptx.RectEmu{X: 838200, Y: 1500000, CX: 10515600, CY: 3913340}
	resolved, err := resolveShapeGrid(merged, pptx.NewShapeIDAllocator(nil), bounds, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatal(err)
	}
	var shapes []pptx.RectEmu
	for _, c := range resolved.Cells {
		if c.Kind == shapegrid.CellKindShape {
			shapes = append(shapes, c.Bounds)
		}
	}
	if len(shapes) != 3 {
		t.Fatalf("expected two left rows and one right hero, got %d shapes", len(shapes))
	}
	if got := float64(shapes[0].CY) / 12700; got < 39 || got > 41 {
		t.Errorf("left first row height = %.1fpt, want 40pt", got)
	}
	if got := float64(shapes[1].CY) / 12700; got < 59 || got > 61 {
		t.Errorf("left second row height = %.1fpt, want 60pt", got)
	}
	if shapes[0].Y <= shapes[2].Y || shapes[1].Y+shapes[1].CY >= shapes[2].Y+shapes[2].CY {
		t.Errorf("left rows should be centered within the right segment's full height: %+v", shapes)
	}
	if shapes[0].Y+shapes[0].CY >= shapes[1].Y {
		t.Errorf("left rows overlap: %+v", shapes)
	}
}

func TestComposeHorizontalNestedTextIsCheckedByPreflight(t *testing.T) {
	longText, _ := json.Marshal(map[string]any{"content": strings.Repeat("word ", 900), "size": 14})
	left := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{MaxHeight: 35, Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect", Text: longText}}}}}}
	right := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	merged, _, err := mergeHorizontal([]*ShapeGridInput{left, right}, []float64{50, 50}, 8)
	if err != nil {
		t.Fatal(err)
	}
	findings := walkShapeGrid(SlideInput{ShapeGrid: merged}, 0, nil, 12192000, 6858000, nil)
	for _, f := range findings {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "/grid/rows/0/cells/0") {
			return
		}
	}
	t.Fatalf("nested segment text overflow missing from preflight: %+v", findings)
}

func TestGridCellAtResolvedSkipsEarlierRowSpans(t *testing.T) {
	want := &GridCellInput{Grid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}}
	grid := &ShapeGridInput{Columns: json.RawMessage(`2`), Rows: []GridRowInput{
		{Cells: []*GridCellInput{{RowSpan: 2, Shape: &ShapeSpecInput{Geometry: "rect"}}, {Shape: &ShapeSpecInput{Geometry: "rect"}}}},
		{Cells: []*GridCellInput{want}},
	}}
	if got := gridCellAtResolved(grid, 1, 1); got != want {
		t.Fatalf("resolved row 1 column 1 maps to %p, want nested cell %p", got, want)
	}
	right := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	merged, _, err := mergeHorizontal([]*ShapeGridInput{grid, right}, []float64{60, 40}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveShapeGrid(merged, pptx.NewShapeIDAllocator(nil), nil, nil, 12192000, 6858000, nil); err != nil {
		t.Fatalf("nested horizontal segment with a row span failed to resolve: %v", err)
	}
}

func TestComposeHorizontalNestedTableGetsPreflight(t *testing.T) {
	rows := make([][]TableCellInput, 30)
	for i := range rows {
		rows[i] = []TableCellInput{{Content: "Revenue"}}
	}
	left := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Table: &TableInput{Headers: []string{"Metric"}, Rows: rows}}}}}}
	right := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	merged, _, err := mergeHorizontal([]*ShapeGridInput{left, right}, []float64{50, 50}, 8)
	if err != nil {
		t.Fatal(err)
	}
	findings := collectGridTablePreflight(merged, 0)
	for _, f := range findings {
		if strings.Contains(f.Path, "/grid/rows/0/cells/0/table") {
			return
		}
	}
	t.Fatalf("nested table preflight missing: %+v", findings)
}

func TestComposeHorizontalNestedImageGetsAltFinding(t *testing.T) {
	left := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Image: &GridImageInput{Path: "/missing.png"}}}}}}
	right := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}
	merged, _, err := mergeHorizontal([]*ShapeGridInput{left, right}, []float64{50, 50}, 8)
	if err != nil {
		t.Fatal(err)
	}
	findings := gridAltFindings(0, merged, false)
	if len(findings) != 1 || !strings.Contains(findings[0].Path, "/grid/rows/0/cells/0/image") {
		t.Fatalf("nested image alt finding missing or misattributed: %+v", findings)
	}
}

func TestComposeIgnoresLeafBoundsBeforeSizing(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Bookings","$987.65M | Revenue","$555.55M | Pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	plain := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	baseline, _, err := expandCompose(plain, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	kpi.Bounds = &GridBoundsInput{X: 10, Y: 30, Width: 25, Height: 25}
	ignored := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	got, warnings, err := expandCompose(ignored, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(baseline)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Error("ignored segment bounds changed pattern sizing")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "COMPOSE_SEGMENT_BOUNDS_IGNORED") {
		t.Fatalf("missing ignored bounds warning: %v", warnings)
	}
}

func TestNestedComposeInheritsOuterSegmentBounds(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Bookings","$987.65M | Revenue","$555.55M | Pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	inner := &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	fullInner, _, err := expandCompose(inner, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	outer := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Compose: inner}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(outer, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if firstCellParagraphSize(t, merged) >= firstCellParagraphSize(t, fullInner) {
		t.Errorf("nested KPI value font did not shrink to fit its outer 50%% width allocation")
	}
}

// go-slide-creator-uhe09: a horizontal segment is its size_pct share of the
// envelope, however many lattice columns its pattern draws in. A gap used to
// be counted per column, so a ring lattice at 60% took two thirds of the
// width and its neighbour a quarter less than its 40%; and a pattern whose
// lattice changes between the probe and the sized expansion was told a width
// the merged grid did not give it.
func TestComposeHorizontalSegmentIsItsShareOfTheEnvelope(t *testing.T) {
	const widthPt, gapPt = 800.0, 12.0
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{Width: int64(widthPt * 12700), Height: 300 * 12700}}
	rows := PatternInput{Name: "labeled-rows", Values: json.RawMessage(`{"rows":[{"label":"WHY","body":"Renewals carry most of the revenue"},{"label":"HOW","body":"One loop owned by the account team"}]}`)}
	for name, circle := range map[string]PatternInput{
		"cycle-intake":     {Name: "cycle-intake", Values: json.RawMessage(`{"intake":[{"label":"Attract"},{"label":"Convert"},{"label":"Onboard"}],"loop":[{"label":"Activate"},{"label":"Engage"},{"label":"Renew"},{"label":"Expand"},{"label":"Refer"},{"label":"Assess"},{"label":"Reprice"},{"label":"Recommit"}]}`)},
		"cycle-ring":       {Name: "cycle-ring", Values: json.RawMessage(`{"phases":[{"label":"Forecast"},{"label":"Review"},{"label":"Decide"},{"label":"Execute"}]}`)},
		"concentric-rings": {Name: "concentric-rings", Values: json.RawMessage(`{"layers":[{"label":"Sponsor"},{"label":"Leadership team"},{"label":"Function heads"},{"label":"Front-line teams"},{"label":"Customers"}]}`)},
	} {
		t.Run(name, func(t *testing.T) {
			c := &ComposeInput{Direction: "horizontal", Gap: gapPt, Segments: []SegmentInput{{SizePct: 60, Pattern: circle}, {SizePct: 40, Pattern: rows}}}
			merged, _, err := expandCompose(c, ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			var cols []float64
			if err := json.Unmarshal(merged.Columns, &cols); err != nil {
				t.Fatal(err)
			}
			total := gapPt * float64(len(cols)-1)
			for _, w := range cols {
				total += w
			}
			at := 0
			for i, share := range []float64{0.6, 0.4} {
				span := max(merged.Rows[0].Cells[i].ColSpan, 1)
				got := gapPt * float64(span-1)
				for _, w := range cols[at : at+span] {
					got += w
				}
				at += span
				// The weights are proportional; scale them to the envelope.
				got *= widthPt / total
				if want := (widthPt - gapPt) * share; got < want-0.5 || got > want+0.5 {
					t.Errorf("segment %d spans %d columns and is %.1fpt wide, want its %.0f%% share: %.1fpt", i, span, got, share*100, want)
				}
			}
			if span := merged.Rows[0].Cells[0].ColSpan; span < 2 {
				t.Fatalf("the %s segment spans %d column: the case needs a lattice", name, span)
			}
			// The rectangle the pattern was expanded in is the one it is drawn in.
			probe, _, err := expandComposeSegments(c, ctx, nil, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			bounds := composeSegmentBounds(c, ctx, probe, []float64{60, 40})
			if got, want := float64(bounds[0].Width+2*subGridInsetEMU)/12700, (widthPt-gapPt)*0.6; got < want-0.5 || got > want+0.5 {
				t.Errorf("the pattern is expanded in %.1fpt, its segment is %.1fpt", got, want)
			}
		})
	}
}

func firstCellParagraphSize(t *testing.T, grid *ShapeGridInput) float64 {
	t.Helper()
	for grid.Rows[0].Cells[0].Grid != nil {
		grid = grid.Rows[0].Cells[0].Grid
	}
	var text struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &text); err != nil {
		t.Fatal(err)
	}
	if len(text.Paragraphs) == 0 {
		t.Fatal("cell has no text paragraphs")
	}
	return text.Paragraphs[0].Size
}
