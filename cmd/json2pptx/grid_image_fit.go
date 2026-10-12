package main

import "github.com/sebahrens/json2pptx/internal/shapegrid"

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
			if spec == nil {
				continue
			}
			if out == nil {
				out = map[*shapegrid.ImageSpec]bool{}
			}
			out[spec] = true
		}
	}
	return out
}

// fillDefaultFitImages lets image cells without an explicit cell.fit occupy
// their whole cell. The generator preserves the SVG or raster source aspect
// ratio using image.fit (cover by default, or contain). Explicit cell.fit modes
// retain their square frame, useful for headshots and other square treatments.
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

// imageOnlyGrid identifies an image composition whose explicit slide-relative
// bounds take priority over the template's title, footer and side decoration.
func imageOnlyGrid(input *ShapeGridInput) bool {
	hasImage := false
	for _, row := range input.Rows {
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			if cell.Image == nil {
				return false
			}
			hasImage = true
		}
	}
	return hasImage
}
