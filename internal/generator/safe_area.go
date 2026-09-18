package generator

import "github.com/sebahrens/json2pptx/internal/pptx"

const defaultSlideWidthEMU int64 = 12192000

// SlideChromeFrame is the dimension-relative contract used by content
// reservation and late shape emission. Every rectangle is guaranteed to lie
// inside the declared slide canvas.
type SlideChromeFrame struct {
	Canvas   pptx.RectEmu
	Content  pptx.RectEmu
	Takeaway pptx.RectEmu
	Source   pptx.RectEmu
}

func ResolveSlideChromeFrame(slideWidth, slideHeight int64, hasTakeaway, hasSource bool) SlideChromeFrame {
	if slideWidth <= 0 {
		slideWidth = defaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	marginX := slideWidth * 375 / 10000
	bottom := slideHeight * 290 / 10000
	gap := slideHeight * 70 / 10000
	sourceH := slideHeight * 292 / 10000
	takeawayH := slideHeight * 525 / 10000
	if sourceH < 160000 {
		sourceH = 160000
	}
	if takeawayH < 300000 {
		takeawayH = 300000
	}

	frame := SlideChromeFrame{Canvas: pptx.RectEmu{CX: slideWidth, CY: slideHeight}}
	y := slideHeight - bottom
	if hasSource {
		y -= sourceH
		frame.Source = pptx.RectEmu{X: marginX, Y: y, CX: slideWidth - 2*marginX, CY: sourceH}
		y -= gap
	}
	if hasTakeaway {
		y -= takeawayH
		frame.Takeaway = pptx.RectEmu{X: marginX, Y: y, CX: slideWidth - 2*marginX, CY: takeawayH}
		y -= gap
	}
	frame.Content = pptx.RectEmu{X: marginX, Y: 0, CX: slideWidth - 2*marginX, CY: y}
	if frame.Content.CY < 0 {
		frame.Content.CY = 0
	}
	return frame
}

func TakeawayBandTopForHeight(slideHeight int64, hasSource bool) int64 {
	return ResolveSlideChromeFrame(0, slideHeight, true, hasSource).Takeaway.Y
}
