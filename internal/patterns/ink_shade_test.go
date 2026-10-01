package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// Mid-tone primary accents of the shipped templates on which white 12-13pt
// labels miss WCAG AA (go-slide-creator-v9tup review table).
var midToneAccents = map[string]string{
	"abstract":          "#8E8172",
	"warm-coral":        "#E64A19",
	"blue-corporate":    "#55BC7E",
	"business-template": "#AD84C6",
	"p-style":           "#FD5108",
}

func ctxWithAccent1(rgb string) ExpandContext {
	ctx := fullThemeCtx()
	for i := range ctx.Theme.Colors {
		if ctx.Theme.Colors[i].Name == "accent1" {
			ctx.Theme.Colors[i].RGB = rgb
		}
	}
	return ctx
}

// A saturated accent that lt1 misses is shaded until lt1 reads; the fixer
// never answers dk1 (pure black) while such a shade exists.
func TestAccentFillAndInkShadesInsteadOfBlack(t *testing.T) {
	for tmpl, rgb := range midToneAccents {
		for _, bar := range []float64{4.5, 3.0} {
			ctx := ctxWithAccent1(rgb)
			tone, ink := accentFillAndInk(ctx, fillTone{Color: "accent1"}, bar)
			if ink != "lt1" {
				t.Errorf("%s bar %.1f: ink = %s, want lt1 on a shaded accent", tmpl, bar, ink)
				continue
			}
			light, _ := resolveThemeColor(ctx, "lt1")
			raw, _ := effectiveFillColor(ctx, fillTone{Color: "accent1"})
			needsShade := light.ContrastWith(raw) < bar
			if tone.Color != "accent1" || (tone.Shade != 0) != needsShade || (needsShade && tone.Shade < int(shadeMinKeep*100000)) {
				t.Errorf("%s bar %.1f: tone = %+v, want accent1 shaded only when lt1 misses (%v)", tmpl, bar, tone, needsShade)
			}
			fill, _ := effectiveFillColor(ctx, tone)
			if r := light.ContrastWith(fill); r < bar {
				t.Errorf("%s bar %.1f: lt1 on shaded fill = %.2f", tmpl, bar, r)
			}
		}
	}
}

// Hex fills get the shaded hex (the shape-grid resolver ignores modifiers on
// hex colours), tints and pale accents are left to the ink swap.
func TestShadeForLightInkScope(t *testing.T) {
	ctx := fullThemeCtx()
	if tone, ok := shadeForLightInk(ctx, fillTone{Color: "#FD5108"}, 4.5); !ok || !isHexColor(tone.Color) || tone.Shade != 0 {
		t.Errorf("hex accent: got %+v ok=%v, want a shaded hex", tone, ok)
	}
	if _, ok := shadeForLightInk(ctx, fillTone{Color: "accent1", LumMod: tintLumMod, LumOff: tintLumOff}, 4.5); ok {
		t.Error("light tint must not be shaded dark")
	}
	if _, ok := shadeForLightInk(ctx, fillTone{Color: "#FFE600"}, 4.5); ok {
		t.Error("pale yellow accent must keep its surface and take dark ink")
	}
	if _, ok := shadeForLightInk(ctx, fillTone{Color: "lt2"}, 4.5); ok {
		t.Error("neutral surface must not be shaded")
	}
	if _, ok := shadeForLightInk(ctx, fillTone{Color: "accent1"}, 4.5); ok {
		t.Error("#2E5090 already reads with lt1; no shade expected")
	}
}

// ApplyReadableInk shades the fill of a pattern label written lt1 on a
// mid-tone accent and keeps the white type; a shape mixing dark and light ink
// keeps its fill and has only the light ink swapped.
func TestApplyReadableInkShadesAccentFill(t *testing.T) {
	ctx := ctxWithAccent1("#E64A19")
	label := &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"accent1"`),
		Text:     json.RawMessage(`{"content":"Design","size":13,"bold":true,"color":"lt1"}`),
	}
	mixed := &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"accent1"`),
		Text:     json.RawMessage(`{"paragraphs":[{"content":"A","color":"lt1"},{"content":"B","color":"dk2"}],"size":12}`),
	}
	grid := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{
		{Shape: label}, {Shape: mixed},
	}}}}
	ApplyReadableInk(ctx, grid)

	tone, ok := parseFillTone(label.Fill)
	if !ok || tone.Color != "accent1" || tone.Shade == 0 {
		t.Fatalf("label fill = %s, want shaded accent1", label.Fill)
	}
	var text map[string]any
	if err := json.Unmarshal(label.Text, &text); err != nil {
		t.Fatal(err)
	}
	if text["color"] != "lt1" {
		t.Errorf("label ink = %v, want lt1 kept", text["color"])
	}
	if string(mixed.Fill) != `"accent1"` {
		t.Errorf("mixed-ink shape fill = %s, want unchanged", mixed.Fill)
	}
}
