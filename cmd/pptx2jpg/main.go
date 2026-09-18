// Package main provides PPTX to JPG conversion using LibreOffice and a PDF
// rasterizer (pdftoppm from poppler, else ImageMagick). It is used for visual
// QA: converting generated slides to images for inspection.
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

// lookPath resolves an executable name to a path. It is a variable so tests
// can simulate hosts with different toolchains installed.
var lookPath = exec.LookPath

// officeCandidates lists LibreOffice entry points in lookup order. Homebrew
// cask and the macOS app bundle only ship `soffice`; Linux distros usually
// provide `libreoffice` (and often `soffice` too).
var officeCandidates = []string{
	"libreoffice",
	"soffice",
	"/Applications/LibreOffice.app/Contents/MacOS/soffice",
}

// rasterizer identifies the PDF->JPG tool that was found.
type rasterizer int

const (
	rasterPdftoppm rasterizer = iota // poppler pdftoppm (no Ghostscript needed)
	rasterMagick                     // ImageMagick 7 `magick`
	rasterConvert                    // ImageMagick 6 `convert`
)

// toolchain is the resolved set of external binaries used for conversion.
type toolchain struct {
	office     string
	raster     string
	rasterKind rasterizer
}

// resolveToolchain finds a LibreOffice binary and a PDF rasterizer.
// pdftoppm is preferred because ImageMagick's PDF delegate needs Ghostscript,
// which is frequently absent on macOS.
func resolveToolchain() (toolchain, error) {
	var tc toolchain
	for _, c := range officeCandidates {
		if p, err := lookPath(c); err == nil {
			tc.office = p
			break
		}
	}
	if tc.office == "" {
		return tc, fmt.Errorf("LibreOffice not found (tried %s); install LibreOffice", strings.Join(officeCandidates, ", "))
	}
	for _, r := range []struct {
		name string
		kind rasterizer
	}{{"pdftoppm", rasterPdftoppm}, {"magick", rasterMagick}, {"convert", rasterConvert}} {
		if p, err := lookPath(r.name); err == nil {
			tc.raster, tc.rasterKind = p, r.kind
			return tc, nil
		}
	}
	return tc, fmt.Errorf("no PDF rasterizer found: install poppler (pdftoppm) or ImageMagick")
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

	tc, err := resolveToolchain()
	if err != nil {
		return err
	}

	// Step 1: Convert PPTX to PDF using LibreOffice. A private user profile
	// keeps the headless run from silently no-op'ing when another soffice
	// instance (e.g. the desktop app or a parallel job) holds the default one.
	profileDir, err := os.MkdirTemp("", "pptx2jpg-lo-")
	if err != nil {
		return fmt.Errorf("failed to create LibreOffice profile dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(profileDir) }()

	fmt.Printf("Converting %s to PDF...\n", pptxPath)
	err = runner.Run(tc.office,
		"-env:UserInstallation=file://"+filepath.ToSlash(profileDir),
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

	// Step 2: Rasterize PDF pages to JPG.
	fmt.Printf("Converting PDF to JPG slides...\n")
	if err := rasterizePDF(runner, tc, pdfPath, outputDir, baseName, density); err != nil {
		return err
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

// rasterizePDF converts every PDF page to "<base>-slide-N.jpg" (unpadded N)
// in outputDir using the resolved rasterizer.
func rasterizePDF(runner CommandRunner, tc toolchain, pdfPath, outputDir, baseName string, density int) error {
	if tc.rasterKind == rasterPdftoppm {
		prefix := filepath.Join(outputDir, baseName+"-slide")
		if err := runner.Run(tc.raster,
			"-r", strconv.Itoa(density),
			"-jpeg", "-jpegopt", "quality=90",
			pdfPath, prefix,
		); err != nil {
			return fmt.Errorf("pdftoppm conversion failed: %w", err)
		}
		return normalizePdftoppmNames(outputDir, baseName)
	}

	jpgPattern := filepath.Join(outputDir, baseName+"-slide-%d.jpg")
	if err := runner.Run(tc.raster,
		"-density", strconv.Itoa(density),
		pdfPath,
		"-quality", "90",
		jpgPattern,
	); err != nil {
		return fmt.Errorf("ImageMagick conversion failed: %w (is ImageMagick installed? PDF input also needs Ghostscript)", err)
	}
	return nil
}

// normalizePdftoppmNames renames pdftoppm output ("<base>-slide-01.jpg",
// 1-based and zero-padded to the page-count width) to the unpadded
// "<base>-slide-N.jpg" names ImageMagick produces, keeping the output
// contract identical regardless of which rasterizer ran.
func normalizePdftoppmNames(outputDir, baseName string) error {
	files, err := filepath.Glob(filepath.Join(outputDir, baseName+"-slide-*.jpg"))
	if err != nil {
		return fmt.Errorf("failed to list pdftoppm output: %w", err)
	}
	for _, f := range files {
		n := slideIndexFromName(f)
		if n < 0 {
			continue
		}
		want := filepath.Join(outputDir, fmt.Sprintf("%s-slide-%d.jpg", baseName, n))
		if want == f {
			continue
		}
		if err := os.Rename(f, want); err != nil {
			return fmt.Errorf("failed to rename %s: %w", f, err)
		}
	}
	return nil
}
