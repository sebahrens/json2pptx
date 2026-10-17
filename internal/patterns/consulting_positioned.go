package patterns

import (
	"fmt"
	"sort"
)

func init() {
	for _, p := range consultingPositionedPatterns() {
		Default().Register(p)
	}
}
func consultingPositionedPatterns() []*consultingPattern {
	stop := visualObject(map[string]*Schema{"date": visualString(32), "title": visualString(80), "detail": StringSchema(180)}, "date", "title")
	marker := visualObject(map[string]*Schema{"label": visualString(70), "position": consultingUnitInterval()}, "label", "position")
	return []*consultingPattern{
		{name: "timeline-vertical", description: "3–8 dated stops on a vertical spine; measured 12pt text, single-sided or alternating", valuesSchema: visualObject(map[string]*Schema{"stops": visualArray(stop, 3, 8), "alternating": BooleanSchema(), "highlight": StringSchema(80)}, "stops"), example: map[string]any{"stops": []any{map[string]any{"date": "Q1", "title": "Align"}, map[string]any{"date": "Q2", "title": "Pilot"}, map[string]any{"date": "Q3", "title": "Scale"}}}, draw: drawConsultingTimeline},
		{name: "spectrum", description: "A continuous axis with explicit endpoints and 2–6 positioned markers; colliding labels occupy separate measured lanes", valuesSchema: visualObject(map[string]*Schema{"left_label": visualString(60), "right_label": visualString(60), "markers": visualArray(marker, 2, 6), "highlight": StringSchema(70)}, "left_label", "right_label", "markers"), example: map[string]any{"left_label": "Centralised", "right_label": "Distributed", "markers": []any{map[string]any{"label": "Today", "position": .25}, map[string]any{"label": "Target", "position": .75}}, "highlight": "Target"}, draw: drawConsultingSpectrum},
	}
}
func drawConsultingTimeline(c *consultingCanvas, v map[string]any) error {
	stops := visualItems(v, "stops")
	row := c.h / float64(len(stops))
	alternate, _ := v["alternating"].(bool)
	spine := 12.0
	if alternate {
		spine = c.w / 2
	}
	if err := c.rule(spine, 10, 2, c.h-20, "dk1"); err != nil {
		return err
	}
	for i, raw := range stops {
		stop := visualObj(raw)
		y := float64(i) * row
		color := "dk1"
		if visualStr(v, "highlight") == visualStr(stop, "title") {
			color = c.ctx.DefaultAccent()
		}
		if err := c.shape(spine-4, y+row/2-4, 10, 10, "ellipse", color, nil); err != nil {
			return err
		}
		x, w := spine+12, c.w-spine-12
		if alternate && i%2 == 0 {
			x = 0
			w = spine - 12
		}
		lines := []visualParagraph{visualP(visualStr(stop, "date")+" · "+visualStr(stop, "title"), 12, true)}
		if detail := visualStr(stop, "detail"); detail != "" {
			lines = append(lines, visualP(detail, 12, false))
		}
		if err := c.text(x, y, w, row, lines); err != nil {
			return err
		}
	}
	return nil
}
func drawConsultingSpectrum(c *consultingCanvas, v map[string]any) error {
	markers := append([]any(nil), visualItems(v, "markers")...)
	sort.SliceStable(markers, func(i, j int) bool {
		return visualNum(visualObj(markers[i]), "position") < visualNum(visualObj(markers[j]), "position")
	})
	axisY := c.h * .72
	margin := 55.0
	axisW := c.w - 2*margin
	if axisW <= 0 {
		return c.refuse("spectrum axis has no usable width")
	}
	if err := c.rule(margin, axisY, axisW, 2, "dk1"); err != nil {
		return err
	}
	if err := c.text(0, axisY+5, c.w/2, c.h-axisY-5, []visualParagraph{visualP(visualStr(v, "left_label"), 12, true)}); err != nil {
		return err
	}
	if err := c.text(c.w/2, axisY+5, c.w/2, c.h-axisY-5, []visualParagraph{visualP(visualStr(v, "right_label"), 12, true)}); err != nil {
		return err
	}
	laneH := 48.0
	labelW := min(170.0, c.w/2)
	ends := []float64{}
	for _, raw := range markers {
		m := visualObj(raw)
		x := margin + visualNum(m, "position")*axisW
		left := max(0.0, min(c.w-labelW, x-labelW/2))
		lane := 0
		for lane < len(ends) && left < ends[lane]+8 {
			lane++
		}
		if lane == len(ends) {
			ends = append(ends, 0)
		}
		ends[lane] = left + labelW
		y := axisY - float64(lane+1)*laneH - 8
		if y < 0 {
			return c.refuse(fmt.Sprintf("spectrum marker labels need %d readable lanes", lane+1))
		}
		color := "dk1"
		bold := visualStr(v, "highlight") == visualStr(m, "label")
		if bold {
			color = c.ctx.DefaultAccent()
		}
		if err := c.rule(x, y+laneH, 1, axisY-y-laneH, color); err != nil {
			return err
		}
		if err := c.shape(x-4, axisY-4, 9, 9, "ellipse", color, nil); err != nil {
			return err
		}
		if err := c.text(left, y, labelW, laneH, []visualParagraph{visualP(visualStr(m, "label"), 12, bold)}); err != nil {
			return err
		}
	}
	return nil
}

func consultingUnitInterval() *Schema {
	s := NumberSchema(0, 1)
	s.raw.Minimum = float64Ptr(0)
	s.raw.Maximum = float64Ptr(1)
	return s
}
