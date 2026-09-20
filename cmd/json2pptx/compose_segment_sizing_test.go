package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestComposeContentSizedLeavesUseAllocatedSegments(t *testing.T) {
	ctx := patterns.ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340},
	}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["1 | A","2 | B","3 | C"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	full, _, err := expandPattern(&kpi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	fullHeight := full.Rows[0].MaxHeight
	for _, tc := range []struct {
		name     string
		compose  *ComposeInput
		maxShare float64
	}{
		{"explicit", &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{SizePct: 40, Pattern: kpi}, {SizePct: 60, Pattern: hero}}}, 0.40},
		{"smart", &ComposeInput{Direction: "vertical", SmartCompose: true, Segments: []SegmentInput{{Pattern: kpi}, {Pattern: hero}}}, 0.75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merged, _, err := expandCompose(tc.compose, ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			got := merged.Rows[0].MaxHeight
			allowed := fullHeight*tc.maxShare + 2
			if tc.maxShare < 0.5 {
				allowed = float64(ctx.LayoutBounds.Height)/12700*tc.maxShare + 2
			}
			if got >= fullHeight || got > allowed {
				t.Errorf("KPI row max height %.1fpt does not fit its %.0f%% vertical segment (full-slide %.1fpt)", got, tc.maxShare*100, fullHeight)
			}
		})
	}
}

func TestComposeHorizontalLeafTextFitsSegmentWidth(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Bookings","$987.65M | Revenue","$555.55M | Pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	full, _, err := expandPattern(&kpi, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if firstCellParagraphSize(t, merged) >= firstCellParagraphSize(t, full) {
		t.Errorf("KPI value font did not shrink to fit the 50%% horizontal segment")
	}
}

func TestComposeHorizontalKeepsCompactCardBesideFullHeightHero(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["1 | A","2 | B","3 | C"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if merged.Rows[0].Cells[0].MaxHeight <= 0 {
		t.Fatal("horizontal merge lost the KPI row height cap")
	}
	if merged.Rows[0].Cells[3].MaxHeight != 0 {
		t.Fatal("KPI height cap leaked into the hero segment")
	}
	result, err := resolveShapeGrid(merged, pptx.NewShapeIDAllocator(nil), &pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}, nil, ctx.SlideWidth, ctx.SlideHeight, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) < 4 {
		t.Fatalf("expected three KPI cards and hero, got %d cells", len(result.Cells))
	}
	card, full := result.Cells[0].Bounds, result.Cells[3].Bounds
	if card.CY >= full.CY/2 {
		t.Errorf("KPI card height %.1fpt is not compact beside %.1fpt hero", float64(card.CY)/12700, float64(full.CY)/12700)
	}
	if card.Y <= full.Y || card.Y+card.CY >= full.Y+full.CY {
		t.Errorf("KPI card is not centered within its segment: card=%+v hero=%+v", card, full)
	}
}

func TestNestedComposeInheritsOuterSegmentBounds(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 838200, Y: 1500000, Width: 10515600, Height: 3913340}}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Bookings","$987.65M | Revenue","$555.55M | Pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}
	inner := &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}
	fullInner, _, err := expandCompose(inner, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	outer := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Compose: inner}, {SizePct: 50, Pattern: hero}}}
	merged, _, err := expandCompose(outer, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if firstCellParagraphSize(t, merged) >= firstCellParagraphSize(t, fullInner) {
		t.Errorf("nested KPI value font did not shrink to fit its outer 50%% width allocation")
	}
}

func firstCellParagraphSize(t *testing.T, grid *ShapeGridInput) float64 {
	t.Helper()
	var text struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &text); err != nil {
		t.Fatal(err)
	}
	if len(text.Paragraphs) == 0 {
		t.Fatal("cell has no text paragraphs")
	}
	return text.Paragraphs[0].Size
}
