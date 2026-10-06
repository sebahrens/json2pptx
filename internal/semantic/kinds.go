package semantic

import "sort"

// SlideKind is the discriminator that selects a slide's semantic payload shape.
type SlideKind string

// The initial slide kind vocabulary. Each value names a semantic intent; the
// kind-specific payload fields live in SlideSpec.Body and are decoded and
// validated by later compiler phases.
const (
	// KindTitle is the opening title slide.
	KindTitle SlideKind = "title"
	// KindSection is a section-divider slide.
	KindSection SlideKind = "section"
	// KindExecutiveSummary is a high-level summary slide.
	KindExecutiveSummary SlideKind = "executive_summary"
	// KindKPISnapshot is a big-number KPI slide.
	KindKPISnapshot SlideKind = "kpi_snapshot"
	// KindChartInsight pairs a chart with an interpretive insight.
	KindChartInsight SlideKind = "chart_insight"
	// KindComparison contrasts two or more options.
	KindComparison SlideKind = "comparison"
	// KindOptionMatrix scores options against shared criteria.
	KindOptionMatrix SlideKind = "option_matrix"
	// KindTable is a native data table.
	KindTable SlideKind = "table"
	// KindArchitecture is a tiered architecture / platform stack.
	KindArchitecture SlideKind = "architecture"
	// KindCycle is a loop, a hub or nested rings: phases in a circular
	// picture chosen by its style (go-slide-creator-53v5u).
	KindCycle SlideKind = "cycle"
	// KindAgenda is the deck's contents page.
	KindAgenda SlideKind = "agenda"
	// KindQuote highlights one attributed quote or a cluster of stakeholder voices.
	KindQuote SlideKind = "quote"
	// KindBridge explains an additive walk from a starting value to an ending value.
	KindBridge SlideKind = "bridge"
	// KindPillars lays out strategic pillars with optional house framing.
	KindPillars SlideKind = "pillars"
	// KindOrg shows a reporting or governance tree.
	KindOrg SlideKind = "org"
	// KindTeam is the people grid: who is on the engagement.
	KindTeam SlideKind = "team"
	// KindStat is one oversized number carrying the slide.
	KindStat SlideKind = "stat"
	// KindTimeline is dated milestones on a line.
	KindTimeline SlideKind = "timeline"
	// KindMatrix2x2 is two axes and four quadrants.
	KindMatrix2x2 SlideKind = "matrix_2x2"
	// KindRiskHeatmap places named risks on a likelihood × impact grid
	// (go-slide-creator-ec74l).
	KindRiskHeatmap SlideKind = "risk_heatmap"
	// KindFramework is a named framework with fixed parts (SWOT, Porter's five
	// forces, the Business Model Canvas).
	KindFramework SlideKind = "framework"
	// KindImageCase is a picture beside the story about it.
	KindImageCase SlideKind = "image_case"
	// KindProcess describes a sequential process or flow.
	KindProcess SlideKind = "process"
	// KindRoadmap describes a phased roadmap or timeline.
	KindRoadmap SlideKind = "roadmap"
	// KindDecision frames a decision and its recommendation.
	KindDecision SlideKind = "decision"
	// KindNextSteps is the consulting closer: actions with owner and date,
	// plus the decisions requested (go-slide-creator-7lzdh).
	KindNextSteps SlideKind = "next_steps"
	// KindClosing is the closing slide.
	KindClosing SlideKind = "closing"
	// KindRegions is one slide split into 2–3 typed regions (chart, stat,
	// kpis, table, timeline, image, text) under one title, in a bounded
	// arrangement with explicit proportions (go-slide-creator-fn2ka).
	KindRegions SlideKind = "regions"
	// KindRawJSON2pptx is an escape hatch carrying a raw json2pptx slide
	// payload verbatim, bypassing the semantic abstraction.
	KindRawJSON2pptx SlideKind = "raw_json2pptx"
)

// KindInfo describes a slide kind for discrimination, schema export, and
// documentation. RequiredFields and TypicalFields are advisory in this scaffold
// — the validation gates added in a later phase enforce them.
type KindInfo struct {
	Kind    SlideKind `json:"kind"`
	Summary string    `json:"summary"`
	// RequiredFields lists payload keys the kind needs to compile.
	RequiredFields []string `json:"required_fields,omitempty"`
	// TypicalFields lists common optional payload keys, for authoring hints.
	TypicalFields []string `json:"typical_fields,omitempty"`
	// RequiredAliases maps a canonical required field (one listed in
	// RequiredFields) to alternative payload keys the compiler accepts in its
	// place. The requirement is satisfied when the canonical field OR any of its
	// aliases carries usable content (required-one-of semantics). This keeps the
	// validator, schema, and discovery from requiring a key the compiler treats
	// as interchangeable — e.g. kpi_snapshot reads "metrics" as an alias for
	// "kpis", so a spec using only "metrics" must validate and compile, not be
	// blocked before compile.
	RequiredAliases map[string][]string `json:"required_aliases,omitempty"`
}

// slideKindRegistry is the canonical source of known slide kinds.
var slideKindRegistry = map[SlideKind]KindInfo{
	KindTitle: {
		Kind:           KindTitle,
		Summary:        "Opening title slide with a headline and optional subtitle.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"subtitle", "eyebrow"},
	},
	KindSection: {
		Kind:           KindSection,
		Summary:        "Section divider introducing the next group of slides. Numbered 01, 02, … automatically; appendix: true (or a title such as Appendix, Backup, Q&A) leaves it unnumbered and uncounted.",
		TypicalFields:  []string{"appendix"},
		RequiredFields: []string{"title"},
	},
	KindExecutiveSummary: {
		Kind:           KindExecutiveSummary,
		Summary:        "High-level summary of the deck's key messages. 3–5 points render as the exec-summary pattern — numbered bold conclusions, each with its supporting sentence, over an optional bottom-line bar; any other count degrades to a bullet list.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"points", "takeaways", "bottom_line", "takeaway"},
	},
	KindKPISnapshot: {
		Kind:            KindKPISnapshot,
		Summary:         "Snapshot of big-number key performance indicators.",
		RequiredFields:  []string{"kpis"},
		RequiredAliases: map[string][]string{"kpis": {"metrics"}},
		TypicalFields:   []string{"title", "takeaway"},
	},
	KindChartInsight: {
		Kind:           KindChartInsight,
		Summary:        "A chart paired with an interpretive insight.",
		RequiredFields: []string{"chart"},
		TypicalFields:  []string{"title", "insights", "source", "takeaway"},
	},
	KindComparison: {
		Kind:           KindComparison,
		Summary:        "Side-by-side comparison of two or more options. Two balanced columns render row-aligned (comparison-2col) and take connectors: true (per-row today → target badge), highlight_column or highlight_row. 3–5 columns are panels, 6–12 cards.",
		RequiredFields: []string{"columns"},
		TypicalFields:  []string{"title", "connectors", "highlight_column", "highlight_row", "takeaway"},
	},
	KindOptionMatrix: {
		Kind:            KindOptionMatrix,
		Summary:         "Options scored against shared criteria — the evaluation matrix a recommendation is argued on. 2–6 criteria × 2–6 options render as the table-highlight pattern with the recommended row(s) and decisive column highlighted; outside that it degrades to a scored bullet list.",
		RequiredFields:  []string{"criteria", "options"},
		RequiredAliases: map[string][]string{"criteria": {"columns"}, "options": {"rows"}},
		TypicalFields:   []string{"title", "scale", "recommended", "decisive_criterion", "highlight_label", "takeaway"},
	},
	KindTable: {
		Kind:            KindTable,
		Summary:         "A native data table — the financials / P&L, the segment split, the price list, a risk register (risk, likelihood, impact, mitigation, owner). Renders with the template's own table style; up to 6 columns × 10 logical rows before the density rules ask for a split.",
		RequiredFields:  []string{"headers", "rows"},
		RequiredAliases: map[string][]string{"headers": {"columns"}},
		TypicalFields:   []string{"title", "column_alignments", "highlight_column", "totals_row", "takeaway"},
	},
	KindArchitecture: {
		Kind:            KindArchitecture,
		Summary:         "A tiered architecture or platform stack — the layers of a system, top to bottom, with optional cross-cutting rails beside them. 3–6 tiers render as the arch-stack visual; outside that, or past its text budgets, it degrades to a bullet list.",
		RequiredFields:  []string{"tiers"},
		RequiredAliases: map[string][]string{"tiers": {"layers"}},
		TypicalFields:   []string{"title", "rails", "takeaway"},
	},
	KindCycle: {
		Kind:            KindCycle,
		Summary:         "A loop, a hub or nested rings: the phases in order and a style. One ordered loop is ring (default; 4–8 segments) or nodes (3–8 circles joined by arrows); 1–3 one-off intake steps feeding a loop of 3–8 are intake; two coupled loops on a full-width slide are figure_eight (4–8); 4–8 peers around a center are radial; 3–5 things that contain one another are concentric (innermost first). highlight marks the one accent phase. Outside the style's count or text budgets it degrades to a numbered list. A sequence that does not loop back is process.",
		RequiredFields:  []string{"phases"},
		RequiredAliases: map[string][]string{"phases": {"steps", "items"}},
		TypicalFields:   []string{"title", "style", "intake", "center", "highlight", "left_label", "right_label", "takeaway"},
	},
	KindAgenda: {
		Kind:            KindAgenda,
		Summary:         "The deck's contents page: the sections it covers, in order, optionally marking the one the deck is at. Renders as the numbered agenda list (2–10 sections), each subtitle as a muted line under its title and the current section bold with the rest dimmed; a subtitle over 120 characters uses agenda-with-images rows (3–6). Outside those bounds it degrades to a numbered bullet list.",
		RequiredFields:  []string{"sections"},
		RequiredAliases: map[string][]string{"sections": {"items", "agenda"}},
		TypicalFields:   []string{"title", "current", "takeaway"},
	},
	KindQuote: {
		Kind:            KindQuote,
		Summary:         "One attributed quote renders as pull-quote; 3–8 render as quote-cluster. Two quotes, missing attributions, and over-budget copy degrade to quote bullets with a finding.",
		RequiredFields:  []string{"quotes"},
		RequiredAliases: map[string][]string{"quotes": {"testimonials", "voices", "quote", "text"}},
		TypicalFields:   []string{"title", "takeaway", "attribution", "role"},
	},
	KindBridge: {
		Kind:           KindBridge,
		Summary:        "An additive P&L or cost walk: 3–10 ordered total, delta, and subtotal columns render as waterfall-bridge. Outside the visual's count or text budgets, every component becomes a bullet with a degradation finding.",
		RequiredFields: []string{"columns"},
		TypicalFields:  []string{"title", "unit", "caption", "takeaway"},
	},
	KindPillars: {
		Kind:           KindPillars,
		Summary:        "Three to five named pillars render as stylish-panels, or as strategy-house (gabled roof, pillars, foundation levels) when both objective and foundation are supplied. The pillar count and the foundation levels follow the content: one pillar per theme, one level per kind of enabler. Over-budget or incomplete framing degrades to complete bullets with a finding.",
		RequiredFields: []string{"pillars"},
		TypicalFields:  []string{"title", "objective", "foundation", "beam", "roof_badges", "takeaway"},
	},
	KindOrg: {
		Kind:           KindOrg,
		Summary:        "A reporting or governance tree. Up to seven people or bodies across three levels render as org_chart; larger trees degrade to an indented list so no named node is hidden.",
		RequiredFields: []string{"nodes"},
		TypicalFields:  []string{"title", "takeaway"},
	},
	KindTeam: {
		Kind:            KindTeam,
		Summary:         "The people on the engagement: 1–8 cards with a name, a role and an optional short bio. Past eight cards, or past a card's text budgets, it degrades to a bullet list.",
		RequiredFields:  []string{"members"},
		RequiredAliases: map[string][]string{"members": {"people", "team"}},
		TypicalFields:   []string{"title", "takeaway"},
	},
	KindStat: {
		Kind:            KindStat,
		Summary:         "One number, made the whole slide: the value, the words beneath it, and optionally a line of context and a source. Renders as the stat-hero visual; past its text budgets it degrades to a content slide. For several numbers at equal weight use kpi_snapshot.",
		RequiredFields:  []string{"value"},
		RequiredAliases: map[string][]string{"value": {"stat", "number", "metric"}},
		TypicalFields:   []string{"title", "label", "unit", "context", "source", "takeaway"},
	},
	KindTimeline: {
		Kind:            KindTimeline,
		Summary:         "Dated milestones on a line: 3–7 stops, each a label with an optional date and a line of detail. A stop that spans a period (an end date) turns the line into range bars. Outside those counts, or past a stop's text budgets, it degrades to a dated bullet list. For parallel workstreams across phases use roadmap.",
		RequiredFields:  []string{"milestones"},
		RequiredAliases: map[string][]string{"milestones": {"stops", "events", "timeline"}},
		TypicalFields:   []string{"title", "takeaway"},
	},
	KindMatrix2x2: {
		Kind:           KindMatrix2x2,
		Summary:        "Two axes and four quadrants — impact against effort, reach against cost. For named risks by likelihood and impact use risk_heatmap. Needs both axis labels and all four quadrants, each with a header; short of that, or past a quadrant's text budgets, it degrades to a bullet list naming each quadrant's position.",
		RequiredFields: []string{"quadrants", "x_axis", "y_axis"},
		RequiredAliases: map[string][]string{
			"quadrants": {"cells", "boxes", "top_left"},
			"x_axis":    {"x_axis_label"},
			"y_axis":    {"y_axis_label"},
		},
		TypicalFields: []string{"title", "x_low", "x_high", "y_low", "y_high", "takeaway"},
	},
	KindRiskHeatmap: {
		Kind:           KindRiskHeatmap,
		Summary:        "The risk heat map: 1–20 named risks placed by likelihood and impact on a 3 × 3 grid (size: 5 for 5 × 5), every cell coloured by its likelihood × impact band; a \"medium\" has a cell of its own and risks sharing a cell stack. A level off the grid degrades to bullets. For mitigations and owners use table.",
		RequiredFields: []string{"items"},
		TypicalFields:  []string{"title", "size", "takeaway"},
	},
	KindFramework: {
		Kind:            KindFramework,
		Summary:         "A named framework with fixed parts: swot (4 quadrants), porters_five_forces (5 forces) or bmc (the 9-cell Business Model Canvas). Give every part or it degrades to grouped bullets — the visual draws all of them or none.",
		RequiredFields:  []string{"framework", "sections"},
		RequiredAliases: map[string][]string{"framework": {"type", "model"}},
		TypicalFields:   []string{"title", "takeaway"},
	},
	KindImageCase: {
		Kind:            KindImageCase,
		Summary:         "A photo or screenshot beside the words about it — the case study or customer story slide; callouts point at parts of it. Needs a body or a bullet; the bundled example omits its image and is a draft: SEMANTIC_IMAGE_MISSING blocks readiness until a picture is supplied. Up to 5 bullets, 3 result metrics and 6 callouts; past its text budgets it degrades to a content slide. For testimony use quote.",
		RequiredFields:  []string{"body"},
		RequiredAliases: map[string][]string{"body": {"text", "story", "description", "bullets"}},
		TypicalFields:   []string{"title", "image", "callouts", "eyebrow", "heading", "bullets", "metrics", "caption", "image_side", "image_width_pct", "image_label", "takeaway"},
	},
	KindProcess: {
		Kind:           KindProcess,
		Summary:        "Sequential process or flow with ordered steps. Steps that carry a description render as numbered rows (3–6), each a bold label over its own detail line; bare labels and branching steps (type: decision) render as the flow diagram (3–8).",
		RequiredFields: []string{"steps"},
		TypicalFields:  []string{"title", "takeaway"},
	},
	KindRoadmap: {
		Kind:           KindRoadmap,
		Summary:        "Phased roadmap: 3–6 named phases on one timeline bar (date range, description or items, optional milestone) plus 0–4 parallel_tracks drawn as bars under them. Outside those counts or budgets it degrades to bullets. Dated stops without descriptions: timeline.",
		RequiredFields: []string{"phases"},
		TypicalFields:  []string{"title", "parallel_tracks", "parallel_label", "takeaway"},
	},
	KindDecision: {
		Kind:            KindDecision,
		Summary:         "The ask: the options considered and the one (or ones) recommended. 3–6 options render as numbered boxes and exactly 2 or 7–12 (each with a detail) as cards, with the recommendation in the band beneath them; outside that it is the recommendation as a lead-in over option bullets.",
		RequiredFields:  []string{"title"},
		RequiredAliases: map[string][]string{"options": {"choices", "alternatives"}},
		TypicalFields:   []string{"options", "recommendation", "takeaway"},
	},
	KindNextSteps: {
		Kind:            KindNextSteps,
		Summary:         "The closer a consulting deck ends on: 2–6 actions, each with an owner and a date, and 0–3 decisions requested. Renders as the next-steps pattern — numbered action rows separated by rules over a 'Decisions requested' band; outside those bounds it degrades to bullets. Prefer it to a plain closing ('Thank you').",
		RequiredFields:  []string{"actions"},
		RequiredAliases: map[string][]string{"actions": {"next_steps", "steps"}},
		TypicalFields:   []string{"title", "decisions", "decisions_label"},
	},
	KindClosing: {
		Kind:           KindClosing,
		Summary:        "Plain closing slide on the template's closing layout (a short display title and a subtitle), or a content slide with a few bullets. Consulting decks should close on next_steps; keep this for a Q&A or contact page.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"subtitle"},
	},
	KindRegions: {
		Kind: KindRegions,
		Summary: "One slide, one title, 2–3 typed regions: the chart on the left with the KPI and the timeline stacked on the right. " +
			"arrangement is columns or rows (2–3 regions; size_pct is each one's width / height) or main_left / main_right / main_top / main_bottom " +
			"(exactly 3: regions[0] is the main region and its size_pct its share, default 60; regions[1..2] stack beside it and split that side by their own size_pct). " +
			"Each region is {kind, size_pct?, heading?, source?, …} with kind chart (chart, unit), stat (value, label, unit, context), kpis (2–4 kpis), " +
			"table (headers, rows; ≤4 columns × 5 rows), timeline (3–7 milestones), image (image, caption), cycle (phases, style) or text (body, bullets) — the fields of the like-named slide kind. " +
			"Region sources join the slide's source line. Use it only when the content types must be read together; one kind per slide stays the default.",
		RequiredFields: []string{"regions"},
		TypicalFields:  []string{"title", "arrangement", "takeaway", "source"},
	},
	KindRawJSON2pptx: {
		Kind:           KindRawJSON2pptx,
		Summary:        "Escape hatch carrying a raw json2pptx slide payload verbatim.",
		RequiredFields: []string{"slide"},
	},
}

// kindSpelling maps a spelling an author is likely to reach for onto the kind
// that carries it. Nothing resolves through this map — an unregistered kind is
// still a hard error — but the error names the one kind meant rather than
// leaving the author to pick it out of the full list.
var kindSpelling = map[SlideKind]SlideKind{
	"metric_hero": KindStat,
	"big_number":  KindStat,
}

// SpellingFor returns the registered kind a near-miss spelling means, and
// whether there is one.
func SpellingFor(k SlideKind) (SlideKind, bool) {
	canonical, ok := kindSpelling[k]
	return canonical, ok
}

// LookupKind returns the KindInfo for a slide kind and whether it is registered.
func LookupKind(k SlideKind) (KindInfo, bool) {
	info, ok := slideKindRegistry[k]
	return info, ok
}

// Valid reports whether the slide kind is registered.
func (k SlideKind) Valid() bool {
	_, ok := slideKindRegistry[k]
	return ok
}

// AllSlideKinds returns every registered slide kind in stable (sorted) order.
func AllSlideKinds() []SlideKind {
	out := make([]SlideKind, 0, len(slideKindRegistry))
	for k := range slideKindRegistry {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
