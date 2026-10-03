package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// gridPathMapper rewrites the JSON path a generate-time grid finding or
// refusal carries, so it names the path validate reports
// (go-slide-creator-epch2).
//
// Generation pins a cell's findings at the resolved cell,
// /slides/{s}/shape_grid/rows/{row}/cells/{column}, and resolves a nested
// sub-grid (an authored `grid` cell, a nested compose segment) as if it were
// the slide's grid. Validate walks the JSON: the cell's index in its row
// (which differs from its column after a column-spanning cell, as every
// horizontal compose segment after a multi-column one does) and the full
// nested path .../cells/{i}/grid/rows/{r}/cells/{i}/.... So a native diagram
// in a nested compose segment reported TEXT_EXCEEDS_SHAPE twice — once at
// validate's path and once at a top-level cell. Each grid maps its own cells
// with authoredCellPaths and each nesting level re-roots its sub-grid with
// nestedGridPaths, so the path is right at any depth.
type gridPathMapper func(path string) string

// authoredGridCellPath is base + "/rows/{row}/cells/{i}" for the resolved cell
// at (row, col), where i indexes the cell the author wrote in grid.Rows[row].
func authoredGridCellPath(base string, grid *ShapeGridInput, row, col int) string {
	if idx, ok := gridCellIndexAtResolved(grid, row, col); ok {
		col = idx
	}
	return fmt.Sprintf("%s/rows/%d/cells/%d", base, row, col)
}

// authoredCellPaths maps input's resolved cell paths, as generation writes
// them for the slide's grid, to the authored cell index.
func authoredCellPaths(input *ShapeGridInput, slideIdx int) gridPathMapper {
	base := slidepath.ShapeGrid(slideIdx)
	return func(path string) string {
		row, col, rest, ok := splitGridCellPath(path, base+"/rows/")
		if !ok {
			return path
		}
		return authoredGridCellPath(base, input, row, col) + rest
	}
}

// nestedGridPaths re-roots a sub-grid's paths (resolved as the slide's grid)
// under parentCell, the authored path of the cell hosting it.
func nestedGridPaths(slideIdx int, parentCell string) gridPathMapper {
	root := slidepath.ShapeGrid(slideIdx)
	return func(path string) string {
		if !slidepath.HasPrefix(path, root) {
			return path
		}
		return parentCell + "/grid" + path[len(root):]
	}
}

// findings rewrites each finding's path in place.
func (m gridPathMapper) findings(fs []patterns.FitFinding) {
	for i := range fs {
		fs[i].Path = m(fs[i].Path)
	}
}

// refusal rewrites the path a generation refusal carries — a native region
// too small for its diagram (DIAGRAM_REGION_TOO_SMALL) or a source-loss
// ValidationError — so a refused cell names the path validate predicted.
func (m gridPathMapper) refusal(err error) {
	var capacity *generator.NativeRegionCapacityError
	if errors.As(err, &capacity) && capacity != nil {
		capacity.Finding.Path = m(capacity.Finding.Path)
	}
	var loss *patterns.ValidationError
	if errors.As(err, &loss) && loss != nil {
		loss.Path = m(loss.Path)
	}
}

// splitGridCellPath parses prefix + "{row}/cells/{col}" + rest.
func splitGridCellPath(path, prefix string) (row, col int, rest string, ok bool) {
	s, found := strings.CutPrefix(path, prefix)
	if !found {
		return 0, 0, "", false
	}
	rowStr, s, found := strings.Cut(s, "/cells/")
	if !found {
		return 0, 0, "", false
	}
	colStr := s
	if j := strings.IndexByte(s, '/'); j >= 0 {
		colStr, rest = s[:j], s[j:]
	}
	row, err1 := strconv.Atoi(rowStr)
	col, err2 := strconv.Atoi(colStr)
	if err1 != nil || err2 != nil {
		return 0, 0, "", false
	}
	return row, col, rest, true
}
