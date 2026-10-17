package main

import (
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func projectRhythmGeometry(s SlideInput, rs *rhythm.Slide) {
	rs.PaneCount = 1
	if s.Compose != nil {
		projectComposeGeometry(s.Compose, rs)
		return
	}
	if rs.Grid != nil {
		projectRhythmRawGrid(s.ShapeGrid, rs)
		return
	}
	left := layout.DerivedColumnLeftPercent(s.LayoutID)
	if left == 0 {
		left = layout.DerivedColumnLeftPercent(s.SlideType)
	}
	if left == 0 {
		left = s.ColumnLeftPercent
	}
	body, second, visual := false, false, ""
	for _, c := range s.Content {
		if c.PlaceholderID == "body" {
			body = true
		}
		if c.PlaceholderID == "body_2" {
			second = true
		}
		if c.Type == "chart" || c.Type == "image" || c.Type == "diagram" {
			if c.PlaceholderID == "body_2" {
				visual = "right"
			} else if c.PlaceholderID == "body" {
				visual = "left"
			}
		}
	}
	if body && second {
		rs.PaneCount, rs.VisualSide = 2, visual
		if left > 0 {
			rs.SplitPct = float64(left)
			if visual == "left" {
				rs.VisualAreaShare = float64(left) / 100
			} else if visual == "right" {
				rs.VisualAreaShare = 1 - float64(left)/100
			}
		}
	} else if rs.Motif == "chart" || rs.Motif == "image" || rs.Motif == "diagram" {
		rs.VisualSide, rs.VisualAreaShare = "full", 1
	}
}

func projectComposeGeometry(c *ComposeInput, rs *rhythm.Slide) {
	rs.PaneCount, rs.VisualAreaShare, rs.SplitPct, rs.VisualSide = 0, 0, 0, ""
	shares := resolveSegmentSizes(c.Segments)
	known := composeGeometryKnown(c)
	if len(shares) > 0 && known {
		rs.SplitPct = shares[0]
	}
	visuals, position := 0, 0
	for i, seg := range c.Segments {
		sub := rhythm.Slide{PaneCount: 1}
		switch {
		case seg.Compose != nil:
			projectComposeGeometry(seg.Compose, &sub)
		case seg.ShapeGrid != nil:
			sub.Grid = buildDensityGrid(seg.ShapeGrid)
			projectRhythmRawGrid(seg.ShapeGrid, &sub)
		case seg.Image != nil || seg.Table != nil || seg.HasDiagram() || visualPattern(seg.Pattern.Name):
			sub.VisualAreaShare = 1
			sub.VisualSide = "full"
		}
		rs.PaneCount += sub.PaneCount
		if sub.VisualSide != "" {
			visuals++
			position = i
			if known {
				rs.VisualAreaShare += shares[i] / 100 * sub.VisualAreaShare
			}
		}
	}
	if visuals == 1 {
		if c.Direction == "vertical" {
			if position == 0 {
				rs.VisualSide = "top"
			} else {
				rs.VisualSide = "bottom"
			}
		} else {
			if position == 0 {
				rs.VisualSide = "left"
			} else {
				rs.VisualSide = "right"
			}
		}
	} else if visuals > 1 {
		rs.VisualSide = "full"
	}
}
func visualPattern(name string) bool {
	return strings.Contains(name, "chart") || name == "image-text-split" || strings.HasPrefix(name, "cycle-")
}

// Walk DTO descendants against each resolved parent frame. Container cells
// reserve space; only their native leaves contribute panes and visual area.
func projectRhythmRawGrid(input *ShapeGridInput, rs *rhythm.Slide) {
	if rs.Grid == nil {
		return
	}
	root := rs.Grid.Bounds
	rs.PaneCount, rs.VisualAreaShare, rs.VisualSide = 0, 0, ""
	panes, visuals := rhythmGridLeaves(input, rs.Grid, 0)
	rs.PaneCount = panes
	rs.PaneCount = max(rs.PaneCount, 1)
	if root.CX <= 0 || root.CY <= 0 {
		return
	}
	for _, box := range visuals {
		box = intersectRect(box, root)
		rs.VisualAreaShare += float64(box.CX) * float64(box.CY) / (float64(root.CX) * float64(root.CY))
	}
	rs.VisualAreaShare = math.Min(rs.VisualAreaShare, 1)
	if len(visuals) == 1 {
		rs.VisualSide = rhythmVisualSide(root, visuals[0])
		if rs.VisualSide == "left" || rs.VisualSide == "right" {
			rs.SplitPct = 100 * float64(visuals[0].CX) / float64(root.CX)
		} else {
			rs.SplitPct = 100 * float64(visuals[0].CY) / float64(root.CY)
		}
	} else if len(visuals) > 1 {
		rs.VisualSide = "full"
	}
}
func rhythmVisualSide(root, box pptx.RectEmu) string {
	x := float64(box.X+box.CX/2-root.X)/float64(root.CX) - .5
	y := float64(box.Y+box.CY/2-root.Y)/float64(root.CY) - .5
	switch {
	case math.Abs(x) < .1 && math.Abs(y) < .1:
		return "full"
	case math.Abs(x) >= math.Abs(y) && x < 0:
		return "left"
	case math.Abs(x) >= math.Abs(y):
		return "right"
	case y < 0:
		return "top"
	default:
		return "bottom"
	}
}

func composeGeometryKnown(c *ComposeInput) bool {
	if c.SmartCompose || composeHasContentSize(c) || c.Banner != nil || c.Callout != nil {
		return false
	}
	for _, seg := range c.Segments {
		if seg.Compose != nil && !composeGeometryKnown(seg.Compose) {
			return false
		}
	}
	return true
}

func rhythmGridLeaves(src *ShapeGridInput, g *shapegrid.Grid, depth int) (panes int, visuals []pptx.RectEmu) {
	if g == nil || depth > maxGeomNestingDepth {
		return
	}
	result, err := shapegrid.Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		return
	}
	for _, cell := range result.Cells {
		if cell.Layer {
			continue
		}
		if cell.Kind == shapegrid.CellKindSubGrid {
			child := gridCellAtResolved(src, cell.RowIdx, cell.ColIdx)
			if child != nil && child.Grid != nil {
				box := cell.Bounds
				box.X += subGridInsetEMU
				box.Y += subGridInsetEMU
				box.CX -= 2 * subGridInsetEMU
				box.CY -= 2 * subGridInsetEMU
				sub := structuralGrid(child.Grid, &box, nil, shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
				n, found := rhythmGridLeaves(child.Grid, sub, depth+1)
				panes += n
				visuals = append(visuals, found...)
			}
			continue
		}
		if cell.Kind == shapegrid.CellKindShape && (cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0) {
			continue
		}
		panes++
		if cell.Kind == shapegrid.CellKindImage || cell.Kind == shapegrid.CellKindDiagram || cell.Kind == shapegrid.CellKindTable {
			visuals = append(visuals, cell.Bounds)
		}
	}
	return
}
