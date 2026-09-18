package generator

import (
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/template"
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

// clampContentPlaceholdersToChrome shrinks content placeholders (body,
// generic content, picture, chart, table) whose bottom edge would run into the
// takeaway/source band stack, so body text, charts, and tables stop above the
// band instead of rendering underneath it. Title, subtitle, and footer chrome
// placeholders are left untouched. A frame that does not fit is ignored: the
// band is not emitted in that case, so there is nothing to reserve.
func clampContentPlaceholdersToChrome(slide *slideXML, frame template.ChromeFrame) {
	if slide == nil || !frame.Fits {
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
		if xfrm.Offset.Y+xfrm.Extent.CY <= limit || xfrm.Offset.Y >= limit {
			continue
		}
		xfrm.Extent.CY = limit - xfrm.Offset.Y
	}
}
