package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// cycle-figure-eight pattern — two coupled loops of 4-8 phases
// ---------------------------------------------------------------------------
//
// The infinity loop (build ↔ run, dev ↔ ops, plan ↔ deliver): one path that
// runs round a left loop, through a crossing, round a right loop and back.
// OOXML has no lemniscate, so the figure is two rings side by side, each open
// at the crossing: the left lobe is travelled counter-clockwise (up from the
// crossing, over the top, back along the bottom), the right lobe clockwise.
// The badges count along the path — 1..k on the left lobe, k+1..N on the
// right.
//
// Each lobe is its own fit-"contain" lattice cell, so both stay round in any
// area; the two squares share an edge, and the middle of that edge is the
// crossing point. The wedge a lobe leaves open towards it is the one at which
// the lobe's centreline is tangent to the straight line through the crossing
// point: cos δ = R / 0.5. Along those two lines run the four arms of the
// crossing, two per cell: a rotated rect as wide as the band, from inside the
// lobe's end segment to just past the crossing point, in the band's fill and
// without an outline — so the band runs through the crossing in one piece and
// the only gaps in the ribbon are the ones between two numbered segments. (A
// highlighted or tinted segment keeps its own colour to its end: the neutral
// arm starts a segment gap away from it.) A rotated layer's frame has to stay
// inside its cell, which a full-width arm ending on the cell's edge only does
// when the lobe is a little smaller than its square (cfeBandRadius): the two
// rings stop 1-5% of a side short of each other and the arms bridge it.
//
// The direction is a band-wide arrowhead, a deeper tint of the band, on each arm that leaves the crossing
// INTO a lobe — the two upper arms, mirror images of each other.
//
// Labels are cycle-ring's rows (ring_draw.go, cycleRingOutsideRows): the left
// lobe's in a right-aligned column on the far left, the right lobe's on the
// far right, each row level with its badge.
//
// The figure needs width: two lobes and two label columns. An area too
// narrow for them (a horizontal 50% compose segment) is refused at Expand
// with a fit_overflow finding whose fix swaps to cycle-ring.

func init() {
	Default().Register(&cycleFigureEight{})
}

type cycleFigureEight struct{}

func (p *cycleFigureEight) Name() string { return cycleFigureEightName }
func (p *cycleFigureEight) Description() string {
	return "Figure-eight / infinity loop of 4-8 phases on two coupled lobes side by side (build ↔ run, dev ↔ ops, plan ↔ deliver): the path runs round the left lobe, through the crossing and round the right, numbered along the way, each lobe's labels in a column on its own side; optional lobe titles, one optional highlighted phase; needs the slide's width"
}
func (p *cycleFigureEight) UseWhen() string {
	return "Two loops that feed each other and share one continuous path of 4-8 phases — a DevOps build / run loop, plan ↔ deliver, supply ↔ demand, learn ↔ scale — on a full-width slide or a full-width band; prefer cycle-ring for one loop or for any half-width segment, cycle-intake when linear steps feed a single loop, before-after for two states without a loop, and swimlane when two teams hand work back and forth along a timeline"
}
func (p *cycleFigureEight) NotWhen() string {
	return "There is one loop (use cycle-ring), a linear intake feeds one loop (use cycle-intake), the two sides are states rather than loops (use before-after), two actors exchange steps over time (use swimlane), the area is a half-width compose segment or grid cell under about 580pt wide (use cycle-ring, which has a legend layout), or there are more than 8 phases (merge phases or split the loops across two slides)"
}
func (p *cycleFigureEight) Version() int { return 1 }
func (p *cycleFigureEight) CellsHint() string {
	return "2 lobes (2-4 segments + badges each) + 4-8 outside labels"
}
func (p *cycleFigureEight) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"cycle-ring", "process-flow", "next-steps"},
		DensityClass:       "medium",
		AccentWeight:       "light",
		SparseThresholdPct: 15,
	}
}
func (p *cycleFigureEight) SupportsInlineMarkdown() bool { return true }

func (p *cycleFigureEight) ExemplarValues() any {
	return &CycleFigureEightValues{
		LeftLabel:  "Build",
		RightLabel: "Run",
		Phases: []CycleRingPhase{
			{Label: "Plan", Description: "Rank the backlog by customer value"},
			{Label: "Code", Description: "Small changes, reviewed in a day"},
			{Label: "Test", Description: "Automated checks gate every merge"},
			{Label: "Release", Description: "Ship behind a feature flag"},
			{Label: "Operate", Description: "On-call owns the service level"},
			{Label: "Monitor", Description: "Usage and incidents feed the plan"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CycleFigureEightValues holds the 4-8 phases of the path (cycle-ring's phase
// type: label, description, highlight), how many of them sit on the left lobe
// and the optional lobe titles.
type CycleFigureEightValues struct {
	Phases     []CycleRingPhase `json:"phases"`
	LeftCount  int              `json:"left_count,omitempty"` // default: half, rounded up
	LeftLabel  string           `json:"left_label,omitempty"`
	RightLabel string           `json:"right_label,omitempty"`
}

// CycleFigureEightOverrides is the standard text overrides (header_size sizes
// the labels, body_size the descriptions) plus the band width.
type CycleFigureEightOverrides struct {
	TextOverrides
	Thickness string `json:"thickness,omitempty"` // thin | regular (default) | thick
}

func (p *cycleFigureEight) NewValues() any       { return &CycleFigureEightValues{} }
func (p *cycleFigureEight) NewOverrides() any    { return &CycleFigureEightOverrides{} }
func (p *cycleFigureEight) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	cycleFigureEightName = "cycle-figure-eight"

	cfeMinPhases = 4
	cfeMaxPhases = 8
	cfeMinLobe   = 2 // phases on one lobe
	cfeMaxLobe   = 4

	cfeLabelMax     = 26
	cfeDescMax      = 60
	cfeLobeLabelMax = 16

	// The smallest area the figure is drawn in: two lobes of cfeMinSidePt
	// and a label column (numeral + cfeMinLabelPt) either side; and the
	// lobes' height.
	cfeMinSidePt   = 140.0
	cfeMinLabelPt  = 130.0
	cfeMinWidthPt  = 2*cfeMinSidePt + 2*(ringNumberColPt+cfeMinLabelPt)
	cfeMinHeightPt = cfeMinSidePt

	// The crossing. An arm is as wide as the band. It starts cfeArmLapFrac
	// of a side inside its segment (drawn under it, in the same fill) — or,
	// when that segment has a colour of its own, cfeArmGapDeg of arc past its
	// end, the gap between two segments — and ends cfeArmPastFrac of a side
	// past the crossing point, where the opposite cell's arm takes over.
	cfeArmGapDeg   = ringDefaultGapDeg
	cfeArmLapFrac  = 0.02
	cfeArmPastFrac = 0.012

	// The direction arrowhead on an entering arm: a triangle cfeHeadBaseFrac
	// of the band wide and cfeHeadLength times that long (longer than wide,
	// so its point is unmistakable), in the band's own colour family — the
	// accent's Lighter cfeHeadLighter% swatch on the Lighter 80% band — so
	// the direction is read from the band, not from a dark glyph laid on it
	// (go-slide-creator-y21pc). It sits midway along the part of the arm the
	// other band does not cross.
	cfeHeadBaseFrac  = 0.62
	cfeHeadLength    = 1.4
	cfeHeadMinAspect = 1.3 // least length : base of a head
	cfeHeadLighter   = 40
)

// cfeLeftCount is how many of n phases sit on the left lobe: the authored
// count, otherwise half rounded up.
func cfeLeftCount(n, authored int) int {
	if authored > 0 {
		return authored
	}
	return (n + 1) / 2
}

// cfeLeftRange is the left_count range for n phases: each lobe holds 2-4.
func cfeLeftRange(n int) (lo, hi int) {
	return max(cfeMinLobe, n-cfeMaxLobe), min(cfeMaxLobe, n-cfeMinLobe)
}

// cfeDescFourMax is the description budget when a lobe carries four phases.
const cfeDescFourMax = 40

// cfeDescBudget is the description budget (characters) for a figure whose
// fuller lobe carries busiest phases. A label column holds one row per phase
// of its lobe, so the budget follows the fuller lobe, not the phase count:
// up to three rows hold the schema maximum; four rows (7-8 phases, or six
// split 2 + 4) hold 40 before the lobes start giving their width to the
// label columns. Measured on the smallest shipped content area (abstract,
// 687 x 294pt) at default sizes with labels at their maximum
// (TestCycleFigureEightMeasuredBudgets); the label budget is the schema
// maximum at every count.
func cfeDescBudget(busiest int) int {
	if busiest >= cfeMaxLobe {
		return cfeDescFourMax
	}
	return cfeDescMax
}

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (p *cycleFigureEight) Schema() *Schema {
	phase := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(cfeLabelMax).WithDescription("Phase name, bold beside its number (e.g. \"Release\")"),
		"description": StringSchema(cfeDescMax).WithDescription("Optional muted line under the label. Budget: 60 characters while each lobe holds at most 3 phases, 40 once a lobe holds 4 (7-8 phases, or 6 split 2 + 4); expand_pattern reports BODY_TOO_LONG past it"),
		"highlight":   BooleanSchema().WithDescription("Fill this phase's segment with the solid accent (at most one phase); the others stay neutral"),
	}, []string{"label"}).WithAdditionalProperties(false)

	values := ObjectSchema(map[string]*Schema{
		"phases":      ArraySchema(phase, cfeMinPhases, cfeMaxPhases).WithDescription("4-8 phases in path order: up and round the left lobe, through the crossing, round the right lobe and back"),
		"left_count":  IntegerSchema(cfeMinLobe, cfeMaxLobe).WithDescription("How many phases sit on the left lobe (the rest on the right); each lobe holds 2-4. Default: half, rounded up"),
		"left_label":  StringSchema(cfeLobeLabelMax).WithDescription("Optional title in the left lobe's centre, e.g. \"Build\" (short words: a word never breaks)"),
		"right_label": StringSchema(cfeLobeLabelMax).WithDescription("Optional title in the right lobe's centre, e.g. \"Run\""),
	}, []string{"phases"}).WithAdditionalProperties(false)

	overrides := textOverridesSchema()
	overrides.raw.Properties["header_size"] = NumberSchema(12, 28).WithDescription("Phase label size in points (default 14, stepping down to 12 when the rows need it)")
	overrides.raw.Properties["body_size"] = NumberSchema(12, 20).WithDescription("Description size in points (default 12)")
	overrides.raw.Properties["cell_accent_mode"] = EnumSchema("uniform", "alternate", "progressive").WithDescription("uniform (default): accent-tint segments, dark badges. alternate / progressive: each segment takes a light tint of its own accent, running along the whole path").WithDefault("uniform")
	overrides.raw.Properties["thickness"] = EnumSchema(cycleRingThicknesses...).WithDescription("Band width: thin, regular or thick (14% / 20% / 28% of a lobe's diameter)").WithDefault("regular")

	return ObjectSchema(map[string]*Schema{
		"values":    values,
		"overrides": overrides,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Figure-eight loop of 4-8 phases on two coupled lobes; needs a content area at least 580pt wide")
}

func cfeOverrides(overrides any) *CycleFigureEightOverrides {
	if o, ok := overrides.(*CycleFigureEightOverrides); ok && o != nil {
		return o
	}
	return &CycleFigureEightOverrides{}
}

func (p *cycleFigureEight) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*CycleFigureEightValues)
	if !ok || v == nil {
		return fmt.Errorf("cycle-figure-eight: values must be *CycleFigureEightValues, got %T", values)
	}
	const name = cycleFigureEightName
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*CycleFigureEightOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("cycle-figure-eight: overrides must be *CycleFigureEightOverrides, got %T", overrides))
		} else if ovr != nil {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Thickness != "" && !slices.Contains(cycleRingThicknesses, ovr.Thickness) {
				errs = append(errs, errInvalidEnum(name, "overrides.thickness", ovr.Thickness, cycleRingThicknesses))
			}
		}
	}

	n := len(v.Phases)
	switch {
	case n < cfeMinPhases:
		errs = append(errs, errMinItems(name, "phases", cfeMinPhases, n, "(hint: each lobe holds 2-4 phases; use cycle-ring for a single loop or cycle-nodes for a 3-phase cycle)"))
	case n > cfeMaxPhases:
		errs = append(errs, errMaxItems(name, "phases", cfeMaxPhases, n, "(hint: merge related phases or give each loop its own cycle-ring slide)"))
	case v.LeftCount != 0:
		if lo, hi := cfeLeftRange(n); v.LeftCount < lo || v.LeftCount > hi {
			errs = append(errs, errOutOfRange(name, "left_count", lo, hi, v.LeftCount))
		}
	}
	highlights := 0
	for i, ph := range v.Phases {
		path := fmt.Sprintf("phases[%d].label", i)
		switch {
		case strings.TrimSpace(ph.Label) == "":
			errs = append(errs, errRequired(name, path))
		case runeLen(ph.Label) > cfeLabelMax:
			errs = append(errs, errMaxLength(name, path, cfeLabelMax, runeLen(ph.Label)))
		}
		if runeLen(ph.Description) > cfeDescMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].description", i), cfeDescMax, runeLen(ph.Description)))
		}
		if ph.Highlight {
			highlights++
		}
	}
	if highlights > 1 {
		errs = append(errs, newValidationError(name, "phases", ErrCodeOutOfRange,
			fmt.Sprintf("cycle-figure-eight: %d phases set highlight; at most one phase is highlighted (the single solid accent segment)", highlights), nil))
	}
	for _, e := range []struct{ path, val string }{{"left_label", v.LeftLabel}, {"right_label", v.RightLabel}} {
		if runeLen(e.val) > cfeLobeLabelMax {
			errs = append(errs, errMaxLength(name, e.path, cfeLobeLabelMax, runeLen(e.val)))
		}
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"cycle-figure-eight: cell_overrides are not supported (use overrides, or phases[].highlight for the one accent segment)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// cfeLobe is one of the two rings.
type cfeLobe struct {
	spec      ringSpec
	items     []ringItem // in path order, indexed within the lobe
	from      int        // index of its first phase
	prefix    string     // layer-name prefix
	label     string     // its title
	labelPt   float64
	labelFits bool
	arms      [2]cfeArm // into the lobe's first segment, out of its last
	head      cfeArm    // the direction arrowhead on the entering arm
}

// cfeLayout carries every measurement Expand and PostExpandWarnings share. The
// embedded cycle-ring layout holds the area, the lobe square (side, y0; x0 is
// the LEFT lobe's left edge), the label rows of all phases and the type
// sizes; its items are the phases of both lobes in path order.
type cfeLayout struct {
	cycleRingLayout
	lobes [2]cfeLobe
}

// cfeArm is one rotated layer of the crossing: its frame and its rotation
// (degrees clockwise; the frame's long side then lies along the arm).
type cfeArm struct {
	frame  ringFrame
	rotDeg float64
}

// cfeCrossDist is how far the crossing point is from a lobe's centre: half
// the cell's side (the middle of the edge the two cells share).
const cfeCrossDist = 0.5

// cfeBandRadius is the centreline radius of a lobe whose band is thickness
// wide (fractions of the cell's side): the largest at which a full-width arm
// from the lobe's end to the crossing point has its unrotated frame inside
// the cell. The arm's centre is halfway between the lobe's end, at
// x = 0.5 + R·cos δ = 0.5 + 2R², and the cell's edge, so half the arm's width
// has to fit in (0.5 − 2R²)/2: R = √((0.5 − thickness)/2). The band then
// stops short of the cell's edge.
func cfeBandRadius(thickness float64) float64 {
	return math.Min(math.Sqrt(math.Max(cfeCrossDist-thickness, 0)/2), cfeCrossDist-thickness/2)
}

// cfeLobeDiameter is the outer diameter (fraction of the cell's side) of a
// lobe whose band is frac of that diameter wide: cfeBandRadius solved for a
// thickness of frac × diameter.
func cfeLobeDiameter(frac float64) float64 {
	return (math.Sqrt(frac*frac+(1-frac)*(1-frac)) - frac) / ((1 - frac) * (1 - frac))
}

// cfeOpenDeg is the half-opening of a lobe at the crossing: the angle at
// which the lobe's centreline is tangent to the straight line through the
// crossing point (cos δ = R / distance to that point).
func cfeOpenDeg(spec ringSpec) float64 {
	return radToDeg(math.Acos(math.Min(spec.Radius/cfeCrossDist, 1)))
}

// cfeArmAt is the arm between the lobe's centreline at deg and the crossing
// point (touchX, 0.5) on the cell's edge. A joined arm starts inside the
// lobe's end segment, any other a segment gap past it; both end just past the
// crossing point.
func cfeArmAt(spec ringSpec, deg, touchX float64, joined bool) cfeArm {
	px, py := pointOnCircle(0.5, 0.5, spec.Radius, deg)
	dx, dy := touchX-px, 0.5-py
	dist := math.Hypot(dx, dy)
	if dist < ringAngleEps {
		return cfeArm{}
	}
	ux, uy := dx/dist, dy/dist
	from := -cfeArmLapFrac
	if !joined {
		from = math.Min(spec.Radius*degToRad(cfeArmGapDeg), dist/2)
	}
	to := dist + cfeArmPastFrac
	mid := (from + to) / 2
	mx, my := px+mid*ux, py+mid*uy
	// The unrotated frame (width × length) must stay inside the cell.
	width := math.Min(spec.Thickness, 2*math.Min(mx, 1-mx))
	return cfeArm{
		frame:  ringFrame{X: mx - width/2, Y: my - (to-from)/2, W: width, H: to - from},
		rotDeg: normDeg(radToDeg(math.Atan2(ux, -uy))), // the frame's top points at the crossing
	}
}

// cfeHeadAt is the direction arrowhead on the arm that enters the lobe at
// deg: centred on the arm's axis, pointing away from the crossing point,
// midway along the stretch between the other band's edge and the lobe's end.
func cfeHeadAt(spec ringSpec, deg, touchX float64) cfeArm {
	px, py := pointOnCircle(0.5, 0.5, spec.Radius, deg)
	dx, dy := touchX-px, 0.5-py
	dist := math.Hypot(dx, dy)
	if dist < ringAngleEps {
		return cfeArm{}
	}
	ux, uy := dx/dist, dy/dist
	// The two bands cross at twice the arm's angle to the horizontal; the
	// other band covers this one up to free from the crossing point.
	sin, cos := math.Abs(2*ux*uy), math.Abs(ux*ux-uy*uy)
	free := math.Min(spec.Thickness/2*(1+cos)/math.Max(sin, ringAngleEps), dist)
	at := (free + dist) / 2
	cx, cy := touchX-at*ux, 0.5-at*uy
	base := spec.Thickness * cfeHeadBaseFrac
	length := math.Min(base*cfeHeadLength, (dist-free)*0.8)
	// A head the arm has no room to draw at full length keeps its
	// proportion: a squat triangle does not say which way it points.
	base = math.Min(base, length/cfeHeadMinAspect)
	return cfeArm{
		frame:  ringFrame{X: cx - base/2, Y: cy - length/2, W: base, H: length},
		rotDeg: normDeg(radToDeg(math.Atan2(-ux, uy))), // a triangle points up at 0
	}
}

// errCFENarrow refuses an area that cannot hold two lobes and their label
// columns; the fix swaps to cycle-ring, whose legend layout fits it.
func errCFENarrow(w, h float64) *ValidationError {
	return newValidationError(cycleFigureEightName, "values", ErrCodeFitOverflow,
		fmt.Sprintf("cycle-figure-eight: two lobes with a label column either side need a content area at least %.0fpt wide and %.0fpt tall, but this one is %.0f x %.0fpt — give the figure the full slide width (a vertical compose segment, not a horizontal one) or use cycle-ring, which draws one ring with a legend in a narrow area",
			cfeMinWidthPt, cfeMinHeightPt, w, h),
		&FixSuggestion{Kind: "swap_pattern", Params: map[string]any{
			"suggested": []any{map[string]any{
				"from":      cycleFigureEightName,
				"to":        cycleRingName,
				"rationale": "cycle-ring draws the phases as one ring with a numbered legend in an area too narrow for two lobes",
			}},
		}})
}

// cfeMeasure resolves the geometry for the content area: the two lobe
// squares, the label rows and the type sizes. An area too small for the
// figure is an errCFENarrow.
func cfeMeasure(ctx ExpandContext, v *CycleFigureEightValues, ovr *CycleFigureEightOverrides) (cfeLayout, error) {
	w, h := sizingAreaPt(ctx)
	if w < cfeMinWidthPt-0.5 || h < cfeMinHeightPt-0.5 {
		return cfeLayout{}, errCFENarrow(w, h)
	}
	base := cfeLayout{cycleRingLayout: cycleRingLayout{
		w: w, h: h,
		descPt:   shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, cycleRingDescPt)),
		showDesc: true,
	}}
	sizes := []float64{shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, cycleRingLabelPt))}
	if ovr.HeaderSize == 0 {
		sizes = append(sizes, shapegrid.MinTextSizePt)
	}

	// A label column either side takes cycle-ring's share of the width; the
	// lobes take the rest, at most the height.
	reserve := math.Max(cycleRingReservePt, w*cycleRingReserveFrac)
	full := math.Min(h, (w-2*reserve)/2)
	if full < cfeMinSidePt {
		full = cfeMinSidePt
	}
	// When the label rows do not fit beside the lobes even at the 12pt floor,
	// the lobes give up to cycleRingMaxShrink of their side to the columns.
	crv := &CycleRingValues{Phases: v.Phases}
	var first cfeLayout
	var held *cfeLayout
	for step := 0; step <= cycleRingShrinkSteps; step++ {
		side := full * (1 - cycleRingMaxShrink*float64(step)/cycleRingShrinkSteps)
		if step > 0 && side < cfeMinSidePt {
			break
		}
		lay, err := cfePlace(ctx, v, ovr, base, side)
		if err != nil {
			return lay, err
		}
		if cycleRingOutsideRows(ctx, crv, &lay.cycleRingLayout, sizes) {
			return lay, nil
		}
		if step == 0 {
			first = lay
		}
		if lay.rowsFit && held == nil {
			held = &lay
		}
	}
	// A word no column can hold: the largest lobes whose rows fit.
	if held != nil {
		return *held, nil
	}
	return first, nil
}

// cfePlace sets two lobe squares of the given side, centred in the area, and
// everything that follows: the bands, the items, the badges, the arrowheads
// and the lobe titles' sizes.
func cfePlace(ctx ExpandContext, v *CycleFigureEightValues, ovr *CycleFigureEightOverrides, lay cfeLayout, side float64) (cfeLayout, error) {
	n := len(v.Phases)
	k := cfeLeftCount(n, v.LeftCount)
	lay.side = math.Max(math.Min(side, math.Min(lay.w/2, lay.h)), 1)
	lay.y0 = (lay.h - lay.side) / 2
	lay.x0 = (lay.w - 2*lay.side) / 2
	lay.labelW = math.Max(lay.x0-ringNumberColPt, 1)

	// The band is its share of the lobe's diameter (wider on a small lobe, so
	// it still carries its badge), and the lobe as large as its arms allow.
	want := cycleRingThicknessFrac(ovr.Thickness)
	thickness := ringBandFrac(want*cfeLobeDiameter(want), lay.side)
	band := newRingSpec(1).withBand(2*cfeBandRadius(thickness)+thickness, thickness)
	open := cfeOpenDeg(band)
	touch := [2]float64{1, 0}
	plain := ovr.CellAccentMode != "alternate" && ovr.CellAccentMode != "progressive"
	// Left: counter-clockwise from the upper side of the crossing (3 o'clock
	// on its ring). Right: clockwise from the upper side of the crossing
	// (9 o'clock on its ring).
	lay.lobes = [2]cfeLobe{
		{spec: newRingArcSpec(k, -open, open, false), from: 0, prefix: "left-", label: strings.TrimSpace(v.LeftLabel)},
		{spec: newRingArcSpec(n-k, 180+open, 180-open, true), from: k, prefix: "right-", label: strings.TrimSpace(v.RightLabel)},
	}
	lay.items = lay.items[:0:0]
	for li := range lay.lobes {
		lobe := &lay.lobes[li]
		lobe.spec = lobe.spec.withBand(2*band.outerRadius(), band.Thickness)
		items, err := lobe.spec.items()
		if err != nil {
			return lay, fmt.Errorf("cycle-figure-eight: %w", err)
		}
		lobe.items = items
		labelSide := ringSideLeft
		if li == 1 {
			labelSide = ringSideRight
		}
		for _, it := range items {
			it.Index += lobe.from
			it.Side = labelSide
			lay.items = append(lay.items, it)
		}
		first, last := lobe.from, lobe.from+len(items)-1
		lobe.arms[0] = cfeArmAt(lobe.spec, items[0].StartDeg, touch[li], plain && !v.Phases[first].Highlight)
		lobe.arms[1] = cfeArmAt(lobe.spec, items[len(items)-1].EndDeg, touch[li], plain && !v.Phases[last].Highlight)
		lobe.head = cfeHeadAt(lobe.spec, items[0].StartDeg, touch[li])
		lobe.labelPt, lobe.labelFits = ringCentreFit(ctx, lobe.spec, lay.side, lobe.label, "")
	}
	lay.spec = lay.lobes[0].spec
	lay.badgeDia = ringBadgeDia(band.Thickness, lay.side)
	return lay, nil
}

// ---------------------------------------------------------------------------
// Warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports copy over the per-count budget, a label row that
// cannot hold its text on this template and a lobe title that does not fit
// its lobe.
func (p *cycleFigureEight) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*CycleFigureEightValues)
	if !ok || v == nil || len(v.Phases) < cfeMinPhases || len(v.Phases) > cfeMaxPhases {
		return nil
	}
	lay, err := cfeMeasure(ctx, v, cfeOverrides(overrides))
	if err != nil {
		return nil
	}
	n := len(v.Phases)
	left := cfeLeftCount(n, v.LeftCount)
	busiest := max(left, n-left)
	descBudget := cfeDescBudget(busiest)
	var warnings []string
	for i, ph := range v.Phases {
		named := false
		if c := runeLen(ph.Description); c > descBudget {
			named = true
			warnings = append(warnings, fmt.Sprintf("%s: cycle-figure-eight phases[%d].description is %d characters; with %d phases on a lobe use about %d — shorten the description, or use at most 6 phases split 3 + 3",
				ErrCodeBodyTooLong, i, c, busiest, descBudget))
		}
		if row := lay.rows[i]; row.capped && !named {
			field := "label"
			if strings.TrimSpace(ph.Description) != "" {
				field = "description"
			}
			warnings = append(warnings, fmt.Sprintf("%s: cycle-figure-eight phases[%d].%s needs %.0fpt but its label row is %.0fpt tall in a %.0f x %.0fpt content area with %d phases — shorten it, drop the descriptions or balance left_count",
				ErrCodeBodyTooLong, i, field, row.need, row.y1-row.y0, lay.w, lay.h, n))
		}
	}
	for li, field := range []string{"left_label", "right_label"} {
		if lobe := lay.lobes[li]; lobe.label != "" && !lobe.labelFits {
			warnings = append(warnings, fmt.Sprintf("%s: cycle-figure-eight %s does not fit the lobe's %.0fpt hole at 12pt (a word breaks or the text runs past it) — shorten it or use overrides.thickness thin",
				ErrCodeBodyTooLong, field, 2*(lobe.spec.Radius-lobe.spec.Thickness/2)*lay.side))
		}
	}
	return warnings
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *cycleFigureEight) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*CycleFigureEightValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("cycle-figure-eight: values must be *CycleFigureEightValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*CycleFigureEightOverrides); !ok {
			return nil, fmt.Errorf("cycle-figure-eight: overrides must be *CycleFigureEightOverrides, got %T", overrides)
		}
	}
	n := len(v.Phases)
	if n < cfeMinPhases || n > cfeMaxPhases {
		return nil, fmt.Errorf("cycle-figure-eight: %d phases; the figure holds %d-%d", n, cfeMinPhases, cfeMaxPhases)
	}
	if lo, hi := cfeLeftRange(n); v.LeftCount != 0 && (v.LeftCount < lo || v.LeftCount > hi) {
		return nil, fmt.Errorf("cycle-figure-eight: left_count %d with %d phases; each lobe holds %d-%d", v.LeftCount, n, cfeMinLobe, cfeMaxLobe)
	}
	ovr := cfeOverrides(overrides)
	lay, err := cfeMeasure(ctx, v, ovr)
	if err != nil {
		return nil, err
	}

	base := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	accents := make([]string, n)
	highlight := -1
	for i, ph := range v.Phases {
		accents[i] = ctx.ResolveCellAccent(base, i, ovr.CellAccentMode)
		if ph.Highlight && highlight < 0 {
			highlight = i
		}
	}

	places := make([]ringPlacement, 0, 2+2*n)
	for li, lobe := range lay.lobes {
		paint := ringPaint{
			Accents:    accents[lobe.from : lobe.from+len(lobe.items)],
			Tinted:     ovr.CellAccentMode == "alternate" || ovr.CellAccentMode == "progressive",
			Highlight:  -1,
			BadgeDia:   lay.badgeDia,
			NumberFrom: lobe.from + 1,
			Prefix:     lobe.prefix,
		}
		if highlight >= lobe.from && highlight < lobe.from+len(lobe.items) {
			paint.Highlight = highlight - lobe.from
		}
		layers := [][]jsonschema.LayerInput{
			{
				cfeArmLayer(lobe.prefix+"arm-in", "rect", lobe.arms[0], ringBandTone(ctx, base).fillJSON()),
				cfeArmLayer(lobe.prefix+"arm-out", "rect", lobe.arms[1], ringBandTone(ctx, base).fillJSON()),
			},
			ringSegmentLayers(ctx, lobe.spec, lobe.items, paint),
			{cfeHeadLayer(lobe.prefix+"arrowhead", lobe.head, cfeHeadTone(ctx, base))},
			ringBadgeLayers(ctx, lobe.spec, lobe.items, paint),
		}
		if centre, ok := ringCentreLayer(lobe.spec, lobe.label, "", lobe.labelPt, lobe.prefix); ok {
			layers = append(layers, []jsonschema.LayerInput{centre})
		}
		// The labels follow the lobes into their bounding squares, so each
		// lobe's cell is the spine column through its centre.
		places = append(places, ringSpinePlacement(lay.x0+(float64(li)+0.5)*lay.side, lay.y0, lay.side, ringCell(layers...)))
	}
	labelAccents := ringPaint{Accents: accents, Highlight: highlight,
		Tinted: ovr.CellAccentMode == "alternate" || ovr.CellAccentMode == "progressive"}.labelAccents(ctx, n)
	places = append(places, lay.labelPlaces(ctx, v.Phases, labelAccents)...)

	grid, err := ringLattice(places, lay.w, lay.h)
	if err != nil {
		return nil, fmt.Errorf("cycle-figure-eight: %w", err)
	}
	grid.VerticalAlign = "middle"
	return grid, nil
}

// cfeArmLayer is one filled, rotated layer of the crossing.
func cfeArmLayer(name, geometry string, a cfeArm, fill json.RawMessage) jsonschema.LayerInput {
	return jsonschema.LayerInput{
		Name:  name,
		Frame: a.frame.layer(),
		Shape: &jsonschema.ShapeSpecInput{Geometry: geometry, Fill: fill, Line: noLine, Rotation: a.rotDeg},
	}
}

// cfeHeadLayer is the direction arrowhead on an entering arm.
func cfeHeadLayer(name string, a cfeArm, tone fillTone) jsonschema.LayerInput {
	return cfeArmLayer(name, "triangle", a, tone.fillJSON())
}

// cfeHeadTone is the arrowhead's fill: a deeper rung of the band's own
// ladder, so the direction is read from the band rather than from a dark
// glyph laid on it.
func cfeHeadTone(ctx ExpandContext, accent string) fillTone {
	return tonalRung(ctx, accent, cfeHeadLighter)
}
