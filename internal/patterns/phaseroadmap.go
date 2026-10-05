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

// Phase text limits, exported so the semantic roadmap kind's budgets and
// findings quote the numbers this pattern enforces (go-slide-creator-ptazs).
const (
	PhaseRoadmapNameMax        = 40
	PhaseRoadmapDateLabelMax   = 30
	PhaseRoadmapDescriptionMax = 160
	PhaseRoadmapMilestoneMax   = 60
	// PhaseRoadmapMaxTracks / PhaseRoadmapTrackMax / PhaseRoadmapLabelMax bound
	// the parallel tracks and their label.
	PhaseRoadmapMaxTracks = phaseRoadmapMaxTracks
	PhaseRoadmapTrackMax  = phaseRoadmapTrackMaxChars
	PhaseRoadmapLabelMax  = phaseRoadmapLabelMaxChars
)

// Parallel-track limits and geometry.
const (
	phaseRoadmapMaxTracks        = 4
	phaseRoadmapTrackMaxChars    = 90
	phaseRoadmapLabelMaxChars    = 24
	phaseRoadmapDefaultLabel     = "In parallel"
	phaseRoadmapTrackLabelColPct = 14.0 // label column share of the grid width
	phaseRoadmapTrackGapPt       = 4.0  // gap between stacked track bars
	phaseRoadmapTrackTopPadPt    = 8.0  // extra air between descriptions and tracks
	phaseRoadmapTrackBarPadPt    = 7.0  // top / bottom text margin of a track bar
	phaseRoadmapRowGapPt         = 4.0  // gap between the roadmap's rows
	phaseRoadmapHeaderPct        = 20.0 // phase-box row height without tracks
	phaseRoadmapTimelinePct      = 6.0  // timeline spine row
	phaseRoadmapTimelineMinPct   = 3.0  // spine row beside parallel tracks on a short area
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

func (pr *phaseRoadmap) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*PhaseRoadmapValues)
	if !ok || v == nil {
		return nil
	}
	if len(v.ParallelTracks) > 0 && len(v.Phases) > 0 {
		ovr, _ := overrides.(*PhaseRoadmapOverrides)
		if w := phaseRoadmapTrackedWarnings(newPhaseRoadmapBuild(ctx, v, ovr, nil)); len(w) > 0 {
			// The height budget is the binding one: it already tells each
			// field what the area holds, so the per-count budgets below
			// would only repeat it with a larger number.
			return w
		}
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
				WithDescription("Optional 0-4 cross-cutting workstreams that run alongside every phase (e.g. governance, change management). Each renders as a full-width tinted bar below the phases, as tall as its one line of up to ~90 characters; on a short content area the rows give up padding before type size."),
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
		} else if runeLen(p.Name) > PhaseRoadmapNameMax {
			errs = append(errs, errMaxLength(name, namePath, PhaseRoadmapNameMax, runeLen(p.Name)))
		}
		if runeLen(p.DateLabel) > PhaseRoadmapDateLabelMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].date_label", i), PhaseRoadmapDateLabelMax, runeLen(p.DateLabel)))
		}
		if runeLen(p.Description) > PhaseRoadmapDescriptionMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].description", i), PhaseRoadmapDescriptionMax, runeLen(p.Description)))
		}
		if runeLen(p.Milestone) > PhaseRoadmapMilestoneMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("phases[%d].milestone", i), PhaseRoadmapMilestoneMax, runeLen(p.Milestone)))
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

	b := newPhaseRoadmapBuild(ctx, vals, ovr, cellOverrides)
	rows := b.rows(phaseRoadmapFit{})
	if len(vals.ParallelTracks) > 0 {
		rows = b.trackedRows().rows
	}

	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, len(vals.Phases))),
		Gap:           ctx.Gap(6),
		RowGap:        ctx.Gap(phaseRoadmapRowGapPt),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

func newPhaseRoadmapBuild(ctx ExpandContext, vals *PhaseRoadmapValues, ovr *PhaseRoadmapOverrides, cellOverrides map[int]any) phaseRoadmapBuild {
	if ovr == nil {
		ovr = &PhaseRoadmapOverrides{}
	}
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
	descIdx0 := 2*n + 1
	if hasMilestones {
		descIdx0 = 3*n + 1
	}
	return phaseRoadmapBuild{
		ctx: ctx, vals: vals, cellOverrides: cellOverrides,
		accent:         ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent),
		headerSize:     ResolveSize(ovr.HeaderSize, scaleSubheadPt),
		headerAuthored: ovr.HeaderSize > 0,
		bodySize:       ResolveSize(ovr.BodySize, scaleCaptionPt),
		hasMilestones:  hasMilestones,
		phaseIdx0:      0, timelineIdx: n, dateIdx0: n + 1, milestoneIdx0: 2*n + 1, descIdx0: descIdx0,
	}
}

// phaseRoadmapBuild carries what every row of one roadmap is built from.
type phaseRoadmapBuild struct {
	ctx           ExpandContext
	vals          *PhaseRoadmapValues
	cellOverrides map[int]any
	accent        string
	headerSize    float64
	bodySize      float64
	hasMilestones bool
	// headerAuthored: overrides.header_size was given, so the fit steps keep it.
	headerAuthored bool

	phaseIdx0, timelineIdx, dateIdx0, milestoneIdx0, descIdx0 int
}

// rows builds the roadmap's rows at one fit step. The zero step is the layout
// without parallel tracks: percentage phase, timeline and date rows over a
// measured description row.
func (b phaseRoadmapBuild) rows(fit phaseRoadmapFit) []jsonschema.GridRowInput {
	ctx, vals, cellOverrides, accent := b.ctx, b.vals, b.cellOverrides, b.accent
	n := len(vals.Phases)
	headerSize, bodySize := b.headerSize, b.bodySize
	if fit.headerSize > 0 {
		headerSize = fit.headerSize
	}
	dateSize, milestoneSize := bodySize, bodySize
	hasMilestones := b.hasMilestones
	phaseIdx0, timelineIdx, dateIdx0, milestoneIdx0, descIdx0 := b.phaseIdx0, b.timelineIdx, b.dateIdx0, b.milestoneIdx0, b.descIdx0
	headerPct, timelinePct, datePct := phaseRoadmapHeaderPct, 6.0, 8.0
	var tracks *phaseRoadmapTracks
	if fit.tracked {
		tracks = buildPhaseRoadmapTracks(ctx, vals, accent, bodySize, b.headerSize, fit)
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
				Text:     withRowPad(buildPhaseRoadmapHeaderText(pptx.ConvertMarkdownEmphasis(p.Name), headerSize, textColor), fit.headerPad),
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
			if p.Milestone != "" && !writtenFitsAt(ctx.themeFonts(), phaseRoadmapMilestoneText(pptx.ConvertMarkdownEmphasis(p.Milestone), milestoneSize), colW, msH) {
				wraps = true
			}
		}
		if wraps {
			msH = math.Round(shapegrid.EffectiveTextSizePt(milestoneSize)*contentLineHeight*2 + 2*defaultShapeInsetTBPt)
		}
		// The label is centred in its row with no margin of its own (marker
		// form) or carries the row padding (wrapping form), so a tightened
		// row gives up only air.
		if trim := rowPadTrimPt(fit.rowPad); trim > 0 {
			floor := 0.0
			for _, p := range vals.Phases {
				if p.Milestone == "" {
					continue
				}
				label := pptx.ConvertMarkdownEmphasis(p.Milestone)
				if wraps {
					floor = math.Max(floor, writtenFitHeightPt(ctx.themeFonts(), withRowPad(phaseRoadmapMilestoneText(label, milestoneSize), fit.rowPad), colW, 0))
					continue
				}
				floor = math.Max(floor, phaseRoadmapMarkerRowPt(ctx, phaseRoadmapMilestoneCell(ctx, n, label, milestoneSize, accent), colW, msH))
			}
			msH = math.Max(msH-trim, math.Ceil(floor))
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
						Text:     withRowPad(phaseRoadmapMilestoneText(label, milestoneSize), fit.rowPad),
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
				Text:     withRowPad(withVerticalAlign(buildPhaseRoadmapPlainText(p.DateLabel, dateSize, true, "dk1", "l"), "ctr"), fit.rowPad),
			},
		}
		applyPhaseRoadmapOverride(dateCells[i], cellOverrides, dateIdx0+i, accent)
	}
	dateRow := jsonschema.GridRowInput{Height: datePct, Cells: dateCells}
	if tracks != nil {
		// Beside parallel tracks every row is pinned in points: the date row
		// at the writer's fit of its labels, never a percentage that can land
		// below it once gaps come off the real grid height
		// (go-slide-creator-n1muf).
		pt := phaseRoadmapDateMinPt(ctx, vals, dateSize, fit.rowPad)
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
				Text:      withRowPad(buildPhaseRoadmapDescText(pptx.ConvertMarkdownEmphasis(p.Description), bodySize), fit.rowPad),
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
	descRowH := math.Max(math.Round(math.Max(descH, bodySize*contentLineHeight)+2*defaultShapeInsetTBPt-rowPadTrimPt(fit.rowPad)+fit.descSparePt()), math.Ceil(descWrittenH))
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

	return rows
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
	barPt float64 // one bar
	rowPt float64 // total block height, including the top padding
}

// buildPhaseRoadmapTracks builds the parallel-track block. A track is a bar,
// not a box: each is as tall as its own text (one line for most tracks) plus
// the fit step's bar padding, so two tracks stay lighter than the phase boxes
// they run under instead of taking a third of the content area
// (go-slide-creator-x1124).
func buildPhaseRoadmapTracks(ctx ExpandContext, vals *PhaseRoadmapValues, accent string, bodySize, headerSize float64, fit phaseRoadmapFit) *phaseRoadmapTracks {
	k := len(vals.ParallelTracks)
	contentW, _ := contentAreaPt(ctx)
	barTextW := contentW*(100-phaseRoadmapTrackLabelColPct)/100 - ctx.Gap(phaseRoadmapTrackGapPt) - 2*defaultShapeInsetLRPt
	barTextH, barWrittenH := shapegrid.EffectiveTextSizePt(bodySize)*contentLineHeight, 0.0
	for _, t := range vals.ParallelTracks {
		barTextH = math.Max(barTextH, textBlockHeightPt(ctx.Theme.BodyFont, barTextW, textParagraph{text: t, size: bodySize}))
		// As for the description row: the writer's fit, not only the
		// theme-font model, sets the bar height.
		text := withRowPad(buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(t), bodySize, false, "dk1", "l"), fit.barPad)
		barWrittenH = math.Max(barWrittenH, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, text, barTextW+2*defaultShapeInsetLRPt))
	}
	barPt := math.Max(math.Round(barTextH+2*defaultShapeInsetTBPt-rowPadTrimPt(fit.barPad)), math.Ceil(barWrittenH))

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
	// Centre the label on the stacked bars rather than pinning it to the top;
	// beside a single slim bar it takes the bar's padding.
	label.Shape.Text = withRowPad(withVerticalAlign(label.Shape.Text, "ctr"), fit.barPad)

	tone := inactiveTintTone(accent)
	textColor := readableTextOn(ctx, tone, "dk1")
	bars := make([]*jsonschema.GridCellInput, k)
	rows := make([]jsonschema.GridRowInput, k)
	for i, t := range vals.ParallelTracks {
		text := withRowPad(withVerticalAlign(buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(t), bodySize, false, textColor, "l"), "ctr"), fit.barPad)
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
		barPt: barPt,
		rowPt: math.Round(blockPt + fit.topPad),
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

// phaseRoadmapMarkerRowPt is the shortest row (at most maxPt) in which the
// writer stores a one-line milestone marker's label with no autofit shrink.
// The label has no margin of its own, so the row is its line plus the inset
// the renderer gives a nested grid.
func phaseRoadmapMarkerRowPt(ctx ExpandContext, cell *jsonschema.GridCellInput, colW, maxPt float64) float64 {
	label := cell.Grid.Rows[0].Cells[1].Shape.Text
	var cols []float64
	if json.Unmarshal(cell.Grid.Columns, &cols) != nil || len(cols) != 2 {
		return maxPt
	}
	labelW := (colW - 2*SubGridInsetPt) * cols[1] / 100
	for h := math.Ceil(largestTextRunPt(label) * contentLineHeight); h+2*SubGridInsetPt < maxPt; h++ {
		if writtenFitsAt(ctx.themeFonts(), label, labelW, h) {
			return h + 2*SubGridInsetPt
		}
	}
	return maxPt
}

// largestTextRunPt is the largest run size of a text input, or the body size
// when it cannot be read.
func largestTextRunPt(text json.RawMessage) float64 {
	tb, err := shapegrid.ResolveTextInput(text)
	if err != nil || tb == nil {
		return scaleBodyPt
	}
	return math.Max(largestRunPt(tb), scaleBodyPt)
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

// phaseRoadmapFit is one step of the vertical tightening a roadmap with
// parallel tracks takes when its rows do not fit the content area at the
// uniform 0.5 cm text margin. The zero value is the roadmap without tracks.
type phaseRoadmapFit struct {
	tracked    bool
	barPad     float64 // top / bottom margin of a track bar
	topPad     float64 // air between the descriptions and the track block
	rowPad     float64 // top / bottom margin of the unfilled milestone, date and description rows; 0 = uniform
	headerPad  float64 // top / bottom margin of the phase boxes; 0 = uniform
	headerSize float64 // phase-name size; 0 = the resolved header size
	tight      bool    // the description row keeps no spare height over its text
}

// phaseRoadmapDescSparePt is the air the description row keeps over its
// measured text.
const phaseRoadmapDescSparePt = 6.0

func (f phaseRoadmapFit) descSparePt() float64 {
	if f.tight {
		return 0
	}
	return phaseRoadmapDescSparePt
}

// phaseRoadmapFitSteps are the steps a tracked roadmap tries, roomiest first.
// Air gives way before type: the padding of the unfilled rows (rowPadStepsPt,
// the steps a ruled list takes), then the track bars and the gap above them,
// then the phase boxes become slimmer and the description row gives up its
// spare height, and only then does the phase name step down to the 12pt body floor — never below it, and an authored header size
// is kept. Descriptions, dates and milestones keep their size throughout
// (go-slide-creator-x1124).
func phaseRoadmapFitSteps(headerSize float64, headerAuthored bool) []phaseRoadmapFit {
	f := phaseRoadmapFit{tracked: true, barPad: phaseRoadmapTrackBarPadPt, topPad: phaseRoadmapTrackTopPadPt}
	steps := []phaseRoadmapFit{f}
	f.rowPad = rowPadStepsPt[0]
	steps = append(steps, f)
	f.rowPad, f.barPad = rowPadStepsPt[1], rowPadStepsPt[2]
	steps = append(steps, f)
	f.rowPad, f.topPad = rowPadStepsPt[2], phaseRoadmapTrackGapPt
	steps = append(steps, f)
	f.headerPad = rowPadStepsPt[0]
	steps = append(steps, f)
	f.headerPad, f.tight = rowPadStepsPt[1], true
	steps = append(steps, f)
	if !headerAuthored && headerSize > scaleBodyPt {
		f.headerSize = scaleBodyPt
		steps = append(steps, f)
	}
	return steps
}

// phaseRoadmapTracked is a tracked roadmap laid out at one fit step.
type phaseRoadmapTracked struct {
	rows   []jsonschema.GridRowInput
	fit    phaseRoadmapFit
	needPt float64 // smallest height the rows hold their text in at this step
	areaPt float64 // height the grid may use
	descPt float64 // description row
	barPt  float64 // one track bar
}

// fits reports whether the rows hold their text inside the content area.
func (t phaseRoadmapTracked) fits() bool { return t.needPt <= t.areaPt }

// trackedRows lays a roadmap with parallel tracks out at the first fit step
// that holds every row inside the content area, or at the tightest step when
// none does (PostExpandWarnings then reports what to cut).
func (b phaseRoadmapBuild) trackedRows() phaseRoadmapTracked {
	var t phaseRoadmapTracked
	for _, fit := range phaseRoadmapFitSteps(b.headerSize, b.headerAuthored) {
		if t = b.trackedAt(fit); t.fits() {
			break
		}
	}
	return t
}

// trackedAt pins every row of a tracked roadmap in points. The milestone,
// date, description and track rows take their measured heights; the phase-box
// row takes what the tracks leave of its 20% share, never less than the
// writer's fit of its longest name; the timeline spine takes 3% to 6%. A
// percentage phase row was scaled down with the rest of an over-committed
// grid, so on the shortest content area the names were written at 8pt
// (go-slide-creator-x1124).
func (b phaseRoadmapBuild) trackedAt(fit phaseRoadmapFit) phaseRoadmapTracked {
	ctx := b.ctx
	rows := b.rows(fit)
	_, areaH := sizingAreaPt(ctx)
	// A point in hand so rounding never over-commits the grid.
	t := phaseRoadmapTracked{rows: rows, fit: fit, areaPt: areaH - 1}
	gap := ctx.Gap(phaseRoadmapRowGapPt)
	fixed := float64(len(rows)-1) * gap
	for _, r := range rows[2:] {
		fixed += r.MinHeight
	}
	t.descPt = rows[len(rows)-2].MinHeight
	block := rows[len(rows)-1]
	t.barPt = block.Cells[0].Grid.Rows[0].MinHeight

	header := b.headerFloorPt(fit)
	timeline := math.Ceil(math.Max(timelineRulePt+4, areaH*phaseRoadmapTimelineMinPct/100))
	t.needPt = fixed + header + timeline
	if slack := t.areaPt - t.needPt; slack > 0 {
		want := areaH*phaseRoadmapHeaderPct/100 - block.MinHeight - gap
		grow := math.Floor(clampPt(want-header, 0, slack))
		header += grow
		slack -= grow
		timeline += math.Floor(clampPt(areaH*phaseRoadmapTimelinePct/100-timeline, 0, slack))
	}
	rows[0].Height, rows[0].MinHeight, rows[0].MaxHeight = 0, header, header
	rows[1].Height, rows[1].MinHeight, rows[1].MaxHeight = 0, timeline, timeline
	return t
}

// phaseRoadmapTrackedWarnings reports a roadmap with parallel tracks that does
// not fit its content area at the tightest fit step, with a budget per field:
// each description that is too long is told how many characters the area
// holds beside the tracks; when the descriptions are not what overflows, the
// last track is told how many tracks the area has room for
// (go-slide-creator-x1124).
func phaseRoadmapTrackedWarnings(b phaseRoadmapBuild) []string {
	t := b.trackedRows()
	if t.fits() {
		return nil
	}
	vals := b.vals
	n, k := len(vals.Phases), len(vals.ParallelTracks)
	beside := fmt.Sprintf("%d parallel tracks", k)
	if k == 1 {
		beside = "1 parallel track"
	}
	drop := "drop a parallel track"
	if b.hasMilestones {
		beside += " and the milestone row"
		drop += " or the milestones"
	}

	// Height the description row may take with every other row as it is.
	lineH := shapegrid.EffectiveTextSizePt(b.bodySize) * contentLineHeight
	margin := 2*defaultShapeInsetTBPt - rowPadTrimPt(t.fit.rowPad) + t.fit.descSparePt()
	lines := int(math.Floor((t.areaPt - (t.needPt - t.descPt) - margin) / lineH))
	var warnings []string
	if lines >= 1 {
		contentW, _ := contentAreaPt(b.ctx)
		descW := equalColumnWidthPt(contentW, n, b.ctx.Gap(6)) - 2*defaultShapeInsetLRPt
		for i, p := range vals.Phases {
			budget := phaseRoadmapLineBudget(b.ctx.Theme.BodyFont, p.Description, b.bodySize, descW, lines)
			if budget <= 0 || runeLen(p.Description) <= budget {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].description is %d characters; beside %s this content area (%.0fpt high) holds about %d description characters per phase (%s) — shorten the description, or %s",
				ErrCodeBodyTooLong, i, runeLen(p.Description), beside, t.areaPt, budget, countNoun(lines, "line"), drop))
		}
	}
	if len(warnings) > 0 {
		return warnings
	}
	room := k - int(math.Ceil((t.needPt-t.areaPt)/(t.barPt+b.ctx.Gap(phaseRoadmapTrackGapPt))))
	advice := fmt.Sprintf("has room for %s", countNoun(max(room, 0), "track"))
	if room < 1 {
		advice = "has no room for a track block"
	}
	return []string{fmt.Sprintf("%s: phase-roadmap parallel_tracks[%d]: %d phases with %s need about %.0fpt but the content area is %.0fpt high and %s — %s, shorten the descriptions, or free height on the slide",
		ErrCodeBodyTooLong, k-1, n, beside, t.needPt, t.areaPt, advice, drop)}
}

// countNoun is "1 line" / "3 lines".
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// phaseRoadmapLineBudget is how many characters of desc (cut at a word) wrap
// to at most lines lines in a widthPt-wide text area, rounded down to 5; the
// whole length when it already fits.
func phaseRoadmapLineBudget(font, desc string, size, widthPt float64, lines int) int {
	if phaseRoadmapDescLines(font, widthPt, desc, size) <= lines {
		return runeLen(desc)
	}
	words := strings.Fields(strings.ReplaceAll(desc, "\n", " "))
	fit := 0
	for i := 1; i <= len(words); i++ {
		head := strings.Join(words[:i], " ")
		if measuredLines(head, font, false, shapegrid.EffectiveTextSizePt(size), widthPt) > lines {
			break
		}
		fit = runeLen(head)
	}
	return fit / 5 * 5
}

// phaseRoadmapDescLines counts a description's wrapped lines, a bullet line in
// the width its hanging indent leaves.
func phaseRoadmapDescLines(font string, widthPt float64, desc string, size float64) int {
	eff := shapegrid.EffectiveTextSizePt(size)
	return int(math.Round(phaseRoadmapDescHeightPt(font, widthPt, desc, size) / (eff * contentLineHeight)))
}

// headerFloorPt is the smallest phase-box row that holds the longest phase
// name at the step's size and padding with no autofit shrink.
func (b phaseRoadmapBuild) headerFloorPt(fit phaseRoadmapFit) float64 {
	size := b.headerSize
	if fit.headerSize > 0 {
		size = fit.headerSize
	}
	n := len(b.vals.Phases)
	contentW, _ := contentAreaPt(b.ctx)
	colW := equalColumnWidthPt(contentW, n, b.ctx.Gap(6))
	margin := 2*defaultShapeInsetTBPt - rowPadTrimPt(fit.headerPad)
	need := 0.0
	for _, p := range b.vals.Phases {
		name := pptx.ConvertMarkdownEmphasis(p.Name)
		need = math.Max(need, textBlockHeightPt(b.ctx.Theme.BodyFont, colW-2*defaultShapeInsetLRPt, textParagraph{text: p.Name, size: size, bold: true})+margin+2)
		need = math.Max(need, writtenFitHeightPt(b.ctx.themeFonts(), withRowPad(buildPhaseRoadmapHeaderText(name, size, "dk1"), fit.headerPad), colW, 0))
	}
	return math.Ceil(need)
}

// withRowPad returns text with its top and bottom margins set to padPt; 0
// (or the uniform margin) leaves the text unchanged.
func withRowPad(text json.RawMessage, padPt float64) json.RawMessage {
	top, bottom := rowPadInsets(padPt, 0)
	if top == nil {
		return text
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(text, &obj) != nil {
		return text
	}
	obj["inset_top"], obj["inset_bottom"] = marshalRaw(*top), marshalRaw(*bottom)
	out, err := json.Marshal(obj)
	if err != nil {
		return text
	}
	return out
}

// phaseRoadmapDateMinPt is the smallest date-row height at which the writer
// stores no autofit shrink for any date label at its column width. A 12pt bold
// date in the row's 8%/squeezed share was written at 90% (10.8pt, below the
// 12pt floor) on examples/phase-roadmap.json (go-slide-creator-n1muf).
func phaseRoadmapDateMinPt(ctx ExpandContext, vals *PhaseRoadmapValues, dateSize, padPt float64) float64 {
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
		need = math.Max(need, writtenFitHeightPt(ctx.themeFonts(), withRowPad(buildPhaseRoadmapPlainText(p.DateLabel, dateSize, true, "dk1", "ctr"), padPt), colW, 0))
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
