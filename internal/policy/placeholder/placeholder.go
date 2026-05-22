// Package placeholder is the single source of truth for detecting unresolved
// skeleton placeholder tokens (patterns.FillPlaceholder, "__FILL__") left in a
// presentation. plan_deck emits fillable skeletons whose string leaves are the
// __FILL__ sentinel so an agent can overwrite each occurrence with real content
// instead of re-deriving the slide structure from prose. A skeleton is draft
// scaffolding, not finished output — this package finds any tokens that survived
// so the generate/validate boundary can surface (or, in strict mode, refuse)
// unfinished decks instead of silently green-lighting them.
//
// Detection is JSON-based (marshal + recursive string walk) so the scan covers
// typed fields, speaker notes, shape_grid cell text, table cells, chart/diagram
// labels, pattern values, and any future schema additions without per-field
// plumbing — exactly like internal/policy/emoji. Keeping the token in one
// importable package means every producer and the enforcer stay consistent by
// construction.
package placeholder

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Token is the sentinel skeleton placeholder. It mirrors
// patterns.FillPlaceholder so the scanner and the skeleton generator can never
// drift apart: plan_deck writes this exact token, and this package looks for it.
const Token = patterns.FillPlaceholder

// FindingCode is the machine-readable code emitted for an unresolved
// placeholder. It is documented in the patterns finding-meta registry so
// describe_finding can resolve it.
const FindingCode = "unresolved_placeholder"

// Contains reports whether s still holds the unresolved placeholder token. The
// match is a substring so partially-edited strings ("Q3 __FILL__ results") are
// caught alongside bare tokens.
func Contains(s string) bool {
	return strings.Contains(s, Token)
}

// Violation is a single unresolved-placeholder hit located by Scan: the
// JSON-style path to the offending string and the full string value that still
// contains the token.
type Violation struct {
	// Path is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
	Path string
	// Value is the full offending string (it contains Token).
	Value string
	// Token is the sentinel that was found, so messages can name it without the
	// caller hardcoding the literal.
	Token string
}

// Scan marshals input to JSON and returns one Violation for every string value
// that still contains the placeholder token. Working from JSON means the scan
// covers typed fields, raw json.RawMessage overrides, shape grids, pattern
// overrides, speaker notes, table cells, chart labels, and any future schema
// additions without per-field plumbing. Violations are returned in stable path
// order.
func Scan(input any) []Violation {
	if input == nil {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}

	var violations []Violation
	walkJSONStrings(root, "", func(value, path string) {
		violations = append(violations, Violation{
			Path:  path,
			Value: value,
			Token: Token,
		})
	})

	// Stable order for deterministic diagnostics (JSON-walk visits map keys in
	// nondeterministic order).
	sort.SliceStable(violations, func(i, j int) bool {
		return violations[i].Path < violations[j].Path
	})
	return violations
}

// walkJSONStrings recursively visits every string value in a decoded JSON tree
// and calls visit when the string contains the placeholder token. The path
// argument is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
func walkJSONStrings(v any, path string, visit func(value, path string)) {
	switch n := v.(type) {
	case string:
		if Contains(n) {
			visit(n, path)
		}
	case map[string]any:
		for k, child := range n {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walkJSONStrings(child, next, visit)
		}
	case []any:
		for i, child := range n {
			walkJSONStrings(child, fmt.Sprintf("%s[%d]", path, i), visit)
		}
	}
}

// DisplayPath returns a human-friendly variant of a JSON path for messages.
// Empty paths render as "<root>" so a message is never blank.
func DisplayPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
