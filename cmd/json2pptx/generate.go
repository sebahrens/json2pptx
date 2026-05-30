package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

func runGenerate() error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	templateName := fs.String("template", "", "Template name (without .pptx extension)")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./output", "Output directory for generated PPTX files (or a .pptx file path, e.g. /tmp/deck.pptx)")
	configPath := fs.String("config", "", "Path to config file (optional)")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	jsonInput := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	jsonOutputReport := fs.String("json-output-report", "", "Path for the JSON result report (success, warnings, quality score) in headless mode")
	jsonOutput := fs.String("json-output", "", "DEPRECATED: alias for --json-output-report")
	chartPNG := fs.Bool("chart-png", false, "DEPRECATED: Use PNG instead of native SVG for charts. Native SVG is now the default and recommended strategy.")
	dryRun := fs.Bool("dry-run", false, "Validate input and show layout selections without generating output")
	fs.BoolVar(dryRun, "n", false, "Shorthand for --dry-run")
	strictFit := fs.String("strict-fit", "warn", "Text-fit checking mode: off, warn (default), or strict (refuse on overflow)")
	partial := fs.Bool("partial", false, "Enable partial mode: skip failing slides instead of aborting the entire deck")
	outputValidation := fs.String("output-validation", "strict", "Post-generation PPTX validation: off (skip), warn (report findings), or strict (default; fail on blocking findings)")
	designMode := fs.String("design-mode", "", "Override the deck's design_mode field: constrained (default, enforces scheme colors and template-managed sizes) or free (allows raw hex colors and absolute sizes). Empty preserves the JSON setting.")
	strictUnknownKeys := fs.Bool("strict-unknown-keys", false, "Fail-fast on misspelled/unknown JSON keys: when true, unknown keys are errors that block generation; when false (default), they are reported as warnings. Mirrors MCP generate_presentation strict_unknown_keys.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx generate [options] --json <file.json>\n\n")
		fmt.Fprintf(os.Stderr, "Convert JSON slide descriptions to PowerPoint presentations.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --json slides.json --output ./output\n")
		fmt.Fprintf(os.Stderr, "  cat slides.json | json2pptx generate --json - --json-output-report result.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --dry-run --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate -n --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --strict-fit=strict --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --output-validation=warn --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --design-mode=free --json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --strict-unknown-keys --json slides.json   # fail-fast on typo'd fields\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Reconcile the deprecated --json-output flag with its preferred alias
	// --json-output-report. Both name the same JSON result report destination.
	// We also record whether --templates-dir/--output were explicitly provided so
	// their non-empty default flag values don't silently overwrite config-file or
	// environment-derived directories (config/env precedence regression).
	var jsonOutputSet, jsonOutputReportSet, templatesDirSet, outputDirSet bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "json-output":
			jsonOutputSet = true
		case "json-output-report":
			jsonOutputReportSet = true
		case "templates-dir":
			templatesDirSet = true
		case "output":
			outputDirSet = true
		}
	})

	// Pass directory overrides only when the user explicitly set the flag. An
	// empty string downstream means "no override" — config-file and environment
	// values win, falling back to config.DefaultConfig() (./templates, ./output).
	effTemplatesDir := ""
	if templatesDirSet {
		effTemplatesDir = *templatesDir
	}
	effOutputDir := ""
	if outputDirSet {
		effOutputDir = *outputDir
	}
	resolvedJSONOutput, err := resolveJSONOutputReport(*jsonOutput, *jsonOutputReport, jsonOutputSet, jsonOutputReportSet)
	if err != nil {
		return err
	}

	// Validate --strict-fit value
	switch *strictFit {
	case "off", "warn", "strict":
		// valid
	default:
		return fmt.Errorf("invalid --strict-fit value %q: must be off, warn, or strict", *strictFit)
	}

	// Validate --output-validation value
	switch *outputValidation {
	case "off", "warn", "strict":
		// valid
	default:
		return fmt.Errorf("invalid --output-validation value %q: must be off, warn, or strict", *outputValidation)
	}

	// Validate --design-mode value (empty string preserves the JSON field).
	switch *designMode {
	case "", "constrained", "free":
		// valid
	default:
		return fmt.Errorf("invalid --design-mode value %q: must be constrained or free", *designMode)
	}

	// JSON input is required
	if *jsonInput == "" {
		fs.Usage()
		return fmt.Errorf("JSON input is required: use --json <file.json> or --json - for stdin")
	}

	// Fail fast if the font subsystem is broken — this prevents silent
	// "fits perfectly" results from textfit when no fonts can be loaded.
	if err := fontcache.Verify(); err != nil {
		return fmt.Errorf("font subsystem check failed: %w", err)
	}

	if *dryRun {
		return runJSONDryRun(*jsonInput, effTemplatesDir, *configPath, *designMode, *strictUnknownKeys)
	}
	return runJSONMode(*jsonInput, resolvedJSONOutput, effTemplatesDir, effOutputDir, *configPath, *verbose, *chartPNG, *templateName, *strictFit, *partial, *outputValidation, *designMode, *strictUnknownKeys)
}

// resolveJSONOutputReport reconciles the deprecated --json-output flag with its
// preferred alias --json-output-report. Both names target the same JSON result
// report destination. oldSet/newSet report which of the two flags were
// explicitly provided on the command line. When both are set with differing
// values, it returns an error so the caller cannot silently pick one.
func resolveJSONOutputReport(oldVal, newVal string, oldSet, newSet bool) (string, error) {
	switch {
	case oldSet && newSet:
		if oldVal != newVal {
			return "", fmt.Errorf("--json-output and --json-output-report were set to conflicting values (%q vs %q); set only one (prefer --json-output-report)", oldVal, newVal)
		}
		return newVal, nil
	case newSet:
		return newVal, nil
	case oldSet:
		return oldVal, nil
	default:
		return "", nil
	}
}
