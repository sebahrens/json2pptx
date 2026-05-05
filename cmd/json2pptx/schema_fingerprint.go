package main

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// schemaFingerprint computes a stable hash over the contract-visible surfaces
// that agents rely on: PresentationInput struct fields (json tags), MCP tool
// names, and Fix.Kind values. If ANY of these change, the fingerprint changes,
// signalling that SchemaVersion must be bumped.
func schemaFingerprint() string {
	h := sha256.New()

	// 1. PresentationInput and key nested struct json tags.
	writeStructFields(h, reflect.TypeOf(PresentationInput{}))
	writeStructFields(h, reflect.TypeOf(SlideInput{}))
	writeStructFields(h, reflect.TypeOf(ContentInput{}))
	writeStructFields(h, reflect.TypeOf(DefaultsInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.ShapeGridInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.GridCellInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.TableInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.TableCellInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.TableStyleInput{}))
	writeStructFields(h, reflect.TypeOf(jsonschema.ShapeSpecInput{}))
	writeStructFields(h, reflect.TypeOf(types.ChartSpec{})) //nolint:staticcheck // ChartSpec is deprecated but still part of the input contract
	writeStructFields(h, reflect.TypeOf(types.DiagramSpec{}))
	writeStructFields(h, reflect.TypeOf(PatternInput{}))

	// 2. MCP tool names (sorted).
	tools := mcpToolNames()
	for _, t := range tools {
		h.Write([]byte("tool:" + t + "\n"))
	}

	// 3. Fix.Kind vocabulary (sorted).
	kinds := fixKindVocabulary()
	for _, k := range kinds {
		h.Write([]byte("fix_kind:" + k + "\n"))
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// writeStructFields writes the json tags of a struct type into the hash.
func writeStructFields(h interface{ Write([]byte) (int, error) }, t reflect.Type) {
	_, _ = h.Write([]byte("struct:" + t.Name() + "\n"))
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		// Normalize: strip omitempty for stability comparison.
		name := strings.Split(tag, ",")[0]
		_, _ = h.Write([]byte("  " + name + "\n"))
	}
}

// fixKindVocabulary returns the sorted list of Fix.Kind values that the
// repair_slide tool and fit-findings system recognize. Keep this in sync
// with the switch in handleRepairSlide and the finding emitters.
func fixKindVocabulary() []string {
	kinds := []string{
		"reduce_text",
		"shorten_title",
		"split_at_row",
		"swap_layout",
		"use_one_of",
		"use_semantic_color",
		"rewrite_field",
		"truncation_summary",
		"replace_color",
		"rename_field",
		"replace_value",
		"reposition_shape",
	}
	sort.Strings(kinds)
	return kinds
}
