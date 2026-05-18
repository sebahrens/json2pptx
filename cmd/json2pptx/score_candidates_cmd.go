package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// runScoreCandidates implements the "score-candidates" CLI subcommand.
// It outputs the same score response as the score_candidates MCP tool.
func runScoreCandidates() error {
	fs := flag.NewFlagSet("score-candidates", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonPath := fs.String("json", "", "Path to JSON input file containing the presentation deck (use - for stdin)")
	candidatesPath := fs.String("candidates", "", "Path to JSON file containing the candidates array (use - for stdin)")
	templateName := fs.String("template", "", "Template name override (uses presentation.template if omitted)")
	slideIndex := fs.Int("slide-index", -1, "0-based index of the slide slot to score candidates against (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx score-candidates -json <deck.json> -candidates <candidates.json> -slide-index N [options]\n\n")
		fmt.Fprintf(os.Stderr, "Score multiple candidate slide_json values for one slot in a deck without rendering.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx score-candidates -json deck.json -candidates cands.json -slide-index 2\n")
		fmt.Fprintf(os.Stderr, "  json2pptx score-candidates -json deck.json -candidates - -slide-index 0 < cands.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *jsonPath == "" {
		fs.Usage()
		return fmt.Errorf("-json is required")
	}
	if *candidatesPath == "" {
		fs.Usage()
		return fmt.Errorf("-candidates is required")
	}
	if *slideIndex < 0 {
		fs.Usage()
		return fmt.Errorf("-slide-index is required (must be >= 0)")
	}

	presentation, err := readJSONObject(*jsonPath)
	if err != nil {
		return err
	}

	candidatesRaw, err := readJSONInput(*candidatesPath)
	if err != nil {
		return err
	}
	var candidates []any
	if err := json.Unmarshal([]byte(candidatesRaw), &candidates); err != nil {
		return fmt.Errorf("candidates must be a JSON array: %w", err)
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"presentation": presentation,
		"slide_index":  float64(*slideIndex),
		"candidates":   candidates,
	}
	if *templateName != "" {
		args["template"] = *templateName
	}

	result, err := mc.handleScoreCandidates(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("score-candidates: %w", err)
	}

	return printMCPResultJSON(result)
}
