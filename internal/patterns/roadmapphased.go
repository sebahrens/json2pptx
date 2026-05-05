package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// roadmap-phased pattern — quarters across, workstreams down, pills per phase
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&roadmapPhased{})
}

type roadmapPhased struct{}

func (r *roadmapPhased) Name() string        { return "roadmap-phased" }
func (r *roadmapPhased) Description() string { return "Phased roadmap with workstreams and time periods" }
func (r *roadmapPhased) UseWhen() string     { return "Multi-phase roadmap, quarterly plan, release timeline" }
func (r *roadmapPhased) Version() int        { return 1 }
func (r *roadmapPhased) CellsHint() string   { return "workstreams × phases" }
func (r *roadmapPhased) SupportsCallout() bool        { return true }
func (r *roadmapPhased) SupportsInlineMarkdown() bool { return true }

func (r *roadmapPhased) ExemplarValues() any {
	return &RoadmapPhasedValues{
		Phases: []string{"Q1", "Q2", "Q3", "Q4"},
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Items: []string{"Auth rewrite", "API v2", "Caching", "Scale testing"}},
			{Name: "Frontend", Items: []string{"Design system", "Dashboard", "Mobile app", "PWA"}},
			{Name: "Data", Items: []string{"Pipeline v2", "ML models", "Analytics", "Reporting"}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// RoadmapWorkstream represents one horizontal workstream with items per phase.
type RoadmapWorkstream struct {
	Name  string   `json:"name"`
	Items []string `json:"items"` // One item per phase (empty string = no activity)
}

// RoadmapPhasedValues holds phases (columns) and workstreams (rows).
type RoadmapPhasedValues struct {
	Phases      []string            `json:"phases"`
	Workstreams []RoadmapWorkstream `json:"workstreams"`
}

// RoadmapPhasedOverrides is the standard text overrides.
type RoadmapPhasedOverrides = TextOverrides

// RoadmapPhasedCellOverride is the shared per-cell override.
type RoadmapPhasedCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (r *roadmapPhased) NewValues() any      { return &RoadmapPhasedValues{} }
func (r *roadmapPhased) NewOverrides() any   { return &RoadmapPhasedOverrides{} }
func (r *roadmapPhased) NewCellOverride() any { return &RoadmapPhasedCellOverride{} }

func (r *roadmapPhased) Schema() *Schema {
	workstreamSchema := ObjectSchema(
		map[string]*Schema{
			"name":  StringSchema(40).WithDescription("Workstream name"),
			"items": ArraySchema(StringSchema(80), 2, 8).WithDescription("One item per phase (empty string = no activity in that phase)"),
		},
		[]string{"name", "items"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases":      ArraySchema(StringSchema(20), 2, 8).WithDescription("Phase/period labels (column headers)"),
			"workstreams": ArraySchema(workstreamSchema, 2, 6).WithDescription("Workstreams (rows) with items per phase"),
		},
		[]string{"phases", "workstreams"},
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
	}).WithDescription("Phased roadmap with workstreams and time periods")
}

func (r *roadmapPhased) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*RoadmapPhasedValues)
	if !ok || vals == nil {
		return fmt.Errorf("roadmap-phased: values must be *RoadmapPhasedValues, got %T", values)
	}

	const name = "roadmap-phased"
	var errs []error

	if len(vals.Phases) < 2 {
		errs = append(errs, errMinItems(name, "phases", 2, len(vals.Phases), ""))
	}
	if len(vals.Phases) > 8 {
		errs = append(errs, errMaxItems(name, "phases", 8, len(vals.Phases), ""))
	}
	for i, phase := range vals.Phases {
		path := fmt.Sprintf("phases[%d]", i)
		if phase == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(phase) > 20 {
			errs = append(errs, errMaxLength(name, path, 20, len(phase)))
		}
	}

	if len(vals.Workstreams) < 2 {
		errs = append(errs, errMinItems(name, "workstreams", 2, len(vals.Workstreams), ""))
	}
	if len(vals.Workstreams) > 6 {
		errs = append(errs, errMaxItems(name, "workstreams", 6, len(vals.Workstreams), ""))
	}

	phaseCount := len(vals.Phases)
	for i, ws := range vals.Workstreams {
		if ws.Name == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("workstreams[%d].name", i)))
		} else if len(ws.Name) > 40 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("workstreams[%d].name", i), 40, len(ws.Name)))
		}

		if len(ws.Items) != phaseCount {
			errs = append(errs, newValidationError(name, fmt.Sprintf("workstreams[%d].items", i), ErrCodeCountMismatch,
				fmt.Sprintf("roadmap-phased: workstreams[%d].items must have %d items (one per phase), got %d", i, phaseCount, len(ws.Items)),
				fmt.Sprintf("provide exactly %d items in workstreams[%d].items (use empty string for inactive phases)", phaseCount, i)))
		}

		for j, item := range ws.Items {
			if item != "" && len(item) > 80 {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("workstreams[%d].items[%d]", i, j), 80, len(item)))
			}
		}
	}

	// Total cells: phase headers + workstream labels + workstream items
	totalCells := len(vals.Phases) // header row
	for _, ws := range vals.Workstreams {
		totalCells += 1 + len(ws.Items) // name + items
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (r *roadmapPhased) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*RoadmapPhasedValues)
	if !ok {
		return nil, fmt.Errorf("roadmap-phased: values must be *RoadmapPhasedValues, got %T", values)
	}
	ovr := &RoadmapPhasedOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*RoadmapPhasedOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("roadmap-phased: overrides must be *RoadmapPhasedOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 11.0)
	bodySize := ResolveSize(ovr.BodySize, 10.0)

	phaseCount := len(vals.Phases)
	numCols := 1 + phaseCount // workstream label + phases

	// Column widths: label 18%, phases split the rest
	cols := make([]float64, numCols)
	cols[0] = 18
	phaseWidth := 82.0 / float64(phaseCount)
	for i := 1; i < numCols; i++ {
		cols[i] = phaseWidth
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	var rows []jsonschema.GridRowInput

	// Header row: empty corner + phase labels
	headerCells := make([]*jsonschema.GridCellInput, numCols)
	headerCells[0] = &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
		},
	}
	for i, phase := range vals.Phases {
		text := buildRoadmapTextContent(phase, headerSize, true, "lt1", "ctr")
		headerCells[i+1] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     text,
			},
		}
		applyRoadmapOverride(headerCells[i+1], cellOverrides, cellIdx, accent)
		cellIdx++
	}
	rows = append(rows, jsonschema.GridRowInput{Height: 15, Cells: headerCells})

	// Workstream rows
	for _, ws := range vals.Workstreams {
		rowCells := make([]*jsonschema.GridCellInput, numCols)

		// Workstream label
		nameText := buildRoadmapTextContent(ws.Name, headerSize, true, "dk1", "l")
		rowCells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt2"`),
				Text:     nameText,
			},
		}
		applyRoadmapOverride(rowCells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Phase items (pills)
		for j, item := range ws.Items {
			if item == "" {
				rowCells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
					},
				}
			} else {
				itemText := buildRoadmapTextContent(pptx.ConvertMarkdownEmphasis(item), bodySize, false, "lt1", "ctr")
				rowCells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
						Text:     itemText,
					},
				}
			}
			applyRoadmapOverride(rowCells[j+1], cellOverrides, cellIdx, accent)
			cellIdx++
		}

		rows = append(rows, jsonschema.GridRowInput{Cells: rowCells})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		Gap:     6,
		RowGap:  4,
		Rows:    rows,
	}

	return grid, nil
}

func buildRoadmapTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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

func applyRoadmapOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*RoadmapPhasedCellOverride)
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
