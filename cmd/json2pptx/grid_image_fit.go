package main

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"math"
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
			old := c.Bounds
			c.Bounds = c.CellBounds
			for j := i + 1; j < len(result.Cells) && isLayerOf(result.Cells[j], *c); j++ {
				layer := &result.Cells[j]
				layer.Bounds = fitImageLayerBounds(layer.Bounds, old, c.Bounds)
				if layer.IconBounds.CX > 0 && layer.IconBounds.CY > 0 {
					layer.IconBounds = fitImageLayerBounds(layer.IconBounds, old, c.Bounds)
				}
			}
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

func fullSlideImageGrid(input *ShapeGridInput) bool {
	if input == nil || input.Bounds == nil || input.BoundsRelativeToContentArea || !imageOnlyGrid(input) {
		return false
	}
	b := input.Bounds
	return b.X == 0 && b.Y == 0 && b.Width == 100 && b.Height == 100
}

func fitImageLayerBounds(rect, old, target pptx.RectEmu) pptx.RectEmu {
	if old.CX <= 0 || old.CY <= 0 {
		return rect
	}
	sx, sy := float64(target.CX)/float64(old.CX), float64(target.CY)/float64(old.CY)
	return pptx.RectEmu{X: target.X + int64(math.Round(float64(rect.X-old.X)*sx)), Y: target.Y + int64(math.Round(float64(rect.Y-old.Y)*sy)), CX: int64(math.Round(float64(rect.CX) * sx)), CY: int64(math.Round(float64(rect.CY) * sy))}
}
