package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Funnel Chart
// =============================================================================

// FunnelChartConfig holds configuration for funnel charts.
type FunnelChartConfig struct {
	ChartConfig

	// NeckWidth is the relative width of the funnel neck (0-1).
	// 0 = triangle (point at bottom), 1 = rectangle (no tapering).
	NeckWidth float64

	// NeckHeight is the relative height of the neck section (0-1).
	// Only applicable when NeckWidth > 0.
	NeckHeight float64

	// Gap is the spacing between segments in points.
	Gap float64

	// CornerRadius rounds segment corners.
	CornerRadius float64

	// LabelPosition determines where labels are placed.
	LabelPosition FunnelLabelPosition

	// ShowPercentage displays each stage as a percentage of the FIRST stage.
	ShowPercentage bool

	// ShowConversion adds the stage-to-stage conversion under each stage's
	// label — the number a funnel exists to show. Default true.
	ShowConversion bool

	// WidthMode decides how a stage's width follows its value:
	//
	//	"clamped"      — proportional, but floored so the last stage is still a
	//	                 shape (default)
	//	"proportional" — width exactly proportional to value
	//	"equal"        — every stage the same width
	//
	// A real SaaS funnel spans two orders of magnitude (12,400 -> 212), and
	// under "proportional" the bottom half is a 2px stick with external leader
	// lines (go-slide-creator-6i6j).
	WidthMode string
}

// FunnelLabelPosition determines label placement for funnel charts.
type FunnelLabelPosition string

const (
	// FunnelLabelInside places labels inside the segment.
	FunnelLabelInside FunnelLabelPosition = "inside"

	// FunnelLabelLeft places labels to the left of the segment.
	FunnelLabelLeft FunnelLabelPosition = "left"

	// FunnelLabelRight places labels to the right of the segment.
	FunnelLabelRight FunnelLabelPosition = "right"
)

// DefaultFunnelChartConfig returns default funnel chart configuration.
func DefaultFunnelChartConfig(width, height float64) FunnelChartConfig {
	cfg := DefaultChartConfig(width, height)
	// Funnel charts ALWAYS show values by default - the numeric values (e.g., 10000 → 3000 → 1500)
	// are essential to understanding conversion drop-off at each stage.
	// Without values, viewers only see relative widths with no quantitative context.
	cfg.ShowValues = true

	return FunnelChartConfig{
		ChartConfig:    cfg,
		NeckWidth:      0,
		NeckHeight:     0,
		Gap:            2,
		CornerRadius:   0,
		LabelPosition:  FunnelLabelInside,
		ShowPercentage: false,
		ShowConversion: true,
		WidthMode:      FunnelWidthClamped,
	}
}

// Funnel width modes.
const (
	FunnelWidthClamped      = "clamped"
	FunnelWidthProportional = "proportional"
	FunnelWidthEqual        = "equal"
)

// funnelClampedNeck is the bottom width the LAST stage keeps in clamped mode,
// as a fraction of its own top width, when the caller set no neck. Tapering
// the final stage to a point leaves nowhere to put its label, which is how
// "Won: 212" ended up on a leader line outside the chart.
const funnelClampedNeck = 0.55

// funnelMinWidthFrac is the share of the plot width the SMALLEST stage keeps in
// clamped mode. Width is interpolated between this floor and the full width by
// the stage's share of the maximum, so the ordering is preserved and the last
// stage is still a shape you can put a label in.
const funnelMinWidthFrac = 0.25

// funnelSegmentWidth is the drawn width of a stage. Both the draw loop and the
// external-label pre-pass read it, so the geometry they reason about cannot
// drift apart.
func funnelSegmentWidth(value, maxValue, plotW float64, mode string) float64 {
	if maxValue <= 0 {
		return 0
	}
	share := value / maxValue
	switch mode {
	case FunnelWidthEqual:
		return plotW
	case FunnelWidthProportional:
		return plotW * share
	default:
		return plotW * (funnelMinWidthFrac + (1-funnelMinWidthFrac)*share)
	}
}

// effectiveNeckWidth is the last stage's bottom width as a fraction of its top.
// In clamped mode an unset neck becomes a real one: the point a funnel
// traditionally tapers to is exactly where the smallest stage's label has to go.
func (fc *FunnelChart) effectiveNeckWidth() float64 {
	if fc.config.NeckWidth > 0 {
		return fc.config.NeckWidth
	}
	if fc.config.WidthMode == "" || fc.config.WidthMode == FunnelWidthClamped {
		return funnelClampedNeck
	}
	return fc.config.NeckWidth
}

// funnelConversionLabel is the stage-to-stage conversion, as "25% of Visitors".
// The first stage has nothing to convert from and returns "".
func funnelConversionLabel(points []FunnelDataPoint, i int) string {
	if i <= 0 || i >= len(points) {
		return ""
	}
	prev := points[i-1]
	if prev.Value <= 0 {
		return ""
	}
	pct := points[i].Value / prev.Value * 100
	switch {
	case pct >= 10:
		return fmt.Sprintf("%.0f%% of %s", pct, prev.Label)
	default:
		return fmt.Sprintf("%.1f%% of %s", pct, prev.Label)
	}
}

// FunnelDataPoint represents a single segment in the funnel chart.
type FunnelDataPoint struct {
	// Label is the segment label.
	Label string

	// Value is the segment value.
	Value float64

	// Color overrides the default color for this segment.
	Color *Color
}

// FunnelData represents the data for a funnel chart.
type FunnelData struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Points are the funnel segments (top to bottom).
	Points []FunnelDataPoint

	// Footnote is an optional footnote text.
	Footnote string
}

// FunnelChart renders funnel charts.
type FunnelChart struct {
	builder     *SVGBuilder
	config      FunnelChartConfig
	numSegments int // set during Draw for use by drawLabel
}

// NewFunnelChart creates a new funnel chart renderer.
func NewFunnelChart(builder *SVGBuilder, config FunnelChartConfig) *FunnelChart {
	return &FunnelChart{
		builder: builder,
		config:  config,
	}
}

// funnelAdaptiveFontSize returns font sizes scaled for the number of segments.
// For 1-5 segments the preset heading size is used. For 6+ segments the font
// is progressively reduced so labels remain legible without overlapping.
// The returned maxFont is never below DefaultMinFontSize.
func funnelAdaptiveFontSize(numSegments int, headingSize float64) float64 {
	if numSegments <= 5 {
		return headingSize
	}
	// Linear interpolation: at 6 segments use 85% of heading, at 10 use 65%.
	// Clamped to DefaultMinFontSize as the absolute floor.
	t := float64(numSegments-5) / 5.0 // 0 at 5, 1 at 10
	if t > 1 {
		t = 1
	}
	scaled := headingSize * (1.0 - 0.35*t) // 100% -> 65%
	return math.Max(DefaultMinFontSize, scaled)
}

// funnelAdaptiveGap returns the inter-segment gap scaled for segment count.
// With many segments the default gap eats too much vertical space, so we
// reduce it progressively while keeping at least 1pt.
func funnelAdaptiveGap(numSegments int, baseGap float64) float64 {
	if numSegments <= 5 {
		return baseGap
	}
	// Scale down: at 6 segments use 75%, at 10+ use 40%
	t := float64(numSegments-5) / 5.0
	if t > 1 {
		t = 1
	}
	scaled := baseGap * (1.0 - 0.6*t) // 100% -> 40%
	return math.Max(1.0, scaled)
}

// Draw renders the funnel chart.
func (fc *FunnelChart) Draw(data FunnelData) error {
	if len(data.Points) == 0 {
		return fmt.Errorf("funnel chart requires at least one data point")
	}

	fc.numSegments = len(data.Points)
	// Show enough decimals for the stage values to be distinct
	// (go-slide-creator-66qb).
	values := make([]float64, 0, len(data.Points))
	for _, p := range data.Points {
		values = append(values, p.Value)
	}
	fc.config.ValueFormat = autoValueFormat(fc.config.ValueFormat, values)
	fc.config.ResolveValueFormatter(values, false)

	b := fc.builder
	style := b.StyleGuide()
	colors := fc.getColors(style, len(data.Points))

	// Calculate plot area
	plotArea := fc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if fc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Adjust for labels if they're on the side
	labelPadding := 0.0
	if fc.config.LabelPosition == FunnelLabelLeft || fc.config.LabelPosition == FunnelLabelRight {
		labelPadding = 100 // Reserve space for labels
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight
	switch fc.config.LabelPosition {
	case FunnelLabelLeft:
		plotArea.X += labelPadding
		plotArea.W -= labelPadding
	case FunnelLabelRight:
		plotArea.W -= labelPadding
	}

	// Find the maximum value for scaling
	maxValue := 0.0
	for _, p := range data.Points {
		if p.Value > maxValue {
			maxValue = p.Value
		}
	}

	if maxValue == 0 {
		maxValue = 1
	}

	// Pre-pass: when using inside labels, check if any segment will need
	// external labels (text too wide for the narrowing trapezoid). If so,
	// shrink the plot area on the right to keep labels inside the viewBox.
	if fc.config.LabelPosition == FunnelLabelInside {
		plotArea = fc.reserveExternalLabelSpace(data, plotArea, maxValue, style)
	}

	// Calculate segment dimensions with adaptive gap for many stages
	numSegments := len(data.Points)
	effectiveGap := funnelAdaptiveGap(numSegments, fc.config.Gap)
	totalGaps := float64(numSegments-1) * effectiveGap
	segmentHeight := (plotArea.H - totalGaps) / float64(numSegments)

	centerX := plotArea.X + plotArea.W/2

	// Draw segments from top to bottom.
	// Each segment is a centered trapezoid whose top width corresponds to this
	// segment's value and whose bottom width corresponds to the NEXT segment's
	// value, creating a stepped funnel where each stage is visually proportional
	// to its value and tapers into the next stage.
	for i, point := range data.Points {
		// Top width = this segment's width under the configured mode
		topWidth := funnelSegmentWidth(point.Value, maxValue, plotArea.W, fc.config.WidthMode)

		// Bottom width = next segment's width (taper toward the next stage)
		var bottomWidth float64
		if i == numSegments-1 {
			// Last segment: taper to neck width or point
			bottomWidth = topWidth * fc.effectiveNeckWidth()
		} else {
			bottomWidth = funnelSegmentWidth(data.Points[i+1].Value, maxValue, plotArea.W, fc.config.WidthMode)
		}

		// Calculate Y positions
		y := plotArea.Y + float64(i)*(segmentHeight+effectiveGap)

		// Get color for this segment
		color := colors[i%len(colors)]
		if point.Color != nil {
			color = *point.Color
		}

		// Draw trapezoid
		fc.drawTrapezoid(centerX, y, topWidth, bottomWidth, segmentHeight, color)

		// Draw label
		conversion := ""
		if fc.config.ShowConversion {
			conversion = funnelConversionLabel(data.Points, i)
		}
		fc.drawLabel(point, i, centerX, y, topWidth, bottomWidth, segmentHeight, maxValue, plotArea, labelPadding, color, conversion)
	}

	// Draw title
	if fc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: fc.config.Width, H: headerHeight + fc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		fh := FootnoteReservedHeight(style)
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: fc.config.Height - fh,
			W: fc.config.Width,
			H: fh,
		})
	}

	return nil
}

// reserveExternalLabelSpace checks whether any inside-label segment will
// overflow, and if so shrinks the plot area from the right so that external
// connector labels stay within the SVG viewBox.
func (fc *FunnelChart) reserveExternalLabelSpace(data FunnelData, plotArea Rect, maxValue float64, style *StyleGuide) Rect {
	b := fc.builder
	numSegments := len(data.Points)

	// Use adaptive font size consistent with drawLabel
	adaptiveMaxFont := funnelAdaptiveFontSize(numSegments, style.Typography.SizeHeading)
	minFont := DefaultMinFontSize

	// Build label text for each segment. Track whether any segment overflows
	// and measure the widest external label across ALL segments (not just the
	// ones that overflow at current width, since shrinking the plot area may
	// cause additional segments to overflow).
	anyOverflow := false
	maxExternalLabelW := 0.0
	textPadding := style.Spacing.SM * 2

	for i, point := range data.Points {
		label := point.Label
		if fc.config.ShowValues {
			label = fmt.Sprintf("%s: %s", point.Label, fc.config.ValueFmt.FormatOr(point.Value, fc.config.ValueFormat))
		}
		if fc.config.ShowPercentage && maxValue > 0 {
			pct := (point.Value / maxValue) * 100
			if fc.config.ShowValues {
				label = fmt.Sprintf("%s (%.1f%%)", label, pct)
			} else {
				label = fmt.Sprintf("%s: %.1f%%", point.Label, pct)
			}
		}

		// Compute segment widths through the shared helper, so this pre-pass
		// and the draw loop cannot disagree about the geometry.
		topWidth := funnelSegmentWidth(point.Value, maxValue, plotArea.W, fc.config.WidthMode)
		var bottomWidth float64
		if i == numSegments-1 {
			bottomWidth = topWidth * fc.effectiveNeckWidth()
		} else {
			bottomWidth = funnelSegmentWidth(data.Points[i+1].Value, maxValue, plotArea.W, fc.config.WidthMode)
		}
		// Must match drawLabel logic: min(midWidth, bottomWidth) with 30% margin.
		midWidth := (topWidth + bottomWidth) / 2
		constraintWidth := midWidth
		if bottomWidth < constraintWidth {
			constraintWidth = bottomWidth
		}
		margin := constraintWidth * 0.3
		if margin < textPadding {
			margin = textPadding
		}
		b.Push()
		b.SetFontSize(adaptiveMaxFont)
		b.SetFontWeight(style.Typography.WeightMedium)
		// Must match drawLabel logic: minInsideWidth threshold.
		const minInsideWidth = 50.0
		availInside := constraintWidth - margin
		if availInside < minInsideWidth {
			anyOverflow = true
		} else {
			preFit := LabelFitStrategy{PreferredSize: adaptiveMaxFont, MinSize: minFont, MinCharWidth: 5.5}
			preResult := preFit.Fit(b, label, availInside, 0)
			b.SetFontSize(preResult.FontSize)
			textW, _ := b.MeasureText(label)
			if constraintWidth <= textW+margin || preResult.FontSize <= minFont {
				anyOverflow = true
			}
		}

		// Measure external label width at adaptive font size for ALL segments.
		b.SetFontSize(adaptiveMaxFont)
		extW, _ := b.MeasureText(label)
		needed := style.Spacing.MD + style.Spacing.XS + extW
		if needed > maxExternalLabelW {
			maxExternalLabelW = needed
		}
		b.Pop()
	}

	if anyOverflow {
		// Ensure external labels fit within the chart's right margin.
		rightMargin := fc.config.Width - (plotArea.X + plotArea.W)
		extra := maxExternalLabelW - rightMargin + style.Spacing.SM // SM buffer
		if extra > 0 {
			plotArea.W -= extra
		}
	}
	return plotArea
}

// drawTrapezoid draws a single funnel segment.
func (fc *FunnelChart) drawTrapezoid(centerX, y, topWidth, bottomWidth, height float64, color Color) {
	b := fc.builder

	// Calculate corner points
	topLeft := Point{X: centerX - topWidth/2, Y: y}
	topRight := Point{X: centerX + topWidth/2, Y: y}
	bottomRight := Point{X: centerX + bottomWidth/2, Y: y + height}
	bottomLeft := Point{X: centerX - bottomWidth/2, Y: y + height}

	// Draw the trapezoid
	b.Push()
	b.SetFillColor(color)
	b.SetStrokeColor(color.Darken(0.1))
	b.SetStrokeWidth(1)

	points := []Point{topLeft, topRight, bottomRight, bottomLeft}
	b.DrawPolygon(points)

	b.Pop()
}

// drawLabel draws the label for a funnel segment.
func (fc *FunnelChart) drawLabel(point FunnelDataPoint, index int, centerX, y, topWidth, bottomWidth, height, maxValue float64, plotArea Rect, labelPadding float64, bgColor Color, conversion string) {
	b := fc.builder
	style := b.StyleGuide()

	// Build label text
	label := point.Label
	if fc.config.ShowValues {
		label = fmt.Sprintf("%s: %s", point.Label, fc.config.ValueFmt.FormatOr(point.Value, fc.config.ValueFormat))
	}
	if fc.config.ShowPercentage && maxValue > 0 {
		pct := (point.Value / maxValue) * 100
		if fc.config.ShowValues {
			label = fmt.Sprintf("%s (%.1f%%)", label, pct)
		} else {
			label = fmt.Sprintf("%s: %.1f%%", point.Label, pct)
		}
	}

	labelY := y + height/2

	b.Push()
	// Adaptive font sizing: scale the starting font based on how many segments
	// we have (inferred from segment height relative to plot area).
	adaptiveMaxFont := funnelAdaptiveFontSize(fc.numSegments, style.Typography.SizeHeading)
	// Also cap font size to segment height so text never exceeds the segment.
	// Leave 20% padding above and below.
	heightCap := height * 0.6
	if adaptiveMaxFont > heightCap && heightCap > DefaultMinFontSize {
		adaptiveMaxFont = heightCap
	}
	minFont := DefaultMinFontSize

	b.SetFontSize(adaptiveMaxFont)
	b.SetFontWeight(style.Typography.WeightMedium)

	switch fc.config.LabelPosition {
	case FunnelLabelInside:
		midWidth := (topWidth + bottomWidth) / 2
		textPadding := style.Spacing.SM * 2

		// Check if inside label fits. Center-aligned text drawn at the
		// midpoint may extend past the narrower bottom edge of tapered
		// segments, causing white-on-white clipping. Use min(midWidth,
		// bottomWidth) as constraint — midWidth is where text is drawn,
		// bottomWidth prevents overflow at the tapered edge. Add generous
		// margin (30% of constraintWidth) for renderer differences.
		constraintWidth := midWidth
		if bottomWidth < constraintWidth {
			constraintWidth = bottomWidth
		}
		margin := constraintWidth * 0.3
		if margin < textPadding {
			margin = textPadding
		}

		fitsInside := false
		// Minimum usable width for inside labels: at 9pt font, ~50pt fits one
		// short word comfortably. Below this, labels are cramped and illegible.
		const minInsideWidth = 50.0
		availInside := constraintWidth - margin
		var insideFit LabelFitResult
		if availInside >= minInsideWidth {
			fit := LabelFitStrategy{PreferredSize: adaptiveMaxFont, MinSize: minFont, MinCharWidth: 5.5}
			insideFit = fit.Fit(b, label, availInside, 0)
			b.SetFontSize(insideFit.FontSize)
			textW, _ := b.MeasureText(label)
			fitsInside = constraintWidth > textW+margin && insideFit.FontSize > minFont
		}

		if fitsInside {
			// Label fits comfortably inside — draw with contrast-aware color.
			b.SetFontSize(insideFit.FontSize)
			b.SetTextColor(bgColor.TextColorFor())
			// The conversion line is what a funnel is FOR: without it the chart
			// shows four numbers and leaves the division to the reader
			// (go-slide-creator-6i6j).
			convFont := math.Max(minFont, insideFit.FontSize*0.75)
			if conversion != "" && height > insideFit.FontSize+convFont*1.6 {
				b.DrawText(insideFit.DisplayText, centerX, labelY-convFont*0.6, TextAlignCenter, TextBaselineMiddle)
				b.Push()
				b.SetFontSize(convFont)
				b.DrawText(conversion, centerX, labelY+insideFit.FontSize*0.7, TextAlignCenter, TextBaselineMiddle)
				b.Pop()
			} else {
				b.DrawText(insideFit.DisplayText, centerX, labelY, TextAlignCenter, TextBaselineMiddle)
			}
		} else {
			// Label overflows — draw external with connector.
			// Use adaptive font for external labels too, and truncate if needed.
			extFont := math.Max(minFont, adaptiveMaxFont)
			b.SetFontSize(extFont)
			segmentRightEdge := centerX + midWidth/2
			connectorEnd := segmentRightEdge + style.Spacing.MD
			labelX := connectorEnd + style.Spacing.XS

			b.Push()
			b.SetStrokeColor(style.Palette.TextSecondary)
			b.SetStrokeWidth(1)
			b.DrawLine(segmentRightEdge, labelY, connectorEnd, labelY)
			b.Pop()

			// Truncate external label to fit within remaining width
			availExtW := fc.config.Width - labelX - style.Spacing.SM
			if availExtW > 0 {
				fit := LabelFitStrategy{PreferredSize: extFont, MinSize: minFont, MinCharWidth: 5.5}
				extResult := fit.Fit(b, label, availExtW, 0)
				b.SetFontSize(extResult.FontSize)
				label = extResult.DisplayText
			}

			b.SetTextColor(style.Palette.TextPrimary)
			b.DrawText(label, labelX, labelY, TextAlignLeft, TextBaselineMiddle)
		}

	case FunnelLabelLeft:
		// Left side — clamp font to fit the reserved label padding area
		availW := labelPadding - style.Spacing.MD*2
		fit := LabelFitStrategy{PreferredSize: adaptiveMaxFont, MinSize: minFont, MinCharWidth: 5.5}
		leftResult := fit.Fit(b, label, availW, 0)
		b.SetFontSize(leftResult.FontSize)
		labelX := plotArea.X - style.Spacing.MD
		b.SetTextColor(style.Palette.TextPrimary)
		b.DrawText(leftResult.DisplayText, labelX, labelY, TextAlignRight, TextBaselineMiddle)

	case FunnelLabelRight:
		// Right side — clamp font to fit the reserved label padding area
		availW := labelPadding - style.Spacing.MD*2
		fit := LabelFitStrategy{PreferredSize: adaptiveMaxFont, MinSize: minFont, MinCharWidth: 5.5}
		rightResult := fit.Fit(b, label, availW, 0)
		b.SetFontSize(rightResult.FontSize)
		labelX := plotArea.X + plotArea.W + labelPadding - style.Spacing.MD
		b.SetTextColor(style.Palette.TextPrimary)
		b.DrawText(rightResult.DisplayText, labelX, labelY, TextAlignLeft, TextBaselineMiddle)
	}

	b.Pop()
}

// getColors returns colors for the funnel segments.
// getColors returns one colour per stage. A funnel is ONE metric falling
// through its stages, not four categories: the accent rotation painted MQL in
// the template's alert red and Won in its positive green, implying a valence
// the data does not carry. Unless the caller supplied colours, the stages take
// a ramp of the first accent — darkest at the top, lightest at the neck
// (go-slide-creator-6i6j).
func (fc *FunnelChart) getColors(style *StyleGuide, count int) []Color {
	if len(fc.config.Colors) >= count {
		return fc.config.Colors[:count]
	}
	if len(fc.config.Colors) > 0 {
		return resolveColors(fc.config.Colors, style, count)
	}
	return sequentialRamp(style.Palette.AccentColors()[0], count)
}

// sequentialRamp returns count shades of base, from the base itself to a light
// tint of it. One hue, ordered by lightness: the reader sees a single quantity
// getting smaller rather than four unrelated categories.
func sequentialRamp(base Color, count int) []Color {
	if count <= 0 {
		return nil
	}
	if count == 1 {
		return []Color{base}
	}
	const maxLighten = 0.55
	out := make([]Color, count)
	for i := range out {
		out[i] = base.Lighten(maxLighten * float64(i) / float64(count-1))
	}
	return out
}

// =============================================================================
// Funnel Chart Diagram Type (for Registry)
// =============================================================================

// FunnelDiagram implements the Diagram interface for funnel charts.
type FunnelDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for funnel charts.
func (d *FunnelDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("funnel chart requires data. Expected format: {\"values\": [{\"label\": \"Leads\", \"value\": 1000}, {\"label\": \"Converted\", \"value\": 200}]}")
	}

	// Normalize: accept "stages" as alias for "values" (documented in CHART_REFERENCE.md)
	if _, hasStages := req.Data["stages"]; hasStages {
		if _, hasValues := req.Data["values"]; !hasValues {
			req.Data["values"] = req.Data["stages"]
		}
	}

	// Check for values or points array
	_, hasValues := req.Data["values"]
	_, hasPoints := req.Data["points"]

	if !hasValues && !hasPoints {
		return fmt.Errorf("funnel chart requires 'values', 'stages', or 'points' array in data. Expected: {\"values\": [{\"label\": \"Leads\", \"value\": 1000}, {\"label\": \"Converted\", \"value\": 200}]}")
	}

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *FunnelDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *FunnelDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseFunnelData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultFunnelChartConfig(width, height)
		config.ValueFormatSpec = req.Style.ValueFormat
		// Funnel charts ALWAYS show values by default - the numeric values are essential
		// to understanding conversion drop-off. DefaultFunnelChartConfig sets ShowValues=true.
		// ShowLegend kept at default true; Draw only renders for multi-series.

		// Apply custom options
		if neckWidth, ok := req.Data["neck_width"].(float64); ok {
			config.NeckWidth = neckWidth
		}
		if gap, ok := req.Data["gap"].(float64); ok {
			config.Gap = gap
		}
		if showPct, ok := req.Data["show_percentage"].(bool); ok {
			config.ShowPercentage = showPct
		}
		if labelPos, ok := req.Data["label_position"].(string); ok {
			config.LabelPosition = FunnelLabelPosition(labelPos)
		}
		if mode, ok := req.Data["width_mode"].(string); ok {
			config.WidthMode = mode
		}
		if showConv, ok := req.Data["show_conversion"].(bool); ok {
			config.ShowConversion = showConv
		}

		chart := NewFunnelChart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseFunnelPointMap extracts a FunnelDataPoint from a map.
func parseFunnelPointMap(p map[string]any) FunnelDataPoint {
	point := FunnelDataPoint{}
	if label, ok := p["label"].(string); ok {
		point.Label = label
	}
	if value, ok := p["value"].(float64); ok {
		point.Value = value
	} else if value, ok := p["value"].(int); ok {
		point.Value = float64(value)
	}
	if colorStr, ok := p["color"].(string); ok {
		if c, err := ParseColor(colorStr); err == nil {
			point.Color = &c
		}
	}
	return point
}

// parseFunnelStructuredPoints parses an array of maps into FunnelDataPoints.
func parseFunnelStructuredPoints(valuesRaw []any) ([]FunnelDataPoint, error) {
	points := make([]FunnelDataPoint, 0, len(valuesRaw))
	for _, pRaw := range valuesRaw {
		p, ok := pRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid point format")
		}
		points = append(points, parseFunnelPointMap(p))
	}
	return points, nil
}

// parseFunnelSimpleValues parses float values with optional category labels.
func parseFunnelSimpleValues(valuesRaw []any, categories []string) []FunnelDataPoint {
	values, _ := toFloat64Slice(valuesRaw)
	points := make([]FunnelDataPoint, len(values))
	for i, v := range values {
		label := fmt.Sprintf("Stage %d", i+1)
		if i < len(categories) {
			label = categories[i]
		}
		points[i] = FunnelDataPoint{Label: label, Value: v}
	}
	return points
}

// parseFunnelData parses the request data into FunnelData.
func parseFunnelData(req *RequestEnvelope) (FunnelData, error) {
	data := FunnelData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Try parsing points array first (structured format)
	if pointsRaw, ok := toAnySlice(req.Data["points"]); ok {
		points, err := parseFunnelStructuredPoints(pointsRaw)
		if err != nil {
			return data, err
		}
		data.Points = points
	} else if valuesRaw, ok := toAnySlice(req.Data["values"]); ok {
		// Check if values contains structured points (maps) or simple floats
		if len(valuesRaw) > 0 {
			if _, isMap := valuesRaw[0].(map[string]any); isMap {
				points, err := parseFunnelStructuredPoints(valuesRaw)
				if err != nil {
					return data, err
				}
				data.Points = points
			} else {
				categories := []string{}
				if catsRaw, ok := req.Data["categories"]; ok {
					categories, _ = toStringSlice(catsRaw)
				}
				data.Points = parseFunnelSimpleValues(valuesRaw, categories)
			}
		}
	} else {
		return data, fmt.Errorf("invalid funnel data format")
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}
