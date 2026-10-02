package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ValidImageFits returns the native picture-placeholder fit vocabulary.
func ValidImageFits() []string { return []string{"cover", "contain"} }

// ValidateImageFit accepts an omitted fit as the existing cover default.
func ValidateImageFit(fit string) error {
	switch strings.ToLower(fit) {
	case "", "cover", "contain":
		return nil
	default:
		return fmt.Errorf("invalid image fit %q: expected cover or contain", fit)
	}
}

// imagePlacement applies the same placement to raster pictures and native SVG
// fallback pairs. Contain retains the whole source and centers its original
// aspect ratio in the native frame; it never stretches or crops evidence.
// The transform itself is utils.PlaceImage, shared with shape_grid pictures
// and the overlay resolver that points callouts at source-image pixels.
func imagePlacement(path string, frame types.BoundingBox, fit string) (types.BoundingBox, *pptx.SrcRect, error) {
	if fit != fitContainStart {
		if err := ValidateImageFit(fit); err != nil {
			return frame, nil, err
		}
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		return frame, nil, fmt.Errorf("image frame must have positive dimensions")
	}
	mode := strings.ToLower(fit)
	if mode == "" {
		mode = utils.PlaceCover
	}
	w, h, err := utils.ImagePixelSize(path)
	if err != nil {
		if mode == utils.PlaceCover {
			return frame, nil, fmt.Errorf("failed to read image dimensions for %s", path)
		}
		return frame, nil, fmt.Errorf("read image dimensions: %w", err)
	}
	p := utils.PlaceImage(w, h, frame, mode)
	if mode != utils.PlaceCover && (p.Bounds.Width <= 0 || p.Bounds.Height <= 0) {
		return frame, nil, fmt.Errorf("contained image has zero extent")
	}
	return p.Bounds, imageCoverCrop(p.Crop), nil
}

// fitContainStart is the internal placement used when an image with no
// authored fit lands in a body / content placeholder: the whole picture is
// kept and anchored to the placeholder's top-left corner, so it starts on
// the content grid instead of floating centred. Body placeholders are text
// frames (One Content is ~2.85:1); cover-cropping a portrait photo into one
// kept ~23% of it (go-slide-creator-dk5sk). Never accepted from input.
const fitContainStart = "contain-start"

// heavyCropThreshold is the share of either image axis cover may discard
// before IMAGE_HEAVY_CROP is reported.
const heavyCropThreshold = 0.30

// effectiveImageFit resolves an omitted fit by placeholder kind: cover for a
// genuine picture placeholder (its frame was designed for a photo), the
// whole-image contain-start placement for any other placeholder.
func effectiveImageFit(fit string, shape *shapeXML) string {
	if fit != "" {
		return strings.ToLower(fit)
	}
	if shape != nil {
		if ph := shape.NonVisualProperties.NvPr.Placeholder; ph != nil && ph.Type == "pic" {
			return "cover"
		}
	}
	return fitContainStart
}

// coverDiscardFraction returns the larger share of either axis a cover crop
// throws away (0 = nothing, 0.77 = 77% of the height discarded).
func coverDiscardFraction(crop *pptx.SrcRect) float64 {
	if crop == nil {
		return 0
	}
	h := float64(crop.L+crop.R) / 100000
	v := float64(crop.T+crop.B) / 100000
	if v > h {
		return v
	}
	return h
}
