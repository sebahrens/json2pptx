package patterns

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// fillCappedRows gives a content-sized block enough vertical presence to read
// as the slide's main content. Only selected, point-capped rows receive the
// surplus; fixed dividers and callouts retain their authored proportions.
// Composite/flex rows are left to the grid resolver and opt out of this policy.
func fillCappedRows(ctx ExpandContext, rows []jsonschema.GridRowInput, rowGapPt, minFraction float64, eligible func(int) bool) {
	if len(rows) == 0 {
		return
	}
	_, areaH := sizingAreaPt(ctx)
	if areaH <= 0 {
		return
	}
	used := rowGapPt * float64(len(rows)-1)
	count := 0
	for i, row := range rows {
		if row.MaxHeight <= 0 || row.AutoHeight || row.Height > 0 {
			return
		}
		used += row.MaxHeight
		if eligible(i) {
			count++
		}
	}
	if count == 0 || used >= areaH*minFraction {
		return
	}
	extra := (areaH*minFraction - used) / float64(count)
	for i := range rows {
		if eligible(i) {
			rows[i].MaxHeight = math.Round((rows[i].MaxHeight+extra)*10) / 10
			rows[i].MinHeight = rows[i].MaxHeight
		}
	}
}
