package svggen

import (
	"math"
	"testing"
)

// ladderAccents are accent1 of every shipped template plus the private
// orange brand theme the ladder was designed on, and three awkward cases: a
// grey accent, a very dark one and a very light one.
var ladderAccents = map[string]string{
	"p-style":           "#FD5108",
	"midnight-blue":     "#2E5090",
	"warm-coral":        "#E64A19",
	"abstract":          "#8E8172",
	"forest-green":      "#2E7D32",
	"modern-template":   "#B5485A",
	"blue-corporate":    "#55BC7E",
	"business-template": "#AD84C6",
	"modern":            "#A53F51",
	"modern-yellow":     "#4472C4",
	"grey":              "#808080",
	"navy":              "#0B1F3A",
	"yellow":            "#FFC000",
}

func hueOf(c Color) float64 {
	h, _, _ := rgbToHSL(c.R, c.G, c.B)
	return h
}

func hueDistance(a, b float64) float64 {
	d := math.Abs(a - b)
	return math.Min(d, 360-d)
}

// TestTintKeepsHue pins the slide engine's tint family (go-slide-creator-7if28):
// a tint or shade keeps the accent's hue within a few degrees, and the "Lighter
// 80%" step of the orange brand accent is the patterns' peach, not salmon pink.
func TestTintKeepsHue(t *testing.T) {
	for name, hex := range ladderAccents {
		base := MustParseColor(hex)
		if _, s, _ := rgbToHSL(base.R, base.G, base.B); s < 0.05 {
			continue
		}
		for _, keep := range []float64{0.8, 0.6, 0.4, 0.2} {
			if d := hueDistance(hueOf(base), hueOf(base.Tint(keep))); d > 4 {
				t.Errorf("%s tint %.0f%% = %s: hue moved %.1f degrees", name, keep*100, base.Tint(keep).Hex(), d)
			}
		}
		if d := hueDistance(hueOf(base), hueOf(base.Shade(0.6))); d > 4 {
			t.Errorf("%s shade = %s: hue moved %.1f degrees", name, base.Shade(0.6).Hex(), d)
		}
	}
	// internal/patterns applyLumModOff(#FD5108, 20000, 80000) is #FFDCCE.
	got := MustParseColor("#FD5108").Tint(0.2)
	want := MustParseColor("#FFDCCE")
	if colorDistanceRGB(got, want) > 2 {
		t.Errorf("orange Lighter 80%% = %s, want the patterns' %s", got.Hex(), want.Hex())
	}
	if got.G <= got.B+8 {
		t.Errorf("orange tint %s is pink (green %d is not above blue %d)", got.Hex(), got.G, got.B)
	}
}

// TestTonalSeriesLadder pins what makes the ladder a ladder on every template
// accent: the solid accent leads, every slot is a visible mark, neighbours
// differ by a clear luminance step, accent steps keep the hue and neutral
// steps are grey.
func TestTonalSeriesLadder(t *testing.T) {
	white := MustParseColor("#FFFFFF")
	black := MustParseColor("#000000")
	for name, hex := range ladderAccents {
		t.Run(name, func(t *testing.T) {
			accent := MustParseColor(hex)
			ladder := TonalSeriesLadder(accent, white, black)
			if len(ladder) != SeriesLadderLen {
				t.Fatalf("ladder has %d slots, want %d", len(ladder), SeriesLadderLen)
			}
			if ladder[0] != accent {
				t.Errorf("slot 1 = %s, want the solid accent %s", ladder[0].Hex(), accent.Hex())
			}
			hexes := make([]string, len(ladder))
			for i, c := range ladder {
				hexes[i] = c.Hex()
			}
			t.Logf("%s: %v", name, hexes)
			_, sat, _ := rgbToHSL(accent.R, accent.G, accent.B)
			for i, c := range ladder {
				if i > 0 && c.ContrastWith(white) < 2 {
					t.Errorf("slot %d %s is %.2f:1 on the background, want >= 2", i+1, c.Hex(), c.ContrastWith(white))
				}
				// A grey accent has one family, so its ladder is an even grey
				// ramp: distinct, but without the two-family luminance step.
				chromatic := sat >= 0.15
				if i > 0 && chromatic {
					if step := c.ContrastWith(ladder[i-1]); step < seriesLadderStep-0.05 {
						t.Errorf("slots %d and %d (%s, %s) differ by %.2f:1 in luminance, want >= %.2f", i, i+1, ladder[i-1].Hex(), c.Hex(), step, seriesLadderStep)
					}
				}
				for j := 0; j < i; j++ {
					minDist := MinSeriesDeltaE / 2
					if !chromatic {
						minDist = 4
					}
					if d := deltaE76(c, ladder[j]); d < minDist {
						t.Errorf("slots %d and %d (%s, %s) are %.1f apart, one colour to the eye", j+1, i+1, ladder[j].Hex(), c.Hex(), d)
					}
				}
				_, s, _ := rgbToHSL(c.R, c.G, c.B)
				switch i {
				case 1, 2, 5:
					if chromatic && s > 0.02 {
						t.Errorf("slot %d %s should be neutral grey", i+1, c.Hex())
					}
				case 3, 4:
					if chromatic && hueDistance(hueOf(c), hueOf(accent)) > 4 {
						t.Errorf("slot %d %s left the accent's hue", i+1, c.Hex())
					}
				}
			}
		})
	}
}

// TestTonalSeriesLadderOnDarkBackground: on a dark canvas the steps move away
// from the background the other way, and stay visible.
func TestTonalSeriesLadderOnDarkBackground(t *testing.T) {
	bg := MustParseColor("#101820")
	ink := MustParseColor("#FFFFFF")
	ladder := TonalSeriesLadder(MustParseColor("#FD5108"), bg, ink)
	for i, c := range ladder {
		if c.ContrastWith(bg) < 2 {
			t.Errorf("slot %d %s is %.2f:1 on the dark background", i+1, c.Hex(), c.ContrastWith(bg))
		}
		if i > 0 && c.ContrastWith(ladder[i-1]) < seriesLadderStep-0.05 {
			t.Errorf("slots %d and %d differ by %.2f:1", i, i+1, c.ContrastWith(ladder[i-1]))
		}
	}
}
