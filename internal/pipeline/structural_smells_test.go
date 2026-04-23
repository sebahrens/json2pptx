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

// --- Table density ---

func makeDensityTable(dataRows, cols int) *jsonschema.TableInput {
	headers := make([]string, cols)
	for i := range headers {
		headers[i] = "H"
	}
	rows := make([][]jsonschema.TableCellInput, dataRows)
	for i := range rows {
		row := make([]jsonschema.TableCellInput, cols)
		for j := range row {
			row[j] = jsonschema.TableCellInput{Content: "x", ColSpan: 1, RowSpan: 1}
		}
		rows[i] = row
	}
	return &jsonschema.TableInput{Headers: headers, Rows: rows}
}

func TestDetectTableDensity_NoWarning(t *testing.T) {
	// 6 data rows + 1 header = 7 logical rows, 6 cols → no warning
	table := makeDensityTable(6, 6)
	warnings := DetectTableDensity(table, "slides[0].content[0]")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for 7×6 table, got %d: %v", len(warnings), warnings)
	}
}

func TestDetectTableDensity_TooManyRowsAndCols(t *testing.T) {
	// 7 data rows + 1 header = 8 logical rows > 7, 7 cols > 6
	table := makeDensityTable(7, 7)
	warnings := DetectTableDensity(table, "slides[0].content[0]")
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings for 8×7 table, got %d: %v", len(warnings), warnings)
	}
	for _, w := range warnings {
		if w.Code != patterns.ErrCodeDensityExceeded {
			t.Errorf("expected code %q, got %q", patterns.ErrCodeDensityExceeded, w.Code)
		}
	}
}

func TestDetectTableDensity_MultilineCells(t *testing.T) {
	// 6 data rows + 1 header = 7, but 3 multiline cells add 3 → 10 > 7
	table := makeDensityTable(6, 6)
	table.Rows[0][0] = jsonschema.TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}
	table.Rows[1][0] = jsonschema.TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}
	table.Rows[2][0] = jsonschema.TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}

	warnings := DetectTableDensity(table, "slides[0].content[0]")
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Code != patterns.ErrCodeDensityExceeded {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeDensityExceeded, warnings[0].Code)
	}
	if warnings[0].Fix == nil || warnings[0].Fix.Kind != "split_at_row" {
		t.Errorf("expected fix kind 'split_at_row', got %v", warnings[0].Fix)
	}
}

func TestDetectTableDensity_NilTable(t *testing.T) {
	warnings := DetectTableDensity(nil, "slides[0]")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil table, got %d", len(warnings))
	}
}

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
