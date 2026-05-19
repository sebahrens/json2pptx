package patterns

import (
	"encoding/json"
	"fmt"
)

// FillPlaceholder is the sentinel token used in plan_deck skeleton output for
// agent-supplied content. String leaves in a skeleton's pattern values and in
// the slide-level title placeholder are replaced with this token so an agent
// can do a literal find-and-replace pass instead of re-deriving the slide
// structure from prose.
const FillPlaceholder = "__FILL__"

// SkeletonForPattern returns a fillable slide-JSON object for the named
// pattern. The returned bytes parse as a valid SlideInput shape (layout_id,
// content[], pattern{name, values}) with every string leaf replaced by
// FillPlaceholder. The agent's job is to overwrite each FillPlaceholder
// occurrence with real content.
//
// Numeric and boolean leaves are preserved so structural defaults (grid
// dimensions, flags) survive the round-trip and the skeleton remains valid for
// validate_input as-is — FillPlaceholder is a non-empty string and satisfies
// required-string checks.
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
	filledValues := replaceStringLeaves(decoded, FillPlaceholder)

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

// replaceStringLeaves walks v, replacing every string leaf with placeholder.
// Numbers, booleans, nulls, and structural keys are preserved. Maps and slices
// are mutated in place; the (possibly mutated) value is also returned for
// convenience.
func replaceStringLeaves(v any, placeholder string) any {
	switch x := v.(type) {
	case string:
		// Preserve empty strings — they signal "omit field" in many patterns.
		if x == "" {
			return ""
		}
		return placeholder
	case []any:
		for i := range x {
			x[i] = replaceStringLeaves(x[i], placeholder)
		}
		return x
	case map[string]any:
		for k, val := range x {
			x[k] = replaceStringLeaves(val, placeholder)
		}
		return x
	default:
		return v
	}
}
