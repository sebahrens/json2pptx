package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// value-chain pattern — horizontal sequence of 4-10 steps with label + description
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&valueChain{})
}

type valueChain struct{}

func (vc *valueChain) Name() string { return "value-chain" }
func (vc *valueChain) Description() string {
	return "Horizontal value chain: equal-width step columns each with a label box on top and a description below; supports per-step highlight"
}
func (vc *valueChain) UseWhen() string {
	return "Porter-style value chain or supply/operations sequence of 4-10 steps where each step needs a short label and a 1-3 line description; prefer process-flow for 3-8 action steps without descriptions, timeline-horizontal when stops are date-based"
}
func (vc *valueChain) NotWhen() string {
	return "Steps have no description beyond the label (use process-flow), stops are calendar milestones (use timeline-horizontal), steps belong to different actors (use swimlane), or chain has fewer than 4 / more than 10 steps"
}
func (vc *valueChain) Version() int      { return 1 }
func (vc *valueChain) CellsHint() string { return "4-10" }
func (vc *valueChain) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "card-grid", "stylish-panels"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (vc *valueChain) SupportsCallout() bool        { return true }
func (vc *valueChain) SupportsInlineMarkdown() bool { return true }

func (vc *valueChain) ExemplarValues() any {
	return &ValueChainValues{
		Steps: []ValueChainStep{
			{Label: "Extraction", Description: "Mining raw materials and managing EPC contracts."},
			{Label: "Processing", Description: "Refining ore into intermediate inputs."},
			{Label: "Manufacturing", Description: "Converting inputs into finished goods.", Highlight: true},
			{Label: "Distribution", Description: "Moving product through wholesale channels."},
			{Label: "Retail", Description: "Reaching end customers via partner stores."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ValueChainStep is a single step in the chain: a short label and a 1-3 line
// description, with an optional highlight flag.
type ValueChainStep struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Highlight   bool   `json:"highlight,omitempty"`
}

// ValueChainValues holds the ordered steps for the value chain (4-10 items)
// plus an optional highlight color (defaults to accent2).
type ValueChainValues struct {
	Steps          []ValueChainStep `json:"steps"`
	HighlightColor string           `json:"highlight_color,omitempty"`
}

// ValueChainOverrides is the standard text overrides.
type ValueChainOverrides = TextOverrides

// ValueChainCellOverride is the shared per-cell override; indexed by step.
type ValueChainCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (vc *valueChain) NewValues() any       { return &ValueChainValues{} }
func (vc *valueChain) NewOverrides() any    { return &ValueChainOverrides{} }
func (vc *valueChain) NewCellOverride() any { return &ValueChainCellOverride{} }

func (vc *valueChain) Schema() *Schema {
	stepSchema := ObjectSchema(
		map[string]*Schema{
			"label":       StringSchema(40).WithDescription("Short step label (1-3 words)"),
			"description": StringSchema(180).WithDescription("1-3 line description rendered below the label"),
			"highlight":   BooleanSchema().WithDescription("When true, the label row uses the highlight color (default accent2) instead of dk1"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"steps":           ArraySchema(stepSchema, 4, 10).WithDescription("Value-chain steps left-to-right (4-10)"),
			"highlight_color": StringSchema(0).WithDescription("Scheme color used to fill highlighted label rows (default accent2)").WithDefault("accent2"),
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
	}).WithDescription("Horizontal value chain of 4-10 equal-width step columns, each with a label box and a description, with per-step highlight support")
}

func (vc *valueChain) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ValueChainValues)
	if !ok || vals == nil {
		return fmt.Errorf("value-chain: values must be *ValueChainValues, got %T", values)
	}

	const name = "value-chain"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*ValueChainOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Steps) < 4 {
		errs = append(errs, errMinItems(name, "steps", 4, len(vals.Steps), "(hint: use process-flow for 3-8 action steps without descriptions)"))
	}
	if len(vals.Steps) > 10 {
		errs = append(errs, errMaxItems(name, "steps", 10, len(vals.Steps), "(hint: split the chain across two slides)"))
	}

	for i, step := range vals.Steps {
		labelPath := fmt.Sprintf("steps[%d].label", i)
		if strings.TrimSpace(step.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if len(step.Label) > 40 {
			errs = append(errs, errMaxLength(name, labelPath, 40, len(step.Label)))
		}
		if len(step.Description) > 180 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("steps[%d].description", i), 180, len(step.Description)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Steps), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (vc *valueChain) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ValueChainValues)
	if !ok {
		return nil, fmt.Errorf("value-chain: values must be *ValueChainValues, got %T", values)
	}
	ovr := &ValueChainOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ValueChainOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("value-chain: overrides must be *ValueChainOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, 12.0)
	descSize := ResolveSize(ovr.BodySize, 9.0)
	cellAccentMode := ovr.CellAccentMode

	highlightColor := vals.HighlightColor
	if highlightColor == "" {
		highlightColor = "accent2"
	}

	n := len(vals.Steps)

	labelCells := make([]*jsonschema.GridCellInput, n)
	descCells := make([]*jsonschema.GridCellInput, n)
	for i, step := range vals.Steps {
		fill := "dk1"
		if step.Highlight {
			fill = highlightColor
		}
		// Resolved accent governs the connector and per-cell override accent bar,
		// but does NOT override the label fill — that semantic is reserved for
		// highlight vs. dk1 contrast per the layout spec.
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)

		labelText := buildValueChainLabelText(pptx.ConvertMarkdownEmphasis(step.Label), labelSize)
		labelCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, fill)),
				Text:     labelText,
			},
		}

		descContent := strings.TrimSpace(step.Description)
		var descShape *jsonschema.ShapeSpecInput
		if descContent == "" {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"bg1"`),
			}
		} else {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"bg1"`),
				Text:     buildValueChainDescriptionText(pptx.ConvertMarkdownEmphasis(descContent), descSize),
			}
		}
		descCell := &jsonschema.GridCellInput{Shape: descShape}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*ValueChainCellOverride); ok2 && cellOvr.AccentBar {
				labelCell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    accent,
					Width:    4,
				}
			}
		}

		labelCells[i] = labelCell
		descCells[i] = descCell
	}

	colsJSON, _ := json.Marshal(n)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     8,
		RowGap:  4,
		Rows: []jsonschema.GridRowInput{
			{
				Height:    25,
				Cells:     labelCells,
				Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: baseAccent, Width: 1.5},
			},
			{
				Cells: descCells,
			},
		},
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type valueChainParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type valueChainTextObj struct {
	Paragraphs    []valueChainParagraph `json:"paragraphs"`
	Align         string                `json:"align"`
	VerticalAlign string                `json:"vertical_align"`
}

func buildValueChainLabelText(label string, size float64) json.RawMessage {
	textObj := valueChainTextObj{
		Paragraphs: []valueChainParagraph{
			{Content: label, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildValueChainDescriptionText(description string, size float64) json.RawMessage {
	textObj := valueChainTextObj{
		Paragraphs: []valueChainParagraph{
			{Content: description, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}
