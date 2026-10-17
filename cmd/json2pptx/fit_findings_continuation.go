package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectContinuationUnderfilledFindings measures native table rows, including
// wrapping/header geometry, in the actual continuation's content frame.
func collectContinuationUnderfilledFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	font := ""
	if theme != nil {
		font = theme.BodyFont
	}
	fittedLayouts := predictSlideLayouts(input, layouts)
	grid := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)
	findings := []patterns.FitFinding{}
	for si, slide := range input.Slides {
		_, title := extractTitleText(slide)
		if title == "" {
			title = slide.Headline
		}
		_, marked := rhythm.ParseContinuation(title)
		if !marked && (slide.Origin == nil || !slide.Origin.SplitPage) {
			continue
		}
		var layout *types.LayoutMetadata
		if si < len(fittedLayouts) {
			layout = fittedLayouts[si]
		}
		for ci, content := range slide.Content {
			if content.Type != "table" {
				continue
			}
			table := resolveTableFromContent(&content)
			if table == nil {
				continue
			}
			bounds := tablePlaceholderBounds(content.PlaceholderID, layout)
			bounds = continuationTableBounds(bounds, slide, layout, layouts, slideWidth, slideHeight)
			fontSize := 0
			if content.FontSize != nil {
				fontSize = int(*content.FontSize * 100)
			}
			if f := continuationTableUnderfilled(table, bounds, contentTablePath(&content, si, ci), fontSize, font); f != nil {
				findings = append(findings, *f)
			}
		}
		if slide.ShapeGrid != nil {
			geom, _ := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, grid)
			if result := resolveGridForStructural(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight); result != nil {
				findings = append(findings, continuationGridTableFindings(slide.ShapeGrid, result, slidepath.ShapeGrid(si), font, 0)...)
			}
		}
	}
	return findings
}

func continuationTableUnderfilled(table *TableInput, bounds types.BoundingBox, path string, fontSize int, font string) *patterns.FitFinding {
	if table == nil || len(table.Headers) == 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}
	height := generator.MeasureTableHeight(table.ToTableSpec(), bounds.Width, fontSize, font)
	if height <= 0 || height >= bounds.Height/2 {
		return nil
	}
	share := 100 * float64(height) / float64(bounds.Height)
	return &patterns.FitFinding{ValidationError: patterns.ValidationError{Path: path, Code: patterns.ErrCodeContinuationUnderfilled,
		Message: fmt.Sprintf("continuation table uses %.0f%% of its available height (%.0f of %.0fpt); merge or rebalance its pages while preserving merged row groups", share, float64(height)/12700, float64(bounds.Height)/12700),
		Fix:     &patterns.FixSuggestion{Kind: "review_layout", Params: map[string]any{"suggestion": "merge or rebalance continuation rows", "used_height_pct": share, "preserve_row_spans": true}}}, Action: "review", Measured: &patterns.Extent{HeightEMU: height}, Allowed: &patterns.Extent{HeightEMU: bounds.Height}}
}

func continuationGridTableFindings(grid *ShapeGridInput, result *shapegrid.ResolveResult, base, font string, depth int) []patterns.FitFinding {
	findings := []patterns.FitFinding{}
	for _, rc := range result.Cells {
		cell := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		if cell == nil {
			continue
		}
		path := authoredGridCellPath(base, grid, rc.RowIdx, rc.ColIdx)
		if rc.Kind == shapegrid.CellKindTable && cell.Table != nil {
			bounds := types.BoundingBox{X: rc.CellBounds.X, Y: rc.CellBounds.Y, Width: rc.CellBounds.CX, Height: rc.CellBounds.CY}
			if f := continuationTableUnderfilled(cell.Table, bounds, path+"/table", 0, font); f != nil {
				findings = append(findings, *f)
			}
		}
		if rc.Kind == shapegrid.CellKindSubGrid && cell.Grid != nil && depth < maxGeomNestingDepth {
			bounds := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
			if bounds.CX <= 0 || bounds.CY <= 0 {
				bounds = rc.Bounds
			}
			if sub := resolveGridForStructural(cell.Grid, &bounds, nil, 0, 0, result.SubGridPeers); sub != nil {
				findings = append(findings, continuationGridTableFindings(cell.Grid, sub, path+"/grid", font, depth+1)...)
			}
		}
	}
	return findings
}

func continuationTableBounds(bounds types.BoundingBox, slide SlideInput, layout *types.LayoutMetadata, layouts []types.LayoutMetadata, width, height int64) types.BoundingBox {
	if layout == nil {
		return bounds
	}
	frame := slideChromeFrame(slide, layout.ID, layouts, width, height)
	if slide.Takeaway == "" && slide.Source == "" {
		frame.Fits = false
	}
	return clipDerivedColumnToChrome(bounds, frame)
}
