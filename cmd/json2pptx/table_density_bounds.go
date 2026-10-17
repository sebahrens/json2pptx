package main

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// collectGridTableDensityResolved measures each table in its rendered frame,
// including tables in nested regions. A parent grid's width is not a table's
// width when another region shares the row.
func collectGridTableDensityResolved(grid *ShapeGridInput, result *shapegrid.ResolveResult, base string, depth int) []*patterns.ValidationError {
	if grid == nil || result == nil {
		return nil
	}
	var warnings []*patterns.ValidationError
	for _, rc := range result.Cells {
		cell := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		if cell == nil {
			continue
		}
		path := authoredGridCellPath(base, grid, rc.RowIdx, rc.ColIdx)
		if rc.Kind == shapegrid.CellKindTable && cell.Table != nil {
			warnings = append(warnings, pipeline.DetectTableDensity(cell.Table, path+"/table", rc.Bounds.CX)...)
		}
		if rc.Kind == shapegrid.CellKindSubGrid && cell.Grid != nil && depth < maxGeomNestingDepth {
			bounds := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
			if bounds.CX <= 0 || bounds.CY <= 0 {
				bounds = rc.Bounds
			}
			sub := resolveGridForStructural(cell.Grid, &bounds, nil, 0, 0, result.SubGridPeers)
			warnings = append(warnings, collectGridTableDensityResolved(cell.Grid, sub, path+"/grid", depth+1)...)
		}
	}
	return warnings
}
