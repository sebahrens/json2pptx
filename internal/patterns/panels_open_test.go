package patterns

import (
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func blockHeightPt(g *jsonschema.ShapeGridInput) float64 {
	gap := g.RowGap
	if gap == 0 {
		gap = g.Gap
	}
	h := float64(len(g.Rows)-1) * gap
	for _, r := range g.Rows {
		h += r.MaxHeight
	}
	return h
}

// go-slide-creator-xvpu2: a default stylish-panels column is a heading over
// an accent rule and an open bullet list — no filled header, no body tile.
func TestStylishPanelsOpenDefault(t *testing.T) {
	p := &stylishPanels{}
	ctx := testThemeCtx()
	for _, n := range []int{3, 4, 5} {
		items := make(StylishPanelsValues, n)
		for i := range items {
			items[i] = StylishPanelsItem{Title: "Pillar", Body: []string{"First point", "Second point"}}
		}
		grid, err := p.Expand(ctx, &items, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("n=%d: want heading, rule and body rows, got %d", n, len(grid.Rows))
		}
		for i := 0; i < n; i++ {
			head, rule, body := grid.Rows[0].Cells[i].Shape, grid.Rows[1].Cells[i].Shape, grid.Rows[2].Cells[i].Shape
			if string(head.Fill) != `"none"` || string(body.Fill) != `"none"` {
				t.Errorf("n=%d column %d: heading fill %s, body fill %s; want none", n, i, head.Fill, body.Fill)
			}
			if string(rule.Fill) != `"accent1"` || len(rule.Text) != 0 {
				t.Errorf("n=%d column %d: rule fill %s, want the accent", n, i, rule.Fill)
			}
			if !strings.Contains(string(head.Text), `"align":"l"`) || !strings.Contains(string(head.Text), `"bold":true`) {
				t.Errorf("n=%d column %d: heading should be bold and left-aligned: %s", n, i, head.Text)
			}
		}
		if got := grid.Rows[1].MaxHeight; got != stylishPanelsRulePt {
			t.Errorf("n=%d: rule row %.2fpt", n, got)
		}

		// The open block is as tall as the ribbon block: the rule and its two
		// half gaps replace the one gap between ribbon and tile.
		ribbon, err := p.Expand(ctx, &items, &StylishPanelsOverrides{Style: "ribbon"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if a, b := blockHeightPt(grid), blockHeightPt(ribbon); math.Abs(a-b) > 0.5 {
			t.Errorf("n=%d: open block %.1fpt vs ribbon block %.1fpt", n, a, b)
		}
	}
}

// One highlighted column carries the only solid fill.
func TestStylishPanelsHighlight(t *testing.T) {
	p := &stylishPanels{}
	items := StylishPanelsValues{
		{Title: "A", Body: []string{"x"}},
		{Title: "B", Body: []string{"y"}, Highlight: true},
		{Title: "C", Body: []string{"z"}},
	}
	if err := p.Validate(&items, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), &items, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range grid.Rows[0].Cells {
		if filled := strings.Contains(string(c.Shape.Fill), "accent1"); filled != (i == 1) {
			t.Errorf("heading %d accent fill = %t", i, filled)
		}
	}
	for i, c := range grid.Rows[2].Cells {
		if string(c.Shape.Fill) != `"none"` {
			t.Errorf("body %d is filled: %s", i, c.Shape.Fill)
		}
	}
	items[2].Highlight = true
	if err := p.Validate(&items, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one column") {
		t.Errorf("two highlighted columns must be refused, got %v", err)
	}
}

// overrides.ribbon is a ribbon-style option: setting it keeps the ribbons an
// existing input asked for; style is validated.
func TestStylishPanelsStyleSelection(t *testing.T) {
	p := &stylishPanels{}
	items := p.ExemplarValues().(*StylishPanelsValues)
	for _, tc := range []struct {
		ovr  *StylishPanelsOverrides
		rows int
	}{
		{nil, 3},
		{&StylishPanelsOverrides{Style: "open"}, 3},
		{&StylishPanelsOverrides{Style: "ribbon"}, 2},
		{&StylishPanelsOverrides{Ribbon: "accent"}, 2},
		{&StylishPanelsOverrides{Ribbon: "dark"}, 2},
		{&StylishPanelsOverrides{Style: "open", Ribbon: "accent"}, 3},
	} {
		var o any
		if tc.ovr != nil {
			o = tc.ovr
		}
		grid, err := p.Expand(testThemeCtx(), items, o, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(grid.Rows) != tc.rows {
			t.Errorf("%+v: %d rows, want %d", tc.ovr, len(grid.Rows), tc.rows)
		}
	}
	if err := p.Validate(items, &StylishPanelsOverrides{Style: "cards"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("want an overrides.style enum error, got %v", err)
	}
}

func openBeforeAfterValues() *BeforeAfterValues {
	return &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Current State", Items: []string{"Manual process", "3-day turnaround"}},
		After:  BeforeAfterColumn{Header: "Future State", Items: []string{"Automated", "Same-day"}},
	}
}

// Both before-after variants default to open states: headings over rules (a
// neutral rule for before, the accent rule for after), open bullet lists, and
// the chevron kept across the whole block.
func TestBeforeAfterOpenDefault(t *testing.T) {
	for _, pat := range []Pattern{&beforeAfter{}, &beforeAfterCompact{}} {
		grid, err := pat.Expand(testThemeCtx(), openBeforeAfterValues(), nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", pat.Name(), err)
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("%s: want heading, rule and body rows, got %d", pat.Name(), len(grid.Rows))
		}
		head, rules, body := grid.Rows[0].Cells, grid.Rows[1].Cells, grid.Rows[2].Cells
		if len(head) != 3 || len(rules) != 2 || len(body) != 2 {
			t.Fatalf("%s: cells %d / %d / %d", pat.Name(), len(head), len(rules), len(body))
		}
		chevron := head[1]
		if chevron.Shape.Geometry != "chevron" || chevron.RowSpan != 3 || chevron.MaxHeight != beforeAfterChevronPt {
			t.Errorf("%s: the transition chevron must span the block: %+v", pat.Name(), chevron)
		}
		for _, c := range []*jsonschema.GridCellInput{head[0], head[2], body[0], body[1]} {
			if string(c.Shape.Fill) != `"none"` {
				t.Errorf("%s: open cell is filled: %s", pat.Name(), c.Shape.Fill)
			}
		}
		if !strings.Contains(string(rules[0].Shape.Fill), `"dk1"`) || string(rules[1].Shape.Fill) != `"accent1"` {
			t.Errorf("%s: rules = %s / %s, want neutral before and accent after", pat.Name(), rules[0].Shape.Fill, rules[1].Shape.Fill)
		}

		panels, err := pat.Expand(testThemeCtx(), openBeforeAfterValues(), &BeforeAfterOverrides{Style: "panels"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(panels.Rows) != 2 {
			t.Errorf("%s: panels style has %d rows, want 2", pat.Name(), len(panels.Rows))
		}
		if a, b := blockHeightPt(grid), blockHeightPt(panels); math.Abs(a-b) > 0.5 {
			t.Errorf("%s: open block %.1fpt vs panels block %.1fpt", pat.Name(), a, b)
		}
	}
}

// overrides.emphasis fills one state's heading: the only solid fill.
func TestBeforeAfterEmphasis(t *testing.T) {
	p := &beforeAfter{}
	ovr := &BeforeAfterOverrides{Emphasis: "after"}
	if err := p.Validate(openBeforeAfterValues(), ovr, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), openBeforeAfterValues(), ovr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"none"` {
		t.Errorf("before heading fill = %s", got)
	}
	if got := string(grid.Rows[0].Cells[2].Shape.Fill); !strings.Contains(got, "accent1") {
		t.Errorf("after heading fill = %s, want the accent", got)
	}
	for _, bad := range []*BeforeAfterOverrides{{Emphasis: "both"}, {Style: "cards"}} {
		if err := p.Validate(openBeforeAfterValues(), bad, nil); err == nil {
			t.Errorf("%+v must be refused", bad)
		}
		if err := (&beforeAfterCompact{}).Validate(&BeforeAfterValues{
			Before: BeforeAfterColumn{Header: "Now", Items: []string{"a"}},
			After:  BeforeAfterColumn{Header: "Next", Items: []string{"b"}},
		}, bad, nil); err == nil {
			t.Errorf("compact: %+v must be refused", bad)
		}
	}
}
