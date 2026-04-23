package jsonschema

import (
	"testing"
)

func TestCellExtraLogicalRows(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"single line", "hello", 0},
		{"two lines", "hello\nworld", 1},
		{"three lines", "a\nb\nc", 2},
		{"two comma items (not a list)", "a, b", 0},
		{"three comma items", "a, b, c", 2},
		{"five comma items", "a, b, c, d, e", 4},
		{"newlines beat commas", "a\nb\nc\nd", 3},
		{"commas beat newlines", "a, b, c, d, e", 4},
		{"mixed newlines and commas", "a, b\nc", 1}, // 2 lines, 2 non-empty comma parts (< 3) → lines win: extra = 1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CellExtraLogicalRows(tt.content)
			if got != tt.want {
				t.Errorf("CellExtraLogicalRows(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func makeRows(n, cols int) [][]TableCellInput {
	rows := make([][]TableCellInput, n)
	for i := range rows {
		row := make([]TableCellInput, cols)
		for j := range row {
			row[j] = TableCellInput{Content: "x", ColSpan: 1, RowSpan: 1}
		}
		rows[i] = row
	}
	return rows
}

func makeHeaders(n int) []string {
	h := make([]string, n)
	for i := range h {
		h[i] = "H"
	}
	return h
}

func TestLogicalRowCount(t *testing.T) {
	tests := []struct {
		name string
		table *TableInput
		want  int
	}{
		{
			"nil table",
			nil,
			0,
		},
		{
			"empty headers",
			&TableInput{Headers: []string{}, Rows: makeRows(3, 3)},
			0,
		},
		{
			"simple 6 data rows + header",
			&TableInput{Headers: makeHeaders(6), Rows: makeRows(6, 6)},
			7,
		},
		{
			"multiline cells add to count",
			func() *TableInput {
				t := &TableInput{Headers: makeHeaders(6), Rows: makeRows(6, 6)}
				t.Rows[0][0] = TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}
				t.Rows[1][0] = TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}
				t.Rows[2][0] = TableCellInput{Content: "a\nb", ColSpan: 1, RowSpan: 1}
				return t
			}(),
			10, // 7 + 3 extra from multiline cells
		},
		{
			"comma list counting",
			func() *TableInput {
				t := &TableInput{Headers: makeHeaders(3), Rows: makeRows(3, 3)}
				t.Rows[0][0] = TableCellInput{Content: "a, b, c, d", ColSpan: 1, RowSpan: 1}
				return t
			}(),
			7, // 4 base + 3 extra from comma list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.table.LogicalRowCount()
			if got != tt.want {
				t.Errorf("LogicalRowCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
