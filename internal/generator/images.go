// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ValidateImagePathWithConfig validates that an image path is safe to use,
// with explicit configuration injection.
// It prevents path traversal attacks (CRIT-03) by:
// 1. Checking for ".." path components that could escape allowed directories
// 2. Ensuring the resolved absolute path is within allowed base directories
//
// Returns nil if the path is safe, or an error describing the security issue.
func ValidateImagePathWithConfig(imagePath string, allowedPaths []string) error {
	return utils.ValidatePath(imagePath, allowedPaths)
}

// scaleImageToFit scales an image to fit within bounds while maintaining aspect ratio.
// AC6: Image Scaling
func scaleImageToFit(imagePath string, bounds types.BoundingBox) (types.BoundingBox, error) {
	return utils.ScaleImageToFit(imagePath, bounds)
}

// XML structures for relationships and content types are defined in internal/pptx package.

// imageExtensionContentTypes maps file extensions to MIME types
var imageExtensionContentTypes = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"tiff": "image/tiff",
	"tif":  "image/tiff",
	"webp": "image/webp",
	"svg":  "image/svg+xml", // For detection; SVG files are converted to PNG or EMF
	"emf":  "image/x-emf",   // Enhanced Metafile for vector graphics in PPTX
}

// updateContentTypes updates [Content_Types].xml to include image extension mappings.
// AC-NEW4: Given image with content type (e.g., image/png), When generated,
// Then [Content_Types].xml includes extension mapping.
func updateContentTypes(idx utils.ZipIndex, usedExtensions map[string]bool) ([]byte, error) {
	// Read existing [Content_Types].xml
	var contentTypes pptx.ContentTypesXML
	ctData, err := utils.ReadFileFromZipIndex(idx, "[Content_Types].xml")
	if err != nil {
		return nil, fmt.Errorf("failed to read [Content_Types].xml: %w", err)
	}

	if err := xml.Unmarshal(ctData, &contentTypes); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml: %w", err)
	}

	// Build set of existing extensions
	existingExtensions := make(map[string]bool)
	for _, def := range contentTypes.Defaults {
		existingExtensions[strings.ToLower(def.Extension)] = true
	}

	// Add missing extension mappings for used image types (sorted for determinism)
	sortedExts := make([]string, 0, len(usedExtensions))
	for ext := range usedExtensions {
		sortedExts = append(sortedExts, ext)
	}
	sort.Strings(sortedExts)

	for _, ext := range sortedExts {
		extLower := strings.ToLower(ext)
		if existingExtensions[extLower] {
			continue // Already exists
		}

		contentType, known := imageExtensionContentTypes[extLower]
		if !known {
			continue // Unknown extension, skip
		}

		contentTypes.Defaults = append(contentTypes.Defaults, pptx.ContentTypeDefault{
			Extension:   extLower,
			ContentType: contentType,
		})
	}

	// Marshal back to XML
	modifiedData, err := xml.Marshal(contentTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml: %w", err)
	}

	// Add XML header
	return []byte(xml.Header + string(modifiedData)), nil
}
