package core

import (
	"encoding/json"
	"fmt"
	"regexp"
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

	// Tolerated marks a documented key the renderer accepts but does not
	// draw (a legacy styling knob, an internal bookkeeping key). Unknown-key
	// validation lets it pass, but leaves it out of the expected keys and
	// did-you-mean suggestions an error offers (go-slide-creator-x9s5i).
	Tolerated bool `json:"-"`
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

// ToleratedDataSchema declares a key the renderer accepts without drawing
// it: documented payloads that carry it still validate, but errors never
// advertise it as an expected key.
func ToleratedDataSchema(desc string) *DataSchema {
	return &DataSchema{Description: desc, Tolerated: true}
}

// EnumDataSchema creates a string enum schema.
func EnumDataSchema(desc string, values ...string) *DataSchema {
	return &DataSchema{Type: "string", Description: desc, Enum: values}
}

// ---------------------------------------------------------------------------
// Unknown-field validation
// ---------------------------------------------------------------------------

// ValidateUnknownFields checks that every key in data is declared by the
// schema, at every level: object properties are checked against the
// property's own schema, and each object element of an array against the
// array's items schema. Keys at a level whose schema leaves
// additionalProperties open (or declares no properties) are not checked.
//
// Each undeclared key is one UNKNOWN_FIELD ValidationError at its path
// ("data.series[0].nme") carrying the keys that level accepts and, when one
// is close, a did-you-mean. The same key repeated across one list's elements
// is reported once with an occurrence count. A misspelled key is never
// drawn, so the payload would render with text or values silently missing
// (go-slide-creator-x9s5i).
func ValidateUnknownFields(data map[string]any, schema *DataSchema, diagramType string) error {
	if schema == nil {
		return nil
	}
	var errs ValidationErrors
	seen := map[string]int{}
	checkUnknownFields(diagramType, "data", data, schema, &errs, seen)
	return errs.AsError()
}

// checkUnknownFields walks value against schema, appending an UNKNOWN_FIELD
// error per undeclared key. seen dedupes a key across one list's elements.
func checkUnknownFields(diagramType, path string, value any, schema *DataSchema, errs *ValidationErrors, seen map[string]int) {
	if schema == nil {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		checkObjectFields(diagramType, path, v, schema, errs, seen)
	case []any:
		for i, el := range v {
			checkUnknownFields(diagramType, fmt.Sprintf("%s[%d]", path, i), el, schema.Items, errs, seen)
		}
	case []map[string]any:
		for i, el := range v {
			checkUnknownFields(diagramType, fmt.Sprintf("%s[%d]", path, i), el, schema.Items, errs, seen)
		}
	}
}

func checkObjectFields(diagramType, path string, obj map[string]any, schema *DataSchema, errs *ValidationErrors, seen map[string]int) {
	if schema.Properties == nil {
		return
	}
	closed := schema.AdditionalProperties != nil && !*schema.AdditionalProperties
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		child, known := schema.Properties[k]
		if known {
			checkUnknownFields(diagramType, path+"."+k, obj[k], child, errs, seen)
			continue
		}
		if !closed {
			continue
		}
		// One error per key per list: series[0..n].nme is one mistake.
		dedupe := listIndexPattern.ReplaceAllString(path, "[]") + "." + k
		if idx, dup := seen[dedupe]; dup {
			errs.Errors[idx].Occurrences++
			errs.Errors[idx].Message = unknownFieldMessage(diagramType, &errs.Errors[idx])
			continue
		}
		expected := expectedFields(schema)
		ve := ValidationError{
			Field:       path + "." + k,
			Code:        ErrCodeUnknownField,
			Value:       k,
			Expected:    expected,
			DidYouMean:  SuggestField(k, expected),
			Occurrences: 1,
		}
		ve.Message = unknownFieldMessage(diagramType, &ve)
		seen[dedupe] = len(errs.Errors)
		errs.Add(ve)
	}
}

var listIndexPattern = regexp.MustCompile(`\[\d+\]`)

func unknownFieldMessage(diagramType string, ve *ValidationError) string {
	key, _ := ve.Value.(string)
	msg := fmt.Sprintf("%s does not accept field %q — it is never drawn; allowed fields: %s", diagramType, key, strings.Join(ve.Expected, ", "))
	if ve.Occurrences > 1 {
		msg += fmt.Sprintf(" (%d list elements carry it)", ve.Occurrences)
	}
	if ve.DidYouMean != "" {
		msg += fmt.Sprintf("; did you mean %q?", ve.DidYouMean)
	}
	return msg
}

// expectedFields returns the sorted keys a schema level advertises: its
// properties minus the tolerated ones.
func expectedFields(schema *DataSchema) []string {
	names := make([]string, 0, len(schema.Properties))
	for k, p := range schema.Properties {
		if p != nil && p.Tolerated {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// fieldSynonyms are the spellings agents reach for that edit distance cannot
// connect to the key a renderer reads ("when" is four edits from "date"). The
// first synonym the level accepts is suggested.
var fieldSynonyms = map[string][]string{
	"title":       {"label", "name"},
	"name":        {"label", "title"},
	"label":       {"name", "title"},
	"text":        {"label", "name", "title", "description"},
	"heading":     {"label", "name", "title"},
	"header":      {"label", "name", "title"},
	"caption":     {"label", "name", "title"},
	"role":        {"title"},
	"position":    {"title"},
	"when":        {"date", "start", "start_date"},
	"time":        {"date", "start"},
	"from":        {"start", "start_date"},
	"to":          {"end", "end_date"},
	"until":       {"end", "end_date"},
	"data":        {"values", "points"},
	"value":       {"values"},
	"amount":      {"value", "values"},
	"count":       {"value"},
	"bullets":     {"items", "causes"},
	"points":      {"items", "causes"},
	"reasons":     {"causes"},
	"children":    {"items", "nodes"},
	"subitems":    {"children", "items"},
	"desc":        {"description"},
	"details":     {"description", "items"},
	"body":        {"description"},
	"events":      {"activities", "milestones"},
	"phases":      {"activities", "tasks"},
	"steps":       {"activities", "tasks", "points"},
	"stages":      {"values", "points"},
	"bars":        {"points"},
	"tasks":       {"activities"},
	"groups":      {"categories"},
	"bones":       {"categories"},
	"causes":      {"categories"},
	"problem":     {"effect"},
	"head":        {"effect"},
	"sets":        {"circles"},
	"overlaps":    {"intersections"},
	"lane":        {"swimlane"},
	"owner":       {"swimlane", "team"},
	"x_axis":      {"x_axis_label", "x_label"},
	"y_axis":      {"y_axis_label", "y_label"},
	"data_points": {"points", "values"},
	"sizes":       {"bubble_values"},
	"size":        {"bubble_values"},
	"items":       {"causes", "children"},
	"category":    {"name", "label"},
	// Venn intersections are keyed by circle letters in order.
	"ba": {"ab"}, "ca": {"ac"}, "cb": {"bc"},
	"acb": {"abc"}, "bac": {"abc"}, "bca": {"abc"}, "cab": {"abc"}, "cba": {"abc"},
}

// SuggestField returns the expected key an unknown key most likely meant, or
// "" when none is close: a synonym the level accepts, then a case- or
// separator-only difference, then the nearest key by edit distance (bounded
// by half the key's length, at most 3, so unrelated keys get no suggestion).
func SuggestField(key string, expected []string) string {
	accepted := make(map[string]bool, len(expected))
	for _, k := range expected {
		accepted[k] = true
	}
	lower := strings.ToLower(key)
	for _, s := range fieldSynonyms[lower] {
		if accepted[s] {
			return s
		}
	}
	normalize := func(s string) string {
		return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(s))
	}
	for _, k := range expected {
		if normalize(k) == normalize(key) {
			return k
		}
	}
	limit := len([]rune(key)) / 2
	if limit > 3 {
		limit = 3
	}
	best, bestDist := "", limit+1
	for _, k := range expected {
		if d := editDistance(lower, strings.ToLower(k)); d < bestDist || (d == bestDist && k < best) {
			best, bestDist = k, d
		}
	}
	if bestDist > limit {
		return ""
	}
	return best
}

func intPtr(v int) *int { return &v }
