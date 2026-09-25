package svggen

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
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

	// Annotations are optional chart overlays (reference lines, trendlines, callouts).
	Annotations []Annotation

	// DataLabels controls data label formatting and display.
	DataLabels *DataLabelConfig
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

	// ShowVerticalGrid, when true, draws vertical gridlines for Cartesian
	// charts (in addition to the horizontal lines that ShowGrid governs).
	// Default false matches the executive token
	// tokens.ChartHideVerticalGridlines. Honored by BarChart, LineChart, and
	// AreaChart; bar charts use category boundaries, line/area use x ticks.
	ShowVerticalGrid bool

	// ForceLegendSingleSeries, when true, renders the legend even on
	// single-series charts. Default false matches the executive token
	// tokens.ChartLegendMinSeries (>=2). Honored anywhere the existing
	// `len(series) > 1` legend gate runs (Bar/Line/Area/Radar/Scatter).
	ForceLegendSingleSeries bool

	// PreferDirectLabels, when true, suppresses the legend and draws
	// inline series labels (at the line end for line/area, above the
	// last bar of each series for bar) when the series count falls in
	// the direct-label window [MinLegendSeriesCount, MaxDirectLabelSeriesCount].
	// Mirrors the executive token tokens.ChartDirectLabelMaxSeries.
	// Above the window the legend wins because in-plot labels collide.
	// Honored by BarChart (non-stacked), LineChart, and AreaChart.
	PreferDirectLabels bool

	// ShowValues enables value labels on data points.
	ShowValues bool

	// Scale is "linear" (the default) or an explicit "log" for bar charts.
	Scale string

	// ValueFormat is the printf-style format for values.
	ValueFormat string

	// ValueFormatSpec is the caller's value_format: one number format for the
	// value axis, the data labels and any in-mark label. Nil means the renderer
	// derives one from the data (go-slide-creator-e2ck9).
	ValueFormatSpec *ValueFormatSpec

	// ValueFmt is the resolved formatter. ResolveValueFormatter sets it from
	// ValueFormatSpec, ValueFormat and the chart's own values, once per render,
	// and every side of the chart formats through it.
	ValueFmt *ValueFormatter

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

// MinLegendSeriesCount is the smallest series count for which a legend is
// rendered by default. A single-series chart carries its label in the
// chart title or in a direct label, so the legend would be redundant.
// Mirrors internal/tokens.ChartLegendMinSeries; parity is asserted by
// TestChartDirectLabel_Parity in internal/tokens/chart_style_test.go.
const MinLegendSeriesCount = 2

// MaxDirectLabelSeriesCount is the largest series count for which direct
// (in-plot) series labels are preferred over a legend. Above this threshold
// the legend wins because direct labels collide. Mirrors
// internal/tokens.ChartDirectLabelMaxSeries.
const MaxDirectLabelSeriesCount = 4

// useDirectLabels reports whether the chart should suppress its legend and
// draw inline series labels per the executive defaults. Returns true only
// when PreferDirectLabels is set and seriesCount is in the direct-label
// window [MinLegendSeriesCount, MaxDirectLabelSeriesCount]. Outside the
// window the legend remains the right choice — too few or too many series
// to label inline without collision or redundancy.
func useDirectLabels(config ChartConfig, seriesCount int) bool {
	return config.PreferDirectLabels &&
		seriesCount >= MinLegendSeriesCount &&
		seriesCount <= MaxDirectLabelSeriesCount
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
	logScale *LogScale // non-nil only for an explicitly requested log axis

	// xDisplayLabels holds wrapped x-axis tick text from AdaptXLabels
	// (nil when labels are drawn verbatim).
	xDisplayLabels []string

	// xAxisCfg is the x-axis configuration drawAxes actually drew with. The
	// legend is placed below everything that config draws, so the two cannot
	// overprint (go-slide-creator-jp5d).
	xAxisCfg AxisConfig
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
	// Scale preparation can reserve an axis gutter or enable value labels for
	// this render. Do not leak either override into a later Draw on the same
	// chart instance (for example, a wide-range chart followed by a narrow one).
	showValues, marginLeft := bc.config.ShowValues, bc.config.MarginLeft
	defer func() {
		bc.config.ShowValues = showValues
		bc.config.MarginLeft = marginLeft
	}()
	if err := bc.prepareValueScale(data); err != nil {
		return err
	}

	b := bc.builder
	style := b.StyleGuide()
	colors := bc.getColors(style, len(data.Series))

	b.CheckChartCapacity(len(data.Series), len(data.Categories))

	// Compute adaptive x-axis labels without thinning named categories.
	isNarrow := bc.config.Width < 500
	prelimPlotW := bc.config.Width - bc.config.MarginLeft - bc.config.MarginRight
	xLayout := AdaptXLabels(b, data.Categories, prelimPlotW, style.Typography.SizeSmall, isNarrow)
	// Cap against the actual pre-label plot height, not raw canvas height:
	// title/footnote/legend reservations make the latter too generous and put
	// bar labels below their line-chart counterparts on the same canvas.
	prelimPlotH := ComputeCartesianLayout(bc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series)).PlotArea.H
	CapXLabelBand(b, &xLayout, prelimPlotH, data.Categories)
	axisFontSize := xLayout.FontSize
	xLabelRotation := xLayout.Rotation
	labelStep := xLayout.LabelStep
	categories := xLayout.Categories
	bc.xDisplayLabels = xLayout.DisplayLabels
	if xLayout.ExtraBottomMargin > 0 {
		bc.config.MarginBottom += xLayout.ExtraBottomMargin
	}

	// Calculate domain early so we can probe y-axis label widths and grow
	// MarginLeft before layout if labels would clip into the title/legend area.
	// One number format for the whole chart: the axis ticks, the bar labels and
	// the measurement that sizes the left margin all read through it
	// (go-slide-creator-e2ck9).
	bc.config.ResolveValueFormatter(chartDataValues(data), true)

	yMin, yMax := bc.calculateDomain(data)
	EnsureYAxisFits(b, &bc.config.ChartConfig, yMin, yMax)

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(bc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	layout = bc.fallBackFromCollidingDirectLabels(style, data, colors, layout)
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if bc.config.ShowLegend && !useDirectLabels(bc.config.ChartConfig, len(data.Series)) {
		RefineLegendHeightForced(b, style, data.Series, &plotArea, &legendHeight, bc.config.ForceLegendSingleSeries)
	}

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

	// Create scales from the original category identities; only axis display
	// text may be ellipsized after its label-band bounds are known.
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, plotArea.W)
	xScale.PaddingOuter(bc.config.GroupPadding)
	xScale.PaddingInner(bc.config.GroupPadding)

	// Keep series data aligned with the scale's original categories.
	displayData := data
	displayData.Categories = categories

	// Bar length encodes value on a linear axis by default. A wide range can
	// obscure small bars, so label the actual values and surface the tradeoff.
	// A logarithmic axis must be requested explicitly.
	bc.logScale = nil
	if bc.config.Scale == "log" {
		minPositive, maxPositive := bc.positiveDomainBounds(data)
		if minPositive <= 0 || maxPositive <= 0 {
			return fmt.Errorf("log scale requires at least one positive bar value")
		}
		logMin, logMax := bc.logDomainBounds(data)
		if logMin == logMax {
			logMin /= 10
			logMax *= 10
		}
		bc.logScale = NewLogScale(logMin, logMax)
		bc.logScale.SetRangeLog(plotArea.H, 0)

		// Draw grid and axes using log scale
		if bc.config.ShowGrid {
			bc.drawLogGrid(plotArea)
		}
		if bc.config.ShowAxes {
			bc.drawLogAxes(plotArea, xScale, axisFontSize, xLabelRotation, labelStep)
		}
		// Emit tick-thinned finding if log scale ticks were decimated.
		if thinned, orig, kept := bc.logScale.WasThinned(); thinned {
			b.AddFinding(Finding{
				Field:    "y_axis.ticks",
				Code:     FindingTickThinned,
				Message:  fmt.Sprintf("log-scale y-axis ticks thinned from %d to %d to prevent overlap", orig, kept),
				Severity: "info",
				Fix: &FixSuggestion{
					Kind:   FixKindReduceItems,
					Params: map[string]any{"original_count": orig, "kept_count": kept},
				},
			})
		}
		// Draw bars using log scale
		bc.drawBars(displayData, plotArea, xScale, nil, colors)

		// Annotations not supported on log-scale charts (no linear yScale)
	} else {
		yScale := NewLinearScale(yMin, withBarTopHeadroom(yMax))
		yScale.SetRangeLinear(plotArea.H, 0)
		yScale.Nice(true)

		// Draw grid (horizontal + optional vertical per chart_style override)
		if bc.config.ShowGrid {
			DrawCartesianGridWithVerticals(b, plotArea, yScale, xScale, bc.config.ShowVerticalGrid)
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

		// Draw annotations (reference lines, trendlines, callouts)
		if len(data.Annotations) > 0 {
			DrawAnnotations(b, data.Annotations, plotArea, xScale, yScale, data, colors)
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

	// Draw legend or inline direct labels. Stacked bars and log-scale bars
	// keep the legend path because each "series" is a stacked segment or
	// the log positions break the "above last bar" geometry.
	directLabels := useDirectLabels(bc.config.ChartConfig, len(data.Series)) && !bc.config.Stacked && bc.logScale == nil
	bc.drawLegendOrDirectLabels(directLabels, style, displayData, plotArea, legendHeight, colors)

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

func (bc *BarChart) prepareValueScale(data ChartData) error {
	if bc.config.Scale != "" && bc.config.Scale != "linear" && bc.config.Scale != "log" {
		return fmt.Errorf("bar chart scale must be linear or log, got %q", bc.config.Scale)
	}
	if bc.config.Scale == "log" && bc.config.Stacked {
		return fmt.Errorf("log scale is not supported for stacked bar charts")
	}
	if bc.config.Scale == "log" && bc.config.YAxisTitle == "" {
		// Reserve the same label gutter as a caller-provided axis title.
		bc.config.MarginLeft += 20
	}
	if bc.config.Scale != "log" && !bc.config.Stacked && bc.needsLogScale(data) {
		bc.config.ShowValues = true
		minVal, maxVal := bc.positiveDomainBounds(data)
		bc.builder.AddFinding(Finding{
			Field: "data.series", Code: FindingWideRangeLinear,
			Message:  fmt.Sprintf("bar values span %.0fx (%.4g to %.4g); kept linear lengths and enabled value labels", maxVal/minVal, minVal, maxVal),
			Severity: "warning",
			Fix: &FixSuggestion{Kind: FixKindExplicitScale, Params: map[string]any{
				"options": []string{"style.show_values=true", "split_chart", "style.scale=log"},
			}},
		})
	}
	return nil
}

// fallBackFromCollidingDirectLabels keeps inline series labels only when
// they fit: if any label would sit on a bar, another label, or spill off the
// canvas, it switches the chart to a legend (go-slide-creator-t2ka) and
// returns a recomputed layout so the legend band is reserved.
func (bc *BarChart) fallBackFromCollidingDirectLabels(style *StyleGuide, data ChartData, colors []Color, layout CartesianLayout) CartesianLayout {
	if !useDirectLabels(bc.config.ChartConfig, len(data.Series)) || bc.config.Stacked || bc.config.Horizontal {
		return layout
	}
	labels, bars := barDirectLabelGeometry(bc.builder, style, data, layout.PlotArea, colors, bc.config)
	if !barDirectLabelsCollide(labels, bars, bc.config.Width) {
		return layout
	}
	bc.config.PreferDirectLabels = false
	bc.config.ShowLegend = true
	return ComputeCartesianLayout(bc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
}

// drawLegendOrDirectLabels routes to either the legend renderer or the
// inline direct-label renderer based on the directLabels flag and the
// existing legend-show gates. Centralises the branch so BarChart.Draw stays
// at one decision call site and the cyclomatic complexity budget holds.
func (bc *BarChart) drawLegendOrDirectLabels(directLabels bool, style *StyleGuide, data ChartData, plotArea Rect, legendHeight float64, colors []Color) {
	b := bc.builder

	if directLabels {
		drawBarDirectSeriesLabels(b, style, data, plotArea, colors, bc.config)
		return
	}
	if !bc.config.ShowLegend {
		return
	}
	if len(data.Series) <= 1 && !bc.config.ForceLegendSingleSeries {
		return
	}

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

	// Place the legend below everything the x axis draws — ticks, tick labels
	// (including the rotation/wrap allowance) and the axis title. Measured by
	// the same helper Axis.drawTitle uses, so they cannot overprint.
	legendBounds := Rect{
		X: plotArea.X,
		Y: plotArea.Y + plotArea.H + XAxisFooterHeight(style, bc.resolvedXAxisConfig()),
		W: plotArea.W,
		H: legendHeight,
	}
	legend.Draw(legendBounds)
}

// barDirectLabel is one inline series label with its measured bounding box.
type barDirectLabel struct {
	Text  string
	X, Y  float64 // anchor: center-x, bottom baseline
	Color Color
	Box   Rect
}

// barDirectLabelGeometry computes the inline series labels (one above the
// last bar of each series) and the rectangles of every rendered bar, using a
// scale that matches drawBars so positions line up with the drawn bars.
// Value-label slots above each bar are included in bars when ShowValues is
// on, because the direct label must not sit on top of a value label either.
func barDirectLabelGeometry(b *SVGBuilder, style *StyleGuide, data ChartData, plotArea Rect, colors []Color, cfg BarChartConfig) ([]barDirectLabel, []Rect) {
	if len(data.Series) == 0 || len(data.Categories) == 0 {
		return nil, nil
	}

	xScale := NewCategoricalScale(data.Categories)
	xScale.SetRangeCategorical(plotArea.X, plotArea.X+plotArea.W)
	xScale.PaddingOuter(cfg.GroupPadding)
	xScale.PaddingInner(cfg.GroupPadding)

	bandwidth := xScale.Bandwidth()
	numSeries := len(data.Series)
	groupWidth := bandwidth * (1 - cfg.BarPadding)
	barWidth := groupWidth / float64(numSeries)

	yMin, yMax := math.Inf(1), math.Inf(-1)
	for _, s := range data.Series {
		for _, v := range s.Values {
			if v < yMin {
				yMin = v
			}
			if v > yMax {
				yMax = v
			}
		}
	}
	if math.IsInf(yMin, 1) || math.IsInf(yMax, -1) {
		return nil, nil
	}
	if yMin > 0 {
		yMin = 0
	}
	yScale := NewLinearScale(yMin, withBarTopHeadroom(yMax))
	yScale.SetRangeLinear(plotArea.H, 0)
	yScale.Nice(true)
	baseY := plotArea.Y + yScale.Scale(0)
	fontSize := style.Typography.SizeSmall

	var bars []Rect
	for ci, cat := range data.Categories {
		for si, series := range data.Series {
			if ci >= len(series.Values) {
				continue
			}
			x := xScale.ScaleStart(cat) + (bandwidth-groupWidth)/2 + barWidth*float64(si)
			top := plotArea.Y + yScale.Scale(series.Values[ci])
			y0, y1 := math.Min(top, baseY), math.Max(top, baseY)
			bars = append(bars, Rect{X: x, Y: y0, W: barWidth, H: y1 - y0})
			if cfg.ShowValues {
				bars = append(bars, Rect{X: x, Y: y0 - fontSize - style.Spacing.XS, W: barWidth, H: fontSize})
			}
		}
	}

	lastCatIdx := len(data.Categories) - 1
	lastCat := data.Categories[lastCatIdx]

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	defer b.Pop()

	var labels []barDirectLabel
	for i, series := range data.Series {
		if len(series.Values) <= lastCatIdx {
			continue
		}
		v := series.Values[lastCatIdx]
		barCenterX := xScale.ScaleStart(lastCat) + (bandwidth-groupWidth)/2 + barWidth*float64(i) + barWidth/2
		barTopY := plotArea.Y + yScale.Scale(v)
		// Place label above the bar top with a small gap, clamped inside the
		// plot area. Uses SM (not XS) so the label clears the bar top cleanly.
		labelY := barTopY - style.Spacing.SM
		if labelY < plotArea.Y+fontSize {
			labelY = plotArea.Y + fontSize
		}
		color := colors[i%len(colors)]
		if series.Color != nil {
			color = *series.Color
		}
		w, h := b.MeasureText(series.Name)
		if h <= 0 {
			h = fontSize
		}
		labels = append(labels, barDirectLabel{
			Text: series.Name, X: barCenterX, Y: labelY, Color: color,
			Box: Rect{X: barCenterX - w/2, Y: labelY - h, W: w, H: h},
		})
	}
	return labels, bars
}

// barDirectLabelsCollide reports whether any inline series label would
// overlap a bar (or value-label slot), another inline label, or spill outside
// the chart canvas horizontally. A 1pt tolerance absorbs sub-pixel rounding
// so labels that merely touch a bar edge are not rejected.
func barDirectLabelsCollide(labels []barDirectLabel, bars []Rect, canvasWidth float64) bool {
	const tol = 1.0
	for i, l := range labels {
		box := l.Box.Inset(tol, tol, tol, tol)
		if l.Box.X < 0 || l.Box.X+l.Box.W > canvasWidth {
			return true
		}
		for _, r := range bars {
			if r.W > 0 && r.H > 0 && box.Intersects(r) {
				return true
			}
		}
		for j := i + 1; j < len(labels); j++ {
			if box.Intersects(labels[j].Box) {
				return true
			}
		}
	}
	return false
}

// drawBarDirectSeriesLabels draws inline series labels above the last bar of
// each series in a grouped bar chart, in the series color. Used in place of a
// legend when the series count is in the direct-label window and the labels
// fit without colliding (see barDirectLabelsCollide / BarChart.Draw).
func drawBarDirectSeriesLabels(b *SVGBuilder, style *StyleGuide, data ChartData, plotArea Rect, colors []Color, cfg BarChartConfig) {
	labels, _ := barDirectLabelGeometry(b, style, data, plotArea, colors, cfg)
	if len(labels) == 0 {
		return
	}
	b.Push()
	b.SetFontSize(style.Typography.SizeSmall)
	b.SetFontWeight(style.Typography.WeightMedium)
	for _, l := range labels {
		b.SetTextColor(l.Color)
		b.DrawText(l.Text, l.X, l.Y, TextAlignCenter, TextBaselineBottom)
	}
	b.Pop()
}

// calculateDomain calculates the y-axis domain.
func (bc *BarChart) calculateDomain(data ChartData) (min, max float64) {
	min = 0
	max = 0

	if bc.config.Stacked {
		// Stacked bars diverge from zero: positive segments stack upward and
		// negative segments stack downward (see drawStackedBars), so the
		// domain spans the deepest negative stack to the tallest positive
		// stack. Using the net sum hid negative segments below the axis
		// (go-slide-creator-s1uvj.29).
		for i := range data.Categories {
			posSum, negSum := 0.0, 0.0
			for _, series := range data.Series {
				if i < len(series.Values) {
					if v := series.Values[i]; v > 0 {
						posSum += v
					} else {
						negSum += v
					}
				}
			}
			if posSum > max {
				max = posSum
			}
			if negSum < min {
				min = negSum
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

// barTopHeadroomFactor expands a bar chart's positive y-domain max so the
// tallest bar leaves clearance for a value/direct label above its top instead
// of touching the plot ceiling. Linear bars rely on Nice() rounding for
// headroom, but when the data max already lands on a tick boundary (e.g. a
// max of 100 with a step of 20) Nice() adds nothing and the bar — plus its
// top-anchored label — crowds the plot edge. This mirrors the ~5% top padding
// line/area charts apply in calculateYDomain, using a slightly larger factor
// because bar value labels sit *above* the bar top rather than at the data
// point, so they need more clearance.
const barTopHeadroomFactor = 1.08

// withBarTopHeadroom expands a positive domain max by barTopHeadroomFactor and
// leaves non-positive maxima (negative-only or all-zero data) untouched so
// Nice() still anchors the axis at a sensible boundary. The same expansion is
// applied wherever a bar y-scale is built so the bars and their direct labels
// stay aligned.
func withBarTopHeadroom(yMax float64) float64 {
	if yMax > 0 {
		return yMax * barTopHeadroomFactor
	}
	return yMax
}

// needsLogScale identifies wide positive ranges for a linear-axis warning.
func (bc *BarChart) needsLogScale(data ChartData) bool {
	minPos, maxVal := bc.positiveDomainBounds(data)
	hasNeg := false

	for _, s := range data.Series {
		for _, v := range s.Values {
			if v < 0 {
				hasNeg = true
			}
		}
	}
	if hasNeg || minPos <= 0 || maxVal <= 0 {
		return false
	}
	return maxVal/minPos >= 1000
}

func (bc *BarChart) positiveDomainBounds(data ChartData) (min, max float64) {
	min = math.Inf(1)
	for _, s := range data.Series {
		for _, v := range s.Values {
			if v > 0 && v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}
	if math.IsInf(min, 1) {
		min = 0
	}
	return min, max
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

	// X axis (same as linear). The shared AxisConfig owns rotated-label geometry.
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = bc.config.XAxisTitle
	xAxisConfig.FontSize = axisFontSize
	xAxisConfig.LabelRotation = xLabelRotation
	xAxisConfig.LabelStep = labelStep
	xAxisConfig.DisplayLabels = bc.xDisplayLabels

	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawCategoricalAxis(xScale, plotArea.X, plotArea.Y+plotArea.H)

	// Y axis — use log scale labels
	yAxisConfig := DefaultAxisConfig(AxisPositionLeft)
	yAxisConfig.Title = "Log scale"
	if bc.config.YAxisTitle != "" {
		yAxisConfig.Title = bc.config.YAxisTitle + " (log scale)"
	}
	yAxis := NewAxis(b, yAxisConfig)
	yAxis.DrawLogAxis(bc.logScale, plotArea.X, plotArea.Y)
}

// drawAxes draws the chart axes.
func (bc *BarChart) drawAxes(plotArea Rect, xScale *CategoricalScale, yScale *LinearScale, axisFontSize, xLabelRotation float64, labelStep int) {
	b := bc.builder

	// X axis — with density-adaptive font size and rotation. The shared
	// AxisConfig owns rotated-label pivot geometry.
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = bc.config.XAxisTitle
	xAxisConfig.FontSize = axisFontSize
	xAxisConfig.LabelRotation = xLabelRotation
	xAxisConfig.LabelStep = labelStep
	xAxisConfig.DisplayLabels = bc.xDisplayLabels
	bc.xAxisCfg = xAxisConfig

	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawCategoricalAxis(xScale, plotArea.X, plotArea.Y+plotArea.H)

	// Y axis (shared)
	DrawCartesianYAxis(b, plotArea, yScale, bc.config.YAxisTitle, bc.config.ValueFmt)
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

	// Show enough decimals for the labels to be distinct. Seven bars reading
	// "5, 5, 5, 6, 6, 6, 7" contradict their own axis (go-slide-creator-66qb).
	valueFormat := autoValueFormat(bc.config.ValueFormat, chartDataValues(data))

	for seriesIdx, series := range data.Series {
		barConfig := DefaultBarSeriesConfig()
		barConfig.Color = colors[seriesIdx%len(colors)]
		barConfig.CornerRadius = bc.config.CornerRadius
		barConfig.ShowValues = bc.config.ShowValues
		barConfig.ValueFormat = valueFormat
		barConfig.ValueFmt = bc.config.ValueFmt
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

	// Track cumulative values per category for stacking. Positive and
	// negative segments keep separate running totals so the stack diverges
	// from zero: positives grow upward, negatives grow downward
	// (go-slide-creator-s1uvj.29).
	numCategories := len(data.Categories)
	posCumulative := make([]float64, numCategories)
	negCumulative := make([]float64, numCategories)
	cumulativeFor := func(pos, neg []float64, v float64) []float64 {
		if v > 0 {
			return pos
		}
		return neg
	}

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

			// Bottom of this segment = same-sign cumulative so far
			cumulative := cumulativeFor(posCumulative, negCumulative, v)
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

		posForLabels := make([]float64, numCategories)
		negForLabels := make([]float64, numCategories)

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

				cumulativeForLabels := cumulativeFor(posForLabels, negForLabels, v)
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

	// xAxisCfg is the x-axis configuration drawAxes actually drew with; the
	// legend is placed below it (go-slide-creator-jp5d).
	xAxisCfg AxisConfig
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

	b.CheckChartCapacity(len(data.Series), len(data.Categories))

	// Reserve horizontal space on the right for inline series labels when
	// direct labels are active so the labels (drawn at the line end) fit
	// inside the SVG viewport instead of being clipped.
	directLabels := useDirectLabels(lc.config.ChartConfig, len(data.Series))
	directLabelMargin := 0.0
	if directLabels {
		directLabelMargin = measureDirectLabelMargin(b, style, data.Series)
		lc.config.MarginRight += directLabelMargin
	}

	// Calculate y-axis domain early so we can probe label widths and grow
	// MarginLeft before layout if labels would clip into the title/legend area.
	lc.config.ResolveValueFormatter(chartDataValues(data), true)

	yMin, yMax := lc.calculateYDomain(data)
	EnsureYAxisFits(b, &lc.config.ChartConfig, yMin, yMax)

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(lc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if lc.config.ShowLegend && !directLabels {
		RefineLegendHeightForced(b, style, data.Series, &plotArea, &legendHeight, lc.config.ForceLegendSingleSeries)
	}

	// Check if data is time-series
	isTimeSeries := lc.hasTimeSeriesData(data)

	// Adaptive x-axis label layout for categorical data
	isNarrow := lc.config.Width < 500
	var xLayout XLabelLayout
	categories := data.Categories

	if len(data.Categories) > 0 && !isTimeSeries {
		prelimPlotW := plotArea.W
		xLayout = AdaptXLabels(b, data.Categories, prelimPlotW, style.Typography.SizeSmall, isNarrow)
		CapXLabelBand(b, &xLayout, plotArea.H, data.Categories)
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

	// Draw grid (horizontal + optional vertical per chart_style override)
	if lc.config.ShowGrid {
		DrawCartesianGridWithVerticals(b, plotArea, yScale, xScale, lc.config.ShowVerticalGrid)
	}

	// Draw axes
	if lc.config.ShowAxes {
		lc.drawAxes(plotArea, xScale, yScale, categories, xLayout)
	}

	// Emit tick-thinned finding if time scale ticks were decimated.
	if timeScale != nil {
		if thinned, orig, kept := timeScale.WasThinned(); thinned {
			b.AddFinding(Finding{
				Field:    "x_axis.ticks",
				Code:     FindingTickThinned,
				Message:  fmt.Sprintf("time-axis ticks thinned from %d to %d to prevent overlap", orig, kept),
				Severity: "info",
				Fix: &FixSuggestion{
					Kind:   FixKindReduceItems,
					Params: map[string]any{"original_count": orig, "kept_count": kept},
				},
			})
		}
	}

	// Draw lines using the original categorical-scale keys. Axis display
	// labels may be shortened independently.
	drawData := data
	if len(categories) > 0 && !isTimeSeries {
		drawData.Categories = categories
	}
	lc.drawLines(drawData, plotArea, xScale, yScale, colors)

	// Draw annotations (reference lines, trendlines, callouts)
	if len(data.Annotations) > 0 {
		DrawAnnotations(b, data.Annotations, plotArea, xScale, yScale, data, colors)
	}

	// Draw title
	if lc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: lc.config.Width, H: headerHeight + lc.config.MarginTop})
	}

	// Draw legend or inline direct labels.
	lc.drawLegendOrDirectLabels(directLabels, style, drawData, plotArea, xScale, yScale, legendHeight, colors, directLabelMargin)

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

// drawLegendOrDirectLabels routes to either the legend renderer or the
// inline direct-label renderer for line/area charts. Centralises the branch
// so LineChart.Draw stays at one decision call site and the cyclomatic
// complexity budget holds.
func (lc *LineChart) drawLegendOrDirectLabels(directLabels bool, style *StyleGuide, data ChartData, plotArea Rect, xScale Scale, yScale *LinearScale, legendHeight float64, colors []Color, marginRight float64) {
	b := lc.builder

	if directLabels {
		if drawLineDirectSeriesLabels(b, style, data, plotArea, xScale, yScale, colors, marginRight) {
			return
		}
		// The end labels could not be stacked inside the plot without
		// overlapping, so the series are named in a legend instead — an
		// illegible blob at the right edge is worse than a legend
		// (go-slide-creator-lntx).
		b.AddFinding(Finding{
			Code:     FindingOverflowSuppressed,
			Message:  fmt.Sprintf("direct series labels suppressed — %d series end too close together to label inline; a legend is drawn instead", len(data.Series)),
			Severity: "info",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"series": len(data.Series)},
			},
		})
	}
	if !lc.config.ShowLegend {
		return
	}
	if len(data.Series) <= 1 && !lc.config.ForceLegendSingleSeries {
		return
	}

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

	legendBounds := Rect{
		X: plotArea.X,
		Y: plotArea.Y + plotArea.H + XAxisFooterHeight(style, lc.resolvedXAxisConfig()),
		W: plotArea.W,
		H: legendHeight,
	}
	legend.Draw(legendBounds)
}

// measureDirectLabelMargin returns the right-margin extra space required to
// fit inline series labels (drawn at the end of each line) without clipping.
// Sized off the widest series name plus a small gap. Caller adds this to
// config.MarginRight before computing the cartesian layout.
func measureDirectLabelMargin(b *SVGBuilder, style *StyleGuide, series []ChartSeries) float64 {
	if len(series) == 0 {
		return 0
	}
	b.Push()
	b.SetFontSize(style.Typography.SizeSmall)
	var maxW float64
	for _, s := range series {
		w, _ := b.MeasureText(s.Name)
		if w > maxW {
			maxW = w
		}
	}
	b.Pop()
	return maxW*1.1 + style.Spacing.SM
}

// drawLineDirectSeriesLabels draws inline series labels at the rightmost
// data point of each line, in the series color. Used in place of a legend
// when the series count is in the direct-label window.
func drawLineDirectSeriesLabels(b *SVGBuilder, style *StyleGuide, data ChartData, plotArea Rect, xScale Scale, yScale *LinearScale, colors []Color, marginRight float64) bool {
	if len(data.Series) == 0 {
		return true
	}

	labels := lineEndLabels(data, plotArea, xScale, yScale, colors)
	if len(labels) == 0 {
		return true
	}

	// Minimum vertical distance between two label baselines. Anything closer
	// and the glyphs touch.
	lineH := style.Typography.SizeSmall * lineLabelLineHeight
	if !stackLineEndLabels(labels, plotArea, lineH) {
		return false
	}

	b.Push()
	b.SetFontSize(style.Typography.SizeSmall)
	b.SetFontWeight(style.Typography.WeightMedium)

	maxLabelX := plotArea.X + plotArea.W + marginRight
	for _, l := range labels {
		labelX := math.Min(l.x+style.Spacing.XS, maxLabelX)

		// A label that had to move needs a leader back to its own line, or the
		// reader cannot tell which series it names.
		if math.Abs(l.labelY-l.y) > lineLabelLeaderMinShift {
			b.SetStrokeColor(l.color.WithAlpha(lineLabelLeaderAlpha))
			b.SetStrokeWidth(style.Strokes.WidthThin)
			b.DrawLine(l.x, l.y, labelX-style.Spacing.XS/2, l.labelY)
		}

		b.SetTextColor(l.color)
		b.DrawText(l.name, labelX, l.labelY, TextAlignLeft, TextBaselineMiddle)
	}

	b.Pop()
	return true
}

const (
	// lineLabelLineHeight is the minimum baseline-to-baseline distance between
	// two direct series labels, as a multiple of the label font size.
	lineLabelLineHeight = 1.45
	// lineLabelLeaderMinShift is how far a label must move from its line's
	// endpoint before a leader line is drawn back to it (points).
	lineLabelLeaderMinShift = 1.5
	// lineLabelLeaderAlpha keeps the leader quieter than the line it points at.
	lineLabelLeaderAlpha = 0.45
)

// lineEndLabel is one series' direct label: where its line ends, and where the
// label ends up after de-collision.
type lineEndLabel struct {
	name   string
	color  Color
	x, y   float64
	labelY float64
}

// lineEndLabels collects the rightmost drawable point of each series.
func lineEndLabels(data ChartData, plotArea Rect, xScale Scale, yScale *LinearScale, colors []Color) []*lineEndLabel {
	out := make([]*lineEndLabel, 0, len(data.Series))
	for i, series := range data.Series {
		if len(series.Values) == 0 {
			continue
		}
		var lastX, lastY float64
		found := false
		for idx, v := range series.Values {
			x, ok := lineLabelPointX(series, data, idx, plotArea, xScale)
			if !ok {
				continue
			}
			lastX, lastY = x, plotArea.Y+yScale.Scale(v)
			found = true
		}
		if !found {
			continue
		}
		color := colors[i%len(colors)]
		if series.Color != nil {
			color = *series.Color
		}
		out = append(out, &lineEndLabel{name: series.Name, color: color, x: lastX, y: lastY, labelY: lastY})
	}
	return out
}

// lineLabelPointX resolves the x coordinate of one data point under whichever
// scale the chart uses.
func lineLabelPointX(series ChartSeries, data ChartData, idx int, plotArea Rect, xScale Scale) (float64, bool) {
	switch xs := xScale.(type) {
	case *CategoricalScale:
		if idx >= len(data.Categories) {
			return 0, false
		}
		return plotArea.X + xs.Scale(data.Categories[idx]), true
	case *LinearScale:
		if idx < len(series.XValues) {
			return xs.Scale(series.XValues[idx]), true
		}
		return xs.Scale(float64(idx)), true
	case *TimeScale:
		timeValues, err := series.GetTimeValues()
		if err != nil || idx >= len(timeValues) {
			return 0, false
		}
		return xs.Scale(timeValues[idx]), true
	}
	return 0, false
}

// stackLineEndLabels pushes labels apart so consecutive baselines are at least
// lineH apart, keeping the stack inside the plot area. It reports false when
// the labels cannot fit at all, in which case the caller draws a legend.
//
// Three series ending at 43, 44 and 44 drew all three names on top of each
// other — one illegible blob at the right edge, with nothing to say so
// (go-slide-creator-lntx).
func stackLineEndLabels(labels []*lineEndLabel, plotArea Rect, lineH float64) bool {
	if len(labels) == 0 {
		return true
	}
	if float64(len(labels))*lineH > plotArea.H {
		return false
	}

	sort.Slice(labels, func(i, j int) bool { return labels[i].y < labels[j].y })

	// Downward pass: no label may sit closer than lineH to the one above it.
	labels[0].labelY = math.Max(labels[0].y, plotArea.Y+lineH/2)
	for i := 1; i < len(labels); i++ {
		labels[i].labelY = math.Max(labels[i].y, labels[i-1].labelY+lineH)
	}

	// Upward pass: if the stack overran the bottom, push it back up. The
	// capacity check above guarantees it now fits.
	if bottom := plotArea.Y + plotArea.H - lineH/2; labels[len(labels)-1].labelY > bottom {
		labels[len(labels)-1].labelY = bottom
		for i := len(labels) - 2; i >= 0; i-- {
			labels[i].labelY = math.Min(labels[i].labelY, labels[i+1].labelY-lineH)
		}
	}
	return true
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

	// A filled mark is read against the axis: the band's area IS the claim
	// ("this much of the total"), and on a stacked area the bands are a
	// part-to-whole. Truncating the axis makes a 40-unit base look like zero
	// and overstates the trend, so area and stacked_area always baseline at
	// zero — matching stacked_bar, which already did (go-slide-creator-6wfe).
	// A plain line chart keeps the zoomed axis: it encodes value by position,
	// not by area.
	if lc.config.FillArea {
		min = math.Min(0, min)
		max = math.Max(0, max)
	}

	// Add small top padding (~5%) so the highest data point doesn't touch
	// the axis boundary. Nice() will round this to a clean tick value.
	span := max - min
	max += span * 0.05

	// For line charts, allow the min to remain above zero when the data range
	// is far from zero (e.g., 100-200 should not show 0-220). Nice() will
	// round to a nearby clean number. If data is close to zero (min < 20% of
	// range), snap to zero for clarity.
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

	// X axis
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxisConfig.Title = lc.config.XAxisTitle

	switch xs := xScale.(type) {
	case *CategoricalScale:
		// Apply adaptive font size and rotation from AdaptXLabels. Nominal
		// categories always use LabelStep=1; AxisConfig owns pivot geometry.
		xAxisConfig.FontSize = xLayout.FontSize
		xAxisConfig.LabelRotation = xLayout.Rotation
		xAxisConfig.LabelStep = xLayout.LabelStep
		xAxisConfig.DisplayLabels = xLayout.DisplayLabels
		xAxisConfig.ExtraLabelHeight = xLayout.ExtraBottomMargin

		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawCategoricalAxis(xs, plotArea.X, plotArea.Y+plotArea.H)
	case *LinearScale:
		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawLinearAxis(xs, plotArea.X, plotArea.Y+plotArea.H)
	case *TimeScale:
		xAxis := NewAxis(b, xAxisConfig)
		xAxis.DrawTimeAxis(xs, plotArea.X, plotArea.Y+plotArea.H)
	}
	lc.xAxisCfg = xAxisConfig

	// Y axis (shared)
	DrawCartesianYAxis(b, plotArea, yScale, lc.config.YAxisTitle, lc.config.ValueFmt)
}

// drawLines draws the line series.
func (lc *LineChart) drawLines(data ChartData, plotArea Rect, xScale Scale, yScale *LinearScale, colors []Color) {
	b := lc.builder

	baseY := plotArea.Y + plotArea.H

	// Show enough decimals for the labels to be distinct (go-slide-creator-66qb).
	lineValueFormat := autoValueFormat(lc.config.ValueFormat, chartDataValues(data))

	for seriesIdx, series := range data.Series {
		lineConfig := DefaultLineSeriesConfig()
		lineConfig.Color = colors[seriesIdx%len(colors)]
		lineConfig.MarkerFillColor = lineConfig.Color
		lineConfig.StrokeWidth = lc.config.StrokeWidth
		lineConfig.ShowMarkers = lc.config.ShowMarkers
		lineConfig.MarkerSize = lc.config.MarkerSize
		lineConfig.Smooth = lc.config.Smooth
		lineConfig.Tension = lc.config.Tension
		lineConfig.FillArea = lc.config.FillArea
		lineConfig.FillColor = colors[seriesIdx%len(colors)]
		lineConfig.FillOpacity = lc.config.FillOpacity
		lineConfig.ShowValues = lc.config.ShowValues
		lineConfig.ValueFormat = lineValueFormat
		lineConfig.ValueFmt = lc.config.ValueFmt

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

	// xAxisCfg is the x-axis configuration drawAxes actually drew with; the
	// legend is placed below it (go-slide-creator-jp5d).
	xAxisCfg AxisConfig
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

	// Check capacity: for scatter charts, total points = sum of all series values.
	totalPoints := 0
	for _, s := range data.Series {
		totalPoints += len(s.Values)
	}
	for _, f := range core.CheckCapacity(len(data.Series), 0, totalPoints) {
		b.AddFinding(f)
	}

	// Calculate domains early so we can probe y-axis label widths and grow
	// MarginLeft before layout if labels would clip into the title/legend area.
	sc.config.ResolveValueFormatter(chartDataValues(data), true)

	xMin, xMax := sc.calculateXDomain(data)
	yMin, yMax := sc.calculateYDomain(data)
	EnsureYAxisFits(b, &sc.config.ChartConfig, yMin, yMax)

	// Calculate layout (shared across Cartesian chart types)
	layout := ComputeCartesianLayout(sc.config.ChartConfig, style, data.Title, data.Subtitle, data.Footnote, len(data.Series))
	plotArea := layout.PlotArea
	headerHeight := layout.HeaderHeight
	legendHeight := layout.LegendHeight

	// Refine legend height so multi-row legends aren't clipped.
	if sc.config.ShowLegend {
		RefineLegendHeightForced(b, style, data.Series, &plotArea, &legendHeight, sc.config.ForceLegendSingleSeries)
	}

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
	if sc.config.ShowLegend && (len(data.Series) > 1 || sc.config.ForceLegendSingleSeries) {
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

		legendBounds := Rect{
			X: plotArea.X,
			Y: plotArea.Y + plotArea.H + XAxisFooterHeight(style, sc.resolvedXAxisConfig()),
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
	DrawCartesianYAxis(b, plotArea, relYScale, sc.config.YAxisTitle, sc.config.ValueFmt)
}

// drawPoints draws the scatter points.
//
//nolint:gocognit,gocyclo // complex chart rendering logic
func (sc *ScatterChart) drawPoints(data ChartData, xScale, yScale *LinearScale, colors []Color) {
	b := sc.builder

	var pending []scatterLabel

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

		// Collect the labels; they are placed once, after every series is
		// drawn. Placing them per series meant the collision set was reset for
		// each one, so labels from different series were laid straight over
		// each other — six series of nine points produced a wall of overlapping
		// text with no finding to say so (go-slide-creator-daqp).
		for _, pt := range points {
			if pt.Label == "" {
				continue
			}
			pending = append(pending, scatterLabel{
				x:    xScale.Scale(pt.X),
				y:    yScale.Scale(pt.Y),
				text: pt.Label,
			})
		}
	}

	sc.drawPointLabels(pending)
}

// scatterLabel is one point's label and the position it belongs to.
type scatterLabel struct {
	x, y float64
	text string
}

const (
	// scatterMaxLabels is the point count past which labelling every point is
	// noise rather than information: the chart becomes a field of text with the
	// data underneath it. Above this the labels are dropped and reported.
	scatterMaxLabels = 15
	// scatterDenseLabels / scatterCrowdedLabels are the counts at which the
	// label font and length start shrinking.
	scatterDenseLabels   = 10
	scatterCrowdedLabels = 15
)

// drawPointLabels places every collected point label with one shared collision
// set, or reports why it placed none.
func (sc *ScatterChart) drawPointLabels(labels []scatterLabel) {
	if len(labels) == 0 {
		return
	}
	b := sc.builder
	style := b.StyleGuide()

	// Past the readable count, labelling every point buries the chart. The
	// caller can still ask for them explicitly.
	if len(labels) > scatterMaxLabels && !sc.config.ShowLabels {
		b.AddFinding(Finding{
			Field:    "series[].labels",
			Code:     FindingScatterLabelSkipped,
			Message:  fmt.Sprintf("%d point labels dropped — above %d the labels cover the plot they describe; label the points that matter, or set show_values to force all of them", len(labels), scatterMaxLabels),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"labelled_points": len(labels), "max_readable": scatterMaxLabels},
			},
		})
		return
	}

	minFont := b.MinFontSize()
	labelFontSize := math.Max(minFont, style.Typography.SizeSmall)
	maxLabelChars := 20
	switch {
	case len(labels) >= scatterCrowdedLabels:
		labelFontSize = minFont
		maxLabelChars = 15
	case len(labels) >= scatterDenseLabels:
		labelFontSize = math.Max(minFont, labelFontSize-0.5)
		maxLabelChars = 18
	}

	b.Push()
	defer b.Pop()
	b.SetFontSize(labelFontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextPrimary)

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

	skipped := 0
	for _, lab := range labels {
		px, py := lab.x, lab.y
		lbl := lab.text
		if len([]rune(lbl)) > maxLabelChars {
			lbl = string([]rune(lbl)[:maxLabelChars-1]) + "\u2026"
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

		placed := false
		for _, c := range candidates {
			if !overlaps(c.rect) {
				b.DrawText(lbl, c.x, c.y, c.align, c.base)
				placedLabels = append(placedLabels, c.rect)
				placed = true
				break
			}
		}
		if !placed {
			skipped++
		}
	}

	// One finding for the whole chart: a finding per skipped label flooded the
	// report and was then truncated, which told the agent nothing.
	if skipped > 0 {
		b.AddFinding(Finding{
			Field:    "series[].labels",
			Code:     FindingScatterLabelSkipped,
			Message:  fmt.Sprintf("%d of %d point labels skipped — every position around the point overlapped a label already placed", skipped, len(labels)),
			Severity: "info",
			Fix: &FixSuggestion{
				// A collision is a room problem: a wider canvas fits the same
				// labels. Dropping them for density is the other finding above,
				// and that one says to reduce.
				Kind:   FixKindIncreaseCanvas,
				Params: map[string]any{"skipped": skipped, "labelled_points": len(labels)},
			},
		})
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
//
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
	labelConfig := ArcSeriesConfig{LabelFormat: pc.config.LabelFormat}
	if spec := pc.config.ValueFormatSpec; !spec.IsZero() {
		labelValues := values
		labelConfig.LabelValuesAreRaw = !strings.EqualFold(strings.TrimSpace(spec.Style), "percent")
		if !labelConfig.LabelValuesAreRaw && total > 0 {
			labelValues = make([]float64, len(values))
			for i, value := range values {
				labelValues[i] = value / total
			}
		}
		labelConfig.ValueFmt = NewValueFormatter(spec, labelValues, "", false)
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

		// Centre the legend in the plot area, but never above it: the plot
		// area already excludes the title band, and a legend taller than the
		// plot area made (plotArea.H-measuredLegendH)/2 negative, lifting the
		// first rows into the title. On a large-font template the chart title
		// was drawn straight across them (go-slide-creator-p142).
		legendBounds = Rect{
			X: plotArea.X + pieW + style.Spacing.MD,
			Y: plotArea.Y + math.Max(0, (plotArea.H-measuredLegendH)/2),
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
				pctStr := labelConfig.formatSliceValue(v, total)
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
	arcConfig.ValueFmt = labelConfig.ValueFmt
	arcConfig.LabelValuesAreRaw = labelConfig.LabelValuesAreRaw
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

	// Draw title. With the legend on the right the title is centred over the
	// PIE, not over the whole canvas: a centred full-width title runs into the
	// legend column, and the two bands sit only a line apart, so a long title
	// was drawn across the first legend row (go-slide-creator-p142).
	if pc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		titleW := pc.config.Width
		if landscapeLegend {
			titleW = legendBounds.X
		}
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: titleW, H: headerHeight + pc.config.MarginTop})
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
//
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
	if rc.config.ShowLegend && (len(data.Series) > 1 || rc.config.ForceLegendSingleSeries) {
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
		minRadius := maxHalfDim * 0.40                                               // don't shrink below 40%
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
	if rc.config.ShowLegend && (len(data.Series) > 1 || rc.config.ForceLegendSingleSeries) {
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
		b.SetStrokeColor(MustParseColor(DefaultThemeLT1Hex))
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

// resolvedXAxisConfig returns the x-axis configuration drawAxes drew with,
// falling back to the bottom-axis default when the axes were suppressed.
func (bc *BarChart) resolvedXAxisConfig() AxisConfig {
	// TickCount 0 means drawAxes never ran (AxisPosition's zero value is a
	// legitimate position, so it cannot signal "unset").
	if bc.xAxisCfg.TickCount == 0 {
		cfg := DefaultAxisConfig(AxisPositionBottom)
		cfg.Title = bc.config.XAxisTitle
		return cfg
	}
	return bc.xAxisCfg
}

// resolvedXAxisConfig returns the x-axis configuration drawAxes drew with,
// falling back to the bottom-axis default when the axes were suppressed.
func (lc *LineChart) resolvedXAxisConfig() AxisConfig {
	if lc.xAxisCfg.TickCount == 0 {
		cfg := DefaultAxisConfig(AxisPositionBottom)
		cfg.Title = lc.config.XAxisTitle
		return cfg
	}
	return lc.xAxisCfg
}

// resolvedXAxisConfig returns the x-axis configuration drawAxes drew with,
// falling back to the bottom-axis default when the axes were suppressed.
func (sc *ScatterChart) resolvedXAxisConfig() AxisConfig {
	if sc.xAxisCfg.TickCount == 0 {
		cfg := DefaultAxisConfig(AxisPositionBottom)
		cfg.Title = sc.config.XAxisTitle
		return cfg
	}
	return sc.xAxisCfg
}
