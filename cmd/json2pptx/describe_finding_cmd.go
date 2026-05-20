package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runDescribeFinding implements the "describe-finding" CLI subcommand.
// Mirrors the describe_finding MCP tool — given a code, prints the metadata
// payload as JSON so agents working through the CLI can hit the same
// dictionary as agents working through MCP.
func runDescribeFinding() error {
	fs := flag.NewFlagSet("describe-finding", flag.ContinueOnError)

	code := fs.String("code", "", "Finding code to describe (e.g. placeholder_overflow, accent_overload, chart.zero_sum_pie)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx describe-finding -code <code>\n\n")
		fmt.Fprintf(os.Stderr, "Print the agent-facing description for a single finding code.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *code == "" {
		fs.Usage()
		return fmt.Errorf("describe-finding: -code is required")
	}

	result, err := handleDescribeFinding(context.Background(), mcpRequestWithArgs(map[string]any{
		"code": *code,
	}))
	if err != nil {
		return fmt.Errorf("describe-finding: %w", err)
	}

	return printMCPResultJSON(result)
}
