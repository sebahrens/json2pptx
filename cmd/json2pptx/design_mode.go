package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// designModeConstrained is the default mode that restricts raw hex colors and
// absolute font sizes to enforce brand consistency.
const designModeConstrained = "constrained"

// designModeFree unlocks raw controls — hex colors and absolute sizes pass through.
const designModeFree = "free"

// effectiveDesignMode returns the design mode for the deck, defaulting to constrained.
func effectiveDesignMode(input *PresentationInput) string {
	switch input.DesignMode {
	case designModeFree:
		return designModeFree
	default:
		return designModeConstrained
	}
}

// validateDesignMode checks the deck for design mode violations when in constrained mode.
// Returns fit findings for each violation found.
func validateDesignMode(input *PresentationInput) []patterns.FitFinding {
	if effectiveDesignMode(input) != designModeConstrained {
		return nil
	}

	var findings []patterns.FitFinding

	for i, slide := range input.Slides {
		slideNum := i + 1

		// Check shape_grid cells
		if slide.ShapeGrid != nil {
			findings = append(findings, checkShapeGrid(slide.ShapeGrid, slideNum)...)
		}

		// Check pattern overrides (patterns expand to shape grids, but the
		// override fields are user-specified and can contain raw colors).
		if slide.Pattern != nil {
			findings = append(findings, checkPatternInput(slide.Pattern, slideNum)...)
		}

		// Check compose segments for pattern overrides.
		if slide.Compose != nil {
			for _, seg := range slide.Compose.Segments {
				findings = append(findings, checkPatternInput(&seg.Pattern, slideNum)...)
			}
		}

		// Check content items (chart/diagram colors)
		for j, ci := range slide.Content {
			findings = append(findings, checkContentInput(&ci, slideNum, j+1)...)
		}
	}

	return findings
}

// checkShapeGrid scans a ShapeGridInput for raw hex colors and absolute font sizes.
func checkShapeGrid(grid *ShapeGridInput, slideNum int) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for ri, row := range grid.Rows {
		if row.Connector != nil && row.Connector.Color != "" {
			if f := checkColorField(row.Connector.Color, slideNum,
				fmt.Sprintf("shape_grid.rows[%d].connector.color", ri)); f != nil {
				findings = append(findings, *f)
			}
		}

		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("shape_grid.rows[%d].cells[%d]", ri, ci)
			findings = append(findings, checkGridCell(cell, slideNum, cellPath)...)
		}
	}

	return findings
}

// checkGridCell validates a single grid cell for design mode violations.
func checkGridCell(cell *GridCellInput, slideNum int, cellPath string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if cell.Shape != nil {
		findings = append(findings, checkShapeSpec(cell.Shape, slideNum, cellPath)...)
	}

	if cell.AccentBar != nil && cell.AccentBar.Color != "" {
		if f := checkColorField(cell.AccentBar.Color, slideNum,
			cellPath+".accent_bar.color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if cell.Icon != nil && cell.Icon.Fill != "" {
		if f := checkColorField(cell.Icon.Fill, slideNum,
			cellPath+".icon.fill"); f != nil {
			findings = append(findings, *f)
		}
	}

	if cell.Image != nil {
		findings = append(findings, checkGridImage(cell.Image, slideNum, cellPath)...)
	}

	if cell.Diagram != nil && cell.Diagram.Style != nil {
		findings = append(findings, checkDiagramStyleColors(cell.Diagram.Style.Colors, slideNum, cellPath+".diagram.style.colors")...)
		if cell.Diagram.Style.Background != "" {
			if f := checkColorField(cell.Diagram.Style.Background, slideNum,
				cellPath+".diagram.style.background"); f != nil {
				findings = append(findings, *f)
			}
		}
	}

	if cell.Table != nil {
		findings = append(findings, checkTableInput(cell.Table, slideNum, cellPath+".table")...)
	}

	return findings
}

// checkGridImage validates image overlay and text colors.
func checkGridImage(img *GridImageInput, slideNum int, cellPath string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if img.Overlay != nil && img.Overlay.Color != "" {
		if f := checkColorField(img.Overlay.Color, slideNum,
			cellPath+".image.overlay.color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if img.Text != nil && img.Text.Color != "" {
		if f := checkColorField(img.Text.Color, slideNum,
			cellPath+".image.text.color"); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

// checkShapeSpec validates fill, line, and text color fields in a ShapeSpecInput.
func checkShapeSpec(spec *ShapeSpecInput, slideNum int, basePath string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// Check fill
	if len(spec.Fill) > 0 {
		if f := checkRawMessageColor(spec.Fill, slideNum, basePath+".shape.fill"); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check line
	if len(spec.Line) > 0 {
		if f := checkRawMessageColor(spec.Line, slideNum, basePath+".shape.line"); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check text color and font size
	if len(spec.Text) > 0 {
		findings = append(findings, checkTextRaw(spec.Text, slideNum, basePath+".shape.text")...)
	}

	return findings
}

// textParagraphProbe is a minimal struct for probing paragraph color/size fields.
type textParagraphProbe struct {
	Color string  `json:"color"`
	Size  float64 `json:"size"`
}

// checkTextRaw examines a text json.RawMessage for raw colors and absolute sizes.
func checkTextRaw(raw json.RawMessage, slideNum int, path string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// Try string form (just content, no color/size)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return nil
	}

	// Object form
	var obj struct {
		Color      string               `json:"color"`
		Size       float64              `json:"size"`
		Paragraphs []textParagraphProbe `json:"paragraphs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}

	if obj.Color != "" {
		if f := checkColorField(obj.Color, slideNum, path+".color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if obj.Size > 0 {
		if f := checkAbsoluteSize(obj.Size, slideNum, path+".size"); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check paragraphs array
	for i, p := range obj.Paragraphs {
		pPath := fmt.Sprintf("%s.paragraphs[%d]", path, i)
		if p.Color != "" {
			if f := checkColorField(p.Color, slideNum, pPath+".color"); f != nil {
				findings = append(findings, *f)
			}
		}
		if p.Size > 0 {
			if f := checkAbsoluteSize(p.Size, slideNum, pPath+".size"); f != nil {
				findings = append(findings, *f)
			}
		}
	}

	return findings
}

// checkRawMessageColor checks a json.RawMessage that can be a string or object with "color".
func checkRawMessageColor(raw json.RawMessage, slideNum int, path string) *patterns.FitFinding {
	// Try string form
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return checkColorField(s, slideNum, path)
	}

	// Object form
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Color != "" {
		return checkColorField(obj.Color, slideNum, path+".color")
	}

	return nil
}

// checkColorField returns a fit finding if the color is a raw hex value.
func checkColorField(color string, slideNum int, path string) *patterns.FitFinding {
	if color == "" || color == "none" {
		return nil
	}

	// If it's a scheme color, it's allowed
	if pptx.IsSchemeColor(color) {
		return nil
	}

	// Check if it looks like a hex color (with or without #)
	hex := strings.TrimPrefix(color, "#")
	if !isHexString(hex) {
		return nil // Not a recognized color format, skip
	}

	nearest := suggestNearestSchemeColor(hex)
	suggestion := fmt.Sprintf("use scheme color %q instead of raw hex %q", nearest, color)

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "design_mode",
			Path:    path,
			Code:    "design_mode_violation",
			Message: fmt.Sprintf("slide %d: %s uses raw hex color %q — constrained mode requires scheme colors (accent1-6, dk1, dk2, lt1, lt2, etc.)", slideNum, path, color),
			Fix:     patterns.TextFix(suggestion),
		},
		Action: "refuse",
	}
}

// checkAbsoluteSize returns a fit finding if an absolute font size is used.
func checkAbsoluteSize(size float64, slideNum int, path string) *patterns.FitFinding {
	// In constrained mode, absolute sizes on body text are not allowed.
	// We flag any explicit numeric size — users should rely on template defaults.
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "design_mode",
			Path:    path,
			Code:    "design_mode_violation",
			Message: fmt.Sprintf("slide %d: %s uses absolute font size %.0fpt — constrained mode requires template-managed sizes; remove the size field to use template defaults", slideNum, path, size),
			Fix:     patterns.TextFix("remove the \"size\" field to use the template's default body text size"),
		},
		Action: "refuse",
	}
}

// checkPatternInput checks a pattern's override fields for raw hex colors.
func checkPatternInput(pattern *PatternInput, slideNum int) []patterns.FitFinding {
	if pattern == nil || len(pattern.Overrides) == 0 {
		return nil
	}

	var findings []patterns.FitFinding

	// Overrides is json.RawMessage — decode as map for scanning.
	var overrides map[string]any
	if err := json.Unmarshal(pattern.Overrides, &overrides); err != nil {
		return nil
	}

	for key, val := range overrides {
		path := fmt.Sprintf("pattern.overrides.%s", key)
		if isColorKey(key) {
			if s, ok := val.(string); ok {
				if f := checkColorField(s, slideNum, path); f != nil {
					findings = append(findings, *f)
				}
			}
		}
	}

	return findings
}

// checkContentInput checks a content item for raw hex colors (chart/diagram data).
func checkContentInput(ci *ContentInput, slideNum, contentNum int) []patterns.FitFinding {
	var findings []patterns.FitFinding
	basePath := slidepath.ContentIndex(slideNum-1, contentNum-1)

	// Check diagram_value style colors
	if ci.DiagramValue != nil && ci.DiagramValue.Style != nil {
		findings = append(findings, checkDiagramStyleColors(
			ci.DiagramValue.Style.Colors, slideNum, basePath+".diagram_value.style.colors")...)
		if ci.DiagramValue.Style.Background != "" {
			if f := checkColorField(ci.DiagramValue.Style.Background, slideNum,
				basePath+".diagram_value.style.background"); f != nil {
				findings = append(findings, *f)
			}
		}
	}

	// Check chart_value (deprecated but still used)
	if ci.ChartValue != nil && ci.ChartValue.Style != nil {
		findings = append(findings, checkDiagramStyleColors(
			ci.ChartValue.Style.Colors, slideNum, basePath+".chart_value.style.colors")...)
	}

	return findings
}

// checkDiagramStyleColors checks a slice of color strings for raw hex values.
func checkDiagramStyleColors(colors []string, slideNum int, basePath string) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for i, c := range colors {
		path := fmt.Sprintf("%s[%d]", basePath, i)
		if f := checkColorField(c, slideNum, path); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// checkTableInput checks table style and conditional format fills for raw hex colors.
func checkTableInput(table *TableInput, slideNum int, basePath string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if table.Style != nil && table.Style.HeaderBackground != nil && *table.Style.HeaderBackground != "" {
		if f := checkColorField(*table.Style.HeaderBackground, slideNum, basePath+".style.header_background"); f != nil {
			findings = append(findings, *f)
		}
	}

	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Conditional != nil && cell.Conditional.Fill != "" {
				path := fmt.Sprintf("%s.rows[%d][%d].conditional.fill", basePath, ri, ci)
				if f := checkColorField(cell.Conditional.Fill, slideNum, path); f != nil {
					findings = append(findings, *f)
				}
			}
		}
	}

	return findings
}

// isHexString checks if a string is a valid 3 or 6 character hex color.
func isHexString(s string) bool {
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isColorKey returns true if a key name suggests it holds a color value.
func isColorKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "color") ||
		strings.Contains(lower, "fill") ||
		strings.HasSuffix(lower, "_bg") ||
		strings.HasSuffix(lower, "_fg")
}

// suggestNearestSchemeColor finds the closest scheme color to a given hex value.
// Uses perceptual color distance (simple RGB Euclidean) against the standard
// OOXML accent palette to suggest the best scheme name.
func suggestNearestSchemeColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")

	// Expand 3-char to 6-char
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	r, g, b := parseHexRGB(hex)

	// Standard palette for suggestion (these are the scheme names, not the actual
	// template colors which vary). We suggest based on luminance and hue heuristics.
	type candidate struct {
		name string
		r, g, b uint8
	}

	// Common accent palette approximations (midtones)
	candidates := []candidate{
		{"accent1", 68, 114, 196},   // blue
		{"accent2", 237, 125, 49},   // orange
		{"accent3", 165, 165, 165},  // gray
		{"accent4", 255, 192, 0},    // gold
		{"accent5", 91, 155, 213},   // light blue
		{"accent6", 112, 173, 71},   // green
		{"dk1", 0, 0, 0},            // black
		{"dk2", 68, 84, 106},        // dark gray-blue
		{"lt1", 255, 255, 255},      // white
		{"lt2", 228, 230, 232},      // light gray
	}

	bestName := "accent1"
	bestDist := math.MaxFloat64

	for _, c := range candidates {
		d := colorDistance(r, g, b, c.r, c.g, c.b)
		if d < bestDist {
			bestDist = d
			bestName = c.name
		}
	}

	return bestName
}

// parseHexRGB parses a 6-character hex string into RGB components.
func parseHexRGB(hex string) (uint8, uint8, uint8) {
	if len(hex) < 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return uint8(r), uint8(g), uint8(b)
}

// colorDistance computes Euclidean distance in RGB space.
func colorDistance(r1, g1, b1, r2, g2, b2 uint8) float64 {
	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
