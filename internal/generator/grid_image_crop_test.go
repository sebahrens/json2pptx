package generator

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestGridImageCoverCrop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tall.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 100, 200))); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	crop := gridImageCoverCrop(ImageInsert{Path: path, ExtentCX: 200, ExtentCY: 100})
	if crop == nil || crop.T == 0 || crop.B == 0 || crop.L != 0 || crop.R != 0 {
		t.Fatalf("1:2 image in a 2:1 frame should be cropped top/bottom, got %+v", crop)
	}
	if got := gridImageCoverCrop(ImageInsert{Path: path, ExtentCX: 100, ExtentCY: 200}); got != nil {
		t.Errorf("matching aspect needs no crop, got %+v", got)
	}
	if got := gridImageCoverCrop(ImageInsert{Path: filepath.Join(t.TempDir(), "none.png"), ExtentCX: 1, ExtentCY: 1}); got != nil {
		t.Errorf("unreadable image keeps the stretch, got %+v", got)
	}
}
