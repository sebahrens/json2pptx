// Package main provides a CLI tool to check template capabilities.
// It outputs which features (charts, images, two-column layouts) a template supports,
// including capabilities provided by synthesized layouts.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ahrens/go-slide-creator/internal/template"
	"github.com/ahrens/go-slide-creator/internal/types"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: templatecaps [options] <template.pptx>\n\n")
		fmt.Fprintf(os.Stderr, "Analyze PPTX template capabilities including layouts, placeholders, and theme.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	templatePath := flag.Arg(0)

	// Open template
	tmpl, err := template.OpenTemplate(templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening template: %v\n", err)
		os.Exit(1)
	}
	defer tmpl.Close()

	// Get layouts
	layouts, err := template.ParseLayouts(tmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing layouts: %v\n", err)
		os.Exit(1)
	}

	// Synthesize missing layouts (e.g., two-column) so capabilities reflect
	// what the generator actually supports, not just native template layouts.
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Layouts:      layouts,
	}
	template.SynthesizeIfNeeded(tmpl, analysis)
	layouts = analysis.Layouts

	// Check capabilities
	hasChart := false
	hasImage := false
	hasTwoColumn := false

	for _, layout := range layouts {
		// Charts can use dedicated chart placeholders OR body placeholders (rendered as embedded images)
		if layout.Capacity.HasChartSlot {
			hasChart = true
		}
		if layout.Capacity.HasImageSlot {
			hasImage = true
		}
		// Check for body/content placeholders
		bodyCount := 0
		for _, ph := range layout.Placeholders {
			if ph.Type == "body" || ph.Type == "content" {
				bodyCount++
			}
		}
		// Body placeholders can also host charts (via SVG embedding as images)
		if bodyCount > 0 {
			hasChart = true
		}
		// Check for two-column: need at least 2 body/content placeholders
		if bodyCount >= 2 {
			hasTwoColumn = true
		}
	}

	// Output capabilities (one per line for easy bash parsing)
	if hasChart {
		fmt.Println("HAS_CHART=true")
	} else {
		fmt.Println("HAS_CHART=false")
	}
	if hasImage {
		fmt.Println("HAS_IMAGE=true")
	} else {
		fmt.Println("HAS_IMAGE=false")
	}
	if hasTwoColumn {
		fmt.Println("HAS_TWO_COLUMN=true")
	} else {
		fmt.Println("HAS_TWO_COLUMN=false")
	}
}
