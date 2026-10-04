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
	"strconv"
	"strings"
)

// Strings visits every string value reachable in input, passing the value and a
// JSON-style accessor path (e.g. "slides[2].content[0].text_value"). A nil input
// or one that cannot be marshalled visits nothing.
func Strings(input any, visit func(value, path string)) {
	if visit == nil {
		return
	}
	StringsAt(input, func(value, path, _ string) { visit(value, path) })
}

// StringsAt is Strings with the address of each value as well: pointer is its
// JSON Pointer (RFC 6901) in the marshalled input, the form a finding reports
// and a patch takes ("/slides/2/content/0/text_value").
func StringsAt(input any, visit func(value, path, pointer string)) {
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
	walk(root, "", "", visit)
}

func walk(v any, path, pointer string, visit func(value, path, pointer string)) {
	switch n := v.(type) {
	case string:
		visit(n, path, pointer)
	case map[string]any:
		for k, child := range n {
			next := k
			if path != "" {
				next = path + "." + k
			}
			token := strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
			walk(child, next, pointer+"/"+token, visit)
		}
	case []any:
		for i, child := range n {
			walk(child, fmt.Sprintf("%s[%d]", path, i), pointer+"/"+strconv.Itoa(i), visit)
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
