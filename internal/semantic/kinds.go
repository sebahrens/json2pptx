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
		Summary:        "High-level summary of the deck's key messages.",
		RequiredFields: []string{"title"},
		TypicalFields:  []string{"points", "takeaways", "takeaway"},
	},
	KindKPISnapshot: {
		Kind:           KindKPISnapshot,
		Summary:        "Snapshot of big-number key performance indicators.",
		RequiredFields: []string{"kpis"},
		TypicalFields:  []string{"title", "takeaway"},
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
