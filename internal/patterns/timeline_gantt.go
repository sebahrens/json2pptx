package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// Gantt style of timeline-horizontal (go-slide-creator-o34er).
//
// One row per stop: its label on the left, and on the right a track that is a
// time axis. A stop with an end_date is a bar from the start of its date to
// the end of its end_date; a stop without one is a diamond at its date. The
// track's columns are the segments between every tick, bar edge and marker
// edge, so a bar spans exactly the columns its dates cover and a two-month bar
// is a third the length of a six-month one. A labelled axis row and a rule
// head the track.
//
// A stop whose dates cannot be read is not drawn as a bar at all — its date
// text is written in the track — and is reported (TIMELINE_DATE_UNPARSEABLE):
// a bar of invented length says something the data does not.

const (
	// timelineGanttGapPt separates the rows.
	timelineGanttGapPt = 8.0
	// timelineGanttColGapPt is the column gap: the track's segments abut, so a
	// bar's edges are its dates. 0 would select the grid's default gap.
	timelineGanttColGapPt = 0.01
	// timelineGanttLabelPct is the label column's default share of the grid,
	// and timelineGanttLabelMaxPct the widest it grows to hold wrapped labels.
	timelineGanttLabelPct    = 30.0
	timelineGanttLabelMaxPct = 45.0
	// timelineGanttMarkerPt is the width of a milestone diamond, and
	// timelineGanttMinBarPt the shortest bar drawn (a one-day task on a
	// two-year axis is still visible).
	timelineGanttMarkerPt = 14.0
	timelineGanttMinBarPt = 6.0
	// timelineGanttSnapPt: a bar edge this close to a tick or another edge
	// shares its column boundary instead of adding a sliver column.
	timelineGanttSnapPt = 2.0
	// timelineGanttRulePt is the weight of the axis rule, and
	// timelineGanttMinTickPt the narrowest axis division that holds its label.
	timelineGanttRulePt    = 0.75
	timelineGanttMinTickPt = 46.0
	// timelineGanttBareInsetPt is the side margin of unfilled track text (axis
	// labels, dates beside a bar): it aligns with the tick or bar edge.
	timelineGanttBareInsetPt = 3.0
)

// timelineGanttLayout is the expanded gantt at one label-column width.
type timelineGanttLayout struct {
	cols     []float64 // column percentages: the label column, then the track segments
	rows     []jsonschema.GridRowInput
	stopRow  []int   // rows index of each stop's row
	fixedPt  float64 // height of the axis rows
	problems []string
}

// timelineGanttText builds a one-paragraph text object. bare text sits in an
// unfilled track cell and takes the small track margin instead of the shape
// margin, so it starts at the tick or bar edge it belongs to.
func timelineGanttText(content string, size float64, color, align string, bold, bare bool) json.RawMessage {
	para := map[string]any{"content": content, "size": size, "align": align}
	if bold {
		para["bold"] = true
	}
	if color != "" {
		para["color"] = color
	}
	obj := map[string]any{"paragraphs": []any{para}, "align": align, "vertical_align": "ctr"}
	if bare {
		obj["inset_left"], obj["inset_right"] = timelineGanttBareInsetPt, timelineGanttBareInsetPt
		obj["inset_top"], obj["inset_bottom"] = 0, 0
	}
	data, _ := json.Marshal(obj)
	return data
}

// timelineGanttDateLabel is the date text of a stop.
func timelineGanttDateLabel(stop TimelineStop) string {
	if stop.EndDate != "" {
		return stop.Date + " → " + stop.EndDate
	}
	return stop.Date
}

// ganttLayout expands the gantt with the label column labelPct of the grid.
//
//nolint:gocognit,gocyclo // one pass over the stops building the track columns and rows
func (th *timelineHorizontal) ganttLayout(ctx ExpandContext, stops []TimelineStop, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any, labelPct float64) timelineGanttLayout {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.LabelSize, scaleDenseBodyPt)
	dateSize := ResolveSize(ovr.DateSize, sizeDenseCaptionPt)
	font := ctx.Theme.BodyFont
	n := len(stops)

	contentW, contentH := contentAreaPt(ctx)
	trackW := math.Max(contentW*(100-labelPct)/100, 1)
	// Bars are capped (go-slide-creator-7km8) so 3 stops do not become three
	// slide-height slabs; the grid centres the stack vertically.
	barMaxPt := math.Round(math.Max(contentH*0.12, labelSize*contentLineHeight*2+2*defaultShapeInsetTBPt))

	dates := resolveTimelineGanttDates(stops)
	lay := timelineGanttLayout{problems: dates.problems, stopRow: make([]int, n)}

	// Track segments: the boundaries are 0, every tick, every bar and marker
	// edge, and 1, as fractions of the axis.
	breaks := []float64{0, 1}
	var axis timelineGanttAxis
	type extent struct{ a, b float64 }
	extents := make([]extent, n)
	if dates.placed() {
		axis = newTimelineGanttAxis(dates, int(trackW/timelineGanttMinTickPt))
		for _, tick := range axis.ticks[1:] {
			breaks = append(breaks, axis.frac(tick.at))
		}
		snap := func(f float64) float64 {
			for _, b := range breaks {
				if math.Abs(b-f)*trackW < timelineGanttSnapPt {
					return b
				}
			}
			breaks = append(breaks, f)
			return f
		}
		for i, span := range dates.spans {
			if !span.ok {
				continue
			}
			a, b := axis.frac(span.start), axis.frac(span.end)
			minW := timelineGanttMinBarPt / trackW
			if span.point {
				minW = timelineGanttMarkerPt / trackW
				a -= minW / 2
			}
			a = min(max(a, 0), math.Max(1-minW, 0))
			if b-a < minW {
				b = math.Min(a+minW, 1)
			}
			if span.point {
				// A marker keeps its width: its edges are not snapped.
				for _, f := range []float64{a, b} {
					if !slices.Contains(breaks, f) {
						breaks = append(breaks, f)
					}
				}
			} else {
				a = snap(a)
				if b = snap(b); b <= a {
					// Both edges met one boundary: keep the shortest bar.
					if b = math.Min(a+minW, 1); !slices.Contains(breaks, b) {
						breaks = append(breaks, b)
					}
				}
			}
			extents[i] = extent{a, b}
		}
	}
	slices.Sort(breaks)
	breaks = slices.Compact(breaks)
	segs := len(breaks) - 1
	lay.cols = make([]float64, 0, segs+1)
	lay.cols = append(lay.cols, labelPct)
	for s := 0; s < segs; s++ {
		lay.cols = append(lay.cols, (breaks[s+1]-breaks[s])*(100-labelPct))
	}
	at := func(f float64) int {
		i, _ := slices.BinarySearch(breaks, f)
		return min(i, segs)
	}
	spacer := func(span int) *jsonschema.GridCellInput { return &jsonschema.GridCellInput{ColSpan: span} }
	bareCell := func(span int, text json.RawMessage) *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{ColSpan: span, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: text}}
	}
	// The date text is measured at the size it is written at: the writer
	// lifts a caption below the readable floor.
	dateWrittenPt := dateSize
	if tb, err := shapegrid.ResolveTextInput(timelineGanttText("0", dateSize, "", "left", false, true)); err == nil && tb != nil {
		dateWrittenPt = math.Max(largestRunPt(tb), dateSize)
	}
	// fitsOneLine reports whether text fits one line in a cell widthPt wide.
	fitsOneLine := func(text string, widthPt, insetPt float64) bool {
		w := textfit.AtomicTokenWidthPt(font, widthPt-2*insetPt)
		return w > 0 && measuredLines(text, font, false, dateWrittenPt, w) <= 1
	}

	// Axis: a row of unit labels, each starting at its tick, over a rule.
	if dates.placed() {
		axisRowPt := math.Ceil(dateWrittenPt * contentLineHeight)
		cells := []*jsonschema.GridCellInput{spacer(1)}
		for i, tick := range axis.ticks {
			from, to := at(axis.frac(tick.at)), segs
			if i+1 < len(axis.ticks) {
				to = at(axis.frac(axis.ticks[i+1].at))
			}
			if to > from {
				text := timelineGanttText(tick.label, dateSize, "", "left", false, true)
				cells = append(cells, bareCell(to-from, text))
				// The row is as tall as its tallest label is written.
				width := textfit.AtomicTokenWidthPt(font, (breaks[to]-breaks[from])*trackW)
				axisRowPt = math.Max(axisRowPt, writtenFitHeightPt(ctx.themeFonts(), text, width, 0))
			}
		}
		lay.rows = append(lay.rows,
			jsonschema.GridRowInput{Cells: cells, MinHeight: axisRowPt, MaxHeight: axisRowPt},
			jsonschema.GridRowInput{
				MinHeight: timelineGanttRulePt, MaxHeight: timelineGanttRulePt,
				Cells: []*jsonschema.GridCellInput{spacer(1), {
					ColSpan: segs,
					Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fillTone{Color: "dk1", Alpha: 30}.fillJSON()},
				}},
			})
		lay.fixedPt = axisRowPt + timelineGanttRulePt
	}

	for i, stop := range stops {
		labelCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     timelineGanttText(stop.Label, labelSize, "", "right", true, false),
		}}
		// Tint gradient per row, and the text colour that reads on this row's
		// own tint rather than a hardcoded lt1.
		tone := chevronGradientTone(accent, i, n)
		dateLabel := timelineGanttDateLabel(stop)
		cells := []*jsonschema.GridCellInput{labelCell}
		var barCell *jsonschema.GridCellInput

		switch span := dates.spans[i]; {
		case !span.ok:
			// Not placed: the date text alone, no bar to imply a duration.
			barCell = bareCell(segs, timelineGanttText(dateLabel, dateSize, "", "left", false, true))
			cells = append(cells, barCell)
		default:
			ext := extents[i]
			from, to := at(ext.a), at(ext.b)
			beforeW, barW, afterW := ext.a*trackW, (ext.b-ext.a)*trackW, (1-ext.b)*trackW
			if span.point {
				barCell = &jsonschema.GridCellInput{
					ColSpan: to - from, MaxHeight: timelineGanttMarkerPt, Fit: "contain",
					// The plain accent: the palest tint of the row gradient
					// leaves a 14pt diamond barely visible.
					Shape: &jsonschema.ShapeSpecInput{Geometry: "diamond", Fill: fillTone{Color: accent}.fillJSON(), Line: noLine},
				}
			} else {
				barCell = &jsonschema.GridCellInput{ColSpan: to - from, Shape: &jsonschema.ShapeSpecInput{Geometry: "roundRect", Fill: tone.fillJSON()}}
			}
			// The date text goes inside a bar that holds it on one line,
			// else beside the bar: after it, or before it when there is no
			// room after. With no zone that holds it, the widest takes it.
			place := "after"
			switch {
			case dateLabel == "":
				place = ""
			case !span.point && fitsOneLine(dateLabel, barW, defaultShapeInsetLRPt):
				place = "inside"
			case fitsOneLine(dateLabel, afterW, timelineGanttBareInsetPt):
			case fitsOneLine(dateLabel, beforeW, timelineGanttBareInsetPt):
				place = "before"
			case !span.point && barW >= afterW && barW >= beforeW:
				place = "inside"
			case beforeW > afterW:
				place = "before"
			}
			if place == "inside" {
				barCell.Shape.Text = timelineGanttText(dateLabel, dateSize, timelineGradientTextColor(ctx, tone), "left", false, false)
			}
			if from > 0 {
				if place == "before" {
					cells = append(cells, bareCell(from, timelineGanttText(dateLabel, dateSize, "", "right", false, true)))
				} else {
					cells = append(cells, spacer(from))
				}
			}
			cells = append(cells, barCell)
			if to < segs {
				if place == "after" {
					cells = append(cells, bareCell(segs-to, timelineGanttText(dateLabel, dateSize, "", "left", false, true)))
				} else {
					cells = append(cells, spacer(segs-to))
				}
			}
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*TimelineHorizontalCellOverride)
			if coOk {
				applyCellTextOverride(barCell, cellOvr)
			}
			if coOk && cellOvr.AccentBar {
				barCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
			}
		}

		lay.stopRow[i] = len(lay.rows)
		lay.rows = append(lay.rows, jsonschema.GridRowInput{Cells: cells, MaxHeight: barMaxPt})
	}
	return lay
}

// timelineGanttFit is the written-fit measurement of a gantt stack.
type timelineGanttFit struct {
	labelPct float64
	needs    []float64 // by row: written fit of a row whose text wraps; 0 for one-line rows
	totalPt  float64   // the stack's minimum height: wrapped rows at their fit, one-line rows at one line
	availPt  float64
	fits     bool
}

// ganttFit expands the gantt at the default label column and, while its
// wrapped rows do not fit, at wider ones in 5-point steps. With no width that
// fits it returns the default column.
func (th *timelineHorizontal) ganttFit(ctx ExpandContext, stops []TimelineStop, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (timelineGanttLayout, timelineGanttFit) {
	first := th.ganttLayout(ctx, stops, ovr, cellOverrides, timelineGanttLabelPct)
	firstFit := timelineGanttRowFit(ctx, first, timelineGanttLabelPct)
	for pct := timelineGanttLabelPct + 5; !firstFit.fits && pct <= timelineGanttLabelMaxPct; pct += 5 {
		lay := th.ganttLayout(ctx, stops, ovr, cellOverrides, pct)
		if fit := timelineGanttRowFit(ctx, lay, pct); fit.fits {
			return lay, fit
		}
	}
	return first, firstFit
}

// timelineGanttRowFit measures each stop row's text at its real cell widths.
func timelineGanttRowFit(ctx ExpandContext, lay timelineGanttLayout, labelPct float64) timelineGanttFit {
	contentW, contentH := contentAreaPt(ctx)
	// A substituted template face draws wider than its stand-in, and a label
	// that only just fits one line wraps in the renderer — where the clamped
	// margin of a one-line row leaves no room for the second line, so
	// LibreOffice shrinks it. Rows are measured at the atomic-token width.
	font := ctx.Theme.BodyFont
	fit := timelineGanttFit{
		labelPct: labelPct,
		needs:    make([]float64, len(lay.rows)),
		availPt:  contentH - float64(max(len(lay.rows)-1, 0))*ctx.Gap(timelineGanttGapPt),
		totalPt:  lay.fixedPt,
	}
	for _, r := range lay.stopRow {
		oneLine, col := 0.0, 0
		for _, c := range lay.rows[r].Cells {
			span := 1
			if c != nil && c.ColSpan > 1 {
				span = c.ColSpan
			}
			widthPct := 0.0
			for k := col; k < col+span && k < len(lay.cols); k++ {
				widthPct += lay.cols[k]
			}
			col += span
			if c == nil || c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			tb, err := shapegrid.ResolveTextInput(c.Shape.Text)
			if err != nil || tb == nil {
				continue
			}
			line := largestRunPt(tb) * contentLineHeight
			oneLine = math.Max(oneLine, line)
			width := textfit.AtomicTokenWidthPt(font, contentW*widthPct/100)
			if need := writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, width, 0); need > math.Ceil(2*defaultShapeInsetTBPt+line) {
				fit.needs[r] = math.Max(fit.needs[r], need)
			}
		}
		fit.totalPt += math.Max(fit.needs[r], oneLine)
	}
	fit.fits = fit.totalPt <= fit.availPt+1
	return fit
}

// expandGantt renders the stops as bars and markers on a shared time axis.
func (th *timelineHorizontal) expandGantt(ctx ExpandContext, stops *TimelineHorizontalValues, ovr *TimelineHorizontalOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	lay, fit := th.ganttFit(ctx, *stops, ovr, cellOverrides)

	// A row whose label wraps is held at its written fit instead of being
	// squeezed to the cap; one-line rows keep giving way, since the writer
	// clamps a short shape's margin so one line always fits. When the wrapped
	// rows do not fit, the label column widens before they are left to shrink
	// (go-slide-creator-n1muf).
	if fit.fits {
		for _, r := range lay.stopRow {
			if fit.needs[r] > 0 {
				lay.rows[r].MinHeight = fit.needs[r]
				lay.rows[r].MaxHeight = math.Max(lay.rows[r].MaxHeight, fit.needs[r])
			}
		}
	}
	cols, err := json.Marshal(lay.cols)
	if err != nil {
		return nil, fmt.Errorf("timeline-horizontal: marshal gantt columns: %w", err)
	}
	return &jsonschema.ShapeGridInput{
		Columns:       cols,
		ColGap:        timelineGanttColGapPt,
		RowGap:        ctx.Gap(timelineGanttGapPt),
		Rows:          lay.rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// timelineGanttDateWarnings reports the dates a gantt-style timeline could
// not place, as at most one TIMELINE_DATE_UNPARSEABLE line. Other styles print
// their dates as text and report nothing.
func timelineGanttDateWarnings(style string, stops []TimelineStop) []string {
	if style != "gantt" {
		return nil
	}
	problems := resolveTimelineGanttDates(stops).problems
	if len(problems) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%s: timeline-horizontal gantt draws no bar for a stop whose dates it cannot place on the time axis (%s) — write dates as 2026-03-15, 2026-03, Mar 2026, Q1 2026, H1 2026 or 2026 (every stop with a year, or every stop without), or use dots style for stops that are not dated",
		ErrCodeTimelineDateUnparseable, strings.Join(problems, "; "))}
}
