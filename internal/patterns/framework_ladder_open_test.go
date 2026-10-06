package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func isRuleRow(row jsonschema.GridRowInput) bool {
	return len(row.Cells) == 1 && row.Cells[0].Shape != nil && len(row.Cells[0].Shape.Text) == 0 && row.MaxHeight > 0 && row.MaxHeight < 2
}

// go-slide-creator-rpz53: the default framework fills no cell. Rows are a
// label and open cards between full-width hairline rules; a ragged row still
// leaves its trailing columns empty.
func TestFrameworkGridOpenDefault(t *testing.T) {
	p := &frameworkGrid{}
	v := p.ExemplarValues().(*FrameworkGridValues)
	grid, err := p.Expand(testThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 2*len(v.Rows)-1 {
		t.Fatalf("rows = %d, want %d dimension rows with a rule between each", len(grid.Rows), len(v.Rows))
	}
	cols := 0
	for _, r := range v.Rows {
		cols = max(cols, len(r.Cards))
	}
	for i, row := range grid.Rows {
		if i%2 == 1 {
			if !isRuleRow(row) || row.Cells[0].ColSpan != cols+1 || row.MaxHeight != fgRulePt {
				t.Errorf("row %d should be a full-width %.2fpt rule: %+v", i, fgRulePt, row)
			}
			continue
		}
		src := v.Rows[i/2]
		if len(row.Cells) != cols+1 {
			t.Fatalf("row %d has %d cells, want label + %d columns", i, len(row.Cells), cols)
		}
		for j, c := range row.Cells {
			if j > len(src.Cards) {
				if c.Shape != nil {
					t.Errorf("row %d column %d: a ragged row's trailing space must stay empty", i, j)
				}
				continue
			}
			if string(c.Shape.Fill) != `"none"` {
				t.Errorf("row %d cell %d is filled: %s", i, j, c.Shape.Fill)
			}
		}
		// The label starts on the card titles' line.
		var label, card struct {
			VerticalAlign string  `json:"vertical_align"`
			InsetTop      float64 `json:"inset_top"`
		}
		_ = json.Unmarshal(row.Cells[0].Shape.Text, &label)
		_ = json.Unmarshal(row.Cells[1].Shape.Text, &card)
		if label != card || label.VerticalAlign != "t" {
			t.Errorf("row %d: label anchor %+v, card anchor %+v", i, label, card)
		}
	}

	// Never taller than the tiles plus the rules (the open cards are a
	// little wider, so they can wrap less).
	tiles, err := p.Expand(testThemeCtx(), v, &FrameworkGridOverrides{Style: "tiles"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rules := float64(len(v.Rows)-1) * fgRulePt
	if a, b := blockHeightPt(grid), blockHeightPt(tiles); a-rules > b+0.5 {
		t.Errorf("open block %.1fpt (rules %.1fpt) vs tiles block %.1fpt", a, rules, b)
	}
}

// A highlighted row is one unbroken band, the only filled area.
func TestFrameworkGridHighlight(t *testing.T) {
	p := &frameworkGrid{}
	v := p.ExemplarValues().(*FrameworkGridValues)
	v.Rows[1].Highlight = true // the two-card row of a three-column framework
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(grid.Rows); i += 2 {
		for j, c := range grid.Rows[i].Cells {
			filled := c.Shape != nil && string(c.Shape.Fill) != `"none"`
			if filled != (i == 2) {
				t.Errorf("row %d cell %d filled = %t", i/2, j, filled)
			}
		}
	}
	v.Rows[0].Highlight = true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one row") {
		t.Errorf("two highlighted rows must be refused, got %v", err)
	}
	if err := p.Validate(p.ExemplarValues(), &FrameworkGridOverrides{Style: "cards"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("want an overrides.style enum error, got %v", err)
	}
}

// The default ladder (go-slide-creator-kyvk5): each org a bold heading on its
// own rule, each pair one pale band across both columns with a block arrow in
// the gutter, names left-aligned; nothing is solid accent but the rules.
func TestDualOrgLadderBandDefault(t *testing.T) {
	p := &dualOrgLadder{}
	v := p.ExemplarValues().(*DualOrgLadderValues)
	for _, ovr := range []any{nil, &DualOrgLadderOverrides{Style: "open"}} {
		grid, err := p.Expand(testThemeCtx(), v, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 3 || cols[0] != cols[2] || cols[1] <= 0 {
			t.Fatalf("columns = %s, want [org A, gutter, org B]", grid.Columns)
		}
		if len(grid.Rows) != len(v.Rows)+2 {
			t.Fatalf("rows = %d, want heading + rule + %d pairs", len(grid.Rows), len(v.Rows))
		}
		heading, rule := grid.Rows[0].Cells, grid.Rows[1]
		for _, i := range []int{0, 2} {
			var text struct {
				InsetLeft  *float64 `json:"inset_left"`
				Paragraphs []struct {
					Bold  bool    `json:"bold"`
					Size  float64 `json:"size"`
					Align string  `json:"align"`
				} `json:"paragraphs"`
			}
			_ = json.Unmarshal(heading[i].Shape.Text, &text)
			if string(heading[i].Shape.Fill) != `"none"` || text.InsetLeft == nil || !text.Paragraphs[0].Bold || text.Paragraphs[0].Size != scaleLeadPt || text.Paragraphs[0].Align != "l" {
				t.Errorf("heading %d should be unfilled bold %vpt text on its names' edge: fill %s text %s", i, scaleLeadPt, heading[i].Shape.Fill, heading[i].Shape.Text)
			}
			if got := string(rule.Cells[i].Shape.Fill); got != `"accent1"` {
				t.Errorf("rule %d fill = %s", i, got)
			}
		}
		if heading[1].Shape != nil || rule.Cells[1].Shape != nil || rule.MaxHeight != dualOrgRulePt {
			t.Errorf("the gutter stays empty between the headings and the %vpt rules", dualOrgRulePt)
		}
		for i, row := range grid.Rows[2:] {
			if len(row.Cells) != 3 || row.Connector != nil {
				t.Fatalf("pair %d: %d cells, connector %v", i, len(row.Cells), row.Connector)
			}
			fill := string(row.Cells[0].Shape.Fill)
			for j, c := range row.Cells {
				if got := string(c.Shape.Fill); got != fill || got == `"none"` || strings.Contains(got, "accent") {
					t.Errorf("pair %d cell %d: fill %s, want one neutral band (%s)", i, j, got, fill)
				}
				if string(c.Shape.Line) == `"none"` {
					t.Errorf("pair %d cell %d: a band cell is outlined in its own colour to hide the seams", i, j)
				}
			}
			for _, j := range []int{0, 2} {
				if !strings.Contains(string(row.Cells[j].Shape.Text), `"align":"l"`) {
					t.Errorf("pair %d cell %d is not left-aligned", i, j)
				}
			}
			layers := row.Cells[1].Layers
			if len(layers) != 1 || layers[0].Shape.Geometry != "leftRightArrow" || strings.Contains(string(layers[0].Shape.Fill), "accent") {
				t.Errorf("pair %d: want one neutral pairing arrow in the gutter: %+v", i, layers)
			}
			if row.MinHeight <= 0 || row.MaxHeight < row.MinHeight {
				t.Errorf("pair %d: band height %v..%v", i, row.MinHeight, row.MaxHeight)
			}
		}
	}

	off := false
	v.ShowConnectors = &off
	grid, _ := p.Expand(testThemeCtx(), v, nil, nil)
	if gutter := grid.Rows[2].Cells[1]; gutter.Shape == nil || len(gutter.Layers) != 0 {
		t.Errorf("show_connectors=false keeps the band and drops the arrow: %+v", gutter)
	}
}

// The highlighted pair is the single accent: tinted band, accent edge and
// accent arrow. accent_b colours only org B's rule.
func TestDualOrgLadderBandHighlight(t *testing.T) {
	p := &dualOrgLadder{}
	v := p.ExemplarValues().(*DualOrgLadderValues)
	v.Rows[2].Highlight = true
	grid, err := p.Expand(testThemeCtx(), v, &DualOrgLadderOverrides{AccentB: "accent4"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := string(grid.Rows[1].Cells[0].Shape.Fill), string(grid.Rows[1].Cells[2].Shape.Fill); a != `"accent1"` || b != `"accent4"` {
		t.Errorf("rules = %s / %s", a, b)
	}
	for i, row := range grid.Rows[2:] {
		for j, c := range row.Cells {
			if tinted := strings.Contains(string(c.Shape.Fill), `"accent1"`); tinted != (i == 2) {
				t.Errorf("pair %d cell %d accent-tinted = %t", i, j, tinted)
			}
		}
		arrowAccent := string(row.Cells[1].Layers[0].Shape.Fill) == `"accent1"`
		edge := len(row.Cells[0].Layers) == 1 && string(row.Cells[0].Layers[0].Shape.Fill) == `"accent1"`
		if arrowAccent != (i == 2) || edge != (i == 2) {
			t.Errorf("pair %d: accent arrow %t, accent edge %t", i, arrowAccent, edge)
		}
	}
}

// Long names and roles on four pairs give up the band margin before any text
// shrinks: the four bands still fit the content area.
func TestDualOrgLadderBandTightensForWrappedText(t *testing.T) {
	p := &dualOrgLadder{}
	v := p.ExemplarValues().(*DualOrgLadderValues)
	ctx := testThemeCtx()
	_, areaH := sizingAreaPt(ctx)
	loose, _ := p.Expand(ctx, v, nil, nil)
	if strings.Contains(string(loose.Rows[2].Cells[0].Shape.Text), "inset_top") {
		t.Errorf("short entries keep the uniform text margin: %s", loose.Rows[2].Cells[0].Shape.Text)
	}
	for i := range v.Rows {
		v.Rows[i].ATitle = strings.Repeat("Programme lead ", 5)
		v.Rows[i].BNameField = strings.Repeat("Maria Lopez ", 5)
	}
	tight, _ := p.Expand(ctx, v, nil, nil)
	if !strings.Contains(string(tight.Rows[2].Cells[0].Shape.Text), "inset_top") {
		t.Errorf("wrapped entries should tighten the band margin: %s", tight.Rows[2].Cells[0].Shape.Text)
	}
	need := float64(len(tight.Rows)-1) * tight.RowGap
	for _, r := range tight.Rows {
		need += r.MinHeight
	}
	if need > areaH+0.5 {
		t.Errorf("four wrapped bands need %.1fpt of %.1fpt", need, areaH)
	}
}

// overrides.style "lines" keeps the earlier open look: tiles only for the two
// org headers, roles open entries joined by a pairing line in the gutter.
func TestDualOrgLadderLinesStyle(t *testing.T) {
	p := &dualOrgLadder{}
	v := p.ExemplarValues().(*DualOrgLadderValues)
	lines := &DualOrgLadderOverrides{Style: "lines"}
	grid, err := p.Expand(testThemeCtx(), v, lines, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 3 || cols[0] != cols[2] || cols[1] <= 0 {
		t.Fatalf("columns = %s, want [org A, link, org B]", grid.Columns)
	}
	header := grid.Rows[0].Cells
	if len(header) != 3 || header[1].Shape != nil || !strings.Contains(string(header[0].Shape.Fill), "accent1") || !strings.Contains(string(header[2].Shape.Fill), "accent1") {
		t.Errorf("header row should be two accent tiles around an empty gutter: %+v", header)
	}
	for i, row := range grid.Rows[1:] {
		if len(row.Cells) != 3 || row.Connector != nil {
			t.Fatalf("pair %d: %d cells, connector %v", i, len(row.Cells), row.Connector)
		}
		a, link, b := row.Cells[0], row.Cells[1], row.Cells[2]
		if string(a.Shape.Fill) != `"none"` || string(b.Shape.Fill) != `"none"` {
			t.Errorf("pair %d: role entries must be unfilled", i)
		}
		if link.Shape == nil || link.MaxHeight != dualOrgLinkLinePt || string(link.Shape.Fill) != `"accent1"` {
			t.Errorf("pair %d: missing pairing line: %+v", i, link)
		}
		if row.MinHeight <= 0 {
			t.Errorf("pair %d has no written-fit floor", i)
		}
	}

	off := false
	v.ShowConnectors = &off
	grid, _ = p.Expand(testThemeCtx(), v, lines, nil)
	if link := grid.Rows[1].Cells[1]; link.Shape != nil {
		t.Errorf("show_connectors=false still draws a pairing line: %+v", link)
	}
}

// In the lines style a highlighted pair is one band across both columns and
// the gutter.
func TestDualOrgLadderHighlight(t *testing.T) {
	p := &dualOrgLadder{}
	v := p.ExemplarValues().(*DualOrgLadderValues)
	v.Rows[2].Highlight = true
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), v, &DualOrgLadderOverrides{Style: "lines"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range grid.Rows[1:] {
		for j, c := range row.Cells {
			tinted := c.Shape != nil && strings.Contains(string(c.Shape.Fill), "lumMod")
			if tinted != (i == 2) {
				t.Errorf("pair %d cell %d tinted = %t", i, j, tinted)
			}
		}
	}
	v.Rows[0].Highlight = true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one pair") {
		t.Errorf("two highlighted pairs must be refused, got %v", err)
	}
	if err := p.Validate(p.ExemplarValues(), &DualOrgLadderOverrides{Style: "cards"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Errorf("want an overrides.style enum error, got %v", err)
	}
}
