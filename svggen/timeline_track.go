package svggen

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// =============================================================================
// Timeline: the milestone track (go-slide-creator-o8cqh)
// =============================================================================
//
// A timeline used to draw each dated event as a short accent bar floating
// above a ruled axis, with its label somewhere above or below and month
// gridlines running through the text: nothing tied a label to its mark, and
// the picture read as a chart that had lost its data. The track draws what a
// timeline is:
//
//   - one weighted axis line across the canvas;
//   - a disc ON the axis for every moment, placed to scale;
//   - a bar hanging from the axis for every item with a duration, drawn to
//     scale from its start to its end;
//   - one label block per item (bold name, its date beneath, the description
//     under that) joined to its mark by a leader. All blocks of a lane hang
//     at one distance from the axis, so leaders of a lane have one length.
//
// Moments are labelled above the axis and durations below it. A timeline of
// moments only uses both sides: a block that would run into its neighbour
// moves to the other side, then to a second lane further out. Only when no
// lane has room is a label narrowed and cut, which is reported as
// chart.label_truncated (shrink_or_split) exactly as before.
//
// The former row-and-grid layout remains for what it alone draws: progress
// shading, an authored label_position, explicit rows, phase bands and icons
// (TimelineConfig.LegacyLayout, timelineNeedsLegacy).

// Track proportions, in multiples of the body type size unless noted.
const (
	trackEdgePad        = 4.0 // canvas edge to the drawing, points
	trackLineFactor     = 1.3 // line pitch
	trackTitleStep      = 14.0 / 12.0
	trackTitleStepMaxN  = 6    // more items than this set titles at the body size
	trackAxisWidth      = 3.0  // points
	trackDiscRadius     = 0.5  // moment marker
	trackDiscRing       = 2.0  // background ring round a marker, points
	trackBarHeight      = 1.5  // duration bar
	trackBarGap         = 0.25 // between stacked bar lanes
	trackLeader         = 1.4  // mark to the nearest lane's blocks
	trackLeaderGap      = 0.3  // leader end to text
	trackLaneGap        = 0.8  // between lanes on one side
	trackBlockGap       = 1.0  // least space between neighbouring blocks
	trackMinBlockWidth  = 6.0
	trackMaxBlockWidth  = 16.0
	trackDescMeasure    = 12.0 // a description alone widens its block up to this
	trackMaxTitleLines  = 2
	trackMaxDescLines   = 3
	trackMaxLanes       = 3 // per side
	trackLeaderWidth    = 1.0
	trackEdgeInsetShare = 0.07 // first and last mark sit this share of the width in from the edge
	trackZoomMaxShare   = 0.75 // a zoomed track uses at most this share of the height
	// trackMeasureSlack widens a block beyond its measured name: the slide
	// sets the text in the template's own face, whose bold can run a tenth
	// wider than the face it was measured in.
	trackMeasureSlack = 1.1
)

// trackZoomSteps are the type sizes a track tries, as multiples of the floor:
// 18, 16, 14 and 12pt on a slide.
var trackZoomSteps = []float64{1.5, 4.0 / 3.0, 7.0 / 6.0, 1}

// timelineNeedsLegacy reports whether the data asks for something only the
// row-and-grid layout draws.
func (tc *TimelineChart) timelineNeedsLegacy(data TimelineData) bool {
	if tc.config.LegacyLayout || tc.config.ShowProgress || !tc.config.ShowLabels {
		return true
	}
	for _, act := range data.Activities {
		if act.Type == TimelineActivityTypePhase || act.Row > 0 || act.Icon != "" || act.Progress > 0 {
			return true
		}
	}
	return false
}

// trackItem is one timeline item resolved for the track.
type trackItem struct {
	index    int // into the activities
	act      TimelineActivity
	isBar    bool
	x0, x1   float64 // mark extent on the axis (equal for a moment)
	barLane  int
	dateText string

	// The label block.
	titleLines []string
	descLines  []string
	truncated  bool
	width      float64
	cx         float64 // block centre
	above      bool
	lane       int
}

func (it trackItem) anchorX() float64 { return (it.x0 + it.x1) / 2 }

func (it trackItem) blockLeft() float64  { return it.cx - it.width/2 }
func (it trackItem) blockRight() float64 { return it.cx + it.width/2 }

// trackType is the type model of a track.
type trackType struct {
	body, title         float64
	bodyLine, titleLine float64
}

// height is the height of an item's label block with at most descLines of
// description.
func (tt trackType) height(it trackItem, descLines int) float64 {
	h := float64(len(it.titleLines)) * tt.titleLine
	if it.dateText != "" {
		h += tt.bodyLine
	}
	return h + float64(min(len(it.descLines), descLines))*tt.bodyLine
}

// drawTrack renders the milestone track.
func (tc *TimelineChart) drawTrack(data TimelineData) error { //nolint:gocognit,gocyclo // one pass per layer of the drawing
	b := tc.builder
	style := b.StyleGuide()
	typ := style.Typography

	floor := math.Max(typ.SizeSmall, b.MinFontSize())

	// Canvas: the drawing runs to the edges; title and footnote keep theirs.
	headerHeight := 0.0
	top := trackEdgePad
	if tc.config.ShowTitle && data.Title != "" {
		headerHeight = typ.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += typ.SizeSubtitle + style.Spacing.XS
		}
		top = tc.config.MarginTop + headerHeight
	}
	footerHeight := 0.0
	if data.Footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}
	plot := Rect{X: trackEdgePad, Y: top, W: math.Max(0, tc.config.Width-2*trackEdgePad), H: math.Max(0, tc.config.Height-top-trackEdgePad-footerHeight)}

	// A sparse timeline is set larger: the biggest step of the type scale at
	// which every label still stands whole in the lane nearest the axis and
	// the drawing keeps to trackZoomMaxShare of the height. A crowded one
	// stays at the floor.
	var (
		tt       trackType
		items    []trackItem
		barLanes int
	)
	for _, zoom := range trackZoomSteps {
		body := floor * zoom
		title := body
		if len(data.Activities) <= trackTitleStepMaxN {
			title = body * trackTitleStep
		}
		tt = trackType{body: body, title: title, bodyLine: body * trackLineFactor, titleLine: title * trackLineFactor}
		items = tc.trackItems(data, plot, tt)
		hasBars := false
		for _, it := range items {
			hasBars = hasBars || it.isBar
		}
		barLanes = trackAssignBarLanes(items)
		tc.trackPlaceBlocks(items, plot, tt, hasBars)
		if zoom == 1 {
			break
		}
		calm := true
		for _, it := range items {
			calm = calm && it.lane == 0 && !it.truncated
		}
		barsH := float64(barLanes) * body * (trackBarHeight + trackBarGap)
		_, above, below := trackSideHeights(items, tt, trackMaxDescLines, barsH)
		if calm && above+below+trackAxisWidth <= plot.H*trackZoomMaxShare {
			break
		}
	}
	body := tt.body

	// Vertical budget: descriptions give way first, a line at a time.
	barH := body * trackBarHeight
	barsH := 0.0
	if barLanes > 0 {
		barsH = float64(barLanes)*barH + float64(barLanes-1)*body*trackBarGap
	}
	descLines := trackMaxDescLines
	var aboveH, belowH float64
	var laneH map[[2]int]float64
	for ; ; descLines-- {
		laneH, aboveH, belowH = trackSideHeights(items, tt, descLines, barsH)
		if aboveH+belowH+trackAxisWidth <= plot.H || descLines == 0 {
			break
		}
	}
	if descLines < trackMaxDescLines {
		tc.reportTrackDescriptions(items, descLines)
	}
	tc.showDescriptions = descLines > 0

	axisY := plot.Y + aboveH + trackAxisWidth/2
	if spare := plot.H - (aboveH + belowH + trackAxisWidth); spare > 0 {
		axisY += spare / 2
	}

	neutral := NeutralInk(style.Palette, tonalRuleShare)
	accent := diagramAccent(style)

	// Leaders first, so marks and text sit on top of them.
	laneOffset := func(above bool, lane int) float64 {
		// Distance from the axis (or the bars' lower edge) to the lane's
		// near edge.
		side := 0
		if above {
			side = 1
		}
		d := body * trackLeader
		for l := 0; l < lane; l++ {
			d += laneH[[2]int{side, l}] + body*trackLaneGap
		}
		return d
	}
	type placed struct {
		it       trackItem
		top, bot float64 // text block extent
	}
	blocks := make([]placed, 0, len(items))
	b.Push()
	b.SetStrokeColor(neutral)
	b.SetStrokeWidth(trackLeaderWidth)
	for _, it := range items {
		h := tt.height(it, descLines)
		var from, to float64
		p := placed{it: it}
		if it.above {
			from = axisY - trackAxisWidth/2
			if !it.isBar {
				from = axisY - body*trackDiscRadius
			}
			p.bot = axisY - trackAxisWidth/2 - laneOffset(true, it.lane)
			p.top = p.bot - h
			to = p.bot + body*trackLeaderGap
		} else {
			from = axisY + body*trackDiscRadius
			base := axisY + trackAxisWidth/2
			if it.isBar {
				from = base + float64(it.barLane+1)*barH + float64(it.barLane)*body*trackBarGap
			}
			p.top = base + barsH + laneOffset(false, it.lane)
			p.bot = p.top + h
			to = p.top - body*trackLeaderGap
		}
		lx := math.Min(math.Max(it.anchorX(), it.blockLeft()), it.blockRight())
		b.DrawLine(lx, from, lx, to)
		blocks = append(blocks, p)
	}
	b.Pop()

	// The axis.
	b.Push()
	b.SetStrokeColor(NeutralInk(style.Palette, tonalSpineShare))
	b.SetStrokeWidth(trackAxisWidth)
	b.DrawLine(plot.X, axisY, plot.X+plot.W, axisY)
	b.Pop()

	// Marks.
	// Bars first: a disc that falls on a bar's end stays whole on top of it.
	barFill := diagramContentFill(style)
	marks := make([]trackItem, 0, len(items))
	for _, it := range items {
		if it.isBar {
			marks = append(marks, it)
		}
	}
	for _, it := range items {
		if !it.isBar {
			marks = append(marks, it)
		}
	}
	for _, it := range marks {
		fill := accent
		if it.act.Color != nil {
			fill = it.act.Color.Opaque()
		}
		b.Push()
		b.SetStrokeWidth(0)
		if it.isBar {
			bf := barFill
			if it.act.Color != nil {
				bf = fill
			}
			y := axisY + trackAxisWidth/2 + float64(it.barLane)*(barH+body*trackBarGap)
			b.SetFillColor(bf)
			b.DrawRect(Rect{X: it.x0, Y: y, W: math.Max(it.x1-it.x0, 2), H: barH})
		} else {
			r := body * trackDiscRadius
			b.SetFillColor(style.Palette.Background.Opaque())
			b.DrawCircle(it.x0, axisY, r+trackDiscRing)
			b.SetFillColor(fill)
			b.DrawCircle(it.x0, axisY, r)
		}
		b.Pop()
	}

	// Today.
	if data.ShowToday && !data.SyntheticDates {
		tc.drawTodayLine(data.TodayLabel, tc.trackRange(data), tc.trackMarkArea(plot))
	}

	// Label blocks: name, date, description.
	ink := style.Palette.TextPrimary
	muted := diagramMutedInk(style)
	for _, p := range blocks {
		it := p.it
		y := p.top
		b.Push()
		b.SetFontSize(tt.title)
		b.SetFontWeight(typ.WeightBold)
		b.SetFillColor(ink)
		for _, line := range it.titleLines {
			b.DrawText(line, it.cx, y+tt.titleLine/2, TextAlignCenter, TextBaselineMiddle)
			y += tt.titleLine
		}
		b.SetFontSize(tt.body)
		b.SetFontWeight(typ.WeightNormal)
		if it.dateText != "" {
			b.SetFillColor(muted)
			b.DrawText(it.dateText, it.cx, y+tt.bodyLine/2, TextAlignCenter, TextBaselineMiddle)
			y += tt.bodyLine
		}
		b.SetFillColor(ink)
		for i, line := range it.descLines {
			if i >= descLines {
				break
			}
			if i == descLines-1 && len(it.descLines) > descLines {
				line = b.TruncateToWidth(line+"…", it.width)
			}
			b.DrawText(line, it.cx, y+tt.bodyLine/2, TextAlignCenter, TextBaselineMiddle)
			y += tt.bodyLine
		}
		b.Pop()
	}

	tc.reportInvalidDates(data)
	tc.reportTrackLabels(items, tt)

	if tc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		NewTitle(b, titleConfig).Draw(Rect{X: 0, Y: 0, W: tc.config.Width, H: headerHeight + tc.config.MarginTop})
	}
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		NewFootnote(b, footnoteConfig).Draw(Rect{X: 0, Y: tc.config.Height - footerHeight, W: tc.config.Width, H: footerHeight})
	}
	return nil
}

// trackMarkArea is the stretch of the axis that marks are mapped onto: the
// plot less an inset at each end, so the first and last label have room.
func (tc *TimelineChart) trackMarkArea(plot Rect) Rect {
	inset := plot.W * trackEdgeInsetShare
	return Rect{X: plot.X + inset, Y: plot.Y, W: math.Max(0, plot.W-2*inset), H: plot.H}
}

// trackRange is the unpadded date range of the data: the first date maps to
// the start of the mark area and the last to its end.
func (tc *TimelineChart) trackRange(data TimelineData) timelineRange {
	start, end := data.StartDate, data.EndDate
	for _, act := range data.Activities {
		s, e := trackDates(act)
		if !s.IsZero() && (start.IsZero() || s.Before(start)) {
			start = s
		}
		if !e.IsZero() && (end.IsZero() || e.After(end)) {
			end = e
		}
	}
	return timelineRange{start: start, end: end, duration: end.Sub(start), dataStart: start, dataEnd: end}
}

// trackItems resolves every activity into a mark and an unplaced label block.
func (tc *TimelineChart) trackItems(data TimelineData, plot Rect, tt trackType) []trackItem {
	b := tc.builder
	area := tc.trackMarkArea(plot)
	dates := tc.trackRange(data)
	n := len(data.Activities)
	maxW := math.Min(tt.body*trackMaxBlockWidth, math.Max(tt.body*trackMinBlockWidth, plot.W/math.Max(1, float64(n))*1.8))

	items := make([]trackItem, 0, n)
	for i, act := range data.Activities {
		it := trackItem{index: i, act: act}
		start, end := trackDates(act)
		it.isBar = end.After(start)
		it.x0 = dates.dateToX(start, area)
		it.x1 = it.x0
		if it.isBar {
			it.x1 = dates.dateToX(end, area)
		}
		if !data.SyntheticDates && !act.undated {
			it.dateText = trackDateText(act, start, end)
		}

		// Width: the name on one line where it fits the cap, else the cap
		// (and the name wraps); never narrower than its date.
		b.Push()
		b.SetFontSize(tt.title)
		b.SetFontWeight(b.StyleGuide().Typography.WeightBold)
		nameW, _ := b.MeasureText(act.Label)
		b.Pop()
		b.Push()
		b.SetFontSize(tt.body)
		dateW, _ := b.MeasureText(it.dateText)
		descW, _ := b.MeasureText(act.Description)
		b.Pop()
		// A description sets the width too, up to a comfortable measure, so
		// a three-word line is not broken under a one-word name.
		want := math.Max(math.Max(nameW*trackMeasureSlack, dateW), math.Min(descW, tt.body*trackDescMeasure))
		it.width = math.Min(maxW, math.Max(want+1, tt.body*trackMinBlockWidth))
		tc.trackWrap(&it, tt)
		items = append(items, it)
	}
	sort.SliceStable(items, func(a, c int) bool { return items[a].anchorX() < items[c].anchorX() })
	return items
}

// trackWrap breaks an item's name and description to its block width.
func (tc *TimelineChart) trackWrap(it *trackItem, tt trackType) {
	b := tc.builder
	it.truncated = false
	b.Push()
	b.SetFontSize(tt.title)
	b.SetFontWeight(b.StyleGuide().Typography.WeightBold)
	lines := fishboneWrap(b, it.act.Label, it.width)
	if len(lines) > trackMaxTitleLines {
		lines = lines[:trackMaxTitleLines]
		lines[trackMaxTitleLines-1] += "…"
		it.truncated = true
	}
	for i, line := range lines {
		if w, _ := b.MeasureText(line); w > it.width {
			lines[i] = b.TruncateToWidth(line, it.width)
			it.truncated = true
		}
	}
	it.titleLines = lines
	b.Pop()

	b.Push()
	b.SetFontSize(tt.body)
	b.SetFontWeight(b.StyleGuide().Typography.WeightNormal)
	desc := fishboneWrap(b, it.act.Description, it.width)
	for i, line := range desc {
		if w, _ := b.MeasureText(line); w > it.width {
			desc[i] = b.TruncateToWidth(line, it.width)
		}
	}
	it.descLines = desc
	b.Pop()
}

// trackDates is the span of an activity on the axis: a moment has equal
// start and end.
func trackDates(act TimelineActivity) (start, end time.Time) {
	if act.Type == TimelineActivityTypeMilestone || act.undated || act.dateOnly {
		d := act.Date
		if d.IsZero() {
			d = act.StartDate
		}
		return d, d
	}
	start, end = act.StartDate, act.EndDate
	if start.IsZero() {
		start = act.Date
	}
	if end.IsZero() || end.Before(start) {
		end = start
	}
	return start, end
}

// trackDateText is the date line of a label block. A date the author wrote
// in words ("Mar 2026", "Q3 2025") is printed as written; an ISO date is set
// as "15 Jan 2024", a year-month as "Jan 2024", a duration as its two ends.
func trackDateText(act TimelineActivity, start, end time.Time) string {
	if end.After(start) {
		wholeMonths := start.Day() == 1 && (end.Day() == 1 || end.AddDate(0, 0, 1).Day() == 1)
		layout := "2 Jan 2006"
		if wholeMonths {
			layout = "Jan 2006"
		}
		from := start.Format(layout)
		if start.Year() == end.Year() {
			from = strings.TrimSuffix(from, start.Format(" 2006"))
		}
		return from + " – " + end.Format(layout)
	}
	if text := strings.TrimSpace(act.dateText); text != "" {
		if t, err := time.Parse("2006-01-02", text); err == nil {
			return t.Format("2 Jan 2006")
		}
		if t, err := time.Parse("2006-01", text); err == nil {
			return t.Format("Jan 2006")
		}
		return text
	}
	if start.IsZero() {
		return ""
	}
	return start.Format("2 Jan 2006")
}

// trackAssignBarLanes stacks overlapping duration bars and returns the number
// of bar lanes.
func trackAssignBarLanes(items []trackItem) int {
	var ends []float64 // right edge of the last bar in each lane
	for i := range items {
		if !items[i].isBar {
			continue
		}
		lane := -1
		for l, end := range ends {
			if items[i].x0 >= end+1 {
				lane = l
				break
			}
		}
		if lane < 0 {
			lane = len(ends)
			ends = append(ends, 0)
		}
		ends[lane] = items[i].x1
		items[i].barLane = lane
	}
	return len(ends)
}

// trackPlaceBlocks gives every label block a side, a lane and a centre.
//
// Items arrive sorted by anchor and are placed in that order. A block takes
// the first lane, nearest the axis first and alternating sides, in which
//
//   - it clears the block before it in the lane,
//   - it does not cover the leader of a block further out on its side, and
//   - its own leader does not run through a block nearer the axis.
//
// A block need not be centred on its mark: it may slide sideways as long as
// its leader still lands inside it, and a block nearer the axis slides left
// to let a later leader past. That is what keeps two events a week apart
// both labelled in full. A block with no lane left is narrowed to the room
// there is, and its text cut (reported by reportTrackLabels).
func (tc *TimelineChart) trackPlaceBlocks(items []trackItem, plot Rect, tt trackType, hasBars bool) { //nolint:gocognit,gocyclo // one greedy pass; the constraints are closures over its state
	gap := tt.body * trackBlockGap
	margin := tt.body * 0.5 // a leader lands at least this far inside its block
	left, right := plot.X, plot.X+plot.W

	type slot struct {
		above bool
		lane  int
	}
	candidates := func(it trackItem) []slot {
		var out []slot
		for lane := 0; lane < trackMaxLanes; lane++ {
			switch {
			case hasBars && it.isBar:
				out = append(out, slot{false, lane})
			case hasBars:
				out = append(out, slot{true, lane})
			default:
				out = append(out, slot{true, lane}, slot{false, lane})
			}
		}
		return out
	}

	// cxMin[j] is the leftmost centre block j may slide to without breaking
	// a constraint that held when it was placed.
	cxMin := make([]float64, len(items))

	type move struct {
		j     int
		cx    float64
		width float64 // 0 keeps the block's width
	}
	// fit tries item i in slot s. It returns the centre the block takes and
	// the earlier blocks that must slide left for it, or how far the block
	// falls short of fitting.
	fit := func(i int, s slot, narrow bool) (cx, lo float64, moves []move, short float64) {
		it := items[i]
		anchor := it.anchorX()
		lo = left + it.width/2
		for j := 0; j < i; j++ {
			o := items[j]
			if o.above != s.above {
				continue
			}
			switch {
			case o.lane == s.lane:
				lo = math.Max(lo, o.blockRight()+gap+it.width/2)
			case o.lane > s.lane:
				// o's leader passes through this lane: stay right of it.
				if o.anchorX()+gap/2 > anchor-it.width {
					lo = math.Max(lo, o.anchorX()+gap/2+it.width/2)
				}
			}
		}
		hi := math.Min(right-it.width/2, anchor+it.width/2-margin)
		if anchor+margin > right {
			hi = right - it.width/2
		}
		cx = math.Min(math.Max(anchor, lo), math.Max(hi, lo))
		if lo > hi {
			short = lo - hi
		}
		// The block's own leader must clear every block nearer the axis.
		for j := 0; j < i; j++ {
			o := items[j]
			if o.above != s.above || o.lane >= s.lane || o.blockRight()+gap/2 <= anchor {
				continue
			}
			want := anchor - gap/2 - o.width/2 // o's centre once it has slid clear
			floor := math.Max(cxMin[j], o.anchorX()-o.width/2+margin)
			if want >= floor {
				moves = append(moves, move{j: j, cx: want})
				continue
			}
			// Sliding is not enough. The nearer block may also give up
			// width (its text wraps again) as long as its own leader still
			// lands inside it.
			edge := cxMin[j] - o.width/2
			width := anchor - gap/2 - edge
			if narrow && width >= tt.body*trackMinBlockWidth && o.anchorX() <= edge+width-margin {
				moves = append(moves, move{j: j, cx: edge + width/2, width: width})
				continue
			}
			short = math.Max(short, floor-want)
		}
		return cx, lo, moves, short
	}

	for i := range items {
		cands := candidates(items[i])
		chosen := -1
		best, bestShort := 0, math.Inf(1)
		narrow := false
		for pass := 0; pass < 2 && chosen < 0; pass++ {
			narrow = pass == 1
			for c, s := range cands {
				if _, _, _, short := fit(i, s, narrow); short == 0 {
					chosen = c
					break
				} else if short < bestShort {
					best, bestShort = c, short
				}
			}
		}
		if chosen < 0 {
			// No lane has room: narrow the block by what it lacks, down to
			// the minimum, and cut its text to the new width.
			chosen = best
			it := &items[i]
			it.width = math.Max(tt.body*trackMinBlockWidth, it.width-bestShort)
			tc.trackWrap(it, tt)
		}
		s := cands[chosen]
		cx, lo, moves, _ := fit(i, s, narrow)
		for _, m := range moves {
			items[m.j].cx = m.cx
			if m.width > 0 {
				items[m.j].width = m.width
				cxMin[m.j] = m.cx
				tc.trackWrap(&items[m.j], tt)
			}
		}
		items[i].above, items[i].lane, items[i].cx = s.above, s.lane, cx
		cxMin[i] = lo
	}
}

// trackSideHeights returns the height of every lane, keyed by (side, lane)
// with side 1 above the axis, and the total height the drawing needs above
// and below the axis.
func trackSideHeights(items []trackItem, tt trackType, descLines int, barsH float64) (laneH map[[2]int]float64, above, below float64) {
	laneH = map[[2]int]float64{}
	lanes := [2]int{}
	discs := false
	for _, it := range items {
		side := 0
		if it.above {
			side = 1
		}
		key := [2]int{side, it.lane}
		laneH[key] = math.Max(laneH[key], tt.height(it, descLines))
		lanes[side] = max(lanes[side], it.lane+1)
		discs = discs || !it.isBar
	}
	sideH := func(side int) float64 {
		if lanes[side] == 0 {
			return 0
		}
		h := tt.body * trackLeader
		for l := 0; l < lanes[side]; l++ {
			h += laneH[[2]int{side, l}]
			if l > 0 {
				h += tt.body * trackLaneGap
			}
		}
		return h
	}
	above, below = sideH(1), barsH+sideH(0)
	if discs {
		// A disc reaches past the axis line on both sides.
		reach := tt.body*trackDiscRadius + trackDiscRing
		above, below = math.Max(above, reach), math.Max(below, reach)
	}
	return laneH, above, below
}

// reportTrackLabels raises chart.label_truncated for every name the track
// could not show whole.
func (tc *TimelineChart) reportTrackLabels(items []trackItem, tt trackType) {
	for _, it := range items {
		if !it.truncated {
			continue
		}
		shown := strings.Join(it.titleLines, " ")
		tc.builder.AddFinding(Finding{
			Field: it.act.field,
			Code:  FindingLabelTruncated,
			Message: fmt.Sprintf("timeline label %q does not fit beside its neighbours even staggered and wrapped at %.1fpt, and was cut to %q — shorten the label, use fewer events or spread their dates",
				it.act.Label, tt.title, shown),
			Severity: core.SeverityShrinkOrSplit,
			Fix: &FixSuggestion{
				Kind:   FixKindTruncateOrSplit,
				Params: map[string]any{"original": it.act.Label, "truncated": shown, "font_size": tt.title, "diagram_type": "timeline"},
			},
		})
	}
}

// reportTrackDescriptions says that descriptions were shortened or left out
// because the canvas is too short for them.
func (tc *TimelineChart) reportTrackDescriptions(items []trackItem, descLines int) {
	cut := 0
	for _, it := range items {
		if len(it.descLines) > descLines {
			cut++
		}
	}
	if cut == 0 {
		return
	}
	what := fmt.Sprintf("cut to %d line(s)", descLines)
	if descLines == 0 {
		what = "left out"
	}
	tc.builder.AddFinding(Finding{
		Field:    "data.items",
		Code:     FindingLabelTruncated,
		Severity: "info",
		Message: fmt.Sprintf("timeline: %d description(s) %s — the diagram's box is too short for them; shorten the descriptions, use fewer events or give the timeline a taller box",
			cut, what),
		Fix: &FixSuggestion{Kind: FixKindTruncateOrSplit, Params: map[string]any{"descriptions_cut": cut, "lines_kept": descLines, "diagram_type": "timeline"}},
	})
}
