package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Engine behavior is tested in internal/rhythm. These tests cover only the
// SlideInput-to-rhythm projection and the MCP adapter's use of that projection.
func TestToRhythmSlide_ProjectsStructureContentAndAccents(t *testing.T) {
	slide := SlideInput{
		SlideType: "comparison",
		Pattern:   &PatternInput{Name: "comparison-2col"},
		Compose:   &ComposeInput{},
		Content:   []ContentInput{{Type: "text"}, {Type: "table"}},
		ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{
			{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)}},
			nil,
			{},
			{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"accent2"`)}},
		}}}},
	}
	got := toRhythmSlide(slide)
	if got.SlideType != "comparison" || got.PatternName != "comparison-2col" ||
		!got.HasPattern || !got.HasShapeGrid || !got.HasCompose {
		t.Errorf("structural projection incorrect: %+v", got)
	}
	if !reflect.DeepEqual(got.ContentKinds, []string{"text", "table"}) {
		t.Errorf("content kinds = %v", got.ContentKinds)
	}
	if got.CellCount != 4 || !reflect.DeepEqual(got.CellAccents, []string{"accent1", "accent2"}) {
		t.Errorf("cell projection: count=%d accents=%v", got.CellCount, got.CellAccents)
	}
}

func TestAnalyzeDeckRhythm_ProjectsContentKindsIntoBreakAdvice(t *testing.T) {
	slides := []SlideInput{
		{Pattern: &PatternInput{Name: "kpi-3up"}},
		{Pattern: &PatternInput{Name: "kpi-4up"}},
		{Pattern: &PatternInput{Name: "kpi-3up"}, Content: []ContentInput{{Type: "chart"}}},
	}
	result := analyzeDeckRhythm(slides)
	if len(result.Recommendations) == 0 || len(result.Recommendations[0].RecommendedBreak) == 0 {
		t.Fatalf("MCP adapter yielded no actionable break: %+v", result.Recommendations)
	}
	if got := result.Recommendations[0].RecommendedBreak[0]; got != "chart-insights-split" {
		t.Errorf("chart content kind was not used to rank break advice: got %q", got)
	}
}

func TestAnalyzeDeckRhythm_ResolvesDTOGridForDensity(t *testing.T) {
	slide := SlideInput{ShapeGrid: &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []GridRowInput{{Cells: []*GridCellInput{
			{Shape: &ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`""`)}},
			{Shape: &ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`""`)}},
		}}},
	}}
	if got := toRhythmSlide(slide); got.Grid == nil {
		t.Fatal("DTO grid was not converted into a resolution-ready grid")
	}
	dd := analyzeDeckRhythm([]SlideInput{slide}).Aggregates.DensityDistribution
	if dd.UnderfilledCells != 2 || dd.OptimalCells != 0 || dd.OverflowCells != 0 {
		t.Errorf("density distribution from converted grid = %+v, want 2 underfilled", dd)
	}
}
