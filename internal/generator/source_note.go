package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// sourceNoteFontSize is the source attribution size in hundredths of a point,
// shared with every pattern that draws a source line. The note's geometry
// comes from the chrome frame's source band.
//
// It was 800 (8pt) in a raw #888888, right-aligned, while the patterns drew
// their own sources at 9-10pt dk1 on the left: one deck showed sources in
// three sizes and two corners depending on which pattern owned the slide
// (go-slide-creator-7eib).
const sourceNoteFontSize = int(patterns.SourceNoteSizePt * 100)

// generateSourceNoteShapeInBounds creates a p:sp element for source
// attribution text in small gray italics, placed at bounds (the chrome frame's
// source band). shapeID must be unique within the slide's shape tree; callers
// allocate it from findMaxShapeID(slideData)+1 just before insertion.
func generateSourceNoteShapeInBounds(sourceText string, shapeID uint32, bounds pptx.RectEmu) string {
	return generateLinkedSourceNoteShapeInBounds(sourceText, shapeID, bounds, "", false)
}

func generateLinkedSourceNoteShapeInBounds(sourceText string, shapeID uint32, bounds pptx.RectEmu, relID string, slideJump bool) string {
	action := ""
	if slideJump {
		action = "ppaction://hlinksldjump"
	}
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
				Align: patterns.SourceNoteAlign,
				Runs: []pptx.Run{{
					Text:            sourceNoteText(sourceText),
					Lang:            "en-US",
					FontSize:        sourceNoteFontSize,
					Italic:          true,
					Dirty:           true,
					Color:           pptx.SchemeFill(patterns.SourceNoteScheme),
					HyperlinkRelID:  relID,
					HyperlinkAction: action,
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// sourceNoteText labels a source line, without stuttering when the author
// already wrote the label. The rule lives in internal/patterns so the slide
// band and the patterns cannot disagree about it (go-slide-creator-xg48 found
// the stutter; go-slide-creator-7eib unified the surfaces).
func sourceNoteText(sourceText string) string {
	return patterns.SourceNoteText(sourceText)
}

// insertSourceNote inserts a source attribution text shape at bounds (the
// chrome frame's source band) before </p:spTree>.
func insertSourceNote(slideData []byte, sourceText string, bounds pptx.RectEmu) ([]byte, error) {
	return insertLinkedSourceNote(slideData, sourceText, bounds, "", false)
}

func insertLinkedSourceNote(slideData []byte, sourceText string, bounds pptx.RectEmu, relID string, slideJump bool) ([]byte, error) {
	// Allocate a slide-unique ID above any existing shape (including shapes
	// injected earlier on this slide, which are already present in slideData).
	shapeXML := generateLinkedSourceNoteShapeInBounds(sourceText, findMaxShapeID(slideData)+1, bounds, relID, slideJump)
	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
