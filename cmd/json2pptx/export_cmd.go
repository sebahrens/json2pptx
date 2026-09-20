package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func runExportDeck() error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	pptxPath := fs.String("pptx", "", "Path to the existing PPTX (required)")
	format := fs.String("format", "pdf", "Export format: pdf or notes")
	outputDir := fs.String("output-dir", "./output", "Directory for the retained export")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: json2pptx export --pptx <file.pptx> --format pdf|notes [--output-dir DIR]")
		printDoubleDashUsage(fs)
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *pptxPath == "" {
		fs.Usage()
		return fmt.Errorf("--pptx is required")
	}
	mc := cliMCPConfig("./templates", *outputDir)
	result, err := mc.handleExportDeck(context.Background(), mcpRequestWithArgs(map[string]any{
		"pptx_path": *pptxPath,
		"format":    *format,
	}))
	if err != nil {
		return fmt.Errorf("export deck: %w", err)
	}
	return printMCPResultJSON(result)
}
