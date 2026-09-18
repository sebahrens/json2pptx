package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// sourceNoteFontSize is the source attribution size in hundredths of a point.
// The note's geometry comes from the chrome frame's source band.
const sourceNoteFontSize = 800 // 8pt

// generateSourceNoteShapeInBounds creates a p:sp element for source
// attribution text in small gray italics, placed at bounds (the chrome frame's
// source band). shapeID must be unique within the slide's shape tree; callers
// allocate it from findMaxShapeID(slideData)+1 just before insertion.
func generateSourceNoteShapeInBounds(sourceText string, shapeID uint32, bounds pptx.RectEmu) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Source Note",
		Bounds:   bounds,
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

// insertSourceNote inserts a source attribution text shape at bounds (the
// chrome frame's source band) before </p:spTree>.
func insertSourceNote(slideData []byte, sourceText string, bounds pptx.RectEmu) ([]byte, error) {
	// Allocate a slide-unique ID above any existing shape (including shapes
	// injected earlier on this slide, which are already present in slideData).
	shapeXML := generateSourceNoteShapeInBounds(sourceText, findMaxShapeID(slideData)+1, bounds)
	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
