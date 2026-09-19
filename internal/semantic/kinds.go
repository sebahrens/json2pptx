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
	// KindProcess describes a sequential process or flow.
	KindProcess SlideKind = "process"
	// KindRoadmap describes a phased roadmap or timeline.
	KindRoadmap SlideKind = "roadmap"
	// KindDecision frames a decision and its recommendation.
	KindDecision SlideKind = "decision"
	// KindClosing is the closing slide.
	KindClosing SlideKind = "closing"
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
		Summary:        "Section divider introducing the next group of slides.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"subtitle"},
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
		Summary:        "Side-by-side comparison of two or more options.",
		RequiredFields: []string{"columns"},
		TypicalFields:  []string{"title", "takeaway"},
	},
	KindOptionMatrix: {
		Kind:            KindOptionMatrix,
		Summary:         "Options scored against shared criteria — the evaluation matrix a recommendation is argued on. 2–6 criteria × 2–6 options render as the table-highlight pattern with the recommended row and decisive column highlighted; outside that it degrades to a scored bullet list.",
		RequiredFields:  []string{"criteria", "options"},
		RequiredAliases: map[string][]string{"criteria": {"columns"}, "options": {"rows"}},
		TypicalFields:   []string{"title", "scale", "recommended", "decisive_criterion", "highlight_label", "takeaway"},
	},
	KindTable: {
		Kind:            KindTable,
		Summary:         "A native data table — the financials, the segment split, the pricing tiers. Renders with the template's own table style; up to 6 columns × 7 logical rows before the density rules ask for a split.",
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
	KindProcess: {
		Kind:           KindProcess,
		Summary:        "Sequential process or flow with ordered steps.",
		RequiredFields: []string{"steps"},
		TypicalFields:  []string{"title", "takeaway"},
	},
	KindRoadmap: {
		Kind:           KindRoadmap,
		Summary:        "Phased roadmap or timeline.",
		RequiredFields: []string{"phases"},
		TypicalFields:  []string{"title", "takeaway"},
	},
	KindDecision: {
		Kind:           KindDecision,
		Summary:        "Frames a decision, options, and a recommendation.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"options", "recommendation", "takeaway"},
	},
	KindClosing: {
		Kind:           KindClosing,
		Summary:        "Closing slide (thank-you, questions, or call to action).",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"subtitle"},
	},
	KindRawJSON2pptx: {
		Kind:           KindRawJSON2pptx,
		Summary:        "Escape hatch carrying a raw json2pptx slide payload verbatim.",
		RequiredFields: []string{"slide"},
	},
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
