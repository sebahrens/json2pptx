package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// tonalMatrixQuadrants returns the four quadrant cells of a tonal matrix
// (top-left, top-right, bottom-left, bottom-right).
func tonalMatrixQuadrants(t *testing.T, grid *jsonschema.ShapeGridInput) [4]*jsonschema.GridCellInput {
	t.Helper()
	var found []*jsonschema.GridCellInput
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if c != nil && c.ColSpan == 2 && c.Shape != nil {
				found = append(found, c)
			}
		}
	}
	if len(found) != 4 {
		t.Fatalf("quadrant cells = %d, want 4", len(found))
	}
	return [4]*jsonschema.GridCellInput(found)
}

// go-slide-creator-ckpye: the default matrix is four filled quadrant fields
// in the accent's tint split by a white gutter, each with its name bold at
// the top left, and two dark axis bars that carry low end, title and high end
// in reading order and end in a point at the high end.
func TestMatrix2x2TonalDefault(t *testing.T) {
	p := &matrix2x2{}
	ctx := testThemeCtx()
	grid, err := p.Expand(ctx, openMatrixValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 7 {
		t.Fatalf("columns = %s, want [y bar, gap, x low, left, gutter, right, x high]", grid.Columns)
	}
	if cols[1] != cols[4] || cols[2] != cols[6] || cols[3] != cols[5] || cols[4] <= 0 {
		t.Errorf("columns = %v: want two equal quadrants either side of a gutter", cols)
	}
	if len(grid.Rows) != 8 {
		t.Fatalf("rows = %d, want cap, y high, top, gutter, bottom, y low, gap, x bar", len(grid.Rows))
	}
	if grid.Rows[0].MaxHeight+grid.Rows[1].MaxHeight != grid.Rows[5].MaxHeight {
		t.Errorf("fixed parts of the two quadrant rows differ (%.0f + %.0f vs %.0f): the quadrants would not be equal", grid.Rows[0].MaxHeight, grid.Rows[1].MaxHeight, grid.Rows[5].MaxHeight)
	}

	quads := tonalMatrixQuadrants(t, grid)
	field := string(quads[0].Shape.Fill)
	for i, q := range quads {
		if string(q.Shape.Fill) != field || !strings.Contains(field, "accent1") || field == `"accent1"` {
			t.Errorf("quadrant %d fill = %s, want one accent tint for all four", i, q.Shape.Fill)
		}
		var text patternTextObj
		if err := json.Unmarshal(q.Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		if text.Align != "l" || text.VerticalAlign != "t" || !text.Paragraphs[0].Bold || text.Paragraphs[0].Size < sizeHeaderPt {
			t.Errorf("quadrant %d text = %+v, want a bold name of 16pt or more at the top left", i, text)
		}
	}

	// The axis bars: dark neutral, one fill, no accent.
	all, _ := json.Marshal(grid)
	if strings.Contains(string(all), "Arrow") || strings.Contains(string(all), `"rotation"`) {
		t.Error("the axis bars are rects, a homePlate and a triangle; no arrow preset, no rotated shape")
	}
	yCap, yHigh, yTitle, yLow := grid.Rows[0].Cells[0], grid.Rows[1].Cells[0], grid.Rows[2].Cells[0], grid.Rows[5].Cells[0]
	xBar := grid.Rows[7].Cells
	xLow, xTitle, xHigh := xBar[1], xBar[2], xBar[3]
	bar := string(xTitle.Shape.Fill)
	if strings.Contains(bar, "accent") {
		t.Errorf("axis bar fill = %s, want the neutral dark", bar)
	}
	for name, c := range map[string]*jsonschema.GridCellInput{"y cap": yCap, "y high": yHigh, "y title": yTitle, "y low": yLow, "x low": xLow, "x high": xHigh} {
		if c.Shape == nil || string(c.Shape.Fill) != bar {
			t.Errorf("%s fill = %v, want the bar fill %s", name, c.Shape, bar)
		}
	}
	if yCap.Shape.Geometry != "triangle" || xHigh.Shape.Geometry != "homePlate" {
		t.Errorf("bar points = %s / %s, want a triangle above the y bar and a homePlate ending the x bar", yCap.Shape.Geometry, xHigh.Shape.Geometry)
	}
	for name, want := range map[string]struct {
		cell *jsonschema.GridCellInput
		text string
		vert bool
	}{
		"y high": {yHigh, "High", true}, "y title": {yTitle, "Market Growth", true}, "y low": {yLow, "Low", true},
		"x low": {xLow, "Low", false}, "x title": {xTitle, "Market Share", false}, "x high": {xHigh, "High", false},
	} {
		text := string(want.cell.Shape.Text)
		if !strings.Contains(text, want.text) || strings.Contains(text, "vert270") != want.vert {
			t.Errorf("%s text = %s", name, text)
		}
	}
	if !strings.Contains(string(xTitle.Shape.Text), `"bold":true`) || !strings.Contains(string(yTitle.Shape.Text), `"bold":true`) {
		t.Error("axis titles must be bold")
	}
}

// The highlighted quadrant is the matrix's one solid accent block.
func TestMatrix2x2TonalHighlight(t *testing.T) {
	p := &matrix2x2{}
	v := openMatrixValues()
	v.BottomLeft.Highlight = true
	grid, err := p.Expand(testThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range tonalMatrixQuadrants(t, grid) {
		if solid := string(q.Shape.Fill) == `"accent1"`; solid != (i == 2) {
			t.Errorf("quadrant %d fill = %s", i, q.Shape.Fill)
		}
	}
}

// The tonal matrix picks its type from one ladder: the larger step for
// short copy, smaller steps as the quadrants fill, an authored size kept.
func TestMatrix2x2TonalScale(t *testing.T) {
	ctx := testThemeCtx()
	if lay := layoutMatrix2x2(ctx, openMatrixValues(), &Matrix2x2Overrides{}); lay.headerSize != scaleLeadPt || lay.bodySize != scaleSubheadPt {
		t.Errorf("short copy set at %.0f / %.0fpt, want 18 / 14", lay.headerSize, lay.bodySize)
	}
	long := openMatrixValues()
	for _, q := range []*Matrix2x2Quadrant{&long.TopLeft, &long.TopRight, &long.BottomLeft, &long.BottomRight} {
		q.Header = strings.Repeat("Header words ", 6)
		q.Body = strings.Repeat("body copy ", 20)
	}
	half := ctx
	half.LayoutBounds = LayoutBounds{Width: 8000000, Height: 2400000}
	if lay := layoutMatrix2x2(half, long, &Matrix2x2Overrides{}); lay.headerSize >= scaleLeadPt || lay.bodySize != scaleBodyPt {
		t.Errorf("long copy set at %.0f / %.0fpt, want a smaller step with a 12pt body", lay.headerSize, lay.bodySize)
	}
	if lay := layoutMatrix2x2(ctx, openMatrixValues(), &Matrix2x2Overrides{TextOverrides: TextOverrides{BodySize: 12}}); lay.headerSize != sizeHeaderPt || lay.bodySize != 12 {
		t.Errorf("authored body_size: set at %.0f / %.0fpt, want 16 / 12", lay.headerSize, lay.bodySize)
	}
	// A long y title wraps across a thicker bar instead of shrinking.
	wide := openMatrixValues()
	wide.YAxisLabel = "Strategic importance of the initiative to the group portfolio"
	short := layoutMatrix2x2Bars(ctx, openMatrixValues(), scaleSubheadPt)
	if bars := layoutMatrix2x2Bars(ctx, wide, scaleSubheadPt); bars.yThickPt <= short.yThickPt || bars.yThickPt > matrix2x2BarMaxPt {
		t.Errorf("y bar %.0fpt for a 60-character title, %.0fpt for a short one", bars.yThickPt, short.yThickPt)
	}
}
