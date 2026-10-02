package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

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

// Readable targets were measured by TestPhaseRoadmapBudgetProbe against the
// written size on every shipped template at the pattern's default text sizes,
// every shape keeping the uniform 0.5 cm text margin.
func phaseRoadmapDateBudget(phases int) int {
	switch {
	case phases <= 3:
		return 30
	case phases == 4:
		return 28
	case phases == 5:
		return 21
	default:
		return 16
	}
}

func phaseRoadmapMilestoneBudget(phases int) int {
	switch {
	case phases <= 2:
		return 60
	case phases == 3:
		return 40
	case phases == 4:
		return 30
	case phases == 5:
		return 21
	default:
		return 16
	}
}

// phaseRoadmapDescriptionBudget / phaseRoadmapNameBudget are the readable
// description and name lengths, or 0 where the schema maximum holds.
func phaseRoadmapDescriptionBudget(phases int) int {
	switch {
	case phases == 5:
		return 142
	case phases >= 6:
		return 107
	}
	return 0
}

func phaseRoadmapNameBudget(phases int) int {
	if phases >= 6 {
		return 31
	}
	return 0
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
		if b := phaseRoadmapDescriptionBudget(len(v.Phases)); b > 0 && runeLen(phase.Description) > b {
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].description is %d characters; %d phases hold about %d readable description characters per phase — shorten the description or use fewer phases", ErrCodeBodyTooLong, i, runeLen(phase.Description), len(v.Phases), b))
		}
		if b := phaseRoadmapNameBudget(len(v.Phases)); b > 0 && runeLen(phase.Name) > b {
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].name is %d characters; %d phases hold about %d readable name characters per phase — shorten the name or use fewer phases", ErrCodeBodyTooLong, i, runeLen(phase.Name), len(v.Phases), b))
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
			"name":        StringSchema(40).WithDescription("Phase name (e.g. \"Plan\", \"Build\"); about 31 readable characters with 6 phases"),
			"date_label":  StringSchema(30).WithDescription("Optional date range below the timeline bar; about 30 readable characters with 3 phases, 28 with 4, 21 with 5, or 16 with 6"),
			"description": StringSchema(160).WithDescription("Short description rendered below the date label; about 142 readable characters with 5 phases, 107 with 6"),
			"active":      BooleanSchema().WithDescription("When true, this phase renders with the accent fill (others use a light tint of the accent)"),
			"milestone":   StringSchema(60).WithDescription("Optional milestone callout; when any phase sets one, a milestone row is rendered. About 40 readable characters with 3 phases, 30 with 4, 21 with 5, or 16 with 6"),
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
		if strings.TrimSpace(t) == "" {
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
	headerSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	bodySize := ResolveSize(ovr.BodySize, scaleCaptionPt)
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

	// Cell index layout (used by cell_overrides). The milestone row renders
	// directly under the timeline rule, above the date labels (when any phase
	// sets a milestone), so each milestone marker sits on the timeline; the
	// indices keep their original order:
	//   0..n-1     : phase label boxes (row 0)
	//   n          : timeline bar (row 1, single colspan cell)
	//   n+1..2n    : date labels (row 2)
	//   2n+1..3n   : milestone markers (rendered between the timeline and
	//                the dates, only when hasMilestones)
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
		// Inactive phases are the neutral 8% step (not dk1 black, and not an
		// accent wash that competes with the one active phase); the active
		// phase keeps the full accent (go-slide-creator-8xsj3). Header text
		// colour follows the effective fill.
		tone := neutralTone(NeutralTint8)
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

	// Row 2 — continuous timeline rule spanning all phases. The row keeps its
	// share of the height as spacing, but the accent "spine" drawn in it is a
	// timelineRulePt rule centred in the row, not a 12-20pt band
	// (go-slide-creator-7z5we), so the phase boxes remain the dominant anchors.
	timelineCell := &jsonschema.GridCellInput{
		ColSpan:   n,
		MaxHeight: timelineRulePt,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
		},
	}
	applyPhaseRoadmapOverride(timelineCell, cellOverrides, timelineIdx, accent)
	rows = append(rows, jsonschema.GridRowInput{Height: timelinePct, Cells: []*jsonschema.GridCellInput{timelineCell}})

	// Milestones (optional) are markers on the timeline, not boxes: a small
	// accent diamond beside a one-line bold label, placed directly under the
	// rule. A full-width two-line accent box read as a second phase box
	// (go-slide-creator-knue6). Cell indices keep their documented order
	// (dates n+1..2n, milestones 2n+1..3n) whatever the row order.
	if hasMilestones {
		// A label that wraps at the column width keeps the two-line row the
		// milestone budgets were measured with, as a plain cell (a marker
		// sub-grid would lose its inset to an already squeezed row); short
		// labels get the one-line diamond marker. One wrapping label switches
		// the whole row, so the markers stay consistent.
		areaW, _ := sizingAreaPt(ctx)
		colW := equalColumnWidthPt(areaW, n, ctx.Gap(6))
		msH := phaseRoadmapMilestoneRowPt(milestoneSize)
		wraps := false
		for _, p := range vals.Phases {
			if p.Milestone != "" && !writtenFitsAt(phaseRoadmapMilestoneText(pptx.ConvertMarkdownEmphasis(p.Milestone), milestoneSize), colW, msH) {
				wraps = true
			}
		}
		if wraps {
			msH = math.Round(shapegrid.EffectiveTextSizePt(milestoneSize)*contentLineHeight*2 + 2*defaultShapeInsetTBPt)
		}
		milestoneCells := make([]*jsonschema.GridCellInput, n)
		for i, p := range vals.Phases {
			label := pptx.ConvertMarkdownEmphasis(p.Milestone)
			switch {
			case p.Milestone == "":
				milestoneCells[i] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
					},
				}
			case wraps:
				milestoneCells[i] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
						Text:     phaseRoadmapMilestoneText(label, milestoneSize),
					},
					AccentBar: &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: 2},
				}
			default:
				milestoneCells[i] = phaseRoadmapMilestoneCell(ctx, n, label, milestoneSize, accent)
			}
			applyPhaseRoadmapOverride(milestoneCells[i], cellOverrides, milestoneIdx0+i, accent)
		}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: msH, MaxHeight: msH, Cells: milestoneCells})
	}

	// Row 3 — date range labels, left-aligned like the descriptions below
	// them so each column keeps one alignment (go-slide-creator-knue6).
	dateCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		dateCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     withVerticalAlign(buildPhaseRoadmapPlainText(p.DateLabel, dateSize, true, "dk1", "l"), "ctr"),
			},
		}
		applyPhaseRoadmapOverride(dateCells[i], cellOverrides, dateIdx0+i, accent)
	}
	dateRow := jsonschema.GridRowInput{Height: datePct, Cells: dateCells}
	if tracks != nil {
		// Parallel tracks squeeze the date row towards its measured need, and
		// a percentage of the estimated area can still land below what the
		// writer needs once gaps come off the real grid height. Pin it in
		// points at no less than the writer-fit height (go-slide-creator-n1muf).
		_, areaH := sizingAreaPt(ctx)
		pt := math.Max(math.Round(datePct*areaH)/100, phaseRoadmapDateMinPt(ctx, vals, dateSize))
		dateRow = jsonschema.GridRowInput{MinHeight: pt, MaxHeight: pt, Cells: dateCells}
	}
	rows = append(rows, dateRow)

	// Final row — per-phase description callouts (left-aligned small font).
	// The row hugs the tallest description (go-slide-creator-7km8) instead of
	// flexing over the rest of the slide; the grid centres the block.
	contentW, _ := contentAreaPt(ctx)
	descW := equalColumnWidthPt(contentW, n, ctx.Gap(6)) - 2*defaultShapeInsetLRPt
	descH, descWrittenH := 0.0, 0.0
	descCells := make([]*jsonschema.GridCellInput, n)
	for i, p := range vals.Phases {
		descH = math.Max(descH, phaseRoadmapDescHeightPt(ctx.Theme.BodyFont, descW, p.Description, bodySize))
		descCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:  "rect",
				TypeScale: peerTextTypeScale,
				Fill:      json.RawMessage(`"none"`),
				Text:      buildPhaseRoadmapDescText(pptx.ConvertMarkdownEmphasis(p.Description), bodySize),
			},
		}
		applyPhaseRoadmapOverride(descCells[i], cellOverrides, descIdx0+i, accent)
		// The theme-font model alone is not enough: a Calibri template is
		// measured with its metric clone (Carlito), narrower than the
		// writer's autofit font, so a row sized by the model alone was
		// written shrunk below the 12pt floor (11.5pt on modern, 11.0pt on
		// business-template). Hold the row at the writer's own fit too.
		descWrittenH = math.Max(descWrittenH, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, descCells[i].Shape.Text, descW+2*defaultShapeInsetLRPt))
	}
	// The row keeps its content height as a minimum so a short content area
	// squeezes the flexible rows, not the descriptions below their margin.
	descRowH := math.Max(math.Round(math.Max(descH, bodySize*contentLineHeight)+2*defaultShapeInsetTBPt+6), math.Ceil(descWrittenH))
	rows = append(rows, jsonschema.GridRowInput{Cells: descCells, MinHeight: descRowH, MaxHeight: descRowH})

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
		Gap:           ctx.Gap(6),
		RowGap:        ctx.Gap(4),
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
	barTextW := contentW*(100-phaseRoadmapTrackLabelColPct)/100 - ctx.Gap(phaseRoadmapTrackGapPt) - 2*defaultShapeInsetLRPt
	barTextH, barWrittenH := bodySize*contentLineHeight, 0.0
	for _, t := range vals.ParallelTracks {
		barTextH = math.Max(barTextH, textBlockHeightPt(ctx.Theme.BodyFont, barTextW, textParagraph{text: t, size: bodySize}))
		// As for the description row: the writer's fit, not only the
		// theme-font model, sets the bar height.
		text := buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(t), bodySize, false, "dk1", "l")
		barWrittenH = math.Max(barWrittenH, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, text, barTextW+2*defaultShapeInsetLRPt))
	}
	barPt := math.Max(math.Round(barTextH+2*defaultShapeInsetTBPt+4), math.Ceil(barWrittenH))

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
	blockPt := float64(k)*barPt + float64(k-1)*ctx.Gap(phaseRoadmapTrackGapPt)
	return &phaseRoadmapTracks{
		grid: &jsonschema.ShapeGridInput{
			Columns:       json.RawMessage(colsJSON),
			ColGap:        ctx.Gap(phaseRoadmapTrackGapPt),
			RowGap:        ctx.Gap(phaseRoadmapTrackGapPt),
			Rows:          rows,
			VerticalAlign: "bottom",
		},
		label: label,
		bars:  bars,
		rowPt: math.Round(blockPt + phaseRoadmapTrackTopPadPt),
	}
}

// phaseRoadmapMilestoneMarkerPt is the milestone diamond's size.
const phaseRoadmapMilestoneMarkerPt = 8.0

// phaseRoadmapMilestoneRowPt is the milestone row: one label line plus the
// uniform shape margin.
func phaseRoadmapMilestoneRowPt(size float64) float64 {
	return math.Round(shapegrid.EffectiveTextSizePt(size)*contentLineHeight + 2*defaultShapeInsetTBPt)
}

// phaseRoadmapMilestoneText is a milestone label: bold dk1, left-aligned
// with the dates and descriptions, centred in its row.
func phaseRoadmapMilestoneText(label string, size float64) json.RawMessage {
	return withVerticalAlign(buildPhaseRoadmapPlainText(label, size, true, "dk1", "l"), "ctr")
}

// phaseRoadmapMarkerPct is the milestone marker column's share of a
// subW-wide milestone sub-grid: the text margin less the sub-grid inset.
func phaseRoadmapMarkerPct(subW float64) float64 {
	if subW <= 0 {
		return 20
	}
	return math.Min(20, (defaultShapeInsetLRPt-SubGridInsetPt)/subW*100)
}

// phaseRoadmapMilestoneCell is one milestone marker: an accent diamond in
// the column's text margin beside a one-line bold dk1 label with no fill, so
// the label starts where the date and description text start.
func phaseRoadmapMilestoneCell(ctx ExpandContext, phases int, label string, size float64, accent string) *jsonschema.GridCellInput {
	// The renderer insets a nested grid by SubGridInsetPt, so the marker
	// column spans the rest of the text margin and the label starts where
	// the dates and descriptions start.
	areaW, _ := sizingAreaPt(ctx)
	markerPct := phaseRoadmapMarkerPct(equalColumnWidthPt(areaW, phases, ctx.Gap(6)) - 2*SubGridInsetPt)
	text := buildPhaseRoadmapPlainText(label, size, true, "dk1", "l")
	var obj map[string]any
	if json.Unmarshal(text, &obj) == nil {
		obj["vertical_align"] = "ctr"
		// The unfilled label needs no margin of its own: the marker column
		// supplies the left one, and the sub-grid inset and the column gap
		// separate it from its neighbours. Explicit margins are not clamped
		// in a squeezed row, so any would starve the text.
		for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
			obj[k] = 0
		}
		text, _ = json.Marshal(obj)
	}
	cols, _ := json.Marshal([]float64{markerPct, 100 - markerPct})
	return &jsonschema.GridCellInput{
		Grid: &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(cols),
			ColGap:  0,
			Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
				{
					MaxHeight: phaseRoadmapMilestoneMarkerPt,
					Fit:       "contain",
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "diamond",
						Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
						Line:     noLine,
					},
				},
				{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
						Text:     text,
					},
				},
			}}},
		},
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

// peerTextTypeScale opts a run of peer labels out of per-cell grow-to-fill.
// Grown one cell at a time, each peer took whatever its own box allowed — a
// waterfall's "−$12.5M" inside a short bar stayed at 12pt while its
// neighbours grew to 16pt, and roadmap descriptions landed at four different
// sizes — so peers that must read as one set keep the authored size
// (go-slide-creator-n83ml).
const peerTextTypeScale = "compact"

// phaseRoadmapBulletIndentPt is the hanging indent a "- " description line
// takes as a native bullet (pptx.BulletMarginLeft, 14pt).
const phaseRoadmapBulletIndentPt = float64(pptx.BulletMarginLeft) / 12700

// buildPhaseRoadmapDescText renders a phase description. A one-line
// description keeps the plain paragraph; a multi-line one ("lead-in\n- item")
// uses the content form, whose "- " lines become native bullets rather than
// one run-on paragraph (go-slide-creator-n83ml).
func buildPhaseRoadmapDescText(content string, size float64) json.RawMessage {
	if !strings.Contains(content, "\n") {
		return buildPhaseRoadmapPlainText(content, size, false, "dk1", "l")
	}
	data, _ := json.Marshal(map[string]any{
		"content": content, "size": size, "color": "dk1", "align": "l", "vertical_align": "t",
	})
	return data
}

// phaseRoadmapDescHeightPt measures a description line by line, a bullet line
// in the width its hanging indent leaves.
func phaseRoadmapDescHeightPt(font string, widthPt float64, desc string, size float64) float64 {
	var h float64
	for _, line := range strings.Split(desc, "\n") {
		w := widthPt
		if rest, ok := strings.CutPrefix(line, "- "); ok {
			line, w = rest, widthPt-phaseRoadmapBulletIndentPt
		}
		h += textBlockHeightPt(font, w, textParagraph{text: line, size: size})
	}
	return h
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
	textW := equalColumnWidthPt(contentW, n, ctx.Gap(6)) - 2*defaultShapeInsetLRPt
	var nameH, dateH float64
	for _, p := range vals.Phases {
		nameH = math.Max(nameH, textBlockHeightPt(ctx.Theme.BodyFont, textW, textParagraph{text: p.Name, size: headerSize, bold: true}))
		dateH = math.Max(dateH, textBlockHeightPt(ctx.Theme.BodyFont, textW, textParagraph{text: p.DateLabel, size: dateSize, bold: true}))
	}
	headerNeed := math.Max(phaseRoadmapMinHeaderPct, pctOf(nameH+2*defaultShapeInsetTBPt+2, areaH))
	dateNeed := pctOf(math.Max(dateH+2*defaultShapeInsetTBPt, phaseRoadmapDateMinPt(ctx, vals, dateSize)), areaH)

	timeline, date = 6.0, 8.0
	want := phaseRoadmapHeaderPct - pctOf(blockPt+ctx.Gap(4), areaH)
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

// phaseRoadmapDateMinPt is the smallest date-row height at which the writer
// stores no autofit shrink for any date label at its column width. A 12pt bold
// date in the row's 8%/squeezed share was written at 90% (10.8pt, below the
// 12pt floor) on examples/phase-roadmap.json (go-slide-creator-n1muf).
func phaseRoadmapDateMinPt(ctx ExpandContext, vals *PhaseRoadmapValues, dateSize float64) float64 {
	n := len(vals.Phases)
	if n == 0 {
		return 0
	}
	contentW, _ := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, ctx.Gap(6))
	need := 0.0
	for _, p := range vals.Phases {
		if p.DateLabel == "" {
			continue
		}
		need = math.Max(need, writtenFitHeightPt(buildPhaseRoadmapPlainText(p.DateLabel, dateSize, true, "dk1", "ctr"), colW, 0))
	}
	return need
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
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    4,
		}
	}
}
