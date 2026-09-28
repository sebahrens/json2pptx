package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func outlineCell(fill, line string) *jsonschema.GridCellInput {
	shape := &jsonschema.ShapeSpecInput{Geometry: "rect"}
	if fill != "" {
		shape.Fill = json.RawMessage(fill)
	}
	if line != "" {
		shape.Line = json.RawMessage(line)
	}
	return &jsonschema.GridCellInput{Shape: shape}
}

func TestDetectFilledShapeOutlined(t *testing.T) {
	grid := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
		outlineCell(`"lt1"`, `{"color":"dk1","width":0.75}`),            // flagged
		outlineCell(`"accent1"`, `"none"`),                              // no outline
		outlineCell(`"none"`, `{"color":"accent1","width":1}`),          // unfilled ring
		outlineCell(`{"color":"lt2","alpha":15}`, `"dk2"`),              // translucent wash
		outlineCell(`{"color":"dk1","lumMod":4000,"lumOff":96000}`, ``), // no line
		{Grid: &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			outlineCell(`"accent2"`, `"dk1"`), // flagged, nested
		}}}}},
	}}}}
	var got []*patterns.ValidationError
	for _, w := range DetectStructuralSmells(grid, 2) {
		if w.Code == patterns.ErrCodeFilledShapeOutlined {
			got = append(got, w)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d FILLED_SHAPE_OUTLINED findings, want 2: %+v", len(got), got)
	}
	if want := "/slides/2/shape_grid/rows/0/cells/0/shape/line"; got[0].Path != want {
		t.Errorf("path = %q, want %q", got[0].Path, want)
	}
	if want := "/slides/2/shape_grid/rows/0/cells/5/grid/rows/0/cells/0/shape/line"; got[1].Path != want {
		t.Errorf("nested path = %q, want %q", got[1].Path, want)
	}
	if got[0].Fix == nil || got[0].Fix.Kind != "remove_outline" {
		t.Errorf("fix = %+v, want remove_outline", got[0].Fix)
	}
}
