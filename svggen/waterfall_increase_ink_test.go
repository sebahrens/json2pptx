package svggen

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestWaterfallIncreaseInk_LegibleOnEveryTemplate keeps the waterfall's three
// bar classes apart on every bundled template (and the local p-style when
// present): the decrease tint stays a visible bar on the background, and the
// increase (accent1) stays at least MinSeriesDeltaE from the decrease (its
// tint) and the total (neutral) (go-slide-creator-rmm0x, go-slide-creator-n978t).
func TestWaterfallIncreaseInk_LegibleOnEveryTemplate(t *testing.T) {
	files, err := filepath.Glob(chartPaletteTemplatesGlob)
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates under %s: %v", chartPaletteTemplatesGlob, err)
	}
	sort.Strings(files)
	for _, f := range files {
		themeColors, _, _, err := loadChartPaletteFromTemplate(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		guide := StyleGuideFromSpec(StyleSpec{ThemeColors: themeColors, DisablePaletteEnforcement: true, Background: "transparent"})
		p := guide.Palette
		total := waterfallTotalInk(p)
		inc := p.Accent1
		dec := waterfallDecreaseInk(p, total)
		bg := p.Background
		if bg.A < 1 {
			bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
		}
		name := filepath.Base(f)
		if c := dec.ContrastWith(bg); c < waterfallDecreaseMinContrast {
			t.Errorf("%s: decrease %s has %.2f:1 against %s, want >= %.1f", name, dec.Hex(), c, bg.Hex(), waterfallDecreaseMinContrast)
		}
		if d := deltaE76(inc, dec); d < MinSeriesDeltaE {
			t.Errorf("%s: increase %s vs decrease %s ΔE %.1f", name, inc.Hex(), dec.Hex(), d)
		}
		if d := deltaE76(inc, total); d < MinSeriesDeltaE {
			t.Errorf("%s: increase %s vs total %s ΔE %.1f", name, inc.Hex(), total.Hex(), d)
		}
	}
}
