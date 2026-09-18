package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Default 16:9 slide geometry used when validation-time pattern expansion has
// no template analysis to size against. Pattern expansion only uses slide
// dimensions for layout geometry; the diagram specs it embeds (the part the
// svggen checks below care about) do not depend on them.
const (
	validationDefaultSlideWidthEMU  int64 = 12192000
	validationDefaultSlideHeightEMU int64 = 6858000
)

// expandSlidePatternGrid expands a slide-level named pattern (and any
// cell-level nested patterns inside it) into a fresh shape grid so
// validation-time checks see the same diagram cells generate will render.
// It returns nil when the slide has no pattern, already carries a
// shape_grid, or expansion fails — pattern value errors are reported by the
// pattern validator, not here.
func expandSlidePatternGrid(slide *SlideInput, slideIdx int, slideWidth, slideHeight int64, theme *types.ThemeInfo) *ShapeGridInput {
	if slide == nil || slide.Pattern == nil || slide.ShapeGrid != nil {
		return nil
	}
	if slideWidth <= 0 || slideHeight <= 0 {
		slideWidth, slideHeight = validationDefaultSlideWidthEMU, validationDefaultSlideHeightEMU
	}
	ctx := patterns.ExpandContext{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
		SlideIndex:  slideIdx,
	}
	if theme != nil {
		ctx.Theme = *theme
	}
	grid, _, err := expandPattern(slide.Pattern, ctx, patterns.Default())
	if err != nil || grid == nil {
		return nil
	}
	if err := expandNestedCellPatterns(grid, ctx, patterns.Default()); err != nil {
		return nil
	}
	return grid
}

// gridDiagramValidationDiagnostics runs svggen's data validation on every
// diagram surface of grid (cell diagrams, composite sub_diagrams, nested
// grids). Generate aborts the whole deck when a shape_grid diagram fails this
// check ("svggen: validation failed for ..."), so validate reports it as an
// error — keeping validate and generate in agreement (go-slide-creator-yzbo).
// label names the grid's owner in messages ("shape_grid" or
// "pattern chart-insights-split"); basePath is its JSON Pointer.
func gridDiagramValidationDiagnostics(grid *ShapeGridInput, slideIdx int, label, basePath string) []diagnostics.Diagnostic {
	if grid == nil {
		return nil
	}
	var out []diagnostics.Diagnostic
	check := func(spec *types.DiagramSpec, path, where string) {
		if spec == nil || spec.Type == "" {
			return
		}
		if svggen.DefaultRegistry().Get(spec.Type) == nil {
			return // unknown types are reported by the structural grid validator
		}
		// DryRender runs the same request validation + layout pass as the
		// generate path, so the error text matches what generate reports.
		if _, err := svggen.DryRender(&svggen.RequestEnvelope{Type: spec.Type, Title: spec.Title, Data: spec.Data}); err != nil {
			out = append(out, diagnostics.Diagnostic{
				Code:     diagnostics.CodeInvalidGrid,
				Path:     path,
				Message:  fmt.Sprintf("slide %d: %s: %s: %v (generate would abort)", slideIdx+1, label, where, err),
				Severity: diagnostics.SeverityError,
			})
		}
	}
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", basePath, ri, ci)
			where := fmt.Sprintf("row %d cell %d", ri+1, ci+1)
			check(cell.Diagram, cellPath+"/diagram", where+" diagram")
			if cell.Composite != nil {
				check(cell.Composite.SubDiagram, cellPath+"/composite/sub_diagram", where+" composite sub_diagram")
			}
			if cell.Grid != nil {
				out = append(out, gridDiagramValidationDiagnostics(cell.Grid, slideIdx, label, cellPath+"/grid")...)
			}
		}
	}
	return out
}
