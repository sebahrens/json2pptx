package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
	findings := collectAltTextFindings(input)
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
	findings := collectAltTextFindings(input)
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
	if findings := collectAltTextFindings(input); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
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
	if findFinding(collectAltTextFindings(input), patterns.ErrCodeMissingAltText) == nil {
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
	findings := collectAltTextFindings(input)
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
	f := findFinding(collectAltTextFindings(input), patterns.ErrCodeMissingAltText)
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
	if findings := collectAltTextFindings(input); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
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
	f := findFinding(collectAltTextFindings(input), patterns.ErrCodeMissingAltText)
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
	f := findFinding(collectAltTextFindings(input), patterns.ErrCodeMissingAltText)
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
	if findings := collectAltTextFindings(input); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
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
	if findings := collectAltTextFindings(input); findFinding(findings, patterns.ErrCodeMissingAltText) != nil {
		t.Errorf("did not expect MISSING_ALT_TEXT for empty icon, got %+v", findings)
	}
}

func TestAltText_NilInput(t *testing.T) {
	if findings := collectAltTextFindings(nil); findings != nil {
		t.Errorf("expected nil for nil input, got %+v", findings)
	}
}
