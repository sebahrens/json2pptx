package generator

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// themePartPattern matches the theme parts of a package: ppt/theme/themeN.xml.
var themePartPattern = regexp.MustCompile(`^ppt/theme/theme\d+\.xml$`)

// schemeColorElements are the theme colour slots a theme_override may address.
// The names match both the JSON override keys and the OOXML element names
// inside <a:clrScheme>.
var schemeColorElements = []string{
	"dk1", "lt1", "dk2", "lt2",
	"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
	"hlink", "folHlink",
}

// applyThemeOverrideToThemeParts rewrites the package's theme parts so a
// theme_override actually reaches the artifact.
//
// Before this existed, the override was applied only to ctx.themeColors — the
// in-memory palette used for chart styling — while ppt/theme/themeN.xml was
// copied from the template unmodified. Every <a:schemeClr val="accent1"> in a
// layout, pattern or native shape therefore resolved through the ORIGINAL
// theme, so a deck asking for accent1 #6A1B9A rendered in the template's navy
// and reported zero warnings, while resolve_theme dutifully reported the
// override as applied (go-slide-creator-p327).
//
// Patching the theme part makes every scheme-colour reference follow the
// override at once, because that is the single place they all resolve through.
// The patched parts are staged in syntheticFiles, which writeTemplateFiles
// already skips in favour of writeSyntheticFiles.
func (ctx *singlePassContext) applyThemeOverrideToThemeParts() {
	if ctx.themeOverride == nil {
		return
	}
	if len(ctx.themeOverride.Colors) == 0 && ctx.themeOverride.TitleFont == "" && ctx.themeOverride.BodyFont == "" {
		return
	}

	for _, f := range ctx.templateReader.File {
		if !themePartPattern.MatchString(f.Name) {
			continue
		}
		if _, already := ctx.syntheticFiles[f.Name]; already {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf(
				"theme_override: could not read %s (%v) — scheme colors and fonts keep the template's values", f.Name, err))
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf(
				"theme_override: could not read %s (%v) — scheme colors and fonts keep the template's values", f.Name, err))
			continue
		}

		patched, applied := patchThemeXML(string(data), ctx.themeOverride)
		if len(applied.unmatchedColors) > 0 {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf(
				"theme_override.colors: %s not found in %s — those slots keep the template's values",
				strings.Join(applied.unmatchedColors, ", "), f.Name))
		}
		if applied.changed() {
			ctx.syntheticFiles[f.Name] = []byte(patched)
			slog.Info("theme override applied to theme part",
				slog.String("part", f.Name),
				slog.Int("colors", applied.colors),
				slog.Bool("major_font", applied.majorFont),
				slog.Bool("minor_font", applied.minorFont))
		}
	}
}

// themePatchResult records what patchThemeXML actually changed, so callers can
// warn about an override that addressed a slot the theme does not declare.
type themePatchResult struct {
	colors          int
	majorFont       bool
	minorFont       bool
	unmatchedColors []string
}

func (r themePatchResult) changed() bool {
	return r.colors > 0 || r.majorFont || r.minorFont
}

// clrSchemeSlotPattern builds a matcher for one colour slot's inner colour
// element, e.g. <a:accent1><a:srgbClr val="2E5090"/></a:accent1>. The inner
// element may be srgbClr or sysClr, so both are replaced with an srgbClr.
func clrSchemeSlotPattern(slot string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)(<a:` + slot + `>)\s*<a:(?:srgbClr|sysClr)\b[^>]*/>\s*(</a:` + slot + `>)`)
}

var (
	majorFontLatinPattern = regexp.MustCompile(`(?s)(<a:majorFont>\s*<a:latin\s+typeface=")[^"]*(")`)
	minorFontLatinPattern = regexp.MustCompile(`(?s)(<a:minorFont>\s*<a:latin\s+typeface=")[^"]*(")`)
)

// patchThemeXML rewrites a theme part's colour scheme and latin typefaces from
// an override, returning the new XML and a record of what changed.
func patchThemeXML(xml string, override *types.ThemeOverride) (string, themePatchResult) {
	var result themePatchResult
	if override == nil {
		return xml, result
	}

	for _, slot := range schemeColorElements {
		hex, ok := override.Colors[slot]
		if !ok || strings.TrimSpace(hex) == "" {
			continue
		}
		val := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(hex), "#"))
		pattern := clrSchemeSlotPattern(slot)
		if !pattern.MatchString(xml) {
			result.unmatchedColors = append(result.unmatchedColors, slot)
			continue
		}
		xml = pattern.ReplaceAllString(xml, `${1}<a:srgbClr val="`+val+`"/>${2}`)
		result.colors++
	}

	if override.TitleFont != "" && majorFontLatinPattern.MatchString(xml) {
		xml = majorFontLatinPattern.ReplaceAllString(xml, `${1}`+escapeXMLAttrValue(override.TitleFont)+`${2}`)
		result.majorFont = true
	}
	if override.BodyFont != "" && minorFontLatinPattern.MatchString(xml) {
		xml = minorFontLatinPattern.ReplaceAllString(xml, `${1}`+escapeXMLAttrValue(override.BodyFont)+`${2}`)
		result.minorFont = true
	}

	return xml, result
}

// escapeXMLAttrValue escapes a font name for use inside a double-quoted XML
// attribute. Font names are author-supplied, so a stray quote or ampersand must
// not break the theme part.
func escapeXMLAttrValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
