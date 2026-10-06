package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

// diagramTintFill lightens a theme accent the way the pattern engine does:
// lumMod / lumOff on HSL lightness (PowerPoint's "Lighter N%" swatches), so a
// native diagram and a pattern in one deck tint the same accent to the same
// colour (go-slide-creator-7if28). The former a:tint mixed toward white in
// linear light, which pulls a saturated orange to salmon pink (#FD5108 at 20%
// rendered #FFE9E7 where the patterns' tint is the peach #FFDCCE).
// Explicit colors are already the author's final choice and remain exact.
func diagramTintFill(color string, retained, offset int) pptx.Fill {
	base := pptx.ResolveColorString(color)
	if !pptx.IsSchemeColor(color) || offset == 0 {
		return base
	}
	return pptx.SchemeFill(color, pptx.LumMod(retained), pptx.LumOff(offset))
}

// diagramTintMods is diagramTintFill as colour modifiers, for the contrast
// helpers that need the colour a viewer sees.
func diagramTintMods(retained, offset int) patterns.ColorMods {
	if offset == 0 {
		return patterns.ColorMods{}
	}
	return patterns.ColorMods{LumMod: retained, LumOff: offset}
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
