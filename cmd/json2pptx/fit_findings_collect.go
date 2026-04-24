package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
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
func collectFitFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// 1. Text-fit findings from existing generateFitReport (tables + shape-grid text).
	for _, tf := range generateFitReport(input) {
		findings = append(findings, convertTextFitFinding(tf))
	}

	// 2. Structural findings using template layout data.
	findings = append(findings,
		collectStructuralFindings(input, layouts, slideWidth, slideHeight)...)

	// Sort by ActionRank desc, then slide index asc.
	sort.Slice(findings, func(i, j int) bool {
		ri := patterns.ActionRank(findings[i].Action)
		rj := patterns.ActionRank(findings[j].Action)
		if ri != rj {
			return ri > rj
		}
		return slideIndexFromPath(findings[i].Path) < slideIndexFromPath(findings[j].Path)
	})

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
		si := slideIndexFromPath(f.Path)
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
		suppressed := len(g.items) - budget
		path := fmt.Sprintf("slides[%d]", si)
		if si < 0 {
			path = "slides[?]"
		}
		result = append(result, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    "findings_truncated",
				Message: fmt.Sprintf("%d more findings suppressed on this slide; use verbose_fit to see all", suppressed),
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
		si := slideIndexFromPath(f.Path)
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
		suppressed := len(g.items) - budget
		path := fmt.Sprintf("slides[%d]", si)
		if si < 0 {
			path = "slides[?]"
		}
		result = append(result, fitFinding{
			Code:    "findings_truncated",
			Path:    path,
			Message: fmt.Sprintf("%d more findings suppressed on this slide; use --verbose-fit to see all", suppressed),
			Action:  "info",
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
			for _, content := range slide.Content {
				ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
				if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
					continue
				}

				paragraphs := extractContentParagraphs(&content)
				if len(paragraphs) == 0 {
					continue
				}

				path := fmt.Sprintf("slides[%d].content.%s", si, content.PlaceholderID)

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
		}

		// Shape grid: footer collision and bounds overflow.
		if slide.ShapeGrid != nil && layout != nil {
			findings = append(findings,
				checkShapeGridStructural(slide.ShapeGrid, si, slideWidth, slideHeight, layout, footerEnabled)...)
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

// checkShapeGridStructural checks shape_grid cells for footer collision and
// bounds overflow using estimated cell positions.
func checkShapeGridStructural(grid *ShapeGridInput, slideIdx int, slideWidth, slideHeight int64, layout *types.LayoutMetadata, footerEnabled bool) []patterns.FitFinding {
	if len(grid.Rows) == 0 {
		return nil
	}

	ctx := resolveGridContext(grid, layout, slideWidth, slideHeight, footerEnabled)
	var findings []patterns.FitFinding

	for ri, row := range grid.Rows {
		cellWidth := estimateCellWidthEMU(grid, 0)
		cellHeight := estimateCellHeightEMU(grid, ri)
		cellY := ctx.gridY + int64(ri)*cellHeight

		for ci, cell := range row.Cells {
			if cell == nil || (cell.Shape == nil && cell.Table == nil) {
				continue
			}
			path := fmt.Sprintf("slides[%d].shape_grid.rows[%d].cells[%d]", slideIdx, ri, ci)
			cellX := ctx.gridX + int64(ci)*cellWidth
			findings = append(findings, checkCellStructural(path, slideIdx, cellX, cellY, cellWidth, cellHeight, ctx)...)
		}
	}

	return findings
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

// slideIndexFromPath extracts the slide index from a JSON path like "slides[3].foo".
func slideIndexFromPath(path string) int {
	if !strings.HasPrefix(path, "slides[") {
		return -1
	}
	rest := path[len("slides["):]
	bracket := strings.IndexByte(rest, ']')
	if bracket < 0 {
		return -1
	}
	idx, err := strconv.Atoi(rest[:bracket])
	if err != nil {
		return -1
	}
	return idx
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
