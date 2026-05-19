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
	jsonOut := fs.Bool("json", false, "Output results as JSON")

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

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("at least one PPTX file is required")
	}

	hasErrors := false
	for _, path := range fs.Args() {
		report, err := pptx.ValidateOutputFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", path, err)
			hasErrors = true
			continue
		}

		if *jsonOut {
			printValidateOutputJSON(path, report)
		} else {
			printValidateOutputHuman(path, report)
		}

		if !report.IsValid() {
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

type validateOutputResult struct {
	FilePath string               `json:"file_path"`
	IsValid  bool                 `json:"is_valid"`
	Findings []pptx.Finding       `json:"findings,omitempty"`
}

func printValidateOutputJSON(path string, report *pptx.Report) {
	result := validateOutputResult{
		FilePath: path,
		IsValid:  report.IsValid(),
		Findings: report.Findings,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
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
