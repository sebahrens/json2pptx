package slides

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// go-slide-creator-e4h1. A plain table — the financials, the segment split, the
// pricing tiers — had no kind, so the most ordinary business slide there is
// needed the raw escape hatch. capability-gaps probed `kind: table` and got
// "unknown slide kind".

func compiledTable(t *testing.T, body map[string]any) *jsonschema.TableInput {
	t.Helper()
	slide, _, err := CompileTable(Input{Title: "Segments", Body: body})
	if err != nil {
		t.Fatalf("CompileTable: %v", err)
	}
	for _, c := range slide.Content {
		if c.Type == "table" {
			return c.TableValue
		}
	}
	return nil
}

func segmentTable(overlay map[string]any) map[string]any {
	body := map[string]any{
		"headers": []any{"Segment", "FY25", "FY26"},
		"rows": []any{
			[]any{"Enterprise", "$28.4M", "$41.2M"},
			[]any{"SMB", "$9.6M", "$8.9M"},
		},
	}
	for k, v := range overlay {
		body[k] = v
	}
	return body
}

func TestTableCompilesToNativeTable(t *testing.T) {
	table := compiledTable(t, segmentTable(nil))
	if table == nil {
		t.Fatal("no table content block")
	}
	if len(table.Headers) != 3 || table.Headers[2] != "FY26" {
		t.Errorf("headers = %v", table.Headers)
	}
	if len(table.Rows) != 2 || table.Rows[0][1].Content != "$28.4M" {
		t.Errorf("rows = %+v", table.Rows)
	}
}

// A row addressed by header label is the shape an author reaches for when the
// columns are named; it must land in the same column order as the headers.
func TestTableAcceptsObjectRows(t *testing.T) {
	table := compiledTable(t, map[string]any{
		"headers": []any{"Segment", "FY25", "FY26"},
		"rows": []any{
			map[string]any{"FY26": "$41.2M", "Segment": "Enterprise", "FY25": "$28.4M"},
			map[string]any{"Segment": "SMB", "FY26": "$8.9M"},
		},
	})
	if table == nil {
		t.Fatal("no table content block")
	}
	want := [][]string{{"Enterprise", "$28.4M", "$41.2M"}, {"SMB", "", "$8.9M"}}
	for i, row := range want {
		for j, cell := range row {
			if got := table.Rows[i][j].Content; got != cell {
				t.Errorf("rows[%d][%d] = %q, want %q", i, j, got, cell)
			}
		}
	}
}

// The renderer's alignment vocabulary is "left"/"center"/"right"; emitting the
// shape-grid codes instead left every numeric column silently left-aligned.
func TestTableAlignmentUsesTheRendererVocabulary(t *testing.T) {
	table := compiledTable(t, segmentTable(map[string]any{
		"column_alignments": []any{"left", "right", "r"},
	}))
	want := []string{"left", "right", "right"}
	if len(table.ColumnAlignments) != len(want) {
		t.Fatalf("column_alignments = %v, want %v", table.ColumnAlignments, want)
	}
	for i := range want {
		if table.ColumnAlignments[i] != want[i] {
			t.Errorf("column_alignments[%d] = %q, want %q", i, table.ColumnAlignments[i], want[i])
		}
	}

	// A short list pads rather than leaving columns unaligned at random.
	table = compiledTable(t, segmentTable(map[string]any{"column_alignments": []any{"right"}}))
	if len(table.ColumnAlignments) != 3 || table.ColumnAlignments[2] != "left" {
		t.Errorf("column_alignments = %v, want three entries padded with left", table.ColumnAlignments)
	}
}

func TestTableEmphasis(t *testing.T) {
	// highlight_column takes a header name; the renderer's index is 1-based.
	table := compiledTable(t, segmentTable(map[string]any{"highlight_column": "FY26"}))
	if table.Style == nil || table.Style.HighlightColumn != 3 {
		t.Errorf("highlight_column = %+v, want the 1-based index of FY26 (3)", table.Style)
	}
	// An index works too.
	table = compiledTable(t, segmentTable(map[string]any{"highlight_column": float64(1)}))
	if table.Style == nil || table.Style.HighlightColumn != 2 {
		t.Errorf("highlight_column = %+v, want 2", table.Style)
	}
	// An unknown name highlights nothing rather than guessing a column.
	table = compiledTable(t, segmentTable(map[string]any{"highlight_column": "FY27"}))
	if table.Style != nil && table.Style.HighlightColumn != 0 {
		t.Errorf("highlight_column = %+v for an unknown header, want none", table.Style)
	}

	table = compiledTable(t, segmentTable(map[string]any{"totals_row": true}))
	if table.Style == nil || !table.Style.TotalsRow {
		t.Errorf("totals_row = %+v, want true", table.Style)
	}

	// No emphasis fields leaves the style unset so the template's own table
	// style is used verbatim.
	if table := compiledTable(t, segmentTable(nil)); table.Style != nil {
		t.Errorf("style = %+v, want nil when nothing was asked for", table.Style)
	}
}

// Cells keep the author's literal: a number must not pick up %v's exponent form.
func TestTableCellLiterals(t *testing.T) {
	table := compiledTable(t, map[string]any{
		"headers": []any{"Metric", "Value"},
		"rows": []any{
			[]any{"Revenue", 1000000.0},
			[]any{"Margin", 0.0000001},
			[]any{"Active", true},
			[]any{"Missing", nil},
		},
	})
	want := []string{"1000000", "0.0000001", "true", ""}
	for i, w := range want {
		if got := table.Rows[i][1].Content; got != w {
			t.Errorf("rows[%d][1] = %q, want %q", i, got, w)
		}
	}
}

// Without a header row there is no table to render, but the rows still say
// something, so they degrade to bullets rather than vanishing.
func TestTableDegradesWithoutHeaders(t *testing.T) {
	slide, _, err := CompileTable(Input{Title: "Numbers", Body: map[string]any{
		"rows": []any{[]any{"Enterprise", "$41.2M"}, []any{"SMB", "$8.9M"}},
	}})
	if err != nil {
		t.Fatalf("CompileTable: %v", err)
	}
	for _, c := range slide.Content {
		if c.Type == "table" {
			t.Fatal("a headerless payload must not compile to a table")
		}
	}
	bullets := *slide.Content[len(slide.Content)-1].BulletsValue
	if len(bullets) != 2 || !strings.Contains(bullets[0], "$41.2M") {
		t.Errorf("bullets = %v, want the row content preserved", bullets)
	}
}

func TestTableFeasibleMatchesCompile(t *testing.T) {
	bodies := []map[string]any{
		segmentTable(nil),
		{"headers": []any{"a"}, "rows": []any{[]any{"1"}}},
		{"rows": []any{[]any{"1"}}},
		{"headers": []any{"a"}},
		{},
	}
	for _, body := range bodies {
		slide, _, err := CompileTable(Input{Title: "T", Body: body})
		if err != nil {
			t.Fatalf("compile %v: %v", body, err)
		}
		compiled := false
		for _, c := range slide.Content {
			if c.Type == "table" {
				compiled = true
			}
		}
		if got := TableFeasible(body); got != compiled {
			t.Errorf("TableFeasible(%v) = %v, compile emitted a table = %v", body, got, compiled)
		}
	}
}
