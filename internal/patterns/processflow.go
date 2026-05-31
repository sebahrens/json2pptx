package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// process-flow pattern — left-to-right rectangles and diamonds with arrows
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&processFlow{})
}

type processFlow struct{}

func (p *processFlow) Name() string        { return "process-flow" }
func (p *processFlow) Description() string { return "Left-to-right process flow with steps and decision points" }
func (p *processFlow) UseWhen() string {
	return "Sequential steps in a single-lane workflow (3-8 steps); prefer swimlane when multiple actors own different steps, timeline-horizontal when stops are date-based"
}
func (p *processFlow) NotWhen() string {
	return "Steps belong to different actors/roles (use swimlane), stops are calendar-based milestones (use timeline-horizontal), or items are unordered (use icon-row or card-grid)"
}
func (p *processFlow) Version() int { return 1 }
func (p *processFlow) CellsHint() string { return "3-8" }
func (p *processFlow) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"kpi-3up", "card-grid", "before-after"},
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

// ProcessFlowStep is a single step in the process flow.
type ProcessFlowStep struct {
	Label string `json:"label"`
	Type  string `json:"type,omitempty"` // "step" (default), "decision", "chevron", or "arrow"
}

// ProcessFlowValues holds the steps for the process flow.
type ProcessFlowValues struct {
	Steps []ProcessFlowStep `json:"steps"`
}

// ProcessFlowOverrides is the standard text overrides.
type ProcessFlowOverrides = TextOverrides

// ProcessFlowCellOverride is the shared per-cell override.
type ProcessFlowCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *processFlow) NewValues() any      { return &ProcessFlowValues{} }
func (p *processFlow) NewOverrides() any   { return &ProcessFlowOverrides{} }
func (p *processFlow) NewCellOverride() any { return &ProcessFlowCellOverride{} }

func (p *processFlow) Schema() *Schema {
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

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	bodySize := ResolveSize(ovr.BodySize, 12.0)
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

func buildProcessFlowTextContent(content string, size float64) json.RawMessage {
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
			{Content: content, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
