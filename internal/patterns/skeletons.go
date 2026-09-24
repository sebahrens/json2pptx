package patterns

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FillPlaceholder is the sentinel token used in plan_deck skeleton output for
// agent-supplied content. Free-text leaves in pattern values and the slide
// title are replaced with this token; schema-constrained strings retain valid
// defaults and are flagged for review in speaker notes.
const FillPlaceholder = "__FILL__"

// SkeletonForPattern returns a fillable slide-JSON object for the named
// pattern. Free-text leaves become FillPlaceholder, while schema-constrained
// leaves keep valid defaults. speaker_notes lists the choices that need review.
//
// Numeric and boolean leaves are preserved so structural defaults (grid
// dimensions, flags) survive the round-trip. FillPlaceholder is a non-empty
// string and satisfies required-string checks. The unresolved-placeholder scan
// (internal/policy/placeholder) still reports any FillPlaceholder left in place
// as an advisory finding, so callers must replace every token before publishable
// generation (or pass placeholder_policy=strict to make leftovers blocking).
//
// Returns nil when the pattern is unknown or does not implement Exemplar.
func SkeletonForPattern(reg *Registry, patternName, narrativeRole string) (json.RawMessage, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil registry")
	}
	pat, ok := reg.Get(patternName)
	if !ok {
		return nil, fmt.Errorf("unknown pattern %q", patternName)
	}
	ex, ok := pat.(Exemplar)
	if !ok {
		return nil, fmt.Errorf("pattern %q has no Exemplar", patternName)
	}
	values := ex.ExemplarValues()
	if values == nil {
		return nil, fmt.Errorf("pattern %q ExemplarValues returned nil", patternName)
	}

	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal exemplar values: %w", err)
	}
	var decoded any
	if err := json.Unmarshal(valuesJSON, &decoded); err != nil {
		return nil, fmt.Errorf("decode exemplar values: %w", err)
	}
	schema := pat.Schema()
	var choices []string
	filledValues := fillSkeletonValues(decoded, schema.Property("values"), schema, "values", &choices)

	slide := map[string]any{
		"layout_id": layoutIDForNarrativeRole(narrativeRole),
		"content": []map[string]any{
			{
				"placeholder_id": "title",
				"type":           "text",
				"text_value":     FillPlaceholder,
			},
		},
		"pattern": map[string]any{
			"name":   pat.Name(),
			"values": filledValues,
		},
	}
	if len(choices) > 0 {
		slide["speaker_notes"] = FillPlaceholder + " __CHOOSE__: Review these schema-constrained defaults before publishing: " + strings.Join(choices, ", ")
	}

	out, err := json.Marshal(slide)
	if err != nil {
		return nil, fmt.Errorf("marshal skeleton: %w", err)
	}
	return out, nil
}

// layoutIDForNarrativeRole returns the canonical layout ID for a given plan
// narrative role. Canonical names (title, blank, content, section, closing)
// are resolved at validate time against the template's layouts.
func layoutIDForNarrativeRole(role string) string {
	switch role {
	case "opening":
		return "title"
	case "closing":
		return "closing"
	case "framework":
		return "section"
	default:
		// evidence, comparison, emphasis, and unknown roles use blank so the
		// pattern owns the visual content without a competing body placeholder.
		return "blank"
	}
}

// fillSkeletonValues preserves typed schema defaults instead of placing a text
// sentinel in enums, icon references, or numeric-or-text scores.
func fillSkeletonValues(v any, schema, root *Schema, path string, choices *[]string) any {
	schema = schema.Deref(root)
	if schema != nil {
		if len(schema.raw.Enum) > 0 {
			*choices = append(*choices, path)
			return schema.raw.Enum[0]
		}
		if schema.raw.Default != nil {
			var def any
			if json.Unmarshal(*schema.raw.Default, &def) == nil {
				*choices = append(*choices, path)
				return def
			}
		}
		for _, branch := range schema.OneOfBranches() {
			branch = branch.Deref(root)
			if skeletonBranchMatches(v, branch) {
				schema = branch
				break
			}
		}
	}
	switch x := v.(type) {
	case string:
		// Preserve empty strings — they signal "omit field" in many patterns.
		if x == "" {
			return ""
		}
		return FillPlaceholder
	case []any:
		for i := range x {
			var item *Schema
			if schema != nil {
				item = schema.ItemSchema()
			}
			x[i] = fillSkeletonValues(x[i], item, root, fmt.Sprintf("%s[%d]", path, i), choices)
		}
		return x
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var property *Schema
			if schema != nil {
				property = schema.Property(k)
			}
			x[k] = fillSkeletonValues(x[k], property, root, path+"."+k, choices)
		}
		return x
	default:
		return v
	}
}

func skeletonBranchMatches(v any, branch *Schema) bool {
	if branch == nil {
		return false
	}
	switch v.(type) {
	case string:
		return branch.TypeName() == TypeString
	case map[string]any:
		return branch.TypeName() == TypeObject
	case []any:
		return branch.TypeName() == TypeArray
	case float64:
		return branch.TypeName() == TypeNumber || branch.TypeName() == TypeInteger
	case bool:
		return branch.TypeName() == TypeBoolean
	default:
		return false
	}
}
