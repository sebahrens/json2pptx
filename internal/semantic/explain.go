package semantic

// This file is the explain scaffold: it renders the compiler's planned
// decisions for a deck as a structured, transport-neutral value, available
// before any raw PresentationInput or rendered PPTX exists. A later phase wires
// this behind a `json2pptx semantic explain` command and the MCP surface; here
// it operates purely on the DeckIR so the planning logic is testable on its
// own.

// SlideExplanation is the per-slide account of the compiler's planned
// decisions: the semantic kind, narrative role, visual family, density, and the
// concrete pattern/layout the slide will compile into.
type SlideExplanation struct {
	Index        int           `json:"index"`
	Kind         SlideKind     `json:"kind"`
	Role         NarrativeRole `json:"role"`
	VisualFamily VisualFamily  `json:"visual_family"`
	Density      Density       `json:"density"`
	Title        string        `json:"title,omitempty"`
	Takeaway     string        `json:"takeaway,omitempty"`
	Pattern      string        `json:"pattern,omitempty"`
	Layout       string        `json:"layout,omitempty"`
}

// DeckExplanation is the deck-level account: the deck title and archetype, the
// rhythm summary, and the per-slide explanations in source order.
type DeckExplanation struct {
	Title     string             `json:"title,omitempty"`
	Archetype Archetype          `json:"archetype,omitempty"`
	Template  string             `json:"template,omitempty"`
	Rhythm    RhythmPlan         `json:"rhythm"`
	Slides    []SlideExplanation `json:"slides"`
}

// Explain renders the planned decisions held in the DeckIR as a
// DeckExplanation. It performs no further planning — it is a pure projection of
// the IR — so the same decisions the compiler will act on are what the author
// sees.
func (ir *DeckIR) Explain() DeckExplanation {
	if ir == nil {
		return DeckExplanation{Rhythm: newRhythmPlan()}
	}
	out := DeckExplanation{
		Title:     ir.Title,
		Archetype: ir.Archetype,
		Template:  ir.Template,
		Rhythm:    ir.Rhythm,
		Slides:    make([]SlideExplanation, 0, len(ir.Slides)),
	}
	for _, s := range ir.Slides {
		out.Slides = append(out.Slides, SlideExplanation{
			Index:        s.SourceIndex,
			Kind:         s.Kind,
			Role:         s.Role,
			VisualFamily: s.Visual.Family,
			Density:      s.Visual.Density,
			Title:        s.Title,
			Takeaway:     s.Takeaway,
			Pattern:      s.Visual.Pattern,
			Layout:       s.Visual.Layout,
		})
	}
	return out
}

// ExplainSpec normalizes a DeckSpec and explains it in one step. It is the
// convenience entry point for callers that hold a parsed spec rather than an IR.
func ExplainSpec(spec *DeckSpec) DeckExplanation {
	return Normalize(spec).Explain()
}
