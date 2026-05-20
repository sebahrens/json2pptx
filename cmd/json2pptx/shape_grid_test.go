package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
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
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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
	gapEMU := int64(9 * 12700)            // 114300
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
	result, err := resolveShapeGrid(grid, newAllocFrom(100), override, nil, 0, 0, nil)
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
	result, err := resolveShapeGrid(grid, newAllocFrom(100), override, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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

func TestResolveShapeGrid_InlineSVGDataIconCell(t *testing.T) {
	// Inline svg_data simulates the agent piping render_diagram output directly
	// into a grid cell without a filesystem roundtrip.
	const inlineSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="#123456"/></svg>`
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Icon: &IconInput{SVGData: inlineSVG, Alt: "rendered pie chart"}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Shapes) != 0 {
		t.Errorf("expected 0 shapes, got %d", len(result.Shapes))
	}
	if len(result.IconInserts) != 1 {
		t.Fatalf("expected 1 icon insert, got %d", len(result.IconInserts))
	}
	ins := result.IconInserts[0]
	if string(ins.SVGData) != inlineSVG {
		t.Errorf("inline svg_data not preserved verbatim; got %q", string(ins.SVGData))
	}
	if ins.Alt != "rendered pie chart" {
		t.Errorf("explicit alt not propagated; got %q", ins.Alt)
	}
	if ins.ExtentCX <= 0 || ins.ExtentCY <= 0 {
		t.Errorf("invalid extent: cx=%d cy=%d", ins.ExtentCX, ins.ExtentCY)
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

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
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
		name      string
		svg       string
		fill      string
		wantFill  string // expected fill attr value in <svg> tag
		wantStr   string // expected stroke attr value in <svg> tag (empty = don't check)
		noDupFill bool   // assert no duplicate fill attributes
	}{
		{
			name:     "no existing fill attr",
			svg:      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>`,
			fill:     "#00FF00",
			wantFill: "#00FF00",
		},
		{
			name:      "outline icon with fill=none and stroke=currentColor",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10 3.2"/></svg>`,
			fill:      "#4A90E2",
			wantFill:  "none",             // fill="none" kept for outline icons
			wantStr:   `stroke="#4A90E2"`, // stroke recolored
			noDupFill: true,
		},
		{
			name:      "filled icon with fill=currentColor",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M10 3.2"/></svg>`,
			fill:      "#FF0000",
			wantFill:  "#FF0000",
			noDupFill: true,
		},
		{
			name:      "replace existing non-none fill",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg" fill="#000000" viewBox="0 0 24 24"><path/></svg>`,
			fill:      "#AABBCC",
			wantFill:  "#AABBCC",
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

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(input, newAllocFrom(100), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0, nil)
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

	result, err := resolveShapeGrid(input, newAllocFrom(1), nil, nil, 0, 0, nil)
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
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0, nil)
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

	// Connectors are emitted FIRST so they render BEHIND cell shapes in spTree
	// (otherwise the connector line shows through cell fills and labels).
	for i := 0; i < 2; i++ {
		xml := string(result.Shapes[i])
		if !strings.Contains(xml, "<p:cxnSp>") {
			t.Errorf("shape %d: expected p:cxnSp connector element, got:\n%s", i, xml)
		}
		if !strings.Contains(xml, `prst="straightConnector1"`) {
			t.Errorf("shape %d: expected straightConnector1 geometry", i)
		}
	}

	// First connector should have arrow tail end
	xml := string(result.Shapes[0])
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

	result, err := resolveShapeGrid(grid, newAllocFrom(1), nil, nil, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 2 shapes + 1 connector = 3
	if len(result.Shapes) != 3 {
		t.Fatalf("expected 3 shapes (2 cells + 1 connector), got %d", len(result.Shapes))
	}

	// Connector is emitted first (z-order: behind cells); line style should NOT have arrowhead.
	xml := string(result.Shapes[0])
	if !strings.Contains(xml, "<p:cxnSp>") {
		t.Errorf("expected first shape to be the connector p:cxnSp, got:\n%s", xml)
	}
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

func TestResolveIconSVG_CustomPathFillOverride(t *testing.T) {
	// Create a temporary SVG file with currentColor attributes
	tmpDir := t.TempDir()
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M12 2L2 7l10 5 10-5-10-5z"/></svg>`
	svgPath := filepath.Join(tmpDir, "custom.svg")
	if err := os.WriteFile(svgPath, []byte(svgContent), 0644); err != nil {
		t.Fatal(err)
	}

	spec := &shapegrid.IconSpec{Path: svgPath, Fill: "#FF6600"}
	data, err := resolveIconSVG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := string(data)
	if !strings.Contains(result, `stroke="#FF6600"`) {
		t.Errorf("expected stroke to be recolored to #FF6600, got: %s", result)
	}
	if strings.Contains(result, `stroke="currentColor"`) {
		t.Errorf("expected currentColor to be replaced, got: %s", result)
	}
}

func TestResolveIconSVG_CustomPathNoFill(t *testing.T) {
	// When Fill is empty, custom SVG should not be modified
	tmpDir := t.TempDir()
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" stroke="currentColor"><circle r="10"/></svg>`
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
		t.Errorf("expected SVG to be unmodified when Fill is empty, got: %s", string(data))
	}
}

func TestResolveIconSVG_InlineSVGData(t *testing.T) {
	// Inline svg_data should pass through verbatim without disk I/O,
	// and Fill should be ignored (agent supplies pre-styled SVG).
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="currentColor"/></svg>`
	spec := &shapegrid.IconSpec{SVGData: svg, Fill: "#FF0000"}
	data, err := resolveIconSVG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != svg {
		t.Errorf("inline svg_data should pass through unchanged; got %s", string(data))
	}
}

func TestIconAltText_PrefersExplicitAlt(t *testing.T) {
	cases := []struct {
		name string
		spec *shapegrid.IconSpec
		want string
	}{
		{"explicit alt wins over name", &shapegrid.IconSpec{Name: "shield", Alt: "Security icon"}, "Security icon"},
		{"explicit alt for inline svg", &shapegrid.IconSpec{SVGData: "<svg/>", Alt: "Pie chart: A 60%, B 40%"}, "Pie chart: A 60%, B 40%"},
		{"inline svg without alt falls back", &shapegrid.IconSpec{SVGData: "<svg/>"}, "icon"},
		{"name-derived alt", &shapegrid.IconSpec{Name: "shield"}, "shield icon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := iconAltText(tc.spec); got != tc.want {
				t.Errorf("iconAltText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveIconPaths_InlineSVGData(t *testing.T) {
	tmpDir := t.TempDir()
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`

	t.Run("svg_data alone passes validation", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{SVGData: svg, Alt: "inline diagram"},
					}},
				}},
			},
		}}
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("unexpected findings: %+v", findings)
		}
		if slides[0].ShapeGrid.Rows[0].Cells[0].Icon.SVGData != svg {
			t.Error("svg_data should be unchanged after path resolution")
		}
	})

	t.Run("svg_data with name is rejected", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{SVGData: svg, Name: "shield"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when both svg_data and name are set, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_AMBIGUOUS" {
			t.Errorf("expected ICON_AMBIGUOUS, got %s", findings[0].Code)
		}
		if !strings.Contains(findings[0].Message, "exactly one") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
	})

	t.Run("svg_data with fill emits non-blocking warning", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{SVGData: svg, Fill: "#FF0000"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for svg_data+fill, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_FILL_IGNORED_ON_INLINE" {
			t.Errorf("expected ICON_FILL_IGNORED_ON_INLINE, got %s", findings[0].Code)
		}
		if findings[0].Severity != diagnostics.SeverityWarning {
			t.Errorf("expected severity=warning (non-blocking), got %s", findings[0].Severity)
		}
		if !strings.Contains(findings[0].Message, "ignored") {
			t.Errorf("message should mention 'ignored': %s", findings[0].Message)
		}
		if !strings.Contains(findings[0].Message, "pre-style") {
			t.Errorf("message should include remediation hint 'pre-style': %s", findings[0].Message)
		}
	})

	t.Run("svg_data without fill emits no warning", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{SVGData: svg},
					}},
				}},
			},
		}}
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("expected 0 findings for svg_data alone, got %+v", findings)
		}
	})
}

// TestResolveIconPaths_AmbiguousMessageNamesConflictingFields verifies that the
// ICON_AMBIGUOUS error message names only the fields that were actually set,
// not all four sources. Naming the conflict makes the remediation obvious
// without forcing the agent to re-read the JSON to spot which fields collide.
func TestResolveIconPaths_AmbiguousMessageNamesConflictingFields(t *testing.T) {
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "icon.svg")
	if err := os.WriteFile(svgPath, []byte(`<svg/>`), 0644); err != nil {
		t.Fatal(err)
	}

	slides := []SlideInput{{
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{{
					Icon: &IconInput{Name: "shield", Path: "icon.svg"},
				}},
			}},
		},
	}}
	findings := resolveIconPaths(slides, tmpDir)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	d := findings[0]
	if d.Code != "ICON_AMBIGUOUS" {
		t.Fatalf("expected ICON_AMBIGUOUS, got %s", d.Code)
	}
	// The message should call out the specific conflicting fields up front,
	// before the rule restatement that names all four valid sources.
	if !strings.Contains(d.Message, "conflicting sources 'name', 'path'") {
		t.Errorf("message should call out conflict 'name', 'path' explicitly: %s", d.Message)
	}
	got, _ := d.Details["conflicting_fields"].([]string)
	if len(got) != 2 || got[0] != "name" || got[1] != "path" {
		t.Errorf("details.conflicting_fields = %v, want [name path]", got)
	}
}

// TestResolveIconPaths_MissingMessageHasExample verifies that ICON_MISSING
// includes a multi-line example block showing each of the four icon source
// shapes. Agents that hit this error need to pick a source and a copy-paste
// example is much faster than reading the schema.
func TestResolveIconPaths_MissingMessageHasExample(t *testing.T) {
	slides := []SlideInput{{
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{{Icon: &IconInput{}}},
			}},
		},
	}}
	findings := resolveIconPaths(slides, t.TempDir())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	d := findings[0]
	if d.Code != "ICON_MISSING" {
		t.Fatalf("expected ICON_MISSING, got %s", d.Code)
	}
	for _, key := range []string{`"name"`, `"path"`, `"url"`, `"svg_data"`} {
		if !strings.Contains(d.Message, key) {
			t.Errorf("missing example for %s in message: %s", key, d.Message)
		}
	}
	// Acceptance: 4-line example block.
	if strings.Count(d.Message, "\n") < 4 {
		t.Errorf("expected at least 4 newlines (one per example line), got message:\n%s", d.Message)
	}
}

func TestConvertGridCell_InlineSVGData(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"/>`
	cell := convertGridCell(&GridCellInput{
		Icon: &IconInput{SVGData: svg, Alt: "alt-text"},
	})
	if cell.Icon == nil {
		t.Fatal("expected Icon to be set")
	}
	if cell.Icon.SVGData != svg {
		t.Errorf("SVGData not propagated: got %q", cell.Icon.SVGData)
	}
	if cell.Icon.Alt != "alt-text" {
		t.Errorf("Alt not propagated: got %q", cell.Icon.Alt)
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
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when both name and path are set, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_AMBIGUOUS" {
			t.Errorf("expected ICON_AMBIGUOUS, got %s", findings[0].Code)
		}
		if !strings.Contains(findings[0].Message, "exactly one") {
			t.Errorf("unexpected message: %s", findings[0].Message)
		}
		if want := "/slides/0/shape_grid/rows/0/cells/0/icon"; findings[0].Path != want {
			t.Errorf("expected path %q, got %q", want, findings[0].Path)
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
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when neither name nor path is set, got %d", len(findings))
		}
		if findings[0].Code != "ICON_MISSING" {
			t.Errorf("expected ICON_MISSING, got %s", findings[0].Code)
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
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("unexpected findings: %+v", findings)
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
		// chart-pie exists in the outline set, so the bare name resolves.
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "chart-pie"},
					}},
				}},
			},
		}}
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("unexpected findings: %+v", findings)
		}
		if slides[0].ShapeGrid.Rows[0].Cells[0].Icon.Name != "chart-pie" {
			t.Error("name should be unchanged")
		}
	})

	t.Run("bundled icon name typo returns ICON_BUNDLED_NAME_UNKNOWN with suggestion", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "chart-pi"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for typo'd bundled name, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_BUNDLED_NAME_UNKNOWN" {
			t.Errorf("expected ICON_BUNDLED_NAME_UNKNOWN, got %s", findings[0].Code)
		}
		if findings[0].Severity != "error" {
			t.Errorf("expected error severity, got %s", findings[0].Severity)
		}
		if got := findings[0].Details["input_value"]; got != "chart-pi" {
			t.Errorf("expected input_value to round-trip, got %v", got)
		}
		suggestions, _ := findings[0].Details["suggestions"].([]string)
		if len(suggestions) == 0 {
			t.Fatalf("expected at least one suggestion, got none")
		}
		if suggestions[0] != "chart-pie" {
			t.Errorf("expected first suggestion 'chart-pie', got %q (full list %v)", suggestions[0], suggestions)
		}
		if !strings.Contains(findings[0].Message, "chart-pie") {
			t.Errorf("expected message to mention 'chart-pie' suggestion, got %q", findings[0].Message)
		}
	})

	t.Run("qualified bundled icon name typo returns ICON_BUNDLED_NAME_UNKNOWN", func(t *testing.T) {
		// 'filled:chart-pi' is a typo of 'filled:chart-pie' (which exists).
		// Catches the agent before generate so it can fix the qualified name.
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "filled:chart-pi"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for typo in qualified name, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_BUNDLED_NAME_UNKNOWN" {
			t.Errorf("expected ICON_BUNDLED_NAME_UNKNOWN, got %s", findings[0].Code)
		}
		suggestions, _ := findings[0].Details["suggestions"].([]string)
		if len(suggestions) == 0 || suggestions[0] != "filled:chart-pie" {
			t.Errorf("expected first suggestion 'filled:chart-pie', got %v", suggestions)
		}
	})

	t.Run("qualified filled name passes through", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "filled:chart-pie"},
					}},
				}},
			},
		}}
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("unexpected findings: %+v", findings)
		}
	})

	t.Run("nonexistent path file is ICON_NOT_FOUND", func(t *testing.T) {
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "nonexistent.svg"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for nonexistent file, got %d", len(findings))
		}
		if findings[0].Code != "ICON_NOT_FOUND" {
			t.Errorf("expected ICON_NOT_FOUND, got %s", findings[0].Code)
		}
		if got := findings[0].Details["input_value"]; got != "nonexistent.svg" {
			t.Errorf("expected input_value to round-trip, got %v", got)
		}
	})

	t.Run("non-svg extension is ICON_PATH_EXT_INVALID", func(t *testing.T) {
		// Even though the file does not exist, the extension check fires
		// first so the agent gets the most actionable diagnostic.
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "icon.png"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for non-svg extension, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_PATH_EXT_INVALID" {
			t.Errorf("expected ICON_PATH_EXT_INVALID, got %s", findings[0].Code)
		}
		if got := findings[0].Details["input_value"]; got != "icon.png" {
			t.Errorf("expected input_value to round-trip, got %v", got)
		}
	})

	t.Run("traversal in input path is ICON_PATH_TRAVERSAL", func(t *testing.T) {
		// "../escape.svg" must be rejected before filepath.Clean collapses
		// the ".." component, since the resolved absolute form would no
		// longer carry traversal intent.
		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "../escape.svg"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for traversal path, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_PATH_TRAVERSAL" {
			t.Errorf("expected ICON_PATH_TRAVERSAL, got %s", findings[0].Code)
		}
	})

	t.Run("symlink escape is ICON_PATH_SYMLINK_ESCAPE", func(t *testing.T) {
		// A relative path whose symlink chain resolves outside baseDir
		// should surface as a distinct code so agents know to pin an
		// absolute path or remove the offending symlink.
		outside := t.TempDir()
		outsideSVG := filepath.Join(outside, "outside.svg")
		if err := os.WriteFile(outsideSVG, []byte(`<svg/>`), 0644); err != nil {
			t.Fatal(err)
		}
		linkInside := filepath.Join(tmpDir, "escape-link.svg")
		if err := os.Symlink(outsideSVG, linkInside); err != nil {
			t.Skipf("symlink creation not supported: %v", err)
		}

		slides := []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "escape-link.svg"},
					}},
				}},
			},
		}}
		findings := resolveIconPaths(slides, tmpDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for symlink escape, got %d: %+v", len(findings), findings)
		}
		if findings[0].Code != "ICON_PATH_SYMLINK_ESCAPE" {
			t.Errorf("expected ICON_PATH_SYMLINK_ESCAPE, got %s", findings[0].Code)
		}
		if got := findings[0].Details["input_value"]; got != "escape-link.svg" {
			t.Errorf("expected input_value to round-trip, got %v", got)
		}
		if got := findings[0].Details["resolved_path"]; got == "" || got == nil {
			t.Errorf("expected resolved_path in details, got %v", got)
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
		if findings := resolveIconPaths(slides, tmpDir); len(findings) != 0 {
			t.Fatalf("unexpected findings: %+v", findings)
		}
		resolved := slides[0].ShapeGrid.Rows[0].Cells[0].Shape.Icon.Path
		if !filepath.IsAbs(resolved) {
			t.Errorf("expected absolute path, got: %s", resolved)
		}
	})
}

// TestResolveIconPaths_CollectsPerIconFindings verifies that a deck with
// multiple broken icons emits one structured finding per broken icon (not a
// single aggregated error), so agents can fix each one independently.
//
// Each finding's Path is a RFC 6901 JSON Pointer that resolves directly to
// the offending icon node.
func TestResolveIconPaths_CollectsPerIconFindings(t *testing.T) {
	tmpDir := t.TempDir()

	// Deck with three broken icons across two slides:
	//  - slide 0, row 0, cell 0: ambiguous (both name+path)
	//  - slide 0, row 1, cell 1: missing (no fields set), nested inside shape
	//  - slide 1, row 0, cell 0: nonexistent file
	slides := []SlideInput{
		{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{
					{Cells: []*GridCellInput{{Icon: &IconInput{Name: "shield", Path: "x.svg"}}}},
					{Cells: []*GridCellInput{
						nil,
						{Shape: &ShapeSpecInput{Geometry: "rect", Icon: &IconInput{}}},
					}},
				},
			},
		},
		{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{
					{Cells: []*GridCellInput{{Icon: &IconInput{Path: "missing.svg"}}}},
				},
			},
		},
	}

	findings := resolveIconPaths(slides, tmpDir)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings for 3 broken icons, got %d: %+v", len(findings), findings)
	}

	wantByPath := map[string]struct {
		code       string
		slideIndex int
	}{
		"/slides/0/shape_grid/rows/0/cells/0/icon":       {code: "ICON_AMBIGUOUS", slideIndex: 0},
		"/slides/0/shape_grid/rows/1/cells/1/shape/icon": {code: "ICON_MISSING", slideIndex: 0},
		"/slides/1/shape_grid/rows/0/cells/0/icon":       {code: "ICON_NOT_FOUND", slideIndex: 1},
	}
	seen := make(map[string]bool)
	for _, d := range findings {
		want, ok := wantByPath[d.Path]
		if !ok {
			t.Errorf("unexpected finding path %q (code=%s): %s", d.Path, d.Code, d.Message)
			continue
		}
		if seen[d.Path] {
			t.Errorf("duplicate finding for path %q", d.Path)
		}
		seen[d.Path] = true
		if d.Code != want.code {
			t.Errorf("path %q: expected code %s, got %s", d.Path, want.code, d.Code)
		}
		if d.Severity != "error" {
			t.Errorf("path %q: expected error severity, got %s", d.Path, d.Severity)
		}
		if idx, ok := d.Details["slide_index"].(int); !ok || idx != want.slideIndex {
			t.Errorf("path %q: expected slide_index %d, got %v", d.Path, want.slideIndex, d.Details["slide_index"])
		}
	}
	for p := range wantByPath {
		if !seen[p] {
			t.Errorf("missing finding for path %q", p)
		}
	}
}

// TestResolveIconPaths_JSONPathRoundTrip verifies each finding's json_path
// can be used to locate the exact icon node by walking the marshaled deck JSON.
// This is the test for the acceptance criterion "Each finding's json_path
// round-trips through jq/jsonpath to the exact icon node".
func TestResolveIconPaths_JSONPathRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	slides := []SlideInput{{
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{
				{Cells: []*GridCellInput{
					{Icon: &IconInput{Path: "nope.svg"}},
					{Shape: &ShapeSpecInput{Geometry: "rect", Icon: &IconInput{Path: "also-nope.svg"}}},
				}},
			},
		},
	}}

	findings := resolveIconPaths(slides, tmpDir)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	deckJSON, err := json.Marshal(map[string]any{"slides": slides})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var deck any
	if err := json.Unmarshal(deckJSON, &deck); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, d := range findings {
		node, err := jsonPointerResolve(deck, d.Path)
		if err != nil {
			t.Errorf("path %q failed to resolve: %v", d.Path, err)
			continue
		}
		obj, ok := node.(map[string]any)
		if !ok {
			t.Errorf("path %q: expected object node, got %T", d.Path, node)
			continue
		}
		if _, hasPath := obj["path"]; !hasPath {
			t.Errorf("path %q: resolved node missing 'path' field — does not point at icon: %v", d.Path, obj)
		}
	}
}

// jsonPointerResolve resolves a minimal RFC 6901 JSON Pointer against a
// decoded JSON value. Test helper — handles only the subset of pointers
// produced by resolveIconPaths (object keys and integer array indices).
func jsonPointerResolve(root any, pointer string) (any, error) {
	if pointer == "" {
		return root, nil
	}
	if pointer[0] != '/' {
		return nil, fmt.Errorf("pointer must start with '/'")
	}
	parts := strings.Split(pointer[1:], "/")
	cur := root
	for _, raw := range parts {
		// Unescape per RFC 6901: ~1 -> /, ~0 -> ~
		tok := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[tok]
			if !ok {
				return nil, fmt.Errorf("key %q not found at %q", tok, pointer)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("invalid array index %q at %q", tok, pointer)
			}
			cur = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse %q into %T at %q", tok, cur, pointer)
		}
	}
	return cur, nil
}

// TestResolveLocalAssetPaths_AllSurfaces verifies that the unified walker
// rewrites every supported relative asset reference (icon, content
// image_value, shape_grid cell image, slide background) to its absolute
// resolved form when the underlying files exist beneath baseDir. This is
// the acceptance test for go-slide-creator-tigj — agents that supply
// relative paths must see them resolve against base_dir on both CLI and
// MCP entry points.
func TestResolveLocalAssetPaths_AllSurfaces(t *testing.T) {
	tmpDir := t.TempDir()
	// Materialize one file per asset surface so we exercise the full
	// resolve + symlink + traversal pipeline (not just the early
	// "missing field" exits).
	iconFile := filepath.Join(tmpDir, "icon.svg")
	imageFile := filepath.Join(tmpDir, "photo.png")
	gridImageFile := filepath.Join(tmpDir, "grid.jpg")
	bgFile := filepath.Join(tmpDir, "bg.jpg")
	for _, p := range []string{iconFile, imageFile, gridImageFile, bgFile} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "bg.jpg"},
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: "photo.png", Alt: "ph"},
		}},
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{
					{Icon: &IconInput{Path: "icon.svg"}},
					{Image: &GridImageInput{Path: "grid.jpg", Alt: "grid"}},
				},
			}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid relative paths, got %d: %+v", len(findings), findings)
	}

	// Every Path/Image field must have been rewritten to an absolute path
	// rooted under tmpDir (allowing for /private symlink on macOS).
	wantResolved := func(p string) string {
		res, _ := filepath.EvalSymlinks(p)
		return res
	}
	if got := slides[0].Background.Image; got != wantResolved(bgFile) {
		t.Errorf("background: got %q, want %q", got, wantResolved(bgFile))
	}
	if got := slides[0].Content[0].ImageValue.Path; got != wantResolved(imageFile) {
		t.Errorf("image_value: got %q, want %q", got, wantResolved(imageFile))
	}
	if got := slides[0].ShapeGrid.Rows[0].Cells[0].Icon.Path; got != wantResolved(iconFile) {
		t.Errorf("icon: got %q, want %q", got, wantResolved(iconFile))
	}
	if got := slides[0].ShapeGrid.Rows[0].Cells[1].Image.Path; got != wantResolved(gridImageFile) {
		t.Errorf("grid image: got %q, want %q", got, wantResolved(gridImageFile))
	}
}

// TestResolveLocalAssetPaths_PerSurfaceFindings verifies that each missing
// asset surface emits its own structured finding with the correct
// diagnostic code and json_path so agents can locate and fix each broken
// reference independently.
func TestResolveLocalAssetPaths_PerSurfaceFindings(t *testing.T) {
	tmpDir := t.TempDir()

	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "missing-bg.jpg"},
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: "missing-photo.png"},
		}},
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{
					{Icon: &IconInput{Path: "missing-icon.svg"}},
					{Image: &GridImageInput{Path: "missing-grid.jpg"}},
				},
			}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings (icon, content image, grid image, background), got %d: %+v", len(findings), findings)
	}

	want := map[string]string{
		"/slides/0/shape_grid/rows/0/cells/0/icon":       "ICON_NOT_FOUND",
		"/slides/0/content/0/image_value/path":           "IMAGE_PATH",
		"/slides/0/shape_grid/rows/0/cells/1/image/path": "IMAGE_PATH",
		"/slides/0/background/image":                     "BACKGROUND_IMAGE_PATH",
	}
	got := make(map[string]string, len(findings))
	for _, d := range findings {
		got[d.Path] = d.Code
		if d.Severity != "error" {
			t.Errorf("path %q: expected error severity, got %s", d.Path, d.Severity)
		}
		if d.Details["input_value"] == nil {
			t.Errorf("path %q: expected input_value in details, got nil", d.Path)
		}
		if d.Details["slide_index"] != 0 {
			t.Errorf("path %q: expected slide_index 0, got %v", d.Path, d.Details["slide_index"])
		}
	}
	for path, code := range want {
		if got[path] != code {
			t.Errorf("path %q: expected %s, got %s", path, code, got[path])
		}
	}
}

// TestResolveLocalAssetPaths_ExtensionAllowlist confirms that image and
// background paths with unsupported extensions are rejected before disk
// I/O. This catches agent typos like `path: "report.pdf"` early instead of
// returning a confusing "file not found" later.
func TestResolveLocalAssetPaths_ExtensionAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	// File exists, but the extension is wrong.
	bad := filepath.Join(tmpDir, "report.pdf")
	if err := os.WriteFile(bad, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "report.pdf"},
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: "report.pdf"},
		}},
	}}

	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (background + image_value with bad ext), got %d: %+v", len(findings), findings)
	}
	for _, d := range findings {
		if !strings.Contains(d.Message, "unsupported extension") && !strings.Contains(d.Message, ".pdf") {
			t.Errorf("expected extension error for %q, got %s", d.Path, d.Message)
		}
	}
}

// TestResolveLocalAssetPaths_AbsolutePathsPassThrough verifies that
// pre-resolved absolute paths skip the baseDir join but still pass through
// symlink + traversal + extension validation.
func TestResolveLocalAssetPaths_AbsolutePathsPassThrough(t *testing.T) {
	tmpDir := t.TempDir()
	absImage := filepath.Join(tmpDir, "abs.png")
	if err := os.WriteFile(absImage, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	slides := []SlideInput{{
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: absImage},
		}},
	}}

	// baseDir is intentionally an unrelated path — absolute paths must not
	// be re-rooted under baseDir.
	otherDir := t.TempDir()
	findings := resolveLocalAssetPaths(slides, otherDir)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid absolute path, got %d: %+v", len(findings), findings)
	}
	resolved, _ := filepath.EvalSymlinks(absImage)
	if got := slides[0].Content[0].ImageValue.Path; got != resolved {
		t.Errorf("expected %s, got %s", resolved, got)
	}
}

// TestExpandAssetPath_TildeAndEnv verifies the pure helper expands "~/" to
// the user's home directory and resolves "$VAR"/"${VAR}" via the process
// environment, while reporting the first unset variable name when expansion
// would silently collapse a reference to an empty string.
func TestExpandAssetPath_TildeAndEnv(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	t.Setenv("JSON2PPTX_TEST_ASSETS", "/tmp/brand-assets")
	t.Setenv("JSON2PPTX_TEST_SUBDIR", "icons")
	os.Unsetenv("JSON2PPTX_DEFINITELY_UNSET_XYZ")

	cases := []struct {
		name      string
		in        string
		wantOut   string
		wantUnset string
	}{
		{name: "tilde slash", in: "~/icons/x.svg", wantOut: filepath.Join(home, "icons/x.svg")},
		{name: "bare tilde", in: "~", wantOut: home},
		{name: "embedded tilde untouched", in: "icons/~tmp/x.svg", wantOut: "icons/~tmp/x.svg"},
		{name: "dollar var", in: "$JSON2PPTX_TEST_ASSETS/icon.svg", wantOut: "/tmp/brand-assets/icon.svg"},
		{name: "braced var", in: "${JSON2PPTX_TEST_ASSETS}/icon.svg", wantOut: "/tmp/brand-assets/icon.svg"},
		{name: "tilde plus relative subdir var", in: "~/${JSON2PPTX_TEST_SUBDIR}/x.svg", wantOut: filepath.Join(home, "icons", "x.svg")},
		{name: "unset var", in: "$JSON2PPTX_DEFINITELY_UNSET_XYZ/icon.svg", wantUnset: "JSON2PPTX_DEFINITELY_UNSET_XYZ"},
		{name: "literal absolute passthrough", in: "/etc/passwd", wantOut: "/etc/passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, unset := expandAssetPath(tc.in)
			if tc.wantUnset != "" {
				if unset != tc.wantUnset {
					t.Fatalf("expected unsetVar %q, got %q (out=%q)", tc.wantUnset, unset, got)
				}
				if got != tc.in {
					t.Errorf("on unset var, expected out to echo raw input %q, got %q", tc.in, got)
				}
				return
			}
			if unset != "" {
				t.Fatalf("expected no unsetVar, got %q (out=%q)", unset, got)
			}
			if got != tc.wantOut {
				t.Errorf("expected %q, got %q", tc.wantOut, got)
			}
		})
	}
}

// TestResolveIconInputPath_TildeExpansion verifies that an icon.path beginning
// with "~/" resolves to a file under the user's home directory. The test
// hijacks HOME via t.Setenv so it can materialize a real SVG inside a temp
// directory without touching the actual home tree.
func TestResolveIconInputPath_TildeExpansion(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	iconDir := filepath.Join(fakeHome, "icons")
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		t.Fatal(err)
	}
	iconFile := filepath.Join(iconDir, "custom.svg")
	if err := os.WriteFile(iconFile, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0644); err != nil {
		t.Fatal(err)
	}

	icon := &IconInput{Path: "~/icons/custom.svg"}
	findings := resolveIconInputPath(icon, t.TempDir(), 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	resolved, _ := filepath.EvalSymlinks(iconFile)
	if icon.Path != resolved {
		t.Errorf("expected icon.Path=%q, got %q", resolved, icon.Path)
	}
}

// TestResolveIconInputPath_EnvExpansion verifies $VAR-style expansion. Setting
// an env var to a temp directory and supplying "$VAR/foo.svg" should resolve
// identically to "<tempdir>/foo.svg".
func TestResolveIconInputPath_EnvExpansion(t *testing.T) {
	assetDir := t.TempDir()
	iconFile := filepath.Join(assetDir, "logo.svg")
	if err := os.WriteFile(iconFile, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JSON2PPTX_BRAND_ASSETS_TEST", assetDir)

	for _, in := range []string{"$JSON2PPTX_BRAND_ASSETS_TEST/logo.svg", "${JSON2PPTX_BRAND_ASSETS_TEST}/logo.svg"} {
		icon := &IconInput{Path: in}
		findings := resolveIconInputPath(icon, t.TempDir(), 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
		if len(findings) != 0 {
			t.Fatalf("input %q: expected no findings, got %+v", in, findings)
		}
		resolved, _ := filepath.EvalSymlinks(iconFile)
		if icon.Path != resolved {
			t.Errorf("input %q: expected resolved=%q, got %q", in, resolved, icon.Path)
		}
	}
}

// TestResolveIconInputPath_UnsetEnvVar verifies that referencing a missing env
// var short-circuits with ASSET_PATH_ENV_UNSET instead of silently expanding
// to an empty path and producing a confusing "no such file" downstream.
func TestResolveIconInputPath_UnsetEnvVar(t *testing.T) {
	os.Unsetenv("JSON2PPTX_DEFINITELY_UNSET_TEST")

	icon := &IconInput{Path: "$JSON2PPTX_DEFINITELY_UNSET_TEST/icon.svg"}
	findings := resolveIconInputPath(icon, t.TempDir(), 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Code != diagnostics.CodeAssetPathEnvUnset {
		t.Errorf("expected code %s, got %s", diagnostics.CodeAssetPathEnvUnset, f.Code)
	}
	if f.Details["env_variable"] != "JSON2PPTX_DEFINITELY_UNSET_TEST" {
		t.Errorf("expected env_variable detail, got %v", f.Details["env_variable"])
	}
	if f.Details["input_value"] != "$JSON2PPTX_DEFINITELY_UNSET_TEST/icon.svg" {
		t.Errorf("expected input_value to echo raw input, got %v", f.Details["input_value"])
	}
}

// TestResolveIconInputPath_TraversalAfterExpansion verifies the post-expansion
// traversal check rejects "$VAR/foo" when VAR itself injects ".." components,
// so an unsuspecting baseDir cannot be escaped via env-supplied parent refs.
func TestResolveIconInputPath_TraversalAfterExpansion(t *testing.T) {
	t.Setenv("JSON2PPTX_TRAVERSAL_TEST", "/etc/../etc")
	icon := &IconInput{Path: "$JSON2PPTX_TRAVERSAL_TEST/icon.svg"}
	findings := resolveIconInputPath(icon, t.TempDir(), 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Code != diagnostics.CodeIconPathTraversal {
		t.Errorf("expected ICON_PATH_TRAVERSAL after expansion, got %s", findings[0].Code)
	}
}

// TestResolveLocalAssetPaths_TildeAndEnvExpansion verifies the unified asset
// walker honors tilde/env expansion for background images and content image
// paths the same way it does for icons, with ASSET_PATH_ENV_UNSET surfaced
// per offending asset surface.
func TestResolveLocalAssetPaths_TildeAndEnvExpansion(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	bgFile := filepath.Join(fakeHome, "bg.jpg")
	if err := os.WriteFile(bgFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	assetDir := t.TempDir()
	imageFile := filepath.Join(assetDir, "photo.png")
	if err := os.WriteFile(imageFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JSON2PPTX_BRAND_ASSETS_TEST2", assetDir)
	os.Unsetenv("JSON2PPTX_DEFINITELY_UNSET_TEST2")

	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "~/bg.jpg"},
		Content: []ContentInput{
			{PlaceholderID: "body", Type: "image", ImageValue: &ImageInput{Path: "${JSON2PPTX_BRAND_ASSETS_TEST2}/photo.png"}},
			{PlaceholderID: "body", Type: "image", ImageValue: &ImageInput{Path: "$JSON2PPTX_DEFINITELY_UNSET_TEST2/missing.png"}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, t.TempDir())
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 ASSET_PATH_ENV_UNSET finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Code != diagnostics.CodeAssetPathEnvUnset {
		t.Errorf("expected %s, got %s", diagnostics.CodeAssetPathEnvUnset, findings[0].Code)
	}
	if findings[0].Path != "/slides/0/content/1/image_value/path" {
		t.Errorf("unexpected json_path: %s", findings[0].Path)
	}
	if findings[0].Details["env_variable"] != "JSON2PPTX_DEFINITELY_UNSET_TEST2" {
		t.Errorf("expected env_variable=JSON2PPTX_DEFINITELY_UNSET_TEST2, got %v", findings[0].Details["env_variable"])
	}

	wantBG, _ := filepath.EvalSymlinks(bgFile)
	if slides[0].Background.Image != wantBG {
		t.Errorf("background: expected %q, got %q", wantBG, slides[0].Background.Image)
	}
	wantImg, _ := filepath.EvalSymlinks(imageFile)
	if slides[0].Content[0].ImageValue.Path != wantImg {
		t.Errorf("image_value: expected %q, got %q", wantImg, slides[0].Content[0].ImageValue.Path)
	}
}

// TestResolveLocalAssetPaths_SkipsURLOnlyImage confirms that an image_value
// with only URL set (Path empty) is left alone — resolveURLs handles
// remote fetches before the local-asset pass runs.
func TestResolveLocalAssetPaths_SkipsURLOnlyImage(t *testing.T) {
	tmpDir := t.TempDir()
	slides := []SlideInput{{
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{URL: "https://example.com/img.png"},
		}},
	}}
	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for URL-only image (Path empty), got: %+v", findings)
	}
}

// writeAssetOfSize creates a file of approximately size bytes (filled with a
// single byte) at the given path. Used to drive the asset-size cap tests
// without keeping huge files in the repo.
func writeAssetOfSize(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if size > 0 {
		// Truncate is the fastest way to fabricate a sparse file of the
		// requested length; the file system reports the requested size
		// via os.Stat.
		if err := f.Truncate(size); err != nil {
			t.Fatalf("truncate %s: %v", path, err)
		}
	}
}

// TestResolveIconInputPath_OversizedSVG_Warning verifies that an SVG icon
// between the soft and hard caps emits an ASSET_TOO_LARGE warning while still
// committing the resolved path so generation can proceed.
func TestResolveIconInputPath_OversizedSVG_Warning(t *testing.T) {
	// Force a tiny soft cap so we don't need a multi-megabyte test fixture.
	t.Setenv(envMaxSVGSoftBytes, "1024")
	t.Setenv(envMaxSVGHardBytes, "1048576")

	dir := t.TempDir()
	iconFile := filepath.Join(dir, "big.svg")
	writeAssetOfSize(t, iconFile, 4096) // 4 KB > 1 KB soft cap, < 1 MB hard cap

	icon := &IconInput{Path: iconFile}
	findings := resolveIconInputPath(icon, dir, 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	var got *diagnostics.Diagnostic
	for i := range findings {
		if findings[i].Code == diagnostics.CodeAssetTooLarge {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected ASSET_TOO_LARGE finding, got %+v", findings)
	}
	if got.Severity != diagnostics.SeverityWarning {
		t.Errorf("expected warning severity, got %s", got.Severity)
	}
	if got.Details["exceeded_cap"] != "soft" {
		t.Errorf("expected exceeded_cap=soft, got %v", got.Details["exceeded_cap"])
	}
	// Resolved path must still be committed so a soft-cap warning does not
	// block downstream generation.
	resolved, _ := filepath.EvalSymlinks(iconFile)
	if icon.Path != resolved {
		t.Errorf("expected icon.Path=%q after warning, got %q", resolved, icon.Path)
	}
}

// TestResolveIconInputPath_OversizedSVG_Blocking verifies that an SVG icon
// past the hard cap emits an ASSET_TOO_LARGE error-severity finding so the
// caller can refuse generation.
func TestResolveIconInputPath_OversizedSVG_Blocking(t *testing.T) {
	t.Setenv(envMaxSVGSoftBytes, "1024")
	t.Setenv(envMaxSVGHardBytes, "2048")

	dir := t.TempDir()
	iconFile := filepath.Join(dir, "huge.svg")
	writeAssetOfSize(t, iconFile, 8192) // 8 KB > 2 KB hard cap

	icon := &IconInput{Path: iconFile}
	findings := resolveIconInputPath(icon, dir, 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	var got *diagnostics.Diagnostic
	for i := range findings {
		if findings[i].Code == diagnostics.CodeAssetTooLarge {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected ASSET_TOO_LARGE finding, got %+v", findings)
	}
	if got.Severity != diagnostics.SeverityError {
		t.Errorf("expected error severity, got %s", got.Severity)
	}
	if got.Details["exceeded_cap"] != "hard" {
		t.Errorf("expected exceeded_cap=hard, got %v", got.Details["exceeded_cap"])
	}
}

// TestResolveIconInputPath_InlineSVGData_OverSoftCap verifies that inline
// svg_data markup is also checked against the SVG caps even though there is
// no file on disk to stat.
func TestResolveIconInputPath_InlineSVGData_OverSoftCap(t *testing.T) {
	t.Setenv(envMaxSVGSoftBytes, "256")
	t.Setenv(envMaxSVGHardBytes, "1048576")

	big := strings.Repeat("a", 512) // > 256-byte soft cap
	icon := &IconInput{SVGData: "<svg>" + big + "</svg>"}
	findings := resolveIconInputPath(icon, t.TempDir(), 0, "/slides/0/shape_grid/rows/0/cells/0/icon")
	var got *diagnostics.Diagnostic
	for i := range findings {
		if findings[i].Code == diagnostics.CodeAssetTooLarge {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected ASSET_TOO_LARGE finding for inline svg_data, got %+v", findings)
	}
	if got.Severity != diagnostics.SeverityWarning {
		t.Errorf("expected warning severity, got %s", got.Severity)
	}
	if got.Details["media_kind"] != "svg" {
		t.Errorf("expected media_kind=svg for inline svg_data, got %v", got.Details["media_kind"])
	}
}

// TestResolveLocalAssetPaths_OversizedRaster verifies that a PNG over the
// raster soft cap emits an ASSET_TOO_LARGE warning while still committing the
// resolved path. Confirms image_value path is checked.
func TestResolveLocalAssetPaths_OversizedRaster(t *testing.T) {
	t.Setenv(envMaxRasterSoftBytes, "1024")
	t.Setenv(envMaxRasterHardBytes, "1048576")

	dir := t.TempDir()
	imgFile := filepath.Join(dir, "photo.png")
	writeAssetOfSize(t, imgFile, 4096) // 4 KB > 1 KB soft cap

	slides := []SlideInput{{
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: imgFile},
		}},
	}}
	findings := resolveLocalAssetPaths(slides, dir)
	var got *diagnostics.Diagnostic
	for i := range findings {
		if findings[i].Code == diagnostics.CodeAssetTooLarge {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected ASSET_TOO_LARGE finding for oversized image, got %+v", findings)
	}
	if got.Severity != diagnostics.SeverityWarning {
		t.Errorf("expected warning severity, got %s", got.Severity)
	}
	if got.Details["media_kind"] != "raster" {
		t.Errorf("expected media_kind=raster, got %v", got.Details["media_kind"])
	}
	resolved, _ := filepath.EvalSymlinks(imgFile)
	if slides[0].Content[0].ImageValue.Path != resolved {
		t.Errorf("expected image_value.path=%q after warning, got %q", resolved, slides[0].Content[0].ImageValue.Path)
	}
}

// TestResolveLocalAssetPaths_OversizedRaster_Blocking verifies that a PNG
// past the raster hard cap emits an ASSET_TOO_LARGE error finding and that
// the resolved path is NOT committed (so generation refuses to embed).
func TestResolveLocalAssetPaths_OversizedRaster_Blocking(t *testing.T) {
	t.Setenv(envMaxRasterSoftBytes, "1024")
	t.Setenv(envMaxRasterHardBytes, "2048")

	dir := t.TempDir()
	imgFile := filepath.Join(dir, "huge.png")
	writeAssetOfSize(t, imgFile, 8192)

	slides := []SlideInput{{
		Content: []ContentInput{{
			PlaceholderID: "body",
			Type:          "image",
			ImageValue:    &ImageInput{Path: imgFile},
		}},
	}}
	rawInput := slides[0].Content[0].ImageValue.Path
	findings := resolveLocalAssetPaths(slides, dir)
	var got *diagnostics.Diagnostic
	for i := range findings {
		if findings[i].Code == diagnostics.CodeAssetTooLarge {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected ASSET_TOO_LARGE finding, got %+v", findings)
	}
	if got.Severity != diagnostics.SeverityError {
		t.Errorf("expected error severity, got %s", got.Severity)
	}
	if slides[0].Content[0].ImageValue.Path != rawInput {
		t.Errorf("hard-cap breach must not commit resolved path; original=%q, got %q",
			rawInput, slides[0].Content[0].ImageValue.Path)
	}
}

// TestResolveIconSVG_RejectsOversizedFile verifies the defense-in-depth check
// in resolveIconSVG: even if preflight is bypassed, the renderer-entry path
// must refuse to read a file beyond the hard cap rather than slurp it into
// memory.
func TestResolveIconSVG_RejectsOversizedFile(t *testing.T) {
	t.Setenv(envMaxSVGSoftBytes, "1024")
	t.Setenv(envMaxSVGHardBytes, "2048")

	dir := t.TempDir()
	iconFile := filepath.Join(dir, "huge.svg")
	writeAssetOfSize(t, iconFile, 8192)

	spec := &shapegrid.IconSpec{Path: iconFile}
	_, err := resolveIconSVG(spec)
	if err == nil {
		t.Fatal("expected error for oversized icon file, got nil")
	}
	if !strings.Contains(err.Error(), "hard cap") {
		t.Errorf("expected error to mention hard cap, got: %v", err)
	}
}

// TestCapabilitiesAssetLimits verifies get_capabilities advertises the
// current limits and the matching env-var names, and that env-var overrides
// flow through into the advertised numbers.
func TestCapabilitiesAssetLimits(t *testing.T) {
	t.Setenv(envMaxSVGSoftBytes, "12345")
	t.Setenv(envMaxRasterHardBytes, "99999999")

	resp := getCapabilitiesResult(t)
	al := resp.Features.AssetLimits
	if al.SVGSoftBytes != 12345 {
		t.Errorf("svg_soft_bytes override not honored: got %d, want 12345", al.SVGSoftBytes)
	}
	if al.RasterHardBytes != 99999999 {
		t.Errorf("raster_hard_bytes override not honored: got %d, want 99999999", al.RasterHardBytes)
	}
	if al.FindingCodeOnBreach != diagnostics.CodeAssetTooLarge {
		t.Errorf("finding_code_on_breach: got %q, want %q", al.FindingCodeOnBreach, diagnostics.CodeAssetTooLarge)
	}
	if al.SVGSoftEnv != envMaxSVGSoftBytes {
		t.Errorf("svg_soft_env mismatch: got %q, want %q", al.SVGSoftEnv, envMaxSVGSoftBytes)
	}
}

func TestResolveShapeGrid_DiagramCell(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Label"`)}},
				{Diagram: &types.DiagramSpec{
					Type: "pie",
					Data: map[string]any{
						"values": []any{60.0, 40.0},
						"labels": []any{"A", "B"},
					},
				}},
			},
		}},
	}

	result, err := resolveShapeGrid(grid, newAllocFrom(100), nil, nil, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// 1 shape XML for the rect cell, 0 for the diagram (diagram goes through IconInserts)
	if len(result.Shapes) != 1 {
		t.Fatalf("expected 1 shape XML (rect only), got %d", len(result.Shapes))
	}
	// Diagram cell should produce an IconInsert with SVG data
	if len(result.IconInserts) != 1 {
		t.Fatalf("expected 1 icon insert (diagram SVG), got %d", len(result.IconInserts))
	}
	if len(result.IconInserts[0].SVGData) == 0 {
		t.Error("diagram icon insert has empty SVG data")
	}
	if result.IconInserts[0].ExtentCX == 0 || result.IconInserts[0].ExtentCY == 0 {
		t.Error("diagram icon insert has zero dimensions")
	}
}

func TestIconAltText(t *testing.T) {
	tests := []struct {
		name string
		spec *shapegrid.IconSpec
		want string
	}{
		{name: "nil spec", spec: nil, want: ""},
		{name: "named icon", spec: &shapegrid.IconSpec{Name: "chart-pie"}, want: "chart-pie icon"},
		{name: "path icon", spec: &shapegrid.IconSpec{Path: "/tmp/icons/logo.svg"}, want: "logo.svg icon"},
		{name: "empty spec", spec: &shapegrid.IconSpec{}, want: "icon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := iconAltText(tt.spec)
			if got != tt.want {
				t.Errorf("iconAltText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveShapeGrid_GroupWrapsInGrpSp(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{Group: true, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Grouped"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"Ungrouped"`)}},
			},
		}},
	}

	alloc := newAllocFrom(100)
	result, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have 2 top-level fragments: one p:grpSp wrapping the grouped cell, one p:sp for the ungrouped cell
	if len(result.Shapes) != 2 {
		t.Fatalf("expected 2 top-level shapes, got %d", len(result.Shapes))
	}

	// First shape should be a group
	xml0 := string(result.Shapes[0])
	if !strings.Contains(xml0, "<p:grpSp>") {
		t.Error("grouped cell: expected <p:grpSp> wrapper")
	}
	if !strings.Contains(xml0, `prst="rect"`) {
		t.Error("grouped cell: expected child shape with rect geometry inside group")
	}
	if !strings.Contains(xml0, "</p:grpSp>") {
		t.Error("grouped cell: expected closing </p:grpSp>")
	}

	// Second shape should be a plain shape
	xml1 := string(result.Shapes[1])
	if strings.Contains(xml1, "<p:grpSp>") {
		t.Error("ungrouped cell: should not be wrapped in p:grpSp")
	}
	if !strings.Contains(xml1, "<p:sp>") {
		t.Error("ungrouped cell: expected plain <p:sp>")
	}
}

func TestResolveShapeGrid_OverlappingCellsValidationError(t *testing.T) {
	// Two cells in a 2-column grid both claim col_span=2, but row has 2 cells.
	// The second cell overlaps columns already occupied by the first.
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{ColSpan: 2, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`), Text: json.RawMessage(`"Wide"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`), Text: json.RawMessage(`"Overlap"`)}},
			},
		}},
	}

	alloc := newAllocFrom(100)
	_, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0, nil)
	if err == nil {
		t.Fatal("expected validation error for overlapping cells, got nil")
	}
	if !strings.Contains(err.Error(), "shape_grid validation") {
		t.Errorf("expected error to mention 'shape_grid validation', got: %s", err.Error())
	}
}

func TestResolveShapeGrid_RowSpanExceedsGrid(t *testing.T) {
	// Single row grid with a cell claiming row_span=3 — exceeds grid height.
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{
			Cells: []*GridCellInput{
				{RowSpan: 3, Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
				{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
			},
		}},
	}

	alloc := newAllocFrom(100)
	_, err := resolveShapeGrid(grid, alloc, nil, nil, 0, 0, nil)
	if err == nil {
		t.Fatal("expected validation error for row_span exceeding grid, got nil")
	}
	if !strings.Contains(err.Error(), "shape_grid validation") {
		t.Errorf("expected error to mention 'shape_grid validation', got: %s", err.Error())
	}
}

func TestDiagramCellInserts_ThemeColorInjection(t *testing.T) {
	cell := shapegrid.ResolvedCell{
		ID:   1,
		Kind: shapegrid.CellKindDiagram,
		Bounds: pptx.RectEmu{
			X: 100000, Y: 100000, CX: 5000000, CY: 3000000,
		},
		DiagramSpec: &types.DiagramSpec{
			Type:  "bar_chart",
			Title: "Test Chart",
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"series":     []any{map[string]any{"name": "S1", "values": []any{1.0, 2.0}}},
			},
		},
	}

	themeColors := []types.ThemeColor{
		{Name: "accent1", RGB: "FF0000"},
		{Name: "accent2", RGB: "00FF00"},
	}
	dataPalette := []string{"#FF0000", "#00FF00"}
	diagCtx := &GridDiagramContext{
		ThemeColors: themeColors,
		DataPalette: dataPalette,
		SlideNum:    1,
	}

	icons, warnings, err := generateDiagramCellInserts(cell, diagCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon insert, got %d", len(icons))
	}
	if len(icons[0].SVGData) == 0 {
		t.Error("expected non-empty SVG data")
	}
	// Theme injection must operate on a local clone — the caller's spec
	// keeps its original (nil) Style so the same DiagramSpec can be reused
	// across cells/slides/retries without hidden statefulness
	// (go-slide-creator-zg8q.7).
	if cell.DiagramSpec.Style != nil {
		t.Errorf("caller's DiagramSpec.Style was mutated: %+v (expected nil)", cell.DiagramSpec.Style)
	}
	// No narrow-cell warnings for a 5M EMU wide cell
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestDiagramCellInserts_NarrowCellWarning(t *testing.T) {
	cell := shapegrid.ResolvedCell{
		ID:   1,
		Kind: shapegrid.CellKindDiagram,
		Bounds: pptx.RectEmu{
			X: 100000, Y: 100000, CX: 3000000, CY: 3000000, // ~25% of slide width — narrow
		},
		DiagramSpec: &types.DiagramSpec{
			Type:  "org_chart",
			Title: "Large Org",
			Data: map[string]any{
				"root": map[string]any{
					"name": "CEO",
					"children": []any{
						map[string]any{"name": "VP1", "children": []any{
							map[string]any{"name": "D1"},
							map[string]any{"name": "D2"},
							map[string]any{"name": "D3"},
						}},
						map[string]any{"name": "VP2", "children": []any{
							map[string]any{"name": "D4"},
							map[string]any{"name": "D5"},
							map[string]any{"name": "D6"},
						}},
					},
				},
			},
		},
	}

	diagCtx := &GridDiagramContext{SlideNum: 3}
	icons, warnings, err := generateDiagramCellInserts(cell, diagCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon insert, got %d", len(icons))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for complex org_chart in narrow cell, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "complex org_chart") {
		t.Errorf("warning should mention complex org_chart, got: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "slide 3") {
		t.Errorf("warning should mention slide 3, got: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "grid cell 1") {
		t.Errorf("warning should mention grid cell 1, got: %s", warnings[0])
	}
}

func TestDiagramCellInserts_ForwardsCellBoundsToDiagramSpec(t *testing.T) {
	// Regression for go-slide-creator-zg8q.4: grid-cell diagrams must forward
	// the cell's EMU bounds into svggen so it renders into the target aspect
	// ratio instead of the default 800x600.
	//
	// Updated for go-slide-creator-zg8q.7: the function now clones the spec
	// before injecting dimensions, so we verify the aspect ratio reaches the
	// renderer by parsing the SVG viewBox rather than inspecting the caller's
	// (now-unmutated) DiagramSpec.
	const cxEMU int64 = 5_000_000 // ~393.7 pt
	const cyEMU int64 = 1_500_000 // ~118.1 pt (wide non-default aspect)
	cell := shapegrid.ResolvedCell{
		ID:   7,
		Kind: shapegrid.CellKindDiagram,
		Bounds: pptx.RectEmu{
			X: 100_000, Y: 100_000, CX: cxEMU, CY: cyEMU,
		},
		DiagramSpec: &types.DiagramSpec{
			Type: "line_chart",
			Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{1.0, 2.0, 3.0}},
				},
			},
		},
	}

	icons, _, err := generateDiagramCellInserts(cell, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon insert, got %d", len(icons))
	}

	// Caller's spec must be byte-identical post-call (no Width/Height mutation).
	if cell.DiagramSpec.Width != 0 || cell.DiagramSpec.Height != 0 {
		t.Errorf("caller DiagramSpec dimensions mutated: got %dx%d, want 0x0",
			cell.DiagramSpec.Width, cell.DiagramSpec.Height)
	}

	// Aspect ratio derived from cell EMU must propagate into the rendered SVG.
	cellAspect := float64(cxEMU) / float64(cyEMU)
	svg := string(icons[0].SVGData)
	vbRe := regexp.MustCompile(`viewBox="0 0 ([0-9.]+) ([0-9.]+)"`)
	m := vbRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("could not find viewBox in rendered SVG (first 200 chars): %s", svg[:min(200, len(svg))])
	}
	vbW, _ := strconv.ParseFloat(m[1], 64)
	vbH, _ := strconv.ParseFloat(m[2], 64)
	if vbH == 0 {
		t.Fatalf("viewBox height is zero in: %s", m[0])
	}
	svgAspect := vbW / vbH
	if diff := svgAspect - cellAspect; diff > 0.02 || diff < -0.02 {
		t.Errorf("aspect ratio mismatch: cell=%.3f svg=%.3f (viewBox %.0fx%.0f)", cellAspect, svgAspect, vbW, vbH)
	}
}

// TestDiagramCellInserts_PropagatesGroupFlag is a regression for
// go-slide-creator-zg8q.10: when cell.Group=true on a diagram cell, the
// resulting IconInsert must carry Group=true so the singlepass emitter wraps
// the p:pic in a p:grpSp (single PowerPoint selection target).
func TestDiagramCellInserts_PropagatesGroupFlag(t *testing.T) {
	mkCell := func(group bool) shapegrid.ResolvedCell {
		return shapegrid.ResolvedCell{
			ID:   1,
			Kind: shapegrid.CellKindDiagram,
			Bounds: pptx.RectEmu{
				X: 100000, Y: 100000, CX: 5000000, CY: 3000000,
			},
			Group: group,
			DiagramSpec: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"categories": []any{"A", "B"},
					"series":     []any{map[string]any{"name": "S1", "values": []any{1.0, 2.0}}},
				},
			},
		}
	}

	grouped := mkCell(true)
	icons, _, err := generateDiagramCellInserts(grouped, nil)
	if err != nil {
		t.Fatalf("unexpected error (grouped): %v", err)
	}
	if len(icons) != 1 || !icons[0].Group {
		t.Fatalf("expected Group=true on IconInsert when cell.Group=true, got icons=%+v", icons)
	}

	bare := mkCell(false)
	icons, _, err = generateDiagramCellInserts(bare, nil)
	if err != nil {
		t.Fatalf("unexpected error (bare): %v", err)
	}
	if len(icons) != 1 || icons[0].Group {
		t.Fatalf("expected Group=false on IconInsert when cell.Group=false, got icons=%+v", icons)
	}
}

func TestDiagramCellInserts_PreservesExplicitDiagramDimensions(t *testing.T) {
	// When DiagramSpec.Width/Height are already set, generateDiagramCellInserts
	// must NOT overwrite them with cell-derived dimensions.
	cell := shapegrid.ResolvedCell{
		ID:   8,
		Kind: shapegrid.CellKindDiagram,
		Bounds: pptx.RectEmu{
			X: 0, Y: 0, CX: 5_000_000, CY: 3_000_000,
		},
		DiagramSpec: &types.DiagramSpec{
			Type:   "bar_chart",
			Width:  640,
			Height: 480,
			Data: map[string]any{
				"categories": []any{"A", "B"},
				"series":     []any{map[string]any{"name": "S1", "values": []any{1.0, 2.0}}},
			},
		},
	}

	if _, _, err := generateDiagramCellInserts(cell, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cell.DiagramSpec.Width != 640 || cell.DiagramSpec.Height != 480 {
		t.Errorf("explicit dimensions overwritten: got %dx%d, want 640x480",
			cell.DiagramSpec.Width, cell.DiagramSpec.Height)
	}
}

func TestDiagramCellInserts_NilContext(t *testing.T) {
	cell := shapegrid.ResolvedCell{
		ID:   1,
		Kind: shapegrid.CellKindDiagram,
		Bounds: pptx.RectEmu{
			X: 100000, Y: 100000, CX: 5000000, CY: 3000000,
		},
		DiagramSpec: &types.DiagramSpec{
			Type:  "pie_chart",
			Title: "Test Pie",
			Data: map[string]any{
				"labels": []any{"A", "B"},
				"values": []any{60.0, 40.0},
			},
		},
	}

	// With nil context, diagram should still render (just without theme colors)
	icons, warnings, err := generateDiagramCellInserts(cell, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon insert, got %d", len(icons))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings with nil context, got: %v", warnings)
	}
}

// TestDiagramCellInserts_DoesNotMutateCallerSpec is the regression test for
// go-slide-creator-zg8q.7: reusing the same *types.DiagramSpec across cells
// with different GridDiagramContext values must produce independent SVGs and
// must leave the caller's struct byte-identical to the pre-call state. Prior
// to the fix, the first call's theme colors and EMU-derived dimensions leaked
// into the spec and silently contaminated subsequent renders.
func TestDiagramCellInserts_DoesNotMutateCallerSpec(t *testing.T) {
	sharedSpec := &types.DiagramSpec{
		Type:  "bar_chart",
		Title: "Shared",
		Data: map[string]any{
			"categories": []any{"A", "B"},
			"series":     []any{map[string]any{"name": "S1", "values": []any{1.0, 2.0}}},
		},
	}
	// Capture pre-call state for byte-identical comparison.
	preCall := *sharedSpec
	preStyle := sharedSpec.Style // nil

	cellA := shapegrid.ResolvedCell{
		ID:          1,
		Kind:        shapegrid.CellKindDiagram,
		Bounds:      pptx.RectEmu{X: 0, Y: 0, CX: 5_000_000, CY: 3_000_000},
		DiagramSpec: sharedSpec,
	}
	ctxA := &GridDiagramContext{
		ThemeColors: []types.ThemeColor{{Name: "accent1", RGB: "FF0000"}},
		DataPalette: []string{"#FF0000"},
		SlideNum:    1,
	}

	cellB := shapegrid.ResolvedCell{
		ID:          2,
		Kind:        shapegrid.CellKindDiagram,
		Bounds:      pptx.RectEmu{X: 0, Y: 0, CX: 2_000_000, CY: 4_000_000},
		DiagramSpec: sharedSpec,
	}
	ctxB := &GridDiagramContext{
		ThemeColors: []types.ThemeColor{{Name: "accent1", RGB: "0000FF"}},
		DataPalette: []string{"#0000FF"},
		SlideNum:    2,
	}

	iconsA, _, err := generateDiagramCellInserts(cellA, ctxA)
	if err != nil {
		t.Fatalf("cellA render: %v", err)
	}
	if len(iconsA) != 1 || len(iconsA[0].SVGData) == 0 {
		t.Fatalf("cellA: expected one non-empty SVG, got %d icons", len(iconsA))
	}

	// After the first call, caller's spec must be untouched.
	if sharedSpec.Style != preStyle {
		t.Errorf("call A mutated caller's Style: was %v, now %v", preStyle, sharedSpec.Style)
	}
	if sharedSpec.Width != preCall.Width || sharedSpec.Height != preCall.Height {
		t.Errorf("call A mutated caller's Width/Height: was %dx%d, now %dx%d",
			preCall.Width, preCall.Height, sharedSpec.Width, sharedSpec.Height)
	}

	iconsB, _, err := generateDiagramCellInserts(cellB, ctxB)
	if err != nil {
		t.Fatalf("cellB render: %v", err)
	}
	if len(iconsB) != 1 || len(iconsB[0].SVGData) == 0 {
		t.Fatalf("cellB: expected one non-empty SVG, got %d icons", len(iconsB))
	}

	// Caller's spec still byte-identical after second call.
	if sharedSpec.Style != preStyle {
		t.Errorf("call B mutated caller's Style: was %v, now %v", preStyle, sharedSpec.Style)
	}
	if sharedSpec.Width != preCall.Width || sharedSpec.Height != preCall.Height {
		t.Errorf("call B mutated caller's Width/Height: was %dx%d, now %dx%d",
			preCall.Width, preCall.Height, sharedSpec.Width, sharedSpec.Height)
	}

	// The two SVGs must reflect the two cells' independent aspects/palettes.
	// Different cell bounds produce different viewBox dimensions; without the
	// clone fix, the second call would inherit dimensions from the first.
	svgA := string(iconsA[0].SVGData)
	svgB := string(iconsB[0].SVGData)
	if svgA == svgB {
		t.Error("expected independent SVGs for cells with distinct bounds and palettes, got identical output")
	}
	vbRe := regexp.MustCompile(`viewBox="0 0 ([0-9.]+) ([0-9.]+)"`)
	mA := vbRe.FindStringSubmatch(svgA)
	mB := vbRe.FindStringSubmatch(svgB)
	if mA == nil || mB == nil {
		t.Fatalf("missing viewBox in SVG output (A=%v B=%v)", mA != nil, mB != nil)
	}
	wA, _ := strconv.ParseFloat(mA[1], 64)
	hA, _ := strconv.ParseFloat(mA[2], 64)
	wB, _ := strconv.ParseFloat(mB[1], 64)
	hB, _ := strconv.ParseFloat(mB[2], 64)
	aspectA := wA / hA
	aspectB := wB / hB
	wantA := 5_000_000.0 / 3_000_000.0
	wantB := 2_000_000.0 / 4_000_000.0
	if diff := aspectA - wantA; diff > 0.05 || diff < -0.05 {
		t.Errorf("cellA SVG aspect %.3f != expected %.3f", aspectA, wantA)
	}
	if diff := aspectB - wantB; diff > 0.05 || diff < -0.05 {
		t.Errorf("cellB SVG aspect %.3f != expected %.3f", aspectB, wantB)
	}
}
