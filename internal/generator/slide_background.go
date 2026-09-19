package generator

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Slide backgrounds (go-slide-creator-uy5s).
//
// A slide could only set a background IMAGE. The standard "one dark slide in a
// light deck" — a statement, a section break, a pull quote — had to be faked
// with a full-bleed shape_grid cell, which then fought the layout's own title
// placeholder for the same space. And an image background had no scrim, so a
// hero photo rendered the template's dark title text over whatever the photo
// happened to be: the contrast auto-fixer reads a layout's solid fill, and an
// image has no colour to read.

// defaultOverlayColor is the scrim colour when the author names none. dk1 is
// the template's own ink colour, so the scrim sits in the template's palette
// rather than introducing a foreign black.
const defaultOverlayColor = "dk1"

// defaultOverlayAlpha is the scrim opacity when the author gives none. 0.45
// darkens a photo enough for white text to clear WCAG AA on most images
// without hiding the picture.
const defaultOverlayAlpha = 0.45

// backgroundSolidFillXML renders a <p:bg> solid fill for a slide background
// colour. The colour is a scheme name (emitted as <a:schemeClr> so it follows
// the template's theme) or a hex value.
func backgroundSolidFillXML(color string) string {
	if color == "" {
		return ""
	}
	trimmed := strings.TrimPrefix(color, "#")
	if pptx.SchemeColorNames[trimmed] {
		return `<p:bg><p:bgPr><a:solidFill><a:schemeClr val="` + trimmed + `"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`
	}
	return `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="` + strings.ToUpper(trimmed) + `"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`
}

// insertBackgroundColor injects a solid-fill <p:bg> before <p:spTree>, the same
// slot insertBackgroundImage uses. A slide that sets both gets the image, which
// paints over the fill anyway.
func insertBackgroundColor(slideData []byte, color string) []byte {
	bgXML := backgroundSolidFillXML(color)
	if bgXML == "" {
		return slideData
	}
	return insertBeforeSpTree(slideData, bgXML)
}

// insertBeforeSpTree splits the slide at <p:spTree> and injects xml ahead of it.
func insertBeforeSpTree(slideData []byte, xmlFragment string) []byte {
	for _, marker := range [][]byte{[]byte("<p:spTree>"), []byte("<p:spTree ")} {
		if pos := bytes.Index(slideData, marker); pos != -1 {
			result := make([]byte, 0, len(slideData)+len(xmlFragment))
			result = append(result, slideData[:pos]...)
			result = append(result, xmlFragment...)
			result = append(result, slideData[pos:]...)
			return result
		}
	}
	return slideData
}

// backgroundScrimXML renders the full-bleed scrim rectangle for a background
// overlay. It is a normal shape, inserted at the START of the shape tree so it
// paints above the background (which lives in <p:bg>, outside the tree) and
// below every placeholder, grid cell and picture on the slide.
func backgroundScrimXML(overlay *BackgroundOverlay, slideWidth, slideHeight int64, id uint32) ([]byte, error) {
	if overlay == nil {
		return nil, nil
	}
	color := overlay.Color
	if color == "" {
		color = defaultOverlayColor
	}
	alpha := overlay.Alpha
	if alpha <= 0 {
		alpha = defaultOverlayAlpha
	}
	xml, err := shapegrid.GenerateImageOverlayXML(
		&shapegrid.OverlaySpec{Color: color, Alpha: alpha},
		id,
		pptx.RectEmu{X: 0, Y: 0, CX: slideWidth, CY: slideHeight},
	)
	if err != nil {
		return nil, fmt.Errorf("background overlay: %w", err)
	}
	return xml, nil
}

// effectiveSlideBackgroundHex returns the colour text contrast should be judged
// against for a slide: its own background colour when it sets one, otherwise
// the scrim colour when it dims a photo, otherwise "" so the caller falls back
// to the layout's fill. A scheme name is resolved through the theme, because
// the contrast check compares luminance, not names.
func effectiveSlideBackgroundHex(bg *BackgroundImage, themeColors []types.ThemeColor) string {
	if bg == nil {
		return ""
	}
	// An opaque-enough scrim decides what the text sits on, not the photo.
	if bg.Overlay != nil {
		alpha := bg.Overlay.Alpha
		if alpha <= 0 {
			alpha = defaultOverlayAlpha
		}
		if alpha >= scrimOpaqueEnough {
			if hex := resolveBackgroundHex(bg.Overlay.Color, themeColors); hex != "" {
				return hex
			}
			return resolveBackgroundHex(defaultOverlayColor, themeColors)
		}
	}
	// A photo with no scrim (or a barely-there one) has no single colour: the
	// caller reports TEXT_OVER_IMAGE_UNVERIFIED rather than guessing one.
	if bg.Path != "" && bg.Overlay == nil {
		return ""
	}
	return resolveBackgroundHex(bg.Color, themeColors)
}

// scrimOpaqueEnough is the alpha above which the scrim, not the image, decides
// the text's background. Below it the photo still shows through enough that a
// contrast verdict computed from the scrim colour would be a guess.
const scrimOpaqueEnough = 0.35

// resolveBackgroundHex turns a background colour — a scheme name or a hex
// value, with or without a leading "#" — into the "#RRGGBB" form the contrast
// checker parses. Returns "" when the value is empty or unresolvable.
func resolveBackgroundHex(color string, themeColors []types.ThemeColor) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(color), "#")
	if trimmed == "" {
		return ""
	}
	if pptx.SchemeColorNames[trimmed] {
		return resolveSchemeColorToHex(trimmed, themeColors)
	}
	if len(trimmed) == 6 && isHexDigits(trimmed) {
		return "#" + strings.ToUpper(trimmed)
	}
	return ""
}

// isHexDigits reports whether s is all hexadecimal digits.
func isHexDigits(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
