package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
)

// preview-patterns builds a local gallery: every named pattern on every
// template, each as the one-slide deck render_deck_spec produces for the
// pattern's exemplar values under a title (go-slide-creator-r1uy7). It shares
// renderSpecPreview with list_slide_kinds(preview:true) and
// recommend_visual(preview:true), so a gallery tile is the generated slide,
// not a separate drawing of the pattern.
//
// The gallery is not committed: 51 patterns on nine templates is several
// hundred images that go stale with every layout change. Agents get the same
// picture on demand, as an MCP image, through the render cache.
func runPreviewPatterns() error {
	fs := flag.NewFlagSet("preview-patterns", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./assets/pattern-previews", "Output directory for pattern preview PNGs")
	density := fs.Int("density", 150, "DPI for rendered PNGs")
	patternFilter := fs.String("pattern", "", "Generate preview for a single pattern only")
	templateFilter := fs.String("template", "", "Generate previews on a single template only")
	manifest := fs.Bool("manifest", false, "Emit a JSON success manifest (written PNG paths, kind, byte length, sha256, and per-pattern warnings) to stdout instead of relying on the stderr progress log.")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx preview-patterns [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render every named pattern on each template as the one-slide deck\n")
		fmt.Fprintf(os.Stderr, "generation produces for its example values, and write one PNG per pattern.\n")
		fmt.Fprintf(os.Stderr, "Requires LibreOffice and ImageMagick on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Output structure:\n")
		fmt.Fprintf(os.Stderr, "  <output>/<template-name>/<pattern-name>.png\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if err := render.CheckDependencies(); err != nil {
		return fmt.Errorf("preview-patterns requires LibreOffice and ImageMagick on PATH: %w", err)
	}

	templateFiles, err := filepath.Glob(filepath.Join(*templatesDir, "*.pptx"))
	if err != nil || len(templateFiles) == 0 {
		return fmt.Errorf("no templates found in %s", *templatesDir)
	}

	reg := patterns.Default()
	var names []string
	for _, p := range reg.List() {
		if *patternFilter != "" && p.Name() != *patternFilter {
			continue
		}
		names = append(names, p.Name())
	}
	if len(names) == 0 {
		return fmt.Errorf("no pattern named %q", *patternFilter)
	}

	mc := &mcpConfig{templatesDir: *templatesDir, cache: template.NewMemoryCache(0)}
	ctx := context.Background()
	var written []artifactSpec
	var warnings []string
	matchedTemplate := false
	for _, tplPath := range templateFiles {
		tplName := strings.TrimSuffix(filepath.Base(tplPath), ".pptx")
		if *templateFilter != "" && tplName != *templateFilter {
			continue
		}
		matchedTemplate = true
		tplOutDir := filepath.Join(*outputDir, tplName)
		if err := os.MkdirAll(tplOutDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", tplOutDir, err)
		}
		for _, name := range names {
			pngPath := filepath.Join(tplOutDir, name+".png")
			if err := mc.writePatternPreview(ctx, reg, name, tplName, pngPath, *density); err != nil {
				warn := fmt.Sprintf("%s/%s: %v", tplName, name, err)
				warnings = append(warnings, warn)
				fmt.Fprintf(os.Stderr, "WARN: %s\n", warn)
				continue
			}
			written = append(written, artifactSpec{path: pngPath, kind: "pattern-preview"})
			fmt.Fprintf(os.Stderr, "  %s/%s.png\n", tplName, name)
		}
	}
	if !matchedTemplate {
		return fmt.Errorf("no template named %q in %s", *templateFilter, *templatesDir)
	}

	fmt.Fprintf(os.Stderr, "\nGenerated %d pattern preview PNGs in %s\n", len(written), *outputDir)

	if *manifest {
		m, err := buildWriteManifest("preview-patterns", written, warnings)
		if err != nil {
			return err
		}
		return printWriteManifest(os.Stdout, m)
	}
	return nil
}

// writePatternPreview renders one pattern's preview on one template and
// writes the PNG.
func (mc *mcpConfig) writePatternPreview(ctx context.Context, reg *patterns.Registry, name, templateName, outputPNG string, dpi int) error {
	spec, err := patternPreviewSpec(reg, name, templateName)
	if err != nil {
		return err
	}
	img, err := mc.renderSpecPreview(ctx, spec, dpi)
	if err != nil {
		return err
	}
	data, err := readSlideImageBytes(img)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPNG, data, 0644) //nolint:gosec // a gallery image the caller asked for at this path
}
