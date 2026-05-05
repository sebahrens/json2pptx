package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runCapabilities implements the "capabilities" CLI subcommand.
// It outputs the same capabilitiesResponse as the get_capabilities MCP tool.
func runCapabilities() error {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx capabilities\n\n")
		fmt.Fprintf(os.Stderr, "Show schema version, available MCP tools, deprecated fields, and feature flags.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	result, err := handleGetCapabilities(context.Background(), mcpNoopRequest())
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}

	return printMCPResultJSON(result)
}
