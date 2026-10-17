package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestMCPPatternSegmentFitDiscovery(t *testing.T) {
	showSchema := successSchemaBranch(t, mcpShowPatternTool().RawOutputSchema)["properties"].(map[string]any)
	listSchema := successSchemaBranch(t, mcpListPatternsTool().RawOutputSchema)["properties"].(map[string]any)
	groupSchema := listSchema["groups"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	entrySchema := groupSchema["patterns"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"min_content_width_pt", "min_content_height_pt", "narrow_fallback", "segment_only", "fallback_below_width_pt", "min_label_column_width_pt", "reference_value_digit_budget", "fit_notes"} {
		if showSchema[field] == nil || !reflect.DeepEqual(showSchema[field], entrySchema[field]) {
			t.Errorf("show/list output schemas must advertise the same %q field", field)
		}
	}
	fullResult, err := handleListPatterns(context.Background(), makeRequest(map[string]any{"fields": "full", "page_size": float64(500)}))
	if err != nil || fullResult.IsError {
		t.Fatalf("list_patterns: %v, %+v", err, fullResult)
	}
	full := fullResult.StructuredContent.(listPatternsResponse)
	entries := map[string]skillPatternCompact{}
	for _, group := range full.Groups {
		for _, entry := range group.Patterns {
			entries[entry.Name] = entry
			if entry.SegmentFit == nil || entry.NarrowFallback == "" {
				t.Errorf("full entry %q has no segment-fit contract", entry.Name)
			}
		}
	}
	for _, tc := range []struct {
		name, fallback                string
		segmentOnly                   bool
		minWidth, threshold, labelMin float64
		digits                        int
	}{
		{"cycle-figure-eight", "refused", false, 580, 0, 0, 0},
		{"cycle-ring", "legend", false, 0, 450, 0, 0},
		{"cycle-nodes", "legend", false, 0, 0, 120, 0},
		{"radial-hub", "legend", false, 0, 0, 110, 0},
		{"cycle-intake", "stacked", false, 0, 0, 96, 0},
		{"kpi-inline", "none", true, 0, 0, 0, 0},
		{"before-after-compact", "none", true, 0, 0, 0, 0},
		{"process-flow-compact", "none", true, 0, 0, 0, 0},
		{"kpi-5up", "none", false, 0, 0, 0, 11},
		{"kpi-6up", "none", false, 0, 0, 0, 9},
		{"card-grid", "none", false, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{"name": tc.name}))
			if err != nil || result.IsError {
				t.Fatalf("show_pattern: %v, %+v", err, result)
			}
			show := result.StructuredContent.(skillPatternFull)
			fit := show.SegmentFit
			if fit == nil {
				t.Fatal("show_pattern has no fit metadata")
			}
			if fit.NarrowFallback != tc.fallback || fit.SegmentOnly != tc.segmentOnly || fit.MinContentWidthPt != tc.minWidth || fit.FallbackBelowWidthPt != tc.threshold || fit.MinLabelColumnWidthPt != tc.labelMin || fit.ReferenceValueDigitBudget != tc.digits {
				t.Errorf("unexpected fit metadata: %+v", fit)
			}
			if !reflect.DeepEqual(fit, entries[tc.name].SegmentFit) {
				t.Errorf("show and list full disagree: %+v / %+v", fit, entries[tc.name].SegmentFit)
			}
			// Verify wire-level flattening and explicit false; callers must not
			// need knowledge of Go's anonymous embedded struct to read this.
			wire, _ := json.Marshal(show)
			var fields map[string]any
			if err := json.Unmarshal(wire, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["narrow_fallback"] != tc.fallback || fields["segment_only"] != tc.segmentOnly {
				t.Errorf("missing flat placement fields: %s", wire)
			}
			if tc.minWidth == 0 && fields["min_content_width_pt"] != nil {
				t.Error("absence of a known hard minimum must not be advertised as a zero-width fit guarantee")
			}
		})
	}
	compactResult, err := handleListPatterns(context.Background(), makeRequest(map[string]any{"page_size": float64(500)}))
	if err != nil || compactResult.IsError {
		t.Fatalf("compact list: %v, %+v", err, compactResult)
	}
	for _, group := range compactResult.StructuredContent.(listPatternsResponse).Groups {
		for _, entry := range group.Patterns {
			wire, _ := json.Marshal(entry)
			var fields map[string]any
			if err := json.Unmarshal(wire, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["narrow_fallback"]; ok {
				t.Errorf("compact listing unexpectedly carries fit metadata: %s", wire)
			}
		}
	}
	_, skillFull := buildPatternEntries("full")
	for _, entry := range skillFull {
		p, _ := patterns.Default().Get(entry.Name)
		if !reflect.DeepEqual(entry.SegmentFit, patterns.SegmentFitFor(p)) {
			t.Errorf("skill-info full disagrees for %s", entry.Name)
		}
	}
}

func TestRecommendVisualSegmentFitSchema(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(outputSchemaRecommendVisual, &schema); err != nil {
		t.Fatal(err)
	}
	defs := schema["$defs"].(map[string]any)
	fit := defs["segment_fit"].(map[string]any)
	showProperties := successSchemaBranch(t, mcpShowPatternTool().RawOutputSchema)["properties"].(map[string]any)
	for field, definition := range fit["properties"].(map[string]any) {
		if !reflect.DeepEqual(definition, showProperties[field]) {
			t.Errorf("recommend_visual disagrees with show_pattern for fit field %q", field)
		}
	}
	properties := schema["properties"].(map[string]any)
	candidate := properties["candidates"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	composition := defs["visual_composition"].(map[string]any)["properties"].(map[string]any)
	region := composition["regions"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	for _, entry := range []map[string]any{candidate, region} {
		if entry["segment_fit"].(map[string]any)["$ref"] != "#/$defs/segment_fit" {
			t.Error("named candidates and composition leaves must advertise the shared fit contract")
		}
	}
	if region["compose"].(map[string]any)["$ref"] != "#/$defs/visual_composition" {
		t.Error("nested compositions must retain leaf fit metadata in their schema")
	}
}
