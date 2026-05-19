package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runRecommendVisual implements the "recommend-visual" CLI subcommand.
// It outputs the same response as the recommend_visual MCP tool.
func runRecommendVisual() error {
	fs := flag.NewFlagSet("recommend-visual", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	intent := fs.String("intent", "", "Natural language description of what to show (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx recommend-visual --intent <description> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Recommend visual approaches (patterns, charts, diagrams, layouts) for a slide.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-visual --intent \"show Q3 revenue trend\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-visual --intent \"compare 3 vendors on 5 dimensions\"\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *intent == "" {
		fs.Usage()
		return fmt.Errorf("--intent is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"intent": *intent,
	}

	result, err := mc.handleRecommendVisual(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("recommend-visual: %w", err)
	}

	return printMCPResultJSON(result)
}
