package svggen

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// Gantt Chart Diagram
// =============================================================================

// GanttConfig holds configuration for Gantt chart diagrams.
type GanttConfig struct {
	ChartConfig

	// RowHeight is the height of each task row.
	RowHeight float64

	// RowSpacing is the vertical spacing between rows.
	RowSpacing float64

	// BarHeight is the height of task bars.
	BarHeight float64

	// BarCornerRadius is the corner radius for task bars.
	BarCornerRadius float64

	// MilestoneSize is the size of milestone diamonds.
	MilestoneSize float64

	// LabelWidth is the width of the left label column.
	LabelWidth float64

	// ShowGrid enables vertical grid lines at time intervals.
	ShowGrid bool

	// GridColor is the color for grid lines.
	GridColor Color

	// DependencyColor is the color for dependency arrows.
	DependencyColor Color

	// DependencyStrokeWidth is the line width for dependency arrows.
	DependencyStrokeWidth float64

	// TimeAxisHeight is the height of the time axis area.
	TimeAxisHeight float64

	// SwimlaneHeaderWidth is the extra width for swimlane headers (0 = no swimlanes).
	SwimlaneHeaderWidth float64

	// SwimlaneColor is the background color for alternating swimlane bands.
	SwimlaneColor Color

	// ShowProgress enables progress bar rendering on tasks.
	ShowProgress bool

	// ProgressColor is the color for the progress fill.
	ProgressColor Color
}

// DefaultGanttConfig returns default configuration for Gantt chart diagrams.
func DefaultGanttConfig(width, height float64) GanttConfig {
	return GanttConfig{
		ChartConfig:           DefaultChartConfig(width, height),
		RowHeight:             32,
		RowSpacing:            6,
		BarHeight:             20,
		BarCornerRadius:       3,
		MilestoneSize:         24,
		LabelWidth:            math.Max(140, width*0.25),
		ShowGrid:              true,
		GridColor:             MustParseColor("#E5E5E5"),
		DependencyColor:       MustParseColor("#666666"),
		DependencyStrokeWidth: 2.5,
		TimeAxisHeight:        28,
		SwimlaneHeaderWidth:   0,
		SwimlaneColor:         MustParseColor("#F8F8F8"),
		ShowProgress:          false,
		ProgressColor:         MustParseColor("#59A14F"),
	}
}

// GanttTask represents a single task in the Gantt chart.
type GanttTask struct {
	// ID is a unique identifier for this task.
	ID string

	// Label is the display text for the task.
	Label string

	// StartDate is the start date of the task.
	StartDate time.Time

	// EndDate is the end date of the task.
	EndDate time.Time

	// Date is for milestone tasks (point-in-time).
	Date time.Time

	// IsMilestone indicates this is a milestone (diamond marker).
	IsMilestone bool

	// Category is an optional grouping key (for color-coding).
	Category string

	// Dependencies are IDs of tasks this task depends on.
	Dependencies []string

	// Progress is the completion percentage (0-100).
	Progress float64

	// Color overrides the default color for this task.
	Color *Color

	// Swimlane is the optional workstream/team grouping.
	Swimlane string
}

// GanttMilestone represents a standalone milestone in the Gantt chart.
type GanttMilestone struct {
	// ID is a unique identifier.
	ID string

	// Label is the display text.
	Label string

	// Date is the milestone date.
	Date time.Time

	// Color overrides the default color.
	Color *Color

	// Swimlane is the optional workstream grouping.
	Swimlane string
}

// GanttData represents the data for a Gantt chart diagram.
type GanttData struct {
	// Title is the diagram title.
	Title string

	// Subtitle is the diagram subtitle.
	Subtitle string

	// Tasks are the Gantt tasks.
	Tasks []GanttTask

	// Milestones are standalone milestones.
	Milestones []GanttMilestone

	// Footnote is an optional footnote text.
	Footnote string

	// TimeUnit overrides auto-detected time unit: "day", "week", "month", "quarter", "year".
	TimeUnit string
}

// GanttChart renders Gantt chart diagrams.
type GanttChart struct {
	builder *SVGBuilder
	config  GanttConfig
}

// NewGanttChart creates a new Gantt chart renderer.
func NewGanttChart(builder *SVGBuilder, config GanttConfig) *GanttChart {
	return &GanttChart{
		builder: builder,
		config:  config,
	}
}

// ganttRow represents a rendered row with its task/milestone and Y position.
type ganttRow struct {
	task      *GanttTask
	milestone *GanttMilestone
	y         float64 // top of the row
	swimlane  string
}

// ganttOverflow tracks how many rows were hidden due to space constraints.
type ganttOverflow struct {
	count int // number of hidden rows
}

// Draw renders the Gantt chart diagram.
func (gc *GanttChart) Draw(data GanttData) error {
	if len(data.Tasks) == 0 && len(data.Milestones) == 0 {
		return nil
	}

	b := gc.builder
	style := b.StyleGuide()

	// Override hardcoded config colors with theme palette so diagrams
	// respect the active template's color scheme.
	gc.config.GridColor = style.Palette.Border
	gc.config.DependencyColor = style.Palette.TextMuted
	gc.config.SwimlaneColor = style.Palette.Surface
	gc.config.ProgressColor = style.Palette.Success

	plotArea := gc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if gc.config.ShowTitle && data.Title != "" {
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
	plotArea.H -= headerHeight + footerHeight + gc.config.TimeAxisHeight

	// Calculate date range
	dateRange := gc.calculateDateRange(data)

	// Detect time unit if not specified
	timeUnit := data.TimeUnit
	if timeUnit == "" {
		timeUnit = gc.detectTimeUnit(dateRange)
	}

	// Build rows (merging tasks and milestones, respecting swimlanes).
	// When there are too many rows to fit at the minimum readable row
	// height, buildRows caps the visible count and returns an overflow.
	rows, overflow := gc.buildRows(data, plotArea)

	// Collect swimlanes for headers
	swimlanes := gc.collectSwimlanes(rows)

	// Auto-size label width to fit the longest label (capped at 35% of plot width)
	labelWidth := gc.autoSizeLabelWidth(rows, plotArea, style)
	if len(swimlanes) > 0 && gc.config.SwimlaneHeaderWidth > 0 {
		labelWidth += gc.config.SwimlaneHeaderWidth
	}

	// Chart area is to the right of labels
	chartArea := Rect{
		X: plotArea.X + labelWidth,
		Y: plotArea.Y,
		W: plotArea.W - labelWidth,
		H: plotArea.H,
	}

	// Draw swimlane backgrounds
	if len(swimlanes) > 0 {
		gc.drawSwimlaneBackgrounds(rows, swimlanes, plotArea, chartArea)
	}

	// Draw grid
	if gc.config.ShowGrid {
		gc.drawGrid(dateRange, timeUnit, chartArea)
	}

	// Build task ID -> row index map for dependency arrows
	taskRowMap := make(map[string]int)
	for i, row := range rows {
		if row.task != nil {
			taskRowMap[row.task.ID] = i
		}
		if row.milestone != nil {
			taskRowMap[row.milestone.ID] = i
		}
	}

	// Draw task bars, milestones, and labels
	categoryColors := gc.getCategoryColors(data, style)
	gc.drawAllRows(rows, dateRange, chartArea, categoryColors, plotArea.X, labelWidth)

	// Draw dependency arrows
	gc.drawDependencies(data.Tasks, rows, taskRowMap, dateRange, chartArea)

	// Draw "+N more" overflow indicator when rows were capped
	if overflow.count > 0 {
		gc.drawOverflowIndicator(overflow.count, rows, plotArea.X, labelWidth, chartArea)
	}

	// Draw swimlane headers
	if len(swimlanes) > 0 && gc.config.SwimlaneHeaderWidth > 0 {
		gc.drawSwimlaneHeaders(rows, swimlanes, plotArea)
	}

	// Draw time axis
	gc.drawTimeAxis(dateRange, timeUnit, Rect{
		X: chartArea.X,
		Y: chartArea.Y + chartArea.H,
		W: chartArea.W,
		H: gc.config.TimeAxisHeight,
	})

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
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: gc.config.Height - footerHeight,
			W: gc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// calculateDateRange determines the Gantt chart's date range.
func (gc *GanttChart) calculateDateRange(data GanttData) timelineRange {
	var minDate, maxDate time.Time

	// Helper to expand the range with a date
	expand := func(d time.Time) {
		if d.IsZero() {
			return
		}
		if minDate.IsZero() || d.Before(minDate) {
			minDate = d
		}
		if maxDate.IsZero() || d.After(maxDate) {
			maxDate = d
		}
	}

	for _, task := range data.Tasks {
		if task.IsMilestone {
			expand(task.Date)
		} else {
			expand(task.StartDate)
			expand(task.EndDate)
		}
	}

	for _, ms := range data.Milestones {
		expand(ms.Date)
	}

	// Add padding (5% on each side)
	duration := maxDate.Sub(minDate)
	padding := time.Duration(float64(duration) * 0.05)
	if padding < 24*time.Hour {
		padding = 24 * time.Hour
	}

	minDate = minDate.Add(-padding)
	maxDate = maxDate.Add(padding)

	return timelineRange{
		start:    minDate,
		end:      maxDate,
		duration: maxDate.Sub(minDate),
	}
}

// detectTimeUnit determines the appropriate time unit for the axis.
func (gc *GanttChart) detectTimeUnit(dateRange timelineRange) string {
	days := float64(dateRange.duration) / float64(24*time.Hour)

	switch {
	case days <= 14:
		return "day"
	case days <= 60:
		return "week"
	case days <= 365:
		return "month"
	case days <= 365*3:
		return "quarter"
	default:
		return "year"
	}
}

// buildRows creates the row layout for all tasks and milestones.
// When there are too many rows to fit at a minimum readable row height,
// the visible count is capped and the overflow count is returned so a
// "+N more" indicator can be rendered.
func (gc *GanttChart) buildRows(data GanttData, plotArea Rect) ([]ganttRow, ganttOverflow) {
	var rows []ganttRow

	// Add tasks
	for i := range data.Tasks {
		rows = append(rows, ganttRow{
			task:     &data.Tasks[i],
			swimlane: data.Tasks[i].Swimlane,
		})
	}

	// Add standalone milestones
	for i := range data.Milestones {
		rows = append(rows, ganttRow{
			milestone: &data.Milestones[i],
			swimlane:  data.Milestones[i].Swimlane,
		})
	}

	// Sort by swimlane, then by start date
	sort.SliceStable(rows, func(i, j int) bool {
		// First by swimlane
		si, sj := rows[i].swimlane, rows[j].swimlane
		if si != sj {
			return si < sj
		}
		// Then by start date
		di := gc.rowStartDate(rows[i])
		dj := gc.rowStartDate(rows[j])
		return di.Before(dj)
	})

	// Cap visible rows when there are too many to fit at the minimum
	// readable row height. The minimum is BarHeight (default 20) + a
	// small spacing so bars never overlap and labels remain readable.
	var overflowCount int
	const minRowHeight = 16.0 // absolute minimum row height for readability
	const minRowSpacing = 2.0 // absolute minimum spacing between rows
	n := len(rows)
	if n > 0 {
		// Calculate how many rows can fit at the minimum row height
		maxRows := int((plotArea.H + minRowSpacing) / (minRowHeight + minRowSpacing))
		if maxRows < 2 {
			maxRows = 2
		}
		if n > maxRows {
			overflowCount = n - maxRows
			rows = rows[:maxRows]
			n = maxRows
		}
	}

	// Scale row dimensions to fill plot area, but cap bar height so
	// charts with very few tasks don't produce absurdly tall bars.
	if n > 0 {
		totalHeight := float64(n)*gc.config.RowHeight + float64(n-1)*gc.config.RowSpacing
		if totalHeight > 0 {
			scale := plotArea.H / totalHeight
			gc.config.RowHeight *= scale
			gc.config.RowSpacing *= scale
			gc.config.BarHeight *= scale
			gc.config.MilestoneSize *= scale

			// Cap bar height to a reasonable maximum
			const maxBarHeight = 60.0
			if gc.config.BarHeight > maxBarHeight {
				capScale := maxBarHeight / gc.config.BarHeight
				gc.config.RowHeight *= capScale
				gc.config.RowSpacing *= capScale
				gc.config.BarHeight = maxBarHeight
				gc.config.MilestoneSize *= capScale
			}

			// Enforce minimum bar height so bars remain visible
			const minBarH = 8.0
			if gc.config.BarHeight < minBarH {
				gc.config.BarHeight = minBarH
			}

			// Enforce a minimum milestone size so diamonds remain visible
			// even when many tasks compress the row height.
			const minMilestoneSize = 14.0
			if gc.config.MilestoneSize < minMilestoneSize {
				gc.config.MilestoneSize = minMilestoneSize
			}
		}
	}

	// Center content vertically when it doesn't fill the full plot area
	// (e.g., after bar-height capping with few tasks).
	if n > 0 {
		totalContentHeight := float64(n)*gc.config.RowHeight + float64(n-1)*gc.config.RowSpacing
		if totalContentHeight < plotArea.H {
			verticalOffset := (plotArea.H - totalContentHeight) / 2
			plotArea.Y += verticalOffset
		}
	}

	// Assign Y positions
	for i := range rows {
		rows[i].y = plotArea.Y + float64(i)*(gc.config.RowHeight+gc.config.RowSpacing)
	}

	return rows, ganttOverflow{count: overflowCount}
}

// rowStartDate returns the effective start date for a row (for sorting).
func (gc *GanttChart) rowStartDate(row ganttRow) time.Time {
	if row.task != nil {
		if row.task.IsMilestone {
			return row.task.Date
		}
		return row.task.StartDate
	}
	if row.milestone != nil {
		return row.milestone.Date
	}
	return time.Time{}
}

// collectSwimlanes returns unique swimlane names in order of appearance.
func (gc *GanttChart) collectSwimlanes(rows []ganttRow) []string {
	seen := make(map[string]bool)
	var swimlanes []string
	for _, row := range rows {
		if row.swimlane != "" && !seen[row.swimlane] {
			seen[row.swimlane] = true
			swimlanes = append(swimlanes, row.swimlane)
		}
	}
	return swimlanes
}

// drawAllRows draws task bars, milestones, and labels for all rows.
func (gc *GanttChart) drawAllRows(rows []ganttRow, dateRange timelineRange, chartArea Rect, categoryColors map[string]Color, labelX, labelWidth float64) {
	for _, row := range rows {
		if row.task != nil {
			if row.task.IsMilestone {
				gc.drawMilestoneMarker(*row.task, row.y, dateRange, chartArea, categoryColors)
			} else {
				gc.drawTaskBar(*row.task, row.y, dateRange, chartArea, categoryColors)
			}
		}
		if row.milestone != nil {
			gc.drawStandaloneMilestone(*row.milestone, row.y, dateRange, chartArea)
		}

		label := row.rowLabel()
		if label != "" {
			gc.drawRowLabel(label, row.y, labelX, labelWidth)
		}
	}
}

// rowLabel returns the display label for this row.
func (r ganttRow) rowLabel() string {
	if r.task != nil {
		return r.task.Label
	}
	if r.milestone != nil {
		return r.milestone.Label
	}
	return ""
}

// drawGrid draws vertical grid lines at time intervals.
func (gc *GanttChart) drawGrid(dateRange timelineRange, timeUnit string, chartArea Rect) {
	b := gc.builder
	style := b.StyleGuide()

	// Reuse the timeline's time tick generation logic
	tc := &TimelineChart{config: TimelineConfig{ChartConfig: gc.config.ChartConfig}}
	ticks := tc.generateTimeTicks(dateRange, timeUnit)

	b.Push()
	b.SetStrokeColor(gc.config.GridColor)
	b.SetStrokeWidth(style.Strokes.WidthThin)

	for _, tick := range ticks {
		x := dateRange.dateToX(tick, chartArea)
		b.DrawLine(x, chartArea.Y, x, chartArea.Y+chartArea.H)
	}

	b.Pop()
}

// drawTaskBar draws a single task bar.
func (gc *GanttChart) drawTaskBar(task GanttTask, rowY float64, dateRange timelineRange, chartArea Rect, categoryColors map[string]Color) {
	b := gc.builder
	style := b.StyleGuide()

	startX := dateRange.dateToX(task.StartDate, chartArea)
	endX := dateRange.dateToX(task.EndDate, chartArea)
	barWidth := endX - startX
	if barWidth < 6 {
		barWidth = 6
	}

	barY := rowY + (gc.config.RowHeight-gc.config.BarHeight)/2

	// Determine color
	fillColor := gc.taskColor(task, categoryColors, style)

	rect := Rect{
		X: startX,
		Y: barY,
		W: barWidth,
		H: gc.config.BarHeight,
	}

	// Draw bar background
	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeColor(fillColor.Darken(0.15))
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawRoundedRect(rect, gc.config.BarCornerRadius)
	b.Pop()

	// Draw progress overlay if enabled
	if gc.config.ShowProgress && task.Progress > 0 {
		progressWidth := barWidth * (task.Progress / 100.0)
		b.Push()
		b.SetFillColor(gc.config.ProgressColor)
		b.SetStrokeWidth(0)
		b.DrawRoundedRect(Rect{
			X: startX,
			Y: barY,
			W: progressWidth,
			H: gc.config.BarHeight,
		}, gc.config.BarCornerRadius)
		b.Pop()
	}

	// Draw bar label inside the bar when it fits comfortably.
	// Use contrast-aware text color so text remains readable on both
	// light and dark bar backgrounds. When the label does not fit
	// inside, it is placed to the right of the bar as a fallback.
	gc.drawBarLabel(task.Label, startX, barY, barWidth, gc.config.BarHeight, fillColor, chartArea.X+chartArea.W)
}

// drawMilestoneMarker draws a milestone diamond for a task.
// Milestones use a distinct accent color (Warning/gold) and a thicker stroke
// so they stand out clearly from regular task bars, even at presentation distance.
func (gc *GanttChart) drawMilestoneMarker(task GanttTask, rowY float64, dateRange timelineRange, chartArea Rect, categoryColors map[string]Color) {
	b := gc.builder
	style := b.StyleGuide()

	x := dateRange.dateToX(task.Date, chartArea)
	y := rowY + gc.config.RowHeight/2
	halfSize := gc.config.MilestoneSize / 2

	// Use Warning (gold/orange) for milestone markers to distinguish them
	// from regular task bars. Explicit per-task color overrides still win.
	fillColor := style.Palette.Warning
	if task.Color != nil {
		fillColor = *task.Color
	}

	points := []Point{
		{X: x, Y: y - halfSize},
		{X: x + halfSize, Y: y},
		{X: x, Y: y + halfSize},
		{X: x - halfSize, Y: y},
	}

	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeColor(fillColor.Darken(0.25))
	b.SetStrokeWidth(style.Strokes.WidthThick)
	b.DrawPolygon(points)
	b.Pop()

	// Draw date label below the diamond
	gc.drawMilestoneDateLabel(task.Date, x, y+halfSize, style)
}

// drawStandaloneMilestone draws a standalone milestone from the Milestones list.
// Uses the Warning (gold/orange) accent color and a thick stroke for visibility.
func (gc *GanttChart) drawStandaloneMilestone(ms GanttMilestone, rowY float64, dateRange timelineRange, chartArea Rect) {
	b := gc.builder
	style := b.StyleGuide()

	x := dateRange.dateToX(ms.Date, chartArea)
	y := rowY + gc.config.RowHeight/2
	halfSize := gc.config.MilestoneSize / 2

	fillColor := style.Palette.Warning // Milestones use gold/orange for visibility
	if ms.Color != nil {
		fillColor = *ms.Color
	}

	points := []Point{
		{X: x, Y: y - halfSize},
		{X: x + halfSize, Y: y},
		{X: x, Y: y + halfSize},
		{X: x - halfSize, Y: y},
	}

	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeColor(fillColor.Darken(0.25))
	b.SetStrokeWidth(style.Strokes.WidthThick)
	b.DrawPolygon(points)
	b.Pop()

	// Draw date label below the diamond
	gc.drawMilestoneDateLabel(ms.Date, x, y+halfSize, style)
}

// drawMilestoneDateLabel renders a compact date label just below a milestone diamond.
// The label uses a small muted font so it provides context without cluttering the chart.
func (gc *GanttChart) drawMilestoneDateLabel(d time.Time, cx, belowY float64, style *StyleGuide) {
	if d.IsZero() {
		return
	}
	b := gc.builder

	label := d.Format("Jan 2")
	fontSize := style.Typography.SizeCaption
	if fontSize == 0 {
		fontSize = style.Typography.SizeSmall * 0.8
	}

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextSecondary)

	// Position just below the diamond with a small gap
	gap := style.Spacing.XS
	b.DrawText(label, cx, belowY+gap, TextAlignCenter, TextBaselineTop)
	b.Pop()
}

// drawDependencies draws arrows between dependent tasks.
func (gc *GanttChart) drawDependencies(tasks []GanttTask, rows []ganttRow, taskRowMap map[string]int, dateRange timelineRange, chartArea Rect) {
	b := gc.builder

	b.Push()
	b.SetStrokeColor(gc.config.DependencyColor)
	b.SetStrokeWidth(gc.config.DependencyStrokeWidth)
	b.SetFillColor(gc.config.DependencyColor)

	for _, task := range tasks {
		if len(task.Dependencies) == 0 {
			continue
		}

		targetIdx, ok := taskRowMap[task.ID]
		if !ok {
			continue
		}
		targetRow := rows[targetIdx]

		for _, depID := range task.Dependencies {
			sourceIdx, ok := taskRowMap[depID]
			if !ok {
				continue
			}
			sourceRow := rows[sourceIdx]

			gc.drawDependencyArrow(sourceRow, targetRow, dateRange, chartArea)
		}
	}

	b.Pop()
}

// drawDependencyArrow draws a single dependency arrow from source to target.
func (gc *GanttChart) drawDependencyArrow(source, target ganttRow, dateRange timelineRange, chartArea Rect) {
	b := gc.builder

	// Source: end of the source task bar
	var sourceX, sourceY float64
	if source.task != nil {
		if source.task.IsMilestone {
			sourceX = dateRange.dateToX(source.task.Date, chartArea) + gc.config.MilestoneSize/2
		} else {
			sourceX = dateRange.dateToX(source.task.EndDate, chartArea)
		}
	} else if source.milestone != nil {
		sourceX = dateRange.dateToX(source.milestone.Date, chartArea) + gc.config.MilestoneSize/2
	}
	sourceY = source.y + gc.config.RowHeight/2

	// Target: start of the target task bar with a gap so the arrowhead doesn't overlap
	arrowSize := 5.0
	arrowGap := arrowSize + 2 // leave space for the filled arrowhead
	var targetX, targetY float64
	if target.task != nil {
		if target.task.IsMilestone {
			targetX = dateRange.dateToX(target.task.Date, chartArea) - gc.config.MilestoneSize/2
		} else {
			targetX = dateRange.dateToX(target.task.StartDate, chartArea)
		}
	} else if target.milestone != nil {
		targetX = dateRange.dateToX(target.milestone.Date, chartArea) - gc.config.MilestoneSize/2
	}
	targetY = target.y + gc.config.RowHeight/2

	// The arrow tip stops before the bar edge
	tipX := targetX - arrowGap

	// Draw right-angle connector: horizontal from source, then vertical, then horizontal to target
	midX := sourceX + (tipX-sourceX)/2
	if midX < sourceX+8 {
		midX = sourceX + 8
	}

	b.DrawLine(sourceX, sourceY, midX, sourceY)
	b.DrawLine(midX, sourceY, midX, targetY)
	b.DrawLine(midX, targetY, tipX, targetY)

	// Draw filled triangle arrowhead pointing right at tipX
	b.DrawPolygon([]Point{
		{X: tipX + arrowSize, Y: targetY},
		{X: tipX, Y: targetY - arrowSize},
		{X: tipX, Y: targetY + arrowSize},
	})
}

// drawRowLabel draws a task label in the left column.
// Font size is adaptively reduced based on row height so that labels remain
// readable even when many tasks compress the rows. The font size floors at
// 9pt (DefaultMinFontSize), and labels that still overflow are truncated
// with an ellipsis.
func (gc *GanttChart) drawRowLabel(label string, rowY, labelX, labelWidth float64) {
	b := gc.builder
	style := b.StyleGuide()

	// Use SizeBody for row labels — they are the primary reading content of a Gantt chart
	// and must remain readable when the chart is scaled down in PPTX placeholders.
	maxFontSize := style.Typography.SizeBody

	// Adaptive font size reduction: when rows are compressed (many tasks),
	// scale the max font size down proportionally to the row height. The
	// nominal row height is 32 (DefaultGanttConfig). With fewer tasks the
	// rows expand and maxFontSize stays at SizeBody; with many tasks the
	// rows shrink and maxFontSize shrinks too, down to a floor of 9pt.
	const nominalRowHeight = 32.0
	if gc.config.RowHeight < nominalRowHeight {
		heightRatio := gc.config.RowHeight / nominalRowHeight
		adaptedSize := maxFontSize * heightRatio
		// Floor at DefaultMinFontSize (9pt)
		if adaptedSize < DefaultMinFontSize {
			adaptedSize = DefaultMinFontSize
		}
		if adaptedSize < maxFontSize {
			maxFontSize = adaptedSize
		}
	}

	minFontSize := DefaultMinFontSize

	// Reserve a gap on the right side of the label column so text never
	// visually collides with the bar area that starts immediately after.
	gap := style.Spacing.MD
	availableWidth := labelWidth - style.Spacing.MD - gap

	// Use LabelFitStrategy for consistent shrink → truncate cascade.
	fit := LabelFitStrategy{PreferredSize: maxFontSize, MinSize: minFontSize, MinCharWidth: 5.5}
	result := fit.Fit(b, label, availableWidth, 0)

	b.Push()
	b.SetFontSize(result.FontSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.SetTextColor(style.Palette.TextPrimary)

	// Right-align in the label column with a gap before the bar area
	x := labelX + labelWidth - gap
	y := rowY + gc.config.RowHeight/2

	b.DrawText(result.DisplayText, x, y, TextAlignRight, TextBaselineMiddle)
	b.Pop()
}

// drawOverflowIndicator renders a "+N more" label below the last visible row
// to indicate that additional tasks were hidden due to space constraints.
func (gc *GanttChart) drawOverflowIndicator(count int, rows []ganttRow, labelX, labelWidth float64, chartArea Rect) {
	if count <= 0 || len(rows) == 0 {
		return
	}

	b := gc.builder
	style := b.StyleGuide()

	lastRow := rows[len(rows)-1]
	y := lastRow.y + gc.config.RowHeight + gc.config.RowSpacing

	label := fmt.Sprintf("+%d more", count)
	fontSize := style.Typography.SizeSmall
	if fontSize < DefaultMinFontSize {
		fontSize = DefaultMinFontSize
	}

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.SetTextColor(style.Palette.TextMuted)

	// Center the indicator across the full chart width (label column + chart area)
	cx := labelX + (labelWidth+chartArea.W)/2
	b.DrawText(label, cx, y, TextAlignCenter, TextBaselineTop)
	b.Pop()
}

// drawBarLabel draws a task label inside a bar when it fits. When the label
// does not fit inside, it is placed to the right of the bar as a fallback.
// The text color is chosen automatically based on the bar's fill color using
// WCAG contrast ratios for internal labels, and uses the primary text color
// for external labels.
func (gc *GanttChart) drawBarLabel(label string, barX, barY, barW, barH float64, fillColor Color, chartRightEdge float64) {
	b := gc.builder
	style := b.StyleGuide()

	// Use compact font size for in-bar labels
	fontSize := style.Typography.SizeSmall

	// Horizontal padding inside the bar
	padX := style.Spacing.SM
	availableW := barW - 2*padX

	// Try to fit inside the bar first
	if availableW > 0 {
		b.Push()
		b.SetFontSize(fontSize)
		b.SetFontWeight(style.Typography.WeightMedium)

		textW, _ := b.MeasureText(label)
		if textW <= availableW {
			// Label fits inside — draw it centered with contrast-aware color
			b.SetTextColor(fillColor.TextColorFor())
			cx := barX + barW/2
			cy := barY + barH/2
			b.DrawText(label, cx, cy, TextAlignCenter, TextBaselineMiddle)
			b.Pop()
			return
		}
		b.Pop()
	}

	// Fallback: place label to the right of the bar
	gap := style.Spacing.SM
	labelX := barX + barW + gap
	externalAvail := chartRightEdge - labelX
	if externalAvail <= 0 {
		return // no space for external label
	}

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.SetTextColor(style.Palette.TextPrimary)

	textW, _ := b.MeasureText(label)
	displayText := label
	if textW > externalAvail {
		displayText = b.TruncateToWidth(label, externalAvail)
		if displayText == "" {
			b.Pop()
			return
		}
	}

	cy := barY + barH/2
	b.DrawText(displayText, labelX, cy, TextAlignLeft, TextBaselineMiddle)
	b.Pop()
}

// autoSizeLabelWidth computes the label column width needed to display the longest
// row label without truncation. Uses real font metrics (MeasureText) for accuracy.
// Returns at least the configured LabelWidth, and caps at 40% of the plot area
// width so the chart area remains usable. The calculation includes a gap between
// the label column and the bar area to prevent text-bar overlap.
func (gc *GanttChart) autoSizeLabelWidth(rows []ganttRow, plotArea Rect, style *StyleGuide) float64 {
	b := gc.builder
	// Inner padding (left of first character + right of last character) plus
	// a dedicated gap that separates the label text from the bar area.
	gap := style.Spacing.MD // clear space between label text and bars
	padding := style.Spacing.MD*2 + gap

	// Save and restore font state (Push/Pop only saves the canvas context,
	// not the builder-level fontSize/fontStyle fields).
	origSize := b.fontSize
	origStyle := b.fontStyle
	defer func() {
		b.SetFontSize(origSize)
		b.fontStyle = origStyle
	}()

	b.SetFontSize(style.Typography.SizeBody)
	b.SetFontWeight(style.Typography.WeightMedium)

	maxTextWidth := 0.0
	for _, row := range rows {
		label := row.rowLabel()
		if label == "" {
			continue
		}
		w, _ := b.MeasureText(label)
		if w > maxTextWidth {
			maxTextWidth = w
		}
	}

	neededWidth := maxTextWidth + padding

	// Use the larger of configured and needed, but cap at 40% of plot width
	labelWidth := gc.config.LabelWidth
	if neededWidth > labelWidth {
		labelWidth = neededWidth
	}
	maxWidth := plotArea.W * 0.40
	if labelWidth > maxWidth {
		labelWidth = maxWidth
	}

	return labelWidth
}

// drawSwimlaneBackgrounds draws alternating background bands for swimlanes.
func (gc *GanttChart) drawSwimlaneBackgrounds(rows []ganttRow, swimlanes []string, plotArea, chartArea Rect) {
	b := gc.builder

	for si, swimlane := range swimlanes {
		if si%2 != 0 {
			continue // Only shade alternating lanes
		}

		// Find first and last row in this swimlane
		firstY := math.MaxFloat64
		lastY := 0.0
		for _, row := range rows {
			if row.swimlane == swimlane {
				if row.y < firstY {
					firstY = row.y
				}
				bottom := row.y + gc.config.RowHeight + gc.config.RowSpacing
				if bottom > lastY {
					lastY = bottom
				}
			}
		}

		if firstY == math.MaxFloat64 {
			continue
		}

		b.Push()
		b.SetFillColor(gc.config.SwimlaneColor)
		b.SetStrokeWidth(0)
		b.FillRect(Rect{
			X: plotArea.X,
			Y: firstY - gc.config.RowSpacing/2,
			W: plotArea.W,
			H: lastY - firstY + gc.config.RowSpacing,
		})
		b.Pop()
	}
}

// drawSwimlaneHeaders draws swimlane group headers on the left.
func (gc *GanttChart) drawSwimlaneHeaders(rows []ganttRow, swimlanes []string, plotArea Rect) {
	b := gc.builder
	style := b.StyleGuide()

	for _, swimlane := range swimlanes {
		// Find vertical extent of this swimlane
		firstY := math.MaxFloat64
		lastY := 0.0
		for _, row := range rows {
			if row.swimlane == swimlane {
				if row.y < firstY {
					firstY = row.y
				}
				bottom := row.y + gc.config.RowHeight
				if bottom > lastY {
					lastY = bottom
				}
			}
		}

		if firstY == math.MaxFloat64 {
			continue
		}

		centerY := (firstY + lastY) / 2

		b.Push()
		b.SetFontSize(style.Typography.SizeSmall)
		b.SetFontWeight(style.Typography.WeightBold)
		b.SetTextColor(style.Palette.TextSecondary)

		headerX := plotArea.X + style.Spacing.SM
		displayText := truncateText(swimlane, gc.config.SwimlaneHeaderWidth-style.Spacing.SM*2, style.Typography.SizeSmall)
		b.DrawText(displayText, headerX, centerY, TextAlignLeft, TextBaselineMiddle)
		b.Pop()
	}
}

// drawTimeAxis draws the time axis with labels (reuses timeline logic).
// When there are many ticks, labels are thinned to prevent overcrowding:
// every Nth label is shown based on the total count, and the first and
// last labels are always displayed.
func (gc *GanttChart) drawTimeAxis(dateRange timelineRange, timeUnit string, axisArea Rect) {
	b := gc.builder
	style := b.StyleGuide()

	// Reuse the timeline's time tick generation logic
	tc := &TimelineChart{config: TimelineConfig{ChartConfig: gc.config.ChartConfig}}
	ticks := tc.generateTimeTicks(dateRange, timeUnit)

	// Draw axis line
	b.Push()
	b.SetStrokeColor(style.Palette.TextSecondary)
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawLine(axisArea.X, axisArea.Y, axisArea.X+axisArea.W, axisArea.Y)
	b.Pop()

	// Determine label thinning interval based on total tick count.
	// This prevents overcrowding when timelines span multiple years
	// with monthly (or finer) tick marks.
	n := len(ticks)
	labelEvery := 1
	switch {
	case n > 36:
		labelEvery = 6 // ~quarterly labels for very long timelines
	case n > 24:
		labelEvery = 3
	case n > 12:
		labelEvery = 2
	}

	// Use a smaller font when there are many labels, even after thinning
	fontSize := style.Typography.SizeSmall
	if n > 12 {
		fontSize = style.Typography.SizeSmall * 0.85
	}

	// Draw tick marks and labels
	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)

	lastIdx := n - 1
	for i, tick := range ticks {
		x := dateRange.dateToX(tick, Rect{X: axisArea.X, Y: 0, W: axisArea.W, H: 0})

		// Always draw tick mark
		b.SetStrokeColor(style.Palette.TextSecondary)
		b.DrawLine(x, axisArea.Y, x, axisArea.Y+5)

		// Only draw label if this tick passes the thinning filter.
		// Always show the first and last tick labels.
		if i == 0 || i == lastIdx || i%labelEvery == 0 {
			label := tc.formatTickLabel(tick, timeUnit)
			b.DrawText(label, x, axisArea.Y+style.Spacing.SM, TextAlignCenter, TextBaselineTop)
		}
	}

	b.Pop()
}

// taskColor determines the color for a task based on category, explicit color, or defaults.
func (gc *GanttChart) taskColor(task GanttTask, categoryColors map[string]Color, style *StyleGuide) Color {
	if task.Color != nil {
		return *task.Color
	}
	if task.Category != "" {
		if c, ok := categoryColors[task.Category]; ok {
			return c
		}
	}
	return style.Palette.Accent1 // Default to theme accent color
}

// getCategoryColors builds a map from category names to colors.
func (gc *GanttChart) getCategoryColors(data GanttData, style *StyleGuide) map[string]Color {
	// Collect unique categories in order
	seen := make(map[string]bool)
	var categories []string
	for _, task := range data.Tasks {
		if task.Category != "" && !seen[task.Category] {
			seen[task.Category] = true
			categories = append(categories, task.Category)
		}
	}

	// Use accent colors from style or defaults
	accents := style.Palette.AccentColors()
	if len(accents) < 3 {
		accents = []Color{
			MustParseColor("#4E79A7"),
			MustParseColor("#59A14F"),
			MustParseColor("#F28E2B"),
			MustParseColor("#E15759"),
			MustParseColor("#76B7B2"),
			MustParseColor("#B07AA1"),
			MustParseColor("#EDC948"),
		}
	}

	result := make(map[string]Color)
	for i, cat := range categories {
		result[cat] = accents[i%len(accents)]
	}
	return result
}

// =============================================================================
// Gantt Diagram Type (for Registry)
// =============================================================================

// GanttDiagram implements the Diagram interface for Gantt chart diagrams.
type GanttDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for Gantt chart diagrams.
func (d *GanttDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("gantt chart requires data. Expected format: {\"tasks\": [{\"name\": \"Design\", \"start\": \"2024-01-01\", \"end\": \"2024-01-15\"}]}")
	}

	_, hasTasks := req.Data["tasks"]
	_, hasMilestones := req.Data["milestones"]
	if !hasTasks && !hasMilestones {
		return fmt.Errorf("gantt chart requires 'tasks' or 'milestones' in data. Expected: {\"tasks\": [{\"name\": \"Design\", \"start\": \"2024-01-01\", \"end\": \"2024-01-15\"}]} or {\"milestones\": [{\"name\": \"Launch\", \"date\": \"2024-03-01\"}]}")
	}

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *GanttDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *GanttDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelperDimensions(req, 900, 500, func(builder *SVGBuilder, req *RequestEnvelope) error {
		// Use default (non-compact) typography for Gantt charts so that row labels
		// and in-bar text remain readable when embedded in narrow PPTX columns.
		// Row heights auto-scale to fit the plot area regardless of font size.

		data, err := parseGanttData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultGanttConfig(width, height)

		// Apply custom settings from data
		if timeUnit, ok := req.Data["time_unit"].(string); ok {
			data.TimeUnit = timeUnit
		}
		if showProgress, ok := req.Data["show_progress"].(bool); ok {
			config.ShowProgress = showProgress
		}
		if showGrid, ok := req.Data["show_grid"].(bool); ok {
			config.ShowGrid = showGrid
		}
		if labelWidth, ok := req.Data["label_width"].(float64); ok {
			config.LabelWidth = labelWidth
		}

		chart := NewGanttChart(builder, config)
		return chart.Draw(data)
	})
}

// parseGanttData parses the request data into GanttData.
func parseGanttData(req *RequestEnvelope) (GanttData, error) {
	data := GanttData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse tasks
	if _, isMap := req.Data["tasks"].(map[string]any); isMap {
		slog.Warn("gantt: 'tasks' should be a JSON array, not an object; tasks will be ignored")
	} else if tasksRaw, ok := toAnySlice(req.Data["tasks"]); ok {
		for i, tRaw := range tasksRaw {
			task := parseGanttTask(tRaw, i)
			data.Tasks = append(data.Tasks, task)
		}
	}

	// Parse standalone milestones
	if msRaw, ok := toAnySlice(req.Data["milestones"]); ok {
		for i, mRaw := range msRaw {
			ms := parseGanttMilestone(mRaw, i)
			data.Milestones = append(data.Milestones, ms)
		}
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	// Parse time unit
	if timeUnit, ok := req.Data["time_unit"].(string); ok {
		data.TimeUnit = timeUnit
	}

	return data, nil
}

// mapStr returns the first non-empty string value for the given keys from a map.
func mapStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

// mapDateField parses a date from the first matching key in the map.
func mapDateField(m map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if t, err := parseDate(s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// parseGanttTask parses a single task from map data.
func parseGanttTask(raw any, index int) GanttTask {
	task := GanttTask{
		ID: fmt.Sprintf("task_%d", index),
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return task
	}

	if id := mapStr(m, "id"); id != "" {
		task.ID = id
	}
	task.Label = mapStr(m, "label", "name")
	task.Category = mapStr(m, "category", "status")
	task.Swimlane = mapStr(m, "swimlane", "team")

	if progress, ok := m["progress"].(float64); ok {
		task.Progress = progress
	}
	if colorStr, ok := m["color"].(string); ok {
		if c, err := ParseColor(colorStr); err == nil {
			task.Color = &c
		}
	}

	// Parse dates
	task.StartDate = mapDateField(m, "start", "start_date")
	task.EndDate = mapDateField(m, "end", "end_date")

	if d := mapDateField(m, "date"); !d.IsZero() {
		task.Date = d
		task.IsMilestone = true
	}

	// Check type field for milestone
	if typ, ok := m["type"].(string); ok && typ == "milestone" {
		task.IsMilestone = true
		if task.Date.IsZero() && !task.StartDate.IsZero() {
			task.Date = task.StartDate
		}
	}

	// Parse dependencies
	task.Dependencies = parseGanttDependencies(m)

	return task
}

// parseGanttDependencies extracts dependency IDs from a task map.
func parseGanttDependencies(m map[string]any) []string {
	if deps, ok := toAnySlice(m["dependencies"]); ok {
		var result []string
		for _, d := range deps {
			if depStr, ok := d.(string); ok {
				result = append(result, depStr)
			}
		}
		return result
	}
	if depStr, ok := m["depends_on"].(string); ok {
		parts := strings.Split(depStr, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return nil
}

// parseGanttMilestone parses a standalone milestone from map data.
func parseGanttMilestone(raw any, index int) GanttMilestone {
	ms := GanttMilestone{
		ID: fmt.Sprintf("milestone_%d", index),
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return ms
	}

	if id, ok := m["id"].(string); ok {
		ms.ID = id
	}
	if label, ok := m["label"].(string); ok {
		ms.Label = label
	} else if name, ok := m["name"].(string); ok {
		ms.Label = name
	}
	if dateStr, ok := m["date"].(string); ok {
		if t, err := parseDate(dateStr); err == nil {
			ms.Date = t
		}
	}
	if colorStr, ok := m["color"].(string); ok {
		if c, err := ParseColor(colorStr); err == nil {
			ms.Color = &c
		}
	}
	if swimlane, ok := m["swimlane"].(string); ok {
		ms.Swimlane = swimlane
	} else if team, ok := m["team"].(string); ok {
		ms.Swimlane = team
	}

	return ms
}

