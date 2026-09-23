package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// crowdedNominalDiagram builds a 40-category bar chart at a render width
// where every named category cannot fit even vertically. Shared by the
// dry-render shape_grid tests below.
func crowdedNominalDiagram() *types.DiagramSpec {
	const n = 40
	cats := make([]any, n)
	vals := make([]any, n)
	for i := 0; i < n; i++ {
		cats[i] = "Category " + string(rune('A'+i%26)) + string(rune('0'+i/26))
		vals[i] = float64(i + 1)
	}
	return &types.DiagramSpec{
		Type:   "bar_chart",
		Width:  400, // narrow to force tick label collisions
		Height: 300,
		Data: map[string]any{
			"categories": cats,
			"series": []any{
				map[string]any{"name": "Series A", "values": vals},
			},
		},
	}
}

// TestCollectChartDryRenderFindings_CrowdedNominal verifies that a 40-category
// bar chart at a narrow render width surfaces actionable crowding via the
// dry-render path, closing the validate → preview → generate feedback loop
// (acceptance criterion 6 of bead go-slide-creator-0ywh).
func TestCollectChartDryRenderFindings_CrowdedNominal(t *testing.T) {
	const n = 40
	cats := make([]any, n)
	vals := make([]any, n)
	for i := 0; i < n; i++ {
		cats[i] = "Category " + string(rune('A'+i%26)) + string(rune('0'+i/26))
		vals[i] = float64(i + 1)
	}

	input := &PresentationInput{
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type:          "diagram",
						PlaceholderID: "body",
						DiagramValue: &types.DiagramSpec{
							Type:   "bar_chart",
							Width:  400, // narrow to force tick label collisions
							Height: 300,
							Data: map[string]any{
								"categories": cats,
								"series": []any{
									map[string]any{"name": "Series A", "values": vals},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding for 40-cat narrow bar chart, got none")
	}

	var sawCrowding bool
	for _, f := range findings {
		if f.Code == "chart.tick_thinned" && strings.Contains(f.Path, "x_axis.labels") {
			t.Errorf("nominal categories must not be thinned: %+v", f)
		}
		if f.Code == "chart.capacity_exceeded" {
			sawCrowding = true
			if !strings.Contains(f.Path, "slides[0].content[0].diagram_value") {
				t.Errorf("capacity_exceeded path = %q; want prefix slides[0].content[0].diagram_value", f.Path)
			}
			if f.Action != "shrink_or_split" {
				t.Errorf("capacity_exceeded action = %q, want shrink_or_split", f.Action)
			}
		}
	}
	if !sawCrowding {
		// Helpful diagnostics when the geometry changes upstream.
		codes := make([]string, 0, len(findings))
		for _, f := range findings {
			codes = append(codes, f.Code)
		}
		t.Errorf("expected chart.capacity_exceeded in dry-render findings; got codes=%v", codes)
	}
}

func TestCollectChartDryRenderFindings_EllipsizedCategoryAction(t *testing.T) {
	categories := make([]any, 14)
	values := make([]any, 14)
	for i := range categories {
		categories[i] = fmt.Sprintf("Category number %d with a long name", i+1)
		values[i] = float64(i + 1)
	}
	input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{
		Type: "diagram", DiagramValue: &types.DiagramSpec{
			Type: "bar_chart", Width: 800, Height: 360,
			Data: map[string]any{"categories": categories, "series": []any{map[string]any{"name": "S", "values": values}}},
		},
	}}}}}
	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	for _, f := range findings {
		if f.Code == "chart.label_ellipsized" && f.Action == "shrink_or_split" {
			return
		}
	}
	t.Errorf("majority-ellipsized category names must reach fit report as shrink_or_split; got %+v", findings)
}

// TestCollectChartDryRenderFindings_EmptyInput is a regression guard: passing
// no slides (or no chart/diagram content) must return nil without panicking.
func TestCollectChartDryRenderFindings_EmptyInput(t *testing.T) {
	if got := collectChartDryRenderFindings(nil, nil, "", "warn"); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	empty := &PresentationInput{}
	if got := collectChartDryRenderFindings(empty, nil, "", "warn"); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
	textOnly := &PresentationInput{
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{Type: "text", PlaceholderID: "title", TextValue: ptr("Hello")},
				},
			},
		},
	}
	if got := collectChartDryRenderFindings(textOnly, nil, "", "warn"); got != nil {
		t.Errorf("text-only input: got %d findings, want 0", len(got))
	}
}

func ptr[T any](v T) *T { return &v }

// TestCollectChartDryRenderFindings_ShapeGridDiagram verifies that a diagram
// embedded directly in a top-level shape_grid cell is dry-rendered and that the
// finding path identifies the owning cell using the slidepath convention
// (go-slide-creator-kzzl).
func TestCollectChartDryRenderFindings_ShapeGridDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Diagram: crowdedNominalDiagram()},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// TestCollectChartDryRenderFindings_CompositeSubDiagram verifies that the
// sub_diagram of a composite cell is traversed and that the finding path points
// at the composite/sub_diagram subfield.
func TestCollectChartDryRenderFindings_CompositeSubDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Composite: &jsonschema.CompositeInput{
									SubDiagram: crowdedNominalDiagram(),
								}},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/composite/sub_diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// TestCollectChartDryRenderFindings_NestedGridDiagram verifies that a diagram
// inside a recursively nested sub-grid is traversed and that the finding path
// threads through the nesting (.../cells/{c}/grid/rows/{r}/cells/{c}/diagram).
func TestCollectChartDryRenderFindings_NestedGridDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Grid: &ShapeGridInput{
									Rows: []GridRowInput{
										{
											Cells: []*GridCellInput{
												nil, // exercise the nil-cell skip
												{Diagram: crowdedNominalDiagram()},
											},
										},
									},
								}},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/1/diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// assertCrowdedNominalAt verifies both the traversal path and action contract.
func assertCrowdedNominalAt(t *testing.T, findings []patterns.FitFinding, wantPath string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == "chart.capacity_exceeded" && f.Action == "shrink_or_split" && strings.Contains(f.Path, wantPath) {
			return
		}
	}
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.Code+"@"+f.Path)
	}
	t.Errorf("expected actionable chart.capacity_exceeded at path containing %q; got %v", wantPath, codes)
}

// go-slide-creator-r87g. The preflight dry-render asked svggen about every
// diagram type, including the half of the catalogue internal/generator draws as
// OOXML shapes. svggen answered "unknown diagram type" and validate reported a
// REFUSE saying the deck would render a grey "Data unavailable" placeholder —
// for diagrams that render perfectly. validate is the precondition gate
// SKILL.md tells agents to trust, so a false refuse there is worse than silence.
func TestDryRender_NativeDiagramsAreNotSvggenBusiness(t *testing.T) {
	for _, typ := range []string{"swot", "business_model_canvas", "value_chain", "heatmap", "panel_layout", "pyramid", "kpi_dashboard"} {
		spec := &types.DiagramSpec{Type: typ, Data: map[string]any{"whatever": "the native builder reads"}}
		if got := dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/diagram_value"); len(got) != 0 {
			t.Errorf("%s: native diagram reported %d finding(s): %+v", typ, len(got), got)
		}
	}
	// A type NEITHER renderer owns still refuses: that one really does become a
	// placeholder.
	spec := &types.DiagramSpec{Type: "sankey_of_the_mind", Data: map[string]any{"a": 1}}
	got := dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/diagram_value")
	if len(got) != 1 || got[0].Action != "refuse" {
		t.Fatalf("an unknown type must still refuse, got %+v", got)
	}
}

func TestDryRender_NativeDiagramGeometryFindings(t *testing.T) {
	items := make([]any, 9)
	for i := range items {
		items[i] = strings.Repeat("Long architecture capability ", 3)
	}
	spec := &types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
		"key_partners": items, "key_activities": items, "value_propositions": items,
	}}
	got := dryRenderSpecToFindings(spec, nil, "Arial", "warn", "slides[0].content[0].diagram_value")
	if len(got) == 0 || got[0].Code != "diagram.text_overlap" || !strings.Contains(got[0].Path, "data.key_partners") {
		t.Fatalf("native BMC findings = %+v", got)
	}
}

// A dry run has to dry-run what actually happens: the render path normalizes an
// authored chart shorthand before svggen sees it, and the dry run used to skip
// that step and validate the shorthand instead. Four shipped example decks
// carried refuse-class findings for charts that render correctly.
func TestDryRender_NormalizesAuthoredChartShorthand(t *testing.T) {
	tests := []struct {
		name  string
		chart *types.ChartSpec //nolint:staticcheck // the authored chart shape
	}{
		{
			name: "waterfall authored as a flat map",
			chart: &types.ChartSpec{ //nolint:staticcheck
				Type:      "waterfall",
				Data:      map[string]any{"Revenue": 100.0, "Costs": -40.0, "Profit": 60.0},
				DataOrder: []string{"Revenue", "Costs", "Profit"},
			},
		},
		{
			name: "radar authored as a flat map",
			chart: &types.ChartSpec{ //nolint:staticcheck
				Type:      "radar",
				Data:      map[string]any{"Architecture": 4.0, "Security": 3.0, "Cost": 5.0},
				DataOrder: []string{"Architecture", "Security", "Cost"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := chartValueToDiagramSpec(tt.chart)
			if spec == nil {
				t.Fatal("no spec")
			}
			for _, f := range dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/chart_value") {
				if f.Action == "refuse" {
					t.Errorf("a chart the renderer draws was refused: %s", f.Message)
				}
			}
		})
	}
}
