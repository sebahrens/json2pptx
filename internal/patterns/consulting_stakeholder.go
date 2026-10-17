package patterns

import "github.com/sebahrens/json2pptx/internal/types"

func init() {
	point := visualObject(map[string]*Schema{"label": visualString(70), "x": NumberSchema(0, 100), "y": NumberSchema(0, 100), "size": NumberSchema(1, 40), "status": EnumSchema("supporter", "neutral", "blocker")}, "label", "x", "y", "status")
	point.raw.Properties["x"].raw.Minimum = float64Ptr(0)
	point.raw.Properties["y"].raw.Minimum = float64Ptr(0)
	Default().Register(&consultingPattern{name: "stakeholder-map", description: "Stakeholders positioned by influence and interest, sized by explicit diameter and colored by supporter/neutral/blocker stance", valuesSchema: visualObject(map[string]*Schema{"points": visualArray(point, 2, 12), "x_axis_label": visualString(60), "y_axis_label": visualString(60)}, "points", "x_axis_label", "y_axis_label"), example: map[string]any{"x_axis_label": "Interest", "y_axis_label": "Influence", "points": []any{map[string]any{"label": "Sponsor", "x": 80.0, "y": 85.0, "size": 20.0, "status": "supporter"}, map[string]any{"label": "Operations", "x": 35.0, "y": 60.0, "size": 12.0, "status": "neutral"}, map[string]any{"label": "Controls", "x": 70.0, "y": 25.0, "size": 16.0, "status": "blocker"}}}, draw: func(c *consultingCanvas, v map[string]any) error {
		data := map[string]any{}
		for key, value := range v {
			data[key] = value
		}
		data["quadrant_labels"] = []any{"", "", "", ""}
		c.diagram = &types.DiagramSpec{Type: "matrix_2x2", Data: data}
		return nil
	}})
}
