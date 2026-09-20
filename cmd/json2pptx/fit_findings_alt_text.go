package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// collectAltTextFindings emits MISSING_ALT_TEXT advisory findings when a slide
// carries a visual with no alt text: an image or icon whose source is a file
// path, URL, or inline SVG markup, or a chart, diagram or table with nothing
// authored to announce. Bundled built-in icons referenced by name are exempt
// because the qualified name itself supplies an implicit caption.
//
// Coverage:
//   - slide.Content[].ImageValue                       (image_value)
//   - slide.ShapeGrid.Rows[].Cells[].Image             (GridImageInput)
//   - slide.ShapeGrid.Rows[].Cells[].Icon              (cell-level IconInput)
//   - slide.ShapeGrid.Rows[].Cells[].Shape.Icon        (shape-overlay IconInput)
//   - slide.Content[].ChartValue / DiagramValue        (chart_value, diagram_value)
//   - slide.Content[].TableValue                       (table_value)
//
// A chart, diagram or table still renders with a derived description (see
// generator.DiagramAltTextFor / generator.TableAltText) — "Bar chart, ARR by
// segment. 6 categories, 1 series, values from 0.4 to 1.24." beats the type
// name it used to announce, but only the author knows what the visual is FOR.
//
// patternSlides names the slides whose ShapeGrid was synthesized by expanding a
// named pattern. Grid-level chart/diagram/table cells on those slides are
// exempt: the cells are the pattern's, not the author's, and there is no field
// in the authored payload to put an alt in. Grid image and icon cells are not
// gated this way — a pattern's placeholder assets carry no source, so they are
// already exempt by the source test.
//
// All findings have action "review"; they never block render.
func collectAltTextFindings(input *PresentationInput, patternSlides map[int]bool) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		findings = append(findings, contentImageAltFindings(si, slide.Content)...)
		findings = append(findings, contentVisualAltFindings(si, slide.Content)...)
		findings = append(findings, gridAltFindings(si, slide.ShapeGrid, !patternSlides[si])...)
	}
	return findings
}

// contentVisualAltFindings emits MISSING_ALT_TEXT for chart, diagram and table
// content items with no authored alt.
func contentVisualAltFindings(slideIdx int, content []ContentInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for ci := range content {
		kind, field, alt := visualAltSurface(&content[ci])
		if kind == "" || strings.TrimSpace(alt) != "" {
			continue
		}
		path := slidepath.ContentField(slideIdx, ci, field)
		findings = append(findings, makeVisualAltTextFinding(path, slideIdx, kind, field))
	}
	return findings
}

// visualAltSurface reports the alt-bearing surface of a chart / diagram / table
// content item: the asset kind, the payload field to address, and the alt that
// is set there. It returns an empty kind for every other content type and for
// an item that carries no payload at all (an empty item is a different defect,
// reported elsewhere).
func visualAltSurface(c *ContentInput) (kind, field, alt string) {
	switch c.Type {
	case "chart":
		if c.ChartValue != nil {
			return "chart", "chart_value", c.ChartValue.Alt
		}
		return legacyVisualAltSurface(c, "chart", "chart_value")
	case "diagram":
		if c.DiagramValue != nil {
			return "diagram", "diagram_value", c.DiagramValue.Alt
		}
		return legacyVisualAltSurface(c, "diagram", "diagram_value")
	case "table":
		if c.TableValue != nil {
			return "table", "table_value", c.TableValue.Alt
		}
		return legacyVisualAltSurface(c, "table", "table_value")
	}
	return "", "", ""
}

// legacyVisualAltSurface reads the alt out of the legacy untyped "value"
// payload, so a deck written before the typed fields existed is linted the same
// way as one written after.
func legacyVisualAltSurface(c *ContentInput, kind, field string) (string, string, string) {
	if len(c.Value) == 0 {
		return "", "", ""
	}
	var payload struct {
		Alt string `json:"alt"`
	}
	if err := json.Unmarshal(c.Value, &payload); err != nil {
		return kind, field, ""
	}
	return kind, field, payload.Alt
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
// image, cell-level icon, and shape-overlay icon assets. When authored is true
// the cells came from the author's own shape_grid, so the diagram and table
// cells are checked too.
func gridAltFindings(slideIdx int, grid *ShapeGridInput, authored bool) []patterns.FitFinding {
	return gridAltFindingsAt(slideIdx, grid, authored, slidepath.ShapeGrid(slideIdx))
}

func gridAltFindingsAt(slideIdx int, grid *ShapeGridInput, authored bool, base string) []patterns.FitFinding {
	if grid == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", base, ri, ci)
			findings = append(findings, cellAltFindingsAt(slideIdx, cell, authored, cellPath)...)
			if cell != nil && cell.Grid != nil {
				findings = append(findings, gridAltFindingsAt(slideIdx, cell.Grid, authored, cellPath+"/grid")...)
			}
		}
	}
	return findings
}

// cellAltFindings checks a single grid cell's image / icon / shape.icon
// surfaces for missing alt text, plus its diagram and table surfaces when the
// cell was authored rather than expanded from a pattern.
func cellAltFindingsAt(slideIdx int, cell *GridCellInput, authored bool, cellPath string) []patterns.FitFinding {
	if cell == nil {
		return nil
	}
	var findings []patterns.FitFinding
	if authored {
		if cell.Diagram != nil && strings.TrimSpace(cell.Diagram.Alt) == "" {
			path := cellPath + "/diagram"
			findings = append(findings, makeVisualAltTextFinding(path, slideIdx, "diagram", "diagram"))
		}
		if cell.Table != nil && strings.TrimSpace(cell.Table.Alt) == "" {
			path := cellPath + "/table"
			findings = append(findings, makeVisualAltTextFinding(path, slideIdx, "table", "table"))
		}
	}
	if cell.Image != nil {
		if src := imageSourceKind(cell.Image.Path, cell.Image.URL, ""); src != "" && strings.TrimSpace(cell.Image.Alt) == "" {
			path := cellPath + "/image"
			findings = append(findings, makeAltTextFinding(path, slideIdx, "image", src))
		}
	}
	if cell.Icon != nil {
		if src := iconSourceKind(cell.Icon); src != "" && strings.TrimSpace(cell.Icon.Alt) == "" {
			path := cellPath + "/icon"
			findings = append(findings, makeAltTextFinding(path, slideIdx, "icon", src))
		}
	}
	if cell.Shape != nil && cell.Shape.Icon != nil {
		if src := iconSourceKind(cell.Shape.Icon); src != "" && strings.TrimSpace(cell.Shape.Icon.Alt) == "" {
			path := cellPath + "/shape/icon"
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

// makeVisualAltTextFinding constructs the MISSING_ALT_TEXT finding for a chart,
// diagram or table — a visual that always renders, with a description derived
// from its own data when the author wrote none.
func makeVisualAltTextFinding(path string, slideIdx int, kind, field string) patterns.FitFinding {
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeMissingAltText,
			Message: fmt.Sprintf(
				"slide %d: %s has no alt text — a screen reader gets the description derived from its data; set alt to one sentence saying what it shows",
				slideIdx+1, kind),
			Fix: &patterns.FixSuggestion{
				Kind: "provide_value",
				Params: map[string]any{
					"field": "alt",
					"kind":  kind,
					"on":    field,
				},
			},
		},
		Action: "review",
	}
}
