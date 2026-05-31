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
)

// TakeawayBandTopEMU is the Y coordinate (EMU) of the top edge of the fixed
// lower band where the takeaway headline renders. It is the single source of
// truth for that boundary: layout code that lays out full-area content (e.g.
// the shape-grid content zone) reserves space above this line when a slide
// carries a takeaway, so cards do not crowd or touch the takeaway text
// (go-slide-creator-rdtn).
const TakeawayBandTopEMU int64 = takeawayOffsetY

// takeawayFontSize is the takeaway font size in hundredths of a point.
// Sourced from the tokens package so the rendered takeaway tracks the
// CardTitle typography role published in skills/generate-deck/RULES.md.
var takeawayFontSize = tokens.CardTitleMinHPt // 12pt

// Takeaway band accent tint (in thousandths of a percent). The band fill is
// the template's accent1 lightened ~80% toward white ("Accent 1, Lighter 80%"
// in PowerPoint terms). Using lumMod/lumOff keeps the band a subtle, branded
// wash that is always light enough for the dark takeaway text to clear WCAG AA
// — including on dark templates, where the takeaway is injected AFTER the
// contrast pass and so cannot rely on auto-flip (go-slide-creator-5ovr).
const (
	takeawayBandLumMod = 20000 // keep 20% of the accent's luminance
	takeawayBandLumOff = 80000 // add 80% luminance → near-white accent wash
	takeawayRuleWidth  = 12700 // 1pt accent border framing the band
)

// generateTakeawayShape creates a p:sp element for slide takeaway text.
// The shape renders as a distinct band in the lower content zone: a subtle
// accent-tinted fill framed by a thin accent rule, carrying bold dark text.
// The band gives the takeaway its own light background, so the headline reads
// regardless of the underlying slide/template color.
// shapeID must be unique within the slide's shape tree; callers allocate it
// from findMaxShapeID(slideData)+1 just before insertion.
func generateTakeawayShape(takeawayText string, shapeID uint32) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Takeaway",
		Bounds:   pptx.RectEmu{X: takeawayOffsetX, Y: takeawayOffsetY, CX: takeawayExtentCX, CY: takeawayExtentCY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill("accent1", pptx.LumMod(takeawayBandLumMod), pptx.LumOff(takeawayBandLumOff)),
		Line:     pptx.Line{Width: takeawayRuleWidth, Fill: pptx.SchemeFill("accent1")},
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
	// Allocate a slide-unique ID above any existing shape so the takeaway
	// cannot collide with content shapes or other late injections.
	shapeXML := generateTakeawayShape(takeawayText, findMaxShapeID(slideData)+1)
	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
