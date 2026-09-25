package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
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
	// ParallelTracks are cross-cutting workstreams that run alongside every
	// phase (governance, change management, …). Each renders as a full-width
	// tinted bar below the phases; ParallelLabel sits at the left spanning
	// them. Omitted or empty: the layout is unchanged.
	ParallelTracks []string `json:"parallel_tracks,omitempty"`
	ParallelLabel  string   `json:"parallel_label,omitempty"`
}

// Parallel-track limits and geometry.
const (
	phaseRoadmapMaxTracks        = 4
	phaseRoadmapTrackMaxChars    = 90
	phaseRoadmapLabelMaxChars    = 24
	phaseRoadmapDefaultLabel     = "In parallel"
	phaseRoadmapTrackLabelColPct = 14.0 // label column share of the grid width
	phaseRoadmapTrackGapPt       = 4.0  // gap between stacked track bars
	phaseRoadmapTrackTopPadPt    = 8.0  // extra air between descriptions and tracks
	phaseRoadmapHeaderPct        = 20.0 // phase-box row height without tracks
	phaseRoadmapMinHeaderPct     = 11.0 // floor the phase-box row gives up space to
)

// parallelLabel returns the label shown beside the parallel tracks.
func (v *PhaseRoadmapValues) parallelLabel() string {
	if v.ParallelLabel != "" {
		return v.ParallelLabel
	}
	return phaseRoadmapDefaultLabel
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

// Readable targets were measured against the fit collector on all four
// bundled templates at the pattern's default text sizes.
func phaseRoadmapDateBudget(phases int) int {
	switch phases {
	case 5:
		return 27
	case 6:
		return 22
	default:
		return 30
	}
}

func phaseRoadmapMilestoneBudget(phases int) int {
	switch phases {
	case 5:
		return 52
	case 6:
		return 42
	default:
		return 60
	}
}

func (pr *phaseRoadmap) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*PhaseRoadmapValues)
	if !ok || v == nil {
		return nil
	}
	dateBudget := phaseRoadmapDateBudget(len(v.Phases))
	milestoneBudget := phaseRoadmapMilestoneBudget(len(v.Phases))
	var warnings []string
	for i, phase := range v.Phases {
		if n := runeLen(phase.DateLabel); n > dateBudget {
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].date_label is %d characters; %d phases hold about %d readable date-label characters per phase — shorten the range or use fewer phases", ErrCodeBodyTooLong, i, n, len(v.Phases), dateBudget))
		}
		if n := runeLen(phase.Milestone); n > milestoneBudget {
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].milestone is %d characters; %d phases hold about %d readable milestone characters per phase — shorten the milestone or use fewer phases", ErrCodeBodyTooLong, i, n, len(v.Phases), milestoneBudget))
		}
	}
	return warnings
}

func (pr *phaseRoadmap) Schema() *Schema {
	phaseSchema := ObjectSchema(
		map[string]*Schema{
			"name":        StringSchema(40).WithDescription("Phase name (e.g. \"Plan\", \"Build\")"),
			"date_label":  StringSchema(30).WithDescription("Optional date range below the timeline bar; about 30 readable characters with 3-4 phases, 27 with 5, or 22 with 6"),
			"description": StringSchema(160).WithDescription("Short description rendered below the date label"),
			"active":      BooleanSchema().WithDescription("When true, this phase renders with the accent fill (others use a light tint of the accent)"),
			"milestone":   StringSchema(60).WithDescription("Optional milestone callout; when any phase sets one, a milestone row is rendered. About 60 readable characters with 3-4 phases, 52 with 5, or 42 with 6"),
		},
		[]string{"name"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases": ArraySchema(phaseSchema, 3, 6).WithDescription("3-6 phases in left-to-right order"),
			"parallel_tracks": ArraySchema(StringSchema(phaseRoadmapTrackMaxChars), 0, phaseRoadmapMaxTracks).
				WithDescription("Optional 0-4 cross-cutting workstreams that run alongside every phase (e.g. governance, change management). Each renders as a full-width tinted bar below the phases, one line of up to ~90 characters; the phase-box row gives up the height they need."),
			"parallel_label": StringSchema(phaseRoadmapLabelMaxChars).WithDescription("Label shown at the left of the parallel-track bars, spanning them (default \"In parallel\"). Ignored without parallel_tracks.").WithDefault(phaseRoadmapDefaultLabel),
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
		} else if runeLen(p.Name) > 40 {
			errs = append(errs, errMaxLength(name, namePath, 40, runeLen(p.Name)))
		}
		if runeLen(p.DateLabel) > 30 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].date_label", i), 30, runeLen(p.DateLabel)))
		}
		if runeLen(p.Description) > 160 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].description", i), 160, runeLen(p.Description)))
		}
		if runeLen(p.Milestone) > 60 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].milestone", i), 60, runeLen(p.Milestone)))
		}
		if p.Active {
			activeCount++
		}
	}
	if len(vals.ParallelTracks) > phaseRoadmapMaxTracks {
		errs = append(errs, errMaxItems(name, "parallel_tracks", phaseRoadmapMaxTracks, len(vals.ParallelTracks),
			"(hint: merge related workstreams, or use roadmap-phased when workstreams differ by phase)"))
	}
	for i, t := range vals.ParallelTracks {
		path := fmt.Sprintf("parallel_tracks[%d]", i)
		if t == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(t) > phaseRoadmapTrackMaxChars {
			errs = append(errs, errMaxLength(name, path, phaseRoadmapTrackMaxChars, runeLen(t)))
		}
	}
	if runeLen(vals.ParallelLabel) > phaseRoadmapLabelMaxChars {
		errs = append(errs, errMaxLength(name, "parallel_label", phaseRoadmapLabelMaxChars, runeLen(vals.ParallelLabel)))
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
	//   then       : description callouts
	//   then       : parallel-track label, then one bar per track (only when
	//                parallel_tracks is non-empty)
	phaseIdx0 := 0
	timelineIdx := n
	dateIdx0 := n + 1
	milestoneIdx0 := 2*n + 1
	descIdx0 := 2*n + 1
	if hasMilestones {
		descIdx0 = 3*n + 1
	}

	// Parallel tracks take their height from the phase-box row, which is
	// the roomiest band (20% for a one-line phase name), so adding them does
	// not push the descriptions or the callout off the slide. The phase row
	// never drops below what its longest (measured) name needs; any remaining
	// deficit comes out of the timeline spine and then the date row.
	headerPct, timelinePct, datePct := phaseRoadmapHeaderPct, 6.0, 8.0
	var tracks *phaseRoadmapTracks
	if len(vals.ParallelTracks) > 0 {
		tracks = buildPhaseRoadmapTracks(ctx, vals, accent, bodySize, headerSize)
		headerPct, timelinePct, datePct = phaseRoadmapTrackedRowPcts(ctx, vals, tracks.rowPt, headerSize, dateSize)
	}

	var rows []jsonschema.GridRowInput

	// Row 1 — phase label boxes
	phaseCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		// Inactive phases use a light tint of the accent (not dk1 black, which
		// reads off-brand next to the accent); the active phase keeps the
		// full accent. Header text colour follows the effective fill.
		tone := inactiveTintTone(accent)
		if p.Active {
			tone = fillTone{Color: accent}
		}
		textColor := readableTextOn(ctx, tone, phaseRoadmapFallbackText(p.Active))
		phaseCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     tone.fillJSON(),
				Text:     buildPhaseRoadmapHeaderText(pptx.ConvertMarkdownEmphasis(p.Name), headerSize, textColor),
			},
		}
		applyPhaseRoadmapOverride(phaseCells[i], cellOverrides, phaseIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Height: headerPct, Cells: phaseCells})

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
	rows = append(rows, jsonschema.GridRowInput{Height: timelinePct, Cells: []*jsonschema.GridCellInput{timelineCell}})

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
	rows = append(rows, jsonschema.GridRowInput{Height: datePct, Cells: dateCells})

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
		msH := math.Round(shapegrid.EffectiveTextSizePt(milestoneSize)*contentLineHeight*2 + 2*defaultShapeInsetTBPt)
		rows = append(rows, jsonschema.GridRowInput{MinHeight: msH, MaxHeight: msH, Cells: milestoneCells})
	}

	// Final row — per-phase description callouts (left-aligned small font).
	// The row hugs the tallest description (go-slide-creator-7km8) instead of
	// flexing over the rest of the slide; the grid centres the block.
	contentW, _ := contentAreaPt(ctx)
	descW := equalColumnWidthPt(contentW, n, 6) - 2*defaultShapeInsetLRPt
	descH := 0.0
	descCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		descH = math.Max(descH, textBlockHeightPt(ctx.Theme.BodyFont, descW, textParagraph{text: p.Description, size: bodySize}))
		descCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(p.Description), bodySize, false, "dk1", "l"),
			},
		}
		applyPhaseRoadmapOverride(descCells[i], cellOverrides, descIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Cells: descCells, MaxHeight: math.Round(math.Max(descH, bodySize*contentLineHeight) + 2*defaultShapeInsetTBPt + 6)})

	// Optional parallel-track block — label + full-width tinted bars.
	if tracks != nil {
		trackIdx0 := descIdx0 + n
		applyPhaseRoadmapOverride(tracks.label, cellOverrides, trackIdx0, accent)
		for i, bar := range tracks.bars {
			applyPhaseRoadmapOverride(bar, cellOverrides, trackIdx0+1+i, accent)
		}
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: tracks.rowPt,
			MaxHeight: tracks.rowPt,
			Cells:     []*jsonschema.GridCellInput{{ColSpan: n, Grid: tracks.grid}},
		})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:           6,
		RowGap:        4,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
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
	if k := len(vals.ParallelTracks); k > 0 {
		total += 1 + k // label + one bar per track
	}
	return total
}

// phaseRoadmapTracks is the rendered parallel-track block: a nested grid of
// a label column (spanning every track) beside one tinted bar per track.
type phaseRoadmapTracks struct {
	grid  *jsonschema.ShapeGridInput
	label *jsonschema.GridCellInput
	bars  []*jsonschema.GridCellInput
	rowPt float64 // total block height, including the top padding
}

func buildPhaseRoadmapTracks(ctx ExpandContext, vals *PhaseRoadmapValues, accent string, bodySize, headerSize float64) *phaseRoadmapTracks {
	k := len(vals.ParallelTracks)
	contentW, _ := contentAreaPt(ctx)
	barTextW := contentW*(100-phaseRoadmapTrackLabelColPct)/100 - phaseRoadmapTrackGapPt - 2*defaultShapeInsetLRPt
	barTextH := bodySize * contentLineHeight
	for _, t := range vals.ParallelTracks {
		barTextH = math.Max(barTextH, textBlockHeightPt(ctx.Theme.BodyFont, barTextW, textParagraph{text: t, size: bodySize}))
	}
	barPt := math.Round(barTextH + 2*defaultShapeInsetTBPt + 4)

	labelSize := math.Max(bodySize+1, math.Min(headerSize-2, 12))
	label := &jsonschema.GridCellInput{
		RowSpan: k,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(vals.parallelLabel()), labelSize, true, inkOnLight(ctx, accent, 4.5), "l"),
		},
		AccentBar: &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 3},
	}
	// Centre the label on the stacked bars rather than pinning it to the top.
	var lt phaseRoadmapTextObj
	if err := json.Unmarshal(label.Shape.Text, &lt); err == nil {
		lt.VerticalAlign = "ctr"
		label.Shape.Text, _ = json.Marshal(lt)
	}

	tone := inactiveTintTone(accent)
	textColor := readableTextOn(ctx, tone, "dk1")
	bars := make([]*jsonschema.GridCellInput, k)
	rows := make([]jsonschema.GridRowInput, k)
	for i, t := range vals.ParallelTracks {
		text := buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(t), bodySize, false, textColor, "l")
		var obj phaseRoadmapTextObj
		if err := json.Unmarshal(text, &obj); err == nil {
			obj.VerticalAlign = "ctr"
			text, _ = json.Marshal(obj)
		}
		bars[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     tone.fillJSON(),
				Text:     text,
			},
		}
		cells := []*jsonschema.GridCellInput{bars[i]}
		if i == 0 {
			cells = []*jsonschema.GridCellInput{label, bars[i]}
		}
		rows[i] = jsonschema.GridRowInput{MinHeight: barPt, MaxHeight: barPt, Cells: cells}
	}

	colsJSON, _ := json.Marshal([]float64{phaseRoadmapTrackLabelColPct, 100 - phaseRoadmapTrackLabelColPct})
	blockPt := float64(k)*barPt + float64(k-1)*phaseRoadmapTrackGapPt
	return &phaseRoadmapTracks{
		grid: &jsonschema.ShapeGridInput{
			Columns:       json.RawMessage(colsJSON),
			ColGap:        phaseRoadmapTrackGapPt,
			RowGap:        phaseRoadmapTrackGapPt,
			Rows:          rows,
			VerticalAlign: "bottom",
		},
		label: label,
		bars:  bars,
		rowPt: math.Round(blockPt + phaseRoadmapTrackTopPadPt),
	}
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

// phaseRoadmapFallbackText is the header text colour used when the theme
// cannot be resolved: light text on the full accent, dark on the tint.
func phaseRoadmapFallbackText(active bool) string {
	if active {
		return "lt1"
	}
	return "dk1"
}

func buildPhaseRoadmapHeaderText(content string, size float64, color string) json.RawMessage {
	textObj := phaseRoadmapTextObj{
		Paragraphs: []phaseRoadmapParagraph{
			{Content: content, Size: size, Bold: true, Color: color, Align: "ctr"},
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

// phaseRoadmapTrackedRowPcts returns the phase-box, timeline and date row
// heights (percent of the content height) once a parallel-track block of
// blockPt has to fit below the roadmap. The block's height comes out of the
// phase-box row down to the larger of its floor and the measured height of
// the longest phase name; what is still missing then comes out of the
// timeline spine (to 3%) and the date row (to its measured need).
func phaseRoadmapTrackedRowPcts(ctx ExpandContext, vals *PhaseRoadmapValues, blockPt, headerSize, dateSize float64) (header, timeline, date float64) {
	n := len(vals.Phases)
	contentW, _ := contentAreaPt(ctx)
	_, areaH := sizingAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, n, 6) - 2*defaultShapeInsetLRPt
	var nameH, dateH float64
	for _, p := range vals.Phases {
		nameH = math.Max(nameH, textBlockHeightPt(ctx.Theme.BodyFont, textW, textParagraph{text: p.Name, size: headerSize, bold: true}))
		dateH = math.Max(dateH, textBlockHeightPt(ctx.Theme.BodyFont, textW, textParagraph{text: p.DateLabel, size: dateSize, bold: true}))
	}
	headerNeed := math.Max(phaseRoadmapMinHeaderPct, pctOf(nameH+2*defaultShapeInsetTBPt+2, areaH))
	dateNeed := pctOf(dateH+2*defaultShapeInsetTBPt, areaH)

	timeline, date = 6.0, 8.0
	want := phaseRoadmapHeaderPct - pctOf(blockPt+4, areaH)
	header = math.Max(headerNeed, want)
	deficit := header - want
	if deficit > 0 {
		take := math.Min(deficit, timeline-3)
		timeline -= take
		deficit -= take
	}
	if deficit > 0 && date > dateNeed {
		date -= math.Min(deficit, date-dateNeed)
	}
	return math.Round(header*10) / 10, math.Round(timeline*10) / 10, math.Round(date*10) / 10
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
