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
