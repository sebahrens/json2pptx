package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestValidateSlidesAgainstTemplate_ChartDiagramSvggen verifies that
// validateSlidesAgainstTemplate dispatches chart/diagram content items to
// svggen Validate() and surfaces structural validation warnings.
func TestValidateSlidesAgainstTemplate_ChartDiagramSvggen(t *testing.T) {
	// Minimal template analysis with one layout containing a content placeholder.
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "content-slide",
				Name: "Content Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "content", Type: types.PlaceholderContent, MaxChars: 0},
				},
			},
		},
	}

	t.Run("diagram with invalid waterfall data produces svggen warning", func(t *testing.T) {
		// Waterfall diagram with flat map (no "points" array) should fail
		// svggen validation since diagram type passes data directly without
		// auto-conversion.
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		found := false
		for _, w := range output.Warnings {
			if strings.Contains(w, "waterfall") && strings.Contains(w, "data validation") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected svggen validation warning for waterfall diagram with flat map data, got warnings: %v", output.Warnings)
		}
	})

	t.Run("chart with flat waterfall data emits conversion warning", func(t *testing.T) {
		// Chart type auto-converts flat maps via buildChartData, which should
		// produce a flat-map conversion warning but not a validation error.
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		foundConversion := false
		for _, w := range output.Warnings {
			if strings.Contains(w, "flat data") {
				foundConversion = true
				break
			}
		}
		if !foundConversion {
			t.Errorf("expected flat-map conversion warning for waterfall chart, got warnings: %v", output.Warnings)
		}
	})

	t.Run("chart with valid bar data produces no svggen warning", func(t *testing.T) {
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20,"Q3":30}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range output.Warnings {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected svggen validation warning for valid bar chart: %s", w)
			}
		}
	})

	t.Run("diagram with valid waterfall data produces no warning", func(t *testing.T) {
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value: json.RawMessage(`{
							"type":"waterfall",
							"data":{
								"points":[
									{"label":"Revenue","value":100,"type":"increase"},
									{"label":"Costs","value":-40,"type":"decrease"}
								]
							}
						}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range output.Warnings {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected svggen validation warning for valid waterfall diagram: %s", w)
			}
		}
	})

	t.Run("aggregate counts populated for mixed content", func(t *testing.T) {
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{PlaceholderID: "content", Type: "chart", Value: json.RawMessage(`{"type":"bar","data":{"Q1":10}}`)},
					{PlaceholderID: "content", Type: "diagram", Value: json.RawMessage(`{"type":"timeline","data":{"events":[{"label":"A","date":"2026"}]}}`)},
					{PlaceholderID: "content", Type: "table", Value: json.RawMessage(`{"headers":["A"],"rows":[["1"]]}`)},
					{PlaceholderID: "content", Type: "text", TextValue: strPtr("hello")},
				},
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{Cells: []*GridCellInput{
							{Shape: &ShapeSpecInput{Geometry: "rect"}},
							{Shape: &ShapeSpecInput{Geometry: "ellipse"}},
						}},
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.SlideCount != 1 {
			t.Errorf("SlideCount: got %d, want 1", output.SlideCount)
		}
		if output.ChartCount != 1 {
			t.Errorf("ChartCount: got %d, want 1", output.ChartCount)
		}
		if output.DiagramCount != 1 {
			t.Errorf("DiagramCount: got %d, want 1", output.DiagramCount)
		}
		if output.TableCount != 1 {
			t.Errorf("TableCount: got %d, want 1", output.TableCount)
		}
		if output.ShapeCount != 2 {
			t.Errorf("ShapeCount: got %d, want 2", output.ShapeCount)
		}
	})

	t.Run("text content is not chart-validated", func(t *testing.T) {
		output := dryRunOutput{
			Valid:    true,
			Warnings: []string{},
			Slides:   []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "text",
						TextValue:     strPtr("Hello world"),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range output.Warnings {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected chart/diagram validation warning for text content: %s", w)
			}
		}
	})
}

// TestValidateSlidesAgainstTemplate_UnknownLayoutID verifies that an unknown
// layout_id produces an error (not a warning), sets Valid=false, and includes
// a structured ValidationError with code "unknown_layout_id" and a did_you_mean
// suggestion when the typo is close.
func TestValidateSlidesAgainstTemplate_UnknownLayoutID(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{ID: "content-slide", Name: "Content Slide"},
			{ID: "title-slide", Name: "Title Slide"},
			{ID: "section-header", Name: "Section Header"},
		},
	}

	t.Run("typo layout_id is error with did_you_mean", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Warnings: []string{}, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "conten-slide"}} // typo
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected Valid=false for unknown layout_id, got true")
		}
		if len(output.Errors) == 0 {
			t.Fatal("expected at least one error for unknown layout_id")
		}
		if !strings.Contains(output.Errors[0], "not found") {
			t.Errorf("error should mention 'not found': %s", output.Errors[0])
		}
		// Check structured ValidationError
		if len(output.ValidationWarnings) == 0 {
			t.Fatal("expected a ValidationError for unknown layout_id")
		}
		ve := output.ValidationWarnings[0]
		if ve.Code != patterns.ErrCodeUnknownLayoutID {
			t.Errorf("expected code %q, got %q", patterns.ErrCodeUnknownLayoutID, ve.Code)
		}
		if ve.Path != "slides[0].layout_id" {
			t.Errorf("expected path slides[0].layout_id, got %q", ve.Path)
		}
		if ve.Fix == nil {
			t.Fatal("expected fix suggestion")
		}
		if ve.Fix.Kind != "use_one_of" {
			t.Errorf("expected fix kind 'use_one_of', got %q", ve.Fix.Kind)
		}
		dym, ok := ve.Fix.Params["did_you_mean"].(string)
		if !ok || dym != "content-slide" {
			t.Errorf("expected did_you_mean='content-slide', got %v", ve.Fix.Params["did_you_mean"])
		}
	})

	t.Run("completely wrong layout_id is error without did_you_mean", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Warnings: []string{}, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "zzz-nonexistent-zzz"}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected Valid=false for unknown layout_id")
		}
		if len(output.ValidationWarnings) == 0 {
			t.Fatal("expected a ValidationError")
		}
		ve := output.ValidationWarnings[0]
		if ve.Fix == nil {
			t.Fatal("expected fix suggestion")
		}
		if _, ok := ve.Fix.Params["did_you_mean"]; ok {
			t.Error("did not expect did_you_mean for completely wrong layout_id")
		}
	})

	t.Run("valid layout_id produces no error", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Warnings: []string{}, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "content-slide"}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if !output.Valid {
			t.Error("expected Valid=true for valid layout_id")
		}
		if len(output.Errors) > 0 {
			t.Errorf("unexpected errors: %v", output.Errors)
		}
	})
}

func TestValidateShapeFillColor_HexWarning(t *testing.T) {
	tests := []struct {
		name        string
		color       string
		wantWarning bool
	}{
		{"scheme name accent1 — no warning", "accent1", false},
		{"scheme name dk1 — no warning", "dk1", false},
		{"allowlisted black — no warning", "#000000", false},
		{"allowlisted white — no warning", "#FFFFFF", false},
		{"allowlisted white lowercase — no warning", "#ffffff", false},
		{"allowlisted short black — no warning", "#000", false},
		{"allowlisted short white — no warning", "#fff", false},
		{"non-allowlisted hex — warning", "#65686B", true},
		{"non-allowlisted hex lowercase — warning", "#65686b", true},
		{"non-allowlisted short hex — warning", "#abc", true},
		{"empty — no warning", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.color)
			var warnings []string
			valWarnings := validateShapeFillColor(json.RawMessage(raw), 1, 1, 1, &warnings)
			got := len(valWarnings) > 0
			if got != tt.wantWarning {
				t.Errorf("color %q: got warning=%v, want %v (valWarnings=%v)", tt.color, got, tt.wantWarning, valWarnings)
			}
			if tt.wantWarning && len(valWarnings) > 0 {
				if valWarnings[0].Code != patterns.ErrCodeHexFillNonBrand {
					t.Errorf("expected code %q, got %q", patterns.ErrCodeHexFillNonBrand, valWarnings[0].Code)
				}
			}
		})
	}
}
