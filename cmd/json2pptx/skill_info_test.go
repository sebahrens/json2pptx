package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/layoutpreview"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestTemplateDiscoveryPlaceholderParity(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	var sawUtility, sawSectionNumber bool
	for _, templatePath := range testutil.TestTemplatePaths() {
		t.Run(filepath.Base(templatePath), func(t *testing.T) {
			compact, err := analyzeTemplateForSkillInfoOpts(templatePath, cache, "compact", skillInfoOptions{NoPreview: true})
			if err != nil {
				t.Fatal(err)
			}
			full, err := analyzeTemplateForSkillInfoOpts(templatePath, cache, "full", skillInfoOptions{NoPreview: true})
			if err != nil {
				t.Fatal(err)
			}
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			report, err := examine.Examine(reader, examine.Options{TemplatePath: templatePath})
			if err != nil {
				t.Fatal(err)
			}
			compactByID := make(map[string]skillLayoutSummary, len(compact.LayoutSummaries))
			fullByID := make(map[string]skillLayoutInfo, len(full.Layouts))
			for _, layout := range compact.LayoutSummaries {
				compactByID[layout.ID] = layout
			}
			for _, layout := range full.Layouts {
				fullByID[layout.ID] = layout
			}
			for _, layout := range report.Layouts {
				cs, ok := compactByID[layout.ID]
				if !ok {
					t.Errorf("examine layout %q missing from compact listing", layout.ID)
					continue
				}
				fs, ok := fullByID[layout.ID]
				if !ok {
					t.Errorf("examine layout %q missing from full listing", layout.ID)
					continue
				}
				if len(cs.Placeholders) != len(layout.Placeholders) || len(fs.Placeholders) != len(layout.Placeholders) {
					t.Errorf("%s: placeholder count examine=%d compact=%d full=%d", layout.ID,
						len(layout.Placeholders), len(cs.Placeholders), len(fs.Placeholders))
					continue
				}
				for i, want := range layout.Placeholders {
					if want.Type == "other" {
						sawUtility = true
					}
					if want.Role == "section_number" {
						sawSectionNumber = true
					}
					c, f := cs.Placeholders[i], fs.Placeholders[i]
					if c.ID != want.ID || c.Type != want.Type || c.Role != want.Role || c.AutoFilled != want.AutoFilled {
						t.Errorf("%s placeholder %d compact=%+v, examine=%+v", layout.ID, i, c, want)
					}
					if f.ID != want.ID || f.Type != want.Type || f.Role != want.Role || f.AutoFilled != want.AutoFilled {
						t.Errorf("%s placeholder %d full=%+v, examine=%+v", layout.ID, i, f, want)
					}
					if c.MaxChars != want.MaxChars || f.MaxChars != want.MaxChars ||
						c.MaxCharsPerLine != want.MaxCharsPerLine || f.MaxCharsPerLine != want.MaxCharsPerLine {
						t.Errorf("%s placeholder %d capacity examine=(%d,%d) compact=(%d,%d) full=(%d,%d)",
							layout.ID, i, want.MaxChars, want.MaxCharsPerLine,
							c.MaxChars, c.MaxCharsPerLine, f.MaxChars, f.MaxCharsPerLine)
					}
				}
			}
		})
	}
	if !sawUtility || !sawSectionNumber {
		t.Errorf("parity corpus lacks a utility placeholder (%v) or section number (%v)", sawUtility, sawSectionNumber)
	}
}

func TestBuildSupportedTypes_DataFormatHints(t *testing.T) {
	st := buildSupportedTypes()

	if st.DataFormatHints == nil {
		t.Fatal("DataFormatHints should not be nil")
	}

	// Every chart type must have a corresponding data format hint.
	for _, ct := range st.ChartTypes {
		if _, ok := st.DataFormatHints[ct]; !ok {
			t.Errorf("chart type %q missing from DataFormatHints", ct)
		}
	}

	// Every diagram type that is not an alias (icon_columns, icon_rows, stat_cards
	// are aliases for panel_layout) must have a data format hint.
	aliases := map[string]bool{
		"icon_columns": true,
		"icon_rows":    true,
		"stat_cards":   true,
	}
	for _, dt := range st.DiagramTypes {
		if aliases[dt] {
			continue
		}
		if _, ok := st.DataFormatHints[dt]; !ok {
			t.Errorf("diagram type %q missing from DataFormatHints", dt)
		}
	}

	// Spot-check a few entries for correct structure.
	tests := []struct {
		name         string
		wantRequired []string
		wantDesc     string
	}{
		{"bar", []string{"categories", "series"}, "categories: string array; series: [{name, values: number[]}]"},
		{"waterfall", []string{"points"}, "points: [{label, value, type: \"increase\"|\"decrease\"|\"total\"}]"},
		{"gauge", []string{"value"}, "value: number; min/max: number; thresholds: [{value, color, label}]"},
		{"fishbone", []string{"effect"}, "effect: string (problem label); categories: [{name, causes: string[]}]"},
		{"panel_layout", []string{"panels"}, "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\"|\"stylish_panels\" (inside data). In stat_cards the NUMBER is the hero: an explicit value wins, otherwise a short body carrying a digit (\"EUR 184m\") is drawn large with title as its caption; a prose body stays small under the title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, ok := st.DataFormatHints[tt.name]
			if !ok {
				t.Fatalf("missing DataFormatHints entry for %q", tt.name)
			}
			got := make([]string, len(df.RequiredKeys))
			copy(got, df.RequiredKeys)
			sort.Strings(got)
			want := make([]string, len(tt.wantRequired))
			copy(want, tt.wantRequired)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Errorf("RequiredKeys = %v, want %v", df.RequiredKeys, tt.wantRequired)
			} else {
				for i := range got {
					if got[i] != want[i] {
						t.Errorf("RequiredKeys = %v, want %v", df.RequiredKeys, tt.wantRequired)
						break
					}
				}
			}
			if df.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", df.Description, tt.wantDesc)
			}
		})
	}
}

func TestFunnelHintsDistinguishPercentageFromConversion(t *testing.T) {
	hint := buildDataFormatHints()["funnel"]
	for _, key := range []string{"show_percentage", "show_conversion"} {
		if !slices.Contains(hint.OptionalKeys, key) {
			t.Errorf("funnel hint omits %s", key)
		}
	}
	if !strings.Contains(hint.Description, "set both false") {
		t.Errorf("funnel hint does not explain how to suppress percentages: %s", hint.Description)
	}
}

func TestBuildPatternEntries_CompactMode(t *testing.T) {
	compact, full := buildPatternEntries("compact")

	// There should be at least the 8 v1 patterns
	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}

	// Full should be nil in compact mode
	if full != nil {
		t.Errorf("expected nil patterns_full in compact mode, got %d entries", len(full))
	}

	// Verify entries are sorted by name
	for i := 1; i < len(compact); i++ {
		if compact[i].Name < compact[i-1].Name {
			t.Errorf("compact entries not sorted: %q comes after %q", compact[i].Name, compact[i-1].Name)
		}
	}

	// Every compact entry must have name, cells, and use_when populated
	for _, c := range compact {
		if c.Name == "" {
			t.Error("compact entry has empty name")
		}
		if c.Cells == "" {
			t.Errorf("compact entry %q has empty cells", c.Name)
		}
		if c.UseWhen == "" {
			t.Errorf("compact entry %q has empty use_when", c.Name)
		}
	}

	// Spot-check specific patterns match their registry data
	reg := patterns.Default()
	for _, c := range compact {
		p, ok := reg.Get(c.Name)
		if !ok {
			t.Errorf("compact entry %q not found in registry", c.Name)
			continue
		}
		if c.UseWhen != p.UseWhen() {
			t.Errorf("compact entry %q: use_when = %q, want %q", c.Name, c.UseWhen, p.UseWhen())
		}
	}
}

func TestBuildPatternEntries_FullMode(t *testing.T) {
	compact, full := buildPatternEntries("full")

	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}
	if len(full) < 8 {
		t.Fatalf("expected at least 8 full patterns, got %d", len(full))
	}
	if len(compact) != len(full) {
		t.Errorf("compact (%d) and full (%d) entry counts should match", len(compact), len(full))
	}

	// Full entries must have valid JSON Schema
	for _, f := range full {
		if f.Name == "" {
			t.Error("full entry has empty name")
		}
		if f.Description == "" {
			t.Errorf("full entry %q has empty description", f.Name)
		}
		if f.Version < 1 {
			t.Errorf("full entry %q has version %d, want >= 1", f.Name, f.Version)
		}
		if len(f.Schema) == 0 {
			t.Errorf("full entry %q has empty schema", f.Name)
			continue
		}
		// Verify schema is valid JSON
		var raw map[string]any
		if err := json.Unmarshal(f.Schema, &raw); err != nil {
			t.Errorf("full entry %q: schema is not valid JSON: %v", f.Name, err)
			continue
		}
		// Must have $schema field (AsRoot)
		if _, ok := raw["$schema"]; !ok {
			t.Errorf("full entry %q: schema missing $schema field", f.Name)
		}
	}
}

func TestBuildPatternEntries_FullMode_AlwaysIncludesFullSchemas(t *testing.T) {
	// Regression: --mode=full must never silently downgrade to compact. The
	// legacy --include-full-schemas flag is a no-op; mode=full alone must
	// produce patterns_full with full JSON schemas.
	compact, full := buildPatternEntries("full")
	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}
	if len(full) < 8 {
		t.Fatalf("expected at least 8 full patterns in mode=full, got %d", len(full))
	}
	for _, f := range full {
		if len(f.Schema) == 0 {
			t.Errorf("full entry %q has empty schema in mode=full", f.Name)
		}
	}
}

func TestBuildPatternEntries_ListMode(t *testing.T) {
	// In list mode, buildPatternEntries is not called (mode == "list" guard in runSkillInfo).
	// But if called directly, it should still return valid results.
	compact, _ := buildPatternEntries("list")

	// Should still produce compact entries (buildPatternEntries doesn't enforce list exclusion)
	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}
}

func TestBuildColorRoles_WhiteTextSafe(t *testing.T) {
	colors := []types.ThemeColor{
		{Name: "accent1", RGB: "#2E5090"}, // dark blue — passes
		{Name: "accent2", RGB: "#D4463A"}, // red — passes large, not body
		{Name: "accent3", RGB: "#E8A838"}, // yellow-orange — fails (too light)
		{Name: "accent4", RGB: "#43A047"}, // green — passes
		{Name: "accent5", RGB: "#5C6BC0"}, // indigo — passes
		{Name: "accent6", RGB: "#26A69A"}, // teal — borderline
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "lt2", RGB: "#E8ECF1"},
	}

	roles := buildColorRoles(colors)

	if roles.PrimaryFill != "accent1" {
		t.Errorf("PrimaryFill = %q, want accent1", roles.PrimaryFill)
	}
	if roles.SecondaryFill != "accent5" {
		t.Errorf("SecondaryFill = %q, want accent5 (accent2 only passes the large-text bar)", roles.SecondaryFill)
	}
	if roles.BodyFill != "lt2" {
		t.Errorf("BodyFill = %q, want lt2", roles.BodyFill)
	}
	if roles.BodyText != "dk1" {
		t.Errorf("BodyText = %q, want dk1", roles.BodyText)
	}

	// accent3 (#E8A838) should NOT be in white_text_safe (low contrast against white)
	for _, s := range roles.WhiteTextSafe {
		if s == "accent3" {
			t.Error("accent3 (#E8A838) should not be white-text-safe")
		}
	}

	// accent1 must be in white_text_safe
	found := false
	for _, s := range roles.WhiteTextSafe {
		if s == "accent1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("accent1 should be in white_text_safe")
	}
	if !slices.Contains(roles.WhiteTextSafeLarge, "accent2") || slices.Contains(roles.WhiteTextSafeBody, "accent2") {
		t.Errorf("accent2 should be large-only: body=%v large=%v", roles.WhiteTextSafeBody, roles.WhiteTextSafeLarge)
	}
}

func TestBuildColorRoles_NoSafeAccents(t *testing.T) {
	// All light accents — none pass WCAG against white
	colors := []types.ThemeColor{
		{Name: "accent1", RGB: "#FFEB3B"}, // bright yellow
		{Name: "accent2", RGB: "#FFF176"}, // light yellow
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt2", RGB: "#F5F5F5"},
	}

	roles := buildColorRoles(colors)

	// No unsafe accent may be recommended as a white-text fill.
	if roles.PrimaryFill != "dk1" {
		t.Errorf("PrimaryFill = %q, want dk1 (safe fallback)", roles.PrimaryFill)
	}
	if roles.SecondaryFill != "dk1" {
		t.Errorf("SecondaryFill = %q, want dk1 (safe fallback)", roles.SecondaryFill)
	}
	if len(roles.WhiteTextSafe) != 0 {
		t.Errorf("WhiteTextSafe = %v, want empty", roles.WhiteTextSafe)
	}
}

func TestBuildColorRoles_AbstractThemeUsesVisibleFirstAccent(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/abstract.pptx", cache, "compact")
	if err != nil {
		t.Fatal(err)
	}
	roles := info.ColorRoles
	if roles.PrimaryFill != "accent1" {
		t.Errorf("abstract primary_fill = %q, want its visible first accent", roles.PrimaryFill)
	}
	if slices.Contains(roles.WhiteTextSafeBody, roles.PrimaryFill) {
		t.Errorf("abstract %s must not be advertised as body-text safe", roles.PrimaryFill)
	}
	if !slices.Equal(roles.NearBackgroundAccents, []string{"accent4"}) {
		t.Errorf("abstract near_background_accents = %v, want only accent4", roles.NearBackgroundAccents)
	}
}

func TestBuildColorRolesUsesLargeSafeAccentBeforeDarkFallback(t *testing.T) {
	roles := buildColorRoles([]types.ThemeColor{
		{Name: "accent1", RGB: "#EEEEEE"},
		{Name: "accent2", RGB: "#8F8F8F"}, // 3.23:1, large-text safe only
		{Name: "dk2", RGB: "#44546A"},
	})
	if roles.PrimaryFill != "accent2" || roles.SecondaryFill != "dk2" {
		t.Errorf("primary/secondary = %s/%s, want accent2/dk2", roles.PrimaryFill, roles.SecondaryFill)
	}
}

func TestBuildColorRoles_SkipsAccent2WhenUnsafe(t *testing.T) {
	// accent2 is too light, should pick accent3 as secondary
	colors := []types.ThemeColor{
		{Name: "accent1", RGB: "#1A237E"}, // very dark blue — passes
		{Name: "accent2", RGB: "#FFEB3B"}, // bright yellow — fails
		{Name: "accent3", RGB: "#B71C1C"}, // dark red — passes
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt2", RGB: "#F5F5F5"},
	}

	roles := buildColorRoles(colors)

	if roles.PrimaryFill != "accent1" {
		t.Errorf("PrimaryFill = %q, want accent1", roles.PrimaryFill)
	}
	if roles.SecondaryFill != "accent3" {
		t.Errorf("SecondaryFill = %q, want accent3 (accent2 is unsafe)", roles.SecondaryFill)
	}
}

func TestBuildColorRolesSeparatesBodyAndLargeWhiteTextSafety(t *testing.T) {
	roles := buildColorRoles([]types.ThemeColor{
		{Name: "accent1", RGB: "#8F8F8F"}, // 3.23:1: large only
		{Name: "accent2", RGB: "#1B2A4A"}, // safe for body
		{Name: "accent3", RGB: "#EEEEEE"}, // unsafe for both
		{Name: "accent4", RGB: "#333333"}, // safe for body
	})
	if !slices.Contains(roles.WhiteTextSafeLarge, "accent1") || slices.Contains(roles.WhiteTextSafeBody, "accent1") {
		t.Errorf("3.23:1 accent classed incorrectly: body=%v large=%v", roles.WhiteTextSafeBody, roles.WhiteTextSafeLarge)
	}
	if !slices.Equal(roles.WhiteTextSafe, roles.WhiteTextSafeBody) {
		t.Errorf("legacy white_text_safe must alias body-safe accents: %v vs %v", roles.WhiteTextSafe, roles.WhiteTextSafeBody)
	}
	if roles.PrimaryFill != "accent2" || roles.SecondaryFill != "accent4" {
		t.Errorf("primary/secondary fills = %s/%s, want body-safe accent2/accent4", roles.PrimaryFill, roles.SecondaryFill)
	}
}

func TestColorRolesSafetyOnLocalTemplateCorpus(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	white := svggen.MustParseColor("#FFFFFF")
	for _, path := range testutil.TestTemplatePaths() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			info, err := analyzeTemplateForSkillInfo(path, cache, "compact")
			if err != nil {
				t.Fatal(err)
			}
			if info.ColorRoles == nil {
				t.Fatal("template has no color_roles")
			}
			roles := info.ColorRoles
			for i := 1; i <= 6; i++ {
				name := fmt.Sprintf("accent%d", i)
				hex, ok := info.ThemeColors[name]
				if !ok {
					continue
				}
				color, err := svggen.ParseColor(hex)
				if err != nil {
					t.Fatal(err)
				}
				ratio := color.ContrastWith(white)
				if got, want := slices.Contains(roles.WhiteTextSafeBody, name), ratio >= svggen.WCAGAANormal; got != want {
					t.Errorf("%s at %.2f: body-safe=%t, want %t", name, ratio, got, want)
				}
				if got, want := slices.Contains(roles.WhiteTextSafeLarge, name), ratio >= svggen.WCAGAALarge; got != want {
					t.Errorf("%s at %.2f: large-safe=%t, want %t", name, ratio, got, want)
				}
				if backgroundHex, ok := info.ThemeColors["lt1"]; ok {
					background, err := svggen.ParseColor(backgroundHex)
					if err != nil {
						t.Fatal(err)
					}
					if got, want := slices.Contains(roles.NearBackgroundAccents, name), color.ContrastWith(background) < 2; got != want {
						t.Errorf("%s near-background=%t, want %t", name, got, want)
					}
				}
			}
			if !slices.Equal(roles.WhiteTextSafe, roles.WhiteTextSafeBody) {
				t.Error("legacy white_text_safe differs from body-safe list")
			}
		})
	}
}

func TestAnalyzeTemplateForSkillInfo_ColorRolesInCompactMode(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "compact")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if info.ColorRoles == nil {
		t.Fatal("ColorRoles should be populated in compact mode")
	}
	if len(info.ColorRoles.WhiteTextSafe) == 0 {
		t.Error("expected at least one white-text-safe accent for midnight-blue")
	}
}

func TestAnalyzeTemplateForSkillInfo_TableStylesAllTemplates(t *testing.T) {
	templates := testutil.TestTemplatePaths()

	cache := template.NewMemoryCache(24 * time.Hour)

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			t.Parallel() // Each template is immutable; MemoryCache is synchronized.
			for _, mode := range []string{"list", "compact", "full"} {
				info, err := analyzeTemplateForSkillInfo(tmpl, cache, mode)
				if err != nil {
					t.Fatalf("mode=%s: %v", mode, err)
				}
				// table_styles must always be a non-nil slice (never null in JSON).
				if info.TableStyles == nil {
					t.Errorf("mode=%s: TableStyles is nil, want empty slice", mode)
				}
			}
		})
	}

	// modern-template should have at least 1 table style.
	info, err := analyzeTemplateForSkillInfo("../../templates/modern-template.pptx", cache, "compact")
	if err != nil {
		t.Fatalf("modern-template: %v", err)
	}
	if len(info.TableStyles) == 0 {
		t.Error("modern-template should have at least one table style")
	}
	for _, ts := range info.TableStyles {
		if ts.ID == "" {
			t.Error("table style entry has empty ID")
		}
		if ts.Name == "" {
			t.Error("table style entry has empty Name")
		}
	}
}

func TestAnalyzeTemplateForSkillInfo_CompactPlaceholders(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "compact")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if len(info.LayoutSummaries) == 0 {
		t.Fatal("expected at least one layout summary in compact mode")
	}

	// At least one layout should have placeholders with id, type, and max_chars.
	foundPH := false
	for _, ls := range info.LayoutSummaries {
		for _, ph := range ls.Placeholders {
			foundPH = true
			if ph.ID == "" {
				t.Errorf("layout %q: placeholder has empty ID", ls.Name)
			}
			if ph.Type == "" {
				t.Errorf("layout %q: placeholder %q has empty type", ls.Name, ph.ID)
			}
			if ph.Type == "other" {
				t.Errorf("layout %q: placeholder %q has type %q; other should be filtered", ls.Name, ph.ID, ph.Type)
			}
			if ph.MaxChars <= 0 {
				t.Errorf("layout %q: placeholder %q has max_chars=%d, want >0", ls.Name, ph.ID, ph.MaxChars)
			}
		}
	}
	if !foundPH {
		t.Error("expected at least one placeholder across layout summaries")
	}

	// Compact placeholders must NOT appear in JSON as the full skillPlaceholderInfo shape
	// (no x_emu, y_emu, width_emu, height_emu, font_* fields).
	b, _ := json.Marshal(info.LayoutSummaries[0].Placeholders)
	raw := string(b)
	for _, banned := range []string{"x_emu", "y_emu", "width_emu", "height_emu", "font_family", "font_size", "font_color"} {
		if containsSubstring(raw, banned) {
			t.Errorf("compact placeholder JSON contains %q, which should only appear in full mode", banned)
		}
	}
}

func TestTemplateDiscoveryMarksSectionNumberAutoFilled(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	for _, mode := range []string{"compact", "full"} {
		t.Run(mode, func(t *testing.T) {
			info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, mode)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			if mode == "compact" {
				for _, layout := range info.LayoutSummaries {
					for _, ph := range layout.Placeholders {
						if ph.ID == "Section Number" || ph.ID == "section_number" {
							found = true
							if !ph.AutoFilled {
								t.Error("compact section number is not auto-filled")
							}
						} else if ph.AutoFilled {
							t.Errorf("compact %q incorrectly marked auto-filled", ph.ID)
						}
					}
				}
			} else {
				for _, layout := range info.Layouts {
					for _, ph := range layout.Placeholders {
						if ph.ID == "Section Number" || ph.ID == "section_number" {
							found = true
							if !ph.AutoFilled {
								t.Error("full section number is not auto-filled")
							}
						} else if ph.AutoFilled {
							t.Errorf("full %q incorrectly marked auto-filled", ph.ID)
						}
					}
				}
			}
			if !found {
				t.Fatal("fixture has no Section Number placeholder")
			}
		})
	}
}

func TestAnalyzeTemplateForSkillInfo_AccentUsageGuideOmittedWhenAbsent(t *testing.T) {
	// Bundled templates don't currently ship accent_usage_guide metadata,
	// so the field should be omitted (nil map → omitempty).
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "compact")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if info.AccentUsageGuide != nil {
		t.Errorf("AccentUsageGuide should be nil when template metadata doesn't supply it, got %v", info.AccentUsageGuide)
	}

	// Verify it's omitted from JSON output.
	b, _ := json.Marshal(info)
	if containsSubstring(string(b), "accent_usage_guide") {
		t.Error("accent_usage_guide should be omitted from JSON when not supplied by template metadata")
	}
}

// TestAnalyzeTemplateForSkillInfo_ReadOnlyWritesNoFiles is the acceptance gate
// for the read-only discovery mode: with NoPreview set, analysis must return
// the full metadata payload (minus preview paths) and write no layout-preview
// cache files anywhere under the user's home cache directory.
func TestAnalyzeTemplateForSkillInfo_ReadOnlyWritesNoFiles(t *testing.T) {
	// Redirect the home dir so the default preview cache
	// (~/.cache/json2pptx/layout-previews) would land somewhere we control and
	// can inspect after the call. HOME governs os.UserHomeDir on darwin/linux.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfoOpts(
		"../../templates/midnight-blue.pptx", cache, "full",
		skillInfoOptions{NoPreview: true},
	)
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfoOpts failed: %v", err)
	}

	// Read-only must not degrade the metadata payload beyond dropping previews.
	if len(info.Layouts) == 0 {
		t.Fatal("read-only full mode should still return layouts")
	}
	if len(info.LayoutSummaries) == 0 {
		t.Fatal("read-only full mode should still return layout summaries")
	}

	// No preview PNG paths should be reported in read-only mode.
	for _, l := range info.Layouts {
		if l.PreviewPNGPath != "" {
			t.Errorf("layout %q carries preview_png_path %q in read-only mode", l.ID, l.PreviewPNGPath)
		}
	}
	for _, s := range info.LayoutSummaries {
		if s.PreviewPNGPath != "" {
			t.Errorf("layout summary %q carries preview_png_path %q in read-only mode", s.ID, s.PreviewPNGPath)
		}
	}

	// Crucially: no preview cache files were created under the redirected HOME.
	cacheRoot := filepath.Join(tmpHome, ".cache", "json2pptx")
	if _, statErr := os.Stat(cacheRoot); !os.IsNotExist(statErr) {
		t.Errorf("read-only analysis created %s (err=%v); expected no cache writes outside the requested output area", cacheRoot, statErr)
	}
}

// TestBuildSkillSideEffects asserts the side-effects descriptor reports the
// correct write intent and opt-out for both read-only and default calls.
func TestBuildSkillSideEffects(t *testing.T) {
	ro := buildSkillSideEffects(true, "read_only=true")
	if ro == nil {
		t.Fatal("buildSkillSideEffects returned nil")
	}
	if ro.PreviewCacheWrites {
		t.Error("read-only side_effects should report preview_cache_writes=false")
	}
	if !ro.ReadOnly {
		t.Error("read-only side_effects should report read_only=true")
	}
	if ro.DisableWith != "read_only=true" {
		t.Errorf("disable_with = %q, want read_only=true", ro.DisableWith)
	}
	if ro.PreviewCacheDir == "" {
		t.Error("preview_cache_dir should be reported even in read-only mode")
	}
	if ro.PreviewCacheDir != layoutpreview.DefaultCacheDir() {
		t.Errorf("preview_cache_dir = %q, want %q", ro.PreviewCacheDir, layoutpreview.DefaultCacheDir())
	}

	def := buildSkillSideEffects(false, "--no-preview")
	if !def.PreviewCacheWrites {
		t.Error("default side_effects should report preview_cache_writes=true")
	}
	if def.ReadOnly {
		t.Error("default side_effects should report read_only=false")
	}
	if def.DisableWith != "--no-preview" {
		t.Errorf("disable_with = %q, want --no-preview", def.DisableWith)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildComposeEntry_PopulatedWithExamples(t *testing.T) {
	entry := buildComposeEntry()
	if entry == nil {
		t.Fatal("buildComposeEntry returned nil")
	}
	if entry.Description == "" {
		t.Error("compose description should not be empty")
	}
	if entry.MaxSegments <= 0 {
		t.Errorf("max_segments should be > 0, got %d", entry.MaxSegments)
	}
	if entry.MaxNestingDepth <= 0 {
		t.Errorf("max_nesting_depth should be > 0, got %d", entry.MaxNestingDepth)
	}
	if len(entry.Directions) < 2 {
		t.Errorf("compose directions should list vertical and horizontal; got %v", entry.Directions)
	}
	if len(entry.Examples) < 2 {
		t.Fatalf("compose entry should ship ≥2 examples; got %d", len(entry.Examples))
	}
	// Each example's JSON must round-trip through json.Unmarshal so consumers
	// can splice it into a deck without re-quoting.
	for i, ex := range entry.Examples {
		if ex.Title == "" {
			t.Errorf("example[%d] missing title", i)
		}
		var raw map[string]any
		if err := json.Unmarshal(ex.JSON, &raw); err != nil {
			t.Errorf("example[%d] JSON is not valid: %v", i, err)
			continue
		}
		// Must contain a compose envelope.
		if _, ok := raw["compose"]; !ok {
			t.Errorf("example[%d] JSON missing top-level \"compose\" key", i)
		}
	}
}

// TestAnalyzeTemplateForSkillInfo_CanonicalMetadataFull asserts that full mode
// surfaces the canonical taxonomy (per-layout type/family/confidence,
// per-placeholder role/confidence/font_size_pt) plus the template-level
// coverage matrix, derivable readiness, and semantic palette metadata.
func TestAnalyzeTemplateForSkillInfo_CanonicalMetadataFull(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "full")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	// Template-level identity + semantic palette metadata.
	if info.SHA256 == "" {
		t.Error("full mode: sha256 should be populated")
	}
	if info.MetadataVersion == "" {
		t.Error("full mode: metadata_version should be populated for a template with metadata")
	}
	if len(info.SemanticAccents) == 0 {
		t.Error("full mode: semantic_accents should be populated for midnight-blue")
	}
	if len(info.SurfaceTints) == 0 {
		t.Error("full mode: surface_tints should be populated for midnight-blue")
	}
	if len(info.DataPalette) == 0 {
		t.Error("full mode: data_palette should be populated for midnight-blue")
	}

	// Canonical coverage lists every content-bearing family; bundled templates
	// cover all four.
	for _, fam := range []string{"title-slide", "section-divider", "one-content", "qa-closing"} {
		cov, ok := info.CanonicalCoverage[fam]
		if !ok {
			t.Errorf("canonical_coverage missing family %q", fam)
			continue
		}
		if !cov.Present || len(cov.Layouts) == 0 {
			t.Errorf("canonical_coverage[%q] expected present with layouts, got %+v", fam, cov)
		}
	}

	// Derivable layouts include the synthesised two-content path.
	if len(info.DerivableLayouts) == 0 {
		t.Fatal("full mode: derivable_layouts should be populated")
	}
	var sawTwoContent bool
	for _, d := range info.DerivableLayouts {
		if d.Name == "two-content" {
			sawTwoContent = true
			if !d.Ready || d.AddressableAs == nil || *d.AddressableAs != "two-column" || d.RequestVia != "layout_id" {
				t.Errorf("two-content must report an actionable layout_id: %+v", d)
			}
		}
		if d.Name == "stat-grid" && (d.AddressableAs != nil || d.RequestVia != "shape_grid_or_pattern") {
			t.Errorf("stat-grid must not masquerade as a layout_id: %+v", d)
		}
	}
	if !sawTwoContent {
		t.Error("derivable_layouts should include two-content")
	}

	// Per-layout canonical classification + per-placeholder role/font evidence.
	var sawCanonicalType, sawRole, sawFontPt bool
	for _, l := range info.Layouts {
		if l.CanonicalType != "" {
			sawCanonicalType = true
			if l.CanonicalFamily == "" {
				t.Errorf("layout %q has canonical_type %q but empty canonical_family", l.ID, l.CanonicalType)
			}
		}
		for _, ph := range l.Placeholders {
			if ph.Role != "" {
				sawRole = true
			}
			if ph.FontSizePt > 0 {
				sawFontPt = true
			}
		}
	}
	if !sawCanonicalType {
		t.Error("no layout exposed canonical_type in full mode")
	}
	if !sawRole {
		t.Error("no placeholder exposed role in full mode")
	}
	if !sawFontPt {
		t.Error("no placeholder exposed font_size_pt in full mode")
	}
}

func TestDerivableLayoutAddressesExistInLocalCorpus(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	for _, file := range testutil.TestTemplatePaths() {
		t.Run(filepath.Base(file), func(t *testing.T) {
			info, err := analyzeTemplateForSkillInfo(file, cache, "full")
			if err != nil {
				t.Fatal(err)
			}
			layouts := make([]types.LayoutMetadata, len(info.Layouts))
			ids := make(map[string]bool, len(info.Layouts))
			for i, item := range info.Layouts {
				layouts[i] = types.LayoutMetadata{ID: item.ID, Tags: item.Tags}
				ids[item.ID] = true
			}
			for _, d := range info.DerivableLayouts {
				if !d.Ready || d.AddressableAs == nil {
					continue
				}
				resolved, ok := layout.ResolveCanonicalLayoutID(*d.AddressableAs, layouts)
				if !ok || !ids[resolved] {
					t.Errorf("%s advertises layout_id %q, but it resolves to missing %q", d.Name, *d.AddressableAs, resolved)
				}
			}
		})
	}
}

// TestAnalyzeTemplateForSkillInfo_CompactSurvivesProjection asserts that the
// compact projection keeps the stable IDs and layout summaries generate-deck
// relies on (canonical_layout_ids, layout_summaries with canonical_type and
// placeholder role) plus the template-level coverage/semantic metadata, while
// the heavy full layouts array stays out of compact.
func TestAnalyzeTemplateForSkillInfo_CompactSurvivesProjection(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "compact")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if len(info.CanonicalLayoutIDs) == 0 {
		t.Error("compact: canonical_layout_ids should survive projection")
	}
	if len(info.LayoutSummaries) == 0 {
		t.Fatal("compact: layout_summaries should survive projection")
	}
	if len(info.CanonicalCoverage) == 0 {
		t.Error("compact: canonical_coverage should survive projection")
	}
	if len(info.SemanticAccents) == 0 {
		t.Error("compact: semantic_accents should survive projection")
	}

	var sawSummaryCanonical, sawSummaryRole bool
	for _, s := range info.LayoutSummaries {
		if s.CanonicalType != "" {
			sawSummaryCanonical = true
		}
		for _, ph := range s.Placeholders {
			if ph.Role != "" {
				sawSummaryRole = true
			}
		}
	}
	if !sawSummaryCanonical {
		t.Error("compact: layout summaries should carry canonical_type")
	}
	if !sawSummaryRole {
		t.Error("compact: compact placeholders should carry role")
	}

	// The full layouts array (placeholder geometry) belongs to full mode only.
	if len(info.Layouts) != 0 {
		t.Errorf("compact: full layouts array should be omitted, got %d", len(info.Layouts))
	}

	// Stable, slimmer compact placeholders must not carry the full geometry
	// shape (those fields are full-mode only).
	b, _ := json.Marshal(info.LayoutSummaries)
	for _, banned := range []string{"x_emu", "font_size_hundredths", "role_confidence", "canonical_confidence"} {
		if containsSubstring(string(b), banned) {
			t.Errorf("compact summaries JSON contains %q, which should only appear in full mode", banned)
		}
	}
}

// TestAnalyzeTemplateForSkillInfo_ListModeKeepsSmallPaletteFields confirms the
// slim projection keeps actionable color intent but omits heavy detail.
func TestAnalyzeTemplateForSkillInfo_ListModeKeepsSmallPaletteFields(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "list")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if info.CanonicalCoverage != nil {
		t.Error("list mode: canonical_coverage should be omitted")
	}
	if info.DerivableLayouts != nil {
		t.Error("list mode: derivable_layouts should be omitted")
	}
	if len(info.SemanticAccents) != 3 || info.ColorRoles == nil || info.TitleFont == "" || info.BodyFont == "" {
		t.Errorf("list mode: missing palette intent or fonts: %+v", info)
	}
	if info.SurfaceTints != nil || info.DataPalette != nil {
		t.Error("list mode: detailed palette fields should be omitted")
	}
	if info.SHA256 != "" {
		t.Error("list mode: sha256 should be omitted (compact+full only)")
	}
}

func TestTemplateListReportsBodyTypographyAcrossTemplates(t *testing.T) {
	cache := template.NewMemoryCache(time.Minute)
	for _, name := range []string{"modern-template", "business-template", "p-style"} {
		path := filepath.Join("..", "..", "templates", name+".pptx")
		if name == "p-style" {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue // local, gitignored fixture; present in local runs
			}
		}
		t.Run(name, func(t *testing.T) {
			info, err := analyzeTemplateForSkillInfoOpts(path, cache, "list", skillInfoOptions{NoPreview: true})
			if err != nil {
				t.Fatal(err)
			}
			if info.TemplateBodyFontSizePt <= 0 || info.BodyFontSizePt <= 0 {
				t.Fatalf("missing body size in list projection: template=%g effective=%g", info.TemplateBodyFontSizePt, info.BodyFontSizePt)
			}
			if info.BodyFontSizePt < 16 || info.BodyFontSizePt > 20 {
				t.Errorf("five-bullet body size %gpt outside 16–20pt", info.BodyFontSizePt)
			}
			if name == "modern-template" && info.TemplateBodyFontSizePt != 14 {
				t.Errorf("modern template body size = %g, want 14pt", info.TemplateBodyFontSizePt)
			}
			if name == "business-template" && info.TemplateBodyFontSizePt != 22 {
				t.Errorf("business canonical content body size = %g, want 22pt", info.TemplateBodyFontSizePt)
			}
		})
	}
}

// go-slide-creator-ccpv: with --templates-dir set, every embedded template was
// listed under its os.CreateTemp filename ("json2pptx-template-2729383514"),
// which changes on every call and is rejected by generate as
// TEMPLATE_NOT_FOUND. The logical name must win over the resolved path.
func TestAnalyzeTemplateForSkillInfo_UsesLogicalName(t *testing.T) {
	// Copy a bundled template to a temp path whose base name is NOT its logical
	// name, mirroring what resolveTemplatePath does for embedded templates.
	src := filepath.Join("..", "..", "templates", "modern-template.pptx")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("bundled template not readable: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "json2pptx-template-2729383514.pptx")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		t.Fatalf("write temp template: %v", err)
	}

	cache := template.NewMemoryCache(time.Minute)

	withName, err := analyzeTemplateForSkillInfoOpts(tmp, cache, "compact",
		skillInfoOptions{NoPreview: true, LogicalName: "modern-template"})
	if err != nil {
		t.Fatalf("analyze with logical name: %v", err)
	}
	if withName.Name != "modern-template" {
		t.Errorf("name = %q, want the logical name %q", withName.Name, "modern-template")
	}

	// With no logical name the file's base name is still used — correct for a
	// template read straight from a directory.
	withoutName, err := analyzeTemplateForSkillInfoOpts(tmp, cache, "compact",
		skillInfoOptions{NoPreview: true})
	if err != nil {
		t.Fatalf("analyze without logical name: %v", err)
	}
	if withoutName.Name != "json2pptx-template-2729383514" {
		t.Errorf("name = %q, want the file base name", withoutName.Name)
	}
}

// go-slide-creator-r9nn: the compact projection must carry the real slide size
// next to the aspect ratio.
func TestAnalyzeTemplateForSkillInfo_ReportsSlideDimensions(t *testing.T) {
	src := filepath.Join("..", "..", "templates", "modern-template.pptx")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled template not present: %v", err)
	}

	info, err := analyzeTemplateForSkillInfoOpts(src, template.NewMemoryCache(time.Minute), "compact",
		skillInfoOptions{NoPreview: true, LogicalName: "modern-template"})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if info.SlideWidthIn <= 0 || info.SlideHeightIn <= 0 {
		t.Fatalf("slide dimensions missing: %gin x %gin", info.SlideWidthIn, info.SlideHeightIn)
	}
	// modern-template is a standard 13.333 x 7.5in widescreen deck.
	if info.SlideWidthIn != 13.333 || info.SlideHeightIn != 7.5 {
		t.Errorf("slide size = %gin x %gin, want 13.333 x 7.5", info.SlideWidthIn, info.SlideHeightIn)
	}
	if info.AspectRatio != "16:9" {
		t.Errorf("aspect_ratio = %q, want 16:9", info.AspectRatio)
	}
}

func TestEmuToInches(t *testing.T) {
	cases := map[int64]float64{
		0:        0,
		-1:       0,
		914400:   1,
		12192000: 13.333,
		6858000:  7.5,
		9144000:  10,
	}
	for emu, want := range cases {
		if got := emuToInches(emu); got != want {
			t.Errorf("emuToInches(%d) = %g, want %g", emu, got, want)
		}
	}
}

// go-slide-creator-3ojy: list_icons offered only a substring filter on glyph
// names, so business vocabulary missed entirely — "strategy", "revenue",
// "governance" and a dozen others returned nothing, while "risk" returned eight
// icons whose only connection was containing "asterisk".
func TestApplyConceptSearch(t *testing.T) {
	sets := []string{"outline"}

	t.Run("a concept with no substring matches resolves via synonym", func(t *testing.T) {
		flat, via, matches, vocab := applyConceptSearch(nil, "strategy", sets)
		if via != "synonym" {
			t.Errorf("matched_via = %q, want synonym", via)
		}
		if len(flat) == 0 || flat[0].name != "target" {
			t.Errorf("first icon = %+v, want target", flat)
		}
		if len(matches) == 0 || matches[0].Concept != "strategy" {
			t.Errorf("concept_matches = %+v, want the strategy concept named", matches)
		}
		if vocab != nil {
			t.Error("a successful match must not also dump the vocabulary")
		}
	})

	t.Run("concept hits lead even when the substring also matched", func(t *testing.T) {
		// This is the "risk" case: the substring filter finds the asterisks.
		substring := []iconSetName{
			{set: "outline", name: "asterisk"},
			{set: "outline", name: "asterisk-simple"},
		}
		flat, via, _, _ := applyConceptSearch(substring, "risk", sets)
		if via != "synonym+name" {
			t.Errorf("matched_via = %q, want synonym+name", via)
		}
		if len(flat) == 0 || flat[0].name != "alert-triangle" {
			t.Errorf("first icon = %+v, want alert-triangle to lead", flat)
		}
		// The substring matches are kept, just after the concept hits.
		var keptAsterisk bool
		for _, e := range flat {
			if e.name == "asterisk" {
				keptAsterisk = true
			}
		}
		if !keptAsterisk {
			t.Error("substring matches should be kept after the concept hits, not dropped")
		}
	})

	t.Run("a pure name query is unaffected", func(t *testing.T) {
		substring := []iconSetName{{set: "outline", name: "chart-bar"}}
		flat, via, matches, _ := applyConceptSearch(substring, "chart-bar", sets)
		if via != "name" {
			t.Errorf("matched_via = %q, want name", via)
		}
		if len(flat) != 1 || flat[0].name != "chart-bar" {
			t.Errorf("flat = %+v, want just chart-bar", flat)
		}
		if len(matches) != 0 {
			t.Errorf("concept_matches = %+v, want none", matches)
		}
	})

	t.Run("no match at all returns the concept vocabulary", func(t *testing.T) {
		flat, via, _, vocab := applyConceptSearch(nil, "zzznonsense", sets)
		if len(flat) != 0 {
			t.Errorf("flat = %+v, want empty", flat)
		}
		if via != "" {
			t.Errorf("matched_via = %q, want empty", via)
		}
		if len(vocab) < 100 {
			t.Errorf("vocabulary has %d concepts, want the full index so the next call is informed", len(vocab))
		}
	})

	t.Run("no filter is a pass-through", func(t *testing.T) {
		all := []iconSetName{{set: "outline", name: "a"}, {set: "outline", name: "b"}}
		flat, via, matches, vocab := applyConceptSearch(all, "", sets)
		if len(flat) != 2 || via != "" || matches != nil || vocab != nil {
			t.Errorf("unfiltered call should pass through unchanged, got %+v %q %+v %v", flat, via, matches, vocab)
		}
	})
}

func TestTemplateInfoCanonicalLayoutAvailability(t *testing.T) {
	cache := template.NewMemoryCache(time.Hour)
	for _, mode := range []string{"list", "compact"} {
		t.Run(mode, func(t *testing.T) {
			info, err := analyzeTemplateForSkillInfoOpts("../../templates/modern.pptx", cache, mode, skillInfoOptions{NoPreview: true})
			if err != nil {
				t.Fatal(err)
			}
			if info.CanonicalLayoutAvailability == nil {
				t.Fatal("list_templates must expose canonical availability even in its slim projection")
			}
			available := info.CanonicalLayoutAvailability.Available
			unavailable := info.CanonicalLayoutAvailability.Unavailable
			if !slices.Contains(available, "image-right") {
				t.Errorf("modern's picture beside subtitle should resolve image-right: %v", available)
			}
			for _, name := range []string{"image-left", "agenda", "quote"} {
				if !slices.Contains(unavailable, name) {
					t.Errorf("%s incorrectly advertised as available: %v", name, unavailable)
				}
			}
			if info.CanonicalLayoutIDs["image-right"] != "slideLayout2" {
				t.Errorf("image-right binding = %q, want slideLayout2", info.CanonicalLayoutIDs["image-right"])
			}
		})
	}
}
