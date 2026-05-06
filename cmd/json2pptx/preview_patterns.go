package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func runPreviewPatterns() error {
	fs := flag.NewFlagSet("preview-patterns", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./assets/pattern-previews", "Output directory for pattern preview PNGs")
	density := fs.Int("density", 150, "DPI for rendered PNGs")
	patternFilter := fs.String("pattern", "", "Generate preview for a single pattern only")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preview-patterns [options]\n\n")
		fmt.Fprintf(os.Stderr, "Generate PNG preview images for all named patterns using each template.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Output structure:\n")
		fmt.Fprintf(os.Stderr, "  <output>/<template-name>/<pattern-name>.png\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if !hasLOBinary() || !hasMagickBinary() {
		return fmt.Errorf("preview-patterns requires LibreOffice and ImageMagick on PATH")
	}

	// Discover templates
	templateFiles, err := filepath.Glob(filepath.Join(*templatesDir, "*.pptx"))
	if err != nil || len(templateFiles) == 0 {
		return fmt.Errorf("no templates found in %s", *templatesDir)
	}

	// Collect patterns with ExemplarValues
	reg := patterns.Default()
	allPatterns := reg.List()
	var exemplarPatterns []patterns.Pattern
	for _, p := range allPatterns {
		if _, ok := p.(patterns.Exemplar); ok {
			if *patternFilter != "" && p.Name() != *patternFilter {
				continue
			}
			exemplarPatterns = append(exemplarPatterns, p)
		}
	}

	if len(exemplarPatterns) == 0 {
		return fmt.Errorf("no patterns with ExemplarValues found")
	}

	cache := template.NewMemoryCache(0)
	generated := 0
	for _, tplPath := range templateFiles {
		tplName := strings.TrimSuffix(filepath.Base(tplPath), ".pptx")
		tplOutDir := filepath.Join(*outputDir, tplName)
		if err := os.MkdirAll(tplOutDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", tplOutDir, err)
		}

		// Analyze template
		analysis, err := getOrAnalyzeTemplate(tplPath, cache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: skip template %s: %v\n", tplName, err)
			continue
		}

		for _, pat := range exemplarPatterns {
			pngPath := filepath.Join(tplOutDir, pat.Name()+".png")
			if err := generateOnePatternPreview(tplPath, analysis, pat, pngPath, *density); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: %s/%s: %v\n", tplName, pat.Name(), err)
				continue
			}
			generated++
			fmt.Fprintf(os.Stderr, "  %s/%s.png\n", tplName, pat.Name())
		}
	}

	fmt.Fprintf(os.Stderr, "\nGenerated %d pattern preview PNGs in %s\n", generated, *outputDir)
	return nil
}

func generateOnePatternPreview(
	templatePath string,
	analysis *types.TemplateAnalysis,
	pat patterns.Pattern,
	outputPNG string,
	dpi int,
) error {
	exemplar, ok := pat.(patterns.Exemplar)
	if !ok {
		return fmt.Errorf("pattern %s does not implement Exemplar", pat.Name())
	}

	// Use template slide dimensions or standard 16:9 defaults
	slideWidth := analysis.SlideWidth
	slideHeight := analysis.SlideHeight
	if slideWidth == 0 {
		slideWidth = 9144000
	}
	if slideHeight == 0 {
		slideHeight = 5143500
	}

	// Build expand context with template theme
	expandCtx := patterns.ExpandContext{
		Theme:       analysis.Theme,
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: slideWidth - 914400, Height: slideHeight - 914400,
		},
	}
	if analysis.Metadata != nil {
		expandCtx.Metadata = analysis.Metadata
	}

	// Expand the pattern with exemplar values
	grid, err := pat.Expand(expandCtx, exemplar.ExemplarValues(), nil, nil)
	if err != nil {
		return fmt.Errorf("expand: %w", err)
	}
	// Convert the jsonschema.ShapeGridInput to our local ShapeGridInput type
	// (both use the same JSON schema, but are different Go types)
	gridJSON, err := json.Marshal(grid)
	if err != nil {
		return fmt.Errorf("marshal grid: %w", err)
	}
	var localGrid ShapeGridInput
	if err := json.Unmarshal(gridJSON, &localGrid); err != nil {
		return fmt.Errorf("unmarshal grid: %w", err)
	}

	// Resolve shape_grid to raw XML
	alloc := &pptx.ShapeIDAllocator{}
	alloc.SetMinID(200)
	gridResult, err := resolveShapeGrid(&localGrid, alloc, nil, nil, slideWidth, slideHeight, nil)
	if err != nil {
		return fmt.Errorf("resolve grid: %w", err)
	}
	if gridResult == nil {
		return fmt.Errorf("empty grid result")
	}

	// Pick a blank-ish layout (fewest placeholders)
	layoutID := pickPreviewLayout(analysis.Layouts)

	// Build single-slide spec
	spec := generator.SlideSpec{
		LayoutID:     layoutID,
		RawShapeXML:  gridResult.Shapes,
		IconInserts:  gridResult.IconInserts,
		ImageInserts: gridResult.ImageInserts,
	}

	// Generate PPTX in temp dir
	tmpDir, err := os.MkdirTemp("", "patternpreview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	pptxPath := filepath.Join(tmpDir, "pattern.pptx")
	ctx := context.Background()
	_, err = generator.Generate(ctx, generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            pptxPath,
		Slides:                []generator.SlideSpec{spec},
		ExcludeTemplateSlides: true,
	})
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// Convert PPTX -> PDF via LibreOffice
	loBin := findLOBinary()
	cmd := exec.Command(loBin, "--headless", "--convert-to", "pdf", "--outdir", tmpDir, pptxPath) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("libreoffice: %w", err)
	}

	pdfPath := filepath.Join(tmpDir, "pattern.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("pdf not created")
	}

	// PDF -> PNG via ImageMagick
	magickBin := findMagickBinary()
	pageSpec := fmt.Sprintf("%s[0]", pdfPath)
	cmd = exec.Command(magickBin, "-density", fmt.Sprintf("%d", dpi), pageSpec, "-quality", "90", outputPNG) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("imagemagick: %w", err)
	}

	return nil
}

// pickPreviewLayout selects a layout with few placeholders for clean pattern rendering.
func pickPreviewLayout(layouts []types.LayoutMetadata) string {
	if len(layouts) == 0 {
		return "slideLayout1"
	}
	best := layouts[0]
	for _, l := range layouts[1:] {
		if len(l.Placeholders) < len(best.Placeholders) {
			best = l
		}
	}
	return best.ID
}

// findPatternPreviewPNGs looks for pre-generated pattern preview PNGs in the
// assets/pattern-previews directory adjacent to the templates directory.
// Returns absolute paths to any found PNGs for the given pattern across all templates.
func findPatternPreviewPNGs(templatesDir, patternName string) []string {
	// Preview PNGs live at <project-root>/assets/pattern-previews/<template>/<pattern>.png
	// The templates dir is typically at <project-root>/templates, so go up one level.
	projectRoot := filepath.Dir(templatesDir)
	previewsDir := filepath.Join(projectRoot, "assets", "pattern-previews")

	matches, err := filepath.Glob(filepath.Join(previewsDir, "*", patternName+".png"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	return matches
}

func hasLOBinary() bool {
	if _, err := exec.LookPath("libreoffice"); err == nil {
		return true
	}
	_, err := exec.LookPath("soffice")
	return err == nil
}

func findLOBinary() string {
	if _, err := exec.LookPath("libreoffice"); err == nil {
		return "libreoffice"
	}
	return "soffice"
}

func hasMagickBinary() bool {
	if _, err := exec.LookPath("magick"); err == nil {
		return true
	}
	_, err := exec.LookPath("convert")
	return err == nil
}

func findMagickBinary() string {
	if path, err := exec.LookPath("magick"); err == nil {
		return path
	}
	path, _ := exec.LookPath("convert")
	return path
}
