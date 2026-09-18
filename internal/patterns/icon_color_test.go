package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Bundled template palettes (theme1.xml) the icons are verified against.
var iconColorThemes = map[string][]types.ThemeColor{
	"midnight-blue": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "lt2", RGB: "E8ECF1"}, {Name: "dk2", RGB: "1B2A4A"},
		{Name: "accent1", RGB: "2E5090"}, {Name: "accent2", RGB: "D4463A"}, {Name: "accent5", RGB: "5C6BC0"},
		{Name: "accent3", RGB: "E8A838"}, {Name: "accent4", RGB: "43A047"},
		{Name: "accent6", RGB: "26A69A"},
	},
	"warm-coral": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "lt2", RGB: "FBE9E7"}, {Name: "dk2", RGB: "3E2723"},
		{Name: "accent1", RGB: "E64A19"}, {Name: "accent2", RGB: "5D4037"}, {Name: "accent5", RGB: "7B1FA2"},
		{Name: "accent3", RGB: "FF8A65"}, {Name: "accent4", RGB: "0097A7"},
		{Name: "accent6", RGB: "689F38"},
	},
}

// assertIconReadable checks that the shape's icon fill differs from the
// shape fill and meets the 3:1 non-text contrast minimum against it.
func assertIconReadable(t *testing.T, ctx ExpandContext, label string, shape *jsonschema.ShapeSpecInput) {
	t.Helper()
	if shape == nil || shape.Icon == nil {
		t.Fatalf("%s: expected an icon overlay", label)
	}
	tone, ok := parseFillTone(shape.Fill)
	if !ok {
		t.Fatalf("%s: cannot parse shape fill %s", label, shape.Fill)
	}
	if shape.Icon.Fill == tone.Color && tone.LumMod == 0 && tone.LumOff == 0 && tone.Alpha == 0 {
		t.Errorf("%s: icon fill %q equals cell fill", label, shape.Icon.Fill)
	}
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		t.Fatalf("%s: cannot resolve fill %q", label, tone.Color)
	}
	ic, ok := resolveThemeColor(ctx, shape.Icon.Fill)
	if !ok {
		t.Fatalf("%s: cannot resolve icon fill %q", label, shape.Icon.Fill)
	}
	if cr := ic.ContrastWith(fill); cr < iconMinContrast {
		t.Errorf("%s: icon %q on fill %q contrast %.2f < %.1f", label, shape.Icon.Fill, tone.Color, cr, iconMinContrast)
	}
}

// TestKPI3up_IconContrastsWithCard is the p1ir regression: the kpi-3up icon
// used to default to the card accent, i.e. the card's own fill colour.
func TestKPI3up_IconContrastsWithCard(t *testing.T) {
	p, ok := Default().Get("kpi-3up")
	if !ok {
		t.Fatal("kpi-3up not registered")
	}
	for name, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		vals := Kpi3upValues{
			{Big: "$4.2M", Small: "ARR", Icon: &IconRef{Name: "currency-dollar"}},
			{Big: "127%", Small: "NRR", Icon: &IconRef{Name: "trending-up"}},
			{Big: "12d", Small: "Cycle", Icon: &IconRef{Name: "clock"}},
		}
		for _, mode := range []string{"", "alternate", "progressive"} {
			ovr := &KPIOverrides{CellAccentMode: mode}
			grid, err := p.Expand(ctx, &vals, ovr, nil)
			if err != nil {
				t.Fatalf("%s: Expand: %v", name, err)
			}
			for i, c := range grid.Rows[0].Cells {
				assertIconReadable(t, ctx, name+"/"+mode+"/cell"+string(rune('0'+i)), c.Shape)
			}
		}
	}
}

func TestIconRow_IconContrastsWithCell(t *testing.T) {
	p, _ := Default().Get("icon-row")
	for name, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		vals := IconRowValues{
			{Icon: &IconRef{Name: "rocket"}, Caption: "Launch"},
			{Icon: &IconRef{Name: "shield"}, Caption: "Trust"},
		}
		grid, err := p.Expand(ctx, &vals, nil, nil)
		if err != nil {
			t.Fatalf("%s: Expand: %v", name, err)
		}
		for _, c := range grid.Rows[0].Cells {
			assertIconReadable(t, ctx, name+"/icon-row", c.Shape)
		}
	}
}

// TestCardGrid_IconColorPerStyle: solid accent cards get a light icon, light
// cards keep the on-brand accent icon when it reads (>= 3:1).
func TestCardGrid_IconColorPerStyle(t *testing.T) {
	p, _ := Default().Get("card-grid")
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: iconColorThemes["midnight-blue"]}}
	vals := CardGridValues{Columns: 2, Rows: 1, Cells: []CardGridCell{
		{Header: "Launch", Body: "b", Icon: &IconRef{Name: "rocket"}},
		{Header: "Growth", Body: "b", Icon: &IconRef{Name: "trending-up"}},
	}}
	cases := map[string]string{"filled": "lt1", "icon-card": "accent1"}
	for style, want := range cases {
		grid, err := p.Expand(ctx, &vals, &CardGridOverrides{Style: style}, nil)
		if err != nil {
			t.Fatalf("%s: Expand: %v", style, err)
		}
		shape := grid.Rows[0].Cells[0].Shape
		assertIconReadable(t, ctx, "card-grid/"+style, shape)
		if shape.Icon.Fill != want {
			t.Errorf("card-grid/%s: icon fill = %q, want %q", style, shape.Icon.Fill, want)
		}
	}

	// An explicit author fill is never overridden.
	vals.Cells[0].Icon = &IconRef{Name: "rocket", Fill: "accent3"}
	grid, err := p.Expand(ctx, &vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := grid.Rows[0].Cells[0].Shape.Icon.Fill; got != "accent3" {
		t.Errorf("explicit icon fill overridden: got %q", got)
	}
}

func TestIconFillOn(t *testing.T) {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: iconColorThemes["midnight-blue"]}}
	tests := []struct {
		name string
		ctx  ExpandContext
		fill string
		want string
	}{
		{"solid accent -> lt1", ctx, `"accent1"`, "lt1"},
		{"white card -> accent", ctx, `"lt1"`, "accent1"},
		{"transparent -> accent", ctx, `"none"`, "accent1"},
		{"empty -> accent", ctx, ``, "accent1"},
		{"light tint -> accent", ctx, `{"color":"accent1","lumMod":20000,"lumOff":80000}`, "accent1"},
		{"accent reads on contrasting fill -> accent", ctx, `"accent3"`, "accent1"},
		{"no theme, solid accent -> lt1", ExpandContext{}, `"accent1"`, "lt1"},
		{"no theme, light card -> accent", ExpandContext{}, `"lt1"`, "accent1"},
	}
	for _, tt := range tests {
		if got := iconFillOn(tt.ctx, json.RawMessage(tt.fill), "accent1"); got != tt.want {
			t.Errorf("%s: iconFillOn = %q, want %q", tt.name, got, tt.want)
		}
	}
	// Mid-tone coral: dk1 contrasts more, but lt1 clears 3:1 and matches
	// the white card text, so the icon stays lt1.
	coral := ExpandContext{Theme: types.ThemeInfo{Colors: iconColorThemes["warm-coral"]}}
	if got := iconFillOn(coral, json.RawMessage(`"accent1"`), "accent1"); got != "lt1" {
		t.Errorf("warm-coral solid accent: iconFillOn = %q, want lt1", got)
	}
	// Solid light accent (yellow): lt1 fails, dk1 wins.
	if got := iconFillOn(ctx, json.RawMessage(`"accent3"`), "accent3"); got != "dk1" {
		t.Errorf("solid yellow accent: iconFillOn = %q, want dk1", got)
	}
	if _, ok := parseFillTone(json.RawMessage(`{bad`)); ok {
		t.Error("parseFillTone accepted malformed JSON")
	}
	if _, ok := parseFillTone(json.RawMessage(`{"color":"none"}`)); ok {
		t.Error("parseFillTone accepted none object")
	}
}
