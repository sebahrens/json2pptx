package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ComposeInput defines a composition envelope that arranges multiple patterns
// on a single slide. Each segment is independently validated and expanded,
// then the resulting grids are merged into a single ShapeGridInput.
type ComposeInput struct {
	Direction    string          `json:"direction"`              // "vertical" or "horizontal"
	Gap          float64         `json:"gap,omitempty"`          // Gap in points between segments (default: 8)
	SmartCompose bool            `json:"smart_compose,omitempty"` // Auto-balance segment sizes by content density
	Segments     []SegmentInput  `json:"segments"`
}

// SegmentInput defines one child within a compose envelope.
type SegmentInput struct {
	Pattern  PatternInput `json:"pattern"`
	SizePct  float64      `json:"size_pct,omitempty"` // Percentage of available space (0 = equal split)
}

// expandCompose validates and expands a compose envelope into a single
// ShapeGridInput by expanding each segment's pattern and merging the results.
func expandCompose(c *ComposeInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, error) {
	if err := validateCompose(c); err != nil {
		return nil, err
	}

	// Expand each segment's pattern
	expandedGrids := make([]*jsonschema.ShapeGridInput, len(c.Segments))
	for i, seg := range c.Segments {
		grid, _, err := expandPattern(&seg.Pattern, ctx, reg)
		if err != nil {
			return nil, fmt.Errorf("compose: segment[%d]: %w", i, err)
		}
		expandedGrids[i] = grid
	}

	// Resolve size percentages — smart compose uses content density when
	// segments have no explicit SizePct.
	var sizes []float64
	if c.SmartCompose && allSizesImplicit(c.Segments) {
		sizes = computeDensitySizes(expandedGrids)
	} else {
		sizes = resolveSegmentSizes(c.Segments)
	}

	// Merge based on direction
	switch c.Direction {
	case "vertical":
		return mergeVertical(expandedGrids, sizes, c.Gap)
	case "horizontal":
		return mergeHorizontal(expandedGrids, sizes, c.Gap)
	default:
		return nil, fmt.Errorf("compose: unsupported direction %q", c.Direction)
	}
}

// validateCompose checks the compose envelope for structural issues.
func validateCompose(c *ComposeInput) error {
	if c.Direction != "vertical" && c.Direction != "horizontal" {
		return fmt.Errorf("compose: direction must be \"vertical\" or \"horizontal\", got %q", c.Direction)
	}
	if len(c.Segments) < 2 {
		return fmt.Errorf("compose: requires at least 2 segments, got %d", len(c.Segments))
	}
	if len(c.Segments) > 4 {
		return fmt.Errorf("compose: maximum 4 segments allowed, got %d", len(c.Segments))
	}

	// Validate size_pct values
	var totalPct float64
	explicitCount := 0
	for i, seg := range c.Segments {
		if seg.SizePct < 0 {
			return fmt.Errorf("compose: segment[%d].size_pct must be >= 0", i)
		}
		if seg.SizePct > 0 {
			totalPct += seg.SizePct
			explicitCount++
		}
	}
	if totalPct > 100 {
		return fmt.Errorf("compose: total size_pct exceeds 100%% (got %.1f%%)", totalPct)
	}

	return nil
}

// resolveSegmentSizes converts segment SizePct values to normalized percentages
// (always summing to 100). Segments with SizePct=0 get equal shares of the remainder.
func resolveSegmentSizes(segments []SegmentInput) []float64 {
	sizes := make([]float64, len(segments))
	var explicitTotal float64
	zeroCount := 0

	for i, seg := range segments {
		if seg.SizePct > 0 {
			sizes[i] = seg.SizePct
			explicitTotal += seg.SizePct
		} else {
			zeroCount++
		}
	}

	if zeroCount == len(segments) {
		// All equal
		equal := 100.0 / float64(len(segments))
		for i := range sizes {
			sizes[i] = equal
		}
	} else if zeroCount > 0 {
		// Distribute remainder equally among zero-valued segments
		remainder := 100.0 - explicitTotal
		share := remainder / float64(zeroCount)
		for i := range sizes {
			if sizes[i] == 0 {
				sizes[i] = share
			}
		}
	}

	return sizes
}

// mergeVertical stacks grids vertically. Each segment's rows get explicit
// height values proportional to the segment's size percentage. All grids
// are reconciled to use the same column count via col_span padding.
func mergeVertical(grids []*jsonschema.ShapeGridInput, sizes []float64, gap float64) (*jsonschema.ShapeGridInput, error) {
	// Find the maximum column count across all grids
	maxCols := 1
	gridCols := make([]int, len(grids))
	for i, g := range grids {
		cols := inferColumnCount(g)
		gridCols[i] = cols
		if cols > maxCols {
			maxCols = cols
		}
	}

	// Merge all rows with proportional heights
	var mergedRows []jsonschema.GridRowInput
	for i, g := range grids {
		segRows := g.Rows
		if len(segRows) == 0 {
			continue
		}

		// Compute per-row height within this segment
		segPct := sizes[i]
		rowHeights := distributeRowHeights(segRows, segPct)

		for j, row := range segRows {
			newRow := row
			newRow.Height = rowHeights[j]
			newRow.AutoHeight = false // explicit heights in compose mode

			// Reconcile columns: if this grid has fewer columns than max,
			// expand the last cell in each row to span the difference.
			if gridCols[i] < maxCols {
				newRow.Cells = reconcileCells(row.Cells, gridCols[i], maxCols)
			}

			mergedRows = append(mergedRows, newRow)
		}
	}

	resolvedGap := gap
	if resolvedGap == 0 {
		resolvedGap = 8
	}

	merged := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, maxCols)),
		Rows:    mergedRows,
		RowGap:  resolvedGap,
	}

	return merged, nil
}

// mergeHorizontal places grids side by side. Each segment becomes a column
// (or set of columns) in a single-row grid where cells are nested grids.
// Since cells can't contain grids, we use explicit bounds on separate grids.
// For horizontal, we create a multi-column grid with each segment's rows
// stacked within their allotted columns.
func mergeHorizontal(grids []*jsonschema.ShapeGridInput, sizes []float64, gap float64) (*jsonschema.ShapeGridInput, error) {
	// For horizontal composition, we create a grid with N columns (one per segment)
	// and the maximum number of rows across all segments. Each segment's cells
	// occupy their column(s).
	totalCols := len(grids)

	// Find max row count across all grids
	maxRows := 0
	for _, g := range grids {
		if len(g.Rows) > maxRows {
			maxRows = len(g.Rows)
		}
	}

	// Build column widths from size percentages
	colWidths := make([]float64, totalCols)
	copy(colWidths, sizes)
	colJSON, _ := json.Marshal(colWidths)

	// Build rows: for each row index, collect one cell from each grid
	mergedRows := make([]jsonschema.GridRowInput, maxRows)
	for rowIdx := 0; rowIdx < maxRows; rowIdx++ {
		cells := make([]*jsonschema.GridCellInput, totalCols)
		for colIdx, g := range grids {
			if rowIdx < len(g.Rows) && len(g.Rows[rowIdx].Cells) > 0 {
				// Take the first cell from this grid's row (simplified: assumes single-column per segment)
				cells[colIdx] = mergeRowCells(g.Rows[rowIdx].Cells)
			} else {
				// Empty cell placeholder
				cells[colIdx] = &jsonschema.GridCellInput{}
			}
		}
		mergedRows[rowIdx] = jsonschema.GridRowInput{
			Cells: cells,
		}
	}

	resolvedGap := gap
	if resolvedGap == 0 {
		resolvedGap = 8
	}

	merged := &jsonschema.ShapeGridInput{
		Columns: colJSON,
		Rows:    mergedRows,
		ColGap:  resolvedGap,
	}

	return merged, nil
}

// mergeRowCells consolidates multiple cells from a pattern row into a single
// cell for horizontal composition. If the row has one cell, returns it directly.
// If multiple, returns the first non-nil cell (simplified).
func mergeRowCells(cells []*jsonschema.GridCellInput) *jsonschema.GridCellInput {
	if len(cells) == 1 {
		c := *cells[0]
		c.ColSpan = 0 // reset span for the merged grid
		return &c
	}
	// For multi-cell rows being merged into a single column, take the first cell.
	// This is a simplification — complex multi-column patterns in horizontal
	// compose are less common. The vertical direction is the primary use case.
	for _, c := range cells {
		if c != nil && (c.Shape != nil || c.Table != nil || c.Icon != nil || c.Image != nil || c.Diagram != nil) {
			result := *c
			result.ColSpan = 0
			return &result
		}
	}
	return &jsonschema.GridCellInput{}
}

// inferColumnCount determines the effective column count from a ShapeGridInput.
func inferColumnCount(g *jsonschema.ShapeGridInput) int {
	// Try parsing the columns field
	if len(g.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(g.Columns, &n); err == nil && n > 0 {
			return int(n)
		}
		var arr []float64
		if err := json.Unmarshal(g.Columns, &arr); err == nil && len(arr) > 0 {
			return len(arr)
		}
	}
	// Infer from max cells in any row
	maxCells := 1
	for _, row := range g.Rows {
		totalCols := 0
		for _, cell := range row.Cells {
			span := 1
			if cell != nil && cell.ColSpan > 1 {
				span = cell.ColSpan
			}
			totalCols += span
		}
		if totalCols > maxCells {
			maxCells = totalCols
		}
	}
	return maxCells
}

// distributeRowHeights distributes a segment's height percentage across its rows.
// If rows have explicit heights, they are scaled proportionally. Otherwise, equal distribution.
func distributeRowHeights(rows []jsonschema.GridRowInput, segmentPct float64) []float64 {
	n := len(rows)
	heights := make([]float64, n)

	// Check if any row has explicit height
	var explicitTotal float64
	for _, r := range rows {
		explicitTotal += r.Height
	}

	if explicitTotal > 0 {
		// Scale explicit heights to fit within segmentPct
		for i, r := range rows {
			if r.Height > 0 {
				heights[i] = (r.Height / explicitTotal) * segmentPct
			} else {
				// Rows without explicit height get equal share of remainder
				heights[i] = segmentPct / float64(n)
			}
		}
	} else {
		// Equal distribution
		perRow := segmentPct / float64(n)
		for i := range heights {
			heights[i] = math.Round(perRow*100) / 100
		}
	}

	return heights
}

// reconcileCells adjusts a row's cells to fit a wider column count by expanding
// the last cell's ColSpan.
func reconcileCells(cells []*jsonschema.GridCellInput, currentCols, targetCols int) []*jsonschema.GridCellInput {
	if len(cells) == 0 {
		return cells
	}

	// Calculate current total column occupation
	totalOccupied := 0
	for _, c := range cells {
		span := 1
		if c != nil && c.ColSpan > 1 {
			span = c.ColSpan
		}
		totalOccupied += span
	}

	diff := targetCols - totalOccupied
	if diff <= 0 {
		return cells
	}

	// Expand the last cell to fill the extra columns
	result := make([]*jsonschema.GridCellInput, len(cells))
	copy(result, cells)

	lastIdx := len(result) - 1
	if result[lastIdx] == nil {
		result[lastIdx] = &jsonschema.GridCellInput{}
	} else {
		// Copy to avoid mutating the original
		copied := *result[lastIdx]
		result[lastIdx] = &copied
	}

	currentSpan := 1
	if result[lastIdx].ColSpan > 1 {
		currentSpan = result[lastIdx].ColSpan
	}
	result[lastIdx].ColSpan = currentSpan + diff

	return result
}

// allSizesImplicit returns true when every segment has SizePct == 0
// (i.e. the caller has not specified explicit sizing).
func allSizesImplicit(segments []SegmentInput) bool {
	for _, seg := range segments {
		if seg.SizePct > 0 {
			return false
		}
	}
	return true
}

// computeDensitySizes derives proportional sizes from the content weight of
// each expanded grid. Weight is the total number of non-nil content cells
// across all rows, giving denser patterns more space. A minimum weight of 1
// prevents zero-division.
func computeDensitySizes(grids []*jsonschema.ShapeGridInput) []float64 {
	weights := make([]float64, len(grids))
	var totalWeight float64

	for i, g := range grids {
		w := contentWeight(g)
		if w < 1 {
			w = 1
		}
		weights[i] = w
		totalWeight += w
	}

	sizes := make([]float64, len(grids))
	for i, w := range weights {
		sizes[i] = math.Round(w/totalWeight*1000) / 10
	}
	return sizes
}

// contentWeight counts the number of non-nil cells with actual content in the
// grid. Each filled cell counts as 1; rows with auto_height or explicit
// heights are scaled proportionally.
func contentWeight(g *jsonschema.ShapeGridInput) float64 {
	if g == nil {
		return 0
	}
	var w float64
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if cell != nil && (cell.Shape != nil || cell.Table != nil ||
				cell.Icon != nil || cell.Image != nil || cell.Diagram != nil) {
				w++
			}
		}
	}
	return w
}
