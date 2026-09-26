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
func imagePlacement(path string, frame types.BoundingBox, fit string) (types.BoundingBox, *pptx.SrcRect, error) {
	if err := ValidateImageFit(fit); err != nil {
		return frame, nil, err
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		return frame, nil, fmt.Errorf("image frame must have positive dimensions")
	}
	if strings.EqualFold(fit, "contain") {
		bounds, err := utils.ScaleImageToFit(path, frame)
		if err != nil {
			return frame, nil, fmt.Errorf("read image dimensions: %w", err)
		}
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return frame, nil, fmt.Errorf("contained image has zero extent")
		}
		return bounds, nil, nil
	}
	crop, ok := utils.CoverCropForFile(path, frame.Width, frame.Height)
	if !ok {
		return frame, nil, fmt.Errorf("failed to read image dimensions for %s", path)
	}
	return frame, imageCoverCrop(crop), nil
}
