package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// The slide takeaway is the shared takeaway component (patterns.Takeaway*,
// go-slide-creator-7b5o6) in its default look (go-slide-creator-3a1rm,
// -fmmec): one band the width of the body column, filled with the dark
// structural neutral (patterns.TakeawayBandTone), carrying 14pt bold text in
// the ink measured against that fill, 12pt in from each side and centred on
// the band's height. No bar and no stroke. A pattern's own so-what (exec
// summary bottom line, metric-list callout, envelope callout) draws the same
// band, so every takeaway on a deck speaks one language. It used to be a
// flush 3pt accent bar beside unfilled dk1 text, and before that an accent1
// "Lighter 80%" wash framed by a 1pt accent rule.
//
// Its band rectangle comes from the chrome frame
// (template.ResolveChromeFrameForTakeaway), which reserves the band the text
// needs — one line or two — with 16pt of air above it and 12pt below it to
// the source line or the footer. Inside that rectangle the band takes the
// height its text needs, so the fill never runs past the words' padding.

// takeawayFontSize is the takeaway font size in hundredths of a point.
var takeawayFontSize = int(patterns.TakeawaySizePt * 100)

const emuPerPt = 12700

// takeawayStyle carries the per-slide inputs the band cannot know on its own.
type takeawayStyle struct {
	// FontName is the theme body font, used to measure the wrap.
	FontName string
	// ThemeColors resolve the band's tone: dk2 where it carries the brand,
	// a charcoal where dk2 is black, and the ink that reads on it. Empty
	// colours give the dk2 band with lt1 text.
	ThemeColors []types.ThemeColor
}

// takeawayBandHeight returns the band height (EMU) the text needs in a band
// bounds.CX wide, capped at bounds.CY.
func takeawayBandHeight(text string, bounds pptx.RectEmu, style takeawayStyle) int64 {
	lines := template.TakeawayLines(text, style.FontName, bounds.CX, 1)
	return min(template.TakeawayTextHeightEMU(lines), bounds.CY)
}

// generateTakeawayShapesInBounds returns the band — one filled shape carrying
// the text — for the chrome takeaway rectangle bounds. id must be unique
// within the slide's shape tree.
func generateTakeawayShapesInBounds(takeawayText string, id uint32, bounds pptx.RectEmu, style takeawayStyle) string {
	band := bounds
	band.CY = takeawayBandHeight(takeawayText, bounds, style)

	tone := patterns.TakeawayBandSchemeFor(patterns.ExpandContext{Theme: types.ThemeInfo{Colors: style.ThemeColors}})
	var mods []pptx.ColorMod
	if tone.LumMod != 0 {
		mods = append(mods, pptx.LumMod(tone.LumMod))
	}
	if tone.LumOff != 0 {
		mods = append(mods, pptx.LumOff(tone.LumOff))
	}
	side := int64(patterns.TakeawayTextInsetPt * emuPerPt)
	pad := int64(patterns.TakeawayBandPadPt * emuPerPt)
	shape, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:   id,
		Name: "Takeaway",
		// The band spans the body column (as the chrome frame and the
		// portability checks expect); the text stands 12pt in on each side.
		Bounds:   band,
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill(tone.Color, mods...),
		Line:     pptx.NoLine(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "ctr",
			Insets: [4]int64{side, pad, side, pad},
			Paragraphs: []pptx.Paragraph{{
				Align: "l",
				Runs: []pptx.Run{{
					Text:     takeawayText,
					Lang:     "en-US",
					FontSize: takeawayFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill(tone.Ink),
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(shape)
}

// insertTakeaway inserts the takeaway band at bounds (the chrome frame's
// takeaway rectangle) before </p:spTree>, so it renders on top of slide
// content, measuring with Arial and toned for an unknown theme.
func insertTakeaway(slideData []byte, takeawayText string, bounds pptx.RectEmu) ([]byte, error) {
	return insertStyledTakeaway(slideData, takeawayText, bounds, takeawayStyle{})
}

func insertStyledTakeaway(slideData []byte, takeawayText string, bounds pptx.RectEmu, style takeawayStyle) ([]byte, error) {
	// Allocate a slide-unique ID above any existing shape so the band cannot
	// collide with content shapes or other late injections.
	shapesXML := generateTakeawayShapesInBounds(takeawayText, findMaxShapeID(slideData)+1, bounds, style)
	return pptx.InsertIntoSpTree(slideData, []byte(shapesXML), pptx.InsertAtEnd)
}
