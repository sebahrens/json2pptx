package svggen

import (
	"fmt"
	"math"
	"strings"
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

	// Style picks the drawing: "" / "bullet" (default) sets a large value over
	// a horizontal bar — a neutral track from min to max, the value in the
	// accent, threshold bands as a neutral ladder with their bounds labelled.
	// "dial" is the earlier speedometer arc with needle and tick marks, which
	// is dashboard chrome on a consulting slide (go-slide-creator-cn8mn).
	Style string
}

// Gauge styles.
const (
	GaugeStyleBullet = "bullet"
	GaugeStyleDial   = "dial"
)

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
	if err := validateGaugeRange(gc.config.MinValue, gc.config.MaxValue); err != nil {
		return err
	}
	b := gc.builder
	style := b.StyleGuide()

	// Thresholds without a colour of their own: the dial paints them in the
	// semantic roles, the bullet bar in a neutral ladder.
	unsetThresholds := make([]bool, len(gc.config.Thresholds))
	for i, t := range gc.config.Thresholds {
		unsetThresholds[i] = t.Color == (Color{})
	}

	// Apply theme colors if needle still has default hardcoded color.
	gc.applyThemeColors()

	// The value and the min/max scale ticks are the same quantity, so they get
	// the same format (go-slide-creator-e2ck9).
	gc.config.ResolveValueFormatter([]float64{data.Value, gc.config.MinValue, gc.config.MaxValue}, false)

	// The value bar (the dial's needle and arc) is clamped to the range; say
	// so, because the printed value and the mark then disagree (value 500 on a 0-100 dial
	// pinned the needle at 100 with nothing reported — go-slide-creator-7w2ed).
	if data.Value < gc.config.MinValue || data.Value > gc.config.MaxValue {
		b.AddFinding(Finding{
			Field:    "value",
			Code:     FindingPointOutOfRange,
			Message:  fmt.Sprintf("gauge: value %g is outside the range %g–%g; the value bar is pinned at the nearest end while the label shows %g", data.Value, gc.config.MinValue, gc.config.MaxValue, data.Value),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindExplicitScale,
				Params: map[string]any{"value": data.Value, "min": gc.config.MinValue, "max": gc.config.MaxValue, "diagram_type": "gauge_chart"},
			},
		})
	}

	// Calculate plot area
	plotArea := gc.config.PlotArea()

	// Adjust for title
	headerHeight := chartHeaderHeight(style, gc.config.ShowTitle, data.Title, data.Subtitle)
	if gc.config.Style != GaugeStyleDial {
		plotArea.Y += headerHeight
		plotArea.H -= headerHeight
		if data.Footnote != "" {
			plotArea.H -= math.Max(0, FootnoteReservedHeight(style)-gc.config.MarginBottom)
		}
		gc.drawBullet(data, plotArea, unsetThresholds)
		gc.drawHeaderAndFootnote(data)
		return nil
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
	topExtent := 0.0 // max distance above center (negative Y in math coords)
	botExtent := 0.0 // max distance below center (positive Y in math coords)
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

	gc.drawHeaderAndFootnote(data)
	return nil
}

// drawHeaderAndFootnote draws the exhibit heading and the footnote.
func (gc *GaugeChart) drawHeaderAndFootnote(data GaugeData) {
	b := gc.builder
	style := b.StyleGuide()
	drawChartHeader(b, gc.config.Width, gc.config.ShowTitle, data.Title, data.Subtitle)
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
}

// Geometry of the default "bullet" gauge.
const (
	// gaugeBulletMaxAspect caps the bar's length at this multiple of the plot
	// height, so a wide body does not stretch it edge to edge.
	gaugeBulletMaxAspect = 3.2
	// gaugeBulletValueLines is the big value's size in lines of the chart
	// title, and gaugeBulletValueMaxFrac its cap as a share of the plot height.
	gaugeBulletValueLines   = 3.6
	gaugeBulletValueMaxFrac = 0.4
	// gaugeBulletValueRise is the height the value's digits take above their
	// baseline, in ems.
	gaugeBulletValueRise = 0.78
	// gaugeBulletTrackLines is the track height in lines of body text.
	gaugeBulletTrackLines = 3.0
	// gaugeBulletBarFrac is the value bar's share of the track height when
	// threshold bands sit behind it.
	gaugeBulletBarFrac = 0.42
)

// gaugeBand is one stretch of the bullet track.
type gaugeBand struct {
	from, to float64
	fill     Color
	label    string
}

// bulletBands splits the range into the track's bands. Thresholds without a
// colour take a neutral ladder, darkest at the low end, so the accent value
// bar stays the one coloured mark; the stretch above the last threshold is
// the bare track.
func (gc *GaugeChart) bulletBands(unset []bool) []gaugeBand {
	style := gc.builder.StyleGuide()
	lo, hi := gc.config.MinValue, gc.config.MaxValue
	neutral := func(alpha float64) Color {
		return style.Palette.TextPrimary.WithAlpha(alpha).BlendOver(style.Palette.Background)
	}
	const trackAlpha, darkAlpha = 0.08, 0.30
	ths := gc.config.Thresholds
	var bands []gaugeBand
	prev := lo
	for i, t := range ths {
		to := math.Max(lo, math.Min(hi, t.Value))
		if to <= prev {
			continue
		}
		fill := t.Color
		if i < len(unset) && unset[i] {
			alpha := trackAlpha
			if len(ths) > 1 {
				alpha = darkAlpha - (darkAlpha-trackAlpha)*float64(i)/float64(len(ths)-1)
			} else if to < hi {
				alpha = darkAlpha
			}
			fill = neutral(alpha)
		}
		bands = append(bands, gaugeBand{from: prev, to: to, fill: fill, label: t.Label})
		prev = to
	}
	if prev < hi {
		bands = append(bands, gaugeBand{from: prev, to: hi, fill: neutral(trackAlpha)})
	}
	return bands
}

// drawBullet draws the default gauge: the value set large over a horizontal
// bar that runs from min to max.
func (gc *GaugeChart) drawBullet(data GaugeData, plotArea Rect, unset []bool) {
	b := gc.builder
	style := b.StyleGuide()
	if plotArea.W <= 0 || plotArea.H <= 0 {
		return
	}
	lo, hi := gc.config.MinValue, gc.config.MaxValue
	bands := gc.bulletBands(unset)
	hasBands := len(gc.config.Thresholds) > 0
	hasBandLabels := false
	for _, bd := range bands {
		hasBandLabels = hasBandLabels || bd.label != ""
	}

	floor := math.Max(DefaultMinFontSize, style.Typography.ReadableFloor)
	tickFont := math.Max(style.Typography.SizeSmall, floor)
	bodyFont := math.Max(style.Typography.SizeBody, floor)
	valueFont := math.Min(style.Typography.SizeTitle*gaugeBulletValueLines, plotArea.H*gaugeBulletValueMaxFrac)
	valueFont = math.Max(valueFont, style.Typography.SizeTitle)
	trackH := bodyFont * gaugeBulletTrackLines
	gap := style.Spacing.MD
	trackW := math.Min(plotArea.W, plotArea.H*gaugeBulletMaxAspect)
	trackX := plotArea.X + (plotArea.W-trackW)/2
	xOf := func(v float64) float64 {
		v = math.Max(lo, math.Min(hi, v))
		return trackX + trackW*(v-lo)/(hi-lo)
	}
	// Band labels: each under its band, moved aside rather than cut short
	// where a band is narrower than its label, on a second row when one row
	// cannot hold them all (go-slide-creator-u3nl6).
	bandLabels := gc.placeBandLabels(bands, xOf, trackX, trackW, tickFont)
	labelsH := tickFont * 1.4
	if hasBandLabels {
		labelsH += tickFont * 1.4 * float64(bandLabelRows(bandLabels))
	}
	// The value stands on its baseline, gaugeBulletValueRise of its size
	// under the block's top, with a gap and a half down to the track.
	valueGap := gap * 1.5
	blockH := valueFont*gaugeBulletValueRise + valueGap + trackH + gap + labelsH
	if blockH > plotArea.H {
		// Short cell: give the fixed text its room and the track the rest.
		trackH = math.Max(bodyFont*0.8, plotArea.H-(valueFont*gaugeBulletValueRise+valueGap+gap+labelsH))
		blockH = valueFont*gaugeBulletValueRise + valueGap + trackH + gap + labelsH
	}
	y := plotArea.Y + math.Max(0, (plotArea.H-blockH)/2)

	// The value, large, with its label beside it.
	valueText := gc.config.ValueFmt.FormatOr(data.Value, gc.config.ValueFormat)
	if data.Unit != "" {
		valueText += data.Unit
	}
	b.Push()
	b.SetFontWeight(style.Typography.WeightBold)
	baseline := y + valueFont*gaugeBulletValueRise
	valueFont = b.ClampFontSize(valueText, trackW, valueFont, style.Typography.SizeTitle)
	b.SetFontSize(valueFont)
	b.SetTextColor(style.Palette.TextPrimary)
	valueW, _ := b.MeasureText(valueText)
	b.DrawText(valueText, trackX, baseline, TextAlignLeft, TextBaselineAlphabetic)
	if data.Label != "" {
		// The slide's own renderer may set the bold value a little wider
		// than it measures here: keep the label clear of it.
		labelX := trackX + valueW*1.06 + valueFont*0.2
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetFontSize(bodyFont)
		b.SetTextColor(style.Palette.TextSecondary)
		b.DrawText(b.TruncateToWidth(data.Label, trackX+trackW-labelX), labelX, baseline, TextAlignLeft, TextBaselineAlphabetic)
	}
	b.Pop()

	// Track bands.
	trackY := baseline + valueGap
	b.Push()
	b.SetStrokeWidth(0)
	for _, bd := range bands {
		x0, x1 := xOf(bd.from), xOf(bd.to)
		b.SetFillColor(bd.fill)
		b.SetStrokeColor(bd.fill)
		b.FillRect(Rect{X: x0, Y: trackY, W: x1 - x0, H: trackH})
	}
	// Value bar: the accent, the one coloured mark.
	// The role-mapped primary fill, as the dial's value arc uses: when the
	// template's accent1 is too pale to carry a mark, the first safe accent.
	accent := style.Palette.Accent1
	if (style.Palette.Roles.PrimaryFill != Color{}) {
		accent = style.Palette.Roles.PrimaryFill
	}
	barH, barY := trackH, trackY
	if hasBands {
		barH = trackH * gaugeBulletBarFrac
		barY = trackY + (trackH-barH)/2
	}
	if w := xOf(data.Value) - trackX; w > 0 {
		b.SetFillColor(accent)
		b.SetStrokeColor(accent)
		b.FillRect(Rect{X: trackX, Y: barY, W: w, H: barH})
	}
	b.Pop()

	// Range labels under the track: min, max and every band bound between,
	// each bound marked by a tick on the track's lower edge. A bound close
	// to an end (95 of 100) keeps its tick where it is and moves its number
	// aside instead of dropping it.
	labelY := trackY + trackH + gap + tickFont/2
	b.Push()
	b.SetFontSize(tickFont)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextSecondary)
	tick := func(v float64) string { return gc.config.ValueFmt.FormatOr(v, "%.0f") }
	minText, maxText := tick(lo), tick(hi)
	minW, _ := b.MeasureText(minText)
	maxW, _ := b.MeasureText(maxText)
	b.DrawText(minText, trackX, labelY, TextAlignLeft, TextBaselineMiddle)
	b.DrawText(maxText, trackX+trackW, labelY, TextAlignRight, TextBaselineMiddle)
	var bounds []spreadLabel
	for i, bd := range bands {
		if i == len(bands)-1 || bd.to >= hi {
			continue
		}
		text := tick(bd.to)
		w, _ := b.MeasureText(text)
		x := xOf(bd.to)
		bounds = append(bounds, spreadLabel{text: text, center: x, width: w})
		b.Push()
		b.SetStrokeColor(style.Palette.TextSecondary)
		b.SetStrokeWidth(style.Strokes.WidthHairline)
		b.DrawLine(x, trackY+trackH, x, trackY+trackH+gap*0.6)
		b.Pop()
	}
	if spreadLabels(bounds, trackX+minW+style.Spacing.SM, trackX+trackW-maxW-style.Spacing.SM, style.Spacing.SM) {
		for _, l := range bounds {
			b.DrawText(l.text, l.left+l.width/2, labelY, TextAlignCenter, TextBaselineMiddle)
		}
	}
	for _, l := range bandLabels {
		b.DrawText(l.text, l.left+l.width/2, labelY+tickFont*1.4*float64(l.row+1), TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()
}

// spreadLabel is one label of a row under the bullet track: where it wants
// to be centred, how wide it is, and where it ends up.
type spreadLabel struct {
	text          string
	center, width float64
	left          float64
	row           int
}

// spreadLabels places a row of labels as near their centres as it can without
// two of them closer than gap or any outside [lo, hi]. Labels keep their
// order. It reports false when the row is too short for them all.
func spreadLabels(labels []spreadLabel, lo, hi, gap float64) bool {
	if len(labels) == 0 {
		return true
	}
	edge := lo
	for i := range labels {
		labels[i].left = math.Max(labels[i].center-labels[i].width/2, edge)
		edge = labels[i].left + labels[i].width + gap
	}
	edge = hi
	for i := len(labels) - 1; i >= 0; i-- {
		labels[i].left = math.Min(labels[i].left, edge-labels[i].width)
		edge = labels[i].left - gap
	}
	return labels[0].left >= lo-0.5
}

// bandLabelRows is the number of rows the placed band labels take.
func bandLabelRows(labels []spreadLabel) int {
	rows := 0
	for _, l := range labels {
		rows = max(rows, l.row+1)
	}
	return rows
}

// placeBandLabels lays the band labels out under the track: one row when they
// fit side by side, two alternating rows when they do not, and only then
// shortened to what their share of the track holds.
func (gc *GaugeChart) placeBandLabels(bands []gaugeBand, xOf func(float64) float64, trackX, trackW, font float64) []spreadLabel {
	b := gc.builder
	style := b.StyleGuide()
	b.Push()
	defer b.Pop()
	b.SetFontSize(font)
	b.SetFontWeight(style.Typography.WeightNormal)
	var labels []spreadLabel
	for _, bd := range bands {
		if bd.label == "" {
			continue
		}
		w, _ := b.MeasureText(bd.label)
		labels = append(labels, spreadLabel{text: bd.label, center: (xOf(bd.from) + xOf(bd.to)) / 2, width: w})
	}
	if len(labels) == 0 {
		return nil
	}
	gap := style.Spacing.MD
	lo, hi := trackX, trackX+trackW
	if spreadLabels(labels, lo, hi, gap) {
		return labels
	}
	// Two rows, alternating, so neighbours no longer compete for width.
	var rows [2][]spreadLabel
	for i, l := range labels {
		l.row = i % 2
		rows[l.row] = append(rows[l.row], l)
	}
	if spreadLabels(rows[0], lo, hi, gap) && spreadLabels(rows[1], lo, hi, gap) {
		return append(rows[0], rows[1]...)
	}
	// Still too many: one row, each label cut to an equal share.
	share := (trackW - gap*float64(len(labels)-1)) / float64(len(labels))
	for i := range labels {
		labels[i].row = 0
		if labels[i].width > share {
			labels[i].text = b.TruncateToWidth(labels[i].text, share)
			labels[i].width, _ = b.MeasureText(labels[i].text)
		}
	}
	spreadLabels(labels, lo, hi, gap)
	return labels
}

// applyThemeColors sets needle and pivot colors from the theme palette.
// Called at draw time so the builder's style guide is available.
func (gc *GaugeChart) applyThemeColors() {
	style := gc.builder.StyleGuide()

	// Use TextPrimary for the needle so it contrasts with the Accent1 value arc.
	// Previously used Accent1.Darken(0.25) which was nearly invisible against
	// the Accent1 arc on templates like forest-green.
	gc.config.NeedleStyle.Color = style.Palette.TextPrimary

	// Unset zones progress from negative through neutral to positive.
	for i := range gc.config.Thresholds {
		if gc.config.Thresholds[i].Color == (Color{}) {
			switch {
			case len(gc.config.Thresholds) == 1:
				gc.config.Thresholds[i].Color = style.Palette.Success
			case i == 0:
				gc.config.Thresholds[i].Color = style.Palette.Error
			case i == len(gc.config.Thresholds)-1:
				gc.config.Thresholds[i].Color = style.Palette.Success
			default:
				gc.config.Thresholds[i].Color = style.Palette.Warning
			}
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

	// Prefer the role-mapped primary fill over a blind Accent1 pick. RoleMap
	// is populated by deriveRoleMap (or overridden via StyleSpec.RoleMap), so
	// when the template's accent1 fails WCAG against white the value arc will
	// land on the first white-text-safe accent instead — matching the native
	// skill_info "primary_fill" choice and keeping native shapes and svggen
	// arcs in lockstep.
	headerFill := style.Palette.Accent1
	if (style.Palette.Roles.PrimaryFill != Color{}) {
		headerFill = style.Palette.Roles.PrimaryFill
	}

	b.Push()
	b.SetFillColor(headerFill)
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

// gaugeZone is one colored band of the dial, as fractions of the sweep.
type gaugeZone struct {
	startRatio, endRatio float64
	color                Color
	remainder            bool // the unfilled band after the last threshold
}

// thresholdZones maps the configured thresholds onto fractions of the dial.
// Thresholds outside [min, max] are clamped to the dial and the walk stops at
// max: an unclamped ratio above 1 swept a zone past the end of the dial and
// round through the gap at its foot (go-slide-creator-s1uvj.32). Zones that
// clamp to nothing (below min, or out of order) are skipped.
func (gc *GaugeChart) thresholdZones() []gaugeZone {
	totalRange := gc.config.MaxValue - gc.config.MinValue
	prevValue := gc.config.MinValue
	var zones []gaugeZone

	for _, threshold := range gc.config.Thresholds {
		if prevValue >= gc.config.MaxValue {
			break
		}
		value := math.Min(threshold.Value, gc.config.MaxValue)
		if value <= prevValue {
			continue
		}
		zones = append(zones, gaugeZone{
			startRatio: (prevValue - gc.config.MinValue) / totalRange,
			endRatio:   (value - gc.config.MinValue) / totalRange,
			color:      threshold.Color,
		})
		prevValue = value
	}

	// Fill remaining area if thresholds don't cover the full range
	if prevValue < gc.config.MaxValue {
		zones = append(zones, gaugeZone{
			startRatio: (prevValue - gc.config.MinValue) / totalRange,
			endRatio:   1,
			remainder:  true,
		})
	}
	return zones
}

// drawThresholdZones draws colored zones on the gauge.
func (gc *GaugeChart) drawThresholdZones(centerX, centerY, outerRadius, innerRadius, startAngle, endAngle float64) {
	b := gc.builder
	totalAngle := endAngle - startAngle

	for _, zone := range gc.thresholdZones() {
		fill := zone.color
		if zone.remainder {
			style := b.StyleGuide()
			fill = lerpColors(style.Palette.Background.Opaque(), style.Palette.Border.Opaque(), 0.30)
			fill = EnsureContrast(fill, style.Palette.Background, WCAGAALarge)
		}

		b.Push()
		b.SetFillColor(fill)
		b.SetStrokeWidth(0)

		gc.drawArc(centerX, centerY, outerRadius, innerRadius,
			startAngle+zone.startRatio*totalAngle, startAngle+zone.endRatio*totalAngle)

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

	// Draw major ticks: at even steps of the range, or, when the dial has
	// threshold zones, at the ends and at every zone bound, so a zone ending
	// at 95 of 100 has its tick and number (go-slide-creator-u3nl6).
	for _, ratio := range gc.majorTickRatios() {
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
			label := gc.config.ValueFmt.FormatOr(value, "%.0f")
			b.DrawText(label, labelX, labelY, align, TextBaselineMiddle)
		}
	}

	// Draw minor ticks (between major ticks); a dial ticked at its zone
	// bounds has none, since they would not fall between its majors.
	b.SetStrokeWidth(1)
	minorTicksPerMajor := 4
	if len(gc.config.Thresholds) > 0 {
		minorTicksPerMajor = 0
	}
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

// majorTickRatios returns the positions of the dial's major ticks as shares
// of the range: TickCount even steps, or the ends plus every threshold bound
// inside the range when the dial has zones.
func (gc *GaugeChart) majorTickRatios() []float64 {
	lo, hi := gc.config.MinValue, gc.config.MaxValue
	if len(gc.config.Thresholds) == 0 {
		out := make([]float64, 0, gc.config.TickCount+1)
		for i := 0; i <= gc.config.TickCount; i++ {
			out = append(out, float64(i)/float64(gc.config.TickCount))
		}
		return out
	}
	out := []float64{0}
	for _, t := range gc.config.Thresholds {
		r := (t.Value - lo) / (hi - lo)
		if r > out[len(out)-1]+1e-9 && r < 1-1e-9 {
			out = append(out, r)
		}
	}
	return append(out, 1)
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
	label := gc.config.ValueFmt.FormatOr(data.Value, gc.config.ValueFormat)
	if data.Unit != "" {
		label = label + data.Unit
	}

	b.DrawText(label, centerX, labelY, TextAlignCenter, TextBaselineMiddle)

	// Draw secondary label if provided: wrapped onto two lines, then cut,
	// where it is wider than the hole it sits in.
	if data.Label != "" {
		size := style.Typography.SizeSmall
		b.SetFontSize(size)
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(style.Palette.TextSecondary)
		holeW := innerRadius * 1.7
		lines := []string{data.Label}
		if w, _ := b.MeasureText(data.Label); w > holeW {
			if wrapped, ok := wrapXLabelsTwoLines(b, lines, holeW, size); ok {
				lines = strings.Split(wrapped[0], "\n")
			}
		}
		for i, line := range lines {
			b.DrawText(b.TruncateToWidth(line, holeW), centerX, labelY+style.Typography.SizeTitle+float64(i)*size*1.2, TextAlignCenter, TextBaselineMiddle)
		}
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

	// An empty or inverted range has no geometry: every value-to-angle ratio
	// divides by max-min, which wrote NaN coordinates into the needle, tick
	// and threshold paths (go-slide-creator-s1uvj.31).
	value, _ := req.Data["value"].(float64)
	if v, ok := req.Data["value"].(int); ok {
		value = float64(v)
	}
	minValue, maxValue := resolveGaugeRange(req.Data, value, DefaultGaugeChartConfig(0, 0))
	if err := validateGaugeRange(minValue, maxValue); err != nil {
		return err
	}

	return nil
}

// validateGaugeRange rejects a gauge scale whose max is not greater than its
// min; such a range cannot be mapped onto the dial.
func validateGaugeRange(minValue, maxValue float64) error {
	if !(maxValue > minValue) {
		return fmt.Errorf("gauge chart requires max greater than min, got min=%v max=%v", minValue, maxValue)
	}
	return nil
}

// resolveGaugeRange returns the effective scale for a gauge request: explicit
// "min"/"max" override the config defaults, and a value in [0,1] with neither
// given selects a 0–1 scale.
func resolveGaugeRange(data map[string]any, value float64, config GaugeChartConfig) (minValue, maxValue float64) {
	minValue, maxValue = config.MinValue, config.MaxValue
	_, hasMin := data["min"]
	_, hasMax := data["max"]
	if v, ok := data["min"].(float64); ok {
		minValue = v
	}
	if v, ok := data["max"].(float64); ok {
		maxValue = v
	}

	// Auto-detect 0-1 scale: if no explicit min/max was provided and
	// the value is in [0,1], assume a 0-1 range instead of the default
	// 0-100. This prevents fractional values like 0.73 from rendering
	// as needle-near-zero on a 0-100 scale.
	if !hasMin && !hasMax && value >= 0 && value <= 1 {
		minValue, maxValue = 0, 1
	}
	return minValue, maxValue
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
		config.ValueFormatSpec = req.Style.ValueFormat
		config.ShowValues = req.Style.ShowValues

		// Apply custom min/max (or the auto-detected 0-1 scale).
		config.MinValue, config.MaxValue = resolveGaugeRange(req.Data, data.Value, config)
		if startAngle, ok := req.Data["start_angle"].(float64); ok {
			config.StartAngle = startAngle
		}
		if endAngle, ok := req.Data["end_angle"].(float64); ok {
			config.EndAngle = endAngle
		}
		// Arc angles describe the dial; asking for either keeps it.
		_, hasStart := req.Data["start_angle"]
		_, hasEnd := req.Data["end_angle"]
		if hasStart || hasEnd {
			config.Style = GaugeStyleDial
		}
		if st, ok := req.Data["style"].(string); ok {
			config.Style = st
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

// DataSchema returns the data contract for gauge charts.
func (d *GaugeDiagram) DataSchema() *DataSchema {
	return diagramDataSchema("Gauge showing one value on a range", map[string]*DataSchema{
		"value":       NumberDataSchema("Value shown"),
		"min":         NumberDataSchema("Range minimum"),
		"max":         NumberDataSchema("Range maximum"),
		"label":       StringDataSchema("Label under the value"),
		"unit":        StringDataSchema("Unit suffix"),
		"footnote":    StringDataSchema("Footnote text"),
		"start_angle": NumberDataSchema("Arc start angle"),
		"end_angle":   NumberDataSchema("Arc end angle"),
		"style":       StringDataSchema("bullet (default: large value over a horizontal bar) or dial (speedometer arc)"),
		"thresholds": ArrayDataSchema("Colored range bands", ObjectDataSchema("A threshold", map[string]*DataSchema{
			"value": NumberDataSchema("Band upper bound"),
			"color": StringDataSchema("Band color"),
			"label": StringDataSchema("Band label"),
		}, nil), 0),
	}, []string{"value"})
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
