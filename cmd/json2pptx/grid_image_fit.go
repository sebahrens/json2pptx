package main

import (
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// defaultFitImageSpecs returns the converted image specs of cells that set no
// explicit `fit`. rows must be convertGridRows(inputRows) (same order).
func defaultFitImageSpecs(inputRows []GridRowInput, rows []shapegrid.Row) map[*shapegrid.ImageSpec]bool {
	var out map[*shapegrid.ImageSpec]bool
	for i, r := range inputRows {
		if i >= len(rows) {
			break
		}
		for j, c := range r.Cells {
			if c == nil || c.Image == nil || c.Fit != "" || j >= len(rows[i].Cells) {
				continue
			}
			spec := rows[i].Cells[j].Image
			if spec == nil || strings.EqualFold(filepath.Ext(spec.Path), ".svg") {
				continue // SVG pictures cannot be cropped; keep the square frame
			}
			if out == nil {
				out = map[*shapegrid.ImageSpec]bool{}
			}
			out[spec] = true
		}
	}
	return out
}

// fillDefaultFitImages lets raster image cells without an explicit `fit`
// occupy their whole cell. shapegrid places every image in a centred square
// (contain) because it cannot know the picture's aspect ratio; the generator
// now cover-crops pictures to their frame (a:srcRect), so the full cell can be
// used without distortion — a photo panel fills its panel instead of shrinking
// to a square in the middle of it. Explicit fit modes keep their square frame
// (and are cover-cropped into it).
func fillDefaultFitImages(result *shapegrid.ResolveResult, specs map[*shapegrid.ImageSpec]bool) {
	if result == nil || len(specs) == 0 {
		return
	}
	for i := range result.Cells {
		c := &result.Cells[i]
		if c.Kind == shapegrid.CellKindImage && c.ImageSpec != nil && specs[c.ImageSpec] && c.CellBounds.CX > 0 && c.CellBounds.CY > 0 {
			c.Bounds = c.CellBounds
		}
	}
}
