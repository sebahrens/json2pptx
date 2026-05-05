package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runRecommendPattern implements the "recommend-pattern" CLI subcommand.
// It outputs the same response as the recommend_pattern MCP tool.
func runRecommendPattern() error {
	fs := flag.NewFlagSet("recommend-pattern", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	intent := fs.String("intent", "", "Natural language description of what you want to show (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx recommend-pattern -intent <description> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Recommend named patterns that match a given intent.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern -intent \"show 3 KPIs\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern -intent \"compare two options side by side\"\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *intent == "" {
		fs.Usage()
		return fmt.Errorf("-intent is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"intent": *intent,
	}

	result, err := mc.handleRecommendPattern(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("recommend-pattern: %w", err)
	}

	return printMCPResultJSON(result)
}
