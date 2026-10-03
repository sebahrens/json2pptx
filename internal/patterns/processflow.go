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
// process-flow pattern — left-to-right rectangles and diamonds with arrows
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processFlow{})
}

type processFlow struct{}

func (p *processFlow) Name() string { return "process-flow" }
func (p *processFlow) Description() string {
	return "Left-to-right process flow with steps and decision points"
}
func (p *processFlow) UseWhen() string {
	return "Sequential steps in a single-lane workflow (3-8 steps); prefer swimlane when multiple actors own different steps, timeline-horizontal when stops are date-based"
}
func (p *processFlow) NotWhen() string {
	return "Steps belong to different actors/roles (use swimlane), stops are calendar-based milestones (use timeline-horizontal), or items are unordered (use icon-row or card-grid)"
}
func (p *processFlow) Version() int      { return 1 }
func (p *processFlow) CellsHint() string { return "3-8" }
func (p *processFlow) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "before-after"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (p *processFlow) SupportsCallout() bool        { return true }
func (p *processFlow) SupportsInlineMarkdown() bool { return true }

func (p *processFlow) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 1},
		{Columns: 4, Rows: 1},
		{Columns: 5, Rows: 1},
		{Columns: 6, Rows: 1},
		{Columns: 7, Rows: 1},
		{Columns: 8, Rows: 1},
	}
}

func (p *processFlow) ExemplarValues() any {
	return &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Request", Type: "step"},
			{Label: "Review", Type: "decision"},
			{Label: "Approve", Type: "step"},
			{Label: "Deploy", Type: "step"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// processFlowDefaultFontPt returns the default step-label font size, shrinking
// it as the step count grows so 3-8 short labels stay on one line inside the
// progressively narrower boxes instead of wrapping awkwardly. n<=4 keeps the
// historical 12pt default; an explicit body_size override always wins via
// ResolveSize. Kept in lockstep between process-flow and process-flow-compact.
func processFlowDefaultFontPt(n int) float64 {
	switch {
	case n >= 7:
		return 9
	case n == 6:
		return 10
	case n == 5:
		return 11
	default:
		return 12
	}
}

// ProcessFlowStep is a single step in the process flow.
type ProcessFlowStep struct {
	Label string `json:"label"`
	Type  string `json:"type,omitempty"` // "step" (default), "decision", "chevron", or "arrow"
	// Highlight fills this one step with the solid accent (at most one).
	Highlight bool `json:"highlight,omitempty"`
}

// ProcessFlowValues holds the steps for the process flow.
type ProcessFlowValues struct {
	Steps []ProcessFlowStep `json:"steps"`
}

// ProcessFlowOverrides is the standard text overrides plus the step style.
// header_size is not supported (step labels are body text) and is rejected
// by Validate.
type ProcessFlowOverrides struct {
	TextOverrides
	// Style is "tinted" (default: neutral steps with dark text; the accent
	// fills only a highlighted step or a lone decision, and draws the
	// connectors) or "solid" (every step filled with the accent; legacy).
	Style string `json:"style,omitempty"`
}

// processFlowStyles are the accepted overrides.style values.
var processFlowStyles = []string{"tinted", "solid"}

// ProcessFlowCellOverride is the shared per-cell override.
type ProcessFlowCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *processFlow) NewValues() any       { return &ProcessFlowValues{} }
func (p *processFlow) NewOverrides() any    { return &ProcessFlowOverrides{} }
func (p *processFlow) NewCellOverride() any { return &ProcessFlowCellOverride{} }

func processFlowLabelBudget(steps int, pointed bool) (wordLike, unbroken int) {
	// Measured against the written size on every shipped template, every
	// shape keeping the uniform 0.5 cm text margin (go-slide-creator-n1muf).
	if pointed {
		switch steps {
		case 4:
			return 80, 69
		case 5:
			return 61, 41
		case 6:
			return 31, 22
		case 7:
			return 12, 9
		case 8:
			return 10, 8
		default:
			return 80, 80
		}
	}
	if steps == 7 {
		return 72, 68
	}
	if steps >= 8 {
		return 71, 59
	}
	return 80, 80
}

func (p *processFlow) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ProcessFlowValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	for i, step := range v.Steps {
		pointed := step.Type == "chevron" || step.Type == "arrow"
		wordBudget, unbrokenBudget := processFlowLabelBudget(len(v.Steps), pointed)
		longest := 0
		for _, word := range strings.Fields(step.Label) {
			longest = max(longest, runeLen(word))
		}
		if runeLen(step.Label) > wordBudget || longest > unbrokenBudget {
			warnings = append(warnings, fmt.Sprintf("%s: process-flow steps[%d].label has %d characters (longest unbroken run %d); this %d-step %s holds about %d word-like or %d wide unbroken characters — shorten the label, add word breaks, or use fewer steps", ErrCodeBodyTooLong, i, runeLen(step.Label), longest, len(v.Steps), step.Type, wordBudget, unbrokenBudget))
		}
	}
	if len(warnings) == 0 {
		warnings = processFlowAreaWarning(ctx, "process-flow", v.Steps, overrides, false)
	}
	return warnings
}

func (p *processFlow) Schema() *Schema {
	stepSchema := processFlowStepSchema("Step label text; chevron/arrow labels tighten to about 61/31/12/10 word-like characters at 5/6/7/8 steps (rectangular steps about 72 at 7 and 71 at 8), less for wide unbroken text")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"steps": ArraySchema(stepSchema, 3, 8).WithDescription("Process steps left-to-right (3-8)"),
		},
		[]string{"steps"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      processFlowOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Left-to-right process flow with steps and decision points")
}

func (p *processFlow) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ProcessFlowValues)
	if !ok || vals == nil {
		return fmt.Errorf("process-flow: values must be *ProcessFlowValues, got %T", values)
	}

	const name = "process-flow"
	var errs []error

	// Validate cell_accent_mode
	errs = append(errs, validateProcessFlowStyle(name, vals.Steps, overrides)...)

	if len(vals.Steps) < 3 {
		errs = append(errs, errMinItems(name, "steps", 3, len(vals.Steps), ""))
	}
	if len(vals.Steps) > 8 {
		errs = append(errs, errMaxItems(name, "steps", 8, len(vals.Steps), ""))
	}

	for i, step := range vals.Steps {
		path := fmt.Sprintf("steps[%d].label", i)
		if step.Label == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(step.Label) > 80 {
			errs = append(errs, errMaxLength(name, path, 80, runeLen(step.Label)))
		}
		if step.Type != "" && step.Type != "step" && step.Type != "decision" && step.Type != "chevron" && step.Type != "arrow" {
			errs = append(errs, newValidationError(name, fmt.Sprintf("steps[%d].type", i), ErrCodeUnknownEnum,
				fmt.Sprintf("process-flow: steps[%d].type must be \"step\", \"decision\", \"chevron\", or \"arrow\", got %q", i, step.Type),
				UseOneOfFix(fmt.Sprintf("steps[%d].type", i), []string{"step", "decision", "chevron", "arrow"})))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (p *processFlow) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ProcessFlowValues)
	if !ok {
		return nil, fmt.Errorf("process-flow: values must be *ProcessFlowValues, got %T", values)
	}
	ovr := &ProcessFlowOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ProcessFlowOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("process-flow: overrides must be *ProcessFlowOverrides, got %T", overrides)
		}
	}

	bodySize := ResolveSize(ovr.BodySize, processFlowFullFontPt(vals.Steps))
	cells := buildProcessFlowCells(ctx, vals.Steps, ovr, cellOverrides, bodySize)

	// Steps are capped at processFlowMaxHeightFrac of the content height
	// (go-slide-creator-7km8) instead of stretching into full-height pillars
	// with needle-thin diamonds; the grid centres the row vertically.
	pointedRow := allStepsPointed(vals.Steps)
	gap := processFlowStepGapPt(ctx, vals.Steps)
	cellW, rowCap := processFlowCellSize(ctx, len(vals.Steps), gap, pointedRow)
	colsJSON, widths := processFlowColumns(ctx, vals.Steps, bodySize, cellW)
	// Steps are content-sized: the written fit of the tallest label, floored
	// at a box proportion so a one-word step still reads as a box, and capped
	// at processFlowMaxHeightFrac (go-slide-creator-xb06p). The cap gives way
	// to the written fit of the tallest label (never past the content area)
	// before the writer would shrink it below the readable floor
	// (go-slide-creator-n1muf).
	_, contentH := contentAreaPt(ctx)
	need := processFlowWrittenNeedPt(ctx.themeFonts(), cells, widths)
	rowHeight := processFlowContentHeight(need, cellW, processFlowBoxAspect, rowCap)
	rowHeight = math.Max(rowHeight, math.Min(need, math.Round(contentH)))
	row := jsonschema.GridRowInput{
		Cells:     cells,
		Connector: processFlowConnector(ctx, ovr),
		MaxHeight: rowHeight,
	}
	// Chevrons point at the next step; an arrow drawn between them is a second
	// statement of the same thing, and it was being drawn straight through the
	// notch (go-slide-creator-czk4).
	if pointedRow {
		row.Connector = nil
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       colsJSON,
		Gap:           gap,
		Rows:          []jsonschema.GridRowInput{row},
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

// processFlowWrittenNeedPt is the height the tallest step needs for the
// writer to store its label without an autofit shrink: the written fit of
// each step's own text at the step width. A diamond's text sits in the
// preset's inner half-size rectangle (pptx.PresetTextRectSize), so a decision
// needs twice the fit of its label at half the step width; measured at the
// full width, a content-sized row left "Within policy?" to be shrunk by the
// renderer (go-slide-creator-xb06p).
func processFlowWrittenNeedPt(fonts pptx.ThemeFonts, cells []*jsonschema.GridCellInput, widths []float64) float64 {
	need := 0.0
	for i, c := range cells {
		if c == nil || c.Shape == nil || i >= len(widths) {
			continue
		}
		cellW := widths[i]
		if c.Shape.Geometry == "diamond" {
			need = math.Max(need, 2*writtenFitHeightPt(fonts, c.Shape.Text, cellW/2, 0))
			continue
		}
		need = math.Max(need, writtenFitHeightPt(fonts, c.Shape.Text, cellW, 0))
	}
	return math.Ceil(need)
}

// processFlowStepsNeedPt builds each step's label cell as Expand writes it and
// returns the written-fit height the tallest needs, with the step width and
// the content-area height.
func processFlowStepsNeedPt(ctx ExpandContext, steps []ProcessFlowStep, bodySize float64, compact bool) (need, areaH float64) {
	cells := buildProcessFlowCells(ctx, steps, &ProcessFlowOverrides{}, nil, bodySize)
	var cellW float64
	gap := processFlowStepGapPt(ctx, steps)
	if compact {
		cellW, _ = processFlowCompactCellSize(ctx, len(steps), gap, false)
	} else {
		cellW, _ = processFlowCellSize(ctx, len(steps), gap, false)
	}
	_, areaH = contentAreaPt(ctx)
	_, widths := processFlowColumns(ctx, steps, bodySize, cellW)
	return processFlowWrittenNeedPt(ctx.themeFonts(), cells, widths), areaH
}

// processFlowAreaWarning reports steps whose written fit needs more height
// than the template's content area holds (go-slide-creator-n1muf).
func processFlowAreaWarning(ctx ExpandContext, name string, steps []ProcessFlowStep, overrides any, compact bool) []string {
	if len(steps) == 0 {
		return nil
	}
	ovr, _ := overrides.(*ProcessFlowOverrides)
	if ovr == nil {
		ovr = &ProcessFlowOverrides{}
	}
	size := processFlowFullFontPt(steps)
	if compact {
		size = processFlowDefaultFontPt(len(steps))
	}
	bodySize := ResolveSize(ovr.BodySize, size)
	gap := processFlowStepGapPt(ctx, steps)
	var equalW float64
	if compact {
		equalW, _ = processFlowCompactCellSize(ctx, len(steps), gap, false)
	} else {
		equalW, _ = processFlowCellSize(ctx, len(steps), gap, false)
	}
	_, widths := processFlowColumns(ctx, steps, bodySize, equalW)
	if broken := processFlowBrokenDecisionWords(ctx, name, steps, bodySize, widths); len(broken) > 0 {
		return broken
	}
	// The height check needs the template's real content area.
	if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return nil
	}
	need, areaH := processFlowStepsNeedPt(ctx, steps, bodySize, compact)
	if need <= areaH+1 {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s step labels need %.0fpt at readable sizes but the content area holds about %.0fpt — shorten the labels or use fewer steps", ErrCodeBodyTooLong, name, need, areaH)}
}

// Decision width (go-slide-creator-66ojb). A diamond's text rectangle is the
// middle half of its width, so in a row of equal columns a decision holds a
// word half as long as its neighbours do — "Approved?" broke after "Approv"
// in an eight-step flow. A decision whose longest word needs more takes a
// wider column, and the other steps give up at most
// processFlowDecisionMaxGiveFrac of their width for it.
const processFlowDecisionMaxGiveFrac = 0.25

// processFlowDecisionNeedPt is the column width a decision needs for its
// longest word to stay whole inside the diamond's text rectangle with the
// diamond text margin; fits is false when the word cannot be measured.
func processFlowDecisionNeedPt(label, font string, sizePt float64) (need float64, fits bool) {
	// The renderer raises a label below its readable floor back to the floor.
	sizePt = shapegrid.EffectiveTextSizePt(sizePt)
	widest := int64(0)
	for _, word := range strings.Fields(label) {
		w, ok := pptx.WordLineNeedEMU(word, font, sizePt, true, 0)
		if !ok {
			return 0, false
		}
		widest = max(widest, w)
	}
	return 2 * (float64(widest)/sizingEMUPerPt + 2*processFlowDiamondInsetPt), true
}

// processFlowColumns returns the grid columns and every step's width in
// points. Columns are equal (the plain step count) unless a decision's
// longest word needs a wider diamond; then the columns are percentages, the
// decisions as wide as they need and the other steps sharing the rest, never
// narrower than 1-processFlowDecisionMaxGiveFrac of the equal width.
func processFlowColumns(ctx ExpandContext, steps []ProcessFlowStep, sizePt, equalW float64) (json.RawMessage, []float64) {
	n := len(steps)
	widths := make([]float64, n)
	for i := range widths {
		widths[i] = equalW
	}
	equal, _ := json.Marshal(n)
	if n == 0 || equalW <= 0 {
		return equal, widths
	}
	font := ctx.Theme.BodyFont
	extra := make([]float64, n)
	totalExtra, others := 0.0, 0
	for i, st := range steps {
		if st.Type != "decision" {
			others++
			continue
		}
		if need, ok := processFlowDecisionNeedPt(pptx.ConvertMarkdownEmphasis(st.Label), font, sizePt); ok && need > equalW {
			extra[i] = need - equalW
			totalExtra += extra[i]
		}
	}
	if totalExtra == 0 || others == 0 {
		return equal, widths
	}
	give := math.Min(totalExtra, processFlowDecisionMaxGiveFrac*equalW*float64(others))
	sum := 0.0
	for i, st := range steps {
		switch {
		case extra[i] > 0:
			widths[i] = equalW + extra[i]*give/totalExtra
		case st.Type != "decision":
			widths[i] = equalW - give/float64(others)
		}
		sum += widths[i]
	}
	pcts := make([]float64, n)
	for i, w := range widths {
		pcts[i] = math.Round(w/sum*100000) / 1000
	}
	cols, _ := json.Marshal(pcts)
	return cols, widths
}

// processFlowBrokenDecisionWords reports the decisions whose longest word is
// wider than the diamond's bare text rectangle at the width it renders at —
// the words a renderer breaks mid-word.
func processFlowBrokenDecisionWords(ctx ExpandContext, name string, steps []ProcessFlowStep, sizePt float64, widths []float64) []string {
	var out []string
	for i, st := range steps {
		if st.Type != "decision" || i >= len(widths) {
			continue
		}
		need, ok := processFlowDecisionNeedPt(pptx.ConvertMarkdownEmphasis(st.Label), ctx.Theme.BodyFont, sizePt)
		if !ok || need-4*processFlowDiamondInsetPt <= widths[i] {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s steps[%d].label %q has a word wider than its decision diamond at %d steps — the renderer breaks it mid-word; shorten the label, use fewer steps, or make it a plain step", ErrCodeTextExceedsShape, name, i, st.Label, len(steps)))
	}
	return out
}

// processFlowMaxHeightFrac caps process-flow steps at this share of the
// content height. It was 0.45, which stretched one-word steps into 155px
// slabs (go-slide-creator-xb06p).
const processFlowMaxHeightFrac = 0.30

// processFlowBoxAspect is the height floor of a step as a share of its width:
// a one-word label still gets a box, not a bar. process-flow-compact uses the
// shallower processFlowCompactBoxAspect.
const processFlowBoxAspect = 0.40

// processFlowContentHeight sizes a step row to its written need, floored at
// aspect × the step width and capped at limit.
func processFlowContentHeight(need, cellW, aspect, limit float64) float64 {
	h := math.Max(need, cellW*aspect)
	if limit > 0 {
		h = math.Min(h, limit)
	}
	return math.Round(h)
}

// processFlowFullFontPt uses the taller full-size row to promote short labels.
// Dense labels keep the conservative scale; compact flows retain their own
// 9–12pt scale via processFlowDefaultFontPt.
func processFlowFullFontPt(steps []ProcessFlowStep) float64 {
	base := processFlowDefaultFontPt(len(steps))
	for _, step := range steps {
		if runeLen(step.Label) > 22 {
			return base
		}
	}
	switch {
	case len(steps) <= 4:
		return 16
	case len(steps) <= 6:
		return 13
	default:
		return base
	}
}

// processFlowGapPt is the gap between the steps of a row of chevrons or
// arrows, which draw their own direction and take no connector.
const processFlowGapPt = 12.0

// Connector geometry (go-slide-creator-66ojb). The connector between two
// steps used to be as long as the 12pt grid gap, with a 4pt arrowhead: at
// presentation size the arrows vanished and the flow read as a row of
// buttons. The step gap is now the connector's own length — a fixed number
// of points per step count, never scaled with the template gutter — and the
// arrowhead is the large preset on a 2pt line (about 10pt long and wide).
const (
	// processFlowConnectorMinPt is the shortest connector, used at 7-8
	// steps: a 10pt shaft plus the 10pt arrowhead.
	processFlowConnectorMinPt = 20.0
	// processFlowConnectorLinePt is the connector line width; with
	// processFlowConnectorHead it sets the arrowhead size.
	processFlowConnectorLinePt = 2.0
	processFlowConnectorHead   = "lg"
)

// processFlowConnectorLenPt is the connector length for a flow of n steps:
// longer where the steps can spare the width, never below
// processFlowConnectorMinPt.
func processFlowConnectorLenPt(n int) float64 {
	switch {
	case n <= 5:
		return 32
	case n == 6:
		return 26
	default:
		return processFlowConnectorMinPt
	}
}

// processFlowStepGapPt is the gap between steps: the connector length when
// connectors are drawn, the plain grid gap for a row of pointed steps.
func processFlowStepGapPt(ctx ExpandContext, steps []ProcessFlowStep) float64 {
	if allStepsPointed(steps) {
		return ctx.Gap(processFlowGapPt)
	}
	return processFlowConnectorLenPt(len(steps))
}

// processFlowCellSize is one step's width and the row's height in points.
// A row of pointed steps is additionally capped to half its step width: the
// notch is a fraction of the SHORTER side, so a tall chevron eats its own
// label — at four steps an uncapped row left 110pt of text width in a 198pt
// shape, and even "Board sign-off" broke mid-word (go-slide-creator-czk4).
func processFlowCellSize(ctx ExpandContext, steps int, gapPt float64, pointed bool) (width, height float64) {
	if steps < 1 {
		steps = 1
	}
	contentW, contentH := contentAreaPt(ctx)
	width = (contentW - gapPt*float64(steps-1)) / float64(steps)
	height = math.Round(contentH * processFlowMaxHeightFrac)
	if pointed {
		height = math.Min(height, math.Round(width*chevronMaxAspectH))
	}
	return width, height
}

// allStepsPointed reports whether every step draws its own direction — a
// chevron or a right arrow — so a connector between them says nothing new.
func allStepsPointed(steps []ProcessFlowStep) bool {
	for _, s := range steps {
		if s.Type != "chevron" && s.Type != "arrow" {
			return false
		}
	}
	return len(steps) > 0
}

// buildProcessFlowPointedText is a pointed step's label. Chevron and
// rightArrow presets already reserve their point/notch width in their text
// rectangle, so the label keeps only the uniform shape text margin inside it;
// adding the notch again as bodyPr insets leaves almost no room for text in
// narrow steps.
func buildProcessFlowPointedText(content string, size float64, ink string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: ink, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func buildProcessFlowTextContent(content string, size float64, ink string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: true, Color: ink, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
