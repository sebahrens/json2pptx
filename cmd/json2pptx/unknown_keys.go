package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ---------------------------------------------------------------------------
// Unknown-key detection for JSON input structs.
//
// Walk raw JSON and detect keys not present in the struct's json tags. For
// each unknown key, emit a ValidationError with code "unknown_key" and a
// did-you-mean Fix suggestion using Levenshtein distance.
//
// This enforces "additionalProperties: false" semantics at every object
// level of the input schema, catching agent typos like "slide_tpye" or
// "accennt1" before they silently become no-ops.
// ---------------------------------------------------------------------------

// jsonFieldNames extracts the set of JSON property names from a struct type's
// tags. Nested/anonymous fields are flattened. The result is sorted.
func jsonFieldNames(t reflect.Type) []string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var names []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			// Anonymous struct → flatten.
			if f.Anonymous {
				names = append(names, jsonFieldNames(f.Type)...)
			}
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// checkUnknownKeys parses raw as a JSON object and reports any keys not in
// knownKeys. For each unknown key a patterns.ValidationError with code
// "unknown_key" is returned, including a did-you-mean FixSuggestion when a
// close match exists.
func checkUnknownKeys(raw json.RawMessage, knownKeys []string, path string) []*patterns.ValidationError {
	if len(raw) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil // not an object — nothing to check
	}

	known := make(map[string]bool, len(knownKeys))
	for _, k := range knownKeys {
		known[k] = true
	}

	var errs []*patterns.ValidationError
	for key := range obj {
		if known[key] {
			continue
		}
		fieldPath := path + "." + key
		if path == "" {
			fieldPath = key
		}

		ve := &patterns.ValidationError{
			Pattern: "input",
			Path:    fieldPath,
			Code:    patterns.ErrCodeUnknownKey,
			Message: fmt.Sprintf("unknown field %q at %s", key, fieldPath),
		}

		// Did-you-mean suggestion via Levenshtein distance.
		const maxDist = 3
		if match, dist := generator.ClosestMatch(key, knownKeys, maxDist); dist >= 0 {
			ve.Message = fmt.Sprintf("unknown field %q at %s (did you mean %q?)", key, fieldPath, match)
			ve.Fix = &patterns.FixSuggestion{
				Kind: "rename_field",
				Params: map[string]any{
					"from": key,
					"to":   match,
				},
			}
		}

		errs = append(errs, ve)
	}
	return errs
}

// checkUnknownKeysForType is a convenience wrapper: extracts known keys from a
// struct type via reflection and delegates to checkUnknownKeys.
func checkUnknownKeysForType(raw json.RawMessage, structType reflect.Type, path string) []*patterns.ValidationError {
	return checkUnknownKeys(raw, jsonFieldNames(structType), path)
}

