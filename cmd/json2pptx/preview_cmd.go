package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// runPreview implements the "preview" CLI subcommand.
// It outputs the same previewPlanOutput as the preview_presentation_plan MCP tool.
func runPreview() error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonPath := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	fitReport := fs.Bool("fit-report", true, "Include fit findings (default: true)")
	verboseFit := fs.Bool("verbose-fit", false, "Return all fit findings without budget limit")
	strictUnknownKeys := fs.Bool("strict-unknown-keys", false, "Fail-fast on misspelled/unknown JSON keys: when true, unknown keys are errors that block preview; when false (default), they are warnings. Mirrors MCP preview_presentation_plan strict_unknown_keys.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preview --json <file.json> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Preview the full generation plan without rendering a PPTX.\n")
		fmt.Fprintf(os.Stderr, "Shows per-slide layout selection, placeholder mapping, and fit findings.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview --json slides.json --fit-report=false\n")
		fmt.Fprintf(os.Stderr, "  cat slides.json | json2pptx preview --json -\n")
		fmt.Fprintf(os.Stderr, "  json2pptx preview --strict-unknown-keys --json slides.json   # fail-fast on typo'd fields\n\n")
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

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"presentation":        presentation,
		"fit_report":          *fitReport,
		"verbose_fit":         *verboseFit,
		"strict_unknown_keys": *strictUnknownKeys,
	}

	result, err := mc.handlePreviewPlan(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("preview: %w", err)
	}

	return printMCPResultJSON(result)
}
