package svggen

import "testing"

// salesPitchGroupedBars is the semantic sales_pitch slide-3 fixture from the
// 2026-09 deck-quality review (go-slide-creator-t2ka): the "Batch reporting"
// direct label was centered over its short last bar and drawn on top of the
// taller neighbouring "Real-time decisioning" bar.
func salesPitchGroupedBars() ChartData {
	return ChartData{
		Title:      "Decision-analytics spend ($B)",
		Categories: []string{"2023", "2024", "2025", "2026"},
		Series: []ChartSeries{
			{Name: "Batch reporting", Values: []float64{12, 11, 10, 9}},
			{Name: "Real-time decisioning", Values: []float64{6, 10, 16, 24}},
		},
	}
}

// drawBarForDirectLabels renders a grouped bar chart with direct labels
// preferred and returns the chart (its config reflects the final labeling
// decision) plus the builder.
func drawBarForDirectLabels(t *testing.T, w, h float64, data ChartData) (*BarChart, *SVGBuilder) {
	t.Helper()
	b := NewSVGBuilder(w, h)
	cfg := DefaultBarChartConfig(w, h)
	cfg.ShowTitle = data.Title != ""
	cfg.PreferDirectLabels = true
	bc := NewBarChart(b, cfg)
	if err := bc.Draw(data); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	return bc, b
}

func TestBarDirectLabels_NoLabelIntersectsBar(t *testing.T) {
	sizes := []struct{ w, h float64 }{{480, 320}, {600, 360}, {800, 450}, {1600, 900}}
	for _, sz := range sizes {
		bc, b := drawBarForDirectLabels(t, sz.w, sz.h, salesPitchGroupedBars())
		if !useDirectLabels(bc.config.ChartConfig, 2) {
			continue // fell back to legend: nothing inline to collide
		}
		layout := ComputeCartesianLayout(bc.config.ChartConfig, b.StyleGuide(), "Decision-analytics spend ($B)", "", "", 2)
		colors := bc.getColors(b.StyleGuide(), 2)
		labels, bars := barDirectLabelGeometry(b, b.StyleGuide(), salesPitchGroupedBars(), layout.PlotArea, colors, bc.config)
		for _, l := range labels {
			for _, r := range bars {
				if l.Box.Inset(1, 1, 1, 1).Intersects(r) {
					t.Errorf("%vx%v: direct label %q bbox %+v intersects bar %+v", sz.w, sz.h, l.Text, l.Box, r)
				}
			}
		}
	}
}

func TestBarDirectLabels_CollisionFallsBackToLegend(t *testing.T) {
	bc, _ := drawBarForDirectLabels(t, 480, 320, salesPitchGroupedBars())
	if bc.config.PreferDirectLabels {
		t.Fatalf("sales-pitch fixture at 480x320: direct labels collide with bars, expected legend fallback")
	}
	if !bc.config.ShowLegend {
		t.Errorf("legend must be enabled after direct-label fallback")
	}
}

func TestBarDirectLabels_KeepsInlineLabelsWhenTheyFit(t *testing.T) {
	data := ChartData{
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []ChartSeries{
			{Name: "A", Values: []float64{10, 12, 14, 16}},
			{Name: "B", Values: []float64{11, 13, 15, 17}},
		},
	}
	bc, _ := drawBarForDirectLabels(t, 1600, 900, data)
	if !bc.config.PreferDirectLabels {
		t.Errorf("short labels on a wide chart should keep the direct-label path")
	}
}

func TestBarDirectLabelsCollide(t *testing.T) {
	bar := Rect{X: 100, Y: 50, W: 20, H: 100}
	cases := []struct {
		name   string
		labels []barDirectLabel
		want   bool
	}{
		{"clear above bar", []barDirectLabel{{Box: Rect{X: 95, Y: 30, W: 30, H: 10}}}, false},
		{"overlaps bar", []barDirectLabel{{Box: Rect{X: 95, Y: 45, W: 30, H: 10}}}, true},
		{"off canvas", []barDirectLabel{{Box: Rect{X: 190, Y: 10, W: 30, H: 10}}}, true},
		{"labels overlap each other", []barDirectLabel{
			{Box: Rect{X: 10, Y: 10, W: 30, H: 10}},
			{Box: Rect{X: 20, Y: 12, W: 30, H: 10}},
		}, true},
	}
	for _, tc := range cases {
		if got := barDirectLabelsCollide(tc.labels, []Rect{bar}, 200); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
