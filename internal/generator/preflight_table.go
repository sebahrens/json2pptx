// Package generator: preflight predictors for table render-time fit findings.
//
// These detectors mirror the scaling and truncation logic in GenerateTableXML
// without rendering. Given the table content (headers + rows) and the
// allocated bounds, they predict whether GenerateTableXML would emit
// table_font_scaled, table_rows_truncated, or column_width_deficit findings.
package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TablePreflightInput describes a table whose render-time fit behaviour
// should be predicted at validate/preview time.
type TablePreflightInput struct {
	// Path is the JSON pointer to the table, e.g. "/slides/0/content/1".
	Path string
	// Headers is the list of column header strings.
	Headers []string
	// Rows is the data rows (each row is a slice of cells).
	Rows [][]types.TableCell
	// Bounds is the allocated bounding box for the table in EMU. When
	// Bounds.Width is 0, only column-count scaling is predicted (row-count
	// truncation requires a height to compute capacity from).
	Bounds types.BoundingBox
	// DefaultSize is the configured font size in hundredths of a point
	// (e.g. 1800 = 18pt). When 0, the engine default (1800) is used.
	DefaultSize int
}

// DetectTablePreflight predicts whether GenerateTableXML would emit
// table_font_scaled / table_rows_truncated / column_width_deficit findings
// for the given table. Returns all applicable findings (zero, one, two, or
// three).
//
// The prediction logic mirrors GenerateTableXML in internal/generator/table.go:
//   - Column-count font scaling (numCols > 4 → table_font_scaled with reason="columns")
//   - Row-count font scaling (numRows > maxVisibleRows → table_font_scaled with reason="rows")
//   - Row truncation when rows still exceed capacity at the minimum floor
//     (table_rows_truncated)
//   - Content-aware column width deficit at the (possibly scaled) font size
//     (column_width_deficit)
func DetectTablePreflight(input TablePreflightInput) []patterns.FitFinding {
	if len(input.Headers) == 0 {
		return nil
	}

	numCols := len(input.Headers)
	numRows := len(input.Rows) + 1 // +1 for header

	fontSize := input.DefaultSize
	if fontSize <= 0 {
		fontSize = defaultFontSize
	}
	originalFontSize := fontSize

	var findings []patterns.FitFinding

	// Column-count scaling: numCols > 4 → scale down by 4/numCols, floor at MinTableFontSize.
	if numCols > 4 {
		scale := 4.0 / float64(numCols)
		scaled := int(float64(fontSize) * scale)
		if scaled < minFontSizeForTable {
			scaled = minFontSizeForTable
		}
		fontSize = scaled
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: input.Path,
				Code: patterns.ErrCodeTableFontScaled,
				Message: fmt.Sprintf(
					"predicted: table font will scale from %.0fpt to %.0fpt for %d columns",
					float64(originalFontSize)/100, float64(fontSize)/100, numCols,
				),
				Fix: &patterns.FixSuggestion{
					Kind:   "review",
					Params: map[string]any{"reason": "columns", "columns": numCols},
				},
			},
			Action: "review",
		})
	}

	// Row-count scaling: maxVisibleRows derived from bounds height.
	if input.Bounds.Height > 0 {
		fontRatio := float64(fontSize) / float64(defaultFontSize)
		scaledRowHeight := int64(float64(defaultRowHeight) * fontRatio)
		if scaledRowHeight < 1 {
			scaledRowHeight = 1
		}
		maxVisibleRows := int(input.Bounds.Height / scaledRowHeight)
		if numRows > maxVisibleRows && maxVisibleRows > 0 {
			preFontSize := fontSize
			rowScale := float64(maxVisibleRows) / float64(numRows)
			scaled := int(float64(fontSize) * rowScale)
			if scaled < minFontSizeForTable {
				scaled = minFontSizeForTable
			}
			fontSize = scaled
			findings = append(findings, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: input.Path,
					Code: patterns.ErrCodeTableFontScaled,
					Message: fmt.Sprintf(
						"predicted: table font will scale from %.0fpt to %.0fpt for %d rows (capacity %d)",
						float64(preFontSize)/100, float64(fontSize)/100, numRows, maxVisibleRows,
					),
					Fix: &patterns.FixSuggestion{
						Kind:   "review",
						Params: map[string]any{"reason": "rows", "rows": numRows},
					},
				},
				Action: "review",
			})
		}

		// Truncation: at the post-scale font size, see how many rows fit
		// when the row height is clamped to defaultRowHeight.
		fontRatio = float64(fontSize) / float64(defaultFontSize)
		scaledRowHeight = int64(float64(defaultRowHeight) * fontRatio)
		if scaledRowHeight < defaultRowHeight {
			scaledRowHeight = defaultRowHeight
		}
		maxVisibleRows = int(input.Bounds.Height / scaledRowHeight)
		headerRowCount := 1
		dataRowCapacity := maxVisibleRows - headerRowCount
		if dataRowCapacity < 1 {
			dataRowCapacity = 1
		}
		if len(input.Rows) > dataRowCapacity {
			overflow := len(input.Rows) - dataRowCapacity + 1 // +1 to make room for summary row
			tableID := strings.Join(input.Headers, ", ")
			findings = append(findings, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: input.Path,
					Code: patterns.ErrCodeTableRowsTruncated,
					Message: fmt.Sprintf(
						"predicted: table rows will be truncated — %d of %d rows hidden (headers: %s)",
						overflow, len(input.Rows), tableID,
					),
					Fix: &patterns.FixSuggestion{
						Kind: "split_at_row",
						Params: map[string]any{
							"visible_rows": len(input.Rows) - overflow,
							"hidden_rows":  overflow,
						},
					},
				},
				Action: "review",
			})
		}
	}

	// Column-width deficit: replicate calculateColumnWidthsWithDiag's deficit
	// signal without producing widths. The deficit fires when the total of
	// per-column content-aware minimums exceeds the available width.
	if input.Bounds.Width > 0 {
		if hasColumnWidthDeficit(numCols, input.Bounds.Width, input.Headers, input.Rows, fontSize) {
			findings = append(findings, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: input.Path,
					Code: patterns.ErrCodeColumnWidthDeficit,
					Message: fmt.Sprintf(
						"predicted: column widths will fall back to global floor (content-aware minimums exceed available width for %d columns)",
						numCols,
					),
					Fix: &patterns.FixSuggestion{
						Kind:   "review",
						Params: map[string]any{"columns": numCols},
					},
				},
				Action: "review",
			})
		}
	}

	return findings
}

// hasColumnWidthDeficit mirrors the content-aware floor check in
// calculateColumnWidthsWithDiag. It returns true when the sum of per-column
// content-aware minimum widths exceeds the available width, in which case
// the renderer falls back to the global floor.
func hasColumnWidthDeficit(numCols int, availableWidth int64, headers []string, rows [][]types.TableCell, fontSize int) bool {
	if numCols == 0 || availableWidth <= 0 {
		return false
	}
	if fontSize <= 0 {
		fontSize = defaultFontSize
	}

	// Character width estimate (Calibri): em height in EMU * 0.6.
	emHeight := int64(fontSize) * 127
	charWidthEst := emHeight * 6 / 10

	// Per-column longest non-breakable token estimate.
	longestToken := make([]int, numCols)
	for i, h := range headers {
		if i >= numCols {
			break
		}
		if l := longestWordLen(h); l > longestToken[i] {
			longestToken[i] = l
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= numCols {
				break
			}
			if l := longestWordLen(cell.Content); l > longestToken[i] {
				longestToken[i] = l
			}
		}
	}

	const cellMarginEMU = int64(45720) // matches generator/table.go cellMargin
	var totalMin int64
	for _, tok := range longestToken {
		min := int64(tok)*charWidthEst + 2*cellMarginEMU
		totalMin += min
	}

	return totalMin > availableWidth
}

// longestWordLen returns the length of the longest whitespace-delimited token.
func longestWordLen(s string) int {
	longest := 0
	for _, field := range strings.Fields(s) {
		if len(field) > longest {
			longest = len(field)
		}
	}
	return longest
}
