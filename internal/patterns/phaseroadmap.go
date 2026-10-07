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
// phase-roadmap pattern — N phases as one band of interlocking pentagon /
// chevron shapes (the current phase solid accent, the others a light accent
// tint), each heading a panel in the lightest neutral surface that holds its
// date range (bold) and description; optional milestone markers sit on the
// band and optional parallel tracks run under the panels as pointed bars.
// Differs from roadmap-phased (workstreams × time grid) by focusing on a
// single horizontal phase sequence with rich per-phase metadata
// (go-slide-creator-dlfm6).
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&phaseRoadmap{})
}

type phaseRoadmap struct{}

func (pr *phaseRoadmap) Name() string { return "phase-roadmap" }
func (pr *phaseRoadmap) Description() string {
	return "Single-track phased roadmap: a chevron band of phases over per-phase panels (date range, description), optional milestones"
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
	// pointed bar in the band's tint below the panels; ParallelLabel sits at
	// the left spanning them. Omitted or empty: the layout is unchanged.
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
	phaseRoadmapTrackTopPadPt    = 8.0  // extra air between the panels and the tracks
	phaseRoadmapTrackBarPadPt    = 7.0  // top / bottom text margin of a track bar
	phaseRoadmapRowGapPt         = 4.0  // gap between the roadmap's rows
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
	if len(v.Phases) > 0 {
		ovr, _ := overrides.(*PhaseRoadmapOverrides)
		if w := phaseRoadmapFitWarnings(newPhaseRoadmapBuild(ctx, v, ovr, nil)); len(w) > 0 {
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
			"date_label":  StringSchema(30).WithDescription("Optional date range, the bold lead of the phase's panel; about 30 readable characters with 3 phases, 28 with 4, 21 with 5, or 16 with 6"),
			"description": StringSchema(160).WithDescription("Short description set under the date range in the phase's panel; about 142 readable characters with 5 phases, 107 with 6"),
			"active":      BooleanSchema().WithDescription("When true, this phase is the band's one solid accent shape (the others are a light tint of the accent)"),
			"milestone":   StringSchema(60).WithDescription("Optional milestone: an accent diamond and a bold label set on the band over this phase. About 40 readable characters with 3 phases, 30 with 4, 21 with 5, or 16 with 6"),
		},
		[]string{"name"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"phases": ArraySchema(phaseSchema, 3, 6).WithDescription("3-6 phases in left-to-right order"),
			"parallel_tracks": ArraySchema(StringSchema(phaseRoadmapTrackMaxChars), 0, phaseRoadmapMaxTracks).
				WithDescription("Optional 0-4 cross-cutting workstreams that run alongside every phase (e.g. governance, change management). Each renders as a full-width pointed bar in the band's tint below the panels, as tall as its one line of up to ~90 characters; on a short content area the rows give up padding before type size."),
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
	}).WithDescription("Single-track phased roadmap: a chevron band of phases over per-phase panels, with optional milestones and parallel tracks")
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
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, len(vals.Phases))),
		// The column gap is the slanted hairline between two interlocking
		// phase shapes, and so the gutter between the panels under them.
		Gap:           phaseRoadmapColGapPt,
		RowGap:        ctx.Gap(phaseRoadmapRowGapPt),
		Rows:          b.layout().rows,
		VerticalAlign: GridVerticalAlignDefault,
	}

	return grid, nil
}

func newPhaseRoadmapBuild(ctx ExpandContext, vals *PhaseRoadmapValues, ovr *PhaseRoadmapOverrides, cellOverrides map[int]any) phaseRoadmapBuild {
	if ovr == nil {
		ovr = &PhaseRoadmapOverrides{}
	}
	n := len(vals.Phases)
	hasMilestones, hasPanels, hasDescriptions := false, false, false
	for _, p := range vals.Phases {
		hasMilestones = hasMilestones || p.Milestone != ""
		hasPanels = hasPanels || p.DateLabel != "" || p.Description != ""
		hasDescriptions = hasDescriptions || p.Description != ""
	}

	// Cell index layout (used by cell_overrides). The indices keep the order
	// they had when the roadmap was a box row over a timeline rule, so a deck
	// written against it still validates:
	//   0..n-1     : phase shapes of the band
	//   n          : reserved (the former timeline rule; the band has no
	//                separate spine, so an override here changes nothing)
	//   n+1..2n    : date ranges (the bold first line of each panel)
	//   2n+1..3n   : milestone markers (only when any phase sets a milestone)
	//   then       : descriptions (the body of each panel)
	//   then       : parallel-track label, then one bar per track (only when
	//                parallel_tracks is non-empty)
	descIdx0 := 2*n + 1
	if hasMilestones {
		descIdx0 = 3*n + 1
	}
	return phaseRoadmapBuild{
		ctx: ctx, vals: vals, cellOverrides: cellOverrides,
		accent:          ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent),
		headerSize:      ResolveSize(ovr.HeaderSize, scaleSubheadPt),
		headerAuthored:  ovr.HeaderSize > 0,
		bodySize:        ResolveSize(ovr.BodySize, scaleCaptionPt),
		bodyAuthored:    ovr.BodySize > 0,
		hasMilestones:   hasMilestones,
		hasPanels:       hasPanels,
		hasDescriptions: hasDescriptions,
		phaseIdx0:       0, dateIdx0: n + 1, milestoneIdx0: 2*n + 1, descIdx0: descIdx0,
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
	// hasPanels: some phase carries a date range or a description, so the
	// panel row is drawn.
	hasPanels bool
	// hasDescriptions: some phase carries a description. Panels that hold a
	// date range and nothing else stay at the height of that line: grown,
	// they stand empty under the dates (go-slide-creator-v6f8j).
	hasDescriptions bool
	// headerAuthored / bodyAuthored: overrides.header_size / body_size was
	// given, so the fit steps keep it.
	headerAuthored bool
	bodyAuthored   bool

	phaseIdx0, dateIdx0, milestoneIdx0, descIdx0 int
}

// Geometry of the band and its panels (go-slide-creator-dlfm6).
const (
	// phaseRoadmapColGapPt is the column gap: the slanted hairline between
	// two interlocking phase shapes, not scaled with the template gutter.
	phaseRoadmapColGapPt = valueChainArrowGapPt
	// phaseRoadmapBandPt is the band's height at the roomy fit step; a name
	// that wraps grows it.
	phaseRoadmapBandPt = 50.0
	// phaseRoadmapDatePt is the date range's size: the panel's bold lead.
	phaseRoadmapDatePt = scaleSubheadPt
	// phaseRoadmapDateGapPt is the space between a panel's date range and its
	// description.
	phaseRoadmapDateGapPt = 4.0
	// phaseRoadmapLeadPt is the description size a roadmap of four phases or
	// fewer takes when its panels hold it with room to spare
	// (phaseRoadmapLeadHeadroom); denser copy keeps the body size.
	phaseRoadmapLeadPt       = scaleSubheadPt
	phaseRoadmapLeadMaxCount = 4
	phaseRoadmapLeadHeadroom = 1.25
	// phaseRoadmapFillFrac is the share of the content area the roadmap grows
	// its panels to, and phaseRoadmapPanelStretch the most a panel grows over
	// the height its text needs: equal columns under the band, not a strip in
	// the middle of the slide and not a slab around one line.
	phaseRoadmapFillFrac     = 0.8
	phaseRoadmapPanelStretch = 2.6
	// phaseRoadmapTrackPointPt is the depth of a parallel track's point.
	phaseRoadmapTrackPointPt = 10.0
)

// colWPt is one phase column's width.
func (b phaseRoadmapBuild) colWPt() float64 {
	contentW, _ := contentAreaPt(b.ctx)
	return equalColumnWidthPt(contentW, len(b.vals.Phases), phaseRoadmapColGapPt)
}

// chevrons fits the band's shapes at one fit step: the point depth and the
// name size at which every name's words stay whole inside their shape
// (fitValueChainArrows — the band is a value-chain arrow row).
func (b phaseRoadmapBuild) chevrons(fit phaseRoadmapFit) valueChainArrowFit {
	size := b.headerSize
	if fit.headerSize > 0 {
		size = fit.headerSize
	}
	steps := make([]ValueChainStep, len(b.vals.Phases))
	for i, p := range b.vals.Phases {
		steps[i] = ValueChainStep{Label: p.Name}
	}
	f := fitValueChainArrows(b.ctx, steps, size)
	f.rowHPt = phaseRoadmapBandPt
	if fit.bandPt > 0 {
		f.rowHPt = fit.bandPt
	}
	f.notchPt = math.Min(f.notchPt, math.Max(math.Round(f.rowHPt*valueChainNotchFrac), valueChainMinNotchPt))
	return f
}

// bandRow builds the phase band: a pentagon followed by chevrons, each
// reaching left over the gap so its notch takes the point before it. The
// current phase is the one solid accent shape; the others are a light tint of
// the accent, and each name's ink is measured on its own fill.
func (b phaseRoadmapBuild) bandRow(fit phaseRoadmapFit) jsonschema.GridRowInput {
	ctx, accent := b.ctx, b.accent
	chev := b.chevrons(fit)
	cells := make([]*jsonschema.GridCellInput, len(b.vals.Phases))
	for i, p := range b.vals.Phases {
		tone := inactiveTintTone(accent)
		if p.Active {
			tone = fillTone{Color: accent}
		}
		ink := readableTextOn(ctx, tone, phaseRoadmapFallbackText(p.Active))
		cells[i] = &jsonschema.GridCellInput{
			BleedLeft: chev.bleedPt(i),
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:  chev.geometry(i),
				TypeScale: peerTextTypeScale,
				Fill:      tone.fillJSON(),
				Line:      noLine,
				Text:      withTextInsets(buildPhaseRoadmapHeaderText(pptx.ConvertMarkdownEmphasis(p.Name), chev.labelPt, ink), valueChainArrowInsetPt),
			},
		}
		applyPhaseRoadmapOverride(cells[i], b.cellOverrides, b.phaseIdx0+i, accent)
	}
	h := chev.rowHeightPt(ctx.themeFonts(), cells)
	for i, c := range cells {
		c.Shape.Adjustments = map[string]int64{"adj": chev.adj(i, h)}
	}
	return jsonschema.GridRowInput{MinHeight: h, MaxHeight: h, Cells: cells}
}

// panelTone is the panel surface: the lightest neutral step, or the
// template's own subtle surface where it declares one that stays lighter than
// the band's tint — a panel darker than the phase that heads it would lead.
func (b phaseRoadmapBuild) panelTone() fillTone {
	tone := neutralTone(NeutralTint4)
	v, ok := declaredSurface(b.ctx, "subtle")
	if !ok || isPageColor(v) {
		return tone
	}
	declared := fillTone{Color: v}
	panel, okP := effectiveFillColor(b.ctx, declared)
	band, okB := effectiveFillColor(b.ctx, inactiveTintTone(b.accent))
	if okP && okB && panel.Luminance() < band.Luminance()+phaseRoadmapPanelLighterBy {
		return tone
	}
	return declared
}

// phaseRoadmapPanelLighterBy is how much lighter (relative luminance) than the
// band's tint a declared panel surface must be to be kept.
const phaseRoadmapPanelLighterBy = 0.05

// lightInk is the ink of the roadmap's text that sits on the slide itself
// (milestone and track labels): the ink the panels' text takes.
func (b phaseRoadmapBuild) lightInk() string {
	return readableTextOn(b.ctx, b.panelTone(), "dk1")
}

// panelRow builds the row of panels under the band — one per phase, in the
// lightest neutral surface, holding the date range (bold) over the
// description — and returns the height the tallest panel's text needs. A
// "- " description line is a native bullet.
func (b phaseRoadmapBuild) panelRow(fit phaseRoadmapFit, descSize float64) (jsonschema.GridRowInput, float64) {
	ctx := b.ctx
	fill := b.panelTone().fillJSON()
	ink := b.lightInk()
	colW := b.colWPt()
	textW := colW - 2*defaultShapeInsetLRPt
	margin := 2*defaultShapeInsetTBPt - rowPadTrimPt(fit.rowPad)
	dateSize := phaseRoadmapDatePt
	if b.bodyAuthored {
		dateSize = b.bodySize
	}
	need := 0.0
	cells := make([]*jsonschema.GridCellInput, len(b.vals.Phases))
	for i, p := range b.vals.Phases {
		cells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", TypeScale: peerTextTypeScale, Fill: fill, Line: noLine},
		}
		var paras []map[string]any
		model := 0.0
		if p.DateLabel != "" {
			date := map[string]any{"content": pptx.ConvertMarkdownEmphasis(p.DateLabel), "size": dateSize, "bold": true, "color": ink, "align": "l"}
			if p.Description != "" {
				date["space_after"] = phaseRoadmapDateGapPt
				model += phaseRoadmapDateGapPt
			}
			paras = append(paras, b.overridden([]map[string]any{date}, b.dateIdx0+i)...)
			model += textBlockHeightPt(ctx.Theme.BodyFont, textW, textParagraph{text: p.DateLabel, size: dateSize, bold: true})
		}
		if p.Description != "" {
			var desc []map[string]any
			for _, line := range strings.Split(pptx.ConvertMarkdownEmphasis(p.Description), "\n") {
				para := map[string]any{"size": descSize, "color": ink, "align": "l"}
				if rest, ok := strings.CutPrefix(line, "- "); ok {
					line = rest
					para["bullet"] = true
				}
				if strings.TrimSpace(line) == "" {
					continue
				}
				para["content"] = line
				desc = append(desc, para)
			}
			paras = append(paras, b.overridden(desc, b.descIdx0+i)...)
			model += phaseRoadmapDescHeightPt(ctx.Theme.BodyFont, textW, p.Description, descSize)
		}
		b.panelAccentBar(cells[i], i)
		if len(paras) == 0 {
			continue
		}
		text := withRowPad(marshalRaw(map[string]any{"paragraphs": paras, "align": "l", "vertical_align": "t"}), fit.rowPad)
		cells[i].Shape.Text = text
		// The theme-font model alone is not enough: a Calibri template is
		// measured with its metric clone, narrower than the writer's autofit
		// font, so the row is held at the writer's own fit too.
		need = math.Max(need, math.Max(math.Round(model+margin), math.Ceil(writtenNeedOrOverflowPt(ctx.Theme.BodyFont, text, colW))))
	}
	return jsonschema.GridRowInput{MinHeight: need, MaxHeight: need, Cells: cells}, need
}

// overridden applies the cell override at idx to one part of a panel's text
// (its date range or its description), so each keeps its own index.
func (b phaseRoadmapBuild) overridden(paras []map[string]any, idx int) []map[string]any {
	co, ok := b.cellOverrides[idx].(*PhaseRoadmapCellOverride)
	if !ok || co == nil || len(paras) == 0 {
		return paras
	}
	var out struct {
		Paragraphs []map[string]any `json:"paragraphs"`
	}
	if json.Unmarshal(applyCellTextOverrideToText(marshalRaw(map[string]any{"paragraphs": paras}), co), &out) != nil || len(out.Paragraphs) != len(paras) {
		return paras
	}
	return out.Paragraphs
}

// panelAccentBar gives phase i's panel the top accent bar when the cell
// override of its date range or its description asks for one.
func (b phaseRoadmapBuild) panelAccentBar(cell *jsonschema.GridCellInput, i int) {
	for _, idx := range []int{b.dateIdx0 + i, b.descIdx0 + i} {
		if co, ok := b.cellOverrides[idx].(*PhaseRoadmapCellOverride); ok && co != nil && co.AccentBar {
			cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: b.accent, Width: 4}
		}
	}
}

// milestoneRow builds the optional milestone row at one fit step. It sits
// directly on the band: each milestone is an accent diamond and a bold label
// over the phase it belongs to.
func (b phaseRoadmapBuild) milestoneRow(fit phaseRoadmapFit, milestoneSize float64) jsonschema.GridRowInput {
	ctx, vals, cellOverrides, accent := b.ctx, b.vals, b.cellOverrides, b.accent
	n := len(vals.Phases)
	milestoneIdx0, ink := b.milestoneIdx0, b.lightInk()
	// A label that wraps at the column width keeps the two-line row the
	// milestone budgets were measured with, as a plain cell (a marker
	// sub-grid would lose its inset to an already squeezed row); short
	// labels get the one-line diamond marker. One wrapping label switches
	// the whole row, so the markers stay consistent.
	colW := b.colWPt()
	msH := phaseRoadmapMilestoneRowPt(milestoneSize)
	wraps := false
	for _, p := range vals.Phases {
		if p.Milestone != "" && !writtenFitsAt(ctx.themeFonts(), phaseRoadmapMilestoneText(pptx.ConvertMarkdownEmphasis(p.Milestone), milestoneSize, ink), colW, msH) {
			wraps = true
		}
	}
	if wraps {
		msH = math.Round(shapegrid.EffectiveTextSizePt(milestoneSize)*contentLineHeight*2 + 2*defaultShapeInsetTBPt)
	}
	// A marker's label has no margin of its own, so its row is the label's
	// line and the marker sits directly on the band; a wrapping label
	// carries the row padding, so a tightened row gives up only air.
	floor := 0.0
	for _, p := range vals.Phases {
		if p.Milestone == "" {
			continue
		}
		label := pptx.ConvertMarkdownEmphasis(p.Milestone)
		if wraps {
			floor = math.Max(floor, writtenFitHeightPt(ctx.themeFonts(), withRowPad(phaseRoadmapMilestoneText(label, milestoneSize, ink), fit.rowPad), colW, 0))
			continue
		}
		floor = math.Max(floor, phaseRoadmapMarkerRowPt(ctx, phaseRoadmapMilestoneCell(colW, label, milestoneSize, accent, ink), colW, msH))
	}
	if wraps {
		msH = math.Max(msH-rowPadTrimPt(fit.rowPad), math.Ceil(floor))
	} else {
		msH = math.Ceil(floor)
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
			// Anchored to the bottom of the row, so the label sits on the
			// band whether it takes one line or two.
			milestoneCells[i] = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"none"`),
					Text:     withRowPad(withVerticalAlign(phaseRoadmapMilestoneText(label, milestoneSize, ink), "b"), fit.rowPad),
				},
			}
		default:
			milestoneCells[i] = phaseRoadmapMilestoneCell(colW, label, milestoneSize, accent, ink)
		}
		applyPhaseRoadmapOverride(milestoneCells[i], cellOverrides, milestoneIdx0+i, accent)
	}
	return jsonschema.GridRowInput{MinHeight: msH, MaxHeight: msH, Cells: milestoneCells}
}

// phaseRoadmapCellCount returns the total addressable cell count for
// cell_overrides validation. Layout:
//
//	phases (n) + reserved (1) + date ranges (n) + descriptions (n)
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
// the fit step's bar padding, so two tracks stay lighter than the phase band
// they run under instead of taking a third of the content area
// (go-slide-creator-x1124). It speaks the band's language: a pointed bar in
// the tint of the phases that are not current, running the width of the
// roadmap (go-slide-creator-dlfm6).
func buildPhaseRoadmapTracks(ctx ExpandContext, vals *PhaseRoadmapValues, accent, labelInk string, bodySize, headerSize float64, fit phaseRoadmapFit) *phaseRoadmapTracks {
	k := len(vals.ParallelTracks)
	contentW, _ := contentAreaPt(ctx)
	tone := inactiveTintTone(accent)
	textColor := readableTextOn(ctx, tone, "dk1")
	// The pointed bar's own text rectangle stops short of its point.
	barW := contentW*(100-phaseRoadmapTrackLabelColPct)/100 - ctx.Gap(phaseRoadmapTrackGapPt)
	barTextW := barW - phaseRoadmapTrackPointPt - 2*defaultShapeInsetLRPt
	barTextH, barWrittenH := shapegrid.EffectiveTextSizePt(bodySize)*contentLineHeight, 0.0
	texts := make([]json.RawMessage, k)
	for i, t := range vals.ParallelTracks {
		barTextH = math.Max(barTextH, textBlockHeightPt(ctx.Theme.BodyFont, barTextW, textParagraph{text: t, size: bodySize}))
		// As for the panels: the writer's fit, not only the theme-font
		// model, sets the bar height.
		texts[i] = withRowPad(withVerticalAlign(buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(t), bodySize, false, textColor, "l"), "ctr"), fit.barPad)
		barWrittenH = math.Max(barWrittenH, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, texts[i], barTextW+2*defaultShapeInsetLRPt))
	}
	barPt := math.Max(math.Round(barTextH+2*defaultShapeInsetTBPt-rowPadTrimPt(fit.barPad)), math.Ceil(barWrittenH))

	labelSize := math.Max(bodySize+1, math.Min(headerSize-2, 12))
	label := &jsonschema.GridCellInput{
		RowSpan: k,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     buildPhaseRoadmapPlainText(pptx.ConvertMarkdownEmphasis(vals.parallelLabel()), labelSize, true, labelInk, "l"),
		},
	}
	// Centre the label on the stacked bars rather than pinning it to the top;
	// beside a single slim bar it takes the bar's padding. Its text starts
	// where the panels' text starts: the nested grid's own inset comes off
	// the uniform margin.
	label.Shape.Text = withTextInsetSides(withRowPad(withVerticalAlign(label.Shape.Text, "ctr"), fit.barPad), defaultShapeInsetLRPt-SubGridInsetPt, "inset_left")

	bars := make([]*jsonschema.GridCellInput, k)
	rows := make([]jsonschema.GridRowInput, k)
	for i := range vals.ParallelTracks {
		bars[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "homePlate",
				TypeScale:   peerTextTypeScale,
				Adjustments: map[string]int64{"adj": int64(math.Round(phaseRoadmapTrackPointPt / math.Min(barPt, barW) * 100000))},
				Fill:        tone.fillJSON(),
				Line:        noLine,
				Text:        texts[i],
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

// Milestone marker geometry: the diamond's size and the column it sits in.
const (
	phaseRoadmapMilestoneMarkerPt = 12.0
	phaseRoadmapMilestoneColPt    = 14.0
	// phaseRoadmapMilestoneGapPt is the gap between the diamond and its label
	// (an explicit one: a gap of 0 reads as unset and takes the 8pt default).
	phaseRoadmapMilestoneGapPt = 4.0
)

// phaseRoadmapMilestoneRowPt is the milestone row: one label line plus the
// uniform shape margin.
func phaseRoadmapMilestoneRowPt(size float64) float64 {
	return math.Round(shapegrid.EffectiveTextSizePt(size)*contentLineHeight + 2*defaultShapeInsetTBPt)
}

// phaseRoadmapMilestoneText is a milestone label: bold, left-aligned,
// centred in its row.
func phaseRoadmapMilestoneText(label string, size float64, ink string) json.RawMessage {
	return withVerticalAlign(buildPhaseRoadmapPlainText(label, size, true, ink, "l"), "ctr")
}

// phaseRoadmapMarkerPct is the milestone marker column's share of a
// subW-wide milestone sub-grid.
func phaseRoadmapMarkerPct(subW float64) float64 {
	if subW <= 0 {
		return 20
	}
	return math.Min(20, phaseRoadmapMilestoneColPt/subW*100)
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
	labelW := (colW - 2*SubGridInsetPt - phaseRoadmapMilestoneGapPt) * cols[1] / 100
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

// phaseRoadmapMilestoneCell is one milestone marker in a colW-wide column: an
// accent diamond beside a one-line bold label with no fill.
func phaseRoadmapMilestoneCell(colW float64, label string, size float64, accent, ink string) *jsonschema.GridCellInput {
	markerPct := phaseRoadmapMarkerPct(colW - 2*SubGridInsetPt)
	text := buildPhaseRoadmapPlainText(label, size, true, ink, "l")
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
			ColGap:  phaseRoadmapMilestoneGapPt,
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

// phaseRoadmapFit is one step of the vertical tightening a roadmap takes when
// its rows do not fit the content area at the uniform 0.5 cm text margin.
type phaseRoadmapFit struct {
	barPad     float64 // top / bottom margin of a track bar
	topPad     float64 // air between the panels and the track block
	rowPad     float64 // top / bottom margin of the milestone row and the panels; 0 = uniform
	bandPt     float64 // band height; 0 = phaseRoadmapBandPt
	headerSize float64 // phase-name size; 0 = the resolved header size
}

// Band heights of the slimmer fit steps.
const (
	phaseRoadmapBandSlimPt  = 40.0
	phaseRoadmapBandSlimmer = 32.0
)

// phaseRoadmapFitSteps are the steps a roadmap tries, roomiest first. Air
// gives way before type: the padding of the milestone row and the panels
// (rowPadStepsPt, the steps a ruled list takes), then the track bars and the
// gap above them, then the band becomes slimmer, and only then does the phase
// name step down to the 12pt body floor — never below it, and an authored
// header size is kept. Descriptions, dates and milestones keep their size
// throughout (go-slide-creator-x1124).
func phaseRoadmapFitSteps(headerSize float64, headerAuthored bool) []phaseRoadmapFit {
	f := phaseRoadmapFit{barPad: phaseRoadmapTrackBarPadPt, topPad: phaseRoadmapTrackTopPadPt}
	steps := []phaseRoadmapFit{f}
	f.rowPad = rowPadStepsPt[0]
	steps = append(steps, f)
	f.rowPad, f.barPad = rowPadStepsPt[1], rowPadStepsPt[2]
	steps = append(steps, f)
	f.rowPad, f.topPad = rowPadStepsPt[2], phaseRoadmapTrackGapPt
	steps = append(steps, f)
	f.bandPt = phaseRoadmapBandSlimPt
	steps = append(steps, f)
	f.bandPt = phaseRoadmapBandSlimmer
	steps = append(steps, f)
	if !headerAuthored && headerSize > scaleBodyPt {
		f.headerSize = scaleBodyPt
		steps = append(steps, f)
	}
	return steps
}

// phaseRoadmapLaid is a roadmap laid out at one fit step, every row pinned in
// points.
type phaseRoadmapLaid struct {
	rows    []jsonschema.GridRowInput
	fit     phaseRoadmapFit
	needPt  float64 // smallest height the rows hold their text in at this step
	areaPt  float64 // height the grid may use
	panelPt float64 // the panel row's text need (0 without panels)
	barPt   float64 // one track bar (0 without tracks)
	descPt  float64 // description size the panels are set in
}

// fits reports whether the rows hold their text inside the content area.
func (t phaseRoadmapLaid) fits() bool { return t.needPt <= t.areaPt }

// layout lays the roadmap out at the first fit step that holds every row
// inside the content area, or at the tightest step when none does
// (PostExpandWarnings then reports what to cut). A roadmap of four phases or
// fewer first tries its descriptions at the lead size.
func (b phaseRoadmapBuild) layout() phaseRoadmapLaid {
	steps := phaseRoadmapFitSteps(b.headerSize, b.headerAuthored)
	if !b.bodyAuthored && b.hasPanels && len(b.vals.Phases) <= phaseRoadmapLeadMaxCount {
		if t := b.layoutAt(steps[0], phaseRoadmapLeadPt); t.fits() && t.panelPt*phaseRoadmapLeadHeadroom <= t.areaPt-(t.needPt-t.panelPt) {
			return t
		}
	}
	var t phaseRoadmapLaid
	for _, fit := range steps {
		if t = b.layoutAt(fit, b.bodySize); t.fits() {
			break
		}
	}
	return t
}

// layoutAt pins every row of the roadmap in points at one fit step: the
// milestone row, the band and the track block at their measured heights, and
// the panels at the height their tallest text needs, grown — equally — toward
// phaseRoadmapFillFrac of the content area when it has the room and some
// phase has a description to set in it (panels of date ranges alone are
// content-sized).
func (b phaseRoadmapBuild) layoutAt(fit phaseRoadmapFit, descSize float64) phaseRoadmapLaid {
	ctx := b.ctx
	_, areaH := sizingAreaPt(ctx)
	// A point in hand so rounding never over-commits the grid.
	t := phaseRoadmapLaid{fit: fit, areaPt: areaH - 1, descPt: descSize}
	var rows []jsonschema.GridRowInput
	if b.hasMilestones {
		rows = append(rows, b.milestoneRow(fit, b.bodySize))
	}
	rows = append(rows, b.bandRow(fit))
	panelIdx := -1
	if b.hasPanels {
		row, need := b.panelRow(fit, descSize)
		panelIdx, t.panelPt = len(rows), need
		rows = append(rows, row)
	}
	if n := len(b.vals.Phases); len(b.vals.ParallelTracks) > 0 {
		tracks := buildPhaseRoadmapTracks(ctx, b.vals, b.accent, b.lightInk(), b.bodySize, b.headerSize, fit)
		trackIdx0 := b.descIdx0 + n
		applyPhaseRoadmapOverride(tracks.label, b.cellOverrides, trackIdx0, b.accent)
		for i, bar := range tracks.bars {
			applyPhaseRoadmapOverride(bar, b.cellOverrides, trackIdx0+1+i, b.accent)
		}
		t.barPt = tracks.barPt
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: tracks.rowPt,
			MaxHeight: tracks.rowPt,
			Cells:     []*jsonschema.GridCellInput{{ColSpan: n, Grid: tracks.grid}},
		})
	}
	t.needPt = float64(len(rows)-1) * ctx.Gap(phaseRoadmapRowGapPt)
	for _, r := range rows {
		t.needPt += r.MinHeight
	}
	if slack := t.areaPt - t.needPt; panelIdx >= 0 && b.hasDescriptions && slack > 0 {
		want := areaH*phaseRoadmapFillFrac - (t.needPt - t.panelPt)
		h := math.Floor(math.Min(clampPt(want, t.panelPt, t.panelPt*phaseRoadmapPanelStretch), t.panelPt+slack))
		rows[panelIdx].MinHeight, rows[panelIdx].MaxHeight = h, h
	}
	t.rows = rows
	return t
}

// phaseRoadmapFitWarnings reports a roadmap that does not fit its content
// area at the tightest fit step, with a budget per field: each description
// that is too long is told how many characters the area holds; when the
// descriptions are not what overflows, the last track is told how many tracks
// the area has room for (go-slide-creator-x1124).
func phaseRoadmapFitWarnings(b phaseRoadmapBuild) []string {
	t := b.layout()
	if t.fits() {
		return nil
	}
	vals := b.vals
	n, k := len(vals.Phases), len(vals.ParallelTracks)
	beside := ""
	switch {
	case k == 1:
		beside = "1 parallel track"
	case k > 1:
		beside = fmt.Sprintf("%d parallel tracks", k)
	}
	drop := "drop a parallel track"
	if b.hasMilestones {
		if k == 0 {
			beside, drop = "the milestone row", "drop the milestones"
		} else {
			beside += " and the milestone row"
			drop += " or the milestones"
		}
	}
	if beside == "" {
		drop = "use fewer phases"
	} else {
		beside = "beside " + beside + " "
	}

	// Height the descriptions may take with every other row as it is.
	textW := b.colWPt() - 2*defaultShapeInsetLRPt
	lineH := shapegrid.EffectiveTextSizePt(t.descPt) * contentLineHeight
	margin := 2*defaultShapeInsetTBPt - rowPadTrimPt(t.fit.rowPad)
	dateH := 0.0
	for _, p := range vals.Phases {
		if p.DateLabel != "" {
			dateH = math.Max(dateH, phaseRoadmapDateGapPt+textBlockHeightPt(b.ctx.Theme.BodyFont, textW, textParagraph{text: p.DateLabel, size: phaseRoadmapDatePt, bold: true}))
		}
	}
	lines := int(math.Floor((t.areaPt - (t.needPt - t.panelPt) - margin - dateH) / lineH))
	var warnings []string
	if lines >= 1 {
		for i, p := range vals.Phases {
			budget := phaseRoadmapLineBudget(b.ctx.Theme.BodyFont, p.Description, t.descPt, textW, lines)
			if budget <= 0 || runeLen(p.Description) <= budget {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s: phase-roadmap phases[%d].description is %d characters; %sthis content area (%.0fpt high) holds about %d description characters per phase (%s) — shorten the description, or %s",
				ErrCodeBodyTooLong, i, runeLen(p.Description), beside, t.areaPt, budget, countNoun(lines, "line"), drop))
		}
	}
	if len(warnings) > 0 {
		return warnings
	}
	if k == 0 {
		return []string{fmt.Sprintf("%s: phase-roadmap phases[%d]: %d phases %sneed about %.0fpt but the content area is %.0fpt high — shorten the names and descriptions, %s, or free height on the slide",
			ErrCodeBodyTooLong, n-1, n, beside, t.needPt, t.areaPt, drop)}
	}
	room := k - int(math.Ceil((t.needPt-t.areaPt)/(t.barPt+b.ctx.Gap(phaseRoadmapTrackGapPt))))
	advice := fmt.Sprintf("has room for %s", countNoun(max(room, 0), "track"))
	if room < 1 {
		advice = "has no room for a track block"
	}
	return []string{fmt.Sprintf("%s: phase-roadmap parallel_tracks[%d]: %d phases with %s need about %.0fpt but the content area is %.0fpt high and %s — %s, shorten the descriptions, or free height on the slide",
		ErrCodeBodyTooLong, k-1, n, strings.TrimSuffix(strings.TrimPrefix(beside, "beside "), " "), t.needPt, t.areaPt, advice, drop)}
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
