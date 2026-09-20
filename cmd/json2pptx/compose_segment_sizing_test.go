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
			got := merged.Rows[0].MaxHeight
			allowed := fullHeight*tc.maxShare + 2
			if tc.maxShare < 0.5 {
				allowed = float64(ctx.LayoutBounds.Height)/12700*tc.maxShare + 2
			}
			if got >= fullHeight || got > allowed {
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
	if occ := computeResolvedOccupancy(merged); occ == nil || occ.FilledSlots != occ.TotalSlots {
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
	if card.CY >= full.CY/2 {
		t.Errorf("KPI card height %.1fpt is not compact beside %.1fpt hero", float64(card.CY)/12700, float64(full.CY)/12700)
	}
	if card.Y <= full.Y || card.Y+card.CY >= full.Y+full.CY {
		t.Errorf("KPI card is not centered within its segment: card=%+v hero=%+v", card, full)
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
