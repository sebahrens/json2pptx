package generator

import (
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// loadTemplateProfile builds (or fetches from the content-hash cache) the
// template profile for the generation run. The profile carries the per-layout
// chrome geometry — footer regions, body column, takeaway/source bands — that
// late shape emission and placeholder reservation read. A template the
// profiler cannot open leaves ctx.profile nil, and chrome falls back to
// slide-relative margins.
func (ctx *singlePassContext) loadTemplateProfile(templatePath string) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		slog.Debug("template profile unavailable; chrome uses slide fallback", slog.String("error", err.Error()))
		return
	}
	defer func() { _ = reader.Close() }()
	profile, err := template.BuildProfile(reader)
	if err != nil {
		slog.Debug("template profile build failed; chrome uses slide fallback", slog.String("error", err.Error()))
		return
	}
	ctx.profile = profile
}

// clampToTemplateGridMargin narrows a late-bound visual frame (a placeholder
// chart / diagram whose transform may be inherited, so the slide-level
// placeholder clamp never saw it) to the content column when the template
// declares a grid margin_pct (go-slide-creator-5ms8c). Templates without a
// grid margin keep the placeholder's own frame.
func (ctx *singlePassContext) clampToTemplateGridMargin(slideNum int, bounds types.BoundingBox) types.BoundingBox {
	if ctx.profile == nil {
		return bounds
	}
	if grid := types.TemplateGridOf(ctx.profile.Layouts); grid == nil || grid.MarginPct <= 0 {
		return bounds
	}
	bounds.X, bounds.Width = clampHorizontalToFrame(bounds.X, bounds.Width, ctx.chromeFrameForSlide(slideNum).Content)
	return bounds
}

// chromeFrameForLayout resolves the chrome frame for a slide on layoutID from
// the template profile. Layouts the profile does not know (synthesized
// layouts) resolve against the profile's One Content reference layout.
func (ctx *singlePassContext) chromeFrameForLayout(layoutID string, hasTakeaway, hasSource bool) template.ChromeFrame {
	if ctx.profile == nil {
		return template.ResolveChromeFrame(nil, nil, ctx.slideWidth, ctx.slideHeight, hasTakeaway, hasSource)
	}
	return ctx.profile.ChromeFrame(layoutID, hasTakeaway, hasSource)
}

// chromeFrameForSlide resolves the chrome frame for an output slide number,
// reserving the bands the slide actually carries.
func (ctx *singlePassContext) chromeFrameForSlide(slideNum int) template.ChromeFrame {
	_, hasTakeaway := ctx.slideTakeaways[slideNum]
	_, hasSource := ctx.slideSources[slideNum]
	return ctx.chromeFrameForLayout(ctx.slideContentMap[slideNum].LayoutID, hasTakeaway, hasSource)
}

// clampBoundsToChrome applies the same reservation as
// clampContentPlaceholdersToChrome to late-bound visual content. Native tables
// are generated after placeholder population and can resolve an inherited
// layout transform even when the slide-level placeholder was already clamped.
func clampBoundsToChrome(bounds types.BoundingBox, frame template.ChromeFrame) types.BoundingBox {
	if bounds.Height <= 0 {
		return bounds
	}
	bounds.X, bounds.Width = clampHorizontalToFrame(bounds.X, bounds.Width, frame.Content)
	if !frame.Fits {
		return bounds
	}
	limit := frame.Content.Bottom()
	if bottom := bounds.Y + bounds.Height; bounds.Y < limit && bottom > limit {
		bounds.Height = limit - bounds.Y
	}
	return bounds
}

// clampContentPlaceholdersToChrome clips content placeholders (body, generic
// content, picture, chart, table) horizontally around side artwork and, when
// the band fits, vertically above the takeaway/source stack. Title, subtitle,
// and footer chrome placeholders are left untouched. A frame that does not
// fit still reserves side artwork but does not shorten content vertically.
func clampContentPlaceholdersToChrome(slide *slideXML, frame template.ChromeFrame) {
	if slide == nil {
		return
	}
	limit := frame.Content.Bottom()
	for i := range slide.CommonSlideData.ShapeTree.Shapes {
		shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
		ph := shape.NonVisualProperties.NvPr.Placeholder
		xfrm := shape.ShapeProperties.Transform
		if ph == nil || xfrm == nil {
			continue
		}
		switch ph.Type {
		case "title":
			alignTitleWithInsetContent(xfrm, frame)
			continue
		case "ctrTitle", "subTitle", "dt", "ftr", "sldNum", "hdr":
			continue
		}
		xfrm.Offset.X, xfrm.Extent.CX = clampHorizontalToFrame(xfrm.Offset.X, xfrm.Extent.CX, frame.Content)
		if !frame.Fits {
			continue
		}
		if xfrm.Offset.Y+xfrm.Extent.CY <= limit || xfrm.Offset.Y >= limit {
			continue
		}
		xfrm.Extent.CY = limit - xfrm.Offset.Y
	}
}

// alignTitleWithInsetContent moves a title that starts left of a content
// column side artwork pushed inward, so the title and the content share one
// left edge instead of the title hugging the art (go-slide-creator-oa0ru). The
// title keeps its width — title fit is measured against the layout's
// placeholder width — unless that would run it off the canvas.
func alignTitleWithInsetContent(xfrm *transformXML, frame template.ChromeFrame) {
	if !frame.SideDecorInset || xfrm.Extent.CX <= 0 || frame.Content.CX <= 0 {
		return
	}
	x, width := xfrm.Offset.X, xfrm.Extent.CX
	if x >= frame.Content.X || x+width <= frame.Content.X {
		return
	}
	newX := frame.Content.X
	// Never closer to the right canvas edge than half the content's right
	// margin.
	rightMargin := frame.Canvas.CX - (frame.Content.X + frame.Content.CX)
	if limit := frame.Canvas.CX - rightMargin/2; newX+width > limit {
		width = limit - newX
	}
	if width <= 0 {
		return
	}
	xfrm.Offset.X, xfrm.Extent.CX = newX, width
}

// alignFooterWithInsetContent returns footer positions whose left footer text
// (the date slot the left text box starts at) begins on the content column
// when side artwork pushed that column inward, so title, content, source and
// footer share one left edge (go-slide-creator-oa0ru). positions is returned
// unchanged otherwise; the cached map is never mutated.
func alignFooterWithInsetContent(positions map[string]*transformXML, frame template.ChromeFrame) map[string]*transformXML {
	dt, ok := positions["type:dt"]
	if !frame.SideDecorInset || !ok || dt == nil || dt.Offset.X >= frame.Content.X {
		return positions
	}
	shift := frame.Content.X - dt.Offset.X
	if shift >= dt.Extent.CX {
		return positions
	}
	out := make(map[string]*transformXML, len(positions))
	for k, v := range positions {
		out[k] = v
	}
	moved := *dt
	moved.Offset.X += shift
	moved.Extent.CX -= shift
	out["type:dt"] = &moved
	return out
}

func clampHorizontalToFrame(x, width int64, content template.ChromeRect) (int64, int64) {
	if width <= 0 || content.CX <= 0 {
		return x, width
	}
	right := x + width
	if x < content.X && right > content.X {
		x = content.X
	}
	limit := content.X + content.CX
	if right > limit && x < limit {
		right = limit
	}
	if right <= x {
		return x, width
	}
	return x, right - x
}
