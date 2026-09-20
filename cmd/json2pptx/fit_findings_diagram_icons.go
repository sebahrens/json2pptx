package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

// Unknown bundled icon names in diagram panels (go-slide-creator-puki).
//
// panels: [{title: "Simplify", icon: "layers"}] rendered a panel with no icon
// while its siblings had one — a slide that looks broken — and the only trace
// was a server log line nobody reads:
//
//	WARN panel native shapes: icon not embedded reason="unknown bundled icon \"layers\""
//
// A shape_grid icon CELL already reports the same typo as
// ICON_BUNDLED_NAME_UNKNOWN with suggestions. A panel icon did not, so the one
// mistake had two fates depending on which surface an author happened to use.

// collectDiagramIconFindings reports bundled icon names in diagram panels that
// do not resolve. It walks the same diagram surfaces the dry-render collector
// does: content-item diagrams, shape_grid cells, and pattern-expanded grids.
func collectDiagramIconFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for slideIdx := range input.Slides {
		slide := input.Slides[slideIdx]
		for contentIdx, item := range slide.Content {
			if item.Type != "diagram" || item.DiagramValue == nil {
				continue
			}
			path := fmt.Sprintf("slides[%d].content[%d].diagram_value", slideIdx, contentIdx)
			findings = append(findings, diagramSpecIconFindings(item.DiagramValue, path)...)
		}
		grid := slide.ShapeGrid
		base := slidepath.ShapeGrid(slideIdx)
		if grid == nil {
			if pg := expandSlidePatternGrid(&slide, slideIdx, 0, 0, nil); pg != nil {
				grid, base = pg, slidepath.SlideField(slideIdx, "pattern")
			}
		}
		findings = append(findings, gridDiagramIconFindings(grid, base)...)
	}
	return findings
}

// gridDiagramIconFindings walks a grid's cells for diagram surfaces, including
// nested sub-grids.
func gridDiagramIconFindings(grid *ShapeGridInput, basePath string) []patterns.FitFinding {
	if grid == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellBase := fmt.Sprintf("%s/rows/%d/cells/%d", basePath, ri, ci)
			if cell.Diagram != nil {
				findings = append(findings, diagramSpecIconFindings(cell.Diagram, cellBase+"/diagram")...)
			}
			findings = append(findings, gridDiagramIconFindings(cell.Grid, cellBase+"/grid")...)
		}
	}
	return findings
}

// diagramSpecIconFindings checks every panels[].icon of one diagram spec.
func diagramSpecIconFindings(spec *types.DiagramSpec, path string) []patterns.FitFinding {
	if spec == nil || spec.Data == nil {
		return nil
	}
	panels, ok := spec.Data["panels"].([]any)
	if !ok {
		return nil
	}
	var findings []patterns.FitFinding
	for i, raw := range panels {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, ok := m["icon"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		// Only a BARE NAME addresses the bundled registry; inline SVG, data
		// URIs, paths and URLs are resolved elsewhere and are not typos.
		// ClassifyIcon answers IconKindName only for a name that RESOLVES, so
		// the test is what the string is not, rather than what it is.
		switch svggen.ClassifyIcon(name) {
		case svggen.IconKindDataURI, svggen.IconKindURL, svggen.IconKindInlineSVG, svggen.IconKindFilePath:
			continue
		}
		if _, err := icons.Lookup(name); err == nil {
			continue
		}
		findings = append(findings, unknownDiagramIconFinding(spec.Type, name, fmt.Sprintf("%s.data.panels[%d].icon", path, i)))
	}
	return findings
}

// unknownDiagramIconFinding builds the finding, carrying the same suggestions
// the shape_grid icon path offers so the repair is one edit either way.
//
// The action is "review", not the shape_grid path's error: a grid icon cell is
// a cell that cannot be drawn at all, while a panel icon is a decoration the
// renderer drops — the panel still renders, which is exactly why it went
// unnoticed.
func unknownDiagramIconFinding(diagramType, name, path string) patterns.FitFinding {
	suggestions := icons.Suggest(name, 3)
	params := map[string]any{
		"input_value": name,
		"remediation": "use a bundled icon name (json2pptx icons / list_icons lists them), or supply svg_data instead",
	}
	msg := fmt.Sprintf("icon name %q is not in the bundled registry, so this panel renders without an icon while its siblings keep theirs", name)
	if len(suggestions) > 0 {
		params["suggestions"] = suggestions
		params["to"] = suggestions[0]
		msg = fmt.Sprintf("%s; did you mean %q?", msg, suggestions[0])
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: diagramType,
			Path:    path,
			Code:    string(diagnostics.CodeIconBundledNameUnknown),
			Message: msg,
			Fix:     &patterns.FixSuggestion{Kind: "replace_value", Params: params},
		},
		Action: "review",
	}
}
