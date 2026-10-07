package svggen

import (
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// =============================================================================
// Small Multiples (coordinated line-chart facets)
// =============================================================================
//
// A QBR comparing revenue for four regions over the same quarters wants one
// small chart per region — four overlapping lines hide each trend — and the
// audience must still compare absolute magnitudes across the panels. Four
// independent line charts placed side by side cannot do that: each computes
// its own y scale, so a region ten times larger draws the identical line
// (go-slide-creator-sxpvy).
//
// small_multiples_chart draws 2–6 line panels from one categories list and one
// named series per panel, in one canvas, with:
//
//   - one x scale (the shared categories) and, by default, ONE y domain
//     computed from every panel's values, so equal values land at identical
//     positions on equal plots and a 10x panel draws visibly taller;
//   - identical plot sizes and a common left gutter, so panel titles of any
//     length cannot shift one plot against another;
//   - one tick precision for every panel's value labels;
//   - data.y_scale "independent" as the explicit opt-out, which gives each
//     panel its own y axis and labels the chart so (the comparison of levels
//     is then invalid, and the reader is told);
//   - a grid chosen to keep every panel above the readability floors (minimum
//     plot height for three ticks, category labels that fit their band, panel
//     titles that fit their plot); when no grid does, the chart reports
//     chart.plot_area_collapsed with action shrink_or_split and how many
//     panels per slide would fit.

const (
	// SmallMultiplesChartType is the registered svggen type ID.
	SmallMultiplesChartType = "small_multiples_chart"

	// SmallMultiplesMinPanels and SmallMultiplesMaxPanels bound the panel
	// count. One panel is a line_chart; past six the panels are too small to
	// read on a slide and belong on two slides.
	SmallMultiplesMinPanels = 2
	SmallMultiplesMaxPanels = 6

	// YScaleShared (the default) gives every panel the same y domain.
	YScaleShared = "shared"
	// YScaleIndependent gives each panel its own y domain; the chart is
	// labelled so the reader does not compare levels across panels.
	YScaleIndependent = "independent"

	// SmallMultiplesIndependentNote is the visible label an independent-scale
	// chart carries under its title.
	SmallMultiplesIndependentNote = "Independent y-axes: each panel has its own scale; compare shapes, not levels"

	// smallMultiplesMinPlotLines is the smallest plot height, in tick-label
	// font sizes, that holds three y ticks 1.6 lines apart (yTickCountForHeight).
	smallMultiplesMinPlotLines = 4.8

	// smallMultiplesPlotAspect weights plot height against width when
	// scoring grids: a panel 1.6x as wide as tall reads a trend well.
	smallMultiplesPlotAspect = 1.6

	// smallMultiplesMarkerMaxPoints is the most categories a panel draws
	// point markers on.
	smallMultiplesMarkerMaxPoints = 12
)

// SmallMultiplesDiagram implements the Diagram interface for coordinated
// small-multiple line charts.
type SmallMultiplesDiagram struct{ BaseDiagram }

// Validate checks the categories, the 2–6 named panels and data.y_scale.
func (d *SmallMultiplesDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data
	if raw, ok := data["series"]; ok {
		if panels, ok := toSeriesSlice(raw); ok {
			if len(panels) < SmallMultiplesMinPanels {
				return &ValidationError{Field: "data.series", Code: ErrCodeConstraint, Value: raw,
					Message: fmt.Sprintf("%s needs %d–%d panels (one named series each), got %d; a single series is a line_chart", SmallMultiplesChartType, SmallMultiplesMinPanels, SmallMultiplesMaxPanels, len(panels))}
			}
			if len(panels) > SmallMultiplesMaxPanels {
				return &ValidationError{Field: "data.series", Code: ErrCodeConstraint, Value: len(panels),
					Message: fmt.Sprintf("%s takes at most %d panels, got %d: panels past %d fall below the readability floors on a slide — split them across slides of %d or fewer", SmallMultiplesChartType, SmallMultiplesMaxPanels, len(panels), SmallMultiplesMaxPanels, SmallMultiplesMaxPanels)}
			}
			for i, p := range panels {
				if name, _ := p["name"].(string); strings.TrimSpace(name) == "" {
					return &ValidationError{Field: fmt.Sprintf("data.series[%d].name", i), Code: ErrCodeRequired,
						Message: fmt.Sprintf("%s panel %d needs a name: it is the panel title", SmallMultiplesChartType, i)}
				}
				if _, timed := p["time_strings"]; timed {
					return &ValidationError{Field: fmt.Sprintf("data.series[%d].time_strings", i), Code: ErrCodeInvalidValue,
						Message: SmallMultiplesChartType + " panels share data.categories; time_strings / time_values are not supported"}
				}
				if _, timed := p["time_values"]; timed {
					return &ValidationError{Field: fmt.Sprintf("data.series[%d].time_values", i), Code: ErrCodeInvalidValue,
						Message: SmallMultiplesChartType + " panels share data.categories; time_strings / time_values are not supported"}
				}
			}
		}
	}
	if err := validateCategoriesAndSeries(data, SmallMultiplesChartType, true, SmallMultiplesMinPanels); err != nil {
		return err
	}
	if _, err := resolveYScaleMode(data); err != nil {
		return err
	}
	if _, set, err := resolveSeriesHighlight(data, panelNamesAsSeries(data)); set && err != nil {
		return &ValidationError{Field: "data.highlight", Code: ErrCodeInvalidValue, Value: data["highlight"], Message: err.Error() + " (name a panel by its series name or 0-based index)"}
	}
	return nil
}

// panelNamesAsSeries returns the panels as name-only series, for resolving
// data.highlight at validation time.
func panelNamesAsSeries(data map[string]any) []ChartSeries {
	panels, _ := toSeriesSlice(data["series"])
	out := make([]ChartSeries, len(panels))
	for i, p := range panels {
		out[i].Name, _ = p["name"].(string)
	}
	return out
}

// resolveYScaleMode reads data.y_scale: "shared" (default) or "independent".
func resolveYScaleMode(data map[string]any) (string, error) {
	raw, ok := data["y_scale"]
	if !ok || raw == nil {
		return YScaleShared, nil
	}
	s, _ := raw.(string)
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", YScaleShared:
		return YScaleShared, nil
	case YScaleIndependent:
		return YScaleIndependent, nil
	}
	return "", &ValidationError{Field: "data.y_scale", Code: ErrCodeInvalidValue, Value: raw,
		Message: fmt.Sprintf("data.y_scale must be %q (default: one y domain for every panel) or %q (each panel its own axis, labelled on the chart)", YScaleShared, YScaleIndependent)}
}

// Render generates an SVG document for the small multiples.
func (d *SmallMultiplesDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *SmallMultiplesDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		chartData, err := extractChartData(req)
		if err != nil {
			return err
		}
		mode, err := resolveYScaleMode(req.Data)
		if err != nil {
			return err
		}
		if len(chartData.Series) == 0 || len(chartData.Categories) == 0 {
			return fmt.Errorf("%s render failed: categories and series are required", SmallMultiplesChartType)
		}
		sm := &smallMultiples{
			b:         builder,
			data:      chartData,
			mode:      mode,
			colors:    extractChartColorsForRequest(req, builder),
			config:    DefaultChartConfig(builder.Width(), builder.Height()),
			showTitle: req.Title != "",
		}
		sm.config.ValueFormatSpec = req.Style.ValueFormat
		return sm.draw()
	})
}

// smallMultiples holds one render.
type smallMultiples struct {
	b         *SVGBuilder
	data      ChartData
	mode      string
	colors    []Color
	config    ChartConfig
	showTitle bool
}

// smGrid is one candidate panel arrangement and the geometry it yields.
type smGrid struct {
	cols, rows     int
	plotW, plotH   float64
	colGap, rowGap float64
	// failures lists the readability floors this grid misses.
	failures []string
}

func (g smGrid) score() float64 {
	return math.Min(g.plotW, smallMultiplesPlotAspect*g.plotH)
}

// smMetrics are the canvas-wide measurements every grid shares.
type smMetrics struct {
	gridLeft, gridTop    float64
	gridW, gridH         float64
	yGutter              float64
	panelTitleH, xLabelH float64
	labelFS, titleFS     float64
	minPlotH             float64
	widestCategory       float64
	panelTitleWidths     []float64
	headerH, noteH       float64
	topPad               float64
	footerH              float64
	domains              [][2]float64
	tickDecimals         int
}

// panelDomains returns the y domain of every panel: one domain computed from
// all values in shared mode (so it is computed once and applied everywhere),
// each panel's own in independent mode. Constant and negative data use the
// line chart's domain rules.
func panelDomains(data ChartData, mode string) [][2]float64 {
	lc := &LineChart{config: DefaultLineChartConfig(1, 1)}
	out := make([][2]float64, len(data.Series))
	if mode != YScaleIndependent {
		lo, hi := lc.calculateYDomain(ChartData{Series: data.Series})
		for i := range out {
			out[i] = [2]float64{lo, hi}
		}
		return out
	}
	for i, s := range data.Series {
		lo, hi := lc.calculateYDomain(ChartData{Series: []ChartSeries{s}})
		out[i] = [2]float64{lo, hi}
	}
	return out
}

// niceScale returns the panel y scale for a domain: the same Nice rounding
// the line chart applies.
func niceScale(domain [2]float64) *LinearScale {
	return NewLinearScale(domain[0], domain[1]).Nice(true)
}

// measure computes the shared metrics.
func (sm *smallMultiples) measure() smMetrics {
	b := sm.b
	style := b.StyleGuide()
	typ := style.Typography
	m := smMetrics{labelFS: typ.SizeSmall, titleFS: typ.SizeHeading}
	if m.titleFS <= 0 {
		m.titleFS = typ.SizeBody
	}
	m.minPlotH = smallMultiplesMinPlotLines * m.labelFS

	m.headerH = chartHeaderHeight(style, sm.showTitle, sm.data.Title, sm.data.Subtitle)
	if sm.mode == YScaleIndependent {
		m.noteH = typ.SizeCaption*1.4 + style.Spacing.SM
	}
	if sm.data.Footnote != "" {
		m.footerH = FootnoteReservedHeight(style)
	}

	m.domains = panelDomains(sm.data, sm.mode)
	sm.config.ResolveValueFormatter(chartDataValues(sm.data), true)

	// One tick precision for every panel: the finest any panel's tick step
	// needs, measured over every tick count a panel may use.
	b.Push()
	b.SetFontSize(m.labelFS)
	for _, d := range m.domains {
		s := niceScale(d)
		for tc := 2; tc <= 5; tc++ {
			if dec := printfDecimals(s.TickFormat(tc)); dec > m.tickDecimals {
				m.tickDecimals = dec
			}
		}
	}
	vf := sm.tickFormatter(m.tickDecimals)
	widestTick := 0.0
	for _, d := range m.domains {
		s := niceScale(d)
		lo, hi := s.DomainBounds()
		for tc := 2; tc <= 5; tc++ {
			for _, v := range s.Ticks(tc) {
				if w, _ := b.MeasureText(formatSMTick(vf, v, hi-lo)); w > widestTick {
					widestTick = w
				}
			}
		}
	}
	for _, c := range sm.data.Categories {
		if w, _ := b.MeasureText(c); w > m.widestCategory {
			m.widestCategory = w
		}
	}
	b.Pop()

	b.Push()
	b.SetFontSize(m.titleFS).SetFontWeight(typ.WeightBold)
	m.panelTitleWidths = make([]float64, len(sm.data.Series))
	for i, s := range sm.data.Series {
		m.panelTitleWidths[i], _ = b.MeasureText(s.Name)
	}
	b.Pop()

	const tickGap = 4.0
	m.yGutter = widestTick*1.1 + tickGap + 1
	// The top y tick label is centred on the plot's top edge, so half a
	// label line sits above it: the panel title clears that too.
	m.panelTitleH = m.titleFS*1.3 + 0.6*m.labelFS + style.Spacing.XS
	m.xLabelH = tickGap + xLabelGlyphEm*m.labelFS

	m.gridLeft = math.Max(style.Spacing.XS, 2) + m.yGutter
	// The panels carry their own titles, so the canvas keeps only a small
	// top pad above the chart title (or the first panel row).
	m.topPad = math.Min(sm.config.MarginTop, style.Spacing.MD)
	m.gridTop = m.topPad + m.headerH + m.noteH
	right := math.Min(sm.config.MarginRight, style.Spacing.LG)
	m.gridW = sm.config.Width - m.gridLeft - right
	m.gridH = sm.config.Height - m.gridTop - m.footerH - style.Spacing.SM
	return m
}

// tickFormatter is the chart's value formatter at the common tick precision.
func (sm *smallMultiples) tickFormatter(decimals int) *ValueFormatter {
	return sm.config.ValueFmt.WithDecimals(decimals)
}

// grids lists every arrangement of n panels without an empty row, scored and
// checked against the readability floors.
func (sm *smallMultiples) grids(m smMetrics, n int, titleWidths []float64) []smGrid {
	style := sm.b.StyleGuide()
	var out []smGrid
	for cols := 1; cols <= n; cols++ {
		rows := (n + cols - 1) / cols
		if rows*cols-n >= cols {
			continue
		}
		g := smGrid{cols: cols, rows: rows, colGap: style.Spacing.MD, rowGap: style.Spacing.MD}
		if sm.mode == YScaleIndependent {
			g.colGap += m.yGutter
		}
		g.plotW = (m.gridW - float64(cols-1)*g.colGap) / float64(cols)
		rowH := (m.gridH - float64(rows-1)*g.rowGap) / float64(rows)
		g.plotH = rowH - m.panelTitleH - m.xLabelH
		if g.plotH < m.minPlotH {
			g.failures = append(g.failures, fmt.Sprintf("plot height %.0fpt is under the %.0fpt a readable value axis needs", math.Max(0, g.plotH), m.minPlotH))
		}
		if band := g.plotW / float64(len(sm.data.Categories)); m.widestCategory*1.1+style.Spacing.XS > band {
			g.failures = append(g.failures, fmt.Sprintf("category labels need %.0fpt each but a panel gives them %.0fpt", m.widestCategory*1.1+style.Spacing.XS, math.Max(0, band)))
		}
		for i, w := range titleWidths {
			if w > g.plotW {
				g.failures = append(g.failures, fmt.Sprintf("panel title %q needs %.0fpt but a panel is %.0fpt wide", sm.data.Series[i].Name, w, math.Max(0, g.plotW)))
				break
			}
		}
		out = append(out, g)
	}
	return out
}

// chooseGrid returns the best readable grid, or the best grid overall with
// ok=false when no arrangement meets the floors.
func chooseGrid(grids []smGrid) (smGrid, bool) {
	var best, bestAny smGrid
	found, foundAny := false, false
	for _, g := range grids {
		if !foundAny || g.score() > bestAny.score() {
			bestAny, foundAny = g, true
		}
		if len(g.failures) == 0 && (!found || g.score() > best.score()) {
			best, found = g, true
		}
	}
	if found {
		return best, true
	}
	return bestAny, false
}

// panelsThatFit returns the most leading panels (down to the minimum) a
// readable grid can hold on this canvas, or 0 when even the minimum cannot.
func (sm *smallMultiples) panelsThatFit(m smMetrics) int {
	for n := len(sm.data.Series) - 1; n >= SmallMultiplesMinPanels; n-- {
		if _, ok := chooseGrid(sm.grids(m, n, m.panelTitleWidths[:n])); ok {
			return n
		}
	}
	return 0
}

// panelRect returns panel i's plot rectangle.
func panelRect(m smMetrics, g smGrid, i int) Rect {
	col, row := i%g.cols, i/g.cols
	rowH := g.plotH + m.panelTitleH + m.xLabelH
	return Rect{
		X: m.gridLeft + float64(col)*(g.plotW+g.colGap),
		Y: m.gridTop + float64(row)*(rowH+g.rowGap) + m.panelTitleH,
		W: g.plotW,
		H: g.plotH,
	}
}

// panelPoints maps a panel's values onto its plot rectangle.
func panelPoints(plot Rect, domain [2]float64, categories []string, values []float64) []Point {
	ys := niceScale(domain)
	lo, hi := ys.DomainBounds()
	ys = NewLinearScale(lo, hi).SetRangeLinear(plot.Y+plot.H, plot.Y)
	xs := NewCategoricalScale(categories)
	xs.SetRangeCategorical(plot.X, plot.X+plot.W)
	out := make([]Point, 0, len(values))
	for i, v := range values {
		if i >= len(categories) {
			break
		}
		out = append(out, Point{X: xs.Scale(categories[i]), Y: ys.Scale(v)})
	}
	return out
}

func (sm *smallMultiples) draw() error {
	b := sm.b
	style := b.StyleGuide()
	b.CheckChartCapacity(len(sm.data.Series), len(sm.data.Categories))

	m := sm.measure()
	grid, ok := chooseGrid(sm.grids(m, len(sm.data.Series), m.panelTitleWidths))
	if !ok {
		sm.reportUnreadable(m, grid)
	}

	colors := sm.panelColors(style)
	vf := sm.tickFormatter(drawnTickDecimals(m.domains, yTickCountForHeight(style, grid.plotH)))
	for i, s := range sm.data.Series {
		sm.drawPanel(m, grid, i, s, colors[i], vf)
	}

	drawChartHeader(b, sm.config.Width, sm.showTitle, sm.data.Title, sm.data.Subtitle)
	if sm.mode == YScaleIndependent {
		b.Push()
		b.SetFontSize(style.Typography.SizeCaption).SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(style.Palette.TextSecondary)
		note := b.TruncateToWidth(SmallMultiplesIndependentNote, sm.config.Width-2*style.Spacing.XS)
		b.DrawText(note, m.gridLeft-m.yGutter, m.gridTop-m.noteH, TextAlignLeft, TextBaselineTop)
		b.Pop()
	}
	if sm.data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = sm.data.Footnote
		NewFootnote(b, footnoteConfig).Draw(Rect{X: 0, Y: sm.config.Height - m.footerH, W: sm.config.Width, H: m.footerH})
	}
	return nil
}

// panelColors gives every panel the same accent — panels are the same
// measure, identified by their titles — unless data.colors sets them or
// data.highlight names the panels the slide is about (those keep the accent,
// the rest turn neutral grey).
func (sm *smallMultiples) panelColors(style *StyleGuide) []Color {
	n := len(sm.data.Series)
	base := resolveColors(sm.colors, style, 1)
	out := make([]Color, n)
	for i := range out {
		if len(sm.colors) > 0 {
			out[i] = sm.colors[i%len(sm.colors)]
			continue
		}
		out[i] = base[0]
	}
	if sm.data.SeriesHighlightSet {
		neutral := NeutralInk(style.Palette, BarNeutralInk)
		for i := range out {
			if !seriesHighlighted(sm.data, i) {
				out[i] = neutral
			}
		}
	}
	return out
}

// drawPanel draws one facet: title, gridlines, y tick labels (the left column
// only when the scale is shared), category labels, baseline and line.
func (sm *smallMultiples) drawPanel(m smMetrics, g smGrid, i int, s ChartSeries, color Color, vf *ValueFormatter) {
	b := sm.b
	style := b.StyleGuide()
	plot := panelRect(m, g, i)

	// Panel title, left-aligned on the plot edge and fitted to its width.
	b.Push()
	b.SetFontSize(m.titleFS).SetFontWeight(style.Typography.WeightBold)
	b.SetTextColor(style.Palette.TextPrimary)
	b.DrawText(b.TruncateToWidth(s.Name, plot.W), plot.X, plot.Y-m.panelTitleH, TextAlignLeft, TextBaselineTop)
	b.Pop()

	domain := m.domains[i]
	ys := niceScale(domain)
	lo, hi := ys.DomainBounds()
	gridScale := NewLinearScale(lo, hi).SetRangeLinear(plot.H, 0)
	if !sm.config.ShowGrid {
		gridScale = nil
	}
	DrawCartesianGrid(b, plot, gridScale, nil)

	// Y tick labels.
	if sm.mode == YScaleIndependent || i%g.cols == 0 {
		tc := yTickCountForHeight(style, plot.H)
		b.Push()
		b.SetFontSize(m.labelFS).SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(style.Palette.TextSecondary)
		axis := NewLinearScale(lo, hi).SetRangeLinear(plot.Y+plot.H, plot.Y)
		for _, v := range axis.Ticks(tc) {
			y := axis.Scale(v)
			if y < plot.Y-0.5 || y > plot.Y+plot.H+0.5 {
				continue
			}
			b.DrawText(formatSMTick(vf, v, hi-lo), plot.X-4, y, TextAlignRight, TextBaselineMiddle)
		}
		b.Pop()
	}

	// Baseline, and a zero line when the domain spans zero.
	b.Push()
	b.SetStrokeColor(style.Palette.TextSecondary).SetStrokeWidth(0.75)
	b.DrawLine(plot.X, plot.Y+plot.H, plot.X+plot.W, plot.Y+plot.H)
	if lo < 0 && hi > 0 {
		zero := NewLinearScale(lo, hi).SetRangeLinear(plot.Y+plot.H, plot.Y).Scale(0)
		b.DrawLine(plot.X, zero, plot.X+plot.W, zero)
	}
	b.Pop()

	// Category labels under every panel: the scale is shared, but each panel
	// must read on its own.
	xs := NewCategoricalScale(sm.data.Categories)
	xs.SetRangeCategorical(plot.X, plot.X+plot.W)
	band := plot.W / float64(len(sm.data.Categories))
	b.Push()
	b.SetFontSize(m.labelFS).SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextSecondary)
	for _, c := range sm.data.Categories {
		b.DrawText(b.TruncateToWidth(c, band), xs.Scale(c), plot.Y+plot.H+4, TextAlignCenter, TextBaselineTop)
	}
	b.Pop()

	// The line.
	cfg := DefaultLineSeriesConfig()
	cfg.Color, cfg.MarkerFillColor = color, color
	cfg.StrokeWidth = 2
	cfg.MarkerSize = 6
	cfg.ShowMarkers = len(sm.data.Categories) <= smallMultiplesMarkerMaxPoints
	if sm.data.SeriesHighlightSet && !seriesHighlighted(sm.data, i) {
		cfg.StrokeWidth = contextStrokeWidth(cfg.StrokeWidth)
		cfg.ShowMarkers = false
	}
	if s.Color != nil {
		cfg.Color, cfg.MarkerFillColor = *s.Color, *s.Color
	}
	points := make([]DataPoint, 0, len(s.Values))
	for k, v := range s.Values {
		if k >= len(sm.data.Categories) {
			break
		}
		points = append(points, DataPoint{XCategory: sm.data.Categories[k], Y: v})
	}
	lineY := NewLinearScale(lo, hi).SetRangeLinear(plot.Y+plot.H, plot.Y)
	NewLineSeries(b, cfg).DrawCategorical(points, xs, lineY, plot.Y+plot.H)
}

// reportUnreadable reports a panel count no grid on this canvas can draw
// above the readability floors, with how many panels per slide would fit.
func (sm *smallMultiples) reportUnreadable(m smMetrics, g smGrid) {
	n := len(sm.data.Series)
	fit := sm.panelsThatFit(m)
	advice := "enlarge the chart's placeholder or shorten the panel titles and category labels"
	if fit > 0 {
		advice = fmt.Sprintf("split the panels across slides of %d or fewer, or enlarge the chart's placeholder", fit)
	}
	sm.b.AddFinding(Finding{
		Field:    "data.series",
		Code:     FindingPlotAreaCollapsed,
		Severity: core.SeverityShrinkOrSplit,
		Message: fmt.Sprintf("%d small-multiple panels do not fit this canvas above the readability floors (best grid %dx%d: %s); %s",
			n, g.cols, g.rows, strings.Join(g.failures, "; "), advice),
		Fix: &FixSuggestion{
			Kind: FixKindTruncateOrSplit,
			Params: map[string]any{
				"panels":               n,
				"max_panels_per_slide": fit,
				"plot_width_pt":        math.Round(math.Max(0, g.plotW)),
				"plot_height_pt":       math.Round(math.Max(0, g.plotH)),
				"min_plot_height_pt":   math.Round(m.minPlotH),
				"alternative":          "split_slide",
			},
		},
	})
}

// DataSchema returns the JSON Schema for small_multiples_chart data.
func (d *SmallMultiplesDiagram) DataSchema() *DataSchema {
	panel := ObjectDataSchema("One panel: its title and one value per category", map[string]*DataSchema{
		"name":   StringDataSchema("Panel title (required), e.g. the region"),
		"values": ArrayDataSchema("One number per category", NumberDataSchema("Value"), 1),
	}, []string{"name", "values"})
	return ObjectDataSchema(
		"Small multiples: 2–6 line panels over shared categories, one named series per panel. The panels share one x scale and, by default, one y domain so magnitudes compare across panels.",
		map[string]*DataSchema{
			"categories": ArrayDataSchema("Category labels shared by every panel (x axis)", StringDataSchema("Category"), 1),
			"labels":     ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"x_labels":   ArrayDataSchema("Alias for categories", StringDataSchema("Label"), 1),
			"series":     ArrayDataSchema(fmt.Sprintf("The panels, %d–%d, drawn left-to-right then top-to-bottom", SmallMultiplesMinPanels, SmallMultiplesMaxPanels), panel, SmallMultiplesMinPanels),
			"y_scale": EnumDataSchema(
				"\"shared\" (default): one y domain computed from every panel, so equal values sit at equal heights and magnitudes compare. \"independent\": each panel its own y axis, for comparing shapes only; the chart is labelled so.",
				YScaleShared, YScaleIndependent),
			"highlight": ArrayDataSchema("Panels the slide is about (series names or 0-based indices): their lines keep the accent, the rest turn neutral grey", &DataSchema{Description: "0-based panel index (integer) or panel name (string)"}, 0),
			"colors":    ArrayDataSchema("Per-panel hex colour overrides (default: every panel the same accent)", StringDataSchema("Hex color, e.g. #FF0000"), 0),
			"footnote":  StringDataSchema("Chart footnote text (source line)"),
		},
		[]string{"categories", "series"},
	)
}

// formatSMTick formats a tick value, snapping floating-point residue around
// zero (a tick accumulated as -1e-14) to zero so it never prints "-0".
func formatSMTick(vf *ValueFormatter, v, span float64) string {
	if math.Abs(v) <= math.Abs(span)*1e-9 {
		v = 0
	}
	return vf.FormatOr(v, "%.0f")
}

// drawnTickDecimals is the one tick precision every panel prints: the finest
// any panel's tick step needs at the tick count the panels are drawn with.
// (measure reserved the gutter for the finest over every count.)
func drawnTickDecimals(domains [][2]float64, tickCount int) int {
	dec := 0
	for _, d := range domains {
		if n := printfDecimals(niceScale(d).TickFormat(tickCount)); n > dec {
			dec = n
		}
	}
	return dec
}
