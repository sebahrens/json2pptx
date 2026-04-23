package patterns

import "encoding/json"

// ---------------------------------------------------------------------------
// Exemplar — optional interface for size-metric golden testing
// ---------------------------------------------------------------------------

// Exemplar is an optional interface patterns can implement to provide canonical
// example values for the size-metric regression harness. The returned value must
// be a pointer to the pattern's Values type (same type as NewValues returns).
type Exemplar interface {
	ExemplarValues() any
}

// ---------------------------------------------------------------------------
// Size metrics — D12: heuristic byte count, NOT tokenizer-exact
// ---------------------------------------------------------------------------

// SizeMetric records the byte counts of the expanded and compact forms of a
// pattern with given exemplar values.
type SizeMetric struct {
	Pattern       string `json:"pattern"`
	ExpandedBytes int    `json:"expanded_bytes"`
	CompactBytes  int    `json:"compact_bytes"`
}

// CanonicalSizeBytes returns the byte count of the JSON-marshalled expanded
// shape_grid output for the given pattern and exemplar values.
func CanonicalSizeBytes(p Pattern, exemplarValues any) (int, error) {
	grid, err := p.Expand(ExpandContext{}, exemplarValues, nil, nil)
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(grid)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// patternInput mirrors the compact JSON form an LLM would write.
type patternInput struct {
	Name   string `json:"name"`
	Values any    `json:"values"`
}

// PatternInputSizeBytes returns the byte count of the JSON-marshalled compact
// PatternInput form for the given pattern and exemplar values.
func PatternInputSizeBytes(p Pattern, exemplarValues any) (int, error) {
	pi := patternInput{
		Name:   p.Name(),
		Values: exemplarValues,
	}
	data, err := json.Marshal(pi)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}
