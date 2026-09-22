package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// generateCanvasHeadline creates the visible title used by content-bearing
// blank-canvas slides. It uses theme color/font references and a reserved band
// shared with the grid geometry code, so the title never depends on a template
// placeholder that the true blank layout does not have.
func generateCanvasHeadline(text string, bounds pptx.RectEmu, ctx *GridDiagramContext) ([]byte, error) {
	font := "+mj-lt"
	if ctx != nil && strings.TrimSpace(ctx.TitleFont) != "" {
		font = ctx.TitleFont
	}
	return pptx.GenerateShape(pptx.ShapeOptions{
		ID:       390,
		Name:     "Canvas Headline",
		Bounds:   bounds,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		Line:     pptx.NoLine(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{0, 0, 0, 0},
			AutoFit: "normAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align: "l",
				Runs: []pptx.Run{{
					Text:       strings.TrimSpace(text),
					FontSize:   2800,
					Bold:       true,
					Dirty:      true,
					Lang:       "en-US",
					FontFamily: font,
					Color:      pptx.SchemeFill("tx1"),
				}},
			}},
		},
	})
}
