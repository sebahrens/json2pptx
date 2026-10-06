package patterns

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// swimlane, default "bands" style (go-slide-creator-vx7wk)
//
// A lane is one pale band (a row band: one rectangle behind the row) headed
// by a pentagon tab that carries the actor. Steps are pentagons in a tint of
// the accent, placed in their time column, so a step points at the next one
// in its lane and no arrow is needed there. An arrow is drawn only where the
// flow leaves its lane, skips a column or runs back, in one neutral weight.
// The one highlighted step takes the solid accent.
// ---------------------------------------------------------------------------

const (
	// swimlaneActorPct is the actor tab's share of the lane width when its
	// labels fill it. Labels that need less on one line give the rest to the
	// steps, down to swimlaneActorMinPct; a tab whose longest word needs more
	// takes it, up to swimlaneActorMaxPct (a swimlane in half a slide:
	// "Finance" is wider than 15% of it).
	swimlaneActorPct    = 15.0
	swimlaneActorMinPct = 11.0
	swimlaneActorMaxPct = 26.0
	// swimlaneBandTileShare is the share of its lane a step takes: the band
	// already says which lane a step is in, so a step needs less air around
	// it than a tile between two rules did.
	swimlaneBandTileShare = 0.8
	// swimlaneTabPointPt is the depth of the actor tab's point.
	swimlaneTabPointPt = 10.0
	// A step's point is swimlaneStepPointFrac of the tile height, between
	// swimlaneStepPointMinPt (below it the shape reads as a box) and
	// swimlaneStepPointMaxPt, and never more than swimlaneStepPointMaxColFrac
	// of the step column: a dense grid gives the point up before the text.
	swimlaneStepPointFrac       = 0.2
	swimlaneStepPointMinPt      = 5.0
	swimlaneStepPointMaxPt      = 14.0
	swimlaneStepPointMaxColFrac = 0.1
	// swimlanePointInsetMinPt is the least right margin of a pentagon's label
	// inside its text rectangle.
	swimlanePointInsetMinPt = 4.0
	// swimlaneBandPadPt is the least air between a step and its band's edge.
	swimlaneBandPadPt = 4.0
	// swimlaneTabTint is the actor tab's neutral step: deeper than the 4%
	// band it heads.
	swimlaneTabTint = 12
	// swimlaneConnectorPt is the one weight of every hand-off arrow.
	swimlaneConnectorPt = 1.5
)

// swimlaneBandRowGap is the white gutter between two lane bands.
func swimlaneBandRowGap(lanes int) float64 {
	switch {
	case lanes >= 6:
		return 2
	case lanes == 5:
		return 3
	}
	return 4
}

// swimlaneBandLayout is the geometry of the band style.
type swimlaneBandLayout struct {
	swimlaneSizing
	colGap, rowGap float64
	// pointPt is the depth of every step's point.
	pointPt float64
	// actorPct is the actor tab's share of the lane width.
	actorPct float64
}

// stepTextW and actorTextW are the widths of a step's and the tab's own text
// rectangle: a pentagon gives up half its point (pptx.PresetTextRectSize).
func (l swimlaneBandLayout) stepTextW() float64  { return l.colW - l.pointPt/2 }
func (l swimlaneBandLayout) actorTextW() float64 { return l.actorW - swimlaneTabPointPt/2 }

// swimlanePointedText builds the text of a pentagon whose point is pointPt
// deep. The preset's own text rectangle already stops half the point short of
// the tip, so the right margin gives that half back (never below
// swimlanePointInsetMinPt): the label keeps the width it has in a rectangle
// of the same size and sits centred between the flat edge and the shoulder of
// the point.
func swimlanePointedText(content string, size float64, bold bool, color, align string, pointPt float64) json.RawMessage {
	text := buildSwimlaneTextContent(content, size, bold, color, align)
	return withTextInsetSides(text, math.Max(swimlanePointInsetMinPt, defaultShapeInsetLRPt-pointPt/2), "inset_right")
}

// swimlaneTabText builds the actor label of a lane tab. It is left-aligned at
// the uniform margin and may run up to swimlanePointInsetMinPt off the tab's
// text rectangle on the right: the tab then holds the label the unfilled
// actor column of the tiles style held.
func swimlaneTabText(actor string, size float64, color string) json.RawMessage {
	return withTextInsetSides(buildSwimlaneTextContent(actor, size, true, color, "l"), swimlanePointInsetMinPt, "inset_right")
}

// swimlaneBandSizing measures the lanes of a steps x lanes band grid in the
// content area. Columns are [actor tab, steps..., edge]: the zero-width edge
// column keeps the last step one column gap off the band's right end.
//
// actorSizePt is the size the actor labels are set in: the tab is as wide as
// its longest word needs at that size, within [swimlaneActorPct,
// swimlaneActorMaxPct].
func swimlaneBandSizing(ctx ExpandContext, vals *SwimlaneValues, actorSizePt float64) swimlaneBandLayout {
	lanes, steps := len(vals.Lanes), 0
	if lanes > 0 {
		steps = len(vals.Lanes[0].Steps)
	}
	l := swimlaneBandLayout{colGap: swimlaneColGap(steps), rowGap: swimlaneBandRowGap(lanes), actorPct: swimlaneActorPct}
	l.tileH = swimlaneTileMinPt
	if lanes < 1 || steps < 1 {
		return l
	}
	contentW, contentH := contentAreaPt(ctx)
	usable := contentW - float64(steps+1)*l.colGap
	if usable > 0 {
		word, line := 0.0, 0.0
		for _, lane := range vals.Lanes {
			line = math.Max(line, rhWordWidthPt(ctx, lane.Actor, true, actorSizePt))
			for _, w := range strings.Fields(lane.Actor) {
				word = math.Max(word, rhWordWidthPt(ctx, w, true, actorSizePt))
			}
		}
		pct := func(textPt float64) float64 {
			return math.Ceil((textPt + 2*defaultShapeInsetLRPt) / usable * 100)
		}
		l.actorPct = math.Max(swimlaneActorMinPct, math.Min(pct(line), swimlaneActorPct))
		l.actorPct = math.Min(math.Max(l.actorPct, pct(word)), swimlaneActorMaxPct)
	}
	l.colW = usable * (100 - l.actorPct) / 100 / float64(steps)
	l.actorW = usable * l.actorPct / 100
	l.laneH = (contentH - float64(lanes-1)*l.rowGap) / float64(lanes)
	canvas := shapegrid.CanvasScaleFor(ctx.SlideWidth, ctx.SlideHeight)
	l.tileH = math.Max(swimlaneTileMinPt, math.Min(math.Min(l.laneH*swimlaneBandTileShare, l.laneH-2*swimlaneBandPadPt)/canvas, l.colW*swimlaneTileMaxAspect))
	l.pointPt = math.Min(math.Max(swimlaneStepPointMinPt, math.Min(l.tileH*swimlaneStepPointFrac, swimlaneStepPointMaxPt)), math.Max(swimlaneStepPointMinPt, l.colW*swimlaneStepPointMaxColFrac))
	return l
}

// holdsAt reports whether every step fits its pentagon and every actor its
// tab at sizePt, as the writer measures them.
func (l swimlaneBandLayout) holdsAt(ctx ExpandContext, vals *SwimlaneValues, sizePt float64) bool {
	if l.colW <= 0 || l.laneH <= 0 {
		return false
	}
	fonts := ctx.themeFonts()
	canvas := shapegrid.CanvasScaleFor(ctx.SlideWidth, ctx.SlideHeight)
	for _, lane := range vals.Lanes {
		actor := swimlaneTabText(lane.Actor, sizePt, "dk1")
		if writtenFitHeightPt(fonts, actor, l.actorTextW(), 0) > l.laneH/canvas {
			return false
		}
		for _, step := range lane.Steps {
			if step == "" {
				continue
			}
			text := swimlanePointedText(pptx.ConvertMarkdownEmphasis(step), sizePt, false, "dk1", "ctr", l.pointPt)
			if writtenFitHeightPt(fonts, text, l.stepTextW(), 0) > l.tileH {
				return false
			}
		}
	}
	return true
}

// pointAdj is a pentagon's "adj" for a point pointPt deep on a shape w x h:
// the preset states it as a share (x100000) of the shorter side.
func swimlanePointAdj(pointPt, w, h float64) int64 {
	short := math.Min(w, h)
	if short <= 0 {
		return 0
	}
	return int64(math.Round(math.Min(pointPt/short, 0.5) * 100000))
}

// swimlaneConnectorColor is the neutral dark of the hand-off arrows: the
// template's dk2 where it carries the brand (navy, forest), its ink otherwise.
func swimlaneConnectorColor(ctx ExpandContext) string {
	if isNearBlack(ctx, "dk2") {
		return "dk1"
	}
	return "dk2"
}

func (s *swimlane) expandBands(ctx ExpandContext, vals *SwimlaneValues, ovr *SwimlaneOverrides, cellOverrides map[int]any, accent string) *jsonschema.ShapeGridInput {
	stepCount := 0
	if len(vals.Lanes) > 0 {
		stepCount = len(vals.Lanes[0].Steps)
	}

	// The lane is set in the subhead step where every step and actor holds
	// at it; authored sizes are kept.
	headerSize := ResolveSize(ovr.HeaderSize, scaleBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)
	lay := swimlaneBandSizing(ctx, vals, headerSize)
	if ovr.BodySize == 0 && ovr.HeaderSize == 0 {
		if big := swimlaneBandSizing(ctx, vals, scaleSubheadPt); big.holdsAt(ctx, vals, scaleSubheadPt) {
			lay, headerSize, bodySize = big, scaleSubheadPt, scaleSubheadPt
		}
	}

	// Columns: actor tab, the step columns, and a zero-width edge column.
	cols := make([]float64, 0, stepCount+2)
	cols = append(cols, lay.actorPct)
	for range stepCount {
		cols = append(cols, (100-lay.actorPct)/float64(stepCount))
	}
	cols = append(cols, 0)
	colsJSON, _ := json.Marshal(cols)

	bandFill := neutralFillJSON(NeutralTint4)
	tabTone := neutralTone(swimlaneTabTint)
	tabInk := readableTextOn(ctx, tabTone, "dk1")
	stepTone := inactiveTintTone(accent)
	stepInk := readableTextOn(ctx, stepTone, "dk1")
	solidTone := fillTone{Color: accent}
	solidInk := readableTextOn(ctx, solidTone, "lt1")

	fonts := ctx.themeFonts()
	need := swimlaneTileMinPt
	actorNeed := 0.0
	rows := make([]jsonschema.GridRowInput, 0, len(vals.Lanes))
	cellIdx := 0
	for i, lane := range vals.Lanes {
		cells := make([]*jsonschema.GridCellInput, 0, stepCount+2)

		actorText := swimlaneTabText(lane.Actor, headerSize, tabInk)
		actorNeed = math.Max(actorNeed, writtenFitHeightPt(fonts, actorText, lay.actorTextW(), 0))
		tab := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "homePlate",
				Fill:     tabTone.fillJSON(),
				Line:     noLine,
				Text:     actorText,
			},
		}
		applySwimlaneOverride(tab, cellOverrides, cellIdx, accent)
		cellIdx++
		cells = append(cells, tab)

		for j, step := range lane.Steps {
			var cell *jsonschema.GridCellInput
			if step == "" {
				// An empty position is an unpainted spacer: the band shows
				// through and arrows skip it.
				cell = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine}}
			} else {
				tone, ink, bold := stepTone, stepInk, false
				if vals.highlighted(i, j) {
					tone, ink, bold = solidTone, solidInk, true
				}
				text := swimlanePointedText(pptx.ConvertMarkdownEmphasis(step), bodySize, bold, ink, "ctr", lay.pointPt)
				need = math.Max(need, writtenFitHeightPt(fonts, text, lay.stepTextW(), 0))
				cell = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "homePlate",
						Fill:     tone.fillJSON(),
						Line:     noLine,
						Text:     text,
					},
				}
			}
			applySwimlaneOverride(cell, cellOverrides, cellIdx, accent)
			cellIdx++
			cells = append(cells, cell)
		}
		cells = append(cells, &jsonschema.GridCellInput{})
		rows = append(rows, jsonschema.GridRowInput{Cells: cells, Band: bandFill})
	}

	// One height for every step: the lane's share, bounded by the longest
	// step's written fit (swimlaneTileHeightPt's rule). The lane is no taller
	// than that step with its share of air, so a short flow is a block of
	// bands the size of its content, not half-empty stripes.
	tileH := math.Max(need, math.Min(lay.tileH, need*swimlaneTileMaxFill))
	laneMax := math.Max(math.Max(tileH/swimlaneBandTileShare, tileH+2*swimlaneBandPadPt), actorNeed)
	laneH := laneMax
	if canvas := shapegrid.CanvasScaleFor(ctx.SlideWidth, ctx.SlideHeight); lay.laneH > 0 {
		laneH = math.Min(laneMax, lay.laneH/canvas)
	}
	for r := range rows {
		rows[r].MaxHeight = laneMax
		for j, c := range rows[r].Cells {
			if c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			if j == 0 {
				c.Shape.Adjustments = map[string]int64{"adj": swimlanePointAdj(swimlaneTabPointPt, lay.actorW, laneH)}
				continue
			}
			c.MaxHeight = tileH
			c.Shape.Adjustments = map[string]int64{"adj": swimlanePointAdj(lay.pointPt, lay.colW, math.Min(tileH, laneH))}
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  lay.colGap,
		RowGap:  lay.rowGap,
		Rows:    rows,
		Links:   swimlaneBandLinks(vals, swimlaneConnectorColor(ctx)),
	}
}

// swimlaneBandLinks draws the hand-offs of the band style. A step followed by
// the next column of its own lane needs no arrow — the pentagon points at it;
// every other transition (another lane, a skipped column, a loop back) gets
// one neutral arrow.
func swimlaneBandLinks(vals *SwimlaneValues, color string) []jsonschema.GridLinkInput {
	order := swimlaneFlowOrder(vals)
	var links []jsonschema.GridLinkInput
	for k := 0; k+1 < len(order); k++ {
		from, to := order[k], order[k+1]
		if from[0] == to[0] && to[1] == from[1]+1 {
			continue
		}
		links = append(links, jsonschema.GridLinkInput{
			From:      from,
			To:        to,
			Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: color, Width: swimlaneConnectorPt, Head: "lg"},
		})
	}
	return links
}
