package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runPlanDeck implements the "plan-deck" CLI subcommand.
// It outputs the same response as the plan_deck MCP tool.
func runPlanDeck() error {
	fs := flag.NewFlagSet("plan-deck", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	brief := fs.String("brief", "", "Deck brief / topic description (required)")
	slideBudget := fs.Int("slides", 0, "Target number of slides (optional; 0 = the count the brief states, else 10)")
	audience := fs.String("audience", "", "Target audience (optional), e.g. \"board of directors\"")
	tmpl := fs.String("template", "", "Optional template name to make the plan template-aware (adds per-slide template_support)")
	format := fs.String("format", "raw", "raw (pattern outline with SlideInput skeletons) or deckspec (DeckSpec draft with narrative slots)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx plan-deck --brief <description> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Plan a presentation deck from a brief.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx plan-deck --brief \"Q3 revenue results for the board\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx plan-deck --brief \"Product roadmap update\" --slides 8\n")
		fmt.Fprintf(os.Stderr, "  json2pptx plan-deck --brief \"Investor pitch\" --template midnight-blue\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if *brief == "" {
		fs.Usage()
		return cliMissingArg("--brief is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"brief": *brief,
	}
	if *slideBudget > 0 {
		args["slide_budget"] = float64(*slideBudget)
	}
	if *tmpl != "" {
		args["template"] = *tmpl
	}
	if *audience != "" {
		args["audience"] = *audience
	}
	if *format != "" && *format != "raw" {
		args["format"] = *format
	}

	result, err := mc.handlePlanDeck(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("plan-deck: %w", err)
	}

	return printMCPResultJSON(result)
}
