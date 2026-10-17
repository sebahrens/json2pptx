package patterns

import (
	"fmt"
	"math"
)

func init() {
	row := visualObject(map[string]*Schema{"label": visualString(80), "actual": NumberSchema(0, 0), "target": NumberSchema(0, 0), "unit": StringSchema(20)}, "label", "actual", "target")
	Default().Register(&consultingPattern{name: "progress-list", description: "3–8 KPI rows with actual bars, explicit target ticks and units; 12pt labels with measured refusal", valuesSchema: visualObject(map[string]*Schema{"rows": visualArray(row, 3, 8)}, "rows"), example: map[string]any{"rows": []any{map[string]any{"label": "Revenue", "actual": 72.0, "target": 90.0, "unit": "M"}, map[string]any{"label": "Customers", "actual": 48.0, "target": 60.0, "unit": "k"}, map[string]any{"label": "Margin", "actual": 21.0, "target": 25.0, "unit": "%"}}}, check: checkConsultingBullet, draw: drawConsultingBullet})
}
func checkConsultingBullet(v map[string]any) error {
	for _, raw := range visualItems(v, "rows") {
		r := visualObj(raw)
		a, t := visualNum(r, "actual"), visualNum(r, "target")
		if math.IsNaN(a) || math.IsInf(a, 0) || math.IsNaN(t) || math.IsInf(t, 0) || a < 0 || t <= 0 {
			return fmt.Errorf("progress-list actual must be finite and nonnegative; target must be finite and positive")
		}
	}
	return nil
}
func drawConsultingBullet(c *consultingCanvas, v map[string]any) error {
	rows := visualItems(v, "rows")
	h := c.h / float64(len(rows))
	if h < 44 {
		return c.refuse("KPI rows need at least 44pt each for 12pt labels, actual bars and target ticks")
	}
	for i, raw := range rows {
		r := visualObj(raw)
		y := float64(i) * h
		a, t := visualNum(r, "actual"), visualNum(r, "target")
		unit := visualStr(r, "unit")
		label := fmt.Sprintf("%s   %g%s   | target %g%s", visualStr(r, "label"), a, unit, t, unit)
		if err := c.text(0, y, c.w, h, []visualParagraph{visualP(label, 12, false)}); err != nil {
			return err
		}
		maxValue := max(a, t) * 1.1
		barW := c.w - 30
		barX := 15.0
		barY := y + h - 7
		if err := c.rule(barX, barY, barW, 4, "lt2"); err != nil {
			return err
		}
		if a > 0 {
			if err := c.rule(barX, barY, barW*a/maxValue, 4, "dk1"); err != nil {
				return err
			}
		}
		if err := c.rule(barX+barW*t/maxValue, barY-3, 2, 10, c.ctx.DefaultAccent()); err != nil {
			return err
		}
	}
	return nil
}
