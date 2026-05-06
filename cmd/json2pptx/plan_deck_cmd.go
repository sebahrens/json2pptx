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

	brief := fs.String("brief", "", "Deck brief / topic description (required)")
	slideBudget := fs.Int("slides", 0, "Target number of slides (optional, 0 = auto)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx plan-deck -brief <description> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Plan a presentation deck from a brief.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx plan-deck -brief \"Q3 revenue results for the board\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx plan-deck -brief \"Product roadmap update\" -slides 8\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *brief == "" {
		fs.Usage()
		return fmt.Errorf("-brief is required")
	}

	args := map[string]any{
		"brief": *brief,
	}
	if *slideBudget > 0 {
		args["slide_budget"] = float64(*slideBudget)
	}

	result, err := handlePlanDeck(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("plan-deck: %w", err)
	}

	return printMCPResultJSON(result)
}
