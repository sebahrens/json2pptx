package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestKPICellSubAliases verifies that the sub/delta/trend/change keys are
// captured into KPICell.Sub instead of being silently dropped (go-slide-creator-09pa).
func TestKPICellSubAliases(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"canonical_sub", `{"big":"$50M","small":"Revenue","sub":"+5%"}`, "+5%"},
		{"alias_delta", `{"value":"$50M","label":"Revenue","delta":"+5%"}`, "+5%"},
		{"alias_trend", `{"big":"127%","small":"NRR","trend":"+3%"}`, "+3%"},
		{"alias_change", `{"big":"2.1%","small":"Churn","change":"-0.4%"}`, "-0.4%"},
		{"no_sub", `{"big":"$50M","small":"Revenue"}`, ""},
		{"sub_precedence_over_delta", `{"big":"x","small":"y","sub":"a","delta":"b"}`, "a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cell KPICell
			if err := json.Unmarshal([]byte(tc.json), &cell); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if cell.Sub != tc.want {
				t.Errorf("Sub = %q, want %q", cell.Sub, tc.want)
			}
		})
	}
}

// TestKPISubRendered verifies the sub annotation reaches the expanded shape text
// as its own paragraph, below the caption at the same baseline in every card.
func TestKPISubRendered(t *testing.T) {
	p, ok := Default().Get("kpi-3up")
	if !ok {
		t.Fatal("kpi-3up not registered")
	}
	vals := KPINupValues{
		{Big: "$50M", Small: "Revenue", Sub: "+5%"},
		{Big: "127%", Small: "NRR", Sub: "+3%"},
		{Big: "2.1%", Small: "Churn"}, // no sub
	}
	grid, err := p.Expand(ExpandContext{}, &vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	type para struct {
		Content string `json:"content"`
	}
	type textObj struct {
		Paragraphs []para `json:"paragraphs"`
	}

	parse := func(i int) textObj {
		var to textObj
		if err := json.Unmarshal(grid.Rows[0].Cells[i].Shape.Text, &to); err != nil {
			t.Fatalf("cell[%d] text unmarshal: %v", i, err)
		}
		return to
	}

	// Cell 0 has a sub: the caption belongs to the number, then the delta.
	c0 := parse(0)
	if len(c0.Paragraphs) != 3 {
		t.Fatalf("cell[0] paragraphs = %d, want 3", len(c0.Paragraphs))
	}
	gotOrder := []string{c0.Paragraphs[0].Content, c0.Paragraphs[1].Content, c0.Paragraphs[2].Content}
	wantOrder := []string{"$50M", "Revenue", "+5%"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("cell[0] paragraph[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}

	// Cell 2 has no sub: a blank final paragraph preserves the same baselines.
	c2 := parse(2)
	if len(c2.Paragraphs) != 3 {
		t.Fatalf("cell[2] paragraphs = %d, want 3", len(c2.Paragraphs))
	}
	if c2.Paragraphs[1].Content != "Churn" || c2.Paragraphs[2].Content != "\u00a0" {
		t.Errorf("cell[2] caption/delta = %+v, want Churn plus reserved blank delta", c2.Paragraphs)
	}
	if strings.Contains(string(grid.Rows[0].Cells[2].Shape.Text), "+") {
		t.Errorf("cell[2] should not contain a delta annotation")
	}
}

// go-slide-creator-4uxi: the constant was called kpiMaxCardHeightFrac but was
// passed to clampPt as the LOWER bound, with the content box as the upper one
// — so the documented "45% cap" was really a floor, and a reviewer asserting
// it got a false answer. These tests pin the invariant that is actually true:
// the row rests on the 45% BASE and is raised only to what the tallest card's
// measured content needs, never past the content box.
func TestKPIRowHeightInvariant(t *testing.T) {
	ctx := ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: LayoutBounds{Width: 10515600, Height: 3913340},
	}
	_, contentH := contentAreaPt(ctx)

	light := KPINupValues{
		{Big: "42%", Small: "Growth"},
		{Big: "1.2M", Small: "Users"},
		{Big: "98", Small: "NPS"},
		{Big: "4.8", Small: "Rating"},
		{Big: "12d", Small: "Cycle"},
		{Big: "3x", Small: "ROI"},
	}
	// Schema-maxima content: the only shape that pushes a card past the base.
	heavy := KPINupValues{
		{Big: "1,240.5", Sub: "Wirtschaftlichkeitsberechnung", Small: "Geschaeftsbereichsverantwortliche across all four regions must sign off"},
		{Big: "980.25", Sub: "Lieferantenrahmenvertraege", Small: "Procurement consolidation delivers addressable spend reduction once renegotiated"},
		{Big: "870.10", Sub: "Bestandsfuehrungssysteme", Small: "Customer onboarding still requires seventeen manual handoffs between teams"},
		{Big: "742.00", Sub: "Vertriebsinnendienst", Small: "Kreditrisikomanagement and Rechtsabteilung extend the cycle to forty-three days"},
		{Big: "618.75", Sub: "Kreditrisikomanagement", Small: "Harmonising the twelve legacy platforms is the gating dependency for phase two"},
		{Big: "503.20", Sub: "Rechtsabteilung", Small: "Steering committee releases the second funding tranche after the review"},
	}

	for n := 2; n <= 6; n++ {
		name := fmt.Sprintf("kpi-%dup", n)
		p, ok := Default().Get(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		for _, c := range []struct {
			label  string
			vals   KPINupValues
			atBase bool
		}{
			{"light", light[:n], true},
			{"heavy", heavy[:n], false},
		} {
			t.Run(name+"/"+c.label, func(t *testing.T) {
				vals := c.vals
				grid, err := p.Expand(ctx, &vals, nil, nil)
				if err != nil {
					t.Fatalf("Expand: %v", err)
				}
				got := grid.Rows[0].MaxHeight
				if got > contentH+0.5 {
					t.Errorf("row max_height %.1fpt exceeds the %.1fpt content box", got, contentH)
				}
				// Content-sized (go-slide-creator-wntyw): content that fits
				// must not stretch the card towards the zone height.
				if c.atBase && got > contentH*0.45 {
					t.Errorf("light KPI row %.1fpt uses %.0f%% of the %.1fpt zone; cards must hug their content", got, got/contentH*100, contentH)
				}
				if got < 60 {
					t.Errorf("row max_height %.1fpt is too short for a value and caption", got)
				}
			})
		}
	}
}

// Content that needs more than its area is refused, not written into it: the
// row used to be clipped to the content box and its lines shrunk into each
// other (go-slide-creator-uj9zq).
func TestKPIRowTallerThanItsAreaIsRefused(t *testing.T) {
	// A short content zone makes even modest content exceed the zone.
	ctx := ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: LayoutBounds{Width: 10515600, Height: 1200000},
	}
	vals := KPINupValues{
		{Big: "1,240.5", Sub: "Wirtschaftlichkeitsberechnung", Small: "Geschaeftsbereichsverantwortliche across all four regions must sign off before the tranche"},
		{Big: "980.25", Sub: "Lieferantenrahmenvertraege", Small: "Procurement consolidation delivers addressable spend reduction once renegotiated"},
		{Big: "870.10", Sub: "Bestandsfuehrungssysteme", Small: "Customer onboarding still requires seventeen manual handoffs between teams"},
	}
	p, _ := Default().Get("kpi-3up")
	_, err := p.Expand(ctx, &vals, nil, nil)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != ErrCodeFitOverflow || ve.Path != "values" {
		t.Fatalf("Expand error = %v, want a fit_overflow refusal at values", err)
	}
	if !strings.Contains(ve.Message, "3 KPIs need") || !strings.Contains(ve.Message, "94pt tall") {
		t.Errorf("refusal does not name the need and the area: %s", ve.Message)
	}
}
