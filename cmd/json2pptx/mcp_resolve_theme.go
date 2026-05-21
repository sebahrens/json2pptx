package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/themeinfo"
)

func mcpResolveThemeTool() mcp.Tool {
	return mcp.NewTool("resolve_theme",
		mcp.WithDescription("Resolve theme colors and fonts for a template. Returns the hex value that each scheme color name (accent1, dk1, lt1, etc.) maps to, plus font families. Use this to preview the palette before authoring slides, avoiding color clashes with the template theme. Pass theme_override to preview the palette after the same overrides that frontmatter would apply (singlepass parity)."),
		mcp.WithRawOutputSchema(outputSchemaResolveTheme),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue). Use list_templates to discover available names."),
		),
		mcp.WithString("color_names",
			mcp.Description("Comma-separated list of color names to resolve (e.g., \"accent1,accent2,dk1\"). Omit to return all theme colors."),
		),
		mcp.WithObject("theme_override",
			mcp.Description("Optional per-deck overrides applied before resolving — same shape as the frontmatter `theme_override` field. Fields: `colors` (map of scheme name → hex, e.g., {\"accent1\":\"#336699\"}), `title_font` (string), `body_font` (string). Returned colors/fonts reflect the post-override values; unrecognized color keys and non-embedded font swaps surface in `warnings[]`."),
		),
	)
}

// resolveThemeResponse is the JSON envelope for resolve_theme. It wraps the
// reusable themeinfo.Result with the MCP-specific applied-override echo and the
// color_roles view derived from skill_info's buildColorRoles.
type resolveThemeResponse struct {
	Template        string                   `json:"template"`
	Colors          map[string]string        `json:"colors"`
	ThemeColors     []themeinfo.ColorEntry   `json:"theme_colors"`
	ColorRoles      *skillColorRoles         `json:"color_roles"`
	Fonts           themeinfo.Fonts          `json:"fonts"`
	ResolvedFor     []string                 `json:"resolved_for,omitempty"`
	Unknown         []themeinfo.UnknownColor `json:"unknown,omitempty"`
	AppliedOverride *ThemeInput              `json:"applied_theme_override,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

func (mc *mcpConfig) handleResolveTheme(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateName, err := request.RequireString("template_name")
	if err != nil {
		return argMissing("resolve_theme", "template_name", "string", "midnight-blue", nextCallListTemplates()), nil
	}

	// Resolve template path.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	// Decode the optional theme_override argument into a ThemeInput.
	overrideInput, overrideErr := parseResolveThemeOverride(request)
	if overrideErr != nil {
		return argInvalidValue("resolve_theme", "INVALID_PARAMETER", "theme_override", overrideErr.Error(), "object", nil, nil), nil
	}

	// Parse the comma-separated color filter, dropping blanks.
	var colorNames []string
	if colorNamesStr, err := request.RequireString("color_names"); err == nil && colorNamesStr != "" {
		for _, name := range strings.Split(colorNamesStr, ",") {
			if name = strings.TrimSpace(name); name != "" {
				colorNames = append(colorNames, name)
			}
		}
	}

	result, err := themeinfo.Resolve(templatePath, themeinfo.Options{
		ColorNames: colorNames,
		Override:   overrideInput.ToThemeOverride(),
	})
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", err.Error()), nil
	}

	resp := resolveThemeResponse{
		Template:        result.Template,
		Colors:          result.Colors,
		ThemeColors:     result.ThemeColors,
		ColorRoles:      buildColorRoles(result.AllColors),
		Fonts:           result.Fonts,
		ResolvedFor:     result.ResolvedFor,
		Unknown:         result.Unknown,
		AppliedOverride: overrideInput,
		Warnings:        result.Warnings,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// parseResolveThemeOverride extracts the optional theme_override argument and
// decodes it into a ThemeInput. Returns (nil, nil) when the argument is absent
// or an empty object. Returns an error for malformed shapes so the handler can
// emit a structured INVALID_PARAMETER response instead of silently dropping it.
func parseResolveThemeOverride(request mcp.CallToolRequest) (*ThemeInput, error) {
	raw, ok := request.GetArguments()["theme_override"]
	if !ok || raw == nil {
		return nil, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("theme_override: expected object, got %T", raw)
	}
	if len(obj) == 0 {
		return nil, nil
	}
	buf, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("theme_override: %v", err)
	}
	var override ThemeInput
	if err := json.Unmarshal(buf, &override); err != nil {
		return nil, fmt.Errorf("theme_override: %v", err)
	}
	if len(override.Colors) == 0 && override.TitleFont == "" && override.BodyFont == "" {
		return nil, nil
	}
	return &override, nil
}
