package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-bzh34: a chart-insights-split with a headline, three
// insights, a so-what and a source — inside every published budget — was
// written at 88% autofit (10.6pt) on midnight-blue and refused, while p-style
// accepted it. Its stacked column sized its rows as percentages of an
// estimated area. Validate did not predict it: the readability check skipped
// nested sub-grid cells and shrinks milder than 0.85.
const bzh34ChartInsights = `{
	"chart": {"type": "bar_chart", "data": {"categories": ["2023", "2024", "2025", "2026"],
		"series": [{"name": "Share of enterprises in production", "values": [22, 35, 58, 78]}]}},
	"insights_title": "What drove it",
	"insights": [
		"Packaged copilots removed the build barrier for common tasks.",
		"Evaluation tooling made quality measurable before launch.",
		"Falling inference costs turned marginal cases positive."
	],
	"headline": {"value": "+56 pts", "label": "production adoption 2023 to 2026"},
	"so_what": "The question is no longer whether to adopt, but how fast to scale.",
	"unit": "%",
	"source": "Illustrative estimates for discussion"
}`

func TestChartInsightsStackedColumnReadableOnEveryTemplate(t *testing.T) {
	title := "Production adoption more than doubled in two years"
	for _, tpl := range []string{"abstract", "blue-corporate", "business-template", "forest-green", "midnight-blue", "modern", "modern-template", "modern-yellow", "warm-coral"} {
		t.Run(tpl, func(t *testing.T) {
			a := loadTemplateAnalysis(t, tpl)
			in := &PresentationInput{Slides: []SlideInput{{
				LayoutID: "blank-title",
				Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
				Pattern:  &PatternInput{Name: "chart-insights-split", Values: json.RawMessage(bzh34ChartInsights)},
			}}}
			for _, f := range collectFitFindings(in, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
				if f.Code == patterns.ErrCodeTextBelowReadableMin || f.Code == patterns.ErrCodeBodyTooLong {
					t.Errorf("%s at %s: %s", f.Code, f.Path, f.Message)
				}
			}
		})
	}
}

// The fit report measures nested sub-grid cells, and reports any predicted
// shrink below the floor — generation refuses both.
func TestReadability_NestedSubGridCellPredicted(t *testing.T) {
	long := strings.Repeat("Evaluation tooling made quality measurable before launch. ", 3)
	text, _ := json.Marshal(map[string]any{"paragraphs": []map[string]any{
		{"content": "What drove it", "size": 12, "bold": true},
		{"content": long, "size": 12},
	}})
	nested := &GridCellInput{Grid: &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows:    []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect", Text: text}}}}},
	}}
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`[70, 30]`),
		Rows: []GridRowInput{
			{MinHeight: 90, MaxHeight: 90, Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}, nested}},
			{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}, ColSpan: 2}}},
		},
	}
	in := &PresentationInput{ViewingMode: "present", Slides: []SlideInput{{LayoutID: "blank", ShapeGrid: grid}}}
	got := readabilityCodes(collectReadabilityFindings(in, nil, 12192000, 6858000))
	if len(got) == 0 {
		t.Fatal("expected TEXT_BELOW_READABLE_MIN for the overflowing nested cell")
	}
	if want := "/slides/0/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/shape/text"; got[0].Path != want {
		t.Errorf("path = %q, want %q", got[0].Path, want)
	}
}
