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
// journey-maturity-model pattern — N-stage horizontal maturity ladder with
// state labels, descriptions, and an optional "where we are" current marker
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&journeyMaturity{})
}

type journeyMaturity struct{}

func (jm *journeyMaturity) Name() string { return "journey-maturity-model" }
func (jm *journeyMaturity) Description() string {
	return "Horizontal maturity ladder: 3-6 stage columns with a numbered header, a 1-3 line description, and an optional 'where we are' marker that highlights the current stage"
}
func (jm *journeyMaturity) UseWhen() string {
	return "Capability or digital maturity model with 3-6 named stages where progression matters and a single stage represents the current state; prefer value-chain when stages have no progression semantics, phase-roadmap when stages are time-anchored, and process-flow for short action steps without descriptions"
}
func (jm *journeyMaturity) NotWhen() string {
	return "Stages are time-anchored milestones (use phase-roadmap), the sequence has no progression semantics (use value-chain), steps are short actions without descriptions (use process-flow), or the ladder has fewer than 3 / more than 6 stages"
}
func (jm *journeyMaturity) Version() int      { return 1 }
func (jm *journeyMaturity) CellsHint() string { return "3-6" }
func (jm *journeyMaturity) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"value-chain", "phase-roadmap", "process-flow", "kpi-3up"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}
func (jm *journeyMaturity) SupportsInlineMarkdown() bool { return true }

func (jm *journeyMaturity) ExemplarValues() any {
	return &JourneyMaturityValues{
		Stages: []JourneyMaturityStage{
			{Label: "Initial", Description: "Ad hoc processes; no formal practices in place."},
			{Label: "Developing", Description: "Repeatable practices; informal governance."},
			{Label: "Defined", Description: "Standardised practices; documented playbooks.", Current: true},
			{Label: "Managed", Description: "Quantitatively measured outcomes; continuous review."},
			{Label: "Optimising", Description: "Continuous improvement embedded across the organisation."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// JourneyMaturityStage is a single stage on the maturity ladder: an optional
// number badge, a short label, a 1-3 line description, and an optional
// `current` flag that marks the stage as the present state.
type JourneyMaturityStage struct {
	Number      int    `json:"number,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Current     bool   `json:"current,omitempty"`
}

// JourneyMaturityValues holds the ordered stages on the ladder (3-6 items).
type JourneyMaturityValues struct {
	Stages []JourneyMaturityStage `json:"stages"`
}

// JourneyMaturityOverrides reuses the standard text overrides.
type JourneyMaturityOverrides = TextOverrides

// JourneyMaturityCellOverride is the shared per-cell override; indexed by stage.
type JourneyMaturityCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (jm *journeyMaturity) NewValues() any       { return &JourneyMaturityValues{} }
func (jm *journeyMaturity) NewOverrides() any    { return &JourneyMaturityOverrides{} }
func (jm *journeyMaturity) NewCellOverride() any { return &JourneyMaturityCellOverride{} }

func (jm *journeyMaturity) Schema() *Schema {
	stageSchema := ObjectSchema(
		map[string]*Schema{
			"number":      IntegerSchema(1, 9).WithDescription("Optional stage number (defaults to 1..N by position)"),
			"label":       StringSchema(40).WithDescription("Short stage label (1-3 words)"),
			"description": StringSchema(180).WithDescription("1-3 line description rendered below the label"),
			"current":     BooleanSchema().WithDescription("When true, marks this stage as the present state and renders a 'where we are' marker beneath it"),
		},
		[]string{"label"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"stages": ArraySchema(stageSchema, 3, 6).WithDescription("Maturity stages left-to-right (3-6)"),
		},
		[]string{"stages"},
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
	}).WithDescription("Horizontal maturity ladder of 3-6 stage columns with a numbered header, description, and optional current-stage marker")
}

func (jm *journeyMaturity) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok || vals == nil {
		return fmt.Errorf("journey-maturity-model: values must be *JourneyMaturityValues, got %T", values)
	}

	const name = "journey-maturity-model"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*JourneyMaturityOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Stages) < 3 {
		errs = append(errs, errMinItems(name, "stages", 3, len(vals.Stages), "(hint: use value-chain when there is no progression semantic)"))
	}
	if len(vals.Stages) > 6 {
		errs = append(errs, errMaxItems(name, "stages", 6, len(vals.Stages), "(hint: collapse adjacent stages or split into two slides)"))
	}

	for i, stage := range vals.Stages {
		labelPath := fmt.Sprintf("stages[%d].label", i)
		if strings.TrimSpace(stage.Label) == "" {
			errs = append(errs, errRequired(name, labelPath))
		} else if len(stage.Label) > 40 {
			errs = append(errs, errMaxLength(name, labelPath, 40, len(stage.Label)))
		}
		if len(stage.Description) > 180 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("stages[%d].description", i), 180, len(stage.Description)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Stages), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// PostExpandWarnings emits a "MULTIPLE_CURRENT_STAGES" advisory when more than
// one stage is flagged as current, since the visual marker only makes sense for
// a single present state.
func (jm *journeyMaturity) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok || vals == nil {
		return nil
	}
	count := 0
	for _, stage := range vals.Stages {
		if stage.Current {
			count++
		}
	}
	if count <= 1 {
		return nil
	}
	return []string{fmt.Sprintf("MULTIPLE_CURRENT_STAGES: %d stages flagged as current; only one 'where we are' marker is rendered meaningfully", count)}
}

func (jm *journeyMaturity) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*JourneyMaturityValues)
	if !ok {
		return nil, fmt.Errorf("journey-maturity-model: values must be *JourneyMaturityValues, got %T", values)
	}
	ovr := &JourneyMaturityOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*JourneyMaturityOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("journey-maturity-model: overrides must be *JourneyMaturityOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelSize := ResolveSize(ovr.HeaderSize, 12.0)
	descSize := ResolveSize(ovr.BodySize, 9.0)
	cellAccentMode := ovr.CellAccentMode

	bodyFill := ctx.ResolveSurface("subtle", "lt2")

	n := len(vals.Stages)

	headerCells := make([]*jsonschema.GridCellInput, n)
	bodyCells := make([]*jsonschema.GridCellInput, n)
	markerCells := make([]*jsonschema.GridCellInput, n)

	for i, stage := range vals.Stages {
		number := stage.Number
		if number <= 0 {
			number = i + 1
		}

		// Header fill: current stage uses the resolved base accent; others
		// follow the cell-accent-mode (default uniform = base accent). When
		// the user picks cell_accent_mode=progressive or alternate this gives
		// per-stage variation while still emphasising the current stage by
		// matching the base accent.
		headerFill := ResolveCellAccent(baseAccent, i, cellAccentMode)
		if stage.Current {
			headerFill = baseAccent
		}

		labelText := buildJourneyMaturityHeaderText(number, pptx.ConvertMarkdownEmphasis(stage.Label), labelSize)
		headerCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, headerFill)),
				Text:     labelText,
			},
		}
		if stage.Current {
			headerCell.AccentBar = &jsonschema.AccentBarInput{
				Position: "left",
				Color:    baseAccent,
				Width:    4,
			}
		}

		descContent := strings.TrimSpace(stage.Description)
		var descShape *jsonschema.ShapeSpecInput
		if descContent == "" {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, bodyFill)),
			}
		} else {
			descShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, bodyFill)),
				Text:     buildJourneyMaturityDescriptionText(pptx.ConvertMarkdownEmphasis(descContent), descSize),
			}
		}
		bodyCell := &jsonschema.GridCellInput{Shape: descShape}

		// Marker row: only the current stage renders a downward-pointing
		// triangle in the resolved base accent. Other cells render an empty
		// rect so the grid stays well-formed.
		var markerShape *jsonschema.ShapeSpecInput
		if stage.Current {
			markerShape = &jsonschema.ShapeSpecInput{
				Geometry: "triangle",
				Rotation: 180,
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
				Text:     buildJourneyMaturityMarkerText("We are here", descSize),
			}
		} else {
			markerShape = &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"bg1"`),
			}
		}
		markerCells[i] = &jsonschema.GridCellInput{Shape: markerShape}

		if co, coOk := cellOverrides[i]; coOk {
			if cellOvr, ok2 := co.(*JourneyMaturityCellOverride); ok2 && cellOvr.AccentBar {
				headerCell.AccentBar = &jsonschema.AccentBarInput{
					Position: "left",
					Color:    baseAccent,
					Width:    4,
				}
			}
		}

		headerCells[i] = headerCell
		bodyCells[i] = bodyCell
	}

	colsJSON, _ := json.Marshal(n)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     8,
		RowGap:  4,
		Rows: []jsonschema.GridRowInput{
			{
				Height:    30,
				Cells:     headerCells,
				Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: baseAccent, Width: 1.5},
			},
			{
				Height: 50,
				Cells:  bodyCells,
			},
			{
				Height: 20,
				Cells:  markerCells,
			},
		},
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type journeyMaturityParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type journeyMaturityTextObj struct {
	Paragraphs    []journeyMaturityParagraph `json:"paragraphs"`
	Align         string                     `json:"align"`
	VerticalAlign string                     `json:"vertical_align"`
}

func buildJourneyMaturityHeaderText(number int, label string, size float64) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: fmt.Sprintf("%d. %s", number, label), Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildJourneyMaturityDescriptionText(description string, size float64) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: description, Size: size, Color: "dk1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildJourneyMaturityMarkerText(label string, size float64) json.RawMessage {
	textObj := journeyMaturityTextObj{
		Paragraphs: []journeyMaturityParagraph{
			{Content: label, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}
