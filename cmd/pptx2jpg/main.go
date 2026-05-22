// Package main provides PPTX to JPG conversion using LibreOffice and ImageMagick.
// It is used for visual QA: converting generated slides to images for inspection.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CommandRunner executes shell commands. This interface allows for mocking in tests.
type CommandRunner interface {
	Run(name string, args ...string) error
}

// RealCommandRunner runs actual shell commands.
type RealCommandRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes the command with the given name and arguments.
func (r *RealCommandRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	return cmd.Run()
}

// defaultRunner is the default command runner for production use.
var defaultRunner CommandRunner = &RealCommandRunner{
	Stdout: os.Stdout,
	Stderr: os.Stderr,
}

// slideIndexFromName extracts the integer N from a filename matching
// "<base>-slide-N.jpg", the unpadded "%d" pattern ImageMagick emits. Returns -1
// when the name doesn't conform, which sorts non-conforming entries first.
func slideIndexFromName(path string) int {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	const marker = "-slide-"
	idx := strings.LastIndex(base, marker)
	if idx < 0 {
		return -1
	}
	n, err := strconv.Atoi(base[idx+len(marker):])
	if err != nil {
		return -1
	}
	return n
}

// sortSlideJPGs orders "<base>-slide-N.jpg" files by the numeric N rather than
// lexicographically. Because ImageMagick emits unpadded indices, a plain string
// sort places slide-10 before slide-2; numeric ordering keeps the listing in
// true slide order for decks with 10+ slides.
func sortSlideJPGs(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return slideIndexFromName(files[i]) < slideIndexFromName(files[j])
	})
}

func main() {
	pptxPath := flag.String("input", "", "Path to PPTX file")
	outputDir := flag.String("output", "", "Output directory for JPG files")
	density := flag.Int("density", 150, "DPI for conversion (default 150)")
	flag.Parse()

	if *pptxPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -input is required")
		os.Exit(1)
	}

	if *outputDir == "" {
		*outputDir = filepath.Dir(*pptxPath)
	}

	if err := convertPPTXToJPG(*pptxPath, *outputDir, *density); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Conversion complete")
}

// convertPPTXToJPG converts a PPTX file to JPG images using LibreOffice and ImageMagick.
// It uses the defaultRunner for command execution, which can be overridden in tests.
func convertPPTXToJPG(pptxPath, outputDir string, density int) error {
	return convertPPTXToJPGWithRunner(pptxPath, outputDir, density, defaultRunner)
}

// convertPPTXToJPGWithRunner converts a PPTX file to JPG images using the provided CommandRunner.
// This function is used internally and for testing.
func convertPPTXToJPGWithRunner(pptxPath, outputDir string, density int, runner CommandRunner) error {
	// Validate input file exists
	if _, err := os.Stat(pptxPath); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", pptxPath)
	}

	// Create output directory if needed
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get base name without extension for output files
	baseName := strings.TrimSuffix(filepath.Base(pptxPath), filepath.Ext(pptxPath))
	pdfPath := filepath.Join(outputDir, baseName+".pdf")

	// Step 1: Convert PPTX to PDF using LibreOffice
	fmt.Printf("Converting %s to PDF...\n", pptxPath)
	err := runner.Run("libreoffice",
		"--headless",
		"--convert-to", "pdf",
		"--outdir", outputDir,
		pptxPath,
	)
	if err != nil {
		return fmt.Errorf("LibreOffice conversion failed: %w (is LibreOffice installed?)", err)
	}

	// Verify PDF was created
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return fmt.Errorf("PDF was not created at expected path: %s", pdfPath)
	}

	// Step 2: Convert PDF pages to JPG using ImageMagick
	fmt.Printf("Converting PDF to JPG slides...\n")
	jpgPattern := filepath.Join(outputDir, baseName+"-slide-%d.jpg")
	err = runner.Run("convert",
		"-density", fmt.Sprintf("%d", density),
		pdfPath,
		"-quality", "90",
		jpgPattern,
	)
	if err != nil {
		return fmt.Errorf("ImageMagick conversion failed: %w (is ImageMagick installed?)", err)
	}

	// List generated files
	files, err := filepath.Glob(filepath.Join(outputDir, baseName+"-slide-*.jpg"))
	if err != nil {
		return fmt.Errorf("failed to list output files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no JPG files were generated")
	}

	// Glob returns names in lexicographic order, which misorders the unpadded
	// "%d" indices (slide-10 before slide-2). Sort numerically so the listing
	// reflects true slide order.
	sortSlideJPGs(files)

	fmt.Printf("Generated %d slide images:\n", len(files))
	for _, f := range files {
		fmt.Printf("  - %s\n", filepath.Base(f))
	}

	// Optional: Clean up PDF
	// os.Remove(pdfPath)

	return nil
}
