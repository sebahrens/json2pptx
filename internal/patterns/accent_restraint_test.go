package patterns

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// Restrained accent defaults (go-slide-creator-fl11f): each child bead's
// acceptance criterion, asserted on the expanded grid.

// restraintCtx is a full theme over a 16:9 content area below a title.
func restraintCtx() ExpandContext {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{X: 457200, Y: 1400000, Width: 11277600, Height: 4700000}
	return ctx
}

func expandFor(t *testing.T, name string, ctx ExpandContext, values, overrides any) *jsonschema.ShapeGridInput {
	t.Helper()
	p, ok := Default().Get(name)
	if !ok {
		t.Fatalf("%s not registered", name)
	}
	if err := p.Validate(values, overrides, nil); err != nil {
		t.Fatalf("%s: validate: %v", name, err)
	}
	grid, err := p.Expand(ctx, values, overrides, nil)
	if err != nil {
		t.Fatalf("%s: expand: %v", name, err)
	}
	return grid
}

// solidAccentCells counts shape cells (nested grids included) filled with an
// unmodified accent slot.
func solidAccentCells(grid *jsonschema.ShapeGridInput) int {
	n := 0
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if c == nil {
				continue
			}
			if c.Grid != nil {
				n += solidAccentCells(c.Grid)
			}
			if c.Shape == nil {
				continue
			}
			if tone, ok := opaqueFillTone(c.Shape.Fill); ok && isAccentSlot(tone.Color) && tone == (fillTone{Color: tone.Color}) {
				n++
			}
		}
	}
	return n
}

func roadmapValues() *RoadmapPhasedValues {
	return (&roadmapPhased{}).ExemplarValues().(*RoadmapPhasedValues)
}

// go-slide-creator-k8x1p: activities are a light tint with dark text, period
// headers carry an accent rule, and style "solid" fills headers and
// activities with the accent. go-slide-creator-4a0sm made the activities bars
// on a time axis; the one filled header is the current period.
func TestRoadmapPhased_TintedByDefault(t *testing.T) {
	vals := roadmapValues()
	vals.CurrentPhase = ""
	grid := expandFor(t, "roadmap-phased", restraintCtx(), vals, nil)
	if n := solidAccentCells(grid); n != 0 {
		t.Errorf("default roadmap-phased has %d solid accent cells, want 0", n)
	}
	for i, c := range grid.Rows[0].Cells[1:] {
		if c.AccentBar == nil || c.AccentBar.Color != "accent1" || c.AccentBar.Position != "bottom" {
			t.Errorf("phase header %d: accent bar = %+v, want an accent1 axis segment under it", i, c.AccentBar)
		}
	}
	item := grid.Rows[1].Cells[1].Shape
	if !strings.Contains(string(item.Fill), "lumMod") || strings.Contains(string(item.Text), `"color":"lt1"`) {
		t.Errorf("bar fill/text = %s / %s, want a light tint with dark text", item.Fill, item.Text)
	}

	current := expandFor(t, "roadmap-phased", restraintCtx(), roadmapValues(), nil)
	if n := solidAccentCells(current); n != 1 {
		t.Errorf("current_phase set: %d solid accent cells, want the current period's header alone", n)
	}

	solid := expandFor(t, "roadmap-phased", restraintCtx(), vals, &RoadmapPhasedOverrides{Style: "solid"})
	if n, want := solidAccentCells(solid), 4+6; n != want {
		t.Errorf("style solid: %d solid accent cells, want the 4 headers and 6 bars (%d)", n, want)
	}
	p, _ := Default().Get("roadmap-phased")
	if err := p.Validate(roadmapValues(), &RoadmapPhasedOverrides{Style: "loud"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("unknown style: err = %v, want an overrides.style error", err)
	}
}

func processSteps() []ProcessFlowStep {
	return []ProcessFlowStep{
		{Label: "Request"}, {Label: "Review", Type: "decision"}, {Label: "Approve"}, {Label: "Deploy"},
	}
}

// go-slide-creator-xb06p: at most one solid accent shape (the lone decision,
// or a highlighted step), accent connectors, content-sized height capped at
// 30% of the content area, and the compact flow visibly shorter.
func TestProcessFlow_RestrainedByDefault(t *testing.T) {
	ctx := restraintCtx()
	_, contentH := contentAreaPt(ctx)
	full := expandFor(t, "process-flow", ctx, &ProcessFlowValues{Steps: processSteps()}, nil)
	row := full.Rows[0]
	if n := solidAccentCells(full); n != 1 {
		t.Errorf("default process-flow has %d solid accent shapes, want 1 (the decision)", n)
	}
	if got := string(row.Cells[1].Shape.Fill); got != `"accent1"` {
		t.Errorf("lone decision fill = %s, want the accent", got)
	}
	// The default look draws no connector; the flowchart look's are accent.
	if row.Connector != nil {
		t.Errorf("connector = %+v, want none in the chevron look", row.Connector)
	}
	flowchart := expandFor(t, "process-flow", ctx, &ProcessFlowValues{Steps: processSteps()}, &ProcessFlowOverrides{Style: processFlowStyleTinted})
	if c := flowchart.Rows[0].Connector; c == nil || c.Color != "accent1" {
		t.Errorf("flowchart connector = %+v, want accent1 arrows", c)
	}
	if n := solidAccentCells(flowchart); n != 1 {
		t.Errorf("flowchart process-flow has %d solid accent shapes, want 1 (the decision)", n)
	}
	if row.MaxHeight <= 0 || row.MaxHeight > math.Round(contentH*processFlowMaxHeightFrac) {
		t.Errorf("step height = %.0fpt, want content-sized within %.0f%% of %.0fpt", row.MaxHeight, processFlowMaxHeightFrac*100, contentH)
	}
	// 150px at 80dpi is 135pt: the review's slabs were taller than that.
	if row.MaxHeight >= 135 {
		t.Errorf("step height = %.0fpt, want below the 135pt slabs", row.MaxHeight)
	}

	compact := expandFor(t, "process-flow-compact", ctx, &ProcessFlowValues{Steps: processSteps()}, nil)
	if compact.Bounds != nil {
		t.Fatalf("process-flow-compact sets a bounds box %+v; its band is a content-sized row", compact.Bounds)
	}
	if compactH := compact.Rows[0].MaxHeight; compactH <= 0 || compactH >= row.MaxHeight {
		t.Errorf("compact band %.0fpt is not shorter than process-flow's %.0fpt", compactH, row.MaxHeight)
	}

	// A highlighted step takes the accent; the decision is then outlined.
	steps := processSteps()
	steps[3].Highlight = true
	hl := expandFor(t, "process-flow", ctx, &ProcessFlowValues{Steps: steps}, nil)
	if n := solidAccentCells(hl); n != 1 || string(hl.Rows[0].Cells[3].Shape.Fill) != `"accent1"` {
		t.Errorf("highlight: %d solid shapes, step 3 fill %s; want only the highlighted step", n, hl.Rows[0].Cells[3].Shape.Fill)
	}
	if line := string(hl.Rows[0].Cells[1].Shape.Line); !strings.Contains(line, "accent1") {
		t.Errorf("decision beside a highlight: line = %s, want an accent outline", line)
	}

	// Two decisions: neither is the one emphasis.
	steps = processSteps()
	steps[2].Type = "decision"
	two := expandFor(t, "process-flow", ctx, &ProcessFlowValues{Steps: steps}, nil)
	if n := solidAccentCells(two); n != 0 {
		t.Errorf("two decisions: %d solid accent shapes, want 0", n)
	}

	solid := expandFor(t, "process-flow", ctx, &ProcessFlowValues{Steps: processSteps()}, &ProcessFlowOverrides{Style: "solid"})
	if n := solidAccentCells(solid); n != 4 {
		t.Errorf("style solid: %d solid accent shapes, want 4", n)
	}

	p, _ := Default().Get("process-flow")
	steps = processSteps()
	steps[0].Highlight, steps[2].Highlight = true, true
	if err := p.Validate(&ProcessFlowValues{Steps: steps}, nil, nil); err == nil || !strings.Contains(err.Error(), "steps[2].highlight") {
		t.Errorf("two highlights: err = %v, want steps[2].highlight rejected", err)
	}
}

func heroValues() *HeroDetailValues {
	return &HeroDetailValues{
		Hero: HeroDetailHero{Value: "EUR 2.4B", Label: "Addressable AI services market by FY27"},
		Details: []HeroDetailItem{
			{Title: "Growth", Body: "42% CAGR driven by enterprise adoption of agentic workflows across every region"},
			{Title: "Lead segment", Body: "Financial services"},
			{Title: "Outlook", Body: "EUR 5.1B by FY30"},
		},
	}
}

// go-slide-creator-19pp9 / go-slide-creator-ppcfn: the default style is
// minimal (no accent-filled cards) and every minimal card is top-anchored,
// so the titles share one line whatever the body length.
func TestHeroDetail_MinimalDefaultTopAnchored(t *testing.T) {
	grid := expandFor(t, "hero-detail", restraintCtx(), heroValues(), nil)
	if n := solidAccentCells(grid); n != 0 {
		t.Errorf("default hero-detail has %d solid accent cells, want 0", n)
	}
	for i, c := range grid.Rows[1].Cells {
		var text struct {
			VerticalAlign string `json:"vertical_align"`
		}
		if err := json.Unmarshal(c.Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		if text.VerticalAlign != "t" {
			t.Errorf("detail %d anchor = %q, want top so the titles align", i, text.VerticalAlign)
		}
		if c.AccentBar == nil {
			t.Errorf("detail %d lost its accent rule", i)
		}
	}
	cards := expandFor(t, "hero-detail", restraintCtx(), heroValues(), &HeroDetailOverrides{Style: "cards"})
	if n := solidAccentCells(cards); n != 3 {
		t.Errorf("style cards: %d accent cards, want 3", n)
	}
}

// go-slide-creator-061ag: tinted kpi-inline cells are a neutral tint under a
// thin accent rule; style "solid" restores the accent blocks. Neither is the
// default for plain cells (go-slide-creator-8zles): that is the open strip.
func TestKPIInline_TintedStyle(t *testing.T) {
	vals := (&kpiInline{}).ExemplarValues().(*KPINupValues)
	if n := solidAccentCells(expandFor(t, "kpi-inline", restraintCtx(), vals, nil)); n != 0 {
		t.Errorf("default kpi-inline has %d solid accent cells, want 0", n)
	}
	grid := expandFor(t, "kpi-inline", restraintCtx(), vals, &KPIInlineOverrides{Style: "tinted"})
	if n := solidAccentCells(grid); n != 0 {
		t.Errorf("tinted kpi-inline has %d solid accent cells, want 0", n)
	}
	for i, c := range grid.Rows[0].Cells {
		if c.AccentBar == nil || c.AccentBar.Position != "top" {
			t.Errorf("cell %d accent bar = %+v, want a top rule", i, c.AccentBar)
		}
		if strings.Contains(string(c.Shape.Text), `"color":"lt1"`) {
			t.Errorf("cell %d keeps lt1 text on the tint: %s", i, c.Shape.Text)
		}
	}
	solid := expandFor(t, "kpi-inline", restraintCtx(), vals, &KPIInlineOverrides{Style: "solid"})
	if n := solidAccentCells(solid); n != len(*vals) {
		t.Errorf("style solid: %d accent cells, want %d", n, len(*vals))
	}
}

// go-slide-creator-knue6: date labels share the descriptions' left
// alignment.
func TestPhaseRoadmap_DatesAlignWithDescriptions(t *testing.T) {
	v := validPhaseRoadmapValues()
	grid := expandFor(t, "phase-roadmap", restraintCtx(), v, nil)
	dates, descs := grid.Rows[2], grid.Rows[3]
	for i := range dates.Cells {
		if !strings.Contains(string(dates.Cells[i].Shape.Text), `"align":"l"`) || !strings.Contains(string(descs.Cells[i].Shape.Text), `"align":"l"`) {
			t.Errorf("column %d: date %s / description %s, want both left-aligned", i, dates.Cells[i].Shape.Text, descs.Cells[i].Shape.Text)
		}
	}
}

func stylishItems() *StylishPanelsValues {
	return (&stylishPanels{}).ExemplarValues().(*StylishPanelsValues)
}

// go-slide-creator-95tp7: the panel body is a visible tint (not lt1), the
// ribbon is its written need without the extra band padding, and the default
// ribbon is the structural dark tone rather than a row of solid accents.
func TestStylishPanels_VisibleBodiesAndRestrainedRibbons(t *testing.T) {
	ctx := restraintCtx()
	ribbon := &StylishPanelsOverrides{Style: "ribbon"}
	grid := expandFor(t, "stylish-panels", ctx, stylishItems(), ribbon)
	if n := solidAccentCells(grid); n != 0 {
		t.Errorf("ribbon stylish-panels has %d solid accent cells, want 0", n)
	}
	for i, c := range grid.Rows[1].Cells {
		if isPageColor(strings.Trim(string(c.Shape.Fill), `"`)) {
			t.Errorf("panel body %d fill = %s, invisible on white paper", i, c.Shape.Fill)
		}
	}
	fit := stylishPanelsFit(ctx, *stylishItems(), &StylishPanelsOverrides{})
	if got := grid.Rows[0].MaxHeight; got > math.Max(fit.headerPt, 1)+headerBandPadPt-1 {
		t.Errorf("ribbon height = %.0fpt, want its written need %.0fpt (no band padding)", got, fit.headerPt)
	}
	accent := expandFor(t, "stylish-panels", ctx, stylishItems(), &StylishPanelsOverrides{Ribbon: "accent"})
	if n := solidAccentCells(accent); n != len(*stylishItems()) {
		t.Errorf("ribbon accent: %d accent ribbons, want %d", n, len(*stylishItems()))
	}
}

// go-slide-creator-3nsll: tiers are content-sized (capped near 20% of the
// content height) and the stack is centred, not stretched over the slide.
func TestArchStack_TiersContentSized(t *testing.T) {
	ctx := restraintCtx()
	_, contentH := contentAreaPt(ctx)
	// The description layout: a stack with components sizes its bands to the
	// blocks instead (TestArchStack_ComponentsRenderAsBlocks).
	vals := &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Presentation", Description: "React, Next.js"},
			{Label: "API Gateway", Description: "Kong, rate limiting"},
			{Label: "Business Logic", Description: "Go services"},
			{Label: "Data Layer", Description: "PostgreSQL, Redis"},
		},
		SideRails: []string{"Security", "Monitoring"},
	}
	grid := expandFor(t, "arch-stack", ctx, vals, nil)
	if grid.VerticalAlign != GridVerticalAlignDefault {
		t.Errorf("vertical_align = %q, want the stack centred", grid.VerticalAlign)
	}
	for i, row := range grid.Rows {
		if row.MaxHeight <= 0 || row.MaxHeight > math.Ceil(contentH*archStackTierMaxHeightFrac) {
			t.Errorf("tier %d max_height = %.0fpt, want content-sized within %.0f%% of %.0fpt", i, row.MaxHeight, archStackTierMaxHeightFrac*100, contentH)
		}
	}
	// A long description still grows its tier past the cap.
	long := &ArchStackValues{Tiers: []ArchStackTier{
		{Label: "Presentation", Description: "React, Next.js and a shared design system serving web, mobile and partner channels with one component library"},
		{Label: "API", Description: "Kong"}, {Label: "Logic", Description: "Go"},
	}}
	grown := expandFor(t, "arch-stack", ctx, long, &ArchStackOverrides{BodySize: 28})
	if grown.Rows[0].MaxHeight <= math.Ceil(contentH*archStackTierMaxHeightFrac) {
		t.Errorf("long tier max_height = %.0fpt, want it to grow past the cap", grown.Rows[0].MaxHeight)
	}
}

// go-slide-creator-x0b82: a 2x2 short-item comparison has a header band at
// its written need and body rows within contentStretch of theirs.
func TestComparison2col_ContentSizedRows(t *testing.T) {
	ctx := restraintCtx()
	vals := &Comparison2colValues{}
	if err := json.Unmarshal([]byte(`{"headers":["Pros","Cons"],"rows":[{"left":"Fast","right":"Expensive"},{"left":"Reliable","right":"Complex"}]}`), vals); err != nil {
		t.Fatal(err)
	}
	tiles := &Comparison2colOverrides{Style: "tiles"}
	grid := expandFor(t, "comparison-2col", ctx, vals, tiles)
	plan := comparisonLayout(ctx, vals, tiles, nil)
	if !plan.fits || !plan.header {
		t.Fatalf("exemplar plan fits=%v header=%v", plan.fits, plan.header)
	}
	header := grid.Rows[0]
	if header.MaxHeight != math.Ceil(plan.needs[0]) || header.MinHeight != header.MaxHeight {
		t.Errorf("header row = [%.0f, %.0f]pt, want exactly its %.0fpt need", header.MinHeight, header.MaxHeight, plan.needs[0])
	}
	for i := 1; i < len(grid.Rows); i++ {
		r := grid.Rows[i]
		if r.MaxHeight < math.Ceil(plan.needs[i]) || r.MaxHeight > math.Ceil(plan.needs[i]*comparisonStretchMax) {
			t.Errorf("body row %d = %.0fpt, want within [%.0f, %.0f]", i, r.MaxHeight, plan.needs[i], plan.needs[i]*comparisonStretchMax)
		}
		if r.Flex != 0 {
			t.Errorf("body row %d still flexes (%v)", i, r.Flex)
		}
		if header.MaxHeight >= r.MaxHeight {
			t.Errorf("header %.0fpt is as tall as body row %d (%.0fpt)", header.MaxHeight, i, r.MaxHeight)
		}
	}
}
