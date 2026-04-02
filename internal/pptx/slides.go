// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// presentationXML represents the presentation.xml file for slide parsing.
// This is a minimal structure that focuses on the slide ID list.
type presentationXML struct {
	SlideIDList slideIDListXML `xml:"sldIdLst"`
}

// slideIDListXML represents the list of slide IDs in presentation order.
type slideIDListXML struct {
	SlideIDs []slideIDXML `xml:"sldId"`
}

// slideIDXML represents a single slide ID entry with relationship reference.
type slideIDXML struct {
	ID  uint32 // p:sldId/@id
	RID string // p:sldId/@r:id
}

// UnmarshalXML implements custom XML unmarshaling for slideIDXML to handle
// the namespace-qualified r:id attribute.
func (s *slideIDXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && attr.Name.Space == "":
			// Parse the numeric ID
			var id uint32
			if _, err := fmt.Sscanf(attr.Value, "%d", &id); err != nil {
				return fmt.Errorf("invalid slide id: %w", err)
			}
			s.ID = id
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			// This is the r:id attribute
			s.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalXML implements custom XML marshaling for slideIDXML to produce
// OOXML-compliant output with namespace-qualified attributes.
// Output format: <p:sldId id="256" r:id="rId2"/>
func (s slideIDXML) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "sldId"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)},
		{Name: xml.Name{Space: NsRelationships, Local: "id"}, Value: s.RID},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// SlideInfo contains information about a slide.
type SlideInfo struct {
	Index    int    // 0-based index in presentation order
	ID       uint32 // Unique slide ID from presentation.xml
	RID      string // Relationship ID (e.g., "rId5")
	PartPath string // Full part path (e.g., "ppt/slides/slide1.xml")
}

// SlideEnumerator enumerates slides in a PPTX package.
// It resolves slide indices to their part paths using presentation.xml and relationships.
type SlideEnumerator struct {
	slides []SlideInfo
	byRID  map[string]int // rId -> index in slides
}

// NewSlideEnumerator creates a SlideEnumerator from a package.
// It parses presentation.xml and the presentation relationships to build
// the slide list in presentation order.
func NewSlideEnumerator(pkg *Package) (*SlideEnumerator, error) {
	// Read presentation.xml
	presData, err := pkg.ReadEntry("ppt/presentation.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to read presentation.xml: %w", err)
	}

	// Parse presentation.xml
	var pres presentationXML
	if err := xml.Unmarshal(presData, &pres); err != nil {
		return nil, fmt.Errorf("failed to parse presentation.xml: %w", err)
	}

	// Read presentation relationships
	relsPath := PresentationRels()
	relsData, err := pkg.ReadEntry(relsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read presentation.xml.rels: %w", err)
	}

	rels, err := ParseRelationships(relsData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse presentation.xml.rels: %w", err)
	}

	// Build slide list in presentation order
	slides := make([]SlideInfo, 0, len(pres.SlideIDList.SlideIDs))
	byRID := make(map[string]int)

	for i, sldID := range pres.SlideIDList.SlideIDs {
		rel := rels.Get(sldID.RID)
		if rel == nil {
			return nil, fmt.Errorf("slide %d: relationship %s not found", i, sldID.RID)
		}

		// Verify it's a slide relationship
		if rel.Type != RelTypeSlide {
			return nil, fmt.Errorf("slide %d: relationship %s has wrong type: %s", i, sldID.RID, rel.Type)
		}

		// Resolve relative target to absolute path
		partPath := resolveRelativePath("ppt/presentation.xml", rel.Target)

		info := SlideInfo{
			Index:    i,
			ID:       sldID.ID,
			RID:      sldID.RID,
			PartPath: partPath,
		}

		slides = append(slides, info)
		byRID[sldID.RID] = i
	}

	return &SlideEnumerator{
		slides: slides,
		byRID:  byRID,
	}, nil
}

// Count returns the number of slides.
func (e *SlideEnumerator) Count() int {
	return len(e.slides)
}

// Slides returns all slides in presentation order.
func (e *SlideEnumerator) Slides() []SlideInfo {
	result := make([]SlideInfo, len(e.slides))
	copy(result, e.slides)
	return result
}

// ByIndex returns slide info by 0-based index.
// Returns nil if the index is out of range.
func (e *SlideEnumerator) ByIndex(index int) *SlideInfo {
	if index < 0 || index >= len(e.slides) {
		return nil
	}
	info := e.slides[index]
	return &info
}

// ByRID returns slide info by relationship ID.
// Returns nil if the relationship ID is not found.
func (e *SlideEnumerator) ByRID(rid string) *SlideInfo {
	if idx, ok := e.byRID[rid]; ok {
		info := e.slides[idx]
		return &info
	}
	return nil
}

// PartPath returns the part path for a slide by 0-based index.
// Returns empty string if the index is out of range.
func (e *SlideEnumerator) PartPath(index int) string {
	if index < 0 || index >= len(e.slides) {
		return ""
	}
	return e.slides[index].PartPath
}

// RelsPath returns the relationships file path for a slide by 0-based index.
// For example: index 0 -> "ppt/slides/_rels/slide1.xml.rels"
func (e *SlideEnumerator) RelsPath(index int) string {
	partPath := e.PartPath(index)
	if partPath == "" {
		return ""
	}
	return GetRelsPath(partPath)
}

// resolveRelativePath resolves a relative path against a base path.
// For example: base="ppt/presentation.xml", rel="slides/slide1.xml" -> "ppt/slides/slide1.xml"
func resolveRelativePath(basePath, relativePath string) string {
	// If it's already absolute (starts with /), strip the leading /
	if strings.HasPrefix(relativePath, "/") {
		return strings.TrimPrefix(relativePath, "/")
	}

	// Get the directory of the base path
	lastSlash := strings.LastIndex(basePath, "/")
	var baseDir string
	if lastSlash >= 0 {
		baseDir = basePath[:lastSlash+1]
	}

	// Process relative path components
	result := baseDir + relativePath

	// Normalize .. segments
	for strings.Contains(result, "/../") {
		// Find /../ and remove the preceding directory
		idx := strings.Index(result, "/../")
		if idx <= 0 {
			// Can't go above root
			result = strings.TrimPrefix(result, "../")
			continue
		}

		// Find the start of the preceding directory
		prevSlash := strings.LastIndex(result[:idx], "/")
		if prevSlash < 0 {
			// Remove from start
			result = result[idx+4:]
		} else {
			// Remove preceding directory and /../
			result = result[:prevSlash+1] + result[idx+4:]
		}
	}

	return result
}
