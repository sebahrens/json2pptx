package main

import (
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// slideExpandContext builds the context a slide's patterns are expanded in:
// its slide-level pattern, its compose envelope and the patterns nested in its
// grid cells. Generation and every read-only pass that predicts generation
// (validate, the fit report, the contrast preflight, preview) build it here,
// so both expand a pattern from the same template metadata (semantic accents,
// surface tints, template grid), theme, geometry and accent position. Built
// separately, the read-only passes carried no metadata and stood the theme in
// for it, and generation carried no title font (go-slide-creator-sw78d).
//
// metadata nil means "the metadata the theme was parsed with"
// (types.ThemeInfo.Metadata). zone and bounds are the slide's content zone and
// content rectangle; both may be zero when the caller has no layouts.
func slideExpandContext(theme *types.ThemeInfo, metadata *types.TemplateMetadata, zone *shapegrid.ContentZone, bounds patterns.LayoutBounds, slideWidth, slideHeight int64, strategy patterns.AccentStrategy, slideIdx, sectionIdx int) patterns.ExpandContext {
	ctx := patterns.ExpandContext{
		Metadata:       metadata,
		ContentZone:    zone,
		SlideWidth:     slideWidth,
		SlideHeight:    slideHeight,
		LayoutBounds:   bounds,
		AccentStrategy: strategy,
		SlideIndex:     slideIdx,
		SectionIndex:   sectionIdx,
	}
	if theme != nil {
		ctx.Theme = *theme
		if ctx.Metadata == nil {
			ctx.Metadata = theme.Metadata
		}
	}
	return ctx
}

// nestedPatternExpansionCopy returns grid with every cell that authors a
// nested pattern — and the grids and rows on the way down to it — replaced by
// a copy, so expanding those patterns (expandNestedCellPatternsInBounds sets
// the cell's Grid and clears its Pattern) never rewrites the grid it was
// handed. Cells with nothing to expand are shared with the source, and no
// field is dropped the way a JSON round trip drops the unexported and
// `json:"-"` ones. A grid without a nested pattern is returned as is.
//
// Generation used to expand the caller's own grid in place, so a second pass
// over one parsed input (a preview and then a generate, a retry) rendered a
// different deck than the first (go-slide-creator-8jp05).
func nestedPatternExpansionCopy(grid *jsonschema.ShapeGridInput) *jsonschema.ShapeGridInput {
	if !hasNestedCellPattern(grid) {
		return grid
	}
	out := *grid
	out.Rows = make([]jsonschema.GridRowInput, len(grid.Rows))
	for ri, row := range grid.Rows {
		out.Rows[ri] = row
		out.Rows[ri].Cells = make([]*jsonschema.GridCellInput, len(row.Cells))
		for ci, cell := range row.Cells {
			if cell == nil || (len(cell.Pattern) == 0 && !hasNestedCellPattern(cell.Grid)) {
				out.Rows[ri].Cells[ci] = cell
				continue
			}
			copied := *cell
			copied.Grid = nestedPatternExpansionCopy(cell.Grid)
			out.Rows[ri].Cells[ci] = &copied
		}
	}
	return &out
}
