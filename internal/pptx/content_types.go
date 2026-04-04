// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Common content type MIME strings used in PPTX files.
const (
	// Image content types
	ContentTypePNG  = "image/png"
	ContentTypeJPEG = "image/jpeg"
	ContentTypeGIF  = "image/gif"
	ContentTypeBMP  = "image/bmp"
	ContentTypeTIFF = "image/tiff"
	ContentTypeWEBP = "image/webp"
	ContentTypeSVG  = "image/svg+xml"
	ContentTypeEMF  = "image/x-emf"

	// OOXML content types
	ContentTypePresentationMain = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ContentTypeSlide            = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ContentTypeSlideLayout      = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ContentTypeNotesSlide       = "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"
	ContentTypeRelationships    = "application/vnd.openxmlformats-package.relationships+xml"
)

// ContentTypesPath is the fixed path for the [Content_Types].xml file.
const ContentTypesPath = "[Content_Types].xml"

// extensionContentTypes maps file extensions to their default content types.
var extensionContentTypes = map[string]string{
	"png":  ContentTypePNG,
	"jpg":  ContentTypeJPEG,
	"jpeg": ContentTypeJPEG,
	"gif":  ContentTypeGIF,
	"bmp":  ContentTypeBMP,
	"tiff": ContentTypeTIFF,
	"tif":  ContentTypeTIFF,
	"webp": ContentTypeWEBP,
	"svg":  ContentTypeSVG,
	"emf":  ContentTypeEMF,
	"xml":  "application/xml",
	"rels": ContentTypeRelationships,
}

// ContentTypes manages the [Content_Types].xml file.
// It tracks extension-based defaults and part-specific overrides.
type ContentTypes struct {
	defaults  map[string]string // extension (lowercase, no dot) -> content type
	overrides map[string]string // part name -> content type
}

// NewContentTypes creates an empty content types manager.
func NewContentTypes() *ContentTypes {
	return &ContentTypes{
		defaults:  make(map[string]string),
		overrides: make(map[string]string),
	}
}

// ParseContentTypes parses a [Content_Types].xml file.
func ParseContentTypes(data []byte) (*ContentTypes, error) {
	var ctXML ContentTypesXML
	if err := xml.Unmarshal(data, &ctXML); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml: %w", err)
	}

	ct := &ContentTypes{
		defaults:  make(map[string]string),
		overrides: make(map[string]string),
	}

	for _, def := range ctXML.Defaults {
		ext := strings.ToLower(def.Extension)
		ct.defaults[ext] = def.ContentType
	}

	for _, ovr := range ctXML.Overrides {
		ct.overrides[ovr.PartName] = ovr.ContentType
	}

	return ct, nil
}

// HasDefault checks if a default content type exists for an extension.
func (ct *ContentTypes) HasDefault(extension string) bool {
	ext := normalizeExtension(extension)
	_, exists := ct.defaults[ext]
	return exists
}

// Default returns the default content type for an extension.
// Returns empty string if no default is registered.
func (ct *ContentTypes) Default(extension string) string {
	ext := normalizeExtension(extension)
	return ct.defaults[ext]
}

// SetDefault sets the default content type for an extension.
func (ct *ContentTypes) SetDefault(extension, contentType string) {
	ext := normalizeExtension(extension)
	ct.defaults[ext] = contentType
}

// EnsureDefault ensures a default content type exists for an extension.
// If the extension already has a default, this is a no-op.
// If not, it uses the provided content type.
func (ct *ContentTypes) EnsureDefault(extension, contentType string) {
	ext := normalizeExtension(extension)
	if _, exists := ct.defaults[ext]; !exists {
		ct.defaults[ext] = contentType
	}
}

// EnsureDefaultForExtension ensures a default content type exists for a known extension.
// Uses the built-in extension->content type mapping.
// Returns false if the extension is not recognized.
func (ct *ContentTypes) EnsureDefaultForExtension(extension string) bool {
	ext := normalizeExtension(extension)
	contentType, known := extensionContentTypes[ext]
	if !known {
		return false
	}
	ct.EnsureDefault(ext, contentType)
	return true
}

// EnsureSVG ensures the SVG content type is registered.
// This is a convenience method for the common SVG insertion use case.
func (ct *ContentTypes) EnsureSVG() {
	ct.EnsureDefault("svg", ContentTypeSVG)
}

// EnsurePNG ensures the PNG content type is registered.
func (ct *ContentTypes) EnsurePNG() {
	ct.EnsureDefault("png", ContentTypePNG)
}

// HasOverride checks if a part has a content type override.
func (ct *ContentTypes) HasOverride(partName string) bool {
	_, exists := ct.overrides[partName]
	return exists
}

// Override returns the content type override for a part.
// Returns empty string if no override is registered.
func (ct *ContentTypes) Override(partName string) string {
	return ct.overrides[partName]
}

// SetOverride sets the content type for a specific part.
func (ct *ContentTypes) SetOverride(partName, contentType string) {
	ct.overrides[partName] = contentType
}

// RemoveOverride removes a part's content type override.
// Returns true if the override existed and was removed.
func (ct *ContentTypes) RemoveOverride(partName string) bool {
	if _, exists := ct.overrides[partName]; exists {
		delete(ct.overrides, partName)
		return true
	}
	return false
}

// ContentType returns the content type for a part, checking overrides first.
// Falls back to extension-based defaults if no override exists.
// Returns empty string if no content type can be determined.
func (ct *ContentTypes) ContentType(partName string) string {
	// Check override first
	if contentType := ct.overrides[partName]; contentType != "" {
		return contentType
	}

	// Fall back to extension default
	ext := filepath.Ext(partName)
	return ct.Default(ext)
}

// AllDefaults returns all registered extension defaults.
func (ct *ContentTypes) AllDefaults() map[string]string {
	result := make(map[string]string)
	for k, v := range ct.defaults {
		result[k] = v
	}
	return result
}

// AllOverrides returns all registered part overrides.
func (ct *ContentTypes) AllOverrides() map[string]string {
	result := make(map[string]string)
	for k, v := range ct.overrides {
		result[k] = v
	}
	return result
}

// Marshal serializes the content types to XML.
// Output is deterministic: entries are sorted alphabetically.
func (ct *ContentTypes) Marshal() ([]byte, error) {
	// Sort extensions for deterministic output
	extensions := make([]string, 0, len(ct.defaults))
	for ext := range ct.defaults {
		extensions = append(extensions, ext)
	}
	slices.Sort(extensions)

	defaults := make([]ContentTypeDefault, len(extensions))
	for i, ext := range extensions {
		defaults[i] = ContentTypeDefault{
			Extension:   ext,
			ContentType: ct.defaults[ext],
		}
	}

	// Sort part names for deterministic output
	partNames := make([]string, 0, len(ct.overrides))
	for partName := range ct.overrides {
		partNames = append(partNames, partName)
	}
	slices.Sort(partNames)

	overrides := make([]ContentTypeOverride, len(partNames))
	for i, partName := range partNames {
		overrides[i] = ContentTypeOverride{
			PartName:    partName,
			ContentType: ct.overrides[partName],
		}
	}

	ctXML := ContentTypesXML{
		XMLName:   xml.Name{Space: NsContentTypes, Local: "Types"},
		Xmlns:     NsContentTypes,
		Defaults:  defaults,
		Overrides: overrides,
	}

	data, err := xml.MarshalIndent(ctXML, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml: %w", err)
	}

	return append([]byte(xml.Header), data...), nil
}

// Clone creates an independent copy of the content types.
func (ct *ContentTypes) Clone() *ContentTypes {
	clone := &ContentTypes{
		defaults:  make(map[string]string),
		overrides: make(map[string]string),
	}

	for k, v := range ct.defaults {
		clone.defaults[k] = v
	}
	for k, v := range ct.overrides {
		clone.overrides[k] = v
	}

	return clone
}

// normalizeExtension normalizes an extension to lowercase without leading dot.
func normalizeExtension(extension string) string {
	ext := strings.ToLower(extension)
	return strings.TrimPrefix(ext, ".")
}

// ExtensionForContentType returns the standard extension for a content type.
// Returns empty string if the content type is not recognized.
func ExtensionForContentType(contentType string) string {
	for ext, ct := range extensionContentTypes {
		if ct == contentType {
			return ext
		}
	}
	return ""
}
