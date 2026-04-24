package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DataSchema is a lightweight JSON Schema representation for diagram data
// payloads. It captures the allowed fields and their types so that unknown
// keys can be rejected at validate-time with structured errors.
//
// Only the subset of JSON Schema needed for svggen data validation is
// modeled here — this is not a general-purpose JSON Schema library.
type DataSchema struct {
	// Type is the JSON Schema type (always "object" for data schemas).
	Type string `json:"type"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty"`

	// Properties maps field names to their sub-schemas.
	Properties map[string]*DataSchema `json:"properties,omitempty"`

	// Required lists required property names.
	Required []string `json:"required,omitempty"`

	// AdditionalProperties when false rejects unknown keys.
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	// Items is the schema for array elements.
	Items *DataSchema `json:"items,omitempty"`

	// Enum lists allowed string values.
	Enum []string `json:"enum,omitempty"`

	// MinItems is the minimum array length.
	MinItems *int `json:"minItems,omitempty"`

	// Minimum is the minimum numeric value.
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum is the maximum numeric value.
	Maximum *float64 `json:"maximum,omitempty"`
}

// MarshalJSON produces clean JSON output.
func (s *DataSchema) MarshalJSON() ([]byte, error) {
	// Use an alias to avoid infinite recursion.
	type Alias DataSchema
	return json.Marshal((*Alias)(s))
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// ObjectDataSchema creates an object schema with the given properties and
// required fields, with additionalProperties set to false.
func ObjectDataSchema(desc string, props map[string]*DataSchema, required []string) *DataSchema {
	f := false
	return &DataSchema{
		Type:                 "object",
		Description:          desc,
		Properties:           props,
		Required:             required,
		AdditionalProperties: &f,
	}
}

// ArrayDataSchema creates an array schema with the given items schema.
func ArrayDataSchema(desc string, items *DataSchema, minItems int) *DataSchema {
	s := &DataSchema{
		Type:        "array",
		Description: desc,
		Items:       items,
	}
	if minItems > 0 {
		s.MinItems = intPtr(minItems)
	}
	return s
}

// StringDataSchema creates a string schema.
func StringDataSchema(desc string) *DataSchema {
	return &DataSchema{Type: "string", Description: desc}
}

// NumberDataSchema creates a number schema.
func NumberDataSchema(desc string) *DataSchema {
	return &DataSchema{Type: "number", Description: desc}
}

// BooleanDataSchema creates a boolean schema.
func BooleanDataSchema(desc string) *DataSchema {
	return &DataSchema{Type: "boolean", Description: desc}
}

// EnumDataSchema creates a string enum schema.
func EnumDataSchema(desc string, values ...string) *DataSchema {
	return &DataSchema{Type: "string", Description: desc, Enum: values}
}

// ---------------------------------------------------------------------------
// Unknown-field validation
// ---------------------------------------------------------------------------

// ValidateUnknownFields checks that all keys in data are declared in
// the schema's Properties map. Returns UNKNOWN_FIELD ValidationErrors
// for any undeclared keys. Only checks the top-level keys (not nested).
func ValidateUnknownFields(data map[string]any, schema *DataSchema, diagramType string) error {
	if schema == nil || schema.Properties == nil {
		return nil
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		return nil
	}

	var errs ValidationErrors
	// Sort keys for deterministic error ordering.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, ok := schema.Properties[k]; !ok {
			allowed := allowedFieldList(schema)
			errs.Add(ValidationError{
				Field:   fmt.Sprintf("data.%s", k),
				Code:    ErrCodeUnknownField,
				Message: fmt.Sprintf("%s does not accept field %q; allowed fields: %s", diagramType, k, allowed),
				Value:   k,
			})
		}
	}
	return errs.AsError()
}

// allowedFieldList returns a sorted, comma-separated list of allowed field
// names from a schema's Properties.
func allowedFieldList(schema *DataSchema) string {
	names := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func intPtr(v int) *int { return &v }
