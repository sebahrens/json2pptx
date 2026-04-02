// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"strconv"
	"strings"
)

// OOXML path constants for PPTX file structure.
// These constants define the standard paths within a PPTX ZIP archive.
const (
	// Core PPTX paths
	PathPresentationXML     = "ppt/presentation.xml"
	PathPresentationRels    = "ppt/_rels/presentation.xml.rels"
	PathContentTypes        = "[Content_Types].xml"
	PathDocPropsApp         = "docProps/app.xml"
	PathMedia               = "ppt/media/"
	PathSlides              = "ppt/slides/"
	PathSlideLayouts        = "ppt/slideLayouts/"
	PathSlideLayoutRels     = "ppt/slideLayouts/_rels/"
	PathSlideRels           = "ppt/slides/_rels/"

	// Notes slides
	PathNotesSlides         = "ppt/notesSlides/"
	PathNotesSlideRels      = "ppt/notesSlides/_rels/"

	// Format patterns for path generation
	PatternSlideXML         = "ppt/slides/slide%d.xml"
	PatternSlideRelsXML     = "ppt/slides/_rels/slide%d.xml.rels"
	PatternSlideLayoutXML   = "ppt/slideLayouts/%s.xml"
	PatternSlideLayoutRels  = "ppt/slideLayouts/_rels/%s.xml.rels"
	PatternMediaImage       = "ppt/media/image%d"
	PatternNotesSlideXML    = "ppt/notesSlides/notesSlide%d.xml"
	PatternNotesSlideRels   = "ppt/notesSlides/_rels/notesSlide%d.xml.rels"

	// Glob patterns for file matching
	PatternMatchSlideXML    = "ppt/slides/slide*.xml"
	PatternMatchMediaImage  = "ppt/media/image*"
	PatternMatchSlideRels   = "ppt/slides/_rels/slide*.xml.rels"
)

// SlidePath returns the path for a slide XML file.
func SlidePath(slideNum int) string {
	return fmt.Sprintf(PatternSlideXML, slideNum)
}

// SlideRelsPath returns the path for a slide's relationships file.
func SlideRelsPath(slideNum int) string {
	return fmt.Sprintf(PatternSlideRelsXML, slideNum)
}

// LayoutPath returns the path for a slide layout XML file.
func LayoutPath(layoutID string) string {
	return fmt.Sprintf(PatternSlideLayoutXML, layoutID)
}

// LayoutRelsPath returns the path for a slide layout's relationships file.
func LayoutRelsPath(layoutID string) string {
	return fmt.Sprintf(PatternSlideLayoutRels, layoutID)
}

// MediaPath returns the path for a media file.
func MediaPath(mediaFileName string) string {
	return PathMedia + mediaFileName
}

// NotesSlideXMLPath returns the path for a notes slide XML file.
func NotesSlideXMLPath(slideNum int) string {
	return fmt.Sprintf(PatternNotesSlideXML, slideNum)
}

// NotesSlideRelsPath returns the path for a notes slide's relationships file.
func NotesSlideRelsPath(slideNum int) string {
	return fmt.Sprintf(PatternNotesSlideRels, slideNum)
}

// parseNotesSlideNum extracts the slide number from a path like "ppt/notesSlides/notesSlide3.xml".
func parseNotesSlideNum(path string) (int, bool) {
	return parseNumFromPath(path, "ppt/notesSlides/notesSlide", ".xml")
}

// parseNotesSlideRelsNum extracts the slide number from a path like "ppt/notesSlides/_rels/notesSlide3.xml.rels".
func parseNotesSlideRelsNum(path string) (int, bool) {
	return parseNumFromPath(path, "ppt/notesSlides/_rels/notesSlide", ".xml.rels")
}

// parseSlideNum extracts the slide number from a path like "ppt/slides/slide3.xml".
// Returns the number and true on success, or 0 and false if the path doesn't match.
func parseSlideNum(path string) (int, bool) {
	return parseNumFromPath(path, "ppt/slides/slide", ".xml")
}

// parseSlideRelsNum extracts the slide number from a path like "ppt/slides/_rels/slide3.xml.rels".
// Returns the number and true on success, or 0 and false if the path doesn't match.
func parseSlideRelsNum(path string) (int, bool) {
	return parseNumFromPath(path, "ppt/slides/_rels/slide", ".xml.rels")
}

// parseNumFromPath extracts a number between a prefix and suffix in a path string.
func parseNumFromPath(path, prefix, suffix string) (int, bool) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}
	numStr := path[len(prefix) : len(path)-len(suffix)]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, false
	}
	return n, true
}
