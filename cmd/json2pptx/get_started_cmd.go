package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runGetStarted implements the "get-started" CLI subcommand.
// It outputs the same getStartedResponse as the get_started MCP tool.
func runGetStarted() error {
	fs := flag.NewFlagSet("get-started", flag.ContinueOnError)

	task := fs.String("task", "", "Task scope: brief (new deck, default), revise (modify existing deck), validate-only (validate JSON without generating)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx get-started [options]\n\n")
		fmt.Fprintf(os.Stderr, "Print the recommended ordered MCP-call sequence for a stated task.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{}
	if *task != "" {
		args["task"] = *task
	}

	result, err := handleGetStarted(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("get-started: %w", err)
	}

	return printMCPResultJSON(result)
}
