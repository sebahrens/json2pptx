// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TableRenderConfig configures table rendering.
type TableRenderConfig struct {
	Bounds           types.BoundingBox // Position and size in EMUs
	Theme            *types.ThemeInfo  // Theme colors and fonts (optional)
	Style            types.TableStyle  // Table styling options
	DefaultFont      string            // Font family (default: "Calibri")
	DefaultSize      int               // Font size in hundredths of point (default: 1800 = 18pt)
	ColumnAlignments []string          // Per-column alignment: "left", "center", "right"
}

// TableRenderResult contains the generated table XML.
type TableRenderResult struct {
	XML    string // Complete graphicFrame XML
	Width  int64  // Actual width used (EMUs)
	Height int64  // Actual height used (EMUs)
}

const (
	// Default row height in EMUs (approximately 0.4 inch)
	defaultRowHeight = 370840
	// Default font size in hundredths of point (18pt)
	defaultFontSize = 1800
	// DefaultTableFontSize is the exported default font size for table cells
	// in hundredths of a point (18pt). Used by the fit-report walker.
	DefaultTableFontSize = defaultFontSize
	// Minimum font size for table cells in hundredths of point (10pt).
	// Below this, text becomes unreadable in projected presentations.
	minFontSizeForTable = 1000
	// MinTableFontSize is the exported minimum font size floor for table cells
	// in hundredths of a point (10pt). Used by the fit-report walker.
	MinTableFontSize = minFontSizeForTable
	// Border width in EMUs (1 point = 12700 EMUs)
	borderWidth = 12700
	// Default font family
	defaultFontFamily = "Calibri"
	// Cell internal margin in EMUs (approximately 0.05 inch).
	// Applied to left and right inside each cell to prevent text from
	// touching the cell border.
	cellMargin = 45720
)

// GenerateTableXML generates PPTX XML for a table.
// Returns nil if the table is nil or has no headers.
func GenerateTableXML(table *types.TableSpec, config TableRenderConfig) (*TableRenderResult, error) { //nolint:gocognit,gocyclo
	if table == nil || len(table.Headers) == 0 {
		return nil, fmt.Errorf("table must have at least one header")
	}

	numCols := len(table.Headers)
	numRows := len(table.Rows) + 1 // +1 for header row

	// Apply defaults
	if config.DefaultSize == 0 {
		config.DefaultSize = defaultFontSize
	}
	if config.DefaultFont == "" {
		config.DefaultFont = defaultFontFamily
	}

	// Scale font size down for wide tables to prevent text overflow and
	// vertical stacking.  Standard slide width (~9 in / 8229600 EMU) can
	// comfortably fit 4 columns at 18pt.  Beyond that we reduce linearly,
	// clamping to a readable minimum of 1000 (10pt).
	if numCols > 4 {
		scale := 4.0 / float64(numCols)
		scaled := int(float64(config.DefaultSize) * scale)
		if scaled < minFontSizeForTable {
			scaled = minFontSizeForTable
		}
		config.DefaultSize = scaled
	}

	// Row-count-based font scaling: when the table has more rows than can
	// fit at the current font size, scale the font down proportionally.
	// Row height scales linearly with font size relative to the default.
	if config.Bounds.Height > 0 {
		fontRatio := float64(config.DefaultSize) / float64(defaultFontSize)
		scaledRowHeight := int64(float64(defaultRowHeight) * fontRatio)
		if scaledRowHeight < 1 {
			scaledRowHeight = 1
		}
		maxVisibleRows := int(config.Bounds.Height / scaledRowHeight)
		if numRows > maxVisibleRows && maxVisibleRows > 0 {
			rowScale := float64(maxVisibleRows) / float64(numRows)
			scaled := int(float64(config.DefaultSize) * rowScale)
			if scaled < minFontSizeForTable {
				scaled = minFontSizeForTable
			}
			config.DefaultSize = scaled
		}
	}

	// After all font scaling, determine how many rows actually fit.
	// If the table still overflows at the minimum font size, truncate
	// data rows and replace the last visible row with an italic summary.
	//
	// IMPORTANT: The row height floor here MUST match the rendering floor
	// below (defaultRowHeight). Using a lower floor causes the truncation
	// check to overestimate capacity, leading to table overflow.
	summaryRowIdx := -1 // index into table.Rows; -1 means no truncation
	if config.Bounds.Height > 0 {
		fontRatio := float64(config.DefaultSize) / float64(defaultFontSize)
		scaledRowHeight := int64(float64(defaultRowHeight) * fontRatio)
		if scaledRowHeight < defaultRowHeight {
			scaledRowHeight = defaultRowHeight
		}
		maxVisibleRows := int(config.Bounds.Height / scaledRowHeight)
		headerRowCount := 1 // header always occupies one row
		dataRowCapacity := maxVisibleRows - headerRowCount
		if dataRowCapacity < 1 {
			dataRowCapacity = 1
		}
		if len(table.Rows) > dataRowCapacity {
			overflow := len(table.Rows) - dataRowCapacity + 1 // +1 to make room for summary row
			truncatedRows := table.Rows[:len(table.Rows)-overflow]
			// Build summary row with the correct number of columns
			summaryRow := make([]types.TableCell, numCols)
			summaryRow[0] = types.TableCell{
				Content: fmt.Sprintf("...and %d more rows", overflow),
				ColSpan: 1,
				RowSpan: 1,
			}
			for i := 1; i < numCols; i++ {
				summaryRow[i] = types.TableCell{Content: "", ColSpan: 1, RowSpan: 1}
			}
			table.Rows = append(truncatedRows, summaryRow)
			summaryRowIdx = len(table.Rows) - 1
			numRows = len(table.Rows) + 1 // +1 for header

			// Warn about truncation so users don't ship decks with missing data.
			tableID := strings.Join(table.Headers, ", ")
			slog.Warn("table rows truncated: data exceeds allocated height",
				slog.String("headers", tableID),
				slog.Int("total_rows", len(truncatedRows)+overflow),
				slog.Int("visible_rows", len(truncatedRows)),
				slog.Int("hidden_rows", overflow),
			)
		}
	}

	// Calculate dimensions
	colWidths := calculateColumnWidths(numCols, config.Bounds.Width, table.Headers, table.Rows, config.DefaultSize)

	// Dynamic row height: fill the placeholder evenly, with a minimum floor.
	// When rows are clamped to defaultRowHeight and the total exceeds bounds,
	// cap totalHeight to bounds so the table doesn't overflow the slide.
	var rowHeight, totalHeight int64
	if config.Bounds.Height > 0 {
		computedRowHeight := config.Bounds.Height / int64(numRows)
		if computedRowHeight < defaultRowHeight {
			computedRowHeight = defaultRowHeight
		}
		rowHeight = computedRowHeight
		totalHeight = rowHeight * int64(numRows)
		if totalHeight > config.Bounds.Height {
			totalHeight = config.Bounds.Height
		}
	} else {
		rowHeight = int64(defaultRowHeight)
		totalHeight = rowHeight * int64(numRows)
	}

	var xml strings.Builder

	// Graphic frame wrapper
	fmt.Fprintf(&xml, `<p:graphicFrame>`+
		`<p:nvGraphicFramePr>`+
		`<p:cNvPr id="4" name="Table 1"/>`+
		`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr>`+
		`<p:nvPr/>`+
		`</p:nvGraphicFramePr>`+
		`<p:xfrm>`+
		`<a:off x="%d" y="%d"/>`+
		`<a:ext cx="%d" cy="%d"/>`+
		`</p:xfrm>`+
		`<a:graphic>`+
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`+
		`<a:tbl>`,
		config.Bounds.X, config.Bounds.Y,
		config.Bounds.Width, totalHeight,
	)

	// Table properties with explicit table-level borders.
	// Without <a:tblBorders>, PowerPoint renders default grid lines
	// even when cell-level borders specify noFill.
	bandRow := "1"
	if config.Style.Striped != nil && !*config.Style.Striped {
		bandRow = "0"
	}
	fmt.Fprintf(&xml, `<a:tblPr firstRow="1" bandRow="%s">`, bandRow)
	// When use_table_style is set, skip tblBorders entirely so the style controls borders.
	// When borders are omitted and a table style is in use, also skip tblBorders
	// so the style's own border definitions (wholeTbl > tcBdr) take effect.
	if !config.Style.UseTableStyle {
		if !(config.Style.Borders == "" && config.Style.StyleID != "") {
			xml.WriteString(generateTableLevelBorders(config.Style.Borders))
		}
	}
	if config.Style.StyleID != "" {
		fmt.Fprintf(&xml, `<a:tableStyleId>%s</a:tableStyleId>`, config.Style.StyleID)
	}
	xml.WriteString(`</a:tblPr>`)

	// Column grid
	xml.WriteString(generateTableGrid(colWidths))

	// Header row
	if len(table.HeaderCells) > 0 {
		xml.WriteString(generateHeaderRowWithMerges(table.HeaderCells, rowHeight, config))
	} else {
		xml.WriteString(generateHeaderRow(table.Headers, rowHeight, config))
	}

	// Data rows
	for rowIdx, row := range table.Rows {
		if rowIdx == summaryRowIdx {
			xml.WriteString(generateSummaryRow(row, rowIdx, rowHeight, config))
		} else {
			xml.WriteString(generateDataRow(row, rowIdx, rowHeight, config))
		}
	}

	// Close elements
	xml.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)

	return &TableRenderResult{
		XML:    xml.String(),
		Width:  config.Bounds.Width,
		Height: totalHeight,
	}, nil
}

// longestToken returns the length of the longest whitespace-separated token
// in s. For example, "North America" → 7 ("America"), "$42M" → 4.
// This identifies the minimum non-breakable unit that must fit on one line.
func longestToken(s string) int {
	longest := 0
	for _, word := range strings.Fields(s) {
		if len(word) > longest {
			longest = len(word)
		}
	}
	return longest
}

// calculateColumnWidths distributes width across columns proportional to
// the maximum content length in each column. Falls back to equal distribution
// when no content is available.
//
// The algorithm uses a two-pass approach:
//  1. Assign a proportional share to each column based on content length.
//  2. Enforce per-column minimum widths based on the longest non-breakable
//     token (word) to prevent mid-token line breaks (e.g., "$42M" → "$42"/"M").
//     Falls back to a global floor when content-aware minimums exceed the
//     available width.
//  3. Re-distribute any overshoot so that the sum always equals availableWidth.
//
// fontSize is the effective font size in hundredths of a point (after any
// column-count or row-count scaling). Pass 0 to use the default (1800).
func calculateColumnWidths(numCols int, availableWidth int64, headers []string, rows [][]types.TableCell, fontSize int) []int64 { //nolint:gocognit,gocyclo
	if numCols == 0 {
		return nil
	}
	if fontSize <= 0 {
		fontSize = defaultFontSize
	}

	// Measure max content length per column (header + all data cells).
	// Headers are rendered at 1.1× bold, so weight them ~20% heavier
	// to prevent awkward wrapping (e.g., "Category" → "Categ"/"ory").
	maxLen := make([]int, numCols)
	for i, h := range headers {
		weighted := len(h) * 6 / 5 // ~1.2× to account for bold+size
		if i < numCols && weighted > maxLen[i] {
			maxLen[i] = weighted
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < numCols && len(cell.Content) > maxLen[i] {
				maxLen[i] = len(cell.Content)
			}
		}
	}

	// Ensure no column has zero length (avoids div-by-zero)
	var totalLen int
	for i := range maxLen {
		if maxLen[i] == 0 {
			maxLen[i] = 1
		}
		totalLen += maxLen[i]
	}

	if totalLen == 0 {
		return equalColumnWidths(numCols, availableWidth)
	}

	// ---- Pass 1: pure proportional allocation ----
	widths := make([]int64, numCols)
	var allocated int64
	for i, l := range maxLen {
		w := availableWidth * int64(l) / int64(totalLen)
		widths[i] = w
		allocated += w
	}

	// Distribute rounding residual to the widest column so sum == availableWidth.
	residual := availableWidth - allocated
	if residual != 0 {
		widest := 0
		for i := 1; i < numCols; i++ {
			if widths[i] > widths[widest] {
				widest = i
			}
		}
		widths[widest] += residual
	}

	// ---- Pass 2: enforce minimum floor ----
	// Two floors are combined per column:
	//   (a) Global floor: prevents ugly lopsided layouts.
	//       ≤3 columns → 70% of equal share; >3 → 50% of equal share.
	//   (b) Content-aware floor: ensures the longest non-breakable token
	//       (word) in each column fits on one line without mid-token breaks.
	//       Estimated as tokenLen × avgCharWidth + 2×cellMargin.
	//
	// The effective minimum per column is max(global, content-aware).
	// If the total effective minimums exceed availableWidth, content-aware
	// floors are disabled and only the global floor is enforced.

	// Global floor
	var globalFloor int64
	if numCols <= 3 {
		globalFloor = availableWidth / int64(numCols) * 7 / 10
	} else {
		globalFloor = availableWidth / int64(numCols) / 2
	}
	if globalFloor < 1 {
		globalFloor = 1
	}

	// Content-aware floor per column: estimate EMU width for the longest
	// non-breakable token at the current font size.
	// Character width ≈ 60% of font em height (conservative for Calibri).
	// Em height in EMU = fontSize × 914400 / 7200 = fontSize × 127.
	emHeight := int64(fontSize) * 127
	charWidthEst := emHeight * 6 / 10

	contentFloor := make([]int64, numCols)
	for i := 0; i < numCols; i++ {
		maxToken := 0
		if i < len(headers) {
			if t := longestToken(headers[i]); t > maxToken {
				maxToken = t
			}
		}
		for _, row := range rows {
			if i < len(row) {
				if t := longestToken(row[i].Content); t > maxToken {
					maxToken = t
				}
			}
		}
		if maxToken > 0 {
			contentFloor[i] = int64(maxToken)*charWidthEst + 2*cellMargin
		}
	}

	// Compute effective per-column minimum: max(globalFloor, contentFloor).
	// If the total exceeds availableWidth, fall back to globalFloor only.
	effectiveMin := make([]int64, numCols)
	var totalEffective int64
	for i := range effectiveMin {
		m := globalFloor
		if contentFloor[i] > m {
			m = contentFloor[i]
		}
		effectiveMin[i] = m
		totalEffective += m
	}
	if totalEffective > availableWidth {
		for i := range effectiveMin {
			effectiveMin[i] = globalFloor
		}
	}

	// Identify columns below their effective minimum and clamp them up.
	var deficit int64
	for i := range widths {
		if widths[i] < effectiveMin[i] {
			deficit += effectiveMin[i] - widths[i]
			widths[i] = effectiveMin[i]
		}
	}

	// Reclaim the deficit from columns with surplus (above their effective
	// minimum), proportional to how much they exceed it.
	if deficit > 0 {
		var surplusTotal int64
		for i := range widths {
			if widths[i] > effectiveMin[i] {
				surplusTotal += widths[i] - effectiveMin[i]
			}
		}
		if surplusTotal > 0 {
			var reclaimed int64
			lastSurplus := -1
			for i := range widths {
				excess := widths[i] - effectiveMin[i]
				if excess > 0 {
					share := deficit * excess / surplusTotal
					widths[i] -= share
					reclaimed += share
					lastSurplus = i
				}
			}
			// Fix rounding residual in deficit redistribution
			if remainder := deficit - reclaimed; remainder != 0 && lastSurplus >= 0 {
				widths[lastSurplus] -= remainder
			}
		}
	}

	return widths
}

// equalColumnWidths distributes width evenly across columns (fallback).
func equalColumnWidths(numCols int, availableWidth int64) []int64 {
	baseWidth := availableWidth / int64(numCols)
	remainder := availableWidth % int64(numCols)
	widths := make([]int64, numCols)
	for i := range widths {
		widths[i] = baseWidth
		if int64(i) < remainder {
			widths[i]++
		}
	}
	return widths
}

// generateTableGrid generates the <a:tblGrid> element with column widths.
func generateTableGrid(colWidths []int64) string {
	var xml strings.Builder
	xml.WriteString(`<a:tblGrid>`)
	for _, w := range colWidths {
		fmt.Fprintf(&xml, `<a:gridCol w="%d"/>`, w)
	}
	xml.WriteString(`</a:tblGrid>`)
	return xml.String()
}

// generateHeaderRow generates the header row XML.
func generateHeaderRow(headers []string, height int64, config TableRenderConfig) string {
	var xml strings.Builder
	fmt.Fprintf(&xml, `<a:tr h="%d">`, height)

	for colIdx, header := range headers {
		xml.WriteString(generateHeaderCell(header, colIdx, config))
	}

	xml.WriteString(`</a:tr>`)
	return xml.String()
}

// generateHeaderRowWithMerges generates the header row XML when headers have colspan/rowspan.
func generateHeaderRowWithMerges(cells []types.TableCell, height int64, config TableRenderConfig) string {
	var xml strings.Builder
	fmt.Fprintf(&xml, `<a:tr h="%d">`, height)

	colIdx := 0
	for _, cell := range cells {
		if cell.IsMerged {
			if cell.ColSpan == 0 {
				xml.WriteString(`<a:tc hMerge="1"><a:txBody><a:bodyPr wrap="square" vert="horz"/><a:lstStyle/><a:p/></a:txBody><a:tcPr/></a:tc>`)
			} else if cell.RowSpan == 0 {
				overrides := &cellBorderOverrides{suppressTop: true}
				fmt.Fprintf(&xml, `<a:tc vMerge="1"><a:txBody><a:bodyPr wrap="square" vert="horz"/><a:lstStyle/><a:p/></a:txBody>%s</a:tc>`,
					generateCellProperties(config, true, 0, overrides))
			}
			colIdx++
			continue
		}

		var attrs string
		if cell.ColSpan > 1 {
			attrs += fmt.Sprintf(` gridSpan="%d"`, cell.ColSpan)
		}
		if cell.RowSpan > 1 {
			attrs += fmt.Sprintf(` rowSpan="%d"`, cell.RowSpan)
		}

		var overrides *cellBorderOverrides
		if cell.RowSpan > 1 {
			overrides = &cellBorderOverrides{suppressBottom: true}
		}
		fmt.Fprintf(&xml, `<a:tc%s>%s%s</a:tc>`,
			attrs,
			generateCellContent(cell.Content, true, config, colIdx),
			generateCellProperties(config, true, 0, overrides),
		)
		colIdx++
	}

	xml.WriteString(`</a:tr>`)
	return xml.String()
}

// generateHeaderCell generates a header cell with styling.
func generateHeaderCell(text string, colIdx int, config TableRenderConfig) string {
	return fmt.Sprintf(`<a:tc>%s%s</a:tc>`,
		generateCellContent(text, true, config, colIdx),
		generateCellProperties(config, true, 0, nil),
	)
}

// generateDataRow generates a data row XML, handling merges.
func generateDataRow(row []types.TableCell, rowIdx int, height int64, config TableRenderConfig) string {
	var xml strings.Builder
	fmt.Fprintf(&xml, `<a:tr h="%d">`, height)

	for colIdx, cell := range row {
		xml.WriteString(generateDataCell(cell, rowIdx, colIdx, config))
	}

	xml.WriteString(`</a:tr>`)
	return xml.String()
}

// generateSummaryRow generates the italic summary row for truncated tables.
func generateSummaryRow(row []types.TableCell, rowIdx int, height int64, config TableRenderConfig) string {
	var xml strings.Builder
	fmt.Fprintf(&xml, `<a:tr h="%d">`, height)

	for colIdx, cell := range row {
		fmt.Fprintf(&xml, `<a:tc>%s%s</a:tc>`,
			generateItalicCellContent(cell.Content, config, colIdx),
			generateCellProperties(config, false, rowIdx, nil),
		)
	}

	xml.WriteString(`</a:tr>`)
	return xml.String()
}

// generateItalicCellContent generates an italic text body for a summary cell.
func generateItalicCellContent(text string, config TableRenderConfig, colIdx int) string {
	fontSize := config.DefaultSize

	algn := "l"
	if colIdx >= 0 && colIdx < len(config.ColumnAlignments) {
		algn = alignmentToOOXML(config.ColumnAlignments[colIdx])
	}

	return fmt.Sprintf(
		`<a:txBody>`+
			`<a:bodyPr wrap="square" vert="horz" lIns="%d" rIns="%d" tIns="%d" bIns="%d" spcFirstLastPara="1">`+
			`<a:normAutofit/>`+
			`</a:bodyPr>`+
			`<a:lstStyle/>`+
			`<a:p><a:pPr algn="%s"/><a:r>`+
			`<a:rPr lang="en-US" sz="%d" b="0" i="1"/>`+
			`<a:t>%s</a:t>`+
			`</a:r></a:p></a:txBody>`,
		cellMargin, cellMargin, cellMargin/2, cellMargin/2,
		algn, fontSize, escapeXMLText(text),
	)
}

// generateDataCell generates a single data cell, handling colspan/rowspan.
func generateDataCell(cell types.TableCell, rowIdx int, colIdx int, config TableRenderConfig) string {
	// Handle merged cells (part of colspan or rowspan)
	if cell.IsMerged {
		if cell.ColSpan == 0 {
			// Horizontal merge continuation
			return `<a:tc hMerge="1"><a:txBody><a:bodyPr wrap="square" vert="horz"/><a:lstStyle/><a:p/></a:txBody><a:tcPr/></a:tc>`
		}
		if cell.RowSpan == 0 {
			// Vertical merge continuation — suppress top border for visual merge
			overrides := &cellBorderOverrides{suppressTop: true}
			return fmt.Sprintf(`<a:tc vMerge="1"><a:txBody><a:bodyPr wrap="square" vert="horz"/><a:lstStyle/><a:p/></a:txBody>%s</a:tc>`,
				generateCellProperties(config, false, rowIdx, overrides))
		}
	}

	// Build cell attributes for colspan/rowspan
	var attrs string
	if cell.ColSpan > 1 {
		attrs += fmt.Sprintf(` gridSpan="%d"`, cell.ColSpan)
	}
	if cell.RowSpan > 1 {
		attrs += fmt.Sprintf(` rowSpan="%d"`, cell.RowSpan)
	}

	// Suppress bottom border on rowspan origin cells — the bottom border
	// will come from the last continuation cell instead.
	var overrides *cellBorderOverrides
	if cell.RowSpan > 1 {
		overrides = &cellBorderOverrides{suppressBottom: true}
	}

	return fmt.Sprintf(`<a:tc%s>%s%s</a:tc>`,
		attrs,
		generateCellContent(cell.Content, false, config, colIdx),
		generateCellProperties(config, false, rowIdx, overrides),
	)
}

// alignmentToOOXML maps alignment names to OOXML paragraph alignment values.
func alignmentToOOXML(alignment string) string {
	switch alignment {
	case "center":
		return "ctr"
	case "right":
		return "r"
	default:
		return "l"
	}
}

// generateCellContent generates the text body for a cell.
//
// The <a:bodyPr> element is critical for wide tables: without explicit
// wrap="square" and vert="horz", PowerPoint may auto-rotate text vertically
// when column widths are narrow.  We also set lIns/rIns/tIns/bIns to keep
// text from touching cell borders, and spcFirstLastPara="1" so paragraph
// spacing applies uniformly.
func generateCellContent(text string, isHeader bool, config TableRenderConfig, colIdx int) string {
	fontSize := config.DefaultSize
	bold := "0"
	// When a table style is active and no explicit header_background is set,
	// the style's firstRow > tcTxStyle controls text formatting (bold, color).
	// Only force bold when we are NOT deferring to the table style.
	// When use_table_style is set, always defer to the style.
	styleControlsHeader := config.Style.UseTableStyle || (config.Style.StyleID != "" && config.Style.HeaderBackground == "")
	if isHeader {
		// Headers are slightly larger and bold
		fontSize = int(float64(fontSize) * 1.1)
		if !styleControlsHeader {
			bold = "1"
		}
	}

	// Determine paragraph alignment from column alignments
	algn := "l" // default left
	if colIdx >= 0 && colIdx < len(config.ColumnAlignments) {
		algn = alignmentToOOXML(config.ColumnAlignments[colIdx])
	}

	return fmt.Sprintf(
		`<a:txBody>`+
			`<a:bodyPr wrap="square" vert="horz" lIns="%d" rIns="%d" tIns="%d" bIns="%d" spcFirstLastPara="1">`+
			`<a:normAutofit/>`+
			`</a:bodyPr>`+
			`<a:lstStyle/>`+
			`<a:p><a:pPr algn="%s"/><a:r>`+
			`<a:rPr lang="en-US" sz="%d" b="%s"/>`+
			`<a:t>%s</a:t>`+
			`</a:r></a:p></a:txBody>`,
		cellMargin, cellMargin, cellMargin/2, cellMargin/2,
		algn, fontSize, bold, escapeXMLText(text),
	)
}

// cellBorderOverrides controls which borders to suppress for merged cells.
type cellBorderOverrides struct {
	suppressTop    bool
	suppressBottom bool
}

// generateCellProperties generates the <a:tcPr> element with borders and fill.
// The overrides parameter controls border suppression for merged cells (nil = no overrides).
func generateCellProperties(config TableRenderConfig, isHeader bool, rowIdx int, overrides *cellBorderOverrides) string {
	var xml strings.Builder
	// anchor="ctr" vertically centers text within the cell, which is especially
	// important for rowspan cells where the merged height is larger than the text.
	xml.WriteString(`<a:tcPr anchor="ctr">`)

	// When use_table_style is set, skip explicit borders and fills entirely
	// so the table style controls all cell appearance.
	if !config.Style.UseTableStyle {
		// Add borders based on style, with merge overrides
		xml.WriteString(generateBorderXMLWithOverrides(config.Style.Borders, isHeader, overrides))

		// Add fill for header cells
		if isHeader {
			hdrBg := config.Style.HeaderBackground
			if hdrBg != "none" && hdrBg != "" {
				fill := pptx.ResolveColorString(hdrBg)
				if !fill.IsZero() {
					var cb bytes.Buffer
					fill.WriteTo(&cb)
					xml.WriteString(cb.String())
				}
			}
		} else if (config.Style.Striped == nil || *config.Style.Striped) && rowIdx%2 == 1 {
			// Use accent1 at 15% saturation for a reliably visible alternating stripe.
			// The previous bg2 scheme color was visually identical to the slide
			// background on many templates, making the stripe invisible.
			xml.WriteString(`<a:solidFill><a:schemeClr val="accent1"><a:lumMod val="15000"/><a:lumOff val="85000"/></a:schemeClr></a:solidFill>`)
		}
	}

	xml.WriteString(`</a:tcPr>`)
	return xml.String()
}

// generateBorderXMLWithOverrides generates border elements based on style,
// with optional overrides for merged cells.
func generateBorderXMLWithOverrides(borderStyle string, isHeader bool, overrides *cellBorderOverrides) string {
	solidLine := func(side string) string {
		return fmt.Sprintf(`<a:ln%s w="%d" cap="flat" cmpd="sng">`+
			`<a:solidFill><a:schemeClr val="tx1"/></a:solidFill>`+
			`</a:ln%s>`, side, borderWidth, side)
	}
	noLine := func(side string) string {
		return fmt.Sprintf(`<a:ln%s w="0"><a:noFill/></a:ln%s>`, side, side)
	}

	borderFor := func(side string, wantSolid bool) string {
		// Check merge overrides
		if overrides != nil {
			if side == "T" && overrides.suppressTop {
				return noLine(side)
			}
			if side == "B" && overrides.suppressBottom {
				return noLine(side)
			}
		}
		if wantSolid {
			return solidLine(side)
		}
		return noLine(side)
	}

	switch borderStyle {
	case "none":
		return borderFor("L", false) + borderFor("R", false) + borderFor("T", false) + borderFor("B", false)
	case "horizontal":
		// Horizontal-only: top and bottom borders on all rows, no vertical borders anywhere.
		return borderFor("L", false) + borderFor("R", false) + borderFor("T", true) + borderFor("B", true)
	case "outer":
		fallthrough
	case "all":
		return borderFor("L", true) + borderFor("R", true) + borderFor("T", true) + borderFor("B", true)
	default:
		return borderFor("L", true) + borderFor("R", true) + borderFor("T", true) + borderFor("B", true)
	}
}

// generateTableLevelBorders generates the <a:tblBorders> element for <a:tblPr>.
// This explicitly defines the table-level border grid, preventing PowerPoint
// from rendering default grid lines that conflict with cell-level border settings.
func generateTableLevelBorders(borderStyle string) string {
	solidBorder := fmt.Sprintf(`<a:ln w="%d" cap="flat" cmpd="sng">`+
		`<a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:ln>`, borderWidth)
	noBorder := `<a:ln w="0"><a:noFill/></a:ln>`

	wrap := func(tag, content string) string {
		return fmt.Sprintf(`<%s>%s</%s>`, tag, content, tag)
	}

	var xml strings.Builder
	xml.WriteString(`<a:tblBorders>`)

	switch borderStyle {
	case "none":
		// Don't emit <a:tblBorders> at all — let the template table style
		// (wholeTbl > tcBdr) control borders. Emitting explicit noFill entries
		// would suppress the style's border definitions.
		return ""
	case "horizontal":
		xml.WriteString(wrap("a:top", solidBorder))
		xml.WriteString(wrap("a:bottom", solidBorder))
		xml.WriteString(wrap("a:left", noBorder))
		xml.WriteString(wrap("a:right", noBorder))
		xml.WriteString(wrap("a:insideH", solidBorder))
		xml.WriteString(wrap("a:insideV", noBorder))
	default: // "all", "outer", or unrecognized → full borders
		xml.WriteString(wrap("a:top", solidBorder))
		xml.WriteString(wrap("a:bottom", solidBorder))
		xml.WriteString(wrap("a:left", solidBorder))
		xml.WriteString(wrap("a:right", solidBorder))
		xml.WriteString(wrap("a:insideH", solidBorder))
		xml.WriteString(wrap("a:insideV", solidBorder))
	}

	xml.WriteString(`</a:tblBorders>`)
	return xml.String()
}

// mapToSchemeColor maps style color names to PowerPoint scheme colors.
// Accepts all valid OOXML scheme color names (accent1-6, dk1/dk2, lt1/lt2, etc.).
func mapToSchemeColor(colorName string) string {
	if colorName == "none" || colorName == "" {
		return "none"
	}
	if pptx.IsSchemeColor(colorName) {
		return colorName
	}
	return "accent1" // Default to accent1 for unrecognized names
}

// escapeXMLText escapes special characters for XML text content.
func escapeXMLText(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
