// Package utils provides shared utility functions used across the application.
package utils

import (
	"image"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"os"

	"github.com/ahrens/go-slide-creator/internal/types"
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
	// Open image to get dimensions
	f, err := os.Open(imagePath)
	if err != nil {
		return bounds, err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.DecodeConfig(f)
	if err != nil {
		return bounds, err
	}

	// Convert pixel dimensions to EMUs
	imgWidthEMU := int64(img.Width) * EMUsPerPixel
	imgHeightEMU := int64(img.Height) * EMUsPerPixel

	// Calculate scale factors
	scaleX := float64(bounds.Width) / float64(imgWidthEMU)
	scaleY := float64(bounds.Height) / float64(imgHeightEMU)

	// Use the smaller scale to maintain aspect ratio
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// Calculate new dimensions
	newWidth := int64(float64(imgWidthEMU) * scale)
	newHeight := int64(float64(imgHeightEMU) * scale)

	// Center the image in the placeholder
	offsetX := bounds.X + (bounds.Width-newWidth)/2
	offsetY := bounds.Y + (bounds.Height-newHeight)/2

	return types.BoundingBox{
		X:      offsetX,
		Y:      offsetY,
		Width:  newWidth,
		Height: newHeight,
	}, nil
}
