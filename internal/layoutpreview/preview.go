// Package layoutpreview generates PNG preview images for template slide layouts.
// It creates a minimal single-slide PPTX per layout using the generator, converts
// to PNG via LibreOffice + ImageMagick, and caches the results on disk.
package layoutpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Result holds the preview PNG paths keyed by layout ID.
type Result struct {
	Paths map[string]string // layout ID -> absolute path to PNG file
}

// Options configures preview generation.
type Options struct {
	// CacheDir is the base directory for cached previews.
	// Defaults to ~/.cache/json2pptx/layout-previews if empty.
	CacheDir string
	// DPI controls the rendering density (default: 96).
	DPI int
}

func (o *Options) cacheDir() string {
	if o != nil && o.CacheDir != "" {
		return o.CacheDir
	}
	return DefaultCacheDir()
}

// DefaultCacheDir returns the base directory layout-preview PNGs are cached
// under when no explicit Options.CacheDir is set
// (~/.cache/json2pptx/layout-previews, falling back to a temp dir when the
// user home directory cannot be resolved). It is exported so discovery
// surfaces (skill-info / list_templates) can report the preview cache location
// to agents alongside the read-only opt-out.
func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "json2pptx-layout-previews")
	}
	return filepath.Join(home, ".cache", "json2pptx", "layout-previews")
}

func (o *Options) dpi() int {
	if o != nil && o.DPI > 0 {
		return o.DPI
	}
	return 96
}

// Generate produces PNG preview images for each layout in the template.
// Returns nil if LibreOffice or ImageMagick is not available (graceful degradation).
func Generate(templatePath string, analysis *types.TemplateAnalysis, opts *Options) (*Result, error) {
	// Check tool availability — graceful degradation when missing
	if !hasLibreOffice() || !hasImageMagick() {
		return nil, nil
	}

	cacheDir := opts.cacheDir()
	templateName := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")

	// Compute a cache key from template file content hash
	hash, err := fileHash(templatePath)
	if err != nil {
		return nil, fmt.Errorf("hash template: %w", err)
	}

	previewDir := filepath.Join(cacheDir, templateName, hash)

	// Check if cache is valid (marker file exists and PNGs present)
	markerPath := filepath.Join(previewDir, ".done")
	if _, err := os.Stat(markerPath); err == nil {
		if result, _ := collectCachedPreviews(previewDir, analysis); result != nil {
			return result, nil
		}
		// Stale marker with no PNGs — regenerate
		_ = os.Remove(markerPath)
	}

	// Generate previews
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return nil, fmt.Errorf("create preview dir: %w", err)
	}

	// Generate a single PPTX with all layouts (one slide per layout)
	// then split the resulting PDF pages into individual PNGs.
	if err := generateAllPreviews(templatePath, analysis, previewDir, opts.dpi()); err != nil {
		return nil, err
	}

	// Write marker
	_ = os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)

	return collectCachedPreviews(previewDir, analysis)
}

// generateAllPreviews creates a PPTX with one slide per layout, converts to PDF,
// then splits into per-layout PNG files.
func generateAllPreviews(templatePath string, analysis *types.TemplateAnalysis, previewDir string, dpi int) error {
	tmpDir, err := os.MkdirTemp("", "layoutpreview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Build slide specs — one slide per layout with sample content so
	// placeholder positions are visible in the rendered PNG.
	slides := make([]generator.SlideSpec, len(analysis.Layouts))
	for i, layout := range analysis.Layouts {
		slides[i] = generator.SlideSpec{
			LayoutID: layout.ID,
			Content:  sampleContent(layout),
		}
	}

	// Generate a PPTX using the real generator
	pptxPath := filepath.Join(tmpDir, "layouts.pptx")
	ctx := context.Background()
	_, err = generator.Generate(ctx, generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            pptxPath,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	})
	if err != nil {
		return fmt.Errorf("generate preview pptx: %w", err)
	}

	// Convert to PDF via LibreOffice
	loBin := libreOfficeBin()
	cmd := exec.Command(loBin, "--headless", "--convert-to", "pdf", "--outdir", tmpDir, pptxPath) //nolint:gosec // binary path from LookPath
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("libreoffice convert: %w", err)
	}

	pdfPath := filepath.Join(tmpDir, "layouts.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("pdf not created by libreoffice")
	}

	// Split PDF pages into individual PNGs using ImageMagick
	magickBin := imageMagickBin()
	for i, layout := range analysis.Layouts {
		pngPath := filepath.Join(previewDir, layout.ID+".png")
		pageSpec := fmt.Sprintf("%s[%d]", pdfPath, i)
		cmd := exec.Command(magickBin, "-density", fmt.Sprintf("%d", dpi), pageSpec, "-quality", "90", pngPath) //nolint:gosec // binary path from LookPath
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			// Non-fatal: skip this layout's preview
			continue
		}
	}

	return nil
}

func collectCachedPreviews(previewDir string, analysis *types.TemplateAnalysis) (*Result, error) {
	result := &Result{Paths: make(map[string]string)}
	for _, layout := range analysis.Layouts {
		pngPath := filepath.Join(previewDir, layout.ID+".png")
		if _, err := os.Stat(pngPath); err == nil {
			result.Paths[layout.ID] = pngPath
		}
	}
	if len(result.Paths) == 0 {
		return nil, nil
	}
	return result, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func hasLibreOffice() bool {
	if _, err := exec.LookPath("libreoffice"); err == nil {
		return true
	}
	_, err := exec.LookPath("soffice")
	return err == nil
}

func libreOfficeBin() string {
	if _, err := exec.LookPath("libreoffice"); err == nil {
		return "libreoffice"
	}
	return "soffice"
}

func hasImageMagick() bool {
	if _, err := exec.LookPath("magick"); err == nil {
		return true
	}
	_, err := exec.LookPath("convert")
	return err == nil
}

func imageMagickBin() string {
	if path, err := exec.LookPath("magick"); err == nil {
		return path
	}
	path, _ := exec.LookPath("convert")
	return path
}

// sampleContent builds placeholder content items for a layout so the preview
// renders visible text in each placeholder region rather than a blank slide.
func sampleContent(layout types.LayoutMetadata) []generator.ContentItem {
	var items []generator.ContentItem
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderTitle:
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentText,
				Value:         layout.Name,
			})
		case types.PlaceholderSubtitle:
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentText,
				Value:         "Subtitle placeholder",
			})
		case types.PlaceholderBody, types.PlaceholderContent:
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentBullets,
				Value:         []string{"First bullet point", "Second bullet point", "Third bullet point"},
			})
		// Skip PlaceholderOther (date, footer, slide number) — not useful for previews.
		}
	}
	return items
}
