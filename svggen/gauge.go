package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Gauge Chart
// =============================================================================

// GaugeChartConfig holds configuration for gauge charts.
type GaugeChartConfig struct {
	ChartConfig

	// MinValue is the minimum scale value.
	MinValue float64

	// MaxValue is the maximum scale value.
	MaxValue float64

	// StartAngle is the start angle in degrees (default: -135 for half-circle).
	StartAngle float64

	// EndAngle is the end angle in degrees (default: 135 for half-circle).
	EndAngle float64

	// InnerRadius is the inner radius ratio (0-1, creates a donut shape).
	InnerRadius float64

	// ShowTicks enables tick marks on the scale.
	ShowTicks bool

	// TickCount is the number of major tick marks.
	TickCount int

	// ShowTickLabels enables labels on tick marks.
	ShowTickLabels bool

	// Thresholds define colored zones on the gauge.
	Thresholds []GaugeThreshold

	// NeedleStyle controls needle appearance.
	NeedleStyle GaugeNeedleStyle

	// ShowNeedle controls whether the needle is displayed.
	ShowNeedle bool

	// CenterLabel is the label shown at the center (e.g., current value).
	ShowCenterLabel bool
}

// GaugeThreshold defines a colored zone on the gauge.
type GaugeThreshold struct {
	// Value is the threshold value (zone extends from previous threshold to this value).
	Value float64

	// Color is the zone color.
	Color Color

	// Label is an optional label for this zone.
	Label string
}

// GaugeNeedleStyle controls needle appearance.
type GaugeNeedleStyle struct {
	// Width is the needle width at the base.
	Width float64

	// Length is the needle length ratio (0-1 of outer radius).
	Length float64

	// Color is the needle color.
	Color Color

	// ShowPivot shows a center pivot circle.
	ShowPivot bool

	// PivotRadius is the pivot circle radius.
	PivotRadius float64
}

// DefaultGaugeNeedleStyle returns the default needle style.
func DefaultGaugeNeedleStyle() GaugeNeedleStyle {
	return GaugeNeedleStyle{
		Width:       12,
		Length:      0.85,
		Color:       MustParseColor("#2C3E50"),
		ShowPivot:   true,
		PivotRadius: 12,
	}
}

// DefaultGaugeChartConfig returns default gauge chart configuration.
// Angles are in degrees using SVG coordinate system (0° = right, 90° = down).
// For a standard speedometer-style gauge with 0 on the left and 100 on the right:
// - StartAngle: 135° (lower-left, 7:30 position)
// - EndAngle: 405° (lower-right via top, 4:30 position = 45° + 360°)
// This creates a 270° arc sweeping counter-clockwise through the top.
func DefaultGaugeChartConfig(width, height float64) GaugeChartConfig {
	return GaugeChartConfig{
		ChartConfig:     DefaultChartConfig(width, height),
		MinValue:        0,
		MaxValue:        100,
		StartAngle:      135,
		EndAngle:        405,
		InnerRadius:     0.6,
		ShowTicks:       true,
		TickCount:       5,
		ShowTickLabels:  true,
		NeedleStyle:     DefaultGaugeNeedleStyle(),
		ShowNeedle:      true,
		ShowCenterLabel: true,
	}
}

// GaugeData represents the data for a gauge chart.
type GaugeData struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Value is the current value to display.
	Value float64

	// Label is an optional label for the value.
	Label string

	// Unit is an optional unit suffix (e.g., "%", "mph").
	Unit string

	// Footnote is an optional footnote text.
	Footnote string
}

// GaugeChart renders gauge/speedometer charts.
type GaugeChart struct {
	builder *SVGBuilder
	config  GaugeChartConfig
}

// NewGaugeChart creates a new gauge chart renderer.
func NewGaugeChart(builder *SVGBuilder, config GaugeChartConfig) *GaugeChart {
	return &GaugeChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the gauge chart.
func (gc *GaugeChart) Draw(data GaugeData) error {
	b := gc.builder
	style := b.StyleGuide()

	// Apply theme colors if needle still has default hardcoded color.
	gc.applyThemeColors()

	// Calculate plot area
	plotArea := gc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if gc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight

	// Calculate center and radius based on actual arc geometry.
	// For a 270° gauge (135°→405°), the arc is asymmetric:
	//   - Top extent from center: radius (arc passes straight up at 270°)
	//   - Bottom extent from center: radius*sin(45°) ≈ radius*0.707
	// Plus tick labels extend (radius + tickLabelPad) beyond the arc.
	// We solve for the largest radius that fits within the plot area.

	centerX := plotArea.X + plotArea.W/2

	// Convert angles to radians (used for both extent calc and drawing).
	startAngleRad := gc.config.StartAngle * math.Pi / 180
	endAngleRad := gc.config.EndAngle * math.Pi / 180

	// Find the max vertical extent above and below the center.
	// Sample key angles: start, end, and cardinal directions within sweep.
	topExtent := 0.0  // max distance above center (negative Y in math coords)
	botExtent := 0.0  // max distance below center (positive Y in math coords)
	sweep := endAngleRad - startAngleRad
	steps := 36
	for i := 0; i <= steps; i++ {
		angle := startAngleRad + sweep*float64(i)/float64(steps)
		sy := math.Sin(angle)
		if sy < -topExtent {
			topExtent = -sy // sin < 0 means above center
		}
		if sy > botExtent {
			botExtent = sy
		}
	}
	if topExtent < 0.01 {
		topExtent = 0.01
	}
	if botExtent < 0.01 {
		botExtent = 0.01
	}

	// Tick labels and center label add padding below the arc.
	tickLabelPad := style.Spacing.MD + style.Typography.SizeSmall
	centerLabelPad := style.Spacing.LG + style.Typography.SizeTitle

	// Available height must contain: topExtent*r (above) + botExtent*r + tickLabelPad (below)
	// Also the center label at centerY + centerLabelPad must fit.
	// Compute max radius from width constraint (horizontal).
	maxRadiusW := (plotArea.W/2 - tickLabelPad) * 0.95

	// Compute max radius from height constraint:
	// plotArea.H >= topExtent*r + botExtent*r + tickLabelPad*botExtent/max(topExtent,botExtent)
	// Simplify: distribute available height proportionally.
	totalExtent := topExtent + botExtent
	availH := plotArea.H - tickLabelPad*1.2 // reserve space for tick labels below
	maxRadiusH := availH / totalExtent

	radius := math.Min(maxRadiusW, maxRadiusH) * 0.92

	// Position center so topExtent*radius from top edge, allowing tick labels.
	centerY := plotArea.Y + topExtent*radius + tickLabelPad*0.5
	// Also ensure the center label fits at the bottom.
	bottomNeeded := centerY + botExtent*radius + tickLabelPad
	plotBottom := plotArea.Y + plotArea.H
	if bottomNeeded > plotBottom {
		// Shift center up to make room
		centerY -= bottomNeeded - plotBottom
	}
	// Ensure center label below center also fits.
	if centerY+centerLabelPad > plotBottom {
		centerY = plotBottom - centerLabelPad
	}

	innerRadius := radius * gc.config.InnerRadius

	// Draw threshold zones or themed value arc
	if len(gc.config.Thresholds) > 0 {
		gc.drawThresholdZones(centerX, centerY, radius, innerRadius, startAngleRad, endAngleRad)
	} else {
		// Draw background arc, then a filled value arc in the primary accent color
		gc.drawBackgroundArc(centerX, centerY, radius, innerRadius, startAngleRad, endAngleRad)
		gc.drawValueArc(centerX, centerY, radius, innerRadius, startAngleRad, endAngleRad, data.Value)
	}

	// Draw ticks
	if gc.config.ShowTicks {
		gc.drawTicks(centerX, centerY, radius, innerRadius, startAngleRad, endAngleRad)
	}

	// Draw needle
	if gc.config.ShowNeedle {
		gc.drawNeedle(centerX, centerY, radius, startAngleRad, endAngleRad, data.Value)
	}

	// Draw center label positioned inside the donut hole, below the pivot.
	// Pass innerRadius so the label can be placed proportionally.
	if gc.config.ShowCenterLabel {
		gc.drawCenterLabel(centerX, centerY, innerRadius, data)
	}

	// Draw title
	if gc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: gc.config.Width, H: headerHeight + gc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		fh := FootnoteReservedHeight(style)
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: gc.config.Height - fh,
			W: gc.config.Width,
			H: fh,
		})
	}

	return nil
}

// applyThemeColors sets needle and pivot colors from the theme palette.
// Called at draw time so the builder's style guide is available.
func (gc *GaugeChart) applyThemeColors() {
	style := gc.builder.StyleGuide()

	// Use TextPrimary for the needle so it contrasts with the Accent1 value arc.
	// Previously used Accent1.Darken(0.25) which was nearly invisible against
	// the Accent1 arc on templates like forest-green.
	gc.config.NeedleStyle.Color = style.Palette.TextPrimary

	// Assign accent colors to any thresholds that have zero-value (unset) colors.
	accents := style.Palette.AccentColors()
	for i := range gc.config.Thresholds {
		if gc.config.Thresholds[i].Color == (Color{}) {
			gc.config.Thresholds[i].Color = accents[i%len(accents)]
		}
	}
}

// drawValueArc draws a filled arc from the start angle to the current value
// using the primary accent color, providing theme-aware visual feedback.
func (gc *GaugeChart) drawValueArc(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle, value float64) {
	b := gc.builder
	style := b.StyleGuide()

	// Clamp value to range
	if value < gc.config.MinValue {
		value = gc.config.MinValue
	}
	if value > gc.config.MaxValue {
		value = gc.config.MaxValue
	}

	totalRange := gc.config.MaxValue - gc.config.MinValue
	if totalRange <= 0 {
		return
	}

	ratio := (value - gc.config.MinValue) / totalRange
	valueAngle := startAngle + ratio*(endAngle-startAngle)

	// Don't draw if value is at minimum
	if ratio < 0.001 {
		return
	}

	b.Push()
	b.SetFillColor(style.Palette.Accent1)
	b.SetStrokeWidth(0)
	gc.drawArc(centerX, centerY, outerRadius, innerRadius, startAngle, valueAngle)
	b.Pop()
}

// drawBackgroundArc draws the gauge background arc as a stroked track
// (no fill) so the unlit region doesn't produce a gray wedge artifact.
func (gc *GaugeChart) drawBackgroundArc(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle float64) {
	b := gc.builder
	style := b.StyleGuide()

	// Draw a thick stroked arc along the centerline of the donut track
	// instead of a filled donut shape. This avoids the gray wedge artifact
	// that appeared when the filled background arc's unlit region was visible.
	arcWidth := outerRadius - innerRadius
	trackRadius := innerRadius + arcWidth/2

	// Use a track color with reliable contrast against the background.
	// The previous Background.Darken(0.08) was only 8% darker, producing
	// insufficient contrast on templates with light backgrounds. We blend
	// the Border color (designed for visibility) toward the background at
	// 30% strength, then verify the result meets WCAG AA for graphical
	// elements (3:1). EnsureContrast pushes the color further if needed.
	trackColor := lerpColors(style.Palette.Background.Opaque(), style.Palette.Border.Opaque(), 0.30)
	trackColor = EnsureContrast(trackColor, style.Palette.Background, WCAGAALarge)

	b.Push()
	b.SetStrokeColor(trackColor)
	b.SetStrokeWidth(arcWidth)

	startX := centerX + trackRadius*math.Cos(startAngle)
	startY := centerY + trackRadius*math.Sin(startAngle)
	endX := centerX + trackRadius*math.Cos(endAngle)
	endY := centerY + trackRadius*math.Sin(endAngle)
	largeArc := (endAngle - startAngle) > math.Pi

	path := b.BeginPath()
	path.MoveTo(startX, startY)
	path.ArcTo(trackRadius, trackRadius, 0, largeArc, true, endX, endY)
	path.Stroke()

	// Draw thin border on inner and outer edges
	b.SetStrokeColor(style.Palette.Border)
	b.SetStrokeWidth(1)

	outerStartX := centerX + outerRadius*math.Cos(startAngle)
	outerStartY := centerY + outerRadius*math.Sin(startAngle)
	outerEndX := centerX + outerRadius*math.Cos(endAngle)
	outerEndY := centerY + outerRadius*math.Sin(endAngle)

	outerPath := b.BeginPath()
	outerPath.MoveTo(outerStartX, outerStartY)
	outerPath.ArcTo(outerRadius, outerRadius, 0, largeArc, true, outerEndX, outerEndY)
	outerPath.Stroke()

	innerStartX := centerX + innerRadius*math.Cos(startAngle)
	innerStartY := centerY + innerRadius*math.Sin(startAngle)
	innerEndX := centerX + innerRadius*math.Cos(endAngle)
	innerEndY := centerY + innerRadius*math.Sin(endAngle)

	innerPath := b.BeginPath()
	innerPath.MoveTo(innerStartX, innerStartY)
	innerPath.ArcTo(innerRadius, innerRadius, 0, largeArc, true, innerEndX, innerEndY)
	innerPath.Stroke()

	b.Pop()
}

// drawThresholdZones draws colored zones on the gauge.
func (gc *GaugeChart) drawThresholdZones(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle float64) {
	b := gc.builder

	totalRange := gc.config.MaxValue - gc.config.MinValue
	totalAngle := endAngle - startAngle

	prevValue := gc.config.MinValue

	for _, threshold := range gc.config.Thresholds {
		// Calculate angle range for this zone
		startRatio := (prevValue - gc.config.MinValue) / totalRange
		endRatio := (threshold.Value - gc.config.MinValue) / totalRange

		zoneStartAngle := startAngle + startRatio*totalAngle
		zoneEndAngle := startAngle + endRatio*totalAngle

		b.Push()
		b.SetFillColor(threshold.Color)
		b.SetStrokeWidth(0)

		gc.drawArc(centerX, centerY, outerRadius, innerRadius, zoneStartAngle, zoneEndAngle)

		b.Pop()

		prevValue = threshold.Value
	}

	// Fill remaining area if thresholds don't cover the full range
	if prevValue < gc.config.MaxValue {
		startRatio := (prevValue - gc.config.MinValue) / totalRange
		zoneStartAngle := startAngle + startRatio*totalAngle

		style := b.StyleGuide()
		remainFill := lerpColors(style.Palette.Background.Opaque(), style.Palette.Border.Opaque(), 0.30)
		remainFill = EnsureContrast(remainFill, style.Palette.Background, WCAGAALarge)
		b.Push()
		b.SetFillColor(remainFill)
		b.SetStrokeWidth(0)

		gc.drawArc(centerX, centerY, outerRadius, innerRadius, zoneStartAngle, endAngle)

		b.Pop()
	}
}

// drawArc draws an arc segment (donut slice).
func (gc *GaugeChart) drawArc(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle float64) {
	b := gc.builder

	// Calculate arc points
	outerStartX := centerX + outerRadius*math.Cos(startAngle)
	outerStartY := centerY + outerRadius*math.Sin(startAngle)
	outerEndX := centerX + outerRadius*math.Cos(endAngle)
	outerEndY := centerY + outerRadius*math.Sin(endAngle)
	innerStartX := centerX + innerRadius*math.Cos(startAngle)
	innerStartY := centerY + innerRadius*math.Sin(startAngle)
	innerEndX := centerX + innerRadius*math.Cos(endAngle)
	innerEndY := centerY + innerRadius*math.Sin(endAngle)

	// Determine if arc is larger than 180 degrees
	largeArc := (endAngle - startAngle) > math.Pi

	path := b.BeginPath()
	path.MoveTo(innerStartX, innerStartY)
	path.LineTo(outerStartX, outerStartY)
	path.ArcTo(outerRadius, outerRadius, 0, largeArc, true, outerEndX, outerEndY)
	path.LineTo(innerEndX, innerEndY)
	path.ArcTo(innerRadius, innerRadius, 0, largeArc, false, innerStartX, innerStartY)
	path.Close()
	path.Fill()
}

// drawTicks draws tick marks on the gauge.
func (gc *GaugeChart) drawTicks(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle float64) {
	b := gc.builder
	style := b.StyleGuide()

	totalRange := gc.config.MaxValue - gc.config.MinValue
	totalAngle := endAngle - startAngle

	tickLength := (outerRadius - innerRadius) * 0.3
	minorTickLength := tickLength * 0.5

	b.Push()
	b.SetStrokeColor(style.Palette.TextPrimary)
	b.SetStrokeWidth(2)

	// Draw major ticks
	for i := 0; i <= gc.config.TickCount; i++ {
		ratio := float64(i) / float64(gc.config.TickCount)
		angle := startAngle + ratio*totalAngle

		// Calculate tick positions (from just inside outer edge)
		outerX := centerX + (outerRadius-2)*math.Cos(angle)
		outerY := centerY + (outerRadius-2)*math.Sin(angle)
		innerX := centerX + (outerRadius-tickLength)*math.Cos(angle)
		innerY := centerY + (outerRadius-tickLength)*math.Sin(angle)

		b.DrawLine(innerX, innerY, outerX, outerY)

		// Draw tick labels
		if gc.config.ShowTickLabels {
			value := gc.config.MinValue + ratio*totalRange
			labelRadius := outerRadius + style.Spacing.MD

			labelX := centerX + labelRadius*math.Cos(angle)
			labelY := centerY + labelRadius*math.Sin(angle)

			// Adjust alignment based on position
			var align TextAlign
			if math.Cos(angle) < -0.1 {
				align = TextAlignRight
			} else if math.Cos(angle) > 0.1 {
				align = TextAlignLeft
			} else {
				align = TextAlignCenter
			}

			b.SetFontSize(style.Typography.SizeSmall)
			label := formatValue(value, "%.0f")
			b.DrawText(label, labelX, labelY, align, TextBaselineMiddle)
		}
	}

	// Draw minor ticks (between major ticks)
	b.SetStrokeWidth(1)
	minorTicksPerMajor := 4
	for i := 0; i < gc.config.TickCount*minorTicksPerMajor; i++ {
		if i%(minorTicksPerMajor) == 0 {
			continue // Skip major tick positions
		}
		ratio := float64(i) / float64(gc.config.TickCount*minorTicksPerMajor)
		angle := startAngle + ratio*totalAngle

		outerX := centerX + (outerRadius-2)*math.Cos(angle)
		outerY := centerY + (outerRadius-2)*math.Sin(angle)
		innerX := centerX + (outerRadius-minorTickLength)*math.Cos(angle)
		innerY := centerY + (outerRadius-minorTickLength)*math.Sin(angle)

		b.DrawLine(innerX, innerY, outerX, outerY)
	}

	b.Pop()
}

// drawNeedle draws the gauge needle.
func (gc *GaugeChart) drawNeedle(centerX, centerY, radius, startAngle, endAngle, value float64) {
	b := gc.builder

	// Clamp value to range
	if value < gc.config.MinValue {
		value = gc.config.MinValue
	}
	if value > gc.config.MaxValue {
		value = gc.config.MaxValue
	}

	// Calculate needle angle
	totalRange := gc.config.MaxValue - gc.config.MinValue
	totalAngle := endAngle - startAngle
	ratio := (value - gc.config.MinValue) / totalRange
	needleAngle := startAngle + ratio*totalAngle

	needleLength := radius * gc.config.NeedleStyle.Length

	// Scale needle width and pivot radius proportionally to the gauge radius.
	// The default 12px values were designed for ~250px radius (800x600 canvas).
	// At smaller PPTX placeholder sizes (250-400pt), the fixed 12px pivot
	// dominated the center, obscuring the needle and value label.
	// Using ~4% of radius for needle width and ~5% for pivot keeps them
	// proportional at any size.
	needleWidth := radius * 0.04
	if needleWidth < 3 {
		needleWidth = 3
	}
	pivotRadius := radius * 0.05
	if pivotRadius < 4 {
		pivotRadius = 4
	}

	// Calculate needle tip
	tipX := centerX + needleLength*math.Cos(needleAngle)
	tipY := centerY + needleLength*math.Sin(needleAngle)

	// Calculate base points (perpendicular to needle direction)
	perpAngle := needleAngle + math.Pi/2
	baseX1 := centerX + needleWidth/2*math.Cos(perpAngle)
	baseY1 := centerY + needleWidth/2*math.Sin(perpAngle)
	baseX2 := centerX - needleWidth/2*math.Cos(perpAngle)
	baseY2 := centerY - needleWidth/2*math.Sin(perpAngle)

	// Draw needle
	b.Push()
	b.SetFillColor(gc.config.NeedleStyle.Color)
	b.SetStrokeWidth(0)

	points := []Point{
		{X: tipX, Y: tipY},
		{X: baseX1, Y: baseY1},
		{X: baseX2, Y: baseY2},
	}
	b.DrawPolygon(points)

	// Draw pivot
	if gc.config.NeedleStyle.ShowPivot {
		b.SetFillColor(gc.config.NeedleStyle.Color)
		b.SetStrokeColor(gc.config.NeedleStyle.Color.Lighten(0.2))
		b.SetStrokeWidth(math.Max(1, pivotRadius*0.15))
		b.DrawCircle(centerX, centerY, pivotRadius)
	}

	b.Pop()
}

// drawCenterLabel draws the value label at the center of the gauge.
// innerRadius is the donut hole radius, used to position the label
// proportionally inside the empty center area.
func (gc *GaugeChart) drawCenterLabel(centerX, centerY, innerRadius float64, data GaugeData) {
	b := gc.builder
	style := b.StyleGuide()

	// Position label inside the donut hole, below the pivot point.
	// Use a fraction of the inner radius so the label stays inside the
	// arc at any size (previously used a fixed Spacing.LG offset which
	// caused overlap with the pivot circle at small dimensions).
	pivotRadius := innerRadius * 0.08
	if pivotRadius < 4 {
		pivotRadius = 4
	}
	labelY := centerY + pivotRadius + style.Typography.SizeTitle*0.6

	b.Push()
	b.SetFontSize(style.Typography.SizeTitle)
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetTextColor(style.Palette.TextPrimary)

	// Format value with unit
	label := formatValue(data.Value, gc.config.ValueFormat)
	if data.Unit != "" {
		label = label + data.Unit
	}

	b.DrawText(label, centerX, labelY, TextAlignCenter, TextBaselineMiddle)

	// Draw secondary label if provided
	if data.Label != "" {
		b.SetFontSize(style.Typography.SizeSmall)
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(style.Palette.TextSecondary)
		b.DrawText(data.Label, centerX, labelY+style.Typography.SizeTitle, TextAlignCenter, TextBaselineMiddle)
	}

	b.Pop()
}

// =============================================================================
// Gauge Chart Diagram Type (for Registry)
// =============================================================================

// GaugeDiagram implements the Diagram interface for gauge charts.
type GaugeDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for gauge charts.
func (d *GaugeDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("gauge chart requires data. Expected format: {\"value\": 75, \"min\": 0, \"max\": 100}")
	}

	// Check for value field
	if _, ok := req.Data["value"]; !ok {
		return fmt.Errorf("gauge chart requires 'value' field in data. Expected: {\"value\": 75} (optionally with \"min\", \"max\", \"thresholds\")")
	}

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *GaugeDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *GaugeDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseGaugeData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultGaugeChartConfig(width, height)
		config.ShowValues = req.Style.ShowValues

		// Apply custom min/max, tracking whether they were explicitly set.
		_, hasMin := req.Data["min"]
		_, hasMax := req.Data["max"]
		if minVal, ok := req.Data["min"].(float64); ok {
			config.MinValue = minVal
		}
		if maxVal, ok := req.Data["max"].(float64); ok {
			config.MaxValue = maxVal
		}

		// Auto-detect 0-1 scale: if no explicit min/max was provided and
		// the value is in [0,1], assume a 0-1 range instead of the default
		// 0-100. This prevents fractional values like 0.73 from rendering
		// as needle-near-zero on a 0-100 scale.
		if !hasMin && !hasMax && data.Value >= 0 && data.Value <= 1 {
			config.MinValue = 0
			config.MaxValue = 1
		}
		if startAngle, ok := req.Data["start_angle"].(float64); ok {
			config.StartAngle = startAngle
		}
		if endAngle, ok := req.Data["end_angle"].(float64); ok {
			config.EndAngle = endAngle
		}

		// Parse thresholds
		if thresholdsRaw, ok := req.Data["thresholds"].([]any); ok {
			config.Thresholds = parseThresholds(thresholdsRaw)
		}

		chart := NewGaugeChart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseGaugeData parses the request data into GaugeData.
func parseGaugeData(req *RequestEnvelope) (GaugeData, error) {
	data := GaugeData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse value
	if value, ok := req.Data["value"].(float64); ok {
		data.Value = value
	} else if value, ok := req.Data["value"].(int); ok {
		data.Value = float64(value)
	} else {
		return data, fmt.Errorf("invalid value format")
	}

	// Parse optional fields
	if label, ok := req.Data["label"].(string); ok {
		data.Label = label
	}
	if unit, ok := req.Data["unit"].(string); ok {
		data.Unit = unit
	}
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

// parseThresholds parses threshold definitions from request data.
func parseThresholds(raw []any) []GaugeThreshold {
	thresholds := make([]GaugeThreshold, 0, len(raw))

	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			t := GaugeThreshold{}

			if value, ok := m["value"].(float64); ok {
				t.Value = value
			} else if value, ok := m["value"].(int); ok {
				t.Value = float64(value)
			}

			if colorStr, ok := m["color"].(string); ok {
				if c, err := ParseColor(colorStr); err == nil {
					t.Color = c
				}
			}

			if label, ok := m["label"].(string); ok {
				t.Label = label
			}

			thresholds = append(thresholds, t)
		}
	}

	return thresholds
}

