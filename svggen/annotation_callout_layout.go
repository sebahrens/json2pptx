package svggen

import (
	"fmt"
	"math"
)

// placeCalloutLabel keeps a readable callout inside the plot and clear of
// existing value, axis and annotation text. Nearby upper-right placement wins;
// successive rows search above and below the target before refusing the layout.
func placeCalloutLabel(b *SVGBuilder, plot Rect, target Point, width, height float64) (Rect, bool) {
	const gap = 4.0
	area := plot.Inset(gap, gap, gap, gap)
	if width <= 0 || height <= 0 || width > area.W || height > area.H {
		return Rect{}, false
	}
	right := math.Max(area.X, math.Min(target.X+30, area.X+area.W-width))
	left := math.Max(area.X, math.Min(target.X-30-width, area.X+area.W-width))
	for step := 0; step <= int(math.Ceil(area.H/(height+gap))); step++ {
		distance := 30 + float64(step)*(height+gap)
		for _, y := range []float64{target.Y - distance - height, target.Y + distance} {
			for _, x := range []float64{right, left} {
				box := Rect{X: x, Y: y, W: width, H: height}
				if calloutLabelFits(b, box, area, target) {
					return box, true
				}
			}
		}
	}
	// At the plot edges, clamping the last two rows covers free space nearer
	// the target than the preferred 30pt offset.
	for _, y := range []float64{area.Y, area.Y + area.H - height} {
		for _, x := range []float64{right, left} {
			box := Rect{X: x, Y: y, W: width, H: height}
			if calloutLabelFits(b, box, area, target) {
				return box, true
			}
		}
	}
	return Rect{}, false
}

func calloutLabelFits(b *SVGBuilder, box, area Rect, target Point) bool {
	if box.X < area.X || box.Y < area.Y || box.X+box.W > area.X+area.W || box.Y+box.H > area.Y+area.H {
		return false
	}
	anchor := calloutLeaderStart(box, target)
	for _, existing := range b.textBoxes {
		if box.Inset(-3, -3, -3, -3).Intersects(existing.rect) || calloutLeaderCrossesText(anchor, target, existing.rect.Inset(-2, -2, -2, -2)) {
			return false
		}
	}
	return true
}

func calloutGeometryError(b *SVGBuilder, text string, plot Rect) error {
	message := fmt.Sprintf("callout %q cannot fit clear of chart labels at a readable size in the %.0f×%.0fpt plot; shorten the callout, use fewer annotations or series, or increase the chart frame", text, plot.W, plot.H)
	b.AddFinding(Finding{Field: "data.annotations", Code: FindingDiagramRegionOverflow, Message: message, Severity: "refuse", Fix: &FixSuggestion{Kind: FixKindReduceItems, Params: map[string]any{"alternatives": []string{"shorter callout", "fewer annotations or series", "larger chart frame"}}}})
	return &ValidationError{Field: "data.annotations", Code: FindingDiagramRegionOverflow, Message: message}
}

func calloutLeaderStart(box Rect, target Point) Point {
	return Point{X: math.Max(box.X, math.Min(target.X, box.X+box.W)), Y: math.Max(box.Y, math.Min(target.Y, box.Y+box.H))}
}

// calloutLeaderCrossesText clips a segment against the font rectangle, so
// moving a caption cannot fix its box by running its leader through a value.
func calloutLeaderCrossesText(start, end Point, box Rect) bool {
	enter, leave := 0.0, 1.0
	for _, axis := range [][4]float64{{start.X, end.X, box.X, box.X + box.W}, {start.Y, end.Y, box.Y, box.Y + box.H}} {
		delta := axis[1] - axis[0]
		if math.Abs(delta) < 1e-9 {
			if axis[0] < axis[2] || axis[0] > axis[3] {
				return false
			}
			continue
		}
		a, c := (axis[2]-axis[0])/delta, (axis[3]-axis[0])/delta
		enter = math.Max(enter, math.Min(a, c))
		leave = math.Min(leave, math.Max(a, c))
		if enter > leave {
			return false
		}
	}
	return true
}
