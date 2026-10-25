package generator

import (
	"github.com/sebahrens/json2pptx/internal/types"
	"math"
	"testing"
)

func TestGridAndPlaceholderFocalPlacementAgree(t *testing.T) {
	path := writeGridTestPNG(t, 1600, 900)
	f := &types.ImageFocalPoint{X: .1, Y: .5}
	frame := types.BoundingBox{Width: 300 * 12700, Height: 300 * 12700}
	p := GridImagePlacement(path, frame, "cover", f)
	bounds, crop, err := imagePlacement(path, frame, "cover", f)
	if err != nil {
		t.Fatal(err)
	}
	if bounds != p.Bounds || crop == nil || crop.L != p.Crop.L || crop.R != p.Crop.R {
		t.Fatalf("placement differs: %+v %+v %+v", p, bounds, crop)
	}
	_, insertCrop := gridImagePlacement(ImageInsert{Path: path, Geometry: "ellipse", Fit: "cover", FocalPoint: f, ExtentCX: frame.Width, ExtentCY: frame.Height})
	if insertCrop == nil || *insertCrop != *crop {
		t.Fatal("ellipse placement lost focal point")
	}
	if _, _, visible := p.SourceToSlide(.1, .5); !visible {
		t.Fatal("callout lost its focal subject")
	}
	for _, invalid := range []*types.ImageFocalPoint{{X: -.1, Y: .5}, {X: .5, Y: 2}, {X: math.NaN(), Y: .5}} {
		if _, _, err := imagePlacement(path, frame, "cover", invalid); err == nil {
			t.Errorf("invalid focal point accepted: %+v", invalid)
		}
	}
}
