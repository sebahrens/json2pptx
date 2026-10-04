package patterns

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// processFlowChevronCtx is a full-slide expand context, so the geometry the
// test reasons about is the one a real slide gets.
func processFlowChevronCtx() ExpandContext {
	return ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		LayoutBounds: LayoutBounds{
			X: 457200, Y: 914400, Width: 11277600, Height: 5029200,
		},
	}
}

// A chevron step drew its label from the bounding box, so the first and last
// characters disappeared into the notch and the point, and the row's connector
// arrow was drawn straight through the shape. numbered-step-strip's chevrons
// were fixed for this in round 1; process-flow's step type was not covered
// (go-slide-creator-czk4).
func TestProcessFlowChevronKeepsItsLabelOutOfTheNotch(t *testing.T) {
	pat, ok := Default().Get("process-flow")
	if !ok {
		t.Fatal("process-flow not registered")
	}
	ctx := processFlowChevronCtx()
	vals := &ProcessFlowValues{Steps: []ProcessFlowStep{
		{Label: "Nachhaltigkeitsberichterstattung and integrated ESG data", Type: "chevron"},
		{Label: "Wesentlichkeitsanalyse across the value chain", Type: "chevron"},
		{Label: "Assurance readiness", Type: "chevron"},
		{Label: "Board sign-off", Type: "chevron"},
	}}

	grid, err := pat.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(grid.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(grid.Rows))
	}

	// Chevrons point at the next step; an arrow between them repeats it, and it
	// was being drawn through the notch.
	if grid.Rows[0].Connector != nil {
		t.Errorf("a row of chevrons still carries a connector: %+v", grid.Rows[0].Connector)
	}

	width, height := processFlowCellSize(ctx, len(vals.Steps), processFlowStepGapPt(ctx, vals.Steps), true)
	notch := float64(chevronAdj) / 100000 * math.Min(width, height)
	if notch <= 0 {
		t.Fatal("notch depth computed as zero")
	}
	for i, cell := range grid.Rows[0].Cells {
		if cell == nil || cell.Shape == nil {
			t.Fatalf("cell %d is empty", i)
		}
		if cell.Shape.Geometry != "chevron" {
			t.Errorf("cell %d geometry = %q, want chevron", i, cell.Shape.Geometry)
		}
		// The shallower point is what leaves room for the label at all.
		if got := cell.Shape.Adjustments["adj"]; got != chevronAdj {
			t.Errorf("cell %d adj = %d, want %d", i, got, chevronAdj)
		}

		// The preset's text rectangle already reserves the notch; the label
		// keeps only the uniform shape margin (no inset_* of its own).
		var text map[string]any
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
			if _, has := text[k]; has {
				t.Errorf("cell %d overrides the uniform shape margin with %s (notch %.1fpt is reserved by the preset)", i, k, notch)
			}
		}
	}

	// The notch is a fraction of the SHORTER side, so a tall chevron eats its
	// own label: the row is capped to half its step width.
	if height > math.Round(width*chevronMaxAspectH) {
		t.Errorf("pointed row height %.0fpt exceeds half its %.0fpt step width", height, width)
	}
	// Content-sized below the pointed cap (go-slide-creator-xb06p).
	if got := grid.Rows[0].MaxHeight; got <= 0 || got > height {
		t.Errorf("row max_height = %.0f, want content-sized within the capped %.0f", got, height)
	}
	// The label is left with a usable share of the shape rather than a column.
	if textWidth := width - 2*(notch+defaultShapeInsetLRPt); textWidth < width*0.45 {
		t.Errorf("the label gets %.0fpt of a %.0fpt chevron (%.0f%%); it used to be a column", textWidth, width, textWidth/width*100)
	}
}

// A flow of plain steps is untouched: rectangles need no notch inset, and the
// arrows between them are the only thing saying which way it runs.
func TestProcessFlowPlainStepsKeepTheirConnectors(t *testing.T) {
	pat, _ := Default().Get("process-flow")
	ctx := processFlowChevronCtx()
	vals := &ProcessFlowValues{Steps: []ProcessFlowStep{
		{Label: "Draft"}, {Label: "Approved?", Type: "decision"}, {Label: "Rehearse"},
	}}

	grid, err := pat.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if grid.Rows[0].Connector == nil {
		t.Error("a flow of plain steps lost its connectors")
	}
	for i, cell := range grid.Rows[0].Cells {
		if len(cell.Shape.Adjustments) != 0 {
			t.Errorf("cell %d carries a chevron adjustment: %+v", i, cell.Shape.Adjustments)
		}
		var text map[string]any
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		if cell.Shape.Geometry == "diamond" {
			// The diamond's text sits in the preset's inner rectangle and
			// keeps only a thin margin inside it (go-slide-creator-xb06p).
			if got, _ := text["inset_left"].(float64); got != processFlowDiamondInsetPt {
				t.Errorf("decision cell %d inset_left = %v, want %v", i, text["inset_left"], processFlowDiamondInsetPt)
			}
			continue
		}
		if _, has := text["inset_left"]; has {
			t.Errorf("cell %d was inset for a notch it does not have", i)
		}
	}
	// A row that is not all-pointed is content-sized: at least the box
	// proportion of its step width, at most the plain cap
	// (go-slide-creator-xb06p).
	plainWidth, plainHeight := processFlowCellSize(ctx, len(vals.Steps), processFlowStepGapPt(ctx, vals.Steps), false)
	if got := grid.Rows[0].MaxHeight; got > plainHeight || got < math.Round(plainWidth*processFlowBoxAspect)-1 && got < plainHeight {
		t.Errorf("row max_height = %.0f, want between the %.0f box floor and the %.0f cap", got, plainWidth*processFlowBoxAspect, plainHeight)
	}
}

// An arrow step's label lives in its shaft (go-slide-creator-fx48s): the step
// is a block arrow whose shaft is processFlowArrowShaftAdj of its height, the
// label keeps the uniform margin left and right and processFlowArrowInsetPt
// above and below, and the row is tall enough for the shaft to hold the label
// at the written size. The preset's half-height shaft under the uniform 0.5 cm
// margin left a 60pt step 2pt of text height, and the label rendered at ~3pt.
func TestProcessFlowArrowLabelsFitTheShaft(t *testing.T) {
	ctx := processFlowChevronCtx()
	for _, name := range []string{"process-flow", "process-flow-compact"} {
		pat, _ := Default().Get(name)
		for _, labels := range [][]string{
			{"Collect", "Reconcile", "Settle"},
			{"Understand the customer need in depth", "Design the offer and price it", "Build and test with pilot users", "Launch and learn from the market"},
			{"Intake", "Triage", "Assess risk", "Decide", "Fulfil order", "Close out"},
		} {
			steps := make([]ProcessFlowStep, len(labels))
			for i, l := range labels {
				steps[i] = ProcessFlowStep{Label: l, Type: "arrow"}
			}
			// A plain step among them: the row is shared.
			steps[0].Type = "step"
			grid, err := pat.Expand(ctx, &ProcessFlowValues{Steps: steps}, nil, nil)
			if err != nil {
				t.Fatalf("%s: expand: %v", name, err)
			}
			rowPt := grid.Rows[0].MaxHeight
			contentW, _ := contentAreaPt(ctx)
			cellW := (contentW - grid.Gap*float64(len(steps)-1)) / float64(len(steps))
			for i, cell := range grid.Rows[0].Cells {
				if cell.Shape.Geometry != "rightArrow" {
					continue
				}
				if cell.Shape.Adjustments["adj1"] != processFlowArrowShaftAdj || cell.Shape.Adjustments["adj2"] != processFlowArrowHeadAdj {
					t.Fatalf("%s: arrow cell %d adjustments = %v", name, i, cell.Shape.Adjustments)
				}
				tb, err := shapegrid.ResolveTextInput(cell.Shape.Text)
				if err != nil {
					t.Fatalf("cell %d text: %v", i, err)
				}
				uniform := pptx.ShapeTextInsets()
				tight := int64(processFlowArrowInsetPt * sizingEMUPerPt)
				if want := [4]int64{uniform[0], tight, uniform[2], tight}; tb.Insets != want {
					t.Errorf("%s: arrow cell %d insets = %v, want %v", name, i, tb.Insets, want)
				}
				bounds := pptx.RectEmu{CX: int64(cellW * sizingEMUPerPt), CY: int64(rowPt * sizingEMUPerPt)}
				w, h := pptx.PresetTextRect(cell.Shape.Geometry, cell.Shape.Adjustments, bounds)
				if !pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: w, CY: h}) {
					t.Errorf("%s: %q does not fit its %.0f×%.0fpt shaft unshrunk (row %.0fpt)", name, labels[i], float64(w)/sizingEMUPerPt, float64(h)/sizingEMUPerPt, rowPt)
				}
			}
		}
	}
}
