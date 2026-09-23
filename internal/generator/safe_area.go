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
		case "title", "ctrTitle", "subTitle", "dt", "ftr", "sldNum", "hdr":
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
