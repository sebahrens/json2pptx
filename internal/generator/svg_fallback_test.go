package generator

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestSVGFallbackImage(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard_size", 400, 300},
		{"small_size", 100, 50},
		{"very_small_size", 20, 20}, // Should clamp to minimum
		{"wide_aspect", 800, 200},
		{"tall_aspect", 200, 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgBytes, err := SVGFallbackImage(tt.width, tt.height)
			if err != nil {
				t.Fatalf("SVGFallbackImage(%d, %d) error = %v", tt.width, tt.height, err)
			}

			if len(imgBytes) == 0 {
				t.Fatal("SVGFallbackImage returned empty bytes")
			}

			// Verify it's a valid PNG
			img, err := png.Decode(bytes.NewReader(imgBytes))
			if err != nil {
				t.Fatalf("SVGFallbackImage output is not valid PNG: %v", err)
			}

			bounds := img.Bounds()
			actualWidth := bounds.Dx()
			actualHeight := bounds.Dy()

			// For very small inputs, minimum size is enforced
			expectedWidth := tt.width
			if expectedWidth < 100 {
				expectedWidth = 100
			}
			expectedHeight := tt.height
			if expectedHeight < 50 {
				expectedHeight = 50
			}

			if actualWidth != expectedWidth {
				t.Errorf("width = %d, want %d", actualWidth, expectedWidth)
			}
			if actualHeight != expectedHeight {
				t.Errorf("height = %d, want %d", actualHeight, expectedHeight)
			}

			// Verify the image has reasonable content (not all same color)
			if isUniformImage(img) {
				t.Error("SVGFallbackImage appears to be uniformly colored, expected text/icon content")
			}
		})
	}
}

// isUniformImage checks if all pixels in the image are the same color.
func isUniformImage(img image.Image) bool {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return true
	}

	// Sample first pixel as reference
	refR, refG, refB, refA := img.At(bounds.Min.X, bounds.Min.Y).RGBA()

	// Check a sample of pixels (not all to keep test fast)
	step := 10
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			if r != refR || g != refG || b != refB || a != refA {
				return false
			}
		}
	}
	return true
}

func TestDiagramPlaceholderImage(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		diagramType string
	}{
		{"bar_chart", 400, 300, "bar_chart"},
		{"gantt", 600, 400, "gantt"},
		{"empty_type", 400, 300, ""},
		{"small_size", 20, 20, "matrix_2x2"},
		{"wide_aspect", 800, 200, "timeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgBytes, err := DiagramPlaceholderImage(tt.width, tt.height, tt.diagramType)
			if err != nil {
				t.Fatalf("DiagramPlaceholderImage error = %v", err)
			}

			if len(imgBytes) == 0 {
				t.Fatal("returned empty bytes")
			}

			img, err := png.Decode(bytes.NewReader(imgBytes))
			if err != nil {
				t.Fatalf("not valid PNG: %v", err)
			}

			bounds := img.Bounds()
			expectedWidth := tt.width
			if expectedWidth < 100 {
				expectedWidth = 100
			}
			expectedHeight := tt.height
			if expectedHeight < 50 {
				expectedHeight = 50
			}

			if bounds.Dx() != expectedWidth {
				t.Errorf("width = %d, want %d", bounds.Dx(), expectedWidth)
			}
			if bounds.Dy() != expectedHeight {
				t.Errorf("height = %d, want %d", bounds.Dy(), expectedHeight)
			}

			if isUniformImage(img) {
				t.Error("image appears uniform, expected visual content")
			}
		})
	}
}

func TestFormatDiagramType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bar_chart", "Bar Chart"},
		{"matrix_2x2", "Matrix 2x2"},
		{"gantt", "Gantt"},
		{"kpi_dashboard", "Kpi Dashboard"},
		{"porters_five_forces", "Porters Five Forces"},
		{"", "Diagram"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatDiagramType(tt.input)
			if got != tt.want {
				t.Errorf("formatDiagramType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInsertDiagramPlaceholder(t *testing.T) {
	ctx := newSinglePassContext("/tmp/test.pptx", nil, nil, false, nil)

	bounds := types.BoundingBox{
		X:      914400,
		Y:      914400,
		Width:  914400 * 4,
		Height: 914400 * 3,
	}

	slideNum := 1
	shapeIdx := 2

	ctx.insertDiagramPlaceholder(slideNum, bounds, shapeIdx, "bar_chart")

	if len(ctx.slideRelUpdates[slideNum]) != 1 {
		t.Fatalf("expected 1 media relationship, got %d", len(ctx.slideRelUpdates[slideNum]))
	}

	rel := ctx.slideRelUpdates[slideNum][0]

	if rel.mediaFileName == "" {
		t.Error("media filename is empty")
	}
	if len(rel.data) == 0 {
		t.Error("media data is empty")
	}
	if rel.placeholderIdx != shapeIdx {
		t.Errorf("placeholderIdx = %d, want %d", rel.placeholderIdx, shapeIdx)
	}
	if rel.offsetX != bounds.X || rel.offsetY != bounds.Y {
		t.Error("offset mismatch")
	}
	if rel.extentCX != bounds.Width || rel.extentCY != bounds.Height {
		t.Error("extent mismatch")
	}
	if !ctx.usedExtensions["png"] {
		t.Error("png extension not tracked")
	}

	// Verify valid PNG
	_, err := png.Decode(bytes.NewReader(rel.data))
	if err != nil {
		t.Errorf("embedded data is not valid PNG: %v", err)
	}
}

func TestInsertSVGFallbackImage(t *testing.T) {
	// Create a mock context
	ctx := newSinglePassContext("/tmp/test.pptx", nil, nil, false, nil)

	// Set up bounds similar to a real placeholder (in EMUs)
	// 4 inches wide x 3 inches tall at 914400 EMU/inch
	bounds := types.BoundingBox{
		X:      914400,           // 1 inch from left
		Y:      914400,           // 1 inch from top
		Width:  914400 * 4,       // 4 inches wide
		Height: 914400 * 3,       // 3 inches tall
	}

	slideNum := 1
	shapeIdx := 0

	// Insert fallback
	ctx.insertSVGFallbackImage(slideNum, bounds, shapeIdx)

	// Verify media relationship was created
	if len(ctx.slideRelUpdates[slideNum]) != 1 {
		t.Fatalf("expected 1 media relationship, got %d", len(ctx.slideRelUpdates[slideNum]))
	}

	rel := ctx.slideRelUpdates[slideNum][0]

	if rel.mediaFileName == "" {
		t.Error("media filename is empty")
	}

	if len(rel.data) == 0 {
		t.Error("media data is empty")
	}

	if rel.offsetX != bounds.X {
		t.Errorf("offsetX = %d, want %d", rel.offsetX, bounds.X)
	}

	if rel.offsetY != bounds.Y {
		t.Errorf("offsetY = %d, want %d", rel.offsetY, bounds.Y)
	}

	if rel.extentCX != bounds.Width {
		t.Errorf("extentCX = %d, want %d", rel.extentCX, bounds.Width)
	}

	if rel.extentCY != bounds.Height {
		t.Errorf("extentCY = %d, want %d", rel.extentCY, bounds.Height)
	}

	if rel.placeholderIdx != shapeIdx {
		t.Errorf("placeholderIdx = %d, want %d", rel.placeholderIdx, shapeIdx)
	}

	// Verify PNG extension was tracked
	if !ctx.usedExtensions["png"] {
		t.Error("png extension was not tracked")
	}

	// Verify the embedded data is a valid PNG
	_, err := png.Decode(bytes.NewReader(rel.data))
	if err != nil {
		t.Errorf("embedded data is not valid PNG: %v", err)
	}
}
