package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// chartInsightsSlide builds a chart-insights-split slide whose chart carries
// the given raw data JSON (go-slide-creator-yzbo repro shape).
func chartInsightsSlide(t *testing.T, chartData string) SlideInput {
	t.Helper()
	raw := `{
		"layout_id": "content-slide",
		"pattern": {
			"name": "chart-insights-split",
			"values": {
				"chart": {"type": "bar", "title": "Revenue ($M)", "data": ` + chartData + `},
				"insights": ["Revenue grew every quarter", "Q4 was strongest"]
			}
		}
	}`
	var s SlideInput
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal slide: %v", err)
	}
	return s
}

func yzboAnalysis() *types.TemplateAnalysis {
	return &types.TemplateAnalysis{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		Layouts: []types.LayoutMetadata{{
			ID: "content-slide", Name: "Content",
			Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle}},
		}},
	}
}

// generateErr runs the generate-path slide conversion (pattern expansion +
// svggen render of grid diagrams) and returns its error.
func generateErr(slide SlideInput) error {
	_, _, _, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000, nil, nil, "", nil, false)
	return err
}

// TestYzbo_MapFormChartInPatternValidatesAndGenerates: the {label: value}
// chart shorthand (valid for chart_value per examples/charts.json) used to
// pass validate but abort generate inside chart-insights-split. It must now
// be normalized so both agree and generation succeeds.
func TestYzbo_MapFormChartInPatternValidatesAndGenerates(t *testing.T) {
	slide := chartInsightsSlide(t, `{"Q1 2025": 12.0, "Q2 2025": 14.5, "Q3 2025": 15.2, "Q4 2025": 18.0}`)

	out := dryRunOutput{Valid: true}
	validateSlidesAgainstTemplate(&out, []SlideInput{slide}, yzboAnalysis())
	if errs := diagMessages(out.Diagnostics, diagnostics.SeverityError); len(errs) > 0 {
		t.Fatalf("validate reported errors for map-form chart: %v", errs)
	}
	if err := generateErr(slide); err != nil {
		t.Fatalf("generate failed for map-form chart: %v", err)
	}
	for _, f := range collectChartDryRenderFindings(&PresentationInput{Slides: []SlideInput{slide}}, nil, "", "warn") {
		if f.Code == patterns.ErrCodeDiagramRenderFailed {
			t.Errorf("unexpected dry-render failure for normalized chart: %+v", f)
		}
	}
}

// TestYzbo_InvalidPatternChartValidateMatchesGenerate: a chart generate
// rejects must also be rejected by validate, with the same svggen error.
func TestYzbo_InvalidPatternChartValidateMatchesGenerate(t *testing.T) {
	slide := chartInsightsSlide(t, `{"Q1 2025": "twelve", "Q2 2025": [1, 2]}`)

	genErr := generateErr(slide)
	if genErr == nil {
		t.Fatal("expected generate to reject the invalid chart")
	}

	out := dryRunOutput{Valid: true}
	validateSlidesAgainstTemplate(&out, []SlideInput{slide}, yzboAnalysis())
	if out.Valid {
		t.Fatal("validate must mark the deck invalid when generate would abort")
	}
	errs := diagMessages(out.Diagnostics, diagnostics.SeverityError)
	idx := strings.Index(genErr.Error(), "svggen: validation failed")
	if idx < 0 {
		t.Fatalf("generate error has unexpected shape: %v", genErr)
	}
	want := genErr.Error()[idx:]
	matched := false
	for _, e := range errs {
		if strings.Contains(e, want) {
			matched = true
		}
	}
	if !matched {
		t.Errorf("validate errors %v do not carry generate's error %q", errs, want)
	}

	var refuse bool
	for _, f := range collectChartDryRenderFindings(&PresentationInput{Slides: []SlideInput{slide}}, nil, "", "warn") {
		if f.Code == patterns.ErrCodeDiagramRenderFailed && f.Action == "refuse" && strings.HasPrefix(f.Path, "/slides/0/pattern") {
			refuse = true
		}
	}
	if !refuse {
		t.Error("fit report must surface diagram_render_failed (refuse) for the pattern chart")
	}
}

func TestExpandSlidePatternGrid_NoPatternOrExistingGrid(t *testing.T) {
	if g := expandSlidePatternGrid(&SlideInput{}, 0, 0, 0, nil); g != nil {
		t.Errorf("slide without pattern should not expand, got %+v", g)
	}
	s := chartInsightsSlide(t, `{"A": 1}`)
	s.ShapeGrid = &ShapeGridInput{}
	if g := expandSlidePatternGrid(&s, 0, 0, 0, nil); g != nil {
		t.Errorf("slide with explicit shape_grid should not expand pattern")
	}
	s.ShapeGrid = nil
	if g := expandSlidePatternGrid(&s, 0, 0, 0, nil); g == nil {
		t.Errorf("expected pattern expansion with default slide geometry")
	}
}
