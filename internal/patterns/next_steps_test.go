package patterns

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestNextStepsRegistered(t *testing.T) {
	if _, ok := Default().Get("next-steps"); !ok {
		t.Fatal("next-steps pattern not registered")
	}
}

func TestNextStepsValidate(t *testing.T) {
	p := &nextSteps{}
	ok := &NextStepsValues{Actions: []NextStepsAction{{Action: "A"}, {Action: "B", Owner: "COO", Date: "Oct"}}}
	if err := p.Validate(ok, nil, nil); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	for name, v := range map[string]*NextStepsValues{
		"one action":     {Actions: []NextStepsAction{{Action: "A"}}},
		"seven actions":  {Actions: make([]NextStepsAction, 7)},
		"empty action":   {Actions: []NextStepsAction{{Action: "A"}, {Owner: "COO"}}},
		"long owner":     {Actions: []NextStepsAction{{Action: "A"}, {Action: "B", Owner: strings.Repeat("x", 31)}}},
		"four decisions": {Actions: []NextStepsAction{{Action: "A"}, {Action: "B"}}, Decisions: []string{"a", "b", "c", "d"}},
		"long decision":  {Actions: []NextStepsAction{{Action: "A"}, {Action: "B"}}, Decisions: []string{strings.Repeat("x", 121)}},
	} {
		if err := p.Validate(v, nil, nil); err == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
}

// TestNextStepsLayout pins design review C6 (go-slide-creator-7lzdh): numbered
// rows (action · owner · date) under a header, 0.5pt rules, no tiles, and a
// decisions band with a left accent rule — no outline, no fill.
func TestNextStepsLayout(t *testing.T) {
	grid, err := (&nextSteps{}).Expand(ExpandContext{}, (&nextSteps{}).ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 5 {
		t.Fatalf("columns = %s, want band rule/number/action/owner/date", grid.Columns)
	}
	// header, then (rule, row) x3, then spacer + band.
	if len(grid.Rows) != 1+2*3+2 {
		t.Fatalf("rows = %d", len(grid.Rows))
	}
	for i := 1; i <= 5; i += 2 {
		if grid.Rows[i].MaxHeight != nextStepsRulePt {
			t.Errorf("row %d is not a 0.5pt rule", i)
		}
	}
	for _, c := range grid.Rows[2].Cells {
		if string(c.Shape.Fill) != `"none"` {
			t.Errorf("action cell fill = %s, want none", c.Shape.Fill)
		}
	}
	// The band's rule fills the first column, flush on the edge the row rules
	// start on, and the band text starts 12pt to its right
	// (go-slide-creator-le9d0).
	last := grid.Rows[len(grid.Rows)-1].Cells
	if len(last) != 2 || last[0].Shape == nil || string(last[0].Shape.Fill) != `"accent1"` || len(last[0].Shape.Text) != 0 {
		t.Fatalf("band row = %+v, want an accent rule cell and the band text", last)
	}
	if got := cols[0] / 100 * 864 * sizingDefaultWidthFrac; math.Abs(got-nextStepsBandBarPt) > 0.2 {
		t.Errorf("band rule column = %.2fpt, want %vpt", got, nextStepsBandBarPt)
	}
	band := last[1]
	if band.ColSpan != 4 || band.AccentBar != nil {
		t.Fatalf("band = %+v, want the text spanning the table beside its rule", band)
	}
	var bandText struct {
		InsetLeft float64 `json:"inset_left"`
	}
	if err := json.Unmarshal(band.Shape.Text, &bandText); err != nil || bandText.InsetLeft != TakeawayTextInsetPt {
		t.Errorf("band text inset_left = %v, want %v", bandText.InsetLeft, TakeawayTextInsetPt)
	}
	for _, r := range []int{0, 2} {
		if grid.Rows[r].Cells[0].ColSpan != 2 {
			t.Errorf("row %d: first cell spans %d columns, want 2 (rule column + numeral column)", r, grid.Rows[r].Cells[0].ColSpan)
		}
	}
	if string(band.Shape.Fill) != `"none"` || string(band.Shape.Line) != `"none"` {
		t.Errorf("band fill/line = %s/%s, want none/none", band.Shape.Fill, band.Shape.Line)
	}
	if !strings.Contains(string(band.Shape.Text), "Decisions requested") || !strings.Contains(string(band.Shape.Text), "Approve the") {
		t.Errorf("band text = %s", band.Shape.Text)
	}
}

func TestNextStepsDropsEmptyColumns(t *testing.T) {
	grid, err := (&nextSteps{}).Expand(ExpandContext{}, &NextStepsValues{Actions: []NextStepsAction{{Action: "A", Date: "Oct"}, {Action: "B"}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 3 {
		t.Fatalf("columns = %s, want number/action/date (no owner column)", grid.Columns)
	}
	if last := grid.Rows[len(grid.Rows)-1]; len(last.Cells) != 3 {
		t.Errorf("no decisions: the last row should be an action row, got %d cells", len(last.Cells))
	}
}
