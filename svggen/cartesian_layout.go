package svggen

import "math"

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
	if config.ShowLegend && seriesCount > 1 {
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
func RefineLegendHeight(b *SVGBuilder, style *StyleGuide, series []ChartSeries, plotArea *Rect, legendHeight *float64) {
	if len(series) <= 1 {
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
