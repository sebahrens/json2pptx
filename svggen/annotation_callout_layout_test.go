package svggen

import (
	"errors"
	"fmt"
	"testing"
)

func annotatedRevenueRequest(chartType string) *RequestEnvelope {
	return &RequestEnvelope{Type: chartType, Output: OutputSpec{Width: 800, Height: 600}, Data: map[string]any{
		"categories":  []any{"Q1", "Q2", "Q3", "Q4"},
		"series":      []any{map[string]any{"name": "Revenue", "values": []any{120.0, 145.0, 160.0, 195.0}}, map[string]any{"name": "Plan", "values": []any{130.0, 140.0, 170.0, 180.0}}},
		"annotations": []any{map[string]any{"kind": "reference_line", "axis": "y", "value": 180.0, "label": "Target: $180M", "style": "dashed"}, map[string]any{"kind": "trendline", "series": "Revenue", "method": "linear"}, map[string]any{"kind": "callout", "x": 2.0, "y": 160.0, "text": "Supply constrained"}},
		"data_labels": map[string]any{"format": "$%.0fM", "show_on": "all"},
	}}
}

func TestGroupedCalloutClearsEveryValueLabel(t *testing.T) {
	for _, chartType := range []string{"bar_chart", "grouped_bar_chart"} {
		for _, calloutFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/calloutFirst%t", chartType, calloutFirst), func(t *testing.T) {
				req := annotatedRevenueRequest(chartType)
				if calloutFirst {
					annotations := req.Data["annotations"].([]any)
					req.Data["annotations"] = []any{annotations[2], annotations[0], annotations[1]}
				}
				var b *SVGBuilder
				var err error
				if chartType == "bar_chart" {
					b, _, err = (&BarChartDiagram{NewBaseDiagram(chartType)}).RenderWithBuilder(req)
				} else {
					b, _, err = (&GroupedBarChartDiagram{NewBaseDiagram(chartType)}).RenderWithBuilder(req)
				}
				if err != nil {
					t.Fatal(err)
				}
				callout, ok := drawnTextBoxFor(b, "Supply constrained")
				if !ok {
					t.Fatal("callout was not drawn")
				}
				for _, value := range []int{120, 145, 160, 195, 130, 140, 170, 180} {
					text := fmt.Sprintf("$%dM", value)
					box, found := drawnTextBoxFor(b, text)
					if !found {
						t.Fatalf("value label %s was dropped", text)
					}
					if callout.Intersects(box) {
						t.Errorf("callout %+v overlaps %s %+v", callout, text, box)
					}
				}
				for _, other := range b.textBoxes {
					if other.text != "Supply constrained" && callout.Intersects(other.rect) {
						t.Errorf("callout overlaps %q", other.text)
					}
				}
				if callout.X < 0 || callout.Y < 0 || callout.X+callout.W > b.Width() || callout.Y+callout.H > b.Height() {
					t.Fatalf("callout outside canvas: %+v", callout)
				}
			})
		}
	}
}

func TestCalloutPlacementAtPlotEdgesAndOtherCallouts(t *testing.T) {
	b := NewSVGBuilder(500, 350)
	plot := Rect{X: 40, Y: 40, W: 420, H: 270}
	xs := NewLinearScale(0, 100)
	xs.SetRangeLinear(0, plot.W)
	ys := NewLinearScale(0, 100)
	ys.SetRangeLinear(plot.H, 0)
	for i, point := range []Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 0, Y: 100}, {X: 100, Y: 100}, {X: 50, Y: 50}, {X: 50, Y: 50}} {
		ann := Annotation{Kind: AnnotationCallout, X: point.X, Y: point.Y, Text: fmt.Sprintf("Callout %d", i)}
		if err := drawCallout(b, ann, plot, xs, ys); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.textBoxes) != 6 {
		t.Fatalf("drew %d of six callouts", len(b.textBoxes))
	}
	for i, box := range b.textBoxes {
		if box.rect.X < plot.X || box.rect.Y < plot.Y || box.rect.X+box.rect.W > plot.X+plot.W || box.rect.Y+box.rect.H > plot.Y+plot.H {
			t.Errorf("outside plot: %+v", box)
		}
		for _, other := range b.textBoxes[i+1:] {
			if box.rect.Intersects(other.rect) {
				t.Errorf("callouts overlap: %+v %+v", box, other)
			}
		}
	}
}

func TestCalloutPlacementRefusesImpossibleGeometry(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(fmt.Sprint(occupied), func(t *testing.T) {
			b := NewSVGBuilder(200, 150)
			plot := Rect{X: 20, Y: 20, W: 160, H: 100}
			text := "Short callout"
			if occupied {
				b.textBoxes = append(b.textBoxes, drawnTextBox{text: "Existing labels", rect: plot})
			} else {
				text = "A callout that is far too wide to fit inside this small plot without shrinking or dropping its words"
			}
			xs := NewLinearScale(0, 100)
			xs.SetRangeLinear(0, plot.W)
			ys := NewLinearScale(0, 100)
			ys.SetRangeLinear(plot.H, 0)
			err := DrawAnnotations(b, []Annotation{{Kind: AnnotationCallout, X: 50, Y: 50, Text: text}}, plot, xs, ys, ChartData{}, nil)
			var refusal *ValidationError
			if !errors.As(err, &refusal) || refusal.Code != FindingDiagramRegionOverflow {
				t.Fatalf("want geometry refusal, got %v", err)
			}
			for _, box := range b.textBoxes {
				if box.text == text {
					t.Fatal("impossible callout was drawn")
				}
			}
		})
	}
}

func TestCalloutLeaderClearsExistingValueText(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	b.SetFontSize(12)
	b.DrawText("$170M", 210, 175, TextAlignLeft, TextBaselineTop)
	existing := b.textBoxes[0].rect
	plot := Rect{X: 20, Y: 20, W: 360, H: 260}
	xs := NewLinearScale(0, 360)
	xs.SetRangeLinear(0, 360)
	ys := NewLinearScale(0, 260)
	ys.SetRangeLinear(260, 0)
	target := Point{X: 200, Y: 200}
	ann := Annotation{Kind: AnnotationCallout, X: 180, Y: 80, Text: "Supply constrained"}
	if err := drawCallout(b, ann, plot, xs, ys); err != nil {
		t.Fatal(err)
	}
	callout, ok := drawnTextBoxFor(b, ann.Text)
	if !ok {
		t.Fatal("callout was dropped")
	}
	if callout.Intersects(existing) {
		t.Fatal("callout text overlaps the value")
	}
	if calloutLeaderCrossesText(calloutLeaderStart(callout, target), target, existing) {
		t.Fatal("callout leader runs through the value")
	}
}
