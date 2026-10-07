package generator

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Process flow detours — connections that are not step-to-next-step
// =============================================================================
//
// A horizontal flowchart sets its steps in one row and used to draw every
// connection as a straight line between its two boxes. That is right for a
// step and the step after it and wrong for everything else: a decision's
// "No" branch to the step after next ran through the text of the step
// between them, a loop back to the decision ran along the same line the
// other way, and the two branch labels sat on top of each other at the
// diamond (go-slide-creator-40xtt).
//
// A connection between two steps that are not neighbours in a row now takes
// a detour outside the rows: it leaves its source vertically, runs along a
// lane clear of every step and enters its target vertically. Within a row,
// skips forward run under the row and loops back over it; a connection to the
// row below (or above) runs in the gap between the two rows. Detours that
// would share a stretch of lane get lanes of their own, the shortest nearest
// the upper row. Because every lane lies between rows and every vertical
// stub lies under (or over) the step it belongs to, no connector crosses a
// step or its text. The branch label sits beside the first stub.
//
// A flow with detours sets its rows left to right, one under the other (the
// plain multi-row flow snakes; a snake leaves no free lane beside its turn).
// A flow whose detours would have to pass a whole row, or that does not fit
// the height with its lanes, is drawn as the vertical flow instead, which
// sets a decision's targets on side lanes (pfLayoutVertical).

const (
	// pfDetourLaneGap is the distance between the row and the first detour
	// lane, and between two lanes (EMU, 0.3"): a 12pt label and clearance.
	pfDetourLaneGap int64 = 274320

	// pfDetourMinLaneGap is the least a lane gap is squeezed to in a short
	// region (EMU, 10pt): room for an arrowhead.
	pfDetourMinLaneGap int64 = 10 * 12700

	// pfDetourLabelPad separates a detour's label from its stub (EMU, 3pt).
	pfDetourLabelPad int64 = 3 * 12700
)

// pfDetour is one connection's route outside the rows.
type pfDetour struct {
	// conn is the index of the connection in the flow's connection list.
	conn int
	// zone is the gap the lane runs in: zone z lies under row z, and zone -1
	// over the first row. level is the lane within it, 0 nearest the top.
	zone, level int
	// The route: from (srcX, srcY) on the source's edge vertically to laneY,
	// along the lane to tgtX, then vertically to (tgtX, tgtY) on the target's
	// edge, where the arrowhead is.
	srcX, srcY, laneY, tgtX, tgtY int64
	// srcDown / tgtDown say which edge the route meets: the bottom (true) or
	// the top of the step.
	srcDown, tgtDown bool
}

// pfNeedsDetours reports whether any connection joins two steps that are not
// a step and the step after it — the connections a row cannot draw as a
// straight line.
func pfNeedsDetours(steps []processFlowStep, connections []processFlowConnection) bool {
	index := make(map[string]int, len(steps))
	for i, s := range steps {
		index[s.id] = i
	}
	for _, c := range connections {
		from, okFrom := index[c.from]
		to, okTo := index[c.to]
		if okFrom && okTo && from != to && to != from+1 {
			return true
		}
	}
	return false
}

// pfDetourAttachX is where a detour meets step i's top or bottom edge. A
// diamond and a terminator are met at the centre, the only point of their
// edge that faces the lane squarely; a box is met a sixth of its width off
// centre, toward the other end of the detour, so a detour that leaves a box
// and one that enters it do not share a stub.
func pfDetourAttachX(step processFlowStep, sl pfStepLayout, towardRight bool) int64 {
	cx := sl.x + sl.cx/2
	switch step.stepType {
	case pfDecisionType, pfStartType, pfEndType:
		return cx
	}
	if towardRight {
		return cx + sl.cx/6
	}
	return cx - sl.cx/6
}

// pfLayoutDetouredRows lays a flow with detours out in rows of perRow steps,
// each row left to right, and routes every connection that is not a step to
// its right-hand neighbour. layouts carry the step sizes. ok is false when
// the rows are wider than bounds, a detour would have to pass a whole row, or
// the rows and their lanes do not fit the height.
func pfLayoutDetouredRows(layouts []pfStepLayout, steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox, perRow int) (pfLayoutResult, bool) { //nolint:gocognit,gocyclo // one pass per layout stage
	n := len(layouts)
	if n == 0 || perRow < 1 {
		return pfLayoutResult{}, false
	}
	rows := (n + perRow - 1) / perRow
	rowOf := func(i int) int { return i / perRow }

	// Columns: every row starts on the same left edge.
	rowH := make([]int64, rows)
	var widest int64
	for r := 0; r < rows; r++ {
		var w int64
		for i := r * perRow; i < min((r+1)*perRow, n); i++ {
			if i > r*perRow {
				w += pfGap
			}
			w += layouts[i].cx
			rowH[r] = max(rowH[r], layouts[i].cy)
		}
		widest = max(widest, w)
	}
	if widest > bounds.Width {
		return pfLayoutResult{}, false
	}
	left := bounds.X + (bounds.Width-widest)/2
	for r := 0; r < rows; r++ {
		x := left
		for i := r * perRow; i < min((r+1)*perRow, n); i++ {
			layouts[i].x = x
			x += layouts[i].cx + pfGap
		}
	}

	// Detours and the zone each one runs in.
	index := make(map[string]int, len(steps))
	for i, s := range steps {
		index[s.id] = i
	}
	var detours []pfDetour
	for ci, c := range connections {
		from, okFrom := index[c.from]
		to, okTo := index[c.to]
		if !okFrom || !okTo || from == to || from >= n || to >= n {
			continue
		}
		a, b := rowOf(from), rowOf(to)
		if a == b && to == from+1 {
			continue // a straight connector between neighbours
		}
		d := pfDetour{conn: ci}
		switch {
		case a == b && to > from: // skip forward: under the row
			d.zone, d.srcDown, d.tgtDown = a, true, true
		case a == b: // loop back: over the row
			d.zone, d.srcDown, d.tgtDown = a-1, false, false
		case b == a+1: // to the row below
			d.zone, d.srcDown, d.tgtDown = a, true, false
		case b == a-1: // to the row above
			d.zone, d.srcDown, d.tgtDown = b, false, true
		default:
			return pfLayoutResult{}, false
		}
		right := layouts[to].x+layouts[to].cx/2 > layouts[from].x+layouts[from].cx/2
		d.srcX = pfDetourAttachX(steps[from], layouts[from], right)
		d.tgtX = pfDetourAttachX(steps[to], layouts[to], !right)
		detours = append(detours, d)
	}

	// Lanes: within a zone the shortest detour first, and no two detours on
	// one lane where their runs would overlap.
	span := func(d pfDetour) (int64, int64) { return min(d.srcX, d.tgtX), max(d.srcX, d.tgtX) }
	order := make([]int, len(detours))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(x, y int) bool {
		lx, hx := span(detours[order[x]])
		ly, hy := span(detours[order[y]])
		return hx-lx < hy-ly
	})
	lanes := make(map[int]int, rows+1) // zone -> lane count
	placed := make([]bool, len(detours))
	for _, i := range order {
		lo, hi := span(detours[i])
		level := 0
		for clash := true; clash; {
			clash = false
			for j, o := range detours {
				if !placed[j] || o.zone != detours[i].zone || o.level != level {
					continue
				}
				if olo, ohi := span(o); lo <= ohi && olo <= hi {
					clash = true
					level++
					break
				}
			}
		}
		detours[i].level, placed[i] = level, true
		lanes[detours[i].zone] = max(lanes[detours[i].zone], level+1)
	}

	// Heights: rows keep theirs; a zone with lanes takes a gap per lane (and
	// one more between two rows, so the last lane clears the row below); a
	// gap between rows with no lane keeps the plain row gap.
	zoneH := func(z int, gap int64) int64 {
		interior := z >= 0 && z < rows-1
		switch {
		case lanes[z] > 0 && interior:
			return gap * int64(lanes[z]+1)
		case lanes[z] > 0:
			return gap * int64(lanes[z])
		case interior:
			return pfGap
		}
		return 0
	}
	total := func(gap int64) int64 {
		var h int64
		for r := 0; r < rows; r++ {
			h += rowH[r]
		}
		for z := -1; z < rows; z++ {
			h += zoneH(z, gap)
		}
		return h
	}
	gap := pfDetourLaneGap
	for gap > pfDetourMinLaneGap && total(gap) > bounds.Height {
		gap -= 12700
	}
	gap = max(gap, pfDetourMinLaneGap)
	if total(gap) > bounds.Height {
		return pfLayoutResult{}, false
	}

	y := bounds.Y + (bounds.Height-total(gap))/2
	zoneTop := make(map[int]int64, rows+1)
	zoneTop[-1] = y
	y += zoneH(-1, gap)
	rowTop := make([]int64, rows)
	for r := 0; r < rows; r++ {
		rowTop[r] = y
		for i := r * perRow; i < min((r+1)*perRow, n); i++ {
			layouts[i].y = y + (rowH[r]-layouts[i].cy)/2
		}
		y += rowH[r]
		zoneTop[r] = y
		y += zoneH(r, gap)
	}

	edge := func(i int, down bool) int64 {
		if down {
			return layouts[i].y + layouts[i].cy
		}
		return layouts[i].y
	}
	for i := range detours {
		d := &detours[i]
		c := connections[d.conn]
		d.srcY, d.tgtY = edge(index[c.from], d.srcDown), edge(index[c.to], d.tgtDown)
		if d.zone < 0 {
			// Over the first row: lanes count upward from the row.
			d.laneY = rowTop[0] - gap*int64(d.level+1)
		} else {
			// Under row zone: measured from the row's own bottom line, so a
			// short step's stub still reaches the lane.
			d.laneY = zoneTop[d.zone] + gap*int64(d.level+1)
		}
	}
	return pfLayoutResult{steps: layouts, direction: "horizontal", detours: detours}, true
}

// pfDetourLabelBounds is where a detour's label sits: beside the stub that
// leaves the source, on the side the detour runs toward, just outside the
// row.
func pfDetourLabelBounds(d pfDetour, labelW, labelH int64) pptx.RectEmu {
	x := d.srcX + pfDetourLabelPad
	if d.tgtX < d.srcX {
		x = d.srcX - pfDetourLabelPad - labelW
	}
	y := d.srcY + pfDetourLabelPad
	if !d.srcDown {
		y = d.srcY - pfDetourLabelPad - labelH
	}
	return pptx.RectEmu{X: x, Y: y, CX: labelW, CY: labelH}
}

// pfDetourSegments are the three straight runs of a detour, as from / to
// points; the last one carries the arrowhead.
func pfDetourSegments(d pfDetour) [3][4]int64 {
	return [3][4]int64{
		{d.srcX, d.srcY, d.srcX, d.laneY},
		{d.srcX, d.laneY, d.tgtX, d.laneY},
		{d.tgtX, d.laneY, d.tgtX, d.tgtY},
	}
}

// pfGenerateDetour draws a detour as three straight connectors, the last with
// the arrowhead, and returns them with the next free shape id. A bent
// connector preset cannot leave and enter on the same side of two shapes
// without an adjustment PowerPoint and LibreOffice read differently; three
// lines are the same in both.
func pfGenerateDetour(d pfDetour, conn processFlowConnection, nextID uint32) ([][]byte, uint32) {
	line := pptx.Line{Width: pfConnectorWidth, Fill: pfConnectorFill(), Cap: "sq"}
	if conn.style == "dashed" {
		line.Dash = "dash"
	}
	var out [][]byte
	for i, seg := range pfDetourSegments(d) {
		x1, y1, x2, y2 := seg[0], seg[1], seg[2], seg[3]
		opts := pptx.ConnectorOptions{
			ID:       nextID,
			Name:     fmt.Sprintf("Flow Connector %d", nextID),
			Geometry: pptx.GeomStraightConnector1,
			Bounds:   pptx.RectEmu{X: min(x1, x2), Y: min(y1, y2), CX: abs64(x2 - x1), CY: abs64(y2 - y1)},
			Line:     line,
			FlipH:    x2 < x1,
			FlipV:    y2 < y1,
		}
		if i == 2 {
			opts.TailEnd = &pptx.ArrowHead{Type: "triangle", W: patterns.ProcessFlowConnectorHead, Len: patterns.ProcessFlowConnectorHead}
		}
		b, err := pptx.GenerateConnector(opts)
		if err != nil {
			slog.Warn("process flow detour failed", "error", err)
			continue
		}
		out = append(out, b)
		nextID++
	}
	return out, nextID
}

// pfGenerateDetourLabel draws a detour's label beside its first stub.
func pfGenerateDetourLabel(shapeID uint32, d pfDetour, label, font string) []byte {
	labelW, labelH := pfConnLabelSize(label, font)
	b, err := pptx.GenerateShape(pfConnLabelShape(shapeID, label, pfDetourLabelBounds(d, labelW, labelH)))
	if err != nil {
		slog.Warn("process flow detour label failed", "error", err)
		return nil
	}
	return b
}

// pfDetourFor returns the detour of connection ci, if it has one.
func pfDetourFor(detours []pfDetour, ci int) (pfDetour, bool) {
	for _, d := range detours {
		if d.conn == ci {
			return d, true
		}
	}
	return pfDetour{}, false
}
