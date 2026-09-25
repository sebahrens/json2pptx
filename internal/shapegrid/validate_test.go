package shapegrid

import (
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestValidate_NilGrid(t *testing.T) {
	if err := Validate(nil); err != nil {
		t.Errorf("expected nil error for nil grid, got %v", err)
	}
}

func TestValidate_EmptyColumns(t *testing.T) {
	grid := &Grid{
		Rows: []Row{{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for empty columns")
	}
	if !strings.Contains(err.Error(), "empty columns") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ImageGeometry(t *testing.T) {
	for _, tc := range []struct {
		geometry string
		ok       bool
	}{{"", true}, {"rect", true}, {"ellipse", true}, {"star5", false}} {
		grid := &Grid{
			Columns: []float64{100},
			Rows:    []Row{{Cells: []Cell{{Image: &ImageSpec{Path: "a.png", Geometry: tc.geometry}}}}},
		}
		err := Validate(grid)
		if tc.ok && err != nil {
			t.Errorf("geometry %q: unexpected error %v", tc.geometry, err)
		}
		if !tc.ok && (err == nil || !strings.Contains(err.Error(), "image geometry")) {
			t.Errorf("geometry %q: want an image geometry error, got %v", tc.geometry, err)
		}
	}
}

func TestValidate_ColSpanExceedsGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{ColSpan: 3, Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for col_span exceeding grid")
	}
	if !strings.Contains(err.Error(), "col_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RowSpanExceedsGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for row_span exceeding grid")
	}
	if !strings.Contains(err.Error(), "row_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ShapeAndTableBothSet(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape:     &ShapeSpec{Geometry: "rect"},
					TableSpec: &types.TableSpec{Headers: []string{"A"}},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when both shape and table are set")
	}
	if !strings.Contains(err.Error(), "both shape and table") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_TableCellOnly(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{{
			Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{TableSpec: &types.TableSpec{Headers: []string{"A"}}},
			},
		}},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for valid grid with table cell, got %v", err)
	}
}

func TestValidate_ValidGrid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for valid grid, got %v", err)
	}
}

func TestValidate_NoRows(t *testing.T) {
	grid := &Grid{
		Columns: []float64{50, 50},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for no rows")
	}
	if !strings.Contains(err.Error(), "no rows") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_CellOverlapFromRowSpan(t *testing.T) {
	// Row 0: cell at col 0 with col_span=2, row_span=2 occupies (0,0),(0,1),(1,0),(1,1)
	// Row 1: cell at col 0 starts, but skip logic moves to col 2 (out of range)
	//        so the second row's cell can't fit, producing a col_span exceeds error.
	// This validates that row_span reservations are detected as conflicts.
	grid := &Grid{
		Columns: []float64{50, 50},
		Rows: []Row{
			{Cells: []Cell{
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}}, // pushed past occupied col 0
				{Shape: &ShapeSpec{Geometry: "rect"}}, // exceeds grid width
			}},
		},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for cell overflow from row_span")
	}
	if !strings.Contains(err.Error(), "col_span") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_EmptyGrid(t *testing.T) {
	grid := &Grid{}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for empty grid")
	}
}

func TestValidate_ValidLargeGrid(t *testing.T) {
	cols := make([]float64, 10)
	for i := range cols {
		cols[i] = 10
	}
	rows := make([]Row, 10)
	for r := range rows {
		cells := make([]Cell, 10)
		for c := range cells {
			cells[c] = Cell{Shape: &ShapeSpec{Geometry: "rect"}}
		}
		rows[r] = Row{Cells: cells}
	}
	grid := &Grid{Columns: cols, Rows: rows}
	if err := Validate(grid); err != nil {
		t.Errorf("expected valid large grid, got %v", err)
	}
}

func TestValidate_CombinedSpansValid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{25, 25, 25, 25},
		Rows: []Row{
			{Cells: []Cell{
				{ColSpan: 2, RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				// cols 0-1 occupied
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
		},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected valid grid with combined spans, got %v", err)
	}
}

func TestValidate_ShapeAndDiagramBothSet(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape:       &ShapeSpec{Geometry: "rect"},
					DiagramSpec: &types.DiagramSpec{Type: "bar_chart"},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when both shape and diagram are set")
	}
	msg := err.Error()
	if !strings.Contains(msg, "shape") || !strings.Contains(msg, "diagram") {
		t.Errorf("expected error to name both conflicting keys, got: %v", err)
	}
}

func TestValidate_DiagramAndImageBothSet(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					DiagramSpec: &types.DiagramSpec{Type: "bar_chart"},
					Image:       &ImageSpec{Path: "/tmp/x.png"},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when both diagram and image are set")
	}
	msg := err.Error()
	if !strings.Contains(msg, "diagram") || !strings.Contains(msg, "image") {
		t.Errorf("expected error to name both conflicting keys, got: %v", err)
	}
}

func TestValidate_IconAndImageBothSet(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Icon:  &IconSpec{Name: "chart-pie"},
					Image: &ImageSpec{Path: "/tmp/x.png"},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when both icon and image are set")
	}
	msg := err.Error()
	if !strings.Contains(msg, "icon") || !strings.Contains(msg, "image") {
		t.Errorf("expected error to name both conflicting keys, got: %v", err)
	}
}

func TestValidate_ShapeIconOverlayPermitted(t *testing.T) {
	// Legacy carve-out: shape+icon overlay must remain valid.
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape: &ShapeSpec{Geometry: "rect"},
					Icon:  &IconSpec{Name: "chart-pie"},
				},
			},
		}},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for legacy shape+icon overlay, got %v", err)
	}
}

func TestValidate_ShapeIconImageTrio(t *testing.T) {
	// Even though shape+icon is permitted, adding a third payload key must
	// still raise an error naming all three.
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Shape: &ShapeSpec{Geometry: "rect"},
					Icon:  &IconSpec{Name: "chart-pie"},
					Image: &ImageSpec{Path: "/tmp/x.png"},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when shape+icon+image are all set")
	}
	msg := err.Error()
	if !strings.Contains(msg, "shape") || !strings.Contains(msg, "icon") || !strings.Contains(msg, "image") {
		t.Errorf("expected error to name all three conflicting keys, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	grid := &Grid{} // no columns AND no rows
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "columns") || !strings.Contains(errStr, "rows") {
		t.Errorf("expected errors about both columns and rows, got: %v", err)
	}
}

func TestValidate_CompositeValid(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
					},
				},
			},
		}},
	}
	if err := Validate(grid); err != nil {
		t.Errorf("expected no error for valid composite cell, got %v", err)
	}
}

func TestValidate_CompositeMissingText(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
					},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when composite text is missing")
	}
	if !strings.Contains(err.Error(), "\"text\"") {
		t.Errorf("expected error to mention missing text, got: %v", err)
	}
}

func TestValidate_CompositeMissingSubDiagram(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text: &ShapeSpec{Geometry: "rect"},
					},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error when composite sub_diagram is missing")
	}
	if !strings.Contains(err.Error(), "\"sub_diagram\"") {
		t.Errorf("expected error to mention missing sub_diagram, got: %v", err)
	}
}

func TestValidate_CompositeWithLegacyKeys(t *testing.T) {
	cases := []struct {
		name   string
		cell   Cell
		needle string
	}{
		{
			name: "composite+shape",
			cell: Cell{
				Composite: &CompositeSpec{
					Text:       &ShapeSpec{Geometry: "rect"},
					SubDiagram: &types.DiagramSpec{Type: "line_chart"},
				},
				Shape: &ShapeSpec{Geometry: "rect"},
			},
			needle: "shape",
		},
		{
			name: "composite+diagram",
			cell: Cell{
				Composite: &CompositeSpec{
					Text:       &ShapeSpec{Geometry: "rect"},
					SubDiagram: &types.DiagramSpec{Type: "line_chart"},
				},
				DiagramSpec: &types.DiagramSpec{Type: "bar_chart"},
			},
			needle: "diagram",
		},
		{
			name: "composite+image",
			cell: Cell{
				Composite: &CompositeSpec{
					Text:       &ShapeSpec{Geometry: "rect"},
					SubDiagram: &types.DiagramSpec{Type: "line_chart"},
				},
				Image: &ImageSpec{Path: "/tmp/x.png"},
			},
			needle: "image",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grid := &Grid{
				Columns: []float64{100},
				Rows:    []Row{{Cells: []Cell{tc.cell}}},
			}
			err := Validate(grid)
			if err == nil {
				t.Fatalf("expected error when composite is combined with %s", tc.needle)
			}
			msg := err.Error()
			if !strings.Contains(msg, "composite") || !strings.Contains(msg, tc.needle) {
				t.Errorf("expected error to name composite and %s, got: %v", tc.needle, err)
			}
		})
	}
}

func TestValidate_CompositeInvalidSplit(t *testing.T) {
	grid := &Grid{
		Columns: []float64{100},
		Rows: []Row{{
			Cells: []Cell{
				{
					Composite: &CompositeSpec{
						Text:       &ShapeSpec{Geometry: "rect"},
						SubDiagram: &types.DiagramSpec{Type: "line_chart"},
						Split:      "middle",
					},
				},
			},
		}},
	}
	err := Validate(grid)
	if err == nil {
		t.Fatal("expected error for invalid composite split value")
	}
	if !strings.Contains(err.Error(), "split") {
		t.Errorf("expected error to mention split, got: %v", err)
	}
}

func TestValidate_CompositeRatioOutOfRange(t *testing.T) {
	// 0 is treated as "unset" and defaults to 0.5, so we don't include 0 here.
	cases := []float64{1.0, 1.5, -0.2}
	for _, ratio := range cases {
		grid := &Grid{
			Columns: []float64{100},
			Rows: []Row{{
				Cells: []Cell{
					{
						Composite: &CompositeSpec{
							Text:       &ShapeSpec{Geometry: "rect"},
							SubDiagram: &types.DiagramSpec{Type: "line_chart"},
							Ratio:      ratio,
						},
					},
				},
			}},
		}
		err := Validate(grid)
		if err == nil {
			t.Errorf("expected error for ratio %g", ratio)
			continue
		}
		if !strings.Contains(err.Error(), "ratio") {
			t.Errorf("expected error to mention ratio (got %g): %v", ratio, err)
		}
	}
}

// go-slide-creator-s1uvj.38: Validate skipped empty spacer cells by one
// column regardless of col_span, so it checked a different layout than the
// one Resolve renders.
func TestValidate_EmptySpacerHonoursColSpan(t *testing.T) {
	ok := &Grid{
		Columns: []float64{100.0 / 3, 100.0 / 3, 100.0 / 3},
		Rows: []Row{{Cells: []Cell{
			{ColSpan: 2},
			{Shape: &ShapeSpec{Geometry: "rect"}},
		}}},
	}
	if err := Validate(ok); err != nil {
		t.Fatalf("spacer col_span 2 + one shape in 3 columns should be valid: %v", err)
	}
	overflow := &Grid{
		Columns: []float64{100.0 / 3, 100.0 / 3, 100.0 / 3},
		Rows: []Row{{Cells: []Cell{
			{ColSpan: 2},
			{ColSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
		}}},
	}
	err := Validate(overflow)
	if err == nil || !strings.Contains(err.Error(), "col_span 2 exceeds grid width") {
		t.Fatalf("expected col_span overflow error after a spanning spacer, got %v", err)
	}
}

// Trailing empty cells standing in for positions covered by an earlier
// row_span (arch-stack side rails) must stay valid.
func TestValidate_TrailingCoveredEmptyCellsAllowed(t *testing.T) {
	grid := &Grid{
		Columns: []float64{60, 20, 20},
		Rows: []Row{
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
				{RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect"}},
				{},
				{},
			}},
		},
	}
	if err := Validate(grid); err != nil {
		t.Fatalf("covered trailing empty cells should be valid: %v", err)
	}
}

// go-slide-creator-s1uvj.39: negative or non-finite track weights produced
// negative extents that passed validation.
func TestValidate_RejectsBadTrackWeights(t *testing.T) {
	shape := []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}
	tests := []struct {
		name string
		grid *Grid
		want string
	}{
		{"negative column", &Grid{Columns: []float64{-20, 60, 60}, Rows: []Row{{Cells: shape}}}, "columns[0] is -20"},
		{"NaN column", &Grid{Columns: []float64{50, math.NaN()}, Rows: []Row{{Cells: shape}}}, "columns[1] is NaN"},
		{"Inf column", &Grid{Columns: []float64{math.Inf(1), 50}, Rows: []Row{{Cells: shape}}}, "columns[0] is +Inf"},
		{"all-zero columns", &Grid{Columns: []float64{0, 0, 0}, Rows: []Row{{Cells: shape}}}, "columns widths sum to 0"},
		{"negative row height", &Grid{Columns: []float64{100}, Rows: []Row{{Height: -10, Cells: shape}}}, "rows[0].height is -10"},
		{"negative flex", &Grid{Columns: []float64{100}, Rows: []Row{{Flex: -1, Cells: shape}}}, "rows[0].flex is -1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.grid)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.want)
			}
		})
	}
	ok := &Grid{Columns: []float64{0, 60, 40}, Rows: []Row{{Cells: []Cell{{}, {Shape: &ShapeSpec{Geometry: "rect"}}, {}}}}}
	if err := Validate(ok); err != nil {
		t.Fatalf("a zero-width column among positive ones is valid: %v", err)
	}
}
