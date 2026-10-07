package svggen

import (
	"fmt"
	"math"
)

// Horizontal bars are the textbook form for a ranked item comparison with
// long category names (Zelazny): the names read left-to-right in a column
// instead of wrapping under narrow columns. data.orientation: "horizontal"
// on bar_chart / grouped_bar_chart / stacked_bar_chart selects this layout
// (go-slide-creator-oocqj).
const (
	// OrientationHorizontal is the data.orientation value for horizontal bars.
	OrientationHorizontal = "horizontal"
	// OrientationVertical is the default data.orientation.
	OrientationVertical = "vertical"

	// hbarLabelColumnMaxShare caps the category-name column at this share of
	// the canvas width; longer names wrap to two lines, then truncate.
	hbarLabelColumnMaxShare = 0.38
	// hbarLabelGapPt separates a category name from its bar.
	hbarLabelGapPt = 8.0
	// hbarNegValueSepPt widens the gap between a category name and a
	// negative bar's value label beside it, so the two never read as one
	// string when the renderer's font runs wider than the measured one.
	hbarNegValueSepPt = 4.0
	// hbarSlotShare is a bar's thickness as a share of its category slot.
	hbarSlotShare = 0.62
	// HorizontalBarLabelMinChars is the longest category name, in
	// characters, a vertical bar chart carries comfortably; past it the
	// capabilities recommend data.orientation "horizontal".
	HorizontalBarLabelMinChars = 14
)

// resolveOrientation reads data.orientation.
func resolveOrientation(data map[string]any) (bool, error) {
	raw, ok := data["orientation"]
	if !ok || raw == nil {
		return false, nil
	}
	s, isString := raw.(string)
	switch {
	case isString && (s == OrientationHorizontal):
		return true, nil
	case isString && (s == OrientationVertical || s == ""):
		return false, nil
	}
	return false, &ValidationError{
		Field:   "data.orientation",
		Code:    ErrCodeInvalidValue,
		Message: fmt.Sprintf("data.orientation must be \"vertical\" or \"horizontal\", got %v", raw),
		Value:   raw,
	}
}

// hbarLayout is the geometry of one horizontal bar render.
type hbarLayout struct {
	plot        Rect
	header      float64
	footer      float64
	legend      float64
	axisH       float64
	labelFont   float64
	valueFont   float64
	labelColumn float64
	// negValueColumn is the band between the category names and the plot
	// that holds the value labels of bars ending at the plot's left edge.
	negValueColumn float64
	displayNames   [][]string
}

// drawHorizontal renders the chart with categories down the left and bars
// growing to the right. With value labels on (the default for a short
// chart) there is no value axis or gridline — each bar carries its number
// and a thin baseline marks zero; without them a bottom value axis and
// light vertical gridlines are drawn.
func (bc *BarChart) drawHorizontal(data ChartData) error {
	if bc.config.Scale == "log" {
		return fmt.Errorf("log scale is not supported for horizontal bars")
	}
	b := bc.builder
	style := b.StyleGuide()
	colors := seriesHighlightColors(style.Palette, bc.getColors(style, len(data.Series)), data)
	b.CheckChartCapacity(len(data.Series), len(data.Categories))
	bc.config.ResolveValueFormatter(chartDataValues(data), true)
	// A zoomed axis (authored y_min above zero) keeps its value axis so the
	// truncated bars do not read as true lengths (go-slide-creator-929jm).
	labelled := bc.config.ShowValues && !data.Axis.zoomed()

	lo, hi := bc.calculateDomain(data)
	bc.reportAxisNotZero(data)
	lay := bc.horizontalLayout(data, labelled)
	plot := lay.plot

	vScale := NewLinearScale(lo, hi)
	vScale.SetRangeLinear(0, plot.W)
	if !labelled {
		vScale.Nice(true)
	}
	cScale := NewCategoricalScale(data.Categories)
	cScale.SetRangeCategorical(plot.Y, plot.Y+plot.H)
	pad := math.Max(0, (1-hbarSlotShare)/(1+hbarSlotShare))
	cScale.PaddingInner(pad)
	cScale.PaddingOuter(pad / 2)

	zeroX := plot.X + vScale.Scale(math.Max(lo, math.Min(0, hi)))
	if !labelled {
		bc.drawHorizontalValueAxis(plot, vScale)
	}

	bc.drawHorizontalCategoryNames(data, cScale, lay, zeroX)
	if bc.config.Stacked {
		bc.drawHorizontalStacks(data, plot, cScale, vScale, colors, labelled, lay.valueFont)
	} else {
		bc.drawHorizontalBars(data, plot, cScale, vScale, colors, labelled, lay.valueFont)
	}

	// Zero baseline over the bars' roots.
	b.Push()
	b.SetStrokeColor(style.Palette.TextPrimary).SetStrokeWidth(labelledBaselinePt)
	b.DrawLine(zeroX, plot.Y, zeroX, plot.Y+plot.H)
	b.Pop()

	drawChartHeader(b, bc.config.Width, bc.config.ShowTitle, data.Title, data.Subtitle)
	if lay.legend > 0 {
		bc.drawHorizontalLegend(data, colors, Rect{
			X: plot.X, Y: plot.Y + plot.H + lay.axisH + style.Spacing.SM, W: plot.W, H: lay.legend,
		})
	}
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		NewFootnote(b, footnoteConfig).Draw(Rect{X: 0, Y: bc.config.Height - lay.footer, W: bc.config.Width, H: lay.footer})
	}
	return nil
}

// horizontalLayout measures the category-name column, the value-label
// reserve and the header / legend / footer bands, and returns the plot rect.
func (bc *BarChart) horizontalLayout(data ChartData, labelled bool) hbarLayout {
	b := bc.builder
	style := b.StyleGuide()
	lay := hbarLayout{labelFont: style.Typography.SizeSmall, valueFont: style.Typography.SizeSmall}
	if labelled {
		lay.valueFont = labelledValueFont(style)
	}
	lay.header = chartHeaderHeight(style, bc.config.ShowTitle, data.Title, data.Subtitle)
	if data.Footnote != "" {
		lay.footer = FootnoteReservedHeight(style)
	}
	if !labelled {
		lay.axisH = style.Typography.SizeSmall*xLabelGlyphEm + 8
	}

	width := bc.config.Width - bc.config.MarginLeft - bc.config.MarginRight
	// Category names: as wide as the widest name, capped, wrapping to two
	// lines past the cap.
	b.Push()
	b.SetFontSize(lay.labelFont)
	maxCol := bc.config.Width * hbarLabelColumnMaxShare
	col := 0.0
	lay.displayNames = make([][]string, len(data.Categories))
	for i, c := range data.Categories {
		lines := []string{c}
		if w, _ := b.MeasureText(c); w > maxCol {
			lines = twoLineLabel(b, c, maxCol)
		}
		lay.displayNames[i] = lines
		for _, l := range lines {
			if w, _ := b.MeasureText(l); w > col {
				col = w
			}
		}
	}
	// Value labels sit past the bar end: reserve the widest label on each
	// side. Only a non-stacked negative bar labels leftward; a stack's total
	// always sits past its positive end.
	rightReserve, leftReserve := 0.0, 0.0
	if labelled {
		b.SetFontSize(lay.valueFont).SetFontWeight(style.Typography.WeightBold)
		for _, v := range hbarLabelValues(data, bc.config.Stacked) {
			w, _ := b.MeasureText(TrueMinus(bc.config.ValueFmt.FormatOr(v, bc.config.ValueFormat)))
			if v < 0 && !bc.config.Stacked {
				leftReserve = math.Max(leftReserve, w+labelledValueGapPt)
			} else {
				rightReserve = math.Max(rightReserve, w+labelledValueGapPt)
			}
		}
		if leftReserve > 0 {
			leftReserve += hbarNegValueSepPt
		}
	}
	b.Pop()
	lay.labelColumn = math.Min(col, maxCol)

	legendItems := 0
	if len(data.Series) > 1 || bc.config.ForceLegendSingleSeries {
		legendItems = len(data.Series)
	}
	if legendItems > 0 {
		legendConfig := PresentationLegendConfig(style)
		legend := NewLegend(b, legendConfig)
		items := make([]LegendItem, len(data.Series))
		for i, s := range data.Series {
			items[i] = LegendItem{Label: s.Name}
		}
		legend.SetItems(items)
		lay.legend = legend.Height(width)
	}

	// The most negative bar ends at the plot's left edge and its label sits
	// left of that, so the label gets its own column between the category
	// names and the plot; anchoring the names to the plot edge put the two
	// in the same slot ("North" over "−0.5", go-slide-creator-yiznx).
	lay.negValueColumn = leftReserve
	left := bc.config.MarginLeft + lay.labelColumn + hbarLabelGapPt + leftReserve
	right := bc.config.MarginRight + rightReserve
	top := bc.config.MarginTop + lay.header
	bottom := bc.config.MarginBottom + lay.footer + lay.axisH
	if lay.legend > 0 {
		bottom += lay.legend + style.Spacing.SM
	}
	lay.plot = Rect{
		X: left,
		Y: top,
		W: math.Max(0, bc.config.Width-left-right),
		H: math.Max(0, bc.config.Height-top-bottom),
	}
	return lay
}

// hbarLabelValues are the numbers the value labels print: each bar, or the
// total of each stack with a positive part (drawHorizontalStacks labels no
// other stack).
func hbarLabelValues(data ChartData, stacked bool) []float64 {
	if !stacked {
		return chartDataValues(data)
	}
	total := make([]float64, len(data.Categories))
	positive := make([]bool, len(data.Categories))
	for _, s := range data.Series {
		for i, v := range s.Values {
			if i < len(total) {
				total[i] += v
				positive[i] = positive[i] || v > 0
			}
		}
	}
	out := make([]float64, 0, len(total))
	for i, t := range total {
		if positive[i] {
			out = append(out, t)
		}
	}
	return out
}

// twoLineLabel breaks a long category name at the word boundary nearest its
// middle; a line still wider than maxWidth is truncated with an ellipsis.
func twoLineLabel(b *SVGBuilder, label string, maxWidth float64) []string {
	block := b.WrapText(label, maxWidth)
	if len(block.Lines) <= 1 {
		return []string{b.TruncateToWidth(label, maxWidth)}
	}
	first := block.Lines[0].Text
	rest := label[min(len(label), len(first)):]
	for len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	return []string{first, b.TruncateToWidth(rest, maxWidth)}
}

// drawHorizontalCategoryNames writes each category name right-aligned in the
// column left of the bars, vertically centred on its slot.
func (bc *BarChart) drawHorizontalCategoryNames(data ChartData, cScale *CategoricalScale, lay hbarLayout, zeroX float64) {
	b := bc.builder
	style := b.StyleGuide()
	b.Push()
	b.SetFontSize(lay.labelFont).SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextPrimary)
	x := math.Min(lay.plot.X, zeroX) - lay.negValueColumn - hbarLabelGapPt
	lineH := lay.labelFont * 1.2
	for i, c := range data.Categories {
		lines := lay.displayNames[i]
		cy := cScale.Scale(c)
		y0 := cy - lineH*float64(len(lines)-1)/2
		for j, l := range lines {
			b.DrawText(l, x, y0+float64(j)*lineH, TextAlignRight, TextBaselineMiddle)
		}
	}
	b.Pop()
}

// drawHorizontalValueAxis draws the bottom value axis and light vertical
// gridlines of an unlabelled horizontal chart.
func (bc *BarChart) drawHorizontalValueAxis(plot Rect, vScale *LinearScale) {
	b := bc.builder
	if bc.config.ShowGrid {
		gridConfig := DefaultGridConfig()
		b.Push()
		b.SetStrokeColor(gridConfig.Color).SetStrokeWidth(gridConfig.StrokeWidth)
		b.SetDashes(gridConfig.DashPattern...)
		for _, v := range vScale.Ticks(5) {
			x := plot.X + vScale.Scale(v)
			b.DrawLine(x, plot.Y, x, plot.Y+plot.H)
		}
		b.Pop()
	}
	if !bc.config.ShowAxes {
		return
	}
	axisConfig := DefaultAxisConfig(AxisPositionBottom)
	axisConfig.TickCount = 5
	axisConfig.ValueFmt = bc.config.ValueFmt
	axisConfig.Title = bc.config.YAxisTitle
	axisConfig.HideAxisLine = true
	NewAxis(b, axisConfig).DrawLinearAxis(vScale, plot.X, plot.Y+plot.H)
}

// drawHorizontalBars draws side-by-side (grouped) bars, one band per series
// inside each category slot, with the value label past each bar's end.
func (bc *BarChart) drawHorizontalBars(data ChartData, plot Rect, cScale *CategoricalScale, vScale *LinearScale, colors []Color, labelled bool, valueFont float64) {
	b := bc.builder
	style := b.StyleGuide()
	pointColors, pointBold := bc.highlightFills(data, colors)
	n := len(data.Series)
	band := cScale.Bandwidth() / float64(n)
	for si, s := range data.Series {
		fill := colors[si%len(colors)]
		if s.Color != nil {
			fill = *s.Color
		}
		for i, c := range data.Categories {
			if i >= len(s.Values) {
				continue
			}
			v := s.Values[i]
			barFill := fill
			if len(pointColors) == len(s.Values) {
				barFill = pointColors[i]
			}
			y := cScale.ScaleStart(c) + band*float64(si)
			x0, x1 := plot.X+vScale.Scale(0), plot.X+vScale.Scale(v)
			b.Push()
			b.SetFillColor(barFill).SetStrokeWidth(0)
			b.FillRect(Rect{X: math.Min(x0, x1), Y: y, W: math.Abs(x1 - x0), H: band * 0.92})
			b.Pop()
			if !labelled {
				continue
			}
			bold := len(pointBold) == len(s.Values) && pointBold[i]
			bc.drawHorizontalValueLabel(v, x1, y+band*0.46, bold, v < 0, valueFont, style)
		}
	}
}

// drawHorizontalStacks draws one bar per category of stacked segments
// (positives rightward, negatives leftward) with the stack total past its end.
func (bc *BarChart) drawHorizontalStacks(data ChartData, plot Rect, cScale *CategoricalScale, vScale *LinearScale, colors []Color, labelled bool, valueFont float64) {
	b := bc.builder
	style := b.StyleGuide()
	band := cScale.Bandwidth()
	for i, c := range data.Categories {
		pos, neg := 0.0, 0.0
		y := cScale.ScaleStart(c)
		for si, s := range data.Series {
			if i >= len(s.Values) {
				continue
			}
			v := s.Values[i]
			start := pos
			if v < 0 {
				start = neg
				neg += v
			} else {
				pos += v
			}
			fill := colors[si%len(colors)]
			if s.Color != nil {
				fill = *s.Color
			}
			x0, x1 := plot.X+vScale.Scale(start), plot.X+vScale.Scale(start+v)
			seg := Rect{X: math.Min(x0, x1), Y: y, W: math.Abs(x1 - x0), H: band}
			b.Push()
			b.SetFillColor(fill).SetStrokeWidth(0)
			b.FillRect(seg)
			b.Pop()
			if labelled {
				drawStackSegmentLabel(b, style, bc.config.ChartConfig, v, seg, fill)
			}
		}
		if labelled && pos > 0 {
			bc.drawHorizontalValueLabel(pos+neg, plot.X+vScale.Scale(pos), y+band/2, true, false, valueFont, style)
		}
	}
}

// drawHorizontalValueLabel writes one value past a bar's end: right of it,
// or left of it when leftward (a negative bar's end).
func (bc *BarChart) drawHorizontalValueLabel(v, endX, cy float64, bold, leftward bool, size float64, style *StyleGuide) {
	b := bc.builder
	label := TrueMinus(bc.config.ValueFmt.FormatOr(v, bc.config.ValueFormat))
	b.Push()
	b.SetFontSize(size).SetTextColor(style.Palette.TextPrimary)
	if bold {
		b.SetFontWeight(style.Typography.WeightBold)
	} else {
		b.SetFontWeight(style.Typography.WeightNormal)
	}
	if leftward {
		b.DrawText(label, endX-labelledValueGapPt, cy, TextAlignRight, TextBaselineMiddle)
	} else {
		b.DrawText(label, endX+labelledValueGapPt, cy, TextAlignLeft, TextBaselineMiddle)
	}
	b.Pop()
}

// drawHorizontalLegend draws the series legend under the plot.
func (bc *BarChart) drawHorizontalLegend(data ChartData, colors []Color, bounds Rect) {
	style := bc.builder.StyleGuide()
	legend := NewLegend(bc.builder, PresentationLegendConfig(style))
	items := make([]LegendItem, len(data.Series))
	for i, s := range data.Series {
		items[i] = LegendItem{Label: s.Name, Color: colors[i%len(colors)]}
		if s.Color != nil {
			items[i].Color = *s.Color
		}
	}
	legend.SetItems(items)
	legend.Draw(bounds)
}
