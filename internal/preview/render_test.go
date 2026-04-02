package preview

import (
	"image/png"
	"bytes"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestRenderSlidePNG_Basic(t *testing.T) {
	slide := types.SlideDefinition{
		Title: "Hello World",
		Type:  types.SlideTypeTitle,
	}

	data, err := RenderSlidePNG(slide, nil, 96)
	if err != nil {
		t.Fatalf("RenderSlidePNG failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}

	// Verify it's valid PNG
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Error("PNG has zero dimensions")
	}
}

func TestRenderSlidePNG_WithTheme(t *testing.T) {
	slide := types.SlideDefinition{
		Title: "Themed Slide",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"Point A", "Point B"},
		},
	}
	theme := &types.ThemeInfo{
		Colors: []types.ThemeColor{
			{Name: "accent1", RGB: "#E94560"},
		},
	}

	data, err := RenderSlidePNG(slide, theme, 0) // default DPI
	if err != nil {
		t.Fatalf("RenderSlidePNG failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}
