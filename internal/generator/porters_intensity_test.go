package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func portersTestTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "lt2", RGB: "E8ECF1"}, {Name: "dk2", RGB: "1B2A4A"},
		{Name: "accent1", RGB: "2E5090"}, {Name: "accent3", RGB: "E8A838"},
		{Name: "accent5", RGB: "5C6BC0"},
	}
}

// A five-forces payload with no intensity used to default to 0.5, so every
// unscored force printed "Medium (50%)" in the accent3 tint: a chart that
// looked like an assessment and was a default (go-slide-creator-ceodq).
func TestPorterUnscoredForceAssertsNothing(t *testing.T) {
	unscored := porterForceFromMap(porterRivalry, map[string]any{"factors": []any{"Three clearers"}})
	if unscored.intensity != nil {
		t.Errorf("an unstated intensity became %v", *unscored.intensity)
	}
	scored := porterForceFromMap(porterRivalry, map[string]any{"intensity": 0.85})
	if scored.intensity == nil || *scored.intensity != 0.85 {
		t.Fatalf("a stated intensity was lost: %v", scored.intensity)
	}

	// The box for an unscored force carries no intensity line and no
	// colour-coded fill.
	xml := generatePorterForceBoxXML(unscored, 0, 0, 2000000, 1000000, 10, false, portersTestTheme())
	for _, unwanted := range []string{"Medium", "50%", "(0%)"} {
		if strings.Contains(xml, unwanted) {
			t.Errorf("an unscored force still prints %q:\n%s", unwanted, xml)
		}
	}
	if !strings.Contains(xml, porterNeutralScheme) {
		t.Errorf("an unscored force is not drawn on the neutral surface:\n%s", xml)
	}
	// LumMod(0) is 0% luminance — black — so the neutral fill must carry no
	// modifiers at all.
	if strings.Contains(xml, `<a:lumMod val="0"/>`) {
		t.Errorf("the neutral fill emitted a zero lumMod, which renders black:\n%s", xml)
	}

	scoredXML := generatePorterForceBoxXML(scored, 0, 0, 2000000, 1000000, 10, false, portersTestTheme())
	if !strings.Contains(scoredXML, "High (85%)") {
		t.Errorf("a scored force lost its intensity line:\n%s", scoredXML)
	}
}

// The intensity line used to be painted in the box's own scheme colour on the
// box's own tint of it — 1.55:1 on the accent3 tile in a real render.
func TestPorterIntensityTextIsPickedByMeasurement(t *testing.T) {
	theme := portersTestTheme()
	for _, tc := range []struct {
		name      string
		intensity float64
	}{
		{name: "high", intensity: 0.85},
		{name: "medium", intensity: 0.5},
		{name: "low", intensity: 0.15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme, lumMod, lumOff := porterIntensityColor(&tc.intensity)
			got := porterIntensityTextColor(scheme, lumMod, lumOff, theme)
			if got == scheme {
				t.Errorf("intensity text takes the fill's own colour (%q) — the bug this replaced", got)
			}
			if got != "lt1" && got != "dk2" {
				t.Errorf("intensity text colour = %q, want a readable text role", got)
			}
		})
	}
	// Without a theme there is nothing to measure and the historical colour
	// stands rather than a guess.
	if got := porterIntensityTextColor("accent3", 40000, 60000, nil); got != "accent3" {
		t.Errorf("with no theme, colour = %q, want the historical accent3", got)
	}
}
