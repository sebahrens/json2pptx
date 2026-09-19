package svggen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// bridgeCategoryLabels are the consulting-deck repro's own labels: long enough
// that AdaptXLabels ellipsizes them, which is the condition that used to
// collapse a chart's x scale.
var bridgeCategoryLabels = []string{
	"Ausgangsbasis Geschaeftsjahr 2024", "Rechtsschutzversicherungsgesellschaften",
	"Enterprise Platform Subscriptions", "Mid-market Professional Services",
	"Donaudampfschifffahrtsgesellschaft", "Public Sector Framework Contracts",
	"Kraftfahrzeughaftpflichtversicherung", "Managed Security Operations (EMEA)",
	"Arbeitsunfaehigkeitsbescheinigung", "Channel Partner Resale - Nordics",
	"Betriebswirtschaftliche Auswertung", "Customer Success Expansion ARR",
	"Datenschutzgrundverordnung Audit", "Endstand Geschaeftsjahr 2025",
}

// filledBarGeometry returns the x, width and height of every filled path in an
// SVG — the bars, in draw order.
func filledBarGeometry(svg string) [][4]float64 {
	pathRe := regexp.MustCompile(`<path ([^>]*)>`)
	dRe := regexp.MustCompile(`d="([^"]*)"`)
	styleRe := regexp.MustCompile(`style="([^"]*)"`)
	numRe := regexp.MustCompile(`-?\d+\.?\d*`)

	var out [][4]float64
	for _, m := range pathRe.FindAllStringSubmatch(svg, -1) {
		style := styleRe.FindStringSubmatch(m[1])
		if style != nil && strings.Contains(style[1], "fill:none") {
			continue
		}
		d := dRe.FindStringSubmatch(m[1])
		if d == nil {
			continue
		}
		nums := numRe.FindAllString(d[1], -1)
		if len(nums) < 4 {
			continue
		}
		var xs, ys []float64
		for i, n := range nums {
			v, err := strconv.ParseFloat(n, 64)
			if err != nil {
				continue
			}
			if i%2 == 0 {
				xs = append(xs, v)
			} else {
				ys = append(ys, v)
			}
		}
		if len(xs) == 0 || len(ys) == 0 {
			continue
		}
		minX, maxX := minMax(xs)
		minY, maxY := minMax(ys)
		out = append(out, [4]float64{minX, maxX - minX, minY, maxY - minY})
	}
	return out
}

func minMax(vs []float64) (float64, float64) {
	lo, hi := math.MaxFloat64, -math.MaxFloat64
	for _, v := range vs {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	return lo, hi
}

// A 14-step bridge with long German category labels rendered as three visible
// bars at the far left and eleven invisible ones, with the value labels piled
// on the y-axis. Every bar was drawn at the same x: AdaptXLabels ellipsized the
// labels IN PLACE, the scale was built from the ellipsized strings, and each
// bar looked its position up by the original — so every lookup missed and
// CategoricalScale.Scale returned its range minimum (go-slide-creator-4xsi).
func TestWaterfallLongLabelsDoNotCollapseTheXScale(t *testing.T) {
	labels := bridgeCategoryLabels
	values := []float64{1250, 340, -1800, 2.5, -0.8, 0, 95, -420, 12, -5, 610, -33, 7, 57.7}
	points := make([]WaterfallDataPoint, len(labels))
	for i := range labels {
		points[i] = WaterfallDataPoint{Label: labels[i], Value: values[i]}
		switch {
		case i == 0 || i == len(labels)-1:
			points[i].Type = WaterfallTypeTotal
		case values[i] < 0:
			points[i].Type = WaterfallTypeDecrease
		default:
			points[i].Type = WaterfallTypeIncrease
		}
	}

	b := NewSVGBuilder(900, 540)
	chart := NewWaterfallChart(b, DefaultWaterfallChartConfig(900, 540))
	if drawErr := chart.Draw(WaterfallData{
		Title:    "waterfall chart title (stress)",
		Points:   points,
		Footnote: "EUR thousand — crosses zero mid-walk",
	}); drawErr != nil {
		t.Fatalf("draw: %v", drawErr)
	}
	raw, err := b.RenderToBytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(raw)

	bars := filledBarGeometry(svg)
	if len(bars) < len(labels)-1 { // the zero-value step draws no bar
		t.Fatalf("drew %d bars for %d steps", len(bars), len(labels))
	}

	seen := map[float64]bool{}
	for i, bar := range bars {
		x, height := bar[0], bar[3]
		if seen[x] {
			t.Fatalf("bar %d is at x=%.1f, where another bar already is — the x scale has collapsed", i, x)
		}
		seen[x] = true
		// Every real step has to be visible: a -0.8 on a 3,000-unit axis is a
		// fraction of a pixel without a floor.
		if height < waterfallMinBarHeight {
			t.Errorf("bar %d is %.2fpt tall, below the %.1fpt minimum", i, height, waterfallMinBarHeight)
		}
	}

	// The bars must span the plot, not huddle in its first slot.
	var minX, maxX = math.MaxFloat64, -math.MaxFloat64
	for _, bar := range bars {
		minX = math.Min(minX, bar[0])
		maxX = math.Max(maxX, bar[0]+bar[1])
	}
	if span := maxX - minX; span < 600 {
		t.Errorf("the bars span %.0fpt of a 900pt canvas; they used to pile into the first 45", span)
	}

	// The footnote is inside the canvas, in full.
	if !strings.Contains(svg, "EUR thousand") {
		t.Error("the footnote is missing entirely")
	}
}

// The same collapse hit every Cartesian chart that adapts its x labels: the bar
// chart builds its scale from the adapted categories and looks each bar up by
// the original. It shares the fix, so it shares the guard.
func TestBarChartLongLabelsDoNotCollapseTheXScale(t *testing.T) {
	values := make([]float64, len(bridgeCategoryLabels))
	for i := range values {
		values[i] = float64(10 + i*7)
	}

	// A narrow canvas is what drives the labels past wrapping and rotation into
	// the ellipsis path, which is the condition that collapsed the scale.
	b := NewSVGBuilder(520, 380)
	chart := NewBarChart(b, DefaultBarChartConfig(520, 380))
	if err := chart.Draw(ChartData{
		Title:      "bar chart with long labels",
		Categories: bridgeCategoryLabels,
		Series:     []ChartSeries{{Name: "Revenue", Values: values}},
	}); err != nil {
		t.Fatalf("draw: %v", err)
	}
	raw, err := b.RenderToBytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	bars := filledBarGeometry(string(raw))
	if len(bars) < len(bridgeCategoryLabels) {
		t.Fatalf("drew %d bars for %d categories", len(bars), len(bridgeCategoryLabels))
	}
	seen := map[float64]bool{}
	for i, bar := range bars[:len(bridgeCategoryLabels)] {
		if seen[bar[0]] {
			t.Fatalf("bar %d is at x=%.1f, where another bar already is — the x scale has collapsed", i, bar[0])
		}
		seen[bar[0]] = true
	}
}
