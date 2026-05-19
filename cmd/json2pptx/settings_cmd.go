package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// runTemplateSettings implements the "template-settings" CLI subcommand with
// sub-subcommands: list, register, delete.
func runTemplateSettings() error {
	if len(os.Args) < 2 {
		printTemplateSettingsUsage()
		return nil
	}

	subcmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "list":
		return runTemplateSettingsList()
	case "register":
		return runTemplateSettingsRegister()
	case "delete":
		return runTemplateSettingsDelete()
	case "help", "-h", "--help":
		printTemplateSettingsUsage()
		return nil
	default:
		printTemplateSettingsUsage()
		return fmt.Errorf("unknown template-settings subcommand %q", subcmd)
	}
}

func printTemplateSettingsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx template-settings <command> [options]

Commands:
  list      List named styles for a template
  register  Register a named table_style or cell_style
  delete    Delete a named style

Run 'json2pptx template-settings <command> -h' for command-specific help.
`)
}

func runTemplateSettingsList() error {
	fs := flag.NewFlagSet("template-settings list", flag.ContinueOnError)
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx template-settings list --template <name>\n\n")
		fmt.Fprintf(os.Stderr, "List named table_styles and cell_styles registered for a template.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" {
		fs.Usage()
		return fmt.Errorf("-template is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	result, err := mc.handleListTemplateSettings(context.Background(), mcpRequestWithArgs(map[string]any{
		"template_name": *templateName,
	}))
	if err != nil {
		return fmt.Errorf("template-settings list: %w", err)
	}
	return printMCPResultJSON(result)
}

func runTemplateSettingsRegister() error {
	fs := flag.NewFlagSet("template-settings register", flag.ContinueOnError)
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")
	kind := fs.String("kind", "", "Setting kind: table_styles or cell_styles (required)")
	name := fs.String("name", "", "Setting name (required)")
	definitionStr := fs.String("definition", "", "JSON definition object (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx template-settings register --template <name> --kind <kind> --name <name> --definition <json>\n\n")
		fmt.Fprintf(os.Stderr, "Register a named table_style or cell_style for a template.\n")
		fmt.Fprintf(os.Stderr, "Requires JSON2PPTX_ALLOW_SETTINGS_WRITE=1.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" || *kind == "" || *name == "" || *definitionStr == "" {
		fs.Usage()
		return fmt.Errorf("-template, -kind, -name, and -definition are all required")
	}

	// Parse definition JSON.
	var definition any
	if err := json.Unmarshal([]byte(*definitionStr), &definition); err != nil {
		return fmt.Errorf("invalid -definition JSON: %w", err)
	}

	mc := cliMCPConfig(*templatesDir, "")

	result, err := mc.handleRegisterTemplateSetting(context.Background(), mcpRequestWithArgs(map[string]any{
		"template_name": *templateName,
		"kind":          *kind,
		"name":          *name,
		"definition":    definition,
	}))
	if err != nil {
		return fmt.Errorf("template-settings register: %w", err)
	}
	return printMCPResultJSON(result)
}

func runTemplateSettingsDelete() error {
	fs := flag.NewFlagSet("template-settings delete", flag.ContinueOnError)
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name (required)")
	kind := fs.String("kind", "", "Setting kind: table_styles or cell_styles (required)")
	name := fs.String("name", "", "Setting name to delete (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx template-settings delete --template <name> --kind <kind> --name <name>\n\n")
		fmt.Fprintf(os.Stderr, "Delete a named style from a template's settings.\n")
		fmt.Fprintf(os.Stderr, "Requires JSON2PPTX_ALLOW_SETTINGS_WRITE=1.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *templateName == "" || *kind == "" || *name == "" {
		fs.Usage()
		return fmt.Errorf("-template, -kind, and -name are all required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	result, err := mc.handleDeleteTemplateSetting(context.Background(), mcpRequestWithArgs(map[string]any{
		"template_name": *templateName,
		"kind":          *kind,
		"name":          *name,
	}))
	if err != nil {
		return fmt.Errorf("template-settings delete: %w", err)
	}
	return printMCPResultJSON(result)
}
