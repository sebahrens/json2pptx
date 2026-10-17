package slides

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// RegionGap returns a valid authored point gap and whether one was supplied.
func RegionGap(body map[string]any) (float64, bool, error) {
	value, exists := body["gap"]
	if !exists {
		return 0, false, nil
	}
	var n float64
	switch v := value.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	default:
		return 0, true, fmt.Errorf("gap must be a finite nonnegative number of points")
	}
	if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, true, fmt.Errorf("gap must be a finite nonnegative number of points")
	}
	return n, true, nil
}

func applyRegionSeams(grid *deckinput.ShapeGridInput, builds []regionBuild, body map[string]any) error {
	gap, explicit, err := RegionGap(body)
	if err != nil {
		return err
	}
	// Positive sub-EMU sentinel survives the raw grid's zero-as-default contract
	// and resolves to exactly zero EMU, matching composition's explicit zero gap.
	if explicit && gap == 0 {
		gap = 0.000001
	}
	divider, _ := body["divider"].(bool)
	leaves := make(map[*deckinput.GridCellInput]bool, len(builds))
	for _, b := range builds {
		leaves[b.cell] = true
	}
	var walk func(*deckinput.ShapeGridInput)
	walk = func(g *deckinput.ShapeGridInput) {
		if g == nil {
			return
		}
		if explicit {
			g.Gap = gap
			g.ColGap = gap
			g.RowGap = gap
		}
		if divider {
			g.ColRule = json.RawMessage(`"all"`)
		}
		for _, row := range g.Rows {
			for _, cell := range row.Cells {
				if cell != nil && !leaves[cell] {
					walk(cell.Grid)
				}
			}
		}
	}
	walk(grid)
	return nil
}
