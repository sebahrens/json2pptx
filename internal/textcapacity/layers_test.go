package textcapacity

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// badgeGrid is a 220pt square cell holding a ring segment (no text) and a 22pt
// badge (a tenth of the square) with the given text at 12pt and 1pt insets.
func badgeGrid(badge string) *shapegrid.Grid {
	text, _ := json.Marshal(map[string]any{
		"content": badge, "size": 12, "align": "ctr", "vertical_align": "ctr",
		"inset_left": 1, "inset_right": 1, "inset_top": 1, "inset_bottom": 1,
	})
	return &shapegrid.Grid{
		Bounds:  pptx.RectEmu{CX: shapegrid.PtToEMU(220), CY: shapegrid.PtToEMU(220)},
		Columns: []float64{100},
		Rows: []shapegrid.Row{{Cells: []shapegrid.Cell{{
			Fit: shapegrid.FitContain,
			Layers: []shapegrid.Layer{
				{Name: "segment", Frame: shapegrid.LayerFrame{X: 0, Y: 0, W: 1, H: 1}, Shape: &shapegrid.ShapeSpec{Geometry: "blockArc", Fill: json.RawMessage(`"accent1"`)}},
				{Name: "badge", Frame: shapegrid.LayerFrame{X: 0.8, Y: 0.1, W: 0.1, H: 0.1}, Shape: &shapegrid.ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`"dk2"`), Text: text}},
			},
		}}}},
	}
}

func resolveForTest(t *testing.T, grid *shapegrid.Grid) *shapegrid.ResolveResult {
	t.Helper()
	res, err := shapegrid.Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil || res == nil {
		t.Fatalf("Resolve: %v", err)
	}
	return res
}

// Layer text has a capacity budget of its own, measured in the layer's frame.
func TestForResolvedGridMeasuresLayerText(t *testing.T) {
	res := resolveForTest(t, badgeGrid("12"))
	d := ForResolvedGrid(res)
	if len(d) != 2 {
		t.Fatalf("densities = %d, want one per layer", len(d))
	}
	if d[0].ActualChars != 0 {
		t.Errorf("the textless segment layer reports text: %+v", d[0])
	}
	badge := d[1]
	if !res.Cells[1].Layer || res.Cells[1].LayerName != "badge" {
		t.Fatalf("entry 1 is not the badge layer: %+v", res.Cells[1])
	}
	if badge.ActualChars != 2 || !badge.Fits || badge.AutofitScale != 1 {
		t.Errorf("a 2-character 12pt badge in a 22pt frame should fit unshrunk: %+v", badge)
	}
	if got := float64(badge.WidthEMU) / 12700; got < 19.9 || got > 20.1 {
		t.Errorf("badge text width = %.2fpt, want the layer's 22pt frame less its 1pt insets", got)
	}
	if badge.AvailableHeightPt < 19.9 || badge.AvailableHeightPt > 20.1 {
		t.Errorf("badge text height = %.2fpt, want the layer's 22pt frame less its 1pt insets", badge.AvailableHeightPt)
	}
}

// Text that cannot fit its layer's frame is an overflow, exactly as in a cell.
func TestForResolvedGridReportsLayerOverflow(t *testing.T) {
	res := resolveForTest(t, badgeGrid("A forty-character label for one badge.."))
	badge := ForResolvedGrid(res)[1]
	if badge.Status != StatusOverflow || badge.AutofitScale >= 1 || badge.DensityPct <= 100 {
		t.Errorf("a 40-character label in a 22pt badge should overflow and need a shrink: %+v", badge)
	}
	if badge.ActualChars <= badge.MaxChars {
		t.Errorf("the label's %d characters are within the badge's budget of %d", badge.ActualChars, badge.MaxChars)
	}
	if badge.RequiredHeightPt <= badge.AvailableHeightPt {
		t.Errorf("required %.1fpt does not exceed the available %.1fpt", badge.RequiredHeightPt, badge.AvailableHeightPt)
	}
}

// The budget guide reads layer text: a grid whose only text sits in a layer
// still gets a configuration, sized by the layer's frame and not by the
// textless drawing layers around it.
func TestBudgetGuideCountsLayerText(t *testing.T) {
	expand := func(labelFrame jsonschema.LayerFrameInput) func(int, int) (*jsonschema.ShapeGridInput, error) {
		return func(cols, rows int) (*jsonschema.ShapeGridInput, error) {
			return &jsonschema.ShapeGridInput{
				Columns: json.RawMessage(`1`),
				Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{
					Layers: []jsonschema.LayerInput{
						{Name: "segment", Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1}, Shape: &jsonschema.ShapeSpecInput{Geometry: "blockArc", Fill: json.RawMessage(`"accent1"`)}},
						{Name: "label", Frame: labelFrame, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`{"content":"Label","size":12}`)}},
					},
				}}}},
			}, nil
		}
	}
	bounds := pptx.RectEmu{CX: shapegrid.PtToEMU(600), CY: shapegrid.PtToEMU(300)}
	configs := []GridBudgetConfig{{Columns: 1, Rows: 1}}
	wide := ComputeBudgetGuide(configs, expand(jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 0.5}), bounds, 0, 0)
	narrow := ComputeBudgetGuide(configs, expand(jsonschema.LayerFrameInput{X: 0, Y: 0, W: 0.25, H: 0.5}), bounds, 0, 0)
	if wide == nil || narrow == nil || len(wide.Configurations) != 1 || len(narrow.Configurations) != 1 {
		t.Fatalf("no budget for a grid whose text is in a layer: %+v / %+v", wide, narrow)
	}
	w, n := wide.Configurations[0].BodyMaxChars, narrow.Configurations[0].BodyMaxChars
	if w <= 0 || n <= 0 || n >= w {
		t.Errorf("body budgets = %d (full-width label) and %d (quarter-width label); want both positive and the narrow one smaller", w, n)
	}
}
