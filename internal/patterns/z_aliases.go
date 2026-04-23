package patterns

// Default aliases for pattern names. Canonical names are preserved;
// aliases provide shorter or alternative forms for convenience.
//
// Naming convention (documented in docs/PATTERN_LIBRARY_SPEC.md §2.1):
//
//	Canonical form: {noun}-{qualifier}
//	  noun      = the layout concept (kpi, card, matrix, timeline, …)
//	  qualifier = variant detail: count+suffix (3up, 4up), dimensions (2x2),
//	              column count (2col), orientation (horizontal), or compound noun (grid, row, canvas)
//
//	Aliases drop the qualifier when unambiguous (e.g. "timeline" → "timeline-horizontal")
//	or use an abbreviation (e.g. "bmc" → "bmc-canvas").
func init() {
	r := Default()

	// Unambiguous short forms (single variant patterns).
	r.RegisterAlias("timeline", "timeline-horizontal")
	r.RegisterAlias("bmc", "bmc-canvas")
	r.RegisterAlias("matrix", "matrix-2x2")
	r.RegisterAlias("comparison", "comparison-2col")
}
