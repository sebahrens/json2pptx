package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
// strictFit threads through to svggen's severity promotion ladder so warn /
// strict modes promote chart findings consistently with the generate path.
func collectChartDryRenderFindings(
	input *PresentationInput,
	themeColors []types.ThemeColor,
	strictFit string,
) []patterns.FitFinding {
	if input == nil || len(input.Slides) == 0 {
		return nil
	}
	var findings []patterns.FitFinding
	for slideIdx, slide := range input.Slides {
		// Placeholder content charts/diagrams. Paths use the legacy bracket
		// notation these findings have always emitted (preserved for callers
		// and tests that key off it).
		for contentIdx, item := range slide.Content {
			switch item.Type {
			case "chart":
				if item.ChartValue == nil {
					continue
				}
				spec := chartValueToDiagramSpec(item.ChartValue)
				path := fmt.Sprintf("slides[%d].content[%d].chart_value", slideIdx, contentIdx)
				findings = append(findings,
					dryRenderSpecToFindings(spec, themeColors, strictFit, path)...)
			case "diagram":
				if item.DiagramValue == nil {
					continue
				}
				path := fmt.Sprintf("slides[%d].content[%d].diagram_value", slideIdx, contentIdx)
				findings = append(findings,
					dryRenderSpecToFindings(item.DiagramValue, themeColors, strictFit, path)...)
			}
		}

		// Shape-grid diagram surfaces. Paths use the slidepath JSON Pointer
		// convention shared with the structural detectors (e.g.
		// checkGridDiagramPreflight), so a dry-render finding and a structural
		// finding for the same cell agree on the cell identity.
		if slide.ShapeGrid != nil {
			findings = append(findings, collectGridDryRenderFindings(
				slide.ShapeGrid, slidepath.ShapeGrid(slideIdx), themeColors, strictFit)...)
		}
	}
	return findings
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
func collectGridDryRenderFindings(
	grid *ShapeGridInput,
	basePath string,
	themeColors []types.ThemeColor,
	strictFit string,
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
			if cell.Diagram != nil {
				findings = append(findings, dryRenderSpecToFindings(
					cell.Diagram, themeColors, strictFit, cellPath+"/diagram")...)
			}
			if cell.Composite != nil && cell.Composite.SubDiagram != nil {
				findings = append(findings, dryRenderSpecToFindings(
					cell.Composite.SubDiagram, themeColors, strictFit, cellPath+"/composite/sub_diagram")...)
			}
			if cell.Grid != nil {
				findings = append(findings, collectGridDryRenderFindings(
					cell.Grid, cellPath+"/grid", themeColors, strictFit)...)
			}
		}
	}
	return findings
}

// chartValueToDiagramSpec adapts the legacy ChartSpec into the DiagramSpec
// shape that svggen consumes. Only the fields needed for dry-render geometry
// (type, data, dimensions, style) are forwarded.
func chartValueToDiagramSpec(c *types.ChartSpec) *types.DiagramSpec { //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
	if c == nil {
		return nil
	}
	spec := &types.DiagramSpec{
		Type:   string(c.Type),
		Title:  c.Title,
		Data:   c.Data,
		Width:  c.Width,
		Height: c.Height,
		Scale:  c.Scale,
	}
	if c.Style != nil {
		spec.Style = &types.DiagramStyle{
			Colors:     c.Style.Colors,
			FontFamily: c.Style.FontFamily,
			ShowLegend: c.Style.ShowLegend,
		}
	}
	return spec
}

// dryRenderSpecToFindings calls svggen.DryRender for a single DiagramSpec
// and converts the returned svggen.Findings into patterns.FitFinding entries
// pinned at the supplied path.
func dryRenderSpecToFindings(
	spec *types.DiagramSpec,
	themeColors []types.ThemeColor,
	strictFit string,
	path string,
) []patterns.FitFinding {
	if spec == nil || spec.Type == "" {
		return nil
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
	if spec.Width > 0 {
		req.Output.Width = spec.Width
	}
	if spec.Height > 0 {
		req.Output.Height = spec.Height
	}
	req.Output.StrictFit = strictFit
	// Style: forward explicit accent colors when set so palette-sensitive
	// findings (none currently, but room to grow) see the right palette.
	if spec.Style != nil && len(spec.Style.Colors) > 0 {
		req.Style.Palette.Colors = spec.Style.Colors
	}
	// Theme colors enable contrast-related findings if any are added later.
	_ = themeColors

	dryFindings, _ := svggen.DryRender(req)
	if len(dryFindings) == 0 {
		return nil
	}
	out := make([]patterns.FitFinding, 0, len(dryFindings))
	for _, df := range dryFindings {
		fixSug := convertSvggenFixSuggestion(df.Fix)
		fieldPath := path
		if df.Field != "" {
			fieldPath = path + "." + df.Field
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: spec.Type,
				Path:    fieldPath,
				Code:    df.Code,
				Message: df.Message,
				Fix:     fixSug,
			},
			Action: actionForSvggenSeverity(df.Severity),
		})
	}
	return out
}

// convertSvggenFixSuggestion mirrors svggen.FixSuggestion into the
// patterns.FixSuggestion shape FitFinding expects. Returns nil when the input
// is nil.
func convertSvggenFixSuggestion(fs *svggen.FixSuggestion) *patterns.FixSuggestion {
	if fs == nil {
		return nil
	}
	return &patterns.FixSuggestion{
		Kind:   fs.Kind,
		Params: fs.Params,
	}
}

// actionForSvggenSeverity maps the svggen severity ladder onto the FitFinding
// Action axis. The mapping mirrors the existing strict-fit promotion table:
//
//	info  / warning              → "review"
//	shrink_or_split               → "shrink_or_split"
//	refuse                        → "refuse"
//
// Anything unrecognised falls back to "info" so unknown codes still surface
// without claiming a severity they don't have.
func actionForSvggenSeverity(sev string) string {
	switch sev {
	case "refuse":
		return "refuse"
	case "shrink_or_split":
		return "shrink_or_split"
	case "warning":
		return "review"
	case "info":
		return "info"
	default:
		return "info"
	}
}
