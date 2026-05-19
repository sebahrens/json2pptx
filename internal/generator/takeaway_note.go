package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Takeaway position constants (in EMUs).
// Positioned in the lower band of a standard 16:9 widescreen slide
// (12192000 x 6858000), above the source note row so source attribution
// sits below the takeaway. The takeaway is the slide's headline answer —
// what the audience should remember if they look at nothing else — so it
// gets bold weight and a darker fill than the source note.
const (
	takeawayOffsetX  = 457200   // ~0.5 inch left margin
	takeawayOffsetY  = 6200000  // ~0.45 inch above the source note row
	takeawayExtentCX = 11277600 // ~12.4 inches wide (matches source note row)
	takeawayExtentCY = 360000   // ~28pt — accommodates a 12pt bold line + padding
	takeawayShapeID  = 998      // High ID to avoid conflicts; one less than source note
)

// takeawayFontSize is the takeaway font size in hundredths of a point.
// Sourced from the tokens package so the rendered takeaway tracks the
// CardTitle typography role published in skills/generate-deck/RULES.md.
var takeawayFontSize = tokens.CardTitleMinHPt // 12pt

// generateTakeawayShape creates a p:sp element for slide takeaway text.
// The shape sits in the lower content band with bold, dark-gray text.
func generateTakeawayShape(takeawayText string) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       takeawayShapeID,
		Name:     "Takeaway",
		Bounds:   pptx.RectEmu{X: takeawayOffsetX, Y: takeawayOffsetY, CX: takeawayExtentCX, CY: takeawayExtentCY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "ctr",
			Insets: [4]int64{91440, 0, 0, 0},
			Paragraphs: []pptx.Paragraph{{
				Align: "l",
				Runs: []pptx.Run{{
					Text:     takeawayText,
					Lang:     "en-US",
					FontSize: takeawayFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SolidFill(tokens.TakeawayColor[1:]),
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// insertTakeaway inserts a takeaway text shape into the slide XML.
// It finds the closing </p:spTree> tag and inserts the shape before it,
// so the takeaway renders on top of any overlapping content.
func insertTakeaway(slideData []byte, takeawayText string) ([]byte, error) {
	shapeXML := generateTakeawayShape(takeawayText)
	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
