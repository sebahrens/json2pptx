package shapegrid

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// resolveRowConnectors generates the auto-connectors for rows that carry a
// connector spec.
//
// Semantics:
//
//   - A row's connector chain runs through every visible cell that OCCUPIES the
//     row, in left-to-right order — including cells that start in an earlier
//     row and span into this one (e.g. a driver-tree branch spanning its leaf
//     rows). Previously only cells originating in the row were chained, so a
//     spanning parent connected to its first child only and later rows chained
//     children to each other or to the far column.
//   - Spacer cells are skipped: shapes with no visible fill and no outline
//     (empty placeholders, text-only labels). A connector ending on an
//     invisible box edge floats in space.
//   - A pair (A, B) is connected in row r only when B starts in row r. A
//     spanning parent therefore fans out to each child exactly once, and a
//     cell spanning several rows is never re-connected from every row it
//     covers.
//   - Cells in a row are connected side edge to side edge. When the two
//     endpoints sit at different heights (a spanning parent's centre vs. a
//     child's centre) the connector is routed as an elbow instead of a
//     diagonal line cutting through neighbouring cells.
func resolveRowConnectors(grid *Grid, cells []ResolvedCell, rowCellIDs [][]int,
	rowYOffsets, rowHeightsEMU []int64, alloc *pptx.ShapeIDAllocator) []ResolvedConnector {

	hasConnector := false
	for _, row := range grid.Rows {
		if row.Connector != nil {
			hasConnector = true
			break
		}
	}
	if !hasConnector {
		return nil
	}

	// Connector anchors: every registered cell (composite diagram halves are
	// not registered in rowCellIDs and never act as anchors).
	var anchors []int
	for _, ids := range rowCellIDs {
		anchors = append(anchors, ids...)
	}

	type pair struct{ a, b int }
	seen := make(map[pair]bool)

	var connectors []ResolvedConnector
	for r, row := range grid.Rows {
		if row.Connector == nil {
			continue
		}
		top := rowYOffsets[r]
		bottom := top + rowHeightsEMU[r]

		// Visible cells occupying row r, ordered left to right.
		var members []int
		for _, idx := range anchors {
			c := cells[idx]
			cb := c.CellBounds
			if cb.Y >= bottom || cb.Y+cb.CY <= top {
				continue
			}
			if !isConnectable(c) {
				continue
			}
			members = append(members, idx)
		}
		sort.SliceStable(members, func(i, j int) bool {
			return cells[members[i]].CellBounds.X < cells[members[j]].CellBounds.X
		})

		for i := 0; i+1 < len(members); i++ {
			a, b := members[i], members[i+1]
			if cells[b].RowIdx != r {
				continue // target started in an earlier row: already connected there
			}
			if seen[pair{a, b}] {
				continue
			}
			seen[pair{a, b}] = true

			srcCell, tgtCell := cells[a], cells[b]
			srcOpts := pptx.ShapeOptions{Bounds: srcCell.Bounds}
			tgtOpts := pptx.ShapeOptions{Bounds: tgtCell.Bounds}
			if srcCell.ShapeSpec != nil {
				srcOpts.Geometry = pptx.PresetGeometry(srcCell.ShapeSpec.Geometry)
			}
			if tgtCell.ShapeSpec != nil {
				tgtOpts.Geometry = pptx.PresetGeometry(tgtCell.ShapeSpec.Geometry)
			}
			route := pptx.Route(srcOpts, tgtOpts, true)

			connectors = append(connectors, ResolvedConnector{
				Bounds:    route.Bounds,
				ID:        alloc.Alloc(),
				Spec:      row.Connector,
				SourceID:  srcCell.ID,
				TargetID:  tgtCell.ID,
				StartSite: route.StartSite,
				EndSite:   route.EndSite,
				Elbow:     elbowNeeded(route),
				FlipH:     route.FlipH,
				FlipV:     route.FlipV,
			})
		}
	}
	return connectors
}

// elbowThresholdEMU is the vertical offset (1pt) above which a side-to-side
// connector is drawn as an elbow rather than a straight line.
const elbowThresholdEMU = 12700

func elbowNeeded(r pptx.ConnectorRoute) bool {
	dy := r.EndY - r.StartY
	if dy < 0 {
		dy = -dy
	}
	return dy > elbowThresholdEMU
}

// isConnectable reports whether a resolved cell is a visible anchor for an
// auto-connector. Non-shape content (images, icons, tables, diagrams,
// sub-grids) always is; a shape needs a visible fill, a visible outline, or
// an icon overlay.
func isConnectable(c ResolvedCell) bool {
	if c.Kind != CellKindShape {
		return true
	}
	if c.IconSpec != nil {
		return true
	}
	s := c.ShapeSpec
	if s == nil {
		return false
	}
	return isVisibleFill(s.Fill) || isVisibleLine(s.Line)
}

// isVisibleFill reports whether a shape fill value paints anything.
func isVisibleFill(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return !isNoneColor(s)
	}
	var obj struct {
		Color string  `json:"color"`
		Alpha float64 `json:"alpha"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return true // unknown shape: assume visible
	}
	return !isNoneColor(obj.Color)
}

// isVisibleLine reports whether a shape outline value draws a line.
func isVisibleLine(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return !isNoneColor(s)
	}
	var obj struct {
		Color string   `json:"color"`
		Width *float64 `json:"width"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return true
	}
	if obj.Width != nil && *obj.Width <= 0 {
		return false
	}
	return !isNoneColor(obj.Color)
}

func isNoneColor(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "" || s == "none" || s == "transparent"
}
