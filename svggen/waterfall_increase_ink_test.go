package svggen

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestWaterfallIncreaseInk_LegibleOnEveryTemplate is the go-slide-creator-rmm0x
// acceptance test: on every bundled template (and the local p-style when
// present) a waterfall's increase bars keep 3:1 against the background and
// the increase / decrease / total fills stay at least MinSeriesDeltaE apart.
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
		total := NeutralInk(p, WaterfallTotalInk)
		dec := p.Accent1
		inc := waterfallIncreaseInk(p, total, dec)
		bg := p.Background
		if bg.A < 1 {
			bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
		}
		name := filepath.Base(f)
		if c := inc.ContrastWith(bg); c < waterfallIncreaseMinContrast {
			t.Errorf("%s: increase %s has %.2f:1 against %s, want >= 3", name, inc.Hex(), c, bg.Hex())
		}
		if d := deltaE76(inc, dec); d < MinSeriesDeltaE {
			t.Errorf("%s: increase %s vs decrease %s ΔE %.1f", name, inc.Hex(), dec.Hex(), d)
		}
		if d := deltaE76(inc, total); d < MinSeriesDeltaE {
			t.Errorf("%s: increase %s vs total %s ΔE %.1f", name, inc.Hex(), total.Hex(), d)
		}
	}
}
