package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestContinuationUnderfilledUsesMeasuredTableHeight(t *testing.T) {
	table := &TableInput{Headers: []string{"Term", "Meaning"}, Rows: [][]TableCellInput{{{Content: "Owner"}, {Content: "Accountable person"}}}, ColumnWidths: []float64{1, 3}}
	bounds := types.BoundingBox{Width: 9000000, Height: 4500000}
	finding := continuationTableUnderfilled(table, bounds, "/table", 0, "Calibri")
	if finding == nil || finding.Code != patterns.ErrCodeContinuationUnderfilled || finding.Action != "review" || finding.Fix.Kind != "review_layout" {
		t.Fatalf("underfilled page missing: %+v", finding)
	}
	measured := generator.MeasureTableHeight(table.ToTableSpec(), bounds.Width, 0, "Calibri")
	if finding.Measured.HeightEMU != measured || finding.Allowed.HeightEMU != bounds.Height {
		t.Fatal("finding did not use renderer row height")
	}
	bounds.Height = measured * 2
	if f := continuationTableUnderfilled(table, bounds, "/table", 0, "Calibri"); f != nil {
		t.Fatal("exactly50percent should not warn")
	}
	bounds.Height = 4500000
	bounds.Width = 1100000
	table.Rows[0][1].Content = strings.Repeat("A complete definition with context ", 20)
	if f := continuationTableUnderfilled(table, bounds, "/table", 0, "Calibri"); f != nil {
		t.Fatalf("wrapped table treated as one row: %+v", f)
	}
}

func TestContinuationUnderfilledRequiresContinuationMarker(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "content", Name: "Content", Tags: []string{"content"}, Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderContent, Bounds: types.BoundingBox{Width: 9000000, Height: 4500000}}}}}
	for _, tc := range []struct {
		name, title string
		origin      bool
		want        int
	}{{"ordinary", "Ledger", false, 0}, {"numbered", "Ledger (2/2)", false, 1}, {"word marker", "Ledger (cont.)", false, 1}, {"origin", "Ledger", true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			title := tc.title
			slide := SlideInput{LayoutID: "content", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}, {PlaceholderID: "body", Type: "table", TableValue: &TableInput{Headers: []string{"A", "B"}, Rows: [][]TableCellInput{{{Content: "1"}, {Content: "2"}}}}}}}
			if tc.origin {
				slide.Origin = &deckinput.SlideOrigin{SplitPage: true}
			}
			findings := collectContinuationUnderfilledFindings(&PresentationInput{Slides: []SlideInput{slide}}, layouts, 12192000, 6858000, &types.ThemeInfo{BodyFont: "Calibri"})
			if len(findings) != tc.want {
				t.Fatalf("findings=%+v want%d", findings, tc.want)
			}
			if len(findings) > 0 && findings[0].Path != "/slides/0/content/1/table_value" {
				t.Fatalf("wrong authored path: %s", findings[0].Path)
			}
		})
	}
}
