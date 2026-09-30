package svggen

import "math"

// BubbleSizeScale maps a bubble chart's size values to marker diameters.
//
// Policy (go-slide-creator-b7qqg.20, .21):
//
//   - One domain for the whole chart. The largest size value across EVERY
//     series sets the maximum diameter, so equal sizes draw equal bubbles
//     whichever series they belong to. A per-series min/max drew size 10 as a
//     large bubble in one series and a tiny one in the next.
//   - Area-proportional. diameter = maxDiameter * sqrt(value / maxValue), so
//     sizes 1:2:4 draw areas 1:2:4. A linear min/max mapping to the radius
//     drew sizes 5 and 10 with a ~17.6x area ratio.
//   - Zero, and positive values too small to see, are drawn at minDiameter:
//     a visibility floor, not a data encoding. Values below
//     maxValue * (minDiameter/maxDiameter)^2 are therefore not proportional.
//   - A chart with no positive size at all encodes nothing by size, so every
//     marker is drawn at the plain scatter point size.
//   - Negative sizes have no area. The bubble chart rejects them at
//     validation; were one to reach the scale it would draw at the floor.
type BubbleSizeScale struct {
	maxValue    float64
	minDiameter float64
	maxDiameter float64
	// plainDiameter is used when no size is positive.
	plainDiameter float64
}

// NewBubbleSizeScale builds the chart-wide scale from every series' bubble
// values.
// plainDiameter is the marker drawn when the chart has no positive size.
func NewBubbleSizeScale(series []ChartSeries, minDiameter, maxDiameter, plainDiameter float64) *BubbleSizeScale {
	maxValue := 0.0
	for _, s := range series {
		for _, v := range s.BubbleValues {
			if v > maxValue && !math.IsInf(v, 1) {
				maxValue = v
			}
		}
	}
	if minDiameter < 0 {
		minDiameter = 0
	}
	if maxDiameter < minDiameter {
		maxDiameter = minDiameter
	}
	return &BubbleSizeScale{maxValue: maxValue, minDiameter: minDiameter, maxDiameter: maxDiameter, plainDiameter: plainDiameter}
}

// Diameter returns the marker diameter for a size value.
func (s *BubbleSizeScale) Diameter(v float64) float64 {
	if s.maxValue <= 0 {
		return s.plainDiameter
	}
	if !(v > 0) {
		return s.minDiameter
	}
	d := s.maxDiameter * math.Sqrt(math.Min(v, s.maxValue)/s.maxValue)
	return math.Max(d, s.minDiameter)
}

// MaxValue is the size value drawn at the maximum diameter.
func (s *BubbleSizeScale) MaxValue() float64 { return s.maxValue }
