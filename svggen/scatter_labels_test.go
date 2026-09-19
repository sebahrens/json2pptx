package svggen

import (
	"strings"
	"testing"
)

// scatterLabelRequest builds a scatter request of seriesCount x perSeries
// labelled points.
func scatterLabelRequest(seriesCount, perSeries int, showValues bool) *RequestEnvelope {
	series := make([]any, 0, seriesCount)
	for s := 0; s < seriesCount; s++ {
		values := make([]any, perSeries)
		xValues := make([]any, perSeries)
		labels := make([]any, perSeries)
		for i := 0; i < perSeries; i++ {
			values[i] = float64(10 + (i*7+s*3)%80)
			xValues[i] = float64(5 + (i*11+s*5)%90)
			labels[i] = string(rune('A'+s)) + "-" + string(rune('1'+i%9))
		}
		series = append(series, map[string]any{
			"name": "Series " + string(rune('A'+s)), "values": values,
			"x_values": xValues, "labels": labels,
		})
	}
	return &RequestEnvelope{
		Type:   "scatter_chart",
		Data:   map[string]any{"series": series},
		Output: OutputSpec{Width: 900, Height: 540},
		Style:  StyleSpec{ShowValues: showValues},
	}
}

// Six series of nine points labelled every one of the 54, and the collision
// pass could not see across series because it ran per series — so labels from
// different series were laid straight over each other. Above the readable
// count the labels are dropped and the chart says so, once
// (go-slide-creator-daqp).
func TestScatterDropsLabelsPastTheReadableCount(t *testing.T) {
	out, err := RenderMultiFormatWithFindings(scatterLabelRequest(6, 9, false), "svg")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(out.SVG.Bytes())
	if strings.Contains(svg, ">A-1<") {
		t.Error("54 labelled points still drew their labels")
	}

	var dropped *Finding
	for i, f := range out.Findings {
		if f.Code == FindingScatterLabelSkipped {
			if dropped != nil {
				t.Errorf("more than one label finding: %+v", out.Findings)
			}
			dropped = &out.Findings[i]
		}
	}
	if dropped == nil {
		t.Fatalf("labels were dropped with no finding: %+v", out.Findings)
	}
	if !strings.Contains(dropped.Message, "54 point labels dropped") {
		t.Errorf("message = %q, want the count it dropped", dropped.Message)
	}
	if dropped.Severity != "warning" {
		t.Errorf("severity = %q, want warning — dropping every label is not an aside", dropped.Severity)
	}
}

// Under the count the labels are drawn, and the collision set is now shared
// across series.
func TestScatterLabelsEveryPointUnderTheThreshold(t *testing.T) {
	out, err := RenderMultiFormatWithFindings(scatterLabelRequest(2, 6, false), "svg")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(out.SVG.Bytes())
	for _, want := range []string{">A-1<", ">B-1<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("12 labelled points lost %s", want)
		}
	}
	for _, f := range out.Findings {
		if f.Code == FindingScatterLabelSkipped && strings.Contains(f.Message, "dropped") {
			t.Errorf("a 12-point chart dropped its labels: %q", f.Message)
		}
	}
}

// An explicit show_values means the caller has asked for every label and knows
// what that costs.
func TestScatterShowValuesForcesLabelsPastTheThreshold(t *testing.T) {
	out, err := RenderMultiFormatWithFindings(scatterLabelRequest(6, 9, true), "svg")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out.SVG.Bytes()), ">A-1<") {
		t.Error("show_values did not force the labels")
	}
	for _, f := range out.Findings {
		if f.Code == FindingScatterLabelSkipped && strings.Contains(f.Message, "dropped") {
			t.Errorf("show_values still reported a drop: %q", f.Message)
		}
	}
}

// A bubble chart is a scatter with variable point sizes, so it reports the
// same way — it used to emit nothing at all.
func TestBubbleChartReportsDroppedLabels(t *testing.T) {
	req := scatterLabelRequest(4, 9, false)
	req.Type = "bubble_chart"
	series, _ := req.Data["series"].([]any)
	for _, s := range series {
		m, _ := s.(map[string]any)
		values, _ := m["values"].([]any)
		sizes := make([]any, len(values))
		for i := range sizes {
			sizes[i] = float64(5 + i*4)
		}
		m["bubble_values"] = sizes
	}

	out, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	found := false
	for _, f := range out.Findings {
		if f.Code == FindingScatterLabelSkipped && strings.Contains(f.Message, "36 point labels dropped") {
			found = true
		}
	}
	if !found {
		t.Errorf("a 36-bubble chart reported nothing about its labels: %+v", out.Findings)
	}
}
