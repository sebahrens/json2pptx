package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// roadmap-phased pattern — periods across as a time axis, workstreams down,
// bars spanning the periods they run over (roadmapphased_bars.go). This file
// holds the contract and the legacy "grid" layout (one tile per period cell).
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&roadmapPhased{})
}

type roadmapPhased struct{}

func (r *roadmapPhased) Name() string { return "roadmap-phased" }
func (r *roadmapPhased) Description() string {
	return "Phased roadmap: period headers as a time axis, workstream rows carrying bars that run from a start period to an end period (overlaps stack in lanes), milestone markers and an optional current-period mark"
}
func (r *roadmapPhased) UseWhen() string {
	return "Multi-phase roadmap with workstreams across time columns (quarterly plan, release timeline); prefer timeline-horizontal for a single-track sequence of milestones, swimlane for cross-actor process"
}
func (r *roadmapPhased) NotWhen() string {
	return "Single-track linear milestones without parallel workstreams (use timeline-horizontal), or steps are owned by actors not workstreams (use swimlane)"
}
func (r *roadmapPhased) Version() int      { return 1 }
func (r *roadmapPhased) CellsHint() string { return "2-6 workstreams × 2-8 periods (1-12 bars each)" }
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
		Phases:       []string{"Q1", "Q2", "Q3", "Q4"},
		CurrentPhase: "Q2",
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Bars: []RoadmapBar{
				{Label: "Auth rewrite", Start: "Q1", End: "Q2"},
				{Label: "API v2", Start: "Q2", End: "Q4"},
			}},
			{Name: "Frontend", Bars: []RoadmapBar{
				{Label: "Design system", Start: "Q1", End: "Q2"},
				{Label: "Mobile app", Start: "Q3", End: "Q4"},
			}},
			{Name: "Data", Bars: []RoadmapBar{
				{Label: "Pipeline v2", Start: "Q1"},
				{Label: "ML models", Start: "Q2", End: "Q3"},
				{Label: "Reporting live", Start: "Q4", Milestone: true},
			}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// RoadmapBar is one activity of a workstream on the time axis: a bar from its
// start period to its end period, or a milestone marker in one period.
type RoadmapBar struct {
	Label string `json:"label"`
	// Start and End name periods of values.phases. End defaults to Start (a
	// one-period bar); Span gives the length in periods instead of End.
	Start string `json:"start"`
	End   string `json:"end,omitempty"`
	Span  int    `json:"span,omitempty"`
	// Milestone draws a marker and the label in the start period, not a bar.
	Milestone bool `json:"milestone,omitempty"`
}

// RoadmapWorkstream represents one horizontal workstream. It carries bars
// (each with its own start and end period), or one item per phase.
type RoadmapWorkstream struct {
	Name  string       `json:"name"`
	Items []string     `json:"items,omitempty"` // One item per phase (empty string = no activity)
	Bars  []RoadmapBar `json:"bars,omitempty"`
}

// RoadmapPhasedValues holds phases (columns) and workstreams (rows).
type RoadmapPhasedValues struct {
	Phases []string `json:"phases"`
	// CurrentPhase names the period the roadmap is in; its header is filled.
	CurrentPhase string              `json:"current_phase,omitempty"`
	Workstreams  []RoadmapWorkstream `json:"workstreams"`
}

// RoadmapPhasedOverrides is the standard text overrides plus the grid style.
type RoadmapPhasedOverrides struct {
	TextOverrides
	// Layout is "bars" (default: activities as bars on a time axis, stacked
	// in lanes where they overlap) or "grid" (the legacy table: one equal tile
	// per period cell; one-item-per-period input only).
	Layout string `json:"layout,omitempty"`
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

func (r *roadmapPhased) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*RoadmapPhasedValues)
	if !ok || v == nil {
		return nil
	}
	var barWarnings []string
	if roadmapHasBars(v) {
		ovr, _ := overrides.(*RoadmapPhasedOverrides)
		if ovr == nil {
			ovr = &RoadmapPhasedOverrides{}
		}
		barWarnings = roadmapBarWarnings(ctx, v, ovr)
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
		if len(ws.Bars) > 0 {
			continue
		}
		for j, item := range ws.Items {
			if n := runeLen(item); n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: roadmap-phased workstreams[%d].items[%d] is %d characters; a %d-phase x %d-workstream roadmap holds about %d per activity pill before text shrinks below the readable minimum — shorten the activity or use fewer phases/workstreams", ErrCodeBodyTooLong, i, j, n, len(v.Phases), len(v.Workstreams), budget))
			}
		}
	}
	return append(warnings, barWarnings...)
}

func (r *roadmapPhased) Schema() *Schema {
	workstreamSchema := ObjectSchema(
		map[string]*Schema{
			"name":  StringSchema(40).WithDescription("Workstream name; about 40 readable characters with 2-3 workstreams, 38 with 4 (32 at 6+ phases), 20 with 5-6 (17 at 6+ phases)"),
			"bars":  ArraySchema(roadmapBarSchema(), 1, roadmapMaxBars).WithDescription("The workstream's activities on the time axis (1-12): each a bar from its start period to its end period, or a milestone marker. Bars that share a period stack in lanes. Use bars or items, not both"),
			"items": ArraySchema(StringSchema(80), 2, 8).WithDescription("One activity per phase (empty = none), each drawn as a one-period bar; use bars for an activity that runs over several periods. Approximate readable chars per pill by phase count x workstream count (workstreams 2/3/4/5/6): phases 2: 80/80/80/51/51; 3: 80/80/61/31/31; 4: 80/62/42/22/22; 5: 80/47/31/16/16; 6: 62/32/22/12/12; 7: 60/30/20/10/10; 8: 32/17/12/7/7"),
		},
		[]string{"name"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases":        ArraySchema(StringSchema(20), 2, 8).WithDescription("Phase/period labels, in order: the time axis; about 20 readable characters up to 4 phases, 16/12/10/7 at 5/6/7/8"),
			"current_phase": StringSchema(20).WithDescription("The period the roadmap is in now (one of phases); its header is filled"),
			"workstreams":   ArraySchema(workstreamSchema, 2, 6).WithDescription("Workstreams (rows), each with bars on the time axis or one item per phase"),
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

// roadmapBarSchema is one entry of workstreams[].bars.
func roadmapBarSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"label":     StringSchema(80).WithDescription("Activity name, written inside the bar; a longer bar holds a longer label"),
			"start":     StringSchema(20).WithDescription("The period the bar starts in: one of phases"),
			"end":       StringSchema(20).WithDescription("The period the bar ends in (inclusive): one of phases, not before start. Omit for a one-period bar"),
			"span":      IntegerSchema(1, 8).WithDescription("Length in periods, counted from start; an alternative to end"),
			"milestone": BooleanSchema().WithDescription("true: a marker with the label in the start period, not a bar (no end or span)"),
		},
		[]string{"label", "start"},
	).WithAdditionalProperties(false)
}

// roadmapPhasedOverridesSchema is the standard text overrides plus style and
// layout.
func roadmapPhasedOverridesSchema() *Schema {
	s := textOverridesSchema()
	s.raw.Properties["style"] = EnumSchema(roadmapPhasedStyles...).WithDescription("tinted (default): bars in a light tint of the accent with dark text, workstream labels on a neutral tint, period headers bold over an accent axis segment — the accent marks structure without a wall of colour; emphasise one activity with cell_overrides accent_bar. solid: every period header and bar filled with the accent").WithDefault("tinted")
	s.raw.Properties["layout"] = EnumSchema(roadmapPhasedLayouts...).WithDescription("bars (default): period headers are a time axis and each activity is a bar over the periods it runs, stacked in lanes where bars overlap; rows are as tall as their text. grid: the legacy table of one equal tile per period cell, filling the slide (workstreams[].items only)").WithDefault("bars")
	return s
}

// validateRoadmapOverrides checks the style and layout enums and reports
// whether the legacy grid layout is asked for.
func validateRoadmapOverrides(name string, overrides any) (gridLayout bool, errs []error) {
	ovr, ok := overrides.(*RoadmapPhasedOverrides)
	if !ok || ovr == nil {
		return false, nil
	}
	if ovr.Style != "" && !slices.Contains(roadmapPhasedStyles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, roadmapPhasedStyles))
	}
	if ovr.Layout != "" && !slices.Contains(roadmapPhasedLayouts, ovr.Layout) {
		errs = append(errs, errInvalidEnum(name, "overrides.layout", ovr.Layout, roadmapPhasedLayouts))
	}
	return ovr.Layout == "grid", errs
}

func (r *roadmapPhased) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*RoadmapPhasedValues)
	if !ok || vals == nil {
		return fmt.Errorf("roadmap-phased: values must be *RoadmapPhasedValues, got %T", values)
	}

	const name = "roadmap-phased"
	var errs []error

	gridLayout, ovrErrs := validateRoadmapOverrides(name, overrides)
	errs = append(errs, ovrErrs...)
	if vals.CurrentPhase != "" && roadmapPhaseIndex(vals.Phases, vals.CurrentPhase) < 0 {
		errs = append(errs, errInvalidEnum(name, "current_phase", vals.CurrentPhase, vals.Phases))
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

		if len(ws.Bars) > 0 {
			errs = append(errs, validateRoadmapBars(name, i, ws, vals.Phases, gridLayout)...)
			continue
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

	// Total cells: phase headers + workstream labels + workstream items / bars
	totalCells := roadmapBarsTotalCells(vals)
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

	if ovr == nil {
		ovr = &RoadmapPhasedOverrides{}
	}
	if ovr.Layout != "grid" || roadmapHasBars(vals) {
		return r.expandBars(ctx, vals, ovr, cellOverrides)
	}
	return r.expandGrid(ctx, vals, ovr, cellOverrides)
}

// validateRoadmapBars checks one workstream's bars against the phases.
func validateRoadmapBars(name string, i int, ws RoadmapWorkstream, phases []string, gridLayout bool) []error {
	var errs []error
	path := fmt.Sprintf("workstreams[%d].bars", i)
	if len(ws.Items) > 0 {
		errs = append(errs, newValidationError(name, fmt.Sprintf("workstreams[%d].items", i), ErrCodeInvalidShape,
			fmt.Sprintf("roadmap-phased: workstreams[%d] sets both bars and items; give its activities as bars (each with a start period) or as one item per phase — keep one", i), nil))
	}
	if gridLayout {
		errs = append(errs, newValidationError(name, path, ErrCodeInvalidShape,
			fmt.Sprintf("roadmap-phased: overrides.layout \"grid\" draws one tile per period cell and cannot draw workstreams[%d].bars; drop the layout override or give one item per phase", i), nil))
	}
	if len(ws.Bars) > roadmapMaxBars {
		errs = append(errs, errMaxItems(name, path, roadmapMaxBars, len(ws.Bars), "(hint: merge short activities or split the workstream)"))
	}
	for j, bar := range ws.Bars {
		at := fmt.Sprintf("%s[%d]", path, j)
		if strings.TrimSpace(bar.Label) == "" {
			errs = append(errs, errRequired(name, at+".label"))
		} else if runeLen(bar.Label) > 80 {
			errs = append(errs, errMaxLength(name, at+".label", 80, runeLen(bar.Label)))
		}
		from := roadmapPhaseIndex(phases, bar.Start)
		switch {
		case strings.TrimSpace(bar.Start) == "":
			errs = append(errs, errRequired(name, at+".start"))
			continue
		case from < 0:
			errs = append(errs, errInvalidEnum(name, at+".start", bar.Start, phases))
			continue
		}
		if bar.Span < 0 {
			errs = append(errs, newValidationError(name, at+".span", ErrCodeOutOfRange,
				fmt.Sprintf("roadmap-phased: %s.span is %d; a span is at least 1 period", at, bar.Span), nil))
		}
		if (bar.End != "" && bar.Span > 0) || (bar.Milestone && (bar.End != "" || bar.Span > 1)) {
			errs = append(errs, newValidationError(name, at, ErrCodeInvalidShape,
				fmt.Sprintf("roadmap-phased: %s gives its length more than once; a bar takes end or span (not both) and a milestone neither", at), nil))
			continue
		}
		if bar.End != "" {
			switch to := roadmapPhaseIndex(phases, bar.End); {
			case to < 0:
				errs = append(errs, errInvalidEnum(name, at+".end", bar.End, phases))
			case to < from:
				errs = append(errs, newValidationError(name, at+".end", ErrCodeOutOfRange,
					fmt.Sprintf("roadmap-phased: %s ends in %q, before it starts in %q; phases run %s", at, bar.End, bar.Start, strings.Join(phases, ", ")), nil))
			}
		}
		if bar.Span > 0 && from+bar.Span > len(phases) {
			errs = append(errs, newValidationError(name, at+".span", ErrCodeOutOfRange,
				fmt.Sprintf("roadmap-phased: %s spans %d periods from %q, past the last of %d phases; use a span of at most %d", at, bar.Span, bar.Start, len(phases), len(phases)-from), nil))
		}
	}
	return errs
}

// expandGrid is the legacy layout (overrides.layout "grid"): a table with one
// equal tile per workstream and period, stretched over the content area.
func (r *roadmapPhased) expandGrid(ctx ExpandContext, vals *RoadmapPhasedValues, ovr *RoadmapPhasedOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
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
