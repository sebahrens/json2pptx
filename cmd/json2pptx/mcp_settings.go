package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/templatesettings"
)

const (
	// envAllowSettingsWrite gates write operations on template settings.
	envAllowSettingsWrite = "JSON2PPTX_ALLOW_SETTINGS_WRITE"
)

// settingsWriteAllowed returns true when the env gate is set.
func settingsWriteAllowed() bool {
	return os.Getenv(envAllowSettingsWrite) == "1"
}

// --- Tool definitions ---

func mcpListTemplateSettingsTool() mcp.Tool {
	return mcp.NewTool("list_template_settings",
		mcp.WithDescription("List named table_styles and cell_styles registered for a template. These named styles can be referenced from deck JSON by name instead of repeating full definitions. Read-only — always available."),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue). Use list_templates to discover available names."),
		),
	)
}

func mcpRegisterTemplateSettingTool() mcp.Tool {
	return mcp.NewTool("register_template_setting",
		mcp.WithDescription("Register a named table_style or cell_style for a template. The setting is persisted in a YAML sidecar file and can be referenced by name from deck JSON. Idempotent — re-registering with the same name overwrites. Requires JSON2PPTX_ALLOW_SETTINGS_WRITE=1."),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue)."),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Setting kind: table_styles or cell_styles."),
			mcp.Enum("table_styles", "cell_styles"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Setting name (alphanumeric, hyphens, underscores; max 64 chars)."),
		),
		mcp.WithObject("definition",
			mcp.Required(),
			mcp.Description(`The style definition object.

For table_styles:
  {"style_id":"{GUID}","use_table_style":true,"header_row":true,"banded_rows":false}
  style_id can be a GUID from list_templates table_styles, or "@template-default".

For cell_styles:
  {"fill":{"color":"accent1","alpha":0.15},"border":{"color":"accent1","weight":1.5},"text_align":"center"}`),
		),
	)
}

func mcpDeleteTemplateSettingTool() mcp.Tool {
	return mcp.NewTool("delete_template_setting",
		mcp.WithDescription("Delete a named table_style or cell_style from a template's settings. Requires JSON2PPTX_ALLOW_SETTINGS_WRITE=1."),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue)."),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Setting kind: table_styles or cell_styles."),
			mcp.Enum("table_styles", "cell_styles"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Setting name to delete."),
		),
	)
}

// --- Handlers ---

type listTemplateSettingsResponse struct {
	Template    string                                    `json:"template"`
	TableStyles map[string]templatesettings.TableStyleDef `json:"table_styles"`
	CellStyles  map[string]templatesettings.CellStyleDef  `json:"cell_styles"`
}

func (mc *mcpConfig) handleListTemplateSettings(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateName, err := request.RequireString("template_name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "template_name is required"), nil
	}
	if err := templatesettings.ValidateTemplateName(templateName); err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	// Verify template exists.
	if _, cleanup, err := resolveTemplatePath(templateName, mc.templatesDir); err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	} else {
		cleanup()
	}

	// Resolve the templates directory for settings files.
	settingsDir := mc.resolveSettingsDir()

	f, err := templatesettings.Load(settingsDir, templateName)
	if err != nil {
		return api.MCPSimpleError("SETTINGS_ERROR", fmt.Sprintf("failed to load settings: %v", err)), nil
	}

	resp := listTemplateSettingsResponse{
		Template:    templateName,
		TableStyles: f.TableStyles,
		CellStyles:  f.CellStyles,
	}
	return api.MCPSuccessResult(ctx, resp)
}

type registerTemplateSettingResponse struct {
	Written  bool   `json:"written"`
	Path     string `json:"path"`
	Template string `json:"template"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

func (mc *mcpConfig) handleRegisterTemplateSetting(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !settingsWriteAllowed() {
		return api.MCPSimpleError("SETTINGS_WRITE_DISABLED",
			"Template settings writes are disabled. Set JSON2PPTX_ALLOW_SETTINGS_WRITE=1 to enable."), nil
	}

	templateName, err := request.RequireString("template_name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "template_name is required"), nil
	}
	if err := templatesettings.ValidateTemplateName(templateName); err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	kindStr, err := request.RequireString("kind")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "kind is required"), nil
	}
	kind, err := templatesettings.ValidateKind(kindStr)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "name is required"), nil
	}
	if err := templatesettings.ValidateName(name); err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	defRaw, ok := request.GetArguments()["definition"]
	if !ok || defRaw == nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "definition is required"), nil
	}

	// Re-marshal the definition so we can unmarshal into the typed struct.
	defJSON, err := json.Marshal(defRaw)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", fmt.Sprintf("failed to encode definition: %v", err)), nil
	}

	// Verify template exists and open it for style_id validation.
	templatePath, cleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer cleanup()

	settingsDir := mc.resolveSettingsDir()

	// Load existing settings.
	f, err := templatesettings.Load(settingsDir, templateName)
	if err != nil {
		return api.MCPSimpleError("SETTINGS_ERROR", fmt.Sprintf("failed to load settings: %v", err)), nil
	}

	// Validate and insert the definition.
	switch kind {
	case templatesettings.KindTableStyle:
		var def templatesettings.TableStyleDef
		if err := json.Unmarshal(defJSON, &def); err != nil {
			return api.MCPSimpleError("INVALID_PARAMETER", fmt.Sprintf("invalid table_style definition: %v", err)), nil
		}
		// Validate style_id against template's table styles.
		if def.StyleID != "" && def.StyleID != template.TemplateDefaultSentinel {
			if err := mc.validateStyleIDAgainstTemplate(templatePath, def.StyleID); err != nil {
				return err, nil
			}
		}
		f.TableStyles[name] = def

	case templatesettings.KindCellStyle:
		var def templatesettings.CellStyleDef
		if err := json.Unmarshal(defJSON, &def); err != nil {
			return api.MCPSimpleError("INVALID_PARAMETER", fmt.Sprintf("invalid cell_style definition: %v", err)), nil
		}
		// Validate fill color resolves through theme if it looks like a scheme name.
		if def.Fill != nil && def.Fill.Color != "" {
			if err := mc.validateCellFillColor(templatePath, def.Fill.Color); err != nil {
				return err, nil
			}
		}
		f.CellStyles[name] = def
	}

	path, err := templatesettings.Save(settingsDir, f)
	if err != nil {
		return api.MCPSimpleError("SETTINGS_ERROR", fmt.Sprintf("failed to save settings: %v", err)), nil
	}

	resp := registerTemplateSettingResponse{
		Written:  true,
		Path:     path,
		Template: templateName,
		Kind:     kindStr,
		Name:     name,
	}
	return api.MCPSuccessResult(ctx, resp)
}

type deleteTemplateSettingResponse struct {
	Removed  bool   `json:"removed"`
	Template string `json:"template"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

func (mc *mcpConfig) handleDeleteTemplateSetting(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !settingsWriteAllowed() {
		return api.MCPSimpleError("SETTINGS_WRITE_DISABLED",
			"Template settings writes are disabled. Set JSON2PPTX_ALLOW_SETTINGS_WRITE=1 to enable."), nil
	}

	templateName, err := request.RequireString("template_name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "template_name is required"), nil
	}
	if err := templatesettings.ValidateTemplateName(templateName); err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	kindStr, err := request.RequireString("kind")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "kind is required"), nil
	}
	kind, err := templatesettings.ValidateKind(kindStr)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "name is required"), nil
	}

	settingsDir := mc.resolveSettingsDir()

	removed, err := templatesettings.Delete(settingsDir, templateName, kind, name)
	if err != nil {
		return api.MCPSimpleError("SETTINGS_ERROR", fmt.Sprintf("failed to delete setting: %v", err)), nil
	}

	resp := deleteTemplateSettingResponse{
		Removed:  removed,
		Template: templateName,
		Kind:     kindStr,
		Name:     name,
	}
	return api.MCPSuccessResult(ctx, resp)
}

// resolveInputNamedSettings loads the template's settings file and resolves
// any named style references in the presentation input. Best-effort — if the
// settings file doesn't exist or template name is empty, this is a no-op.
func (mc *mcpConfig) resolveInputNamedSettings(input *PresentationInput) {
	if input == nil || input.Template == "" {
		return
	}
	settingsDir := mc.resolveSettingsDir()
	settings, err := templatesettings.Load(settingsDir, input.Template)
	if err != nil {
		return // Best-effort: skip if settings can't be loaded.
	}
	if len(settings.TableStyles) == 0 && len(settings.CellStyles) == 0 {
		return // No settings registered — nothing to resolve.
	}
	resolveNamedSettings(input, settings)
}

// --- Helpers ---

// resolveSettingsDir returns the filesystem directory where settings YAML files
// should be stored. For on-disk templates it's the templates directory; for
// embedded templates it's not supported (settings require a writable directory).
func (mc *mcpConfig) resolveSettingsDir() string {
	dir, embedded := resolveTemplatesDir(mc.templatesDir)
	if embedded {
		// Embedded templates can't have sidecar files, but we still need a
		// readable path for Load (which returns empty File for missing files).
		return dir
	}
	return dir
}

// validateStyleIDAgainstTemplate opens the template and checks whether the
// style_id GUID exists in its tableStyles.xml.
func (mc *mcpConfig) validateStyleIDAgainstTemplate(templatePath, styleID string) *mcp.CallToolResult {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("failed to open template for validation: %v", err))
	}
	defer func() { _ = reader.Close() }()

	styles := reader.TableStyles()
	styleByID := make(map[string]string, len(styles))
	available := make([]string, 0, len(styles))
	for _, s := range styles {
		styleByID[s.ID] = s.Name
		available = append(available, fmt.Sprintf("%s (%s)", s.ID, s.Name))
	}

	// If no table styles are declared, skip validation.
	if len(styleByID) == 0 {
		return nil
	}

	if _, ok := styleByID[styleID]; ok {
		return nil
	}

	msg := fmt.Sprintf("style_id %q not found in template table styles; available: %v", styleID, available)
	return api.MCPSimpleError("UNKNOWN_TABLE_STYLE_ID", msg)
}

// validateCellFillColor checks that a cell fill color looks valid.
// Hex colors (#RRGGBB) are always accepted. Scheme names (accent1, dk1, etc.)
// are validated against the template theme.
func (mc *mcpConfig) validateCellFillColor(templatePath, color string) *mcp.CallToolResult {
	// Hex colors are always valid.
	if len(color) > 0 && color[0] == '#' {
		return nil
	}

	// Treat as scheme name — validate against theme.
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("failed to open template for color validation: %v", err))
	}
	defer func() { _ = reader.Close() }()

	theme := template.ParseTheme(reader)
	for _, c := range theme.Colors {
		if c.Name == color {
			return nil
		}
	}

	// Collect valid scheme names for the error message.
	var names []string
	for _, c := range theme.Colors {
		names = append(names, c.Name)
	}
	msg := fmt.Sprintf("fill color %q is not a recognized hex color or theme scheme name; available scheme names: %v", color, names)
	return api.MCPSimpleError("UNKNOWN_THEME_COLOR", msg)
}
