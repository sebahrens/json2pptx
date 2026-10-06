package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ringCellGrid is a one-cell grid holding a contained ring: a full-frame
// segment, a badge and an inner arrow. It is what a circular pattern returns.
func ringCellGrid() *jsonschema.ShapeGridInput {
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{
			Fit: "contain",
			Layers: []jsonschema.LayerInput{
				{Name: "ring", Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1},
					Shape: &ShapeSpecInput{Geometry: "blockArc", Fill: json.RawMessage(`"accent1"`), Line: json.RawMessage(`"none"`), Adjustments: map[string]int64{"adj1": 16200000, "adj2": 0, "adj3": 20000}}},
				{Name: "badge", Frame: jsonschema.LayerFrameInput{X: 0.72, Y: 0.06, W: 0.2, H: 0.2},
					Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"dk2"`), Line: json.RawMessage(`"none"`), Text: json.RawMessage(`{"content":"1","size":12,"color":"lt1","align":"ctr","vertical_align":"ctr"}`)}},
				{Name: "arrow", Frame: jsonschema.LayerFrameInput{X: 0.25, Y: 0.25, W: 0.5, H: 0.5},
					Shape: &ShapeSpecInput{Geometry: "circularArrow", Fill: json.RawMessage(`"accent2"`), Line: json.RawMessage(`"none"`)}},
			},
		}}}},
	}
}

func textGrid(label string) *jsonschema.ShapeGridInput {
	text, _ := json.Marshal(label)
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"lt2"`), Text: text}}}}},
	}
}

// layerFractions returns each layer's rectangle as fractions of the first
// layer's (the full-frame ring), in layer order.
func layerFractions(t *testing.T, cells []shapegrid.ResolvedCell) [][4]float64 {
	t.Helper()
	var layers []shapegrid.ResolvedCell
	for _, c := range cells {
		if c.Layer {
			layers = append(layers, c)
		}
	}
	if len(layers) != 3 {
		t.Fatalf("resolved %d layers, want the ring's 3", len(layers))
	}
	ring := layers[0].Bounds
	if ring.CX != ring.CY {
		t.Errorf("the contained ring is %d x %d EMU, not a square", ring.CX, ring.CY)
	}
	out := make([][4]float64, len(layers))
	for i, l := range layers {
		if l.LayerIdx != i {
			t.Errorf("layer %d resolved out of order (index %d)", i, l.LayerIdx)
		}
		out[i] = [4]float64{
			float64(l.Bounds.X-ring.X) / float64(ring.CX), float64(l.Bounds.Y-ring.Y) / float64(ring.CY),
			float64(l.Bounds.CX) / float64(ring.CX), float64(l.Bounds.CY) / float64(ring.CY),
		}
	}
	return out
}

// The same layered cell resolves the same fractional geometry alone on the
// slide, in a 50% horizontal compose segment, in a vertical segment and in a
// nested grid cell: frames are relative to the square the cell gets there.
func TestLayeredCellKeepsItsGeometryInSplitLayouts(t *testing.T) {
	horizontal, _, err := mergeHorizontal([]*jsonschema.ShapeGridInput{ringCellGrid(), textGrid("Beside the ring")}, []float64{50, 50}, 16)
	if err != nil {
		t.Fatal(err)
	}
	vertical, err := mergeVertical([]*jsonschema.ShapeGridInput{textGrid("Above the ring"), ringCellGrid()}, []float64{30, 70}, 16)
	if err != nil {
		t.Fatal(err)
	}
	nested := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[35, 65]`),
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
			{Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"lt2"`)}},
			{Grid: ringCellGrid()},
		}}},
	}

	want := [][4]float64{{0, 0, 1, 1}, {0.72, 0.06, 0.2, 0.2}, {0.25, 0.25, 0.5, 0.5}}
	sides := map[string]int64{}
	for _, tc := range []struct {
		name string
		grid *jsonschema.ShapeGridInput
	}{
		{"alone", ringCellGrid()},
		{"horizontal segment 50%", horizontal},
		{"vertical segment 70%", vertical},
		{"nested grid cell", nested},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := resolveShapeGrid(tc.grid, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
			if err != nil || res == nil {
				t.Fatalf("resolveShapeGrid: %v", err)
			}
			got := layerFractions(t, res.Cells)
			for i := range want {
				for k := range want[i] {
					if math.Abs(got[i][k]-want[i][k]) > 1e-4 {
						t.Errorf("layer %d fractions = %v, want %v", i, got[i], want[i])
						break
					}
				}
			}
			// One <p:sp> per layer reaches the slide, in layer order.
			var order []string
			for _, xml := range res.Shapes {
				for _, geom := range []string{"blockArc", "circularArrow"} {
					if strings.Contains(string(xml), `prst="`+geom+`"`) {
						order = append(order, geom)
					}
				}
				if strings.Contains(string(xml), `prst="ellipse"`) {
					order = append(order, "ellipse")
				}
			}
			if strings.Join(order, ",") != "blockArc,ellipse,circularArrow" {
				t.Errorf("layer shapes written as %v, want blockArc, ellipse, circularArrow", order)
			}
			for _, c := range res.Cells {
				if c.Layer && c.LayerIdx == 0 {
					sides[tc.name] = c.Bounds.CX
				}
			}
		})
	}
	// The contexts really differ: the ring is smaller in a split than alone.
	if sides["horizontal segment 50%"] <= 0 || sides["vertical segment 70%"] >= sides["alone"] || sides["nested grid cell"] <= 0 {
		t.Errorf("ring sides by context = %v; want a ring in each and a smaller one in the vertical split", sides)
	}
}

// What the writer emits for a layered cell: connectors first (they sit behind
// the cells they join), then each cell's own shape followed at once by its
// layers in order, then accent bars.
func TestLayerShapesFollowTheirCellInTheShapeList(t *testing.T) {
	filled := func(fill string) *ShapeSpecInput {
		return &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"` + fill + `"`), Line: json.RawMessage(`"none"`)}
	}
	layer := func(geom string) jsonschema.LayerInput {
		return jsonschema.LayerInput{Frame: jsonschema.LayerFrameInput{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}, Shape: &ShapeSpecInput{Geometry: geom, Fill: json.RawMessage(`"accent2"`)}}
	}
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`2`),
		ColGap:  24,
		Rows: []GridRowInput{{
			Connector: &jsonschema.ConnectorSpecInput{Style: "arrow"},
			Cells: []*GridCellInput{
				{Shape: filled("accent1"), Layers: []jsonschema.LayerInput{layer("blockArc"), layer("ellipse")}, AccentBar: &jsonschema.AccentBarInput{Position: "top"}},
				{Shape: filled("lt2"), Layers: []jsonschema.LayerInput{layer("circularArrow")}},
			},
		}},
	}
	res, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid: %v", err)
	}
	var got []string
	for _, xml := range res.Shapes {
		s := string(xml)
		switch {
		case strings.Contains(s, "<p:cxnSp>"):
			got = append(got, "connector")
		case strings.Contains(s, `prst="blockArc"`):
			got = append(got, "blockArc")
		case strings.Contains(s, `prst="ellipse"`):
			got = append(got, "ellipse")
		case strings.Contains(s, `prst="circularArrow"`):
			got = append(got, "circularArrow")
		case strings.Contains(s, `<a:schemeClr val="accent1"/>`) && strings.Contains(s, `prst="rect"`) && len(got) == 1:
			got = append(got, "cell-1")
		case strings.Contains(s, `<a:schemeClr val="lt2"`):
			got = append(got, "cell-2")
		default:
			got = append(got, "bar")
		}
	}
	if want := "connector,cell-1,blockArc,ellipse,cell-2,circularArrow,bar"; strings.Join(got, ",") != want {
		t.Fatalf("shape order = %v, want %s", got, want)
	}
	// Each written shape knows the authored element behind it.
	var paths []string
	for _, src := range res.ShapeSources {
		paths = append(paths, strings.TrimPrefix(src.Path, "/slides/0/shape_grid"))
	}
	wantPaths := []string{
		"/rows/0/connector",
		"/rows/0/cells/0/shape/text", "/rows/0/cells/0/layers/0/shape/text", "/rows/0/cells/0/layers/1/shape/text",
		"/rows/0/cells/1/shape/text", "/rows/0/cells/1/layers/0/shape/text",
		"/rows/0/cells/0/accent_bar",
	}
	if strings.Join(paths, " ") != strings.Join(wantPaths, " ") {
		t.Errorf("shape sources = %v, want %v", paths, wantPaths)
	}
}

// A grouped cell's layers are written inside its group, after its own shape.
func TestGroupedCellGroupsItsLayers(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{Cells: []*GridCellInput{{
			Group: true,
			Shape: &ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"lt2"`)},
			Layers: []jsonschema.LayerInput{
				{Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1}, Shape: &ShapeSpecInput{Geometry: "blockArc", Fill: json.RawMessage(`"accent1"`)}},
				{Frame: jsonschema.LayerFrameInput{X: 0.4, Y: 0.4, W: 0.2, H: 0.2}, Shape: &ShapeSpecInput{Geometry: "ellipse", Fill: json.RawMessage(`"dk2"`)}},
			},
		}}}},
	}
	res, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid: %v", err)
	}
	if len(res.Shapes) != 1 {
		t.Fatalf("wrote %d top-level shapes, want the one group", len(res.Shapes))
	}
	group := string(res.Shapes[0])
	if !strings.Contains(group, "<p:grpSp>") || strings.Count(group, "<p:sp>") != 3 {
		t.Fatalf("the group does not hold the cell's shape and its 2 layers:\n%s", group)
	}
	rect, arc, dot := strings.Index(group, `prst="rect"`), strings.Index(group, `prst="blockArc"`), strings.Index(group, `prst="ellipse"`)
	if rect < 0 || rect >= arc || arc >= dot {
		t.Errorf("group children are not the cell's shape then its layers in order (rect %d, blockArc %d, ellipse %d)", rect, arc, dot)
	}
}
