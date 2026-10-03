package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/testutil"
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
	xml, findings, err := generateTableCell(cell, 2, nil)
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
	if _, findings, _ := generateTableCell(cell, 2, nil); len(findings) != 0 {
		t.Errorf("a table that fits reports nothing, got %+v", findings)
	}
}

// twoStackedFiveRowTables is tests/quality/fixtures/two-tables-stacked.json as
// it stood before go-slide-creator-fn2ka: two 5-row tables in 48% rows of the
// midnight-blue content area, where each keeps only 3 rows.
const twoStackedFiveRowTables = `{"template":"midnight-blue","output_filename":"two-tables.pptx","slides":[{"layout_id":"slideLayout7",
"content":[{"placeholder_id":"title","type":"text","text_value":"Revenue vs. Expenses"}],
"shape_grid":{"columns":1,"gap":1,"rows":[
{"height":48,"cells":[{"table":{"headers":["Revenue Stream","Q1","Q2","Q3","Q4"],"rows":[["Product Sales","$3.2M","$3.5M","$3.8M","$4.1M"],["Services","$1.1M","$1.2M","$1.3M","$1.4M"],["Licensing","$0.8M","$0.9M","$0.9M","$1.0M"],["Support","$0.5M","$0.5M","$0.6M","$0.6M"],["Total","$5.6M","$6.1M","$6.6M","$7.1M"]]}}]},
{"height":48,"cells":[{"table":{"headers":["Expense Category","Q1","Q2","Q3","Q4"],"rows":[["COGS","$1.8M","$1.9M","$2.0M","$2.1M"],["R&D","$1.2M","$1.3M","$1.4M","$1.5M"],["Sales & Marketing","$0.9M","$1.0M","$1.1M","$1.2M"],["G&A","$0.4M","$0.4M","$0.5M","$0.5M"],["Total","$4.3M","$4.6M","$5.0M","$5.3M"]]}}]}]}}]}`

// A grid table that drops rows refuses generation (it used to publish
// "...and 2 more rows" with success: true), and validate predicts it in the
// cell generation lays out, not in the default slide bounds
// (go-slide-creator-fn2ka).
func TestGridTableTruncationRefusedAndPredicted(t *testing.T) {
	var input PresentationInput
	if err := json.Unmarshal([]byte(twoStackedFiveRowTables), &input); err != nil {
		t.Fatal(err)
	}
	applyDefaults(&input)

	geom := loadSchemaMaximaGeometry(t, "midnight-blue")
	predicted := false
	for _, f := range collectFitFindings(&input, geom.layouts, geom.width, geom.height, nil) {
		if f.Code == patterns.ErrCodeTableRowsTruncated && f.Action == "refuse" && f.Path == "/slides/0/shape_grid/rows/0/cells/0/table" {
			predicted = true
		}
	}
	if !predicted {
		t.Error("validate did not predict the truncated grid table")
	}

	_, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	loss := generationRefusal(err)
	if loss == nil || loss.Code != patterns.ErrCodeTableRowsTruncated || loss.Path != "/slides/0/shape_grid/rows/0/cells/0/table" {
		t.Fatalf("generation must refuse the dropped rows, got err=%v", err)
	}
}

// A grid table's highlight column is a tint of the template's primary fill,
// like a placeholder table's — not accent3, which is blue on forest-green
// (go-slide-creator-290o9).
func TestGenerateTableCell_HighlightColumnUsesTemplatePrimaryFill(t *testing.T) {
	spec := &types.TableSpec{
		Headers: []string{"Segment", "Revenue"},
		Rows:    [][]types.TableCell{{{Content: "North", ColSpan: 1, RowSpan: 1}, {Content: "€14m", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HighlightColumn: 2},
	}
	cell := shapegrid.ResolvedCell{
		Kind:      shapegrid.CellKindTable,
		Bounds:    pptx.RectEmu{CX: 4000000, CY: 2000000},
		TableSpec: spec,
	}
	// A pale accent1 cannot carry white text, so the primary fill is accent2.
	theme := []types.ThemeColor{{Name: "accent1", RGB: "#FFD0C0"}, {Name: "accent2", RGB: "#8E2B1F"}, {Name: "accent3", RGB: "#1565C0"}}
	xml, _, err := generateTableCell(cell, 0, theme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), `<a:schemeClr val="accent2"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr>`) {
		t.Errorf("highlight column is not an accent2 tint:\n%s", xml)
	}
	if spec.Style.HighlightAccent != "" {
		t.Error("the caller's table spec was mutated")
	}
}
