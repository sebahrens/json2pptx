package generator

import "github.com/sebahrens/json2pptx/internal/pptx"

// diagramTintFill blends a theme accent toward white in linear-light RGB. A
// luminance offset brightens fully saturated accents without desaturating
// them, producing fluorescent panels; DrawingML tint keeps the brand hue soft.
// Explicit colors are already the author's final choice and remain exact.
func diagramTintFill(color string, retained, offset int) pptx.Fill {
	base := pptx.ResolveColorString(color)
	if !pptx.IsSchemeColor(color) || offset == 0 {
		return base
	}
	return pptx.SchemeFill(color, pptx.Tint(retained))
}
