package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// buildExpandedCompose re-runs per-segment expansion to collect the
// per-segment metadata exposed in resolvedSlide.ExpandedCompose. It mirrors
// the size/merge logic in expandCompose so that the bounds_pct, row_range,
// and col_range reflect what mergeVertical / mergeHorizontal actually
// produced. Returns nil when the compose envelope is malformed in a way that
// would already have surfaced as an error to the caller.
func buildExpandedCompose(c *ComposeInput, ctx patterns.ExpandContext, reg *patterns.Registry, merged *jsonschema.ShapeGridInput) *resolvedCompose {
	if c == nil || merged == nil {
		return nil
	}

	expandedGrids := make([]*jsonschema.ShapeGridInput, len(c.Segments))
	for i, seg := range c.Segments {
		switch {
		case seg.Compose != nil:
			// Inner-envelope metadata is summarized through the outer
			// segment's row/col range; we recurse to produce its merged
			// grid so the parent merge sees the correct row count.
			inner, _, err := expandCompose(seg.Compose, ctx, reg)
			if err != nil {
				return nil
			}
			expandedGrids[i] = inner
		case seg.hasDiagram():
			expandedGrids[i] = diagramSegmentGrid(seg.Diagram)
		default:
			grid, _, err := expandPattern(&seg.Pattern, ctx, reg)
			if err != nil {
				return nil
			}
			expandedGrids[i] = grid
		}
	}

	// Mirror expandCompose's smart-vs-explicit size resolution so bounds_pct
	// matches the merged grid that the caller actually got back.
	var sizes []float64
	if c.SmartCompose && allSizesImplicit(c.Segments) {
		sizes = computeDensitySizes(expandedGrids)
	} else {
		sizes = resolveSegmentSizes(c.Segments)
	}

	var segments []resolvedComposeSegment
	switch c.Direction {
	case "vertical":
		segments = buildVerticalSegments(c, expandedGrids, sizes, merged)
	case "horizontal":
		segments = buildHorizontalSegments(c, expandedGrids, sizes, merged)
	default:
		return nil
	}

	return &resolvedCompose{
		Direction: c.Direction,
		Segments:  segments,
	}
}

// buildVerticalSegments computes per-segment metadata for vertical compose.
// Each segment owns a contiguous row range of the merged grid spanning all
// columns; bounds_pct stack from top to bottom by cumulative size_pct.
// The bannerOffset and calloutOffset arguments account for envelope-level
// banner/callout rows prepended/appended to the merged grid so segment row
// ranges remain accurate when those decorations are present.
func buildVerticalSegments(c *ComposeInput, expandedGrids []*jsonschema.ShapeGridInput, sizes []float64, merged *jsonschema.ShapeGridInput) []resolvedComposeSegment {
	totalCols := inferColumnCount(merged)
	if totalCols < 1 {
		totalCols = 1
	}

	bannerOffset, _ := composeAuxRowCounts(c)

	segments := make([]resolvedComposeSegment, 0, len(c.Segments))
	rowCursor := bannerOffset
	yPct := 0.0
	for i := range c.Segments {
		segGrid := expandedGrids[i]
		segRows := 0
		if segGrid != nil {
			segRows = len(segGrid.Rows)
		}
		cellCount := countContentCells(segGrid)
		seg := resolvedComposeSegment{
			Index:               i,
			Pattern:             c.Segments[i].Pattern.Name,
			CellsAfterExpansion: cellCount,
			BoundsPct: resolvedComposeSegmentRect{
				XPct:      0,
				YPct:      yPct,
				WidthPct:  100,
				HeightPct: sizes[i],
			},
			RowRange: [2]int{rowCursor, rowCursor + segRows},
			ColRange: [2]int{0, totalCols},
		}
		segments = append(segments, seg)
		rowCursor += segRows
		yPct += sizes[i]
	}
	return segments
}

// composeAuxRowCounts reports the number of banner rows prepended and callout
// rows appended to a compose envelope's merged grid. Used by preview metadata
// to offset segment row ranges so they reference the actual rows the segment
// owns inside the rendered grid.
func composeAuxRowCounts(c *ComposeInput) (banner, callout int) {
	if c == nil {
		return 0, 0
	}
	if c.Banner != nil {
		banner = 1
	}
	if c.Callout != nil {
		callout = 1
	}
	return banner, callout
}

// buildHorizontalSegments computes per-segment metadata for horizontal
// compose. Each segment owns a contiguous col range of the merged grid
// spanning all rows; bounds_pct stack left to right by cumulative size_pct.
func buildHorizontalSegments(c *ComposeInput, expandedGrids []*jsonschema.ShapeGridInput, sizes []float64, merged *jsonschema.ShapeGridInput) []resolvedComposeSegment {
	bannerOffset, calloutOffset := composeAuxRowCounts(c)
	totalRows := len(merged.Rows)
	// Segments occupy the rows between banner and callout decorations. When a
	// banner is prepended at row 0 or a callout appended at the last row, the
	// segments span [bannerOffset, totalRows-calloutOffset).
	segmentRowEnd := totalRows - calloutOffset
	if segmentRowEnd < bannerOffset {
		segmentRowEnd = bannerOffset
	}

	segments := make([]resolvedComposeSegment, 0, len(c.Segments))
	colCursor := 0
	xPct := 0.0
	for i := range c.Segments {
		segGrid := expandedGrids[i]
		segCols := 1
		if segGrid != nil {
			segCols = inferColumnCount(segGrid)
		}
		cellCount := countContentCells(segGrid)
		seg := resolvedComposeSegment{
			Index:               i,
			Pattern:             c.Segments[i].Pattern.Name,
			CellsAfterExpansion: cellCount,
			BoundsPct: resolvedComposeSegmentRect{
				XPct:      xPct,
				YPct:      0,
				WidthPct:  sizes[i],
				HeightPct: 100,
			},
			RowRange: [2]int{bannerOffset, segmentRowEnd},
			ColRange: [2]int{colCursor, colCursor + segCols},
		}
		segments = append(segments, seg)
		colCursor += segCols
		xPct += sizes[i]
	}
	return segments
}

// countContentCells counts the cells in an expanded segment grid that carry
// any populated content. Empty padding cells are skipped so the agent sees
// the segment's intrinsic cell count, not its column-padded extent.
func countContentCells(g *jsonschema.ShapeGridInput) int {
	if g == nil {
		return 0
	}
	n := 0
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			if cell.Shape != nil || cell.Table != nil || cell.Icon != nil ||
				cell.Image != nil || cell.Diagram != nil {
				n++
			}
		}
	}
	return n
}

// composeWarningRE matches the leading "CODE: segment[N]" portion of the
// structured compose warning strings emitted by expandCompose. Capture 1 is
// the warning code (e.g. COMPOSE_HORIZONTAL_TRUNCATION) and capture 2 is the
// 0-based segment index.
var composeWarningRE = regexp.MustCompile(`^([A-Z_]+):\s*segment\[(\d+)\]`)

// composeWarningAsFinding converts an expandCompose warning string into a
// structured FitFinding tagged with the originating segment index. Returns
// nil when the string does not match the expected prefix shape (e.g. a
// future warning format that does not name a segment).
func composeWarningAsFinding(slideIdx int, warning string) *patterns.FitFinding {
	m := composeWarningRE.FindStringSubmatch(warning)
	if len(m) != 3 {
		return nil
	}
	code := m[1]
	segIdx, err := strconv.Atoi(m[2])
	if err != nil {
		return nil
	}
	action := composeWarningAction(code)
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    slidepath.SlideField(slideIdx, "compose"),
			Code:    code,
			Message: fmt.Sprintf("slide %d: %s", slideIdx+1, warning),
		},
		Action:       action,
		SegmentIndex: intPtr(segIdx),
	}
}

// composeErrorAsFinding extracts a segment-attributable finding from the
// "compose: segment[N]: ..." error wrapper that expandCompose emits when a
// child segment fails to expand. Returns nil when the error does not name a
// segment.
func composeErrorAsFinding(slideIdx int, err error) *patterns.FitFinding {
	if err == nil {
		return nil
	}
	msg := err.Error()
	const prefix = "compose: segment["
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return nil
	}
	rest := msg[idx+len(prefix):]
	closeIdx := strings.IndexByte(rest, ']')
	if closeIdx <= 0 {
		return nil
	}
	segIdx, parseErr := strconv.Atoi(rest[:closeIdx])
	if parseErr != nil {
		return nil
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    slidepath.SlideField(slideIdx, "compose"),
			Code:    "COMPOSE_SEGMENT_EXPAND_FAILED",
			Message: fmt.Sprintf("slide %d: %s", slideIdx+1, msg),
		},
		Action:       "refuse",
		SegmentIndex: intPtr(segIdx),
	}
}

// composeWarningAction maps a structured compose warning code to its
// recommended fit-finding action. Unknown codes default to "review".
func composeWarningAction(code string) string {
	switch code {
	case "COMPOSE_HORIZONTAL_TRUNCATION":
		// Content was silently dropped — the agent needs to act.
		return "shrink_or_split"
	case "COMPOSE_SEGMENT_BOUNDS_IGNORED":
		// The override was honored at validation time but dropped during
		// merge; informational so the agent can restructure if intended.
		return "review"
	}
	return "review"
}

// attachComposeSegmentIndex post-processes the fit-finding list and, for any
// finding whose path points at a compose-merged grid cell, fills in
// SegmentIndex by looking up the cell's row/col against the resolvedSlide's
// ExpandedCompose segment ranges. Findings that already carry a segment
// index (e.g. emitted by composeWarningAsFinding) are left alone.
func attachComposeSegmentIndex(findings []patterns.FitFinding, resolved []resolvedSlide) {
	if len(resolved) == 0 || len(findings) == 0 {
		return
	}
	byIdx := make(map[int]*resolvedCompose, len(resolved))
	for i := range resolved {
		if resolved[i].ExpandedCompose != nil {
			byIdx[resolved[i].SlideIndex] = resolved[i].ExpandedCompose
		}
	}
	if len(byIdx) == 0 {
		return
	}

	for i := range findings {
		f := &findings[i]
		if f.SegmentIndex != nil {
			continue
		}
		slideIdx, rowIdx, colIdx, ok := slidepath.ParseGridCell(f.Path)
		if !ok {
			continue
		}
		ec, present := byIdx[slideIdx]
		if !present {
			continue
		}
		if seg := segmentForCell(ec, rowIdx, colIdx); seg != nil {
			f.SegmentIndex = intPtr(seg.Index)
		}
	}
}

// segmentForCell returns the compose segment containing the given (row, col)
// in the merged grid, or nil when no segment matches (which can happen if
// the path references a row/col outside any segment's allocation).
func segmentForCell(ec *resolvedCompose, rowIdx, colIdx int) *resolvedComposeSegment {
	if ec == nil {
		return nil
	}
	for i := range ec.Segments {
		seg := &ec.Segments[i]
		if rowIdx >= seg.RowRange[0] && rowIdx < seg.RowRange[1] &&
			colIdx >= seg.ColRange[0] && colIdx < seg.ColRange[1] {
			return seg
		}
	}
	return nil
}

// intPtr returns a pointer to i. Used so callers can populate
// optional pointer-typed JSON fields without a temporary local.
func intPtr(i int) *int { return &i }
