package patterns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func raggedCards(n int) []CardGridCell {
	cells := make([]CardGridCell, n)
	for i := range cells {
		cells[i] = CardGridCell{Header: fmt.Sprintf("Card %d", i+1), Body: "One line of body copy"}
	}
	return cells
}

// go-slide-creator-0w4va: columns and rows are optional and derived from the
// card count; a stated grid only has to hold the cards.
func TestCardGridShape(t *testing.T) {
	for _, tc := range []struct {
		cells, columns, rows int
		wantCols, wantRows   int
		ok                   bool
	}{
		{1, 0, 0, 1, 1, true}, {2, 0, 0, 2, 1, true}, {3, 0, 0, 3, 1, true}, {4, 0, 0, 2, 2, true},
		{5, 0, 0, 3, 2, true}, {6, 0, 0, 3, 2, true}, {7, 0, 0, 4, 2, true}, {8, 0, 0, 4, 2, true},
		{9, 0, 0, 3, 3, true}, {10, 0, 0, 5, 2, true}, {11, 0, 0, 4, 3, true}, {12, 0, 0, 4, 3, true},
		{13, 0, 0, 5, 3, true}, {25, 0, 0, 5, 5, true},
		{5, 3, 2, 3, 2, true},  // explicit grid larger than the count
		{5, 3, 3, 3, 2, true},  // an unused row is dropped
		{5, 2, 0, 2, 3, true},  // rows derived
		{5, 0, 2, 3, 2, true},  // columns derived
		{6, 3, 2, 3, 2, true},  // exact, as before
		{7, 3, 2, 0, 0, false}, // more cards than the grid holds
		{6, 1, 0, 0, 0, false}, // six rows
		{6, 0, 1, 0, 0, false}, // six columns
		{0, 0, 0, 0, 0, false},
		{3, 6, 1, 0, 0, false},
	} {
		v := &CardGridValues{Columns: tc.columns, Rows: tc.rows, Cells: raggedCards(tc.cells)}
		cols, rows, ok := v.Shape()
		if ok != tc.ok || cols != tc.wantCols || rows != tc.wantRows {
			t.Errorf("%d cells on columns=%d rows=%d: Shape = %d, %d, %t; want %d, %d, %t",
				tc.cells, tc.columns, tc.rows, cols, rows, ok, tc.wantCols, tc.wantRows, tc.ok)
		}
		if err := (&cardGrid{}).Validate(v, nil, nil); (err == nil) != tc.ok {
			t.Errorf("%d cells on columns=%d rows=%d: Validate = %v, want ok=%t", tc.cells, tc.columns, tc.rows, err, tc.ok)
		}
	}
}

// cardSpans returns the col_span of every card and spacer in a row; spacers
// are negative.
func cardSpans(row jsonschema.GridRowInput) []int {
	var out []int
	for _, c := range row.Cells {
		span := max(c.ColSpan, 1)
		if c.Shape == nil {
			span = -span
		}
		out = append(out, span)
	}
	return out
}

// 5 and 7 cards keep one card width in every row; the short row is centred by
// default and left-aligned on request. The placement is the same for the open
// default (three grid rows per row of cards) and the filled tile (one).
func TestCardGridRaggedLastRow(t *testing.T) {
	p := &cardGrid{}
	for _, tc := range []struct {
		cells   int
		lastRow string
		columns string
		want    [][]int
	}{
		{5, "", "6", [][]int{{2, 2, 2}, {-1, 2, 2, -1}}},
		{7, "", "8", [][]int{{2, 2, 2, 2}, {-1, 2, 2, 2, -1}}},
		{11, "", "8", [][]int{{2, 2, 2, 2}, {2, 2, 2, 2}, {-1, 2, 2, 2, -1}}},
		{5, "left", "3", [][]int{{1, 1, 1}, {1, 1, -1}}},
		{7, "left", "4", [][]int{{1, 1, 1, 1}, {1, 1, 1, -1}}},
		{6, "", "3", [][]int{{1, 1, 1}, {1, 1, 1}}}, // a full grid is unchanged
	} {
		// The filled tile: one grid row per row of cards.
		t.Run(fmt.Sprintf("filled_%d_%s", tc.cells, tc.lastRow), func(t *testing.T) {
			vals := &CardGridValues{Cells: raggedCards(tc.cells)}
			ovr := &CardGridOverrides{LastRow: tc.lastRow, Style: "filled"}
			if err := p.Validate(vals, ovr, nil); err != nil {
				t.Fatalf("validate: %v", err)
			}
			grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if string(grid.Columns) != tc.columns {
				t.Errorf("columns = %s, want %s", grid.Columns, tc.columns)
			}
			if len(grid.Rows) != len(tc.want) {
				t.Fatalf("rows = %d, want %d", len(grid.Rows), len(tc.want))
			}
			cards := 0
			for r, row := range grid.Rows {
				got := cardSpans(row)
				if fmt.Sprint(got) != fmt.Sprint(tc.want[r]) {
					t.Errorf("row %d spans = %v, want %v", r, got, tc.want[r])
				}
				if row.MaxHeight <= 0 {
					t.Errorf("row %d is not content-sized", r)
				}
				for _, s := range got {
					if s > 0 {
						cards++
					}
				}
			}
			if cards != tc.cells {
				t.Errorf("%d cards placed, want %d", cards, tc.cells)
			}
		})

		// The open default: a heading row, a rule row and a body row per row of
		// cards, each placed like the filled row, and one full-width spacer row
		// between two rows of cards — 4N-1 grid rows.
		t.Run(fmt.Sprintf("open_%d_%s", tc.cells, tc.lastRow), func(t *testing.T) {
			vals := &CardGridValues{Cells: raggedCards(tc.cells)}
			ovr := &CardGridOverrides{LastRow: tc.lastRow}
			if err := p.Validate(vals, ovr, nil); err != nil {
				t.Fatalf("validate: %v", err)
			}
			grid, err := p.Expand(ExpandContext{}, vals, ovr, nil)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if string(grid.Columns) != tc.columns {
				t.Errorf("columns = %s, want %s", grid.Columns, tc.columns)
			}
			if want := 4*len(tc.want) - 1; len(grid.Rows) != want {
				t.Fatalf("rows = %d, want %d (heading, rule, body per row of cards + a spacer between)", len(grid.Rows), want)
			}
			var gridCols int
			if _, err := fmt.Sscan(tc.columns, &gridCols); err != nil {
				t.Fatal(err)
			}
			if grid.ColGap != cardGridOpenColGapPt || grid.RowGap != cardGridOpenRowGapPt || grid.Gap != 0 {
				t.Errorf("gaps col=%v row=%v gap=%v, want a %vpt gutter, %vpt between heading, rule and body, no uniform gap",
					grid.ColGap, grid.RowGap, grid.Gap, cardGridOpenColGapPt, cardGridOpenRowGapPt)
			}
			headings, rules, bodies := 0, 0, 0
			for r, row := range grid.Rows {
				got := cardSpans(row)
				if row.MaxHeight <= 0 {
					t.Errorf("row %d is not content-sized", r)
				}
				band, role := r/4, r%4
				if role == 3 {
					// The whitespace between two rows of cards: one empty
					// cell across every grid column, at a fixed height.
					if fmt.Sprint(got) != fmt.Sprint([]int{-gridCols}) {
						t.Errorf("row %d (spacer) spans = %v, want one empty cell across %d columns", r, got, gridCols)
					}
					if row.MinHeight != row.MaxHeight {
						t.Errorf("row %d (spacer) height %v..%v, want fixed", r, row.MinHeight, row.MaxHeight)
					}
					continue
				}
				if fmt.Sprint(got) != fmt.Sprint(tc.want[band]) {
					t.Errorf("row %d (cards row %d, part %d) spans = %v, want %v", r, band, role, got, tc.want[band])
				}
				for _, c := range row.Cells {
					if c.Shape == nil {
						continue
					}
					if c.AccentBar != nil {
						t.Errorf("row %d: an open card carries no accent bar", r)
					}
					switch role {
					case 0:
						headings++
						if string(c.Shape.Fill) != `"none"` || !strings.Contains(string(c.Shape.Text), `"vertical_align":"b"`) {
							t.Errorf("row %d: heading cell fill %s text %s, want unfilled and bottom-anchored", r, c.Shape.Fill, c.Shape.Text)
						}
					case 1:
						rules++
						if got := string(c.Shape.Fill); got != string(neutralFillJSON(NeutralTint60)) {
							t.Errorf("row %d: rule fill = %s, want neutral 60%%", r, got)
						}
						if len(c.Shape.Text) != 0 {
							t.Errorf("row %d: a rule carries no text, got %s", r, c.Shape.Text)
						}
					case 2:
						bodies++
						if string(c.Shape.Fill) != `"none"` || !strings.Contains(string(c.Shape.Text), `"vertical_align":"t"`) {
							t.Errorf("row %d: body cell fill %s text %s, want unfilled and top-anchored", r, c.Shape.Fill, c.Shape.Text)
						}
					}
				}
				switch role {
				case 0:
					if row.MinHeight != row.MaxHeight {
						t.Errorf("row %d (headings) height %v..%v, want fixed", r, row.MinHeight, row.MaxHeight)
					}
				case 1:
					if row.MinHeight != cardGridOpenRulePt || row.MaxHeight != cardGridOpenRulePt {
						t.Errorf("row %d (rules) height %v..%v, want %vpt", r, row.MinHeight, row.MaxHeight, cardGridOpenRulePt)
					}
				case 2:
					if row.MinHeight != 0 {
						t.Errorf("row %d (bodies) min height = %v, want only a max height", r, row.MinHeight)
					}
				}
			}
			if headings != tc.cells || rules != tc.cells || bodies != tc.cells {
				t.Errorf("%d headings, %d rules, %d bodies placed, want %d of each", headings, rules, bodies, tc.cells)
			}
		})
	}
}

// A short row of open cards shares the baseline without filler lines: the
// headings are their own bottom-anchored row, sized to the row's tallest
// heading, so the one-line heading stands on the same rule as the wrapped one.
func TestCardGridRaggedOpenRowSharesHeadingBaseline(t *testing.T) {
	cells := raggedCards(5)
	cells[3].Header = "A header long enough to wrap onto a second line in a third-width card"
	grid, err := (&cardGrid{}).Expand(ExpandContext{}, &CardGridValues{Cells: cells}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 7 {
		t.Fatalf("rows = %d, want 7", len(grid.Rows))
	}
	first, last := grid.Rows[0], grid.Rows[4]
	if last.MinHeight != last.MaxHeight || last.MinHeight <= first.MinHeight {
		t.Errorf("the heading row holding a wrapped heading is %v..%vpt, want fixed and taller than the one-line row's %vpt",
			last.MinHeight, last.MaxHeight, first.MinHeight)
	}
	headings := 0
	for _, c := range last.Cells {
		if c.Shape == nil {
			continue
		}
		headings++
		text := string(c.Shape.Text)
		if n := strings.Count(text, `"size"`); n != 1 {
			t.Errorf("an open heading is one paragraph with no filler lines, got %d in %s", n, text)
		}
		if !strings.Contains(text, `"vertical_align":"b"`) {
			t.Errorf("an open heading is bottom-anchored on its rule, got %s", text)
		}
	}
	if headings != 2 {
		t.Errorf("%d headings in the short row, want 2", headings)
	}
}

// A short row of filled tiles still shares one body baseline: its headers are
// padded to the row's tallest header at the full row's card width.
func TestCardGridRaggedRowSharesBodyBaseline(t *testing.T) {
	cells := raggedCards(5)
	cells[3].Header = "A header long enough to wrap onto a second line in a third-width card"
	grid, err := (&cardGrid{}).Expand(ExpandContext{}, &CardGridValues{Cells: cells}, &CardGridOverrides{Style: "filled"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := grid.Rows[1]
	var paras []int
	for _, c := range last.Cells {
		if c.Shape == nil {
			continue
		}
		paras = append(paras, strings.Count(string(c.Shape.Text), `"size"`))
	}
	if len(paras) != 2 || paras[1] <= paras[0] {
		t.Errorf("the one-line header beside a wrapped one must gain filler lines, got paragraph counts %v", paras)
	}
}

func TestCardGridLastRowOverrideValidated(t *testing.T) {
	err := (&cardGrid{}).Validate(&CardGridValues{Cells: raggedCards(5)}, &CardGridOverrides{LastRow: "right"}, nil)
	if err == nil || !strings.Contains(err.Error(), "overrides.last_row") {
		t.Fatalf("want an overrides.last_row enum error, got %v", err)
	}
}

// The budget of a ragged grid is measured on the grid it resolves to.
func TestCardGridBodyBudgetsRagged(t *testing.T) {
	five := CardGridBodyBudgets(ExpandContext{}, &CardGridValues{Cells: raggedCards(5)}, nil)
	six := CardGridBodyBudgets(ExpandContext{}, &CardGridValues{Columns: 3, Rows: 2, Cells: raggedCards(6)}, nil)
	if len(five) != 5 || five[0] <= 0 || five[0] != six[0] {
		t.Errorf("5 auto-arranged cards budget %v, want the 3x2 budget %v", five, six)
	}
}
