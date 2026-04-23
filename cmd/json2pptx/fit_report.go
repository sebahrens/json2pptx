package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// fitFinding is a single fit-report entry written as NDJSON. It extends
// ValidationError with measurement-specific fields for agent consumption.
type fitFinding struct {
	Code             string                  `json:"code"`
	Path             string                  `json:"path"`
	Message          string                  `json:"message"`
	Fix              *patterns.FixSuggestion `json:"fix,omitempty"`
	BindingDimension string                  `json:"binding_dimension,omitempty"`
	RequiredPt       float64                 `json:"required_pt,omitempty"`
	AllocatedPt      float64                 `json:"allocated_pt,omitempty"`
	WrapLines        int                     `json:"wrap_lines,omitempty"`
	Action           string                  `json:"action,omitempty"`
}

// generateFitReport walks all tables and shape-grid text cells in the
// presentation, measuring text against available cell dimensions. It returns
// findings for cells that overflow.
func generateFitReport(input *PresentationInput) []fitFinding {
	var findings []fitFinding

	for si, slide := range input.Slides {
		// Walk content-level tables.
		for ci, content := range slide.Content {
			if content.Type != "table" {
				continue
			}
			table := resolveTableFromContent(&content)
			if table == nil {
				continue
			}
			findings = append(findings,
				measureTable(table, fmt.Sprintf("slides[%d].content[%d]", si, ci), si)...)
		}

		// Walk shape_grid cells.
		if slide.ShapeGrid != nil {
			findings = append(findings,
				walkShapeGrid(slide.ShapeGrid, si)...)
		}
	}

	return findings
}

// resolveTableFromContent extracts a TableInput from a ContentInput.
func resolveTableFromContent(c *ContentInput) *jsonschema.TableInput {
	if c.TableValue != nil {
		return c.TableValue
	}
	// Try legacy Value field.
	if len(c.Value) > 0 {
		var t jsonschema.TableInput
		if json.Unmarshal(c.Value, &t) == nil && len(t.Headers) > 0 {
			return &t
		}
	}
	return nil
}

// measureTable measures all cells in a table and returns findings for overflow.
func measureTable(table *jsonschema.TableInput, pathPrefix string, slideIdx int) []fitFinding {
	if len(table.Headers) == 0 {
		return nil
	}

	numCols := len(table.Headers)
	numRows := len(table.Rows) + 1 // +1 for header

	// Compute effective font size using the same logic as generator/table.go.
	fontSize := generator.DefaultTableFontSize
	if numCols > 4 {
		scale := 4.0 / float64(numCols)
		scaled := int(float64(fontSize) * scale)
		if scaled < generator.MinTableFontSize {
			scaled = generator.MinTableFontSize
		}
		fontSize = scaled
	}
	fontPt := float64(fontSize) / 100.0

	// Estimate cell width from default slide width and equal column split.
	slideWidthEMU := int64(shapegrid.DefaultSlideWidthEMU)
	// Typical table occupies ~90% of slide width (body placeholder).
	tableWidthEMU := int64(float64(slideWidthEMU) * 0.9)
	colWidthEMU := tableWidthEMU / int64(numCols)

	// Row height: use the generator's default row height for max lines calc.
	const defaultRowHeightEMU int64 = 370840
	const defaultLineSpacing = 1.2
	lineHeightPt := fontPt * defaultLineSpacing
	maxLines := int(float64(defaultRowHeightEMU) / (lineHeightPt * 12700)) // 12700 EMU per pt
	if maxLines < 1 {
		maxLines = 1
	}

	var findings []fitFinding

	// Measure header cells.
	for hi, header := range table.Headers {
		m := textfit.MeasureRun(header, "Calibri", fontPt, colWidthEMU, maxLines)
		if !m.Fits {
			findings = append(findings, fitFinding{
				Code:             patterns.ErrCodeFitOverflow,
				Path:             fmt.Sprintf("%s.headers[%d]", pathPrefix, hi),
				Message:          fmt.Sprintf("header %q needs %d lines @ %.0fpt; cell allows %d", header, m.Lines, fontPt, maxLines),
				Fix:              &patterns.FixSuggestion{Kind: "reduce_text"},
				BindingDimension: "height",
				RequiredPt:       float64(m.RequiredEMU) / 12700.0,
				AllocatedPt:      float64(defaultRowHeightEMU) / 12700.0,
				WrapLines:        m.Lines,
				Action:           "unfittable",
			})
		}
	}

	// Measure data cells.
	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Content == "" {
				continue
			}
			m := textfit.MeasureRun(cell.Content, "Calibri", fontPt, colWidthEMU, maxLines)
			if !m.Fits {
				findings = append(findings, fitFinding{
					Code:             patterns.ErrCodeFitOverflow,
					Path:             fmt.Sprintf("%s.rows[%d][%d]", pathPrefix, ri, ci),
					Message:          fmt.Sprintf("text needs %d lines @ %.0fpt; cell allows %d", m.Lines, fontPt, maxLines),
					Fix:              &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": ri + numRows/2}},
					BindingDimension: "height",
					RequiredPt:       float64(m.RequiredEMU) / 12700.0,
					AllocatedPt:      float64(defaultRowHeightEMU) / 12700.0,
					WrapLines:        m.Lines,
					Action:           "unfittable",
				})
			}
		}
	}

	// Density check: rows*cols vs TDR ceiling.
	totalCells := numRows * numCols
	tdrCeiling := tdrCeilingForFont(fontPt)
	if totalCells > tdrCeiling {
		findings = append(findings, fitFinding{
			Code:    patterns.ErrCodeDensityExceeded,
			Path:    pathPrefix,
			Message: fmt.Sprintf("table has %d cells (%d rows × %d cols) at %.0fpt; TDR ceiling is %d", totalCells, numRows, numCols, fontPt, tdrCeiling),
			Fix:     &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": numRows / 2}},
			Action:  "unfittable",
		})
	}

	// Schema-level density check (rows ≤ 7, cols ≤ 6 with multiline counting).
	for _, ve := range pipeline.DetectTableDensity(table, pathPrefix) {
		findings = append(findings, fitFinding{
			Code:    ve.Code,
			Path:    ve.Path,
			Message: ve.Message,
			Fix:     ve.Fix,
			Action:  "warning",
		})
	}

	return findings
}

// tdrCeilingForFont returns the maximum number of table cells that can fit
// on a standard 16:9 slide at the given font size. This is a heuristic
// based on typical slide dimensions and row heights.
func tdrCeilingForFont(fontPt float64) int {
	switch {
	case fontPt >= 18:
		return 60 // ~10 rows × 6 cols
	case fontPt >= 14:
		return 80
	case fontPt >= 12:
		return 100
	default:
		return 120 // 10pt floor
	}
}

// walkShapeGrid walks all cells in a shape grid and measures text content.
func walkShapeGrid(grid *ShapeGridInput, slideIdx int) []fitFinding {
	var findings []fitFinding

	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			pathPrefix := fmt.Sprintf("slides[%d].shape_grid.rows[%d].cells[%d]", slideIdx, ri, ci)

			// Embedded table in shape_grid cell.
			if cell.Table != nil {
				findings = append(findings,
					measureTable(cell.Table, pathPrefix+".table", slideIdx)...)
			}

			// Shape with text.
			if cell.Shape != nil && len(cell.Shape.Text) > 0 {
				findings = append(findings,
					measureShapeText(cell.Shape, pathPrefix+".shape", grid, ri, ci)...)
			}
		}
	}

	return findings
}

// measureShapeText measures text in a shape-grid shape cell.
func measureShapeText(shape *ShapeSpecInput, pathPrefix string, grid *ShapeGridInput, rowIdx, cellIdx int) []fitFinding {
	text, fontPt := extractShapeTextAndFont(shape.Text)
	if text == "" {
		return nil
	}

	// Estimate cell width from grid dimensions.
	cellWidthEMU := estimateCellWidthEMU(grid, cellIdx)

	// Estimate max lines from cell height.
	cellHeightEMU := estimateCellHeightEMU(grid, rowIdx)
	const defaultLineSpacing = 1.2
	lineHeightPt := fontPt * defaultLineSpacing
	maxLines := int(float64(cellHeightEMU) / (lineHeightPt * 12700))
	if maxLines < 1 {
		maxLines = 1
	}

	m := textfit.MeasureRun(text, "Arial", fontPt, cellWidthEMU, maxLines)
	if m.Fits {
		return nil
	}

	return []fitFinding{{
		Code:             patterns.ErrCodeFitOverflow,
		Path:             pathPrefix + ".text",
		Message:          fmt.Sprintf("text needs %d lines @ %.0fpt; cell allows %d", m.Lines, fontPt, maxLines),
		Fix:              &patterns.FixSuggestion{Kind: "reduce_text"},
		BindingDimension: "height",
		RequiredPt:       float64(m.RequiredEMU) / 12700.0,
		AllocatedPt:      float64(cellHeightEMU) / 12700.0,
		WrapLines:        m.Lines,
		Action:           "unfittable",
	}}
}

// extractShapeTextAndFont parses the text content and font size from a shape's
// json.RawMessage Text field. Returns the text and font size in points.
func extractShapeTextAndFont(raw json.RawMessage) (string, float64) {
	if len(raw) == 0 {
		return "", 0
	}

	// Try string shorthand.
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, 14.0 // default shape text size
	}

	// Try object form.
	var obj struct {
		Content    string          `json:"content"`
		Paragraphs json.RawMessage `json:"paragraphs"`
		Size       float64         `json:"size"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		fontSize := 14.0
		if obj.Size > 0 {
			fontSize = obj.Size
		}
		if obj.Content != "" {
			return obj.Content, fontSize
		}
		// Paragraphs form — concatenate text.
		if len(obj.Paragraphs) > 0 {
			var paras []struct {
				Content string `json:"content"`
			}
			if json.Unmarshal(obj.Paragraphs, &paras) == nil {
				var combined string
				for i, p := range paras {
					if i > 0 {
						combined += "\n"
					}
					combined += p.Content
				}
				return combined, fontSize
			}
		}
	}

	return "", 0
}

// estimateCellWidthEMU estimates the width of a cell in EMU based on grid config.
func estimateCellWidthEMU(grid *ShapeGridInput, cellIdx int) int64 {
	slideWidthEMU := int64(shapegrid.DefaultSlideWidthEMU)

	// Grid typically occupies ~90% of slide width if no bounds specified.
	gridWidthEMU := int64(float64(slideWidthEMU) * 0.9)
	if grid.Bounds != nil && grid.Bounds.Width > 0 {
		gridWidthEMU = int64(float64(slideWidthEMU) * grid.Bounds.Width / 100.0)
	}

	// Parse column count.
	numCols := 1
	if len(grid.Columns) > 0 {
		var n int
		if json.Unmarshal(grid.Columns, &n) == nil && n > 0 {
			numCols = n
		} else {
			var arr []float64
			if json.Unmarshal(grid.Columns, &arr) == nil && len(arr) > 0 {
				numCols = len(arr)
			}
		}
	}

	return gridWidthEMU / int64(numCols)
}

// estimateCellHeightEMU estimates the height of a cell in EMU based on grid config.
func estimateCellHeightEMU(grid *ShapeGridInput, rowIdx int) int64 {
	slideHeightEMU := int64(shapegrid.DefaultSlideHeightEMU)

	// Grid typically occupies ~70% of slide height if no bounds specified.
	gridHeightEMU := int64(float64(slideHeightEMU) * 0.7)
	if grid.Bounds != nil && grid.Bounds.Height > 0 {
		gridHeightEMU = int64(float64(slideHeightEMU) * grid.Bounds.Height / 100.0)
	}

	numRows := len(grid.Rows)
	if numRows == 0 {
		numRows = 1
	}

	// Check if the row has a specific height percentage.
	if rowIdx < len(grid.Rows) && grid.Rows[rowIdx].Height > 0 {
		return int64(float64(gridHeightEMU) * grid.Rows[rowIdx].Height / 100.0)
	}

	return gridHeightEMU / int64(numRows)
}

// writeFitReport writes fit findings as NDJSON to the given path.
// Use "-" to write to stdout.
func writeFitReport(path string, findings []fitFinding) error {
	var w io.Writer
	if path == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create fit report file: %w", err)
		}
		defer f.Close()
		w = f
	}

	enc := json.NewEncoder(w)
	for _, f := range findings {
		if err := enc.Encode(f); err != nil {
			return fmt.Errorf("failed to write fit finding: %w", err)
		}
	}

	return nil
}

// printFitReportSummary prints a human-readable summary of fit findings to stderr.
func printFitReportSummary(findings []fitFinding) {
	if len(findings) == 0 {
		fmt.Fprintln(os.Stderr, "Fit report: no issues found")
		return
	}

	fmt.Fprintf(os.Stderr, "Fit report: %d issue(s) found\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", f.Code, f.Path, f.Message)
	}
}
