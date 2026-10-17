package patterns

import (
	"encoding/json"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

func init() {
	point := visualObject(map[string]*Schema{"label": visualString(70), "x": NumberSchema(0, 100), "y": NumberSchema(0, 100), "size": NumberSchema(1, 40), "status": EnumSchema("supporter", "neutral", "blocker")}, "label", "x", "y", "status")
	point.raw.Properties["x"].raw.Minimum = float64Ptr(0)
	point.raw.Properties["y"].raw.Minimum = float64Ptr(0)
	Default().Register(&consultingPattern{name: "stakeholder-map", description: "Stakeholders positioned by influence and interest, sized by explicit diameter and colored by supporter/neutral/blocker stance", valuesSchema: visualObject(map[string]*Schema{"points": visualArray(point, 2, 12), "x_axis_label": visualString(60), "y_axis_label": visualString(60)}, "points", "x_axis_label", "y_axis_label"), example: map[string]any{"x_axis_label": "Interest", "y_axis_label": "Influence", "points": []any{map[string]any{"label": "Sponsor", "x": 80.0, "y": 85.0, "size": 20.0, "status": "supporter"}, map[string]any{"label": "Operations", "x": 35.0, "y": 60.0, "size": 12.0, "status": "neutral"}, map[string]any{"label": "Controls", "x": 70.0, "y": 25.0, "size": 16.0, "status": "blocker"}}}, draw: drawConsultingStakeholder})
}

type stakeholderBox struct{ x, y, w, h float64 }

func (a stakeholderBox) intersects(b stakeholderBox) bool {
	return a.x < b.x+b.w && a.x+a.w > b.x && a.y < b.y+b.h && a.y+a.h > b.y
}

func drawConsultingStakeholder(c *consultingCanvas, v map[string]any) error {
	left, top := 35.0, 44.0
	plotW, plotH := c.w-70, c.h-132
	if plotW <= 0 || plotH <= 0 {
		return c.refuse("stakeholder plot has no usable area")
	}
	if err := c.shape(left, top, plotW, plotH, "rect", "lt2", nil); err != nil {
		return err
	}
	if err := c.rule(left+plotW/2, top, 1, plotH, "dk1"); err != nil {
		return err
	}
	if err := c.rule(left, top+plotH/2, plotW, 1, "dk1"); err != nil {
		return err
	}
	if err := c.text(0, 0, c.w, 44, []visualParagraph{visualP(visualStr(v, "y_axis_label")+" ↑", 12, true)}); err != nil {
		return err
	}
	if err := c.text(0, c.h-88, c.w, 44, []visualParagraph{visualP(visualStr(v, "x_axis_label")+" →", 12, true)}); err != nil {
		return err
	}
	points := visualItems(v, "points")
	// Marker centres use an inset vertical domain inside the existing plot.
	// Its midpoint and ordering stay fixed, while y=0/100 remain valid without
	// letting the largest authored marker cover either axis text band.
	maxRadius := 0.0
	for _, raw := range points {
		diameter := visualNum(visualObj(raw), "size")
		if diameter == 0 {
			diameter = 14
		}
		maxRadius = max(maxRadius, diameter/2)
	}
	verticalInset := maxRadius + 4
	pointHeight := plotH - 2*verticalInset
	if pointHeight <= 0 {
		return c.refuse("stakeholder plot is too short for the authored marker diameters and axis clearances")
	}
	circles := make([]stakeholderBox, len(points))
	used := map[string]bool{}
	for i, raw := range points {
		p := visualObj(raw)
		diam := visualNum(p, "size")
		if diam == 0 {
			diam = 14
		}
		x := left + visualNum(p, "x")/100*plotW
		y := top + verticalInset + (1-visualNum(p, "y")/100)*pointHeight
		circles[i] = stakeholderBox{x - diam/2, y - diam/2, diam, diam}
		used[visualStr(p, "status")] = true
	}
	for i, circle := range circles {
		for _, other := range circles[i+1:] {
			if circle.intersects(other) {
				return c.refuse("stakeholder markers overlap; move the points or reduce their explicit diameters")
			}
		}
	}
	colors := map[string]string{"supporter": "#2E7D32", "neutral": "#6B7280", "blocker": "#B71C1C"}
	placed := []stakeholderBox{}
	for i, raw := range points {
		p := visualObj(raw)
		circle := circles[i]
		if err := c.shape(circle.x, circle.y, circle.w, circle.h, "ellipse", colors[visualStr(p, "status")], nil); err != nil {
			return err
		}
		label, err := placeStakeholderLabel(c, visualStr(p, "label"), circle, circles, placed)
		if err != nil {
			return err
		}
		placed = append(placed, label)
		if err := c.text(label.x, label.y, label.w, label.h, []visualParagraph{visualP(visualStr(p, "label"), 12, false)}); err != nil {
			return err
		}
	}
	index := 0
	for _, status := range []string{"supporter", "neutral", "blocker"} {
		if !used[status] {
			continue
		}
		w := c.w / float64(len(used))
		x := float64(index) * w
		index++
		if err := c.shape(x, c.h-27, 10, 10, "ellipse", colors[status], nil); err != nil {
			return err
		}
		if err := c.text(x+12, c.h-44, w-12, 44, []visualParagraph{visualP(status, 12, false)}); err != nil {
			return err
		}
	}
	return nil
}
func placeStakeholderLabel(c *consultingCanvas, text string, circle stakeholderBox, circles, placed []stakeholderBox) (stakeholderBox, error) {
	width := min(170.0, c.w/3)
	measured, err := textfit.MeasureLineWidth(text, c.ctx.Theme.BodyFont, 12)
	if err != nil {
		return stakeholderBox{}, err
	}
	width = min(width, float64(measured)/12700+2*defaultShapeInsetLRPt+1)
	raw, _ := json.Marshal(map[string]any{"paragraphs": []visualParagraph{visualP(text, 12, false)}})
	height := shapeTextHeightPt(c.ctx.Theme.BodyFont, raw, width-2*defaultShapeInsetLRPt) + 2*defaultShapeInsetTBPt
	x, y := circle.x+circle.w/2, circle.y+circle.h/2
	candidates := []stakeholderBox{{circle.x + circle.w + 3, y - height/2, width, height}, {circle.x - width - 3, y - height/2, width, height}, {x - width/2, circle.y - height - 3, width, height}, {x - width/2, circle.y + circle.h + 3, width, height}}
	for _, candidate := range candidates {
		if candidate.x < 0 || candidate.y < 44 || candidate.x+candidate.w > c.w || candidate.y+candidate.h > c.h-88 {
			continue
		}
		clear := true
		for _, obstacle := range append(append([]stakeholderBox{}, circles...), placed...) {
			if candidate.intersects(obstacle) {
				clear = false
				break
			}
		}
		if clear {
			return candidate, nil
		}
	}
	return stakeholderBox{}, c.refuse(fmt.Sprintf("stakeholder label %q cannot fit clear of points and other full12pt labels; move points or use fewer labels", text))
}
