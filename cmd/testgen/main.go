// Package main provides a CLI tool for generating PPTX files from markdown.
// It's used by the E2E visual test script to generate test presentations.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ahrens/go-slide-creator/internal/generator"
	"github.com/ahrens/go-slide-creator/internal/parser"
	"github.com/ahrens/go-slide-creator/internal/pipeline"
	"github.com/ahrens/go-slide-creator/internal/template"
	"github.com/ahrens/go-slide-creator/internal/types"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: testgen <markdown> <template> <output>")
		os.Exit(1)
	}

	markdownPath := os.Args[1]
	templatePath := os.Args[2]
	outputPath := os.Args[3]

	if err := run(markdownPath, templatePath, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(markdownPath, templatePath, outputPath string) error {
	// Read markdown
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("reading markdown: %w", err)
	}

	// Parse markdown using parser package.
	presentation, err := parser.Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing markdown: %w", err)
	}

	// Enrich title slides with frontmatter metadata (subtitle, author, date)
	pipeline.EnrichTitleSlides(presentation)

	// Auto-generate agenda slide if enabled in frontmatter
	if presentation.Metadata.AutoAgenda {
		pipeline.GenerateAgenda(presentation)
	}

	// Open template
	tmpl, err := template.OpenTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("opening template: %w", err)
	}
	defer tmpl.Close()

	// Get layouts
	layouts, err := template.ParseLayouts(tmpl)
	if err != nil {
		return fmt.Errorf("parsing layouts: %w", err)
	}

	// Synthesize missing layout capabilities (e.g., two-column layouts).
	// This is needed so templates without native two-column layouts can still
	// handle two-column content by generating synthetic layouts.
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Layouts:      layouts,
	}
	template.SynthesizeIfNeeded(tmpl, analysis)

	// Collect synthetic files for the generator
	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}

	// Map slides to SlideSpecs using proper layout selection
	slideSpecs, _, _, err := pipeline.ConvertSlidesPartial(presentation, analysis, true)
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Generate PPTX
	// ExcludeTemplateSlides=true removes the template's example slides from output,
	// so only our generated slides are in the final PPTX (avoiding empty/master slide issues)
	req := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
	}

	result, err := generator.Generate(context.Background(), req)
	if err != nil {
		return fmt.Errorf("generating PPTX: %w", err)
	}

	// Print any warnings
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	fmt.Printf("Generated %s (%d slides, %.2f MB)\n", outputPath, result.SlideCount, float64(result.FileSize)/1024/1024)
	// When ExcludeTemplateSlides=true, template slides are removed, so no slides to skip
	fmt.Printf("SKIP_SLIDES=0\n")
	return nil
}


