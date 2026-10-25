package patterns

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func cardGridHeight(grid *jsonschema.ShapeGridInput) float64 {
	if grid == nil {
		return math.Inf(1)
	}
	height := float64(max(0, len(grid.Rows)-1)) * grid.RowGap
	for _, row := range grid.Rows {
		height += max(row.MinHeight, row.MaxHeight)
	}
	return height
}

// chooseShape tries the small legal arrangement set through the real expander.
// Explicit dimensions are never reconsidered. The dominant cost is overflow
// at the written type size; aspect and last-row balance break fitting ties.
func (c *cardGrid) chooseShape(ctx ExpandContext, vals *CardGridValues, ovr *CardGridOverrides, overrides map[int]any, defaultCols, defaultRows int) (int, int) {
	w, h := sizingAreaPt(ctx)
	if w <= 0 || h <= 0 {
		return defaultCols, defaultRows
	}
	bestCols, bestRows, best := defaultCols, defaultRows, math.Inf(1)
	for columns := 1; columns <= cardGridMaxColumns; columns++ {
		rows := (len(vals.Cells) + columns - 1) / columns
		if rows > cardGridMaxRows || columns > len(vals.Cells) {
			continue
		}
		probe := *vals
		probe.Columns = columns
		grid, err := c.Expand(ctx, &probe, ovr, overrides)
		if err != nil {
			continue
		}
		height := cardGridHeight(grid)
		score := 100*math.Max(0, height/h-1) + 0.05*math.Abs(math.Log((float64(columns)/float64(rows))/(w/h)))
		// A fitting but shallow strip wastes a full slide. Prefer the fuller
		// fitting arrangement; overflow remains the dominant cost in short bands.
		score += 0.25 * math.Max(0, 0.65-height/h)
		score += 0.10 * float64(columns*rows-len(vals.Cells)) / float64(len(vals.Cells))
		if columns != defaultCols {
			score += 0.025
		}
		if score < best {
			best, bestCols, bestRows = score, columns, rows
		}
	}
	return bestCols, bestRows
}
