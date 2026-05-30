package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Source note position constants (in EMUs).
// Positioned at the bottom-right of a standard 16:9 widescreen slide (12192000 x 6858000).
// The shape sits in the lower-right area with small margins.
const (
	sourceNoteOffsetX  = 457200   // ~0.5 inch left margin
	sourceNoteOffsetY  = 6607200  // Shape bottom lands ~4pt above slide edge
	sourceNoteExtentCX = 11277600 // ~12.4 inches wide (full width minus margins)
	sourceNoteExtentCY = 200000   // ~15.7pt — sufficient for 8pt text with zero margins
	sourceNoteFontSize = 800      // 8pt in hundredths of a point
)

// generateSourceNoteShape creates a p:sp element for source attribution text.
// The shape is positioned at the bottom of the slide with small, gray text.
// shapeID must be unique within the slide's shape tree; callers allocate it
// from findMaxShapeID(slideData)+1 just before insertion.
func generateSourceNoteShape(sourceText string, shapeID uint32) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Source Note",
		Bounds:   pptx.RectEmu{X: sourceNoteOffsetX, Y: sourceNoteOffsetY, CX: sourceNoteExtentCX, CY: sourceNoteExtentCY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "t",
			Insets: [4]int64{91440, 0, 0, 0},
			Paragraphs: []pptx.Paragraph{{
				Align: "r",
				Runs: []pptx.Run{{
					Text:     "Source: " + sourceText,
					Lang:     "en-US",
					FontSize: sourceNoteFontSize,
					Italic:   true,
					Dirty:    true,
					Color:    pptx.SolidFill("888888"),
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// insertSourceNote inserts a source attribution text shape into the slide XML.
// It finds the closing </p:spTree> tag and inserts the shape before it.
func insertSourceNote(slideData []byte, sourceText string) ([]byte, error) {
	// Allocate a slide-unique ID above any existing shape (including shapes
	// injected earlier on this slide, which are already present in slideData).
	shapeXML := generateSourceNoteShape(sourceText, findMaxShapeID(slideData)+1)

	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
