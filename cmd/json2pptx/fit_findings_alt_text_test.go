package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestAltText_ImageValuePath_NoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "image",
				ImageValue:    &ImageInput{Path: "team.png"},
			}},
		}},
	}
	findings := collectAltTextFindings(input, nil)
	f := findFinding(findings, patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT finding, got %+v", findings)
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review", f.Action)
	}
	if !strings.Contains(f.Path, "image_value") {
		t.Errorf("path %q should mention image_value", f.Path)
	}
	if !strings.Contains(f.Message, "path") {
		t.Errorf("message should report source=path, got %q", f.Message)
	}
	if f.Fix == nil || f.Fix.Kind != "provide_value" {
		t.Errorf("fix kind = %v, want provide_value", f.Fix)
	}
	if got := f.Fix.Params["field"]; got != "alt" {
		t.Errorf("fix.params.field = %v, want alt", got)
	}
}

func TestAltText_ImageValueURL_NoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "image",
				ImageValue:    &ImageInput{URL: "https://example.com/team.png"},
			}},
		}},
	}
	findings := collectAltTextFindings(input, nil)
	f := findFinding(findings, patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT finding for url-sourced image_value, got %+v", findings)
	}
	if !strings.Contains(f.Message, "url") {
		t.Errorf("message should report source=url, got %q", f.Message)
	}
}

func TestAltText_ImageValueWithAlt_NoFinding(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "image",
				ImageValue:    &ImageInput{Path: "team.png", Alt: "Leadership team on stage"},
			}},
		}},
	}
	if findings := collectAltTextFindings(input, nil); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
		t.Errorf("did not expect MISSING_ALT_TEXT when alt is set, got %+v", findings)
	}
}

func TestAltText_BlankAltCountsAsMissing(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "image",
				ImageValue:    &ImageInput{Path: "team.png", Alt: "   "},
			}},
		}},
	}
	if findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText) == nil {
		t.Errorf("expected MISSING_ALT_TEXT for whitespace-only alt")
	}
}

func TestAltText_GridImage_NoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Image: &GridImageInput{Path: "photo.jpg"},
					}},
				}},
			},
		}},
	}
	findings := collectAltTextFindings(input, nil)
	f := findFinding(findings, patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT for grid image without alt, got %+v", findings)
	}
	if !strings.Contains(f.Path, "shape_grid/rows/0/cells/0/image") {
		t.Errorf("path %q should target shape_grid image", f.Path)
	}
}

func TestAltText_CellIcon_PathNoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "custom.svg"},
					}},
				}},
			},
		}},
	}
	f := findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT for cell icon sourced from path")
	}
	if !strings.Contains(f.Path, "shape_grid/rows/0/cells/0/icon") {
		t.Errorf("path %q should target cell icon", f.Path)
	}
}

func TestAltText_CellIcon_BundledNameExempt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Name: "chart-pie"},
					}},
				}},
			},
		}},
	}
	if findings := collectAltTextFindings(input, nil); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
		t.Errorf("did not expect MISSING_ALT_TEXT for bundled icon by name (implicit caption), got %+v", findings)
	}
}

func TestAltText_CellIcon_SVGDataNoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{SVGData: "<svg xmlns=\"http://www.w3.org/2000/svg\"/>"},
					}},
				}},
			},
		}},
	}
	f := findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT for inline svg_data icon")
	}
	if !strings.Contains(f.Message, "svg_data") {
		t.Errorf("message should report source=svg_data, got %q", f.Message)
	}
}

func TestAltText_ShapeIcon_URLNoAlt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Shape: &ShapeSpecInput{
							Geometry: "rect",
							Icon:     &IconInput{URL: "https://example.com/icon.svg"},
						},
					}},
				}},
			},
		}},
	}
	f := findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText)
	if f == nil {
		t.Fatalf("expected MISSING_ALT_TEXT for shape-overlay icon")
	}
	if !strings.Contains(f.Path, "shape/icon") {
		t.Errorf("path %q should target shape/icon overlay", f.Path)
	}
}

func TestAltText_IconWithAlt_NoFinding(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{
						Icon: &IconInput{Path: "custom.svg", Alt: "Workflow"},
					}, {
						Image: &GridImageInput{URL: "https://example.com/p.png", Alt: "Headshot"},
					}},
				}},
			},
		}},
	}
	if findings := collectAltTextFindings(input, nil); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
		t.Errorf("did not expect MISSING_ALT_TEXT when alt is set on both icon and image, got %+v", findings)
	}
}

func TestAltText_NoSourceNoFinding(t *testing.T) {
	// Icon with neither name, path, url, nor svg_data — no source means no
	// finding (other validators flag the invalid icon separately).
	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Rows: []GridRowInput{{
					Cells: []*GridCellInput{{Icon: &IconInput{}}},
				}},
			},
		}},
	}
	if findings := collectAltTextFindings(input, nil); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
		t.Errorf("did not expect MISSING_ALT_TEXT for empty icon, got %+v", findings)
	}
}

func TestAltText_NilInput(t *testing.T) {
	if findings := collectAltTextFindings(nil, nil); findings != nil {
		t.Errorf("expected nil for nil input, got %+v", findings)
	}
}

// TestAltText_ChartAndTableSurfaces covers the two surfaces the lint gained in
// go-slide-creator-6e8h: a content-level chart/diagram and a content-level
// table with nothing authored to announce.
func TestAltText_ChartAndTableSurfaces(t *testing.T) {
	tests := []struct {
		name     string
		content  ContentInput
		wantPath string
		wantKind string
	}{
		{
			name: "diagram without alt",
			content: ContentInput{
				PlaceholderID: "body",
				Type:          "diagram",
				DiagramValue:  &types.DiagramSpec{Type: "bar_chart", Title: "Revenue"},
			},
			wantPath: "diagram_value",
			wantKind: "diagram",
		},
		{
			name: "chart without alt",
			content: ContentInput{
				PlaceholderID: "body",
				Type:          "chart",
				ChartValue:    &types.ChartSpec{Type: "bar", Title: "Revenue"}, //nolint:staticcheck // ChartSpec is still part of the input contract
			},
			wantPath: "chart_value",
			wantKind: "chart",
		},
		{
			name: "table without alt",
			content: ContentInput{
				PlaceholderID: "body",
				Type:          "table",
				TableValue:    &TableInput{Headers: []string{"Segment"}},
			},
			wantPath: "table_value",
			wantKind: "table",
		},
		{
			name: "legacy untyped value without alt",
			content: ContentInput{
				PlaceholderID: "body",
				Type:          "diagram",
				Value:         []byte(`{"type":"bar_chart","data":{}}`),
			},
			wantPath: "diagram_value",
			wantKind: "diagram",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{tt.content}}}}
			f := findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText)
			if f == nil {
				t.Fatalf("expected MISSING_ALT_TEXT for %s", tt.name)
			}
			if f.Action != "review" {
				t.Errorf("action = %q, want review", f.Action)
			}
			if !strings.Contains(f.Path, tt.wantPath) {
				t.Errorf("path %q should mention %s", f.Path, tt.wantPath)
			}
			if f.Fix == nil || f.Fix.Params["kind"] != tt.wantKind {
				t.Errorf("fix.params.kind = %v, want %s", f.Fix, tt.wantKind)
			}
		})
	}
}

// TestAltText_AuthoredVisualAltIsExempt checks the other half: an alt on the
// chart, diagram or table silences the finding, including one written into the
// legacy untyped payload.
func TestAltText_AuthoredVisualAltIsExempt(t *testing.T) {
	inputs := map[string]ContentInput{
		"diagram": {PlaceholderID: "body", Type: "diagram", DiagramValue: &types.DiagramSpec{Type: "bar_chart", Alt: "Revenue rises every quarter."}},
		"chart":   {PlaceholderID: "body", Type: "chart", ChartValue: &types.ChartSpec{Type: "bar", Alt: "Revenue rises every quarter."}}, //nolint:staticcheck // see above
		"table":   {PlaceholderID: "body", Type: "table", TableValue: &TableInput{Headers: []string{"Segment"}, Alt: "Enterprise is 64% of revenue."}},
		"legacy":  {PlaceholderID: "body", Type: "diagram", Value: []byte(`{"type":"bar_chart","alt":"Revenue rises every quarter."}`)},
	}
	for name, content := range inputs {
		t.Run(name, func(t *testing.T) {
			input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{content}}}}
			if f := findFinding(collectAltTextFindings(input, nil), patterns.ErrCodeMissingAltText); f != nil {
				t.Errorf("did not expect MISSING_ALT_TEXT with alt set, got %+v", f)
			}
		})
	}
}

// TestAltText_PatternExpandedGridCellsAreExempt pins the exemption that keeps
// the lint actionable: a pattern's own grid cells are not the author's, so a
// diagram or table cell the expansion synthesized must not be reported —
// there is no field in the authored payload to put an alt in.
func TestAltText_PatternExpandedGridCellsAreExempt(t *testing.T) {
	grid := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{
		{Diagram: &types.DiagramSpec{Type: "bar_chart"}},
		{Table: &TableInput{Headers: []string{"Segment"}}},
	}}}}
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}

	if f := findFinding(collectAltTextFindings(input, map[int]bool{0: true}), patterns.ErrCodeMissingAltText); f != nil {
		t.Errorf("pattern-expanded cells should be exempt, got %+v", f)
	}
	findings := collectAltTextFindings(input, nil)
	if len(findings) != 2 {
		t.Fatalf("authored grid cells: got %d findings, want 2: %+v", len(findings), findings)
	}
}
