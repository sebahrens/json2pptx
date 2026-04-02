package main

import (
	"flag"
	"fmt"
	"os"
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
	fs.BoolVar(dryRun, "n", false, "Shorthand for -dry-run")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx generate [options] -json <file.json>\n\n")
		fmt.Fprintf(os.Stderr, "Convert JSON slide descriptions to PowerPoint presentations.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate -json slides.json -output ./output\n")
		fmt.Fprintf(os.Stderr, "  cat slides.json | json2pptx generate -json - -json-output result.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate -dry-run -json slides.json\n")
		fmt.Fprintf(os.Stderr, "  json2pptx generate -n -json slides.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// JSON input is required
	if *jsonInput == "" {
		fs.Usage()
		return fmt.Errorf("JSON input is required: use -json <file.json> or -json - for stdin")
	}

	_ = verbose

	if *dryRun {
		return runJSONDryRun(*jsonInput, *templatesDir, *configPath)
	}
	return runJSONMode(*jsonInput, *jsonOutput, *templatesDir, *outputDir, *configPath, *verbose, *chartPNG, *templateName)
}
