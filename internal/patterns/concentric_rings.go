package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// concentric-rings pattern — the onion / layers-of-influence model
// ---------------------------------------------------------------------------
//
// 3-5 nested circles (core → adjacent → ecosystem) drawn as `ellipse` layers
// of ONE shape-grid cell, outermost first so every inner ring sits on top.
// The fills are a neutral tint ladder, lightest outside, with one solid
// accent ring (the core unless `highlight` names another).
//
// The labels never go inside the bands. A band of five rings on the shortest
// shipped content area (abstract, 294pt) is 29pt wide and the chord across
// an inner ring's crest is shorter than a 20-character label at 12pt, so
// in-band text would fit on some rings and not on others. Every label
// therefore sits on a ladder to the right of the rings, one row per ring,
// joined to its ring by a hairline leader that ends in a dot inside the band.
//
// For the ladder rows to line up with the rings, the rings share their base
// instead of their centre: each ring's crest (the part of its band above the
// next ring in) is one ladder row tall and at that row's height, so every
// leader is a straight horizontal hairline and the rows fill the height of
// the drawing. A small step is kept between the rings at the base
// (crBaseStepFrac of the crest) so the bands close all the way round; it
// gives way, down to circles that touch at the base, when the rows need the
// height for their text.

func init() {
	Default().Register(&concentricRings{})
}

type concentricRings struct{}

func (p *concentricRings) Name() string { return crName }
func (p *concentricRings) Description() string {
	return "Onion / layers-of-influence model: 3-5 nested rings sharing a base (core innermost), neutral tint ladder with one solid accent ring, every ring labelled on a side ladder (bold label + optional description) joined by a hairline leader"
}
func (p *concentricRings) UseWhen() string {
	return "3-5 layers that contain one another — core → adjacent → ecosystem, team → function → enterprise, must-have → should-have → could-have — each with a short label and an optional one-line description; prefer pyramid when the levels rank by size or priority rather than contain each other, radial-hub for one centre with separate satellites, and cycle-ring when the items are a repeating sequence"
}
func (p *concentricRings) NotWhen() string {
	return "The levels are a ranked hierarchy that narrows (use pyramid), one centre with independent satellites (use radial-hub), a repeating sequence or loop (use cycle-ring or cycle-nodes), equal-width technology tiers (use arch-stack), or more than 5 layers (merge layers or use labeled-rows)"
}
func (p *concentricRings) Version() int      { return 1 }
func (p *concentricRings) CellsHint() string { return "3-5 rings + 3-5 ladder rows" }
func (p *concentricRings) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"card-grid", "labeled-rows", "exec-summary"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (p *concentricRings) ExemplarValues() any {
	return &ConcentricRingsValues{
		Layers: []ConcentricRingsLayer{
			{Label: "Core business", Description: "Retail banking, where we hold the right to win"},
			{Label: "Adjacent markets", Description: "Wealth and SME lending, built on the core"},
			{Label: "Partner ecosystem", Description: "Fintech and insurer alliances that extend reach"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ConcentricRingsLayer is one ring: a short label and an optional one-line
// description, both set on the ladder beside the rings.
type ConcentricRingsLayer struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ConcentricRingsValues holds the layers, innermost first, and the 1-based
// index of the ring that takes the solid accent (0 = the core).
type ConcentricRingsValues struct {
	Layers    []ConcentricRingsLayer `json:"layers"`
	Highlight int                    `json:"highlight,omitempty"`
}

// ConcentricRingsOverrides is the standard text overrides: header_size sizes
// the labels, body_size the descriptions, cell_accent_mode alternate /
// progressive paints every ring a solid accent instead of the tint ladder.
type ConcentricRingsOverrides = TextOverrides

// ConcentricRingsCellOverride is the shared per-cell override; index i
// restyles the ladder text of layers[i].
type ConcentricRingsCellOverride = CellOverride

func (p *concentricRings) NewValues() any       { return &ConcentricRingsValues{} }
func (p *concentricRings) NewOverrides() any    { return &ConcentricRingsOverrides{} }
func (p *concentricRings) NewCellOverride() any { return &ConcentricRingsCellOverride{} }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	crName      = "concentric-rings"
	crMinLayers = 3
	crMaxLayers = 5
	crLabelMax  = 24
	crDescMax   = 70

	crLabelPt      = scaleSubheadPt // steps down to crLabelFloorPt when the rows need the height
	crLabelFloorPt = scaleBodyPt
	crDescPt       = scaleBodyPt
	// crTightPadPt is the margin a one-line label keeps above and below in a
	// row shorter than the uniform text margin allows: the writer clamps the
	// margin of such a shape until its one line fits, so the label is still
	// written at its size (the numbered nodes of state-shift-hub rely on the
	// same clamp).
	crTightPadPt = 5.0

	crLadderGapPt = 6.0   // ring square to the ladder column (the text keeps its own margin)
	crLadderMinPt = 130.0 // the ladder column a narrow cell still gets
	// The ladder is as wide as its longest line (estimated from the capacity
	// model with crNaturalSlack of room, bold labels crBoldWiden wider), so
	// the rings and their labels are centred as one block; the measured fit
	// decides whether that width holds the text.
	crNaturalSlack = 1.15
	crBoldWiden    = 1.1
	crSideStepPt   = 12.0 // step of the ring square while a smaller one is tried for a wider ladder

	// crBaseStepFrac is the step between two rings at the base, as a share of
	// the crest: 0 would make every circle touch at the base point.
	crBaseStepFrac = 0.25

	crDotPt      = 5.0  // leader end dot
	crDotInsetPt = 10.0 // dot centre to the ring's edge
	crLeaderPt   = 0.75 // hairline
	crFrameRound = 1e5  // layer frames are rounded to 1/100000 of the square
)

// crTintLadder is the accent "Lighter N%" swatch of a ring by its rank from
// the outside: the outermost is the lightest, and every ring is a rung of the
// one ladder that ends in the solid core (tonalRung).
var crTintLadder = [crMaxLayers]int{90, 80, 68, 56, 44}

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (p *concentricRings) Schema() *Schema {
	layer := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(crLabelMax).WithDescription("Ring label on the ladder (bold), e.g. \"Core business\""),
		"description": StringSchema(crDescMax).WithDescription("Optional line under the label. Realistic copy of up to 70 characters is written unshrunk at every layer count on every shipped template (with 5 layers on a narrow template the rings give width to the ladder); in a half-width compose segment or cell use labels only. expand_pattern reports BODY_TOO_LONG when a row cannot hold its text"),
	}, []string{"label"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"layers":    ArraySchema(RefSchema("layer"), crMinLayers, crMaxLayers).WithDescription("3-5 layers ordered inner → outer: layers[0] is the core, the last is the outermost ring (top ladder row)"),
		"highlight": IntegerSchema(1, crMaxLayers).WithDescription("1-based index of the layer drawn in the solid accent (default 1, the core); every other ring is an accent tint"),
	}, []string{"layers"}).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":         valuesSchema,
		"overrides":      textOverridesSchema(),
		"cell_overrides": CellOverridesSchema("cellOverride"),
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"layer":        layer,
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Onion model: 3-5 nested rings with a side ladder of labels")
}

func (p *concentricRings) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*ConcentricRingsValues)
	if !ok || v == nil {
		return fmt.Errorf("concentric-rings: values must be *ConcentricRingsValues, got %T", values)
	}
	var errs []error
	if overrides != nil {
		ovr, ok := overrides.(*ConcentricRingsOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("concentric-rings: overrides must be *ConcentricRingsOverrides, got %T", overrides))
		} else if ovr != nil {
			if err := ValidateCellAccentMode(crName, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	n := len(v.Layers)
	if n < crMinLayers {
		errs = append(errs, errMinItems(crName, "layers", crMinLayers, n, "(hint: use before-after or comparison-2col for two levels)"))
	}
	if n > crMaxLayers {
		errs = append(errs, errMaxItems(crName, "layers", crMaxLayers, n, "(hint: merge related layers, or use labeled-rows for a longer list)"))
	}
	for i, l := range v.Layers {
		path := fmt.Sprintf("layers[%d].label", i)
		switch {
		case strings.TrimSpace(l.Label) == "":
			errs = append(errs, errRequired(crName, path))
		case runeLen(l.Label) > crLabelMax:
			errs = append(errs, errMaxLength(crName, path, crLabelMax, runeLen(l.Label)))
		}
		if runeLen(l.Description) > crDescMax {
			errs = append(errs, errMaxLength(crName, fmt.Sprintf("layers[%d].description", i), crDescMax, runeLen(l.Description)))
		}
	}
	if v.Highlight < 0 || (n > 0 && v.Highlight > n) {
		errs = append(errs, newValidationError(crName, "highlight", ErrCodeOutOfRange,
			fmt.Sprintf("concentric-rings: highlight must be 1-%d (1 = the core, %d = the outermost ring); got %d", max(n, 1), max(n, 1), v.Highlight), nil))
	}
	if err := validateCellOverrideKeys(crName, cellOverrides, n, "(index i restyles the ladder text of layers[i])"); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// crLayout is the measured geometry Expand and PostExpandWarnings share.
// Lengths are points from the top-left corner of the pattern's block; crest
// and base are fractions of the ring square's side.
type crLayout struct {
	width, side float64
	squareX     float64 // left edge of the ring square
	ladderX     float64 // left edge of the ladder column
	ladderW     float64
	crest       float64   // height of a ring's crest = of a ladder row
	base        float64   // step between two rings at the base
	labelSize   float64   //
	descSize    float64   //
	needs       []float64 // written height each layer's ladder text needs, by layer index
}

// rowPt is the height of a ladder row.
func (l crLayout) rowPt() float64 { return l.crest * l.side }

// overflowPt is how far the tallest ladder text overflows its row.
func (l crLayout) overflowPt() float64 {
	over := 0.0
	for _, need := range l.needs {
		over = math.Max(over, need-l.rowPt())
	}
	return over
}

func (l crLayout) fits() bool { return l.overflowPt() <= ringAngleEps }

// ringFrame is the bounding square of ring k (1 = the core, m = the
// outermost) of m: every ring is centred on the vertical axis, its top one
// crest below the next ring out and its bottom one base step above it.
func (l crLayout) ringFrame(k, m int) ringFrame {
	d := l.crest + float64(k-1)*(l.crest+l.base)
	return ringFrame{X: (1 - d) / 2, Y: float64(m-k) * l.crest, W: d, H: d}
}

// rowCentre is the y of ring k's ladder row and leader (fraction of the
// square): the middle of its crest, which for the core is its own centre.
func (l crLayout) rowCentre(k, m int) float64 {
	return (float64(m-k) + 0.5) * l.crest
}

// halfChord is half the width of ring k at height y.
func (l crLayout) halfChord(k, m int, y float64) float64 {
	f := l.ringFrame(k, m)
	r := f.W / 2
	dy := y - (f.Y + r)
	return math.Sqrt(math.Max(r*r-dy*dy, 0))
}

func crOverrides(overrides any) *ConcentricRingsOverrides {
	if o, ok := overrides.(*ConcentricRingsOverrides); ok && o != nil {
		return o
	}
	return &ConcentricRingsOverrides{}
}

// crMeasure sizes the ring square and the ladder. The square takes the height
// of the content area (less in a narrow cell, where the ladder keeps
// crLadderMinPt), and a ladder row is as tall as a ring's crest. When a row
// cannot hold its text the layout gives way in order: the base step between
// the rings (the crest grows), the ladder's width (from its longest line to
// all the room beside the square), the label size (down to the 12pt floor,
// unless header_size set it), then the square itself, which widens the ladder.
// Content that fits nowhere keeps the largest square and PostExpandWarnings
// names the rows.
func crMeasure(ctx ExpandContext, v *ConcentricRingsValues, ovr *ConcentricRingsOverrides) crLayout {
	w, h := sizingAreaPt(ctx)
	gap := ctx.Gap(crLadderGapPt)
	descSize := shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, crDescPt))
	labelSizes := []float64{shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, crLabelPt))}
	if ovr.HeaderSize == 0 && labelSizes[0] > crLabelFloorPt {
		labelSizes = append(labelSizes, crLabelFloorPt)
	}
	maxSide := ringSquareSide(w, h, 0, gap+crLadderMinPt)
	minSide := math.Min(maxSide, ringMinSidePt)

	var best crLayout
	for side := maxSide; side >= minSide-ringAngleEps; side -= crSideStepPt {
		for _, ls := range labelSizes {
			for _, full := range []bool{false, true} {
				cand := crPlace(ctx, v, w, side, gap, ls, descSize, full)
				if cand.fits() {
					return cand
				}
				// A smaller square is only worth it when everything then fits.
				if side == maxSide && (best.needs == nil || cand.overflowPt() < best.overflowPt()-0.5) {
					best = cand
				}
			}
		}
	}
	return best
}

// crPlace resolves one candidate: a ring square of the given side with the
// ladder beside it, the pair centred in the width. The ladder is as wide as
// its longest line, or (full) as all the room beside the square.
func crPlace(ctx ExpandContext, v *ConcentricRingsValues, w, side, gap, labelSize, descSize float64, full bool) crLayout {
	m := max(len(v.Layers), 1)
	lay := crLayout{width: w, side: side, labelSize: labelSize, descSize: descSize}
	lay.ladderW = math.Max(w-side-gap, 1)
	if !full {
		lay.ladderW = math.Min(lay.ladderW, math.Max(crNaturalLadderPt(v, labelSize, descSize), crLadderMinPt))
	}
	lay.squareX = math.Max((w-side-gap-lay.ladderW)/2, 0)
	lay.ladderX = lay.squareX + side + gap

	need := 0.0
	lay.needs = make([]float64, len(v.Layers))
	for i, l := range v.Layers {
		lay.needs[i] = crRowNeedPt(ctx, l, lay)
		need = math.Max(need, lay.needs[i])
	}
	// The designed crest keeps the base step; a row that needs more takes it
	// from the step, down to none (m crests fill the square).
	mf := float64(m)
	lay.crest = 1 / (mf + (mf-1)*crBaseStepFrac)
	lay.crest = math.Max(lay.crest, math.Min(need/side, 1/mf))
	if m > 1 {
		lay.base = math.Max((1-mf*lay.crest)/(mf-1), 0)
	}
	return lay
}

// crNaturalLadderPt estimates the ladder width that sets every label and
// description on one line.
func crNaturalLadderPt(v *ConcentricRingsValues, labelSize, descSize float64) float64 {
	widest := 0.0
	for _, l := range v.Layers {
		widest = math.Max(widest, float64(runeLen(strings.TrimSpace(l.Label)))*labelSize*crBoldWiden)
		widest = math.Max(widest, float64(runeLen(strings.TrimSpace(l.Description)))*descSize)
	}
	return math.Ceil(widest*sizingCapacityEm*crNaturalSlack + 2*defaultShapeInsetLRPt)
}

// crRowNeedPt is the row height the ladder text of l needs: its written-fit
// height at the ladder's width, or, for a label on its own that stays on one
// line (with the slack a wider renderer face needs), that line and a tight
// margin.
func crRowNeedPt(ctx ExpandContext, l ConcentricRingsLayer, lay crLayout) float64 {
	need := rowTextNeedPt(ctx.themeFonts(), crLadderText(l, lay, "dk1"), lay.ladderW)
	if strings.TrimSpace(l.Description) != "" {
		return need
	}
	textW := (lay.ladderW - 2*defaultShapeInsetLRPt) / pptx.StandInWordFitSlack
	if measuredLines(strings.TrimSpace(l.Label), ctx.Theme.BodyFont, true, lay.labelSize, textW) > 1 {
		return need
	}
	return math.Min(need, math.Ceil(lay.labelSize*sizingLineSpacing+2*crTightPadPt))
}

// PostExpandWarnings reports, by measurement, every layer whose label and
// description outgrow their ladder row at the readable floor.
func (p *concentricRings) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ConcentricRingsValues)
	if !ok || v == nil || len(v.Layers) == 0 {
		return nil
	}
	lay := crMeasure(ctx, v, crOverrides(overrides))
	if lay.fits() {
		return nil
	}
	var warnings []string
	for i, l := range v.Layers {
		if lay.needs[i] <= lay.rowPt()+ringAngleEps {
			continue
		}
		field, advice := "label", "shorten the label or use fewer layers"
		if strings.TrimSpace(l.Description) != "" {
			field, advice = "description", fmt.Sprintf("keep it to about %d characters, drop it, or use fewer layers", crDescBudget(ctx, l, lay))
		}
		warnings = append(warnings, fmt.Sprintf("%s: concentric-rings layers[%d].%s needs %.0fpt but a ladder row with %d layers is %.0fpt tall beside a %.0fpt ring; %s",
			ErrCodeBodyTooLong, i, field, lay.needs[i], len(v.Layers), lay.rowPt(), lay.side, advice))
	}
	return warnings
}

// crDescBudget is about how many description characters the row of l holds
// under its label (0 when the label alone fills the row).
func crDescBudget(ctx ExpandContext, l ConcentricRingsLayer, lay crLayout) int {
	labelOnly := rowTextNeedPt(ctx.themeFonts(), crLadderText(ConcentricRingsLayer{Label: l.Label}, lay, "dk1"), lay.ladderW)
	lines := math.Floor((lay.rowPt() - labelOnly) / (lay.descSize * sizingLineSpacing))
	if lines < 1 {
		return 0
	}
	cpl := math.Floor((lay.ladderW - 2*sizingInsetLRPt) / (sizingCapacityEm * lay.descSize))
	return int(lines * cpl)
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *concentricRings) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*ConcentricRingsValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("concentric-rings: values must be *ConcentricRingsValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*ConcentricRingsOverrides); !ok {
			return nil, fmt.Errorf("concentric-rings: overrides must be *ConcentricRingsOverrides, got %T", overrides)
		}
	}
	m := len(v.Layers)
	if m == 0 {
		return nil, fmt.Errorf("concentric-rings: at least one layer is required")
	}
	ovr := crOverrides(overrides)
	lay := crMeasure(ctx, v, ovr)

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	tones := make([]fillTone, m)
	inks := make([]string, m)
	for i := range tones {
		tones[i] = crRingTone(ctx, accent, ovr.CellAccentMode, i, m, v.highlighted(i))
		inks[i] = crInkOn(ctx, tones[i])
	}
	pageInk := crInkOn(ctx, fillTone{Color: "lt1"})

	ring := &jsonschema.GridCellInput{Fit: "contain"}
	// Outermost first: every inner ring is drawn on top of the one around it.
	for k := m; k >= 1; k-- {
		ring.Layers = append(ring.Layers, jsonschema.LayerInput{
			Name:  fmt.Sprintf("ring-%d", k),
			Frame: crLayerFrame(lay.ringFrame(k, m)),
			Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: tones[k-1].fillJSON(), Line: noLine},
		})
	}
	for k := m; k >= 1; k-- {
		ring.Layers = append(ring.Layers, crLeaderLayers(lay, k, m, inks, pageInk)...)
	}

	places := []ringPlacement{{X0: lay.squareX, X1: lay.squareX + lay.side, Y0: 0, Y1: lay.side, Cell: ring}}
	accentInk := sshAccentInk(ctx, accent)
	for i, l := range v.Layers {
		k := i + 1
		labelInk := "dk1"
		if v.highlighted(i) && isUniformAccentMode(ovr.CellAccentMode) {
			labelInk = accentInk
		}
		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     json.RawMessage(`"none"`),
				Text:     crLadderText(l, lay, labelInk),
			},
		}
		if co, ok := cellOverrides[i].(*ConcentricRingsCellOverride); ok {
			applyCellTextOverride(cell, co)
			if co.AccentBar {
				cell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 3}
			}
		}
		top := float64(m-k) * lay.rowPt()
		places = append(places, ringPlacement{X0: lay.ladderX, X1: lay.ladderX + lay.ladderW, Y0: top, Y1: top + lay.rowPt(), Cell: cell})
	}
	return ringLattice(places, lay.width, lay.side)
}

// highlighted reports whether layer i (0-based) takes the solid accent: the
// one `highlight` names, the core without one.
func (v *ConcentricRingsValues) highlighted(i int) bool {
	return i == max(v.Highlight, 1)-1
}

func isUniformAccentMode(mode string) bool {
	return mode == "" || mode == CellAccentUniform
}

// crRingTone is the fill of layer i of m. By default (and with
// cell_accent_mode uniform) the highlighted ring is the one solid accent and
// every other ring a rung of the accent ladder, lightest outside; alternate / progressive
// paint every ring the solid accent the mode gives its index.
func crRingTone(ctx ExpandContext, accent, mode string, i, m int, highlighted bool) fillTone {
	switch {
	case !isUniformAccentMode(mode):
		return fillTone{Color: ctx.ResolveCellAccent(accent, i, mode)}
	case highlighted:
		return fillTone{Color: accent}
	default:
		return tonalRung(ctx, accent, crTintLadder[min(max(m-1-i, 0), crMaxLayers-1)])
	}
}

// crInkOn is the ink of a leader and its dot on tone, chosen by measurement
// in the contrast fixer's order (lt1, dk2, dk1). Without a theme a tint
// (lumOff) takes dark ink and a solid fill light ink.
func crInkOn(ctx ExpandContext, tone fillTone) string {
	fallback := "lt1"
	if tone.LumOff > 0 || tone.Color == "lt1" {
		fallback = "dk1"
	}
	return readableTextOn(ctx, tone, fallback)
}

// crLeaderLayers draws ring k's leader: a hairline from a dot inside the
// ring's crest to the right edge of the square, where the ladder row begins.
// The line crosses every ring outside k, so it is cut at their edges and each
// piece takes the ink measured on the fill under it.
func crLeaderLayers(lay crLayout, k, m int, inks []string, pageInk string) []jsonschema.LayerInput {
	y := lay.rowCentre(k, m)
	dot := crDotPt / lay.side
	thick := crLeaderPt / lay.side
	start := math.Max(0.5+lay.halfChord(k, m, y)-crDotInsetPt/lay.side, 0.5)

	type piece struct {
		x0, x1 float64
		ink    string
	}
	var pieces []piece
	x := start
	add := func(to float64, ink string) {
		to = math.Min(to, 1)
		if to-x < 1/crFrameRound {
			return
		}
		if n := len(pieces); n > 0 && pieces[n-1].ink == ink {
			pieces[n-1].x1 = to
		} else {
			pieces = append(pieces, piece{x, to, ink})
		}
		x = to
	}
	for j := k; j <= m; j++ {
		add(0.5+lay.halfChord(j, m, y), inks[j-1])
	}
	add(1, pageInk)

	out := make([]jsonschema.LayerInput, 0, len(pieces)+1)
	for n, pc := range pieces {
		ink, _ := json.Marshal(pc.ink)
		out = append(out, jsonschema.LayerInput{
			Name:  fmt.Sprintf("leader-%d-%d", k, n+1),
			Frame: crLayerFrame(ringFrame{X: pc.x0, Y: y - thick/2, W: pc.x1 - pc.x0, H: thick}),
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ink, Line: noLine},
		})
	}
	ink, _ := json.Marshal(inks[k-1])
	return append(out, jsonschema.LayerInput{
		Name:  fmt.Sprintf("dot-%d", k),
		Frame: crLayerFrame(ringFrame{X: start - dot/2, Y: y - dot/2, W: dot, H: dot}),
		Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: ink, Line: noLine},
	})
}

// crLayerFrame rounds a frame and keeps it inside the unit square.
func crLayerFrame(f ringFrame) jsonschema.LayerFrameInput {
	r := func(v float64) float64 { return math.Round(v*crFrameRound) / crFrameRound }
	x, y := math.Max(r(f.X), 0), math.Max(r(f.Y), 0)
	return jsonschema.LayerFrameInput{X: x, Y: y, W: math.Min(r(f.W), 1-x), H: math.Min(r(f.H), 1-y)}
}

type crPara struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align"`
}

// crLadderText is the ladder text of one layer: the bold label and, when
// given, the description under it, left-aligned and centred on the leader.
func crLadderText(l ConcentricRingsLayer, lay crLayout, labelInk string) json.RawMessage {
	label := crPara{Content: strings.TrimSpace(l.Label), Size: lay.labelSize, Bold: true, Color: labelInk, Align: "l"}
	paras := []crPara{label}
	if d := strings.TrimSpace(l.Description); d != "" {
		paras = append(paras, crPara{Content: d, Size: lay.descSize, Color: "dk1", Align: "l"})
	}
	data, _ := json.Marshal(struct {
		Paragraphs    []crPara `json:"paragraphs"`
		Align         string   `json:"align"`
		VerticalAlign string   `json:"vertical_align"`
	}{paras, "l", "ctr"})
	return data
}
