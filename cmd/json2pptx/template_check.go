package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/template"
)

// runTemplateCheck implements the template-check subcommand. The actual
// conformance logic lives in internal/template/conformance.go so that the
// internal/template/conformance_corpus_test.go corpus test (and any other
// package that needs to enforce template conformance programmatically) can
// reuse it.
func runTemplateCheck() error {
	fs := flag.NewFlagSet("template-check", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) != 1 {
		return fmt.Errorf("usage: json2pptx template-check [--json] <template.pptx>")
	}
	templatePath := args[0]

	report, err := template.CheckConformance(templatePath)
	if err != nil {
		return err
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printTemplateCheckText(report)

	if !report.Pass {
		return fmt.Errorf("template conformance check failed")
	}
	return nil
}

// printTemplateCheckText outputs a conformance report as human-readable text.
func printTemplateCheckText(report *template.ConformanceReport) {
	if report.Pass {
		fmt.Printf("PASS: %s\n", report.Template)
	} else {
		fmt.Printf("FAIL: %s\n", report.Template)
	}
	fmt.Println()

	for _, c := range report.Checks {
		var icon string
		switch c.Status {
		case template.ConformanceStatusPass:
			icon = "  [OK]  "
		case template.ConformanceStatusFail:
			icon = "  [FAIL]"
		case template.ConformanceStatusWarn:
			icon = "  [WARN]"
		}

		line := fmt.Sprintf("%s %s", icon, c.Check)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		fmt.Println(line)
	}
}
