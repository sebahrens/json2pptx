package utils

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
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

func TestScaleImageToFit_RectangularImages(t *testing.T) {
	tests := []struct {
		name                    string
		imageWidth, imageHeight int
		bounds                  types.BoundingBox
		constrainedAxis         string
		wantAspect              float64
	}{
		{
			name:       "fit width",
			imageWidth: 200, imageHeight: 100,
			bounds:          types.BoundingBox{Width: 1000000, Height: 2000000},
			constrainedAxis: "width", wantAspect: 2.0,
		},
		{
			name:       "fit height",
			imageWidth: 100, imageHeight: 200,
			bounds:          types.BoundingBox{Width: 2000000, Height: 1000000},
			constrainedAxis: "height", wantAspect: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgPath := filepath.Join(t.TempDir(), "image.png")
			createTestImage(t, imgPath, tt.imageWidth, tt.imageHeight)

			result, err := ScaleImageToFit(imgPath, tt.bounds)
			if err != nil {
				t.Fatalf("ScaleImageToFit() returned error: %v", err)
			}
			switch tt.constrainedAxis {
			case "width":
				if result.Width != tt.bounds.Width || result.Height >= tt.bounds.Height {
					t.Errorf("width fit = %dx%d inside %dx%d", result.Width, result.Height, tt.bounds.Width, tt.bounds.Height)
				}
			case "height":
				if result.Height != tt.bounds.Height || result.Width >= tt.bounds.Width {
					t.Errorf("height fit = %dx%d inside %dx%d", result.Width, result.Height, tt.bounds.Width, tt.bounds.Height)
				}
			}

			aspect := float64(result.Width) / float64(result.Height)
			if diff := aspect - tt.wantAspect; diff > 0.01 || diff < -0.01 {
				t.Errorf("aspect ratio = %f, want ≈ %f", aspect, tt.wantAspect)
			}
		})
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
