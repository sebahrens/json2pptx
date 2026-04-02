package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerate_ValidPPTX tests AC1: Valid PPTX output
func TestGenerate_ValidPPTX(t *testing.T) {
	// Use the standard.pptx template from testdata
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Create temp output directory
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout1", // Assuming first layout
				Content:  []ContentItem{},
			},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify result
	if result.OutputPath != outputPath {
		t.Errorf("OutputPath = %s, want %s", result.OutputPath, outputPath)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file not created: %v", err)
	}

	// Verify it's a valid ZIP (PPTX is ZIP format)
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Output is not a valid ZIP: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Verify required PPTX structure
	requiredFiles := []string{
		"ppt/presentation.xml",
		"[Content_Types].xml",
		"_rels/.rels",
	}

	fileMap := make(map[string]bool)
	for _, f := range r.File {
		fileMap[f.Name] = true
	}

	for _, required := range requiredFiles {
		if !fileMap[required] {
			t.Errorf("Required file missing: %s", required)
		}
	}
}

// TestGenerate_SlideCount tests AC2: Correct slide count
func TestGenerate_SlideCount(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tests := []struct {
		name       string
		slideCount int
	}{
		{"single slide", 1},
		{"three slides", 3},
		{"ten slides", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "output.pptx")

			// Create N slide specs
			slides := make([]SlideSpec, tt.slideCount)
			for i := 0; i < tt.slideCount; i++ {
				slides[i] = SlideSpec{
					LayoutID: "slideLayout1",
					Content:  []ContentItem{},
				}
			}

			req := GenerationRequest{
				TemplatePath: templatePath,
				OutputPath:   outputPath,
				Slides:       slides,
			}

			result, err := Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Verify slide count in result
			if result.SlideCount != tt.slideCount {
				t.Errorf("SlideCount = %d, want %d", result.SlideCount, tt.slideCount)
			}

			// Verify actual slide files in PPTX
			r, err := zip.OpenReader(outputPath)
			if err != nil {
				t.Fatalf("Failed to open output: %v", err)
			}
			defer func() { _ = r.Close() }()

			slideFiles := 0
			for _, f := range r.File {
				if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
					slideFiles++
				}
			}

			// Note: template may have existing slides, we just verify our slides were added
			if slideFiles < tt.slideCount {
				t.Errorf("Found %d slide files, expected at least %d", slideFiles, tt.slideCount)
			}
		})
	}
}

// TestGenerate_LayoutAccuracy tests AC9: Layout accuracy
func TestGenerate_LayoutAccuracy(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	// Test with specific layout ID
	layoutID := "slideLayout1"
	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: layoutID,
				Content:  []ContentItem{},
			},
		},
	}

	_, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Open output and verify slide uses correct layout
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Read first slide
	var slideData []byte
	for _, f := range r.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open slide1.xml: %v", err)
			}
			slideData, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("Failed to read slide1.xml: %v", err)
			}
			break
		}
	}

	if len(slideData) == 0 {
		t.Fatal("slide1.xml not found")
	}

	// Parse slide XML
	var slide slideXML
	if err := xml.Unmarshal(slideData, &slide); err != nil {
		t.Fatalf("Failed to parse slide XML: %v", err)
	}

	// Verify slide has shape tree (indicating it came from layout)
	if len(slide.CommonSlideData.ShapeTree.Shapes) == 0 {
		t.Error("Slide has no shapes - layout not applied correctly")
	}
}

// TestGenerate_InvalidTemplate tests error handling
func TestGenerate_InvalidTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wantError string
	}{
		{
			name:      "non-existent template",
			template:  "/nonexistent/template.pptx",
			wantError: "template file not found",
		},
		{
			name:      "empty template path",
			template:  "",
			wantError: "template path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "output.pptx")

			req := GenerationRequest{
				TemplatePath: tt.template,
				OutputPath:   outputPath,
				Slides: []SlideSpec{
					{LayoutID: "slideLayout1", Content: []ContentItem{}},
				},
			}

			_, err := Generate(context.Background(), req)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.wantError)
			} else if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Error %q does not contain %q", err.Error(), tt.wantError)
			}
		})
	}
}

// TestGenerate_InvalidOutput tests output validation
func TestGenerate_InvalidOutput(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tests := []struct {
		name      string
		output    string
		wantError string
	}{
		{
			name:      "empty output path",
			output:    "",
			wantError: "output path is required",
		},
		{
			name:      "invalid directory",
			output:    "/nonexistent/directory/output.pptx",
			wantError: "output directory does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := GenerationRequest{
				TemplatePath: templatePath,
				OutputPath:   tt.output,
				Slides: []SlideSpec{
					{LayoutID: "slideLayout1", Content: []ContentItem{}},
				},
			}

			_, err := Generate(context.Background(), req)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.wantError)
			} else if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Error %q does not contain %q", err.Error(), tt.wantError)
			}
		})
	}
}

// TestGenerate_NoSlides tests validation of empty slides
func TestGenerate_NoSlides(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides:       []SlideSpec{}, // Empty slides
	}

	_, err := Generate(context.Background(), req)
	if err == nil {
		t.Error("Expected error for empty slides, got nil")
	} else if !strings.Contains(err.Error(), "at least one slide is required") {
		t.Errorf("Error %q does not contain expected message", err.Error())
	}
}

// TestGenerate_InvalidLayoutID tests handling of invalid layout IDs
func TestGenerate_InvalidLayoutID(t *testing.T) {
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
				LayoutID: "nonExistentLayout999",
				Content:  []ContentItem{},
			},
		},
	}

	_, err := Generate(context.Background(), req)
	if err == nil {
		t.Error("Expected error for invalid layout ID, got nil")
	} else if !strings.Contains(err.Error(), "not found in template") {
		t.Errorf("Error %q does not contain 'not found in template'", err.Error())
	} else if !strings.Contains(err.Error(), "nonExistentLayout999") {
		t.Errorf("Error %q does not contain the invalid layout ID", err.Error())
	} else if !strings.Contains(err.Error(), "available layouts:") {
		t.Errorf("Error %q does not contain 'available layouts:'", err.Error())
	}

	// Verify output file is cleaned up on error
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Error("Output file should be removed on error")
	}
}

// TestGenerate_FileSize tests that file size is reported
func TestGenerate_FileSize(t *testing.T) {
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
			{LayoutID: "slideLayout1", Content: []ContentItem{}},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", result.FileSize)
	}

	// Verify reported size matches actual file
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat output: %v", err)
	}

	if result.FileSize != info.Size() {
		t.Errorf("Reported size %d does not match actual size %d", result.FileSize, info.Size())
	}
}

// TestGenerate_Duration tests that duration is reported
func TestGenerate_Duration(t *testing.T) {
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
			{LayoutID: "slideLayout1", Content: []ContentItem{}},
		},
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

// TestDefaultGenerator_Interface tests that DefaultGenerator implements Generator interface
func TestDefaultGenerator_Interface(t *testing.T) {
	// Verify DefaultGenerator implements Generator interface
	var _ Generator = (*DefaultGenerator)(nil)

	// Create generator via constructor
	gen := NewGenerator()
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
}

// TestDefaultGenerator_Generate tests DefaultGenerator.Generate method
func TestDefaultGenerator_Generate(t *testing.T) {
	// Use the standard.pptx template from testdata
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Create temp output directory
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	gen := NewGenerator()
	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout1",
				Content:  []ContentItem{},
			},
		},
	}

	result, err := gen.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.OutputPath != outputPath {
		t.Errorf("OutputPath = %s, want %s", result.OutputPath, outputPath)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file not created: %v", err)
	}
}
