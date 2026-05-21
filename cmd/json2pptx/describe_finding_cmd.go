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
		fmt.Fprintf(os.Stderr, "Usage: json2pptx describe-finding <code>\n")
		fmt.Fprintf(os.Stderr, "       json2pptx describe-finding -code <code>\n\n")
		fmt.Fprintf(os.Stderr, "Print the agent-facing description for a single finding code.\n")
		fmt.Fprintf(os.Stderr, "Accepts the bare legacy code or the dotted namespaced form from a\n")
		fmt.Fprintf(os.Stderr, "finding envelope (e.g. INPUT.MISSING_PARAMETER), so a finding's\n")
		fmt.Fprintf(os.Stderr, "describe_command runs verbatim.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx describe-finding placeholder_overflow\n")
		fmt.Fprintf(os.Stderr, "  json2pptx describe-finding MISSING_PARAMETER\n")
		fmt.Fprintf(os.Stderr, "  json2pptx describe-finding INPUT.MISSING_PARAMETER\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Accept the code as a positional argument too, so the describe_command
	// strings emitted on the wire ("json2pptx describe-finding <code>") run
	// verbatim. -code takes precedence when both are given.
	resolvedCode := *code
	if resolvedCode == "" && fs.NArg() > 0 {
		resolvedCode = fs.Arg(0)
	}

	if resolvedCode == "" {
		fs.Usage()
		return fmt.Errorf("describe-finding: a finding code is required (positional or -code)")
	}

	result, err := handleDescribeFinding(context.Background(), mcpRequestWithArgs(map[string]any{
		"code": resolvedCode,
	}))
	if err != nil {
		return fmt.Errorf("describe-finding: %w", err)
	}

	return printMCPResultJSON(result)
}
