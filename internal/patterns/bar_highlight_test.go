package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

const neutralBarFill = `{"color":"dk1","lumMod":38000,"lumOff":62000}`

// hbcBarFills returns each row's bar fill (the sub-grid's second cell).
func hbcBarFills(t *testing.T, v *HorizontalBarCalloutsValues, ovr *HorizontalBarCalloutsOverrides) []string {
	t.Helper()
	p, _ := Default().Get("horizontal-bar-with-callouts")
	var o any
	if ovr != nil {
		o = ovr
	}
	grid, err := p.Expand(ExpandContext{}, v, o, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	fills := make([]string, len(grid.Rows))
	for i, row := range grid.Rows {
		fills[i] = string(row.Cells[0].Grid.Rows[0].Cells[1].Shape.Fill)
	}
	return fills
}

func sdxiiBars() []HorizontalBarCalloutsBar {
	return []HorizontalBarCalloutsBar{
		{Label: "Marketing", Value: 52, Callout: "Content ops."},
		{Label: "Engineering", Value: 71, Callout: "Code assistants."},
		{Label: "Customer support", Value: 64, Callout: "Agents."},
		{Label: "Legal", Value: 26, Callout: "Gated."},
	}
}

// go-slide-creator-sdxii: the bars are neutral and only the top one — or the
// ones the author names — take accent1.
func TestHorizontalBarCallouts_HighlightDefaultsToTopBar(t *testing.T) {
	fills := hbcBarFills(t, &HorizontalBarCalloutsValues{Bars: sdxiiBars()}, nil)
	for i, f := range fills {
		want := neutralBarFill
		if i == 1 {
			want = `"accent1"`
		}
		if f != want {
			t.Errorf("bar %d fill = %s, want %s", i, f, want)
		}
	}
}

func TestHorizontalBarCallouts_ExplicitHighlight(t *testing.T) {
	var v HorizontalBarCalloutsValues
	if err := json.Unmarshal([]byte(`{"bars":[
		{"label":"Marketing","value":52},{"label":"Engineering","value":71},
		{"label":"Customer support","value":64},{"label":"Legal","value":26}],
		"highlight":["engineering", 2]}`), &v); err != nil {
		t.Fatal(err)
	}
	p, _ := Default().Get("horizontal-bar-with-callouts")
	if err := p.Validate(&v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	fills := hbcBarFills(t, &v, &HorizontalBarCalloutsOverrides{TextOverrides: TextOverrides{Accent: "accent3"}})
	want := []string{neutralBarFill, `"accent3"`, `"accent3"`, neutralBarFill}
	for i := range want {
		if fills[i] != want[i] {
			t.Errorf("bar %d fill = %s, want %s", i, fills[i], want[i])
		}
	}

	v.Highlight = []any{}
	for i, f := range hbcBarFills(t, &v, nil) {
		if f != neutralBarFill {
			t.Errorf("empty highlight: bar %d fill = %s, want neutral", i, f)
		}
	}

	v.Highlight = []any{"Finance"}
	err := p.Validate(&v, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "highlight") {
		t.Errorf("an unknown highlight label must fail validation, got %v", err)
	}
}

// A neutral bar's in-bar value label must stay readable and is not bold; the
// highlighted bar's label is bold.
func TestHorizontalBarCallouts_HighlightLabelWeight(t *testing.T) {
	p, _ := Default().Get("horizontal-bar-with-callouts")
	grid, err := p.Expand(ExpandContext{}, &HorizontalBarCalloutsValues{Bars: sdxiiBars()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range grid.Rows {
		fill := row.Cells[0].Grid.Rows[0].Cells[1].Shape
		if len(fill.Text) == 0 {
			continue
		}
		bold := strings.Contains(string(fill.Text), `"bold":true`)
		if bold != (i == 1) {
			t.Errorf("bar %d value label bold = %v", i, bold)
		}
	}
}

func TestWaterfallBridge_AccentLabelsBold(t *testing.T) {
	p, _ := Default().Get("waterfall-bridge")
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, &WaterfallBridgeValues{
		Unit: "$",
		Columns: []WaterfallBridgeColumn{
			{Label: "Human", Value: 12, Type: "total"},
			{Label: "Agent", Value: -8.5, Type: "delta"},
			{Label: "Review", Value: 1.5, Type: "delta"},
			{Label: "AI", Type: "subtotal"},
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(grid)
	out := string(raw)
	if !strings.Contains(out, "−$8.5") {
		t.Fatalf("decrease label should carry a true minus sign: %s", out)
	}
	// Only the accent (decrease) label is bold.
	if n := strings.Count(out, `\"bold\":true`) + strings.Count(out, `"bold":true`); n != 1 {
		t.Errorf("want exactly one bold value label (the decrease), got %d", n)
	}
}
