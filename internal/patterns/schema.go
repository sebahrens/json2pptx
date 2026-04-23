package patterns

import "encoding/json"

// Schema holds a hand-authored JSON Schema (draft 2020-12) for a pattern's
// external contract. Used by discovery surfaces (skill-info, MCP show_pattern,
// HTTP GET /patterns/{name}). See design decision D13.
//
// Construct schemas using the helpers: ObjectSchema, ArraySchema,
// StringSchema, NumberSchema, EnumSchema, BooleanSchema, Const, Ref.

// SchemaType represents a JSON Schema type keyword.
type SchemaType string

const (
	TypeObject  SchemaType = "object"
	TypeArray   SchemaType = "array"
	TypeString  SchemaType = "string"
	TypeNumber  SchemaType = "number"
	TypeInteger SchemaType = "integer"
	TypeBoolean SchemaType = "boolean"
)

// schemaJSON is the serialization shape for JSON Schema draft 2020-12.
// We use a private struct so the public API is constructor-based, not
// field-assignment-based.
type schemaJSON struct {
	Schema      string                `json:"$schema,omitempty"`
	Type        SchemaType            `json:"type,omitempty"`
	Description string                `json:"description,omitempty"`
	Properties  map[string]*Schema    `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Items       *Schema               `json:"items,omitempty"`
	MinItems    *int                  `json:"minItems,omitempty"`
	MaxItems    *int                  `json:"maxItems,omitempty"`
	MaxLength   *int                  `json:"maxLength,omitempty"`
	MinLength   *int                  `json:"minLength,omitempty"`
	Minimum     *float64              `json:"minimum,omitempty"`
	Maximum     *float64              `json:"maximum,omitempty"`
	Enum        []string              `json:"enum,omitempty"`
	Const       *json.RawMessage      `json:"const,omitempty"`
	Ref         string                `json:"$ref,omitempty"`
	Defs        map[string]*Schema    `json:"$defs,omitempty"`
	Default     *json.RawMessage      `json:"default,omitempty"`
	AdditionalProperties *bool        `json:"additionalProperties,omitempty"`
}

// MarshalJSON implements json.Marshaler for Schema.
func (s *Schema) MarshalJSON() ([]byte, error) {
	return json.Marshal(&s.raw)
}

// UnmarshalJSON implements json.Unmarshaler for Schema.
func (s *Schema) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.raw)
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// ObjectSchema creates an object schema with the given properties and required
// field names. Pass nil for properties or required if not needed.
func ObjectSchema(properties map[string]*Schema, required []string) *Schema {
	return &Schema{raw: schemaJSON{
		Type:       TypeObject,
		Properties: properties,
		Required:   required,
	}}
}

// ArraySchema creates an array schema. minItems/maxItems of 0 means no
// constraint (omitted from JSON output).
func ArraySchema(items *Schema, minItems, maxItems int) *Schema {
	s := &Schema{raw: schemaJSON{
		Type:  TypeArray,
		Items: items,
	}}
	if minItems > 0 {
		s.raw.MinItems = intPtr(minItems)
	}
	if maxItems > 0 {
		s.raw.MaxItems = intPtr(maxItems)
	}
	return s
}

// StringSchema creates a string schema. maxLen of 0 means no constraint.
func StringSchema(maxLen int) *Schema {
	s := &Schema{raw: schemaJSON{Type: TypeString}}
	if maxLen > 0 {
		s.raw.MaxLength = intPtr(maxLen)
	}
	return s
}

// NumberSchema creates a number schema with optional min/max bounds.
// Pass math.Inf(-1) or math.Inf(1) (or any NaN) for unbounded; however
// the simplest convention is: both 0 means "no constraint" only when
// that's contextually obvious. For explicit control use WithMinimum/WithMaximum.
func NumberSchema(min, max float64) *Schema {
	s := &Schema{raw: schemaJSON{Type: TypeNumber}}
	if min != 0 || max != 0 {
		s.raw.Minimum = float64Ptr(min)
		s.raw.Maximum = float64Ptr(max)
	}
	return s
}

// IntegerSchema creates an integer schema with optional min/max bounds.
func IntegerSchema(min, max float64) *Schema {
	s := &Schema{raw: schemaJSON{Type: TypeInteger}}
	if min != 0 || max != 0 {
		s.raw.Minimum = float64Ptr(min)
		s.raw.Maximum = float64Ptr(max)
	}
	return s
}

// EnumSchema creates a string enum schema with the given allowed values.
func EnumSchema(values ...string) *Schema {
	return &Schema{raw: schemaJSON{
		Type: TypeString,
		Enum: values,
	}}
}

// BooleanSchema creates a boolean schema.
func BooleanSchema() *Schema {
	return &Schema{raw: schemaJSON{Type: TypeBoolean}}
}

// ---------------------------------------------------------------------------
// Fluent modifiers — return *Schema for chaining
// ---------------------------------------------------------------------------

// WithDescription sets the "description" keyword.
func (s *Schema) WithDescription(desc string) *Schema {
	s.raw.Description = desc
	return s
}

// WithDefault sets the "default" keyword from any JSON-serializable value.
func (s *Schema) WithDefault(v any) *Schema {
	data, err := json.Marshal(v)
	if err == nil {
		raw := json.RawMessage(data)
		s.raw.Default = &raw
	}
	return s
}

// WithDefs sets the "$defs" keyword for local schema definitions.
func (s *Schema) WithDefs(defs map[string]*Schema) *Schema {
	s.raw.Defs = defs
	return s
}

// WithAdditionalProperties sets "additionalProperties" to the given value.
func (s *Schema) WithAdditionalProperties(allowed bool) *Schema {
	s.raw.AdditionalProperties = boolPtr(allowed)
	return s
}

// AsRoot stamps "$schema": "https://json-schema.org/draft/2020-12/schema"
// onto the schema, marking it as a root schema document.
func (s *Schema) AsRoot() *Schema {
	s.raw.Schema = "https://json-schema.org/draft/2020-12/schema"
	return s
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intPtr(v int) *int          { return &v }
func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool       { return &v }
