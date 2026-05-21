package generator

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestGenerate_ExcludeTemplateSlides_NoOrphanNotesRels verifies the fix for
// go-slide-creator-0g9c. Several designer templates (abstract.pptx, modern.pptx)
// ship with notesSlide{2..4}.xml plus matching _rels in ppt/notesSlides/, whose
// rels target ppt/slides/slide{2..4}.xml. When ExcludeTemplateSlides=true those
// template slide parts are dropped, but the notes-slide rels survived the copy
// — yielding three blocking OPC_DANGLING_REL findings per template.
//
// The fix skips template notes-slide files (and their .rels) tied to excluded
// template slides, and strips the matching Override entries from
// [Content_Types].xml. This test re-renders abstract.pptx and asserts that
// pptx.ValidateOutputBytes returns no OPC_DANGLING_REL findings whose path
// points into ppt/notesSlides/.
func TestGenerate_ExcludeTemplateSlides_NoOrphanNotesRels(t *testing.T) {
	templates := []string{
		"abstract.pptx",
		"modern.pptx",
	}

	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			templatePath := filepath.Join("..", "..", "templates", name)
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				t.Skipf("template %s not found at %s", name, templatePath)
			}

			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "out.pptx")

			req := GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            outputPath,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{
						LayoutID: "slideLayout1",
						Content: []ContentItem{
							{
								PlaceholderID: "title",
								Type:          ContentText,
								Value:         "Orphan notes regression",
							},
						},
					},
				},
			}

			if _, err := Generate(context.Background(), req); err != nil {
				t.Fatalf("Generate failed for %s: %v", name, err)
			}

			report, err := pptx.ValidateOutputFile(outputPath)
			if err != nil {
				t.Fatalf("ValidateOutputFile failed for %s: %v", name, err)
			}

			for _, f := range report.Findings {
				if f.Code == "OPC_DANGLING_REL" && strings.Contains(f.Path, "ppt/notesSlides/") {
					t.Errorf("unexpected orphan notes rel finding for %s: %s", name, f.Error())
				}
			}

			// Belt-and-braces: no template notes-slide N>=2 files should remain
			// in the output ZIP for these templates. (slide1's notes survive
			// only if it's also pruned; we just want to confirm the dangling
			// ones are gone.)
			r, err := zip.OpenReader(outputPath)
			if err != nil {
				t.Fatalf("open output zip: %v", err)
			}
			defer func() { _ = r.Close() }()

			for _, f := range r.File {
				if num, ok := parseNotesSlideNum(f.Name); ok && num >= 2 {
					t.Errorf("template notes slide %s still present in %s output", f.Name, name)
				}
				if num, ok := parseNotesSlideRelsNum(f.Name); ok && num >= 2 {
					t.Errorf("template notes rels %s still present in %s output", f.Name, name)
				}
			}
		})
	}
}
