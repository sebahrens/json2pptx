package patterns

import (
	"encoding/json"
	"math"
	"testing"
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

	notch := processFlowNotchPt(ctx, len(vals.Steps))
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

		var text struct {
			InsetLeft  float64 `json:"inset_left"`
			InsetRight float64 `json:"inset_right"`
		}
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		if text.InsetLeft < notch || text.InsetRight < notch {
			t.Errorf("cell %d insets (%.1f, %.1f) do not clear the %.1fpt notch",
				i, text.InsetLeft, text.InsetRight, notch)
		}
	}

	// The notch is a fraction of the SHORTER side, so a tall chevron eats its
	// own label: the row is capped to half its step width.
	width, height := processFlowCellSize(ctx, len(vals.Steps), true)
	if height > math.Round(width*chevronMaxAspectH) {
		t.Errorf("pointed row height %.0fpt exceeds half its %.0fpt step width", height, width)
	}
	if grid.Rows[0].MaxHeight != height {
		t.Errorf("row max_height = %.0f, want the capped %.0f", grid.Rows[0].MaxHeight, height)
	}
	// The label is left with a usable share of the shape rather than a column.
	if textWidth := width - 2*(notch+chevronTextPadPt); textWidth < width*0.6 {
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
		if _, has := text["inset_left"]; has {
			t.Errorf("cell %d was inset for a notch it does not have", i)
		}
	}
	// A row that is not all-pointed keeps the taller box.
	_, plainHeight := processFlowCellSize(ctx, len(vals.Steps), false)
	if grid.Rows[0].MaxHeight != plainHeight {
		t.Errorf("row max_height = %.0f, want the uncapped %.0f", grid.Rows[0].MaxHeight, plainHeight)
	}
}

// A single arrow step is pointed too, and gets the same treatment.
func TestProcessFlowArrowStepsAreInsetLikeChevrons(t *testing.T) {
	pat, _ := Default().Get("process-flow")
	vals := &ProcessFlowValues{Steps: []ProcessFlowStep{
		{Label: "Collect", Type: "arrow"}, {Label: "Reconcile", Type: "arrow"}, {Label: "Settle", Type: "arrow"},
	}}
	grid, err := pat.Expand(processFlowChevronCtx(), vals, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for i, cell := range grid.Rows[0].Cells {
		var text struct {
			InsetLeft float64 `json:"inset_left"`
		}
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		if text.InsetLeft <= 0 {
			t.Errorf("arrow cell %d has no side inset; its point will eat the label", i)
		}
	}
}
