package svggen

import (
	"fmt"
	"math"
)

// groupedLabelOffsets separates labels whose measured text boxes overlap.
// Each label moves away from zero so negative and zero bars keep the same
// outside-label convention as positive bars.
func (bc *BarChart) groupedLabelOffsets(data ChartData, scale *LinearScale, bandwidth float64, format string) [][]float64 {
	offsets := make([][]float64, len(data.Series))
	for i, s := range data.Series {
		offsets[i] = make([]float64, len(s.Values))
	}
	if len(data.Series) < 2 || !bc.config.ShowValues {
		return offsets
	}
	b := bc.builder
	style := b.StyleGuide()
	font, gap := style.Typography.SizeSmall, style.Spacing.SM
	if bc.labelledMode {
		font, gap = labelledValueFont(style), labelledValueGapPt
	}
	b.Push()
	defer b.Pop()
	b.SetFontSize(font)
	groupWidth := bandwidth * (1 - DefaultBarSeriesConfig().BarPadding)
	step := groupWidth / float64(len(data.Series))
	for ci := range data.Categories {
		var boxes []Rect
		for si, series := range data.Series {
			if ci >= len(series.Values) {
				continue
			}
			v := series.Values[ci]
			weight := style.Typography.WeightNormal
			if (data.SeriesHighlightSet && seriesHighlighted(data, si)) || (data.HighlightSet && containsHighlight(data.Highlight, ci)) {
				weight = style.Typography.WeightBold
			}
			b.SetFontWeight(weight)
			text := TrueMinus(bc.config.ValueFmt.FormatOr(v, format))
			w, _ := b.MeasureText(text)
			yv := v
			if bc.logScale != nil {
				dmin, _ := bc.logScale.DomainBounds()
				yv = math.Log10(math.Max(v, dmin))
			}
			direction := -1.0
			y := scale.Scale(yv) - gap - font
			if v < 0 && bc.logScale == nil {
				direction, y = 1, scale.Scale(yv)+gap
			}
			box := Rect{X: float64(si)*step - w/2, Y: y, W: w, H: font}
			box, offsets[si][ci] = staggerGroupedLabel(box, boxes, direction*(font+2))
			boxes = append(boxes, box)
		}
	}
	return offsets
}

func containsHighlight(indices []int, i int) bool {
	for _, h := range indices {
		if h == i {
			return true
		}
	}
	return false
}

// validateGroupedValueLabels refuses a layout whose requested labels cannot
// all be drawn readably. The actual font boxes cover both orientations and
// include title/category/legend collisions that a plot-only check misses.
func (bc *BarChart) validateGroupedValueLabels(data ChartData) error {
	if len(data.Series) < 2 || bc.config.Stacked || len(bc.groupedValueBoxes) == 0 {
		return nil
	}
	b := bc.builder
	for _, i := range bc.groupedValueBoxes {
		label := b.textBoxes[i]
		r := label.rect
		reason := ""
		if r.X < -0.1 || r.Y < -0.1 || r.X+r.W > bc.config.Width+0.1 || r.Y+r.H > bc.config.Height+0.1 {
			reason = "extends outside the chart frame"
		}
		for j := bc.groupedTextStart; reason == "" && j < len(b.textBoxes); j++ {
			if i == j {
				continue
			}
			other := b.textBoxes[j]
			w := math.Min(r.X+r.W, other.rect.X+other.rect.W) - math.Max(r.X, other.rect.X)
			h := math.Min(r.Y+r.H, other.rect.Y+other.rect.H) - math.Max(r.Y, other.rect.Y)
			if w > 0.5 && h > 0.5 {
				reason = fmt.Sprintf("overlaps %q", other.text)
			}
		}
		if reason != "" {
			message := fmt.Sprintf("grouped bar value label %q %s at a readable font size in the %.0f×%.0fpt frame; use fewer series or categories, split the chart, or increase its width and height", label.text, reason, bc.config.Width, bc.config.Height)
			b.AddFinding(Finding{Field: "data.series", Code: FindingDiagramRegionOverflow, Severity: "refuse", Message: message, Fix: &FixSuggestion{Kind: FixKindReduceItems, Params: map[string]any{"series_count": len(data.Series), "category_count": len(data.Categories), "alternatives": []string{"fewer series or categories", "split the chart", "larger chart frame"}}}})
			return &ValidationError{Field: "data.series", Code: FindingDiagramRegionOverflow, Message: message}
		}
	}
	return nil
}

func staggerGroupedLabel(box Rect, previous []Rect, delta float64) (Rect, float64) {
	offset := 0.0
	for groupedLabelCollides(box, previous) {
		box.Y += delta
		offset += delta
	}
	return box, offset
}

func groupedLabelCollides(box Rect, previous []Rect) bool {
	for _, other := range previous {
		if box.X < other.X+other.W+2 && box.X+box.W+2 > other.X && box.Y < other.Y+other.H+2 && box.Y+box.H+2 > other.Y {
			return true
		}
	}
	return false
}
