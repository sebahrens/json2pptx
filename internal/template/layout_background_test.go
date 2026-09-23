package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestResolveLayoutBackgroundHexAppliesLumMod(t *testing.T) {
	colors := []types.ThemeColor{{Name: "accent6", RGB: "#60A2F5"}, {Name: "lt1", RGB: "#FFFFFF"}}
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:schemeClr val="accent6"><a:lumMod val="50000"/></a:schemeClr></a:solidFill></p:bgPr></p:bg></p:cSld></p:sldLayout>`)
	mods := ResolveLayoutBackgroundModifiers(layout)
	if !mods.HasLumMod || mods.LumMod != 50000 {
		t.Fatalf("background modifiers = %+v", mods)
	}
	hex := ResolveLayoutBackgroundHex(layout, colors)
	if hex == "#60A2F5" {
		t.Fatal("layout background returned the unmodified theme accent")
	}
	visible := svggen.MustParseColor(hex)
	if ratio := visible.ContrastWith(svggen.MustParseColor("#FFFFFF")); ratio < 4.5 {
		t.Errorf("white text on modified background %s has contrast %.2f:1, want >= 4.5:1", hex, ratio)
	}
	if got := ResolveBackgroundRefHexWithMods("accent6", mods, colors); got != hex {
		t.Errorf("re-resolved modified background = %s, want %s", got, hex)
	}
	// A theme override must change the base before reapplying the same transform.
	overridden := []types.ThemeColor{{Name: "accent6", RGB: "#204080"}, {Name: "lt1", RGB: "#FFFFFF"}}
	if got := ResolveBackgroundRefHexWithMods("accent6", mods, overridden); got == hex || got == "#204080" {
		t.Errorf("theme override did not retain the transform: %s", got)
	}
}

func TestResolveLayoutBackgroundHexAppliesOtherModifiers(t *testing.T) {
	for _, tc := range []struct {
		name, base, modifier, want string
	}{
		{name: "lumOff", base: "#000000", modifier: `<a:lumOff val="100000"/>`, want: "#FFFFFF"},
		{name: "tint", base: "#000000", modifier: `<a:tint val="50000"/>`, want: "#BCBCBC"},
		{name: "shade", base: "#FFFFFF", modifier: `<a:shade val="50000"/>`, want: "#BCBCBC"},
		{name: "alpha zero", base: "#000000", modifier: `<a:alpha val="0"/>`, want: "#FFFFFF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			colors := []types.ThemeColor{{Name: "accent1", RGB: tc.base}, {Name: "lt1", RGB: "#FFFFFF"}}
			xml := []byte(`<p:bg><p:bgPr><a:solidFill><a:schemeClr val="accent1">` + tc.modifier + `</a:schemeClr></a:solidFill></p:bgPr></p:bg>`)
			if got := ResolveLayoutBackgroundHex(xml, colors); got != tc.want {
				t.Errorf("visible background = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBlueCorporateClosingBackgroundIsDarkAfterLumMod(t *testing.T) {
	reader, err := OpenTemplate("../../templates/blue-corporate.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		if layout.ID != "slideLayout4" {
			continue
		}
		if layout.BackgroundRef != "accent6" || layout.BackgroundMods.LumMod != 50000 {
			t.Fatalf("closing background ref/modifiers = %q / %+v", layout.BackgroundRef, layout.BackgroundMods)
		}
		if ratio := svggen.MustParseColor(layout.BackgroundHex).ContrastWith(svggen.MustParseColor("#FFFFFF")); ratio < 4.5 {
			t.Errorf("closing background %s makes inherited white text unreadable: %.2f:1", layout.BackgroundHex, ratio)
		}
		return
	}
	t.Fatal("blue-corporate closing layout missing")
}

func TestResolveLayoutBackgroundHexAppliesColorMapOverride(t *testing.T) {
	colors := []types.ThemeColor{{Name: "dk1", RGB: "#111111"}, {Name: "lt1", RGB: "#FFFFFF"}}
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></p:bgPr></p:bg></p:cSld><p:clrMapOvr><a:overrideClrMapping tx1="lt1"/></p:clrMapOvr></p:sldLayout>`)
	if got := ResolveLayoutBackgroundHex(layout, colors); got != "#FFFFFF" {
		t.Errorf("mapped layout background = %s, want #FFFFFF", got)
	}
	if ref := ResolveLayoutBackgroundRef(layout); ref != "lt1" {
		t.Errorf("mapped background reference = %s, want lt1", ref)
	}
	overridden := []types.ThemeColor{{Name: "lt1", RGB: "#222222"}}
	if got := ResolveBackgroundRefHex(ResolveLayoutBackgroundRef(layout), overridden); got != "#222222" {
		t.Errorf("overridden background = %s, want #222222", got)
	}
}

func TestParseLayoutsCarriesEffectiveBackground(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		if layout.ID == "slideLayout2" {
			if layout.BackgroundHex != "#FFFFFF" || layout.BackgroundRef != "lt1" {
				t.Errorf("slideLayout2 background = %q / %q, want #FFFFFF / lt1", layout.BackgroundHex, layout.BackgroundRef)
			}
			return
		}
	}
	t.Fatal("modern-template slideLayout2 missing")
}
