package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// runValidateOutput implements the "validate-output" subcommand.
// It validates a generated PPTX file using the unified output-validation suite
// (structural OPC checks + OOXML content checks).
func runValidateOutput() error {
	fs := flag.NewFlagSet("validate-output", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output results as one JSON array (one entry per file, with an error field for unreadable files)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx validate-output [options] <file.pptx ...>\n\n")
		fmt.Fprintf(os.Stderr, "Validate PPTX files for structural and OOXML content correctness.\n")
		fmt.Fprintf(os.Stderr, "Checks package integrity, color values, shape ID uniqueness, table structure, etc.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output presentation.pptx\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output --json presentation.pptx\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("at least one PPTX file is required")
	}

	// --json emits one JSON array covering every file (including files that
	// could not be opened), so the output parses as a single document
	// (go-slide-creator-csclk.30).
	hasErrors := false
	var results []validateOutputResult
	for _, path := range fs.Args() {
		report, err := pptx.ValidateOutputFile(path)
		if err != nil {
			hasErrors = true
			if *jsonOut {
				results = append(results, validateOutputResult{FilePath: path, Error: err.Error()})
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s: %v\n", path, err)
			}
			continue
		}

		if *jsonOut {
			results = append(results, validateOutputResult{
				FilePath: path,
				IsValid:  report.IsValid(),
				Findings: report.Findings,
			})
		} else {
			printValidateOutputHuman(path, report)
		}

		if !report.IsValid() {
			hasErrors = true
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

type validateOutputResult struct {
	FilePath string         `json:"file_path"`
	IsValid  bool           `json:"is_valid"`
	Findings []pptx.Finding `json:"findings,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func printValidateOutputHuman(path string, report *pptx.Report) {
	if len(report.Findings) == 0 {
		fmt.Printf("✓ %s: output valid\n", path)
		return
	}

	blocking := report.Blocking()
	warnings := report.Warnings()

	if len(blocking) > 0 {
		fmt.Printf("✗ %s: %d blocking, %d warning finding(s)\n", path, len(blocking), len(warnings))
	} else {
		fmt.Printf("⚠ %s: %d warning finding(s)\n", path, len(warnings))
	}

	for _, f := range report.Findings {
		fmt.Printf("  %s\n", f.Error())
	}
}
