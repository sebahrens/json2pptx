package svggen

import "fmt"

var stakeholderStatusColors = map[string]Color{"supporter": MustParseColor("#2E7D32"), "neutral": MustParseColor("#6B7280"), "blocker": MustParseColor("#B71C1C")}

func validateMatrixPointStatus(data map[string]any) error {
	raw, _ := toAnySlice(data["points"])
	for i, value := range raw {
		point, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if status, set := point["status"]; set {
			if _, ok := stakeholderStatusColors[fmt.Sprint(status)]; !ok {
				return fmt.Errorf("data.points[%d].status must be supporter, neutral or blocker", i)
			}
			if _, set := point["color"]; set {
				return fmt.Errorf("data.points[%d]: status and color cannot both control the marker", i)
			}
		}
		if value, set := point["size"]; set {
			size, ok := consultingFinite(value)
			if !ok || size <= 0 || size > 40 {
				return fmt.Errorf("data.points[%d].size must be a finite diameter in (0,40]", i)
			}
		}
	}
	return nil
}
func matrixHasStatus(data Matrix2x2Data) bool {
	for _, point := range data.Points {
		if point.Status != "" {
			return true
		}
	}
	return false
}
func drawMatrixStatusLegend(b *SVGBuilder, data Matrix2x2Data) error {
	used := map[string]bool{}
	for _, p := range data.Points {
		if p.Status != "" {
			used[p.Status] = true
		}
	}
	if len(used) == 0 {
		return nil
	}
	x := 20.0
	y := b.Height() - 13
	if data.Footnote != "" {
		y -= FootnoteReservedHeight(b.style)
	}
	for _, status := range []string{"supporter", "neutral", "blocker"} {
		if !used[status] {
			continue
		}
		color := stakeholderStatusColors[status]
		if color.ContrastWith(b.style.Palette.Background) < 3 {
			return consultingChartError("stakeholder status ink is not readable against the template background")
		}
		b.SetFillColor(color).DrawRect(Rect{X: x, Y: y - 4, W: 8, H: 8})
		if err := consultingChartText(b, status, x+13, y, 110, TextAlignLeft); err != nil {
			return err
		}
		x += 125
	}
	return nil
}
