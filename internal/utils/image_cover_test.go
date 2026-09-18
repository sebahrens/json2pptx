package utils

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverCrop(t *testing.T) {
	cases := []struct {
		name       string
		iw, ih     int
		bw, bh     int64
		want       CropRect
		approxTrim bool
	}{
		{name: "same aspect", iw: 1600, ih: 900, bw: 1600, bh: 900, want: CropRect{}},
		{name: "wide image in square frame", iw: 1200, ih: 800, bw: 100, bh: 100, want: CropRect{L: 16666, R: 16666}},
		{name: "tall image in square frame", iw: 800, ih: 1200, bw: 100, bh: 100, want: CropRect{T: 16666, B: 16666}},
		{name: "invalid", iw: 0, ih: 10, bw: 10, bh: 10, want: CropRect{}},
	}
	for _, tc := range cases {
		got := CoverCrop(tc.iw, tc.ih, tc.bw, tc.bh)
		if got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
	if !(CropRect{}).IsZero() || (CropRect{L: 1}).IsZero() {
		t.Error("IsZero mismatch")
	}
}

func TestCoverCropForFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wide.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 300, 100))); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	c, ok := CoverCropForFile(path, 100, 100)
	if !ok || c.L != 33333 || c.R != 33333 || c.T != 0 {
		t.Errorf("3:1 image in a square frame: got %+v ok=%v", c, ok)
	}
	if _, ok := CoverCropForFile(filepath.Join(t.TempDir(), "missing.png"), 1, 1); ok {
		t.Error("missing file should report !ok")
	}
}
