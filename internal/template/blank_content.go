package template

import "github.com/sebahrens/json2pptx/internal/types"

// IsTrueBlankLayout excludes legacy "Blank" layouts that still carry a title
// or body placeholder; those must retain their placeholder-based content zone.
func IsTrueBlankLayout(layout *types.LayoutMetadata) bool {
	if layout == nil || EffectiveCanonicalType(layout) != types.CanonicalLayoutBlank {
		return false
	}
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderTitle, types.PlaceholderBody, types.PlaceholderContent:
			return false
		}
	}
	return true
}

// BlankContentRect is the usable rectangle of a truly blank layout. It is
// shared by template discovery and grid rendering so a title-free canvas does
// not inherit another layout's title band. Footer placeholders and recognized
// footer regions still bound the bottom edge.
func BlankContentRect(layout *types.LayoutMetadata, slideWidth, slideHeight int64) types.BoundingBox {
	left := int64(float64(slideWidth) * 0.05)
	top := int64(float64(slideHeight) * 0.05)
	right := int64(float64(slideWidth) * 0.95)
	bottom := int64(float64(slideHeight) * 0.95)
	if layout != nil {
		for _, ph := range layout.Placeholders {
			switch ph.Role {
			case types.PlaceholderRoleFooter, types.PlaceholderRoleDate, types.PlaceholderRolePageNumber:
				if ph.Bounds.Y > slideHeight/2 && ph.Bounds.Y < bottom {
					bottom = ph.Bounds.Y
				}
			}
		}
		if footerTop, ok := FooterTop(layout, slideHeight); ok && footerTop < bottom {
			bottom = footerTop
		}
	}
	if bottom < top {
		bottom = top
	}
	return types.BoundingBox{X: left, Y: top, Width: right - left, Height: bottom - top}
}
