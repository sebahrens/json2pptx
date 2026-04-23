package main

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestAnalyzeTemplateForSkillInfo_FiltersOtherPlaceholders(t *testing.T) {
	templatePath := "../../templates/forest-green.pptx"

	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo(templatePath, cache, "full")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	// Verify no placeholder has type "other" in any layout
	for _, layout := range info.Layouts {
		for _, ph := range layout.Placeholders {
			if ph.Type == "other" {
				t.Errorf("layout %q contains placeholder %q with type %q; "+
					"internal OOXML metadata placeholders should be filtered out",
					layout.Name, ph.ID, ph.Type)
			}
		}
	}

	// Verify we still have some placeholders (sanity check)
	totalPhs := 0
	for _, layout := range info.Layouts {
		totalPhs += len(layout.Placeholders)
	}
	if totalPhs == 0 {
		t.Error("expected at least one placeholder across all layouts after filtering")
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
		{"panel_layout", []string{"panels"}, "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\""},
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

func TestBuildPatternEntries_FullModeWithoutFlag_OmitsFull(t *testing.T) {
	// Simulates the effective-mode logic in runSkillInfo: when mode=full but
	// includeFullSchemas=false, buildPatternEntries receives "compact".
	includeFullSchemas := false
	mode := "full"

	effectiveMode := mode
	if effectiveMode == "full" && !includeFullSchemas {
		effectiveMode = "compact"
	}

	compact, full := buildPatternEntries(effectiveMode)
	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}
	if full != nil {
		t.Errorf("expected nil patterns_full when includeFullSchemas=false, got %d entries", len(full))
	}
}

func TestBuildPatternEntries_FullModeWithFlag_IncludesFull(t *testing.T) {
	// Simulates the effective-mode logic in runSkillInfo: when mode=full and
	// includeFullSchemas=true, buildPatternEntries receives "full".
	includeFullSchemas := true
	mode := "full"

	effectiveMode := mode
	if effectiveMode == "full" && !includeFullSchemas {
		effectiveMode = "compact"
	}

	compact, full := buildPatternEntries(effectiveMode)
	if len(compact) < 8 {
		t.Fatalf("expected at least 8 compact patterns, got %d", len(compact))
	}
	if len(full) < 8 {
		t.Fatalf("expected at least 8 full patterns when includeFullSchemas=true, got %d", len(full))
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
		{Name: "accent2", RGB: "#D4463A"}, // red — passes
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
	if roles.SecondaryFill != "accent2" {
		t.Errorf("SecondaryFill = %q, want accent2", roles.SecondaryFill)
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

	// Falls back to accent1/accent2 defaults even though they aren't safe
	if roles.PrimaryFill != "accent1" {
		t.Errorf("PrimaryFill = %q, want accent1 (fallback)", roles.PrimaryFill)
	}
	if roles.SecondaryFill != "accent2" {
		t.Errorf("SecondaryFill = %q, want accent2 (fallback)", roles.SecondaryFill)
	}
	if len(roles.WhiteTextSafe) != 0 {
		t.Errorf("WhiteTextSafe = %v, want empty", roles.WhiteTextSafe)
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

func TestAnalyzeTemplateForSkillInfo_NoColorRolesInListMode(t *testing.T) {
	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo("../../templates/midnight-blue.pptx", cache, "list")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	if info.ColorRoles != nil {
		t.Error("ColorRoles should be nil in list mode")
	}
}
