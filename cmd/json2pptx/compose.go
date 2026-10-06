package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// composeMaxSegments is the maximum number of top-level segments allowed in a
// single compose envelope. Agents that need to arrange more patterns on one
// slide should nest a compose inside a segment (see go-slide-creator-f1ic.2).
// The value is surfaced through get_capabilities().features.compose.max_segments
// so agents can discover it without parsing error messages.
const composeMaxSegments = 8

// composeMaxNestingDepth is the maximum compose envelope nesting depth. A
// top-level compose is depth 1; a compose nested inside one of its segments
// is depth 2. Deeper nesting is rejected by validateCompose so a single slide
// cannot recursively explode the merged grid.
const composeMaxNestingDepth = 2

// composeMaxLeafPatterns caps the total number of leaf (non-compose) segments
// summed across the entire envelope tree, so nesting cannot be used to bypass
// the per-envelope max_segments and pack an unbounded number of patterns onto
// one slide.
const composeMaxLeafPatterns = 12

// composeDirections enumerates the directions a compose envelope may use.
// Kept here so capabilities discovery and the validator share one source.
var composeDirections = []string{"vertical", "horizontal"}

// composeFeatureCapabilities describes the compose feature flags surfaced
// through get_capabilities().features.compose.
type composeFeatureCapabilities struct {
	MaxSegments             int      `json:"max_segments"`
	MaxNestingDepth         int      `json:"max_nesting_depth"`
	MaxLeafPatterns         int      `json:"max_leaf_patterns"`
	Directions              []string `json:"directions"`
	SupportsSmartCompose    bool     `json:"supports_smart_compose"`
	SupportsNestedCompose   bool     `json:"supports_nested_compose"`
	SupportsDiagramSegments bool     `json:"supports_diagram_segments"`
}

// composeCapabilities returns the canonical compose capability descriptor.
// It is the single source of truth used by both validateCompose and
// get_capabilities to keep the advertised cap in sync with the enforced cap.
func composeCapabilities() composeFeatureCapabilities {
	dirs := make([]string, len(composeDirections))
	copy(dirs, composeDirections)
	return composeFeatureCapabilities{
		MaxSegments:             composeMaxSegments,
		MaxNestingDepth:         composeMaxNestingDepth,
		MaxLeafPatterns:         composeMaxLeafPatterns,
		Directions:              dirs,
		SupportsSmartCompose:    true,
		SupportsNestedCompose:   true,
		SupportsDiagramSegments: true,
	}
}

// ComposeInput is defined in internal/deckinput; aliased here so package-main
// call sites are unchanged.
type ComposeInput = deckinput.ComposeInput

// bannerLikePatterns enumerates leaf patterns whose first row is intrinsically
// a banner. When ComposeInput.Banner is set and the first segment uses one of
// these patterns, validateCompose rejects the envelope to prevent a duplicate
// banner stacking on top of the pattern's own banner.
//
//   - strategy-house: emits an explicit objective banner as its first row.
//   - pull-quote: agents commonly used this as a makeshift banner before the
//     envelope-level Banner was available; stacking a real banner on top of it
//     produces visual redundancy.
var bannerLikePatterns = map[string]bool{
	"strategy-house": true,
	"pull-quote":     true,
}

// SegmentInput (with exported HasPattern/HasDiagram) is defined in
// internal/deckinput; aliased here so package-main call sites are unchanged.
type SegmentInput = deckinput.SegmentInput

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

	// Probe each segment once to learn its row/column structure. Smart compose
	// also derives its density shares from this content-only probe. Then expand
	// with the segment's allocated rectangle so content-sized decisions use the
	// frame the merged grid will render into (go-slide-creator-burq4).
	probeGrids, _, err := expandComposeSegments(c, ctx, nil, reg)
	if err != nil {
		return nil, nil, err
	}

	// Resolve size percentages — smart compose uses content density when
	// segments have no explicit SizePct.
	var sizes []float64
	if c.SmartCompose && allSizesImplicit(c.Segments) {
		sizes = computeDensitySizes(probeGrids)
	} else {
		sizes = resolveSegmentSizes(c.Segments)
	}
	segmentBounds := composeSegmentBounds(c, ctx, probeGrids, sizes)
	expandedGrids, warnings, err := expandComposeSegments(c, ctx, segmentBounds, reg)
	if err != nil {
		return nil, nil, err
	}

	// Merge based on direction
	var merged *jsonschema.ShapeGridInput
	switch c.Direction {
	case "vertical":
		g, err := mergeVertical(expandedGrids, sizes, composeGapPt(c, ctx))
		if err != nil {
			return nil, warnings, err
		}
		merged = g
	case "horizontal":
		g, mergeWarnings, err := mergeHorizontal(expandedGrids, sizes, composeGapPt(c, ctx))
		if err != nil {
			return nil, warnings, err
		}
		if len(mergeWarnings) > 0 {
			warnings = append(warnings, mergeWarnings...)
		}
		merged = g
	default:
		return nil, nil, fmt.Errorf("compose: unsupported direction %q", c.Direction)
	}

	// Envelope decorations: banner prepended, callout appended. The callout is
	// appended first so its column-count inference reads the original merged
	// row 0 (which may have multiple cells); the banner prepend that follows
	// uses inferColumnCount so it is robust against single-cell rows.
	if c.Callout != nil {
		merged = appendCalloutRow(merged, c.Callout, ctx)
	}
	if c.Banner != nil {
		merged = prependBannerRow(merged, c.Banner, ctx)
	}

	return merged, warnings, nil
}

// expandComposeSegments runs the leaf expansion with optional per-segment
// bounds. Passing nil is the structure probe used before size allocation.
func expandComposeSegments(c *ComposeInput, ctx patterns.ExpandContext, bounds []patterns.LayoutBounds, reg *patterns.Registry) ([]*jsonschema.ShapeGridInput, []string, error) {
	grids := make([]*jsonschema.ShapeGridInput, len(c.Segments))
	var warnings []string
	for i, seg := range c.Segments {
		segCtx := ctx
		if bounds != nil {
			segCtx.LayoutBounds = bounds[i]
		}
		switch {
		case seg.Compose != nil:
			grid, innerWarnings, err := expandCompose(seg.Compose, segCtx, reg)
			if err != nil {
				// A pattern's per-field findings are addressed from the
				// envelope: segments[i].compose.… (go-slide-creator-oddtr).
				return nil, nil, fmt.Errorf("compose: segment[%d]: %w", i, prefixPatternFindingPaths(err, fmt.Sprintf("segments[%d].compose.", i)))
			}
			warnings = append(warnings, innerWarnings...)
			grid.Bounds = nil // The outer merge owns the inner envelope's slot.
			grids[i] = grid
		case seg.HasDiagram():
			grids[i] = diagramSegmentGrid(seg.Diagram)
		default:
			pattern := seg.Pattern
			ignoredBoundsWarning := segmentBoundsIgnoredWarning(i, &pattern)
			if ignoredBoundsWarning != "" {
				pattern.Bounds = nil
				pattern.MaxHeightPct = 0
			}
			grid, patternWarnings, err := expandPattern(&pattern, segCtx, reg)
			if err != nil {
				// The finding of values[0].small is at
				// segments[i].pattern.values[0].small of the envelope, not at
				// compose.values[0].small, which the deck does not have.
				return nil, nil, fmt.Errorf("compose: segment[%d]: %w", i, prefixPatternFindingPaths(err, fmt.Sprintf("segments[%d].pattern.", i)))
			}
			if bounds != nil {
				for _, warning := range patternWarnings {
					if m := patternWarningRE.FindStringSubmatch(warning); len(m) == 3 {
						warnings = append(warnings, fmt.Sprintf("%s: segment[%d] %s", m[1], i, m[2]))
					}
				}
			}
			if ignoredBoundsWarning != "" {
				warnings = append(warnings, ignoredBoundsWarning)
				grid.Bounds = nil
			}
			grids[i] = grid
		}
	}
	return grids, warnings, nil
}

// composeGapPt is the gap between compose segments: the authored gap, else
// the template grid's gutter (default 8pt, go-slide-creator-5ms8c).
func composeGapPt(c *ComposeInput, ctx patterns.ExpandContext) float64 {
	if c.Gap != 0 {
		return c.Gap
	}
	return ctx.GutterPt()
}

// composeSegmentBounds translates size_pct shares into each segment's actual
// axis allocation, accounting for the gaps the merged grid subtracts before
// distributing rows or columns. Smart-compose shares are fixed by the probe's
// content density; the final pass can then size text without circularity.
func composeSegmentBounds(c *ComposeInput, ctx patterns.ExpandContext, grids []*jsonschema.ShapeGridInput, sizes []float64) []patterns.LayoutBounds {
	base := ctx.LayoutBounds
	if base.Width <= 0 || base.Height <= 0 {
		db := shapegrid.DefaultBounds(ctx.SlideWidth, ctx.SlideHeight)
		base = patterns.LayoutBounds{X: db.X, Y: db.Y, Width: db.CX, Height: db.CY}
	}
	gap := int64(math.Round(composeGapPt(c, ctx) * 12700))
	units := make([]int, len(grids))
	totalUnits := 0
	for i, grid := range grids {
		if c.Direction == "horizontal" {
			units[i] = inferColumnCount(grid)
		} else {
			// Vertical segments are one parent row each (a nested sub-grid).
			units[i] = 1
		}
		totalUnits += units[i]
	}
	axisLength, axisStart := base.Height, base.Y
	if c.Direction == "horizontal" {
		axisLength, axisStart = base.Width, base.X
	}
	available := max(int64(0), axisLength-gap*int64(max(0, totalUnits-1)))
	result := make([]patterns.LayoutBounds, len(grids))
	for i, unitsInSegment := range units {
		length := int64(math.Round(float64(available)*sizes[i]/100)) + gap*int64(max(0, unitsInSegment-1))
		b := base
		if c.Direction == "horizontal" {
			b.X, b.Width = axisStart, length
		} else {
			b.Y, b.Height = axisStart, length
		}
		// Segments render as nested grids inside parent cells. Their 4pt
		// inset is part of the real content frame and must also constrain
		// pattern text sizing during expansion.
		if b.Width > 2*subGridInsetEMU && b.Height > 2*subGridInsetEMU {
			b.X += subGridInsetEMU
			b.Y += subGridInsetEMU
			b.Width -= 2 * subGridInsetEMU
			b.Height -= 2 * subGridInsetEMU
		}
		result[i] = b
		axisStart += length + gap
	}
	return result
}

// prependBannerRow inserts a full-width banner row at the top of the merged
// grid. The banner cell spans every column (computed via inferColumnCount so
// existing column-span structure is honored) and is rendered as bold light
// text on the accent fill, matching the Strategy-House banner styling.
// Banner cells are NOT addressable via cell_overrides — the band is a fixed
// envelope decoration, like PatternCallout.
func prependBannerRow(grid *jsonschema.ShapeGridInput, banner *patterns.BannerSpec, ctx patterns.ExpandContext) *jsonschema.ShapeGridInput {
	if grid == nil || banner == nil {
		return grid
	}

	numCols := inferColumnCount(grid)
	if numCols < 1 {
		numCols = 1
	}

	// The template's primary fill carries white text (go-slide-creator-2mia4).
	accent := ctx.DefaultAccent()
	if banner.Accent != "" {
		accent = banner.Accent
	}

	// Default emphasis is bold (banners are headers). Callers can opt into
	// italic or bold-italic via the same vocabulary as PatternCallout.
	emphasis := banner.Emphasis
	if emphasis == "" {
		emphasis = "bold"
	}
	bold := emphasis == "bold" || emphasis == "bold-italic"
	italic := emphasis == "italic" || emphasis == "bold-italic"

	textContent := buildCalloutTextContent(banner.Text, 16.0, bold, italic, "lt1", "ctr")

	bannerCell := &jsonschema.GridCellInput{
		ColSpan: numCols,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     jsonStringRaw(accent),
			Text:     textContent,
		},
	}

	bannerRow := jsonschema.GridRowInput{
		AutoHeight: true,
		Cells:      []*jsonschema.GridCellInput{bannerCell},
	}

	// Prepend to grid.Rows.
	newRows := make([]jsonschema.GridRowInput, 0, len(grid.Rows)+1)
	newRows = append(newRows, bannerRow)
	newRows = append(newRows, grid.Rows...)
	grid.Rows = newRows
	return grid
}

// diagramSegmentGrid builds a single-row, single-cell ShapeGridInput whose
// only cell carries the supplied DiagramSpec. The resulting grid is used by
// expandCompose to insert a standalone diagram into the merged grid via the
// same merge path that pattern-expanded grids take, so compose.direction,
// size_pct, and gap apply uniformly across pattern and diagram segments.
//
// The cell omits col_span / row_span — the merge step honors the segment's
// share of the parent grid via size_pct, and inferColumnCount sees a single
// content cell so horizontal merges allocate the diagram's column share via
// the segment's sizes[i] entry. Bounds are intentionally not set; the
// envelope governs placement.
func diagramSegmentGrid(d *types.DiagramSpec) *jsonschema.ShapeGridInput {
	return &jsonschema.ShapeGridInput{
		Rows: []jsonschema.GridRowInput{
			{
				Cells: []*jsonschema.GridCellInput{
					{Diagram: d},
				},
			},
		},
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

// validateCompose checks the compose envelope for structural issues at the
// top level. It delegates to validateComposeRec, which walks any nested
// envelopes and enforces depth + leaf-count caps in addition to the
// per-envelope structural checks.
func validateCompose(c *ComposeInput) error {
	if err := validateComposeRec(c, 1); err != nil {
		return err
	}
	leaves := countComposeLeafPatterns(c)
	if leaves > composeMaxLeafPatterns {
		return fmt.Errorf(
			"compose: total leaf patterns %d exceeds maximum %d — flatten the envelope or split across multiple slides",
			leaves, composeMaxLeafPatterns,
		)
	}
	return nil
}

// validateComposeRec performs the per-envelope structural checks at a given
// nesting depth (1 = top-level). It is recursive so the same validation logic
// (direction, segment count, size_pct, XOR per segment) applies uniformly to
// inner envelopes; the depth parameter is used solely to reject overly deep
// trees.
func validateComposeRec(c *ComposeInput, depth int) error {
	if c == nil {
		return fmt.Errorf("compose: envelope is nil")
	}
	if depth > composeMaxNestingDepth {
		return fmt.Errorf(
			"compose: nesting depth %d exceeds maximum %d — see get_capabilities().features.compose.max_nesting_depth",
			depth, composeMaxNestingDepth,
		)
	}
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

	// Validate each segment's XOR contract and size_pct value, then recurse
	// into any nested compose envelopes. A segment must set exactly one of
	// pattern / compose / diagram.
	var totalPct float64
	for i, seg := range c.Segments {
		hasPattern := seg.HasPattern()
		hasCompose := seg.Compose != nil
		hasDiagram := seg.HasDiagram()
		setCount := 0
		if hasPattern {
			setCount++
		}
		if hasCompose {
			setCount++
		}
		if hasDiagram {
			setCount++
		}
		switch {
		case setCount > 1:
			return fmt.Errorf(
				"compose: segment[%d] sets more than one of \"pattern\", \"compose\", \"diagram\" — choose exactly one",
				i,
			)
		case setCount == 0:
			return fmt.Errorf(
				"compose: segment[%d] must set exactly one of \"pattern\", \"compose\", or \"diagram\"",
				i,
			)
		}
		if seg.SizePct < 0 {
			return fmt.Errorf("compose: segment[%d].size_pct must be >= 0", i)
		}
		if seg.SizePct > 0 {
			totalPct += seg.SizePct
		}
		if hasCompose {
			if err := validateComposeRec(seg.Compose, depth+1); err != nil {
				return fmt.Errorf("compose: segment[%d]: %w", i, err)
			}
		}
		if hasDiagram && seg.Diagram.Type == "" {
			return fmt.Errorf(
				"compose: segment[%d].diagram.type is required (e.g. \"bar\", \"process_flow\")",
				i,
			)
		}
	}
	if totalPct > 100 {
		return fmt.Errorf("compose: total size_pct exceeds 100%% (got %.1f%%)", totalPct)
	}

	// Envelope-level Banner cannot stack on top of a first segment whose pattern
	// already emits its own banner row (strategy-house) or is conventionally
	// used as a faux banner (pull-quote). This protects against duplicate
	// banners landing on the same slide.
	if c.Banner != nil && len(c.Segments) > 0 {
		first := c.Segments[0]
		if first.HasPattern() && bannerLikePatterns[first.Pattern.Name] {
			return fmt.Errorf(
				"compose: banner conflicts with first segment pattern %q — that pattern already provides a banner-like header row; remove compose.banner or replace the first segment with a non-banner pattern",
				first.Pattern.Name,
			)
		}
	}

	return nil
}

// countComposeLeafPatterns sums the number of leaf (pattern-bearing or
// diagram-bearing) segments across the entire envelope tree. Nested compose
// segments are descended into; pattern and diagram segments contribute 1
// each. Used to enforce composeMaxLeafPatterns globally so nesting cannot
// smuggle more leaves past the per-envelope max_segments cap. Diagram
// segments count because they consume slide real-estate the same way a
// pattern segment does.
func countComposeLeafPatterns(c *ComposeInput) int {
	if c == nil {
		return 0
	}
	n := 0
	for _, seg := range c.Segments {
		switch {
		case seg.Compose != nil:
			n += countComposeLeafPatterns(seg.Compose)
		case seg.HasPattern():
			n++
		case seg.HasDiagram():
			n++
		}
	}
	return n
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

// mergeVertical stacks grids vertically. Each segment is a full-width
// sub-grid in its own parent row whose height is the segment's size share, so
// it keeps its own column widths, column gap, row gap and row min/max heights
// (go-slide-creator-csclk.103) — flattening the segments into one shared grid
// forced equal columns on every segment and distorted uneven-track patterns
// (label gutters, connector columns, photo/text pairs).
func mergeVertical(grids []*jsonschema.ShapeGridInput, sizes []float64, gap float64) (*jsonschema.ShapeGridInput, error) {
	var mergedRows []jsonschema.GridRowInput
	for i, g := range grids {
		if g == nil || len(g.Rows) == 0 {
			continue
		}
		child := *g
		child.Bounds = nil // The parent cell owns this segment's rectangle.
		mergedRows = append(mergedRows, jsonschema.GridRowInput{
			Height: sizes[i],
			Cells:  []*jsonschema.GridCellInput{{Grid: &child}},
		})
	}

	resolvedGap := gap
	if resolvedGap == 0 {
		resolvedGap = 8
	}

	merged := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows:    mergedRows,
		RowGap:  resolvedGap,
	}

	return merged, nil
}

// mergeHorizontal places each segment in a full-height sub-grid spanning its
// allocated columns. This keeps independent row heights, gaps, and vertical
// alignment: segments with different row counts cannot share parent rows.
// The parent retains the sum of segment columns for compose col attribution.
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

	rowCells := make([]*jsonschema.GridCellInput, len(grids))
	for segIdx, g := range grids {
		child := *g
		child.Rows = make([]jsonschema.GridRowInput, len(g.Rows))
		for rowIdx, row := range g.Rows {
			child.Rows[rowIdx] = row
			cells := segmentRowCells(g, rowIdx, segCols[segIdx], segIdx, &warnings)
			// The parent cell already reserves the segment's full width. Do
			// not retain filler cells: a child row may have fewer authored
			// cells because an earlier row spans into its columns.
			child.Rows[rowIdx].Cells = cells[:min(len(cells), len(row.Cells))]
		}
		child.Bounds = nil // The parent cell owns this segment's rectangle.
		rowCells[segIdx] = &jsonschema.GridCellInput{ColSpan: segCols[segIdx], Grid: &child}
	}

	resolvedGap := gap
	if resolvedGap == 0 {
		resolvedGap = 8
	}

	merged := &jsonschema.ShapeGridInput{
		Columns: colJSON,
		Rows:    []jsonschema.GridRowInput{{Cells: rowCells}},
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

	row := g.Rows[rowIdx]
	src := row.Cells
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
		// A cell that spans rows is as tall as all of them: capping it at
		// its first row's height shrank a ring / hub cell to one ladder row.
		if c != nil && row.MaxHeight > 0 && c.RowSpan <= 1 {
			copyCell := *c
			if copyCell.MaxHeight == 0 || row.MaxHeight < copyCell.MaxHeight {
				copyCell.MaxHeight = row.MaxHeight
			}
			c = &copyCell
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
		if err := json.Unmarshal(g.Columns, &n); err == nil {
			// An out-of-range count (1e308, 5e7) is refused by validation;
			// never size a slice from it (go-slide-creator-4kq5m).
			if count, err := shapegrid.ColumnCount(n); err == nil {
				return count
			}
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
				cell.Icon != nil || cell.Image != nil || cell.Diagram != nil || len(cell.Layers) > 0) {
				w++
			}
		}
	}
	return w
}
