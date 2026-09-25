package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
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

// evaluateStrictFit runs the shared fit collector and applies the given mode's policy
// without any stderr side effects. It returns the raw findings and, in strict
// mode, a refuse error when any finding's action is "refuse". In warn mode it
// corrects conflicting authored numeric section labels before conversion and
// replaces their refusal with an informational renumbered finding.
// Callers are responsible for deciding how to surface findings
// (structured response, stderr, etc.).
//
// layouts/slideWidth/slideHeight carry the resolved template geometry so the
// shape_grid bounds measured here match what generation renders. Pass nil/0/0
// when no template has been analyzed (the report then falls back to generic
// default bounds — identical to the pre-geometry behavior).
func evaluateStrictFit(input *PresentationInput, mode string, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) ([]patterns.FitFinding, error) {
	findings := collectFitFindings(input, layouts, slideWidth, slideHeight, theme)
	if len(findings) == 0 {
		return nil, nil
	}
	if mode == "warn" {
		var err error
		findings, err = renumberWarnModeSections(input, findings)
		if err != nil {
			return findings, err
		}
	}

	if mode == "strict" {
		if hasRefuseFinding(findings) {
			return findings, fmt.Errorf("strict-fit: %d finding(s), generation refused", len(findings))
		}
	}

	return findings, nil
}

func hasRefuseFinding(findings []patterns.FitFinding) bool {
	for _, finding := range findings {
		if finding.Action == "refuse" {
			return true
		}
	}
	return false
}

func renumberWarnModeSections(input *PresentationInput, findings []patterns.FitFinding) ([]patterns.FitFinding, error) {
	for i := range findings {
		f := &findings[i]
		if f.Code != patterns.ErrCodeSectionNumberSequenceMismatch {
			continue
		}
		if f.Fix == nil {
			return findings, fmt.Errorf("section-number finding at %s has no correction", f.Path)
		}
		slideIndex, slideOK := f.Fix.Params["slide_index"].(int)
		contentIndex, contentOK := f.Fix.Params["content_index"].(int)
		expected, expectedOK := f.Fix.Params["expected"].(string)
		if !slideOK || !contentOK || !expectedOK || slideIndex < 0 || slideIndex >= len(input.Slides) || contentIndex < 0 || contentIndex >= len(input.Slides[slideIndex].Content) {
			return findings, fmt.Errorf("section-number finding at %s has invalid correction target", f.Path)
		}
		item := &input.Slides[slideIndex].Content[contentIndex]
		item.TextValue = &expected
		item.Value = nil
		f.Code = patterns.ErrCodeSectionNumberRenumbered
		f.Message = fmt.Sprintf("slide %d section number was corrected from %q to %q in warn mode", slideIndex+1, f.Fix.Params["actual"], expected)
		f.Fix = nil
		f.NextToolCall = nil
		f.Action = "info"
	}
	// The collector sorted the mismatch as a refusal. After correction it is
	// informational, so restore the canonical severity-first wire order.
	patterns.SortCanonical(findings, slidepath.SlideIndex)
	return findings, nil
}

// generateFitReport walks all tables and shape-grid text cells in the
// presentation, measuring text against available cell dimensions. It returns
// findings for cells that overflow, in canonical
// (severity_desc, slide_index_asc, code_asc) order.
//
// layouts/slideWidth/slideHeight supply the resolved template geometry used to
// place shape_grid cells. They are threaded into walkShapeGrid so the report
// resolves grids through the SAME layout and rhythm geometry generation uses.
// With neither template layouts nor a deck grid, it falls back to the legacy
// generic bounds.
func generateFitReport(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []fitFinding {
	var findings []fitFinding
	rhythm := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)

	// Pattern slides carry no ShapeGrid until generation, so every text-capacity
	// check below used to skip the surface agents are told to author through
	// (go-slide-creator-adur). Expand once, measure the same cells generation
	// renders, and report them under /slides/N/pattern.
	input, fromPattern := expandPatternsForFit(input, slideWidth, slideHeight, nil, layouts...)

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
			cells := walkShapeGrid(slide, si, layouts, slideWidth, slideHeight, rhythm)
			if fromPattern[si] {
				patternName := ""
				if slide.Pattern != nil {
					patternName = slide.Pattern.Name
				}
				for i := range cells {
					cells[i].Path = rerootPatternPath(cells[i].Path, fromPattern)
					cells[i].Fix = patternCellFix(cells[i].Fix, patternName)
				}
			}
			findings = append(findings, cells...)
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

	// Row height budget: rows grow with their content, so a cell may wrap as
	// long as the table still fits its body area. Budget each row an equal
	// share of a typical body placeholder (~60% of the slide height), never
	// less than the generator's minimum row height. (With glyph widths now
	// measured at the real point size, the old fixed one-line-per-row budget
	// flagged ordinary two-line cells as overflow.)
	const minRowHeightEMU int64 = 370840
	const defaultLineSpacing = 1.2
	defaultRowHeightEMU := int64(float64(shapegrid.DefaultSlideHeightEMU) * 0.6 / float64(numRows))
	if defaultRowHeightEMU < minRowHeightEMU {
		defaultRowHeightEMU = minRowHeightEMU
	}
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
	// The action comes from what the CODE declares, not from this call site:
	// the same facts are also emitted by the validator, and hardcoding "review"
	// here put one table's density in a single response as both a warning and an
	// info (go-slide-creator-7xyy).
	for _, ve := range pipeline.DetectTableDensity(table, pathPrefix) {
		findings = append(findings, fitFinding{
			Code:    ve.Code,
			Path:    ve.Path,
			Message: ve.Message,
			Fix:     ve.Fix,
			Action:  declaredFindingAction(ve.Code),
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
// generation uses (patternExpansionGeometry → resolveGridBounds, via
// resolveGridForStructural) and runs textcapacity.ForResolvedGrid over the
// result, emitting fit findings for overflowing cells and row max_height
// violations. Resolving with the template's content-zone / virtual-layout
// bounds (rather than generic defaults) is what keeps strict_fit and
// fit-report cell measurements in lockstep with the rendered PPTX, including
// when a deck rhythm grid overrides the template zone
// (go-slide-creator-ur3z, go-slide-creator-vrckb). Without layouts or a deck
// grid, geometry reduces to the legacy "explicit bounds or DefaultBounds" behavior.
func walkShapeGrid(slide SlideInput, slideIdx int, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, rhythm *resolvedGrid) []fitFinding {
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
	geom, _ := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythm)
	result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
	if result == nil {
		return nil
	}

	acc := &gridFitAccum{slideIdx: slideIdx, slideWidth: slideWidth, slideHeight: slideHeight}
	acc.walk(grid, result, slidepath.ShapeGrid(slideIdx), 0)
	// Text-capacity occupancy is meaningful for an author-sized raw grid. A
	// named pattern's cells are sized and padded by its expander; terse text in
	// a framework cell is often complete content, not a missing argument.
	// Pattern slides get the resolved-ink SLIDE_UNDERUSED/SPARSE_FILL checks
	// instead, while their overflow and readability checks above remain.
	if slide.Pattern == nil && slide.Compose == nil {
		if f := aggregateUnderfilledFinding(slideIdx, acc.underfilled, acc.measuredTextCells, acc.filledChars, acc.totalCapacity); f != nil {
			acc.findings = append(acc.findings, *f)
		}
	}
	return acc.findings
}

type gridFitAccum struct {
	slideIdx                   int
	slideWidth, slideHeight    int64
	findings                   []fitFinding
	underfilled                []underfilledCell
	measuredTextCells          int
	filledChars, totalCapacity int
}

func (a *gridFitAccum) walk(grid *ShapeGridInput, result *shapegrid.ResolveResult, base string, depth int) {
	for _, ro := range result.RowOverflows {
		a.findings = append(a.findings, fitFinding{
			Code:    patterns.ErrCodeFitOverflow,
			Path:    fmt.Sprintf("%s/rows/%d", base, ro.RowIndex),
			Message: fmt.Sprintf("row content ~%.0fpt exceeds max_height %.0fpt", ro.ContentPt, ro.MaxHeightPt),
			// A row is not a text target: the executable repairs are per-cell
			// (the cell findings below carry reduce_cell_text with each cell's
			// budget) or a reshape. reduce_text here named no reachable content
			// and applied nothing (go-slide-creator-9zof).
			Fix: &patterns.FixSuggestion{Kind: "increase_row_height", Params: map[string]any{
				"row":           ro.RowIndex,
				"content_pt":    math.Round(ro.ContentPt),
				"max_height_pt": math.Round(ro.MaxHeightPt),
			}},
			BindingDimension: "height",
			RequiredPt:       ro.ContentPt,
			AllocatedPt:      ro.MaxHeightPt,
			// Advisory, not blocking: this is a row-level estimate, and the
			// authoritative per-cell checks below already refuse text that is
			// clipped after autofit. Every row overflow across the bundled and
			// gallery decks is a 4-8% overshoot — blocking a deck because a row
			// wants 27pt in a 25pt slot is the kind of refusal
			// go-slide-creator-lmpu was about (surfaced once pattern slides
			// started being measured at all, go-slide-creator-adur).
			Action: "review",
		})
	}
	densities := textcapacity.ForResolvedGrid(result)
	for i, rc := range result.Cells {
		if rc.RowIdx < 0 || rc.RowIdx >= len(grid.Rows) {
			continue
		}
		cell := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		pathPrefix := fmt.Sprintf("%s/rows/%d/cells/%d", base, rc.RowIdx, rc.ColIdx)
		if cell != nil && cell.Table != nil {
			a.findings = append(a.findings, measureTable(cell.Table, slidepath.Join(pathPrefix, "table"), a.slideIdx)...)
		}
		if i < len(densities) && densities[i].MaxChars > 0 {
			cellFindings, counted, under := cellDensityFindings(densities[i], pathPrefix)
			a.findings = append(a.findings, cellFindings...)
			if counted {
				a.measuredTextCells++
				a.filledChars += densities[i].ActualChars
				a.totalCapacity += densities[i].MaxChars
			}
			if under != nil {
				a.underfilled = append(a.underfilled, *under)
			}
		}
		if rc.Kind == shapegrid.CellKindSubGrid && cell != nil && cell.Grid != nil && depth < maxGeomNestingDepth {
			bounds := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
			if bounds.CX <= 0 || bounds.CY <= 0 {
				bounds = rc.Bounds
			}
			if sub := resolveGridForStructural(cell.Grid, &bounds, nil, a.slideWidth, a.slideHeight); sub != nil {
				a.walk(cell.Grid, sub, slidepath.Join(pathPrefix, "grid"), depth+1)
			}
		}
	}
}

// cellDensityFindings classifies one cell's text density. It returns any
// immediate findings (overflow), whether the cell counts toward the slide's
// measured text capacity, and — when the cell is underfilled — the cell held
// back for the per-slide aggregate.
//
// Only overflow is reported per cell. Underfill is aggregated, because one
// finding per cell let a slide of cards accumulate 20+ review-weight findings
// (go-slide-creator-xpz8).
func cellDensityFindings(d textcapacity.Density, pathPrefix string) (findings []fitFinding, counted bool, under *underfilledCell) {
	if d.ActualChars == 0 || underfillExemptRole(d.FontPt, d.ActualChars) {
		// An empty cell has no density to judge, and a metric or label's
		// character count says nothing about whether it is "full".
		return nil, false, nil
	}
	counted = true

	textPath := slidepath.Join(pathPrefix, "shape/text")
	overflow := func(severity, action string) fitFinding {
		return fitFinding{
			Code:     patterns.ErrCodeFitOverflow,
			Path:     textPath,
			Severity: severity,
			Message: fmt.Sprintf("text needs %.0fpt of height in a cell that offers %.0fpt (%d%% at %.0fpt); even the renderer's smallest autofit shrink leaves it clipped",
				d.RequiredHeightPt, d.AvailableHeightPt, d.DensityPct, d.FontPt),
			// reduce_cell_text, not reduce_text: this text lives in a grid cell,
			// and reduce_text only edits content items — so the suggested fix
			// applied 0 of 60 times on a shape_grid deck
			// (go-slide-creator-9zof).
			Fix: &patterns.FixSuggestion{Kind: "reduce_cell_text", Params: map[string]any{
				"cell_path": pathPrefix,
				"max_chars": d.MaxChars,
			}},
			BindingDimension: "height",
			RequiredPt:       d.RequiredHeightPt,
			AllocatedPt:      d.AvailableHeightPt,
			Action:           action,
		}
	}

	switch {
	case !d.Fits:
		// Genuine overflow: the wrapped block does not fit even at the smallest
		// shrink <a:normAutofit/> applies, so text IS clipped on the rendered
		// slide.
		//
		// A height ratio over 110% used to be reported as overflow on its own,
		// which refused the project's own showcase decks: a cell PowerPoint
		// autofits from 12pt to 8.6pt renders every word, and the real defect
		// there is the 8.6pt — reported by TEXT_BELOW_READABLE_MIN, at review
		// severity, where it belongs (go-slide-creator-lmpu).
		findings = append(findings, overflow("error", "refuse"))
	case d.DensityPct < textcapacity.UnderfilledPct:
		under = &underfilledCell{
			path:       textPath,
			chars:      d.ActualChars,
			maxChars:   d.MaxChars,
			densityPct: d.DensityPct,
		}
	}
	return findings, counted, under
}

// underfilledCell is one cell held back for the per-slide cell_underfilled
// aggregate.
type underfilledCell struct {
	path       string
	chars      int
	maxChars   int
	densityPct int
}

// underfillSlideFillPct is the slide-level fill below which a shape grid is
// genuinely mostly empty and worth acting on. Counting sparse CELLS was the
// wrong measure: a card grid whose bodies are one deliberate sentence each has
// every cell "sparse" while the slide reads fine. Measuring the slide's total
// characters against its total capacity distinguishes a deliberately airy
// layout from one that has barely any content in it
// (go-slide-creator-xpz8).
const underfillSlideFillPct = 30

// underfillSparseSlideMinCells is the smallest number of underfilled cells that
// can make a slide "mostly empty". Below this a slide is too small for the
// measure to mean anything.
const underfillSparseSlideMinCells = 3

// underfillExemptRole reports whether a cell's text role makes the
// character-count capacity model meaningless. A KPI value ("$12.4M") and a
// label or chip caption ("Revenue", a chevron step name) are CORRECT when
// short — reporting them as underfilled is noise, and it was the dominant
// source of it (go-slide-creator-xpz8).
func underfillExemptRole(fontPt float64, chars int) bool {
	switch cellTextRole(fontPt, false, chars) {
	case tokens.TextRoleKPIValue, tokens.TextRoleCaption:
		return true
	default:
		return false
	}
}

// aggregateUnderfilledFinding folds a slide's underfilled cells into a single
// finding. It is advisory (action "info") unless the grid as a whole carries
// barely any content, in which case the slide really is mostly empty.
func aggregateUnderfilledFinding(slideIdx int, cells []underfilledCell, measuredTextCells, filledChars, totalCapacity int) *fitFinding {
	if len(cells) == 0 {
		return nil
	}

	paths := make([]any, 0, len(cells))
	minDensity := 100
	for _, c := range cells {
		paths = append(paths, map[string]any{
			"path":        c.path,
			"chars":       c.chars,
			"density_pct": c.densityPct,
		})
		if c.densityPct < minDensity {
			minDensity = c.densityPct
		}
	}

	slideFillPct := 0
	if totalCapacity > 0 {
		slideFillPct = filledChars * 100 / totalCapacity
	}
	mostlyEmpty := len(cells) >= underfillSparseSlideMinCells &&
		totalCapacity > 0 &&
		slideFillPct < underfillSlideFillPct

	action, severity := "info", "info"
	message := fmt.Sprintf(
		"%d of %d text cells fill under %d%% of their box height (slide fill %d%%, sparsest cell %d%%) — the grid may be larger than the content needs",
		len(cells), measuredTextCells, textcapacity.UnderfilledPct, slideFillPct, minDensity)
	if mostlyEmpty {
		action, severity = "review", "warning"
		message = fmt.Sprintf(
			"the shape grid carries only %d%% of its text capacity across %d cells (sparsest %d%%) — the slide is mostly empty; add detail or use a smaller grid",
			slideFillPct, measuredTextCells, minDensity)
	}

	return &fitFinding{
		Code:     patterns.ErrCodeCellUnderfilled,
		Path:     slidepath.ShapeGrid(slideIdx),
		Severity: severity,
		Message:  message,
		Fix: &patterns.FixSuggestion{
			Kind: "add_detail_or_resize",
			Params: map[string]any{
				"cells":               paths,
				"underfilled_cells":   len(cells),
				"measured_text_cells": measuredTextCells,
				"min_density_pct":     minDensity,
				"slide_fill_pct":      slideFillPct,
				"slide_mostly_empty":  mostlyEmpty,
			},
		},
		Action: action,
	}
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
		si := slidepath.SlideIndex(f.Path)
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

// declaredFindingAction returns the action a finding code declares in the
// finding-meta registry, or "review" when the code has no entry. An emitter that
// has no better information should use it rather than picking an action, so one
// code means one severity wherever it is raised (go-slide-creator-7xyy).
func declaredFindingAction(code string) string {
	if meta, ok := patterns.GetFindingMeta(code); ok && meta.Severity != "" {
		return meta.Severity
	}
	return "review"
}
