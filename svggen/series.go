package svggen

import (
	"fmt"
	"math"
)

// SeriesOrientation specifies vertical or horizontal orientation for series.
type SeriesOrientation int

const (
	SeriesVertical SeriesOrientation = iota
	SeriesHorizontal
)

// MarkerShape specifies the shape for data point markers.
type MarkerShape int

const (
	MarkerCircle MarkerShape = iota
	MarkerSquare
	MarkerDiamond
	MarkerTriangle
	MarkerTriangleDown
	MarkerCross
	MarkerPlus
	MarkerNone
)

// =============================================================================
// Data Point
// =============================================================================

// DataPoint represents a single data point in a series.
type DataPoint struct {
	// X is the x-coordinate value (for categorical scales, use XCategory).
	X float64

	// Y is the y-coordinate value.
	Y float64

	// XCategory is the category name (for categorical x-scales).
	XCategory string

	// Label is an optional label for the data point.
	Label string

	// Value is the raw value for display purposes.
	Value float64
}

// =============================================================================
// Bar Series
// =============================================================================

// BarSeriesConfig holds configuration for rendering bar series.
type BarSeriesConfig struct {
	// Orientation is vertical (default) or horizontal.
	Orientation SeriesOrientation

	// BarWidth is the width of each bar in points.
	// If 0, calculated from scale bandwidth.
	BarWidth float64

	// BarPadding is padding between bars in the same group (0-1).
	BarPadding float64

	// Color is the fill color for bars.
	Color Color

	// StrokeColor is the stroke color for bars (optional).
	StrokeColor Color

	// StrokeWidth is the stroke width in points.
	StrokeWidth float64

	// CornerRadius is the corner radius for rounded bars.
	CornerRadius float64

	// ShowValues enables value labels on bars.
	ShowValues bool

	// ValueFormat is the printf-style format for value labels.
	ValueFormat string

	// ValuePosition specifies where to place value labels.
	ValuePosition ValuePosition

	// SeriesIndex is the index of this series in a grouped bar chart.
	SeriesIndex int

	// SeriesCount is the total number of series in a grouped bar chart.
	SeriesCount int
}

// ValuePosition specifies where to place value labels.
type ValuePosition int

const (
	ValuePositionTop ValuePosition = iota
	ValuePositionCenter
	ValuePositionBottom
	ValuePositionOutside
)

// DefaultBarSeriesConfig returns default bar series configuration.
func DefaultBarSeriesConfig() BarSeriesConfig {
	return BarSeriesConfig{
		Orientation:   SeriesVertical,
		BarWidth:      0, // Auto-calculate
		BarPadding:    0.1,
		Color:         MustParseColor("#4E79A7"),
		StrokeColor:   Color{A: 0}, // Transparent
		StrokeWidth:   0,
		CornerRadius:  0,
		ShowValues:    false,
		ValueFormat:   "%.0f",
		ValuePosition: ValuePositionTop,
		SeriesIndex:   0,
		SeriesCount:   1,
	}
}

// BarSeries renders bar chart series.
type BarSeries struct {
	builder *SVGBuilder
	config  BarSeriesConfig
}

// NewBarSeries creates a new bar series renderer.
func NewBarSeries(builder *SVGBuilder, config BarSeriesConfig) *BarSeries {
	return &BarSeries{
		builder: builder,
		config:  config,
	}
}

// DrawCategorical draws bars for categorical x-scale and linear y-scale.
func (bs *BarSeries) DrawCategorical(points []DataPoint, xScale *CategoricalScale, yScale *LinearScale, baseY float64) *BarSeries {
	if len(points) == 0 {
		return bs
	}

	b := bs.builder
	style := b.StyleGuide()

	// Calculate bar width from bandwidth if not specified
	barWidth := bs.config.BarWidth
	if barWidth == 0 {
		bandwidth := xScale.Bandwidth()
		if bs.config.SeriesCount > 1 {
			// Grouped bars: divide bandwidth by series count
			barWidth = bandwidth * (1 - bs.config.BarPadding) / float64(bs.config.SeriesCount)
		} else {
			barWidth = bandwidth * (1 - bs.config.BarPadding)
		}
	}

	b.Push()

	for _, point := range points {
		x := xScale.Scale(point.XCategory)
		y := yScale.Scale(point.Y)

		// Adjust x position for grouped bars
		if bs.config.SeriesCount > 1 {
			bandwidth := xScale.Bandwidth()
			groupWidth := bandwidth * (1 - bs.config.BarPadding)
			barOffset := groupWidth / float64(bs.config.SeriesCount) * float64(bs.config.SeriesIndex)
			x = xScale.ScaleStart(point.XCategory) + (bandwidth-groupWidth)/2 + barOffset + barWidth/2
		}

		bs.drawBar(x, y, barWidth, baseY)
	}

	// Draw value labels if enabled
	if bs.config.ShowValues {
		b.SetFontSize(style.Typography.SizeSmall).SetFontWeight(style.Typography.WeightNormal)
		for _, point := range points {
			x := xScale.Scale(point.XCategory)
			y := yScale.Scale(point.Y)

			if bs.config.SeriesCount > 1 {
				bandwidth := xScale.Bandwidth()
				groupWidth := bandwidth * (1 - bs.config.BarPadding)
				barOffset := groupWidth / float64(bs.config.SeriesCount) * float64(bs.config.SeriesIndex)
				x = xScale.ScaleStart(point.XCategory) + (bandwidth-groupWidth)/2 + barOffset + barWidth/2
			}

			bs.drawValueLabel(x, y, point.Y, barWidth, baseY)
		}
	}

	b.Pop()
	return bs
}

// DrawLinear draws bars for linear scales on both axes (e.g., histogram).
func (bs *BarSeries) DrawLinear(points []DataPoint, xScale, yScale *LinearScale, barWidth, baseY float64) *BarSeries {
	if len(points) == 0 {
		return bs
	}

	b := bs.builder
	style := b.StyleGuide()

	b.Push()

	for _, point := range points {
		x := xScale.Scale(point.X)
		y := yScale.Scale(point.Y)
		bs.drawBar(x, y, barWidth, baseY)
	}

	if bs.config.ShowValues {
		b.SetFontSize(style.Typography.SizeSmall).SetFontWeight(style.Typography.WeightNormal)
		for _, point := range points {
			x := xScale.Scale(point.X)
			y := yScale.Scale(point.Y)
			bs.drawValueLabel(x, y, point.Y, barWidth, baseY)
		}
	}

	b.Pop()
	return bs
}

// drawBar draws a single bar.
func (bs *BarSeries) drawBar(x, y, width, baseY float64) {
	b := bs.builder

	var rect Rect
	if bs.config.Orientation == SeriesVertical {
		// Bar extends from baseY to y
		barHeight := baseY - y
		if barHeight < 0 {
			// Negative value: bar extends downward
			rect = Rect{
				X: x - width/2,
				Y: baseY,
				W: width,
				H: -barHeight,
			}
		} else {
			rect = Rect{
				X: x - width/2,
				Y: y,
				W: width,
				H: barHeight,
			}
		}
	} else {
		// Horizontal bar
		barWidth := y - baseY
		if barWidth < 0 {
			rect = Rect{
				X: y,
				Y: x - width/2,
				W: -barWidth,
				H: width,
			}
		} else {
			rect = Rect{
				X: baseY,
				Y: x - width/2,
				W: barWidth,
				H: width,
			}
		}
	}

	// Set colors
	b.SetFillColor(bs.config.Color)
	if bs.config.StrokeWidth > 0 {
		b.SetStrokeColor(bs.config.StrokeColor)
		b.SetStrokeWidth(bs.config.StrokeWidth)
	}

	// Draw the bar
	if bs.config.CornerRadius > 0 {
		b.DrawRoundedRect(rect, bs.config.CornerRadius)
	} else {
		b.FillRect(rect)
	}
}

// drawValueLabel draws a value label for a bar.
func (bs *BarSeries) drawValueLabel(x, y, value, width, baseY float64) {
	b := bs.builder
	style := b.StyleGuide()

	label := formatValue(value, bs.config.ValueFormat)

	var labelX, labelY float64
	var baseline TextBaseline

	barHeight := baseY - y
	if bs.config.Orientation == SeriesVertical {
		labelX = x
		switch bs.config.ValuePosition {
		case ValuePositionTop:
			if barHeight >= 0 {
				labelY = y - style.Spacing.XS
				baseline = TextBaselineBottom
			} else {
				labelY = y + style.Spacing.XS
				baseline = TextBaselineTop
			}
		case ValuePositionCenter:
			labelY = (y + baseY) / 2
			baseline = TextBaselineMiddle
		case ValuePositionBottom:
			labelY = baseY - style.Spacing.XS
			baseline = TextBaselineBottom
		case ValuePositionOutside:
			if barHeight >= 0 {
				labelY = y - style.Spacing.SM
				baseline = TextBaselineBottom
			} else {
				labelY = y + style.Spacing.SM
				baseline = TextBaselineTop
			}
		}
	} else {
		// Horizontal bar labels
		labelY = x
		baseline = TextBaselineMiddle
		switch bs.config.ValuePosition {
		case ValuePositionTop, ValuePositionOutside:
			labelX = y + style.Spacing.XS
		case ValuePositionCenter:
			labelX = (y + baseY) / 2
		default:
			labelX = baseY + style.Spacing.XS
		}
	}

	b.DrawText(label, labelX, labelY, TextAlignCenter, baseline)
}

// =============================================================================
// Line Series
// =============================================================================

// LineSeriesConfig holds configuration for rendering line series.
type LineSeriesConfig struct {
	// Color is the line color.
	Color Color

	// StrokeWidth is the line stroke width in points.
	StrokeWidth float64

	// DashPattern is an optional dash pattern for the line.
	DashPattern []float64

	// ShowMarkers enables markers at data points.
	ShowMarkers bool

	// MarkerShape is the marker shape.
	MarkerShape MarkerShape

	// MarkerSize is the marker size in points.
	MarkerSize float64

	// MarkerFillColor is the fill color for markers.
	MarkerFillColor Color

	// MarkerStrokeColor is the stroke color for markers.
	MarkerStrokeColor Color

	// MarkerStrokeWidth is the stroke width for markers.
	MarkerStrokeWidth float64

	// FillArea enables area fill below the line.
	FillArea bool

	// FillColor is the area fill color.
	FillColor Color

	// FillOpacity is the area fill opacity (0-1).
	FillOpacity float64

	// Smooth enables curved line interpolation.
	Smooth bool

	// Tension controls the smoothness (0-1, default 0.5).
	Tension float64

	// ShowValues enables value labels at data points.
	ShowValues bool

	// ValueFormat is the printf-style format for value labels.
	ValueFormat string
}

// DefaultLineSeriesConfig returns default line series configuration.
func DefaultLineSeriesConfig() LineSeriesConfig {
	return LineSeriesConfig{
		Color:             MustParseColor("#4E79A7"),
		StrokeWidth:       3.0,
		DashPattern:       nil,
		ShowMarkers:       true,
		MarkerShape:       MarkerCircle,
		MarkerSize:        8,
		MarkerFillColor:   MustParseColor("#4E79A7"),
		MarkerStrokeColor: MustParseColor("#FFFFFF"),
		MarkerStrokeWidth: 2.0,
		FillArea:          false,
		FillColor:         MustParseColor("#4E79A7"),
		FillOpacity:       0.2,
		Smooth:            false,
		Tension:           0.5,
		ShowValues:        false,
		ValueFormat:       "%.0f",
	}
}

// LineSeries renders line chart series.
type LineSeries struct {
	builder *SVGBuilder
	config  LineSeriesConfig
}

// NewLineSeries creates a new line series renderer.
func NewLineSeries(builder *SVGBuilder, config LineSeriesConfig) *LineSeries {
	return &LineSeries{
		builder: builder,
		config:  config,
	}
}

// DrawCategorical draws a line for categorical x-scale and linear y-scale.
func (ls *LineSeries) DrawCategorical(points []DataPoint, xScale *CategoricalScale, yScale *LinearScale, baseY float64) *LineSeries {
	if len(points) < 2 {
		if len(points) == 1 && ls.config.ShowMarkers {
			// Draw single marker
			x := xScale.Scale(points[0].XCategory)
			y := yScale.Scale(points[0].Y)
			ls.drawMarker(x, y)
		}
		return ls
	}

	// Convert to screen coordinates
	coords := make([]Point, len(points))
	for i, p := range points {
		coords[i] = Point{
			X: xScale.Scale(p.XCategory),
			Y: yScale.Scale(p.Y),
		}
	}

	ls.draw(coords, baseY, points)
	return ls
}

// DrawLinear draws a line for linear scales on both axes.
func (ls *LineSeries) DrawLinear(points []DataPoint, xScale, yScale *LinearScale, baseY float64) *LineSeries {
	if len(points) < 2 {
		if len(points) == 1 && ls.config.ShowMarkers {
			x := xScale.Scale(points[0].X)
			y := yScale.Scale(points[0].Y)
			ls.drawMarker(x, y)
		}
		return ls
	}

	// Convert to screen coordinates
	coords := make([]Point, len(points))
	for i, p := range points {
		coords[i] = Point{
			X: xScale.Scale(p.X),
			Y: yScale.Scale(p.Y),
		}
	}

	ls.draw(coords, baseY, points)
	return ls
}

// draw draws the line with optional area fill and markers.
func (ls *LineSeries) draw(coords []Point, baseY float64, points []DataPoint) {
	b := ls.builder
	style := b.StyleGuide()

	b.Push()

	// Draw area fill first (below the line)
	if ls.config.FillArea {
		ls.drawAreaFill(coords, baseY)
	}

	// Draw the line
	b.SetStrokeColor(ls.config.Color)
	b.SetStrokeWidth(ls.config.StrokeWidth)
	b.SetLineCap(LineCapRound)
	b.SetLineJoin(LineJoinRound)

	if len(ls.config.DashPattern) > 0 {
		b.SetDashes(ls.config.DashPattern...)
	}

	if ls.config.Smooth && len(coords) >= 3 {
		ls.drawSmoothLine(coords)
	} else {
		b.DrawPolyline(coords)
	}

	// Draw markers
	if ls.config.ShowMarkers {
		for _, c := range coords {
			ls.drawMarker(c.X, c.Y)
		}
	}

	// Draw value labels
	if ls.config.ShowValues {
		b.SetFontSize(style.Typography.SizeSmall).SetFontWeight(style.Typography.WeightNormal)
		for i, c := range coords {
			label := formatValue(points[i].Y, ls.config.ValueFormat)
			labelY := c.Y - ls.config.MarkerSize - style.Spacing.XS
			b.DrawText(label, c.X, labelY, TextAlignCenter, TextBaselineBottom)
		}
	}

	b.Pop()
}

// drawAreaFill draws the filled area below the line.
func (ls *LineSeries) drawAreaFill(coords []Point, baseY float64) {
	if len(coords) < 2 {
		return
	}

	b := ls.builder

	fillColor := ls.config.FillColor.WithAlpha(ls.config.FillOpacity)

	// Ensure the fill is visible against the background by boosting opacity
	// when the composited color lacks sufficient contrast.
	bg := b.StyleGuide().Palette.Background
	fillColor = ensureAreaFillContrast(fillColor, bg)

	b.SetFillColor(fillColor)

	// Create path: line + close to baseline
	path := b.BeginPath()
	path.MoveTo(coords[0].X, baseY)

	if ls.config.Smooth && len(coords) >= 3 {
		// Use smooth curve for top edge
		path.LineTo(coords[0].X, coords[0].Y)
		for i := 1; i < len(coords); i++ {
			// Simple quadratic interpolation
			prev := coords[i-1]
			curr := coords[i]
			cpX := (prev.X + curr.X) / 2
			path.LineTo(cpX, prev.Y)
			path.LineTo(cpX, curr.Y)
		}
		path.LineTo(coords[len(coords)-1].X, coords[len(coords)-1].Y)
	} else {
		for _, c := range coords {
			path.LineTo(c.X, c.Y)
		}
	}

	path.LineTo(coords[len(coords)-1].X, baseY)
	path.Close()
	path.Fill()
}

// ensureAreaFillContrast boosts fill opacity when the composited color would
// be too close to the background. This prevents area fills from becoming
// invisible on templates with muted accent palettes (e.g. template_2).
func ensureAreaFillContrast(fill Color, bg Color) Color {
	const minContrast = 2.5 // minimum contrast ratio for area fills
	const maxOpacity = 0.85
	const step = 0.05

	blended := blendOver(fill, bg)
	if blended.ContrastWith(bg) >= minContrast {
		return fill
	}

	// Increase opacity in steps until contrast is sufficient
	opacity := fill.A
	for opacity < maxOpacity {
		opacity += step
		candidate := fill.WithAlpha(opacity)
		blended = blendOver(candidate, bg)
		if blended.ContrastWith(bg) >= minContrast {
			return candidate
		}
	}
	return fill.WithAlpha(maxOpacity)
}

// blendOver composites a foreground color (with alpha) over a background color,
// returning the resulting opaque color.
func blendOver(fg, bg Color) Color {
	a := fg.A
	return Color{
		R: uint8(float64(fg.R)*a + float64(bg.R)*(1-a)),
		G: uint8(float64(fg.G)*a + float64(bg.G)*(1-a)),
		B: uint8(float64(fg.B)*a + float64(bg.B)*(1-a)),
		A: 1.0,
	}
}

// drawSmoothLine draws a smooth curved line through the points.
func (ls *LineSeries) drawSmoothLine(coords []Point) {
	b := ls.builder
	tension := ls.config.Tension

	// Use cardinal spline interpolation
	path := b.BeginPath()
	path.MoveTo(coords[0].X, coords[0].Y)

	for i := 0; i < len(coords)-1; i++ {
		p0 := coords[max(0, i-1)]
		p1 := coords[i]
		p2 := coords[min(len(coords)-1, i+1)]
		p3 := coords[min(len(coords)-1, i+2)]

		// Cardinal spline control points
		cp1x := p1.X + (p2.X-p0.X)/6*tension
		cp1y := p1.Y + (p2.Y-p0.Y)/6*tension
		cp2x := p2.X - (p3.X-p1.X)/6*tension
		cp2y := p2.Y - (p3.Y-p1.Y)/6*tension

		path.CubicTo(cp1x, cp1y, cp2x, cp2y, p2.X, p2.Y)
	}

	path.Stroke()
}

// drawMarker draws a marker at the specified position.
func (ls *LineSeries) drawMarker(x, y float64) {
	b := ls.builder
	size := ls.config.MarkerSize / 2 // radius

	b.Push()
	b.SetFillColor(ls.config.MarkerFillColor)
	b.SetStrokeColor(ls.config.MarkerStrokeColor)
	b.SetStrokeWidth(ls.config.MarkerStrokeWidth)

	switch ls.config.MarkerShape {
	case MarkerCircle:
		b.DrawCircle(x, y, size)

	case MarkerSquare:
		b.DrawRect(Rect{X: x - size, Y: y - size, W: size * 2, H: size * 2})

	case MarkerDiamond:
		points := []Point{
			{X: x, Y: y - size},
			{X: x + size, Y: y},
			{X: x, Y: y + size},
			{X: x - size, Y: y},
		}
		b.DrawPolygon(points)

	case MarkerTriangle:
		h := size * math.Sqrt(3)
		points := []Point{
			{X: x, Y: y - size},
			{X: x + h/2, Y: y + size/2},
			{X: x - h/2, Y: y + size/2},
		}
		b.DrawPolygon(points)

	case MarkerTriangleDown:
		h := size * math.Sqrt(3)
		points := []Point{
			{X: x, Y: y + size},
			{X: x + h/2, Y: y - size/2},
			{X: x - h/2, Y: y - size/2},
		}
		b.DrawPolygon(points)

	case MarkerCross:
		b.DrawLine(x-size, y-size, x+size, y+size)
		b.DrawLine(x-size, y+size, x+size, y-size)

	case MarkerPlus:
		b.DrawLine(x-size, y, x+size, y)
		b.DrawLine(x, y-size, x, y+size)

	case MarkerNone:
		// Do nothing
	}

	b.Pop()
}

// =============================================================================
// Point Series (Scatter)
// =============================================================================

// PointSeriesConfig holds configuration for rendering scatter/point series.
type PointSeriesConfig struct {
	// Color is the default point color.
	Color Color

	// Size is the default point size in points.
	Size float64

	// Shape is the point marker shape.
	Shape MarkerShape

	// StrokeColor is the stroke color for points.
	StrokeColor Color

	// StrokeWidth is the stroke width for points.
	StrokeWidth float64

	// Opacity is the fill opacity (0-1).
	Opacity float64

	// ShowLabels enables labels next to points.
	ShowLabels bool

	// LabelOffset is the offset from point center for labels.
	LabelOffset float64

	// SizeScale enables variable point sizes based on value.
	SizeScale *LinearScale

	// ColorScale enables variable colors based on value (uses palette).
	ColorScale *LinearScale
}

// DefaultPointSeriesConfig returns default point series configuration.
func DefaultPointSeriesConfig() PointSeriesConfig {
	return PointSeriesConfig{
		Color:       MustParseColor("#4E79A7"),
		Size:        8,
		Shape:       MarkerCircle,
		StrokeColor: MustParseColor("#FFFFFF"),
		StrokeWidth: 2.0,
		Opacity:     0.8,
		ShowLabels:  false,
		LabelOffset: 8,
		SizeScale:   nil,
		ColorScale:  nil,
	}
}

// PointSeries renders scatter/point chart series.
type PointSeries struct {
	builder *SVGBuilder
	config  PointSeriesConfig
}

// NewPointSeries creates a new point series renderer.
func NewPointSeries(builder *SVGBuilder, config PointSeriesConfig) *PointSeries {
	return &PointSeries{
		builder: builder,
		config:  config,
	}
}

// DrawLinear draws points for linear scales on both axes.
func (ps *PointSeries) DrawLinear(points []DataPoint, xScale, yScale *LinearScale) *PointSeries {
	if len(points) == 0 {
		return ps
	}

	b := ps.builder
	style := b.StyleGuide()

	b.Push()

	for _, point := range points {
		x := xScale.Scale(point.X)
		y := yScale.Scale(point.Y)
		size := ps.getPointSize(point)
		color := ps.getPointColor(point, style.Palette)

		ps.drawPoint(x, y, size, color)

		if ps.config.ShowLabels && point.Label != "" {
			b.SetFontSize(style.Typography.SizeSmall)
			labelX := x + ps.config.LabelOffset
			b.DrawText(point.Label, labelX, y, TextAlignLeft, TextBaselineMiddle)
		}
	}

	b.Pop()
	return ps
}

// DrawCategorical draws points for categorical x-scale and linear y-scale.
func (ps *PointSeries) DrawCategorical(points []DataPoint, xScale *CategoricalScale, yScale *LinearScale) *PointSeries {
	if len(points) == 0 {
		return ps
	}

	b := ps.builder
	style := b.StyleGuide()

	b.Push()

	for _, point := range points {
		x := xScale.Scale(point.XCategory)
		y := yScale.Scale(point.Y)
		size := ps.getPointSize(point)
		color := ps.getPointColor(point, style.Palette)

		ps.drawPoint(x, y, size, color)

		if ps.config.ShowLabels && point.Label != "" {
			b.SetFontSize(style.Typography.SizeSmall)
			labelX := x + ps.config.LabelOffset
			b.DrawText(point.Label, labelX, y, TextAlignLeft, TextBaselineMiddle)
		}
	}

	b.Pop()
	return ps
}

// getPointSize returns the size for a point, optionally scaled by value.
func (ps *PointSeries) getPointSize(point DataPoint) float64 {
	if ps.config.SizeScale != nil {
		// Scale size based on value
		scaledValue := ps.config.SizeScale.Scale(point.Value)
		return ps.config.Size * 0.5 * (1 + scaledValue) // Range from 0.5x to 1.5x base size
	}
	return ps.config.Size
}

// getPointColor returns the color for a point, optionally based on value.
func (ps *PointSeries) getPointColor(point DataPoint, palette *Palette) Color {
	if ps.config.ColorScale != nil {
		// Map value to color index
		scaledValue := ps.config.ColorScale.Scale(point.Value)
		index := int(scaledValue * 5) // Use 6 accent colors
		if index < 0 {
			index = 0
		}
		if index > 5 {
			index = 5
		}
		return palette.AccentColor(index).WithAlpha(ps.config.Opacity)
	}
	return ps.config.Color.WithAlpha(ps.config.Opacity)
}

// drawPoint draws a single point.
func (ps *PointSeries) drawPoint(x, y, size float64, color Color) {
	b := ps.builder
	radius := size / 2

	b.Push()
	b.SetFillColor(color)
	b.SetStrokeColor(ps.config.StrokeColor)
	b.SetStrokeWidth(ps.config.StrokeWidth)

	switch ps.config.Shape {
	case MarkerCircle:
		b.DrawCircle(x, y, radius)

	case MarkerSquare:
		b.DrawRect(Rect{X: x - radius, Y: y - radius, W: size, H: size})

	case MarkerDiamond:
		points := []Point{
			{X: x, Y: y - radius},
			{X: x + radius, Y: y},
			{X: x, Y: y + radius},
			{X: x - radius, Y: y},
		}
		b.DrawPolygon(points)

	case MarkerTriangle:
		h := radius * math.Sqrt(3)
		points := []Point{
			{X: x, Y: y - radius},
			{X: x + h/2, Y: y + radius/2},
			{X: x - h/2, Y: y + radius/2},
		}
		b.DrawPolygon(points)

	case MarkerTriangleDown:
		h := radius * math.Sqrt(3)
		points := []Point{
			{X: x, Y: y + radius},
			{X: x + h/2, Y: y - radius/2},
			{X: x - h/2, Y: y - radius/2},
		}
		b.DrawPolygon(points)

	default:
		b.DrawCircle(x, y, radius)
	}

	b.Pop()
}

// =============================================================================
// Arc Series (Pie/Donut)
// =============================================================================

// ArcSeriesConfig holds configuration for rendering pie/donut charts.
type ArcSeriesConfig struct {
	// CenterX is the center x-coordinate.
	CenterX float64

	// CenterY is the center y-coordinate.
	CenterY float64

	// OuterRadius is the outer radius.
	OuterRadius float64

	// InnerRadius is the inner radius (0 for pie, >0 for donut).
	InnerRadius float64

	// StartAngle is the starting angle in degrees (0 = top).
	StartAngle float64

	// PadAngle is the padding angle between slices in degrees.
	PadAngle float64

	// CornerRadius is the corner radius for rounded arcs.
	CornerRadius float64

	// StrokeColor is the stroke color between slices.
	StrokeColor Color

	// StrokeWidth is the stroke width between slices.
	StrokeWidth float64

	// ShowLabels enables labels on/near slices.
	ShowLabels bool

	// LabelPosition determines where labels are placed.
	LabelPosition ArcLabelPosition

	// LabelFormat is the format string for labels.
	LabelFormat string

	// ExplodeOffset is the offset for exploded slices.
	ExplodeOffset float64

	// ExplodedSlices is a list of slice indices to explode.
	ExplodedSlices []int

	// Colors is the color palette for slices.
	Colors []Color

	// SortSlices sorts slices by value (descending).
	SortSlices bool
}

// ArcLabelPosition specifies where arc labels are placed.
type ArcLabelPosition int

const (
	ArcLabelInside ArcLabelPosition = iota
	ArcLabelOutside
	ArcLabelNone
)

// DefaultArcSeriesConfig returns default arc series configuration.
func DefaultArcSeriesConfig(centerX, centerY, radius float64) ArcSeriesConfig {
	return ArcSeriesConfig{
		CenterX:        centerX,
		CenterY:        centerY,
		OuterRadius:    radius,
		InnerRadius:    0, // Pie chart by default
		StartAngle:     -90,
		PadAngle:       1,
		CornerRadius:   0,
		StrokeColor:    MustParseColor("#FFFFFF"),
		StrokeWidth:    2,
		ShowLabels:     true,
		LabelPosition:  ArcLabelInside,
		LabelFormat:    "%.1f%%",
		ExplodeOffset:  10,
		ExplodedSlices: nil,
		Colors:         nil, // Use palette
		SortSlices:     false,
	}
}

// ArcSlice represents a single slice in a pie/donut chart.
type ArcSlice struct {
	// Value is the slice value.
	Value float64

	// Label is the slice label.
	Label string

	// Color overrides the default color for this slice.
	Color *Color

	// Exploded indicates if this slice should be exploded.
	Exploded bool
}

// ArcSeries renders pie and donut chart series.
type ArcSeries struct {
	builder *SVGBuilder
	config  ArcSeriesConfig
}

// NewArcSeries creates a new arc series renderer.
func NewArcSeries(builder *SVGBuilder, config ArcSeriesConfig) *ArcSeries {
	return &ArcSeries{
		builder: builder,
		config:  config,
	}
}

// Draw draws the arc series from slice data.
func (as *ArcSeries) Draw(slices []ArcSlice) *ArcSeries {
	if len(slices) == 0 {
		return as
	}

	b := as.builder
	style := b.StyleGuide()

	// Calculate total value
	total := 0.0
	for _, s := range slices {
		total += s.Value
	}
	if total == 0 {
		return as
	}

	// Get colors from config or palette
	colors := as.config.Colors
	if len(colors) == 0 {
		colors = style.Palette.AccentColors()
	}

	// Calculate angles
	currentAngle := as.config.StartAngle
	padAngleRad := as.config.PadAngle * math.Pi / 180

	b.Push()

	// Determine minimum sweep angle for showing labels to prevent overlap.
	// With many slices, small-percentage labels merge into an unreadable mess.
	minLabelSweep := 0.0
	if len(slices) >= 15 {
		minLabelSweep = 18.0 // ~5% of 360°
	} else if len(slices) >= 10 {
		minLabelSweep = 12.0 // ~3.3%
	}

	for i, slice := range slices {
		// Calculate arc angles
		sweepAngle := (slice.Value / total) * 360

		// Check if exploded
		exploded := slice.Exploded
		for _, idx := range as.config.ExplodedSlices {
			if idx == i {
				exploded = true
				break
			}
		}

		// Get color
		color := colors[i%len(colors)]
		if slice.Color != nil {
			color = *slice.Color
		}

		// Calculate center offset for exploded slices
		centerX := as.config.CenterX
		centerY := as.config.CenterY
		if exploded && as.config.ExplodeOffset > 0 {
			midAngle := (currentAngle + sweepAngle/2) * math.Pi / 180
			centerX += as.config.ExplodeOffset * math.Cos(midAngle)
			centerY += as.config.ExplodeOffset * math.Sin(midAngle)
		}

		// Draw the arc
		as.drawArc(centerX, centerY, currentAngle, sweepAngle, color, padAngleRad)

		// Draw label — skip for tiny slices to prevent overlap
		if as.config.ShowLabels && as.config.LabelPosition != ArcLabelNone && sweepAngle >= minLabelSweep {
			as.drawArcLabel(centerX, centerY, currentAngle, sweepAngle, slice, total)
		}

		currentAngle += sweepAngle
	}

	b.Pop()
	return as
}

// DrawFromValues draws arcs from simple value slice.
func (as *ArcSeries) DrawFromValues(values []float64, labels []string) *ArcSeries {
	slices := make([]ArcSlice, len(values))
	for i, v := range values {
		slices[i] = ArcSlice{
			Value: v,
		}
		if i < len(labels) {
			slices[i].Label = labels[i]
		}
	}
	return as.Draw(slices)
}

// drawArc draws a single arc segment.
func (as *ArcSeries) drawArc(centerX, centerY, startAngle, sweepAngle float64, color Color, padAngle float64) {
	b := as.builder

	// Adjust for padding
	effectiveStartAngle := startAngle + (padAngle/2)*180/math.Pi
	effectiveSweepAngle := sweepAngle - padAngle*180/math.Pi
	if effectiveSweepAngle < 0 {
		effectiveSweepAngle = 0
	}

	// Convert to radians
	startRad := effectiveStartAngle * math.Pi / 180
	endRad := (effectiveStartAngle + effectiveSweepAngle) * math.Pi / 180

	// Calculate arc points
	outerR := as.config.OuterRadius
	innerR := as.config.InnerRadius

	b.SetFillColor(color)
	if as.config.StrokeWidth > 0 {
		b.SetStrokeColor(as.config.StrokeColor)
		b.SetStrokeWidth(as.config.StrokeWidth)
	}

	// Build path
	path := b.BeginPath()

	// Outer arc start point
	outerStartX := centerX + outerR*math.Cos(startRad)
	outerStartY := centerY + outerR*math.Sin(startRad)

	if innerR > 0 {
		// Donut: start from inner arc
		innerStartX := centerX + innerR*math.Cos(startRad)
		innerStartY := centerY + innerR*math.Sin(startRad)
		path.MoveTo(innerStartX, innerStartY)
		path.LineTo(outerStartX, outerStartY)
	} else {
		// Pie: start from center
		path.MoveTo(centerX, centerY)
		path.LineTo(outerStartX, outerStartY)
	}

	// Outer arc
	largeArc := effectiveSweepAngle > 180
	outerEndX := centerX + outerR*math.Cos(endRad)
	outerEndY := centerY + outerR*math.Sin(endRad)
	path.ArcTo(outerR, outerR, 0, largeArc, true, outerEndX, outerEndY)

	if innerR > 0 {
		// Line to inner arc end
		innerEndX := centerX + innerR*math.Cos(endRad)
		innerEndY := centerY + innerR*math.Sin(endRad)
		path.LineTo(innerEndX, innerEndY)

		// Inner arc (reverse direction)
		innerStartX := centerX + innerR*math.Cos(startRad)
		innerStartY := centerY + innerR*math.Sin(startRad)
		path.ArcTo(innerR, innerR, 0, largeArc, false, innerStartX, innerStartY)
	} else {
		// Line back to center
		path.LineTo(centerX, centerY)
	}

	path.Close()
	path.Draw()
}

// drawArcLabel draws a label for an arc slice.
func (as *ArcSeries) drawArcLabel(centerX, centerY, startAngle, sweepAngle float64, slice ArcSlice, total float64) {
	b := as.builder
	style := b.StyleGuide()

	midAngle := (startAngle + sweepAngle/2) * math.Pi / 180

	var labelRadius float64
	if as.config.LabelPosition == ArcLabelInside {
		// Place label at midpoint between inner and outer radius
		labelRadius = (as.config.InnerRadius + as.config.OuterRadius) / 2
		if as.config.InnerRadius == 0 {
			labelRadius = as.config.OuterRadius * 0.6
		}
	} else {
		// Outside label
		labelRadius = as.config.OuterRadius + style.Spacing.MD
	}

	labelX := centerX + labelRadius*math.Cos(midAngle)
	labelY := centerY + labelRadius*math.Sin(midAngle)

	// Format label. For outside labels, show only the percentage value
	// since the legend already displays segment names. This prevents
	// long labels from overflowing the SVG viewBox when rendered in
	// LibreOffice (which uses wider font metrics than the Go canvas library).
	percentage := (slice.Value / total) * 100
	pctStr := formatValue(percentage, as.config.LabelFormat)
	var label string
	if as.config.LabelPosition == ArcLabelOutside {
		label = pctStr
	} else if slice.Label != "" {
		label = slice.Label + " " + pctStr
	} else {
		label = pctStr
	}

	// Determine text alignment based on position
	var align TextAlign
	if as.config.LabelPosition == ArcLabelInside {
		align = TextAlignCenter
	} else {
		// Outside: align based on which side of the chart
		if math.Cos(midAngle) >= 0 {
			align = TextAlignLeft
			labelX += style.Spacing.XS
		} else {
			align = TextAlignRight
			labelX -= style.Spacing.XS
		}
	}

	// Set font size based on position
	fontSize := style.Typography.SizeSmall
	if as.config.LabelPosition == ArcLabelInside {
		b.SetFontSize(fontSize)
	} else {
		fontSize = style.Typography.SizeBody
		b.SetFontSize(fontSize)
	}
	b.SetFontWeight(style.Typography.WeightNormal)

	// Measure actual label dimensions for boundary clamping.
	// Apply 30% safety margin because LibreOffice renders SVG text wider
	// than the Go canvas library measures (different font metrics engines).
	labelWidth, labelHeight := b.MeasureText(label)
	if labelWidth == 0 {
		// Fallback if font not loaded
		labelWidth = float64(len(label)) * fontSize * 0.6
		labelHeight = fontSize
	}
	labelWidth *= 1.4 // safety margin for LibreOffice font metric discrepancy

	// Clamp label position to canvas boundaries with padding
	canvasWidth := b.Width()
	canvasHeight := b.Height()
	padding := style.Spacing.XS

	// Adjust for text alignment when calculating bounds
	var minX, maxX float64
	switch align {
	case TextAlignLeft:
		minX = padding
		maxX = canvasWidth - labelWidth - padding
	case TextAlignRight:
		minX = labelWidth + padding
		maxX = canvasWidth - padding
	default: // TextAlignCenter
		minX = labelWidth/2 + padding
		maxX = canvasWidth - labelWidth/2 - padding
	}

	// Clamp X position
	if labelX < minX {
		labelX = minX
	} else if labelX > maxX {
		labelX = maxX
	}

	// Clamp Y position (account for vertical centering)
	minY := labelHeight/2 + padding
	maxY := canvasHeight - labelHeight/2 - padding
	if labelY < minY {
		labelY = minY
	} else if labelY > maxY {
		labelY = maxY
	}

	b.DrawText(label, labelX, labelY, align, TextBaselineMiddle)
}

// =============================================================================
// Utility Functions
// =============================================================================

// formatValue formats a numeric value using the given format string.
func formatValue(value float64, format string) string {
	if format == "" {
		format = "%.0f"
	}
	return fmt.Sprintf(format, value)
}
