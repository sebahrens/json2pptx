package patterns

import (
	"strings"
	"testing"
)

// TestStrategyHouseTonesAreTheHierarchy pins go-slide-creator-pyeba: the roof
// is the only solid accent; a pillar with bullets is a cap in the accent's
// content swatch (a layer carrying the bold title) over a shaft on the panel
// surface carrying the bullets; the upper foundation level is the content
// swatch and the base — the lowest level — the neutral dark.
func TestStrategyHouseTonesAreTheHierarchy(t *testing.T) {
	ctx := testThemeCtx()
	accent := ctx.DefaultAccent()
	v := &StrategyHouseValues{
		Objective: "Become the trusted platform for global commerce",
		Pillars:   housePillars(4),
		FoundationLayers: []StrategyHouseLayer{
			{"One operating model in every market"},
			{"People", "Technology", "Data"},
		},
	}
	grid := expandHouse(t, ctx, v, nil)
	if len(grid.Rows) != 4 {
		t.Fatalf("rows = %d, want roof, pillars and two foundation levels", len(grid.Rows))
	}
	content := string(tonalContent(ctx, accent).fillJSON())
	baseTone, baseInk := tonalBadge(ctx)

	for i, c := range grid.Rows[1].Cells {
		if got := string(c.Shape.Fill); got != string(tonalPanel(ctx, accent).fillJSON()) {
			t.Errorf("pillar %d: shaft fill %s, want the panel surface", i, got)
		}
		if strings.Contains(string(c.Shape.Text), v.Pillars[i].Title) || !strings.Contains(string(c.Shape.Text), v.Pillars[i].Body[0]) {
			t.Errorf("pillar %d: the shaft carries the bullets and not the title: %s", i, c.Shape.Text)
		}
		if len(c.Layers) != 1 || c.Layers[0].Name != HouseCapLayerName {
			t.Fatalf("pillar %d: want one cap layer, got %d", i, len(c.Layers))
		}
		cp := c.Layers[0]
		if got := string(cp.Shape.Fill); got != content {
			t.Errorf("pillar %d: cap fill %s, want the accent's content swatch %s", i, got, content)
		}
		if text := string(cp.Shape.Text); !strings.Contains(text, v.Pillars[i].Title) || !strings.Contains(text, `"bold":true`) {
			t.Errorf("pillar %d: the cap carries the bold title: %s", i, text)
		}
		if cp.Frame.X != 0 || cp.Frame.Y != 0 || cp.Frame.W != 1 || cp.Frame.H <= 0 || cp.Frame.H >= 0.6 {
			t.Errorf("pillar %d: cap frame %+v, want a short band across the top", i, cp.Frame)
		}
	}
	if got := string(grid.Rows[2].Cells[0].Shape.Fill); got != content {
		t.Errorf("upper foundation level fill %s, want the content swatch", got)
	}
	for i, c := range grid.Rows[3].Cells {
		if got := string(c.Shape.Fill); got != string(baseTone.fillJSON()) {
			t.Errorf("base cell %d: fill %s, want the neutral dark %s", i, got, baseTone.fillJSON())
		}
		if !strings.Contains(string(c.Shape.Text), `"color":"`+baseInk+`"`) {
			t.Errorf("base cell %d: text is not in the measured ink %q: %s", i, baseInk, c.Shape.Text)
		}
	}
	// Only the roof is solid accent.
	solid := 0
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if string(c.Shape.Fill) == `"`+accent+`"` {
				solid++
			}
		}
	}
	if solid != 1 || grid.Rows[0].Cells[0].Shape.Geometry != "upArrow" {
		t.Errorf("solid accent shapes = %d, want the roof alone", solid)
	}

	// A row of titles alone draws each pillar as one tinted block.
	bare := *v
	bare.Pillars = []StrategyHousePillar{{Title: "A"}, {Title: "B"}, {Title: "C"}}
	for i, c := range expandHouse(t, ctx, &bare, nil).Rows[1].Cells {
		if len(c.Layers) != 0 || string(c.Shape.Fill) != content {
			t.Errorf("title-only pillar %d: fill %s layers %d, want one tinted block", i, c.Shape.Fill, len(c.Layers))
		}
	}
}

// TestHouseGrowsItsTypeWhereItFits: a house with height to spare takes one
// step up the type scale; an authored size, a short region or a word that
// would outgrow its pillar keeps the written sizes.
func TestHouseGrowsItsTypeWhereItFits(t *testing.T) {
	ctx := testThemeCtx()
	v := &StrategyHouseValues{
		Objective:  "Become the trusted platform for global commerce",
		Pillars:    housePillars(3),
		Foundation: "One operating model in every market",
	}
	w, h := contentAreaPt(ctx)
	bullet := func(l *HouseLayout) string { return string(l.Grid.Rows[1].Cells[0].Shape.Text) }

	grown, err := BuildHouse(v.model(), v.style(ctx, &StrategyHouseOverrides{}, nil), w, h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bullet(grown), `"size":14`) || grown.RoofFlattened || grown.HeightPt > h+1 {
		t.Errorf("a three-pillar house in a %.0fpt area should set its bullets at 14pt and fit: %s (height %.0f)", h, bullet(grown), grown.HeightPt)
	}
	authored, err := BuildHouse(v.model(), v.style(ctx, &StrategyHouseOverrides{BodySize: 12}, nil), w, h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bullet(authored), `"size":12`) {
		t.Errorf("an authored body_size must be kept: %s", bullet(authored))
	}
	short, err := BuildHouse(v.model(), v.style(ctx, &StrategyHouseOverrides{}, nil), w, 230)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bullet(short), `"size":12`) {
		t.Errorf("a house short of height keeps its written sizes: %s", bullet(short))
	}
	long := *v
	long.Pillars = []StrategyHousePillar{
		{Title: "Internationalisation", Body: []string{"Privacy"}}, {Title: "B", Body: []string{"x"}}, {Title: "C", Body: []string{"x"}},
		{Title: "D", Body: []string{"x"}}, {Title: "E", Body: []string{"x"}},
	}
	narrow, err := BuildHouse(long.model(), long.style(ctx, &StrategyHouseOverrides{}, nil), w/2, h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bullet(narrow), `"size":12`) {
		t.Errorf("a title word wider than its pillar at the step keeps the written sizes: %s", bullet(narrow))
	}
}
