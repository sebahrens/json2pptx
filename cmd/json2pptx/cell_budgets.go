package main

import (
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

// cellBudgetEntry is a single cell's text capacity and density in the expand response.
type cellBudgetEntry struct {
	CellIndex  int     `json:"cell_index"`
	Row        int     `json:"row"`
	Col        int     `json:"col"`
	MaxChars   int     `json:"max_chars"`
	ActualChars int    `json:"actual_chars"`
	DensityPct int     `json:"density_pct"`
	Status     string  `json:"status"`
	FontSizePt float64 `json:"font_size_pt"`
}

// cellDensityWarning flags a cell that is underfilled or overflowing.
type cellDensityWarning struct {
	CellIndex    int                         `json:"cell_index"`
	Field        string                      `json:"field"`
	Actual       int                         `json:"actual"`
	Budget       int                         `json:"budget"`
	Status       string                      `json:"status"`
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// computeCellBudgets resolves an expanded ShapeGridInput into cell budgets and
// density warnings using the textcapacity package. The layout bounds from the
// ExpandContext determine the content area.
//
// Returns nil slices (not errors) when the grid cannot be resolved — this keeps
// the expand response valid even when budget computation fails.
func computeCellBudgets(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext) ([]cellBudgetEntry, []cellDensityWarning) {
	if grid == nil || len(grid.Rows) == 0 {
		return nil, nil
	}

	// Convert DTO columns to []float64
	colWidths, err := resolveColumnsDTO(grid.Columns, grid.Rows)
	if err != nil {
		return nil, nil
	}

	// Resolve gaps
	colGap := grid.ColGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}

	// Convert DTO rows to shapegrid.Row
	rows := convertGridRows(grid.Rows)

	// Build bounds from ExpandContext layout bounds
	bounds := pptx.RectEmu{
		X:  ctx.LayoutBounds.X,
		Y:  ctx.LayoutBounds.Y,
		CX: ctx.LayoutBounds.Width,
		CY: ctx.LayoutBounds.Height,
	}

	// If grid has explicit bounds, use percentage-based resolution instead
	if grid.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(
			grid.Bounds.X, grid.Bounds.Y,
			grid.Bounds.Width, grid.Bounds.Height,
			ctx.SlideWidth, ctx.SlideHeight,
		)
	}

	sgGrid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    rows,
		ColGap:  colGap,
		RowGap:  rowGap,
	}

	// Validate before resolving
	if vErr := shapegrid.Validate(sgGrid); vErr != nil {
		return nil, nil
	}

	// Resolve with a dummy allocator (we only need cell bounds, not shape IDs)
	alloc := pptx.NewShapeIDAllocator(nil)
	result, err := shapegrid.Resolve(sgGrid, alloc)
	if err != nil || result == nil {
		return nil, nil
	}

	// Compute densities
	densities := textcapacity.ForResolvedGrid(result)
	if len(densities) == 0 {
		return nil, nil
	}

	// Map resolved cells back to row/col positions
	budgets := make([]cellBudgetEntry, 0, len(densities))
	var warnings []cellDensityWarning

	cellIdx := 0
	for rowIdx, row := range grid.Rows {
		for colIdx := range row.Cells {
			if cellIdx >= len(densities) {
				break
			}
			d := densities[cellIdx]
			budgets = append(budgets, cellBudgetEntry{
				CellIndex:   cellIdx,
				Row:         rowIdx,
				Col:         colIdx,
				MaxChars:    d.MaxChars,
				ActualChars: d.ActualChars,
				DensityPct:  d.DensityPct,
				Status:      string(d.Status),
				FontSizePt:  d.FontPt,
			})

			// Emit warning for non-optimal cells that have content
			if d.Status != textcapacity.StatusOptimal && d.ActualChars > 0 {
				field := "body"
				if cellIdx < len(result.Cells) && result.Cells[cellIdx].ShapeSpec != nil {
					field = inferCellField(result.Cells[cellIdx].ShapeSpec.Text)
				}
				warnings = append(warnings, cellDensityWarning{
					CellIndex: cellIdx,
					Field:     field,
					Actual:    d.ActualChars,
					Budget:    d.MaxChars,
					Status:    string(d.Status),
				})
			}
			cellIdx++
		}
	}

	return budgets, warnings
}

// sparseLayoutWarning checks whether the average cell density across all
// content-bearing cells is below the pattern's sparse threshold. When it is,
// the pattern is likely to produce comically tall blocks. Returns nil when
// density is adequate or there are no content cells to measure.
func sparseLayoutWarning(budgets []cellBudgetEntry, pat patterns.Pattern, patternName string, pi *PatternInput) *cellDensityWarning {
	if len(budgets) == 0 {
		return nil
	}
	// Skip if the caller already constrained bounds
	if pi.Bounds != nil || pi.MaxHeightPct > 0 {
		return nil
	}

	// Compute average density across cells that have content
	var totalDensity, contentCells int
	for _, b := range budgets {
		if b.ActualChars > 0 {
			totalDensity += b.DensityPct
			contentCells++
		}
	}
	if contentCells == 0 {
		return nil
	}
	avgDensity := totalDensity / contentCells

	threshold := pat.Taxonomy().EffectiveSparseThreshold(20)
	if avgDensity >= threshold {
		return nil
	}

	// Suggest a max_height_pct that would bring content to ~70% fill
	suggestedPct := int(float64(avgDensity) / 0.7)
	if suggestedPct < 20 {
		suggestedPct = 20
	}
	if suggestedPct > 90 {
		suggestedPct = 90
	}

	return &cellDensityWarning{
		CellIndex: -1, // grid-level, not cell-specific
		Field:     "layout",
		Actual:    avgDensity,
		Budget:    threshold,
		Status:    "sparse_layout",
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool: "expand_pattern",
			ArgsTemplate: map[string]any{
				"name":           patternName,
				"max_height_pct": suggestedPct,
			},
		},
	}
}

// inferCellField examines shape text JSON to determine whether the cell
// contains a "title", "header", or generic "body" text.
func inferCellField(text json.RawMessage) string {
	if len(text) == 0 {
		return "body"
	}
	// Simple heuristic: if it's a short string or has small font, it's likely a label
	var obj struct {
		Paragraphs []struct {
			Size float64 `json:"size,omitempty"`
		} `json:"paragraphs,omitempty"`
		Size float64 `json:"size,omitempty"`
	}
	if err := json.Unmarshal(text, &obj); err == nil {
		if obj.Size >= 18 || (len(obj.Paragraphs) > 0 && obj.Paragraphs[0].Size >= 18) {
			return "header"
		}
	}
	return "body"
}
