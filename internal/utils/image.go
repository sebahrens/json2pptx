// Package utils provides shared utility functions used across the application.
package utils

import (
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder

	"github.com/sebahrens/json2pptx/internal/types"
)

// EMUsPerPixel is the conversion factor from pixels to EMUs (English Metric Units).
// Re-exported from types package for backward compatibility.
// New code should use types.FromPixels() or int64(types.EMUPerPixel) directly.
const EMUsPerPixel = int64(types.EMUPerPixel)

// ScaleImageToFit scales an image to fit within bounds while maintaining aspect ratio.
// It returns the new bounding box with proper dimensions and centered position.
//
// The function reads the image file to get its dimensions, converts pixel dimensions
// to EMUs, and calculates the scaled dimensions that maintain aspect ratio while
// fitting within the provided bounds. The resulting position is centered within
// the original bounds.
func ScaleImageToFit(imagePath string, bounds types.BoundingBox) (types.BoundingBox, error) {
	w, h, err := ImagePixelSize(imagePath)
	if err != nil {
		return bounds, err
	}
	return containBounds(w, h, bounds), nil
}
