package template_test

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// requiredSchemeColors is the canonical set of 12 OOXML theme scheme color
// slots. Every shipped template MUST define all 12 so that engine code can
// resolve any semantic color reference without falling back to defaults.
var requiredSchemeColors = []string{
	"dk1", "dk2", "lt1", "lt2",
	"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
	"hlink", "folHlink",
}

// surfaceTintRoles lists the 4 surface tint roles the engine expects.
// See internal/patterns/overrides.go ValidSurfaceTintRoles and
// docs/TEMPLATE_SPEC.md.
var surfaceTintRoles = []string{"subtle", "paper", "elevated", "inverse"}

// validSchemeColorTargets is the set of scheme color names that
// SurfaceTints and DataPalette entries are allowed to reference. It is a
// superset of requiredSchemeColors because the OOXML spec also recognises
// the bg1/bg2/tx1/tx2 aliases.
var validSchemeColorTargets = map[string]bool{
	"dk1": true, "dk2": true, "lt1": true, "lt2": true,
	"bg1": true, "bg2": true, "tx1": true, "tx2": true,
	"accent1": true, "accent2": true, "accent3": true,
	"accent4": true, "accent5": true, "accent6": true,
	"hlink": true, "folHlink": true,
}

// schemeAliasToTheme normalises OOXML scheme color aliases (bg1/bg2/tx1/tx2)
// to the theme color slots actually present in the parsed ThemeInfo
// (dk1/dk2/lt1/lt2). Used when resolving a placeholder font color or
// background reference to a hex value.
var schemeAliasToTheme = map[string]string{
	"bg1": "lt1",
	"bg2": "lt2",
	"tx1": "dk1",
	"tx2": "dk2",
}

// themeCorpusKnownBroken records template+gate pairs that are scheduled for
// repair. Each entry logs its failures (so they remain visible in CI output)
// without failing the test. Removing an entry re-enables the gate; an entry
// that no longer fails produces a "stale allow-list" error pointing at the
// entry, which catches drift after a repair lands.
//
// The category MUST match the string passed to reportTemplateFailures so a
// template can be exempt from one gate without silencing the others.
//
// Templates listed here are also tracked in
// testdata/conformance_allowlist.json under their respective vqad child
// issues — when the layout repairs land, the theme defects exposed by these
// gates are repaired in the same commit.
var themeCorpusKnownBroken = []themeCorpusException{
	{
		// blue-corporate.pptx: explicit white text on accent6 (#60A2F5) light
		// blue background falls below WCAG AA Large 3.0:1. Repair will adjust
		// either the layout background or the text color.
		Template: "blue-corporate.pptx",
		Category: "layout contrast",
		Tracking: "go-slide-creator-pxdp",
	},
	{
		// modern-template.pptx: Section Divider body placeholder color="tx1"
		// against a layout background that resolves to the same theme dk1,
		// giving 1.00 contrast. Repair will fix the layout's body color.
		Template: "modern-template.pptx",
		Category: "layout contrast",
		Tracking: "go-slide-creator-iy2k",
	},
}

// themeCorpusException allow-lists a single (template, gate) pair.
type themeCorpusException struct {
	Template string
	Category string
	Tracking string
}

// lookupException returns the tracking issue for a known-broken
// (template, category) pair, or empty string if the pair is not on the
// allow-list.
func lookupException(template, category string) string {
	for _, e := range themeCorpusKnownBroken {
		if e.Template == template && e.Category == category {
			return e.Tracking
		}
	}
	return ""
}

// TestThemeCorpus_AllSchemeColorsResolveWithCorrectPolarity asserts that
// every shipped template defines all 12 scheme color slots, that each
// resolves to a valid #RRGGBB hex value, and that the dark slots
// (dk1, dk2) carry luminance below 50% while the light slots (lt1, lt2)
// carry luminance above 50%. A polarity inversion (e.g. dk1=#FFFFFF) makes
// the template unusable because every contrast-fix path picks the wrong
// extreme.
//
// Acceptance criteria source: bd go-slide-creator-cla4.
func TestThemeCorpus_AllSchemeColorsResolveWithCorrectPolarity(t *testing.T) {
	forEachTemplate(t, func(t *testing.T, name string, reader *template.Reader) {
		theme := template.ParseTheme(reader)
		colorMap := buildColorMap(theme)

		var failures []string

		for _, slot := range requiredSchemeColors {
			rgb, ok := colorMap[slot]
			if !ok {
				failures = append(failures,
					"missing scheme color "+slot+" (expected #RRGGBB hex value in theme1.xml)")
				continue
			}
			if !isValidHexRGB(rgb) {
				failures = append(failures,
					"scheme color "+slot+"="+rgb+" is not a valid #RRGGBB hex value")
				continue
			}
			if expectsDark(slot) {
				lum := relativeLuminance(rgb)
				if lum >= 0.5 {
					failures = append(failures,
						polarityFailure(slot, rgb, lum, "dark", "< 50%"))
				}
			}
			if expectsLight(slot) {
				lum := relativeLuminance(rgb)
				if lum < 0.5 {
					failures = append(failures,
						polarityFailure(slot, rgb, lum, "light", "> 50%"))
				}
			}
		}

		reportTemplateFailures(t, name, "scheme colors", failures)
	})
}

// TestThemeCorpus_SurfaceTintsReferenceValidSchemeColors asserts that, for
// every template carrying TemplateMetadata.SurfaceTints, each declared role
// (subtle, paper, elevated, inverse) points at a recognised scheme color
// name. Templates without metadata are skipped — runtime fallback defaults
// are intrinsically valid.
//
// Acceptance criteria source: bd go-slide-creator-cla4.
func TestThemeCorpus_SurfaceTintsReferenceValidSchemeColors(t *testing.T) {
	forEachTemplate(t, func(t *testing.T, name string, reader *template.Reader) {
		metadata, err := template.ParseMetadata(reader)
		if err != nil {
			t.Fatalf("%s: ParseMetadata: %v", name, err)
		}
		if metadata == nil || len(metadata.SurfaceTints) == 0 {
			t.Skipf("%s: no surface_tints declared in metadata", name)
		}

		theme := template.ParseTheme(reader)
		colorMap := buildColorMap(theme)

		var failures []string

		// Every declared role MUST point at a recognised scheme color name
		// that exists in the theme.
		for role, schemeName := range metadata.SurfaceTints {
			if !knownSurfaceTintRole(role) {
				failures = append(failures,
					"surface_tint role "+role+" is not one of subtle/paper/elevated/inverse")
				continue
			}
			if !validSchemeColorTargets[schemeName] {
				failures = append(failures,
					"surface_tint["+role+"]="+schemeName+" is not a recognised scheme color name")
				continue
			}
			if !resolvesToTheme(schemeName, colorMap) {
				failures = append(failures,
					"surface_tint["+role+"]="+schemeName+" does not resolve to any scheme color present in the theme")
			}
		}

		// All 4 expected roles SHOULD be declared so every pattern path has
		// a template-provided tint rather than a generic fallback.
		for _, role := range surfaceTintRoles {
			if _, ok := metadata.SurfaceTints[role]; !ok {
				failures = append(failures,
					"surface_tint role "+role+" is missing from metadata (expected one of the 4 roles)")
			}
		}

		reportTemplateFailures(t, name, "surface tints", failures)
	})
}

// TestThemeCorpus_DataPaletteHasSixValidSchemeColors asserts that, for every
// template carrying TemplateMetadata.DataPalette, the palette contains
// exactly 6 entries (one per accent slot) and every entry resolves to a
// recognised scheme color present in the theme. svggen chart series rely on
// this ordering — a missing or invalid name silently substitutes a
// fallback palette color and breaks the template's visual identity.
//
// Acceptance criteria source: bd go-slide-creator-cla4.
func TestThemeCorpus_DataPaletteHasSixValidSchemeColors(t *testing.T) {
	forEachTemplate(t, func(t *testing.T, name string, reader *template.Reader) {
		metadata, err := template.ParseMetadata(reader)
		if err != nil {
			t.Fatalf("%s: ParseMetadata: %v", name, err)
		}
		if metadata == nil || len(metadata.DataPalette) == 0 {
			t.Skipf("%s: no data_palette declared in metadata", name)
		}

		theme := template.ParseTheme(reader)
		colorMap := buildColorMap(theme)

		var failures []string

		if len(metadata.DataPalette) != 6 {
			failures = append(failures,
				"data_palette has "+itoa(len(metadata.DataPalette))+" entries (expected 6 — one per accent slot)")
		}

		seen := make(map[string]bool, len(metadata.DataPalette))
		for i, schemeName := range metadata.DataPalette {
			pos := "data_palette[" + itoa(i) + "]"
			if !validSchemeColorTargets[schemeName] {
				failures = append(failures,
					pos+"="+schemeName+" is not a recognised scheme color name")
				continue
			}
			if !resolvesToTheme(schemeName, colorMap) {
				failures = append(failures,
					pos+"="+schemeName+" does not resolve to any scheme color present in the theme")
			}
			if seen[schemeName] {
				failures = append(failures,
					pos+"="+schemeName+" is duplicated earlier in the palette")
			}
			seen[schemeName] = true
		}

		reportTemplateFailures(t, name, "data palette", failures)
	})
}

// TestThemeCorpus_LayoutContrastMeetsWCAGAALarge asserts that, for every
// layout that authors an explicit background AND resolves a title/body
// placeholder foreground color, the contrast ratio meets WCAG AA Large
// (3.0:1). The renderer's contrast pass auto-fixes failures at render
// time, but a template whose static colors already pass needs no fixing —
// catching breakage here keeps the auto-fixer for genuine edge cases.
//
// Layouts that inherit background from the master, and placeholders whose
// font color is inherited (empty string from the font resolver), are
// skipped — there is no static fact to assert.
//
// Acceptance criteria source: bd go-slide-creator-cla4.
func TestThemeCorpus_LayoutContrastMeetsWCAGAALarge(t *testing.T) {
	forEachTemplate(t, func(t *testing.T, name string, reader *template.Reader) {
		theme := template.ParseTheme(reader)
		colorMap := buildColorMap(theme)

		layouts, err := template.ParseLayouts(reader)
		if err != nil {
			t.Fatalf("%s: ParseLayouts: %v", name, err)
		}

		layoutFiles, err := reader.ListFiles("ppt/slideLayouts/slideLayout*.xml")
		if err != nil {
			t.Fatalf("%s: list layouts: %v", name, err)
		}
		sort.Strings(layoutFiles)
		if len(layoutFiles) != len(layouts) {
			t.Fatalf("%s: layout count mismatch — files=%d layouts=%d", name, len(layoutFiles), len(layouts))
		}

		var failures []string
		checked := 0

		for i, l := range layouts {
			bgHex := extractLayoutBackground(reader, layoutFiles[i], colorMap)
			if bgHex == "" {
				continue
			}

			for _, p := range l.Placeholders {
				if p.Type != types.PlaceholderTitle && p.Type != types.PlaceholderBody {
					continue
				}
				if p.FontColor == "" {
					continue
				}
				fgHex := resolveFontColor(p.FontColor, colorMap)
				if fgHex == "" {
					continue
				}
				checked++

				ratio, ok := contrastRatio(fgHex, bgHex)
				if !ok {
					continue
				}
				if ratio < svggen.WCAGAALarge {
					failures = append(failures,
						"layout "+l.Name+" placeholder "+p.ID+
							" ("+string(p.Type)+"): "+fgHex+" on "+bgHex+
							" has contrast "+ratioStr(ratio)+
							" (< WCAG AA Large 3.0:1)")
				}
			}
		}

		t.Logf("%s: checked %d title/body placeholder pairs across %d layouts", name, checked, len(layouts))
		reportTemplateFailures(t, name, "layout contrast", failures)
	})
}

// =============================================================================
// Test helpers
// =============================================================================

// forEachTemplate iterates every templates/*.pptx file, opens it as a Reader,
// and calls fn under a subtest named after the file basename.
func forEachTemplate(t *testing.T, fn func(t *testing.T, name string, reader *template.Reader)) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)
	for _, f := range files {
		f := f
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(f)
			if err != nil {
				t.Fatalf("OpenTemplate(%s): %v", name, err)
			}
			defer func() { _ = reader.Close() }()
			fn(t, name, reader)
		})
	}
}

// reportTemplateFailures emits the collected per-template failure messages.
// A (template, category) pair listed in themeCorpusKnownBroken logs its
// failures (so they stay visible in CI output) without failing the test;
// repair causes removal from the list, which re-enables enforcement. A
// pair that no longer fails but is still allow-listed produces a stale-
// entry error pointing at the entry to delete.
func reportTemplateFailures(t *testing.T, name, category string, failures []string) {
	t.Helper()
	tracking := lookupException(name, category)
	if tracking != "" {
		if len(failures) == 0 {
			t.Errorf("%s is on the theme-corpus allow-list for %s (tracking %s) but now passes — remove the matching entry from themeCorpusKnownBroken in theme_corpus_test.go",
				name, category, tracking)
			return
		}
		t.Logf("%s allow-listed for %s under %s — %d failures (logged, not failed):\n%s",
			name, category, tracking, len(failures), formatFailures(failures))
		return
	}
	if len(failures) == 0 {
		return
	}
	t.Errorf("%s failed %s gate with %d issue(s):\n%s",
		name, category, len(failures), formatFailures(failures))
}

func formatFailures(failures []string) string {
	var b strings.Builder
	for _, f := range failures {
		b.WriteString("  - ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}

// buildColorMap converts ThemeInfo.Colors into a name→hex lookup map.
func buildColorMap(theme types.ThemeInfo) map[string]string {
	m := make(map[string]string, len(theme.Colors))
	for _, c := range theme.Colors {
		m[c.Name] = c.RGB
	}
	return m
}

// resolvesToTheme returns true if schemeName (or its alias) maps to a slot
// present in colorMap.
func resolvesToTheme(schemeName string, colorMap map[string]string) bool {
	canonical := schemeName
	if alias, ok := schemeAliasToTheme[schemeName]; ok {
		canonical = alias
	}
	_, ok := colorMap[canonical]
	return ok
}

// resolveFontColor turns a placeholder font color reference (scheme name like
// "tx1" or a #RRGGBB hex) into a normalised #RRGGBB hex string, or empty if
// it cannot be resolved.
func resolveFontColor(ref string, colorMap map[string]string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "#") && isValidHexRGB(ref) {
		return strings.ToUpper(ref)
	}
	canonical := ref
	if alias, ok := schemeAliasToTheme[ref]; ok {
		canonical = alias
	}
	if hex, ok := colorMap[canonical]; ok {
		return strings.ToUpper(hex)
	}
	return ""
}

var (
	layoutBgSrgbRegexp = regexp.MustCompile(
		`<p:bg\b[^>]*>` +
			`\s*<p:bgPr\b[^>]*>` +
			`\s*<a:solidFill\b[^>]*>` +
			`\s*<a:srgbClr\s+val="([0-9A-Fa-f]{6})"`,
	)
	layoutBgSchemeRegexp = regexp.MustCompile(
		`<p:bg\b[^>]*>` +
			`\s*<p:bgPr\b[^>]*>` +
			`\s*<a:solidFill\b[^>]*>` +
			`\s*<a:schemeClr\s+val="([^"]+)"`,
	)
)

// extractLayoutBackground returns the layout's explicit background color as
// a #RRGGBB hex string, or empty if the layout has no <p:bg>/<p:bgPr>
// solid fill (i.e. background is inherited from the master). The regex
// patterns mirror internal/generator/text_contrast.go to stay consistent
// with how the contrast-fix pass detects layout backgrounds.
func extractLayoutBackground(reader *template.Reader, layoutPath string, colorMap map[string]string) string {
	data, err := reader.ReadFile(layoutPath)
	if err != nil {
		return ""
	}
	xmlStr := string(data)
	if m := layoutBgSrgbRegexp.FindStringSubmatch(xmlStr); len(m) >= 2 {
		return "#" + strings.ToUpper(m[1])
	}
	if m := layoutBgSchemeRegexp.FindStringSubmatch(xmlStr); len(m) >= 2 {
		return resolveFontColor(m[1], colorMap)
	}
	return ""
}

// contrastRatio returns the WCAG relative contrast ratio between two
// #RRGGBB hex colors, and a boolean indicating success.
func contrastRatio(fgHex, bgHex string) (float64, bool) {
	fg, err := svggen.ParseColor(fgHex)
	if err != nil {
		return 0, false
	}
	bg, err := svggen.ParseColor(bgHex)
	if err != nil {
		return 0, false
	}
	return fg.ContrastWith(bg), true
}

// relativeLuminance returns the WCAG relative luminance of a #RRGGBB hex
// color, in the 0..1 range.
func relativeLuminance(hex string) float64 {
	c, err := svggen.ParseColor(hex)
	if err != nil {
		return 0
	}
	return c.Luminance()
}

func isValidHexRGB(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, ch := range s[1:] {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

func expectsDark(slot string) bool  { return slot == "dk1" || slot == "dk2" }
func expectsLight(slot string) bool { return slot == "lt1" || slot == "lt2" }

func knownSurfaceTintRole(role string) bool {
	for _, r := range surfaceTintRoles {
		if r == role {
			return true
		}
	}
	return false
}

func polarityFailure(slot, rgb string, lum float64, polarity, expectation string) string {
	return "scheme color " + slot + "=" + rgb +
		" has luminance " + percentStr(lum) +
		" — expected " + polarity + " (" + expectation + ")"
}

func percentStr(v float64) string {
	// Two decimal places without pulling in strconv.FormatFloat formatting flags.
	scaled := int(v*10000 + 0.5)
	whole := scaled / 100
	frac := scaled % 100
	return itoa(whole) + "." + pad2(frac) + "%"
}

func ratioStr(v float64) string {
	scaled := int(v*100 + 0.5)
	whole := scaled / 100
	frac := scaled % 100
	return itoa(whole) + "." + pad2(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}
