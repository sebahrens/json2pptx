package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// journey-maturity-model, default "staircase" style (go-slide-creator-an4ao).
//
// One solid step per stage. The steps stand on one floor and each is one rise
// taller than the stage before it, so the silhouette is the staircase. Their
// fills are a tonal ladder of the accent: the palette's lighter swatches
// deepen stage by stage up to the solid accent on the current stage (the last
// stage when none is marked), and the stages still ahead keep the palest
// swatch. Each step carries a big stage numeral and the bold stage name on its
// tread; the descriptions stand under the floor, one per column, at one size.
// The "We are here" marker is a bold label over a solid accent pointer that
// rests on the current step.
// ---------------------------------------------------------------------------

const (
	// journeyMaturityStepGapPt separates two steps, so two stages of the same
	// tone still read as two steps.
	journeyMaturityStepGapPt = 4.0
	// journeyMaturityStepRowGapPt is the space between the staircase's floor
	// and the description row; the descriptions' own text margin is the rest
	// of that gap.
	journeyMaturityStepRowGapPt = 0.01
	// journeyMaturityStepMinRisePt / MaxRisePt bound the step up from one
	// stage to the next.
	journeyMaturityStepMinRisePt = 10.0
	journeyMaturityStepMaxRisePt = 64.0
	// journeyMaturityStepBaseGrow is how far the first step may grow past the
	// height its numeral and name need when the content area has room left
	// after the tallest rise.
	journeyMaturityStepBaseGrow = 1.5
	// journeyMaturityPointerWPt / HPt are the marker's pointer;
	// journeyMaturityPointerGapPt its distance from the step it points at and
	// from the label above it.
	journeyMaturityPointerWPt   = 18.0
	journeyMaturityPointerHPt   = 10.0
	journeyMaturityPointerGapPt = 3.0
	// journeyMaturityMarkerLabelInsetPt is the marker label's text margin.
	journeyMaturityMarkerLabelInsetPt = 2.0
	// journeyMaturityAccentBarPt is the width of a cell_overrides accent_bar on
	// a step.
	journeyMaturityAccentBarPt = 4.0
)

// The tonal ladder, as the lumMod of PowerPoint's "Lighter N%" palette
// swatches (lumOff is the complement): the stages behind the solid one run
// from at least journeyMaturityLadderLoMod (Lighter 80%) up to the deepest
// swatch in steps of at most journeyMaturityLadderStepMod, and the stages
// ahead of it keep journeyMaturityAheadMod (Lighter 90%). The deepest swatch
// is the first of journeyMaturityLadderHiMods the theme's dark ink reads on,
// so one ink serves every tinted step.
const (
	journeyMaturityLadderLoMod   = 20000
	journeyMaturityLadderStepMod = 15000
	journeyMaturityAheadMod      = 10000
)

var journeyMaturityLadderHiMods = []int{60000, 50000, 40000}

// journeyMaturityMarkerLabel is the marker's text.
const journeyMaturityMarkerLabel = "We are here"

// journeyMaturityPointerGeometry is a triangle that points down without being
// rotated.
const journeyMaturityPointerGeometry = "flowChartMerge"

// journeyMaturityLighterTone is the accent's "Lighter" swatch that keeps
// lumMod of its lightness. A scheme accent keeps its theme link; a hex accent
// is returned as the lightened hex, since the shape-grid resolver honours
// modifiers on scheme colours only.
func journeyMaturityLighterTone(accent string, lumMod int) fillTone {
	if isHexColor(accent) {
		if c, err := svggen.ParseColor(accent); err == nil {
			return fillTone{Color: applyLumModOff(c, lumMod, 100000-lumMod).Hex()}
		}
		return fillTone{Color: "lt2"}
	}
	return fillTone{Color: accent, LumMod: lumMod, LumOff: 100000 - lumMod}
}

// journeyMaturityLadderMod is the lumMod of stage i when the solid stage is
// target and the deepest swatch hiMod: the stage before the target is the
// deepest swatch and every earlier stage one ladder step lighter, the steps
// shrinking so the first stage never falls under the ladder's lightest swatch.
func journeyMaturityLadderMod(i, target, hiMod int) int {
	if i > target {
		return journeyMaturityAheadMod
	}
	step := journeyMaturityLadderStepMod
	if target > 1 {
		step = min(step, (hiMod-journeyMaturityLadderLoMod)/(target-1))
	}
	return hiMod - step*(target-1-i)
}

// journeyMaturityLadder is every stage's fill and the ink set on it. The
// target stage is the solid accent, deepened where white would not read on
// it. In the default uniform mode the other stages are rungs of the tonal
// ladder under ONE dark ink, chosen by measurement over the whole group: the
// ladder's deepest swatch steps down until dk2 reads on every rung, and when
// it reads on none the ladder keeps its depth and takes dk1. cell_accent_mode
// alternate / progressive paints every other stage in its own accent slot, as
// the mode asks.
func journeyMaturityLadder(ctx ExpandContext, accent string, n, target int, mode string) ([]fillTone, []string) {
	tones := make([]fillTone, n)
	inks := make([]string, n)
	tones[target], inks[target] = accentFillAndInk(ctx, fillTone{Color: accent}, svggen.WCAGAANormal)
	if mode != "" && mode != CellAccentUniform {
		for i := range tones {
			if i != target {
				tones[i], inks[i] = accentFillAndInk(ctx, fillTone{Color: ctx.ResolveCellAccent(accent, i, mode)}, svggen.WCAGAANormal)
			}
		}
		return tones, inks
	}
	rungs := func(hiMod int) {
		for i := range tones {
			if i != target {
				tones[i] = journeyMaturityLighterTone(accent, journeyMaturityLadderMod(i, target, hiMod))
			}
		}
	}
	reads := func(ink string) bool {
		c, ok := resolveThemeColor(ctx, ink)
		if !ok {
			return false
		}
		for i, tone := range tones {
			if i == target {
				continue
			}
			if fill, fok := effectiveFillColor(ctx, tone); !fok || c.ContrastWith(fill) < svggen.WCAGAANormal {
				return false
			}
		}
		return true
	}
	groupInk := ""
	for _, hiMod := range journeyMaturityLadderHiMods {
		rungs(hiMod)
		if reads("dk2") {
			groupInk = "dk2"
			break
		}
	}
	if groupInk == "" {
		rungs(journeyMaturityLadderHiMods[0])
		if reads("dk1") {
			groupInk = "dk1"
		}
	}
	for i := range inks {
		switch {
		case i == target:
		case groupInk != "":
			inks[i] = groupInk
		default:
			// No theme to measure against, or no single ink reads everywhere.
			inks[i] = readableTextOn(ctx, tones[i], "dk1")
		}
	}
	return tones, inks
}

// journeyMaturityCurrent is the index of the stage marked current, -1 when
// none is. Only the first counts: MULTIPLE_CURRENT_STAGES reports the rest.
func journeyMaturityCurrent(stages []JourneyMaturityStage) int {
	for i, s := range stages {
		if s.Current {
			return i
		}
	}
	return -1
}

// journeyMaturityStepRisePt is the step up from one stage to the next and the
// room kept above the top step for the marker: the tallest rise at which the
// staircase (the first step's needPt, n-1 rises, the marker's room) fits
// availPt, never under the minimum. The marker stands in the air above the
// current step, which the later, taller steps leave free, so it costs height
// only where that air is lower than the marker: on the last stages.
func journeyMaturityStepRisePt(availPt, needPt, markerPt float64, n, current int) (rise, reserve float64) {
	reserveAt := func(rise float64) float64 {
		if current < 0 {
			return 0
		}
		return math.Max(0, markerPt-float64(n-1-current)*rise)
	}
	for rise = journeyMaturityStepMaxRisePt; rise > journeyMaturityStepMinRisePt; rise-- {
		if needPt+float64(n-1)*rise+reserveAt(rise) <= availPt {
			break
		}
	}
	return rise, reserveAt(rise)
}

// journeyMaturitySteps expands the default staircase.
func journeyMaturitySteps(ctx ExpandContext, vals *JourneyMaturityValues, ovr *JourneyMaturityOverrides, cellOverrides map[int]any) *jsonschema.ShapeGridInput {
	n := len(vals.Stages)
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	nameSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	numeralSize := math.Max(scaleDisplayPt, nameSize)
	descSize := ResolveSize(ovr.BodySize, scaleBodyPt)
	current := journeyMaturityCurrent(vals.Stages)
	target := current
	if target < 0 {
		// No stage is marked: the ladder ends on full maturity.
		target = n - 1
	}

	gap := ctx.Gap(journeyMaturityStepGapPt)
	contentW, contentH := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, gap)
	fonts := ctx.themeFonts()

	// One text payload per role and stage, measured over the whole group so
	// every stage's numeral, name and description is set at one size in a box
	// of one height.
	stepTexts := make([]json.RawMessage, n)
	tones, inks := journeyMaturityLadder(ctx, accent, n, target, ovr.CellAccentMode)
	descTexts := make([]json.RawMessage, n)
	accentBars := make([]bool, n)
	stepNeed, descNeed := 0.0, 0.0
	for i, stage := range vals.Stages {
		number := stage.Number
		if number <= 0 {
			number = i + 1
		}
		stepTexts[i] = buildJourneyMaturityStepText(number, pptx.ConvertMarkdownEmphasis(stage.Label), numeralSize, nameSize, inks[i])
		if co, ok := cellOverrides[i].(*JourneyMaturityCellOverride); ok && co != nil {
			stepTexts[i] = applyCellTextOverrideToText(stepTexts[i], co)
			accentBars[i] = co.AccentBar
		}
		stepNeed = math.Max(stepNeed, writtenFitHeightPt(fonts, stepTexts[i], colW, 0))
		if desc := strings.TrimSpace(stage.Description); desc != "" {
			descTexts[i] = buildJourneyMaturityStepDescription(pptx.ConvertMarkdownEmphasis(desc), descSize)
			descNeed = math.Max(descNeed, writtenFitHeightPt(fonts, descTexts[i], colW, 0))
		}
	}
	descNeed = math.Ceil(descNeed)

	markerInk := inkOnLight(ctx, accent, 4.5)
	markerText := withTextInsets(buildJourneyMaturityMarkerText(journeyMaturityMarkerLabel, nameSize, markerInk), journeyMaturityMarkerLabelInsetPt)
	labelH := math.Ceil(writtenFitHeightPt(fonts, markerText, colW, 0))
	markerH := labelH + journeyMaturityPointerHPt + 2*journeyMaturityPointerGapPt

	avail := contentH - descNeed - journeyMaturityStepRowGapPt
	rise, reserve := journeyMaturityStepRisePt(avail, stepNeed, markerH, n, current)
	// Room left under the tallest rise goes to the floor: every step grows by
	// the same amount, the first by at most journeyMaturityStepBaseGrow.
	base := stepNeed
	if spare := avail - (stepNeed + float64(n-1)*rise + reserve); spare > 0 {
		base += math.Min(spare, stepNeed*(journeyMaturityStepBaseGrow-1))
	}
	base = math.Floor(base)
	stairH := base + float64(n-1)*rise + reserve

	steps := make([]*jsonschema.GridCellInput, n)
	descs := make([]*jsonschema.GridCellInput, n)
	for i := range vals.Stages {
		top := (reserve + float64(n-1-i)*rise) / stairH
		layers := []jsonschema.LayerInput{{
			Name:  fmt.Sprintf("step-%d", i+1),
			Frame: jsonschema.LayerFrameInput{X: 0, Y: top, W: 1, H: 1 - top},
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: tones[i].fillJSON(), Line: noLine, Text: stepTexts[i]},
		}}
		if accentBars[i] {
			layers = append(layers, jsonschema.LayerInput{
				Name:  fmt.Sprintf("accent-bar-%d", i+1),
				Frame: jsonschema.LayerFrameInput{X: 0, Y: top, W: journeyMaturityAccentBarPt / colW, H: 1 - top},
				Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: marshalRaw(accent), Line: noLine},
			})
		}
		if i == current {
			pointerW := journeyMaturityPointerWPt / colW
			pointerY := top - (journeyMaturityPointerGapPt+journeyMaturityPointerHPt)/stairH
			labelY := pointerY - (journeyMaturityPointerGapPt+labelH)/stairH
			layers = append(layers,
				jsonschema.LayerInput{
					Name:  "marker-label",
					Frame: jsonschema.LayerFrameInput{X: 0, Y: math.Max(0, labelY), W: 1, H: labelH / stairH},
					Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: noLine, Line: noLine, Text: markerText},
				},
				jsonschema.LayerInput{
					Name:  "marker-pointer",
					Frame: jsonschema.LayerFrameInput{X: (1 - pointerW) / 2, Y: math.Max(0, pointerY), W: pointerW, H: journeyMaturityPointerHPt / stairH},
					// The pointer is the current step's own tone, so the two read
					// as one mark.
					Shape: &jsonschema.ShapeSpecInput{Geometry: journeyMaturityPointerGeometry, Fill: tones[i].fillJSON(), Line: noLine},
				})
		}
		steps[i] = &jsonschema.GridCellInput{Layers: layers}

		descs[i] = &jsonschema.GridCellInput{}
		if len(descTexts[i]) > 0 {
			descs[i].Shape = &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: noLine, Line: noLine, Text: descTexts[i]}
		}
	}

	rows := []jsonschema.GridRowInput{{MinHeight: stairH, MaxHeight: stairH, Cells: steps}}
	if descNeed > 0 {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: descNeed, MaxHeight: descNeed, Cells: descs})
	}
	colsJSON, _ := json.Marshal(n)
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		Gap:           gap,
		RowGap:        journeyMaturityStepRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}
}

// buildJourneyMaturityStepText is a step's tread: the stage numeral over the
// bold stage name, both at the step's left text edge.
func buildJourneyMaturityStepText(number int, label string, numeralSize, nameSize float64, ink string) json.RawMessage {
	data, _ := json.Marshal(journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: fmt.Sprintf("%02d", number), Size: numeralSize, Color: ink, Align: "l"},
			{Content: label, Size: nameSize, Bold: true, Color: ink, Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	})
	return data
}

// buildJourneyMaturityStepDescription is the unboxed description under a step,
// on the step's own left text edge.
func buildJourneyMaturityStepDescription(description string, size float64) json.RawMessage {
	data, _ := json.Marshal(journeyMaturityTextObj{
		Paragraphs:    []journeyMaturityParagraph{{Content: description, Size: size, Color: "dk1", Align: "l"}},
		Align:         "l",
		VerticalAlign: "t",
	})
	// An explicit left margin: the first column's unboxed text would otherwise
	// be pulled out to the title's text edge, off its own step's.
	return withTextInsetSides(data, defaultShapeInsetLRPt, "inset_left")
}
