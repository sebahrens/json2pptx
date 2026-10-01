package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// recordPictureFrame remembers the final frame of an authored content image.
func (ctx *singlePassContext) recordPictureFrame(slideNum int, frame types.BoundingBox) {
	if frame.Width <= 0 || frame.Height <= 0 {
		return
	}
	if ctx.pictureFrames == nil {
		ctx.pictureFrames = make(map[int][]types.BoundingBox)
	}
	ctx.pictureFrames[slideNum] = append(ctx.pictureFrames[slideNum], frame)
}

// footerBandTop is the top edge of the footer chrome band: the layout's
// resolved footer top, or the bottom 8% of the slide when it has none.
func (ctx *singlePassContext) footerBandTop(slideNum int) int64 {
	if frame := ctx.chromeFrameForSlide(slideNum); frame.HasFooter && frame.FooterTop > 0 {
		return frame.FooterTop
	}
	return ctx.slideHeight * 92 / 100
}

// pictureCoversFooterBand reports whether an inserted content picture reaches
// into the footer band. Chrome contrast is resolved against the layout
// background only, so footer text drawn there lands on unverified photo
// pixels (modern's full-bleed Title Only layout).
func (ctx *singlePassContext) pictureCoversFooterBand(slideNum int) bool {
	frames := ctx.pictureFrames[slideNum]
	if len(frames) == 0 || ctx.slideHeight <= 0 {
		return false
	}
	top := ctx.footerBandTop(slideNum)
	for _, f := range frames {
		if f.Y+f.Height > top && f.Y < ctx.slideHeight {
			return true
		}
	}
	return false
}

// suppressChromeOverPicture reports whether the slide's footer chrome must be
// omitted because a picture covers the footer band, emitting
// CHROME_OVER_IMAGE when it does (go-slide-creator-3bph8).
func (ctx *singlePassContext) suppressChromeOverPicture(slideNum int) bool {
	if !ctx.pictureCoversFooterBand(slideNum) {
		return false
	}
	ctx.emitFitFinding(patterns.ChromeOverImage(slidepath.SlideField(slideNum-1, "chrome"), slideNum))
	return true
}
