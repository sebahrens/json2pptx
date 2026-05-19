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
	outputDir := fs.String("output", "./output", "Output directory for generated PPTX files")
	configPath := fs.String("config", "", "Path to config file (optional)")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	jsonInput := fs.String("json", "", "Path to JSON input file (use - for stdin)")
	jsonOutput := fs.String("json-output", "", "Path for JSON result output (headless mode)")
	chartPNG := fs.Bool("chart-png", false, "DEPRECATED: Use PNG instead of native SVG for charts. Native SVG is now the default and recommended strategy.")
	dryRun := fs.Bool("dry-run", false, "Validate input and show layout selections without generating output")
	fs.BoolVar(dryRun, "n", false, "Shorthand for --dry-run")
	strictFit := fs.String("strict-fit", "warn", "Text-fit checking mode: off, warn (default), or strict (refuse on overflow)")
	partial := fs.Bool("partial", false, "Enable partial mode: skip failing slides instead of aborting the entire deck")
	outputValidation := fs.String("output-validation", "off", "Post-generation PPTX validation: off (default), warn (report findings), or strict (fail on blocking findings)")
	designMode := fs.String("design-mode", "", "Override the deck's design_mode field: constrained (default, enforces scheme colors and template-managed sizes) or free (allows raw hex colors and absolute sizes). Empty preserves the JSON setting.")
	strictUnknownKeys := fs.Bool("strict-unknown-keys", false, "Fail-fast on misspelled/unknown JSON keys: when true, unknown keys are errors that block generation; when false (default), they are reported as warnings. Mirrors MCP generate_presentation strict_unknown_keys.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx generate [options] --json <file.json>\n\n")
		fmt.Fprintf(os.Stderr, "Convert JSON slide descriptions to PowerPoint presentations.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate --json slides.json --output ./output\n")
		fmt.Fprintf(os.Stderr, "  cat slides.json | json2pptx generate --json - --json-output result.json\n")
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
		return runJSONDryRun(*jsonInput, *templatesDir, *configPath, *designMode, *strictUnknownKeys)
	}
	return runJSONMode(*jsonInput, *jsonOutput, *templatesDir, *outputDir, *configPath, *verbose, *chartPNG, *templateName, *strictFit, *partial, *outputValidation, *designMode, *strictUnknownKeys)
}
