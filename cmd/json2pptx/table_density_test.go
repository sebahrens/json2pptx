package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

func TestTableDensityGuide_NoArgs(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleTableDensityGuide(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp densityGuideResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Tiers) == 0 {
		t.Error("expected density tiers, got none")
	}
	if resp.Limits.MaxRows != 7 {
		t.Errorf("expected max_rows=7, got %d", resp.Limits.MaxRows)
	}
	if resp.Limits.MaxColumns != 6 {
		t.Errorf("expected max_columns=6, got %d", resp.Limits.MaxColumns)
	}
	if resp.Limits.MinFontPt != 9 {
		t.Errorf("expected min_font_pt=9, got %d", resp.Limits.MinFontPt)
	}
	if resp.Template != "" {
		t.Errorf("expected empty template, got %q", resp.Template)
	}
	if resp.TableStyles != nil {
		t.Error("expected nil table_styles without template")
	}
}

func TestTableDensityGuide_WithTemplate(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleTableDensityGuide(context.Background(), makeRequest(map[string]any{
		"template": "modern-template",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp densityGuideResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Template != "modern-template" {
		t.Errorf("expected template=modern-template, got %q", resp.Template)
	}
	if len(resp.TableStyles) == 0 {
		t.Error("expected table_styles for modern-template, got none")
	}
	if len(resp.Tiers) == 0 {
		t.Error("expected density tiers even with template")
	}
}

func TestTableDensityGuide_UnknownTemplate(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleTableDensityGuide(context.Background(), makeRequest(map[string]any{
		"template": "nonexistent-template",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for unknown template")
	}
}

// TestTableDensityGuide_EmbeddedTemplate verifies that template-scoped density
// queries succeed when only embedded templates are reachable (no on-disk
// templates directory). Regression for go-slide-creator-xvm3: handler used to
// stat templatesDir/<name>.pptx directly, so embedded-only environments
// couldn't reach bundled templates that generation can use.
func TestTableDensityGuide_EmbeddedTemplate(t *testing.T) {
	withClearedTemplateEnv(t)
	mc := &mcpConfig{
		// Point at a non-existent on-disk dir so resolution must fall through
		// to embedded templates.
		templatesDir: filepath.Join(t.TempDir(), "no-such-dir"),
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	result, err := mc.handleTableDensityGuide(context.Background(), makeRequest(map[string]any{
		"template": "midnight-blue",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp densityGuideResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Template != "midnight-blue" {
		t.Errorf("expected template=midnight-blue, got %q", resp.Template)
	}
}

func TestTableDensityGuide_StyleIDWithoutTemplate(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleTableDensityGuide(context.Background(), makeRequest(map[string]any{
		"style_id": "some-style",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when style_id is provided without template")
	}
}
