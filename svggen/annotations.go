package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Annotation Types
// =============================================================================

// AnnotationKind identifies the type of chart annotation.
type AnnotationKind string

const (
	// AnnotationReferenceLine draws a horizontal or vertical reference line
	// (e.g., a "Target: 95%" dashed line).
	AnnotationReferenceLine AnnotationKind = "reference_line"

	// AnnotationTrendline overlays a linear regression line on a series.
	AnnotationTrendline AnnotationKind = "trendline"

	// AnnotationCallout draws a callout arrow pointing to a data point with a label.
	AnnotationCallout AnnotationKind = "callout"
)

// Annotation represents a chart annotation such as a reference line,
// trendline, or callout arrow.
type Annotation struct {
	// Kind is the annotation type.
	Kind AnnotationKind

	// Axis is the axis to draw on ("x" or "y"). Used by reference_line.
	Axis string

	// Value is the axis value for the reference line.
	Value float64

	// Label is the text label for the annotation.
	Label string

	// Style is the line style ("solid", "dashed", "dotted"). Default: "dashed".
	Style string

	// Color overrides the annotation color. If nil, uses the style guide's TextSecondary.
	Color *Color

	// Series is the series name for trendlines.
	Series string

	// Method is the trendline method ("linear"). Default: "linear".
	Method string

	// X is the x-coordinate (category index) for callouts.
	X float64

	// Y is the y-coordinate value for callouts.
	Y float64

	// Text is the callout text. If empty, Label is used.
	Text string
}

// =============================================================================
// Data Label Configuration
// =============================================================================

// DataLabelConfig controls how data labels are formatted and displayed.
type DataLabelConfig struct {
	// Format is a Go fmt-style format string for values (e.g., "$%.1fM", "%.0f%%").
	Format string

	// ShowOn controls which data points get labels: "all", "last", "peaks", "first_last".
	ShowOn string
}

// FormatValue formats a numeric value using the configured format string.
// Falls back to "%.0f" if no format is set.
func (dlc DataLabelConfig) FormatValue(v float64) string {
	if dlc.Format == "" {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf(dlc.Format, v)
}

// ShouldShow returns true if a data label should be shown for the given index.
func (dlc DataLabelConfig) ShouldShow(index, count int, values []float64) bool {
	switch dlc.ShowOn {
	case "last":
		return index == count-1
	case "first_last":
		return index == 0 || index == count-1
	case "peaks":
		return isPeak(index, values)
	case "all", "":
		return true
	default:
		return true
	}
}

// isPeak returns true if the value at index is a local maximum or minimum.
func isPeak(index int, values []float64) bool {
	if len(values) < 2 {
		return true
	}
	if index == 0 || index == len(values)-1 {
		return true
	}
	v := values[index]
	prev := values[index-1]
	next := values[index+1]
	// Local max or local min
	return (v >= prev && v >= next) || (v <= prev && v <= next)
}

// =============================================================================
// Annotation Rendering
// =============================================================================

// DrawAnnotations renders all annotations onto the chart plot area.
// It accepts the plot area, scales, and chart data for coordinate mapping.
func DrawAnnotations(b *SVGBuilder, annotations []Annotation, plotArea Rect,
	xScale Scale, yScale *LinearScale, data ChartData, colors []Color) {
	for _, ann := range annotations {
		switch ann.Kind {
		case AnnotationReferenceLine:
			drawReferenceLine(b, ann, plotArea, xScale, yScale)
		case AnnotationTrendline:
			drawTrendline(b, ann, plotArea, xScale, yScale, data, colors)
		case AnnotationCallout:
			drawCallout(b, ann, plotArea, xScale, yScale)
		}
	}
}

// drawReferenceLine draws a horizontal or vertical reference line across the plot area.
func drawReferenceLine(b *SVGBuilder, ann Annotation, plotArea Rect,
	xScale Scale, yScale *LinearScale) {
	style := b.StyleGuide()

	lineColor := style.Palette.TextSecondary
	if ann.Color != nil {
		lineColor = *ann.Color
	}

	b.Push()
	b.SetStrokeColor(lineColor)
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	applyLineStyle(b, ann.Style)
	b.SetFillColor(Color{A: 0}) // no fill

	switch ann.Axis {
	case "x":
		// Vertical reference line at the given x value
		var x float64
		switch xs := xScale.(type) {
		case *CategoricalScale:
			catIdx := int(ann.Value)
			cats := xs.Categories()
			if catIdx >= 0 && catIdx < len(cats) {
				x = plotArea.X + xs.Scale(cats[catIdx])
			} else {
				b.Pop()
				return
			}
		case *LinearScale:
			x = plotArea.X + xs.Scale(ann.Value)
		default:
			b.Pop()
			return
		}
		b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)

		// Draw label at top of the line
		if ann.Label != "" {
			b.SetFillColor(lineColor)
			b.SetFontSize(style.Typography.SizeSmall)
			b.DrawText(ann.Label, x+4, plotArea.Y+style.Typography.SizeSmall, TextAlignLeft, TextBaselineAlphabetic)
		}
	default: // "y" or unspecified defaults to horizontal
		y := plotArea.Y + yScale.Scale(ann.Value)
		b.DrawLine(plotArea.X, y, plotArea.X+plotArea.W, y)

		// Draw label at the right end of the line
		if ann.Label != "" {
			b.SetFillColor(lineColor)
			b.SetFontSize(style.Typography.SizeSmall)
			b.DrawText(ann.Label, plotArea.X+plotArea.W-4, y-4, TextAlignRight, TextBaselineAlphabetic)
		}
	}

	b.Pop()
}

// drawTrendline draws a linear regression overlay on a specific series.
func drawTrendline(b *SVGBuilder, ann Annotation, plotArea Rect,
	xScale Scale, yScale *LinearScale, data ChartData, colors []Color) {
	style := b.StyleGuide()

	// Find the series by name
	seriesIdx := -1
	for i, s := range data.Series {
		if s.Name == ann.Series {
			seriesIdx = i
			break
		}
	}
	if seriesIdx < 0 || len(data.Series[seriesIdx].Values) < 2 {
		return
	}

	values := data.Series[seriesIdx].Values
	n := len(values)

	// Linear regression: y = slope*x + intercept
	slope, intercept := linearRegression(values)

	// Determine line color
	lineColor := style.Palette.TextSecondary
	if ann.Color != nil {
		lineColor = *ann.Color
	} else if seriesIdx < len(colors) {
		// Use a darker/lighter version of the series color
		lineColor = colors[seriesIdx]
	}

	// Calculate start and end points
	y0 := intercept
	y1 := slope*float64(n-1) + intercept

	var x0px, x1px float64
	switch xs := xScale.(type) {
	case *CategoricalScale:
		cats := xs.Categories()
		if len(cats) >= 2 {
			x0px = plotArea.X + xs.Scale(cats[0])
			x1px = plotArea.X + xs.Scale(cats[len(cats)-1])
		} else {
			return
		}
	case *LinearScale:
		x0px = plotArea.X + xs.Scale(0)
		x1px = plotArea.X + xs.Scale(float64(n-1))
	default:
		return
	}

	y0px := plotArea.Y + yScale.Scale(y0)
	y1px := plotArea.Y + yScale.Scale(y1)

	b.Push()
	b.SetStrokeColor(lineColor)
	b.SetStrokeWidth(style.Strokes.WidthNormal * 1.5)
	lineStyle := ann.Style
	if lineStyle == "" {
		lineStyle = "dashed"
	}
	applyLineStyle(b, lineStyle)
	b.DrawLine(x0px, y0px, x1px, y1px)

	// Draw label
	if ann.Label != "" {
		b.SetFillColor(lineColor)
		b.SetFontSize(style.Typography.SizeSmall)
		b.DrawText(ann.Label, x1px+4, y1px, TextAlignLeft, TextBaselineMiddle)
	}

	b.Pop()
}

// drawCallout draws a callout arrow pointing to a data point with a label.
func drawCallout(b *SVGBuilder, ann Annotation, plotArea Rect,
	xScale Scale, yScale *LinearScale) {
	style := b.StyleGuide()

	lineColor := style.Palette.TextPrimary
	if ann.Color != nil {
		lineColor = *ann.Color
	}

	// Calculate the target point
	var targetX float64
	catIdx := int(ann.X)
	switch xs := xScale.(type) {
	case *CategoricalScale:
		cats := xs.Categories()
		if catIdx >= 0 && catIdx < len(cats) {
			targetX = plotArea.X + xs.Scale(cats[catIdx])
		} else {
			return
		}
	case *LinearScale:
		targetX = plotArea.X + xs.Scale(ann.X)
	default:
		return
	}

	targetY := plotArea.Y + yScale.Scale(ann.Y)

	// Place the callout label offset from the target point
	// Prefer upper-right placement
	text := ann.Text
	if text == "" {
		text = ann.Label
	}
	if text == "" {
		return
	}

	// Offset the callout text above and to the right
	offsetX := 30.0
	offsetY := -30.0
	labelX := targetX + offsetX
	labelY := targetY + offsetY

	// Clamp label within plot area
	if labelX+80 > plotArea.X+plotArea.W {
		offsetX = -30
		labelX = targetX + offsetX
	}
	if labelY < plotArea.Y+10 {
		offsetY = 30
		labelY = targetY + offsetY
	}

	b.Push()

	// Draw connecting line from label to target
	b.SetStrokeColor(lineColor)
	b.SetStrokeWidth(1)
	b.DrawLine(labelX, labelY, targetX, targetY)

	// Draw a small circle at the target point
	b.SetFillColor(lineColor)
	b.DrawCircle(targetX, targetY, 3)

	// Draw the label text
	b.SetFontSize(style.Typography.SizeSmall)
	align := TextAlignLeft
	if offsetX < 0 {
		align = TextAlignRight
	}
	b.DrawText(text, labelX, labelY-4, align, TextBaselineAlphabetic)

	b.Pop()
}

// =============================================================================
// Helpers
// =============================================================================

// applyLineStyle sets the dash pattern based on style name.
func applyLineStyle(b *SVGBuilder, styleName string) {
	switch styleName {
	case "dotted":
		b.SetDashes(2, 4)
	case "solid":
		b.SetDashes() // solid
	case "dashed", "":
		b.SetDashes(6, 4)
	default:
		b.SetDashes(6, 4) // default to dashed
	}
}

// linearRegression computes the slope and intercept for y = slope*x + intercept
// where x values are 0, 1, 2, ..., n-1.
func linearRegression(values []float64) (slope, intercept float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, values[0]
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}
