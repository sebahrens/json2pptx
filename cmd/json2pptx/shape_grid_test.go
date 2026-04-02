package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func newAllocFrom(startID uint32) *pptx.ShapeIDAllocator {
	alloc := &pptx.ShapeIDAllocator{}
	alloc.SetMinID(startID)
	return alloc
}

func TestResolveColumnsDTO_Number(t *testing.T) {
	raw := json.RawMessage(`3`)
	cols, err := resolveColumnsDTO(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	for i, c := range cols {
		if c < 33.3 || c > 33.4 {
			t.Errorf("col %d: expected ~33.33, got %f", i, c)
		}
	}
}

func TestResolveColumnsDTO_Array(t *testing.T) {
	raw := json.RawMessage(`[30, 40, 30]`)
	cols, err := resolveColumnsDTO(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0] != 30 || cols[1] != 40 || cols[2] != 30 {
		t.Errorf("unexpected columns: %v", cols)
	}
}

func TestResolveColumnsDTO_InferFromRows(t *testing.T) {
	rows := []GridRowInput{
		{Cells: make([]*GridCellInput, 4)},
		{Cells: make([]*GridCellInput, 2)},
	}
	cols, err := resolveColumnsDTO(nil, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 4 {
		t.Fatalf("expected 4 columns (max from rows), got %d", len(cols))
	}
}

func TestResolveShapeGrid_Simple3Columns(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Trust"`)}},
				{Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"Quality"`)}},
				{Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"accent3"`), Text: json.RawMessage(`"Speed"`)}},
			},
		}},
	}

	alloc := newAllocFrom(200)
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Shapes) != 3 {
		t.Fatalf("expected 3 shapes, got %d", len(result.Shapes))
	}
	if len(result.Cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(result.Cells))
	}
	// After allocating 3 IDs starting from 200, next would be 203
	if alloc.NextID() != 203 {
		t.Errorf("expected nextID=203, got %d", alloc.NextID())
	}

	// Verify each shape contains expected geometry
	for i, s := range result.Shapes {
		xml := string(s)
		if !strings.Contains(xml, `prst="ellipse"`) {
			t.Errorf("shape %d: missing ellipse geometry", i)
		}
		if !strings.Contains(xml, `<p:sp>`) {
			t.Errorf("shape %d: missing p:sp open tag", i)
		}
	}
}

func TestResolveShapeGrid_ColSpan(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`[25, 25, 25, 25]`),
		Rows: []GridRowInput{
			{
				Height: 30,
				Cells: []*GridCellInput{
					{ColSpan: 4, Shape: &ShapeSpecInput{Geometry: "roundRect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Header"`)}},
				},
			},
			{
				Height: 70,
				Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`)}},
					{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent4"`)}},
				},
			},
		},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 5 {
		t.Fatalf("expected 5 shapes (1 header + 4 cells), got %d", len(result.Shapes))
	}
}

func TestResolveShapeGrid_NullCells(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				nil, // null cell
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`)}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 2 {
		t.Fatalf("expected 2 shapes (null cell skipped), got %d", len(result.Shapes))
	}
}

func TestResolveShapeGrid_EmptyCell(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
				{}, // empty cell (no shape)
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape (empty cell skipped), got %d", len(result.Shapes))
	}
}

func TestResolveShapeGrid_RowSpan(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{
			{Cells: []*GridCellInput{
				{RowSpan: 2, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
			}},
			{Cells: []*GridCellInput{
				// First column occupied by row_span from above
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent3"`)}},
			}},
		},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 3 {
		t.Fatalf("expected 3 shapes, got %d", len(result.Shapes))
	}
}

func TestResolveShapeGrid_ProcessDiagram(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`[20, 10, 20, 10, 20, 10, 20]`),
		Rows: []GridRowInput{
			{Height: 55, Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "roundRect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`{"content":"Design","bold":true,"color":"#FFF"}`)}},
				{Shape: &ShapeSpecInput{Geometry: "rightArrow", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "roundRect", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`{"content":"Develop","bold":true,"color":"#FFF"}`)}},
				{Shape: &ShapeSpecInput{Geometry: "rightArrow", Fill: json.RawMessage(`"accent2"`)}},
				{Shape: &ShapeSpecInput{Geometry: "roundRect", Fill: json.RawMessage(`"accent3"`), Text: json.RawMessage(`{"content":"Test","bold":true,"color":"#FFF"}`)}},
				{Shape: &ShapeSpecInput{Geometry: "rightArrow", Fill: json.RawMessage(`"accent3"`)}},
				{Shape: &ShapeSpecInput{Geometry: "roundRect", Fill: json.RawMessage(`"accent4"`), Text: json.RawMessage(`{"content":"Deploy","bold":true,"color":"#FFF"}`)}},
			}},
			{Height: 45, Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Figma\nPrototypes"`)}},
				nil,
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"Go\nServices"`)}},
				nil,
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"accent3"`), Text: json.RawMessage(`"CI/CD\nPipeline"`)}},
				nil,
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"accent4"`), Text: json.RawMessage(`"Kubernetes"`)}},
			}},
		},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 11 {
		t.Fatalf("expected 11 shapes, got %d", len(result.Shapes))
	}

	if !strings.Contains(string(result.Shapes[0]), `prst="roundRect"`) {
		t.Error("first shape should be roundRect")
	}
	if !strings.Contains(string(result.Shapes[1]), `prst="rightArrow"`) {
		t.Error("second shape should be rightArrow")
	}
}

func TestResolveShapeGrid_Rotation(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Rotation: 45}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(result.Shapes))
	}
	if !strings.Contains(string(result.Shapes[0]), `rot="2700000"`) {
		t.Errorf("expected rot=2700000 in shape XML, got:\n%s", string(result.Shapes[0]))
	}
}

func TestResolveShapeGrid_Adjustments(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{
					Geometry:    "roundRect",
					Adjustments: map[string]int64{"adj": 25000},
				}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(result.Shapes))
	}
	xml := string(result.Shapes[0])
	if !strings.Contains(xml, `name="adj"`) || !strings.Contains(xml, `val 25000`) {
		t.Errorf("expected adjustment adj=25000 in shape XML, got:\n%s", xml)
	}
}

func TestResolveShapeGrid_CustomBounds(t *testing.T) {
	grid := &ShapeGridInput{
		Bounds:  &GridBoundsInput{X: 10, Y: 20, Width: 80, Height: 60},
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(result.Shapes))
	}

	xml := string(result.Shapes[0])
	if !strings.Contains(xml, `x="1219200"`) {
		t.Errorf("expected x=1219200 (10%% of slide width), got:\n%s", xml)
	}
}

// Tests for shapegrid package functions used directly

func TestResolveFillString_Hex(t *testing.T) {
	fill := shapegrid.ResolveFillString("#4472C4")
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
}

func TestResolveFillString_Scheme(t *testing.T) {
	fill := shapegrid.ResolveFillString("accent1")
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
}

func TestResolveFillString_None(t *testing.T) {
	fill := shapegrid.ResolveFillString("none")
	if fill.IsZero() {
		t.Error("expected non-zero fill (noFill is still set)")
	}
}

func TestResolveLineInput_String(t *testing.T) {
	raw := json.RawMessage(`"accent1"`)
	line, err := shapegrid.ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if line.Width != 12700 {
		t.Errorf("expected default width 12700, got %d", line.Width)
	}
}

func TestResolveLineInput_Object(t *testing.T) {
	raw := json.RawMessage(`{"color":"#FF0000","width":2.5,"dash":"dash"}`)
	line, err := shapegrid.ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	expected := int64(2.5 * 12700)
	if line.Width != expected {
		t.Errorf("expected width %d, got %d", expected, line.Width)
	}
	if line.Dash != "dash" {
		t.Errorf("expected dash 'dash', got %q", line.Dash)
	}
}

func TestResolveTextInput_String(t *testing.T) {
	raw := json.RawMessage(`"Hello"`)
	tb, err := shapegrid.ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(tb.Paragraphs))
	}
	if tb.Paragraphs[0].Runs[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %q", tb.Paragraphs[0].Runs[0].Text)
	}
}

func TestResolveTextInput_Multiline(t *testing.T) {
	raw := json.RawMessage(`"Line1\nLine2"`)
	tb, err := shapegrid.ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(tb.Paragraphs))
	}
}

func TestResolveTextInput_Object(t *testing.T) {
	raw := json.RawMessage(`{"content":"Bold Title","size":16,"bold":true,"align":"ctr","color":"#FFFFFF"}`)
	tb, err := shapegrid.ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(tb.Paragraphs))
	}
	run := tb.Paragraphs[0].Runs[0]
	if run.Text != "Bold Title" {
		t.Errorf("expected 'Bold Title', got %q", run.Text)
	}
	if !run.Bold {
		t.Error("expected bold=true")
	}
	if run.FontSize != 1600 {
		t.Errorf("expected fontSize=1600, got %d", run.FontSize)
	}
}

func TestResolveFillInput_ObjectWithAlpha(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","alpha":20}`)
	fill, err := shapegrid.ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
}

func TestPctToEMU(t *testing.T) {
	// 50% of 12192000 = 6096000
	got := shapegrid.PctToEMU(50, 12192000)
	if got != 6096000 {
		t.Errorf("expected 6096000, got %d", got)
	}
}

func TestResolveVirtualLayout_BlankWithTitle(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout1",
			Name: "Content",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 8229600, Height: 461963}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 5029200}},
			},
		},
		{
			ID:   "layout2",
			Name: "Blank + Title",
			Tags: []string{"blank"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 8229600, Height: 461963}},
				{Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 457200, Y: 6356350, Width: 2895600, Height: 365125}},
			},
		},
	}

	result := resolveVirtualLayout(layouts, 0, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LayoutID != "layout2" {
		t.Errorf("expected layout2 (blank), got %s", result.LayoutID)
	}
	// Bounds should be between title bottom and footer top with 9pt gap
	titleBottom := int64(274638 + 461963) // 736601
	gapEMU := int64(9 * 12700)           // 114300
	expectedTop := titleBottom + gapEMU   // 850901
	if result.Bounds.Y != expectedTop {
		t.Errorf("expected Y=%d, got %d", expectedTop, result.Bounds.Y)
	}
}

func TestResolveVirtualLayout_BlankTitleFallback(t *testing.T) {
	// Only blank-title (synthesized), no native blank with title
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout1",
			Name: "Synthesized Blank + Title",
			Tags: []string{"blank-title", "virtual-base"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 8229600, Height: 461963}},
			},
		},
	}

	result := resolveVirtualLayout(layouts, 0, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LayoutID != "layout1" {
		t.Errorf("expected layout1, got %s", result.LayoutID)
	}
	// No footer → bottom boundary is slide height
	if result.Bounds.CY <= 0 {
		t.Error("expected positive grid height")
	}
}

func TestResolveVirtualLayout_ContentFallback(t *testing.T) {
	// No blank layouts, only content layout with body placeholder
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout1",
			Name: "Content",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 274638, Width: 8229600, Height: 461963}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 5029200}},
			},
		},
	}

	result := resolveVirtualLayout(layouts, 0, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LayoutID != "layout1" {
		t.Errorf("expected layout1, got %s", result.LayoutID)
	}
	// Bounds should match body placeholder exactly
	if result.Bounds.X != 457200 || result.Bounds.Y != 914400 {
		t.Errorf("expected bounds from body placeholder, got X=%d Y=%d", result.Bounds.X, result.Bounds.Y)
	}
	if result.Bounds.CX != 8229600 || result.Bounds.CY != 5029200 {
		t.Errorf("expected bounds from body placeholder, got CX=%d CY=%d", result.Bounds.CX, result.Bounds.CY)
	}
}

func TestResolveVirtualLayout_NoSuitableLayout(t *testing.T) {
	// Only title slide with no body
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout1",
			Name: "Title Slide",
			Tags: []string{"title"},
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 2000000, Width: 8229600, Height: 1000000}},
				{Type: types.PlaceholderSubtitle, Bounds: types.BoundingBox{X: 457200, Y: 3200000, Width: 8229600, Height: 800000}},
			},
		},
	}

	result := resolveVirtualLayout(layouts, 0, 0)
	if result != nil {
		t.Errorf("expected nil for layout with no blank/body, got %v", result)
	}
}

func TestNeedsVirtualLayout(t *testing.T) {
	grid := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{}}}}}

	tests := []struct {
		name     string
		slide    SlideInput
		expected bool
	}{
		{"no grid", SlideInput{}, false},
		{"grid no layout", SlideInput{ShapeGrid: grid}, true},
		{"grid blank type", SlideInput{ShapeGrid: grid, LayoutID: "x", SlideType: "blank"}, true},
		{"grid virtual type", SlideInput{ShapeGrid: grid, LayoutID: "x", SlideType: "virtual"}, true},
		{"grid explicit layout", SlideInput{ShapeGrid: grid, LayoutID: "x", SlideType: "content"}, false},
		{"grid explicit layout no type", SlideInput{ShapeGrid: grid, LayoutID: "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsVirtualLayout(tt.slide)
			if got != tt.expected {
				t.Errorf("needsVirtualLayout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResolveShapeGrid_OverrideBounds(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}

	override := &pptx.RectEmu{X: 500000, Y: 600000, CX: 7000000, CY: 4000000}
	result, err := resolveShapeGrid(grid, newAllocFrom(100), override, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(result.Shapes))
	}
	xml := string(result.Shapes[0])
	// Shape should use override bounds
	if !strings.Contains(xml, `x="500000"`) {
		t.Errorf("expected x=500000 from override bounds, got:\n%s", xml)
	}
	if !strings.Contains(xml, `y="600000"`) {
		t.Errorf("expected y=600000 from override bounds, got:\n%s", xml)
	}
}

func TestResolveShapeGrid_ExplicitBoundsOverrideOverride(t *testing.T) {
	// When input.Bounds is set, it takes precedence over overrideBounds
	grid := &ShapeGridInput{
		Bounds:  &GridBoundsInput{X: 10, Y: 20, Width: 80, Height: 60},
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}

	override := &pptx.RectEmu{X: 500000, Y: 600000, CX: 7000000, CY: 4000000}
	result, err := resolveShapeGrid(grid, newAllocFrom(100), override, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	xml := string(result.Shapes[0])
	// Should use input.Bounds (10% of slide width = 1219200), not override
	if !strings.Contains(xml, `x="1219200"`) {
		t.Errorf("expected x=1219200 from explicit bounds (not override), got:\n%s", xml)
	}
}

func TestResolveShapeGrid_IconCell(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Icon: &IconInput{Name: "abacus"}},
				{Icon: &IconInput{Name: "filled:search", Fill: "#FF0000"}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Icon cells produce no shapes — they go into IconInserts
	if len(result.Shapes) != 0 {
		t.Errorf("expected 0 shapes for icon-only grid, got %d", len(result.Shapes))
	}
	if len(result.IconInserts) != 2 {
		t.Fatalf("expected 2 icon inserts, got %d", len(result.IconInserts))
	}

	// First icon: outline/abacus.svg, no fill override
	if len(result.IconInserts[0].SVGData) == 0 {
		t.Error("icon 0: SVG data is empty")
	}
	if !strings.Contains(string(result.IconInserts[0].SVGData), "<svg") {
		t.Error("icon 0: SVG data doesn't contain <svg tag")
	}

	// Second icon: filled/search.svg with fill override
	svg1 := string(result.IconInserts[1].SVGData)
	if !strings.Contains(svg1, `fill="#FF0000"`) {
		t.Error("icon 1: expected fill=\"#FF0000\" in SVG")
	}

	// Both icons should have valid EMU bounds
	for i, ic := range result.IconInserts {
		if ic.ExtentCX <= 0 || ic.ExtentCY <= 0 {
			t.Errorf("icon %d: invalid extent: cx=%d cy=%d", i, ic.ExtentCX, ic.ExtentCY)
		}
	}

	// Resolved cells should have icon kind
	for i, cell := range result.Cells {
		if cell.Kind != shapegrid.CellKindIcon {
			t.Errorf("cell %d: expected kind %q, got %q", i, shapegrid.CellKindIcon, cell.Kind)
		}
	}
}

func TestResolveShapeGrid_MixedShapesAndIcons(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
				{Icon: &IconInput{Name: "abacus"}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Shapes) != 1 {
		t.Errorf("expected 1 shape, got %d", len(result.Shapes))
	}
	if len(result.IconInserts) != 1 {
		t.Errorf("expected 1 icon insert, got %d", len(result.IconInserts))
	}
}

func TestResolveIconSVG_NotFound(t *testing.T) {
	spec := &shapegrid.IconSpec{Name: "nonexistent-icon-xyz"}
	_, err := resolveIconSVG(spec)
	if err == nil {
		t.Fatal("expected error for nonexistent icon")
	}
}

func TestResolveIconSVG_FillOverride(t *testing.T) {
	tests := []struct {
		name      string
		spec      shapegrid.IconSpec
		wantFill  string // expected fill or stroke color in output
		wantNoErr bool
	}{
		{
			name:      "outline icon with red fill override",
			spec:      shapegrid.IconSpec{Name: "chart-pie", Fill: "#FF0000"},
			wantFill:  "#FF0000",
			wantNoErr: true,
		},
		{
			name:      "filled icon default color (no override)",
			spec:      shapegrid.IconSpec{Name: "filled:alert-circle"},
			wantNoErr: true,
		},
		{
			name:      "outline icon with blue fill override",
			spec:      shapegrid.IconSpec{Name: "users", Fill: "#4472C4"},
			wantFill:  "#4472C4",
			wantNoErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgData, err := resolveIconSVG(&tt.spec)
			if tt.wantNoErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(svgData)

			// Basic SVG structure check
			if !strings.Contains(s, "<svg") || !strings.Contains(s, "</svg>") {
				t.Fatal("SVG structure broken")
			}

			if tt.wantFill != "" {
				// The fill color should appear somewhere in the SVG tag (as fill= or stroke=)
				svgStart := strings.Index(s, "<svg")
				closeIdx := strings.Index(s[svgStart:], ">")
				tag := s[svgStart : svgStart+closeIdx]

				if !strings.Contains(tag, tt.wantFill) {
					t.Errorf("expected color %s in <svg> tag, got: %s", tt.wantFill, tag)
				}

				// Ensure no duplicate fill attributes
				if count := strings.Count(tag, ` fill="`); count > 1 {
					t.Errorf("duplicate fill attributes (%d) in <svg> tag: %s", count, tag)
				}
			}

			if tt.spec.Fill == "" {
				// Without override, SVG should be unmodified (contain original attributes)
				svgStart := strings.Index(s, "<svg")
				closeIdx := strings.Index(s[svgStart:], ">")
				tag := s[svgStart : svgStart+closeIdx]
				// Should still have original fill attribute (currentColor or similar)
				if !strings.Contains(tag, "fill=") {
					t.Errorf("expected original fill attribute preserved, got: %s", tag)
				}
			}
		})
	}
}

func TestApplyIconFill(t *testing.T) {
	tests := []struct {
		name     string
		svg      string
		fill     string
		wantFill string // expected fill attr value in <svg> tag
		wantStr  string // expected stroke attr value in <svg> tag (empty = don't check)
		noDupFill bool  // assert no duplicate fill attributes
	}{
		{
			name:     "no existing fill attr",
			svg:      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>`,
			fill:     "#00FF00",
			wantFill: "#00FF00",
		},
		{
			name:     "outline icon with fill=none and stroke=currentColor",
			svg:      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10 3.2"/></svg>`,
			fill:     "#4A90E2",
			wantFill: "none",            // fill="none" kept for outline icons
			wantStr:  `stroke="#4A90E2"`, // stroke recolored
			noDupFill: true,
		},
		{
			name:     "filled icon with fill=currentColor",
			svg:      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M10 3.2"/></svg>`,
			fill:     "#FF0000",
			wantFill: "#FF0000",
			noDupFill: true,
		},
		{
			name:     "replace existing non-none fill",
			svg:      `<svg xmlns="http://www.w3.org/2000/svg" fill="#000000" viewBox="0 0 24 24"><path/></svg>`,
			fill:     "#AABBCC",
			wantFill: "#AABBCC",
			noDupFill: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyIconFill([]byte(tt.svg), tt.fill)
			s := string(result)
			if !strings.Contains(s, fmt.Sprintf(`fill="%s"`, tt.wantFill)) {
				t.Errorf("expected fill=%q, got: %s", tt.wantFill, s)
			}
			if tt.wantStr != "" && !strings.Contains(s, tt.wantStr) {
				t.Errorf("expected %s in output, got: %s", tt.wantStr, s)
			}
			if tt.noDupFill {
				// Count fill= occurrences in the <svg> opening tag
				svgIdx := strings.Index(s, "<svg")
				closeIdx := strings.Index(s[svgIdx:], ">")
				tag := s[svgIdx : svgIdx+closeIdx]
				if count := strings.Count(tag, ` fill="`); count > 1 {
					t.Errorf("duplicate fill attributes (%d) in <svg> tag: %s", count, tag)
				}
			}
			if !strings.Contains(s, "<svg") || !strings.Contains(s, "</svg>") {
				t.Error("SVG structure broken after fill injection")
			}
		})
	}
}

func TestGenerateShapeFromSpec_MinimalRect(t *testing.T) {
	spec := &shapegrid.ShapeSpec{Geometry: "rect"}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}

	xml, err := shapegrid.GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `prst="rect"`) {
		t.Error("missing rect geometry")
	}
	if !strings.Contains(s, `<p:sp>`) || !strings.Contains(s, `</p:sp>`) {
		t.Error("missing p:sp element")
	}
}

func TestResolveShapeGrid_ImageCell(t *testing.T) {
	input := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
				{Image: &GridImageInput{Path: "/tmp/photo.jpg", Alt: "A photo"}},
			},
		}},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Shape cell produces XML, image cell produces an ImageInsert
	if len(result.Shapes) != 1 {
		t.Errorf("expected 1 shape XML fragment, got %d", len(result.Shapes))
	}
	if len(result.ImageInserts) != 1 {
		t.Fatalf("expected 1 image insert, got %d", len(result.ImageInserts))
	}
	img := result.ImageInserts[0]
	if img.Path != "/tmp/photo.jpg" {
		t.Errorf("expected path /tmp/photo.jpg, got %s", img.Path)
	}
	if img.Alt != "A photo" {
		t.Errorf("expected alt 'A photo', got %s", img.Alt)
	}
	if img.ExtentCX == 0 || img.ExtentCY == 0 {
		t.Error("expected non-zero image dimensions")
	}
}

func TestResolveShapeGrid_ImageWithOverlay(t *testing.T) {
	input := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Image: &GridImageInput{
					Path:    "/tmp/photo.jpg",
					Overlay: &GridOverlayInput{Color: "000000", Alpha: 0.4},
				}},
				{Image: &GridImageInput{
					Path: "/tmp/photo2.jpg",
				}},
			},
		}},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// 2 image inserts
	if len(result.ImageInserts) != 2 {
		t.Fatalf("expected 2 image inserts, got %d", len(result.ImageInserts))
	}
	// 1 overlay shape (only first image has overlay)
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape (overlay), got %d", len(result.Shapes))
	}
	xml := string(result.Shapes[0])
	if !strings.Contains(xml, `<p:sp>`) {
		t.Error("overlay should be a p:sp element")
	}
	// Should have alpha transparency
	if !strings.Contains(xml, `<a:alpha`) {
		t.Error("overlay should have alpha transparency")
	}
}

func TestResolveShapeGrid_ImageWithOverlayAndText(t *testing.T) {
	input := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Image: &GridImageInput{
					Path:    "/tmp/hero.jpg",
					Overlay: &GridOverlayInput{Color: "000000", Alpha: 0.5},
					Text:    &GridImageTextInput{Content: "Category Label", Size: 16, Bold: true, Color: "FFFFFF"},
				}},
			},
		}},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ImageInserts) != 1 {
		t.Fatalf("expected 1 image insert, got %d", len(result.ImageInserts))
	}
	// 2 shapes: overlay rect + text box
	if len(result.Shapes) != 2 {
		t.Fatalf("expected 2 shapes (overlay + text), got %d", len(result.Shapes))
	}

	// First shape is the overlay
	overlayXML := string(result.Shapes[0])
	if !strings.Contains(overlayXML, `<a:alpha`) {
		t.Error("overlay should have alpha transparency")
	}

	// Second shape is the text
	textXML := string(result.Shapes[1])
	if !strings.Contains(textXML, "Category Label") {
		t.Error("text shape should contain the label content")
	}
}

func TestResolveShapeGrid_ImageStripPattern(t *testing.T) {
	// 5-column image strip with overlays and labels (Bain slide 14 pattern)
	cells := make([]*GridCellInput, 5)
	labels := []string{"Geopolitical", "Social", "Economic", "Technology", "Environmental"}
	for i := range cells {
		cells[i] = &GridCellInput{
			Image: &GridImageInput{
				Path:    fmt.Sprintf("/tmp/img%d.jpg", i+1),
				Overlay: &GridOverlayInput{Color: "000000", Alpha: 0.4},
				Text:    &GridImageTextInput{Content: labels[i], Size: 14, Bold: true, Color: "FFFFFF"},
			},
		}
	}

	input := &ShapeGridInput{
		Columns: json.RawMessage(`5`),
		Rows:    []GridRowInput{{Height: 45, Cells: cells}},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ImageInserts) != 5 {
		t.Fatalf("expected 5 image inserts, got %d", len(result.ImageInserts))
	}
	// 5 overlays + 5 text boxes = 10 shapes
	if len(result.Shapes) != 10 {
		t.Fatalf("expected 10 shapes (5 overlays + 5 texts), got %d", len(result.Shapes))
	}

	// Verify each label is present
	for _, label := range labels {
		found := false
		for _, s := range result.Shapes {
			if strings.Contains(string(s), label) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find label %q in shapes", label)
		}
	}
}

func TestResolveShapeGrid_ImageOnlyGrid(t *testing.T) {
	input := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Image: &GridImageInput{Path: "/tmp/hero.png"}},
			},
		}},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Shapes) != 0 {
		t.Errorf("expected 0 shape XML fragments for image-only grid, got %d", len(result.Shapes))
	}
	if len(result.ImageInserts) != 1 {
		t.Fatalf("expected 1 image insert, got %d", len(result.ImageInserts))
	}
}

func TestResolveShapeGrid_SplitColumnImageRowSpan(t *testing.T) {
	// Bain slide 8 pattern: left column with 2 stacked text shapes,
	// right column with a single full-height image spanning both rows.
	input := &ShapeGridInput{
		Columns: json.RawMessage(`[55, 45]`),
		Gap:     2,
		Rows: []GridRowInput{
			{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{
					Geometry: "roundRect",
					Fill:     json.RawMessage(`"dk1"`),
					Text:     json.RawMessage(`"VOLATILITY\n\nDefinition text here"`),
				}},
				{RowSpan: 2, Image: &GridImageInput{
					Path:    "/tmp/ocean-waves.jpg",
					Alt:     "Ocean waves",
					Overlay: &GridOverlayInput{Color: "000000", Alpha: 0.3},
					Text:    &GridImageTextInput{Content: "Market Volatility", Size: 18, Bold: true, Color: "FFFFFF"},
				}},
			}},
			{Cells: []*GridCellInput{
				// col 1 occupied by row_span image
				{Shape: &ShapeSpecInput{
					Geometry: "roundRect",
					Fill:     json.RawMessage(`"accent1"`),
					Text:     json.RawMessage(`"KEY TAKEAWAY\n\nSummary text here"`),
				}},
			}},
		},
	}

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 1 image insert for the full-height photo
	if len(result.ImageInserts) != 1 {
		t.Fatalf("expected 1 image insert, got %d", len(result.ImageInserts))
	}
	if result.ImageInserts[0].Path != "/tmp/ocean-waves.jpg" {
		t.Errorf("expected image path /tmp/ocean-waves.jpg, got %s", result.ImageInserts[0].Path)
	}

	// 2 shape XMLs (overlay + text shapes for the shape cells) + 1 overlay + 1 text label = 4 shapes
	// Actually: 2 shape cells + 1 image overlay + 1 image text = 4 shapes
	if len(result.Shapes) != 4 {
		t.Fatalf("expected 4 shapes (2 shape cells + 1 overlay + 1 text), got %d", len(result.Shapes))
	}

	// 3 resolved cells (2 shape + 1 image)
	if len(result.Cells) != 3 {
		t.Fatalf("expected 3 resolved cells, got %d", len(result.Cells))
	}

	// The image cell should span both rows and be taller than shape cells
	var imageCell, shape1, shape2 *shapegrid.ResolvedCell
	for i := range result.Cells {
		switch result.Cells[i].Kind {
		case shapegrid.CellKindImage:
			imageCell = &result.Cells[i]
		case shapegrid.CellKindShape:
			if shape1 == nil {
				shape1 = &result.Cells[i]
			} else {
				shape2 = &result.Cells[i]
			}
		}
	}

	if imageCell == nil {
		t.Fatal("no image cell found")
	}
	if shape1 == nil || shape2 == nil {
		t.Fatal("expected 2 shape cells")
	}

	// Image should be taller than either single-row shape
	if imageCell.Bounds.CY <= shape1.Bounds.CY {
		t.Errorf("image cell height (%d) should be > shape cell height (%d)",
			imageCell.Bounds.CY, shape1.Bounds.CY)
	}

	// Verify overlay and text content in shapes XML
	found := false
	for _, s := range result.Shapes {
		if strings.Contains(string(s), "Market Volatility") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected image text label 'Market Volatility' in shapes")
	}
}

func TestResolveShapeGrid_WithConnectors(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{{
			Connector: &ConnectorSpecInput{Style: "arrow", Color: "FF0000", Width: 1.5, Dash: "dot"},
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "homePlate", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Phase 1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "homePlate", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"Phase 2"`)}},
				{Shape: &ShapeSpecInput{Geometry: "homePlate", Fill: json.RawMessage(`"accent3"`), Text: json.RawMessage(`"Phase 3"`)}},
			},
		}},
	}

	alloc := newAllocFrom(200)
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// 3 shapes + 2 connectors = 5 XML fragments
	if len(result.Shapes) != 5 {
		t.Fatalf("expected 5 shapes (3 cells + 2 connectors), got %d", len(result.Shapes))
	}

	// Last two shapes should be connectors (p:cxnSp)
	for i := 3; i < 5; i++ {
		xml := string(result.Shapes[i])
		if !strings.Contains(xml, "<p:cxnSp>") {
			t.Errorf("shape %d: expected p:cxnSp connector element, got:\n%s", i, xml)
		}
		if !strings.Contains(xml, `prst="straightConnector1"`) {
			t.Errorf("shape %d: expected straightConnector1 geometry", i)
		}
	}

	// First connector should have arrow tail end
	xml := string(result.Shapes[3])
	if !strings.Contains(xml, `<a:tailEnd type="triangle"`) {
		t.Errorf("first connector: expected triangle arrowhead, got:\n%s", xml)
	}
	// Should have dot dash
	if !strings.Contains(xml, `val="dot"`) {
		t.Errorf("first connector: expected dot dash, got:\n%s", xml)
	}
}

func TestResolveShapeGrid_LineConnector(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Connector: &ConnectorSpecInput{Style: "line"},
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
				{Shape: &ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(1), nil, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 2 shapes + 1 connector = 3
	if len(result.Shapes) != 3 {
		t.Fatalf("expected 3 shapes (2 cells + 1 connector), got %d", len(result.Shapes))
	}

	// Line style should NOT have arrowhead
	xml := string(result.Shapes[2])
	if strings.Contains(xml, "a:tailEnd") {
		t.Errorf("line connector should not have tailEnd arrowhead, got:\n%s", xml)
	}
}

func TestResolveIconSVG_CustomPath(t *testing.T) {
	// Create a temporary SVG file
	tmpDir := t.TempDir()
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect width="100" height="50" fill="blue"/></svg>`
	svgPath := filepath.Join(tmpDir, "custom.svg")
	if err := os.WriteFile(svgPath, []byte(svgContent), 0644); err != nil {
		t.Fatal(err)
	}

	spec := &shapegrid.IconSpec{Path: svgPath}
	data, err := resolveIconSVG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != svgContent {
		t.Errorf("expected SVG content to match, got: %s", string(data))
	}
}

func TestResolveIconSVG_CustomPathNotFound(t *testing.T) {
	spec := &shapegrid.IconSpec{Path: "/nonexistent/path/icon.svg"}
	_, err := resolveIconSVG(spec)
	if err == nil {
		t.Fatal("expected error for nonexistent custom icon path")
	}
}

func TestResolveIconPaths_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "icon.svg")
	if err := os.WriteFile(svgPath, []byte(`<svg/>`), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("both name and path is error", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "shield", Path: "icon.svg"},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err == nil {
			t.Fatal("expected error when both name and path are set")
		}
		if !strings.Contains(err.Error(), "exactly one") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("neither name nor path is error", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err == nil {
			t.Fatal("expected error when neither name nor path is set")
		}
	})

	t.Run("relative path resolved against baseDir", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "icon.svg"},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolved := slides[0].ShapeGrid.Rows[0].Cells[0].Icon.Path
		if !filepath.IsAbs(resolved) {
			t.Errorf("expected absolute path, got: %s", resolved)
		}
		// EvalSymlinks may resolve /var -> /private/var on macOS, so compare
		// the resolved form of the expected path too.
		wantResolved, _ := filepath.EvalSymlinks(svgPath)
		if resolved != wantResolved {
			t.Errorf("expected %s, got %s", wantResolved, resolved)
		}
	})

	t.Run("bundled icon name passes through", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "shield"},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if slides[0].ShapeGrid.Rows[0].Cells[0].Icon.Name != "shield" {
			t.Error("name should be unchanged")
		}
	})

	t.Run("nonexistent path file is error", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "nonexistent.svg"},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("icon on shape resolved", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Shape: &ShapeSpecInput{
							Geometry: "rect",
							Icon:     &IconInput{Path: "icon.svg"},
						},
					}},
				}},
			},
		}}
		err := resolveIconPaths(slides, tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolved := slides[0].ShapeGrid.Rows[0].Cells[0].Shape.Icon.Path
		if !filepath.IsAbs(resolved) {
			t.Errorf("expected absolute path, got: %s", resolved)
		}
	})
}
