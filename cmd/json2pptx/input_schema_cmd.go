package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runInputSchema implements the "input-schema" CLI subcommand.
// It outputs the same schema as the get_input_schema MCP tool.
func runInputSchema() error {
	fs := flag.NewFlagSet("input-schema", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx input-schema\n\n")
		fmt.Fprintf(os.Stderr, "Print the authoritative JSON Schema for PresentationInput.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	result, err := handleGetInputSchema(context.Background(), mcpNoopRequest())
	if err != nil {
		return fmt.Errorf("input-schema: %w", err)
	}

	return printMCPResultJSON(result)
}
