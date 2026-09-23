package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-uy5s. A slide could only set a background IMAGE, so the
// standard "one dark slide in a light deck" had to be faked with a full-bleed
// shape_grid cell that then fought the layout's title placeholder. And an image
// background had no scrim, so the contrast pass — which reads a solid fill —
// had nothing to judge the text against.

func bgThemeColors() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "000000"},
		{Name: "dk2", RGB: "1B2A4A"},
		{Name: "lt1", RGB: "FFFFFF"},
		{Name: "lt2", RGB: "E8ECF1"},
	}
}

func TestEffectiveGridSlideBackgroundHex(t *testing.T) {
	layout := []byte(`<p:sldLayout><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="EEEEEE"/></a:solidFill></p:bgPr></p:bg></p:cSld></p:sldLayout>`)
	master := []byte(`<p:sldMaster><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="222222"/></a:solidFill></p:bgPr></p:bg></p:cSld></p:sldMaster>`)
	tests := []struct {
		name           string
		bg             *BackgroundImage
		layout, master []byte
		want           string
	}{
		{"authored dark", &BackgroundImage{Color: "dk2"}, layout, master, "#1B2A4A"},
		{"layout", nil, layout, master, "#EEEEEE"},
		{"master", nil, nil, master, "#222222"},
		{"theme fallback", nil, nil, nil, "#FFFFFF"},
		{"photo unknown", &BackgroundImage{Path: "photo.png"}, layout, master, ""},
		{"weak photo scrim unknown", &BackgroundImage{Path: "photo.png", Overlay: &BackgroundOverlay{Color: "dk2", Alpha: 0.1}}, layout, master, ""},
		{"opaque photo scrim", &BackgroundImage{Path: "photo.png", Overlay: &BackgroundOverlay{Color: "dk2", Alpha: 0.8}}, layout, master, "#1B2A4A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveGridSlideBackgroundHex(tc.bg, tc.layout, tc.master, bgThemeColors()); got != tc.want {
				t.Errorf("background = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBackgroundSolidFillXML(t *testing.T) {
	// A scheme name stays a scheme reference so it follows the template theme.
	got := backgroundSolidFillXML("dk2")
	if !strings.Contains(got, `<a:schemeClr val="dk2"/>`) {
		t.Errorf("scheme background = %q", got)
	}
	// A hex value is emitted literally, upper-cased, with any "#" stripped.
	got = backgroundSolidFillXML("#1b2a4a")
	if !strings.Contains(got, `<a:srgbClr val="1B2A4A"/>`) {
		t.Errorf("hex background = %q", got)
	}
	for _, want := range []string{"<p:bg>", "<p:bgPr>", "<a:solidFill>", "<a:effectLst/>"} {
		if !strings.Contains(got, want) {
			t.Errorf("background XML missing %s: %q", want, got)
		}
	}
	if backgroundSolidFillXML("") != "" {
		t.Error("no colour must emit no background element")
	}
}

func TestInsertBackgroundColorBeforeSpTree(t *testing.T) {
	slide := []byte(`<p:sld><p:cSld><p:spTree><p:nvGrpSpPr/></p:spTree></p:cSld></p:sld>`)
	got := string(insertBackgroundColor(slide, "dk2"))
	bgPos := strings.Index(got, "<p:bg>")
	treePos := strings.Index(got, "<p:spTree>")
	if bgPos < 0 || treePos < 0 || bgPos > treePos {
		t.Errorf("background must sit before the shape tree: %q", got)
	}
	// No colour leaves the slide untouched.
	if string(insertBackgroundColor(slide, "")) != string(slide) {
		t.Error("an empty colour must not rewrite the slide")
	}
}

func TestBackgroundScrimXML(t *testing.T) {
	xml, err := backgroundScrimXML(&BackgroundOverlay{Color: "dk1", Alpha: 0.55}, 12192000, 6858000, 42)
	if err != nil {
		t.Fatalf("backgroundScrimXML: %v", err)
	}
	s := string(xml)
	for _, want := range []string{`val="dk1"`, `<a:alpha val="55000"/>`, `cx="12192000"`, `cy="6858000"`} {
		if !strings.Contains(s, want) {
			t.Errorf("scrim XML missing %s: %s", want, s)
		}
	}
	// The defaults are the template's own ink at 45%: dark enough for light
	// text on most photos without hiding the picture.
	xml, err = backgroundScrimXML(&BackgroundOverlay{}, 100, 100, 1)
	if err != nil {
		t.Fatalf("backgroundScrimXML defaults: %v", err)
	}
	s = string(xml)
	if !strings.Contains(s, `val="dk1"`) || !strings.Contains(s, `<a:alpha val="45000"/>`) {
		t.Errorf("scrim defaults = %s", s)
	}
	if xml, err := backgroundScrimXML(nil, 100, 100, 1); err != nil || xml != nil {
		t.Errorf("no overlay must emit no scrim: %v %v", xml, err)
	}
}

// The contrast pass has to judge text against what the audience sees: the
// slide's own fill, or the scrim when it is opaque enough to decide the
// background. A photo with no scrim has no answer, and guessing one would be
// worse than the TEXT_OVER_IMAGE_UNVERIFIED finding that reports it.
func TestEffectiveSlideBackgroundHex(t *testing.T) {
	tests := []struct {
		name string
		bg   *BackgroundImage
		want string
	}{
		{name: "no background", bg: nil, want: ""},
		{name: "scheme colour resolves through the theme", bg: &BackgroundImage{Color: "dk2"}, want: "#1B2A4A"},
		{name: "hex colour is normalised", bg: &BackgroundImage{Color: "#1b2a4a"}, want: "#1B2A4A"},
		{name: "photo with no scrim has no single colour", bg: &BackgroundImage{Path: "hero.jpg"}, want: ""},
		{
			name: "an opaque scrim decides the background",
			bg:   &BackgroundImage{Path: "hero.jpg", Overlay: &BackgroundOverlay{Color: "dk1", Alpha: 0.55}},
			want: "#000000",
		},
		{
			name: "a scrim with no colour uses the default ink",
			bg:   &BackgroundImage{Path: "hero.jpg", Overlay: &BackgroundOverlay{Alpha: 0.5}},
			want: "#000000",
		},
		{
			name: "a barely-there scrim still lets the photo through",
			bg:   &BackgroundImage{Path: "hero.jpg", Overlay: &BackgroundOverlay{Color: "dk1", Alpha: 0.1}},
			want: "",
		},
		{name: "an unresolvable colour is no answer", bg: &BackgroundImage{Color: "brand-navy"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveSlideBackgroundHex(tt.bg, bgThemeColors()); got != tt.want {
				t.Errorf("effectiveSlideBackgroundHex = %q, want %q", got, tt.want)
			}
		})
	}
}
