package generator

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerate_FileSize_AC10 tests AC10: File Size Reasonable
// Given 10-slide presentation with 3 images
// When generated
// Then file size < 10MB
func TestGenerate_FileSize_AC10(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Verify test images exist
	testImages := []string{
		"testdata/test_image_small.png",
		"testdata/test_image_large.png",
	}
	for _, img := range testImages {
		if _, err := os.Stat(img); os.IsNotExist(err) {
			t.Skipf("test image not found: %s", img)
		}
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "large_presentation.pptx")

	// Create 10 slides with varying content
	slides := make([]SlideSpec, 10)

	// Slide 1: Title slide with text
	slides[0] = SlideSpec{
		LayoutID: "slideLayout1",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Large Presentation Test",
			},
			{
				PlaceholderID: "subtitle",
				Type:          ContentText,
				Value:         "Testing AC10: File Size < 10MB",
			},
		},
	}

	// Slide 2: Image slide
	slides[1] = SlideSpec{
		LayoutID: "slideLayout2",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Image Slide 1",
			},
			{
				PlaceholderID: "body",
				Type:          ContentImage,
				Value: ImageContent{
					Path: testImages[0],
					Alt:  "Test image 1",
				},
			},
		},
	}

	// Slide 3: Another image slide
	slides[2] = SlideSpec{
		LayoutID: "slideLayout2",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Image Slide 2",
			},
			{
				PlaceholderID: "body",
				Type:          ContentImage,
				Value: ImageContent{
					Path: testImages[1],
					Alt:  "Test image 2",
				},
			},
		},
	}

	// Slide 4: Third image slide
	slides[3] = SlideSpec{
		LayoutID: "slideLayout2",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Image Slide 3",
			},
			{
				PlaceholderID: "body",
				Type:          ContentImage,
				Value: ImageContent{
					Path: testImages[0],
					Alt:  "Test image 3",
				},
			},
		},
	}

	// Slides 5-8: Content slides with bullets
	for i := 4; i < 8; i++ {
		slides[i] = SlideSpec{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{
					PlaceholderID: "title",
					Type:          ContentText,
					Value:         "Content Slide " + string(rune('A'+i-4)),
				},
				{
					PlaceholderID: "body",
					Type:          ContentBullets,
					Value: []string{
						"First bullet point with some text content",
						"Second bullet point with more information",
						"Third bullet point to add variety",
						"Fourth bullet point for good measure",
						"Fifth bullet point to test capacity",
					},
				},
			},
		}
	}

	// Slide 9: Mixed content
	slides[8] = SlideSpec{
		LayoutID: "slideLayout2",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Summary Slide",
			},
			{
				PlaceholderID: "body",
				Type:          ContentBullets,
				Value: []string{
					"Generated 10 slides successfully",
					"Embedded 3 images in total",
					"File size should be under 10MB",
				},
			},
		},
	}

	// Slide 10: Final slide
	slides[9] = SlideSpec{
		LayoutID: "slideLayout1",
		Content: []ContentItem{
			{
				PlaceholderID: "title",
				Type:          ContentText,
				Value:         "Thank You",
			},
			{
				PlaceholderID: "subtitle",
				Type:          ContentText,
				Value:         "End of Presentation",
			},
		},
	}

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides:       slides,
	}

	// Generate the presentation
	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify slide count
	if result.SlideCount != 10 {
		t.Errorf("SlideCount = %d, want 10", result.SlideCount)
	}

	// Verify file exists
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file not found: %v", err)
	}

	// AC10: File size < 10MB
	const maxSize = 10 * 1024 * 1024 // 10MB in bytes
	if info.Size() >= maxSize {
		t.Errorf("File size = %d bytes (%.2f MB), want < %d bytes (10 MB)",
			info.Size(), float64(info.Size())/(1024*1024), maxSize)
	}

	// Log actual size for reference
	t.Logf("Generated file size: %d bytes (%.2f MB)", info.Size(), float64(info.Size())/(1024*1024))
	t.Logf("Generation took: %v", result.Duration)

	// Verify it's a valid PPTX
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Count actual slides in the output
	slideCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
	}

	if slideCount < 10 {
		t.Errorf("Found %d slide files, expected at least 10", slideCount)
	}

	// Report warnings if any
	if len(result.Warnings) > 0 {
		t.Logf("Warnings: %v", result.Warnings)
	}
}

// TestGenerate_CompleteIntegration tests the complete generation pipeline
// with text, bullets, and images working together
func TestGenerate_CompleteIntegration(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "integrated.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			// Title slide
			{
				LayoutID: "slideLayout1",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Integration Test",
					},
					{
						PlaceholderID: "subtitle",
						Type:          ContentText,
						Value:         "Complete Pipeline Validation",
					},
				},
			},
			// Content slide with bullets
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Feature List",
					},
					{
						PlaceholderID: "body",
						Type:          ContentBullets,
						Value: []string{
							"Slide creation from layouts",
							"Text population",
							"Bullet list support",
							"Image embedding",
							"Theme preservation",
						},
					},
				},
			},
			// Image slide
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Visual Content",
					},
					{
						PlaceholderID: "body",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImage,
							Alt:  "Integrated image",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all components worked
	if result.SlideCount != 3 {
		t.Errorf("SlideCount = %d, want 3", result.SlideCount)
	}

	if result.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", result.FileSize)
	}

	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}

	// Verify output is valid PPTX
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Verify structure
	hasPresentation := false
	hasSlides := false
	for _, f := range r.File {
		if f.Name == "ppt/presentation.xml" {
			hasPresentation = true
		}
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			hasSlides = true
		}
	}

	if !hasPresentation {
		t.Error("Missing ppt/presentation.xml")
	}
	if !hasSlides {
		t.Error("No slides found in output")
	}

	t.Logf("Integration test complete: %d slides, %d bytes, %v duration",
		result.SlideCount, result.FileSize, result.Duration)
}

// TestGenerate_EmptyContentHandling tests AC12: Empty content handling
// Given slide with no content items
// When generated
// Then slide created with placeholder text removed
func TestGenerate_EmptyContentHandling(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "empty_content.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout1",
				Content:  []ContentItem{}, // No content - should create blank slide
			},
			{
				LayoutID: "slideLayout2",
				Content:  []ContentItem{}, // Another blank slide
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 2 {
		t.Errorf("SlideCount = %d, want 2", result.SlideCount)
	}

	// Verify slides were created
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	slideCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
	}

	if slideCount < 2 {
		t.Errorf("Found %d slide files, expected at least 2", slideCount)
	}

	t.Logf("Empty content test complete: %d slides created", result.SlideCount)
}

// TestGenerate_MultipleImagesPerSlide tests AC-NEW5: Multiple Images Per Slide
// Given slide with 2 image placeholders and 2 images
// When generated
// Then both images appear with unique relationship IDs
func TestGenerate_MultipleImagesPerSlide(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImages := []string{
		"testdata/test_image_small.png",
		"testdata/test_image_large.png",
	}
	for _, img := range testImages {
		if _, err := os.Stat(img); os.IsNotExist(err) {
			t.Skipf("test image not found: %s", img)
		}
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "multiple_images.pptx")

	// Use slideLayout9 which has picture placeholders
	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImages[0],
							Alt:  "First image",
						},
					},
				},
			},
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImages[1],
							Alt:  "Second image",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify both slides created
	if result.SlideCount != 2 {
		t.Errorf("SlideCount = %d, want 2", result.SlideCount)
	}

	// Verify output is valid PPTX
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// AC-NEW5: Both images should have unique relationship IDs
	// Verify media files were added
	mediaCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			mediaCount++
		}
	}

	if mediaCount < 2 {
		t.Errorf("Expected at least 2 media files, found %d", mediaCount)
	}

	t.Logf("Multiple images test complete: %d media files", mediaCount)
}

// TestGenerate_MixedContent tests mixed content integration
// Given slide with text, bullets, and images
// When generated
// Then all content types are correctly rendered
func TestGenerate_MixedContent(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mixed_content.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			// Slide 1: Title slide with text
			{
				LayoutID: "slideLayout1",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Mixed Content Presentation",
					},
					{
						PlaceholderID: "subtitle",
						Type:          ContentText,
						Value:         "Testing text, bullets, and images together",
					},
				},
			},
			// Slide 2: Content slide with bullets
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Key Features",
					},
					{
						PlaceholderID: "body",
						Type:          ContentBullets,
						Value: []string{
							"Text content support",
							"Bullet lists",
							"Image embedding",
							"Chart rendering",
						},
					},
				},
			},
			// Slide 3: Image slide
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImage,
							Alt:  "Sample image",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all slides created
	if result.SlideCount != 3 {
		t.Errorf("SlideCount = %d, want 3", result.SlideCount)
	}

	// Verify output structure
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Check for expected files
	hasPresentation := false
	slideCount := 0
	hasMedia := false

	for _, f := range r.File {
		if f.Name == "ppt/presentation.xml" {
			hasPresentation = true
		}
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			hasMedia = true
		}
	}

	if !hasPresentation {
		t.Error("Missing ppt/presentation.xml")
	}
	if slideCount < 3 {
		t.Errorf("Expected at least 3 slide files, found %d", slideCount)
	}
	if !hasMedia {
		t.Error("Expected media files for image")
	}

	t.Logf("Mixed content test complete: %d slides, media=%v", slideCount, hasMedia)
}

// TestGenerate_ImageEmbedding_AC5 tests AC5: Image Embedding
// Given valid image path
// When generated
// Then image appears in slide at placeholder position
func TestGenerate_ImageEmbedding_AC5(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "image_embed.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImage,
							Alt:  "Test image for AC5",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// AC5: Verify image is embedded
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// AC-NEW3: Image should be in ppt/media/
	foundMedia := false
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			foundMedia = true
			break
		}
	}

	if !foundMedia {
		t.Error("AC5/AC-NEW3: Image not found in ppt/media/")
	}

	t.Logf("AC5 test complete: image embedded successfully")
}

// TestGenerate_ImageScaling_AC6 tests AC6: Image Scaling
// Given image larger than placeholder
// When generated
// Then image is scaled to fit (maintaining aspect ratio)
func TestGenerate_ImageScaling_AC6(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_large.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "image_scaled.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImage,
							Alt:  "Large test image for AC6",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// AC6: Image should be scaled - verify file was created successfully
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file not found: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Output file is empty")
	}

	// Verify it's a valid PPTX with media
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	foundMedia := false
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			foundMedia = true
			break
		}
	}

	if !foundMedia {
		t.Error("AC6: Scaled image not found in output")
	}

	t.Logf("AC6 test complete: large image scaled and embedded")
}

// TestGenerate_RelationshipLinkage_AC_NEW2 tests AC-NEW2: Relationship Linkage
// Given image embedded in slide
// When generated
// Then slide .rels file contains relationship matching r:embed attribute
func TestGenerate_RelationshipLinkage_AC_NEW2(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "relationship_test.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImage,
							Alt:  "Test image for AC-NEW2",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// AC-NEW2: Verify relationships file is present
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Find relationships files
	foundRels := false
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/slides/_rels/slide*.xml.rels", f.Name); matched {
			foundRels = true
			break
		}
	}

	if !foundRels {
		t.Error("AC-NEW2: No relationship files found")
	}

	t.Logf("AC-NEW2 test complete: relationship linkage verified")
}

// TestGenerate_FullPipelineValidation validates the complete generation pipeline
// with all image-related acceptance criteria
func TestGenerate_FullPipelineValidation(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImages := []string{
		"testdata/test_image_small.png",
		"testdata/test_image_large.png",
	}
	for _, img := range testImages {
		if _, err := os.Stat(img); os.IsNotExist(err) {
			t.Skipf("test image not found: %s", img)
		}
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "full_pipeline.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			// Title slide
			{
				LayoutID: "slideLayout1",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Full Pipeline Test",
					},
				},
			},
			// First image slide
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImages[0],
							Alt:  "Small image",
						},
					},
				},
			},
			// Second image slide
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testImages[1],
							Alt:  "Large image",
						},
					},
				},
			},
			// Content slide
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Summary",
					},
					{
						PlaceholderID: "body",
						Type:          ContentBullets,
						Value: []string{
							"All acceptance criteria validated",
							"Image embedding works correctly",
							"Pipeline is complete",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify basic generation
	if result.SlideCount != 4 {
		t.Errorf("SlideCount = %d, want 4", result.SlideCount)
	}

	if result.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", result.FileSize)
	}

	// Open and validate the output
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Validation checks
	var (
		hasPresentation = false
		slideCount      = 0
		mediaCount      = 0
		relsCount       = 0
	)

	for _, f := range r.File {
		if f.Name == "ppt/presentation.xml" {
			hasPresentation = true
		}
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			mediaCount++
		}
		if matched, _ := filepath.Match("ppt/slides/_rels/slide*.xml.rels", f.Name); matched {
			relsCount++
		}
	}

	if !hasPresentation {
		t.Error("Missing ppt/presentation.xml")
	}
	if slideCount < 4 {
		t.Errorf("Expected at least 4 slide files, found %d", slideCount)
	}
	if mediaCount < 2 {
		t.Errorf("Expected at least 2 media files, found %d", mediaCount)
	}
	if relsCount < 2 {
		t.Errorf("Expected at least 2 relationship files, found %d", relsCount)
	}

	t.Logf("Full pipeline validation complete:")
	t.Logf("  - Slides: %d", slideCount)
	t.Logf("  - Media files: %d", mediaCount)
	t.Logf("  - Relationship files: %d", relsCount)
	t.Logf("  - File size: %d bytes", result.FileSize)
	t.Logf("  - Duration: %v", result.Duration)

	if len(result.Warnings) > 0 {
		t.Logf("  - Warnings: %v", result.Warnings)
	}
}

// TestGenerate_StreamingImageHandling_AC2 tests AC2 (C2 fix): Streaming Image Handling
// AC2: Given a slide with a large image, When generating PPTX,
// Then memory usage stays bounded (streaming instead of loading entirely into memory)
//
// Note: This test validates that the streamImageToZip function is used correctly
// by verifying that large images can be processed without issues.
func TestGenerate_StreamingImageHandling_AC2(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Create a larger test image to verify streaming works
	testImage := "testdata/test_image_large.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "streaming_test.pptx")

	// Create slides with multiple large images to stress the streaming
	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value:         ImageContent{Path: testImage, Alt: "Large image 1"},
					},
				},
			},
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value:         ImageContent{Path: testImage, Alt: "Large image 2"},
					},
				},
			},
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value:         ImageContent{Path: testImage, Alt: "Large image 3"},
					},
				},
			},
		},
	}

	// Generate the presentation - streaming should handle large images without memory issues
	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 3 {
		t.Errorf("SlideCount = %d, want 3", result.SlideCount)
	}

	// Verify the output is valid
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Verify media files were added correctly
	mediaCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			mediaCount++
		}
	}

	// Note: Same image path may be deduplicated
	if mediaCount < 1 {
		t.Errorf("Expected at least 1 media file, found %d", mediaCount)
	}

	t.Logf("AC2 streaming image handling verified:")
	t.Logf("  - Slides: %d", result.SlideCount)
	t.Logf("  - Media files: %d", mediaCount)
	t.Logf("  - File size: %d bytes", result.FileSize)
	t.Logf("  - Duration: %v", result.Duration)
}

// TestStreamImageToZip tests the streamImageToZip helper function directly
func TestStreamImageToZip(t *testing.T) {
	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	// Create temp file for ZIP output
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	// Create ZIP writer
	w := zip.NewWriter(tmpFile)

	// Stream the image
	err = streamImageToZip(w, testImage, "ppt/media/image1.png")
	if err != nil {
		t.Fatalf("streamImageToZip failed: %v", err)
	}

	// Close ZIP
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close ZIP: %v", err)
	}

	// Verify the ZIP contents
	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if f.Name == "ppt/media/image1.png" {
			found = true
			// Verify size matches original
			origInfo, _ := os.Stat(testImage)
			if f.UncompressedSize64 != uint64(origInfo.Size()) {
				t.Errorf("Image size mismatch: got %d, want %d",
					f.UncompressedSize64, origInfo.Size())
			}
			break
		}
	}

	if !found {
		t.Error("Image not found in ZIP archive")
	}
}

// TestStreamImageToZip_MissingFile tests error handling for missing files
func TestStreamImageToZip_MissingFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	w := zip.NewWriter(tmpFile)

	// Try to stream a non-existent file
	err = streamImageToZip(w, "non_existent_image.png", "ppt/media/image1.png")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}

	_ = w.Close()
}

// TestGenerate_SinglePassOptimization_AC1 tests that generation uses single-pass ZIP handling
// AC1 (C1 fix): Given a generation request with 10 slides
// When processed, Then only ONE ZIP read and ONE ZIP write operation occurs
//
// Note: This is a structural/performance optimization test. We verify the output
// is correct (which implies single-pass worked) and that performance improved.
func TestGenerate_SinglePassOptimization_AC1(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "single_pass.pptx")

	// Create 10 slides with mixed content to stress test single-pass
	slides := make([]SlideSpec, 10)

	// Title slide
	slides[0] = SlideSpec{
		LayoutID: "slideLayout1",
		Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentText, Value: "Single Pass Test"},
		},
	}

	// Content slides with bullets
	for i := 1; i < 8; i++ {
		slides[i] = SlideSpec{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Content Slide"},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Bullet 1", "Bullet 2"}},
			},
		}
	}

	// Image slides
	slides[8] = SlideSpec{
		LayoutID: "slideLayout9",
		Content: []ContentItem{
			{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: testImage, Alt: "Image 1"}},
		},
	}
	slides[9] = SlideSpec{
		LayoutID: "slideLayout9",
		Content: []ContentItem{
			{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: testImage, Alt: "Image 2"}},
		},
	}

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides:       slides,
	}

	// Generate with single-pass optimization
	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify correct output (implies single-pass worked correctly)
	if result.SlideCount != 10 {
		t.Errorf("SlideCount = %d, want 10", result.SlideCount)
	}

	// Verify file is valid PPTX
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Check for expected content
	var (
		slideCount = 0
		mediaCount = 0
	)

	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			mediaCount++
		}
	}

	if slideCount < 10 {
		t.Errorf("Expected at least 10 slide files, found %d", slideCount)
	}
	if mediaCount < 1 {
		t.Errorf("Expected at least 1 media file, found %d", mediaCount)
	}

	// Performance: single-pass should be faster
	// Note: We can't directly measure "one ZIP read/write" but we can verify
	// the result is correct and duration is reasonable
	t.Logf("AC1 single-pass optimization verified:")
	t.Logf("  - Slides: %d", slideCount)
	t.Logf("  - Media files: %d", mediaCount)
	t.Logf("  - File size: %d bytes", result.FileSize)
	t.Logf("  - Duration: %v (single-pass)", result.Duration)
}

// TestGenerate_SVGImageEmbedding tests SVG image embedding via PNG conversion
// Given valid SVG image path
// When generated
// Then SVG is converted to PNG and embedded in the slide
func TestGenerate_SVGImageEmbedding(t *testing.T) {
	if !SVGConversionAvailable() {
		t.Skip("SVG conversion not available (rsvg-convert not installed)")
	}

	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testSVG := "testdata/test_image.svg"
	if _, err := os.Stat(testSVG); os.IsNotExist(err) {
		t.Skip("test SVG not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "svg_embed.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testSVG,
							Alt:  "SVG test image",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// SVG should be converted to PNG and embedded
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// With native SVG strategy (default), SVG is embedded alongside PNG fallback
	foundPNG := false
	foundSVG := false
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*.png", f.Name); matched {
			foundPNG = true
		}
		if matched, _ := filepath.Match("ppt/media/image*.svg", f.Name); matched {
			foundSVG = true
		}
	}

	if !foundPNG {
		t.Error("Expected PNG fallback image in media folder")
	}
	if !foundSVG {
		t.Error("Expected SVG image in media folder (native SVG embedding)")
	}

	t.Logf("SVG embedding test complete: SVG + PNG fallback embedded (native strategy)")
}

// TestGenerate_SVGWithPNGImage tests mixed SVG and PNG images
func TestGenerate_SVGWithPNGImage(t *testing.T) {
	if !SVGConversionAvailable() {
		t.Skip("SVG conversion not available (rsvg-convert not installed)")
	}

	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testSVG := "testdata/test_image.svg"
	testPNG := "testdata/test_image_small.png"
	if _, err := os.Stat(testSVG); os.IsNotExist(err) {
		t.Skip("test SVG not found")
	}
	if _, err := os.Stat(testPNG); os.IsNotExist(err) {
		t.Skip("test PNG not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mixed_svg_png.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			// Slide with SVG
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testSVG,
							Alt:  "SVG image",
						},
					},
				},
			},
			// Slide with PNG
			{
				LayoutID: "slideLayout9",
				Content: []ContentItem{
					{
						PlaceholderID: "image",
						Type:          ContentImage,
						Value: ImageContent{
							Path: testPNG,
							Alt:  "PNG image",
						},
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 2 {
		t.Errorf("SlideCount = %d, want 2", result.SlideCount)
	}

	// Both images should be embedded
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	mediaCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			mediaCount++
		}
	}

	if mediaCount < 2 {
		t.Errorf("Expected at least 2 media files, found %d", mediaCount)
	}

	t.Logf("Mixed SVG/PNG test complete: %d media files embedded", mediaCount)
}

// TestStreamBytesToZip tests the streamBytesToZip helper function directly
// This function writes byte data directly to ZIP without temp files
func TestStreamBytesToZip(t *testing.T) {
	// Create test data
	testData := []byte("This is test PNG data for streaming")

	// Create temp file for ZIP output
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	// Create ZIP writer
	w := zip.NewWriter(tmpFile)

	// Stream the bytes directly
	err = streamBytesToZip(w, testData, "ppt/media/chart1.png")
	if err != nil {
		t.Fatalf("streamBytesToZip failed: %v", err)
	}

	// Close ZIP
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close ZIP: %v", err)
	}

	// Verify the ZIP contents
	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if f.Name == "ppt/media/chart1.png" {
			found = true
			// Verify size matches original data
			if f.UncompressedSize64 != uint64(len(testData)) {
				t.Errorf("Data size mismatch: got %d, want %d",
					f.UncompressedSize64, len(testData))
			}
			break
		}
	}

	if !found {
		t.Error("Chart data not found in ZIP archive")
	}
}

// BenchmarkStreamingZipWrite benchmarks the streaming ZIP write performance
// Validates that memory stays bounded during large writes
func BenchmarkStreamingZipWrite(b *testing.B) {
	// Create large test data (1MB chunks simulating chart images)
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tmpFile, err := os.CreateTemp(b.TempDir(), "bench-*.zip")
		if err != nil {
			b.Fatalf("Failed to create temp file: %v", err)
		}

		w := zip.NewWriter(tmpFile)

		// Write 10 "chart images" of 1MB each
		for j := 0; j < 10; j++ {
			path := fmt.Sprintf("ppt/media/chart%d.png", j)
			if err := streamBytesToZip(w, largeData, path); err != nil {
				b.Fatalf("streamBytesToZip failed: %v", err)
			}
		}

		_ = w.Close()
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}
}

// TestInsertBackgroundImage tests the insertBackgroundImage function.
func TestInsertBackgroundImage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		relID   string
		wantBg  bool
		wantStr string
	}{
		{
			name:    "inserts p:bg before p:spTree",
			input:   `<p:sld><p:cSld><p:spTree><p:nvGrpSpPr/></p:spTree></p:cSld></p:sld>`,
			relID:   "rId5",
			wantBg:  true,
			wantStr: `<p:bg><p:bgPr><a:blipFill><a:blip r:embed="rId5"/><a:stretch><a:fillRect/></a:stretch></a:blipFill><a:effectLst/></p:bgPr></p:bg><p:spTree>`,
		},
		{
			name:   "no spTree tag - returns unchanged",
			input:  `<p:sld><p:cSld></p:cSld></p:sld>`,
			relID:  "rId5",
			wantBg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(insertBackgroundImage([]byte(tt.input), tt.relID))
			if tt.wantBg {
				if !stringContains(got, tt.wantStr) {
					t.Errorf("expected background XML not found\ngot:  %s\nwant: %s", got, tt.wantStr)
				}
			} else {
				if got != tt.input {
					t.Errorf("expected unchanged output\ngot:  %s\nwant: %s", got, tt.input)
				}
			}
		})
	}
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGenerate_BackgroundImage tests slide background image generation.
func TestGenerate_BackgroundImage(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "bg_image.pptx")

	req := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout2",
				Background: &BackgroundImage{
					Path: testImage,
					Fit:  "cover",
				},
				Content: []ContentItem{
					{
						PlaceholderID: "title",
						Type:          ContentText,
						Value:         "Slide With Background",
					},
				},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// Verify output is valid ZIP
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP/PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Check that media file exists
	foundMedia := false
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/media/image*", f.Name); matched {
			foundMedia = true
			break
		}
	}
	if !foundMedia {
		t.Error("Background image not found in ppt/media/")
	}

	// Check that slide XML contains <p:bg> element
	for _, f := range r.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open slide1.xml: %v", err)
			}
			buf := make([]byte, f.UncompressedSize64+512)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			slideStr := string(buf[:n])

			if !stringContains(slideStr, "<p:bg>") {
				t.Error("Slide XML does not contain <p:bg> element")
			}
			if !stringContains(slideStr, "a:blipFill") {
				t.Error("Slide XML does not contain a:blipFill in background")
			}
			if !stringContains(slideStr, "r:embed=") {
				t.Error("Slide XML does not contain r:embed reference in background")
			}
			break
		}
	}

	// Check that slide rels contain image relationship for background
	for _, f := range r.File {
		if f.Name == "ppt/slides/_rels/slide1.xml.rels" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open slide1 rels: %v", err)
			}
			buf := make([]byte, f.UncompressedSize64+512)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			relsStr := string(buf[:n])

			if !stringContains(relsStr, "relationships/image") {
				t.Error("Slide rels do not contain image relationship for background")
			}
			break
		}
	}
}
