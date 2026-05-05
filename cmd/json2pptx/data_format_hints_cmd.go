package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runDataFormatHints implements the "data-format-hints" CLI subcommand.
// It outputs the same dataFormatHintsResponse as the get_data_format_hints MCP tool.
func runDataFormatHints() error {
	fs := flag.NewFlagSet("data-format-hints", flag.ContinueOnError)

	digest := fs.String("digest", "", "Previous digest for cache check (returns not_modified if unchanged)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx data-format-hints [options]\n\n")
		fmt.Fprintf(os.Stderr, "Show data format hints for all chart and diagram types.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{}
	if *digest != "" {
		args["digest"] = *digest
	}

	result, err := handleGetDataFormatHints(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("data-format-hints: %w", err)
	}

	return printMCPResultJSON(result)
}
