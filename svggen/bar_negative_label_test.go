package svggen

import (
	"math"
	"strings"
	"testing"
)

// go-slide-creator-5na8e: a negative bar's value label hangs below the bar end,
// and the category labels sit just below the plot. With [9, 4, 0, -2] Nice()
// put the domain floor exactly at -2, so "−2" printed on top of "East" — the
// top headroom only ever grew the max. The lower edge must keep measured room
// for that label, in mixed-sign and all-negative data alike, without moving
// the zero baseline or the positive labels' top clearance.

func TestBarNegativeValueLabelsClearCategoryLabels(t *testing.T) {
	cases := []struct {
		name       string
		categories []string
		values     []float64
	}{
		{"the reported regional savings", []string{"North", "West", "South", "East"}, []float64{9, 4, 0, -2}},
		{"mixed sign, minimum on a nice tick", []string{"Q1", "Q2", "Q3", "Q4"}, []float64{100, 60, -20, -40}},
		{"all negative, minimum on a nice tick", []string{"North", "West", "South", "East"}, []float64{-2, -4, -6, -8}},
		{"long category labels", []string{"Northern Europe", "Western Europe", "Southern Europe", "Eastern Europe and Central Asia"}, []float64{9, 4, 0, -2}},
	}
	sizes := []struct{ w, h float64 }{{900, 500}, {600, 300}}
	for _, tc := range cases {
		for _, size := range sizes {
			t.Run(tc.name, func(t *testing.T) {
				b := NewSVGBuilder(size.w, size.h)
				cfg := DefaultBarChartConfig(size.w, size.h)
				cfg.ShowValues = true
				data := ChartData{
					Title:      "Annual net savings (€m)",
					Categories: tc.categories,
					Series:     []ChartSeries{{Name: "Savings", Values: tc.values}},
				}
				if err := NewBarChart(b, cfg).Draw(data); err != nil {
					t.Fatalf("draw: %v", err)
				}
				if findings := b.textOverlapFindings(); len(findings) > 0 {
					t.Errorf("%.0fx%.0f: text overlap: %s", size.w, size.h, findings[0].Message)
				}
				for _, v := range tc.values {
					if v >= 0 {
						continue
					}
					label := TrueMinus(formatValueGrouped(v, "%.0f"))
					box, ok := drawnTextBoxFor(b, label)
					if !ok {
						t.Fatalf("%.0fx%.0f: value label %q was not drawn", size.w, size.h, label)
					}
					// Every text drawn below the label that shares its column
					// (the category name) must start clearly below it.
					for _, other := range b.textBoxes {
						if other.text == label || other.rect.Y <= box.Y {
							continue
						}
						if other.rect.X >= box.X+box.W || other.rect.X+other.rect.W <= box.X {
							continue
						}
						if gap := other.rect.Y - (box.Y + box.H); gap < 2 {
							t.Errorf("%.0fx%.0f: %q sits %.1fpt above %q; want a clear gap of at least 2pt", size.w, size.h, label, gap, other.text)
						}
					}
				}
			})
		}
	}
}

func drawnTextBoxFor(b *SVGBuilder, text string) (Rect, bool) {
	for _, tb := range b.textBoxes {
		if strings.TrimSpace(tb.text) == text {
			return tb.rect, true
		}
	}
	return Rect{}, false
}

func TestBarLinearYScaleReservesLowerClearance(t *testing.T) {
	const plotH, clearance = 300.0, 24.0
	cases := []struct {
		name       string
		yMin, yMax float64
	}{
		{"mixed sign, floor exactly on a tick", -2, 9},
		{"mixed sign, deep negative", -40, 100},
		{"all negative", -8, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := barLinearYScale(tc.yMin, tc.yMax, plotH, clearance)
			lo, hi := s.DomainBounds()
			if room := plotH - s.Scale(tc.yMin); room < clearance-1e-6 {
				t.Errorf("room below the lowest bar = %.2fpt, want >= %.0fpt (domain %v..%v)", room, clearance, lo, hi)
			}
			// The zero baseline stays in the domain; all-negative data keeps
			// zero as its ceiling rather than gaining positive headroom.
			if lo > 0 || hi < 0 {
				t.Errorf("domain %v..%v lost the zero baseline", lo, hi)
			}
			if tc.yMax <= 0 && hi != 0 {
				t.Errorf("all-negative domain max = %v, want the zero baseline at the top", hi)
			}
			// Positive top clearance is unchanged by the lower reservation.
			if tc.yMax > 0 {
				top := barLinearYScale(tc.yMin, tc.yMax, plotH, 0)
				if _, plainHi := top.DomainBounds(); hi < plainHi || hi <= tc.yMax {
					t.Errorf("domain max = %v, want >= %v (top headroom) and > %v", hi, plainHi, tc.yMax)
				}
			}
		})
	}

	// No negative values or no labels: the scale is exactly the old one.
	for _, c := range [][2]float64{{0, 100}, {-2, 9}} {
		got := barLinearYScale(c[0], c[1], plotH, 0)
		want := NewLinearScale(c[0], withBarTopHeadroom(c[1]))
		want.SetRangeLinear(plotH, 0)
		want.Nice(true)
		gl, gh := got.DomainBounds()
		wl, wh := want.DomainBounds()
		if math.Abs(gl-wl) > 1e-9 || math.Abs(gh-wh) > 1e-9 {
			t.Errorf("zero clearance changed the domain for %v: got %v..%v, want %v..%v", c, gl, gh, wl, wh)
		}
	}
}
