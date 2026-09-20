package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Content sizing for native panel groups (go-slide-creator-5smrk).
//
// A columns panel group used to give every body rectangle the whole remaining
// placeholder height, and a stat-card grid used to give every card an equal
// share of it. One sentence of body text therefore rendered as a bordered box
// around roughly 80% empty space, and a two-line stat card as a tinted tile
// three times taller than its own content.
//
// These helpers measure the text at the panel's own width and hand back the
// height the content actually needs plus the y-offset that centres the shrunk
// group inside the placeholder — the native-shape equivalent of what
// internal/patterns does with contentCardHeightPt and the default vertical
// alignment. Centring alone would not have fixed it: the box itself was the
// wrong size, and centred text inside an oversized outline still reads as an
// empty box.

const (
	// panelMeasureDefaultFont is the font used to measure panel text when the
	// theme's body font is unknown. Metrics differ between faces, so the
	// measured height carries panelBodySlackFrac of slack on top.
	panelMeasureDefaultFont = "Arial"

	// panelBodySlackFrac is the fraction of a line height added to every
	// measured block. The measurement uses the theme's body font where it is
	// known, but the renderer may substitute a face with taller metrics, and a
	// box one pixel too short clips its last line — a worse defect than the
	// empty space this sizing removes.
	panelBodySlackFrac = 0.5

	// panelBodyMinLines is the smallest body box, in lines of body text. A
	// single short line still wants a box that reads as a panel rather than a
	// rule under the header.
	panelBodyMinLines = 2.0

	// panelContentLineHeight is the line-height multiplier the measurement
	// assumes, matching textfit's own default.
	panelContentLineHeight = 1.2
)

// measureWidthEMU converts a shape's width and its left/right text insets into
// the width to hand textfit. textfit.MeasureHeight subtracts the OOXML default
// 7.2pt side margins itself, so the caller has to add them back for a shape
// that declares its own insets, or the text is measured in a box narrower than
// the one it renders in and every block comes out too tall.
func measureWidthEMU(shapeWidth, insetLeft, insetRight int64) int64 {
	const defaultSideMarginEMU = int64(91440) // 7.2pt
	return shapeWidth - insetLeft - insetRight + 2*defaultSideMarginEMU
}

// panelTextHeightEMU measures the wrapped height of paragraphs at one font
// size. extraSpacingHPt is the per-paragraph space-after in hundredths of a
// point. Returns 0 when there is nothing to measure or no font is available.
func panelTextHeightEMU(paragraphs []string, fontName string, widthEMU int64, fontSizeHPt, extraSpacingHPt int) int64 {
	paragraphs = nonEmptyStrings(paragraphs)
	if len(paragraphs) == 0 || widthEMU <= 0 {
		return 0
	}
	if fontName == "" {
		fontName = panelMeasureDefaultFont
	}
	h, err := textfit.MeasureHeight(textfit.Params{
		WidthEMU: widthEMU,
		// The measurement is unbounded: the caller is asking how tall the text
		// is, not whether it fits a box it has not sized yet.
		HeightEMU:      int64(types.EMUPerInch) * 100,
		FontSizeHPt:    fontSizeHPt,
		FontName:       fontName,
		Paragraphs:     paragraphs,
		LineSpacing:    panelContentLineHeight,
		ExtraSpacingPt: float64(extraSpacingHPt) / 100.0,
	})
	if err != nil || h <= 0 {
		return 0
	}
	return h + panelSlackEMU(fontSizeHPt)
}

// nonEmptyStrings drops blank paragraphs so an absent caption or body costs
// nothing rather than a line of slack.
func nonEmptyStrings(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// panelSlackEMU is the per-block safety margin for one font size.
func panelSlackEMU(fontSizeHPt int) int64 {
	return int64(float64(fontSizeHPt) / 100.0 * panelContentLineHeight * panelBodySlackFrac * float64(types.EMUPerPoint))
}

// panelLineHeightEMU is one rendered line of text at the given size.
func panelLineHeightEMU(fontSizeHPt int) int64 {
	return int64(float64(fontSizeHPt) / 100.0 * panelContentLineHeight * float64(types.EMUPerPoint))
}

// panelBodyParagraphTexts flattens a panel body into the plain paragraph
// strings the renderer will lay out, so the measurement sees the same line
// breaks the bullet parser produces.
func panelBodyParagraphTexts(body string, fontSizeHPt int) []string {
	paras := panelBulletsParagraphs(body, fontSizeHPt)
	if len(paras) == 0 {
		return nil
	}
	texts := make([]string, 0, len(paras))
	for _, p := range paras {
		var sb strings.Builder
		for _, r := range p.Runs {
			sb.WriteString(r.Text)
		}
		if s := sb.String(); s != "" {
			texts = append(texts, s)
		}
	}
	return texts
}

// panelBodyBoxHeight is the height one panel body rectangle needs for its text:
// the measured block plus the box's own top and bottom insets, floored at
// panelBodyMinLines. Returns 0 for an empty body so the caller can keep its
// default.
func panelBodyBoxHeight(body, fontName string, panelWidth int64) int64 {
	texts := panelBodyParagraphTexts(body, panelBodyFontSize)
	if len(texts) == 0 {
		return 0
	}
	textH := panelTextHeightEMU(
		texts, fontName,
		measureWidthEMU(panelWidth, panelBodyMarginLeft, panelBodyMarginRight),
		panelBodyFontSize, panelBulletSpaceAfter,
	)
	if textH <= 0 {
		return 0
	}
	if floor := int64(panelBodyMinLines * float64(panelLineHeightEMU(panelBodyFontSize))); textH < floor {
		textH = floor
	}
	return textH + panelBodyMarginTop + panelBodyMarginBottom
}

// panelColumnsLayout returns the vertical bands of a columns panel group and
// the y-offset that centres it in its bounds.
//
// The icon, header and gap bands keep the sizes they have always had — they are
// fixed furniture, not content — and only the body band is resized, to the
// tallest body in the row so the panels keep a shared baseline. When the bodies
// need everything available (or cannot be measured) the result is exactly the
// old geometry, offset 0.
func panelColumnsLayout(bounds types.BoundingBox, panels []nativePanelData, panelWidth int64, fontName string) (offsetY, iconBandCY, headerCY, gapCY, bodyCY int64) {
	iconBandCY, headerCY, gapCY, bodyCY = panelColumnsBands(bounds.Height, panelsHaveIcons(panels))

	var need int64
	for i := range panels {
		if h := panelBodyBoxHeight(panels[i].body, fontName, panelWidth); h > need {
			need = h
		}
	}
	if need > 0 && need < bodyCY {
		bodyCY = need
	}
	used := iconBandCY + headerCY + gapCY + bodyCY
	if used < bounds.Height {
		offsetY = (bounds.Height - used) / 2
	}
	return offsetY, iconBandCY, headerCY, gapCY, bodyCY
}

// statCardBoxHeight is the height one stat card needs: its hero, caption and
// body lines measured at their own sizes, plus the insets and any icon band.
// Returns 0 when the card carries no text.
func statCardBoxHeight(panel nativePanelData, fontName string, cardW, iconBandCY int64) int64 {
	hero, caption, body := statCardParts(panel)
	textWidth := measureWidthEMU(cardW, statCardInset, statCardInset)

	var h int64
	h += panelTextHeightEMU([]string{hero}, fontName, textWidth, statCardValueFontSize, 0)
	h += panelTextHeightEMU([]string{caption}, fontName, textWidth, statCardLabelFontSize, statCardCaptionSpaceAfter)
	h += panelTextHeightEMU([]string{body}, fontName, textWidth, statCardBodyFontSize, 0)
	if h <= 0 {
		return 0
	}
	return h + 2*statCardInset + iconBandCY
}

// statCardsLayout returns the card height and the y-offset that centres the
// grid in its bounds. Every card in the grid gets the tallest card's height, so
// the row keeps one baseline; a grid whose content needs the full box (or
// cannot be measured) keeps the old full-height geometry.
func statCardsLayout(bounds types.BoundingBox, panels []nativePanelData, rows int, cardW int64, fontName string) (offsetY, cardH int64) {
	vGapTotal := int64(rows-1) * statCardGap
	cardH = (bounds.Height - vGapTotal) / int64(rows)

	var need int64
	for i := range panels {
		iconBandCY := statCardIconBandCY(cardH, len(panels[i].iconSVG) > 0)
		if h := statCardBoxHeight(panels[i], fontName, cardW, iconBandCY); h > need {
			need = h
		}
	}
	if need > 0 && need < cardH {
		cardH = need
	}
	used := int64(rows)*cardH + vGapTotal
	if used < bounds.Height {
		offsetY = (bounds.Height - used) / 2
	}
	return offsetY, cardH
}

// panelGroupBounds is the group rectangle for a content-sized panel group: the
// original box narrowed to the height the children occupy, so the group's own
// extent matches what it draws.
func panelGroupBounds(bounds types.BoundingBox, offsetY, usedHeight int64) pptx.RectEmu {
	if usedHeight <= 0 || usedHeight > bounds.Height {
		usedHeight = bounds.Height
		offsetY = 0
	}
	return pptx.RectEmu{X: bounds.X, Y: bounds.Y + offsetY, CX: bounds.Width, CY: usedHeight}
}
