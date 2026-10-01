package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func themeWith(accents ...string) types.ThemeInfo {
	colors := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#1F2A44"}, {Name: "lt2", RGB: "#EEEEEE"},
	}
	for i, hex := range accents {
		colors = append(colors, types.ThemeColor{Name: accentSlotName(i + 1), RGB: hex})
	}
	return types.ThemeInfo{Colors: colors}
}

// TestDefaultAccent pins go-slide-creator-2mia4: a pattern's default accent is
// the template's primary fill — accent1 wherever white text reads on it, else
// the first accent that carries white text, else dk2 — and accent1 when the
// theme is unknown.
func TestDefaultAccent(t *testing.T) {
	cases := []struct {
		name  string
		theme types.ThemeInfo
		want  string
	}{
		{"no theme", types.ThemeInfo{}, "accent1"},
		{"accent1 body-safe", themeWith("#1F4E79", "#2E7D32", "#C00000", "#7030A0", "#404040", "#005A9E"), "accent1"},
		// warm-coral's shape: accent1 passes only the 3:1 large-text bar,
		// accent2 passes 4.5:1 — the body-safe slot wins.
		{"accent1 large-only, accent2 body-safe", themeWith("#E8603C", "#1F4E79", "#FFAA72", "#A1A8B3", "#2E7D32", "#CBD1D6"), "accent2"},
		// p-style / abstract: no body-safe accent, accent1 passes 3:1.
		{"accent1 large-only, none body-safe", themeWith("#FD5108", "#FE7C39", "#FFAA72", "#A1A8B3", "#B5BCC4", "#CBD1D6"), "accent1"},
		// blue-corporate: every accent pastel — the dark brand slot.
		{"all pastel", themeWith("#9DC3E6", "#BDD7EE", "#DEEBF7", "#C5E0B4", "#FFE699", "#F8CBAD"), "dk2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ExpandContext{Theme: tc.theme}
			if got := ctx.DefaultAccent(); got != tc.want {
				t.Errorf("DefaultAccent = %q, want %q", got, tc.want)
			}
			// The primary strategy (and an unset one) resolves to it; an
			// explicit accent still wins.
			if got := ctx.ResolveAccent("", ""); got != tc.want {
				t.Errorf("ResolveAccent(primary) = %q, want %q", got, tc.want)
			}
			ctx.AccentStrategy = AccentStrategyPrimary
			if got := ctx.ResolveAccent("", ""); got != tc.want {
				t.Errorf("ResolveAccent(primary) = %q, want %q", got, tc.want)
			}
			if got := ctx.ResolveAccent("accent4", ""); got != "accent4" {
				t.Errorf("explicit accent overridden: %q", got)
			}
			ctx.AccentStrategy = AccentStrategySectionKeyed
			ctx.SectionIndex = 2
			if got := ctx.ResolveAccent("", ""); got != "accent3" {
				t.Errorf("section-keyed = %q, want accent3", got)
			}
		})
	}
}
