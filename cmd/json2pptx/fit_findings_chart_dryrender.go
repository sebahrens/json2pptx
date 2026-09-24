package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// collectChartDryRenderFindings iterates every slide's chart_value /
// diagram_value content item AND every diagram surface embedded in a slide's
// shape_grid (cell diagrams, composite sub_diagrams, and recursively nested
// sub-grids), invoking svggen.DryRender and converting the returned chart.*
// findings into patterns.FitFinding entries. This closes the
// validate → preview feedback loop for render-only findings (chart.tick_thinned,
// chart.label_clipped, chart.legend_overflow_dropped, chart.label_truncated,
// chart.scatter_label_skipped, etc.) so agents see them before paying the
// cost of a full generate — including for the embedded-diagram paths used in
// complex shape_grid slides (go-slide-creator-kzzl).
//
// themeColors is forwarded to svggen so dry-run findings reflect the same
// palette the renderer will use. Pass nil when theme isn't available — the
// pipeline still runs (findings affected by geometry, not color, are emitted).
//
// bodyFont is the active template body font. It is forwarded to svggen so
// label-fit findings (chart.label_clipped, chart.label_truncated,
// chart.tick_thinned, etc.) are measured under the same typeface the renderer
// injects via resolveDiagramWithMetadata / generateDiagramCellInserts. A
// diagram's explicit style.font_family still wins. Pass "" when no template
// font is available.
//
// strictFit threads through to svggen's severity promotion ladder so warn /
// strict modes promote chart findings consistently with the generate path.
func collectChartDryRenderFindings(
	input *PresentationInput,
	themeColors []types.ThemeColor,
	bodyFont string,
	strictFit string,
) []patterns.FitFinding {
	return collectChartDryRenderFindingsWithConverter(input, themeColors, bodyFont, strictFit,
		generator.NewSVGConverter().IsPNGAvailable())
}

// The explicit availability argument keeps dependency-state tests independent
// of the host PATH while production still probes the actual converter tools.
func collectChartDryRenderFindingsWithConverter(
	input *PresentationInput,
	themeColors []types.ThemeColor,
	bodyFont string,
	strictFit string,
	converterAvailable bool,
) []patterns.FitFinding {
	return collectChartDryRenderFindingsResolved(input, themeColors, bodyFont, strictFit, converterAvailable, nil, 0, 0)
}

// collectChartDryRenderFindingsInFrames asks svggen about the frame generation
// will actually render into, not svggen's 800x600 default. A template's layout
// and post-fit grid cell can both materially change what labels fit.
func collectChartDryRenderFindingsInFrames(
	input *PresentationInput,
	themeColors []types.ThemeColor,
	bodyFont, strictFit string,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
) []patterns.FitFinding {
	return collectChartDryRenderFindingsResolved(input, themeColors, bodyFont, strictFit,
		generator.NewSVGConverter().IsPNGAvailable(), layouts, slideWidth, slideHeight)
}

func collectChartDryRenderFindingsResolved(
	input *PresentationInput,
	themeColors []types.ThemeColor,
	bodyFont, strictFit string,
	converterAvailable bool,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
) []patterns.FitFinding {
	if input == nil || len(input.Slides) == 0 {
		return nil
	}
	var findings []patterns.FitFinding
	predictedLayouts := predictSlideLayouts(input, layouts)
	rhythmGrid := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)
	for slideIdx, slide := range input.Slides {
		layout := predictedLayouts[slideIdx]
		// Placeholder content charts/diagrams use the same JSON Pointer
		// grammar as shape-grid findings and repair_slide.
		for contentIdx, item := range slide.Content {
			bounds := tablePlaceholderBounds(item.PlaceholderID, layout)
			switch item.Type {
			case "chart":
				if item.ChartValue == nil {
					continue
				}
				spec := chartValueToDiagramSpec(item.ChartValue)
				path := slidepath.ContentField(slideIdx, contentIdx, "chart_value")
				findings = append(findings,
					dryRenderSpecInBounds(spec, themeColors, bodyFont, strictFit, path, false, bounds)...)
			case "diagram":
				if item.DiagramValue == nil {
					continue
				}
				path := slidepath.ContentField(slideIdx, contentIdx, "diagram_value")
				findings = append(findings,
					dryRenderSpecInBounds(item.DiagramValue, themeColors, bodyFont, strictFit, path, false, bounds)...)
			}
		}

		// Shape-grid diagram surfaces. Paths use the slidepath JSON Pointer
		// convention shared with the structural detectors (e.g.
		// checkGridDiagramPreflight), so a dry-render finding and a structural
		// finding for the same cell agree on the cell identity.
		if slide.ShapeGrid != nil {
			geom, _ := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythmGrid)
			result := resolveGridForStructural(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
			findings = append(findings, collectGridDryRenderFindingsResolved(
				slide.ShapeGrid, slidepath.ShapeGrid(slideIdx), themeColors, bodyFont, strictFit,
				converterAvailable, result, slideWidth, slideHeight)...)
		} else if slide.Pattern != nil {
			// Named patterns that embed charts (chart-insights-split, ...)
			// are only expanded at generate time; expand them here so their
			// charts get the same dry-render as raw shape_grid diagrams
			// (go-slide-creator-yzbo). Paths are rooted at the pattern.
			geom, frame := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythmGrid)
			pg, _ := expandSlidePatternGridWithWarningsAtBounds(&slide, slideIdx, slideWidth, slideHeight, nil,
				patterns.LayoutBounds{X: frame.X, Y: frame.Y, Width: frame.CX, Height: frame.CY}, geom.Zone)
			if pg == nil {
				continue
			}
			result := resolveGridForStructural(pg, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
			findings = append(findings, collectGridDryRenderFindingsResolved(
				pg, slidepath.SlideField(slideIdx, "pattern"), themeColors, bodyFont, strictFit,
				converterAvailable, result, slideWidth, slideHeight)...)
		}
	}
	return chartFindingsAtAuthoredPaths(input, findings)
}

// collectGridDryRenderFindings recursively walks a shape grid's rows and cells,
// invoking svggen.DryRender for every embedded diagram surface: a cell's direct
// diagram, a composite cell's sub_diagram, and any diagram inside a recursively
// nested sub-grid.
//
// basePath is the slidepath JSON Pointer of the grid itself —
// "/slides/{i}/shape_grid" for a top-level grid, or ".../cells/{c}/grid" for a
// nested one. Row/cell/field segments are appended so each finding pins the
// owning cell and subfield. For a top-level cell diagram this yields exactly
// slidepath.GridCellField(slideIdx, ri, ci, "diagram"), matching the path the
// structural diagram detectors emit.
func collectGridDryRenderFindingsResolved(
	grid *ShapeGridInput,
	basePath string,
	themeColors []types.ThemeColor,
	bodyFont, strictFit string,
	converterAvailable bool,
	result *shapegrid.ResolveResult,
	slideWidth, slideHeight int64,
) []patterns.FitFinding {
	if grid == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", basePath, ri, ci)
			diagramBounds := resolvedGridCellBounds(result, ri, ci, shapegrid.CellKindDiagram)
			if cell.Diagram != nil {
				findings = append(findings, dryRenderGridSpecInBounds(
					cell.Diagram, themeColors, bodyFont, strictFit, cellPath+"/diagram", converterAvailable, diagramBounds)...)
			}
			if cell.Composite != nil && cell.Composite.SubDiagram != nil {
				findings = append(findings, dryRenderGridSpecInBounds(
					cell.Composite.SubDiagram, themeColors, bodyFont, strictFit, cellPath+"/composite/sub_diagram", converterAvailable, diagramBounds)...)
			}
			if cell.Grid != nil {
				var nested *shapegrid.ResolveResult
				if parent := resolvedGridCellBounds(result, ri, ci, shapegrid.CellKindSubGrid); parent.Width > 0 && parent.Height > 0 {
					inset := pptx.RectEmu{X: parent.X + subGridInsetEMU, Y: parent.Y + subGridInsetEMU,
						CX: parent.Width - 2*subGridInsetEMU, CY: parent.Height - 2*subGridInsetEMU}
					if inset.CX <= 0 || inset.CY <= 0 {
						inset = pptx.RectEmu{X: parent.X, Y: parent.Y, CX: parent.Width, CY: parent.Height}
					}
					nested = resolveGridForStructural(cell.Grid, &inset, nil, slideWidth, slideHeight)
				}
				findings = append(findings, collectGridDryRenderFindingsResolved(
					cell.Grid, cellPath+"/grid", themeColors, bodyFont, strictFit,
					converterAvailable, nested, slideWidth, slideHeight)...)
			}
		}
	}
	return findings
}

func resolvedGridCellBounds(result *shapegrid.ResolveResult, row, col int, kind shapegrid.CellKind) types.BoundingBox {
	if result == nil {
		return types.BoundingBox{}
	}
	for _, cell := range result.Cells {
		if cell.RowIdx == row && cell.ColIdx == col && cell.Kind == kind {
			return types.BoundingBox{X: cell.Bounds.X, Y: cell.Bounds.Y, Width: cell.Bounds.CX, Height: cell.Bounds.CY}
		}
	}
	return types.BoundingBox{}
}

// chartValueToDiagramSpec adapts the authored ChartSpec into the DiagramSpec
// svggen consumes — through ToDiagramSpec, the SAME conversion the render path
// runs.
//
// It used to copy c.Data verbatim, which meant the dry run validated the
// shorthand the author wrote rather than the payload the renderer builds from
// it. svggen then refused shapes it never sees: a waterfall authored as
// {"Revenue": 100, "Costs": -40} came back "requires 'points' array", a radar
// as "does not accept field Architecture". Four of the shipped example decks
// carried refuse-class findings for charts that render correctly
// (go-slide-creator-r87g). A dry run has to dry-run what actually happens.
func chartValueToDiagramSpec(c *types.ChartSpec) *types.DiagramSpec { //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
	if c == nil {
		return nil
	}
	return c.ToDiagramSpec()
}

// dryRenderSpecToFindings calls svggen.DryRender for a single DiagramSpec
// and converts the returned svggen.Findings into patterns.FitFinding entries
// pinned at the supplied path. A render error is dropped: placeholder
// content items fall back to a placeholder image at generate time and the
// chart_value shorthand forms are only normalized on the render path.
func dryRenderSpecToFindings(
	spec *types.DiagramSpec,
	themeColors []types.ThemeColor,
	bodyFont string,
	strictFit string,
	path string,
) []patterns.FitFinding {
	return dryRenderSpec(spec, themeColors, bodyFont, strictFit, path, false)
}

// dryRenderGridSpecInBounds is dryRenderSpecToFindings for shape_grid
// (and pattern-expanded) diagram cells. Generate aborts the deck when such a
// cell fails to render, so a dry-render error is surfaced as a
// diagram_render_failed finding with action "refuse" instead of being
// dropped (go-slide-creator-yzbo).
func dryRenderGridSpecInBounds(
	spec *types.DiagramSpec,
	themeColors []types.ThemeColor,
	bodyFont, strictFit, path string,
	converterAvailable bool,
	bounds types.BoundingBox,
) []patterns.FitFinding {
	findings := dryRenderSpecInBounds(spec, themeColors, bodyFont, strictFit, path, true, bounds)
	if converterAvailable || spec == nil || spec.Type == "" || generator.IsNativeDiagramType(spec) {
		return findings
	}
	return append(findings, patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: spec.Type,
			Path:    path,
			Code:    patterns.ErrCodeDiagramRenderFailed,
			Message: generator.GridDiagramConverterMissingMessage,
			Fix: &patterns.FixSuggestion{
				Kind:   "review",
				Params: map[string]any{"reason": generator.GridDiagramConverterMissingMessage},
			},
		},
		Action: "refuse",
	})
}

func dryRenderSpec(
	spec *types.DiagramSpec,
	themeColors []types.ThemeColor,
	bodyFont string,
	strictFit string,
	path string,
	gridSurface bool,
) []patterns.FitFinding {
	return dryRenderSpecInBounds(spec, themeColors, bodyFont, strictFit, path, gridSurface, types.BoundingBox{})
}

func dryRenderSpecInBounds(
	spec *types.DiagramSpec,
	themeColors []types.ThemeColor,
	bodyFont, strictFit, path string,
	gridSurface bool,
	bounds types.BoundingBox,
) []patterns.FitFinding {
	if spec == nil || spec.Type == "" {
		return nil
	}
	colorFindings := generator.SvggenFindingsToFit(
		generator.DiagramStyleColorFindings(spec, themeColors), spec.Type, path)
	// Half the diagram catalogue is drawn as native OOXML shapes by
	// internal/generator, not by svggen. Asking svggen about one of those gets
	// "unknown diagram type", which used to be reported as a REFUSE claiming
	// the deck would render a grey "Data unavailable" placeholder — for a
	// diagram that renders perfectly. validate is the precondition gate
	// SKILL.md tells agents to trust, so a false refuse there is worse than no
	// finding at all (go-slide-creator-r87g). A type neither renderer owns
	// still refuses: that one really does become a placeholder.
	if generator.IsNativeDiagramType(spec) {
		return generator.NativeDiagramPreflight(spec, bodyFont, path)
	}
	// Build a minimal RequestEnvelope. The full diagramSpecToSVGGen converter
	// in internal/generator pulls in too many dependencies (and is render-path
	// specific); for dry-run we only need geometry + palette routing so the
	// labeling pass produces correct findings.
	req := &svggen.RequestEnvelope{
		Type:     spec.Type,
		Title:    spec.Title,
		Subtitle: spec.Subtitle,
		Data:     spec.Data,
	}
	req.Output.Width, req.Output.Height, _ = generator.ResolveDiagramRenderDimensions(spec, bounds)
	req.Output.StrictFit = strictFit
	// Forward resolved colors so dry-render palette behavior matches generation.
	if spec.Style != nil && len(spec.Style.Colors) > 0 {
		effectiveTheme := themeColors
		if len(spec.Style.ThemeColors) > 0 {
			effectiveTheme = spec.Style.ThemeColors
		}
		for i, value := range spec.Style.Colors {
			color, ok := generator.ResolveDiagramStyleColor(value, effectiveTheme)
			if !ok {
				color = generator.ChartAccentFallback(i, effectiveTheme)
			}
			req.Style.Palette.Colors = append(req.Style.Palette.Colors, color)
		}
	}
	// Font: measure label fit under the same typeface the renderer will use.
	// An explicit per-diagram style.font_family wins; otherwise fall back to the
	// template body font (matching resolveDiagramWithMetadata /
	// generateDiagramCellInserts on the render path). svggen defaults an empty
	// FontFamily to Arial, so leaving it unset would measure label-fit findings
	// under a different font than the rendered diagram.
	if spec.Style != nil && spec.Style.FontFamily != "" {
		req.Style.FontFamily = spec.Style.FontFamily
	} else if bodyFont != "" {
		req.Style.FontFamily = bodyFont
	}
	dryFindings, renderErr := svggen.DryRender(req)
	out := colorFindings
	if renderErr != nil {
		// Both outcomes lose the visual, so both refuse. A grid/pattern surface
		// aborts generation outright; a content placeholder degrades to a
		// slide-sized grey "Data unavailable" box, which used to be predicted
		// nowhere at all — validate said clean and the deck shipped the
		// placeholder (go-slide-creator-rrjj).
		outcome := "render as a \"Data unavailable\" placeholder instead of the chart"
		if gridSurface {
			outcome = "fail to render and abort generation"
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: spec.Type,
				Path:    path,
				Code:    patterns.ErrCodeDiagramRenderFailed,
				Message: fmt.Sprintf("diagram would %s: %v", outcome, renderErr),
				Fix: &patterns.FixSuggestion{
					Kind:   "review",
					Params: map[string]any{"diagram_type": spec.Type, "reason": renderErr.Error()},
				},
			},
			Action: "refuse",
		})
	}
	if len(dryFindings) == 0 {
		return out
	}
	// One conversion, shared with the render path, so a dry-run finding and the
	// same finding raised while actually drawing agree on code, path and action.
	return append(out, generator.SvggenFindingsToFit(dryFindings, spec.Type, path)...)
}
