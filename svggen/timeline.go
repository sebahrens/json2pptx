package svggen

import (
	"fmt"
	"math"
	"slices"
	"time"
)

// =============================================================================
// Timeline Diagram (Activities + Milestones)
// =============================================================================

// TimelineActivityType identifies the type of timeline item.
type TimelineActivityType string

const (
	// TimelineActivityTypeActivity represents a duration-based activity (bar).
	TimelineActivityTypeActivity TimelineActivityType = "activity"

	// TimelineActivityTypeMilestone represents a point-in-time milestone (diamond).
	TimelineActivityTypeMilestone TimelineActivityType = "milestone"

	// TimelineActivityTypePhase represents a phase grouping (background band).
	TimelineActivityTypePhase TimelineActivityType = "phase"
)

// TimelineActivity represents a single item on the timeline.
type TimelineActivity struct {
	// ID is a unique identifier for this activity.
	ID string

	// Label is the display text for the activity.
	Label string

	// Description is optional additional text.
	Description string

	// Type determines how this item is rendered.
	Type TimelineActivityType

	// StartDate is the start date/time (for activities and phases).
	StartDate time.Time

	// EndDate is the end date/time (for activities and phases).
	EndDate time.Time

	// Date is the date for milestones.
	Date time.Time

	// Color overrides the default color for this item.
	Color *Color

	// Icon is an optional icon or emoji.
	Icon string

	// Row specifies which row this activity appears on (0-indexed).
	// If -1, it will be auto-assigned based on overlap.
	Row int

	// Progress is the completion percentage (0-100) for activities.
	Progress float64
}

// TimelineData represents the data for a timeline diagram.
type TimelineData struct {
	// Title is the diagram title.
	Title string

	// Subtitle is the diagram subtitle.
	Subtitle string

	// Activities are the timeline items.
	Activities []TimelineActivity

	// Footnote is an optional footnote.
	Footnote string

	// StartDate overrides the auto-calculated start date.
	StartDate time.Time

	// EndDate overrides the auto-calculated end date.
	EndDate time.Time

	// ShowToday draws a vertical line at today's date.
	ShowToday bool

	// TodayLabel is the label for the today line.
	TodayLabel string

	// TimeUnit is the display unit: "day", "week", "month", "quarter", "year".
	// Default: auto-detected based on date range.
	TimeUnit string
}

// TimelineConfig holds configuration for timeline diagrams.
type TimelineConfig struct {
	ChartConfig

	// RowHeight is the height of each activity row.
	RowHeight float64

	// RowSpacing is the vertical spacing between rows.
	RowSpacing float64

	// BarHeight is the height of activity bars.
	BarHeight float64

	// BarCornerRadius is the corner radius for activity bars.
	BarCornerRadius float64

	// MilestoneSize is the size of milestone diamonds.
	MilestoneSize float64

	// ActivityFillColor is the default fill color for activities.
	ActivityFillColor Color

	// ActivityStrokeColor is the default stroke color for activities.
	ActivityStrokeColor Color

	// MilestoneFillColor is the default fill color for milestones.
	MilestoneFillColor Color

	// PhaseFillColor is the default fill color for phases.
	PhaseFillColor Color

	// TodayLineColor is the color of the "today" line.
	TodayLineColor Color

	// ShowLabels enables labels on activities.
	ShowLabels bool

	// LabelPosition is where labels appear: "inside", "left", "right", "above", "below".
	LabelPosition string

	// ShowProgress enables progress bar rendering on activities.
	ShowProgress bool

	// ProgressColor is the color for the progress fill.
	ProgressColor Color

	// TimeAxisHeight is the height of the time axis area.
	TimeAxisHeight float64

	// ShowTimeGrid enables vertical grid lines at time intervals.
	ShowTimeGrid bool

	// TimeGridColor is the color for time grid lines.
	TimeGridColor Color
}

// DefaultTimelineConfig returns default configuration for timeline diagrams.
func DefaultTimelineConfig(width, height float64) TimelineConfig {
	return TimelineConfig{
		ChartConfig:         DefaultChartConfig(width, height),
		RowHeight:           40,
		RowSpacing:          8,
		BarHeight:           24,
		BarCornerRadius:     4,
		MilestoneSize:       16,
		ActivityFillColor:   MustParseColor("#4E79A7"),
		ActivityStrokeColor: MustParseColor("#3D5A80"),
		MilestoneFillColor:  MustParseColor("#E15759"),
		PhaseFillColor:      MustParseColor("#E8F4FD"),
		TodayLineColor:      MustParseColor("#E15759"),
		ShowLabels:          true,
		LabelPosition:       "inside",
		ShowProgress:        false,
		ProgressColor:       MustParseColor("#59A14F"),
		TimeAxisHeight:      30,
		ShowTimeGrid:        true,
		TimeGridColor:       MustParseColor("#E5E5E5"),
	}
}

// TimelineChart renders timeline diagrams.
type TimelineChart struct {
	builder          *SVGBuilder
	config           TimelineConfig
	accentColors     []Color // theme-aware accent colors, set in Draw()
	showDescriptions bool    // false when vertical space is too tight for descriptions
}

// NewTimelineChart creates a new timeline chart renderer.
func NewTimelineChart(builder *SVGBuilder, config TimelineConfig) *TimelineChart {
	return &TimelineChart{
		builder: builder,
		config:  config,
	}
}

// timelineRange holds the calculated date range for the timeline.
type timelineRange struct {
	start    time.Time
	end      time.Time
	duration time.Duration
	// dataStart/dataEnd are the unpadded data boundaries, used to
	// filter axis ticks so padding doesn't introduce extra labels.
	dataStart time.Time
	dataEnd   time.Time
}

// dateToX converts a date to an X coordinate.
func (tr timelineRange) dateToX(date time.Time, plotArea Rect) float64 {
	if tr.duration == 0 {
		return plotArea.X + plotArea.W/2
	}
	elapsed := date.Sub(tr.start)
	ratio := float64(elapsed) / float64(tr.duration)
	return plotArea.X + ratio*plotArea.W
}

// Draw renders the timeline diagram.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (tc *TimelineChart) Draw(data TimelineData) error {
	if len(data.Activities) == 0 {
		return fmt.Errorf("timeline has no activities to render")
	}

	b := tc.builder
	style := b.StyleGuide()

	// Override hardcoded config colors with theme palette so diagrams
	// respect the active template's color scheme.
	tc.config.ProgressColor = style.Palette.Success
	tc.config.TodayLineColor = style.Palette.Error
	tc.config.TimeGridColor = style.Palette.Border
	tc.config.PhaseFillColor = style.Palette.Accent1.WithAlpha(0.15)
	tc.config.ActivityFillColor = style.Palette.Accent1
	tc.config.ActivityStrokeColor = style.Palette.Accent1.Darken(0.2)
	tc.config.MilestoneFillColor = style.Palette.Error

	// Build accent color palette for theme-aware rendering
	tc.accentColors = style.Palette.AccentColors()
	if len(tc.accentColors) < 3 {
		tc.accentColors = []Color{
			MustParseColor("#4E79A7"),
			MustParseColor("#59A14F"),
			MustParseColor("#F28E2B"),
			MustParseColor("#E15759"),
			MustParseColor("#76B7B2"),
			MustParseColor("#B07AA1"),
		}
	}

	// Calculate plot area
	plotArea := tc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if tc.config.ShowTitle && data.Title != "" {
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

	// Adjust for time axis at bottom
	timeAxisHeight := tc.config.TimeAxisHeight

	// Check if any activities have descriptions — if so, reserve
	// extra vertical space above the time axis so description text
	// below centered bars doesn't collide with the axis labels.
	hasDescriptions := false
	for _, act := range data.Activities {
		if act.Description != "" && act.Type != TimelineActivityTypePhase {
			hasDescriptions = true
			break
		}
	}
	descBuffer := 0.0
	if hasDescriptions {
		// The description rect is positioned below the centered bar and
		// can extend past the row boundary. Reserve enough buffer to
		// prevent overlap with the time axis labels (4 lines to match
		// the desc rect height in drawActivityDescription).
		descBuffer = style.Typography.SizeBody*4*style.Typography.LineHeight + style.Spacing.SM
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight + timeAxisHeight + descBuffer

	// Calculate date range
	dateRange := tc.calculateDateRange(data)

	// Detect time unit if not specified
	timeUnit := data.TimeUnit
	if timeUnit == "" {
		timeUnit = tc.detectTimeUnit(dateRange)
	}

	// Assign rows to activities
	rowAssignments := tc.assignRows(data.Activities, dateRange, plotArea)

	// Count non-phase activities to determine space requirements.
	numNonPhase := 0
	for _, act := range data.Activities {
		if act.Type != TimelineActivityTypePhase {
			numNonPhase++
		}
	}

	// When the vertical space is too tight to render both bars and
	// descriptions legibly, drop descriptions to reclaim the descBuffer
	// height. This happens frequently in narrow two-column placeholders
	// where the 2:1 timeline aspect ratio produces a very short SVG.
	const minHeightPerBarWithDesc = 30.0 // minimum height per activity for bars + descriptions
	if hasDescriptions && plotArea.H < float64(numNonPhase)*minHeightPerBarWithDesc {
		plotArea.H += descBuffer
		descBuffer = 0
		hasDescriptions = false
	}
	tc.showDescriptions = hasDescriptions

	// Remember original plot bottom for time axis placement.
	// The time axis is placed after the plot area + description buffer.
	timeAxisY := plotArea.Y + plotArea.H + descBuffer

	// Scale row dimensions to fill the plot area vertically
	maxRow := 0
	for _, r := range rowAssignments {
		if r > maxRow {
			maxRow = r
		}
	}
	numRows := maxRow + 1

	// --- Adaptive row sizing (proportional scaling like gantt charts) ---
	// Minimum bar height below which bars become unreadable.
	const minBarHeight = 8.0

	// Compute how many rows can physically fit at the minimum bar height.
	// Each row needs at least minBarHeight plus a small minimum spacing.
	minRowSpacing := math.Max(2, tc.config.RowSpacing*0.25)
	minRowUnit := minBarHeight + minRowSpacing
	if hasDescriptions {
		// Descriptions add roughly body-text height below each bar (up to 4 lines).
		minRowUnit += style.Typography.SizeBody*3 + style.Spacing.XS
	}
	maxVisibleRows := int(plotArea.H / minRowUnit)
	if maxVisibleRows < 1 {
		maxVisibleRows = 1
	}

	// Cap visible events when there are too many rows to fit.
	overflowCount := 0
	if numRows > maxVisibleRows {
		overflowCount = numRows - maxVisibleRows
		numRows = maxVisibleRows
	}

	// Proportionally scale row dimensions to fill the available area,
	// mirroring the gantt chart approach: compute total at defaults,
	// then uniformly scale to fit plotArea.H.
	totalHeight := float64(numRows)*tc.config.RowHeight + float64(numRows-1)*tc.config.RowSpacing
	if totalHeight > 0 {
		scale := plotArea.H / totalHeight
		tc.config.RowHeight *= scale
		tc.config.RowSpacing *= scale
		tc.config.BarHeight *= scale
		tc.config.MilestoneSize *= scale
	}

	// Cap bar height so charts with few tasks don't produce absurdly tall bars.
	const maxBarHeight = 60.0
	if tc.config.BarHeight > maxBarHeight {
		capScale := maxBarHeight / tc.config.BarHeight
		tc.config.RowHeight *= capScale
		tc.config.RowSpacing *= capScale
		tc.config.BarHeight = maxBarHeight
		tc.config.MilestoneSize *= capScale
	}

	// Floor bar height at the readability minimum.
	if tc.config.BarHeight < minBarHeight {
		tc.config.BarHeight = minBarHeight
	}

	// When descriptions exist, the row is taller than the bar.
	if hasDescriptions {
		descExtra := style.Typography.SizeBody*4 + style.Spacing.XS
		if tc.config.RowHeight < tc.config.BarHeight+descExtra {
			tc.config.RowHeight = tc.config.BarHeight + descExtra
		}
	} else if tc.config.RowHeight < tc.config.BarHeight {
		tc.config.RowHeight = tc.config.BarHeight
	}

	// Ensure milestone size stays reasonable relative to bar height.
	if tc.config.MilestoneSize > tc.config.BarHeight*0.8 {
		tc.config.MilestoneSize = tc.config.BarHeight * 0.8
	}
	if tc.config.MilestoneSize < 6 {
		tc.config.MilestoneSize = 6
	}

	// Ensure row spacing doesn't go below minimum.
	if tc.config.RowSpacing < minRowSpacing {
		tc.config.RowSpacing = minRowSpacing
	}

	// Center content vertically in the plot area so whitespace is
	// distributed evenly above and below, rather than clustering
	// bars at the top with all whitespace at the bottom.
	totalSpacing := float64(numRows-1) * tc.config.RowSpacing
	totalContentHeight := float64(numRows)*tc.config.RowHeight + totalSpacing
	if totalContentHeight < plotArea.H {
		verticalOffset := (plotArea.H - totalContentHeight) / 2
		plotArea.Y += verticalOffset
		plotArea.H = totalContentHeight
	}

	// Grid lines span from content top to time axis
	gridArea := Rect{X: plotArea.X, Y: plotArea.Y, W: plotArea.W, H: timeAxisY - plotArea.Y}
	if tc.config.ShowTimeGrid {
		tc.drawTimeGrid(dateRange, timeUnit, gridArea)
	}

	// Draw phases first (background)
	for i, activity := range data.Activities {
		if activity.Type == TimelineActivityTypePhase {
			tc.drawPhase(activity, i, rowAssignments[i], dateRange, plotArea)
		}
	}

	// Draw today line if enabled
	if data.ShowToday {
		tc.drawTodayLine(data.TodayLabel, dateRange, plotArea)
	}

	// Pre-compute per-activity label budgets based on actual X-position
	// spacing to neighbors on the same row. This prevents label overlap
	// when activities cluster in a narrow date range.
	activityBudgets := tc.computePerActivityBudgets(data.Activities, rowAssignments, dateRange, plotArea)

	// Pre-compute which activities need their label below (instead of above)
	// to avoid horizontal overlap with adjacent items on the same row.
	// This applies to both milestones and activity bars.
	labelBelow := tc.computeLabelStagger(data.Activities, rowAssignments, dateRange, plotArea, activityBudgets)

	// Draw activities (skip overflow rows)
	for i, activity := range data.Activities {
		if activity.Type != TimelineActivityTypePhase {
			row := rowAssignments[i]
			if row >= numRows {
				continue // skip activities in overflow rows
			}
			labelBudget := activityBudgets[i]
			if labelBudget <= 0 {
				labelBudget = plotArea.W
			}
			if activity.Type == TimelineActivityTypeMilestone {
				tc.drawMilestone(activity, i, row, dateRange, plotArea, labelBudget, labelBelow[i])
			} else {
				tc.drawActivityBar(activity, i, row, dateRange, plotArea, labelBudget, labelBelow[i])
			}
		}
	}

	// Draw overflow indicator when events were capped
	if overflowCount > 0 {
		overflowText := fmt.Sprintf("+%d more event", overflowCount)
		if overflowCount > 1 {
			overflowText += "s"
		}
		b.Push()
		b.SetFontSize(style.Typography.SizeSmall)
		b.SetFontWeight(style.Typography.WeightMedium)
		b.SetTextColor(style.Palette.TextSecondary)
		// Place the overflow indicator just below the last visible row.
		overflowY := plotArea.Y + plotArea.H + style.Spacing.XS
		b.DrawText(overflowText, plotArea.X+plotArea.W/2, overflowY, TextAlignCenter, TextBaselineTop)
		b.Pop()
	}

	// Collect activity dates so the axis can prioritise labelling months
	// where milestones/activities actually occur.
	var activityDates []time.Time
	for _, activity := range data.Activities {
		if !activity.Date.IsZero() {
			activityDates = append(activityDates, activity.Date)
		}
		if !activity.StartDate.IsZero() {
			activityDates = append(activityDates, activity.StartDate)
		}
	}

	// Draw time axis at original bottom position
	tc.drawTimeAxis(dateRange, timeUnit, Rect{
		X: plotArea.X,
		Y: timeAxisY,
		W: plotArea.W,
		H: timeAxisHeight,
	}, activityDates)

	// Draw title
	if tc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: tc.config.Width, H: headerHeight + tc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: tc.config.Height - footerHeight,
			W: tc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// calculateDateRange determines the timeline's date range.
func (tc *TimelineChart) calculateDateRange(data TimelineData) timelineRange {
	var minDate, maxDate time.Time

	// Use explicit dates if provided
	if !data.StartDate.IsZero() {
		minDate = data.StartDate
	}
	if !data.EndDate.IsZero() {
		maxDate = data.EndDate
	}

	// Calculate from activities if not provided
	for _, activity := range data.Activities {
		var actStart, actEnd time.Time

		switch activity.Type {
		case TimelineActivityTypeMilestone:
			actStart = activity.Date
			actEnd = activity.Date
		default:
			actStart = activity.StartDate
			actEnd = activity.EndDate
		}

		if !actStart.IsZero() {
			if minDate.IsZero() || actStart.Before(minDate) {
				minDate = actStart
			}
		}
		if !actEnd.IsZero() {
			if maxDate.IsZero() || actEnd.After(maxDate) {
				maxDate = actEnd
			}
		}
	}

	// Store unpadded data boundaries for tick filtering
	dataStart := minDate
	dataEnd := maxDate

	// Add padding (5% on each side) for visual breathing room
	duration := maxDate.Sub(minDate)
	padding := time.Duration(float64(duration) * 0.05)
	if padding < 24*time.Hour {
		padding = 24 * time.Hour
	}

	minDate = minDate.Add(-padding)
	maxDate = maxDate.Add(padding)

	return timelineRange{
		start:     minDate,
		end:       maxDate,
		duration:  maxDate.Sub(minDate),
		dataStart: dataStart,
		dataEnd:   dataEnd,
	}
}

// detectTimeUnit determines the appropriate time unit for the axis.
func (tc *TimelineChart) detectTimeUnit(dateRange timelineRange) string {
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

// assignRows assigns row numbers to activities, avoiding overlap.
func (tc *TimelineChart) assignRows(activities []TimelineActivity, dateRange timelineRange, plotArea Rect) []int {
	rows := make([]int, len(activities))
	rowEndTimes := make([]time.Time, 0) // tracks when each row becomes free

	// Sort activities by start date for consistent assignment
	indices := make([]int, len(activities))
	for i := range indices {
		indices[i] = i
	}
	slices.SortFunc(indices, func(a, b int) int {
		aa, ab := activities[a], activities[b]
		startA := aa.StartDate
		if aa.Type == TimelineActivityTypeMilestone {
			startA = aa.Date
		}
		startB := ab.StartDate
		if ab.Type == TimelineActivityTypeMilestone {
			startB = ab.Date
		}
		return startA.Compare(startB)
	})

	for _, idx := range indices {
		activity := activities[idx]

		// If row is pre-assigned, use it
		if activity.Row >= 0 {
			rows[idx] = activity.Row
			// Extend rowEndTimes if needed
			for len(rowEndTimes) <= activity.Row {
				rowEndTimes = append(rowEndTimes, time.Time{})
			}
			if activity.Type == TimelineActivityTypeMilestone {
				if activity.Date.After(rowEndTimes[activity.Row]) {
					rowEndTimes[activity.Row] = activity.Date.Add(time.Hour)
				}
			} else if activity.EndDate.After(rowEndTimes[activity.Row]) {
				rowEndTimes[activity.Row] = activity.EndDate
			}
			continue
		}

		// Find a free row
		var actStart, actEnd time.Time
		if activity.Type == TimelineActivityTypeMilestone {
			actStart = activity.Date
			actEnd = activity.Date.Add(time.Hour) // Milestones have minimal width
		} else {
			actStart = activity.StartDate
			actEnd = activity.EndDate
		}

		assignedRow := -1
		for r, endTime := range rowEndTimes {
			if endTime.IsZero() || actStart.After(endTime) || actStart.Equal(endTime) {
				assignedRow = r
				break
			}
		}

		if assignedRow == -1 {
			assignedRow = len(rowEndTimes)
			rowEndTimes = append(rowEndTimes, time.Time{})
		}

		rows[idx] = assignedRow
		rowEndTimes[assignedRow] = actEnd
	}

	return rows
}

// drawActivityBar draws an activity bar.
// labelBudget is the horizontal space allocated per activity for label sizing.
// labelBelow overrides the label position to "below" when staggering is needed
// to prevent horizontal overlap with adjacent activities on the same row.
func (tc *TimelineChart) drawActivityBar(activity TimelineActivity, index int, row int, dateRange timelineRange, plotArea Rect, labelBudget float64, labelBelow bool) {
	b := tc.builder
	style := b.StyleGuide()

	// Calculate position
	startX := dateRange.dateToX(activity.StartDate, plotArea)
	endX := dateRange.dateToX(activity.EndDate, plotArea)
	barWidth := endX - startX
	if barWidth < 10 {
		barWidth = 10 // Minimum bar width
	}

	rowY := plotArea.Y + float64(row)*(tc.config.RowHeight+tc.config.RowSpacing)
	barY := rowY + (tc.config.RowHeight-tc.config.BarHeight)/2

	// Determine colors - use theme accent colors, cycling by activity index
	fillColor := tc.accentColors[index%len(tc.accentColors)]
	strokeColor := fillColor.Darken(0.2)
	if activity.Color != nil {
		fillColor = *activity.Color
		strokeColor = activity.Color.Darken(0.2)
	}

	rect := Rect{
		X: startX,
		Y: barY,
		W: barWidth,
		H: tc.config.BarHeight,
	}

	// Draw bar background
	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeColor(strokeColor)
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawRoundedRect(rect, tc.config.BarCornerRadius)
	b.Pop()

	// Draw progress if enabled
	if tc.config.ShowProgress && activity.Progress > 0 {
		progressWidth := barWidth * (activity.Progress / 100.0)
		progressRect := Rect{
			X: startX,
			Y: barY,
			W: progressWidth,
			H: tc.config.BarHeight,
		}
		b.Push()
		b.SetFillColor(tc.config.ProgressColor)
		b.SetStrokeWidth(0)
		b.DrawRoundedRect(progressRect, tc.config.BarCornerRadius)
		b.Pop()
	}

	// Draw icon inside the bar (left side) if present.
	if activity.Icon != "" {
		tc.drawTimelineIcon(activity.Icon, activity.Label, fillColor,
			startX+tc.config.BarHeight/2, barY+tc.config.BarHeight/2,
			tc.config.BarHeight*0.4)
	}

	// Draw label
	if tc.config.ShowLabels && activity.Label != "" {
		tc.drawActivityLabel(activity.Label, rect, row, plotArea, fillColor, labelBudget, labelBelow)
	}

	// Draw description below the bar (if present and space permits).
	// Descriptions are disabled by Draw() when vertical space is too tight
	// (e.g., narrow two-column placeholders) to prevent illegible text.
	if tc.showDescriptions && activity.Description != "" {
		tc.drawActivityDescription(activity.Description, rect, fillColor, labelBudget)
	}
}

// drawActivityLabel draws the label for an activity.
// labelBudget is the horizontal space allocated per activity for label sizing.
// labelBelow overrides the label position to "below" when staggering is needed
// to prevent horizontal overlap with adjacent activities on the same row.
func (tc *TimelineChart) drawActivityLabel(label string, barRect Rect, row int, plotArea Rect, bgColor Color, labelBudget float64, labelBelow bool) {
	b := tc.builder
	style := b.StyleGuide()

	b.Push()
	b.SetFontWeight(style.Typography.WeightMedium)

	// Compute available width for the label.
	availW := barRect.W
	if labelBudget > availW {
		availW = labelBudget
	}
	maxLabelW := availW * 0.85

	// Use shared LabelFitStrategy for consistent sizing across diagram types.
	fit := DefaultLabelFit(style.Typography).Fit(b, label, maxLabelW, 0)
	displayLabel := fit.DisplayText

	labelY := barRect.Y + barRect.H/2

	// Determine effective position, applying stagger override.
	pos := tc.config.LabelPosition
	if labelBelow && (pos == "" || pos == "inside" || pos == "above") {
		pos = "below"
	}

	switch pos {
	case "inside":
		b.SetTextColor(style.Palette.TextPrimary)
		labelX := clampCenterLabelX(b, displayLabel, barRect.X+barRect.W/2, plotArea)
		b.DrawText(displayLabel, labelX, barRect.Y-style.Spacing.XS, TextAlignCenter, TextBaselineBottom)

	case "right":
		// Draw to the right of the bar; budget is the remaining width.
		rightBudget := (plotArea.X + plotArea.W) - (barRect.X + barRect.W + style.Spacing.SM)
		if rightBudget > 0 {
			rFit := DefaultLabelFit(style.Typography).Fit(b, label, rightBudget*0.95, 0)
			displayLabel = rFit.DisplayText
		}
		labelX := barRect.X + barRect.W + style.Spacing.SM
		b.DrawText(displayLabel, labelX, labelY, TextAlignLeft, TextBaselineMiddle)

	case "above":
		b.SetTextColor(style.Palette.TextPrimary)
		labelX := clampCenterLabelX(b, displayLabel, barRect.X+barRect.W/2, plotArea)
		labelY = barRect.Y - style.Spacing.XS
		b.DrawText(displayLabel, labelX, labelY, TextAlignCenter, TextBaselineBottom)

	case "below":
		labelX := clampCenterLabelX(b, displayLabel, barRect.X+barRect.W/2, plotArea)
		labelY = barRect.Y + barRect.H + style.Spacing.XS
		b.DrawText(displayLabel, labelX, labelY, TextAlignCenter, TextBaselineTop)

	default: // "left"
		// Draw to the left of the bar (in margin)
		leftBudget := barRect.X - plotArea.X
		if leftBudget > 0 {
			lFit := DefaultLabelFit(style.Typography).Fit(b, label, leftBudget*0.95, 0)
			displayLabel = lFit.DisplayText
		}
		labelX := plotArea.X - style.Spacing.SM
		b.DrawText(displayLabel, labelX, labelY, TextAlignRight, TextBaselineMiddle)
	}

	b.Pop()
}

// drawActivityDescription draws the description text below an activity bar.
// labelBudget is the horizontal space allocated per activity for text sizing.
func (tc *TimelineChart) drawActivityDescription(desc string, barRect Rect, barColor Color, labelBudget float64) {
	b := tc.builder
	style := b.StyleGuide()

	b.Push()
	availW := barRect.W
	if labelBudget > availW {
		availW = labelBudget
	}
	maxDescW := availW * 0.95

	// Use shared LabelFitStrategy for consistent sizing across diagram types.
	// Descriptions use SizeBody with wrapping for readability.
	strat := DescriptionLabelFit(style.Typography)
	descH := strat.PreferredSize*float64(strat.MaxLines)*style.Typography.LineHeight + style.Spacing.XS
	fit := strat.Fit(b, desc, maxDescW, descH)

	b.SetFontSize(fit.FontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextSecondary)

	descRect := Rect{
		X: barRect.X + barRect.W/2 - maxDescW/2,
		Y: barRect.Y + barRect.H + style.Spacing.XS,
		W: maxDescW,
		H: descH,
	}
	b.DrawWrappedText(desc, descRect, AlignTopCenter)
	b.Pop()
}

// computePerActivityBudgets calculates label width budgets for each non-phase
// activity based on actual X-position spacing to neighbors on the same row.
// This prevents labels from being sized wider than the gap between adjacent
// activities, which causes overlap when activities cluster in a narrow date range.
func (tc *TimelineChart) computePerActivityBudgets(activities []TimelineActivity, rowAssignments []int, dateRange timelineRange, plotArea Rect) map[int]float64 {
	type actInfo struct {
		idx int
		x   float64 // center X of the activity
	}
	rowActs := make(map[int][]actInfo)

	for i, act := range activities {
		if act.Type == TimelineActivityTypePhase {
			continue
		}
		var x float64
		if act.Type == TimelineActivityTypeMilestone {
			x = dateRange.dateToX(act.Date, plotArea)
		} else {
			startX := dateRange.dateToX(act.StartDate, plotArea)
			endX := dateRange.dateToX(act.EndDate, plotArea)
			x = (startX + endX) / 2
		}
		rowActs[rowAssignments[i]] = append(rowActs[rowAssignments[i]], actInfo{idx: i, x: x})
	}

	budgets := make(map[int]float64)
	for _, acts := range rowActs {
		if len(acts) == 1 {
			budgets[acts[0].idx] = plotArea.W
			continue
		}
		slices.SortFunc(acts, func(a, b actInfo) int {
			if a.x < b.x {
				return -1
			}
			if a.x > b.x {
				return 1
			}
			return 0
		})

		for j, a := range acts {
			var leftGap, rightGap float64
			if j == 0 {
				leftGap = a.x - plotArea.X
			} else {
				leftGap = (a.x - acts[j-1].x) / 2
			}
			if j == len(acts)-1 {
				rightGap = (plotArea.X + plotArea.W) - a.x
			} else {
				rightGap = (acts[j+1].x - a.x) / 2
			}
			budget := leftGap + rightGap
			budgets[a.idx] = budget
		}
	}

	return budgets
}

// computeLabelStagger determines which activities need their labels
// rendered below (instead of above) to prevent horizontal overlap.
// Works for both milestones and activity bars on the same row.
// Returns a map from activity index to true if the label should be below.
func (tc *TimelineChart) computeLabelStagger(activities []TimelineActivity, rowAssignments []int, dateRange timelineRange, plotArea Rect, budgets map[int]float64) map[int]bool {
	result := make(map[int]bool)
	style := tc.builder.StyleGuide()

	// Only stagger for label positions that render above (the default).
	pos := tc.config.LabelPosition
	if pos != "" && pos != "inside" && pos != "above" {
		return result
	}

	// Group all non-phase activity indices by row, sorted by X position.
	type actInfo struct {
		actIdx int
		x      float64
		labelW float64
	}
	rowItems := make(map[int][]actInfo)

	b := tc.builder
	for i, act := range activities {
		if act.Type == TimelineActivityTypePhase {
			continue
		}
		var x float64
		if act.Type == TimelineActivityTypeMilestone {
			x = dateRange.dateToX(act.Date, plotArea)
		} else {
			startX := dateRange.dateToX(act.StartDate, plotArea)
			endX := dateRange.dateToX(act.EndDate, plotArea)
			x = (startX + endX) / 2
		}

		// Estimate label width using the per-activity budget.
		maxLabelW := budgets[i] * 0.85
		b.Push()
		b.SetFontWeight(style.Typography.WeightMedium)
		fit := DefaultLabelFit(style.Typography).Fit(b, act.Label, maxLabelW, 0)
		labelW, _ := b.MeasureText(fit.DisplayText)
		b.Pop()

		rowItems[rowAssignments[i]] = append(rowItems[rowAssignments[i]], actInfo{
			actIdx: i,
			x:      x,
			labelW: labelW,
		})
	}

	// For each row with multiple items, check for overlaps and stagger.
	for _, items := range rowItems {
		if len(items) < 2 {
			continue
		}
		// Sort by X position.
		slices.SortFunc(items, func(a, b actInfo) int {
			if a.x < b.x {
				return -1
			}
			if a.x > b.x {
				return 1
			}
			return 0
		})

		// Check if any consecutive pair overlaps. A label centered at x
		// with width w occupies [x-w/2, x+w/2].
		minGap := style.Spacing.SM
		hasOverlap := false
		for j := 1; j < len(items); j++ {
			prev := items[j-1]
			curr := items[j]
			rightEdgePrev := prev.x + prev.labelW/2
			leftEdgeCurr := curr.x - curr.labelW/2
			if leftEdgeCurr-rightEdgePrev < minGap {
				hasOverlap = true
				break
			}
		}

		// When overlap is detected, alternate every other item below.
		if hasOverlap {
			for j, m := range items {
				if j%2 == 1 {
					result[m.actIdx] = true
				}
			}
		}
	}

	return result
}

// drawMilestone draws a milestone diamond.
// labelBudget is the horizontal space allocated per activity for label sizing.
// labelBelow overrides the label position to "below" when staggering is needed
// to prevent horizontal overlap with adjacent milestones on the same row.
func (tc *TimelineChart) drawMilestone(activity TimelineActivity, index int, row int, dateRange timelineRange, plotArea Rect, labelBudget float64, labelBelow bool) {
	b := tc.builder
	style := b.StyleGuide()

	// Calculate position
	x := dateRange.dateToX(activity.Date, plotArea)
	rowY := plotArea.Y + float64(row)*(tc.config.RowHeight+tc.config.RowSpacing)
	y := rowY + tc.config.RowHeight/2

	// Diamond half-size
	halfSize := tc.config.MilestoneSize / 2

	// Determine color - use theme accent colors, cycling by activity index
	fillColor := tc.accentColors[index%len(tc.accentColors)]
	if activity.Color != nil {
		fillColor = *activity.Color
	}

	// Draw diamond
	points := []Point{
		{X: x, Y: y - halfSize}, // Top
		{X: x + halfSize, Y: y}, // Right
		{X: x, Y: y + halfSize}, // Bottom
		{X: x - halfSize, Y: y}, // Left
	}

	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeColor(fillColor.Darken(0.2))
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawPolygon(points)
	b.Pop()

	// Draw icon centered on the diamond if present.
	if activity.Icon != "" {
		tc.drawTimelineIcon(activity.Icon, activity.Label, fillColor,
			x, y, halfSize*0.7)
	}

	// Draw label
	if tc.config.ShowLabels && activity.Label != "" {
		b.Push()
		b.SetFontWeight(style.Typography.WeightMedium)

		// Use shared LabelFitStrategy for consistent sizing across diagram types.
		maxLabelW := labelBudget * 0.85
		fit := DefaultLabelFit(style.Typography).Fit(b, activity.Label, maxLabelW, 0)
		displayLabel := fit.DisplayText

		// Determine effective label position. When staggering is active
		// (labelBelow=true), override "inside"/"above" to "below" so
		// adjacent milestones alternate above/below to avoid overlap.
		pos := tc.config.LabelPosition
		if labelBelow && (pos == "inside" || pos == "above" || pos == "") {
			pos = "below"
		}

		switch pos {
		case "inside":
			// Milestones have no interior, so render the label above the
			// diamond (same approach as activity bars with "inside" labels).
			b.SetTextColor(style.Palette.TextPrimary)
			labelY := y - halfSize - style.Spacing.XS
			labelX := clampCenterLabelX(b, displayLabel, x, plotArea)
			b.DrawText(displayLabel, labelX, labelY, TextAlignCenter, TextBaselineBottom)
		case "above":
			labelY := y - halfSize - style.Spacing.XS
			labelX := clampCenterLabelX(b, displayLabel, x, plotArea)
			b.DrawText(displayLabel, labelX, labelY, TextAlignCenter, TextBaselineBottom)
		case "below":
			labelY := y + halfSize + style.Spacing.XS
			labelX := clampCenterLabelX(b, displayLabel, x, plotArea)
			b.DrawText(displayLabel, labelX, labelY, TextAlignCenter, TextBaselineTop)
		case "right":
			labelX := x + halfSize + style.Spacing.SM
			b.DrawText(displayLabel, labelX, y, TextAlignLeft, TextBaselineMiddle)
		default: // "left"
			labelX := plotArea.X - style.Spacing.SM
			b.DrawText(displayLabel, labelX, y, TextAlignRight, TextBaselineMiddle)
		}

		b.Pop()
	}
}

// drawTimelineIcon draws an icon (emoji or loaded image) at the given center
// position. Emoji icons are drawn as text; loadable icons (URL, data URI,
// inline SVG, file path) are rasterized. If neither works, no fallback is
// drawn (the underlying bar/diamond shape serves as visual marker).
func (tc *TimelineChart) drawTimelineIcon(icon, label string, accent Color, cx, cy, size float64) {
	b := tc.builder
	if size < 6 {
		return // too small to be legible
	}

	if IsEmojiIcon(icon) {
		b.Push()
		b.SetFontSize(size)
		b.SetTextColor(Color{R: 1, G: 1, B: 1, A: 1}) // white on colored shape
		b.DrawText(icon, cx, cy, TextAlignCenter, TextBaselineMiddle)
		b.Pop()
	} else if img := LoadIcon(icon, int(size*2)); img != nil {
		r := Rect{
			X: cx - size/2,
			Y: cy - size/2,
			W: size,
			H: size,
		}
		b.DrawImage(img, r)
	}
}

// clampCenterLabelX adjusts a center-aligned label's X position so the text
// stays within the plotArea bounds. Without this, labels near the left or right
// edge of the timeline can be clipped by the SVG viewport.
func clampCenterLabelX(b *SVGBuilder, text string, x float64, plotArea Rect) float64 {
	w, _ := b.MeasureText(text)
	halfW := w / 2
	minX := plotArea.X + halfW
	maxX := plotArea.X + plotArea.W - halfW
	if x < minX {
		return minX
	}
	if x > maxX {
		return maxX
	}
	return x
}

// drawPhase draws a phase background band.
func (tc *TimelineChart) drawPhase(activity TimelineActivity, index int, row int, dateRange timelineRange, plotArea Rect) {
	b := tc.builder
	style := b.StyleGuide()

	// Calculate position
	startX := dateRange.dateToX(activity.StartDate, plotArea)
	endX := dateRange.dateToX(activity.EndDate, plotArea)
	width := endX - startX

	// Determine color - use theme accent colors with transparency
	accentColor := tc.accentColors[index%len(tc.accentColors)]
	fillColor := accentColor.WithAlpha(0.15)
	if activity.Color != nil {
		fillColor = activity.Color.WithAlpha(0.2)
	}

	rect := Rect{
		X: startX,
		Y: plotArea.Y,
		W: width,
		H: plotArea.H,
	}

	// Draw phase background
	b.Push()
	b.SetFillColor(fillColor)
	b.SetStrokeWidth(0)
	b.DrawRect(rect)
	b.Pop()

	// Draw phase label at top, wrapping within the phase band width.
	// Use SM padding per side (instead of XS) so adjacent phase labels
	// have a visible gap even when SVG renderers use wider font metrics
	// than Go's text measurement.
	if activity.Label != "" {
		hPad := style.Spacing.SM // per-side horizontal padding
		labelW := width - 2*hPad
		if labelW >= style.Typography.SizeBody*2 { // skip if phase too narrow
			b.Push()
			b.SetFontSize(style.Typography.SizeBody)
			b.SetFontWeight(style.Typography.WeightBold)
			// Use the accent color (darkened) for phase label text for theme consistency
			b.SetTextColor(accentColor.Darken(0.3))
			if activity.Color != nil {
				b.SetTextColor(activity.Color.Darken(0.3))
			}
			labelRect := Rect{
				X: startX + hPad,
				Y: plotArea.Y + style.Spacing.SM,
				W: labelW,
				H: style.Typography.SizeBody * 3, // allow up to ~2 lines
			}
			b.DrawWrappedText(activity.Label, labelRect, AlignTopCenter)
			b.Pop()
		}
	}
}

// drawTodayLine draws a vertical line at today's date.
func (tc *TimelineChart) drawTodayLine(label string, dateRange timelineRange, plotArea Rect) {
	b := tc.builder
	style := b.StyleGuide()

	now := time.Now()
	x := dateRange.dateToX(now, plotArea)

	// Draw vertical line
	b.Push()
	b.SetStrokeColor(tc.config.TodayLineColor)
	b.SetStrokeWidth(style.Strokes.WidthThick)
	b.SetDashes(4, 4)
	b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)
	b.Pop()

	// Draw label
	if label == "" {
		label = "Today"
	}
	b.Push()
	b.SetFontSize(style.Typography.SizeSmall)
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetTextColor(tc.config.TodayLineColor)
	b.DrawText(label, x, plotArea.Y-style.Spacing.XS, TextAlignCenter, TextBaselineBottom)
	b.Pop()
}

// drawTimeGrid draws vertical grid lines at time intervals.
func (tc *TimelineChart) drawTimeGrid(dateRange timelineRange, timeUnit string, plotArea Rect) {
	b := tc.builder
	style := b.StyleGuide()

	ticks := tc.generateTimeTicks(dateRange, timeUnit)

	b.Push()
	b.SetStrokeColor(tc.config.TimeGridColor)
	b.SetStrokeWidth(style.Strokes.WidthThin)

	for _, tick := range ticks {
		x := dateRange.dateToX(tick, plotArea)
		b.DrawLine(x, plotArea.Y, x, plotArea.Y+plotArea.H)
	}

	b.Pop()
}

// drawTimeAxis draws the time axis with labels.
// activityDates are dates where activities/milestones occur. When label
// thinning is active (labelStep > 1), the axis will prioritise showing
// labels at these dates so the axis aligns with visible chart elements.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (tc *TimelineChart) drawTimeAxis(dateRange timelineRange, timeUnit string, axisArea Rect, activityDates []time.Time) {
	b := tc.builder
	style := b.StyleGuide()

	ticks := tc.generateTimeTicks(dateRange, timeUnit)

	// Draw axis line
	b.Push()
	b.SetStrokeColor(style.Palette.TextSecondary)
	b.SetStrokeWidth(style.Strokes.WidthNormal)
	b.DrawLine(axisArea.X, axisArea.Y, axisArea.X+axisArea.W, axisArea.Y)
	b.Pop()

	// Compute label step to prevent overlap on narrow canvases.
	// Measure the widest tick label and compare to available spacing.
	labelStep := 1
	fontSize := style.Typography.SizeSmall
	if len(ticks) >= 2 {
		// Measure widest label
		b.Push()
		b.SetFontSize(fontSize)
		var maxLabelW float64
		for _, tick := range ticks {
			label := tc.formatTickLabel(tick, timeUnit)
			w, _ := b.MeasureText(label)
			if w > maxLabelW {
				maxLabelW = w
			}
		}
		b.Pop()

		// Add padding between labels (1.5x safety factor for font measurement)
		labelWidthWithPad := maxLabelW * 1.5
		// Average spacing between ticks
		avgSpacing := axisArea.W / float64(len(ticks))
		if avgSpacing > 0 && labelWidthWithPad > avgSpacing {
			// Need to thin: show every Nth label so they don't overlap
			labelStep = int(math.Ceil(labelWidthWithPad / avgSpacing))
			if labelStep < 1 {
				labelStep = 1
			}
		}
	}

	// Build the set of labels to show. When labelStep > 1 and we have
	// activity dates, prioritise axis labels at activity-aligned ticks.
	// This ensures milestone/bar dates are always readable on the axis.
	showLabel := make([]bool, len(ticks))

	if labelStep <= 1 {
		// No thinning needed — show all labels
		for i := range showLabel {
			showLabel[i] = true
		}
	} else {
		// Start with the default step-based labels
		for i := range ticks {
			if i%labelStep == 0 {
				showLabel[i] = true
			}
		}

		// Force-show labels at activity-aligned ticks and suppress
		// neighbouring regular labels to prevent overlap.
		if len(activityDates) > 0 {
			activityAligned := make([]bool, len(ticks))
			for i, tick := range ticks {
				for _, ad := range activityDates {
					if tickMatchesTimeUnit(tick, ad, timeUnit) {
						activityAligned[i] = true
						break
					}
				}
			}

			for i := range ticks {
				if !activityAligned[i] || showLabel[i] {
					continue // already shown or not a priority tick
				}
				showLabel[i] = true
				// Suppress the nearest non-activity labels within labelStep
				// distance on each side to make room.
				for delta := 1; delta < labelStep; delta++ {
					if j := i - delta; j >= 0 && showLabel[j] && !activityAligned[j] {
						showLabel[j] = false
					}
					if j := i + delta; j < len(ticks) && showLabel[j] && !activityAligned[j] {
						showLabel[j] = false
					}
				}
			}
		}
	}

	// Draw tick marks and labels
	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)

	for i, tick := range ticks {
		x := dateRange.dateToX(tick, Rect{X: axisArea.X, Y: 0, W: axisArea.W, H: 0})

		// Draw tick mark (always)
		b.SetStrokeColor(style.Palette.TextSecondary)
		b.DrawLine(x, axisArea.Y, x, axisArea.Y+6)

		// Draw label based on computed visibility
		if showLabel[i] {
			label := tc.formatTickLabel(tick, timeUnit)
			b.DrawText(label, x, axisArea.Y+style.Spacing.SM, TextAlignCenter, TextBaselineTop)
		}
	}

	b.Pop()
}

// generateTimeTicks generates tick marks for the time axis.
// Ticks are limited to the unpadded data range so padding doesn't
// introduce extra labels beyond the actual data boundaries.
func (tc *TimelineChart) generateTimeTicks(dateRange timelineRange, timeUnit string) []time.Time {
	var ticks []time.Time

	// Use the unpadded data range for tick boundaries if available
	tickStart := dateRange.start
	tickEnd := dateRange.end
	if !dateRange.dataStart.IsZero() {
		tickStart = dateRange.dataStart
		tickEnd = dateRange.dataEnd
	}

	current := tc.roundToTimeUnit(tickStart, timeUnit)
	for current.Before(tickEnd) || current.Equal(tickEnd) {
		if current.After(tickStart) || current.Equal(tickStart) {
			ticks = append(ticks, current)
		}
		current = tc.advanceTimeUnit(current, timeUnit)
	}

	return ticks
}

// roundToTimeUnit rounds a time down to the nearest time unit.
func (tc *TimelineChart) roundToTimeUnit(t time.Time, timeUnit string) time.Time {
	switch timeUnit {
	case "day":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "week":
		// Round to Monday
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, t.Location())
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case "quarter":
		quarter := (int(t.Month()) - 1) / 3
		return time.Date(t.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, t.Location())
	case "year":
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// advanceTimeUnit advances a time by one time unit.
func (tc *TimelineChart) advanceTimeUnit(t time.Time, timeUnit string) time.Time {
	switch timeUnit {
	case "day":
		return t.AddDate(0, 0, 1)
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	case "quarter":
		return t.AddDate(0, 3, 0)
	case "year":
		return t.AddDate(1, 0, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// tickMatchesTimeUnit reports whether two times fall in the same time unit.
// For example, with timeUnit "month", two dates in June 2026 match regardless
// of their day values.
func tickMatchesTimeUnit(a, b time.Time, timeUnit string) bool {
	switch timeUnit {
	case "day":
		return a.Year() == b.Year() && a.YearDay() == b.YearDay()
	case "week":
		ay, aw := a.ISOWeek()
		by, bw := b.ISOWeek()
		return ay == by && aw == bw
	case "month":
		return a.Year() == b.Year() && a.Month() == b.Month()
	case "quarter":
		return a.Year() == b.Year() && (int(a.Month())-1)/3 == (int(b.Month())-1)/3
	case "year":
		return a.Year() == b.Year()
	default:
		return a.Equal(b)
	}
}

// formatTickLabel formats a tick mark label.
func (tc *TimelineChart) formatTickLabel(t time.Time, timeUnit string) string {
	switch timeUnit {
	case "day":
		return t.Format("Jan 2")
	case "week":
		return t.Format("Jan 2")
	case "month":
		return t.Format("Jan '06")
	case "quarter":
		quarter := (int(t.Month())-1)/3 + 1
		return fmt.Sprintf("Q%d '%02d", quarter, t.Year()%100)
	case "year":
		return t.Format("2006")
	default:
		return t.Format("Jan 2")
	}
}

// =============================================================================
// Timeline Diagram Type (for Registry)
// =============================================================================

// Timeline implements the Diagram interface for timeline diagrams.
type Timeline struct{ BaseDiagram }

// Validate checks that the request data is valid for timeline diagrams.
func (d *Timeline) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("timeline requires data. Expected format: {\"activities\": [{\"name\": \"Phase 1\", \"start\": \"2024-01\", \"end\": \"2024-03\"}]}")
	}

	// Accept "events" as alias for "activities"
	if _, hasEvents := req.Data["events"]; hasEvents {
		if _, hasActivities := req.Data["activities"]; !hasActivities {
			req.Data["activities"] = req.Data["events"]
		}
	}

	// Check that at least one of the known activity keys exists
	// Use toAnySlice to handle both []any and []map[string]any
	_, hasActivities := toAnySlice(req.Data["activities"])
	_, hasItems := toAnySlice(req.Data["items"])
	_, hasPhases := toAnySlice(req.Data["phases"])
	_, hasMilestones := toAnySlice(req.Data["milestones"])
	if !hasActivities && !hasItems && !hasPhases && !hasMilestones {
		return fmt.Errorf("timeline requires 'activities', 'events', 'items', 'phases', or 'milestones' in data. Expected: {\"activities\": [{\"name\": \"Phase 1\", \"start\": \"2024-01\", \"end\": \"2024-03\"}]} or {\"milestones\": [{\"name\": \"Launch\", \"date\": \"2024-06\"}]}")
	}
	return nil
}

// Render generates an SVG document from the request envelope.
func (d *Timeline) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
// This allows callers to generate PNG/PDF output from the same builder.
func (d *Timeline) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelperDimensions(req, 800, 400, func(builder *SVGBuilder, req *RequestEnvelope) error {
		// Dense date labels and task bars need compact typography
		builder.StyleGuide().Typography = CompactTypography().ScaleForDimensions(builder.Width(), builder.Height())

		data, err := parseTimelineData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultTimelineConfig(width, height)

		// Apply custom settings from data
		if showToday, ok := req.Data["show_today"].(bool); ok {
			data.ShowToday = showToday
		}
		if todayLabel, ok := req.Data["today_label"].(string); ok {
			data.TodayLabel = todayLabel
		}
		if timeUnit, ok := req.Data["time_unit"].(string); ok {
			data.TimeUnit = timeUnit
		}
		if showProgress, ok := req.Data["show_progress"].(bool); ok {
			config.ShowProgress = showProgress
		}
		if showLabels, ok := req.Data["show_labels"].(bool); ok {
			config.ShowLabels = showLabels
		}
		if labelPos, ok := req.Data["label_position"].(string); ok {
			config.LabelPosition = labelPos
		}

		chart := NewTimelineChart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseTimelineData parses the request data into TimelineData.
func parseTimelineData(req *RequestEnvelope) (TimelineData, error) {
	data := TimelineData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse activities - also handle "items" and "phases" as aliases
	// Use toAnySlice to handle both []any and []map[string]any
	activitiesRaw, ok := toAnySlice(req.Data["activities"])
	if !ok {
		// Try "items" as an alias (markdown format uses items)
		activitiesRaw, ok = toAnySlice(req.Data["items"])
	}
	if !ok {
		// Try "phases" as an alias (common in generated/user data)
		activitiesRaw, ok = toAnySlice(req.Data["phases"])
	}
	if ok {
		data.Activities = make([]TimelineActivity, 0, len(activitiesRaw))
		for i, aRaw := range activitiesRaw {
			activity := parseTimelineActivity(aRaw, i)
			data.Activities = append(data.Activities, activity)
		}
	}

	// Parse milestones (convenience alias)
	if milestonesRaw, ok := toAnySlice(req.Data["milestones"]); ok {
		for i, mRaw := range milestonesRaw {
			activity := parseTimelineActivity(mRaw, len(data.Activities)+i)
			activity.Type = TimelineActivityTypeMilestone
			data.Activities = append(data.Activities, activity)
		}
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	// Parse date range overrides
	if startStr, ok := req.Data["start_date"].(string); ok {
		if t, err := parseDate(startStr); err == nil {
			data.StartDate = t
		}
	}
	if endStr, ok := req.Data["end_date"].(string); ok {
		if t, err := parseDate(endStr); err == nil {
			data.EndDate = t
		}
	}

	// Auto-assign evenly spaced dates to dateless activities so they
	// render in a readable horizontal layout instead of collapsing to
	// a single point. This handles the common pattern where milestones
	// use descriptive text in the "date" field rather than real dates.
	data.Activities = autoAssignDatelessActivities(data.Activities)

	return data, nil
}

// parseTimelineActivity parses a single activity from map data.
//nolint:gocognit,gocyclo // complex chart rendering logic
func parseTimelineActivity(raw any, index int) TimelineActivity {
	activity := TimelineActivity{
		ID:   fmt.Sprintf("activity_%d", index),
		Type: TimelineActivityTypeActivity,
		Row:  -1, // Auto-assign
	}

	switch a := raw.(type) {
	case string:
		// Simple string form - just a label
		activity.Label = a
	case map[string]any:
		if id, ok := a["id"].(string); ok {
			activity.ID = id
		}
		// Handle "label", "title", and "name" aliases
		if label, ok := a["label"].(string); ok {
			activity.Label = label
		} else if title, ok := a["title"].(string); ok {
			activity.Label = title
		} else if name, ok := a["name"].(string); ok {
			activity.Label = name
		} else if event, ok := a["event"].(string); ok {
			activity.Label = event
		}
		if desc, ok := a["description"].(string); ok {
			activity.Description = desc
		}
		if icon, ok := a["icon"].(string); ok {
			activity.Icon = icon
		}
		if typ, ok := a["type"].(string); ok {
			activity.Type = TimelineActivityType(typ)
		}
		if colorStr, ok := a["color"].(string); ok {
			if c, err := ParseColor(colorStr); err == nil {
				activity.Color = &c
			}
		}
		if row, ok := a["row"].(float64); ok {
			activity.Row = int(row)
		}
		if progress, ok := a["progress"].(float64); ok {
			activity.Progress = progress
		}

		// Parse dates — accept both "start_date"/"end_date" and the
		// shorter "start"/"end" aliases used by the gantt format.
		if startStr, ok := a["start_date"].(string); ok {
			if t, err := parseDate(startStr); err == nil {
				activity.StartDate = t
			}
		}
		if activity.StartDate.IsZero() {
			if startStr, ok := a["start"].(string); ok {
				if t, err := parseDate(startStr); err == nil {
					activity.StartDate = t
				}
			}
		}
		if endStr, ok := a["end_date"].(string); ok {
			if t, err := parseDate(endStr); err == nil {
				activity.EndDate = t
			}
		}
		if activity.EndDate.IsZero() {
			if endStr, ok := a["end"].(string); ok {
				if t, err := parseDate(endStr); err == nil {
					activity.EndDate = t
				}
			}
		}
		if dateStr, ok := a["date"].(string); ok {
			if start, end, err := parseDateRange(dateStr); err == nil {
				if start.Equal(end) {
					// Point-in-time date: use as StartDate for
					// activities when no explicit start_date was
					// provided.  This handles the common JSON
					// pattern {"date": "2026-01-01", "end_date":
					// "2026-06-30", "type": "activity"} where
					// "date" is the activity start.  For
					// milestones the Date field is also set.
					activity.Date = start
					if activity.StartDate.IsZero() {
						activity.StartDate = start
					}
				} else {
					// Range date (e.g. "Mar 2026" → Mar 1 to Mar 31):
					// fill in start/end when not already set.
					if activity.StartDate.IsZero() {
						activity.StartDate = start
					}
					if activity.EndDate.IsZero() {
						activity.EndDate = end
					}
					// For milestones, also set Date to the midpoint
					// of the range so they render at the correct
					// X position (milestones use Date, not StartDate).
					if activity.Date.IsZero() {
						mid := start.Add(end.Sub(start) / 2)
						activity.Date = mid
					}
				}
			} else if activity.Label == "" {
				// Date string is not a parseable date — use it as the
				// label. This handles the common pattern where
				// milestones use {"date": "Phase 1: Foundation", "event": "..."}
				// and "date" is really a phase name, not a date.
				activity.Label = dateStr
			}
		}
	}

	return activity
}

// autoAssignDatelessActivities detects activities/milestones with no parseable
// dates and assigns them evenly spaced synthetic dates so the timeline renders
// a clean horizontal layout. If some activities have real dates and others
// don't, the dateless ones are spaced across the existing date range.
func autoAssignDatelessActivities(activities []TimelineActivity) []TimelineActivity {
	// Find which activities are dateless
	var datelessIdx []int
	var minDate, maxDate time.Time
	for i, act := range activities {
		if hasDate(act) {
			d := effectiveDate(act)
			if minDate.IsZero() || d.Before(minDate) {
				minDate = d
			}
			e := effectiveEndDate(act)
			if maxDate.IsZero() || e.After(maxDate) {
				maxDate = e
			}
		} else {
			datelessIdx = append(datelessIdx, i)
		}
	}

	if len(datelessIdx) == 0 {
		return activities
	}

	// If ALL activities are dateless, create a synthetic range
	// spanning one month per activity for readable spacing.
	if minDate.IsZero() {
		now := time.Now().UTC()
		minDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		maxDate = minDate.AddDate(0, len(datelessIdx), 0)
	}

	// Space dateless activities evenly across the range
	n := len(datelessIdx)
	duration := maxDate.Sub(minDate)
	if duration <= 0 {
		duration = time.Duration(n) * 30 * 24 * time.Hour
		// maxDate not updated here; only duration is needed for spacing below.
	}

	for j, idx := range datelessIdx {
		// Position at (j+1)/(n+1) to avoid placing on exact boundaries
		frac := float64(j+1) / float64(n+1)
		t := minDate.Add(time.Duration(float64(duration) * frac))
		act := &activities[idx]
		if act.Type == TimelineActivityTypeMilestone {
			act.Date = t
		} else {
			// For activities, give them a small duration window
			act.StartDate = t
			act.EndDate = t.AddDate(0, 0, 14)
		}
	}

	return activities
}

// hasDate returns true if the activity has any parseable date set.
func hasDate(act TimelineActivity) bool {
	return !act.Date.IsZero() || !act.StartDate.IsZero() || !act.EndDate.IsZero()
}

// effectiveDate returns the earliest date for an activity.
func effectiveDate(act TimelineActivity) time.Time {
	if !act.StartDate.IsZero() {
		return act.StartDate
	}
	return act.Date
}

// effectiveEndDate returns the latest date for an activity.
func effectiveEndDate(act TimelineActivity) time.Time {
	if !act.EndDate.IsZero() {
		return act.EndDate
	}
	if !act.Date.IsZero() {
		return act.Date
	}
	return act.StartDate
}

// =============================================================================
// Convenience Functions
// =============================================================================

// DrawTimelineFromActivities creates and draws a simple timeline.
func DrawTimelineFromActivities(builder *SVGBuilder, title string, activities []TimelineActivity) error {
	config := DefaultTimelineConfig(builder.Width(), builder.Height())
	chart := NewTimelineChart(builder, config)
	return chart.Draw(TimelineData{
		Title:      title,
		Activities: activities,
	})
}

// CreateSimpleTimeline creates a timeline with activities from labels and date ranges.
func CreateSimpleTimeline(items []struct {
	Label string
	Start time.Time
	End   time.Time
}) []TimelineActivity {
	activities := make([]TimelineActivity, len(items))
	for i, item := range items {
		activities[i] = TimelineActivity{
			ID:        fmt.Sprintf("activity_%d", i),
			Label:     item.Label,
			Type:      TimelineActivityTypeActivity,
			StartDate: item.Start,
			EndDate:   item.End,
			Row:       -1,
		}
	}
	return activities
}

