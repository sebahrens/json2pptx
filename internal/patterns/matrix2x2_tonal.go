package patterns

import (
	"encoding/json"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Tonal matrix (go-slide-creator-ckpye): the default look of matrix-2x2.
//
//	▲ ┌───────────────┐ ┌───────────────┐
//	│ │ Quick wins    │ │ Major projects│
//	I │ body          │ │ body          │
//	m └───────────────┘ └───────────────┘
//	p ┌───────────────┐ ┌───────────────┐
//	a │ Fill-ins      │ │ Thankless     │
//	c │ body          │ │ body          │
//	t └───────────────┘ └───────────────┘
//	  [ Low        Effort          High >
//
// The four quadrants are filled fields in the accent's content swatch, held
// apart by a white gutter — the cross is the paper, not two hairlines — and
// the quadrant the slide is about (highlight) is the one solid accent block.
// Each quadrant sets its name bold at the top left with its body under it.
// The axes are two dark bars along the left and bottom edges, each ending in
// a point at its "high" end and carrying its low end, its bold title and its
// high end in that order: a reader sees which way each axis grows without
// hunting for a "Low" floating beside a line.
//
// A bar is three abutting shapes of one fill, so every label is measured on
// its own fill and no shape is rotated (J2P-MATRIX-005: only the y bar's text
// direction is). They sit on the matrix's own grid — columns [y bar | gap |
// x low | left rest | gutter | right rest | x high], rows [cap | y high | top
// rest | gutter | bottom rest | y low | gap | x bar] — because a nested grid
// would be inset from the quadrants' edges.

const (
	// matrix2x2TonalGutterPt is the white cross between the quadrants and the
	// gap between the quadrants and the axis bars.
	matrix2x2TonalGutterPt = 6.0
	// matrix2x2BarPointPt is the depth of an axis bar's point.
	matrix2x2BarPointPt = 12.0
	// matrix2x2BarInsetPt is an axis bar's text margin across its thickness.
	matrix2x2BarInsetPt = 3.0
	// matrix2x2XEndFrac is each end label's share of the x bar's length.
	matrix2x2XEndFrac = 0.18
	// matrix2x2YEndMaxFrac caps each end label's share of the y bar's length
	// and matrix2x2YEndMinPt is the least it takes.
	matrix2x2YEndMaxFrac = 0.30
	matrix2x2YEndMinPt   = 36.0
	// matrix2x2BarMaxPt is the thickest an axis bar grows for a long title.
	matrix2x2BarMaxPt = 84.0
	// matrix2x2LengthSafety discounts the y bar's estimated length: its rows
	// flex, and a bar measured to the last point would be written shrunk.
	matrix2x2LengthSafety = 0.92
)

// matrix2x2TonalScales is the tonal matrix's default type ladder, header then
// body, largest first: the first at which both quadrant rows fit is taken.
var matrix2x2TonalScales = [][2]float64{{scaleLeadPt, scaleSubheadPt}, {sizeHeaderPt, scaleBodyPt}, {scaleSubheadPt, scaleBodyPt}, {matrix2x2MinHeaderPt, scaleBodyPt}}

// matrix2x2Bars is the measured geometry of the two axis bars.
type matrix2x2Bars struct {
	xThickPt float64 // height of the x bar
	yThickPt float64 // width of the y bar
	yEndPt   float64 // length of each end label of the y bar
	quadW    float64 // width of one quadrant
	availPt  float64 // height the two quadrant rows share
	matrixW  float64 // width of the quadrant field (both quadrants and the gutter)
}

// matrix2x2BarText is one segment of an axis bar: text at the bar's small
// margin across its thickness. vert is "vert270" on the y bar.
func matrix2x2BarText(content string, size float64, bold bool, ink, align, vert string, insetEnd float64) json.RawMessage {
	para := map[string]any{"content": content, "size": size, "color": ink, "align": align}
	if bold {
		para["bold"] = true
	}
	obj := map[string]any{"paragraphs": []any{para}, "align": align, "vertical_align": "ctr"}
	lr, tb := defaultShapeInsetLRPt, matrix2x2BarInsetPt
	left, right, top, bottom := lr, lr, tb, tb
	if vert != "" {
		// The text runs up the bar: its line margins are the bar's top and
		// bottom, its across-thickness margins the bar's left and right.
		obj["vert"] = vert
		left, right, top, bottom = tb, tb, lr, lr
		if insetEnd > 0 {
			top = insetEnd
		}
	} else if insetEnd > 0 {
		right = insetEnd
	}
	obj["inset_left"], obj["inset_right"], obj["inset_top"], obj["inset_bottom"] = left, right, top, bottom
	data, _ := json.Marshal(obj)
	return data
}

// matrix2x2BarFits reports whether the writer stores text unshrunk in a box
// of wPt x hPt.
func matrix2x2BarFits(fonts pptx.ThemeFonts, text json.RawMessage, wPt, hPt float64) bool {
	tb, err := shapegrid.ResolveTextInput(text)
	if err != nil || tb == nil {
		return true
	}
	tb.ThemeFonts = fonts
	return pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(wPt * sizingEMUPerPt), CY: int64(hPt * sizingEMUPerPt)})
}

// layoutMatrix2x2Bars measures the axis bars: the x bar as thick as its
// title and ends need at their shares of its length, the y bar's end labels
// as long as their words need and the bar as thick as its title then needs
// (a long y title wraps onto a second line across the bar).
func layoutMatrix2x2Bars(ctx ExpandContext, v *Matrix2x2Values, labelSize float64) matrix2x2Bars {
	fonts := ctx.themeFonts()
	areaW, areaH := sizingAreaPt(ctx)
	gap := ctx.Gap(matrix2x2TonalGutterPt)
	endSize := matrix2x2EndSize(labelSize)
	xLow, xHigh := axisEnds(v.XLow, v.XHigh)
	yLow, yHigh := axisEnds(v.YLow, v.YHigh)
	oneLine := math.Ceil(labelSize*sizingLineSpacing + 2*matrix2x2BarInsetPt + 2)

	b := matrix2x2Bars{yThickPt: oneLine}
	measure := func() {
		b.matrixW = areaW - b.yThickPt - gap
		b.quadW = (b.matrixW - gap) / 2
		b.xThickPt = oneLine
		for _, seg := range []struct {
			text json.RawMessage
			w    float64
		}{
			{matrix2x2BarText(v.XAxisLabel, labelSize, true, "lt1", "ctr", "", 0), b.matrixW * (1 - 2*matrix2x2XEndFrac)},
			{matrix2x2BarText(xLow, endSize, false, "lt1", "l", "", 0), b.matrixW * matrix2x2XEndFrac},
			{matrix2x2BarText(xHigh, endSize, false, "lt1", "r", "", defaultShapeInsetLRPt), b.matrixW*matrix2x2XEndFrac - matrix2x2BarPointPt/2},
		} {
			b.xThickPt = math.Max(b.xThickPt, writtenFitHeightPt(fonts, seg.text, seg.w, 0))
		}
		b.xThickPt = math.Min(math.Ceil(b.xThickPt), matrix2x2BarMaxPt)
		b.availPt = areaH - b.xThickPt - 2*gap
	}
	measure()

	// The y bar: end labels first, at one line across the bar.
	length := (b.availPt + gap - matrix2x2BarPointPt) * matrix2x2LengthSafety
	b.yEndPt = matrix2x2YEndMinPt
	maxEnd := math.Max(length*matrix2x2YEndMaxFrac, matrix2x2YEndMinPt)
	for _, end := range []string{yLow, yHigh} {
		text := matrix2x2BarText(end, endSize, false, "lt1", "ctr", "vert270", 0)
		for b.yEndPt < maxEnd && !matrix2x2BarFits(fonts, text, b.yThickPt, b.yEndPt) {
			b.yEndPt += 4
		}
	}
	b.yEndPt = math.Min(math.Ceil(b.yEndPt), maxEnd)
	title := matrix2x2BarText(v.YAxisLabel, labelSize, true, "lt1", "ctr", "vert270", 0)
	for b.yThickPt < matrix2x2BarMaxPt && !matrix2x2BarFits(fonts, title, b.yThickPt, length-2*b.yEndPt) {
		b.yThickPt += 2
	}
	measure()
	return b
}

// matrix2x2EndSize is the size of an axis end label under a title of size.
func matrix2x2EndSize(labelSize float64) float64 {
	return math.Max(labelSize-2, shapegrid.MinTextSizePt)
}

// matrix2x2TonalContent is a tonal quadrant's text: its name bold at the top
// left, its body under it.
func matrix2x2TonalContent(q Matrix2x2Quadrant, headerSize, bodySize float64, ink string) json.RawMessage {
	paras := []chartInsightsParagraph{{Content: q.Header, Size: headerSize, Bold: true, Color: ink, Align: "l", SpaceAfter: 4}}
	if q.Body != "" {
		paras = append(paras, chartInsightsParagraph{Content: q.Body, Size: bodySize, Color: ink, Align: "l"})
	}
	return patternTextObj{Paragraphs: paras, Align: "l", VerticalAlign: "t"}.json()
}

// expandMatrix2x2Tonal draws the tonal matrix (see the file comment).
func expandMatrix2x2Tonal(ctx ExpandContext, vals *Matrix2x2Values, lay matrix2x2Layout, labelSize float64, accent string, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	bars := lay.bars
	gap := ctx.Gap(matrix2x2TonalGutterPt)
	endSize := matrix2x2EndSize(labelSize)

	field := tonalContent(ctx, accent)
	fieldInk := tonalInk(ctx, field)
	quadrants := []Matrix2x2Quadrant{vals.TopLeft, vals.TopRight, vals.BottomLeft, vals.BottomRight}
	cells := make([]*jsonschema.GridCellInput, 4)
	for i, q := range quadrants {
		tone, ink := field, fieldInk
		if q.Highlight {
			tone, ink = tonalEmphasis(ctx, accent)
		}
		shape := &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     tone.fillJSON(),
			Line:     noLine,
			Text:     matrix2x2TonalContent(q, lay.headerSize, lay.bodySize, ink),
		}
		if q.Icon != nil {
			if icon := q.Icon.Resolve(iconFillOn(ctx, shape.Fill, accent), "top"); icon != nil {
				shape.Icon = icon
			}
		}
		cells[i] = &jsonschema.GridCellInput{ColSpan: 2, RowSpan: 2, Shape: shape}
		applyMatrix2x2CellOverride(cells[i], cellOverrides, i, accent)
	}
	// The top quadrants also span the y bar's cap row.
	cells[0].RowSpan, cells[1].RowSpan = 3, 3

	barTone, barInk := tonalBadge(ctx)
	// Outlined in its own colour: the segments of a bar sit a hair apart and
	// the page would show through as seams.
	barFill, barLine := barTone.fillJSON(), metricListBandLine(barTone)
	seg := func(geometry string, text json.RawMessage) *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: geometry, Fill: barFill, Line: barLine, Text: text}}
	}
	xLow, xHigh := axisEnds(vals.XLow, vals.XHigh)
	yLow, yHigh := axisEnds(vals.YLow, vals.YHigh)

	// y bar, top to bottom: the point, the high end, the title, the low end.
	yCap := seg("triangle", nil)
	yHighCell := seg("rect", matrix2x2BarText(yHigh, endSize, false, barInk, "ctr", "vert270", 0))
	yTitle := seg("rect", matrix2x2BarText(vals.YAxisLabel, labelSize, true, barInk, "ctr", "vert270", 0))
	yTitle.RowSpan = 3
	yLowCell := seg("rect", matrix2x2BarText(yLow, endSize, false, barInk, "ctr", "vert270", 0))

	// x bar, left to right: the low end, the title, the high end in the point.
	xEndW := bars.matrixW * matrix2x2XEndFrac
	xLowCell := seg("rect", matrix2x2BarText(xLow, endSize, false, barInk, "l", "", 0))
	xTitle := seg("rect", matrix2x2BarText(vals.XAxisLabel, labelSize, true, barInk, "ctr", "", 0))
	xTitle.ColSpan = 3
	xHighCell := seg("homePlate", matrix2x2BarText(xHigh, endSize, false, barInk, "r", "", defaultShapeInsetLRPt))
	xHighCell.Shape.Adjustments = map[string]int64{"adj": swimlanePointAdj(matrix2x2BarPointPt, xEndW, bars.xThickPt)}

	fixed := func(h float64, cells ...*jsonschema.GridCellInput) jsonschema.GridRowInput {
		return jsonschema.GridRowInput{MinHeight: h, MaxHeight: h, Cells: cells}
	}
	spacer := func(span int) *jsonschema.GridCellInput { return &jsonschema.GridCellInput{ColSpan: span} }
	// The low end is as long as the high end and its point together, so the
	// two quadrant rows stay equal.
	yLowPt := bars.yEndPt + matrix2x2BarPointPt
	top := jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{yTitle, spacer(1), spacer(1)}}
	bottom := jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{spacer(1), cells[2], spacer(1), cells[3]}}
	// The quadrant rows share the height equally unless one pair's written
	// fit needs more than half (go-slide-creator-n1muf); each row's fixed
	// part (the y bar's end) counts towards its need.
	if lay.needs[0] > lay.availPt/2 || lay.needs[1] > lay.availPt/2 {
		for i, row := range []*jsonschema.GridRowInput{&top, &bottom} {
			rest := math.Ceil(math.Max(lay.needs[i]-yLowPt, 1))
			row.MinHeight, row.Flex = rest, rest
		}
	}
	rows := []jsonschema.GridRowInput{
		fixed(matrix2x2BarPointPt, yCap, spacer(1), cells[0], spacer(1), cells[1]),
		fixed(bars.yEndPt, yHighCell, spacer(1), spacer(1)),
		top,
		fixed(gap, spacer(1), spacer(5)),
		bottom,
		fixed(yLowPt, yLowCell, spacer(1), spacer(1)),
		fixed(gap, spacer(7)),
		fixed(bars.xThickPt, spacer(2), xLowCell, xTitle, xHighCell),
	}

	rest := bars.quadW - xEndW
	colsJSON, _ := json.Marshal([]float64{bars.yThickPt, gap, xEndW, rest, gap, rest, xEndW})
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  matrix2x2OpenGapPt,
		RowGap:  matrix2x2OpenGapPt,
		Rows:    rows,
	}
}
