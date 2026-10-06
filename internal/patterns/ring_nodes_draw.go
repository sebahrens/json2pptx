package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// Node-ring drawing shared by the circular family: N circles on a ring joined
// by curved arrows (cycle-nodes, and cycle-intake's discrete-step loop). Every
// builder takes a ringSpec and returns shape-grid layers of the ring cell, so
// a caller composes them with its own layers in the z-order it needs. Segment
// rings (blockArc bands) live in ring_draw.go.
// ---------------------------------------------------------------------------

const (
	// Node diameters as a share of the ring square's side, by node count.
	ringNodeDiaWide   = 0.26 // 3-5 nodes
	ringNodeDiaMedium = 0.22 // 6 nodes
	ringNodeDiaDense  = 0.18 // 7-8 nodes

	// ringNodeClearDeg is the angle left free between a node's edge and the
	// arrow that meets it.
	ringNodeClearDeg = 4.0

	// Link arrows (circularArrow), as shares of the ring square's side: a thin
	// shaft with a small head. The head's half width stays well under the
	// preset's 0.125 limit and the shaft under the head, where LibreOffice and
	// PowerPoint draw the same outline.
	ringLinkShaft    = 0.010
	ringLinkHeadHalf = 0.026
	ringLinkHeadLen  = 0.050 // along the arc
	// ringLinkHeadShare is the most of a link's arc its head may take.
	ringLinkHeadShare = 0.45

	// ringLinkTintPct is the dk1 coverage of a link arrow: a mid grey, so the
	// arrows read as structure and the one solid accent stays the highlight.
	ringLinkTintPct = 45

	// ringCircleTextShare is the side of the square inscribed in a circle, the
	// text rectangle of an ellipse, as a share of its diameter.
	ringCircleTextShare = math.Sqrt2 / 2
)

// ringNodeDiameter is the default node diameter for n nodes on a full ring.
func ringNodeDiameter(n int) float64 {
	switch {
	case n <= 5:
		return ringNodeDiaWide
	case n == 6:
		return ringNodeDiaMedium
	default:
		return ringNodeDiaDense
	}
}

// newRingNodeSpec is a full ring of n nodes of the given diameter starting at
// 12 o'clock whose circles touch the square's edge: the centreline radius is
// 0.5 − diameter/2.
func newRingNodeSpec(n int, diameter float64, clockwise bool) ringSpec {
	s := newRingSpec(n)
	s.Clockwise = clockwise
	s.GapDeg = 0
	s.Radius = 0.5 - diameter/2
	s.Thickness = diameter
	return s
}

// ringLayerFrame converts a ring frame into a layer frame, pulled inside the
// cell by the float error trigonometry leaves.
func ringLayerFrame(f ringFrame) jsonschema.LayerFrameInput {
	x, y := math.Max(f.X, 0), math.Max(f.Y, 0)
	return jsonschema.LayerFrameInput{X: x, Y: y, W: math.Min(f.W, 1-x), H: math.Min(f.H, 1-y)}
}

// ringNodePara is one paragraph of a ring shape's text.
type ringNodePara struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
}

// ringNodeTextJSON is a text payload with one alignment for every paragraph.
// insetPt < 0 keeps the writer's uniform shape margin (clamped by the writer
// in a small circle to what still holds one line); 0 and above sets all four
// insets explicitly, for unfilled label frames and for circles whose text was
// measured against the inscribed square.
func ringNodeTextJSON(align, vAlign string, insetPt float64, paras ...ringNodePara) json.RawMessage {
	for i := range paras {
		paras[i].Align = align
	}
	obj := map[string]any{"paragraphs": paras, "align": align, "vertical_align": vAlign}
	if insetPt >= 0 {
		for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
			obj[k] = insetPt
		}
	}
	data, _ := json.Marshal(obj)
	return data
}

// ringNodePaint is how one node is painted: its fill, its text (nil for a
// bare circle) and an optional icon overlay (centred when there is no text).
type ringNodePaint struct {
	Fill json.RawMessage
	Text json.RawMessage
	Icon *jsonschema.IconInput
}

// ringNodeLayers is one ellipse layer per node, named "node-1".."node-N" in
// node order, painted by paint(node index). Filled nodes carry no outline.
func ringNodeLayers(nodes []ringNode, paint func(i int) ringNodePaint) []jsonschema.LayerInput {
	out := make([]jsonschema.LayerInput, len(nodes))
	for i, n := range nodes {
		p := paint(n.Index)
		out[i] = jsonschema.LayerInput{
			Name:  fmt.Sprintf("node-%d", n.Index+1),
			Frame: ringLayerFrame(n.Frame),
			Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: p.Fill, Line: noLine, Text: p.Text, Icon: p.Icon},
		}
	}
	return out
}

// ringLinkArrow is the circularArrow from node n to the next one on s: its
// shaft starts ringNodeClearDeg past n's edge and its tip stops the same angle
// short of the next node. ok is false for a node without a link (the last one
// of an open arc).
func ringLinkArrow(s ringSpec, n ringNode) (arrow ringArrow, ok bool) {
	if !n.HasLink {
		return ringArrow{}, false
	}
	// On a dense ring the gap between two nodes is short: the head gives way
	// (keeping its proportions) so the arrow still has a shaft.
	arc := degToRad(ringTravelDeg(n.LinkFromDeg, n.LinkToDeg, s.Clockwise)) * s.Radius
	headLen := math.Min(ringLinkHeadLen, ringLinkHeadShare*arc)
	headHalf := ringLinkHeadHalf * headLen / ringLinkHeadLen
	return s.arrow(n.LinkFromDeg, n.LinkToDeg, math.Min(ringLinkShaft, headHalf), headHalf, radToDeg(headLen/s.Radius)), true
}

// ringLinkArrowLayers is one circularArrow layer per link between
// consecutive nodes, named "link-1".."link-N" (link-k leaves node k), filled
// with fill and without an outline. On a counter-clockwise ring the layers are
// mirrored (flip_h): the preset only points clockwise.
func ringLinkArrowLayers(s ringSpec, nodes []ringNode, fill json.RawMessage) []jsonschema.LayerInput {
	out := make([]jsonschema.LayerInput, 0, len(nodes))
	for _, n := range nodes {
		a, ok := ringLinkArrow(s, n)
		if !ok {
			continue
		}
		out = append(out, jsonschema.LayerInput{
			Name:  fmt.Sprintf("link-%d", n.Index+1),
			Frame: ringLayerFrame(a.Frame),
			Shape: &jsonschema.ShapeSpecInput{Geometry: "circularArrow", Fill: fill, Line: noLine, Adjustments: a.Adjustments, FlipH: a.FlipH},
		})
	}
	return out
}

// ringLinkFillJSON is the default fill of the link arrows.
func ringLinkFillJSON() json.RawMessage {
	return neutralFillJSON(ringLinkTintPct)
}

// ringInnerBadgeFrame is a circle of the given diameter centred on the inner
// edge of node n (the point of the node nearest the ring's centre), where it
// meets neither a link arrow nor the square's edge.
func ringInnerBadgeFrame(s ringSpec, n ringNode, nodeDiameter, diameter float64) ringFrame {
	return s.frameAt(n.Deg, s.Radius-nodeDiameter/2, diameter, diameter)
}

// ringNodeCentreFrame is the circle left free inside a ring of nodes: centred on
// the ring, reaching to margin short of whatever reaches inward furthest
// (inward is the distance from the centreline to that edge — half the node
// diameter, plus half an inner badge when there is one).
func ringNodeCentreFrame(s ringSpec, inward, margin float64) ringFrame {
	r := math.Max(s.Radius-inward-margin, 0.01)
	return ringFrame{X: 0.5 - r, Y: 0.5 - r, W: 2 * r, H: 2 * r}
}

// ringCentreLabelLayer is the unfilled circle that carries a ring's centre
// label ("centre"): the text sits in the circle's inscribed square.
func ringCentreLabelLayer(frame ringFrame, text json.RawMessage) jsonschema.LayerInput {
	return jsonschema.LayerInput{
		Name:  "centre",
		Frame: ringLayerFrame(frame),
		Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"none"`), Line: noLine, Text: text},
	}
}

// ringFitCircleLabel returns the largest of sizes (descending) at which label,
// set bold in the body face, keeps every word on one line and fits within
// maxLines lines inside the text square of a circle diaPt across with insetPt
// of margin; fits is false when not even the last size does (it is returned).
func ringFitCircleLabel(ctx ExpandContext, label string, sizes []float64, diaPt, insetPt float64, maxLines int) (size float64, fits bool) {
	inner := diaPt*ringCircleTextShare - 2*insetPt
	for _, s := range sizes {
		size = s
		if inner <= 0 {
			continue
		}
		lines := measuredLines(label, ctx.Theme.BodyFont, true, s, inner)
		if lines > maxLines || float64(lines)*s*contentLineHeight > inner || ringBreaksWord(ctx, label, s, inner) {
			continue
		}
		// The writer's own measure, with a 1pt margin so the probe starts at
		// one line rather than at the uniform shape margin.
		text := ringNodeTextJSON("ctr", "ctr", 1, ringNodePara{Content: label, Size: s, Bold: true})
		if writtenFitHeightPt(ctx.themeFonts(), text, inner+2, 0) > inner+2 {
			continue
		}
		return s, true
	}
	return size, false
}

// ringBreaksWord reports whether a word of label is wider than widthPt at
// sizePt bold.
func ringBreaksWord(ctx ExpandContext, label string, sizePt, widthPt float64) bool {
	for _, word := range strings.Fields(label) {
		if measuredLines(word, ctx.Theme.BodyFont, true, sizePt, widthPt) > 1 {
			return true
		}
	}
	return false
}
