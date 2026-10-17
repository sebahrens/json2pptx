package shapegrid

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func columnRuleIndices(raw json.RawMessage, columns int) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var all string
	if json.Unmarshal(raw, &all) == nil {
		if all != "all" {
			return nil, fmt.Errorf("col_rule must be all or an array of column seam indexes")
		}
		seams := make([]int, max(columns-1, 0))
		for i := range seams {
			seams[i] = i
		}
		return seams, nil
	}
	var indexes []int
	if err := json.Unmarshal(raw, &indexes); err != nil {
		return nil, fmt.Errorf("col_rule must be all or an array of column seam indexes: %w", err)
	}
	seen := map[int]bool{}
	var out []int
	for _, i := range indexes {
		if i < 0 || i >= columns-1 {
			return nil, fmt.Errorf("col_rule index %d is outside %d column seams", i, max(columns-1, 0))
		}
		if !seen[i] {
			out = append(out, i)
			seen[i] = true
		}
	}
	return out, nil
}

// Rules are decoration, not content cells. Clip each seam around spanning
// cells, including full-width headings and envelope bands.
func resolveColumnRules(g *Grid, cells []ResolvedCell, x, w []int64, gap int64, y, h []int64, rowGap int64, alloc *pptx.ShapeIDAllocator) []ResolvedAccentBar {
	seams, _ := columnRuleIndices(g.ColRule, len(x))
	if len(y) == 0 {
		return nil
	}
	var out []ResolvedAccentBar
	for _, seam := range seams {
		centre := x[seam] + w[seam] + gap/2
		cuts := [][2]int64{{y[0], y[len(y)-1] + h[len(h)-1]}}
		var blocks [][2]int64
		for _, c := range cells {
			b := c.CellBounds
			if b.X < centre && b.X+b.CX > centre {
				blocks = append(blocks, [2]int64{b.Y - rowGap/2, b.Y + b.CY + rowGap/2})
			}
		}
		sort.Slice(blocks, func(i, j int) bool { return blocks[i][0] < blocks[j][0] })
		for _, b := range blocks {
			var next [][2]int64
			for _, r := range cuts {
				if b[1] <= r[0] || b[0] >= r[1] {
					next = append(next, r)
					continue
				}
				if b[0] > r[0] {
					next = append(next, [2]int64{r[0], b[0]})
				}
				if b[1] < r[1] {
					next = append(next, [2]int64{b[1], r[1]})
				}
			}
			cuts = next
		}
		for _, r := range cuts {
			if r[1] <= r[0] {
				continue
			}
			out = append(out, ResolvedAccentBar{Decorative: true, Bounds: pptx.RectEmu{X: centre - PtToEMU(RowRulePt)/2, Y: r[0], CX: PtToEMU(RowRulePt), CY: r[1] - r[0]}, ID: alloc.Alloc(), Spec: &AccentBarSpec{Fill: rowRuleFill, Width: RowRulePt}})
		}
	}
	return out
}
