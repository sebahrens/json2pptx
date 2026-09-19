package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// The `values` argument of expand_pattern / validate_pattern was declared
// {"type":"object"}, but nine patterns take an ARRAY there — kpi-2up..6up,
// icon-row, stylish-panels, timeline-horizontal. A client that validates tool
// arguments against inputSchema could not call those patterns at all, and the
// server accepted the array only because it does not enforce its own advertised
// schema (go-slide-creator-ldu3d).
//
// The declared type now covers both, and the description names the array
// patterns — generated from the registry, so the list cannot drift from the
// code that decides it.

// arrayValuedPatternNames returns the registered patterns whose `values` is an
// array rather than an object, in stable order.
func arrayValuedPatternNames() []string {
	var out []string
	for _, p := range patterns.Default().List() {
		schema := p.Schema()
		if schema == nil {
			continue
		}
		if vs := schema.Properties()["values"]; vs != nil && vs.TypeName() == patterns.TypeArray {
			out = append(out, p.Name())
		}
	}
	sort.Strings(out)
	return out
}

// patternValuesDescription is the shared description of the `values` argument.
func patternValuesDescription() string {
	arrays := arrayValuedPatternNames()
	if len(arrays) == 0 {
		return "Pattern values. The shape is per pattern: call show_pattern and read schema.properties.values."
	}
	return fmt.Sprintf(
		"Pattern values. The shape is PER PATTERN — call show_pattern and read schema.properties.values for the one you are expanding. Most patterns take an object; these %d take an ARRAY of cells: %s.",
		len(arrays), strings.Join(arrays, ", "))
}

// patternValuesArg declares the `values` argument shared by expand_pattern and
// validate_pattern: required, and typed as object-or-array.
func patternValuesArg() mcp.ToolOption {
	return mcp.WithObject("values",
		mcp.Required(),
		mcp.Description(patternValuesDescription()),
		withObjectOrArray(),
	)
}

// withObjectOrArray replaces a property's declared type with a union of object
// and array, so a schema-validating client accepts both spellings. The empty
// "properties" the object builder leaves behind goes with it: under a oneOf it
// constrains nothing and only invites a reader to think it does.
func withObjectOrArray() mcp.PropertyOption {
	return func(schema map[string]any) {
		delete(schema, "type")
		if props, ok := schema["properties"].(map[string]any); ok && len(props) == 0 {
			delete(schema, "properties")
		}
		schema["oneOf"] = []any{
			map[string]any{"type": "object"},
			map[string]any{"type": "array"},
		}
	}
}
