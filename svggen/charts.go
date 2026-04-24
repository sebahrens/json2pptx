package svggen

import (
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// Common Chart Types
// =============================================================================

// ChartType identifies the type of chart.
type ChartType string

const (
	ChartTypeBar     ChartType = "bar"
	ChartTypeLine    ChartType = "line"
	ChartTypeArea    ChartType = "area"
	ChartTypeScatter ChartType = "scatter"
	ChartTypePie     ChartType = "pie"
	ChartTypeDonut   ChartType = "donut"
	ChartTypeRadar   ChartType = "radar"
)

// ChartData represents the data for a chart.
type ChartData struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Categories are the x-axis categories (for categorical charts).
	Categories []string

	// Series contains the data series.
	Series []ChartSeries

	// Footnote is an optional footnote text.
	Footnote string
}

// ChartSeries represents a single data series.
type ChartSeries struct {
	// Name is the series name (used in legend).
	Name string

	// Values are the data values.
	Values []float64

	// XValues are the x-coordinates (for scatter/bubble charts).
	XValues []float64

	// BubbleValues are the size dimension for bubble charts.
	// Each value corresponds to the bubble radius at the same index.
	BubbleValues []float64

	// TimeValues are Unix timestamps for time-series charts.
	// When set, the chart will use a time scale for the X axis.
	TimeValues []int64

	// TimeStrings are time values as strings (ISO8601, etc.).
	// Parsed to TimeValues during rendering.
	TimeStrings []string

	// Color overrides the default series color.
	Color *Color

	// Labels are optional labels for each data point.
	Labels []string
}

// HasTimeData returns true if this series contains time-series data.
func (cs ChartSeries) HasTimeData() bool {
	return len(cs.TimeValues) > 0 || len(cs.TimeStrings) > 0
}

// GetTimeValues returns the time values, parsing TimeStrings if necessary.
// Returns nil if no time data is present.
func (cs ChartSeries) GetTimeValues() ([]int64, error) {
	if len(cs.TimeValues) > 0 {
		return cs.TimeValues, nil
	}
	if len(cs.TimeStrings) > 0 {
		return ParseTimeStrings(cs.TimeStrings)
	}
	return nil, nil
}

// ScaledMargins returns margins proportional to chart dimensions.
// Reference: 800x600 chart has margins of top=40, right=20, bottom=60, left=60.
// Margins scale based on geometric mean of dimension ratios, clamped to [0.5, 2.0].
func ScaledMargins(width, height float64) (top, right, bottom, left float64) {
	const (
		refWidth  = 800.0
		refHeight = 600.0
		refTop    = 40.0
		refRight  = 20.0
		refBottom = 60.0
		refLeft   = 60.0
	)

	// Scale based on geometric mean of dimension ratios
	scale := math.Sqrt((width * height) / (refWidth * refHeight))

	// Clamp scale to reasonable bounds (0.5x to 2.0x)
	scale = math.Max(0.5, math.Min(2.0, scale))

	top = refTop * scale
	right = refRight * scale
	bottom = refBottom * scale
	left = refLeft * scale

	// Narrow aspect-ratio adjustment: when W/H < 0.6 (portrait/narrow layout),
	// reduce left/right margins by 30% to give more horizontal space to content.
	if height > 0 && width/height < 0.6 {
		right *= 0.7
		left *= 0.7
	}

	return top, right, bottom, left
}

// ChartConfig holds common configuration for all chart types.
type ChartConfig struct {
	// Width is the total chart width in points.
	Width float64

	// Height is the total chart height in points.
	Height float64

	// Margins around the chart area.
	MarginTop    float64
	MarginRight  float64
	MarginBottom float64
	MarginLeft   float64

	// ShowLegend enables the legend.
	ShowLegend bool

	// LegendPosition determines legend placement.
	LegendPosition LegendPosition

	// ShowTitle enables the title.
	ShowTitle bool

	// ShowAxes enables axes (where applicable).
	ShowAxes bool

	// ShowGrid enables grid lines.
	ShowGrid bool

	// ShowValues enables value labels on data points.
	ShowValues bool

	// ValueFormat is the printf-style format for values.
	ValueFormat string

	// XAxisTitle is the x-axis title.
	XAxisTitle string

	// YAxisTitle is the y-axis title.
	YAxisTitle string

	// Colors is the color palette for series.
	Colors []Color

	// Animate enables animation hints (for interactive output).
	Animate bool
}

// DefaultChartConfig returns a default chart configuration.
func DefaultChartConfig(width, height float64) ChartConfig {
	top, right, bottom, left := ScaledMargins(width, height)
	return ChartConfig{
		Width:          width,
		Height:         height,
		MarginTop:      top,
		MarginRight:    right,
		MarginBottom:   bottom,
		MarginLeft:     left,
		ShowLegend:     true,
		LegendPosition: LegendPositionBottom,
		ShowTitle:      true,
		ShowAxes:       true,
		ShowGrid:       true,
		ShowValues:     false,
		ValueFormat:    "%.0f",
		Colors:         nil, // Use palette
	}
}

// PlotArea returns the inner plot area rect.
// W and H are clamped to zero so that oversized margins never produce
// negative dimensions (which would cause panics or inverted rendering).
func (c ChartConfig) PlotArea() Rect {
	return Rect{
		X: c.MarginLeft,
		Y: c.MarginTop,
		W: math.Max(0, c.Width-c.MarginLeft-c.MarginRight),
		H: math.Max(0, c.Height-c.MarginTop-c.MarginBottom),
	}
}

// =============================================================================
// Bar Chart
// =============================================================================

// BarChartConfig holds configuration specific to bar charts.
type BarChartConfig struct {
	ChartConfig

	// Horizontal renders bars horizontally.
	Horizontal bool

	// Stacked stacks series on top of each other.
	Stacked bool

	// BarPadding is the padding between bars (0-1).
	BarPadding float64

	// GroupPadding is the padding between bar groups (0-1).
	GroupPadding float64

	// CornerRadius rounds bar corners.
	CornerRadius float64
}

// DefaultBarChartConfig returns default bar chart configuration.
func DefaultBarChartConfig(width, height float64) BarChartConfig {
	return BarChartConfig{
		ChartConfig:  DefaultChartConfig(width, height),
		Horizontal:   false,
		Stacked:      false,
		BarPadding:   0.1,
		GroupPadding: 0.2,
		CornerRadius: 0,
	}
}

// BarChart renders bar charts.
type BarChart struct {
	builder  *SVGBuilder
	config   BarChartConfig
	logScale *LogScale // non-nil when auto-log-scale is active
}

// NewBarChart creates a new bar chart renderer.
func NewBarChart(builder *SVGBuilder, config BarChartConfig) *BarChart {
	return &BarChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the bar chart.
func (bc *BarChart) Draw(data ChartData) error {
	if len(data.Series) == 0 || len(data.Categories) == 0 {
		return fmt.Errorf("bar chart requires at least one series and categories")
	}

	b := bc.builder
	style := b.StyleGuide()
	colors := bc.getColors(style, len(data.Series))

	// Compute adaptive x-axis label layout (font size, rotation, thinning,
	// and truncation) using the shared strategy so labels are not clipped.
	isNarrow := bc.config.Width < 500
	prelimPlotW := bc.config.Width - bc.config.MarginLeft - bc.config.MarginRight
	xLayout := AdaptXLabels(b, data.Categories, prelimPlotW, style.Typography.SizeSmall, isNarrow)
	axisFontSize := xLayout.FontSize
	xLabelRotation := xLayout.Rotation
	labelStep := xLayout.LabelStep
	categories := xLayout.Categories
	if xLayout.ExtraBottomMargin > 0 {
		bc.config.MarginBottom += xLayout.ExtraBottomMargin
	}

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(bc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if bc.config.ShowLegend {
		RefineLegendHeight(b, style, data.Series, &plotArea, &legendHeight)
	}

	// Calculate domain
	yMin, yMax := bc.calculateDomain(data)

	// Detect all-zero series: flat/blank chart.
	if yMin == 0 && yMax == 0 {
		b.AddFinding(Finding{
			Field:    "data.series",
			Code:     FindingAllZeroSeries,
			Message:  "all series values are zero — chart will render flat/blank",
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReplaceValue,
				Params: map[string]any{"series_count": len(data.Series)},
			},
		})
	}

	// Create scales (use potentially-truncated categories for label display)
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)
	xScale.PaddingOuter(bc.config.GroupPadding)
	xScale.PaddingInner(bc.config.GroupPadding)

	// Use a display data copy with potentially-truncated categories for drawing
	displayData := data
	displayData.Categories = categories

	// ── Auto-detect extreme value ranges for log scale ──
	// When positive values span 3+ orders of magnitude (e.g. 0.001 to 1,000,000)
	// linear scale compresses small bars to invisible slivers. Switch to log10
	// scale automatically so every bar gets proportional visual weight.
	// Log scale is only used for non-stacked charts with all-positive data.
	bc.logScale = nil
	if !bc.config.Stacked && bc.needsLogScale(data) {
		logMin, logMax := bc.logDomainBounds(data)
		bc.logScale = NewLogScale(logMin, logMax)
		bc.logScale.SetRangeLog(plotArea.H, 0)

		// Draw grid and axes using log scale
		if bc.config.ShowGrid {
			bc.drawLogGrid(plotArea)
		}
		if bc.config.ShowAxes {
			bc.drawLogAxes(plotArea, xScale, axisFontSize, xLabelRotation, labelStep)
		}
		// Draw bars using log scale
		bc.drawBars(displayData, plotArea, xScale, nil, colors)
	} else {
		yScale := NewLinearScale(yMin, yMax)
		yScale.SetRangeLinear(plotArea.H, 0)
		yScale.Nice(true)

		// Draw grid
		if bc.config.ShowGrid {
			DrawCartesianGrid(b, plotArea, yScale, nil)
		}

		// Draw axes
		if bc.config.ShowAxes {
			bc.drawAxes(plotArea, xScale, yScale, axisFontSize, xLabelRotation, labelStep)
		}

		// Draw bars
		if bc.config.Stacked {
			bc.drawStackedBars(displayData, plotArea, xScale, yScale, colors)
		} else {
			bc.drawBars(displayData, plotArea, xScale, yScale, colors)
		}
	}

	// Draw title
	if bc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: bc.config.Width, H: headerHeight + bc.config.MarginTop})
	}

	// Draw legend
	if bc.config.ShowLegend && len(data.Series) > 1 {
		legendConfig := PresentationLegendConfig(style)
		legendConfig.Position = bc.config.LegendPosition

		legend := NewLegend(b, legendConfig)

		items := make([]LegendItem, len(data.Series))
		for i, series := range data.Series {
			items[i] = LegendItem{
				Label: series.Name,
				Color: colors[i%len(colors)],
			}
			if series.Color != nil {
				items[i].Color = *series.Color
			}
		}
		legend.SetItems(items)

		// Calculate x-axis label space: tick size + tick padding + label height + extra gap
		xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
		xAxisLabelSpace := xAxisConfig.TickSize + xAxisConfig.TickPadding + style.Typography.SizeSmall + style.Spacing.SM

		legendBounds := Rect{
			X: plotArea.X,
			Y: plotArea.Y + plotArea.H + xAxisLabelSpace,
			W: plotArea.W,
			H: legendHeight,
		}
		legend.Draw(legendBounds)
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: bc.config.Height - layout.FooterHeight,
			W: bc.config.Width,
			H: layout.FooterHeight,
		})
	}

	return nil
}

// calculateDomain calculates the y-axis domain.
func (bc *BarChart) calculateDomain(data ChartData) (min, max float64) {
	min = 0
	max = 0

	if bc.config.Stacked {
		// For stacked bars, calculate max stack height
		for i := range data.Categories {
			stackSum := 0.0
			for _, series := range data.Series {
				if i < len(series.Values) {
					stackSum += series.Values[i]
				}
			}
			if stackSum > max {
				max = stackSum
			}
		}
	} else {
		// For grouped bars, find max value
		for _, series := range data.Series {
			for _, v := range series.Values {
				if v > max {
					max = v
				}
				if v < min {
					min = v
				}
			}
		}
	}

	// Include zero in domain
	if min > 0 {
		min = 0
	}

	return min, max
}

// needsLogScale returns true when the chart data spans enough orders of
// magnitude that a log scale would be more readable than linear.
// Requires all values to be positive (or zero).
func (bc *BarChart) needsLogScale(data ChartData) bool {
	var minPos float64
	var maxVal float64
	hasNeg := false
	first := true

	for _, s := range data.Series {
		for _, v := range s.Values {
			if v < 0 {
				hasNeg = true
			}
			if v > 0 {
				if first || v < minPos {
					minPos = v
				}
				first = false
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}

	// Log scale needs all-positive data with a wide range
	if hasNeg || first || minPos <= 0 || maxVal <= 0 {
		return false
	}
	return maxVal/minPos >= 1000
}

// logDomainBounds returns the min/max positive values across all series,
// extended to the nearest power of 10 for clean axis boundaries.
func (bc *BarChart) logDomainBounds(data ChartData) (min, max float64) {
	min = math.Inf(1)
	max = 0

	for _, s := range data.Series {
		for _, v := range s.Values {
			if v > 0 {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
		}
	}

	// Extend to clean power-of-10 boundaries
	min = math.Pow(10, math.Floor(math.Log10(min)))
	max = math.Pow(10, math.Ceil(math.Log10(max)))

	return min, max
}

// drawLogGrid draws grid lines at powers of 10 using the log scale.
func (bc *BarChart) drawLogGrid(plotArea Rect) {
	b := bc.builder

	gridConfig := DefaultGridConfig()
	gridConfig.ShowHorizontal = true
	gridConfig.ShowVertical = false

	b.Push()
	b.SetStrokeColor(gridConfig.Color)
	b.SetStrokeWidth(gridConfig.StrokeWidth)
	b.SetDashes(gridConfig.DashPattern...)

	ticks := bc.logScale.Ticks(5)
	for _, v := range ticks {
		y := plotArea.Y + bc.logScale.Scale(v)
		b.DrawLine(plotArea.X, y, plotArea.X+plotArea.W, y)
	}

	b.Pop()
}

// drawLogAxes draws x and y axes where the y-axis uses log scale labels.
func (bc *BarChart) drawLogAxes(plotArea Rect, xScale *CategoricalScale, axisFontSize, xLabelRotation float64, labelStep int) {
	b := bc.builder

	// X axis (same as linear)
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = bc.config.XAxisTitle
	xAxisConfig.FontSize = axisFontSize
	xAxisConfig.LabelRotation = xLabelRotation

	if labelStep > 1 {
		xAxisConfig.HideLabels = true
	}

	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawCategoricalAxis(xScale, plotArea.X, plotArea.Y+plotArea.H)

	// If thinning, draw every Nth label manually (same as linear path)
	if labelStep > 1 {
		style := b.StyleGuide()
		cats := xScale.Categories()
		b.Push()
		b.SetFontSize(axisFontSize)
		b.SetFontWeight(style.Typography.WeightNormal)
		lastIdx := len(cats) - 1
		for i, cat := range cats {
			if i%labelStep != 0 && i != lastIdx {
				continue
			}
			labelX := plotArea.X + xScale.Scale(cat)
			labelY := plotArea.Y + plotArea.H + xAxisConfig.TickSize + xAxisConfig.TickPadding

			if xLabelRotation != 0 {
				labelY += axisFontSize // offset to avoid overlapping axis line
				b.Push()
				b.RotateAround(xLabelRotation, labelX, labelY)
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

	// Y axis — use log scale labels
	yAxisConfig := DefaultAxisConfig(AxisPositionLeft)
	yAxisConfig.Title = bc.config.YAxisTitle
	yAxis := NewAxis(b, yAxisConfig)
	yAxis.DrawLogAxis(bc.logScale, plotArea.X, plotArea.Y)
}

// drawAxes draws the chart axes.
func (bc *BarChart) drawAxes(plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, axisFontSize, xLabelRotation float64, labelStep int) {
	b := bc.builder

	// X axis — with density-adaptive font size and rotation
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = bc.config.XAxisTitle
	xAxisConfig.FontSize = axisFontSize
	xAxisConfig.LabelRotation = xLabelRotation

	if labelStep > 1 {
		// Label thinning: hide labels on the axis, we draw them manually below
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
			if i%labelStep != 0 && i != lastIdx {
				continue
			}
			labelX := plotArea.X + xScale.Scale(cat)
			labelY := plotArea.Y + plotArea.H + xAxisConfig.TickSize + xAxisConfig.TickPadding

			if xLabelRotation != 0 {
				labelY += axisFontSize // offset to avoid overlapping axis line
				b.Push()
				b.RotateAround(xLabelRotation, labelX, labelY)
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
	DrawCartesianYAxis(b, plotArea, yScale, bc.config.YAxisTitle)
}

// drawBars draws the bar series.
// When bc.logScale is non-nil, yScale may be nil and bars are positioned
// using log10-transformed data on a linear scale in log space.
func (bc *BarChart) drawBars(data ChartData, plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, colors []Color) {
	b := bc.builder

	numSeries := len(data.Series)
	bandwidth := xScale.Bandwidth()
	barWidth := bandwidth * (1 - bc.config.BarPadding) / float64(numSeries)

	// Build the adjusted y scale.  In log mode we create a LinearScale
	// whose domain is the log10 boundaries so that BarSeries (which only
	// knows about LinearScale) positions bars in log space.
	var adjustedYScale *LinearScale
	var baseY float64

	if bc.logScale != nil {
		dMin, dMax := bc.logScale.DomainBounds()
		logMin := math.Log10(dMin)
		logMax := math.Log10(dMax)
		adjustedYScale = NewLinearScale(logMin, logMax)
		adjustedYScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)
		// Baseline is the bottom of the plot (smallest power of 10)
		baseY = plotArea.Y + plotArea.H
	} else {
		adjustedYScale = NewLinearScale(yScale.domainMin, yScale.domainMax)
		adjustedYScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)
		baseY = plotArea.Y + yScale.Scale(0)
	}

	for seriesIdx, series := range data.Series {
		barConfig := DefaultBarSeriesConfig()
		barConfig.Color = colors[seriesIdx%len(colors)]
		barConfig.CornerRadius = bc.config.CornerRadius
		barConfig.ShowValues = bc.config.ShowValues
		barConfig.ValueFormat = bc.config.ValueFormat
		barConfig.SeriesIndex = seriesIdx
		barConfig.SeriesCount = numSeries

		if series.Color != nil {
			barConfig.Color = *series.Color
		}

		bs := NewBarSeries(b, barConfig)

		points := make([]DataPoint, len(series.Values))
		negOnLogCount := 0
		for i, v := range series.Values {
			yVal := v
			if bc.logScale != nil && v > 0 {
				yVal = math.Log10(v)
			} else if bc.logScale != nil {
				// Zero/negative values: place at baseline
				dMin, _ := bc.logScale.DomainBounds()
				yVal = math.Log10(dMin)
				negOnLogCount++
			}
			points[i] = DataPoint{
				XCategory: data.Categories[i],
				Y:         yVal,
				Value:     v, // original value for labels
			}
		}
		if negOnLogCount > 0 {
			b.AddFinding(Finding{
				Field:    fmt.Sprintf("data.series[%d].values", seriesIdx),
				Code:     FindingNegativeOnLog,
				Message:  fmt.Sprintf("%d value(s) are zero or negative on log scale — placed at baseline", negOnLogCount),
				Severity: "warning",
				Fix: &FixSuggestion{
					Kind:   FixKindExplicitScale,
					Params: map[string]any{"clamped_count": negOnLogCount, "series_index": seriesIdx},
				},
			})
		}

		// Adjust x scale for bar positions within plot area
		adjustedXScale := NewCategoricalScale(data.Categories)
		adjustedXScale.SetRangeCategorical(plotArea.X, plotArea.X+plotArea.W)
		adjustedXScale.PaddingOuter(bc.config.GroupPadding)
		adjustedXScale.PaddingInner(bc.config.GroupPadding)

		bs.DrawCategorical(points, adjustedXScale, adjustedYScale, baseY)
		_ = barWidth
	}
}

// drawStackedBars draws stacked bar segments where each series is stacked on top
// of the previous one. Each category gets a single full-width bar composed of
// colored segments, one per series.
func (bc *BarChart) drawStackedBars(data ChartData, plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, colors []Color) {
	b := bc.builder
	style := b.StyleGuide()

	// Create adjusted scales for the plot area
	adjustedXScale := NewCategoricalScale(data.Categories)
	adjustedXScale.SetRangeCategorical(plotArea.X, plotArea.X+plotArea.W)
	adjustedXScale.PaddingOuter(bc.config.GroupPadding)
	adjustedXScale.PaddingInner(bc.config.GroupPadding)

	adjustedYScale := NewLinearScale(yScale.domainMin, yScale.domainMax)
	adjustedYScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)

	bandwidth := adjustedXScale.Bandwidth()
	barWidth := bandwidth * (1 - bc.config.BarPadding)

	baseY := plotArea.Y + adjustedYScale.Scale(0)

	// Track cumulative values per category for stacking
	numCategories := len(data.Categories)
	cumulative := make([]float64, numCategories)

	b.Push()

	for seriesIdx, series := range data.Series {
		color := colors[seriesIdx%len(colors)]
		if series.Color != nil {
			color = *series.Color
		}

		for catIdx := 0; catIdx < numCategories && catIdx < len(series.Values); catIdx++ {
			v := series.Values[catIdx]
			if v == 0 {
				continue
			}

			cat := data.Categories[catIdx]
			x := adjustedXScale.Scale(cat)

			// Bottom of this segment = cumulative so far
			segBottom := cumulative[catIdx]
			// Top of this segment = cumulative + current value
			segTop := segBottom + v

			// Convert to pixel positions
			yBottom := adjustedYScale.Scale(segBottom)
			yTop := adjustedYScale.Scale(segTop)

			rectX := x - barWidth/2
			rectY := yTop
			rectH := yBottom - yTop

			if rectH < 0 {
				rectY = yBottom
				rectH = -rectH
			}

			b.SetFillColor(color)
			if bc.config.CornerRadius > 0 {
				b.DrawRoundedRect(Rect{X: rectX, Y: rectY, W: barWidth, H: rectH}, bc.config.CornerRadius)
			} else {
				b.FillRect(Rect{X: rectX, Y: rectY, W: barWidth, H: rectH})
			}

			cumulative[catIdx] = segTop
		}
	}

	// Draw per-segment value labels inside each stacked bar segment.
	// Uses contrast-aware text color so labels are readable on any fill.
	{
		labelFontSize := math.Max(7, math.Min(8, style.Typography.SizeCaption))
		b.SetFontSize(labelFontSize)
		b.SetFontWeight(style.Typography.WeightNormal)

		cumulativeForLabels := make([]float64, numCategories)

		for seriesIdx, series := range data.Series {
			segColor := colors[seriesIdx%len(colors)]
			if series.Color != nil {
				segColor = *series.Color
			}

			for catIdx := 0; catIdx < numCategories && catIdx < len(series.Values); catIdx++ {
				v := series.Values[catIdx]
				if v == 0 {
					continue
				}

				cat := data.Categories[catIdx]
				x := adjustedXScale.Scale(cat)

				segBottom := cumulativeForLabels[catIdx]
				segTop := segBottom + v

				yBottom := adjustedYScale.Scale(segBottom)
				yTop := adjustedYScale.Scale(segTop)

				// Center the label in the segment
				labelY := (yTop + yBottom) / 2
				segHeight := math.Abs(yBottom - yTop)

				// Only show label if segment is tall enough (15px minimum)
				if segHeight > 15 {
					// Format: integers if >= 10, one decimal if < 10
					var label string
					if math.Abs(v) >= 10 {
						label = fmt.Sprintf("%.0f", v)
					} else {
						label = fmt.Sprintf("%.1f", v)
					}

					// Use contrast-aware text color for readability
					b.SetTextColor(segColor.TextColorFor())
					b.DrawText(label, x, labelY, TextAlignCenter, TextBaselineMiddle)
				}

				cumulativeForLabels[catIdx] = segTop
			}
		}
	}

	b.Pop()
	_ = baseY
}

// getColors returns colors for the series.
func (bc *BarChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(bc.config.Colors, style, count)
}

// =============================================================================
// Line Chart
// =============================================================================

// LineChartConfig holds configuration specific to line charts.
type LineChartConfig struct {
	ChartConfig

	// Smooth enables curved line interpolation.
	Smooth bool

	// Tension controls the smoothness (0-1).
	Tension float64

	// ShowMarkers enables markers at data points.
	ShowMarkers bool

	// MarkerSize is the marker size in points.
	MarkerSize float64

	// StrokeWidth is the line stroke width.
	StrokeWidth float64

	// FillArea fills the area under the line.
	FillArea bool

	// FillOpacity is the fill opacity (0-1).
	FillOpacity float64
}

// DefaultLineChartConfig returns default line chart configuration.
func DefaultLineChartConfig(width, height float64) LineChartConfig {
	return LineChartConfig{
		ChartConfig: DefaultChartConfig(width, height),
		Smooth:      false,
		Tension:     0.5,
		ShowMarkers: true,
		MarkerSize:  8,
		StrokeWidth: 3,
		FillArea:    false,
		FillOpacity: 0.2,
	}
}

// LineChart renders line charts.
type LineChart struct {
	builder *SVGBuilder
	config  LineChartConfig
}

// NewLineChart creates a new line chart renderer.
func NewLineChart(builder *SVGBuilder, config LineChartConfig) *LineChart {
	return &LineChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the line chart.
func (lc *LineChart) Draw(data ChartData) error {
	if len(data.Series) == 0 {
		return fmt.Errorf("line chart requires at least one series")
	}

	b := lc.builder
	style := b.StyleGuide()
	colors := lc.getColors(style, len(data.Series))

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(lc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if lc.config.ShowLegend {
		RefineLegendHeight(b, style, data.Series, &plotArea, &legendHeight)
	}

	// Calculate domains
	yMin, yMax := lc.calculateYDomain(data)

	// Check if data is time-series
	isTimeSeries := lc.hasTimeSeriesData(data)

	// Adaptive x-axis label layout for categorical data
	isNarrow := lc.config.Width < 500
	var xLayout XLabelLayout
	categories := data.Categories

	if len(data.Categories) > 0 && !isTimeSeries {
		prelimPlotW := plotArea.W
		xLayout = AdaptXLabels(b, data.Categories, prelimPlotW, style.Typography.SizeSmall, isNarrow)
		categories = xLayout.Categories
		if xLayout.ExtraBottomMargin > 0 {
			plotArea.H -= xLayout.ExtraBottomMargin
		}
	}

	// Create scales
	var xScale Scale
	var timeScale *TimeScale
	if len(categories) > 0 && !isTimeSeries {
		cs := NewCategoricalScale(categories)
		cs.SetRangeCategorical(0, plotArea.W)
		xScale = cs
	} else if isTimeSeries {
		tMin, tMax := lc.calculateTimeDomain(data)
		timeScale = NewTimeScale(tMin, tMax)
		timeScale.SetRangeTime(0, plotArea.W)
		xScale = timeScale
	} else {
		xMin, xMax := lc.calculateXDomain(data)
		ls := NewLinearScale(xMin, xMax)
		ls.SetRangeLinear(0, plotArea.W)
		xScale = ls
	}

	yScale := NewLinearScale(yMin, yMax)
	yScale.SetRangeLinear(plotArea.H, 0)
	yScale.Nice(true)

	// Draw grid
	if lc.config.ShowGrid {
		DrawCartesianGrid(b, plotArea, yScale, nil)
	}

	// Draw axes
	if lc.config.ShowAxes {
		lc.drawAxes(plotArea, xScale, yScale, categories, xLayout)
	}

	// Draw lines — use adapted categories (potentially truncated) so
	// DataPoint.XCategory matches the CategoricalScale keys.
	drawData := data
	if len(categories) > 0 && !isTimeSeries {
		drawData.Categories = categories
	}
	lc.drawLines(drawData, plotArea, xScale, yScale, colors)

	// Draw title
	if lc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: lc.config.Width, H: headerHeight + lc.config.MarginTop})
	}

	// Draw legend
	if lc.config.ShowLegend && len(data.Series) > 1 {
		legendConfig := PresentationLegendConfig(style)
		legendConfig.MarkerShape = LegendMarkerLine

		legend := NewLegend(b, legendConfig)

		items := make([]LegendItem, len(data.Series))
		for i, series := range data.Series {
			items[i] = LegendItem{
				Label: series.Name,
				Color: colors[i%len(colors)],
			}
			if series.Color != nil {
				items[i].Color = *series.Color
			}
		}
		legend.SetItems(items)

		// Calculate x-axis label space: tick size + tick padding + label height + extra gap
		xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
		xAxisLabelSpace := xAxisConfig.TickSize + xAxisConfig.TickPadding + style.Typography.SizeSmall + style.Spacing.SM

		legendBounds := Rect{
			X: plotArea.X,
			Y: plotArea.Y + plotArea.H + xAxisLabelSpace,
			W: plotArea.W,
			H: legendHeight,
		}
		legend.Draw(legendBounds)
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: lc.config.Height - layout.FooterHeight,
			W: lc.config.Width,
			H: layout.FooterHeight,
		})
	}

	return nil
}

// calculateXDomain calculates the x-axis domain for linear scales.
func (lc *LineChart) calculateXDomain(data ChartData) (min, max float64) {
	if len(data.Series) == 0 || len(data.Series[0].XValues) == 0 {
		return 0, 1
	}

	min = data.Series[0].XValues[0]
	max = data.Series[0].XValues[0]

	for _, series := range data.Series {
		for _, v := range series.XValues {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}

	return min, max
}

// calculateYDomain calculates the y-axis domain.
// Returns tight bounds with minimal padding; the caller applies Nice() on the
// scale which rounds to clean tick-aligned boundaries, providing natural headroom.
func (lc *LineChart) calculateYDomain(data ChartData) (min, max float64) {
	if len(data.Series) == 0 || len(data.Series[0].Values) == 0 {
		return 0, 1
	}

	min = data.Series[0].Values[0]
	max = data.Series[0].Values[0]

	for _, series := range data.Series {
		for _, v := range series.Values {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}

	// Handle degenerate case where all values are identical.
	if min == max {
		if min == 0 {
			return 0, 1
		}
		// Provide a small range around the single value.
		offset := math.Abs(min) * 0.1
		if offset == 0 {
			offset = 1
		}
		return min - offset, max + offset
	}

	// Add small top padding (~5%) so the highest data point doesn't touch
	// the axis boundary. Nice() will round this to a clean tick value.
	span := max - min
	max += span * 0.05

	// For line/area charts, allow the min to remain above zero when the
	// data range is far from zero (e.g., 100-200 should not show 0-220).
	// Nice() will round to a nearby clean number.
	// If data is close to zero (min < 20% of range), snap to zero for clarity.
	if min > 0 && min < span*0.2 {
		min = 0
	}

	return min, max
}

// hasTimeSeriesData checks if any series contains time-series data.
func (lc *LineChart) hasTimeSeriesData(data ChartData) bool {
	for _, series := range data.Series {
		if series.HasTimeData() {
			return true
		}
	}
	return false
}

// calculateTimeDomain calculates the time domain from all series.
func (lc *LineChart) calculateTimeDomain(data ChartData) (min, max int64) {
	initialized := false

	for seriesIdx, series := range data.Series {
		timeValues, err := series.GetTimeValues()
		if err != nil {
			lc.builder.AddFinding(Finding{
				Field:    fmt.Sprintf("data.series[%d].time_strings", seriesIdx),
				Code:     FindingInvalidTimeFormat,
				Message:  fmt.Sprintf("time series %d has unparseable time values: %v", seriesIdx, err),
				Severity: "warning",
				Fix: &FixSuggestion{
					Kind:   FixKindReplaceValue,
					Params: map[string]any{"series_index": seriesIdx, "error": err.Error()},
				},
			})
			continue
		}
		if len(timeValues) == 0 {
			continue
		}

		for _, ts := range timeValues {
			if !initialized {
				min = ts
				max = ts
				initialized = true
			} else {
				if ts < min {
					min = ts
				}
				if ts > max {
					max = ts
				}
			}
		}
	}

	if !initialized {
		return 0, 86400 // Default to 1 day span
	}

	return min, max
}

// drawAxes draws the chart axes.
func (lc *LineChart) drawAxes(plotArea Rect, xScale Scale, yScale *LinearScale, categories []string, xLayout XLabelLayout) {
	b := lc.builder
	style := b.StyleGuide()

	// X axis
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = lc.config.XAxisTitle

	switch xs := xScale.(type) {
	case *CategoricalScale:
		// Apply adaptive font size and rotation from AdaptXLabels
		xAxisConfig.FontSize = xLayout.FontSize
		xAxisConfig.LabelRotation = xLayout.Rotation

		if xLayout.LabelStep > 1 {
			// Label thinning: hide labels on the axis, draw them manually
			xAxisConfig.HideLabels = true
		}

		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawCategoricalAxis(xs, plotArea.X, plotArea.Y+plotArea.H)

		// If thinning, draw every Nth label manually (always show first and last)
		if xLayout.LabelStep > 1 {
			cats := xs.Categories()
			b.Push()
			b.SetFontSize(xLayout.FontSize)
			b.SetFontWeight(style.Typography.WeightNormal)
			lastIdx := len(cats) - 1
			for i, cat := range cats {
				if i%xLayout.LabelStep != 0 && i != lastIdx {
					continue
				}
				labelX := plotArea.X + xs.Scale(cat)
				labelY := plotArea.Y + plotArea.H + xAxisConfig.TickSize + xAxisConfig.TickPadding

				if xLayout.Rotation != 0 {
					labelY += xLayout.FontSize * 0.5 // offset to avoid overlapping axis line
					b.Push()
					b.RotateAround(xLayout.Rotation, labelX, labelY)
					b.DrawText(cat, labelX, labelY, TextAlignLeft, TextBaselineTop)
					b.Pop()
				} else {
					b.DrawText(cat, labelX, labelY, TextAlignCenter, TextBaselineTop)
				}
			}
			b.Pop()
		}
	case *LinearScale:
		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawLinearAxis(xs, plotArea.X, plotArea.Y+plotArea.H)
	case *TimeScale:
		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawTimeAxis(xs, plotArea.X, plotArea.Y+plotArea.H)
	}

	// Y axis (shared)
	DrawCartesianYAxis(b, plotArea, yScale, lc.config.YAxisTitle)
}

// drawLines draws the line series.
func (lc *LineChart) drawLines(data ChartData, plotArea Rect, xScale Scale, yScale *LinearScale, colors []Color) {
	b := lc.builder

	baseY := plotArea.Y + plotArea.H

	for seriesIdx, series := range data.Series {
		lineConfig := DefaultLineSeriesConfig()
		lineConfig.Color = colors[seriesIdx%len(colors)]
		lineConfig.StrokeWidth = lc.config.StrokeWidth
		lineConfig.ShowMarkers = lc.config.ShowMarkers
		lineConfig.MarkerSize = lc.config.MarkerSize
		lineConfig.Smooth = lc.config.Smooth
		lineConfig.Tension = lc.config.Tension
		lineConfig.FillArea = lc.config.FillArea
		lineConfig.FillColor = colors[seriesIdx%len(colors)]
		lineConfig.FillOpacity = lc.config.FillOpacity
		lineConfig.ShowValues = lc.config.ShowValues
		lineConfig.ValueFormat = lc.config.ValueFormat

		if series.Color != nil {
			lineConfig.Color = *series.Color
			lineConfig.FillColor = *series.Color
			lineConfig.MarkerFillColor = *series.Color
		}

		ls := NewLineSeries(b, lineConfig)

		adjustedYScale := NewLinearScale(yScale.domainMin, yScale.domainMax)
		adjustedYScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)

		// Create adjusted scales for plot area based on scale type
		switch xs := xScale.(type) {
		case *CategoricalScale:
			points := make([]DataPoint, len(series.Values))
			for i, v := range series.Values {
				cat := data.Categories[i%len(data.Categories)]
				points[i] = DataPoint{
					XCategory: cat,
					Y:         v,
				}
			}

			adjustedXScale := NewCategoricalScale(data.Categories)
			adjustedXScale.SetRangeCategorical(plotArea.X, plotArea.X+plotArea.W)

			ls.DrawCategorical(points, adjustedXScale, adjustedYScale, baseY)
			_ = xs

		case *TimeScale:
			// Handle time-series data
			timeValues, err := series.GetTimeValues()
			if err != nil {
				b.AddFinding(Finding{
					Field:    fmt.Sprintf("data.series[%d].time_strings", seriesIdx),
					Code:     FindingInvalidTimeFormat,
					Message:  fmt.Sprintf("time series %d skipped — unparseable time values: %v", seriesIdx, err),
					Severity: "warning",
					Fix: &FixSuggestion{
						Kind:   FixKindReplaceValue,
						Params: map[string]any{"series_index": seriesIdx, "error": err.Error()},
					},
				})
				continue
			}
			if len(timeValues) == 0 {
				continue
			}

			points := make([]DataPoint, len(series.Values))
			for i, v := range series.Values {
				ts := int64(0)
				if i < len(timeValues) {
					ts = timeValues[i]
				}
				points[i] = DataPoint{
					X: float64(ts), // Store timestamp as X for linear drawing
					Y: v,
				}
			}

			tMin, tMax := lc.calculateTimeDomain(data)
			// Convert time scale to linear for drawing (X is now timestamp)
			adjustedXScale := NewLinearScale(float64(tMin), float64(tMax))
			adjustedXScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)

			ls.DrawLinear(points, adjustedXScale, adjustedYScale, baseY)

		case *LinearScale:
			points := make([]DataPoint, len(series.Values))
			for i, v := range series.Values {
				x := 0.0
				if i < len(series.XValues) {
					x = series.XValues[i]
				} else {
					x = float64(i)
				}
				points[i] = DataPoint{
					X: x,
					Y: v,
				}
			}

			xMin, xMax := lc.calculateXDomain(data)
			adjustedXScale := NewLinearScale(xMin, xMax)
			adjustedXScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)

			ls.DrawLinear(points, adjustedXScale, adjustedYScale, baseY)
		}
	}
}

// getColors returns colors for the series.
func (lc *LineChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(lc.config.Colors, style, count)
}

// =============================================================================
// Area Chart (extends Line Chart)
// =============================================================================

// AreaChartConfig holds configuration for area charts.
type AreaChartConfig struct {
	LineChartConfig

	// Stacked stacks areas on top of each other.
	Stacked bool
}

// DefaultAreaChartConfig returns default area chart configuration.
func DefaultAreaChartConfig(width, height float64) AreaChartConfig {
	lineConfig := DefaultLineChartConfig(width, height)
	lineConfig.FillArea = true
	lineConfig.FillOpacity = 0.5
	lineConfig.ShowMarkers = false

	return AreaChartConfig{
		LineChartConfig: lineConfig,
		Stacked:         false,
	}
}

// AreaChart renders area charts.
type AreaChart struct {
	builder *SVGBuilder
	config  AreaChartConfig
}

// NewAreaChart creates a new area chart renderer.
func NewAreaChart(builder *SVGBuilder, config AreaChartConfig) *AreaChart {
	return &AreaChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the area chart.
func (ac *AreaChart) Draw(data ChartData) error {
	// Area chart is essentially a line chart with fill enabled
	lineChart := NewLineChart(ac.builder, ac.config.LineChartConfig)
	return lineChart.Draw(data)
}

// StackedAreaChart renders stacked area charts where series are cumulated.
type StackedAreaChart struct {
	builder *SVGBuilder
	config  AreaChartConfig
}

// NewStackedAreaChart creates a new stacked area chart renderer.
func NewStackedAreaChart(builder *SVGBuilder, config AreaChartConfig) *StackedAreaChart {
	return &StackedAreaChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the stacked area chart by accumulating series values.
func (sac *StackedAreaChart) Draw(data ChartData) error {
	if len(data.Series) == 0 || len(data.Categories) == 0 {
		return fmt.Errorf("stacked area chart requires series and categories")
	}

	numCategories := len(data.Categories)

	// Build cumulative series: each series stacks on top of the previous one.
	// Render in reverse order so the first series appears on top visually.
	stackedData := ChartData{
		Title:      data.Title,
		Subtitle:   data.Subtitle,
		Categories: data.Categories,
		Footnote:   data.Footnote,
		Series:     make([]ChartSeries, len(data.Series)),
	}

	// Cumulative sums per category
	cumulative := make([]float64, numCategories)

	for i := range data.Series {
		vals := make([]float64, numCategories)
		for j := 0; j < numCategories; j++ {
			v := 0.0
			if j < len(data.Series[i].Values) {
				v = data.Series[i].Values[j]
			}
			cumulative[j] += v
			vals[j] = cumulative[j]
		}
		stackedData.Series[i] = ChartSeries{
			Name:   data.Series[i].Name,
			Values: vals,
			Color:  data.Series[i].Color,
			Labels: data.Series[i].Labels,
		}
	}

	// Reverse the series order so first series renders last (on top)
	for i, j := 0, len(stackedData.Series)-1; i < j; i, j = i+1, j-1 {
		stackedData.Series[i], stackedData.Series[j] = stackedData.Series[j], stackedData.Series[i]
	}

	lineChart := NewLineChart(sac.builder, sac.config.LineChartConfig)
	return lineChart.Draw(stackedData)
}

// =============================================================================
// Scatter Chart
// =============================================================================

// ScatterChartConfig holds configuration for scatter charts.
type ScatterChartConfig struct {
	ChartConfig

	// PointSize is the default point size.
	PointSize float64

	// PointShape is the marker shape.
	PointShape MarkerShape

	// ShowLabels enables labels on points.
	ShowLabels bool

	// VariableSize enables bubble chart mode.
	VariableSize bool

	// SizeRange is the min/max size for bubble mode.
	SizeRange [2]float64
}

// DefaultScatterChartConfig returns default scatter chart configuration.
func DefaultScatterChartConfig(width, height float64) ScatterChartConfig {
	return ScatterChartConfig{
		ChartConfig:  DefaultChartConfig(width, height),
		PointSize:    8,
		PointShape:   MarkerCircle,
		ShowLabels:   false,
		VariableSize: false,
		SizeRange:    [2]float64{4, 20},
	}
}

// ScatterChart renders scatter/bubble charts.
type ScatterChart struct {
	builder *SVGBuilder
	config  ScatterChartConfig
}

// NewScatterChart creates a new scatter chart renderer.
func NewScatterChart(builder *SVGBuilder, config ScatterChartConfig) *ScatterChart {
	return &ScatterChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the scatter chart.
func (sc *ScatterChart) Draw(data ChartData) error {
	if len(data.Series) == 0 {
		return fmt.Errorf("scatter chart requires at least one series")
	}

	b := sc.builder
	style := b.StyleGuide()
	colors := sc.getColors(style, len(data.Series))

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(sc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if sc.config.ShowLegend {
		RefineLegendHeight(b, style, data.Series, &plotArea, &legendHeight)
	}

	// Calculate domains
	xMin, xMax := sc.calculateXDomain(data)
	yMin, yMax := sc.calculateYDomain(data)

	// Create scales
	xScale := NewLinearScale(xMin, xMax)
	xScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)
	xScale.Nice(true)

	yScale := NewLinearScale(yMin, yMax)
	yScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)
	yScale.Nice(true)

	// Draw grid
	if sc.config.ShowGrid {
		sc.drawGrid(plotArea, xScale, yScale)
	}

	// Draw axes
	if sc.config.ShowAxes {
		sc.drawAxes(plotArea, xScale, yScale)
	}

	// Draw points
	sc.drawPoints(data, xScale, yScale, colors)

	// Draw title
	if sc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: sc.config.Width, H: headerHeight + sc.config.MarginTop})
	}

	// Draw legend
	if sc.config.ShowLegend && len(data.Series) > 1 {
		legendConfig := PresentationLegendConfig(style)
		legendConfig.MarkerShape = LegendMarkerCircle

		legend := NewLegend(b, legendConfig)

		items := make([]LegendItem, len(data.Series))
		for i, series := range data.Series {
			items[i] = LegendItem{
				Label: series.Name,
				Color: colors[i%len(colors)],
			}
		}
		legend.SetItems(items)

		// Calculate x-axis label space: tick size + tick padding + label height + extra gap
		xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
		xAxisLabelSpace := xAxisConfig.TickSize + xAxisConfig.TickPadding + style.Typography.SizeSmall + style.Spacing.SM

		legendBounds := Rect{
			X: plotArea.X,
			Y: plotArea.Y + plotArea.H + xAxisLabelSpace,
			W: plotArea.W,
			H: legendHeight,
		}
		legend.Draw(legendBounds)
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: sc.config.Height - layout.FooterHeight,
			W: sc.config.Width,
			H: layout.FooterHeight,
		})
	}

	return nil
}

// calculateXDomain calculates the x-axis domain.
// Returns tight bounds; Nice() on the scale provides clean tick-aligned rounding.
func (sc *ScatterChart) calculateXDomain(data ChartData) (min, max float64) {
	if len(data.Series) == 0 || len(data.Series[0].XValues) == 0 {
		return 0, 1
	}

	min = data.Series[0].XValues[0]
	max = data.Series[0].XValues[0]

	for _, series := range data.Series {
		for _, v := range series.XValues {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}

	// Handle degenerate case.
	if min == max {
		if min == 0 {
			return 0, 1
		}
		offset := math.Abs(min) * 0.1
		if offset == 0 {
			offset = 1
		}
		return min - offset, max + offset
	}

	// Add small padding (~5%) so extreme points don't touch axis edges.
	span := max - min
	min -= span * 0.05
	max += span * 0.05

	return min, max
}

// calculateYDomain calculates the y-axis domain.
// Returns tight bounds; Nice() on the scale provides clean tick-aligned rounding.
func (sc *ScatterChart) calculateYDomain(data ChartData) (min, max float64) {
	if len(data.Series) == 0 || len(data.Series[0].Values) == 0 {
		return 0, 1
	}

	min = data.Series[0].Values[0]
	max = data.Series[0].Values[0]

	for _, series := range data.Series {
		for _, v := range series.Values {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}

	// Handle degenerate case.
	if min == max {
		if min == 0 {
			return 0, 1
		}
		offset := math.Abs(min) * 0.1
		if offset == 0 {
			offset = 1
		}
		return min - offset, max + offset
	}

	// Add small padding (~5%) so extreme points don't touch axis edges.
	span := max - min
	min -= span * 0.05
	max += span * 0.05

	return min, max
}

// drawGrid draws the chart grid.
func (sc *ScatterChart) drawGrid(plotArea Rect, xScale, yScale *LinearScale) {
	b := sc.builder

	gridConfig := DefaultGridConfig()
	gridConfig.ShowVertical = true

	b.Push()
	b.SetStrokeColor(gridConfig.Color)
	b.SetStrokeWidth(gridConfig.StrokeWidth)
	b.SetDashes(gridConfig.DashPattern...)

	// Horizontal grid lines
	yTicks := yScale.Ticks(5)
	for _, v := range yTicks {
		y := yScale.Scale(v)
		if y < plotArea.Y-0.5 || y > plotArea.Y+plotArea.H+0.5 {
			continue // skip out-of-bounds ticks
		}
		b.DrawLine(plotArea.X, y, plotArea.X+plotArea.W, y)
	}

	// Vertical grid lines
	xTicks := xScale.Ticks(5)
	for _, v := range xTicks {
		x := xScale.Scale(v)
		if x < plotArea.X-0.5 || x > plotArea.X+plotArea.W+0.5 {
			continue // skip out-of-bounds ticks
		}
		b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)
	}

	b.Pop()
}

// drawAxes draws the chart axes.
func (sc *ScatterChart) drawAxes(plotArea Rect, xScale, yScale *LinearScale) {
	b := sc.builder

	// The shared axis functions (DrawLinearAxis, DrawCartesianYAxis) add the
	// origin offset (plotArea.X/Y) to each tick position, so scales must
	// output relative positions within [0, W] / [0, H].  The caller's scales
	// use absolute ranges, so we create relative copies for axis drawing.
	relXScale := NewLinearScale(xScale.domainMin, xScale.domainMax)
	relXScale.SetRangeLinear(0, plotArea.W)

	relYScale := NewLinearScale(yScale.domainMin, yScale.domainMax)
	relYScale.SetRangeLinear(plotArea.H, 0)

	// X axis
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = sc.config.XAxisTitle
	xAxisConfig.RangeExtent = plotArea.W
	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawLinearAxis(relXScale, plotArea.X, plotArea.Y+plotArea.H)

	// Y axis (shared)
	DrawCartesianYAxis(b, plotArea, relYScale, sc.config.YAxisTitle)
}

// drawPoints draws the scatter points.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (sc *ScatterChart) drawPoints(data ChartData, xScale, yScale *LinearScale, colors []Color) {
	b := sc.builder
	style := b.StyleGuide()

	for seriesIdx, series := range data.Series {
		pointConfig := DefaultPointSeriesConfig()
		pointConfig.Color = colors[seriesIdx%len(colors)]
		pointConfig.Size = sc.config.PointSize
		pointConfig.Shape = sc.config.PointShape
		// Let PointSeries handle labels only when the config explicitly requests it.
		// We render labels ourselves below with scatter-specific positioning.
		pointConfig.ShowLabels = false

		if series.Color != nil {
			pointConfig.Color = *series.Color
		}

		// Handle bubble chart mode: variable-size points from BubbleValues
		if sc.config.VariableSize && len(series.BubbleValues) > 0 {
			bMin, bMax := series.BubbleValues[0], series.BubbleValues[0]
			for _, bv := range series.BubbleValues {
				if bv < bMin {
					bMin = bv
				}
				if bv > bMax {
					bMax = bv
				}
			}
			sizeScale := NewLinearScale(bMin, bMax)
			sizeScale.SetRangeLinear(sc.config.SizeRange[0], sc.config.SizeRange[1])
			pointConfig.SizeScale = sizeScale
		}

		ps := NewPointSeries(b, pointConfig)

		points := make([]DataPoint, len(series.Values))
		for i, v := range series.Values {
			x := 0.0
			if i < len(series.XValues) {
				x = series.XValues[i]
			}
			label := ""
			if i < len(series.Labels) {
				label = series.Labels[i]
			}
			// When ShowLabels is enabled and no explicit label is provided,
			// auto-generate a value label from the y-value so that data
			// points are always annotated (matching other chart types'
			// ShowValues behaviour).
			if label == "" && sc.config.ShowLabels {
				if math.Abs(v) >= 10 {
					label = fmt.Sprintf("%.0f", v)
				} else {
					label = fmt.Sprintf("%.1f", v)
				}
			}
			bubbleSize := 0.0
			if i < len(series.BubbleValues) {
				bubbleSize = series.BubbleValues[i]
			}
			points[i] = DataPoint{
				X:     x,
				Y:     v,
				Value: bubbleSize, // Value is used by getPointSize when SizeScale is set
				Label: label,
			}
		}

		ps.DrawLinear(points, xScale, yScale)

		// Render data point labels for each point that has one.
		// Labels are placed slightly above-right of the point for readability.
		hasLabels := false
		for _, pt := range points {
			if pt.Label != "" {
				hasLabels = true
				break
			}
		}
		if hasLabels {
			numLabeled := 0
			for _, pt := range points {
				if pt.Label != "" {
					numLabeled++
				}
			}

			// Adaptive font and truncation based on label density.
			// Floor at builder's minimum font size to ensure legibility.
			minFont := b.MinFontSize()
			labelFontSize := math.Max(minFont, math.Min(style.Typography.SizeSmall, style.Typography.SizeSmall))
			maxLabelChars := 20
			if numLabeled >= 15 {
				labelFontSize = minFont
				maxLabelChars = 15
			} else if numLabeled >= 10 {
				labelFontSize = math.Max(minFont, labelFontSize-0.5)
				maxLabelChars = 18
			}

			b.Push()
			b.SetFontSize(labelFontSize)
			b.SetFontWeight(style.Typography.WeightNormal)
			b.SetTextColor(style.Palette.TextPrimary)

			// Track placed label bounding boxes for collision detection
			type labelRect struct {
				x1, y1, x2, y2 float64
			}
			var placedLabels []labelRect

			pad := 2.0
			labelH := labelFontSize * 1.3
			pointOffset := sc.config.PointSize/2 + 3

			overlaps := func(r labelRect) bool {
				for _, placed := range placedLabels {
					if r.x1-pad < placed.x2+pad && r.x2+pad > placed.x1-pad &&
						r.y1-pad < placed.y2+pad && r.y2+pad > placed.y1-pad {
						return true
					}
				}
				return false
			}

			for _, pt := range points {
				if pt.Label == "" {
					continue
				}
				px := xScale.Scale(pt.X)
				py := yScale.Scale(pt.Y)

				lbl := pt.Label
				if len(lbl) > maxLabelChars {
					lbl = lbl[:maxLabelChars-1] + "\u2026"
				}

				labelW, _ := b.MeasureText(lbl)
				labelW *= 1.1 // safety margin

				// Try 4 positions: right, above, left, below
				type candidate struct {
					x, y  float64
					align TextAlign
					base  TextBaseline
					rect  labelRect
				}

				candidates := []candidate{
					{ // Right
						x: px + pointOffset, y: py - labelH/4,
						align: TextAlignLeft, base: TextBaselineBottom,
						rect: labelRect{px + pointOffset, py - labelH, px + pointOffset + labelW, py},
					},
					{ // Above
						x: px, y: py - pointOffset - 2,
						align: TextAlignCenter, base: TextBaselineBottom,
						rect: labelRect{px - labelW/2, py - pointOffset - labelH - 2, px + labelW/2, py - pointOffset - 2},
					},
					{ // Left
						x: px - pointOffset, y: py - labelH/4,
						align: TextAlignRight, base: TextBaselineBottom,
						rect: labelRect{px - pointOffset - labelW, py - labelH, px - pointOffset, py},
					},
					{ // Below
						x: px, y: py + pointOffset + labelH,
						align: TextAlignCenter, base: TextBaselineBottom,
						rect: labelRect{px - labelW/2, py + pointOffset, px + labelW/2, py + pointOffset + labelH},
					},
				}

				for _, c := range candidates {
					if !overlaps(c.rect) {
						b.DrawText(lbl, c.x, c.y, c.align, c.base)
						placedLabels = append(placedLabels, c.rect)
						break
					}
				}
			}
			b.Pop()
		}
	}
}

// getColors returns colors for the series.
func (sc *ScatterChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(sc.config.Colors, style, count)
}

// =============================================================================
// Pie/Donut Chart
// =============================================================================

// PieChartConfig holds configuration for pie/donut charts.
type PieChartConfig struct {
	ChartConfig

	// InnerRadius creates a donut chart (0 = pie chart).
	InnerRadius float64

	// StartAngle is the starting angle in degrees.
	StartAngle float64

	// PadAngle is padding between slices in degrees.
	PadAngle float64

	// ShowLabels enables slice labels.
	ShowLabels bool

	// LabelPosition determines label placement.
	LabelPosition ArcLabelPosition

	// LabelFormat is the format string for labels.
	LabelFormat string

	// ExplodeOffset is the offset for exploded slices.
	ExplodeOffset float64

	// ExplodedSlices is a list of slice indices to explode.
	ExplodedSlices []int
}

// DefaultPieChartConfig returns default pie chart configuration.
// Pie charts use smaller, uniform margins because they have no axes.
func DefaultPieChartConfig(width, height float64) PieChartConfig {
	config := DefaultChartConfig(width, height)
	// Override axis-heavy margins with smaller uniform margins for pie/donut.
	// Pie charts need only minimal padding around the circular plot area.
	margin := math.Min(config.MarginTop, config.MarginRight)
	config.MarginTop = margin
	config.MarginRight = margin
	config.MarginBottom = margin
	config.MarginLeft = margin
	return PieChartConfig{
		ChartConfig:   config,
		InnerRadius:   0,
		StartAngle:    -90,
		PadAngle:      1,
		ShowLabels:    true,
		LabelPosition: ArcLabelOutside,
		LabelFormat:   "%.0f%%",
		ExplodeOffset: 10,
	}
}

// DefaultDonutChartConfig returns default donut chart configuration.
func DefaultDonutChartConfig(width, height float64) PieChartConfig {
	config := DefaultPieChartConfig(width, height)
	// Set inner radius to create donut effect
	// Will be calculated based on actual radius when drawn
	config.InnerRadius = -1 // -1 means "auto" (50% of outer radius)
	return config
}

// PieChart renders pie and donut charts.
type PieChart struct {
	builder *SVGBuilder
	config  PieChartConfig
}

// NewPieChart creates a new pie chart renderer.
func NewPieChart(builder *SVGBuilder, config PieChartConfig) *PieChart {
	return &PieChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the pie chart.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (pc *PieChart) Draw(data ChartData) error {
	if len(data.Series) == 0 || len(data.Series[0].Values) == 0 {
		return fmt.Errorf("pie chart requires values")
	}

	b := pc.builder
	style := b.StyleGuide()

	// Get values and labels from first series
	values := data.Series[0].Values
	labels := data.Categories
	if len(labels) == 0 && len(data.Series[0].Labels) > 0 {
		labels = data.Series[0].Labels
	}

	// Detect zero-sum condition: all values zero or all negative.
	total := 0.0
	for _, v := range values {
		total += v
	}
	if total <= 0 {
		b.AddFinding(Finding{
			Field:    "data.series[0].values",
			Code:     FindingZeroSumPie,
			Message:  "pie chart has zero or negative total — chart will render blank",
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReplaceValue,
				Params: map[string]any{"total": total, "count": len(values)},
			},
		})
	}

	colors := pc.getColors(style, len(values))

	// Calculate layout
	plotArea := pc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if pc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Adjust for footnote
	footerHeight := 0.0
	if data.Footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight

	// Determine legend placement: landscape layouts use a right-side legend
	// to let the pie use the full vertical space, while portrait/square
	// layouts keep the traditional bottom legend.
	landscapeLegend := pc.config.ShowLegend && plotArea.W > plotArea.H*1.1

	// Variables for pie area and legend area
	var pieArea Rect
	var legendBounds Rect
	var legendHeight float64

	if landscapeLegend {
		// Landscape mode: legend on the RIGHT side (~30% of width).
		// The pie gets the remaining ~70% of width and the full height.
		legendWidthFraction := 0.30
		legendW := plotArea.W * legendWidthFraction
		pieW := plotArea.W - legendW - style.Spacing.MD // gap between pie and legend

		pieArea = Rect{
			X: plotArea.X,
			Y: plotArea.Y,
			W: pieW,
			H: plotArea.H,
		}

		// Measure legend height for vertical layout within the right column
		legendConfig := PresentationPieLegendConfig(style)
		legendConfig.Layout = LegendLayoutVertical
		legendConfig.HorizontalAlign = "left"
		legend := NewLegend(b, legendConfig)
		legend.SetItems(pieLegendItems(values, labels, style.Palette.AccentColors()))
		measuredLegendH := legend.Height(legendW)

		legendBounds = Rect{
			X: plotArea.X + pieW + style.Spacing.MD,
			Y: plotArea.Y + (plotArea.H-measuredLegendH)/2, // vertically center
			W: legendW,
			H: math.Min(measuredLegendH, plotArea.H),
		}
	} else {
		// Portrait/square mode: legend at the BOTTOM (existing behavior).
		legendHeight = 0.0
		if pc.config.ShowLegend {
			legendHeight = pc.measureLegendHeight(values, labels, plotArea.W)
			// Dynamic cap: allow more legend space when there are many items.
			// The cap prevents the legend from consuming too much vertical space,
			// but must never clip items — a truncated legend is worse than a
			// slightly smaller pie.
			numItems := len(values)
			capFraction := 0.25
			if numItems >= 8 {
				capFraction = 0.40
			} else if numItems >= 6 {
				capFraction = 0.35
			} else if numItems >= 4 {
				capFraction = 0.30
			}
			maxLegend := plotArea.H * capFraction
			if legendHeight > maxLegend {
				// Safety: never cap below the measured height if doing so would
				// clip legend items. Allow up to 45% of plot height as an
				// absolute ceiling to guarantee all items remain visible.
				absoluteMax := plotArea.H * 0.45
				if legendHeight <= absoluteMax {
					// The measured height fits within the absolute ceiling —
					// use it uncapped so all items are visible.
					// (legendHeight stays as-is)
				} else {
					legendHeight = absoluteMax
				}
			}
		}

		pieArea = Rect{
			X: plotArea.X,
			Y: plotArea.Y,
			W: plotArea.W,
			H: plotArea.H - legendHeight,
		}
	}

	// Calculate center and radius from the pie area
	centerX := pieArea.X + pieArea.W/2
	centerY := pieArea.Y + pieArea.H/2
	halfSize := math.Min(pieArea.W, pieArea.H) / 2
	radiusScale := 0.9
	if pc.config.LabelPosition == ArcLabelOutside && pc.config.ShowLabels {
		// Dynamically size the radius so the longest outside label fits.
		// Label extends: outerRadius + spacing.MD (gap) + spacing.XS (pad) + labelWidth
		// This must fit within halfSize.
		total := 0.0
		for _, v := range values {
			total += v
		}
		maxLabelW := 0.0
		if total > 0 {
			b.Push()
			b.SetFontSize(style.Typography.SizeBody)
			b.SetFontWeight(style.Typography.WeightNormal)
			for i, v := range values {
				pct := (v / total) * 100
				pctStr := formatValue(pct, pc.config.LabelFormat)
				lbl := pctStr
				if i < len(labels) && labels[i] != "" {
					lbl = labels[i] + " " + pctStr
				}
				w, _ := b.MeasureText(lbl)
				if w > maxLabelW {
					maxLabelW = w
				}
			}
			b.Pop()
		}
		// Diagrams are embedded as PNG (not SVG), so the Go canvas rasterizer
		// determines final text sizes. No LibreOffice SVG text scaling mismatch.
		// A small 10% margin accounts for font metric approximation.
		maxLabelW *= 1.1
		labelGap := style.Spacing.MD + style.Spacing.XS // gap from arc + alignment pad
		needed := labelGap + maxLabelW
		// Radius must leave room for the label on each side
		maxRadius := halfSize - needed
		// Floor at 65% of halfSize so the chart never becomes too tiny
		minRadius := halfSize * 0.65
		if maxRadius < minRadius {
			maxRadius = minRadius
		}
		radiusScale = maxRadius / halfSize
	}
	radius := halfSize * radiusScale

	// In landscape mode the pie circle may be much narrower than pieArea.W
	// (height is the constraining dimension). Re-center the pie+legend group
	// within the full plotArea width to eliminate dead space on the right.
	if landscapeLegend {
		actualPieW := 2 * halfSize // visual diameter of pie bounding area
		gap := style.Spacing.MD
		groupW := actualPieW + gap + legendBounds.W
		if groupW < plotArea.W {
			groupX := plotArea.X + (plotArea.W-groupW)/2
			centerX = groupX + actualPieW/2
			legendBounds.X = groupX + actualPieW + gap
		}
	}

	innerRadius := pc.config.InnerRadius
	if innerRadius < 0 {
		// Auto: 50% of outer radius for donut
		innerRadius = radius * 0.5
	}

	// Draw arcs
	arcConfig := DefaultArcSeriesConfig(centerX, centerY, radius)
	arcConfig.InnerRadius = innerRadius
	arcConfig.StartAngle = pc.config.StartAngle
	arcConfig.PadAngle = pc.config.PadAngle
	arcConfig.ShowLabels = pc.config.ShowLabels
	arcConfig.LabelPosition = pc.config.LabelPosition
	arcConfig.LabelFormat = pc.config.LabelFormat
	arcConfig.ExplodeOffset = pc.config.ExplodeOffset
	arcConfig.ExplodedSlices = pc.config.ExplodedSlices
	arcConfig.Colors = colors

	arcs := NewArcSeries(b, arcConfig)

	slices := make([]ArcSlice, len(values))
	for i, v := range values {
		slices[i] = ArcSlice{
			Value: v,
		}
		if i < len(labels) {
			slices[i].Label = labels[i]
		}
	}
	arcs.Draw(slices)

	// Draw title
	if pc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: pc.config.Width, H: headerHeight + pc.config.MarginTop})
	}

	// Draw legend
	if pc.config.ShowLegend {
		if landscapeLegend {
			pc.drawLegendInBounds(values, labels, colors, legendBounds, true)
		} else {
			pc.drawLegend(values, labels, colors, pieArea, legendHeight)
		}
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: pc.config.Height - footerHeight,
			W: pc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// pieLegendItems builds LegendItem slice from values/labels/colors.
func pieLegendItems(values []float64, labels []string, colors []Color) []LegendItem {
	items := make([]LegendItem, len(values))
	for i := range values {
		label := ""
		if i < len(labels) {
			label = labels[i]
		} else {
			label = fmt.Sprintf("Slice %d", i+1)
		}
		items[i] = LegendItem{
			Label: label,
			Color: colors[i%len(colors)],
		}
	}
	return items
}

// measureLegendHeight measures the height needed by the pie chart legend.
func (pc *PieChart) measureLegendHeight(values []float64, labels []string, availableWidth float64) float64 {
	b := pc.builder
	style := b.StyleGuide()

	legendConfig := PresentationPieLegendConfig(style)
	legend := NewLegend(b, legendConfig)
	legend.SetItems(pieLegendItems(values, labels, style.Palette.AccentColors()))

	return legend.Height(availableWidth) + style.Spacing.MD
}

// drawLegend draws the chart legend below the pie area (bottom placement).
func (pc *PieChart) drawLegend(values []float64, labels []string, colors []Color, plotArea Rect, legendHeight float64) {
	b := pc.builder
	style := b.StyleGuide()

	legendConfig := PresentationPieLegendConfig(style)

	legend := NewLegend(b, legendConfig)
	legend.SetItems(pieLegendItems(values, labels, colors))

	// Add small gap between chart and legend for visual clarity
	legendBounds := Rect{
		X: plotArea.X,
		Y: plotArea.Y + plotArea.H + style.Spacing.MD,
		W: plotArea.W,
		H: legendHeight,
	}
	legend.Draw(legendBounds)
}

// drawLegendInBounds draws the chart legend within pre-calculated bounds.
// When vertical is true, uses a vertical layout suitable for right-side placement.
func (pc *PieChart) drawLegendInBounds(values []float64, labels []string, colors []Color, bounds Rect, vertical bool) {
	b := pc.builder
	style := b.StyleGuide()

	legendConfig := PresentationPieLegendConfig(style)
	if vertical {
		legendConfig.Layout = LegendLayoutVertical
		legendConfig.HorizontalAlign = "left"
		legendConfig.VerticalAlign = "middle"
	}

	legend := NewLegend(b, legendConfig)
	legend.SetItems(pieLegendItems(values, labels, colors))
	legend.Draw(bounds)
}

// getColors returns colors for the slices.
func (pc *PieChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(pc.config.Colors, style, count)
}

// =============================================================================
// Radar Chart
// =============================================================================

// RadarChartConfig holds configuration for radar charts.
type RadarChartConfig struct {
	ChartConfig

	// FillOpacity is the fill opacity for each series (0-1).
	FillOpacity float64

	// ShowPoints enables data point markers.
	ShowPoints bool

	// PointSize is the marker size.
	PointSize float64

	// Levels is the number of concentric grid levels.
	Levels int

	// MaxValue overrides automatic max calculation.
	MaxValue float64
}

// DefaultRadarChartConfig returns default radar chart configuration.
func DefaultRadarChartConfig(width, height float64) RadarChartConfig {
	return RadarChartConfig{
		ChartConfig: DefaultChartConfig(width, height),
		FillOpacity: 0.2,
		ShowPoints:  true,
		PointSize:   6,
		Levels:      5,
		MaxValue:    0, // Auto-calculate
	}
}

// RadarChart renders radar/spider charts.
type RadarChart struct {
	builder *SVGBuilder
	config  RadarChartConfig
}

// NewRadarChart creates a new radar chart renderer.
func NewRadarChart(builder *SVGBuilder, config RadarChartConfig) *RadarChart {
	return &RadarChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the radar chart.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (rc *RadarChart) Draw(data ChartData) error {
	if len(data.Series) == 0 || len(data.Categories) == 0 {
		return fmt.Errorf("radar chart requires series and categories")
	}

	b := rc.builder
	style := b.StyleGuide()
	colors := rc.getColors(style, len(data.Series))

	// Calculate plot area
	plotArea := rc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if rc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Adjust for footnote
	footerHeight := 0.0
	if data.Footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}

	legendHeight := 0.0
	if rc.config.ShowLegend && len(data.Series) > 1 {
		legendHeight = style.Typography.SizeSmall + style.Spacing.LG
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight
	if rc.config.LegendPosition == LegendPositionBottom {
		plotArea.H -= legendHeight
	}

	// Calculate center and radius — shrink radius when there are many axes
	// so that labels have more room around the perimeter.
	centerX := plotArea.X + plotArea.W/2
	centerY := plotArea.Y + plotArea.H/2
	radiusFactor := 0.8
	numAxes := len(data.Categories)
	if numAxes >= 16 {
		radiusFactor = 0.60
	} else if numAxes >= 12 {
		radiusFactor = 0.65
	} else if numAxes >= 8 {
		radiusFactor = 0.70
	}
	maxHalfDim := math.Min(plotArea.W, plotArea.H) / 2
	radius := maxHalfDim * radiusFactor

	// Ensure labels are never hyphenated: measure the widest single word at
	// minimum font size and shrink the radius if the most-constrained label
	// position (left side) would not have enough room.
	minLabelFont := math.Max(style.Typography.SizeCaption, 8.0)
	b.SetFontSize(minLabelFont)
	b.SetFontWeight(style.Typography.WeightNormal)
	var widestWord float64
	for _, cat := range data.Categories {
		for _, word := range strings.Fields(cat) {
			w, _ := b.MeasureText(word)
			if w > widestWord {
				widestWord = w
			}
		}
	}
	// The most constrained position is a left-side label whose anchor is at
	// centerX - (radius + spacing.MD). Available width = anchor - spacing.XS.
	// Solve for radius: anchor - spacing.XS >= widestWord + padding
	// centerX - (radius + spacing.MD) - spacing.XS >= widestWord + spacing.XS
	// radius <= centerX - spacing.MD - widestWord - 2*spacing.XS
	if widestWord > 0 {
		neededLabelW := widestWord + 2*style.Spacing.XS
		maxRadius := (centerX - plotArea.X - style.Spacing.MD - neededLabelW) / 0.81 // worst-case cos ≈ -0.81
		minRadius := maxHalfDim * 0.40 // don't shrink below 40%
		if maxRadius < minRadius {
			maxRadius = minRadius
		}
		if radius > maxRadius {
			radius = maxRadius
		}
	}

	// Calculate max value
	maxValue := rc.config.MaxValue
	if maxValue == 0 {
		for _, series := range data.Series {
			for _, v := range series.Values {
				if v > maxValue {
					maxValue = v
				}
			}
		}
		maxValue *= 1.1 // Add 10% padding
	}

	// Guard against division-by-zero when all values are zero
	if maxValue == 0 {
		maxValue = 1.0
	}

	angleStep := 2 * math.Pi / float64(numAxes)

	// Draw grid
	if rc.config.ShowGrid {
		rc.drawGrid(centerX, centerY, radius, numAxes, angleStep, maxValue)
	}

	// Draw axes
	if rc.config.ShowAxes {
		rc.drawAxes(centerX, centerY, radius, data.Categories, angleStep)
	}

	// Draw series
	for seriesIdx, series := range data.Series {
		rc.drawSeries(centerX, centerY, radius, maxValue, series, angleStep, colors[seriesIdx%len(colors)])
	}

	// Draw title
	if rc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: rc.config.Width, H: headerHeight + rc.config.MarginTop})
	}

	// Draw legend
	if rc.config.ShowLegend && len(data.Series) > 1 {
		rc.drawLegend(data, colors, plotArea, legendHeight)
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: rc.config.Height - footerHeight,
			W: rc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// drawGrid draws the radar grid.
func (rc *RadarChart) drawGrid(centerX, centerY, radius float64, numAxes int, angleStep float64, maxValue float64) {
	b := rc.builder
	style := b.StyleGuide()

	b.Push()
	b.SetStrokeColor(style.Palette.Border.WithAlpha(0.5))
	b.SetStrokeWidth(style.Strokes.WidthHairline)

	// Draw concentric polygons
	for level := 1; level <= rc.config.Levels; level++ {
		levelRadius := radius * float64(level) / float64(rc.config.Levels)

		points := make([]Point, numAxes)
		for i := 0; i < numAxes; i++ {
			angle := -math.Pi/2 + float64(i)*angleStep
			points[i] = Point{
				X: centerX + levelRadius*math.Cos(angle),
				Y: centerY + levelRadius*math.Sin(angle),
			}
		}

		// Draw polygon
		path := b.BeginPath()
		path.MoveTo(points[0].X, points[0].Y)
		for i := 1; i < len(points); i++ {
			path.LineTo(points[i].X, points[i].Y)
		}
		path.Close()
		path.Stroke()
	}

	// Draw spokes
	for i := 0; i < numAxes; i++ {
		angle := -math.Pi/2 + float64(i)*angleStep
		endX := centerX + radius*math.Cos(angle)
		endY := centerY + radius*math.Sin(angle)
		b.DrawLine(centerX, centerY, endX, endY)
	}

	b.Pop()

	// Draw value scale labels along the 12-o'clock spoke (top axis)
	rc.drawScaleLabels(centerX, centerY, radius, maxValue)
}

// drawScaleLabels draws value labels at each concentric gridline along the
// top (12-o'clock) spoke so users can read exact scores.
func (rc *RadarChart) drawScaleLabels(centerX, centerY, radius, maxValue float64) {
	b := rc.builder
	style := b.StyleGuide()

	// Use caption size for scale labels — subtle but readable
	labelSize := style.Typography.SizeCaption
	// Offset labels slightly to the right of the spoke to avoid overlap
	xOffset := style.Spacing.XS + 2

	b.Push()
	b.SetFontSize(labelSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextMuted)

	for level := 1; level <= rc.config.Levels; level++ {
		levelRadius := radius * float64(level) / float64(rc.config.Levels)
		value := maxValue * float64(level) / float64(rc.config.Levels)

		// Position along the top spoke (angle = -π/2, i.e. straight up)
		labelX := centerX + xOffset
		labelY := centerY - levelRadius

		// Format label: use integer if whole number, otherwise one decimal
		var label string
		if value == math.Trunc(value) {
			label = fmt.Sprintf("%.0f", value)
		} else {
			label = fmt.Sprintf("%.1f", value)
		}

		b.DrawText(label, labelX, labelY, TextAlignLeft, TextBaselineMiddle)
	}

	b.Pop()
}

// drawAxes draws the radar axis labels.
func (rc *RadarChart) drawAxes(centerX, centerY, radius float64, categories []string, angleStep float64) {
	b := rc.builder
	style := b.StyleGuide()

	// Start from SizeBody and shrink if labels don't fit in maxLabelLines.
	// For dense radars (many axes), start smaller and limit to 1 line.
	b.SetFontWeight(style.Typography.WeightNormal)

	numCats := len(categories)
	labelOffset := radius + style.Spacing.MD
	maxLabelLines := 2
	startFontSize := style.Typography.SizeBody
	if numCats >= 16 {
		startFontSize = style.Typography.SizeCaption
		maxLabelLines = 1
	} else if numCats >= 12 {
		startFontSize = style.Typography.SizeSmall
		maxLabelLines = 1
	} else if numCats >= 8 {
		startFontSize = math.Min(style.Typography.SizeBody, style.Typography.SizeSmall+1)
	}
	// Minimum font size for axis labels — shrink to fit, but stay readable.
	minLabelFontSize := math.Max(style.Typography.SizeCaption, 8.0)

	for i, cat := range categories {
		angle := -math.Pi/2 + float64(i)*angleStep
		cosA := math.Cos(angle)
		sinA := math.Sin(angle)

		labelX := centerX + labelOffset*cosA
		labelY := centerY + labelOffset*sinA

		// Determine horizontal alignment and available width based on
		// where the label sits around the chart.
		var hAlign HorizontalAlign
		var maxLabelW float64

		if cosA > 0.1 {
			// Right side: label starts at labelX, extends rightward.
			hAlign = HorizontalAlignLeft
			maxLabelW = rc.config.Width - labelX - style.Spacing.XS
		} else if cosA < -0.1 {
			// Left side: label ends at labelX, extends leftward.
			hAlign = HorizontalAlignRight
			maxLabelW = labelX - style.Spacing.XS
		} else {
			// Top or bottom center: center-aligned.
			hAlign = HorizontalAlignCenter
			maxLabelW = math.Min(labelX, rc.config.Width-labelX) * 2
		}
		// Guarantee a sane minimum so we always show something.
		if maxLabelW < 20 {
			maxLabelW = 20
		}

		// Shrink font if the label doesn't fit in maxLabelLines.
		fontSize := startFontSize
		b.SetFontSize(fontSize)
		block := b.WrapText(cat, maxLabelW)
		for len(block.Lines) > maxLabelLines && fontSize > minLabelFontSize {
			fontSize -= 0.5
			if fontSize < minLabelFontSize {
				fontSize = minLabelFontSize
			}
			b.SetFontSize(fontSize)
			block = b.WrapText(cat, maxLabelW)
		}
		if len(block.Lines) == 0 {
			continue
		}

		// Limit to maxLabelLines; truncate last visible line with ellipsis.
		if len(block.Lines) > maxLabelLines {
			block.Lines = block.Lines[:maxLabelLines]
			last := &block.Lines[maxLabelLines-1]
			last.Text = b.TruncateToWidth(last.Text+"…", maxLabelW)
			lw, _ := b.MeasureText(last.Text)
			last.Width = lw
		}

		// Recalculate total height for the (possibly trimmed) block.
		lineSpacing := block.LineHeight
		if style.Typography != nil {
			lineSpacing = block.LineHeight * style.Typography.LineHeight
		}
		blockH := float64(len(block.Lines)) * lineSpacing

		// Adjust startY so that the block is vertically centred on labelY.
		startY := labelY - blockH/2 + block.LineHeight*0.5

		// For labels at the very top, nudge down so they don't clip above
		// the SVG; for labels at the very bottom, nudge up.
		if sinA < -0.5 {
			// Top region: ensure first line stays inside the canvas.
			if startY < style.Spacing.XS {
				startY = style.Spacing.XS
			}
		} else if sinA > 0.5 {
			// Bottom region: ensure last line stays inside the canvas.
			bottomEdge := startY + blockH
			if bottomEdge > rc.config.Height-style.Spacing.XS {
				startY = rc.config.Height - style.Spacing.XS - blockH
			}
		}

		b.DrawTextBlock(block, labelX, startY, hAlign)
	}
}

// drawSeries draws a single series on the radar chart.
func (rc *RadarChart) drawSeries(centerX, centerY, radius, maxValue float64, series ChartSeries, angleStep float64, color Color) {
	b := rc.builder
	style := b.StyleGuide()

	if len(series.Values) == 0 {
		return
	}

	numPoints := len(series.Values)
	points := make([]Point, numPoints)

	for i, v := range series.Values {
		normalizedValue := v / maxValue
		if normalizedValue > 1 {
			normalizedValue = 1
		}
		if normalizedValue < 0 {
			normalizedValue = 0
		}

		angle := -math.Pi/2 + float64(i)*angleStep
		pointRadius := radius * normalizedValue

		points[i] = Point{
			X: centerX + pointRadius*math.Cos(angle),
			Y: centerY + pointRadius*math.Sin(angle),
		}
	}

	// Draw filled area
	b.Push()
	b.SetFillColor(color.WithAlpha(rc.config.FillOpacity))
	b.SetStrokeColor(color)
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawPolygon(points)
	b.Pop()

	// Draw points
	if rc.config.ShowPoints {
		b.Push()
		b.SetFillColor(color)
		b.SetStrokeColor(MustParseColor("#FFFFFF"))
		b.SetStrokeWidth(1.5)

		for _, p := range points {
			b.DrawCircle(p.X, p.Y, rc.config.PointSize/2)
		}

		b.Pop()
	}
}

// drawLegend draws the chart legend.
func (rc *RadarChart) drawLegend(data ChartData, colors []Color, plotArea Rect, legendHeight float64) {
	b := rc.builder
	style := b.StyleGuide()

	legendConfig := PresentationLegendConfig(style)

	legend := NewLegend(b, legendConfig)

	items := make([]LegendItem, len(data.Series))
	for i, series := range data.Series {
		items[i] = LegendItem{
			Label: series.Name,
			Color: colors[i%len(colors)],
		}
	}
	legend.SetItems(items)

	// Add small gap between chart and legend for visual clarity
	legendBounds := Rect{
		X: plotArea.X,
		Y: plotArea.Y + plotArea.H + style.Spacing.MD,
		W: plotArea.W,
		H: legendHeight,
	}
	legend.Draw(legendBounds)
}

// getColors returns colors for the series.
func (rc *RadarChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(rc.config.Colors, style, count)
}
