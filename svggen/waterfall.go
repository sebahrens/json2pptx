package svggen

import (
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// Waterfall Chart (Bridge Chart)
// =============================================================================

// WaterfallChartType identifies the type of waterfall bar.
type WaterfallChartType string

const (
	// WaterfallTypeIncrease represents a positive change (upward bar).
	WaterfallTypeIncrease WaterfallChartType = "increase"

	// WaterfallTypeDecrease represents a negative change (downward bar).
	WaterfallTypeDecrease WaterfallChartType = "decrease"

	// WaterfallTypeTotal represents a summary bar starting from baseline.
	WaterfallTypeTotal WaterfallChartType = "total"

	// WaterfallTypeSubtotal represents an intermediate subtotal bar.
	WaterfallTypeSubtotal WaterfallChartType = "subtotal"
)

// WaterfallDataPoint represents a single bar in the waterfall chart.
type WaterfallDataPoint struct {
	// Label is the category label for this bar.
	Label string

	// Value is the value of this bar.
	// For increase/decrease types, this is the delta.
	// For total/subtotal types, this is the absolute value.
	Value float64

	// Type determines how this bar is rendered and positioned.
	Type WaterfallChartType

	// Color overrides the default color for this bar.
	Color *Color
}

// WaterfallChartConfig holds configuration for waterfall charts.
type WaterfallChartConfig struct {
	ChartConfig

	// IncreaseColor is the color for positive changes.
	IncreaseColor Color

	// DecreaseColor is the color for negative changes.
	DecreaseColor Color

	// TotalColor is the color for total/subtotal bars.
	TotalColor Color

	// ConnectorColor is the color for connector lines.
	ConnectorColor Color

	// ConnectorWidth is the width of connector lines in points.
	ConnectorWidth float64

	// ConnectorDash enables dashed connectors.
	ConnectorDash bool

	// ShowConnectors enables connector lines between bars.
	ShowConnectors bool

	// BarPadding is the padding between bars (0-1).
	BarPadding float64

	// BarCornerRadius rounds bar corners.
	BarCornerRadius float64
}

// DefaultWaterfallChartConfig returns default waterfall chart configuration.
func DefaultWaterfallChartConfig(width, height float64) WaterfallChartConfig {
	cfg := DefaultChartConfig(width, height)
	// Waterfall charts ALWAYS show values by default - the numeric deltas are essential
	// to understanding the flow from one state to another (e.g., Revenue → Net Income)
	// Without labels, viewers cannot interpret what each bar represents
	cfg.ShowValues = true

	return WaterfallChartConfig{
		ChartConfig:     cfg,
		IncreaseColor:   MustParseColor("#59A14F"), // Green
		DecreaseColor:   MustParseColor("#E15759"), // Red
		TotalColor:      MustParseColor("#4E79A7"), // Blue
		ConnectorColor:  MustParseColor("#6C757D"), // Gray
		ConnectorWidth:  1.5,
		ConnectorDash:   false,
		ShowConnectors:  true,
		BarPadding:      0.2,
		BarCornerRadius: 0,
	}
}

// WaterfallChart renders waterfall/bridge charts.
type WaterfallChart struct {
	builder *SVGBuilder
	config  WaterfallChartConfig
}

// NewWaterfallChart creates a new waterfall chart renderer.
func NewWaterfallChart(builder *SVGBuilder, config WaterfallChartConfig) *WaterfallChart {
	return &WaterfallChart{
		builder: builder,
		config:  config,
	}
}

// WaterfallData represents the data for a waterfall chart.
type WaterfallData struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Points are the data points for the waterfall.
	Points []WaterfallDataPoint

	// Footnote is an optional footnote text.
	Footnote string
}

// Draw renders the waterfall chart.
func (wc *WaterfallChart) Draw(data WaterfallData) error {
	if len(data.Points) == 0 {
		return fmt.Errorf("waterfall chart requires at least one data point")
	}

	// Auto-detect decimal precision: if using the default integer format but
	// data contains fractional values, switch to one decimal place.
	if wc.config.ValueFormat == "%.0f" {
		for _, p := range data.Points {
			if p.Value != math.Trunc(p.Value) {
				wc.config.ValueFormat = "%.1f"
				break
			}
		}
	}

	b := wc.builder
	style := b.StyleGuide()

	// Extract categories
	categories := make([]string, len(data.Points))
	for i, p := range data.Points {
		categories[i] = p.Label
	}

	// Narrow-width detection: when the chart is narrow (e.g., half_16x9 at 760px,
	// or third_16x9 at 500px), labels need special treatment to avoid truncation
	// and overlap. We consider anything below 500pt "narrow".
	isNarrow := wc.config.Width < 500

	// For narrow charts, increase the left margin so y-axis tick labels
	// (which can be 3-4 digit numbers) don't overlap with the bar bodies.
	if isNarrow {
		extraLeft := wc.config.MarginLeft * 0.25
		wc.config.MarginLeft += extraLeft
	}

	// Track which labels are "important" (totals/subtotals) — always shown.
	importantLabels := make(map[int]bool)
	for i, p := range data.Points {
		if p.Type == WaterfallTypeTotal || p.Type == WaterfallTypeSubtotal {
			importantLabels[i] = true
		}
	}

	// Compute adaptive x-axis label layout (font size, rotation, thinning,
	// and truncation) using the shared strategy so labels are not clipped.
	prelimPlotW := wc.config.Width - wc.config.MarginLeft - wc.config.MarginRight
	xLayout := AdaptXLabels(b, categories, prelimPlotW, style.Typography.SizeBody, isNarrow)
	axisFontSize := xLayout.FontSize
	xLabelRotation := xLayout.Rotation
	labelStep := xLayout.LabelStep
	categories = xLayout.Categories
	if xLayout.ExtraBottomMargin > 0 {
		wc.config.MarginBottom += xLayout.ExtraBottomMargin
	}

	// Calculate layout (shared across Cartesian chart types; fixes missing footerHeight)
	layout := ComputeCartesianLayout(wc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, 1)
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight

	// Calculate domain
	yMin, yMax := wc.calculateDomain(data.Points)

	// Create scales
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)
	xScale.PaddingOuter(wc.config.BarPadding)
	xScale.PaddingInner(wc.config.BarPadding)

	yScale := NewLinearScale(yMin, yMax)
	yScale.SetRangeLinear(plotArea.H, 0)
	yScale.Nice(true)

	// Draw grid
	if wc.config.ShowGrid {
		DrawCartesianGrid(b, plotArea, yScale, nil)
	}

	// Draw axes
	if wc.config.ShowAxes {
		wc.drawAxes(plotArea, xScale, yScale, axisFontSize, xLabelRotation, labelStep, importantLabels)
	}

	// Draw bars and connectors
	wc.drawBarsAndConnectors(data.Points, plotArea, xScale, yScale, isNarrow)

	// Draw title
	if wc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: wc.config.Width, H: headerHeight + wc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: wc.config.Height - layout.FooterHeight,
			W: wc.config.Width,
			H: layout.FooterHeight,
		})
	}

	return nil
}

// calculateDomain calculates the y-axis domain for the waterfall chart.
//
// When the running totals are all far from zero (e.g., a bridge from 485M to 528M),
// forcing the y-axis to start at 0 would compress the incremental bars into tiny
// slivers. Instead, this function uses a "broken" y-axis that zooms in on the
// data range, with ~10% padding on each side. Zero is still included when:
//   - Any running total actually reaches or crosses zero
//   - The data is close enough to zero that including it wouldn't more than
//     double the visible axis range
func (wc *WaterfallChart) calculateDomain(points []WaterfallDataPoint) (min, max float64) {
	// Calculate running total to find min/max of actual data values.
	// Initialize min/max to extreme opposites so the first value always wins.
	var running float64
	min = math.MaxFloat64
	max = -math.MaxFloat64

	for i, p := range points {
		switch p.Type {
		case WaterfallTypeTotal, WaterfallTypeSubtotal:
			// Total bars show absolute value
			running = p.Value
		default:
			// First bar or increment/decrement
			if i == 0 && (p.Type == "" || p.Type == WaterfallTypeIncrease || p.Type == WaterfallTypeDecrease) {
				// First bar might be a starting value
				running = p.Value
			} else {
				running += p.Value
			}
		}

		if running > max {
			max = running
		}
		if running < min {
			min = running
		}
	}

	// Guard: if no points were processed, fall back to 0-1.
	if min == math.MaxFloat64 {
		return 0, 1
	}

	// Decide whether to include zero in the domain.
	// We include zero if:
	//   1. The data already crosses zero (min <= 0 <= max), OR
	//   2. Including zero wouldn't more than double the data span
	//      (i.e., the data is "close to" zero relative to its own extent).
	dataSpan := max - min
	if dataSpan == 0 {
		// All points at same value — add a small range around it
		if max == 0 {
			return 0, 1
		}
		padding := math.Abs(max) * 0.1
		return max - padding, max + padding
	}

	if min > 0 {
		// All data is positive. Include zero only if the gap from 0 to min
		// is not larger than the data span (otherwise it compresses the bars).
		if min <= dataSpan {
			min = 0
		} else {
			// "Broken" axis: add 10% padding below the minimum.
			min -= dataSpan * 0.1
			// Clamp: never go below 0 if we're close (avoids a weird tiny
			// negative axis start).
			if min < 0 {
				min = 0
			}
		}
	}

	if max < 0 {
		// All data is negative. Include zero only if the gap from max to 0
		// is not larger than the data span.
		if -max <= dataSpan {
			max = 0
		} else {
			max += dataSpan * 0.1
			if max > 0 {
				max = 0
			}
		}
	}

	// Add a small amount of headroom above max (and below min if not 0)
	// so bars don't touch the chart edge. applyNice() will further round
	// these, but we add padding first to make sure the nice rounding
	// doesn't collapse the range.
	headroom := dataSpan * 0.05
	if min != 0 {
		min -= headroom
	}
	if max != 0 {
		max += headroom
	}

	return min, max
}

// drawAxes draws the chart axes.
func (wc *WaterfallChart) drawAxes(plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, axisFontSize float64, xLabelRotation float64, labelStep int, importantLabels map[int]bool) {
	b := wc.builder

	// X axis — horizontal by default, rotated when labels are dense
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = wc.config.XAxisTitle
	xAxisConfig.FontSize = axisFontSize
	xAxisConfig.LabelRotation = xLabelRotation

	if labelStep > 1 {
		// Label thinning: hide axis labels, we draw them manually below
		xAxisConfig.HideLabels = true
	}

	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawCategoricalAxis(xScale, plotArea.X, plotArea.Y+plotArea.H)

	// If thinning, draw every Nth label manually (always show first and last)
	if labelStep > 1 {
		style := b.StyleGuide()
		cats := xScale.Categories()
		b.Push()
		b.SetFontSize(axisFontSize)
		b.SetFontWeight(style.Typography.WeightNormal)
		lastIdx := len(cats) - 1
		for i, cat := range cats {
			if i != 0 && i%labelStep != 0 && i != lastIdx && !importantLabels[i] {
				continue
			}
			labelX := plotArea.X + xScale.Scale(cat)
			labelY := plotArea.Y + plotArea.H + xAxisConfig.TickSize + xAxisConfig.TickPadding

			if xLabelRotation != 0 {
				labelY += axisFontSize // offset to avoid overlapping axis line
				b.Push()
				b.RotateAround(xLabelRotation, labelX, labelY)
				// Negative rotation (e.g. -45°): text-anchor:end so label
				// extends down-left away from the chart area.
				rotAlign := TextAlignLeft
				if xLabelRotation < 0 {
					rotAlign = TextAlignRight
				}
				b.DrawText(cat, labelX, labelY, rotAlign, TextBaselineTop)
				b.Pop()
			} else {
				b.DrawText(cat, labelX, labelY, TextAlignCenter, TextBaselineTop)
			}
		}
		b.Pop()
	}

	// Y axis (shared)
	DrawCartesianYAxis(b, plotArea, yScale, wc.config.YAxisTitle)
}

// drawBarsAndConnectors draws the waterfall bars and connecting lines.
func (wc *WaterfallChart) drawBarsAndConnectors(points []WaterfallDataPoint, plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, isNarrow bool) {
	b := wc.builder
	style := b.StyleGuide()

	bandwidth := xScale.Bandwidth()
	barWidth := bandwidth * (1 - wc.config.BarPadding)
	numPoints := len(points)

	// Ensure bars don't get too thin at narrow widths.
	// Minimum bar width of 12pt keeps bars visually distinct.
	minBarWidth := 12.0
	if barWidth < minBarWidth && bandwidth > minBarWidth {
		barWidth = minBarWidth
	}

	// ── Density-adaptive value label settings ──
	// At high density (10+ bars), value labels overlap connector lines.
	// We apply three strategies:
	// 1. Thin labels: show only every Nth label (totals/subtotals always shown)
	// 2. Reduce font size so labels occupy less vertical space
	// 3. Switch connectors to dashed lines to reduce visual weight
	valueLabelStep := 1 // show every Nth value label (1 = all)
	isDense := numPoints >= 8

	// Identify "important" bars (totals/subtotals) — always show their labels.
	importantBars := make(map[int]bool)
	for i, p := range points {
		if p.Type == WaterfallTypeTotal || p.Type == WaterfallTypeSubtotal {
			importantBars[i] = true
		}
	}

	// Determine value label thinning step for dense charts
	if isNarrow && numPoints >= 12 {
		valueLabelStep = 3
	} else if isNarrow && numPoints >= 8 {
		valueLabelStep = 2
	} else if numPoints >= 14 {
		valueLabelStep = 3
	} else if numPoints >= 10 {
		valueLabelStep = 2
	}

	// At high density, switch connectors to dashed to reduce visual clutter
	// so that value labels (which are more informative) stand out.
	useDashedConnectors := isDense && wc.config.ShowConnectors && !wc.config.ConnectorDash

	// Determine the visual baseline for total/subtotal bars.
	// When the y-axis includes 0, total bars extend from 0 to their value.
	// When using a broken axis (y-min > 0), total bars extend from the
	// bottom of the visible chart area (y-axis minimum) to their value,
	// creating a visually "broken" column.
	yDomainMin, _ := yScale.DomainBounds()
	if yDomainMin < 0 {
		yDomainMin = 0
	}
	baseY := plotArea.Y + yScale.Scale(yDomainMin)
	var running float64
	var prevBarEnd float64 // End position of previous bar for connectors

	for i, p := range points {
		x := plotArea.X + xScale.Scale(p.Label)

		var barTop, barBottom float64
		var color Color

		switch p.Type {
		case WaterfallTypeTotal, WaterfallTypeSubtotal:
			// Total bars extend from baseline to value
			barTop = plotArea.Y + yScale.Scale(p.Value)
			barBottom = baseY
			color = wc.config.TotalColor
			if p.Color != nil {
				color = *p.Color
			}
			running = p.Value

		default:
			// Increase/decrease bars are floating
			if i == 0 {
				// First bar: treat as starting value from baseline
				barTop = plotArea.Y + yScale.Scale(p.Value)
				barBottom = baseY
				if p.Value >= 0 {
					color = wc.config.IncreaseColor
				} else {
					color = wc.config.DecreaseColor
				}
				running = p.Value
			} else {
				prevRunning := running
				running += p.Value

				if p.Value >= 0 {
					// Positive: bar floats upward from previous running total
					barTop = plotArea.Y + yScale.Scale(running)
					barBottom = plotArea.Y + yScale.Scale(prevRunning)
					color = wc.config.IncreaseColor
				} else {
					// Negative: bar floats downward from previous running total
					barTop = plotArea.Y + yScale.Scale(prevRunning)
					barBottom = plotArea.Y + yScale.Scale(running)
					color = wc.config.DecreaseColor
				}
			}

			if p.Color != nil {
				color = *p.Color
			}
		}

		// Handle bar direction (ensure top < bottom)
		if barTop > barBottom {
			barTop, barBottom = barBottom, barTop
		}

		barHeight := barBottom - barTop
		if barHeight < 1 && p.Value != 0 {
			barHeight = 1
		}

		// Draw connector from previous bar
		if wc.config.ShowConnectors && i > 0 {
			wc.drawConnectorAdaptive(prevBarEnd, x-barWidth/2, barTop, barBottom, points[i-1], p, useDashedConnectors)
		}

		// Draw the bar
		rect := Rect{
			X: x - barWidth/2,
			Y: barTop,
			W: barWidth,
			H: barHeight,
		}

		b.Push()
		b.SetFillColor(color)
		if wc.config.BarCornerRadius > 0 {
			b.DrawRoundedRect(rect, wc.config.BarCornerRadius)
		} else {
			b.FillRect(rect)
		}
		b.Pop()

		// Draw value label (with density-adaptive thinning)
		if wc.config.ShowValues {
			// Determine whether to show this label:
			// - Important bars (totals/subtotals) are always shown
			// - First and last bars are always shown
			// - Otherwise, show every Nth bar based on valueLabelStep
			showLabel := importantBars[i] || i == 0 || i == numPoints-1 || valueLabelStep <= 1 || i%valueLabelStep == 0

			if showLabel {
				b.Push()
				// Use smaller font for value labels on narrow/dense charts to prevent
				// labels from overlapping adjacent bars and connector lines.
				// SizeBody (11pt floor) ensures readability; SizeCaption (10pt floor)
				// for narrow/dense charts keeps labels legible without overlap.
				valueFontSize := style.Typography.SizeBody
				if isNarrow || isDense {
					valueFontSize = style.Typography.SizeCaption
				}
				b.SetFontSize(valueFontSize)
				b.SetFontWeight(style.Typography.WeightNormal)

				label := formatValue(p.Value, wc.config.ValueFormat)
				if p.Value > 0 && p.Type != WaterfallTypeTotal && p.Type != WaterfallTypeSubtotal && i > 0 {
					label = "+" + label
				}

				// Use larger spacing to avoid connector line overlap.
				// Connectors are drawn at bar edges, so we need extra padding.
				// At high density, increase clearance further since bars are
				// closer together and connectors span the full inter-bar gap.
				connectorClearance := wc.config.ConnectorWidth + style.Spacing.SM
				if isDense {
					connectorClearance += style.Spacing.XS
				}
				labelY := barTop - connectorClearance
				if p.Value < 0 {
					labelY = barBottom + valueFontSize + connectorClearance
				}

				// Clamp to canvas bounds to avoid clipping
				if labelY < valueFontSize+style.Spacing.XS {
					labelY = valueFontSize + style.Spacing.XS
				}
				if labelY > wc.config.Height-style.Spacing.XS {
					labelY = wc.config.Height - style.Spacing.XS
				}

				b.DrawText(label, x, labelY, TextAlignCenter, TextBaselineBottom)
				b.Pop()
			}
		}

		// Store bar end for connector
		prevBarEnd = x + barWidth/2
	}
}

// drawConnector draws a connector line between bars.
func (wc *WaterfallChart) drawConnector(prevX, currX, currTop, currBottom float64, prevPoint, currPoint WaterfallDataPoint) {
	wc.drawConnectorAdaptive(prevX, currX, currTop, currBottom, prevPoint, currPoint, false)
}

// drawConnectorAdaptive draws a connector line between bars, with optional
// density-adaptive dashing. When forceDash is true, connectors are rendered
// with a short dash pattern regardless of the config.ConnectorDash setting.
// This reduces visual clutter at high density so value labels stand out.
func (wc *WaterfallChart) drawConnectorAdaptive(prevX, currX, currTop, currBottom float64, prevPoint, currPoint WaterfallDataPoint, forceDash bool) {
	b := wc.builder

	// Determine connector Y position based on bar types
	var connectorY float64

	if currPoint.Type == WaterfallTypeTotal || currPoint.Type == WaterfallTypeSubtotal {
		// No connector to total bars (they start from baseline)
		return
	}

	// Connect from the end of previous bar to start of current bar
	// For positive changes, connect at the top of current bar
	// For negative changes, connect at the bottom of current bar (where it starts)
	if currPoint.Value >= 0 {
		connectorY = currBottom // Bottom of current bar (where it starts, which is where prev ended)
	} else {
		connectorY = currTop // Top of current bar (where it starts, which is where prev ended)
	}

	b.Push()
	b.SetStrokeColor(wc.config.ConnectorColor)
	b.SetStrokeWidth(wc.config.ConnectorWidth)

	if wc.config.ConnectorDash || forceDash {
		b.SetDashes(3, 2)
	}

	b.DrawLine(prevX, connectorY, currX, connectorY)
	b.Pop()
}

// =============================================================================
// Waterfall Chart Diagram Type (for Registry)
// =============================================================================

// WaterfallDiagram implements the Diagram interface for waterfall charts.
type WaterfallDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for waterfall charts.
func (d *WaterfallDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("waterfall chart requires data. Expected format: {\"points\": [{\"label\": \"Revenue\", \"value\": 100}, {\"label\": \"Costs\", \"value\": -40}]}")
	}

	// Accept "labels" + "values" arrays as alternative to "points"
	normalizeWaterfallData(req)

	// Check for points array
	pointsRaw, ok := req.Data["points"]
	if !ok {
		return fmt.Errorf("waterfall chart requires 'points' array in data. Expected format: {\"points\": [{\"label\": \"Revenue\", \"value\": 100}, {\"label\": \"Costs\", \"value\": -40, \"type\": \"negative\"}]} or {\"labels\": [\"Revenue\", \"Costs\"], \"values\": [100, -40]}")
	}

	// Accept multiple array types: []any (same as []interface{}), []map[string]any
	var pointsLen int
	switch p := pointsRaw.(type) {
	case []any:
		pointsLen = len(p)
	case []map[string]any:
		pointsLen = len(p)
	default:
		return fmt.Errorf("waterfall chart 'points' must be an array of objects, e.g. [{\"label\": \"Revenue\", \"value\": 100}, {\"label\": \"Costs\", \"value\": -40}]")
	}

	if pointsLen == 0 {
		return fmt.Errorf("waterfall chart requires at least one point, e.g. {\"points\": [{\"label\": \"Revenue\", \"value\": 100}]}")
	}

	return nil
}

// normalizeWaterfallData converts "labels" + "values" format to "points" format.
func normalizeWaterfallData(req *RequestEnvelope) {
	if _, hasPoints := req.Data["points"]; hasPoints {
		return
	}
	labelsRaw, hasLabels := req.Data["labels"]
	valuesRaw, hasValues := req.Data["values"]
	if !hasLabels || !hasValues {
		return
	}
	labels, labelsOK := toStringSlice(labelsRaw)
	values, valuesOK := toFloat64Slice(valuesRaw)
	if !labelsOK || !valuesOK {
		return
	}
	n := len(labels)
	if len(values) < n {
		n = len(values)
	}
	points := make([]any, n)
	for i := 0; i < n; i++ {
		points[i] = map[string]any{
			"label": labels[i],
			"value": values[i],
		}
	}
	req.Data["points"] = points
}

// Render generates an SVG document from the request envelope.
func (d *WaterfallDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
// This allows callers to generate PNG/PDF output from the same builder.
func (d *WaterfallDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseWaterfallData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultWaterfallChartConfig(width, height)
		// Waterfall charts ALWAYS show values by default - the numeric deltas are essential
		// to understanding the flow (e.g., Revenue → Net Income)
		// Note: DefaultWaterfallChartConfig sets ShowValues=true, so we keep that default
		config.ShowGrid = req.Style.ShowGrid
		// ShowLegend kept at default true; Draw only renders for multi-series.

		// Override hardcoded config colors with theme palette so diagrams
		// respect the active template's color scheme.
		// These are applied BEFORE user custom colors so that explicit
		// user-specified colors take priority over the theme defaults.
		style := builder.StyleGuide()
		config.IncreaseColor = style.Palette.Success
		config.DecreaseColor = style.Palette.Error
		config.TotalColor = style.Palette.Primary
		config.ConnectorColor = style.Palette.TextMuted

		// Apply custom colors if specified (overrides theme defaults)
		if colors, ok := req.Data["colors"].(map[string]any); ok {
			if inc, ok := colors["increase"].(string); ok {
				if c, err := ParseColor(inc); err == nil {
					config.IncreaseColor = c
				}
			}
			if dec, ok := colors["decrease"].(string); ok {
				if c, err := ParseColor(dec); err == nil {
					config.DecreaseColor = c
				}
			}
			if tot, ok := colors["total"].(string); ok {
				if c, err := ParseColor(tot); err == nil {
					config.TotalColor = c
				}
			}
		}

		chart := NewWaterfallChart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseWaterfallData parses the request data into WaterfallData.
func parseWaterfallData(req *RequestEnvelope) (WaterfallData, error) {
	data := WaterfallData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse points - handle multiple array types
	// Note: []any and []interface{} are the same type (any is alias for interface{})
	// but []map[string]any is a different type that needs separate handling
	pointsRaw := req.Data["points"]
	var points []any
	switch p := pointsRaw.(type) {
	case []any:
		points = p
	case []map[string]any:
		points = make([]any, len(p))
		for i, v := range p {
			points[i] = v
		}
	default:
		return data, fmt.Errorf("invalid points data")
	}

	data.Points = make([]WaterfallDataPoint, 0, len(points))

	for _, pRaw := range points {
		point := WaterfallDataPoint{}

		// Note: map[string]any and map[string]interface{} are the same type
		p, ok := pRaw.(map[string]any)
		if !ok {
			return data, fmt.Errorf("invalid point format")
		}

		if label, ok := p["label"].(string); ok {
			point.Label = label
		}
		if value, ok := p["value"].(float64); ok {
			point.Value = value
		} else if value, ok := p["value"].(int); ok {
			point.Value = float64(value)
		}
		if typ, ok := p["type"].(string); ok {
			point.Type = WaterfallChartType(strings.ToLower(typ))
		}
		if colorStr, ok := p["color"].(string); ok {
			if c, err := ParseColor(colorStr); err == nil {
				point.Color = &c
			}
		}

		// Infer type from explicit label or value if not specified
		if point.Type == "" {
			lower := strings.ToLower(point.Label)
			if strings.Contains(lower, "subtotal") || strings.Contains(lower, "sub-total") {
				point.Type = WaterfallTypeSubtotal
			} else if strings.Contains(lower, "total") {
				point.Type = WaterfallTypeTotal
			} else if point.Value >= 0 {
				point.Type = WaterfallTypeIncrease
			} else {
				point.Type = WaterfallTypeDecrease
			}
		}

		data.Points = append(data.Points, point)
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

// =============================================================================
// Convenience Functions
// =============================================================================

// DrawWaterfallFromData creates and draws a waterfall chart from simple data.
func DrawWaterfallFromData(builder *SVGBuilder, title string, points []WaterfallDataPoint) error {
	config := DefaultWaterfallChartConfig(builder.Width(), builder.Height())
	config.ShowValues = true
	config.ShowGrid = true

	chart := NewWaterfallChart(builder, config)
	return chart.Draw(WaterfallData{
		Title:  title,
		Points: points,
	})
}

// CreateWaterfallPoints creates a series of waterfall data points from values.
// The first value is treated as the starting point, and the last can be a total.
func CreateWaterfallPoints(labels []string, values []float64, markLastAsTotal bool) []WaterfallDataPoint {
	n := len(values)
	if len(labels) < n {
		n = len(labels)
	}

	points := make([]WaterfallDataPoint, n)

	for i := 0; i < n; i++ {
		points[i] = WaterfallDataPoint{
			Label: labels[i],
			Value: values[i],
		}

		// Infer type
		if i == n-1 && markLastAsTotal {
			points[i].Type = WaterfallTypeTotal
		} else if values[i] >= 0 {
			points[i].Type = WaterfallTypeIncrease
		} else {
			points[i].Type = WaterfallTypeDecrease
		}
	}

	return points
}


// Utility to calculate absolute values from deltas
func calculateRunningTotals(points []WaterfallDataPoint) []float64 {
	totals := make([]float64, len(points))
	var running float64

	for i, p := range points {
		switch p.Type {
		case WaterfallTypeTotal, WaterfallTypeSubtotal:
			running = p.Value
		default:
			if i == 0 {
				running = p.Value
			} else {
				running += p.Value
			}
		}
		totals[i] = running
	}

	return totals
}
