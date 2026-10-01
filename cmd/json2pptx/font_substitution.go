package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// themeFontSubstitutionWarnings reports template theme fonts that are not
// installed on this host. Fit measurement then falls back to another face and
// LibreOffice renders substitute their own (often wider) fallback, so a line
// that fits in measurement can wrap in a render with no finding explaining why
// (go-slide-creator-csclk.95). Arial is exempt: it resolves to the embedded,
// metric-compatible Liberation Sans; Calibri / Cambria resolve to their
// metric clones Carlito / Caladea when present (go-slide-creator-tl7vf).
func themeFontSubstitutionWarnings(theme types.ThemeInfo) []string {
	var out []string
	seen := map[string]bool{}
	for _, font := range []string{theme.TitleFont, theme.BodyFont} {
		if font == "" || font == "Arial" || seen[font] {
			continue
		}
		seen[font] = true
		ff, resolved, substituted := fontcache.Resolve(font, "Arial")
		if ff == nil || !substituted {
			continue
		}
		out = append(out, fmt.Sprintf(
			"FONT_SUBSTITUTED: template theme font %q is not installed on this host; fit measurement uses %q and LibreOffice renders substitute their own fallback, so text in render_* images may wrap differently than fit findings predict. Install %q (or a metric-compatible font) for faithful measurement and renders.",
			font, resolved, font))
	}
	return out
}
