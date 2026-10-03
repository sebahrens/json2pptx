package semantic

import (
	"reflect"
	"sort"
	"strings"
)

// KindItemSchemaCompact returns the schema an author reads for one slide of
// the given kind: every canonical field once, with the alternative spellings
// the compiler also accepts named in an "aliases" annotation on that field
// instead of repeated as properties of their own (go-slide-creator-l6mcj).
// The same folding applies to the keys of list entries (a KPI's value / label
// / delta, a milestone's label / date / body).
//
// It is a reading aid, not a validator's schema: a spec written with an alias
// is still accepted by the compiler but is not valid against this schema.
// KindItemSchema is the expanded contract that lists every accepted key.
func KindItemSchemaCompact(k SlideKind) map[string]any {
	info, ok := LookupKind(k)
	if !ok {
		return nil
	}
	variant := kindVariantSchema(info)
	props, _ := variant["properties"].(map[string]any)
	foldAliases(props, kindFieldAliases(info))

	// The required-one-of groups exist to admit the aliases. What is left of
	// one is the alternatives of another shape (quote, a string, beside
	// quotes, a list), which stay fields of their own.
	delete(variant, "allOf")
	required := []any{"kind"}
	var groups []any
	for _, f := range info.RequiredFields {
		alternatives := []any{map[string]any{"required": []any{f}}}
		for _, alias := range info.RequiredAliases[f] {
			if _, kept := props[alias]; kept {
				alternatives = append(alternatives, map[string]any{"required": []any{alias}})
			}
		}
		if len(alternatives) == 1 {
			required = append(required, f)
			continue
		}
		groups = append(groups, map[string]any{"anyOf": alternatives})
	}
	variant["required"] = required
	if len(groups) > 0 {
		variant["allOf"] = groups
	}
	delete(variant, "title")
	delete(variant, "description") // the list_slide_kinds row carries the summary

	for name, raw := range props {
		p, _ := raw.(map[string]any)
		if p == nil {
			continue
		}
		if d, _ := p["description"].(string); d != "" {
			// "required" and typical_fields already say which is which.
			d = strings.TrimPrefix(d, "Required payload field. ")
			d = strings.TrimPrefix(d, "Typical (optional) payload field. ")
			p["description"] = d
		}
		if short, ok := compactUniversalDescriptions[name]; ok {
			p["description"] = short
		}
		foldEntryAliases(p)
	}
	return variant
}

// compactUniversalDescriptions are the fields every kind carries; their full
// descriptions repeat verbatim in each kind's schema.
var compactUniversalDescriptions = map[string]string{
	"id":      "Stable slide handle (letter first; unique). Never rendered; a patch may use it: /slides/<id>/title.",
	"notes":   "Speaker notes.",
	"source":  "Source line under the content.",
	"pattern": "Composition override: a pattern from this kind's compositions.",
	"layout":  "Composition override: a layout from this kind's compositions.",
}

// kindFieldAliases maps each alias field of a kind to its canonical field: the
// registered required-field aliases plus the fields documented "Alias for X.".
func kindFieldAliases(info KindInfo) map[string]string {
	canonicalOf := map[string]string{}
	for canonical, aliases := range info.RequiredAliases {
		for _, a := range aliases {
			canonicalOf[a] = canonical
		}
	}
	for name, f := range kindPayloadFields[info.Kind] {
		if rest, ok := strings.CutPrefix(f.desc, "Alias for "); ok {
			if canonical, _, found := strings.Cut(rest, "."); found {
				canonicalOf[name] = canonical
			}
		}
	}
	return canonicalOf
}

// foldAliases removes each alias property that has its canonical property's
// shape and records it in that property's "aliases" list. An alternative of
// another shape (the one-quote string beside the quotes list) is a field of
// its own and stays one.
func foldAliases(props map[string]any, canonicalOf map[string]string) {
	if len(props) == 0 || len(canonicalOf) == 0 {
		return
	}
	aliases := make([]string, 0, len(canonicalOf))
	for a := range canonicalOf {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		aliasSchema, _ := props[alias].(map[string]any)
		canonical, _ := props[canonicalOf[alias]].(map[string]any)
		if aliasSchema == nil || canonical == nil || !sameShape(aliasSchema, canonical) {
			continue
		}
		// An alias of an alias names the same field.
		names := append(aliasNames(canonical), alias)
		names = append(names, aliasNames(aliasSchema)...)
		sort.Strings(names)
		list := make([]any, len(names))
		for i, n := range names {
			list[i] = n
		}
		canonical["aliases"] = list
		delete(props, alias)
	}
}

// aliasNames reads a property's "aliases" annotation.
func aliasNames(prop map[string]any) []string {
	list, _ := prop["aliases"].([]any)
	names := make([]string, 0, len(list)+1)
	for _, a := range list {
		if s, ok := a.(string); ok {
			names = append(names, s)
		}
	}
	return names
}

// sameShape reports whether two property schemas describe the same value,
// setting aside their descriptions and alias annotations.
func sameShape(a, b map[string]any) bool {
	strip := func(m map[string]any) map[string]any {
		out := make(map[string]any, len(m))
		for k, v := range m {
			if k != "description" && k != "aliases" {
				out[k] = v
			}
		}
		return out
	}
	return reflect.DeepEqual(strip(a), strip(b))
}

// foldEntryAliases folds the alias keys of every closed entry object below a
// schema node (list entries, object-valued fields, region variants).
func foldEntryAliases(node any) {
	switch t := node.(type) {
	case map[string]any:
		if props, ok := t["properties"].(map[string]any); ok {
			keys := make([]string, 0, len(props))
			for k := range props {
				keys = append(keys, k)
			}
			foldAliases(props, entryKeyAliases[keySetID(keys)])
		}
		for _, v := range t {
			foldEntryAliases(v)
		}
	case []any:
		for _, v := range t {
			foldEntryAliases(v)
		}
	}
}
