package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

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

// diagramPanelTextFill keeps theme-linked light cells on the template's text
// role. An explicit RGB panel can be dark, so choose absolute black or white
// against that exact color instead of assuming the theme's dk1 is readable.
func diagramPanelTextFill(color string) pptx.Fill {
	if pptx.IsSchemeColor(color) {
		return pptx.SchemeFill("dk1")
	}
	bg, err := svggen.ParseColor("#" + strings.TrimPrefix(color, "#"))
	if err != nil {
		return pptx.SchemeFill("dk1")
	}
	black := svggen.Color{A: 1}
	white := svggen.Color{R: 255, G: 255, B: 255, A: 1}
	if white.ContrastWith(bg) > black.ContrastWith(bg) {
		return pptx.SolidFill("FFFFFF")
	}
	return pptx.SolidFill("000000")
}

func diagramPanelBodyColors(paras []pptx.Paragraph, color string) {
	if pptx.IsSchemeColor(color) {
		return
	}
	text := diagramPanelTextFill(color)
	for i := range paras {
		for j := range paras[i].Runs {
			paras[i].Runs[j].Color = text
		}
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = text
		}
	}
}
