package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// dryRunOutput is the top-level JSON printed to stdout in dry-run mode.
type dryRunOutput struct {
	Valid        bool           `json:"valid"`
	SlideCount   int            `json:"slide_count"`
	ChartCount   int            `json:"chart_count"`
	DiagramCount int            `json:"diagram_count"`
	TableCount   int            `json:"table_count"`
	ShapeCount   int            `json:"shape_count"`
	Warnings           []string                    `json:"warnings"`
	ValidationWarnings []*patterns.ValidationError  `json:"validation_warnings,omitempty"`
	Errors             []string                    `json:"errors,omitempty"`
	Slides             []dryRunSlide               `json:"slides"`
	FitFindings        []fitFinding                `json:"fit_findings,omitempty"`
}

// dryRunSlide describes one slide in the dry-run report.
type dryRunSlide struct {
	SlideNumber  int                    `json:"slide_number"`
	Title        string                 `json:"title,omitempty"`
	LayoutID     string                 `json:"layout_id"`
	LayoutName   string                 `json:"layout_name,omitempty"`
	Placeholders []dryRunPlaceholder    `json:"placeholders"`
	ShapeCount   int                    `json:"shape_count,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
}

// dryRunPlaceholder describes a placeholder mapping in the dry-run report.
type dryRunPlaceholder struct {
	PlaceholderID string `json:"placeholder_id"`
	ContentType   string `json:"content_type"`
	MaxChars      int    `json:"max_chars,omitempty"`
	Truncated     bool   `json:"truncated,omitempty"`
	TruncateAt    int    `json:"truncate_at,omitempty"`
}

// runJSONDryRun validates JSON input against the template without generating
// a PPTX file. It checks layout_id references, placeholder_id references,
// and content types.
func runJSONDryRun(jsonPath, templatesDir, configPath string) error {
	output := dryRunOutput{
		Valid:    true,
		Warnings: []string{},
		Slides:   []dryRunSlide{},
	}

	// Read JSON input
	var inputData []byte
	var err error
	if jsonPath == "-" {
		inputData, err = io.ReadAll(os.Stdin)
	} else {
		inputData, err = os.ReadFile(jsonPath)
	}
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, fmt.Sprintf("failed to read JSON input: %v", err))
		return writeDryRunOutput(output)
	}

	// Parse JSON as PresentationInput (superset of legacy JSONInput)
	var input PresentationInput
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(inputData, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			output.Valid = false
			output.Errors = append(output.Errors, fmt.Sprintf("failed to apply patch: %v", patchErr))
			return writeDryRunOutput(output)
		}
		input = *patched
	} else {
		if err := json.Unmarshal(inputData, &input); err != nil {
			output.Valid = false
			output.Errors = append(output.Errors, fmt.Sprintf("failed to parse JSON: %v", err))
			return writeDryRunOutput(output)
		}
	}
	applyDefaults(&input)

	// Validate required fields
	if input.Template == "" {
		output.Valid = false
		output.Errors = append(output.Errors, "template is required in JSON input")
	}
	if len(input.Slides) == 0 {
		output.Valid = false
		output.Errors = append(output.Errors, "at least one slide is required")
	}
	if !output.Valid {
		return writeDryRunOutput(output)
	}

	// Load configuration
	cfg := config.DefaultConfig()
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			output.Valid = false
			output.Errors = append(output.Errors, fmt.Sprintf("failed to load config: %v", err))
			return writeDryRunOutput(output)
		}
	}
	if templatesDir != "" {
		cfg.Templates.Dir = templatesDir
	}

	// Resolve template for validation
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, cfg.Templates.Dir)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, templateNotFoundError(input.Template, cfg.Templates.Dir))
		return writeDryRunOutput(output)
	}
	defer templateCleanup()

	cache := template.NewMemoryCache(24 * time.Hour)
	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, fmt.Sprintf("template analysis failed: %v", err))
		return writeDryRunOutput(output)
	}

	// Validate slides against template
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)

	return writeDryRunOutput(output)
}

// validateJSONContentValue checks that a JSON content item's value can be
// parsed according to its declared type. Returns error/warning messages.
func validateJSONContentValue(item JSONContentItem, slideNum, contentNum int) string {
	switch item.Type {
	case "text":
		var text string
		if err := json.Unmarshal(item.Value, &text); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid text value: %v", slideNum, contentNum, err)
		}
	case "bullets":
		var bullets []string
		if err := json.Unmarshal(item.Value, &bullets); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid bullets value (expected array of strings): %v", slideNum, contentNum, err)
		}
	case "image":
		var img struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(item.Value, &img); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid image value: %v", slideNum, contentNum, err)
		}
		if img.Path == "" {
			return fmt.Sprintf("slide %d, content %d: image path is required", slideNum, contentNum)
		}
	case "chart":
		var chart types.ChartSpec                                    //nolint:staticcheck // backward compatibility
		if err := json.Unmarshal(item.Value, &chart); err != nil { //nolint:staticcheck // backward compatibility
			return fmt.Sprintf("slide %d, content %d: invalid chart value: %v", slideNum, contentNum, err)
		}
		if chart.Type == "" {
			return fmt.Sprintf("slide %d, content %d: chart type is required", slideNum, contentNum)
		}
		// Validate chart data structure via svggen
		spec := chart.ToDiagramSpec()
		if w := validateDiagramSpec(spec, slideNum, contentNum); w != "" {
			return w
		}
	case "body_and_bullets":
		var bab struct {
			Body    string   `json:"body"`
			Bullets []string `json:"bullets"`
		}
		if err := json.Unmarshal(item.Value, &bab); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid body_and_bullets value: %v", slideNum, contentNum, err)
		}
	case "bullet_groups":
		var bg struct {
			Groups []struct {
				Bullets []string `json:"bullets"`
			} `json:"groups"`
		}
		if err := json.Unmarshal(item.Value, &bg); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid bullet_groups value: %v", slideNum, contentNum, err)
		}
		if len(bg.Groups) == 0 {
			return fmt.Sprintf("slide %d, content %d: bullet_groups must have at least one group", slideNum, contentNum)
		}
	case "table":
		var table struct {
			Headers []string          `json:"headers"`
			Rows    []json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(item.Value, &table); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid table value: %v", slideNum, contentNum, err)
		}
	case "diagram":
		var diagram types.DiagramSpec
		if err := json.Unmarshal(item.Value, &diagram); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid diagram value: %v", slideNum, contentNum, err)
		}
		if diagram.Type == "" {
			return fmt.Sprintf("slide %d, content %d: diagram type is required", slideNum, contentNum)
		}
		// Validate diagram data structure via svggen
		if w := validateDiagramSpec(&diagram, slideNum, contentNum); w != "" {
			return w
		}
	}
	return ""
}

// validateSlidesAgainstTemplate validates the slides in a PresentationInput
// against a resolved TemplateAnalysis, populating the dryRunOutput with
// per-slide details, warnings, and errors. This is the shared validation
// core used by both the CLI dry-run and the MCP validate_input handler.
func validateSlidesAgainstTemplate(output *dryRunOutput, slides []SlideInput, analysis *types.TemplateAnalysis) { //nolint:gocognit,gocyclo
	// Build layout and placeholder lookup maps from template analysis
	layoutByID := make(map[string]types.LayoutMetadata, len(analysis.Layouts))
	for _, l := range analysis.Layouts {
		layoutByID[l.ID] = l
	}

	type phKey struct{ layoutID, phID string }
	phByKey := make(map[phKey]types.PlaceholderInfo)
	for _, l := range analysis.Layouts {
		for _, ph := range l.Placeholders {
			phByKey[phKey{l.ID, ph.ID}] = ph
		}
	}

	output.SlideCount = len(slides)

	// Validate each slide
	for i, slideInput := range slides {
		slide := dryRunSlide{
			SlideNumber:  i + 1,
			LayoutID:     slideInput.LayoutID,
			Placeholders: []dryRunPlaceholder{},
		}

		// Check layout_id reference
		lm, layoutFound := layoutByID[slideInput.LayoutID]
		if !layoutFound {
			output.Warnings = append(output.Warnings,
				fmt.Sprintf("slide %d: layout_id %q not found in template", i+1, slideInput.LayoutID))
		} else {
			slide.LayoutName = lm.Name
		}

		if slideInput.LayoutID == "" {
			output.Valid = false
			output.Errors = append(output.Errors,
				fmt.Sprintf("slide %d: layout_id is required", i+1))
		}

		// Validate content items using ContentInput.ResolveValue() for typed + legacy support
		for j, item := range slideInput.Content {
			ph := dryRunPlaceholder{
				PlaceholderID: item.PlaceholderID,
				ContentType:   item.Type,
			}

			if item.PlaceholderID == "" {
				output.Valid = false
				output.Errors = append(output.Errors,
					fmt.Sprintf("slide %d, content %d: placeholder_id is required", i+1, j+1))
			} else if layoutFound {
				// Check placeholder_id reference against layout
				phInfo, phFound := phByKey[phKey{slideInput.LayoutID, item.PlaceholderID}]
				if !phFound {
					output.Warnings = append(output.Warnings,
						fmt.Sprintf("slide %d, content %d: placeholder_id %q not found in layout %q",
							i+1, j+1, item.PlaceholderID, slideInput.LayoutID))
				} else {
					ph.MaxChars = phInfo.MaxChars

					// Check character limits for text content
					if item.Type == "text" && phInfo.MaxChars > 0 {
						resolved, _ := item.ResolveValue()
						if text, ok := resolved.(string); ok && len(text) > phInfo.MaxChars {
							ph.Truncated = true
							ph.TruncateAt = phInfo.MaxChars
							output.Warnings = append(output.Warnings,
								fmt.Sprintf("slide %d, content %d: text (%d chars) exceeds placeholder %q limit (%d chars)",
									i+1, j+1, len(text), item.PlaceholderID, phInfo.MaxChars))
						}
					}
				}
			}

			// Validate content type
			if item.Type == "" {
				output.Valid = false
				output.Errors = append(output.Errors,
					fmt.Sprintf("slide %d, content %d: type is required", i+1, j+1))
			} else {
				switch item.Type {
				case "text", "bullets", "body_and_bullets", "bullet_groups", "table", "image", "chart", "diagram":
					// valid types
				default:
					output.Valid = false
					output.Errors = append(output.Errors,
						fmt.Sprintf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)",
							i+1, j+1, item.Type))
				}
				// Count content types
				switch item.Type {
				case "chart":
					output.ChartCount++
				case "diagram":
					output.DiagramCount++
				case "table":
					output.TableCount++
					// Density check for content-level table.
					table := resolveTableFromContent(&item)
					if table != nil {
						tablePath := fmt.Sprintf("slides[%d].content[%d]", i, j)
						output.ValidationWarnings = append(output.ValidationWarnings, pipeline.DetectTableDensity(table, tablePath)...)

						// Warn when both header_background and style_id are explicitly authored.
						if table.Style != nil {
							if w := generator.WarnStyleCollision(i,
								table.Style.HeaderBackground != nil,
								table.Style.StyleID != "",
							); w != "" {
								output.ValidationWarnings = append(output.ValidationWarnings, &patterns.ValidationError{
									Path:    tablePath,
									Code:    "style_collision",
									Message: w,
								})
							}
						}
					}
				}
			}

			// Validate content value is parseable using ResolveValue
			if item.Type != "" {
				if _, err := item.ResolveValue(); err != nil {
					output.Valid = false
					output.Errors = append(output.Errors,
						fmt.Sprintf("slide %d, content %d: %v", i+1, j+1, err))
				}
			}

			// Validate chart/diagram data structure via svggen
			if item.Type == "chart" || item.Type == "diagram" {
				output.Warnings = append(output.Warnings, validateContentDiagramData(item, i+1, j+1)...)
			}

			slide.Placeholders = append(slide.Placeholders, ph)
		}

		// Validate shape_grid if present
		if slideInput.ShapeGrid != nil {
			gridCounts, gridWarnings, gridErrors, gridValWarnings := validateShapeGrid(slideInput.ShapeGrid, i+1)
			slide.ShapeCount = gridCounts.Shapes
			output.ShapeCount += gridCounts.Shapes
			output.TableCount += gridCounts.Tables
			output.DiagramCount += gridCounts.Diagrams
			output.Warnings = append(output.Warnings, gridWarnings...)
			output.ValidationWarnings = append(output.ValidationWarnings, gridValWarnings...)
			if len(gridErrors) > 0 {
				output.Valid = false
				output.Errors = append(output.Errors, gridErrors...)
			}
		}

		output.Slides = append(output.Slides, slide)
	}
}

// hexColorRe matches #RGB or #RRGGBB hex color strings.
var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// brandHexAllowlist contains lowercase hex values that are deliberately used as
// raw colors (black, white) and should NOT trigger the hex_fill_non_brand
// warning. Lookups must lowercase the input first.
var brandHexAllowlist = map[string]bool{
	"#000000": true, "#000": true,
	"#ffffff": true, "#fff": true,
}

// isAllowlistedHex reports whether a hex color string is in the brand allowlist.
func isAllowlistedHex(s string) bool {
	return brandHexAllowlist[strings.ToLower(s)]
}

// schemeColorNames aliases the canonical set from the pptx package, plus "none".
var schemeColorNames = func() map[string]bool {
	m := make(map[string]bool, len(pptx.SchemeColorNames)+1)
	for k, v := range pptx.SchemeColorNames {
		m[k] = v
	}
	m["none"] = true
	return m
}()

// isValidFillColor checks whether a color string is a valid hex color or scheme color name.
func isValidFillColor(s string) bool {
	return hexColorRe.MatchString(s) || schemeColorNames[s]
}

// gridContentCounts holds counts of content types found inside shape_grid cells.
type gridContentCounts struct {
	Shapes   int
	Tables   int
	Diagrams int
}

// validateShapeGrid validates a ShapeGridInput and returns content counts,
// warnings, errors, and structured validation warnings.
func validateShapeGrid(grid *ShapeGridInput, slideNum int) (counts gridContentCounts, warnings []string, errors []string, valWarnings []*patterns.ValidationError) {
	if len(grid.Rows) == 0 {
		errors = append(errors, fmt.Sprintf("slide %d: shape_grid has no rows", slideNum))
		return
	}

	// Validate columns
	if len(grid.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(grid.Columns, &n); err != nil {
			var arr []float64
			if err := json.Unmarshal(grid.Columns, &arr); err != nil {
				errors = append(errors, fmt.Sprintf("slide %d: shape_grid columns must be a number or array of numbers", slideNum))
			}
		}
	}

	for rowIdx, row := range grid.Rows {
		for cellIdx, cell := range row.Cells {
			if cell == nil {
				continue
			}
			if cell.Shape != nil {
				counts.Shapes++
				// Validate geometry name
				if cell.Shape.Geometry == "" {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: geometry is required", slideNum, rowIdx+1, cellIdx+1))
				} else if !pptx.IsKnownGeometry(cell.Shape.Geometry) {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: unknown geometry %q", slideNum, rowIdx+1, cellIdx+1, cell.Shape.Geometry))
				}
				// Validate fill color
				vw := validateShapeFillColor(cell.Shape.Fill, slideNum, rowIdx+1, cellIdx+1, &warnings)
				valWarnings = append(valWarnings, vw...)
			}
			if cell.Table != nil {
				counts.Shapes++
				counts.Tables++
				// Density check for embedded table.
				tablePath := fmt.Sprintf("slides[%d].shape_grid.rows[%d].cells[%d].table", slideNum-1, rowIdx, cellIdx)
				valWarnings = append(valWarnings, pipeline.DetectTableDensity(cell.Table, tablePath)...)
			}
			if cell.Diagram != nil {
				counts.Shapes++
				counts.Diagrams++
				if cell.Diagram.Type == "" {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: diagram type is required", slideNum, rowIdx+1, cellIdx+1))
				}
			}
		}
	}

	return
}

// validateShapeFillColor checks that a shape fill value has a valid color format.
// It also returns structured warnings for hex colors not in the brand allowlist.
func validateShapeFillColor(raw json.RawMessage, slideNum, row, cell int, warnings *[]string) []*patterns.ValidationError {
	if len(raw) == 0 {
		return nil
	}
	var valWarnings []*patterns.ValidationError
	checkHex := func(color string) {
		if hexColorRe.MatchString(color) && !isAllowlistedHex(color) {
			path := fmt.Sprintf("slides[%d].shape_grid.rows[%d].cells[%d].shape.fill", slideNum-1, row-1, cell-1)
			valWarnings = append(valWarnings, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeHexFillNonBrand,
				Message: fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q is a raw hex value; prefer a scheme color for template portability", slideNum, row, cell, color),
				Fix:     &patterns.FixSuggestion{Kind: "use_semantic_color", Params: map[string]any{"message": "use accent1/accent2/lt2/dk1 instead"}},
			})
		}
	}
	// Try string form
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s != "" && !isValidFillColor(s) {
			*warnings = append(*warnings, fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q should be #RGB, #RRGGBB, or a scheme color name (e.g. accent1, dk1)", slideNum, row, cell, s))
		}
		checkHex(s)
		return valWarnings
	}
	// Try object form
	var obj ShapeFillInput
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Color != "" && !isValidFillColor(obj.Color) {
			*warnings = append(*warnings, fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q should be #RGB, #RRGGBB, or a scheme color name (e.g. accent1, dk1)", slideNum, row, cell, obj.Color))
		}
		checkHex(obj.Color)
	}
	return valWarnings
}

// writeDryRunOutput writes the dry-run result as JSON to stdout.
// Returns nil on valid output, or an error to signal exit code 1 for invalid.
func writeDryRunOutput(output dryRunOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("failed to encode dry-run output: %w", err)
	}

	if !output.Valid {
		return fmt.Errorf("dry-run validation failed")
	}
	return nil
}
