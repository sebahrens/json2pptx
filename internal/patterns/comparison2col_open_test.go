package patterns

import (
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func openComparison(rows int, headers bool) *Comparison2colValues {
	v := &Comparison2colValues{}
	if headers {
		v.Headers, v.HeaderLeft, v.HeaderRight = [2]string{"Pros", "Cons"}, "Pros", "Cons"
	}
	for i := 0; i < rows; i++ {
		v.Rows = append(v.Rows, Comparison2colRow{Left: "Live in eight weeks", Right: "Licence fees add EUR 1.2M"})
	}
	return v
}

// textRows returns the rows that carry text; the rest are rules.
func textRows(grid *jsonschema.ShapeGridInput) (text, rules []jsonschema.GridRowInput) {
	for _, row := range grid.Rows {
		isText := false
		for _, c := range row.Cells {
			if c != nil && c.Shape != nil && len(c.Shape.Text) > 0 {
				isText = true
			}
		}
		if isText {
			text = append(text, row)
		} else {
			rules = append(rules, row)
		}
	}
	return text, rules
}

// go-slide-creator-zawui: the default comparison has no per-item fill; rows
// are separated by per-column rules and each row's two cells share a grid row.
func TestComparison2colOpenDefault(t *testing.T) {
	p := &comparison2col{}
	ctx := testThemeCtx()
	for _, tc := range []struct {
		rows    int
		headers bool
	}{{1, false}, {3, false}, {2, true}, {5, true}, {10, true}} {
		grid, err := p.Expand(ctx, openComparison(tc.rows, tc.headers), nil, nil)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		text, rules := textRows(grid)
		want := tc.rows
		if tc.headers {
			want++
		}
		if len(text) != want || len(rules) != want-1 || len(grid.Rows) != 2*want-1 {
			t.Fatalf("%+v: %d text rows and %d rules, want %d and %d", tc, len(text), len(rules), want, want-1)
		}
		for i, row := range text {
			if len(row.Cells) != 2 {
				t.Fatalf("%+v: row %d has %d cells: left and right must share a row", tc, i, len(row.Cells))
			}
			for _, c := range row.Cells {
				if string(c.Shape.Fill) != `"none"` || string(c.Shape.Line) != `"none"` {
					t.Errorf("%+v: row %d cell is filled or outlined: %s / %s", tc, i, c.Shape.Fill, c.Shape.Line)
				}
			}
		}
		for i, row := range rules {
			wantH := comparisonRulePt
			if i == 0 && tc.headers {
				wantH = comparisonHeaderRulePt
			}
			if row.MinHeight != wantH || row.MaxHeight != wantH || len(row.Cells) != 2 {
				t.Errorf("%+v: rule %d = %.2f–%.2fpt with %d cells, want %.2fpt per column", tc, i, row.MinHeight, row.MaxHeight, len(row.Cells), wantH)
			}
			fill := string(row.Cells[0].Shape.Fill)
			if i == 0 && tc.headers {
				if fill != `"accent1"` {
					t.Errorf("%+v: header rule fill = %s, want the accent", tc, fill)
				}
			} else if !strings.Contains(fill, `"dk1"`) {
				t.Errorf("%+v: body rule fill = %s, want a neutral hairline", tc, fill)
			}
		}
		if grid.RowGap != comparisonRuleRowGapPt {
			t.Errorf("%+v: row_gap = %v, rows and rules must touch", tc, grid.RowGap)
		}
	}
}

// The open rows take exactly the height the tiles took: each text row grows by
// its share of the row gaps, so the measured character budgets carry over.
func TestComparison2colOpenKeepsTileHeight(t *testing.T) {
	p := &comparison2col{}
	ctx := testThemeCtx()
	for _, n := range []int{2, 4} {
		v := openComparison(n, true)
		open, err := p.Expand(ctx, v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		tiles, err := p.Expand(ctx, v, &Comparison2colOverrides{Style: "tiles"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		height := func(g *jsonschema.ShapeGridInput, gap float64) float64 {
			h := float64(len(g.Rows)-1) * gap
			for _, r := range g.Rows {
				h += r.MaxHeight
			}
			return h
		}
		tileGap := tiles.RowGap
		if tileGap == 0 {
			tileGap = tiles.Gap
		}
		openH, tileH := height(open, open.RowGap), height(tiles, tileGap)
		rulesH := comparisonRulesPt(n+1, true)
		if math.Abs(openH-rulesH-tileH) > 1.5*float64(n+1) {
			t.Errorf("%d rows: open block %.1fpt (rules %.1fpt) vs tiles %.1fpt", n, openH, rulesH, tileH)
		}
		_, areaH := sizingAreaPt(ctx)
		if openH > areaH+0.5 {
			t.Errorf("%d rows: open block %.1fpt exceeds the %.1fpt area", n, openH, areaH)
		}
	}
}

// One highlighted row, or one highlighted column, is the only filled area.
func TestComparison2colOpenHighlight(t *testing.T) {
	p := &comparison2col{}
	ctx := testThemeCtx()

	v := openComparison(4, true)
	v.Rows[2].Highlight = true
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, _ := p.Expand(ctx, v, nil, nil)
	text, _ := textRows(grid)
	for i, row := range text {
		for _, c := range row.Cells {
			if filled := string(c.Shape.Fill) != `"none"`; filled != (i == 3) {
				t.Errorf("row highlight: text row %d filled = %t", i, filled)
			}
		}
	}

	col := openComparison(3, true)
	ovr := &Comparison2colOverrides{HighlightColumn: "right"}
	if err := p.Validate(col, ovr, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, _ = p.Expand(ctx, col, ovr, nil)
	text, _ = textRows(grid)
	for i, row := range text {
		if string(row.Cells[0].Shape.Fill) != `"none"` {
			t.Errorf("column highlight: left cell of row %d is filled", i)
		}
		right := string(row.Cells[1].Shape.Fill)
		if i == 0 && !strings.Contains(right, "accent1") || i > 0 && !strings.Contains(right, "lumMod") {
			t.Errorf("column highlight: right cell of row %d fill = %s", i, right)
		}
	}

	v.Rows[0].Highlight = true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one row") {
		t.Errorf("two highlighted rows must be refused, got %v", err)
	}
	one := openComparison(2, true)
	one.Rows[0].Highlight = true
	if err := p.Validate(one, ovr, nil); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("row and column highlight together must be refused, got %v", err)
	}
	for _, bad := range []*Comparison2colOverrides{{Style: "boxes"}, {HighlightColumn: "middle"}} {
		if err := p.Validate(openComparison(2, true), bad, nil); err == nil {
			t.Errorf("%+v must be refused", bad)
		}
	}
}

// overrides.connectors keeps its centre gutter and badge in the open style,
// without the tile stripe and tint.
func TestComparison2colOpenConnectors(t *testing.T) {
	p := &comparison2col{}
	grid, err := p.Expand(testThemeCtx(), openComparison(3, true), &Comparison2colOverrides{Connectors: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(grid.Columns); got != "[45, 10, 45]" {
		t.Errorf("columns = %s, want [45, 10, 45]", got)
	}
	text, rules := textRows(grid)
	for i, row := range text[1:] {
		if len(row.Cells) != 3 {
			t.Fatalf("body row %d has %d cells, want 3", i, len(row.Cells))
		}
		mid := row.Cells[1]
		if mid.Shape == nil || mid.Shape.Geometry != "ellipse" || mid.Shape.Icon == nil {
			t.Errorf("body row %d gutter cell is not the connector badge: %+v", i, mid)
		}
		if row.Cells[0].AccentBar != nil || string(row.Cells[0].Shape.Fill) != `"none"` || string(row.Cells[2].Shape.Fill) != `"none"` {
			t.Errorf("body row %d: open connector rows carry no stripe and no fill", i)
		}
	}
	for i, row := range rules {
		if len(row.Cells) != 3 || row.Cells[1].Shape != nil {
			t.Errorf("rule %d must leave the gutter open", i)
		}
	}
}
