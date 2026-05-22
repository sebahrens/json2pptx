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
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// fitFinding is a single fit-report entry written as NDJSON. It extends
// ValidationError with measurement-specific fields for agent consumption.
type fitFinding struct {
	Code             string                  `json:"code"`
	Path             string                  `json:"path"`
	Severity         string                  `json:"severity,omitempty"`
	Message          string                  `json:"message"`
	Fix              *patterns.FixSuggestion `json:"fix,omitempty"`
	BindingDimension string                  `json:"binding_dimension,omitempty"`
	RequiredPt       float64                 `json:"required_pt,omitempty"`
	AllocatedPt      float64                 `json:"allocated_pt,omitempty"`
	WrapLines        int                     `json:"wrap_lines,omitempty"`
	Action           string                  `json:"action,omitempty"`
}

// evaluateStrictFit runs the fit report and applies the given mode's policy
// without any stderr side effects. It returns the raw findings and, in strict
// mode, a refuse error when any finding's action is "refuse" or severity is
// "error". Callers are responsible for deciding how to surface findings
// (structured response, stderr, etc.).
//
// layouts/slideWidth/slideHeight carry the resolved template geometry so the
// shape_grid bounds measured here match what generation renders. Pass nil/0/0
// when no template has been analyzed (the report then falls back to generic
// default bounds — identical to the pre-geometry behavior).
func evaluateStrictFit(input *PresentationInput, mode string, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) ([]fitFinding, error) {
	findings := generateFitReport(input, layouts, slideWidth, slideHeight)
	if len(findings) == 0 {
		return nil, nil
	}

	if mode == "strict" {
		hasRefuse := false
		for _, f := range findings {
			if f.Action == "refuse" || f.Severity == "error" {
				hasRefuse = true
				break
			}
		}
		if hasRefuse {
			return findings, fmt.Errorf("strict-fit: %d finding(s), generation refused", len(findings))
		}
	}

	return findings, nil
}

// generateFitReport walks all tables and shape-grid text cells in the
// presentation, measuring text against available cell dimensions. It returns
// findings for cells that overflow, in canonical
// (severity_desc, slide_index_asc, code_asc) order.
//
// layouts/slideWidth/slideHeight supply the resolved template geometry used to
// place shape_grid cells. They are threaded into walkShapeGrid so the report
// resolves grids through the SAME layout-aware helpers generation uses
// (resolveGridGeometry → resolveGridBounds). With nil layouts the result is
// identical to the legacy generic-default-bounds behavior.
func generateFitReport(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []fitFinding {
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
				measureTable(table, slidepath.ContentIndex(si, ci), si)...)
		}

		// Walk shape_grid cells using the same layout-aware geometry as
		// generation so strict_fit / fit-report bounds match render.
		if slide.ShapeGrid != nil {
			findings = append(findings,
				walkShapeGrid(slide, si, layouts, slideWidth, slideHeight)...)
		}
	}

	sortLocalFitFindingsCanonical(findings)
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
		m, err := textfit.MeasureRun(header, "Calibri", fontPt, colWidthEMU, maxLines)
		if err != nil {
			continue
		}
		if !m.Fits {
			findings = append(findings, fitFinding{
				Code:             patterns.ErrCodeFitOverflow,
				Path:             slidepath.TableHeader(pathPrefix, hi),
				Message:          fmt.Sprintf("header %q needs %d lines @ %.0fpt; cell allows %d", header, m.Lines, fontPt, maxLines),
				Fix:              &patterns.FixSuggestion{Kind: "reduce_text"},
				BindingDimension: "height",
				RequiredPt:       float64(m.RequiredEMU) / 12700.0,
				AllocatedPt:      float64(defaultRowHeightEMU) / 12700.0,
				WrapLines:        m.Lines,
				Action:           "refuse",
			})
		}
	}

	// Measure data cells.
	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Content == "" {
				continue
			}
			m, err := textfit.MeasureRun(cell.Content, "Calibri", fontPt, colWidthEMU, maxLines)
			if err != nil {
				continue
			}
			if !m.Fits {
				findings = append(findings, fitFinding{
					Code:             patterns.ErrCodeFitOverflow,
					Path:             slidepath.TableCell(pathPrefix, ri, ci),
					Message:          fmt.Sprintf("text needs %d lines @ %.0fpt; cell allows %d", m.Lines, fontPt, maxLines),
					Fix:              &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": ri + numRows/2}},
					BindingDimension: "height",
					RequiredPt:       float64(m.RequiredEMU) / 12700.0,
					AllocatedPt:      float64(defaultRowHeightEMU) / 12700.0,
					WrapLines:        m.Lines,
					Action:           "refuse",
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
			Action:  "refuse",
		})
	}

	// Schema-level density check (rows ≤ 7, cols ≤ 6 with multiline counting).
	for _, ve := range pipeline.DetectTableDensity(table, pathPrefix) {
		findings = append(findings, fitFinding{
			Code:    ve.Code,
			Path:    ve.Path,
			Message: ve.Message,
			Fix:     ve.Fix,
			Action:  "review",
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

// walkShapeGrid resolves a shape grid against the SAME layout-aware geometry
// generation uses (resolveGridGeometry → resolveGridBounds, via
// resolveGridForStructural) and runs textcapacity.ForResolvedGrid over the
// result, emitting fit findings for overflowing cells and row max_height
// violations. Resolving with the template's content-zone / virtual-layout
// bounds (rather than generic defaults) is what keeps strict_fit and
// fit-report cell measurements in lockstep with the rendered PPTX
// (go-slide-creator-ur3z). With nil layouts the geometry reduces to the legacy
// "explicit bounds or DefaultBounds" behavior.
func walkShapeGrid(slide SlideInput, slideIdx int, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []fitFinding {
	grid := slide.ShapeGrid
	if grid == nil {
		return nil
	}
	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	// Resolve the grid to authoritative cell bounds using generation's geometry.
	geom := resolveGridGeometry(slide, layouts, slideWidth, slideHeight)
	result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
	if result == nil {
		return nil
	}

	// Compute densities using the single source of truth.
	densities := textcapacity.ForResolvedGrid(result)

	var findings []fitFinding

	// Emit row overflow findings from the resolve result.
	for _, ro := range result.RowOverflows {
		findings = append(findings, fitFinding{
			Code:             patterns.ErrCodeFitOverflow,
			Path:             slidepath.GridRow(slideIdx, ro.RowIndex),
			Message:          fmt.Sprintf("row content ~%.0fpt exceeds max_height %.0fpt", ro.ContentPt, ro.MaxHeightPt),
			Fix:              &patterns.FixSuggestion{Kind: "reduce_text"},
			BindingDimension: "height",
			RequiredPt:       ro.ContentPt,
			AllocatedPt:      ro.MaxHeightPt,
			Action:           "refuse",
		})
	}

	// Walk cells: emit overflow findings from density and handle embedded tables.
	cellIdx := 0
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cellIdx >= len(densities) {
				break
			}
			pathPrefix := slidepath.GridCell(slideIdx, ri, ci)

			// Embedded table in shape_grid cell.
			if cell != nil && cell.Table != nil {
				findings = append(findings,
					measureTable(cell.Table, slidepath.Join(pathPrefix, "table"), slideIdx)...)
			}

			// Shape text density via textcapacity.
			d := densities[cellIdx]
			if d.MaxChars > 0 {
				switch {
				case d.DensityPct > 130:
					// Severe overflow — error severity.
					findings = append(findings, fitFinding{
						Code:             patterns.ErrCodeFitOverflow,
						Path:             slidepath.Join(pathPrefix, "shape/text"),
						Severity:         "error",
						Message:          fmt.Sprintf("text needs %d chars @ %.0fpt; cell allows %d (%d%% of capacity)", d.ActualChars, d.FontPt, d.MaxChars, d.DensityPct),
						Fix:              &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_chars": d.MaxChars}},
						BindingDimension: "height",
						RequiredPt:       float64(d.HeightEMU) / 12700.0 * float64(d.DensityPct) / 100.0,
						AllocatedPt:      float64(d.HeightEMU) / 12700.0,
						Action:           "refuse",
					})
				case d.DensityPct > 110:
					// Moderate overflow — warning severity.
					findings = append(findings, fitFinding{
						Code:             patterns.ErrCodeFitOverflow,
						Path:             slidepath.Join(pathPrefix, "shape/text"),
						Severity:         "warning",
						Message:          fmt.Sprintf("text needs %d chars @ %.0fpt; cell allows %d (%d%% of capacity)", d.ActualChars, d.FontPt, d.MaxChars, d.DensityPct),
						Fix:              &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_chars": d.MaxChars}},
						BindingDimension: "height",
						RequiredPt:       float64(d.HeightEMU) / 12700.0 * float64(d.DensityPct) / 100.0,
						AllocatedPt:      float64(d.HeightEMU) / 12700.0,
						Action:           "review",
					})
				case d.DensityPct < 40 && d.ActualChars > 0:
					// Very underfilled — warning severity.
					findings = append(findings, fitFinding{
						Code:     patterns.ErrCodeCellUnderfilled,
						Path:     slidepath.Join(pathPrefix, "shape/text"),
						Severity: "warning",
						Message:  fmt.Sprintf("cell content is %d chars (%d%% of capacity) — consider adding detail or smaller grid", d.ActualChars, d.DensityPct),
						Fix:      &patterns.FixSuggestion{Kind: "add_detail_or_resize", Params: map[string]any{"current_density_pct": d.DensityPct}},
						Action:   "review",
					})
				case d.DensityPct >= 40 && d.DensityPct < 60 && d.ActualChars > 0:
					// Underfilled — info severity.
					findings = append(findings, fitFinding{
						Code:     patterns.ErrCodeCellUnderfilled,
						Path:     slidepath.Join(pathPrefix, "shape/text"),
						Severity: "info",
						Message:  fmt.Sprintf("cell content is %d chars (%d%% of capacity) — consider adding detail or smaller grid", d.ActualChars, d.DensityPct),
						Fix:      &patterns.FixSuggestion{Kind: "add_detail_or_resize", Params: map[string]any{"current_density_pct": d.DensityPct}},
						Action:   "review",
					})
				}
			}

			cellIdx++
		}
	}

	return findings
}



// writeFitReportNDJSON writes fit findings as NDJSON to the given writer.
func writeFitReportNDJSON(w io.Writer, findings []fitFinding) {
	enc := json.NewEncoder(w)
	for _, f := range findings {
		_ = enc.Encode(f)
	}
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

	writeFitReportNDJSON(w, findings)
	return nil
}

// printFitFindingsBySlide prints fit findings grouped by slide index to stderr.
func printFitFindingsBySlide(findings []fitFinding) {
	if len(findings) == 0 {
		fmt.Fprintln(os.Stderr, "Fit report: no issues found")
		return
	}

	// Group by slide index extracted from path.
	groups := make(map[int][]fitFinding)
	var slideOrder []int
	for _, f := range findings {
		si := slideIndexFromPath(f.Path)
		if _, seen := groups[si]; !seen {
			slideOrder = append(slideOrder, si)
		}
		groups[si] = append(groups[si], f)
	}

	fmt.Fprintf(os.Stderr, "Fit report: %d finding(s)\n", len(findings))
	for _, si := range slideOrder {
		if si >= 0 {
			fmt.Fprintf(os.Stderr, "  Slide %d:\n", si+1)
		} else {
			fmt.Fprintf(os.Stderr, "  Unknown slide:\n")
		}
		for _, f := range groups[si] {
			fmt.Fprintf(os.Stderr, "    [%s] %s — %s\n", f.Action, f.Path, f.Message)
		}
	}
}
