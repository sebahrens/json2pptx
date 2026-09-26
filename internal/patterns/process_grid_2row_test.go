package patterns

import (
	"strings"
	"testing"
)

func TestProcessGrid2Row_Registration(t *testing.T) {
	p, ok := Default().Get("process-grid-2row")
	if !ok {
		t.Fatal("expected process-grid-2row to be registered in default registry")
	}
	if p.Name() != "process-grid-2row" {
		t.Errorf("Name() = %q, want %q", p.Name(), "process-grid-2row")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func validProcessGrid2RowValues(n int) *ProcessGrid2RowValues {
	row1 := []string{"DESIGN", "EDIT", "ASSETS", "UX / UI", "QA", "RELEASE"}
	row2 := []string{"PROTOTYPE", "DEVELOP", "USER TESTING", "RELEASE", "MONITOR", "ITERATE"}
	return &ProcessGrid2RowValues{
		Row1Label:  "DESIGN PROCESS",
		Row1Phases: row1[:n],
		Row2Label:  "PRODUCTION",
		Row2Phases: row2[:n],
	}
}

func TestProcessGrid2Row_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(t.Name(), func(t *testing.T) {
			if err := p.Validate(validProcessGrid2RowValues(n), nil, nil); err != nil {
				t.Errorf("n=%d: unexpected validation error: %v", n, err)
			}
		})
	}
}

func TestProcessGrid2Row_Validate_TooFewPhases(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row1Phases = v.Row1Phases[:2]
	v.Row2Phases = v.Row2Phases[:2]
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for fewer than 3 phases")
	}
}

func TestProcessGrid2Row_Validate_TooManyPhases(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(6)
	v.Row1Phases = append(v.Row1Phases, "EXTRA")
	v.Row2Phases = append(v.Row2Phases, "EXTRA")
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 6 phases")
	}
}

func TestProcessGrid2Row_Validate_UnequalPhaseCounts(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row2Phases = v.Row2Phases[:3]
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for unequal phase counts")
	}
	if !strings.Contains(err.Error(), "same length") {
		t.Errorf("expected error to mention 'same length', got: %v", err)
	}
}

func TestProcessGrid2Row_Validate_MissingRowLabel(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row1Label = "   "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank row1_label")
	}
}

func TestProcessGrid2Row_Validate_RowLabelTooLong(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row2Label = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for row2_label > 40 chars")
	}
}

func TestProcessGrid2Row_Validate_PhaseLabelTooLong(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row1Phases[2] = strings.Repeat("X", 41)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for phase label > 40 chars")
	}
}

func TestProcessGrid2Row_Validate_PhaseLabelBlank(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row2Phases[1] = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank phase label")
	}
}

func TestProcessGrid2Row_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	// Total cells: 2 labels + 4 + 4 = 10. Index 42 is out of range.
	overrides := map[int]any{42: &ProcessGrid2RowCellOverride{AccentBar: true}}
	if err := p.Validate(v, nil, overrides); err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
}

func TestProcessGrid2Row_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
	for i, row := range grid.Rows {
		if got := len(row.Cells); got != 5 {
			t.Errorf("row %d: expected 5 cells (1 label + 4 phases), got %d", i, got)
		}
	}
}

func TestProcessGrid2Row_Expand_DefaultColorsAreAccent1Accent3(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row label cells (col 0 of each row) should be dk2 (never dk1 black).
	for i, row := range grid.Rows {
		labelCell := row.Cells[0]
		if labelCell.Shape == nil || !strings.Contains(string(labelCell.Shape.Fill), "dk2") {
			t.Errorf("row %d label cell: expected dk2 fill, got %q", i, string(labelCell.Shape.Fill))
		}
	}
	// Row 1 phase cells default to accent1.
	for i := 1; i < len(grid.Rows[0].Cells); i++ {
		cell := grid.Rows[0].Cells[i]
		if !strings.Contains(string(cell.Shape.Fill), "accent1") {
			t.Errorf("row 1 phase %d: expected accent1 fill, got %q", i, string(cell.Shape.Fill))
		}
	}
	// go-slide-creator-at7ij: row 2 defaults to the SAME accent, darker. The two
	// rows are parallel tracks of one process; accent3 is a second brand hue
	// (green beside bright blue on forest-green).
	for i := 1; i < len(grid.Rows[1].Cells); i++ {
		got := string(grid.Rows[1].Cells[i].Shape.Fill)
		if !strings.Contains(got, "accent1") {
			t.Errorf("row 2 phase %d: expected a tone of accent1, got %q", i, got)
		}
		if strings.Contains(got, "accent3") {
			t.Errorf("row 2 phase %d still reaches for accent3: %q", i, got)
		}
		if !strings.Contains(got, "lumMod") {
			t.Errorf("row 2 phase %d should be tinted so the rows are distinguishable, got %q", i, got)
		}
		if row1 := string(grid.Rows[0].Cells[i].Shape.Fill); row1 == got {
			t.Errorf("row 2 phase %d renders the same fill as row 1 (%q)", i, row1)
		}
	}
}

func TestProcessGrid2Row_Expand_CustomColors(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.Row1Color = "accent2"
	v.Row2Color = "accent5"
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i := 1; i < len(grid.Rows[0].Cells); i++ {
		if !strings.Contains(string(grid.Rows[0].Cells[i].Shape.Fill), "accent2") {
			t.Errorf("row 1 phase %d: expected accent2 fill, got %q", i, string(grid.Rows[0].Cells[i].Shape.Fill))
		}
	}
	for i := 1; i < len(grid.Rows[1].Cells); i++ {
		if !strings.Contains(string(grid.Rows[1].Cells[i].Shape.Fill), "accent5") {
			t.Errorf("row 2 phase %d: expected accent5 fill, got %q", i, string(grid.Rows[1].Cells[i].Shape.Fill))
		}
	}
}

func TestProcessGrid2Row_Expand_BoundaryPhaseCounts(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	for _, n := range []int{3, 6} {
		t.Run(t.Name(), func(t *testing.T) {
			v := validProcessGrid2RowValues(n)
			grid, err := p.Expand(ExpandContext{}, v, nil, nil)
			if err != nil {
				t.Fatalf("n=%d: Expand failed: %v", n, err)
			}
			for i, row := range grid.Rows {
				if got := len(row.Cells); got != n+1 {
					t.Errorf("n=%d: row %d: expected %d cells (1 label + %d phases), got %d", n, i, n+1, n, got)
				}
			}
		})
	}
}

func TestProcessGrid2Row_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(3)
	// Cell index 1 = first phase of row 1.
	cellOverrides := map[int]any{1: &ProcessGrid2RowCellOverride{AccentBar: true}}
	grid, err := p.Expand(ExpandContext{}, v, nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[1].AccentBar == nil {
		t.Error("expected accent bar on cell 1 (row 1 phase 0)")
	}
}

func TestProcessGrid2Row_Schema(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	schema := p.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestProcessGrid2Row_Taxonomy(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want %q", tax.Category, "structural")
	}
	if tax.DensityClass != "medium" {
		t.Errorf("DensityClass = %q, want %q", tax.DensityClass, "medium")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("expected NarrativeRole to be non-empty")
	}
}

func TestProcessGrid2Row_HeadersAndOutcomes_Validate(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.ColumnHeaders = []string{"Segment", "Prospect", "Meet", "Retain"}
	v.Outcomes = []string{"+5% wallet", "+5% win", "+15% coverage", "-3% attrition"}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("matching headers/outcomes: %v", err)
	}
	// Addressable cells extend by N headers + N outcomes.
	last := 2 + 4 + 4 + 4 + 4 - 1
	if err := p.Validate(v, nil, map[int]any{last: &ProcessGrid2RowCellOverride{AccentBar: true}}); err != nil {
		t.Errorf("cell override on the last outcome: %v", err)
	}

	short := validProcessGrid2RowValues(4)
	short.ColumnHeaders = []string{"A", "B", "C"}
	if err := p.Validate(short, nil, nil); err == nil || !strings.Contains(err.Error(), "column_headers needs one entry per phase") {
		t.Errorf("3 headers for 4 phases: want count mismatch, got %v", err)
	}
	uneven := validProcessGrid2RowValues(4)
	uneven.Row2Phases = uneven.Row2Phases[:3]
	uneven.Outcomes = []string{"a", "b", "c", "d"}
	if err := p.Validate(uneven, nil, nil); err == nil || !strings.Contains(err.Error(), "outcomes needs one entry per phase") {
		t.Errorf("outcomes with unequal rows: want count mismatch, got %v", err)
	}
	long := validProcessGrid2RowValues(3)
	long.Outcomes = []string{strings.Repeat("x", 61), "b", ""}
	err := p.Validate(long, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "outcomes[0]") || !strings.Contains(err.Error(), "outcomes[2]") {
		t.Errorf("want max-length on outcomes[0] and required on outcomes[2], got %v", err)
	}
}

func TestProcessGrid2Row_HeadersAndOutcomes_Expand(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	plain, err := p.Expand(ExpandContext{}, validProcessGrid2RowValues(4), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Rows) != 2 {
		t.Fatalf("without headers/outcomes want 2 rows, got %d", len(plain.Rows))
	}

	v := validProcessGrid2RowValues(4)
	v.ColumnHeaders = []string{"Segment", "Prospect", "Meet", "Retain"}
	v.Outcomes = []string{"+5% wallet", "+5% win", "+15% coverage", "-3% attrition"}
	cellOverrides := map[int]any{0: &ProcessGrid2RowCellOverride{AccentBar: true}}
	grid, err := p.Expand(fullThemeCtx(), v, nil, cellOverrides)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("want header + 2 tracks + outcomes, got %d rows", len(grid.Rows))
	}
	header, outcomes := grid.Rows[0], grid.Rows[3]
	for _, r := range []struct {
		name string
		row  int
	}{{"header", 0}, {"outcomes", 3}} {
		row := grid.Rows[r.row]
		if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("%s row must be content-sized, got min=%v max=%v", r.name, row.MinHeight, row.MaxHeight)
		}
		if len(row.Cells) != 5 || row.Cells[0].Shape != nil {
			t.Errorf("%s row: want an empty label-column cell + 4 cells, got %d", r.name, len(row.Cells))
		}
	}
	for i, c := range header.Cells[1:] {
		if c.AccentBar == nil || c.AccentBar.Position != "bottom" {
			t.Errorf("header %d lacks the accent underline", i)
		}
		if !strings.Contains(string(c.Shape.Text), v.ColumnHeaders[i]) {
			t.Errorf("header %d text = %s", i, c.Shape.Text)
		}
	}
	for i, c := range outcomes.Cells[1:] {
		if c.Shape.Geometry != "roundRect" || !strings.Contains(string(c.Shape.Fill), "lumMod") {
			t.Errorf("outcome %d is not a tinted pill: %s %s", i, c.Shape.Geometry, c.Shape.Fill)
		}
	}
	// Track indices are unchanged: override 0 still targets the row-1 label.
	if grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("cell_overrides[0] must still land on row1_label")
	}
	// Tracks keep flexing over the remaining height.
	if grid.Rows[1].MaxHeight != 0 || grid.Rows[2].MaxHeight != 0 {
		t.Error("track rows must stay flex rows")
	}
	assertPatternGolden(t, grid, "testdata/process-grid-2row/headers_outcomes.golden.json")
}
