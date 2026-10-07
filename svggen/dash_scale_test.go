package svggen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-rtfyz: a 6 / 3pt dash pattern on a 1.5pt stroke reached
// the SVG as stroke-dasharray:1.12 .56 beside stroke-width:2 — dashes shorter
// than the line is wide, so every dashed stroke read as a thin solid one.

var dashStyleRe = regexp.MustCompile(`stroke-width:([0-9.]+);stroke-dasharray:([0-9. ]+)`)

// dashedStrokes returns each dashed stroke of svg as its width and dash
// pattern, in px.
func dashedStrokes(t *testing.T, svg string) [][]float64 {
	t.Helper()
	var out [][]float64
	for _, m := range dashStyleRe.FindAllStringSubmatch(svg, -1) {
		row := []float64{}
		for _, f := range append([]string{m[1]}, strings.Fields(m[2])...) {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				t.Fatalf("bad number %q in %q", f, m[0])
			}
			row = append(row, v)
		}
		out = append(out, row)
	}
	return out
}

// A dash pattern is drawn at the lengths asked for, in points, whatever the
// stroke width and whichever of SetDashes / SetStrokeWidth is called first.
func TestDashPatternIsDrawnInPoints(t *testing.T) {
	const ptToPx = 96.0 / 72.0
	for _, width := range []float64{0.5, 1.5, 4} {
		for _, dashesFirst := range []bool{true, false} {
			b := NewSVGBuilder(400, 200)
			b.SetStrokeColor(MustParseColor("#333333"))
			if dashesFirst {
				b.SetDashes(6, 3).SetStrokeWidth(width)
			} else {
				b.SetStrokeWidth(width).SetDashes(6, 3)
			}
			b.DrawLine(20, 100, 380, 100)
			svg, err := b.RenderToString()
			if err != nil {
				t.Fatal(err)
			}
			strokes := dashedStrokes(t, svg)
			if len(strokes) != 1 || len(strokes[0]) != 3 {
				t.Fatalf("width %.1f dashesFirst=%t: dashed strokes = %v in %s", width, dashesFirst, strokes, svg)
			}
			got := strokes[0]
			for i, want := range []float64{width * ptToPx, 6 * ptToPx, 3 * ptToPx} {
				if math.Abs(got[i]-want) > 0.02*want {
					t.Errorf("width %.1f dashesFirst=%t: stroke %v, want width %.2f dashes %.2f %.2f px", width, dashesFirst, got, width*ptToPx, 6*ptToPx, 3*ptToPx)
					break
				}
			}
		}
	}
}

// Push / Pop restore the dash pattern with the rest of the stroke state, and
// an empty pattern is solid again.
func TestDashPatternFollowsPushPop(t *testing.T) {
	b := NewSVGBuilder(400, 200)
	b.SetStrokeColor(MustParseColor("#333333")).SetStrokeWidth(1).SetDashes(4, 2)
	b.Push()
	b.SetDashes().SetStrokeWidth(2)
	b.DrawLine(20, 150, 380, 150) // solid
	b.Pop()
	b.SetStrokeWidth(2)           // the restored 4 / 2pt pattern at the new width
	b.DrawLine(20, 100, 380, 100) // dashed
	svg, err := b.RenderToString()
	if err != nil {
		t.Fatal(err)
	}
	strokes := dashedStrokes(t, svg)
	if len(strokes) != 1 {
		t.Fatalf("want one dashed stroke, got %v in %s", strokes, svg)
	}
	const dash, gap = 4 * 96.0 / 72.0, 2 * 96.0 / 72.0
	if len(strokes[0]) != 3 || math.Abs(strokes[0][1]-dash) > 0.02*dash || math.Abs(strokes[0][2]-gap) > 0.02*gap {
		t.Errorf("restored pattern drawn as %v, want %.2f %.2f px", strokes[0][1:], dash, gap)
	}
}

func TestScaleStrokeDashes(t *testing.T) {
	in := `style="fill:none;stroke:#000;stroke-width:.5;stroke-dasharray:2 1;stroke-dashoffset:-1" d="M0 0"`
	got := scaleStrokeDashes(in)
	for _, want := range []string{"stroke-dasharray:7.5591 3.7795", "stroke-dashoffset:-3.7795", "stroke-width:.5", `d="M0 0"`} {
		if !strings.Contains(got, want) {
			t.Errorf("scaleStrokeDashes = %s, want it to contain %s", got, want)
		}
	}
}

// Every dashed stroke a chart or diagram draws has dashes longer than the
// stroke is wide: the property the unscaled patterns lost.
func TestDashedStrokesReadAsDashed(t *testing.T) {
	reqs := map[string]*RequestEnvelope{
		"radar target outline": {Type: "radar_chart", Data: map[string]any{
			"categories": []any{"A", "B", "C", "D", "E"},
			"series": []any{
				map[string]any{"name": "Team", "values": []any{80.0, 70.0, 90.0, 60.0, 75.0}},
				map[string]any{"name": "Target", "values": []any{90.0, 90.0, 85.0, 85.0, 85.0}},
			},
		}},
		"waterfall connectors": {Type: "waterfall", Data: map[string]any{"points": []any{
			map[string]any{"label": "Start", "value": 100.0, "type": "total"},
			map[string]any{"label": "Up", "value": 30.0, "type": "increase"},
			map[string]any{"label": "Down", "value": -20.0, "type": "decrease"},
			map[string]any{"label": "End", "value": 110.0, "type": "total"},
		}}},
	}
	for name, req := range reqs {
		req.Output = OutputSpec{Width: 800, Height: 500}
		doc, err := Render(req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		strokes := dashedStrokes(t, string(doc.Content))
		if name == "radar target outline" && len(strokes) == 0 {
			t.Errorf("%s: no dashed stroke drawn", name)
		}
		for _, s := range strokes {
			if s[1] < 2*s[0] {
				t.Errorf("%s: dash %.2fpx on a %.2fpx stroke reads as solid", name, s[1], s[0])
			}
		}
	}
}
