package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
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
	if pointed {
		switch steps {
		case 5:
			return 80, 52
		case 6:
			return 41, 30
		case 7:
			return 40, 18
		case 8:
			return 35, 13
		default:
			return 80, 80
		}
	}
	if steps == 7 {
		return 80, 76
	}
	if steps >= 8 {
		return 80, 64
	}
	return 80, 80
}

func (p *processFlowCompact) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
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
	return warnings
}

func (p *processFlowCompact) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label": StringSchema(80).WithDescription("Step label text; compact chevron/arrow labels tighten to about 41/40/35 word-like characters at 6/7/8 steps, less for wide unbroken text"),
			"type":  EnumSchema("step", "decision", "chevron", "arrow").WithDescription("Shape type: rectangle (step), diamond (decision), chevron, or right-arrow (arrow)").WithDefault("step"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"steps": ArraySchema(stepSchema, 3, 8).WithDescription("Process steps left-to-right (3-8)"),
		},
		[]string{"steps"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchema(),
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

	if overrides != nil {
		if ovr, ok := overrides.(*ProcessFlowOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

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

// processFlowCompactHeightPct is the share of the content area the compact
// band occupies. The notch calculation reads it, so the two cannot drift.
const processFlowCompactHeightPct = 35.0

// processFlowCompactNotchPt is the depth of a pointed step's point in the
// compact band.
func processFlowCompactNotchPt(ctx ExpandContext, steps int) float64 {
	if steps < 1 {
		steps = 1
	}
	contentW, contentH := contentAreaPt(ctx)
	cellW := (contentW - processFlowGapPt*float64(steps-1)) / float64(steps)
	cellH := contentH * processFlowCompactHeightPct / 100
	if cellW <= 0 || cellH <= 0 {
		return 0
	}
	return float64(chevronAdj) / 100000 * math.Min(cellW, cellH)
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

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	bodySize := ResolveSize(ovr.BodySize, processFlowDefaultFontPt(len(vals.Steps)))
	cellAccentMode := ovr.CellAccentMode

	// The compact variant draws the same shapes in a shorter band, so it takes
	// the same notch treatment: text inset past the point, a shallower point,
	// and no connector when every step already points (go-slide-creator-czk4).
	notchPt := processFlowCompactNotchPt(ctx, len(vals.Steps))

	cells := make([]*jsonschema.GridCellInput, len(vals.Steps))
	for i, step := range vals.Steps {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		geometry := "roundRect"
		pointed := false
		switch step.Type {
		case "decision":
			geometry = "diamond"
		case "chevron":
			geometry = "chevron"
			pointed = true
		case "arrow":
			geometry = "rightArrow"
			pointed = true
		}

		text := buildProcessFlowTextContent(pptx.ConvertMarkdownEmphasis(step.Label), bodySize)
		if pointed {
			text = buildProcessFlowPointedText(pptx.ConvertMarkdownEmphasis(step.Label), bodySize, notchPt+chevronTextPadPt)
		}

		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: geometry,
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text,
			},
		}
		if pointed {
			cell.Shape.Adjustments = map[string]int64{"adj": chevronAdj}
		}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*ProcessFlowCellOverride); ok2 && cellOvr.AccentBar {
				cell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
		}

		cells[i] = cell
	}

	colsJSON, _ := json.Marshal(len(vals.Steps))

	row := jsonschema.GridRowInput{
		Cells:     cells,
		Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: "dk1", Width: 1.5},
	}
	if allStepsPointed(vals.Steps) {
		row.Connector = nil
	}

	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: processFlowCompactHeightPct,
		},
		Columns: json.RawMessage(colsJSON),
		Gap:     processFlowGapPt,
		Rows:    []jsonschema.GridRowInput{row},
	}

	return grid, nil
}
