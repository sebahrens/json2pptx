package svggen

import (
	"encoding/json"
	"strings"
	"testing"
)

func consultingChartFixture(typ string) *RequestEnvelope {
	fixtures := map[string]string{
		"butterfly_chart": `{"categories":["North","South","West"],"left":{"name":"Before","values":[20,40,30]},"right":{"name":"After","values":[30,45,40]}}`,
		"mekko_chart":     `{"segments":[{"name":"Enterprise","width":60,"parts":[{"name":"Product","value":60},{"name":"Service","value":40}]},{"name":"SMB","width":40,"parts":[{"name":"Product","value":40},{"name":"Service","value":60}]}]}`,
		"combo_chart":     `{"categories":["Q1","Q2","Q3","Q4"],"bars":[{"name":"Revenue","values":[20,40,30,45]}],"lines":[{"name":"Margin","values":[10,20,15,22]}],"secondary_axis":true,"bar_unit":"$M","line_unit":"%"}`,
	}
	var data map[string]any
	_ = json.Unmarshal([]byte(fixtures[typ]), &data)
	return &RequestEnvelope{Type: typ, Data: data, Output: OutputSpec{Width: 800, Height: 500}}
}
func TestConsultingChartsRenderAllLabelsWithinFrame(t *testing.T) {
	for _, typ := range []string{"butterfly_chart", "mekko_chart", "combo_chart"} {
		t.Run(typ, func(t *testing.T) {
			d := &ConsultingChartDiagram{NewBaseDiagram(typ)}
			req := consultingChartFixture(typ)
			if err := d.Validate(req); err != nil {
				t.Fatal(err)
			}
			b, doc, err := d.RenderWithBuilder(req)
			if err != nil {
				t.Fatal(err)
			}
			if doc == nil || len(b.textBoxes) < 5 {
				t.Fatal("missing chart labels")
			}
			for _, box := range b.textBoxes {
				if box.rect.X < 0 || box.rect.Y < 0 || box.rect.X+box.rect.W > b.Width()+.1 || box.rect.Y+box.rect.H > b.Height()+.1 {
					t.Fatalf("label outside frame %+v", box)
				}
			}
			for i, a := range b.textBoxes {
				for _, other := range b.textBoxes[i+1:] {
					if a.rect.Intersects(other.rect) {
						t.Errorf("labels overlap: %q and %q", a.text, other.text)
					}
				}
			}
			req.Output.Width = 150
			req.Output.Height = 100
			if _, _, err := d.RenderWithBuilder(req); err == nil || !strings.Contains(err.Error(), "fewer") {
				t.Fatalf("missing readable refusal: %v", err)
			}
		})
	}
}
func TestConsultingChartsRejectUnknownAndInvalidData(t *testing.T) {
	for _, typ := range []string{"butterfly_chart", "mekko_chart", "combo_chart"} {
		t.Run(typ, func(t *testing.T) {
			d := &ConsultingChartDiagram{NewBaseDiagram(typ)}
			req := consultingChartFixture(typ)
			req.Data["invented"] = true
			if err := d.Validate(req); err == nil {
				t.Fatal("unknown field accepted")
			}
		})
	}
	req := consultingChartFixture("butterfly_chart")
	req.Data["left"].(map[string]any)["values"] = []any{1.0, -1.0, 2.0}
	if err := (&ConsultingChartDiagram{NewBaseDiagram("butterfly_chart")}).Validate(req); err == nil {
		t.Fatal("negative mirrored magnitude accepted")
	}
	req = consultingChartFixture("mekko_chart")
	req.Data["segments"].([]any)[0].(map[string]any)["width"] = 0.0
	if err := (&ConsultingChartDiagram{NewBaseDiagram("mekko_chart")}).Validate(req); err == nil {
		t.Fatal("zero width accepted")
	}
	req = consultingChartFixture("combo_chart")
	req.Data["secondary_axis"] = false
	if err := (&ConsultingChartDiagram{NewBaseDiagram("combo_chart")}).Validate(req); err == nil {
		t.Fatal("different units on one axis accepted")
	}
}
