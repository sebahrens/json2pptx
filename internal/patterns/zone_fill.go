package patterns

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// contentStretchMax caps how far a content-sized box or row may grow past
// its natural (measured) height: at most 1.6x (go-slide-creator-wntyw). A box
// stretched further reads as an empty panel; the leftover zone height stays
// as margin around the middle-anchored block instead.
const contentStretchMax = 1.6

// fillCappedRows gives a content-sized list of UNFILLED rows (statement /
// metric / label rows separated by rules) enough vertical rhythm to read as
// the slide's main content. Only selected, point-capped rows receive the
// surplus, and each at most contentStretchMax x its natural height, so short
// lists open up their spacing without turning into stretched boxes; the rest
// of the zone stays as margin around the middle-anchored block. Fixed
// dividers and callouts retain their authored proportions. Composite/flex
// rows are left to the grid resolver and opt out of this policy. Patterns of
// filled cards / panels (card-grid, before-after, kpi-Nup) do not use it.
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
			grown := math.Min(rows[i].MaxHeight+extra, rows[i].MaxHeight*contentStretchMax)
			rows[i].MaxHeight = math.Round(grown*10) / 10
			rows[i].MinHeight = rows[i].MaxHeight
		}
	}
}
