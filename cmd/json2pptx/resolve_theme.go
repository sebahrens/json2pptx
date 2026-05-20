package main

import (
	"context"
	"encoding/json"
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

// resolveThemeResponse is the JSON envelope for resolve_theme.
type resolveThemeResponse struct {
	Template        string                     `json:"template"`
	Colors          map[string]string          `json:"colors"`
	ThemeColors     []resolveThemeColorEntry   `json:"theme_colors"`
	ColorRoles      *skillColorRoles           `json:"color_roles"`
	Fonts           resolveThemeFonts          `json:"fonts"`
	ResolvedFor     []string                   `json:"resolved_for,omitempty"`
	Unknown         []resolveThemeUnknownColor `json:"unknown,omitempty"`
	AppliedOverride *ThemeInput                `json:"applied_theme_override,omitempty"`
	Warnings        []string                   `json:"warnings,omitempty"`
}

// resolveThemeColorEntry mirrors svggen-mcp's StyleSpec.theme_colors entry shape
// ({name, rgb}) so an agent can copy this slice straight into render_diagram's
// `style.theme_colors` without pivoting the colors map by hand.
type resolveThemeColorEntry struct {
	Name string `json:"name"`
	RGB  string `json:"rgb"`
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
		return argMissing("resolve_theme", "template_name", "string", "midnight-blue", nextCallListTemplates()), nil
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

	// Apply optional theme_override before reading colors/fonts, so the response
	// mirrors what the singlepass generator would see for the same frontmatter.
	overrideInput, overrideErr := parseResolveThemeOverride(request)
	if overrideErr != nil {
		return argInvalidValue("resolve_theme", "INVALID_PARAMETER", "theme_override", overrideErr.Error(), "object", nil, nil), nil
	}
	var overrideWarnings []string
	if overrideInput != nil {
		theme, overrideWarnings = theme.ApplyOverride(overrideInput.ToThemeOverride())
	}

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
	// themeColors mirrors the colors map as the [{name,rgb}] array that
	// svggen-mcp's StyleSpec.theme_colors expects. Built in stable order:
	// theme-defined order when unfiltered, request order when filtered.
	themeColors := make([]resolveThemeColorEntry, 0, len(theme.Colors))

	if len(requestedNames) > 0 {
		colors = make(map[string]string, len(requestedNames))
		resolvedFor = requestedNames
		for _, name := range requestedNames {
			if hex, ok := allColors[name]; ok {
				colors[name] = hex
				themeColors = append(themeColors, resolveThemeColorEntry{Name: name, RGB: hex})
			} else {
				entry := resolveThemeUnknownColor{Name: name}
				if match, _ := generator.ClosestMatch(name, allColorNames, 3); match != "" {
					entry.DidYouMean = match
				}
				unknown = append(unknown, entry)
			}
		}
	} else {
		for _, c := range theme.Colors {
			themeColors = append(themeColors, resolveThemeColorEntry{Name: c.Name, RGB: c.RGB})
		}
	}

	// Strip template name from path.
	name := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")

	resp := resolveThemeResponse{
		Template:    name,
		Colors:      colors,
		ThemeColors: themeColors,
		ColorRoles:  buildColorRoles(theme.Colors),
		Fonts: resolveThemeFonts{
			Major: resolveThemeFontEntry{Latin: theme.TitleFont},
			Minor: resolveThemeFontEntry{Latin: theme.BodyFont},
		},
		ResolvedFor:     resolvedFor,
		Unknown:         unknown,
		AppliedOverride: overrideInput,
		Warnings:        overrideWarnings,
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
