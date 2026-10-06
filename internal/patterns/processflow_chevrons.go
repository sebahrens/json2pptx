package patterns

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

// Interlocking-arrow look (go-slide-creator-cuq95).
//
// A process flow used to be a row of equal mid-grey boxes joined by thin
// accent arrows: every step its own box, the sequence drawn by hairlines, and
// the emphasis one orange box among grey ones — the look of a generated
// flowchart. In the default "chevrons" style the shapes carry the order
// instead. A plain step is an arrow: the flow opens with a pentagon and every
// later step is a chevron whose notch tucks round the point before it, so the
// row reads as one band with no connector between its parts. The steps take a
// light tint of the accent, the one emphasised step the solid accent, and a
// full process-flow sets a large numeral over each label.
//
// Decisions stay diamonds, and the explicit chevron and arrow step types keep
// their own free-standing geometry (a deep notch, a block arrow); all take the
// same tint. The boxes-and-connectors flowchart is overrides.style "tinted".
const (
	// processFlowChevronGapPt is the column gap of the chevron look: the
	// slanted gap between one step's point and the next step's notch. It is
	// a hairline between interlocking shapes, so it is not scaled with the
	// template gutter.
	processFlowChevronGapPt = 4.0
	// processFlowChevronRowGapPt is the gap between the two rows of a bent
	// flow in the chevron look.
	processFlowChevronRowGapPt = 12.0
	// processFlowChevronInsetPt is a step label's margin left and right,
	// inside the preset's own text rectangle, which already stops short of
	// the point and the notch.
	processFlowChevronInsetPt = 6.0
	// The point (and notch) is processFlowNotchFrac of the step width,
	// between processFlowMinNotchPt and processFlowMaxNotchPt: a step that
	// holds a sentence is a card with a point, not a dart.
	processFlowNotchFrac  = 0.09
	processFlowMinNotchPt = 8.0
	processFlowMaxNotchPt = 18.0
	// processFlowNumeralGapPt is the space between a step's numeral and its
	// label.
	processFlowNumeralGapPt = 2.0
)

// processFlowChevronStyle reports whether the overrides select the default
// interlocking-arrow look.
func processFlowChevronStyle(ovr *ProcessFlowOverrides) bool {
	return ovr == nil || ovr.Style == "" || ovr.Style == processFlowStyleChevrons
}

// processFlowLook is the style a flow's step cells are built in and, for the
// chevron look, the geometry of its plain steps.
type processFlowLook struct {
	// chevrons is the default look; false is the flowchart (tinted / solid).
	chevrons bool
	// numerals sets a step numeral over each label (process-flow only, and
	// only when every step that is not a decision is a plain step).
	numerals  bool
	numeralPt float64
	// notchPt is the depth of every plain step's point and notch.
	notchPt float64
	lay     processFlowLayout
	steps   []ProcessFlowStep
}

// processFlowLookFor is the look of a flow whose steps are cellW wide.
// numbered asks for step numerals (process-flow; the compact band has none).
func processFlowLookFor(steps []ProcessFlowStep, ovr *ProcessFlowOverrides, lay processFlowLayout, cellW float64, numbered bool) processFlowLook {
	look := processFlowLook{chevrons: processFlowChevronStyle(ovr), lay: lay, steps: steps}
	if !look.chevrons {
		return look
	}
	look.notchPt = clampPt(math.Round(cellW*processFlowNotchFrac), processFlowMinNotchPt, processFlowMaxNotchPt)
	look.numerals = numbered
	for _, s := range steps {
		if s.Type == "chevron" || s.Type == "arrow" {
			look.numerals = false
		}
	}
	switch {
	case lay.perRow <= 4:
		look.numeralPt = 24
	case lay.perRow <= 6:
		look.numeralPt = 20
	default:
		look.numeralPt = 18
	}
	return look
}

// interlocks reports whether step i is drawn as an interlocking arrow.
func (l processFlowLook) interlocks(i int) bool {
	if !l.chevrons || i < 0 || i >= len(l.steps) {
		return false
	}
	return l.steps[i].Type == "" || l.steps[i].Type == "step"
}

// stepGeometry is plain step i's preset: a pentagon opens the flow, chevrons
// follow it.
func (l processFlowLook) stepGeometry(i int) string {
	if i == 0 {
		return "homePlate"
	}
	return "chevron"
}

// leftNeighbour is the step in the grid cell left of step i's, or -1.
func (l processFlowLook) leftNeighbour(i int) int {
	row, col := l.lay.cell(i)
	if col == 0 {
		return -1
	}
	for j := range l.steps {
		if r, c := l.lay.cell(j); r == row && c == col-1 {
			return j
		}
	}
	return -1
}

// returning reports whether step i is on the row that runs right to left.
func (l processFlowLook) returning(i int) bool {
	row, _ := l.lay.cell(i)
	return l.lay.snake && row == 1
}

// bleedPt is how far plain step i's shape reaches left of its column, over
// the gap. On a row running left to right that side is its notch, which
// tucks round whatever the step before it ends in. On the returning row the
// shape is mirrored and that side is its point, which needs a notch to reach
// into: the step it leads to must be a chevron too.
func (l processFlowLook) bleedPt(i int) float64 {
	if !l.interlocks(i) {
		return 0
	}
	left := l.leftNeighbour(i)
	if left < 0 {
		return 0
	}
	if l.returning(i) && !l.interlocks(left) && l.steps[left].Type != "chevron" {
		return 0
	}
	return l.notchPt
}

// textRectPt is the width of plain step i's preset text rectangle in a column
// colW wide (pptx.PresetTextRectSize): a pentagon gives up half its point, a
// chevron its point and its notch.
func (l processFlowLook) textRectPt(i int, colW float64) float64 {
	if l.stepGeometry(i) == "homePlate" {
		return colW - l.notchPt/2
	}
	return colW + l.bleedPt(i) - 2*l.notchPt
}

// applyAdjustments sets every plain step's point depth for a row rowPt tall:
// the preset measures it as a share (x100000) of the shape's shorter side.
func (l processFlowLook) applyAdjustments(cells []*jsonschema.GridCellInput, widths []float64, rowPt float64) {
	for i, c := range cells {
		if !l.interlocks(i) || c == nil || c.Shape == nil || i >= len(widths) {
			continue
		}
		short := math.Min(widths[i]+c.BleedLeft, rowPt)
		if short <= 0 {
			continue
		}
		c.Shape.Adjustments = map[string]int64{"adj": int64(math.Round(math.Min(l.notchPt/short, 0.5) * 100000))}
	}
}

// stepText is a plain step's text: the label, under its numeral when the flow
// is numbered. A numbered step reads from its left edge like a card; a bare
// label is centred in its arrow.
func (l processFlowLook) stepText(label string, size float64, ink string, number int, numeralInk string) json.RawMessage {
	type paragraph struct {
		Content    string  `json:"content"`
		Size       float64 `json:"size"`
		Bold       bool    `json:"bold,omitempty"`
		Color      string  `json:"color,omitempty"`
		Align      string  `json:"align,omitempty"`
		SpaceAfter float64 `json:"space_after,omitempty"`
	}
	align, anchor := "ctr", "ctr"
	var paras []paragraph
	if l.numerals {
		// Top-anchored: the numerals of a row share one line whatever the
		// length of the labels under them.
		align, anchor = "l", "t"
		paras = append(paras, paragraph{Content: fmt.Sprintf("%02d", number), Size: l.numeralPt, Bold: true, Color: numeralInk, Align: align, SpaceAfter: processFlowNumeralGapPt})
	}
	paras = append(paras, paragraph{Content: label, Size: size, Bold: true, Color: ink, Align: align})
	data, _ := json.Marshal(struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
		InsetLeft     float64     `json:"inset_left"`
		InsetRight    float64     `json:"inset_right"`
	}{paras, align, anchor, processFlowChevronInsetPt, processFlowChevronInsetPt})
	return data
}

// processFlowNumeralInk is the colour of a step numeral on the tinted step
// fill: the accent where it reads as large type on the tint, else the label's
// own ink.
func processFlowNumeralInk(ctx ExpandContext, tone fillTone, accent, ink string) string {
	if ratio, ok := fillContrast(ctx, tone, fillTone{Color: accent}); ok && ratio >= svggen.WCAGAALarge {
		return accent
	}
	return ink
}
