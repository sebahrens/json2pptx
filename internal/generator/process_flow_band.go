package generator

import (
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Process flow band — a linear flow as interlocking arrows
// =============================================================================
//
// A process_flow whose steps simply follow one another used to be a row of
// equal boxes joined by thin accent arrows: every step its own box and the
// sequence carried by hairlines — the look the process-flow pattern gave up
// (go-slide-creator-cuq95). The diagram now draws that flow the way the
// pattern does (go-slide-creator-av25u): the first step is a pentagon and
// every later one a chevron whose notch tucks round the point before it, so
// the row is one band and needs no connector. Steps take the accent's light
// tint and carry a large numeral over the label; a decision stays a diamond,
// in the solid accent when it is the flow's only one. A band too long for one
// row bends: the next row runs back right to left with its arrows mirrored.
//
// A flow that is a graph — authored connections that branch, skip or carry a
// label, start / end terminators, subprocesses, a vertical direction — is a
// flowchart and keeps boxes and connectors (process_flow_shapes.go).

const (
	// pfBandGap is the slanted gap between one step's point and the next
	// step's notch (EMU, 4pt): the process-flow pattern's.
	pfBandGap int64 = 4 * 12700

	// pfBandRowGap is the gap between the rows of a bent band (EMU, 12pt).
	pfBandRowGap int64 = 12 * 12700

	// The point (and notch) is pfBandNotchFrac of the column width, between
	// pfBandMinNotch and pfBandMaxNotch: a step that holds a sentence is a
	// card with a point, not a dart.
	pfBandNotchFrac        = 0.09
	pfBandMinNotch   int64 = 8 * 12700
	pfBandMaxNotch   int64 = 18 * 12700
	pfBandSideInset  int64 = 8 * 12700
	pfBandNumeralGap       = 200 // hundredths of a point after the numeral

	// pfBandMinDescWidth is the narrowest text column that carries a
	// description (EMU, 1.6"): about twenty characters of 12pt per line.
	pfBandMinDescWidth int64 = 1463040

	// pfBandMaxHeightFactor caps how far a sparse band's rows grow past the
	// height their text needs.
	pfBandMaxHeightFactor = 1.3
)

// pfBandType is the type scale of a band with perRow steps in a row
// (hundredths of a point): the process-flow pattern's numerals, labels one
// step above the body size, descriptions at body size.
type pfBandType struct {
	numeral, label, desc int
}

func pfBandTypeFor(perRow int) pfBandType {
	switch {
	case perRow <= 4:
		return pfBandType{numeral: 2400, label: 1600, desc: 1400}
	case perRow <= 6:
		return pfBandType{numeral: 2000, label: 1400, desc: 1200}
	default:
		return pfBandType{numeral: 1800, label: 1400, desc: 1200}
	}
}

// pfBand is the geometry of a band: how its steps wrap and the type they are
// set in. The step boxes themselves are the layout's pfStepLayouts.
type pfBand struct {
	perRow int
	notch  int64
	typ    pfBandType
	// flip marks the steps on a row that runs right to left.
	flip []bool
	// number is the step's numeral (1-based, decisions are not counted); 0
	// for a decision.
	number []int
}

// pfBandEligible reports whether the flow is a plain sequence: horizontal,
// only steps and decisions, and connections that are either the generated
// defaults or an unlabelled chain from each step to the next.
func pfBandEligible(steps []processFlowStep, connections []processFlowConnection, direction string) bool {
	if direction != "horizontal" || len(steps) < 2 {
		return false
	}
	index := make(map[string]int, len(steps))
	for i, s := range steps {
		if s.stepType != pfStepType && s.stepType != pfDecisionType && s.stepType != "" {
			return false
		}
		if _, dup := index[s.id]; dup {
			return false
		}
		index[s.id] = i
	}
	if pfConnectionsEqual(connections, generateSequentialFlowConnections(steps)) {
		return true
	}
	if len(connections) != len(steps)-1 {
		return false
	}
	for i, c := range connections {
		from, okFrom := index[c.from]
		to, okTo := index[c.to]
		if !okFrom || !okTo || from != i || to != i+1 || c.label != "" || c.style == "dashed" {
			return false
		}
	}
	return true
}

func pfConnectionsEqual(a, b []processFlowConnection) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pfBandNotch is the depth of every arrow's point in a column colW wide.
func pfBandNotch(colW int64) int64 {
	return min(max(int64(math.Round(float64(colW)*pfBandNotchFrac)), pfBandMinNotch), pfBandMaxNotch)
}

// pfBandColumnWidth is the width of one of perRow equal columns.
func pfBandColumnWidth(width int64, perRow int) int64 {
	return (width - int64(perRow-1)*pfBandGap) / int64(perRow)
}

// pfBandTextWidth is the text column every plain step of the band can count
// on: a chevron's text rectangle (the column less its point; its notch lies
// in the gap before it) inside the side insets.
func pfBandTextWidth(colW, notch int64) int64 {
	return colW - notch - 2*pfBandSideInset
}

// pfBandPerRow picks how many steps share a row: all of them when every
// label keeps its widest word whole at the row's label size (and a
// description gets a readable line), otherwise the fewest balanced rows up to
// pfMaxRows. ok is false when even pfMaxRows rows do not hold the labels.
func pfBandPerRow(steps []processFlowStep, bounds types.BoundingBox, font string) (int, bool) {
	n := len(steps)
	for rows := 1; rows <= pfMaxRows && rows <= n; rows++ {
		perRow := (n + rows - 1) / rows
		if rows > 1 && perRow < 2 {
			break
		}
		typ := pfBandTypeFor(perRow)
		colW := pfBandColumnWidth(bounds.Width, perRow)
		textW := pfBandTextWidth(colW, pfBandNotch(colW))
		fits := textW > 0
		for _, s := range steps {
			if !fits {
				break
			}
			if s.stepType == pfDecisionType {
				// A diamond holds its label in the middle half of its width.
				fits = pfWidestWordEMU(s.label, font, pfDecisionMinLabelSize)*2 <= colW
				continue
			}
			word := int64(math.Ceil(float64(pfWidestWordEMU(s.label, font, typ.label)) * pptx.StandInWordFitSlack))
			if word > textW || (s.description != "" && textW < pfBandMinDescWidth) {
				fits = false
			}
		}
		if fits {
			return perRow, true
		}
	}
	return 0, false
}

// pfWidestWordEMU is the width of text's widest word, bold at size.
func pfWidestWordEMU(text, font string, size int) int64 {
	return pfWidestLabelWordEMU(processFlowStep{label: text}, font) * int64(size) / int64(pfLabelFontSize)
}

// pfBandGeometry is step i's preset: a pentagon opens the flow, chevrons
// follow it, a decision is a diamond.
func pfBandGeometry(steps []processFlowStep, i int) pptx.PresetGeometry {
	switch {
	case steps[i].stepType == pfDecisionType:
		return pptx.GeomFlowChartDecision
	case i == 0:
		return pptx.GeomHomePlate
	default:
		return pptx.GeomChevron
	}
}

// pfBandStepText is a plain step's text body: the numeral, the bold label and
// the description under it, left-aligned from the top so the numerals of a
// row share one line.
func pfBandStepText(step processFlowStep, number int, typ pfBandType, tone nativeTone, numeralInk, font string) pptx.TextBody {
	run := func(text string, size int, bold bool, ink string) pptx.Run {
		return pptx.Run{Text: text, Lang: "en-US", FontSize: size, Bold: bold, Dirty: true, Color: pptx.SchemeFill(ink)}
	}
	paras := []pptx.Paragraph{
		{Align: "l", NoBullet: true, SpaceAfter: pfBandNumeralGap, Runs: []pptx.Run{run(fmt.Sprintf("%02d", number), typ.numeral, true, numeralInk)}},
		{Align: "l", NoBullet: true, Runs: []pptx.Run{run(step.label, typ.label, true, tone.ink)}},
	}
	if step.description != "" {
		paras = append(paras, pptx.Paragraph{Align: "l", NoBullet: true, Runs: []pptx.Run{run(step.description, typ.desc, false, tone.ink)}})
	}
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "t",
		Insets:     [4]int64{pfBandSideInset, pptx.ShapeTextInsetEMU, pfBandSideInset, pptx.ShapeTextInsetEMU},
		AutoFit:    "normAutofit",
		Paragraphs: paras,
		ThemeFonts: pptx.ThemeFonts{Major: font, Minor: font},
	}
}

// pfBandStepNeed is the row height at which a plain step's text is stored at
// its declared sizes, measured with the writer's own estimator in the text
// column every arrow of the band has.
func pfBandStepNeed(step processFlowStep, typ pfBandType, textW int64, font string, limit int64) int64 {
	tb := pfBandStepText(step, 1, typ, nativeTone{ink: "dk1"}, "dk1", font)
	return nativeTextNeedAtMarginEMU(tb, textW+2*pfBandSideInset, limit)
}

// computeProcessFlowBand lays a linear flow out as a band in bounds. ok is
// false when the flow is not a plain sequence or its labels do not fit the
// band at any row count; the caller then draws the flowchart.
func computeProcessFlowBand(steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox, direction, font string) (pfLayoutResult, bool) {
	if !pfBandEligible(steps, connections, direction) || bounds.Width <= 0 || bounds.Height <= 0 {
		return pfLayoutResult{}, false
	}
	perRow, ok := pfBandPerRow(steps, bounds, font)
	if !ok {
		return pfLayoutResult{}, false
	}
	n := len(steps)
	rows := (n + perRow - 1) / perRow
	typ := pfBandTypeFor(perRow)
	colW := pfBandColumnWidth(bounds.Width, perRow)
	notch := pfBandNotch(colW)
	textW := pfBandTextWidth(colW, notch)

	// One height for every row: what the fullest step needs, a decision's
	// diamond included, never under the strip height of the flowchart look.
	limit := max(bounds.Height, 1)
	need := pfStepHeight(bounds)
	for _, s := range steps {
		if s.stepType == pfDecisionType {
			continue
		}
		need = max(need, pfBandStepNeed(s, typ, textW, font, 4*limit))
	}
	for _, s := range steps {
		if s.stepType != pfDecisionType || s.description != "" {
			continue
		}
		for h := need; h <= colW && !pfDecisionLabelFits(s.label, font, colW, h); h += max(1, need/20) {
			need = h + max(1, need/20)
		}
	}
	// A band with room to spare grows toward the body: a strip of arrows in
	// the middle of an empty slide reads as a thumbnail of a slide.
	rowH := need
	avail := (bounds.Height - int64(rows-1)*pfBandRowGap) / int64(rows)
	if grown := int64(float64(need) * pfBandMaxHeightFactor); avail > need {
		rowH = min(grown, max(need, avail/2))
	}
	if avail > 0 {
		rowH = min(rowH, avail)
	}
	block := int64(rows)*rowH + int64(rows-1)*pfBandRowGap
	// The optical centre: a little more of the spare height below the band.
	top := bounds.Y + max(0, bounds.Height-block)*45/100

	band := &pfBand{perRow: perRow, notch: notch, typ: typ, flip: make([]bool, n), number: make([]int, n)}
	layouts := make([]pfStepLayout, n)
	number := 0
	for i, s := range steps {
		row, pos := i/perRow, i%perRow
		col := pos
		returning := row%2 == 1
		if returning {
			col = perRow - 1 - pos
		}
		band.flip[i] = returning
		if s.stepType != pfDecisionType {
			number++
			band.number[i] = number
		}
		x := bounds.X + int64(col)*(colW+pfBandGap)
		cx := colW
		// A chevron reaches over the gap on the side the flow comes from (its
		// notch on a row running left to right, its point on a returning
		// row), so the arrows interlock.
		if pfBandGeometry(steps, i) == pptx.GeomChevron && pfBandBleeds(steps, i, perRow, col, returning) {
			x -= notch
			cx += notch
		}
		layouts[i] = pfStepLayout{x: x, y: top + int64(row)*(rowH+pfBandRowGap), cx: cx, cy: rowH}
	}
	return pfLayoutResult{steps: layouts, direction: "horizontal", band: band}, true
}

// pfBandBleeds reports whether chevron i reaches left over the column gap.
// On a row running left to right that side is its notch, which tucks round
// whatever the step before it ends in. On a returning row the shape is
// mirrored and that side is its point, which needs the notch of the step it
// leads to: that step must be a chevron too.
func pfBandBleeds(steps []processFlowStep, i, perRow, col int, returning bool) bool {
	if col == 0 {
		return false
	}
	if !returning {
		return true
	}
	next := i + 1
	return next < len(steps) && next/perRow == i/perRow && steps[next].stepType != pfDecisionType
}

// pfBandTones are the fills of a band: every plain step in the accent's
// content tint, a decision in the solid accent when it is the flow's only one
// and otherwise in the tint under an accent outline.
type pfBandTones struct {
	step       nativeTone
	numeralInk string
	decision   nativeTone
	outline    pptx.Line
}

func pfBandTonesFor(steps []processFlowStep, surface nativeSurface) pfBandTones {
	t := pfBandTones{step: surface.content(patterns.TonalLighterContent), outline: pptx.Line{Width: 0, Fill: pptx.NoFill()}}
	t.numeralInk = surface.accentInk(t.step)
	decisions := 0
	for _, s := range steps {
		if s.stepType == pfDecisionType {
			decisions++
		}
	}
	if decisions == 1 {
		t.decision = surface.emphasis()
		return t
	}
	t.decision = t.step
	t.outline = pptx.Line{Width: int64(patterns.ProcessFlowDecisionLinePt * 12700), Fill: pptx.SchemeFill(surface.accent())}
	return t
}

// pfBandStepShape builds step i of a band.
func pfBandStepShape(steps []processFlowStep, i int, sl pfStepLayout, band *pfBand, tones pfBandTones, shapeID uint32, font string) pptx.ShapeOptions {
	step := steps[i]
	if step.stepType == pfDecisionType {
		opts := pfGenerateStepShape(step, sl, shapeID, len(steps), font)
		opts.Fill = tones.decision.fill()
		opts.Line = tones.outline
		for p := range opts.Text.Paragraphs {
			for r := range opts.Text.Paragraphs[p].Runs {
				opts.Text.Paragraphs[p].Runs[r].Color = tones.decision.inkFill()
			}
		}
		return opts
	}
	geom := pfBandGeometry(steps, i)
	text := pfBandStepText(step, band.number[i], band.typ, tones.step, tones.numeralInk, font)
	short := min(sl.cx, sl.cy)
	adj := int64(50000)
	if short > 0 {
		adj = min(int64(math.Round(float64(band.notch)/float64(short)*100000)), 50000)
	}
	return pptx.ShapeOptions{
		ID:          shapeID,
		Name:        fmt.Sprintf("Step %s", step.label),
		Bounds:      pptx.RectEmu{X: sl.x, Y: sl.y, CX: sl.cx, CY: sl.cy},
		Geometry:    geom,
		Adjustments: []pptx.AdjustValue{{Name: "adj", Value: adj}},
		Fill:        tones.step.fill(),
		Line:        pptx.Line{Width: 0, Fill: pptx.NoFill()},
		FlipH:       band.flip[i],
		Text:        &text,
	}
}

// pfBandTextDeficit reports the text height step i needs and the height its
// arrow gives it, for the readability preflight. A decision is checked by
// its own label fit.
func pfBandTextDeficit(steps []processFlowStep, i int, layout pfLayoutResult, font string) (need, have int64) {
	sl := layout.steps[i]
	textW := pfBandTextWidth(pfBandColumnWidthOf(layout), layout.band.notch)
	return pfBandStepNeed(steps[i], layout.band.typ, textW, font, 8*max(sl.cy, 1)), sl.cy
}

// pfBandColumnWidthOf is the band's column width: the width of its first
// step, which never bleeds.
func pfBandColumnWidthOf(layout pfLayoutResult) int64 {
	if len(layout.steps) == 0 {
		return 0
	}
	return layout.steps[0].cx
}
