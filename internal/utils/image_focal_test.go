package utils

import (
	"github.com/sebahrens/json2pptx/internal/types"
	"testing"
)

func TestCoverFocalPointPreservesSubjectAndDefault(t *testing.T) {
	frame := types.BoundingBox{Width: 300 * 12700, Height: 300 * 12700}
	for _, dims := range [][2]int{{1600, 900}, {900, 1600}} {
		center := PlaceImage(dims[0], dims[1], frame, PlaceCover)
		explicit := PlaceImage(dims[0], dims[1], frame, PlaceCover, &types.ImageFocalPoint{X: .5, Y: .5})
		if center != explicit {
			t.Fatalf("explicit center changed bytes: %+v vs %+v", center, explicit)
		}
		for _, point := range []*types.ImageFocalPoint{{X: .1, Y: .1}, {X: .9, Y: .9}, {X: 0, Y: 1}} {
			p := PlaceImage(dims[0], dims[1], frame, PlaceCover, point)
			if _, _, visible := p.SourceToSlide(point.X, point.Y); !visible {
				t.Errorf("subject %+v cropped from %v: %+v", point, dims, p)
			}
			if p.Bounds != center.Bounds {
				t.Fatal("focal crop changed frame")
			}
			contained := PlaceImage(dims[0], dims[1], frame, PlaceContain, point)
			if !contained.Crop.IsZero() {
				t.Fatal("focal point cropped a contain image")
			}
		}
	}
}
