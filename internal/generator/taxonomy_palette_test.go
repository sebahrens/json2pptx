package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-w0kj: the taxonomy frameworks used to colour every cell with
// a different theme accent. go-slide-creator-amtkg: they then took one accent
// pastel, a different surface from every pattern. These tests pin the
// contract — the shared neutral card with one accent (the title ink) by
// default, and accent tints reachable through style.colors.

func TestTaxonomyPaletteDefaultsToOneHue(t *testing.T) {
	for _, c := range []struct {
		name     string
		n        int
		defaults func(int) taxonomyTint
		want     map[string]bool
	}{
		{"bmc", len(bmcSectionOrder), bmcDefaultTint, map[string]bool{"dk1": true}},
		{"pestel", 6, uniformTaxonomyTint, map[string]bool{"dk1": true}},
		{"swot", 4, swotDefaultTint, map[string]bool{"dk1": true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := taxonomyPalette(nil, c.n, c.defaults)
			if len(got) != c.n {
				t.Fatalf("palette has %d entries, want %d", len(got), c.n)
			}
			seen := map[string]bool{}
			for _, tint := range got {
				seen[tint.scheme] = true
				if tint.ink != "accent1" {
					t.Errorf("title ink = %q, want the one accent", tint.ink)
				}
				if tint != nativeNeutralTintWithInk(patterns.NeutralTint4, "accent1") {
					t.Errorf("cell tint = %+v, want the neutral 4%% card", tint)
				}
			}
			for scheme := range seen {
				if !c.want[scheme] {
					t.Errorf("uses %q; the framework is limited to %v", scheme, keysOf(c.want))
				}
			}
			if len(seen) != 1 {
				t.Errorf("uses %d distinct surfaces (%v); the cards are one neutral", len(seen), keysOf(seen))
			}
		})
	}
}

// The native canvas draws the cell the bmc-canvas pattern draws: every
// section, the Value Proposition included, is the same neutral card.
func TestBMCSectionsShareOneSurface(t *testing.T) {
	want := nativeNeutralTintWithInk(patterns.NeutralTint4, "accent1")
	for i, key := range bmcSectionOrder {
		if got := bmcDefaultTint(i); got != want {
			t.Errorf("section %q tint = %+v, want %+v", key, got, want)
		}
	}
}

// SWOT's polarity is in its labels and grid by default; the two-accent tints
// are one style.colors away.
func TestSWOTPolarityIsOptIn(t *testing.T) {
	want := nativeNeutralTintWithInk(patterns.NeutralTint4, "accent1")
	for i := range 4 {
		if got := swotDefaultTint(i); got != want {
			t.Errorf("quadrant %d (%s) tint = %+v, want %+v", i, swotQuadrantColors[i].label, got, want)
		}
	}
	spec := &types.DiagramSpec{Type: "swot", Style: &types.DiagramStyle{Colors: swotPolarityColors}}
	// Order: Strengths, Weaknesses, Opportunities, Threats.
	negative := taxonomyTint{scheme: "accent2", lumMod: taxonomyLight.lumMod, lumOff: taxonomyLight.lumOff}
	polar := []taxonomyTint{taxonomyLight, negative, taxonomyLight, negative}
	for i, got := range taxonomyPalette(spec, 4, swotDefaultTint) {
		if got != polar[i] {
			t.Errorf("polarity quadrant %d tint = %+v, want %+v", i, got, polar[i])
		}
	}
}

// The title ink is the template's primary accent where it reads on the card,
// and a theme ink where it does not — the rule the patterns apply.
func TestNativeSurfaceTitleInkReadsOnTheCard(t *testing.T) {
	dark := nativeSurface{colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#111111"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"},
		{Name: "lt2", RGB: "#EEEEEE"}, {Name: "accent1", RGB: "#2B4C8C"},
	}}
	if got := dark.cardTint(1200).ink; got != "accent1" {
		t.Errorf("readable accent: title ink = %q, want accent1", got)
	}
	pale := nativeSurface{colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#111111"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"},
		{Name: "lt2", RGB: "#EEEEEE"}, {Name: "accent1", RGB: "#F5D76E"},
	}}
	if got := pale.cardTint(1200).ink; got == "accent1" || got == "" {
		t.Errorf("pale accent: title ink = %q, want a readable theme ink", got)
	}
	// style.colors wins over the neutral default, in cell order, repeating.
	authored := nativeSurface{authored: []taxonomyTint{{scheme: "accent3", lumMod: 20000, lumOff: 80000}}}
	if got := authored.tint(5, patterns.NeutralTint4, 1200); got.scheme != "accent3" || got.ink != "" {
		t.Errorf("authored tint = %+v, want the authored accent3 under the text role", got)
	}
}

func nativeNeutralTintWithInk(pct int, ink string) taxonomyTint {
	t := nativeNeutralTint(pct)
	t.ink = ink
	return t
}

// style.colors is the opt-in: it was ignored by these diagram types entirely,
// so an author who wants the old rainbow (or any other scheme) can ask for it.
func TestTaxonomyPaletteHonoursAuthoredColors(t *testing.T) {
	spec := &types.DiagramSpec{Style: &types.DiagramStyle{
		Colors: []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"},
	}}
	got := taxonomyPalette(spec, 6, uniformTaxonomyTint)
	for i, tint := range got {
		want := spec.Style.Colors[i]
		if tint.scheme != want {
			t.Errorf("cell %d scheme = %q, want the authored %q", i, tint.scheme, want)
		}
		if tint.lumMod != taxonomyLight.lumMod {
			t.Errorf("cell %d lost the cell tint: %+v", i, tint)
		}
	}

	// A short list repeats, so one colour recolours the whole framework.
	one := &types.DiagramSpec{Style: &types.DiagramStyle{Colors: []string{"accent4"}}}
	for i, tint := range taxonomyPalette(one, 9, bmcDefaultTint) {
		if tint.scheme != "accent4" {
			t.Errorf("cell %d scheme = %q, want the single authored colour repeated", i, tint.scheme)
		}
	}

	// An explicit hex is used at full strength — the author named that colour.
	hex := &types.DiagramSpec{Style: &types.DiagramStyle{Colors: []string{"#112233"}}}
	got = taxonomyPalette(hex, 2, uniformTaxonomyTint)
	if got[0].scheme != "#112233" || got[0].lumMod != 0 {
		t.Errorf("hex colour = %+v, want it used verbatim", got[0])
	}

	// Blank entries are not colours.
	blank := &types.DiagramSpec{Style: &types.DiagramStyle{Colors: []string{"  ", ""}}}
	if got := taxonomyPalette(blank, 3, uniformTaxonomyTint); got[0] != uniformTaxonomyTint(0) {
		t.Errorf("blank style.colors should fall back to the default, got %+v", got[0])
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Style is a pointer on DiagramSpec and most decks never set it, so the
// palette must not assume one is there.
func TestTaxonomyPaletteWithoutStyle(t *testing.T) {
	for _, spec := range []*types.DiagramSpec{
		nil,
		{Type: "swot"},
		{Type: "swot", Style: &types.DiagramStyle{}},
	} {
		got := taxonomyPalette(spec, 4, swotDefaultTint)
		if len(got) != 4 || got[0] != swotDefaultTint(0) || got[1] != swotDefaultTint(1) {
			t.Errorf("spec %+v gave %+v, want the SWOT defaults", spec, got)
		}
	}
}
