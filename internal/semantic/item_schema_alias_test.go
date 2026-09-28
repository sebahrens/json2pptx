package semantic

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestKindItemSchemaAliasesAreRefs pins go-slide-creator-ppned: an alias whose
// schema repeats its canonical field's is published as a $ref to it, and the
// ref always resolves to a sibling property.
func TestKindItemSchemaAliasesAreRefs(t *testing.T) {
	var refs int
	for _, k := range AllSlideKinds() {
		props, _ := KindItemSchema(k)["properties"].(map[string]any)
		for name, raw := range props {
			p, _ := raw.(map[string]any)
			ref, ok := p["$ref"].(string)
			if !ok {
				continue
			}
			refs++
			target := ref[len("#/properties/"):]
			if _, ok := props[target]; !ok {
				t.Errorf("%s.%s: $ref %q does not resolve", k, name, ref)
			}
		}
	}
	if refs == 0 {
		t.Fatal("no alias was published as a $ref")
	}
	cols := KindItemSchema(KindOptionMatrix)["properties"].(map[string]any)["columns"].(map[string]any)
	if cols["$ref"] != "#/properties/criteria" {
		t.Errorf("option_matrix columns = %v, want a $ref to criteria", cols)
	}
}

// TestOptionMatrixScoresAcceptTheExample: scores were typed string, so the
// kind's own example ([1, 1, 3, 2]) failed its schema.
func TestOptionMatrixScoresAcceptTheExample(t *testing.T) {
	props := KindItemSchema(KindOptionMatrix)["properties"].(map[string]any)
	entry := props["options"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	want := map[string]any{"type": "array", "items": map[string]any{"type": []any{"number", "string"}}}
	for _, key := range []string{"scores", "values"} {
		got, _ := json.Marshal(entry[key])
		exp, _ := json.Marshal(want)
		var g, e any
		_ = json.Unmarshal(got, &g)
		_ = json.Unmarshal(exp, &e)
		if !reflect.DeepEqual(g, e) {
			t.Errorf("options[].%s = %s, want %s", key, got, exp)
		}
	}
}
