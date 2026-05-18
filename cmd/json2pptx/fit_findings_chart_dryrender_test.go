package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

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
