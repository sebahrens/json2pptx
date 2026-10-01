package main

import (
	"github.com/sebahrens/json2pptx/internal/tokens"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// The distance from the bottom of a measured (top-anchored) title's text to
// the top of the body zone is the template grid's title_gap_pt, default
// types.DefaultGridTitleGapPt = 18pt (go-slide-creator-wntyw, -5ms8c). It
// absorbs small measurement differences between the estimator and the
// renderer, and it is the one fixed start line the eye finds on every slide;
// the standard gridChromeGapPt is part of it.

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
	textBottom, ok := measuredTitleBottom(title, text, types.TemplateGridOf(layouts).TitleGapPtOrDefault())
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
// ends, plus the part of the title gap (titleGapPt) the zone's own gap does not cover. ok is false when the text cannot be
// measured (no font cache), in which case the caller keeps the box edge.
func measuredTitleBottom(title *types.PlaceholderInfo, text string, titleGapPt float64) (int64, bool) {
	if title.TextCaps {
		text = strings.ToUpper(text)
	}
	lineSpacing := tokens.LineHeight
	if title.LineSpacingPct > 0 {
		lineSpacing = tokens.LineHeight * float64(title.LineSpacingPct) / 100.0
	}
	bold, width := title.TextBold, title.Bounds.Width
	if textfit.FontSubstituted(title.FontFamily) {
		// The template's title face is not available to the measurer, and the
		// renderer's substitute is unknown too (modern-yellow's "Segoe UI
		// Semibold" measures as regular Arial here but wraps "quarters" onto a
		// second line in LibreOffice). The reservation errs toward the extra
		// line: a heavy-weight family measures bold, and every substituted
		// face gets a width margin (go-slide-creator-b7qqg.13).
		bold = bold || heavyWeightFamily(title.FontFamily)
		width = width * (100 - substitutedTitleWidthMarginPct) / 100
	}
	h, err := textfit.MeasureHeight(textfit.Params{
		WidthEMU:    width,
		HeightEMU:   title.Bounds.Height,
		FontSizeHPt: title.FontSize,
		FontName:    title.FontFamily,
		Paragraphs:  []string{text},
		LineSpacing: lineSpacing,
		// A bold, tracked title (blue-corporate: b="1", spc="300", all caps)
		// wraps well before the regular untracked face says it does
		// (go-slide-creator-b7qqg.13).
		Bold:            bold,
		LetterSpacingPt: float64(title.CharSpacingHPt) / 100.0,
	})
	if err != nil || h <= 0 {
		return 0, false
	}
	// The zone's bounds add gridChromeGapPt below TitleBottom; reserve the
	// rest of the title gap (the template grid's title_gap_pt, default
	// types.DefaultGridTitleGapPt) here.
	safety := int64(max(titleGapPt-gridChromeGapPt, 0) * 12700)
	return title.Bounds.Y + h + safety, true
}

// substitutedTitleWidthMarginPct is the share of the title width the body-zone
// reservation withholds when the title face had to be substituted: the
// substitute's metrics are a guess, and a guess that is a few percent narrow
// puts the pattern into the title's second line.
const substitutedTitleWidthMarginPct = 10

// heavyWeightFamily reports whether a family name itself names a weight above
// regular ("Segoe UI Semibold", "Arial Black"). Such weights ship as separate
// fonts; when one is substituted by a regular face, the bold face is the
// closer width proxy.
func heavyWeightFamily(name string) bool {
	n := strings.ToLower(name)
	for _, w := range []string{"semibold", "semi bold", "demibold", "bold", "black", "heavy"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// reserveBodyAnchor records where the template starts native body content
// (go-slide-creator-e17xy).
//
// The zone's top edge comes from the title (its box, or its measured text —
// reserveMeasuredTitle), so a short pattern on p-style started hard under a
// one-line title while the native bullets of the next slide started at the
// body placeholder, ~70pt lower. Consulting decks hang every slide's content
// from one line. The layout's body / content placeholder top — or, for a
// title-only layout that draws the same title box as the template's
// reference one-content layout, that layout's — is recorded as BodyTop: a
// content-sized block ("auto" / "top") starts there whenever its slack
// allows. Full-area grids start there too (ContentTop) as far as
// bodyStartTop allows, so dense patterns keep the height they are sized for.
func reserveBodyAnchor(g GridGeometry, slide SlideInput, layouts []types.LayoutMetadata) GridGeometry {
	if g.Zone == nil {
		return g
	}
	id := g.LayoutID
	if id == "" {
		id = canonicalGridLayoutID(slide.LayoutID, layouts)
	}
	layout := findLayoutByID(layouts, id)
	if layout == nil || isBlankCanvasLayout(id, layouts) {
		return g
	}
	title := layoutTitlePlaceholder(layout)
	if title == nil {
		return g
	}
	anchor, ok := bodyAnchorTop(layout, title, layouts)
	if !ok {
		return g
	}
	gap := int64(gridChromeGapPt * 12700)
	if anchor >= g.Zone.FooterTop-gap {
		return g
	}
	zone := *g.Zone
	zone.BodyTop = anchor
	if top := bodyStartTop(zone.TitleBottom, title, anchor); top > zone.TitleBottom+gap {
		zone.ContentTop = top
		if g.OverrideBounds != nil && g.OverrideBounds.Y < top {
			b := *g.OverrideBounds
			b.CY -= top - b.Y
			b.Y = top
			if b.CY < 0 {
				b.CY = 0
			}
			g.OverrideBounds = &b
		}
	}
	g.Zone = &zone
	return g
}

// bodyStartTop is where full-area grids start (ContentZone.ContentTop): the
// body line anchor, but never below gridChromeGapPt under the title box — the
// zone every pattern's schema maxima are sized and pinned for
// (TestSchemaMaximaStayReadable). Where a template's body placeholder sits
// further below its title box (p-style ~11pt, modern-yellow ~19pt,
// business-template ~28pt), starting there would shrink that zone and push
// dense content under the readable floor, so those templates start full-area
// grids at the title-box line and only content-sized blocks hang from the
// body line. A measured short title (reserveMeasuredTitle) lifts only the
// zone's limit, never this start line, so pattern slides no longer start
// hard under a one-line title while native body text starts lower.
func bodyStartTop(titleBottom int64, title *types.PlaceholderInfo, anchor int64) int64 {
	base := title.Bounds.Y + title.Bounds.Height
	if titleBottom > base {
		base = titleBottom
	}
	return min64(anchor, base+int64(gridChromeGapPt*12700))
}

// layoutTitlePlaceholder returns the layout's first title placeholder.
func layoutTitlePlaceholder(layout *types.LayoutMetadata) *types.PlaceholderInfo {
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == types.PlaceholderTitle {
			return &layout.Placeholders[i]
		}
	}
	return nil
}

// bodyAnchorTitleTolerance is how far (EMU, ~2pt) two layouts' title boxes may
// differ and still count as the same title band.
const bodyAnchorTitleTolerance = 25400

// bodyAnchorTop returns the top of the native body content the layout's
// slides line up with: the layout's own body / content placeholder, else the
// template's reference one-content layout's when both layouts draw the same
// title box (blank-title / title-only pattern layouts).
func bodyAnchorTop(layout *types.LayoutMetadata, title *types.PlaceholderInfo, layouts []types.LayoutMetadata) (int64, bool) {
	if content, ok := firstBodyOrContentBounds(layout); ok {
		return content.Y, true
	}
	ref := template.ChromeReferenceLayout(layouts)
	if ref == nil || ref.ID == layout.ID {
		return 0, false
	}
	refTitle := layoutTitlePlaceholder(ref)
	if refTitle == nil {
		return 0, false
	}
	absDiff := func(a, b int64) int64 {
		if a > b {
			return a - b
		}
		return b - a
	}
	if absDiff(refTitle.Bounds.Y, title.Bounds.Y) > bodyAnchorTitleTolerance ||
		absDiff(refTitle.Bounds.Height, title.Bounds.Height) > bodyAnchorTitleTolerance {
		return 0, false
	}
	content, ok := firstBodyOrContentBounds(ref)
	if !ok {
		return 0, false
	}
	return content.Y, true
}

// reserveTitleTextEdge records where the slide title's text starts — the
// title placeholder X plus its left inset — so unfilled first-column grid
// text can start on the same line (go-slide-creator-svrpx). When side artwork
// moved the content column right, the generator shifts the title with it
// (go-slide-creator-oa0ru), so the edge follows the column.
func reserveTitleTextEdge(g GridGeometry, slide SlideInput, layouts []types.LayoutMetadata) GridGeometry {
	if g.Zone == nil {
		return g
	}
	id := g.LayoutID
	if id == "" {
		id = canonicalGridLayoutID(slide.LayoutID, layouts)
	}
	layout := findLayoutByID(layouts, id)
	if layout == nil || isBlankCanvasLayout(id, layouts) {
		return g
	}
	title := layoutTitlePlaceholder(layout)
	// Anchor is only resolved (with the inset beside it) for titles parsed
	// from a real template.
	if title == nil || title.Anchor == "" {
		return g
	}
	x := title.Bounds.X
	if g.Zone.SideDecor && g.Zone.LeftMargin > x {
		x = g.Zone.LeftMargin
	}
	zone := *g.Zone
	zone.TextLeft = x + title.TextInsetLeftEMU
	g.Zone = &zone
	return g
}
