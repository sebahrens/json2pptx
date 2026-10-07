package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
)

// The slide takeaway is the shared takeaway component (patterns.Takeaway*,
// go-slide-creator-7b5o6): a flush 3pt accent1 bar the full height of the
// band, and 14pt bold text in the theme's dk1 ink 12pt from the bar,
// top-anchored. No fill and no stroke. It used to be an accent1 "Lighter 80%"
// wash framed by a 1pt accent rule carrying hard-coded #1F1F1F text — a
// peach box on warm templates, and one of four different boxes the engine
// drew for the same job.
//
// Its band rectangle comes from the chrome frame
// (template.ResolveChromeFrameForTakeaway), which reserves the band the text
// needs — one line or two — with 16pt of air above it and 12pt below it to
// the source line or the footer. Inside that rectangle the band takes the
// height its text needs, so the bar never runs past the words.

// takeawayFontSize is the takeaway font size in hundredths of a point.
var takeawayFontSize = int(patterns.TakeawaySizePt * 100)

const emuPerPt = 12700

// takeawayStyle carries the per-slide inputs the band cannot know on its own.
type takeawayStyle struct {
	// FontName is the theme body font, used to measure the wrap.
	FontName string
	// InkHex, when set, replaces the dk1 scheme ink: the layout's background
	// would leave dk1 unreadable (a dark layout) and the contrast pass has
	// already run by the time the takeaway is injected.
	InkHex string
}

// takeawayBandHeight returns the band height (EMU) the text needs in a band
// bounds.CX wide, capped at bounds.CY.
func takeawayBandHeight(text string, bounds pptx.RectEmu, style takeawayStyle) int64 {
	lines := template.TakeawayLines(text, style.FontName, bounds.CX, 1)
	return min(template.TakeawayTextHeightEMU(lines), bounds.CY)
}

// generateTakeawayShapesInBounds returns the band's two shapes — the accent
// bar and the text — for the chrome takeaway rectangle bounds. firstID and
// firstID+1 must be unique within the slide's shape tree.
func generateTakeawayShapesInBounds(takeawayText string, firstID uint32, bounds pptx.RectEmu, style takeawayStyle) string {
	band := bounds
	band.CY = takeawayBandHeight(takeawayText, bounds, style)
	barW := int64(patterns.TakeawayBarPt * emuPerPt)

	bar, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       firstID,
		Name:     "Takeaway Bar",
		Bounds:   pptx.RectEmu{X: band.X, Y: band.Y, CX: barW, CY: band.CY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill("accent1"),
		Line:     pptx.NoLine(),
	})
	if err != nil {
		return ""
	}

	ink := pptx.SchemeFill(patterns.TakeawayInk)
	if style.InkHex != "" {
		ink = pptx.SolidFill(strings.TrimPrefix(style.InkHex, "#"))
	}
	pad := int64(patterns.TakeawayPadPt * emuPerPt)
	text, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:   firstID + 1,
		Name: "Takeaway",
		// The text box spans the whole band (so the band's x is the body
		// column, as the chrome frame and portability checks expect) and
		// clears the bar with its left inset.
		Bounds:   band,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		Line:     pptx.NoLine(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "t",
			Insets: [4]int64{barW + int64(patterns.TakeawayTextInsetPt*emuPerPt), pad, 0, pad},
			Paragraphs: []pptx.Paragraph{{
				Align: "l",
				Runs: []pptx.Run{{
					Text:     takeawayText,
					Lang:     "en-US",
					FontSize: takeawayFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    ink,
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(bar) + string(text)
}

// insertTakeaway inserts the takeaway band at bounds (the chrome frame's
// takeaway rectangle) before </p:spTree>, so it renders on top of slide
// content, measuring with Arial and inking in dk1.
func insertTakeaway(slideData []byte, takeawayText string, bounds pptx.RectEmu) ([]byte, error) {
	return insertStyledTakeaway(slideData, takeawayText, bounds, takeawayStyle{})
}

func insertStyledTakeaway(slideData []byte, takeawayText string, bounds pptx.RectEmu, style takeawayStyle) ([]byte, error) {
	// Allocate slide-unique IDs above any existing shape so the band cannot
	// collide with content shapes or other late injections.
	shapesXML := generateTakeawayShapesInBounds(takeawayText, findMaxShapeID(slideData)+1, bounds, style)
	return pptx.InsertIntoSpTree(slideData, []byte(shapesXML), pptx.InsertAtEnd)
}

// takeawayInkForLayout returns "" when dk1 reads on the layout's background
// (the common case: keep the scheme ink) or the theme colour that does when
// it would not. It records no finding — the chrome pass owns those.
func (ctx *singlePassContext) takeawayInkForLayout(layoutID string) string {
	if layoutID == "" {
		return ""
	}
	layoutData, err := ctx.readLayoutFile(layoutID)
	if err != nil {
		return ""
	}
	bgHex := extractLayoutBackgroundColor(layoutData, ctx.themeColors)
	inkHex := resolveSchemeColorToHex(patterns.TakeawayInk, ctx.themeColors)
	return chromeColorVerdict(bgHex, inkHex, ctx.themeColors, 0).hex
}
