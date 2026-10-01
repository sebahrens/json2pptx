package svggen

import (
	"fmt"
	"math"
)

// Pie / donut defaults follow the narrow role dataviz authorities allow a pie
// (go-slide-creator-ihlsr): a coarse part-to-whole view, slices sorted
// largest-first clockwise from 12 o'clock, labelled directly with
// "Name NN%" and no legend, the long tail of tiny slices folded into
// "Other", and — when the slide is about one slice — that slice in accent1
// against neutral greys.
const (
	// PieDirectLabelMaxSlices is the most slices a pie labels directly; above
	// it the legend returns and labels carry the value only.
	PieDirectLabelMaxSlices = 6
	// PieGroupSmallBelowPct is the default share (percent) under which slices
	// are folded into one "Other" slice — only when two or more fall below it.
	PieGroupSmallBelowPct = 3.0
	// pieOtherLabel names the folded slice.
	pieOtherLabel = "Other"
)

// preparePieData applies the pie defaults to extracted pie data in place:
// placeholder labels, highlight resolution, the descending sort, and small
// slice grouping.
func preparePieData(req *RequestEnvelope, chart *ChartData) error {
	if len(chart.Series) == 0 {
		return nil
	}
	values := chart.Series[0].Values
	if len(chart.Categories) < len(values) {
		cats := make([]string, len(values))
		copy(cats, chart.Categories)
		for i := len(chart.Categories); i < len(values); i++ {
			cats[i] = fmt.Sprintf("Slice %d", i+1)
		}
		chart.Categories = cats
	}
	highlight, set, err := resolveHighlight(req.Data, chart.Categories)
	if err != nil {
		return err
	}
	chart.Highlight, chart.HighlightSet = highlight, set

	mode := SortDesc
	if raw, ok := req.Data["sort"]; ok && raw != nil {
		m, isString := raw.(string)
		switch {
		case isString && (m == SortDesc || m == SortAsc || m == SortNone):
			mode = m
		case isString && m == "":
		default:
			return &ValidationError{Field: "data.sort", Code: ErrCodeInvalidValue,
				Message: fmt.Sprintf("data.sort must be \"desc\", \"asc\" or \"none\", got %v", raw), Value: raw}
		}
	}
	applyCategorySort(chart, mode)

	threshold := PieGroupSmallBelowPct
	if raw, ok := req.Data["group_small_below_pct"]; ok {
		v, isNum := toFloat64Value(raw)
		if !isNum || v < 0 || v > 50 {
			return &ValidationError{Field: "data.group_small_below_pct", Code: ErrCodeInvalidValue,
				Message: fmt.Sprintf("data.group_small_below_pct must be a number from 0 (never group) to 50, got %v", raw), Value: raw}
		}
		threshold = v
	}
	groupSmallSlices(chart, threshold)
	return nil
}

// groupSmallSlices folds every positive slice under thresholdPct of the
// total into one trailing "Other" slice, when at least two qualify (one tiny
// slice is clearer named than renamed). A highlighted slice is never folded.
func groupSmallSlices(chart *ChartData, thresholdPct float64) {
	if thresholdPct <= 0 || len(chart.Series) == 0 {
		return
	}
	s := &chart.Series[0]
	total := 0.0
	for _, v := range s.Values {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		return
	}
	highlighted := make(map[int]bool, len(chart.Highlight))
	for _, h := range chart.Highlight {
		highlighted[h] = true
	}
	small := 0
	for i, v := range s.Values {
		if v > 0 && 100*v/total < thresholdPct && !highlighted[i] {
			small++
		}
	}
	if small < 2 {
		return
	}
	var (
		cats    []string
		vals    []float64
		newHigh []int
		other   float64
	)
	for i, v := range s.Values {
		if v > 0 && 100*v/total < thresholdPct && !highlighted[i] {
			other += v
			continue
		}
		if highlighted[i] {
			newHigh = append(newHigh, len(vals))
		}
		vals = append(vals, v)
		if i < len(chart.Categories) {
			cats = append(cats, chart.Categories[i])
		} else {
			cats = append(cats, fmt.Sprintf("Slice %d", i+1))
		}
	}
	chart.Categories = append(cats, pieOtherLabel)
	s.Values = append(vals, other)
	s.Labels = nil
	s.labelValues = nil
	if chart.HighlightSet {
		chart.Highlight = newHigh
	}
}

// pieHighlightColors paints the highlighted slices in accent and the rest in
// two alternating neutral greys, so neighbouring context slices still part.
func pieHighlightColors(p *Palette, accent Color, n int, highlight []int) []Color {
	greys := []Color{NeutralInk(p, BarNeutralInk), NeutralInk(p, BarNeutralInk*0.6)}
	colors := make([]Color, n)
	k := 0
	hl := make(map[int]bool, len(highlight))
	for _, h := range highlight {
		hl[h] = true
	}
	for i := range colors {
		if hl[i] {
			colors[i] = accent
			continue
		}
		colors[i] = greys[k%len(greys)]
		k++
	}
	return colors
}

// pieUsesDirectLabels reports whether a pie with n slices drops its legend
// for "Name NN%" labels: up to PieDirectLabelMaxSlices slices, unless the
// author forced the legend with style.show_legend.
func pieUsesDirectLabels(req *RequestEnvelope, n int) bool {
	return !req.Style.ShowLegend && n > 0 && n <= PieDirectLabelMaxSlices
}

// pieSliceCount counts the slices a prepared pie will draw.
func pieSliceCount(chart ChartData) int {
	if len(chart.Series) == 0 {
		return 0
	}
	n := 0
	for _, v := range chart.Series[0].Values {
		if v >= 0 && !math.IsNaN(v) {
			n++
		}
	}
	return n
}
