package patterns

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// Shared ring drawing for the circular pattern family (cycle-ring,
// cycle-intake, cycle-figure-eight). ring_common.go is the geometry; this file
// turns a ringSpec and its items into shape-grid pieces:
//
//   - the layers of ONE ring cell: segments (blockArc, or circularArrow for
//     the chasing-arrows look), numbered badges on the centreline, a text
//     layer in the hole;
//   - the cells that go beside the ring on the lattice: a label (bold label
//     over a muted description) and the accent numeral that ties it to its
//     badge.
//
// A pattern builds its own ringSpec (a full ring, or an open arc for one lobe
// of a figure eight), calls the layer builders, joins them with ringCell and
// places that cell and the label cells with ringLattice.
// ---------------------------------------------------------------------------

const (
	// ringSegmentTint is the neutral step of a segment that is not the
	// highlight: the structural-box tint, visible on white paper without
	// competing with the one solid accent.
	ringSegmentTint = NeutralTint16

	// ringInkContrastMin is the bar accent ink must clear on its ground
	// (the generator's contrast pass swaps anything under it).
	ringInkContrastMin = 4.5

	ringBadgeMinPt   = 24.0 // smallest badge that holds a 12pt numeral
	ringBadgeMaxPt   = 30.0
	ringBadgeBandFr  = 0.6  // badge diameter as a share of the band's width
	ringBandMinPt    = 30.0 // a band thinner than this is widened to carry its badge
	ringBandMaxFrac  = 0.30 // … up to this share of the square's side
	ringNumberColPt  = 20.0 // width of the numeral cell beside a label
	ringLabelNearPt  = 6.0  // label inset on the numeral's side
	ringLabelFarPt   = 2.0  // label inset on the far side
	ringLabelInsetTB = 3.0  // top / bottom inset of a label row
	ringLabelSpacePt = 1.0  // space under the bold label line
	ringLabelSlackPt = 2.0  // rounding slack on a measured label row
	ringMutedAlpha   = 70.0 // opacity of a description (the agenda's muted line)

	ringArrowShaftFrac = 0.62 // arrow shaft as a share of the band's width
	ringArrowHeadShare = 0.35 // head length as a share of the segment's sweep
	ringArrowHeadMax   = 22.0 // … capped at this many degrees

	ringCentreBoxFrac  = 0.72 // centre text box as a share of the hole's diameter (≈ its inscribed square)
	ringCentreInsetPt  = 2.0
	ringCentreSpacePt  = 2.0
	ringCentreMaxPt    = scaleLeadPt
	ringCentreSubPt    = scaleBodyPt
	ringFrameRoundUnit = 1e6 // layer frames are rounded to 1e-6 of the square
)

// ringPaint is how one ring is painted.
type ringPaint struct {
	// Arrows draws every segment as a circularArrow whose tip chases the next
	// one, instead of a blockArc.
	Arrows bool
	// Accents is the accent of each item, indexed like items: its badge's
	// fill and, for the highlight, its segment's solid fill.
	Accents []string
	// Tinted fills a segment with the light tint of its own accent
	// (cell_accent_mode alternate / progressive) instead of the neutral step.
	Tinted bool
	// Highlight is the position in items of the one solid-accent segment;
	// negative = none.
	Highlight int
	// BadgeDia is the badge diameter as a fraction of the square's side
	// (ringBadgeDia picks it from the band).
	BadgeDia float64
	// NumberPt is the badge numeral's size (default 12).
	NumberPt float64
	// NumberFrom is the numeral of the first item's badge (default 1): a
	// second ring of a figure eight continues where the first stopped.
	NumberFrom int
	// Prefix is prepended to the layer names ("segment-1", "badge-1") when a
	// slide carries more than one ring.
	Prefix string
}

func (p ringPaint) accent(ctx ExpandContext, i int) string {
	if i >= 0 && i < len(p.Accents) && p.Accents[i] != "" {
		return p.Accents[i]
	}
	return ctx.DefaultAccent()
}

func (p ringPaint) number(i int) int {
	from := p.NumberFrom
	if from == 0 {
		from = 1
	}
	return from + i
}

// layer converts a ring frame to a layer frame, rounded and kept inside the
// cell (trigonometry leaves it a hair outside).
func (f ringFrame) layer() jsonschema.LayerFrameInput {
	r := func(v float64) float64 { return math.Round(v*ringFrameRoundUnit) / ringFrameRoundUnit }
	x, y := math.Max(r(f.X), 0), math.Max(r(f.Y), 0)
	w, h := r(f.W), r(f.H)
	if x+w > 1 {
		x = math.Max(1-w, 0)
	}
	if y+h > 1 {
		y = math.Max(1-h, 0)
	}
	return jsonschema.LayerFrameInput{X: x, Y: y, W: math.Min(w, 1), H: math.Min(h, 1)}
}

// ringBandFrac is the band thickness (fraction of the square's side) a ring
// of sidePt takes for the wanted thickness: a small ring gets a wider band so
// it still carries its badge.
func ringBandFrac(want, sidePt float64) float64 {
	if sidePt <= 0 {
		return want
	}
	return math.Max(want, math.Min(ringBandMaxFrac, ringBandMinPt/sidePt))
}

// ringBadgeDia is the badge diameter (fraction of the square's side) for a
// band of the given thickness on a square of sidePt.
func ringBadgeDia(thickness, sidePt float64) float64 {
	if sidePt <= 0 {
		return thickness * ringBadgeBandFr
	}
	pt := math.Min(math.Max(thickness*sidePt*ringBadgeBandFr, ringBadgeMinPt), ringBadgeMaxPt)
	return pt / sidePt
}

// ringSegmentTone is the fill of item i's segment: the solid accent for the
// highlight, otherwise the neutral step (or the item's own light tint).
func ringSegmentTone(ctx ExpandContext, p ringPaint, i int) fillTone {
	switch {
	case i == p.Highlight:
		return fillTone{Color: p.accent(ctx, i)}
	case p.Tinted:
		return inactiveTintTone(p.accent(ctx, i))
	default:
		return neutralTone(ringSegmentTint)
	}
}

// ringSegmentLayers is one layer per item: a blockArc in the band's frame, or
// with p.Arrows a circularArrow from the segment's start to its end. Filled,
// no outline; at most the highlight is a solid accent.
func ringSegmentLayers(ctx ExpandContext, spec ringSpec, items []ringItem, p ringPaint) []jsonschema.LayerInput {
	out := make([]jsonschema.LayerInput, 0, len(items))
	for i, it := range items {
		shape := &jsonschema.ShapeSpecInput{Fill: ringSegmentTone(ctx, p, i).fillJSON(), Line: noLine}
		frame := spec.bandFrame()
		if p.Arrows {
			sweep := ringTravelDeg(it.StartDeg, it.EndDeg, spec.Clockwise)
			a := spec.arrow(it.StartDeg, it.EndDeg, spec.Thickness*ringArrowShaftFrac, spec.Thickness/2,
				math.Min(sweep*ringArrowHeadShare, ringArrowHeadMax))
			shape.Geometry, shape.Adjustments, shape.FlipH = "circularArrow", a.Adjustments, a.FlipH
			frame = a.Frame
		} else {
			shape.Geometry, shape.Adjustments = "blockArc", spec.segmentAdj(it)
		}
		out = append(out, jsonschema.LayerInput{
			Name:  fmt.Sprintf("%ssegment-%d", p.Prefix, p.number(i)),
			Frame: frame.layer(),
			Shape: shape,
		})
	}
	return out
}

// ringBadgeLayers is one numbered circle per item on the band's centreline at
// the middle of its segment: filled with the item's accent (ink measured
// against it), or on the highlight — already a solid accent — with the page
// colour and the accent as ink.
func ringBadgeLayers(ctx ExpandContext, spec ringSpec, items []ringItem, p ringPaint) []jsonschema.LayerInput {
	size := shapegrid.EffectiveTextSizePt(ResolveSize(p.NumberPt, scaleBodyPt))
	dia := p.BadgeDia
	if dia <= 0 {
		dia = spec.Thickness * ringBadgeBandFr
	}
	out := make([]jsonschema.LayerInput, 0, len(items))
	for i, it := range items {
		accent := p.accent(ctx, i)
		fill := fillTone{Color: accent}
		ink := readableTextOn(ctx, fill, "lt1")
		if i == p.Highlight {
			fill = fillTone{Color: "lt1"}
			ink = accentInkOnTone(ctx, accent, fill, ringInkContrastMin)
		}
		out = append(out, jsonschema.LayerInput{
			Name:  fmt.Sprintf("%sbadge-%d", p.Prefix, p.number(i)),
			Frame: spec.badgeFrame(it, dia).layer(),
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "ellipse",
				Fill:     fill.fillJSON(),
				Line:     noLine,
				Text:     ringTextJSON("ctr", "ctr", nil, ringPara{Content: fmt.Sprintf("%d", p.number(i)), Size: size, Bold: true, Color: ink}),
			},
		})
	}
	return out
}

// ringCentreFrame is the text box in the ring's hole: a square a little
// larger than the one inscribed in the hole, which a centred label of a few
// short lines never fills to its corners.
func ringCentreFrame(spec ringSpec) ringFrame {
	side := 2 * (spec.Radius - spec.Thickness/2) * ringCentreBoxFrac
	return ringFrame{X: 0.5 - side/2, Y: 0.5 - side/2, W: side, H: side}
}

// ringCentreTextPt is the width and height the centre label may take on a
// square of sidePt, inside ringCentreFrame's own small inset.
func ringCentreTextPt(spec ringSpec, sidePt float64) (w, h float64) {
	box := ringCentreFrame(spec).W*sidePt - 2*ringCentreInsetPt
	return math.Max(box, 0), math.Max(box, 0)
}

// ringCentreFit sizes the centre label for a square of sidePt: the largest
// size from ringCentreMaxPt down to the 12pt floor at which no word breaks and
// label plus sublabel (12pt) fit the hole. fits is false when the floor does
// not hold them.
func ringCentreFit(ctx ExpandContext, spec ringSpec, sidePt float64, label, sublabel string) (sizePt float64, fits bool) {
	w, h := ringCentreTextPt(spec, sidePt)
	if sublabel != "" {
		lines := measuredLines(sublabel, ctx.Theme.BodyFont, false, ringCentreSubPt, w)
		h -= float64(lines)*ringCentreSubPt*contentLineHeight + ringCentreSpacePt
	}
	if w <= 0 || h <= 0 {
		return shapegrid.MinTextSizePt, false
	}
	if label == "" {
		return shapegrid.MinTextSizePt, true
	}
	size, ok := sshFitHubLabel(ctx, label, ringCentreMaxPt, w, h)
	if !ok {
		return size, false
	}
	// The model's line count is not the writer's: a face set wider than the
	// theme font's metrics (abstract's Tenorite, where it is substituted)
	// broke a one-word label in a small hole — "Monthl / y" in a 50% region
	// (go-slide-creator-t5ndl). The label keeps the largest size at which the
	// writer needs no more lines than the model counted and every word has
	// ringCentreWordShare of the width to spare.
	for s := size; s >= shapegrid.MinTextSizePt; s-- {
		if ringCentreHolds(ctx, label, s, w, h) {
			return s, true
		}
	}
	return shapegrid.MinTextSizePt, false
}

// ringCentreWordShare is the share of the hole's text width a word of the
// centre label may take by the theme font's metrics: the rest is what a
// renderer's own, wider face may add.
const ringCentreWordShare = 0.85

// ringCentreHolds reports whether the centre label at sizePt keeps every word
// whole in a text box w wide with room to spare, and takes no more height
// than h by the writer's own measure.
func ringCentreHolds(ctx ExpandContext, label string, sizePt, w, h float64) bool {
	if ringBreaksWord(ctx, label, sizePt, w*ringCentreWordShare) {
		return false
	}
	lines := measuredLines(label, ctx.Theme.BodyFont, true, sizePt, w)
	if float64(lines)*sizePt*contentLineHeight > h {
		return false
	}
	inset := [4]float64{ringCentreInsetPt, ringCentreInsetPt, ringCentreInsetPt, ringCentreInsetPt}
	text := ringTextJSON("ctr", "ctr", &inset, ringPara{Content: pptx.ConvertMarkdownEmphasis(label), Size: sizePt, Bold: true})
	written := writtenFitHeightPt(ctx.themeFonts(), text, w+2*ringCentreInsetPt, 0)
	return written <= float64(lines)*sizePt*sizingLineSpacing+2*ringCentreInsetPt+ringLabelSlackPt
}

// ringCentreLayer is the unfilled text layer in the hole: the bold label at
// labelPt over the muted sublabel. It returns false when both are empty.
func ringCentreLayer(spec ringSpec, label, sublabel string, labelPt float64, prefix string) (jsonschema.LayerInput, bool) {
	var paras []ringPara
	if label != "" {
		paras = append(paras, ringPara{Content: pptx.ConvertMarkdownEmphasis(label), Size: labelPt, Bold: true, Color: "dk1", SpaceAfter: ringCentreSpacePt})
	}
	if sublabel != "" {
		paras = append(paras, ringPara{Content: pptx.ConvertMarkdownEmphasis(sublabel), Size: ringCentreSubPt, Color: "dk1", Alpha: ringMutedAlpha})
	}
	if len(paras) == 0 {
		return jsonschema.LayerInput{}, false
	}
	inset := ringCentreInsetPt
	return jsonschema.LayerInput{
		Name:  prefix + "centre",
		Frame: ringCentreFrame(spec).layer(),
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     noLine,
			Text:     ringTextJSON("ctr", "ctr", &[4]float64{inset, inset, inset, inset}, paras...),
		},
	}, true
}

// ringCell is the ring's grid cell: a transparent canvas whose layers share
// the centred square (fit "contain"), so the ring stays round whatever
// rectangle the lattice finally gives it.
func ringCell(layers ...[]jsonschema.LayerInput) *jsonschema.GridCellInput {
	cell := &jsonschema.GridCellInput{Fit: "contain"}
	for _, l := range layers {
		cell.Layers = append(cell.Layers, l...)
	}
	return cell
}

// ringLabelText is a label row's text: the bold label over its muted
// description (left off when empty), top-anchored so the numeral cell beside
// it sits on the label's line. align is "l" for a label right of the ring and
// "r" for one left of it; the wider inset is on the numeral's side.
func ringLabelText(label, description string, labelPt, descPt float64, align string) json.RawMessage {
	paras := []ringPara{{Content: pptx.ConvertMarkdownEmphasis(label), Size: labelPt, Bold: true, Color: "dk1"}}
	if description != "" {
		paras[0].SpaceAfter = ringLabelSpacePt
		paras = append(paras, ringPara{Content: pptx.ConvertMarkdownEmphasis(description), Size: descPt, Color: "dk1", Alpha: ringMutedAlpha})
	}
	insets := [4]float64{ringLabelNearPt, ringLabelInsetTB, ringLabelFarPt, ringLabelInsetTB}
	if align == "r" {
		insets[0], insets[2] = ringLabelFarPt, ringLabelNearPt
	}
	return ringTextJSON(align, "t", &insets, paras...)
}

// ringLabelTextCell wraps a label text in its unfilled lattice cell.
func ringLabelTextCell(text json.RawMessage) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine, Text: text},
	}
}

// ringNumberCell is the numeral beside a label (ringNumberColPt wide), in the
// item's accent where that reads on the page: the cue that ties the label to
// its badge. align is the label's align, so the numeral hugs the label.
func ringNumberCell(ctx ExpandContext, number int, accent string, sizePt float64, align string) *jsonschema.GridCellInput {
	ink := accentInkOnTone(ctx, accent, fillTone{Color: "lt1"}, ringInkContrastMin)
	numAlign := "r"
	if align == "r" {
		numAlign = "l"
	}
	insets := [4]float64{0, ringLabelInsetTB, 0, ringLabelInsetTB}
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     noLine,
			Text:     ringTextJSON(numAlign, "t", &insets, ringPara{Content: fmt.Sprintf("%d", number), Size: sizePt, Bold: true, Color: ink}),
		},
	}
}

// ringLabelNeedPt is the height a label text needs in a cell widthPt wide:
// the larger of the writer's own measure (the size it would store unshrunk)
// and the capacity model's line count, plus rounding slack.
func ringLabelNeedPt(ctx ExpandContext, label, description string, labelPt, descPt, widthPt float64) float64 {
	text := ringLabelText(label, description, labelPt, descPt, "l")
	inner := widthPt - ringLabelNearPt - ringLabelFarPt
	model := 2*ringLabelInsetTB + float64(paragraphLines(ctx, sizedPara{text: label, sizePt: labelPt, bold: true}, inner+2*sizingInsetLRPt))*labelPt*sizingLineSpacing
	if description != "" {
		model += ringLabelSpacePt + float64(paragraphLines(ctx, sizedPara{text: description, sizePt: descPt}, inner+2*sizingInsetLRPt))*descPt*sizingLineSpacing
	}
	return math.Ceil(math.Max(model, writtenFitHeightPt(ctx.themeFonts(), text, widthPt, 0)) + ringLabelSlackPt)
}

// ringPara is one paragraph of a ring text.
type ringPara struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Alpha      float64 `json:"alpha,omitempty"`
	Align      string  `json:"align,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
}

type ringTextObj struct {
	Paragraphs    []ringPara `json:"paragraphs"`
	Align         string     `json:"align"`
	VerticalAlign string     `json:"vertical_align"`
	InsetLeft     *float64   `json:"inset_left,omitempty"`
	InsetTop      *float64   `json:"inset_top,omitempty"`
	InsetRight    *float64   `json:"inset_right,omitempty"`
	InsetBottom   *float64   `json:"inset_bottom,omitempty"`
}

// ringTextJSON is a text object of paragraphs sharing one alignment. insets
// (left, top, right, bottom in points) replace the uniform shape margin; nil
// keeps it (a badge, whose margin the writer clamps as a degenerate shape).
func ringTextJSON(align, vAlign string, insets *[4]float64, paras ...ringPara) json.RawMessage {
	for i := range paras {
		paras[i].Align = align
	}
	obj := ringTextObj{Paragraphs: paras, Align: align, VerticalAlign: vAlign}
	if insets != nil {
		obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom = &insets[0], &insets[1], &insets[2], &insets[3]
	}
	data, _ := json.Marshal(obj)
	return data
}
