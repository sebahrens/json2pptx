package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// runValidateOutput implements the "validate-output" subcommand.
// It validates a generated PPTX file using the unified output-validation suite
// (structural OPC checks + OOXML content checks).
func runValidateOutput() error {
	fs := flag.NewFlagSet("validate-output", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output the per-file results alone, as one JSON array (one entry per file, with an error field for unreadable files); --format json wraps the same array as files[] in the shared finding envelope")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx validate-output [options] <file.pptx ...>\n\n")
		fmt.Fprintf(os.Stderr, "Validate PPTX files for structural and OOXML content correctness.\n")
		fmt.Fprintf(os.Stderr, "Checks package integrity, color values, shape ID uniqueness, table structure, etc.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output presentation.pptx\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate-output --format json presentation.pptx\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return cliMissingArg("at least one PPTX file is required")
	}

	// --json emits one JSON array covering every file (including files that
	// could not be opened), so the output parses as a single document
	// (go-slide-creator-csclk.30).
	hasErrors := false
	results := []validateOutputResult{}
	openErrs := map[string]error{}
	for _, path := range fs.Args() {
		report, err := pptx.ValidateOutputFile(path)
		if err != nil {
			hasErrors = true
			if *jsonOut {
				openErrs[path] = err
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
		enc.SetEscapeHTML(false)
		if validateOutputLegacyArray(fs) {
			_ = enc.Encode(results)
		} else {
			_ = enc.Encode(newValidateOutputEnvelope(results, openErrs))
		}
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

// validateOutputLegacyArray reports whether the caller asked for JSON with the
// deprecated --json flag alone. That form keeps the bare array it always
// printed, so a script reading `.[0].is_valid` still works; --format json is
// the shared finding envelope with the same array under files[]
// (go-slide-creator-u1c9c).
func validateOutputLegacyArray(fs *flag.FlagSet) bool {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	return given["json"] && !given["format"]
}

// validateOutputEnvelope is validate-output's JSON answer: the shared finding
// envelope every other command answers with, and the per-file results beside
// it.
type validateOutputEnvelope struct {
	diagnostics.FindingEnvelope
	Files []validateOutputResult `json:"files"`
}

// newValidateOutputEnvelope builds the envelope from the per-file results: one
// finding per output-validation finding (a blocking one is an error), and one
// for each file that could not be read, each naming its file.
func newValidateOutputEnvelope(results []validateOutputResult, openErrs map[string]error) validateOutputEnvelope {
	ds := []diagnostics.Diagnostic{}
	for _, r := range results {
		if err := openErrs[r.FilePath]; err != nil {
			code := cliErrorCode(err)
			if code == diagnostics.CodeInternal {
				code = diagnostics.CodeValidationFailed
			}
			ds = append(ds, diagnostics.Diagnostic{
				Code: string(code), Message: fmt.Sprintf("%s: %v", r.FilePath, err), Severity: diagnostics.SeverityError,
				Details: map[string]any{"file_path": r.FilePath},
			})
			continue
		}
		for _, f := range r.Findings {
			severity := diagnostics.SeverityWarning
			if f.Severity == pptx.SeverityBlocking {
				severity = diagnostics.SeverityError
			}
			details := map[string]any{"file_path": r.FilePath, "phase": f.Phase, "validator": f.Validator, "scope": f.Scope}
			if f.Path != "" {
				details["part"] = f.Path
			}
			if f.SlideIndex >= 0 {
				details["slide_index"] = f.SlideIndex
			}
			ds = append(ds, diagnostics.Diagnostic{
				Code: f.Code, Message: f.Message, Path: f.SourcePath, Severity: severity, Details: details,
			})
		}
	}
	return validateOutputEnvelope{
		FindingEnvelope: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "validate-output"}, ds),
		Files:           results,
	}
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
