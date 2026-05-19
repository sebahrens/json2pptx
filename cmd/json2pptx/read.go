package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/pptxread"
)

// runRead implements the "read" subcommand.
// It reads a PPTX file and outputs the extracted content as JSON.
func runRead() error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	slideIndex := fs.Int("slide", -1, "Extract a single slide by 0-based index (default: all slides)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx read [options] <file.pptx>\n\n")
		fmt.Fprintf(os.Stderr, "Read a PPTX file and output extracted content as JSON.\n\n")
		fmt.Fprintf(os.Stderr, "This is a best-effort extraction: it returns placeholders, shapes,\n")
		fmt.Fprintf(os.Stderr, "tables, and speaker notes found in the file. It does not require\n")
		fmt.Fprintf(os.Stderr, "LibreOffice or any external dependencies.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("pptx file path is required")
	}

	pptxPath := fs.Arg(0)

	pres, err := pptxread.ReadFile(pptxPath)
	if err != nil {
		return fmt.Errorf("read presentation: %w", err)
	}

	// Filter to a single slide if requested.
	if *slideIndex >= 0 {
		if *slideIndex >= len(pres.Slides) {
			return fmt.Errorf("slide index %d out of range (presentation has %d slides)", *slideIndex, len(pres.Slides))
		}
		pres.Slides = []pptxread.Slide{pres.Slides[*slideIndex]}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(pres)
}
