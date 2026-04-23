package pipeline

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func makeGrid(rows ...jsonschema.GridRowInput) *jsonschema.ShapeGridInput {
	return &jsonschema.ShapeGridInput{Rows: rows}
}

func tableRow(cells ...*jsonschema.GridCellInput) jsonschema.GridRowInput {
	return jsonschema.GridRowInput{Cells: cells}
}

func tableCell() *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Table: &jsonschema.TableInput{
			Headers: []string{"A"},
			Rows:    [][]jsonschema.TableCellInput{{{Content: "x"}}},
		},
	}
}

func shapeCell(fill string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"` + fill + `"`),
		},
	}
}

func shapeCellObjFill(color string) *jsonschema.GridCellInput {
	raw, _ := json.Marshal(map[string]string{"color": color})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(raw),
		},
	}
}

func emptyShapeCell() *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
		},
	}
}

// --- Stacked Tables ---

func TestDetectStackedTables_Positive(t *testing.T) {
	grid := makeGrid(
		tableRow(tableCell()),
		tableRow(tableCell()),
	)
	grid.RowGap = 2.0 // Below 4pt threshold

	warnings := DetectStructuralSmells(grid, 0)
	found := false
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrStackedTables) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stacked_tables warning, got %v", warnings)
	}
}

func TestDetectStackedTables_NegativeGapOK(t *testing.T) {
	grid := makeGrid(
		tableRow(tableCell()),
		tableRow(tableCell()),
	)
	grid.RowGap = 8.0 // Above 4pt threshold

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrStackedTables) {
			t.Errorf("unexpected stacked_tables warning: %v", w)
		}
	}
}

func TestDetectStackedTables_NegativeNoAdjacentTables(t *testing.T) {
	grid := makeGrid(
		tableRow(tableCell()),
		tableRow(emptyShapeCell()),
		tableRow(tableCell()),
	)
	grid.RowGap = 2.0

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrStackedTables) {
			t.Errorf("unexpected stacked_tables warning when tables not adjacent: %v", w)
		}
	}
}

func TestDetectStackedTables_DefaultGapOK(t *testing.T) {
	// Default gap is 8pt which is above threshold.
	grid := makeGrid(
		tableRow(tableCell()),
		tableRow(tableCell()),
	)

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrStackedTables) {
			t.Errorf("unexpected stacked_tables warning with default gap: %v", w)
		}
	}
}

// --- Divider Too Thin ---

func TestDetectDividerTooThin_SmallGap(t *testing.T) {
	grid := makeGrid(
		tableRow(emptyShapeCell()),
		tableRow(emptyShapeCell()),
	)
	grid.RowGap = 1.0 // Below 3pt threshold

	warnings := DetectStructuralSmells(grid, 0)
	found := false
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrDividerTooThin) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected divider_too_thin warning, got %v", warnings)
	}
}

func TestDetectDividerTooThin_SmallRowHeight(t *testing.T) {
	grid := makeGrid(
		jsonschema.GridRowInput{Height: 2.0, Cells: []*jsonschema.GridCellInput{emptyShapeCell()}},
		jsonschema.GridRowInput{Height: 50.0, Cells: []*jsonschema.GridCellInput{emptyShapeCell()}},
	)
	grid.RowGap = 8.0 // Gap is fine

	warnings := DetectStructuralSmells(grid, 0)
	found := false
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrDividerTooThin) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected divider_too_thin warning for tiny row height, got %v", warnings)
	}
}

func TestDetectDividerTooThin_NegativeAllOK(t *testing.T) {
	grid := makeGrid(
		jsonschema.GridRowInput{Height: 40.0, Cells: []*jsonschema.GridCellInput{emptyShapeCell()}},
		jsonschema.GridRowInput{Height: 60.0, Cells: []*jsonschema.GridCellInput{emptyShapeCell()}},
	)
	grid.RowGap = 8.0

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrDividerTooThin) {
			t.Errorf("unexpected divider_too_thin warning: %v", w)
		}
	}
}

// --- Mixed Fill Scheme ---

func TestDetectMixedFillScheme_Positive(t *testing.T) {
	grid := makeGrid(
		tableRow(shapeCell("#FF0000"), shapeCell("accent1")),
	)

	warnings := DetectStructuralSmells(grid, 0)
	found := false
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mixed_fill_scheme warning, got %v", warnings)
	}
}

func TestDetectMixedFillScheme_PositiveObjFill(t *testing.T) {
	grid := makeGrid(
		tableRow(shapeCellObjFill("#00FF00"), shapeCell("dk1")),
	)

	warnings := DetectStructuralSmells(grid, 0)
	found := false
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mixed_fill_scheme warning with object fill, got %v", warnings)
	}
}

func TestDetectMixedFillScheme_NegativeAllSemantic(t *testing.T) {
	grid := makeGrid(
		tableRow(shapeCell("accent1"), shapeCell("accent2")),
	)

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			t.Errorf("unexpected mixed_fill_scheme warning with all semantic fills: %v", w)
		}
	}
}

func TestDetectMixedFillScheme_NegativeAllHex(t *testing.T) {
	grid := makeGrid(
		tableRow(shapeCell("#FF0000"), shapeCell("#00FF00")),
	)

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			t.Errorf("unexpected mixed_fill_scheme warning with all hex fills: %v", w)
		}
	}
}

func TestDetectMixedFillScheme_AllowlistIgnored(t *testing.T) {
	// Black/white hex are allowlisted and should not count as "hex fill".
	grid := makeGrid(
		tableRow(shapeCell("#000000"), shapeCell("accent1")),
	)

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			t.Errorf("unexpected mixed_fill_scheme warning with allowlisted hex: %v", w)
		}
	}
}

func TestDetectMixedFillScheme_NoFills(t *testing.T) {
	grid := makeGrid(
		tableRow(emptyShapeCell(), emptyShapeCell()),
	)

	warnings := DetectStructuralSmells(grid, 0)
	for _, w := range warnings {
		if errors.Is(w, patterns.ErrMixedFillScheme) {
			t.Errorf("unexpected mixed_fill_scheme warning with no fills: %v", w)
		}
	}
}

// --- Nil / Empty ---

func TestDetectStructuralSmells_NilGrid(t *testing.T) {
	warnings := DetectStructuralSmells(nil, 0)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil grid, got %d", len(warnings))
	}
}

func TestDetectStructuralSmells_EmptyGrid(t *testing.T) {
	grid := &jsonschema.ShapeGridInput{}
	warnings := DetectStructuralSmells(grid, 0)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty grid, got %d", len(warnings))
	}
}

// --- Error code verification ---

func TestStructuralSmellErrorCodes(t *testing.T) {
	// Stacked tables
	grid := makeGrid(tableRow(tableCell()), tableRow(tableCell()))
	grid.RowGap = 1.0
	warnings := detectStackedTables(grid, 0)
	if len(warnings) == 0 {
		t.Fatal("expected stacked_tables warning")
	}
	if warnings[0].Code != patterns.ErrCodeStackedTables {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeStackedTables, warnings[0].Code)
	}

	// Divider too thin (gap)
	grid2 := makeGrid(tableRow(emptyShapeCell()), tableRow(emptyShapeCell()))
	grid2.RowGap = 1.0
	warnings2 := detectDividerTooThin(grid2, 0)
	if len(warnings2) == 0 {
		t.Fatal("expected divider_too_thin warning")
	}
	if warnings2[0].Code != patterns.ErrCodeDividerTooThin {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeDividerTooThin, warnings2[0].Code)
	}

	// Mixed fill scheme
	grid3 := makeGrid(tableRow(shapeCell("#FF0000"), shapeCell("accent1")))
	warnings3 := detectMixedFillScheme(grid3, 0)
	if len(warnings3) == 0 {
		t.Fatal("expected mixed_fill_scheme warning")
	}
	if warnings3[0].Code != patterns.ErrCodeMixedFillScheme {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeMixedFillScheme, warnings3[0].Code)
	}
}

// --- effectiveRowGap ---

func TestEffectiveRowGap(t *testing.T) {
	tests := []struct {
		name string
		grid *jsonschema.ShapeGridInput
		want float64
	}{
		{"default", &jsonschema.ShapeGridInput{}, defaultGridGapPt},
		{"gap override", &jsonschema.ShapeGridInput{Gap: 5.0}, 5.0},
		{"row_gap override", &jsonschema.ShapeGridInput{Gap: 5.0, RowGap: 2.0}, 2.0},
		{"row_gap only", &jsonschema.ShapeGridInput{RowGap: 10.0}, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveRowGap(tt.grid)
			if got != tt.want {
				t.Errorf("effectiveRowGap() = %v, want %v", got, tt.want)
			}
		})
	}
}
