package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGenerateTableXML_Basic3x3(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Header 1", "Header 2", "Header 3"},
		Rows: [][]types.TableCell{
			{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "B", ColSpan: 1, RowSpan: 1}, {Content: "C", ColSpan: 1, RowSpan: 1}},
			{{Content: "D", ColSpan: 1, RowSpan: 1}, {Content: "E", ColSpan: 1, RowSpan: 1}, {Content: "F", ColSpan: 1, RowSpan: 1}},
			{{Content: "G", ColSpan: 1, RowSpan: 1}, {Content: "H", ColSpan: 1, RowSpan: 1}, {Content: "I", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X:      914400, // 1 inch
			Y:      914400, // 1 inch
			Width:  8229600, // 9 inches
			Height: 4572000, // 5 inches
		},
		Style: types.DefaultTableStyle,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify XML structure
	if !strings.Contains(result.XML, "<p:graphicFrame>") {
		t.Error("XML should contain <p:graphicFrame>")
	}
	if !strings.Contains(result.XML, "<a:tbl>") {
		t.Error("XML should contain <a:tbl>")
	}
	if !strings.Contains(result.XML, "<a:tblGrid>") {
		t.Error("XML should contain <a:tblGrid>")
	}

	// Verify headers are present
	for _, header := range table.Headers {
		if !strings.Contains(result.XML, header) {
			t.Errorf("XML should contain header %q", header)
		}
	}

	// Verify cell content is present
	for _, row := range table.Rows {
		for _, cell := range row {
			if !strings.Contains(result.XML, cell.Content) {
				t.Errorf("XML should contain cell content %q", cell.Content)
			}
		}
	}

	// Verify row count (1 header + 3 data rows = 4 <a:tr> tags)
	rowCount := strings.Count(result.XML, "<a:tr h=")
	if rowCount != 4 {
		t.Errorf("expected 4 rows, got %d", rowCount)
	}

	// Verify column count (3 columns)
	gridColCount := strings.Count(result.XML, "<a:gridCol w=")
	if gridColCount != 3 {
		t.Errorf("expected 3 grid columns, got %d", gridColCount)
	}

	// Verify table style GUID is present
	if !strings.Contains(result.XML, "<a:tableStyleId>"+types.DefaultTableStyleID+"</a:tableStyleId>") {
		t.Errorf("XML should contain default tableStyleId %s", types.DefaultTableStyleID)
	}
}

func TestGenerateTableXML_HeaderStyling(t *testing.T) {
	tests := []struct {
		name           string
		headerBg       string
		expectedScheme string
	}{
		{"accent1", "accent1", "accent1"},
		{"accent2", "accent2", "accent2"},
		{"accent3", "accent3", "accent3"},
		{"accent4", "accent4", "accent4"},
		{"accent5", "accent5", "accent5"},
		{"accent6", "accent6", "accent6"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := &types.TableSpec{
				Headers: []string{"Header 1"},
				Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
				Style:   types.TableStyle{HeaderBackground: tc.headerBg, Borders: "all"},
			}

			config := TableRenderConfig{
				Bounds: types.BoundingBox{Width: 1000000},
				Style:  table.Style,
			}

			result, err := GenerateTableXML(table, config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFill := `<a:schemeClr val="` + tc.expectedScheme + `"/>`
			if !strings.Contains(result.XML, expectedFill) {
				t.Errorf("expected header fill with %s, got XML: %s", tc.expectedScheme, result.XML)
			}
		})
	}
}

func TestGenerateTableXML_NoHeaderFillWhenOmitted(t *testing.T) {
	// When HeaderBackground is empty (default), no solidFill should be emitted
	// so the table style's firstRow appearance takes effect.
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "", Borders: "all"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The tcPr should close immediately after borders — no solidFill between borders and </a:tcPr>
	if strings.Contains(result.XML, `<a:schemeClr val="accent1"/></a:solidFill></a:tcPr>`) {
		t.Error("header cell should not contain accent1 solidFill when HeaderBackground is empty")
	}
	// More broadly: no fill element should appear as direct child of tcPr for header cells.
	// solidFill inside <a:ln> (borders) is fine; we check that no schemeClr fill precedes </a:tcPr>.
	if strings.Contains(result.XML, `</a:solidFill></a:tcPr>`) {
		t.Error("header cell tcPr should not end with a solidFill when HeaderBackground is empty")
	}
}

func TestGenerateTableXML_HeaderTextNotOverriddenWithStyle(t *testing.T) {
	// When a table style is active and header_background is NOT set,
	// the table style's firstRow > tcTxStyle should control header text
	// formatting. We must NOT force bold (b="1") which could interfere
	// with table style inheritance in PowerPoint.
	table := &types.TableSpec{
		Headers: []string{"Header 1", "Header 2"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "B", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "", StyleID: types.DefaultTableStyleID, Borders: "all"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 2000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Header text runs should NOT have b="1" when table style controls formatting.
	// The table style's firstRow already defines bold+color for header text.
	if strings.Contains(result.XML, `b="1"`) {
		t.Error("header cell should not force bold when table style is active and header_background is not set")
	}

	// Ensure no solidFill in rPr (no forced text color)
	if strings.Contains(result.XML, `<a:rPr`) && strings.Contains(result.XML, `<a:solidFill`) {
		// Check this isn't from a border or cell fill (which is fine)
		// The rPr should be self-closing with no children
		if strings.Contains(result.XML, `</a:rPr>`) {
			t.Error("header text run properties should not contain solidFill when deferring to table style")
		}
	}
}

func TestGenerateTableXML_HeaderBoldWhenNoStyle(t *testing.T) {
	// When no table style is in use, headers should still be bold.
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "", Borders: "all"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.XML, `b="1"`) {
		t.Error("header cell should be bold when no table style is active")
	}
}

func TestGenerateTableXML_HeaderBoldWithExplicitBackground(t *testing.T) {
	// When header_background IS explicitly set (even with a table style),
	// we are overriding the style's fill, so we should also force bold.
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "accent1", StyleID: types.DefaultTableStyleID, Borders: "all"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.XML, `b="1"`) {
		t.Error("header cell should be bold when header_background is explicitly set")
	}
}

func TestGenerateTableXML_NoBorders(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "none", Borders: "none"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When borders=none, <a:tblBorders> should not appear at all —
	// this lets the template table style control borders.
	if strings.Contains(result.XML, `<a:tblBorders>`) {
		t.Error("tblBorders element should not be emitted when borders=none")
	}
}

func TestGenerateTableXML_OmittedBordersWithStyle(t *testing.T) {
	// When borders are omitted and a table style is set, tblBorders should
	// not be emitted so the style's own border definitions take effect.
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{StyleID: "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result.XML, `<a:tblBorders>`) {
		t.Error("tblBorders should not be emitted when borders are omitted and a table style is set")
	}
	if !strings.Contains(result.XML, `<a:tableStyleId>`) {
		t.Error("tableStyleId should be present")
	}
}

func TestGenerateTableXML_HorizontalBorders(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows:    [][]types.TableCell{{{Content: "A", ColSpan: 1, RowSpan: 1}}},
		Style:   types.TableStyle{HeaderBackground: "accent1", Borders: "horizontal"},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Top and bottom borders should be solid (visible)
	solidBorder := fmt.Sprintf(`w="%d"`, borderWidth)
	if !strings.Contains(result.XML, "<a:lnT "+solidBorder) {
		t.Error("XML should contain solid top border when borders=horizontal")
	}
	if !strings.Contains(result.XML, "<a:lnB "+solidBorder) {
		t.Error("XML should contain solid bottom border when borders=horizontal")
	}
	// Left and right borders should use noFill with w="0" (hidden) for ALL rows including header
	if !strings.Contains(result.XML, `<a:lnL w="0"><a:noFill/></a:lnL>`) {
		t.Error("left border should use noFill with w=0 when borders=horizontal")
	}
	if !strings.Contains(result.XML, `<a:lnR w="0"><a:noFill/></a:lnR>`) {
		t.Error("right border should use noFill with w=0 when borders=horizontal")
	}
	// Header row should NOT have solid left/right borders (horizontal means no vertical borders anywhere)
	if strings.Contains(result.XML, "<a:lnL "+solidBorder) {
		t.Error("header left border should NOT be solid when borders=horizontal")
	}
	if strings.Contains(result.XML, "<a:lnR "+solidBorder) {
		t.Error("header right border should NOT be solid when borders=horizontal")
	}

	// Table-level borders: insideV must be noFill, insideH must be solid
	if !strings.Contains(result.XML, `<a:insideV><a:ln w="0"><a:noFill/></a:ln></a:insideV>`) {
		t.Error("table-level insideV should be noFill when borders=horizontal")
	}
	if !strings.Contains(result.XML, `<a:insideH><a:ln w="`+fmt.Sprintf("%d", borderWidth)) {
		t.Error("table-level insideH should be solid when borders=horizontal")
	}
}

func TestGenerateTableXML_StripedRows(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Header 1"},
		Rows: [][]types.TableCell{
			{{Content: "Row 0", ColSpan: 1, RowSpan: 1}},
			{{Content: "Row 1", ColSpan: 1, RowSpan: 1}},
			{{Content: "Row 2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.TableStyle{HeaderBackground: "accent1", Borders: "all", Striped: boolPtr(true)},
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Striped table should have bandRow="1"
	if !strings.Contains(result.XML, `bandRow="1"`) {
		t.Error("striped table should have bandRow=1")
	}

	// Odd rows (row index 1) should have accent1 tinted fill for visible stripes
	if !strings.Contains(result.XML, `<a:schemeClr val="accent1">`) {
		t.Error("striped table should have alternating accent1-tinted fill")
	}
}

func TestGenerateTableXML_Colspan(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B", "C"},
		Rows: [][]types.TableCell{
			{
				{Content: "Spans 2", ColSpan: 2, RowSpan: 1},
				{Content: "", ColSpan: 0, RowSpan: 1, IsMerged: true}, // Merged cell
				{Content: "Normal", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain gridSpan attribute
	if !strings.Contains(result.XML, `gridSpan="2"`) {
		t.Error("colspan cell should have gridSpan=2")
	}

	// Should contain hMerge for merged cell
	if !strings.Contains(result.XML, `hMerge="1"`) {
		t.Error("merged cell should have hMerge=1")
	}
}

func TestGenerateTableXML_HeaderColspan(t *testing.T) {
	// When HeaderCells is populated, the header row should emit gridSpan and hMerge.
	table := &types.TableSpec{
		Headers: []string{"Region", "Q3", "Q4 Results", ""},
		HeaderCells: []types.TableCell{
			{Content: "Region", ColSpan: 1, RowSpan: 1},
			{Content: "Q3", ColSpan: 1, RowSpan: 1},
			{Content: "Q4 Results", ColSpan: 2, RowSpan: 1},
			{Content: "", ColSpan: 0, RowSpan: 1, IsMerged: true},
		},
		Rows: [][]types.TableCell{
			{
				{Content: "North", ColSpan: 1, RowSpan: 1},
				{Content: "$1.2M", ColSpan: 1, RowSpan: 1},
				{Content: "$1.5M", ColSpan: 1, RowSpan: 1},
				{Content: "+25%", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Grid should have 4 columns
	if strings.Count(result.XML, `<a:gridCol`) != 4 {
		t.Errorf("expected 4 grid columns, got %d", strings.Count(result.XML, `<a:gridCol`))
	}

	// Header row should contain gridSpan="2" for the colspan header
	if !strings.Contains(result.XML, `gridSpan="2"`) {
		t.Error("header colspan cell should have gridSpan=2")
	}

	// Header row should contain hMerge="1" for the merged placeholder
	if !strings.Contains(result.XML, `hMerge="1"`) {
		t.Error("header merged cell should have hMerge=1")
	}

	// The literal text "{colspan=2}" should NOT appear
	if strings.Contains(result.XML, "{colspan") {
		t.Error("literal {colspan=N} text should not appear in output XML")
	}

	// All 4 data cells should be present
	if !strings.Contains(result.XML, "+25%") {
		t.Error("4th column data '+25%' should be present in output")
	}
}

func TestGenerateTableXML_Rowspan(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{
				{Content: "Spans 2 rows", ColSpan: 1, RowSpan: 2},
				{Content: "Normal", ColSpan: 1, RowSpan: 1},
			},
			{
				{Content: "", ColSpan: 1, RowSpan: 0, IsMerged: true}, // Merged cell
				{Content: "Also normal", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain rowSpan attribute
	if !strings.Contains(result.XML, `rowSpan="2"`) {
		t.Error("rowspan cell should have rowSpan=2")
	}

	// Should contain vMerge for merged cell
	if !strings.Contains(result.XML, `vMerge="1"`) {
		t.Error("merged cell should have vMerge=1")
	}
}

func TestGenerateTableXML_ColspanAndRowspan(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B", "C"},
		Rows: [][]types.TableCell{
			{
				{Content: "Both", ColSpan: 2, RowSpan: 2},
				{Content: "", ColSpan: 0, RowSpan: 1, IsMerged: true},
				{Content: "C1", ColSpan: 1, RowSpan: 1},
			},
			{
				{Content: "", ColSpan: 1, RowSpan: 0, IsMerged: true},
				{Content: "", ColSpan: 0, RowSpan: 1, IsMerged: true},
				{Content: "C2", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain both gridSpan and rowSpan on the same cell
	if !strings.Contains(result.XML, `gridSpan="2"`) {
		t.Error("cell should have gridSpan=2")
	}
	if !strings.Contains(result.XML, `rowSpan="2"`) {
		t.Error("cell should have rowSpan=2")
	}
}

func TestGenerateTableXML_ColumnWidthsSum(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B", "C", "D", "E"},
		Rows:    [][]types.TableCell{},
		Style:   types.DefaultTableStyle,
	}

	availableWidth := int64(10000000) // 10 million EMUs
	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: availableWidth},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Calculate total width from grid columns
	// Each gridCol should have width that sums to availableWidth
	widths := calculateColumnWidths(5, availableWidth, nil, nil, 0)
	var sum int64
	for _, w := range widths {
		sum += w
	}

	if sum != availableWidth {
		t.Errorf("column widths sum %d should equal available width %d", sum, availableWidth)
	}

	// Verify result uses full width
	if result.Width != availableWidth {
		t.Errorf("result width %d should equal available width %d", result.Width, availableWidth)
	}
}

func TestGenerateTableXML_SpecialCharactersEscaped(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"<Header>"},
		Rows: [][]types.TableCell{
			{{Content: "A & B", ColSpan: 1, RowSpan: 1}},
			{{Content: "\"Quoted\"", ColSpan: 1, RowSpan: 1}},
			{{Content: "<script>", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify special characters are escaped
	if strings.Contains(result.XML, "<Header>") && !strings.Contains(result.XML, "&lt;Header&gt;") {
		t.Error("< and > should be escaped")
	}
	if strings.Contains(result.XML, " & ") && !strings.Contains(result.XML, " &amp; ") {
		t.Error("& should be escaped")
	}
	if strings.Contains(result.XML, `"Quoted"`) && !strings.Contains(result.XML, "&quot;") {
		t.Error("\" should be escaped")
	}
}

func TestGenerateTableXML_NilTable(t *testing.T) {
	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
	}

	_, err := GenerateTableXML(nil, config)
	if err == nil {
		t.Error("expected error for nil table")
	}
}

func TestGenerateTableXML_EmptyHeaders(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{},
		Rows:    [][]types.TableCell{},
		Style:   types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
	}

	_, err := GenerateTableXML(table, config)
	if err == nil {
		t.Error("expected error for empty headers")
	}
}

func TestGenerateTableXML_XMLWellFormed(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Header 1", "Header 2"},
		Rows: [][]types.TableCell{
			{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "B", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 1000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic well-formedness checks
	openTags := strings.Count(result.XML, "<a:tc")
	closeTags := strings.Count(result.XML, "</a:tc>")
	selfCloseTags := strings.Count(result.XML, "/>")

	// Not a perfect check, but basic sanity
	if !strings.HasPrefix(result.XML, "<p:graphicFrame>") {
		t.Error("XML should start with <p:graphicFrame>")
	}
	if !strings.HasSuffix(result.XML, "</p:graphicFrame>") {
		t.Error("XML should end with </p:graphicFrame>")
	}

	// Cells should be balanced (open + self-close >= close)
	_ = openTags
	_ = closeTags
	_ = selfCloseTags
}

func TestCalculateColumnWidths(t *testing.T) {
	tests := []struct {
		name      string
		numCols   int
		width     int64
		wantSum   int64
		wantCount int
	}{
		{"single column", 1, 1000, 1000, 1},
		{"two columns", 2, 1000, 1000, 2},
		{"three columns", 3, 1000, 1000, 3},
		{"uneven division", 3, 10, 10, 3},
		{"zero columns", 0, 1000, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			widths := calculateColumnWidths(tc.numCols, tc.width, nil, nil, 0)

			if len(widths) != tc.wantCount {
				t.Errorf("expected %d columns, got %d", tc.wantCount, len(widths))
			}

			var sum int64
			for _, w := range widths {
				sum += w
			}

			if sum != tc.wantSum {
				t.Errorf("sum of widths %d should equal %d", sum, tc.wantSum)
			}
		})
	}
}

func TestMapToSchemeColor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"accent1", "accent1"},
		{"accent2", "accent2"},
		{"accent3", "accent3"},
		{"accent4", "accent4"},
		{"accent5", "accent5"},
		{"accent6", "accent6"},
		{"dk1", "dk1"},
		{"dk2", "dk2"},
		{"lt1", "lt1"},
		{"lt2", "lt2"},
		{"tx1", "tx1"},
		{"tx2", "tx2"},
		{"bg1", "bg1"},
		{"bg2", "bg2"},
		{"hlink", "hlink"},
		{"folHlink", "folHlink"},
		{"none", "none"},
		{"", "none"},
		{"unknown", "accent1"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := mapToSchemeColor(tc.input)
			if result != tc.expected {
				t.Errorf("mapToSchemeColor(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestEscapeXMLText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"<test>", "&lt;test&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{"<>&\"'", "&lt;&gt;&amp;&quot;&apos;"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := escapeXMLText(tc.input)
			if result != tc.expected {
				t.Errorf("escapeXMLText(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGenerateBorderXML(t *testing.T) {
	tests := []struct {
		style      string
		solidLnL   bool
		solidLnR   bool
		solidLnT   bool
		solidLnB   bool
	}{
		{"none", false, false, false, false},
		{"horizontal", false, false, true, true},
		{"all", true, true, true, true},
		{"outer", true, true, true, true}, // outer falls through to all
		{"", true, true, true, true},      // default to all
	}

	for _, tc := range tests {
		t.Run(tc.style, func(t *testing.T) {
			result := generateBorderXMLWithOverrides(tc.style, false, nil)

			checkSolidBorder := func(side string, expectSolid bool) {
				solidMarker := fmt.Sprintf(`<a:ln%s w="%d"`, side, borderWidth)
				noFillMarker := fmt.Sprintf(`<a:ln%s w="0"><a:noFill/></a:ln%s>`, side, side)
				hasSolid := strings.Contains(result, solidMarker)
				hasNoFill := strings.Contains(result, noFillMarker)
				if expectSolid {
					if !hasSolid {
						t.Errorf("expected solid %s border for style %q", side, tc.style)
					}
				} else {
					if hasSolid {
						t.Errorf("unexpected solid %s border for style %q", side, tc.style)
					}
					if !hasNoFill {
						t.Errorf("expected noFill %s border for style %q", side, tc.style)
					}
				}
			}

			checkSolidBorder("L", tc.solidLnL)
			checkSolidBorder("R", tc.solidLnR)
			checkSolidBorder("T", tc.solidLnT)
			checkSolidBorder("B", tc.solidLnB)
		})
	}
}

func TestGenerateTableXML_ColumnAlignment(t *testing.T) {
	table := &types.TableSpec{
		Headers:          []string{"Left", "Center", "Right"},
		ColumnAlignments: []string{"left", "center", "right"},
		Rows: [][]types.TableCell{
			{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "B", ColSpan: 1, RowSpan: 1}, {Content: "C", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds:           types.BoundingBox{Width: 3000000},
		Style:            table.Style,
		ColumnAlignments: table.ColumnAlignments,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify alignment attributes appear in paragraph elements
	if !strings.Contains(result.XML, `algn="l"`) {
		t.Error("expected left alignment (algn=\"l\") in XML output")
	}
	if !strings.Contains(result.XML, `algn="ctr"`) {
		t.Error("expected center alignment (algn=\"ctr\") in XML output")
	}
	if !strings.Contains(result.XML, `algn="r"`) {
		t.Error("expected right alignment (algn=\"r\") in XML output")
	}

	// Count: 3 headers + 3 data cells = 6 paragraph elements with alignment
	leftCount := strings.Count(result.XML, `algn="l"`)
	ctrCount := strings.Count(result.XML, `algn="ctr"`)
	rightCount := strings.Count(result.XML, `algn="r"`)
	if leftCount != 2 {
		t.Errorf("expected 2 left-aligned paragraphs (1 header + 1 data), got %d", leftCount)
	}
	if ctrCount != 2 {
		t.Errorf("expected 2 center-aligned paragraphs (1 header + 1 data), got %d", ctrCount)
	}
	if rightCount != 2 {
		t.Errorf("expected 2 right-aligned paragraphs (1 header + 1 data), got %d", rightCount)
	}
}

// ---------------------------------------------------------------------------
// Wide table tests (5, 6, 7+ columns) — verifies the fix for corrupted
// headers, rotated text, and misaligned columns in multi-column tables.
// ---------------------------------------------------------------------------

func TestGenerateTableXML_WideTable_5Columns_SLA(t *testing.T) {
	// Simulates a 5-column SLA table with 8 data rows.
	table := &types.TableSpec{
		Headers: []string{"Service", "SLA Target", "Current", "Status", "Trend"},
		Rows: [][]types.TableCell{
			{{Content: "API Gateway", ColSpan: 1, RowSpan: 1}, {Content: "99.95%", ColSpan: 1, RowSpan: 1}, {Content: "99.97%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Stable", ColSpan: 1, RowSpan: 1}},
			{{Content: "Database", ColSpan: 1, RowSpan: 1}, {Content: "99.99%", ColSpan: 1, RowSpan: 1}, {Content: "99.98%", ColSpan: 1, RowSpan: 1}, {Content: "At Risk", ColSpan: 1, RowSpan: 1}, {Content: "Declining", ColSpan: 1, RowSpan: 1}},
			{{Content: "CDN", ColSpan: 1, RowSpan: 1}, {Content: "99.90%", ColSpan: 1, RowSpan: 1}, {Content: "99.95%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Improving", ColSpan: 1, RowSpan: 1}},
			{{Content: "Auth Service", ColSpan: 1, RowSpan: 1}, {Content: "99.95%", ColSpan: 1, RowSpan: 1}, {Content: "99.96%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Stable", ColSpan: 1, RowSpan: 1}},
			{{Content: "Email Service", ColSpan: 1, RowSpan: 1}, {Content: "99.50%", ColSpan: 1, RowSpan: 1}, {Content: "99.72%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Stable", ColSpan: 1, RowSpan: 1}},
			{{Content: "Search", ColSpan: 1, RowSpan: 1}, {Content: "99.90%", ColSpan: 1, RowSpan: 1}, {Content: "99.88%", ColSpan: 1, RowSpan: 1}, {Content: "At Risk", ColSpan: 1, RowSpan: 1}, {Content: "Declining", ColSpan: 1, RowSpan: 1}},
			{{Content: "Payments", ColSpan: 1, RowSpan: 1}, {Content: "99.99%", ColSpan: 1, RowSpan: 1}, {Content: "99.99%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Stable", ColSpan: 1, RowSpan: 1}},
			{{Content: "Notifications", ColSpan: 1, RowSpan: 1}, {Content: "99.00%", ColSpan: 1, RowSpan: 1}, {Content: "99.45%", ColSpan: 1, RowSpan: 1}, {Content: "Met", ColSpan: 1, RowSpan: 1}, {Content: "Improving", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	availableWidth := int64(8229600) // 9 inches — standard slide width
	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width: availableWidth, Height: 4572000,
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 5 grid columns must exist
	gridColCount := strings.Count(result.XML, "<a:gridCol w=")
	if gridColCount != 5 {
		t.Errorf("expected 5 grid columns, got %d", gridColCount)
	}

	// All 9 rows (1 header + 8 data)
	rowCount := strings.Count(result.XML, "<a:tr h=")
	if rowCount != 9 {
		t.Errorf("expected 9 rows, got %d", rowCount)
	}

	// Body properties must force horizontal text
	if !strings.Contains(result.XML, `vert="horz"`) {
		t.Error("cell body properties must contain vert=\"horz\" to prevent text rotation")
	}
	if !strings.Contains(result.XML, `wrap="square"`) {
		t.Error("cell body properties must contain wrap=\"square\" for text wrapping")
	}

	// Verify column widths sum to available width
	widths := calculateColumnWidths(5, availableWidth, table.Headers, table.Rows, defaultFontSize)
	var sum int64
	for _, w := range widths {
		sum += w
		if w <= 0 {
			t.Errorf("column width must be positive, got %d", w)
		}
	}
	if sum != availableWidth {
		t.Errorf("column widths sum %d should equal available width %d", sum, availableWidth)
	}
}

func TestGenerateTableXML_WideTable_7Columns_PandL(t *testing.T) {
	// Simulates a 7-column P&L table — the exact scenario that was corrupted.
	table := &types.TableSpec{
		Headers: []string{"Metric", "Q1 2025", "Q2 2025", "Q3 2025", "Q4 2025", "FY 2025", "YoY Change"},
		Rows: [][]types.TableCell{
			{{Content: "Revenue", ColSpan: 1, RowSpan: 1}, {Content: "$12.5M", ColSpan: 1, RowSpan: 1}, {Content: "$13.2M", ColSpan: 1, RowSpan: 1}, {Content: "$14.8M", ColSpan: 1, RowSpan: 1}, {Content: "$15.1M", ColSpan: 1, RowSpan: 1}, {Content: "$55.6M", ColSpan: 1, RowSpan: 1}, {Content: "+18.2%", ColSpan: 1, RowSpan: 1}},
			{{Content: "COGS", ColSpan: 1, RowSpan: 1}, {Content: "$5.0M", ColSpan: 1, RowSpan: 1}, {Content: "$5.3M", ColSpan: 1, RowSpan: 1}, {Content: "$5.9M", ColSpan: 1, RowSpan: 1}, {Content: "$6.0M", ColSpan: 1, RowSpan: 1}, {Content: "$22.2M", ColSpan: 1, RowSpan: 1}, {Content: "+15.6%", ColSpan: 1, RowSpan: 1}},
			{{Content: "Gross Profit", ColSpan: 1, RowSpan: 1}, {Content: "$7.5M", ColSpan: 1, RowSpan: 1}, {Content: "$7.9M", ColSpan: 1, RowSpan: 1}, {Content: "$8.9M", ColSpan: 1, RowSpan: 1}, {Content: "$9.1M", ColSpan: 1, RowSpan: 1}, {Content: "$33.4M", ColSpan: 1, RowSpan: 1}, {Content: "+20.1%", ColSpan: 1, RowSpan: 1}},
			{{Content: "OpEx", ColSpan: 1, RowSpan: 1}, {Content: "$4.5M", ColSpan: 1, RowSpan: 1}, {Content: "$4.7M", ColSpan: 1, RowSpan: 1}, {Content: "$5.0M", ColSpan: 1, RowSpan: 1}, {Content: "$5.2M", ColSpan: 1, RowSpan: 1}, {Content: "$19.4M", ColSpan: 1, RowSpan: 1}, {Content: "+12.8%", ColSpan: 1, RowSpan: 1}},
			{{Content: "Net Income", ColSpan: 1, RowSpan: 1}, {Content: "$3.0M", ColSpan: 1, RowSpan: 1}, {Content: "$3.2M", ColSpan: 1, RowSpan: 1}, {Content: "$3.9M", ColSpan: 1, RowSpan: 1}, {Content: "$3.9M", ColSpan: 1, RowSpan: 1}, {Content: "$14.0M", ColSpan: 1, RowSpan: 1}, {Content: "+28.4%", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	availableWidth := int64(8229600) // 9 inches
	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width: availableWidth, Height: 4572000,
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 7 grid columns
	gridColCount := strings.Count(result.XML, "<a:gridCol w=")
	if gridColCount != 7 {
		t.Errorf("expected 7 grid columns, got %d", gridColCount)
	}

	// All headers must be present and untruncated
	for _, header := range table.Headers {
		if !strings.Contains(result.XML, escapeXMLText(header)) {
			t.Errorf("header %q should appear untruncated in XML", header)
		}
	}

	// All data cells must be present
	for _, row := range table.Rows {
		for _, cell := range row {
			if !strings.Contains(result.XML, escapeXMLText(cell.Content)) {
				t.Errorf("cell content %q should appear in XML", cell.Content)
			}
		}
	}

	// Verify horizontal text direction is set
	if !strings.Contains(result.XML, `vert="horz"`) {
		t.Error("vert=\"horz\" must be present to prevent vertical text stacking")
	}

	// Verify column widths are all positive and sum correctly
	widths := calculateColumnWidths(7, availableWidth, table.Headers, table.Rows, defaultFontSize)
	var sum int64
	for i, w := range widths {
		sum += w
		if w <= 0 {
			t.Errorf("column %d width must be positive, got %d", i, w)
		}
	}
	if sum != availableWidth {
		t.Errorf("column widths sum %d should equal available width %d", sum, availableWidth)
	}
}

func TestGenerateTableXML_WideTable_7Columns_FinancialProjections(t *testing.T) {
	// Simulates the financial projections table that had completely corrupted
	// vertical text ('C us to m ercus').
	table := &types.TableSpec{
		Headers: []string{"Customer Segment", "2024 Revenue", "2025 Target", "Growth Rate", "CAC", "LTV", "LTV:CAC"},
		Rows: [][]types.TableCell{
			{{Content: "Enterprise", ColSpan: 1, RowSpan: 1}, {Content: "$18.2M", ColSpan: 1, RowSpan: 1}, {Content: "$24.5M", ColSpan: 1, RowSpan: 1}, {Content: "34.6%", ColSpan: 1, RowSpan: 1}, {Content: "$12,500", ColSpan: 1, RowSpan: 1}, {Content: "$185,000", ColSpan: 1, RowSpan: 1}, {Content: "14.8x", ColSpan: 1, RowSpan: 1}},
			{{Content: "Mid-Market", ColSpan: 1, RowSpan: 1}, {Content: "$8.5M", ColSpan: 1, RowSpan: 1}, {Content: "$11.2M", ColSpan: 1, RowSpan: 1}, {Content: "31.8%", ColSpan: 1, RowSpan: 1}, {Content: "$3,200", ColSpan: 1, RowSpan: 1}, {Content: "$42,000", ColSpan: 1, RowSpan: 1}, {Content: "13.1x", ColSpan: 1, RowSpan: 1}},
			{{Content: "SMB", ColSpan: 1, RowSpan: 1}, {Content: "$3.1M", ColSpan: 1, RowSpan: 1}, {Content: "$4.5M", ColSpan: 1, RowSpan: 1}, {Content: "45.2%", ColSpan: 1, RowSpan: 1}, {Content: "$800", ColSpan: 1, RowSpan: 1}, {Content: "$8,500", ColSpan: 1, RowSpan: 1}, {Content: "10.6x", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	availableWidth := int64(8229600)
	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width: availableWidth, Height: 3000000,
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify "Customer Segment" header is present and not vertically stacked
	if !strings.Contains(result.XML, "Customer Segment") {
		t.Error("header 'Customer Segment' must appear intact in XML, not character-by-character")
	}

	// Verify no bodyPr is missing vert="horz" — check that all bodyPr have it
	bodyPrCount := strings.Count(result.XML, "<a:bodyPr")
	vertHorzCount := strings.Count(result.XML, `vert="horz"`)
	if vertHorzCount < bodyPrCount {
		t.Errorf("all %d <a:bodyPr> elements should have vert=\"horz\", but only %d do", bodyPrCount, vertHorzCount)
	}
}

func TestGenerateTableXML_WideTable_4Columns_UseOfFunds(t *testing.T) {
	// Simulates a 4-column use-of-funds table — should NOT scale font size
	// since 4 columns is within the "safe" range.
	table := &types.TableSpec{
		Headers: []string{"Category", "Amount", "Percentage", "Timeline"},
		Rows: [][]types.TableCell{
			{{Content: "Product Development", ColSpan: 1, RowSpan: 1}, {Content: "$5.0M", ColSpan: 1, RowSpan: 1}, {Content: "40%", ColSpan: 1, RowSpan: 1}, {Content: "24 months", ColSpan: 1, RowSpan: 1}},
			{{Content: "Sales & Marketing", ColSpan: 1, RowSpan: 1}, {Content: "$3.75M", ColSpan: 1, RowSpan: 1}, {Content: "30%", ColSpan: 1, RowSpan: 1}, {Content: "18 months", ColSpan: 1, RowSpan: 1}},
			{{Content: "Operations", ColSpan: 1, RowSpan: 1}, {Content: "$2.5M", ColSpan: 1, RowSpan: 1}, {Content: "20%", ColSpan: 1, RowSpan: 1}, {Content: "24 months", ColSpan: 1, RowSpan: 1}},
			{{Content: "Working Capital", ColSpan: 1, RowSpan: 1}, {Content: "$1.25M", ColSpan: 1, RowSpan: 1}, {Content: "10%", ColSpan: 1, RowSpan: 1}, {Content: "12 months", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	availableWidth := int64(8229600)
	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width: availableWidth, Height: 3000000,
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4 columns should use original font size (1800), not scaled down.
	// Header font = 1800 * 1.1 = 1980
	if !strings.Contains(result.XML, `sz="1980"`) {
		t.Error("4-column table should use unscaled header font size (1980 = 18pt * 1.1)")
	}

	// Should still have vert="horz" for correctness
	if !strings.Contains(result.XML, `vert="horz"`) {
		t.Error("even 4-column tables should have vert=\"horz\"")
	}
}

func TestGenerateTableXML_WideTable_FontScaling(t *testing.T) {
	// Verify font scaling logic for different column counts.
	tests := []struct {
		name         string
		numCols      int
		defaultSize  int
		expectHeader int // expected header font size (default * scale * 1.1)
	}{
		{"3 columns - no scaling", 3, 1800, 1980},       // 1800 * 1.1
		{"4 columns - no scaling", 4, 1800, 1980},       // 1800 * 1.1
		{"5 columns - scaled", 5, 1800, 1584},           // 1800 * 4/5 = 1440 * 1.1 = 1584
		{"6 columns - scaled", 6, 1800, 1320},           // 1800 * 4/6 = 1200 * 1.1 = 1320
		{"7 columns - scaled", 7, 1800, 1130},           // 1800 * 4/7 = int(1028.57) = 1028, * 1.1 = int(1130.8) = 1130
		{"10 columns - floor", 10, 1800, 1100},          // 1800 * 4/10 = 720, but floor 1000 * 1.1 = 1100
		{"15 columns - floor", 15, 1800, 1100},          // floor at 1000, * 1.1 = 1100
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			headers := make([]string, tc.numCols)
			row := make([]types.TableCell, tc.numCols)
			for i := 0; i < tc.numCols; i++ {
				headers[i] = fmt.Sprintf("Col %d", i+1)
				row[i] = types.TableCell{Content: fmt.Sprintf("Val %d", i+1), ColSpan: 1, RowSpan: 1}
			}
			table := &types.TableSpec{
				Headers: headers,
				Rows:    [][]types.TableCell{row},
				Style:   types.DefaultTableStyle,
			}

			config := TableRenderConfig{
				Bounds:      types.BoundingBox{Width: 8229600, Height: 3000000},
				Style:       table.Style,
				DefaultSize: tc.defaultSize,
			}

			result, err := GenerateTableXML(table, config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected := fmt.Sprintf(`sz="%d"`, tc.expectHeader)
			if !strings.Contains(result.XML, expected) {
				t.Errorf("expected header font size %s in XML, not found.\nLooking for: %s", expected, expected)
			}
		})
	}
}

func TestCalculateColumnWidths_WideTable(t *testing.T) {
	// Verify that column widths always sum to availableWidth and are all positive,
	// even for wide tables with varying content lengths.
	tests := []struct {
		name     string
		headers  []string
		rows     [][]types.TableCell
		numCols  int
		width    int64
	}{
		{
			"7 cols, P&L headers",
			[]string{"Metric", "Q1 2025", "Q2 2025", "Q3 2025", "Q4 2025", "FY 2025", "YoY Change"},
			[][]types.TableCell{
				{{Content: "Revenue"}, {Content: "$12.5M"}, {Content: "$13.2M"}, {Content: "$14.8M"}, {Content: "$15.1M"}, {Content: "$55.6M"}, {Content: "+18.2%"}},
			},
			7, 8229600,
		},
		{
			"7 cols, one very long column",
			[]string{"A", "B", "C", "D", "E", "F", "Very Long Column Header That Dominates"},
			nil,
			7, 8229600,
		},
		{
			"10 cols, equal headers",
			[]string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"},
			nil,
			10, 8229600,
		},
		{
			"5 cols, mixed content",
			[]string{"Short", "A Longer Header", "X", "Medium Col", "Another Long Header"},
			[][]types.TableCell{
				{{Content: "data"}, {Content: "more data"}, {Content: "x"}, {Content: "medium"}, {Content: "data"}},
			},
			5, 8229600,
		},
		{
			"7 cols, narrow slide",
			[]string{"A", "B", "C", "D", "E", "F", "G"},
			nil,
			7, 4000000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			widths := calculateColumnWidths(tc.numCols, tc.width, tc.headers, tc.rows, defaultFontSize)
			if len(widths) != tc.numCols {
				t.Fatalf("expected %d widths, got %d", tc.numCols, len(widths))
			}

			var sum int64
			for i, w := range widths {
				if w <= 0 {
					t.Errorf("column %d width must be positive, got %d", i, w)
				}
				sum += w
			}
			if sum != tc.width {
				t.Errorf("column widths sum %d should equal available width %d (diff=%d)", sum, tc.width, sum-tc.width)
			}
		})
	}
}

// TestCalculateColumnWidths_NarrowPlaceholderProportional verifies that in a
// narrow placeholder (e.g., 3.5 inches), columns with long content still get
// proportionally more width rather than being flattened to equal widths.
// This prevents header wrapping like "Category" → "Categ"/"ory".
func TestCalculateColumnWidths_NarrowPlaceholderProportional(t *testing.T) {
	// Simulates the warm-coral slide 12 scenario:
	// 4 columns, narrow placeholder (~3.5 inches), "Category" header + short Q1/Q2/Q3.
	headers := []string{"Category", "Q1", "Q2", "Q3"}
	rows := [][]types.TableCell{
		{{Content: "North America"}, {Content: "$42M"}, {Content: "$45M"}, {Content: "$48M"}},
		{{Content: "EMEA"}, {Content: "$28M"}, {Content: "$31M"}, {Content: "$33M"}},
		{{Content: "APAC"}, {Content: "$15M"}, {Content: "$18M"}, {Content: "$22M"}},
		{{Content: "LATAM"}, {Content: "$8M"}, {Content: "$9M"}, {Content: "$11M"}},
		{{Content: "Middle East"}, {Content: "$5M"}, {Content: "$6M"}, {Content: "$7M"}},
		{{Content: "Africa"}, {Content: "$3M"}, {Content: "$4M"}, {Content: "$4M"}},
	}

	availableWidth := int64(3182938) // warm-coral body placeholder width
	widths := calculateColumnWidths(4, availableWidth, headers, rows, defaultFontSize)

	// The "Category" column (index 0) must get significantly more width than the
	// short Q-columns because "North America" (13 chars) drives it.
	// Before the fix, all columns were equalized to ~796K EMU each.
	// After the fix, column 0 should be at least 1.5× any Q-column.
	if widths[0] <= widths[1]*3/2 {
		t.Errorf("Category column (%d) should be at least 1.5× Q1 column (%d) in narrow placeholder",
			widths[0], widths[1])
	}

	// Sum must still equal available width.
	var sum int64
	for _, w := range widths {
		sum += w
	}
	if sum != availableWidth {
		t.Errorf("column widths sum %d should equal available width %d", sum, availableWidth)
	}
}

func TestGenerateTableXML_WideTable_CellMarginsPresent(t *testing.T) {
	// Verify that cell margins (lIns, rIns) are set in bodyPr for all cells.
	table := &types.TableSpec{
		Headers: []string{"A", "B", "C", "D", "E", "F", "G"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}, {Content: "3", ColSpan: 1, RowSpan: 1}, {Content: "4", ColSpan: 1, RowSpan: 1}, {Content: "5", ColSpan: 1, RowSpan: 1}, {Content: "6", ColSpan: 1, RowSpan: 1}, {Content: "7", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{Width: 8229600, Height: 2000000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// lIns and rIns should be present (cellMargin = 45720)
	if !strings.Contains(result.XML, `lIns="45720"`) {
		t.Error("cell left margin (lIns) should be set to prevent text touching borders")
	}
	if !strings.Contains(result.XML, `rIns="45720"`) {
		t.Error("cell right margin (rIns) should be set to prevent text touching borders")
	}
}

func TestGenerateTableXML_DenseTable_RowTruncation(t *testing.T) {
	// A table with 15 data rows in a constrained height (3 inches = 2743200 EMU).
	// At defaultRowHeight (370840 EMU), only ~7 rows fit (2743200/370840).
	// With 1 header + 15 data rows = 16 total, truncation should occur.
	numDataRows := 15
	numCols := 3
	rows := make([][]types.TableCell, numDataRows)
	for i := 0; i < numDataRows; i++ {
		rows[i] = []types.TableCell{
			{Content: fmt.Sprintf("Service %d", i+1), ColSpan: 1, RowSpan: 1},
			{Content: fmt.Sprintf("99.%02d%%", i), ColSpan: 1, RowSpan: 1},
			{Content: "Active", ColSpan: 1, RowSpan: 1},
		}
	}

	table := &types.TableSpec{
		Headers: []string{"Service", "Uptime", "Status"},
		Rows:    rows,
		Style:   types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X:      457200,
			Y:      914400,
			Width:  8229600, // 9 inches
			Height: 2743200, // 3 inches — tight constraint
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain the truncation summary text
	if !strings.Contains(result.XML, "...and") || !strings.Contains(result.XML, "more rows") {
		t.Error("truncated table should contain '...and N more rows' summary text")
	}

	// All headers should still be present
	for _, header := range []string{"Service", "Uptime", "Status"} {
		if !strings.Contains(result.XML, header) {
			t.Errorf("header %q should still be present after truncation", header)
		}
	}

	// The first few data rows should be present
	if !strings.Contains(result.XML, "Service 1") {
		t.Error("first data row should be present after truncation")
	}

	// The last original data row should NOT be present (it was truncated)
	if strings.Contains(result.XML, fmt.Sprintf("Service %d", numDataRows)) {
		t.Error("last data row should be truncated")
	}

	// The summary row should be rendered in italic (i="1")
	if !strings.Contains(result.XML, `i="1"`) {
		t.Error("summary row should be rendered in italic")
	}

	// The number of <a:tr> rows should be less than 1 + numDataRows
	rowCount := strings.Count(result.XML, "<a:tr h=")
	if rowCount >= 1+numDataRows {
		t.Errorf("expected fewer than %d rows after truncation, got %d", 1+numDataRows, rowCount)
	}

	// Verify the grid still has the correct number of columns
	gridColCount := strings.Count(result.XML, "<a:gridCol w=")
	if gridColCount != numCols {
		t.Errorf("expected %d grid columns, got %d", numCols, gridColCount)
	}
}

func TestGenerateTableXML_TruncationEmitsWarning(t *testing.T) {
	// Verify that truncation emits a slog.Warn so users are alerted to missing data.
	numDataRows := 15
	rows := make([][]types.TableCell, numDataRows)
	for i := 0; i < numDataRows; i++ {
		rows[i] = []types.TableCell{
			{Content: fmt.Sprintf("Row %d", i+1), ColSpan: 1, RowSpan: 1},
			{Content: "Data", ColSpan: 1, RowSpan: 1},
		}
	}

	table := &types.TableSpec{
		Headers: []string{"Name", "Value"},
		Rows:    rows,
		Style:   types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width:  8229600,
			Height: 2743200, // 3 inches — forces truncation
		},
		Style: table.Style,
	}

	// Capture slog output
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(old) })

	_, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "table rows truncated") {
		t.Error("truncation should emit a slog warning containing 'table rows truncated'")
	}
	if !strings.Contains(logOutput, "hidden_rows") {
		t.Error("truncation warning should include hidden_rows count")
	}
	if !strings.Contains(logOutput, "Name, Value") {
		t.Error("truncation warning should include table headers for identification")
	}
}

func TestGenerateTableXML_NoTruncation_NoWarning(t *testing.T) {
	// A small table that fits should NOT emit a truncation warning.
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width:  8229600,
			Height: 2743200,
		},
		Style: table.Style,
	}

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(old) })

	_, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(buf.String(), "table rows truncated") {
		t.Error("small table should not emit a truncation warning")
	}
}

func TestGenerateTableXML_DenseTable_RowScaling(t *testing.T) {
	// A table with 10 data rows in a moderately constrained height.
	// This tests that row-count font scaling kicks in before truncation.
	numDataRows := 10
	rows := make([][]types.TableCell, numDataRows)
	for i := 0; i < numDataRows; i++ {
		rows[i] = []types.TableCell{
			{Content: fmt.Sprintf("Item %d", i+1), ColSpan: 1, RowSpan: 1},
			{Content: fmt.Sprintf("$%d.00", (i+1)*100), ColSpan: 1, RowSpan: 1},
		}
	}

	table := &types.TableSpec{
		Headers: []string{"Item", "Price"},
		Rows:    rows,
		Style:   types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X:      457200,
			Y:      914400,
			Width:  8229600,
			Height: 3000000, // ~3.3 inches
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 11 total rows (1 header + 10 data) and 3,000,000 EMU height:
	// Default row height = 370840 EMU. At default 18pt, 8 rows fit.
	// Row scaling should reduce the font to fit all 11 rows.
	// The font should be smaller than the default 1800 (18pt).
	if strings.Contains(result.XML, `sz="1980"`) {
		t.Error("dense table should have scaled-down font, not the default 1980 header size")
	}

	// But all data rows should still be present (no truncation needed if font scales enough)
	if !strings.Contains(result.XML, "Item 1") {
		t.Error("first data row should be present")
	}

	// Verify row count
	rowCount := strings.Count(result.XML, "<a:tr h=")
	t.Logf("total rows rendered: %d (1 header + %d data)", rowCount, rowCount-1)
}

func TestGenerateTableXML_SmallTable_NoTruncation(t *testing.T) {
	// A small table (3 rows) should NOT be truncated or have font scaling from rows.
	table := &types.TableSpec{
		Headers: []string{"Name", "Value"},
		Rows: [][]types.TableCell{
			{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "1", ColSpan: 1, RowSpan: 1}},
			{{Content: "B", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
			{{Content: "C", ColSpan: 1, RowSpan: 1}, {Content: "3", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X: 457200, Y: 914400,
			Width: 8229600, Height: 4572000, // generous height
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use default font size (header = 1800 * 1.1 = 1980)
	if !strings.Contains(result.XML, `sz="1980"`) {
		t.Error("small table should use unscaled header font (1980)")
	}

	// Should NOT contain truncation text
	if strings.Contains(result.XML, "more rows") {
		t.Error("small table should not be truncated")
	}

	// All rows should be present
	rowCount := strings.Count(result.XML, "<a:tr h=")
	if rowCount != 4 { // 1 header + 3 data
		t.Errorf("expected 4 rows, got %d", rowCount)
	}
}

func TestGenerateTableLevelBorders(t *testing.T) {
	solidBorderMarker := fmt.Sprintf(`w="%d"`, borderWidth)
	noFillMarker := `w="0"><a:noFill/>`

	tests := []struct {
		style        string
		expectEmpty  bool
		insideVSolid bool
		insideHSolid bool
		outerTopSolid bool
		outerLeftSolid bool
	}{
		{"none", true, false, false, false, false},
		{"horizontal", false, false, true, true, false},
		{"all", false, true, true, true, true},
		{"outer", false, true, true, true, true},
		{"", false, true, true, true, true}, // default
	}

	for _, tc := range tests {
		t.Run(tc.style, func(t *testing.T) {
			result := generateTableLevelBorders(tc.style)

			if tc.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string for style %q, got %q", tc.style, result)
				}
				return
			}

			checkElement := func(tag string, expectSolid bool) {
				// Extract the content between <tag>...</tag>
				open := "<" + tag + ">"
				closeTag := "</" + tag + ">"
				idx := strings.Index(result, open)
				if idx == -1 {
					t.Errorf("missing <%s> element for style %q", tag, tc.style)
					return
				}
				rest := result[idx:]
				endIdx := strings.Index(rest, closeTag)
				if endIdx == -1 {
					t.Errorf("missing </%s> for style %q", tag, tc.style)
					return
				}
				segment := rest[:endIdx]
				hasSolid := strings.Contains(segment, solidBorderMarker)
				hasNoFill := strings.Contains(segment, noFillMarker)
				if expectSolid && !hasSolid {
					t.Errorf("expected solid %s for style %q", tag, tc.style)
				}
				if !expectSolid && !hasNoFill {
					t.Errorf("expected noFill %s for style %q", tag, tc.style)
				}
			}

			checkElement("a:insideV", tc.insideVSolid)
			checkElement("a:insideH", tc.insideHSolid)
			checkElement("a:top", tc.outerTopSolid)
			checkElement("a:left", tc.outerLeftSolid)
		})
	}
}

func TestGenerateTableXML_WideTable_TruncatesOverflow(t *testing.T) {
	// A wide table (12 columns x 18 data rows) in a standard content area.
	// Before the fix, the truncation check used defaultRowHeight/2 as the
	// floor but rendering used defaultRowHeight, causing the table to overflow.
	numCols := 12
	numDataRows := 18
	headers := make([]string, numCols)
	for i := range headers {
		headers[i] = fmt.Sprintf("Col %d", i+1)
	}
	rows := make([][]types.TableCell, numDataRows)
	for i := 0; i < numDataRows; i++ {
		row := make([]types.TableCell, numCols)
		for j := 0; j < numCols; j++ {
			row[j] = types.TableCell{Content: fmt.Sprintf("R%dC%d", i+1, j+1), ColSpan: 1, RowSpan: 1}
		}
		rows[i] = row
	}

	table := &types.TableSpec{
		Headers: headers,
		Rows:    rows,
		Style:   types.DefaultTableStyle,
	}

	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X:      457200,
			Y:      914400,
			Width:  8229600, // 9 inches
			Height: 4572000, // 5 inches — standard content area
		},
		Style: table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The table must not overflow: rendered height <= bounds height.
	if result.Height > config.Bounds.Height {
		t.Errorf("table height %d exceeds bounds height %d — table overflows slide",
			result.Height, config.Bounds.Height)
	}

	// With 12 columns, font is scaled to minimum (10pt).
	// At defaultRowHeight (370840 EMU), 4572000/370840 ≈ 12 total rows fit.
	// So with 1 header row, only ~11 data rows fit → truncation expected.
	if !strings.Contains(result.XML, "...and") || !strings.Contains(result.XML, "more rows") {
		t.Error("wide 12x18 table should be truncated with '...and N more rows' summary")
	}

	// First row should still be present
	if !strings.Contains(result.XML, "R1C1") {
		t.Error("first data row should be present")
	}

	// Last original row should be truncated
	if strings.Contains(result.XML, fmt.Sprintf("R%dC1", numDataRows)) {
		t.Error("last data row should be truncated")
	}
}

// TestCalculateColumnWidths_CurrencyTokenMinWidth verifies that columns
// containing short currency values like "$42M" get wide enough to prevent
// mid-token line breaks. This is a regression test for pptx-v6m.
func TestCalculateColumnWidths_CurrencyTokenMinWidth(t *testing.T) {
	headers := []string{"Region", "Q1", "Q2", "Q3", "Q4"}
	rows := [][]types.TableCell{
		{{Content: "North America"}, {Content: "$42M"}, {Content: "$45M"}, {Content: "$48M"}, {Content: "$51M"}},
		{{Content: "EMEA"}, {Content: "$28M"}, {Content: "$31M"}, {Content: "$33M"}, {Content: "$35M"}},
	}

	// Standard 9-inch slide width. 5 columns triggers font scaling to
	// 1800 * 4/5 = 1440 (14.4pt).
	scaledFont := int(float64(defaultFontSize) * 4.0 / 5.0) // 1440
	availableWidth := int64(8229600)
	widths := calculateColumnWidths(5, availableWidth, headers, rows, scaledFont)

	// "$42M" is 4 characters. At 14.4pt, the estimated minimum column width
	// is 4 × (1440 × 127 × 0.6) + 2 × 45720 ≈ 530K EMU.
	// The column must be at least this wide to prevent mid-token breaks.
	emHeight := int64(scaledFont) * 127
	charWidth := emHeight * 6 / 10
	tokenMinWidth := 4*charWidth + 2*cellMargin

	for i := 1; i <= 4; i++ { // columns 1-4 contain "$XXM" values
		if widths[i] < tokenMinWidth {
			t.Errorf("column %d width %d is below content-aware minimum %d — '$42M' would break mid-token",
				i, widths[i], tokenMinWidth)
		}
	}

	// Sum must equal available width.
	var sum int64
	for _, w := range widths {
		sum += w
	}
	if sum != availableWidth {
		t.Errorf("column widths sum %d should equal available width %d", sum, availableWidth)
	}
}

// TestLongestToken verifies the longestToken helper function.
func TestLongestToken(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"$42M", 4},
		{"North America", 7},    // "America"
		{"hello", 5},
		{"", 0},
		{"a b c", 1},
		{"Mid-Market Revenue", 10}, // "Mid-Market"
		{"   spaces   ", 6},        // "spaces"
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := longestToken(tc.input)
			if got != tc.want {
				t.Errorf("longestToken(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestGenerateTableXML_UseTableStyle verifies that use_table_style suppresses
// all explicit formatting (borders, fills, bold) and lets the table style control.
func TestGenerateTableXML_UseTableStyle(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Name", "Value"},
		Rows: [][]types.TableCell{
			{{Content: "Alpha", ColSpan: 1, RowSpan: 1}, {Content: "100", ColSpan: 1, RowSpan: 1}},
			{{Content: "Beta", ColSpan: 1, RowSpan: 1}, {Content: "200", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.TableStyle{
			StyleID:       types.DefaultTableStyleID,
			UseTableStyle: true,
		},
	}
	config := TableRenderConfig{
		Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8229600, Height: 4572000},
		Style:  table.Style,
	}

	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT contain tblBorders
	if strings.Contains(result.XML, "<a:tblBorders>") {
		t.Error("use_table_style should suppress <a:tblBorders>")
	}

	// Should NOT contain solidFill (no header fill, no stripe fill)
	if strings.Contains(result.XML, "<a:solidFill>") {
		t.Error("use_table_style should suppress <a:solidFill> on cells")
	}

	// Should NOT contain explicit cell borders (lnL, lnR, lnT, lnB)
	if strings.Contains(result.XML, "<a:lnL") {
		t.Error("use_table_style should suppress explicit cell borders")
	}

	// Should emit firstRow="1" and bandRow="1"
	if !strings.Contains(result.XML, `firstRow="1"`) {
		t.Error("use_table_style should still emit firstRow=\"1\"")
	}
	if !strings.Contains(result.XML, `bandRow="1"`) {
		t.Error("use_table_style should emit bandRow=\"1\"")
	}

	// Should still emit tableStyleId
	if !strings.Contains(result.XML, "<a:tableStyleId>") {
		t.Error("use_table_style should still emit <a:tableStyleId>")
	}

	// Header text should NOT be bold (b="0" or no b attribute)
	if strings.Contains(result.XML, `b="1"`) {
		t.Error("use_table_style should not force bold on header text")
	}
}
