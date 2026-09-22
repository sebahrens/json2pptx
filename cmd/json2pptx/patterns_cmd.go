package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// runPatterns implements the "patterns" subcommand with sub-subcommands:
// list, show, validate, expand.
func runPatterns() error {
	if len(os.Args) < 2 {
		printPatternsUsage()
		return nil
	}

	subcmd := os.Args[1]
	// Shift args for the sub-subcommand's flags
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "list":
		return runPatternsList()
	case "show":
		return runPatternsShow()
	case "validate":
		return runPatternsValidate()
	case "expand":
		return runPatternsExpand()
	case "help", "-h", "--help":
		printPatternsUsage()
		return nil
	default:
		printPatternsUsage()
		return fmt.Errorf("unknown patterns subcommand %q", subcmd)
	}
}

func printPatternsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx patterns <command> [options]

Commands:
  list                         List all available patterns
  show <name>                  Show full schema and details for a pattern
  validate <name> <file.json>  Validate pattern values without generating
  expand <name> <file.json>    Expand pattern to shape_grid JSON

Run 'json2pptx patterns <command> -h' for command-specific help.
`)
}

// ---------------------------------------------------------------------------
// patterns list
// ---------------------------------------------------------------------------

func runPatternsList() error {
	fs := flag.NewFlagSet("patterns list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns list [--json]\n\n")
		fmt.Fprintf(os.Stderr, "List all available named patterns.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	reg := patterns.Default()
	all := reg.List()

	if *jsonOut {
		entries := make([]skillPatternCompact, len(all))
		for i, p := range all {
			cells := p.CellsHint()
			var sizeBytes int
			if ex, ok := p.(patterns.Exemplar); ok {
				sizeBytes, _ = patterns.CanonicalSizeBytes(p, ex.ExemplarValues())
			}
			supportsCallout := false
			if cs, ok := p.(patterns.CalloutSupport); ok {
				supportsCallout = cs.SupportsCallout()
			}
			tax := p.Taxonomy()
			entries[i] = skillPatternCompact{
				Name:                     p.Name(),
				Cells:                    cells,
				UseWhen:                  p.UseWhen(),
				NotWhen:                  p.NotWhen(),
				Category:                 tax.Category,
				NarrativeRole:            tax.NarrativeRole,
				PairsWith:                tax.PairsWith,
				ComposesWith:             tax.ComposesWith,
				RoleOnSlide:              tax.RoleOnSlide,
				DensityClass:             tax.DensityClass,
				AccentWeight:             tax.AccentWeight,
				SupportsCallout:          supportsCallout,
				EstimatedPromptSizeBytes: sizeBytes,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable table
	fmt.Printf("%-25s %-10s %s\n", "NAME", "CELLS", "USE WHEN")
	fmt.Printf("%-25s %-10s %s\n", strings.Repeat("-", 25), strings.Repeat("-", 10), strings.Repeat("-", 40))
	for _, p := range all {
		fmt.Printf("%-25s %-10s %s\n", p.Name(), p.CellsHint(), p.UseWhen())
	}
	return nil
}

// ---------------------------------------------------------------------------
// patterns show
// ---------------------------------------------------------------------------

func runPatternsShow() error {
	fs := flag.NewFlagSet("patterns show", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns show [--json] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Show full schema and details for a named pattern.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("pattern name is required")
	}
	name := args[0]

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		return unknownPatternError(name, reg)
	}

	if *jsonOut {
		schemaJSON := patterns.SchemaJSON(pat)
		tax := pat.Taxonomy()
		result := skillPatternFull{
			Name:            pat.Name(),
			Description:     pat.Description(),
			UseWhen:         pat.UseWhen(),
			NotWhen:         pat.NotWhen(),
			Version:         pat.Version(),
			Schema:          schemaJSON,
			TextBudgetGuide: computeTextBudgetGuide(pat),
			ComposesWith:    tax.ComposesWith,
			RoleOnSlide:     tax.RoleOnSlide,
		}
		result.Cells = pat.CellsHint()
		if cs, ok := pat.(patterns.CalloutSupport); ok {
			result.SupportsCallout = cs.SupportsCallout()
			if cs.SupportsCallout() {
				result.CalloutSchema = patternCalloutSchemaJSON()
			}
		}
		if ex, ok := pat.(patterns.Exemplar); ok {
			result.ExampleValues = ex.ExemplarValues()
		}
		result.RenderingCapabilities = patternRenderingCapabilities(pat.Name())
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	fmt.Printf("Pattern: %s (v%d)\n", pat.Name(), pat.Version())
	fmt.Printf("Description: %s\n", pat.Description())
	fmt.Printf("Use when: %s\n", pat.UseWhen())
	fmt.Printf("Not when: %s\n", pat.NotWhen())
	fmt.Printf("Cells: %s\n", pat.CellsHint())
	if cs, ok := pat.(patterns.CalloutSupport); ok && cs.SupportsCallout() {
		fmt.Printf("Supports callout: yes\n")
	} else {
		fmt.Printf("Supports callout: no\n")
	}
	fmt.Println()
	schemaJSON := patterns.SchemaJSONIndent(pat)
	fmt.Printf("Schema:\n%s\n", string(schemaJSON))

	// Show cell_overrides example if the pattern supports them
	if pat.NewCellOverride() != nil {
		fmt.Println()
		fmt.Printf("cell_overrides (per-cell, keyed by cell index):\n")
		fmt.Printf("  Allowed keys: %s\n", patterns.CellOverrideAllowedList())
		fmt.Printf("  Example:\n")
		fmt.Printf("    \"cell_overrides\": {\n")
		fmt.Printf("      \"0\": {\"accent_bar\": true, \"emphasis\": \"bold\"},\n")
		fmt.Printf("      \"2\": {\"color\": \"accent2\", \"font_size\": 11}\n")
		fmt.Printf("    }\n")
	}
	return nil
}

// ---------------------------------------------------------------------------
// patterns validate
// ---------------------------------------------------------------------------

func runPatternsValidate() error {
	fs := flag.NewFlagSet("patterns validate", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output a standard findings envelope as JSON")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name for template-aware fit checks")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns validate [--json] [--templates-dir DIR] [--template NAME] <name> <values.json>\n\n")
		fmt.Fprintf(os.Stderr, "Validate pattern values and content fit without generating a deck.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) < 2 {
		fs.Usage()
		return fmt.Errorf("pattern name and values file are required")
	}
	name := args[0]
	valuesFile := args[1]

	reg := patterns.Default()
	if _, ok := reg.Get(name); !ok {
		if *jsonOut {
			return emitPatternCLIValidation(name, nil, *templateName, true,
				patternInputDiagnostics(unknownPatternInputError(reg, name), "", ""))
		}
		return unknownPatternError(name, reg)
	}

	// Read and parse values file — expects a PatternInput-shaped JSON
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		if *jsonOut {
			code := diagnostics.CodeInvalidParameter
			if os.IsNotExist(err) {
				code = diagnostics.CodeFileNotFound
			}
			return emitPatternCLIValidation(name, nil, *templateName, true, []diagnostics.Diagnostic{{
				Code: code, Message: err.Error(), Path: "/input_file", Severity: diagnostics.SeverityError,
			}})
		}
		return fmt.Errorf("failed to read %s: %w", valuesFile, err)
	}

	pi, err := parsePatternInputFile(content, name)
	if err != nil {
		if *jsonOut {
			return emitPatternCLIValidation(name, content, *templateName, true, []diagnostics.Diagnostic{{
				Code: diagnostics.CodeInvalidParameter, Message: err.Error(), Path: "/values", Severity: diagnostics.SeverityError,
			}})
		}
		return err
	}

	expandCtx, boundsSource, err := resolveExpandContext(*templateName, *templatesDir)
	if err != nil {
		if *jsonOut {
			return emitPatternCLIValidation(name, content, *templateName, true, []diagnostics.Diagnostic{{
				Code: diagnostics.CodeTemplateNotFound, Message: err.Error(), Path: "/template", Severity: diagnostics.SeverityError,
			}})
		}
		return err
	}
	ds := patternValidationDiagnostics(pi, expandCtx, boundsSource, reg)
	return emitPatternCLIValidation(name, content, *templateName, *jsonOut, ds)
}

func emitPatternCLIValidation(name string, content []byte, templateName string, jsonOut bool, ds []diagnostics.Diagnostic) error {
	inputHash := ""
	if len(content) > 0 {
		inputHash = diagnostics.ComputeInputSHA256(content)
	}
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "patterns validate",
		Template:    templateName,
		InputSHA256: inputHash,
	}, ds)
	if jsonOut {
		data, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal pattern findings: %w", err)
		}
		fmt.Println(string(data))
	} else if envelope.OK {
		fmt.Printf("Pattern %q: valid\n", name)
		for _, d := range ds {
			fmt.Printf("  %s: %s\n", d.Code, d.Message)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Pattern %q: validation failed\n", name)
		for _, d := range ds {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", d.Code, d.Message)
		}
	}
	if !envelope.OK {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// ---------------------------------------------------------------------------
// patterns expand
// ---------------------------------------------------------------------------

func runPatternsExpand() error {
	fs := flag.NewFlagSet("patterns expand", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON (always JSON, flag is for consistency)")
	_ = jsonOut // expand always outputs JSON; flag accepted for consistency
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Template name for template-aware bounds resolution")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns expand [--json] [--templates-dir DIR] [--template NAME] <name> <values.json>\n\n")
		fmt.Fprintf(os.Stderr, "Expand a pattern to its shape_grid equivalent.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) < 2 {
		fs.Usage()
		return fmt.Errorf("pattern name and values file are required")
	}
	name := args[0]
	valuesFile := args[1]

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		return unknownPatternError(name, reg)
	}

	// Read values file
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", valuesFile, err)
	}

	pi, err := parsePatternInputFile(content, name)
	if err != nil {
		return err
	}

	// Build ExpandContext — template-aware when -template is provided
	expandCtx, boundsSource, err := resolveExpandContext(*templateName, *templatesDir)
	if err != nil {
		return err
	}

	grid, expandWarnings, err := expandPattern(pi, expandCtx, reg)
	if err != nil {
		return err
	}

	// Compute cell budgets and density warnings from the resolved grid
	cellBudgets, densityWarnings := computeCellBudgets(grid, expandCtx)

	// Suggest alternative layouts when density is consistently suboptimal
	layoutSuggestions := suggestAlternativeLayouts(pat.Name(), cellBudgets, reg)

	result := struct {
		Pattern           string                     `json:"pattern"`
		Version           int                        `json:"version"`
		BoundsSource      string                     `json:"bounds_source"`
		BoundsAssumption  string                     `json:"bounds_assumption"`
		ShapeGrid         *jsonschema.ShapeGridInput `json:"shape_grid"`
		CellBudgets       []cellBudgetEntry          `json:"cell_budgets,omitempty"`
		DensityWarnings   []cellDensityWarning       `json:"density_warnings,omitempty"`
		LayoutSuggestions []layoutSuggestion         `json:"layout_suggestions,omitempty"`
		// Warnings are the pattern's own PostExpandWarnings about the values
		// it was just given (go-slide-creator-wn4v).
		Warnings []string `json:"warnings,omitempty"`
	}{
		Pattern:           pat.Name(),
		Version:           pat.Version(),
		BoundsSource:      boundsSource,
		BoundsAssumption:  "full_content_area",
		ShapeGrid:         grid,
		Warnings:          expandWarnings,
		CellBudgets:       cellBudgets,
		DensityWarnings:   densityWarnings,
		LayoutSuggestions: layoutSuggestions,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// resolveExpandContext builds an ExpandContext from a template (if provided) or
// falls back to hardcoded defaults. Returns the context, a bounds_source label,
// and any error.
func resolveExpandContext(templateName, templatesDir string) (patterns.ExpandContext, string, error) {
	// Default fallback
	defaultCtx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}

	if templateName == "" {
		return defaultCtx, "default_fallback", nil
	}

	templatePath, cleanup, err := resolveTemplatePath(templateName, templatesDir)
	if err != nil {
		return patterns.ExpandContext{}, "", fmt.Errorf("template %q not found: %w", templateName, err)
	}
	defer cleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return patterns.ExpandContext{}, "", fmt.Errorf("failed to open template %q: %w", templateName, err)
	}
	defer func() { _ = reader.Close() }()

	ctx := patterns.ExpandContext{
		Theme: template.ParseTheme(reader),
	}

	w, h := template.ParseSlideDimensions(reader)
	if w > 0 {
		ctx.SlideWidth = w
	} else {
		ctx.SlideWidth = defaultCtx.SlideWidth
	}
	if h > 0 {
		ctx.SlideHeight = h
	} else {
		ctx.SlideHeight = defaultCtx.SlideHeight
	}

	// Resolve layout bounds from template layouts
	layouts, err := template.ParseLayouts(reader)
	if err == nil && len(layouts) > 0 {
		ctx.LayoutBounds = layoutBoundsFromLayouts(layouts, ctx.SlideWidth, ctx.SlideHeight)
	} else {
		// Fall back to percentage-based defaults using the template's slide dimensions
		db := shapegrid.DefaultBounds(ctx.SlideWidth, ctx.SlideHeight)
		ctx.LayoutBounds = patterns.LayoutBounds{
			X: db.X, Y: db.Y,
			Width: db.CX, Height: db.CY,
		}
	}

	return ctx, "template", nil
}

// layoutBoundsFromLayouts derives LayoutBounds from parsed template layouts by
// reusing the same virtual layout resolution logic as the generate path.
func layoutBoundsFromLayouts(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) patterns.LayoutBounds {
	if vl := resolveVirtualLayout(layouts, slideWidth, slideHeight); vl != nil {
		return patterns.LayoutBounds{
			X: vl.Bounds.X, Y: vl.Bounds.Y,
			Width: vl.Bounds.CX, Height: vl.Bounds.CY,
		}
	}
	// No suitable layout found — use percentage-based defaults
	db := shapegrid.DefaultBounds(slideWidth, slideHeight)
	return patterns.LayoutBounds{
		X: db.X, Y: db.Y,
		Width: db.CX, Height: db.CY,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parsePatternInputFile parses a values file. It accepts two formats:
// 1. A full PatternInput object (with "values", "overrides", "cell_overrides" keys)
// 2. A bare values array/object (treated as the "values" field directly)
func parsePatternInputFile(content []byte, name string) (*PatternInput, error) {
	// Try full PatternInput first
	var pi PatternInput
	if err := json.Unmarshal(content, &pi); err == nil && len(pi.Values) > 0 {
		// The explicit CLI argument wins over an embedded name so validate and
		// expand cannot silently operate on different patterns for one command.
		pi.Name = name
		return &pi, nil
	}

	// Fall back to bare values
	pi = PatternInput{
		Name:   name,
		Values: json.RawMessage(content),
	}
	return &pi, nil
}

// patternCalloutSchemaJSON returns the JSON Schema fragment for PatternCallout.
// This is a static schema describing the envelope-level callout DTO.
func patternCalloutSchemaJSON() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "description": "Optional callout band rendered below the pattern content",
  "properties": {
    "text": {
      "type": "string",
      "description": "Callout text content"
    },
    "emphasis": {
      "type": "string",
      "description": "Text emphasis style",
      "enum": ["bold", "italic", "bold-italic"]
    },
    "accent": {
      "type": "string",
      "description": "Scheme color reference (e.g. accent1, accent2)"
    }
  },
  "required": ["text"],
  "additionalProperties": false
}`)
}

// unknownPatternError returns a helpful error when a pattern name is not found.
func unknownPatternError(name string, reg *patterns.Registry) error {
	all := reg.List()
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name()
	}
	msg := fmt.Sprintf("unknown pattern %q", name)
	if suggestion, ok := reg.Suggest(name); ok {
		msg += fmt.Sprintf("; did you mean %q?", suggestion)
	}
	return fmt.Errorf("%s; available: %s\nHint: use `json2pptx patterns list` to see all patterns", msg, strings.Join(names, ", "))
}
