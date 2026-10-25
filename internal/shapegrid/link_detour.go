package shapegrid

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

type linkPoint struct{ x, y int64 }

func linkObstacle(c ResolvedCell) bool {
	return isConnectable(c) || (c.ShapeSpec != nil && hasNonEmptyText(c.ShapeSpec.Text))
}

func linkPathClear(points []linkPoint, cells []ResolvedCell, a, b int) bool {
	for i, c := range cells {
		if i == a || i == b || !linkObstacle(c) {
			continue
		}
		// Layers belong to their host endpoint, not to an unrelated obstacle.
		if c.Layer && ((c.RowIdx == cells[a].RowIdx && c.ColIdx == cells[a].ColIdx) || (c.RowIdx == cells[b].RowIdx && c.ColIdx == cells[b].ColIdx)) {
			continue
		}
		r := c.Bounds
		for j := 1; j < len(points); j++ {
			p, q := points[j-1], points[j]
			if p.x == q.x {
				if p.x > r.X && p.x < r.X+r.CX && max(p.y, q.y) > r.Y && min(p.y, q.y) < r.Y+r.CY {
					return false
				}
			} else if p.y == q.y {
				if p.y > r.Y && p.y < r.Y+r.CY && max(p.x, q.x) > r.X && min(p.x, q.x) < r.X+r.CX {
					return false
				}
			} else if max(p.x, q.x) > r.X && min(p.x, q.x) < r.X+r.CX && max(p.y, q.y) > r.Y && min(p.y, q.y) < r.Y+r.CY {
				return false // conservative diagonal envelope; choose a checked orthogonal detour
			}
		}
	}
	return true
}

func linkRouteBlocked(route pptx.ConnectorRoute, elbow bool, cells []ResolvedCell, a, b int) bool {
	points := []linkPoint{{route.StartX, route.StartY}}
	if elbow {
		mid := (route.StartX + route.EndX) / 2
		points = append(points, linkPoint{mid, route.StartY}, linkPoint{mid, route.EndY})
	}
	points = append(points, linkPoint{route.EndX, route.EndY})
	return !linkPathClear(points, cells, a, b)
}

// Try the free lanes beside cell outlines. Every candidate is checked against
// every occupied cell and the actual grid frame, including its short stubs.
// Three native straight segments render consistently in Office and LibreOffice.
func routeLinkDetour(grid *Grid, cells []ResolvedCell, a, b int, spec *ConnectorSpec, alloc *pptx.ShapeIDAllocator) ([]ResolvedConnector, bool) {
	const pad = 6 * 12700
	frame := grid.Bounds
	xs := []int64{frame.X + pad, frame.X + frame.CX - pad}
	ys := []int64{frame.Y + pad, frame.Y + frame.CY - pad}
	for _, c := range cells {
		if linkObstacle(c) {
			xs = append(xs, c.Bounds.X-pad, c.Bounds.X+c.Bounds.CX+pad)
			ys = append(ys, c.Bounds.Y-pad, c.Bounds.Y+c.Bounds.CY+pad)
		}
	}
	bestLength := int64(math.MaxInt64)
	var best []linkPoint
	var startSite, endSite int
	for _, side := range []pptx.ConnectionSide{pptx.SideRight, pptx.SideLeft, pptx.SideTop, pptx.SideBottom} {
		sx, sy, ss := pptx.OutlineAnchor(connectorEnd(cells[a]), side)
		tx, ty, ts := pptx.OutlineAnchor(connectorEnd(cells[b]), side)
		vertical := side == pptx.SideLeft || side == pptx.SideRight
		lanes := ys
		if vertical {
			lanes = xs
		}
		for _, lane := range lanes {
			points := candidateLinkPath(linkPoint{sx, sy}, linkPoint{tx, ty}, side, lane)
			if len(points) == 0 {
				continue
			}
			length, inside := linkPathLength(points, frame)
			if inside && length < bestLength && linkPathClear(points, cells, a, b) {
				best, bestLength, startSite, endSite = points, length, ss, ts
			}
		}
	}
	if len(best) == 0 {
		return nil, false
	}
	return linkDetourSegments(best, cells[a].ID, cells[b].ID, startSite, endSite, spec, alloc), true
}

func candidateLinkPath(source, target linkPoint, side pptx.ConnectionSide, lane int64) []linkPoint {
	vertical := side == pptx.SideLeft || side == pptx.SideRight
	if vertical {
		if side == pptx.SideRight && lane <= max(source.x, target.x) || side == pptx.SideLeft && lane >= min(source.x, target.x) {
			return nil
		}
		return []linkPoint{source, {lane, source.y}, {lane, target.y}, target}
	}
	if side == pptx.SideBottom && lane <= max(source.y, target.y) || side == pptx.SideTop && lane >= min(source.y, target.y) {
		return nil
	}
	return []linkPoint{source, {source.x, lane}, {target.x, lane}, target}
}

func linkPathLength(points []linkPoint, frame pptx.RectEmu) (length int64, inside bool) {
	inside = true
	for i, p := range points {
		inside = inside && p.x >= frame.X && p.x <= frame.X+frame.CX && p.y >= frame.Y && p.y <= frame.Y+frame.CY
		if i > 0 {
			dx, dy := p.x-points[i-1].x, p.y-points[i-1].y
			length += max(dx, -dx) + max(dy, -dy)
		}
	}
	return length, inside
}

func linkDetourSegments(best []linkPoint, sourceID, targetID uint32, startSite, endSite int, spec *ConnectorSpec, alloc *pptx.ShapeIDAllocator) []ResolvedConnector {
	var out []ResolvedConnector
	for i := 1; i < len(best); i++ {
		p, q := best[i-1], best[i]
		dx, dy := q.x-p.x, q.y-p.y
		conn := ResolvedConnector{ID: alloc.Alloc(), Spec: spec, Bounds: pptx.RectEmu{X: min(p.x, q.x), Y: min(p.y, q.y), CX: max(1, max(dx, -dx)), CY: max(1, max(dy, -dy))}, FlipH: dx < 0, FlipV: dy < 0, StartSite: -1, EndSite: -1, NoArrow: i < len(best)-1}
		if i == 1 {
			conn.SourceID = sourceID
			conn.StartSite = startSite
		}
		if i == len(best)-1 {
			conn.TargetID = targetID
			conn.EndSite = endSite
		}
		out = append(out, conn)
	}
	return out
}
