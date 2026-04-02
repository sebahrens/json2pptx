package utils

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// createTestImage creates a test PNG image with the given dimensions.
func createTestImage(t *testing.T, path string, width, height int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}
	defer func() { _ = f.Close() }()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}
}

func TestScaleImageToFit_FitWidth(t *testing.T) {
	// Create a wide image that needs to fit into a tall placeholder
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "wide.png")

	// Image: 200x100 pixels (wide)
	createTestImage(t, imgPath, 200, 100)

	// Placeholder: 1000000x2000000 EMUs (tall)
	// With EMUsPerPixel = 9525, 200px = 1905000 EMU, 100px = 952500 EMU
	// Scale to fit width: 1000000 / 1905000 ≈ 0.525
	// Resulting height: 952500 * 0.525 ≈ 500000 EMU
	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  1000000,
		Height: 2000000,
	}

	result, err := ScaleImageToFit(imgPath, bounds)
	if err != nil {
		t.Fatalf("ScaleImageToFit() returned error: %v", err)
	}

	// Width should match placeholder width
	if result.Width != bounds.Width {
		t.Errorf("ScaleImageToFit() width = %d, want %d", result.Width, bounds.Width)
	}

	// Height should be smaller than placeholder (aspect ratio preserved)
	if result.Height >= bounds.Height {
		t.Errorf("ScaleImageToFit() height = %d, should be < %d", result.Height, bounds.Height)
	}

	// Aspect ratio preserved (2:1 for 200x100)
	aspectRatio := float64(result.Width) / float64(result.Height)
	expectedRatio := 2.0
	if diff := aspectRatio - expectedRatio; diff > 0.01 || diff < -0.01 {
		t.Errorf("ScaleImageToFit() aspect ratio = %f, want ≈ %f", aspectRatio, expectedRatio)
	}
}

func TestScaleImageToFit_FitHeight(t *testing.T) {
	// Create a tall image that needs to fit into a wide placeholder
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "tall.png")

	// Image: 100x200 pixels (tall)
	createTestImage(t, imgPath, 100, 200)

	// Placeholder: 2000000x1000000 EMUs (wide)
	// With EMUsPerPixel = 9525, 100px = 952500 EMU, 200px = 1905000 EMU
	// Scale to fit height: 1000000 / 1905000 ≈ 0.525
	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  2000000,
		Height: 1000000,
	}

	result, err := ScaleImageToFit(imgPath, bounds)
	if err != nil {
		t.Fatalf("ScaleImageToFit() returned error: %v", err)
	}

	// Height should match placeholder height
	if result.Height != bounds.Height {
		t.Errorf("ScaleImageToFit() height = %d, want %d", result.Height, bounds.Height)
	}

	// Width should be smaller than placeholder
	if result.Width >= bounds.Width {
		t.Errorf("ScaleImageToFit() width = %d, should be < %d", result.Width, bounds.Width)
	}

	// Aspect ratio preserved (1:2 for 100x200)
	aspectRatio := float64(result.Width) / float64(result.Height)
	expectedRatio := 0.5
	if diff := aspectRatio - expectedRatio; diff > 0.01 || diff < -0.01 {
		t.Errorf("ScaleImageToFit() aspect ratio = %f, want ≈ %f", aspectRatio, expectedRatio)
	}
}

func TestScaleImageToFit_SquareImage(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "square.png")

	// Square image: 100x100 pixels
	createTestImage(t, imgPath, 100, 100)

	// Square placeholder
	bounds := types.BoundingBox{
		X:      1000,
		Y:      2000,
		Width:  500000,
		Height: 500000,
	}

	result, err := ScaleImageToFit(imgPath, bounds)
	if err != nil {
		t.Fatalf("ScaleImageToFit() returned error: %v", err)
	}

	// Should fill the placeholder exactly
	if result.Width != bounds.Width || result.Height != bounds.Height {
		t.Errorf("ScaleImageToFit() = %dx%d, want %dx%d",
			result.Width, result.Height, bounds.Width, bounds.Height)
	}

	// Position should match (no offset needed when exactly fitting)
	if result.X != bounds.X || result.Y != bounds.Y {
		t.Errorf("ScaleImageToFit() position = (%d, %d), want (%d, %d)",
			result.X, result.Y, bounds.X, bounds.Y)
	}
}

func TestScaleImageToFit_Centering(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")

	// Wide image: 200x100 pixels
	createTestImage(t, imgPath, 200, 100)

	// Tall placeholder
	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  1000000,
		Height: 2000000,
	}

	result, err := ScaleImageToFit(imgPath, bounds)
	if err != nil {
		t.Fatalf("ScaleImageToFit() returned error: %v", err)
	}

	// Image should be centered vertically
	// Expected: Y offset = (2000000 - resultHeight) / 2
	expectedY := (bounds.Height - result.Height) / 2
	if result.Y != expectedY {
		t.Errorf("ScaleImageToFit() Y = %d, want %d (centered)", result.Y, expectedY)
	}

	// X should be at placeholder X (no horizontal offset when width matches)
	if result.X != bounds.X {
		t.Errorf("ScaleImageToFit() X = %d, want %d", result.X, bounds.X)
	}
}

func TestScaleImageToFit_WithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")

	createTestImage(t, imgPath, 100, 100)

	// Placeholder with offset
	bounds := types.BoundingBox{
		X:      500000,
		Y:      300000,
		Width:  400000,
		Height: 400000,
	}

	result, err := ScaleImageToFit(imgPath, bounds)
	if err != nil {
		t.Fatalf("ScaleImageToFit() returned error: %v", err)
	}

	// Result should be relative to placeholder position
	if result.X < bounds.X || result.Y < bounds.Y {
		t.Errorf("ScaleImageToFit() position (%d, %d) should be >= placeholder position (%d, %d)",
			result.X, result.Y, bounds.X, bounds.Y)
	}

	// Result should be within bounds
	if result.X+result.Width > bounds.X+bounds.Width {
		t.Error("ScaleImageToFit() result exceeds placeholder width")
	}
	if result.Y+result.Height > bounds.Y+bounds.Height {
		t.Error("ScaleImageToFit() result exceeds placeholder height")
	}
}

func TestScaleImageToFit_FileNotFound(t *testing.T) {
	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  1000000,
		Height: 1000000,
	}

	_, err := ScaleImageToFit("/nonexistent/path/image.png", bounds)
	if err == nil {
		t.Error("ScaleImageToFit() with nonexistent file should return error")
	}
}

func TestScaleImageToFit_InvalidImage(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.png")

	// Create a file that's not a valid image
	if err := os.WriteFile(invalidPath, []byte("not an image"), 0644); err != nil {
		t.Fatalf("failed to create invalid file: %v", err)
	}

	bounds := types.BoundingBox{
		X:      0,
		Y:      0,
		Width:  1000000,
		Height: 1000000,
	}

	_, err := ScaleImageToFit(invalidPath, bounds)
	if err == nil {
		t.Error("ScaleImageToFit() with invalid image should return error")
	}
}

func TestEMUsPerPixel(t *testing.T) {
	// Verify the constant is correct: 914400 EMUs/inch ÷ 96 pixels/inch = 9525 EMUs/pixel
	expected := int64(914400 / 96)
	if EMUsPerPixel != expected {
		t.Errorf("EMUsPerPixel = %d, want %d", EMUsPerPixel, expected)
	}
}
