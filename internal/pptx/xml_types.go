// Package pptx provides shared XML types for PPTX file parsing and generation.
//
// This package consolidates XML struct definitions used across the generator
// and template packages to avoid duplication and ensure consistency.
//
// Note: Some structs exist in both this package and consuming packages because
// XML marshaling/unmarshaling requires specific struct tags for different contexts.
// This package provides the canonical definitions that can be used directly or
// embedded in package-specific wrappers.
package pptx

import (
	"encoding/xml"
	"strconv"
)

// OOXML namespace constants.
//
// IMPORTANT: These URIs use http:// intentionally - they are XML namespace
// identifiers as defined by the ECMA-376 Office Open XML standard, NOT URLs
// that are fetched over the network. This is a common pattern in XML where
// namespaces are identified by URIs that serve as unique identifiers.
//
// These values are required by the OOXML specification and must match exactly
// for Office applications (PowerPoint, LibreOffice Impress, etc.) to correctly
// parse the generated files. The http:// scheme does not indicate a security
// vulnerability or network access.
//
// Reference: ECMA-376 Office Open XML File Formats
// https://www.ecma-international.org/publications-and-standards/standards/ecma-376/
const (
	NsPresentation  = "http://schemas.openxmlformats.org/presentationml/2006/main"
	NsDrawingML     = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NsRelationships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	NsPackageRels   = "http://schemas.openxmlformats.org/package/2006/relationships"
	NsContentTypes  = "http://schemas.openxmlformats.org/package/2006/content-types"
)

// ============================================================================
// Relationship XML Types (shared between generator/images.go and generator/relationships.go)
// ============================================================================

// RelationshipsXML represents the root element of a .rels file.
type RelationshipsXML struct {
	XMLName       xml.Name          `xml:"Relationships"`
	Xmlns         string            `xml:"xmlns,attr,omitempty"`
	Relationships []RelationshipXML `xml:"Relationship"`
}

// RelationshipXML represents a single relationship entry.
type RelationshipXML struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr,omitempty"`
}

// ============================================================================
// Content Types XML Types (from generator/images.go)
// ============================================================================

// ContentTypesXML represents the [Content_Types].xml file.
type ContentTypesXML struct {
	XMLName   xml.Name              `xml:"Types"`
	Xmlns     string                `xml:"xmlns,attr"`
	Defaults  []ContentTypeDefault  `xml:"Default"`
	Overrides []ContentTypeOverride `xml:"Override"`
}

// ContentTypeDefault represents a Default element in content types.
type ContentTypeDefault struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

// ContentTypeOverride represents an Override element in content types.
type ContentTypeOverride struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}

// ============================================================================
// Theme XML Types (from template/theme.go)
// ============================================================================

// ThemeXML represents the root element of a theme XML file.
type ThemeXML struct {
	XMLName       xml.Name         `xml:"theme"`
	Name          string           `xml:"name,attr"`
	ThemeElements ThemeElementsXML `xml:"themeElements"`
}

// ThemeElementsXML represents theme elements (colors, fonts, etc.).
type ThemeElementsXML struct {
	ColorScheme ColorSchemeXML `xml:"clrScheme"`
	FontScheme  FontSchemeXML  `xml:"fontScheme"`
}

// ColorSchemeXML represents a theme color scheme.
type ColorSchemeXML struct {
	Name              string      `xml:"name,attr"`
	Dark1             ColorDefXML `xml:"dk1"`
	Light1            ColorDefXML `xml:"lt1"`
	Dark2             ColorDefXML `xml:"dk2"`
	Light2            ColorDefXML `xml:"lt2"`
	Accent1           ColorDefXML `xml:"accent1"`
	Accent2           ColorDefXML `xml:"accent2"`
	Accent3           ColorDefXML `xml:"accent3"`
	Accent4           ColorDefXML `xml:"accent4"`
	Accent5           ColorDefXML `xml:"accent5"`
	Accent6           ColorDefXML `xml:"accent6"`
	Hyperlink         ColorDefXML `xml:"hlink"`
	FollowedHyperlink ColorDefXML `xml:"folHlink"`
}

// ColorDefXML represents a color definition (sRGB or system color).
type ColorDefXML struct {
	SRGBColor   SRGBColorXML `xml:"srgbClr"`
	SystemColor SysColorXML  `xml:"sysClr"`
}

// SRGBColorXML represents an sRGB color value.
type SRGBColorXML struct {
	Val string `xml:"val,attr"`
}

// SysColorXML represents a system color value.
type SysColorXML struct {
	Val     string `xml:"val,attr"`
	LastClr string `xml:"lastClr,attr"`
}

// FontSchemeXML represents a theme font scheme.
type FontSchemeXML struct {
	Name      string       `xml:"name,attr"`
	MajorFont MajorFontXML `xml:"majorFont"`
	MinorFont MinorFontXML `xml:"minorFont"`
}

// MajorFontXML represents the major (title) font.
type MajorFontXML struct {
	Latin LatinFontXML `xml:"latin"`
}

// MinorFontXML represents the minor (body) font.
type MinorFontXML struct {
	Latin LatinFontXML `xml:"latin"`
}

// LatinFontXML represents a Latin font typeface.
type LatinFontXML struct {
	Typeface string `xml:"typeface,attr"`
}

// ============================================================================
// Presentation XML Types (from generator/slides.go)
// ============================================================================

// PresentationXML represents the presentation.xml file.
type PresentationXML struct {
	XMLName     xml.Name       `xml:"presentation"`
	SlideIDList SlideIDListXML `xml:"sldIdLst"`
	SlideSize   *SlideSizeXML  `xml:"sldSz"`
}

// SlideSizeXML represents the <p:sldSz> element with slide dimensions in EMU.
type SlideSizeXML struct {
	CX int64  `xml:"cx,attr"`
	CY int64  `xml:"cy,attr"`
}

// SlideIDListXML represents the list of slide IDs.
type SlideIDListXML struct {
	SlideIDs []SlideIDXML `xml:"sldId"`
}

// SlideIDXML represents a single slide ID entry.
type SlideIDXML struct {
	ID  uint32
	RID string
}

// UnmarshalXML implements custom XML unmarshaling for SlideIDXML to handle
// namespace-qualified attributes.
func (s *SlideIDXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" && attr.Name.Space == "" {
			id, err := strconv.ParseUint(attr.Value, 10, 32)
			if err != nil {
				return err
			}
			s.ID = uint32(id)
		} else if attr.Name.Local == "id" && attr.Name.Space == NsRelationships {
			s.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalXML implements custom XML marshaling for SlideIDXML to produce
// OOXML-compliant output with namespace-qualified attributes.
// Output format: <p:sldId id="256" r:id="rId2"/>
func (s SlideIDXML) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Override the element name to ensure it's sldId (without namespace prefix in attr)
	start.Name = xml.Name{Local: "sldId"}

	// Clear any existing attributes and add our own
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: strconv.FormatUint(uint64(s.ID), 10)},
		{Name: xml.Name{Space: NsRelationships, Local: "id"}, Value: s.RID},
	}

	// Write the self-closing element
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// ============================================================================
// Common Slide Structure Types
// ============================================================================

// These types represent the common structure shared between slide layouts
// (for reading) and slides (for generation). Different packages may embed
// or wrap these types with specific XML tags.

// PlaceholderXML represents a placeholder (ph) element.
// Used by both template and generator packages.
type PlaceholderXML struct {
	Type  string `xml:"type,attr,omitempty"`
	Index *int   `xml:"idx,attr,omitempty"`
}

// OffsetXML represents the off element with x,y coordinates.
type OffsetXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// ExtentsXML represents the ext element with width/height.
type ExtentsXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

