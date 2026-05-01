package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
)

func mcpResolveThemeTool() mcp.Tool {
	return mcp.NewTool("resolve_theme",
		mcp.WithDescription("Resolve theme colors and fonts for a template. Returns the hex value that each scheme color name (accent1, dk1, lt1, etc.) maps to, plus font families. Use this to preview the palette before authoring slides, avoiding color clashes with the template theme."),
		mcp.WithString("template_name",
			mcp.Required(),
			mcp.Description("Template name (e.g., midnight-blue). Use list_templates to discover available names."),
		),
		mcp.WithString("color_names",
			mcp.Description("Comma-separated list of color names to resolve (e.g., \"accent1,accent2,dk1\"). Omit to return all theme colors."),
		),
	)
}

// resolveThemeResponse is the JSON envelope for resolve_theme.
type resolveThemeResponse struct {
	Template    string                     `json:"template"`
	Colors      map[string]string          `json:"colors"`
	ColorRoles  *skillColorRoles           `json:"color_roles"`
	Fonts       resolveThemeFonts          `json:"fonts"`
	ResolvedFor []string                   `json:"resolved_for,omitempty"`
	Unknown     []resolveThemeUnknownColor `json:"unknown,omitempty"`
}

// resolveThemeFonts describes the font families in the theme.
type resolveThemeFonts struct {
	Major resolveThemeFontEntry `json:"major"`
	Minor resolveThemeFontEntry `json:"minor"`
}

// resolveThemeFontEntry describes a single font slot.
type resolveThemeFontEntry struct {
	Latin string `json:"latin"`
}

// resolveThemeUnknownColor is returned for color names not found in the theme.
type resolveThemeUnknownColor struct {
	Name       string `json:"name"`
	DidYouMean string `json:"did_you_mean,omitempty"`
}

func (mc *mcpConfig) handleResolveTheme(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateName, err := request.RequireString("template_name")
	if err != nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "template_name is required"), nil
	}

	// Resolve template path.
	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	// Parse theme from template.
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("failed to open template: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	theme := template.ParseTheme(reader)

	// Build full color map.
	allColors := make(map[string]string, len(theme.Colors))
	allColorNames := make([]string, 0, len(theme.Colors))
	for _, c := range theme.Colors {
		allColors[c.Name] = c.RGB
		allColorNames = append(allColorNames, c.Name)
	}

	// Determine which colors to return.
	var requestedNames []string
	if colorNamesStr, err := request.RequireString("color_names"); err == nil && colorNamesStr != "" {
		for _, name := range strings.Split(colorNamesStr, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				requestedNames = append(requestedNames, name)
			}
		}
	}

	colors := allColors
	var unknown []resolveThemeUnknownColor
	var resolvedFor []string

	if len(requestedNames) > 0 {
		colors = make(map[string]string, len(requestedNames))
		resolvedFor = requestedNames
		for _, name := range requestedNames {
			if hex, ok := allColors[name]; ok {
				colors[name] = hex
			} else {
				entry := resolveThemeUnknownColor{Name: name}
				if match, _ := generator.ClosestMatch(name, allColorNames, 3); match != "" {
					entry.DidYouMean = match
				}
				unknown = append(unknown, entry)
			}
		}
	}

	// Strip template name from path.
	name := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")

	resp := resolveThemeResponse{
		Template: name,
		Colors:   colors,
		ColorRoles: buildColorRoles(theme.Colors),
		Fonts: resolveThemeFonts{
			Major: resolveThemeFontEntry{Latin: theme.TitleFont},
			Minor: resolveThemeFontEntry{Latin: theme.BodyFont},
		},
		ResolvedFor: resolvedFor,
		Unknown:     unknown,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

