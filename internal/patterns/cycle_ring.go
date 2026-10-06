package patterns

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// cycle-ring pattern — a closed ring of 4-8 phases
// ---------------------------------------------------------------------------
//
// The consulting cycle (SmartArt "Segmented Cycle" / "Continuous Cycle"): a
// ring of equal segments for a lifecycle, a PDCA loop, an operating rhythm or
// a flywheel. Phase 1's segment begins at 12 o'clock and the ring runs
// clockwise (overrides.direction turns it round).
//
// The ring is ONE lattice cell whose layers are the segments (blockArc, or
// circularArrow for overrides.style "arrows"), a numbered badge on each
// segment and an optional label in the hole (ring_draw.go). The phase labels
// are their own lattice cells OUTSIDE the ring, left and right of it: a
// numeral in the accent — the same number as the badge — then the bold label
// over a muted description. Each label row sits as near its badge's height as
// its neighbours allow (ringLabelRows) and at one constant gap from the ring's
// outer edge at that height (outsideRow), so the rows follow the ring's curve;
// the ring cell is then the spine column through the ring's centre
// (ringSpinePlacement), and the fit "contain" square in the legend layout.
//
// A content area too narrow for a label column each side of the ring (a 50%
// compose segment) takes the legend layout instead: the ring on the left and
// one numbered list beside it.

func init() {
	Default().Register(&cycleRing{})
}

type cycleRing struct{}

func (p *cycleRing) Name() string { return "cycle-ring" }
func (p *cycleRing) Description() string {
	return "Closed ring of 4-8 equal phase segments with numbered badges and the phase labels outside beside their segments (lifecycle, PDCA, operating rhythm, flywheel); one optional highlighted phase, optional centre label, `arrows` style for chasing arrows"
}
func (p *cycleRing) UseWhen() string {
	return "A recurring cycle of 4-8 equally weighted phases that closes on itself — continuous improvement / PDCA, a customer or product lifecycle, an operating rhythm, a flywheel — each phase a short label and an optional one-to-two line description; prefer cycle-nodes for 3 phases or discrete stations joined by arrows, cycle-intake when linear steps feed the loop, cycle-figure-eight for two coupled loops, radial-hub when the items relate to a centre rather than to each other, and process-flow when the sequence does not loop back"
}
func (p *cycleRing) NotWhen() string {
	return "The steps run once from start to end with no loop back (use process-flow or numbered-step-strip), there are 3 phases or the phases are discrete stations joined by arrows (use cycle-nodes), a linear intake feeds the loop (use cycle-intake), two loops are coupled (use cycle-figure-eight), the items surround a central idea without an order (use radial-hub), layers nest inside each other (use concentric-rings), or there are more than 8 phases (split the cycle across two slides)"
}
func (p *cycleRing) Version() int      { return 1 }
func (p *cycleRing) CellsHint() string { return "1 ring (4-8 segments + badges) + 4-8 outside labels" }
func (p *cycleRing) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"process-flow", "kpi-3up", "next-steps"},
		DensityClass:       "medium",
		AccentWeight:       "light",
		SparseThresholdPct: 15,
	}
}
func (p *cycleRing) SupportsInlineMarkdown() bool { return true }

func (p *cycleRing) ExemplarValues() any {
	return &CycleRingValues{
		Center: &CycleRingCenter{Label: "Continuous improvement"},
		Phases: []CycleRingPhase{
			{Label: "Plan", Description: "Set the target and the hypothesis to test"},
			{Label: "Do", Description: "Run the change in one pilot plant"},
			{Label: "Check", Description: "Compare results against the baseline"},
			{Label: "Act", Description: "Standardise what worked and scale it"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CycleRingPhase is one segment of the ring.
type CycleRingPhase struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Highlight   bool   `json:"highlight,omitempty"`
}

// CycleRingCenter is the optional text in the ring's hole.
type CycleRingCenter struct {
	Label    string `json:"label,omitempty"`
	Sublabel string `json:"sublabel,omitempty"`
}

// CycleRingValues holds the 4-8 phases and the optional centre.
type CycleRingValues struct {
	Phases []CycleRingPhase `json:"phases"`
	Center *CycleRingCenter `json:"center,omitempty"`
}

// CycleRingOverrides is the standard text overrides (header_size sizes the
// labels, body_size the descriptions) plus the ring's look.
type CycleRingOverrides struct {
	TextOverrides
	Style     string `json:"style,omitempty"`     // segments (default) | arrows
	Direction string `json:"direction,omitempty"` // clockwise (default) | counter_clockwise
	Thickness string `json:"thickness,omitempty"` // thin | regular (default) | thick
	Labels    string `json:"labels,omitempty"`    // outside | legend; default: outside when the area is wide enough
}

func (p *cycleRing) NewValues() any       { return &CycleRingValues{} }
func (p *cycleRing) NewOverrides() any    { return &CycleRingOverrides{} }
func (p *cycleRing) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	cycleRingName      = "cycle-ring"
	cycleRingMinPhases = 4
	cycleRingMaxPhases = 8

	cycleRingLabelMax    = 28
	cycleRingDescMax     = 90
	cycleRingCenterMax   = 24
	cycleRingSublabelMax = 32

	cycleRingLabelPt = scaleSubheadPt // steps down to 12pt when the rows need it
	cycleRingDescPt  = scaleBodyPt

	// A label column each side of the ring is at least cycleRingReservePt
	// wide (wider on a wide slide); the outside layout needs that and a ring
	// of cycleRingOutsideSidePt, otherwise the legend layout takes over.
	cycleRingReservePt     = 150.0
	cycleRingReserveFrac   = 0.22
	cycleRingOutsideSidePt = 150.0
	// Without descriptions the column is as wide as the longest label needs,
	// from cycleRingShortLabelPt up in steps of cycleRingReserveStepPt.
	cycleRingShortLabelPt  = 48.0
	cycleRingReserveStepPt = 6.0

	cycleRingRowGapPt    = 4.0
	cycleRingMinRowGapPt = 2.0
	cycleRingArrowGapDeg = 3.0  // chasing arrows sit closer than segments
	cycleRingLegendFrac  = 0.45 // ring side as a share of the width in the legend layout
	cycleRingMaxShrink   = 0.3  // share of its side the ring gives to the label columns when the rows need it
	cycleRingShrinkSteps = 6
	cycleRingLegendGapPt = 10.0 // ring to legend
	cycleRingLegendMinPt = 96.0 // narrowest legend column

	cycleRingStyleSegments = "segments"
	cycleRingStyleArrows   = "arrows"
	cycleRingLabelsOutside = "outside"
	cycleRingLabelsLegend  = "legend"
	cycleRingCCW           = "counter_clockwise"
)

var (
	cycleRingStyles      = []string{cycleRingStyleSegments, cycleRingStyleArrows}
	cycleRingDirections  = []string{"clockwise", cycleRingCCW}
	cycleRingThicknesses = []string{"thin", "regular", "thick"}
	cycleRingLabelModes  = []string{cycleRingLabelsOutside, cycleRingLabelsLegend}
)

// cycleRingThicknessFrac is the band width as a share of the ring's diameter.
func cycleRingThicknessFrac(name string) float64 {
	switch name {
	case "thin":
		return 0.14
	case "thick":
		return 0.28
	default:
		return ringDefaultThickness
	}
}

// Copy budgets per phase count (characters), measured on the smallest shipped
// content area (abstract, 687 x 294pt) at default sizes: text inside them is
// written at 12pt or above on every shipped template
// (TestCycleRingMeasuredBudgets). The schema maxima are the 4-phase budget; five
// phases put three label rows on one side, like six.
func cycleRingLabelBudget(n int) int {
	if n >= 7 {
		return 24
	}
	return cycleRingLabelMax
}

func cycleRingDescBudget(n int) int {
	switch {
	case n <= 4:
		return cycleRingDescMax
	case n <= 6:
		return 70
	default:
		return 50
	}
}

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (p *cycleRing) Schema() *Schema {
	phase := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(cycleRingLabelMax).WithDescription("Phase name, bold beside its number (e.g. \"Plan\"). Budget: 28 characters with 4-6 phases, 24 with 7-8"),
		"description": StringSchema(cycleRingDescMax).WithDescription("Optional muted line under the label. Budget by phase count: 4: 90 characters, 5-6: 70, 7-8: 50; expand_pattern reports BODY_TOO_LONG past it. Left off in the legend layout when its rows cannot hold it"),
		"highlight":   BooleanSchema().WithDescription("Fill this phase's segment with the solid accent (at most one phase); the others stay neutral"),
	}, []string{"label"}).WithAdditionalProperties(false)

	center := ObjectSchema(map[string]*Schema{
		"label":    StringSchema(cycleRingCenterMax).WithDescription("Short bold text in the ring's hole, e.g. \"Continuous improvement\" (short words: a word never breaks)"),
		"sublabel": StringSchema(cycleRingSublabelMax).WithDescription("Optional muted line under the centre label"),
	}, nil).WithAdditionalProperties(false)

	values := ObjectSchema(map[string]*Schema{
		"phases": ArraySchema(phase, cycleRingMinPhases, cycleRingMaxPhases).WithDescription("4-8 phases in order; phase 1's segment starts at 12 o'clock and the ring runs clockwise"),
		"center": center,
	}, []string{"phases"}).WithAdditionalProperties(false)

	overrides := textOverridesSchema()
	overrides.raw.Properties["header_size"] = NumberSchema(12, 28).WithDescription("Phase label size in points (default 14, stepping down to 12 when the rows need it)")
	overrides.raw.Properties["body_size"] = NumberSchema(12, 20).WithDescription("Description size in points (default 12)")
	overrides.raw.Properties["cell_accent_mode"] = EnumSchema("uniform", "alternate", "progressive").WithDescription("uniform (default): neutral segments, badges in the accent. alternate / progressive: each segment takes a light tint of its own accent").WithDefault("uniform")
	overrides.raw.Properties["style"] = EnumSchema(cycleRingStyles...).WithDescription("segments (default): ring segments. arrows: each phase is a curved arrow chasing the next").WithDefault(cycleRingStyleSegments)
	overrides.raw.Properties["direction"] = EnumSchema(cycleRingDirections...).WithDescription("Direction of travel from phase 1 at 12 o'clock").WithDefault("clockwise")
	overrides.raw.Properties["thickness"] = EnumSchema(cycleRingThicknesses...).WithDescription("Band width: thin, regular or thick (14% / 20% / 28% of the ring's diameter)").WithDefault("regular")
	overrides.raw.Properties["labels"] = EnumSchema(cycleRingLabelModes...).WithDescription("outside: each label beside its segment. legend: the ring on the left and one numbered list beside it. Default: outside, or legend when the content area is under about 450pt wide (a 50% or 60% compose segment)")

	return ObjectSchema(map[string]*Schema{
		"values":    values,
		"overrides": overrides,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Closed ring of 4-8 phase segments with numbered badges and outside labels")
}

func cycleRingOverrides(overrides any) *CycleRingOverrides {
	if o, ok := overrides.(*CycleRingOverrides); ok && o != nil {
		return o
	}
	return &CycleRingOverrides{}
}

func (p *cycleRing) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*CycleRingValues)
	if !ok || v == nil {
		return fmt.Errorf("cycle-ring: values must be *CycleRingValues, got %T", values)
	}
	const name = cycleRingName
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*CycleRingOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("cycle-ring: overrides must be *CycleRingOverrides, got %T", overrides))
		} else if ovr != nil {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			for _, e := range []struct {
				key, val string
				allowed  []string
			}{
				{"style", ovr.Style, cycleRingStyles},
				{"direction", ovr.Direction, cycleRingDirections},
				{"thickness", ovr.Thickness, cycleRingThicknesses},
				{"labels", ovr.Labels, cycleRingLabelModes},
			} {
				if e.val != "" && !slices.Contains(e.allowed, e.val) {
					errs = append(errs, errInvalidEnum(name, "overrides."+e.key, e.val, e.allowed))
				}
			}
		}
	}

	if n := len(v.Phases); n < cycleRingMinPhases {
		errs = append(errs, errMinItems(name, "phases", cycleRingMinPhases, n, "(hint: cycle-nodes holds a 3-phase cycle; use process-flow when the steps do not loop back)"))
	} else if n > cycleRingMaxPhases {
		errs = append(errs, errMaxItems(name, "phases", cycleRingMaxPhases, n, "(hint: merge related phases, split the cycle across two slides, or use cycle-figure-eight for two coupled loops)"))
	}
	highlights := 0
	for i, ph := range v.Phases {
		path := fmt.Sprintf("phases[%d].label", i)
		switch {
		case strings.TrimSpace(ph.Label) == "":
			errs = append(errs, errRequired(name, path))
		case runeLen(ph.Label) > cycleRingLabelMax:
			errs = append(errs, errMaxLength(name, path, cycleRingLabelMax, runeLen(ph.Label)))
		}
		if runeLen(ph.Description) > cycleRingDescMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].description", i), cycleRingDescMax, runeLen(ph.Description)))
		}
		if ph.Highlight {
			highlights++
		}
	}
	if highlights > 1 {
		errs = append(errs, newValidationError(name, "phases", ErrCodeOutOfRange,
			fmt.Sprintf("cycle-ring: %d phases set highlight; at most one phase is highlighted (the single solid accent segment)", highlights), nil))
	}
	if c := v.Center; c != nil {
		if runeLen(c.Label) > cycleRingCenterMax {
			errs = append(errs, errMaxLength(name, "center.label", cycleRingCenterMax, runeLen(c.Label)))
		}
		if runeLen(c.Sublabel) > cycleRingSublabelMax {
			errs = append(errs, errMaxLength(name, "center.sublabel", cycleRingSublabelMax, runeLen(c.Sublabel)))
		}
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"cycle-ring: cell_overrides are not supported (use overrides, or phases[].highlight for the one accent segment)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// cycleRingRow is one phase's label row, in points from the top-left corner
// of the pattern's block.
type cycleRingRow struct {
	side   string  // right | left: which side of the ring the label is on
	y0, y1 float64 // the row's box
	need   float64 // height its text needs
	capped bool    // the row is shorter than its text needs
	// The numeral cell is numX0 .. numX0 + ringNumberColPt and the label cell
	// labelX0 .. labelX1; together they are the row's label block.
	numX0, labelX0, labelX1 float64
}

// cycleRingLayout carries every measurement Expand and PostExpandWarnings
// share.
type cycleRingLayout struct {
	w, h         float64
	legend       bool
	side, x0, y0 float64 // the ring square
	spec         ringSpec
	items        []ringItem
	badgeDia     float64
	labelPt      float64
	descPt       float64
	labelW       float64 // room for a label cell (without its numeral) between the content edge and the figure's bounding box
	showDesc     bool    // false: the legend had no room for descriptions
	rows         []cycleRingRow
	centrePt     float64
	centreFits   bool
}

func (v *CycleRingValues) centre() (label, sublabel string) {
	if v.Center == nil {
		return "", ""
	}
	return strings.TrimSpace(v.Center.Label), strings.TrimSpace(v.Center.Sublabel)
}

func (v *CycleRingValues) hasDescriptions() bool {
	for _, ph := range v.Phases {
		if strings.TrimSpace(ph.Description) != "" {
			return true
		}
	}
	return false
}

// cycleRingMeasure resolves the geometry for the content area: the ring
// square, the label rows and the type sizes.
func cycleRingMeasure(ctx ExpandContext, v *CycleRingValues, ovr *CycleRingOverrides) (cycleRingLayout, error) {
	w, h := sizingAreaPt(ctx)
	base := cycleRingLayout{
		w: w, h: h,
		descPt:   shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, cycleRingDescPt)),
		showDesc: true,
	}
	// The label steps from its default down to the 12pt floor when the rows
	// do not fit; an authored header_size is kept.
	sizes := []float64{shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, cycleRingLabelPt))}
	if ovr.HeaderSize == 0 {
		sizes = append(sizes, shapegrid.MinTextSizePt)
	}

	base.legend = ovr.Labels == cycleRingLabelsLegend ||
		(ovr.Labels == "" && w-2*cycleRingReservePt < cycleRingOutsideSidePt)
	if base.legend {
		side := math.Min(h, math.Max(ringMinSidePt, w*cycleRingLegendFrac))
		side = math.Min(side, math.Max(w-cycleRingLegendGapPt-ringNumberColPt-cycleRingLegendMinPt, ringMinSidePt/2))
		lay, err := cycleRingPlace(ctx, v, ovr, base, side)
		if err != nil {
			return lay, err
		}
		lay.x0 = 0
		lay.labelW = math.Max(w-lay.side-ctx.Gap(cycleRingLegendGapPt)-ringNumberColPt, 1)
		cycleRingLegendRows(ctx, v, &lay, sizes)
		return lay, nil
	}

	reserve := math.Max(cycleRingReservePt, w*cycleRingReserveFrac)
	if w-2*reserve < cycleRingOutsideSidePt {
		reserve = cycleRingReservePt
	}
	// Labels on their own seldom need the whole column: the ring takes what
	// they leave (go-slide-creator-cxidm).
	if !v.hasDescriptions() {
		reserve = math.Min(reserve, cycleRingLabelReservePt(ctx, v, sizes[0], base.descPt, reserve))
	}
	full := math.Min(ringSquareSide(w, h, reserve, reserve), math.Max(w-2*(ringNumberColPt+cycleRingLegendMinPt), ringMinSidePt/2))
	// The ring takes the height it is given. When the label rows do not fit
	// beside it even at the 12pt floor, it gives up to cycleRingMaxShrink of
	// its side to the label columns before any row is cut short.
	var first cycleRingLayout
	for step := 0; step <= cycleRingShrinkSteps; step++ {
		side := full * (1 - cycleRingMaxShrink*float64(step)/cycleRingShrinkSteps)
		if step > 0 && side < cycleRingOutsideSidePt {
			break
		}
		lay, err := cycleRingPlace(ctx, v, ovr, base, side)
		if err != nil {
			return lay, err
		}
		lay.x0 = (w - lay.side) / 2
		lay.labelW = math.Max(lay.x0-ringNumberColPt, 1)
		if cycleRingOutsideRows(ctx, v, &lay, sizes) {
			return lay, nil
		}
		if step == 0 {
			first = lay
		}
	}
	// Nothing fits: keep the full ring; PostExpandWarnings names the phases
	// whose rows were cut.
	return first, nil
}

// cycleRingLabelReservePt is the label column a ring whose phases carry no
// description needs each side: the numeral, the gap to the ring and the width
// that sets the longest label on one line (with the slack a renderer's own
// face may take), at most maxPt.
func cycleRingLabelReservePt(ctx ExpandContext, v *CycleRingValues, labelPt, descPt, maxPt float64) float64 {
	fixed := ringNumberColPt + ctx.Gap(ringLabelGapPt)
	widest := cycleRingShortLabelPt
	for _, ph := range v.Phases {
		label := strings.TrimSpace(ph.Label)
		oneLine := ringLabelNeedPt(ctx, label, "", labelPt, descPt, maxPt*4)
		width := widest
		for width < maxPt && ringLabelNeedPt(ctx, label, "", labelPt, descPt, width) > oneLine {
			width += cycleRingReserveStepPt
		}
		widest = math.Max(widest, width)
	}
	return math.Min(math.Ceil(widest/ringSettleWidthShare)+fixed, maxPt)
}

// cycleRingPlace sets the ring square of the given side (centred vertically)
// and everything that follows from it: the band, the items, the badge and the
// centre label's size.
func cycleRingPlace(ctx ExpandContext, v *CycleRingValues, ovr *CycleRingOverrides, lay cycleRingLayout, side float64) (cycleRingLayout, error) {
	lay.side = math.Max(math.Min(side, math.Min(lay.w, lay.h)), 1)
	lay.y0 = (lay.h - lay.side) / 2

	spec := newRingSpec(len(v.Phases))
	spec.Clockwise = ovr.Direction != cycleRingCCW
	if ovr.Style == cycleRingStyleArrows {
		spec.GapDeg = cycleRingArrowGapDeg
	}
	spec = spec.withBand(1, ringBandFrac(cycleRingThicknessFrac(ovr.Thickness), lay.side))
	items, err := spec.items()
	if err != nil {
		return lay, fmt.Errorf("cycle-ring: %w", err)
	}
	lay.spec, lay.items = spec, ringSidesLR(items, spec.Clockwise)
	lay.badgeDia = ringBadgeDia(spec.Thickness, lay.side)

	label, sub := v.centre()
	lay.centrePt, lay.centreFits = ringCentreFit(ctx, spec, lay.side, label, sub)
	return lay, nil
}

// needs measures every phase's label row at the layout's sizes in a label
// cell widthPt wide.
func (l *cycleRingLayout) needs(ctx ExpandContext, v *CycleRingValues, widthPt float64) []float64 {
	out := make([]float64, len(v.Phases))
	for i := range v.Phases {
		out[i] = l.needAt(ctx, v, i, widthPt)
	}
	return out
}

// needAt measures phase i's label row at the layout's sizes in a label cell
// widthPt wide.
func (l *cycleRingLayout) needAt(ctx ExpandContext, v *CycleRingValues, i int, widthPt float64) float64 {
	ph := v.Phases[i]
	desc := ""
	if l.showDesc {
		desc = strings.TrimSpace(ph.Description)
	}
	return ringLabelNeedPt(ctx, strings.TrimSpace(ph.Label), desc, l.labelPt, l.descPt, widthPt)
}

// outsideRow is the label row of the outside layout for a placed ring row.
// Its block (numeral, then label) starts gapPt from the outer edge of the
// ring it belongs to, on the side it stands on, and its label cell runs to
// the content edge: rows near the top and the bottom of the ring start nearer
// the centre line than those at 3 and 9 o'clock. The figure is centred in the
// area, so the ring of a right-hand row mirrors the ring of a left-hand one
// (one ring for cycle-ring, the outer lobes of a figure eight).
func (l *cycleRingLayout) outsideRow(r ringRow, gapPt float64) cycleRingRow {
	y0 := math.Max(r.Y-r.H/2, 0)
	row := cycleRingRow{side: r.Side, y0: y0, y1: math.Min(y0+r.H, l.h)}
	cx := l.x0 + l.side/2
	if r.Side != ringSideLeft {
		cx = l.w - cx
	}
	edge := ringLabelEdgeX(cx, l.y0+l.side/2, l.spec.outerRadius()*l.side, row.y0, row.y1, gapPt, r.Side)
	if r.Side == ringSideLeft {
		row.numX0 = edge - ringNumberColPt
		row.labelX0, row.labelX1 = 0, row.numX0
	} else {
		row.numX0 = edge
		row.labelX0, row.labelX1 = edge+ringNumberColPt, l.w
	}
	return row
}

// cycleRingOutsideRows places a label row beside every badge: each side's
// rows as near their badges as the spacing allows. When a side cannot hold
// its rows at any label size they are cut to equal shares of the height (the
// writer then shrinks the text, and PostExpandWarnings names the phases).
// It reports whether every row got the height its text needs.
func cycleRingOutsideRows(ctx ExpandContext, v *CycleRingValues, lay *cycleRingLayout, sizes []float64) bool {
	rowsSpec := ringRowsSpec{
		CentreY:  lay.y0 + lay.side/2,
		RadiusPt: lay.spec.Radius * lay.side,
		GapPt:    ctx.Gap(cycleRingRowGapPt),
		Top:      0,
		Bottom:   lay.h,
	}
	labelGap := ctx.Gap(ringLabelGapPt)
	// The narrowest label cell stands beside 3 or 9 o'clock: every row holds
	// its text at that width, and a row nearer a pole is measured again at the
	// wider cell its place gives it.
	narrow := math.Max(lay.labelW-labelGap, 1)
	widthOf := func(r ringRow) float64 {
		row := lay.outsideRow(r, labelGap)
		return math.Max(row.labelX1-row.labelX0, 1)
	}
	needAt := func(i int, widthPt float64) float64 { return lay.needAt(ctx, v, i, widthPt) }
	var need []float64
	var rows []ringRow
	fits := false
	for _, size := range sizes {
		lay.labelPt = size
		base := lay.needs(ctx, v, narrow)
		for _, gap := range []float64{ctx.Gap(cycleRingRowGapPt), ctx.Gap(cycleRingMinRowGapPt)} {
			rowsSpec.GapPt, rowsSpec.Heights = gap, base
			if rows, need, fits = ringSettleRows(lay.items, rowsSpec, widthOf, needAt); fits {
				break
			}
		}
		if fits {
			break
		}
	}
	capped := make([]bool, len(need))
	if !fits {
		heights := append([]float64(nil), need...)
		for _, side := range []string{ringSideLeft, ringSideRight} {
			var idx []int
			for _, it := range lay.items {
				if it.Side == side {
					idx = append(idx, it.Index)
				}
			}
			cycleRingCapHeights(heights, idx, lay.h, rowsSpec.GapPt)
		}
		for i := range need {
			capped[i] = heights[i] < need[i]
		}
		rowsSpec.Heights = heights
		rows, _ = ringLabelRows(lay.items, rowsSpec)
	}
	lay.rows = make([]cycleRingRow, len(need))
	for _, r := range rows {
		row := lay.outsideRow(r, labelGap)
		row.need, row.capped = need[r.Index], capped[r.Index]
		lay.rows[r.Index] = row
	}
	return fits
}

// cycleRingCapHeights cuts the heights of the rows idx so that together with
// their gaps they fit total: the tallest rows give way first, down to one
// common cap (short rows keep their height).
func cycleRingCapHeights(heights []float64, idx []int, total, gap float64) {
	if len(idx) == 0 {
		return
	}
	avail := total - float64(len(idx)-1)*gap
	sum := 0.0
	for _, i := range idx {
		sum += heights[i]
	}
	if sum <= avail {
		return
	}
	sorted := append([]int(nil), idx...)
	slices.SortFunc(sorted, func(a, b int) int {
		switch {
		case heights[a] < heights[b]:
			return -1
		case heights[a] > heights[b]:
			return 1
		}
		return a - b
	})
	left := avail
	for k, i := range sorted {
		share := left / float64(len(sorted)-k)
		if heights[i] > share {
			// Every remaining row is at least this tall: they share what is left.
			for _, j := range sorted[k:] {
				heights[j] = math.Max(math.Floor(share*100)/100, 1)
			}
			return
		}
		left -= heights[i]
	}
}

// cycleRingLegendRows stacks one row per phase beside the ring, the block
// centred on the ring's height. Descriptions are left off when the rows do
// not hold them at the 12pt floor.
func cycleRingLegendRows(ctx ExpandContext, v *CycleRingValues, lay *cycleRingLayout, sizes []float64) {
	n := len(v.Phases)
	gaps := []float64{ctx.Gap(cycleRingRowGapPt), ctx.Gap(cycleRingMinRowGapPt)}
	var need []float64
	gap, total := gaps[0], 0.0
	fit := func() bool {
		for _, size := range sizes {
			lay.labelPt = size
			need = lay.needs(ctx, v, lay.labelW)
			sum := 0.0
			for _, x := range need {
				sum += x
			}
			for _, g := range gaps {
				gap, total = g, sum+float64(n-1)*g
				if total <= lay.h {
					return true
				}
			}
		}
		return false
	}
	fits := fit()
	if !fits && v.hasDescriptions() {
		lay.showDesc = false
		fits = fit()
	}
	heights := append([]float64(nil), need...)
	if !fits {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		cycleRingCapHeights(heights, idx, lay.h, gap)
		total = float64(n-1) * gap
		for _, x := range heights {
			total += x
		}
	}
	y := math.Max((lay.h-total)/2, 0)
	lay.rows = make([]cycleRingRow, n)
	for i := range lay.rows {
		lay.rows[i] = cycleRingRow{side: ringSideRight, y0: y, y1: math.Min(y+heights[i], lay.h), need: need[i], capped: heights[i] < need[i],
			numX0: lay.w - lay.labelW - ringNumberColPt, labelX0: lay.w - lay.labelW, labelX1: lay.w}
		y += heights[i] + gap
	}
}

// ---------------------------------------------------------------------------
// Warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports copy over the per-count budget, a label row that
// cannot hold its text on this template, descriptions the legend layout left
// off and a centre label that does not fit the hole.
func (p *cycleRing) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*CycleRingValues)
	if !ok || v == nil || len(v.Phases) < cycleRingMinPhases || len(v.Phases) > cycleRingMaxPhases {
		return nil
	}
	lay, err := cycleRingMeasure(ctx, v, cycleRingOverrides(overrides))
	if err != nil {
		return nil
	}
	n := len(v.Phases)
	labelBudget, descBudget := cycleRingLabelBudget(n), cycleRingDescBudget(n)
	var warnings []string
	for i, ph := range v.Phases {
		named := false
		if c := runeLen(ph.Label); c > labelBudget {
			named = true
			warnings = append(warnings, fmt.Sprintf("%s: cycle-ring phases[%d].label is %d characters; with %d phases use about %d — shorten the label or use fewer phases",
				ErrCodeBodyTooLong, i, c, n, labelBudget))
		}
		if c := runeLen(ph.Description); c > descBudget && lay.showDesc {
			named = true
			warnings = append(warnings, fmt.Sprintf("%s: cycle-ring phases[%d].description is %d characters; with %d phases use about %d — shorten the description or use fewer phases",
				ErrCodeBodyTooLong, i, c, n, descBudget))
		}
		if row := lay.rows[i]; row.capped && !named {
			field := "label"
			if lay.showDesc && strings.TrimSpace(ph.Description) != "" {
				field = "description"
			}
			warnings = append(warnings, fmt.Sprintf("%s: cycle-ring phases[%d].%s needs %.0fpt but its label row is %.0fpt tall in a %.0f x %.0fpt content area with %d phases — shorten it, drop the descriptions or use fewer phases",
				ErrCodeBodyTooLong, i, field, row.need, row.y1-row.y0, lay.w, lay.h, n))
		}
	}
	if !lay.showDesc {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-ring phases[].description is left off: the legend layout in a %.0f x %.0fpt content area holds the %d labels only — give the ring the full slide width or drop the descriptions",
			ErrCodeBodyTooLong, lay.w, lay.h, n))
	}
	if label, sub := v.centre(); (label != "" || sub != "") && !lay.centreFits {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-ring center does not fit the ring's %.0fpt hole at 12pt (a word breaks or the text runs past it) — shorten center.label, drop center.sublabel or use overrides.thickness thin",
			ErrCodeBodyTooLong, 2*(lay.spec.Radius-lay.spec.Thickness/2)*lay.side))
	}
	return warnings
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *cycleRing) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*CycleRingValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("cycle-ring: values must be *CycleRingValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*CycleRingOverrides); !ok {
			return nil, fmt.Errorf("cycle-ring: overrides must be *CycleRingOverrides, got %T", overrides)
		}
	}
	if n := len(v.Phases); n < cycleRingMinPhases || n > cycleRingMaxPhases {
		return nil, fmt.Errorf("cycle-ring: %d phases; the ring holds %d-%d", n, cycleRingMinPhases, cycleRingMaxPhases)
	}
	ovr := cycleRingOverrides(overrides)
	lay, err := cycleRingMeasure(ctx, v, ovr)
	if err != nil {
		return nil, err
	}

	base := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	paint := ringPaint{
		Arrows:    ovr.Style == cycleRingStyleArrows,
		Accents:   make([]string, len(v.Phases)),
		Tinted:    ovr.CellAccentMode == "alternate" || ovr.CellAccentMode == "progressive",
		Highlight: -1,
		BadgeDia:  lay.badgeDia,
	}
	for i, ph := range v.Phases {
		paint.Accents[i] = ctx.ResolveCellAccent(base, i, ovr.CellAccentMode)
		if ph.Highlight && paint.Highlight < 0 {
			paint.Highlight = i
		}
	}

	layers := [][]jsonschema.LayerInput{
		ringSegmentLayers(ctx, lay.spec, lay.items, paint),
		ringBadgeLayers(ctx, lay.spec, lay.items, paint),
	}
	if label, sub := v.centre(); label != "" || sub != "" {
		if centre, ok := ringCentreLayer(lay.spec, label, sub, lay.centrePt, ""); ok {
			layers = append(layers, []jsonschema.LayerInput{centre})
		}
	}
	// Outside labels follow the ring into its bounding square, so the ring
	// cell is the spine column there; the legend keeps the square.
	places := []ringPlacement{{X0: lay.x0, X1: lay.x0 + lay.side, Y0: lay.y0, Y1: lay.y0 + lay.side, Cell: ringCell(layers...)}}
	if !lay.legend {
		places[0] = ringSpinePlacement(lay.x0+lay.side/2, lay.y0, lay.side, places[0].Cell)
	}

	places = append(places, lay.labelPlaces(ctx, v.Phases, paint.Accents)...)

	grid, err := ringLattice(places, lay.w, lay.h)
	if err != nil {
		return nil, fmt.Errorf("cycle-ring: %w", err)
	}
	grid.VerticalAlign = "middle"
	return grid, nil
}

// labelPlaces is the numeral and label cell of every phase on its row: right
// of the figure the numeral, then the label; left of it, mirrored. The rows
// carry their own x (outsideRow: a constant gap from the ring's outer edge at
// the row's height; the legend: one column).
// cycle-figure-eight places the labels of its two lobes with it too.
func (l *cycleRingLayout) labelPlaces(ctx ExpandContext, phases []CycleRingPhase, accents []string) []ringPlacement {
	places := make([]ringPlacement, 0, 2*len(phases))
	gapEdges := make([]float64, 0, len(phases))
	for i, ph := range phases {
		row := l.rows[i]
		desc := ""
		if l.showDesc {
			desc = strings.TrimSpace(ph.Description)
		}
		align := "l"
		gapEdges = append(gapEdges, row.numX0)
		if row.side == ringSideLeft {
			align = "r"
			gapEdges[i] = row.numX0 + ringNumberColPt
		}
		places = append(places,
			ringPlacement{X0: row.numX0, X1: row.numX0 + ringNumberColPt, Y0: row.y0, Y1: row.y1,
				Cell: ringNumberCell(ctx, i+1, accents[i], l.labelPt, align)},
			ringPlacement{X0: row.labelX0, X1: row.labelX1, Y0: row.y0, Y1: row.y1,
				Cell: ringLabelTextCell(ringLabelText(strings.TrimSpace(ph.Label), desc, l.labelPt, l.descPt, align))},
		)
	}
	if !l.legend {
		ringProtectEdges(places, gapEdges)
	}
	return places
}
