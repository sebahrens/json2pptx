package svggen

import (
	"testing"
)

func TestMatrixStakeholderStatusLegendAndSize(t *testing.T) {
	req := &RequestEnvelope{Type: "matrix_2x2", Output: OutputSpec{Width: 800, Height: 600}, Data: map[string]any{"quadrant_labels": []any{"", "", "", ""}, "points": []any{map[string]any{"label": "Sponsor", "x": 75.0, "y": 75.0, "size": 20.0, "status": "supporter"}, map[string]any{"label": "Finance", "x": 30.0, "y": 35.0, "size": 10.0, "status": "neutral"}}}}
	d := &Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}
	if err := d.Validate(req); err != nil {
		t.Fatal(err)
	}
	data, _, err := parseMatrix2x2Data(req)
	if err != nil {
		t.Fatal(err)
	}
	if data.Points[0].Size != 20 || data.Points[0].Color == nil || *data.Points[0].Color != stakeholderStatusColors["supporter"] {
		t.Fatalf("lost status or size %+v", data.Points[0])
	}
	b, _, err := d.RenderWithBuilder(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Sponsor", "Finance", "supporter", "neutral"} {
		if _, found := drawnTextBoxFor(b, label); !found {
			t.Fatalf("label %q missing", label)
		}
	}
	if _, found := drawnTextBoxFor(b, "blocker"); found {
		t.Fatal("unused status legend item")
	}
	req.Data["points"].([]any)[0].(map[string]any)["status"] = "maybe"
	if err := d.Validate(req); err == nil {
		t.Fatal("invalid status accepted")
	}
}
