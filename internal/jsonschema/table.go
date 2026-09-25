package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Table density rule (TDR) thresholds.
const (
	TDRMaxRows   = 7
	TDRMaxCols   = 6
	TDRMinFontPt = 9
)

// TableInput represents a table with headers, rows, and optional styling.
type TableInput struct {
	Headers          []string           `json:"headers"`
	Rows             [][]TableCellInput `json:"rows"`
	Style            *TableStyleInput   `json:"style,omitempty"`
	ColumnAlignments []string           `json:"column_alignments,omitempty"`
	// Alt is the description a screen reader announces for the table. One
	// sentence saying what it shows; without it the engine derives one from
	// the table's shape and column names (go-slide-creator-6e8h).
	Alt string `json:"alt,omitempty"`
}

// ToTableSpec converts TableInput to types.TableSpec.
func (t *TableInput) ToTableSpec() *types.TableSpec {
	if t == nil {
		return nil
	}
	spec := &types.TableSpec{
		Headers:          t.Headers,
		ColumnAlignments: t.ColumnAlignments,
		Alt:              t.Alt,
	}
	for _, row := range t.Rows {
		cells := make([]types.TableCell, len(row))
		for j, cell := range row {
			cells[j] = types.TableCell{
				Content: cell.Content,
				ColSpan: cell.ColSpan,
				RowSpan: cell.RowSpan,
			}
			if cell.Conditional != nil {
				threshold, _ := cell.Conditional.ThresholdValue()
				cells[j].Conditional = &types.ConditionalFormat{
					Rule:      cell.Conditional.Rule,
					Threshold: threshold,
					Fill:      cell.Conditional.Fill,
				}
			}
		}
		spec.Rows = append(spec.Rows, cells)
	}
	spec.Rows = expandCellSpans(spec.Rows, len(t.Headers))
	if t.Style != nil {
		spec.Style = types.TableStyle{
			Borders:         t.Style.Borders,
			Striped:         t.Style.Striped, // nil means unset (default banding on)
			UseTableStyle:   t.Style.UseTableStyle,
			StyleID:         t.Style.StyleID,
			HighlightColumn: t.Style.HighlightColumn,
			TotalsRow:       t.Style.TotalsRow,
			ColumnTypes:     t.Style.ColumnTypes,
		}
		if t.Style.HeaderBackground != nil {
			spec.Style.HeaderBackground = *t.Style.HeaderBackground
		}
		// Default StyleID when not explicitly set
		if spec.Style.StyleID == "" {
			spec.Style.StyleID = types.DefaultTableStyleID
		}
	} else {
		spec.Style = types.DefaultTableStyle
	}
	return spec
}

// TableCellInput supports both string shorthand and full object form.
type TableCellInput struct {
	Content     string                  `json:"content"`
	ColSpan     int                     `json:"col_span,omitempty"`
	RowSpan     int                     `json:"row_span,omitempty"`
	Conditional *ConditionalFormatInput `json:"conditional,omitempty"`
}

// UnmarshalJSON supports string shorthand: "cell text" or {"content":"cell text","col_span":2}.
func (c *TableCellInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Content = s
		c.ColSpan = 1
		c.RowSpan = 1
		return nil
	}
	type alias TableCellInput
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("TableCellInput must be string or {content, col_span, row_span}: %w", err)
	}
	*c = TableCellInput(a)
	if c.ColSpan == 0 {
		c.ColSpan = 1
	}
	if c.RowSpan == 0 {
		c.RowSpan = 1
	}
	return nil
}

// TableStyleInput maps to types.TableStyle.
type TableStyleInput struct {
	HeaderBackground *string  `json:"header_background,omitempty"`
	Borders          string   `json:"borders,omitempty"`
	Striped          *bool    `json:"striped,omitempty"`
	UseTableStyle    bool     `json:"use_table_style,omitempty"`
	StyleID          string   `json:"style_id,omitempty"`
	HighlightColumn  int      `json:"highlight_column,omitempty"`
	TotalsRow        bool     `json:"totals_row,omitempty"`
	ColumnTypes      []string `json:"column_types,omitempty"`
}

// ConditionalFormatInput represents a conditional formatting rule for a cell.
//
// Threshold is deliberately untyped. It used to be a float64, so the natural
// RAG rule {"rule":"equals","threshold":"On track"} aborted the WHOLE deck at
// parse time with a Go type error that named the wrong struct and pointed at
// no path (go-slide-creator-6hlu). Whatever JSON the author wrote is kept here
// and interpreted per rule.
type ConditionalFormatInput struct {
	Rule      string          `json:"rule"`
	Threshold json.RawMessage `json:"threshold,omitempty"`
	Fill      string          `json:"fill,omitempty"`
}

// ThresholdValue decodes the authored threshold into the operand the renderer
// compares against: a float64, a string, or a []float64 for "between". It
// returns nil when there is none, and ok=false when the JSON is a shape no rule
// can use (an object, say), which validation reports at the threshold's path.
func (c *ConditionalFormatInput) ThresholdValue() (value any, ok bool) {
	if c == nil || len(c.Threshold) == 0 {
		return nil, true
	}
	var num float64
	if err := json.Unmarshal(c.Threshold, &num); err == nil {
		return num, true
	}
	var text string
	if err := json.Unmarshal(c.Threshold, &text); err == nil {
		return text, true
	}
	var pair []float64
	if err := json.Unmarshal(c.Threshold, &pair); err == nil && len(pair) == 2 {
		return pair, true
	}
	var b bool
	if err := json.Unmarshal(c.Threshold, &b); err == nil {
		return strconv.FormatBool(b), true
	}
	return nil, false
}

// LogicalRowCount returns the effective row count for this table, accounting
// for multiline cells. The count includes the header row.
func (t *TableInput) LogicalRowCount() int {
	if t == nil || len(t.Headers) == 0 {
		return 0
	}
	logicalRows := len(t.Rows) + 1 // +1 for header row
	for _, row := range t.Rows {
		for _, cell := range row {
			logicalRows += CellExtraLogicalRows(cell.Content)
		}
	}
	return logicalRows
}

// CellExtraLogicalRows returns the number of extra logical rows a cell's
// content contributes beyond 1. A cell with N newlines contributes N extra rows.
// A cell with a comma-separated list of ≥3 items contributes (items-1) extra rows.
// The higher of the two counts is used.
//
// This matches the multiline cell counting rule in the generate-deck skill:
// effective logical rows = max(line_count, comma_items).
func CellExtraLogicalRows(content string) int {
	if content == "" {
		return 0
	}

	// Count newline-separated lines.
	lineCount := strings.Count(content, "\n") + 1

	// Count comma-separated items (only if ≥3).
	commaItems := 0
	parts := strings.Split(content, ",")
	if len(parts) >= 3 {
		// Verify these look like list items (non-empty after trim).
		count := 0
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				count++
			}
		}
		if count >= 3 {
			commaItems = count
		}
	}

	// Effective logical row count for this cell.
	effective := int(math.Max(float64(lineCount), float64(commaItems)))

	// Extra rows beyond the 1 row the cell already occupies.
	if effective <= 1 {
		return 0
	}
	return effective - 1
}

// takeCoveredContinuation returns the continuation cell for a grid column a
// vertical span from an earlier row still covers, consuming one row of that
// coverage. The interior column of a rectangle merge gets a horizontal
// continuation (horizontal wins).
func takeCoveredContinuation(covered, hSpanOf map[int]int, col int) (types.TableCell, bool) {
	n := covered[col]
	if n <= 0 {
		return types.TableCell{}, false
	}
	covered[col] = n - 1
	cont := types.TableCell{IsMerged: true, RowSpan: 0, ColSpan: 1}
	if hSpanOf[col] == 0 {
		cont.ColSpan = 0
	}
	return cont, true
}

// expandCellSpans materialises the continuation cells a merge requires.
//
// ECMA-376 expresses a merge as a gridSpan/rowSpan on the origin cell PLUS an
// explicit placeholder <a:tc hMerge="1"/> or <a:tc vMerge="1"/> for every grid
// column the merge covers, so each <a:tr> still carries exactly one <a:tc> per
// <a:gridCol>. The generator already knew how to emit those placeholders (the
// cell.IsMerged branches in generateDataCell / generateHeaderRowWithMerges) but
// nothing ever set IsMerged from JSON input, so the branch was dead: a col_span
// row emitted 3 cells against 5 grid columns and LibreOffice dropped the text
// of the second merged cell, painting it as an empty block
// (go-slide-creator-chvf).
//
// The pass walks each row against a grid-occupancy model. Positions covered by
// a span reuse an author-supplied empty filler cell when there is one — decks
// in the wild pad rowspans by hand — and get a synthetic continuation
// otherwise. Rows that already carry IsMerged cells are left untouched, so
// re-converting an expanded spec is a no-op.
//
// When numCols > 0 a row whose cells run out before the grid width is padded
// to numCols: columns a vertical span still covers get their continuation,
// the rest get an empty cell. A short row otherwise rendered fewer <a:tc> than
// <a:gridCol> and failed OOXML_INVALID_TABLE (go-slide-creator-s1uvj.26).
// Rows wider than numCols are left as they are; types.TableSpec.CheckRowWidths
// reports them.
func expandCellSpans(rows [][]types.TableCell, numCols int) [][]types.TableCell {
	if len(rows) == 0 {
		return rows
	}
	for _, row := range rows {
		for _, c := range row {
			if c.IsMerged {
				return rows // already expanded
			}
		}
	}

	// covered[col] counts how many further rows a vertical span still covers,
	// and hSpanOf[col] carries that span's horizontal width so a rectangle
	// merge gets a placeholder at every position it covers.
	covered := map[int]int{}
	hSpanOf := map[int]int{}

	out := make([][]types.TableCell, len(rows))
	for i, row := range rows {
		var expanded []types.TableCell
		col := 0
		src := 0

		emit := func(c types.TableCell) {
			expanded = append(expanded, c)
			col++
		}

		for {
			// Fill any grid columns a span from an earlier row covers.
			if cont, ok := takeCoveredContinuation(covered, hSpanOf, col); ok {
				// Consume an author-supplied empty filler at this position
				// rather than inserting a second cell for it.
				if src < len(row) && row[src].Content == "" && row[src].ColSpan <= 1 && row[src].RowSpan <= 1 {
					src++
				}
				emit(cont)
				continue
			}
			if src >= len(row) {
				if col < numCols {
					emit(types.TableCell{ColSpan: 1, RowSpan: 1})
					continue
				}
				break
			}

			cell := row[src]
			src++
			emit(cell)

			// Reserve the rows this cell's vertical span covers, across every
			// column its horizontal span covers.
			if cell.RowSpan > 1 {
				covered[col-1] = cell.RowSpan - 1
				hSpanOf[col-1] = cell.ColSpan
			}
			for k := 1; k < cell.ColSpan; k++ {
				emit(types.TableCell{IsMerged: true, ColSpan: 0, RowSpan: 1})
				if cell.RowSpan > 1 {
					covered[col-1] = cell.RowSpan - 1
					hSpanOf[col-1] = 0
				}
			}
		}

		out[i] = expanded
	}
	return out
}
