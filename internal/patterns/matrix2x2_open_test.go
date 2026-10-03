package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func openMatrixValues() *Matrix2x2Values {
	return &Matrix2x2Values{
		XAxisLabel: "Market Share", YAxisLabel: "Market Growth",
		TopLeft:     Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
		TopRight:    Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
		BottomLeft:  Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
		BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
	}
}

// go-slide-creator-jnkiq: the default matrix is two crossing axis lines with
// open quadrants; titles and low / high ends sit in strips along the axes.
func TestMatrix2x2OpenDefault(t *testing.T) {
	p := &matrix2x2{}
	grid, err := p.Expand(testThemeCtx(), openMatrixValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 4 {
		t.Fatalf("columns = %s, want [y strip, left, axis, right]", grid.Columns)
	}
	if cols[0] != matrix2x2YStripPct || cols[1] != cols[3] || cols[2] <= 0 || cols[2] > 0.5 {
		t.Errorf("columns = %v: want equal quadrant columns beside a hairline axis column", cols)
	}
	if len(grid.Rows) != 4 {
		t.Fatalf("rows = %d, want top, axis, bottom, x strip", len(grid.Rows))
	}
	if grid.ColGap != matrix2x2OpenGapPt || grid.RowGap != matrix2x2OpenGapPt {
		t.Errorf("gaps = %v / %v: the axis lines must touch the quadrants to cross", grid.ColGap, grid.RowGap)
	}

	top, axis, bottom, strip := grid.Rows[0], grid.Rows[1], grid.Rows[2], grid.Rows[3]
	yStrip, vAxis := top.Cells[0], top.Cells[2]
	if yStrip.Grid == nil || yStrip.RowSpan != 3 {
		t.Errorf("y strip must run the height of the vertical axis: %+v", yStrip)
	}
	if vAxis.Shape == nil || vAxis.RowSpan != 3 || len(vAxis.Shape.Text) != 0 || !strings.Contains(string(vAxis.Shape.Fill), `"dk1"`) {
		t.Errorf("vertical axis must be one neutral line through both quadrant rows: %+v", vAxis)
	}
	if axis.MinHeight != matrix2x2AxisLinePt || axis.MaxHeight != matrix2x2AxisLinePt || len(axis.Cells) != 2 {
		t.Errorf("horizontal axis row = %+v", axis)
	}
	for _, c := range axis.Cells {
		if c.Shape == nil || string(c.Shape.Fill) != string(vAxis.Shape.Fill) {
			t.Errorf("horizontal axis segment fill = %v", c.Shape)
		}
	}
	for i, c := range []*struct {
		fill string
		text string
	}{
		{string(top.Cells[1].Shape.Fill), string(top.Cells[1].Shape.Text)},
		{string(top.Cells[3].Shape.Fill), string(top.Cells[3].Shape.Text)},
		{string(bottom.Cells[0].Shape.Fill), string(bottom.Cells[0].Shape.Text)},
		{string(bottom.Cells[1].Shape.Fill), string(bottom.Cells[1].Shape.Text)},
	} {
		if c.fill != `"none"` || c.text == "" {
			t.Errorf("quadrant %d: fill %s, want an open quadrant with text", i, c.fill)
		}
	}
	if strip.Cells[1].Grid == nil || strip.Cells[1].ColSpan != 3 || strip.MaxHeight <= 0 || strip.MinHeight != strip.MaxHeight {
		t.Errorf("x strip must run the width of the horizontal axis at its written height: %+v", strip)
	}

	// Ends and titles: HIGH above the rotated title above LOW; LOW, title, HIGH.
	yText, _ := json.Marshal(yStrip.Grid)
	xText, _ := json.Marshal(strip.Cells[1].Grid)
	if y := string(yText); !(strings.Index(y, "High") < strings.Index(y, "Market Growth") && strings.Index(y, "Market Growth") < strings.Index(y, "Low")) || !strings.Contains(y, "vert270") {
		t.Errorf("y strip order / rotation wrong: %s", y)
	}
	if x := string(xText); !(strings.Index(x, "Low") < strings.Index(x, "Market Share") && strings.Index(x, "Market Share") < strings.Index(x, "High")) {
		t.Errorf("x strip order wrong: %s", x)
	}
	if strings.Contains(string(yText)+string(xText), "Arrow") {
		t.Error("the open matrix draws axis lines, not arrow shapes")
	}
}

// One highlighted quadrant is the only filled area.
func TestMatrix2x2OpenHighlight(t *testing.T) {
	p := &matrix2x2{}
	v := openMatrixValues()
	v.BottomLeft.Highlight = true
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	quads := []string{
		string(grid.Rows[0].Cells[1].Shape.Fill), string(grid.Rows[0].Cells[3].Shape.Fill),
		string(grid.Rows[2].Cells[0].Shape.Fill), string(grid.Rows[2].Cells[1].Shape.Fill),
	}
	for i, fill := range quads {
		if filled := fill != `"none"`; filled != (i == 2) {
			t.Errorf("quadrant %d fill = %s", i, fill)
		}
	}
	if !strings.Contains(quads[2], "accent1") {
		t.Errorf("highlight fill = %s, want an accent tint", quads[2])
	}
	v.TopRight.Highlight = true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one quadrant") {
		t.Errorf("two highlighted quadrants must be refused, got %v", err)
	}
	if err := p.Validate(openMatrixValues(), &Matrix2x2Overrides{Style: "boxes"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("want an overrides.style enum error, got %v", err)
	}
}

// The open quadrants have more room than the tiles: a wider column (no grid
// gaps) and a content-sized x strip instead of a 16% axis row.
func TestMatrix2x2OpenHasMoreRoom(t *testing.T) {
	ctx := testThemeCtx()
	v := openMatrixValues()
	open := layoutMatrix2x2(ctx, v, &Matrix2x2Overrides{})
	tiles := layoutMatrix2x2(ctx, v, &Matrix2x2Overrides{Style: "tiles"})
	if open.availPt <= tiles.availPt {
		t.Errorf("open quadrant height %.0fpt, tiles %.0fpt: removing the containers must not cost room", open.availPt, tiles.availPt)
	}
	if open.needs[0] > tiles.needs[0] || open.needs[1] > tiles.needs[1] {
		t.Errorf("open needs %v exceed tile needs %v", open.needs, tiles.needs)
	}
}
