package svggen

import (
	"strings"
	"testing"
)

// pStylePalette mirrors a single-hue template: two saturated oranges in the
// first two accent slots (go-slide-creator-iry0p).
func pStylePalette() *Palette {
	p := DefaultPalette()
	for i, hex := range []string{"#FD5108", "#FE7C39", "#A1A8B3", "#000000", "#4E79A7", "#F28E2B"} {
		c := MustParseColor(hex)
		switch i {
		case 0:
			p.Accent1 = c
		case 1:
			p.Accent2 = c
		case 2:
			p.Accent3 = c
		case 3:
			p.Accent4 = c
		case 4:
			p.Accent5 = c
		case 5:
			p.Accent6 = c
		}
	}
	return p
}

func TestDistinctSeriesPalette_SeparatesSingleHueAccents(t *testing.T) {
	p := pStylePalette()
	if d := deltaE76(p.Accent1, p.Accent2); d >= MinSeriesDeltaE {
		t.Fatalf("fixture accents already distinct (ΔE %.1f)", d)
	}
	got := distinctSeriesPalette(p, 3)
	if got[0] != p.Accent1 {
		t.Errorf("series 0 = %s, want accent1 %s", got[0].Hex(), p.Accent1.Hex())
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < i; j++ {
			if d := deltaE76(got[i], got[j]); d < MinSeriesDeltaE {
				t.Errorf("series %d (%s) and %d (%s) ΔE %.1f < %.0f",
					i, got[i].Hex(), j, got[j].Hex(), d, MinSeriesDeltaE)
			}
		}
	}
	if len(got) != len(p.AccentColors()) {
		t.Errorf("palette length %d, want %d (near-duplicates deferred, not dropped)", len(got), len(p.AccentColors()))
	}
}

func TestDistinctSeriesPalette_FallsBackBeyondTemplateAccents(t *testing.T) {
	p := DefaultPalette()
	ramp := []string{"#AD84C6", "#8784C7", "#9A82C0", "#A088D0", "#9B7FC9", "#A58BCB", "#B08AC4"}
	p.Accent1, p.Accent2, p.Accent3 = MustParseColor(ramp[0]), MustParseColor(ramp[1]), MustParseColor(ramp[2])
	p.Accent4, p.Accent5, p.Accent6 = MustParseColor(ramp[3]), MustParseColor(ramp[4]), MustParseColor(ramp[5])
	p.ExtraAccents = []Color{MustParseColor(ramp[6])}
	p.TextSecondary = MustParseColor("#1F2A44")
	got := distinctSeriesPalette(p, 3)
	for i := 0; i < 3; i++ {
		for j := 0; j < i; j++ {
			if d := deltaE76(got[i], got[j]); d < MinSeriesDeltaE {
				t.Errorf("series %d (%s) and %d (%s) ΔE %.1f", i, got[i].Hex(), j, got[j].Hex(), d)
			}
		}
	}
	for i := 0; i < 3; i++ {
		if got[i] == p.TextSecondary || got[i] == p.TextPrimary {
			t.Errorf("series %d = %s: a text ink is never a series colour (go-slide-creator-orqni)", i, got[i].Hex())
		}
	}
}

// TestDistinctSeriesPalette_DefersSemanticAndNearBlack covers
// go-slide-creator-orqni: the template's semantic red / green and a
// near-black accent go behind every other usable colour, and the fallbacks
// never reach for text ink.
func TestDistinctSeriesPalette_DefersSemanticAndNearBlack(t *testing.T) {
	p := DefaultPalette()
	blue, red, amber, green := MustParseColor("#2E5090"), MustParseColor("#D4463A"), MustParseColor("#E8A838"), MustParseColor("#43A047")
	p.Accent1, p.Accent2, p.Accent3, p.Accent4 = blue, red, amber, green
	p.Accent5, p.Accent6 = MustParseColor("#26A69A"), MustParseColor("#5C6BC0")
	p.SeriesAvoid = []Color{green, red}
	got := distinctSeriesPalette(p, 3)
	for i := 0; i < 3; i++ {
		if got[i] == red || got[i] == green {
			t.Errorf("series %d painted the semantic %s; got %v", i, got[i].Hex(), got[:3])
		}
	}
	if got[0] != blue {
		t.Errorf("accent1 must lead, got %s", got[0].Hex())
	}

	ps := pStylePalette() // accent4 is #000000
	ps.TextPrimary = MustParseColor("#000000")
	ps.TextSecondary = MustParseColor("#000000")
	for n := 2; n <= 5; n++ {
		for i, c := range distinctSeriesPalette(ps, n)[:n] {
			if d := deltaE76(c, ps.TextPrimary); d < MinSeriesDeltaE {
				t.Errorf("n=%d series %d = %s is within ΔE %.1f of dk1", n, i, c.Hex(), d)
			}
		}
	}
}

func TestDistinctSeriesPalette_KeepsDistinctOrFullPalette(t *testing.T) {
	def := DefaultPalette()
	got := distinctSeriesPalette(def, 4)
	for i, c := range def.AccentColors() {
		if got[i] != c {
			t.Fatalf("well-separated palette reordered at %d: %s != %s", i, got[i].Hex(), c.Hex())
		}
	}
	p := pStylePalette()
	full := distinctSeriesPalette(p, 6)
	for i, c := range p.AccentColors() {
		if full[i] != c {
			t.Fatalf("all-slots chart reordered at %d: %s != %s", i, full[i].Hex(), c.Hex())
		}
	}
}

func TestRadarChart_OuterRingFollowsDataMax(t *testing.T) {
	// Scores peaking at 92 belong on a 0-100 web; padding the max by 10%
	// before rounding stretched the ring to 125 (go-slide-creator-iry0p).
	req := &RequestEnvelope{
		Type:  "radar_chart",
		Title: "Skills",
		Data: map[string]any{
			"categories": []any{"Frontend", "Backend", "DevOps", "Security", "Testing", "Architecture"},
			"series": []any{
				map[string]any{"name": "Team", "values": []any{85.0, 92.0, 78.0, 70.0, 88.0, 82.0}},
			},
		},
		Output: OutputSpec{Width: 1199, Height: 420},
	}
	result, err := RenderMultiFormat(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(result.SVG.Content)
	if !strings.Contains(svg, ">100<") {
		t.Errorf("outer ring label 100 missing")
	}
	if strings.Contains(svg, ">125<") {
		t.Errorf("outer ring stretched to 125 for a data max of 92")
	}
}
