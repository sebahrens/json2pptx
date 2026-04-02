package generator

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
	"github.com/ahrens/go-slide-creator/internal/utils"
)

// Security tests for path traversal prevention (CRIT-03)

// TestValidateImagePath_PathTraversal tests CRIT-03: Path Traversal Prevention
func TestValidateImagePath_PathTraversal(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "safe relative path",
			path:      "images/photo.png",
			wantError: false,
		},
		{
			name:      "safe absolute path",
			path:      "/tmp/images/photo.png",
			wantError: false,
		},
		{
			name:      "path traversal with forward slashes",
			path:      "../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "hidden path traversal",
			path:      "images/../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "double dots in filename (safe)",
			path:      "images/file..name.png",
			wantError: false,
		},
		{
			name:      "dots at end (safe)",
			path:      "images/file...png",
			wantError: false,
		},
		{
			name:      "empty path",
			path:      "",
			wantError: false,
		},
		{
			name:      "just dots (traversal)",
			path:      "..",
			wantError: true,
		},
		{
			name:      "path traversal in middle",
			path:      "images/../secret.png",
			wantError: true,
		},
		{
			name:      "multiple levels of traversal",
			path:      "a/b/../../../etc/passwd",
			wantError: true,
		},
	}

	// Use config-based validation with nil allowed paths (no sandboxing)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImagePathWithConfig(tt.path, nil)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateImagePathWithConfig(%q, nil) error = %v, wantError %v", tt.path, err, tt.wantError)
			}
		})
	}
}

// TestValidateImagePath_AllowedBasePaths tests sandboxing with base paths
func TestValidateImagePath_AllowedBasePaths(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	disallowedDir := filepath.Join(tmpDir, "disallowed")

	// Create directories
	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed dir: %v", err)
	}
	if err := os.MkdirAll(disallowedDir, 0755); err != nil {
		t.Fatalf("Failed to create disallowed dir: %v", err)
	}

	// Use config-based validation with allowed paths
	allowedPaths := []string{allowedDir}

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "path in allowed directory",
			path:      filepath.Join(allowedDir, "image.png"),
			wantError: false,
		},
		{
			name:      "path in disallowed directory",
			path:      filepath.Join(disallowedDir, "image.png"),
			wantError: true,
		},
		{
			name:      "path in subdirectory of allowed",
			path:      filepath.Join(allowedDir, "subdir", "image.png"),
			wantError: false,
		},
		{
			name:      "path traversal to escape allowed",
			path:      filepath.Join(allowedDir, "..", "disallowed", "image.png"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImagePathWithConfig(tt.path, allowedPaths)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateImagePathWithConfig(%q, %v) error = %v, wantError %v", tt.path, allowedPaths, err, tt.wantError)
			}
		})
	}
}

// TestValidateImagePath_NoBasePaths tests validation when no base paths configured
func TestValidateImagePath_NoBasePaths(t *testing.T) {
	// Use nil allowed paths (no sandboxing)

	// Safe path should pass
	if err := ValidateImagePathWithConfig("/any/path/image.png", nil); err != nil {
		t.Errorf("Expected safe path to pass: %v", err)
	}

	// Path traversal should still fail
	if err := ValidateImagePathWithConfig("../etc/passwd", nil); err == nil {
		t.Error("Expected path traversal to fail")
	}
}

// TestGenerate_PathTraversalBlocked tests CRIT-03 through the Generate API
func TestGenerate_PathTraversalBlocked(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	// Try to use path traversal in image path
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
							Path: "../../../etc/passwd",
							Alt:  "Should be blocked",
						},
					},
				},
			},
		},
	}

	// Generation should succeed but with warning about path traversal
	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// CRIT-03: Path traversal should be blocked with a warning
	foundSecurityWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "security") || strings.Contains(w, "path") || strings.Contains(w, "traversal") {
			foundSecurityWarning = true
			break
		}
	}

	if !foundSecurityWarning {
		t.Log("Warnings:", result.Warnings)
		t.Error("Expected security warning about path traversal")
	}
}

// Image scaling tests

// TestScaleImageToFit tests AC6: Image Scaling
func TestScaleImageToFit(t *testing.T) {
	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	// Test scaling with various bounds
	tests := []struct {
		name   string
		bounds types.BoundingBox
	}{
		{
			name: "standard placeholder size",
			bounds: types.BoundingBox{
				X:      1000000,
				Y:      1000000,
				Width:  4000000,
				Height: 3000000,
			},
		},
		{
			name: "small placeholder",
			bounds: types.BoundingBox{
				X:      0,
				Y:      0,
				Width:  1000000,
				Height: 1000000,
			},
		},
		{
			name: "wide placeholder",
			bounds: types.BoundingBox{
				X:      0,
				Y:      0,
				Width:  8000000,
				Height: 2000000,
			},
		},
		{
			name: "tall placeholder",
			bounds: types.BoundingBox{
				X:      0,
				Y:      0,
				Width:  2000000,
				Height: 8000000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scaleImageToFit(testImage, tt.bounds)
			if err != nil {
				t.Fatalf("scaleImageToFit failed: %v", err)
			}

			// Verify result fits within bounds
			if result.Width > tt.bounds.Width {
				t.Errorf("Result width %d exceeds bounds width %d", result.Width, tt.bounds.Width)
			}
			if result.Height > tt.bounds.Height {
				t.Errorf("Result height %d exceeds bounds height %d", result.Height, tt.bounds.Height)
			}

			// Verify result is centered
			expectedCenterX := tt.bounds.X + tt.bounds.Width/2
			actualCenterX := result.X + result.Width/2
			if abs(expectedCenterX-actualCenterX) > 1 {
				t.Errorf("Result not horizontally centered: expected center %d, got %d", expectedCenterX, actualCenterX)
			}

			expectedCenterY := tt.bounds.Y + tt.bounds.Height/2
			actualCenterY := result.Y + result.Height/2
			if abs(expectedCenterY-actualCenterY) > 1 {
				t.Errorf("Result not vertically centered: expected center %d, got %d", expectedCenterY, actualCenterY)
			}
		})
	}
}

// TestScaleImageToFit_InvalidImage tests error handling for invalid images
func TestScaleImageToFit_InvalidImage(t *testing.T) {
	bounds := types.BoundingBox{
		X: 0, Y: 0, Width: 1000000, Height: 1000000,
	}

	// Test with non-existent file
	_, err := scaleImageToFit("nonexistent.png", bounds)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test with non-image file
	tmpFile, err := os.CreateTemp(t.TempDir(), "notimage-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	_, _ = tmpFile.WriteString("this is not an image")
	_ = tmpFile.Close()

	_, err = scaleImageToFit(tmpFile.Name(), bounds)
	if err == nil {
		t.Error("Expected error for non-image file")
	}
}

// TestScaleImageToFit_AspectRatio tests that aspect ratio is preserved
func TestScaleImageToFit_AspectRatio(t *testing.T) {
	testImage := "testdata/test_image_small.png"
	if _, err := os.Stat(testImage); os.IsNotExist(err) {
		t.Skip("test image not found")
	}

	// Test with significantly different aspect ratio than image
	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  10000000, // Very wide
		Height: 1000000,  // Short
	}

	result, err := scaleImageToFit(testImage, bounds)
	if err != nil {
		t.Fatalf("scaleImageToFit failed: %v", err)
	}

	// Height should be maximized (limited by height bound)
	// Width should be less than bound width to preserve aspect ratio
	if result.Width >= bounds.Width {
		t.Errorf("Width should be limited to preserve aspect ratio, got %d (bound: %d)", result.Width, bounds.Width)
	}
	if result.Height > bounds.Height {
		t.Errorf("Height should not exceed bound, got %d (bound: %d)", result.Height, bounds.Height)
	}
}

// Content Types tests

// TestUpdateContentTypes tests AC-NEW4: Content-Types Update
func TestUpdateContentTypes(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Open template ZIP
	r, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = r.Close() }()
	idx := utils.BuildZipIndex(&r.Reader)

	// Test adding new extensions
	usedExtensions := map[string]bool{
		"png":  true, // Likely already exists
		"webp": true, // Likely needs to be added
	}

	result, err := updateContentTypes(idx, usedExtensions)
	if err != nil {
		t.Fatalf("updateContentTypes failed: %v", err)
	}

	// Verify result is valid XML
	if !strings.Contains(string(result), "<?xml") {
		t.Error("Result missing XML header")
	}

	// Verify Types root element
	if !strings.Contains(string(result), "<Types") {
		t.Error("Result missing Types element")
	}

	// Verify extension mapping for webp was added
	if !strings.Contains(string(result), "webp") || !strings.Contains(string(result), "image/webp") {
		t.Error("Expected webp content type to be added")
	}
}

// TestUpdateContentTypes_NoNewExtensions tests when no new extensions needed
func TestUpdateContentTypes_NoNewExtensions(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	r, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = r.Close() }()

	idx := utils.BuildZipIndex(&r.Reader)

	// Empty extensions map
	usedExtensions := map[string]bool{}

	result, err := updateContentTypes(idx, usedExtensions)
	if err != nil {
		t.Fatalf("updateContentTypes failed: %v", err)
	}

	// Result should still be valid XML
	if !strings.Contains(string(result), "<?xml") {
		t.Error("Result missing XML header")
	}
}

// Helper function for absolute value
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
