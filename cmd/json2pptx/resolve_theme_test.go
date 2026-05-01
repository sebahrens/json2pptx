package main

import (
	"context"
	"encoding/json"
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
