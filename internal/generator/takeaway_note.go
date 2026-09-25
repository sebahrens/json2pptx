package generator

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// takeawayFontSize is the takeaway font size in hundredths of a point. The
// takeaway is the slide's headline answer, so it uses a 16pt banner size
// rather than a card-title or footnote size. Its band geometry
// (x-range, height, position above the footer chrome) comes from the template
// profile via template.ResolveChromeFrame — there are no fixed EMU positions.
var takeawayFontSize = tokens.GridHeaderDefaultHPt // 16pt

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

// generateTakeawayShapeInBounds creates a p:sp element for slide takeaway text.
// The shape renders as a distinct band: a subtle accent-tinted fill framed by
// a thin accent rule, carrying bold dark text, so the headline reads
// regardless of the underlying slide/template color. bounds is the takeaway
// rectangle of the slide's chrome frame. shapeID must be unique within the
// slide's shape tree; callers allocate it from findMaxShapeID(slideData)+1.
func generateTakeawayShapeInBounds(takeawayText string, shapeID uint32, bounds pptx.RectEmu) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Takeaway",
		Bounds:   bounds,
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

// insertTakeaway inserts a takeaway text shape at bounds (the chrome frame's
// takeaway band) before </p:spTree>, so it renders on top of slide content.
func insertTakeaway(slideData []byte, takeawayText string, bounds pptx.RectEmu) ([]byte, error) {
	// Allocate a slide-unique ID above any existing shape so the takeaway
	// cannot collide with content shapes or other late injections.
	shapeXML := generateTakeawayShapeInBounds(takeawayText, findMaxShapeID(slideData)+1, bounds)
	return pptx.InsertIntoSpTree(slideData, []byte(shapeXML), pptx.InsertAtEnd)
}
