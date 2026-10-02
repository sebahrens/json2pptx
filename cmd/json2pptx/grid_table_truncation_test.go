package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// A grid table cell too short for its rows must report the dropped rows as
// table_rows_truncated (refuse), not render "…and N more rows" silently
// (go-slide-creator-fn2ka).
func TestGenerateTableCell_ReportsTruncation(t *testing.T) {
	rows := make([][]types.TableCell, 8)
	for i := range rows {
		rows[i] = []types.TableCell{{Content: "Segment", ColSpan: 1, RowSpan: 1}, {Content: "€14m", ColSpan: 1, RowSpan: 1}}
	}
	cell := shapegrid.ResolvedCell{
		Kind:      shapegrid.CellKindTable,
		RowIdx:    0,
		ColIdx:    1,
		Bounds:    pptx.RectEmu{X: 0, Y: 0, CX: 4000000, CY: 900000},
		TableSpec: &types.TableSpec{Headers: []string{"Segment", "Revenue"}, Rows: rows},
	}
	xml, findings, err := generateTableCell(cell, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(xml) == 0 {
		t.Fatal("no table XML")
	}
	if len(findings) != 1 || findings[0].Code != patterns.ErrCodeTableRowsTruncated || findings[0].Action != "refuse" {
		t.Fatalf("want one refusing table_rows_truncated finding, got %+v", findings)
	}
	if findings[0].Path != "/slides/2/shape_grid/rows/0/cells/1/table" {
		t.Errorf("path = %q", findings[0].Path)
	}

	cell.Bounds.CY = 9000000
	if _, findings, _ := generateTableCell(cell, 2); len(findings) != 0 {
		t.Errorf("a table that fits reports nothing, got %+v", findings)
	}
}
