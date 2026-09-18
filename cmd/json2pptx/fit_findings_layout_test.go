package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// titleAndBlankOnly mirrors the reported template: only a Title Slide and a
// Blank layout, so nothing can host a content slide.
func titleAndBlankOnly() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			ID:   "slideLayout1",
			Name: "Title Slide",
			Tags: []string{"title-slide"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, MaxChars: 100},
				{ID: "subtitle", Type: types.PlaceholderSubtitle, MaxChars: 80},
			},
			Capacity: types.CapacityEstimate{MaxTextLines: 2, VisualFocused: true},
		},
		{
			ID:           "slideLayout6",
			Name:         "Blank",
			Tags:         []string{"blank"},
			Placeholders: nil,
		},
	}
}

// go-slide-creator-9svyz: with a template containing only Title Slide + Blank,
// validate_input returned valid:true with no findings, and generate then failed
// the whole deck — naming only the first of seven affected slides, and naming
// the INTERNAL coerced slide type ("diagram") rather than what the author wrote.
func TestCollectLayoutResolutionFindings(t *testing.T) {
	kpiPattern := json.RawMessage(`{"name":"kpi-3up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}`)

	input := &PresentationInput{
		Template: "only-title-blank",
		Slides: []SlideInput{
			// Slide 1: a title slide the template CAN host.
			{SlideType: "title", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Programme review")},
			}},
			// Slides 2-4: content slides the template cannot host.
			{SlideType: "content", Pattern: mustPatternInput(t, kpiPattern)},
			{SlideType: "content", ShapeGrid: &ShapeGridInput{
				Columns: json.RawMessage(`1`),
				Rows:    []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}},
			}},
			{SlideType: "content", Pattern: mustPatternInput(t, kpiPattern)},
		},
	}

	findings := collectLayoutResolutionFindings(input, titleAndBlankOnly())

	// EVERY affected slide is reported, not just the first.
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (slides 2-4), got %d: %+v", len(findings), findings)
	}
	wantPaths := []string{"/slides/1", "/slides/2", "/slides/3"}
	for i, f := range findings {
		if f.Path != wantPaths[i] {
			t.Errorf("finding %d path = %q, want %q", i, f.Path, wantPaths[i])
		}
		if f.Code != patterns.ErrCodeLayoutUnresolvable {
			t.Errorf("finding %d code = %q, want %q", i, f.Code, patterns.ErrCodeLayoutUnresolvable)
		}
		if f.Action != "review" {
			t.Errorf("finding %d action = %q, want review", i, f.Action)
		}
		// The AUTHORED slide type, not the internal coerced one.
		if !strings.Contains(f.Message, `"content"`) {
			t.Errorf("finding %d should name the authored slide_type, got: %s", i, f.Message)
		}
		if strings.Contains(f.Message, "diagram") {
			t.Errorf("finding %d leaks the internal coerced slide type: %s", i, f.Message)
		}
		if f.Fix == nil || f.Fix.Kind != "swap_layout" {
			t.Fatalf("finding %d fix = %+v, want kind swap_layout", i, f.Fix)
		}
		cands, ok := f.Fix.Params["candidates"].([]string)
		if !ok || len(cands) != 2 {
			t.Errorf("finding %d should list the template's layouts as candidates, got %v", i, f.Fix.Params["candidates"])
		}
		// Composition slides name the canvas they will land on.
		if got, ok := f.Fix.Params["fallback_layout_id"].(string); !ok || got != "slideLayout6" {
			t.Errorf("finding %d fallback_layout_id = %v, want slideLayout6", i, f.Fix.Params["fallback_layout_id"])
		}
	}
}

// A slide with an explicit layout_id is the author's call and must not be
// second-guessed.
func TestCollectLayoutResolutionFindings_ExplicitLayoutIgnored(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{
		{SlideType: "content", LayoutID: "slideLayout6"},
	}}
	if got := collectLayoutResolutionFindings(input, titleAndBlankOnly()); len(got) != 0 {
		t.Errorf("an explicit layout_id must not be flagged, got %+v", got)
	}
}

// A template that can host the slides must produce nothing.
func TestCollectLayoutResolutionFindings_NoFalsePositives(t *testing.T) {
	layouts := append(titleAndBlankOnly(), types.LayoutMetadata{
		ID:   "slideLayout2",
		Name: "One Content",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body", Type: types.PlaceholderBody, MaxChars: 500,
				Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 8229600, Height: 4525963}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 6, MaxTextLines: 10, TextHeavy: true},
	})

	input := &PresentationInput{Slides: []SlideInput{
		{SlideType: "content", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: strPtr("Findings")},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{"one", "two"}},
		}},
	}}
	if got := collectLayoutResolutionFindings(input, layouts); len(got) != 0 {
		t.Errorf("a template that can host the slide must produce no finding, got %+v", got)
	}
}

// mustPatternInput decodes a pattern JSON fixture.
func mustPatternInput(t *testing.T, raw json.RawMessage) *PatternInput {
	t.Helper()
	var p PatternInput
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("pattern fixture: %v", err)
	}
	return &p
}
