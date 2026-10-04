package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
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

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{}
	if *category != "" {
		// The MCP tool's schema refuses an unknown category; called directly
		// the handler returned an empty catalogue and the command exited 0.
		names := make([]string, len(shapeCategories))
		known := false
		for i, cat := range shapeCategories {
			names[i] = cat.name
			known = known || cat.name == *category
		}
		if !known {
			return cliInvalidArg("unknown --category %q: must be one of %s", *category, strings.Join(names, ", "))
		}
		args["category"] = *category
	}

	result, err := handleGetShapeCatalog(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("shape-catalog: %w", err)
	}

	return printMCPResultJSON(result)
}
