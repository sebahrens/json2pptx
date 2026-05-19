package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runShapeCatalog implements the "shape-catalog" CLI subcommand.
// It outputs the same response as the get_shape_catalog MCP tool.
func runShapeCatalog() error {
	fs := flag.NewFlagSet("shape-catalog", flag.ContinueOnError)

	category := fs.String("category", "", "Filter by category name (omit for all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx shape-catalog [options]\n\n")
		fmt.Fprintf(os.Stderr, "List available preset geometries for shapes.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{}
	if *category != "" {
		args["category"] = *category
	}

	result, err := handleGetShapeCatalog(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("shape-catalog: %w", err)
	}

	return printMCPResultJSON(result)
}
