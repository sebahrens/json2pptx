package svggen

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestGroupedBarCloseValueLabels(t *testing.T) {
	for _, values := range [][]float64{{3.3, 3.2}, {-3.3, -3.2}, {0, 0}, {3.3, 3.2, 3.1}, {-0.1, 0, 0.1}} {
		t.Run(fmt.Sprint(values), func(t *testing.T) {
			b := NewSVGBuilder(350, 260)
			cfg := DefaultBarChartConfig(350, 260)
			cfg.ShowValues = true
			cfg.ShowLegend = false
			cfg.ValueFormat = "%.1f"
			data := ChartData{Categories: []string{"A", "B", "C", "D", "E", "F"}}
			for si, v := range values {
				data.Series = append(data.Series, ChartSeries{Name: fmt.Sprint(si), Values: []float64{v, v, v, v, v, v}})
			}
			if err := NewBarChart(b, cfg).Draw(data); err != nil {
				t.Fatal(err)
			}
			for _, finding := range b.textOverlapFindings() {
				t.Error(finding.Message)
			}
			labels := 0
			for _, tb := range b.textBoxes {
				if strings.Contains(tb.text, ".") {
					labels++
				}
			}
			if labels != len(values)*6 {
				t.Fatalf("got %d data labels, want %d", labels, len(values)*6)
			}
			for i, a := range b.textBoxes {
				if !strings.Contains(a.text, ".") {
					continue
				}
				for _, z := range b.textBoxes[i+1:] {
					if !strings.Contains(z.text, ".") {
						continue
					}
					if a.rect.X < z.rect.X+z.rect.W && a.rect.X+a.rect.W > z.rect.X && a.rect.Y < z.rect.Y+z.rect.H && a.rect.Y+a.rect.H > z.rect.Y {
						t.Errorf("labels %q %+v and %q %+v overlap", a.text, a.rect, z.text, z.rect)
					}
				}
			}
		})
	}
}

func TestGroupedBarHighlightedLabelsBold(t *testing.T) {
	for _, orientation := range []string{"vertical", "horizontal"} {
		for _, highlight := range []string{"North", "A"} {
			t.Run(orientation+highlight, func(t *testing.T) {
				req := highlightReq([]any{"A", "B"}, []any{3.3, 0.7}, map[string]any{"highlight": []any{highlight}, "orientation": orientation}, true)
				req.Type = "grouped_bar_chart"
				req.Data["series"] = []any{map[string]any{"name": "North", "values": []any{3.3, 0.7}}, map[string]any{"name": "South", "values": []any{3.2, 0.6}}}
				doc, err := Render(req)
				if err != nil {
					t.Fatal(err)
				}
				boldValue := regexp.MustCompile(`<text[^>]*font-weight:700[^>]*><tspan[^>]*>3\.3</tspan></text>`)
				if !boldValue.Match(doc.Content) {
					t.Fatal("highlighted value 3.3 must be bold")
				}
			})
		}
	}
}

// The staggering pass may not trade one collision for clipped values or an
// intersection with the title/category labels. Refuse when every label cannot
// fit at the readable size, in either orientation.
func TestGroupedBarLabelGeometryRefusesCrowdedFrames(t *testing.T) {
	for _, n := range []int{6, 50} {
		for _, v := range []float64{-3.3, 0, 3.3} {
			for _, horizontal := range []bool{false, true} {
				t.Run(fmt.Sprintf("series%d/value%g/horizontal%t", n, v, horizontal), func(t *testing.T) {
					b := NewSVGBuilder(350, 200)
					cfg := DefaultBarChartConfig(350, 200)
					cfg.ShowValues = true
					cfg.ShowLegend = false
					cfg.Horizontal = horizontal
					cfg.ValueFormat = "%.1f"
					data := ChartData{Title: "Every value remains readable", Categories: []string{"A", "B", "C", "D", "E", "F"}}
					for si := 0; si < n; si++ {
						data.Series = append(data.Series, ChartSeries{Name: fmt.Sprint(si), Values: []float64{v, v, v, v, v, v}})
					}
					err := NewBarChart(b, cfg).Draw(data)
					if err == nil {
						// Some distributions fit: if accepted, require every label inside
						// the canvas and require no overlap with any other rendered text.
						if got := len(b.textBoxes); got < n*6 {
							t.Fatalf("only %d text boxes for %d labels", got, n*6)
						}
						for _, box := range b.textBoxes {
							if box.rect.X < -0.1 || box.rect.Y < -0.1 || box.rect.X+box.rect.W > 350.1 || box.rect.Y+box.rect.H > 200.1 {
								t.Fatalf("accepted out-of-bounds text %+v", box)
							}
						}
						if overlaps := b.textOverlapFindings(); len(overlaps) > 0 {
							t.Fatalf("accepted overlap: %+v", overlaps)
						}
						return
					}
					var refusal *ValidationError
					if !errors.As(err, &refusal) || refusal.Code != FindingDiagramRegionOverflow || !strings.Contains(err.Error(), "fewer series") || !strings.Contains(err.Error(), "increase its width and height") {
						t.Fatalf("not actionable geometry refusal: %v", err)
					}
					found := false
					for _, finding := range b.Findings() {
						if finding.Code == FindingDiagramRegionOverflow && finding.Severity == "refuse" {
							found = true
						}
					}
					if !found {
						t.Fatal("missing blocking geometry finding")
					}
				})
			}
		}
	}
}

func TestGroupedBarLabelGeometryDoesNotLeakBetweenDraws(t *testing.T) {
	b := NewSVGBuilder(800, 500)
	cfg := DefaultBarChartConfig(800, 500)
	cfg.ShowValues = true
	cfg.ShowLegend = false
	chart := NewBarChart(b, cfg)
	data := ChartData{Categories: []string{"A", "B"}, Series: []ChartSeries{{Name: "One", Values: []float64{3.3, 2.2}}, {Name: "Two", Values: []float64{3.2, 2.1}}}}
	if err := chart.Draw(data); err != nil {
		t.Fatal(err)
	}
	if err := chart.Draw(data); err != nil {
		t.Fatalf("previous render text caused refusal: %v", err)
	}
}

func TestGroupedBarCategoryHighlightPreservesSeriesPalette(t *testing.T) {
	req := highlightReq([]any{"A", "C"}, nil, nil, true)
	req.Type = "grouped_bar_chart"
	req.Data["series"] = []any{map[string]any{"name": "Before", "values": []any{3.3, 0.7}}, map[string]any{"name": "After", "values": []any{3.2, 0.6}}}
	baseline := barFills(t, req)
	req.Data["highlight"] = []any{"C"}
	highlighted := barFills(t, req)
	if !reflect.DeepEqual(highlighted, baseline) {
		t.Fatalf("category highlight changed series colours: %v -> %v", baseline, highlighted)
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"0.7", "0.6"} {
		re := regexp.MustCompile(`<text[^>]*font-weight:700[^>]*><tspan[^>]*>` + regexp.QuoteMeta(value) + `</tspan></text>`)
		if !re.Match(doc.Content) {
			t.Errorf("highlighted category value %s is not bold", value)
		}
	}
}
