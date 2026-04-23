package patterns

import (
	"encoding/json"
	"testing"
)

func TestObjectSchemaRoundTrip(t *testing.T) {
	// Build a schema: object with required string "title" and optional
	// array of strings "tags".
	s := ObjectSchema(
		map[string]*Schema{
			"title": StringSchema(100).WithDescription("The slide title"),
			"tags":  ArraySchema(StringSchema(0), 1, 10).WithDescription("Tags for the slide"),
		},
		[]string{"title"},
	).AsRoot().WithDescription("Sample pattern schema")

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	// Unmarshal into a generic map to validate structure.
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Check $schema
	if got["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("$schema = %v, want draft 2020-12 URI", got["$schema"])
	}

	// Check type
	if got["type"] != "object" {
		t.Errorf("type = %v, want object", got["type"])
	}

	// Check description
	if got["description"] != "Sample pattern schema" {
		t.Errorf("description = %v, want 'Sample pattern schema'", got["description"])
	}

	// Check required
	required, ok := got["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "title" {
		t.Errorf("required = %v, want [title]", got["required"])
	}

	// Check properties exist
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is not a map: %T", got["properties"])
	}

	// Check title property
	titleProp, ok := props["title"].(map[string]any)
	if !ok {
		t.Fatalf("properties.title is not a map: %T", props["title"])
	}
	if titleProp["type"] != "string" {
		t.Errorf("title.type = %v, want string", titleProp["type"])
	}
	if titleProp["maxLength"] != float64(100) {
		t.Errorf("title.maxLength = %v, want 100", titleProp["maxLength"])
	}

	// Check tags property
	tagsProp, ok := props["tags"].(map[string]any)
	if !ok {
		t.Fatalf("properties.tags is not a map: %T", props["tags"])
	}
	if tagsProp["type"] != "array" {
		t.Errorf("tags.type = %v, want array", tagsProp["type"])
	}
	if tagsProp["minItems"] != float64(1) {
		t.Errorf("tags.minItems = %v, want 1", tagsProp["minItems"])
	}
	if tagsProp["maxItems"] != float64(10) {
		t.Errorf("tags.maxItems = %v, want 10", tagsProp["maxItems"])
	}

	// Round-trip: unmarshal back into Schema
	var s2 Schema
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("round-trip Unmarshal: %v", err)
	}

	data2, err := json.Marshal(&s2)
	if err != nil {
		t.Fatalf("round-trip Marshal: %v", err)
	}

	// Compare normalized JSON
	var m1, m2 map[string]any
	json.Unmarshal(data, &m1)
	json.Unmarshal(data2, &m2)

	j1, _ := json.Marshal(m1)
	j2, _ := json.Marshal(m2)
	if string(j1) != string(j2) {
		t.Errorf("round-trip mismatch:\n  original: %s\n  got:      %s", j1, j2)
	}
}

func TestEnumSchema(t *testing.T) {
	s := EnumSchema("red", "green", "blue")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["type"] != "string" {
		t.Errorf("type = %v, want string", got["type"])
	}

	enum, ok := got["enum"].([]any)
	if !ok || len(enum) != 3 {
		t.Fatalf("enum = %v, want [red green blue]", got["enum"])
	}
	want := []string{"red", "green", "blue"}
	for i, v := range enum {
		if v != want[i] {
			t.Errorf("enum[%d] = %v, want %s", i, v, want[i])
		}
	}
}

func TestNumberSchema(t *testing.T) {
	s := NumberSchema(0.5, 100.0)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["type"] != "number" {
		t.Errorf("type = %v, want number", got["type"])
	}
	if got["minimum"] != 0.5 {
		t.Errorf("minimum = %v, want 0.5", got["minimum"])
	}
	if got["maximum"] != float64(100) {
		t.Errorf("maximum = %v, want 100", got["maximum"])
	}
}

func TestIntegerSchema(t *testing.T) {
	s := IntegerSchema(1, 10)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["type"] != "integer" {
		t.Errorf("type = %v, want integer", got["type"])
	}
}

func TestBooleanSchema(t *testing.T) {
	s := BooleanSchema()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["type"] != "boolean" {
		t.Errorf("type = %v, want boolean", got["type"])
	}
}

func TestSchemaWithDefault(t *testing.T) {
	s := StringSchema(0).WithDefault("hello")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["default"] != "hello" {
		t.Errorf("default = %v, want hello", got["default"])
	}
}

func TestSchemaWithAdditionalProperties(t *testing.T) {
	s := ObjectSchema(nil, nil).WithAdditionalProperties(false)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want false", got["additionalProperties"])
	}
}

func TestOmitsZeroConstraints(t *testing.T) {
	// StringSchema with maxLen=0 should omit maxLength
	s := StringSchema(0)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if _, ok := got["maxLength"]; ok {
		t.Errorf("maxLength should be omitted when 0, got %v", got["maxLength"])
	}

	// ArraySchema with min/max=0 should omit both
	s2 := ArraySchema(StringSchema(0), 0, 0)
	data2, err := json.Marshal(s2)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got2 map[string]any
	json.Unmarshal(data2, &got2)

	if _, ok := got2["minItems"]; ok {
		t.Errorf("minItems should be omitted when 0")
	}
	if _, ok := got2["maxItems"]; ok {
		t.Errorf("maxItems should be omitted when 0")
	}
}

func TestSchemaNoSchemaFieldWithoutAsRoot(t *testing.T) {
	s := StringSchema(0)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if _, ok := got["$schema"]; ok {
		t.Error("$schema should be omitted unless AsRoot() is called")
	}
}

func TestRefSchema(t *testing.T) {
	s := RefSchema("cellOverride")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	json.Unmarshal(data, &got)

	if got["$ref"] != "#/$defs/cellOverride" {
		t.Errorf("$ref = %v, want #/$defs/cellOverride", got["$ref"])
	}
	// $ref schemas should not have a type
	if _, ok := got["type"]; ok {
		t.Error("$ref schema should not have type")
	}
}

func TestCellOverridesSchema(t *testing.T) {
	cellOverride := ObjectSchema(
		map[string]*Schema{
			"bold":  BooleanSchema(),
			"color": StringSchema(0),
		},
		nil,
	).WithAdditionalProperties(false)

	root := ObjectSchema(
		map[string]*Schema{
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		nil,
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": cellOverride,
	})

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Check $defs.cellOverride exists
	defs, ok := got["$defs"].(map[string]any)
	if !ok {
		t.Fatal("$defs missing or not an object")
	}
	if _, ok := defs["cellOverride"]; !ok {
		t.Error("$defs.cellOverride missing")
	}

	// Check cell_overrides uses patternProperties
	props := got["properties"].(map[string]any)
	co := props["cell_overrides"].(map[string]any)

	pp, ok := co["patternProperties"].(map[string]any)
	if !ok {
		t.Fatal("cell_overrides.patternProperties missing")
	}
	ref, ok := pp["^[0-9]+$"].(map[string]any)
	if !ok {
		t.Fatal("patternProperties['^[0-9]+$'] missing")
	}
	if ref["$ref"] != "#/$defs/cellOverride" {
		t.Errorf("$ref = %v, want #/$defs/cellOverride", ref["$ref"])
	}

	// additionalProperties should be false
	if co["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want false", co["additionalProperties"])
	}
}

func TestPatternSchemaCompression(t *testing.T) {
	// Verify all registered patterns produce schemas under 6 KB
	// (the old card-grid schema was ~17.5 KB)
	for _, p := range Default().List() {
		s := p.Schema()
		data, err := json.Marshal(s)
		if err != nil {
			t.Errorf("%s: marshal error: %v", p.Name(), err)
			continue
		}
		if len(data) > 6000 {
			t.Errorf("%s: schema too large: %d bytes (max 6000)", p.Name(), len(data))
		}

		// Verify schema contains $defs and patternProperties
		var m map[string]any
		json.Unmarshal(data, &m)
		if _, ok := m["$defs"]; !ok {
			t.Errorf("%s: schema missing $defs", p.Name())
		}
	}
}

func TestSchemaJSONCacheConsistency(t *testing.T) {
	// Verify SchemaJSON returns the same bytes as json.Marshal(p.Schema())
	for _, p := range Default().List() {
		direct, err := json.Marshal(p.Schema())
		if err != nil {
			t.Fatalf("%s: marshal error: %v", p.Name(), err)
		}
		cached := SchemaJSON(p)
		if string(cached) != string(direct) {
			t.Errorf("%s: SchemaJSON mismatch with json.Marshal(Schema())", p.Name())
		}

		// Call again to verify cache hit returns same result
		cached2 := SchemaJSON(p)
		if string(cached2) != string(cached) {
			t.Errorf("%s: SchemaJSON not stable across calls", p.Name())
		}
	}
}

func BenchmarkSchemaJSONDirect(b *testing.B) {
	all := Default().List()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range all {
			json.Marshal(p.Schema()) //nolint:errcheck
		}
	}
}

func BenchmarkSchemaJSONCached(b *testing.B) {
	all := Default().List()
	// Prime cache
	for _, p := range all {
		SchemaJSON(p)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range all {
			SchemaJSON(p)
		}
	}
}
