package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// phase-roadmap pattern — N phase boxes + timeline bar + date labels + per-phase
// description callouts + optional milestone row. Differs from roadmap-phased
// (workstreams × time grid) by focusing on a single horizontal phase sequence
// with rich per-phase metadata (active flag, date range, description, milestone).
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&phaseRoadmap{})
}

type phaseRoadmap struct{}

func (pr *phaseRoadmap) Name() string { return "phase-roadmap" }
func (pr *phaseRoadmap) Description() string {
	return "Single-track phased roadmap with phase labels, timeline bar, date ranges, per-phase descriptions, and optional milestones"
}
func (pr *phaseRoadmap) UseWhen() string {
	return "Project roadmap with 3-6 named phases each having date range and short description, optionally with an active phase highlight or per-phase milestone; prefer roadmap-phased when multiple parallel workstreams cross the phases, timeline-horizontal when stops are date milestones not phases with descriptions"
}
func (pr *phaseRoadmap) NotWhen() string {
	return "Multiple parallel workstreams cross the phases (use roadmap-phased), stops are single-line date milestones without descriptions (use timeline-horizontal), or steps are actions/decisions not time-bound phases (use process-flow)"
}
func (pr *phaseRoadmap) Version() int      { return 1 }
func (pr *phaseRoadmap) CellsHint() string { return "3-6 phases (× 3-4 rows)" }

func (pr *phaseRoadmap) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "structural",
		NarrativeRole:      []string{"frame", "evidence"},
		PairsWith:          []string{"kpi-3up", "stylish-panels", "process-flow"},
		DensityClass:       "medium",
		AccentWeight:       "normal",
		SparseThresholdPct: 15,
	}
}

func (pr *phaseRoadmap) SupportsCallout() bool        { return true }
func (pr *phaseRoadmap) SupportsInlineMarkdown() bool { return true }

func (pr *phaseRoadmap) ExemplarValues() any {
	return &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{
				Name:        "Plan",
				DateLabel:   "Mar–Apr 2025",
				Description: "Define scope, establish governance, align stakeholders.",
			},
			{
				Name:        "Build",
				DateLabel:   "May–Jul 2025",
				Description: "Implement core capabilities and run pilot.",
				Active:      true,
				Milestone:   "Pilot go-live",
			},
			{
				Name:        "Scale",
				DateLabel:   "Aug–Oct 2025",
				Description: "Roll out to remaining business units.",
			},
			{
				Name:        "Optimize",
				DateLabel:   "Nov–Dec 2025",
				Description: "Tune cost, capture lessons, hand to BAU.",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PhaseRoadmapPhase is a single named phase on the roadmap.
type PhaseRoadmapPhase struct {
	Name        string `json:"name"`
	DateLabel   string `json:"date_label,omitempty"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Milestone   string `json:"milestone,omitempty"`
}

// PhaseRoadmapValues holds the ordered phases for the roadmap.
type PhaseRoadmapValues struct {
	Phases []PhaseRoadmapPhase `json:"phases"`
}

// PhaseRoadmapOverrides is the standard text overrides.
type PhaseRoadmapOverrides = TextOverrides

// PhaseRoadmapCellOverride is the shared per-cell override.
type PhaseRoadmapCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (pr *phaseRoadmap) NewValues() any       { return &PhaseRoadmapValues{} }
func (pr *phaseRoadmap) NewOverrides() any    { return &PhaseRoadmapOverrides{} }
func (pr *phaseRoadmap) NewCellOverride() any { return &PhaseRoadmapCellOverride{} }

func (pr *phaseRoadmap) Schema() *Schema {
	phaseSchema := ObjectSchema(
		map[string]*Schema{
			"name":        StringSchema(40).WithDescription("Phase name (e.g. \"Plan\", \"Build\")"),
			"date_label":  StringSchema(30).WithDescription("Optional date range label rendered below the timeline bar (e.g. \"Mar–Apr 2025\")"),
			"description": StringSchema(160).WithDescription("Short description rendered below the date label"),
			"active":      BooleanSchema().WithDescription("When true, this phase renders with the accent fill (others use dk1/neutral)"),
			"milestone":   StringSchema(60).WithDescription("Optional milestone callout (e.g. \"Pilot go-live\"); when any phase sets one, a milestone row is rendered"),
		},
		[]string{"name"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases": ArraySchema(phaseSchema, 3, 6).WithDescription("3-6 phases in left-to-right order"),
		},
		[]string{"phases"},
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
	}).WithDescription("Single-track phased roadmap with phase labels, timeline bar, date ranges, descriptions, and optional milestones")
}

func (pr *phaseRoadmap) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*PhaseRoadmapValues)
	if !ok || vals == nil {
		return fmt.Errorf("phase-roadmap: values must be *PhaseRoadmapValues, got %T", values)
	}

	const name = "phase-roadmap"
	var errs []error

	if len(vals.Phases) < 3 {
		errs = append(errs, errMinItems(name, "phases", 3, len(vals.Phases),
			"(hint: use timeline-horizontal for date-based milestones or icon-row for fewer titled items)"))
	}
	if len(vals.Phases) > 6 {
		errs = append(errs, errMaxItems(name, "phases", 6, len(vals.Phases),
			"(hint: use roadmap-phased when workstreams cross many phases)"))
	}

	activeCount := 0
	for i, p := range vals.Phases {
		namePath := fmt.Sprintf("phases[%d].name", i)
		if p.Name == "" {
			errs = append(errs, errRequired(name, namePath))
		} else if len(p.Name) > 40 {
			errs = append(errs, errMaxLength(name, namePath, 40, len(p.Name)))
		}
		if len(p.DateLabel) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].date_label", i), 30, len(p.DateLabel)))
		}
		if len(p.Description) > 160 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].description", i), 160, len(p.Description)))
		}
		if len(p.Milestone) > 60 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].milestone", i), 60, len(p.Milestone)))
		}
		if p.Active {
			activeCount++
		}
	}
	if activeCount > 1 {
		errs = append(errs, newValidationError(name, "phases", ErrCodeCountMismatch,
			fmt.Sprintf("phase-roadmap: at most one phase may set active=true, got %d", activeCount),
			ReplaceValueFix("phases[].active", 0, 1)))
	}

	totalCells := phaseRoadmapCellCount(vals)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (pr *phaseRoadmap) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*PhaseRoadmapValues)
	if !ok {
		return nil, fmt.Errorf("phase-roadmap: values must be *PhaseRoadmapValues, got %T", values)
	}
	ovr := &PhaseRoadmapOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*PhaseRoadmapOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("phase-roadmap: overrides must be *PhaseRoadmapOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 14.0)
	bodySize := ResolveSize(ovr.BodySize, 10.0)
	dateSize := bodySize
	milestoneSize := bodySize

	n := len(vals.Phases)
	hasMilestones := false
	for _, p := range vals.Phases {
		if p.Milestone != "" {
			hasMilestones = true
			break
		}
	}

	// Cell index layout (used by cell_overrides), matching top-to-bottom render
	// order. The milestone row renders directly under the date labels (when any
	// phase sets a milestone) so milestone badges stay tucked into the roadmap
	// body rather than orphaned below the descriptions near the slide footer:
	//   0..n-1     : phase label boxes (row 0)
	//   n          : timeline bar (row 1, single colspan cell)
	//   n+1..2n    : date labels (row 2)
	//   2n+1..3n   : milestone callouts (row 3, only when hasMilestones)
	//   then       : description callouts (last row)
	phaseIdx0 := 0
	timelineIdx := n
	dateIdx0 := n + 1
	milestoneIdx0 := 2*n + 1
	descIdx0 := 2*n + 1
	if hasMilestones {
		descIdx0 = 3*n + 1
	}

	var rows []jsonschema.GridRowInput

	// Row 1 — phase label boxes
	phaseCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		fill := json.RawMessage(`"dk1"`)
		if p.Active {
			fill = json.RawMessage(fmt.Sprintf(`"%s"`, accent))
		}
		phaseCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     fill,
				Text:     buildPhaseRoadmapHeaderText(pptx.ConvertMarkdownEmphasis(p.Name), headerSize),
			},
		}
		applyPhaseRoadmapOverride(phaseCells[i], cellOverrides, phaseIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Height: 20, Cells: phaseCells})

	// Row 2 — continuous timeline bar spanning all phases. Rendered as a slim
	// accent "spine" (not a heavy dk1 band) so the phase boxes remain the
	// dominant anchors and the bar reads as a brand-coloured connector.
	timelineCell := &jsonschema.GridCellInput{
		ColSpan: n,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
		},
	}
	applyPhaseRoadmapOverride(timelineCell, cellOverrides, timelineIdx, accent)
	rows = append(rows, jsonschema.GridRowInput{Height: 6, Cells: []*jsonschema.GridCellInput{timelineCell}})

	// Row 3 — date range labels (centred under each phase)
	dateCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		dateCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildPhaseRoadmapPlainText(p.DateLabel, dateSize, true, "dk1", "ctr"),
			},
		}
		applyPhaseRoadmapOverride(dateCells[i], cellOverrides, dateIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Height: 8, Cells: dateCells})

	// Row 4 (optional) — milestone callouts as small accent badges, placed
	// directly under the date labels so they stay anchored to the timeline
	// rather than floating below the descriptions near the slide footer.
	if hasMilestones {
		milestoneCells := make([]*jsonschema.GridCellInput, n)
		for i, p := range vals.Phases {
			if p.Milestone == "" {
				milestoneCells[i] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
					},
				}
			} else {
				milestoneCells[i] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
						Text:     buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(p.Milestone), milestoneSize, true, "lt1", "ctr"),
					},
				}
			}
			applyPhaseRoadmapOverride(milestoneCells[i], cellOverrides, milestoneIdx0+i, accent)
		}
		rows = append(rows, jsonschema.GridRowInput{Height: 14, Cells: milestoneCells})
	}

	// Final row — per-phase description callouts (left-aligned small font).
	descCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		descCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(p.Description), bodySize, false, "dk1", "l"),
			},
		}
		applyPhaseRoadmapOverride(descCells[i], cellOverrides, descIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Cells: descCells})

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:     6,
		RowGap:  4,
		Rows:    rows,
	}

	return grid, nil
}

// phaseRoadmapCellCount returns the total addressable cell count for
// cell_overrides validation. Layout:
//
//	phases (n) + timeline bar (1) + date labels (n) + descriptions (n)
//	+ milestones (n, only when any phase has a milestone)
func phaseRoadmapCellCount(vals *PhaseRoadmapValues) int {
	n := len(vals.Phases)
	total := 1 + 3*n
	for _, p := range vals.Phases {
		if p.Milestone != "" {
			total += n
			break
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type phaseRoadmapParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type phaseRoadmapTextObj struct {
	Paragraphs    []phaseRoadmapParagraph `json:"paragraphs"`
	Align         string                  `json:"align"`
	VerticalAlign string                  `json:"vertical_align"`
}

func buildPhaseRoadmapHeaderText(content string, size float64) json.RawMessage {
	textObj := phaseRoadmapTextObj{
		Paragraphs: []phaseRoadmapParagraph{
			{Content: content, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildPhaseRoadmapPlainText(content string, size float64, bold bool, color, align string) json.RawMessage {
	if content == "" {
		content = " "
	}
	verticalAlign := "ctr"
	if align == "l" {
		verticalAlign = "t"
	}
	textObj := phaseRoadmapTextObj{
		Paragraphs: []phaseRoadmapParagraph{
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: verticalAlign,
	}
	data, _ := json.Marshal(textObj)
	return data
}

func applyPhaseRoadmapOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*PhaseRoadmapCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    4,
		}
	}
}
