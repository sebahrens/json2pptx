package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// swimlane pattern — horizontal bands per actor with steps placed per lane
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&swimlane{})
}

type swimlane struct{}

func (s *swimlane) Name() string        { return "swimlane" }
func (s *swimlane) Description() string { return "Horizontal swimlane diagram with actors and steps" }
func (s *swimlane) UseWhen() string     { return "Cross-functional process, RACI, swimlane diagram" }
func (s *swimlane) Version() int        { return 1 }
func (s *swimlane) CellsHint() string { return "lanes × steps" }
func (s *swimlane) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"kpi-3up", "arch-stack", "process-flow"},
		DensityClass:  "high",
		AccentWeight:  "normal",
	}
}
func (s *swimlane) SupportsCallout() bool        { return true }
func (s *swimlane) SupportsInlineMarkdown() bool { return true }

func (s *swimlane) ExemplarValues() any {
	return &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "Customer", Steps: []string{"Submit request", "Receive update", "Approve"}},
			{Actor: "Support", Steps: []string{"Triage", "Investigate", "Resolve"}},
			{Actor: "Engineering", Steps: []string{"", "Fix bug", "Deploy"}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SwimlaneLane represents one horizontal band with an actor label and steps.
type SwimlaneLane struct {
	Actor string   `json:"actor"`
	Steps []string `json:"steps"` // Empty string = no shape in that column
}

// SwimlaneValues holds the lanes for the swimlane pattern.
type SwimlaneValues struct {
	Lanes []SwimlaneLane `json:"lanes"`
}

// SwimlaneOverrides is the standard text overrides.
type SwimlaneOverrides = TextOverrides

// SwimlaneCellOverride is the shared per-cell override.
type SwimlaneCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (s *swimlane) NewValues() any      { return &SwimlaneValues{} }
func (s *swimlane) NewOverrides() any   { return &SwimlaneOverrides{} }
func (s *swimlane) NewCellOverride() any { return &SwimlaneCellOverride{} }

func (s *swimlane) Schema() *Schema {
	laneSchema := ObjectSchema(
		map[string]*Schema{
			"actor": StringSchema(40).WithDescription("Lane actor/function label"),
			"steps": ArraySchema(StringSchema(80), 2, 8).WithDescription("Steps in this lane (empty string = no shape in that column)"),
		},
		[]string{"actor", "steps"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"lanes": ArraySchema(laneSchema, 2, 6).WithDescription("Horizontal lanes (2-6 actors)"),
		},
		[]string{"lanes"},
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
	}).WithDescription("Horizontal swimlane diagram with actors and steps")
}

func (s *swimlane) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*SwimlaneValues)
	if !ok || vals == nil {
		return fmt.Errorf("swimlane: values must be *SwimlaneValues, got %T", values)
	}

	const name = "swimlane"
	var errs []error

	if len(vals.Lanes) < 2 {
		errs = append(errs, errMinItems(name, "lanes", 2, len(vals.Lanes), ""))
	}
	if len(vals.Lanes) > 6 {
		errs = append(errs, errMaxItems(name, "lanes", 6, len(vals.Lanes), ""))
	}

	// All lanes must have the same number of steps.
	var stepCount int
	for i, lane := range vals.Lanes {
		if lane.Actor == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("lanes[%d].actor", i)))
		} else if len(lane.Actor) > 40 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].actor", i), 40, len(lane.Actor)))
		}

		if len(lane.Steps) < 2 {
			errs = append(errs, errMinItems(name, fmt.Sprintf("lanes[%d].steps", i), 2, len(lane.Steps), ""))
		}
		if len(lane.Steps) > 8 {
			errs = append(errs, errMaxItems(name, fmt.Sprintf("lanes[%d].steps", i), 8, len(lane.Steps), ""))
		}

		if i == 0 {
			stepCount = len(lane.Steps)
		} else if len(lane.Steps) != stepCount {
			errs = append(errs, newValidationError(name, fmt.Sprintf("lanes[%d].steps", i), ErrCodeCountMismatch,
				fmt.Sprintf("swimlane: all lanes must have the same number of steps (lane 0 has %d, lane %d has %d)", stepCount, i, len(lane.Steps)),
				"ensure all lanes have the same number of steps (use empty string for empty cells)"))
		}

		for j, step := range lane.Steps {
			if step != "" && len(step) > 80 {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].steps[%d]", i, j), 80, len(step)))
			}
		}
	}

	// Total cells: lanes * (1 actor label + steps)
	totalCells := 0
	for _, lane := range vals.Lanes {
		totalCells += 1 + len(lane.Steps)
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (s *swimlane) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*SwimlaneValues)
	if !ok {
		return nil, fmt.Errorf("swimlane: values must be *SwimlaneValues, got %T", values)
	}
	ovr := &SwimlaneOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*SwimlaneOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("swimlane: overrides must be *SwimlaneOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 12.0)
	bodySize := ResolveSize(ovr.BodySize, 11.0)

	// Determine number of columns: 1 actor label + N steps
	stepCount := 0
	if len(vals.Lanes) > 0 {
		stepCount = len(vals.Lanes[0].Steps)
	}
	numCols := 1 + stepCount

	// Column widths: actor label gets 15%, steps split the rest
	cols := make([]float64, numCols)
	cols[0] = 15
	stepWidth := 85.0 / float64(stepCount)
	for i := 1; i < numCols; i++ {
		cols[i] = stepWidth
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	var rows []jsonschema.GridRowInput

	for i, lane := range vals.Lanes {
		// Alternate lane background
		laneFill := `"lt1"`
		if i%2 == 1 {
			laneFill = `{"color":"lt2","alpha":50}`
		}

		cells := make([]*jsonschema.GridCellInput, numCols)

		// Actor label cell
		actorText := buildSwimlaneTextContent(lane.Actor, headerSize, true, "lt1", "ctr")
		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     actorText,
			},
		}
		applySwimlaneOverride(cells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Step cells
		for j, step := range lane.Steps {
			if step == "" {
				// Empty cell — transparent placeholder
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(laneFill),
					},
				}
			} else {
				stepText := buildSwimlaneTextContent(pptx.ConvertMarkdownEmphasis(step), bodySize, false, "dk1", "ctr")
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(laneFill),
						Text:     stepText,
					},
				}
			}
			applySwimlaneOverride(cells[j+1], cellOverrides, cellIdx, accent)
			cellIdx++
		}

		rows = append(rows, jsonschema.GridRowInput{Cells: cells})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     4,
		RowGap:  2,
		Rows:    rows,
	}

	return grid, nil
}

func buildSwimlaneTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func applySwimlaneOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*SwimlaneCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
