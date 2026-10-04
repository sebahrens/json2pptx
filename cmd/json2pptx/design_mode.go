package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
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
	// A value a table took from the deck defaults is one field in the deck,
	// however many tables render it.
	defaultsSeen := map[string]bool{}
	for i := range input.Slides {
		for _, f := range validateSlideDesignMode(&input.Slides[i], i+1, input.Defaults) {
			if strings.HasPrefix(f.Path, "/defaults/") {
				if defaultsSeen[f.Path] {
					continue
				}
				defaultsSeen[f.Path] = true
			}
			findings = append(findings, f)
		}
	}
	return findings
}

// validateSlideDesignMode returns constrained-mode violations for a single slide.
// The caller is responsible for checking that the deck is in constrained mode;
// this function always inspects the slide regardless of deck mode.
//
// defaults is the deck's defaults block (nil when it has none): a value a cell
// took from it is reported at the defaults field the author wrote.
func validateSlideDesignMode(slide *SlideInput, slideNum int, defaults *DefaultsInput) []patterns.FitFinding {
	var cellStyle *ShapeSpecInput
	if defaults != nil {
		cellStyle = defaults.CellStyle
	}
	var findings []patterns.FitFinding

	// Check shape_grid cells
	if slide.ShapeGrid != nil {
		findings = append(findings, checkShapeGrid(slide.ShapeGrid, slideNum, cellStyle)...)
	}

	// Check pattern overrides (patterns expand to shape grids, but the
	// override fields are user-specified and can contain raw colors).
	if slide.Pattern != nil {
		findings = append(findings, checkPatternInput(slide.Pattern, slideNum, slidepath.SlideField(slideNum-1, "pattern"))...)
	}

	// Check compose segments for pattern overrides.
	if slide.Compose != nil {
		findings = append(findings, checkCompose(slide.Compose, slideNum, slidepath.SlideField(slideNum-1, "compose"))...)
	}

	// Check content items (table/chart/diagram colors)
	for j := range slide.Content {
		findings = append(findings, checkContentInput(&slide.Content[j], slideNum, j+1)...)
	}

	return findings
}

// checkCompose scans a compose envelope: each segment is a pattern, a diagram
// or another envelope, and answers to the rule that thing answers to on a
// slide of its own.
func checkCompose(compose *ComposeInput, slideNum int, composePath string) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for si := range compose.Segments {
		seg := &compose.Segments[si]
		segPath := fmt.Sprintf("%s/segments/%d", composePath, si)
		findings = append(findings, checkPatternInput(&seg.Pattern, slideNum, segPath+"/pattern")...)
		findings = append(findings, checkDiagramStyle(seg.Diagram, slideNum, segPath+"/diagram")...)
		if seg.Compose != nil {
			findings = append(findings, checkCompose(seg.Compose, slideNum, segPath+"/compose")...)
		}
	}
	return findings
}

// checkShapeGrid scans a slide's ShapeGridInput for raw hex colors and absolute
// font sizes.
func checkShapeGrid(grid *ShapeGridInput, slideNum int, cellStyle *ShapeSpecInput) []patterns.FitFinding {
	return checkShapeGridAt(grid, slideNum, slidepath.ShapeGrid(slideNum-1), cellStyle, false)
}

// checkShapeGridAt scans the grid at gridPath. A sub-grid nested in a cell is
// the same grid one level down: its cells answer to the same rule, and it
// inherits the size waiver of the expansion it sits in.
func checkShapeGridAt(grid *ShapeGridInput, slideNum int, gridPath string, cellStyle *ShapeSpecInput, sizesAreEngineOwned bool) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// A grid the engine's own expander produced carries explicit sizes by
	// design; refusing them made expand_pattern output preview-only
	// (go-slide-creator-c3po).
	sizesAreEngineOwned = sizesAreEngineOwned || gridFromPatternExpander(grid.Source)

	for ri, row := range grid.Rows {
		rowPath := fmt.Sprintf("%s/rows/%d", gridPath, ri)
		if row.Connector != nil && row.Connector.Color != "" {
			if f := checkColorField(row.Connector.Color, slideNum, rowPath+"/connector/color"); f != nil {
				findings = append(findings, *f)
			}
		}

		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/cells/%d", rowPath, ci)
			for _, f := range checkGridCell(cell, slideNum, cellPath, cellStyle) {
				if sizesAreEngineOwned && isAbsoluteSizeFinding(f) {
					continue
				}
				findings = append(findings, f)
			}
			if cell.Grid != nil {
				findings = append(findings, checkShapeGridAt(cell.Grid, slideNum, cellPath+"/grid", cellStyle, sizesAreEngineOwned)...)
			}
		}
	}

	return findings
}

// patternSourcePrefix marks a shape_grid the engine expanded from a named
// pattern.
const patternSourcePrefix = "pattern:"

// gridFromPatternExpander reports whether a grid's source names a registered
// pattern. An unrecognised source waives nothing: the stamp is the engine's,
// and a value it would not have written is treated as absent.
func gridFromPatternExpander(source string) bool {
	name := strings.TrimPrefix(source, patternSourcePrefix)
	if name == source || name == "" {
		return false
	}
	_, ok := patterns.Default().Get(name)
	return ok
}

// isAbsoluteSizeFinding reports whether a design-mode finding is the
// absolute-font-size rule rather than a colour one.
func isAbsoluteSizeFinding(f patterns.FitFinding) bool {
	return f.Code == "design_mode_violation" && strings.Contains(f.Message, "absolute font size")
}

// checkGridCell validates a single grid cell for design mode violations.
func checkGridCell(cell *GridCellInput, slideNum int, cellPath string, cellStyle *ShapeSpecInput) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if cell.Shape != nil {
		findings = append(findings, checkShapeSpec(cell.Shape, slideNum, cellPath+"/shape", cellStyle)...)
	}

	if cell.AccentBar != nil && cell.AccentBar.Color != "" {
		if f := checkColorField(cell.AccentBar.Color, slideNum,
			cellPath+"/accent_bar/color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if cell.Icon != nil && cell.Icon.Fill != "" {
		if f := checkColorField(cell.Icon.Fill, slideNum,
			cellPath+"/icon/fill"); f != nil {
			findings = append(findings, *f)
		}
	}

	if cell.Image != nil {
		findings = append(findings, checkGridImage(cell.Image, slideNum, cellPath)...)
	}

	findings = append(findings, checkDiagramStyle(cell.Diagram, slideNum, cellPath+"/diagram")...)

	if cell.Table != nil {
		findings = append(findings, checkTableInput(cell.Table, slideNum, cellPath+"/table")...)
	}

	// A composite cell stacks a text shape on a sub-diagram: the two things a
	// cell otherwise holds one at a time.
	if cell.Composite != nil {
		if cell.Composite.Text != nil {
			findings = append(findings, checkShapeSpec(cell.Composite.Text, slideNum, cellPath+"/composite/text", nil)...)
		}
		findings = append(findings, checkDiagramStyle(cell.Composite.SubDiagram, slideNum, cellPath+"/composite/sub_diagram")...)
	}

	// A pattern nested in a cell takes the overrides a slide-level one does.
	if len(cell.Pattern) > 0 {
		var nested PatternInput
		if err := json.Unmarshal(cell.Pattern, &nested); err == nil {
			findings = append(findings, checkPatternInput(&nested, slideNum, cellPath+"/pattern")...)
		}
	}

	return findings
}

// checkDiagramStyle checks a diagram's style colors and background, wherever
// the diagram sits (a placeholder, a grid cell, a composite cell, a compose
// segment).
func checkDiagramStyle(diagram *types.DiagramSpec, slideNum int, diagramPath string) []patterns.FitFinding {
	if diagram == nil || diagram.Style == nil {
		return nil
	}
	return checkStyleColors(diagram.Style.Colors, diagram.Style.Background, slideNum, diagramPath+"/style")
}

// checkStyleColors checks the series colors and background of the chart or
// diagram style object at stylePath.
func checkStyleColors(colors []string, background string, slideNum int, stylePath string) []patterns.FitFinding {
	findings := checkDiagramStyleColors(colors, slideNum, stylePath+"/colors")
	if background != "" {
		if f := checkColorField(background, slideNum, stylePath+"/background"); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// checkGridImage validates image overlay and text colors.
func checkGridImage(img *GridImageInput, slideNum int, cellPath string) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if img.Overlay != nil && img.Overlay.Color != "" {
		if f := checkColorField(img.Overlay.Color, slideNum,
			cellPath+"/image/overlay/color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if img.Text != nil && img.Text.Color != "" {
		if f := checkColorField(img.Text.Color, slideNum,
			cellPath+"/image/text/color"); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

// checkShapeSpec validates fill, line, and text color fields in the
// ShapeSpecInput at shapePath.
func checkShapeSpec(spec *ShapeSpecInput, slideNum int, shapePath string, cellStyle *ShapeSpecInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	// A field the cell took from defaults.cell_style is addressed there.
	fieldPath := func(field string, value, fromDefaults json.RawMessage) string {
		if sameRawMessage(value, fromDefaults) {
			return "/defaults/cell_style/" + field
		}
		return shapePath + "/" + field
	}
	var defFill, defLine, defText json.RawMessage
	if cellStyle != nil {
		defFill, defLine, defText = cellStyle.Fill, cellStyle.Line, cellStyle.Text
	}

	// Check fill
	if len(spec.Fill) > 0 {
		if f := checkRawMessageColor(spec.Fill, slideNum, fieldPath("fill", spec.Fill, defFill)); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check line
	if len(spec.Line) > 0 {
		if f := checkRawMessageColor(spec.Line, slideNum, fieldPath("line", spec.Line, defLine)); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check text color and font size
	if len(spec.Text) > 0 {
		findings = append(findings, checkTextRaw(spec.Text, slideNum, fieldPath("text", spec.Text, defText))...)
	}

	return findings
}

// sameRawMessage reports whether a is the very value b: applyCellStyleDefaults
// hands a cell the default's own bytes, so identity tells a value the cell
// adopted from one it wrote itself, even when the two spell the same colour.
func sameRawMessage(a, b json.RawMessage) bool {
	return len(a) > 0 && len(a) == len(b) && &a[0] == &b[0]
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
		if f := checkColorField(obj.Color, slideNum, path+"/color"); f != nil {
			findings = append(findings, *f)
		}
	}

	if obj.Size > 0 {
		if f := checkAbsoluteSize(obj.Size, slideNum, path+"/size"); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check paragraphs array
	for i, p := range obj.Paragraphs {
		pPath := fmt.Sprintf("%s/paragraphs/%d", path, i)
		if p.Color != "" {
			if f := checkColorField(p.Color, slideNum, pPath+"/color"); f != nil {
				findings = append(findings, *f)
			}
		}
		if p.Size > 0 {
			if f := checkAbsoluteSize(p.Size, slideNum, pPath+"/size"); f != nil {
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
		return checkColorField(obj.Color, slideNum, path+"/color")
	}

	return nil
}

// isRawHexColor reports whether color is a raw hex value (with or without a
// leading "#") rather than an allowed scheme-color name or sentinel. This is the
// shared predicate behind both the constrained-mode refusal (checkColorField)
// and the diagram-data drop warning (collectDroppedDiagramColorWarnings).
func isRawHexColor(color string) bool {
	if color == "" || color == "none" {
		return false
	}
	if pptx.IsSchemeColor(color) {
		return false
	}
	return isHexString(strings.TrimPrefix(color, "#"))
}

// checkColorField returns a fit finding if the color is a raw hex value.
func checkColorField(color string, slideNum int, path string) *patterns.FitFinding {
	if !isRawHexColor(color) {
		return nil
	}

	hex := strings.TrimPrefix(color, "#")
	nearest := suggestNearestSchemeColor(hex)

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "design_mode",
			Path:    path,
			Code:    "design_mode_violation",
			Message: fmt.Sprintf("slide %d: %s uses raw hex color %q — constrained mode requires scheme colors (accent1-6, dk1, dk2, lt1, lt2, etc.)", slideNum, path, color),
			Fix: &patterns.FixSuggestion{
				Kind:   "use_semantic_color",
				Params: map[string]any{"path": path, "value": nearest},
			},
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
			Fix:     patterns.RemoveFieldFix(path),
		},
		Action: "refuse",
	}
}

// checkPatternInput checks a pattern's override fields for raw hex colors.
func checkPatternInput(pattern *PatternInput, slideNum int, patternPath string) []patterns.FitFinding {
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
		path := patternPath + "/overrides/" + escapePointerSegment(key)
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

// checkContentInput checks a content item for raw hex colors: the styling of
// a table, a chart or a diagram.
func checkContentInput(ci *ContentInput, slideNum, contentNum int) []patterns.FitFinding {
	var findings []patterns.FitFinding
	basePath := slidepath.ContentIndex(slideNum-1, contentNum-1)

	// A table answers to one rule wherever it sits: this is the table a grid
	// cell holds, in a placeholder (go-slide-creator-gpbjx).
	if ci.TableValue != nil {
		findings = append(findings, checkTableInput(ci.TableValue, slideNum, basePath+"/table_value")...)
	}
	findings = append(findings, checkDiagramStyle(ci.DiagramValue, slideNum, basePath+"/diagram_value")...)
	// chart_value is deprecated but still used.
	if ci.ChartValue != nil && ci.ChartValue.Style != nil {
		findings = append(findings, checkStyleColors(ci.ChartValue.Style.Colors, ci.ChartValue.Style.Background, slideNum, basePath+"/chart_value/style")...)
	}
	return append(findings, checkLegacyContentValue(ci, slideNum, basePath+"/value")...)
}

// checkLegacyContentValue checks the legacy "value" spelling of a table, chart
// or diagram: the same payload under another key, read only when the typed
// field is absent.
func checkLegacyContentValue(ci *ContentInput, slideNum int, valuePath string) []patterns.FitFinding {
	if len(ci.Value) == 0 {
		return nil
	}
	switch {
	case ci.Type == "table" && ci.TableValue == nil:
		var table TableInput
		if err := json.Unmarshal(ci.Value, &table); err == nil {
			return checkTableInput(&table, slideNum, valuePath)
		}
	case ci.Type == "chart" && ci.ChartValue == nil, ci.Type == "diagram" && ci.DiagramValue == nil:
		var visual struct {
			Style *struct {
				Colors     []string `json:"colors"`
				Background string   `json:"background"`
			} `json:"style"`
		}
		if err := json.Unmarshal(ci.Value, &visual); err == nil && visual.Style != nil {
			return checkStyleColors(visual.Style.Colors, visual.Style.Background, slideNum, valuePath+"/style")
		}
	}
	return nil
}

// collectDroppedDiagramColorWarnings returns advisory (info) findings for
// constrained-mode decks whose diagram data payloads embed raw hex colors in
// per-item fields (e.g. pyramid levels[].color). Unlike the documented
// diagram_value.style.colors surface — which the constrained-mode validator
// refuses outright — these data-embedded colors are not part of the validated
// override surface, so the engine silently renders them with the template scheme
// instead of aborting. This collector turns that silent drop into a visible
// signal that tells the author design_mode "free" is required to honor the
// custom colors. It never blocks generation (action "info").
//
// The caller is responsible for only invoking this in constrained mode; it is a
// no-op for free decks because raw colors pass through there.
func collectDroppedDiagramColorWarnings(input *PresentationInput) []patterns.FitFinding {
	if effectiveDesignMode(input) != designModeConstrained {
		return nil
	}

	var findings []patterns.FitFinding
	for i := range input.Slides {
		slide := &input.Slides[i]
		slideNum := i + 1
		for j := range slide.Content {
			ci := &slide.Content[j]
			if ci.DiagramValue == nil || len(ci.DiagramValue.Data) == 0 {
				continue
			}
			if rawColors := rawHexColorsInData(ci.DiagramValue.Data); len(rawColors) > 0 {
				basePath := slidepath.ContentIndex(slideNum-1, j) + "/diagram_value/data"
				findings = append(findings, droppedDiagramColorFinding(slideNum, basePath, ci.DiagramValue.Type, rawColors))
			}
		}
	}
	return findings
}

// droppedDiagramColorFinding builds the CUSTOM_COLOR_DROPPED info finding for a
// single diagram whose data embeds raw hex colors that constrained mode ignores.
func droppedDiagramColorFinding(slideNum int, path, diagramType string, rawColors []string) patterns.FitFinding {
	diagramLabel := diagramType
	if diagramLabel == "" {
		diagramLabel = "diagram"
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "design_mode",
			Path:    path,
			Code:    patterns.ErrCodeCustomColorDropped,
			Message: fmt.Sprintf("slide %d: %s data embeds custom color(s) %s — constrained mode (default) renders with the template scheme and ignores them; rerun with design_mode \"free\" to honor custom colors",
				slideNum, diagramLabel, strings.Join(rawColors, ", ")),
			Fix: &patterns.FixSuggestion{
				Kind:   "set_design_mode_free",
				Params: map[string]any{"path": path, "dropped_colors": rawColors},
			},
		},
		Action: "info",
	}
}

// rawHexColorsInData walks a decoded diagram data payload and returns the
// distinct raw hex color values found under color-like keys (e.g. "color",
// "fill"). Scheme-color names and non-color values are ignored. Insertion order
// is preserved (first occurrence wins) so messages are deterministic.
func rawHexColorsInData(data map[string]any) []string {
	seen := map[string]bool{}
	var ordered []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if s, ok := val.(string); ok {
					if isColorKey(k) && isRawHexColor(s) && !seen[s] {
						seen[s] = true
						ordered = append(ordered, s)
					}
					continue
				}
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(data)
	return ordered
}

// checkDiagramStyleColors checks a slice of color strings for raw hex values.
func checkDiagramStyleColors(colors []string, slideNum int, basePath string) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for i, c := range colors {
		path := fmt.Sprintf("%s/%d", basePath, i)
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
		path := basePath + "/style/header_background"
		if table.Style.HeaderBackgroundFromDefaults {
			path = "/defaults/table_style/header_background"
		}
		if f := checkColorField(*table.Style.HeaderBackground, slideNum, path); f != nil {
			findings = append(findings, *f)
		}
	}

	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Conditional != nil && cell.Conditional.Fill != "" {
				path := fmt.Sprintf("%s/rows/%d/%d/conditional/fill", basePath, ri, ci)
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
		name    string
		r, g, b uint8
	}

	// Common accent palette approximations (midtones)
	candidates := []candidate{
		{"accent1", 68, 114, 196},  // blue
		{"accent2", 237, 125, 49},  // orange
		{"accent3", 165, 165, 165}, // gray
		{"accent4", 255, 192, 0},   // gold
		{"accent5", 91, 155, 213},  // light blue
		{"accent6", 112, 173, 71},  // green
		{"dk1", 0, 0, 0},           // black
		{"dk2", 68, 84, 106},       // dark gray-blue
		{"lt1", 255, 255, 255},     // white
		{"lt2", 228, 230, 232},     // light gray
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

// designModeFreeSuggestion is the "re-submit with design_mode: free" hop.
//
// design_mode is a field of the PRESENTATION, not a tool argument: the
// suggestion used to read {tool: generate_presentation, args_template:
// {design_mode: "free"}}, and following it verbatim was rejected with
// UNKNOWN_PARAMETER — while the word design_mode appeared nowhere in tools/list
// or get_capabilities, so the only way out was to guess where it belonged
// (go-slide-creator-g0er).
func designModeFreeSuggestion() *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool: "generate_presentation",
		ArgsTemplate: map[string]any{
			"presentation": map[string]any{
				"design_mode": "free",
				"slides":      "<your existing slides, unchanged>",
			},
		},
	}
}

// designModeDiagnostics converts design-mode FitFindings into Diagnostics with
// a next_tool_call hint that tells agents to re-submit with design_mode:"free"
// if the raw values are intentional.
func designModeDiagnostics(violations []patterns.FitFinding) []diagnostics.Diagnostic {
	diags := make([]diagnostics.Diagnostic, 0, len(violations))
	for _, v := range violations {
		d := diagnostics.Diagnostic{
			Code:         "design_mode_violation",
			Path:         v.Path,
			Message:      v.Message,
			Severity:     diagnostics.SeverityError,
			NextToolCall: designModeFreeSuggestion(),
		}
		if v.Fix != nil {
			d.Fix = &diagnostics.Fix{Kind: v.Fix.Kind, Params: v.Fix.Params}
		}
		diags = append(diags, d)
	}
	return diags
}

// droppedDiagramColorDiagnostics converts CUSTOM_COLOR_DROPPED info findings into
// warning-severity Diagnostics for the MCP boundary. These are advisory — they
// flow through alongside errors without blocking generation — and carry the same
// design_mode:"free" next_tool_call hint as the hard refusals so an agent can
// opt into honoring the custom colors.
func droppedDiagramColorDiagnostics(findings []patterns.FitFinding) []diagnostics.Diagnostic {
	diags := make([]diagnostics.Diagnostic, 0, len(findings))
	for _, f := range findings {
		d := diagnostics.Diagnostic{
			Code:         f.Code,
			Path:         f.Path,
			Message:      f.Message,
			Severity:     diagnostics.SeverityWarning,
			NextToolCall: designModeFreeSuggestion(),
		}
		if f.Fix != nil {
			d.Fix = &diagnostics.Fix{Kind: f.Fix.Kind, Params: f.Fix.Params}
		}
		diags = append(diags, d)
	}
	return diags
}
