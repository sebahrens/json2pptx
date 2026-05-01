package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/templatesettings"
)

func settingsTestConfig(t *testing.T) *mcpConfig {
	t.Helper()
	// Copy a real template into a temp dir so we have a writable templates dir.
	tmpDir := t.TempDir()
	src := "../../templates/midnight-blue.pptx"
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "midnight-blue.pptx"), data, 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return &mcpConfig{
		templatesDir: tmpDir,
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

func TestMCPListTemplateSettings_Empty(t *testing.T) {
	mc := settingsTestConfig(t)
	result, err := mc.handleListTemplateSettings(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var resp listTemplateSettingsResponse
	b, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Template != "midnight-blue" {
		t.Errorf("template = %q", resp.Template)
	}
	if len(resp.TableStyles) != 0 {
		t.Errorf("expected empty table_styles, got %d", len(resp.TableStyles))
	}
}

func TestMCPRegisterTemplateSetting_GatedOff(t *testing.T) {
	mc := settingsTestConfig(t)
	// Ensure env is not set.
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "")

	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "test-style",
		"definition":    map[string]any{"style_id": "@template-default"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error when write gate is off")
	}
	b, _ := json.Marshal(result.StructuredContent)
	if got := string(b); !strings.Contains(got, "SETTINGS_WRITE_DISABLED") {
		t.Errorf("expected SETTINGS_WRITE_DISABLED code, got: %s", got)
	}
}

func TestMCPDeleteTemplateSetting_GatedOff(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "")

	result, err := mc.handleDeleteTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "test-style",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error when write gate is off")
	}
}

func TestMCPRegisterAndListTemplateSetting(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	// Register a table style with @template-default.
	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "brand-table",
		"definition": map[string]any{
			"style_id":        "@template-default",
			"use_table_style": true,
			"banded_rows":     false,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		b, _ := json.Marshal(result.StructuredContent)
		t.Fatalf("register failed: %s", b)
	}

	var regResp registerTemplateSettingResponse
	b, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(b, &regResp); err != nil {
		t.Fatal(err)
	}
	if !regResp.Written {
		t.Error("expected written=true")
	}

	// List and verify.
	listResult, err := mc.handleListTemplateSettings(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var listResp listTemplateSettingsResponse
	b, _ = json.Marshal(listResult.StructuredContent)
	if err := json.Unmarshal(b, &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.TableStyles) != 1 {
		t.Fatalf("expected 1 table style, got %d", len(listResp.TableStyles))
	}
	ts := listResp.TableStyles["brand-table"]
	if ts.StyleID != "@template-default" {
		t.Errorf("style_id = %q", ts.StyleID)
	}
}

func TestMCPRegisterTemplateSetting_BadStyleID(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	// When the template has table styles declared, an unknown GUID should be rejected.
	// When the template has NO table styles (like midnight-blue), validation is skipped
	// and the registration succeeds — matching eq3k behavior.
	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "bad-style",
		"definition": map[string]any{
			"style_id": "{00000000-0000-0000-0000-000000000000}",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	// midnight-blue has no table styles in tableStyles.xml, so validation
	// is skipped and registration succeeds.
	if result.IsError {
		b, _ := json.Marshal(result.StructuredContent)
		t.Fatalf("unexpected error: %s", b)
	}
}

func TestMCPRegisterCellStyle_BadColor(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "cell_styles",
		"name":          "bad-cell",
		"definition": map[string]any{
			"fill": map[string]any{"color": "nonexistent_color"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown theme color")
	}
	b, _ := json.Marshal(result.StructuredContent)
	if got := string(b); !strings.Contains(got, "UNKNOWN_THEME_COLOR") {
		t.Errorf("expected UNKNOWN_THEME_COLOR code, got: %s", got)
	}
}

func TestMCPDeleteTemplateSetting(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	// Register then delete.
	_, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "to-delete",
		"definition":    map[string]any{"style_id": "@template-default"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	result, err := mc.handleDeleteTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "to-delete",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		b, _ := json.Marshal(result.StructuredContent)
		t.Fatalf("delete failed: %s", b)
	}

	var delResp deleteTemplateSettingResponse
	b, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(b, &delResp); err != nil {
		t.Fatal(err)
	}
	if !delResp.Removed {
		t.Error("expected removed=true")
	}
}

func TestMCPRegisterTemplateSetting_InvalidName(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"kind":          "table_styles",
		"name":          "has spaces",
		"definition":    map[string]any{"style_id": "@template-default"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid name")
	}
}

func TestMCPRegisterTemplateSetting_PathTraversal(t *testing.T) {
	mc := settingsTestConfig(t)
	t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")

	result, err := mc.handleRegisterTemplateSetting(context.Background(), makeRequest(map[string]any{
		"template_name": "../escape",
		"kind":          "table_styles",
		"name":          "test",
		"definition":    map[string]any{"style_id": "@template-default"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for path traversal")
	}
}

func TestResolveNamedSettings_TableStyle(t *testing.T) {
	settings := &templatesettings.File{
		Template: "test",
		TableStyles: map[string]templatesettings.TableStyleDef{
			"brand-table": {
				StyleID:       "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}",
				UseTableStyle: true,
			},
		},
		CellStyles: make(map[string]templatesettings.CellStyleDef),
	}

	boolFalse := false
	input := &PresentationInput{
		Template: "test",
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &TableInput{
							Headers: []string{"A", "B"},
							Style: &TableStyleInput{
								StyleID: "brand-table", // Named reference.
								Striped: &boolFalse,    // Inline override should be preserved.
							},
						},
					},
				},
			},
		},
	}

	resolveNamedSettings(input, settings)

	ts := input.Slides[0].Content[0].TableValue.Style
	if ts.StyleID != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("style_id should be resolved to GUID, got %q", ts.StyleID)
	}
	if !ts.UseTableStyle {
		t.Error("use_table_style should be set from named definition")
	}
	if ts.Striped == nil || *ts.Striped != false {
		t.Error("inline striped override should be preserved")
	}
}

func TestResolveNamedSettings_GUIDNotResolved(t *testing.T) {
	settings := &templatesettings.File{
		Template:    "test",
		TableStyles: map[string]templatesettings.TableStyleDef{},
		CellStyles:  make(map[string]templatesettings.CellStyleDef),
	}

	input := &PresentationInput{
		Template: "test",
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &TableInput{
							Headers: []string{"A"},
							Style: &TableStyleInput{
								StyleID: "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}",
							},
						},
					},
				},
			},
		},
	}

	resolveNamedSettings(input, settings)

	// GUID should pass through unchanged.
	if input.Slides[0].Content[0].TableValue.Style.StyleID != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Error("GUID should not be modified")
	}
}
