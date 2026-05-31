package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestPrepareImages_VisualCollisionEmitsContentDropped verifies that when two
// visual content blocks (here, two tables) resolve to the same placeholder, the
// second is dropped rather than silently overlapping the first, and a
// machine-actionable CONTENT_DROPPED finding plus a human warning are emitted.
// Regression for go-slide-creator-nujb (field-test 8.8: >2 content blocks
// silently dropped).
func TestPrepareImages_VisualCollisionEmitsContentDropped(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	table := func(h string) *types.TableSpec {
		return &types.TableSpec{
			Headers: []string{h, h + "2"},
			Rows:    [][]types.TableCell{{{Content: "a"}, {Content: "b"}}},
		}
	}

	req := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{
			{
				// slideLayout10 ("Title and Vertical Text") has a single body
				// placeholder, so both tables resolve to the same shape.
				LayoutID: "slideLayout10",
				Content: []ContentItem{
					{PlaceholderID: "body", Type: ContentTable, Value: table("First")},
					{PlaceholderID: "body", Type: ContentTable, Value: table("Second")},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	var dropped *patterns.FitFinding
	for i := range result.FitFindings {
		if result.FitFindings[i].Code == patterns.ErrCodeContentDropped {
			dropped = &result.FitFindings[i]
			break
		}
	}
	if dropped == nil {
		t.Fatalf("expected a CONTENT_DROPPED finding for the colliding visual, got findings: %v", result.FitFindings)
	}
	if dropped.Action != "review" {
		t.Errorf("CONTENT_DROPPED action = %q, want review", dropped.Action)
	}
	// The dropped block is the second content item (index 1).
	if want := "/slides/0/content/1"; dropped.Path != want {
		t.Errorf("CONTENT_DROPPED path = %q, want %q", dropped.Path, want)
	}

	// A human-readable warning must accompany the finding.
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "dropped") && strings.Contains(w, "compose") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Errorf("expected a human warning about the dropped visual, got warnings: %v", result.Warnings)
	}
}

// TestPrepareImages_SingleVisualNoFinding ensures a single visual per
// placeholder does not trigger a false-positive CONTENT_DROPPED finding.
func TestPrepareImages_SingleVisualNoFinding(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout10",
				Content: []ContentItem{
					{PlaceholderID: "body", Type: ContentTable, Value: &types.TableSpec{
						Headers: []string{"H1", "H2"},
						Rows:    [][]types.TableCell{{{Content: "a"}, {Content: "b"}}},
					}},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for i := range result.FitFindings {
		if result.FitFindings[i].Code == patterns.ErrCodeContentDropped {
			t.Fatalf("unexpected CONTENT_DROPPED finding for a single visual: %+v", result.FitFindings[i])
		}
	}
}
