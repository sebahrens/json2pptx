package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
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

// go-slide-creator-xvek: validate must run the same pattern value-schema check
// generate runs. For a corpus of mutated pattern values, the two must agree on
// accept/reject and report the same message.
func TestSlidePatternDiagnostics_ValidateGenerateParity(t *testing.T) {
	mustJSON := func(t *testing.T, v string) json.RawMessage {
		t.Helper()
		if !json.Valid([]byte(v)) {
			t.Fatalf("test fixture is not valid JSON: %s", v)
		}
		return json.RawMessage(v)
	}

	tests := []struct {
		name       string
		pattern    string
		values     string
		wantReject bool
		wantSubstr string
	}{
		{
			name:    "kpi-3up valid",
			pattern: "kpi-3up",
			values: `[{"big":"42%","small":"Share of revenue"},
			          {"big":"17","small":"New logos"},
			          {"big":"3x","small":"Pipeline growth"}]`,
		},
		{
			name:    "kpi-3up small over maxLength",
			pattern: "kpi-3up",
			values: `[{"big":"42%","small":"This caption is deliberately far longer than the forty character limit"},
			          {"big":"17","small":"New logos"},
			          {"big":"3x","small":"Pipeline growth"}]`,
			wantReject: true,
			wantSubstr: "maxLength",
		},
		{
			name:       "exec-summary under minItems",
			pattern:    "exec-summary",
			values:     `[{"lead":"Only one point","support":"Not enough for the pattern"}]`,
			wantReject: true,
		},
		{
			name:    "icon-row unknown bundled icon",
			pattern: "icon-row",
			values: `[{"icon":"strategy","caption":"Strategy"},
			          {"icon":"chart-bar","caption":"Growth"},
			          {"icon":"chart-bar","caption":"Margin"}]`,
			wantReject: true,
			wantSubstr: "bundled icon name",
		},
		{
			name:       "icon-row missing required caption",
			pattern:    "icon-row",
			values:     `[{"icon":"rocket"},{"icon":"rocket","caption":"B"},{"icon":"rocket","caption":"C"}]`,
			wantReject: true,
			wantSubstr: "caption",
		},
		{
			name:       "unknown pattern name",
			pattern:    "kpi-9up",
			values:     `[{"big":"1","small":"a"}]`,
			wantReject: true,
			wantSubstr: "unknown pattern",
		},
		{
			name:       "wrong values type",
			pattern:    "kpi-3up",
			values:     `{"big":"42%"}`,
			wantReject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide := SlideInput{
				LayoutID: "blank-title",
				Pattern: &PatternInput{
					Name:   tt.pattern,
					Values: mustJSON(t, tt.values),
				},
			}
			ctx := patterns.ExpandContext{
				SlideWidth:  validationDefaultSlideWidthEMU,
				SlideHeight: validationDefaultSlideHeightEMU,
			}

			// What validate now reports.
			diags := slidePatternDiagnostics(&slide, 0, ctx, patterns.Default())

			// What generate reports for the same slide: the raw expansion error.
			_, _, genErr := expandPattern(slide.Pattern, ctx, patterns.Default())

			if (genErr != nil) != tt.wantReject {
				t.Fatalf("test expectation is stale: generate rejected=%v, want %v (err=%v)", genErr != nil, tt.wantReject, genErr)
			}
			if (len(diags) > 0) != tt.wantReject {
				t.Fatalf("validate rejected=%v, want %v (diags=%+v)", len(diags) > 0, tt.wantReject, diags)
			}
			if !tt.wantReject {
				return
			}

			d := diags[0]
			if d.Severity != diagnostics.SeverityError {
				t.Errorf("severity = %q, want error", d.Severity)
			}
			if d.Code != diagnostics.CodePatternError {
				t.Errorf("code = %q, want %q", d.Code, diagnostics.CodePatternError)
			}
			if d.Path != "/slides/0/pattern" {
				t.Errorf("path = %q, want /slides/0/pattern", d.Path)
			}
			// Parity: validate carries generate's own message verbatim.
			if !strings.Contains(d.Message, genErr.Error()) {
				t.Errorf("validate message %q does not carry generate's error %q", d.Message, genErr.Error())
			}
			if tt.wantSubstr != "" && !strings.Contains(d.Message, tt.wantSubstr) {
				t.Errorf("validate message %q does not mention %q", d.Message, tt.wantSubstr)
			}
		})
	}
}

// slidePatternDiagnostics must not mutate the caller's deck: expanding nested
// cell patterns rewrites cells in place, so an input shape_grid is cloned.
func TestSlidePatternDiagnostics_DoesNotMutateInput(t *testing.T) {
	slide := SlideInput{
		LayoutID: "blank-title",
		ShapeGrid: &ShapeGridInput{
			Rows: []jsonschema.GridRowInput{{
				Cells: []*jsonschema.GridCellInput{{
					Pattern: json.RawMessage(`{"name":"kpi-3up","values":[
						{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}`),
				}},
			}},
		},
	}
	before := string(slide.ShapeGrid.Rows[0].Cells[0].Pattern)

	ctx := patterns.ExpandContext{SlideWidth: validationDefaultSlideWidthEMU, SlideHeight: validationDefaultSlideHeightEMU}
	if d := slidePatternDiagnostics(&slide, 0, ctx, patterns.Default()); len(d) > 0 {
		t.Fatalf("valid nested pattern reported as invalid: %+v", d)
	}

	if got := string(slide.ShapeGrid.Rows[0].Cells[0].Pattern); got != before {
		t.Errorf("input shape_grid was mutated: cell pattern %q, want %q", got, before)
	}
	if slide.ShapeGrid.Rows[0].Cells[0].Grid != nil {
		t.Error("input shape_grid was mutated: nested grid written back onto the caller's cell")
	}
}

// A nested cell pattern with invalid values must be rejected too — generate
// aborts on it, so validate cannot report the deck valid.
func TestSlidePatternDiagnostics_NestedCellPatternRejected(t *testing.T) {
	slide := SlideInput{
		LayoutID: "blank-title",
		ShapeGrid: &ShapeGridInput{
			Rows: []jsonschema.GridRowInput{{
				Cells: []*jsonschema.GridCellInput{{
					Pattern: json.RawMessage(`{"name":"kpi-3up","values":[{"big":"1","small":"a"}]}`),
				}},
			}},
		},
	}
	ctx := patterns.ExpandContext{SlideWidth: validationDefaultSlideWidthEMU, SlideHeight: validationDefaultSlideHeightEMU}
	diags := slidePatternDiagnostics(&slide, 0, ctx, patterns.Default())
	if len(diags) == 0 {
		t.Fatal("nested cell pattern with too few values must be rejected")
	}
	if diags[0].Path != "/slides/0/shape_grid" {
		t.Errorf("path = %q, want /slides/0/shape_grid", diags[0].Path)
	}
}
