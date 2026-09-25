package jsonschema

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// grid renders a spec's rows as (content, colSpan, rowSpan, isMerged) tuples
// for readable assertions.
type cellShape struct {
	content  string
	colSpan  int
	rowSpan  int
	isMerged bool
}

func shapeOf(rows [][]types.TableCell) [][]cellShape {
	out := make([][]cellShape, len(rows))
	for i, row := range rows {
		out[i] = make([]cellShape, len(row))
		for j, c := range row {
			out[i][j] = cellShape{c.Content, c.ColSpan, c.RowSpan, c.IsMerged}
		}
	}
	return out
}

// go-slide-creator-chvf: a col_span cell emitted one <a:tc gridSpan="N"> and
// nothing for the columns it covered, so a row carried fewer cells than the
// table had <a:gridCol>. ECMA-376 requires an explicit hMerge/vMerge
// placeholder per covered column; without them LibreOffice drops the merged
// cell's text and paints an empty block.
func TestExpandCellSpans(t *testing.T) {
	tests := []struct {
		name      string
		gridWidth int
		rows      [][]types.TableCell
		want      [][]cellShape
	}{
		{
			name:      "horizontal span gets hMerge continuations",
			gridWidth: 5,
			rows: [][]types.TableCell{
				{{Content: "FY24 Actuals", ColSpan: 2, RowSpan: 1}, {Content: "FY25 Plan", ColSpan: 2, RowSpan: 1}, {Content: "Var", ColSpan: 1, RowSpan: 1}},
			},
			want: [][]cellShape{{
				{"FY24 Actuals", 2, 1, false},
				{"", 0, 1, true},
				{"FY25 Plan", 2, 1, false},
				{"", 0, 1, true},
				{"Var", 1, 1, false},
			}},
		},
		{
			name:      "vertical span gets a vMerge continuation in the next row",
			gridWidth: 3,
			rows: [][]types.TableCell{
				{{Content: "North America", ColSpan: 1, RowSpan: 2}, {Content: "Enterprise", ColSpan: 1, RowSpan: 1}, {Content: "$5.2M", ColSpan: 1, RowSpan: 1}},
				{{Content: "SMB", ColSpan: 1, RowSpan: 1}, {Content: "$2.1M", ColSpan: 1, RowSpan: 1}},
			},
			want: [][]cellShape{
				{{"North America", 1, 2, false}, {"Enterprise", 1, 1, false}, {"$5.2M", 1, 1, false}},
				{{"", 1, 0, true}, {"SMB", 1, 1, false}, {"$2.1M", 1, 1, false}},
			},
		},
		{
			name:      "an author-supplied empty filler is consumed, not doubled",
			gridWidth: 3,
			rows: [][]types.TableCell{
				{{Content: "Europe", ColSpan: 1, RowSpan: 2}, {Content: "Enterprise", ColSpan: 1, RowSpan: 1}, {Content: "$3.8M", ColSpan: 1, RowSpan: 1}},
				{{Content: "", ColSpan: 1, RowSpan: 1}, {Content: "SMB", ColSpan: 1, RowSpan: 1}, {Content: "$1.4M", ColSpan: 1, RowSpan: 1}},
			},
			want: [][]cellShape{
				{{"Europe", 1, 2, false}, {"Enterprise", 1, 1, false}, {"$3.8M", 1, 1, false}},
				{{"", 1, 0, true}, {"SMB", 1, 1, false}, {"$1.4M", 1, 1, false}},
			},
		},
		{
			name:      "plain rows are untouched",
			gridWidth: 3,
			rows: [][]types.TableCell{
				{{Content: "a", ColSpan: 1, RowSpan: 1}, {Content: "b", ColSpan: 1, RowSpan: 1}, {Content: "c", ColSpan: 1, RowSpan: 1}},
			},
			want: [][]cellShape{
				{{"a", 1, 1, false}, {"b", 1, 1, false}, {"c", 1, 1, false}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shapeOf(expandCellSpans(tt.rows, 0))
			if len(got) != len(tt.want) {
				t.Fatalf("row count = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if len(got[i]) != tt.gridWidth {
					t.Errorf("row %d has %d cells, want exactly the grid width %d: %v",
						i, len(got[i]), tt.gridWidth, got[i])
				}
				if len(got[i]) != len(tt.want[i]) {
					t.Errorf("row %d = %v, want %v", i, got[i], tt.want[i])
					continue
				}
				for j := range tt.want[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("row %d cell %d = %+v, want %+v", i, j, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

// Re-expanding an already-expanded spec must be a no-op.
func TestExpandCellSpans_Idempotent(t *testing.T) {
	rows := [][]types.TableCell{
		{{Content: "Total", ColSpan: 2, RowSpan: 1}, {Content: "9", ColSpan: 1, RowSpan: 1}},
	}
	once := expandCellSpans(rows, 0)
	twice := expandCellSpans(once, 0)

	if len(once) != len(twice) {
		t.Fatalf("row count changed on re-expansion: %d then %d", len(once), len(twice))
	}
	for i := range once {
		if len(once[i]) != len(twice[i]) {
			t.Errorf("row %d cell count changed on re-expansion: %d then %d", i, len(once[i]), len(twice[i]))
		}
	}
}

// ToTableSpec must wire the expansion in, so merged headers reach the generator
// with their continuation cells.
func TestToTableSpec_ExpandsSpans(t *testing.T) {
	in := &TableInput{
		Headers: []string{"Metric", "A", "B", "C", "D"},
		Rows: [][]TableCellInput{
			{{Content: "FY24 Actuals", ColSpan: 2, RowSpan: 1}, {Content: "FY25 Plan", ColSpan: 2, RowSpan: 1}, {Content: "Var", ColSpan: 1, RowSpan: 1}},
		},
	}
	spec := in.ToTableSpec()
	if len(spec.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(spec.Rows))
	}
	if len(spec.Rows[0]) != len(in.Headers) {
		t.Errorf("row has %d cells against %d headers — continuations missing: %v",
			len(spec.Rows[0]), len(in.Headers), shapeOf(spec.Rows))
	}
	merged := 0
	for _, c := range spec.Rows[0] {
		if c.IsMerged {
			merged++
		}
	}
	if merged != 2 {
		t.Errorf("expected 2 merge continuations for two col_span=2 cells, got %d", merged)
	}
}

// go-slide-creator-s1uvj.26: a row shorter than the header count rendered
// fewer <a:tc> than <a:gridCol>. ToTableSpec pads it to the grid width, giving
// a column that a row_span still covers its continuation, not a plain cell.
func TestToTableSpec_PadsShortRows(t *testing.T) {
	in := &TableInput{
		Headers: []string{"Region", "Revenue", "Growth"},
		Rows: [][]TableCellInput{
			{{Content: "NA", ColSpan: 1, RowSpan: 1}, {Content: "$12.4M", ColSpan: 1, RowSpan: 1}, {Content: "+3%", ColSpan: 1, RowSpan: 2}},
			{{Content: "$8.7M", ColSpan: 1, RowSpan: 1}},
			{{Content: "EU", ColSpan: 1, RowSpan: 1}, {Content: "$9.1M", ColSpan: 1, RowSpan: 1}},
		},
	}
	got := shapeOf(in.ToTableSpec().Rows)
	want := [][]cellShape{
		{{"NA", 1, 1, false}, {"$12.4M", 1, 1, false}, {"+3%", 1, 2, false}},
		{{"$8.7M", 1, 1, false}, {"", 1, 1, false}, {"", 1, 0, true}},
		{{"EU", 1, 1, false}, {"$9.1M", 1, 1, false}, {"", 1, 1, false}},
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("row %d = %v, want %v", i, got[i], want[i])
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("row %d cell %d = %+v, want %+v", i, j, got[i][j], want[i][j])
			}
		}
	}
}
