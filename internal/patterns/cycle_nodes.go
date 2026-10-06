package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// cycle-nodes pattern — 3-8 numbered circles on a ring joined by curved arrows
// ---------------------------------------------------------------------------
//
// A discrete-step loop (plan → do → check → act): the steps are separate
// circles on a ring and the transitions are explicit arrows between them
// (SmartArt "Basic Cycle"). The ring is ONE shape-grid cell with fit
// "contain", so it stays round wherever it lands, and every node, arrow and
// the optional centre label is a layer of that cell (ring_nodes_draw.go).
//
// Labels stand outside the ring in two clean columns, each row beside its
// node's height and led by the step number that the node carries: the number
// is the cue that ties a label to its circle. Rows are lattice cells
// (ringLattice), so nothing can overlap whatever size the grid resolves at.
// In an area too narrow for two label columns (a compose half) the labels move
// to one legend column beside the ring; overrides.labels "inside" puts short
// labels in the nodes themselves.

func init() {
	Default().Register(&cycleNodes{})
}

type cycleNodes struct{}

func (c *cycleNodes) Name() string { return "cycle-nodes" }
func (c *cycleNodes) Description() string {
	return "Recurring loop of 3-8 numbered circles on a ring joined by curved arrows, each step labelled outside the ring (label + optional description), with an optional centre label and one highlighted step"
}
func (c *cycleNodes) UseWhen() string {
	return "A closed loop of 3-8 discrete steps that returns to its start (plan-do-check-act, sense-decide-act-learn, a feedback or continuous-improvement cycle) where the steps and the hand-offs between them are the message; prefer cycle-ring when the phases form one continuous filled ring, cycle-intake when linear steps feed the loop, and numbered-step-strip or process-flow when the sequence does not return to the start"
}
func (c *cycleNodes) NotWhen() string {
	return "The phases are one continuous ring of segments (use cycle-ring), linear intake steps feed the loop (use cycle-intake), two loops share a crossing point (use cycle-figure-eight), items surround a hub without an order (use radial-hub), the levels nest (use concentric-rings), or the sequence ends instead of returning to its start (use numbered-step-strip or process-flow)"
}
func (c *cycleNodes) Version() int      { return 1 }
func (c *cycleNodes) CellsHint() string { return "3-8 nodes + labels (+ centre)" }
func (c *cycleNodes) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"numbered-step-strip", "kpi-3up", "next-steps"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}
func (c *cycleNodes) SupportsInlineMarkdown() bool { return true }

func (c *cycleNodes) ExemplarValues() any {
	return &CycleNodesValues{
		Steps: []CycleNodesStep{
			{Label: "Plan", Description: "Set the target and the test for it"},
			{Label: "Do", Description: "Run the change on one line first"},
			{Label: "Check", Description: "Compare the result with the target"},
			{Label: "Act", Description: "Standardise what worked, then repeat"},
		},
		Highlight: 3,
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CycleNodesStep is one step of the loop.
type CycleNodesStep struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// CycleNodesCenter is the optional label in the middle of the ring.
type CycleNodesCenter struct {
	Label string `json:"label"`
}

// CycleNodesValues holds the 3-8 steps, the optional centre label and the
// 1-based number of the highlighted step (0 = none).
type CycleNodesValues struct {
	Steps     []CycleNodesStep  `json:"steps"`
	Center    *CycleNodesCenter `json:"center,omitempty"`
	Highlight int               `json:"highlight,omitempty"`
}

// CycleNodesOverrides is the standard text overrides (header_size sizes the
// step labels, body_size the descriptions) plus the loop's own switches.
type CycleNodesOverrides struct {
	TextOverrides
	Direction string `json:"direction,omitempty"` // clockwise (default) | counter_clockwise
	Labels    string `json:"labels,omitempty"`    // outside (default) | inside | legend
	Arrows    string `json:"arrows,omitempty"`    // curved (default) | none
}

func (c *cycleNodes) NewValues() any       { return &CycleNodesValues{} }
func (c *cycleNodes) NewOverrides() any    { return &CycleNodesOverrides{} }
func (c *cycleNodes) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	cycleNodesName = "cycle-nodes"

	cycleNodesLabelMax  = 28
	cycleNodesDescMax   = 70
	cycleNodesCenterMax = 24

	// Inside labels: the node carries the label, so it is short and the ring
	// holds at most five nodes.
	cycleNodesInsideMaxSteps = 5
	cycleNodesInsideLabelMax = 14
	cycleNodesInsideLines    = 2

	cycleNodesLabelsOutside = "outside"
	cycleNodesLabelsInside  = "inside"
	cycleNodesLabelsLegend  = "legend"
	cycleNodesArrowsCurved  = "curved"
	cycleNodesArrowsNone    = "none"
	cycleNodesClockwise     = "clockwise"
	cycleNodesCounter       = "counter_clockwise"

	cycleNodesLabelPt = scaleSubheadPt
	cycleNodesBodyPt  = scaleBodyPt
	cycleNodesFloorPt = scaleBodyPt

	// The ring square takes at most this share of the width, so the two label
	// columns keep 30% each; the legend layout gives the ring a little more.
	cycleNodesRingWidthFrac = 0.40
	// cycleNodesMinLabelColPt is the narrowest label text column the outside
	// layout keeps: under it the labels move to one legend column.
	cycleNodesMinLabelColPt = 120.0
	// cycleNodesMaxLabelColPt caps a label column so its text stays beside the
	// ring on a wide slide.
	cycleNodesMaxLabelColPt = 260.0
	// cycleNodesMaxLegendColPt caps the one legend column: a list beside the
	// ring reads at a longer line than a label hugging its node.
	cycleNodesMaxLegendColPt = 420.0
	cycleNodesRingGapPt      = 12.0 // ring square edge to the number cue
	cycleNodesCueColPt       = 22.0 // number cue column: the numeral plus its gap to the label
	cycleNodesRowGapPt       = 6.0
	cycleNodesMinRowGapPt    = 2.0 // what the row gap gives way to before text shrinks
	cycleNodesLabelInsetPt   = 2.0 // text margin of the unfilled label frames
	cycleNodesNodeInsetPt    = 2.0 // text margin of a node that carries its label
	cycleNodesLabelSpacePt   = 1.0 // space after a label above its description
	cycleNodesRowSlackPt     = 1.0

	// Inside labels use larger nodes; the number moves to a small badge on the
	// node's inner edge.
	cycleNodesInsideDia3    = 0.34
	cycleNodesInsideDia4    = 0.32
	cycleNodesInsideDia5    = 0.30
	cycleNodesBadgeFrac     = 0.085
	cycleNodesBadgeMinPt    = 24.0
	cycleNodesCentreMargin  = 0.03
	cycleNodesCentreInsetPt = 2.0
	cycleNodesCentreLines   = 3

	// cycleNodesNumeralShare is the largest numeral as a share of its node's
	// diameter.
	cycleNodesNumeralShare = 0.40
	cycleNodesInkContrast  = 4.5
)

// cycleNodesLegendWidthFracs are the shares of the width the ring takes in
// the legend layout, largest first: the ring gives width to the legend column
// when its rows do not fit.
var cycleNodesLegendWidthFracs = [...]float64{0.48, 0.40, 0.33}

var (
	cycleNodesLabelModes = []string{cycleNodesLabelsOutside, cycleNodesLabelsInside, cycleNodesLabelsLegend}
	cycleNodesArrowModes = []string{cycleNodesArrowsCurved, cycleNodesArrowsNone}
	cycleNodesDirections = []string{cycleNodesClockwise, cycleNodesCounter}
)

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (c *cycleNodes) Schema() *Schema {
	step := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(cycleNodesLabelMax).WithDescription("Step name, e.g. \"Plan\". Readable budget: about 28 characters with 3-6 steps, 22 with 7-8; 14 (two short words) with labels \"inside\""),
		"description": StringSchema(cycleNodesDescMax).WithDescription("Optional one-sentence detail under the label. Readable budget: about 70 characters with 3-6 steps, 40 with 7-8 (in a compose half about 50 with 3-5 steps, 20 with 6-7, none with 8); BODY_TOO_LONG names the step that outgrows its row. Not drawn with labels \"inside\""),
	}, []string{"label"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"steps": ArraySchema(RefSchema("step"), ringMinItems, ringMaxItems).WithDescription("3-8 steps in loop order; step 1 sits at 12 o'clock and the loop returns from the last step to the first"),
		"center": ObjectSchema(map[string]*Schema{
			"label": StringSchema(cycleNodesCenterMax).WithDescription("Short name of the loop set in the middle of the ring, e.g. \"Continuous improvement\""),
		}, []string{"label"}).WithAdditionalProperties(false).WithDescription("Optional centre label"),
		"highlight": IntegerSchema(0, ringMaxItems).WithDescription("1-based number of the step to highlight: its node is the one solid accent circle (0 or omitted = none)"),
	}, []string{"steps"}).WithAdditionalProperties(false)

	overridesSchema := textOverridesSchema()
	overridesSchema.raw.Properties["header_size"] = NumberSchema(12, 28).WithDescription("Step label font size in points (default 14; steps down to 12 when the rows need it)")
	overridesSchema.raw.Properties["body_size"] = NumberSchema(12, 20).WithDescription("Description font size in points (default 12)")
	overridesSchema.raw.Properties["direction"] = EnumSchema(cycleNodesDirections...).WithDescription("Direction of travel from step 1 (default clockwise)").WithDefault(cycleNodesClockwise)
	overridesSchema.raw.Properties["labels"] = EnumSchema(cycleNodesLabelModes...).WithDescription("outside (default): label rows in a column either side of the ring, each led by its step number; an area too narrow for two columns takes legend by itself. legend: one numbered list beside the ring. inside: the node carries the label and the number moves to a small badge (3-5 steps, labels of at most 14 characters, no descriptions)").WithDefault(cycleNodesLabelsOutside)
	overridesSchema.raw.Properties["arrows"] = EnumSchema(cycleNodesArrowModes...).WithDescription("curved (default): an arrow from each node to the next. none: a nondirectional cycle with plain gaps").WithDefault(cycleNodesArrowsCurved)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"step": step,
	}).WithDescription("Loop of 3-8 numbered circles on a ring joined by curved arrows, labels outside the ring")
}

func cycleNodesOverrides(overrides any) *CycleNodesOverrides {
	if o, ok := overrides.(*CycleNodesOverrides); ok && o != nil {
		return o
	}
	return &CycleNodesOverrides{}
}

func (c *cycleNodes) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*CycleNodesValues)
	if !ok || v == nil {
		return fmt.Errorf("cycle-nodes: values must be *CycleNodesValues, got %T", values)
	}
	const name = cycleNodesName
	var errs []error

	if overrides != nil {
		if _, ok := overrides.(*CycleNodesOverrides); !ok {
			errs = append(errs, fmt.Errorf("cycle-nodes: overrides must be *CycleNodesOverrides, got %T", overrides))
		}
	}
	ovr := cycleNodesOverrides(overrides)
	errs = append(errs, validateCycleNodesOverrides(ovr)...)
	errs = append(errs, validateCycleNodesSteps(v.Steps, ovr.Labels == cycleNodesLabelsInside)...)

	n := len(v.Steps)
	if v.Center != nil {
		switch l := runeLen(v.Center.Label); {
		case strings.TrimSpace(v.Center.Label) == "":
			errs = append(errs, errRequired(name, "center.label"))
		case l > cycleNodesCenterMax:
			errs = append(errs, errMaxLength(name, "center.label", cycleNodesCenterMax, l))
		}
	}
	if v.Highlight < 0 || (n >= ringMinItems && v.Highlight > n) {
		errs = append(errs, newValidationError(name, "highlight", ErrCodeInvalidShape,
			fmt.Sprintf("cycle-nodes: highlight is %d; it is the 1-based number of a step (1-%d), or 0 for none", v.Highlight, n), nil))
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"cycle-nodes: cell_overrides are not supported (use overrides, and values.highlight for the emphasised step)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// validateCycleNodesOverrides checks the accent mode and the loop's enums.
func validateCycleNodesOverrides(ovr *CycleNodesOverrides) []error {
	const name = cycleNodesName
	var errs []error
	if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}
	for _, e := range []struct {
		path, got string
		allowed   []string
	}{
		{"overrides.direction", ovr.Direction, cycleNodesDirections},
		{"overrides.labels", ovr.Labels, cycleNodesLabelModes},
		{"overrides.arrows", ovr.Arrows, cycleNodesArrowModes},
	} {
		if e.got != "" && !slices.Contains(e.allowed, e.got) {
			errs = append(errs, errInvalidEnum(name, e.path, e.got, e.allowed))
		}
	}
	return errs
}

// validateCycleNodesSteps checks the step count and every step's text;
// inside is true when the nodes carry the labels.
func validateCycleNodesSteps(steps []CycleNodesStep, inside bool) []error {
	const name = cycleNodesName
	var errs []error
	n := len(steps)
	if n < ringMinItems {
		errs = append(errs, errMinItems(name, "steps", ringMinItems, n, "(hint: a loop needs at least 3 steps; use before-after or comparison-2col for two states)"))
	}
	if n > ringMaxItems {
		errs = append(errs, errMaxItems(name, "steps", ringMaxItems, n, "(hint: merge related steps, or use numbered-step-strip / process-flow for a long sequence)"))
	}
	if inside && n > cycleNodesInsideMaxSteps {
		errs = append(errs, newValidationError(name, "overrides.labels", ErrCodeInvalidShape,
			fmt.Sprintf("cycle-nodes: labels \"inside\" holds at most %d steps (got %d): the nodes of a larger ring are too small for a label — use labels \"outside\"", cycleNodesInsideMaxSteps, n),
			RemoveFieldFix("overrides.labels")))
	}
	for i, s := range steps {
		labelPath := fmt.Sprintf("steps[%d].label", i)
		switch l := runeLen(s.Label); {
		case strings.TrimSpace(s.Label) == "":
			errs = append(errs, errRequired(name, labelPath))
		case l > cycleNodesLabelMax:
			errs = append(errs, errMaxLength(name, labelPath, cycleNodesLabelMax, l))
		case inside && l > cycleNodesInsideLabelMax:
			errs = append(errs, newValidationError(name, labelPath, ErrCodeMaxLength,
				fmt.Sprintf("cycle-nodes: %s is %d characters; a label set inside its node holds at most %d — shorten it or use labels \"outside\"", labelPath, l, cycleNodesInsideLabelMax),
				RemoveFieldFix("overrides.labels")))
		}
		descPath := fmt.Sprintf("steps[%d].description", i)
		switch l := runeLen(s.Description); {
		case l > cycleNodesDescMax:
			errs = append(errs, errMaxLength(name, descPath, cycleNodesDescMax, l))
		case inside && strings.TrimSpace(s.Description) != "":
			errs = append(errs, newValidationError(name, descPath, ErrCodeInvalidShape,
				fmt.Sprintf("cycle-nodes: %s is set but labels \"inside\" draws no descriptions — remove it or use labels \"outside\"", descPath),
				RemoveFieldFix(descPath)))
		}
	}
	return errs
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// cycleNodesRow is the label row of one step, in points from the top-left
// corner of the pattern's block.
type cycleNodesRow struct {
	side         string  // ringSideLeft | ringSideRight: where the row stands
	textX0       float64 // label text frame
	textX1       float64
	cueX0, cueX1 float64 // number cue
	y0, y1       float64
	need         float64 // written height of the label at textX1 − textX0
}

// cycleNodesLayout carries every measurement Expand and PostExpandWarnings
// share.
type cycleNodesLayout struct {
	mode       string // resolved labels mode
	w, h       float64
	side       float64 // ring square
	ringX      float64
	ringY      float64
	spec       ringSpec
	nodes      []ringNode
	dia        float64 // node diameter, fraction of the square
	labelSize  float64
	bodySize   float64
	numeralPt  float64
	textW      float64 // label text frame width
	rows       []cycleNodesRow
	rowsFit    bool
	rowGap     float64            // gap between label rows the layout settled on
	roomPt     map[string]float64 // height a side offers its rows
	needPt     map[string]float64 // height the rows of a side need
	nodeFits   []bool             // inside: label i fits its node
	nodePt     float64            // inside: label size in the nodes
	badgeDia   float64            // inside: number badge, fraction of the square
	centreFr   ringFrame
	centrePt   float64
	centreFits bool
}

// cycleNodesMode resolves the labels mode: an authored mode stands; the
// default is outside, or legend when two label columns would be narrower than
// cycleNodesMinLabelColPt.
func cycleNodesMode(ovr *CycleNodesOverrides, w, h float64) string {
	if ovr.Labels != "" {
		return ovr.Labels
	}
	side := cycleNodesRingSide(w, h, cycleNodesRingWidthFrac)
	if (w-side)/2-cycleNodesRingGapPt-cycleNodesCueColPt < cycleNodesMinLabelColPt {
		return cycleNodesLabelsLegend
	}
	return cycleNodesLabelsOutside
}

// cycleNodesRingSide is the ring square's side in a w × h area when it may
// take widthFrac of the width: never under ringMinSidePt unless the area
// itself is smaller.
func cycleNodesRingSide(w, h, widthFrac float64) float64 {
	side := math.Min(h, w*widthFrac)
	if side < ringMinSidePt {
		side = math.Min(ringMinSidePt, math.Min(w, h))
	}
	return math.Max(side, 1)
}

func cycleNodesInsideDia(n int) float64 {
	switch {
	case n <= 3:
		return cycleNodesInsideDia3
	case n == 4:
		return cycleNodesInsideDia4
	default:
		return cycleNodesInsideDia5
	}
}

// cycleNodesNumeralPt is the numeral size for a node diaPt across: the largest
// step that stays under cycleNodesNumeralShare of it, never under the floor.
func cycleNodesNumeralPt(diaPt float64) float64 {
	for _, s := range [...]float64{sizeFigurePt, scaleLeadPt, scaleSubheadPt} {
		if s <= diaPt*cycleNodesNumeralShare {
			return s
		}
	}
	return cycleNodesFloorPt
}

// cycleNodesMeasure resolves the geometry: the ring square, its nodes, and
// the label rows sized to the WRITTEN height of their text. The label size
// steps from 14pt to the 12pt floor before any row is left to the writer's
// autofit shrink; PostExpandWarnings names the steps that still do not fit.
func cycleNodesMeasure(ctx ExpandContext, v *CycleNodesValues, ovr *CycleNodesOverrides) (cycleNodesLayout, error) {
	w, h := sizingAreaPt(ctx)
	n := len(v.Steps)
	lay := cycleNodesLayout{
		mode:     cycleNodesMode(ovr, w, h),
		w:        w,
		h:        h,
		bodySize: shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, cycleNodesBodyPt)),
		rowsFit:  true,
	}
	clockwise := ovr.Direction != cycleNodesCounter

	lay.dia = ringNodeDiameter(n)
	if lay.mode == cycleNodesLabelsInside {
		lay.dia = cycleNodesInsideDia(n)
	}
	lay.spec = newRingNodeSpec(n, lay.dia, clockwise)
	nodes, err := lay.spec.nodes(lay.dia, ringNodeClearDeg)
	if err != nil {
		return lay, fmt.Errorf("cycle-nodes: %w", err)
	}
	lay.nodes = nodes

	// Label rows are sized to their written height. When they do not fit, the
	// layout gives way in order — the row gap (inside placeRows), the label
	// size (14pt to the 12pt floor), then, in the legend, the ring's share of
	// the width — before any row is left to the writer's autofit shrink.
	lay.labelSize = shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, cycleNodesLabelPt))
	sizes := []float64{lay.labelSize}
	if ovr.HeaderSize == 0 && lay.labelSize > cycleNodesFloorPt {
		sizes = append(sizes, cycleNodesFloorPt)
	}
	fracs := []float64{cycleNodesRingWidthFrac}
	if lay.mode == cycleNodesLabelsLegend {
		fracs = cycleNodesLegendWidthFracs[:]
	}
	best, bestOver := lay, math.Inf(1)
	for _, frac := range fracs {
		for _, size := range sizes {
			lay.labelSize = size
			lay.arrange(frac)
			if lay.mode != cycleNodesLabelsInside {
				lay.placeRows(ctx, v)
			}
			if over := lay.overflow(); over < bestOver-1e-9 {
				best, bestOver = lay, over
			}
			if lay.rowsFit {
				break
			}
		}
		if lay.rowsFit {
			break
		}
	}
	// Nothing fits: keep the arrangement whose rows overflow least;
	// PostExpandWarnings names the steps that outgrow their row.
	lay = best
	diaPt := lay.dia * lay.side
	lay.numeralPt = cycleNodesNumeralPt(diaPt)

	inward := lay.dia / 2
	if lay.mode == cycleNodesLabelsInside {
		lay.badgeDia = math.Min(math.Max(cycleNodesBadgeFrac, cycleNodesBadgeMinPt/lay.side), lay.dia/2)
		inward += lay.badgeDia / 2
		// One size for every node: the subhead step when all labels hold
		// there, else the floor, where nodeFits says which do not.
		for _, size := range []float64{scaleSubheadPt, cycleNodesFloorPt} {
			lay.nodePt, lay.nodeFits = size, make([]bool, n)
			all := true
			for i, s := range v.Steps {
				_, lay.nodeFits[i] = ringFitCircleLabel(ctx, cycleNodesPlain(s.Label), []float64{size}, diaPt, cycleNodesNodeInsetPt, cycleNodesInsideLines)
				all = all && lay.nodeFits[i]
			}
			if all {
				break
			}
		}
	}
	lay.centreFr = ringNodeCentreFrame(lay.spec, inward, cycleNodesCentreMargin)
	lay.centrePt, lay.centreFits = cycleNodesFloorPt, true
	if v.Center != nil && strings.TrimSpace(v.Center.Label) != "" {
		lay.centrePt, lay.centreFits = ringFitCircleLabel(ctx, cycleNodesPlain(v.Center.Label),
			[]float64{scaleSubheadPt, scaleBodyPt}, lay.centreFr.W*lay.side, cycleNodesCentreInsetPt, cycleNodesCentreLines)
	}

	return lay, nil
}

// overflow is how far the fullest side's rows outgrow the height they are
// given, as a ratio (1 or less = they fit).
func (l *cycleNodesLayout) overflow() float64 {
	over := 0.0
	for side, need := range l.needPt {
		if room := l.roomPt[side]; room > 0 {
			over = math.Max(over, need/room)
		}
	}
	return over
}

// arrange places the ring square for the layout's mode: alone and centred
// (inside), centred between two label columns (outside), or left of one
// legend column with which it is centred as a block. widthFrac is the share
// of the width the ring may take in the outside and legend layouts.
func (l *cycleNodesLayout) arrange(widthFrac float64) {
	switch l.mode {
	case cycleNodesLabelsInside:
		l.side = math.Max(math.Min(l.w, l.h), 1)
		l.ringX = (l.w - l.side) / 2
	case cycleNodesLabelsLegend:
		l.side = cycleNodesRingSide(l.w, l.h, widthFrac)
		l.textW = math.Min(l.w-l.side-cycleNodesRingGapPt-cycleNodesCueColPt, cycleNodesMaxLegendColPt)
		l.textW = math.Max(l.textW, 1)
		l.ringX = math.Max((l.w-l.side-cycleNodesRingGapPt-cycleNodesCueColPt-l.textW)/2, 0)
	default:
		l.side = cycleNodesRingSide(l.w, l.h, widthFrac)
		l.ringX = (l.w - l.side) / 2
		l.textW = math.Min(l.ringX-cycleNodesRingGapPt-cycleNodesCueColPt, cycleNodesMaxLabelColPt)
		l.textW = math.Max(l.textW, 1)
	}
	l.ringY = (l.h - l.side) / 2
}

// placeRows measures every label at the current sizes and places its row:
// beside its node's height in the outside layout, stacked in step order in the
// legend.
func (l *cycleNodesLayout) placeRows(ctx ExpandContext, v *CycleNodesValues) {
	n := len(v.Steps)
	heights := make([]float64, n)
	for i, s := range v.Steps {
		heights[i] = cycleNodesRowNeedPt(ctx, s, *l)
	}
	// The gap between rows gives way before any text does.
	l.placeRowsAt(heights, ctx.Gap(cycleNodesRowGapPt))
	if !l.rowsFit {
		l.placeRowsAt(heights, ctx.Gap(cycleNodesMinRowGapPt))
	}
}

// placeRowsAt places the rows of the given written heights gap apart.
func (l *cycleNodesLayout) placeRowsAt(heights []float64, gap float64) {
	n := len(heights)
	l.rowGap = gap
	l.rows = make([]cycleNodesRow, n)
	l.roomPt = map[string]float64{}
	l.needPt = map[string]float64{}
	l.rowsFit = true

	// stack lays the given steps out top to bottom, centred in the block; rows
	// that are taller than the block together are scaled down to it (the
	// writer then shrinks their text, and PostExpandWarnings says which).
	fitHeights := func(idx []int) []float64 {
		need := float64(len(idx)-1) * gap
		for _, i := range idx {
			need += heights[i]
		}
		side := l.rows[idx[0]].side
		l.needPt[side], l.roomPt[side] = need, l.h
		out := make([]float64, n)
		copy(out, heights)
		if need > l.h {
			l.rowsFit = false
			scale := math.Max(l.h-float64(len(idx)-1)*gap, 1) / (need - float64(len(idx)-1)*gap)
			for _, i := range idx {
				out[i] = math.Floor(heights[i] * scale)
			}
		}
		return out
	}

	if l.mode == cycleNodesLabelsLegend {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
			l.rows[i].side = ringSideRight
		}
		hs := fitHeights(idx)
		total := float64(n-1) * gap
		for _, i := range idx {
			total += hs[i]
		}
		y := math.Max((l.h-total)/2, 0)
		cue := l.ringX + l.side + cycleNodesRingGapPt
		for _, i := range idx {
			l.rows[i] = cycleNodesRow{side: ringSideRight, cueX0: cue, cueX1: cue + cycleNodesCueColPt,
				textX0: cue + cycleNodesCueColPt, textX1: cue + cycleNodesCueColPt + l.textW,
				y0: y, y1: y + hs[i], need: heights[i]}
			y += hs[i] + gap
		}
		return
	}

	items := ringSidesLR(ringNodeItems(l.nodes), l.spec.Clockwise)
	bySide := map[string][]int{}
	for _, it := range items {
		l.rows[it.Index].side = it.Side
		bySide[it.Side] = append(bySide[it.Side], it.Index)
	}
	hs := make([]float64, n)
	copy(hs, heights)
	for _, side := range []string{ringSideLeft, ringSideRight} {
		if idx := bySide[side]; len(idx) > 0 {
			fitted := fitHeights(idx)
			for _, i := range idx {
				hs[i] = fitted[i]
			}
		}
	}
	placed, _ := ringLabelRows(items, ringRowsSpec{
		CentreY:  l.h / 2,
		RadiusPt: l.spec.Radius * l.side,
		Heights:  hs,
		GapPt:    gap,
		Top:      0,
		Bottom:   l.h,
	})
	for _, r := range placed {
		row := cycleNodesRow{side: r.Side, need: heights[r.Index],
			y0: math.Max(r.Y-r.H/2, 0), y1: math.Min(r.Y+r.H/2, l.h)}
		if r.Side == ringSideRight {
			row.cueX0 = l.ringX + l.side + cycleNodesRingGapPt
			row.cueX1 = row.cueX0 + cycleNodesCueColPt
			row.textX0, row.textX1 = row.cueX1, row.cueX1+l.textW
		} else {
			row.cueX1 = l.ringX - cycleNodesRingGapPt
			row.cueX0 = row.cueX1 - cycleNodesCueColPt
			row.textX0, row.textX1 = row.cueX0-l.textW, row.cueX0
		}
		l.rows[r.Index] = row
	}
}

// cycleNodesRowNeedPt is the height the writer needs for one label at the
// layout's sizes and text width, plus a point of rounding slack.
func cycleNodesRowNeedPt(ctx ExpandContext, s CycleNodesStep, lay cycleNodesLayout) float64 {
	text := cycleNodesLabelText(s, "l", lay.labelSize, lay.bodySize)
	return math.Ceil(writtenFitHeightPt(ctx.themeFonts(), text, lay.textW, 0)) + cycleNodesRowSlackPt
}

// cycleNodesPlain strips inline markdown markers for measuring.
func cycleNodesPlain(s string) string {
	return strings.TrimSpace(s)
}

// PostExpandWarnings reports, by measurement on the current template, the
// steps whose label rows cannot hold their text at the 12pt floor, an inside
// label that does not fit its node and a centre label that does not fit the
// ring.
func (c *cycleNodes) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*CycleNodesValues)
	if !ok || v == nil || len(v.Steps) < ringMinItems || len(v.Steps) > ringMaxItems {
		return nil
	}
	lay, err := cycleNodesMeasure(ctx, v, cycleNodesOverrides(overrides))
	if err != nil {
		return nil
	}
	var warnings []string
	if lay.mode == cycleNodesLabelsInside {
		for i, fits := range lay.nodeFits {
			if !fits {
				warnings = append(warnings, fmt.Sprintf("%s: cycle-nodes steps[%d].label does not fit its %.0fpt node at %.0fpt on %d lines (a word breaks or the label runs past the circle); keep inside labels to two short words, or set overrides.labels to \"outside\"",
					ErrCodeBodyTooLong, i, lay.dia*lay.side, cycleNodesFloorPt, cycleNodesInsideLines))
			}
		}
	} else if !lay.rowsFit {
		warnings = append(warnings, lay.rowWarnings(v)...)
	}
	if v.Center != nil && strings.TrimSpace(v.Center.Label) != "" && !lay.centreFits {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-nodes center.label does not fit the %.0fpt circle inside the ring even at %.0fpt (a word breaks or the label runs past the circle); keep it to about two short words or drop it",
			ErrCodeBodyTooLong, lay.centreFr.W*lay.side, lay.centrePt))
	}
	return warnings
}

// rowWarnings names, for every side whose label rows outgrow the height they
// share, the steps that take more than an equal share of it (all of them when
// every row is over, or when all are equally tall).
func (l *cycleNodesLayout) rowWarnings(v *CycleNodesValues) []string {
	var warnings []string
	lineChars := int(math.Floor((l.textW - 2*cycleNodesLabelInsetPt) / (sizingCapacityEm * l.bodySize)))
	for _, side := range []string{ringSideLeft, ringSideRight} {
		need, room := l.needPt[side], l.roomPt[side]
		if need <= room {
			continue
		}
		var idx []int
		tallest := 0.0
		for i, r := range l.rows {
			if r.side == side {
				idx = append(idx, i)
				tallest = math.Max(tallest, r.need)
			}
		}
		share := (room - float64(len(idx)-1)*l.rowGap) / float64(len(idx))
		lines := max(int((share-l.labelSize*sizingLineSpacing-2*cycleNodesLabelInsetPt)/(l.bodySize*sizingLineSpacing)), 0)
		advice := fmt.Sprintf("keep each description to about %d lines (%d characters)", lines, lines*lineChars)
		if lines < 1 {
			advice = "a row holds only a one-line label here, so drop the descriptions"
		}
		for _, i := range idx {
			if l.rows[i].need <= share && l.rows[i].need < tallest {
				continue
			}
			field := "description"
			if strings.TrimSpace(v.Steps[i].Description) == "" {
				field = "label"
			}
			warnings = append(warnings, fmt.Sprintf("%s: cycle-nodes steps[%d].%s needs %.0fpt but a label row with %d steps is %.0fpt tall (%d rows share %.0fpt); %s — shorten it or use fewer steps",
				ErrCodeBodyTooLong, i, field, l.rows[i].need, len(v.Steps), math.Max(share, 0), len(idx), room, advice))
		}
	}
	return warnings
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (c *cycleNodes) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*CycleNodesValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("cycle-nodes: values must be *CycleNodesValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*CycleNodesOverrides); !ok {
			return nil, fmt.Errorf("cycle-nodes: overrides must be *CycleNodesOverrides, got %T", overrides)
		}
	}
	if len(v.Steps) < ringMinItems || len(v.Steps) > ringMaxItems {
		return nil, fmt.Errorf("cycle-nodes: %d steps; the ring holds %d-%d", len(v.Steps), ringMinItems, ringMaxItems)
	}
	ovr := cycleNodesOverrides(overrides)
	lay, err := cycleNodesMeasure(ctx, v, ovr)
	if err != nil {
		return nil, err
	}
	base := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	paints := cycleNodesPaints(ctx, v, ovr, base)

	ring := &jsonschema.GridCellInput{Fit: "contain"}
	if ovr.Arrows != cycleNodesArrowsNone {
		ring.Layers = append(ring.Layers, ringLinkArrowLayers(lay.spec, lay.nodes, ringLinkFillJSON())...)
	}
	inside := lay.mode == cycleNodesLabelsInside
	ring.Layers = append(ring.Layers, ringNodeLayers(lay.nodes, func(i int) ringNodePaint {
		p := paints[i]
		if inside {
			return ringNodePaint{Fill: p.fill, Text: ringNodeTextJSON("ctr", "ctr", cycleNodesNodeInsetPt,
				ringNodePara{Content: pptx.ConvertMarkdownEmphasis(v.Steps[i].Label), Size: lay.nodePt, Bold: true, Color: p.labelInk})}
		}
		return ringNodePaint{Fill: p.fill, Text: ringNodeTextJSON("ctr", "ctr", -1,
			ringNodePara{Content: strconv.Itoa(i + 1), Size: lay.numeralPt, Bold: true, Color: p.ink})}
	})...)
	if inside {
		badge := structuralDarkTone(ctx)
		badgeInk := readableTextOn(ctx, badge, "lt1")
		for _, n := range lay.nodes {
			ring.Layers = append(ring.Layers, jsonschema.LayerInput{
				Name:  fmt.Sprintf("badge-%d", n.Index+1),
				Frame: ringLayerFrame(ringInnerBadgeFrame(lay.spec, n, lay.dia, lay.badgeDia)),
				Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: badge.fillJSON(), Line: noLine,
					Text: ringNodeTextJSON("ctr", "ctr", 0, ringNodePara{Content: strconv.Itoa(n.Index + 1), Size: cycleNodesFloorPt, Bold: true, Color: badgeInk})},
			})
		}
	}
	if v.Center != nil && strings.TrimSpace(v.Center.Label) != "" {
		ring.Layers = append(ring.Layers, ringCentreLabelLayer(lay.centreFr, ringNodeTextJSON("ctr", "ctr", cycleNodesCentreInsetPt,
			ringNodePara{Content: pptx.ConvertMarkdownEmphasis(v.Center.Label), Size: lay.centrePt, Bold: true, Color: "dk1"})))
	}

	places := []ringPlacement{{X0: lay.ringX, X1: lay.ringX + lay.side, Y0: lay.ringY, Y1: lay.ringY + lay.side, Cell: ring}}
	for i, r := range lay.rows {
		// The label turns towards the ring: right-aligned in the left column.
		align := "l"
		if r.side == ringSideLeft {
			align = "r"
		}
		places = append(places,
			ringPlacement{X0: r.cueX0, X1: r.cueX1, Y0: r.y0, Y1: r.y1, Cell: cycleNodesTextCell(ringNodeTextJSON(align, "t", cycleNodesLabelInsetPt,
				ringNodePara{Content: strconv.Itoa(i + 1), Size: lay.labelSize, Bold: true, Color: paints[i].cueInk}))},
			ringPlacement{X0: r.textX0, X1: r.textX1, Y0: r.y0, Y1: r.y1, Cell: cycleNodesTextCell(cycleNodesLabelText(v.Steps[i], align, lay.labelSize, lay.bodySize))},
		)
	}
	grid, err := ringLattice(places, lay.w, lay.h)
	if err != nil {
		return nil, fmt.Errorf("cycle-nodes: %w", err)
	}
	grid.VerticalAlign = "center"
	return grid, nil
}

// cycleNodesPaint is the colouring of one step.
type cycleNodesPaint struct {
	fill     json.RawMessage
	ink      string // numeral in the node
	labelInk string // label in the node (labels inside)
	cueInk   string // number cue beside the label
}

// cycleNodesPaints colours the steps: every node a neutral tint with its
// number in the accent, and the highlighted one the single solid accent
// circle. cell_accent_mode alternate / progressive tints each node in its own
// accent instead.
func cycleNodesPaints(ctx ExpandContext, v *CycleNodesValues, ovr *CycleNodesOverrides, base string) []cycleNodesPaint {
	out := make([]cycleNodesPaint, len(v.Steps))
	varied := ovr.CellAccentMode != "" && ovr.CellAccentMode != CellAccentUniform
	for i := range out {
		accent := ctx.ResolveCellAccent(base, i, ovr.CellAccentMode)
		tone := neutralTone(NeutralTint8)
		if varied {
			tone = inactiveTintTone(accent)
		}
		p := cycleNodesPaint{
			fill:     tone.fillJSON(),
			ink:      accentInkOnTone(ctx, accent, tone, cycleNodesInkContrast),
			labelInk: readableInkOn(ctx, tone, "dk1", cycleNodesInkContrast),
			cueInk:   accentInkOnLight(ctx, accent, cycleNodesInkContrast),
		}
		if i == v.Highlight-1 {
			solid := fillTone{Color: accent}
			p.fill = accentFillJSON(accent)
			p.ink = readableTextOn(ctx, solid, "lt1")
			p.labelInk = p.ink
		}
		out[i] = p
	}
	return out
}

// cycleNodesLabelText is a step's label (bold) over its optional description,
// in an unfilled frame with a hairline margin.
func cycleNodesLabelText(s CycleNodesStep, align string, labelSize, bodySize float64) json.RawMessage {
	paras := []ringNodePara{{Content: pptx.ConvertMarkdownEmphasis(s.Label), Size: labelSize, Bold: true, Color: "dk1"}}
	if d := strings.TrimSpace(s.Description); d != "" {
		paras[0].SpaceAfter = cycleNodesLabelSpacePt
		paras = append(paras, ringNodePara{Content: pptx.ConvertMarkdownEmphasis(d), Size: bodySize, Color: "dk1"})
	}
	return ringNodeTextJSON(align, "t", cycleNodesLabelInsetPt, paras...)
}

// cycleNodesTextCell is an unfilled, unoutlined text frame.
func cycleNodesTextCell(text json.RawMessage) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"none"`), Text: text},
	}
}
