package semantic

// This file defines the semantic compiler's intermediate representation (IR):
// a normalized, planned view of a DeckSpec produced before the raw
// internal/deckinput.PresentationInput is emitted. The IR is the single place
// where deck rhythm, per-slide density, and visual-family policy are applied,
// so the later semantic-to-raw compiler phase (and the explain scaffold) read
// decisions from one source of truth rather than re-deriving them.
//
// Normalize is intentionally tolerant: an unknown or malformed slide kind
// yields a passthrough SlideIR rather than an error, so Explain works even on
// specs that still carry validation findings. The raw-pointer entries in the
// SourceMap are left empty here; the compiler populates them as it emits the
// PresentationInput in a later phase.

// NarrativeRole names the role a slide plays in the deck's narrative arc. It is
// derived from the slide kind and biases rhythm and emphasis decisions.
type NarrativeRole string

const (
	// RoleOpening is the deck's opening (title) slide.
	RoleOpening NarrativeRole = "opening"
	// RoleTransition is a section divider between content groups.
	RoleTransition NarrativeRole = "transition"
	// RoleSummary is a high-level summary of the deck's messages.
	RoleSummary NarrativeRole = "summary"
	// RoleEvidence presents supporting data (KPIs, charts).
	RoleEvidence NarrativeRole = "evidence"
	// RoleAnalysis interprets or contrasts evidence (comparison, process).
	RoleAnalysis NarrativeRole = "analysis"
	// RolePlan lays out a forward-looking plan (roadmap).
	RolePlan NarrativeRole = "plan"
	// RoleRecommendation frames a decision and its recommendation.
	RoleRecommendation NarrativeRole = "recommendation"
	// RoleClosing is the deck's closing slide.
	RoleClosing NarrativeRole = "closing"
	// RolePassthrough is an unrecognized or raw escape-hatch slide whose role is
	// not modeled by the compiler.
	RolePassthrough NarrativeRole = "passthrough"
)

// VisualFamily names the broad visual treatment a slide will receive. It drives
// monotony detection in the deck-level RhythmPlan.
type VisualFamily string

const (
	// FamilyStructural is title/section/closing chrome with minimal content.
	FamilyStructural VisualFamily = "structural"
	// FamilyText is prose- or bullet-dominated content.
	FamilyText VisualFamily = "text"
	// FamilyKPI is a big-number KPI treatment.
	FamilyKPI VisualFamily = "kpi"
	// FamilyChart is a chart-led treatment.
	FamilyChart VisualFamily = "chart"
	// FamilyComparison is a side-by-side comparison treatment.
	FamilyComparison VisualFamily = "comparison"
	// FamilyProcess is a sequential process/flow treatment.
	FamilyProcess VisualFamily = "process"
	// FamilyTimeline is a phased roadmap/timeline treatment.
	FamilyTimeline VisualFamily = "timeline"
	// FamilyRaw is an unmodeled raw/passthrough slide.
	FamilyRaw VisualFamily = "raw"
)

// Density estimates how much content a slide carries, which informs pattern
// selection and overflow risk in later phases.
type Density string

const (
	// DensityLight is sparse content (structural chrome, a single statistic).
	DensityLight Density = "light"
	// DensityMedium is a typical content slide.
	DensityMedium Density = "medium"
	// DensityHeavy is a dense slide at risk of overflow.
	DensityHeavy Density = "heavy"
)

// VisualPlan captures the planned visual treatment of a single slide: its
// family, estimated density, and the selected named pattern and/or layout. A
// structural slide carries an empty Pattern and a non-empty Layout; a
// pattern-driven content slide carries both.
type VisualPlan struct {
	Family  VisualFamily `json:"family"`
	Density Density      `json:"density"`
	// Pattern is the selected named pattern (see internal/patterns), or "" when
	// the slide is rendered from a layout alone.
	Pattern string `json:"pattern,omitempty"`
	// Layout is the selected json2pptx slide_type / layout tag.
	Layout string `json:"layout,omitempty"`
}

// SlideIR is the normalized, planned view of one semantic slide.
type SlideIR struct {
	// SourceIndex is the slide's index in DeckSpec.Slides.
	SourceIndex int `json:"source_index"`
	// Kind is the semantic slide kind.
	Kind SlideKind `json:"kind"`
	// Title is the slide title extracted from the payload (may be empty).
	Title string `json:"title,omitempty"`
	// Takeaway is the slide's one-line takeaway (or insight), if present.
	Takeaway string `json:"takeaway,omitempty"`
	// Role is the slide's narrative role.
	Role NarrativeRole `json:"role"`
	// Visual is the planned visual treatment.
	Visual VisualPlan `json:"visual"`
	// Body is the carried kind-specific payload, retained for the compiler.
	Body map[string]any `json:"-"`
}

// RhythmPlan summarizes deck-level rhythm: how slides distribute across visual
// families and densities. It backs monotony/imbalance detection without
// re-walking the slides.
type RhythmPlan struct {
	// SlideCount is the total number of slides.
	SlideCount int `json:"slide_count"`
	// Families counts slides per visual family.
	Families map[VisualFamily]int `json:"families"`
	// Densities counts slides per density bucket.
	Densities map[Density]int `json:"densities"`
}

// DeckIR is the compiler intermediate representation for a whole deck: the
// normalized meta, the planned slides, the deck-level rhythm summary, and the
// raw↔semantic SourceMap that later phases populate.
type DeckIR struct {
	Title     string    `json:"title,omitempty"`
	Subtitle  string    `json:"subtitle,omitempty"`
	Archetype Archetype `json:"archetype,omitempty"`
	Template  string    `json:"template,omitempty"`
	Audience  string    `json:"audience,omitempty"`
	Author    string    `json:"author,omitempty"`
	Date      string    `json:"date,omitempty"`
	// ArchetypeTemplate is the template the deck's archetype prefers (empty when
	// no archetype is set or it has no preference). It is the lowest-priority
	// template source: the spec's own Template wins, then a caller default, then
	// this. Carried on the IR so compile and explain resolve it identically.
	ArchetypeTemplate string `json:"archetype_template,omitempty"`
	// Executive is true when the deck's archetype expects a synthesis/decision
	// slide; it drives the rhythm synthesis rule.
	Executive bool `json:"executive,omitempty"`
	// Slides are the normalized, planned slides in source order.
	Slides []SlideIR `json:"slides"`
	// Rhythm is the deck-level rhythm summary.
	Rhythm RhythmPlan `json:"rhythm"`
	// SourceMap maps generated raw JSON pointers back to semantic paths. It is
	// empty until the compiler emits PresentationInput in a later phase.
	SourceMap *SourceMap `json:"-"`
}

// kindPlan is the per-kind planning template applied during normalization. The
// pattern selector may inspect the slide payload (e.g. KPI count) to choose a
// concrete named pattern; a nil selector means the kind is layout-only.
type kindPlan struct {
	role    NarrativeRole
	family  VisualFamily
	density Density
	layout  string
	pattern func(body map[string]any) string
}

// kindPlanRegistry maps each registered slide kind to its base plan. Unknown
// kinds fall back to passthroughPlan.
var kindPlanRegistry = map[SlideKind]kindPlan{
	KindTitle:   {role: RoleOpening, family: FamilyStructural, density: DensityLight, layout: "title"},
	KindSection: {role: RoleTransition, family: FamilyStructural, density: DensityLight, layout: "section"},
	KindExecutiveSummary: {
		role: RoleSummary, family: FamilyText, density: DensityMedium, layout: "content",
		pattern: func(map[string]any) string { return "scqa-summary" },
	},
	KindKPISnapshot: {
		role: RoleEvidence, family: FamilyKPI, density: DensityMedium, layout: "content",
		pattern: kpiPattern,
	},
	KindChartInsight: {
		role: RoleEvidence, family: FamilyChart, density: DensityMedium, layout: "chart",
		pattern: func(map[string]any) string { return "chart-insights-split" },
	},
	KindComparison: {
		role: RoleAnalysis, family: FamilyComparison, density: DensityMedium, layout: "two-column",
		pattern: func(map[string]any) string { return "comparison-2col" },
	},
	KindProcess: {
		role: RoleAnalysis, family: FamilyProcess, density: DensityMedium, layout: "diagram",
		pattern: func(map[string]any) string { return "process-flow" },
	},
	KindRoadmap: {
		role: RolePlan, family: FamilyTimeline, density: DensityMedium, layout: "diagram",
		pattern: func(map[string]any) string { return "phase-roadmap" },
	},
	KindDecision: {
		role: RoleRecommendation, family: FamilyText, density: DensityMedium, layout: "content",
		pattern: func(map[string]any) string { return "scqa-summary" },
	},
	KindClosing:      {role: RoleClosing, family: FamilyStructural, density: DensityLight, layout: "title"},
	KindRawJSON2pptx: passthroughPlan,
}

// passthroughPlan is the fallback for unknown or raw escape-hatch slides.
var passthroughPlan = kindPlan{role: RolePassthrough, family: FamilyRaw, density: DensityMedium, layout: ""}

// kpiPattern selects the kpi-Nup pattern matching the declared KPI count,
// clamped to the registered 2up–6up range.
func kpiPattern(body map[string]any) string {
	n, _ := listLen(body, "kpis")
	switch {
	case n <= 2:
		return "kpi-2up"
	case n == 3:
		return "kpi-3up"
	case n == 4:
		return "kpi-4up"
	case n == 5:
		return "kpi-5up"
	default:
		return "kpi-6up"
	}
}

// Normalize converts a DeckSpec into a planned DeckIR, applying the per-kind
// rhythm/visual plan, computing the deck-level RhythmPlan, and attaching an
// empty SourceMap for the compiler to populate. A nil spec yields an empty IR
// with an initialized (empty) SourceMap and rhythm, so callers can always
// Explain the result safely.
func Normalize(spec *DeckSpec) *DeckIR {
	ir := &DeckIR{SourceMap: NewSourceMap(), Rhythm: newRhythmPlan()}
	if spec == nil {
		return ir
	}
	ir.Title = spec.Meta.Title
	ir.Subtitle = spec.Meta.Subtitle
	ir.Archetype = spec.Meta.Archetype
	ir.Template = spec.Meta.Template
	ir.Audience = spec.Meta.Audience
	ir.Author = spec.Meta.Author
	ir.Date = spec.Meta.Date

	defaults := DefaultsFor(spec.Meta.Archetype)
	ir.ArchetypeTemplate = defaults.Template
	ir.Executive = defaults.Executive

	ir.Slides = make([]SlideIR, 0, len(spec.Slides))
	for i := range spec.Slides {
		ir.Slides = append(ir.Slides, normalizeSlide(i, spec.Slides[i]))
	}
	ir.Rhythm = computeRhythm(ir.Slides)
	return ir
}

// normalizeSlide plans a single slide from its semantic spec.
func normalizeSlide(index int, slide SlideSpec) SlideIR {
	plan, ok := kindPlanRegistry[slide.Kind]
	if !ok {
		plan = passthroughPlan
	}

	pattern := ""
	if plan.pattern != nil {
		pattern = plan.pattern(slide.Body)
	}

	return SlideIR{
		SourceIndex: index,
		Kind:        slide.Kind,
		Title:       slide.String("title"),
		Takeaway:    slideTakeaway(slide),
		Role:        plan.role,
		Visual: VisualPlan{
			Family:  plan.family,
			Density: plan.density,
			Pattern: pattern,
			Layout:  plan.layout,
		},
		Body: slide.Body,
	}
}

// slideTakeaway extracts the slide's one-line takeaway, falling back to the
// chart insight (chart_insight slides carry their takeaway as "insight").
func slideTakeaway(slide SlideSpec) string {
	if t := slide.String("takeaway"); t != "" {
		return t
	}
	return slide.String("insight")
}

// newRhythmPlan returns an empty, initialized RhythmPlan.
func newRhythmPlan() RhythmPlan {
	return RhythmPlan{
		Families:  map[VisualFamily]int{},
		Densities: map[Density]int{},
	}
}

// computeRhythm tallies visual families and densities across the planned slides.
func computeRhythm(slides []SlideIR) RhythmPlan {
	r := newRhythmPlan()
	r.SlideCount = len(slides)
	for _, s := range slides {
		r.Families[s.Visual.Family]++
		r.Densities[s.Visual.Density]++
	}
	return r
}
