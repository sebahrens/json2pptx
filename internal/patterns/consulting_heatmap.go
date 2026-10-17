package patterns

import (
	"fmt"
	"math"
)

// Numeric heatmaps keep axis labels as 12pt body copy. Only the numeric cell
// values use the 10pt dense-matrix caption role; all text is measured in full.
func drawConsultingHeatmap(c *consultingCanvas, v map[string]any) error {
	rows, cols := visualStrings(v, "row_labels"), visualStrings(v, "col_labels")
	values := visualItems(v, "values")
	labelW, headerH := 120.0, 54.0
	cellW := (c.w - labelW) / float64(len(cols))
	cellH := (c.h - headerH) / float64(len(rows))
	if cellW <= 0 || cellH <= 0 {
		return c.refuse("heatmap cells have no usable area")
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, raw := range values {
		nums, _ := raw.([]any)
		for _, x := range nums {
			n, _ := x.(float64)
			lo = min(lo, n)
			hi = max(hi, n)
		}
	}
	for j, label := range cols {
		if err := c.text(labelW+float64(j)*cellW, 0, cellW, headerH, []visualParagraph{visualP(label, 12, true)}); err != nil {
			return err
		}
	}
	for i, label := range rows {
		y := headerH + float64(i)*cellH
		if err := c.text(0, y, labelW, cellH, []visualParagraph{visualP(label, 12, true)}); err != nil {
			return err
		}
		nums, _ := values[i].([]any)
		for j, raw := range nums {
			n, _ := raw.(float64)
			shade := .5
			if hi > lo {
				shade = (n - lo) / (hi - lo)
				if math.IsInf(hi-lo, 0) {
					shade = (n/2 - lo/2) / (hi/2 - lo/2)
				}
			}
			r := int(232 - shade*180)
			g := int(238 - shade*130)
			b := int(245 - shade*90)
			x := labelW + float64(j)*cellW
			if err := c.shape(x, y, cellW-1, cellH-1, "rect", fmt.Sprintf("#%02X%02X%02X", r, g, b), nil); err != nil {
				return err
			}
			value := visualP(fmt.Sprintf("%g", n), 10, false)
			value.Color = "#000000"
			if shade > .9 {
				value.Color = "#FFFFFF"
			}
			if err := c.text(x, y, cellW-1, cellH-1, []visualParagraph{value}); err != nil {
				return err
			}
		}
	}
	return nil
}
