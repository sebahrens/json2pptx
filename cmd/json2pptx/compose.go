package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// composeMaxSegments is the maximum number of top-level segments allowed in a
// single compose envelope. Agents that need to arrange more patterns on one
// slide should nest a compose inside a segment (see go-slide-creator-f1ic.2).
// The value is surfaced through get_capabilities().features.compose.max_segments
// so agents can discover it without parsing error messages.
const composeMaxSegments = 8

// composeDirections enumerates the directions a compose envelope may use.
// Kept here so capabilities discovery and the validator share one source.
var composeDirections = []string{"vertical", "horizontal"}

// composeFeatureCapabilities describes the compose feature flags surfaced
// through get_capabilities().features.compose.
type composeFeatureCapabilities struct {
	MaxSegments         int      `json:"max_segments"`
	Directions          []string `json:"directions"`
	SupportsSmartCompose bool    `json:"supports_smart_compose"`
}

// composeCapabilities returns the canonical compose capability descriptor.
// It is the single source of truth used by both validateCompose and
// get_capabilities to keep the advertised cap in sync with the enforced cap.
func composeCapabilities() composeFeatureCapabilities {
	dirs := make([]string, len(composeDirections))
	copy(dirs, composeDirections)
	return composeFeatureCapabilities{
		MaxSegments:          composeMaxSegments,
		Directions:           dirs,
		SupportsSmartCompose: true,
	}
}

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
// The returned warnings slice carries non-fatal diagnostics (e.g.,
// COMPOSE_HORIZONTAL_TRUNCATION when a segment's row over-occupies its
// allocated column range and content must be dropped, or
// COMPOSE_SEGMENT_BOUNDS_IGNORED when a segment's PatternInput.Bounds /
// MaxHeightPct cannot be honored because the merged grid governs segment
// placement via direction + size_pct).
func expandCompose(c *ComposeInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, []string, error) {
	if err := validateCompose(c); err != nil {
		return nil, nil, err
	}

	var warnings []string

	// Expand each segment's pattern
	expandedGrids := make([]*jsonschema.ShapeGridInput, len(c.Segments))
	for i, seg := range c.Segments {
		grid, _, err := expandPattern(&seg.Pattern, ctx, reg)
		if err != nil {
			return nil, nil, fmt.Errorf("compose: segment[%d]: %w", i, err)
		}

		// Segment-level bounds (PatternInput.Bounds / MaxHeightPct) cannot be
		// honored inside a compose envelope: the merged grid's region is
		// governed by compose direction + size_pct, and mergeVertical /
		// mergeHorizontal build a fresh grid that drops grid.Bounds. Surface
		// this as a structured warning so agents are not silently misled
		// (go-slide-creator-f1ic.7).
		if w := segmentBoundsIgnoredWarning(i, &seg.Pattern); w != "" {
			warnings = append(warnings, w)
			// Clear the inherited Bounds so downstream consumers don't see a
			// value that the merge step will discard anyway.
			grid.Bounds = nil
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
		grid, err := mergeVertical(expandedGrids, sizes, c.Gap)
		return grid, warnings, err
	case "horizontal":
		grid, mergeWarnings, err := mergeHorizontal(expandedGrids, sizes, c.Gap)
		if len(mergeWarnings) > 0 {
			warnings = append(warnings, mergeWarnings...)
		}
		return grid, warnings, err
	default:
		return nil, nil, fmt.Errorf("compose: unsupported direction %q", c.Direction)
	}
}

// segmentBoundsIgnoredWarning returns a COMPOSE_SEGMENT_BOUNDS_IGNORED warning
// string when a segment's PatternInput carries explicit Bounds or
// MaxHeightPct, which cannot be applied inside a compose envelope. Returns
// the empty string when no bounds override was specified.
func segmentBoundsIgnoredWarning(segIdx int, p *PatternInput) string {
	if p == nil {
		return ""
	}
	switch {
	case p.Bounds != nil:
		return fmt.Sprintf(
			"COMPOSE_SEGMENT_BOUNDS_IGNORED: segment[%d] pattern %q sets bounds, but segment placement inside a compose envelope is governed by compose.direction + size_pct; the bounds are dropped during merge",
			segIdx, p.Name,
		)
	case p.MaxHeightPct > 0 && p.MaxHeightPct < 100:
		return fmt.Sprintf(
			"COMPOSE_SEGMENT_BOUNDS_IGNORED: segment[%d] pattern %q sets max_height_pct=%.1f, but segment placement inside a compose envelope is governed by compose.direction + size_pct; the height cap is dropped during merge",
			segIdx, p.Name, p.MaxHeightPct,
		)
	}
	return ""
}

// validateCompose checks the compose envelope for structural issues.
func validateCompose(c *ComposeInput) error {
	if c.Direction != "vertical" && c.Direction != "horizontal" {
		return fmt.Errorf("compose: direction must be \"vertical\" or \"horizontal\", got %q", c.Direction)
	}
	if len(c.Segments) < 2 {
		return fmt.Errorf("compose: requires at least 2 segments, got %d", len(c.Segments))
	}
	maxSeg := composeCapabilities().MaxSegments
	if len(c.Segments) > maxSeg {
		return fmt.Errorf(
			"compose: maximum %d segments allowed, got %d — for larger arrangements nest a compose envelope inside a segment (see get_capabilities().features.compose.max_segments)",
			maxSeg, len(c.Segments),
		)
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

// mergeHorizontal places grids side by side by concatenating each segment's
// columns into a single wide grid. Per-segment column counts are summed to
// form the merged grid's column count, and each segment's sizes[i] share is
// distributed across its own columns (preserving the segment's original
// column-width proportions when an explicit array is provided).
//
// For each output row, the segment's row cells (padded with empty cells when
// the row is under-occupied or absent) are appended in segment order. This
// preserves every input cell — patterns like kpi-3up keep all three cards
// instead of being silently collapsed to one.
//
// If a segment's row over-occupies its allocated column range (sum of cell
// ColSpans exceeds the segment's column count), the excess cells are dropped
// and a COMPOSE_HORIZONTAL_TRUNCATION warning is appended to the returned
// warnings slice so callers can surface it instead of dropping content
// silently.
func mergeHorizontal(grids []*jsonschema.ShapeGridInput, sizes []float64, gap float64) (*jsonschema.ShapeGridInput, []string, error) {
	var warnings []string

	// Determine each segment's column count and distribute its size share
	// across those columns.
	segCols := make([]int, len(grids))
	totalCols := 0
	colWidths := make([]float64, 0, len(grids))
	for i, g := range grids {
		cols := inferColumnCount(g)
		segCols[i] = cols
		totalCols += cols
		colWidths = append(colWidths, allocateSegmentColumns(g, cols, sizes[i])...)
	}
	colJSON, _ := json.Marshal(colWidths)

	// Find max row count across all grids
	maxRows := 0
	for _, g := range grids {
		if len(g.Rows) > maxRows {
			maxRows = len(g.Rows)
		}
	}

	// Build rows: for each row index, concatenate each segment's row cells
	// (padded with empty cells to fill the segment's column allocation).
	mergedRows := make([]jsonschema.GridRowInput, maxRows)
	for rowIdx := 0; rowIdx < maxRows; rowIdx++ {
		rowCells := make([]*jsonschema.GridCellInput, 0, totalCols)
		for segIdx, g := range grids {
			n := segCols[segIdx]
			segPart := segmentRowCells(g, rowIdx, n, segIdx, &warnings)
			rowCells = append(rowCells, segPart...)
		}
		mergedRows[rowIdx] = jsonschema.GridRowInput{Cells: rowCells}
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

	return merged, warnings, nil
}

// segmentRowCells extracts the cells from grid g at rowIdx and returns
// exactly n cells, padding under-occupied rows with empty cells and
// truncating (with a warning appended) when the row's ColSpan-weighted
// occupancy exceeds n.
func segmentRowCells(g *jsonschema.ShapeGridInput, rowIdx, n, segIdx int, warnings *[]string) []*jsonschema.GridCellInput {
	if rowIdx >= len(g.Rows) {
		// Segment has no row at this index — pad with empty cells.
		out := make([]*jsonschema.GridCellInput, n)
		for i := range out {
			out[i] = &jsonschema.GridCellInput{}
		}
		return out
	}

	src := g.Rows[rowIdx].Cells
	out := make([]*jsonschema.GridCellInput, 0, n)
	used := 0
	for _, c := range src {
		span := 1
		if c != nil && c.ColSpan > 1 {
			span = c.ColSpan
		}
		if used+span > n {
			// Over-occupancy: dropping this cell would shed content silently.
			if warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf(
					"COMPOSE_HORIZONTAL_TRUNCATION: segment[%d] row[%d] cells span more columns (%d so far + %d) than the segment's allocated width (%d); excess cells dropped",
					segIdx, rowIdx, used, span, n))
			}
			break
		}
		out = append(out, c)
		used += span
	}
	// Pad remaining space with empty cells so the segment occupies exactly n columns.
	for used < n {
		out = append(out, &jsonschema.GridCellInput{})
		used++
	}
	return out
}

// allocateSegmentColumns distributes a segment's percentage share across its
// columns. When the segment carries an explicit per-column widths array of
// matching length, those proportions are preserved within the segment's share;
// otherwise the share is split equally.
func allocateSegmentColumns(g *jsonschema.ShapeGridInput, cols int, share float64) []float64 {
	widths := make([]float64, cols)
	if g != nil && len(g.Columns) > 0 {
		var arr []float64
		if err := json.Unmarshal(g.Columns, &arr); err == nil && len(arr) == cols {
			var sum float64
			for _, v := range arr {
				if v > 0 {
					sum += v
				}
			}
			if sum > 0 {
				for i, v := range arr {
					if v > 0 {
						widths[i] = (v / sum) * share
					} else {
						widths[i] = 0
					}
				}
				return widths
			}
		}
	}
	per := share / float64(cols)
	for i := range widths {
		widths[i] = per
	}
	return widths
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
