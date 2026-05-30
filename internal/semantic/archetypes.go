package semantic

import "sort"

// Archetype names the overall purpose of a deck. It biases template choice,
// default slide rhythm, and tone in later compiler phases.
type Archetype string

// The initial deck archetype vocabulary.
const (
	// ArchetypeBoardUpdate is a periodic update to a board of directors.
	ArchetypeBoardUpdate Archetype = "board_update"
	// ArchetypeQBR is a quarterly business review.
	ArchetypeQBR Archetype = "qbr"
	// ArchetypeSalesPitch is a sales or pitch deck.
	ArchetypeSalesPitch Archetype = "sales_pitch"
	// ArchetypeStrategyProposal is a strategy or initiative proposal.
	ArchetypeStrategyProposal Archetype = "strategy_proposal"
	// ArchetypeProjectRoadmap is a project plan or roadmap deck.
	ArchetypeProjectRoadmap Archetype = "project_roadmap"
	// ArchetypeMarketAnalysis is a market or competitive analysis.
	ArchetypeMarketAnalysis Archetype = "market_analysis"
)

// ArchetypeInfo describes a deck archetype for schema export and documentation.
type ArchetypeInfo struct {
	Archetype Archetype `json:"archetype"`
	Summary   string    `json:"summary"`
}

// archetypeRegistry is the canonical source of known archetypes.
var archetypeRegistry = map[Archetype]ArchetypeInfo{
	ArchetypeBoardUpdate:      {Archetype: ArchetypeBoardUpdate, Summary: "Periodic update to a board of directors."},
	ArchetypeQBR:              {Archetype: ArchetypeQBR, Summary: "Quarterly business review."},
	ArchetypeSalesPitch:       {Archetype: ArchetypeSalesPitch, Summary: "Sales or pitch deck."},
	ArchetypeStrategyProposal: {Archetype: ArchetypeStrategyProposal, Summary: "Strategy or initiative proposal."},
	ArchetypeProjectRoadmap:   {Archetype: ArchetypeProjectRoadmap, Summary: "Project plan or roadmap deck."},
	ArchetypeMarketAnalysis:   {Archetype: ArchetypeMarketAnalysis, Summary: "Market or competitive analysis."},
}

// ArchetypeDefaults are the deterministic deck-level biases an archetype
// applies during normalization when the author leaves a choice unset. They are
// pure data (no I/O, no randomness) so compile and explain reach the same
// decisions on every run.
type ArchetypeDefaults struct {
	// Template is the json2pptx template the archetype prefers. It fills the
	// emitted deck's template only when neither the spec nor the caller pins one
	// (spec template > CLI --template > archetype default).
	Template string
	// Executive marks archetypes whose decks are expected to land a message with
	// a synthesis (executive_summary) or decision slide. It drives the rhythm
	// synthesis rule (see rhythm.go).
	Executive bool
}

// archetypeDefaults is the canonical, deterministic defaults table. Template
// choices map each purpose onto a bundled template with a fitting tone; the
// Executive flag marks decks that should carry a synthesis/decision slide.
var archetypeDefaults = map[Archetype]ArchetypeDefaults{
	ArchetypeBoardUpdate:      {Template: "midnight-blue", Executive: true},
	ArchetypeQBR:              {Template: "midnight-blue", Executive: true},
	ArchetypeSalesPitch:       {Template: "warm-coral", Executive: false},
	ArchetypeStrategyProposal: {Template: "forest-green", Executive: true},
	ArchetypeProjectRoadmap:   {Template: "modern-template", Executive: false},
	ArchetypeMarketAnalysis:   {Template: "forest-green", Executive: false},
}

// DefaultsFor returns the archetype's deterministic defaults. An unknown or
// empty archetype yields the zero value (no template preference, not executive).
func DefaultsFor(a Archetype) ArchetypeDefaults {
	return archetypeDefaults[a]
}

// LookupArchetype returns the ArchetypeInfo and whether it is registered.
func LookupArchetype(a Archetype) (ArchetypeInfo, bool) {
	info, ok := archetypeRegistry[a]
	return info, ok
}

// Valid reports whether the archetype is registered.
func (a Archetype) Valid() bool {
	_, ok := archetypeRegistry[a]
	return ok
}

// AllArchetypes returns every registered archetype in stable (sorted) order.
func AllArchetypes() []Archetype {
	out := make([]Archetype, 0, len(archetypeRegistry))
	for a := range archetypeRegistry {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
