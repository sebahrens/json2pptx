package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestSchemaDocsDoctor enforces that SLIDE_FORMAT.md and docs/INPUT_FORMAT.md
// remain short tutorials that point at the canonical schema rather than
// duplicating it. The canonical source is get_input_schema (MCP) /
// `json2pptx input-schema` (CLI), generated from the Go input structs.
//
// Three checks per file:
//
//  1. References the canonical schema (literal "get_input_schema").
//  2. Stays under a max line budget (tutorials, not references).
//  3. Does NOT contain markdown table rows that redefine a Go schema field —
//     i.e., lines of the form `| `fieldname` | ... |` where `fieldname` is a
//     property in the live PresentationInput schema. Prose mentions and JSON
//     examples are fine; field-definition tables are not.
func TestSchemaDocsDoctor(t *testing.T) {
	files := []struct {
		path        string
		maxLines    int
		description string
	}{
		{"../../SLIDE_FORMAT.md", 120, "root-level quick tutorial"},
		{"../../docs/INPUT_FORMAT.md", 250, "docs/ tutorial"},
	}

	schemaFields := collectSchemaPropertyNames()

	for _, f := range files {
		f := f
		t.Run(f.path, func(t *testing.T) {
			data, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatalf("cannot read %s: %v", f.path, err)
			}
			text := string(data)

			t.Run("references_get_input_schema", func(t *testing.T) {
				if !strings.Contains(text, "get_input_schema") {
					t.Errorf("%s (%s) must reference `get_input_schema` "+
						"so readers know where the canonical schema lives", f.path, f.description)
				}
			})

			t.Run("line_budget", func(t *testing.T) {
				n := strings.Count(text, "\n") + 1
				if n > f.maxLines {
					t.Errorf("%s has %d lines (max %d); keep it a tutorial and "+
						"defer the canonical field list to get_input_schema",
						f.path, n, f.maxLines)
				}
			})

			t.Run("no_field_definition_tables", func(t *testing.T) {
				offenders := findFieldDefinitionRows(text, schemaFields)
				if len(offenders) > 0 {
					t.Errorf("%s redefines %d Go schema field(s) in markdown "+
						"tables: %s\n"+
						"  These rows duplicate the canonical schema and "+
						"drift over time. Move them to the get_input_schema "+
						"description for the field, or rephrase as prose.",
						f.path, len(offenders), strings.Join(offenders, ", "))
				}
			})
		})
	}
}

// collectSchemaPropertyNames walks the buildInputSchema() output, descending
// into every $defs entry, and returns the set of all property names defined
// anywhere in the schema. This is the live Go-derived field vocabulary.
func collectSchemaPropertyNames() map[string]bool {
	out := make(map[string]bool)
	schema := buildInputSchema()

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		return out
	}
	for _, def := range defs {
		m, ok := def.(map[string]any)
		if !ok {
			continue
		}
		props, ok := m["properties"].(map[string]any)
		if !ok {
			continue
		}
		for name := range props {
			out[name] = true
		}
	}
	return out
}

// fieldDefRowRE matches a markdown table row whose first cell is a single
// backtick-wrapped lowercase identifier. Examples that match:
//
//	| `template` | string | ... |
//	|`slides`|array|...|
//
// Examples that do NOT match (prose mentions, multi-token cells, code blocks):
//
//	The `template` field selects ...
//	| `slides[].layout_id` | string | ... |
//	| `none` | the engine default |
var fieldDefRowRE = regexp.MustCompile("^\\s*\\|\\s*`([a-z_][a-z0-9_]*)`\\s*\\|")

// TestSchemaDocsDoctor_DetectorSanity verifies the field-definition-row
// detector actually flags table-style redefinitions and ignores prose mentions
// / JSON examples. Guards against the doctor test silently passing if the
// regex breaks.
func TestSchemaDocsDoctor_DetectorSanity(t *testing.T) {
	schemaFields := map[string]bool{
		"template":    true,
		"slides":      true,
		"slide_type":  true,
		"text_value":  true,
		"placeholder": true,
	}

	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "flags table row first cell",
			input: "| `template` | string | name |\n" +
				"|`slides`|array|list|\n",
			want: []string{"slides", "template"},
		},
		{
			name:  "ignores prose mention",
			input: "The `template` field selects the .pptx file.",
			want:  nil,
		},
		{
			name:  "ignores JSON example",
			input: "  \"template\": \"midnight-blue\",\n",
			want:  nil,
		},
		{
			name:  "ignores non-first-cell mentions",
			input: "| `text` | `text_value` (string) | the body text |\n",
			want:  nil,
		},
		{
			name:  "ignores field not in schema",
			input: "| `unrelated_doc_field` | string | placeholder |\n",
			want:  nil,
		},
		{
			name: "ignores compound paths",
			input: "| `slides[].layout_id` | string | per-slide |\n" +
				"| `defaults.table_style` | object | deck default |\n",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findFieldDefinitionRows(tc.input, schemaFields)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// findFieldDefinitionRows returns the sorted, de-duplicated set of schema
// field names that appear as the first cell of any markdown table row.
func findFieldDefinitionRows(text string, schemaFields map[string]bool) []string {
	seen := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		m := fieldDefRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if schemaFields[name] {
			seen[name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
