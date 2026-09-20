package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
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
	grid, _ := expandSlidePatternGridWithWarnings(slide, slideIdx, slideWidth, slideHeight, theme)
	return grid
}

// expandSlidePatternGridWithWarnings is expandSlidePatternGrid plus the
// pattern's own PostExpandWarnings. Those warnings — BODY_TOO_LONG on a bio
// over its budget, CHART_PLACEHOLDER_EMPTY on a chart panel with no chart —
// used to be discarded here and reached agents only through
// preview_presentation_plan, so a deck that validate and generate both called
// clean rendered 75% empty (go-slide-creator-wn4v).
func expandSlidePatternGridWithWarnings(slide *SlideInput, slideIdx int, slideWidth, slideHeight int64, theme *types.ThemeInfo) (*ShapeGridInput, []string) {
	if slide == nil || slide.Pattern == nil || slide.ShapeGrid != nil {
		return nil, nil
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
	grid, warnings, err := expandPattern(slide.Pattern, ctx, patterns.Default())
	if err != nil || grid == nil {
		return nil, nil
	}
	if err := expandNestedCellPatterns(grid, ctx, patterns.Default()); err != nil {
		return nil, nil
	}
	return grid, warnings
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

// slidePatternDiagnostics runs the SAME pattern resolution generate runs over
// one slide — compose envelope, slide-level named pattern, and any cell-level
// nested patterns — and reports every failure as a blocking diagnostic.
//
// Before this existed, validate / validate_input expanded patterns only to
// harvest geometry findings and discarded the expansion error, so a deck whose
// pattern values violate the pattern's own value schema (over maxLength, under
// minItems, an unknown bundled icon name) was reported VALID and then refused
// by generate with INPUT.INVALID_SLIDE. Agents following the documented
// validate-then-render loop burned a full round-trip per mistake
// (go-slide-creator-xvek).
//
// The expansion order mirrors convertSinglePresentationSlide exactly: compose
// first (it produces the grid), then a slide-level pattern (which replaces it),
// then nested cell patterns over whichever grid resulted. Stopping at the first
// failing stage matches generate, which aborts there.
func slidePatternDiagnostics(slide *SlideInput, slideIdx int, ctx patterns.ExpandContext, reg *patterns.Registry) []diagnostics.Diagnostic {
	if slide == nil {
		return nil
	}

	// A pattern-input failure carries one finding per field, each with its own
	// JSON path, fix and next_tool_call; only a failure with no per-field
	// structure falls back to the single PATTERN_ERROR finding
	// (go-slide-creator-20jm).
	patternErr := func(field string, err error) []diagnostics.Diagnostic {
		if ds := patternInputDiagnostics(err, slidepath.SlideField(slideIdx, field),
			fmt.Sprintf("slide %d", slideIdx+1)); len(ds) > 0 {
			for i := range ds {
				ds[i].Message += " (generate would refuse this deck)"
			}
			return ds
		}
		return []diagnostics.Diagnostic{{
			Code:     diagnostics.CodePatternError,
			Path:     slidepath.SlideField(slideIdx, field),
			Message:  fmt.Sprintf("slide %d: %v (generate would refuse this deck)", slideIdx+1, err),
			Severity: diagnostics.SeverityError,
		}}
	}

	grid := slide.ShapeGrid
	if slide.Compose != nil {
		expanded, _, err := expandCompose(slide.Compose, ctx, reg)
		if err != nil {
			return patternErr("compose", err)
		}
		grid = expanded
	}
	if slide.Pattern != nil {
		expanded, _, err := expandPattern(slide.Pattern, ctx, reg)
		if err != nil {
			return patternErr("pattern", err)
		}
		grid = expanded
	}
	if grid != nil {
		// expandNestedCellPatterns mutates the grid it walks, so a grid that
		// came straight off the input is cloned first — validate must never
		// rewrite the caller's deck.
		if grid == slide.ShapeGrid {
			cloned, err := cloneShapeGrid(grid)
			if err != nil {
				return patternErr("shape_grid", err)
			}
			grid = cloned
		}
		if err := expandNestedCellPatterns(grid, ctx, reg); err != nil {
			return patternErr("shape_grid", err)
		}
	}
	return nil
}

// cloneShapeGrid deep-copies a shape grid via its JSON representation so
// validation-time expansion cannot mutate the caller's input.
func cloneShapeGrid(grid *ShapeGridInput) (*ShapeGridInput, error) {
	data, err := json.Marshal(grid)
	if err != nil {
		return nil, fmt.Errorf("shape_grid: %w", err)
	}
	var out ShapeGridInput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("shape_grid: %w", err)
	}
	return &out, nil
}
