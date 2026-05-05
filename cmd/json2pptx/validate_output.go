package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// runValidateOutput implements the "validate-output" subcommand.
// It validates a generated PPTX file for OOXML content correctness.
func runValidateOutput() error {
	fs := flag.NewFlagSet("validate-output", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx validate-output [options] <file.pptx ...>\n\n")
		fmt.Fprintf(os.Stderr, "Validate PPTX files for OOXML content correctness.\n")
		fmt.Fprintf(os.Stderr, "Checks color values, shape ID uniqueness, table structure, etc.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output presentation.pptx\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output -json presentation.pptx\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
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
		errs, err := generator.ValidateOutputFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", path, err)
			hasErrors = true
			continue
		}

		if *jsonOut {
			printValidateOutputJSON(path, errs)
		} else {
			printValidateOutputHuman(path, errs)
		}

		if len(errs) > 0 {
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

type validateOutputResult struct {
	FilePath string                 `json:"file_path"`
	IsValid  bool                   `json:"is_valid"`
	Errors   []validateOutputError  `json:"errors,omitempty"`
}

type validateOutputError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func printValidateOutputJSON(path string, errs []pptx.ValidationError) {
	result := validateOutputResult{
		FilePath: path,
		IsValid:  len(errs) == 0,
	}
	for _, e := range errs {
		result.Errors = append(result.Errors, validateOutputError{
			Path:    e.Path,
			Code:    e.Code,
			Message: e.Message,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

func printValidateOutputHuman(path string, errs []pptx.ValidationError) {
	if len(errs) == 0 {
		fmt.Printf("✓ %s: OOXML valid\n", path)
		return
	}
	fmt.Printf("✗ %s: %d OOXML violation(s)\n", path, len(errs))
	for _, e := range errs {
		fmt.Printf("  [%s] %s: %s\n", e.Code, e.Path, e.Message)
	}
}
