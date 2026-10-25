package rhythm

import (
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// measureDensity uses actual text and resolved geometry. A text-only projection
// without geometry gets an explicitly estimated reference-frame load; mere
// content-kind flags carry no occupancy evidence and remain unknown.
func measureDensity(s Slide) (class string, occupancy float64, source string) {
	if isStructural(s) {
		return "unknown", 0, "unknown"
	}
	source = "unknown"
	if s.Grid != nil && s.Grid.Bounds.CX > 0 && s.Grid.Bounds.CY > 0 && shapegrid.Validate(s.Grid) == nil {
		if result, err := shapegrid.Resolve(s.Grid, pptx.NewShapeIDAllocator(nil)); err == nil {
			area := float64(s.Grid.Bounds.CX) * float64(s.Grid.Bounds.CY)
			densities := textcapacity.ForResolvedGrid(result)
			for i, cell := range result.Cells {
				if cell.Kind == shapegrid.CellKindShape {
					occupancy += densities[i].RequiredHeightPt * 12700 * float64(cell.Bounds.CX) / area
				} else {
					occupancy += float64(cell.Bounds.CX) * float64(cell.Bounds.CY) / area
				}
			}
			source = "measured"
		}
	}
	if source == "unknown" && strings.TrimSpace(s.Text) != "" {
		// Same reference size for prose, bullets, pattern payloads and compose
		// text: changing representation alone must not manufacture variety.
		m, err := textfit.MeasureRun(s.Text, "Liberation Sans", tokens.TypeScaleBodyPt, 800*12700, 0)
		if err == nil {
			occupancy = float64(m.RequiredEMU) / (350 * 12700)
			source = "estimated"
		}
	}
	if source == "unknown" {
		return "unknown", 0, source
	}
	occupancy = math.Round(occupancy*1000) / 1000
	switch {
	case occupancy < 0.35:
		class = "low"
	case occupancy < 0.75:
		class = "med"
	default:
		class = "high"
	}
	return class, occupancy, source
}
