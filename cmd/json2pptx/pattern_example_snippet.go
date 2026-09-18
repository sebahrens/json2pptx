package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// exampleSnippetPattern is the pattern whose exemplar is embedded in the
// generate_presentation / validate_input tool description as the one inline
// "here is what a pattern slide looks like" example.
const exampleSnippetPattern = "kpi-3up"

// patternExampleSnippet renders a single-line, schema-true `"pattern"` example
// for the named pattern, built from the pattern's own ExemplarValues().
//
// The description used to carry a hand-written example
// (`values: {items: [{label, value}, ...]}`) that no pattern has ever
// accepted: copying it passed validate_input and then failed
// generate_presentation with INPUT.INVALID_SLIDE, and the error pointed at
// unrelated patterns. Deriving the snippet from the registry means the one
// example an agent sees first cannot drift from the schema
// (go-slide-creator-e5tm).
//
// It never panics: if the pattern or its exemplar is unavailable, it degrades
// to a name-only snippet that is still valid JSON and still points at
// show_pattern.
func patternExampleSnippet(name string) string {
	fallback := fmt.Sprintf(`{"pattern":{"name":%q,"values":...}} (call show_pattern for the exact values shape)`, name)

	pat, ok := patterns.Default().Get(name)
	if !ok {
		return fallback
	}
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		return fallback
	}
	values, err := json.Marshal(ex.ExemplarValues())
	if err != nil {
		return fallback
	}
	return fmt.Sprintf(`{"pattern":{"name":%q,"values":%s}}`, name, values)
}
