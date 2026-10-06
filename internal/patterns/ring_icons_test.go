package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// Icons on the circular family (go-slide-creator-yjsvq): radial-hub draws a
// spoke's icon in its satellite disc and cycle-nodes a step's icon in its node,
// both as an icon overlay on the layer's shape.

var ringTestIcons = []string{"speakerphone", "trending-up", "headset", "report-money", "shield-check", "bulb", "settings", "users"}

func rhIconValues(n int) *RadialHubValues {
	v := rhValues(n, true)
	for i := range v.Spokes {
		v.Spokes[i].Icon = &IconRef{Name: ringTestIcons[i]}
	}
	return v
}

// ringIconContrast is the contrast of a layer's icon ink against the layer's
// own fill on ctx's theme.
func ringIconContrast(t *testing.T, ctx ExpandContext, l jsonschema.LayerInput) float64 {
	t.Helper()
	tone, ok := parseFillTone(l.Shape.Fill)
	if !ok {
		t.Fatalf("%s: fill %s is not a tone", l.Name, l.Shape.Fill)
	}
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		t.Fatalf("%s: fill %s does not resolve on the theme", l.Name, l.Shape.Fill)
	}
	ink, ok := resolveThemeColor(ctx, l.Shape.Icon.Fill)
	if !ok {
		t.Fatalf("%s: icon fill %q is not a theme colour", l.Name, l.Shape.Icon.Fill)
	}
	return ink.ContrastWith(fill)
}

func TestRadialHub_IconsSitInTheSatellites(t *testing.T) {
	for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
		v := rhIconValues(n)
		v.Spokes[1].Icon = nil // one spoke without an icon keeps an empty disc
		ring := rhRingCell(t, rhExpand(t, rhCtx(899, 360), v, nil))
		sats := rhLayersNamed(ring, "satellite-")
		if len(sats) != n {
			t.Fatalf("n=%d: %d satellites", n, len(sats))
		}
		for i, l := range sats {
			if i == 1 {
				if l.Shape.Icon != nil {
					t.Errorf("n=%d: %s has an icon its spoke did not ask for", n, l.Name)
				}
				continue
			}
			ic := l.Shape.Icon
			if ic == nil {
				t.Fatalf("n=%d: %s has no icon", n, l.Name)
			}
			if ic.Name != ringTestIcons[i] || ic.Position != "center" || ic.Scale != rhIconScale {
				t.Errorf("n=%d: %s icon = %+v, want %s centred at scale %.2f", n, l.Name, ic, ringTestIcons[i], rhIconScale)
			}
			if len(l.Shape.Text) != 0 {
				t.Errorf("n=%d: %s carries text under its icon: %s", n, l.Name, l.Shape.Text)
			}
		}
		for _, l := range ring.Layers {
			if !strings.HasPrefix(l.Name, "satellite-") && l.Shape.Icon != nil {
				t.Errorf("n=%d: layer %s carries an icon", n, l.Name)
			}
		}
	}
}

// The icon's ink is measured on the disc it sits on: the neutral tint, the
// highlighted accent tint and every solid accent of cell_accent_mode.
func TestRadialHub_IconInkReadsOnEveryFill(t *testing.T) {
	ctx := rhCtx(899, 360)
	for _, tc := range []struct {
		name string
		mut  func(*RadialHubValues, *RadialHubOverrides)
	}{
		{"uniform", func(*RadialHubValues, *RadialHubOverrides) {}},
		{"highlight", func(v *RadialHubValues, _ *RadialHubOverrides) { v.Highlight = 3 }},
		{"alternate", func(_ *RadialHubValues, o *RadialHubOverrides) { o.CellAccentMode = CellAccentAlternate }},
		{"progressive", func(_ *RadialHubValues, o *RadialHubOverrides) { o.CellAccentMode = CellAccentProgressive }},
		{"accent3", func(_ *RadialHubValues, o *RadialHubOverrides) { o.Accent = "accent3" }},
	} {
		v, ovr := rhIconValues(8), &RadialHubOverrides{}
		tc.mut(v, ovr)
		ring := rhRingCell(t, rhExpand(t, ctx, v, ovr))
		for _, l := range rhLayersNamed(ring, "satellite-") {
			if strings.HasPrefix(l.Shape.Icon.Fill, "#") {
				t.Errorf("%s: %s icon fill %q is a hex colour", tc.name, l.Name, l.Shape.Icon.Fill)
			}
			if c := ringIconContrast(t, ctx, l); c < iconMinContrast {
				t.Errorf("%s: %s icon %s on %s is %.2f:1, under %.1f:1", tc.name, l.Name, l.Shape.Icon.Fill, l.Shape.Fill, c, iconMinContrast)
			}
		}
	}
}

func TestRadialHub_AuthorIconSettingsWin(t *testing.T) {
	v := rhIconValues(4)
	v.Spokes[0].Icon = &IconRef{Name: "shield", Fill: "accent2", Scale: 0.4, Alt: "Protected"}
	l := rhLayersNamed(rhRingCell(t, rhExpand(t, rhCtx(899, 360), v, nil)), "satellite-")[0]
	if ic := l.Shape.Icon; ic.Fill != "accent2" || ic.Scale != 0.4 || ic.Alt != "Protected" {
		t.Errorf("authored icon settings were replaced: %+v", ic)
	}
}

// In the legend an icon is the item's key: it replaces the letter in the disc
// and in the key column; a spoke without one keeps its letter in both.
func TestRadialHub_LegendKeysByIcon(t *testing.T) {
	v := rhIconValues(6)
	v.Spokes[3].Icon = nil
	grid := rhExpand(t, rhCtx(899, 360), v, &RadialHubOverrides{Labels: rhLabelsLegend})
	for i, l := range rhLayersNamed(rhRingCell(t, grid), "satellite-") {
		letter := fmt.Sprintf(`"content":%q`, rhKey(i))
		if i == 3 {
			if l.Shape.Icon != nil || !strings.Contains(string(l.Shape.Text), letter) {
				t.Errorf("%s: want the key letter %s and no icon, got icon %+v text %s", l.Name, rhKey(i), l.Shape.Icon, l.Shape.Text)
			}
			continue
		}
		if l.Shape.Icon == nil || len(l.Shape.Text) != 0 {
			t.Errorf("%s: want the icon in place of the key letter, got icon %+v text %s", l.Name, l.Shape.Icon, l.Shape.Text)
		}
	}
	var iconKeys, letterKeys int
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if c == nil || c.Shape == nil || len(c.Layers) > 0 {
				continue
			}
			switch {
			case c.Shape.Icon != nil:
				iconKeys++
				if c.Shape.Icon.Position != "top" || len(c.Shape.Text) != 0 {
					t.Errorf("legend key icon %+v: want a top icon without text", c.Shape.Icon)
				}
			case strings.Contains(string(c.Shape.Text), `"content":"D"`):
				letterKeys++
			}
			for _, letter := range []string{"A", "B", "C", "E", "F"} {
				if strings.Contains(string(c.Shape.Text), fmt.Sprintf(`"content":%q`, letter)) {
					t.Errorf("legend still shows the key letter %s of a spoke with an icon", letter)
				}
			}
		}
	}
	if iconKeys != 5 || letterKeys != 1 {
		t.Errorf("legend has %d icon keys and %d letter keys, want 5 and 1", iconKeys, letterKeys)
	}
}

func TestRadialHub_IconValidation(t *testing.T) {
	p := &radialHub{}
	v := rhIconValues(4)
	v.Spokes[2].Icon = &IconRef{Name: "no-such-icon"}
	if codes := rhErrCodes(p.Validate(v, nil, nil)); codes["spokes[2].icon.name"] != ErrCodeInvalidShape {
		t.Errorf("unknown icon name: codes = %v", codes)
	}
	v = rhIconValues(4)
	v.Spokes[0].Icon = &IconRef{Name: "shield", Path: "a.svg"}
	if codes := rhErrCodes(p.Validate(v, nil, nil)); codes["spokes[0].icon"] != ErrCodeInvalidShape {
		t.Errorf("two icon sources: codes = %v", codes)
	}
	// A disc that holds its label has no room for an icon.
	v = rhValues(4, false)
	v.Spokes[1].Icon = &IconRef{Name: "shield"}
	err := p.Validate(v, &RadialHubOverrides{Labels: rhLabelsInside}, nil)
	if codes := rhErrCodes(err); codes["spokes[1].icon"] != ErrCodeUnknownKey {
		t.Fatalf("icon with labels inside: codes = %v", codes)
	}
	if !strings.Contains(err.Error(), "labels outside") {
		t.Errorf("error does not name the way out: %v", err)
	}
	if err := p.Validate(rhIconValues(8), nil, nil); err != nil {
		t.Errorf("eight spokes with bundled icons: %v", err)
	}
}

func TestRingIconsParseBothForms(t *testing.T) {
	var hub RadialHubValues
	if err := json.Unmarshal([]byte(`{"center":{"label":"Hub"},"spokes":[{"label":"A","icon":"shield"},{"label":"B","icon":{"name":"bulb","fill":"accent2"}},{"label":"C"}]}`), &hub); err != nil {
		t.Fatal(err)
	}
	if hub.Spokes[0].Icon == nil || hub.Spokes[0].Icon.Name != "shield" || hub.Spokes[1].Icon.Fill != "accent2" || hub.Spokes[2].Icon != nil {
		t.Errorf("radial-hub icons parsed as %+v", hub.Spokes)
	}
	var loop CycleNodesValues
	if err := json.Unmarshal([]byte(`{"steps":[{"label":"Plan","icon":"target"},{"label":"Do"},{"label":"Check","icon":{"name":"chart-bar"}}]}`), &loop); err != nil {
		t.Fatal(err)
	}
	if loop.Steps[0].Icon == nil || loop.Steps[0].Icon.Name != "target" || loop.Steps[1].Icon != nil || loop.Steps[2].Icon.Name != "chart-bar" {
		t.Errorf("cycle-nodes icons parsed as %+v", loop.Steps)
	}
	for name, schema := range map[string]*Schema{"radial-hub": (&radialHub{}).Schema(), "cycle-nodes": (&cycleNodes{}).Schema()} {
		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) > 6000 {
			t.Errorf("%s schema is %d bytes, over 6000", name, len(data))
		}
		if !strings.Contains(string(data), `"icon"`) {
			t.Errorf("%s schema does not describe icon", name)
		}
	}
}

// A step's icon takes the numeral's place in the node, in the numeral's ink;
// the number stays as the cue beside the label.
func TestCycleNodes_IconReplacesTheNumeral(t *testing.T) {
	ctx := fullThemeCtx()
	for _, mode := range []string{"", CellAccentProgressive} {
		v := cycleNodesTestValues(5)
		v.Highlight = 2
		for _, i := range []int{0, 1, 3} {
			v.Steps[i].Icon = &IconRef{Name: ringTestIcons[i]}
		}
		ovr := &CycleNodesOverrides{}
		ovr.CellAccentMode = mode
		grid := cycleNodesExpand(t, ctx, v, ovr)
		paints := cycleNodesPaints(ctx, v, ovr, ctx.ResolveAccent("", ""))
		nodes := cycleNodesLayersNamed(cycleNodesRing(t, grid), "node-")
		for i, l := range nodes {
			numeral := fmt.Sprintf(`"content":"%d"`, i+1)
			if v.Steps[i].Icon == nil {
				if l.Shape.Icon != nil || !strings.Contains(string(l.Shape.Text), numeral) {
					t.Errorf("mode %q %s: want the numeral and no icon, got %+v / %s", mode, l.Name, l.Shape.Icon, l.Shape.Text)
				}
				continue
			}
			ic := l.Shape.Icon
			if ic == nil || len(l.Shape.Text) != 0 {
				t.Fatalf("mode %q %s: want an icon in place of the numeral, got %+v / %s", mode, l.Name, ic, l.Shape.Text)
			}
			if ic.Position != "center" || ic.Scale != cycleNodesIconScale || ic.Fill != paints[i].ink {
				t.Errorf("mode %q %s: icon %+v, want centred at %.2f in the numeral ink %s", mode, l.Name, ic, cycleNodesIconScale, paints[i].ink)
			}
			if c := ringIconContrast(t, ctx, l); c < cycleNodesInkContrast {
				t.Errorf("mode %q %s: icon %s on %s is %.2f:1, under %.1f:1", mode, l.Name, ic.Fill, l.Shape.Fill, c, cycleNodesInkContrast)
			}
		}
		// Every step keeps its number cue beside the label.
		cues := map[string]bool{}
		for _, c := range cycleNodesTextCells(t, grid) {
			if len(c.paras) == 1 {
				cues[c.paras[0].Content] = true
			}
		}
		for i := range v.Steps {
			if !cues[fmt.Sprint(i+1)] {
				t.Errorf("mode %q: step %d lost its number cue", mode, i+1)
			}
		}
	}
}

func TestCycleNodes_IconValidation(t *testing.T) {
	p := &cycleNodes{}
	v := cycleNodesTestValues(4)
	v.Steps[1].Icon = &IconRef{Name: "no-such-icon"}
	if codes := rhErrCodes(p.Validate(v, nil, nil)); codes["steps[1].icon.name"] != ErrCodeInvalidShape {
		t.Errorf("unknown icon name: codes = %v", codes)
	}
	v = &CycleNodesValues{Steps: []CycleNodesStep{{Label: "Plan", Icon: &IconRef{Name: "target"}}, {Label: "Do"}, {Label: "Check"}}}
	err := p.Validate(v, &CycleNodesOverrides{Labels: cycleNodesLabelsInside}, nil)
	if codes := rhErrCodes(err); codes["steps[0].icon"] != ErrCodeInvalidShape {
		t.Fatalf("icon with labels inside: codes = %v", codes)
	}
	if !strings.Contains(err.Error(), `labels "outside"`) {
		t.Errorf("error does not name the way out: %v", err)
	}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("a step icon with labels outside: %v", err)
	}
}
