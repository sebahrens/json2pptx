package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runScore implements the "score" CLI subcommand.
// It outputs the same score response as the score_deck MCP tool.
func runScore() error {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonPath := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	templateName := fs.String("template", "", "Template name override (uses presentation.template if omitted)")
	mode := fs.String("mode", "deterministic", "Scoring mode. Only 'deterministic' is implemented; 'with_heuristics' is reserved and currently rejected with UNSUPPORTED_MODE — use the 'inspect' subcommand on rendered thumbnails for vision-based visual QA.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx score --json <file.json> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Score a JSON deck spec for visual quality using deterministic rules.\n")
		fmt.Fprintf(os.Stderr, "Note: this scores the JSON input spec, not a rendered .pptx file.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx score --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx score --json slides.json --template midnight-blue\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *jsonPath == "" {
		fs.Usage()
		return fmt.Errorf("--json is required")
	}

	presentation, err := readJSONObject(*jsonPath)
	if err != nil {
		return err
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"presentation": presentation,
		"mode":         *mode,
	}
	if *templateName != "" {
		args["template"] = *templateName
	}

	result, err := mc.handleScoreDeck(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("score: %w", err)
	}

	return printMCPResultJSON(result)
}
