// Package textwalk provides a single traversal over every authored string in a
// presentation input.
//
// Working from JSON (marshal + recursive walk) rather than per-field plumbing
// means a scan covers typed fields, raw json.RawMessage overrides, shape grids,
// pattern values and any future schema addition without being taught about
// them. The emoji policy established this approach; it is shared here so
// additional text policies (unsupported inline markup, placeholder tokens, …)
// cannot drift from it or miss a field it already reaches.
package textwalk

import (
	"encoding/json"
	"fmt"
)

// Strings visits every string value reachable in input, passing the value and a
// JSON-style accessor path (e.g. "slides[2].content[0].text_value"). A nil input
// or one that cannot be marshalled visits nothing.
func Strings(input any, visit func(value, path string)) {
	if input == nil || visit == nil {
		return
	}
	data, err := json.Marshal(input)
	if err != nil {
		return
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}
	walk(root, "", visit)
}

func walk(v any, path string, visit func(value, path string)) {
	switch n := v.(type) {
	case string:
		visit(n, path)
	case map[string]any:
		for k, child := range n {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walk(child, next, visit)
		}
	case []any:
		for i, child := range n {
			walk(child, fmt.Sprintf("%s[%d]", path, i), visit)
		}
	}
}

// DisplayPath returns a human-friendly variant of a JSON path for messages.
// An empty path renders as "<root>" so a message is never blank.
func DisplayPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
