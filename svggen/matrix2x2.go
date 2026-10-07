package svggen

import (
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// 2x2 Matrix (Quadrant Chart)
// =============================================================================

// Matrix2x2Config holds configuration for 2x2 matrix charts.
type Matrix2x2Config struct {
	ChartConfig

	// QuadrantLabels are the labels for each quadrant [top-left, top-right, bottom-left, bottom-right].
	QuadrantLabels [4]string

	// QuadrantColors are the background colors for each quadrant.
	QuadrantColors [4]Color

	// QuadrantOpacity is the fill opacity for quadrant backgrounds.
	QuadrantOpacity float64

	// XAxisLabel is the label for the x-axis (bottom).
	XAxisLabel string

	// YAxisLabel is the label for the y-axis (left).
	YAxisLabel string

	// XAxisMin is the minimum x value (left edge).
	XAxisMin float64

	// XAxisMax is the maximum x value (right edge).
	XAxisMax float64

	// YAxisMin is the minimum y value (bottom edge).
	YAxisMin float64

	// YAxisMax is the maximum y value (top edge).
	YAxisMax float64

	// ShowGridLines enables grid lines at the axis midpoints.
	ShowGridLines bool

	// GridLineColor is the color of the grid lines.
	GridLineColor Color

	// GridLineWidth is the width of the grid lines.
	GridLineWidth float64

	// GridLineDash enables dashed grid lines.
	GridLineDash bool

	// PointSize is the default size for data points.
	PointSize float64

	// PointShape is the default shape for data points.
	PointShape MarkerShape

	// ShowPointLabels enables labels next to points.
	ShowPointLabels bool

	// LabelOffset is the distance from point center to label.
	LabelOffset float64

	// Gutter is the width of the background-coloured cross that holds the
	// four quadrant fields apart. Zero draws them abutting (with ShowGridLines
	// the two midlines then mark the split).
	Gutter float64

	// AxisBars draws each axis as a dark bar along the matrix's edge that
	// ends in a point at its high end and carries the low end, the bold axis
	// title and the high end. False draws the axis titles as plain text.
	AxisBars bool
}

// DefaultMatrix2x2Config returns default matrix 2x2 configuration.
func DefaultMatrix2x2Config(width, height float64) Matrix2x2Config {
	config := Matrix2x2Config{
		ChartConfig: DefaultChartConfig(width, height),
		QuadrantColors: [4]Color{
			MustParseColor(DefaultThemeAccent5Hex).WithAlpha(0.30), // Green tint
			MustParseColor(DefaultThemeAccent6Hex).WithAlpha(0.30), // Yellow tint
			MustParseColor(DefaultThemeAccent4Hex).WithAlpha(0.30), // Teal tint
			MustParseColor(DefaultThemeAccent3Hex).WithAlpha(0.30), // Red tint
		},
		QuadrantOpacity: 0.30,
		XAxisLabel:      "Effort",
		YAxisLabel:      "Value",
		XAxisMin:        0,
		XAxisMax:        100,
		YAxisMin:        0,
		YAxisMax:        100,
		ShowGridLines:   true,
		GridLineColor:   MustParseColor(DefaultThemeTextMutedHex),
		GridLineWidth:   1.5,
		GridLineDash:    false,
		PointSize:       12,
		PointShape:      MarkerCircle,
		ShowPointLabels: true,
		LabelOffset:     16,
	}
	config.QuadrantLabels = defaultMatrixQuadrantLabels(config.XAxisLabel, config.YAxisLabel)
	return config
}

func defaultMatrixQuadrantLabels(xAxis, yAxis string) [4]string {
	xAxis = strings.TrimSpace(xAxis)
	yAxis = strings.TrimSpace(yAxis)
	if xAxis == "" {
		xAxis = "Effort"
	}
	if yAxis == "" {
		yAxis = "Value"
	}
	return [4]string{
		"High " + yAxis + " / Low " + xAxis,
		"High " + yAxis + " / High " + xAxis,
		"Low " + yAxis + " / Low " + xAxis,
		"Low " + yAxis + " / High " + xAxis,
	}
}

// Matrix2x2Point represents a data point in the matrix.
type Matrix2x2Point struct {
	// Label is the point label.
	Label string

	// X is the x-coordinate (typically 0-100).
	X float64

	// Y is the y-coordinate (typically 0-100).
	Y float64

	// Size overrides the default point size.
	Size float64

	// Color overrides the default point color.
	Color *Color

	// Shape overrides the default point shape.
	Shape MarkerShape

	// Description is optional additional text.
	Description string

	// Series groups points that share a colour (one accent per series, in
	// first-appearance order). Read from "series", "group" or "category".
	Series string
}

// Matrix2x2Data represents the data for a 2x2 matrix chart.
type Matrix2x2Data struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Points are the data points to plot.
	Points []Matrix2x2Point

	// QuadrantItems holds the coordinate-free form: per quadrant, the items
	// that belong in it. data-format-hints promotes this as the alternative for
	// an agent with no numbers, and it used to be turned into fabricated scatter
	// coordinates — two items per quadrant were already enough for the dots'
	// labels to overprint each other and the quadrant caption
	// (go-slide-creator-s27x). Quadrants carrying items render as titled lists
	// instead, and Points is left empty.
	// Index order matches QuadrantLabels: top-left, top-right, bottom-left,
	// bottom-right.
	QuadrantItems [4][]string

	// Footnote is an optional footnote.
	Footnote string
}

// HasQuadrantItems reports whether the coordinate-free quadrant form was used.
func (d Matrix2x2Data) HasQuadrantItems() bool {
	for _, items := range d.QuadrantItems {
		if len(items) > 0 {
			return true
		}
	}
	return false
}

// Matrix2x2Chart renders 2x2 matrix charts.
type Matrix2x2Chart struct {
	builder *SVGBuilder
	config  Matrix2x2Config

	// markerBoxes are the boxes of the point markers of the drawing in
	// progress: what a point label must keep clear of besides the headings
	// and the labels placed before it.
	markerBoxes []placedLabel
}

// NewMatrix2x2Chart creates a new matrix 2x2 chart renderer.
func NewMatrix2x2Chart(builder *SVGBuilder, config Matrix2x2Config) *Matrix2x2Chart {
	return &Matrix2x2Chart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the matrix 2x2 chart.
func (mc *Matrix2x2Chart) Draw(data Matrix2x2Data) error {
	b := mc.builder
	style := b.StyleGuide()

	// Calculate plot area
	plotArea := mc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if mc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Reserve space for axis name labels only (no "High"/"Low" value labels —
	// those are implied by the quadrant names and removing them prevents overlap
	// caused by LibreOffice SVG text scaling, bug go-slide-ceator-a8ax).
	textScale := 1.3 // line-height factor for layout calculations
	bodyH := style.Typography.SizeBody * textScale

	// Y-axis label is rotated -90°. Reserve bodyH plus padding.
	leftSpace := bodyH + style.Spacing.LG
	bottomSpace := bodyH + style.Spacing.LG // X-axis name (horizontal text)
	rightPad := style.Spacing.LG
	if mc.config.AxisBars {
		// The bars sit against the matrix: a bar and its gap on the left and
		// under it, and the matrix runs to the right margin.
		leftSpace = mc.axisBarThickness() + mc.axisBarGap()
		bottomSpace = leftSpace
		rightPad = 0
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + bottomSpace
	plotArea.X += leftSpace
	plotArea.W -= leftSpace + rightPad

	// Draw quadrant backgrounds
	mc.drawQuadrants(plotArea)

	// Draw grid lines
	if mc.config.ShowGridLines {
		mc.drawGridLines(plotArea)
	}

	// Draw the axes
	if mc.config.AxisBars {
		mc.drawAxisBars(plotArea)
	} else {
		mc.drawAxisLabels(plotArea)
	}

	if data.HasQuadrantItems() {
		// Coordinate-free form: each quadrant is a titled list filling its own
		// rectangle. No markers, and the title is the topmost line in the
		// rectangle rather than one more centred string among the item labels.
		mc.drawQuadrantLists(data, plotArea)
	} else {
		// Draw quadrant labels
		captionBands := mc.drawQuadrantLabels(plotArea)

		// Draw data points
		mc.drawPoints(data.Points, plotArea, captionBands)
	}

	// Draw title
	if mc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: mc.config.Width, H: headerHeight + mc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		fh := FootnoteReservedHeight(style)
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: mc.config.Height - fh,
			W: mc.config.Width,
			H: fh,
		})
	}

	return nil
}

// drawQuadrants draws the four quadrant backgrounds.
func (mc *Matrix2x2Chart) drawQuadrants(plotArea Rect) {
	b := mc.builder

	for i, rect := range mc.quadrantRects(plotArea) {
		b.Push()
		color := mc.config.QuadrantColors[i]
		if color.A == 0 {
			color = mc.config.QuadrantColors[i].WithAlpha(mc.config.QuadrantOpacity)
		}
		b.SetFillColor(color)
		b.SetStrokeColor(Color{A: 0}) // No stroke
		b.FillRect(rect)
		b.Pop()
	}
}

// quadrantRects returns the four quadrant fields — top-left, top-right,
// bottom-left, bottom-right — each pulled back from the midlines by half the
// gutter.
func (mc *Matrix2x2Chart) quadrantRects(plotArea Rect) [4]Rect {
	halfW := plotArea.W / 2
	halfH := plotArea.H / 2
	g := math.Max(0, math.Min(mc.config.Gutter, math.Min(halfW, halfH))) / 2
	return [4]Rect{
		{X: plotArea.X, Y: plotArea.Y, W: halfW - g, H: halfH - g},
		{X: plotArea.X + halfW + g, Y: plotArea.Y, W: halfW - g, H: halfH - g},
		{X: plotArea.X, Y: plotArea.Y + halfH + g, W: halfW - g, H: halfH - g},
		{X: plotArea.X + halfW + g, Y: plotArea.Y + halfH + g, W: halfW - g, H: halfH - g},
	}
}

// quadrantInk returns the text colour on quadrant i's field: preferred when
// it reads there (WCAG AA), else whichever of white and dark does.
func (mc *Matrix2x2Chart) quadrantInk(i int, preferred Color) Color {
	bg := mc.builder.StyleGuide().Palette.Background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	fill := mc.config.QuadrantColors[i]
	if fill.A == 0 {
		fill = fill.WithAlpha(mc.config.QuadrantOpacity)
	}
	fill = fill.BlendOver(bg)
	if preferred.ContrastWith(fill) >= WCAGAANormal {
		return preferred
	}
	return fill.TextColorFor()
}

// Axis bars (go-slide-creator-ckpye): the matrix_2x2 diagram and the
// matrix-2x2 pattern draw one family — tonal quadrant fields split by a
// gutter, and two dark bars that say which way each axis grows.
const (
	// matrixAxisBarInk is the share of the text ink an axis bar is filled
	// with (the pattern engine's badge tone).
	matrixAxisBarInk = 0.65
	// matrixAxisLow and matrixAxisHigh are the end labels of an axis bar.
	matrixAxisLow  = "Low"
	matrixAxisHigh = "High"
)

// matrixAxisBarColor is the fill of an axis bar, the pattern engine's badge
// tone: the template's dk2 where it carries the brand (a navy, a forest
// green) and reads as a dark mark on the background, else the text ink at
// matrixAxisBarInk.
func matrixAxisBarColor(p *Palette) Color {
	bg := p.Background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	dk2 := p.TextSecondary.BlendOver(bg)
	hi, lo := max(dk2.R, dk2.G, dk2.B), min(dk2.R, dk2.G, dk2.B)
	nearBlack := dk2.Luminance() < 0.02 && hi-lo <= 24
	if nearBlack || dk2.ContrastWith(bg) < WCAGAANormal {
		return NeutralInk(p, matrixAxisBarInk)
	}
	return dk2
}

// axisBarThickness is the thickness of an axis bar: one line of the axis
// title with room above and below.
func (mc *Matrix2x2Chart) axisBarThickness() float64 {
	return math.Ceil(mc.builder.StyleGuide().Typography.SizeBody * 2)
}

// axisBarGap separates an axis bar from the matrix.
func (mc *Matrix2x2Chart) axisBarGap() float64 {
	return math.Max(mc.config.Gutter, mc.builder.StyleGuide().Spacing.XS)
}

// drawAxisBars draws the two axes as dark bars: the x bar under the matrix
// pointing right, the y bar left of it pointing up. Each carries its low end,
// its bold title and its high end; the ends are dropped where the bar is too
// short to hold them beside the title.
func (mc *Matrix2x2Chart) drawAxisBars(plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()
	t, gap := mc.axisBarThickness(), mc.axisBarGap()
	point := t / 2
	fill := matrixAxisBarColor(style.Palette)
	ink := fill.TextColorFor()
	pad := style.Spacing.MD

	// drawTexts sets the low end, the title and the high end along a bar of
	// the given length; at(d) is the point d along the bar's centre line.
	drawTexts := func(title string, length float64, draw func(text string, d float64, align TextAlign)) {
		titleSize, endSize := style.Typography.SizeBody, style.Typography.SizeSmall
		b.SetFontWeight(style.Typography.WeightBold)
		b.SetFontSize(titleSize)
		floor := LabelFloor(b, 8)
		avail := length - point - 2*pad
		titleW, _ := b.MeasureText(title)
		for titleW > avail && titleSize > floor {
			titleSize = math.Max(floor, titleSize-0.5)
			b.SetFontSize(titleSize)
			titleW, _ = b.MeasureText(title)
		}
		centre := (length - point) / 2
		draw(title, centre, TextAlignCenter)

		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetFontSize(endSize)
		lowW, _ := b.MeasureText(matrixAxisLow)
		highW, _ := b.MeasureText(matrixAxisHigh)
		// The ends need clear air beside the title (LibreOffice sets text a
		// little wider than it is measured here).
		if centre-titleW/2-pad/2 < pad+math.Max(lowW, highW)*1.2 {
			return
		}
		draw(matrixAxisLow, pad, TextAlignLeft)
		draw(matrixAxisHigh, length-point-pad*0.25, TextAlignRight)
	}

	b.Push()
	b.SetFillColor(fill)
	b.SetStrokeColor(Color{A: 0})
	b.SetStrokeWidth(0)

	// X bar.
	x0, x1 := plotArea.X, plotArea.X+plotArea.W
	y0 := plotArea.Y + plotArea.H + gap
	b.DrawPolygon([]Point{{X: x0, Y: y0}, {X: x1 - point, Y: y0}, {X: x1, Y: y0 + t/2}, {X: x1 - point, Y: y0 + t}, {X: x0, Y: y0 + t}})

	// Y bar.
	xr := plotArea.X - gap
	xl := xr - t
	yt, yb := plotArea.Y, plotArea.Y+plotArea.H
	b.DrawPolygon([]Point{{X: xl, Y: yb}, {X: xl, Y: yt + point}, {X: xl + t/2, Y: yt}, {X: xr, Y: yt + point}, {X: xr, Y: yb}})
	b.Pop()

	b.Push()
	b.SetTextColor(ink)
	drawTexts(mc.config.XAxisLabel, plotArea.W, func(text string, d float64, align TextAlign) {
		b.DrawText(text, x0+d, y0+t/2, align, TextBaselineMiddle)
	})
	// The y bar's text reads bottom to top: rotated -90° about its anchor
	// (the SVG postprocessor rewrites the matrix transform for LibreOffice).
	drawTexts(mc.config.YAxisLabel, plotArea.H, func(text string, d float64, align TextAlign) {
		cx, cy := xl+t/2, yb-d
		b.Push()
		b.RotateAround(-90, cx, cy)
		b.DrawText(text, cx, cy, align, TextBaselineMiddle)
		b.Pop()
	})
	b.Pop()
}

// drawGridLines draws the grid lines at the midpoints.
func (mc *Matrix2x2Chart) drawGridLines(plotArea Rect) {
	b := mc.builder

	midX := plotArea.X + plotArea.W/2
	midY := plotArea.Y + plotArea.H/2

	b.Push()
	b.SetStrokeColor(mc.config.GridLineColor)
	b.SetStrokeWidth(mc.config.GridLineWidth)

	if mc.config.GridLineDash {
		b.SetDashes(6, 3)
	}

	// Vertical line (x-axis midpoint)
	b.DrawLine(midX, plotArea.Y, midX, plotArea.Y+plotArea.H)

	// Horizontal line (y-axis midpoint)
	b.DrawLine(plotArea.X, midY, plotArea.X+plotArea.W, midY)

	b.Pop()
}

// drawAxisLabels draws the axis name labels outside the plot area.
// Only axis names are drawn — "High"/"Low" value labels are omitted to avoid
// overlap with quadrant labels (LibreOffice SVG text scaling, bug a8ax).
// The quadrant labels themselves convey the High/Low semantics.
// The Y-axis label is rotated -90° (reads bottom-to-top) using RotateAround;
// the SVG postprocessor (fixSVGMatrixRotations) converts the matrix transform
// to translate+rotate form for LibreOffice compatibility.
func (mc *Matrix2x2Chart) drawAxisLabels(plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()

	b.Push()

	// X-axis name (bottom center)
	xLabelY := plotArea.Y + plotArea.H + style.Spacing.LG
	b.SetFontSize(style.Typography.SizeBody)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.DrawText(mc.config.XAxisLabel, plotArea.X+plotArea.W/2, xLabelY, TextAlignCenter, TextBaselineTop)

	// Y-axis name (rotated -90°, reads bottom-to-top)
	b.SetFontSize(style.Typography.SizeBody)
	b.SetFontWeight(style.Typography.WeightMedium)
	// Beside the matrix's left edge, not at the canvas edge: anchored at the
	// canvas edge the title floated a margin's width away from the matrix
	// (go-slide-creator-njdno). Mirrors the x-axis title's offset below.
	yLabelX := math.Max(style.Typography.SizeBody, plotArea.X-style.Spacing.LG)
	yLabelY := plotArea.Y + plotArea.H/2
	b.Push()
	b.RotateAround(-90, yLabelX, yLabelY)
	b.DrawText(mc.config.YAxisLabel, yLabelX, yLabelY, TextAlignCenter, TextBaselineMiddle)
	b.Pop()

	b.Pop()
}

// drawQuadrantLabels draws the labels and returns the occupied header bands so
// point markers and point labels can stay out of them.
func (mc *Matrix2x2Chart) drawQuadrantLabels(plotArea Rect) [4]placedLabel {
	b := mc.builder
	style := b.StyleGuide()

	halfW := plotArea.W / 2
	halfH := plotArea.H / 2
	pad := style.Spacing.MD
	labels := mc.config.QuadrantLabels
	if halfW < 150 && labels == defaultMatrixQuadrantLabels(mc.config.XAxisLabel, mc.config.YAxisLabel) {
		// The axis names already spell out what High/Low refer to. On a tiny
		// plot, repeating them in every quadrant consumes the entire point area.
		labels = [4]string{"High / Low", "High / High", "Low / Low", "Low / High"}
	}
	// Each quadrant label gets a bounded rectangle within its quadrant's field.
	fields := mc.quadrantRects(plotArea)
	rects := make([]Rect, len(fields))
	for i, f := range fields {
		rects[i] = Rect{X: f.X + pad, Y: f.Y + pad, W: f.W - 2*pad, H: f.H - 2*pad}
	}

	// Captions sit in each quadrant's OUTER TOP corner, not its centre.
	// Centred captions occupy exactly the region a point cluster lands in — on
	// the reported five-initiative chart "ERP upgrade" printed straight through
	// "Major projects" (go-slide-creator-s27x). The corner is also the
	// conventional place for a quadrant name.
	aligns := []BoxAlign{
		AlignTopLeft,  // top-left quadrant
		AlignTopRight, // top-right quadrant
		AlignTopLeft,  // bottom-left quadrant
		AlignTopRight, // bottom-right quadrant
	}

	// Use LabelFitStrategy with wrapping to adapt heading size for narrow canvases.
	// In narrow placeholders (e.g., right column of two-column layouts),
	// quadrant label boxes can be very small. The strategy sizes for
	// multi-line wrapped text within the rect (both width and height),
	// so labels wrap naturally instead of being truncated with ellipsis.
	labelBoxW := halfW - 2*pad
	labelBoxH := halfH - 2*pad
	// Use a generous preferred size and a 10pt absolute floor so quadrant
	// labels remain readable even on small half-width canvases where the
	// style guide's scaled SizeBody can drop to ~7-8pt.
	preferredSize := math.Max(style.Typography.SizeHeading, 14)
	minLabelSize := math.Max(style.Typography.SizeBody, 10)
	quadrantFit := LabelFitStrategy{
		PreferredSize: preferredSize,
		MinSize:       minLabelSize,
		AllowWrap:     true,
		MaxLines:      4,
		MinCharWidth:  5.5,
	}
	// Find the longest quadrant label to determine font scaling.
	longestLabel := ""
	for _, label := range labels {
		if len([]rune(label)) > len([]rune(longestLabel)) {
			longestLabel = label
		}
	}
	fontSize := style.Typography.SizeHeading
	if longestLabel != "" {
		result := quadrantFit.Fit(b, longestLabel, labelBoxW, labelBoxH)
		fontSize = result.FontSize
	}

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightBold)

	var bands [4]placedLabel
	for i, label := range labels {
		if label == "" {
			continue
		}
		// A caption is a top band, not free space for plotted data. Use the
		// actual wrapped block height, leaving at least 38% of each quadrant for
		// points even when the heading is long.
		block := b.WrapText(label, rects[i].W)
		blockH := block.TotalHeight
		bandH := math.Min(halfH*0.62, math.Max(fontSize*1.3, blockH)+2*pad)
		// Match the conservative LibreOffice width allowance used for point
		// labels; otherwise a marker can still strike the first glyph.
		bandW := math.Min(halfW, block.TotalWidth*1.20+2*pad)
		bandX := rects[i].X - pad
		if aligns[i].Horizontal == HorizontalAlignRight {
			bandX = rects[i].X + rects[i].W + pad - bandW
		}
		bands[i] = placedLabel{x: bandX, y: rects[i].Y - pad, w: bandW, h: bandH}
		labelRect := rects[i]
		labelRect.H = math.Max(1, bandH-2*pad)
		b.SetTextColor(mc.quadrantInk(i, style.Palette.TextPrimary))
		b.DrawWrappedText(label, labelRect, aligns[i])
	}

	b.Pop()
	return bands
}

// drawQuadrantLists renders the coordinate-free form: each quadrant is a
// heading followed by a bulleted list, laid out top-down inside its own
// rectangle (go-slide-creator-s27x). Nothing is plotted, so nothing can collide
// with anything in another quadrant, and the title is always the topmost line.
func (mc *Matrix2x2Chart) drawQuadrantLists(data Matrix2x2Data, plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()

	pad := style.Spacing.MD
	rects := mc.quadrantRects(plotArea)
	for i, f := range rects {
		rects[i] = Rect{X: f.X + pad, Y: f.Y + pad, W: f.W - 2*pad, H: f.H - 2*pad}
	}

	// One type scale across all four quadrants, driven by the fullest one, so
	// the matrix reads as a single object rather than four independent lists.
	titleSize, itemSize := mc.quadrantListSizes(data, rects[0])

	for i, rect := range rects {
		title := mc.config.QuadrantLabels[i]
		items := data.QuadrantItems[i]
		if title == "" && len(items) == 0 {
			continue
		}

		y := rect.Y
		if title != "" {
			b.Push()
			b.SetFontSize(titleSize)
			b.SetFontWeight(style.Typography.WeightBold)
			b.SetTextColor(mc.quadrantInk(i, style.Palette.TextPrimary))
			b.DrawText(title, rect.X, y+titleSize*0.5, TextAlignLeft, TextBaselineMiddle)
			b.Pop()
			y += titleSize * quadrantListLineFactor
		}

		if len(items) == 0 {
			continue
		}
		b.Push()
		b.SetFontSize(itemSize)
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(mc.quadrantInk(i, style.Palette.TextSecondary))
		bulletIndent := itemSize * 0.9
		itemFit := LabelFitStrategy{PreferredSize: itemSize, MinSize: itemSize, MinCharWidth: 4.5}
		for _, item := range items {
			lineH := itemSize * quadrantListLineFactor
			if y+lineH > rect.Y+rect.H {
				// Out of room: say so rather than drawing past the quadrant.
				b.DrawText("…", rect.X, y+itemSize*0.5, TextAlignLeft, TextBaselineMiddle)
				break
			}
			text := itemFit.Fit(b, item, rect.W-bulletIndent, 0).DisplayText
			b.DrawText("•", rect.X, y+itemSize*0.5, TextAlignLeft, TextBaselineMiddle)
			b.DrawText(text, rect.X+bulletIndent, y+itemSize*0.5, TextAlignLeft, TextBaselineMiddle)
			y += lineH
		}
		b.Pop()
	}
}

// quadrantListLineFactor is the line height of a quadrant list line as a
// multiple of its font size.
const quadrantListLineFactor = 1.45

// quadrantListSizes picks one heading and one item size for all four quadrants,
// shrinking until the fullest quadrant's block fits its rectangle.
func (mc *Matrix2x2Chart) quadrantListSizes(data Matrix2x2Data, rect Rect) (titleSize, itemSize float64) {
	style := mc.builder.StyleGuide()

	maxItems := 0
	for _, items := range data.QuadrantItems {
		if len(items) > maxItems {
			maxItems = len(items)
		}
	}

	titleSize = math.Max(style.Typography.SizeHeading, 12)
	itemSize = math.Max(style.Typography.SizeBody, 9)
	const minTitle, minItem = 9.0, 7.0

	for titleSize > minTitle || itemSize > minItem {
		needed := titleSize * quadrantListLineFactor
		needed += float64(maxItems) * itemSize * quadrantListLineFactor
		if needed <= rect.H {
			break
		}
		titleSize = math.Max(minTitle, titleSize*0.92)
		itemSize = math.Max(minItem, itemSize*0.92)
	}
	return titleSize, itemSize
}

// placedLabel tracks a rendered label's bounding box for collision avoidance.
type placedLabel struct {
	x, y, w, h float64
}

// overlaps checks whether two label bounding boxes overlap.
func (a placedLabel) overlaps(b placedLabel) bool {
	return a.x < b.x+b.w && a.x+a.w > b.x && a.y < b.y+b.h && a.y+a.h > b.y
}

// drawPoints draws the data points.
func (mc *Matrix2x2Chart) drawPoints(points []Matrix2x2Point, plotArea Rect, captionBands [4]placedLabel) {
	b := mc.builder
	style := b.StyleGuide()
	palette := style.Palette

	// Create scales
	xScale := NewLinearScale(mc.config.XAxisMin, mc.config.XAxisMax)
	xScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)

	yScale := NewLinearScale(mc.config.YAxisMin, mc.config.YAxisMax)
	yScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y) // Inverted for screen coords

	// Adaptive font size: reduce label font when there are many points to
	// decrease collision pressure. Below 8 points use the normal SizeSmall;
	// from 8-15 points interpolate down to SizeCaption; above 15 stay at
	// SizeCaption.
	labelFontSize := style.Typography.SizeSmall
	n := len(points)
	if n >= 8 {
		minFont := style.Typography.SizeCaption
		if n >= 15 {
			labelFontSize = minFont
		} else {
			// Linear interpolation: 8 → SizeSmall, 15 → SizeCaption
			t := float64(n-8) / 7.0
			labelFontSize = style.Typography.SizeSmall - t*(style.Typography.SizeSmall-minFont)
		}
	}

	// Adaptive label offset: tighten when font shrinks so labels stay close
	// to their data points.
	labelOffset := mc.config.LabelOffset
	if labelFontSize < style.Typography.SizeSmall {
		labelOffset = labelOffset * (labelFontSize / style.Typography.SizeSmall)
	}

	// Quadrant captions occupy the first collision boxes. The old pass only
	// compared point labels with earlier point labels, letting them print over
	// headings such as "Strategic bets".
	placed := make([]placedLabel, 0, len(captionBands)+len(points))
	for _, band := range captionBands {
		if band.w > 0 && band.h > 0 {
			placed = append(placed, band)
		}
	}

	seriesIndex := map[string]int{}
	for _, point := range points {
		if _, seen := seriesIndex[point.Series]; point.Series != "" && !seen {
			seriesIndex[point.Series] = len(seriesIndex)
		}
	}

	// Markers first, labels second: a label is placed clear of every marker,
	// not only of the labels placed before it. Labelling point by point let
	// a label sit on the marker of a point that came later in the list
	// (go-slide-creator-995rf).
	type plotted struct {
		x, y, size float64
		label      string
	}
	marks := make([]plotted, 0, len(points))
	for _, point := range points {
		// A coordinate outside the axis range used to be plotted wherever the
		// scale put it: outside the plot frame, over the axis titles, or off the
		// canvas — silently (go-slide-creator-s27x). Clamp it back into the
		// frame and say so.
		point = mc.clampPointToAxes(point)
		x := xScale.Scale(point.X)
		y := yScale.Scale(point.Y)

		// Determine point properties
		size := point.Size
		if size == 0 {
			size = mc.config.PointSize
		}
		var bandStatus captionBandStatus
		y, bandStatus = reserveMatrixCaptionBand(x, y, size, plotArea, captionBands, style.Spacing.XS)
		if bandStatus != captionBandClear {
			message := fmt.Sprintf("matrix_2x2: point %q would cover a quadrant heading; its marker was moved just below the heading band — use a less extreme y value or a labelled quadrant list if exact coordinates are not required", point.Label)
			if bandStatus == captionBandNoRoom {
				message = fmt.Sprintf("matrix_2x2: point %q overlaps a quadrant heading and there is not enough room to move it within its quadrant — enlarge the diagram, shorten the heading, or use a labelled quadrant list", point.Label)
			}
			b.AddFinding(Finding{
				Code:     FindingDiagramTextOverlap,
				Message:  message,
				Severity: "warning",
				Fix:      &FixSuggestion{Kind: FixKindShortenLabels},
			})
		}

		// One ink for every point: colouring each by index suggested
		// categories the data does not have (go-slide-creator-njdno). Points
		// that name a series take one accent per series.
		color := palette.Accent1
		if point.Series != "" {
			color = palette.AccentColor(seriesIndex[point.Series])
		}
		if point.Color != nil {
			color = *point.Color
		}

		shape := mc.config.PointShape
		if point.Shape != MarkerNone {
			shape = point.Shape
		}

		// Draw the point
		mc.drawPoint(x, y, size, color, shape)
		marks = append(marks, plotted{x: x, y: y, size: size, label: point.Label})
		mc.markerBoxes = append(mc.markerBoxes, placedLabel{x: x - size/2, y: y - size/2, w: size, h: size})
	}
	defer func() { mc.markerBoxes = nil }()

	if !mc.config.ShowPointLabels {
		return
	}
	for _, m := range marks {
		if m.label == "" {
			continue
		}
		lbl := mc.drawPointLabelAvoiding(m.x, m.y, m.size, m.label, plotArea, placed, labelFontSize, labelOffset)
		placed = append(placed, lbl)
	}
}

type captionBandStatus uint8

const (
	captionBandClear captionBandStatus = iota
	captionBandMoved
	captionBandNoRoom
)

// reserveMatrixCaptionBand moves only markers that would cover a heading. The
// numeric scale is otherwise untouched; each adjustment is disclosed as a
// finding because its rendered position no longer exactly encodes the value.
func reserveMatrixCaptionBand(x, y, size float64, plot Rect, bands [4]placedLabel, pad float64) (float64, captionBandStatus) {
	q := 0
	if x >= plot.X+plot.W/2 {
		q++
	}
	if y >= plot.Y+plot.H/2 {
		q += 2
	}
	band := bands[q]
	if band.h <= 0 || x+size/2 < band.x-pad || x-size/2 > band.x+band.w+pad ||
		y+size/2 < band.y-pad || y-size/2 >= band.y+band.h+pad {
		return y, captionBandClear
	}
	target := band.y + band.h + pad + size/2
	quadrantBottom := plot.Y + plot.H
	if q < 2 {
		quadrantBottom = plot.Y + plot.H/2
	}
	if target+size/2+pad > quadrantBottom {
		return y, captionBandNoRoom
	}
	return target, captionBandMoved
}

// clampPointToAxes brings a point's coordinates back inside the configured axis
// range, reporting each coordinate it had to move.
func (mc *Matrix2x2Chart) clampPointToAxes(point Matrix2x2Point) Matrix2x2Point {
	clampAxis := func(v, min, max float64, axis string) float64 {
		// A reversed axis (x_min > x_max, as the BCG preset uses) is valid:
		// clamp to its numeric span, not to [min, max] literally, which
		// pinned every point to one edge (go-slide-creator-csclk.14).
		lo, hi := math.Min(min, max), math.Max(min, max)
		if v >= lo && v <= hi {
			return v
		}
		clamped := math.Min(math.Max(v, lo), hi)
		mc.builder.AddFinding(Finding{
			Code: FindingPointOutOfRange,
			Message: fmt.Sprintf(
				"matrix_2x2: point %q has %s=%g, outside the %g–%g axis range; it was clamped to %g so it stays inside the matrix — check the value or widen the axis",
				point.Label, axis, v, min, max, clamped),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind: FixKindReplaceValue,
				Params: map[string]any{
					"label":     point.Label,
					"axis":      axis,
					"value":     v,
					"clamped":   clamped,
					"axis_min":  min,
					"axis_max":  max,
					"diagram":   "matrix_2x2",
					"parameter": axis,
				},
			},
		})
		return clamped
	}
	point.X = clampAxis(point.X, mc.config.XAxisMin, mc.config.XAxisMax, "x")
	point.Y = clampAxis(point.Y, mc.config.YAxisMin, mc.config.YAxisMax, "y")
	return point
}

// drawPoint draws a single data point.
func (mc *Matrix2x2Chart) drawPoint(x, y, size float64, color Color, shape MarkerShape) {
	b := mc.builder
	style := b.StyleGuide()
	radius := size / 2

	b.Push()
	b.SetFillColor(color)
	b.SetStrokeColor(style.Palette.Background)
	b.SetStrokeWidth(style.Strokes.WidthNormal)

	switch shape {
	case MarkerCircle:
		b.DrawCircle(x, y, radius)

	case MarkerSquare:
		b.DrawRect(Rect{X: x - radius, Y: y - radius, W: size, H: size})

	case MarkerDiamond:
		pts := []Point{
			{X: x, Y: y - radius},
			{X: x + radius, Y: y},
			{X: x, Y: y + radius},
			{X: x - radius, Y: y},
		}
		b.DrawPolygon(pts)

	case MarkerTriangle:
		h := radius * math.Sqrt(3)
		pts := []Point{
			{X: x, Y: y - radius},
			{X: x + h/2, Y: y + radius/2},
			{X: x - h/2, Y: y + radius/2},
		}
		b.DrawPolygon(pts)

	default:
		b.DrawCircle(x, y, radius)
	}

	b.Pop()
}

// matrixLabelBoxSlack widens a point label's collision box past its measured
// width.
const matrixLabelBoxSlack = 1.08

// labelDirection describes a placement direction relative to a data point.
type labelDirection struct {
	dx, dy float64   // offset multipliers relative to the label offset distance
	align  TextAlign // text alignment for this direction
}

// drawPointLabelAvoiding draws a label for a data point with collision avoidance.
// It returns the bounding box of the placed label so subsequent labels can avoid it.
//
// The algorithm tries four placement directions (right, left, below, above) before
// falling back to vertical shifting. This distributes labels around their points
// and dramatically reduces overlap when many points are clustered together.
//
// fontSize and offset are pre-computed by drawPoints based on the total number of
// data points so that dense charts get smaller, tighter labels.
//
//nolint:gocognit,gocyclo // complex chart rendering logic
func (mc *Matrix2x2Chart) drawPointLabelAvoiding(x, y, pointSize float64, label string, plotArea Rect, placed []placedLabel, fontSize, offset float64) placedLabel {
	b := mc.builder
	style := b.StyleGuide()

	// Measure label dimensions using real font metrics for accurate collision
	// detection and bounds checking. The old heuristic (charCount * fontSize * 0.55)
	// underestimated wide labels, causing edge clipping.
	origSize := b.fontSize
	b.SetFontSize(fontSize)
	measuredW, _ := b.MeasureText(label)
	b.SetFontSize(origSize)
	labelW := measuredW
	labelH := fontSize * 1.3

	// LibreOffice renders SVG text ~15-20% wider than MeasureText predicts
	// (different font metrics / hinting). Use an inflated width for clipping
	// and wrapping decisions so labels near the SVG edge get wrapped instead
	// of being viewport-clipped mid-word (bug go-slide-creator-tfjmo).
	const libreOfficeTextInflation = 1.20
	clipW := labelW * libreOfficeTextInflation

	// Four candidate directions first: right, left, below, above.
	// Each direction moves the label anchor by (dx*offset, dy*offset) from the
	// point center, with an appropriate text alignment. When all four are
	// taken the four diagonals follow (appended after the reordering below).
	// Nothing further out is tried: a label two lines from its marker reads
	// as another point's.
	directions := []labelDirection{
		{dx: 1, dy: 0, align: TextAlignLeft},    // right of point
		{dx: -1, dy: 0, align: TextAlignRight},  // left of point
		{dx: 0, dy: 1, align: TextAlignCenter},  // below point
		{dx: 0, dy: -1, align: TextAlignCenter}, // above point
	}

	// Prefer to place the label away from the plot center so it doesn't
	// obscure the data area. Re-order directions accordingly.
	midX := plotArea.X + plotArea.W/2
	midY := plotArea.Y + plotArea.H/2
	if x > midX {
		// Point is in the right half — try right first (push outward)
		// default order is already right-first
	} else {
		// Point is in the left half — try left first
		directions[0], directions[1] = directions[1], directions[0]
	}
	if y > midY {
		// Point is in the bottom half — try below first, then above
		// default order is already below-first
	} else {
		// Point is in the top half — try above first
		directions[2], directions[3] = directions[3], directions[2]
	}

	// The diagonals sit a line above or below the marker, beside it. They
	// are tried only after the four sides.
	diag := labelH / math.Max(offset, 1)
	directions = append(directions,
		labelDirection{dx: 0.7, dy: -diag, align: TextAlignLeft},
		labelDirection{dx: 0.7, dy: diag, align: TextAlignLeft},
		labelDirection{dx: -0.7, dy: -diag, align: TextAlignRight},
		labelDirection{dx: -0.7, dy: diag, align: TextAlignRight},
	)

	// Helper: build a candidate bounding box for a given direction. The box
	// is a little wider than the measured text (boxW): a renderer whose face
	// runs wider than the measure drew two labels that just cleared one
	// another into each other.
	boxW := labelW * matrixLabelBoxSlack
	buildCandidate := func(dir labelDirection) (placedLabel, float64, TextAlign) {
		lx := x + dir.dx*offset
		ly := y + dir.dy*offset

		bx := lx
		switch dir.align {
		case TextAlignRight:
			bx = lx - boxW
		case TextAlignCenter:
			bx = lx - boxW/2
		}
		return placedLabel{x: bx, y: ly - labelH/2, w: boxW, h: labelH}, lx, dir.align
	}
	// A label stays inside the plot vertically as well: one placed above a
	// point near the top edge was drawn over the diagram's title.
	inPlotY := func(c placedLabel) bool {
		return c.y >= plotArea.Y && c.y+c.h <= plotArea.Y+plotArea.H
	}

	collidesWith := func(c placedLabel) bool {
		for _, p := range placed {
			if c.overlaps(p) {
				return true
			}
		}
		return false
	}
	// hitsMarker reports whether c covers another point's marker (the
	// label's own marker is the one centred on its point).
	hitsMarker := func(c placedLabel) bool {
		for _, m := range mc.markerBoxes {
			if math.Abs(m.x+m.w/2-x) < 0.01 && math.Abs(m.y+m.h/2-y) < 0.01 {
				continue
			}
			if c.overlaps(m) {
				return true
			}
		}
		return false
	}

	// Labels must stay inside the plot, not merely inside the SVG canvas:
	// axis titles and margins occupy the space around the plot.
	plotRight := plotArea.X + plotArea.W
	availableRun := func(lx float64, al TextAlign) float64 {
		var availW float64
		switch al {
		case TextAlignLeft:
			availW = plotRight - lx
		case TextAlignRight:
			availW = lx - plotArea.X
		case TextAlignCenter:
			availW = math.Min(lx-plotArea.X, plotRight-lx) * 2
		}
		return availW
	}
	wouldClip := func(lx float64, al TextAlign) bool {
		availW := availableRun(lx, al)
		return availW <= 0 || clipW > availW
	}

	// Try each direction without vertical shifting first.
	// Two-pass: prefer a collision-free direction with a full-width run, then
	// the widest available run for wrapping. This keeps labels inside the plot
	// and preserves complete names when the opposite side has room.
	var bestCandidate placedLabel
	var bestLabelX float64
	var bestAlign TextAlign
	found := false

	// Pass 1: clear of every label, heading and marker, and no clipping.
	// Pass 1b: the same, but over another point's marker: on a canvas too
	// small for its labels a whole name across a marker loses less than a
	// name cut short, and it is reported below.
	overMarker := false
	for _, strict := range []bool{true, false} {
		for _, dir := range directions {
			c, lx, al := buildCandidate(dir)
			if !collidesWith(c) && !wouldClip(lx, al) && inPlotY(c) && (!strict || !hitsMarker(c)) {
				bestCandidate = c
				bestLabelX = lx
				bestAlign = al
				found = true
				overMarker = !strict
				break
			}
		}
		if found {
			break
		}
	}

	// Pass 2: choose the widest positive run among collision-free options.
	// A candidate beyond the plot edge cannot be rescued by wrapping.
	if !found {
		bestRun := 0.0
		for _, dir := range directions {
			c, lx, al := buildCandidate(dir)
			if run := availableRun(lx, al); !collidesWith(c) && inPlotY(c) && run > bestRun {
				bestCandidate = c
				bestLabelX = lx
				bestAlign = al
				bestRun = run
				found = true
			}
		}
	}

	// No clean place: the label stays beside its point, where it covers
	// least, and the overlap is reported below. It used to be shifted up or
	// down until it was clear, up to six lines away: a label that far from
	// its marker names the wrong point.
	if !found {
		overlap := func(c placedLabel) float64 {
			area := func(o placedLabel) float64 {
				w := math.Min(c.x+c.w, o.x+o.w) - math.Max(c.x, o.x)
				h := math.Min(c.y+c.h, o.y+o.h) - math.Max(c.y, o.y)
				if w <= 0 || h <= 0 {
					return 0
				}
				return w * h
			}
			total := 0.0
			for _, o := range placed {
				total += area(o)
			}
			for _, m := range mc.markerBoxes {
				if math.Abs(m.x+m.w/2-x) < 0.01 && math.Abs(m.y+m.h/2-y) < 0.01 {
					continue
				}
				total += area(m)
			}
			return total
		}
		least := math.Inf(1)
		for _, dir := range directions {
			c, lx, al := buildCandidate(dir)
			if !inPlotY(c) || wouldClip(lx, al) {
				continue
			}
			if o := overlap(c); o < least {
				bestCandidate, bestLabelX, bestAlign, least = c, lx, al, o
				found = true
			}
		}
		if !found {
			// Nothing fits whole inside the plot: the widest run, wrapped.
			fallback := directions[0]
			for _, dir := range directions[1:] {
				_, lx, al := buildCandidate(dir)
				_, bestX, bestAl := buildCandidate(fallback)
				if availableRun(lx, al) > availableRun(bestX, bestAl) {
					fallback = dir
				}
			}
			bestCandidate, bestLabelX, bestAlign = buildCandidate(fallback)
			bestCandidate.y = math.Min(math.Max(bestCandidate.y, plotArea.Y), math.Max(plotArea.Y, plotArea.Y+plotArea.H-labelH))
		}
	}
	if overMarker {
		b.AddFinding(Finding{
			Code:     FindingDiagramTextOverlap,
			Message:  fmt.Sprintf("matrix_2x2: point label %q has no free place beside its point and is drawn across another point's marker — move the point, shorten the label, or enlarge the diagram", label),
			Severity: "warning",
			Fix:      &FixSuggestion{Kind: FixKindShortenLabels},
		})
	}
	if collidesWith(bestCandidate) {
		b.AddFinding(Finding{
			Code:     FindingDiagramTextOverlap,
			Message:  fmt.Sprintf("matrix_2x2: point label %q could not be placed clear of a quadrant heading or another label — move the point, shorten the label, or enlarge the diagram", label),
			Severity: "warning",
			Fix:      &FixSuggestion{Kind: FixKindShortenLabels},
		})
	}

	// Draw at the resolved position
	resolvedY := bestCandidate.y + labelH/2 // center Y of the label

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)

	// Wrap only when measured text exceeds its available run in the plot.
	availW := availableRun(bestLabelX, bestAlign)

	if availW > 0 && clipW > availW {
		// Wrap the label into a multi-line bounding box instead of truncating.
		// This preserves full item names in narrow two-column charts
		// (bug go-slide-creator-zci7z: matrix item labels truncated).
		// Uses clipW (inflated for LibreOffice) so labels near the edge wrap
		// proactively instead of being viewport-clipped (bug go-slide-creator-tfjmo).
		wrapH := labelH * 2 // allow up to 2 lines
		var wrapRect Rect
		var wrapAlign BoxAlign
		switch bestAlign {
		case TextAlignLeft:
			wrapRect = Rect{X: bestLabelX, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopLeft
		case TextAlignRight:
			wrapRect = Rect{X: bestLabelX - availW, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopRight
		default: // TextAlignCenter
			wrapRect = Rect{X: bestLabelX - availW/2, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopCenter
		}
		b.DrawWrappedText(label, wrapRect, wrapAlign)
		// A wrapped label occupies its actual run, not its old one-line width;
		// otherwise subsequent points are pushed away from empty space.
		bestCandidate.x = wrapRect.X
		bestCandidate.w = wrapRect.W
		bestCandidate.h = wrapH
	} else {
		b.DrawText(label, bestLabelX, resolvedY, bestAlign, TextBaselineMiddle)
	}

	b.Pop()

	return bestCandidate
}

// =============================================================================
// Matrix 2x2 Diagram Type (for Registry)
// =============================================================================

// Matrix2x2Diagram implements the Diagram interface for 2x2 matrix charts.
type Matrix2x2Diagram struct{ BaseDiagram }

// Validate checks that the request data is valid for matrix 2x2 charts.
func (d *Matrix2x2Diagram) Validate(req *RequestEnvelope) error {
	if req == nil || req.Data == nil {
		return fmt.Errorf("matrix_2x2 chart requires data. x/y use a 0-100 scale by default (origin bottom-left, quadrant split at 50). Expected format: {\"x_axis_label\": \"Impact\", \"y_axis_label\": \"Effort\", \"points\": [{\"label\": \"Task A\", \"x\": 80, \"y\": 60}]}")
	}

	if raw, exists := req.Data["quadrants"]; exists {
		quadrants, ok := raw.([]any)
		if !ok {
			return &ValidationError{Field: "data.quadrants", Code: ErrCodeInvalidType, Message: "quadrants must be an array", Value: raw}
		}
		for i, rawQuadrant := range quadrants {
			field := fmt.Sprintf("data.quadrants[%d]", i)
			quadrant, ok := rawQuadrant.(map[string]any)
			if !ok {
				return &ValidationError{Field: field, Code: ErrCodeInvalidType, Message: "quadrant must be an object", Value: rawQuadrant}
			}
			rawItems, exists := quadrant["items"]
			if !exists {
				continue
			}
			items, ok := rawItems.([]any)
			if !ok {
				return &ValidationError{Field: field + ".items", Code: ErrCodeInvalidType, Message: "items must be an array", Value: rawItems}
			}
			for j, item := range items {
				itemField := fmt.Sprintf("%s.items[%d]", field, j)
				switch value := item.(type) {
				case string:
				case map[string]any:
					if _, ok := value["label"].(string); !ok {
						return &ValidationError{Field: itemField + ".label", Code: ErrCodeInvalidType, Message: "item label must be a string", Value: value["label"]}
					}
				default:
					return &ValidationError{Field: itemField, Code: ErrCodeInvalidType, Message: "item must be a string or an object with a string label", Value: item}
				}
			}
		}
	}

	// Points are optional - can show empty matrix
	return nil
}

// Render generates an SVG document from the request envelope.
func (d *Matrix2x2Diagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
// This allows callers to generate PNG/PDF output from the same builder.
//
//nolint:gocognit,gocyclo // complex chart rendering logic
func (d *Matrix2x2Diagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	if err := d.Validate(req); err != nil {
		return nil, nil, err
	}
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, findings, err := parseMatrix2x2Data(req)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			builder.AddFinding(finding)
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultMatrix2x2Config(width, height)
		config.ShowPointLabels = true

		// One tone for all four quadrants — four different accents read as a
		// patchwork and bury which quadrant matters (go-slide-creator-njdno) —
		// and that tone is the accent's own light swatch, the fields held
		// apart by a gutter instead of two crossing lines; a flat grey slab
		// with a hairline cross was a wireframe (go-slide-creator-ckpye). The
		// accent proper is reserved for the quadrant the author highlights.
		style := builder.StyleGuide()
		field, deep := matrixFieldColors(style.Palette)
		for i := range config.QuadrantColors {
			config.QuadrantColors[i] = field
		}
		config.ShowGridLines = false
		config.Gutter = style.Spacing.SM
		config.AxisBars = true

		// Apply custom axis labels (support multiple key formats)
		if xLabel, ok := req.Data["x_axis_label"].(string); ok {
			config.XAxisLabel = xLabel
		} else if xLabel, ok := req.Data["x_label"].(string); ok {
			config.XAxisLabel = xLabel
		} else if xAxis, ok := req.Data["x_axis"].(map[string]any); ok {
			if label, ok := xAxis["label"].(string); ok {
				config.XAxisLabel = label
			}
		}
		if yLabel, ok := req.Data["y_axis_label"].(string); ok {
			config.YAxisLabel = yLabel
		} else if yLabel, ok := req.Data["y_label"].(string); ok {
			config.YAxisLabel = yLabel
		} else if yAxis, ok := req.Data["y_axis"].(map[string]any); ok {
			if label, ok := yAxis["label"].(string); ok {
				config.YAxisLabel = label
			}
		}
		config.QuadrantLabels = defaultMatrixQuadrantLabels(config.XAxisLabel, config.YAxisLabel)

		// Apply custom quadrant labels from quadrant_labels array
		if labels, ok := req.Data["quadrant_labels"].([]any); ok {
			for i, l := range labels {
				if i < 4 {
					if label, ok := l.(string); ok {
						config.QuadrantLabels[i] = label
					}
				}
			}
		}

		// Apply quadrant labels from quadrants array (title or label field)
		if quadrants, ok := req.Data["quadrants"].([]any); ok {
			for i, q := range quadrants {
				qMap, ok := q.(map[string]any)
				if !ok {
					continue
				}
				idx, _ := resolvedQuadrantIndex(qMap, i)
				// Prefer "title", fall back to "label"
				if title, ok := qMap["title"].(string); ok {
					config.QuadrantLabels[idx] = title
				} else if label, ok := qMap["label"].(string); ok {
					config.QuadrantLabels[idx] = label
				}
			}
		}

		// The highlighted quadrant (by index, position or label) is the one
		// solid accent field when the quadrants hold lists; under plotted
		// points, which are drawn in the accent themselves, it takes the
		// deeper swatch instead.
		if hi, ok := matrixHighlightQuadrant(req.Data, config.QuadrantLabels); ok {
			config.QuadrantColors[hi] = deep
			if data.HasQuadrantItems() {
				config.QuadrantColors[hi] = style.Palette.Accent1.Opaque()
			}
		}

		// Apply custom quadrant colors
		if colors, ok := req.Data["quadrant_colors"].([]any); ok {
			for i, c := range colors {
				if i < 4 {
					if colorStr, ok := c.(string); ok {
						if color, err := ParseColor(colorStr); err == nil {
							config.QuadrantColors[i] = color.WithAlpha(config.QuadrantOpacity)
						}
					}
				}
			}
		}

		// Apply custom quadrant opacity (to authored quadrant_colors; the
		// neutral default and the highlight keep their calibrated tints).
		if opacity, ok := req.Data["quadrant_opacity"].(float64); ok && req.Data["quadrant_colors"] != nil {
			config.QuadrantOpacity = opacity
			// Re-apply opacity to existing colors
			for i := range config.QuadrantColors {
				config.QuadrantColors[i] = config.QuadrantColors[i].WithAlpha(opacity)
			}
		}

		// Apply axis ranges
		xRangeSet := false
		yRangeSet := false
		if xMin, ok := req.Data["x_min"].(float64); ok {
			config.XAxisMin = xMin
			xRangeSet = true
		}
		if xMax, ok := req.Data["x_max"].(float64); ok {
			config.XAxisMax = xMax
			xRangeSet = true
		}
		if yMin, ok := req.Data["y_min"].(float64); ok {
			config.YAxisMin = yMin
			yRangeSet = true
		}
		if yMax, ok := req.Data["y_max"].(float64); ok {
			config.YAxisMax = yMax
			yRangeSet = true
		}

		// Rescale normalized 0-1 coordinates to the axis range. Agents sometimes
		// supply x/y in [0,1] instead of the documented 0-100 scale; without
		// rescaling every point collapses into the bottom-left quadrant
		// (go-slide-creator-pc2a). Only applies when axis ranges are left at
		// their defaults so explicit ranges are always respected.
		if !xRangeSet && !yRangeSet {
			maybeScaleNormalizedPoints(data.Points, config.XAxisMin, config.XAxisMax, config.YAxisMin, config.YAxisMax)
		}

		chart := NewMatrix2x2Chart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// Quadrant fields. A field is the accent's "Lighter 80%" swatch and the
// deeper field its "Lighter 50%" (Color.Tint, the pattern engine's tonal
// system). Where that swatch cannot be told from the background or from the
// solid accent (a yellow accent), both come from the neutral ink instead.
const (
	matrixFieldKeep       = 0.2
	matrixFieldDeepKeep   = 0.5
	matrixFieldPaperMin   = 1.12
	matrixFieldAccentMin  = 1.6
	matrixNeutralField    = 0.10
	matrixNeutralDeepFill = 0.24
)

// matrixFieldColors returns the fill of a quadrant field and of the deeper
// field a highlighted quadrant takes under plotted points.
func matrixFieldColors(p *Palette) (field, deep Color) {
	bg := p.Background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	accent := p.Accent1.BlendOver(bg)
	field = accent.Tint(matrixFieldKeep)
	if field.ContrastWith(bg) < matrixFieldPaperMin || accent.ContrastWith(field) < matrixFieldAccentMin {
		return NeutralInk(p, matrixNeutralField), NeutralInk(p, matrixNeutralDeepFill)
	}
	return field, accent.Tint(matrixFieldDeepKeep)
}

// matrixHighlightQuadrant resolves data.highlight_quadrant — an index 0-3
// (top-left, top-right, bottom-left, bottom-right), a position such as
// "top-left" / "top_right", or a quadrant's label — or a quadrants[] entry
// carrying highlight: true.
func matrixHighlightQuadrant(data map[string]any, labels [4]string) (int, bool) {
	switch v := data["highlight_quadrant"].(type) {
	case float64:
		if i := int(v); float64(i) == v && i >= 0 && i < 4 {
			return i, true
		}
	case int:
		if v >= 0 && v < 4 {
			return v, true
		}
	case string:
		s := strings.TrimSpace(v)
		pos := strings.NewReplacer("_", "-", " ", "-").Replace(strings.ToLower(s))
		if i := quadrantPositionIndex(pos); i >= 0 {
			return i, true
		}
		for i, label := range labels {
			if label != "" && strings.EqualFold(label, s) {
				return i, true
			}
		}
	}
	if quadrants, ok := data["quadrants"].([]any); ok {
		for i, q := range quadrants {
			qMap, ok := q.(map[string]any)
			if !ok {
				continue
			}
			if hi, _ := qMap["highlight"].(bool); hi {
				idx, _ := resolvedQuadrantIndex(qMap, i)
				return idx, true
			}
		}
	}
	return 0, false
}

// DataSchema returns the data contract for matrix_2x2 diagrams: plotted
// points, or coordinate-free quadrant lists.
func (d *Matrix2x2Diagram) DataSchema() *DataSchema {
	axis := ObjectDataSchema("Axis", map[string]*DataSchema{"label": StringDataSchema("Axis title")}, nil)
	point := ObjectDataSchema("A plotted point", map[string]*DataSchema{
		"label":       StringDataSchema("Point label"),
		"x":           NumberDataSchema("X position (0-100 by default)"),
		"y":           NumberDataSchema("Y position (0-100 by default)"),
		"size":        NumberDataSchema("Bubble size"),
		"color":       StringDataSchema("Hex color override"),
		"description": StringDataSchema("Point description"),
		"series":      StringDataSchema("Series / group name (aliases: group, category)"),
		"group":       StringDataSchema("Alias for series"),
		"category":    StringDataSchema("Alias for series"),
	}, nil)
	quadrant := ObjectDataSchema("A quadrant with its item list", map[string]*DataSchema{
		"position":  StringDataSchema("Quadrant: top-left, top-right, bottom-left or bottom-right (underscores accepted); omit on every quadrant to place them in that order"),
		"title":     StringDataSchema("Quadrant caption (alias: label)"),
		"label":     StringDataSchema("Alias for title"),
		"items":     ArrayDataSchema("Items listed in the quadrant: strings or {label}", ObjectDataSchema("An item, or a plain string", map[string]*DataSchema{"label": StringDataSchema("Item text")}, nil), 0),
		"highlight": BooleanDataSchema("Accent this quadrant"),
	}, nil)
	return diagramDataSchema("2x2 matrix with plotted points or quadrant lists", map[string]*DataSchema{
		"points":             ArrayDataSchema("Plotted points", point, 0),
		"quadrants":          ArrayDataSchema("Quadrant captions and item lists", quadrant, 0),
		"x_axis_label":       StringDataSchema("X axis title (aliases: x_label, x_axis.label)"),
		"x_label":            StringDataSchema("Alias for x_axis_label"),
		"x_axis":             axis,
		"y_axis_label":       StringDataSchema("Y axis title (aliases: y_label, y_axis.label)"),
		"y_label":            StringDataSchema("Alias for y_axis_label"),
		"y_axis":             axis,
		"quadrant_labels":    stringListSchema("Quadrant captions: top-left, top-right, bottom-left, bottom-right"),
		"quadrant_colors":    stringListSchema("Quadrant fill colors"),
		"quadrant_opacity":   NumberDataSchema("Quadrant fill opacity (with quadrant_colors)"),
		"highlight_quadrant": &DataSchema{Description: "Quadrant to accent: index 0-3, position, or caption"},
		"x_min":              NumberDataSchema("X axis minimum"),
		"x_max":              NumberDataSchema("X axis maximum"),
		"y_min":              NumberDataSchema("Y axis minimum"),
		"y_max":              NumberDataSchema("Y axis maximum"),
		"footnote":           StringDataSchema("Footnote text"),
	}, nil)
}

// parseMatrix2x2Data parses the request data into Matrix2x2Data.
//
//nolint:gocognit,gocyclo // complex chart rendering logic
func parseMatrix2x2Data(req *RequestEnvelope) (Matrix2x2Data, []Finding, error) {
	data := Matrix2x2Data{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}
	var findings []Finding

	// Parse points
	if pointsRaw, ok := req.Data["points"].([]any); ok {
		data.Points = make([]Matrix2x2Point, 0, len(pointsRaw))

		for _, pRaw := range pointsRaw {
			point := Matrix2x2Point{}

			if p, ok := pRaw.(map[string]any); ok {
				if label, ok := p["label"].(string); ok {
					point.Label = label
				}
				if x, ok := p["x"].(float64); ok {
					point.X = x
				} else if x, ok := p["x"].(int); ok {
					point.X = float64(x)
				}
				if y, ok := p["y"].(float64); ok {
					point.Y = y
				} else if y, ok := p["y"].(int); ok {
					point.Y = float64(y)
				}
				if size, ok := p["size"].(float64); ok {
					point.Size = size
				} else if size, ok := p["size"].(int); ok {
					point.Size = float64(size)
				}
				if colorStr, ok := p["color"].(string); ok {
					if c, err := ParseColor(colorStr); err == nil {
						point.Color = &c
					}
				}
				if desc, ok := p["description"].(string); ok {
					point.Description = desc
				}
				for _, key := range []string{"series", "group", "category"} {
					if s, ok := p[key].(string); ok && strings.TrimSpace(s) != "" {
						point.Series = strings.TrimSpace(s)
						break
					}
				}
			}

			data.Points = append(data.Points, point)
		}
	}

	// Parse quadrants format (alternative to points). This form carries no
	// coordinates, so it is kept as lists rather than converted into invented
	// scatter positions (go-slide-creator-s27x).
	if quadrants, ok := req.Data["quadrants"].([]any); ok && len(data.Points) == 0 {
		data.QuadrantItems, findings = parseQuadrantItemLists(quadrants)
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, findings, nil
}

// maybeScaleNormalizedPoints detects matrix points provided on a normalized
// 0-1 scale and rescales them to the configured axis range. Agents sometimes
// supply x/y in [0,1] instead of the documented 0-100 scale; without rescaling
// they all collapse into the bottom-left quadrant (go-slide-creator-pc2a).
//
// To avoid mis-firing it only rescales when (a) the axis spans a much larger
// range than [0,1], (b) every coordinate is within [0,1], and (c) at least one
// coordinate is strictly positive (so genuinely all-zero data isn't "scaled").
func maybeScaleNormalizedPoints(points []Matrix2x2Point, xMin, xMax, yMin, yMax float64) {
	if len(points) == 0 {
		return
	}
	xSpan := xMax - xMin
	ySpan := yMax - yMin
	// If the axis is itself a small range (already 0-1-ish), the data is
	// presumably already in the right units — leave it alone.
	if math.Abs(xSpan) <= 2 || math.Abs(ySpan) <= 2 {
		return
	}
	anyPositive := false
	for _, p := range points {
		if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return // a point outside [0,1] → not normalized data
		}
		if p.X > 0 || p.Y > 0 {
			anyPositive = true
		}
	}
	if !anyPositive {
		return
	}
	for i := range points {
		points[i].X = xMin + points[i].X*xSpan
		points[i].Y = yMin + points[i].Y*ySpan
	}
}

// quadrantPositionIndex returns the index (0-3) for a normalized quadrant position string.
// Returns -1 if the position is not recognized.
// Order: 0=top-left, 1=top-right, 2=bottom-left, 3=bottom-right.
func quadrantPositionIndex(position string) int {
	switch position {
	case "top-left":
		return 0
	case "top-right":
		return 1
	case "bottom-left":
		return 2
	case "bottom-right":
		return 3
	default:
		return -1
	}
}

// parseQuadrantItemLists reads the coordinate-free quadrant form into per
// quadrant item lists, indexed the same way as Matrix2x2Config.QuadrantLabels.
//
// It replaces parseQuadrantItems, which invented an (x, y) for every item so the
// scatter renderer could draw it. Those positions were guesses — a fixed
// quadrant centre plus a small spread — and with two items per quadrant their
// labels already collided with each other and with the quadrant caption
// (go-slide-creator-s27x).
//
// A list in which no quadrant names a position is read in list order —
// top-left, top-right, bottom-left, bottom-right, the order of
// quadrant_labels — and that is a way to author the matrix, not something
// to report: four notes on a well-formed 2x2 cost the slide twenty points
// (go-slide-creator-t3k06). A position that is given and not one of the four
// is reported, and so is a missing one in a list where others are given,
// because list order can then land on a quadrant another entry named.
func parseQuadrantItemLists(quadrants []any) ([4][]string, []Finding) {
	var out [4][]string
	var findings []Finding
	positioned := false
	for _, q := range quadrants {
		if qMap, ok := q.(map[string]any); ok && quadrantStatesPosition(qMap) {
			positioned = true
		}
	}
	for qi, q := range quadrants {
		qMap, ok := q.(map[string]any)
		if !ok {
			continue
		}
		idx, defaulted := resolvedQuadrantIndex(qMap, qi)
		if defaulted && positioned {
			field := fmt.Sprintf("data.quadrants[%d].position", qi)
			cause := "has no position while other quadrants name one"
			if quadrantStatesPosition(qMap) {
				cause = fmt.Sprintf("position %q is not top-left, top-right, bottom-left or bottom-right", fmt.Sprint(qMap["position"]))
			}
			findings = append(findings, Finding{
				Field:    field,
				Code:     FindingQuadrantPositionDefaulted,
				Message:  fmt.Sprintf("matrix_2x2: quadrant %d %s; placed at %s by list order", qi+1, cause, matrixQuadrantPositions[idx]),
				Severity: "warning",
				Fix: &FixSuggestion{Kind: FixKindReplaceValue, Params: map[string]any{
					"field": field, "value": matrixQuadrantPositions[idx],
				}},
			})
		}
		items, ok := qMap["items"].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			var label string
			switch v := item.(type) {
			case string:
				label = v
			case map[string]any:
				label, _ = v["label"].(string)
			}
			if label = strings.TrimSpace(label); label != "" {
				out[idx] = append(out[idx], label)
			}
		}
	}
	return out, findings
}

var matrixQuadrantPositions = [4]string{"top-left", "top-right", "bottom-left", "bottom-right"}

// quadrantStatesPosition reports whether a quadrant gives a position at all:
// a value that is not null and not a blank string.
func quadrantStatesPosition(quadrant map[string]any) bool {
	switch v := quadrant["position"].(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	}
	return true
}

func resolvedQuadrantIndex(quadrant map[string]any, listIndex int) (index int, defaulted bool) {
	position, _ := quadrant["position"].(string)
	index = quadrantPositionIndex(strings.ReplaceAll(strings.TrimSpace(position), "_", "-"))
	if index >= 0 {
		return index, false
	}
	return listIndex % len(matrixQuadrantPositions), true
}

// =============================================================================
// Convenience Functions
// =============================================================================

// DrawMatrix2x2FromData creates and draws a matrix 2x2 chart from simple data.
func DrawMatrix2x2FromData(builder *SVGBuilder, title string, points []Matrix2x2Point) error {
	config := DefaultMatrix2x2Config(builder.Width(), builder.Height())
	config.ShowPointLabels = true
	config.ShowGridLines = true

	chart := NewMatrix2x2Chart(builder, config)
	return chart.Draw(Matrix2x2Data{
		Title:  title,
		Points: points,
	})
}

// CreateBCGMatrixConfig returns a config preset for BCG Matrix (Growth-Share Matrix).
func CreateBCGMatrixConfig(width, height float64) Matrix2x2Config {
	config := DefaultMatrix2x2Config(width, height)
	config.XAxisLabel = "Relative Market Share"
	config.YAxisLabel = "Market Growth Rate"
	config.QuadrantLabels = [4]string{
		"Stars",          // high growth, high share
		"Question Marks", // high growth, low share
		"Cash Cows",      // low growth, high share
		"Dogs",           // low growth, low share
	}
	// For BCG, x-axis is reversed (high share on left)
	config.XAxisMin = 100
	config.XAxisMax = 0
	return config
}

// CreateEisenhowerMatrixConfig returns a config preset for Eisenhower Matrix (Urgent-Important).
func CreateEisenhowerMatrixConfig(width, height float64) Matrix2x2Config {
	config := DefaultMatrix2x2Config(width, height)
	config.XAxisLabel = "Urgency"
	config.YAxisLabel = "Importance"
	config.QuadrantLabels = [4]string{
		"Do First",  // important, not urgent
		"Schedule",  // important, urgent
		"Delegate",  // not important, not urgent
		"Eliminate", // not important, urgent
	}
	return config
}
