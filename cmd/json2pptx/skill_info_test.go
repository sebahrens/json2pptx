package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/layoutpreview"
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
		{"panel_layout", []string{"panels"}, "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\"|\"stylish_panels\""},
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

func TestAnalyzeTemplateForSkillInfo_TableStylesAllTemplates(t *testing.T) {
	templates := []string{
		"../../templates/forest-green.pptx",
		"../../templates/midnight-blue.pptx",
		"../../templates/modern-template.pptx",
		"../../templates/warm-coral.pptx",
	}

	cache := template.NewMemoryCache(24 * time.Hour)

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
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

// TestAnalyzeTemplateForSkillInfo_NoCanonicalMetadataInListMode confirms the
// slim list projection (MCP fields=compact) omits the canonical/semantic
// metadata entirely.
func TestAnalyzeTemplateForSkillInfo_NoCanonicalMetadataInListMode(t *testing.T) {
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
	if info.SemanticAccents != nil {
		t.Error("list mode: semantic_accents should be omitted")
	}
	if info.SHA256 != "" {
		t.Error("list mode: sha256 should be omitted (compact+full only)")
	}
}
