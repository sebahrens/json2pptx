package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// builtinThemeVariations is the registry of named theme variations the CLI's
// -variation flag accepts. Each entry is a theme_override value forwarded
// verbatim to the resolve_theme MCP handler. The map is intentionally empty
// at present: it is the extension point for future built-in presets such as
// "dark" or "high-contrast". Adding an entry here is the only step required
// to make -variation <name> work — the CLI plumbing handles the rest.
var builtinThemeVariations = map[string]map[string]any{}

// runResolveTheme implements the "resolve-theme" CLI subcommand.
// It outputs the same resolveThemeResponse as the resolve_theme MCP tool.
func runResolveTheme() error {
	fs := flag.NewFlagSet("resolve-theme", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")
	colorNames := fs.String("colors", "", "Comma-separated list of color names to resolve (omit for all)")
	override := fs.String("override", "", "Theme override as inline JSON or @path/to/file.json (same shape as frontmatter theme_override: {colors, title_font, body_font})")
	variation := fs.String("variation", "", "Named theme variation preset (reserved for future built-in variants like 'dark', 'high-contrast'); use --override for custom JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx resolve-theme --template <name> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Resolve theme colors and fonts for a template.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme --template midnight-blue\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme --template midnight-blue --colors accent1,accent2,dk1\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme --template midnight-blue --override '{\"colors\":{\"accent1\":\"#336699\"}}'\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme --template midnight-blue --override @theme.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" {
		fs.Usage()
		return fmt.Errorf("--template is required")
	}

	if *override != "" && *variation != "" {
		return fmt.Errorf("--override and --variation are mutually exclusive; pass only one")
	}

	themeOverride, err := buildThemeOverrideArg(*override, *variation)
	if err != nil {
		return fmt.Errorf("resolve-theme: %w", err)
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"template_name": *templateName,
	}
	if *colorNames != "" {
		args["color_names"] = *colorNames
	}
	if themeOverride != nil {
		args["theme_override"] = themeOverride
	}

	result, err := mc.handleResolveTheme(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("resolve-theme: %w", err)
	}

	return printMCPResultJSON(result)
}

// buildThemeOverrideArg returns the theme_override object the resolve_theme
// MCP handler expects, derived from either -override JSON or a -variation
// name. Returns (nil, nil) when neither flag is set so the MCP request omits
// the parameter entirely.
func buildThemeOverrideArg(overrideArg, variationArg string) (map[string]any, error) {
	switch {
	case overrideArg != "":
		return parseOverrideJSONArg(overrideArg)
	case variationArg != "":
		preset, ok := builtinThemeVariations[variationArg]
		if !ok {
			return nil, fmt.Errorf("--variation: unknown variation %q (known: %s); use --override for custom JSON",
				variationArg, formatKnownVariations())
		}
		// Return a shallow copy so callers cannot mutate the registry entry.
		out := make(map[string]any, len(preset))
		for k, v := range preset {
			out[k] = v
		}
		return out, nil
	default:
		return nil, nil
	}
}

// parseOverrideJSONArg accepts either inline JSON or @path/to/file.json and
// returns the parsed object. The result must be a JSON object so it can be
// forwarded as the resolve_theme tool's `theme_override` parameter.
func parseOverrideJSONArg(arg string) (map[string]any, error) {
	var raw string
	if strings.HasPrefix(arg, "@") {
		path := strings.TrimPrefix(arg, "@")
		if path == "" {
			return nil, fmt.Errorf("-override: empty path after '@'")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("-override: failed to read %s: %w", path, err)
		}
		raw = string(data)
	} else {
		raw = arg
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("-override: invalid JSON: %w", err)
	}
	return obj, nil
}

// formatKnownVariations renders the registry's keys as a stable, comma-separated
// list for error messages.
func formatKnownVariations() string {
	if len(builtinThemeVariations) == 0 {
		return "none yet"
	}
	names := make([]string, 0, len(builtinThemeVariations))
	for name := range builtinThemeVariations {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
