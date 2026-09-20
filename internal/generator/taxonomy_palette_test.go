package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-w0kj: the taxonomy frameworks used to colour every cell with
// a different theme accent. These tests pin the replacement contract — one hue
// by default, a second only where the framework encodes a real contrast, and
// the old behaviour reachable through style.colors.

func TestTaxonomyPaletteDefaultsToOneHue(t *testing.T) {
	for _, c := range []struct {
		name     string
		n        int
		defaults func(int) taxonomyTint
		want     map[string]bool
	}{
		{"bmc", len(bmcSectionOrder), bmcDefaultTint, map[string]bool{"accent1": true}},
		{"pestel", 6, uniformTaxonomyTint, map[string]bool{"accent1": true}},
		{"swot", 4, swotDefaultTint, map[string]bool{"accent1": true, "accent2": true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := taxonomyPalette(nil, c.n, c.defaults)
			if len(got) != c.n {
				t.Fatalf("palette has %d entries, want %d", len(got), c.n)
			}
			seen := map[string]bool{}
			for _, tint := range got {
				seen[tint.scheme] = true
			}
			for scheme := range seen {
				if !c.want[scheme] {
					t.Errorf("uses %q; the framework is limited to %v", scheme, keysOf(c.want))
				}
			}
			if len(seen) > 2 {
				t.Errorf("uses %d distinct accents (%v); at most two are meaningful", len(seen), keysOf(seen))
			}
		})
	}
}

// The BMC's one privileged cell is the Value Proposition — every other block
// exists to explain it — so it is the only one carried a step deeper.
func TestBMCDeepensOnlyTheValueProposition(t *testing.T) {
	for i, key := range bmcSectionOrder {
		got := bmcDefaultTint(i)
		if key == bmcValueProposition {
			if got != taxonomyDeep {
				t.Errorf("value proposition tint = %+v, want the deeper one", got)
			}
			continue
		}
		if got != taxonomyLight {
			t.Errorf("section %q tint = %+v, want the standard light one", key, got)
		}
	}
}

// SWOT's negative column is the one place a second accent means something.
func TestSWOTSplitsPositiveFromNegative(t *testing.T) {
	// Order: Strengths, Weaknesses, Opportunities, Threats.
	want := []taxonomyTint{taxonomyLight, taxonomyNegative, taxonomyLight, taxonomyNegative}
	for i, w := range want {
		if got := swotDefaultTint(i); got != w {
			t.Errorf("quadrant %d (%s) tint = %+v, want %+v", i, swotQuadrantColors[i].label, got, w)
		}
	}
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
	if got := taxonomyPalette(blank, 3, uniformTaxonomyTint); got[0] != taxonomyLight {
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
		if len(got) != 4 || got[0] != taxonomyLight || got[1] != taxonomyNegative {
			t.Errorf("spec %+v gave %+v, want the SWOT defaults", spec, got)
		}
	}
}
