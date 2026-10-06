package shapegrid

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// layeredGrid is a grid of one row whose cells are given; the bounds are a
// wide 800 x 200pt rectangle so a contained square is narrower than its cell.
func layeredGrid(columns []float64, cells ...Cell) *Grid {
	return &Grid{
		Bounds:  pptx.RectEmu{X: PtToEMU(100), Y: PtToEMU(50), CX: PtToEMU(800), CY: PtToEMU(200)},
		Columns: columns,
		ColGap:  0.01,
		RowGap:  0.01,
		Rows:    []Row{{Cells: cells}},
	}
}

func arcLayer(name string, x, y, w, h float64) Layer {
	return Layer{Name: name, Frame: LayerFrame{X: x, Y: y, W: w, H: h}, Shape: &ShapeSpec{Geometry: "blockArc", Fill: json.RawMessage(`"accent1"`)}}
}

func mustResolve(t *testing.T, grid *Grid) *ResolveResult {
	t.Helper()
	if err := Validate(grid); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	res, err := Resolve(grid, newAlloc(100))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return res
}

// A layer's frame is a fraction of the rectangle the cell's own shape gets.
func TestResolveLayerFramesMapIntoTheCell(t *testing.T) {
	grid := layeredGrid([]float64{100}, Cell{
		Shape: &ShapeSpec{Geometry: "rect"},
		Layers: []Layer{
			arcLayer("full", 0, 0, 1, 1),
			arcLayer("quarter", 0.5, 0.5, 0.5, 0.5),
			arcLayer("inner", 0.25, 0.1, 0.5, 0.2),
		},
	})
	res := mustResolve(t, grid)
	if len(res.Cells) != 4 {
		t.Fatalf("resolved %d entries, want the cell and its 3 layers", len(res.Cells))
	}
	base := res.Cells[0]
	if base.Layer || base.PathSuffix() != "" {
		t.Errorf("the cell's own entry is marked as a layer: %+v", base)
	}
	want := []pptx.RectEmu{
		base.Bounds,
		{X: base.Bounds.X + base.Bounds.CX/2, Y: base.Bounds.Y + base.Bounds.CY/2, CX: base.Bounds.CX / 2, CY: base.Bounds.CY / 2},
		{X: base.Bounds.X + base.Bounds.CX/4, Y: base.Bounds.Y + base.Bounds.CY/10, CX: base.Bounds.CX / 2, CY: base.Bounds.CY / 5},
	}
	for i, w := range want {
		got := res.Cells[i+1]
		if !got.Layer || got.LayerIdx != i || got.Kind != CellKindShape {
			t.Errorf("entry %d = %+v, want layer %d as a shape", i+1, got, i)
		}
		if absEMU(got.Bounds.X-w.X) > 1 || absEMU(got.Bounds.Y-w.Y) > 1 || absEMU(got.Bounds.CX-w.CX) > 1 || absEMU(got.Bounds.CY-w.CY) > 1 {
			t.Errorf("layer %d bounds = %+v, want %+v", i, got.Bounds, w)
		}
		if got.CellBounds != base.CellBounds || got.RowIdx != base.RowIdx || got.ColIdx != base.ColIdx {
			t.Errorf("layer %d does not share its cell's rectangle and position: %+v", i, got)
		}
	}
	if res.Cells[2].LayerName != "quarter" || res.Cells[2].PathSuffix() != "/layers/1" {
		t.Errorf("layer 1 = name %q suffix %q, want quarter and /layers/1", res.Cells[2].LayerName, res.Cells[2].PathSuffix())
	}
}

func absEMU(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// With fit "contain" the frame is relative to the centred square, not to the
// wide cell: frame (0,0,1,1) is the square a circle actually gets.
func TestResolveLayerFramesFollowFitContain(t *testing.T) {
	for _, tc := range []struct {
		name string
		cell Cell
	}{
		{"shape and layers", Cell{Fit: FitContain, Shape: &ShapeSpec{Geometry: "ellipse"}, Layers: []Layer{arcLayer("ring", 0, 0, 1, 1)}}},
		{"layers only", Cell{Fit: FitContain, Layers: []Layer{arcLayer("ring", 0, 0, 1, 1)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := mustResolve(t, layeredGrid([]float64{100}, tc.cell))
			ring := res.Cells[len(res.Cells)-1]
			if !ring.Layer {
				t.Fatalf("last entry is not the layer: %+v", ring)
			}
			side := PtToEMU(200)
			if ring.Bounds.CX != side || ring.Bounds.CY != side {
				t.Fatalf("layer bounds = %+v, want the %d EMU square", ring.Bounds, side)
			}
			if wantX := PtToEMU(100) + (PtToEMU(800)-side)/2; ring.Bounds.X != wantX {
				t.Errorf("layer X = %d, want the square centred at %d", ring.Bounds.X, wantX)
			}
			if ring.CellBounds.CX != PtToEMU(800) {
				t.Errorf("CellBounds = %+v, want the whole cell", ring.CellBounds)
			}
		})
	}
}

// Two frames that share an edge share it in EMU: segments of a ring meet.
func TestLayerFrameEdgesMeet(t *testing.T) {
	fitted := pptx.RectEmu{X: 1234567, Y: 7654321, CX: 3333333, CY: 2222221}
	a := LayerFrame{X: 0, Y: 0, W: 1.0 / 3, H: 1}.Rect(fitted)
	b := LayerFrame{X: 1.0 / 3, Y: 0, W: 2.0 / 3, H: 1}.Rect(fitted)
	if a.X+a.CX != b.X {
		t.Errorf("frames do not meet: first ends at %d, second starts at %d", a.X+a.CX, b.X)
	}
	if b.X+b.CX != fitted.X+fitted.CX {
		t.Errorf("second frame ends at %d, want the cell edge %d", b.X+b.CX, fitted.X+fitted.CX)
	}
}

// Layers follow their cell in input order, before the next cell: that order
// is the z-order the writer keeps.
func TestResolveLayersKeepInputOrder(t *testing.T) {
	grid := layeredGrid([]float64{50, 50},
		Cell{Shape: &ShapeSpec{Geometry: "rect"}, Layers: []Layer{arcLayer("a", 0, 0, 1, 1), arcLayer("b", 0, 0, 0.5, 0.5), arcLayer("c", 0.5, 0.5, 0.5, 0.5)}},
		Cell{Shape: &ShapeSpec{Geometry: "rect"}},
	)
	res := mustResolve(t, grid)
	var got []string
	for _, c := range res.Cells {
		switch {
		case c.Layer:
			got = append(got, c.LayerName)
		default:
			got = append(got, "cell")
		}
	}
	if want := []string{"cell", "a", "b", "c", "cell"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := 1; i < len(res.Cells); i++ {
		if res.Cells[i].ID <= res.Cells[i-1].ID {
			t.Errorf("shape ids are not ascending in draw order: %d then %d", res.Cells[i-1].ID, res.Cells[i].ID)
		}
	}
}

// A cell may hold only layers: it draws no shape of its own, keeps its
// footprint, and the cells after it stay where they were placed.
func TestResolveLayersOnlyCell(t *testing.T) {
	grid := layeredGrid([]float64{50, 50},
		Cell{Layers: []Layer{arcLayer("a", 0, 0, 1, 1), arcLayer("b", 0.4, 0.4, 0.2, 0.2)}},
		Cell{Shape: &ShapeSpec{Geometry: "rect"}},
	)
	res := mustResolve(t, grid)
	if len(res.Cells) != 3 {
		t.Fatalf("resolved %d entries, want 2 layers and the second cell", len(res.Cells))
	}
	if !res.Cells[0].Layer || !res.Cells[1].Layer || res.Cells[2].Layer {
		t.Fatalf("unexpected entries: %+v", res.Cells)
	}
	if res.Cells[2].ColIdx != 1 || res.Cells[2].Bounds.X <= res.Cells[0].CellBounds.X {
		t.Errorf("the cell after a layers-only cell moved: %+v", res.Cells[2])
	}
}

// Layers resolve inside a spanning cell's whole rectangle.
func TestResolveLayersInSpanningCells(t *testing.T) {
	grid := &Grid{
		Bounds:  pptx.RectEmu{CX: PtToEMU(600), CY: PtToEMU(300)},
		Columns: []float64{25, 25, 50},
		ColGap:  10,
		RowGap:  10,
		Rows: []Row{
			{Cells: []Cell{
				{ColSpan: 2, RowSpan: 2, Layers: []Layer{arcLayer("span", 0, 0, 1, 1), arcLayer("corner", 0.9, 0.9, 0.1, 0.1)}},
				{Shape: &ShapeSpec{Geometry: "rect"}},
			}},
			{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}}},
		},
	}
	res := mustResolve(t, grid)
	span := res.Cells[0]
	if !span.Layer || span.Bounds != span.CellBounds {
		t.Fatalf("first layer = %+v, want the whole spanning cell", span)
	}
	if span.Bounds.CY != PtToEMU(300) {
		t.Errorf("row-spanning layer is %d EMU tall, want the grid's %d", span.Bounds.CY, PtToEMU(300))
	}
	if wantW := res.Cells[2].Bounds.X - PtToEMU(10); span.Bounds.CX != wantW {
		t.Errorf("col-spanning layer is %d EMU wide, want %d (two columns and their gap)", span.Bounds.CX, wantW)
	}
	corner := res.Cells[1]
	if corner.Bounds.X+corner.Bounds.CX != span.Bounds.X+span.Bounds.CX || corner.Bounds.Y+corner.Bounds.CY != span.Bounds.Y+span.Bounds.CY {
		t.Errorf("corner layer = %+v does not end at the cell's corner %+v", corner.Bounds, span.Bounds)
	}
}

// A layered cell is not a connector endpoint through its layers: the row
// connector joins the cells' own shapes.
func TestLayersAreNotConnectorEndpoints(t *testing.T) {
	filled := func() *ShapeSpec { return &ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"accent1"`)} }
	grid := layeredGrid([]float64{50, 50},
		Cell{Shape: filled(), Layers: []Layer{arcLayer("a", 0, 0, 1, 1)}},
		Cell{Shape: filled(), Layers: []Layer{arcLayer("b", 0, 0, 1, 1)}},
	)
	grid.ColGap = 20
	grid.Rows[0].Connector = &ConnectorSpec{Style: "arrow"}
	res := mustResolve(t, grid)
	if len(res.Connectors) != 1 {
		t.Fatalf("connectors = %d, want 1 between the two cells", len(res.Connectors))
	}
	conn := res.Connectors[0]
	if conn.SourceID != res.Cells[0].ID || conn.TargetID != res.Cells[2].ID {
		t.Errorf("connector joins %d -> %d, want the cells' own shapes %d -> %d", conn.SourceID, conn.TargetID, res.Cells[0].ID, res.Cells[2].ID)
	}
}

// Layers do not join their row's shared autofit shrink.
func TestLayersDoNotShareRowAutofit(t *testing.T) {
	long := json.RawMessage(`{"content":"A long label that has to shrink a good deal to fit this small box","size":14}`)
	short := json.RawMessage(`{"content":"Ok","size":14}`)
	grid := layeredGrid([]float64{50, 50},
		Cell{Shape: &ShapeSpec{Geometry: "rect", Text: short}, Layers: []Layer{{Frame: LayerFrame{X: 0, Y: 0, W: 0.1, H: 0.2}, Shape: &ShapeSpec{Geometry: "rect", Text: long}}}},
		Cell{Shape: &ShapeSpec{Geometry: "rect", Text: short}},
	)
	res, err := resolveGrid(grid, newAlloc(100), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Cells {
		if c.AutofitScale != 0 {
			t.Errorf("entry (layer=%v) took a shared autofit scale %v from a layer", c.Layer, c.AutofitScale)
		}
	}
}

func TestValidateLayerFrames(t *testing.T) {
	ok := &ShapeSpec{Geometry: "ellipse"}
	for _, tc := range []struct {
		name  string
		cell  Cell
		wants []string
	}{
		{"zero width", Cell{Layers: []Layer{{Frame: LayerFrame{X: 0, Y: 0, W: 0, H: 1}, Shape: ok}}}, []string{"layers[0]", LayerFrameOutOfCell, "no area"}},
		{"negative height", Cell{Layers: []Layer{{Frame: LayerFrame{X: 0, Y: 0, W: 1, H: -0.5}, Shape: ok}}}, []string{"layers[0]", LayerFrameOutOfCell}},
		{"runs past the right edge", Cell{Layers: []Layer{arcLayer("ok", 0, 0, 1, 1), {Name: "segment-3", Frame: LayerFrame{X: 0.6, Y: 0, W: 0.5, H: 1}, Shape: ok}}}, []string{"layers[1]", `"segment-3"`, LayerFrameOutOfCell, "leaves the cell"}},
		{"runs past the bottom edge", Cell{Layers: []Layer{{Frame: LayerFrame{X: 0, Y: 0.6, W: 1, H: 0.5}, Shape: ok}}}, []string{LayerFrameOutOfCell}},
		{"negative origin", Cell{Layers: []Layer{{Frame: LayerFrame{X: -0.1, Y: 0, W: 0.5, H: 0.5}, Shape: ok}}}, []string{LayerFrameOutOfCell}},
		{"missing shape", Cell{Layers: []Layer{{Name: "empty", Frame: LayerFrame{X: 0, Y: 0, W: 1, H: 1}}}}, []string{"layers[0]", `"empty"`, `missing "shape"`}},
		{"on a table", Cell{TableSpec: sampleTable(), Layers: []Layer{arcLayer("a", 0, 0, 1, 1)}}, []string{`"layers" alongside "table"`}},
		{"on a sub-grid", Cell{Placeholder: true, Layers: []Layer{arcLayer("a", 0, 0, 1, 1)}}, []string{`"layers" alongside "grid / pattern"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(layeredGrid([]float64{100}, tc.cell))
			if err == nil {
				t.Fatal("Validate accepted the cell")
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

// Frames computed from trigonometry land a hair past the edge; that is not
// an error.
func TestValidateLayerFramesAllowRoundingSlack(t *testing.T) {
	cell := Cell{Layers: []Layer{{Frame: LayerFrame{X: 0.50004, Y: -0.00005, W: 0.5, H: 1.00005}, Shape: &ShapeSpec{Geometry: "ellipse"}}}}
	if err := Validate(layeredGrid([]float64{100}, cell)); err != nil {
		t.Fatalf("a frame within rounding of the cell was rejected: %v", err)
	}
}

// Two resolves of one grid give the same cells, layer for layer.
func TestResolveLayersIsDeterministic(t *testing.T) {
	build := func() *Grid {
		return layeredGrid([]float64{40, 60},
			Cell{Fit: FitContain, Layers: []Layer{arcLayer("a", 0, 0, 1, 1), arcLayer("b", 1.0/3, 1.0/7, 1.0/3, 2.0/7)}},
			Cell{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(`{"content":"Label","size":14}`)}, Layers: []Layer{{Name: "badge", Frame: LayerFrame{X: 0.8, Y: 0, W: 0.2, H: 0.4}, Shape: &ShapeSpec{Geometry: "ellipse", Text: json.RawMessage(`{"content":"1","size":12}`)}}}},
		)
	}
	a, b := mustResolve(t, build()), mustResolve(t, build())
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("two resolves differ:\n%s\n%s", ja, jb)
	}
}

func sampleTable() *types.TableSpec {
	return &types.TableSpec{Headers: []string{"A"}}
}
