package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runResolveTheme implements the "resolve-theme" CLI subcommand.
// It outputs the same resolveThemeResponse as the resolve_theme MCP tool.
func runResolveTheme() error {
	fs := flag.NewFlagSet("resolve-theme", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")
	colorNames := fs.String("colors", "", "Comma-separated list of color names to resolve (omit for all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx resolve-theme -template <name> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Resolve theme colors and fonts for a template.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme -template midnight-blue\n")
		fmt.Fprintf(os.Stderr, "  json2pptx resolve-theme -template midnight-blue -colors accent1,accent2,dk1\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" {
		fs.Usage()
		return fmt.Errorf("-template is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"template_name": *templateName,
	}
	if *colorNames != "" {
		args["color_names"] = *colorNames
	}

	result, err := mc.handleResolveTheme(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("resolve-theme: %w", err)
	}

	return printMCPResultJSON(result)
}
