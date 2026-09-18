package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

const themeFixture = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Midnight">
  <a:themeElements>
    <a:clrScheme name="Midnight">
      <a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
      <a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="1F2430"/></a:dk2>
      <a:lt2><a:srgbClr val="E8EAF0"/></a:lt2>
      <a:accent1><a:srgbClr val="2E5090"/></a:accent1>
      <a:accent2><a:srgbClr val="C0504D"/></a:accent2>
      <a:hlink><a:srgbClr val="0563C1"/></a:hlink>
      <a:folHlink><a:srgbClr val="954F72"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="Midnight">
      <a:majorFont><a:latin typeface="Gill Sans"/><a:ea typeface=""/></a:majorFont>
      <a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/></a:minorFont>
    </a:fontScheme>
  </a:themeElements>
</a:theme>`

func schemeColor(t *testing.T, xml, slot string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<a:` + slot + `>\s*<a:srgbClr val="([^"]*)"`)
	m := re.FindStringSubmatch(xml)
	if m == nil {
		return ""
	}
	return m[1]
}

func latinTypeface(t *testing.T, xml, fontElem string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<a:` + fontElem + `>\s*<a:latin typeface="([^"]*)"`)
	m := re.FindStringSubmatch(xml)
	if m == nil {
		return ""
	}
	return m[1]
}

// go-slide-creator-p327: theme_override was applied only to the in-memory
// palette. ppt/theme/themeN.xml was copied from the template unmodified, so
// every <a:schemeClr val="accent1"> — in layouts, patterns and native shapes —
// still resolved to the template's colour, and the deck reported zero warnings
// while resolve_theme claimed the override had landed.
func TestPatchThemeXML(t *testing.T) {
	override := &types.ThemeOverride{
		Colors:    map[string]string{"accent1": "#6A1B9A", "dk1": "#101010", "lt2": "F0F0F0"},
		TitleFont: "Georgia",
		BodyFont:  "Verdana",
	}

	patched, result := patchThemeXML(themeFixture, override)

	if got := schemeColor(t, patched, "accent1"); got != "6A1B9A" {
		t.Errorf("accent1 = %q, want 6A1B9A", got)
	}
	// A sysClr slot must be replaced with an srgbClr, not left alone.
	if got := schemeColor(t, patched, "dk1"); got != "101010" {
		t.Errorf("dk1 = %q, want 101010 (a sysClr slot must be replaceable)", got)
	}
	// A hex without the "#" prefix is accepted and normalised to upper case.
	if got := schemeColor(t, patched, "lt2"); got != "F0F0F0" {
		t.Errorf("lt2 = %q, want F0F0F0", got)
	}
	// Untouched slots keep the template's values.
	if got := schemeColor(t, patched, "accent2"); got != "C0504D" {
		t.Errorf("accent2 = %q, want the template's C0504D", got)
	}

	if got := latinTypeface(t, patched, "majorFont"); got != "Georgia" {
		t.Errorf("majorFont latin = %q, want Georgia", got)
	}
	if got := latinTypeface(t, patched, "minorFont"); got != "Verdana" {
		t.Errorf("minorFont latin = %q, want Verdana", got)
	}

	if result.colors != 3 {
		t.Errorf("patched %d colors, want 3", result.colors)
	}
	if !result.majorFont || !result.minorFont {
		t.Errorf("font flags = major:%v minor:%v, want both true", result.majorFont, result.minorFont)
	}
	if len(result.unmatchedColors) != 0 {
		t.Errorf("unexpected unmatched colors: %v", result.unmatchedColors)
	}
}

// An override naming a slot the theme does not declare must be reported, not
// silently ignored.
func TestPatchThemeXML_UnmatchedSlotReported(t *testing.T) {
	override := &types.ThemeOverride{Colors: map[string]string{"accent6": "#123456"}}
	patched, result := patchThemeXML(themeFixture, override)

	if result.colors != 0 {
		t.Errorf("colors = %d, want 0 (accent6 is absent from the fixture)", result.colors)
	}
	if len(result.unmatchedColors) != 1 || result.unmatchedColors[0] != "accent6" {
		t.Errorf("unmatchedColors = %v, want [accent6]", result.unmatchedColors)
	}
	if patched != themeFixture {
		t.Error("an unmatched slot must leave the XML untouched")
	}
}

// A nil or empty override must change nothing.
func TestPatchThemeXML_NoOverrideIsNoOp(t *testing.T) {
	for _, o := range []*types.ThemeOverride{nil, {}, {Colors: map[string]string{}}} {
		patched, result := patchThemeXML(themeFixture, o)
		if result.changed() {
			t.Errorf("override %+v reported changes", o)
		}
		if patched != themeFixture {
			t.Errorf("override %+v modified the XML", o)
		}
	}
}

// Author-supplied font names must not be able to break the theme part.
func TestPatchThemeXML_EscapesFontNames(t *testing.T) {
	override := &types.ThemeOverride{TitleFont: `Ampersand & "Quote"`}
	patched, _ := patchThemeXML(themeFixture, override)

	if strings.Contains(patched, `typeface="Ampersand & "Quote""`) {
		t.Error("font name was injected unescaped, breaking the attribute")
	}
	if !strings.Contains(patched, "&amp;") || !strings.Contains(patched, "&quot;") {
		t.Errorf("font name should be XML-escaped, got: %s", latinTypeface(t, patched, "majorFont"))
	}
}
