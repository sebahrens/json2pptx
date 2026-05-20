package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// collectAltTextFindings emits MISSING_ALT_TEXT advisory findings when a slide
// carries an image or icon whose source is a file path, URL, or inline SVG
// markup but whose alt field is empty. Bundled built-in icons referenced by
// name are exempt because the qualified name itself supplies an implicit
// caption.
//
// Coverage:
//   - slide.Content[].ImageValue                       (image_value)
//   - slide.ShapeGrid.Rows[].Cells[].Image             (GridImageInput)
//   - slide.ShapeGrid.Rows[].Cells[].Icon              (cell-level IconInput)
//   - slide.ShapeGrid.Rows[].Cells[].Shape.Icon        (shape-overlay IconInput)
//
// All findings have action "review"; they never block render.
func collectAltTextFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		findings = append(findings, contentImageAltFindings(si, slide.Content)...)
		findings = append(findings, gridAltFindings(si, slide.ShapeGrid)...)
	}
	return findings
}

// contentImageAltFindings emits MISSING_ALT_TEXT for content-level image_value
// items whose source is set but whose alt is empty.
func contentImageAltFindings(slideIdx int, content []ContentInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for ci, c := range content {
		if c.Type != "image" || c.ImageValue == nil {
			continue
		}
		src := imageSourceKind(c.ImageValue.Path, c.ImageValue.URL, "")
		if src == "" || strings.TrimSpace(c.ImageValue.Alt) != "" {
			continue
		}
		path := slidepath.ContentField(slideIdx, ci, "image_value")
		findings = append(findings, makeAltTextFinding(path, slideIdx, "image_value", src))
	}
	return findings
}

// gridAltFindings walks shape_grid cells and emits MISSING_ALT_TEXT for grid
// image, cell-level icon, and shape-overlay icon assets.
func gridAltFindings(slideIdx int, grid *ShapeGridInput) []patterns.FitFinding {
	if grid == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			findings = append(findings, cellAltFindings(slideIdx, ri, ci, cell)...)
		}
	}
	return findings
}

// cellAltFindings checks a single grid cell's image / icon / shape.icon
// surfaces for missing alt text.
func cellAltFindings(slideIdx, ri, ci int, cell *GridCellInput) []patterns.FitFinding {
	if cell == nil {
		return nil
	}
	var findings []patterns.FitFinding
	if cell.Image != nil {
		if src := imageSourceKind(cell.Image.Path, cell.Image.URL, ""); src != "" && strings.TrimSpace(cell.Image.Alt) == "" {
			path := slidepath.GridCellField(slideIdx, ri, ci, "image")
			findings = append(findings, makeAltTextFinding(path, slideIdx, "image", src))
		}
	}
	if cell.Icon != nil {
		if src := iconSourceKind(cell.Icon); src != "" && strings.TrimSpace(cell.Icon.Alt) == "" {
			path := slidepath.GridCellField(slideIdx, ri, ci, "icon")
			findings = append(findings, makeAltTextFinding(path, slideIdx, "icon", src))
		}
	}
	if cell.Shape != nil && cell.Shape.Icon != nil {
		if src := iconSourceKind(cell.Shape.Icon); src != "" && strings.TrimSpace(cell.Shape.Icon.Alt) == "" {
			path := slidepath.GridCellField(slideIdx, ri, ci, "shape/icon")
			findings = append(findings, makeAltTextFinding(path, slideIdx, "icon", src))
		}
	}
	return findings
}

// imageSourceKind returns "path", "url", "svg_data", or "" — the source field
// the image / icon is loaded from. Empty when no remote source is set
// (e.g., a bundled icon name).
func imageSourceKind(path, url, svgData string) string {
	if strings.TrimSpace(path) != "" {
		return "path"
	}
	if strings.TrimSpace(url) != "" {
		return "url"
	}
	if strings.TrimSpace(svgData) != "" {
		return "svg_data"
	}
	return ""
}

// iconSourceKind returns the IconInput source kind, treating a bundled name as
// "" (exempt — the qualified name supplies an implicit caption).
func iconSourceKind(icon *IconInput) string {
	if icon == nil {
		return ""
	}
	return imageSourceKind(icon.Path, icon.URL, icon.SVGData)
}

// makeAltTextFinding constructs the MISSING_ALT_TEXT finding for one asset.
// The kind argument is the asset kind ("image_value", "image", "icon"); the
// source argument is the source field name ("path", "url", "svg_data") that
// triggered the check.
func makeAltTextFinding(path string, slideIdx int, kind, source string) patterns.FitFinding {
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeMissingAltText,
			Message: fmt.Sprintf(
				"slide %d: %s sourced from %s is missing alt text — set alt for screen-reader accessibility",
				slideIdx+1, kind, source),
			Fix: &patterns.FixSuggestion{
				Kind: "provide_value",
				Params: map[string]any{
					"field":  "alt",
					"kind":   kind,
					"source": source,
				},
			},
		},
		Action: "review",
	}
}
