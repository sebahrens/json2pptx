package semantic

import (
	"encoding/json"
	"testing"
)

func TestSchemaIsValidJSON(t *testing.T) {
	out, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	// Round-trip to confirm the schema marshals to valid JSON.
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if back["$schema"] != schemaDialect {
		t.Errorf("$schema = %v, want %v", back["$schema"], schemaDialect)
	}
}

func TestSchemaKindEnumMatchesRegistry(t *testing.T) {
	schema := Schema()
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("schema missing $defs")
	}
	slide, ok := defs["SlideSpec"].(map[string]any)
	if !ok {
		t.Fatal("schema missing $defs.SlideSpec")
	}
	props := slide["properties"].(map[string]any)
	kind := props["kind"].(map[string]any)
	enum, ok := kind["enum"].([]any)
	if !ok {
		t.Fatal("SlideSpec.kind has no enum")
	}
	if len(enum) != len(slideKindRegistry) {
		t.Errorf("kind enum has %d entries, registry has %d", len(enum), len(slideKindRegistry))
	}
	got := make(map[string]bool, len(enum))
	for _, e := range enum {
		got[e.(string)] = true
	}
	for k := range slideKindRegistry {
		if !got[string(k)] {
			t.Errorf("kind enum missing %q", k)
		}
	}
}

func TestSchemaEmitsPerKindVariants(t *testing.T) {
	schema := Schema()
	defs := schema["$defs"].(map[string]any)
	slide := defs["SlideSpec"].(map[string]any)

	// SlideSpec is a discriminated union: one oneOf $ref per registered kind.
	oneOf, ok := slide["oneOf"].([]any)
	if !ok {
		t.Fatal("SlideSpec missing oneOf discriminated union")
	}
	if len(oneOf) != len(slideKindRegistry) {
		t.Errorf("oneOf has %d variants, registry has %d", len(oneOf), len(slideKindRegistry))
	}

	for k, info := range slideKindRegistry {
		name := kindDefName(k)
		variant, ok := defs[name].(map[string]any)
		if !ok {
			t.Errorf("missing $defs.%s for kind %q", name, k)
			continue
		}
		props := variant["properties"].(map[string]any)

		// kind is pinned via const.
		kindProp, ok := props["kind"].(map[string]any)
		if !ok || kindProp["const"] != string(k) {
			t.Errorf("kind %q: variant does not pin kind via const, got %v", k, props["kind"])
		}

		// Every required field is a documented property. A field with no aliases is
		// listed in the flat required array; a field with aliases is expressed as
		// required-one-of (an anyOf over the field and its aliases, under allOf) so
		// an alias-only spec stays schema-valid.
		req := variant["required"].([]any)
		reqSet := make(map[string]bool, len(req))
		for _, r := range req {
			reqSet[r.(string)] = true
		}
		oneOfRequired := requiredOneOfFields(variant)
		for _, f := range info.RequiredFields {
			if _, ok := props[f]; !ok {
				t.Errorf("kind %q: required field %q missing from variant properties", k, f)
			}
			if aliases := info.RequiredAliases[f]; len(aliases) > 0 {
				if reqSet[f] {
					t.Errorf("kind %q: aliased required field %q must not be in the flat required list (use required-one-of)", k, f)
				}
				if !oneOfRequired[f] {
					t.Errorf("kind %q: aliased required field %q missing from required-one-of (allOf/anyOf)", k, f)
				}
				for _, a := range aliases {
					if _, ok := props[a]; !ok {
						t.Errorf("kind %q: alias %q for %q missing from variant properties", k, a, f)
					}
					if !oneOfRequired[a] {
						t.Errorf("kind %q: alias %q missing from required-one-of (allOf/anyOf)", k, a)
					}
				}
				continue
			}
			if !reqSet[f] {
				t.Errorf("kind %q: required field %q missing from variant required list", k, f)
			}
		}
		// Every typical field is a documented property.
		for _, f := range info.TypicalFields {
			if _, ok := props[f]; !ok {
				t.Errorf("kind %q: typical field %q missing from variant properties", k, f)
			}
		}
	}
}

// requiredOneOfFields collects the field names named in a variant's
// required-one-of clauses (allOf -> anyOf -> {required:[field]}), so a test can
// assert that an aliased required field and its aliases are expressed there.
func requiredOneOfFields(variant map[string]any) map[string]bool {
	out := map[string]bool{}
	groups, ok := variant["allOf"].([]any)
	if !ok {
		return out
	}
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		opts, ok := gm["anyOf"].([]any)
		if !ok {
			continue
		}
		for _, o := range opts {
			om, ok := o.(map[string]any)
			if !ok {
				continue
			}
			req, ok := om["required"].([]any)
			if !ok {
				continue
			}
			for _, r := range req {
				if name, ok := r.(string); ok {
					out[name] = true
				}
			}
		}
	}
	return out
}

func TestSchemaArchetypeEnumMatchesRegistry(t *testing.T) {
	schema := Schema()
	defs := schema["$defs"].(map[string]any)
	meta := defs["DeckMeta"].(map[string]any)
	props := meta["properties"].(map[string]any)
	arch := props["archetype"].(map[string]any)
	enum, ok := arch["enum"].([]any)
	if !ok {
		t.Fatal("DeckMeta.archetype has no enum")
	}
	if len(enum) != len(archetypeRegistry) {
		t.Errorf("archetype enum has %d entries, registry has %d", len(enum), len(archetypeRegistry))
	}
}
