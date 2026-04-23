package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestWarnTableCellOverflow_NoOverflow(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "short"}, {Content: "text"}},
		},
	}
	warnings := WarnTableCellOverflow(table, 0)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for short content, got %d", len(warnings))
		for _, w := range warnings {
			t.Logf("  measured=%d declared=%d", w.MeasuredEMU, w.DeclaredEMU)
		}
	}
}

func TestWarnTableCellOverflow_ThreeLineCell(t *testing.T) {
	// A cell with 3 lines of text should overflow a single default row height.
	table := &types.TableSpec{
		Headers: []string{"Name", "Description"},
		Rows: [][]types.TableCell{
			{
				{Content: "Item"},
				{Content: "This is a very long description that spans multiple lines because it contains enough text to wrap around the column width and then some more text to ensure it really does overflow the single row height allocated by default"},
			},
		},
	}
	warnings := WarnTableCellOverflow(table, 0)

	// Log measured values for platform-variance visibility.
	for _, w := range warnings {
		t.Logf("row=%d col=%d measured=%d declared=%d", w.Row, w.Col, w.MeasuredEMU, w.DeclaredEMU)
	}

	// Assert presence of at least one warning for the overflowing cell.
	found := false
	for _, w := range warnings {
		if w.Row == 1 && w.Col == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for row=1 col=1 (3-line cell), but none found; got %d warnings total", len(warnings))
	}
}

func TestWarnTableCellOverflow_NilTable(t *testing.T) {
	warnings := WarnTableCellOverflow(nil, 0)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for nil table, got %d", len(warnings))
	}
}

func TestWarnTableCellOverflow_EmptyHeaders(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{},
	}
	warnings := WarnTableCellOverflow(table, 0)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for empty headers, got %d", len(warnings))
	}
}

func TestWarnTableCellOverflow_SkipsMergedCells(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{
				{Content: "This is a very long description that spans multiple lines because it contains enough text to wrap around"},
				{Content: "same long text", IsMerged: true},
			},
		},
	}
	warnings := WarnTableCellOverflow(table, 0)
	// Merged cells should be skipped, so only col 0 can produce a warning.
	for _, w := range warnings {
		if w.Col == 1 {
			t.Errorf("merged cell at col=1 should be skipped, but got warning")
		}
		t.Logf("row=%d col=%d measured=%d declared=%d", w.Row, w.Col, w.MeasuredEMU, w.DeclaredEMU)
	}
}

func TestWarnTableCellOverflow_StringOutput(t *testing.T) {
	w := TableCellOverflowWarning{
		SlideIndex:  0,
		Row:         1,
		Col:         2,
		DeclaredEMU: 370840,
		MeasuredEMU: 500000,
	}
	s := w.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	t.Logf("warning string: %s", s)
}
