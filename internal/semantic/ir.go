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

import "github.com/sebahrens/json2pptx/internal/semantic/slides"

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
	// Alternatives is a small deterministic set of supported compositions with
	// capacity reasons. It lets callers choose deliberately without invoking an
	// unconstrained layout optimizer.
	Alternatives []CompositionCandidate `json:"alternatives,omitempty"`
}

type CompositionCandidate struct {
	Pattern string `json:"pattern,omitempty"`
	Layout  string `json:"layout"`
	Reason  string `json:"reason"`
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
	// Chrome, ViewingMode and AccentStrategy are deck-level passthroughs to the
	// raw model (go-slide-creator-zmjs).
	Chrome         *ChromeSpec `json:"chrome,omitempty"`
	ViewingMode    string      `json:"viewing_mode,omitempty"`
	DesignMode     string      `json:"design_mode,omitempty"`
	AccentStrategy string      `json:"accent_strategy,omitempty"`
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
		role: RoleSummary, family: FamilyText, density: DensityMedium, layout: "blank-title",
		pattern: execSummaryPattern,
	},
	KindKPISnapshot: {
		role: RoleEvidence, family: FamilyKPI, density: DensityMedium, layout: "blank-title",
		pattern: kpiPattern,
	},
	KindChartInsight: {
		role: RoleEvidence, family: FamilyChart, density: DensityMedium, layout: "blank-title",
		pattern: chartInsightPattern,
	},
	KindComparison: {
		role: RoleAnalysis, family: FamilyComparison, density: DensityMedium, layout: "blank-title",
		pattern: comparisonPattern,
	},
	KindOptionMatrix: {
		role: RoleRecommendation, family: FamilyComparison, density: DensityHeavy, layout: "blank-title",
		pattern: optionMatrixPattern,
	},
	KindTable: {
		// A native table is a content slide, not a pattern: the renderer builds
		// it from the template's own table style, so the plan advertises the
		// layout only (explain/compile parity).
		role: RoleEvidence, family: FamilyText, density: DensityHeavy, layout: "content",
	},
	KindArchitecture: {
		role: RoleAnalysis, family: FamilyProcess, density: DensityMedium, layout: "blank-title",
		pattern: architecturePattern,
	},
	KindTeam: {
		role: RoleEvidence, family: FamilyText, density: DensityMedium, layout: "blank-title",
		pattern: teamPattern,
	},
	KindAgenda: {
		// An agenda is navigation, not argument — the same structural family as
		// a section divider, and it transitions the deck rather than carrying
		// evidence.
		role: RoleTransition, family: FamilyStructural, density: DensityLight, layout: "blank-title",
		pattern: agendaPattern,
	},
	KindQuote: {
		role: RoleEvidence, family: FamilyText, density: DensityLight, layout: "blank-title",
		pattern: quotePattern,
	},
	KindBridge: {
		role: RoleEvidence, family: FamilyChart, density: DensityMedium, layout: "blank-title",
		pattern: slides.BridgePattern,
	},
	KindPillars: {
		role: RoleAnalysis, family: FamilyComparison, density: DensityMedium, layout: "blank-title",
		pattern: slides.PillarsPattern,
	},
	KindStat: {
		// One number is the lightest slide in the deck and it is there to make a
		// point, not to lay out evidence in parts. It shares the big-number
		// family with kpi_snapshot, so monotony detection sees a stat beside a
		// KPI grid for what it is: the same treatment twice.
		role: RoleEvidence, family: FamilyKPI, density: DensityLight, layout: "blank-title",
		pattern: statPattern,
	},
	KindTimeline: {
		// Dated milestones are the timeline family, the same as a roadmap — so a
		// timeline next to a roadmap reads as two of the same slide, which it is.
		role: RolePlan, family: FamilyTimeline, density: DensityMedium, layout: "blank-title",
		pattern: timelinePattern,
	},
	KindMatrix2x2: {
		// A 2x2 sorts options against two axes: the comparison family, the same
		// as a scored option matrix.
		role: RoleAnalysis, family: FamilyComparison, density: DensityMedium, layout: "blank-title",
		pattern: matrixPattern,
	},
	KindFramework: {
		// A framework lays its parts out side by side to be read against each
		// other, which is the comparison family, and it is always dense.
		role: RoleAnalysis, family: FamilyComparison, density: DensityHeavy, layout: "blank-title",
		pattern: frameworkPattern,
	},
	KindImageCase: {
		// A case study is evidence told as a story, and the picture makes it the
		// lightest slide in a run of analysis.
		role: RoleEvidence, family: FamilyText, density: DensityMedium, layout: "blank-title",
		pattern: imageCasePattern,
	},
	KindProcess: {
		role: RoleAnalysis, family: FamilyProcess, density: DensityMedium, layout: "blank-title",
		pattern: processPattern,
	},
	KindRoadmap: {
		role: RolePlan, family: FamilyTimeline, density: DensityMedium, layout: "blank-title",
		pattern: func(map[string]any) string { return "phase-roadmap" },
	},
	KindDecision: {
		// The ask is the slide a board deck exists for, so its options take a
		// numbered visual when they fit one; outside that the plan advertises a
		// layout only, which is the content slide compile emits
		// (go-slide-creator-4ndv).
		role: RoleRecommendation, family: FamilyText, density: DensityMedium, layout: "blank-title",
		pattern: decisionPattern,
	},
	KindClosing:      {role: RoleClosing, family: FamilyStructural, density: DensityLight, layout: "title"},
	KindRawJSON2pptx: passthroughPlan,
}

// passthroughPlan is the fallback for unknown or raw escape-hatch slides.
var passthroughPlan = kindPlan{role: RolePassthrough, family: FamilyRaw, density: DensityMedium, layout: ""}

// comparisonPattern advertises the visual the payload will actually compile to
// — comparison-2col for a balanced pair, stylish-panels or card-grid for 3–5
// columns — and "" when it degrades to a content slide, so the explain
// projection never over-promises a visual the renderer will not emit
// (explain/compile parity for degraded/over-cap inputs).
func comparisonPattern(body map[string]any) string {
	return slides.ComparisonPattern(body)
}

// execSummaryPattern advertises exec-summary only when the payload will actually
// compile to it (3–5 points within the pattern's char budgets); otherwise it
// returns "" so the explain projection matches compile's bullet fallback rather
// than promising a visual the renderer will not emit (go-slide-creator-ku6t).
func execSummaryPattern(body map[string]any) string {
	if slides.ExecSummaryPatternFeasible(body) {
		return "exec-summary"
	}
	return ""
}

// architecturePattern advertises arch-stack only when the payload will actually
// compile to it (3–6 tiers inside the pattern's text budgets); otherwise it
// returns "" so the explain projection matches compile's bullet fallback
// (go-slide-creator-162os).
func architecturePattern(body map[string]any) string {
	if slides.ArchitecturePatternFeasible(body) {
		return "arch-stack"
	}
	return ""
}

// optionMatrixPattern advertises table-highlight only when the payload will
// actually compile to it (a 2–6 × 2–6 matrix whose scores read on the chosen
// scale); otherwise it returns "" so the explain projection matches compile's
// scored-bullet fallback (go-slide-creator-6o1r).
func optionMatrixPattern(body map[string]any) string {
	if slides.OptionMatrixPatternFeasible(body) {
		return "table-highlight"
	}
	return ""
}

// chartInsightPattern advertises chart-insights-split only when the payload will
// actually compile to it (1–6 usable insight bullets); otherwise it returns ""
// so the explain projection matches compile's content fallback rather than
// over-promising a chart+insights visual the renderer will not emit
// (explain/compile parity for over-cap inputs).
func chartInsightPattern(body map[string]any) string {
	if slides.ChartInsightPatternFeasible(body) {
		return "chart-insights-split"
	}
	return ""
}

// teamPattern advertises team-bios only when the roster will compile to it
// (go-slide-creator-13lj).
func teamPattern(body map[string]any) string {
	return slides.TeamPattern(body)
}

// statPattern advertises stat-hero only when the number and its words fit the
// pattern's budgets (go-slide-creator-2hkc).
func statPattern(body map[string]any) string {
	return slides.StatPattern(body)
}

// timelinePattern advertises timeline-horizontal only when the milestones will
// actually compile to it (go-slide-creator-wrsb).
func timelinePattern(body map[string]any) string {
	return slides.TimelinePattern(body)
}

// matrixPattern advertises matrix-2x2 only when the payload will actually
// compile to it (go-slide-creator-ykjh).
func matrixPattern(body map[string]any) string {
	return slides.MatrixPattern(body)
}

// frameworkPattern advertises bmc-canvas for a complete Business Model Canvas.
// SWOT and the five forces are native diagrams rather than patterns, so they
// advertise a layout only — which is what compile emits (go-slide-creator-anzx).
func frameworkPattern(body map[string]any) string {
	return slides.FrameworkPattern(body)
}

// imageCasePattern advertises image-text-split only when the payload will
// actually compile to it (go-slide-creator-q31s).
func imageCasePattern(body map[string]any) string {
	return slides.ImageCasePattern(body)
}

// decisionPattern advertises the numbered visual the options will actually
// compile to, or "" for the content slide (go-slide-creator-4ndv).
func decisionPattern(body map[string]any) string {
	return slides.DecisionPattern(body)
}

// processPattern advertises the visual the steps will actually compile to: the
// numbered strip when they carry descriptions, the flow diagram when they
// branch or are bare labels (go-slide-creator-61up).
func processPattern(body map[string]any) string {
	return slides.ProcessPattern(body)
}

// agendaPattern advertises the agenda pattern the payload will actually
// compile to — the numbered list, the subtitled rows, or none when it degrades
// to bullets (go-slide-creator-3rvk).
func agendaPattern(body map[string]any) string {
	return slides.AgendaPattern(body)
}

func quotePattern(body map[string]any) string {
	if slides.QuoteOverBudget(body) != "" {
		return ""
	}
	switch slides.UsableQuoteCount(body) {
	case 1:
		return "pull-quote"
	case 3, 4, 5, 6, 7, 8:
		return "quote-cluster"
	default:
		return ""
	}
}

// kpiPattern advertises the kpi-Nup pattern the payload will actually compile
// to. It used to report kpi-Nup from the declared count alone, so a metric that
// overflowed the compact cards left the plan promising a KPI visual while the
// compiler emitted bullets — and nothing but the pixels said which was true
// (go-slide-creator-5ok4).
func kpiPattern(body map[string]any) string {
	return slides.KPIPattern(body)
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
	ir.Chrome = spec.Meta.Chrome
	ir.ViewingMode = spec.Meta.ViewingMode
	ir.DesignMode = spec.Meta.DesignMode
	ir.AccentStrategy = spec.Meta.AccentStrategy

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
	// An over-cap (or insight-less) chart_insight compiles to the native
	// fallback slide rather than the blank-title pattern canvas; plan that
	// layout so explain reports the slide_type compile actually emits.
	if slide.Kind == KindChartInsight && pattern == "" {
		plan.layout = slides.ChartInsightFallbackSlideType(slide.Body)
	}
	if slide.Kind == KindPillars && pattern == "" {
		plan.layout = "content"
	}
	alternatives := compositionCandidates(slide.Kind, pattern)
	if requested := slide.String("pattern"); requested != "" {
		for _, candidate := range alternatives {
			if candidate.Pattern == requested {
				pattern = requested
				plan.layout = candidate.Layout
				break
			}
		}
	}
	if requested := slide.String("layout"); requested != "" {
		for _, candidate := range alternatives {
			if candidate.Layout == requested {
				plan.layout = requested
				if slide.Kind == KindPillars && requested == "content" {
					pattern = ""
				}
				break
			}
		}
	}

	return SlideIR{
		SourceIndex: index,
		Kind:        slide.Kind,
		Title:       slide.String("title"),
		Takeaway:    slideTakeaway(slide),
		Role:        plan.role,
		Visual: VisualPlan{
			Family:       plan.family,
			Density:      plan.density,
			Pattern:      pattern,
			Layout:       plan.layout,
			Alternatives: alternatives,
		},
		Body: slide.Body,
	}
}

// SlideAlternatives returns the compositions a slide of this kind can be asked
// for: the pattern the planner would choose plus the alternatives it would
// accept as a `pattern` / `layout` override. Validation and list_slide_kinds
// read it so the allowed set is the same one normalizeSlide matches against
// (go-slide-creator-u5az).
func SlideAlternatives(kind SlideKind, body map[string]any) []CompositionCandidate {
	plan, ok := kindPlanRegistry[kind]
	if !ok {
		plan = passthroughPlan
	}
	pattern := ""
	if plan.pattern != nil {
		pattern = plan.pattern(body)
	}
	return compositionCandidates(kind, pattern)
}

// compositionCandidates lists the compositions a kind's optional pattern /
// layout override accepts. validateCompositionOverride reads this list to
// decide whether an override is honourable, so everything in it must be
// something THIS kind compiles to: a cross-kind suggestion here validates clean
// and then silently does nothing, which is the bug the override reporting
// exists to prevent. Those suggestions belong in the kind's summary
// (go-slide-creator-4fr1).
func compositionCandidates(kind SlideKind, selected string) []CompositionCandidate {
	visual := func(pattern, reason string) CompositionCandidate {
		return CompositionCandidate{Pattern: pattern, Layout: "blank-title", Reason: reason}
	}
	switch kind {
	case KindExecutiveSummary:
		return []CompositionCandidate{
			visual("exec-summary", "3-5 lead-in statements, each with one supporting sentence"),
			{Layout: "content", Reason: "native bullets preserve more or fewer points, or longer ones"},
		}
	case KindDecision:
		return []CompositionCandidate{
			visual("numbered-step-strip", "3-6 options as numbered boxes, the recommendation in the callout band"),
			visual("card-grid", "exactly 2 options, each with a detail, as two cards side by side"),
			{Layout: "content", Reason: "the recommendation as a lead-in, then the options as bullets"},
		}
	case KindTitle, KindSection, KindClosing:
		// Structural slides take the template's own chrome; there is no visual
		// to choose, and saying so is more use than saying nothing.
		return []CompositionCandidate{{Layout: string(kind), Reason: "the template's own " + string(kind) + " layout"}}
	case KindKPISnapshot:
		return []CompositionCandidate{visual(selected, "compact visual for 2-6 readable metrics"), {Layout: "content", Reason: "native bullets preserve out-of-range or long metrics"}}
	case KindChartInsight:
		return []CompositionCandidate{visual("chart-insights-split", "chart with up to 6 insights"), {Layout: "two-column", Reason: "native chart and full insight list when density exceeds the pattern"}}
	case KindComparison:
		return []CompositionCandidate{
			visual("comparison-2col", "balanced two-column comparison"),
			visual("stylish-panels", "3-5 columns, each a titled panel with its own bullets"),
			visual("card-grid", "2-5 columns as titled cards when panels do not fit"),
			{Layout: "content", Reason: "native bullets preserve six or more columns"},
		}
	case KindOptionMatrix:
		return []CompositionCandidate{
			visual("table-highlight", "2-6 options scored against 2-6 criteria"),
			{Layout: "content", Reason: "native bullets preserve a matrix outside those bounds"},
		}
	case KindTable:
		return []CompositionCandidate{
			{Layout: "content", Reason: "a native table in the template's own table style"},
		}
	case KindArchitecture:
		return []CompositionCandidate{
			visual("arch-stack", "3-6 tiers with up to 3 cross-cutting rails"),
			{Layout: "content", Reason: "native bullets preserve a stack outside the pattern's tier count or text budgets"},
		}
	case KindTeam:
		return []CompositionCandidate{
			visual("team-bios", "1-8 people as cards with a photo badge, role and short bio"),
			{Layout: "content", Reason: "native bullets preserve a larger roster or longer bios"},
		}
	case KindAgenda:
		return []CompositionCandidate{
			visual("agenda", "2-10 sections as a numbered list"),
			visual("agenda-with-images", "3-6 sections, each with a subtitle, as rows"),
			{Layout: "content", Reason: "native numbered bullets preserve an agenda outside those counts"},
		}
	case KindQuote:
		return []CompositionCandidate{
			visual("pull-quote", "one attributed quotation"),
			visual("quote-cluster", "3–8 attributed stakeholder quotations"),
			{Layout: "content", Reason: "bullets preserve two quotes or copy outside the patterns' bounds"},
		}
	case KindBridge:
		return []CompositionCandidate{
			visual("waterfall-bridge", "3–10 ordered additive components with totals and deltas"),
			{Layout: "content", Reason: "bullets preserve a walk outside the visual's count or text budgets"},
		}
	case KindPillars:
		candidates := []CompositionCandidate{}
		if selected != "" {
			candidates = append(candidates, visual(selected, "3–5 pillars within the selected visual's text budgets"))
		}
		return append(candidates, CompositionCandidate{Layout: "content", Reason: "bullets preserve an over-budget or partly framed strategy"})
	case KindStat:
		return []CompositionCandidate{
			visual("stat-hero", "one oversized number with a label, context and source"),
			{Layout: "content", Reason: "a content slide preserves a number or label past the hero's text budgets"},
		}
	case KindTimeline:
		return []CompositionCandidate{
			visual("timeline-horizontal", "3-7 dated milestones on one line"),
			{Layout: "content", Reason: "a dated bullet list preserves more or fewer milestones"},
		}
	case KindMatrix2x2:
		return []CompositionCandidate{
			visual("matrix-2x2", "four quadrants against two named axes"),
			{Layout: "content", Reason: "native bullets preserve a half-filled matrix or unnamed axes"},
		}
	case KindFramework:
		return []CompositionCandidate{
			visual("bmc-canvas", "the 9-cell Business Model Canvas (framework: bmc)"),
			{Layout: "blank-title", Reason: "a native swot or five-forces diagram (framework: swot / porters_five_forces)"},
			{Layout: "content", Reason: "grouped bullets preserve a framework missing one of its parts"},
		}
	case KindImageCase:
		return []CompositionCandidate{
			visual("image-text-split", "a picture beside the story, with up to 3 result metrics"),
			{Layout: "content", Reason: "a content slide preserves a longer story than the column holds"},
		}
	case KindProcess:
		return []CompositionCandidate{
			visual("numbered-step-strip", "3-6 steps that each carry a description, as numbered rows"),
			visual("process-flow", "3-8 bare or branching steps as a flow diagram"),
			{Layout: "content", Reason: "native bullets preserve shorter or longer processes"},
		}
	case KindRoadmap:
		return []CompositionCandidate{visual("phase-roadmap", "3-6 phases"), {Layout: "content", Reason: "native bullets preserve dense roadmaps"}}
	default:
		return nil
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
