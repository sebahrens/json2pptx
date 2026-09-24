package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

func TestReadableTextOn(t *testing.T) {
	ctx := testThemeCtx()
	tests := []struct {
		name string
		tone fillTone
		want string
	}{
		// dk2 before dk1 on every light fill: the same order the generator's
		// contrast fixer uses, so a card's icon and its text land on one theme
		// ink instead of pure black beside brand dark (go-slide-creator-1sel).
		{"opaque dark accent", fillTone{Color: "accent1"}, "lt1"},
		{"light alpha tint", fillTone{Color: "accent1", Alpha: 40}, "dk2"},
		{"lumMod/lumOff tint", fillTone{Color: "accent1", LumMod: 20000, LumOff: 80000}, "dk2"},
		{"light yellow accent", fillTone{Color: "accent2"}, "dk2"},
		{"dk2 label column", fillTone{Color: "dk2"}, "lt1"},
		{"bg alias", fillTone{Color: "bg1"}, "dk2"},
		{"hex literal", fillTone{Color: "#101010"}, "lt1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readableTextOn(ctx, tt.tone, "fallback"); got != tt.want {
				t.Errorf("readableTextOn = %q, want %q", got, tt.want)
			}
		})
	}

	if got := readableTextOn(ExpandContext{}, fillTone{Color: "accent1"}, "fallback"); got != "fallback" {
		t.Errorf("no theme: want fallback, got %q", got)
	}
	if got := readableTextOn(ctx, fillTone{Color: "nonsense"}, "fallback"); got != "fallback" {
		t.Errorf("unknown colour: want fallback, got %q", got)
	}
}

func TestEffectiveColor(t *testing.T) {
	white := svggen.Color{R: 255, G: 255, B: 255, A: 1}
	base := svggen.MustParseColor("#2E5090")

	if got := EffectiveColor(base, 0, 0, 1, white); got.Hex() != "#2E5090" {
		t.Errorf("unmodified = %s", got.Hex())
	}
	// 50% alpha over white lands half-way.
	half := EffectiveColor(svggen.MustParseColor("#000000"), 0, 0, 0.5, white)
	if half.R < 127 || half.R > 128 {
		t.Errorf("50%% black over white = %s", half.Hex())
	}
	// lumMod 20% + lumOff 80% is a very light tint.
	tint := EffectiveColor(base, 20000, 80000, 1, white)
	if tint.Luminance() < 0.6 {
		t.Errorf("lumMod/lumOff tint too dark: %s (L=%.2f)", tint.Hex(), tint.Luminance())
	}
	// lumMod 75% darkens.
	if dark := EffectiveColor(base, 75000, 0, 1, white); dark.Luminance() >= base.Luminance() {
		t.Errorf("lumMod 75%% should darken: %s", dark.Hex())
	}
	// HSL round-trip on a grey keeps it grey.
	if g := fromHSL(toHSL(svggen.MustParseColor("#808080"))); g.Hex() != "#808080" {
		t.Errorf("grey round-trip = %s", g.Hex())
	}
}

func TestEffectiveColorModsExplicitZeroAlpha(t *testing.T) {
	base := svggen.MustParseColor("#2E5090")
	canvas := svggen.MustParseColor("#F5F5F5")
	if got := EffectiveColorMods(base, ColorMods{Alpha: 0, HasAlpha: true}, canvas).Hex(); got != canvas.Hex() {
		t.Errorf("explicit zero alpha = %s, want canvas %s", got, canvas.Hex())
	}
	if got := EffectiveColorMods(base, ColorMods{}, canvas).Hex(); got != base.Hex() {
		t.Errorf("omitted alpha = %s, want opaque base %s", got, base.Hex())
	}
}
