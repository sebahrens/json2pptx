package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKPIWideCardsFitRangeValues(t *testing.T) {
	for _, n := range []int{2, 3} {
		p := NewKPINup(KPINupConfig{Count: n})
		cells := make(KPINupValues, n)
		for i := range cells {
			cells[i] = KPICell{Big: []string{"USD 0.02–0.10", "About 3 weeks", "under 4 weeks"}[i], Small: "Unit cost"}
		}
		if err := p.Validate(&cells, nil, nil); err != nil {
			t.Fatal(err)
		}
		grid, err := p.Expand(kpiTestCtx(), &cells, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		var text kpiTextObj
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		size := text.Paragraphs[0].Size
		width := kpiCardGeometryWithGap(kpiTestCtx(), n, kpiOpenGapPt).valueWidthPt(nil, "")
		if size < 18 || !kpiValueFits(cells[0].Big, "", size, width) {
			t.Fatalf("range unreadable at %.0fpt", size)
		}
		cells[0].Big = strings.Repeat("W", 17)
		if p.Validate(&cells, nil, nil) == nil {
			t.Fatal("unbounded value accepted")
		}
	}
}

func TestKPIExtendedValueRefusesUnreadableWidth(t *testing.T) {
	p := NewKPINup(KPINupConfig{Count: 3})
	cells := KPINupValues{{Big: "WWWWWWWWWWWWWWWW", Small: "Cost"}, {Big: "42", Small: "Count"}, {Big: "42", Small: "Days"}}
	if err := p.Validate(&cells, nil, nil); err != nil {
		t.Fatal(err)
	}
	ctx := kpiTestCtx()
	ctx.LayoutBounds.Width = 4000000
	if _, err := p.Expand(ctx, &cells, nil, nil); err == nil || !strings.Contains(err.Error(), "18pt") {
		t.Fatalf("unreadable extended figure not refused: %v", err)
	}
	for _, n := range []int{4, 5, 6} {
		dense := make(KPINupValues, n)
		for i := range dense {
			dense[i] = KPICell{Big: "USD 0.02–0.100", Small: "Cost"}
		}
		if NewKPINup(KPINupConfig{Count: n}).Validate(&dense, nil, nil) == nil {
			t.Fatalf("dense %d cap relaxed", n)
		}
	}
}

func TestKPIExtendedRangeRefusesBetweenLegacyAndReadableFloor(t *testing.T) {
	p := NewKPINup(KPINupConfig{Count: 3})
	cells := KPINupValues{{Big: "USD 0.02–0.10", Small: "Unit cost"}, {Big: "42", Small: "Count"}, {Big: "42", Small: "Days"}}
	ctx := kpiTestCtx()
	// This440pt region fits the full range at the legacy16pt floor but
	// cannot hold it at the18pt KPI reading floor.
	ctx.LayoutBounds.Width = 440 * 12700
	geo := kpiCardGeometryWithGap(ctx, 3, kpiOpenGapPt)
	available := geo.valueWidthPt(nil, "")
	if !kpiValueFits(cells[0].Big, "", 16, available) || kpiValueFits(cells[0].Big, "", 18, available) {
		t.Fatalf("fixture must straddle16–18pt at width%.1f", available)
	}
	if _, err := p.Expand(ctx, &cells, nil, nil); err == nil || !strings.Contains(err.Error(), "18pt") {
		t.Fatalf("range accepted where16pt fits but18pt does not: %v", err)
	}
}
