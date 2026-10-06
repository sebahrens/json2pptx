package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// process-grid-2row "lanes" style (default, go-slide-creator-06bnr).
//
// Each track is one process lane: a pentagon carrying the row label points
// into a chain of chevrons, one per phase, whose tails tuck under the point
// before them. The shape says "left to right", so there are no boxes, rules
// or connectors. The colour is one accent in PowerPoint's own "lighter"
// swatches: the first lane's label is the solid accent (the slide's single
// solid block) over Lighter 60% phases, the second lane's label is Lighter
// 40% over Lighter 80% phases, so the two lanes are told apart by depth
// rather than by a second hue. Lanes are content-sized, never stretched over
// the slide. column_headers are bold text standing on the lanes, outcomes
// bold accent-ink text under them: no underline, pill or tile.
// ---------------------------------------------------------------------------

const (
	processGrid2RowStyleLanes  = "lanes"
	processGrid2RowStyleTinted = "tinted"
	processGrid2RowStyleSolid  = "solid"

	// processGrid2RowLaneGapPt is the column gap, and so the width of the
	// slanted gap between one point and the next notch. A hairline between
	// interlocking shapes: not scaled with the template gutter.
	processGrid2RowLaneGapPt = 4.0
	// processGrid2RowLaneRowGapPt separates the two lanes (and the header and
	// outcome lines from them).
	processGrid2RowLaneRowGapPt = 8.0
	// processGrid2RowLaneInsetPt is the label margin inside a shape's own
	// text rectangle, which already excludes the point and the notch.
	processGrid2RowLaneInsetPt = 4.0
	// processGrid2RowLaneLineInsetPt is the margin a header or outcome line
	// keeps towards the lanes.
	processGrid2RowLaneLineInsetPt = 2.0
	// A lane is contentStretchMax x the written fit of its tallest label,
	// within these bounds: a band with room around its text, never a box
	// stretched over the slide. The notch is fitted on the nominal height.
	processGrid2RowLaneNominalHPt = 72.0
	processGrid2RowLaneMinHPt     = 56.0
	processGrid2RowLaneMaxHPt     = 96.0
	// The deepest point / notch as a share of the lane height, and the
	// shallowest the fit goes to before a label wraps or shrinks.
	processGrid2RowLaneNotchFrac  = 0.30
	processGrid2RowLaneMinNotchPt = 6.0
	// processGrid2RowLaneLabelPointPt is the width the label column keeps
	// free for the half of the pentagon's point its text rectangle gives up.
	processGrid2RowLaneLabelPointPt = 11.0
	// processGrid2RowLaneLabelColMaxPct is how wide the label column may
	// grow to keep a label word whole. Wider than the box styles' 22%: in a
	// half-width segment a one-word label needs more of the lane.
	processGrid2RowLaneLabelColMaxPct = 28.0
	// processGrid2RowLaneFitFrac is the share of a text rectangle a one-line
	// label has to fit inside: the renderer's face may run wider than the
	// measured one.
	processGrid2RowLaneFitFrac = 0.90

	// The accent ladder, as "lighter" percentages (PowerPoint's swatches).
	processGrid2RowLane1PhaseLighter = 70
	processGrid2RowLane2LabelLighter = 40
	processGrid2RowLane2PhaseLighter = 85
)

// processGrid2RowStyle is the effective overrides.style.
func processGrid2RowStyle(ovr *ProcessGrid2RowOverrides) string {
	if ovr == nil || ovr.Style == "" {
		return processGrid2RowStyleLanes
	}
	return ovr.Style
}

// processGrid2RowLighter is color at PowerPoint's "Lighter pct%" swatch:
// L' = (1-pct)·L + pct, hue and theme link kept. A hex colour, whose
// modifiers the fill resolver ignores, is mixed toward white instead.
func processGrid2RowLighter(color string, pct int) fillTone {
	if isHexColor(color) {
		c := svggen.MustParseColor(color)
		return fillTone{Color: applyLinearMix(c, float64(100-pct)/100, svggen.Color{R: 255, G: 255, B: 255, A: 1}).Hex()}
	}
	return fillTone{Color: color, LumMod: (100 - pct) * 1000, LumOff: pct * 1000}
}

// processGrid2RowLaneFit is the measured geometry of the lanes style.
type processGrid2RowLaneFit struct {
	labelPct           float64    // row-label column, percent of the grid
	labelPt, phasePt   float64    // type sizes
	headerPt           float64    // column header size
	outcomePt          float64    // outcome size
	labelWPt, phaseWPt float64    // column widths
	notchPt            float64    // depth of every point and notch
	rowPt              [2]float64 // lane heights
	needs              [2]float64 // written fit of each lane's tallest cell
	headerRowPt        float64    // 0 when absent
	outcomeRowPt       float64    // 0 when absent
	availPt            float64    // height the two lanes share
	fits               bool       // the lanes fit availPt at their needs
	unfit              []string   // row-label words no label column holds
}

// labelRectPt / phaseRectPt are the widths of the preset text rectangles
// (pptx.PresetTextRect): a pentagon gives up half its point; a chevron, which
// reaches notchPt left of its column, its point and its notch.
func (f processGrid2RowLaneFit) labelRectPt() float64 { return f.labelWPt - f.notchPt/2 }
func (f processGrid2RowLaneFit) phaseRectPt() float64 { return f.phaseWPt - f.notchPt }

// adj is the preset adjustment for a shape shapeWPt wide in a lane rowPt
// tall: the point depth as a share (x100000) of the shorter side.
func (f processGrid2RowLaneFit) adj(shapeWPt, rowPt float64) int64 {
	short := math.Min(shapeWPt, rowPt)
	if short <= 0 {
		return 0
	}
	return int64(math.Round(f.notchPt / short * 100000))
}

// processGrid2RowLaneText is the text payload of a lanes-style shape.
type processGrid2RowLaneText struct {
	content       string
	size          float64
	color         string
	verticalAlign string  // "ctr" (default), "t" or "b"
	insetRight    float64 // extra right inset (header / outcome lines centre over the chevron's text rectangle)
	insetTop      float64 // -1 keeps the default margin
	insetBottom   float64 // -1 keeps the default margin
}

func (t processGrid2RowLaneText) json() json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	obj := map[string]any{
		"paragraphs":     []paragraph{{Content: pptx.ConvertMarkdownEmphasis(t.content), Size: t.size, Bold: true, Color: t.color, Align: "ctr"}},
		"align":          "ctr",
		"vertical_align": "ctr",
		"inset_left":     processGrid2RowLaneInsetPt,
		"inset_right":    processGrid2RowLaneInsetPt + t.insetRight,
	}
	if t.verticalAlign != "" {
		obj["vertical_align"] = t.verticalAlign
	}
	if t.insetTop >= 0 {
		obj["inset_top"] = t.insetTop
	}
	if t.insetBottom >= 0 {
		obj["inset_bottom"] = t.insetBottom
	}
	data, _ := json.Marshal(obj)
	return data
}

// laneText is a centred lane label with the default vertical margin.
func processGrid2RowLaneLabel(content string, size float64, color string) json.RawMessage {
	return processGrid2RowLaneText{content: content, size: size, color: color, insetTop: -1, insetBottom: -1}.json()
}

// fitProcessGrid2RowLanes measures the lanes style at the width the grid
// gives its cells. Order of giving way: the label column widens before the
// row label shrinks; phase labels are set at 14pt when every one stays on one
// line and at the 12pt body size otherwise; the point gets blunter before a
// label word breaks; and the default 14pt row label (with the column headers)
// steps to 12pt only when the lanes would not otherwise fit the area.
func fitProcessGrid2RowLanes(ctx ExpandContext, vals *ProcessGrid2RowValues, ovr *ProcessGrid2RowOverrides) processGrid2RowLaneFit {
	n := len(vals.Row1Phases)
	// The type steps tried in order: label / phase sizes. Authored sizes are
	// kept; the defaults give way phase first, then label.
	type step struct{ labelPt, phasePt float64 }
	labelPt := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	steps := []step{{labelPt, ResolveSize(ovr.BodySize, scaleSubheadPt)}}
	if ovr.BodySize == 0 {
		steps = append(steps, step{labelPt, scaleBodyPt})
	}
	if ovr.HeaderSize == 0 {
		steps = append(steps, step{shapegrid.MinTextSizePt, ResolveSize(ovr.BodySize, scaleBodyPt)})
	}
	var fit processGrid2RowLaneFit
	if n == 0 || n != len(vals.Row2Phases) {
		fit.labelPct, fit.labelPt = processGrid2RowLabelColPct, labelPt
		return fit
	}
	areaW, areaH := sizingAreaPt(ctx)
	usable := areaW - float64(n)*processGrid2RowLaneGapPt
	font := ctx.Theme.BodyFont
	fonts := ctx.themeFonts()
	rowGap := ctx.Gap(processGrid2RowLaneRowGapPt)

	for i, st := range steps {
		fit = processGrid2RowLaneFit{headerPt: st.labelPt, phasePt: st.phasePt}
		fit.labelPct, fit.labelPt, fit.unfit = fitProcessGrid2RowLaneLabelCol(vals, font, usable, st.labelPt)
		fit.labelWPt = usable * fit.labelPct / 100
		fit.phaseWPt = usable * (100 - fit.labelPct) / 100 / float64(n)
		var lines int
		fit.notchPt, lines = fitProcessGrid2RowLaneNotch(vals, font, fit.phaseWPt, processGrid2RowLaneNominalHPt, fit.phasePt)
		// The larger phase size is for labels of one or two lines: longer
		// copy is set at the body size.
		last := i == len(steps)-1
		if !last && ovr.BodySize == 0 && fit.phasePt > scaleBodyPt && lines > 2 {
			continue
		}

		fixed := rowGap // between the two lanes
		// A header or outcome line is as tall as its text: the written fit,
		// and never less than the lines it takes in a face running wider
		// than the measured one.
		lineW := (fit.phaseWPt - fit.notchPt - 2*processGrid2RowLaneInsetPt) * processGrid2RowLaneFitFrac
		linePt := func(items []string, size float64, text func(string) json.RawMessage) float64 {
			h := 0.0
			for _, item := range items {
				lines := measuredLines(item, font, true, size, lineW)
				h = math.Max(h, float64(lines)*size*contentLineHeight+processGrid2RowLaneLineInsetPt)
				h = math.Max(h, writtenFitHeightPt(fonts, text(item), fit.phaseWPt, 0))
			}
			return math.Ceil(h)
		}
		if len(vals.ColumnHeaders) == n {
			fit.headerRowPt = linePt(vals.ColumnHeaders, fit.headerPt, func(s string) json.RawMessage {
				return processGrid2RowLaneHeaderText(fit, s, "dk1")
			})
			fixed += fit.headerRowPt + rowGap
		}
		if len(vals.Outcomes) == n {
			// Outcomes are set at 14pt, where bold accent ink is large text,
			// when every one stays on one line there.
			fit.outcomePt = math.Max(fit.phasePt, scaleSubheadPt)
			for _, o := range vals.Outcomes {
				if measuredLines(o, font, true, fit.outcomePt, lineW) > 1 {
					fit.outcomePt = fit.phasePt
				}
			}
			fit.outcomeRowPt = linePt(vals.Outcomes, fit.outcomePt, func(s string) json.RawMessage {
				return processGrid2RowLaneOutcomeText(fit, s, "dk1")
			})
			fixed += fit.outcomeRowPt + rowGap
		}
		fit.availPt = areaH - fixed

		for r, track := range [2]struct {
			label  string
			phases []string
		}{{vals.Row1Label, vals.Row1Phases}, {vals.Row2Label, vals.Row2Phases}} {
			need := writtenFitHeightPt(fonts, processGrid2RowLaneLabel(track.label, fit.labelPt, "lt1"), fit.labelRectPt(), 0)
			for _, phase := range track.phases {
				need = math.Max(need, writtenFitHeightPt(fonts, processGrid2RowLaneLabel(phase, fit.phasePt, "dk1"), fit.phaseRectPt(), 0))
			}
			fit.needs[r] = math.Ceil(need)
		}
		fit.fits = fit.needs[0]+fit.needs[1] <= fit.availPt+1
		// Equal lanes, contentStretchMax x the taller need where the area
		// has the room; lanes that cannot both take the taller need keep
		// their own.
		tallest := math.Max(fit.needs[0], fit.needs[1])
		equal := clampPt(math.Round(tallest*contentStretchMax), processGrid2RowLaneMinHPt, processGrid2RowLaneMaxHPt)
		equal = math.Floor(math.Min(equal, fit.availPt/2))
		if equal >= tallest {
			fit.rowPt = [2]float64{equal, equal}
		} else {
			fit.rowPt = fit.needs
		}
		if fit.fits {
			break
		}
	}
	return fit
}

// fitProcessGrid2RowLaneLabelCol sizes the row-label column so every label
// word stays whole inside the pentagon's text rectangle: at each size from
// labelPt down to the readable floor the column widens (12% up to
// processGrid2RowLaneLabelColMaxPct) before the label gives up a point.
// Words that fit no column at the floor are returned, with the widest column.
func fitProcessGrid2RowLaneLabelCol(vals *ProcessGrid2RowValues, font string, usable, labelPt float64) (colPct, sizePt float64, unfit []string) {
	var words []string
	for _, label := range []string{vals.Row1Label, vals.Row2Label} {
		words = append(words, strings.Fields(label)...)
	}
	textW := func(pct float64) float64 {
		w := (usable*pct/100 - processGrid2RowLaneLabelPointPt - 2*processGrid2RowLaneInsetPt) * processGrid2RowLaneFitFrac
		return math.Max(textfit.AtomicTokenWidthPt(font, w), 1)
	}
	floor := math.Min(labelPt, shapegrid.MinTextSizePt)
	for size := labelPt; size >= floor; size-- {
		for pct := processGrid2RowLabelColPct; pct <= processGrid2RowLaneLabelColMaxPct; pct++ {
			whole := true
			for _, word := range words {
				if measuredLines(word, font, true, size, textW(pct)) > 1 {
					whole = false
					break
				}
			}
			if whole {
				return pct, size, nil
			}
		}
	}
	for _, word := range words {
		if measuredLines(word, font, true, floor, textW(processGrid2RowLaneLabelColMaxPct)) > 1 {
			unfit = append(unfit, word)
		}
	}
	// The widest column breaks the word in the fewest places.
	return processGrid2RowLaneLabelColMaxPct, floor, unfit
}

// fitProcessGrid2RowLaneNotch picks the point depth for phase labels set at
// sizePt and reports the most lines a label then takes: the deepest point at
// which every label stays on one line, else the deepest at which no word
// breaks, else the shallowest.
func fitProcessGrid2RowLaneNotch(vals *ProcessGrid2RowValues, font string, phaseW, basePt, sizePt float64) (notchPt float64, lines int) {
	deepest := math.Max(math.Round(basePt*processGrid2RowLaneNotchFrac), processGrid2RowLaneMinNotchPt)
	phases := append(append([]string{}, vals.Row1Phases...), vals.Row2Phases...)
	textW := func(d float64) float64 {
		w := (phaseW - d - 2*processGrid2RowLaneInsetPt) * processGrid2RowLaneFitFrac
		return math.Max(textfit.AtomicTokenWidthPt(font, w), 1)
	}
	maxLines := func(d float64) int {
		most := 0
		for _, p := range phases {
			most = max(most, measuredLines(p, font, true, sizePt, textW(d)))
		}
		return most
	}
	wordsWhole := func(d float64) bool {
		for _, p := range phases {
			for _, word := range strings.Fields(p) {
				if measuredLines(word, font, true, sizePt, textW(d)) > 1 {
					return false
				}
			}
		}
		return true
	}
	for d := deepest; d >= processGrid2RowLaneMinNotchPt; d-- {
		if maxLines(d) <= 1 {
			return d, 1
		}
	}
	for d := deepest; d >= processGrid2RowLaneMinNotchPt; d-- {
		if wordsWhole(d) {
			return d, maxLines(d)
		}
	}
	// A word no depth holds: more lines than any label is meant to take.
	return processGrid2RowLaneMinNotchPt, math.MaxInt
}

// processGrid2RowLaneHeaderText is a column header: bold text standing on
// the lanes, centred over its chevron's text rectangle.
func processGrid2RowLaneHeaderText(f processGrid2RowLaneFit, header, ink string) json.RawMessage {
	return processGrid2RowLaneText{content: header, size: f.headerPt, color: ink, verticalAlign: "b",
		insetRight: f.notchPt, insetTop: 0, insetBottom: processGrid2RowLaneLineInsetPt}.json()
}

// processGrid2RowLaneOutcomeText is a column outcome: bold accent-ink text
// hanging under the lanes, centred like the header.
func processGrid2RowLaneOutcomeText(f processGrid2RowLaneFit, outcome, ink string) json.RawMessage {
	return processGrid2RowLaneText{content: outcome, size: f.outcomePt, color: ink, verticalAlign: "t",
		insetRight: f.notchPt, insetTop: processGrid2RowLaneLineInsetPt, insetBottom: 0}.json()
}

// expandLanes builds the default lanes style.
func (p *processGrid2Row) expandLanes(ctx ExpandContext, vals *ProcessGrid2RowValues, ovr *ProcessGrid2RowOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	n := len(vals.Row1Phases)
	if n == 0 || n != len(vals.Row2Phases) {
		return nil, fmt.Errorf("process-grid-2row: row1_phases (%d) and row2_phases (%d) must be non-empty and equal length", n, len(vals.Row2Phases))
	}
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	fit := fitProcessGrid2RowLanes(ctx, vals, ovr)

	row1Color := vals.Row1Color
	if row1Color == "" {
		row1Color = ctx.DefaultAccent()
	}
	labelBar := TextContrastThreshold(fit.labelPt, true)
	phaseBar := TextContrastThreshold(fit.phasePt, true)

	// One accent in four depths. An authored row2_color is a second colour
	// family: its lane takes the first lane's depths in that colour.
	label1, label1Ink := accentFillAndInk(ctx, fillTone{Color: row1Color}, labelBar)
	phase1 := processGrid2RowLighter(row1Color, processGrid2RowLane1PhaseLighter)
	label2 := processGrid2RowLighter(row1Color, processGrid2RowLane2LabelLighter)
	label2Ink := readableInkOn(ctx, label2, "dk1", labelBar)
	phase2 := processGrid2RowLighter(row1Color, processGrid2RowLane2PhaseLighter)
	if vals.Row2Color != "" {
		label2, label2Ink = accentFillAndInk(ctx, fillTone{Color: vals.Row2Color}, labelBar)
		phase2 = processGrid2RowLighter(vals.Row2Color, processGrid2RowLane1PhaseLighter)
	}

	cols := make([]float64, 1+n)
	cols[0] = fit.labelPct
	for i := 1; i <= n; i++ {
		cols[i] = (100 - fit.labelPct) / float64(n)
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	lane := func(r int, label string, phases []string, labelTone fillTone, labelInk string, phaseTone fillTone) jsonschema.GridRowInput {
		rowPt := fit.rowPt[r]
		phaseInk := readableInkOn(ctx, phaseTone, "dk1", phaseBar)
		cells := make([]*jsonschema.GridCellInput, 1+n)
		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "homePlate",
				Fill:        labelTone.fillJSON(),
				Line:        noLine,
				Text:        processGrid2RowLaneLabel(label, fit.labelPt, labelInk),
				Adjustments: map[string]int64{"adj": fit.adj(fit.labelWPt, rowPt)},
			},
		}
		applyProcessGrid2RowOverride(cells[0], cellOverrides, cellIdx, baseAccent)
		cellIdx++
		for i, phase := range phases {
			cells[1+i] = &jsonschema.GridCellInput{
				// The tail tucks under the point before it.
				BleedLeft: fit.notchPt,
				Shape: &jsonschema.ShapeSpecInput{
					Geometry:    "chevron",
					Fill:        phaseTone.fillJSON(),
					Line:        noLine,
					Text:        processGrid2RowLaneLabel(phase, fit.phasePt, phaseInk),
					Adjustments: map[string]int64{"adj": fit.adj(fit.phaseWPt+fit.notchPt, rowPt)},
				},
			}
			applyProcessGrid2RowOverride(cells[1+i], cellOverrides, cellIdx, baseAccent)
			cellIdx++
		}
		return jsonschema.GridRowInput{MinHeight: rowPt, MaxHeight: rowPt, Cells: cells}
	}
	rows := []jsonschema.GridRowInput{
		lane(0, vals.Row1Label, vals.Row1Phases, label1, label1Ink, phase1),
		lane(1, vals.Row2Label, vals.Row2Phases, label2, label2Ink, phase2),
	}
	if !fit.fits {
		// Past the area the lanes share what there is in proportion to their
		// needs; PostExpandWarnings reports it.
		for r := range rows {
			rows[r].MaxHeight = 0
		}
		floorFlexRowsAtNeeds(rows, fit.needs[:], fit.availPt)
	}

	textRow := func(items []string, heightPt float64, text func(string) json.RawMessage) jsonschema.GridRowInput {
		cells := []*jsonschema.GridCellInput{{}}
		for _, item := range items {
			cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     text(item),
			}}
			applyProcessGrid2RowOverride(cell, cellOverrides, cellIdx, baseAccent)
			cellIdx++
			cells = append(cells, cell)
		}
		return jsonschema.GridRowInput{MinHeight: heightPt, MaxHeight: heightPt, Cells: cells}
	}
	if len(vals.ColumnHeaders) == n {
		ink := inkOnLight(ctx, "dk2", 4.5)
		header := textRow(vals.ColumnHeaders, fit.headerRowPt, func(s string) json.RawMessage {
			return processGrid2RowLaneHeaderText(fit, s, ink)
		})
		rows = append([]jsonschema.GridRowInput{header}, rows...)
	}
	if len(vals.Outcomes) == n {
		ink := accentInkOnLight(ctx, row1Color, TextContrastThreshold(fit.outcomePt, true))
		if isHexColor(row1Color) {
			ink = inkOnLight(ctx, "dk2", 4.5)
		}
		rows = append(rows, textRow(vals.Outcomes, fit.outcomeRowPt, func(s string) json.RawMessage {
			return processGrid2RowLaneOutcomeText(fit, s, ink)
		}))
	}

	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     processGrid2RowLaneGapPt,
		RowGap:  ctx.Gap(processGrid2RowLaneRowGapPt),
		Rows:    rows,
	}, nil
}

// lanesWarnings are the lanes style's advisories: row-label words no label
// column holds, and lanes that do not fit the content area.
func (p *processGrid2Row) lanesWarnings(ctx ExpandContext, vals *ProcessGrid2RowValues, ovr *ProcessGrid2RowOverrides) []string {
	fit := fitProcessGrid2RowLanes(ctx, vals, ovr)
	var warnings []string
	if len(fit.unfit) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%s: process-grid-2row row label word(s) %s cannot fit one line of the label column (even widened to %.0f%%) at %.0fpt — the renderer breaks them mid-word; shorten or abbreviate row1_label / row2_label",
			ErrCodeTextExceedsShape, listFirstN(fit.unfit, 3), processGrid2RowLaneLabelColMaxPct, fit.labelPt))
	}
	if ctx.LayoutBounds.Width > 0 && ctx.LayoutBounds.Height > 0 && len(vals.Row1Phases) > 0 && !fit.fits {
		warnings = append(warnings, fmt.Sprintf(
			"%s: process-grid-2row tracks need %.0fpt at readable sizes but the content area leaves about %.0fpt for them — shorten the row labels or phase labels, drop column_headers or outcomes, or use fewer phases",
			ErrCodeBodyTooLong, fit.needs[0]+fit.needs[1], math.Max(fit.availPt, 0)))
	}
	return warnings
}
