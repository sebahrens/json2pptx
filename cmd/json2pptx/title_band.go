package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// bodyZoneTitleGapPt is the distance from the bottom of a measured
// (top-anchored) title's text to the top of the body zone: 18pt
// (go-slide-creator-wntyw). It absorbs small measurement differences between
// the estimator and the renderer, and it is the one fixed start line the
// eye finds on every slide; the standard gridChromeGapPt is part of it.
const bodyZoneTitleGapPt = 18.0

// reserveMeasuredTitle pulls the content zone's TitleBottom up from the title
// placeholder's bottom edge to the bottom of the title's measured text when the
// title is top-anchored (go-slide-creator-pymy7).
//
// Templates such as p-style draw a tall, top-anchored title box (~100pt) that
// a one-line 28pt title fills only at the top; reserving the whole box left a
// dead band between the title and every pattern / shape_grid. The rule is
// template-agnostic: it reads the layout's resolved title anchor, font size,
// line spacing and caps, and it never moves content above the text. Centre and
// bottom anchored titles, slides without title text, and zones whose title edge
// came from something other than the title box (a canvas headline, a
// disclosure reservation) keep their geometry. Virtual override bounds that
// start just below the old edge move up by the same amount.
func reserveMeasuredTitle(g GridGeometry, slide SlideInput, layouts []types.LayoutMetadata) GridGeometry {
	if g.Zone == nil {
		return g
	}
	id := g.LayoutID
	if id == "" {
		id = canonicalGridLayoutID(slide.LayoutID, layouts)
	}
	layout := findLayoutByID(layouts, id)
	if layout == nil {
		return g
	}
	var title *types.PlaceholderInfo
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == types.PlaceholderTitle {
			title = &layout.Placeholders[i]
			break
		}
	}
	if title == nil || title.Anchor != "t" || title.FontSize <= 0 {
		return g
	}
	boxBottom := title.Bounds.Y + title.Bounds.Height
	if g.Zone.TitleBottom != boxBottom {
		return g
	}
	_, text := extractTitleText(slide)
	if text == "" {
		return g
	}
	textBottom, ok := measuredTitleBottom(title, text)
	if !ok || textBottom >= boxBottom {
		return g
	}

	zone := *g.Zone
	zone.TitleBottom = textBottom
	g.Zone = &zone

	if g.OverrideBounds != nil {
		gap := int64(gridChromeGapPt * 12700)
		b := *g.OverrideBounds
		if b.Y >= boxBottom && b.Y <= boxBottom+2*gap {
			delta := boxBottom - textBottom
			b.Y -= delta
			b.CY += delta
			g.OverrideBounds = &b
		}
	}
	return g
}

// measuredTitleBottom estimates the Y (EMU) where a top-anchored title's text
// ends, plus the part of bodyZoneTitleGapPt the zone's own gap does not cover. ok is false when the text cannot be
// measured (no font cache), in which case the caller keeps the box edge.
func measuredTitleBottom(title *types.PlaceholderInfo, text string) (int64, bool) {
	if title.TextCaps {
		text = strings.ToUpper(text)
	}
	lineSpacing := 1.2
	if title.LineSpacingPct > 0 {
		lineSpacing = 1.2 * float64(title.LineSpacingPct) / 100.0
	}
	h, err := textfit.MeasureHeight(textfit.Params{
		WidthEMU:    title.Bounds.Width,
		HeightEMU:   title.Bounds.Height,
		FontSizeHPt: title.FontSize,
		FontName:    title.FontFamily,
		Paragraphs:  []string{text},
		LineSpacing: lineSpacing,
	})
	if err != nil || h <= 0 {
		return 0, false
	}
	// The zone's bounds add gridChromeGapPt below TitleBottom; reserve the
	// rest of bodyZoneTitleGapPt here.
	safety := int64((bodyZoneTitleGapPt - gridChromeGapPt) * 12700)
	return title.Bounds.Y + h + safety, true
}
