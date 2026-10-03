package main

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// slideKindUniversalFields are accepted by every kind; the brief names them
// once for the response instead of once per kind.
var slideKindUniversalFields = map[string]bool{"id": true, "notes": true, "source": true, "pattern": true, "layout": true}

// slideKindBriefNote is the response-level line that covers them.
const slideKindBriefNote = "Every kind also takes id, notes and source (strings) and a pattern / layout override (fields:[\"compositions\"]). brief lists canonical fields only; fields:[\"item_schema\"] adds their descriptions and accepted aliases."

// slideKindBrief is the field list an agent looks one thing up in: each
// canonical payload field of the kind with a one-line signature — its type,
// the keys of its entries, whether it is required. Aliases and prose are left
// to item_schema; text budgets ride beside it in budgets
// (go-slide-creator-l6mcj).
func slideKindBrief(k semantic.SlideKind) map[string]string {
	schema := semantic.KindItemSchemaCompact(k)
	props, _ := schema["properties"].(map[string]any)
	// A required field is required unless an alternative of another shape
	// (the one-quote string beside the quotes list) stands in for it.
	required := map[string]string{}
	if info, ok := semantic.LookupKind(k); ok {
		for _, f := range info.RequiredFields {
			required[f] = "; required"
			var alternatives []string
			for _, alias := range info.RequiredAliases[f] {
				if _, kept := props[alias]; kept {
					alternatives = append(alternatives, alias)
				}
			}
			if len(alternatives) > 0 {
				required[f] = "; required (or " + strings.Join(alternatives, ", ") + ")"
			}
		}
	}
	out := make(map[string]string, len(props))
	for name, raw := range props {
		if name == "kind" || slideKindUniversalFields[name] {
			continue
		}
		prop, _ := raw.(map[string]any)
		out[name] = briefSignature(prop) + required[name]
	}
	return out
}

// briefSignature renders one schema node as a short type expression.
func briefSignature(node map[string]any) string {
	if node == nil {
		return "any"
	}
	if c, ok := node["const"].(string); ok {
		return c
	}
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		values := make([]string, 0, len(enum))
		for _, v := range enum {
			if s, ok := v.(string); ok {
				values = append(values, s)
			}
		}
		return strings.Join(values, " | ")
	}
	for _, union := range []string{"anyOf", "oneOf"} {
		if alts, ok := node[union].([]any); ok && len(alts) > 0 {
			return briefUnion(alts)
		}
	}
	switch t := node["type"].(type) {
	case string:
		switch t {
		case "array":
			items, _ := node["items"].(map[string]any)
			if items == nil {
				return "list"
			}
			return "list of " + briefSignature(items)
		case "object":
			return briefObject(node)
		default:
			return t
		}
	case []any:
		names := make([]string, 0, len(t))
		for _, v := range t {
			if s, ok := v.(string); ok {
				names = append(names, s)
			}
		}
		return strings.Join(names, " | ")
	}
	if _, ok := node["properties"].(map[string]any); ok {
		return briefObject(node)
	}
	return "any"
}

// briefObject lists an object's keys: required ones bare, the rest with "?".
func briefObject(node map[string]any) string {
	props, _ := node["properties"].(map[string]any)
	if len(props) == 0 {
		return "object"
	}
	required := map[string]bool{}
	if list, ok := node["required"].([]any); ok {
		for _, r := range list {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}
	if list, ok := node["required"].([]string); ok {
		for _, name := range list {
			required[name] = true
		}
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if required[keys[i]] != required[keys[j]] {
			return required[keys[i]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		p, _ := props[key].(map[string]any)
		part := key
		if c, ok := p["const"].(string); ok {
			part = key + ": " + c
		} else if !required[key] {
			part += "?"
		}
		parts = append(parts, part)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// briefUnion joins the alternatives of an anyOf / oneOf.
func briefUnion(alts []any) string {
	parts := make([]string, 0, len(alts))
	seen := map[string]bool{}
	for _, a := range alts {
		m, _ := a.(map[string]any)
		if m == nil {
			continue
		}
		// A bare {required:[…]} alternative constrains, it is not a shape.
		if _, hasType := m["type"]; !hasType && m["properties"] == nil && m["const"] == nil && m["enum"] == nil && m["anyOf"] == nil && m["oneOf"] == nil {
			continue
		}
		sig := briefSignature(m)
		if !seen[sig] {
			seen[sig] = true
			parts = append(parts, sig)
		}
	}
	if len(parts) == 0 {
		return "any"
	}
	return strings.Join(parts, " | ")
}
