package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runCapabilities implements the "capabilities" CLI subcommand.
// It outputs the same capabilitiesResponse as the get_capabilities MCP tool.
//
// The --templates-dir / --output flags default to the same values runMCP uses
// (./templates, ./output) so the runtime block reports the resolved directories
// the engine would operate on, matching the MCP get_capabilities response
// instead of leaving them blank.
func runCapabilities() error {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates (resolved and reported in the runtime block)")
	outputDir := fs.String("output", "./output", "Output directory reported in the runtime block")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx capabilities [options]\n\n")
		fmt.Fprintf(os.Stderr, "Show schema version, available MCP tools, deprecated fields, and feature flags.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	result, err := buildCapabilitiesResult(context.Background(), *templatesDir, *outputDir)
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}

	return printMCPResultJSON(result)
}
