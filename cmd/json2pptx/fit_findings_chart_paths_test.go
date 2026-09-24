package main

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestChartFindingPathsResolveAgainstAuthoredInput(t *testing.T) {
	diagram := crowdedNominalDiagram()
	input := &PresentationInput{Slides: []SlideInput{
		{Content: []ContentInput{{Type: "diagram", DiagramValue: diagram}}},
		{ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Diagram: diagram}}}}}},
		{Pattern: &PatternInput{Name: "chart-insights-split"}},
	}}
	findings := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/0/diagram_value/data/series/0/values"}},
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/0/diagram_value/x_axis/labels"}},
		{ValidationError: patterns.ValidationError{Path: "/slides/1/shape_grid/rows/0/cells/0/diagram/data/categories"}},
		{ValidationError: patterns.ValidationError{Path: "/slides/2/pattern/rows/0/cells/0/diagram/data"}},
	}
	got := chartFindingsAtAuthoredPaths(input, findings)
	want := []string{
		"/slides/0/content/0/diagram_value/data/series/0/values",
		"/slides/0/content/0/diagram_value",
		"/slides/1/shape_grid/rows/0/cells/0/diagram/data/categories",
		"/slides/2/pattern",
	}
	for i, finding := range got {
		if finding.Path != want[i] {
			t.Errorf("finding %d path = %q, want %q", i, finding.Path, want[i])
		}
		assertFindingPointerReplaceable(t, input, finding.Path)
	}
}

func TestChartFindingPathsFallBackWhenInputCannotMarshal(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{
		Type: "diagram", DiagramValue: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"value": math.NaN()}},
	}}}}}
	got := chartFindingsAtAuthoredPaths(input, []patterns.FitFinding{{ValidationError: patterns.ValidationError{
		Path: "/slides/0/content/0/diagram_value/data/value",
	}}})
	if len(got) != 1 || got[0].Path != "/slides/0" {
		t.Fatalf("unserializable input should fall back to slide root: %+v", got)
	}
}

func TestEmittedChartFindingPathsAreReplaceable(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{
		Content:   []ContentInput{{Type: "diagram", PlaceholderID: "body", DiagramValue: crowdedNominalDiagram()}},
		ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Diagram: crowdedNominalDiagram()}}}}},
	}}}
	findings := collectChartDryRenderFindingsWithConverter(input, nil, "", "warn", true)
	if len(findings) == 0 {
		t.Fatal("stress deck emitted no chart findings")
	}
	for _, finding := range findings {
		assertFindingPointerReplaceable(t, input, finding.Path)
	}
}

func assertFindingPointerReplaceable(t *testing.T, input *PresentationInput, path string) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if err := setJSONPointer(document, path, "probe"); err != nil {
		t.Errorf("finding path %q cannot be used as a replace-field JSON Pointer: %v", path, err)
	}
}
