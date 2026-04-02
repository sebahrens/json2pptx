package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
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
