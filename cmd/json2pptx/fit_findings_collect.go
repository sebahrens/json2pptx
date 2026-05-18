package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// DefaultFindingBudget is the maximum number of findings returned per slide
// before overflow is summarised. Use verbose=true in BudgetFitFindings to
// bypass the limit.
const DefaultFindingBudget = 5

// collectFitFindings runs all fit-report detectors (text overflow, placeholder
// overflow, title wraps, footer collision, bounds overflow) and returns sorted
// findings. The result is sorted by ActionRank descending (most severe first),
// then by slide index ascending.
//
// When theme is non-nil, preflight predictors that need theme colors run too:
// currently this gates the contrast_predicted detector. Pass nil to skip
// those (callers that don't have a parsed template theme).
func collectFitFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// Expand compose envelopes into ShapeGrid so downstream detectors evaluate
	// against post-merge cell geometry. Without this step, a diagram placed in
	// a horizontally-merged segment would never trip CheckDiagramInNarrowBoundsFinding
	// or aspect-mismatch findings during preflight — the unexpanded slide
	// carries ShapeGrid == nil, so checkShapeGridStructural is skipped.
	input = expandComposeForPreflight(input, slideWidth, slideHeight)

	// 1. Text-fit findings from existing generateFitReport (tables + shape-grid text).
	for _, tf := range generateFitReport(input) {
		findings = append(findings, convertTextFitFinding(tf))
	}

	// 2. Structural findings using template layout data.
	findings = append(findings,
		collectStructuralFindings(input, layouts, slideWidth, slideHeight)...)

	// 3. Grid rhythm violations when a deck-level grid is configured.
	if input.Grid != nil {
		if err := validateGridConfig(input.Grid); err == nil {
			rg := resolveGrid(input.Grid, layouts, slideWidth, slideHeight)
			findings = append(findings, detectGridViolations(rg, layouts, input.Slides)...)
		}
	}

	// 4. Grid occupancy: pattern_underfilled / pattern_overcrowded.
	findings = append(findings, collectGridOccupancyFindings(input)...)

	// 5. Preflight predictions for render-time-only findings: table_font_scaled,
	// table_rows_truncated, column_width_deficit, text_trimmed,
	// readability_trimmed. These mirror the renderer's scaling/trimming logic
	// without rendering.
	findings = append(findings, collectTablePreflightFindings(input, layouts)...)
	findings = append(findings, collectTextAutofitPreflightFindings(input, layouts)...)

	// 6. Contrast prediction (contrast_predicted) — runs only when theme
	// colors are available to resolve scheme references.
	if theme != nil {
		findings = append(findings, collectContrastPreflightFindings(input, theme.Colors)...)
	}

	// 7. Chart / diagram dry-render findings (chart.tick_thinned,
	// chart.label_clipped, chart.legend_overflow_dropped, etc.) — runs
	// svggen's layout/labeling pass per chart/diagram content item and
	// surfaces render-only findings at validate/preview time, closing the
	// validate → preview → generate feedback loop for visual chart issues.
	var chartThemeColors []types.ThemeColor
	if theme != nil {
		chartThemeColors = theme.Colors
	}
	findings = append(findings,
		collectChartDryRenderFindings(input, chartThemeColors, "warn")...)

	// Deduplicate findings that share (Code, Path, Action, Message). This guards
	// against the case where a pre-compose detector and the post-compose
	// structural pass both emit the same diagnostic for one cell — the
	// post-compose check on the merged grid should not double-surface.
	findings = dedupFitFindings(findings)

	// Sort by ActionRank desc, then slide index asc.
	sort.Slice(findings, func(i, j int) bool {
		ri := patterns.ActionRank(findings[i].Action)
		rj := patterns.ActionRank(findings[j].Action)
		if ri != rj {
			return ri > rj
		}
		return slidepath.SlideIndex(findings[i].Path) < slidepath.SlideIndex(findings[j].Path)
	})

	// Attach next_tool_call to actionable findings.
	patterns.AttachNextToolCalls(findings, slidepath.SlideIndex)

	return findings
}

// BudgetFitFindings enforces a per-slide finding budget. Within each slide,
// findings are ranked by severity (ActionRank descending) then actionability
// (findings with a Fix set are ranked above those without). If a slide exceeds
// the budget, only the top findings are kept and a summary finding is appended
// indicating how many were suppressed.
//
// When verbose is true the budget is not applied and all findings are returned.
func BudgetFitFindings(findings []patterns.FitFinding, budget int, verbose bool) []patterns.FitFinding {
	if verbose || len(findings) == 0 {
		return findings
	}
	if budget <= 0 {
		budget = DefaultFindingBudget
	}

	// Group findings by slide index.
	type group struct {
		slideIdx int
		items    []patterns.FitFinding
	}
	order := []int{}           // insertion-order slide indices
	bySlide := map[int]*group{}

	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		g, ok := bySlide[si]
		if !ok {
			g = &group{slideIdx: si}
			bySlide[si] = g
			order = append(order, si)
		}
		g.items = append(g.items, f)
	}

	// Sort each group: ActionRank desc, then Fix-present before Fix-absent.
	for _, si := range order {
		g := bySlide[si]
		sort.SliceStable(g.items, func(i, j int) bool {
			ri := patterns.ActionRank(g.items[i].Action)
			rj := patterns.ActionRank(g.items[j].Action)
			if ri != rj {
				return ri > rj
			}
			fi := g.items[i].Fix != nil
			fj := g.items[j].Fix != nil
			if fi != fj {
				return fi
			}
			return false
		})
	}

	// Apply budget per slide.
	var result []patterns.FitFinding
	for _, si := range order {
		g := bySlide[si]
		if len(g.items) <= budget {
			result = append(result, g.items...)
			continue
		}
		result = append(result, g.items[:budget]...)
		suppressed := g.items[budget:]
		path := slidepath.Slide(si)
		if si < 0 {
			path = "/slides/?"
		}
		topCodes := findingCodeHistogram(suppressed)
		result = append(result, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    "findings_truncated",
				Message: fmt.Sprintf("%d more findings suppressed on this slide; use verbose_fit to see all", len(suppressed)),
				Fix: &patterns.FixSuggestion{
					Kind: "truncation_summary",
					Params: map[string]any{
						"suppressed_count": len(suppressed),
						"top_codes":        topCodes,
					},
				},
			},
			Action: "info",
		})
	}

	return result
}

// budgetLocalFindings applies the same per-slide budget as BudgetFitFindings
// but operates on the local fitFinding type used by generateFitReport.
func budgetLocalFindings(findings []fitFinding, budget int, verbose bool) []fitFinding {
	if verbose || len(findings) == 0 {
		return findings
	}
	if budget <= 0 {
		budget = DefaultFindingBudget
	}

	type group struct {
		slideIdx int
		items    []fitFinding
	}
	order := []int{}
	bySlide := map[int]*group{}

	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		g, ok := bySlide[si]
		if !ok {
			g = &group{slideIdx: si}
			bySlide[si] = g
			order = append(order, si)
		}
		g.items = append(g.items, f)
	}

	for _, si := range order {
		g := bySlide[si]
		sort.SliceStable(g.items, func(i, j int) bool {
			ri := patterns.ActionRank(g.items[i].Action)
			rj := patterns.ActionRank(g.items[j].Action)
			if ri != rj {
				return ri > rj
			}
			fi := g.items[i].Fix != nil
			fj := g.items[j].Fix != nil
			if fi != fj {
				return fi
			}
			return false
		})
	}

	var result []fitFinding
	for _, si := range order {
		g := bySlide[si]
		if len(g.items) <= budget {
			result = append(result, g.items...)
			continue
		}
		result = append(result, g.items[:budget]...)
		suppressed := g.items[budget:]
		path := slidepath.Slide(si)
		if si < 0 {
			path = "/slides/?"
		}
		topCodes := localFindingCodeHistogram(suppressed)
		result = append(result, fitFinding{
			Code:    "findings_truncated",
			Path:    path,
			Message: fmt.Sprintf("%d more findings suppressed on this slide; use --verbose-fit to see all", len(suppressed)),
			Action:  "info",
			Fix: &patterns.FixSuggestion{
				Kind: "truncation_summary",
				Params: map[string]any{
					"suppressed_count": len(suppressed),
					"top_codes":        topCodes,
				},
			},
		})
	}

	return result
}

// convertTextFitFinding converts a local fitFinding to patterns.FitFinding.
func convertTextFitFinding(tf fitFinding) patterns.FitFinding {
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    tf.Path,
			Code:    tf.Code,
			Message: tf.Message,
			Fix:     tf.Fix,
		},
		Action: tf.Action,
	}
	if tf.RequiredPt > 0 || tf.AllocatedPt > 0 {
		f.Measured = &patterns.Extent{HeightEMU: int64(tf.RequiredPt * 12700)}
		f.Allowed = &patterns.Extent{HeightEMU: int64(tf.AllocatedPt * 12700)}
		if tf.AllocatedPt > 0 {
			f.OverflowRatio = tf.RequiredPt / tf.AllocatedPt
		}
	}
	return f
}

// collectStructuralFindings runs placeholder overflow, title wraps, footer
// collision, and bounds overflow detectors using template layout data.
func collectStructuralFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	var findings []patterns.FitFinding

	footerEnabled := input.Footer != nil && input.Footer.Enabled

	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)

		// Placeholder overflow and title wraps.
		if layout != nil {
			findings = append(findings, checkPlaceholderFindings(&slide, si, layout)...)
		}

		// Shape grid: footer collision and bounds overflow.
		if slide.ShapeGrid != nil && layout != nil {
			patternName := ""
			if slide.Pattern != nil {
				patternName = slide.Pattern.Name
			}
			findings = append(findings,
				checkShapeGridStructural(slide.ShapeGrid, si, slideWidth, slideHeight, layout, footerEnabled, patternName)...)
		}
	}

	return findings
}

// checkPlaceholderFindings checks a slide's content placeholders for title
// wraps, body overflow, and (for diagram content) intrinsic aspect conflicts.
func checkPlaceholderFindings(slide *SlideInput, si int, layout *types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for _, content := range slide.Content {
		ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
		if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
			continue
		}

		// Diagram content placed in a placeholder: predict aspect conflict
		// against the diagram type's natural viewBox aspect.
		if content.Type == "diagram" && content.DiagramValue != nil {
			path := slidepath.Content(si, content.PlaceholderID)
			if f := generator.CheckDiagramAspectConflictFinding(content.DiagramValue, ph.Bounds.Width, ph.Bounds.Height, path); f != nil {
				findings = append(findings, *f)
			}
		}

		paragraphs := extractContentParagraphs(&content)
		if len(paragraphs) == 0 {
			continue
		}

		path := slidepath.Content(si, content.PlaceholderID)

		if ph.Type == types.PlaceholderTitle {
			if f := generator.DetectTitleWraps(generator.TitleWrapsInput{
				SlideIndex:  si,
				Path:        path,
				Title:       strings.Join(paragraphs, " "),
				WidthEMU:    ph.Bounds.Width,
				HeightEMU:   ph.Bounds.Height,
				FontSizeHPt: ph.FontSize,
				FontName:    ph.FontFamily,
			}); f != nil {
				findings = append(findings, *f)
			}
		} else if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			if f := generator.DetectPlaceholderOverflow(generator.PlaceholderOverflowInput{
				SlideIndex:  si,
				Path:        path,
				Paragraphs:  paragraphs,
				WidthEMU:    ph.Bounds.Width,
				HeightEMU:   ph.Bounds.Height,
				FontSizeHPt: ph.FontSize,
				FontName:    ph.FontFamily,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}
	return findings
}

// gridContext holds pre-computed layout data for shape grid structural checks.
type gridContext struct {
	gridX, gridY         int64
	footerY, footerCY    int64
	layoutDeclaresFooter bool
	footerEnabled        bool
	slideWidth           int64
	slideHeight          int64
}

// resolveGridContext extracts footer and grid origin data from a layout.
func resolveGridContext(grid *ShapeGridInput, layout *types.LayoutMetadata, slideWidth, slideHeight int64, footerEnabled bool) gridContext {
	ctx := gridContext{
		gridX:         457200,  // 0.5 inch default
		gridY:         1600200, // ~1.26 inch default (below title)
		footerEnabled: footerEnabled,
		slideWidth:    slideWidth,
		slideHeight:   slideHeight,
	}

	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderOther && ph.Bounds.Height > 0 && !ctx.layoutDeclaresFooter {
			ctx.footerY = ph.Bounds.Y
			ctx.footerCY = ph.Bounds.Height
			ctx.layoutDeclaresFooter = true
		}
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			ctx.gridX = ph.Bounds.X
			ctx.gridY = ph.Bounds.Y
		}
	}

	if grid.Bounds != nil {
		if grid.Bounds.X > 0 {
			sw := slideWidth
			if sw <= 0 {
				sw = shapegrid.DefaultSlideWidthEMU
			}
			ctx.gridX = int64(float64(sw) * grid.Bounds.X / 100.0)
		}
		if grid.Bounds.Y > 0 {
			sh := slideHeight
			if sh <= 0 {
				sh = shapegrid.DefaultSlideHeightEMU
			}
			ctx.gridY = int64(float64(sh) * grid.Bounds.Y / 100.0)
		}
	}

	return ctx
}

// checkShapeGridStructural checks shape_grid cells for footer collision,
// bounds overflow, sparse layout, and approximable non-text visual issues
// using resolved cell positions from shapegrid.Resolve.
//
// Non-text grid findings approximable at preflight:
//   - grid_diagram_narrow: complex diagram in a narrow cell (same logic as render-time)
//
// Non-text grid findings that remain render-time-only:
//   - diagram_clamped: requires actual SVG render to know output dimensions
//   - diagram_render_failed: only knowable when rendering is attempted
//   - image file/dimension issues: require filesystem access and image decoding
//   - icon resolution failures: require loading SVG icons from the registry
func checkShapeGridStructural(grid *ShapeGridInput, slideIdx int, slideWidth, slideHeight int64, layout *types.LayoutMetadata, footerEnabled bool, patternName string) []patterns.FitFinding {
	if len(grid.Rows) == 0 {
		return nil
	}

	ctx := resolveGridContext(grid, layout, slideWidth, slideHeight, footerEnabled)

	// Resolve the grid to get authoritative cell bounds.
	result := resolveGridForStructural(grid, slideWidth, slideHeight)

	var findings []patterns.FitFinding

	if result != nil {
		// Walk resolved cells with row/col mapping.
		cellIdx := 0
		for ri, row := range grid.Rows {
			for ci, cell := range row.Cells {
				if cellIdx >= len(result.Cells) {
					break
				}
				if cell == nil || (cell.Shape == nil && cell.Table == nil && cell.Icon == nil && cell.Image == nil && cell.Diagram == nil) {
					cellIdx++
					continue
				}
				rc := result.Cells[cellIdx]
				path := slidepath.GridCell(slideIdx, ri, ci)
				findings = append(findings, checkCellStructural(path, slideIdx, rc.CellBounds.X, rc.CellBounds.Y, rc.CellBounds.CX, rc.CellBounds.CY, ctx)...)

				if cell.Diagram != nil {
					findings = append(findings,
						checkGridDiagramPreflight(cell.Diagram, slideIdx, ri, ci, rc.CellBounds.CX, rc.CellBounds.CY)...)
				}

				cellIdx++
			}
		}
	}

	// Sparse layout detection: bounds are authoritative (never shrink), so
	// content may occupy a small fraction of the allocated bounds.
	if f := detectSparseLayoutForGrid(grid, slideIdx, slideWidth, slideHeight, patternName); f != nil {
		findings = append(findings, *f)
	}

	return findings
}

// checkGridDiagramPreflight runs the diagram preflight detectors (narrow cell
// legibility, explicit-spec aspect mismatch, and natural-aspect conflict) for
// one resolved grid cell carrying a diagram. Extracted from
// checkShapeGridStructural to keep that function's cognitive complexity under
// the gocognit lint threshold.
func checkGridDiagramPreflight(diagram *types.DiagramSpec, slideIdx, ri, ci int, cellCX, cellCY int64) []patterns.FitFinding {
	if diagram == nil {
		return nil
	}
	diagPath := slidepath.GridCellField(slideIdx, ri, ci, "diagram")
	var findings []patterns.FitFinding
	if f := generator.CheckDiagramInNarrowBoundsFinding(diagram, cellCX, diagPath); f != nil {
		findings = append(findings, *f)
	}
	if f := generator.CheckDiagramAspectMismatchFinding(diagram, cellCX, cellCY, diagPath); f != nil {
		findings = append(findings, *f)
	}
	// Non-chart diagrams only — chart aspect issues come from svggen dry-render.
	if f := generator.CheckDiagramAspectConflictFinding(diagram, cellCX, cellCY, diagPath); f != nil {
		findings = append(findings, *f)
	}
	return findings
}

// resolveGridForStructural builds and resolves a shapegrid.Grid from a
// ShapeGridInput using default slide dimensions. Returns nil if resolution fails.
func resolveGridForStructural(grid *ShapeGridInput, slideWidth, slideHeight int64) *shapegrid.ResolveResult {
	colWidths, err := resolveColumnsDTO(grid.Columns, grid.Rows)
	if err != nil {
		return nil
	}

	colGap := grid.ColGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}

	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	bounds := shapegrid.DefaultBounds(slideWidth, slideHeight)
	if grid.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(
			grid.Bounds.X, grid.Bounds.Y,
			grid.Bounds.Width, grid.Bounds.Height,
			slideWidth, slideHeight,
		)
	}

	sgGrid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    convertGridRows(grid.Rows),
		ColGap:  colGap,
		RowGap:  rowGap,
	}

	if vErr := shapegrid.Validate(sgGrid); vErr != nil {
		return nil
	}

	alloc := pptx.NewShapeIDAllocator(nil)
	result, err := shapegrid.Resolve(sgGrid, alloc)
	if err != nil {
		return nil
	}
	return result
}

// detectSparseLayoutForGrid estimates the content height of a shape grid and
// compares it against the bounds height to detect mostly-empty slides.
func detectSparseLayoutForGrid(grid *ShapeGridInput, slideIdx int, slideWidth, slideHeight int64, patternName string) *patterns.FitFinding {
	boundsH := estimateGridBoundsHeightEMU(grid, slideHeight)
	if boundsH <= 0 {
		return nil
	}

	contentH := estimateGridContentHeightEMU(grid, slideHeight)
	if contentH <= 0 {
		return nil
	}

	// Count filled slots and grid dimensions for reshape recommendation.
	numCols := inferGridColumns(grid)
	numRows := len(grid.Rows)
	filledSlots := 0
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				filledSlots++
			}
		}
	}

	path := slidepath.ShapeGrid(slideIdx)
	return generator.DetectSparseLayout(generator.SparseLayoutInput{
		SlideIndex:       slideIdx,
		Path:             path,
		BoundsHeightEMU:  boundsH,
		ContentHeightEMU: contentH,
		PatternName:      patternName,
		FilledSlots:      filledSlots,
		GridRows:         numRows,
		GridCols:         numCols,
	})
}

// estimateGridBoundsHeightEMU computes the total allocated bounds height for a grid.
func estimateGridBoundsHeightEMU(grid *ShapeGridInput, slideHeight int64) int64 {
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}
	if grid.Bounds != nil && grid.Bounds.Height > 0 {
		return int64(float64(slideHeight) * grid.Bounds.Height / 100.0)
	}
	// Default: ~70% of slide height.
	return int64(float64(slideHeight) * 0.7)
}

// estimateGridContentHeightEMU estimates the total content height from row cells.
// It sums up per-row content estimates (tallest cell in each row) plus gaps.
func estimateGridContentHeightEMU(grid *ShapeGridInput, slideHeight int64) int64 {
	numRows := len(grid.Rows)
	if numRows == 0 {
		return 0
	}

	rowGapPt := grid.RowGap
	if rowGapPt == 0 {
		rowGapPt = grid.Gap
	}
	if rowGapPt == 0 {
		rowGapPt = 8 // default 8pt
	}
	rowGapEMU := int64(rowGapPt * 12700)

	var totalContentH int64
	for _, row := range grid.Rows {
		rowH := estimateRowInputContentHeightEMU(row)
		totalContentH += rowH
	}

	// Add inter-row gaps.
	totalContentH += rowGapEMU * int64(numRows-1)
	return totalContentH
}

// estimateRowInputContentHeightEMU returns the estimated content height for a
// GridRowInput based on the tallest cell's content.
func estimateRowInputContentHeightEMU(row GridRowInput) int64 {
	var maxH int64
	for _, cell := range row.Cells {
		h := estimateCellInputContentHeightEMU(cell)
		if h > maxH {
			maxH = h
		}
	}
	if maxH == 0 {
		maxH = int64(24 * 12700) // 24pt minimum fallback
	}
	return maxH
}

// estimateCellInputContentHeightEMU estimates content height for a single cell.
func estimateCellInputContentHeightEMU(cell *GridCellInput) int64 {
	if cell == nil {
		return 0
	}

	// Table cells: estimate from row count.
	if cell.Table != nil {
		rows := len(cell.Table.Rows)
		if rows == 0 {
			rows = 1
		}
		// ~20pt per row + 8pt header
		return int64((float64(rows)*20 + 8) * 12700)
	}

	// Shape cells: estimate from text content.
	if cell.Shape != nil && len(cell.Shape.Text) > 0 {
		return estimateShapeTextHeightEMU(cell.Shape.Text)
	}

	// Icon/image cells: ~40pt default.
	if cell.Icon != nil || cell.Image != nil {
		return int64(40 * 12700)
	}

	// Diagram cells: ~100pt default.
	if cell.Diagram != nil {
		return int64(100 * 12700)
	}

	return 0
}

// estimateShapeTextHeightEMU estimates the height of a shape's text content.
func estimateShapeTextHeightEMU(textRaw json.RawMessage) int64 {
	// Try string shorthand.
	var s string
	if json.Unmarshal(textRaw, &s) == nil {
		lines := strings.Count(s, "\n") + 1
		return cellTextHeightEMU(lines, 11)
	}

	// Try object form.
	var obj struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
	}
	if json.Unmarshal(textRaw, &obj) == nil && obj.Content != "" {
		fontSize := obj.Size
		if fontSize == 0 {
			fontSize = 11
		}
		lines := strings.Count(obj.Content, "\n") + 1
		return cellTextHeightEMU(lines, fontSize)
	}

	return 0
}

// cellTextHeightEMU computes text height in EMU from line count and font size.
func cellTextHeightEMU(lines int, fontSizePt float64) int64 {
	lineHeightPt := fontSizePt * 1.4
	totalPt := float64(lines)*lineHeightPt + 12 // 12pt for padding
	return int64(totalPt * 12700)
}

// checkCellStructural runs bounds overflow and footer collision on one cell.
func checkCellStructural(path string, slideIdx int, x, y, cx, cy int64, ctx gridContext) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if f := generator.DetectSlideBoundsOverflow(generator.BoundsCheckInput{
		SlideIndex:  slideIdx,
		Path:        path,
		X:           x,
		Y:           y,
		CX:          cx,
		CY:          cy,
		SlideWidth:  ctx.slideWidth,
		SlideHeight: ctx.slideHeight,
	}); f != nil {
		findings = append(findings, *f)
	}

	if ctx.footerEnabled && ctx.layoutDeclaresFooter {
		if f := generator.DetectFooterCollision(generator.FooterCollisionInput{
			SlideIndex:           slideIdx,
			Path:                 path,
			ShapeX:               x,
			ShapeY:               y,
			ShapeCX:              cx,
			ShapeCY:              cy,
			FooterY:              ctx.footerY,
			FooterCY:             ctx.footerCY,
			LayoutDeclaresFooter: true,
			StrictFit:            "warn",
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

// findLayoutForSlide resolves the layout metadata for a slide input.
func findLayoutForSlide(slide *SlideInput, layouts []types.LayoutMetadata) *types.LayoutMetadata {
	if slide.LayoutID == "" {
		return nil
	}
	for i := range layouts {
		if layouts[i].ID == slide.LayoutID {
			return &layouts[i]
		}
	}
	return nil
}

// findPlaceholderByID finds a placeholder by its ID within a layout.
func findPlaceholderByID(id string, phs []types.PlaceholderInfo) *types.PlaceholderInfo {
	for i := range phs {
		if phs[i].ID == id {
			return &phs[i]
		}
	}
	return nil
}

// extractContentParagraphs extracts text paragraphs from a content input.
func extractContentParagraphs(c *ContentInput) []string {
	switch c.Type {
	case "text":
		if c.TextValue != nil && *c.TextValue != "" {
			return []string{*c.TextValue}
		}
	case "bullets":
		if c.BulletsValue != nil {
			return *c.BulletsValue
		}
	case "body_and_bullets":
		if c.BodyAndBulletsValue != nil {
			var paras []string
			if c.BodyAndBulletsValue.Body != "" {
				paras = append(paras, c.BodyAndBulletsValue.Body)
			}
			paras = append(paras, c.BodyAndBulletsValue.Bullets...)
			if c.BodyAndBulletsValue.TrailingBody != "" {
				paras = append(paras, c.BodyAndBulletsValue.TrailingBody)
			}
			return paras
		}
	case "bullet_groups":
		if c.BulletGroupsValue != nil {
			var paras []string
			if c.BulletGroupsValue.Body != "" {
				paras = append(paras, c.BulletGroupsValue.Body)
			}
			for _, g := range c.BulletGroupsValue.Groups {
				if g.Header != "" {
					paras = append(paras, g.Header)
				}
				paras = append(paras, g.Bullets...)
			}
			if c.BulletGroupsValue.TrailingBody != "" {
				paras = append(paras, c.BulletGroupsValue.TrailingBody)
			}
			return paras
		}
	}
	return nil
}

// slideIndexFromPath extracts the slide index from a JSON Pointer path like "/slides/3/foo".
// Deprecated: use slidepath.SlideIndex directly. Kept as a package-level alias for callers
// in other files that reference it.
func slideIndexFromPath(path string) int {
	return slidepath.SlideIndex(path)
}

// findingCodeHistogram builds a sorted "code:count" list from suppressed
// patterns.FitFinding items, ordered by count descending.
func findingCodeHistogram(items []patterns.FitFinding) []string {
	counts := map[string]int{}
	for _, f := range items {
		counts[f.Code]++
	}
	return formatCodeCounts(counts)
}

// localFindingCodeHistogram builds a sorted "code:count" list from suppressed
// fitFinding items, ordered by count descending.
func localFindingCodeHistogram(items []fitFinding) []string {
	counts := map[string]int{}
	for _, f := range items {
		counts[f.Code]++
	}
	return formatCodeCounts(counts)
}

// formatCodeCounts formats a code→count map as a sorted "code:N" string slice,
// ordered by count descending, then code ascending for stability.
func formatCodeCounts(counts map[string]int) []string {
	type codeCount struct {
		code  string
		count int
	}
	pairs := make([]codeCount, 0, len(counts))
	for code, n := range counts {
		pairs = append(pairs, codeCount{code, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].code < pairs[j].code
	})
	result := make([]string, len(pairs))
	for i, p := range pairs {
		result[i] = fmt.Sprintf("%s:%d", p.code, p.count)
	}
	return result
}

// contrastSwapsToFindings converts generator ContrastSwap records into
// patterns.FitFinding values with action "info" and code "contrast_autofixed".
func contrastSwapsToFindings(swaps []generator.ContrastSwap) []patterns.FitFinding {
	if len(swaps) == 0 {
		return nil
	}
	findings := make([]patterns.FitFinding, 0, len(swaps))
	for _, s := range swaps {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Code: "contrast_autofixed",
				Message: fmt.Sprintf(
					"auto-fixed low-contrast text: %s → %s (on %s, ratio %.1f → %.1f)",
					s.OriginalColor, s.ReplacedColor, s.BackgroundColor,
					s.RatioBefore, s.RatioAfter,
				),
				Fix: &patterns.FixSuggestion{
					Kind: "replace_color",
					Params: map[string]any{
						"original_color":       s.OriginalColor,
						"replacement_color":    s.ReplacedColor,
						"background_color":     s.BackgroundColor,
						"contrast_ratio_before": s.RatioBefore,
						"contrast_ratio_after":  s.RatioAfter,
					},
				},
			},
			Action: "info",
		})
	}
	return findings
}

// patternRecommendedMax maps known patterns to their recommended maximum cell
// count. Patterns not listed here have no overcrowding limit enforced.
var patternRecommendedMax = map[string]int{
	"card-grid":           9,
	"icon-row":            5,
	"kpi-3up":             3,
	"kpi-4up":             4,
	"matrix-2x2":          4,
	"timeline-horizontal": 6,
	"process-flow":        8,
	"comparison-2col":     8,
	"before-after":        8,
}

// collectGridOccupancyFindings checks each slide's shape_grid for underfilled
// or overcrowded patterns, emitting pattern_underfilled / pattern_overcrowded.
func collectGridOccupancyFindings(input *PresentationInput) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range input.Slides {
		grid := slide.ShapeGrid
		if grid == nil || len(grid.Rows) == 0 {
			continue
		}

		// Determine column count.
		numCols := inferGridColumns(grid)

		// Count filled slots.
		totalSlots := len(grid.Rows) * numCols
		filledSlots := 0
		for _, row := range grid.Rows {
			for _, cell := range row.Cells {
				if cell != nil {
					filledSlots++
				}
			}
		}

		if totalSlots <= 0 {
			continue
		}

		// Determine pattern name.
		patternName := ""
		if slide.Pattern != nil {
			patternName = slide.Pattern.Name
		} else if slide.Compose != nil {
			patternName = "compose"
		}

		path := slidepath.ShapeGrid(si)

		// Check underfilled.
		if f := generator.DetectPatternUnderfilled(generator.GridOccupancyInput{
			SlideIndex:  si,
			Path:        path,
			PatternName: patternName,
			FilledSlots: filledSlots,
			TotalSlots:  totalSlots,
		}); f != nil {
			findings = append(findings, *f)
		}

		// Check overcrowded (only for known patterns with a recommended max).
		recMax := 0
		if patternName != "" {
			recMax = patternRecommendedMax[patternName]
		}
		if recMax > 0 {
			if f := generator.DetectPatternOvercrowded(generator.GridOccupancyInput{
				SlideIndex:     si,
				Path:           path,
				PatternName:    patternName,
				FilledSlots:    filledSlots,
				TotalSlots:     totalSlots,
				RecommendedMax: recMax,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}

	return findings
}

// inferGridColumns determines the column count from the grid's Columns field
// or infers from the maximum cells per row.
func inferGridColumns(grid *ShapeGridInput) int {
	if len(grid.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(grid.Columns, &n); err == nil {
			return int(n)
		}
		var arr []float64
		if err := json.Unmarshal(grid.Columns, &arr); err == nil {
			return len(arr)
		}
	}
	numCols := 0
	for _, row := range grid.Rows {
		if len(row.Cells) > numCols {
			numCols = len(row.Cells)
		}
	}
	return numCols
}

// allGridRowHeightsZero returns true when no row specifies an explicit height.
func allGridRowHeightsZero(rows []GridRowInput) bool {
	for _, r := range rows {
		if r.Height > 0 {
			return false
		}
	}
	return true
}

// rawGridSparseThreshold is the minimum ratio of content height to layout area
// height. Grids occupying less than 60% of the layout content area are sparse.
const rawGridSparseThreshold = 0.60

// detectSparseRawGrid emits a sparse_layout finding for raw
// grids whose auto-shrunk content height is less than 60% of the full layout
// content area height.
func detectSparseRawGrid(grid *ShapeGridInput, slideIdx int, slideWidth, slideHeight int64) *patterns.FitFinding {
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	// The full layout area height is what the grid bounds would be if it
	// filled the content zone (same logic as estimateGridBoundsHeightEMU).
	layoutAreaH := estimateGridBoundsHeightEMU(grid, slideHeight)
	if layoutAreaH <= 0 {
		return nil
	}

	// Content height is the actual rendered height after auto-shrink.
	contentH := estimateGridContentHeightEMU(grid, slideHeight)
	if contentH <= 0 {
		return nil
	}

	ratio := float64(contentH) / float64(layoutAreaH)
	if ratio >= rawGridSparseThreshold {
		return nil
	}

	path := slidepath.ShapeGrid(slideIdx)
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    path,
			Code:    patterns.ErrCodeSparseLayout,
			Message: fmt.Sprintf(
				"raw grid content occupies %.0f%% of layout area (%d / %d EMU) — consider explicit row heights or adopting a pattern",
				ratio*100, contentH, layoutAreaH,
			),
			Fix: &patterns.FixSuggestion{
				Kind: "adopt_pattern",
				Params: map[string]any{
					"filled_pct":        ratio,
					"content_height":    contentH,
					"layout_area_height": layoutAreaH,
				},
			},
		},
		Action: "review",
		Measured: &patterns.Extent{
			HeightEMU: contentH,
		},
		Allowed: &patterns.Extent{
			HeightEMU: layoutAreaH,
		},
		OverflowRatio: ratio,
	}
}

// =============================================================================
// Preflight predictors for render-time-only findings.
// =============================================================================

// collectTablePreflightFindings walks all content-level tables and shape_grid
// embedded tables, predicting render-time scaling/truncation/deficit findings
// using only the JSON content and template-resolved bounds.
func collectTablePreflightFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)

		// Content-level tables: bounds inherit from the placeholder.
		for ci, content := range slide.Content {
			if content.Type != "table" {
				continue
			}
			table := resolveTableFromContent(&content)
			if table == nil {
				continue
			}
			spec := table.ToTableSpec()
			bounds := tablePlaceholderBounds(content.PlaceholderID, layout)
			pathPrefix := slidepath.ContentIndex(si, ci)
			findings = append(findings, generator.DetectTablePreflight(generator.TablePreflightInput{
				Path:    pathPrefix,
				Headers: spec.Headers,
				Rows:    spec.Rows,
				Bounds:  bounds,
			})...)
		}

		// Shape-grid embedded tables: bounds derived from the resolved grid.
		if slide.ShapeGrid != nil {
			findings = append(findings, collectGridTablePreflight(slide.ShapeGrid, si)...)
		}
	}

	return findings
}

// tablePlaceholderBounds returns the bounds for a content-level table by
// looking up the placeholder in the layout. Returns a zero box when the
// placeholder can't be found — preflight then skips row/width checks.
func tablePlaceholderBounds(placeholderID string, layout *types.LayoutMetadata) types.BoundingBox {
	if layout == nil {
		return types.BoundingBox{}
	}
	if ph := findPlaceholderByID(placeholderID, layout.Placeholders); ph != nil {
		return ph.Bounds
	}
	return types.BoundingBox{}
}

// collectGridTablePreflight walks shape_grid cells and emits table preflight
// findings for any embedded tables, using the resolved cell bounds.
func collectGridTablePreflight(grid *ShapeGridInput, slideIdx int) []patterns.FitFinding {
	result := resolveGridForStructural(grid, 0, 0)
	if result == nil {
		return nil
	}

	var findings []patterns.FitFinding
	cellIdx := 0
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cellIdx >= len(result.Cells) {
				break
			}
			if cell == nil {
				cellIdx++
				continue
			}
			if cell.Table != nil {
				spec := cell.Table.ToTableSpec()
				rc := result.Cells[cellIdx]
				pathPrefix := slidepath.Join(slidepath.GridCell(slideIdx, ri, ci), "table")
				findings = append(findings, generator.DetectTablePreflight(generator.TablePreflightInput{
					Path:    pathPrefix,
					Headers: spec.Headers,
					Rows:    spec.Rows,
					Bounds: types.BoundingBox{
						X:      rc.CellBounds.X,
						Y:      rc.CellBounds.Y,
						Width:  rc.CellBounds.CX,
						Height: rc.CellBounds.CY,
					},
				})...)
			}
			cellIdx++
		}
	}
	return findings
}

// collectTextAutofitPreflightFindings walks placeholder content (body /
// content placeholders) and emits text_trimmed / readability_trimmed
// predictions. Title placeholders are excluded — those have their own
// DetectTitleWraps detector.
func collectTextAutofitPreflightFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)
		if layout == nil {
			continue
		}
		for _, content := range slide.Content {
			ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
			if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
				continue
			}
			if ph.Type != types.PlaceholderBody && ph.Type != types.PlaceholderContent {
				continue
			}
			paragraphs := extractContentParagraphs(&content)
			if len(paragraphs) == 0 {
				continue
			}
			path := slidepath.Content(si, content.PlaceholderID)
			findings = append(findings, generator.DetectTextAutofitPreflight(generator.TextAutofitPreflightInput{
				Path:        path,
				Paragraphs:  paragraphs,
				WidthEMU:    ph.Bounds.Width,
				HeightEMU:   ph.Bounds.Height,
				FontSizeHPt: ph.FontSize,
				FontName:    ph.FontFamily,
			})...)
		}
	}

	return findings
}

// collectContrastPreflightFindings walks shape_grid cells that author both a
// fill color and a text color, and emits contrast_predicted findings where
// the renderer would auto-replace the text color.
func collectContrastPreflightFindings(input *PresentationInput, themeColors []types.ThemeColor) []patterns.FitFinding {
	if len(themeColors) == 0 {
		return nil
	}

	var pairs []generator.ContrastPreflightPair
	for si, slide := range input.Slides {
		if slide.ShapeGrid == nil {
			continue
		}
		for ri, row := range slide.ShapeGrid.Rows {
			for ci, cell := range row.Cells {
				if cell == nil || cell.Shape == nil {
					continue
				}
				fill := extractShapeFillColor(cell.Shape.Fill)
				if fill == "" {
					continue
				}
				for _, tc := range extractShapeTextColors(cell.Shape.Text) {
					pairs = append(pairs, generator.ContrastPreflightPair{
						Path:       slidepath.GridCellField(si, ri, ci, "shape/text"),
						Foreground: tc,
						Background: fill,
						Source:     "shape_grid",
					})
				}
			}
		}
	}

	return generator.DetectContrastPreflight(pairs, themeColors)
}

// extractShapeFillColor extracts a color string from a shape fill RawMessage.
// Returns "" when fill is empty, "none", or unparseable.
func extractShapeFillColor(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// String form.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "none") {
			return ""
		}
		return s
	}
	// Object form.
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Color != "" {
		c := strings.TrimSpace(obj.Color)
		if strings.EqualFold(c, "none") {
			return ""
		}
		return c
	}
	return ""
}

// extractShapeTextColors extracts all authored text colors from a shape text
// RawMessage. A single object-form text contributes one color (if set); a
// paragraphs-array form contributes one color per paragraph that sets one.
// String-form text contributes nothing (no authored color).
func extractShapeTextColors(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	// String form has no color.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return nil
	}
	// Object / paragraphs-array form.
	var obj struct {
		Color      string `json:"color"`
		Paragraphs []struct {
			Color string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var colors []string
	if obj.Color != "" {
		colors = append(colors, obj.Color)
	}
	for _, p := range obj.Paragraphs {
		if p.Color != "" {
			colors = append(colors, p.Color)
		}
	}
	return colors
}

// expandComposeForPreflight returns a shallow copy of input where each slide
// whose Compose envelope has not yet been materialized into ShapeGrid has its
// ShapeGrid populated by running the compose merge. This lets preflight
// detectors (checkShapeGridStructural in particular) see post-merge cell
// geometry, so a diagram in a horizontally-narrowed compose segment trips
// grid_diagram_narrow and diagram_aspect_* findings rather than passing
// silently.
//
// The expansion uses a minimal ExpandContext (no template metadata, default
// accent strategy) because the structural detectors targeted by this pass
// depend only on cell rectangles. Compose envelopes that fail to expand are
// left as-is — render-time will surface the parse error separately, and we
// avoid masking it here.
//
// When no slide has an unexpanded compose envelope, the original input is
// returned unchanged (no allocation).
func expandComposeForPreflight(input *PresentationInput, slideWidth, slideHeight int64) *PresentationInput {
	if input == nil {
		return nil
	}
	needsExpansion := false
	for i := range input.Slides {
		s := &input.Slides[i]
		if s.Compose != nil && s.ShapeGrid == nil {
			needsExpansion = true
			break
		}
	}
	if !needsExpansion {
		return input
	}

	expanded := *input
	expanded.Slides = make([]SlideInput, len(input.Slides))
	copy(expanded.Slides, input.Slides)

	for i := range expanded.Slides {
		s := &expanded.Slides[i]
		if s.Compose == nil || s.ShapeGrid != nil {
			continue
		}
		ctx := patterns.ExpandContext{
			SlideWidth:  slideWidth,
			SlideHeight: slideHeight,
			SlideIndex:  i,
		}
		eg, _, err := expandCompose(s.Compose, ctx, patterns.Default())
		if err != nil {
			continue
		}
		s.ShapeGrid = eg
	}
	return &expanded
}

// dedupFitFindings removes findings that share the same
// (Code, Path, Action, Message) tuple. The first occurrence is kept, preserving
// caller insertion order. Same-code findings on different paths are kept,
// since they describe different cells. This guards against double-emission
// when pre-compose detectors and the post-compose structural pass both
// surface a finding for the same diagram cell.
func dedupFitFindings(in []patterns.FitFinding) []patterns.FitFinding {
	if len(in) <= 1 {
		return in
	}
	type key struct{ code, path, action, msg string }
	seen := make(map[key]struct{}, len(in))
	out := make([]patterns.FitFinding, 0, len(in))
	for _, f := range in {
		k := key{f.Code, f.Path, f.Action, f.Message}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, f)
	}
	return out
}
