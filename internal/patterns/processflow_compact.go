package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

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

func (p *processFlowCompact) Name() string        { return "process-flow-compact" }
func (p *processFlowCompact) Description() string { return "Compact left-to-right process flow, height-capped for short content" }
func (p *processFlowCompact) UseWhen() string {
	return "3-8 short-label steps where the process is supporting context (not the hero content); prefer full process-flow when steps have long labels or fill the slide"
}
func (p *processFlowCompact) NotWhen() string {
	return "Steps have long labels needing vertical space (use process-flow), or steps belong to different actors (use swimlane)"
}
func (p *processFlowCompact) Version() int     { return 1 }
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

func (p *processFlowCompact) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label": StringSchema(80).WithDescription("Step label text"),
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
		} else if len(step.Label) > 80 {
			errs = append(errs, errMaxLength(name, path, 80, len(step.Label)))
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

	cells := make([]*jsonschema.GridCellInput, len(vals.Steps))
	for i, step := range vals.Steps {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		geometry := "roundRect"
		switch step.Type {
		case "decision":
			geometry = "diamond"
		case "chevron":
			geometry = "chevron"
		case "arrow":
			geometry = "rightArrow"
		}

		text := buildProcessFlowTextContent(pptx.ConvertMarkdownEmphasis(step.Label), bodySize)

		cell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: geometry,
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text,
			},
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

	grid := &jsonschema.ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{
			X: 0, Y: 0, Width: 100, Height: 35,
		},
		Columns: json.RawMessage(colsJSON),
		Gap:     12,
		Rows: []jsonschema.GridRowInput{
			{
				Cells:     cells,
				Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: "dk1", Width: 1.5},
			},
		},
	}

	return grid, nil
}
