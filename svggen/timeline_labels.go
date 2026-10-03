package svggen

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// Event-label layout for the timeline (go-slide-creator-ze6qs).
//
// Labels drawn above or below their marker used to be fitted one at a time
// against half the distance to the neighbouring marker, shrunk to one shared
// size and then cut with an ellipsis — "Frontend D…" beside an empty canvas.
// The plan here places whole labels instead: on one line above the markers,
// then staggered above / below, then wrapped onto two lines, then at a smaller
// shared size, and only when every one of those fails is a label shortened,
// which is reported at a severity that blocks a clean render.

// timelineLabelPlan is how one event label is drawn.
type timelineLabelPlan struct {
	lines     []string // one or two lines; the whole label unless truncated
	below     bool     // on the lane below the marker (band: the second label line)
	cx        float64  // centre x, kept inside the plot area
	width     float64  // widest line
	truncated bool
}

// timelineLabelItem is one label to place: its activity and marker centre.
type timelineLabelItem struct {
	idx   int
	x     float64
	label string
}

// timelineLabelRoom reports how many label lines fit above and below the
// markers of a row. nil means two everywhere.
type timelineLabelRoom func(row int, below bool) int

// timelineLabelsPlanned reports whether the configured label position is one
// the planner lays out (above the marker, staggering below it). The "left",
// "right" and "below" positions keep their per-label fit.
func (tc *TimelineChart) timelineLabelsPlanned() bool {
	if !tc.config.ShowLabels {
		return false
	}
	switch tc.config.LabelPosition {
	case "", "inside", "above":
		return true
	}
	return false
}

// barSpan returns the drawn x and width of an activity bar. A bar with no
// usable end is drawn at the minimum width from its start.
func (tc *TimelineChart) barSpan(act TimelineActivity, dateRange timelineRange, plotArea Rect) (x, w float64) {
	const minBarWidth = 10.0
	x = dateRange.dateToX(act.StartDate, plotArea)
	w = minBarWidth
	if !act.EndDate.IsZero() {
		w = math.Max(dateRange.dateToX(act.EndDate, plotArea)-x, minBarWidth)
	}
	return x, w
}

// markerCenterX is the x an event's label is centred on: the milestone's
// date, or the middle of the drawn bar.
func (tc *TimelineChart) markerCenterX(act TimelineActivity, dateRange timelineRange, plotArea Rect) float64 {
	if act.Type == TimelineActivityTypeMilestone {
		return dateRange.dateToX(act.Date, plotArea)
	}
	x, w := tc.barSpan(act, dateRange, plotArea)
	return x + w/2
}

// labelFloorSize is the smallest shared label size: one cramped label must
// not take every label below the small size.
func (tc *TimelineChart) labelFloorSize() float64 {
	typo := tc.builder.StyleGuide().Typography
	return math.Max(math.Min(typo.SizeSmall, typo.SizeBody), tc.builder.MinFontSize())
}

// labelLineHeight is the height of one label line at the given size.
func (tc *TimelineChart) labelLineHeight(size float64) float64 {
	b := tc.builder
	style := b.StyleGuide()
	b.Push()
	b.SetFontSize(size)
	b.SetFontWeight(style.Typography.WeightMedium)
	_, lineH := b.MeasureText("Hg")
	b.Pop()
	return math.Max(lineH, size) * style.Typography.LineHeight
}

// planEventLabels places every event label and returns the plans by activity
// index with the one size they are all drawn at. Sizes are tried from the body
// size down to the floor; the first at which every row places whole labels
// wins. At the floor a row that still cannot is shortened (plan.truncated).
func (tc *TimelineChart) planEventLabels(activities []TimelineActivity, rowAssignments []int, dateRange timelineRange, plotArea Rect, room timelineLabelRoom) (map[int]timelineLabelPlan, float64) {
	b := tc.builder
	style := b.StyleGuide()

	rows := map[int][]timelineLabelItem{}
	var rowOrder []int
	for i, act := range activities {
		if act.Type == TimelineActivityTypePhase || act.Label == "" {
			continue
		}
		row := rowAssignments[i]
		if _, seen := rows[row]; !seen {
			rowOrder = append(rowOrder, row)
		}
		rows[row] = append(rows[row], timelineLabelItem{idx: i, x: tc.markerCenterX(act, dateRange, plotArea), label: act.Label})
	}
	if len(rowOrder) == 0 {
		return nil, 0
	}
	slices.Sort(rowOrder)
	for _, row := range rowOrder {
		slices.SortStableFunc(rows[row], func(a, c timelineLabelItem) int {
			switch {
			case a.x < c.x:
				return -1
			case a.x > c.x:
				return 1
			}
			return 0
		})
	}
	lines := func(row int, below bool) int {
		if room == nil {
			return 2
		}
		return min(max(room(row, below), 1), 2)
	}

	floor := tc.labelFloorSize()
	preferred := math.Max(style.Typography.SizeBody, floor)
	b.Push()
	defer b.Pop()
	b.SetFontWeight(style.Typography.WeightMedium)
	for size := preferred; ; size = math.Max(size-0.5, floor) {
		b.SetFontSize(size)
		plans := map[int]timelineLabelPlan{}
		whole := true
		for _, row := range rowOrder {
			placed, ok := tc.placeLabelRow(rows[row], plotArea, lines(row, false), lines(row, true))
			whole = whole && ok
			for idx, p := range placed {
				plans[idx] = p
			}
		}
		if whole || size <= floor {
			return plans, size
		}
	}
}

// timelineLabelRow places the labels of one row at the builder's current font.
type timelineLabelRow struct {
	b                      *SVGBuilder
	items                  []timelineLabelItem // sorted by x
	plotArea               Rect
	gap                    float64 // clear space kept between neighbouring labels
	linesAbove, linesBelow int     // lines each lane has room for
}

// placeLabelRow places one row's labels at the builder's current font. It
// reports false when a label had to be shortened.
func (tc *TimelineChart) placeLabelRow(items []timelineLabelItem, plotArea Rect, linesAbove, linesBelow int) (map[int]timelineLabelPlan, bool) {
	row := timelineLabelRow{
		b: tc.builder, items: items, plotArea: plotArea, gap: tc.builder.StyleGuide().Spacing.SM,
		linesAbove: linesAbove, linesBelow: linesBelow,
	}
	// Every label on one line: all above, then alternating above / below.
	for _, stagger := range []bool{false, true} {
		if plans, ok := row.onOneLine(stagger); ok {
			return plans, true
		}
	}
	if plans, ok := row.firstFit(); ok {
		return plans, true
	}
	return row.inSlots()
}

func (r timelineLabelRow) oneLine(it timelineLabelItem, below bool) timelineLabelPlan {
	w, _ := r.b.MeasureText(it.label)
	return timelineLabelPlan{lines: []string{it.label}, below: below, width: w}
}

// twoLines wraps a label at its most balanced space. It reports false when
// the lane has no room for a second line or the label has no space.
func (r timelineLabelRow) twoLines(it timelineLabelItem, below bool) (timelineLabelPlan, bool) {
	if (below && r.linesBelow < 2) || (!below && r.linesAbove < 2) {
		return timelineLabelPlan{}, false
	}
	first, second, ok := splitLabelBalanced(r.b, it.label)
	if !ok {
		return timelineLabelPlan{}, false
	}
	w1, _ := r.b.MeasureText(first)
	w2, _ := r.b.MeasureText(second)
	return timelineLabelPlan{lines: []string{first, second}, below: below, width: math.Max(w1, w2)}, true
}

// place centres a plan on its marker inside the plot area and reports
// whether it clears lastHi, the right edge of the lane's previous label.
func (r timelineLabelRow) place(it timelineLabelItem, p timelineLabelPlan, lastHi float64) (timelineLabelPlan, bool) {
	if p.width > r.plotArea.W {
		return p, false
	}
	p.cx = math.Min(math.Max(it.x, r.plotArea.X+p.width/2), r.plotArea.X+r.plotArea.W-p.width/2)
	return p, p.cx-p.width/2 >= lastHi+r.gap
}

// onOneLine puts every label on one line: all above the markers, or with
// stagger every second one below.
func (r timelineLabelRow) onOneLine(stagger bool) (map[int]timelineLabelPlan, bool) {
	plans := map[int]timelineLabelPlan{}
	hi := [2]float64{math.Inf(-1), math.Inf(-1)}
	for j, it := range r.items {
		lane := 0
		if stagger && j%2 == 1 {
			lane = 1
		}
		p, fits := r.place(it, r.oneLine(it, lane == 1), hi[lane])
		if !fits {
			return nil, false
		}
		hi[lane] = p.cx + p.width/2
		plans[it.idx] = p
	}
	return plans, true
}

// firstFit gives each label, left to right, the first placement that clears
// its lane: one line above, one line below, two lines above, two lines below.
func (r timelineLabelRow) firstFit() (map[int]timelineLabelPlan, bool) {
	plans := map[int]timelineLabelPlan{}
	hi := [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, it := range r.items {
		placed := false
		for attempt := 0; attempt < 4 && !placed; attempt++ {
			lane, wrap := attempt%2, attempt >= 2
			p, can := r.oneLine(it, lane == 1), true
			if wrap {
				p, can = r.twoLines(it, lane == 1)
			}
			if !can {
				continue
			}
			if p, placed = r.place(it, p, hi[lane]); placed {
				hi[lane] = p.cx + p.width/2
				plans[it.idx] = p
			}
		}
		if !placed {
			return nil, false
		}
	}
	return plans, true
}

// inSlots alternates the lanes and keeps each label inside the slot between
// its lane neighbours' markers. A label wider than its slot wraps, and one
// that still does not fit is shortened: the only placement that loses text.
func (r timelineLabelRow) inSlots() (map[int]timelineLabelPlan, bool) {
	plans := map[int]timelineLabelPlan{}
	whole := true
	var lanes [2][]timelineLabelItem
	for j, it := range r.items {
		lanes[j%2] = append(lanes[j%2], it)
	}
	for lane, items := range lanes {
		for j, it := range items {
			lo, hi := r.plotArea.X, r.plotArea.X+r.plotArea.W
			if j > 0 {
				lo = (items[j-1].x+it.x)/2 + r.gap/2
			}
			if j < len(items)-1 {
				hi = (it.x+items[j+1].x)/2 - r.gap/2
			}
			p := r.inSlot(it, lane == 1, math.Max(hi-lo, 0))
			whole = whole && !p.truncated
			p.cx = math.Min(math.Max(it.x, lo+p.width/2), math.Max(hi-p.width/2, lo+p.width/2))
			plans[it.idx] = p
		}
	}
	return plans, whole
}

// inSlot fits one label in a slot: on one line, wrapped, or shortened.
func (r timelineLabelRow) inSlot(it timelineLabelItem, below bool, slot float64) timelineLabelPlan {
	p := r.oneLine(it, below)
	if p.width <= slot {
		return p
	}
	if wrapped, can := r.twoLines(it, below); can && wrapped.width <= slot {
		return wrapped
	}
	short := r.b.TruncateToWidth(it.label, slot)
	w, _ := r.b.MeasureText(short)
	return timelineLabelPlan{lines: []string{short}, below: below, width: w, truncated: short != it.label}
}

// splitLabelBalanced breaks a label at the space that leaves the two lines
// closest in width. It reports false for a label with no space to break at.
func splitLabelBalanced(b *SVGBuilder, label string) (first, second string, ok bool) {
	words := strings.Fields(label)
	if len(words) < 2 {
		return "", "", false
	}
	best := math.Inf(1)
	for i := 1; i < len(words); i++ {
		l1, l2 := strings.Join(words[:i], " "), strings.Join(words[i:], " ")
		w1, _ := b.MeasureText(l1)
		w2, _ := b.MeasureText(l2)
		if w := math.Max(w1, w2); w < best {
			best, first, second = w, l1, l2
		}
	}
	return first, second, true
}

// reportShortenedLabels raises one finding per label the plan could not show
// in full. It is not advisory: the slide has lost source text.
func (tc *TimelineChart) reportShortenedLabels(activities []TimelineActivity, plans map[int]timelineLabelPlan, size float64) {
	for i, act := range activities {
		p, ok := plans[i]
		if !ok || !p.truncated {
			continue
		}
		tc.builder.AddFinding(Finding{
			Field: act.field,
			Code:  FindingLabelTruncated,
			Message: fmt.Sprintf("timeline label %q does not fit beside its neighbours even staggered and wrapped at %.1fpt, and was cut to %q — shorten the label, use fewer events or spread their dates",
				act.Label, size, strings.Join(p.lines, " ")),
			Severity: core.SeverityShrinkOrSplit,
			Fix: &FixSuggestion{
				Kind:   FixKindTruncateOrSplit,
				Params: map[string]any{"original": act.Label, "truncated": strings.Join(p.lines, " "), "font_size": size, "diagram_type": "timeline"},
			},
		})
	}
}

// drawPlannedLabel draws a planned label above markerTop or below
// markerBottom.
func (tc *TimelineChart) drawPlannedLabel(p timelineLabelPlan, markerTop, markerBottom float64) {
	b := tc.builder
	style := b.StyleGuide()
	lineH := tc.labelLineHeight(tc.labelFontSize)

	b.Push()
	defer b.Pop()
	b.SetFontSize(tc.labelFontSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.SetTextColor(style.Palette.TextPrimary)
	baseline := TextBaselineTop
	y := markerBottom + style.Spacing.XS
	if !p.below {
		baseline = TextBaselineBottom
		y = markerTop - style.Spacing.XS - float64(len(p.lines)-1)*lineH
	}
	for _, line := range p.lines {
		b.DrawText(line, p.cx, y, TextAlignCenter, baseline)
		y += lineH
	}
}

// fitEventLabel is the per-label fit of the label positions the planner does
// not lay out ("left", "right", "below"). A truncation it reports is raised to
// a blocking severity: a shortened event label is source loss whichever side
// of the marker it sits on.
func (tc *TimelineChart) fitEventLabel(label string, maxW float64) LabelFitResult {
	b := tc.builder
	before := len(b.findings)
	fit := DefaultLabelFit(b.StyleGuide().Typography).Fit(b, label, maxW, 0)
	for i := before; i < len(b.findings); i++ {
		if b.findings[i].Code == FindingLabelTruncated {
			b.findings[i].Severity = core.SeverityShrinkOrSplit
		}
	}
	return fit
}

// planDescriptionSpans gives each described bar the horizontal span its
// description wraps in: the slot between the neighbouring described bars of
// its row, kept centred on the bar unless the bar is the first or last of the
// row, where the span runs on to the plot edge. A bar wider than its slot
// keeps its own width.
func (tc *TimelineChart) planDescriptionSpans(activities []TimelineActivity, rowAssignments []int, dateRange timelineRange, plotArea Rect) map[int][2]float64 {
	type described struct {
		idx  int
		x, w float64 // bar centre and width
	}
	rows := map[int][]described{}
	for i, act := range activities {
		if act.Description == "" || act.Type == TimelineActivityTypePhase || act.Type == TimelineActivityTypeMilestone {
			continue
		}
		x, w := tc.barSpan(act, dateRange, plotArea)
		rows[rowAssignments[i]] = append(rows[rowAssignments[i]], described{idx: i, x: x + w/2, w: w})
	}
	spans := map[int][2]float64{}
	for _, items := range rows {
		slices.SortStableFunc(items, func(a, c described) int {
			switch {
			case a.x < c.x:
				return -1
			case a.x > c.x:
				return 1
			}
			return 0
		})
		for j, it := range items {
			lo, hi := plotArea.X, plotArea.X+plotArea.W
			first, last := j == 0, j == len(items)-1
			if !first {
				lo = (items[j-1].x + it.x) / 2
			}
			if !last {
				hi = (it.x + items[j+1].x) / 2
			}
			// Centred on the bar, except at the ends of the row.
			half := math.Min(it.x-lo, hi-it.x)
			switch {
			case first && last:
				// Alone on its row: at least half the plot, shifted inside it.
				w := math.Min(math.Max(2*half, plotArea.W/2), plotArea.W)
				lo = math.Min(math.Max(it.x-w/2, plotArea.X), plotArea.X+plotArea.W-w)
				hi = lo + w
			case first && !last:
				lo = math.Max(lo, it.x-(hi-it.x))
			case last && !first:
				hi = math.Min(hi, it.x+(it.x-lo))
			default:
				lo, hi = it.x-half, it.x+half
			}
			if hi-lo < it.w {
				lo, hi = it.x-it.w/2, it.x+it.w/2
			}
			spans[it.idx] = [2]float64{lo, hi - lo}
		}
	}
	return spans
}
