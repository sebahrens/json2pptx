package svggen

import (
	"errors"
	"strings"
	"testing"
)

// go-slide-creator-929jm: an EBITDA bridge 24.0 → 31.0 rendered with its
// y-axis starting at 22, so the opening total was a stub and the walk
// misread; a line of 2024–2026 counts (11 / 9 / 7) started at 2 and
// exaggerated the trend. Consulting bridges and counts start at zero; an
// author who wants a zoomed axis says so with data.y_min / data.y_max.

func ebitdaBridgePoints() []WaterfallDataPoint {
	return []WaterfallDataPoint{
		{Label: "2023 EBITDA", Value: 24.0, Type: WaterfallTypeTotal},
		{Label: "Price", Value: 6.5, Type: WaterfallTypeIncrease},
		{Label: "Volume", Value: 3.2, Type: WaterfallTypeIncrease},
		{Label: "Raw material", Value: -4.1, Type: WaterfallTypeDecrease},
		{Label: "Opex", Value: 1.4, Type: WaterfallTypeIncrease},
		{Label: "2025 EBITDA", Value: 31.0, Type: WaterfallTypeTotal},
	}
}

func ebitdaBridgeData(extra map[string]any) map[string]any {
	data := map[string]any{
		"points": []any{
			map[string]any{"label": "2023 EBITDA", "value": 24.0, "type": "total"},
			map[string]any{"label": "Price", "value": 6.5, "type": "increase"},
			map[string]any{"label": "Volume", "value": 3.2, "type": "increase"},
			map[string]any{"label": "Raw material", "value": -4.1, "type": "decrease"},
			map[string]any{"label": "Opex", "value": 1.4, "type": "increase"},
			map[string]any{"label": "2025 EBITDA", "value": 31.0, "type": "total"},
		},
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func f64(v float64) *float64 { return &v }

func TestWaterfallDomain_StartsAtZeroWhenNoTotalIsNegative(t *testing.T) {
	wc := &WaterfallChart{config: DefaultWaterfallChartConfig(800, 600)}

	t.Run("EBITDA bridge", func(t *testing.T) {
		min, max := wc.calculateDomain(ebitdaBridgePoints(), AxisBounds{})
		if min != 0 {
			t.Errorf("domain min = %v, want 0: every running total is positive, so the bridge starts at zero", min)
		}
		if max < 34.7 {
			t.Errorf("domain max = %v, must cover the 34.7 peak running total", max)
		}
	})

	t.Run("NOI bridge far from zero still starts at zero", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Prior Year NOI", Value: 485, Type: WaterfallTypeTotal},
			{Label: "Same-Store Growth", Value: 28, Type: WaterfallTypeIncrease},
			{Label: "Acquisitions", Value: 45, Type: WaterfallTypeIncrease},
			{Label: "Dispositions", Value: -18, Type: WaterfallTypeDecrease},
			{Label: "Vacancy Impact", Value: -12, Type: WaterfallTypeDecrease},
			{Label: "Current NOI", Value: 528, Type: WaterfallTypeTotal},
		}
		min, _ := wc.calculateDomain(points, AxisBounds{})
		if min != 0 {
			t.Errorf("domain min = %v, want 0: a zoomed bridge is an authored choice (data.y_min), not a default", min)
		}
	})

	t.Run("a negative running total is still covered", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Start", Value: 500, Type: WaterfallTypeTotal},
			{Label: "Big Loss", Value: -600, Type: WaterfallTypeDecrease},
			{Label: "Recovery", Value: 200, Type: WaterfallTypeIncrease},
			{Label: "End", Value: 100, Type: WaterfallTypeTotal},
		}
		min, max := wc.calculateDomain(points, AxisBounds{})
		if min > -100 {
			t.Errorf("domain min = %v, must cover the -100 running total", min)
		}
		if max < 500 {
			t.Errorf("domain max = %v, must cover 500", max)
		}
	})

	t.Run("authored y_min zooms the axis", func(t *testing.T) {
		min, max := wc.calculateDomain(ebitdaBridgePoints(), AxisBounds{Min: f64(20), Max: f64(36)})
		if min != 20 || max != 36 {
			t.Errorf("domain = [%v, %v], want the authored [20, 36]", min, max)
		}
	})

	t.Run("small fees off a high baseline still start at zero", func(t *testing.T) {
		points := []WaterfallDataPoint{
			{Label: "Starting Value", Value: 1000, Type: WaterfallTypeTotal},
			{Label: "Fee A", Value: -5, Type: WaterfallTypeDecrease},
			{Label: "Fee B", Value: -3, Type: WaterfallTypeDecrease},
			{Label: "Net Value", Value: 992, Type: WaterfallTypeTotal},
		}
		min, max := wc.calculateDomain(points, AxisBounds{})
		if min != 0 {
			t.Errorf("domain min = %v, want 0", min)
		}
		if max <= 1000 {
			t.Errorf("domain max = %v, must leave headroom above 1000", max)
		}
	})

	t.Run("single total bar", func(t *testing.T) {
		min, max := wc.calculateDomain([]WaterfallDataPoint{{Label: "Total", Value: 500, Type: WaterfallTypeTotal}}, AxisBounds{})
		if min != 0 || max <= 500 {
			t.Errorf("domain = [%v, %v], want [0, >500]", min, max)
		}
	})

	t.Run("no points", func(t *testing.T) {
		if min, max := wc.calculateDomain(nil, AxisBounds{}); min != 0 || max != 1 {
			t.Errorf("domain = [%v, %v], want the [0, 1] fallback", min, max)
		}
	})
}

func TestLineDomain_NonNegativeValuesStartAtZero(t *testing.T) {
	lc := NewLineChart(NewSVGBuilder(900, 500), DefaultLineChartConfig(900, 500))

	t.Run("counts start at zero", func(t *testing.T) {
		data := ChartData{
			Categories: []string{"2024", "2025", "2026"},
			Series: []ChartSeries{
				{Name: "Exceptions", Values: []float64{11, 9, 7}},
				{Name: "High findings", Values: []float64{4, 3, 2}},
			},
		}
		min, max := lc.calculateYDomain(data)
		if min != 0 {
			t.Errorf("line y-domain min = %v, want 0 for all-non-negative data", min)
		}
		if max < 11 {
			t.Errorf("line y-domain max = %v, must cover 11", max)
		}
	})

	t.Run("far-from-zero positives still start at zero", func(t *testing.T) {
		data := ChartData{
			Categories: []string{"FY21", "FY22", "FY23"},
			Series:     []ChartSeries{{Name: "Revenue", Values: []float64{128, 134, 141}}},
		}
		if min, _ := lc.calculateYDomain(data); min != 0 {
			t.Errorf("line y-domain min = %v, want 0; zoom in with data.y_min", min)
		}
	})

	t.Run("constant positive data starts at zero", func(t *testing.T) {
		data := ChartData{Categories: []string{"a", "b"}, Series: []ChartSeries{{Values: []float64{5, 5}}}}
		min, max := lc.calculateYDomain(data)
		if min != 0 || max <= 5 {
			t.Errorf("line y-domain = [%v, %v], want [0, >5]", min, max)
		}
	})

	t.Run("negative values keep their minimum", func(t *testing.T) {
		data := ChartData{Categories: []string{"a", "b"}, Series: []ChartSeries{{Values: []float64{-15, 22}}}}
		min, _ := lc.calculateYDomain(data)
		if min > -15 {
			t.Errorf("line y-domain min = %v, must cover -15", min)
		}
	})

	t.Run("authored bounds win", func(t *testing.T) {
		data := ChartData{
			Categories: []string{"FY21", "FY22", "FY23"},
			Series:     []ChartSeries{{Name: "Index", Values: []float64{100, 104, 109}}},
			Axis:       AxisBounds{Min: f64(90), Max: f64(120)},
		}
		min, max := lc.calculateYDomain(data)
		if min != 90 || max != 120 {
			t.Errorf("line y-domain = [%v, %v], want the authored [90, 120]", min, max)
		}
	})
}

func TestBarDomain_AuthoredBoundsApply(t *testing.T) {
	bc := NewBarChart(NewSVGBuilder(900, 500), DefaultBarChartConfig(900, 500))
	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series:     []ChartSeries{{Values: []float64{80, 90, 95}}},
		Axis:       AxisBounds{Min: f64(60), Max: f64(100)},
	}
	min, max := bc.calculateDomain(data)
	if min != 60 || max != 100 {
		t.Errorf("bar y-domain = [%v, %v], want the authored [60, 100]", min, max)
	}
}

func TestAxisBounds_Validation(t *testing.T) {
	cases := []struct {
		name      string
		req       *RequestEnvelope
		wantField string
		wantIn    string
	}{
		{
			name:      "waterfall y_min above the lowest running total cuts a bar",
			req:       &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": 25.0})},
			wantField: "data.y_min",
			wantIn:    "24",
		},
		{
			name:      "y_min must be below y_max",
			req:       &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": 30.0, "y_max": 30.0})},
			wantField: "data.y_min",
			wantIn:    "y_max",
		},
		{
			name:      "y_min must be a number",
			req:       &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": "twenty"})},
			wantField: "data.y_min",
			wantIn:    "number",
		},
		{
			name: "bar y_min above the smallest bar cuts it",
			req: &RequestEnvelope{Type: "bar_chart", Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series":     []any{map[string]any{"name": "S", "values": []any{80.0, 90.0, 95.0}}},
				"y_min":      85.0,
			}},
			wantField: "data.y_min",
			wantIn:    "80",
		},
		{
			name: "bar y_max below the tallest bar cuts it",
			req: &RequestEnvelope{Type: "bar_chart", Data: map[string]any{
				"categories": []any{"A", "B", "C"},
				"series":     []any{map[string]any{"name": "S", "values": []any{80.0, 90.0, 95.0}}},
				"y_max":      90.0,
			}},
			wantField: "data.y_max",
			wantIn:    "95",
		},
		{
			name: "line y_min above the lowest point clips the line",
			req: &RequestEnvelope{Type: "line_chart", Data: map[string]any{
				"categories": []any{"2024", "2025", "2026"},
				"series":     []any{map[string]any{"name": "S", "values": []any{11.0, 9.0, 7.0}}},
				"y_min":      8.0,
			}},
			wantField: "data.y_min",
			wantIn:    "7",
		},
		{
			name: "stacked bar y_max below the tallest stack cuts it",
			req: &RequestEnvelope{Type: "stacked_bar_chart", Data: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{40.0, 50.0}},
					map[string]any{"name": "S2", "values": []any{30.0, 35.0}},
				},
				"y_max": 60.0,
			}},
			wantField: "data.y_max",
			wantIn:    "85",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Render(tc.req)
			if err == nil {
				t.Fatal("render succeeded; want a validation error naming the axis field")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error %T %v is not a ValidationError", err, err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("field = %q, want %q (%v)", ve.Field, tc.wantField, err)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantIn)
			}
		})
	}
}

func TestAxisBounds_AcceptedAndRendered(t *testing.T) {
	t.Run("waterfall with an authored y_min keeps its value axis", func(t *testing.T) {
		doc, err := Render(&RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": 20.0})})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !hasAxisTick(string(doc.Content), "20") {
			t.Error("a zoomed waterfall must keep its value axis so the truncated totals do not read as true lengths")
		}
	})

	t.Run("default waterfall draws the totals from zero", func(t *testing.T) {
		doc, err := Render(&RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(nil)})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		svg := string(doc.Content)
		for _, tick := range []string{"22", "20"} {
			if hasAxisTick(svg, tick) {
				t.Errorf("default bridge renders a %q tick: its axis should start at zero", tick)
			}
		}
	})

	t.Run("integer bounds are accepted", func(t *testing.T) {
		_, err := Render(&RequestEnvelope{Type: "line_chart", Data: map[string]any{
			"categories": []any{"2024", "2025", "2026"},
			"series":     []any{map[string]any{"name": "S", "values": []any{11.0, 9.0, 7.0}}},
			"y_min":      5, "y_max": 12,
		}})
		if err != nil {
			t.Fatalf("render with integer y_min / y_max: %v", err)
		}
	})

	t.Run("bar with an authored y_min keeps its value axis", func(t *testing.T) {
		doc, err := Render(&RequestEnvelope{Type: "bar_chart", Data: map[string]any{
			"categories": []any{"A", "B", "C"},
			"series":     []any{map[string]any{"name": "S", "values": []any{80.0, 90.0, 95.0}}},
			"y_min":      60.0,
		}})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !hasAxisTick(string(doc.Content), "60") {
			t.Error("a bar chart with a zoomed axis must keep its value axis")
		}
	})
}

func TestAxisNotZeroFinding(t *testing.T) {
	findingsFor := func(t *testing.T, req *RequestEnvelope) []Finding {
		t.Helper()
		out, err := RenderMultiFormatWithFindings(req, "svg")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return out.Findings
	}
	has := func(fs []Finding) *Finding {
		for i := range fs {
			if fs[i].Code == FindingAxisNotZero {
				return &fs[i]
			}
		}
		return nil
	}

	t.Run("waterfall y_min hiding more than half the smallest total", func(t *testing.T) {
		f := has(findingsFor(t, &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": 20.0})}))
		if f == nil {
			t.Fatal("y_min 20 hides 20 of the 24.0 opening total; want chart.axis_not_zero")
		}
		if f.Severity != "info" {
			t.Errorf("severity = %q, want info", f.Severity)
		}
		if f.Field != "data.y_min" {
			t.Errorf("field = %q, want data.y_min", f.Field)
		}
		if !strings.Contains(f.Message, "2023 EBITDA") {
			t.Errorf("message %q should name the truncated bar", f.Message)
		}
	})

	t.Run("waterfall y_min hiding less than half is silent", func(t *testing.T) {
		if f := has(findingsFor(t, &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(map[string]any{"y_min": 10.0})})); f != nil {
			t.Errorf("y_min 10 hides under half of 24.0; got %v", f.Message)
		}
	})

	t.Run("default waterfall is silent", func(t *testing.T) {
		if f := has(findingsFor(t, &RequestEnvelope{Type: "waterfall", Data: ebitdaBridgeData(nil)})); f != nil {
			t.Errorf("no authored y_min; got %v", f.Message)
		}
	})

	t.Run("bar y_min hiding more than half the smallest bar", func(t *testing.T) {
		f := has(findingsFor(t, &RequestEnvelope{Type: "bar_chart", Data: map[string]any{
			"categories": []any{"A", "B", "C"},
			"series":     []any{map[string]any{"name": "S", "values": []any{80.0, 90.0, 95.0}}},
			"y_min":      60.0,
		}}))
		if f == nil {
			t.Fatal("y_min 60 hides 60 of the 80 bar; want chart.axis_not_zero")
		}
		if f.Severity != "info" {
			t.Errorf("severity = %q, want info", f.Severity)
		}
	})
}
