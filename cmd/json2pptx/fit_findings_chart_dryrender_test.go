package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// narrowTickThinnedDiagram builds a 40-category bar chart at a render width
// narrow enough that svggen's labeling pass thins tick labels, emitting
// chart.tick_thinned. Shared by the dry-render shape_grid tests below.
func narrowTickThinnedDiagram() *types.DiagramSpec {
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

// TestCollectChartDryRenderFindings_TickThinned verifies that a 20-category
// bar chart at a narrow render width surfaces chart.tick_thinned via the
// dry-render path, closing the validate → preview → generate feedback loop
// (acceptance criterion 6 of bead go-slide-creator-0ywh).
func TestCollectChartDryRenderFindings_TickThinned(t *testing.T) {
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

	findings := collectChartDryRenderFindings(input, nil, "warn")
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding for 20-cat narrow bar chart, got none")
	}

	var sawTickThinned bool
	for _, f := range findings {
		if f.Code == "chart.tick_thinned" {
			sawTickThinned = true
			if !strings.Contains(f.Path, "slides[0].content[0].diagram_value") {
				t.Errorf("tick_thinned path = %q; want prefix slides[0].content[0].diagram_value", f.Path)
			}
		}
	}
	if !sawTickThinned {
		// Helpful diagnostics when the geometry changes upstream.
		codes := make([]string, 0, len(findings))
		for _, f := range findings {
			codes = append(codes, f.Code)
		}
		t.Errorf("expected chart.tick_thinned in dry-render findings; got codes=%v", codes)
	}
}

// TestCollectChartDryRenderFindings_EmptyInput is a regression guard: passing
// no slides (or no chart/diagram content) must return nil without panicking.
func TestCollectChartDryRenderFindings_EmptyInput(t *testing.T) {
	if got := collectChartDryRenderFindings(nil, nil, "warn"); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	empty := &PresentationInput{}
	if got := collectChartDryRenderFindings(empty, nil, "warn"); got != nil {
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
	if got := collectChartDryRenderFindings(textOnly, nil, "warn"); got != nil {
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
								{Diagram: narrowTickThinnedDiagram()},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/diagram"
	assertTickThinnedAt(t, findings, wantPath)
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
									SubDiagram: narrowTickThinnedDiagram(),
								}},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/composite/sub_diagram"
	assertTickThinnedAt(t, findings, wantPath)
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
												{Diagram: narrowTickThinnedDiagram()},
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

	findings := collectChartDryRenderFindings(input, nil, "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/1/diagram"
	assertTickThinnedAt(t, findings, wantPath)
}

// assertTickThinnedAt fails the test unless findings contains a
// chart.tick_thinned finding whose Path contains wantPath.
func assertTickThinnedAt(t *testing.T, findings []patterns.FitFinding, wantPath string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == "chart.tick_thinned" && strings.Contains(f.Path, wantPath) {
			return
		}
	}
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.Code+"@"+f.Path)
	}
	t.Errorf("expected chart.tick_thinned at path containing %q; got %v", wantPath, codes)
}
