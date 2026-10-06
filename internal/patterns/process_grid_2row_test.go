package patterns

import (
	"encoding/json"
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

// TestProcessGrid2Row_Expand_LanesDefault: each track is a lane, a pentagon
// label pointing into interlocking chevrons, in four depths of one accent
// with at most one solid block; lanes are content-sized (go-slide-creator-06bnr).
func TestProcessGrid2Row_Expand_LanesDefault(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	for _, n := range []int{3, 4, 5, 6} {
		v := validProcessGrid2RowValues(n)
		for _, ovr := range []any{nil, &ProcessGrid2RowOverrides{Style: "lanes"}} {
			grid, err := p.Expand(fullThemeCtx(), v, ovr, nil)
			if err != nil {
				t.Fatalf("n=%d: Expand failed: %v", n, err)
			}
			if len(grid.Rows) != 2 {
				t.Fatalf("n=%d: want 2 lanes, got %d rows", n, len(grid.Rows))
			}
			fills := map[string]bool{}
			solid := 0
			for r, row := range grid.Rows {
				if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
					t.Errorf("n=%d lane %d must be content-sized, got min=%v max=%v", n, r+1, row.MinHeight, row.MaxHeight)
				}
				if row.MaxHeight > processGrid2RowLaneMaxHPt {
					t.Errorf("n=%d lane %d is %vpt tall, cap %v", n, r+1, row.MaxHeight, processGrid2RowLaneMaxHPt)
				}
				for i, c := range row.Cells {
					want := "chevron"
					if i == 0 {
						want = "homePlate"
					}
					if c.Shape.Geometry != want {
						t.Errorf("n=%d lane %d cell %d: geometry %q, want %q", n, r+1, i, c.Shape.Geometry, want)
					}
					if (c.BleedLeft > 0) != (i > 0) {
						t.Errorf("n=%d lane %d cell %d: bleed_left %v (only chevrons tuck under the point before them)", n, r+1, i, c.BleedLeft)
					}
					if adj := c.Shape.Adjustments["adj"]; adj <= 0 || adj > 50000 {
						t.Errorf("n=%d lane %d cell %d: adj %d", n, r+1, i, adj)
					}
					fill := string(c.Shape.Fill)
					if !strings.Contains(fill, "accent1") {
						t.Errorf("n=%d lane %d cell %d: fill %s is not a depth of the accent", n, r+1, i, fill)
					}
					if !strings.Contains(fill, "lumMod") {
						solid++
					}
					if string(c.Shape.Line) != `"none"` || c.AccentBar != nil {
						t.Errorf("n=%d lane %d cell %d: lanes carry no outline or rule", n, r+1, i)
					}
					fills[fill] = true
					var text struct {
						Paragraphs []struct {
							Size float64 `json:"size"`
						} `json:"paragraphs"`
					}
					if err := json.Unmarshal(c.Shape.Text, &text); err != nil || len(text.Paragraphs) != 1 || text.Paragraphs[0].Size < 12 {
						t.Errorf("n=%d lane %d cell %d: text %s", n, r+1, i, c.Shape.Text)
					}
				}
			}
			if len(fills) != 4 {
				t.Errorf("n=%d: want four depths (two labels, two phase tints), got %d: %v", n, len(fills), fills)
			}
			if solid != 1 {
				t.Errorf("n=%d: want exactly one solid accent block (the first lane's label), got %d", n, solid)
			}
		}
	}
	// An authored row2_color is a second family at the first lane's depths.
	v := validProcessGrid2RowValues(4)
	v.Row2Color = "accent5"
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range grid.Rows[1].Cells {
		if !strings.Contains(string(c.Shape.Fill), "accent5") {
			t.Errorf("row2_color lane cell %d: fill %s", i, c.Shape.Fill)
		}
	}
	for _, style := range processGrid2RowStyles {
		if err := p.Validate(v, &ProcessGrid2RowOverrides{Style: style}, nil); err != nil {
			t.Errorf("style %q: %v", style, err)
		}
	}
}

// TestProcessGrid2Row_Lanes_HeadersAndOutcomes: headers and outcomes are
// plain bold text lines over and under the lanes: no underline, no pill.
func TestProcessGrid2Row_Lanes_HeadersAndOutcomes(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	v.ColumnHeaders = []string{"Segment", "Prospect", "Meet", "Retain"}
	v.Outcomes = []string{"+5% wallet", "+5% win", "+15% coverage", "-3% attrition"}
	cellOverrides := map[int]any{0: &ProcessGrid2RowCellOverride{AccentBar: true}}
	grid, err := p.Expand(fullThemeCtx(), v, nil, cellOverrides)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("want header + 2 lanes + outcomes, got %d rows", len(grid.Rows))
	}
	for _, r := range []int{0, 3} {
		row := grid.Rows[r]
		if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("row %d must be content-sized, got min=%v max=%v", r, row.MinHeight, row.MaxHeight)
		}
		if len(row.Cells) != 5 || row.Cells[0].Shape != nil {
			t.Fatalf("row %d: want an empty label-column cell + 4 cells, got %d", r, len(row.Cells))
		}
		for i, c := range row.Cells[1:] {
			if c.Shape.Geometry != "rect" || string(c.Shape.Fill) != `"none"` || c.AccentBar != nil {
				t.Errorf("row %d cell %d: want unfilled text, got %s %s bar=%v", r, i, c.Shape.Geometry, c.Shape.Fill, c.AccentBar)
			}
		}
	}
	if !strings.Contains(string(grid.Rows[0].Cells[1].Shape.Text), "Segment") || !strings.Contains(string(grid.Rows[3].Cells[4].Shape.Text), "-3% attrition") {
		t.Error("header / outcome text missing")
	}
	if grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("cell_overrides[0] must still land on row1_label")
	}
	assertPatternGolden(t, grid, "testdata/process-grid-2row/lanes.golden.json")
}

// TestProcessGrid2Row_Expand_TintedStyle: under style tinted the phase boxes
// are neutral tints under a thin rule in the track colour, the second track a
// lighter step (the default before go-slide-creator-06bnr; go-slide-creator-fl11f).
func TestProcessGrid2Row_Expand_TintedStyle(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	grid, err := p.Expand(ExpandContext{}, v, &ProcessGrid2RowOverrides{Style: "tinted"}, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for r := 0; r < 2; r++ {
		for i := 1; i < len(grid.Rows[r].Cells); i++ {
			cell := grid.Rows[r].Cells[i]
			if strings.Contains(string(cell.Shape.Fill), "accent") {
				t.Errorf("row %d phase %d: want a neutral tint, got %s", r+1, i, cell.Shape.Fill)
			}
			if cell.AccentBar == nil || cell.AccentBar.Color != "accent1" || cell.AccentBar.Position != "top" {
				t.Errorf("row %d phase %d: want a top accent1 rule, got %+v", r+1, i, cell.AccentBar)
			}
		}
	}
	if string(grid.Rows[0].Cells[1].Shape.Fill) == string(grid.Rows[1].Cells[1].Shape.Fill) {
		t.Errorf("the two tracks share one tint %s", grid.Rows[0].Cells[1].Shape.Fill)
	}
	if err := p.Validate(v, &ProcessGrid2RowOverrides{Style: "loud"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("expected overrides.style error, got %v", err)
	}
}

func TestProcessGrid2Row_Expand_DefaultColorsAreAccent1Accent3(t *testing.T) {
	p, _ := Default().Get("process-grid-2row")
	v := validProcessGrid2RowValues(4)
	grid, err := p.Expand(ExpandContext{}, v, &ProcessGrid2RowOverrides{Style: "solid"}, nil)
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
	grid, err := p.Expand(ExpandContext{}, v, &ProcessGrid2RowOverrides{Style: "solid"}, nil)
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
	grid, err := p.Expand(fullThemeCtx(), v, &ProcessGrid2RowOverrides{Style: "tinted"}, cellOverrides)
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
