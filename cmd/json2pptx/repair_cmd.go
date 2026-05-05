package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// runRepair implements the "repair" CLI subcommand.
// It outputs the same repairSlideOutput as the repair_slide MCP tool.
func runRepair() error {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonPath := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	slideIndex := fs.Int("slide-index", 0, "0-based index of the slide to repair")
	fixesStr := fs.String("fixes", "", `JSON array of fix directives, e.g. '[{"kind":"reduce_text","params":{"max_items":5}}]'`)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx repair -json <file.json> -fixes <json-array> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Apply targeted fixes to a single slide without regenerating the entire deck.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx repair -json deck.json -slide-index 2 -fixes '[{\"kind\":\"reduce_text\",\"params\":{\"max_items\":5}}]'\n")
		fmt.Fprintf(os.Stderr, "  json2pptx repair -json deck.json -slide-index 0 -fixes '[{\"kind\":\"swap_layout\",\"params\":{\"layout_id\":\"slideLayout3\"}}]'\n\n")
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
	if *fixesStr == "" {
		fs.Usage()
		return fmt.Errorf("-fixes is required")
	}

	jsonInput, err := readJSONInput(*jsonPath)
	if err != nil {
		return err
	}

	// Parse fixes to validate JSON before sending.
	var fixes []any
	if err := json.Unmarshal([]byte(*fixesStr), &fixes); err != nil {
		return fmt.Errorf("invalid -fixes JSON: %w", err)
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"json_input":  jsonInput,
		"slide_index": float64(*slideIndex),
		"fixes":       fixes,
	}

	result, err := mc.handleRepairSlide(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("repair: %w", err)
	}

	return printMCPResultJSON(result)
}
