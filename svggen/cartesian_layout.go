package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Cartesian Layout — shared layout computation for Cartesian chart types
// =============================================================================
//
// Eliminates duplicated headerHeight/footerHeight/legendHeight/plotArea
// calculations from BarChart, LineChart, ScatterChart, and WaterfallChart.

// CartesianLayout holds pre-computed layout regions for a Cartesian chart.
type CartesianLayout struct {
	// PlotArea is the inner area where data is drawn, after accounting for
	// title, subtitle, footnote, and legend.
	PlotArea Rect

	// HeaderHeight is the space reserved for title + subtitle at the top.
	HeaderHeight float64

	// FooterHeight is the space reserved for the footnote at the bottom.
	FooterHeight float64

	// LegendHeight is the space reserved for the legend.
	LegendHeight float64
}

// ComputeCartesianLayout calculates standard layout dimensions from a chart
// config, style guide, and content metadata. This replaces the 4 duplicated
// layout blocks across BarChart.Draw, LineChart.Draw, ScatterChart.Draw,
// and WaterfallChart.Draw.
func ComputeCartesianLayout(config ChartConfig, style *StyleGuide, title, subtitle, footnote string, seriesCount int) CartesianLayout {
	plotArea := config.PlotArea()

	// Header: title + optional subtitle
	headerHeight := 0.0
	if config.ShowTitle && title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Footer: footnote — reserve enough space for SizeCaption + padding.
	footerHeight := 0.0
	if footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}

	// Legend
	legendHeight := 0.0
	if config.ShowLegend && (seriesCount > 1 || config.ForceLegendSingleSeries) {
		legendHeight = style.Typography.SizeSmall + style.Spacing.LG
	}

	// Adjust plot area
	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight
	if config.LegendPosition == LegendPositionBottom {
		plotArea.H -= legendHeight
	}

	return CartesianLayout{
		PlotArea:     plotArea,
		HeaderHeight: headerHeight,
		FooterHeight: footerHeight,
		LegendHeight: legendHeight,
	}
}

// RefineLegendHeight recomputes the legend height using the actual presentation
// legend config and series names, then adjusts the plotArea and legendHeight if
// the real height exceeds the initial estimate. This prevents multi-row legends
// (caused by large PresentationLegendConfig fonts and TextWidthFactor) from
// being clipped by the SVG viewBox.
//
// Single-series charts are skipped by default since they don't render a
// legend. Callers that have opted into a single-series legend via
// ChartConfig.ForceLegendSingleSeries should use RefineLegendHeightForced.
func RefineLegendHeight(b *SVGBuilder, style *StyleGuide, series []ChartSeries, plotArea *Rect, legendHeight *float64) {
	refineLegendHeight(b, style, series, plotArea, legendHeight, false)
}

// RefineLegendHeightForced is the force-aware variant of RefineLegendHeight.
// When force is true, the single-series early return is skipped so the
// legend height reflects the real measurement even with one series.
func RefineLegendHeightForced(b *SVGBuilder, style *StyleGuide, series []ChartSeries, plotArea *Rect, legendHeight *float64, force bool) {
	refineLegendHeight(b, style, series, plotArea, legendHeight, force)
}

func refineLegendHeight(b *SVGBuilder, style *StyleGuide, series []ChartSeries, plotArea *Rect, legendHeight *float64, force bool) {
	if len(series) <= 1 && !force {
		return
	}
	if len(series) == 0 {
		return
	}
	legendConfig := PresentationLegendConfig(style)
	legend := NewLegend(b, legendConfig)
	items := make([]LegendItem, len(series))
	for i, s := range series {
		items[i] = LegendItem{Label: s.Name}
	}
	legend.SetItems(items)
	actual := legend.Height(plotArea.W)
	if actual > *legendHeight {
		diff := actual - *legendHeight
		plotArea.H -= diff
		*legendHeight = actual
	}
}

// =============================================================================
// Shared Grid Drawing
// =============================================================================

// DrawCartesianGridWithVerticals draws the standard horizontal y-axis grid
// (via DrawCartesianGrid) and, when showVertical is true, additionally draws
// per-category vertical gridlines for charts whose x axis is a
// CategoricalScale. Combining the two paths keeps the per-chart Draw method
// at a single grid call site (and a single cyclomatic branch).
func DrawCartesianGridWithVerticals(b *SVGBuilder, plotArea Rect, yScale *LinearScale, xScale Scale, showVertical bool) {
	DrawCartesianGrid(b, plotArea, yScale, nil)
	if showVertical {
		DrawCategoricalVerticalGrid(b, plotArea, xScale)
	}
}

// DrawCategoricalVerticalGrid draws vertical gridlines at category positions on
// a Cartesian chart whose x axis is a CategoricalScale (bar/line/area). Used
// to honour ChartConfig.ShowVerticalGrid when the executive token default
// (horizontal-only) has been overridden via chart_style.show_vertical_gridlines.
//
// The function quietly no-ops on non-CategoricalScale inputs so callers can
// pass the chart's existing Scale without type-switching at the call site.
func DrawCategoricalVerticalGrid(b *SVGBuilder, plotArea Rect, xScale Scale) {
	cat, ok := xScale.(*CategoricalScale)
	if !ok || cat == nil {
		return
	}

	gridConfig := DefaultGridConfig()

	b.Push()
	b.SetStrokeColor(gridConfig.Color)
	b.SetStrokeWidth(gridConfig.StrokeWidth)
	b.SetDashes(gridConfig.DashPattern...)

	// Place one vertical line at the center of each category band so the
	// gridline visually anchors the corresponding bar/marker. Skip lines
	// that fall outside the plot rect (e.g. when the scale has padding
	// that pushes the first/last band beyond plotArea).
	for _, label := range cat.Categories() {
		x := plotArea.X + cat.Scale(label)
		if x < plotArea.X-0.5 || x > plotArea.X+plotArea.W+0.5 {
			continue
		}
		b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)
	}

	b.Pop()
}

// DrawCartesianGrid draws horizontal grid lines (and optionally vertical) for
// Cartesian charts. This replaces the duplicated drawGrid methods on BarChart,
// LineChart, ScatterChart, and WaterfallChart.
//
// For charts with only a y-axis grid (bar, line, waterfall), pass xScale=nil.
// For scatter charts with a 2D grid, pass both scales.
func DrawCartesianGrid(b *SVGBuilder, plotArea Rect, yScale *LinearScale, xScale *LinearScale) {
	gridConfig := DefaultGridConfig()

	b.Push()
	b.SetStrokeColor(gridConfig.Color)
	b.SetStrokeWidth(gridConfig.StrokeWidth)
	b.SetDashes(gridConfig.DashPattern...)

	// Horizontal grid lines from y-axis ticks
	if yScale != nil {
		yTicks := yScale.Ticks(5)
		for _, v := range yTicks {
			y := plotArea.Y + yScale.Scale(v)
			if y < plotArea.Y-0.5 || y > plotArea.Y+plotArea.H+0.5 {
				continue // skip out-of-bounds ticks
			}
			b.DrawLine(plotArea.X, y, plotArea.X+plotArea.W, y)
		}
	}

	// Vertical grid lines from x-axis ticks (scatter charts)
	if xScale != nil {
		xTicks := xScale.Ticks(5)
		for _, v := range xTicks {
			x := xScale.Scale(v)
			if x < plotArea.X-0.5 || x > plotArea.X+plotArea.W+0.5 {
				continue // skip out-of-bounds ticks
			}
			b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)
		}
	}

	b.Pop()
}

// =============================================================================
// Shared Y-Axis Drawing
// =============================================================================

// DrawCartesianYAxis draws the y-axis for a Cartesian chart using the standard
// left-side linear axis configuration. This is the common y-axis pattern shared
// by BarChart, LineChart, ScatterChart, and WaterfallChart.
func DrawCartesianYAxis(b *SVGBuilder, plotArea Rect, yScale *LinearScale, title string) {
	yAxisConfig := DefaultAxisConfig(AxisPositionLeft)
	yAxisConfig.Title = title
	yAxisConfig.RangeExtent = plotArea.H
	yAxis := NewAxis(b, yAxisConfig)
	yAxis.DrawLinearAxis(yScale, plotArea.X, plotArea.Y)
}

// =============================================================================
// Adaptive Y-Axis Label Layout
// =============================================================================

// AdaptYLabelsResult holds the result of measuring linear-scale y-axis labels
// and deciding whether MarginLeft must grow to fit them.
type AdaptYLabelsResult struct {
	// WidestLabel is the measured width of the widest formatted tick label,
	// including a 1.1x safety factor for inter-character spacing variance
	// (same factor used by AdaptXLabels).
	WidestLabel float64

	// ExtraLeftMargin is the additional MarginLeft needed so the widest label
	// fits in the area to the left of tick marks. Zero when the existing
	// MarginLeft already accommodates the labels.
	ExtraLeftMargin float64

	// Clipped is true when the existing MarginLeft was insufficient — caller
	// should emit a chart.label_clipped finding.
	Clipped bool
}

// AdaptYLabels measures the widest formatted y-axis tick label for a linear
// scale and reports how much MarginLeft must grow to keep the label fully
// inside the chart viewBox. This mirrors AdaptXLabels.ExtraBottomMargin for
// the bottom axis.
//
// Label formatting replicates DrawLinearAxis exactly: TickFormat is used by
// default, switching to FormatCompact when any tick magnitude exceeds 9999.
//
// Parameters:
//   - b: SVGBuilder for text measurement.
//   - domainMin, domainMax: y-axis domain (used to generate ticks).
//   - tickCount: requested number of ticks (typically 5).
//   - fontSize: label font size (typically style.Typography.SizeSmall).
//   - tickSize: tick mark length (typically 6pt).
//   - tickPadding: label-to-tick gap (drawTick floors this to 3pt).
//   - hasTitle: true if an axis title is also drawn (consumes additional
//     horizontal space because the title is rotated 90°).
//   - titleFontSize: axis-title font size (only used when hasTitle is true).
//   - marginLeft: current MarginLeft to check against.
func AdaptYLabels(b *SVGBuilder, domainMin, domainMax float64, tickCount int, fontSize, tickSize, tickPadding float64, hasTitle bool, titleFontSize, marginLeft float64) AdaptYLabelsResult {
	if tickCount <= 0 {
		tickCount = 5
	}
	scale := NewLinearScale(domainMin, domainMax)
	ticks := scale.Ticks(tickCount)
	if len(ticks) == 0 {
		return AdaptYLabelsResult{}
	}

	// Choose format and compact mode the same way DrawLinearAxis does so the
	// measured labels match what is actually rendered.
	format := scale.TickFormat(tickCount)
	useCompact := false
	for _, v := range ticks {
		if v > 9999 || v < -9999 {
			useCompact = true
			break
		}
	}

	labels := make([]string, len(ticks))
	for i, v := range ticks {
		if useCompact {
			labels[i] = FormatCompact(v)
		} else {
			labels[i] = fmt.Sprintf(format, v)
		}
	}

	// Measure widest with 1.1x safety factor (matches AdaptXLabels).
	b.Push()
	b.SetFontSize(fontSize)
	var maxW float64
	for _, label := range labels {
		w, _ := b.MeasureText(label)
		if w > maxW {
			maxW = w
		}
	}
	b.Pop()
	widest := maxW * 1.1

	// Required left space: rotated title strip (when present) + label width +
	// tick mark + min label-to-tick gap (mirrors drawTick's verticalLabelGap).
	verticalLabelGap := math.Max(tickPadding, 3)
	required := widest + tickSize + verticalLabelGap
	if hasTitle {
		// The vertical axis title is rotated 90° and centered along the axis,
		// so its horizontal footprint is ~one line of titleFontSize, plus a
		// small gap between title and label column.
		required += titleFontSize + verticalLabelGap
	}

	// 1pt of viewBox slack so the widest label edge isn't flush with the SVG
	// boundary (matches the strict no-overlap unit-test budget).
	required += 1

	res := AdaptYLabelsResult{WidestLabel: widest}
	if required > marginLeft {
		res.ExtraLeftMargin = required - marginLeft
		res.Clipped = true
	}
	return res
}

// EnsureYAxisFits measures linear-scale y-axis labels and, if necessary, grows
// config.MarginLeft so the widest label is fully visible. When an adjustment
// is made, a chart.label_clipped finding is emitted with the widest measured
// label width and the applied extra margin.
//
// Callers should invoke this BEFORE ComputeCartesianLayout so the layout
// reflects the adjusted margins. Charts that use a non-linear y-axis (e.g.
// log scale) should skip this call — DrawLogAxis uses different label
// formatting that isn't covered here.
func EnsureYAxisFits(b *SVGBuilder, config *ChartConfig, domainMin, domainMax float64) AdaptYLabelsResult {
	style := b.StyleGuide()
	fontSize := style.Typography.SizeSmall
	res := AdaptYLabels(
		b,
		domainMin, domainMax,
		5,
		fontSize,
		6, // matches DefaultAxisConfig.TickSize
		4, // matches DefaultAxisConfig.TickPadding
		config.YAxisTitle != "",
		style.Typography.SizeBody,
		config.MarginLeft,
	)
	if res.ExtraLeftMargin > 0 {
		config.MarginLeft += res.ExtraLeftMargin
		b.AddFinding(Finding{
			Field: "y_axis.labels",
			Code:  FindingLabelClipped,
			Message: fmt.Sprintf(
				"y-axis label width %.1fpt exceeded available MarginLeft — grew left margin by %.1fpt",
				res.WidestLabel, res.ExtraLeftMargin,
			),
			Severity: "info",
			Fix: &FixSuggestion{
				Kind: FixKindIncreaseCanvas,
				Params: map[string]any{
					"axis":                 "y",
					"widest_label_pt":      res.WidestLabel,
					"extra_left_margin_pt": res.ExtraLeftMargin,
				},
			},
		})
	}
	return res
}

// =============================================================================
// Shared Label Step Computation
// =============================================================================

// ComputeLabelStep determines how many x-axis labels to skip for dense
// categorical charts. Returns 1 (show all), 2 (every other), etc.
// Always show first and last labels.
//
// This replaces the duplicated label-thinning logic in BarChart.Draw and
// WaterfallChart.Draw.
func ComputeLabelStep(numCategories int) int {
	switch {
	case numCategories >= 25:
		return (numCategories + 7) / 8 // ~8 labels max
	case numCategories >= 20:
		return (numCategories + 9) / 10 // ~10 labels max
	case numCategories >= 15:
		return 2
	default:
		return 1
	}
}

// =============================================================================
// Adaptive X-Axis Label Layout
// =============================================================================

// XLabelLayout holds the computed x-axis label parameters after adaptive sizing.
type XLabelLayout struct {
	// FontSize is the final font size for x-axis labels.
	FontSize float64

	// Rotation is the label rotation angle in degrees (0 or negative).
	Rotation float64

	// LabelStep is the thinning factor (1 = show all, 2 = every other, etc.).
	LabelStep int

	// Categories holds the (potentially truncated) category labels.
	Categories []string

	// ExtraBottomMargin is the additional bottom margin needed for rotated labels.
	ExtraBottomMargin float64
}

// AdaptXLabels computes adaptive font size, rotation, thinning, and truncation
// for x-axis category labels to prevent overlap and truncation.
//
// Strategy (applied in order):
//  1. Shrink font size toward a 9pt floor.
//  2. Rotate labels 45 degrees when they still exceed available space.
//  3. Thin labels (show every Nth) if rotated labels still overlap.
//  4. Truncate with ellipsis as a last resort.
//
// Parameters:
//   - b: SVGBuilder for text measurement
//   - categories: the raw category strings (will be copied, not mutated)
//   - plotWidth: the available horizontal space for all categories
//   - baseFontSize: the starting font size (typically style.Typography.SizeSmall)
//   - isNarrow: true when the chart is narrow (width < 500pt)
//nolint:gocognit,gocyclo // complex chart rendering logic
func AdaptXLabels(b *SVGBuilder, categories []string, plotWidth, baseFontSize float64, isNarrow bool) XLabelLayout {
	numCats := len(categories)
	if numCats == 0 {
		return XLabelLayout{FontSize: baseFontSize, LabelStep: 1}
	}

	// Work on a copy so we don't mutate the caller's slice.
	cats := make([]string, numCats)
	copy(cats, categories)

	bandwidth := plotWidth / float64(numCats)
	fontSize := baseFontSize
	rotation := 0.0
	labelStep := 1
	extraBottom := 0.0

	const fontFloor = 9.0 // Minimum readable font size

	// measureMaxLabel returns the widest label at the given font size,
	// including a 1.1x safety factor for inter-character spacing variance.
	// Reduced from 1.5→1.2→1.1: MeasureText uses real font metrics via the
	// canvas library, so only a small margin is needed. The previous 1.2x
	// factor forced unnecessary rotation on moderate-density charts (e.g.,
	// 6-label waterfall at half-width where "Downgrades" measured 56pt in
	// a 68pt bandwidth).
	measureMaxLabel := func(fs float64) float64 {
		b.Push()
		b.SetFontSize(fs)
		var maxW float64
		for _, cat := range cats {
			w, _ := b.MeasureText(cat)
			if w > maxW {
				maxW = w
			}
		}
		b.Pop()
		return maxW * 1.1
	}

	maxLabelWidth := measureMaxLabel(fontSize)

	// ── Step 1: Shrink font toward 9pt floor ──
	if maxLabelWidth > bandwidth*0.95 && fontSize > fontFloor {
		// Try progressively smaller font sizes down to the floor.
		for _, candidate := range []float64{fontSize * 0.9, fontSize * 0.8, fontFloor} {
			candidate = math.Max(fontFloor, candidate)
			w := measureMaxLabel(candidate)
			fontSize = candidate
			maxLabelWidth = w
			if w <= bandwidth*0.95 {
				break
			}
		}
	}

	// ── Step 2: Rotate 45 degrees if still overflowing ──
	if maxLabelWidth > bandwidth*0.95 {
		rotation = -45

		// Use steeper angle on narrow charts or very dense charts.
		// cos(-60°) = 0.5 vs cos(-45°) = 0.707, giving ~30% more
		// horizontal room per label — critical in slot2 of two-column layouts.
		if numCats >= 15 || isNarrow {
			rotation = -60
		}

		maxLabelWidth = measureMaxLabel(fontSize)

		rotAngleRad := math.Abs(rotation) * math.Pi / 180
		horizFootprint := maxLabelWidth * math.Cos(rotAngleRad)

		// ── Step 3: Thin labels if rotated labels still overlap ──
		if horizFootprint > bandwidth*0.95 {
			labelStep = ComputeLabelStep(numCats)
			// Also try further thinning for narrow charts
			if isNarrow && labelStep < 2 {
				labelStep = 2
			}
		}

		// After thinning, re-check: if thinned labels still overlap, truncate.
		effectiveBandwidth := bandwidth * float64(labelStep)
		if horizFootprint > effectiveBandwidth*0.95 {
			// ── Step 4: Truncate with ellipsis as last resort ──
			targetW := effectiveBandwidth * 0.90 / math.Cos(rotAngleRad)
			charW := fontSize * 0.6
			maxChars := int(targetW / charW)
			if maxChars < 3 {
				maxChars = 3
			}
			for i, cat := range cats {
				runes := []rune(cat)
				if len(runes) > maxChars {
					cats[i] = string(runes[:maxChars-1]) + "\u2026"
				}
			}
			maxLabelWidth = measureMaxLabel(fontSize)
		}

		// Add extra bottom margin for rotated labels.
		rotatedHeight := maxLabelWidth * math.Sin(rotAngleRad)
		eb := rotatedHeight - fontSize
		if eb > 0 {
			extraBottom = eb
		}
	}

	return XLabelLayout{
		FontSize:          fontSize,
		Rotation:          rotation,
		LabelStep:         labelStep,
		Categories:        cats,
		ExtraBottomMargin: extraBottom,
	}
}
