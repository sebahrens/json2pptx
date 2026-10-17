package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

type deckGutter struct {
	slide     int
	gap       float64
	direction string
}

func collectGutterFindings(input *PresentationInput, layouts []types.LayoutMetadata, w, h int64) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	groups := map[string][]deckGutter{}
	for si, slide := range input.Slides {
		if !slideCarriesArgument(slide, layouts...) {
			continue
		}
		horizontal, vertical := automaticSlideGutters(slide, layoutAt(predicted, si), layouts, w, h)
		if gap, ok := horizontalGutter(horizontal); ok {
			groups["horizontal"] = append(groups["horizontal"], deckGutter{si, gap, "horizontal"})
		}
		if gap, ok := verticalGutter(vertical); ok {
			groups["vertical"] = append(groups["vertical"], deckGutter{si, gap, "vertical"})
		}
	}
	var out []patterns.FitFinding
	for _, direction := range []string{"horizontal", "vertical"} {
		group := groups[direction]
		if len(group) < 3 {
			continue
		}
		gaps := make([]float64, len(group))
		for i, g := range group {
			gaps[i] = g.gap
		}
		sort.Float64s(gaps)
		median := gaps[len(gaps)/2]
		for _, g := range group {
			if math.Abs(g.gap-median) > 4 {
				out = append(out, layoutReviewFinding(patterns.ErrCodeGutterInconsistent, slidepath.Slide(g.slide), "info", fmt.Sprintf("The automatic %s gutter is %.1fpt versus the deck median %.1fpt; use a consistent gutter or author an intentional gap.", direction, g.gap, median), map[string]any{"gutter_pt": g.gap, "median_gutter_pt": median, "tolerance_pt": 4, "slide": g.slide}))
			}
		}
	}
	return out
}

func horizontalGutter(boxes []pptx.RectEmu) (float64, bool) {
	if len(boxes) < 2 {
		return 0, false
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].X < boxes[j].X })
	if math.Abs(float64(boxes[0].Y-boxes[1].Y))/emuPerPt > 2 {
		return 0, false
	}
	gap := float64(boxes[1].X-boxes[0].X-boxes[0].CX) / emuPerPt
	return gap, gap >= 0
}

func verticalGutter(boxes []pptx.RectEmu) (float64, bool) {
	if len(boxes) < 2 {
		return 0, false
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].Y < boxes[j].Y })
	if math.Abs(float64(boxes[0].X-boxes[1].X))/emuPerPt > 2 {
		return 0, false
	}
	gap := float64(boxes[1].Y-boxes[0].Y-boxes[0].CY) / emuPerPt
	return gap, gap >= 0
}

func layoutAt(layouts []*types.LayoutMetadata, index int) *types.LayoutMetadata {
	if index < len(layouts) {
		return layouts[index]
	}
	return nil
}

func automaticSlideGutters(slide SlideInput, predicted *types.LayoutMetadata, layouts []types.LayoutMetadata, w, h int64) (horizontal, vertical []pptx.RectEmu) {
	if grid := slide.ShapeGrid; grid != nil {
		if grid.Bounds != nil || grid.Gap > 0 || grid.RowGap > 0 || grid.ColGap > 0 {
			return nil, nil
		}
		geom := resolveGridGeometry(slide, layouts, w, h)
		result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, w, h)
		if result == nil {
			return nil, nil
		}
		for _, cell := range result.Cells {
			if cell.Layer {
				continue
			}
			if cell.RowIdx == 0 {
				horizontal = append(horizontal, cell.CellBounds)
			}
			if cell.ColIdx == 0 {
				vertical = append(vertical, cell.CellBounds)
			}
		}
		return horizontal, vertical
	}
	if predicted == nil {
		return nil, nil
	}
	for _, item := range slide.Content {
		if !strings.HasPrefix(item.PlaceholderID, "body") {
			continue
		}
		if ph := findPlaceholderByID(item.PlaceholderID, predicted.Placeholders); ph != nil {
			b := ph.Bounds
			box := pptx.RectEmu{X: b.X, Y: b.Y, CX: b.Width, CY: b.Height}
			horizontal = append(horizontal, box)
			vertical = append(vertical, box)
		}
	}
	return horizontal, vertical
}
