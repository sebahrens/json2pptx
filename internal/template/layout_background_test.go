package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

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
