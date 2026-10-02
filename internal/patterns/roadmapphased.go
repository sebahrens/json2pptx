package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

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

func (r *roadmapPhased) Name() string { return "roadmap-phased" }
func (r *roadmapPhased) Description() string {
	return "Phased roadmap with workstreams and time periods"
}
func (r *roadmapPhased) UseWhen() string {
	return "Multi-phase roadmap with workstreams across time columns (quarterly plan, release timeline); prefer timeline-horizontal for a single-track sequence of milestones, swimlane for cross-actor process"
}
func (r *roadmapPhased) NotWhen() string {
	return "Single-track linear milestones without parallel workstreams (use timeline-horizontal), or steps are owned by actors not workstreams (use swimlane)"
}
func (r *roadmapPhased) Version() int      { return 1 }
func (r *roadmapPhased) CellsHint() string { return "workstreams × phases" }
func (r *roadmapPhased) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"kpi-3up", "timeline-horizontal", "card-grid"},
		DensityClass:  "high",
		AccentWeight:  "normal",
	}
}
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

// RoadmapPhasedOverrides is the standard text overrides plus the grid style.
type RoadmapPhasedOverrides struct {
	TextOverrides
	// Style is "tinted" (default: neutral activity cells with dk1 text, phase
	// headers marked by an accent rule) or "solid" (the legacy look: every
	// header and activity filled with the accent).
	Style string `json:"style,omitempty"`
}

// roadmapPhasedStyles are the accepted overrides.style values.
var roadmapPhasedStyles = []string{"tinted", "solid"}

// RoadmapPhasedCellOverride is the shared per-cell override.
type RoadmapPhasedCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (r *roadmapPhased) NewValues() any       { return &RoadmapPhasedValues{} }
func (r *roadmapPhased) NewOverrides() any    { return &RoadmapPhasedOverrides{} }
func (r *roadmapPhased) NewCellOverride() any { return &RoadmapPhasedCellOverride{} }

// Measured with TestRoadmapPhasedBudgetProbe against the written size (no run
// stored below its role floor) on every shipped template
// (go-slide-creator-n1muf), every shape keeping the uniform 0.5 cm text
// margin. Rows are phase counts 2..8, columns workstream counts 2..6.
var roadmapPhasedItemBudgets = [7][5]int{
	{80, 80, 80, 51, 51},
	{80, 80, 61, 31, 31},
	{80, 62, 42, 22, 22},
	{80, 47, 31, 16, 16},
	{62, 32, 22, 12, 12},
	{60, 30, 20, 10, 10},
	{32, 17, 12, 7, 7},
}

// roadmapPhasedNameBudget is the readable workstream-name length.
func roadmapPhasedNameBudget(phases, workstreams int) int {
	switch {
	case workstreams <= 3:
		return 40
	case workstreams == 4 && phases <= 5:
		return 38
	case workstreams == 4:
		return 32
	case phases <= 5:
		return 20
	default:
		return 17
	}
}

// roadmapPhasedPhaseBudget is the readable phase-label length.
func roadmapPhasedPhaseBudget(phases int) int {
	switch {
	case phases <= 4:
		return 20
	case phases == 5:
		return 16
	case phases == 6:
		return 12
	case phases == 7:
		return 10
	default:
		return 7
	}
}

func roadmapPhasedItemBudget(phases, workstreams int) int {
	if phases < 2 || phases > 8 || workstreams < 2 || workstreams > 6 {
		return 80
	}
	return roadmapPhasedItemBudgets[phases-2][workstreams-2]
}

func (r *roadmapPhased) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*RoadmapPhasedValues)
	if !ok || v == nil {
		return nil
	}
	budget := roadmapPhasedItemBudget(len(v.Phases), len(v.Workstreams))
	var warnings []string
	phaseBudget := roadmapPhasedPhaseBudget(len(v.Phases))
	for i, phase := range v.Phases {
		if n := runeLen(phase); n > phaseBudget {
			warnings = append(warnings, fmt.Sprintf("%s: roadmap-phased phases[%d] is %d characters; %d phases hold about %d readable characters per phase label — shorten the label or use fewer phases", ErrCodeBodyTooLong, i, n, len(v.Phases), phaseBudget))
		}
	}
	nameBudget := roadmapPhasedNameBudget(len(v.Phases), len(v.Workstreams))
	for i, ws := range v.Workstreams {
		if n := runeLen(ws.Name); n > nameBudget {
			warnings = append(warnings, fmt.Sprintf("%s: roadmap-phased workstreams[%d].name is %d characters; a %d-phase x %d-workstream roadmap holds about %d readable name characters — shorten the name or use fewer workstreams", ErrCodeBodyTooLong, i, n, len(v.Phases), len(v.Workstreams), nameBudget))
		}
	}
	for i, ws := range v.Workstreams {
		for j, item := range ws.Items {
			if n := runeLen(item); n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: roadmap-phased workstreams[%d].items[%d] is %d characters; a %d-phase x %d-workstream roadmap holds about %d per activity pill before text shrinks below the readable minimum — shorten the activity or use fewer phases/workstreams", ErrCodeBodyTooLong, i, j, n, len(v.Phases), len(v.Workstreams), budget))
			}
		}
	}
	return warnings
}

func (r *roadmapPhased) Schema() *Schema {
	workstreamSchema := ObjectSchema(
		map[string]*Schema{
			"name":  StringSchema(40).WithDescription("Workstream name; about 40 readable characters with 2-3 workstreams, 38 with 4 (32 at 6+ phases), 20 with 5-6 (17 at 6+ phases)"),
			"items": ArraySchema(StringSchema(80), 2, 8).WithDescription("One activity per phase (empty = none). Approximate readable chars per pill by phase count x workstream count (workstreams 2/3/4/5/6): phases 2: 80/80/80/51/51; 3: 80/80/61/31/31; 4: 80/62/42/22/22; 5: 80/47/31/16/16; 6: 62/32/22/12/12; 7: 60/30/20/10/10; 8: 32/17/12/7/7"),
		},
		[]string{"name", "items"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases":      ArraySchema(StringSchema(20), 2, 8).WithDescription("Phase/period labels (column headers); about 20 readable characters up to 4 phases, 16/12/10/7 at 5/6/7/8"),
			"workstreams": ArraySchema(workstreamSchema, 2, 6).WithDescription("Workstreams (rows) with items per phase"),
		},
		[]string{"phases", "workstreams"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      roadmapPhasedOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Phased roadmap with workstreams and time periods")
}

// roadmapPhasedOverridesSchema is the standard text overrides plus style.
func roadmapPhasedOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(roadmapPhasedStyles...).WithDescription("tinted (default): activity cells on a neutral tint with dark text, workstream labels on a darker tint, phase headers bold with an accent rule — the accent marks structure without a wall of colour; emphasise one activity with cell_overrides accent_bar. solid: every phase header and activity filled with the accent (legacy look)").WithDefault("tinted")
	return s
}

func (r *roadmapPhased) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*RoadmapPhasedValues)
	if !ok || vals == nil {
		return fmt.Errorf("roadmap-phased: values must be *RoadmapPhasedValues, got %T", values)
	}

	const name = "roadmap-phased"
	var errs []error

	if ovr, ok := overrides.(*RoadmapPhasedOverrides); ok && ovr != nil && ovr.Style != "" && !slices.Contains(roadmapPhasedStyles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, roadmapPhasedStyles))
	}

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
		} else if runeLen(phase) > 20 {
			errs = append(errs, errMaxLength(name, path, 20, runeLen(phase)))
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
		} else if runeLen(ws.Name) > 40 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("workstreams[%d].name", i), 40, runeLen(ws.Name)))
		}

		if len(ws.Items) != phaseCount {
			errs = append(errs, newValidationError(name, fmt.Sprintf("workstreams[%d].items", i), ErrCodeCountMismatch,
				fmt.Sprintf("roadmap-phased: workstreams[%d].items must have %d items (one per phase), got %d", i, phaseCount, len(ws.Items)),
				ResizeListFix(fmt.Sprintf("workstreams[%d].items", i), phaseCount)))
		}

		for j, item := range ws.Items {
			if item != "" && runeLen(item) > 80 {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("workstreams[%d].items[%d]", i, j), 80, runeLen(item)))
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
	headerSize := ResolveSize(ovr.HeaderSize, scaleDenseBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleCaptionPt)

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
	// The default "tinted" style keeps the accent for structure: a rule over
	// each phase header, dark text on neutral activity cells. Sixteen solid
	// accent rectangles read as a wall of colour, not a plan
	// (go-slide-creator-k8x1p).
	solid := ovr.Style == "solid"
	itemFill, itemInk := neutralFillJSON(NeutralTint4), "dk1"
	labelFill := neutralFillJSON(NeutralTint8)
	if solid {
		itemFill, itemInk = json.RawMessage(fmt.Sprintf(`"%s"`, accent)), "lt1"
		labelFill = json.RawMessage(`"lt2"`)
	}
	for i, phase := range vals.Phases {
		if solid {
			headerCells[i+1] = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
					Text:     buildRoadmapTextContent(phase, headerSize, true, "lt1", "ctr"),
				},
			}
		} else {
			headerCells[i+1] = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"none"`),
					Text:     buildRoadmapTextContent(phase, headerSize, true, "dk1", "ctr"),
				},
				AccentBar: &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt},
			}
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
				Fill:     labelFill,
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
				itemText := buildRoadmapTextContent(pptx.ConvertMarkdownEmphasis(item), bodySize, false, itemInk, "ctr")
				rowCells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     itemFill,
						Line:     noLine,
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
		Gap:     ctx.Gap(6),
		RowGap:  ctx.Gap(4),
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
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
