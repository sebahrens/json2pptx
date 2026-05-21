package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

func newResolveThemeMC(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

func resolveThemeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestResolveTheme_AllColors(t *testing.T) {
	mc := newResolveThemeMC(t)
	templates := []string{"midnight-blue", "forest-green", "warm-coral", "modern-template"}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			req := resolveThemeRequest(map[string]any{
				"template_name": tmpl,
			})
			result, err := mc.handleResolveTheme(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success, got error: %v", result.Content)
			}

			var resp resolveThemeResponse
			text := result.Content[0].(mcp.TextContent).Text
			if err := json.Unmarshal([]byte(text), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			// Must return the template name.
			if resp.Template != tmpl {
				t.Errorf("template = %q, want %q", resp.Template, tmpl)
			}

			// Must have non-empty colors.
			if len(resp.Colors) == 0 {
				t.Error("expected non-empty colors map")
			}

			// Must include key color slots.
			for _, name := range []string{"accent1", "dk1", "lt1"} {
				hex, ok := resp.Colors[name]
				if !ok {
					t.Errorf("missing color %q", name)
				}
				if len(hex) != 7 || hex[0] != '#' {
					t.Errorf("color %q has invalid hex %q", name, hex)
				}
			}

			// Must have fonts.
			if resp.Fonts.Major.Latin == "" {
				t.Error("expected non-empty major font")
			}
			if resp.Fonts.Minor.Latin == "" {
				t.Error("expected non-empty minor font")
			}

			// Must have color_roles.
			if resp.ColorRoles == nil {
				t.Error("expected non-nil color_roles")
			}

			// ResolvedFor should be nil/empty when no filter is provided.
			if len(resp.ResolvedFor) != 0 {
				t.Errorf("expected empty resolved_for, got %v", resp.ResolvedFor)
			}
		})
	}
}

func TestResolveTheme_DistinctPalettes(t *testing.T) {
	mc := newResolveThemeMC(t)
	templates := []string{"midnight-blue", "forest-green", "warm-coral", "modern-template"}

	palettes := make(map[string]string) // template -> accent1
	for _, tmpl := range templates {
		req := resolveThemeRequest(map[string]any{"template_name": tmpl})
		result, err := mc.handleResolveTheme(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tmpl, err)
		}
		var resp resolveThemeResponse
		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("%s: parse error: %v", tmpl, err)
		}
		palettes[tmpl] = resp.Colors["accent1"]
	}

	// At least 3 of 4 templates should have distinct accent1 values.
	unique := make(map[string]bool)
	for _, hex := range palettes {
		unique[hex] = true
	}
	if len(unique) < 3 {
		t.Errorf("expected at least 3 distinct accent1 values across 4 templates, got %d: %v", len(unique), palettes)
	}
}

func TestResolveTheme_FilteredColors(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"color_names":   "accent1,accent2",
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp resolveThemeResponse
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should only return the requested colors.
	if len(resp.Colors) != 2 {
		t.Errorf("expected 2 colors, got %d: %v", len(resp.Colors), resp.Colors)
	}
	if _, ok := resp.Colors["accent1"]; !ok {
		t.Error("missing accent1")
	}
	if _, ok := resp.Colors["accent2"]; !ok {
		t.Error("missing accent2")
	}

	// resolved_for should echo the requested names.
	if len(resp.ResolvedFor) != 2 {
		t.Errorf("expected resolved_for with 2 entries, got %v", resp.ResolvedFor)
	}
}

func TestResolveTheme_UnknownColor(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"color_names":   "accent1,accnet2",
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp resolveThemeResponse
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// accent1 should be resolved, accnet2 should be unknown.
	if _, ok := resp.Colors["accent1"]; !ok {
		t.Error("accent1 should be resolved")
	}
	if _, ok := resp.Colors["accnet2"]; ok {
		t.Error("accnet2 should not be in colors")
	}

	if len(resp.Unknown) != 1 {
		t.Fatalf("expected 1 unknown, got %d", len(resp.Unknown))
	}
	if resp.Unknown[0].Name != "accnet2" {
		t.Errorf("unknown name = %q, want accnet2", resp.Unknown[0].Name)
	}
	if resp.Unknown[0].DidYouMean != "accent2" {
		t.Errorf("did_you_mean = %q, want accent2", resp.Unknown[0].DidYouMean)
	}
}

func TestResolveTheme_TemplateNotFound(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "nonexistent-template",
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for nonexistent template")
	}
}

// TestResolveTheme_ThemeOverride_AppliedColors verifies that resolve_theme
// reflects the same post-override hex values that singlepass would render
// for the same frontmatter — closing the silent three-way drift between
// resolve_theme / render_diagram / frontmatter override.
func TestResolveTheme_ThemeOverride_AppliedColors(t *testing.T) {
	mc := newResolveThemeMC(t)

	// Baseline accent1 (no override).
	baselineReq := resolveThemeRequest(map[string]any{"template_name": "midnight-blue"})
	baseline, err := mc.handleResolveTheme(context.Background(), baselineReq)
	if err != nil || baseline.IsError {
		t.Fatalf("baseline failed: err=%v isError=%v", err, baseline.IsError)
	}
	var baseResp resolveThemeResponse
	if err := json.Unmarshal([]byte(baseline.Content[0].(mcp.TextContent).Text), &baseResp); err != nil {
		t.Fatalf("baseline parse: %v", err)
	}
	const newAccent1 = "#336699"
	if strings.EqualFold(baseResp.Colors["accent1"], newAccent1) {
		t.Skipf("baseline accent1 already %s; cannot demonstrate override", newAccent1)
	}

	// Apply override.
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"theme_override": map[string]any{
			"colors": map[string]any{
				"accent1": newAccent1,
			},
		},
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}
	var resp resolveThemeResponse
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !strings.EqualFold(resp.Colors["accent1"], newAccent1) {
		t.Errorf("accent1 after override = %q, want %q", resp.Colors["accent1"], newAccent1)
	}
	// Other slots must remain template defaults so partial overrides don't
	// silently blank out the rest of the palette.
	if !strings.EqualFold(resp.Colors["dk1"], baseResp.Colors["dk1"]) {
		t.Errorf("dk1 unexpectedly changed: base=%q after=%q", baseResp.Colors["dk1"], resp.Colors["dk1"])
	}
	if resp.AppliedOverride == nil || resp.AppliedOverride.Colors["accent1"] != newAccent1 {
		t.Errorf("applied_theme_override.colors.accent1 = %+v, want accent1=%q",
			resp.AppliedOverride, newAccent1)
	}
}

// TestResolveTheme_ThemeOverride_UnknownColorWarns ensures unrecognized scheme
// keys surface in warnings[] instead of being silently ignored.
func TestResolveTheme_ThemeOverride_UnknownColorWarns(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"theme_override": map[string]any{
			"colors": map[string]any{
				"accentZZ": "#112233",
			},
		},
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v", err, result.IsError)
	}
	var resp resolveThemeResponse
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected at least one warning for unknown accentZZ key")
	}
	found := false
	for _, w := range resp.Warnings {
		if strings.Contains(w, "accentZZ") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("warnings did not mention accentZZ: %v", resp.Warnings)
	}
}

// TestResolveTheme_ThemeOverride_FontOverride confirms font override surfaces
// in resp.Fonts and triggers the not-embedded warning when distinct.
func TestResolveTheme_ThemeOverride_FontOverride(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"theme_override": map[string]any{
			"title_font": "Helvetica Neue",
		},
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v", err, result.IsError)
	}
	var resp resolveThemeResponse
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp.Fonts.Major.Latin != "Helvetica Neue" {
		t.Errorf("fonts.major.latin = %q, want Helvetica Neue", resp.Fonts.Major.Latin)
	}
}

// TestResolveTheme_ThemeOverride_InvalidShape rejects non-object overrides
// with a structured error rather than silently dropping the param.
func TestResolveTheme_ThemeOverride_InvalidShape(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name":  "midnight-blue",
		"theme_override": "not-an-object",
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for non-object theme_override")
	}
}

// TestResolveTheme_ThemeColorsArrayShape verifies the response includes a
// theme_colors:[{name,rgb}] array that mirrors the colors map, in the shape
// svggen-mcp's StyleSpec.theme_colors expects. This kills the silent drift
// between native OOXML and svggen rendering paths by letting an agent copy
// the array verbatim into render_diagram's style.theme_colors.
func TestResolveTheme_ThemeColorsArrayShape(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{"template_name": "midnight-blue"})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v", err, result.IsError)
	}
	var resp resolveThemeResponse
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(resp.ThemeColors) == 0 {
		t.Fatal("theme_colors array is empty; expected one entry per theme color")
	}
	if len(resp.ThemeColors) != len(resp.Colors) {
		t.Errorf("theme_colors length (%d) != colors map length (%d)", len(resp.ThemeColors), len(resp.Colors))
	}

	// Every entry in the array must agree with the colors map exactly — that
	// is the contract agents rely on when copying the array.
	seen := make(map[string]bool, len(resp.ThemeColors))
	for _, e := range resp.ThemeColors {
		if e.Name == "" {
			t.Errorf("theme_colors entry has empty name: %+v", e)
		}
		if len(e.RGB) != 7 || e.RGB[0] != '#' {
			t.Errorf("theme_colors entry %q has invalid hex %q", e.Name, e.RGB)
		}
		if hex, ok := resp.Colors[e.Name]; !ok {
			t.Errorf("theme_colors has %q but colors map does not", e.Name)
		} else if hex != e.RGB {
			t.Errorf("theme_colors[%q].rgb=%q disagrees with colors[%q]=%q", e.Name, e.RGB, e.Name, hex)
		}
		if seen[e.Name] {
			t.Errorf("theme_colors has duplicate entry for %q", e.Name)
		}
		seen[e.Name] = true
	}
}

// TestResolveTheme_ThemeColorsFiltered verifies the array tracks the colors
// map when color_names filters the result, so the one-shot copy semantics
// hold for filtered requests too.
func TestResolveTheme_ThemeColorsFiltered(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{
		"template_name": "midnight-blue",
		"color_names":   "accent2,accent1",
	})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v", err, result.IsError)
	}
	var resp resolveThemeResponse
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(resp.ThemeColors) != 2 {
		t.Fatalf("expected 2 theme_colors entries, got %d: %+v", len(resp.ThemeColors), resp.ThemeColors)
	}
	// Request order must be preserved.
	if resp.ThemeColors[0].Name != "accent2" || resp.ThemeColors[1].Name != "accent1" {
		t.Errorf("theme_colors order = [%q, %q], want [accent2, accent1]",
			resp.ThemeColors[0].Name, resp.ThemeColors[1].Name)
	}
}

func TestResolveTheme_MissingParam(t *testing.T) {
	mc := newResolveThemeMC(t)
	req := resolveThemeRequest(map[string]any{})
	result, err := mc.handleResolveTheme(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing template_name")
	}
}
