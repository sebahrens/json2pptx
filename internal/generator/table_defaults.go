package generator

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Default table styling (go-slide-creator-weaq).
//
// Without an explicit style, tables referenced the built-in "Medium Style 2 -
// Accent 1" GUID and deferred the header look to it — but most templates do
// not ship that style in ppt/tableStyles.xml, so headers rendered as plain,
// non-bold text, numbers were left-aligned and a "Total" row looked like any
// other row. The engine now renders a consulting-grade default explicitly,
// using only theme scheme colors so it stays template-driven:
//
//   - header row: accent1 fill, bold, lt1 text
//   - numeric columns (detected from the data) right-aligned, header included
//   - rows labelled Total / Sum / Grand total: bold with a top rule
//   - row heights are content-driven minimums rather than stretched to fill
//     the placeholder

// defaultHeaderFill is the scheme color used for the default header row.
const defaultHeaderFill = "accent1"

// applyDefaultTableStyling fills unset table style fields with the engine
// defaults. Explicit author choices always win: an explicit
// header_background (including "none"), use_table_style, a non-default
// style_id, column_types, or a per-column alignment.
func applyDefaultTableStyling(table *types.TableSpec, config *TableRenderConfig) {
	st := &config.Style
	if !st.UseTableStyle && st.HeaderBackground == "" &&
		(st.StyleID == "" || st.StyleID == types.DefaultTableStyleID) {
		st.HeaderBackground = defaultHeaderFill
	}
	if len(st.ColumnTypes) == 0 {
		st.ColumnTypes = inferColumnTypes(table, config.ColumnAlignments)
	}
}

// inferColumnTypes marks columns whose non-empty data cells are all numeric
// (numbers, currency, percentages, multiples, deltas) as "number" so they are
// right-aligned. Columns with an explicit alignment keep it ("text"). Returns
// nil when no column is numeric.
func inferColumnTypes(table *types.TableSpec, alignments []string) []string {
	numCols := len(table.Headers)
	if numCols == 0 || len(table.Rows) == 0 {
		return nil
	}
	colTypes := make([]string, numCols)
	found := false
	for col := 0; col < numCols; col++ {
		colTypes[col] = "text"
		if col < len(alignments) && alignments[col] != "" {
			continue
		}
		numeric, seen := 0, 0
		for _, row := range table.Rows {
			if col >= len(row) || row[col].IsMerged {
				continue
			}
			v := strings.TrimSpace(row[col].Content)
			if v == "" || v == "-" || v == "—" || strings.EqualFold(v, "n/a") {
				continue
			}
			seen++
			if isNumericCell(v) {
				numeric++
			}
		}
		if seen > 0 && numeric == seen {
			colTypes[col] = "number"
			found = true
		}
	}
	if !found {
		return nil
	}
	return colTypes
}

// numericCellRegexp matches common numeric cell renderings: optional sign or
// parentheses, currency symbol, digits with separators/decimals, and an
// optional unit suffix (%, x, pp, bps, K/M/B/bn/mn, pts).
var numericCellRegexp = regexp.MustCompile(
	`^[+\-−±(]?\s*[$€£¥]?\s*[+\-−]?\d[\d,.\s]*(?:[.,]\d+)?\s*(?:%|x|×|pp|bps|pts?|k|m|b|bn|mn|mm|t)?\)?$`)

// isNumericCell reports whether a cell's text reads as a number.
func isNumericCell(s string) bool {
	return numericCellRegexp.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// totalRowLabelRegexp matches first-column labels of total/summary rows.
var totalRowLabelRegexp = regexp.MustCompile(`^(?:grand\s+)?(?:total|totals|sum|subtotal|sub-total)\b`)

// isTotalRow reports whether a data row is a total/summary row by its label.
func isTotalRow(row []types.TableCell) bool {
	if len(row) == 0 {
		return false
	}
	return totalRowLabelRegexp.MatchString(strings.ToLower(strings.TrimSpace(row[0].Content)))
}

// headerTextColorXML returns the run fill for header text on a filled header:
// lt1 on dark scheme fills (accents, dk*, tx*), nothing otherwise.
func headerTextColorXML(config TableRenderConfig) string {
	if config.Style.UseTableStyle {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(config.Style.HeaderBackground)) {
	case "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "dk1", "dk2", "tx1", "tx2":
		return `<a:solidFill><a:schemeClr val="lt1"/></a:solidFill>`
	}
	return ""
}

// contentRowHeight is the minimum row height for content-driven tables: a
// single line at the table font plus cell insets, never below the legacy
// defaultRowHeight floor used by the truncation math. Renderers grow rows
// that need more lines.
func contentRowHeight(fontSizeHPt int) int64 {
	h := int64(float64(fontSizeHPt)/100.0*1.2*12700) + cellMargin*2
	if h < defaultRowHeight {
		h = defaultRowHeight
	}
	return h
}
