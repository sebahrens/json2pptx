package generator

import (
	"math"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// GridImagePlacement returns where a shape_grid image cell's picture lands
// inside its frame and how it is cropped. fit is the cell's image fit:
// "cover" (also the default) cover-fills the frame — the picture keeps its
// aspect ratio and is centre-cropped instead of being stretched — and
// "contain" keeps the whole picture centred in the frame. An image whose
// size cannot be read is stretched to the frame.
//
// Picture insertion and the overlay resolver (anchor_image targets) both call
// this, so a callout endpoint is computed with exactly the transform that
// places the picture.
func GridImagePlacement(path string, frame types.BoundingBox, fit string) utils.ImagePlacement {
	mode := utils.PlaceCover
	if strings.EqualFold(fit, utils.PlaceContain) {
		mode = utils.PlaceContain
	}
	w, h, err := utils.ImagePixelSize(path)
	if err != nil && IsSVGFile(path) {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			sw, sh, sizeErr := svgImageDimensions(data)
			if sizeErr == nil {
				// Fit using proportional integers to preserve fractional SVG
				// units while exposing natural pixels to px-space anchors.
				scale := 1000000 / math.Max(sw, sh)
				p := utils.PlaceImage(max(1, int(math.Round(sw*scale))), max(1, int(math.Round(sh*scale))), frame, mode)
				p.PixelW, p.PixelH = sw, sh
				return p
			}
		}
	}
	if err != nil {
		return utils.PlaceImage(0, 0, frame, utils.PlaceStretch)
	}
	return utils.PlaceImage(w, h, frame, mode)
}

// gridImagePlacement places a shape_grid picture insert: its on-slide frame
// and the a:srcRect crop (nil when nothing is trimmed).
func gridImagePlacement(img ImageInsert) (types.BoundingBox, *pptx.SrcRect) {
	frame := types.BoundingBox{X: img.OffsetX, Y: img.OffsetY, Width: img.ExtentCX, Height: img.ExtentCY}
	p := GridImagePlacement(img.Path, frame, img.Fit)
	return p.Bounds, imageCoverCrop(p.Crop)
}

func imageCoverCrop(c utils.CropRect) *pptx.SrcRect {
	if c.IsZero() {
		return nil
	}
	return &pptx.SrcRect{L: c.L, T: c.T, R: c.R, B: c.B}
}
