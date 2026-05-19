package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
)

// runValidate implements the "validate" subcommand. It validates JSON slide
// input against the template without generating PPTX output. This delegates
// to the same validation logic used by the dry-run mode.
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

// validateJSONFile validates a single JSON input file against the schema and
// optionally against a template. When strictUnknownKeys is true, unknown JSON
// keys are reported as errors (matching MCP validate_input strict_unknown_keys
// semantics) instead of warnings.
func validateJSONFile(filePath, templatesDir string, strictUnknownKeys bool) validateResult { //nolint:gocognit,gocyclo
	result := validateResult{
		File:     filePath,
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Read the file.
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

	// Parse as PresentationInput.
	var input PresentationInput
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(content, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			result.Valid = false
			msg := fmt.Sprintf("failed to apply patch: %v", patchErr)
			result.Errors = append(result.Errors, msg)
			result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
				Code: "PATCH_ERROR", Message: msg, Severity: diagnostics.SeverityError,
			})
			return result
		}
		input = *patched
	} else {
		if err := json.Unmarshal(content, &input); err != nil {
			result.Valid = false
			msg := fmt.Sprintf("failed to parse JSON: %v", err)
			result.Errors = append(result.Errors, msg)
			result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
				Code: "INVALID_JSON", Message: msg, Severity: diagnostics.SeverityError,
			})
			return result
		}
	}
	applyDefaults(&input)

	// Check for unknown keys (additionalProperties:false). Warnings by default;
	// when strictUnknownKeys is set, unknown keys become errors (mirroring MCP
	// validate_input strict_unknown_keys=true semantics).
	for _, ve := range checkInputUnknownKeys(content) {
		if strictUnknownKeys {
			result.Valid = false
			result.Errors = append(result.Errors, ve.Error())
		} else {
			result.Warnings = append(result.Warnings, ve.Error())
		}
	}

	// Enum validation — unknown values for transition, transition_speed, build, background.fit.
	for _, ve := range checkInputEnumValues(&input) {
		result.Valid = false
		result.Errors = append(result.Errors, ve.Error())
	}

	// Validate required fields.
	if input.Template == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "template is required in JSON input")
	}
	if len(input.Slides) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "at least one slide is required")
	}
	if !result.Valid {
		// Build diagnostics for early return.
		for _, e := range result.Errors {
			result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
				Code: "VALIDATION_ERROR", Message: e, Severity: diagnostics.SeverityError,
			})
		}
		for _, w := range result.Warnings {
			result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
				Code: "VALIDATION_WARNING", Message: w, Severity: diagnostics.SeverityWarning,
			})
		}
		return result
	}

	result.SlideCount = len(input.Slides)

	// Validate content items.
	for i, slide := range input.Slides {
		// layout_id and slide_type are alternatives: layout_id pins a specific
		// template layout, while slide_type is a hint for auto-selection. The
		// generator accepts either (with auto-selection picking a layout when
		// only slide_type is provided), so the validator should mirror that.
		if slide.LayoutID == "" && slide.SlideType == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("slide %d: layout_id or slide_type is required", i+1))
		}
		for j, item := range slide.Content {
			if item.PlaceholderID == "" {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("slide %d, content %d: placeholder_id is required", i+1, j+1))
			}
			if item.Type == "" {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("slide %d, content %d: type is required", i+1, j+1))
			} else {
				// Validate type value.
				switch item.Type {
				case "text", "bullets", "body_and_bullets", "bullet_groups", "table", "image", "chart", "diagram":
					// valid
				default:
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("slide %d, content %d: unknown type %q", i+1, j+1, item.Type))
				}
			}
			// Count content types.
			switch item.Type {
			case "chart":
				result.ChartCount++
			case "diagram":
				result.DiagramCount++
			case "table":
				result.TableCount++
			}
			// Validate content value is parseable.
			if item.Type != "" {
				if _, err := item.ResolveValue(); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("slide %d, content %d: %v", i+1, j+1, err))
				}
			}
			// Detect legacy authoring form.
			if item.UsesLegacyValue() {
				typedField := item.Type + "_value"
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("slide %d, content %d: uses legacy \"value\" field; prefer \"%s\" for new decks", i+1, j+1, typedField))
			}
		}

		// Validate shape_grid if present.
		if slide.ShapeGrid != nil {
			gridCounts, gridWarnings, gridErrors, _ := validateShapeGrid(slide.ShapeGrid, i+1)
			result.ShapeCount += gridCounts.Shapes
			result.TableCount += gridCounts.Tables
			result.DiagramCount += gridCounts.Diagrams
			result.Warnings = append(result.Warnings, gridWarnings...)
			if len(gridErrors) > 0 {
				result.Valid = false
				result.Errors = append(result.Errors, gridErrors...)
			}
		}

		// Measure table cell overflow via textfit.
		for _, item := range slide.Content {
			if item.Type != "table" {
				continue
			}
			table := resolveTableFromContent(&item)
			if table == nil {
				continue
			}
			spec := table.ToTableSpec()
			for _, w := range generator.WarnTableCellOverflow(spec, i) {
				result.Warnings = append(result.Warnings, w.String())
			}

			// Warn when both header_background and style_id are explicitly authored.
			if table.Style != nil {
				if w := generator.WarnStyleCollision(i,
					table.Style.HeaderBackground != nil,
					table.Style.StyleID != "",
				); w != "" {
					result.Warnings = append(result.Warnings, w)
				}
			}
		}
	}

	// Build diagnostics from string errors/warnings so CLI -json output
	// emits the same {code, path, severity, fix} shape as MCP tools.
	for _, e := range result.Errors {
		result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
			Code:     "VALIDATION_ERROR",
			Message:  e,
			Severity: diagnostics.SeverityError,
		})
	}
	for _, w := range result.Warnings {
		result.Diagnostics = append(result.Diagnostics, diagnostics.Diagnostic{
			Code:     "VALIDATION_WARNING",
			Message:  w,
			Severity: diagnostics.SeverityWarning,
		})
	}

	return result
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
