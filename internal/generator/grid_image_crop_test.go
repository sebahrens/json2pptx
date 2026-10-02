package generator

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func writeGridTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "img.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return path
}

func TestGridImageCoverCrop(t *testing.T) {
	path := writeGridTestPNG(t, 100, 200)

	_, crop := gridImagePlacement(ImageInsert{Path: path, ExtentCX: 200, ExtentCY: 100})
	if crop == nil || crop.T == 0 || crop.B == 0 || crop.L != 0 || crop.R != 0 {
		t.Fatalf("1:2 image in a 2:1 frame should be cropped top/bottom, got %+v", crop)
	}
	if _, got := gridImagePlacement(ImageInsert{Path: path, ExtentCX: 100, ExtentCY: 200}); got != nil {
		t.Errorf("matching aspect needs no crop, got %+v", got)
	}
	bounds, got := gridImagePlacement(ImageInsert{Path: filepath.Join(t.TempDir(), "none.png"), ExtentCX: 1, ExtentCY: 1})
	if got != nil || bounds.Width != 1 || bounds.Height != 1 {
		t.Errorf("unreadable image keeps the stretch, got %+v %+v", bounds, got)
	}
}

// TestGridImageContainPlacement: fit "contain" keeps the whole picture,
// centred, with no crop (go-slide-creator-kkc5t).
func TestGridImageContainPlacement(t *testing.T) {
	path := writeGridTestPNG(t, 1600, 900)
	bounds, crop := gridImagePlacement(ImageInsert{Path: path, Fit: "contain", OffsetX: 1000, OffsetY: 2000, ExtentCX: 900000, ExtentCY: 900000})
	if crop != nil {
		t.Fatalf("contain must not crop, got %+v", crop)
	}
	if bounds.Width != 900000 || bounds.Height != 506250 || bounds.X != 1000 || bounds.Y != 2000+(900000-506250)/2 {
		t.Errorf("contain bounds = %+v", bounds)
	}
}

// TestGridImagePlacementMapsSourcePoints pins the bead's geometry evidence: a
// 1600x900 screenshot cover-filled into a square shows source x in
// [0.21875, 0.78125], so source x=0.25 lands at ~5.56% of the frame width.
func TestGridImagePlacementMapsSourcePoints(t *testing.T) {
	path := writeGridTestPNG(t, 1600, 900)
	frame := types.BoundingBox{X: 0, Y: 0, Width: 900000, Height: 900000}

	square := GridImagePlacement(path, frame, "")
	x0, x1, y0, y1 := square.VisibleSource()
	if x0 < 0.2187 || x0 > 0.2188 || x1 < 0.7812 || x1 > 0.7813 || y0 != 0 || y1 != 1 {
		t.Fatalf("visible source = [%g,%g]x[%g,%g]", x0, x1, y0, y1)
	}
	x, y, ok := square.SourceToSlide(0.25, 0.5)
	if !ok || x < 49900 || x > 50100 || y != 450000 {
		t.Errorf("square cover: (0.25,0.5) -> (%d,%d) visible=%v; want ~(50000,450000)", x, y, ok)
	}
	if _, _, ok := square.SourceToSlide(0.1, 0.5); ok {
		t.Error("source x=0.1 is cropped away in a square frame and must report not visible")
	}

	wide := GridImagePlacement(path, types.BoundingBox{Width: 1600000, Height: 900000}, "cover")
	if x, _, ok := wide.SourceToSlide(0.1, 0.5); !ok || x != 160000 {
		t.Errorf("matching-aspect frame: x=0.1 -> %d visible=%v; want 160000", x, ok)
	}

	contain := GridImagePlacement(path, frame, "contain")
	if x, y, ok := contain.SourceToSlide(0.1, 0); !ok || x != 90000 || y != contain.Bounds.Y {
		t.Errorf("contain: (0.1,0) -> (%d,%d) visible=%v", x, y, ok)
	}
}
