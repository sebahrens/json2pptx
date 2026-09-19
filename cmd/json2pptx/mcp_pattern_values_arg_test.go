package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestPatternValuesArgAcceptsArrays is the go-slide-creator-ldu3d acceptance
// test: the advertised inputSchema for the pattern tools must allow the array
// form, because nine registered patterns require it. A client that validates
// arguments against inputSchema could not call those patterns at all, and the
// server accepted them only because it does not enforce its own schema.
func TestPatternValuesArgAcceptsArrays(t *testing.T) {
	// The full profile, because validate_pattern is not a core tool;
	// expand_pattern is, and both declare the same argument.
	withToolProfile(t, toolProfileAll)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, s)

	var checked int
	for _, tool := range tools {
		if tool.Name != "expand_pattern" && tool.Name != "validate_pattern" {
			continue
		}
		checked++
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s inputSchema: %v", tool.Name, err)
		}
		var schema struct {
			Properties map[string]struct {
				Type        any              `json:"type"`
				OneOf       []map[string]any `json:"oneOf"`
				Description string           `json:"description"`
			} `json:"properties"`
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("decode %s inputSchema: %v", tool.Name, err)
		}
		values, ok := schema.Properties["values"]
		if !ok {
			t.Fatalf("%s declares no values property", tool.Name)
		}
		if values.Type != nil {
			t.Errorf("%s values still declares type %v — an array argument fails that schema", tool.Name, values.Type)
		}
		var kinds []string
		for _, branch := range values.OneOf {
			if s, ok := branch["type"].(string); ok {
				kinds = append(kinds, s)
			}
		}
		slices.Sort(kinds)
		if !slices.Equal(kinds, []string{"array", "object"}) {
			t.Errorf("%s values oneOf = %v, want object and array", tool.Name, kinds)
		}
		if !slices.Contains(schema.Required, "values") {
			t.Errorf("%s no longer requires values", tool.Name)
		}
		// The description has to say which patterns need the array form, or the
		// union just moves the guess from the client to the model.
		for _, name := range []string{"kpi-3up", "timeline-horizontal"} {
			if !strings.Contains(values.Description, name) {
				t.Errorf("%s values description does not name %s: %q", tool.Name, name, values.Description)
			}
		}
	}
	if checked != 2 {
		t.Fatalf("checked %d pattern tools, want expand_pattern and validate_pattern", checked)
	}
}

// TestArrayValuedPatternNamesMatchesTheRegistry keeps the advertised list from
// drifting: it is generated from the same schemas the validator enforces, and a
// pattern added tomorrow must show up without anyone editing a string.
func TestArrayValuedPatternNamesMatchesTheRegistry(t *testing.T) {
	want := map[string]bool{}
	for _, p := range patterns.Default().List() {
		if vs := p.Schema().Properties()["values"]; vs != nil && vs.TypeName() == patterns.TypeArray {
			want[p.Name()] = true
		}
	}
	got := arrayValuedPatternNames()
	if len(got) != len(want) {
		t.Fatalf("arrayValuedPatternNames() = %v (%d), want %d entries", got, len(got), len(want))
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("%s is listed as array-valued but its schema says otherwise", name)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("list is not sorted, so tools/list churns between runs: %v", got)
	}
	// The kpi family is the reported case; if it ever stops being array-valued
	// this test should be revisited rather than silently passing on an empty set.
	if !slices.Contains(got, "kpi-3up") {
		t.Errorf("kpi-3up missing from %v", got)
	}
}

// TestPatternToolsAcceptBothShapes exercises the handlers themselves: the array
// form for an array pattern and the object form for an object pattern both
// expand, so the widened schema describes what the server actually does.
func TestPatternToolsAcceptBothShapes(t *testing.T) {
	cases := []struct {
		name   string
		values any
	}{
		{"kpi-3up", []any{
			map[string]any{"value": "42%", "label": "Growth"},
			map[string]any{"value": "1.2M", "label": "ARR"},
			map[string]any{"value": "11%", "label": "Churn"},
		}},
		{"pull-quote", map[string]any{"quote": "It worked.", "attribution": "A customer"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &mcpConfig{templatesDir: "../../templates"}
			res, err := mc.handleExpandPattern(t.Context(), makeRequest(map[string]any{
				"name":   tc.name,
				"values": tc.values,
			}))
			if err != nil {
				t.Fatalf("expand_pattern(%s): %v", tc.name, err)
			}
			if res.IsError {
				t.Fatalf("expand_pattern(%s) errored: %s", tc.name, resultText(res))
			}
		})
	}
}
