package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Bars layout of roadmap-phased (go-slide-creator-4a0sm) — the default.
//
// The legacy roadmap was a workstream × period table with one tile per cell,
// so every activity was exactly one period long. Here the period headers are
// a time axis and a workstream carries bars on it:
//
//	            Q1      Q2      Q3      Q4
//	            ──────  ██████  ──────  ──────      Q2 is the current period
//	Platform    [ Auth rewrite ]
//	                    [ API v2                ]
//	                                    ◆ GA
//	Frontend    [ Design system         ]
//	                            [ Mobile app    ]
//
// A bar runs from its start period to its end period (one period when it has
// no end or span). Bars of one workstream that share a period are stacked in
// lanes, first fit in the order written, so they never collide; the workstream
// label spans its lanes. A milestone is a marker with its label in one period.
// Lanes are as tall as the tallest bar's text needs and the block is not
// stretched over the slide.
//
// One-item-per-period input (workstreams[].items) is drawn the same way: each
// non-empty item is a one-period bar. overrides.layout "grid" keeps the legacy
// table of equal tiles for that input.

const (
	// roadmapMaxBars is the most bars one workstream holds.
	roadmapMaxBars = 12
	// roadmapLaneGapPt separates two lanes, and roadmapStreamGapPt is the
	// blank row between two workstreams (on top of the lane gaps round it).
	roadmapLaneGapPt   = 4.0
	roadmapStreamGapPt = 4.0
	// roadmapAxisRulePt is the weight of the axis segment under each period.
	roadmapAxisRulePt = 2.0
	// roadmapMilestoneGlyph marks a milestone.
	roadmapMilestoneGlyph = "◆"
)

// roadmapPhasedLayouts are the accepted overrides.layout values.
var roadmapPhasedLayouts = []string{"bars", "grid"}

// roadmapPhaseIndex returns the index of the phase a bar names, or -1. The
// match ignores case and surrounding space; the first of two equal labels wins.
func roadmapPhaseIndex(phases []string, name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1
	}
	for i, p := range phases {
		if strings.EqualFold(strings.TrimSpace(p), name) {
			return i
		}
	}
	return -1
}

// roadmapHasBars reports whether any workstream declares bars.
func roadmapHasBars(v *RoadmapPhasedValues) bool {
	for _, ws := range v.Workstreams {
		if len(ws.Bars) > 0 {
			return true
		}
	}
	return false
}

// roadmapPlacedBar is one bar or milestone placed on the axis.
type roadmapPlacedBar struct {
	label     string
	from, to  int // first and last phase index, inclusive
	milestone bool
	lane      int
	// index is the bar's position in workstreams[].bars, or in
	// workstreams[].items for one-item-per-period input; cell is its
	// cell_overrides index.
	index int
	cell  int
	// path names the field in a warning.
	path string
}

// roadmapStream is one workstream's placed bars.
type roadmapStream struct {
	bars  []roadmapPlacedBar
	lanes int
	cell  int // cell_overrides index of the workstream label
}

// roadmapExtent resolves a bar's first and last phase. Validate has already
// refused what does not resolve; an unresolved bar here is clamped onto the
// axis rather than dropped.
func roadmapExtent(phases []string, bar RoadmapBar) (from, to int) {
	last := len(phases) - 1
	from = max(roadmapPhaseIndex(phases, bar.Start), 0)
	to = from
	switch {
	case bar.Milestone:
	case bar.End != "":
		to = roadmapPhaseIndex(phases, bar.End)
	case bar.Span > 1:
		to = from + bar.Span - 1
	}
	return from, min(max(to, from), max(last, from))
}

// placeRoadmapStreams turns every workstream into bars with lanes. Bars take
// the first lane free over their whole extent, in the order written.
func placeRoadmapStreams(vals *RoadmapPhasedValues) []roadmapStream {
	streams := make([]roadmapStream, len(vals.Workstreams))
	cell := len(vals.Phases)
	for i, ws := range vals.Workstreams {
		st := roadmapStream{cell: cell}
		cell++
		if len(ws.Bars) > 0 {
			for j, bar := range ws.Bars {
				from, to := roadmapExtent(vals.Phases, bar)
				st.bars = append(st.bars, roadmapPlacedBar{
					label: bar.Label, from: from, to: to, milestone: bar.Milestone,
					index: j, cell: cell, path: fmt.Sprintf("workstreams[%d].bars[%d].label", i, j),
				})
				cell++
			}
		} else {
			for j, item := range ws.Items {
				if item != "" && j < len(vals.Phases) {
					st.bars = append(st.bars, roadmapPlacedBar{
						label: item, from: j, to: j,
						index: j, cell: cell, path: fmt.Sprintf("workstreams[%d].items[%d]", i, j),
					})
				}
				cell++
			}
		}
		var laneEnd [][]bool // by lane, by phase: occupied
		for k := range st.bars {
			b := &st.bars[k]
			lane := 0
			for ; lane < len(laneEnd); lane++ {
				free := true
				for p := b.from; p <= b.to; p++ {
					free = free && !laneEnd[lane][p]
				}
				if free {
					break
				}
			}
			if lane == len(laneEnd) {
				laneEnd = append(laneEnd, make([]bool, len(vals.Phases)))
			}
			for p := b.from; p <= b.to; p++ {
				laneEnd[lane][p] = true
			}
			b.lane = lane
		}
		st.lanes = max(len(laneEnd), 1)
		streams[i] = st
	}
	return streams
}

// roadmapBarsTotalCells is the cell_overrides index ceiling: the phase
// headers, then per workstream its label and one slot per bar (or per item).
func roadmapBarsTotalCells(vals *RoadmapPhasedValues) int {
	total := len(vals.Phases)
	for _, ws := range vals.Workstreams {
		if len(ws.Bars) > 0 {
			total += 1 + len(ws.Bars)
		} else {
			total += 1 + len(ws.Items)
		}
	}
	return total
}

// roadmapBarsLayout is the measured bars layout.
type roadmapBarsLayout struct {
	streams []roadmapStream
	text    [][]json.RawMessage // by workstream, by placed bar
	widthPt [][]float64         // by workstream, by placed bar
	laneH   float64             // lane height as laid out
	lineH   float64             // one line of bar text
	lanes   int                 // lanes over all workstreams
	holds   int                 // lanes the content area holds at one line each
	needH   float64             // lane height the tallest bar needs
	headerH float64
	fits    bool
	accent  string
	barTone fillTone
}

// layoutRoadmapBars measures the bars at the context's content area.
func layoutRoadmapBars(ctx ExpandContext, vals *RoadmapPhasedValues, ovr *RoadmapPhasedOverrides) roadmapBarsLayout {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, scaleDenseBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleCaptionPt)
	fonts := ctx.themeFonts()
	contentW, contentH := contentAreaPt(ctx)

	lay := roadmapBarsLayout{streams: placeRoadmapStreams(vals), accent: accent}
	// Bars are a light tint of the accent with the ink that reads on it; style
	// "solid" fills them with the accent.
	lay.barTone = inactiveTintTone(accent)
	if ovr.Style == "solid" {
		lay.barTone = fillTone{Color: accent}
	}
	barInk := readableTextOn(ctx, lay.barTone, "dk1")
	if ovr.Style == "solid" {
		barInk = readableTextOn(ctx, lay.barTone, "lt1")
	}
	markerInk := accentInkOnLight(ctx, accent, 4.5)

	phases := max(len(vals.Phases), 1)
	gap := ctx.Gap(6)
	usable := contentW - float64(phases)*gap
	phaseW := usable * roadmapPhasesPct / 100 / float64(phases)

	lanes := 0
	lay.text = make([][]json.RawMessage, len(lay.streams))
	lay.widthPt = make([][]float64, len(lay.streams))
	for i, st := range lay.streams {
		lanes += st.lanes
		for _, b := range st.bars {
			span := float64(b.to - b.from + 1)
			w := span*phaseW + (span-1)*gap
			text := buildRoadmapTextContent(pptx.ConvertMarkdownEmphasis(b.label), bodySize, false, barInk, "ctr")
			if b.milestone {
				text = buildRoadmapTextContent(roadmapMilestoneGlyph+" "+pptx.ConvertMarkdownEmphasis(b.label), bodySize, true, markerInk, "l")
			}
			lay.text[i] = append(lay.text[i], text)
			lay.widthPt[i] = append(lay.widthPt[i], w)
			lay.needH = math.Max(lay.needH, driverTreeNeedPt(fonts, text, w, bodySize))
		}
	}
	if lay.needH == 0 {
		lay.needH = math.Ceil(bodySize*contentLineHeight + 2*defaultShapeInsetTBPt)
	}
	for _, phase := range vals.Phases {
		text := buildRoadmapTextContent(phase, headerSize, true, "dk1", "ctr")
		lay.headerH = math.Max(lay.headerH, driverTreeNeedPt(fonts, text, phaseW, headerSize))
	}

	laneGap := ctx.Gap(roadmapLaneGapPt)
	nStreams := len(lay.streams)
	// Rows: header, then per workstream its lanes, with a blank row between
	// two workstreams.
	rowCount := 1 + lanes + max(nStreams-1, 0)
	fixed := lay.headerH + float64(max(nStreams-1, 0))*roadmapStreamGapPt + float64(rowCount-1)*laneGap
	lay.laneH = lay.needH
	lay.lanes = lanes
	lay.fits = contentH <= 0 || fixed+float64(lanes)*lay.needH <= contentH+1
	if !lay.fits && lanes > 0 {
		lay.laneH = math.Max((contentH-fixed)/float64(lanes), 1)
	}
	// The writer clamps a squeezed bar's margin until one line fits, so a
	// lane is readable down to one line of text.
	writtenPt := bodySize // the writer lifts a caption below the readable floor
	if tb, err := shapegrid.ResolveTextInput(buildRoadmapTextContent("0", bodySize, false, "dk1", "ctr")); err == nil && tb != nil {
		writtenPt = math.Max(largestRunPt(tb), bodySize)
	}
	lay.lineH = math.Ceil(writtenPt * contentLineHeight)
	lay.holds = lanes
	if contentH > 0 {
		lay.holds = int((contentH - lay.headerH - float64(max(nStreams-1, 0))*(roadmapStreamGapPt+laneGap)) / (lay.lineH + laneGap))
	}
	return lay
}

// roadmapBarWarnings names the bars whose label is written shrunk.
func roadmapBarWarnings(ctx ExpandContext, vals *RoadmapPhasedValues, ovr *RoadmapPhasedOverrides) []string {
	lay := layoutRoadmapBars(ctx, vals, ovr)
	fonts := ctx.themeFonts()
	lanes := lay.lanes
	if lay.laneH < lay.lineH {
		// No bar is readable: say so once instead of once per bar.
		return []string{fmt.Sprintf("%s: roadmap-phased stacks %d lanes of bars over %d workstreams; this slide holds about %d lanes at a readable size — let fewer bars of a workstream share a period, merge activities, or split the roadmap", ErrCodeBodyTooLong, lanes, len(vals.Workstreams), max(lay.holds, 1))}
	}
	var warnings []string
	for i, st := range lay.streams {
		if len(vals.Workstreams[i].Bars) == 0 {
			continue // one-item-per-period input is reported against its measured budgets
		}
		for k, b := range st.bars {
			tb, err := shapegrid.ResolveTextInput(lay.text[i][k])
			if err != nil || tb == nil {
				continue
			}
			tb.ThemeFonts = fonts
			box := pptx.RectEmu{CX: int64(lay.widthPt[i][k] * sizingEMUPerPt), CY: int64(lay.laneH * sizingEMUPerPt)}
			if !pptx.AutofitFitsFor(tb, box) {
				warnings = append(warnings, fmt.Sprintf("%s: roadmap-phased %s is %d characters; a bar over %d of %d periods in a roadmap of %d lanes does not hold it at a readable size — shorten the label, extend the bar, or use fewer overlapping bars or workstreams", ErrCodeBodyTooLong, b.path, runeLen(b.label), b.to-b.from+1, len(vals.Phases), lanes))
			}
		}
	}
	return warnings
}

// roadmapPhasesPct is the share of the grid the period columns take; the
// workstream label column takes the rest.
const roadmapPhasesPct = 82.0

// expandBars renders the roadmap as bars on a shared time axis.
//
//nolint:gocognit // header, then one pass over the workstreams' lanes
func (r *roadmapPhased) expandBars(ctx ExpandContext, vals *RoadmapPhasedValues, ovr *RoadmapPhasedOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	lay := layoutRoadmapBars(ctx, vals, ovr)
	accent := lay.accent
	headerSize := ResolveSize(ovr.HeaderSize, scaleDenseBodyPt)
	solid := ovr.Style == "solid"
	phaseCount := len(vals.Phases)
	current := roadmapPhaseIndex(vals.Phases, vals.CurrentPhase)

	cols := make([]float64, 1+phaseCount)
	cols[0] = 100 - roadmapPhasesPct
	for i := 1; i <= phaseCount; i++ {
		cols[i] = roadmapPhasesPct / float64(phaseCount)
	}
	colsJSON, _ := json.Marshal(cols)

	// Time axis: a bold label over an accent segment per period. The current
	// period is the one filled header.
	accentFill := json.RawMessage(`"` + accent + `"`)
	header := []*jsonschema.GridCellInput{{}}
	for i, phase := range vals.Phases {
		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildRoadmapTextContent(phase, headerSize, true, "dk1", "ctr"),
			},
			AccentBar: &jsonschema.AccentBarInput{Position: "bottom", Color: accent, Width: roadmapAxisRulePt},
		}
		switch {
		case i == current && solid:
			dark := structuralDarkTone(ctx)
			cell.Shape.Fill, cell.Shape.Line, cell.AccentBar = dark.fillJSON(), noLine, nil
			cell.Shape.Text = buildRoadmapTextContent(phase, headerSize, true, readableTextOn(ctx, dark, "lt1"), "ctr")
		case i == current, solid:
			cell.Shape.Fill, cell.Shape.Line, cell.AccentBar = accentFill, noLine, nil
			cell.Shape.Text = buildRoadmapTextContent(phase, headerSize, true, readableTextOn(ctx, fillTone{Color: accent}, "lt1"), "ctr")
		}
		applyRoadmapOverride(cell, cellOverrides, i, accent)
		header = append(header, cell)
	}
	rows := []jsonschema.GridRowInput{{Cells: header, MinHeight: lay.headerH, MaxHeight: lay.headerH}}

	labelFill := tonalPanel(ctx, accent).fillJSON()
	if solid {
		labelFill = json.RawMessage(`"lt2"`)
	}
	laneRow := func(cells []*jsonschema.GridCellInput) jsonschema.GridRowInput {
		row := jsonschema.GridRowInput{Cells: cells, MaxHeight: lay.needH}
		if lay.fits {
			row.MinHeight = lay.needH
		}
		return row
	}
	for i, st := range lay.streams {
		if i > 0 {
			rows = append(rows, jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{{}}, MinHeight: roadmapStreamGapPt, MaxHeight: roadmapStreamGapPt})
		}
		label := &jsonschema.GridCellInput{
			RowSpan: st.lanes,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     labelFill,
				Line:     noLine,
				Text:     buildRoadmapTextContent(vals.Workstreams[i].Name, headerSize, true, "dk1", "l"),
			},
		}
		applyRoadmapOverride(label, cellOverrides, st.cell, accent)
		for lane := 0; lane < st.lanes; lane++ {
			var cells []*jsonschema.GridCellInput
			if lane == 0 {
				cells = append(cells, label)
			}
			next := 0 // first phase column not yet filled in this lane
			for _, k := range roadmapLaneOrder(st.bars, lane) {
				b := st.bars[k]
				for ; next < b.from; next++ {
					cells = append(cells, &jsonschema.GridCellInput{})
				}
				cell := &jsonschema.GridCellInput{
					ColSpan: b.to - b.from + 1,
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     lay.barTone.fillJSON(),
						Line:     noLine,
						Text:     lay.text[i][k],
					},
				}
				if b.milestone {
					cell.Shape.Geometry, cell.Shape.Fill, cell.Shape.Line = "rect", json.RawMessage(`"none"`), nil
				}
				applyRoadmapOverride(cell, cellOverrides, b.cell, accent)
				cells = append(cells, cell)
				next = b.to + 1
			}
			if len(cells) == 0 {
				cells = append(cells, &jsonschema.GridCellInput{})
			}
			rows = append(rows, laneRow(cells))
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		Gap:           ctx.Gap(6),
		RowGap:        ctx.Gap(roadmapLaneGapPt),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// roadmapLaneOrder returns the indices of the bars in one lane, left to right.
func roadmapLaneOrder(bars []roadmapPlacedBar, lane int) []int {
	var out []int
	for k, b := range bars {
		if b.lane == lane {
			out = append(out, k)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && bars[out[j]].from < bars[out[j-1]].from; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
