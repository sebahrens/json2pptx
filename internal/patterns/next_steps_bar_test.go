package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// The decisions are a flush accent rule beside unfilled dk1 text by default,
// which overrides.takeaway_emphasis "bar" names; "band" is the dark neutral
// band.
func TestNextStepsDecisionsBarOverride(t *testing.T) {
	p := &nextSteps{}
	vals := p.ExemplarValues().(*NextStepsValues)
	ctx := restraintCtx()

	for _, e := range []string{"bar", "band"} {
		if err := p.Validate(vals, &NextStepsOverrides{TakeawayEmphasis: e}, nil); err != nil {
			t.Fatalf("takeaway_emphasis %s must validate: %v", e, err)
		}
	}
	err := p.Validate(vals, &NextStepsOverrides{TakeawayEmphasis: "subtle"}, nil)
	if err == nil || !strings.Contains(err.Error(), "overrides.takeaway_emphasis must be bar") {
		t.Errorf("takeaway_emphasis subtle must be refused with the allowed value, got %v", err)
	}

	band, err := p.Expand(ctx, vals, &NextStepsOverrides{TakeawayEmphasis: "band"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bar, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	named, err := p.Expand(ctx, vals, &NextStepsOverrides{TakeawayEmphasis: "bar"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := mustJSON(t, bar), mustJSON(t, named); a != b {
		t.Errorf("takeaway_emphasis bar should be the default look")
	}

	lastBand := band.Rows[len(band.Rows)-1]
	wantFill, _ := TakeawayBandTone(ctx)
	if len(lastBand.Cells) != 1 || string(lastBand.Cells[0].Shape.Fill) != string(wantFill) {
		t.Fatalf("band decisions row should be one cell in the band tone, got %d cells", len(lastBand.Cells))
	}

	lastBar := bar.Rows[len(bar.Rows)-1]
	if len(lastBar.Cells) != 2 {
		t.Fatalf("bar decisions row should be [rule, text], got %d cells", len(lastBar.Cells))
	}
	rule, text := lastBar.Cells[0], lastBar.Cells[1]
	if string(rule.Shape.Fill) != `"`+ctx.DefaultAccent()+`"` || len(rule.Shape.Text) != 0 {
		t.Errorf("the rule is a bare accent rect, got fill %s text %s", rule.Shape.Fill, rule.Shape.Text)
	}
	if len(text.Shape.Fill) != 0 && string(text.Shape.Fill) != `"none"` {
		t.Errorf("the bar variant's text carries no fill, got %s", text.Shape.Fill)
	}
	if !strings.Contains(string(text.Shape.Text), `"color":"dk1"`) {
		t.Errorf("the asks are set in dk1 beside the rule: %s", text.Shape.Text)
	}
	// The rule takes a column of its own, 3pt wide, at the table's left edge.
	var bandCols, barCols []float64
	if err := json.Unmarshal(band.Columns, &bandCols); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bar.Columns, &barCols); err != nil {
		t.Fatal(err)
	}
	areaW, _ := sizingAreaPt(ctx)
	if len(barCols) != len(bandCols)+1 || barCols[0]*areaW/100 < 2.5 || barCols[0]*areaW/100 > 3.5 {
		t.Errorf("bar columns = %v (band columns %v), want a leading ~3pt rule column", barCols, bandCols)
	}
	// The schema names the override.
	raw, _ := json.Marshal(p.Schema())
	if !strings.Contains(string(raw), `"takeaway_emphasis"`) {
		t.Error("next-steps schema does not document overrides.takeaway_emphasis")
	}
}
