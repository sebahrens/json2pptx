package utils

import (
	"image"
	"os"
)

// CropRect is an OOXML a:srcRect crop in thousandths of a percent of the
// source image (100000 = 100%): how much to trim from each edge.
type CropRect struct {
	L, T, R, B int
}

// IsZero reports whether the crop trims nothing.
func (c CropRect) IsZero() bool { return c == CropRect{} }

// CoverCrop returns the centred crop that makes an image of the given pixel
// size fill a boxW × boxH frame without distortion ("object-fit: cover"):
// the axis whose aspect is too long is trimmed equally on both sides.
func CoverCrop(imgW, imgH int, boxW, boxH int64) CropRect {
	if imgW <= 0 || imgH <= 0 || boxW <= 0 || boxH <= 0 {
		return CropRect{}
	}
	imgAspect := float64(imgW) / float64(imgH)
	boxAspect := float64(boxW) / float64(boxH)
	switch {
	case imgAspect > boxAspect*1.001: // too wide: trim left / right
		keep := boxAspect / imgAspect
		trim := int((1 - keep) / 2 * 100000)
		return CropRect{L: trim, R: trim}
	case imgAspect < boxAspect/1.001: // too tall: trim top / bottom
		keep := imgAspect / boxAspect
		trim := int((1 - keep) / 2 * 100000)
		return CropRect{T: trim, B: trim}
	default:
		return CropRect{}
	}
}

// CoverCropForFile reads the pixel size of a PNG / JPEG image and returns
// its cover crop for a boxW × boxH frame. ok is false when the image cannot
// be decoded (the caller then keeps the plain stretch).
func CoverCropForFile(imagePath string, boxW, boxH int64) (CropRect, bool) {
	f, err := os.Open(imagePath) //nolint:gosec // path validated by the caller
	if err != nil {
		return CropRect{}, false
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return CropRect{}, false
	}
	return CoverCrop(cfg.Width, cfg.Height, boxW, boxH), true
}
