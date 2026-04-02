package generator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestGenerate_Determinism_TextOnly verifies that identical text-only input
// produces byte-identical PPTX output across multiple runs.
// This is the core deterministic compiler mandate: same input → same output.
func TestGenerate_Determinism_TextOnly(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout1",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Determinism Test"},
				{PlaceholderID: "subtitle", Type: ContentText, Value: "Same input must produce same output"},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Slide Two"},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{
					"First bullet point",
					"Second bullet point",
					"Third bullet point",
				}},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Slide Three"},
				{PlaceholderID: "body", Type: ContentText, Value: "Some body text for the third slide with enough content to be realistic."},
			},
			SpeakerNotes: "Remember to discuss the key points here.",
		},
	}

	tmpDir := t.TempDir()

	// Generate PPTX twice with identical input
	output1 := filepath.Join(tmpDir, "run1.pptx")
	output2 := filepath.Join(tmpDir, "run2.pptx")

	req1 := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            output1,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	}
	req2 := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            output2,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	}

	result1, err := Generate(context.Background(), req1)
	if err != nil {
		t.Fatalf("Generate run 1 failed: %v", err)
	}
	result2, err := Generate(context.Background(), req2)
	if err != nil {
		t.Fatalf("Generate run 2 failed: %v", err)
	}

	// File sizes must match
	if result1.FileSize != result2.FileSize {
		t.Errorf("file sizes differ: run1=%d, run2=%d", result1.FileSize, result2.FileSize)
	}

	// Byte-for-byte comparison via SHA-256
	data1, err := os.ReadFile(output1)
	if err != nil {
		t.Fatalf("failed to read run1 output: %v", err)
	}
	data2, err := os.ReadFile(output2)
	if err != nil {
		t.Fatalf("failed to read run2 output: %v", err)
	}

	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)

	if hash1 != hash2 {
		t.Errorf("DETERMINISM VIOLATION: identical input produced different output\n"+
			"  run1: sha256=%x (%d bytes)\n"+
			"  run2: sha256=%x (%d bytes)",
			hash1, len(data1), hash2, len(data2))

		// Save both for manual diffing
		_ = os.WriteFile(filepath.Join(tmpDir, "run1_SAVED.pptx"), data1, 0o644)
		_ = os.WriteFile(filepath.Join(tmpDir, "run2_SAVED.pptx"), data2, 0o644)
		t.Logf("saved outputs to %s for inspection", tmpDir)
	}
}

// TestGenerate_Determinism_WithImages verifies determinism when slides include images.
func TestGenerate_Determinism_WithImages(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout1",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Image Determinism Test"},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Slide With Image"},
				{PlaceholderID: "body", Type: ContentImage, Value: ImageContent{
					Path: testImage,
					Alt:  "Test image",
				}},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Another Text Slide"},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{
					"Bullet A",
					"Bullet B",
				}},
			},
		},
	}

	tmpDir := t.TempDir()
	output1 := filepath.Join(tmpDir, "img_run1.pptx")
	output2 := filepath.Join(tmpDir, "img_run2.pptx")

	absImage, _ := filepath.Abs(testImage)
	allowedPaths := []string{filepath.Dir(absImage)}

	req1 := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            output1,
		Slides:                slides,
		AllowedImagePaths:     allowedPaths,
		ExcludeTemplateSlides: true,
	}
	req2 := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            output2,
		Slides:                slides,
		AllowedImagePaths:     allowedPaths,
		ExcludeTemplateSlides: true,
	}

	_, err := Generate(context.Background(), req1)
	if err != nil {
		t.Fatalf("Generate run 1 failed: %v", err)
	}
	_, err = Generate(context.Background(), req2)
	if err != nil {
		t.Fatalf("Generate run 2 failed: %v", err)
	}

	data1, err := os.ReadFile(output1)
	if err != nil {
		t.Fatalf("failed to read run1: %v", err)
	}
	data2, err := os.ReadFile(output2)
	if err != nil {
		t.Fatalf("failed to read run2: %v", err)
	}

	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)

	if hash1 != hash2 {
		t.Errorf("DETERMINISM VIOLATION (with images): identical input produced different output\n"+
			"  run1: sha256=%x (%d bytes)\n"+
			"  run2: sha256=%x (%d bytes)",
			hash1, len(data1), hash2, len(data2))
	}
}

// TestGenerate_Determinism_ManySlides verifies determinism with many slides
// to stress-test map iteration ordering.
func TestGenerate_Determinism_ManySlides(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Create 10 slides to exercise map iteration paths thoroughly
	slides := make([]SlideSpec, 10)
	slides[0] = SlideSpec{
		LayoutID: "slideLayout1",
		Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentText, Value: "Many Slides Determinism Test"},
			{PlaceholderID: "subtitle", Type: ContentText, Value: "10 slides for stress testing"},
		},
	}
	for i := 1; i < 10; i++ {
		slides[i] = SlideSpec{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: fmt.Sprintf("Slide %d", i+1)},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{
					fmt.Sprintf("Point A on slide %d", i+1),
					fmt.Sprintf("Point B on slide %d", i+1),
					fmt.Sprintf("Point C on slide %d", i+1),
				}},
			},
			SpeakerNotes: fmt.Sprintf("Notes for slide %d", i+1),
		}
	}

	tmpDir := t.TempDir()

	// Run 3 times to catch intermittent ordering issues
	hashes := make([][32]byte, 3)
	for run := 0; run < 3; run++ {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("run%d.pptx", run))
		req := GenerationRequest{
			TemplatePath:          templatePath,
			OutputPath:            outputPath,
			Slides:                slides,
			ExcludeTemplateSlides: true,
		}
		_, err := Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Generate run %d failed: %v", run, err)
		}

		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("failed to read run %d: %v", run, err)
		}
		hashes[run] = sha256.Sum256(data)
	}

	for i := 1; i < 3; i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("DETERMINISM VIOLATION: run %d differs from run 0\n"+
				"  run 0: sha256=%x\n"+
				"  run %d: sha256=%x",
				i, hashes[0], i, hashes[i])
		}
	}
}

// TestGenerate_Determinism_PanelLayout verifies byte-reproducible output
// for slides containing native panel column shapes.
// Key risks: map iteration order in panelShapeInserts, shape ID allocation
// order, and icon processing order must all be deterministic.
func TestGenerate_Determinism_PanelLayout(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// 4-panel columns layout with icons and bullet text, spread across
	// two slides to exercise sorted map iteration in allocatePanelIconRelIDs.
	panelSlide := func(title string, panels []any) SlideSpec {
		return SlideSpec{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: title},
				{PlaceholderID: "body", Type: ContentDiagram, Value: &types.DiagramSpec{
					Type: "panel_layout",
					Data: map[string]any{
						"layout": "columns",
						"panels": panels,
					},
				}},
			},
		}
	}

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout1",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Panel Determinism Test"},
				{PlaceholderID: "subtitle", Type: ContentText, Value: "Verifying byte-identical output"},
			},
		},
		panelSlide("Four Panels", []any{
			map[string]any{"title": "Strategy", "body": "- Define goals\n- Align teams\n- Set KPIs"},
			map[string]any{"title": "Operations", "body": "- Streamline processes\n- Reduce waste"},
			map[string]any{"title": "Technology", "body": "- Modernize stack\n- Cloud migration\n- API-first"},
			map[string]any{"title": "People", "body": "- Hire talent\n- Upskill teams"},
		}),
		panelSlide("Two Panels", []any{
			map[string]any{"title": "Pros", "body": "- Faster delivery\n- Lower cost"},
			map[string]any{"title": "Cons", "body": "- Migration risk\n- Training needed"},
		}),
	}

	tmpDir := t.TempDir()

	// Run 3 times to catch intermittent ordering issues
	hashes := make([][32]byte, 3)
	for run := 0; run < 3; run++ {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("panel_run%d.pptx", run))
		req := GenerationRequest{
			TemplatePath:          templatePath,
			OutputPath:            outputPath,
			Slides:                slides,
			ExcludeTemplateSlides: true,
		}
		_, err := Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Generate run %d failed: %v", run, err)
		}

		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("failed to read run %d: %v", run, err)
		}
		hashes[run] = sha256.Sum256(data)
	}

	for i := 1; i < 3; i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("DETERMINISM VIOLATION (panel layout): run %d differs from run 0\n"+
				"  run 0: sha256=%x\n"+
				"  run %d: sha256=%x",
				i, hashes[0], i, hashes[i])
		}
	}
}
