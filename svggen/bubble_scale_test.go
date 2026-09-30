package svggen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-b7qqg.21: sizes 1:2:4 must draw areas 1:2:4.
func TestBubbleSizeScale_AreaProportional(t *testing.T) {
	s := NewBubbleSizeScale([]ChartSeries{{BubbleValues: []float64{1, 2, 4}}}, 3, 84, 8)
	area := func(v float64) float64 { d := s.Diameter(v); return d * d }
	if got := area(2) / area(1); math.Abs(got-2) > 1e-9 {
		t.Errorf("area(2)/area(1) = %v, want 2", got)
	}
	if got := area(4) / area(1); math.Abs(got-4) > 1e-9 {
		t.Errorf("area(4)/area(1) = %v, want 4", got)
	}
	if got := s.Diameter(4); got != 84 {
		t.Errorf("largest size drew diameter %v, want the 84 maximum", got)
	}
}

// Zero and negative sizes have no area: they draw at the visibility floor. A
// chart with no positive size at all draws plain scatter markers.
func TestBubbleSizeScale_ZeroNegativePolicy(t *testing.T) {
	s := NewBubbleSizeScale([]ChartSeries{{BubbleValues: []float64{0, 10}}}, 3, 84, 8)
	if got := s.Diameter(0); got != 3 {
		t.Errorf("zero size drew %v, want the 3pt floor", got)
	}
	if got := s.Diameter(-5); got != 3 {
		t.Errorf("negative size drew %v, want the 3pt floor", got)
	}
	if got := s.Diameter(1e-9); got != 3 {
		t.Errorf("near-zero size drew %v, want the 3pt floor", got)
	}
	none := NewBubbleSizeScale([]ChartSeries{{BubbleValues: []float64{0, 0}}}, 3, 84, 8)
	if got := none.Diameter(0); got != 8 {
		t.Errorf("all-zero chart drew %v, want the plain 8pt marker", got)
	}
}

// go-slide-creator-b7qqg.20: the size domain is chart-wide. The same size in
// different series — including singleton and constant-size series — draws the
// same bubble, and adding a series with no new maximum changes nothing.
func TestBubbleSizeScale_ChartWideDomain(t *testing.T) {
	series := []ChartSeries{
		{Name: "A", BubbleValues: []float64{5, 10}},
		{Name: "B", BubbleValues: []float64{10, 20}},
		{Name: "Singleton", BubbleValues: []float64{10}},
		{Name: "Constant", BubbleValues: []float64{10, 10, 10}},
	}
	s := NewBubbleSizeScale(series, 3, 84, 8)
	s2 := NewBubbleSizeScale(series[:2], 3, 84, 8)
	if s.Diameter(10) != s2.Diameter(10) {
		t.Errorf("adding series without a new maximum changed size 10: %v vs %v", s.Diameter(10), s2.Diameter(10))
	}
	if s.Diameter(20) != 84 {
		t.Errorf("chart maximum 20 drew %v, want 84", s.Diameter(20))
	}
}

// End to end: in the review specimen (Region A sizes [5,10], Region B [10,20])
// the two size-10 bubbles must be identical and areas proportional.
func TestBubbleChart_RenderedRadiiShareOneAreaScale(t *testing.T) {
	doc, err := Render(&RequestEnvelope{
		Type: "bubble_chart",
		Data: map[string]any{"series": []any{
			map[string]any{"name": "Region A", "values": []any{10.0, 20.0}, "x_values": []any{1.0, 2.0}, "bubble_values": []any{5.0, 10.0}},
			map[string]any{"name": "Region B", "values": []any{15.0, 25.0}, "x_values": []any{3.0, 4.0}, "bubble_values": []any{10.0, 20.0}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Circles are drawn as two half arcs "A r r ...".
	re := regexp.MustCompile(`A([0-9.]+) ([0-9.]+)`)
	counts := map[float64]int{}
	rMax := 0.0
	for _, m := range re.FindAllStringSubmatch(string(doc.Content), -1) {
		r, _ := strconv.ParseFloat(m[1], 64)
		counts[r]++
		rMax = math.Max(rMax, r)
	}
	near := func(want float64) int {
		n := 0
		for r, c := range counts {
			if math.Abs(r-want) < 0.01 {
				n += c
			}
		}
		return n
	}
	if n := near(rMax); n != 2 {
		t.Errorf("size 20 should be one bubble (2 arcs) at the max radius, got %d arcs; radii %v", n, counts)
	}
	if n := near(rMax / math.Sqrt2); n != 4 {
		t.Errorf("both size-10 bubbles should share radius rMax/sqrt2 = %.4f (4 arcs), got %d; radii %v", rMax/math.Sqrt2, n, counts)
	}
	if n := near(rMax / 2); n != 2 {
		t.Errorf("size 5 should have radius rMax/2 = %.4f (2 arcs), got %d; radii %v", rMax/2, n, counts)
	}
}

func TestBubbleChart_RejectsNegativeSize(t *testing.T) {
	_, err := Render(&RequestEnvelope{
		Type: "bubble_chart",
		Data: map[string]any{"series": []any{
			map[string]any{"name": "A", "values": []any{1.0, 2.0}, "x_values": []any{1.0, 2.0}, "bubble_values": []any{5.0, -1.0}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "bubble_values[1]") {
		t.Fatalf("negative bubble size: err = %v, want a validation error naming bubble_values[1]", err)
	}
}
