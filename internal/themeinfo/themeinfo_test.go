package themeinfo

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

const testTemplate = "../../templates/midnight-blue.pptx"

func TestResolve_FullPalette(t *testing.T) {
	res, err := Resolve(testTemplate, Options{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if res.Template != "midnight-blue" {
		t.Errorf("Template = %q, want midnight-blue", res.Template)
	}
	if len(res.Colors) == 0 {
		t.Fatal("expected non-empty Colors")
	}
	for _, name := range []string{"accent1", "dk1", "lt1"} {
		hex, ok := res.Colors[name]
		if !ok {
			t.Errorf("missing color %q", name)
			continue
		}
		if len(hex) != 7 || hex[0] != '#' {
			t.Errorf("color %q has invalid hex %q", name, hex)
		}
	}

	// Unfiltered: ResolvedFor is nil, ThemeColors mirrors Colors 1:1.
	if res.ResolvedFor != nil {
		t.Errorf("ResolvedFor = %v, want nil when unfiltered", res.ResolvedFor)
	}
	if len(res.ThemeColors) != len(res.Colors) {
		t.Errorf("ThemeColors len %d != Colors len %d", len(res.ThemeColors), len(res.Colors))
	}
	for _, e := range res.ThemeColors {
		if res.Colors[e.Name] != e.RGB {
			t.Errorf("ThemeColors[%q]=%q disagrees with Colors[%q]=%q", e.Name, e.RGB, e.Name, res.Colors[e.Name])
		}
	}

	// AllColors carries the full theme palette for caller-side derivation.
	if len(res.AllColors) != len(res.Colors) {
		t.Errorf("AllColors len %d != Colors len %d when unfiltered", len(res.AllColors), len(res.Colors))
	}

	if res.Fonts.Major.Latin == "" || res.Fonts.Minor.Latin == "" {
		t.Errorf("expected non-empty fonts, got major=%q minor=%q", res.Fonts.Major.Latin, res.Fonts.Minor.Latin)
	}
}

func TestResolve_Filtered_PreservesRequestOrder(t *testing.T) {
	res, err := Resolve(testTemplate, Options{ColorNames: []string{"accent2", "accent1"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(res.Colors) != 2 {
		t.Errorf("Colors len = %d, want 2: %v", len(res.Colors), res.Colors)
	}
	if len(res.ThemeColors) != 2 {
		t.Fatalf("ThemeColors len = %d, want 2", len(res.ThemeColors))
	}
	if res.ThemeColors[0].Name != "accent2" || res.ThemeColors[1].Name != "accent1" {
		t.Errorf("ThemeColors order = [%q, %q], want [accent2, accent1]", res.ThemeColors[0].Name, res.ThemeColors[1].Name)
	}
	if len(res.ResolvedFor) != 2 {
		t.Errorf("ResolvedFor = %v, want 2 entries", res.ResolvedFor)
	}
	// AllColors stays the full palette even when the view is filtered.
	if len(res.AllColors) <= 2 {
		t.Errorf("AllColors len = %d, want full palette regardless of filter", len(res.AllColors))
	}
}

func TestResolve_UnknownColor_SuggestsMatch(t *testing.T) {
	res, err := Resolve(testTemplate, Options{ColorNames: []string{"accent1", "accnet2"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if _, ok := res.Colors["accent1"]; !ok {
		t.Error("accent1 should resolve")
	}
	if _, ok := res.Colors["accnet2"]; ok {
		t.Error("misspelled accnet2 should not appear in Colors")
	}
	if len(res.Unknown) != 1 {
		t.Fatalf("Unknown len = %d, want 1", len(res.Unknown))
	}
	if res.Unknown[0].Name != "accnet2" {
		t.Errorf("Unknown name = %q, want accnet2", res.Unknown[0].Name)
	}
	if res.Unknown[0].DidYouMean != "accent2" {
		t.Errorf("DidYouMean = %q, want accent2", res.Unknown[0].DidYouMean)
	}
}

func TestResolve_Override_AppliesColorsAndWarns(t *testing.T) {
	const newAccent1 = "#336699"
	base, err := Resolve(testTemplate, Options{})
	if err != nil {
		t.Fatalf("baseline Resolve: %v", err)
	}
	if strings.EqualFold(base.Colors["accent1"], newAccent1) {
		t.Skipf("baseline accent1 already %s", newAccent1)
	}

	res, err := Resolve(testTemplate, Options{
		Override: &types.ThemeOverride{
			Colors:    map[string]string{"accent1": newAccent1},
			TitleFont: "Helvetica Neue",
		},
	})
	if err != nil {
		t.Fatalf("Resolve with override: %v", err)
	}

	if !strings.EqualFold(res.Colors["accent1"], newAccent1) {
		t.Errorf("accent1 after override = %q, want %q", res.Colors["accent1"], newAccent1)
	}
	if !strings.EqualFold(res.Colors["dk1"], base.Colors["dk1"]) {
		t.Errorf("dk1 unexpectedly changed: base=%q after=%q", base.Colors["dk1"], res.Colors["dk1"])
	}
	if res.Fonts.Major.Latin != "Helvetica Neue" {
		t.Errorf("Fonts.Major.Latin = %q, want Helvetica Neue", res.Fonts.Major.Latin)
	}
	// A non-embedded font swap must surface a warning.
	if len(res.Warnings) == 0 {
		t.Error("expected a warning for the non-embedded title_font swap")
	}
}

func TestResolve_OpenError_WrapsMessage(t *testing.T) {
	_, err := Resolve("../../templates/does-not-exist.pptx", Options{})
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.HasPrefix(err.Error(), "failed to open template:") {
		t.Errorf("error = %q, want prefix %q", err.Error(), "failed to open template:")
	}
}
