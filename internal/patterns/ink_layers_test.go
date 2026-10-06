package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// The readable-ink pass reaches a cell's layers: a badge layer with light text
// on a mid-tone accent is fixed exactly like a cell shape with the same fill
// and text (go-slide-creator-x1fjb).
func TestApplyReadableInkReachesLayers(t *testing.T) {
	const fill = `"accent1"`
	const text = `{"content":"1","size":12,"color":"lt1"}`
	for tmpl, rgb := range midToneAccents {
		ctx := ctxWithAccent1(rgb)
		shape := func() *jsonschema.ShapeSpecInput {
			return &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(fill), Text: json.RawMessage(text)}
		}
		asCell, asLayer, nested := shape(), shape(), shape()
		grid := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			{Shape: asCell},
			{Layers: []jsonschema.LayerInput{
				{Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: &jsonschema.ShapeSpecInput{Geometry: "blockArc", Fill: json.RawMessage(`"lt2"`)}},
				{Frame: jsonschema.LayerFrameInput{X: 0.4, Y: 0.4, W: 0.2, H: 0.2}, Shape: asLayer},
			}},
			{Grid: &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
				{Layers: []jsonschema.LayerInput{{Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: nested}}},
			}}}}},
		}}}}
		ApplyReadableInk(ctx, grid)
		if string(asCell.Fill) == fill && string(asCell.Text) == text {
			t.Fatalf("%s: the cell shape was not fixed; the fixture no longer fails contrast", tmpl)
		}
		for name, s := range map[string]*jsonschema.ShapeSpecInput{"layer": asLayer, "nested layer": nested} {
			if string(s.Fill) != string(asCell.Fill) || string(s.Text) != string(asCell.Text) {
				t.Errorf("%s: %s = fill %s text %s, want the cell shape's fix: fill %s text %s", tmpl, name, s.Fill, s.Text, asCell.Fill, asCell.Text)
			}
		}
	}
}
