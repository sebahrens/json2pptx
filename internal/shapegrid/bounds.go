package shapegrid

import "github.com/ahrens/go-slide-creator/internal/pptx"

// Standard 16:9 slide dimensions in EMU.
const (
	DefaultSlideWidthEMU  int64 = 12192000
	DefaultSlideHeightEMU int64 = 6858000

	// MinBottomMarginEMU is the minimum reserved space from the slide bottom
	// for footer clearance (~0.4 inches). Applied when no footer placeholder exists.
	MinBottomMarginEMU int64 = 365760 // 0.4 * 914400
)

// ContentZone defines the template-derived safe content area between title and footer.
// All values are in EMU. This is the single source of truth for where content can be placed.
type ContentZone struct {
	TitleBottom int64 // Y of title bottom edge
	FooterTop   int64 // Y of footer top edge
	LeftMargin  int64 // X of left content edge
	RightEdge   int64 // X of right content edge
	SlideWidth  int64
	SlideHeight int64
}

// DefaultBoundsFromZone derives grid bounds from a ContentZone, applying a gap (in points)
// between the title/footer edges and the content area.
func DefaultBoundsFromZone(zone ContentZone, gapPt float64) pptx.RectEmu {
	gapEMU := int64(gapPt * 12700)
	top := zone.TitleBottom + gapEMU
	bottom := zone.FooterTop - gapEMU
	height := bottom - top
	if height < 0 {
		height = 0
	}
	width := zone.RightEdge - zone.LeftMargin
	if width < 0 {
		width = 0
	}
	return pptx.RectEmu{
		X:  zone.LeftMargin,
		Y:  top,
		CX: width,
		CY: height,
	}
}

// ClampBoundsToZone ensures that bounds do not extend above the title bottom edge
// or below the footer top edge. The bounds are adjusted inward if they overlap chrome.
func ClampBoundsToZone(bounds pptx.RectEmu, zone ContentZone) pptx.RectEmu {
	// Clamp top edge: content must not start above title bottom
	if bounds.Y < zone.TitleBottom {
		delta := zone.TitleBottom - bounds.Y
		bounds.Y = zone.TitleBottom
		bounds.CY -= delta
		if bounds.CY < 0 {
			bounds.CY = 0
		}
	}
	// Clamp bottom edge: content must not extend below footer top
	if bounds.Y+bounds.CY > zone.FooterTop {
		bounds.CY = zone.FooterTop - bounds.Y
		if bounds.CY < 0 {
			bounds.CY = 0
		}
	}
	return bounds
}

// Default grid bounds as percentages of slide dimensions — content area below title.
var DefaultGridBoundsPct = [4]float64{5, 18, 90, 72} // x%, y%, w%, h%

// BoundsFromPercentages converts percentage-based bounds (x%, y%, w%, h%)
// to absolute EMU coordinates using the given slide dimensions.
// If slideWidth or slideHeight is 0, the 16:9 defaults are used.
func BoundsFromPercentages(xPct, yPct, wPct, hPct float64, slideWidth, slideHeight int64) pptx.RectEmu {
	if slideWidth <= 0 {
		slideWidth = DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = DefaultSlideHeightEMU
	}
	return pptx.RectEmu{
		X:  PctToEMU(xPct, slideWidth),
		Y:  PctToEMU(yPct, slideHeight),
		CX: PctToEMU(wPct, slideWidth),
		CY: PctToEMU(hPct, slideHeight),
	}
}

// BoundsFromPlaceholder returns the bounds of a placeholder rectangle as grid bounds.
func BoundsFromPlaceholder(ph pptx.RectEmu) pptx.RectEmu {
	return ph
}

// BoundsFromTitleAndFooter computes grid bounds from title and footer rectangles,
// placing the grid between them with a gap (in points).
// slideWidth is the actual slide width in EMU; if > 0, the grid width is computed
// as slideWidth - 2*title.X (symmetric margins) to avoid inheriting a narrow title width.
func BoundsFromTitleAndFooter(title, footer pptx.RectEmu, gapPt float64, slideWidth int64) pptx.RectEmu {
	gapEMU := int64(gapPt * 12700) // points to EMU
	top := title.Y + title.CY + gapEMU
	bottom := footer.Y - gapEMU

	height := bottom - top
	if height < 0 {
		height = 0
	}

	// Use slide width with symmetric margins when available, otherwise fall back to title width.
	cx := title.CX
	if slideWidth > 0 {
		w := slideWidth - 2*title.X
		if w > cx {
			cx = w
		}
	}

	return pptx.RectEmu{
		X:  title.X,
		Y:  top,
		CX: cx,
		CY: height,
	}
}

// DefaultBounds returns the default grid bounds in EMU using the given slide dimensions.
// If slideWidth or slideHeight is 0, the 16:9 defaults are used.
func DefaultBounds(slideWidth, slideHeight int64) pptx.RectEmu {
	return BoundsFromPercentages(
		DefaultGridBoundsPct[0],
		DefaultGridBoundsPct[1],
		DefaultGridBoundsPct[2],
		DefaultGridBoundsPct[3],
		slideWidth, slideHeight,
	)
}
