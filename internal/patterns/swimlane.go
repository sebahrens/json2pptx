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
func (s *swimlane) UseWhen() string {
	return "Cross-functional process where steps are owned by different actors/roles; prefer process-flow when all steps belong to one actor, roadmap-phased when lanes are workstreams over time"
}
func (s *swimlane) NotWhen() string {
	return "All steps belong to a single actor (use process-flow), lanes represent time-phased workstreams (use roadmap-phased), or responsibilities are a simple list (use card-grid)"
}
func (s *swimlane) Version() int      { return 1 }
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
			{Actor: "Customer", Steps: []string{"Report incident through the portal", "", "", "", "", "Confirm service restored and close"}},
			{Actor: "Service desk", Steps: []string{"", "Log, classify and assign priority", "", "", "Verify fix with the customer", ""}},
			{Actor: "Engineering", Steps: []string{"", "", "Diagnose the root cause", "Deploy and monitor the fix", "", ""}},
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

func (s *swimlane) NewValues() any       { return &SwimlaneValues{} }
func (s *swimlane) NewOverrides() any    { return &SwimlaneOverrides{} }
func (s *swimlane) NewCellOverride() any { return &SwimlaneCellOverride{} }

// Measured with TestSwimlaneBudgetProbe against the written size (no run
// stored below its role floor) on every shipped template
// (go-slide-creator-n1muf), every shape keeping the uniform 0.5 cm text
// margin. Rows are step columns 2..8; columns are lane counts 2..6.
var swimlaneStepBudgets = [7][5]int{
	{80, 80, 80, 80, 52},
	{80, 80, 80, 62, 32},
	{80, 80, 75, 50, 25},
	{80, 77, 47, 32, 17},
	{80, 75, 45, 30, 15},
	{80, 51, 31, 21, 11},
	{78, 50, 30, 20, 10},
}

// swimlaneActorBudget is the readable actor label length with five lanes
// (swimlaneActorBudgetSix with six).
const (
	swimlaneActorBudget    = 30
	swimlaneActorBudgetSix = 15
)

func swimlaneStepBudget(steps, lanes int) int {
	if steps < 2 || steps > 8 || lanes < 2 || lanes > 6 {
		return 80
	}
	return swimlaneStepBudgets[steps-2][lanes-2]
}

func (s *swimlane) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*SwimlaneValues)
	if !ok || v == nil {
		return nil
	}
	steps := 0
	if len(v.Lanes) > 0 {
		steps = len(v.Lanes[0].Steps)
	}
	budget := swimlaneStepBudget(steps, len(v.Lanes))
	var warnings []string
	for i, lane := range v.Lanes {
		actorBudget := 0
		switch {
		case len(v.Lanes) >= 6:
			actorBudget = swimlaneActorBudgetSix
		case len(v.Lanes) == 5:
			actorBudget = swimlaneActorBudget
		}
		if n := runeLen(lane.Actor); actorBudget > 0 && n > actorBudget {
			warnings = append(warnings, fmt.Sprintf("%s: swimlane lanes[%d].actor is %d characters; %d lanes hold about %d actor characters — shorten the label or use fewer lanes", ErrCodeBodyTooLong, i, n, len(v.Lanes), actorBudget))
		}
		for j, step := range lane.Steps {
			if n := runeLen(step); n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: swimlane lanes[%d].steps[%d] is %d characters; a %d-step x %d-lane grid holds about %d per step before text shrinks below the readable minimum — shorten the step or use fewer columns/lanes", ErrCodeBodyTooLong, i, j, n, steps, len(v.Lanes), budget))
			}
		}
	}
	return warnings
}

func (s *swimlane) Schema() *Schema {
	laneSchema := ObjectSchema(
		map[string]*Schema{
			"actor": StringSchema(40).WithDescription("Lane actor/function label; about 30 characters with five lanes, 15 with six"),
			"steps": ArraySchema(StringSchema(80), 2, 8).WithDescription("Steps in this lane (empty = no shape). Approximate readable chars per step by step columns x lanes (lanes 2/3/4/5/6): steps 2: 80/80/80/80/52; 3: 80/80/80/62/32; 4: 80/80/75/50/25; 5: 80/77/47/32/17; 6: 80/75/45/30/15; 7: 80/51/31/21/11; 8: 78/50/30/20/10"),
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
		} else if runeLen(lane.Actor) > 40 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].actor", i), 40, runeLen(lane.Actor)))
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
				ResizeListFix(fmt.Sprintf("lanes[%d].steps", i), stepCount)))
		}

		for j, step := range lane.Steps {
			if step != "" && runeLen(step) > 80 {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].steps[%d]", i, j), 80, runeLen(step)))
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
	headerSize := ResolveSize(ovr.HeaderSize, scaleBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)

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
		// Lanes alternate two neutral steps with no outline
		// (go-slide-creator-pgdkp); the 4pt grid gutter separates the cells.
		laneFill := string(neutralFillJSON(NeutralTint4))
		if i%2 == 1 {
			laneFill = string(neutralFillJSON(NeutralTint8))
		}
		laneLine := noLine

		cells := make([]*jsonschema.GridCellInput, numCols)

		// Actor label cell
		// Actor labels are structure, not emphasis: a neutral 16% block with
		// dk1 text instead of a column of solid accent (go-slide-creator-8xsj3).
		actorText := buildSwimlaneTextContent(lane.Actor, headerSize, true, "dk1", "ctr")
		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     neutralFillJSON(NeutralTint16),
				Text:     actorText,
			},
		}
		applySwimlaneOverride(cells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Step cells
		for j, step := range lane.Steps {
			if step == "" {
				// Empty position: an unpainted spacer, so the lane reads as
				// a flow with gaps rather than a table of blank tiles
				// (go-slide-creator-0b3f6). Connectors skip it.
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
						Line:     laneLine,
					},
				}
			} else {
				stepText := buildSwimlaneTextContent(pptx.ConvertMarkdownEmphasis(step), bodySize, false, "dk1", "ctr")
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(laneFill),
						Line:     laneLine,
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
		ColGap:  swimlaneColGap(stepCount),
		RowGap:  swimlaneRowGap(len(vals.Lanes)),
		Rows:    rows,
		Links:   swimlaneLinks(vals.Lanes, accent),
	}

	return grid, nil
}

// Gutters wide enough to carry a visible arrow between steps: the column
// gap holds a lane change's elbow turn plus an arrowhead, the row gap a
// same-column hand-off arrow. Dense grids narrow them so the step text keeps
// the room its budget was measured with (the 8-step x 6-lane schema maximum
// is pinned by TestSchemaMaximaStayReadable).
func swimlaneColGap(steps int) float64 {
	switch {
	case steps >= 7:
		return 6
	case steps == 6:
		return 10
	}
	return 14
}

func swimlaneRowGap(lanes int) float64 {
	switch {
	case lanes >= 6:
		return 3
	case lanes == 5:
		return 6
	case lanes == 4:
		return 8
	}
	return 10
}

// swimlaneLinks joins consecutive steps in reading order — column by
// column, top lane first within a column — with accent arrows, the same
// connector style value-chain and journey-maturity use. A lane change in the
// next column turns in the column gutter; a hand-off within one column runs
// straight down. Empty positions are skipped (go-slide-creator-0b3f6).
func swimlaneLinks(lanes []SwimlaneLane, accent string) []jsonschema.GridLinkInput {
	steps := 0
	if len(lanes) > 0 {
		steps = len(lanes[0].Steps)
	}
	var order [][2]int // {grid row, grid column}
	for j := 0; j < steps; j++ {
		for i, lane := range lanes {
			if j < len(lane.Steps) && lane.Steps[j] != "" {
				order = append(order, [2]int{i, j + 1})
			}
		}
	}
	var links []jsonschema.GridLinkInput
	for k := 0; k+1 < len(order); k++ {
		links = append(links, jsonschema.GridLinkInput{
			From:      order[k],
			To:        order[k+1],
			Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: accent, Width: 1.5},
		})
	}
	return links
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
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
