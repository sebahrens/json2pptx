package main

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// skillInk names the readable text colour on an accent fill and its WCAG
// contrast ratio (go-slide-creator-2mia4).
type skillInk struct {
	Ink   string  `json:"ink"`   // scheme colour for text on this fill: lt1, dk2 or dk1
	Ratio float64 `json:"ratio"` // contrast of Ink on the accent, rounded to 0.1
}

var guideAccents = []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}

// buildInkOnAccent picks, per accent, the text colour a fill of that accent
// can carry: lt1 (white type) when it passes 4.5:1, else dk2 then dk1 when
// they do, else whichever of the three contrasts most. Patterns default to
// accent1, which fails white text on abstract, warm-coral, blue-corporate,
// business-template and p-style; this map tells an agent what ink to set.
func buildInkOnAccent(colors []types.ThemeColor) map[string]skillInk {
	out := make(map[string]skillInk, len(guideAccents))
	for _, name := range guideAccents {
		fill, err := svggen.ParseColor(findColorHex(colors, name))
		if err != nil {
			continue
		}
		var best skillInk
		for _, ink := range []string{"lt1", "dk2", "dk1"} {
			c, err := svggen.ParseColor(findColorHex(colors, ink))
			if err != nil {
				continue
			}
			r := math.Round(fill.ContrastWith(c)*10) / 10
			if r >= svggen.WCAGAANormal {
				best = skillInk{Ink: ink, Ratio: r}
				break
			}
			if r > best.Ratio {
				best = skillInk{Ink: ink, Ratio: r}
			}
		}
		if best.Ink != "" {
			out[name] = best
		}
	}
	return out
}

// deriveAccentUsageGuide synthesises a one-line role per accent from the
// theme's measured contrast when the template ships no authored
// accent_usage_guide (a bring-your-own template such as p-style has no
// metadata part at all). It states only what the colours prove: which fill
// carries white body / large text, which accent is too close to the canvas
// for text or thin lines, and which accents are the readable primary /
// secondary fills (go-slide-creator-u1h9b).
func deriveAccentUsageGuide(colors []types.ThemeColor, roles *skillColorRoles) map[string]string {
	if roles == nil {
		return nil
	}
	ink := buildInkOnAccent(colors)
	bg, bgErr := svggen.ParseColor(findColorHex(colors, "lt1"))
	out := make(map[string]string, len(guideAccents))
	for _, name := range guideAccents {
		hex := strings.TrimPrefix(findColorHex(colors, name), "#")
		c, err := svggen.ParseColor(hex)
		if err != nil {
			continue
		}
		var role string
		switch name {
		case roles.PrimaryFill:
			role = "primary fill — headers, bands, key callouts"
		case roles.SecondaryFill:
			role = "secondary fill — a second category or supporting emphasis"
		}
		onCanvas := 0.0
		if bgErr == nil {
			onCanvas = math.Round(c.ContrastWith(bg)*10) / 10
		}
		var use string
		switch {
		case slices.Contains(roles.NearBackgroundAccents, name):
			use = fmt.Sprintf("near-background tint (%.1f:1 on lt1): subtle surfaces only, never text or thin lines", onCanvas)
		case slices.Contains(roles.WhiteTextSafeBody, name):
			use = fmt.Sprintf("carries white body text (%.1f:1)", ink[name].Ratio)
		case slices.Contains(roles.WhiteTextSafeLarge, name):
			use = fmt.Sprintf("white text only at 18pt+ or bold; set %s ink for body text on it (%.1f:1)", ink[name].Ink, ink[name].Ratio)
		default:
			use = fmt.Sprintf("fails white text; set %s ink on it (%.1f:1); as a line or text colour on lt1 it reads at %.1f:1", ink[name].Ink, ink[name].Ratio, onCanvas)
		}
		if role != "" {
			out[name] = fmt.Sprintf("%s (#%s): %s; %s", name, hex, role, use)
		} else {
			out[name] = fmt.Sprintf("%s (#%s): %s", name, hex, use)
		}
	}
	return out
}
