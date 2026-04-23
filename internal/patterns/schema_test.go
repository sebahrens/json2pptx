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
