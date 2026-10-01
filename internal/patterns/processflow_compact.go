package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// process-flow-compact — same as process-flow but height-capped at 35%
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processFlowCompact{})
}

type processFlowCompact struct{}

func (p *processFlowCompact) Name() string { return "process-flow-compact" }
func (p *processFlowCompact) Description() string {
	return "Compact left-to-right process flow, height-capped for short content"
}
func (p *processFlowCompact) UseWhen() string {
	return "3-8 short-label steps where the process is supporting context (not the hero content); prefer full process-flow when steps have long labels or fill the slide"
}
func (p *processFlowCompact) NotWhen() string {
	return "Steps have long labels needing vertical space (use process-flow), or steps belong to different actors (use swimlane)"
}
func (p *processFlowCompact) Version() int      { return 1 }
func (p *processFlowCompact) CellsHint() string { return "3-8" }
func (p *processFlowCompact) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "pull-quote"},
		DensityClass:       "low",
		AccentWeight:       "normal",
		SparseThresholdPct: 25,
	}
}
func (p *processFlowCompact) SupportsCallout() bool        { return true }
func (p *processFlowCompact) SupportsInlineMarkdown() bool { return true }

func (p *processFlowCompact) ExemplarValues() any {
	return &ProcessFlowValues{
		Steps: []ProcessFlowStep{
			{Label: "Request", Type: "step"},
			{Label: "Review", Type: "decision"},
			{Label: "Approve", Type: "step"},
			{Label: "Deploy", Type: "step"},
		},
	}
}

// Reuse types from process-flow.
func (p *processFlowCompact) NewValues() any       { return &ProcessFlowValues{} }
func (p *processFlowCompact) NewOverrides() any    { return &ProcessFlowOverrides{} }
func (p *processFlowCompact) NewCellOverride() any { return &ProcessFlowCellOverride{} }

func processFlowCompactLabelBudget(steps int, pointed bool) (wordLike, unbroken int) {
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
	switch {
	case steps == 5:
		return 80, 71
	case steps == 6:
		return 76, 58
	case steps == 7:
		return 52, 49
	case steps >= 8:
		return 51, 42
	}
	return 80, 80
}

func (p *processFlowCompact) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ProcessFlowValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	for i, step := range v.Steps {
		pointed := step.Type == "chevron" || step.Type == "arrow"
		wordBudget, unbrokenBudget := processFlowCompactLabelBudget(len(v.Steps), pointed)
		longest := 0
		for _, word := range strings.Fields(step.Label) {
			longest = max(longest, runeLen(word))
		}
		if runeLen(step.Label) > wordBudget || longest > unbrokenBudget {
			warnings = append(warnings, fmt.Sprintf("%s: process-flow-compact steps[%d].label has %d characters (longest unbroken run %d); this %d-step %s holds about %d word-like or %d wide unbroken characters — shorten the label, add word breaks, or use fewer steps", ErrCodeBodyTooLong, i, runeLen(step.Label), longest, len(v.Steps), step.Type, wordBudget, unbrokenBudget))
		}
	}
	if len(warnings) == 0 {
		warnings = processFlowAreaWarning(ctx, "process-flow-compact", v.Steps, overrides, true)
	}
	return warnings
}

func (p *processFlowCompact) Schema() *Schema {
	stepSchema := processFlowStepSchema("Step label text; compact chevron/arrow labels tighten to about 61/31/12/10 word-like characters at 5/6/7/8 steps (rectangular steps about 76/52/51 at 6/7/8), less for wide unbroken text")

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
	}).WithDescription("Compact left-to-right process flow, height-capped at ~35% of content area")
}

func (p *processFlowCompact) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ProcessFlowValues)
	if !ok || vals == nil {
		return fmt.Errorf("process-flow-compact: values must be *ProcessFlowValues, got %T", values)
	}

	const name = "process-flow-compact"
	var errs []error

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
				fmt.Sprintf("process-flow-compact: steps[%d].type must be \"step\", \"decision\", \"chevron\", or \"arrow\", got %q", i, step.Type),
				UseOneOfFix(fmt.Sprintf("steps[%d].type", i), []string{"step", "decision", "chevron", "arrow"})))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// processFlowCompactHeightPct is the largest share of the content area the
// compact band occupies. Pointed steps can reduce it further.
const processFlowCompactHeightPct = 22.0

// processFlowCompactBoxAspect is the compact step's height floor as a share
// of its width — shallower than process-flow's processFlowBoxAspect.
const processFlowCompactBoxAspect = 0.28

// processFlowCompactCellSize returns the compact step width and band height in
// points. Pointed presets need a shallow band so their own text rectangle
// retains useful width after the point and notch.
func processFlowCompactCellSize(ctx ExpandContext, steps int, pointed bool) (width, height float64) {
	if steps < 1 {
		steps = 1
	}
	contentW, contentH := contentAreaPt(ctx)
	width = (contentW - processFlowGapPt*float64(steps-1)) / float64(steps)
	height = contentH * processFlowCompactHeightPct / 100
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	if pointed {
		height = math.Min(height, width*chevronMaxAspectH)
	}
	return width, height
}

func (p *processFlowCompact) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ProcessFlowValues)
	if !ok {
		return nil, fmt.Errorf("process-flow-compact: values must be *ProcessFlowValues, got %T", values)
	}
	ovr := &ProcessFlowOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ProcessFlowOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("process-flow-compact: overrides must be *ProcessFlowOverrides, got %T", overrides)
		}
	}

	bodySize := ResolveSize(ovr.BodySize, processFlowDefaultFontPt(len(vals.Steps)))

	pointedRow := false
	for _, step := range vals.Steps {
		if step.Type == "chevron" || step.Type == "arrow" {
			pointedRow = true
			break
		}
	}

	cells := buildProcessFlowCells(ctx, vals.Steps, ovr, cellOverrides, bodySize)

	colsJSON, _ := json.Marshal(len(vals.Steps))

	row := jsonschema.GridRowInput{
		Cells:     cells,
		Connector: processFlowConnector(ctx, ovr),
	}
	if allStepsPointed(vals.Steps) {
		row.Connector = nil
	}

	cellW, bandCap := processFlowCompactCellSize(ctx, len(vals.Steps), pointedRow)
	_, contentHeight := contentAreaPt(ctx)
	// The band is content-sized below its cap, shallower than process-flow's
	// steps (go-slide-creator-xb06p). The cap gives way to the written fit of
	// the tallest label (never past the content area) before the writer would
	// shrink it below the readable floor (go-slide-creator-n1muf).
	need := processFlowWrittenNeedPt(cells, cellW)
	bandHeight := processFlowContentHeight(need, cellW, processFlowCompactBoxAspect, bandCap)
	bandHeight = math.Max(bandHeight, math.Min(need, contentHeight))
	bandHeightPct := processFlowCompactHeightPct
	if contentHeight > 0 {
		bandHeightPct = bandHeight / contentHeight * 100
	}
	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: bandHeightPct,
		},
		Columns: json.RawMessage(colsJSON),
		Gap:     processFlowGapPt,
		Rows:    []jsonschema.GridRowInput{row},
		// The compact band is supporting context: it sits under the title
		// and leaves the space below for other content, instead of floating
		// a shallow strip in the middle of the slide (go-slide-creator-xb06p).
		VerticalAlign: "top",
	}

	return grid, nil
}
