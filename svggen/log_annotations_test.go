package svggen

import (
	"errors"
	"strings"
	"testing"
)

func TestLogBarAnnotationsSurviveRender(t *testing.T) {
	for _, typ := range []string{"bar_chart", "grouped_bar_chart"} {
		req := &RequestEnvelope{Type: typ, Style: StyleSpec{Scale: "log"}, Data: map[string]any{
			"categories": []any{"A", "B", "C"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{10.0, 100.0, 1000.0}}, map[string]any{"name": "Costs", "values": []any{20.0, 80.0, 900.0}}},
			"annotations": []any{map[string]any{"kind": "reference_line", "axis": "y", "value": 100.0, "label": "Target milestone"}, map[string]any{"kind": "callout", "x": 1.0, "y": 100.0, "text": "Customer launch"}},
		}}
		if _, err := DryRender(req); err != nil {
			t.Fatalf("%s dry: %v", typ, err)
		}
		out, err := Render(req)
		if err != nil {
			t.Fatalf("%s render: %v", typ, err)
		}
		for _, label := range []string{"Target milestone", "Customer launch"} {
			if !strings.Contains(string(out.Content), label) {
				t.Errorf("%s lost %s", typ, label)
			}
		}
		for _, ann := range []map[string]any{
			{"kind": "reference_line", "value": 0.0},
			{"kind": "callout", "x": 1.0, "y": -1.0, "text": "Invalid"},
			{"kind": "trendline", "series": "Revenue"},
		} {
			req.Data["annotations"] = []any{ann}
			for _, render := range []func(*RequestEnvelope) error{
				func(r *RequestEnvelope) error { _, e := DryRender(r); return e }, func(r *RequestEnvelope) error { _, e := Render(r); return e },
			} {
				err = render(req)
				var invalid *ValidationError
				if !errors.As(err, &invalid) || !strings.HasPrefix(invalid.Field, "data.annotations[0].") {
					t.Errorf("%s accepted/dropped invalid annotation: %v", typ, err)
				}
			}
		}
	}
}
