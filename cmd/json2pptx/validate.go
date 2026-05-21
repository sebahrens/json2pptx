package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// runValidate implements the "validate" subcommand. It validates JSON slide
// input against the template without generating PPTX output. Both the human
// path and the structured-JSON path delegate to mc.handleValidate so the CLI
// and MCP validate_input tool emit identical findings, codes, and remediations.
func runValidate() error { //nolint:gocognit
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	templateName := fs.String("template", "", "Template name for layout validation (optional)")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonOut := fs.Bool("json", false, "Alias for --format=json: emit the MCP validate_input dryRunOutput shape to stdout")
	jsonOutputPath := fs.String("json-output", "", "Write JSON results (dryRunOutput shape) to file (use - for stdout)")
	fitReport := fs.Bool("fit-report", false, "Run per-cell text overflow measurement and print findings")
	verboseFit := fs.Bool("verbose-fit", false, "Return all fit findings without the per-slide budget limit")
	format := fs.String("format", "", "Output format: json (MCP-identical dryRunOutput), ndjson, or human (default)")
	strictUnknownKeys := fs.Bool("strict-unknown-keys", false, "Fail-fast on misspelled/unknown JSON keys: when true, unknown keys are validation errors; when false (default), they are warnings. Mirrors MCP validate_input strict_unknown_keys.")
	_ = fs.Bool("partial", false, "Accepted for CLI compatibility (validation always reports per-slide diagnostics)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx validate [options] <file.json ...>\n\n")
		fmt.Fprintf(os.Stderr, "Validate JSON slide files without generating PPTX output.\n")
		fmt.Fprintf(os.Stderr, "Reports errors, warnings, and content statistics.\n\n")
		fmt.Fprintf(os.Stderr, "Output shapes:\n")
		fmt.Fprintf(os.Stderr, "  - human (default): human-readable summary on stdout, fit findings on stderr\n")
		fmt.Fprintf(os.Stderr, "  - --json / --format=json / --json-output: MCP validate_input dryRunOutput shape\n")
		fmt.Fprintf(os.Stderr, "  - --format=ndjson: MCP dryRunOutput, one object per line per file\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --json-output results.json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --json-output - slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --template corporate slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --fit-report slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --fit-report --verbose-fit slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --format=json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate slides.json chapter2.json chapter3.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate --strict-unknown-keys slides.json   # fail-fast on typo'd fields\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("at least one input file is required")
	}

	// Determine effective output format. There is exactly one structured JSON
	// shape: the MCP validate_input dryRunOutput. --json and --json-output are
	// aliases for --format=json (with --json-output also choosing a destination).
	// Human mode is the only non-JSON shape; it never emits NDJSON on stdout.
	effectiveFormat := *format
	effectiveJSONPath := *jsonOutputPath
	if effectiveFormat == "" && (*jsonOut || effectiveJSONPath != "") {
		effectiveFormat = "json"
	}
	if effectiveFormat == "json" && effectiveJSONPath == "" {
		effectiveJSONPath = "-"
	}

	if effectiveFormat == "json" || effectiveFormat == "ndjson" {
		return runValidateMCPFormat(args, *templatesDir, *fitReport, *verboseFit, effectiveFormat, *strictUnknownKeys, effectiveJSONPath)
	}

	// Suppress unused warnings for flags consumed below.
	_ = templateName

	hasErrors := false
	var results []validateResult

	for _, filePath := range args {
		result := validateJSONFile(filePath, *templatesDir, *strictUnknownKeys)
		results = append(results, result)
		if !result.Valid {
			hasErrors = true
		}
	}

	// Fit-report: walk all tables and shape-grid text cells for overflow.
	// In human mode we only emit human-readable findings to stderr — NDJSON on
	// stdout would mix shapes and break agents parsing the human output.
	if *fitReport {
		for _, filePath := range args {
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			var input PresentationInput
			var patchInput PresentationPatchInput
			if json.Unmarshal(content, &patchInput) == nil && len(patchInput.Operations) > 0 {
				patched, patchErr := applyPresentationPatch(patchInput)
				if patchErr != nil {
					continue
				}
				input = *patched
			} else if json.Unmarshal(content, &input) != nil {
				continue
			}
			applyDefaults(&input)

			findings := generateFitReport(&input)
			findings = budgetLocalFindings(findings, DefaultFindingBudget, *verboseFit)
			printFitFindingsBySlide(findings)
			for _, f := range findings {
				if f.Action == "refuse" {
					hasErrors = true
				}
			}
		}
	}

	// Human-readable output to stdout.
	for _, r := range results {
		printValidateResult(r)
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// validateResult holds the structured validation output for a single file.
type validateResult struct {
	Valid        bool                     `json:"valid"`
	File         string                   `json:"file"`
	SlideCount   int                      `json:"slide_count"`
	ChartCount   int                      `json:"chart_count"`
	DiagramCount int                      `json:"diagram_count"`
	TableCount   int                      `json:"table_count"`
	ShapeCount   int                      `json:"shape_count"`
	Errors       []string                 `json:"errors"`
	Warnings     []string                 `json:"warnings"`
	Diagnostics  []diagnostics.Diagnostic `json:"diagnostics,omitempty"`
}

// validateJSONFile validates a single JSON input file by delegating to the
// shared MCP validate_input handler. CLI human mode and MCP agents therefore
// see identical findings, codes, and remediations; the only difference is
// presentation (printValidateResult formats this shape for the terminal).
//
// When strictUnknownKeys is true, unknown JSON keys are reported as errors
// (matching MCP validate_input strict_unknown_keys semantics) instead of
// warnings. Patch input is resolved to a presentation before delegating so
// the agent path always sees a normal PresentationInput.
//
// fit_report is intentionally disabled on this MCP call: the CLI surfaces fit
// findings on a separate stderr path (gated by --fit-report). Promoting them
// into Warnings/Errors here would duplicate output.
func validateJSONFile(filePath, templatesDir string, strictUnknownKeys bool) validateResult {
	result := validateResult{
		File:     filePath,
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Valid = false
		msg := fmt.Sprintf("failed to read file: %v", err)
		result.Errors = append(result.Errors, msg)
		result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
			Code: "FILE_READ_ERROR", Message: msg, Severity: diagnostics.SeverityError,
		})
		return result
	}

	// Resolve patch input to a presentation up front so the MCP handler always
	// validates the effective deck, not the patch envelope.
	presentation, parseErr := parseValidateInputAsAny(content)
	if parseErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, parseErr.message)
		result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
			Code: parseErr.code, Message: parseErr.message, Severity: diagnostics.SeverityError,
		})
		return result
	}

	mc := cliMCPConfig(templatesDir, "")
	mcpResult, err := mc.handleValidate(context.Background(), mcpRequestWithArgs(
		mcpHumanValidateArgs(presentation, strictUnknownKeys),
	))
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("validate %s: %v", filePath, err))
		return result
	}

	mergeValidateResultFromMCP(&result, mcpResult)
	return result
}

// mcpHumanValidateArgs is the canonical request shape sent into
// mc.handleValidate from CLI human mode. Keeping the construction in one
// place keeps the CLI and MCP paths byte-identical aside from the fields the
// human path doesn't drive.
//
// fit_report is disabled because the CLI surfaces fit findings on a separate
// stderr path gated by --fit-report; promoting them into Warnings/Errors here
// would duplicate output.
func mcpHumanValidateArgs(presentation any, strictUnknownKeys bool) map[string]any {
	return map[string]any{
		"presentation":        presentation,
		"strict_unknown_keys": strictUnknownKeys,
		"fit_report":          false,
	}
}

// validateInputParseError captures the structured failure shape from
// parseValidateInputAsAny so the caller can build a Diagnostic without losing
// the code/message distinction (patch failures vs. invalid JSON).
type validateInputParseError struct {
	code    string
	message string
}

// parseValidateInputAsAny reads CLI JSON input and returns it as an untyped
// JSON value suitable for passing to mc.handleValidate. Patch envelopes are
// applied first so MCP always sees a PresentationInput shape.
func parseValidateInputAsAny(content []byte) (any, *validateInputParseError) {
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(content, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			return nil, &validateInputParseError{
				code:    "PATCH_ERROR",
				message: fmt.Sprintf("failed to apply patch: %v", patchErr),
			}
		}
		patchedJSON, mErr := json.Marshal(patched)
		if mErr != nil {
			return nil, &validateInputParseError{
				code:    "INVALID_JSON",
				message: fmt.Sprintf("failed to marshal patched presentation: %v", mErr),
			}
		}
		var presentation any
		if err := json.Unmarshal(patchedJSON, &presentation); err != nil {
			return nil, &validateInputParseError{
				code:    "INVALID_JSON",
				message: fmt.Sprintf("invalid JSON after patch: %v", err),
			}
		}
		return presentation, nil
	}

	var presentation any
	if err := json.Unmarshal(content, &presentation); err != nil {
		return nil, &validateInputParseError{
			code:    "INVALID_JSON",
			message: fmt.Sprintf("failed to parse JSON: %v", err),
		}
	}
	return presentation, nil
}

// mergeValidateResultFromMCP populates result from an MCP validate
// CallToolResult. Diagnostics are the source of truth for severity: error
// diagnostics become Errors, warnings become Warnings, info is dropped from
// the human string slices (still present in Diagnostics).
//
// Success path: the result text is a dryRunOutput JSON object. Counts and
// per-slide details ride straight through; warning/error findings are read from
// the embedded Findings envelope.
//
// Error path: the result text is an mcpErrorEnvelope ({diagnostics, summary}).
// Counts are not available; the caller's defaults (zeroed ints, Valid=false)
// remain.
func mergeValidateResultFromMCP(result *validateResult, mcpResult *mcpgo.CallToolResult) {
	if mcpResult == nil || len(mcpResult.Content) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "empty MCP validate response")
		return
	}
	tc, ok := mcpResult.Content[0].(mcpgo.TextContent)
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, "unexpected MCP validate content type")
		return
	}

	if mcpResult.IsError {
		var env struct {
			Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
			Summary     string                   `json:"summary"`
		}
		if err := json.Unmarshal([]byte(tc.Text), &env); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("failed to parse MCP validate error: %v", err))
			return
		}
		result.Valid = false
		result.Diagnostics = env.Diagnostics
		appendDiagnosticStrings(result, env.Diagnostics)
		return
	}

	var output dryRunOutput
	if err := json.Unmarshal([]byte(tc.Text), &output); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to parse MCP validate output: %v", err))
		return
	}
	result.Valid = output.Valid
	result.SlideCount = output.SlideCount
	result.ChartCount = output.ChartCount
	result.DiagramCount = output.DiagramCount
	result.TableCount = output.TableCount
	result.ShapeCount = output.ShapeCount
	appendFindingStrings(result, output.Findings.Findings)
}

// appendDiagnosticStrings copies diagnostic messages into result.Errors /
// result.Warnings based on severity. Info-level diagnostics are dropped from the
// human string slices.
func appendDiagnosticStrings(result *validateResult, diags []diagnostics.Diagnostic) {
	for _, d := range diags {
		switch d.Severity {
		case diagnostics.SeverityError:
			result.Errors = append(result.Errors, d.Message)
		case diagnostics.SeverityWarning:
			result.Warnings = append(result.Warnings, d.Message)
		}
	}
}

// appendFindingStrings copies finding messages from a Findings envelope into
// result.Errors / result.Warnings based on severity (info-level findings are
// dropped from the human string slices), and reconstructs result.Diagnostics
// with the un-namespaced legacy codes. The CLI's internal validateResult uses
// the legacy code vocabulary so it stays consistent with the error-path
// diagnostics and the describe-finding registry.
func appendFindingStrings(result *validateResult, findings []diagnostics.Finding) {
	for _, f := range findings {
		switch f.Severity {
		case diagnostics.SeverityError:
			result.Errors = append(result.Errors, f.Message)
		case diagnostics.SeverityWarning:
			result.Warnings = append(result.Warnings, f.Message)
		}
		result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
			Code:     legacyFindingCode(f.Code),
			Message:  f.Message,
			Severity: f.Severity,
		})
	}
}

// legacyFindingCode strips the leading "<NAMESPACE>." prefix from a namespaced
// finding code, recovering the un-namespaced code used by the CLI's internal
// validateResult and the describe-finding registry. Codes without a known
// namespace prefix are returned unchanged.
func legacyFindingCode(code string) string {
	if i := strings.IndexByte(code, '.'); i >= 0 {
		prefix := code[:i]
		for _, ns := range diagnostics.AllNamespaces() {
			if prefix == ns {
				return code[i+1:]
			}
		}
	}
	return code
}

// runValidateMCPFormat delegates to the MCP validate handler to produce output
// identical to validate_input. This is the single structured JSON contract for
// CLI validate output: --format=json, --format=ndjson, --json, and
// --json-output all route here. strictUnknownKeys is passed straight through
// to the MCP handler so the JSON output matches what validate_input would
// return for the same flag value.
//
// outputPath selects the destination: "" or "-" means stdout; any other value
// is a file path. For --format=ndjson the output is one JSON object per file,
// per line. For --format=json each file's response is pretty-printed; when
// multiple files are validated, their pretty-printed objects are concatenated
// (matching the prior --format=json behavior).
func runValidateMCPFormat(files []string, templatesDir string, fitReport, verboseFit bool, format string, strictUnknownKeys bool, outputPath string) error {
	mc := cliMCPConfig(templatesDir, "")
	hasErrors := false

	var out io.Writer = os.Stdout
	if outputPath != "" && outputPath != "-" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create JSON output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	for _, filePath := range files {
		jsonInput, err := readJSONInput(filePath)
		if err != nil {
			return err
		}

		var presentation any
		if err := json.Unmarshal([]byte(jsonInput), &presentation); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", filePath, err)
		}

		args := map[string]any{
			"presentation":        presentation,
			"fit_report":          fitReport,
			"verbose_fit":         verboseFit,
			"strict_unknown_keys": strictUnknownKeys,
		}

		result, err := mc.handleValidate(context.Background(), mcpRequestWithArgs(args))
		if err != nil {
			return fmt.Errorf("validate %s: %w", filePath, err)
		}

		if result.IsError {
			hasErrors = true
		}

		if err := writeMCPResultJSON(out, result, format); err != nil {
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// writeMCPResultJSON writes the text content of an MCP CallToolResult to w as
// either pretty-printed JSON ("json") or one JSON object per line ("ndjson").
// Unlike printMCPResultJSON, both success and error envelopes are written to
// the destination (so machine-readable consumers can parse failures), and
// nothing is written to stderr.
func writeMCPResultJSON(w io.Writer, result *mcpgo.CallToolResult, format string) error {
	if result == nil || len(result.Content) == 0 {
		return fmt.Errorf("empty result")
	}
	tc, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		return fmt.Errorf("unexpected content type")
	}

	// Decode once; the MCP text content may be either compact or pretty-printed
	// depending on session negotiation, so we re-emit in the format CLI users
	// actually requested.
	var raw any
	if err := json.Unmarshal([]byte(tc.Text), &raw); err != nil {
		// Not JSON — write as-is.
		_, err := fmt.Fprintln(w, tc.Text)
		return err
	}

	if format == "ndjson" {
		// NDJSON: one compact JSON object per file, terminated by newline.
		compact, err := json.Marshal(raw)
		if err != nil {
			_, err := fmt.Fprintln(w, tc.Text)
			return err
		}
		if _, err := w.Write(append(compact, '\n')); err != nil {
			return err
		}
		return nil
	}

	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		_, err := fmt.Fprintln(w, tc.Text)
		return err
	}
	if _, err := w.Write(append(pretty, '\n')); err != nil {
		return err
	}
	return nil
}

// printValidateResult prints a human-readable validation summary for one file.
func printValidateResult(r validateResult) {
	status := "VALID"
	if !r.Valid {
		status = "INVALID"
	}

	fmt.Printf("%s: %s\n", r.File, status)
	fmt.Printf("  Slides: %d | Charts: %d | Diagrams: %d | Tables: %d | Shapes: %d\n",
		r.SlideCount, r.ChartCount, r.DiagramCount, r.TableCount, r.ShapeCount)

	for _, e := range r.Errors {
		fmt.Printf("  ERROR: %s\n", e)
	}
	for _, w := range r.Warnings {
		fmt.Printf("  WARN:  %s\n", w)
	}
	fmt.Println()
}
