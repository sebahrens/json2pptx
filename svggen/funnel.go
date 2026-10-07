package svggen

import (
	"fmt"
	"math"
	"strings"
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

	// Style picks the drawing: "" / "steps" (default) sets each stage as a
	// centred bar whose width follows its value, the stage name in a bold
	// column on the left, the value inside the bar and the stage-to-stage
	// conversion in a pale connector between two bars. "tapered" is the
	// earlier stack of trapezoids, whose first stage flared to the full
	// width of a wide slide body like a lampshade (go-slide-creator-cn8mn).
	Style string
}

// Funnel styles.
const (
	FunnelStyleSteps   = "steps"
	FunnelStyleTapered = "tapered"
)

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
// reportIncreasingStages emits FindingFunnelStageIncrease for every stage
// larger than the one above it. A funnel reads as progressive narrowing, so a
// widening stage is a data error or the wrong chart; it used to render
// without comment (go-slide-creator-7w2ed).
func (fc *FunnelChart) reportIncreasingStages(data FunnelData) {
	var rising []string
	var indices []int
	for i := 1; i < len(data.Points); i++ {
		prev, cur := data.Points[i-1], data.Points[i]
		if cur.Value > prev.Value {
			rising = append(rising, fmt.Sprintf("%q (%g) > %q (%g)", cur.Label, cur.Value, prev.Label, prev.Value))
			indices = append(indices, i)
		}
	}
	if len(rising) == 0 {
		return
	}
	fc.builder.AddFinding(Finding{
		Code:     FindingFunnelStageIncrease,
		Message:  fmt.Sprintf("funnel: %d stage(s) are larger than the stage above them (%s); a funnel should narrow — check the values or use a bar chart", len(rising), strings.Join(rising, "; ")),
		Severity: "warning",
		Fix: &FixSuggestion{
			Kind:   FixKindReplaceValue,
			Params: map[string]any{"stage_indices": indices, "diagram_type": "funnel_chart"},
		},
	})
}

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
	fc.reportIncreasingStages(data)
	style := b.StyleGuide()
	colors := fc.getColors(style, len(data.Points))

	// Calculate plot area
	plotArea := fc.config.PlotArea()

	// Adjust for title
	headerHeight := chartHeaderHeight(style, fc.config.ShowTitle, data.Title, data.Subtitle)
	if fc.config.Style != FunnelStyleTapered {
		plotArea.Y += headerHeight
		plotArea.H -= headerHeight
		if data.Footnote != "" {
			plotArea.H -= math.Max(0, FootnoteReservedHeight(style)-fc.config.MarginBottom)
		}
		fc.drawSteps(data, plotArea, colors)
		fc.drawHeaderAndFootnote(data)
		return nil
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

	fc.drawHeaderAndFootnote(data)
	return nil
}

// drawHeaderAndFootnote draws the exhibit heading and the footnote.
func (fc *FunnelChart) drawHeaderAndFootnote(data FunnelData) {
	b := fc.builder
	style := b.StyleGuide()
	drawChartHeader(b, fc.config.Width, fc.config.ShowTitle, data.Title, data.Subtitle)
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
}

// funnelStepLabel is the text set inside a stage bar: the value, and the share
// of the first stage when asked for.
func (fc *FunnelChart) funnelStepLabel(point FunnelDataPoint, maxValue float64) string {
	label := ""
	if fc.config.ShowValues {
		label = fc.config.ValueFmt.FormatOr(point.Value, fc.config.ValueFormat)
	}
	if fc.config.ShowPercentage && maxValue > 0 {
		pct := fmt.Sprintf("%.1f%%", point.Value/maxValue*100)
		if label == "" {
			return pct
		}
		label += " (" + pct + ")"
	}
	return label
}

// Geometry of the default "steps" funnel, in units of the stage bar height.
const (
	// funnelStepGapFrac is the height of the pale connector between two bars.
	funnelStepGapFrac = 0.35
	// funnelStepMaxBarLines caps a bar's height in lines of its value text, so
	// a few stages in a tall body do not become slabs.
	funnelStepMaxBarLines = 5.0
	// funnelStepMaxAspect caps the widest bar at this multiple of the funnel's
	// height: on a wide, short body the funnel stays a funnel instead of
	// stretching edge to edge.
	funnelStepMaxAspect = 2.6
	// funnelStepConnectorAlpha is how much of the stage colour the pale
	// connector keeps over the background.
	funnelStepConnectorAlpha = 0.16
)

// drawSteps draws the default funnel: one centred bar per stage, joined by pale
// connectors; the stage names stand in a bold column on the left and the
// stage-to-stage conversions in a column on the right, each level with the
// connector it describes.
func (fc *FunnelChart) drawSteps(data FunnelData, plotArea Rect, colors []Color) {
	b := fc.builder
	style := b.StyleGuide()
	n := len(data.Points)
	if plotArea.W <= 0 || plotArea.H <= 0 {
		return
	}
	maxValue := 0.0
	for _, p := range data.Points {
		maxValue = math.Max(maxValue, p.Value)
	}
	if maxValue == 0 {
		maxValue = 1
	}

	floor := math.Max(DefaultMinFontSize, style.Typography.ReadableFloor)
	nameFont := math.Max(style.Typography.SizeBody, floor)
	convFont := math.Max(style.Typography.SizeSmall, floor)

	// Vertical rhythm: bars with connectors between them.
	barH := plotArea.H / (float64(n) + float64(n-1)*funnelStepGapFrac)
	barH = math.Min(barH, nameFont*funnelStepMaxBarLines)
	connH := barH * funnelStepGapFrac
	if barH < nameFont*1.2 {
		nameFont = math.Max(floor*0.85, barH/1.2)
	}
	valueFont := nameFont
	// A conversion line needs a row pitch that holds it.
	showConv := fc.config.ShowConversion && n > 1 && barH+connH >= convFont*1.25
	blockH := float64(n)*barH + float64(n-1)*connH
	top := plotArea.Y + (plotArea.H-blockH)/2

	// Stage-name column on the left, bold.
	b.Push()
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetFontSize(nameFont)
	nameW := 0.0
	for _, p := range data.Points {
		w, _ := b.MeasureText(p.Label)
		nameW = math.Max(nameW, w)
	}
	b.Pop()
	nameW = math.Min(nameW, plotArea.W*0.3)
	nameGap := style.Spacing.LG
	if nameW == 0 {
		nameGap = 0
	}

	// Conversion column on the right.
	convs := make([]string, n)
	convW, convGap := 0.0, 0.0
	if showConv {
		b.Push()
		b.SetFontSize(convFont)
		for i := 1; i < n; i++ {
			convs[i] = funnelConversionLabel(data.Points, i)
			w, _ := b.MeasureText(convs[i])
			convW = math.Max(convW, w)
		}
		b.Pop()
		convW = math.Min(convW, plotArea.W*0.3)
		if convW > 0 {
			convGap = style.Spacing.LG
		}
	}

	// Bar zone: as wide as the body allows, capped so the funnel keeps a
	// funnel's proportions, and the whole block centred.
	zoneW := math.Min(plotArea.W-nameW-nameGap-convW-convGap, blockH*funnelStepMaxAspect)
	if zoneW <= 0 {
		return
	}
	blockX := plotArea.X + (plotArea.W-(nameW+nameGap+zoneW+convGap+convW))/2
	zoneX := blockX + nameW + nameGap
	centerX := zoneX + zoneW/2

	// Bar widths: proportional, floored in clamped mode so the value fits.
	pad := style.Spacing.MD
	widths := make([]float64, n)
	labels := make([]string, n)
	b.Push()
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetFontSize(valueFont)
	for i, p := range data.Points {
		labels[i] = fc.funnelStepLabel(p, maxValue)
		share := p.Value / maxValue
		switch fc.config.WidthMode {
		case FunnelWidthEqual:
			widths[i] = zoneW
		case FunnelWidthProportional:
			widths[i] = zoneW * share
		default:
			tw, _ := b.MeasureText(labels[i])
			widths[i] = math.Min(zoneW, math.Max(zoneW*share, tw+2*pad))
		}
	}
	b.Pop()

	for i, p := range data.Points {
		y := top + float64(i)*(barH+connH)
		color := colors[i%len(colors)]
		if p.Color != nil {
			color = *p.Color
		}
		w := widths[i]

		// Connector into the next stage.
		if i < n-1 && connH > 0 {
			next := widths[i+1]
			pale := color.WithAlpha(funnelStepConnectorAlpha).BlendOver(style.Palette.Background)
			b.Push()
			b.SetFillColor(pale)
			b.SetStrokeColor(pale)
			b.SetStrokeWidth(0)
			b.DrawPolygon([]Point{
				{X: centerX - w/2, Y: y + barH},
				{X: centerX + w/2, Y: y + barH},
				{X: centerX + next/2, Y: y + barH + connH},
				{X: centerX - next/2, Y: y + barH + connH},
			})
			b.Pop()
		}
		if convs[i] != "" && convW > 0 {
			b.Push()
			b.SetFontSize(convFont)
			b.SetFontWeight(style.Typography.WeightNormal)
			b.SetTextColor(style.Palette.TextSecondary)
			b.DrawText(b.TruncateToWidth(convs[i], convW), zoneX+zoneW+convGap, y-connH/2, TextAlignLeft, TextBaselineMiddle)
			b.Pop()
		}

		// Stage bar.
		b.Push()
		b.SetFillColor(color)
		b.SetStrokeColor(color)
		b.SetStrokeWidth(0)
		b.FillRect(Rect{X: centerX - w/2, Y: y, W: w, H: barH})
		b.Pop()

		// Value inside the bar, or beside it when the bar is too narrow.
		if labels[i] != "" {
			b.Push()
			b.SetFontWeight(style.Typography.WeightBold)
			b.SetFontSize(valueFont)
			tw, _ := b.MeasureText(labels[i])
			if tw+pad <= w {
				b.SetTextColor(color.TextColorFor())
				b.DrawText(labels[i], centerX, y+barH/2, TextAlignCenter, TextBaselineMiddle)
			} else {
				b.SetTextColor(style.Palette.TextPrimary)
				avail := zoneX + zoneW - (centerX + w/2) - style.Spacing.SM
				b.DrawText(b.TruncateToWidth(labels[i], math.Max(avail, 0)), centerX+w/2+style.Spacing.SM, y+barH/2, TextAlignLeft, TextBaselineMiddle)
			}
			b.Pop()
		}

		// Stage name.
		if nameW > 0 {
			b.Push()
			b.SetFontWeight(style.Typography.WeightBold)
			b.SetFontSize(nameFont)
			b.SetTextColor(style.Palette.TextPrimary)
			b.DrawText(b.TruncateToWidth(p.Label, nameW), blockX, y+barH/2, TextAlignLeft, TextBaselineMiddle)
			b.Pop()
		}
	}
}

// reserveExternalLabelSpace checks whether any inside-label segment will
// overflow, and if so shrinks the plot area from the right so that external
// connector labels stay within the SVG viewBox.
func (fc *FunnelChart) reserveExternalLabelSpace(data FunnelData, plotArea Rect, maxValue float64, style *StyleGuide) Rect {
	b := fc.builder
	numSegments := len(data.Points)

	// Use adaptive font size consistent with drawLabel
	adaptiveMaxFont := funnelAdaptiveFontSize(numSegments, style.Typography.SizeHeading)
	minFont := math.Max(DefaultMinFontSize, style.Typography.ReadableFloor)

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
			if constraintWidth <= textW+margin || preResult.FontSize <= DefaultMinFontSize {
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
	minFont := math.Max(DefaultMinFontSize, style.Typography.ReadableFloor)

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
			fitsInside = constraintWidth > textW+margin && insideFit.FontSize > DefaultMinFontSize
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
		out[i] = base.Tint(1 - maxLighten*float64(i)/float64(count-1))
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

	if data, err := parseFunnelData(req); err == nil {
		return validateFunnelNonNegative(data)
	}
	return nil
}

// validateFunnelNonNegative rejects negative stage values: a negative width
// inverts its trapezoid into a self-intersecting bow-tie labelled with a
// negative share (go-slide-creator-csclk.7).
func validateFunnelNonNegative(data FunnelData) error {
	for i, p := range data.Points {
		if p.Value < 0 {
			label := p.Label
			if label == "" {
				label = fmt.Sprintf("stage %d", i+1)
			}
			return &ValidationError{
				Field:   fmt.Sprintf("data.values[%d]", i),
				Code:    ErrCodeConstraint,
				Message: fmt.Sprintf("funnel_chart stage values must be >= 0 (%q is %v); a funnel cannot draw a negative stage — show losses as a separate stage count or use a waterfall", label, p.Value),
				Value:   p.Value,
			}
		}
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
		// The side-label positions and an explicit neck belong to the tapered
		// drawing; asking for either keeps it.
		_, hasNeck := req.Data["neck_width"]
		_, hasLabelPos := req.Data["label_position"]
		if hasNeck || hasLabelPos {
			config.Style = FunnelStyleTapered
		}
		if st, ok := req.Data["style"].(string); ok {
			config.Style = st
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

// DataSchema returns the data contract for funnel charts.
func (d *FunnelDiagram) DataSchema() *DataSchema {
	stage := ObjectDataSchema("A stage, or a plain number (labelled from categories)", map[string]*DataSchema{
		"label": StringDataSchema("Stage label"),
		"value": NumberDataSchema("Stage value"),
		"color": StringDataSchema("Hex color override"),
	}, nil)
	return diagramDataSchema("Funnel of decreasing stages", map[string]*DataSchema{
		"values":          ArrayDataSchema("Stages: {label, value} objects or numbers", stage, 0),
		"stages":          ArrayDataSchema("Alias for values", stage, 0),
		"points":          ArrayDataSchema("Stages as {label, value} objects", stage, 0),
		"categories":      stringListSchema("Stage labels for numeric values"),
		"footnote":        StringDataSchema("Footnote text"),
		"neck_width":      NumberDataSchema("Neck width ratio"),
		"gap":             NumberDataSchema("Gap between stages"),
		"show_percentage": BooleanDataSchema("Show each stage's share"),
		"label_position":  StringDataSchema("Label placement"),
		"width_mode":      StringDataSchema("Stage width mode"),
		"show_conversion": BooleanDataSchema("Show stage-to-stage conversion"),
		"style":           StringDataSchema("steps (default: centred bars joined by conversion connectors) or tapered (stacked trapezoids)"),
	}, nil)
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

	if err := validateFunnelNonNegative(data); err != nil {
		return data, err
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}
