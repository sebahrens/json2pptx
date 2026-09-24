package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPlaceholderAndContrastPreflightPathsResolveAgainstAuthoredDeck(t *testing.T) {
	slide := denseBulletsSlide("layout1", 14)
	slide.Background = &BackgroundInput{Color: "#000000"}
	input := &PresentationInput{Slides: []SlideInput{slide}}
	layout := types.LayoutMetadata{ID: "layout1", Placeholders: []types.PlaceholderInfo{{
		ID: "body", Type: types.PlaceholderBody, FontSize: 1200,
		FontColor: "#111111", FontFamily: "Calibri",
		Bounds: types.BoundingBox{X: 800000, Y: 1500000, Width: 5000000, Height: 1000000},
	}}}
	placeholder := checkPlaceholderFindings(&input.Slides[0], 0, &layout)
	if len(placeholder) == 0 {
		t.Fatal("dense body did not produce a placeholder finding")
	}
	for _, finding := range placeholder {
		if finding.Path != "/slides/0/content/1" {
			t.Errorf("placeholder finding path = %q, want authored body index", finding.Path)
		}
		assertFindingPointerReplaceable(t, input, finding.Path)
	}

	pairs := authorBackgroundContrastPairs(input, []types.LayoutMetadata{layout}, nil)
	if len(pairs) == 0 {
		t.Fatal("author background did not produce a contrast preflight pair")
	}
	for _, pair := range pairs {
		if pair.Path != "/slides/0/content/1" {
			t.Errorf("contrast preflight path = %q, want authored body index", pair.Path)
		}
		assertFindingPointerReplaceable(t, input, pair.Path)
	}
}
