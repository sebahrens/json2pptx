package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runAnalyzeRhythm implements the "analyze-rhythm" CLI subcommand.
func runAnalyzeRhythm() error {
	fs := flag.NewFlagSet("analyze-rhythm", flag.ContinueOnError)

	jsonPath := fs.String("json", "", "Path to JSON input file (use - for stdin)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx analyze-rhythm --json <file.json>\n\n")
		fmt.Fprintf(os.Stderr, "Analyze deck visual rhythm — pattern runs, density variation, accent balance.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx analyze-rhythm --json slides.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *jsonPath == "" {
		fs.Usage()
		return fmt.Errorf("-json is required")
	}

	presentation, err := readJSONObject(*jsonPath)
	if err != nil {
		return err
	}

	args := map[string]any{
		"presentation": presentation,
	}

	result, err := handleAnalyzeDeckRhythm(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("analyze-rhythm: %w", err)
	}

	return printMCPResultJSON(result)
}
