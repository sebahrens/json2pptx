package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// Shared ring / loop geometry for the circular pattern family (cycle-ring,
// cycle-nodes, cycle-intake, cycle-figure-eight, radial-hub,
// concentric-rings). Pure functions: no I/O, no ExpandContext, the same
// output for the same input.
//
// # Coordinates
//
// Angles are degrees, 0 = 3 o'clock, positive = clockwise (the slide's y axis
// points down), so 12 o'clock is -90 (= 270), 6 o'clock is 90 and 9 o'clock
// is 180. That is the OOXML preset convention, so an angle converts to an
// adjust value by a multiplication alone.
//
// Shapes of one ring are layers of ONE shape-grid cell. A layer's frame is a
// ringFrame: x, y, w, h as fractions (0..1) of the cell's fitted square, with
// the ring's centre at (0.5, 0.5). ringSpec.Radius and ringSpec.Thickness use
// the same unit (fractions of the square's side).
//
// # OOXML preset semantics (verified against a soffice render of a
// hand-written shape_grid deck on midnight-blue, 2026-10-06)
//
// blockArc — a ring segment in the shape's own bounding box:
//
//	adj1  start angle, 60000ths of a degree, 0 = 3 o'clock, clockwise
//	adj2  end angle, same unit; the band is drawn CLOCKWISE from adj1 to
//	      adj2, always (adj1=270°, adj2=0° is the top-right quarter;
//	      adj1=300°, adj2=240° is a 300° ring open at 12 o'clock;
//	      adj1 == adj2 is the full ring)
//	adj3  band thickness, 100000ths of the shape's SHORT side min(w, h)
//	      (not of the radius): 50000 reaches the centre (a pie wedge),
//	      20000 on a square leaves a hole of 60% of the side
//
// The outer edge is the ellipse inscribed in the bounding box, so the frame
// of a blockArc layer is the bounding square of the ring's OUTER circle and
// the band's centreline radius is side × (0.5 − adj3/200000).
//
// circularArrow — an arc shaft with a triangular head, always pointing
// CLOCKWISE:
//
//	adj1  shaft thickness, 100000ths of min(w, h); pinned to at most 2 × adj5
//	adj2  angular LENGTH of the head, 60000ths of a degree (not an angle on
//	      the circle): the tip sits at adj3 + adj2
//	adj3  angle where the shaft ends and the head begins (pinned to >= 1)
//	adj4  start angle of the shaft
//	adj5  half the head's radial width, 100000ths of min(w, h), at most
//	      25000. The head's outer corner touches the bounding box, so the
//	      centreline radius is side × (0.5 − adj5/100000).
//
// (The bead's guess — adj2 = end angle, adj3 = start angle, adj4 = head
// sweep — is wrong; the list above is what renders.) A counter-clockwise
// arrow is the clockwise arrow between the mirrored angles (180° − a) with
// flip_h set: ringSpec.arrow returns both.
//
// rotation on a shape is degrees clockwise about the centre of its frame: a
// blockArc from 270° to 0° with rotation 90 renders as the one from 0° to
// 90°. A `triangle` points up at rotation 0.
// ---------------------------------------------------------------------------

const (
	ringMinItems = 3 // a full ring holds 3..8 items
	ringMaxItems = 8

	ringDefaultStartDeg  = -90.0 // 12 o'clock
	ringDefaultGapDeg    = 6.0
	ringDefaultThickness = 0.20 // band width as a share of the square's side
	ringFullSweepDeg     = 360.0

	// ringMinSidePt is the smallest ring square a pattern draws: under it the
	// band of an 8-item ring cannot hold a 12pt numeral in its badge.
	ringMinSidePt = 120.0

	// ringPoleBandDeg is how far either side of 12 / 6 o'clock a label still
	// goes above / below the ring instead of beside it.
	ringPoleBandDeg = 15.0

	ringColGapPt    = 0.01 // lattice tracks are geometric: a hairline gap
	ringEdgeMergePt = 1.0  // lattice edges closer than this collapse into one

	ooxmlAngleUnits = 60000.0  // preset angle units per degree
	ooxmlFracUnits  = 100000.0 // preset units per 1.0 of the short side

	ringAngleEps = 1e-9

	ringSideRight  = "right"
	ringSideLeft   = "left"
	ringSideTop    = "top"
	ringSideBottom = "bottom"
)

// ringFrame is a layer frame inside the ring cell's fitted square: fractions
// of the square's side, origin at its top-left corner.
type ringFrame struct {
	X, Y, W, H float64
}

// ringSpec describes one ring (or one open arc of it) inside the square.
// Build it with newRingSpec or newRingArcSpec and then override fields; the
// zero value is not a usable ring (Clockwise would be false and StartDeg
// 3 o'clock).
type ringSpec struct {
	N         int     // items on the ring
	StartDeg  float64 // where item 0 begins (the seam of a full ring)
	SweepDeg  float64 // angular extent the items share; 360 = full ring, less = an open arc
	Clockwise bool    // direction of travel from item 0 to item 1
	GapDeg    float64 // angular gap between neighbouring segments
	Radius    float64 // centreline radius, fraction of the square's side
	Thickness float64 // band width, fraction of the square's side
}

// newRingSpec is a full clockwise ring of n items starting at 12 o'clock with
// the default gap, whose band fills the square (outer edge on the square's
// edge).
func newRingSpec(n int) ringSpec {
	return ringSpec{
		N:         n,
		StartDeg:  ringDefaultStartDeg,
		SweepDeg:  ringFullSweepDeg,
		Clockwise: true,
		GapDeg:    ringDefaultGapDeg,
		Radius:    0.5 - ringDefaultThickness/2,
		Thickness: ringDefaultThickness,
	}
}

// newRingArcSpec is newRingSpec with the n items spread over the arc that
// runs from fromDeg to toDeg in the given direction (a ring with a wedge
// left out, such as one lobe of a figure eight). fromDeg == toDeg is the
// full ring.
func newRingArcSpec(n int, fromDeg, toDeg float64, clockwise bool) ringSpec {
	s := newRingSpec(n)
	s.StartDeg = fromDeg
	s.Clockwise = clockwise
	s.SweepDeg = ringTravelDeg(fromDeg, toDeg, clockwise)
	if s.SweepDeg < ringAngleEps {
		s.SweepDeg = ringFullSweepDeg
	}
	return s
}

// withBand returns s with the band set from its outer diameter and its
// thickness, both fractions of the square's side: a ring that leaves room
// around it for badges or nodes that reach past the band.
func (s ringSpec) withBand(outerDiameter, thickness float64) ringSpec {
	s.Thickness = thickness
	s.Radius = outerDiameter/2 - thickness/2
	return s
}

// full reports whether the items close the circle.
func (s ringSpec) full() bool {
	return s.SweepDeg <= 0 || s.SweepDeg >= ringFullSweepDeg-ringAngleEps
}

// sweep is the angular extent the items share.
func (s ringSpec) sweep() float64 {
	if s.full() {
		return ringFullSweepDeg
	}
	return s.SweepDeg
}

// dir is +1 for clockwise travel and -1 for counter-clockwise.
func (s ringSpec) dir() float64 {
	if s.Clockwise {
		return 1
	}
	return -1
}

// outerRadius is the radius of the band's outer edge.
func (s ringSpec) outerRadius() float64 {
	return s.Radius + s.Thickness/2
}

// validate checks the item count against the arc and the band against the
// square.
func (s ringSpec) validate() error {
	lo := 1
	if s.full() {
		lo = ringMinItems
	}
	if s.N < lo || s.N > ringMaxItems {
		return fmt.Errorf("ring: %d items; a full ring holds %d-%d and an open arc 1-%d", s.N, ringMinItems, ringMaxItems, ringMaxItems)
	}
	if s.Radius <= 0 || s.Thickness <= 0 || s.Thickness > 2*s.Radius+ringAngleEps {
		return fmt.Errorf("ring: radius %.4f with thickness %.4f is not a band", s.Radius, s.Thickness)
	}
	if s.outerRadius() > 0.5+ringAngleEps {
		return fmt.Errorf("ring: outer radius %.4f leaves the square (at most 0.5)", s.outerRadius())
	}
	if s.GapDeg < 0 {
		return fmt.Errorf("ring: negative gap %.2f", s.GapDeg)
	}
	return nil
}

// ringItem is one segment of a ring. StartDeg and EndDeg are in the order of
// travel (item 0 towards item 1), normalised to [0, 360); a segment never
// crosses the seam at ringSpec.StartDeg.
type ringItem struct {
	Index                    int
	StartDeg, EndDeg, MidDeg float64
	Side                     string // right | left | top | bottom: where its label goes (labelSide of MidDeg)
}

// items splits the ring into N equal segments separated by GapDeg. A full
// ring has N gaps and its seam gap is centred on StartDeg, so item 0 begins
// at StartDeg + GapDeg/2 and a ring of four is symmetric about both axes; an
// open arc has N-1 gaps and its first and last segments end exactly on the
// arc's ends.
func (s ringSpec) items() ([]ringItem, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	n := float64(s.N)
	gaps, lead := n, s.GapDeg/2
	if !s.full() {
		gaps, lead = n-1, 0
	}
	seg := (s.sweep() - gaps*s.GapDeg) / n
	if seg <= ringAngleEps {
		return nil, fmt.Errorf("ring: %d items with a %.1f° gap leave no segment on a %.1f° arc", s.N, s.GapDeg, s.sweep())
	}
	d := s.dir()
	out := make([]ringItem, s.N)
	for i := range out {
		a := s.StartDeg + d*(lead+float64(i)*(seg+s.GapDeg))
		mid := normDeg(a + d*seg/2)
		out[i] = ringItem{Index: i, StartDeg: normDeg(a), EndDeg: normDeg(a + d*seg), MidDeg: mid, Side: labelSide(mid)}
	}
	return out, nil
}

// ringItems is the bead's name for ringSpec.items.
func ringItems(s ringSpec) ([]ringItem, error) {
	return s.items()
}

// ringNode is one circle centred on the ring's centreline, with the arc that
// joins it to the next node.
type ringNode struct {
	Index int
	Deg   float64   // angle of the node's centre, [0, 360)
	Side  string    // labelSide of Deg
	Frame ringFrame // the node circle

	// The arc to the next node in the order of travel, already shortened by
	// the node's own angular half-width plus the clearance, so a link drawn
	// from LinkFromDeg to LinkToDeg touches neither circle. HasLink is false
	// for the last node of an open arc.
	HasLink                bool
	LinkFromDeg, LinkToDeg float64
}

// nodes places N circles of the given diameter (fraction of the square's
// side) on the centreline: on a full ring node 0 sits AT StartDeg and the
// rest follow at 360/N; on an open arc the first and last nodes sit on the
// arc's ends (a single node at its middle). clearDeg is the extra angle left
// free between a circle and the link that meets it. GapDeg is not used.
func (s ringSpec) nodes(diameter, clearDeg float64) ([]ringNode, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if diameter <= 0 || diameter > 4*s.Radius {
		return nil, fmt.Errorf("ring: node diameter %.4f does not fit a centreline of radius %.4f", diameter, s.Radius)
	}
	d := s.dir()
	step, first := s.sweep()/float64(s.N), 0.0
	if !s.full() {
		if s.N == 1 {
			step, first = 0, s.sweep()/2
		} else {
			step = s.sweep() / float64(s.N-1)
		}
	}
	// A circle of diameter D centred on a circle of radius R cuts it where
	// the chord from its centre is D/2 long: 2·asin(D/4R) to either side.
	half := 2*radToDeg(math.Asin(diameter/(4*s.Radius))) + math.Max(clearDeg, 0)
	if s.N > 1 && step-2*half <= ringAngleEps {
		return nil, fmt.Errorf("ring: %d nodes of diameter %.4f leave no arc between them", s.N, diameter)
	}
	out := make([]ringNode, s.N)
	for i := range out {
		deg := normDeg(s.StartDeg + d*(first+float64(i)*step))
		out[i] = ringNode{Index: i, Deg: deg, Side: labelSide(deg), Frame: s.frameAt(deg, s.Radius, diameter, diameter)}
		if s.N > 1 && (s.full() || i < s.N-1) {
			out[i].HasLink = true
			out[i].LinkFromDeg = normDeg(deg + d*half)
			out[i].LinkToDeg = normDeg(deg + d*(step-half))
		}
	}
	return out, nil
}

// ringNodeItems turns nodes into zero-width items (Start = End = Mid = the
// node's angle) so ringLabelRows can lay out their labels.
func ringNodeItems(nodes []ringNode) []ringItem {
	out := make([]ringItem, len(nodes))
	for i, n := range nodes {
		out[i] = ringItem{Index: n.Index, StartDeg: n.Deg, EndDeg: n.Deg, MidDeg: n.Deg, Side: n.Side}
	}
	return out
}

// ---------------------------------------------------------------------------
// Frames and adjust values
// ---------------------------------------------------------------------------

// pointOnCircle is the point at deg on a circle of radius r about (cx, cy),
// with y pointing down.
func pointOnCircle(cx, cy, r, deg float64) (x, y float64) {
	sin, cos := math.Sincos(degToRad(deg))
	return cx + r*cos, cy + r*sin
}

// frameAt is a w × h frame centred on the point at deg, radius away from the
// ring's centre (all fractions of the square's side).
func (s ringSpec) frameAt(deg, radius, w, h float64) ringFrame {
	x, y := pointOnCircle(0.5, 0.5, radius, deg)
	return ringFrame{X: x - w/2, Y: y - h/2, W: w, H: h}
}

// bandFrame is the frame of every blockArc layer of the ring: the bounding
// square of its outer circle.
func (s ringSpec) bandFrame() ringFrame {
	r := s.outerRadius()
	return ringFrame{X: 0.5 - r, Y: 0.5 - r, W: 2 * r, H: 2 * r}
}

// badgeFrame is a circle of the given diameter centred on the band's
// centreline at the middle of the segment.
func (s ringSpec) badgeFrame(it ringItem, diameter float64) ringFrame {
	return s.frameAt(it.MidDeg, s.Radius, diameter, diameter)
}

// blockArcAdj is the adjust map of a blockArc drawn clockwise from fromDeg to
// toDeg whose band is thickness (a fraction of the shape's own short side,
// at most 0.5) wide.
func blockArcAdj(fromDeg, toDeg, thickness float64) map[string]int64 {
	return map[string]int64{
		"adj1": ooxmlAngle(fromDeg),
		"adj2": ooxmlAngle(toDeg),
		"adj3": int64(math.Round(math.Min(math.Max(thickness, 0), 0.5) * ooxmlFracUnits)),
	}
}

// segmentAdj is the blockArc adjust map of one segment, for a layer in
// bandFrame. A counter-clockwise ring swaps the segment's ends, because the
// preset always draws clockwise.
func (s ringSpec) segmentAdj(it ringItem) map[string]int64 {
	from, to := it.StartDeg, it.EndDeg
	if !s.Clockwise {
		from, to = to, from
	}
	return blockArcAdj(from, to, s.Thickness/(2*s.outerRadius()))
}

// ringArrow is one circularArrow layer.
type ringArrow struct {
	Frame       ringFrame
	Adjustments map[string]int64
	FlipH       bool // set for counter-clockwise travel: the preset only points clockwise
}

// arrow is a circularArrow whose shaft runs on the ring's centreline from
// fromDeg and whose TIP lands on toDeg, travelling in the ring's direction.
// shaft is the shaft width and headHalf half the head's radial width
// (fractions of the square's side; headHalf is raised to shaft/2, where the
// head is as wide as the shaft); headDeg is the head's angular length, cut to
// half the travel when the arc is shorter than that.
func (s ringSpec) arrow(fromDeg, toDeg, shaft, headHalf, headDeg float64) ringArrow {
	headHalf = math.Max(headHalf, shaft/2)
	travel := ringTravelDeg(fromDeg, toDeg, s.Clockwise)
	if travel < ringAngleEps {
		travel = ringFullSweepDeg
	}
	headDeg = math.Min(math.Max(headDeg, 0), travel/2)
	// The head's outer corner touches the frame, so the frame is the
	// centreline circle grown by half the head.
	side := 2 * (s.Radius + headHalf)
	start, tip := fromDeg, toDeg
	if !s.Clockwise {
		start, tip = 180-fromDeg, 180-toDeg
	}
	return ringArrow{
		Frame: ringFrame{X: 0.5 - side/2, Y: 0.5 - side/2, W: side, H: side},
		Adjustments: map[string]int64{
			"adj1": int64(math.Round(shaft / side * ooxmlFracUnits)),
			"adj2": int64(math.Round(headDeg * ooxmlAngleUnits)),
			"adj3": max(ooxmlAngle(tip-headDeg), 1),
			"adj4": ooxmlAngle(start),
			"adj5": int64(math.Round(math.Min(headHalf/side, 0.25) * ooxmlFracUnits)),
		},
		FlipH: !s.Clockwise,
	}
}

// arrowhead is a `triangle` layer centred on the centreline at deg and
// rotated to point along the ring's direction of travel. base is the width of
// the side across the travel and length the distance from that side to the
// tip (fractions of the square's side); the frame is the unrotated base ×
// length box. Give it a length clearly above its base: a triangle as long as
// it is wide is almost equilateral and reads as pointing three ways.
func (s ringSpec) arrowhead(deg, base, length float64) (frame ringFrame, rotationDeg float64) {
	rotationDeg = normDeg(deg) // counter-clockwise at 3 o'clock points up
	if s.Clockwise {
		rotationDeg = normDeg(deg + 180)
	}
	return s.frameAt(deg, s.Radius, base, length), rotationDeg
}

// ---------------------------------------------------------------------------
// Label placement
// ---------------------------------------------------------------------------

// labelSide is where the label of an item at midDeg goes: above the ring
// within 15° of 12 o'clock, below it within 15° of 6 o'clock, otherwise on
// the side the item is on.
func labelSide(midDeg float64) string {
	d := normDeg(midDeg)
	switch {
	case math.Abs(d-90) <= ringPoleBandDeg+ringAngleEps:
		return ringSideBottom
	case math.Abs(d-270) <= ringPoleBandDeg+ringAngleEps:
		return ringSideTop
	case d < 90 || d > 270:
		return ringSideRight
	default:
		return ringSideLeft
	}
}

// labelSideLR is labelSide for a layout with label columns only: an item
// exactly at 12 o'clock goes to the side the ring travels to next and one
// exactly at 6 o'clock to the other, so a clockwise ring of four nodes puts
// two labels on each side.
func labelSideLR(midDeg float64, clockwise bool) string {
	d := normDeg(midDeg)
	right := d < 90-ringAngleEps || d > 270+ringAngleEps
	switch {
	case math.Abs(d-270) <= ringAngleEps:
		right = clockwise
	case math.Abs(d-90) <= ringAngleEps:
		right = !clockwise
	}
	if right {
		return ringSideRight
	}
	return ringSideLeft
}

// ringSidesLR returns a copy of items with every Side recomputed by
// labelSideLR.
func ringSidesLR(items []ringItem, clockwise bool) []ringItem {
	out := make([]ringItem, len(items))
	for i, it := range items {
		it.Side = labelSideLR(it.MidDeg, clockwise)
		out[i] = it
	}
	return out
}

// ringSquareSide is the side of the ring square in a content area of
// contentW × contentH points that keeps reserveLeft and reserveRight for the
// label columns: min(height, width − reserves). When that is under
// ringMinSidePt the reserves give way and the square takes ringMinSidePt, or
// the whole short side of a content area smaller than that.
func ringSquareSide(contentW, contentH, reserveLeft, reserveRight float64) float64 {
	side := math.Min(contentH, contentW-reserveLeft-reserveRight)
	if side < ringMinSidePt {
		side = math.Min(ringMinSidePt, math.Min(contentW, contentH))
	}
	return math.Max(side, 0)
}

// ringReserve is the room kept around the ring square for labels, in points.
type ringReserve struct {
	Left, Right, Top, Bottom float64
}

// ringSquareBox places the ring square in a contentW × contentH area: its
// side (ringSquareSide after the top and bottom reserves) and its top-left
// corner, centred in what the reserves leave. When the reserves had to give
// way the square is centred in the whole area.
func ringSquareBox(contentW, contentH float64, r ringReserve) (x, y, side float64) {
	side = ringSquareSide(contentW, contentH-r.Top-r.Bottom, r.Left, r.Right)
	side = math.Min(side, math.Min(contentW, contentH))
	x = r.Left + (contentW-r.Left-r.Right-side)/2
	if x < 0 || x+side > contentW || contentW-r.Left-r.Right < side {
		x = (contentW - side) / 2
	}
	y = r.Top + (contentH-r.Top-r.Bottom-side)/2
	if y < 0 || y+side > contentH || contentH-r.Top-r.Bottom < side {
		y = (contentH - side) / 2
	}
	return x, y, side
}

// ringExclusion is a vertical band on one side that label rows must stay out
// of (cycle-intake's feed arrow).
type ringExclusion struct {
	Side   string // right | left
	Y0, Y1 float64
}

// ringRowsSpec is the label-row layout of one ring, in points measured from
// the top of the pattern's block.
type ringRowsSpec struct {
	CentreY  float64         // y of the ring's centre
	RadiusPt float64         // radius the label anchors sit on (the badge / node centres)
	RowPt    float64         // height of a label row
	Heights  []float64       // optional per-item heights, indexed by ringItem.Index; 0 or missing = RowPt
	GapPt    float64         // least gap between two rows on one side
	Top      float64         // rows stay inside [Top, Bottom]
	Bottom   float64         //
	Exclude  []ringExclusion // bands the rows of a side keep out of
	// Outward moves a left / right row away from the ring's equator by this
	// share of half its height times sin² of its item's angle: 0 centres every
	// row on its anchor, 1 sets the row of an item at 12 o'clock wholly above
	// its anchor and one at 6 o'clock wholly below (rows at 3 and 9 o'clock
	// stay centred), clear of whatever leaves the item along the ring.
	Outward float64
}

// ringRow is the label row of one item.
type ringRow struct {
	Index   int
	Side    string
	AnchorY float64 // y of the item's badge / node centre
	Y       float64 // y of the row's centre (its box is Y − H/2 .. Y + H/2)
	H       float64
}

// ringLabelRows gives every item a label row centred as near its anchor as
// the other rows allow: rows on one side (left / right) keep at least GapPt
// between them, stay inside [Top, Bottom] and out of the side's exclusion
// bands, and move the least total distance (squared) from their anchors, so a
// symmetric ring keeps symmetric rows. An item whose Side is top or bottom
// keeps its anchor: the pattern sets that label above / below the ring. Rows
// come back in the order of items. fits is false when a side cannot hold its
// rows; they are then stacked from the top of the band they were given and
// run past its end.
func ringLabelRows(items []ringItem, spec ringRowsSpec) (rows []ringRow, fits bool) {
	rows = make([]ringRow, len(items))
	fits = true
	bySide := map[string][]int{}
	for i, it := range items {
		_, ay := pointOnCircle(0, spec.CentreY, spec.RadiusPt, it.MidDeg)
		h := spec.RowPt
		if it.Index >= 0 && it.Index < len(spec.Heights) && spec.Heights[it.Index] > 0 {
			h = spec.Heights[it.Index]
		}
		rows[i] = ringRow{Index: it.Index, Side: it.Side, AnchorY: ay, Y: ay, H: h}
		if it.Side == ringSideLeft || it.Side == ringSideRight {
			sin := math.Sin(degToRad(it.MidDeg))
			rows[i].Y += spec.Outward * sin * math.Abs(sin) * h / 2
			bySide[it.Side] = append(bySide[it.Side], i)
		}
	}
	for _, side := range []string{ringSideLeft, ringSideRight} {
		idx := bySide[side]
		if len(idx) == 0 {
			continue
		}
		sort.SliceStable(idx, func(a, b int) bool { return rows[idx[a]].Y < rows[idx[b]].Y })
		anchors, heights := make([]float64, len(idx)), make([]float64, len(idx))
		for k, i := range idx {
			anchors[k], heights[k] = rows[i].Y, rows[i].H
		}
		ys, ok := ringSpreadBands(anchors, heights, spec.GapPt, ringFreeBands(spec.Top, spec.Bottom, side, spec.Exclude))
		fits = fits && ok
		for k, i := range idx {
			rows[i].Y = ys[k]
		}
	}
	return rows, fits
}

// ringBand is a vertical interval rows may occupy.
type ringBand struct {
	y0, y1 float64
}

// ringFreeBands is [top, bottom] minus the exclusion bands of side, top to
// bottom.
func ringFreeBands(top, bottom float64, side string, exclude []ringExclusion) []ringBand {
	var cuts []ringBand
	for _, e := range exclude {
		if e.Side == side && e.Y1 > e.Y0 {
			cuts = append(cuts, ringBand{e.Y0, e.Y1})
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].y0 < cuts[j].y0 })
	var out []ringBand
	y := top
	for _, c := range cuts {
		if c.y0 > y {
			out = append(out, ringBand{y, math.Min(c.y0, bottom)})
		}
		y = math.Max(y, c.y1)
	}
	if y < bottom {
		out = append(out, ringBand{y, bottom})
	}
	if len(out) == 0 {
		out = []ringBand{{top, bottom}}
	}
	return out
}

// ringSpreadBands spreads rows (sorted by anchor) over one or more bands:
// each row starts in the band nearest its anchor, rows move to a neighbouring
// band while theirs is over-full and the neighbour has room, and every band
// then spreads its own rows.
func ringSpreadBands(anchors, heights []float64, gap float64, bands []ringBand) ([]float64, bool) {
	n := len(anchors)
	assign := make([]int, n)
	for i, a := range anchors {
		best, bestD := 0, math.Inf(1)
		for b, band := range bands {
			d := math.Max(math.Max(band.y0-a, a-band.y1), 0)
			if d < bestD {
				best, bestD = b, d
			}
		}
		assign[i] = best
	}
	// Anchors are sorted and bands ordered, so assign is non-decreasing and
	// the rows of band b are the run [lo, hi).
	run := func(b int) (lo, hi int) {
		lo = sort.SearchInts(assign, b)
		return lo, sort.SearchInts(assign, b+1)
	}
	need := func(lo, hi int) float64 {
		total := 0.0
		for i := lo; i < hi; i++ {
			total += heights[i]
		}
		return total + float64(max(hi-lo-1, 0))*gap
	}
	room := func(b int) float64 { return bands[b].y1 - bands[b].y0 }
	for moves := 0; moves < n*len(bands); moves++ {
		moved := false
		for b := range bands {
			lo, hi := run(b)
			if hi == lo || need(lo, hi) <= room(b)+ringAngleEps {
				continue
			}
			if b > 0 {
				if plo, _ := run(b - 1); need(plo, lo+1) <= room(b-1)+ringAngleEps {
					assign[lo], moved = b-1, true
					break
				}
			}
			if b < len(bands)-1 {
				if _, nhi := run(b + 1); need(hi-1, nhi) <= room(b+1)+ringAngleEps {
					assign[hi-1], moved = b+1, true
					break
				}
			}
		}
		if !moved {
			break
		}
	}
	out := make([]float64, n)
	fits := true
	for b, band := range bands {
		lo, hi := run(b)
		if hi == lo {
			continue
		}
		ys, ok := ringSpreadRows(anchors[lo:hi], heights[lo:hi], gap, band.y0, band.y1)
		copy(out[lo:hi], ys)
		fits = fits && ok
	}
	return out, fits
}

// ringSpreadRows returns the centre of each row of the given height so that
// consecutive rows (anchors must be sorted ascending) are at least gap apart,
// every row lies inside [top, bottom], and the sum of squared distances from
// the anchors is the least possible. fits is false when the rows are taller
// than the band together; they are then stacked from top.
func ringSpreadRows(anchors, heights []float64, gap, top, bottom float64) (ys []float64, fits bool) {
	n := len(anchors)
	if n == 0 {
		return nil, true
	}
	// offset[i] is the least distance from row 0's centre to row i's.
	offset := make([]float64, n)
	for i := 1; i < n; i++ {
		offset[i] = offset[i-1] + heights[i-1]/2 + gap + heights[i]/2
	}
	// With u[i] = y[i] − offset[i] the spacing rule is "u is non-decreasing":
	// the nearest such u is the isotonic regression of the shifted anchors.
	u := make([]float64, n)
	for i := range u {
		u[i] = anchors[i] - offset[i]
	}
	u = ringIsotonic(u)
	lo, hi := top+heights[0]/2, bottom-heights[n-1]/2-offset[n-1]
	fits = hi >= lo-ringAngleEps
	ys = make([]float64, n)
	for i := range ys {
		v := lo
		if fits {
			v = math.Min(math.Max(u[i], lo), math.Max(hi, lo))
		}
		ys[i] = v + offset[i]
	}
	return ys, fits
}

// ringIsotonic is the non-decreasing sequence nearest v in least squares
// (pool adjacent violators).
func ringIsotonic(v []float64) []float64 {
	type block struct {
		sum float64
		n   int
	}
	blocks := make([]block, 0, len(v))
	for _, x := range v {
		blocks = append(blocks, block{x, 1})
		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/float64(a.n) <= b.sum/float64(b.n) {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{a.sum + b.sum, a.n + b.n})
		}
	}
	out := make([]float64, 0, len(v))
	for _, b := range blocks {
		for i := 0; i < b.n; i++ {
			out = append(out, b.sum/float64(b.n))
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Lattice
// ---------------------------------------------------------------------------

// ringPlacement is one cell placed by its rectangle, in points from the
// top-left corner of the pattern's block.
type ringPlacement struct {
	X0, X1, Y0, Y1 float64
	Cell           *jsonschema.GridCellInput
}

// ringLattice turns freely placed rectangles into a shape grid: every
// rectangle edge becomes a lattice line (edges under ringEdgeMergePt apart
// collapse into one), the columns are percentages of width, the rows are
// fixed heights in points (min_height = max_height, so the block is exactly
// height tall and a ring cell that is square in points stays square), and
// each cell gets the col_span / row_span of the tracks it covers. Before a
// cell, one empty cell is emitted per free lattice column, so the shape
// grid's column cursor — which skips columns taken by a row span from above
// by itself — lands on the cell's first column; a row no cell starts in gets
// one empty cell. The placements' cells are modified (their spans are set).
//
// It is state-shift-hub's sshLattice made two-dimensional; that pattern keeps
// its own copy.
func ringLattice(places []ringPlacement, width, height float64) (*jsonschema.ShapeGridInput, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("ring lattice: %.1f × %.1fpt is not an area", width, height)
	}
	xs, ys := []float64{0, width}, []float64{0, height}
	for i, p := range places {
		if p.Cell == nil {
			return nil, fmt.Errorf("ring lattice: placement %d has no cell", i)
		}
		if p.X0 < -ringEdgeMergePt || p.Y0 < -ringEdgeMergePt || p.X1 > width+ringEdgeMergePt || p.Y1 > height+ringEdgeMergePt {
			return nil, fmt.Errorf("ring lattice: placement %d (%.1f-%.1f × %.1f-%.1fpt) leaves the %.1f × %.1fpt area", i, p.X0, p.X1, p.Y0, p.Y1, width, height)
		}
		xs = append(xs, p.X0, p.X1)
		ys = append(ys, p.Y0, p.Y1)
	}
	xs, ys = ringMergeEdges(xs, width), ringMergeEdges(ys, height)
	nCols, nRows := len(xs)-1, len(ys)-1
	if nCols > shapegrid.MaxColumns {
		return nil, fmt.Errorf("ring lattice: %d columns, more than the shape grid's %d", nCols, shapegrid.MaxColumns)
	}
	slots, occupied, err := ringLatticeSlots(places, xs, ys)
	if err != nil {
		return nil, err
	}

	rows := make([]jsonschema.GridRowInput, nRows)
	for r := range rows {
		h := math.Round((ys[r+1]-ys[r])*1000) / 1000
		rows[r] = jsonschema.GridRowInput{MinHeight: h, MaxHeight: h}
	}
	cursor := make([]int, nRows)
	for _, s := range slots {
		for c := cursor[s.r0]; c < s.c0; c++ {
			if !occupied[s.r0][c] {
				rows[s.r0].Cells = append(rows[s.r0].Cells, &jsonschema.GridCellInput{})
			}
		}
		s.cell.ColSpan, s.cell.RowSpan = 0, 0
		if span := s.c1 - s.c0; span > 1 {
			s.cell.ColSpan = span
		}
		if span := s.r1 - s.r0; span > 1 {
			s.cell.RowSpan = span
		}
		rows[s.r0].Cells = append(rows[s.r0].Cells, s.cell)
		cursor[s.r0] = s.c1
	}
	for r := range rows {
		if len(rows[r].Cells) == 0 {
			rows[r].Cells = []*jsonschema.GridCellInput{{}}
		}
	}

	cols := make([]float64, nCols)
	for c := range cols {
		cols[c] = math.Round((xs[c+1]-xs[c])/width*100*1000) / 1000
	}
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  ringColGapPt,
		RowGap:  ringColGapPt,
		Rows:    rows,
	}, nil
}

// ringSlot is a placement snapped to the lattice: the tracks [c0, c1) ×
// [r0, r1).
type ringSlot struct {
	c0, c1, r0, r1 int
	cell           *jsonschema.GridCellInput
}

// ringLatticeSlots snaps every placement to the lattice edges and returns the
// slots in grid order (by first row, then first column) with the tracks they
// occupy. A placement that covers no track, or one another already covers, is
// an error.
func ringLatticeSlots(places []ringPlacement, xs, ys []float64) ([]ringSlot, [][]bool, error) {
	slots := make([]ringSlot, len(places))
	occupied := make([][]bool, len(ys)-1)
	for r := range occupied {
		occupied[r] = make([]bool, len(xs)-1)
	}
	for i, p := range places {
		s := ringSlot{c0: ringNearestEdge(xs, p.X0), c1: ringNearestEdge(xs, p.X1), r0: ringNearestEdge(ys, p.Y0), r1: ringNearestEdge(ys, p.Y1), cell: p.Cell}
		if s.c1 <= s.c0 || s.r1 <= s.r0 {
			return nil, nil, fmt.Errorf("ring lattice: placement %d (%.1f-%.1f × %.1f-%.1fpt) collapsed on the lattice", i, p.X0, p.X1, p.Y0, p.Y1)
		}
		for r := s.r0; r < s.r1; r++ {
			for c := s.c0; c < s.c1; c++ {
				if occupied[r][c] {
					return nil, nil, fmt.Errorf("ring lattice: placement %d overlaps another at row %d column %d", i, r, c)
				}
				occupied[r][c] = true
			}
		}
		slots[i] = s
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].r0 != slots[j].r0 {
			return slots[i].r0 < slots[j].r0
		}
		return slots[i].c0 < slots[j].c0
	})
	return slots, occupied, nil
}

// ringMergeEdges sorts the edges, collapses those under ringEdgeMergePt apart
// and pins the first to 0 and the last to total.
func ringMergeEdges(edges []float64, total float64) []float64 {
	sort.Float64s(edges)
	merged := []float64{0}
	for _, e := range edges {
		if e-merged[len(merged)-1] >= ringEdgeMergePt {
			merged = append(merged, e)
		}
	}
	if len(merged) == 1 {
		return append(merged, total)
	}
	merged[len(merged)-1] = total
	return merged
}

// ringNearestEdge is the index of the lattice edge nearest x.
func ringNearestEdge(edges []float64, x float64) int {
	best, bestD := 0, math.Inf(1)
	for i, e := range edges {
		if d := math.Abs(e - x); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Angle helpers
// ---------------------------------------------------------------------------

// normDeg brings an angle into [0, 360), rounded to 1e-9 so sums of equal
// steps land on the same value whichever way they were added up.
func normDeg(deg float64) float64 {
	d := math.Mod(deg, ringFullSweepDeg)
	if d < 0 {
		d += ringFullSweepDeg
	}
	d = math.Round(d/ringAngleEps) * ringAngleEps
	if d >= ringFullSweepDeg {
		d = 0
	}
	return d
}

// ringTravelDeg is the angle travelled from fromDeg to toDeg in the given
// direction, in [0, 360).
func ringTravelDeg(fromDeg, toDeg float64, clockwise bool) float64 {
	if clockwise {
		return normDeg(toDeg - fromDeg)
	}
	return normDeg(fromDeg - toDeg)
}

// ooxmlAngle converts degrees to the preset unit (60000ths of a degree) in
// [0, 21600000).
func ooxmlAngle(deg float64) int64 {
	v := int64(math.Round(normDeg(deg) * ooxmlAngleUnits))
	if v >= int64(ringFullSweepDeg*ooxmlAngleUnits) {
		v = 0
	}
	return v
}

func degToRad(deg float64) float64 { return deg * math.Pi / 180 }

func radToDeg(rad float64) float64 { return rad * 180 / math.Pi }

// ---------------------------------------------------------------------------
// Labels at a constant gap from their circle
// ---------------------------------------------------------------------------
//
// A label beside a ring belongs to one circle: its node, its satellite, or
// the ring's outer edge at its own height. Its block (number cue included)
// starts ringLabelGapPt from that circle, measured horizontally where the
// circle reaches furthest within the row's height (ringBandEdgeX), so every
// label of every pattern of the family stands the same distance from its
// circle and the labels follow the curve: a side's labels share no column x.
//
// A label that follows the curve stands inside the ring's bounding square, so
// the ring cell cannot be that square on the lattice (two lattice cells never
// overlap). The ring cell is a ringSpinePt-wide column through the ring's
// centre instead, as tall as the ring, with fit "fit-height": its fitted
// bounds are the square of the cell's height centred on the spine, which is
// the ring square, and it is a square whatever width the lattice resolves at.
// No label reaches the spine: a label block ends at least the gap from the
// centre line.

const (
	// ringLabelGapPt is the gap between a circle and its label block for the
	// whole family (scaled to the template gutter by ExpandContext.Gap).
	ringLabelGapPt = 12.0
	// ringSpinePt is the width of the ring cell's lattice column: under twice
	// the label gap, over the lattice's edge-merge distance.
	ringSpinePt = 4.0
	// ringSpineFit keeps the ring cell's layers on the square of the cell's
	// height, centred on the spine.
	ringSpineFit = "fit-height"
	// ringSettlePasses bounds ringSettleRows.
	ringSettlePasses = 4
	// ringSettleWidthShare is the share of a row's width ringSettleRows
	// measures it at: a renderer's own face may run a little wider than the
	// measurer's, and a label that only just holds one line here would wrap
	// there and outgrow the row sized for it.
	ringSettleWidthShare = 0.94
)

// ringEdgeXAt is the x at which a circle of radius rPt about (cxPt, cyPt)
// ends on the given side (ringSideLeft, otherwise right) at the height yPt.
// Above and below the circle it is the centre's x.
func ringEdgeXAt(cxPt, cyPt, rPt, yPt float64, side string) float64 {
	half := 0.0
	if dy := math.Abs(yPt - cyPt); dy < rPt {
		half = math.Sqrt(rPt*rPt - dy*dy)
	}
	if side == ringSideLeft {
		return cxPt - half
	}
	return cxPt + half
}

// ringBandEdgeX is the furthest the circle reaches on the given side within
// the band y0Pt..y1Pt: its edge at the height of the band nearest its centre.
func ringBandEdgeX(cxPt, cyPt, rPt, y0Pt, y1Pt float64, side string) float64 {
	return ringEdgeXAt(cxPt, cyPt, rPt, math.Min(math.Max(cyPt, y0Pt), y1Pt), side)
}

// ringLabelEdgeX is where the label block of the row y0Pt..y1Pt begins (right
// of the circle) or ends (left of it): gapPt clear of the circle.
func ringLabelEdgeX(cxPt, cyPt, rPt, y0Pt, y1Pt, gapPt float64, side string) float64 {
	edge := ringBandEdgeX(cxPt, cyPt, rPt, y0Pt, y1Pt, side)
	if side == ringSideLeft {
		return edge - gapPt
	}
	return edge + gapPt
}

// ringSpinePlacement places a ring cell whose square is sidePt across, centred
// on cxPt with its top at y0Pt: the lattice column is the spine, and the
// cell's fit makes its layers' frame the whole square.
func ringSpinePlacement(cxPt, y0Pt, sidePt float64, cell *jsonschema.GridCellInput) ringPlacement {
	cell.Fit = ringSpineFit
	return ringPlacement{X0: cxPt - ringSpinePt/2, X1: cxPt + ringSpinePt/2, Y0: y0Pt, Y1: y0Pt + sidePt, Cell: cell}
}

// ringSettleRows is ringLabelRows for rows whose width depends on where they
// stand (a label that follows the curve is wider near a pole than at 3
// o'clock). spec.Heights are the heights at the narrowest width any row can
// get, indexed by ringItem.Index; they always hold, and are what comes back
// when nothing better settles. The left / right rows are then measured again
// at the width their place gives them, less a margin (widthOf; needAt measures
// item index at a width), and placed again, until every row holds its text at the width of
// the place it ends up in. need is the height each item's text takes in the
// rows returned, indexed like spec.Heights.
func ringSettleRows(items []ringItem, spec ringRowsSpec, widthOf func(ringRow) float64, needAt func(index int, widthPt float64) float64) (rows []ringRow, need []float64, fits bool) {
	base := append([]float64(nil), spec.Heights...)
	spec.Heights = base
	rows, fits = ringLabelRows(items, spec)
	if widthOf == nil || needAt == nil {
		return rows, base, fits
	}
	cur, curFits := rows, fits
	need = append([]float64(nil), base...)
	for pass := 0; pass < ringSettlePasses; pass++ {
		next, grew := append([]float64(nil), need...), false
		for _, r := range cur {
			if (r.Side != ringSideLeft && r.Side != ringSideRight) || r.Index < 0 || r.Index >= len(base) {
				continue
			}
			// A row never needs more than at the narrowest width.
			h := math.Min(needAt(r.Index, widthOf(r)*ringSettleWidthShare), base[r.Index])
			switch {
			case pass == 0:
				next[r.Index] = h
			case h > need[r.Index]+ringAngleEps:
				next[r.Index], grew = h, true
			}
		}
		if pass > 0 && !grew {
			// cur was placed at need, and every row holds where it stands.
			if curFits || !fits {
				return cur, need, curFits
			}
			break
		}
		need = next
		spec.Heights = need
		cur, curFits = ringLabelRows(items, spec)
	}
	return rows, base, fits
}

// ringProtectEdges keeps the lattice from moving a label block's gap edge.
// ringLattice collapses x edges under ringEdgeMergePt apart into the lower
// one, so a block's near edge that happens to lie that close to another cell's
// edge (a number cue is about as wide as the step between two rows on the
// curve) would move, and its gap with it. Here every other x edge of places
// within the merge distance of a gap edge is set onto it, and gap edges that
// close to each other meet at their mean, before the lattice sees them.
func ringProtectEdges(places []ringPlacement, gapEdges []float64) {
	if len(gapEdges) == 0 {
		return
	}
	keep := append([]float64(nil), gapEdges...)
	sort.Float64s(keep)
	// Runs of gap edges chained under the merge distance meet at their mean.
	snapped := make([]float64, 0, len(keep))
	for i := 0; i < len(keep); {
		j, sum := i, 0.0
		for ; j < len(keep) && (j == i || keep[j]-keep[j-1] < ringEdgeMergePt); j++ {
			sum += keep[j]
		}
		snapped = append(snapped, sum/float64(j-i))
		i = j
	}
	snap := func(x float64) float64 {
		for _, e := range snapped {
			if math.Abs(x-e) < ringEdgeMergePt+ringAngleEps {
				return e
			}
		}
		return x
	}
	for i := range places {
		places[i].X0, places[i].X1 = snap(places[i].X0), snap(places[i].X1)
	}
}
