package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Regression tests for the go-slide-creator-csclk svggen batch.

var csclkTextYRe = regexp.MustCompile(`<t(?:ext|span)[^>]*? y="([0-9.]+)"[^>]*>([^<]*)`)
var csclkViewBoxRe = regexp.MustCompile(`viewBox="0 0 ([0-9.]+) ([0-9.]+)"`)

func maxTextBaseline(t *testing.T, svg string) (maxY, vbH float64) {
	t.Helper()
	for _, m := range csclkTextYRe.FindAllStringSubmatch(svg, -1) {
		y, _ := strconv.ParseFloat(m[1], 64)
		if y > maxY {
			maxY = y
		}
	}
	vb := csclkViewBoxRe.FindStringSubmatch(svg)
	if vb == nil {
		t.Fatal("no viewBox")
	}
	vbH, _ = strconv.ParseFloat(vb[2], 64)
	return maxY, vbH
}

// csclk.87: a wrapped bottom legend must reserve every row it draws.
func TestBarLegendWrappedRowsStayInsideCanvas(t *testing.T) {
	for _, size := range [][2]int{{600, 250}, {600, 300}} {
		req := &RequestEnvelope{Type: "bar_chart", Output: OutputSpec{Width: size[0], Height: size[1]}, Data: map[string]any{
			"categories": []any{"FY22", "FY23", "FY24", "FY25"},
			"series": []any{
				map[string]any{"name": "Contract logistics", "values": []any{4.2, 5.1, 6.3, 8.1}},
				map[string]any{"name": "Last-mile delivery", "values": []any{2.2, 3.1, 3.3, 4.1}},
				map[string]any{"name": "Freight brokerage", "values": []any{1.2, 1.1, 1.3, 1.1}},
			},
		}}
		doc, err := Render(req)
		if err != nil {
			t.Fatal(err)
		}
		if maxY, vbH := maxTextBaseline(t, string(doc.Content)); maxY > vbH {
			t.Errorf("%v: text baseline %.1f below viewBox height %.1f", size, maxY, vbH)
		}
	}
}

// csclk.13: an overflowing middle-aligned vertical legend must not start above
// the canvas.
func TestVerticalLegendOverflowStartsInsideBounds(t *testing.T) {
	cats := make([]any, 60)
	vals := make([]any, 60)
	for i := range cats {
		cats[i] = "Category " + strconv.Itoa(i)
		vals[i] = float64(i + 1)
	}
	req := &RequestEnvelope{Type: "pie_chart", Data: map[string]any{"categories": cats, "values": vals}}
	doc, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`y="-[0-9.]+"[^>]*>Category`).FindString(string(doc.Content)); m != "" {
		t.Errorf("legend item drawn above canvas: %s", m)
	}
}

// csclk.9: unparseable, missing and reversed dates are rejected.
func TestGanttTimelineRejectBadDates(t *testing.T) {
	cases := []struct {
		typ  string
		data map[string]any
		want string
	}{
		{"gantt", map[string]any{"tasks": []any{map[string]any{"label": "Discovery", "start_date": "Q3 FY25", "end_date": "2024-06-01"}}}, "start_date"},
		{"gantt", map[string]any{"tasks": []any{map[string]any{"label": "Discovery", "end_date": "2024-06-01"}}}, "missing start_date"},
		{"gantt", map[string]any{"tasks": []any{map[string]any{"label": "D", "start": "2024-05-01", "end": "2024-01-01"}}}, "before"},
		{"timeline", map[string]any{"activities": []any{map[string]any{"label": "D", "start": "2024-05-01", "end": "2024-01-01"}}}, "before"},
		{"timeline", map[string]any{"activities": []any{map[string]any{"label": "D", "start_date": "someday", "end_date": "2024-01-01"}}}, "start_date"},
	}
	for _, c := range cases {
		_, err := Render(&RequestEnvelope{Type: c.typ, Data: c.data})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %v: err = %v, want mention of %q", c.typ, c.data, err, c.want)
		}
	}
}

// csclk.80: a fine time_unit over a long range is coarsened, not rendered as
// one tick per day.
func TestTimelineTickCountBounded(t *testing.T) {
	for _, typ := range []string{"timeline", "gantt"} {
		key := "activities"
		if typ == "gantt" {
			key = "tasks"
		}
		req := &RequestEnvelope{Type: typ, Data: map[string]any{"time_unit": "day", key: []any{
			map[string]any{"label": "Era", "start": "1900-01-01", "end": "2100-01-01"},
		}}}
		start := time.Now()
		doc, err := Render(req)
		if err != nil {
			t.Fatal(err)
		}
		if el := time.Since(start); el > 3*time.Second {
			t.Errorf("%s render took %v", typ, el)
		}
		if n := len(doc.Content); n > 2_000_000 {
			t.Errorf("%s SVG is %d bytes", typ, n)
		}
	}
}

// csclk.6 / csclk.7 / csclk.16: negative values are rejected where the chart
// cannot draw them.
func TestNegativeValuesRejected(t *testing.T) {
	cases := []*RequestEnvelope{
		{Type: "stacked_area_chart", Data: map[string]any{"categories": []any{"Q1", "Q2"}, "series": []any{
			map[string]any{"name": "Revenue", "values": []any{10.0, 12.0}},
			map[string]any{"name": "Refunds", "values": []any{-5.0, -6.0}},
		}}},
		{Type: "funnel_chart", Data: map[string]any{"values": []any{100.0, -40.0, 20.0}, "categories": []any{"Leads", "Lost", "Won"}}},
		{Type: "radar_chart", Data: map[string]any{"categories": []any{"A", "B", "C", "D"}, "series": []any{
			map[string]any{"name": "S", "values": []any{-5.0, 5.0, 10.0, -10.0}},
		}}},
	}
	for _, req := range cases {
		if _, err := Render(req); err == nil || !strings.Contains(err.Error(), ">= 0") {
			t.Errorf("%s: err = %v, want non-negative rejection", req.Type, err)
		}
	}
}

// csclk.10: data_labels: true turns value labels on.
func TestDataLabelsBoolTrue(t *testing.T) {
	if dl := extractDataLabels(map[string]any{"data_labels": true}); dl == nil {
		t.Error("data_labels: true should enable labels")
	}
	if dl := extractDataLabels(map[string]any{"data_labels": false}); dl != nil {
		t.Error("data_labels: false should leave labels off")
	}
}

// csclk.11 / csclk.12: waterfall type sets direction, totals are checked and
// small deltas never print as -0.0.
func TestWaterfallTypeSignTotalsAndPrecision(t *testing.T) {
	req := &RequestEnvelope{Type: "waterfall", Data: map[string]any{"points": []any{
		map[string]any{"label": "A", "value": 10.0, "type": "increase"},
		map[string]any{"label": "B", "value": 5.0, "type": "decrease"},
		map[string]any{"label": "T", "value": 99.0, "type": "total"},
	}}}
	data, err := parseWaterfallData(req)
	if err != nil {
		t.Fatal(err)
	}
	if data.Points[1].Value != -5 {
		t.Errorf("decrease value = %v, want -5", data.Points[1].Value)
	}
	builder, _, err := (&WaterfallDiagram{}).RenderWithBuilder(req)
	if err != nil {
		t.Fatal(err)
	}
	if findFindingByCode(builder.Findings(), FindingWaterfallTotalMismatch) == nil {
		t.Error("expected chart.waterfall_total_mismatch for total 99 vs running 5")
	}

	small := &RequestEnvelope{Type: "waterfall", Data: map[string]any{"points": []any{
		map[string]any{"label": "Start", "value": 10.0, "type": "total"},
		map[string]any{"label": "Tiny", "value": -0.04, "type": "decrease"},
		map[string]any{"label": "End", "value": 9.96, "type": "total"},
	}}}
	doc, err := Render(small)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc.Content), ">-0.0<") {
		t.Error("small delta printed as -0.0")
	}
	if !strings.Contains(string(doc.Content), "-0.04") {
		t.Error("small delta should print as -0.04")
	}
}

// csclk.14: a reversed axis maps points instead of clamping them to one edge.
func TestMatrixReversedAxisDoesNotClamp(t *testing.T) {
	req := &RequestEnvelope{Type: "matrix_2x2", Data: map[string]any{
		"x_min": 100.0, "x_max": 0.0,
		"points": []any{map[string]any{"label": "A", "x": 20.0, "y": 30.0}},
	}}
	builder, _, err := (&Matrix2x2Diagram{}).RenderWithBuilder(req)
	if err != nil {
		t.Fatal(err)
	}
	if f := findFindingByCode(builder.Findings(), FindingPointOutOfRange); f != nil {
		t.Errorf("unexpected out-of-range finding: %s", f.Message)
	}
}

// csclk.16: radar ring labels go through the value formatter.
func TestRadarRingLabelsUsePercentFormat(t *testing.T) {
	req := &RequestEnvelope{Type: "radar_chart", Style: StyleSpec{ValueFormat: &ValueFormatSpec{Style: "percent"}}, Data: map[string]any{
		"categories": []any{"A", "B", "C"},
		"series":     []any{map[string]any{"name": "S", "values": []any{0.02, 0.05, 0.08}}},
	}}
	doc, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc.Content), "%<") {
		t.Error("radar ring labels should carry the percent format")
	}
}

// csclk.19: a chart in a short embedded cell keeps its x labels inside the
// viewBox when placement typography enlarges the fonts.
func TestShortEmbeddedLineChartLabelsInsideViewBox(t *testing.T) {
	for _, h := range []int{80, 100, 120} {
		req := &RequestEnvelope{Type: "line_chart", Output: OutputSpec{Width: 828, Height: h},
			Style: StyleSpec{PlacementWidthPt: 828 * 0.75, PlacementHeightPt: float64(h) * 0.75, MinReadablePt: 12},
			Data: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series":     []any{map[string]any{"name": "Revenue", "values": []any{4.2, 5.1, 6.3, 8.1}}},
			}}
		doc, err := Render(req)
		if err != nil {
			t.Fatal(err)
		}
		if maxY, vbH := maxTextBaseline(t, string(doc.Content)); maxY > vbH {
			t.Errorf("h=%d: text baseline %.1f below viewBox height %.1f", h, maxY, vbH)
		}
	}
}
