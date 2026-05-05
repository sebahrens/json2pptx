package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	jsonOut := fs.Bool("json", false, "Output results as JSON to stdout")
	jsonOutputPath := fs.String("json-output", "", "Write JSON results to file (use - for stdout)")
	fitReport := fs.Bool("fit-report", false, "Run per-cell text overflow measurement and print findings")
	verboseFit := fs.Bool("verbose-fit", false, "Return all fit findings without the per-slide budget limit")
	format := fs.String("format", "", "Output format: json (MCP-identical dryRunOutput), ndjson, or human (default)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx validate [options] <file.json ...>\n\n")
		fmt.Fprintf(os.Stderr, "Validate JSON slide files without generating PPTX output.\n")
		fmt.Fprintf(os.Stderr, "Reports errors, warnings, and content statistics.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -json-output results.json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -json-output - slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -template corporate slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -fit-report slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -fit-report -verbose-fit slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate -format=json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx validate slides.json chapter2.json chapter3.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("at least one input file is required")
	}

	// When -format=json is used, delegate to the MCP validate handler to get
	// the identical dryRunOutput shape (with structured diagnostics and fit_findings).
	if *format == "json" || *format == "ndjson" {
		return runValidateMCPFormat(args, *templatesDir, *fitReport, *verboseFit, *format)
	}

	// Suppress unused warnings for flags consumed below.
	_ = templateName

	hasErrors := false
	var results []validateResult

	for _, filePath := range args {
		result := validateJSONFile(filePath, *templatesDir)
		results = append(results, result)
		if !result.Valid {
			hasErrors = true
		}
	}

	// Fit-report: walk all tables and shape-grid text cells for overflow.
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
			writeFitReportNDJSON(os.Stdout, findings)
			for _, f := range findings {
				if f.Action == "refuse" {
					hasErrors = true
				}
			}
		}
	}

	// Resolve effective JSON output destination:
	// -json-output takes precedence; -json is shorthand for -json-output -.
	effectiveJSONPath := *jsonOutputPath
	if effectiveJSONPath == "" && *jsonOut {
		effectiveJSONPath = "-"
	}

	// Output results.
	if effectiveJSONPath != "" {
		if err := writeValidateJSON(effectiveJSONPath, results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			printValidateResult(r)
		}
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
// optionally against a template.
func validateJSONFile(filePath, templatesDir string) validateResult { //nolint:gocognit,gocyclo
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

	// Check for unknown keys (warn severity — additionalProperties:false).
	for _, ve := range checkInputUnknownKeys(content) {
		result.Warnings = append(result.Warnings, ve.Error())
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
		if slide.LayoutID == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("slide %d: layout_id is required", i+1))
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

// writeValidateJSON writes validation results as JSON to a file path or stdout
// (when path is "-").
func writeValidateJSON(path string, results []validateResult) error {
	var v any = results
	if len(results) == 1 {
		v = results[0]
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if path == "-" {
		_, err = os.Stdout.Write(data)
		_, _ = os.Stdout.WriteString("\n")
	} else {
		err = os.WriteFile(path, append(data, '\n'), 0644)
	}
	if err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}
	return nil
}

// runValidateMCPFormat delegates to the MCP validate handler to produce output
// identical to validate_input. This ensures CLI and MCP return the same shape.
func runValidateMCPFormat(files []string, templatesDir string, fitReport, verboseFit bool, format string) error {
	mc := cliMCPConfig(templatesDir, "")
	hasErrors := false

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
			"presentation": presentation,
			"fit_report":   fitReport,
			"verbose_fit":  verboseFit,
		}

		result, err := mc.handleValidate(context.Background(), mcpRequestWithArgs(args))
		if err != nil {
			return fmt.Errorf("validate %s: %w", filePath, err)
		}

		if result.IsError {
			hasErrors = true
		}

		if format == "ndjson" {
			// NDJSON: one JSON object per line, no indentation.
			if len(result.Content) > 0 {
				if tc, ok := result.Content[0].(mcpgo.TextContent); ok {
					fmt.Println(tc.Text)
				}
			}
		} else {
			if err := printMCPResultJSON(result); err != nil {
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
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
