package semantic

import (
	"sort"
	"testing"
)

// propertyKeys returns the property names of a schema node, sorted.
func propertyKeys(node map[string]any) []string {
	props, _ := node["properties"].(map[string]any)
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compactKeysWithAliases returns the property names of a compact schema node
// together with the aliases its properties name, sorted.
func compactKeysWithAliases(t *testing.T, where string, node map[string]any) []string {
	t.Helper()
	props, _ := node["properties"].(map[string]any)
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if seen[k] {
			t.Errorf("%s: %q appears twice (as a field and as an alias)", where, k)
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for name, raw := range props {
		add(name)
		p, _ := raw.(map[string]any)
		aliases, _ := p["aliases"].([]any)
		for _, a := range aliases {
			add(a.(string))
		}
	}
	sort.Strings(keys)
	return keys
}

// entryObjects returns the closed entry objects directly below a field
// schema: the items object, or the object alternative of a string-or-object
// entry.
func entryObjects(field map[string]any) []map[string]any {
	var out []map[string]any
	items, _ := field["items"].(map[string]any)
	if items == nil {
		return nil
	}
	if _, ok := items["properties"]; ok {
		out = append(out, items)
	}
	alts, _ := items["anyOf"].([]any)
	for _, a := range alts {
		if m, _ := a.(map[string]any); m != nil {
			if _, ok := m["properties"]; ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// TestCompactItemSchemaListsEachFieldOnce pins go-slide-creator-l6mcj: the
// compact schema names every key the expanded schema accepts exactly once —
// as a canonical field or as an alias of one — at the top level and inside
// every list entry, so nothing the compiler reads is lost from view.
func TestCompactItemSchemaListsEachFieldOnce(t *testing.T) {
	folded := 0
	for _, k := range AllSlideKinds() {
		full := KindItemSchema(k)
		compact := KindItemSchemaCompact(k)
		if got, want := compactKeysWithAliases(t, string(k), compact), propertyKeys(full); !equalStrings(got, want) {
			t.Errorf("%s: compact fields + aliases = %v, expanded schema has %v", k, got, want)
		}
		fullProps, _ := full["properties"].(map[string]any)
		compactProps, _ := compact["properties"].(map[string]any)
		folded += len(fullProps) - len(compactProps)
		for name, raw := range compactProps {
			field, _ := raw.(map[string]any)
			if _, isRef := field["$ref"]; isRef {
				t.Errorf("%s.%s is still an alias $ref in the compact schema", k, name)
			}
			fullField, _ := fullProps[name].(map[string]any)
			compactEntries, fullEntries := entryObjects(field), entryObjects(fullField)
			if len(compactEntries) != len(fullEntries) {
				t.Errorf("%s.%s: %d entry objects, expanded has %d", k, name, len(compactEntries), len(fullEntries))
				continue
			}
			for i := range compactEntries {
				where := string(k) + "." + name + "[]"
				if got, want := compactKeysWithAliases(t, where, compactEntries[i]), propertyKeys(fullEntries[i]); !equalStrings(got, want) {
					t.Errorf("%s: compact keys + aliases = %v, expanded entry has %v", where, got, want)
				}
				folded += len(propertyKeys(fullEntries[i])) - len(propertyKeys(compactEntries[i]))
			}
		}
		// A required-one-of group survives only for alternatives that stayed
		// fields (another shape than the canonical one).
		groups, _ := compact["allOf"].([]any)
		for _, g := range groups {
			for _, alt := range g.(map[string]any)["anyOf"].([]any) {
				name := alt.(map[string]any)["required"].([]any)[0].(string)
				if _, ok := compactProps[name]; !ok {
					t.Errorf("%s: required-one-of names %q, which is not a field of the compact schema", k, name)
				}
			}
		}
	}
	if folded < 100 {
		t.Errorf("only %d alias keys were folded; the catalogue has well over a hundred", folded)
	}

	// The alias groups are what the compilers read.
	kpis := KindItemSchemaCompact(KindKPISnapshot)["properties"].(map[string]any)["kpis"].(map[string]any)
	entry := entryObjects(kpis)[0]["properties"].(map[string]any)
	value, _ := entry["value"].(map[string]any)
	if aliases, _ := value["aliases"].([]any); len(aliases) != 1 || aliases[0] != "big" {
		t.Errorf("kpis[].value aliases = %v, want [big]", value["aliases"])
	}
	// The one-quote shorthand is a string, not the quotes list under another
	// name: it stays a field, and its own alias folds into it.
	quote := KindItemSchemaCompact(KindQuote)["properties"].(map[string]any)
	shorthand, _ := quote["quote"].(map[string]any)
	if shorthand == nil {
		t.Fatal("quote.quote was folded into quotes although it is a string")
	}
	if aliases, _ := shorthand["aliases"].([]any); len(aliases) != 1 || aliases[0] != "text" {
		t.Errorf("quote.quote aliases = %v, want [text]", shorthand["aliases"])
	}
	if _, ok := entry["trend"]; !ok {
		t.Error("kpis[].trend is appended to delta, not an alias of it: it must stay a field")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
