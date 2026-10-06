package patterns

import (
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// go-slide-creator-cuq95: the default process flow was a row of equal grey
// boxes joined by thin arrows. Its steps are now interlocking arrows in a
// light accent tint, one of them solid, with a numeral over each label.

func TestProcessFlowChevronLook(t *testing.T) {
	ctx := processFlowChevronCtx()
	ctx.Theme = testThemeCtx().Theme
	steps := []ProcessFlowStep{
		{Label: "Team files the change request with an impact assessment"},
		{Label: "Within policy?", Type: "decision"},
		{Label: "Security and architecture review the design", Highlight: true},
		{Label: "Change board approves a release window"},
	}
	p, _ := Default().Get("process-flow")
	grid, err := p.Expand(ctx, &ProcessFlowValues{Steps: steps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := grid.Rows[0]
	if row.Connector != nil || len(grid.Links) != 0 || grid.Gap != processFlowChevronGapPt {
		t.Errorf("connector %+v, %d links, gap %.0fpt; want interlocking steps over the %.0fpt hairline", row.Connector, len(grid.Links), grid.Gap, processFlowChevronGapPt)
	}
	if n := solidAccentCells(grid); n != 1 {
		t.Errorf("%d solid accent shapes, want the highlighted step alone", n)
	}

	contentW, _ := contentAreaPt(ctx)
	colW := (contentW - 3*grid.Gap) / 4
	wantGeometry := []string{"homePlate", "diamond", "chevron", "chevron"}
	wantNumeral := []string{"01", "", "02", "03"}
	for i, cell := range row.Cells {
		if cell.Shape.Geometry != wantGeometry[i] {
			t.Errorf("cell %d geometry = %q, want %q", i, cell.Shape.Geometry, wantGeometry[i])
		}
		paras := cellText(t, cell.Shape.Text).Paragraphs
		if steps[i].Type == "decision" {
			// A decision is a gate between steps: not numbered, not counted.
			if len(paras) != 1 || cell.BleedLeft != 0 {
				t.Errorf("decision cell has %d paragraphs and bleeds %.0fpt, want its bare question in its own column", len(paras), cell.BleedLeft)
			}
			continue
		}
		if len(paras) != 2 || paras[0].Content != wantNumeral[i] || paras[1].Content != steps[i].Label {
			t.Fatalf("cell %d paragraphs = %+v, want numeral %q over the label", i, paras, wantNumeral[i])
		}
		if paras[0].Size < 18 || paras[0].Size <= paras[1].Size {
			t.Errorf("cell %d numeral is %.0fpt over a %.0fpt label, want a display numeral", i, paras[0].Size, paras[1].Size)
		}
		if i != 2 && !strings.Contains(string(cell.Shape.Fill), "lumMod") {
			t.Errorf("cell %d fill = %s, want a tint of the accent", i, cell.Shape.Fill)
		}

		// The notch is the same depth on every step, each later step bleeds
		// by it, and the text the row is sized for fits the preset's own
		// text rectangle unshrunk.
		bounds := pptx.RectEmu{CX: int64((colW + cell.BleedLeft) * sizingEMUPerPt), CY: int64(row.MaxHeight * sizingEMUPerPt)}
		notch := float64(cell.Shape.Adjustments["adj"]) / 100000 * math.Min(colW+cell.BleedLeft, row.MaxHeight)
		if notch < processFlowMinNotchPt-0.5 || notch > processFlowMaxNotchPt+0.5 {
			t.Errorf("cell %d notch = %.1fpt, want within %.0f–%.0fpt", i, notch, processFlowMinNotchPt, processFlowMaxNotchPt)
		}
		if wantBleed := i > 0; (cell.BleedLeft > 0) != wantBleed || (wantBleed && math.Abs(cell.BleedLeft-notch) > 0.5) {
			t.Errorf("cell %d bleeds %.1fpt with a %.1fpt notch", i, cell.BleedLeft, notch)
		}
		tb, err := shapegrid.ResolveTextInput(cell.Shape.Text)
		if err != nil {
			t.Fatal(err)
		}
		w, h := pptx.PresetTextRect(cell.Shape.Geometry, cell.Shape.Adjustments, bounds)
		if !pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: w, CY: h}) {
			t.Errorf("cell %d: %q does not fit its %.0f×%.0fpt text rectangle unshrunk", i, steps[i].Label, float64(w)/sizingEMUPerPt, float64(h)/sizingEMUPerPt)
		}
	}

	// The compact band interlocks bare, centred labels.
	compact, err := (&processFlowCompact{}).Expand(ctx, &ProcessFlowValues{Steps: steps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if compact.Rows[0].MaxHeight >= row.MaxHeight {
		t.Errorf("compact band %.0fpt is not shallower than the flow's %.0fpt row", compact.Rows[0].MaxHeight, row.MaxHeight)
	}
	for i, cell := range compact.Rows[0].Cells {
		if cell.Shape.Geometry != wantGeometry[i] {
			t.Errorf("compact cell %d geometry = %q, want %q", i, cell.Shape.Geometry, wantGeometry[i])
		}
		if paras := cellText(t, cell.Shape.Text).Paragraphs; len(paras) != 1 {
			t.Errorf("compact cell %d has %d paragraphs, want the bare label", i, len(paras))
		}
	}

	// A flow that names a chevron or an arrow keeps those shapes and is not
	// numbered: a numeral on some steps only would count nothing.
	mixed := []ProcessFlowStep{{Label: "Intake"}, {Label: "Assess", Type: "chevron"}, {Label: "Approve", Type: "arrow"}, {Label: "Pay out"}}
	grid, err = p.Expand(ctx, &ProcessFlowValues{Steps: mixed}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, cell := range grid.Rows[0].Cells {
		if paras := cellText(t, cell.Shape.Text).Paragraphs; len(paras) != 1 {
			t.Errorf("mixed flow cell %d has %d paragraphs, want no numeral", i, len(paras))
		}
	}
	if got := grid.Rows[0].Cells[1].Shape.Adjustments["adj"]; got != chevronAdj {
		t.Errorf("an explicit chevron's adj = %d, want its own %d notch", got, chevronAdj)
	}
}

// On the returning row a step is mirrored, so the side that reaches over the
// gap is its point: it bleeds only into a step that has a notch to take it.
func TestProcessFlowChevronReturningRowBleed(t *testing.T) {
	steps := pfSteps(8)
	steps[6] = ProcessFlowStep{Label: "Live?", Type: "decision"}
	lay := processFlowLayoutFor(steps, nil)
	look := processFlowLookFor(steps, nil, lay, 200, true)
	if !look.returning(5) || look.returning(3) {
		t.Fatalf("layout %+v: want steps 5-8 on the returning row", lay)
	}
	// Step 6 (index 5) leads to the decision on its left: no notch there.
	if got := look.bleedPt(5); got != 0 {
		t.Errorf("the step before a decision on the returning row bleeds %.0fpt into the diamond", got)
	}
	// Step 5 (index 4) leads to step 6, a chevron.
	if got := look.bleedPt(4); got != look.notchPt {
		t.Errorf("a returning step bleeds %.0fpt into the chevron after it, want its %.0fpt point", got, look.notchPt)
	}
	// Left to right the bleeding side is the notch, which tucks round a diamond.
	ltr := []ProcessFlowStep{{Label: "A"}, {Label: "B?", Type: "decision"}, {Label: "C"}}
	look = processFlowLookFor(ltr, nil, processFlowLayoutFor(ltr, nil), 200, true)
	if got := look.bleedPt(2); got != look.notchPt {
		t.Errorf("the step after a decision bleeds %.0fpt, want its %.0fpt notch", got, look.notchPt)
	}
}

func TestProcessFlowStyleValues(t *testing.T) {
	for _, name := range []string{"process-flow", "process-flow-compact"} {
		p, _ := Default().Get(name)
		vals := &ProcessFlowValues{Steps: pfSteps(4)}
		for _, style := range []string{"", "chevrons", "tinted", "solid"} {
			if err := p.Validate(vals, &ProcessFlowOverrides{Style: style}, nil); err != nil {
				t.Errorf("%s: style %q refused: %v", name, style, err)
			}
		}
		if err := p.Validate(vals, &ProcessFlowOverrides{Style: "boxes"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
			t.Errorf("%s: unknown style: err = %v", name, err)
		}
		// "chevrons" is the default spelled out.
		a, _ := p.Expand(processFlowChevronCtx(), vals, nil, nil)
		b, _ := p.Expand(processFlowChevronCtx(), vals, &ProcessFlowOverrides{Style: "chevrons"}, nil)
		if string(a.Rows[0].Cells[1].Shape.Text) != string(b.Rows[0].Cells[1].Shape.Text) || a.Gap != b.Gap {
			t.Errorf("%s: style chevrons differs from the default", name)
		}
	}
}
