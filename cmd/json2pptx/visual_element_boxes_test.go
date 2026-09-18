package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// kpi4upDeck is a single kpi-4up pattern slide (4 cards in one row).
func kpi4upDeck() PresentationInput {
	values, _ := json.Marshal([]map[string]any{
		{"big": "$4.2M", "small": "ARR"},
		{"big": "87%", "small": "NPS Score"},
		{"big": "1.2K", "small": "MAU"},
		{"big": "42", "small": "Open Deals"},
	})
	title := "KPI 4-Up"
	return PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID: "slideLayout2",
			Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
			Pattern:  &PatternInput{Name: "kpi-4up", Values: values},
		}},
	}
}

func boxFor(t *testing.T, boxes []visualqa.ElementBox, path string) visualqa.BBox {
	t.Helper()
	for _, b := range boxes {
		if b.Path == path {
			return b.Box
		}
	}
	t.Fatalf("no element box for %s in %+v", path, boxes)
	return visualqa.BBox{}
}

func TestVisualElementBoxes_KPIPatternCells(t *testing.T) {
	input := kpi4upDeck()
	tdir, _ := filepath.Abs(testTemplatesDir)
	if loadDeckGeometry("midnight-blue", tdir) == nil {
		t.Fatal("loadDeckGeometry failed for a bundled template")
	}
	for name, geom := range map[string]*deckGeometry{"default": nil, "template": loadDeckGeometry("midnight-blue", tdir)} {
		t.Run(name, func(t *testing.T) {
			boxes := visualElementBoxes(&input, 0, geom)
			if len(boxes) < 4 {
				t.Fatalf("expected >=4 element boxes for kpi-4up, got %d: %+v", len(boxes), boxes)
			}
			c1 := boxFor(t, boxes, slidepath.GridCell(0, 0, 0))
			c2 := boxFor(t, boxes, slidepath.GridCell(0, 0, 1))
			if !(c2.X > c1.X) || c1.W <= 0 || c2.H <= 0 {
				t.Errorf("cells are not laid out left-to-right: c1=%+v c2=%+v", c1, c2)
			}
		})
	}
	if input.Slides[0].ShapeGrid != nil {
		t.Error("visualElementBoxes must not mutate the caller's slide")
	}
}

func TestProposeRepairs_VisualBBoxTargetsKPICell(t *testing.T) {
	input := kpi4upDeck()
	boxes := visualElementBoxes(&input, 0, nil)
	cell2 := slidepath.GridCell(0, 0, 1)
	c2 := boxFor(t, boxes, cell2)

	idx := 0
	finding := proposeRepairsFinding{
		SlideIndex:  &idx,
		Severity:    "P1",
		Category:    "text_overflow",
		Description: "KPI label overflows its card",
		// A small region well inside the second KPI card.
		BBox: &visualqa.BBox{X: c2.X + c2.W*0.3, Y: c2.Y + c2.H*0.4, W: c2.W * 0.2, H: c2.H * 0.2},
	}
	out := proposeRepairs(&input, []proposeRepairsFinding{finding})
	if len(out.Slides) != 1 || len(out.Slides[0].Directives) == 0 {
		t.Fatalf("expected directives, got %+v", out)
	}
	for _, d := range out.Slides[0].Directives {
		if d.Source.Path != cell2 || d.Target.Path != cell2 {
			t.Errorf("directive %s targets source=%q target=%q, want %q", d.Kind, d.Source.Path, d.Target.Path, cell2)
		}
		if d.Kind == "reduce_cell_text" && d.Params["cell_path"] != cell2 {
			t.Errorf("reduce_cell_text cell_path = %v, want %q", d.Params["cell_path"], cell2)
		}
	}

	// No bbox -> whole-slide fallback.
	finding.BBox = nil
	out = proposeRepairs(&input, []proposeRepairsFinding{finding})
	if got := out.Slides[0].Directives[0].Source.Path; got != slidepath.Slide(0) {
		t.Errorf("bbox-less finding path = %q, want slide path", got)
	}

	// A bbox in empty space (bottom-right corner strip) hits no cell -> fallback.
	finding.BBox = &visualqa.BBox{X: 0.98, Y: 0.98, W: 0.01, H: 0.01}
	out = proposeRepairs(&input, []proposeRepairsFinding{finding})
	if got := out.Slides[0].Directives[0].Source.Path; got != slidepath.Slide(0) {
		t.Errorf("miss path = %q, want slide path", got)
	}
}

func TestGridColumnToCellIndex_SpansAndEmpties(t *testing.T) {
	shape := &ShapeSpecInput{}
	grid := &ShapeGridInput{Rows: []GridRowInput{
		{Cells: []*GridCellInput{{Shape: shape, ColSpan: 2}, {Shape: shape}}},
		{Cells: []*GridCellInput{nil, {Shape: shape}, {Shape: shape}}},
	}}
	m := gridColumnToCellIndex(grid)
	want := map[[2]int]int{{0, 0}: 0, {0, 2}: 1, {1, 0}: 0, {1, 1}: 1, {1, 2}: 2}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("m[%v] = %d, want %d (map=%v)", k, m[k], v, m)
		}
	}
}
