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
// cycle-intake pattern — 1-3 linear intake steps feeding a loop of 3-8 phases
// ---------------------------------------------------------------------------
//
// "Onboard once, then run the service cycle": a short lane of interlocking
// arrows (the value chain's pentagon + chevrons) on the ring's horizontal
// centreline, an accent arrow that touches the ring at 9 o'clock, and the
// recurring loop to its right. The loop's phase 1 begins at 9 o'clock, where
// the arrow lands, and runs clockwise.
//
// The loop is ONE lattice cell (fit "contain") drawn by the family's shared
// builders: ring segments with numbered badges (ring_draw.go), or with
// overrides.loop_style "nodes" numbered circles joined by curved arrows
// (ring_nodes_draw.go). The intake lane takes the left of the slide, so every
// loop label stands in ONE numbered list right of the ring (cycle-ring's
// legend rows); the intake descriptions sit under their arrows.
//
// A content area too narrow for lane, ring and list side by side (a compose
// half) stacks them: the lane across the top, a down arrow from its last step
// into the ring at 12 o'clock — where phase 1 then begins — and the list left
// of the ring. Intake descriptions are left off there.

func init() {
	Default().Register(&cycleIntake{})
}

type cycleIntake struct{}

func (p *cycleIntake) Name() string { return cycleIntakeName }
func (p *cycleIntake) Description() string {
	return "Linear intake feeding a recurring loop: 1-3 interlocking intake arrows on the left, an accent arrow into the ring at 9 o'clock, then a loop of 3-8 numbered phases (ring segments, or `loop_style` `nodes` circles joined by arrows) with the phase labels in one numbered list right of the ring; one optional highlighted phase, optional centre label"
}
func (p *cycleIntake) UseWhen() string {
	return "One to three one-off steps lead into a cycle of 3-8 phases that then repeats — onboarding then the recurring service cycle, deal intake then the portfolio review loop, data ingestion then the model-improvement loop, acquire then retain; prefer cycle-ring or cycle-nodes when there is no intake and the loop is the whole message, process-flow or value-chain when the steps run once and never loop back, swimlane when actors own the steps, and cycle-figure-eight for two coupled loops"
}
func (p *cycleIntake) NotWhen() string {
	return "Nothing feeds the loop (use cycle-ring, or cycle-nodes for discrete stations), the sequence runs once without looping back (use process-flow, numbered-step-strip or value-chain), actors own the steps (use swimlane), two loops are coupled (use cycle-figure-eight), there are more than 3 intake steps (show the intake as its own process-flow slide) or more than 8 loop phases (merge phases)"
}
func (p *cycleIntake) Version() int { return 1 }
func (p *cycleIntake) CellsHint() string {
	return "1-3 intake arrows + entry arrow + 1 loop (3-8 phases) + 3-8 list labels"
}
func (p *cycleIntake) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"process-flow", "kpi-3up", "next-steps"},
		DensityClass:       "medium",
		AccentWeight:       "light",
		SparseThresholdPct: 15,
	}
}
func (p *cycleIntake) SupportsInlineMarkdown() bool { return true }

func (p *cycleIntake) ExemplarValues() any {
	return &CycleIntakeValues{
		Intake: []CycleIntakeStep{
			{Label: "Sign contract", Description: "Scope and service levels agreed"},
			{Label: "Onboard", Description: "Data migrated, users trained"},
		},
		Loop: []CycleIntakePhase{
			{Label: "Plan the quarter", Description: "Agree priorities with the client"},
			{Label: "Deliver", Description: "Run the service to the agreed levels", Highlight: true},
			{Label: "Review", Description: "Report results against the targets"},
			{Label: "Improve", Description: "Fix root causes, adjust the scope"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CycleIntakeStep is one linear step of the intake lane.
type CycleIntakeStep struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// CycleIntakePhase is one phase of the recurring loop.
type CycleIntakePhase struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Highlight   bool   `json:"highlight,omitempty"`
}

// CycleIntakeCenter is the optional label inside the loop.
type CycleIntakeCenter struct {
	Label string `json:"label"`
}

// CycleIntakeValues holds the 1-3 intake steps, the 3-8 loop phases and the
// optional centre label.
type CycleIntakeValues struct {
	Intake []CycleIntakeStep  `json:"intake"`
	Loop   []CycleIntakePhase `json:"loop"`
	Center *CycleIntakeCenter `json:"center,omitempty"`
}

// CycleIntakeOverrides is the standard text overrides (header_size sizes the
// intake and loop labels, body_size the descriptions) plus the loop's look.
type CycleIntakeOverrides struct {
	TextOverrides
	LoopStyle string `json:"loop_style,omitempty"` // segments (default) | nodes
}

func (p *cycleIntake) NewValues() any       { return &CycleIntakeValues{} }
func (p *cycleIntake) NewOverrides() any    { return &CycleIntakeOverrides{} }
func (p *cycleIntake) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	cycleIntakeName = "cycle-intake"

	cycleIntakeMinSteps = 1
	cycleIntakeMaxSteps = 3

	cycleIntakeStepLabelMax = 24
	cycleIntakeStepDescMax  = 60
	cycleIntakeLabelMax     = 26
	cycleIntakeDescMax      = 70
	cycleIntakeCenterMax    = 20

	cycleIntakeStyleSegments = "segments"
	cycleIntakeStyleNodes    = "nodes"

	cycleIntakeStartDeg        = 180.0 // side-by-side: phase 1 begins at 9 o'clock, where the arrow lands
	cycleIntakeStackedStartDeg = -90.0 // stacked: the arrow comes down into 12 o'clock

	cycleIntakeStepPt     = scaleBodyPt    // intake label
	cycleIntakeLabelPt    = scaleSubheadPt // loop label, stepping down to 12pt when the rows need it
	cycleIntakeDescPt     = scaleBodyPt
	cycleIntakeMinStepPt  = 80.0  // narrowest intake arrow of the side-by-side layout
	cycleIntakeMaxStepPt  = 150.0 // widest intake arrow
	cycleIntakeLaneHPt    = 40.0  // arrow height before a wrapped label needs more
	cycleIntakeArrowLenPt = 22.0  // entry arrow, along its direction
	cycleIntakeArrowWPt   = 18.0  // … and across it
	cycleIntakeArrowGapPt = 4.0   // last intake arrow's point to the entry arrow
	cycleIntakeDescGapPt  = 4.0   // lane to the descriptions under it
	cycleIntakeDescInset  = 2.0
	cycleIntakeMaxLabelPt = 280.0 // widest list column: the rest of a wide slide centres the block

	cycleIntakeMaxShrink   = 0.15 // share of its side the ring gives the list when its rows would not keep one pitch
	cycleIntakeShrinkSteps = 3

	cycleIntakeStackedRingFrac = 0.45 // ring side as a share of the width when stacked

	// Entry arrow: a block arrow with a head as long as the arrow is wide.
	cycleIntakeArrowShaftAdj = 50000
	cycleIntakeArrowHeadAdj  = 60000
)

// Shares of the width the lane and the ring square take side by side, by
// intake step count (index = count); the rest is the entry arrow and the
// list. Three steps take the most lane, so the ring gives a little.
var (
	cycleIntakeLaneFrac = [...]float64{0, 0.17, 0.29, 0.39}
	cycleIntakeRingFrac = [...]float64{0, 0.36, 0.33, 0.30}

	cycleIntakeStyles = []string{cycleIntakeStyleSegments, cycleIntakeStyleNodes}
)

// Copy budgets (characters), measured on the smallest shipped content area
// (abstract, 687 x 294pt) at default sizes: text inside them is written at
// 12pt or above with every row as tall as its text needs on every shipped
// template (TestCycleIntakeMeasuredBudgets). The intake's budgets are its
// schema maxima at every count (24 / 60: an arrow grows with its label, and
// the descriptions have the half of the slide under the lane); the loop's
// depend on the width the lane leaves the list and on its row count.

// cycleIntakeLabelBudget is a loop label's budget: one line of the list
// beside a three-step lane once there are four phases or more.
func cycleIntakeLabelBudget(nIntake, nLoop int) int {
	if nIntake >= 3 && nLoop >= 4 {
		return 22
	}
	return cycleIntakeLabelMax
}

// cycleIntakeDescBudget is a loop description's budget; 0 = no static budget
// (eight phases: the list holds one description line only where the area is
// tall enough, and PostExpandWarnings says when it left them off).
func cycleIntakeDescBudget(nIntake, nLoop int) int {
	if nLoop >= ringMaxItems {
		return 0
	}
	switch nIntake {
	case 1:
		if nLoop <= 5 {
			return cycleIntakeDescMax
		}
		return 40
	case 2:
		switch {
		case nLoop <= 4:
			return cycleIntakeDescMax
		case nLoop == 5:
			return 60
		}
		return 30
	default:
		switch nLoop {
		case 3:
			return cycleIntakeDescMax
		case 4:
			return 65
		case 5:
			return 45
		}
		return 22
	}
}

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (p *cycleIntake) Schema() *Schema {
	step := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(cycleIntakeStepLabelMax).WithDescription("Intake step name set in its arrow, e.g. \"Onboard\". With 3 steps keep each word to about 9 letters (a word never breaks; BODY_TOO_LONG names the label)"),
		"description": StringSchema(cycleIntakeStepDescMax).WithDescription("Optional muted line under the arrow. Left off in the stacked layout of a narrow area"),
	}, []string{"label"}).WithAdditionalProperties(false)

	phase := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(cycleIntakeLabelMax).WithDescription("Phase name, bold beside its number. Budget: 26 characters; 22 (one line) with 3 intake steps and 4-8 phases"),
		"description": StringSchema(cycleIntakeDescMax).WithDescription("Optional muted line under the label. Budget by intake steps 1 / 2 / 3 — 3 phases: 70 / 70 / 70; 4: 70 / 70 / 65; 5: 70 / 60 / 45; 6-7: 40 / 30 / 22; 8: one line, drawn only where the content area is at least about 320pt tall. expand_pattern reports BODY_TOO_LONG past it or when the list leaves the descriptions off"),
		"highlight":   BooleanSchema().WithDescription("Make this phase the one solid accent segment / node (at most one phase); the others stay neutral"),
	}, []string{"label"}).WithAdditionalProperties(false)

	values := ObjectSchema(map[string]*Schema{
		"intake": ArraySchema(RefSchema("step"), cycleIntakeMinSteps, cycleIntakeMaxSteps).WithDescription("1-3 linear steps that happen once, in order, before the loop starts"),
		"loop":   ArraySchema(RefSchema("phase"), ringMinItems, ringMaxItems).WithDescription("3-8 phases of the recurring loop in order; phase 1 begins where the intake arrow enters the ring (9 o'clock) and the loop runs clockwise"),
		"center": ObjectSchema(map[string]*Schema{
			"label": StringSchema(cycleIntakeCenterMax).WithDescription("Short name of the loop inside the ring, e.g. \"Every quarter\" (short words: a word never breaks)"),
		}, []string{"label"}).WithAdditionalProperties(false).WithDescription("Optional centre label"),
	}, []string{"intake", "loop"}).WithAdditionalProperties(false)

	overrides := textOverridesSchema()
	overrides.raw.Properties["header_size"] = NumberSchema(12, 24).WithDescription("Label size in points: loop labels (default 14, stepping down to 12 when the list needs it) and intake labels (default 12)")
	overrides.raw.Properties["body_size"] = NumberSchema(12, 20).WithDescription("Description size in points (default 12)")
	overrides.raw.Properties["cell_accent_mode"] = EnumSchema("uniform", "alternate", "progressive").WithDescription("Loop only (the intake keeps the base tint). uniform (default): tinted phases, dark badges. alternate / progressive: each phase takes a light tint of its own accent").WithDefault("uniform")
	overrides.raw.Properties["loop_style"] = EnumSchema(cycleIntakeStyles...).WithDescription("segments (default): a ring of phase segments with numbered badges. nodes: numbered circles joined by curved arrows").WithDefault(cycleIntakeStyleSegments)

	return ObjectSchema(map[string]*Schema{
		"values":    values,
		"overrides": overrides,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"step":  step,
		"phase": phase,
	}).WithDescription("1-3 linear intake steps feeding a recurring loop of 3-8 phases")
}

func cycleIntakeOverrides(overrides any) *CycleIntakeOverrides {
	if o, ok := overrides.(*CycleIntakeOverrides); ok && o != nil {
		return o
	}
	return &CycleIntakeOverrides{}
}

func (p *cycleIntake) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*CycleIntakeValues)
	if !ok || v == nil {
		return fmt.Errorf("cycle-intake: values must be *CycleIntakeValues, got %T", values)
	}
	const name = cycleIntakeName
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*CycleIntakeOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("cycle-intake: overrides must be *CycleIntakeOverrides, got %T", overrides))
		} else if ovr != nil {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.LoopStyle != "" && !slices.Contains(cycleIntakeStyles, ovr.LoopStyle) {
				errs = append(errs, errInvalidEnum(name, "overrides.loop_style", ovr.LoopStyle, cycleIntakeStyles))
			}
		}
	}

	if n := len(v.Intake); n < cycleIntakeMinSteps {
		errs = append(errs, errMinItems(name, "intake", cycleIntakeMinSteps, n, "(hint: a loop that nothing feeds is cycle-ring or cycle-nodes)"))
	} else if n > cycleIntakeMaxSteps {
		errs = append(errs, errMaxItems(name, "intake", cycleIntakeMaxSteps, n, "(hint: keep the three steps that lead into the loop, or show the intake as its own process-flow slide before a cycle-ring)"))
	}
	for i, s := range v.Intake {
		errs = append(errs, cycleIntakeTextErrors(fmt.Sprintf("intake[%d]", i), s.Label, s.Description, cycleIntakeStepLabelMax, cycleIntakeStepDescMax)...)
	}

	if n := len(v.Loop); n < ringMinItems {
		errs = append(errs, errMinItems(name, "loop", ringMinItems, n, "(hint: a loop needs at least 3 phases; steps that run once are process-flow)"))
	} else if n > ringMaxItems {
		errs = append(errs, errMaxItems(name, "loop", ringMaxItems, n, "(hint: merge related phases, or use cycle-figure-eight for two coupled loops)"))
	}
	highlights := 0
	for i, ph := range v.Loop {
		errs = append(errs, cycleIntakeTextErrors(fmt.Sprintf("loop[%d]", i), ph.Label, ph.Description, cycleIntakeLabelMax, cycleIntakeDescMax)...)
		if ph.Highlight {
			highlights++
		}
	}
	if highlights > 1 {
		errs = append(errs, newValidationError(name, "loop", ErrCodeOutOfRange,
			fmt.Sprintf("cycle-intake: %d phases set highlight; at most one phase is highlighted (the single solid accent of the loop)", highlights), nil))
	}
	if c := v.Center; c != nil {
		switch l := runeLen(c.Label); {
		case strings.TrimSpace(c.Label) == "":
			errs = append(errs, errRequired(name, "center.label"))
		case l > cycleIntakeCenterMax:
			errs = append(errs, errMaxLength(name, "center.label", cycleIntakeCenterMax, l))
		}
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"cycle-intake: cell_overrides are not supported (use overrides, or loop[].highlight for the one accent phase)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// cycleIntakeTextErrors checks one item's required label and its lengths.
func cycleIntakeTextErrors(path, label, description string, labelMax, descMax int) []error {
	var errs []error
	switch l := runeLen(label); {
	case strings.TrimSpace(label) == "":
		errs = append(errs, errRequired(cycleIntakeName, path+".label"))
	case l > labelMax:
		errs = append(errs, errMaxLength(cycleIntakeName, path+".label", labelMax, l))
	}
	if l := runeLen(description); l > descMax {
		errs = append(errs, errMaxLength(cycleIntakeName, path+".description", descMax, l))
	}
	return errs
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// cycleIntakeBox is a rectangle in points from the top-left corner of the
// pattern's block.
type cycleIntakeBox struct {
	x0, x1, y0, y1 float64
}

// cycleIntakeLayout carries every measurement Expand and PostExpandWarnings
// share.
type cycleIntakeLayout struct {
	w, h    float64
	stacked bool // lane above the ring instead of left of it
	nodes   bool // loop_style nodes

	// Intake lane.
	fit       valueChainArrowFit // step width, point depth, label size
	laneX0    float64
	lane      cycleIntakeBox // the row of intake arrows
	unfit     []string       // intake labels with a word no arrow holds on one line
	descPt    float64
	showDesc  bool      // intake descriptions are drawn
	descNeed  []float64 // height each intake description needs
	descRoom  float64   // height they are given
	descY0    float64
	entry     cycleIntakeBox // the accent arrow into the ring
	entryDown bool

	// Loop.
	side, ringX, ringY float64
	spec               ringSpec
	items              []ringItem
	ringNodes          []ringNode
	dia                float64 // node diameter (nodes) as a fraction of the square
	badgeDia           float64
	centrePt           float64
	centreFits         bool
	centreFr           ringFrame

	// The numbered list beside the ring: cycle-ring's legend rows, measured in
	// a strip listH tall that starts at listY.
	list   cycleRingLayout
	listX0 float64 // left edge of the list (numeral column first; label column first when stacked)
	listY  float64
}

func (v *CycleIntakeValues) hasStepDescriptions() bool {
	for _, s := range v.Intake {
		if strings.TrimSpace(s.Description) != "" {
			return true
		}
	}
	return false
}

func (v *CycleIntakeValues) centre() string {
	if v.Center == nil {
		return ""
	}
	return strings.TrimSpace(v.Center.Label)
}

// phases is the loop as cycle-ring phases, for the shared list rows.
func (v *CycleIntakeValues) phases() *CycleRingValues {
	out := &CycleRingValues{Phases: make([]CycleRingPhase, len(v.Loop))}
	for i, ph := range v.Loop {
		out.Phases[i] = CycleRingPhase(ph)
	}
	return out
}

func (v *CycleIntakeValues) highlight() int {
	for i, ph := range v.Loop {
		if ph.Highlight {
			return i
		}
	}
	return -1
}

// cycleIntakeMeasure resolves the geometry for the content area.
func cycleIntakeMeasure(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides) (cycleIntakeLayout, error) {
	w, h := sizingAreaPt(ctx)
	nI := len(v.Intake)
	lay := cycleIntakeLayout{
		w: w, h: h,
		nodes:    ovr.LoopStyle == cycleIntakeStyleNodes,
		descPt:   shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, cycleIntakeDescPt)),
		showDesc: true,
	}
	stepPt := shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, cycleIntakeStepPt))
	gap := valueChainArrowGapPt
	gaps := float64(nI-1) * gap

	// Side by side: lane, entry arrow, ring, list.
	laneW := math.Max(w*cycleIntakeLaneFrac[nI], float64(nI)*cycleIntakeMinStepPt+gaps)
	stepW := math.Min((laneW-gaps)/float64(nI), cycleIntakeMaxStepPt)
	laneW = float64(nI)*stepW + gaps
	full := cycleNodesRingSide(w, h, cycleIntakeRingFrac[nI])
	if w-cycleIntakeFixedPt(laneW, full) < cycleRingLegendMinPt {
		return cycleIntakeStack(ctx, v, ovr, lay, stepPt)
	}

	lay.fit = cycleIntakeFitLane(ctx, v, stepW, stepPt)
	lay.unfit = lay.fit.unfit
	laneH := math.Min(cycleIntakeLaneHeight(ctx, v, lay.fit), h)
	lay.descNeed = make([]float64, nI)
	for i, s := range v.Intake {
		if d := strings.TrimSpace(s.Description); d != "" {
			lay.descNeed[i] = math.Ceil(writtenFitHeightPt(ctx.themeFonts(), cycleIntakeDescText(d, lay.descPt), stepW, 0)) + ringLabelSlackPt
		}
	}

	// The ring takes its share of the width. When the list beside it cannot
	// keep one pitch (a row measured a line taller than the others, and the
	// height does not hold every row at that height), the ring gives up to
	// cycleIntakeMaxShrink of its side to the list column, where the long row
	// sets on the lines of the others (go-slide-creator-by2e3). A ring whose
	// centre label would stop fitting keeps its side.
	var first cycleIntakeLayout
	for step := 0; step <= cycleIntakeShrinkSteps; step++ {
		side := full * (1 - cycleIntakeMaxShrink*float64(step)/cycleIntakeShrinkSteps)
		if step > 0 && side < ringMinSidePt {
			break
		}
		try := lay
		even, err := try.placeSideBySide(ctx, v, ovr, laneW, laneH, side)
		if err != nil {
			return try, err
		}
		if step == 0 {
			first = try
		}
		if even && (step == 0 || try.centreFits || !first.centreFits) {
			return try, nil
		}
	}
	return first, nil
}

// cycleIntakeFixedPt is the width of the side-by-side layout without its list
// labels: lane, entry arrow, ring, the gap to the list and the numerals.
func cycleIntakeFixedPt(laneW, side float64) float64 {
	return laneW + cycleIntakeArrowGapPt + cycleIntakeArrowLenPt + side + cycleRingLegendGapPt + ringNumberColPt
}

// placeSideBySide places the lane (laneW x laneH), the entry arrow, a ring of
// the given side and the list, left to right and centred as a block. It
// reports whether the list keeps one pitch with nothing cut or left off.
func (l *cycleIntakeLayout) placeSideBySide(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides, laneW, laneH, side float64) (bool, error) {
	w, h := l.w, l.h
	fixed := cycleIntakeFixedPt(laneW, side)
	labelW := math.Min(math.Max(w-fixed, 1), cycleIntakeMaxLabelPt)
	l.laneX0 = math.Max((w-fixed-labelW)/2, 0)
	l.lane = cycleIntakeBox{x0: l.laneX0, x1: l.laneX0 + laneW, y0: (h - laneH) / 2, y1: (h + laneH) / 2}

	l.ringX = l.lane.x1 + cycleIntakeArrowGapPt + cycleIntakeArrowLenPt
	l.entry = cycleIntakeBox{x0: l.lane.x1 + cycleIntakeArrowGapPt, x1: l.ringX, y0: (h - cycleIntakeArrowWPt) / 2, y1: (h + cycleIntakeArrowWPt) / 2}
	if err := l.placeRing(ctx, v, side, (h-math.Min(side, h))/2, cycleIntakeStartDeg); err != nil {
		return false, err
	}

	// Descriptions under the lane, each as wide as its arrow.
	l.descY0 = l.lane.y1 + ctx.Gap(cycleIntakeDescGapPt)
	l.descRoom = math.Max(h-l.descY0, 0)

	l.listX0 = l.ringX + l.side + ctx.Gap(cycleRingLegendGapPt)
	return l.placeList(ctx, v, ovr, math.Min(labelW, w-l.listX0-ringNumberColPt), 0, h), nil
}

// cycleIntakeStack lays the pattern out for a narrow area: the lane across
// the top with its last arrow above the ring, a down arrow into the ring at
// 12 o'clock, and the list left of the ring.
func cycleIntakeStack(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides, lay cycleIntakeLayout, stepPt float64) (cycleIntakeLayout, error) {
	w, h := lay.w, lay.h
	nI := float64(len(v.Intake))
	gaps := (nI - 1) * valueChainArrowGapPt
	lay.stacked, lay.entryDown, lay.showDesc = true, true, false
	between := cycleIntakeArrowLenPt + cycleIntakeArrowGapPt // lane to ring

	// The last arrow's centre is the ring's centre: a ring wider than a step
	// reaches past the lane on the right.
	stepFor := func(side float64) float64 {
		s := (w - gaps) / nI
		if s < side {
			s = (w - side/2 - gaps) / (nI - 0.5)
		}
		return math.Max(math.Min(s, cycleIntakeMaxStepPt), 1)
	}
	side := cycleNodesRingSide(w, math.Max(h-cycleIntakeLaneHPt-between, 1), cycleIntakeStackedRingFrac)
	var laneH float64
	for pass := 0; pass < 2; pass++ {
		lay.fit = cycleIntakeFitLane(ctx, v, stepFor(side), stepPt)
		laneH = math.Min(cycleIntakeLaneHeight(ctx, v, lay.fit), h/2)
		side = math.Max(math.Min(side, h-laneH-between), 1)
	}
	lay.unfit = lay.fit.unfit
	stepW := lay.fit.colWPt
	laneW := nI*stepW + gaps

	top := math.Max((h-laneH-between-side)/2, 0)
	centreX := math.Max(w-math.Max(side, stepW)/2, side/2)
	laneX1 := math.Min(centreX+stepW/2, w)
	lay.laneX0 = math.Max(laneX1-laneW, 0)
	lay.lane = cycleIntakeBox{x0: lay.laneX0, x1: lay.laneX0 + laneW, y0: top, y1: top + laneH}
	ringY := lay.lane.y1 + between
	if err := lay.placeRing(ctx, v, side, ringY, cycleIntakeStackedStartDeg); err != nil {
		return lay, err
	}
	lay.ringX = math.Min(math.Max(centreX-lay.side/2, 0), w-lay.side)
	cx := lay.ringX + lay.side/2
	lay.entry = cycleIntakeBox{x0: cx - cycleIntakeArrowWPt/2, x1: cx + cycleIntakeArrowWPt/2, y0: lay.lane.y1 + cycleIntakeArrowGapPt, y1: ringY}

	// The list turns towards the ring: right-aligned, each number beside it.
	labelW := math.Min(lay.ringX-ctx.Gap(cycleRingLegendGapPt)-ringNumberColPt, cycleIntakeMaxLabelPt)
	lay.listX0 = math.Max(lay.ringX-ctx.Gap(cycleRingLegendGapPt)-ringNumberColPt-labelW, 0)
	lay.placeList(ctx, v, ovr, labelW, lay.lane.y1+cycleIntakeArrowGapPt, h)
	return lay, nil
}

// cycleIntakeFitLane finds the deepest point at which every intake label word
// stays whole inside its arrow of the given width (the value chain's arrow
// fit); the labels that cannot are in the fit's unfit list.
func cycleIntakeFitLane(ctx ExpandContext, v *CycleIntakeValues, stepW, labelPt float64) valueChainArrowFit {
	steps := make([]ValueChainStep, len(v.Intake))
	for i, s := range v.Intake {
		steps[i] = ValueChainStep{Label: strings.TrimSpace(s.Label)}
	}
	fit := valueChainArrowFit{colWPt: stepW, rowHPt: cycleIntakeLaneHPt, labelPt: labelPt}
	font := ctx.Theme.BodyFont
	for d := math.Max(math.Round(fit.rowHPt*valueChainNotchFrac), valueChainMinNotchPt); d >= valueChainMinNotchPt; d-- {
		fit.notchPt = d
		if len(fit.unfitLabels(steps, font)) == 0 {
			return fit
		}
	}
	fit.unfit = fit.unfitLabels(steps, font)
	return fit
}

// cycleIntakeLaneCells is the intake lane: a pentagon, then chevrons whose
// tails tuck under the point before them, on the ring's own band tone (the
// accent's content swatch) with measured ink. The adjust values are set by the caller once the lane's height is
// known.
func cycleIntakeLaneCells(ctx ExpandContext, v *CycleIntakeValues, fit valueChainArrowFit, accent string) []*jsonschema.GridCellInput {
	tone := ringBandTone(ctx, accent)
	ink := readableInkOn(ctx, tone, "dk1", ringInkContrastMin)
	cells := make([]*jsonschema.GridCellInput, len(v.Intake))
	for i, s := range v.Intake {
		text := buildValueChainLabelText(pptx.ConvertMarkdownEmphasis(strings.TrimSpace(s.Label)), fit.labelPt, ink)
		cells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: fit.geometry(i),
				Fill:     tone.fillJSON(),
				Line:     noLine,
				Text:     withTextInsets(text, valueChainArrowInsetPt),
			},
			BleedLeft: fit.bleedPt(i),
		}
	}
	return cells
}

// cycleIntakeLaneHeight is the lane's height: the base arrow height, or what
// the tallest label needs inside its arrow's own text rectangle — the larger
// of the writer's measure and the capacity model's line count, so a face
// wider than the theme font's metrics still gets its lines.
func cycleIntakeLaneHeight(ctx ExpandContext, v *CycleIntakeValues, fit valueChainArrowFit) float64 {
	h := fit.rowHeightPt(ctx.themeFonts(), cycleIntakeLaneCells(ctx, v, fit, ctx.DefaultAccent()))
	for i, s := range v.Intake {
		inner := fit.textRectPt(i) - 2*valueChainArrowInsetPt
		lines := paragraphLines(ctx, sizedPara{text: strings.TrimSpace(s.Label), sizePt: fit.labelPt, bold: true}, inner+2*sizingInsetLRPt)
		h = math.Max(h, math.Ceil(float64(lines)*fit.labelPt*sizingLineSpacing+2*valueChainArrowInsetPt))
	}
	return h
}

// cycleIntakeDescText is an intake description: muted, centred under its
// arrow, top-anchored.
func cycleIntakeDescText(description string, sizePt float64) json.RawMessage {
	inset := [4]float64{cycleIntakeDescInset, cycleIntakeDescInset, cycleIntakeDescInset, cycleIntakeDescInset}
	return ringTextJSON("ctr", "t", &inset, ringPara{Content: pptx.ConvertMarkdownEmphasis(description), Size: sizePt, Color: "dk1", Alpha: ringMutedAlpha})
}

// placeRing sets the ring square (side, top) and the loop's geometry: phase 1
// begins at startDeg and the loop runs clockwise.
func (l *cycleIntakeLayout) placeRing(ctx ExpandContext, v *CycleIntakeValues, side, top, startDeg float64) error {
	n := len(v.Loop)
	l.side = math.Max(math.Min(side, math.Min(l.w, l.h)), 1)
	l.ringY = top
	label := v.centre()
	if l.nodes {
		l.dia = ringNodeDiameter(n)
		l.spec = newRingNodeSpec(n, l.dia, true)
		l.spec.StartDeg = startDeg
		nodes, err := l.spec.nodes(l.dia, ringNodeClearDeg)
		if err != nil {
			return fmt.Errorf("cycle-intake: %w", err)
		}
		l.ringNodes = nodes
		l.centreFr = ringNodeCentreFrame(l.spec, l.dia/2, cycleNodesCentreMargin)
		l.centrePt, l.centreFits = cycleNodesFloorPt, true
		if label != "" {
			l.centrePt, l.centreFits = ringFitCircleLabel(ctx, label, []float64{scaleSubheadPt, scaleBodyPt}, l.centreFr.W*l.side, cycleNodesCentreInsetPt, cycleNodesCentreLines)
		}
		return nil
	}
	spec := newRingSpec(n)
	spec.StartDeg = startDeg
	spec = spec.withBand(1, ringBandFrac(ringDefaultThickness, l.side))
	items, err := spec.items()
	if err != nil {
		return fmt.Errorf("cycle-intake: %w", err)
	}
	l.spec, l.items = spec, items
	l.badgeDia = ringBadgeDia(spec.Thickness, l.side)
	l.centrePt, l.centreFits = ringCentreFit(ctx, spec, l.side, label, "")
	return nil
}

// placeList measures the numbered list (cycle-ring's legend rows) in the
// strip [top, bottom] with labels labelW wide, and reports whether the list
// keeps one pitch with nothing cut or left off.
//
// The rows share one pitch wherever the strip holds every row at the height
// of the tallest, so the numbers step down evenly: at the default label size
// and row gap first, then at the tighter gap and the 12pt floor. Only a list
// that holds its rows at their own heights and no more keeps them (uneven),
// and one that does not hold them at all is cut as cycle-ring's legend is.
func (l *cycleIntakeLayout) placeList(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides, labelW, top, bottom float64) bool {
	l.listY = top
	l.list = cycleRingLayout{
		w: l.w, h: math.Max(bottom-top, 1),
		descPt:   l.descPt,
		showDesc: true,
		labelW:   math.Max(labelW, 1),
	}
	sizes := []float64{shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, cycleIntakeLabelPt))}
	if ovr.HeaderSize == 0 {
		sizes = append(sizes, shapegrid.MinTextSizePt)
	}
	phases := v.phases()
	if cycleIntakeEvenRows(ctx, phases, &l.list, sizes) {
		return true
	}
	cycleRingLegendRows(ctx, phases, &l.list, sizes)
	if !l.list.showDesc {
		// The descriptions are left off: the labels alone keep one pitch where
		// the strip holds them (at the 12pt floor when one wraps above it).
		cycleIntakeEvenRows(ctx, phases, &l.list, sizes)
	}
	return false
}

// cycleIntakeEvenRows stacks one row per phase at one pitch, every row as
// tall as the tallest, the block centred on the strip. It tries each label
// size at the default and then the tighter row gap, and reports false (the
// layout's rows untouched) when the strip holds none of them.
func cycleIntakeEvenRows(ctx ExpandContext, v *CycleRingValues, lay *cycleRingLayout, sizes []float64) bool {
	n := len(v.Phases)
	try := *lay
	for _, size := range sizes {
		try.labelPt = size
		need := try.needs(ctx, v, try.labelW)
		tallest := 0.0
		for _, x := range need {
			tallest = math.Max(tallest, x)
		}
		for _, gap := range []float64{ctx.Gap(cycleRingRowGapPt), ctx.Gap(cycleRingMinRowGapPt)} {
			total := float64(n)*tallest + float64(n-1)*gap
			if total > try.h+0.01 {
				continue
			}
			y := math.Max((try.h-total)/2, 0)
			try.rows = make([]cycleRingRow, n)
			for i := range try.rows {
				try.rows[i] = cycleRingRow{side: ringSideRight, y0: y, y1: math.Min(y+tallest, try.h), need: need[i],
					numX0: try.w - try.labelW - ringNumberColPt, labelX0: try.w - try.labelW, labelX1: try.w}
				y += tallest + gap
			}
			*lay = try
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports copy over the budget of its (intake, loop)
// combination, an intake label with a word its arrow cannot hold, intake
// descriptions that outgrow the room under the lane or are left off in the
// stacked layout, list rows that cannot hold their text, loop descriptions the
// list left off and a centre label that does not fit the ring.
func (p *cycleIntake) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*CycleIntakeValues)
	if !ok || v == nil || !cycleIntakeCountsOK(v) {
		return nil
	}
	lay, err := cycleIntakeMeasure(ctx, v, cycleIntakeOverrides(overrides))
	if err != nil {
		return nil
	}
	nI, nL := len(v.Intake), len(v.Loop)
	var warnings []string

	for i, s := range v.Intake {
		if slices.Contains(lay.unfit, strings.TrimSpace(s.Label)) {
			warnings = append(warnings, fmt.Sprintf("%s: cycle-intake intake[%d].label has a word wider than its %.0fpt arrow at %.0fpt, so the word breaks; with %d intake steps keep each word to about %d letters — use shorter words or fewer intake steps",
				ErrCodeBodyTooLong, i, lay.fit.colWPt, lay.fit.labelPt, nI, cycleIntakeWordLetters(lay.fit)))
		}
		if !lay.showDesc {
			continue
		}
		if need := lay.descNeed[i]; need > lay.descRoom+0.01 {
			warnings = append(warnings, fmt.Sprintf("%s: cycle-intake intake[%d].description needs %.0fpt but only %.0fpt is free under the intake lane in a %.0f x %.0fpt content area — shorten it or drop the intake descriptions",
				ErrCodeBodyTooLong, i, need, lay.descRoom, lay.w, lay.h))
		}
	}
	if !lay.showDesc && v.hasStepDescriptions() {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-intake intake[].description is left off: a %.0f x %.0fpt content area stacks the intake lane above the loop — give the pattern the full slide width or drop the intake descriptions",
			ErrCodeBodyTooLong, lay.w, lay.h))
	}

	label, desc := cycleIntakeLabelBudget(nI, nL), cycleIntakeDescBudget(nI, nL)
	for i, ph := range v.Loop {
		named := false
		if c := runeLen(ph.Label); c > label {
			named = true
			warnings = append(warnings, fmt.Sprintf("%s: cycle-intake loop[%d].label is %d characters; with %d intake steps and %d phases use about %d — shorten the label or use fewer phases",
				ErrCodeBodyTooLong, i, c, nI, nL, label))
		}
		if c := runeLen(ph.Description); desc > 0 && c > desc {
			named = true
			warnings = append(warnings, fmt.Sprintf("%s: cycle-intake loop[%d].description is %d characters; with %d intake steps and %d phases use about %d — shorten the description or use fewer phases",
				ErrCodeBodyTooLong, i, c, nI, nL, desc))
		}
		if row := lay.list.rows[i]; row.capped && !named {
			field := "label"
			if lay.list.showDesc && strings.TrimSpace(ph.Description) != "" {
				field = "description"
			}
			warnings = append(warnings, fmt.Sprintf("%s: cycle-intake loop[%d].%s needs %.0fpt but its list row is %.0fpt tall in a %.0f x %.0fpt content area with %d phases — shorten it, drop the descriptions or use fewer phases",
				ErrCodeBodyTooLong, i, field, row.need, row.y1-row.y0, lay.w, lay.h, nL))
		}
	}
	if !lay.list.showDesc {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-intake loop[].description is left off: the list beside the ring in a %.0f x %.0fpt content area holds the %d labels only — drop the descriptions or use fewer phases",
			ErrCodeBodyTooLong, lay.w, lay.h, nL))
	}
	if v.centre() != "" && !lay.centreFits {
		warnings = append(warnings, fmt.Sprintf("%s: cycle-intake center.label does not fit inside the %.0fpt loop at 12pt (a word breaks or the text runs past it) — keep it to about two short words or drop it",
			ErrCodeBodyTooLong, lay.side))
	}
	return warnings
}

// cycleIntakeWordLetters is about how many letters a word may have to stay
// whole in the narrowest arrow of the lane at its shallowest point.
func cycleIntakeWordLetters(fit valueChainArrowFit) int {
	fit.notchPt = valueChainMinNotchPt
	return max(int((fit.textRectPt(1)-2*valueChainArrowInsetPt)/(sizingCapacityEm*fit.labelPt*1.15)), 1)
}

func cycleIntakeCountsOK(v *CycleIntakeValues) bool {
	return len(v.Intake) >= cycleIntakeMinSteps && len(v.Intake) <= cycleIntakeMaxSteps &&
		len(v.Loop) >= ringMinItems && len(v.Loop) <= ringMaxItems
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *cycleIntake) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*CycleIntakeValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("cycle-intake: values must be *CycleIntakeValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*CycleIntakeOverrides); !ok {
			return nil, fmt.Errorf("cycle-intake: overrides must be *CycleIntakeOverrides, got %T", overrides)
		}
	}
	if !cycleIntakeCountsOK(v) {
		return nil, fmt.Errorf("cycle-intake: %d intake steps and %d loop phases; the pattern holds %d-%d and %d-%d",
			len(v.Intake), len(v.Loop), cycleIntakeMinSteps, cycleIntakeMaxSteps, ringMinItems, ringMaxItems)
	}
	ovr := cycleIntakeOverrides(overrides)
	lay, err := cycleIntakeMeasure(ctx, v, ovr)
	if err != nil {
		return nil, err
	}
	base := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)

	// Intake lane: neutral arrows; the entry arrow is the accent cue.
	var places []ringPlacement
	stepW := lay.fit.colWPt
	laneH := lay.lane.y1 - lay.lane.y0
	for i, cell := range cycleIntakeLaneCells(ctx, v, lay.fit, base) {
		cell.Shape.Adjustments = map[string]int64{"adj": lay.fit.adj(i, laneH)}
		x0 := lay.laneX0 + float64(i)*(stepW+valueChainArrowGapPt)
		places = append(places, ringPlacement{X0: x0, X1: x0 + stepW, Y0: lay.lane.y0, Y1: lay.lane.y1, Cell: cell})
		d := strings.TrimSpace(v.Intake[i].Description)
		if !lay.showDesc || d == "" || lay.descRoom < 1 {
			continue
		}
		places = append(places, ringPlacement{X0: x0, X1: x0 + stepW, Y0: lay.descY0, Y1: lay.descY0 + math.Min(lay.descNeed[i], lay.descRoom),
			Cell: ringLabelTextCell(cycleIntakeDescText(d, lay.descPt))})
	}
	entry := &jsonschema.ShapeSpecInput{
		Geometry:    "rightArrow",
		Fill:        accentFillJSON(base),
		Line:        noLine,
		Adjustments: map[string]int64{"adj1": cycleIntakeArrowShaftAdj, "adj2": cycleIntakeArrowHeadAdj},
	}
	if lay.entryDown {
		entry.Geometry = "downArrow"
	}
	places = append(places, ringPlacement{X0: lay.entry.x0, X1: lay.entry.x1, Y0: lay.entry.y0, Y1: lay.entry.y1, Cell: &jsonschema.GridCellInput{Shape: entry}})

	// The loop.
	ring, accents := cycleIntakeRing(ctx, v, ovr, lay, base)
	places = append(places, ringPlacement{X0: lay.ringX, X1: lay.ringX + lay.side, Y0: lay.ringY, Y1: lay.ringY + lay.side, Cell: ring})

	// The numbered list.
	for i, ph := range v.Loop {
		row := lay.list.rows[i]
		desc := ""
		if lay.list.showDesc {
			desc = strings.TrimSpace(ph.Description)
		}
		y0, y1 := lay.listY+row.y0, lay.listY+row.y1
		// Right of the ring: numeral, then the label. Left of it (stacked),
		// mirrored, so the numeral stays on the ring's side.
		align := "l"
		numX0, labelX0 := lay.listX0, lay.listX0+ringNumberColPt
		if lay.stacked {
			align = "r"
			numX0, labelX0 = lay.listX0+lay.list.labelW, lay.listX0
		}
		places = append(places,
			ringPlacement{X0: numX0, X1: numX0 + ringNumberColPt, Y0: y0, Y1: y1,
				Cell: ringNumberCell(ctx, i+1, accents[i], lay.list.labelPt, align)},
			ringPlacement{X0: labelX0, X1: labelX0 + lay.list.labelW, Y0: y0, Y1: y1,
				Cell: ringLabelTextCell(ringLabelText(strings.TrimSpace(ph.Label), desc, lay.list.labelPt, lay.list.descPt, align))},
		)
	}

	grid, err := ringLattice(places, lay.w, lay.h)
	if err != nil {
		return nil, fmt.Errorf("cycle-intake: %w", err)
	}
	grid.VerticalAlign = "middle"
	return grid, nil
}

// cycleIntakeRing is the loop's cell and the accent of each phase (its number
// in the list): cycle-ring's segments and badges, or cycle-nodes' circles and
// link arrows.
func cycleIntakeRing(ctx ExpandContext, v *CycleIntakeValues, ovr *CycleIntakeOverrides, lay cycleIntakeLayout, base string) (*jsonschema.GridCellInput, []string) {
	n := len(v.Loop)
	accents := make([]string, n)
	for i := range accents {
		accents[i] = ctx.ResolveCellAccent(base, i, ovr.CellAccentMode)
	}
	label := v.centre()

	if lay.nodes {
		paints := cycleNodesPaints(ctx,
			&CycleNodesValues{Steps: make([]CycleNodesStep, n), Highlight: v.highlight() + 1},
			&CycleNodesOverrides{TextOverrides: ovr.TextOverrides}, base)
		numeralPt := cycleNodesNumeralPt(lay.dia * lay.side)
		ring := &jsonschema.GridCellInput{Fit: "contain"}
		ring.Layers = append(ring.Layers, ringLinkArrowLayers(lay.spec, lay.ringNodes, ringLinkFillJSON())...)
		ring.Layers = append(ring.Layers, ringNodeLayers(lay.ringNodes, func(i int) ringNodePaint {
			return ringNodePaint{Fill: paints[i].fill, Text: ringNodeTextJSON("ctr", "ctr", -1,
				ringNodePara{Content: strconv.Itoa(i + 1), Size: numeralPt, Bold: true, Color: paints[i].ink})}
		})...)
		if label != "" {
			ring.Layers = append(ring.Layers, ringCentreLabelLayer(lay.centreFr, ringNodeTextJSON("ctr", "ctr", cycleNodesCentreInsetPt,
				ringNodePara{Content: pptx.ConvertMarkdownEmphasis(label), Size: lay.centrePt, Bold: true, Color: "dk1"})))
		}
		return ring, accents
	}

	paint := ringPaint{
		Accents:   accents,
		Tinted:    ovr.CellAccentMode == "alternate" || ovr.CellAccentMode == "progressive",
		Highlight: v.highlight(),
		BadgeDia:  lay.badgeDia,
	}
	layers := [][]jsonschema.LayerInput{
		ringSegmentLayers(ctx, lay.spec, lay.items, paint),
		ringBadgeLayers(ctx, lay.spec, lay.items, paint),
	}
	if centre, ok := ringCentreLayer(lay.spec, label, "", lay.centrePt, ""); ok {
		layers = append(layers, []jsonschema.LayerInput{centre})
	}
	return ringCell(layers...), paint.labelAccents(ctx, n)
}
