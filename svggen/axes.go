package svggen

import (
	"fmt"
)

// AxisPosition specifies where an axis is placed.
type AxisPosition int

const (
	AxisPositionBottom AxisPosition = iota
	AxisPositionTop
	AxisPositionLeft
	AxisPositionRight
)

// AxisConfig holds configuration for rendering an axis.
type AxisConfig struct {
	// Position determines where the axis is placed.
	Position AxisPosition

	// Title is the axis title text.
	Title string

	// TickCount is the approximate number of ticks to generate.
	// Only used for linear scales; categorical scales use all categories.
	TickCount int

	// TickSize is the length of tick marks in points.
	TickSize float64

	// TickPadding is the space between tick marks and labels in points.
	TickPadding float64

	// ShowGridLines enables grid lines across the chart area.
	ShowGridLines bool

	// GridLineLength is the length of grid lines (typically chart width/height).
	GridLineLength float64

	// LabelRotation rotates labels by this angle in degrees.
	// Useful for long category names on X axis.
	LabelRotation float64

	// Format is a printf-style format string for numeric values.
	// If empty, auto-detected from scale.
	Format string

	// HideAxisLine hides the main axis line.
	HideAxisLine bool

	// HideTicks hides tick marks (but shows labels).
	HideTicks bool

	// HideLabels hides tick labels.
	HideLabels bool

	// FontSize is the font size for axis labels in points.
	// If 0, defaults to style.Typography.SizeSmall.
	FontSize float64

	// RangeExtent, when > 0, clips tick marks and labels to
	// [0, RangeExtent] along the axis direction (relative to origin).
	// This prevents Nice()-expanded ticks from rendering outside the
	// plot area.
	RangeExtent float64
}

// DefaultAxisConfig returns default axis configuration.
func DefaultAxisConfig(position AxisPosition) AxisConfig {
	return AxisConfig{
		Position:       position,
		TickCount:      5,
		TickSize:       6,
		TickPadding:    4,
		ShowGridLines:  false,
		GridLineLength: 0,
		LabelRotation:  0,
		Format:         "",
		HideAxisLine:   false,
		HideTicks:      false,
		HideLabels:     false,
	}
}

// Axis renders axes for charts using the SVGBuilder.
type Axis struct {
	builder *SVGBuilder
	config  AxisConfig
}

// NewAxis creates a new axis renderer.
func NewAxis(builder *SVGBuilder, config AxisConfig) *Axis {
	return &Axis{
		builder: builder,
		config:  config,
	}
}

// DrawLinearAxis draws an axis for a linear scale.
func (a *Axis) DrawLinearAxis(scale *LinearScale, x, y float64) *Axis {
	ticks := scale.Ticks(a.config.TickCount)
	format := a.config.Format
	if format == "" {
		format = scale.TickFormat(a.config.TickCount)
	}

	// Auto-detect whether to use compact format: if any tick value exceeds
	// 9999 in absolute value and no explicit format was provided, use
	// FormatCompact (K/M/B suffixes) to keep labels short.
	useCompact := false
	if a.config.Format == "" {
		for _, v := range ticks {
			if v > 9999 || v < -9999 {
				useCompact = true
				break
			}
		}
	}

	// Convert tick values to labels
	labels := make([]string, len(ticks))
	for i, v := range ticks {
		if useCompact {
			labels[i] = FormatCompact(v)
		} else {
			labels[i] = fmt.Sprintf(format, v)
		}
	}

	// Get tick positions
	positions := make([]float64, len(ticks))
	for i, v := range ticks {
		positions[i] = scale.Scale(v)
	}

	a.drawAxis(positions, labels, x, y)
	return a
}

// DrawCategoricalAxis draws an axis for a categorical scale.
func (a *Axis) DrawCategoricalAxis(scale *CategoricalScale, x, y float64) *Axis {
	categories := scale.Categories()

	// Get positions (center of each band)
	positions := make([]float64, len(categories))
	for i, cat := range categories {
		positions[i] = scale.Scale(cat)
	}

	a.drawAxis(positions, categories, x, y)
	return a
}

// DrawTimeAxis draws an axis for a time scale.
func (a *Axis) DrawTimeAxis(scale *TimeScale, x, y float64) *Axis {
	ticks, labels := scale.TicksWithLabels(a.config.TickCount)

	// Get tick positions
	positions := make([]float64, len(ticks))
	for i, ts := range ticks {
		positions[i] = scale.Scale(ts)
	}

	a.drawAxis(positions, labels, x, y)
	return a
}

// DrawLogAxis draws an axis for a log scale with compact power-of-10 labels.
func (a *Axis) DrawLogAxis(scale *LogScale, x, y float64) *Axis {
	ticks := scale.Ticks(a.config.TickCount)

	labels := make([]string, len(ticks))
	for i, v := range ticks {
		labels[i] = FormatLogLabel(v)
	}

	positions := make([]float64, len(ticks))
	for i, v := range ticks {
		positions[i] = scale.Scale(v)
	}

	a.drawAxis(positions, labels, x, y)
	return a
}

// drawAxis is the internal implementation for drawing axes.
func (a *Axis) drawAxis(positions []float64, labels []string, originX, originY float64) {
	if len(positions) == 0 {
		return
	}

	style := a.builder.StyleGuide()
	b := a.builder

	// Save state
	b.Push()

	// Set up colors
	axisColor := style.Palette.TextSecondary
	gridColor := style.Palette.Border.WithAlpha(0.5)

	// Calculate axis line bounds
	var axisStart, axisEnd float64
	if len(positions) > 0 {
		axisStart = positions[0]
		axisEnd = positions[len(positions)-1]
	}

	isHorizontal := a.config.Position == AxisPositionBottom || a.config.Position == AxisPositionTop

	// Draw axis line
	if !a.config.HideAxisLine {
		b.SetStrokeColor(axisColor).SetStrokeWidth(style.Strokes.WidthThin)
		if isHorizontal {
			b.DrawLine(originX+axisStart, originY, originX+axisEnd, originY)
		} else {
			b.DrawLine(originX, originY+axisStart, originX, originY+axisEnd)
		}
	}

	// Draw ticks, grid lines, and labels
	for i, pos := range positions {
		a.drawTick(pos, originX, originY, labels[i], axisColor, gridColor)
	}

	// Draw title
	if a.config.Title != "" {
		a.drawTitle(originX, originY, positions)
	}

	// Restore state
	b.Pop()
}

// drawTick draws a single tick mark, optional grid line, and label.
func (a *Axis) drawTick(pos, originX, originY float64, label string, axisColor, gridColor Color) {
	// Skip ticks outside the visible range when RangeExtent is set.
	if a.config.RangeExtent > 0 {
		const eps = 0.5 // sub-pixel tolerance
		if pos < -eps || pos > a.config.RangeExtent+eps {
			return
		}
	}

	b := a.builder
	style := b.StyleGuide()

	isHorizontal := a.config.Position == AxisPositionBottom || a.config.Position == AxisPositionTop

	var tickX1, tickY1, tickX2, tickY2 float64
	var labelX, labelY float64
	var align TextAlign
	var baseline TextBaseline

	tickSize := a.config.TickSize
	tickPadding := a.config.TickPadding

	switch a.config.Position {
	case AxisPositionBottom:
		tickX1 = originX + pos
		tickY1 = originY
		tickX2 = tickX1
		tickY2 = originY + tickSize
		labelX = tickX1
		labelY = tickY2 + tickPadding
		align = TextAlignCenter
		baseline = TextBaselineTop

	case AxisPositionTop:
		tickX1 = originX + pos
		tickY1 = originY
		tickX2 = tickX1
		tickY2 = originY - tickSize
		labelX = tickX1
		labelY = tickY2 - tickPadding
		align = TextAlignCenter
		baseline = TextBaselineBottom

	case AxisPositionLeft:
		tickX1 = originX
		tickY1 = originY + pos
		tickX2 = originX - tickSize
		tickY2 = tickY1
		labelX = tickX2 - tickPadding
		labelY = tickY1
		align = TextAlignRight
		baseline = TextBaselineMiddle

	case AxisPositionRight:
		tickX1 = originX
		tickY1 = originY + pos
		tickX2 = originX + tickSize
		tickY2 = tickY1
		labelX = tickX2 + tickPadding
		labelY = tickY1
		align = TextAlignLeft
		baseline = TextBaselineMiddle
	}

	// Draw grid line
	if a.config.ShowGridLines && a.config.GridLineLength > 0 {
		b.Push()
		b.SetStrokeColor(gridColor).SetStrokeWidth(style.Strokes.WidthHairline)
		b.SetDashes(style.Strokes.PatternDotted...)

		if isHorizontal {
			// Vertical grid line
			switch a.config.Position {
			case AxisPositionBottom:
				b.DrawLine(tickX1, originY, tickX1, originY-a.config.GridLineLength)
			case AxisPositionTop:
				b.DrawLine(tickX1, originY, tickX1, originY+a.config.GridLineLength)
			}
		} else {
			// Horizontal grid line
			switch a.config.Position {
			case AxisPositionLeft:
				b.DrawLine(originX, tickY1, originX+a.config.GridLineLength, tickY1)
			case AxisPositionRight:
				b.DrawLine(originX, tickY1, originX-a.config.GridLineLength, tickY1)
			}
		}

		b.Pop()
	}

	// Draw tick mark
	if !a.config.HideTicks {
		b.SetStrokeColor(axisColor).SetStrokeWidth(style.Strokes.WidthThin)
		b.DrawLine(tickX1, tickY1, tickX2, tickY2)
	}

	// Draw label
	if !a.config.HideLabels && label != "" {
		fontSize := a.config.FontSize
		if fontSize == 0 {
			fontSize = style.Typography.SizeSmall
		}
		b.SetFontSize(fontSize).SetFontWeight(style.Typography.WeightNormal)

		if a.config.LabelRotation != 0 {
			// Rotated labels need a vertical offset on bottom axes to
			// prevent the rotated text from overlapping the axis line.
			// The rotation anchor is at (labelX, labelY); without an
			// offset the text swings upward into the chart area.
			if a.config.Position == AxisPositionBottom {
				labelY += fontSize
			}
			b.Push()
			b.RotateAround(a.config.LabelRotation, labelX, labelY)
			// Adjust alignment for rotation on bottom axis:
			// Negative rotation (e.g. -45°) tilts labels to the left —
			// text-anchor:end keeps the label end at the tick mark so text
			// extends down-left away from the chart (standard convention).
			// Positive rotation tilts labels to the right — text-anchor:start
			// keeps the label start at the tick mark.
			if a.config.LabelRotation < 0 {
				align = TextAlignRight
			} else {
				align = TextAlignLeft
			}
			b.DrawText(label, labelX, labelY, align, baseline)
			b.Pop()
		} else {
			b.DrawText(label, labelX, labelY, align, baseline)
		}
	}
}

// drawTitle draws the axis title.
func (a *Axis) drawTitle(originX, originY float64, positions []float64) {
	if len(positions) == 0 || a.config.Title == "" {
		return
	}

	b := a.builder
	style := b.StyleGuide()

	// Calculate title position
	axisStart := positions[0]
	axisEnd := positions[len(positions)-1]
	axisMid := (axisStart + axisEnd) / 2

	b.Push()
	b.SetFontSize(style.Typography.SizeBody).SetFontWeight(style.Typography.WeightMedium)

	var titleX, titleY float64
	var align TextAlign

	titleOffset := a.config.TickSize + a.config.TickPadding + style.Typography.SizeSmall + style.Spacing.LG

	switch a.config.Position {
	case AxisPositionBottom:
		titleX = originX + axisMid
		titleY = originY + titleOffset
		align = TextAlignCenter
		b.DrawText(a.config.Title, titleX, titleY, align, TextBaselineTop)

	case AxisPositionTop:
		titleX = originX + axisMid
		titleY = originY - titleOffset
		align = TextAlignCenter
		b.DrawText(a.config.Title, titleX, titleY, align, TextBaselineBottom)

	case AxisPositionLeft:
		titleX = originX - titleOffset
		titleY = originY + axisMid
		// Rotate title for vertical axis
		b.Push()
		b.RotateAround(-90, titleX, titleY)
		b.DrawText(a.config.Title, titleX, titleY, TextAlignCenter, TextBaselineBottom)
		b.Pop()

	case AxisPositionRight:
		titleX = originX + titleOffset
		titleY = originY + axisMid
		// Rotate title for vertical axis
		b.Push()
		b.RotateAround(90, titleX, titleY)
		b.DrawText(a.config.Title, titleX, titleY, TextAlignCenter, TextBaselineBottom)
		b.Pop()
	}

	b.Pop()
}

// =============================================================================
// Chart Area Helper
// =============================================================================

// ChartArea defines the plot area dimensions and margins.
type ChartArea struct {
	// Total dimensions
	Width  float64
	Height float64

	// Margins
	MarginTop    float64
	MarginRight  float64
	MarginBottom float64
	MarginLeft   float64
}

// DefaultChartArea creates a chart area with default margins.
func DefaultChartArea(width, height float64) ChartArea {
	margin := 40.0
	return ChartArea{
		Width:        width,
		Height:       height,
		MarginTop:    margin,
		MarginRight:  margin,
		MarginBottom: margin * 1.5, // Extra for X axis labels
		MarginLeft:   margin * 1.5, // Extra for Y axis labels
	}
}

// PlotRect returns the inner plot area rectangle.
func (ca ChartArea) PlotRect() Rect {
	return Rect{
		X: ca.MarginLeft,
		Y: ca.MarginTop,
		W: ca.Width - ca.MarginLeft - ca.MarginRight,
		H: ca.Height - ca.MarginTop - ca.MarginBottom,
	}
}

// PlotWidth returns the inner plot area width.
func (ca ChartArea) PlotWidth() float64 {
	return ca.Width - ca.MarginLeft - ca.MarginRight
}

// PlotHeight returns the inner plot area height.
func (ca ChartArea) PlotHeight() float64 {
	return ca.Height - ca.MarginTop - ca.MarginBottom
}

// XAxisY returns the Y coordinate for the X axis (at bottom of plot area).
func (ca ChartArea) XAxisY() float64 {
	return ca.Height - ca.MarginBottom
}

// YAxisX returns the X coordinate for the Y axis (at left of plot area).
func (ca ChartArea) YAxisX() float64 {
	return ca.MarginLeft
}

// =============================================================================
// Grid Lines Helper
// =============================================================================

// GridConfig holds configuration for drawing grid lines.
type GridConfig struct {
	ShowHorizontal bool
	ShowVertical   bool
	Color          Color
	StrokeWidth    float64
	DashPattern    []float64
}

// DefaultGridConfig returns default grid configuration.
// Grid lines are subtle light gray to appear professional without visual clutter.
func DefaultGridConfig() GridConfig {
	return GridConfig{
		ShowHorizontal: true,
		ShowVertical:   false,
		Color:          MustParseColor("#E0E0E0"),
		StrokeWidth:    0.75,
		DashPattern:    nil, // Solid lines for a clean dashboard look
	}
}

// DrawGrid draws grid lines for the specified scales.
func DrawGrid(b *SVGBuilder, area ChartArea, xScale, yScale Scale, config GridConfig) {
	plotRect := area.PlotRect()

	b.Push()
	b.SetStrokeColor(config.Color).SetStrokeWidth(config.StrokeWidth)
	if len(config.DashPattern) > 0 {
		b.SetDashes(config.DashPattern...)
	}

	// Draw horizontal grid lines (for Y scale)
	if config.ShowHorizontal {
		if ls, ok := yScale.(*LinearScale); ok {
			ticks := ls.Ticks(5)
			for _, v := range ticks {
				y := ls.Scale(v)
				// Y scale typically goes from bottom to top, so invert
				y = plotRect.Y + plotRect.H - y + plotRect.Y
				b.DrawLine(plotRect.X, y, plotRect.X+plotRect.W, y)
			}
		}
	}

	// Draw vertical grid lines (for X scale)
	if config.ShowVertical {
		if ls, ok := xScale.(*LinearScale); ok {
			ticks := ls.Ticks(5)
			for _, v := range ticks {
				x := ls.Scale(v) + plotRect.X
				b.DrawLine(x, plotRect.Y, x, plotRect.Y+plotRect.H)
			}
		} else if cs, ok := xScale.(*CategoricalScale); ok {
			for _, cat := range cs.Categories() {
				x := cs.Scale(cat) + plotRect.X
				b.DrawLine(x, plotRect.Y, x, plotRect.Y+plotRect.H)
			}
		}
	}

	b.Pop()
}

// =============================================================================
// Axis Builder (Fluent API)
// =============================================================================

// AxisBuilder provides a fluent API for configuring and drawing axes.
type AxisBuilder struct {
	builder *SVGBuilder
	config  AxisConfig
}

// NewAxisBuilder creates a new axis builder.
func NewAxisBuilder(builder *SVGBuilder, position AxisPosition) *AxisBuilder {
	return &AxisBuilder{
		builder: builder,
		config:  DefaultAxisConfig(position),
	}
}

// Title sets the axis title.
func (ab *AxisBuilder) Title(title string) *AxisBuilder {
	ab.config.Title = title
	return ab
}

// TickCount sets the approximate number of ticks.
func (ab *AxisBuilder) TickCount(count int) *AxisBuilder {
	ab.config.TickCount = count
	return ab
}

// TickSize sets the tick mark length.
func (ab *AxisBuilder) TickSize(size float64) *AxisBuilder {
	ab.config.TickSize = size
	return ab
}

// TickPadding sets the space between ticks and labels.
func (ab *AxisBuilder) TickPadding(padding float64) *AxisBuilder {
	ab.config.TickPadding = padding
	return ab
}

// ShowGrid enables grid lines with the specified length.
func (ab *AxisBuilder) ShowGrid(length float64) *AxisBuilder {
	ab.config.ShowGridLines = true
	ab.config.GridLineLength = length
	return ab
}

// LabelRotation sets the label rotation angle.
func (ab *AxisBuilder) LabelRotation(degrees float64) *AxisBuilder {
	ab.config.LabelRotation = degrees
	return ab
}

// Format sets the printf-style format string.
func (ab *AxisBuilder) Format(format string) *AxisBuilder {
	ab.config.Format = format
	return ab
}

// HideAxisLine hides the main axis line.
func (ab *AxisBuilder) HideAxisLine() *AxisBuilder {
	ab.config.HideAxisLine = true
	return ab
}

// HideTicks hides tick marks.
func (ab *AxisBuilder) HideTicks() *AxisBuilder {
	ab.config.HideTicks = true
	return ab
}

// HideLabels hides tick labels.
func (ab *AxisBuilder) HideLabels() *AxisBuilder {
	ab.config.HideLabels = true
	return ab
}

// DrawLinear draws the axis for a linear scale.
func (ab *AxisBuilder) DrawLinear(scale *LinearScale, x, y float64) *SVGBuilder {
	axis := NewAxis(ab.builder, ab.config)
	axis.DrawLinearAxis(scale, x, y)
	return ab.builder
}

// DrawCategorical draws the axis for a categorical scale.
func (ab *AxisBuilder) DrawCategorical(scale *CategoricalScale, x, y float64) *SVGBuilder {
	axis := NewAxis(ab.builder, ab.config)
	axis.DrawCategoricalAxis(scale, x, y)
	return ab.builder
}

// DrawTime draws the axis for a time scale.
func (ab *AxisBuilder) DrawTime(scale *TimeScale, x, y float64) *SVGBuilder {
	axis := NewAxis(ab.builder, ab.config)
	axis.DrawTimeAxis(scale, x, y)
	return ab.builder
}
