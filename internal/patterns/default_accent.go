package patterns

import (
	"slices"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// The default accent a pattern paints with when the author names none is the
// template's primary fill, the same slot list_templates reports as
// color_roles.primary_fill (go-slide-creator-2mia4). On templates whose
// accent1 carries white text it is accent1 and nothing changes; on templates
// where white fails on accent1 (warm-coral, business-template,
// blue-corporate) patterns default to the first slot that can carry it.

// PrimaryFillCandidates lists the theme slots that can carry white text, best
// first: accents passing 4.5:1 against white, then accents passing 3:1, then
// dk2 / dk1 when they pass 3:1. It is the single source of
// color_roles.primary_fill / secondary_fill and of DefaultAccent.
func PrimaryFillCandidates(colors []types.ThemeColor) []string {
	white := svggen.MustParseColor("#FFFFFF")
	ratio := func(name string) (float64, bool) {
		for _, c := range colors {
			if c.Name != name {
				continue
			}
			parsed, err := svggen.ParseColor(c.RGB)
			if err != nil {
				return 0, false
			}
			return parsed.ContrastWith(white), true
		}
		return 0, false
	}
	var body, large []string
	for slot := 1; slot <= NumAccentSlots; slot++ {
		name := accentSlotName(slot)
		r, ok := ratio(name)
		if !ok {
			continue
		}
		if r >= svggen.WCAGAANormal {
			body = append(body, name)
		} else if r >= svggen.WCAGAALarge {
			large = append(large, name)
		}
	}
	candidates := append(body, large...)
	for _, name := range []string{"dk2", "dk1"} {
		if r, ok := ratio(name); ok && r >= svggen.WCAGAALarge && !slices.Contains(candidates, name) {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

// PrimaryFill is the template's primary fill: the first PrimaryFillCandidates
// slot, or accent1 when the theme is unknown or no slot qualifies.
func PrimaryFill(colors []types.ThemeColor) string {
	if c := PrimaryFillCandidates(colors); len(c) > 0 {
		return c[0]
	}
	return "accent1"
}

// DefaultAccent is the accent a pattern uses when neither an explicit accent,
// a semantic_accent nor a rotating / section-keyed strategy chooses one: the
// template's primary fill (accent1 when the theme is unknown).
func (c ExpandContext) DefaultAccent() string {
	return PrimaryFill(c.Theme.Colors)
}

func accentSlotName(slot int) string {
	return "accent" + string(rune('0'+slot))
}
