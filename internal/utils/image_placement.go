package utils

import (
	"image"
	"os"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Image placement modes understood by PlaceImage.
const (
	// PlaceCover fills the frame and centre-crops the long axis (a:srcRect).
	PlaceCover = "cover"
	// PlaceContain keeps the whole picture, scaled to fit and centred.
	PlaceContain = "contain"
	// PlaceContainStart keeps the whole picture, scaled to fit and anchored
	// to the frame's top-left corner.
	PlaceContainStart = "contain-start"
	// PlaceStretch fills the frame without preserving the aspect ratio.
	PlaceStretch = "stretch"
)

// ImagePlacement is the transform from a source image onto a slide frame: the
// rectangle the picture occupies on the slide and the a:srcRect crop applied
// to it. Picture insertion and anything pointing INTO a picture (overlay
// callouts targeting a source-image point) derive from the same value, so a
// target stays on its pixel when the frame, fit or template changes.
type ImagePlacement struct {
	Bounds types.BoundingBox // picture frame on the slide, in EMU
	Crop   CropRect          // source share trimmed from each edge
	// PixelW / PixelH are the intrinsic image size; zero when it could not
	// be read (the picture is then stretched to Bounds).
	PixelW, PixelH int
}

// ImagePixelSize returns the intrinsic pixel size of a raster image file.
func ImagePixelSize(path string) (int, int, error) {
	f, err := os.Open(path) //nolint:gosec // path validated by the caller
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// PlaceImage places an imgW × imgH pixel image in frame with the given fit
// (PlaceCover, PlaceContain, PlaceContainStart or PlaceStretch; any other
// value stretches). An unknown image size stretches.
func PlaceImage(imgW, imgH int, frame types.BoundingBox, fit string) ImagePlacement {
	p := ImagePlacement{Bounds: frame, PixelW: imgW, PixelH: imgH}
	if imgW <= 0 || imgH <= 0 {
		p.PixelW, p.PixelH = 0, 0
		return p
	}
	switch fit {
	case PlaceCover:
		p.Crop = CoverCrop(imgW, imgH, frame.Width, frame.Height)
	case PlaceContain, PlaceContainStart:
		p.Bounds = containBounds(imgW, imgH, frame)
		if fit == PlaceContainStart {
			p.Bounds.X, p.Bounds.Y = frame.X, frame.Y
		}
	}
	return p
}

// containBounds scales the image to fit within frame, preserving its aspect
// ratio, and centres it.
func containBounds(imgW, imgH int, frame types.BoundingBox) types.BoundingBox {
	imgWidthEMU := int64(imgW) * EMUsPerPixel
	imgHeightEMU := int64(imgH) * EMUsPerPixel

	scaleX := float64(frame.Width) / float64(imgWidthEMU)
	scaleY := float64(frame.Height) / float64(imgHeightEMU)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	newWidth := int64(float64(imgWidthEMU) * scale)
	newHeight := int64(float64(imgHeightEMU) * scale)
	return types.BoundingBox{
		X:      frame.X + (frame.Width-newWidth)/2,
		Y:      frame.Y + (frame.Height-newHeight)/2,
		Width:  newWidth,
		Height: newHeight,
	}
}

// VisibleSource returns the part of the source image the placement shows, as
// fractions of its width (x0..x1) and height (y0..y1).
func (p ImagePlacement) VisibleSource() (x0, x1, y0, y1 float64) {
	return float64(p.Crop.L) / 100000, 1 - float64(p.Crop.R)/100000,
		float64(p.Crop.T) / 100000, 1 - float64(p.Crop.B)/100000
}

// visibleSlack absorbs float rounding at the crop edge (1/100 000 of the
// source is the srcRect resolution).
const visibleSlack = 1e-5

// SourceToSlide maps a source-image point, as fractions of the image width
// (u) and height (v), to slide EMU. visible is false when the crop trims the
// point away; x, y are then clamped to the nearest visible picture edge.
func (p ImagePlacement) SourceToSlide(u, v float64) (x, y int64, visible bool) {
	x0, x1, y0, y1 := p.VisibleSource()
	visible = u >= x0-visibleSlack && u <= x1+visibleSlack && v >= y0-visibleSlack && v <= y1+visibleSlack
	fx := clampUnit((u - x0) / (x1 - x0))
	fy := clampUnit((v - y0) / (y1 - y0))
	x = p.Bounds.X + int64(fx*float64(p.Bounds.Width)+0.5)
	y = p.Bounds.Y + int64(fy*float64(p.Bounds.Height)+0.5)
	return x, y, visible
}

func clampUnit(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
