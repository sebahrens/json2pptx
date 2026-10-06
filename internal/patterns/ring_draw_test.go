package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// The drawing helpers work for any ringSpec, not only cycle-ring's full ring:
// an open arc (one lobe of a figure eight) with its own numbering and layer
// prefix gets one blockArc and one badge per item, inside the cell.
func TestRingDraw_OpenArcWithPrefixAndNumbering(t *testing.T) {
	spec := newRingArcSpec(3, -60, 240, true).withBand(0.9, 0.18)
	items, err := spec.items()
	if err != nil {
		t.Fatal(err)
	}
	paint := ringPaint{Accents: []string{"accent2", "accent2", "accent2"}, Highlight: 1, BadgeDia: 0.1, NumberFrom: 5, Prefix: "right-"}
	segments := ringSegmentLayers(ExpandContext{}, spec, items, paint)
	badges := ringBadgeLayers(ExpandContext{}, spec, items, paint)
	if len(segments) != 3 || len(badges) != 3 {
		t.Fatalf("%d segments, %d badges", len(segments), len(badges))
	}
	for i, want := range []string{"right-segment-5", "right-segment-6", "right-segment-7"} {
		s := segments[i]
		if s.Name != want || s.Shape.Geometry != "blockArc" || string(s.Shape.Line) != `"none"` {
			t.Errorf("segment %d = %s %s line %s", i, s.Name, s.Shape.Geometry, s.Shape.Line)
		}
		// The band's frame is the bounding square of its outer circle (0.9).
		if f := s.Frame; f.W != 0.9 || f.H != 0.9 || f.X != 0.05 || f.Y != 0.05 {
			t.Errorf("segment %d frame %+v, want the 0.9 square centred", i, f)
		}
		solid := string(s.Shape.Fill) == `"accent2"`
		if solid != (i == 1) {
			t.Errorf("segment %d fill %s: only the highlight is a solid accent", i, s.Shape.Fill)
		}
	}
	for i, b := range badges {
		if !strings.Contains(string(b.Shape.Text), `"content":"`+string(rune('5'+i))+`"`) {
			t.Errorf("badge %d text %s", i, b.Shape.Text)
		}
		if b.Frame.X < 0 || b.Frame.Y < 0 || b.Frame.X+b.Frame.W > 1 || b.Frame.Y+b.Frame.H > 1 {
			t.Errorf("badge %d frame %+v leaves the cell", i, b.Frame)
		}
	}
	if string(badges[1].Shape.Fill) != `"lt1"` || string(badges[0].Shape.Fill) != `"accent2"` {
		t.Errorf("badge fills %s / %s: the highlight's badge is the page colour", badges[0].Shape.Fill, badges[1].Shape.Fill)
	}

	cell := ringCell(segments, badges)
	if cell.Fit != "contain" || len(cell.Layers) != 6 || cell.Shape != nil {
		t.Errorf("ring cell fit %q with %d layers", cell.Fit, len(cell.Layers))
	}
}

func TestRingDraw_FrameStaysInsideTheCell(t *testing.T) {
	for _, f := range []ringFrame{{X: -1e-9, Y: 0.95, W: 0.1, H: 0.1}, {X: 0.4, Y: 0.4, W: 0.2, H: 0.2}, {X: 0, Y: 0, W: 1.0000001, H: 1}} {
		l := f.layer()
		if l.X < 0 || l.Y < 0 || l.X+l.W > 1 || l.Y+l.H > 1 || l.W <= 0 || l.H <= 0 {
			t.Errorf("%+v became %+v, outside the cell", f, l)
		}
	}
}

func TestRingDraw_BandAndBadgeFollowTheRingSize(t *testing.T) {
	// A 294pt ring keeps the wanted 20% band; a 120pt one widens it to carry a badge.
	if got := ringBandFrac(0.20, 294); got != 0.20 {
		t.Errorf("band on a 294pt ring = %v", got)
	}
	if got := ringBandFrac(0.14, 120); got*120 < ringBandMinPt-0.01 || got > ringBandMaxFrac {
		t.Errorf("band on a 120pt ring = %v (%.1fpt)", got, got*120)
	}
	for _, side := range []float64{120, 180, 294, 360} {
		dia := ringBadgeDia(ringBandFrac(0.20, side), side) * side
		if dia < ringBadgeMinPt-0.01 || dia > ringBadgeMaxPt+0.01 {
			t.Errorf("badge on a %.0fpt ring is %.1fpt, want %v-%vpt", side, dia, ringBadgeMinPt, ringBadgeMaxPt)
		}
	}
}

// Label cells: the numeral hugs its label (right-aligned before a
// left-aligned label, and mirrored), both top-anchored with the same top
// inset so the numeral sits on the label's first line; a longer text needs a
// taller row.
func TestRingDraw_LabelCells(t *testing.T) {
	type textObj struct {
		Paragraphs []struct {
			Content string  `json:"content"`
			Bold    bool    `json:"bold"`
			Alpha   float64 `json:"alpha"`
			Align   string  `json:"align"`
		} `json:"paragraphs"`
		VerticalAlign string  `json:"vertical_align"`
		InsetTop      float64 `json:"inset_top"`
		InsetLeft     float64 `json:"inset_left"`
		InsetRight    float64 `json:"inset_right"`
	}
	parse := func(c *jsonschema.GridCellInput) textObj {
		var o textObj
		if err := json.Unmarshal(c.Shape.Text, &o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	for align, numAlign := range map[string]string{"l": "r", "r": "l"} {
		label := parse(ringLabelTextCell(ringLabelText("Plan", "Set the target", 14, 12, align)))
		num := parse(ringNumberCell(ExpandContext{}, 3, "accent1", 14, align))
		if len(label.Paragraphs) != 2 || !label.Paragraphs[0].Bold || label.Paragraphs[1].Alpha != ringMutedAlpha || label.Paragraphs[0].Align != align {
			t.Errorf("align %s label = %+v", align, label)
		}
		if num.Paragraphs[0].Content != "3" || num.Paragraphs[0].Align != numAlign {
			t.Errorf("align %s numeral = %+v", align, num)
		}
		if label.VerticalAlign != "t" || num.VerticalAlign != "t" || label.InsetTop != num.InsetTop {
			t.Errorf("align %s: label and numeral are not anchored alike", align)
		}
		near := label.InsetLeft
		if align == "r" {
			near = label.InsetRight
		}
		if near != ringLabelNearPt {
			t.Errorf("align %s: the inset on the numeral's side is %v", align, near)
		}
	}
	if got := parse(ringLabelTextCell(ringLabelText("Plan", "", 14, 12, "l"))); len(got.Paragraphs) != 1 {
		t.Errorf("a label without a description has %d paragraphs", len(got.Paragraphs))
	}
	short := ringLabelNeedPt(ExpandContext{}, "Plan", "", 14, 12, 180)
	long := ringLabelNeedPt(ExpandContext{}, "Plan", "Set the target and the hypothesis to test in the pilot plant", 14, 12, 180)
	if short < 14*sizingLineSpacing || long <= short+12 {
		t.Errorf("label needs: %vpt alone, %vpt with a two-line description", short, long)
	}
}

func TestRingDraw_CentreLayer(t *testing.T) {
	spec := newRingSpec(4)
	if _, ok := ringCentreLayer(spec, "", "", 14, ""); ok {
		t.Error("an empty centre draws a layer")
	}
	l, ok := ringCentreLayer(spec, "Flywheel", "FY26", 16, "left-")
	if !ok || l.Name != "left-centre" || string(l.Shape.Fill) != `"none"` {
		t.Fatalf("centre layer = %+v", l)
	}
	// The hole is 0.6 of the square; the text box sits inside it.
	if l.Frame.W >= 0.6 || l.Frame.W != l.Frame.H {
		t.Errorf("centre frame %+v", l.Frame)
	}
	size, fits := ringCentreFit(ExpandContext{}, spec, 300, "Flywheel", "FY26")
	if !fits || size < 12 || size > ringCentreMaxPt {
		t.Errorf("centre fit = %v %v", size, fits)
	}
	if _, fits := ringCentreFit(ExpandContext{}, spec, 120, "Institutionalisation", "of continuous improvement"); fits {
		t.Error("a long word fits the hole of a 120pt ring")
	}
}
