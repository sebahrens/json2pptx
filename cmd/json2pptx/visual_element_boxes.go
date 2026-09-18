package main

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// deckGeometry carries the template geometry used to reproduce generation's
// element bounds outside the render pipeline. A nil *deckGeometry (or zero
// dimensions) falls back to the default 16:9 slide and generic grid bounds.
type deckGeometry struct {
	Layouts     []types.LayoutMetadata
	SlideWidth  int64
	SlideHeight int64
	Metadata    *types.TemplateMetadata
}

// loadDeckGeometry best-effort resolves the deck template's geometry. It
// returns nil on any failure so callers degrade to default bounds.
func loadDeckGeometry(templateName, templatesDir string) *deckGeometry {
	if templateName == "" {
		return nil
	}
	path, cleanup, err := resolveTemplatePath(templateName, templatesDir)
	if err != nil {
		return nil
	}
	defer cleanup()
	reader, err := template.OpenTemplate(path)
	if err != nil {
		return nil
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil
	}
	w, h := template.ParseSlideDimensions(reader)
	return &deckGeometry{Layouts: layouts, SlideWidth: w, SlideHeight: h}
}

// visualElementBoxes reproduces generation's shape_grid geometry for one slide
// and returns each rendered cell's bounds in normalized slide coordinates,
// keyed by the cell's JSON Pointer (/slides/N/shape_grid/rows/R/cells/C).
// Pattern slides are expanded first, so pattern cells (e.g. KPI cards) are
// addressable too. Slides without a grid return nil.
func visualElementBoxes(input *PresentationInput, slideIdx int, geom *deckGeometry) []visualqa.ElementBox {
	if input == nil || slideIdx < 0 || slideIdx >= len(input.Slides) {
		return nil
	}
	var g deckGeometry
	if geom != nil {
		g = *geom
	}
	if g.SlideWidth <= 0 {
		g.SlideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if g.SlideHeight <= 0 {
		g.SlideHeight = shapegrid.DefaultSlideHeightEMU
	}

	slide := input.Slides[slideIdx]
	grid := slide.ShapeGrid
	if grid == nil && slide.Pattern != nil {
		ctx := patterns.ExpandContext{
			Metadata:       g.Metadata,
			SlideWidth:     g.SlideWidth,
			SlideHeight:    g.SlideHeight,
			AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
			SlideIndex:     slideIdx,
		}
		expanded, _, err := expandPattern(slide.Pattern, ctx, patterns.Default())
		if err != nil {
			return nil
		}
		grid = expanded
		slide.ShapeGrid = expanded
	}
	if grid == nil || len(grid.Rows) == 0 {
		return nil
	}

	gg := resolveGridGeometry(slide, g.Layouts, g.SlideWidth, g.SlideHeight)
	result := resolveGridForStructural(grid, gg.OverrideBounds, gg.Zone, g.SlideWidth, g.SlideHeight)
	if result == nil {
		return nil
	}

	arrayIdx := gridColumnToCellIndex(grid)
	sw, sh := float64(g.SlideWidth), float64(g.SlideHeight)
	seen := make(map[string]int)
	var boxes []visualqa.ElementBox
	for _, rc := range result.Cells {
		ci, ok := arrayIdx[[2]int{rc.RowIdx, rc.ColIdx}]
		if !ok {
			continue
		}
		b := rc.CellBounds
		box := visualqa.BBox{X: float64(b.X) / sw, Y: float64(b.Y) / sh, W: float64(b.CX) / sw, H: float64(b.CY) / sh}
		path := slidepath.GridCell(slideIdx, rc.RowIdx, ci)
		// Composite cells resolve to two rects sharing one source cell; keep a
		// single box that spans both.
		if prev, dup := seen[path]; dup {
			boxes[prev].Box = unionBox(boxes[prev].Box, box)
			continue
		}
		seen[path] = len(boxes)
		boxes = append(boxes, visualqa.ElementBox{Path: path, Box: box})
	}
	return boxes
}

// gridColumnToCellIndex maps each (row, grid column) a shape_grid cell starts
// at to that cell's index in row.cells — mirroring shapegrid.Resolve's column
// walk (row-span occupancy, col-span advance, empty cells consuming a column).
func gridColumnToCellIndex(grid *ShapeGridInput) map[[2]int]int {
	numCols := gridColumnCount(grid)
	occupied := make([][]bool, len(grid.Rows))
	for r := range occupied {
		occupied[r] = make([]bool, numCols)
	}
	out := make(map[[2]int]int)
	for r, row := range grid.Rows {
		col := 0
		for ci, c := range row.Cells {
			for col < numCols && occupied[r][col] {
				col++
			}
			if col >= numCols {
				break
			}
			out[[2]int{r, col}] = ci
			if isEmptyGridCell(c) {
				col++ // empty cells consume one column and claim no span
				continue
			}
			colSpan, rowSpan := cellSpans(c)
			markOccupied(occupied, r, col, rowSpan, colSpan)
			col += colSpan
		}
	}
	return out
}

// gridColumnCount returns the grid's column count: the resolved columns when
// declared, else the widest row's span sum.
func gridColumnCount(grid *ShapeGridInput) int {
	if cols, err := resolveColumnsDTO(grid.Columns, grid.Rows); err == nil && len(cols) > 0 {
		return len(cols)
	}
	numCols := 0
	for _, row := range grid.Rows {
		n := 0
		for _, c := range row.Cells {
			span, _ := cellSpans(c)
			n += span
		}
		numCols = max(numCols, n)
	}
	return numCols
}

func cellSpans(c *GridCellInput) (colSpan, rowSpan int) {
	colSpan, rowSpan = 1, 1
	if c != nil && c.ColSpan > 1 {
		colSpan = c.ColSpan
	}
	if c != nil && c.RowSpan > 1 {
		rowSpan = c.RowSpan
	}
	return colSpan, rowSpan
}

func isEmptyGridCell(c *GridCellInput) bool {
	return c == nil || (c.Shape == nil && c.Table == nil && c.Icon == nil && c.Image == nil && c.Diagram == nil && c.Composite == nil && c.Grid == nil)
}

func markOccupied(occupied [][]bool, r, col, rowSpan, colSpan int) {
	for dr := 0; dr < rowSpan && r+dr < len(occupied); dr++ {
		for dc := 0; dc < colSpan && col+dc < len(occupied[r+dr]); dc++ {
			occupied[r+dr][col+dc] = true
		}
	}
}

func unionBox(a, b visualqa.BBox) visualqa.BBox {
	x0, y0 := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	x1, y1 := math.Max(a.X+a.W, b.X+b.W), math.Max(a.Y+a.H, b.Y+b.H)
	return visualqa.BBox{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
